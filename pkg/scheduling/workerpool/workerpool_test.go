package workerpool

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vnykmshr/goflow/internal/testutil"
)

func TestWorkerPool_BasicExecution(t *testing.T) {
	pool := New(2, 10)
	defer func() { <-pool.Shutdown() }()

	var executed int32
	task := TaskFunc(func(_ context.Context) error {
		atomic.AddInt32(&executed, 1)
		return nil
	})

	if err := pool.Submit(task); err != nil {
		t.Fatal(err)
	}

	// Wait for execution
	time.Sleep(50 * time.Millisecond)

	if count := atomic.LoadInt32(&executed); count != 1 {
		t.Errorf("expected 1 execution, got %d", count)
	}
}

func TestWorkerPool_MultipleTasksExecution(t *testing.T) {
	pool := New(3, 20)
	defer func() { <-pool.Shutdown() }()

	var executed int32
	task := TaskFunc(func(_ context.Context) error {
		atomic.AddInt32(&executed, 1)
		time.Sleep(5 * time.Millisecond) // Simulate work
		return nil
	})

	// Submit multiple tasks
	for i := 0; i < 10; i++ {
		if err := pool.Submit(task); err != nil {
			t.Fatal(err)
		}
	}

	// Wait for all executions with timeout
	for i := 0; i < 50; i++ {
		if atomic.LoadInt32(&executed) >= 10 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	count := atomic.LoadInt32(&executed)
	if count != 10 {
		t.Errorf("expected 10 executions, got %d", count)
	}
}

func TestWorkerPool_TaskWithError(t *testing.T) {
	pool := New(1, 5)
	defer func() { <-pool.Shutdown() }()

	expectedErr := errors.New("test error")
	task := TaskFunc(func(_ context.Context) error {
		return expectedErr
	})

	if err := pool.Submit(task); err != nil {
		t.Fatal(err)
	}

	// Check result
	select {
	case result := <-pool.Results():
		if result.Error == nil {
			t.Error("expected error but got nil")
		}
		if result.Error.Error() != expectedErr.Error() {
			t.Errorf("expected error %v, got %v", expectedErr, result.Error)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for result")
	}
}

func TestWorkerPool_TaskTimeout(t *testing.T) {
	config := Config{
		WorkerCount: 1,
		QueueSize:   5,
		TaskTimeout: 50 * time.Millisecond,
	}
	pool := NewWithConfig(config)
	defer func() { <-pool.Shutdown() }()

	task := TaskFunc(func(ctx context.Context) error {
		select {
		case <-time.After(100 * time.Millisecond): // Longer than timeout
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	if err := pool.Submit(task); err != nil {
		t.Fatal(err)
	}

	// Check result
	select {
	case result := <-pool.Results():
		if result.Error == nil {
			t.Error("expected timeout error but got nil")
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("timeout waiting for result")
	}
}

func TestWorkerPool_Stats(t *testing.T) {
	pool := New(2, 10)
	defer func() { <-pool.Shutdown() }()

	if pool.Size() != 2 {
		t.Errorf("expected pool size 2, got %d", pool.Size())
	}

	if pool.QueueSize() < 0 {
		t.Errorf("expected non-negative queue size, got %d", pool.QueueSize())
	}

	task := TaskFunc(func(_ context.Context) error {
		time.Sleep(10 * time.Millisecond)
		return nil
	})

	_ = pool.Submit(task)

	// Just verify basic functionality works
	time.Sleep(50 * time.Millisecond)
}

func TestWorkerPool_SubmitAfterShutdown(t *testing.T) {
	pool := New(1, 5)
	<-pool.Shutdown()

	task := TaskFunc(func(_ context.Context) error {
		return nil
	})

	err := pool.Submit(task)
	if err == nil {
		t.Error("expected error when submitting to shut down pool")
	}
}

func TestWorkerPool_PanicRecovery(t *testing.T) {
	pool := New(1, 5)
	defer func() { <-pool.Shutdown() }()

	task := TaskFunc(func(_ context.Context) error {
		panic("test panic")
	})

	if err := pool.Submit(task); err != nil {
		t.Fatal(err)
	}

	// Check result
	select {
	case result := <-pool.Results():
		if result.Error == nil {
			t.Error("expected panic error but got nil")
		}
		if !strings.Contains(result.Error.Error(), "panic") {
			t.Errorf("expected panic in error message, got %v", result.Error)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for result")
	}
}

// Context propagation tests (v1.3.0)

func TestSubmitWithContext(t *testing.T) {
	pool := New(1, 5)
	defer func() { <-pool.Shutdown() }()

	// Create a context with a custom value to verify propagation
	type ctxKey string
	key := ctxKey("test-key")
	expectedValue := "test-value"
	ctx := context.WithValue(context.Background(), key, expectedValue)

	var receivedValue string
	task := TaskFunc(func(taskCtx context.Context) error {
		// Verify the context value was propagated
		if v, ok := taskCtx.Value(key).(string); ok {
			receivedValue = v
		}
		return nil
	})

	if err := pool.SubmitWithContext(ctx, task); err != nil {
		t.Fatal(err)
	}

	// Wait for execution
	select {
	case <-pool.Results():
		if receivedValue != expectedValue {
			t.Errorf("expected context value %q, got %q", expectedValue, receivedValue)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for result")
	}
}

func TestContextCancellation(t *testing.T) {
	pool := New(1, 5)
	defer func() { <-pool.Shutdown() }()

	ctx, cancel := context.WithCancel(context.Background())
	var taskStarted atomic.Bool
	var taskCanceled atomic.Bool

	task := TaskFunc(func(taskCtx context.Context) error {
		taskStarted.Store(true)
		// Wait for cancellation or timeout
		select {
		case <-taskCtx.Done():
			taskCanceled.Store(true)
			return taskCtx.Err()
		case <-time.After(500 * time.Millisecond):
			return nil
		}
	})

	if err := pool.SubmitWithContext(ctx, task); err != nil {
		t.Fatal(err)
	}

	// Wait for task to start
	testutil.Eventually(t, func() bool {
		return taskStarted.Load()
	}, 250*time.Millisecond, 5*time.Millisecond)

	// Cancel the context
	cancel()

	// Check result
	select {
	case result := <-pool.Results():
		if !taskCanceled.Load() {
			t.Error("expected task to observe cancellation")
		}
		if result.Error == nil {
			t.Error("expected cancellation error but got nil")
		}
		if !errors.Is(result.Error, context.Canceled) {
			t.Errorf("expected context.Canceled error, got %v", result.Error)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("timeout waiting for result")
	}
}

func TestContextTimeout(t *testing.T) {
	pool := New(1, 5)
	defer func() { <-pool.Shutdown() }()

	// Create a context with a short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	var taskTimedOut atomic.Bool

	task := TaskFunc(func(taskCtx context.Context) error {
		// Wait for timeout or longer duration
		select {
		case <-taskCtx.Done():
			taskTimedOut.Store(true)
			return taskCtx.Err()
		case <-time.After(500 * time.Millisecond):
			return nil
		}
	})

	if err := pool.SubmitWithContext(ctx, task); err != nil {
		t.Fatal(err)
	}

	// Check result
	select {
	case result := <-pool.Results():
		if !taskTimedOut.Load() {
			t.Error("expected task to observe timeout")
		}
		if result.Error == nil {
			t.Error("expected timeout error but got nil")
		}
		if !errors.Is(result.Error, context.DeadlineExceeded) {
			t.Errorf("expected context.DeadlineExceeded error, got %v", result.Error)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("timeout waiting for result")
	}
}

func TestSubmitWithContext_NilTask(t *testing.T) {
	pool := New(1, 5)
	defer func() { <-pool.Shutdown() }()

	err := pool.SubmitWithContext(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nil task")
	}
}

func TestSubmitWithContext_NilContext(t *testing.T) {
	pool := New(1, 5)
	defer func() { <-pool.Shutdown() }()

	var executed atomic.Bool
	task := TaskFunc(func(ctx context.Context) error {
		// Verify we got a valid context (not nil)
		if ctx == nil {
			return errors.New("received nil context")
		}
		executed.Store(true)
		return nil
	})

	// Submit with nil context - should use context.Background()
	//nolint:staticcheck // SA1012: intentionally testing nil context handling
	if err := pool.SubmitWithContext(nil, task); err != nil {
		t.Fatal(err)
	}

	// Wait for execution
	select {
	case result := <-pool.Results():
		if result.Error != nil {
			t.Errorf("expected no error, got %v", result.Error)
		}
		if !executed.Load() {
			t.Error("task was not executed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for result")
	}
}

func TestSubmitWithContext_CanceledBeforeSubmit(t *testing.T) {
	pool := New(1, 5)
	defer func() { <-pool.Shutdown() }()

	// Create an already-canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	task := TaskFunc(func(_ context.Context) error {
		return nil
	})

	// Submit should return an error because context is already canceled
	err := pool.SubmitWithContext(ctx, task)
	if err == nil {
		t.Fatal("expected error when submitting with canceled context")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected context canceled error, got %v", err)
	}
}
