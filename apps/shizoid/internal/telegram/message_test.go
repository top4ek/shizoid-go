package telegram

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/go-telegram/bot"
	"github.com/stretchr/testify/assert"
)

func TestFitV2HonoursTheGivenBudget(t *testing.T) {
	got := FitV2(strings.Repeat(".", 100), 10)
	assert.LessOrEqual(t, utf8.RuneCountInString(got), 10)
	assert.NotEmpty(t, got)
	assert.NoError(t, ValidateV2(got))

	// a budget the caller has already spent leaves room for nothing
	assert.Empty(t, FitV2("церемония", 0))
	assert.Empty(t, FitV2("церемония", -5))
}

func TestIsPermanentErrorSeparatesTheAnswersRetryingCannotChange(t *testing.T) {
	assert.False(t, IsPermanentError(nil))
	assert.True(t, IsPermanentError(fmt.Errorf("%w, bot was kicked from the group chat", bot.ErrorForbidden)))
	assert.True(t, IsPermanentError(fmt.Errorf("%w, chat not found", bot.ErrorBadRequest)))
	assert.True(t, IsPermanentError(&bot.MigrateError{MigrateToChatID: 42}))

	assert.False(t, IsPermanentError(&bot.TooManyRequestsError{RetryAfter: 5}))
	assert.False(t, IsPermanentError(errors.New("dial tcp: i/o timeout")))
}

func TestPrepareOutboundText_ShortPassesThrough(t *testing.T) {
	got := prepareOutboundText("hello *bold* world")
	assert.NoError(t, ValidateV2(got))
	assert.Contains(t, got, "*bold*")
}

func TestPrepareOutboundText_TruncatesLongTextKeepingValidMarkdown(t *testing.T) {
	long := strings.Repeat("word ", MaxMessageRunes/5) + "*bold entity spanning the cut boundary*"
	got := prepareOutboundText(long)
	assert.LessOrEqual(t, utf8.RuneCountInString(got), MaxMessageRunes)
	assert.NoError(t, ValidateV2(got))
	assert.NotEmpty(t, got)
}

func TestPrepareOutboundText_EscapingGrowthConverges(t *testing.T) {
	// Every rune escapes to two runes; naive truncation would still exceed the limit.
	long := strings.Repeat(".", MaxMessageRunes+10)
	got := prepareOutboundText(long)
	assert.LessOrEqual(t, utf8.RuneCountInString(got), MaxMessageRunes)
	assert.NoError(t, ValidateV2(got))
	assert.NotEmpty(t, got)
}

func TestPrepareOutboundText_EntityExactlyAtBoundary(t *testing.T) {
	pad := strings.Repeat("a", MaxMessageRunes-5)
	long := pad + " *bold text far beyond the limit*"
	got := prepareOutboundText(long)
	assert.LessOrEqual(t, utf8.RuneCountInString(got), MaxMessageRunes)
	assert.NoError(t, ValidateV2(got))
}
