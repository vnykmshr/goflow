package scheduler

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

// Benchmark basic scheduler performance
func BenchmarkScheduler_Schedule(b *testing.B) {
	scheduler := New()
	scheduler.Start()
	defer func() { <-scheduler.Stop() }()

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		return nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scheduler.Schedule(fmt.Sprintf("task-%d", i), task, time.Now().Add(time.Millisecond))
	}
}

// Benchmark repeating task scheduling
func BenchmarkScheduler_ScheduleRepeating(b *testing.B) {
	scheduler := New()
	scheduler.Start()
	defer func() { <-scheduler.Stop() }()

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		return nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scheduler.ScheduleRepeating(fmt.Sprintf("repeat-%d", i), task, 100*time.Millisecond, 1)
	}
}

// Benchmark cron scheduler performance
func BenchmarkCronScheduler_ScheduleCron(b *testing.B) {
	scheduler := NewCronScheduler()
	scheduler.Start()
	defer func() { <-scheduler.Stop() }()

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		return nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scheduler.ScheduleCron(fmt.Sprintf("cron-%d", i), "@hourly", task)
	}
}

// Benchmark advanced scheduler backoff
func BenchmarkAdvancedScheduler_Backoff(b *testing.B) {
	scheduler := NewAdvancedScheduler()
	scheduler.Start()
	defer func() { <-scheduler.Stop() }()

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		return nil
	})

	config := BackoffConfig{
		InitialDelay: time.Millisecond,
		MaxDelay:     time.Second,
		Multiplier:   2.0,
		MaxRetries:   3,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scheduler.ScheduleWithBackoff(fmt.Sprintf("backoff-%d", i), task, config)
	}
}

// Benchmark task execution throughput
func BenchmarkScheduler_ExecutionThroughput(b *testing.B) {
	scheduler := NewWithConfig(Config{
		WorkerPool: workerpool.New(runtime.NumCPU(), 1000),
	})
	scheduler.Start()
	defer func() { <-scheduler.Stop() }()

	var executed int64
	task := workerpool.TaskFunc(func(ctx context.Context) error {
		atomic.AddInt64(&executed, 1)
		return nil
	})

	b.ResetTimer()
	start := time.Now()
	
	for i := 0; i < b.N; i++ {
		scheduler.Schedule(fmt.Sprintf("exec-%d", i), task, time.Now())
	}

	// Wait for all tasks to complete
	for {
		if atomic.LoadInt64(&executed) >= int64(b.N) {
			break
		}
		time.Sleep(time.Millisecond)
	}

	duration := time.Since(start)
	b.ReportMetric(float64(b.N)/duration.Seconds(), "tasks/sec")
}

// Benchmark memory usage with many scheduled tasks
func BenchmarkScheduler_MemoryUsage(b *testing.B) {
	scheduler := New()
	scheduler.Start()
	defer func() { <-scheduler.Stop() }()

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		time.Sleep(time.Second) // Keep tasks running
		return nil
	})

	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scheduler.Schedule(fmt.Sprintf("mem-%d", i), task, time.Now().Add(time.Hour))
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)
	
	bytesPerTask := float64(m2.Alloc-m1.Alloc) / float64(b.N)
	b.ReportMetric(bytesPerTask, "bytes/task")
}

// Benchmark concurrent scheduling operations
func BenchmarkScheduler_ConcurrentScheduling(b *testing.B) {
	scheduler := New()
	scheduler.Start()
	defer func() { <-scheduler.Stop() }()

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		return nil
	})

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			scheduler.Schedule(fmt.Sprintf("concurrent-%d-%d", b.N, i), task, time.Now().Add(time.Millisecond))
			i++
		}
	})
}

// Benchmark cron expression parsing
func BenchmarkCronScheduler_ParseExpression(b *testing.B) {
	scheduler := NewCronScheduler()

	expressions := []string{
		"@daily",
		"@hourly", 
		"*/5 * * * * *",
		"0 30 14 * * 1-5",
		"0 0 9 1 * *",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		expr := expressions[i%len(expressions)]
		scheduler.ValidateCronExpression(expr)
	}
}

