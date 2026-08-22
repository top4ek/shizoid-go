package models

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX is the subset of *pgxpool.Pool and pgx.Tx used by repositories, so the
// same repository code runs both on the pool and inside a transaction.
type DBTX interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// queryRows runs q and drains every row through scan. Repositories share it
// instead of repeating the Query/Close/Next/Scan/Err loop.
func queryRows[T any](ctx context.Context, db DBTX, q string, args []any, scan func(pgx.Rows) (T, error)) ([]T, error) {
	rows, err := db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return collect(rows, scan)
}

// collect drains rows through scan, closing them and reporting any iteration
// error. Callers that already hold rows use this directly.
func collect[T any](rows pgx.Rows, scan func(pgx.Rows) (T, error)) ([]T, error) {
	defer rows.Close()
	var out []T
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// queryFlag runs a "SELECT <col> IS NOT NULL" style probe, reporting false for
// a row that does not exist rather than an error.
func queryFlag(ctx context.Context, db DBTX, q string, args ...any) (bool, error) {
	var flag bool
	if err := db.QueryRow(ctx, q, args...).Scan(&flag); err != nil {
		if notFound(err) {
			return false, nil
		}
		return false, err
	}
	return flag, nil
}

// notFound reports whether err is pgx's "no rows" sentinel, which most probe
// queries treat as a zero value rather than an error.
func notFound(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// Store aggregates the entity repositories over one connection pool.
type Store struct {
	pool *pgxpool.Pool

	Chats          chats
	Users          users
	Pairs          pairs
	Words          words
	Messages       messages
	Participations participations
	Winners        winners
	Greetings      greetings
	Ingest         ingest
	SummaryJobs    summaryJobs
}

// NewStore wires all repositories to the given pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{
		pool:           pool,
		Chats:          chats{db: pool},
		Users:          users{db: pool},
		Pairs:          pairs{db: pool},
		Words:          words{db: pool},
		Messages:       messages{db: pool},
		Participations: participations{db: pool},
		Winners:        winners{db: pool},
		Greetings:      greetings{db: pool},
		Ingest:         ingest{pool: pool},
		SummaryJobs:    summaryJobs{db: pool},
	}
}

// DSN builds a PostgreSQL connection string. TLS is terminated at the load
// balancer, hence sslmode=disable.
func DSN(host, port, user, password, name string) string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, name)
}

// OpenPool opens and pings a pgx connection pool.
func OpenPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 25
	cfg.MaxConnLifetime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}
