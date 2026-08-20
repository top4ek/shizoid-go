package models

import (
	"context"
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
