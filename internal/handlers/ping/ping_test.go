package ping

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func mockedUpdate() *models.Update {
	return &models.Update{
		Message: &models.Message{
			From: &models.User{
				ID: 234,
			},
			Chat: models.Chat{
				ID: 123,
			},
			Text: "/ping",
		},
	}
}

func TestMessageParams(t *testing.T) {
	update := mockedUpdate()
	expected := &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		ReplyParameters: &models.ReplyParameters{
			MessageID: update.Message.ID,
		},
		Text: text(),
	}

	assert.Equal(t, messageParams(update), expected)
}

func TestText(t *testing.T) {
	assert.Equal(t, text(), "Pong!")
}
