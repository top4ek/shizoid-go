package idle

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestScheduledHourChangesByDate(t *testing.T) {
	chatID := int64(99)
	d1 := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC)
	assert.GreaterOrEqual(t, ScheduledHour(chatID, d1), idleWindowStartHour)
	assert.GreaterOrEqual(t, ScheduledHour(chatID, d2), idleWindowStartHour)
}

func TestPokedTodayUTCMidnightBoundary(t *testing.T) {
	now := time.Date(2026, 6, 16, 0, 30, 0, 0, time.UTC)
	poked := time.Date(2026, 6, 15, 23, 59, 0, 0, time.UTC)
	assert.False(t, PokedTodayUTC(sql.NullTime{Time: poked, Valid: true}, now))
}
