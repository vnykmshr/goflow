/*
Package scheduler provides a flexible and efficient task scheduling system for Go applications.

The scheduler enables delayed execution, periodic tasks, and cron-like scheduling patterns
while providing comprehensive monitoring, error handling, and lifecycle management.

Basic Usage:

	scheduler := scheduler.New()
	defer func() { <-scheduler.Stop() }()

	// Start the scheduler
	scheduler.Start()

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		fmt.Println("Task executed!")
		return nil
	})

	// Schedule a one-time task
	scheduler.Schedule("my-task", task, time.Now().Add(time.Second))

	// Schedule a repeating task (5 times, every 30 seconds)
	scheduler.ScheduleRepeating("daily-report", task, 30*time.Second, 5)

Key Features:

The scheduler provides:
  - One-time delayed task execution with precise timing
  - Repeating tasks with configurable intervals and max runs
  - Immediate execution scheduling with ScheduleEvery
  - Task cancellation and management operations
  - Real-time statistics and monitoring
  - Flexible configuration with lifecycle callbacks
  - Thread-safe concurrent access from multiple goroutines
  - Integration with worker pools for scalable execution
  - Context-aware task execution with timeout support

Scheduling Methods:

Multiple ways to schedule tasks:

	// Schedule for specific time
	scheduler.Schedule("task1", task, time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC))

	// Schedule with delay
	scheduler.ScheduleAfter("task2", task, 5*time.Minute)

	// Schedule repeating with delay
	scheduler.ScheduleRepeating("task3", task, time.Hour, 10) // 10 times, every hour

	// Schedule repeating immediately
	scheduler.ScheduleEvery("task4", task, 30*time.Second, 0) // infinite, every 30s

Task Management:

Comprehensive task management operations:

	// Get task information
	if scheduledTask, exists := scheduler.GetTask("task1"); exists {
		fmt.Printf("Next run: %v\n", scheduledTask.RunAt)
		fmt.Printf("Run count: %d\n", scheduledTask.RunCount)
	}

	// List all scheduled tasks
	tasks := scheduler.ListTasks()
	fmt.Printf("Total tasks: %d\n", len(tasks))

	// Cancel specific task
	canceled := scheduler.Cancel("task1")
	fmt.Printf("Task canceled: %v\n", canceled)

	// Cancel all tasks
	scheduler.CancelAll()

Configuration Options:

Advanced configuration through Config struct:

	config := scheduler.Config{
		WorkerPool:   workerpool.New(8, 200),
		TickInterval: 50 * time.Millisecond,
		TaskTimeout:  30 * time.Second,
		OnTaskScheduled: func(task scheduler.ScheduledTask) {
			log.Printf("Scheduled task: %s", task.ID)
		},
		OnTaskExecuted: func(task scheduler.ScheduledTask, result workerpool.Result) {
			if result.Error != nil {
				log.Printf("Task %s failed: %v", task.ID, result.Error)
			}
		},
		OnTaskCanceled: func(task scheduler.ScheduledTask) {
			log.Printf("Canceled task: %s", task.ID)
		},
		OnError: func(err error) {
			log.Printf("Scheduler error: %v", err)
		},
	}
	scheduler := scheduler.NewWithConfig(config)

Monitoring and Statistics:

Real-time monitoring capabilities:

	stats := scheduler.Stats()
	fmt.Printf("Total scheduled: %d\n", stats.TotalScheduled)
	fmt.Printf("Total executed: %d\n", stats.TotalExecuted)
	fmt.Printf("Total canceled: %d\n", stats.TotalCanceled)
	fmt.Printf("Currently scheduled: %d\n", stats.CurrentScheduled)
	fmt.Printf("Uptime: %v\n", stats.UpTime)

Use Cases:

Cron-like Job Scheduling:

	scheduler := scheduler.New()
	defer func() { <-scheduler.Stop() }()
	scheduler.Start()

	// Daily backup at 2 AM
	backupTask := workerpool.TaskFunc(func(ctx context.Context) error {
		return performBackup()
	})

	// Calculate next 2 AM
	now := time.Now()
	next2AM := time.Date(now.Year(), now.Month(), now.Day()+1, 2, 0, 0, 0, now.Location())
	scheduler.ScheduleRepeating("daily-backup", backupTask, 24*time.Hour, 0)

Batch Processing:

	scheduler := scheduler.New()
	defer func() { <-scheduler.Stop() }()
	scheduler.Start()

	processBatch := workerpool.TaskFunc(func(ctx context.Context) error {
		return processPendingRecords()
	})

	// Process batches every 15 minutes
	scheduler.ScheduleEvery("batch-processor", processBatch, 15*time.Minute, 0)

Health Monitoring:

	config := scheduler.Config{
		OnTaskExecuted: func(task scheduler.ScheduledTask, result workerpool.Result) {
			metrics.TaskExecutionTime.Observe(result.Duration.Seconds())
			if result.Error != nil {
				metrics.TaskErrors.Inc()
				alerting.NotifyTaskFailure(task.ID, result.Error)
			}
		},
	}

	scheduler := scheduler.NewWithConfig(config)
	scheduler.Start()

	healthCheck := workerpool.TaskFunc(func(ctx context.Context) error {
		return checkSystemHealth()
	})

	scheduler.ScheduleEvery("health-check", healthCheck, time.Minute, 0)

Delayed Notifications:

	scheduler := scheduler.New()
	defer func() { <-scheduler.Stop() }()
	scheduler.Start()

	// Schedule reminder emails
	for _, user := range users {
		reminderTask := workerpool.TaskFunc(func(ctx context.Context) error {
			return sendReminderEmail(user.Email)
		})

		// Send reminder 1 day before expiration
		scheduler.Schedule(
			fmt.Sprintf("reminder-%s", user.ID),
			reminderTask,
			user.ExpirationDate.Add(-24*time.Hour),
		)
	}

Rate-Limited Processing:

	config := scheduler.Config{
		WorkerPool: workerpool.New(2, 50), // Limit concurrent processing
	}

	scheduler := scheduler.NewWithConfig(config)
	defer func() { <-scheduler.Stop() }()
	scheduler.Start()

	// Process items with rate limiting
	for i, item := range items {
		processTask := workerpool.TaskFunc(func(ctx context.Context) error {
			return processItem(item)
		})

		// Spread processing over time (100ms intervals)
		delay := time.Duration(i) * 100 * time.Millisecond
		scheduler.ScheduleAfter(fmt.Sprintf("item-%d", i), processTask, delay)
	}

Error Handling:

Comprehensive error handling patterns:

	config := scheduler.Config{
		OnTaskExecuted: func(task scheduler.ScheduledTask, result workerpool.Result) {
			if result.Error != nil {
				// Log error
				log.Printf("Task %s failed: %v", task.ID, result.Error)

				// Implement retry logic
				if task.RunCount < 3 {
					retryTask := workerpool.TaskFunc(func(ctx context.Context) error {
						return task.Task.Execute(ctx)
					})

					// Retry with exponential backoff
					delay := time.Duration(task.RunCount) * time.Minute
					scheduler.ScheduleAfter(task.ID+"-retry", retryTask, delay)
				}
			}
		},
		OnError: func(err error) {
			log.Printf("Scheduler error: %v", err)
			metrics.SchedulerErrors.Inc()
		},
	}

Context and Cancellation:

Tasks respect context cancellation and timeouts:

	config := scheduler.Config{
		TaskTimeout: 30 * time.Second, // Global timeout
	}

	scheduler := scheduler.NewWithConfig(config)
	defer func() { <-scheduler.Stop() }()
	scheduler.Start()

	contextAwareTask := workerpool.TaskFunc(func(ctx context.Context) error {
		// Respect context cancellation
		select {
		case <-time.After(time.Hour):
			return nil
		case <-ctx.Done():
			return ctx.Err() // Return timeout or cancellation error
		}
	})

	scheduler.Schedule("long-task", contextAwareTask, time.Now().Add(time.Second))

Performance Characteristics:

  - Task scheduling: O(1) for individual operations
  - Task lookup: O(1) average case with map-based storage
  - Memory usage: Scales linearly with number of scheduled tasks
  - Tick overhead: Minimal, configurable interval (default 100ms)
  - Execution latency: Depends on tick interval + worker pool availability

Thread Safety:

All scheduler operations are safe for concurrent use from multiple goroutines.
Internal synchronization ensures consistent state without external locking.

Lifecycle Management:

Proper startup and shutdown procedures:

	scheduler := scheduler.New()

	// Start scheduler (begins tick loop)
	if err := scheduler.Start(); err != nil {
		log.Fatal("Failed to start scheduler:", err)
	}

	// Graceful shutdown (completes running tasks)
	shutdownComplete := scheduler.Stop()
	<-shutdownComplete // Wait for shutdown to complete

Integration with Worker Pools:

The scheduler integrates seamlessly with the workerpool package:

	// Use custom worker pool
	pool := workerpool.NewWithConfig(workerpool.Config{
		WorkerCount:     10,
		QueueSize:       500,
		TaskTimeout:     time.Minute,
		BufferedResults: true,
	})

	config := scheduler.Config{
		WorkerPool: pool,
	}

	scheduler := scheduler.NewWithConfig(config)

Best Practices:

1. Use meaningful task IDs for easier debugging and monitoring
2. Set appropriate timeouts for tasks that might hang
3. Implement proper error handling and retry logic
4. Monitor scheduler statistics in production
5. Use callbacks for logging and metrics collection
6. Choose appropriate tick intervals (balance between precision and overhead)
7. Size worker pools based on task characteristics and system resources
8. Implement graceful shutdown in application lifecycle
9. Use context-aware tasks for long-running operations
10. Consider memory usage when scheduling many tasks

Comparison with Alternatives:

vs time.Timer/time.Ticker:
  - Higher-level abstraction with task management
  - Support for repeating tasks and cancellation
  - Integrated with worker pools for concurrent execution
  - Built-in monitoring and statistics

vs cron libraries:
  - More flexible scheduling patterns
  - Programmatic task management
  - Better integration with Go concurrency patterns
  - Real-time monitoring capabilities

vs manual goroutine management:
  - Centralized task scheduling and management
  - Built-in error handling and recovery
  - Resource limiting through worker pools
  - Comprehensive observability

Common Pitfalls:

1. Not calling Start() before scheduling tasks for immediate execution
2. Forgetting to call Stop() during application shutdown
3. Using the same task ID for multiple schedules (causes conflicts)
4. Not handling task errors appropriately
5. Scheduling too many tasks without considering memory usage
6. Using very short tick intervals unnecessarily (impacts performance)
7. Not configuring appropriate timeouts for long-running tasks
*/
package scheduler
