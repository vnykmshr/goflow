package leakybucket

import (
	"context"
	"testing"
	"time"

	"github.com/vnykmshr/goflow/internal/testutil"
	"github.com/vnykmshr/goflow/pkg/ratelimit/bucket"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		leakRate bucket.Limit
		capacity int
		panic    bool
	}{
		{"valid parameters", 10, 5, false},
		{"zero rate", 0, 5, false},
		{"infinite rate", bucket.Inf, 5, false},
		{"negative rate", -1, 5, true},
		{"zero capacity", 10, 0, true},
		{"negative capacity", 10, -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.panic {
				defer func() {
					if r := recover(); r == nil {
						t.Error("expected panic")
					}
				}()
			}

			limiter := New(tt.leakRate, tt.capacity)
			if !tt.panic {
				testutil.AssertEqual(t, limiter.LeakRate(), tt.leakRate)
				testutil.AssertEqual(t, limiter.Capacity(), tt.capacity)
				testutil.AssertEqual(t, limiter.Level(), 0.0) // Starts empty
			}
		})
	}
}

func TestBasicFlow(t *testing.T) {
	clock := testutil.NewMockClock(time.Now())
	limiter := NewWithConfig(Config{
		LeakRate:     10, // 10 requests per second = 1 request per 100ms
		Capacity:     5,  // Can hold 5 requests
		Clock:        clock,
		InitialLevel: 0, // Start empty
	})

	// Should allow requests up to capacity
	for i := 0; i < 5; i++ {
		if !limiter.Allow() {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 6th request should be denied (bucket full)
	if limiter.Allow() {
		t.Error("6th request should be denied")
	}

	// After 100ms, 1 request should have leaked (10 requests/sec = 1 request/100ms)
	clock.Advance(100 * time.Millisecond)
	if !limiter.Allow() {
		t.Error("request after 100ms should be allowed")
	}

	// Next request should be denied again (bucket full)
	if limiter.Allow() {
		t.Error("immediate request after leak should be denied")
	}

	// After 500ms, 5 requests should leak, making bucket empty
	clock.Advance(500 * time.Millisecond)
	testutil.AssertEqual(t, limiter.Level(), 0.0)
}

func TestAllowN(t *testing.T) {
	clock := testutil.NewMockClock(time.Now())
	limiter := NewWithConfig(Config{
		LeakRate: 10,
		Capacity: 10,
		Clock:    clock,
	})

	// Should allow consuming multiple requests
	if !limiter.AllowN(3) {
		t.Error("AllowN(3) should succeed with empty bucket")
	}

	testutil.AssertEqual(t, limiter.Level(), 3.0)

	// Should allow consuming remaining capacity
	if !limiter.AllowN(7) {
		t.Error("AllowN(7) should succeed with 7 available")
	}

	// Should deny when at capacity
	if limiter.AllowN(1) {
		t.Error("AllowN(1) should fail when bucket is full")
	}

	// AllowN(0) should always succeed
	if !limiter.AllowN(0) {
		t.Error("AllowN(0) should always succeed")
	}
}

func TestLeakBehavior(t *testing.T) {
	clock := testutil.NewMockClock(time.Now())
	limiter := NewWithConfig(Config{
		LeakRate: 10, // 10 requests/sec
		Capacity: 10,
		Clock:    clock,
	})

	// Fill bucket completely
	for i := 0; i < 10; i++ {
		limiter.Allow()
	}
	testutil.AssertEqual(t, limiter.Level(), 10.0)

	// After 500ms, 5 requests should leak
	clock.Advance(500 * time.Millisecond)
	testutil.AssertEqual(t, limiter.Level(), 5.0)

	// After another 500ms, all should leak
	clock.Advance(500 * time.Millisecond)
	testutil.AssertEqual(t, limiter.Level(), 0.0)

	// Verify available space
	testutil.AssertEqual(t, limiter.Available(), 10.0)
}

func TestWaitWithContext(t *testing.T) {
	// Test context cancellation
	limiter := New(1, 1) // 1 request/sec, capacity 1

	// Fill bucket
	limiter.Allow()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := limiter.Wait(ctx)
	if err == nil {
		t.Error("Wait should return error when context is canceled")
	}
	if err != context.Canceled {
		t.Errorf("Wait should return context.Canceled, got %v", err)
	}

	// Test with timeout - use a limiter that will definitely timeout
	limiter2 := New(0.1, 1) // Very slow rate: 0.1 requests/sec = 10 seconds per request
	limiter2.Allow()        // Fill bucket

	ctx2, cancel2 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel2()

	err = limiter2.Wait(ctx2)
	if err == nil {
		t.Error("Wait should return error when context times out")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("Wait should return context.DeadlineExceeded, got %v", err)
	}
}

func TestReserve(t *testing.T) {
	clock := testutil.NewMockClock(time.Now())
	limiter := NewWithConfig(Config{
		LeakRate: 10, // 10 requests/sec = 100ms per request
		Capacity: 2,
		Clock:    clock,
	})

	// First reservation should be immediate (bucket empty)
	r1 := limiter.Reserve()
	if !r1.OK() {
		t.Error("first reservation should be OK")
	}
	if r1.Delay() != 0 {
		t.Errorf("first reservation delay = %v, want 0", r1.Delay())
	}

	// Second reservation should be immediate (still space)
	r2 := limiter.Reserve()
	if !r2.OK() {
		t.Error("second reservation should be OK")
	}
	if r2.Delay() != 0 {
		t.Errorf("second reservation delay = %v, want 0", r2.Delay())
	}

	// Third reservation should require waiting
	r3 := limiter.Reserve()
	if !r3.OK() {
		t.Error("third reservation should be OK")
	}

	expectedDelay := 100 * time.Millisecond // Need to wait for 1 request to leak at 10 requests/sec
	actualDelay := r3.DelayFrom(clock.Now())

	// Allow for some timing precision issues
	if actualDelay < 90*time.Millisecond || actualDelay > 110*time.Millisecond {
		t.Errorf("third reservation delay = %v, want ~%v", actualDelay, expectedDelay)
	}
}

func TestReservationCancel(t *testing.T) {
	clock := testutil.NewMockClock(time.Now())
	limiter := NewWithConfig(Config{
		LeakRate: 10,
		Capacity: 2,
		Clock:    clock,
	})

	initialLevel := limiter.Level()
	testutil.AssertEqual(t, initialLevel, 0.0)

	// Make a reservation
	r := limiter.ReserveN(2)
	if !r.OK() {
		t.Fatal("reservation should be OK")
	}

	// Level should increase
	levelAfterReservation := limiter.Level()
	testutil.AssertEqual(t, levelAfterReservation, 2.0)

	// Cancel the reservation
	r.Cancel()

	// After cancellation, level should be restored
	levelAfterCancel := limiter.Level()
	testutil.AssertEqual(t, levelAfterCancel, 0.0)

	// Test that we can make the reservation again
	if !limiter.AllowN(1) {
		t.Error("should be able to allow again after cancellation")
	}
}

func TestSetLeakRate(t *testing.T) {
	limiter := New(10, 5)

	limiter.SetLeakRate(20)
	testutil.AssertEqual(t, limiter.LeakRate(), bucket.Limit(20))
	testutil.AssertEqual(t, limiter.Capacity(), 5) // Capacity should remain unchanged

	limiter.SetLeakRate(bucket.Inf)
	testutil.AssertEqual(t, limiter.LeakRate(), bucket.Inf)
}

func TestSetCapacity(t *testing.T) {
	limiter := New(10, 5)

	limiter.SetCapacity(10)
	testutil.AssertEqual(t, limiter.Capacity(), 10)
	testutil.AssertEqual(t, limiter.LeakRate(), bucket.Limit(10)) // Rate should remain unchanged

	// Test panic on invalid capacity
	defer func() {
		if r := recover(); r == nil {
			t.Error("SetCapacity(0) should panic")
		}
	}()
	limiter.SetCapacity(0)
}

func TestInfiniteRate(t *testing.T) {
	limiter := New(bucket.Inf, 1)

	// Should always allow with infinite rate
	for i := 0; i < 1000; i++ {
		if !limiter.Allow() {
			t.Errorf("request %d should be allowed with infinite rate", i)
		}
	}

	// Level should always be 0 (immediate leak)
	testutil.AssertEqual(t, limiter.Level(), 0.0)
}

func TestZeroRate(t *testing.T) {
	clock := testutil.NewMockClock(time.Now())
	limiter := NewWithConfig(Config{
		LeakRate: 0,
		Capacity: 5,
		Clock:    clock,
	})

	// Should allow up to capacity
	for i := 0; i < 5; i++ {
		if !limiter.Allow() {
			t.Errorf("request %d should be allowed", i)
		}
	}

	// Should deny further requests (no leaking with zero rate)
	if limiter.Allow() {
		t.Error("request should be denied after capacity reached with zero rate")
	}

	// Even after time passes, should still deny (no leaking)
	clock.Advance(time.Hour)
	if limiter.Allow() {
		t.Error("request should still be denied after time passes with zero rate")
	}

	// Level should remain at capacity
	testutil.AssertEqual(t, limiter.Level(), 5.0)
}

func TestInitialLevel(t *testing.T) {
	clock := testutil.NewMockClock(time.Now())

	// Test starting with initial level
	limiter := NewWithConfig(Config{
		LeakRate:     10,
		Capacity:     5,
		InitialLevel: 3,
		Clock:        clock,
	})

	testutil.AssertEqual(t, limiter.Level(), 3.0)
	testutil.AssertEqual(t, limiter.Available(), 2.0)

	// Test initial level exceeding capacity
	limiter2 := NewWithConfig(Config{
		LeakRate:     10,
		Capacity:     3,
		InitialLevel: 5, // Exceeds capacity
		Clock:        clock,
	})

	testutil.AssertEqual(t, limiter2.Level(), 3.0) // Clamped to capacity
}

func TestConcurrentAccess(_ *testing.T) {
	limiter := New(100, 10) // High rate to avoid blocking

	done := make(chan bool)
	const numGoroutines = 10
	const requestsPerGoroutine = 100

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- true }()
			for j := 0; j < requestsPerGoroutine; j++ {
				limiter.Allow() // Just test that it doesn't panic
				limiter.Level()
				limiter.LeakRate()
				limiter.Capacity()
				limiter.Available()
			}
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

func TestSmoothTrafficFlow(t *testing.T) {
	clock := testutil.NewMockClock(time.Now())
	limiter := NewWithConfig(Config{
		LeakRate: 10, // 10 requests/sec
		Capacity: 20,
		Clock:    clock,
	})

	// Fill bucket with burst
	for i := 0; i < 20; i++ {
		if !limiter.Allow() {
			t.Fatalf("should allow burst up to capacity")
		}
	}

	// Now bucket is full, no more requests allowed
	if limiter.Allow() {
		t.Error("should not allow request when bucket is full")
	}

	// After 1 second, 10 requests should leak, allowing 10 more
	clock.Advance(1 * time.Second)
	testutil.AssertEqual(t, limiter.Level(), 10.0)

	// Should be able to add 10 more requests
	for i := 0; i < 10; i++ {
		if !limiter.Allow() {
			t.Errorf("should allow request %d after leak", i+1)
		}
	}

	// Should be full again
	if limiter.Allow() {
		t.Error("should not allow request when bucket is full again")
	}
}
