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

type pairs struct{}

// Pairs provides persistence operations for pairs.
var Pairs pairs

type matchKind int

const (
	matchAny  matchKind = iota
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

func (pairs) FetchPair(ctx context.Context, chatID int64, first, second Matcher) (*PairWithReplies, error) {
	args := []any{chatID}
	conds := []string{"chat_id = $1"}
	if c := first.condition("first_id", &args); c != "" {
		conds = append(conds, c)
	}
	if c := second.condition("second_id", &args); c != "" {
		conds = append(conds, c)
	}
	q := `SELECT id, second_id FROM pairs WHERE ` + strings.Join(conds, " AND ") +
		` ORDER BY RANDOM() LIMIT 1`

	p := &PairWithReplies{}
	err := db.QueryRowContext(ctx, q, args...).Scan(&p.ID, &p.SecondID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx,
		`SELECT word_id, count FROM replies WHERE pair_id = $1 ORDER BY count DESC`, p.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var rr ReplyRow
		if err := rows.Scan(&rr.WordID, &rr.Count); err != nil {
			return nil, err
		}
		p.Replies = append(p.Replies, rr)
	}
	return p, rows.Err()
}

func (pairs) LearnTrigram(ctx context.Context, chatID int64, first, second, third sql.NullInt64) error {
	var pairID int64
	const upsertPair = `
		INSERT INTO pairs (chat_id, first_id, second_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (chat_id, first_id, second_id) DO UPDATE SET chat_id = EXCLUDED.chat_id
		RETURNING id`
	if err := db.QueryRowContext(ctx, upsertPair, chatID, first, second).Scan(&pairID); err != nil {
		return err
	}
	const upsertReply = `
		INSERT INTO replies (pair_id, word_id, count)
		VALUES ($1, $2, 1)
		ON CONFLICT (pair_id, word_id) DO UPDATE SET count = replies.count + 1`
	_, err := db.ExecContext(ctx, upsertReply, pairID, third)
	return err
}
