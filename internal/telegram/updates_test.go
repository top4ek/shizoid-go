package telegram

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAllowedUpdates_IncludesChatMemberAndMessage(t *testing.T) {
	updates := AllowedUpdates()
	assert.True(t, slices.Contains(updates, "chat_member"))
	assert.True(t, slices.Contains(updates, "message"))
	assert.True(t, slices.Contains(updates, "callback_query"))
	assert.False(t, slices.Contains(updates, "message_reaction"))
}
