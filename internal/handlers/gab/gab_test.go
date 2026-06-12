package gab

import (
	"testing"

	"github.com/go-telegram/bot"
	"github.com/stretchr/testify/assert"

	"shizoid/internal/locale"
)

func TestGabLevelIsValidMarkdownV2(t *testing.T) {
	for _, lang := range []string{"en", "ru"} {
		got := levelText(lang, 10)
		assert.Contains(t, got, locale.T(lang, "gab.prefix"))
		assert.Contains(t, got, "*"+bot.EscapeMarkdown("10")+"%*")
		assert.Contains(t, got, `\.`, "trailing period must be escaped for MarkdownV2")
	}
}
