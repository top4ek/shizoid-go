package utils

import (
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"

	"shizoid/internal/config"
)

func update() *models.Update {
	return &models.Update{
		Message: &models.Message{
			From: &models.User{
				ID: 234,
			},
		},
	}
}

func withBotOwners(t *testing.T, owners []int64, testFn func()) {
	old := config.Environment.BotOwners
	t.Cleanup(func() {
		config.Environment.BotOwners = old
	})

	config.Environment.BotOwners = owners
	testFn()
}

func TestIsBotOwner_True(t *testing.T) {
	withBotOwners(t, []int64{123, 234, 345}, func() {
		result := IsBotOwner(update())

		assert.True(t, result)
	})
}

func TestIsBotOwner_False(t *testing.T) {
	withBotOwners(t, []int64{123, 456, 345}, func() {
		result := IsBotOwner(update())

		assert.False(t, result)
	})
}

func TestPickRandomString(t *testing.T) {
	input := []string{"a", "b", "c", "d"}

	result := PickRandomString(input)

	assert.Contains(t, input, result)
}

func updateText(str string) *models.Update {
	return &models.Update{
		Message: &models.Message{
			Text: "/command " + str,
			From: &models.User{
				ID: 234,
			},
		},
	}
}

func TestExtractCommandPayloadText_EmptyPayload(t *testing.T) {
	str := ""
	assert.Equal(t, str, ExtractCommandPayloadText((updateText(str))))
}

func TestExtractCommandPayloadText_WithPayload(t *testing.T) {
	str := "with test    string"
	assert.Equal(t, str, ExtractCommandPayloadText((updateText(str))))
}

func TestParseLeadingCommand(t *testing.T) {
	cases := []struct {
		text    string
		command string
		mention string
		ok      bool
	}{
		{"/start", "start", "", true},
		{"/start@GluChatAI_Dev_bot", "start", "GluChatAI_Dev_bot", true},
		{"/start@GluChatAI_Dev_bot hello", "start", "GluChatAI_Dev_bot", true},
		{"hello", "", "", false},
		{"/say@mybot", "say", "mybot", true},
	}
	for _, c := range cases {
		command, mention, ok := ParseLeadingCommand(c.text)
		assert.Equal(t, c.ok, ok, c.text)
		if !c.ok {
			continue
		}
		assert.Equal(t, c.command, command, c.text)
		assert.Equal(t, c.mention, mention, c.text)
	}
}

func TestMatchesLeadingCommand(t *testing.T) {
	const bot = "GluChatAI_Dev_bot"
	assert.True(t, MatchesLeadingCommand("/start", "start", bot))
	assert.True(t, MatchesLeadingCommand("/start@"+bot, "start", bot))
	assert.True(t, MatchesLeadingCommand("/start@"+bot+" hello", "start", bot))
	assert.True(t, MatchesLeadingCommand("/START@"+bot, "start", bot))
	assert.True(t, MatchesLeadingCommand("/captcha@"+strings.ToUpper(bot), "captcha", bot))
	assert.True(t, MatchesLeadingCommand("/CAPTCHA@"+bot, "captcha", strings.ToLower(bot)))
	assert.True(t, MatchesLeadingCommand("/captcha@"+bot, "captcha", ""))
	assert.False(t, MatchesLeadingCommand("/start@jopa", "start", bot))
	assert.False(t, MatchesLeadingCommand("/stop", "start", bot))
	assert.False(t, MatchesLeadingCommand("hello", "start", bot))
}

func TestExtractCommandPayloadText_NilUpdate(t *testing.T) {
	assert.Equal(t, "", ExtractCommandPayloadText(nil))
}

func TestIsBotOwner_NilUpdate(t *testing.T) {
	assert.False(t, IsBotOwner(nil))
}

func TestUserName_Nil(t *testing.T) {
	name, err := UserName(nil)
	assert.Equal(t, "Unknown", name)
	assert.Error(t, err)
}

func TestUserMarkdownLink_WithUsername(t *testing.T) {
	got := UserMarkdownLink(42, "alice", "alice")
	assert.Equal(t, "[alice](https://t.me/alice)", got)
}

func TestUserMarkdownLink_WithoutUsername(t *testing.T) {
	got := UserMarkdownLink(42, "", "Bob")
	assert.Equal(t, "[Bob](tg://user?id=42)", got)
}

func TestUserMarkdownLink_EscapesLabel(t *testing.T) {
	got := UserMarkdownLink(1, "", "a.b")
	assert.Equal(t, `[a\.b](tg://user?id=1)`, got)
}

func TestUserMarkdownLink_NoID(t *testing.T) {
	got := UserMarkdownLink(0, "", "plain")
	assert.Equal(t, "plain", got)
}
