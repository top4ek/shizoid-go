package models

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// SummaryJobKind is the task a queued job stands for. Jobs of different kinds
// for one chat are independent: each has its own row and its own backoff.
type SummaryJobKind int16

const (
	SummaryJobWinner SummaryJobKind = iota
	SummaryJobMemory
)

var summaryJobKindNames = [...]string{
	SummaryJobWinner: "winner",
	SummaryJobMemory: "memory",
}

func (k SummaryJobKind) String() string {
	if !k.Valid() {
		return "unknown"
	}
	return summaryJobKindNames[k]
}

// Valid reports whether the kind is one this build knows how to run. A row
// written by a newer build and read back after a rollback is not.
func (k SummaryJobKind) Valid() bool {
	return int(k) >= 0 && int(k) < len(summaryJobKindNames)
}

// SummaryJob is one unit of work for the summary neural chain, persisted so it
// outlives both a model outage and a process restart.
type SummaryJob struct {
	ID        int64
	ChatID    int64
	Kind      SummaryJobKind
	Payload   []byte
	Attempts  int
	ExpiresAt time.Time
}

type summaryJobs struct{ db DBTX }

const summaryJobColumns = `id, chat_id, kind, payload, attempts, expires_at`

func scanSummaryJob(rows pgx.Rows) (j SummaryJob, err error) {
	return j, rows.Scan(&j.ID, &j.ChatID, &j.Kind, &j.Payload, &j.Attempts, &j.ExpiresAt)
}

// Enqueue records work of one kind for one chat. A job already pending for that
// pair is replaced rather than queued behind: the newer payload is the current
// state of the world, and its backoff should start over.
func (r summaryJobs) Enqueue(ctx context.Context, chatID int64, kind SummaryJobKind, payload []byte, ttl time.Duration) error {
	if len(payload) == 0 {
		payload = []byte(`{}`)
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO summary_jobs (chat_id, kind, payload, expires_at)
		VALUES ($1, $2, $3, NOW() + $4::interval)
		ON CONFLICT (chat_id, kind) DO UPDATE SET
			payload    = EXCLUDED.payload,
			attempts   = 0,
			run_after  = NOW(),
			expires_at = EXCLUDED.expires_at`,
		chatID, kind, payload, ttl)
	return err
}

// ExpireOverdue removes the jobs that ran out of time and returns them so the
// caller can report what was given up on.
func (r summaryJobs) ExpireOverdue(ctx context.Context) ([]SummaryJob, error) {
	return queryRows(ctx, r.db,
		`DELETE FROM summary_jobs WHERE expires_at <= NOW() RETURNING `+summaryJobColumns,
		nil, scanSummaryJob)
}

// Claim takes the job that has been due the longest and leases it for the given
// duration by pushing run_after forward. The lease is what makes a crash
// mid-job recoverable: the row becomes due again instead of staying claimed.
func (r summaryJobs) Claim(ctx context.Context, lease time.Duration) (SummaryJob, bool, error) {
	const q = `
		UPDATE summary_jobs SET run_after = NOW() + $1::interval, attempts = attempts + 1
		WHERE id = (
			SELECT id FROM summary_jobs WHERE run_after <= NOW()
			ORDER BY run_after LIMIT 1 FOR UPDATE SKIP LOCKED
		)
		RETURNING ` + summaryJobColumns
	rows, err := r.db.Query(ctx, q, lease)
	if err != nil {
		return SummaryJob{}, false, err
	}
	jobs, err := collect(rows, scanSummaryJob)
	if err != nil || len(jobs) == 0 {
		return SummaryJob{}, false, err
	}
	return jobs[0], true, nil
}

// Done drops a job that has been carried out. It matches on the attempt count
// the caller ran, so an Enqueue that landed mid-job - which resets attempts and
// replaces the payload on the same row - is left alone rather than deleted as if
// it were the work that just finished.
func (r summaryJobs) Done(ctx context.Context, job SummaryJob) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM summary_jobs WHERE id = $1 AND attempts = $2`,
		job.ID, job.Attempts)
	return err
}

// Retry puts a failed job back on the clock, overwriting the lease Claim set.
// Like Done it only touches the attempt it was handed, so a job replaced while
// it was running keeps the fresh backoff Enqueue gave it.
func (r summaryJobs) Retry(ctx context.Context, job SummaryJob, delay time.Duration) error {
	_, err := r.db.Exec(ctx,
		`UPDATE summary_jobs SET run_after = NOW() + $3::interval WHERE id = $1 AND attempts = $2`,
		job.ID, job.Attempts, delay)
	return err
}
