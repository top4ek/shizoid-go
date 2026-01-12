package say

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"shizoid/internal/config"
)

func update() *models.Update {
	return &models.Update{
		Message: &models.Message{
			From: &models.User{
				ID: 234,
			},
			Chat: models.Chat{
				ID: 123,
			},
			Text: "/say blah-blah-blah",
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

func TestReplyForOwner(t *testing.T) {
	withBotOwners(t, []int64{123, 234, 345}, func() {
		result := reply(update())

		assert.True(t, result)
	})
}

func TestReplyForUser(t *testing.T) {
	withBotOwners(t, []int64{123, 456, 345}, func() {
		result := reply(update())

		assert.False(t, result)
	})
}

func TestMessageParams(t *testing.T) {
	update := update()
	expected := &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		ReplyParameters: &models.ReplyParameters{
			MessageID: update.Message.ID,
		},
		Text:      text(update),
		ParseMode: models.ParseModeMarkdown,
	}

	assert.Equal(t, messageParams(update), expected)
}

func TestText(t *testing.T) {
	assert.Equal(t, text(update()), "blah-blah-blah")
}
