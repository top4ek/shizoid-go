package models

import (
	"context"
	"database/sql"
	"time"
)

// Chat represents the chats table. ID equals the Telegram chat id.
type Chat struct {
	ID                 int64          `db:"id"`
	Kind               string         `db:"kind"` // private, group, supergroup, channel
	Random             int16          `db:"random"`
	Greeting           bool           `db:"greeting"`
	Winner             sql.NullString `db:"winner"`
	Locale             string         `db:"locale"`
	GenerationMode     GenerationMode `db:"generation_mode"`
	Title              sql.NullString `db:"title"`
	FirstName          sql.NullString `db:"first_name"`
	LastName           sql.NullString `db:"last_name"`
	Username           sql.NullString `db:"username"`
	ActiveAt           sql.NullTime   `db:"active_at"`
	CaptchaEnabledAt   sql.NullTime   `db:"captcha_enabled_at"`
	SystemPrompt       sql.NullString `db:"system_prompt"`
	Memory             sql.NullString `db:"memory"`
	MemorySummarizedAt sql.NullTime   `db:"memory_summarized_at"`
	News               sql.NullString `db:"news"`
}

type chats struct{ db DBTX }

// Enabled reports whether the bot is active in this chat.
func (c *Chat) Enabled() bool {
	return c.ActiveAt.Valid
}

// WinnerEnabled reports whether daily winner selection is configured.
func (c *Chat) WinnerEnabled() bool {
	return c.Winner.Valid && c.Winner.String != ""
}

// NewsEnabled reports whether the daily news issue is configured. The stored
// value is the issue style ("sport", "general news", a tone), so an empty or
// NULL value means the issue is off for this chat.
func (c *Chat) NewsEnabled() bool {
	return c.News.Valid && c.News.String != ""
}

// CaptchaEnabled reports whether captcha is active for new members.
func (c *Chat) CaptchaEnabled() bool {
	return c.CaptchaEnabledAt.Valid
}

const chatColumns = `id, kind, random, greeting, winner, locale, generation_mode,
	title, first_name, last_name, username, active_at,
	captcha_enabled_at, system_prompt, memory, memory_summarized_at, news`

func scanChat(row interface{ Scan(...any) error }) (*Chat, error) {
	c := &Chat{}
	err := row.Scan(
		&c.ID, &c.Kind, &c.Random, &c.Greeting, &c.Winner, &c.Locale, &c.GenerationMode,
		&c.Title, &c.FirstName, &c.LastName, &c.Username, &c.ActiveAt,
		&c.CaptchaEnabledAt, &c.SystemPrompt, &c.Memory, &c.MemorySummarizedAt, &c.News,
	)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// Upsert inserts or updates a chat keyed by its Telegram id and returns the row.
func (r chats) Upsert(ctx context.Context, c *Chat) (*Chat, error) {
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
	return scanChat(r.db.QueryRow(ctx, q,
		c.ID, c.Kind, c.Title, c.FirstName, c.LastName, c.Username, c.Locale, c.GenerationMode))
}

func (r chats) Get(ctx context.Context, id int64) (*Chat, error) {
	row := r.db.QueryRow(ctx, `SELECT `+chatColumns+` FROM chats WHERE id = $1`, id)
	return scanChat(row)
}

func (r chats) Enable(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `UPDATE chats SET active_at = NOW() WHERE id = $1`, id)
	return err
}

func (r chats) Disable(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `UPDATE chats SET active_at = NULL WHERE id = $1`, id)
	return err
}

func (r chats) SetRandom(ctx context.Context, id int64, value int) error {
	_, err := r.db.Exec(ctx, `UPDATE chats SET random = $2 WHERE id = $1`, id, value)
	return err
}

func (r chats) SetWinner(ctx context.Context, id int64, label sql.NullString) error {
	_, err := r.db.Exec(ctx, `UPDATE chats SET winner = $2 WHERE id = $1`, id, label)
	return err
}

func (r chats) SetLocale(ctx context.Context, id int64, locale string) error {
	_, err := r.db.Exec(ctx, `UPDATE chats SET locale = $2 WHERE id = $1`, id, locale)
	return err
}

func (r chats) SetGenerationMode(ctx context.Context, id int64, mode GenerationMode) error {
	_, err := r.db.Exec(ctx, `UPDATE chats SET generation_mode = $2 WHERE id = $1`, id, mode)
	return err
}

func (r chats) SetSystemPrompt(ctx context.Context, id int64, prompt sql.NullString) error {
	_, err := r.db.Exec(ctx, `UPDATE chats SET system_prompt = $2 WHERE id = $1`, id, prompt)
	return err
}

func (r chats) SetMemory(ctx context.Context, id int64, memory sql.NullString) error {
	_, err := r.db.Exec(ctx, `UPDATE chats SET memory = $2 WHERE id = $1`, id, memory)
	return err
}

func (r chats) SetNews(ctx context.Context, id int64, style sql.NullString) error {
	_, err := r.db.Exec(ctx, `UPDATE chats SET news = $2 WHERE id = $1`, id, style)
	return err
}

func (r chats) SetCaptcha(ctx context.Context, id int64, enabled bool) error {
	const q = `UPDATE chats SET
		captcha_enabled_at = CASE WHEN $2 THEN NOW() ELSE NULL END
		WHERE id = $1`
	_, err := r.db.Exec(ctx, q, id, enabled)
	return err
}

func (r chats) SetGreeting(ctx context.Context, id int64, enabled bool) error {
	_, err := r.db.Exec(ctx, `UPDATE chats SET greeting = $2 WHERE id = $1`, id, enabled)
	return err
}

func (r chats) SetMemorySummarizedAt(ctx context.Context, id int64, at time.Time) error {
	_, err := r.db.Exec(ctx, `UPDATE chats SET memory_summarized_at = $2 WHERE id = $1`, id, at)
	return err
}

func (r chats) Active(ctx context.Context) ([]*Chat, error) {
	rows, err := r.db.Query(ctx, `SELECT `+chatColumns+` FROM chats WHERE active_at IS NOT NULL`)
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

func (r chats) PairsCount(ctx context.Context, id int64) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT COUNT(1) FROM pairs WHERE chat_id = $1`, id).Scan(&count)
	return count, err
}
