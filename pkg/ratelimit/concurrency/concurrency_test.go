package concurrency

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/vnykmshr/goflow/internal/testutil"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
		panic    bool
	}{
		{"valid capacity", 10, false},
		{"capacity one", 1, false},
		{"zero capacity", 0, true},
		{"negative capacity", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.panic {
				limiter, err := NewSafe(tt.capacity)
				if err == nil {
					t.Error("expected error for invalid capacity")
				}
				if limiter != nil {
					t.Error("expected nil limiter on error")
				}
			} else {
				limiter, err := NewSafe(tt.capacity)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				testutil.AssertEqual(t, limiter.Capacity(), tt.capacity)
				testutil.AssertEqual(t, limiter.Available(), tt.capacity)
				testutil.AssertEqual(t, limiter.InUse(), 0)
			}
		})
	}
}

func TestNewWithConfig(t *testing.T) {
	tests := []struct {
		name              string
		config            Config
		expectedCapacity  int
		expectedAvailable int
		panic             bool
	}{
		{
			name:             "default config",
			config:           Config{Capacity: 10, InitialAvailable: -1},
			expectedCapacity: 10, expectedAvailable: 10,
		},
		{
			name:             "custom initial available",
			config:           Config{Capacity: 10, InitialAvailable: 5},
			expectedCapacity: 10, expectedAvailable: 5,
		},
		{
			name:             "initial available exceeds capacity",
			config:           Config{Capacity: 5, InitialAvailable: 10},
			expectedCapacity: 5, expectedAvailable: 5, // Clamped to capacity
		},
		{
			name:             "zero initial available",
			config:           Config{Capacity: 10, InitialAvailable: 0},
			expectedCapacity: 10, expectedAvailable: 0,
		},
		{
			name:   "invalid capacity",
			config: Config{Capacity: 0},
			panic:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.panic {
				limiter, err := NewWithConfigSafe(tt.config)
				if err == nil {
					t.Error("expected error for invalid config")
				}
				if limiter != nil {
					t.Error("expected nil limiter on error")
				}
			} else {
				limiter, err := NewWithConfigSafe(tt.config)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				testutil.AssertEqual(t, limiter.Capacity(), tt.expectedCapacity)
				testutil.AssertEqual(t, limiter.Available(), tt.expectedAvailable)
				testutil.AssertEqual(t, limiter.InUse(), tt.expectedCapacity-tt.expectedAvailable)
			}
		})
	}
}

