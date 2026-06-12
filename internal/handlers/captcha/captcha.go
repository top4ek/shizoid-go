// Package captcha gates newly joined members behind an emoji button challenge.
package captcha

import (
	"context"
	"database/sql"
	"strings"
	"time"

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
	Command     = "captcha"
	Description = "Enable/disable join captcha"
	HandlerType = bot.HandlerTypeMessageText
	MatchType   = bot.MatchTypeCommand

	CallbackPrefix = "captcha:"
	CallbackType   = bot.HandlerTypeCallbackQueryData
	CallbackMatch  = bot.MatchTypePrefix
)

// Handler toggles captcha for the chat (chat admins / bot owners only).
func Handler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	if update.Message == nil || update.Message.From == nil || !app.Enabled(ctx) || !app.Ready() {
		return
	}
	chatID := update.Message.Chat.ID
	lang := app.Locale(ctx)
	if !utils.IsChatAdmin(ctx, b, chatID, update.Message.From.ID) {
		telegram.Reply(ctx, b, update, locale.T(lang, "common.not_admin"), "")
		return
	}

	payload := strings.TrimSpace(utils.ExtractCommandPayloadText(update))
	cmd, _, _ := strings.Cut(payload, " ")
	switch strings.ToLower(cmd) {
	case "enable":
		if err := models.Chats.SetCaptcha(ctx, chatID, true); err != nil {
			logger.Instance().Error("captcha enable", zap.Error(err))
			return
		}
		telegram.Reply(ctx, b, update, locale.T(lang, "captcha.enabled"), "")
	case "disable":
		if err := models.Chats.SetCaptcha(ctx, chatID, false); err != nil {
			logger.Instance().Error("captcha disable", zap.Error(err))
			return
		}
		telegram.Reply(ctx, b, update, locale.T(lang, "captcha.disabled"), "")
	default:
		telegram.Reply(ctx, b, update, locale.T(lang, "captcha.usage"), "")
	}
}

// OnNewMembers mutes and challenges each unsolved new member.
func OnNewMembers(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	msg := update.Message
	chat := app.ChatFrom(ctx)
	if chat == nil {
		return
	}
	lang := app.Locale(ctx)
	for i := range msg.NewChatMembers {
		member := msg.NewChatMembers[i]
		if member.IsBot {
			continue
		}
		if err := challengeMember(ctx, b, msg.Chat.ID, lang, member); err != nil {
			logger.Instance().Error("captcha challenge", zap.Int64("user_id", member.ID), zap.Error(err))
		}
	}
}

func challengeMember(ctx context.Context, b *bot.Bot, chatID int64, lang string, member tgmodels.User) error {
	if app.Ready() {
		global, err := models.Users.CaptchaSolved(ctx, member.ID)
		if err != nil {
			return err
		}
		if global {
			return models.Participations.MarkCaptchaSolved(ctx, chatID, member.ID)
		}
		solved, err := models.Participations.CaptchaSolved(ctx, chatID, member.ID)
		if err != nil {
			return err
		}
		if solved {
			return nil
		}
	}

	correct, buttons, err := buildChallenge(lang)
	if err != nil {
		return err
	}

	telegram.Mute(ctx, b, chatID, member.ID)

	text := locale.T(lang, "captcha.message",
		"user", formatUserLink(member),
		"word", bot.EscapeMarkdown(correct.Word),
	)
	row := make([]tgmodels.InlineKeyboardButton, len(buttons))
	for i, sym := range buttons {
		row[i] = tgmodels.InlineKeyboardButton{
			Text:         sym.Emoji,
			CallbackData: callbackData(member.ID, sym.Emoji),
		}
	}
	kb := &tgmodels.InlineKeyboardMarkup{InlineKeyboard: [][]tgmodels.InlineKeyboardButton{row}}

	sent, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   tgmodels.ParseModeMarkdown,
		ReplyMarkup: kb,
	})
	if err != nil {
		return err
	}

	if app.Ready() && sent != nil {
		return models.Participations.StartCaptcha(ctx, chatID, member.ID, correct.Emoji, sent.ID)
	}
	return nil
}

