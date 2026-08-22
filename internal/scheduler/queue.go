package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"go.uber.org/zap"

	"shizoid/internal/app"
	"shizoid/internal/config"
	"shizoid/internal/handlers/winner"
	"shizoid/internal/logger"
	"shizoid/internal/models"
)

const (
	// queueLease is how long a claimed job stays invisible. It is longer than
	// any single job so that what re-releases a row is a crashed process, not a
	// slow model.
	queueLease = 10 * time.Minute
	// queueBudget bounds one drain. Overlapping ticks are already dropped by
	// cron.SkipIfStillRunning, so this only decides how much of the backlog a
	// single tick works through.
	queueBudget = 15 * time.Minute
	// baseBackoff and maxBackoff shape the retry curve: 1m, 2m, 4m, 8m, 16m,
	// then every 30m until the job succeeds or expires.
	baseBackoff = time.Minute
	maxBackoff  = 30 * time.Minute
	// memoryTTL is how long a memory job stays worth running. Past it the
	// window it would summarize has been superseded several times over.
	memoryTTL = 24 * time.Hour
)

// runSummaryQueue drains the jobs whose model call was deferred. It stops at the
// first failure: the summary chain is one busy backend, so the jobs behind this
// one would only spend their timeouts discovering the same outage.
func runSummaryQueue(b *bot.Bot) {
	logger.Instance().Debug("cron: summary queue")
	ctx, cancel := context.WithTimeout(context.Background(), queueBudget)
	defer cancel()

	// The sweep runs whatever the config says: rows left over from a build that
	// had a summary chain are nobody else's to clean up, and without it they
	// would sit in the queue forever once the chain is taken out.
	expireJobs(ctx)

	if !app.Neural().SummaryConfigured() {
		return
	}

	done, failed := 0, 0
	// Stop claiming once the budget can no longer give a job its full turn:
	// a job cut short by the tick deadline would burn an attempt and widen its
	// backoff without ever having had a chance to run.
	for budgetLeft(ctx) >= summaryChatTimeout {
		job, ok, err := app.Store().SummaryJobs.Claim(ctx, queueLease)
		if err != nil {
			logger.Instance().Error("summary queue: claim", zap.Error(err))
			break
		}
		if !ok {
			break
		}
		if err := runJob(ctx, b, job); err != nil {
			failed++
			logger.Instance().Warn("summary queue: job failed",
				zap.Int64("chat_id", job.ChatID),
				zap.String("kind", job.Kind.String()),
				zap.Int("attempts", job.Attempts),
				zap.Error(err),
			)
			retryJob(ctx, job)
			break
		}
		done++
		finishJob(ctx, job)
	}
	if done > 0 || failed > 0 {
		logger.Instance().Info("cron: summary queue done",
			zap.Int("handled", done),
			zap.Int("failed", failed),
		)
	}
}

// expireJobs drops the jobs that ran out of time. Giving up on one is the only
// way work leaves the queue unfinished, so it is reported as an error.
func expireJobs(ctx context.Context) {
	expired, err := app.Store().SummaryJobs.ExpireOverdue(ctx)
	if err != nil {
		logger.Instance().Error("summary queue: expire", zap.Error(err))
		return
	}
	for _, job := range expired {
		logger.Instance().Error("summary queue: job expired",
			zap.Int64("chat_id", job.ChatID),
			zap.String("kind", job.Kind.String()),
			zap.Int("attempts", job.Attempts),
		)
	}
}

// budgetLeft reports how much of the tick's budget is unspent.
func budgetLeft(ctx context.Context) time.Duration {
	if ctx.Err() != nil {
		return 0
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return queueBudget
	}
	return time.Until(deadline)
}

// finishJob drops a job that has been carried out. It deliberately outlives the
// tick's budget: the announcement is already in the chat by now, and leaving the
// row behind would post it a second time once the lease runs out.
func finishJob(ctx context.Context, job models.SummaryJob) {
	doneCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := app.Store().SummaryJobs.Done(doneCtx, job); err != nil {
		logger.Instance().Error("summary queue: done",
			zap.Int64("chat_id", job.ChatID),
			zap.String("kind", job.Kind.String()),
			zap.Error(err),
		)
	}
}

// retryJob puts a failed job back on the clock. Like finishJob it outlives the
// budget, so a job that failed at the very end of a tick still gets its backoff
// rather than coming straight back when the lease expires.
func retryJob(ctx context.Context, job models.SummaryJob) {
	retryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := app.Store().SummaryJobs.Retry(retryCtx, job, backoff(job.Attempts)); err != nil {
		logger.Instance().Error("summary queue: retry", zap.Error(err))
	}
}

func runJob(ctx context.Context, b *bot.Bot, job models.SummaryJob) error {
	jobCtx, cancel := context.WithTimeout(ctx, summaryChatTimeout)
	defer cancel()

	switch job.Kind {
	case models.SummaryJobWinner:
		return winner.Deliver(jobCtx, b, job.ChatID, job.Payload)
	case models.SummaryJobMemory:
		return summarizeMemory(jobCtx, job.ChatID)
	default:
		// Written by a newer build and read back after a rollback. Dropping it
		// keeps one unknown row from wedging the queue behind it.
		logger.Instance().Error("summary queue: unknown kind",
			zap.Int64("chat_id", job.ChatID),
			zap.Int("kind", int(job.Kind)),
		)
		return nil
	}
}

// summarizeMemory folds everything said since the last summary into the chat's
// long-term memory. The window is read here rather than snapshotted at enqueue
// time: re-deriving it costs one query and takes in whatever arrived while the
// job waited.
func summarizeMemory(ctx context.Context, chatID int64) error {
	chat, err := app.Store().Chats.Get(ctx, chatID)
	if err != nil {
		return err
	}
	existing := ""
	if chat.Memory.Valid {
		existing = chat.Memory.String
	}
	budget := summaryMessageBudget(existing)
	if budget <= 0 {
		return nil
	}
	since := time.Time{}
	if chat.MemorySummarizedAt.Valid {
		since = chat.MemorySummarizedAt.Time
	}
	msgs, err := app.Store().Messages.TextsSinceByBytes(ctx, chatID, since, budget)
	if err != nil {
		return err
	}
	if len(msgs) == 0 {
		return nil
	}
	logger.Instance().Debug("memory summarize",
		zap.Int64("chat_id", chatID),
		zap.Int("messages", len(msgs)),
	)
	summary, err := app.Neural().Summarize(ctx, config.Environment.SummaryPrompt, existing, msgs)
	if err != nil {
		return err
	}
	if summary = strings.TrimSpace(summary); summary == "" {
		return fmt.Errorf("memory: empty summary for chat %d", chatID)
	}
	summary = truncateRunes(summary, 4096)
	if err := app.Store().Chats.SetMemory(ctx, chatID, sql.NullString{String: summary, Valid: true}); err != nil {
		return err
	}
	return app.Store().Chats.SetMemorySummarizedAt(ctx, chatID, time.Now())
}

// backoff spaces the retries of a job that has failed attempts times: doubling
// from a minute, then flat once it reaches the cap.
func backoff(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	if attempts > 16 {
		return maxBackoff
	}
	d := baseBackoff << (attempts - 1)
	if d > maxBackoff {
		return maxBackoff
	}
	return d
}
