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
	persisted, err := testStore.Ingest.EnsureEntities(ctx, chat, user, false)
	require.NoError(t, err)
	require.NotNil(t, persisted)
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

	first, err := s.Words.EnsureIDs(ctx, tokens)
	require.NoError(t, err)
	require.Len(t, first, 3)
	// idempotent second call returns the same ids
	second, err := s.Words.EnsureIDs(ctx, tokens)
	require.NoError(t, err)
	assert.Equal(t, first, second)

	ids, err := s.Words.ToIDs(ctx, tokens)
	require.NoError(t, err)
	require.Len(t, ids, 3)
	assert.Equal(t, first, ids)

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

}

// A chat whose memory has never been summarized has no memory_summarized_at,
// so the summarizer asks for everything with a zero since. The time bound has
// to be dropped from the query then, not left as a placeholder nothing binds.
func TestIntegrationMessagesTextsSinceNeverSummarized(t *testing.T) {
	s := requireDB(t)
	ctx := context.Background()
	chat := seedChat(t, ctx)
	userID := chat.ID + 500_000

	require.NoError(t, s.Messages.Append(ctx, chat.ID, userID, "первое"))
	require.NoError(t, s.Messages.Append(ctx, chat.ID, userID, "второе"))

	texts, err := s.Messages.TextsSinceByBytes(ctx, chat.ID, time.Time{}, 10_000)
	require.NoError(t, err)
	assert.Equal(t, []string{"первое", "второе"}, texts, "chronological order")

	// a since bound still filters
	texts, err = s.Messages.TextsSinceByBytes(ctx, chat.ID, time.Now().Add(time.Hour), 10_000)
	require.NoError(t, err)
	assert.Empty(t, texts)
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

	deleted, err := s.Messages.PruneChatByBytes(ctx, chat.ID, 50, false)
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

	first, err := s.Ingest.EnsureEntities(ctx, chat, user, false)
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.Equal(t, chat.ID, first.ID)

	second, err := s.Ingest.EnsureEntities(ctx, chat, user, false)
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID)

	// Ensure no longer returns the row, so prove the participation was reused
	// rather than duplicated through its observable effect: a score increment
	// must land on exactly one row.
	require.NoError(t, s.Participations.IncrScore(ctx, chat.ID, user.ID, 5))
	top, err := s.Participations.TopByScore(ctx, chat.ID, 10)
	require.NoError(t, err)
	require.Len(t, top, 1, "participation must be reused, not duplicated")
	assert.Equal(t, 5, top[0].Score)
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

func TestIntegrationMessagesPruneKeepsUnsummarized(t *testing.T) {
	s := requireDB(t)
	ctx := context.Background()
	chat := seedChat(t, ctx)
	userID := chat.ID + 500_000

	payload := "0123456789" // 10 bytes each
	for range 5 {
		require.NoError(t, s.Messages.Append(ctx, chat.ID, userID, payload))
	}
	// Everything so far counts as summarized; everything after it does not.
	require.NoError(t, s.Chats.SetMemorySummarizedAt(ctx, chat.ID, time.Now()))
	for range 5 {
		require.NoError(t, s.Messages.Append(ctx, chat.ID, userID, payload))
	}

	deleted, err := s.Messages.PruneChatByBytes(ctx, chat.ID, 20, true)
	require.NoError(t, err)
	assert.Equal(t, int64(5), deleted, "only the summarized rows may be pruned")

	texts, err := s.Messages.RecentTextsByBytes(ctx, chat.ID, 10_000)
	require.NoError(t, err)
	assert.Len(t, texts, 5, "the unsummarized tail must survive its byte budget")
}

func TestIntegrationMessagesPruneKeepsAllWhenNeverSummarized(t *testing.T) {
	s := requireDB(t)
	ctx := context.Background()
	chat := seedChat(t, ctx)
	userID := chat.ID + 500_000

	for range 10 {
		require.NoError(t, s.Messages.Append(ctx, chat.ID, userID, "0123456789"))
	}

	deleted, err := s.Messages.PruneChatByBytes(ctx, chat.ID, 20, true)
	require.NoError(t, err)
	assert.Equal(t, int64(0), deleted, "a chat with no summary yet has nothing prunable")
}