func TestBasicAcquireRelease(t *testing.T) {
	limiter, err := NewSafe(3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be able to acquire up to capacity
	testutil.AssertEqual(t, limiter.Acquire(), true)
	testutil.AssertEqual(t, limiter.Available(), 2)
	testutil.AssertEqual(t, limiter.InUse(), 1)

	testutil.AssertEqual(t, limiter.Acquire(), true)
	testutil.AssertEqual(t, limiter.Available(), 1)
	testutil.AssertEqual(t, limiter.InUse(), 2)

	testutil.AssertEqual(t, limiter.Acquire(), true)
	testutil.AssertEqual(t, limiter.Available(), 0)
	testutil.AssertEqual(t, limiter.InUse(), 3)

	// Should fail when at capacity
	testutil.AssertEqual(t, limiter.Acquire(), false)
	testutil.AssertEqual(t, limiter.Available(), 0)
	testutil.AssertEqual(t, limiter.InUse(), 3)

	// Release should make permits available
	limiter.Release()
	testutil.AssertEqual(t, limiter.Available(), 1)
	testutil.AssertEqual(t, limiter.InUse(), 2)

	// Should be able to acquire again
	testutil.AssertEqual(t, limiter.Acquire(), true)
	testutil.AssertEqual(t, limiter.Available(), 0)
	testutil.AssertEqual(t, limiter.InUse(), 3)
}

func TestAcquireReleaseN(t *testing.T) {
	limiter, err := NewSafe(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Acquire multiple permits
	testutil.AssertEqual(t, limiter.AcquireN(3), true)
	testutil.AssertEqual(t, limiter.Available(), 7)
	testutil.AssertEqual(t, limiter.InUse(), 3)

	// Acquire more
	testutil.AssertEqual(t, limiter.AcquireN(5), true)
	testutil.AssertEqual(t, limiter.Available(), 2)
	testutil.AssertEqual(t, limiter.InUse(), 8)

	// Should fail to acquire more than available
	testutil.AssertEqual(t, limiter.AcquireN(3), false)
	testutil.AssertEqual(t, limiter.Available(), 2)
	testutil.AssertEqual(t, limiter.InUse(), 8)

	// Can acquire exactly what's available
	testutil.AssertEqual(t, limiter.AcquireN(2), true)
	testutil.AssertEqual(t, limiter.Available(), 0)
	testutil.AssertEqual(t, limiter.InUse(), 10)

	// Release multiple permits
	limiter.ReleaseN(4)
	testutil.AssertEqual(t, limiter.Available(), 4)
	testutil.AssertEqual(t, limiter.InUse(), 6)

	// AcquireN(0) should always succeed
	testutil.AssertEqual(t, limiter.AcquireN(0), true)
	testutil.AssertEqual(t, limiter.Available(), 4)
}

func TestWaitWithContext(t *testing.T) {
	limiter, err := NewSafe(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()

	// First wait should succeed immediately
	err = limiter.Wait(ctx)
	testutil.AssertEqual(t, err, nil)
	testutil.AssertEqual(t, limiter.Available(), 0)

	// Second wait with canceled context should fail
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()

	err = limiter.Wait(canceledCtx)
	testutil.AssertNotEqual(t, err, nil)
	testutil.AssertEqual(t, err, context.Canceled)

	// Wait with timeout
	timeoutCtx, cancelTimeout := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancelTimeout()

	err = limiter.Wait(timeoutCtx)
	testutil.AssertNotEqual(t, err, nil)
	testutil.AssertEqual(t, err, context.DeadlineExceeded)
}

func TestWaitWithRelease(t *testing.T) {
	limiter, err := NewSafe(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Fill the limiter
	limiter.Acquire()
	testutil.AssertEqual(t, limiter.Available(), 0)

	// Start a goroutine that will wait
	done := make(chan error, 1)
	go func() {
		err := limiter.Wait(context.Background())
		done <- err
	}()

	// Give the goroutine time to start waiting
	time.Sleep(10 * time.Millisecond)

	// Release should wake up the waiting goroutine
	limiter.Release()

	// Verify the wait succeeded
	select {
	case err := <-done:
		testutil.AssertEqual(t, err, nil)
		testutil.AssertEqual(t, limiter.Available(), 0) // Should be acquired by waiting goroutine
	case <-time.After(100 * time.Millisecond):
		t.Error("waiting goroutine should have been woken up")
	}
}

func TestWaitN(t *testing.T) {
	limiter, err := NewSafe(5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()

	// WaitN should work immediately when permits available
	err = limiter.WaitN(ctx, 3)
	testutil.AssertEqual(t, err, nil)
	testutil.AssertEqual(t, limiter.Available(), 2)

	// WaitN(0) should always succeed
	err = limiter.WaitN(ctx, 0)
	testutil.AssertEqual(t, err, nil)
	testutil.AssertEqual(t, limiter.Available(), 2)

	// WaitN with more than available should block
	done := make(chan error, 1)
	go func() {
		err := limiter.WaitN(ctx, 4) // Need 4, only 2 available
		done <- err
	}()

	// Give goroutine time to start waiting
	time.Sleep(10 * time.Millisecond)

	// Should still be waiting
	select {
	case <-done:
		t.Error("WaitN should still be waiting")
	default:
	}

	// Release enough permits
	limiter.ReleaseN(3)

	// Now it should succeed
	select {
	case err := <-done:
		testutil.AssertEqual(t, err, nil)
		testutil.AssertEqual(t, limiter.Available(), 1) // 2 + 3 - 4 = 1
	case <-time.After(100 * time.Millisecond):
		t.Error("WaitN should have succeeded")
	}
}

func TestSetCapacity(t *testing.T) {
	limiter, err := NewSafe(5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Acquire some permits
	limiter.AcquireN(3)
	testutil.AssertEqual(t, limiter.Available(), 2)
	testutil.AssertEqual(t, limiter.InUse(), 3)

	// Increase capacity
	limiter.SetCapacity(8)
	testutil.AssertEqual(t, limiter.Capacity(), 8)
	testutil.AssertEqual(t, limiter.Available(), 5) // 2 + 3 new permits
	testutil.AssertEqual(t, limiter.InUse(), 3)

	// Decrease capacity
	limiter.SetCapacity(6)
	testutil.AssertEqual(t, limiter.Capacity(), 6)
	testutil.AssertEqual(t, limiter.Available(), 3) // 5 - 2 reduction
	testutil.AssertEqual(t, limiter.InUse(), 3)

	// Decrease capacity below current usage
	limiter.AcquireN(3)                         // Now at capacity (6)
	testutil.AssertEqual(t, limiter.InUse(), 6) // 3 + 3 = 6 in use
	limiter.SetCapacity(4)                      // Reduce below in-use (6)
	testutil.AssertEqual(t, limiter.Capacity(), 4)
	testutil.AssertEqual(t, limiter.Available(), 0) // Can't go negative
	testutil.AssertEqual(t, limiter.InUse(), 6)     // Still 6 in use

	// Test panic on invalid capacity
	defer func() {
		if r := recover(); r == nil {
			t.Error("SetCapacity(0) should panic")
		}
	}()
	limiter.SetCapacity(0)
}

func TestReleaseMoreThanCapacity(t *testing.T) {
	limiter, err := NewSafe(2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("releasing more permits than capacity should panic")
		}
	}()

	limiter.ReleaseN(3) // More than capacity of 2
}

func TestConcurrentAccess(t *testing.T) {
	limiter, err := NewSafe(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	const numGoroutines = 20
	const operationsPerGoroutine = 100

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				err := limiter.Wait(ctx)
				cancel()
				if err != nil {
					errors <- err
					return
				}

				// Simulate some work
				time.Sleep(time.Microsecond)

				limiter.Release()
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		if err != context.DeadlineExceeded {
			t.Errorf("unexpected error: %v", err)
		}
	}

	// Final state should be back to initial
	testutil.AssertEqual(t, limiter.Available(), 10)
	testutil.AssertEqual(t, limiter.InUse(), 0)
}

func TestMultipleWaiters(t *testing.T) {
	limiter, err := NewSafe(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Fill the limiter
	limiter.Acquire()

	const numWaiters = 5
	results := make(chan error, numWaiters)

	// Start multiple waiters
	for i := 0; i < numWaiters; i++ {
		go func() {
			err := limiter.Wait(context.Background())
			results <- err
			if err == nil {
				// If successful, immediately release
				limiter.Release()
			}
		}()
	}

	// Give waiters time to queue up
	time.Sleep(10 * time.Millisecond)

	// Release the initial permit
	limiter.Release()

	// All waiters should eventually succeed (one at a time)
	for i := 0; i < numWaiters; i++ {
		select {
		case err := <-results:
			testutil.AssertEqual(t, err, nil)
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("waiter %d timed out", i)
		}
	}
}

func TestContextCancellation(t *testing.T) {
	limiter, err := NewSafe(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Fill the limiter
	limiter.Acquire()

	ctx, cancel := context.WithCancel(context.Background())

	// Start waiting
	done := make(chan error, 1)
	go func() {
		err := limiter.Wait(ctx)
		done <- err
	}()

	// Give waiter time to start
	time.Sleep(10 * time.Millisecond)

	// Cancel context
	cancel()

	// Should get cancellation error
	select {
	case err := <-done:
		testutil.AssertEqual(t, err, context.Canceled)
	case <-time.After(100 * time.Millisecond):
		t.Error("wait should have been canceled")
	}

	// Limiter should still be at capacity (no permits consumed by canceled waiter)
	testutil.AssertEqual(t, limiter.Available(), 0)
	testutil.AssertEqual(t, limiter.InUse(), 1)
}
