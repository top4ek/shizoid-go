package telegram

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func TestPrepareOutboundText_ShortPassesThrough(t *testing.T) {
	got := prepareOutboundText("hello *bold* world")
	assert.NoError(t, ValidateV2(got))
	assert.Contains(t, got, "*bold*")
}

func TestPrepareOutboundText_TruncatesLongTextKeepingValidMarkdown(t *testing.T) {
	long := strings.Repeat("word ", maxMessageRunes/5) + "*bold entity spanning the cut boundary*"
	got := prepareOutboundText(long)
	assert.LessOrEqual(t, utf8.RuneCountInString(got), maxMessageRunes)
	assert.NoError(t, ValidateV2(got))
	assert.NotEmpty(t, got)
}

func TestPrepareOutboundText_EscapingGrowthConverges(t *testing.T) {
	// Every rune escapes to two runes; naive truncation would still exceed the limit.
	long := strings.Repeat(".", maxMessageRunes+10)
	got := prepareOutboundText(long)
	assert.LessOrEqual(t, utf8.RuneCountInString(got), maxMessageRunes)
	assert.NoError(t, ValidateV2(got))
	assert.NotEmpty(t, got)
}

func TestPrepareOutboundText_EntityExactlyAtBoundary(t *testing.T) {
	pad := strings.Repeat("a", maxMessageRunes-5)
	long := pad + " *bold text far beyond the limit*"
	got := prepareOutboundText(long)
	assert.LessOrEqual(t, utf8.RuneCountInString(got), maxMessageRunes)
	assert.NoError(t, ValidateV2(got))
}
