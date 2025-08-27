package scheduler

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vnykmshr/goflow/internal/testutil"
	"github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

// TestTask is a helper for testing scheduled tasks.
type TestTask struct {
	ID         string
	Executed   *int32
	Duration   time.Duration
	ShouldFail bool
}

func (t *TestTask) Execute(ctx context.Context) error {
	atomic.AddInt32(t.Executed, 1)
	if t.Duration > 0 {
		select {
		case <-time.After(t.Duration):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if t.ShouldFail {
		return context.Canceled
	}
	return nil
}

func TestNew(t *testing.T) {
	s := New()
	if s == nil {
		t.Fatal("scheduler should not be nil")
	}
	testutil.AssertEqual(t, s.IsRunning(), false)
}

func TestNewWithConfig(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name:   "default config",
			config: Config{},
		},
		{
			name: "custom tick interval",
			config: Config{
				TickInterval: 50 * time.Millisecond,
			},
		},
		{
			name: "with worker pool",
			config: Config{
				WorkerPool: workerpool.New(2, 50),
			},
		},
		{
			name: "with timeout",
			config: Config{
				TaskTimeout: time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewWithConfig(tt.config)
			if s == nil {
				t.Fatal("scheduler should not be nil")
			}
			testutil.AssertEqual(t, s.IsRunning(), false)

			// Clean up
			<-s.Stop()
		})
	}
}

func TestSchedule(t *testing.T) {
	s := New()
	defer func() { <-s.Stop() }()

	var executed int32
	task := &TestTask{ID: "test", Executed: &executed}

	tests := []struct {
		name        string
		id          string
		task        workerpool.Task
		runAt       time.Time
		expectError bool
	}{
		{
			name:        "valid task",
			id:          "task1",
			task:        task,
			runAt:       time.Now().Add(time.Hour),
			expectError: false,
		},
		{
			name:        "nil task",
			id:          "task2",
			task:        nil,
			runAt:       time.Now().Add(time.Hour),
			expectError: true,
		},
		{
			name:        "empty id",
			id:          "",
			task:        task,
			runAt:       time.Now().Add(time.Hour),
			expectError: true,
		},
		{
			name:        "duplicate id",
			id:          "task1",
			task:        task,
			runAt:       time.Now().Add(time.Hour),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.Schedule(tt.id, tt.task, tt.runAt)
			if tt.expectError {
				testutil.AssertError(t, err)
			} else {
				testutil.AssertNoError(t, err)
			}
		})
	}
}

func TestScheduleExecution(t *testing.T) {
	s := New()
	defer func() { <-s.Stop() }()

	var executed int32
	task := &TestTask{ID: "test", Executed: &executed}

	// Schedule task to run very soon
	runAt := time.Now().Add(50 * time.Millisecond)
	err := s.Schedule("test-task", task, runAt)
	testutil.AssertNoError(t, err)

	// Start scheduler
	err = s.Start()
	testutil.AssertNoError(t, err)

	// Wait for task to execute
	time.Sleep(200 * time.Millisecond)

	// Verify task was executed
	testutil.AssertEqual(t, atomic.LoadInt32(&executed), int32(1))

	// Verify task was removed after execution
	_, exists := s.GetTask("test-task")
	testutil.AssertEqual(t, exists, false)
}

func TestScheduleRepeating(t *testing.T) {
	s := New()
	defer func() { <-s.Stop() }()

	tests := []struct {
		name        string
		id          string
		task        workerpool.Task
		interval    time.Duration
		maxRuns     int
		expectError bool
	}{
		{
			name:        "valid repeating task",
			id:          "repeat1",
			task:        &TestTask{ID: "test", Executed: new(int32)},
			interval:    100 * time.Millisecond,
			maxRuns:     3,
			expectError: false,
		},
		{
			name:        "infinite repeating task",
			id:          "repeat2",
			task:        &TestTask{ID: "test", Executed: new(int32)},
			interval:    100 * time.Millisecond,
			maxRuns:     0,
			expectError: false,
		},
		{
			name:        "nil task",
			id:          "repeat3",
			task:        nil,
			interval:    100 * time.Millisecond,
			maxRuns:     1,
			expectError: true,
		},
		{
			name:        "zero interval",
			id:          "repeat4",
			task:        &TestTask{ID: "test", Executed: new(int32)},
			interval:    0,
			maxRuns:     1,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.ScheduleRepeating(tt.id, tt.task, tt.interval, tt.maxRuns)
			if tt.expectError {
				testutil.AssertError(t, err)
			} else {
				testutil.AssertNoError(t, err)
			}
		})
	}
}

