package me

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"shizoid/internal/utils"
)

const (
	Command     = "me"
	Description = "Simulates /me like in XMPP or IRC"
	HandlerType = bot.HandlerTypeMessageText
	MatchType   = bot.MatchTypeCommand
)

var (
	emptyResponses = []string{
		"многозначительно молчит.",
		"громко думает.",
		"размышляет о всяком.",
		"медитирует.",
		"ничего не понимает.",
		"спокойно ждет, пока мимо приплывут трупы врагов.",
		"любит всех.",
		"в ресурсе, в потоке, в своём уме.",
		"прячет голову в песок.",
		"курит бамбук.",
		"загадывает желание.",
		"листает мемы и ностальгирует по ибаш.орг.",
	}
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
	payload := utils.ExtractCommandPayloadText(update)
	if payload == "" {
		return fmt.Sprintf("*@%s* %s", update.Message.From.Username, utils.PickRandomString(emptyResponses))
	}
	return fmt.Sprintf("*@%s* %s", update.Message.From.Username, payload)
}
