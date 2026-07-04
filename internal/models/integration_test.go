package models

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"shizoid/internal/logger"
	"shizoid/internal/migrations"
)

// testStore is non-nil when the dockerized Postgres started successfully; tests
// that need it call requireDB to skip gracefully otherwise (e.g. no docker).
var testStore *Store

// chatSeq hands out unique chat ids so tests do not interfere.
var chatSeq atomic.Int64

func nextChatID() int64 { return 1_000_000 + chatSeq.Add(1) }

func TestMain(m *testing.M) {
	flag.Parse()
	logger.Init(true, "")
	os.Exit(runWithPostgres(m))
}

func runWithPostgres(m *testing.M) int {
	if testing.Short() {
		return m.Run()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	container, err := tcpostgres.Run(ctx, "postgres:18-alpine",
		tcpostgres.WithDatabase("shizoid_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration tests skipped: postgres container: %v\n", err)
		return m.Run()
	}
	defer func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			fmt.Fprintf(os.Stderr, "terminate container: %v\n", err)
		}
	}()

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration tests skipped: connection string: %v\n", err)
		return m.Run()
	}
	if err := runMigrations(dsn); err != nil {
		fmt.Fprintf(os.Stderr, "integration tests failed: migrations: %v\n", err)
		return 1
	}
	pool, err := OpenPool(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration tests skipped: pool: %v\n", err)
		return m.Run()
	}
	defer pool.Close()

	testStore = NewStore(pool)
	defer func() { testStore = nil }()
	return m.Run()
}

// runMigrations applies goose migrations over a short-lived database/sql
// connection, mirroring cmd/app.
func runMigrations(dsn string) error {
	dbh, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer dbh.Close()
	return migrations.Run(dbh)
}

func requireDB(t *testing.T) *Store {
	t.Helper()
	if testStore == nil {
		t.Skip("postgres container unavailable")
	}
	return testStore
}

func seedChat(t *testing.T, ctx context.Context) *Chat {
	t.Helper()
	chat := &Chat{ID: nextChatID(), Kind: "supergroup", Locale: "ru"}
	user := &User{ID: chat.ID + 500_000}
	user.Username = sql.NullString{String: fmt.Sprintf("user%d", user.ID), Valid: true}
	persisted, participation, err := testStore.Ingest.EnsureEntities(ctx, chat, user, false)
	require.NoError(t, err)
	require.NotNil(t, persisted)
	require.NotNil(t, participation)
	return persisted
}

func TestOpenPoolInvalidDSN(t *testing.T) {
	_, err := OpenPool(context.Background(), "host=invalid connect_timeout=1")
	require.Error(t, err)
}

func TestIntegrationWordsRoundTrip(t *testing.T) {
	s := requireDB(t)
	ctx := context.Background()
	tokens := []string{"альфа", "бета", "гамма", "альфа", ""}

	require.NoError(t, s.Words.EnsureWords(ctx, tokens))
	// idempotent second call
	require.NoError(t, s.Words.EnsureWords(ctx, tokens))

	ids, err := s.Words.ToIDs(ctx, tokens)
	require.NoError(t, err)
	require.Len(t, ids, 3)

	var back []int64
	for _, id := range ids {
		back = append(back, id)
	}
	words, err := s.Words.ToWords(ctx, back)
	require.NoError(t, err)
	assert.Len(t, words, 3)
	assert.Equal(t, ids["бета"], func() int64 {
		for id, w := range words {
			if w == "бета" {
				return id
			}
		}
		return 0
	}())
}

func TestIntegrationLearnTrigramAndFetchPair(t *testing.T) {
	s := requireDB(t)
	ctx := context.Background()
	chat := seedChat(t, ctx)

	tokens := []string{"один", "два", "три"}
	ids, err := s.Words.EnsureIDs(ctx, tokens)
	require.NoError(t, err)
	require.Len(t, ids, 3)

	// EnsureIDs must return the same ids for pre-existing words
	again, err := s.Words.EnsureIDs(ctx, tokens)
	require.NoError(t, err)
	assert.Equal(t, ids, again)

	null := sql.NullInt64{}
	w := func(str string) sql.NullInt64 { return sql.NullInt64{Int64: ids[str], Valid: true} }

	// learn (NULL, один, два) and (один, два, три) twice: once batched, once split,
	// so batching accumulates counts identically to sequential learning
	require.NoError(t, s.Pairs.LearnTrigrams(ctx, chat.ID, []Trigram{
		{First: null, Second: w("один"), Third: w("два")},
		{First: w("один"), Second: w("два"), Third: w("три")},
		{First: null, Second: w("один"), Third: w("два")},
	}))
	require.NoError(t, s.Pairs.LearnTrigrams(ctx, chat.ID, []Trigram{
		{First: w("один"), Second: w("два"), Third: w("три")},
	}))

	pair, err := s.Pairs.FetchPair(ctx, chat.ID, MatchNull(), MatchIn([]int64{ids["один"]}))
	require.NoError(t, err)
	require.NotNil(t, pair, "seeded pair must be found")
	assert.Equal(t, ids["один"], pair.SecondID.Int64)
	require.Len(t, pair.Replies, 1)
	assert.Equal(t, ids["два"], pair.Replies[0].WordID.Int64)
	assert.Equal(t, 2, pair.Replies[0].Count, "repeated learning must increment reply count")

	next, err := s.Pairs.FetchPair(ctx, chat.ID, MatchEq(w("один")), MatchEq(w("два")))
	require.NoError(t, err)
	require.NotNil(t, next)
	require.Len(t, next.Replies, 1)
	assert.Equal(t, ids["три"], next.Replies[0].WordID.Int64)

	missing, err := s.Pairs.FetchPair(ctx, chat.ID, MatchEq(w("три")), MatchEq(w("один")))
	require.NoError(t, err)
	assert.Nil(t, missing, "unknown pair must yield nil, not error")
}

