package workerpool

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"
)

// Submit adds a task to the pool for execution.
func (p *workerPool) Submit(task Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	p.mu.RLock()
	isShutdown := p.isShutdown
	p.mu.RUnlock()

	if isShutdown {
		return fmt.Errorf("cannot submit task: worker pool has been shut down")
	}

	select {
	case p.taskQueue <- task:
		return nil
	case <-p.shutdownCh:
		return fmt.Errorf("cannot submit task: worker pool has been shut down")
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
		case task, ok := <-w.pool.taskQueue:
			if !ok {
				// Task queue closed, shutdown
				return
			}
			w.executeTask(task)
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

// executeTask executes a single task.
func (w *worker) executeTask(task Task) {
	start := time.Now()
	var err error

	// Handle panics during task execution
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("task panicked: %v\nStack trace:\n%s", r, debug.Stack())
		}

		duration := time.Since(start)
		result := Result{
			Task:     task,
			Error:    err,
			Duration: duration,
			WorkerID: w.id,
		}

		// Send result
		w.sendResult(result)
	}()

	// Create execution context with timeout if configured
	ctx := context.Background()
	if w.pool.config.TaskTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, w.pool.config.TaskTimeout)
		defer cancel()
	}

	// Execute the task
	err = task.Execute(ctx)
}
