// Package captcha gates newly joined members behind an emoji button challenge.
package captcha

import (
	"context"
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

	CallbackPrefix = "captcha:"
	CallbackType   = bot.HandlerTypeCallbackQueryData
	CallbackMatch  = bot.MatchTypePrefix
)

// Handler toggles captcha for the chat (chat admins / bot owners only).
func Handler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	lang := app.Locale(ctx)

	verb, _ := utils.CutSubcommand(update)
	switch verb {
	case "enable":
		if err := app.Store().Chats.SetCaptcha(ctx, chatID, true); err != nil {
			logger.Instance().Error("captcha enable", zap.Error(err))
			return
		}
		telegram.Reply(ctx, b, update, locale.T(lang, "captcha.enabled"))
	case "disable":
		if err := app.Store().Chats.SetCaptcha(ctx, chatID, false); err != nil {
			logger.Instance().Error("captcha disable", zap.Error(err))
			return
		}
		telegram.Reply(ctx, b, update, locale.T(lang, "captcha.disabled"))
	default:
		telegram.Reply(ctx, b, update, locale.T(lang, "captcha.usage"))
	}
}

// OnMemberJoined challenges a single member who just joined the chat.
func OnMemberJoined(ctx context.Context, b *bot.Bot, chatID int64, member tgmodels.User) {
	lang := app.Locale(ctx)
	if err := challengeMember(ctx, b, chatID, lang, member); err != nil {
		logger.Instance().Error("captcha challenge", zap.Int64("user_id", member.ID), zap.Error(err))
	}
}

// challengeMember posts a captcha for one member who just joined, unless they
// have already solved one or another delivery of the same join already claimed
// the challenge.
func challengeMember(ctx context.Context, b *bot.Bot, chatID int64, lang string, member tgmodels.User) error {
	claimed, err := claimChallenge(ctx, chatID, member.ID)
	if err != nil || !claimed {
		return err
	}

	logger.Instance().Debug("captcha challenge: start",
		zap.Int64("chat_id", chatID),
		zap.Int64("user_id", member.ID),
	)

	correct, sent, err := sendChallenge(ctx, b, chatID, lang, member)
	if err != nil {
		// The claim above is what stops a second challenge; release it so a
		// later join attempt is not silently swallowed.
		_ = app.Store().Participations.ClearCaptcha(ctx, chatID, member.ID)
		return err
	}

	logger.Instance().Debug("captcha challenge: sent",
		zap.Int64("chat_id", chatID),
		zap.Int64("user_id", member.ID),
		zap.Int("message_id", sent.ID),
	)
	if err := app.Store().Participations.SetCaptchaDetails(ctx, chatID, member.ID, correct.Emoji, sent.ID); err != nil {
		return err
	}
	logger.Instance().Debug("captcha challenge: persisted",
		zap.Int64("chat_id", chatID),
		zap.Int64("user_id", member.ID),
	)
	return nil
}

// claimChallenge reports whether this caller should post the challenge: false
// when the member already solved a captcha (globally or in this chat) or when a
// concurrent join already claimed it.
func claimChallenge(ctx context.Context, chatID, userID int64) (bool, error) {
	skip := func(reason string) {
		logger.Instance().Debug("captcha skip: "+reason,
			zap.Int64("chat_id", chatID),
			zap.Int64("user_id", userID),
		)
	}

	global, err := app.Store().Users.CaptchaSolved(ctx, userID)
	if err != nil {
		return false, err
	}
	if global {
		skip("global_solved")
		return false, app.Store().Participations.MarkCaptchaSolved(ctx, chatID, userID)
	}

	solved, err := app.Store().Participations.CaptchaSolved(ctx, chatID, userID)
	if err != nil {
		return false, err
	}
	if solved {
		skip("chat_solved")
		return false, nil
	}

	claimed, err := app.Store().Participations.TryClaimCaptcha(ctx, chatID, userID)
	if err != nil {
		return false, err
	}
	if !claimed {
		skip("duplicate")
	}
	return claimed, nil
}

