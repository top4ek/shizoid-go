package models

import (
	"context"
	"database/sql"
	"time"
)

// Chat represents the chats table. ID equals the Telegram chat id.
type Chat struct {
	ID               int64          `db:"id"`
	Kind             string         `db:"kind"` // private, group, supergroup, channel
	Random           int16          `db:"random"`
	Eightball        bool           `db:"eightball"`
	GreetingText     sql.NullString `db:"greeting_text"`
	Winner           sql.NullString `db:"winner"`
	Locale           string         `db:"locale"`
	GenerationMode   GenerationMode `db:"generation_mode"`
	Title            sql.NullString `db:"title"`
	FirstName        sql.NullString `db:"first_name"`
	LastName         sql.NullString `db:"last_name"`
	Username         sql.NullString `db:"username"`
	ActiveAt         sql.NullTime   `db:"active_at"`
	IdleDays         sql.NullInt64  `db:"idle_days"`
	CaptchaEnabledAt sql.NullTime   `db:"captcha_enabled_at"`
	CaptchaGreeting  sql.NullString `db:"captcha_greeting"`
	SystemPrompt     sql.NullString `db:"system_prompt"`
	Memory               sql.NullString `db:"memory"`
	IdlePokedAt          sql.NullTime   `db:"idle_poked_at"`
	MemorySummarizedAt   sql.NullTime   `db:"memory_summarized_at"`
	CreatedAt            time.Time      `db:"created_at"`
}

type chats struct{}

// Chats provides persistence operations for chats.
var Chats chats

// Enabled reports whether the bot is active in this chat.
func (c *Chat) Enabled() bool {
	return c.ActiveAt.Valid
}

// WinnerEnabled reports whether daily winner selection is configured.
func (c *Chat) WinnerEnabled() bool {
	return c.Winner.Valid && c.Winner.String != ""
}

// CaptchaEnabled reports whether captcha is active for new members.
func (c *Chat) CaptchaEnabled() bool {
	return c.CaptchaEnabledAt.Valid
}

// GreetingEnabled reports whether a join greeting is configured for the chat.
func (c *Chat) GreetingEnabled() bool {
	return c.GreetingText.Valid
}

const chatColumns = `id, kind, random, eightball, greeting_text, winner, locale, generation_mode,
	title, first_name, last_name, username, active_at, idle_days,
	captcha_enabled_at, captcha_greeting, system_prompt, memory,
	idle_poked_at, memory_summarized_at, created_at`

