package stop

import (
	"context"
	"shizoid/internal/config"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	Command     = "stop"
	Description = "Stop bot(leaves)"
	HandlerType = bot.HandlerTypeMessageText
	MatchType   = bot.MatchTypeCommandStartOnly
)

func Handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if reply(update, config.Environment.BotOwners) == true {
		b.SendMessage(ctx, messageParams(update))
		disableChains(update)
	}
}

func disableChains(update *models.Update) {

}

func userAdmin(update *models.Update, userId int64) bool {
	return true
}

func reply(update *models.Update, owners []int64) bool {
	// if update.Message.From.ID == config.Environment.BotOwners[] || u
	return true
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
	return "Stop!"
}
