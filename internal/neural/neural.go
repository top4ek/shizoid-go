// Package neural talks to OpenAI-compatible chat completion endpoints (the local
// llama.cpp server and optional external providers such as OpenRouter). It keeps
// two ordered fallback chains: one for replies and one for summarization, walking
// each chain until a provider answers.
package neural

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"

	"shizoid/internal/logger"
)

// ErrUnavailable means no provider in the chain produced a usable answer. Callers
// should fall back (e.g. to the classic generator).
var ErrUnavailable = errors.New("neural: no provider available")

type Client struct {
	reply   []Provider
	summary []Provider
	http    *http.Client
	ledger  *usageLedger
}

// New builds a Client from provider fallback chains. Nil or empty chains yield
// ErrUnavailable on every call so callers degrade gracefully.
func New(reply, summary []Provider) *Client {
	return &Client{
		reply:   reply,
		summary: summary,
		http: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:    10,
				IdleConnTimeout: 90 * time.Second,
			},
		},
		ledger: newUsageLedger(),
	}
}

// ReplyConfigured reports whether at least one reply provider is configured.
func (c *Client) ReplyConfigured() bool { return c != nil && len(c.reply) > 0 }

// HistoryMessage is one turn in a group-chat conversation sent to the model.
// Name carries the Telegram user_id for human messages (OpenAI "name" field).
type HistoryMessage struct {
	Role string
	Name string
	Text string
}

type contentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type chatMessage struct {
	Role    string        `json:"role"`
	Name    string        `json:"name,omitempty"`
	Content []contentPart `json:"content"`
}

// Reply builds a chat completion from a system prompt and the current user
// message, walking the reply chain until a provider answers.
func (c *Client) Reply(ctx context.Context, system, user string) (string, error) {
	history := []HistoryMessage{}
	if user = strings.TrimSpace(user); user != "" {
		history = append(history, HistoryMessage{Role: "user", Text: user})
	}
	return c.ReplyWithHistory(ctx, system, history)
}

// ReplyWithHistory builds a chat completion from a system prompt and ordered
// conversation history, walking the reply chain until a provider answers.
func (c *Client) ReplyWithHistory(ctx context.Context, system string, history []HistoryMessage) (string, error) {
	messages := make([]chatMessage, 0, 1+len(history))
	if system = strings.TrimSpace(system); system != "" {
		messages = append(messages, newMessage("system", system))
	}
	for _, h := range history {
		if text := strings.TrimSpace(h.Text); text != "" {
			messages = append(messages, newHistoryMessage(h.Role, h.Name, text))
		}
	}
	if len(messages) == 0 {
		return "", ErrUnavailable
	}
	return c.complete(ctx, c.reply, messages)
}

