/*
Package scheduling provides task scheduling and execution primitives for Go applications.

This package offers components for managing and executing tasks in various patterns:

  - workerpool: Fixed worker pool for concurrent task execution
  - scheduler: Time-based task scheduling and cron-like functionality
  - pipeline: Task composition and processing pipeline patterns

Worker Pool:

The worker pool provides controlled concurrent execution:

	pool := workerpool.New(4, 100) // 4 workers, queue size 100
	defer pool.Shutdown()

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		// Do work
		return nil
	})

	pool.Submit(task)
	result := <-pool.Results()

Task Scheduler:

The scheduler enables time-based task execution:

	scheduler := scheduler.New()
	defer scheduler.Stop()

	// Schedule one-time task
	scheduler.After(time.Minute, task)

	// Schedule recurring task
	scheduler.Every(time.Hour, task)

	// Cron-style scheduling
	scheduler.Cron("0 9 * * MON-FRI", task) // Weekdays at 9 AM

Pipeline:

Pipelines enable task composition and data flow:

	pipeline := pipeline.New().
		Stage("validate", validateTask).
		Stage("process", processTask).
		Stage("save", saveTask)

	result := pipeline.Execute(ctx, inputData)

All scheduling components are thread-safe and integrate with context
for cancellation and timeout handling.
*/
package scheduling
