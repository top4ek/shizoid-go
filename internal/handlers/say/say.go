package say

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"shizoid/internal/utils"
)

const (
	Command     = "say"
	Description = "Says something from bot's name"
	HandlerType = bot.HandlerTypeMessageText
	MatchType   = bot.MatchTypeCommand
)

func Handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if reply(update) {
		b.SendMessage(ctx, messageParams(update))
		b.DeleteMessage(ctx, deleteParams(update))
	}
}

func deleteParams(update *models.Update) *bot.DeleteMessageParams {
	return &bot.DeleteMessageParams{
		ChatID:    update.Message.Chat.ID,
		MessageID: update.Message.ID,
	}
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

func reply(update *models.Update) bool {
	return utils.IsBotOwner(update) && text(update) != ""
}

func text(update *models.Update) string {
	return utils.ExtractCommandPayloadText(update)
}
