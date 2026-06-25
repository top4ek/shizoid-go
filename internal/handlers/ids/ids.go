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
	HandlerType = bot.HandlerTypeMessageText
	MatchType   = bot.MatchTypeCommandStartOnly
)

func Handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}
	telegram.Reply(ctx, b, update, text(app.Locale(ctx), update))
}

func text(lang string, update *models.Update) string {
	chatID := bot.EscapeMarkdown(fmt.Sprint(update.Message.Chat.ID))
	chatType := bot.EscapeMarkdown(string(update.Message.Chat.Type))
	userID := bot.EscapeMarkdown(fmt.Sprint(update.Message.From.ID))
	return fmt.Sprintf(
		"*%s:* %s \\(%s\\)\n*%s:* %s",
		bot.EscapeMarkdown(locale.T(lang, "ids.chat")),
		chatID,
		chatType,
		bot.EscapeMarkdown(locale.T(lang, "ids.user")),
		userID,
	)
}
