package me

import (
	"strings"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"

	"shizoid/internal/locale"
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

func TestResponseText_EmptyPayload(t *testing.T) {
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

	strs := strings.SplitN(responseText("ru", mock), " ", 2)

	assert.Equal(t, "*shizoid*", strs[0])
	actions := locale.List("ru", "me")
	assert.NotEmpty(t, actions)
	assert.Contains(t, escapedActions(actions), strs[1])
}

func TestResponseText_NonEmptyPayload(t *testing.T) {
	strs := strings.SplitN(responseText("ru", updateMock()), " ", 2)

	assert.Equal(t, "*shizoid*", strs[0])
	assert.Equal(t, bot.EscapeMarkdown("thinks different"), strs[1])
}

func escapedActions(actions []string) []string {
	out := make([]string, len(actions))
	for i, a := range actions {
		out[i] = bot.EscapeMarkdown(a)
	}
	return out
}
