package models

import (
	"context"
	"database/sql"
	"fmt"

	tgmodels "github.com/go-telegram/bot/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ingest provides cross-entity transactional persistence; access via Store.Ingest.
type ingest struct{ pool *pgxpool.Pool }

func (r ingest) EnsureEntities(ctx context.Context, chat *Chat, user *User, left bool) (*Chat, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	persistedChat, err := (chats{db: tx}).Upsert(ctx, chat)
	if err != nil {
		return nil, fmt.Errorf("ensure chat: %w", err)
	}
	if _, err := (users{db: tx}).Upsert(ctx, user); err != nil {
		return nil, fmt.Errorf("ensure user: %w", err)
	}
	if err := (participations{db: tx}).Ensure(ctx, chat.ID, user.ID, left); err != nil {
		return nil, fmt.Errorf("ensure participation: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return persistedChat, nil
}

func (r ingest) EnsureJoin(ctx context.Context, chat *Chat, members []tgmodels.User) (*Chat, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	persistedChat, err := (chats{db: tx}).Upsert(ctx, chat)
	if err != nil {
		return nil, fmt.Errorf("ensure chat: %w", err)
	}
	for i := range members {
		m := &members[i]
		if m.IsBot {
			continue
		}
		user := userFromTelegram(m)
		if _, err := (users{db: tx}).Upsert(ctx, user); err != nil {
			return nil, fmt.Errorf("ensure user: %w", err)
		}
		if err := (participations{db: tx}).Ensure(ctx, chat.ID, user.ID, false); err != nil {
			return nil, fmt.Errorf("ensure participation: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return persistedChat, nil
}

func (r ingest) EnsureMember(ctx context.Context, chatID int64, user *User) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	if _, err := (users{db: tx}).Upsert(ctx, user); err != nil {
		return fmt.Errorf("ensure user: %w", err)
	}
	if err := (participations{db: tx}).Ensure(ctx, chatID, user.ID, false); err != nil {
		return fmt.Errorf("ensure participation: %w", err)
	}
	return tx.Commit(ctx)
}

func userFromTelegram(u *tgmodels.User) *User {
	m := &User{ID: u.ID}
	m.IsBot.Bool, m.IsBot.Valid = u.IsBot, true
	m.FirstName = nullString(u.FirstName)
	m.LastName = nullString(u.LastName)
	m.Username = nullString(u.Username)
	m.LanguageCode = nullString(u.LanguageCode)
	return m
}

func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
