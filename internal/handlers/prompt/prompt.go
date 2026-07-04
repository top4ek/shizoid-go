// Package prompt lets bot owners set a per-chat extra system prompt used in
// neural generation, personalizing the bot per chat.
package prompt

import (
	"context"
	"database/sql"
	"strings"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"go.uber.org/zap"

	"shizoid/internal/app"
	"shizoid/internal/locale"
	"shizoid/internal/logger"
	"shizoid/internal/models"
	"shizoid/internal/telegram"
	"shizoid/internal/utils"
)

const (
	Command     = "prompt"
	Description = "Show, set, or clear the chat's extra system prompt"
	HandlerType = bot.HandlerTypeMessageText
	MatchType   = bot.MatchTypeCommand
)

func Handler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	lang := app.Locale(ctx)
	chat := app.ChatFrom(ctx)
	if chat == nil {
		return
	}

	payload := strings.TrimSpace(utils.ExtractCommandPayloadText(update))
	switch {
	case payload == "":
		telegram.Reply(ctx, b, update, currentPromptText(chat, lang))
	case strings.EqualFold(payload, "disable"):
		if err := app.Store().Chats.SetSystemPrompt(ctx, chat.ID, sql.NullString{}); err != nil {
			logger.Instance().Error("prompt clear", zap.Error(err))
			return
		}
		telegram.Reply(ctx, b, update, locale.T(lang, "prompt.cleared"))
	default:
		if err := app.Store().Chats.SetSystemPrompt(ctx, chat.ID, sql.NullString{String: payload, Valid: true}); err != nil {
			logger.Instance().Error("prompt set", zap.Error(err))
			return
		}
		telegram.Reply(ctx, b, update, locale.T(lang, "prompt.set"))
	}
}

func currentPromptText(chat *models.Chat, lang string) string {
	if chat.SystemPrompt.Valid {
		if p := strings.TrimSpace(chat.SystemPrompt.String); p != "" {
			return locale.T(lang, "prompt.current", "prompt", p)
		}
	}
	return locale.T(lang, "prompt.none")
}
