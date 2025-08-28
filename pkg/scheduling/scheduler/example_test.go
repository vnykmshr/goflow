package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

// Example demonstrates basic scheduler usage.
func Example() {
	// Create scheduler
	scheduler := New()
	defer func() { <-scheduler.Stop() }()

	// Start the scheduler
	scheduler.Start()

	var counter int32
	task := workerpool.TaskFunc(func(ctx context.Context) error {
		count := atomic.AddInt32(&counter, 1)
		fmt.Printf("Task executed: %d\n", count)
		return nil
	})

	// Schedule a one-time task
	scheduler.Schedule("one-time", task, time.Now().Add(50*time.Millisecond))

	// Wait for execution
	time.Sleep(200 * time.Millisecond)

	fmt.Printf("Final count: %d\n", atomic.LoadInt32(&counter))

	// Output:
	// Task executed: 1
	// Final count: 1
}

// Example_repeatingTasks demonstrates repeating task scheduling.
func Example_repeatingTasks() {
	scheduler := New()
	defer func() { <-scheduler.Stop() }()

	scheduler.Start()

	var counter int32
	task := workerpool.TaskFunc(func(ctx context.Context) error {
		count := atomic.AddInt32(&counter, 1)
		fmt.Printf("Repeat %d\n", count)
		return nil
	})

	// Schedule task to repeat 3 times every 50ms
	scheduler.ScheduleRepeating("repeater", task, 50*time.Millisecond, 3)

	// Wait for all executions
	time.Sleep(300 * time.Millisecond)

	fmt.Printf("Total: %d\n", atomic.LoadInt32(&counter))

	// Output:
	// Repeat 1
	// Repeat 2
	// Repeat 3
	// Total: 3
}

// Example_taskManagement demonstrates task management operations.
func Example_taskManagement() {
	scheduler := New()
	defer func() { <-scheduler.Stop() }()

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		fmt.Println("Task executed")
		return nil
	})

	// Schedule tasks
	scheduler.Schedule("task1", task, time.Now().Add(time.Hour))
	scheduler.Schedule("task2", task, time.Now().Add(time.Hour))
	scheduler.ScheduleRepeating("task3", task, time.Second, 5)

	// List all tasks
	tasks := scheduler.ListTasks()
	fmt.Printf("Scheduled tasks: %d\n", len(tasks))

	// Get specific task
	if scheduledTask, exists := scheduler.GetTask("task1"); exists {
		fmt.Printf("Task1 is repeating: %v\n", scheduledTask.IsRepeating)
	}

	// Cancel a task
	canceled := scheduler.Cancel("task2")
	fmt.Printf("Task2 canceled: %v\n", canceled)

	// List remaining tasks
	tasks = scheduler.ListTasks()
	fmt.Printf("Remaining tasks: %d\n", len(tasks))

	// Output:
	// Scheduled tasks: 3
	// Task1 is repeating: false
	// Task2 canceled: true
	// Remaining tasks: 2
}

// Example_delayedExecution demonstrates scheduling tasks with delays.
func Example_delayedExecution() {
	scheduler := New()
	defer func() { <-scheduler.Stop() }()

	scheduler.Start()

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		fmt.Println("Delayed task executed")
		return nil
	})

	// Schedule task to run after a delay
	scheduler.ScheduleAfter("delayed", task, 30*time.Millisecond)

	// Wait for execution
	time.Sleep(100 * time.Millisecond)

	// Output:
	// Delayed task executed
}

// Example_periodicTasks demonstrates periodic task scheduling.
func Example_periodicTasks() {
	scheduler := New()
	defer func() { <-scheduler.Stop() }()

	scheduler.Start()

	var counter int32
	task := workerpool.TaskFunc(func(ctx context.Context) error {
		count := atomic.AddInt32(&counter, 1)
		fmt.Printf("Periodic task: %d\n", count)
		return nil
	})

	// Schedule task to run immediately and then every 25ms, max 2 times
	scheduler.ScheduleEvery("periodic", task, 25*time.Millisecond, 2)

	// Wait for executions
	time.Sleep(120 * time.Millisecond)

	fmt.Printf("Executions: %d\n", atomic.LoadInt32(&counter))

	// Output:
	// Periodic task: 1
	// Periodic task: 2
	// Executions: 2
}

