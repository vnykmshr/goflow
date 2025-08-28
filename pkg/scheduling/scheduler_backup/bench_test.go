package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

// BenchmarkScheduling measures the overhead of task scheduling.
func BenchmarkScheduling(b *testing.B) {
	scheduler := New()
	defer func() { <-scheduler.Stop() }()

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		return nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		taskID := "task-" + string(rune(i))
		scheduler.Schedule(taskID, task, time.Now().Add(time.Hour))
	}
}

// BenchmarkSchedulingAndExecution measures end-to-end performance.
func BenchmarkSchedulingAndExecution(b *testing.B) {
	scheduler := New()
	defer func() { <-scheduler.Stop() }()

	scheduler.Start()

	var completed int32
	task := workerpool.TaskFunc(func(ctx context.Context) error {
		atomic.AddInt32(&completed, 1)
		return nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		taskID := "task-" + string(rune(i))
		scheduler.Schedule(taskID, task, time.Now().Add(time.Millisecond))
	}

	// Wait for all tasks to complete
	for atomic.LoadInt32(&completed) < int32(b.N) {
		time.Sleep(time.Microsecond)
	}
}

// BenchmarkRepeatingTasks measures repeating task performance.
func BenchmarkRepeatingTasks(b *testing.B) {
	scheduler := New()
	defer func() { <-scheduler.Stop() }()

	scheduler.Start()

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		return nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		taskID := "repeat-" + string(rune(i))
		scheduler.ScheduleRepeating(taskID, task, time.Millisecond, 1)
	}
}

// BenchmarkTaskManagement measures task management operations.
func BenchmarkTaskManagement(b *testing.B) {
	scheduler := New()
	defer func() { <-scheduler.Stop() }()

	// Pre-populate with tasks
	task := workerpool.TaskFunc(func(ctx context.Context) error {
		return nil
	})

	for i := 0; i < 1000; i++ {
		taskID := "setup-" + string(rune(i))
		scheduler.Schedule(taskID, task, time.Now().Add(time.Hour))
	}

	b.ResetTimer()
	b.Run("ListTasks", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			scheduler.ListTasks()
		}
	})

	b.Run("GetTask", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			scheduler.GetTask("setup-500")
		}
	})

	b.Run("Stats", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			scheduler.Stats()
		}
	})
}

// BenchmarkCancellation measures task cancellation performance.
func BenchmarkCancellation(b *testing.B) {
	scheduler := New()
	defer func() { <-scheduler.Stop() }()

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		return nil
	})

	// Pre-schedule tasks to cancel
	taskIDs := make([]string, b.N)
	for i := 0; i < b.N; i++ {
		taskID := "cancel-" + string(rune(i))
		taskIDs[i] = taskID
		scheduler.Schedule(taskID, task, time.Now().Add(time.Hour))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scheduler.Cancel(taskIDs[i])
	}
}

// BenchmarkHighFrequencyScheduling measures performance with many short-lived tasks.
func BenchmarkHighFrequencyScheduling(b *testing.B) {
	scheduler := New()
	defer func() { <-scheduler.Stop() }()

	scheduler.Start()

	var completed int64
	task := workerpool.TaskFunc(func(ctx context.Context) error {
		atomic.AddInt64(&completed, 1)
		return nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		taskID := "hf-" + string(rune(i))
		scheduler.ScheduleAfter(taskID, task, time.Microsecond)
	}

	// Wait for completion
	for atomic.LoadInt64(&completed) < int64(b.N) {
		time.Sleep(time.Microsecond)
	}
}

// BenchmarkConcurrentOperations measures performance under concurrent load.
func BenchmarkConcurrentOperations(b *testing.B) {
	scheduler := New()
	defer func() { <-scheduler.Stop() }()

	scheduler.Start()

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		return nil
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			taskID := "concurrent-" + string(rune(i))
			scheduler.Schedule(taskID, task, time.Now().Add(time.Millisecond))

			// Occasionally cancel tasks
			if i%10 == 0 && i > 0 {
				cancelID := "concurrent-" + string(rune(i-5))
				scheduler.Cancel(cancelID)
			}

			// Occasionally query tasks
			if i%20 == 0 {
				scheduler.ListTasks()
				scheduler.Stats()
			}

			i++
		}
	})
}

