package app

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"shizoid/internal/config"
	"shizoid/internal/logger"
	"shizoid/internal/models"
)

func TestMain(m *testing.M) {
	logger.Init(true, "")
	os.Exit(m.Run())
}

func loadTestConfig(t *testing.T, yamlBody string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(yamlBody), 0o600))
	require.NoError(t, config.Load(path))
}

func TestReady(t *testing.T) {
	models.Init(nil)
	assert.False(t, Ready())

	db, err := sql.Open("postgres", "host=invalid")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	Init(db)
	assert.True(t, Ready())
	assert.NotNil(t, Gen())
}

func TestIsOwner(t *testing.T) {
	loadTestConfig(t, `
telegram:
  token: "123:ABC"
app:
  bot_owners:
    - 42
    - 99
`)

	assert.True(t, IsOwner(42))
	assert.True(t, IsOwner(99))
	assert.False(t, IsOwner(1))
}

func TestBotIDAndUsername(t *testing.T) {
	SetBotID(12345)
	assert.Equal(t, int64(12345), BotID())

	SetBotUsername("Shizoid_Bot")
	assert.Equal(t, "shizoid_bot", BotUsername())
}

func TestChatFrom(t *testing.T) {
	ctx := context.Background()
	assert.Nil(t, ChatFrom(ctx))

	chat := &models.Chat{ID: 1, Locale: "en"}
	ctx = WithChat(ctx, chat)
	got := ChatFrom(ctx)
	require.NotNil(t, got)
	assert.Equal(t, int64(1), got.ID)
	assert.Equal(t, "en", got.Locale)
}

func TestParticipationFrom(t *testing.T) {
	ctx := context.Background()
	assert.Nil(t, ParticipationFrom(ctx))

	p := &models.Participation{ID: 7, ChatID: 1, UserID: 2}
	ctx = WithParticipation(ctx, p)
	got := ParticipationFrom(ctx)
	require.NotNil(t, got)
	assert.Equal(t, int64(7), got.ID)
}

func TestSkipMessageHistory(t *testing.T) {
	ctx := context.Background()
	assert.False(t, SkipMessageHistory(ctx))

	ctx = WithSkipMessageHistory(ctx)
	assert.True(t, SkipMessageHistory(ctx))
}

func TestLocale(t *testing.T) {
	loadTestConfig(t, `
telegram:
  token: "123:ABC"
app:
  locale: en
`)

	ctx := context.Background()
	assert.Equal(t, "en", Locale(ctx))

	chat := &models.Chat{Locale: "ru"}
	ctx = WithChat(ctx, chat)
	assert.Equal(t, "ru", Locale(ctx))
}

func TestLocaleDefaultRu(t *testing.T) {
	loadTestConfig(t, `
telegram:
  token: "123:ABC"
`)

	ctx := context.Background()
	assert.Equal(t, "ru", Locale(ctx))
}

func TestEnabled(t *testing.T) {
	loadTestConfig(t, `
telegram:
  token: "123:ABC"
app:
  allow_to_all: true
`)
	ctx := context.Background()
	assert.True(t, Enabled(ctx))

	loadTestConfig(t, fmt.Sprintf(`
telegram:
  token: "123:ABC"
app:
  allow_to_all: false
`))

	active := &models.Chat{ActiveAt: sql.NullTime{Time: time.Now(), Valid: true}}
	ctx = WithChat(context.Background(), active)
	assert.True(t, Enabled(ctx))

	inactive := &models.Chat{}
	ctx = WithChat(context.Background(), inactive)
	assert.False(t, Enabled(ctx))
}
