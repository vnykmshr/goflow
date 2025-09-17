package leakybucket

import (
	"context"
	"testing"
	"time"

	"github.com/vnykmshr/goflow/pkg/ratelimit/bucket"
)

// BenchmarkAllow measures the performance of Allow calls
func BenchmarkAllow(b *testing.B) {
	limiter := New(1000000, 1000) // High rate to avoid blocking

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow()
		}
	})
}

// BenchmarkAllowN measures the performance of AllowN calls
func BenchmarkAllowN(b *testing.B) {
	limiter := New(1000000, 1000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.AllowN(1)
		}
	})
}

// BenchmarkReserve measures the performance of Reserve calls
func BenchmarkReserve(b *testing.B) {
	limiter := New(1000000, 1000)

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
	limiter := New(1000000, 1000)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Wait(ctx)
		}
	})
}

// BenchmarkLevel measures the performance of Level calls
func BenchmarkLevel(b *testing.B) {
	limiter := New(1000000, 1000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Level()
		}
	})
}

// BenchmarkAvailable measures the performance of Available calls
func BenchmarkAvailable(b *testing.B) {
	limiter := New(1000000, 1000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Available()
		}
	})
}

// BenchmarkSetLeakRate measures the performance of SetLeakRate calls
func BenchmarkSetLeakRate(b *testing.B) {
	limiter := New(100, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.SetLeakRate(bucket.Limit(100 + i%100))
	}
}

// BenchmarkConcurrentMixed simulates mixed workload with different operations
func BenchmarkConcurrentMixed(b *testing.B) {
	limiter := New(100000, 1000)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			switch i % 5 {
			case 0:
				limiter.Allow()
			case 1:
				limiter.AllowN(2)
			case 2:
				limiter.Wait(ctx)
			case 3:
				r := limiter.Reserve()
				r.Cancel() // Cancel immediately to avoid affecting other operations
			case 4:
				limiter.Level()
			}
			i++
		}
	})
}

// BenchmarkHighContention simulates high contention scenarios
func BenchmarkHighContention(b *testing.B) {
	// Lower rate/capacity to create more contention
	limiter := New(100, 10)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow()
		}
	})
}

// BenchmarkZeroRate benchmarks a limiter with zero leak rate
func BenchmarkZeroRate(b *testing.B) {
	limiter := New(0, 1000) // No leaking, just initial capacity

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Allow()
	}
}

// BenchmarkInfiniteRate benchmarks a limiter with infinite leak rate
func BenchmarkInfiniteRate(b *testing.B) {
	limiter := New(bucket.Inf, 1000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow()
		}
	})
}

// BenchmarkLeakProcessing measures the cost of leak processing over time
func BenchmarkLeakProcessing(b *testing.B) {
	clock := &MockClock{now: time.Now()}
	limiter := NewWithConfig(Config{
		LeakRate:     100,
		Capacity:     100,
		Clock:        clock,
		InitialLevel: 50,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Advance time to trigger leak processing
		clock.Advance(10 * time.Millisecond)
		limiter.Allow()
	}
}

// BenchmarkMemoryAllocation measures memory allocation patterns
func BenchmarkMemoryAllocation(b *testing.B) {
	b.ReportAllocs()

	limiter := New(1000, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if limiter.Allow() {
			// Request processed
		}
	}
}

// BenchmarkReservationLifecycle measures the full reservation lifecycle
func BenchmarkReservationLifecycle(b *testing.B) {
	limiter := New(1000, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := limiter.Reserve()
		if r.OK() {
			if r.Delay() > 0 {
				// Would need to wait
			}
			r.Cancel() // Cancel to restore level
		}
	}
}

// BenchmarkCapacityChange measures the performance of dynamic capacity changes
func BenchmarkCapacityChange(b *testing.B) {
	limiter := New(100, 50)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		newCapacity := 50 + (i % 50) // Vary between 50-100
		limiter.SetCapacity(newCapacity)
	}
}

// BenchmarkLevelInspection measures the performance of state inspection
func BenchmarkLevelInspection(b *testing.B) {
	limiter := New(1000, 100)

	// Pre-fill some level
	for i := 0; i < 50; i++ {
		limiter.Allow()
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Measure cost of inspecting state
			level := limiter.Level()
			available := limiter.Available()
			capacity := limiter.Capacity()
			rate := limiter.LeakRate()

			// Use values to prevent optimization (allow for floating point precision)
			if level < 0 || available < 0 || capacity <= 0 || rate < 0 {
				b.Fatal("unexpected negative values")
			}
		}
	})
}

// BenchmarkComparison_TokenVsLeaky compares token bucket vs leaky bucket performance
func BenchmarkComparison_TokenVsLeaky(b *testing.B) {
	// Token bucket from existing implementation
	tokenBucket, err := bucket.NewSafe(1000000, 1000)
	if err != nil {
		b.Fatalf("unexpected error: %v", err)
	}

	// Leaky bucket
	leakyBucket, err2 := NewSafe(1000000, 1000)
	if err2 != nil {
		b.Fatalf("unexpected error: %v", err2)
	}

	b.Run("TokenBucket", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				tokenBucket.Allow()
			}
		})
	})

	b.Run("LeakyBucket", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				leakyBucket.Allow()
			}
		})
	})
}
