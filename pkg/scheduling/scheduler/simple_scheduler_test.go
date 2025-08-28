package scheduler

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

func TestScheduler_BasicScheduling(t *testing.T) {
	s := New()
	defer func() { <-s.Stop() }()

	if err := s.Start(); err != nil {
		t.Fatal(err)
	}

	var executed int32
	task := workerpool.TaskFunc(func(ctx context.Context) error {
		atomic.AddInt32(&executed, 1)
		return nil
	})

	// Test immediate scheduling
	if err := s.Schedule("test1", task, time.Now()); err != nil {
		t.Fatal(err)
	}

	// Test delayed scheduling
	if err := s.ScheduleAfter("test2", task, 50*time.Millisecond); err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)

	if count := atomic.LoadInt32(&executed); count != 2 {
		t.Errorf("expected 2 executions, got %d", count)
	}
}

func TestScheduler_RepeatingTask(t *testing.T) {
	s := New()
	defer func() { <-s.Stop() }()

	if err := s.Start(); err != nil {
		t.Fatal(err)
	}

	var executed int32
	task := workerpool.TaskFunc(func(ctx context.Context) error {
		atomic.AddInt32(&executed, 1)
		return nil
	})

	if err := s.ScheduleRepeating("repeat", task, 75*time.Millisecond); err != nil {
		t.Fatal(err)
	}

	time.Sleep(300 * time.Millisecond) // Should run at least 3 times

	if count := atomic.LoadInt32(&executed); count < 3 {
		t.Errorf("expected at least 3 executions, got %d", count)
	}
}

func TestScheduler_CronScheduling(t *testing.T) {
	s := New()
	defer func() { <-s.Stop() }()

	if err := s.Start(); err != nil {
		t.Fatal(err)
	}

	var executed int32
	task := workerpool.TaskFunc(func(ctx context.Context) error {
		atomic.AddInt32(&executed, 1)
		return nil
	})

	// Schedule to run every second
	if err := s.ScheduleCron("cron", "* * * * * *", task); err != nil {
		t.Fatal(err)
	}

	time.Sleep(1200 * time.Millisecond)

	if count := atomic.LoadInt32(&executed); count == 0 {
		t.Error("cron task should have executed at least once")
	}
}

func TestScheduler_TaskManagement(t *testing.T) {
	s := New()
	defer func() { <-s.Stop() }()

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		return nil
	})

	// Test duplicate ID prevention
	if err := s.Schedule("dup", task, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	if err := s.Schedule("dup", task, time.Now().Add(time.Hour)); err == nil {
		t.Error("should not allow duplicate task IDs")
	}

	// Test task listing
	tasks := s.List()
	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}

	// Test cancellation
	if !s.Cancel("dup") {
		t.Error("should successfully cancel existing task")
	}

	if s.Cancel("nonexistent") {
		t.Error("should return false for nonexistent task")
	}

	tasks = s.List()
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks after cancellation, got %d", len(tasks))
	}
}

func TestBackoffTask(t *testing.T) {
	attempts := 0
	failingTask := workerpool.TaskFunc(func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary failure")
		}
		return nil
	})

	backoffTask := BackoffTask{
		Task:         failingTask,
		MaxRetries:   5,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
	}

	ctx := context.Background()
	err := backoffTask.Execute(ctx)
	
	if err != nil {
		t.Errorf("expected task to succeed after retries, got error: %v", err)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestScheduler_InputValidation(t *testing.T) {
	s := New()
	task := workerpool.TaskFunc(func(ctx context.Context) error { return nil })

	tests := []struct {
		name string
		fn   func() error
	}{
		{
			"empty ID",
			func() error { return s.Schedule("", task, time.Now()) },
		},
		{
			"nil task",
			func() error { return s.Schedule("test", nil, time.Now()) },
		},
		{
			"negative interval",
			func() error { return s.ScheduleRepeating("test", task, -time.Second) },
		},
		{
			"empty cron expression",
			func() error { return s.ScheduleCron("test", "", task) },
		},
		{
			"invalid cron expression",
			func() error { return s.ScheduleCron("test", "invalid", task) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.fn(); err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

