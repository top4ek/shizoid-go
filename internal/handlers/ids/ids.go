package ids

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	Command     = "ids"
	Description = "Returns IDs of chat and user"
	HandlerType = bot.HandlerTypeMessageText
	MatchType   = bot.MatchTypeCommandStartOnly
)

func Handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	b.SendMessage(ctx, messageParams(update))
}

func messageParams(update *models.Update) *bot.SendMessageParams {
	return &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		ReplyParameters: &models.ReplyParameters{
			MessageID: update.Message.ID,
		},
		Text:      text(update),
		ParseMode: models.ParseModeMarkdown,
	}
}

func text(update *models.Update) string {
	return fmt.Sprintf("*Chat*: %d\n*User*: %d\n*Type*: %s",
		update.Message.Chat.ID,
		update.Message.From.ID,
		update.Message.Chat.Type)
}
