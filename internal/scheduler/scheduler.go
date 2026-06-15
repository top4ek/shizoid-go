// Package scheduler runs periodic jobs: daily winner draw, idle pokes and
// maintenance tasks such as message history pruning.
package scheduler

import (
	"context"
	"database/sql"
	"math/rand/v2"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"shizoid/internal/app"
	"shizoid/internal/config"
	"shizoid/internal/handlers/captcha"
	"shizoid/internal/handlers/idle"
	"shizoid/internal/handlers/winner"
	"shizoid/internal/locale"
	"shizoid/internal/logger"
	"shizoid/internal/models"
)

// Start configures and launches the cron jobs. The returned Cron should be
// stopped on shutdown. Idle UTC window (9–20) is enforced inside idle.PokeChat.
func Start(b *bot.Bot) *cron.Cron {
	c := cron.New(cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)))

	if _, err := c.AddFunc(config.Environment.WinnerCron, func() { runWinners(b) }); err != nil {
		logger.Instance().Error("schedule winner", zap.Error(err))
	}
	if _, err := c.AddFunc(config.Environment.IdleCron, func() { runIdle(b) }); err != nil {
		logger.Instance().Error("schedule idle", zap.Error(err))
	}
	if _, err := c.AddFunc("@daily", runMessagePrune); err != nil {
		logger.Instance().Error("schedule message prune", zap.Error(err))
	}
	if _, err := c.AddFunc(config.Environment.MemoryCron, func() { runMemory() }); err != nil {
		logger.Instance().Error("schedule memory", zap.Error(err))
	}
	if _, err := c.AddFunc(config.Environment.CaptchaCron, func() { runCaptcha(b) }); err != nil {
		logger.Instance().Error("schedule captcha", zap.Error(err))
	}

	c.Start()
	return c
}

func runWinners(b *bot.Bot) {
	logger.Instance().Debug("cron: winners")
	if !app.Ready() {
		return
	}
	ctx := context.Background()
	chats, err := models.Chats.Active(ctx)
	if err != nil {
		logger.Instance().Error("winners: active chats", zap.Error(err))
		return
	}
	for _, chat := range chats {
		if !chat.WinnerEnabled() {
			continue
		}
		done, err := models.Winners.HasToday(ctx, chat.ID)
		if err != nil {
			logger.Instance().Error("winners: has today", zap.Error(err))
			continue
		}
		if done {
			continue
		}
		top, err := models.Participations.TopByScore(ctx, chat.ID, 3)
		if err != nil || len(top) == 0 {
			continue
		}
		chosen := top[rand.IntN(len(top))]
		inserted, err := models.Winners.Create(ctx, chat.ID, chosen.UserID)
		if err != nil {
			logger.Instance().Error("winners: create", zap.Error(err))
			continue
		}
		if !inserted {
			continue
		}
		if err := models.Participations.ResetScores(ctx, chat.ID); err != nil {
			logger.Instance().Error("winners: reset", zap.Error(err))
		}
		announceWinner(ctx, b, chat.ID, chat.Locale, winnerLabel(chat.Winner.String, chat.Locale),
			chosen.UserID, chosen.Username, chosen.Name)
	}
}

func announceWinner(ctx context.Context, b *bot.Bot, chatID int64, lang, label string, userID int64, username, name string) {
	entries, _ := models.Winners.TopOfYear(ctx, chatID, 10)
	text := locale.T(lang, "winner.winner",
		"name", bot.EscapeMarkdown(label),
		"user", winner.FormatWinnerUser(lang, userID, username, name),
		"top", winner.FormatTop(lang, entries))
	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:              chatID,
		Text:                text,
		ParseMode:           tgmodels.ParseModeMarkdown,
		DisableNotification: true,
		LinkPreviewOptions:  &tgmodels.LinkPreviewOptions{IsDisabled: bot.True()},
	}); err != nil {
		logger.Instance().Error("winners: announce", zap.Error(err))
	}
}

func winnerLabel(label, lang string) string {
	if label == "" {
		return locale.T(lang, "winner.default")
	}
	return label
}

func runIdle(b *bot.Bot) {
	logger.Instance().Debug("cron: idle")
	if !app.Ready() {
		return
	}
	ctx := context.Background()
	now := time.Now().UTC()
	chats, err := models.Chats.Active(ctx)
	if err != nil {
		logger.Instance().Error("idle: active chats", zap.Error(err))
		return
	}
	for _, chat := range chats {
		idle.PokeChat(ctx, b, chat, now)
	}
}

func runMemory() {
	logger.Instance().Debug("cron: memory")
	if !app.Ready() || app.Neural() == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	chats, err := models.Chats.Active(ctx)
	if err != nil {
		logger.Instance().Error("memory: active chats", zap.Error(err))
		return
	}
	for _, chat := range chats {
		existing := ""
		if chat.Memory.Valid {
			existing = chat.Memory.String
		}
		budget := summaryMessageBudget(existing)
		if budget <= 0 {
			continue
		}
		since := time.Time{}
		if chat.MemorySummarizedAt.Valid {
			since = chat.MemorySummarizedAt.Time
		}
		msgs, err := models.Messages.TextsSinceByBytes(ctx, chat.ID, since, budget)
		if err != nil || len(msgs) == 0 {
			continue
		}
		logger.Instance().Debug("memory summarize",
			zap.Int64("chat_id", chat.ID),
			zap.Int("messages", len(msgs)),
		)
		summary, err := app.Neural().Summarize(ctx, config.Environment.SummaryPrompt, existing, msgs)
		if err != nil || strings.TrimSpace(summary) == "" {
			continue
		}
		summary = truncateRunes(summary, 4096)
		if err := models.Chats.SetMemory(ctx, chat.ID, sql.NullString{String: summary, Valid: true}); err != nil {
			logger.Instance().Error("memory: store", zap.Error(err))
			continue
		}
		if err := models.Chats.SetMemorySummarizedAt(ctx, chat.ID, time.Now()); err != nil {
			logger.Instance().Error("memory: mark summarized", zap.Error(err))
		}
	}
}

func runCaptcha(b *bot.Bot) {
	logger.Instance().Debug("cron: captcha")
	captcha.ExpirePending(context.Background(), b)
}

func runMessagePrune() {
	logger.Instance().Debug("cron: message prune")
	if !app.Ready() {
		return
	}
	n, err := models.Messages.PruneByBytes(context.Background(), config.MaxReplyContextBytes)
	if err != nil {
		logger.Instance().Error("prune messages", zap.Error(err))
		return
	}
	if n > 0 {
		logger.Instance().Info("prune messages", zap.Int64("deleted", n))
	}
}

const summaryBudgetMargin = 256

func summaryMessageBudget(existing string) int {
	overhead := len(config.Environment.SummaryPrompt) + len("New messages:\n") + summaryBudgetMargin
	if existing != "" {
		overhead += len("Existing memory:\n") + len(existing) + len("\n\n")
	}
	return config.MaxSummaryContextBytes - overhead
}

func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max])
}
