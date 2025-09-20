package concurrency

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// mustNewSafe creates a new limiter or panics on error (for benchmarks only)
func mustNewSafe(capacity int) Limiter {
	limiter, err := NewSafe(capacity)
	if err != nil {
		panic(err)
	}
	return limiter
}

// BenchmarkAcquire measures the performance of Acquire calls
func BenchmarkAcquire(b *testing.B) {
	limiter := mustNewSafe(1000) // High capacity to avoid blocking

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if limiter.Acquire() {
				limiter.Release()
			}
		}
	})
}

// BenchmarkAcquireN measures the performance of AcquireN calls
func BenchmarkAcquireN(b *testing.B) {
	limiter := mustNewSafe(1000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if limiter.AcquireN(1) {
				limiter.ReleaseN(1)
			}
		}
	})
}

// BenchmarkWait measures the performance of Wait calls that succeed immediately
func BenchmarkWait(b *testing.B) {
	limiter := mustNewSafe(1000)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if limiter.Wait(ctx) == nil {
				limiter.Release()
			}
		}
	})
}

// BenchmarkRelease measures the performance of Release calls
func BenchmarkRelease(b *testing.B) {
	limiter := mustNewSafe(1000)

	// Pre-acquire permits to release
	for i := 0; i < 1000; i++ {
		limiter.Acquire()
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Release()
			limiter.Acquire() // Re-acquire for next iteration
		}
	})
}

// BenchmarkStateInspection measures the performance of state inspection methods
func BenchmarkStateInspection(b *testing.B) {
	limiter := mustNewSafe(1000)

	// Use some permits
	for i := 0; i < 500; i++ {
		limiter.Acquire()
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			capacity := limiter.Capacity()
			available := limiter.Available()
			inUse := limiter.InUse()

			// Use values to prevent optimization
			if capacity <= 0 || available < 0 || inUse < 0 {
				b.Fatal("unexpected negative values")
			}
		}
	})
}

// BenchmarkSetCapacity measures the performance of SetCapacity calls
func BenchmarkSetCapacity(b *testing.B) {
	limiter := mustNewSafe(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		newCapacity := 100 + (i % 100) // Vary between 100-200
		limiter.SetCapacity(newCapacity)
	}
}

// BenchmarkHighContention simulates high contention scenarios
func BenchmarkHighContention(b *testing.B) {
	limiter := mustNewSafe(10) // Low capacity to create contention

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if limiter.Acquire() {
				// Simulate very brief work
				limiter.Release()
			}
		}
	})
}

// BenchmarkMixedOperations simulates mixed workload
func BenchmarkMixedOperations(b *testing.B) {
	limiter := mustNewSafe(100)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			switch i % 4 {
			case 0:
				if limiter.Acquire() {
					limiter.Release()
				}
			case 1:
				if limiter.AcquireN(2) {
					limiter.ReleaseN(2)
				}
			case 2:
				if limiter.Wait(ctx) == nil {
					limiter.Release()
				}
			case 3:
				limiter.Available()
			}
			i++
		}
	})
}

// BenchmarkWaitWithTimeout measures Wait performance with context timeout
func BenchmarkWaitWithTimeout(b *testing.B) {
	limiter := mustNewSafe(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		if limiter.Wait(ctx) == nil {
			limiter.Release()
		}
		cancel()
	}
}

// BenchmarkWakeupPerformance measures waiter wakeup performance
func BenchmarkWakeupPerformance(b *testing.B) {
	limiter := mustNewSafe(1)

	// Fill the limiter
	limiter.Acquire()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Start a waiter in background
		done := make(chan struct{})
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = limiter.Wait(ctx)
			limiter.Release() // Release immediately
			close(done)
		}()

		// Brief pause to let waiter start
		time.Sleep(time.Microsecond)

		// Release to wake up waiter
		limiter.Release()

		// Wait for operation to complete
		<-done

		// Re-acquire for next iteration
		limiter.Acquire()
	}
}

// BenchmarkMemoryAllocation measures memory allocation patterns
func BenchmarkMemoryAllocation(b *testing.B) {
	b.ReportAllocs()

	limiter := mustNewSafe(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if limiter.Acquire() {
			limiter.Release()
		}
	}
}

// BenchmarkCapacityScaling measures performance at different capacity levels
func BenchmarkCapacityScaling(b *testing.B) {
	capacities := []int{1, 10, 100, 1000, 10000}

	for _, capacity := range capacities {
		b.Run(fmt.Sprintf("Capacity-%d", capacity), func(b *testing.B) {
			limiter := mustNewSafe(capacity)

			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					if limiter.Acquire() {
						limiter.Release()
					}
				}
			})
		})
	}
}

// BenchmarkWaiterQueueManagement measures performance with many waiters
func BenchmarkWaiterQueueManagement(b *testing.B) {
	limiter := mustNewSafe(1)

	// Fill the limiter
	limiter.Acquire()

	const numWaiters = 100

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Start many waiters
		done := make(chan struct{}, numWaiters)
		for j := 0; j < numWaiters; j++ {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()
				if limiter.Wait(ctx) == nil {
					limiter.Release()
				}
				done <- struct{}{}
			}()
		}

		// Release the permit to start the cascade
		limiter.Release()

		// Wait for all to complete
		for j := 0; j < numWaiters; j++ {
			<-done
		}

		// Re-acquire for next iteration
		limiter.Acquire()
	}
}

// BenchmarkContextCancellation measures performance of context cancellation
func BenchmarkContextCancellation(b *testing.B) {
	limiter := mustNewSafe(1)

	// Fill the limiter
	limiter.Acquire()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan error, 1)
		go func() {
			err := limiter.Wait(ctx)
			done <- err
		}()

		// Cancel immediately
		cancel()

		// Wait for cancellation to be processed
		<-done
	}
}
