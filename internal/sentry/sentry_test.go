package sentry

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"shizoid/internal/config"
	"shizoid/internal/logger"
)

func TestMain(m *testing.M) {
	logger.Init(true, "error")
	os.Exit(m.Run())
}

func loadTestConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
telegram:
  token: "123:ABC"
`), 0o600))
	require.NoError(t, config.Load(path))
}

func TestCaptureNil(t *testing.T) {
	loadTestConfig(t)
	Capture(nil)
}

func TestCaptureDisabled(t *testing.T) {
	loadTestConfig(t)
	assert.False(t, config.SentryEnabled())
	Capture(assert.AnError)
}

func TestInitFlushDisabled(t *testing.T) {
	loadTestConfig(t)
	Init()
	Flush()
}

func TestRecover(t *testing.T) {
	loadTestConfig(t)

	called := false
	handler := Recover(func(ctx context.Context, b *bot.Bot, update *models.Update) {
		called = true
	})
	handler(context.Background(), nil, &models.Update{})

	assert.True(t, called)

	assert.NotPanics(t, func() {
		panicHandler := Recover(func(ctx context.Context, b *bot.Bot, update *models.Update) {
			panic("test panic")
		})
		panicHandler(context.Background(), nil, &models.Update{})
	})
}
