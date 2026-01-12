package gab

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	Command     = "gab"
	Description = "Sets flood level (0-100)"
	HandlerType = bot.HandlerTypeMessageText
	MatchType   = bot.MatchTypeCommand
)

func Handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		ReplyParameters: &models.ReplyParameters{
			MessageID: update.Message.ID,
		},
		Text: text(),
	})
}

func text() string {
	return "Gab!"
}
