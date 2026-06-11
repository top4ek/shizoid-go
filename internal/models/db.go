package models

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

var db *sql.DB

// Init wires the package-level database handle used by all accessors.
func Init(database *sql.DB) {
	db = database
}

// DB returns the initialized database handle (nil before Init).
func DB() *sql.DB {
	return db
}

// OpenDB opens and pings a PostgreSQL connection.
func OpenDB(host, port, user, password, name string) (*sql.DB, error) {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, name)

	database, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	database.SetMaxOpenConns(25)
	database.SetMaxIdleConns(10)
	database.SetConnMaxLifetime(30 * time.Minute)

	if err = database.Ping(); err != nil {
		_ = database.Close()
		return nil, err
	}

	return database, nil
}
