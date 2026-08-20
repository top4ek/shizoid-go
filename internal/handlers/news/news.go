package news

import (
	"context"
	"database/sql"
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
	Command     = "news"
	Description = "Daily joke news issue for this chat"
)

func Handler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
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
	case "":
		telegram.Reply(ctx, b, update, currentStyleText(chat, lang))
	default:
		telegram.Reply(ctx, b, update, locale.T(lang, "news.usage"))
	}
}

func enable(ctx context.Context, b *bot.Bot, update *tgmodels.Update, chatID int64, style, lang string) {
	if !utils.IsChatAdmin(ctx, b, chatID, update.Message.From.ID) {
		telegram.Reply(ctx, b, update, locale.Random(lang, "nok"))
		return
	}
	if style == "" {
		style = locale.T(lang, "news.default")
	}
	if err := app.Store().Chats.SetNews(ctx, chatID, sql.NullString{String: style, Valid: true}); err != nil {
		logger.Instance().Error("news enable", zap.Error(err))
		return
	}
	telegram.Reply(ctx, b, update, locale.T(lang, "news.enabled", "style", telegram.FormatPlain(style)))
}

func disable(ctx context.Context, b *bot.Bot, update *tgmodels.Update, chatID int64, lang string) {
	if !utils.IsChatAdmin(ctx, b, chatID, update.Message.From.ID) {
		telegram.Reply(ctx, b, update, locale.Random(lang, "nok"))
		return
	}
	if err := app.Store().Chats.SetNews(ctx, chatID, sql.NullString{}); err != nil {
		logger.Instance().Error("news disable", zap.Error(err))
		return
	}
	telegram.Reply(ctx, b, update, locale.T(lang, "news.turned_off"))
}

func currentStyleText(chat *models.Chat, lang string) string {
	if chat.NewsEnabled() {
		return locale.T(lang, "news.current", "style", telegram.FormatPlain(chat.News.String))
	}
	return locale.T(lang, "news.none")
}
