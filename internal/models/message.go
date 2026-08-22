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
	return queryRows(ctx, r.db, `SELECT DISTINCT chat_id FROM messages`, nil,
		func(rows pgx.Rows) (id int64, err error) { return id, rows.Scan(&id) })
}

// PruneChatByBytes keeps the newest keepBytes of history for one chat. Pruning
// per chat keeps the window scan bounded by a single chat's history instead of
// the whole table.
//
// keepUnsummarized additionally spares everything the memory summarizer has not
// read yet, so a model outage lasting longer than a prune cycle cannot delete
// history before it is summarized. Callers must leave it off when memory
// summarization is not running at all: memory_summarized_at would never advance
// and the table would grow without bound.
func (r messages) PruneChatByBytes(ctx context.Context, chatID int64, keepBytes int, keepUnsummarized bool) (int64, error) {
	if keepBytes <= 0 {
		return 0, nil
	}
	summarized := ``
	if keepUnsummarized {
		summarized = ` AND created_at <= COALESCE(
				(SELECT memory_summarized_at FROM chats WHERE id = $1),
				'-infinity'::timestamptz)`
	}
	res, err := r.db.Exec(ctx, `
		DELETE FROM messages
		WHERE chat_id = $1`+summarized+` AND id IN (
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

// The byte-budgeted history queries below all have the same shape: walk the
// chat's messages newest-first, keep the prefix whose cumulative text bytes fit
// the budget, and if that prefix is empty fall back to the single latest row so
// a message larger than the whole budget is not silently dropped.

const (
	// projections inside the window subquery (aliased m/u) and back out of it
	messageRowInner = `m.user_id, m.text, u.first_name, u.last_name, u.username, u.is_bot`
	messageRowOuter = `user_id, text, first_name, last_name, username, is_bot`
	usersJoin       = `LEFT JOIN users u ON u.id = m.user_id`
)

// byteBudgetedQuery renders the windowed query. A zero since drops the time
// bound, which also shifts the budget placeholder from $3 to $2.
func byteBudgetedQuery(outer, inner, join string, withSince bool, order string) string {
	where, budget := `WHERE m.chat_id = $1`, `$2`
	if withSince {
		where, budget = where+` AND m.created_at >= $2`, `$3`
	}
	return `
		SELECT ` + outer + `
		FROM (
			SELECT ` + inner + `, m.created_at, m.id,
				SUM(octet_length(m.text)) OVER (
					ORDER BY m.created_at DESC, m.id DESC
				) AS cum_bytes
			FROM messages m
			` + join + `
			` + where + `
		) ranked
		WHERE cum_bytes <= ` + budget + `
		ORDER BY created_at ` + order + `, id ` + order
}

// latestQuery renders the single-newest-row fallback for the same projection.
func latestQuery(inner, join string, withSince bool) string {
	where := `WHERE m.chat_id = $1`
	if withSince {
		where += ` AND m.created_at >= $2`
	}
	return `
		SELECT ` + inner + `
		FROM messages m
		` + join + `
		` + where + `
		ORDER BY m.created_at DESC, m.id DESC
		LIMIT 1`
}

// budgetArgs orders the placeholders to match byteBudgetedQuery.
func budgetArgs(chatID int64, since time.Time, maxBytes int) []any {
	if since.IsZero() {
		return []any{chatID, maxBytes}
	}
	return []any{chatID, since, maxBytes}
}

func scanMessageRow(rows pgx.Rows) (MessageRow, error) {
	var row MessageRow
	err := rows.Scan(&row.UserID, &row.Text, &row.FirstName, &row.LastName, &row.Username, &row.IsBot)
	return row, err
}

func scanText(rows pgx.Rows) (string, error) {
	var t string
	err := rows.Scan(&t)
	return t, err
}

// RecentByBytes returns the newest messages fitting maxBytes, newest first.
func (r messages) RecentByBytes(ctx context.Context, chatID int64, maxBytes int) ([]MessageRow, error) {
	return r.recentRows(ctx, chatID, time.Time{}, maxBytes)
}

// RecentByBytesSince is RecentByBytes bounded to messages at or after since.
func (r messages) RecentByBytesSince(ctx context.Context, chatID int64, since time.Time, maxBytes int) ([]MessageRow, error) {
	return r.recentRows(ctx, chatID, since, maxBytes)
}

func (r messages) recentRows(ctx context.Context, chatID int64, since time.Time, maxBytes int) ([]MessageRow, error) {
	if maxBytes <= 0 {
		return nil, nil
	}
	withSince := !since.IsZero()
	out, err := queryRows(ctx, r.db,
		byteBudgetedQuery(messageRowOuter, messageRowInner, usersJoin, withSince, `DESC`),
		budgetArgs(chatID, since, maxBytes), scanMessageRow)
	if err != nil || len(out) > 0 {
		return out, err
	}
	args := []any{chatID}
	if withSince {
		args = append(args, since)
	}
	return queryRows(ctx, r.db, latestQuery(messageRowInner, usersJoin, withSince), args, scanMessageRow)
}

// RecentTextsByBytes returns the newest message texts fitting maxBytes, newest
// first.
func (r messages) RecentTextsByBytes(ctx context.Context, chatID int64, maxBytes int) ([]string, error) {
	if maxBytes <= 0 {
		return nil, nil
	}
	texts, err := queryRows(ctx, r.db,
		byteBudgetedQuery(`text`, `m.text`, ``, false, `DESC`),
		budgetArgs(chatID, time.Time{}, maxBytes), scanText)
	if err != nil || len(texts) > 0 {
		return texts, err
	}
	return queryRows(ctx, r.db, latestQuery(`m.text`, ``, false), []any{chatID}, scanText)
}

// TextsSinceByBytes returns the texts at or after since that fit maxBytes, in
// chronological order. Unlike the others it has no single-row fallback: the
// summarizer treats an empty window as "nothing new to summarize".
func (r messages) TextsSinceByBytes(ctx context.Context, chatID int64, since time.Time, maxBytes int) ([]string, error) {
	if maxBytes <= 0 {
		return nil, nil
	}
	return queryRows(ctx, r.db,
		byteBudgetedQuery(`text`, `m.text`, ``, true, `ASC`),
		budgetArgs(chatID, since, maxBytes), scanText)
}
