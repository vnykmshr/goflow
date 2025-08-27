package workerpool

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vnykmshr/goflow/internal/testutil"
)

// TestTask is a simple task for testing.
type TestTask struct {
	ID          int
	Duration    time.Duration
	ShouldErr   bool
	ShouldPanic bool
	Executed    *int32 // Atomic counter
}

func (t *TestTask) Execute(ctx context.Context) error {
	atomic.AddInt32(t.Executed, 1)

	if t.ShouldPanic {
		panic("test panic")
	}

	if t.Duration > 0 {
		select {
		case <-time.After(t.Duration):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if t.ShouldErr {
		return errors.New("test error")
	}

	return nil
}

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		workerCount int
		queueSize   int
		expectPanic bool
	}{
		{"valid params", 2, 10, false},
		{"single worker", 1, 5, false},
		{"unbounded queue", 3, 0, false},
		{"synchronous", 2, -1, false},
		{"zero workers", 0, 10, true},
		{"negative workers", -1, 10, true},
		{"invalid queue size", 2, -2, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectPanic {
				defer func() {
					if r := recover(); r == nil {
						t.Error("expected panic")
					}
				}()
			}

			pool := New(tt.workerCount, tt.queueSize)
			if !tt.expectPanic {
				testutil.AssertEqual(t, pool.Size(), tt.workerCount)
				pool.Shutdown()
			}
		})
	}
}

func TestBasicTaskExecution(t *testing.T) {
	pool := New(2, 5)
	defer pool.Shutdown()

	var executed int32
	task := &TestTask{
		ID:       1,
		Duration: 10 * time.Millisecond,
		Executed: &executed,
	}

	err := pool.Submit(task)
	testutil.AssertEqual(t, err, nil)

	// Wait for result
	select {
	case result := <-pool.Results():
		testutil.AssertEqual(t, result.Error, nil)
		testutil.AssertEqual(t, result.Task == task, true)
		testutil.AssertEqual(t, result.WorkerID >= 0, true)
		testutil.AssertEqual(t, result.Duration >= 10*time.Millisecond, true)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for result")
	}

	testutil.AssertEqual(t, atomic.LoadInt32(&executed), int32(1))
}

func TestMultipleTaskExecution(t *testing.T) {
	pool := New(3, 10)
	defer pool.Shutdown()

	const numTasks = 10
	var executed int32
	tasks := make([]*TestTask, numTasks)

	// Submit tasks
	for i := 0; i < numTasks; i++ {
		tasks[i] = &TestTask{
			ID:       i,
			Duration: 5 * time.Millisecond,
			Executed: &executed,
		}
		err := pool.Submit(tasks[i])
		testutil.AssertEqual(t, err, nil)
	}

	// Collect results
	results := make([]Result, 0, numTasks)
	for i := 0; i < numTasks; i++ {
		select {
		case result := <-pool.Results():
			results = append(results, result)
		case <-time.After(time.Second):
			t.Fatalf("timeout waiting for result %d", i)
		}
	}

	testutil.AssertEqual(t, len(results), numTasks)
	testutil.AssertEqual(t, atomic.LoadInt32(&executed), int32(numTasks))
	testutil.AssertEqual(t, pool.TotalSubmitted(), int64(numTasks))
	testutil.AssertEqual(t, pool.TotalCompleted(), int64(numTasks))
}

func TestTaskError(t *testing.T) {
	pool := New(1, 1)
	defer pool.Shutdown()

	var executed int32
	task := &TestTask{
		ID:        1,
		ShouldErr: true,
		Executed:  &executed,
	}

	err := pool.Submit(task)
	testutil.AssertEqual(t, err, nil)

	select {
	case result := <-pool.Results():
		testutil.AssertNotEqual(t, result.Error, nil)
		testutil.AssertEqual(t, result.Error.Error(), "test error")
		testutil.AssertEqual(t, result.Task == task, true)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for result")
	}

	testutil.AssertEqual(t, atomic.LoadInt32(&executed), int32(1))
}

func TestTaskPanic(t *testing.T) {
	var panicHandlerCalled bool
	var recoveredValue interface{}

	config := Config{
		WorkerCount: 1,
		QueueSize:   1,
		PanicHandler: func(task Task, recovered interface{}) {
			panicHandlerCalled = true
			recoveredValue = recovered
		},
	}

	pool := NewWithConfig(config)
	defer pool.Shutdown()

	var executed int32
	task := &TestTask{
		ID:          1,
		ShouldPanic: true,
		Executed:    &executed,
	}

	err := pool.Submit(task)
	testutil.AssertEqual(t, err, nil)

	select {
	case result := <-pool.Results():
		testutil.AssertEqual(t, panicHandlerCalled, true)
		testutil.AssertEqual(t, recoveredValue, "test panic")
		testutil.AssertEqual(t, result.Task == task, true)
		// Error should be nil when custom panic handler is provided
		testutil.AssertEqual(t, result.Error, nil)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for result")
	}

	testutil.AssertEqual(t, atomic.LoadInt32(&executed), int32(1))
}

