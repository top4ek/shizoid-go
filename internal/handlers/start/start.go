package start

import (
	"context"
	"shizoid/internal/config"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	Command     = "start"
	Description = "Starts bot in current chat"
	HandlerType = bot.HandlerTypeMessageText
	MatchType   = bot.MatchTypeCommandStartOnly
)

func Handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if reply(update, config.Environment.BotOwners) == true {
		enableChains(update)
		b.SendMessage(ctx, messageParams(update))
	}
}

func enableChains(update *models.Update) {
	// TODO
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

func reply(update *models.Update, owners []int64) bool {
	// if update.Message.From.ID == config.Environment.BotOwners[].
	return true
}

func text() string {
	return "Start!"
}
