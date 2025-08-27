/*
Package scheduling provides utilities for scheduling and managing goroutines.

This package offers three main scheduling primitives:

  - pool: Worker pool implementation that reuses goroutines
  - scheduler: Goroutine scheduler for time-based task execution
  - pipeline: Data pipeline builder for multi-stage concurrent processing

Basic usage:

	// Create a worker pool with 5 workers
	pool := pool.New(5)
	defer pool.Close()

	// Submit a task
	err := pool.Submit(ctx, task)

All scheduling components support graceful shutdown and integrate with
the context package for cancellation and timeout handling.
*/
package scheduling
