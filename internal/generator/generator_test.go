package generator

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"shizoid/internal/config"
	"shizoid/internal/models"
	"shizoid/internal/neural"
)

func TestCapitalize(t *testing.T) {
	assert.Equal(t, "", capitalize(""))
	assert.Equal(t, "Привет", capitalize("привет"))
	assert.Equal(t, "Hello world", capitalize("hello world"))
}

func TestEndsSentence(t *testing.T) {
	assert.True(t, endsSentence("конец."))
	assert.True(t, endsSentence("что?"))
	assert.True(t, endsSentence("ого!"))
	assert.True(t, endsSentence("ну…"))
	assert.False(t, endsSentence("слово"))
	assert.False(t, endsSentence(""))
}

func TestPickReplyEmpty(t *testing.T) {
	assert.Nil(t, pickReply(nil))
}

func TestBuildNeuralSystem(t *testing.T) {
	prev := config.Environment.AppPrompt
	config.Environment.AppPrompt = "Ты дружелюбный бот."
	defer func() { config.Environment.AppPrompt = prev }()

	chat := &models.Chat{
		SystemPrompt: sql.NullString{String: "Отвечай коротко.", Valid: true},
		Memory:       sql.NullString{String: "Вася любит котов.", Valid: true},
	}
	g := &Generator{}
	got := g.buildNeuralSystem(chat)
	assert.Equal(t, "Ты дружелюбный бот.\n\nОтвечай коротко.\n\nLong-term chat memory:\nВася любит котов.", got)

	empty := g.buildNeuralSystem(&models.Chat{})
	assert.Equal(t, "Ты дружелюбный бот.", empty)
}

func TestBuildNeuralHistory(t *testing.T) {
	botID := int64(42)
	rows := []models.MessageRow{
		{UserID: 1, Text: "current"},
		{UserID: botID, Text: "reply", IsBot: sql.NullBool{Bool: true, Valid: true}},
		{UserID: 2, Text: "hello"},
	}
	got := buildNeuralHistory(rows, botID, "current", 1)
	require.Len(t, got, 3)
	assert.Equal(t, neural.HistoryMessage{Role: "user", Name: "2", Text: "hello"}, got[0])
	assert.Equal(t, neural.HistoryMessage{Role: "assistant", Text: "reply"}, got[1])
	assert.Equal(t, neural.HistoryMessage{Role: "user", Name: "1", Text: "current"}, got[2])
}

func TestBuildNeuralHistoryDedupFromDB(t *testing.T) {
	botID := int64(1)
	rows := []models.MessageRow{
		{UserID: 5, Text: "same"},
		{UserID: 2, Text: "older"},
	}
	got := buildNeuralHistory(rows, botID, "same", 5)
	require.Len(t, got, 3)
	assert.Equal(t, "2", got[0].Name)
	assert.Equal(t, "older", got[0].Text)
	assert.Equal(t, "assistant", got[1].Role)
	assert.Equal(t, ".", got[1].Text)
	assert.Equal(t, "5", got[2].Name)
	assert.Equal(t, "same", got[2].Text)
}

func TestAppendCurrentMessageSkipsDuplicate(t *testing.T) {
	msgs := []neural.HistoryMessage{{Role: "user", Name: "1", Text: "hi"}}
	got := appendCurrentMessage(msgs, "hi", 1, 0)
	assert.Len(t, got, 1)
	got = appendCurrentMessage(msgs, "hi", 2, 0)
	assert.Len(t, got, 2)
}

func TestNormalizeRoleAlternation(t *testing.T) {
	in := []neural.HistoryMessage{
		{Role: "user", Name: "1", Text: "a"},
		{Role: "user", Name: "2", Text: "b"},
		{Role: "assistant", Text: "c"},
		{Role: "assistant", Text: "d"},
	}
	got := normalizeRoleAlternation(in)
	require.Len(t, got, 4)
	assert.Equal(t, "user", got[0].Role)
	assert.Equal(t, "assistant", got[1].Role)
	assert.Equal(t, ".", got[1].Text)
	assert.Equal(t, "user", got[2].Role)
	assert.Equal(t, "c\n\nd", got[3].Text)
}

func TestPickReplyWithinPool(t *testing.T) {
	replies := []models.ReplyRow{
		{WordID: sql.NullInt64{Int64: 1, Valid: true}, Count: 5},
		{WordID: sql.NullInt64{Int64: 2, Valid: true}, Count: 3},
	}
	for i := 0; i < 50; i++ {
		got := pickReply(replies)
		assert.NotNil(t, got)
		assert.True(t, got.WordID.Int64 == 1 || got.WordID.Int64 == 2)
	}
}