type chatRequest struct {
	Model              string         `json:"model"`
	Messages           []chatMessage  `json:"messages"`
	Stream             bool           `json:"stream"`
	ChatTemplateKwargs map[string]any `json:"chat_template_kwargs,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func messageText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	var parts []contentPart
	if err := json.Unmarshal(raw, &parts); err == nil {
		var b strings.Builder
		for _, p := range parts {
			if p.Type != "text" || p.Text == "" {
				continue
			}
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(p.Text)
		}
		return strings.TrimSpace(b.String())
	}
	return ""
}

var thinkingBlockRE = regexp.MustCompile(`(?s)<(?:redacted_)?think(?:ing)?>\s*.*?\s*</(?:redacted_)?think(?:ing)?>\s*`)

// stripThinking removes Qwen-style reasoning wrappers left in message content
// even when thinking is disabled on the server.
func stripThinking(s string) string {
	return strings.TrimSpace(thinkingBlockRE.ReplaceAllString(s, ""))
}

func newMessage(role, text string) chatMessage {
	return newHistoryMessage(role, "", text)
}

func messageBytes(m chatMessage) int {
	n := 0
	for _, p := range m.Content {
		n += len(p.Text)
	}
	return n
}

// trimMessages fits the payload into maxBytes. System prompt and the last
// history turn are always kept; older history is dropped from the front.
func trimMessages(messages []chatMessage, maxBytes int) []chatMessage {
	if maxBytes <= 0 || len(messages) == 0 {
		return messages
	}

	systemIdx := -1
	if messages[0].Role == "system" {
		systemIdx = 0
	}

	lastIdx := len(messages) - 1
	if lastIdx < 0 {
		return messages
	}

	kept := make([]bool, len(messages))
	total := 0
	if systemIdx >= 0 {
		kept[systemIdx] = true
		total += messageBytes(messages[systemIdx])
	}
	if lastIdx != systemIdx {
		kept[lastIdx] = true
		total += messageBytes(messages[lastIdx])
	}
	if total > maxBytes {
		return truncateMessageBudget(messages, kept, systemIdx, lastIdx, maxBytes)
	}

	for i := lastIdx - 1; i >= 0; i-- {
		if kept[i] {
			continue
		}
		b := messageBytes(messages[i])
		if total+b > maxBytes {
			continue
		}
		kept[i] = true
		total += b
	}

	out := make([]chatMessage, 0, len(messages))
	for i, m := range messages {
		if kept[i] {
			out = append(out, m)
		}
	}
	return out
}

func truncateMessageBudget(messages []chatMessage, kept []bool, systemIdx, lastIdx, maxBytes int) []chatMessage {
	out := make([]chatMessage, 0, 2)
	if systemIdx >= 0 && kept[systemIdx] {
		out = append(out, messages[systemIdx])
	}
	last := messages[lastIdx]
	excess := messageBytes(last)
	if systemIdx >= 0 && kept[systemIdx] {
		excess += messageBytes(messages[systemIdx])
	}
	excess -= maxBytes
	if excess > 0 && len(last.Content) > 0 && last.Content[0].Text != "" {
		text := last.Content[0].Text
		for excess > 0 && text != "" {
			runes := []rune(text)
			if len(runes) <= 1 {
				text = ""
				break
			}
			text = string(runes[:len(runes)-1])
			excess = messageBytes(newHistoryMessage(last.Role, last.Name, text))
			if systemIdx >= 0 && kept[systemIdx] {
				excess += messageBytes(messages[systemIdx])
			}
			excess -= maxBytes
		}
		last = newHistoryMessage(last.Role, last.Name, text)
	}
	out = append(out, last)
	return out
}

func newHistoryMessage(role, name, text string) chatMessage {
	m := chatMessage{
		Role:    role,
		Content: []contentPart{{Type: "text", Text: text}},
	}
	if name != "" {
		m.Name = name
	}
	return m
}

// Summarize condenses recent messages together with the existing memory using
// the summary chain. prompt carries the summarization instructions.
func (c *Client) Summarize(ctx context.Context, prompt, existing string, msgs []string) (string, error) {
	var b strings.Builder
	if existing != "" {
		b.WriteString("Existing memory:\n")
		b.WriteString(existing)
		b.WriteString("\n\n")
	}
	b.WriteString("New messages:\n")
	b.WriteString(strings.Join(msgs, "\n"))
	messages := []chatMessage{
		newMessage("system", prompt),
		newMessage("user", b.String()),
	}
	return c.complete(ctx, c.summary, messages)
}

func (c *Client) complete(ctx context.Context, chain []Provider, messages []chatMessage) (string, error) {
	if len(chain) == 0 {
		return "", ErrUnavailable
	}
	lastErr := ErrUnavailable
	for _, p := range chain {
		trimmed := trimMessages(messages, p.ContextSize)
		out, err := c.call(ctx, p, trimmed)
		if err != nil {
			logger.Instance().Debug("neural provider failed, trying next",
				zap.String("provider", p.Name),
				zap.Error(err),
			)
			lastErr = err
			continue
		}
		if out = strings.TrimSpace(out); out != "" {
			return out, nil
		}
	}
	return "", lastErr
}

func (c *Client) call(ctx context.Context, p Provider, messages []chatMessage) (string, error) {
	if c.ledger != nil && !c.ledger.reserve(p.Name, p.DailyLimit) {
		logger.Instance().Debug("neural daily limit exceeded",
			zap.String("provider", p.Name),
			zap.Int("limit", p.DailyLimit),
		)
		return "", fmt.Errorf("neural: %s: daily limit exceeded", p.Name)
	}
	if err := c.checkSlots(ctx, p); err != nil {
		return "", err
	}

	timeout := time.Duration(p.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	body, err := json.Marshal(chatRequest{
		Model:              p.Model,
		Messages:           messages,
		ChatTemplateKwargs: map[string]any{"enable_thinking": false},
	})
	if err != nil {
		return "", err
	}
	url := strings.TrimRight(p.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}

	log := logger.Instance()
	log.Info("neural request",
		zap.String("provider", p.Name),
		zap.String("model", p.Model),
		zap.String("url", url),
		zap.Int("messages_count", len(messages)),
	)
	log.Debug("neural request payload",
		zap.String("provider", p.Name),
		zap.String("body", string(body)),
	)
	start := time.Now()

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		log.Info("neural error",
			zap.String("provider", p.Name),
			zap.String("model", p.Model),
			zap.String("url", url),
			zap.Int("status", resp.StatusCode),
			zap.Duration("duration", time.Since(start)),
			zap.String("response_body", strings.TrimSpace(string(respBody))),
		)
		return "", fmt.Errorf("neural: %s returned status %d", p.Name, resp.StatusCode)
	}
	var parsed chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("neural: %s returned no choices", p.Name)
	}
	out := stripThinking(messageText(parsed.Choices[0].Message.Content))
	log.Info("neural response",
		zap.String("provider", p.Name),
		zap.String("model", p.Model),
		zap.String("url", url),
		zap.Int("status", resp.StatusCode),
		zap.Duration("duration", time.Since(start)),
		zap.Int("text_len", len(out)),
		zap.Int("text_runes", utf8.RuneCountInString(out)),
	)
	return out, nil
}

// checkSlots asks llama.cpp whether a free inference slot exists. When all slots
// are busy the server queues chat/completions; fail_on_no_slot makes /health
// return 503 immediately so we can fall back without waiting for that queue.
func (c *Client) checkSlots(ctx context.Context, p Provider) error {
	if !p.SlotCheck {
		return nil
	}
	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	url := serverRoot(p.BaseURL) + "/health?fail_on_no_slot=1"
	logger.Instance().Debug("neural slot check",
		zap.String("provider", p.Name),
		zap.String("url", url),
	)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusServiceUnavailable {
		_, _ = io.Copy(io.Discard, resp.Body)
		logger.Instance().Debug("neural no slots",
			zap.String("provider", p.Name),
			zap.String("url", url),
		)
		return fmt.Errorf("neural: %s: no slot available", p.Name)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return fmt.Errorf("neural: %s health returned status %d", p.Name, resp.StatusCode)
	}
	return nil
}

// serverRoot strips the OpenAI /v1 prefix from a provider base URL.
func serverRoot(baseURL string) string {
	base := strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(base, "/v1") {
		return strings.TrimSuffix(base, "/v1")
	}
	return base
}
