package models

import (
	"context"
	"database/sql"
	"time"
)

// Participation represents the participations table (a user in a chat).
type Participation struct {
	ID              int64        `db:"id"`
	ChatID          int64        `db:"chat_id"`
	UserID          int64        `db:"user_id"`
	LeftAt          sql.NullTime `db:"left_at"`
	Score           int          `db:"score"`
	ActiveAt        sql.NullTime `db:"active_at"`
	CaptchaSolvedAt      sql.NullTime   `db:"captcha_solved_at"`
	CaptchaRequestedAt   sql.NullTime   `db:"captcha_requested_at"`
	CaptchaCorrectEmoji  sql.NullString `db:"captcha_correct_emoji"`
	CaptchaMessageID     sql.NullInt64  `db:"captcha_message_id"`
	CreatedAt            time.Time      `db:"created_at"`
	UpdatedAt       time.Time    `db:"updated_at"`
}

type participations struct{}

// Participations provides persistence operations for participations.
var Participations participations

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

const participationColumns = `id, chat_id, user_id, left_at, score, active_at, captcha_solved_at,
	captcha_requested_at, captcha_correct_emoji, captcha_message_id, created_at, updated_at`

func scanParticipation(row interface{ Scan(...any) error }) (*Participation, error) {
	p := &Participation{}
	err := row.Scan(
		&p.ID, &p.ChatID, &p.UserID, &p.LeftAt, &p.Score, &p.ActiveAt, &p.CaptchaSolvedAt,
		&p.CaptchaRequestedAt, &p.CaptchaCorrectEmoji, &p.CaptchaMessageID,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func ensureParticipationTx(ctx context.Context, tx *sql.Tx, chatID, userID int64, left bool) (*Participation, error) {
	const q = `
		INSERT INTO participations (chat_id, user_id, left_at, active_at, updated_at)
		VALUES ($1, $2, CASE WHEN $3 THEN NOW() ELSE NULL END, NOW(), NOW())
		ON CONFLICT (chat_id, user_id) DO UPDATE SET
			left_at = CASE WHEN $3 THEN NOW() ELSE NULL END,
			active_at = CASE WHEN $3 THEN participations.active_at ELSE NOW() END,
			updated_at = NOW()
		RETURNING ` + participationColumns
	return scanParticipation(tx.QueryRowContext(ctx, q, chatID, userID, left))
}

func (participations) Ensure(ctx context.Context, chatID, userID int64, left bool) (*Participation, error) {
	const q = `
		INSERT INTO participations (chat_id, user_id, left_at, active_at, updated_at)
		VALUES ($1, $2, CASE WHEN $3 THEN NOW() ELSE NULL END, NOW(), NOW())
		ON CONFLICT (chat_id, user_id) DO UPDATE SET
			left_at = CASE WHEN $3 THEN NOW() ELSE NULL END,
			active_at = CASE WHEN $3 THEN participations.active_at ELSE NOW() END,
			updated_at = NOW()
		RETURNING ` + participationColumns
	return scanParticipation(db.QueryRowContext(ctx, q, chatID, userID, left))
}

func (participations) IncrScore(ctx context.Context, chatID, userID int64, delta int) error {
	_, err := db.ExecContext(ctx,
		`UPDATE participations SET score = score + $3, updated_at = NOW() WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID, delta)
	return err
}

func (participations) ResetScores(ctx context.Context, chatID int64) error {
	_, err := db.ExecContext(ctx, `UPDATE participations SET score = 0 WHERE chat_id = $1`, chatID)
	return err
}

func (participations) CaptchaSolved(ctx context.Context, chatID, userID int64) (bool, error) {
	var solved bool
	err := db.QueryRowContext(ctx,
		`SELECT captcha_solved_at IS NOT NULL FROM participations WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID).Scan(&solved)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return solved, err
}

func (participations) GetCaptchaPending(ctx context.Context, chatID, userID int64) (correctEmoji string, messageID int, ok bool, err error) {
	var emoji sql.NullString
	var msgID sql.NullInt64
	err = db.QueryRowContext(ctx, `
		SELECT captcha_correct_emoji, captcha_message_id
		FROM participations
		WHERE chat_id = $1 AND user_id = $2
		  AND captcha_requested_at IS NOT NULL
		  AND captcha_solved_at IS NULL`,
		chatID, userID).Scan(&emoji, &msgID)
	if err == sql.ErrNoRows {
		return "", 0, false, nil
	}
	if err != nil {
		return "", 0, false, err
	}
	if !emoji.Valid {
		return "", 0, false, nil
	}
	id := 0
	if msgID.Valid {
		id = int(msgID.Int64)
	}
	return emoji.String, id, true, nil
}

func (participations) StartCaptcha(ctx context.Context, chatID, userID int64, emoji string, messageID int) error {
	_, err := db.ExecContext(ctx, `
		UPDATE participations SET
			captcha_requested_at = NOW(),
			captcha_correct_emoji = $3,
			captcha_message_id = $4,
			updated_at = NOW()
		WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID, emoji, messageID)
	return err
}

func (participations) ClearCaptcha(ctx context.Context, chatID, userID int64) error {
	_, err := db.ExecContext(ctx, `
		UPDATE participations SET
			captcha_requested_at = NULL,
			captcha_correct_emoji = NULL,
			captcha_message_id = NULL,
			updated_at = NOW()
		WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID)
	return err
}

func (participations) MarkCaptchaSolved(ctx context.Context, chatID, userID int64) error {
	_, err := db.ExecContext(ctx, `
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

func (participations) ExpiredPending(ctx context.Context, timeout time.Duration) ([]CaptchaPending, error) {
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
	rows, err := db.QueryContext(ctx, q, deadline)
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

func (participations) TopByScore(ctx context.Context, chatID int64, limit int) ([]ScoreEntry, error) {
	const q = `
		SELECT p.user_id, COALESCE(u.username, ''),
			COALESCE(NULLIF(u.username, ''), NULLIF(u.first_name, ''), NULLIF(u.last_name, ''), '') AS name,
			p.score
		FROM participations p
		LEFT JOIN users u ON u.id = p.user_id
		WHERE p.chat_id = $1 AND p.score > 0
		ORDER BY p.score DESC
		LIMIT $2`
	rows, err := db.QueryContext(ctx, q, chatID, limit)
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
