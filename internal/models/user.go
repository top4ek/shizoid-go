package models

import (
	"context"
	"database/sql"
	"time"
)

// User represents the users table (Telegram users, id = Telegram user id).
type User struct {
	ID              int64          `db:"id"`
	IsBot           sql.NullBool   `db:"is_bot"`
	FirstName       sql.NullString `db:"first_name"`
	LastName        sql.NullString `db:"last_name"`
	Username        sql.NullString `db:"username"`
	LanguageCode    sql.NullString `db:"language_code"`
	CreatedAt       time.Time      `db:"created_at"`
	UpdatedAt       time.Time      `db:"updated_at"`
	CaptchaSolvedAt sql.NullTime   `db:"captcha_solved_at"`
}

type users struct{}

// Users provides persistence operations for users.
var Users users

const userColumns = `id, is_bot, first_name, last_name, username, language_code,
	captcha_solved_at, created_at, updated_at`

func scanUser(row interface{ Scan(...any) error }) (*User, error) {
	u := &User{}
	err := row.Scan(
		&u.ID, &u.IsBot, &u.FirstName, &u.LastName, &u.Username, &u.LanguageCode,
		&u.CaptchaSolvedAt, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func upsertUserTx(ctx context.Context, tx *sql.Tx, u *User) error {
	const q = `
		INSERT INTO users (id, is_bot, first_name, last_name, username, language_code, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (id) DO UPDATE SET
			is_bot = EXCLUDED.is_bot,
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name,
			username = EXCLUDED.username,
			language_code = EXCLUDED.language_code,
			updated_at = NOW()`
	_, err := tx.ExecContext(ctx, q,
		u.ID, u.IsBot, u.FirstName, u.LastName, u.Username, u.LanguageCode)
	return err
}

func (users) Upsert(ctx context.Context, u *User) (*User, error) {
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
	return scanUser(db.QueryRowContext(ctx, q,
		u.ID, u.IsBot, u.FirstName, u.LastName, u.Username, u.LanguageCode))
}

func (users) Get(ctx context.Context, id int64) (*User, error) {
	row := db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id)
	return scanUser(row)
}

func (users) CaptchaSolved(ctx context.Context, id int64) (bool, error) {
	var solved bool
	err := db.QueryRowContext(ctx,
		`SELECT captcha_solved_at IS NOT NULL FROM users WHERE id = $1`, id).Scan(&solved)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return solved, err
}

func (users) MarkCaptchaSolved(ctx context.Context, id int64) error {
	_, err := db.ExecContext(ctx,
		`UPDATE users SET captcha_solved_at = NOW() WHERE id = $1 AND captcha_solved_at IS NULL`, id)
	return err
}
