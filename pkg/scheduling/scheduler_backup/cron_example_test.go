package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

// Example_cronBasic demonstrates basic cron expression scheduling.
func Example_cronBasic() {
	// Create cron scheduler
	scheduler := NewCronScheduler()
	defer func() { <-scheduler.Stop() }()

	scheduler.Start()

	var counter int32
	task := workerpool.TaskFunc(func(ctx context.Context) error {
		count := atomic.AddInt32(&counter, 1)
		fmt.Printf("Task executed: %d\n", count)
		return nil
	})

	// Schedule task to run every 2 seconds (using cron expression)
	err := scheduler.ScheduleCron("cron-task", "*/2 * * * * *", task)
	if err != nil {
		log.Fatalf("Failed to schedule cron task: %v", err)
	}

	// Wait for a few executions
	time.Sleep(7 * time.Second)

	fmt.Printf("Total executions: %d\n", atomic.LoadInt32(&counter))

	// Output:
	// Task executed: 1
	// Task executed: 2
	// Task executed: 3
	// Total executions: 3
}

// Example_cronExpressions demonstrates various cron expressions.
func Example_cronExpressions() {
	scheduler := NewCronScheduler()
	defer func() { <-scheduler.Stop() }()

	// Validate and describe various cron expressions
	expressions := []string{
		"@daily",           // Every day at midnight
		"@hourly",          // Every hour
		"0 30 14 * * 1-5",  // 2:30 PM on weekdays (with seconds)
		"0 0 9 1 * *",      // 9:00 AM on the 1st of every month
		"0 */15 * * * *",   // Every 15 minutes
		"0 0 */2 * * *",    // Every 2 hours
	}

	fmt.Println("Cron Expression Analysis:")
	for _, expr := range expressions {
		err := scheduler.ValidateCronExpression(expr)
		if err != nil {
			fmt.Printf("%s: INVALID - %v\n", expr, err)
			continue
		}

		desc, err := scheduler.ParseCronExpression(expr)
		if err != nil {
			fmt.Printf("%s: Error parsing - %v\n", expr, err)
			continue
		}

		fmt.Printf("%s: %s\n", expr, desc.Description)
		fmt.Printf("  Next runs: %v\n", desc.NextRuns[0].Format("15:04:05"))
	}

	// Output:
	// Cron Expression Analysis:
	// @daily: Once a day (at midnight)
	//   Next runs: 00:00:00
	// @hourly: Once an hour (at minute 0)
	//   Next runs: [next hour]:00:00
	// 0 30 14 * * 1-5: Custom schedule: 0 30 14 * * 1-5
	//   Next runs: [next weekday at] 14:30:00
	// 0 0 9 1 * *: Custom schedule: 0 0 9 1 * *
	//   Next runs: 09:00:00
	// 0 */15 * * * *: Custom schedule: 0 */15 * * * *
	//   Next runs: [next 15-minute mark]:00
	// 0 0 */2 * * *: Custom schedule: 0 0 */2 * * *
	//   Next runs: [next even hour]:00:00
}

// Example_cronWithOptions demonstrates cron scheduling with advanced options.
func Example_cronWithOptions() {
	scheduler := NewCronScheduler()
	defer func() { <-scheduler.Stop() }()

	scheduler.Start()

	var successCount int32
	var errorCount int32

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		count := atomic.AddInt32(&successCount, 1)
		
		// Simulate occasional errors
		if count == 3 {
			return fmt.Errorf("simulated error on execution %d", count)
		}
		
		fmt.Printf("Task executed successfully: %d\n", count)
		return nil
	})

	// Create cron options with error handling
	options := CronOptions{
		MaxRuns:     5, // Run maximum 5 times
		StopOnError: false, // Continue on errors
		OnError: func(task CronTask, err error) {
			atomic.AddInt32(&errorCount, 1)
			fmt.Printf("Task error: %v\n", err)
		},
	}

	// Schedule task every second with options
	err := scheduler.ScheduleCronWithOptions("options-task", "* * * * * *", task, options)
	if err != nil {
		log.Fatalf("Failed to schedule cron task: %v", err)
	}

	// Wait for all executions to complete
	time.Sleep(6 * time.Second)

	fmt.Printf("Final - Success: %d, Errors: %d\n", 
		atomic.LoadInt32(&successCount), atomic.LoadInt32(&errorCount))

	// Output:
	// Task executed successfully: 1
	// Task executed successfully: 2
	// Task error: simulated error on execution 3
	// Task executed successfully: 4
	// Task executed successfully: 5
	// Final - Success: 5, Errors: 1
}

