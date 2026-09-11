package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadYAML(t *testing.T, body string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	require.NoError(t, Load(path))
}

func TestPromptsAssembledFromDefaults(t *testing.T) {
	loadYAML(t, "telegram:\n  token: \"123:ABC\"\n")

	assert.NotEmpty(t, Environment.AppPrompt)
	assert.NotEmpty(t, Environment.WinnerPrompt)
	assert.NotEmpty(t, Environment.SummaryPrompt)

	// shared blocks reach every prompt that lists them
	for _, prompt := range []string{Environment.AppPrompt, Environment.WinnerPrompt} {
		assert.Contains(t, prompt, "[TELEGRAM MARKUP]")
		assert.Contains(t, prompt, "[RULE PRECEDENCE]")
	}
	assert.NotContains(t, Environment.SummaryPrompt, "[TELEGRAM MARKUP]")
}

// The reply prompt was tuned to stop small models emitting tables and lists.
// Splitting it into blocks must not touch those lines, and must not leak them
// into the winner announcement, which runs far longer than a chat reply.
func TestChatFormatRulesSurviveTheSplit(t *testing.T) {
	loadYAML(t, "telegram:\n  token: \"123:ABC\"\n")

	const noListMarker = "Never begin a line with a list marker or a number."
	const noTables = "Never produce tables, pipe-separated columns, headings, horizontal rules, LaTeX, HTML tags, footnotes, task lists or lists of any kind."

	assert.Contains(t, Environment.AppPrompt, noListMarker)
	assert.Contains(t, Environment.AppPrompt, noTables)
	assert.NotContains(t, Environment.WinnerPrompt, noListMarker)
	assert.NotContains(t, Environment.WinnerPrompt, noTables)
}

// Every label appended to a system prompt at runtime must be named by the
// precedence block, otherwise the rules bind to nothing. The headers live in this
// package precisely so this test can see all of them.
func TestRuntimeLabelsAreNamedInThePrompts(t *testing.T) {
	loadYAML(t, "telegram:\n  token: \"123:ABC\"\n")

	for _, header := range []string{ChatInstructionsHeader, ChatMemoryHeader} {
		label, _, ok := strings.Cut(header, "\n")
		require.True(t, ok, "header %q must start with a label line", header)
		require.True(t, strings.HasPrefix(label, "["), "header %q must start with a [LABEL]", header)
		assert.Contains(t, Environment.AppPrompt, label)
	}

	// The winner announcement appends both runtime blocks: the chat persona and
	// the long-term memory it draws its in-jokes from.
	for _, label := range []string{"[CHAT INSTRUCTIONS]", "[LONG-TERM CHAT MEMORY]"} {
		assert.Contains(t, Environment.WinnerPrompt, label)
	}
}

// The winner prompt drops [RESPONSE LENGTH & TONE] on purpose (an announcement
// must not obey the chat contract's 1-3 sentence limit), so no block it does
// carry may point at that section by name.
func TestWinnerPromptReferencesNoMissingSection(t *testing.T) {
	loadYAML(t, "telegram:\n  token: \"123:ABC\"\n")

	assert.NotContains(t, Environment.WinnerPrompt, "[RESPONSE LENGTH & TONE]")
}

func TestSharedBlockOverrideReachesEveryPromptUsingIt(t *testing.T) {
	loadYAML(t, `
telegram:
  token: "123:ABC"
app:
  prompts:
    telegram_markup: |
      [TELEGRAM MARKUP]
      - Only plain text, nothing else.
`)

	const custom = "Only plain text, nothing else."
	assert.Contains(t, Environment.AppPrompt, custom)
	assert.Contains(t, Environment.WinnerPrompt, custom)
	assert.NotContains(t, Environment.SummaryPrompt, custom)

	// the block it replaced is gone, the untouched blocks stay
	assert.NotContains(t, Environment.AppPrompt, "the bot escapes them itself")
	assert.Contains(t, Environment.AppPrompt, "[RULE PRECEDENCE]")
}

func TestSingleUseBlockOverrideStaysInItsOwnPrompt(t *testing.T) {
	loadYAML(t, `
telegram:
  token: "123:ABC"
app:
  prompts:
    winner_tone: |
      [TONE]
      - Report it straight, no jokes.
`)

	const custom = "Report it straight, no jokes."
	assert.Contains(t, Environment.WinnerPrompt, custom)
	assert.NotContains(t, Environment.AppPrompt, custom)
}
