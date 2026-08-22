// Package scheduler runs periodic jobs: daily winner draw and maintenance
// tasks such as message history pruning.
package scheduler

import (
	"context"
	"math/rand/v2"
	"time"
	"unicode/utf8"

	"github.com/go-telegram/bot"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"shizoid/internal/app"
	"shizoid/internal/config"
	"shizoid/internal/handlers/captcha"
	"shizoid/internal/handlers/winner"
	"shizoid/internal/logger"
	"shizoid/internal/models"
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
		{"summary queue", config.Environment.SummaryQueueCron, func() { runSummaryQueue(b) }},
		{"captcha", config.Environment.CaptchaCron, func() { runCaptcha(b) }},
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

// drawPool is how many of the day's top scorers the winner is drawn from, and
// statsTop how many the announcement reports on.
const (
	drawPool = 3
	statsTop = 10
)

func runWinners(b *bot.Bot) {
	now := time.Now().UTC()
	forEachActiveChat("winners", 30*time.Minute, time.Minute, func(ctx context.Context, chat *models.Chat) bool {
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
		// Read before ResetScores: these are the standings the announcement
		// reports on, the same ones /winner current shows.
		standings, err := app.Store().Participations.TopByScore(ctx, chat.ID, statsTop)
		if err != nil || len(standings) == 0 {
			return false
		}
		pool := standings
		if len(pool) > drawPool {
			pool = pool[:drawPool]
		}
		chosen := pool[rand.IntN(len(pool))]
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
		if err := winner.Announce(ctx, b, chat, chosen, standings, now); err != nil {
			logger.Instance().Error("winners: announce", zap.Int64("chat_id", chat.ID), zap.Error(err))
		}
		return true
	})
}

// runMemory queues a memory summary for every active chat. The work itself is
// done by the summary queue, so a model that is down only delays it.
func runMemory() {
	if !app.Neural().SummaryConfigured() {
		return
	}
	forEachActiveChat("memory", 10*time.Minute, time.Minute, func(ctx context.Context, chat *models.Chat) bool {
		if err := app.Store().SummaryJobs.Enqueue(ctx, chat.ID, models.SummaryJobMemory, nil, memoryTTL); err != nil {
			logger.Instance().Error("memory: enqueue", zap.Int64("chat_id", chat.ID), zap.Error(err))
			return false
		}
		return true
	})
}

// summaryChatTimeout bounds one job's turn on the summary chain, which runs over
// far more context than a reply and is the slowest call in the codebase.
const summaryChatTimeout = 5 * time.Minute

func runCaptcha(b *bot.Bot) {
	logger.Instance().Debug("cron: captcha")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	captcha.ExpirePending(ctx, b)
}

func runMessagePrune() {
	logger.Instance().Debug("cron: message prune")
	// While memory summarization is running, history it has not read yet has to
	// outlive the byte budget: the summary queue may be hours behind a model
	// outage, and pruned messages are gone before it catches up.
	keepUnsummarized := app.Neural().SummaryConfigured()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	chatIDs, err := app.Store().Messages.ChatIDs(ctx)
	if err != nil {
		logger.Instance().Error("prune messages: chat ids", zap.Error(err))
		return
	}
	var total int64
	for _, chatID := range chatIDs {
		n, err := app.Store().Messages.PruneChatByBytes(ctx, chatID, config.MaxReplyContextBytes, keepUnsummarized)
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
