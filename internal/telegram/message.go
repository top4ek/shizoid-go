package telegram

import (
	"context"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"go.uber.org/zap"

	"shizoid/internal/app"
	"shizoid/internal/logger"
	"shizoid/internal/sentry"
)

// MaxMessageRunes is Telegram's per-message limit. Text over it is cut, so a
// caller that assembles a message out of blocks has to budget against it.
const MaxMessageRunes = 4096

// ChatMessageOpts configures outbound chat messages.
type ChatMessageOpts struct {
	MessageThreadID     int
	ReplyToMessageID    int
	ReplyMarkup         tgmodels.ReplyMarkup
	DisableNotification bool
	DisableLinkPreview  bool
}

func Typing(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	if update.Message == nil {
		return
	}
	_, err := b.SendChatAction(ctx, &bot.SendChatActionParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Action:          tgmodels.ChatActionTyping,
	})
	if err != nil {
		logger.Instance().Warn("send typing action", zap.Error(err))
		sentry.Capture(err)
	}
}

// Reply answers the message that triggered the handler.
func Reply(ctx context.Context, b *bot.Bot, update *tgmodels.Update, text string) {
	if update.Message == nil {
		return
	}
	SendFromUpdate(ctx, b, update, text, ChatMessageOpts{ReplyToMessageID: update.Message.ID})
}

// ReplyNoPreview is Reply for text carrying a @mention or link, whose preview
// card Telegram would otherwise render (and turn into a tappable button).
func ReplyNoPreview(ctx context.Context, b *bot.Bot, update *tgmodels.Update, text string) {
	if update.Message == nil {
		return
	}
	SendFromUpdate(ctx, b, update, text, ChatMessageOpts{
		ReplyToMessageID:   update.Message.ID,
		DisableLinkPreview: true,
	})
}

// Impersonate posts text as the bot and removes the command that asked for it,
// so the result reads as if the bot spoke on its own.
func Impersonate(ctx context.Context, b *bot.Bot, update *tgmodels.Update, text string, opts ChatMessageOpts) {
	if update.Message == nil {
		return
	}
	if update.Message.ReplyToMessage != nil {
		opts.ReplyToMessageID = update.Message.ReplyToMessage.ID
	}
	SendFromUpdate(ctx, b, update, text, opts)
	Delete(ctx, b, update.Message.Chat.ID, update.Message.ID)
}

// SendFromUpdate sends into the chat (and forum topic) the update came from.
// It never adds a reply target of its own: Impersonate deletes the triggering
// message, so replying to it would leave a dangling reply.
func SendFromUpdate(ctx context.Context, b *bot.Bot, update *tgmodels.Update, text string, opts ChatMessageOpts) {
	if update.Message == nil {
		return
	}
	opts.MessageThreadID = update.Message.MessageThreadID
	_, _ = SendToChat(ctx, b, update.Message.Chat.ID, text, opts)
}

func SendToChat(ctx context.Context, b *bot.Bot, chatID int64, text string, opts ChatMessageOpts) (*tgmodels.Message, error) {
	text = prepareOutboundText(text)
	params := &bot.SendMessageParams{
		ChatID:              chatID,
		MessageThreadID:     opts.MessageThreadID,
		Text:                text,
		ParseMode:           tgmodels.ParseModeMarkdown,
		DisableNotification: opts.DisableNotification,
		ReplyMarkup:         opts.ReplyMarkup,
	}
	if opts.ReplyToMessageID != 0 {
		params.ReplyParameters = &tgmodels.ReplyParameters{
			MessageID: opts.ReplyToMessageID,
		}
	}
	if opts.DisableLinkPreview {
		params.LinkPreviewOptions = &tgmodels.LinkPreviewOptions{IsDisabled: bot.True()}
	}
	logger.Instance().Debug("send message",
		zap.Int64("chat_id", chatID),
		zap.String("text", logger.TruncateLogText(text)),
		zap.Int("text_len", len(text)),
		zap.Int("reply_to", opts.ReplyToMessageID),
	)
	sent, err := b.SendMessage(ctx, params)
	if err != nil {
		logger.Instance().Error("send message",
			zap.Error(err),
			zap.Int64("chat_id", chatID),
			zap.Int("text_len", len(text)),
			zap.Int("text_runes", utf8.RuneCountInString(text)),
		)
		sentry.Capture(err)
		return nil, err
	}
	persistBotMessage(ctx, chatID, text)
	return sent, nil
}

func prepareOutboundText(text string) string {
	return FitV2(text, MaxMessageRunes)
}

// FitV2 sanitizes text as MarkdownV2 and cuts it until the sanitized result fits
// maxRunes. The cut is on the raw text and stepped down proportionally: escaping
// grows the text by an amount only the sanitizer knows, so where the raw cut has
// to fall cannot be computed in one go.
func FitV2(text string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	out := SanitizeV2(text)
	if utf8.RuneCountInString(out) <= maxRunes {
		return out
	}
	raw := []rune(text)
	limit := maxRunes
	if limit > len(raw) {
		limit = len(raw)
	}
	for {
		out = SanitizeV2(string(raw[:limit]))
		n := utf8.RuneCountInString(out)
		if n <= maxRunes || limit == 0 {
			return out
		}
		next := limit * maxRunes / n
		if next >= limit {
			next = limit - 1
		}
		limit = next
	}
}

// IsPermanentError reports whether the Telegram API refused a call for a reason
// retrying cannot fix: the bot was removed, the chat is gone or was migrated,
// the request itself is malformed. Callers that own a retry loop drop the work
// instead of spending the rest of its lifetime on the same answer.
func IsPermanentError(err error) bool {
	if err == nil {
		return false
	}
	if bot.IsMigrateError(err) {
		return true
	}
	return errors.Is(err, bot.ErrorForbidden) ||
		errors.Is(err, bot.ErrorBadRequest) ||
		errors.Is(err, bot.ErrorUnauthorized) ||
		errors.Is(err, bot.ErrorNotFound)
}

func persistBotMessage(ctx context.Context, chatID int64, text string) {
	if text == "" || app.SkipMessageHistory(ctx) {
		return
	}
	botID := app.BotID()
	if botID == 0 {
		return
	}
	bg, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := app.Store().Messages.Append(bg, chatID, botID, text); err != nil {
		logger.Instance().Error("persist bot message", zap.Error(err))
	}
}

func Delete(ctx context.Context, b *bot.Bot, chatID int64, messageID int) {
	_, err := b.DeleteMessage(ctx, &bot.DeleteMessageParams{
		ChatID:    chatID,
		MessageID: messageID,
	})
	if err != nil {
		logger.Instance().Error("delete message", zap.Error(err))
		sentry.Capture(err)
	}
}
