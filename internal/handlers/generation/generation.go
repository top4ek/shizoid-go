// Package generation lets chat admins switch generation mode: classic/simplified
// Markov-only walks, or neural (LLM with classic Markov fallback).
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
)

func Handler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	lang := app.Locale(ctx)
	chat := app.ChatFrom(ctx)
	if chat == nil {
		return
	}
	// Reading the mode is public; setting it is admin-only, which is why this
	// command registers as roleEveryone and checks below instead of in gate.
	payload := normalizedPayload(update)
	if payload == "" {
		telegram.Reply(ctx, b, update, locale.T(lang, "generation.current", "mode", chat.GenerationMode.String()))
		return
	}
	mode, ok := models.ParseGenerationMode(payload)
	if !ok {
		telegram.Reply(ctx, b, update, locale.T(lang, "generation.unknown", "list", modeList()))
		return
	}
	if !utils.RequireChatAdmin(ctx, b, update, lang) {
		return
	}
	if err := app.Store().Chats.SetGenerationMode(ctx, chat.ID, mode); err != nil {
		logger.Instance().Error("set generation mode", zap.Error(err))
		return
	}
	telegram.Reply(ctx, b, update, locale.T(lang, "generation.set", "mode", mode.String()))
}

func normalizedPayload(update *tgmodels.Update) string {
	return strings.ToLower(strings.TrimSpace(utils.ExtractCommandPayloadText(update)))
}

func modeList() string {
	names := make([]string, len(models.GenerationModes()))
	for i, m := range models.GenerationModes() {
		names[i] = m.String()
	}
	return strings.Join(names, ", ")
}
