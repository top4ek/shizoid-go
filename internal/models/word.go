package models

import (
	"context"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

type words struct{ db DBTX }

// wordPair is one (id, word) row; all three lookups below read it and fold the
// rows into a map keyed either way.
type wordPair struct {
	id   int64
	word string
}

func scanWordPair(rows pgx.Rows) (wordPair, error) {
	var w wordPair
	err := rows.Scan(&w.id, &w.word)
	return w, err
}

// inPlaceholders renders "$1,$2,..." for an IN list together with its args.
func inPlaceholders[T any](vals []T) (string, []any) {
	ph := make([]string, len(vals))
	args := make([]any, len(vals))
	for i, v := range vals {
		ph[i] = "$" + strconv.Itoa(i+1)
		args[i] = v
	}
	return strings.Join(ph, ","), args
}

// EnsureIDs upserts the words and returns their ids in one round-trip
// (the no-op DO UPDATE makes RETURNING yield pre-existing rows too).
func (r words) EnsureIDs(ctx context.Context, list []string) (map[string]int64, error) {
	uniq := uniqueNonEmpty(list)
	out := make(map[string]int64, len(uniq))
	if len(uniq) == 0 {
		return out, nil
	}
	pairs, err := queryRows(ctx, r.db, `
		INSERT INTO words (word)
		SELECT unnest($1::text[])
		ON CONFLICT (word) DO UPDATE SET word = EXCLUDED.word
		RETURNING id, word`, []any{uniq}, scanWordPair)
	if err != nil {
		return nil, err
	}
	for _, p := range pairs {
		out[p.word] = p.id
	}
	return out, nil
}

func (r words) ToIDs(ctx context.Context, list []string) (map[string]int64, error) {
	uniq := uniqueNonEmpty(list)
	out := make(map[string]int64, len(uniq))
	if len(uniq) == 0 {
		return out, nil
	}
	ph, args := inPlaceholders(uniq)
	pairs, err := queryRows(ctx, r.db,
		`SELECT id, word FROM words WHERE word IN (`+ph+`)`, args, scanWordPair)
	if err != nil {
		return nil, err
	}
	for _, p := range pairs {
		out[p.word] = p.id
	}
	return out, nil
}

func (r words) ToWords(ctx context.Context, ids []int64) (map[int64]string, error) {
	out := make(map[int64]string, len(ids))
	uniq := uniqueIDs(ids)
	if len(uniq) == 0 {
		return out, nil
	}
	ph, args := inPlaceholders(uniq)
	pairs, err := queryRows(ctx, r.db,
		`SELECT id, word FROM words WHERE id IN (`+ph+`)`, args, scanWordPair)
	if err != nil {
		return nil, err
	}
	for _, p := range pairs {
		out[p.id] = p.word
	}
	return out, nil
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
