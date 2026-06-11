package models

import (
	"context"
	"database/sql"
	"time"
)

// Message represents the messages table.
type Message struct {
	ID        int64     `db:"id"`
	ChatID    int64     `db:"chat_id"`
	UserID    int64     `db:"user_id"`
	Text      string    `db:"text"`
	CreatedAt time.Time `db:"created_at"`
}

type messages struct{}

// Messages provides persistence operations for messages.
var Messages messages

// MessageRow is a message joined with sender profile fields.
type MessageRow struct {
	UserID    int64
	Text      string
	FirstName sql.NullString
	LastName  sql.NullString
	Username  sql.NullString
	IsBot     sql.NullBool
}

func (messages) Append(ctx context.Context, chatID, userID int64, text string) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO messages (chat_id, user_id, text) VALUES ($1, $2, $3)`,
		chatID, userID, text)
	return err
}

func (messages) PruneByBytes(ctx context.Context, keepBytes int) (int64, error) {
	if keepBytes <= 0 {
		return 0, nil
	}
	res, err := db.ExecContext(ctx, `
		DELETE FROM messages
		WHERE id IN (
			SELECT id FROM (
				SELECT id,
					SUM(octet_length(text)) OVER (
						PARTITION BY chat_id
						ORDER BY created_at DESC, id DESC
					) AS cum_bytes
				FROM messages
			) ranked
			WHERE cum_bytes > $1
		)`, keepBytes)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (messages) RecentByBytes(ctx context.Context, chatID int64, maxBytes int) ([]MessageRow, error) {
	if maxBytes <= 0 {
		return nil, nil
	}
	rows, err := db.QueryContext(ctx, `
		SELECT user_id, text, first_name, last_name, username, is_bot
		FROM (
			SELECT m.user_id, m.text,
				u.first_name, u.last_name, u.username, u.is_bot,
				m.created_at, m.id,
				SUM(octet_length(m.text)) OVER (
					ORDER BY m.created_at DESC, m.id DESC
				) AS cum_bytes
			FROM messages m
			LEFT JOIN users u ON u.id = m.user_id
			WHERE m.chat_id = $1
		) ranked
		WHERE cum_bytes <= $2
		ORDER BY created_at DESC, id DESC`, chatID, maxBytes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out, err := scanMessageRows(rows)
	if err != nil {
		return nil, err
	}
	if len(out) > 0 {
		return out, nil
	}
	return recentLatestMessages(ctx, chatID)
}

func (messages) RecentTextsByBytes(ctx context.Context, chatID int64, maxBytes int) ([]string, error) {
	if maxBytes <= 0 {
		return nil, nil
	}
	rows, err := db.QueryContext(ctx, `
		SELECT text
		FROM (
			SELECT text, created_at, id,
				SUM(octet_length(text)) OVER (
					ORDER BY created_at DESC, id DESC
				) AS cum_bytes
			FROM messages
			WHERE chat_id = $1
		) ranked
		WHERE cum_bytes <= $2
		ORDER BY created_at DESC, id DESC`, chatID, maxBytes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var texts []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		texts = append(texts, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(texts) > 0 {
		return texts, nil
	}
	return recentLatestMessageText(ctx, chatID)
}

func (messages) TextsSince(ctx context.Context, chatID int64, since time.Time) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT text FROM messages WHERE chat_id = $1 AND created_at >= $2 ORDER BY created_at ASC, id ASC`,
		chatID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var texts []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		texts = append(texts, t)
	}
	return texts, rows.Err()
}

func (messages) LastActivity(ctx context.Context, chatID int64) (sql.NullTime, error) {
	var last sql.NullTime
	err := db.QueryRowContext(ctx,
		`SELECT MAX(created_at) FROM messages WHERE chat_id = $1`, chatID).Scan(&last)
	return last, err
}

func recentLatestMessages(ctx context.Context, chatID int64) ([]MessageRow, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT m.user_id, m.text, u.first_name, u.last_name, u.username, u.is_bot
		FROM messages m
		LEFT JOIN users u ON u.id = m.user_id
		WHERE m.chat_id = $1
		ORDER BY m.created_at DESC, m.id DESC
		LIMIT 1`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMessageRows(rows)
}

func recentLatestMessageText(ctx context.Context, chatID int64) ([]string, error) {
	var t string
	err := db.QueryRowContext(ctx,
		`SELECT text FROM messages WHERE chat_id = $1 ORDER BY created_at DESC, id DESC LIMIT 1`,
		chatID).Scan(&t)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return []string{t}, nil
}

func scanMessageRows(rows *sql.Rows) ([]MessageRow, error) {
	var out []MessageRow
	for rows.Next() {
		var row MessageRow
		if err := rows.Scan(&row.UserID, &row.Text, &row.FirstName, &row.LastName, &row.Username, &row.IsBot); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
