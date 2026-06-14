package neural

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"shizoid/internal/logger"
)

func init() {
	logger.Init(true, "error")
}

func TestNewEmptyChains(t *testing.T) {
	c := New(nil, nil)
	assert.False(t, c.ReplyConfigured())
}

func TestNewWithProviders(t *testing.T) {
	c := New([]Provider{{Name: "local"}}, nil)
	assert.True(t, c.ReplyConfigured())
}

func TestReplyChainFallsBackToNextProvider(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hi there"}}]}`))
	}))
	defer good.Close()

	c := &Client{
		http: http.DefaultClient,
		reply: []Provider{
			{Name: "bad", BaseURL: bad.URL + "/v1", Model: "m", TimeoutSeconds: 5},
			{Name: "good", BaseURL: good.URL + "/v1", Model: "m", TimeoutSeconds: 5},
		},
	}
	out, err := c.Reply(context.Background(), "sys", "hello")
	require.NoError(t, err)
	assert.Equal(t, "hi there", out)
}

func TestReplyAllProvidersFail(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer bad.Close()

	c := &Client{
		http:  http.DefaultClient,
		reply: []Provider{{Name: "bad", BaseURL: bad.URL + "/v1", Model: "m", TimeoutSeconds: 5}},
	}
	_, err := c.Reply(context.Background(), "", "hello")
	assert.Error(t, err)
}

func TestReplyConfigured(t *testing.T) {
	var nilClient *Client
	assert.False(t, nilClient.ReplyConfigured())
	assert.False(t, (&Client{}).ReplyConfigured())
	assert.True(t, (&Client{reply: []Provider{{}}}).ReplyConfigured())
}

func TestServerRoot(t *testing.T) {
	assert.Equal(t, "http://llama:8080", serverRoot("http://llama:8080/v1"))
	assert.Equal(t, "http://llama:8080", serverRoot("http://llama:8080/v1/"))
	assert.Equal(t, "https://api.example.com", serverRoot("https://api.example.com"))
}

func TestReplyBuildsStructuredMessages(t *testing.T) {
	var got chatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &got))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	c := &Client{
		http:  http.DefaultClient,
		reply: []Provider{{Name: "local", BaseURL: srv.URL + "/v1", Model: "m", TimeoutSeconds: 5}},
	}
	_, err := c.Reply(context.Background(), "sys prompt", "hello")
	require.NoError(t, err)

	require.Len(t, got.Messages, 2)
	assert.Equal(t, "system", got.Messages[0].Role)
	assert.Equal(t, "user", got.Messages[1].Role)
	require.Len(t, got.Messages[0].Content, 1)
	assert.Equal(t, "text", got.Messages[0].Content[0].Type)
	assert.Equal(t, "sys prompt", got.Messages[0].Content[0].Text)
	assert.Equal(t, "hello", got.Messages[1].Content[0].Text)
	require.NotNil(t, got.ChatTemplateKwargs)
	assert.Equal(t, false, got.ChatTemplateKwargs["enable_thinking"])
	assert.Nil(t, got.Temperature)
}

func TestCallAppliesSamplingParams(t *testing.T) {
	var got chatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &got))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	c := &Client{
		http: http.DefaultClient,
		reply: []Provider{{
			Name:           "local",
			BaseURL:        srv.URL + "/v1",
			Model:          "m",
			TimeoutSeconds: 5,
			Sampling: &SamplingParams{
				Temperature:       0.7,
				TopP:              0.8,
				TopK:              20,
				MinP:              0.0,
				PresencePenalty:   1.5,
				RepetitionPenalty: 1.0,
			},
		}},
	}
	_, err := c.Reply(context.Background(), "sys", "hello")
	require.NoError(t, err)

	require.NotNil(t, got.Temperature)
	assert.Equal(t, 0.7, *got.Temperature)
	require.NotNil(t, got.TopP)
	assert.Equal(t, 0.8, *got.TopP)
	require.NotNil(t, got.TopK)
	assert.Equal(t, 20, *got.TopK)
	require.NotNil(t, got.MinP)
	assert.Equal(t, 0.0, *got.MinP)
	require.NotNil(t, got.PresencePenalty)
	assert.Equal(t, 1.5, *got.PresencePenalty)
	require.NotNil(t, got.RepeatPenalty)
	assert.Equal(t, 1.0, *got.RepeatPenalty)
}

