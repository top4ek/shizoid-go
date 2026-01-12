package me

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func updateMock() *models.Update {
	return &models.Update{
		Message: &models.Message{
			Text: "/me thinks different",
			From: &models.User{
				ID:       234,
				Username: "shizoid",
			},
			Chat: models.Chat{
				ID:   123,
				Type: models.ChatTypePrivate,
			},
		},
	}
}

func TestText_EmptyStrging(t *testing.T) {
	mock := &models.Update{
		Message: &models.Message{
			From: &models.User{
				ID:       234,
				Username: "shizoid",
			},
			Chat: models.Chat{
				ID:   123,
				Type: models.ChatTypePrivate,
			},
		},
	}

	strs := strings.SplitN(text(mock), " ", 2)

	assert.Equal(t, "*@shizoid*", strs[0])
	assert.Contains(t, emptyResponses, strs[1])
}

func TestText_NonEmptyStrging(t *testing.T) {
	strs := strings.SplitN(text(updateMock()), " ", 2)

	assert.Equal(t, "*@shizoid*", strs[0])
	assert.Equal(t, "thinks different", strs[1])
}

func TestMessageParams(t *testing.T) {
	update := updateMock()
	expected := &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		ReplyParameters: &models.ReplyParameters{
			MessageID: update.Message.ID,
		},
		Text:      text(update),
		ParseMode: models.ParseModeMarkdown,
	}

	assert.Equal(t, expected, messageParams(update))
}
