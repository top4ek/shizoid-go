package idle

import (
	"database/sql"
	"fmt"
	"hash/fnv"
	"time"
)

const (
	idleWindowStartHour = 9
	idleWindowHours     = 12 // 9..20 UTC inclusive
)

// ScheduledHour returns the UTC hour (9–20) when this chat should be poked today.
func ScheduledHour(chatID int64, day time.Time) int {
	day = day.UTC()
	key := fmt.Sprintf("%d:%04d-%02d-%02d", chatID, day.Year(), int(day.Month()), day.Day())
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	return idleWindowStartHour + int(h.Sum64()%idleWindowHours)
}

// PokedTodayUTC reports whether idle_poked_at falls on the same UTC calendar day as now.
func PokedTodayUTC(poked sql.NullTime, now time.Time) bool {
	if !poked.Valid {
		return false
	}
	y1, m1, d1 := poked.Time.UTC().Date()
	y2, m2, d2 := now.UTC().Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}
