package prompt

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"

	"shizoid/internal/models"
)

func TestCurrentPromptText(t *testing.T) {
	chat := &models.Chat{
		SystemPrompt: sql.NullString{String: "  Be brief.  ", Valid: true},
	}
	assert.Contains(t, currentPromptText(chat, "en"), "Be brief.")

	chat.SystemPrompt = sql.NullString{}
	assert.Equal(t, "Chat system prompt is not set.", currentPromptText(chat, "en"))

	chat.SystemPrompt = sql.NullString{String: "   ", Valid: true}
	assert.Equal(t, "Chat system prompt is not set.", currentPromptText(chat, "en"))
}
