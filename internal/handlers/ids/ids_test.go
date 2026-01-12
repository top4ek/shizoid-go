package ids

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/go-telegram/bot/models"
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
	expected := "*Chat*: 123\n*User*: 234\n*Type*: private"

	assert.Equal(t, text(mock), expected)
}
