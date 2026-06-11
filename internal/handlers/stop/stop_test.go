package stop

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"shizoid/internal/locale"
)

func TestStopOkRepliesLocalized(t *testing.T) {
	replies := locale.List("ru", "ok")
	assert.NotEmpty(t, replies)
	assert.Contains(t, replies, locale.Random("ru", "ok"))
}

func TestCommandMetadata(t *testing.T) {
	assert.Equal(t, "stop", Command)
	assert.NotEmpty(t, Description)
}