func TestIntegrationSummaryJobsClaimLeasesOneJob(t *testing.T) {
	s := requireDB(t)
	ctx := context.Background()
	chat := seedChat(t, ctx)

	require.NoError(t, s.SummaryJobs.Enqueue(ctx, chat.ID, SummaryJobWinner, []byte(`{"a":1}`), time.Hour))

	job, ok, err := s.SummaryJobs.Claim(ctx, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, chat.ID, job.ChatID)
	assert.Equal(t, SummaryJobWinner, job.Kind)
	assert.JSONEq(t, `{"a":1}`, string(job.Payload))
	assert.Equal(t, 1, job.Attempts, "claiming counts as an attempt")

	_, ok, err = s.SummaryJobs.Claim(ctx, time.Minute)
	require.NoError(t, err)
	assert.False(t, ok, "a leased job must not be claimable again")

	// Retry overwrites the lease, so a job due again is picked back up.
	require.NoError(t, s.SummaryJobs.Retry(ctx, job, -time.Second))
	again, ok, err := s.SummaryJobs.Claim(ctx, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, job.ID, again.ID)
	assert.Equal(t, 2, again.Attempts)

	require.NoError(t, s.SummaryJobs.Done(ctx, again))
	_, ok, err = s.SummaryJobs.Claim(ctx, time.Minute)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestIntegrationSummaryJobsEnqueueReplacesThePendingOne(t *testing.T) {
	s := requireDB(t)
	ctx := context.Background()
	chat := seedChat(t, ctx)

	require.NoError(t, s.SummaryJobs.Enqueue(ctx, chat.ID, SummaryJobWinner, []byte(`{"draw":"old"}`), time.Hour))
	_, ok, err := s.SummaryJobs.Claim(ctx, time.Hour)
	require.NoError(t, err)
	require.True(t, ok)

	require.NoError(t, s.SummaryJobs.Enqueue(ctx, chat.ID, SummaryJobWinner, []byte(`{"draw":"new"}`), time.Hour))

	job, ok, err := s.SummaryJobs.Claim(ctx, time.Minute)
	require.NoError(t, err)
	require.True(t, ok, "re-enqueueing must clear the lease of the job it replaces")
	require.Equal(t, chat.ID, job.ChatID)
	assert.JSONEq(t, `{"draw":"new"}`, string(job.Payload))
	assert.Equal(t, 1, job.Attempts, "a replaced job starts its backoff over")

	require.NoError(t, s.SummaryJobs.Done(ctx, job))
}

// The memory cron and the queue drain run side by side, so an Enqueue can land
// between claiming a job and finishing it. Finishing the claim must not take the
// work that was queued in the meantime down with it.
func TestIntegrationSummaryJobsDoneSparesWorkQueuedMidJob(t *testing.T) {
	s := requireDB(t)
	ctx := context.Background()
	chat := seedChat(t, ctx)

	require.NoError(t, s.SummaryJobs.Enqueue(ctx, chat.ID, SummaryJobMemory, []byte(`{"run":"first"}`), time.Hour))
	job, ok, err := s.SummaryJobs.Claim(ctx, time.Hour)
	require.NoError(t, err)
	require.True(t, ok)

	require.NoError(t, s.SummaryJobs.Enqueue(ctx, chat.ID, SummaryJobMemory, []byte(`{"run":"second"}`), time.Hour))
	require.NoError(t, s.SummaryJobs.Done(ctx, job))

	queued, ok, err := s.SummaryJobs.Claim(ctx, time.Minute)
	require.NoError(t, err)
	require.True(t, ok, "the job queued mid-run must survive the finish of the one it replaced")
	assert.JSONEq(t, `{"run":"second"}`, string(queued.Payload))
	require.NoError(t, s.SummaryJobs.Done(ctx, queued))
}

// A failed job that was replaced while it ran keeps the schedule Enqueue gave
// it: the backoff belongs to the attempt that failed, not to the new payload.
func TestIntegrationSummaryJobsRetrySparesWorkQueuedMidJob(t *testing.T) {
	s := requireDB(t)
	ctx := context.Background()
	chat := seedChat(t, ctx)

	require.NoError(t, s.SummaryJobs.Enqueue(ctx, chat.ID, SummaryJobMemory, nil, time.Hour))
	job, ok, err := s.SummaryJobs.Claim(ctx, time.Hour)
	require.NoError(t, err)
	require.True(t, ok)

	require.NoError(t, s.SummaryJobs.Enqueue(ctx, chat.ID, SummaryJobMemory, nil, time.Hour))
	require.NoError(t, s.SummaryJobs.Retry(ctx, job, time.Hour))

	queued, ok, err := s.SummaryJobs.Claim(ctx, time.Minute)
	require.NoError(t, err)
	require.True(t, ok, "the requeued job must stay due instead of inheriting the failure's backoff")
	assert.Equal(t, 1, queued.Attempts)
	require.NoError(t, s.SummaryJobs.Done(ctx, queued))
}

func TestIntegrationSummaryJobsKindsAreIndependent(t *testing.T) {
	s := requireDB(t)
	ctx := context.Background()
	chat := seedChat(t, ctx)

	require.NoError(t, s.SummaryJobs.Enqueue(ctx, chat.ID, SummaryJobWinner, []byte(`{"k":"w"}`), time.Hour))
	require.NoError(t, s.SummaryJobs.Enqueue(ctx, chat.ID, SummaryJobMemory, nil, time.Hour))

	kinds := map[SummaryJobKind][]byte{}
	for range 2 {
		job, ok, err := s.SummaryJobs.Claim(ctx, time.Hour)
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, chat.ID, job.ChatID)
		kinds[job.Kind] = job.Payload
		require.NoError(t, s.SummaryJobs.Done(ctx, job))
	}
	require.Len(t, kinds, 2, "both kinds must coexist for one chat")
	assert.JSONEq(t, `{"k":"w"}`, string(kinds[SummaryJobWinner]))
	assert.JSONEq(t, `{}`, string(kinds[SummaryJobMemory]), "an empty payload is stored as an empty object")
}

func TestIntegrationSummaryJobsExpireOverdue(t *testing.T) {
	s := requireDB(t)
	ctx := context.Background()
	chat := seedChat(t, ctx)

	require.NoError(t, s.SummaryJobs.Enqueue(ctx, chat.ID, SummaryJobWinner, nil, -time.Second))
	require.NoError(t, s.SummaryJobs.Enqueue(ctx, chat.ID, SummaryJobMemory, nil, time.Hour))

	all, err := s.SummaryJobs.ExpireOverdue(ctx)
	require.NoError(t, err)
	var expired []SummaryJob
	for _, job := range all {
		if job.ChatID == chat.ID {
			expired = append(expired, job)
		}
	}
	require.Len(t, expired, 1)
	assert.Equal(t, SummaryJobWinner, expired[0].Kind)

	job, ok, err := s.SummaryJobs.Claim(ctx, time.Minute)
	require.NoError(t, err)
	require.True(t, ok, "a job still in date must survive the sweep")
	assert.Equal(t, SummaryJobMemory, job.Kind)
	require.NoError(t, s.SummaryJobs.Done(ctx, job))
}
