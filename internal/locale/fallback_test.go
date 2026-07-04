package locale

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestT_FallsBackToRuForUnknownLocale(t *testing.T) {
	got := T("de", "winner.default")
	want := T("ru", "winner.default")
	require.NotEqual(t, "winner.default", want, "fixture key must exist in ru")
	assert.Equal(t, want, got)
}

func TestT_MissingKeyEverywhereReturnsKey(t *testing.T) {
	assert.Equal(t, "no.such.key", T("en", "no.such.key"))
}

func TestLoad(t *testing.T) {
	assert.NoError(t, Load())
}
