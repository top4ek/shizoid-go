package ping

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"shizoid/internal/app"
	"shizoid/internal/locale"
	"shizoid/internal/telegram"
)

const (
	Command     = "ping"
	Description = "Says Pong"
	HandlerType = bot.HandlerTypeMessageText
	MatchType   = bot.MatchTypeCommandStartOnly
)

func Handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	text := locale.Random(app.Locale(ctx), "ping")
	if text == "" {
		text = "Pong!"
	}
	telegram.Reply(ctx, b, update, text, "")
}
