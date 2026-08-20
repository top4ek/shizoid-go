package models

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5"
	"time"
)

type messages struct{ db DBTX }

// MessageRow is a message joined with sender profile fields.
type MessageRow struct {
	UserID    int64
	Text      string
	FirstName sql.NullString
	LastName  sql.NullString
	Username  sql.NullString
	IsBot     sql.NullBool
}

func (r messages) Append(ctx context.Context, chatID, userID int64, text string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO messages (chat_id, user_id, text) VALUES ($1, $2, $3)`,
		chatID, userID, text)
	return err
}

// ChatIDs lists the chats that currently have stored messages.
func (r messages) ChatIDs(ctx context.Context) ([]int64, error) {
	rows, err := r.db.Query(ctx, `SELECT DISTINCT chat_id FROM messages`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// PruneChatByBytes keeps the newest keepBytes of history for one chat. Pruning
// per chat keeps the window scan bounded by a single chat's history instead of
// the whole table.
func (r messages) PruneChatByBytes(ctx context.Context, chatID int64, keepBytes int) (int64, error) {
	if keepBytes <= 0 {
		return 0, nil
	}
	res, err := r.db.Exec(ctx, `
		DELETE FROM messages
		WHERE chat_id = $1 AND id IN (
			SELECT id FROM (
				SELECT id,
					SUM(octet_length(text)) OVER (
						ORDER BY created_at DESC, id DESC
					) AS cum_bytes
				FROM messages
				WHERE chat_id = $1
			) ranked
			WHERE cum_bytes > $2
		)`, chatID, keepBytes)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected(), nil
}

func (r messages) RecentByBytes(ctx context.Context, chatID int64, maxBytes int) ([]MessageRow, error) {
	if maxBytes <= 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx, `
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
	return r.recentLatestMessages(ctx, chatID)
}

func (r messages) RecentByBytesSince(ctx context.Context, chatID int64, since time.Time, maxBytes int) ([]MessageRow, error) {
	if since.IsZero() {
		return r.RecentByBytes(ctx, chatID, maxBytes)
	}
	if maxBytes <= 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx, `
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
			WHERE m.chat_id = $1 AND m.created_at >= $2
		) ranked
		WHERE cum_bytes <= $3
		ORDER BY created_at DESC, id DESC`, chatID, since, maxBytes)
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
	return r.recentLatestMessagesSince(ctx, chatID, since)
}

func (r messages) RecentTextsByBytes(ctx context.Context, chatID int64, maxBytes int) ([]string, error) {
	if maxBytes <= 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx, `
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
	return r.recentLatestMessageText(ctx, chatID)
}

func (r messages) TextsSinceByBytes(ctx context.Context, chatID int64, since time.Time, maxBytes int) ([]string, error) {
	if maxBytes <= 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT text
		FROM (
			SELECT text, created_at, id,
				SUM(octet_length(text)) OVER (
					ORDER BY created_at DESC, id DESC
				) AS cum_bytes
			FROM messages
			WHERE chat_id = $1 AND created_at >= $2
		) ranked
		WHERE cum_bytes <= $3
		ORDER BY created_at ASC, id ASC`, chatID, since, maxBytes)
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

func (r messages) recentLatestMessages(ctx context.Context, chatID int64) ([]MessageRow, error) {
	rows, err := r.db.Query(ctx, `
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

func (r messages) recentLatestMessagesSince(ctx context.Context, chatID int64, since time.Time) ([]MessageRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT m.user_id, m.text, u.first_name, u.last_name, u.username, u.is_bot
		FROM messages m
		LEFT JOIN users u ON u.id = m.user_id
		WHERE m.chat_id = $1 AND m.created_at >= $2
		ORDER BY m.created_at DESC, m.id DESC
		LIMIT 1`, chatID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMessageRows(rows)
}

func (r messages) recentLatestMessageText(ctx context.Context, chatID int64) ([]string, error) {
	var t string
	err := r.db.QueryRow(ctx,
		`SELECT text FROM messages WHERE chat_id = $1 ORDER BY created_at DESC, id DESC LIMIT 1`,
		chatID).Scan(&t)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return []string{t}, nil
}

func scanMessageRows(rows pgx.Rows) ([]MessageRow, error) {
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
