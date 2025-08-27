package workerpool

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// BenchmarkTaskExecution measures the overhead of task submission and execution
func BenchmarkTaskExecution(b *testing.B) {
	pool := New(4, 1000)
	defer pool.Shutdown()

	// Consume results in background
	go func() {
		for range pool.Results() {
			// Consume results
		}
	}()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			task := TaskFunc(func(ctx context.Context) error {
				// Minimal work
				return nil
			})
			pool.Submit(task)
		}
	})
}

// BenchmarkTaskExecutionWithWork measures performance with actual work
func BenchmarkTaskExecutionWithWork(b *testing.B) {
	pool := New(4, 1000)
	defer pool.Shutdown()

	// Consume results in background
	go func() {
		for range pool.Results() {
			// Consume results
		}
	}()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			task := TaskFunc(func(ctx context.Context) error {
				// Simulate some CPU work
				sum := 0
				for i := 0; i < 1000; i++ {
					sum += i
				}
				return nil
			})
			pool.Submit(task)
		}
	})
}

// BenchmarkHighThroughput measures maximum throughput
func BenchmarkHighThroughput(b *testing.B) {
	pool := New(8, 10000)
	defer pool.Shutdown()

	var completed int64

	// Consume results in background
	go func() {
		for range pool.Results() {
			atomic.AddInt64(&completed, 1)
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		task := TaskFunc(func(ctx context.Context) error {
			return nil
		})
		pool.Submit(task)
	}

	// Wait for all tasks to complete
	for atomic.LoadInt64(&completed) < int64(b.N) {
		time.Sleep(time.Microsecond)
	}
}

// BenchmarkWorkerPoolScaling tests performance across different worker counts
func BenchmarkWorkerPoolScaling(b *testing.B) {
	workerCounts := []int{1, 2, 4, 8, 16}

	for _, workerCount := range workerCounts {
		b.Run(fmt.Sprintf("Workers-%d", workerCount), func(b *testing.B) {
			pool := New(workerCount, 1000)
			defer pool.Shutdown()

			// Consume results
			go func() {
				for range pool.Results() {
				}
			}()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				task := TaskFunc(func(ctx context.Context) error {
					return nil
				})
				pool.Submit(task)
			}
		})
	}
}

// BenchmarkQueueSizeImpact tests performance with different queue sizes
func BenchmarkQueueSizeImpact(b *testing.B) {
	queueSizes := []int{10, 100, 1000, 10000}

	for _, queueSize := range queueSizes {
		b.Run(fmt.Sprintf("Queue-%d", queueSize), func(b *testing.B) {
			pool := New(4, queueSize)
			defer pool.Shutdown()

			// Consume results
			go func() {
				for range pool.Results() {
				}
			}()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				task := TaskFunc(func(ctx context.Context) error {
					return nil
				})
				pool.Submit(task)
			}
		})
	}
}

// BenchmarkBufferedVsUnbuffered compares buffered vs unbuffered results
func BenchmarkBufferedVsUnbuffered(b *testing.B) {
	b.Run("Unbuffered", func(b *testing.B) {
		pool := NewWithConfig(Config{
			WorkerCount:     4,
			QueueSize:       1000,
			BufferedResults: false,
		})
		defer pool.Shutdown()

		// Consume results
		go func() {
			for range pool.Results() {
			}
		}()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			task := TaskFunc(func(ctx context.Context) error {
				return nil
			})
			pool.Submit(task)
		}
	})

	b.Run("Buffered", func(b *testing.B) {
		pool := NewWithConfig(Config{
			WorkerCount:     4,
			QueueSize:       1000,
			BufferedResults: true,
		})
		defer pool.Shutdown()

		// Consume results
		go func() {
			for range pool.Results() {
			}
		}()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			task := TaskFunc(func(ctx context.Context) error {
				return nil
			})
			pool.Submit(task)
		}
	})
}

