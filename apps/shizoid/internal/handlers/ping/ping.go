package ping

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"apps/shizoid/internal/app"
	"apps/shizoid/internal/locale"
	"apps/shizoid/internal/telegram"
)

const (
	Command     = "ping"
	Description = "Says Pong"
)

func Handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	text := locale.Random(app.Locale(ctx), "ping")
	if text == "" {
		text = "Pong!"
	}
	telegram.Reply(ctx, b, update, text)
}
