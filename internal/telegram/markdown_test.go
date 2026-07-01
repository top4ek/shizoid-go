package telegram

import (
	"testing"

	"github.com/go-telegram/bot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateV2_PlainEscaped(t *testing.T) {
	assert.NoError(t, ValidateV2(`Hello\.`))
	assert.Error(t, ValidateV2(`Hello.`))
}

func TestSanitizeV2_Plain(t *testing.T) {
	got := SanitizeV2("Pong!")
	assert.Equal(t, `Pong\!`, got)
	assert.NoError(t, ValidateV2(got))
}

func TestSanitizeV2_PreservesBold(t *testing.T) {
	got := SanitizeV2("*Active:* yes")
	assert.NoError(t, ValidateV2(got))
	assert.Contains(t, got, "*Active:*")
}

func TestSanitizeV2_UserLink(t *testing.T) {
	link := "[Alice](tg://user?id=42)"
	got := SanitizeV2("Hi, " + link + "\\!")
	assert.NoError(t, ValidateV2(got))
	assert.Contains(t, got, link)
}

func TestSanitizeV2_BrokenBoldFallsBack(t *testing.T) {
	got := SanitizeV2("*unclosed")
	assert.NoError(t, ValidateV2(got))
}

func TestFormatTemplate(t *testing.T) {
	got := FormatTemplate("Hey %{user}, wait.")
	assert.Contains(t, got, "%{user}")
	assert.Contains(t, got, `wait\.`)
}

func TestFormatPlain(t *testing.T) {
	assert.Equal(t, bot.EscapeMarkdown("(private)"), FormatPlain("(private)"))
}

func TestSanitizeV2_StatusLike(t *testing.T) {
	yes := bot.EscapeMarkdown("yes")
	text := "*Active:* " + yes + "\n*Version:* " + bot.EscapeMarkdown("1.2.3")
	got := SanitizeV2(text)
	assert.NoError(t, ValidateV2(got))
}

func TestSanitizeV2_GabLevel(t *testing.T) {
	got := SanitizeV2("prefix *10%*\\.")
	assert.NoError(t, ValidateV2(got))
	assert.Contains(t, got, "*10%*")
}

func TestSanitizeV2_BoldWithSpecialCharsInBody(t *testing.T) {
	got := SanitizeV2("*Шиза!* и *что-то важное*")
	assert.NoError(t, ValidateV2(got))
	assert.Contains(t, got, `*Шиза\!*`)
	assert.Contains(t, got, `*что\-то важное*`)
}

func TestSanitizeV2_NeuralGarbage(t *testing.T) {
	cases := []string{
		"Hello (world)",
		"Price is 3.14",
		"*bold _broken",
		"[bad link](not closed",
	}
	for _, tc := range cases {
		got := SanitizeV2(tc)
		require.NoError(t, ValidateV2(got), "input: %q output: %q", tc, got)
	}
}