// BenchmarkMemoryAllocation measures memory allocation patterns
func BenchmarkMemoryAllocation(b *testing.B) {
	b.ReportAllocs()

	pool := New(4, 1000)
	defer pool.Shutdown()

	// Consume results
	go func() {
		for range pool.Results() {
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		task := TaskFunc(func(ctx context.Context) error {
			return nil
		})
		pool.Submit(task)
	}
}

// BenchmarkContextualTasks measures performance with context-aware tasks
func BenchmarkContextualTasks(b *testing.B) {
	pool := NewWithConfig(Config{
		WorkerCount: 4,
		QueueSize:   1000,
		TaskTimeout: time.Second,
	})
	defer pool.Shutdown()

	// Consume results
	go func() {
		for range pool.Results() {
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		task := TaskFunc(func(ctx context.Context) error {
			select {
			case <-time.After(time.Microsecond):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		pool.Submit(task)
	}
}

// BenchmarkErrorHandling measures performance when tasks return errors
func BenchmarkErrorHandling(b *testing.B) {
	pool := New(4, 1000)
	defer pool.Shutdown()

	// Consume results
	go func() {
		for range pool.Results() {
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		task := TaskFunc(func(ctx context.Context) error {
			if i%2 == 0 {
				return nil
			}
			return context.Canceled
		})
		pool.Submit(task)
	}
}

// BenchmarkPanicRecovery measures performance when tasks panic
func BenchmarkPanicRecovery(b *testing.B) {
	pool := New(4, 1000)
	defer pool.Shutdown()

	// Consume results
	go func() {
		for range pool.Results() {
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		i := i // Capture loop variable
		task := TaskFunc(func(ctx context.Context) error {
			if i%10 == 0 {
				panic("benchmark panic")
			}
			return nil
		})
		pool.Submit(task)
	}
}

// BenchmarkSubmissionMethods compares different submission methods
func BenchmarkSubmissionMethods(b *testing.B) {
	pool := New(4, 10000)
	defer pool.Shutdown()

	// Consume results
	go func() {
		for range pool.Results() {
		}
	}()

	task := TaskFunc(func(ctx context.Context) error {
		return nil
	})

	b.Run("Submit", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			pool.Submit(task)
		}
	})

	b.Run("SubmitWithContext", func(b *testing.B) {
		ctx := context.Background()
		for i := 0; i < b.N; i++ {
			pool.SubmitWithContext(ctx, task)
		}
	})

	b.Run("SubmitWithTimeout", func(b *testing.B) {
		timeout := time.Second
		for i := 0; i < b.N; i++ {
			pool.SubmitWithTimeout(task, timeout)
		}
	})
}

// BenchmarkConcurrentSubmission measures performance under concurrent load
func BenchmarkConcurrentSubmission(b *testing.B) {
	pool := New(8, 10000)
	defer pool.Shutdown()

	// Consume results
	go func() {
		for range pool.Results() {
		}
	}()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		task := TaskFunc(func(ctx context.Context) error {
			return nil
		})

		for pb.Next() {
			pool.Submit(task)
		}
	})
}

// BenchmarkShutdownPerformance measures shutdown time
func BenchmarkShutdownPerformance(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		pool := New(4, 100)

		// Submit some tasks
		for j := 0; j < 50; j++ {
			task := TaskFunc(func(ctx context.Context) error {
				return nil
			})
			pool.Submit(task)
		}

		b.StartTimer()
		<-pool.Shutdown()
	}
}

// BenchmarkStateInspection measures performance of state inspection methods
func BenchmarkStateInspection(b *testing.B) {
	pool := New(4, 100)
	defer pool.Shutdown()

	// Submit some tasks to create state
	for i := 0; i < 10; i++ {
		task := TaskFunc(func(ctx context.Context) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		})
		pool.Submit(task)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Measure various state inspection operations
			pool.Size()
			pool.QueueSize()
			pool.ActiveWorkers()
			pool.TotalSubmitted()
			pool.TotalCompleted()
		}
	})
}

// BenchmarkCallbackOverhead measures the overhead of callbacks
func BenchmarkCallbackOverhead(b *testing.B) {
	var counter int64

	b.Run("WithCallbacks", func(b *testing.B) {
		pool := NewWithConfig(Config{
			WorkerCount: 4,
			QueueSize:   1000,
			OnTaskStart: func(workerID int, task Task) {
				atomic.AddInt64(&counter, 1)
			},
			OnTaskComplete: func(workerID int, result Result) {
				atomic.AddInt64(&counter, 1)
			},
		})
		defer pool.Shutdown()

		// Consume results
		go func() {
			for range pool.Results() {
			}
		}()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			task := TaskFunc(func(ctx context.Context) error {
				return nil
			})
			pool.Submit(task)
		}
	})

	b.Run("WithoutCallbacks", func(b *testing.B) {
		pool := New(4, 1000)
		defer pool.Shutdown()

		// Consume results
		go func() {
			for range pool.Results() {
			}
		}()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			task := TaskFunc(func(ctx context.Context) error {
				return nil
			})
			pool.Submit(task)
		}
	})
}