func TestReplyWithHistoryIncludesName(t *testing.T) {
	var got chatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &got))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	c := &Client{
		http:  http.DefaultClient,
		reply: []Provider{{Name: "local", BaseURL: srv.URL + "/v1", Model: "m", TimeoutSeconds: 5}},
	}
	history := []HistoryMessage{
		{Role: "user", Name: "111", Text: "привет"},
		{Role: "assistant", Text: "здравствуй"},
		{Role: "user", Name: "222", Text: "как дела?"},
	}
	_, err := c.ReplyWithHistory(context.Background(), "sys", history)
	require.NoError(t, err)

	require.Len(t, got.Messages, 4)
	assert.Equal(t, "system", got.Messages[0].Role)
	assert.Equal(t, "111", got.Messages[1].Name)
	assert.Equal(t, "user", got.Messages[1].Role)
	assert.Equal(t, "привет", got.Messages[1].Content[0].Text)
	assert.Equal(t, "assistant", got.Messages[2].Role)
	assert.Empty(t, got.Messages[2].Name)
	assert.Equal(t, "222", got.Messages[3].Name)
}

func TestMessageText(t *testing.T) {
	assert.Equal(t, "plain", messageText(json.RawMessage(`"plain"`)))
	assert.Equal(t, "one\ntwo", messageText(json.RawMessage(`[{"type":"text","text":"one"},{"type":"text","text":"two"}]`)))
}

func TestStripThinking(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty redacted block",
			in:   "<think>\n\n</think>\n\nПривет!",
			want: "Привет!",
		},
		{
			name: "block with reasoning",
			in:   "<" + "think" + ">" + "\nreasoning\n\n" + "</" + "think" + ">" + "\n\nОтвет",
			want: "Ответ",
		},
		{
			name: "think tags",
			in:   "<" + "thinking" + ">" + "\ninternal\n\n" + "</" + "thinking" + ">" + "\n\nHello",
			want: "Hello",
		},
		{
			name: "no tags",
			in:   "чистый текст",
			want: "чистый текст",
		},
		{
			name: "empty input",
			in:   "",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, stripThinking(tt.in))
		})
	}
}

func TestReplyStripsThinkingTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"<think>\n\n</think>\n\nHi there!"}}]}`))
	}))
	defer srv.Close()

	c := &Client{
		http:  http.DefaultClient,
		reply: []Provider{{Name: "local", BaseURL: srv.URL + "/v1", Model: "m", TimeoutSeconds: 5}},
	}
	out, err := c.Reply(context.Background(), "", "hello")
	require.NoError(t, err)
	assert.Equal(t, "Hi there!", out)
}

func TestReplySkipsProviderWhenNoSlots(t *testing.T) {
	busy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			t.Fatal("chat/completions should not be called when slots are full")
		}
	}))
	defer busy.Close()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"classic avoided queue"}}]}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer good.Close()

	c := &Client{
		http: http.DefaultClient,
		reply: []Provider{
			{Name: "busy", BaseURL: busy.URL + "/v1", Model: "m", TimeoutSeconds: 5, SlotCheck: true},
			{Name: "good", BaseURL: good.URL + "/v1", Model: "m", TimeoutSeconds: 5, SlotCheck: true},
		},
	}
	out, err := c.Reply(context.Background(), "", "hello")
	require.NoError(t, err)
	assert.Equal(t, "classic avoided queue", out)
}

func TestReplySkipsProviderWhenDailyLimitExceeded(t *testing.T) {
	limitHit := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("chat/completions should not be called when daily limit is exceeded")
	}))
	defer limitHit.Close()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"fallback provider"}}]}`))
	}))
	defer good.Close()

	c := New([]Provider{
		{Name: "limited", BaseURL: limitHit.URL + "/v1", Model: "m", TimeoutSeconds: 5, DailyLimit: 1},
		{Name: "good", BaseURL: good.URL + "/v1", Model: "m", TimeoutSeconds: 5},
	}, nil)
	c.ledger.setCount("limited", c.ledger.today(), 1)

	out, err := c.Reply(context.Background(), "", "hello")
	require.NoError(t, err)
	assert.Equal(t, "fallback provider", out)
}

