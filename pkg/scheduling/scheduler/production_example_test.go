package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

func ExampleScheduler_production() {
	// Create a production-ready scheduler with custom config
	config := Config{
		WorkerPool:   workerpool.New(4, 100),
		TickInterval: 100 * time.Millisecond, // Check every 100ms
		MaxTasks:     1000,                   // Limit to 1000 concurrent tasks
	}
	
	scheduler := NewWithConfig(config)
	defer func() { <-scheduler.Stop() }()
	
	scheduler.Start()

	// Example: Database cleanup task
	cleanupTask := workerpool.TaskFunc(func(ctx context.Context) error {
		log.Println("Running database cleanup...")
		// Simulate cleanup work
		select {
		case <-time.After(50 * time.Millisecond):
			log.Println("Database cleanup completed")
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	// Example: Health check task
	healthTask := workerpool.TaskFunc(func(ctx context.Context) error {
		log.Println("Running health check...")
		// Simulate health check
		return nil
	})

	// Example: Report generation task
	reportTask := workerpool.TaskFunc(func(ctx context.Context) error {
		log.Println("Generating daily report...")
		return nil
	})

	// Schedule cleanup to run every hour
	scheduler.ScheduleRepeating("db-cleanup", cleanupTask, time.Hour)

	// Schedule health check every 30 seconds
	scheduler.ScheduleRepeating("health-check", healthTask, 30*time.Second)

	// Schedule daily report at 9 AM using cron
	scheduler.ScheduleCron("daily-report", "0 0 9 * * *", reportTask)

	// Schedule one-time maintenance task
	maintenanceTask := workerpool.TaskFunc(func(ctx context.Context) error {
		log.Println("Running maintenance...")
		return nil
	})
	scheduler.ScheduleAfter("maintenance", maintenanceTask, 5*time.Minute)

	// In production, you'd run this indefinitely
	// select {}
	
	// For this example, just run briefly to demonstrate functionality
	time.Sleep(200 * time.Millisecond)
}

func ExampleBackoffTask_production() {
	// Example of using BackoffTask for resilient operations
	unreliableAPI := workerpool.TaskFunc(func(ctx context.Context) error {
		// Simulate unreliable external API call
		if time.Now().UnixNano()%3 == 0 {
			return nil // Success
		}
		return context.DeadlineExceeded // Simulate timeout
	})

	// Wrap with exponential backoff retry logic
	resilientTask := BackoffTask{
		Task:         unreliableAPI,
		MaxRetries:   3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     time.Second,
	}

	scheduler := New()
	defer func() { <-scheduler.Stop() }()
	scheduler.Start()

	// Schedule the resilient task
	scheduler.ScheduleAfter("api-call", resilientTask, time.Millisecond)

	// Wait for execution
	time.Sleep(100 * time.Millisecond)
}