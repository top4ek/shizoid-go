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
			fields = append(fields, zap.String("text", logger.TruncateLogText(msg.Text)))
		}
	}
	if update.CallbackQuery != nil {
		fields = append(fields,
			zap.Int64("user_id", update.CallbackQuery.From.ID),
			zap.String("data", update.CallbackQuery.Data),
		)
	}
	if cm := update.ChatMember; cm != nil {
		fields = append(fields,
			zap.Int64("chat_id", cm.Chat.ID),
			zap.String("old_status", string(cm.OldChatMember.Type)),
			zap.String("new_status", string(cm.NewChatMember.Type)),
		)
		if u, ok := memberUser(cm.NewChatMember); ok {
			fields = append(fields, zap.Int64("user_id", u.ID))
		}
	}
	return fields
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
		if !app.Ready() {
			next(ctx, b, update)
			return
		}

		if cm := update.ChatMember; cm != nil {
			if isJoinTransition(cm.OldChatMember, cm.NewChatMember) {
				if user, ok := memberUser(cm.NewChatMember); ok && !user.IsBot {
					ingestJoin(ctx, b, update, chatModelFromChat(cm.Chat), []tgmodels.User{*user}, "chat_member", next)
					return
				}
			}
			next(ctx, b, update)
			return
		}

		msg := update.Message
		if msg == nil {
			next(ctx, b, update)
			return
		}

		if len(msg.NewChatMembers) > 0 {
			ingestJoin(ctx, b, update, chatModel(msg), msg.NewChatMembers, "new_chat_members", next)
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

func ingestJoin(ctx context.Context, b *bot.Bot, update *tgmodels.Update, chat *models.Chat, members []tgmodels.User, source string, next bot.HandlerFunc) {
	logger.Instance().Debug("ingest join",
		zap.String("source", source),
		zap.Int64("chat_id", chat.ID),
		zap.Int("members_count", len(members)),
	)
	persisted, err := models.Ingest.EnsureJoin(ctx, chat, members)
	if err != nil {
		logger.Instance().Error("ingest join", zap.String("source", source), zap.Error(err))
	} else if persisted != nil {
		ctx = app.WithChat(ctx, persisted)
	}
	next(ctx, b, update)
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
	return chatModelFromChat(msg.Chat)
}

func chatModelFromChat(c tgmodels.Chat) *models.Chat {
	out := &models.Chat{
		ID:             c.ID,
		Kind:           string(c.Type),
		Locale:         defaultLocale(),
		GenerationMode: config.DefaultGenerationMode,
	}
	out.Title = nullString(c.Title)
	out.FirstName = nullString(c.FirstName)
	out.LastName = nullString(c.LastName)
	out.Username = nullString(c.Username)
	return out
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