func TestDailyLimitSharedAcrossChains(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	provider := Provider{Name: "shared", BaseURL: srv.URL + "/v1", Model: "m", TimeoutSeconds: 5, DailyLimit: 1}
	c := New([]Provider{provider}, []Provider{provider})

	_, err := c.Reply(context.Background(), "", "hello")
	require.NoError(t, err)

	_, err = c.Summarize(context.Background(), "prompt", "", []string{"msg"})
	assert.Error(t, err)
}

func TestDailyLimitOmittedIsUnlimited(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	c := New([]Provider{
		{Name: "free", BaseURL: srv.URL + "/v1", Model: "m", TimeoutSeconds: 5},
	}, nil)

	for range 3 {
		_, err := c.Reply(context.Background(), "", "hello")
		require.NoError(t, err)
	}
	assert.Equal(t, 3, calls)
}

func TestTrimMessagesKeepsSystemAndLast(t *testing.T) {
	messages := []chatMessage{
		newMessage("system", "sys"),
		newHistoryMessage("user", "1", "old"),
		newHistoryMessage("assistant", "", "mid"),
		newHistoryMessage("user", "2", "current"),
	}
	got := trimMessages(messages, 1000)
	assert.Equal(t, messages, got)

	got = trimMessages(messages, len("sys")+len("mid")+len("current"))
	require.Len(t, got, 3)
	assert.Equal(t, "system", got[0].Role)
	assert.Equal(t, "mid", got[1].Content[0].Text)
	assert.Equal(t, "current", got[2].Content[0].Text)
}

func TestTrimMessagesNoLimit(t *testing.T) {
	messages := []chatMessage{newMessage("user", "hello")}
	assert.Equal(t, messages, trimMessages(messages, 0))
}

func TestReplyWithHistoryTrimsToProviderContextSize(t *testing.T) {
	var got chatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &got))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	c := &Client{
		http: http.DefaultClient,
		reply: []Provider{{
			Name: "local", BaseURL: srv.URL + "/v1", Model: "m",
			TimeoutSeconds: 5, ContextSize: len("sys") + len("current"),
		}},
	}
	history := []HistoryMessage{
		{Role: "user", Name: "1", Text: "drop-me"},
		{Role: "assistant", Text: "also-drop"},
		{Role: "user", Name: "2", Text: "current"},
	}
	_, err := c.ReplyWithHistory(context.Background(), "sys", history)
	require.NoError(t, err)

	require.Len(t, got.Messages, 2)
	assert.Equal(t, "system", got.Messages[0].Role)
	assert.Equal(t, "current", got.Messages[1].Content[0].Text)
}

func TestCallOrderLimitBeforeSlotCheck(t *testing.T) {
	var healthCalls int
	busy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			healthCalls++
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer busy.Close()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"next"}}]}`))
	}))
	defer good.Close()

	c := New([]Provider{
		{Name: "limited", BaseURL: busy.URL + "/v1", Model: "m", TimeoutSeconds: 5, DailyLimit: 1, SlotCheck: true},
		{Name: "good", BaseURL: good.URL + "/v1", Model: "m", TimeoutSeconds: 5},
	}, nil)
	c.ledger.setCount("limited", c.ledger.today(), 1)

	out, err := c.Reply(context.Background(), "", "hello")
	require.NoError(t, err)
	assert.Equal(t, "next", out)
	assert.Zero(t, healthCalls, "slot check must not run when daily limit already exceeded")
}
