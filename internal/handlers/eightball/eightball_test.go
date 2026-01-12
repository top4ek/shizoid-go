package eightball

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestResponse_Payload(t *testing.T) {
	got := response("/eightball test?", 234)

	assert.Contains(t, replies, got)

}

func TestResponse_Empty(t *testing.T) {
	got := response("/eightball", 234)

	assert.Contains(t, emptyReplies, got)

}

func TestDigest(t *testing.T) {
	now := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)

	result := digest("test", 234, now)

	assert.Equal(t, uint64(0xa94a8fe5cba3e4fc), result)
}
