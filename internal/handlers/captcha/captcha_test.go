package captcha

import (
	"os"
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"

	"shizoid/internal/locale"
	"shizoid/internal/logger"
)

func TestMain(m *testing.M) {
	logger.Init(true, "")
	os.Exit(m.Run())
}

func TestBuildChallenge(t *testing.T) {
	correct, buttons, err := buildChallenge("en")
	if err != nil {
		t.Fatal(err)
	}
	if len(buttons) != 4 {
		t.Fatalf("buttons: got %d, want 4", len(buttons))
	}
	seen := make(map[string]bool)
	found := false
	for _, b := range buttons {
		if b.Emoji == "" || b.Word == "" {
			t.Fatalf("empty symbol: %+v", b)
		}
		if seen[b.Emoji] {
			t.Fatalf("duplicate emoji: %s", b.Emoji)
		}
		seen[b.Emoji] = true
		if b.Emoji == correct.Emoji {
			found = true
		}
	}
	if !found {
		t.Fatalf("correct emoji %q not in buttons", correct.Emoji)
	}
}

func TestBuildChallengeNotEnoughSymbols(t *testing.T) {
	_, _, err := buildChallenge("missing")
	if err == nil {
		t.Fatal("expected error for missing locale")
	}
}

func TestParseCallback(t *testing.T) {
	id, emoji, ok := parseCallback("captcha:123456789:🔒")
	if !ok || id != 123456789 || emoji != "🔒" {
		t.Fatalf("parseCallback: ok=%v id=%d emoji=%q", ok, id, emoji)
	}
	_, _, ok = parseCallback("captcha:bad")
	if ok {
		t.Fatal("expected parse failure")
	}
}

func TestCallbackDataRoundTrip(t *testing.T) {
	data := callbackData(42, "🌹")
	id, emoji, ok := parseCallback(data)
	if !ok || id != 42 || emoji != "🌹" {
		t.Fatalf("round trip: ok=%v id=%d emoji=%q", ok, id, emoji)
	}
}

func TestFormatUserLink_WithUsername(t *testing.T) {
	got := formatUserLink(models.User{ID: 42, Username: "alice", FirstName: "Alice"})
	assert.Contains(t, got, "https://t.me/alice")
}

func TestFormatUserLink_WithoutUsername(t *testing.T) {
	got := formatUserLink(models.User{ID: 42, FirstName: "Bob"})
	assert.Contains(t, got, "tg://user?id=42")
}

func TestCaptchaMessage_ContainsUserLink(t *testing.T) {
	member := models.User{ID: 99, FirstName: "Test"}
	msg := locale.T("en", "captcha.message",
		"user", formatUserLink(member),
		"word", "rose",
	)
	assert.True(t, strings.Contains(msg, "tg://user?id=99"))
}

func TestLocaleSymbols(t *testing.T) {
	symbols := locale.Symbols("en", "captcha.symbols")
	if len(symbols) < 4 {
		t.Fatalf("en captcha.symbols: got %d, want >= 4", len(symbols))
	}
	for _, s := range symbols {
		if s.Emoji == "" || s.Word == "" {
			t.Fatalf("empty pair: %+v", s)
		}
	}
}
