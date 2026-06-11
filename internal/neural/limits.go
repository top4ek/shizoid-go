package neural

import (
	"sync"
	"time"
)

type usageLedger struct {
	mu      sync.Mutex
	entries map[string]dayCount
	now     func() time.Time
}

type dayCount struct {
	day   string
	count int
}

func newUsageLedger() *usageLedger {
	return &usageLedger{
		entries: make(map[string]dayCount),
		now:     time.Now,
	}
}

func (l *usageLedger) reserve(name string, limit int) bool {
	if limit <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.entries[name]
	if e.day != l.today() {
		e = dayCount{day: l.today()}
	}
	if e.count >= limit {
		return false
	}
	e.count++
	l.entries[name] = e
	return true
}

func (l *usageLedger) setCount(name, day string, count int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries[name] = dayCount{day: day, count: count}
}

func (l *usageLedger) today() string {
	return l.now().Format("2006-01-02")
}
