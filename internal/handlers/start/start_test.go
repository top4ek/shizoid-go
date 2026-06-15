package start

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"shizoid/internal/locale"
)

func TestCommandMetadata(t *testing.T) {
	assert.Equal(t, "start", Command)
	assert.NotEmpty(t, Description)
}

func TestStartOkRepliesLocalized(t *testing.T) {
	replies := locale.List("ru", "ok")
	assert.NotEmpty(t, replies)
	assert.Contains(t, replies, locale.Random("ru", "ok"))
}
