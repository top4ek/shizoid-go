package status

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	Command     = "status"
	Description = "Some statistics for chat"
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
		Text:      text(),
		ParseMode: models.ParseModeMarkdown,
	}
}

func text() string {
	return "Status!"
}