// BenchmarkMemoryAllocation measures memory allocation patterns.
func BenchmarkMemoryAllocation(b *testing.B) {
	b.ReportAllocs()

	scheduler := New()
	defer func() { <-scheduler.Stop() }()

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		return nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		taskID := "alloc-" + string(rune(i))
		scheduler.Schedule(taskID, task, time.Now().Add(time.Hour))
	}
}

// BenchmarkTickerOverhead measures the overhead of the scheduler ticker.
func BenchmarkTickerOverhead(b *testing.B) {
	scheduler := New()
	defer func() { <-scheduler.Stop() }()

	// Start with no tasks to measure pure ticker overhead
	scheduler.Start()

	b.ResetTimer()
	// Run for a fixed time to measure ticker overhead
	start := time.Now()
	for time.Since(start) < time.Duration(b.N)*time.Microsecond {
		time.Sleep(time.Microsecond)
	}
}

// BenchmarkSchedulerScaling tests performance across different numbers of tasks.
func BenchmarkSchedulerScaling(b *testing.B) {
	taskCounts := []int{10, 100, 1000, 10000}

	for _, taskCount := range taskCounts {
		b.Run("Tasks-"+string(rune(taskCount)), func(b *testing.B) {
			scheduler := New()
			defer func() { <-scheduler.Stop() }()

			task := workerpool.TaskFunc(func(ctx context.Context) error {
				return nil
			})

			// Pre-populate with tasks
			for i := 0; i < taskCount; i++ {
				taskID := "scale-" + string(rune(i))
				scheduler.Schedule(taskID, task, time.Now().Add(time.Hour))
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				scheduler.ListTasks()
			}
		})
	}
}

// BenchmarkCallbackOverhead measures the overhead of callbacks.
func BenchmarkCallbackOverhead(b *testing.B) {
	var callbackCount int64

	b.Run("WithCallbacks", func(b *testing.B) {
		config := Config{
			OnTaskScheduled: func(task ScheduledTask) {
				atomic.AddInt64(&callbackCount, 1)
			},
			OnTaskExecuted: func(task ScheduledTask, result workerpool.Result) {
				atomic.AddInt64(&callbackCount, 1)
			},
		}

		scheduler := NewWithConfig(config)
		defer func() { <-scheduler.Stop() }()

		scheduler.Start()

		task := workerpool.TaskFunc(func(ctx context.Context) error {
			return nil
		})

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			taskID := "callback-" + string(rune(i))
			scheduler.Schedule(taskID, task, time.Now().Add(time.Millisecond))
		}
	})

	b.Run("WithoutCallbacks", func(b *testing.B) {
		scheduler := New()
		defer func() { <-scheduler.Stop() }()

		scheduler.Start()

		task := workerpool.TaskFunc(func(ctx context.Context) error {
			return nil
		})

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			taskID := "no-callback-" + string(rune(i))
			scheduler.Schedule(taskID, task, time.Now().Add(time.Millisecond))
		}
	})
}

// BenchmarkTaskTypes compares different task types.
func BenchmarkTaskTypes(b *testing.B) {
	scheduler := New()
	defer func() { <-scheduler.Stop() }()

	scheduler.Start()

	simpleTask := workerpool.TaskFunc(func(ctx context.Context) error {
		return nil
	})

	complexTask := workerpool.TaskFunc(func(ctx context.Context) error {
		// Simulate some work
		sum := 0
		for i := 0; i < 1000; i++ {
			sum += i
		}
		return nil
	})

	b.Run("SimpleTask", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			taskID := "simple-" + string(rune(i))
			scheduler.Schedule(taskID, simpleTask, time.Now().Add(time.Millisecond))
		}
	})

	b.Run("ComplexTask", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			taskID := "complex-" + string(rune(i))
			scheduler.Schedule(taskID, complexTask, time.Now().Add(time.Millisecond))
		}
	})
}
