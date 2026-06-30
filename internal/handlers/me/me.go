package me

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"shizoid/internal/app"
	"shizoid/internal/locale"
	"shizoid/internal/telegram"
	"shizoid/internal/utils"
)

const (
	Command     = "me"
	Description = "Simulates /me like in XMPP or IRC"
	HandlerType = bot.HandlerTypeMessageText
	MatchType   = bot.MatchTypeCommand
)

func Handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}
	text := responseText(app.Locale(ctx), update)
	if text == "" {
		return
	}
	replyToID := 0
	if update.Message.ReplyToMessage != nil {
		replyToID = update.Message.ReplyToMessage.ID
	}
	telegram.Send(ctx, b, update, text, replyToID)
	telegram.Delete(ctx, b, update.Message.Chat.ID, update.Message.ID)
}

func responseText(lang string, update *models.Update) string {
	displayName := update.Message.From.Username
	if displayName == "" {
		displayName = update.Message.From.FirstName
	}
	if displayName == "" {
		displayName = "Unknown"
	}
	payload := utils.ExtractCommandPayloadText(update)
	if payload == "" {
		action := locale.Random(lang, "me")
		if action == "" {
			action = "..."
		}
		return fmt.Sprintf("*%s* %s", bot.EscapeMarkdown(displayName), bot.EscapeMarkdown(action))
	}
	return fmt.Sprintf("*%s* %s", bot.EscapeMarkdown(displayName), bot.EscapeMarkdown(payload))
}
