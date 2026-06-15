package scheduler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/go-telegram/bot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"shizoid/internal/config"
	"shizoid/internal/locale"
	"shizoid/internal/logger"
)

const testToken = "123:ABC"

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

func TestWinnerLabel(t *testing.T) {
	assert.Equal(t, locale.T("ru", "winner.default"), winnerLabel("", "ru"))
	assert.Equal(t, "daily", winnerLabel("daily", "ru"))
}

func TestSummaryMessageBudget(t *testing.T) {
	config.Environment.SummaryPrompt = "prompt"
	config.MaxSummaryContextBytes = 10000

	budget := summaryMessageBudget("")
	assert.Greater(t, budget, 0)

	withMemory := summaryMessageBudget("existing memory text")
	assert.Less(t, withMemory, budget)

	config.MaxSummaryContextBytes = 10
	assert.LessOrEqual(t, summaryMessageBudget("long existing memory"), 0)
}

func TestTruncateRunes(t *testing.T) {
	short := "hello"
	assert.Equal(t, short, truncateRunes(short, 10))

	long := strings.Repeat("ж", 20)
	got := truncateRunes(long, 5)
	assert.Equal(t, 5, utf8.RuneCountInString(got))
}

func TestStart(t *testing.T) {
	loadTestConfig(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	}))
	t.Cleanup(server.Close)

	b, err := bot.New(testToken, bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	require.NoError(t, err)

	c := Start(b)
	require.NotNil(t, c)
	c.Stop()
}