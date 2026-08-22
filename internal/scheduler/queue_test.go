package scheduler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBackoffDoublesThenFlattens(t *testing.T) {
	want := []time.Duration{
		time.Minute,
		2 * time.Minute,
		4 * time.Minute,
		8 * time.Minute,
		16 * time.Minute,
		maxBackoff,
		maxBackoff,
	}
	for i, d := range want {
		assert.Equal(t, d, backoff(i+1), "attempt %d", i+1)
	}
}

// A job failing for days must not shift the delay past what a duration holds.
func TestBackoffStaysCappedForALongOutage(t *testing.T) {
	for _, attempts := range []int{17, 64, 1000} {
		assert.Equal(t, maxBackoff, backoff(attempts), "attempts %d", attempts)
	}
}

func TestBackoffFloorsAtTheFirstStep(t *testing.T) {
	assert.Equal(t, baseBackoff, backoff(0))
	assert.Equal(t, baseBackoff, backoff(-3))
}
