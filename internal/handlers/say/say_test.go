package say

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/go-telegram/bot/models"
)

func update(text string) *models.Update {
	return &models.Update{
		Message: &models.Message{
			From: &models.User{ID: 234},
			Chat: models.Chat{ID: 123},
			Text: text,
		},
	}
}

// The owner check lives in handlers.gate (role: roleOwner); canReply is only
// responsible for rejecting an empty payload.
func TestCanReply(t *testing.T) {
	assert.True(t, canReply(update("/say blah-blah-blah")))
	assert.False(t, canReply(update("/say")))
	assert.False(t, canReply(update("/say   ")))
}

func TestText(t *testing.T) {
	assert.Equal(t, "blah-blah-blah", text(update("/say blah-blah-blah")))
}