func TestScheduleRepeatingExecution(t *testing.T) {
	s := New()
	defer func() { <-s.Stop() }()

	var executed int32
	task := &TestTask{ID: "test", Executed: &executed}

	// Schedule repeating task (3 times, every 80ms)
	err := s.ScheduleRepeating("repeat-task", task, 80*time.Millisecond, 3)
	testutil.AssertNoError(t, err)

	// Start scheduler
	err = s.Start()
	testutil.AssertNoError(t, err)

	// Wait for all executions (80ms + 80ms + 80ms + buffer)
	time.Sleep(400 * time.Millisecond)

	// Verify task was executed 3 times
	executions := atomic.LoadInt32(&executed)
	testutil.AssertEqual(t, executions, int32(3))

	// Verify task was removed after max runs
	_, exists := s.GetTask("repeat-task")
	testutil.AssertEqual(t, exists, false)
}

func TestScheduleAfter(t *testing.T) {
	s := New()
	defer func() { <-s.Stop() }()

	var executed int32
	task := &TestTask{ID: "test", Executed: &executed}

	// Schedule task to run after delay
	err := s.ScheduleAfter("delayed-task", task, 50*time.Millisecond)
	testutil.AssertNoError(t, err)

	// Start scheduler
	err = s.Start()
	testutil.AssertNoError(t, err)

	// Wait for task to execute
	time.Sleep(150 * time.Millisecond)

	// Verify task was executed
	testutil.AssertEqual(t, atomic.LoadInt32(&executed), int32(1))
}

func TestScheduleEvery(t *testing.T) {
	s := New()
	defer func() { <-s.Stop() }()

	var executed int32
	task := &TestTask{ID: "test", Executed: &executed}

	// Schedule task to run immediately and then every 70ms, max 2 times
	err := s.ScheduleEvery("every-task", task, 70*time.Millisecond, 2)
	testutil.AssertNoError(t, err)

	// Start scheduler
	err = s.Start()
	testutil.AssertNoError(t, err)

	// Wait for executions (immediate + 70ms + buffer)
	time.Sleep(250 * time.Millisecond)

	// Verify task was executed 2 times
	executions := atomic.LoadInt32(&executed)
	testutil.AssertEqual(t, executions, int32(2))
}

func TestCancel(t *testing.T) {
	s := New()
	defer func() { <-s.Stop() }()

	var executed int32
	task := &TestTask{ID: "test", Executed: &executed}

	// Schedule task
	err := s.Schedule("cancel-task", task, time.Now().Add(time.Hour))
	testutil.AssertNoError(t, err)

	// Verify task exists
	_, exists := s.GetTask("cancel-task")
	testutil.AssertEqual(t, exists, true)

	// Cancel task
	canceled := s.Cancel("cancel-task")
	testutil.AssertEqual(t, canceled, true)

	// Verify task was removed
	_, exists = s.GetTask("cancel-task")
	testutil.AssertEqual(t, exists, false)

	// Try to cancel non-existent task
	canceled = s.Cancel("non-existent")
	testutil.AssertEqual(t, canceled, false)
}

func TestCancelAll(t *testing.T) {
	s := New()
	defer func() { <-s.Stop() }()

	var executed1, executed2 int32
	task1 := &TestTask{ID: "test1", Executed: &executed1}
	task2 := &TestTask{ID: "test2", Executed: &executed2}

	// Schedule multiple tasks
	err := s.Schedule("task1", task1, time.Now().Add(time.Hour))
	testutil.AssertNoError(t, err)
	err = s.Schedule("task2", task2, time.Now().Add(time.Hour))
	testutil.AssertNoError(t, err)

	// Verify tasks exist
	tasks := s.ListTasks()
	testutil.AssertEqual(t, len(tasks), 2)

	// Cancel all tasks
	s.CancelAll()

	// Verify all tasks were removed
	tasks = s.ListTasks()
	testutil.AssertEqual(t, len(tasks), 0)
}