// Callback verifies a captcha button press and unmutes the solver.
func Callback(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	cq := update.CallbackQuery
	if cq == nil || cq.Message.Message == nil {
		return
	}

	targetID, pressedEmoji, ok := parseCallback(cq.Data)
	if !ok {
		logger.Instance().Warn("captcha callback parse", zap.String("data", cq.Data))
		return
	}

	chatID := cq.Message.Message.Chat.ID

	if cq.From.ID != targetID {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: cq.ID})
		return
	}

	lang := callbackLocale(ctx, chatID)

	if !app.Ready() {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID,
			Text:            locale.T(lang, "common.error"),
			ShowAlert:       true,
		})
		return
	}

	correctEmoji, messageID, pending, err := models.Participations.GetCaptchaPending(ctx, chatID, targetID)
	if err != nil {
		logger.Instance().Error("captcha get pending", zap.Error(err))
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID,
			Text:            locale.T(lang, "common.error"),
			ShowAlert:       true,
		})
		return
	}
	if !pending {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: cq.ID})
		return
	}

	if pressedEmoji != correctEmoji {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID,
			Text:            locale.T(lang, "captcha.wrong"),
			ShowAlert:       true,
		})
		failCaptcha(ctx, b, chatID, targetID, messageID)
		return
	}

	user := userFromTelegram(&cq.From)
	if err := models.Ingest.EnsureMember(ctx, chatID, user); err != nil {
		logger.Instance().Error("captcha ensure member", zap.Error(err))
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID,
			Text:            locale.T(lang, "common.error"),
			ShowAlert:       true,
		})
		return
	}
	if err := models.Participations.MarkCaptchaSolved(ctx, chatID, targetID); err != nil {
		logger.Instance().Error("captcha mark participation", zap.Error(err))
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID,
			Text:            locale.T(lang, "common.error"),
			ShowAlert:       true,
		})
		return
	}
	if err := models.Users.MarkCaptchaSolved(ctx, targetID); err != nil {
		logger.Instance().Error("captcha mark user", zap.Error(err))
	}

	telegram.Unmute(ctx, b, chatID, targetID)
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: cq.ID,
		Text:            locale.T(lang, "captcha.solved"),
	})
	deleteCaptchaMessage(ctx, b, chatID, cq.Message.Message.ID)
}

// ExpirePending kicks users with captcha challenges past the timeout.
func ExpirePending(ctx context.Context, b *bot.Bot) {
	if !app.Ready() {
		return
	}
	pending, err := models.Participations.ExpiredPending(ctx, time.Minute)
	if err != nil {
		logger.Instance().Error("captcha expired pending", zap.Error(err))
		return
	}
	for _, p := range pending {
		failCaptcha(ctx, b, p.ChatID, p.UserID, p.MessageID)
	}
}

func failCaptcha(ctx context.Context, b *bot.Bot, chatID, userID int64, messageID int) {
	telegram.Kick(ctx, b, chatID, userID)
	if messageID != 0 {
		deleteCaptchaMessage(ctx, b, chatID, messageID)
	}
	if app.Ready() {
		if err := models.Participations.ClearCaptcha(ctx, chatID, userID); err != nil {
			logger.Instance().Error("captcha clear", zap.Error(err))
		}
	}
}

func deleteCaptchaMessage(ctx context.Context, b *bot.Bot, chatID int64, messageID int) {
	_, _ = b.DeleteMessage(ctx, &bot.DeleteMessageParams{ChatID: chatID, MessageID: messageID})
}

func callbackLocale(ctx context.Context, chatID int64) string {
	if chat := app.ChatFrom(ctx); chat != nil {
		return app.Locale(ctx)
	}
	if !app.Ready() {
		return app.Locale(ctx)
	}
	chat, err := models.Chats.Get(ctx, chatID)
	if err != nil || chat == nil {
		return app.Locale(ctx)
	}
	return app.Locale(app.WithChat(ctx, chat))
}

func userFromTelegram(u *tgmodels.User) *models.User {
	m := &models.User{ID: u.ID}
	m.IsBot.Bool, m.IsBot.Valid = u.IsBot, true
	if u.FirstName != "" {
		m.FirstName = sql.NullString{String: u.FirstName, Valid: true}
	}
	if u.LastName != "" {
		m.LastName = sql.NullString{String: u.LastName, Valid: true}
	}
	if u.Username != "" {
		m.Username = sql.NullString{String: u.Username, Valid: true}
	}
	if u.LanguageCode != "" {
		m.LanguageCode = sql.NullString{String: u.LanguageCode, Valid: true}
	}
	return m
}

func formatUserLink(u tgmodels.User) string {
	return utils.UserMarkdownLink(
		u.ID,
		u.Username,
		utils.DisplayName(u.Username, u.FirstName, u.LastName),
	)
}
