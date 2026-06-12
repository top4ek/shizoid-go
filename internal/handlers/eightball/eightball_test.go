package eightball

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"shizoid/internal/locale"
)

func TestResponse_Payload(t *testing.T) {
	got := response("ru", "test?", 234)
	replies := locale.List("ru", "eightball.replies")
	assert.NotEmpty(t, replies)
	assert.Contains(t, replies, got)
}

func TestResponse_Empty(t *testing.T) {
	got := response("ru", "", 234)
	empty := locale.List("ru", "eightball.empty")
	assert.NotEmpty(t, empty)
	assert.Contains(t, empty, got)
}

func TestDigest(t *testing.T) {
	now := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)

	result := digest("test", 234, now)

	assert.Equal(t, uint64(0xa94a8fe5cba3e4fc), result)
}
