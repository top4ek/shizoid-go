package idle

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"go.uber.org/zap"

	"shizoid/internal/app"
	"shizoid/internal/config"
	"shizoid/internal/locale"
	"shizoid/internal/logger"
	"shizoid/internal/models"
	"shizoid/internal/telegram"
	"shizoid/internal/utils"
)

// PokeChat sends a daily idle question when the chat's scheduled hour matches now.
func PokeChat(ctx context.Context, b *bot.Bot, chat *models.Chat, now time.Time) bool {
	if !chat.IdleDays.Valid || chat.IdleDays.Int64 < 1 {
		return false
	}
	if PokedTodayUTC(chat.IdlePokedAt, now) {
		return false
	}
	if now.UTC().Hour() != ScheduledHour(chat.ID, now) {
		return false
	}

	days := int(chat.IdleDays.Int64)
	inactivePool, err := models.Participations.InactiveSince(ctx, chat.ID, days)
	if err != nil {
		logger.Instance().Error("idle: inactive members", zap.Int64("chat_id", chat.ID), zap.Error(err))
		return false
	}
	if len(inactivePool) == 0 {
		return false
	}
	inactive := inactivePool[rand.IntN(len(inactivePool))]

	activePool, err := models.Participations.ActiveSince(ctx, chat.ID, days)
	if err != nil {
		logger.Instance().Error("idle: active members", zap.Int64("chat_id", chat.ID), zap.Error(err))
		return false
	}
	asker, ok := pickAsker(activePool, inactive)
	if !ok {
		return false
	}

	text := composePokeText(ctx, chat, *asker, inactive, days)
	if text == "" {
		return false
	}

	if _, err := telegram.SendToChat(ctx, b, chat.ID, text, telegram.ChatMessageOpts{
		DisableNotification: true,
		DisableLinkPreview:  true,
	}); err != nil {
		logger.Instance().Error("idle: send", zap.Int64("chat_id", chat.ID), zap.Error(err))
		return false
	}
	if err := models.Chats.SetIdlePokedAt(ctx, chat.ID, now); err != nil {
		logger.Instance().Error("idle: mark poked", zap.Int64("chat_id", chat.ID), zap.Error(err))
	}
	return true
}

func pickAsker(active []models.MemberInfo, inactive models.MemberInfo) (*models.MemberInfo, bool) {
	var pool []models.MemberInfo
	for _, m := range active {
		if m.UserID != inactive.UserID {
			pool = append(pool, m)
		}
	}
	if len(pool) == 0 {
		return nil, false
	}
	return &pool[rand.IntN(len(pool))], true
}

func composePokeText(ctx context.Context, chat *models.Chat, asker, inactive models.MemberInfo, days int) string {
	if chat.GenerationMode == models.GenerationModeNeural {
		n := app.Neural()
		if n != nil && n.ReplyConfigured() {
			system := buildIdleSystem(chat)
			user := buildIdleUserMessage(chat, asker, inactive, days)
			text, err := n.Reply(ctx, system, user)
			if err != nil {
				logger.Instance().Warn("idle: neural fallback to locale",
					zap.Int64("chat_id", chat.ID),
					zap.Error(err),
				)
			} else if text = strings.TrimSpace(text); text != "" {
				return text
			}
		}
	}
	return formatLocalePoke(chat.Locale, asker, inactive, days)
}

func buildIdleSystem(chat *models.Chat) string {
	var parts []string
	if p := strings.TrimSpace(config.Environment.IdlePrompt); p != "" {
		parts = append(parts, p)
	}
	if p := strings.TrimSpace(config.Environment.AppPrompt); p != "" {
		parts = append(parts, p)
	}
	if chat.SystemPrompt.Valid {
		if p := strings.TrimSpace(chat.SystemPrompt.String); p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, "\n\n")
}

func buildIdleUserMessage(chat *models.Chat, asker, inactive models.MemberInfo, days int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Chat locale: %s\n", chat.Locale)
	fmt.Fprintf(&b, "Inactive days threshold: %d\n", days)
	fmt.Fprintf(&b, "Active member (address them): %s", memberLabel(asker))
	if asker.Username != "" {
		fmt.Fprintf(&b, " @%s", asker.Username)
	}
	fmt.Fprintf(&b, " (id: %d)\n", asker.UserID)
	fmt.Fprintf(&b, "Inactive member (ask about): %s", memberLabel(inactive))
	if inactive.Username != "" {
		fmt.Fprintf(&b, " @%s", inactive.Username)
	}
	fmt.Fprintf(&b, " (id: %d)\n", inactive.UserID)
	return b.String()
}

func memberLabel(m models.MemberInfo) string {
	if m.Name != "" {
		return m.Name
	}
	return "Unknown"
}

func formatLocalePoke(lang string, asker, inactive models.MemberInfo, days int) string {
	template := locale.Random(lang, "idle")
	if template == "" {
		return ""
	}
	return interpolatePoke(telegram.FormatTemplate(template), map[string]any{
		"asker": formatMemberLink(lang, asker),
		"user":  formatMemberLink(lang, inactive),
		"days":  telegram.FormatPlain(fmt.Sprint(days)),
	})
}

func interpolatePoke(s string, vars map[string]any) string {
	for k, v := range vars {
		s = strings.ReplaceAll(s, "%{"+k+"}", fmt.Sprint(v))
	}
	return s
}

func formatMemberLink(lang string, m models.MemberInfo) string {
	name := m.Name
	if name == "" {
		name = locale.T(lang, "winner.default")
	}
	return utils.UserMarkdownLink(m.UserID, m.Username, name)
}
