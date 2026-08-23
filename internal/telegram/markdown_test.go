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

func TestSanitizeV2_KeepsAnExpandableQuote(t *testing.T) {
	quote := "**>первая строка\n>вторая строка||"

	got := SanitizeV2(quote)

	assert.Equal(t, quote, got)
	assert.NoError(t, ValidateV2(got))
}

func TestSanitizeV2_KeepsAPlainQuote(t *testing.T) {
	got := SanitizeV2(">одна строка\n>вторая")

	assert.Equal(t, ">одна строка\n>вторая", got)
	assert.NoError(t, ValidateV2(got))
}

// The quote is markup, its body is not: what the model wrote inside still has
// to be escaped.
func TestSanitizeV2_EscapesInsideAQuote(t *testing.T) {
	got := SanitizeV2("**>вася набрал 5 * 5 очков!||")

	assert.Equal(t, `**>вася набрал 5 \* 5 очков\!||`, got)
	assert.NoError(t, ValidateV2(got))
}

// A cut through the quote takes its closing mark with it. What is left is still
// a quote, not a wall of literal '>' characters.
func TestSanitizeV2_UnclosedExpandableQuoteStaysAQuote(t *testing.T) {
	got := SanitizeV2("**>первая строка\n>вторая строка")

	assert.Equal(t, ">первая строка\n>вторая строка", got)
	assert.NoError(t, ValidateV2(got))
}

func TestSanitizeV2_QuoteEndsAtTheFirstPlainLine(t *testing.T) {
	got := SanitizeV2("**>церемония||\n\nПиздабол дня")

	assert.Equal(t, "**>церемония||\n\nПиздабол дня", got)
	assert.NoError(t, ValidateV2(got))
}

func TestSanitizeV2_EscapesAQuoteMarkerInsideALine(t *testing.T) {
	got := SanitizeV2("5 > 3")

	assert.Equal(t, `5 \> 3`, got)
	assert.NoError(t, ValidateV2(got))
}

func TestValidateV2_RejectsAQuoteMarkerInsideALine(t *testing.T) {
	assert.NoError(t, ValidateV2(">quote"))
	assert.Error(t, ValidateV2("5 > 3"))
}

func TestQuoteExpandableV2_WrapsEveryLine(t *testing.T) {
	got := QuoteExpandableV2("первая\nвторая")

	assert.Equal(t, "**>первая\n>вторая||", got)
	assert.NoError(t, ValidateV2(got))
	assert.Equal(t, got, SanitizeV2(got), "the send path must leave the quote alone")
}

func TestQuoteExpandableV2_NothingToQuote(t *testing.T) {
	assert.Empty(t, QuoteExpandableV2(""))
	assert.Empty(t, QuoteExpandableV2("  \n "))
}

// A body ending in an escaped pipe must not have its escape swallowed by the
// expandability mark that follows it.
func TestQuoteExpandableV2_BodyEndingInAnEscapedPipe(t *testing.T) {
	got := QuoteExpandableV2(SanitizeV2("вася |"))

	assert.Equal(t, `**>вася \|||`, got)
	assert.NoError(t, ValidateV2(got))
	assert.Equal(t, got, SanitizeV2(got))
}