// Benchmark scheduler with different tick intervals
func BenchmarkScheduler_TickIntervals(b *testing.B) {
	intervals := []time.Duration{
		10 * time.Millisecond,
		50 * time.Millisecond,
		100 * time.Millisecond,
		500 * time.Millisecond,
	}

	for _, interval := range intervals {
		b.Run(fmt.Sprintf("tick-%v", interval), func(b *testing.B) {
			scheduler := NewWithConfig(Config{
				TickInterval: interval,
			})
			scheduler.Start()
			defer func() { <-scheduler.Stop() }()

			task := workerpool.TaskFunc(func(ctx context.Context) error {
				return nil
			})

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				scheduler.Schedule(fmt.Sprintf("tick-%d", i), task, time.Now().Add(time.Millisecond))
			}
		})
	}
}

// Benchmark scheduler scalability with varying worker pool sizes
func BenchmarkScheduler_WorkerPoolSizes(b *testing.B) {
	sizes := []int{1, 2, 4, 8, 16, 32}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("workers-%d", size), func(b *testing.B) {
			scheduler := NewWithConfig(Config{
				WorkerPool: workerpool.New(size, 100),
			})
			scheduler.Start()
			defer func() { <-scheduler.Stop() }()

			var completed int64
			task := workerpool.TaskFunc(func(ctx context.Context) error {
				time.Sleep(time.Millisecond) // Simulate work
				atomic.AddInt64(&completed, 1)
				return nil
			})

			b.ResetTimer()
			start := time.Now()
			
			for i := 0; i < b.N; i++ {
				scheduler.Schedule(fmt.Sprintf("work-%d", i), task, time.Now())
			}

			// Wait for completion
			for atomic.LoadInt64(&completed) < int64(b.N) {
				time.Sleep(time.Millisecond)
			}

			duration := time.Since(start)
			b.ReportMetric(float64(b.N)/duration.Seconds(), "tasks/sec")
		})
	}
}

// Test scheduler performance under load
func BenchmarkScheduler_HighLoad(b *testing.B) {
	scheduler := NewWithConfig(Config{
		WorkerPool:   workerpool.New(runtime.NumCPU(), 10000),
		TickInterval: 10 * time.Millisecond,
	})
	scheduler.Start()
	defer func() { <-scheduler.Stop() }()

	var executed int64
	task := workerpool.TaskFunc(func(ctx context.Context) error {
		atomic.AddInt64(&executed, 1)
		return nil
	})

	// Schedule tasks at different intervals
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		switch i % 4 {
		case 0:
			scheduler.Schedule(fmt.Sprintf("immediate-%d", i), task, time.Now())
		case 1:
			scheduler.ScheduleAfter(fmt.Sprintf("delayed-%d", i), task, time.Millisecond)
		case 2:
			scheduler.ScheduleRepeating(fmt.Sprintf("repeat-%d", i), task, 10*time.Millisecond, 3)
		case 3:
			if cronSched, ok := scheduler.(CronScheduler); ok {
				cronSched.ScheduleCron(fmt.Sprintf("cron-%d", i), "*/1 * * * * *", task)
			}
		}
	}
}

// Benchmark advanced scheduler patterns
func BenchmarkAdvancedScheduler_Patterns(b *testing.B) {
	scheduler := NewAdvancedScheduler()
	scheduler.Start()
	defer func() { <-scheduler.Stop() }()

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		return nil
	})

	b.Run("backoff", func(b *testing.B) {
		config := BackoffConfig{
			InitialDelay: time.Microsecond,
			MaxDelay:     time.Millisecond,
			Multiplier:   1.5,
			MaxRetries:   2,
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			scheduler.ScheduleWithBackoff(fmt.Sprintf("backoff-%d", i), task, config)
		}
	})

	b.Run("conditional", func(b *testing.B) {
		condition := func() bool { return true }
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			scheduler.ScheduleConditional(fmt.Sprintf("conditional-%d", i), task, condition, time.Millisecond)
		}
	})

	b.Run("jittered", func(b *testing.B) {
		jitter := JitterConfig{
			Type:   JitterUniform,
			Amount: time.Millisecond,
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			scheduler.ScheduleJittered(fmt.Sprintf("jitter-%d", i), task, time.Millisecond, jitter)
		}
	})
}