package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunMissingConfig(t *testing.T) {
	err := run([]string{"-config", filepath.Join(t.TempDir(), "missing.yaml")})
	require.Error(t, err)
}

func TestRunInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
telegram:
  token: ""
`), 0o600))

	err := run([]string{"-config", path})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "telegram.token")
}
