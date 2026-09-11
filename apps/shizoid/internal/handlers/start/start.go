package start

import (
	"context"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"go.uber.org/zap"

	"apps/shizoid/internal/app"
	"apps/shizoid/internal/locale"
	"apps/shizoid/internal/logger"
	"apps/shizoid/internal/telegram"
)

const (
	Command     = "start"
	Description = "Start the bot in current chat"
)

func Handler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	if err := app.Store().Chats.Enable(ctx, update.Message.Chat.ID); err != nil {
		logger.Instance().Error("start enable", zap.Error(err))
		return
	}
	telegram.Reply(ctx, b, update, locale.Random(app.Locale(ctx), "ok"))
}
