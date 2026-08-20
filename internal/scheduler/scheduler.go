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
	"shizoid/internal/models"
	"shizoid/internal/telegram"
)

// Start configures and launches the cron jobs. The returned Cron should be
// stopped on shutdown.
func Start(b *bot.Bot) *cron.Cron {
	c := cron.New(cron.WithChain(
		cron.Recover(cron.DefaultLogger),
		cron.SkipIfStillRunning(cron.DefaultLogger),
	))

	for _, job := range []struct {
		name string
		spec string
		run  func()
	}{
		{"winner", config.Environment.WinnerCron, func() { runWinners(b) }},
		{"message prune", "@daily", runMessagePrune},
		{"memory", config.Environment.MemoryCron, runMemory},
		{"captcha", config.Environment.CaptchaCron, func() { runCaptcha(b) }},
		{"news", config.Environment.NewsCron, func() { runNews(b) }},
	} {
		if _, err := c.AddFunc(job.spec, job.run); err != nil {
			logger.Instance().Error("schedule "+job.name, zap.String("spec", job.spec), zap.Error(err))
		}
	}

	c.Start()
	return c
}

// forEachActiveChat runs fn over every active chat under a job-wide deadline,
// giving each chat its own slice of it so one slow chat cannot starve the chats
// after it in Active() order. fn reports whether it did any work; the count is
// logged when the job finishes.
func forEachActiveChat(name string, budget, perChat time.Duration, fn func(ctx context.Context, chat *models.Chat) bool) {
	logger.Instance().Debug("cron: " + name)
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()

	chats, err := app.Store().Chats.Active(ctx)
	if err != nil {
		logger.Instance().Error(name+": active chats", zap.Error(err))
		return
	}
	done := 0
	for _, chat := range chats {
		chatCtx, chatCancel := context.WithTimeout(ctx, perChat)
		if fn(chatCtx, chat) {
			done++
		}
		chatCancel()
	}
	logger.Instance().Info("cron: "+name+" done",
		zap.Int("chats", len(chats)),
		zap.Int("handled", done),
	)
}

func runWinners(b *bot.Bot) {
	forEachActiveChat("winners", 10*time.Minute, time.Minute, func(ctx context.Context, chat *models.Chat) bool {
		if !chat.WinnerEnabled() {
			return false
		}
		done, err := app.Store().Winners.HasToday(ctx, chat.ID)
		if err != nil {
			logger.Instance().Error("winners: has today", zap.Error(err))
			return false
		}
		if done {
			return false
		}
		top, err := app.Store().Participations.TopByScore(ctx, chat.ID, 3)
		if err != nil || len(top) == 0 {
			return false
		}
		chosen := top[rand.IntN(len(top))]
		inserted, err := app.Store().Winners.Create(ctx, chat.ID, chosen.UserID)
		if err != nil {
			logger.Instance().Error("winners: create", zap.Error(err))
			return false
		}
		if !inserted {
			return false
		}
		if err := app.Store().Participations.ResetScores(ctx, chat.ID); err != nil {
			logger.Instance().Error("winners: reset", zap.Error(err))
		}
		announceWinner(ctx, b, chat.ID, chat.Locale, winnerLabel(chat.Winner.String, chat.Locale),
			chosen.UserID, chosen.Username, chosen.Name)
		return true
	})
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
	forEachActiveChat("memory", 10*time.Minute, summaryChatTimeout, func(ctx context.Context, chat *models.Chat) bool {
		existing := ""
		if chat.Memory.Valid {
			existing = chat.Memory.String
		}
		budget := summaryMessageBudget(existing)
		if budget <= 0 {
			return false
		}
		since := time.Time{}
		if chat.MemorySummarizedAt.Valid {
			since = chat.MemorySummarizedAt.Time
		}
		msgs, err := app.Store().Messages.TextsSinceByBytes(ctx, chat.ID, since, budget)
		if err != nil || len(msgs) == 0 {
			return false
		}
		logger.Instance().Debug("memory summarize",
			zap.Int64("chat_id", chat.ID),
			zap.Int("messages", len(msgs)),
		)
		summary, err := app.Neural().Summarize(ctx, config.Environment.SummaryPrompt, existing, msgs)
		if err != nil || strings.TrimSpace(summary) == "" {
			return false
		}
		summary = truncateRunes(summary, 4096)
		if err := app.Store().Chats.SetMemory(ctx, chat.ID, sql.NullString{String: summary, Valid: true}); err != nil {
			logger.Instance().Error("memory: store", zap.Error(err))
			return false
		}
		if err := app.Store().Chats.SetMemorySummarizedAt(ctx, chat.ID, time.Now()); err != nil {
			logger.Instance().Error("memory: mark summarized", zap.Error(err))
		}
		return true
	})
}

// summaryChatTimeout bounds one chat's turn on the summary chain, which runs
// over far more context than a reply and is the slowest call in the codebase.
const summaryChatTimeout = 5 * time.Minute

func runNews(b *bot.Bot) {
	if !app.Neural().SummaryConfigured() {
		return
	}
	now := time.Now().UTC()
	forEachActiveChat("news", 30*time.Minute, summaryChatTimeout, func(ctx context.Context, chat *models.Chat) bool {
		return news.PostChat(ctx, b, chat, now)
	})
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
