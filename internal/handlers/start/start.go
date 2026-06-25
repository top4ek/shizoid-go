package start

import (
	"context"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"go.uber.org/zap"

	"shizoid/internal/app"
	"shizoid/internal/locale"
	"shizoid/internal/logger"
	"shizoid/internal/models"
	"shizoid/internal/telegram"
)

const (
	Command     = "start"
	Description = "Start the bot in current chat"
	HandlerType = bot.HandlerTypeMessageText
	MatchType   = bot.MatchTypeCommandStartOnly
)

func Handler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}
	if !app.IsOwner(update.Message.From.ID) || !app.Ready() {
		return
	}
	if err := models.Chats.Enable(ctx, update.Message.Chat.ID); err != nil {
		logger.Instance().Error("start enable", zap.Error(err))
		return
	}
	telegram.Reply(ctx, b, update, locale.Random(app.Locale(ctx), "ok"))
}
