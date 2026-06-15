package locale

import (
	"os"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"shizoid/internal/logger"
)

func TestMain(m *testing.M) {
	logger.Init(true, "")
	os.Exit(m.Run())
}

func TestAvailable(t *testing.T) {
	got := Available()
	require.NotEmpty(t, got)
	assert.True(t, slices.Contains(got, "ru"))
	assert.True(t, slices.Contains(got, "en"))
	assert.True(t, slices.IsSorted(got))
}

func TestHas(t *testing.T) {
	assert.True(t, Has("ru"))
	assert.True(t, Has("en"))
	assert.False(t, Has("missing"))
}

func TestT(t *testing.T) {
	got := T("ru", "lang.current", "lang", "ru")
	assert.Contains(t, got, "ru")

	assert.Equal(t, "missing.key", T("ru", "missing.key"))
}

func TestTUnknownVariable(t *testing.T) {
	got := T("ru", "lang.current", "other", "x")
	assert.Contains(t, got, "%{lang}")
}

func TestList(t *testing.T) {
	ok := List("ru", "ok")
	assert.NotEmpty(t, ok)

	ping := List("ru", "ping")
	assert.NotEmpty(t, ping)

	assert.Nil(t, List("ru", "missing.key"))
}

func TestRandom(t *testing.T) {
	replies := List("ru", "ping")
	require.NotEmpty(t, replies)
	assert.Contains(t, replies, Random("ru", "ping"))
}

func TestSymbols(t *testing.T) {
	symbols := Symbols("en", "captcha.symbols")
	require.NotEmpty(t, symbols)
	for _, s := range symbols {
		assert.NotEmpty(t, s.Emoji)
		assert.NotEmpty(t, s.Word)
	}
}
