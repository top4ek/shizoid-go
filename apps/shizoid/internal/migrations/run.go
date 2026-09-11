// Package migrations applies database migrations using goose. Migrations live
// in the embedded sql/ directory and are versioned the "rails way": goose
// records applied versions in goose_db_version and applies only pending ones.
package migrations

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed sql/*.sql
var sqlFS embed.FS

// Run applies all pending migrations from the embedded sql/ directory.
func Run(db *sql.DB) error {
	goose.SetBaseFS(sqlFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}
	if err := goose.Up(db, "sql"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}