func TestGetTask(t *testing.T) {
	s := New()
	defer func() { <-s.Stop() }()

	var executed int32
	task := &TestTask{ID: "test", Executed: &executed}
	runAt := time.Now().Add(time.Hour)

	// Schedule task
	err := s.Schedule("get-task", task, runAt)
	testutil.AssertNoError(t, err)

	// Get task
	scheduledTask, exists := s.GetTask("get-task")
	testutil.AssertEqual(t, exists, true)
	if scheduledTask == nil {
		t.Fatal("scheduled task should not be nil")
	}
	testutil.AssertEqual(t, scheduledTask.ID, "get-task")
	testutil.AssertEqual(t, scheduledTask.IsRepeating, false)

	// Try to get non-existent task
	_, exists = s.GetTask("non-existent")
	testutil.AssertEqual(t, exists, false)
}

func TestListTasks(t *testing.T) {
	s := New()
	defer func() { <-s.Stop() }()

	// Initially no tasks
	tasks := s.ListTasks()
	testutil.AssertEqual(t, len(tasks), 0)

	// Schedule some tasks
	var executed1, executed2 int32
	task1 := &TestTask{ID: "test1", Executed: &executed1}
	task2 := &TestTask{ID: "test2", Executed: &executed2}

	err := s.Schedule("task1", task1, time.Now().Add(time.Hour))
	testutil.AssertNoError(t, err)
	err = s.ScheduleRepeating("task2", task2, time.Second, 5)
	testutil.AssertNoError(t, err)

	// List tasks
	tasks = s.ListTasks()
	testutil.AssertEqual(t, len(tasks), 2)

	// Find and verify tasks
	var oneTimeTask, repeatingTask *ScheduledTask
	for i := range tasks {
		if tasks[i].ID == "task1" {
			oneTimeTask = &tasks[i]
		} else if tasks[i].ID == "task2" {
			repeatingTask = &tasks[i]
		}
	}

	if oneTimeTask == nil {
		t.Fatal("one-time task should be found")
	}
	testutil.AssertEqual(t, oneTimeTask.IsRepeating, false)

	if repeatingTask == nil {
		t.Fatal("repeating task should be found")
	}
	testutil.AssertEqual(t, repeatingTask.IsRepeating, true)
	testutil.AssertEqual(t, repeatingTask.MaxRuns, 5)
}

func TestStartStop(t *testing.T) {
	s := New()

	// Initially not running
	testutil.AssertEqual(t, s.IsRunning(), false)

	// Start scheduler
	err := s.Start()
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, s.IsRunning(), true)

	// Try to start again (should error)
	err = s.Start()
	testutil.AssertError(t, err)

	// Stop scheduler
	done := s.Stop()
	<-done
	testutil.AssertEqual(t, s.IsRunning(), false)
}

func TestStats(t *testing.T) {
	s := New()
	defer func() { <-s.Stop() }()

	// Initially empty stats
	stats := s.Stats()
	testutil.AssertEqual(t, stats.TotalScheduled, int64(0))
	testutil.AssertEqual(t, stats.CurrentScheduled, 0)

	// Schedule some tasks
	var executed1, executed2 int32
	task1 := &TestTask{ID: "test1", Executed: &executed1}
	task2 := &TestTask{ID: "test2", Executed: &executed2}

	err := s.Schedule("task1", task1, time.Now().Add(50*time.Millisecond))
	testutil.AssertNoError(t, err)
	err = s.Schedule("task2", task2, time.Now().Add(time.Hour))
	testutil.AssertNoError(t, err)

	// Check stats after scheduling
	stats = s.Stats()
	testutil.AssertEqual(t, stats.TotalScheduled, int64(2))
	testutil.AssertEqual(t, stats.CurrentScheduled, 2)

	// Start scheduler and wait for one task to execute
	err = s.Start()
	testutil.AssertNoError(t, err)
	time.Sleep(150 * time.Millisecond)

	// Check stats after execution
	stats = s.Stats()
	testutil.AssertEqual(t, stats.TotalScheduled, int64(2))
	testutil.AssertEqual(t, stats.TotalExecuted, int64(1))
	testutil.AssertEqual(t, stats.CurrentScheduled, 1)

	// Cancel remaining task
	s.Cancel("task2")
	stats = s.Stats()
	testutil.AssertEqual(t, stats.TotalCanceled, int64(1))
	testutil.AssertEqual(t, stats.CurrentScheduled, 0)
}

