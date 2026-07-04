package handlers

import (
	"os"
	"testing"
	"time"

	"shizoid/internal/logger"
)

func TestMain(m *testing.M) {
	logger.Init(true, "")
	os.Exit(m.Run())
}

func TestWaitCollectStats_NoPendingReturnsImmediately(t *testing.T) {
	start := time.Now()
	WaitCollectStats(5 * time.Second)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("WaitCollectStats blocked %v with no pending writes", elapsed)
	}
}

func TestWaitCollectStats_TimesOutOnStuckWriter(t *testing.T) {
	statsWG.Add(1)
	release := make(chan struct{})
	go func() {
		defer statsWG.Done()
		<-release
	}()

	start := time.Now()
	WaitCollectStats(100 * time.Millisecond)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("WaitCollectStats did not respect timeout, blocked %v", elapsed)
	}
	close(release)
	statsWG.Wait()
}
