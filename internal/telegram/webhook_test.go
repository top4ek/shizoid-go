package telegram

import (
	"context"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"shizoid/internal/config"
	"shizoid/internal/logger"
)

const testToken = "123:ABC"

const webhookInfoEmpty = `{"ok":true,"result":{"url":"","has_custom_certificate":false,"pending_update_count":0}}`
const webhookInfoSet = `{"ok":true,"result":{"url":"https://example.com/hook","has_custom_certificate":false,"pending_update_count":0}}`
const apiOKTrue = `{"ok":true,"result":true}`

func TestMain(m *testing.M) {
	logger.Init(true, "error")
	os.Exit(m.Run())
}

func loadTelegramConfig(t *testing.T, yamlBody string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(yamlBody), 0o600))
	require.NoError(t, config.Load(path))
}

func parseMultipartForm(t *testing.T, req *http.Request) map[string]string {
	t.Helper()
	mediaType, _, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	require.NoError(t, err)
	require.Equal(t, "multipart/form-data", mediaType)
	require.NoError(t, req.ParseMultipartForm(1<<20))
	values := make(map[string]string, len(req.MultipartForm.Value))
	for key, vals := range req.MultipartForm.Value {
		if len(vals) > 0 {
			values[key] = vals[0]
		}
	}
	return values
}

func newMockBot(t *testing.T, handler http.HandlerFunc) *bot.Bot {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	b, err := bot.New(testToken, bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	require.NoError(t, err)
	return b
}

func TestConfigureDeliveryPollModeNoWebhook(t *testing.T) {
	loadTelegramConfig(t, `
telegram:
  token: "123:ABC"
  webhook_url: ""
`)

	var methods []string
	b := newMockBot(t, func(rw http.ResponseWriter, req *http.Request) {
		methods = append(methods, req.URL.Path)
		body := apiOKTrue
		if req.URL.Path == "/bot"+testToken+"/getWebhookInfo" {
			body = webhookInfoEmpty
		}
		_, err := rw.Write([]byte(body))
		require.NoError(t, err)
	})

	require.NoError(t, ConfigureDelivery(context.Background(), b))
	require.Equal(t, []string{"/bot" + testToken + "/getWebhookInfo"}, methods)
}

func TestConfigureDeliveryPollModeClearsWebhook(t *testing.T) {
	loadTelegramConfig(t, `
telegram:
  token: "123:ABC"
  webhook_url: ""
`)

	var methods []string
	b := newMockBot(t, func(rw http.ResponseWriter, req *http.Request) {
		methods = append(methods, req.URL.Path)
		var body string
		switch req.URL.Path {
		case "/bot" + testToken + "/getWebhookInfo":
			body = webhookInfoSet
		default:
			body = apiOKTrue
		}
		_, err := rw.Write([]byte(body))
		require.NoError(t, err)
	})

	require.NoError(t, ConfigureDelivery(context.Background(), b))
	assert.Equal(t, []string{
		"/bot" + testToken + "/getWebhookInfo",
		"/bot" + testToken + "/deleteWebhook",
	}, methods)
}

func TestConfigureDeliveryPollModeDeleteFailsStillOK(t *testing.T) {
	loadTelegramConfig(t, `
telegram:
  token: "123:ABC"
  webhook_url: ""
`)

	b := newMockBot(t, func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/bot"+testToken+"/getWebhookInfo" {
			_, err := rw.Write([]byte(webhookInfoSet))
			require.NoError(t, err)
			return
		}
		// empty body simulates network/proxy failure
	})

	require.NoError(t, ConfigureDelivery(context.Background(), b))
}

func TestConfigureDeliveryWebhookMode(t *testing.T) {
	loadTelegramConfig(t, `
telegram:
  token: "123:ABC"
  webhook_url: "https://example.com/hook"
  webhook_secret_token: "secret-42"
`)

	var gotMethod string
	var gotForm map[string]string
	b := newMockBot(t, func(rw http.ResponseWriter, req *http.Request) {
		gotMethod = req.URL.Path
		gotForm = parseMultipartForm(t, req)
		_, err := rw.Write([]byte(apiOKTrue))
		require.NoError(t, err)
	})

	require.NoError(t, ConfigureDelivery(context.Background(), b))
	assert.Equal(t, "/bot"+testToken+"/setWebhook", gotMethod)
	assert.Equal(t, "https://example.com/hook", gotForm["url"])
	assert.Equal(t, "secret-42", gotForm["secret_token"])
	assert.Contains(t, gotForm["allowed_updates"], "chat_member")
	assert.Contains(t, gotForm["allowed_updates"], "message")
}

func TestConfigureDeliveryWebhookWithoutSecret(t *testing.T) {
	loadTelegramConfig(t, `
telegram:
  token: "123:ABC"
  webhook_url: "https://example.com/hook"
`)

	var gotForm map[string]string
	b := newMockBot(t, func(rw http.ResponseWriter, req *http.Request) {
		gotForm = parseMultipartForm(t, req)
		_, err := rw.Write([]byte(apiOKTrue))
		require.NoError(t, err)
	})

	require.NoError(t, ConfigureDelivery(context.Background(), b))
	assert.Equal(t, "https://example.com/hook", gotForm["url"])
	assert.NotEmpty(t, gotForm["secret_token"])
	assert.Equal(t, gotForm["secret_token"], config.Telegram.WebhookSecretToken)
}

func TestEnsureWebhookSecretPollMode(t *testing.T) {
	loadTelegramConfig(t, `
telegram:
  token: "123:ABC"
  webhook_url: ""
`)

	require.NoError(t, EnsureWebhookSecret())
	assert.Empty(t, config.Telegram.WebhookSecretToken)
}

func TestEnsureWebhookSecretGenerates(t *testing.T) {
	loadTelegramConfig(t, `
telegram:
  token: "123:ABC"
  webhook_url: "https://example.com/hook"
`)

	require.NoError(t, EnsureWebhookSecret())
	assert.NotEmpty(t, config.Telegram.WebhookSecretToken)

	first := config.Telegram.WebhookSecretToken
	require.NoError(t, EnsureWebhookSecret())
	assert.Equal(t, first, config.Telegram.WebhookSecretToken)
}

func TestEnsureWebhookSecretPreservesConfigured(t *testing.T) {
	loadTelegramConfig(t, `
telegram:
  token: "123:ABC"
  webhook_url: "https://example.com/hook"
  webhook_secret_token: "secret-42"
`)

	require.NoError(t, EnsureWebhookSecret())
	assert.Equal(t, "secret-42", config.Telegram.WebhookSecretToken)
}
