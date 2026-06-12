package ids

import (
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"
)

func TestText(t *testing.T) {
	mock := &models.Update{
		Message: &models.Message{
			From: &models.User{
				ID: 234,
			},
			Chat: models.Chat{
				ID:   123,
				Type: models.ChatTypePrivate,
			},
		},
	}
	got := strings.TrimSpace(text("ru", mock))

	assert.Contains(t, got, "123")
	assert.Contains(t, got, "234")
	assert.Contains(t, got, "private")
}

func TestReplyTextIsValidMarkdownV2(t *testing.T) {
	update := &models.Update{
		Message: &models.Message{
			From: &models.User{ID: 234},
			Chat: models.Chat{
				ID:   123,
				Type: models.ChatTypePrivate,
			},
		},
	}
	got := strings.TrimSpace(text("en", update))

	assert.Equal(t, models.ParseModeMarkdown, replyParseMode)
	assert.Contains(t, got, `\(private\)`)
	assert.NotContains(t, got, "(private)", "parentheses must be escaped for MarkdownV2")
}
