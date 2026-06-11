package winner

import (
	"context"
	"database/sql"
	"fmt"
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
	Command     = "winner"
	Description = "Daily winner draw and stats"
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
	payload := strings.TrimSpace(utils.ExtractCommandPayloadText(update))
	first, rest, _ := strings.Cut(payload, " ")

	switch strings.ToLower(first) {
	case "enable":
		enable(ctx, b, update, chat.ID, strings.TrimSpace(rest), lang)
	case "disable":
		disable(ctx, b, update, chat.ID, lang)
	case "current":
		telegram.Reply(ctx, b, update, currentStats(ctx, chat.ID, lang), tgmodels.ParseModeMarkdown)
	case "":
		telegram.Reply(ctx, b, update, previousWinner(ctx, chat.ID, lang), tgmodels.ParseModeMarkdown, true)
	default:
		telegram.Reply(ctx, b, update, locale.T(lang, "winner.usage"), "")
	}
}

func enable(ctx context.Context, b *bot.Bot, update *tgmodels.Update, chatID int64, label, lang string) {
	if !utils.IsChatAdmin(ctx, b, chatID, update.Message.From.ID) {
		telegram.Reply(ctx, b, update, locale.T(lang, "common.not_admin"), "")
		return
	}
	if label == "" {
		label = locale.T(lang, "winner.default")
	}
	if err := models.Chats.SetWinner(ctx, chatID, sql.NullString{String: label, Valid: true}); err != nil {
		logger.Instance().Error("winner enable", zap.Error(err))
		return
	}
	telegram.Reply(ctx, b, update, locale.T(lang, "winner.enabled", "name", label), "")
}

func disable(ctx context.Context, b *bot.Bot, update *tgmodels.Update, chatID int64, lang string) {
	if !utils.IsChatAdmin(ctx, b, chatID, update.Message.From.ID) {
		telegram.Reply(ctx, b, update, locale.T(lang, "common.not_admin"), "")
		return
	}
	if err := models.Chats.SetWinner(ctx, chatID, sql.NullString{}); err != nil {
		logger.Instance().Error("winner disable", zap.Error(err))
		return
	}
	telegram.Reply(ctx, b, update, locale.T(lang, "winner.turned_off"), "")
}

func markdownPlain(s string) string {
	return bot.EscapeMarkdown(s)
}

func currentStats(ctx context.Context, chatID int64, lang string) string {
	entries, err := models.Participations.TopByScore(ctx, chatID, 10)
	if err != nil {
		logger.Instance().Error("winner current", zap.Error(err))
		return markdownPlain(locale.T(lang, "winner.no_one"))
	}
	if len(entries) == 0 {
		return markdownPlain(locale.T(lang, "winner.no_one"))
	}
	return locale.T(lang, "winner.current", "top", FormatTop(lang, entries))
}

func previousWinner(ctx context.Context, chatID int64, lang string) string {
	userID, username, name, ok, err := models.Winners.LastWinner(ctx, chatID)
	if err != nil {
		logger.Instance().Error("winner last", zap.Error(err))
	}
	if !ok || name == "" {
		return markdownPlain(locale.T(lang, "winner.no_one"))
	}
	entries, err := models.Winners.TopOfYear(ctx, chatID, 10)
	if err != nil {
		logger.Instance().Error("winner top year", zap.Error(err))
	}
	chat := app.ChatFrom(ctx)
	label := locale.T(lang, "winner.default")
	if chat != nil && chat.Winner.Valid && chat.Winner.String != "" {
		label = chat.Winner.String
	}
	return locale.T(lang, "winner.winner",
		"name", bot.EscapeMarkdown(label),
		"user", FormatWinnerUser(lang, userID, username, name),
		"top", FormatTop(lang, entries))
}

// FormatWinnerUser renders the daily winner as a MarkdownV2 user link.
func FormatWinnerUser(lang string, userID int64, username, name string) string {
	if name == "" {
		name = locale.T(lang, "winner.default")
	}
	return utils.UserMarkdownLink(userID, username, name)
}

// FormatTop renders leaderboard entries into localized MarkdownV2 lines.
func FormatTop(lang string, entries []models.ScoreEntry) string {
	var lines []string
	for i, e := range entries {
		name := e.Name
		if name == "" {
			name = locale.T(lang, "winner.default")
		}
		lines = append(lines, fmt.Sprintf("*%s\\.* %s — %s",
			bot.EscapeMarkdown(fmt.Sprint(i+1)),
			bot.EscapeMarkdown(name),
			bot.EscapeMarkdown(fmt.Sprint(e.Score))))
	}
	return strings.Join(lines, "\n")
}
