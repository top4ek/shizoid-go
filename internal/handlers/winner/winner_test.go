package winner

import (
	"strings"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/stretchr/testify/assert"

	"shizoid/internal/locale"
	"shizoid/internal/models"
)

func TestFormatTop(t *testing.T) {
	entries := []models.ScoreEntry{
		{UserID: 1, Name: "alice", Score: 10},
		{UserID: 2, Name: "", Score: 5},
	}
	out := FormatTop("ru", entries)
	lines := strings.Split(out, "\n")

	assert.Len(t, lines, 2)
	assert.Contains(t, lines[0], "alice")
	assert.Contains(t, lines[0], "10")
	assert.Contains(t, lines[0], `*1\.`)
	assert.Contains(t, lines[1], "Флудер")
}

func TestFormatTop_EscapesMarkdownInName(t *testing.T) {
	out := FormatTop("en", []models.ScoreEntry{{UserID: 1, Name: "*name*", Score: 1}})
	assert.Contains(t, out, `\*name\*`)
	assert.NotContains(t, out, "*name*")
}

func TestFormatTopEmpty(t *testing.T) {
	assert.Equal(t, "", FormatTop("ru", nil))
}

func TestFormatWinnerUser_WithUsername(t *testing.T) {
	got := FormatWinnerUser("en", 1, "alice", "alice")
	assert.Equal(t, "[alice](https://t.me/alice)", got)
}

func TestMarkdownPlain_EscapesLocaleString(t *testing.T) {
	got := markdownPlain(locale.T("ru", "winner.no_one"))
	assert.Contains(t, got, `:\(`)
	assert.NotContains(t, got, ":(")
}

func TestFormatWinnerUser_EmptyNameUsesDefault(t *testing.T) {
	for _, lang := range []string{"en", "ru"} {
		got := FormatWinnerUser(lang, 42, "", "")
		assert.Contains(t, got, "tg://user?id=42")
		assert.Contains(t, got, bot.EscapeMarkdown(locale.T(lang, "winner.default")))
	}
}
