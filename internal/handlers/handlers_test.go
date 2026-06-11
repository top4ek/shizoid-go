package handlers

import (
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"
)

func TestUpdateKind(t *testing.T) {
	cases := []struct {
		name   string
		update *models.Update
		want   string
	}{
		{"message", &models.Update{Message: &models.Message{}}, "message"},
		{"callback", &models.Update{CallbackQuery: &models.CallbackQuery{}}, "callback_query"},
		{"other", &models.Update{}, "other"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, updateKind(c.update))
		})
	}
}

func TestIsBotCommand(t *testing.T) {
	assert.True(t, isBotCommand(&models.Message{Text: "/ping"}))
	assert.True(t, isBotCommand(&models.Message{
		Text:     "/ping@shizoid_bot",
		Entities: []models.MessageEntity{{Type: models.MessageEntityTypeBotCommand, Offset: 0, Length: 17}},
	}))
	assert.False(t, isBotCommand(&models.Message{Text: "hello"}))
	assert.False(t, isBotCommand(nil))
}

func TestCommandsUnique(t *testing.T) {
	cmds := commands()
	seen := make(map[string]struct{}, len(cmds))
	for _, c := range cmds {
		assert.NotEmpty(t, c.name)
		assert.NotEmpty(t, c.description)
		assert.NotNil(t, c.handler)
		_, dup := seen[c.name]
		assert.False(t, dup, "duplicate command %q", c.name)
		seen[c.name] = struct{}{}
	}
}
