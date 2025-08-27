package workerpool

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync/atomic"
	"time"
)

// Submit adds a task to the pool for execution.
func (p *workerPool) Submit(task Task) error {
	return p.SubmitWithContext(context.Background(), task)
}

// SubmitWithTimeout submits a task with a timeout for queuing.
func (p *workerPool) SubmitWithTimeout(task Task, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return p.SubmitWithContext(ctx, task)
}

// SubmitWithContext submits a task with a context for cancellation.
func (p *workerPool) SubmitWithContext(ctx context.Context, task Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	p.mu.RLock()
	isShutdown := p.isShutdown
	p.mu.RUnlock()

	if isShutdown {
		return fmt.Errorf("pool is shut down")
	}

	atomic.AddInt64(&p.totalSubmitted, 1)

	select {
	case p.taskQueue <- task:
		return nil
	case <-ctx.Done():
		atomic.AddInt64(&p.totalSubmitted, -1) // Rollback counter
		return ctx.Err()
	case <-p.shutdownCh:
		atomic.AddInt64(&p.totalSubmitted, -1) // Rollback counter
		return fmt.Errorf("pool is shut down")
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

// ShutdownWithTimeout shuts down the pool with a timeout.
func (p *workerPool) ShutdownWithTimeout(timeout time.Duration) <-chan struct{} {
	done := make(chan struct{})
	shutdownDone := p.Shutdown()

	go func() {
		select {
		case <-shutdownDone:
			close(done)
		case <-time.After(timeout):
			// Force close everything
			close(done)
		}
	}()

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

// ActiveWorkers returns the number of workers currently executing tasks.
func (p *workerPool) ActiveWorkers() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.activeWorkers
}

// TotalSubmitted returns the total number of tasks submitted to the pool.
func (p *workerPool) TotalSubmitted() int64 {
	return atomic.LoadInt64(&p.totalSubmitted)
}

// TotalCompleted returns the total number of tasks completed by the pool.
func (p *workerPool) TotalCompleted() int64 {
	return atomic.LoadInt64(&p.totalCompleted)
}

// run is the main loop for a worker.
func (w *worker) run() {
	defer w.pool.workerWg.Done()
	defer close(w.stopped)

	// Call worker start callback
	if w.pool.config.OnWorkerStart != nil {
		w.pool.config.OnWorkerStart(w.id)
	}

	// Call worker stop callback when we exit
	defer func() {
		if w.pool.config.OnWorkerStop != nil {
			w.pool.config.OnWorkerStop(w.id)
		}
	}()

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

// sendResult sends a task result to the result queue with appropriate handling
// for buffered vs unbuffered scenarios and shutdown conditions.
func (w *worker) sendResult(result Result) {
	select {
	case w.pool.resultQueue <- result:
	case <-w.stopCh:
		// Worker is shutting down, don't block on result delivery
	default:
		// If unbuffered and receiver isn't ready, we might need to handle this
		// For now, we'll try to send with a short timeout
		if !w.pool.config.BufferedResults {
			select {
			case w.pool.resultQueue <- result:
			case <-time.After(100 * time.Millisecond):
				// Result delivery timed out, which is acceptable during shutdown
			case <-w.stopCh:
				// Worker shutting down
			}
		}
	}
}

// executeTask executes a single task.
func (w *worker) executeTask(task Task) {
	// Update active worker count
	w.pool.mu.Lock()
	w.pool.activeWorkers++
	w.pool.mu.Unlock()

	defer func() {
		w.pool.mu.Lock()
		w.pool.activeWorkers--
		w.pool.mu.Unlock()
		atomic.AddInt64(&w.pool.totalCompleted, 1)
	}()

	// Call task start callback
	if w.pool.config.OnTaskStart != nil {
		w.pool.config.OnTaskStart(w.id, task)
	}

	start := time.Now()
	var err error

	// Handle panics during task execution
	defer func() {
		if r := recover(); r != nil {
			if w.pool.config.PanicHandler != nil {
				w.pool.config.PanicHandler(task, r)
			} else {
				// Default panic handling - convert to error
				err = fmt.Errorf("task panicked: %v\nStack trace:\n%s", r, debug.Stack())
			}
		}

		duration := time.Since(start)
		result := Result{
			Task:     task,
			Error:    err,
			Duration: duration,
			WorkerID: w.id,
		}

		// Call task complete callback
		if w.pool.config.OnTaskComplete != nil {
			w.pool.config.OnTaskComplete(w.id, result)
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
