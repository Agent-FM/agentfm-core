package testutil

import (
	"context"
	"testing"
	"time"
)

// WithTimeout returns a context bounded to d, cancelled on test cleanup.
// Prefer over context.Background() so every test's network / sub-process
// call has a deterministic upper bound and no goroutine leaks.
func WithTimeout(t testing.TB, d time.Duration) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	t.Cleanup(cancel)
	return ctx
}

// Eventually waits up to d for predicate to return true, polling every
// 10ms. Used to synchronise on async stream handlers and connection
// establishment without arbitrary time.Sleep calls.
func Eventually(t testing.TB, d time.Duration, predicate func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out after %s waiting for: %s", d, msg)
}

// WaitFor blocks on done until either it closes or d elapses. Used to
// synchronise on a handler goroutine that signals completion via a
// channel — the Go-idiomatic alternative to sync.WaitGroup.Wait with
// a bound.
func WaitFor(t testing.TB, done <-chan struct{}, d time.Duration, msg string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("timed out after %s waiting for: %s", d, msg)
	}
}
