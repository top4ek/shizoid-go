package lang

import (
	"context"
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
	Command     = "lang"
	Description = "Show or set the chat language"
	HandlerType = bot.HandlerTypeMessageText
	MatchType   = bot.MatchTypeCommand
)

func Handler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	if update.Message == nil || update.Message.From == nil || !app.Enabled(ctx) || !app.Ready() {
		return
	}
	chat := app.ChatFrom(ctx)
	if chat == nil {
		return
	}
	lang := app.Locale(ctx)
	available := strings.Join(locale.Available(), ", ")
	payload := strings.ToLower(strings.TrimSpace(utils.ExtractCommandPayloadText(update)))

	if payload == "" {
		telegram.Reply(ctx, b, update, locale.T(lang, "lang.current", "lang", chat.Locale))
		return
	}
	if !locale.Has(payload) {
		telegram.Reply(ctx, b, update, locale.T(lang, "lang.unknown", "list", available))
		return
	}
	if !utils.IsChatAdmin(ctx, b, update.Message.Chat.ID, update.Message.From.ID) {
		telegram.Reply(ctx, b, update, locale.T(lang, "common.not_admin"))
		return
	}
	if err := models.Chats.SetLocale(ctx, chat.ID, payload); err != nil {
		logger.Instance().Error("set locale", zap.Error(err))
		return
	}
	telegram.Reply(ctx, b, update, locale.T(payload, "lang.set", "lang", payload))
}
