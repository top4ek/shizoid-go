package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSummaryJobKindString(t *testing.T) {
	assert.Equal(t, "winner", SummaryJobWinner.String())
	assert.Equal(t, "memory", SummaryJobMemory.String())
}

// A row written by a newer build and read back after a rollback carries a kind
// this build cannot run. It has to be recognizable rather than panic a lookup.
func TestSummaryJobKindRejectsUnknownValues(t *testing.T) {
	for _, k := range []SummaryJobKind{-1, 2, 300} {
		assert.False(t, k.Valid(), "kind %d", k)
		assert.Equal(t, "unknown", k.String())
	}
}
