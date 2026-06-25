package say

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"shizoid/internal/telegram"
	"shizoid/internal/utils"
)

const (
	Command     = "say"
	Description = "Says something from bot's name"
	HandlerType = bot.HandlerTypeMessageText
	MatchType   = bot.MatchTypeCommand
)

func Handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if !canReply(update) {
		return
	}

	replyToID := 0
	if update.Message.ReplyToMessage != nil {
		replyToID = update.Message.ReplyToMessage.ID
	}

	telegram.Send(ctx, b, update, text(update), replyToID)
	telegram.Delete(ctx, b, update.Message.Chat.ID, update.Message.ID)
}

func canReply(update *models.Update) bool {
	return utils.IsBotOwner(update) && text(update) != ""
}

func text(update *models.Update) string {
	return utils.ExtractCommandPayloadText(update)
}
