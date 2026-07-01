package handlers

import (
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"

	"shizoid/internal/app"
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

func TestIsMentioned(t *testing.T) {
	cases := []struct {
		name     string
		msg      *models.Message
		username string
		want     bool
	}{
		{
			name: "mentions bot",
			msg: &models.Message{
				Text:     "@testbot hello",
				Entities: []models.MessageEntity{{Type: models.MessageEntityTypeMention, Offset: 0, Length: 8}},
			},
			username: "testbot",
			want:     true,
		},
		{
			name: "mentions other user",
			msg: &models.Message{
				Text:     "@someone hello",
				Entities: []models.MessageEntity{{Type: models.MessageEntityTypeMention, Offset: 0, Length: 8}},
			},
			username: "testbot",
			want:     false,
		},
		{
			name:     "no entities",
			msg:      &models.Message{Text: "@testbot hello"},
			username: "testbot",
			want:     false,
		},
		{
			name: "empty bot username",
			msg: &models.Message{
				Text:     "@testbot hello",
				Entities: []models.MessageEntity{{Type: models.MessageEntityTypeMention, Offset: 0, Length: 8}},
			},
			username: "",
			want:     false,
		},
		{
			name: "case insensitive",
			msg: &models.Message{
				Text:     "@TestBot hello",
				Entities: []models.MessageEntity{{Type: models.MessageEntityTypeMention, Offset: 0, Length: 8}},
			},
			username: "testbot",
			want:     true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			app.SetBotUsername(c.username)
			assert.Equal(t, c.want, isMentioned(c.msg))
		})
	}
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
