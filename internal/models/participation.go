package models

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5"
	"time"
)

type participations struct{ db DBTX }

// ScoreEntry is a single line of a chat leaderboard.
type ScoreEntry struct {
	UserID   int64
	Username string
	Name     string
	Score    int
}

// CaptchaPending is an active captcha challenge past its deadline.
type CaptchaPending struct {
	ChatID    int64
	UserID    int64
	MessageID int
}

func (r participations) Ensure(ctx context.Context, chatID, userID int64, left bool) error {
	const q = `
		INSERT INTO participations (chat_id, user_id, left_at, active_at, updated_at)
		VALUES ($1, $2, CASE WHEN $3 THEN NOW() ELSE NULL END, NOW(), NOW())
		ON CONFLICT (chat_id, user_id) DO UPDATE SET
			left_at = CASE WHEN $3 THEN NOW() ELSE NULL END,
			active_at = CASE WHEN $3 THEN participations.active_at ELSE NOW() END,
			updated_at = NOW()`
	_, err := r.db.Exec(ctx, q, chatID, userID, left)
	return err
}

func (r participations) IncrScore(ctx context.Context, chatID, userID int64, delta int) error {
	_, err := r.db.Exec(ctx,
		`UPDATE participations SET score = score + $3, updated_at = NOW() WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID, delta)
	return err
}

func (r participations) ResetScores(ctx context.Context, chatID int64) error {
	_, err := r.db.Exec(ctx, `UPDATE participations SET score = 0 WHERE chat_id = $1`, chatID)
	return err
}

func (r participations) CaptchaSolved(ctx context.Context, chatID, userID int64) (bool, error) {
	var solved bool
	err := r.db.QueryRow(ctx,
		`SELECT captcha_solved_at IS NOT NULL FROM participations WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID).Scan(&solved)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	return solved, err
}

func (r participations) GetCaptchaPending(ctx context.Context, chatID, userID int64) (correctEmoji string, messageID int, ok bool, err error) {
	var emoji sql.NullString
	var msgID sql.NullInt64
	err = r.db.QueryRow(ctx, `
		SELECT captcha_correct_emoji, captcha_message_id
		FROM participations
		WHERE chat_id = $1 AND user_id = $2
		  AND captcha_requested_at IS NOT NULL
		  AND captcha_solved_at IS NULL`,
		chatID, userID).Scan(&emoji, &msgID)
	if err == pgx.ErrNoRows {
		return "", 0, false, nil
	}
	if err != nil {
		return "", 0, false, err
	}
	id := 0
	if msgID.Valid {
		id = int(msgID.Int64)
	}
	return emoji.String, id, true, nil
}

func (r participations) GreetingGreeted(ctx context.Context, chatID, userID int64) (bool, error) {
	var greeted bool
	err := r.db.QueryRow(ctx,
		`SELECT greeted_at IS NOT NULL FROM participations WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID).Scan(&greeted)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	return greeted, err
}

func (r participations) TryClaimGreeting(ctx context.Context, chatID, userID int64) (bool, error) {
	res, err := r.db.Exec(ctx, `
		UPDATE participations SET greeted_at = NOW(), updated_at = NOW()
		WHERE chat_id = $1 AND user_id = $2 AND greeted_at IS NULL`,
		chatID, userID)
	if err != nil {
		return false, err
	}
	return res.RowsAffected() == 1, nil
}

func (r participations) ClearGreeting(ctx context.Context, chatID, userID int64) error {
	_, err := r.db.Exec(ctx, `
		UPDATE participations SET greeted_at = NULL, updated_at = NOW()
		WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID)
	return err
}

func (r participations) TryClaimCaptcha(ctx context.Context, chatID, userID int64) (bool, error) {
	res, err := r.db.Exec(ctx, `
		UPDATE participations SET captcha_requested_at = NOW(), updated_at = NOW()
		WHERE chat_id = $1 AND user_id = $2
		  AND captcha_solved_at IS NULL
		  AND captcha_requested_at IS NULL`,
		chatID, userID)
	if err != nil {
		return false, err
	}
	return res.RowsAffected() == 1, nil
}

func (r participations) SetCaptchaDetails(ctx context.Context, chatID, userID int64, emoji string, messageID int) error {
	_, err := r.db.Exec(ctx, `
		UPDATE participations SET
			captcha_correct_emoji = $3,
			captcha_message_id = $4,
			updated_at = NOW()
		WHERE chat_id = $1 AND user_id = $2
		  AND captcha_requested_at IS NOT NULL
		  AND captcha_solved_at IS NULL`,
		chatID, userID, emoji, messageID)
	return err
}

func (r participations) ClearCaptcha(ctx context.Context, chatID, userID int64) error {
	_, err := r.db.Exec(ctx, `
		UPDATE participations SET
			captcha_requested_at = NULL,
			captcha_correct_emoji = NULL,
			captcha_message_id = NULL,
			updated_at = NOW()
		WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID)
	return err
}

func (r participations) MarkCaptchaSolved(ctx context.Context, chatID, userID int64) error {
	_, err := r.db.Exec(ctx, `
		UPDATE participations SET
			captcha_solved_at = NOW(),
			captcha_requested_at = NULL,
			captcha_correct_emoji = NULL,
			captcha_message_id = NULL,
			updated_at = NOW()
		WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID)
	return err
}

func (r participations) ExpiredPending(ctx context.Context, timeout time.Duration) ([]CaptchaPending, error) {
	deadline := time.Now().Add(-timeout)
	const q = `
		SELECT p.chat_id, p.user_id, p.captcha_message_id
		FROM participations p
		JOIN chats c ON c.id = p.chat_id
		WHERE p.captcha_requested_at IS NOT NULL
		  AND p.captcha_solved_at IS NULL
		  AND p.left_at IS NULL
		  AND c.captcha_enabled_at IS NOT NULL
		  AND p.captcha_message_id IS NOT NULL
		  AND p.captcha_requested_at < $1`
	rows, err := r.db.Query(ctx, q, deadline)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CaptchaPending
	for rows.Next() {
		var p CaptchaPending
		var msgID sql.NullInt64
		if err := rows.Scan(&p.ChatID, &p.UserID, &msgID); err != nil {
			return nil, err
		}
		if msgID.Valid {
			p.MessageID = int(msgID.Int64)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r participations) TopByScore(ctx context.Context, chatID int64, limit int) ([]ScoreEntry, error) {
	const q = `
		SELECT p.user_id, COALESCE(u.username, ''),
			COALESCE(NULLIF(u.username, ''), NULLIF(u.first_name, ''), NULLIF(u.last_name, ''), '') AS name,
			p.score
		FROM participations p
		LEFT JOIN users u ON u.id = p.user_id
		WHERE p.chat_id = $1 AND p.score > 0
		ORDER BY p.score DESC
		LIMIT $2`
	rows, err := r.db.Query(ctx, q, chatID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []ScoreEntry
	for rows.Next() {
		var e ScoreEntry
		if err := rows.Scan(&e.UserID, &e.Username, &e.Name, &e.Score); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
