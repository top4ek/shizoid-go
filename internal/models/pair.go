package models

import (
	"context"
	"database/sql"

	"strconv"
	"strings"
)

// Pair represents the pairs table.
type Pair struct {
	ID       int64         `db:"id"`
	ChatID   int64         `db:"chat_id"`
	FirstID  sql.NullInt64 `db:"first_id"`
	SecondID sql.NullInt64 `db:"second_id"`
}

type pairs struct{ db DBTX }

type matchKind int

const (
	matchAny matchKind = iota
	matchNull
	matchEq
	matchIn
)

// Matcher constrains a pair's first_id or second_id during a fetch.
type Matcher struct {
	kind matchKind
	val  sql.NullInt64
	vals []int64
}

func MatchAny() Matcher  { return Matcher{kind: matchAny} }
func MatchNull() Matcher { return Matcher{kind: matchNull} }
func MatchEq(v sql.NullInt64) Matcher {
	if !v.Valid {
		return Matcher{kind: matchNull}
	}
	return Matcher{kind: matchEq, val: v}
}
func MatchIn(vals []int64) Matcher { return Matcher{kind: matchIn, vals: vals} }

// ReplyRow is a candidate continuation word with its frequency.
type ReplyRow struct {
	WordID sql.NullInt64
	Count  int
}

// PairWithReplies is a fetched pair together with its ordered replies.
type PairWithReplies struct {
	ID       int64
	SecondID sql.NullInt64
	Replies  []ReplyRow
}

func (m Matcher) condition(column string, args *[]any) string {
	switch m.kind {
	case matchNull:
		return column + " IS NULL"
	case matchEq:
		*args = append(*args, m.val.Int64)
		return column + " = $" + strconv.Itoa(len(*args))
	case matchIn:
		if len(m.vals) == 0 {
			return "false"
		}
		placeholders := make([]string, len(m.vals))
		for i, v := range m.vals {
			*args = append(*args, v)
			placeholders[i] = "$" + strconv.Itoa(len(*args))
		}
		return column + " IN (" + strings.Join(placeholders, ",") + ")"
	default:
		return ""
	}
}

// FetchPair picks one random matching pair together with its replies in a
// single round-trip. The random pick is deliberate: it is what makes the
// Markov walk a "бредогенератор".
func (r pairs) FetchPair(ctx context.Context, chatID int64, first, second Matcher) (*PairWithReplies, error) {
	args := []any{chatID}
	conds := []string{"chat_id = $1"}
	if c := first.condition("first_id", &args); c != "" {
		conds = append(conds, c)
	}
	if c := second.condition("second_id", &args); c != "" {
		conds = append(conds, c)
	}
	q := `
		WITH picked AS (
			SELECT id, second_id FROM pairs
			WHERE ` + strings.Join(conds, " AND ") + `
			ORDER BY RANDOM() LIMIT 1
		)
		SELECT p.id, p.second_id, r.word_id, r.count
		FROM picked p
		LEFT JOIN replies r ON r.pair_id = p.id
		ORDER BY r.count DESC`

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var p *PairWithReplies
	for rows.Next() {
		var id int64
		var secondID, wordID sql.NullInt64
		var count sql.NullInt32
		if err := rows.Scan(&id, &secondID, &wordID, &count); err != nil {
			return nil, err
		}
		if p == nil {
			p = &PairWithReplies{ID: id, SecondID: secondID}
		}
		// count is NULL only when the LEFT JOIN found no replies at all;
		// a reply with NULL word_id still carries a count.
		if count.Valid {
			p.Replies = append(p.Replies, ReplyRow{WordID: wordID, Count: int(count.Int32)})
		}
	}
	return p, rows.Err()
}

// Trigram is one Markov transition: (first, second) -> third.
type Trigram struct {
	First  sql.NullInt64
	Second sql.NullInt64
	Third  sql.NullInt64
}

// LearnTrigrams upserts all trigrams of a message in one round-trip: pairs are
// upserted from the deduplicated keys, then reply counts are incremented by
// how many times each transition occurred.
func (r pairs) LearnTrigrams(ctx context.Context, chatID int64, trigrams []Trigram) error {
	if len(trigrams) == 0 {
		return nil
	}
	firsts := make([]*int64, len(trigrams))
	seconds := make([]*int64, len(trigrams))
	thirds := make([]*int64, len(trigrams))
	for i, t := range trigrams {
		firsts[i] = nullableInt64(t.First)
		seconds[i] = nullableInt64(t.Second)
		thirds[i] = nullableInt64(t.Third)
	}
	const q = `
		WITH input AS (
			SELECT first_id, second_id, third_id, COUNT(*)::int AS cnt
			FROM unnest($2::bigint[], $3::bigint[], $4::bigint[]) AS t(first_id, second_id, third_id)
			GROUP BY first_id, second_id, third_id
		),
		pair_keys AS (
			SELECT DISTINCT first_id, second_id FROM input
		),
		upserted AS (
			INSERT INTO pairs (chat_id, first_id, second_id)
			SELECT $1, first_id, second_id FROM pair_keys
			ON CONFLICT (chat_id, first_id, second_id) DO UPDATE SET chat_id = EXCLUDED.chat_id
			RETURNING id, first_id, second_id
		)
		INSERT INTO replies (pair_id, word_id, count)
		SELECT u.id, i.third_id, i.cnt
		FROM upserted u
		JOIN input i ON i.first_id IS NOT DISTINCT FROM u.first_id
			AND i.second_id IS NOT DISTINCT FROM u.second_id
		ON CONFLICT (pair_id, word_id) DO UPDATE SET count = replies.count + EXCLUDED.count`
	_, err := r.db.Exec(ctx, q, chatID, firsts, seconds, thirds)
	return err
}

func nullableInt64(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}
