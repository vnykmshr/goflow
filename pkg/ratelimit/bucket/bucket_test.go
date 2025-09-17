package bucket

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/vnykmshr/goflow/internal/testutil"
)

// MockClock implements Clock for testing
type MockClock struct {
	now time.Time
}

func (m *MockClock) Now() time.Time {
	return m.now
}

func (m *MockClock) Advance(d time.Duration) {
	m.now = m.now.Add(d)
}

func TestNew(t *testing.T) {
	tests := []struct {
		name  string
		rate  Limit
		burst int
		panic bool
	}{
		{"valid parameters", 10, 5, false},
		{"zero rate", 0, 5, false},
		{"infinite rate", Inf, 5, false},
		{"negative rate", -1, 5, true},
		{"zero burst", 10, 0, true},
		{"negative burst", 10, -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.panic {
				limiter, err := NewSafe(tt.rate, tt.burst)
				if err == nil {
					t.Error("expected error for invalid parameters")
				}
				if limiter != nil {
					t.Error("expected nil limiter on error")
				}
			} else {
				limiter, err := NewSafe(tt.rate, tt.burst)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				testutil.AssertEqual(t, limiter.Limit(), tt.rate)
				testutil.AssertEqual(t, limiter.Burst(), tt.burst)
				testutil.AssertEqual(t, limiter.Tokens(), float64(tt.burst))
			}
		})
	}
}

func TestEvery(t *testing.T) {
	tests := []struct {
		name     string
		interval time.Duration
		want     Limit
	}{
		{"100ms", 100 * time.Millisecond, 10},
		{"1s", time.Second, 1},
		{"2s", 2 * time.Second, 0.5},
		{"zero", 0, Inf},
		{"negative", -time.Second, Inf},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Every(tt.interval)
			if math.IsInf(float64(tt.want), 1) {
				if !math.IsInf(float64(got), 1) {
					t.Errorf("Every(%v) = %v, want Inf", tt.interval, got)
				}
			} else {
				if math.Abs(float64(got-tt.want)) > 1e-10 {
					t.Errorf("Every(%v) = %v, want %v", tt.interval, got, tt.want)
				}
			}
		})
	}
}

