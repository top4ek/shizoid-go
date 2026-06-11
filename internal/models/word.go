package models

import (
	"context"
	"strconv"
	"strings"
)

// Word represents the words table.
type Word struct {
	ID   int64  `db:"id"`
	Word string `db:"word"`
}

type words struct{}

// Words provides persistence operations for words.
var Words words

func (words) EnsureWords(ctx context.Context, list []string) error {
	uniq := uniqueNonEmpty(list)
	if len(uniq) == 0 {
		return nil
	}
	placeholders := make([]string, len(uniq))
	args := make([]any, len(uniq))
	for i, w := range uniq {
		placeholders[i] = "($" + strconv.Itoa(i+1) + ")"
		args[i] = w
	}
	q := `INSERT INTO words (word) VALUES ` + strings.Join(placeholders, ",") +
		` ON CONFLICT (word) DO NOTHING`
	_, err := db.ExecContext(ctx, q, args...)
	return err
}

func (words) ToIDs(ctx context.Context, list []string) (map[string]int64, error) {
	uniq := uniqueNonEmpty(list)
	out := make(map[string]int64, len(uniq))
	if len(uniq) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(uniq))
	args := make([]any, len(uniq))
	for i, w := range uniq {
		placeholders[i] = "$" + strconv.Itoa(i+1)
		args[i] = w
	}
	q := `SELECT id, word FROM words WHERE word IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var word string
		if err := rows.Scan(&id, &word); err != nil {
			return nil, err
		}
		out[word] = id
	}
	return out, rows.Err()
}

func (words) ToWords(ctx context.Context, ids []int64) (map[int64]string, error) {
	out := make(map[int64]string, len(ids))
	uniq := uniqueIDs(ids)
	if len(uniq) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(uniq))
	args := make([]any, len(uniq))
	for i, id := range uniq {
		placeholders[i] = "$" + strconv.Itoa(i+1)
		args[i] = id
	}
	q := `SELECT id, word FROM words WHERE id IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var word string
		if err := rows.Scan(&id, &word); err != nil {
			return nil, err
		}
		out[id] = word
	}
	return out, rows.Err()
}

func uniqueNonEmpty(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	var out []string
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func uniqueIDs(in []int64) []int64 {
	seen := make(map[int64]struct{}, len(in))
	var out []int64
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