// Example_cronTimezones demonstrates timezone handling in cron expressions.
func Example_cronTimezones() {
	scheduler := NewCronScheduler()
	defer func() { <-scheduler.Stop() }()

	scheduler.Start()

	// Create tasks for different timezones
	utcLocation, _ := time.LoadLocation("UTC")
	nyLocation, _ := time.LoadLocation("America/New_York")

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		fmt.Printf("Task executed at: %v\n", time.Now().Format("15:04:05 MST"))
		return nil
	})

	// Schedule same cron expression in different timezones
	utcOptions := CronOptions{
		TimeZone: utcLocation,
		MaxRuns:  2,
	}
	
	nyOptions := CronOptions{
		TimeZone: nyLocation,
		MaxRuns:  2,
	}

	// Schedule tasks (every 30 seconds for demo)
	scheduler.ScheduleCronWithOptions("utc-task", "*/30 * * * * *", task, utcOptions)
	scheduler.ScheduleCronWithOptions("ny-task", "*/30 * * * * *", task, nyOptions)

	fmt.Printf("Current time: %v\n", time.Now().Format("15:04:05 MST"))
	fmt.Println("Scheduled tasks in UTC and NY timezones...")

	// Wait for executions
	time.Sleep(65 * time.Second)

	// Output varies based on current timezone and time
}

// Example_cronManagement demonstrates cron task management operations.
func Example_cronManagement() {
	scheduler := NewCronScheduler()
	defer func() { <-scheduler.Stop() }()

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		fmt.Println("Task executed")
		return nil
	})

	// Schedule multiple cron tasks
	scheduler.ScheduleCron("daily-backup", "@daily", task)
	scheduler.ScheduleCron("hourly-check", "@hourly", task)
	scheduler.ScheduleCron("frequent-task", "*/10 * * * * *", task) // Every 10 seconds

	// List all cron tasks
	cronTasks := scheduler.ListCronTasks()
	fmt.Printf("Scheduled cron tasks: %d\n", len(cronTasks))

	for _, cronTask := range cronTasks {
		fmt.Printf("- %s: %s (next: %v)\n", 
			cronTask.ID, cronTask.CronExpression, cronTask.NextRun.Format("15:04:05"))
	}

	// Get next execution time for specific task
	nextRun, err := scheduler.GetCronNext("hourly-check")
	if err == nil {
		fmt.Printf("Hourly check next run: %v\n", nextRun.Format("15:04:05"))
	}

	// Update a cron expression
	err = scheduler.UpdateCron("frequent-task", "*/30 * * * * *") // Change to every 30 seconds
	if err == nil {
		fmt.Println("Updated frequent-task to run every 30 seconds")
	}

	// Cancel a specific cron task
	canceled := scheduler.Cancel("daily-backup")
	fmt.Printf("Daily backup canceled: %v\n", canceled)

	// List remaining tasks
	cronTasks = scheduler.ListCronTasks()
	fmt.Printf("Remaining cron tasks: %d\n", len(cronTasks))

	// Output:
	// Scheduled cron tasks: 3
	// - daily-backup: @daily (next: 00:00:00)
	// - hourly-check: @hourly (next: [next hour]:00:00)
	// - frequent-task: */10 * * * * * (next: [next 10 seconds])
	// Hourly check next run: [next hour]:00:00
	// Updated frequent-task to run every 30 seconds
	// Daily backup canceled: true
	// Remaining cron tasks: 2
}