func TestIntegrationMessagesByteWindows(t *testing.T) {
	s := requireDB(t)
	ctx := context.Background()
	chat := seedChat(t, ctx)
	userID := chat.ID + 500_000

	for i := range 5 {
		require.NoError(t, s.Messages.Append(ctx, chat.ID, userID, fmt.Sprintf("msg-%d-xxxxxxxxxx", i)))
	}

	// generous budget returns all, newest first
	texts, err := s.Messages.RecentTextsByBytes(ctx, chat.ID, 10_000)
	require.NoError(t, err)
	require.Len(t, texts, 5)
	assert.Equal(t, "msg-4-xxxxxxxxxx", texts[0])

	// budget smaller than one row falls back to the single latest message
	texts, err = s.Messages.RecentTextsByBytes(ctx, chat.ID, 3)
	require.NoError(t, err)
	require.Len(t, texts, 1)
	assert.Equal(t, "msg-4-xxxxxxxxxx", texts[0])

	rows, err := s.Messages.RecentByBytes(ctx, chat.ID, 10_000)
	require.NoError(t, err)
	require.Len(t, rows, 5)
	assert.Equal(t, userID, rows[0].UserID)
	assert.True(t, rows[0].Username.Valid, "join with users must populate profile fields")

	last, err := s.Messages.LastActivity(ctx, chat.ID)
	require.NoError(t, err)
	assert.True(t, last.Valid)
}

func TestIntegrationMessagesPruneByBytes(t *testing.T) {
	s := requireDB(t)
	ctx := context.Background()
	chat := seedChat(t, ctx)
	userID := chat.ID + 500_000

	payload := "0123456789" // 10 bytes each
	for range 10 {
		require.NoError(t, s.Messages.Append(ctx, chat.ID, userID, payload))
	}

	chatIDs, err := s.Messages.ChatIDs(ctx)
	require.NoError(t, err)
	assert.Contains(t, chatIDs, chat.ID)

	deleted, err := s.Messages.PruneChatByBytes(ctx, chat.ID, 50)
	require.NoError(t, err)
	assert.Equal(t, int64(5), deleted, "rows beyond the 50-byte budget must be pruned")

	texts, err := s.Messages.RecentTextsByBytes(ctx, chat.ID, 10_000)
	require.NoError(t, err)
	assert.Len(t, texts, 5)
}

func TestIntegrationIngestEnsureEntitiesIdempotent(t *testing.T) {
	s := requireDB(t)
	ctx := context.Background()

	chat := &Chat{ID: nextChatID(), Kind: "supergroup", Locale: "ru"}
	user := &User{ID: chat.ID + 500_000}

	first, p1, err := s.Ingest.EnsureEntities(ctx, chat, user, false)
	require.NoError(t, err)
	require.NotNil(t, first)
	require.NotNil(t, p1)
	assert.Equal(t, chat.ID, first.ID)

	second, p2, err := s.Ingest.EnsureEntities(ctx, chat, user, false)
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID)
	assert.Equal(t, p1.ID, p2.ID, "participation must be reused, not duplicated")
}

func TestIntegrationWinnersLifecycle(t *testing.T) {
	s := requireDB(t)
	ctx := context.Background()
	chat := seedChat(t, ctx)
	userID := chat.ID + 500_000

	has, err := s.Winners.HasToday(ctx, chat.ID)
	require.NoError(t, err)
	assert.False(t, has)

	inserted, err := s.Winners.Create(ctx, chat.ID, userID)
	require.NoError(t, err)
	assert.True(t, inserted)

	inserted, err = s.Winners.Create(ctx, chat.ID, userID)
	require.NoError(t, err)
	assert.False(t, inserted, "same-day duplicate must be rejected")

	has, err = s.Winners.HasToday(ctx, chat.ID)
	require.NoError(t, err)
	assert.True(t, has)

	id, username, _, found, err := s.Winners.LastWinner(ctx, chat.ID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, userID, id)
	assert.NotEmpty(t, username)

	top, err := s.Winners.TopOfYear(ctx, chat.ID, 10)
	require.NoError(t, err)
	require.Len(t, top, 1)
	assert.Equal(t, userID, top[0].UserID)
	assert.Equal(t, 1, top[0].Score)
}
