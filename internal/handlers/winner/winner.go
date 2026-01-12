package winner

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"shizoid/internal/utils"
)

// /winner
// /winner current
// /winner enable name
// /winner disable

const (
	Command     = "winner"
	Description = "Starts bot in current chat"
	HandlerType = bot.HandlerTypeMessageText
	MatchType   = bot.MatchTypeCommand
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
		Text: text(update),
	}
}

func enable(update *models.Update, label string) {
	// TODO
}

func disable(update *models.Update) {
	// TODO
}

func text(update *models.Update) string {
	payload := utils.ExtractCommandPayloadText(update)
	if payload == "" {
		return "Winner!"
	}
	// if payload == "current" {
	return "WinnerCurrent"
	// }
}
