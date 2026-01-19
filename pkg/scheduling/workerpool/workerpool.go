package workerpool

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"
)

// Submit adds a task to the pool for execution.
// The task will be executed with context.Background().
// Use SubmitWithContext to provide a custom context.
func (p *workerPool) Submit(task Task) error {
	return p.SubmitWithContext(context.Background(), task)
}

// SubmitWithContext adds a task to the pool for execution with the given context.
// The context is passed to the task's Execute method, enabling timeout and
// cancellation propagation. If the pool has a TaskTimeout configured, the
// effective timeout will be the minimum of the context deadline and TaskTimeout.
func (p *workerPool) SubmitWithContext(ctx context.Context, task Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	p.mu.RLock()
	isShutdown := p.isShutdown
	p.mu.RUnlock()

	if isShutdown {
		return fmt.Errorf("cannot submit task: worker pool has been shut down")
	}

	// Check if context is already canceled before attempting to queue
	// This ensures deterministic behavior for pre-canceled contexts
	select {
	case <-ctx.Done():
		return fmt.Errorf("cannot submit task: context canceled: %w", ctx.Err())
	default:
	}

	twc := taskWithContext{
		task: task,
		ctx:  ctx,
	}

	select {
	case p.taskQueue <- twc:
		return nil
	case <-p.shutdownCh:
		return fmt.Errorf("cannot submit task: worker pool has been shut down")
	case <-ctx.Done():
		return fmt.Errorf("cannot submit task: context canceled: %w", ctx.Err())
	}
}

// Results returns a channel of task results.
func (p *workerPool) Results() <-chan Result {
	return p.resultQueue
}

// Shutdown initiates a graceful shutdown of the pool.
func (p *workerPool) Shutdown() <-chan struct{} {
	done := make(chan struct{})

	p.shutdownOnce.Do(func() {
		p.mu.Lock()
		p.isShutdown = true
		p.mu.Unlock()

		// Signal shutdown to all components
		close(p.shutdownCh)

		// Stop all workers gracefully
		for i := range p.workers {
			close(p.workers[i].stopCh)
		}

		// Wait for all workers to finish in a separate goroutine
		go func() {
			p.workerWg.Wait()
			close(p.resultQueue)
			close(done)
		}()
	})

	return done
}

// Size returns the number of workers in the pool.
func (p *workerPool) Size() int {
	return p.config.WorkerCount
}

// QueueSize returns the current number of queued tasks waiting for execution.
func (p *workerPool) QueueSize() int {
	return len(p.taskQueue)
}

// run is the main loop for a worker.
func (w *worker) run() {
	defer w.pool.workerWg.Done()
	defer close(w.stopped)

	for {
		select {
		case <-w.stopCh:
			return
		case twc, ok := <-w.pool.taskQueue:
			if !ok {
				// Task queue closed, shutdown
				return
			}
			w.executeTask(twc)
		}
	}
}

// sendResult sends a task result to the result queue with appropriate handling.
func (w *worker) sendResult(result Result) {
	select {
	case w.pool.resultQueue <- result:
	case <-w.stopCh:
		// Worker is shutting down, don't block on result delivery
	case <-time.After(100 * time.Millisecond):
		// Result delivery timed out, which is acceptable during shutdown
	}
}

// executeTask executes a single task with the provided context.
func (w *worker) executeTask(twc taskWithContext) {
	start := time.Now()
	var err error

	// Handle panics during task execution
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("task panicked: %v\nStack trace:\n%s", r, debug.Stack())
		}

		duration := time.Since(start)
		result := Result{
			Task:     twc.task,
			Error:    err,
			Duration: duration,
			WorkerID: w.id,
		}

		// Send result
		w.sendResult(result)
	}()

	// Start with the caller-provided context
	ctx := twc.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	// Apply TaskTimeout if configured
	// The effective timeout is the minimum of the context deadline and TaskTimeout
	if w.pool.config.TaskTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, w.pool.config.TaskTimeout)
		defer cancel()
	}

	// Execute the task with the propagated context
	err = twc.task.Execute(ctx)
}
