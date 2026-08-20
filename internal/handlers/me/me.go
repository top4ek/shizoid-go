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
)

func Handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	text := responseText(app.Locale(ctx), update)
	if text == "" {
		return
	}
	// the rendered nickname is a user link, whose preview Telegram would turn
	// into a tappable button
	telegram.Impersonate(ctx, b, update, text, telegram.ChatMessageOpts{DisableLinkPreview: true})
}

func responseText(lang string, update *models.Update) string {
	displayName := update.Message.From.Username
	if displayName == "" {
		displayName = update.Message.From.FirstName
	}
	if displayName == "" {
		displayName = "Unknown"
	}
	userLink := utils.UserMarkdownLink(update.Message.From.ID, update.Message.From.Username, displayName)
	payload := utils.ExtractCommandPayloadText(update)
	if payload == "" {
		action := locale.Random(lang, "me")
		if action == "" {
			action = "..."
		}
		return fmt.Sprintf("%s %s", userLink, telegram.FormatPlain(action))
	}
	return fmt.Sprintf("%s %s", userLink, telegram.FormatPlain(payload))
}