// Example_statistics demonstrates scheduler statistics monitoring.
func Example_statistics() {
	scheduler := New()
	defer func() { <-scheduler.Stop() }()

	scheduler.Start()

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		return nil
	})

	// Schedule some tasks
	scheduler.Schedule("task1", task, time.Now().Add(10*time.Millisecond))
	scheduler.Schedule("task2", task, time.Now().Add(20*time.Millisecond))
	scheduler.Schedule("task3", task, time.Now().Add(time.Hour))

	// Get initial stats
	stats := scheduler.Stats()
	fmt.Printf("Initial - Scheduled: %d, Current: %d\n",
		stats.TotalScheduled, stats.CurrentScheduled)

	// Wait for some tasks to execute
	time.Sleep(50 * time.Millisecond)

	// Cancel remaining task
	scheduler.Cancel("task3")

	// Get final stats
	stats = scheduler.Stats()
	fmt.Printf("Final - Executed: %d, Canceled: %d, Current: %d\n",
		stats.TotalExecuted, stats.TotalCanceled, stats.CurrentScheduled)

	// Output:
	// Initial - Scheduled: 3, Current: 3
	// Final - Executed: 2, Canceled: 1, Current: 0
}

// Example_callbacks demonstrates scheduler callbacks.
func Example_callbacks() {
	config := Config{
		OnTaskScheduled: func(task ScheduledTask) {
			fmt.Printf("Scheduled: %s\n", task.ID)
		},
		OnTaskExecuted: func(task ScheduledTask, result workerpool.Result) {
			fmt.Printf("Executed: %s\n", task.ID)
		},
		OnTaskCanceled: func(task ScheduledTask) {
			fmt.Printf("Canceled: %s\n", task.ID)
		},
	}

	scheduler := NewWithConfig(config)
	defer func() { <-scheduler.Stop() }()

	scheduler.Start()

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		return nil
	})

	// Schedule and execute a task
	scheduler.Schedule("callback-task", task, time.Now().Add(10*time.Millisecond))

	// Schedule and cancel a task
	scheduler.Schedule("cancel-task", task, time.Now().Add(time.Hour))
	scheduler.Cancel("cancel-task")

	// Wait for execution
	time.Sleep(30 * time.Millisecond)

	// Output:
	// Scheduled: callback-task
	// Scheduled: cancel-task
	// Canceled: cancel-task
	// Executed: callback-task
}

// Example_cronLikeScheduling demonstrates cron-like scheduling patterns.
func Example_cronLikeScheduling() {
	scheduler := New()
	defer func() { <-scheduler.Stop() }()

	scheduler.Start()

	// Daily report task (simulate with shorter interval for example)
	dailyTask := workerpool.TaskFunc(func(ctx context.Context) error {
		fmt.Println("Daily report generated")
		return nil
	})

	// Hourly cleanup task (simulate with shorter interval for example)
	cleanupTask := workerpool.TaskFunc(func(ctx context.Context) error {
		fmt.Println("Cleanup completed")
		return nil
	})

	// Schedule "daily" task (every 50ms for demo, normally 24 hours)
	scheduler.ScheduleRepeating("daily-report", dailyTask, 50*time.Millisecond, 2)

	// Schedule "hourly" task (every 30ms for demo, normally 1 hour)
	scheduler.ScheduleRepeating("hourly-cleanup", cleanupTask, 30*time.Millisecond, 3)

	// Wait for executions
	time.Sleep(120 * time.Millisecond)

	// Output:
	// Cleanup completed
	// Daily report generated
	// Cleanup completed
	// Daily report generated
	// Cleanup completed
}

// Example_errorHandling demonstrates error handling in scheduled tasks.
func Example_errorHandling() {
	var errorCount int32

	config := Config{
		OnTaskExecuted: func(task ScheduledTask, result workerpool.Result) {
			if result.Error != nil {
				atomic.AddInt32(&errorCount, 1)
				fmt.Printf("Task %s failed: %v\n", task.ID, result.Error)
			} else {
				fmt.Printf("Task %s succeeded\n", task.ID)
			}
		},
		OnError: func(err error) {
			log.Printf("Scheduler error: %v", err)
		},
	}

	scheduler := NewWithConfig(config)
	defer func() { <-scheduler.Stop() }()

	scheduler.Start()

	// Successful task
	successTask := workerpool.TaskFunc(func(ctx context.Context) error {
		return nil
	})

	// Failing task
	failTask := workerpool.TaskFunc(func(ctx context.Context) error {
		return fmt.Errorf("task failed")
	})

	scheduler.Schedule("success", successTask, time.Now().Add(10*time.Millisecond))
	scheduler.Schedule("failure", failTask, time.Now().Add(20*time.Millisecond))

	// Wait for executions
	time.Sleep(50 * time.Millisecond)

	fmt.Printf("Errors: %d\n", atomic.LoadInt32(&errorCount))

	// Output:
	// Task success succeeded
	// Task failure failed: task failed
	// Errors: 1
}