func TestTaskPanicDefaultHandler(t *testing.T) {
	pool := New(1, 1)
	defer pool.Shutdown()

	var executed int32
	task := &TestTask{
		ID:          1,
		ShouldPanic: true,
		Executed:    &executed,
	}

	err := pool.Submit(task)
	testutil.AssertEqual(t, err, nil)

	select {
	case result := <-pool.Results():
		testutil.AssertNotEqual(t, result.Error, nil)
		testutil.AssertEqual(t, result.Task == task, true)
		// Should contain panic message and stack trace
		errMsg := result.Error.Error()
		testutil.AssertEqual(t, len(errMsg) > 0, true)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for result")
	}
}

func TestSubmitWithTimeout(t *testing.T) {
	// Create pool with small queue to test timeout
	pool := New(1, 1)
	defer pool.Shutdown()

	// Fill the worker with a long-running task
	longTask := &TestTask{
		ID:       1,
		Duration: 100 * time.Millisecond,
		Executed: new(int32),
	}
	pool.Submit(longTask)

	// Fill the queue
	queueTask := &TestTask{
		ID:       2,
		Duration: 10 * time.Millisecond,
		Executed: new(int32),
	}
	pool.Submit(queueTask)

	// Try to submit with short timeout - should fail
	timeoutTask := &TestTask{
		ID:       3,
		Executed: new(int32),
	}

	err := pool.SubmitWithTimeout(timeoutTask, 10*time.Millisecond)
	testutil.AssertNotEqual(t, err, nil)
}

func TestSubmitWithContext(t *testing.T) {
	pool := New(1, 1)
	defer pool.Shutdown()

	// Fill the pool
	pool.Submit(&TestTask{Duration: 100 * time.Millisecond, Executed: new(int32)})
	pool.Submit(&TestTask{Duration: 10 * time.Millisecond, Executed: new(int32)})

	// Try to submit with canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	task := &TestTask{ID: 3, Executed: new(int32)}
	err := pool.SubmitWithContext(ctx, task)
	testutil.AssertNotEqual(t, err, nil)
	testutil.AssertEqual(t, err, context.Canceled)
}

func TestSubmitNilTask(t *testing.T) {
	pool := New(1, 1)
	defer pool.Shutdown()

	err := pool.Submit(nil)
	testutil.AssertNotEqual(t, err, nil)
}

func TestSubmitToShutdownPool(t *testing.T) {
	pool := New(1, 1)

	// Shutdown the pool
	done := pool.Shutdown()
	<-done

	// Try to submit task
	task := &TestTask{ID: 1, Executed: new(int32)}
	err := pool.Submit(task)
	testutil.AssertNotEqual(t, err, nil)
}

func TestTaskTimeout(t *testing.T) {
	config := Config{
		WorkerCount: 1,
		QueueSize:   1,
		TaskTimeout: 50 * time.Millisecond,
	}

	pool := NewWithConfig(config)
	defer pool.Shutdown()

	var executed int32
	task := &TestTask{
		ID:       1,
		Duration: 100 * time.Millisecond, // Longer than timeout
		Executed: &executed,
	}

	err := pool.Submit(task)
	testutil.AssertEqual(t, err, nil)

	select {
	case result := <-pool.Results():
		testutil.AssertNotEqual(t, result.Error, nil)
		testutil.AssertEqual(t, result.Error, context.DeadlineExceeded)
		testutil.AssertEqual(t, atomic.LoadInt32(&executed), int32(1))
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for result")
	}
}

