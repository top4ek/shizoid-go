package handlers

import (
	"context"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"

	"shizoid/internal/app"
	"shizoid/internal/config"
	"shizoid/internal/handlers/news"
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
	cmds := buildCommands(true)
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

// /news generates its issue over the neural summary chain, so it must not be
// registered at all when no summary provider is configured: no handler and no
// entry in the published Telegram command menu.
func TestNewsCommandGatedOnSummaryChain(t *testing.T) {
	has := func(cmds []command, name string) bool {
		for _, c := range cmds {
			if c.name == name {
				return true
			}
		}
		return false
	}

	withNews := buildCommands(true)
	assert.True(t, has(withNews, news.Command))

	withoutNews := buildCommands(false)
	assert.False(t, has(withoutNews, news.Command))

	// only /news is gated; every other command stays registered
	assert.Len(t, withoutNews, len(withNews)-1)
	for _, c := range withNews {
		if c.name != news.Command {
			assert.True(t, has(withoutNews, c.name), "command %q must not be gated", c.name)
		}
	}
}

// gate is the single place the nil/enabled/permission prologue lives, so the
// checks the command handlers used to repeat are covered here instead.
func TestGate(t *testing.T) {
	withOwners := func(t *testing.T, owners []int64) {
		t.Helper()
		old := config.Environment.BotOwners
		t.Cleanup(func() { config.Environment.BotOwners = old })
		config.Environment.BotOwners = owners
	}
	withAllowToAll := func(t *testing.T, v bool) {
		t.Helper()
		old := config.Environment.AllowToAll
		t.Cleanup(func() { config.Environment.AllowToAll = old })
		config.Environment.AllowToAll = v
	}
	msgUpdate := func() *models.Update {
		return &models.Update{Message: &models.Message{
			From: &models.User{ID: 234},
			Chat: models.Chat{ID: 123},
			Text: "/cmd",
		}}
	}

	run := func(c command, update *models.Update) bool {
		called := false
		c.handler = func(context.Context, *bot.Bot, *models.Update) { called = true }
		gate(c)(context.Background(), nil, update)
		return called
	}

	t.Run("drops updates without a message or sender", func(t *testing.T) {
		assert.False(t, run(command{}, &models.Update{}))
		assert.False(t, run(command{}, &models.Update{Message: &models.Message{}}))
	})

	t.Run("passes a public command through", func(t *testing.T) {
		assert.True(t, run(command{}, msgUpdate()))
	})

	t.Run("enforces needsEnabled", func(t *testing.T) {
		withAllowToAll(t, false)
		// no chat in the context and allow_to_all off means not enabled
		assert.False(t, run(command{needsEnabled: true}, msgUpdate()))

		withAllowToAll(t, true)
		assert.True(t, run(command{needsEnabled: true}, msgUpdate()))
	})

	t.Run("enforces roleOwner", func(t *testing.T) {
		withOwners(t, []int64{123, 456})
		assert.False(t, run(command{role: roleOwner}, msgUpdate()))

		withOwners(t, []int64{123, 234, 345})
		assert.True(t, run(command{role: roleOwner}, msgUpdate()))
	})
}
