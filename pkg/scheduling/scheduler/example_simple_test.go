package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

func ExampleScheduler_basic() {
	// Create scheduler
	scheduler := New()
	defer func() { <-scheduler.Stop() }()

	// Start the scheduler
	scheduler.Start()

	// Simple task
	task := workerpool.TaskFunc(func(ctx context.Context) error {
		fmt.Println("Task executed")
		return nil
	})

	// Schedule task to run in 100ms
	scheduler.ScheduleAfter("simple-task", task, 100*time.Millisecond)

	time.Sleep(200 * time.Millisecond)
	// Output: Task executed
}

func ExampleScheduler_repeating() {
	scheduler := New()
	defer func() { <-scheduler.Stop() }()
	scheduler.Start()

	count := 0
	task := workerpool.TaskFunc(func(ctx context.Context) error {
		count++
		fmt.Printf("Execution %d\n", count)
		return nil
	})

	// Run every 75ms
	scheduler.ScheduleRepeating("counter", task, 75*time.Millisecond)

	time.Sleep(300 * time.Millisecond)

	// Output:
	// Execution 1
	// Execution 2
	// Execution 3
}

func ExampleScheduler_cron() {
	scheduler := New()
	defer func() { <-scheduler.Stop() }()
	scheduler.Start()

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		fmt.Println("Daily backup started")
		return nil
	})

	// Run at 2:30 AM every day
	if err := scheduler.ScheduleCron("backup", "0 30 2 * * *", task); err != nil {
		log.Fatal(err)
	}

	// In real usage, this would run continuously
}

func ExampleBackoffTask() {
	// Task that fails a few times then succeeds
	attempts := 0
	unreliableTask := workerpool.TaskFunc(func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return fmt.Errorf("temporary failure (attempt %d)", attempts)
		}
		fmt.Println("Task succeeded!")
		return nil
	})

	// Wrap with retry logic
	resilientTask := BackoffTask{
		Task:         unreliableTask,
		MaxRetries:   5,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     1 * time.Second,
	}

	ctx := context.Background()
	if err := resilientTask.Execute(ctx); err != nil {
		log.Printf("Task failed: %v", err)
	}

	// Output: Task succeeded!
}

func ExampleScheduler_webServerTasks() {
	scheduler := New()
	defer func() { <-scheduler.Stop() }()
	scheduler.Start()

	// Cleanup old sessions every hour
	cleanupTask := workerpool.TaskFunc(func(ctx context.Context) error {
		fmt.Println("Cleaning up expired sessions...")
		// Your cleanup logic here
		return nil
	})

	scheduler.ScheduleCron("cleanup", "@hourly", cleanupTask)

	// Health check every 30 seconds
	healthTask := workerpool.TaskFunc(func(ctx context.Context) error {
		fmt.Println("Running health check...")
		// Your health check logic here
		return nil
	})

	scheduler.ScheduleRepeating("health", healthTask, 30*time.Second)

	// Send metrics report every 5 minutes
	metricsTask := workerpool.TaskFunc(func(ctx context.Context) error {
		fmt.Println("Sending metrics report...")
		// Your metrics reporting logic here
		return nil
	})

	scheduler.ScheduleRepeating("metrics", metricsTask, 5*time.Minute)

	// In a real server, you'd run indefinitely
	// select {} 
}