package bucket

import (
	"context"
	"testing"
	"time"
)

// mustNewSafe creates a new limiter or panics on error (for benchmarks only)
func mustNewSafe(rate Limit, burst int) Limiter {
	limiter, err := NewSafe(rate, burst)
	if err != nil {
		panic(err)
	}
	return limiter
}

// BenchmarkAllow measures the performance of Allow calls
func BenchmarkAllow(b *testing.B) {
	limiter := mustNewSafe(1000000, 1000) // High rate to avoid blocking

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow()
		}
	})
}

// BenchmarkAllowN measures the performance of AllowN calls
func BenchmarkAllowN(b *testing.B) {
	limiter := mustNewSafe(1000000, 1000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.AllowN(1)
		}
	})
}

// BenchmarkReserve measures the performance of Reserve calls
func BenchmarkReserve(b *testing.B) {
	limiter := mustNewSafe(1000000, 1000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			r := limiter.Reserve()
			if r.OK() && r.Delay() == 0 {
				// Success case, no delay needed
			}
		}
	})
}

// BenchmarkWait measures the performance of Wait calls that succeed immediately
func BenchmarkWait(b *testing.B) {
	limiter := mustNewSafe(1000000, 1000)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Wait(ctx)
		}
	})
}

// BenchmarkTokens measures the performance of Tokens calls
func BenchmarkTokens(b *testing.B) {
	limiter := mustNewSafe(1000000, 1000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Tokens()
		}
	})
}

// BenchmarkSetLimit measures the performance of SetLimit calls
func BenchmarkSetLimit(b *testing.B) {
	limiter := mustNewSafe(100, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.SetLimit(Limit(100 + i%100))
	}
}

// BenchmarkConcurrentMixed simulates mixed workload with different operations
func BenchmarkConcurrentMixed(b *testing.B) {
	limiter := mustNewSafe(100000, 1000)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			switch i % 4 {
			case 0:
				limiter.Allow()
			case 1:
				limiter.AllowN(2)
			case 2:
				limiter.Wait(ctx)
			case 3:
				r := limiter.Reserve()
				r.Cancel() // Cancel immediately to avoid affecting other operations
			}
			i++
		}
	})
}

// BenchmarkHighContention simulates high contention scenarios
func BenchmarkHighContention(b *testing.B) {
	// Lower rate/burst to create more contention
	limiter := mustNewSafe(100, 10)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow()
		}
	})
}

// BenchmarkZeroRate benchmarks a limiter with zero refill rate
func BenchmarkZeroRate(b *testing.B) {
	limiter := mustNewSafe(0, 1000) // No refill, just initial burst

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Allow()
	}
}

// BenchmarkInfiniteRate benchmarks a limiter with infinite rate
func BenchmarkInfiniteRate(b *testing.B) {
	limiter := mustNewSafe(Inf, 1000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow()
		}
	})
}

// BenchmarkTimeUpdate measures the cost of time-based token updates
func BenchmarkTimeUpdate(b *testing.B) {
	clock := &MockClock{now: time.Now()}
	limiter := NewWithConfig(Config{
		Rate:          100,
		Burst:         100,
		Clock:         clock,
		InitialTokens: 0,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Advance time to trigger token updates
		clock.Advance(10 * time.Millisecond)
		limiter.Allow()
	}
}

// BenchmarkMemoryAllocation measures memory allocation patterns
func BenchmarkMemoryAllocation(b *testing.B) {
	b.ReportAllocs()

	limiter := mustNewSafe(1000, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if limiter.Allow() {
			// Token consumed
		}
	}
}

// BenchmarkReservationLifecycle measures the full reservation lifecycle
func BenchmarkReservationLifecycle(b *testing.B) {
	limiter := mustNewSafe(1000, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := limiter.Reserve()
		if r.OK() {
			if r.Delay() > 0 {
				// Would need to wait
			}
			r.Cancel() // Cancel to restore tokens
		}
	}
}
