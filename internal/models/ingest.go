package models

import (
	"context"
	"database/sql"
	"fmt"

	tgmodels "github.com/go-telegram/bot/models"
)

type ingest struct{}

// Ingest provides cross-entity transactional persistence.
var Ingest ingest

func (ingest) EnsureEntities(ctx context.Context, chat *Chat, user *User, left bool) (*Chat, *Participation, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	persistedChat, err := upsertChatTx(ctx, tx, chat)
	if err != nil {
		return nil, nil, fmt.Errorf("ensure chat: %w", err)
	}
	if err := upsertUserTx(ctx, tx, user); err != nil {
		return nil, nil, fmt.Errorf("ensure user: %w", err)
	}
	p, err := ensureParticipationTx(ctx, tx, chat.ID, user.ID, left)
	if err != nil {
		return nil, nil, fmt.Errorf("ensure participation: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	return persistedChat, p, nil
}

func (ingest) EnsureJoin(ctx context.Context, chat *Chat, members []tgmodels.User) (*Chat, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	persistedChat, err := upsertChatTx(ctx, tx, chat)
	if err != nil {
		return nil, fmt.Errorf("ensure chat: %w", err)
	}
	for i := range members {
		m := &members[i]
		if m.IsBot {
			continue
		}
		user := userFromTelegram(m)
		if err := upsertUserTx(ctx, tx, user); err != nil {
			return nil, fmt.Errorf("ensure user: %w", err)
		}
		if _, err := ensureParticipationTx(ctx, tx, chat.ID, user.ID, false); err != nil {
			return nil, fmt.Errorf("ensure participation: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return persistedChat, nil
}

func (ingest) EnsureMember(ctx context.Context, chatID int64, user *User) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	if err := upsertUserTx(ctx, tx, user); err != nil {
		return fmt.Errorf("ensure user: %w", err)
	}
	if _, err := ensureParticipationTx(ctx, tx, chatID, user.ID, false); err != nil {
		return fmt.Errorf("ensure participation: %w", err)
	}
	return tx.Commit()
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
