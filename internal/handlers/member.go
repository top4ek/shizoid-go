package handlers

import (
	"context"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"go.uber.org/zap"

	"shizoid/internal/app"
	"shizoid/internal/handlers/captcha"
	"shizoid/internal/logger"
)

// ChatMemberHandler handles chat_member updates (join transitions).
func ChatMemberHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	cm := update.ChatMember
	if cm == nil || !isJoinTransition(cm.OldChatMember, cm.NewChatMember) {
		return
	}
	user, ok := memberUser(cm.NewChatMember)
	if !ok || user.IsBot {
		return
	}
	handleMembersJoined(ctx, b, cm.Chat.ID, []tgmodels.User{*user}, "chat_member")
}

func isJoinTransition(old, new tgmodels.ChatMember) bool {
	if !wasAbsent(old.Type) {
		return false
	}
	switch new.Type {
	case tgmodels.ChatMemberTypeMember:
		return true
	case tgmodels.ChatMemberTypeRestricted:
		return new.Restricted != nil && new.Restricted.IsMember
	default:
		return false
	}
}

func wasAbsent(t tgmodels.ChatMemberType) bool {
	return t == tgmodels.ChatMemberTypeLeft || t == tgmodels.ChatMemberTypeBanned
}

func memberUser(cm tgmodels.ChatMember) (*tgmodels.User, bool) {
	switch cm.Type {
	case tgmodels.ChatMemberTypeMember:
		if cm.Member != nil && cm.Member.User != nil {
			return cm.Member.User, true
		}
	case tgmodels.ChatMemberTypeRestricted:
		if cm.Restricted != nil && cm.Restricted.User != nil {
			return cm.Restricted.User, true
		}
	case tgmodels.ChatMemberTypeAdministrator:
		return &cm.Administrator.User, true
	case tgmodels.ChatMemberTypeOwner:
		if cm.Owner != nil && cm.Owner.User != nil {
			return cm.Owner.User, true
		}
	case tgmodels.ChatMemberTypeLeft:
		if cm.Left != nil && cm.Left.User != nil {
			return cm.Left.User, true
		}
	case tgmodels.ChatMemberTypeBanned:
		if cm.Banned != nil && cm.Banned.User != nil {
			return cm.Banned.User, true
		}
	}
	return nil, false
}

// handleMembersJoined runs captcha and greeting for users who just joined.
func handleMembersJoined(ctx context.Context, b *bot.Bot, chatID int64, users []tgmodels.User, source string) {
	if !app.Enabled(ctx) {
		logger.Instance().Debug("join skip: chat disabled", zap.Int64("chat_id", chatID))
		return
	}
	chat := app.ChatFrom(ctx)
	if chat == nil {
		logger.Instance().Debug("join skip: chat missing from context", zap.Int64("chat_id", chatID))
		return
	}

	logger.Instance().Debug("join",
		zap.String("source", source),
		zap.Int64("chat_id", chatID),
		zap.Int("members_count", len(users)),
	)

	challenged := false
	for i := range users {
		member := users[i]
		if member.IsBot {
			logger.Instance().Debug("captcha skip: is_bot",
				zap.Int64("chat_id", chatID),
				zap.Int64("user_id", member.ID),
			)
			continue
		}
		if !chat.CaptchaEnabled() {
			continue
		}
		challenged = true
		captcha.OnMemberJoined(ctx, b, chatID, member)
	}
	if chat.CaptchaEnabled() && !challenged {
		logger.Instance().Debug("join skip: all members are bots", zap.Int64("chat_id", chatID))
	} else if !chat.CaptchaEnabled() {
		logger.Instance().Debug("join skip: captcha disabled", zap.Int64("chat_id", chatID))
	}
	if chat.Greeting {
		sendGreeting(ctx, b, chatID)
	}
}