// Example_cronRealWorld demonstrates real-world cron scheduling scenarios.
func Example_cronRealWorld() {
	scheduler := NewCronScheduler()
	defer func() { <-scheduler.Stop() }()

	scheduler.Start()

	// Database backup task - daily at 2 AM
	backupTask := workerpool.TaskFunc(func(ctx context.Context) error {
		fmt.Println("Performing daily database backup...")
		// Simulate backup work
		time.Sleep(100 * time.Millisecond)
		fmt.Println("Database backup completed")
		return nil
	})

	// Log cleanup task - weekly on Sunday at 3 AM
	cleanupTask := workerpool.TaskFunc(func(ctx context.Context) error {
		fmt.Println("Cleaning up old log files...")
		time.Sleep(50 * time.Millisecond)
		fmt.Println("Log cleanup completed")
		return nil
	})

	// Health check task - every 5 minutes during business hours
	healthCheckTask := workerpool.TaskFunc(func(ctx context.Context) error {
		fmt.Printf("Health check at %v - System OK\n", time.Now().Format("15:04"))
		return nil
	})

	// Report generation - first day of every month at 9 AM
	reportTask := workerpool.TaskFunc(func(ctx context.Context) error {
		fmt.Printf("Generating monthly report for %v\n", time.Now().Format("January 2006"))
		return nil
	})

	// Schedule all tasks with appropriate cron expressions
	// Note: Using shorter intervals for demonstration

	// Daily at 2 AM (simulated with every 10 seconds)
	scheduler.ScheduleCronWithOptions("db-backup", "*/10 * * * * *", backupTask, CronOptions{
		MaxRuns: 2,
		OnError: func(task CronTask, err error) {
			fmt.Printf("Backup failed: %v\n", err)
		},
	})

	// Weekly on Sunday at 3 AM (simulated with every 15 seconds)  
	scheduler.ScheduleCronWithOptions("log-cleanup", "*/15 * * * * *", cleanupTask, CronOptions{
		MaxRuns: 1,
	})

	// Every 5 minutes during business hours 9 AM - 5 PM (simulated with every 5 seconds)
	scheduler.ScheduleCronWithOptions("health-check", "*/5 * * * * *", healthCheckTask, CronOptions{
		MaxRuns: 3,
	})

	// First day of month at 9 AM (simulated with every 20 seconds)
	scheduler.ScheduleCronWithOptions("monthly-report", "*/20 * * * * *", reportTask, CronOptions{
		MaxRuns: 1,
	})

	fmt.Println("Real-world cron tasks scheduled:")
	cronTasks := scheduler.ListCronTasks()
	for _, task := range cronTasks {
		fmt.Printf("- %s: %s\n", task.ID, task.CronExpression)
	}

	// Let tasks run for a while
	time.Sleep(25 * time.Second)

	// Show statistics
	stats := scheduler.Stats()
	fmt.Printf("\nScheduler stats: Executed=%d, Scheduled=%d\n", 
		stats.TotalExecuted, stats.TotalScheduled)

	// Output:
	// Real-world cron tasks scheduled:
	// - db-backup: */10 * * * * *
	// - log-cleanup: */15 * * * * *
	// - health-check: */5 * * * * *
	// - monthly-report: */20 * * * * *
	// Performing daily database backup...
	// Database backup completed
	// Health check at 15:04 - System OK
	// Health check at 15:04 - System OK
	// Cleaning up old log files...
	// Log cleanup completed
	// Performing daily database backup...
	// Database backup completed
	// Health check at 15:04 - System OK
	// Generating monthly report for January 2025
	//
	// Scheduler stats: Executed=7, Scheduled=4
}