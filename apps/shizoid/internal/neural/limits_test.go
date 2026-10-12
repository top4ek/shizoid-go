package neural

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDailyLimitReleasedOnFailedCall(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New([]Provider{
		{Name: "flaky", BaseURL: srv.URL + "/v1", Model: "m", TimeoutSeconds: 5, DailyLimit: 1},
	}, nil)

	for range 3 {
		_, err := reply(c, context.Background(), "")
		require.Error(t, err)
	}
	assert.Equal(t, 3, calls, "failed calls must not consume the daily budget")
}

func TestDailyLimitReleasedOnEmptyResponse(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":""}}]}`))
	}))
	defer srv.Close()

	c := New([]Provider{
		{Name: "empty", BaseURL: srv.URL + "/v1", Model: "m", TimeoutSeconds: 5, DailyLimit: 1},
	}, nil)

	for range 2 {
		_, err := reply(c, context.Background(), "")
		require.Error(t, err)
	}
	assert.Equal(t, 2, calls, "empty completions must not consume the daily budget")
}

func TestUsageLedgerMidnightRelease(t *testing.T) {
	current := time.Date(2026, 1, 1, 23, 59, 59, 0, time.UTC)
	l := newUsageLedger()
	l.now = func() time.Time { return current }

	firstDay, ok := l.reserve("p", 1)
	require.True(t, ok)

	current = current.Add(5 * time.Minute)

	_, ok = l.reserve("p", 1)
	require.True(t, ok)

	l.release("p", 1, firstDay)
	_, ok = l.reserve("p", 1)
	require.False(t, ok)
}

func TestUsageLedgerRelease(t *testing.T) {
	l := newUsageLedger()
	day, ok := l.reserve("p", 1)
	require.True(t, ok)

	_, ok = l.reserve("p", 1)
	require.False(t, ok)

	l.release("p", 1, day)
	_, ok = l.reserve("p", 1)
	assert.True(t, ok)

	// double release must not create budget below zero: after draining back to
	// zero, exactly one reserve fits the limit again
	l.release("p", 1, day)
	l.release("p", 1, day)
	_, ok = l.reserve("p", 1)
	require.True(t, ok)

	_, ok = l.reserve("p", 1)
	assert.False(t, ok)

	// unlimited providers are untouched
	l.release("unlimited", 0, day)

	_, ok = l.reserve("unlimited", 0)
	assert.True(t, ok)
}
