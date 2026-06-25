// Package idle configures daily questions about inactive chat members.
package idle

import (
	"context"
	"database/sql"
	"strconv"
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
	Command     = "idle"
	Description = "Ask about silent members once a day"
	HandlerType = bot.HandlerTypeMessageText
	MatchType   = bot.MatchTypeCommand
)

func Handler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	if update.Message == nil || update.Message.From == nil || !app.Enabled(ctx) || !app.Ready() {
		return
	}
	chatID := update.Message.Chat.ID
	lang := app.Locale(ctx)
	if !utils.IsChatAdmin(ctx, b, chatID, update.Message.From.ID) {
		telegram.Reply(ctx, b, update, locale.T(lang, "common.not_admin"))
		return
	}

	payload := strings.ToLower(strings.TrimSpace(utils.ExtractCommandPayloadText(update)))
	if payload == "" {
		telegram.Reply(ctx, b, update, locale.T(lang, "idle_cmd.usage"))
		return
	}
	if payload == "disable" || payload == "0" {
		if err := models.Chats.SetIdle(ctx, chatID, sql.NullInt64{}); err != nil {
			logger.Instance().Error("idle disable", zap.Error(err))
			return
		}
		telegram.Reply(ctx, b, update, locale.T(lang, "idle_cmd.disabled"))
		return
	}

	days, err := strconv.Atoi(payload)
	if err != nil || days < 1 {
		telegram.Reply(ctx, b, update, locale.T(lang, "idle_cmd.usage"))
		return
	}
	if err := models.Chats.SetIdle(ctx, chatID, sql.NullInt64{Int64: int64(days), Valid: true}); err != nil {
		logger.Instance().Error("idle set", zap.Error(err))
		return
	}
	telegram.Reply(ctx, b, update, locale.T(lang, "idle_cmd.enabled", "days", days))
}
