package telegram

import (
	"context"
	"time"
	"unicode/utf8"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"go.uber.org/zap"

	"shizoid/internal/app"
	"shizoid/internal/logger"
	"shizoid/internal/models"
	"shizoid/internal/sentry"
)

const maxMessageRunes = 4096

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

func Reply(ctx context.Context, b *bot.Bot, update *tgmodels.Update, text string, disableLinkPreview ...bool) {
	if update.Message == nil {
		return
	}
	opts := ChatMessageOpts{ReplyToMessageID: update.Message.ID}
	if len(disableLinkPreview) > 0 && disableLinkPreview[0] {
		opts.DisableLinkPreview = true
	}
	SendFromUpdate(ctx, b, update, text, opts)
}

func Send(ctx context.Context, b *bot.Bot, update *tgmodels.Update, text string, replyToMessageID int, disableLinkPreview ...bool) {
	if update.Message == nil {
		return
	}
	opts := ChatMessageOpts{ReplyToMessageID: replyToMessageID}
	if len(disableLinkPreview) > 0 && disableLinkPreview[0] {
		opts.DisableLinkPreview = true
	}
	SendFromUpdate(ctx, b, update, text, opts)
}

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
	return truncateMessage(SanitizeV2(text))
}

func truncateMessage(text string) string {
	if utf8.RuneCountInString(text) <= maxMessageRunes {
		return text
	}
	r := []rune(text)
	return string(r[:maxMessageRunes])
}

func persistBotMessage(ctx context.Context, chatID int64, text string) {
	if text == "" || !app.Ready() || app.SkipMessageHistory(ctx) {
		return
	}
	botID := app.BotID()
	if botID == 0 {
		return
	}
	bg, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := models.Messages.Append(bg, chatID, botID, text); err != nil {
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
