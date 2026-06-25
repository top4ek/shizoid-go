package status

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"go.uber.org/zap"

	"shizoid/internal/app"
	"shizoid/internal/locale"
	"shizoid/internal/logger"
	"shizoid/internal/models"
	"shizoid/internal/telegram"
	"shizoid/internal/version"
)

const (
	Command     = "status"
	Description = "Some statistics for chat"
	HandlerType = bot.HandlerTypeMessageText
	MatchType   = bot.MatchTypeCommandStartOnly
)

func Handler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	if update.Message == nil || !app.Enabled(ctx) {
		return
	}
	chat := app.ChatFrom(ctx)
	if chat == nil || !app.Ready() {
		return
	}
	lang := app.Locale(ctx)
	pairs, err := models.Chats.PairsCount(ctx, chat.ID)
	if err != nil {
		logger.Instance().Error("status pairs", zap.Error(err))
	}
	telegram.Reply(ctx, b, update, statusText(lang, chat, pairs))
}

func statusText(lang string, chat *models.Chat, pairs int) string {
	active := locale.T(lang, "no")
	if chat.Enabled() {
		active = locale.T(lang, "yes")
	}
	captcha := locale.T(lang, "no")
	if chat.CaptchaEnabled() {
		captcha = locale.T(lang, "yes")
	}
	greeting := locale.T(lang, "no")
	if chat.Greeting {
		greeting = locale.T(lang, "yes")
	}
	winnerLabel := locale.T(lang, "winner.disabled")
	if chat.WinnerEnabled() {
		winnerLabel = chat.Winner.String
	}
	return locale.T(lang, "status",
		"active", bot.EscapeMarkdown(active),
		"gab", bot.EscapeMarkdown(fmt.Sprint(chat.Random)),
		"pairs", bot.EscapeMarkdown(fmt.Sprint(pairs)),
		"captcha", bot.EscapeMarkdown(captcha),
		"greeting", bot.EscapeMarkdown(greeting),
		"winner", bot.EscapeMarkdown(winnerLabel),
		"lang", bot.EscapeMarkdown(lang),
		"version", bot.EscapeMarkdown(version.Version()),
	)
}
