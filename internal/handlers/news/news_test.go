package news

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"

	"shizoid/internal/models"
)

func TestCurrentStyleText(t *testing.T) {
	chat := &models.Chat{News: sql.NullString{String: "sport", Valid: true}}
	assert.Contains(t, currentStyleText(chat, "en"), "sport")

	chat.News = sql.NullString{}
	assert.Equal(t, "The daily news issue is off.", currentStyleText(chat, "en"))

	chat.News = sql.NullString{String: "", Valid: true}
	assert.Equal(t, "The daily news issue is off.", currentStyleText(chat, "en"))
}
