package cool_story

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"shizoid/internal/locale"
)

func TestShouldTellStory(t *testing.T) {
	assert.True(t, shouldTellStory(50, 99))
	assert.False(t, shouldTellStory(0, 50))
}

func TestLazyRepliesLocalized(t *testing.T) {
	replies := locale.List("ru", "cool_story.lazy")
	assert.NotEmpty(t, replies)
	assert.Contains(t, replies, locale.Random("ru", "cool_story.lazy"))
}
