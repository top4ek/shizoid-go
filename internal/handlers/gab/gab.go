package gab

import (
	"context"
	"fmt"
	"strconv"
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
	Command     = "gab"
	Description = "Show or set flood level (0-50, chat admins)"
	HandlerType = bot.HandlerTypeMessageText
	MatchType   = bot.MatchTypeCommand
)

func Handler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	if update.Message == nil || update.Message.From == nil || !app.Enabled(ctx) || !app.Ready() {
		return
	}
	chat := app.ChatFrom(ctx)
	if chat == nil {
		return
	}
	lang := app.Locale(ctx)
	chatID := update.Message.Chat.ID
	if !utils.IsChatAdmin(ctx, b, chatID, update.Message.From.ID) {
		telegram.Reply(ctx, b, update, locale.T(lang, "common.not_admin"), "")
		return
	}

	payload := strings.TrimSpace(utils.ExtractCommandPayloadText(update))

	if payload == "" {
		telegram.Reply(ctx, b, update, levelText(lang, chat.Random), tgmodels.ParseModeMarkdown)
		return
	}

	value, err := strconv.Atoi(payload)
	if err != nil || value < 0 || value > 50 {
		telegram.Reply(ctx, b, update, locale.T(lang, "gab.error"), "")
		return
	}
	if err := models.Chats.SetRandom(ctx, chat.ID, value); err != nil {
		logger.Instance().Error("set gab", zap.Error(err))
		return
	}
	telegram.Reply(ctx, b, update, levelText(lang, int16(value)), tgmodels.ParseModeMarkdown)
}

func levelText(lang string, chance int16) string {
	return locale.T(lang, "gab.prefix") + " *" + bot.EscapeMarkdown(fmt.Sprint(chance)) + "%*\\."
}
