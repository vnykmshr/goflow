package workerpool

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkerPool_BasicExecution(t *testing.T) {
	pool := New(2, 10)
	defer func() { <-pool.Shutdown() }()

	var executed int32
	task := TaskFunc(func(ctx context.Context) error {
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
	task := TaskFunc(func(ctx context.Context) error {
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
	task := TaskFunc(func(ctx context.Context) error {
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

	task := TaskFunc(func(ctx context.Context) error {
		time.Sleep(10 * time.Millisecond)
		return nil
	})

	pool.Submit(task)
	
	// Just verify basic functionality works
	time.Sleep(50 * time.Millisecond)
}

func TestWorkerPool_SubmitAfterShutdown(t *testing.T) {
	pool := New(1, 5)
	<-pool.Shutdown()

	task := TaskFunc(func(ctx context.Context) error {
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

	task := TaskFunc(func(ctx context.Context) error {
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
		if !contains(result.Error.Error(), "panic") {
			t.Errorf("expected panic in error message, got %v", result.Error)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for result")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && 
			(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
				func() bool {
					for i := 0; i <= len(s)-len(substr); i++ {
						if s[i:i+len(substr)] == substr {
							return true
						}
					}
					return false
				}())))
}