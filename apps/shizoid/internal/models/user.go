package models

import (
	"context"
	"database/sql"
)

// User represents the users table (Telegram users, id = Telegram user id).
type User struct {
	ID           int64          `db:"id"`
	IsBot        sql.NullBool   `db:"is_bot"`
	FirstName    sql.NullString `db:"first_name"`
	LastName     sql.NullString `db:"last_name"`
	Username     sql.NullString `db:"username"`
	LanguageCode sql.NullString `db:"language_code"`
}

type users struct{ db DBTX }

const userColumns = `id, is_bot, first_name, last_name, username, language_code`

func scanUser(row interface{ Scan(...any) error }) (*User, error) {
	u := &User{}
	err := row.Scan(
		&u.ID, &u.IsBot, &u.FirstName, &u.LastName, &u.Username, &u.LanguageCode,
	)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r users) Upsert(ctx context.Context, u *User) (*User, error) {
	const q = `
		INSERT INTO users (id, is_bot, first_name, last_name, username, language_code, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (id) DO UPDATE SET
			is_bot = EXCLUDED.is_bot,
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name,
			username = EXCLUDED.username,
			language_code = EXCLUDED.language_code,
			updated_at = NOW()
		RETURNING ` + userColumns
	return scanUser(r.db.QueryRow(ctx, q,
		u.ID, u.IsBot, u.FirstName, u.LastName, u.Username, u.LanguageCode))
}

func (r users) CaptchaSolved(ctx context.Context, id int64) (bool, error) {
	return queryFlag(ctx, r.db, `SELECT captcha_solved_at IS NOT NULL FROM users WHERE id = $1`, id)
}

func (r users) MarkCaptchaSolved(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE users SET captcha_solved_at = NOW() WHERE id = $1 AND captcha_solved_at IS NULL`, id)
	return err
}
