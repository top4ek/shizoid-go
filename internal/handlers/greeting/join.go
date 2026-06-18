package greeting

import (
	"context"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"go.uber.org/zap"

	"shizoid/internal/app"
	"shizoid/internal/config"
	"shizoid/internal/logger"
	"shizoid/internal/models"
	"shizoid/internal/telegram"
)

// OnMemberJoined claims greeting for one member; returns true if the chat message should be sent.
func OnMemberJoined(ctx context.Context, chatID int64, member tgmodels.User) (bool, error) {
	if !app.Ready() {
		return false, nil
	}
	greeted, err := models.Participations.GreetingGreeted(ctx, chatID, member.ID)
	if err != nil {
		return false, err
	}
	if greeted {
		logger.Instance().Debug("greeting skip: already_greeted",
			zap.Int64("chat_id", chatID),
			zap.Int64("user_id", member.ID),
		)
		return false, nil
	}
	claimed, err := models.Participations.TryClaimGreeting(ctx, chatID, member.ID)
	if err != nil {
		return false, err
	}
	if !claimed {
		logger.Instance().Debug("greeting skip: duplicate",
			zap.Int64("chat_id", chatID),
			zap.Int64("user_id", member.ID),
		)
		return false, nil
	}
	return true, nil
}

// Send posts the configured greeting text to the chat.
func Send(ctx context.Context, b *bot.Bot, chatID int64) (messageID int, sent bool, err error) {
	if !app.Ready() {
		return 0, false, nil
	}
	chat := app.ChatFrom(ctx)
	if chat == nil || !chat.GreetingEnabled() {
		return 0, false, nil
	}
	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:             chatID,
		Text:               chat.GreetingText.String,
		LinkPreviewOptions: &tgmodels.LinkPreviewOptions{IsDisabled: bot.True()},
	})
	if err != nil {
		return 0, false, err
	}
	if msg == nil {
		return 0, false, nil
	}
	return msg.ID, true, nil
}

// ExpirePending deletes greeting messages past the configured TTL.
func ExpirePending(ctx context.Context, b *bot.Bot) {
	if !app.Ready() {
		return
	}
	pending, err := models.Participations.ExpiredGreeting(ctx, config.GreetingDeleteAfter)
	if err != nil {
		logger.Instance().Error("greeting expired pending", zap.Error(err))
		return
	}
	for _, p := range pending {
		if p.MessageID != 0 {
			telegram.Delete(ctx, b, p.ChatID, p.MessageID)
		}
		if err := models.Participations.ClearGreetingMessageID(ctx, p.ChatID, p.MessageID); err != nil {
			logger.Instance().Error("greeting clear message id", zap.Error(err))
		}
		logger.Instance().Debug("greeting expired",
			zap.Int64("chat_id", p.ChatID),
			zap.Int("message_id", p.MessageID),
		)
	}
}
