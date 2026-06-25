package greeting

import (
	"context"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"go.uber.org/zap"

	"shizoid/internal/app"
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

// Send posts the configured greeting text to the chat. The second return value reports whether a message was sent.
func Send(ctx context.Context, b *bot.Bot, chatID int64) (bool, error) {
	if !app.Ready() {
		return false, nil
	}
	text, ok, err := models.Greetings.Get(ctx, chatID)
	if err != nil {
		return false, err
	}
	if !ok || text == "" {
		return false, nil
	}
	_, err = telegram.SendToChat(ctx, b, chatID, text, telegram.ChatMessageOpts{
		DisableLinkPreview: true,
	})
	if err != nil {
		return false, err
	}
	return true, nil
}