// sendChallenge mutes the member and posts the emoji keyboard, returning the
// expected answer and the message carrying it.
func sendChallenge(ctx context.Context, b *bot.Bot, chatID int64, lang string, member tgmodels.User) (locale.Symbol, *tgmodels.Message, error) {
	correct, buttons, err := buildChallenge(lang)
	if err != nil {
		return correct, nil, err
	}

	telegram.Mute(ctx, b, chatID, member.ID)

	row := make([]tgmodels.InlineKeyboardButton, len(buttons))
	for i, sym := range buttons {
		row[i] = tgmodels.InlineKeyboardButton{
			Text:         sym.Emoji,
			CallbackData: callbackData(member.ID, sym.Emoji),
		}
	}
	text := locale.T(lang, "captcha.message",
		"user", formatUserLink(member),
		"word", telegram.FormatPlain(correct.Word),
	)
	sent, err := telegram.SendToChat(ctx, b, chatID, text, telegram.ChatMessageOpts{
		ReplyMarkup:        &tgmodels.InlineKeyboardMarkup{InlineKeyboard: [][]tgmodels.InlineKeyboardButton{row}},
		DisableLinkPreview: true,
	})
	return correct, sent, err
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

	// Anyone can see the buttons; only the challenged member's press counts.
	if cq.From.ID != targetID {
		answerSilently(ctx, b, cq)
		return
	}

	lang := callbackLocale(ctx, chatID)

	correctEmoji, messageID, pending, err := app.Store().Participations.GetCaptchaPending(ctx, chatID, targetID)
	if err != nil {
		logger.Instance().Error("captcha get pending", zap.Error(err))
		answerError(ctx, b, cq, lang)
		return
	}
	if !pending {
		answerSilently(ctx, b, cq)
		return
	}

	if pressedEmoji != correctEmoji {
		answerAlert(ctx, b, cq, locale.T(lang, "captcha.wrong"))
		failCaptcha(ctx, b, chatID, targetID, messageID)
		return
	}

	acceptSolution(ctx, b, cq, chatID, targetID, lang)
}

// acceptSolution records the solve, unmutes the member and clears the challenge
// message.
func acceptSolution(ctx context.Context, b *bot.Bot, cq *tgmodels.CallbackQuery, chatID, targetID int64, lang string) {
	user := models.UserFromTelegram(&cq.From)
	if err := app.Store().Ingest.EnsureMember(ctx, chatID, user); err != nil {
		logger.Instance().Error("captcha ensure member", zap.Error(err))
		answerError(ctx, b, cq, lang)
		return
	}
	if err := app.Store().Participations.MarkCaptchaSolved(ctx, chatID, targetID); err != nil {
		logger.Instance().Error("captcha mark participation", zap.Error(err))
		answerError(ctx, b, cq, lang)
		return
	}
	// Best-effort: the chat-level solve above is what unblocks the member.
	if err := app.Store().Users.MarkCaptchaSolved(ctx, targetID); err != nil {
		logger.Instance().Error("captcha mark user", zap.Error(err))
	}

	telegram.Unmute(ctx, b, chatID, targetID)
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: cq.ID,
		Text:            locale.T(lang, "captcha.solved"),
	})
	deleteCaptchaMessage(ctx, b, chatID, cq.Message.Message.ID)
	logger.Instance().Debug("captcha solved",
		zap.Int64("chat_id", chatID),
		zap.Int64("user_id", targetID),
	)
}

// answerSilently dismisses the button's spinner without showing anything.
func answerSilently(ctx context.Context, b *bot.Bot, cq *tgmodels.CallbackQuery) {
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: cq.ID})
}

func answerAlert(ctx context.Context, b *bot.Bot, cq *tgmodels.CallbackQuery, text string) {
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: cq.ID,
		Text:            text,
		ShowAlert:       true,
	})
}

func answerError(ctx context.Context, b *bot.Bot, cq *tgmodels.CallbackQuery, lang string) {
	answerAlert(ctx, b, cq, locale.T(lang, "common.error"))
}

// ExpirePending kicks users with captcha challenges past the timeout.
func ExpirePending(ctx context.Context, b *bot.Bot) {
	pending, err := app.Store().Participations.ExpiredPending(ctx, time.Minute)
	if err != nil {
		logger.Instance().Error("captcha expired pending", zap.Error(err))
		return
	}
	for _, p := range pending {
		failCaptcha(ctx, b, p.ChatID, p.UserID, p.MessageID)
		logger.Instance().Debug("captcha expired",
			zap.Int64("chat_id", p.ChatID),
			zap.Int64("user_id", p.UserID),
		)
	}
}

func failCaptcha(ctx context.Context, b *bot.Bot, chatID, userID int64, messageID int) {
	telegram.Kick(ctx, b, chatID, userID)
	if messageID != 0 {
		deleteCaptchaMessage(ctx, b, chatID, messageID)
	}
	if err := app.Store().Participations.ClearCaptcha(ctx, chatID, userID); err != nil {
		logger.Instance().Error("captcha clear", zap.Error(err))
	}
	logger.Instance().Debug("captcha failed",
		zap.Int64("chat_id", chatID),
		zap.Int64("user_id", userID),
	)
}

func deleteCaptchaMessage(ctx context.Context, b *bot.Bot, chatID int64, messageID int) {
	_, _ = b.DeleteMessage(ctx, &bot.DeleteMessageParams{ChatID: chatID, MessageID: messageID})
}

func callbackLocale(ctx context.Context, chatID int64) string {
	if chat := app.ChatFrom(ctx); chat != nil {
		return app.Locale(ctx)
	}
	chat, err := app.Store().Chats.Get(ctx, chatID)
	if err != nil || chat == nil {
		return app.Locale(ctx)
	}
	return app.Locale(app.WithChat(ctx, chat))
}

func formatUserLink(u tgmodels.User) string {
	return utils.UserMarkdownLink(
		u.ID,
		u.Username,
		utils.DisplayName(u.Username, u.FirstName, u.LastName),
	)
}
