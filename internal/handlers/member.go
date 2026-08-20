package handlers

import (
	"context"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"go.uber.org/zap"

	"shizoid/internal/app"
	"shizoid/internal/handlers/captcha"
	"shizoid/internal/handlers/greeting"
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

	humans := make([]tgmodels.User, 0, len(users))
	for _, member := range users {
		if member.IsBot {
			logger.Instance().Debug("captcha skip: is_bot",
				zap.Int64("chat_id", chatID),
				zap.Int64("user_id", member.ID),
			)
			continue
		}
		humans = append(humans, member)
	}

	if chat.CaptchaEnabled() {
		if len(humans) == 0 {
			logger.Instance().Debug("join skip: all members are bots", zap.Int64("chat_id", chatID))
		}
		for _, member := range humans {
			captcha.OnMemberJoined(ctx, b, chatID, member)
		}
	} else {
		logger.Instance().Debug("join skip: captcha disabled", zap.Int64("chat_id", chatID))
	}

	if chat.Greeting {
		sendGreetingOrRollback(ctx, b, chatID, claimGreetings(ctx, chatID, humans))
	}
}

// claimGreetings returns the members this delivery of the join won the greeting
// for; a concurrent delivery claiming first is the normal case, not an error.
func claimGreetings(ctx context.Context, chatID int64, humans []tgmodels.User) []int64 {
	var claimedIDs []int64
	for _, member := range humans {
		claimed, err := greeting.OnMemberJoined(ctx, chatID, member)
		if err != nil {
			logger.Instance().Error("greeting claim", zap.Int64("user_id", member.ID), zap.Error(err))
			continue
		}
		if claimed {
			claimedIDs = append(claimedIDs, member.ID)
		}
	}
	return claimedIDs
}

// sendGreetingOrRollback posts one greeting for the whole batch, releasing every
// claim if nothing went out so a later join can greet these members instead.
func sendGreetingOrRollback(ctx context.Context, b *bot.Bot, chatID int64, claimedIDs []int64) {
	if len(claimedIDs) == 0 {
		return
	}
	sent, err := greeting.Send(ctx, b, chatID)
	if err != nil {
		logger.Instance().Error("greeting send", zap.Int64("chat_id", chatID), zap.Error(err))
	}
	if sent && err == nil {
		return
	}
	for _, uid := range claimedIDs {
		if clearErr := app.Store().Participations.ClearGreeting(ctx, chatID, uid); clearErr != nil {
			logger.Instance().Error("greeting clear", zap.Int64("user_id", uid), zap.Error(clearErr))
		}
	}
}