func TestCallbacks(t *testing.T) {
	var scheduledCount, executedCount, canceledCount int32

	config := Config{
		OnTaskScheduled: func(task ScheduledTask) {
			atomic.AddInt32(&scheduledCount, 1)
		},
		OnTaskExecuted: func(task ScheduledTask, result workerpool.Result) {
			atomic.AddInt32(&executedCount, 1)
		},
		OnTaskCanceled: func(task ScheduledTask) {
			atomic.AddInt32(&canceledCount, 1)
		},
	}

	s := NewWithConfig(config)
	defer func() { <-s.Stop() }()

	var executed int32
	task := &TestTask{ID: "test", Executed: &executed}

	// Schedule task (should trigger scheduled callback)
	err := s.Schedule("callback-task", task, time.Now().Add(50*time.Millisecond))
	testutil.AssertNoError(t, err)

	// Start and wait for execution (should trigger executed callback)
	err = s.Start()
	testutil.AssertNoError(t, err)
	time.Sleep(150 * time.Millisecond)

	// Schedule another task and cancel it (should trigger canceled callback)
	err = s.Schedule("cancel-task", task, time.Now().Add(time.Hour))
	testutil.AssertNoError(t, err)
	s.Cancel("cancel-task")

	// Verify callbacks were called
	testutil.AssertEqual(t, atomic.LoadInt32(&scheduledCount), int32(2))
	testutil.AssertEqual(t, atomic.LoadInt32(&executedCount), int32(1))
	testutil.AssertEqual(t, atomic.LoadInt32(&canceledCount), int32(1))
}

func TestTaskTimeout(t *testing.T) {
	config := Config{
		TaskTimeout: 50 * time.Millisecond,
	}

	s := NewWithConfig(config)
	defer func() { <-s.Stop() }()

	var executed int32
	task := &TestTask{
		ID:       "timeout-test",
		Executed: &executed,
		Duration: 100 * time.Millisecond, // Longer than timeout
	}

	// Schedule task
	err := s.Schedule("timeout-task", task, time.Now().Add(10*time.Millisecond))
	testutil.AssertNoError(t, err)

	// Start and wait
	err = s.Start()
	testutil.AssertNoError(t, err)
	time.Sleep(200 * time.Millisecond)

	// Task should have started but may have been canceled due to timeout
	// The exact behavior depends on timing, but the task should be removed
	_, exists := s.GetTask("timeout-task")
	testutil.AssertEqual(t, exists, false)
}

func TestConcurrentAccess(t *testing.T) {
	s := New()
	defer func() { <-s.Stop() }()

	err := s.Start()
	testutil.AssertNoError(t, err)

	// Test concurrent scheduling, canceling, and querying
	done := make(chan struct{})
	numGoroutines := 10
	tasksPerGoroutine := 20

	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer func() { done <- struct{}{} }()

			for j := 0; j < tasksPerGoroutine; j++ {
				taskID := fmt.Sprintf("task-%d-%d", goroutineID, j)
				var executed int32
				task := &TestTask{ID: taskID, Executed: &executed}

				// Schedule task
				err := s.Schedule(taskID, task, time.Now().Add(time.Millisecond*time.Duration(j+1)))
				if err != nil {
					continue // Task may already exist due to timing
				}

				// Randomly cancel some tasks
				if j%3 == 0 {
					s.Cancel(taskID)
				}

				// Query tasks
				s.ListTasks()
				s.GetTask(taskID)
				s.Stats()
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Let tasks execute
	time.Sleep(100 * time.Millisecond)

	// Scheduler should still be functional
	testutil.AssertEqual(t, s.IsRunning(), true)
}
