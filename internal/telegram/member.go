package telegram

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"go.uber.org/zap"

	"shizoid/internal/logger"
	"shizoid/internal/sentry"
)

func Mute(ctx context.Context, b *bot.Bot, chatID, userID int64) {
	if _, err := b.RestrictChatMember(ctx, &bot.RestrictChatMemberParams{
		ChatID:      chatID,
		UserID:      userID,
		Permissions: &models.ChatPermissions{},
	}); err != nil {
		logger.Instance().Error("mute chat member",
			zap.Error(err),
			zap.Int64("chat_id", chatID),
			zap.Int64("user_id", userID),
		)
		sentry.Capture(err)
	}
}

// IsMuted reports whether the user cannot send messages in the chat.
func IsMuted(ctx context.Context, b *bot.Bot, chatID, userID int64) bool {
	member, err := b.GetChatMember(ctx, &bot.GetChatMemberParams{
		ChatID: chatID,
		UserID: userID,
	})
	if err != nil {
		return false
	}
	if member.Type == models.ChatMemberTypeRestricted && member.Restricted != nil {
		return !member.Restricted.CanSendMessages
	}
	return false
}

// Kick removes a user from the chat only when they are muted (captcha safety check).
func Kick(ctx context.Context, b *bot.Bot, chatID, userID int64) bool {
	if !IsMuted(ctx, b, chatID, userID) {
		return false
	}
	if _, err := b.BanChatMember(ctx, &bot.BanChatMemberParams{
		ChatID: chatID,
		UserID: userID,
	}); err != nil {
		logger.Instance().Error("kick chat member ban",
			zap.Error(err),
			zap.Int64("chat_id", chatID),
			zap.Int64("user_id", userID),
		)
		sentry.Capture(err)
		return false
	}
	if _, err := b.UnbanChatMember(ctx, &bot.UnbanChatMemberParams{
		ChatID:         chatID,
		UserID:         userID,
		OnlyIfBanned:   true,
	}); err != nil {
		logger.Instance().Error("kick chat member unban",
			zap.Error(err),
			zap.Int64("chat_id", chatID),
			zap.Int64("user_id", userID),
		)
		sentry.Capture(err)
	}
	return true
}

func Unmute(ctx context.Context, b *bot.Bot, chatID, userID int64) {
	perms := &models.ChatPermissions{
		CanSendMessages:       true,
		CanSendAudios:         true,
		CanSendDocuments:      true,
		CanSendPhotos:         true,
		CanSendVideos:         true,
		CanSendVideoNotes:     true,
		CanSendVoiceNotes:     true,
		CanSendPolls:          true,
		CanSendOtherMessages:  true,
		CanAddWebPagePreviews: true,
	}
	if _, err := b.RestrictChatMember(ctx, &bot.RestrictChatMemberParams{
		ChatID:      chatID,
		UserID:      userID,
		Permissions: perms,
	}); err != nil {
		logger.Instance().Error("unmute chat member",
			zap.Error(err),
			zap.Int64("chat_id", chatID),
			zap.Int64("user_id", userID),
		)
		sentry.Capture(err)
	}
}
