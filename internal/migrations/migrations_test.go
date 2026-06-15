package migrations

import (
	"database/sql"
	"io/fs"
	"testing"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedMigrations(t *testing.T) {
	entries, err := fs.ReadDir(sqlFS, "sql")
	require.NoError(t, err)
	require.NotEmpty(t, entries)

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	assert.Contains(t, names, "00001_init.sql")
	assert.Contains(t, names, "00002_drop_neural_mode.sql")
}

func TestRunRequiresDB(t *testing.T) {
	db, err := sql.Open("postgres", "host=invalid")
	require.NoError(t, err)
	require.NoError(t, db.Close())

	err = Run(db)
	require.Error(t, err)
}
