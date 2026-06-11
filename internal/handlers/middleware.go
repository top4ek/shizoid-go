package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"go.uber.org/zap"

	"shizoid/internal/app"
	"shizoid/internal/config"
	"shizoid/internal/logger"
	"shizoid/internal/sentry"
	"shizoid/internal/models"
)

func LogUpdate(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
		logger.Instance().Debug("update", updateLogFields(update)...)
		next(ctx, b, update)
	}
}

func updateLogFields(update *tgmodels.Update) []zap.Field {
	fields := []zap.Field{
		zap.Int64("update_id", update.ID),
		zap.String("kind", updateKind(update)),
	}
	if msg := updateMessage(update); msg != nil {
		fields = append(fields,
			zap.Int64("chat_id", msg.Chat.ID),
			zap.String("chat_type", string(msg.Chat.Type)),
		)
		if msg.From != nil {
			fields = append(fields, zap.Int64("user_id", msg.From.ID))
		}
		if msg.Text != "" {
			fields = append(fields, zap.String("text", truncateLogText(msg.Text)))
		}
	}
	if update.CallbackQuery != nil {
		fields = append(fields,
			zap.Int64("user_id", update.CallbackQuery.From.ID),
			zap.String("data", update.CallbackQuery.Data),
		)
	}
	return fields
}

func truncateLogText(text string) string {
	const max = 200
	r := []rune(text)
	if len(r) <= max {
		return text
	}
	return string(r[:max]) + "…"
}

func updateKind(update *tgmodels.Update) string {
	switch {
	case update.Message != nil:
		return "message"
	case update.EditedMessage != nil:
		return "edited_message"
	case update.ChannelPost != nil:
		return "channel_post"
	case update.EditedChannelPost != nil:
		return "edited_channel_post"
	case update.CallbackQuery != nil:
		return "callback_query"
	case update.InlineQuery != nil:
		return "inline_query"
	case update.MyChatMember != nil:
		return "my_chat_member"
	case update.ChatMember != nil:
		return "chat_member"
	case update.ChatJoinRequest != nil:
		return "chat_join_request"
	default:
		return "other"
	}
}

func updateMessage(update *tgmodels.Update) *tgmodels.Message {
	switch {
	case update.Message != nil:
		return update.Message
	case update.EditedMessage != nil:
		return update.EditedMessage
	case update.ChannelPost != nil:
		return update.ChannelPost
	case update.EditedChannelPost != nil:
		return update.EditedChannelPost
	default:
		return nil
	}
}

func Ingest(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
		msg := update.Message
		if msg == nil || !app.Ready() {
			next(ctx, b, update)
			return
		}

		if len(msg.NewChatMembers) > 0 {
			persisted, err := models.Ingest.EnsureJoin(ctx, chatModel(msg), msg.NewChatMembers)
			if err != nil {
				logger.Instance().Error("ingest join", zap.Error(err))
			} else if persisted != nil {
				ctx = app.WithChat(ctx, persisted)
			}
			next(ctx, b, update)
			return
		}

		if msg.From == nil {
			next(ctx, b, update)
			return
		}

		chat := chatModel(msg)
		user := userModel(msg.From)
		left := msg.LeftChatMember != nil && msg.LeftChatMember.ID == msg.From.ID

		persistedChat, participation, err := models.Ingest.EnsureEntities(ctx, chat, user, left)
		if err != nil {
			logger.Instance().Error("ingest ensure", zap.Error(err))
			next(ctx, b, update)
			return
		}

		ctx = app.WithChat(ctx, persistedChat)
		ctx = app.WithParticipation(ctx, participation)
		if isBotCommand(msg) {
			ctx = app.WithSkipMessageHistory(ctx)
		}

		go runCollectStats(persistedChat, msg)

		next(ctx, b, update)
	}
}

func runCollectStats(chat *models.Chat, msg *tgmodels.Message) {
	defer func() {
		if r := recover(); r != nil {
			logger.Instance().Error("collectStats panic", zap.Any("panic", r))
			sentry.Capture(fmt.Errorf("collectStats panic: %v", r))
		}
	}()
	collectStats(chat, msg)
}

// collectStats updates learning, context and scoring in the background.
func collectStats(chat *models.Chat, msg *tgmodels.Message) {
	bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if !chat.Enabled() || isBotCommand(msg) {
		return
	}

	if msg.Text != "" {
		if err := app.Gen().Learn(bgCtx, chat.ID, msg.Text); err != nil {
			logger.Instance().Error("learn", zap.Error(err))
		}
		if err := models.Messages.Append(bgCtx, chat.ID, msg.From.ID, msg.Text); err != nil {
			logger.Instance().Error("messages append", zap.Error(err))
		}
	}

	if chat.WinnerEnabled() && msg.Text != "" {
		delta := len(strings.Fields(msg.Text))
		if delta > 0 {
			if err := models.Participations.IncrScore(bgCtx, chat.ID, msg.From.ID, delta); err != nil {
				logger.Instance().Error("incr score", zap.Error(err))
			}
		}
	}
}

func chatModel(msg *tgmodels.Message) *models.Chat {
	c := &models.Chat{
		ID:             msg.Chat.ID,
		Kind:           string(msg.Chat.Type),
		Locale:         defaultLocale(),
		GenerationMode: config.DefaultGenerationMode,
	}
	c.Title = nullString(msg.Chat.Title)
	c.FirstName = nullString(msg.Chat.FirstName)
	c.LastName = nullString(msg.Chat.LastName)
	c.Username = nullString(msg.Chat.Username)
	return c
}

func userModel(u *tgmodels.User) *models.User {
	m := &models.User{ID: u.ID}
	m.IsBot.Bool, m.IsBot.Valid = u.IsBot, true
	m.FirstName = nullString(u.FirstName)
	m.LastName = nullString(u.LastName)
	m.Username = nullString(u.Username)
	m.LanguageCode = nullString(u.LanguageCode)
	return m
}

func defaultLocale() string {
	if config.Environment.Locale != "" {
		return config.Environment.Locale
	}
	return "ru"
}

func isBotCommand(msg *tgmodels.Message) bool {
	if msg == nil {
		return false
	}
	for _, part := range []struct {
		text     string
		entities []tgmodels.MessageEntity
	}{
		{msg.Text, msg.Entities},
		{msg.Caption, msg.CaptionEntities},
	} {
		if part.text == "" {
			continue
		}
		for _, e := range part.entities {
			if e.Type == tgmodels.MessageEntityTypeBotCommand {
				return true
			}
		}
		if strings.HasPrefix(strings.TrimSpace(part.text), "/") {
			return true
		}
	}
	return false
}

func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
