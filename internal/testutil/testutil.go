// Package testutil provides common testing utilities for the goflow library.
package testutil

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestTimeout is the default timeout for tests
const TestTimeout = 5 * time.Second

// WithTimeout creates a context with the default test timeout
func WithTimeout(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), TestTimeout)
}

// AssertNoError fails the test if err is not nil
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// AssertError fails the test if err is nil
func AssertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// AssertEqual fails the test if got != want
func AssertEqual[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// AssertNotEqual fails the test if got == want
func AssertNotEqual[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got == want {
		t.Fatalf("got %v, expected not equal to %v", got, want)
	}
}

// Eventually repeatedly checks a condition until it's true or timeout is reached.
// This is useful for testing asynchronous operations without using fixed time.Sleep().
//
// Example:
//
//	Eventually(t, func() bool {
//	    return atomic.LoadInt32(&counter) == expected
//	}, 500*time.Millisecond, 10*time.Millisecond)
func Eventually(t *testing.T, condition func() bool, timeout, interval time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(interval)
	}

	t.Fatalf("condition not met within %v", timeout)
}

// EventuallyWithContext is like Eventually but respects context cancellation.
func EventuallyWithContext(t *testing.T, ctx context.Context, condition func() bool, interval time.Duration) {
	t.Helper()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			t.Fatalf("condition not met before context canceled: %v", ctx.Err())
		case <-ticker.C:
			if condition() {
				return
			}
		}
	}
}

// WaitForInt32 waits for an atomic int32 to reach the expected value.
// This is a common pattern for testing concurrent operations.
func WaitForInt32(t *testing.T, value *int32, expected int32, timeout time.Duration) {
	t.Helper()
	Eventually(t, func() bool {
		return atomic.LoadInt32(value) == expected
	}, timeout, 10*time.Millisecond)
}

// WaitForInt64 waits for an atomic int64 to reach the expected value.
func WaitForInt64(t *testing.T, value *int64, expected int64, timeout time.Duration) {
	t.Helper()
	Eventually(t, func() bool {
		return atomic.LoadInt64(value) == expected
	}, timeout, 10*time.Millisecond)
}

// AssertEventually is like Eventually but with a default timeout of 1 second.
func AssertEventually(t *testing.T, condition func() bool) {
	t.Helper()
	Eventually(t, condition, time.Second, 10*time.Millisecond)
}

// CallbackTracker tracks whether a callback was called and how many times.
// Useful for testing callback functionality.
type CallbackTracker struct {
	called int32
	value  interface{}
}

// NewCallbackTracker creates a new callback tracker.
func NewCallbackTracker() *CallbackTracker {
	return &CallbackTracker{}
}

// Mark marks the callback as called and optionally stores a value.
func (ct *CallbackTracker) Mark(value ...interface{}) {
	atomic.AddInt32(&ct.called, 1)
	if len(value) > 0 {
		ct.value = value[0]
	}
}

// Called returns true if the callback was called at least once.
func (ct *CallbackTracker) Called() bool {
	return atomic.LoadInt32(&ct.called) > 0
}

// CallCount returns the number of times the callback was called.
func (ct *CallbackTracker) CallCount() int {
	return int(atomic.LoadInt32(&ct.called))
}

// Value returns the last value passed to Mark.
func (ct *CallbackTracker) Value() interface{} {
	return ct.value
}

// Reset resets the tracker.
func (ct *CallbackTracker) Reset() {
	atomic.StoreInt32(&ct.called, 0)
	ct.value = nil
}

// AssertCalled fails if the callback was not called.
func (ct *CallbackTracker) AssertCalled(t *testing.T) {
	t.Helper()
	if !ct.Called() {
		t.Fatal("callback was not called")
	}
}

// AssertNotCalled fails if the callback was called.
func (ct *CallbackTracker) AssertNotCalled(t *testing.T) {
	t.Helper()
	if ct.Called() {
		t.Fatalf("callback was called %d times, expected 0", ct.CallCount())
	}
}

// AssertCallCount fails if the callback was not called exactly n times.
func (ct *CallbackTracker) AssertCallCount(t *testing.T, expected int) {
	t.Helper()
	actual := ct.CallCount()
	if actual != expected {
		t.Fatalf("callback called %d times, expected %d", actual, expected)
	}
}
