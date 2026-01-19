package benchmark

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

// BenchmarkWorkerPoolSubmit measures task submission performance.
func BenchmarkWorkerPoolSubmit(b *testing.B) {
	workerCounts := []int{2, 4, 8}

	for _, workers := range workerCounts {
		b.Run(workerLabel(workers), func(b *testing.B) {
			pool, err := workerpool.NewSafe(workers, 1000)
			if err != nil {
				b.Fatalf("failed to create pool: %v", err)
			}
			defer func() { <-pool.Shutdown() }()

			// Consume results
			go func() {
				for range pool.Results() {
					_ = struct{}{} // Drain results channel
				}
			}()

			task := workerpool.TaskFunc(func(_ context.Context) error {
				return nil
			})

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = pool.Submit(task)
			}
		})
	}
}

// BenchmarkWorkerPoolSubmitWithContext measures context-aware submission.
func BenchmarkWorkerPoolSubmitWithContext(b *testing.B) {
	pool, err := workerpool.NewSafe(4, 1000)
	if err != nil {
		b.Fatalf("failed to create pool: %v", err)
	}
	defer func() { <-pool.Shutdown() }()

	// Consume results
	go func() {
		for range pool.Results() {
			_ = struct{}{} // Drain results channel
		}
	}()

	task := workerpool.TaskFunc(func(_ context.Context) error {
		return nil
	})

	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pool.SubmitWithContext(ctx, task)
	}
}

// BenchmarkWorkerPoolThroughput measures end-to-end task execution.
func BenchmarkWorkerPoolThroughput(b *testing.B) {
	pool, err := workerpool.NewSafe(4, 100)
	if err != nil {
		b.Fatalf("failed to create pool: %v", err)
	}
	defer func() { <-pool.Shutdown() }()

	var completed int64

	// Result consumer
	go func() {
		for range pool.Results() {
			atomic.AddInt64(&completed, 1)
		}
	}()

	task := workerpool.TaskFunc(func(_ context.Context) error {
		return nil
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pool.Submit(task)
	}

	// Wait for all tasks to complete
	for atomic.LoadInt64(&completed) < int64(b.N) {
		time.Sleep(time.Microsecond)
	}
}

// BenchmarkWorkerPoolContention measures performance under contention.
func BenchmarkWorkerPoolContention(b *testing.B) {
	pool, err := workerpool.NewSafe(8, 500)
	if err != nil {
		b.Fatalf("failed to create pool: %v", err)
	}
	defer func() { <-pool.Shutdown() }()

	// Consume results
	go func() {
		for range pool.Results() {
			_ = struct{}{} // Drain results channel
		}
	}()

	task := workerpool.TaskFunc(func(_ context.Context) error {
		return nil
	})

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = pool.Submit(task)
		}
	})
}

// BenchmarkWorkerPoolWithWork measures performance with actual work.
func BenchmarkWorkerPoolWithWork(b *testing.B) {
	workDurations := []time.Duration{
		0,
		time.Microsecond,
		10 * time.Microsecond,
	}

	for _, workDuration := range workDurations {
		label := "NoWork"
		if workDuration > 0 {
			label = workDuration.String()
		}

		b.Run(label, func(b *testing.B) {
			pool, err := workerpool.NewSafe(4, 100)
			if err != nil {
				b.Fatalf("failed to create pool: %v", err)
			}
			defer func() { <-pool.Shutdown() }()

			var completed int64

			// Result consumer
			go func() {
				for range pool.Results() {
					atomic.AddInt64(&completed, 1)
				}
			}()

			dur := workDuration // capture for closure
			task := workerpool.TaskFunc(func(_ context.Context) error {
				if dur > 0 {
					time.Sleep(dur)
				}
				return nil
			})

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = pool.Submit(task)
			}

			// Wait for completion
			for atomic.LoadInt64(&completed) < int64(b.N) {
				time.Sleep(time.Microsecond)
			}
		})
	}
}

// BenchmarkWorkerPoolScaling measures performance with different pool sizes.
func BenchmarkWorkerPoolScaling(b *testing.B) {
	scales := []struct {
		workers int
		queue   int
	}{
		{1, 100},
		{2, 100},
		{4, 100},
		{8, 100},
		{4, 10},
		{4, 1000},
	}

	for _, scale := range scales {
		b.Run(scaleLabel(scale.workers, scale.queue), func(b *testing.B) {
			pool, err := workerpool.NewSafe(scale.workers, scale.queue)
			if err != nil {
				b.Fatalf("failed to create pool: %v", err)
			}
			defer func() { <-pool.Shutdown() }()

			go func() {
				for range pool.Results() {
					_ = struct{}{} // Drain results channel
				}
			}()

			task := workerpool.TaskFunc(func(_ context.Context) error {
				return nil
			})

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = pool.Submit(task)
			}
		})
	}
}

// BenchmarkWorkerPoolShutdown measures graceful shutdown performance.
func BenchmarkWorkerPoolShutdown(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		pool, err := workerpool.NewSafe(4, 100)
		if err != nil {
			b.Fatalf("failed to create pool: %v", err)
		}

		// Submit some tasks
		task := workerpool.TaskFunc(func(_ context.Context) error {
			return nil
		})
		for j := 0; j < 10; j++ {
			_ = pool.Submit(task)
		}

		// Consume results
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range pool.Results() {
				_ = struct{}{} // Drain results channel
			}
		}()

		<-pool.Shutdown()
		wg.Wait()
	}
}

// workerLabel returns a readable label for worker counts.
func workerLabel(workers int) string {
	return string(rune('0'+workers)) + "workers"
}

// scaleLabel returns a label for scale configuration.
func scaleLabel(workers, queue int) string {
	return workerLabel(workers) + "_q" + sizeLabel(queue)
}
