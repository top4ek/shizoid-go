package idle

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"shizoid/internal/models"
)

func TestScheduledHourRange(t *testing.T) {
	day := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	for chatID := int64(1); chatID <= 100; chatID++ {
		h := ScheduledHour(chatID, day)
		assert.GreaterOrEqual(t, h, idleWindowStartHour)
		assert.LessOrEqual(t, h, idleWindowStartHour+idleWindowHours-1)
	}
}

func TestScheduledHourStablePerDay(t *testing.T) {
	day := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	assert.Equal(t, ScheduledHour(42, day), ScheduledHour(42, day))
	assert.NotEqual(t, ScheduledHour(1, day), ScheduledHour(2, day))
}

func TestPokedTodayUTC(t *testing.T) {
	now := time.Date(2026, 6, 15, 14, 0, 0, 0, time.UTC)
	assert.False(t, PokedTodayUTC(sql.NullTime{}, now))
	assert.False(t, PokedTodayUTC(sql.NullTime{Time: now.Add(-25 * time.Hour), Valid: true}, now))
	assert.True(t, PokedTodayUTC(sql.NullTime{Time: now.Add(-2 * time.Hour), Valid: true}, now))
}

func TestPickAsker(t *testing.T) {
	inactive := models.MemberInfo{UserID: 1, Name: "a"}
	active := []models.MemberInfo{
		{UserID: 1, Name: "a"},
		{UserID: 2, Name: "b"},
		{UserID: 3, Name: "c"},
	}
	got, ok := pickAsker(active, inactive)
	require.True(t, ok)
	assert.NotEqual(t, inactive.UserID, got.UserID)

	_, ok = pickAsker([]models.MemberInfo{inactive}, inactive)
	assert.False(t, ok)
}

func TestInterpolatePoke(t *testing.T) {
	got := interpolatePoke("%{asker} -> %{user} (%{days}d)", map[string]any{
		"asker": "@alice",
		"user":  "@bob",
		"days":  7,
	})
	assert.Equal(t, "@alice -> @bob (7d)", got)
}

func TestBuildIdleUserMessage(t *testing.T) {
	chat := &models.Chat{Locale: "ru"}
	asker := models.MemberInfo{UserID: 10, Username: "alice", Name: "alice"}
	inactive := models.MemberInfo{UserID: 20, Username: "bob", Name: "bob"}
	msg := buildIdleUserMessage(chat, asker, inactive, 5)
	assert.Contains(t, msg, "Chat locale: ru")
	assert.Contains(t, msg, "Inactive days threshold: 5")
	assert.Contains(t, msg, "@alice")
	assert.Contains(t, msg, "@bob")
}

func TestBuildIdleSystem(t *testing.T) {
	chat := &models.Chat{
		SystemPrompt: sql.NullString{String: "be funny", Valid: true},
	}
	// config.Environment may be empty in unit tests; just ensure chat prompt is included when set.
	sys := buildIdleSystem(chat)
	assert.Contains(t, sys, "be funny")
}