func TestAllow(t *testing.T) {
	clock := &MockClock{now: time.Now()}
	limiter, err := NewWithConfigSafe(Config{
		Rate:          10, // 10 tokens per second
		Burst:         5,  // 5 token capacity
		Clock:         clock,
		InitialTokens: 5, // Start full
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should allow 5 requests immediately (full burst)
	for i := 0; i < 5; i++ {
		if !limiter.Allow() {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 6th request should be denied (bucket empty)
	if limiter.Allow() {
		t.Error("6th request should be denied")
	}

	// After 100ms, 1 more token should be available (10 tokens/sec = 1 token/100ms)
	clock.Advance(100 * time.Millisecond)
	if !limiter.Allow() {
		t.Error("request after 100ms should be allowed")
	}

	// Next request should be denied again
	if limiter.Allow() {
		t.Error("immediate request after consuming refilled token should be denied")
	}
}

func TestAllowN(t *testing.T) {
	clock := &MockClock{now: time.Now()}
	limiter, err := NewWithConfigSafe(Config{
		Rate:          10,
		Burst:         10,
		Clock:         clock,
		InitialTokens: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should allow consuming multiple tokens
	if !limiter.AllowN(3) {
		t.Error("AllowN(3) should succeed with 10 tokens available")
	}

	testutil.AssertEqual(t, limiter.Tokens(), 7.0)

	// Should allow consuming remaining tokens
	if !limiter.AllowN(7) {
		t.Error("AllowN(7) should succeed with 7 tokens available")
	}

	// Should deny when no tokens available
	if limiter.AllowN(1) {
		t.Error("AllowN(1) should fail with 0 tokens available")
	}

	// AllowN(0) should always succeed
	if !limiter.AllowN(0) {
		t.Error("AllowN(0) should always succeed")
	}
}

func TestWait(t *testing.T) {
	clock := &MockClock{now: time.Now()}
	limiter, err := NewWithConfigSafe(Config{
		Rate:          10, // 10 tokens/sec = 100ms per token
		Burst:         1,
		Clock:         clock,
		InitialTokens: 1, // Start with one token
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()

	// First wait should succeed immediately (have 1 token)
	err = limiter.Wait(ctx)
	testutil.AssertNoError(t, err)

	// Second wait should need to wait for refill
	// For this test, we'll test the reservation system
	r := limiter.Reserve()
	if !r.OK() {
		t.Fatal("reservation should be OK")
	}

	expectedDelay := 100 * time.Millisecond // 1 token / 10 tokens per second
	actualDelay := r.DelayFrom(clock.Now())

	// Allow for some timing precision
	if actualDelay < 90*time.Millisecond || actualDelay > 110*time.Millisecond {
		t.Errorf("delay = %v, want ~%v", actualDelay, expectedDelay)
	}
}

func TestWaitWithContext(t *testing.T) {
	// Test context cancellation
	limiter, err := NewSafe(1, 1) // 1 token per second, start with 1 token
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = limiter.Wait(ctx)
	if err == nil {
		t.Error("Wait should return error when context is canceled")
	}
	if err != context.Canceled {
		t.Errorf("Wait should return context.Canceled, got %v", err)
	}

	// Test with timeout - use a limiter that will definitely timeout
	limiter2, err := NewSafe(0.1, 1) // Very slow rate: 0.1 tokens per second = 10 seconds per token
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	limiter2.Allow() // Consume the initial token

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
	clock := &MockClock{now: time.Now()}
	limiter, err := NewWithConfigSafe(Config{
		Rate:          10, // 10 tokens/sec = 100ms per token
		Burst:         2,
		Clock:         clock,
		InitialTokens: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// First reservation should be immediate
	r1 := limiter.Reserve()
	if !r1.OK() {
		t.Error("first reservation should be OK")
	}
	if r1.Delay() != 0 {
		t.Errorf("first reservation delay = %v, want 0", r1.Delay())
	}

	// Second reservation should require waiting
	r2 := limiter.Reserve()
	if !r2.OK() {
		t.Error("second reservation should be OK")
	}

	expectedDelay := 100 * time.Millisecond // Need to wait for 1 token at 10 tokens/sec
	actualDelay := r2.DelayFrom(clock.Now())

	// Allow for some timing precision issues
	if actualDelay < 90*time.Millisecond || actualDelay > 110*time.Millisecond {
		t.Errorf("second reservation delay = %v, want ~%v", actualDelay, expectedDelay)
	}
}

func TestReservationCancel(t *testing.T) {
	clock := &MockClock{now: time.Now()}
	limiter, err := NewWithConfigSafe(Config{
		Rate:          10,
		Burst:         2,
		Clock:         clock,
		InitialTokens: 1, // Start with 1 token
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	initialTokens := limiter.Tokens()
	testutil.AssertEqual(t, initialTokens, 1.0)

	// Make a reservation that consumes more than available (will go into debt)
	r := limiter.ReserveN(2)
	if !r.OK() {
		t.Fatal("reservation should be OK")
	}

	// Tokens should now be negative
	tokensAfterReservation := limiter.Tokens()
	testutil.AssertEqual(t, tokensAfterReservation, -1.0)

	// Cancel the reservation
	r.Cancel()

	// After cancellation, tokens should be restored
	tokensAfterCancel := limiter.Tokens()
	testutil.AssertEqual(t, tokensAfterCancel, 1.0) // Back to initial state

	// Test that we can make the reservation again
	r2 := limiter.ReserveN(1)
	if !r2.OK() {
		t.Error("should be able to reserve again after cancellation")
	}
	testutil.AssertEqual(t, limiter.Tokens(), 0.0)
}

func TestSetLimit(t *testing.T) {
	limiter, err := NewSafe(10, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	limiter.SetLimit(20)
	testutil.AssertEqual(t, limiter.Limit(), Limit(20))
	testutil.AssertEqual(t, limiter.Burst(), 5) // Burst should remain unchanged

	limiter.SetLimit(Inf)
	testutil.AssertEqual(t, limiter.Limit(), Inf)
}

func TestSetBurst(t *testing.T) {
	limiter, err := NewSafe(10, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	limiter.SetBurst(10)
	testutil.AssertEqual(t, limiter.Burst(), 10)
	testutil.AssertEqual(t, limiter.Limit(), Limit(10)) // Rate should remain unchanged

	// Test panic on invalid burst
	defer func() {
		if r := recover(); r == nil {
			t.Error("SetBurst(0) should panic")
		}
	}()
	limiter.SetBurst(0)
}

func TestInfiniteRate(t *testing.T) {
	limiter, err := NewSafe(Inf, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should always allow with infinite rate
	for i := 0; i < 1000; i++ {
		if !limiter.Allow() {
			t.Errorf("request %d should be allowed with infinite rate", i)
		}
	}

	// Tokens should always be at burst capacity
	testutil.AssertEqual(t, limiter.Tokens(), 1.0)
}

func TestZeroRate(t *testing.T) {
	clock := &MockClock{now: time.Now()}
	limiter, err := NewWithConfigSafe(Config{
		Rate:          0,
		Burst:         5,
		Clock:         clock,
		InitialTokens: 5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should allow initial burst
	for i := 0; i < 5; i++ {
		if !limiter.Allow() {
			t.Errorf("initial request %d should be allowed", i)
		}
	}

	// Should deny further requests (no refill with zero rate)
	if limiter.Allow() {
		t.Error("request should be denied after burst exhausted with zero rate")
	}

	// Even after time passes, should still deny (zero refill rate)
	clock.Advance(time.Hour)
	if limiter.Allow() {
		t.Error("request should still be denied after time passes with zero rate")
	}
}

func TestConcurrentAccess(t *testing.T) {
	limiter, err := NewSafe(100, 10) // High rate to avoid blocking
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	done := make(chan bool)
	const numGoroutines = 10
	const requestsPerGoroutine = 100

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- true }()
			for j := 0; j < requestsPerGoroutine; j++ {
				limiter.Allow() // Just test that it doesn't panic
				limiter.Tokens()
				limiter.Limit()
				limiter.Burst()
			}
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}
