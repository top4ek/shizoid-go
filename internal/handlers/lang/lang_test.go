package lang

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"shizoid/internal/locale"
)

func TestCommandMetadata(t *testing.T) {
	assert.Equal(t, "lang", Command)
	assert.NotEmpty(t, Description)
}

func TestLangMessagesLocalized(t *testing.T) {
	current := locale.T("ru", "lang.current", "lang", "ru")
	assert.Contains(t, current, "ru")

	available := strings.Join(locale.Available(), ", ")
	unknown := locale.T("ru", "lang.unknown", "list", available)
	assert.Contains(t, unknown, available)

	set := locale.T("en", "lang.set", "lang", "en")
	assert.Contains(t, set, "en")
}

func TestLocaleAvailableMatchesHas(t *testing.T) {
	for _, code := range locale.Available() {
		assert.True(t, locale.Has(code), code)
	}
}
