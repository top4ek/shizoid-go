package models

import (
	"context"
)

type greetings struct{ db DBTX }

func (r greetings) Set(ctx context.Context, chatID int64, text string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO greetings (chat_id, text) VALUES ($1, $2)
		 ON CONFLICT (chat_id) DO UPDATE SET text = EXCLUDED.text`,
		chatID, text)
	return err
}

func (r greetings) Get(ctx context.Context, chatID int64) (string, bool, error) {
	var text string
	err := r.db.QueryRow(ctx, `SELECT text FROM greetings WHERE chat_id = $1`, chatID).Scan(&text)
	if notFound(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return text, true, nil
}

func (r greetings) Delete(ctx context.Context, chatID int64) error {
	_, err := r.db.Exec(ctx, `DELETE FROM greetings WHERE chat_id = $1`, chatID)
	return err
}