func scanChat(row interface{ Scan(...any) error }) (*Chat, error) {
	c := &Chat{}
	err := row.Scan(
		&c.ID, &c.Kind, &c.Random, &c.Eightball, &c.GreetingText, &c.Winner, &c.Locale, &c.GenerationMode,
		&c.Title, &c.FirstName, &c.LastName, &c.Username, &c.ActiveAt, &c.IdleDays,
		&c.CaptchaEnabledAt, &c.CaptchaGreeting, &c.SystemPrompt, &c.Memory,
		&c.IdlePokedAt, &c.MemorySummarizedAt, &c.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func upsertChatTx(ctx context.Context, tx *sql.Tx, c *Chat) (*Chat, error) {
	const q = `
		INSERT INTO chats (id, kind, title, first_name, last_name, username, locale, generation_mode)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			kind = EXCLUDED.kind,
			title = EXCLUDED.title,
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name,
			username = EXCLUDED.username
		RETURNING ` + chatColumns
	return scanChat(tx.QueryRowContext(ctx, q,
		c.ID, c.Kind, c.Title, c.FirstName, c.LastName, c.Username, c.Locale, c.GenerationMode))
}

// Upsert inserts or updates a chat keyed by its Telegram id and returns the row.
func (chats) Upsert(ctx context.Context, c *Chat) (*Chat, error) {
	const q = `
		INSERT INTO chats (id, kind, title, first_name, last_name, username, locale, generation_mode)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			kind = EXCLUDED.kind,
			title = EXCLUDED.title,
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name,
			username = EXCLUDED.username
		RETURNING ` + chatColumns
	return scanChat(db.QueryRowContext(ctx, q,
		c.ID, c.Kind, c.Title, c.FirstName, c.LastName, c.Username, c.Locale, c.GenerationMode))
}

func (chats) Get(ctx context.Context, id int64) (*Chat, error) {
	row := db.QueryRowContext(ctx, `SELECT `+chatColumns+` FROM chats WHERE id = $1`, id)
	return scanChat(row)
}

func (chats) Enable(ctx context.Context, id int64) error {
	_, err := db.ExecContext(ctx, `UPDATE chats SET active_at = NOW() WHERE id = $1`, id)
	return err
}

func (chats) Disable(ctx context.Context, id int64) error {
	_, err := db.ExecContext(ctx, `UPDATE chats SET active_at = NULL WHERE id = $1`, id)
	return err
}

func (chats) Touch(ctx context.Context, id int64) error {
	_, err := db.ExecContext(ctx, `UPDATE chats SET active_at = NOW() WHERE id = $1`, id)
	return err
}

func (chats) SetRandom(ctx context.Context, id int64, value int) error {
	_, err := db.ExecContext(ctx, `UPDATE chats SET random = $2 WHERE id = $1`, id, value)
	return err
}

func (chats) SetWinner(ctx context.Context, id int64, label sql.NullString) error {
	_, err := db.ExecContext(ctx, `UPDATE chats SET winner = $2 WHERE id = $1`, id, label)
	return err
}

func (chats) SetLocale(ctx context.Context, id int64, locale string) error {
	_, err := db.ExecContext(ctx, `UPDATE chats SET locale = $2 WHERE id = $1`, id, locale)
	return err
}

func (chats) SetGenerationMode(ctx context.Context, id int64, mode GenerationMode) error {
	_, err := db.ExecContext(ctx, `UPDATE chats SET generation_mode = $2 WHERE id = $1`, id, mode)
	return err
}

func (chats) SetSystemPrompt(ctx context.Context, id int64, prompt sql.NullString) error {
	_, err := db.ExecContext(ctx, `UPDATE chats SET system_prompt = $2 WHERE id = $1`, id, prompt)
	return err
}

func (chats) SetMemory(ctx context.Context, id int64, memory sql.NullString) error {
	_, err := db.ExecContext(ctx, `UPDATE chats SET memory = $2 WHERE id = $1`, id, memory)
	return err
}

func (chats) SetCaptcha(ctx context.Context, id int64, enabled bool) error {
	const q = `UPDATE chats SET
		captcha_enabled_at = CASE WHEN $2 THEN NOW() ELSE NULL END
		WHERE id = $1`
	_, err := db.ExecContext(ctx, q, id, enabled)
	return err
}

func (chats) SetIdle(ctx context.Context, id int64, days sql.NullInt64) error {
	_, err := db.ExecContext(ctx, `UPDATE chats SET idle_days = $2 WHERE id = $1`, id, days)
	return err
}

func (chats) SetGreetingText(ctx context.Context, id int64, text sql.NullString) error {
	_, err := db.ExecContext(ctx, `UPDATE chats SET greeting_text = $2 WHERE id = $1`, id, text)
	return err
}

func (chats) SetIdlePokedAt(ctx context.Context, id int64, at time.Time) error {
	_, err := db.ExecContext(ctx, `UPDATE chats SET idle_poked_at = $2 WHERE id = $1`, id, at)
	return err
}

func (chats) SetMemorySummarizedAt(ctx context.Context, id int64, at time.Time) error {
	_, err := db.ExecContext(ctx, `UPDATE chats SET memory_summarized_at = $2 WHERE id = $1`, id, at)
	return err
}

func (chats) Active(ctx context.Context) ([]*Chat, error) {
	rows, err := db.QueryContext(ctx, `SELECT `+chatColumns+` FROM chats WHERE active_at IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Chat
	for rows.Next() {
		c, err := scanChat(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (chats) PairsCount(ctx context.Context, id int64) (int, error) {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(1) FROM pairs WHERE chat_id = $1`, id).Scan(&count)
	return count, err
}
