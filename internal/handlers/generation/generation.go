// Package generation lets chat admins switch classic/simplified Markov modes.
package generation

import (
	"context"
	"strings"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"go.uber.org/zap"

	"shizoid/internal/app"
	"shizoid/internal/locale"
	"shizoid/internal/logger"
	"shizoid/internal/models"
	"shizoid/internal/telegram"
	"shizoid/internal/utils"
)

const (
	Command     = "generation"
	Description = "Show or set generation mode"
	HandlerType = bot.HandlerTypeMessageText
	MatchType   = bot.MatchTypeCommand
)

func Handler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	if update.Message == nil || update.Message.From == nil || !app.Enabled(ctx) {
		return
	}
	lang := app.Locale(ctx)
	chat := app.ChatFrom(ctx)
	if chat == nil {
		return
	}
	userID := update.Message.From.ID

	payload := normalizedPayload(update)
	modes := modeList()
	switch action, mode := classifyGenerationAction(payload); action {
	case generationShow:
		telegram.Reply(ctx, b, update, locale.T(lang, "generation.current", "mode", chat.GenerationMode.String()), "")
	case generationUnknown:
		telegram.Reply(ctx, b, update, locale.T(lang, "generation.unknown", "list", modes), "")
	case generationSet:
		if !utils.IsChatAdmin(ctx, b, chat.ID, userID) {
			telegram.Reply(ctx, b, update, locale.T(lang, "common.not_admin"), "")
			return
		}
		if err := models.Chats.SetGenerationMode(ctx, chat.ID, mode); err != nil {
			logger.Instance().Error("set generation mode", zap.Error(err))
			return
		}
		telegram.Reply(ctx, b, update, locale.T(lang, "generation.set", "mode", mode.String()), "")
	}
}

type generationAction int

const (
	generationShow generationAction = iota
	generationUnknown
	generationSet
)

func normalizedPayload(update *tgmodels.Update) string {
	return strings.ToLower(strings.TrimSpace(utils.ExtractCommandPayloadText(update)))
}

func classifyGenerationAction(payload string) (generationAction, models.GenerationMode) {
	if payload == "" {
		return generationShow, 0
	}
	mode, ok := models.ParseGenerationMode(payload)
	if !ok {
		return generationUnknown, 0
	}
	return generationSet, mode
}

func modeList() string {
	names := make([]string, len(models.GenerationModes()))
	for i, m := range models.GenerationModes() {
		names[i] = m.String()
	}
	return strings.Join(names, ", ")
}
