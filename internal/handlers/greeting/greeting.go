// Package greeting manages the per-chat join greeting (chat admins).
package greeting

import (
	"context"
	"strings"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"go.uber.org/zap"

	"shizoid/internal/app"
	"shizoid/internal/locale"
	"shizoid/internal/logger"
	"shizoid/internal/telegram"
	"shizoid/internal/utils"
)

const (
	Command     = "greeting"
	Description = "Set or clear the chat greeting"
)

func Handler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	lang := app.Locale(ctx)

	payload := strings.TrimSpace(utils.ExtractCommandPayloadText(update))
	switch parseGreetingAction(payload) {
	case greetingUsage:
		telegram.Reply(ctx, b, update, locale.T(lang, "greeting.usage"))
	case greetingClear:
		if err := app.Store().Greetings.Delete(ctx, chatID); err != nil {
			logger.Instance().Error("greeting delete", zap.Error(err))
			return
		}
		if err := app.Store().Chats.SetGreeting(ctx, chatID, false); err != nil {
			logger.Instance().Error("greeting flag", zap.Error(err))
			return
		}
		telegram.Reply(ctx, b, update, locale.T(lang, "greeting.cleared"))
	case greetingSet:
		if err := telegram.ValidateV2(payload); err != nil {
			telegram.Reply(ctx, b, update, locale.Random(lang, "nok"))
			return
		}
		if err := app.Store().Greetings.Set(ctx, chatID, payload); err != nil {
			logger.Instance().Error("greeting set", zap.Error(err))
			return
		}
		if err := app.Store().Chats.SetGreeting(ctx, chatID, true); err != nil {
			logger.Instance().Error("greeting flag", zap.Error(err))
			return
		}
		telegram.Reply(ctx, b, update, locale.T(lang, "greeting.set"))
	}
}

type greetingAction int

const (
	greetingUsage greetingAction = iota
	greetingClear
	greetingSet
)

func parseGreetingAction(payload string) greetingAction {
	payload = strings.TrimSpace(payload)
	switch {
	case payload == "":
		return greetingUsage
	case strings.EqualFold(payload, "disable"):
		return greetingClear
	default:
		return greetingSet
	}
}
