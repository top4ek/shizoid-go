package utils

import (
	"github.com/go-telegram/bot/models"
	"testing"

	"shizoid/internal/config"

	"github.com/stretchr/testify/assert"
)

func update() *models.Update {
	return &models.Update{
		Message: &models.Message{
			From: &models.User{
				ID: 234,
			},
		},
	}
}

func withBotOwners(t *testing.T, owners []int64, testFn func()) {
	old := config.Environment.BotOwners
	t.Cleanup(func() {
		config.Environment.BotOwners = old
	})

	config.Environment.BotOwners = owners
	testFn()
}

func TestIsBotOwner_True(t *testing.T) {
	withBotOwners(t, []int64{123, 234, 345}, func() {
		result := IsBotOwner(update())

		assert.True(t, result)
	})
}

func TestIsBotOwner_False(t *testing.T) {
	withBotOwners(t, []int64{123, 456, 345}, func() {
		result := IsBotOwner(update())

		assert.False(t, result)
	})
}

func TestPickRandomString(t *testing.T) {
	input := []string{"a", "b", "c", "d"}

	result := PickRandomString(input)

	assert.Contains(t, input, result)
}

func updateText(str string) *models.Update {
	return &models.Update{
		Message: &models.Message{
			Text: "/command " + str,
			From: &models.User{
				ID: 234,
			},
		},
	}
}

func TestExtractCommandPayloadText_EmptyPayload(t *testing.T) {
	str := ""
	assert.Equal(t, str, ExtractCommandPayloadText((updateText(str))))
}

func TestExtractCommandPayloadText_WithPayload(t *testing.T) {
	str := "with test    string"
	assert.Equal(t, str, ExtractCommandPayloadText((updateText(str))))
}
