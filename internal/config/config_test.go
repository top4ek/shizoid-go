package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
app_env: development
log_level: debug
database:
  host: dbhost
  port: "5433"
  name: testdb
  user: testuser
  password: secret
telegram:
  token: "123:ABC"
  webhook_url: ""
app:
  bot_owners:
    - 42
    - 99
  bind_to: 8080
  generation_mode: simplified
neural:
  reply:
    - name: local
      base_url: http://llama:3110/v1
      model: test.gguf
      context_size: 8192
      timeout_seconds: 10
    - name: cloud
      base_url: https://api.example/v1
      model: big.gguf
      context_size: 32000
  summary:
    - name: summary-local
      base_url: http://llama:3110/v1
      model: test.gguf
      context_size: 4096
`), 0o600))

	require.NoError(t, Load(path))

	assert.Equal(t, "dbhost", Database.Host)
	assert.Equal(t, "5433", Database.Port)
	assert.Equal(t, "123:ABC", Telegram.Token)
	assert.Equal(t, []int64{42, 99}, Environment.BotOwners)
	assert.Equal(t, int16(8080), Environment.BindTo)
	require.Len(t, Neural.Reply, 2)
	assert.Equal(t, "local", Neural.Reply[0].Name)
	assert.Equal(t, 32000, MaxReplyContextBytes)
	assert.Equal(t, 4096, MaxSummaryContextBytes)
	assert.True(t, Development())
	assert.Equal(t, "debug", Runtime.AppLogLevel)
}

func TestLoadTopLevelRuntimeKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
app:
  allow_to_all: false
  bind_to: 8095
  bot_owners:
    - 808
  generation_mode: classic
  idle_cron: "40 16 * * *"
  locale: ru
  memory_cron: "0 */3 * * *"
  winner_cron: "20 1 * * *"
app_env: production
log_level: debug
database:
  host: postgresql
  name: shizoid
  password: secret
  port: "5432"
  user: shizoid
telegram:
  token: "123:ABC"
  webhook_url: ""
`), 0o600))

	require.NoError(t, Load(path))

	assert.Equal(t, "production", Runtime.AppEnv)
	assert.Equal(t, "debug", Runtime.AppLogLevel)
	assert.False(t, Development())
	assert.Equal(t, "postgresql", Database.Host)
	assert.Equal(t, int16(8095), Environment.BindTo)
}

func TestLoadAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
telegram:
  token: "123:ABC"
`), 0o600))

	require.NoError(t, Load(path))

	assert.Equal(t, "database", Database.Host)
	assert.Equal(t, "5432", Database.Port)
	assert.Equal(t, "shizoid", Database.Name)
	assert.Equal(t, "shizoid", Database.User)
	assert.Equal(t, int16(3000), Environment.BindTo)
	assert.Equal(t, "ru", Environment.Locale)
	assert.Equal(t, "classic", Environment.GenerationMode)
	assert.Equal(t, "production", Runtime.AppEnv)
	assert.Equal(t, "production", Sentry.Environment)
	assert.NotEmpty(t, Environment.AppPrompt)
	assert.NotEmpty(t, Environment.SummaryPrompt)
	assert.NotEmpty(t, Environment.IdlePrompt)
}

func TestLoadRequiresToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
telegram:
  token: ""
`), 0o600))

	err := Load(path)
	require.Error(t, err)
	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "telegram.token", ve.Field)
}

func TestLoadWebhookRequiresURL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
telegram:
  token: "123:ABC"
  webhook_url: "https://example.com/hook"
`), 0o600))

	// webhook_url set but empty would be poll mode; non-empty without full URL is valid
	require.NoError(t, Load(path))
	assert.False(t, Telegram.PollMode())
}
