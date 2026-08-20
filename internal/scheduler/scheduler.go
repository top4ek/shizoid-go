// Package scheduler runs periodic jobs: daily winner draw and maintenance
// tasks such as message history pruning.
package scheduler

import (
	"context"
	"database/sql"
	"math/rand/v2"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-telegram/bot"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"shizoid/internal/app"
	"shizoid/internal/config"
	"shizoid/internal/handlers/captcha"
	"shizoid/internal/handlers/news"
	"shizoid/internal/handlers/winner"
	"shizoid/internal/locale"
	"shizoid/internal/logger"
	"shizoid/internal/telegram"
)

// Start configures and launches the cron jobs. The returned Cron should be
// stopped on shutdown.
func Start(b *bot.Bot) *cron.Cron {
	c := cron.New(cron.WithChain(
		cron.Recover(cron.DefaultLogger),
		cron.SkipIfStillRunning(cron.DefaultLogger),
	))

	if _, err := c.AddFunc(config.Environment.WinnerCron, func() { runWinners(b) }); err != nil {
		logger.Instance().Error("schedule winner", zap.Error(err))
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
	if _, err := c.AddFunc(config.Environment.NewsCron, func() { runNews(b) }); err != nil {
		logger.Instance().Error("schedule news", zap.Error(err))
	}

	c.Start()
	return c
}

func runWinners(b *bot.Bot) {
	logger.Instance().Debug("cron: winners")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	chats, err := app.Store().Chats.Active(ctx)
	if err != nil {
		logger.Instance().Error("winners: active chats", zap.Error(err))
		return
	}
	for _, chat := range chats {
		if !chat.WinnerEnabled() {
			continue
		}
		done, err := app.Store().Winners.HasToday(ctx, chat.ID)
		if err != nil {
			logger.Instance().Error("winners: has today", zap.Error(err))
			continue
		}
		if done {
			continue
		}
		top, err := app.Store().Participations.TopByScore(ctx, chat.ID, 3)
		if err != nil || len(top) == 0 {
			continue
		}
		chosen := top[rand.IntN(len(top))]
		inserted, err := app.Store().Winners.Create(ctx, chat.ID, chosen.UserID)
		if err != nil {
			logger.Instance().Error("winners: create", zap.Error(err))
			continue
		}
		if !inserted {
			continue
		}
		if err := app.Store().Participations.ResetScores(ctx, chat.ID); err != nil {
			logger.Instance().Error("winners: reset", zap.Error(err))
		}
		announceWinner(ctx, b, chat.ID, chat.Locale, winnerLabel(chat.Winner.String, chat.Locale),
			chosen.UserID, chosen.Username, chosen.Name)
	}
}

func announceWinner(ctx context.Context, b *bot.Bot, chatID int64, lang, label string, userID int64, username, name string) {
	entries, err := app.Store().Winners.TopOfYear(ctx, chatID, 10)
	if err != nil {
		logger.Instance().Error("winners: top of year", zap.Error(err))
	}
	text := locale.T(lang, "winner.winner",
		"name", telegram.FormatPlain(label),
		"user", winner.FormatWinnerUser(lang, userID, username, name),
		"top", winner.FormatTop(lang, entries))
	if _, err := telegram.SendToChat(ctx, b, chatID, text, telegram.ChatMessageOpts{
		DisableNotification: true,
		DisableLinkPreview:  true,
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

func runMemory() {
	logger.Instance().Debug("cron: memory")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	chats, err := app.Store().Chats.Active(ctx)
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
		msgs, err := app.Store().Messages.TextsSinceByBytes(ctx, chat.ID, since, budget)
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
		if err := app.Store().Chats.SetMemory(ctx, chat.ID, sql.NullString{String: summary, Valid: true}); err != nil {
			logger.Instance().Error("memory: store", zap.Error(err))
			continue
		}
		if err := app.Store().Chats.SetMemorySummarizedAt(ctx, chat.ID, time.Now()); err != nil {
			logger.Instance().Error("memory: mark summarized", zap.Error(err))
		}
	}
}

const newsChatTimeout = 5 * time.Minute

func runNews(b *bot.Bot) {
	logger.Instance().Debug("cron: news")
	if !app.Neural().SummaryConfigured() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	now := time.Now().UTC()
	chats, err := app.Store().Chats.Active(ctx)
	if err != nil {
		logger.Instance().Error("news: active chats", zap.Error(err))
		return
	}
	sent := 0
	for _, chat := range chats {
		// A per-chat deadline: generation runs over the summary chain with a full
		// day of log, so without one a slow chat eats the whole job's budget and
		// every chat after it in Active() order fails with a deadline error.
		chatCtx, chatCancel := context.WithTimeout(ctx, newsChatTimeout)
		if news.PostChat(chatCtx, b, chat, now) {
			sent++
		}
		chatCancel()
	}
	logger.Instance().Info("cron: news done", zap.Int("issues_sent", sent), zap.Int("chats", len(chats)))
}

func runCaptcha(b *bot.Bot) {
	logger.Instance().Debug("cron: captcha")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	captcha.ExpirePending(ctx, b)
}

func runMessagePrune() {
	logger.Instance().Debug("cron: message prune")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	chatIDs, err := app.Store().Messages.ChatIDs(ctx)
	if err != nil {
		logger.Instance().Error("prune messages: chat ids", zap.Error(err))
		return
	}
	var total int64
	for _, chatID := range chatIDs {
		n, err := app.Store().Messages.PruneChatByBytes(ctx, chatID, config.MaxReplyContextBytes)
		if err != nil {
			logger.Instance().Error("prune messages", zap.Int64("chat_id", chatID), zap.Error(err))
			continue
		}
		total += n
	}
	if total > 0 {
		logger.Instance().Info("prune messages", zap.Int64("deleted", total))
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
