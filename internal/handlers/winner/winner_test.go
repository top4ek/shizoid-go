package winner

import (
	"strings"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/stretchr/testify/assert"

	"shizoid/internal/locale"
	"shizoid/internal/models"
	"shizoid/internal/telegram"
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
	got := telegram.FormatPlain(locale.T("ru", "winner.no_one"))
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

// A chat that has never had a draw would otherwise get a bare "top of the year"
// header with no rows under it.
func TestYearBlockIsEmptyWithoutWinners(t *testing.T) {
	assert.Equal(t, "", yearBlock("ru", nil))
	assert.Equal(t, "head", joinBlock("head", yearBlock("ru", nil)))

	block := yearBlock("ru", []models.ScoreEntry{{UserID: 1, Name: "alice", Score: 2}})
	assert.Contains(t, block, "alice")
	assert.Equal(t, "head\n\n"+block, joinBlock("head", block))
}

// Without a model the announcement is only the result line, so the separator
// must not be prepended to it.
func TestJoinBlockSkipsEmptySides(t *testing.T) {
	assert.Equal(t, "result", joinBlock("", "result"))
	assert.Equal(t, "ceremony", joinBlock("ceremony", ""))
	assert.Equal(t, "", joinBlock("", ""))
}