func TestWorkerCallbacks(t *testing.T) {
	var workerStarted, workerStopped int32
	var taskStarted, taskCompleted int32

	config := Config{
		WorkerCount: 2,
		QueueSize:   1,
		OnWorkerStart: func(workerID int) {
			atomic.AddInt32(&workerStarted, 1)
		},
		OnWorkerStop: func(workerID int) {
			atomic.AddInt32(&workerStopped, 1)
		},
		OnTaskStart: func(workerID int, task Task) {
			atomic.AddInt32(&taskStarted, 1)
		},
		OnTaskComplete: func(workerID int, result Result) {
			atomic.AddInt32(&taskCompleted, 1)
		},
	}

	pool := NewWithConfig(config)

	// Wait for workers to start
	time.Sleep(10 * time.Millisecond)
	testutil.AssertEqual(t, atomic.LoadInt32(&workerStarted), int32(2))

	// Submit a task
	task := &TestTask{ID: 1, Executed: new(int32)}
	pool.Submit(task)

	// Wait for task completion
	<-pool.Results()

	testutil.AssertEqual(t, atomic.LoadInt32(&taskStarted), int32(1))
	testutil.AssertEqual(t, atomic.LoadInt32(&taskCompleted), int32(1))

	// Shutdown and wait
	<-pool.Shutdown()

	testutil.AssertEqual(t, atomic.LoadInt32(&workerStopped), int32(2))
}

func TestConcurrentAccess(t *testing.T) {
	pool := New(5, 20)
	defer pool.Shutdown()

	const numGoroutines = 10
	const tasksPerGoroutine = 20

	var wg sync.WaitGroup
	var totalExecuted int32

	// Start multiple goroutines submitting tasks
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < tasksPerGoroutine; j++ {
				task := &TestTask{
					ID:       goroutineID*1000 + j,
					Duration: time.Millisecond,
					Executed: &totalExecuted,
				}
				if err := pool.Submit(task); err != nil {
					t.Errorf("Failed to submit task: %v", err)
					return
				}
			}
		}(i)
	}

	// Collect results
	expectedTasks := numGoroutines * tasksPerGoroutine
	go func() {
		wg.Wait()
	}()

	// Wait for all tasks to complete
	for i := 0; i < expectedTasks; i++ {
		select {
		case <-pool.Results():
			// Task completed
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting for task %d", i)
		}
	}

	testutil.AssertEqual(t, atomic.LoadInt32(&totalExecuted), int32(expectedTasks))
	testutil.AssertEqual(t, pool.TotalSubmitted(), int64(expectedTasks))
	testutil.AssertEqual(t, pool.TotalCompleted(), int64(expectedTasks))
}

func TestBufferedResults(t *testing.T) {
	config := Config{
		WorkerCount:     2,
		QueueSize:       5,
		BufferedResults: true,
	}

	pool := NewWithConfig(config)
	defer pool.Shutdown()

	// Submit multiple quick tasks
	for i := 0; i < 3; i++ {
		task := &TestTask{
			ID:       i,
			Duration: time.Millisecond,
			Executed: new(int32),
		}
		pool.Submit(task)
	}

	// Results should be available without blocking
	results := make([]Result, 0, 3)
	for i := 0; i < 3; i++ {
		select {
		case result := <-pool.Results():
			results = append(results, result)
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("timeout getting result %d", i)
		}
	}

	testutil.AssertEqual(t, len(results), 3)
}

func TestActiveWorkers(t *testing.T) {
	pool := New(2, 5)
	defer pool.Shutdown()

	testutil.AssertEqual(t, pool.ActiveWorkers(), 0)

	// Submit long-running tasks to occupy both workers
	var executed int32
	for i := 0; i < 2; i++ {
		task := &TestTask{
			ID:       i,
			Duration: 100 * time.Millisecond,
			Executed: &executed,
		}
		pool.Submit(task)
	}

	// Give tasks time to start
	time.Sleep(20 * time.Millisecond)
	testutil.AssertEqual(t, pool.ActiveWorkers(), 2)

	// Wait for tasks to complete
	<-pool.Results()
	<-pool.Results()

	// Active workers should go back to 0
	testutil.AssertEqual(t, pool.ActiveWorkers(), 0)
}

func TestQueueSize(t *testing.T) {
	pool := New(1, 3)
	defer pool.Shutdown()

	testutil.AssertEqual(t, pool.QueueSize(), 0)

	// Submit tasks to fill worker and queue
	var executed int32

	// This task will occupy the worker
	longTask := &TestTask{
		ID:       0,
		Duration: 100 * time.Millisecond,
		Executed: &executed,
	}
	pool.Submit(longTask)
	time.Sleep(10 * time.Millisecond) // Give time for worker to pick up task

	// These tasks will go to the queue
	for i := 1; i <= 3; i++ {
		task := &TestTask{
			ID:       i,
			Duration: 10 * time.Millisecond,
			Executed: &executed,
		}
		pool.Submit(task)
	}

	testutil.AssertEqual(t, pool.QueueSize(), 3)

	// Wait for all tasks to complete
	for i := 0; i < 4; i++ {
		<-pool.Results()
	}

	testutil.AssertEqual(t, pool.QueueSize(), 0)
}
