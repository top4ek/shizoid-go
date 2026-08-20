package ids

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"shizoid/internal/app"
	"shizoid/internal/locale"
	"shizoid/internal/telegram"
)

const (
	Command     = "ids"
	Description = "Returns IDs of chat and user"
)

func Handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	telegram.Reply(ctx, b, update, text(app.Locale(ctx), update))
}

func text(lang string, update *models.Update) string {
	chatID := telegram.FormatPlain(fmt.Sprint(update.Message.Chat.ID))
	chatType := telegram.FormatPlain(string(update.Message.Chat.Type))
	userID := telegram.FormatPlain(fmt.Sprint(update.Message.From.ID))
	return fmt.Sprintf(
		"*%s:* %s \\(%s\\)\n*%s:* %s",
		telegram.FormatPlain(locale.T(lang, "ids.chat")),
		chatID,
		chatType,
		telegram.FormatPlain(locale.T(lang, "ids.user")),
		userID,
	)
}
