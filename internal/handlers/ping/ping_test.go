package ping

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"shizoid/internal/locale"
)

func TestPingRepliesLocalized(t *testing.T) {
	replies := locale.List("ru", "ping")
	assert.NotEmpty(t, replies)
	assert.Contains(t, replies, locale.Random("ru", "ping"))
}
