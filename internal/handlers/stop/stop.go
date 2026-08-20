package stop

import (
	"context"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"go.uber.org/zap"

	"shizoid/internal/app"
	"shizoid/internal/locale"
	"shizoid/internal/logger"
	"shizoid/internal/telegram"
)

const (
	Command     = "stop"
	Description = "Stop the bot in current chat"
)

func Handler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	if err := app.Store().Chats.Disable(ctx, update.Message.Chat.ID); err != nil {
		logger.Instance().Error("stop disable", zap.Error(err))
		return
	}
	telegram.Reply(ctx, b, update, locale.Random(app.Locale(ctx), "ok"))
}
