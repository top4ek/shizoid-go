package models

import (
	"context"
	"database/sql"
	"time"
)

// Winner represents the winners table.
type Winner struct {
	ID        int64     `db:"id"`
	ChatID    int64     `db:"chat_id"`
	UserID    int64     `db:"user_id"`
	Date      time.Time `db:"date"`
	CreatedAt time.Time `db:"created_at"`
}

type winners struct{}

// Winners provides persistence operations for winners.
var Winners winners

const maxWinnersPerChat = 365

func (winners) HasToday(ctx context.Context, chatID int64) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM winners WHERE chat_id = $1 AND date = CURRENT_DATE)`,
		chatID).Scan(&exists)
	return exists, err
}

func (winners) Create(ctx context.Context, chatID, userID int64) (bool, error) {
	res, err := db.ExecContext(ctx,
		`INSERT INTO winners (chat_id, user_id, date) VALUES ($1, $2, CURRENT_DATE)
		 ON CONFLICT (chat_id, date) DO NOTHING`,
		chatID, userID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if n == 0 {
		return false, nil
	}
	_, err = db.ExecContext(ctx,
		`DELETE FROM winners WHERE chat_id = $1 AND id NOT IN (
			SELECT id FROM winners WHERE chat_id = $1 ORDER BY date DESC LIMIT $2
		)`,
		chatID, maxWinnersPerChat)
	return true, err
}

func (winners) LastWinner(ctx context.Context, chatID int64) (int64, string, string, bool, error) {
	const q = `
		SELECT w.user_id, COALESCE(u.username, ''),
			COALESCE(NULLIF(u.username, ''), NULLIF(u.first_name, ''), NULLIF(u.last_name, ''), '')
		FROM winners w
		LEFT JOIN users u ON u.id = w.user_id
		WHERE w.chat_id = $1
		ORDER BY w.date DESC
		LIMIT 1`
	var id int64
	var username, name string
	err := db.QueryRowContext(ctx, q, chatID).Scan(&id, &username, &name)
	if err == sql.ErrNoRows {
		return 0, "", "", false, nil
	}
	if err != nil {
		return 0, "", "", false, err
	}
	return id, username, name, true, nil
}

func (winners) TopOfYear(ctx context.Context, chatID int64, limit int) ([]ScoreEntry, error) {
	const q = `
		SELECT w.user_id, COALESCE(NULLIF(u.username, ''), NULLIF(u.first_name, ''), NULLIF(u.last_name, ''), '') AS name, COUNT(*) AS wins
		FROM winners w
		LEFT JOIN users u ON u.id = w.user_id
		WHERE w.chat_id = $1 AND w.date > CURRENT_DATE - INTERVAL '365 days'
		GROUP BY w.user_id, name
		ORDER BY wins DESC
		LIMIT $2`
	rows, err := db.QueryContext(ctx, q, chatID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []ScoreEntry
	for rows.Next() {
		var e ScoreEntry
		if err := rows.Scan(&e.UserID, &e.Name, &e.Score); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
