package scheduler

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

// Task represents a scheduled task.
type Task struct {
	ID       string
	RunAt    time.Time
	Interval time.Duration // Zero for one-time tasks
	Created  time.Time
}

// Scheduler provides task scheduling with cron support.
type Scheduler interface {
	// Basic scheduling
	Schedule(id string, task workerpool.Task, runAt time.Time) error
	ScheduleAfter(id string, task workerpool.Task, delay time.Duration) error
	ScheduleRepeating(id string, task workerpool.Task, interval time.Duration) error

	// Cron scheduling
	ScheduleCron(id string, cronExpr string, task workerpool.Task) error

	// Task management
	Cancel(id string) bool
	CancelAll()
	List() []Task

	// Lifecycle
	Start() error
	Stop() <-chan struct{}
}

// BackoffTask wraps a task with retry logic.
type BackoffTask struct {
	Task         workerpool.Task
	MaxRetries   int
	InitialDelay time.Duration
	MaxDelay     time.Duration
}

// Execute implements workerpool.Task with exponential backoff.
func (bt BackoffTask) Execute(ctx context.Context) error {
	if err := bt.validate(); err != nil {
		return err
	}

	var lastErr error
	delay := bt.InitialDelay

	for attempt := 0; attempt <= bt.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		lastErr = bt.Task.Execute(ctx)
		if lastErr == nil {
			return nil
		}

		// Double delay for next attempt
		delay *= 2
		if delay > bt.MaxDelay {
			delay = bt.MaxDelay
		}
	}

	return lastErr
}

// validate checks and sets defaults for BackoffTask configuration.
func (bt *BackoffTask) validate() error {
	if bt.Task == nil {
		return fmt.Errorf("wrapped task cannot be nil")
	}
	if bt.MaxRetries < 0 {
		return fmt.Errorf("max retries cannot be negative")
	}
	if bt.InitialDelay <= 0 {
		bt.InitialDelay = 100 * time.Millisecond // reasonable default
	}
	if bt.MaxDelay <= 0 {
		bt.MaxDelay = 30 * time.Second // reasonable default
	}
	return nil
}

// Config holds scheduler configuration.
type Config struct {
	WorkerPool   workerpool.Pool
	Location     *time.Location
	TickInterval time.Duration
	MaxTasks     int
}

// Default values
const (
	defaultTickInterval = 50 * time.Millisecond
	defaultMaxTasks     = 10000
	maxTaskIDLength     = 255
)

type scheduledTask struct {
	id           string
	task         workerpool.Task
	runAt        time.Time
	interval     time.Duration
	cronSchedule cron.Schedule
	created      time.Time
}

type scheduler struct {
	pool         workerpool.Pool
	ownPool      bool
	location     *time.Location
	tickInterval time.Duration
	maxTasks     int
	cronParser   cron.Parser

	mu      sync.RWMutex
	tasks   map[string]*scheduledTask
	ticker  *time.Ticker
	done    chan struct{}
	running bool
}

// New creates a scheduler with default configuration.
func New() Scheduler {
	return NewWithConfig(Config{})
}

// NewWithConfig creates a scheduler with custom configuration.
func NewWithConfig(cfg Config) Scheduler {
	s := &scheduler{
		pool:         cfg.WorkerPool,
		location:     cfg.Location,
		tickInterval: cfg.TickInterval,
		maxTasks:     cfg.MaxTasks,
		cronParser:   cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow),
		tasks:        make(map[string]*scheduledTask),
		done:         make(chan struct{}),
	}

	// Apply defaults
	if s.pool == nil {
		s.pool = workerpool.New(4, 100)
		s.ownPool = true
	}
	if s.location == nil {
		s.location = time.Local
	}
	if s.tickInterval <= 0 {
		s.tickInterval = defaultTickInterval
	}
	if s.maxTasks <= 0 {
		s.maxTasks = defaultMaxTasks
	}

	return s
}

// validateTaskRequest validates common task parameters.
func (s *scheduler) validateTaskRequest(id string, task workerpool.Task) error {
	if id == "" {
		return fmt.Errorf("task ID cannot be empty")
	}
	if len(id) > maxTaskIDLength {
		return fmt.Errorf("task ID too long (max %d characters)", maxTaskIDLength)
	}
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}
	return nil
}

// checkTaskExists checks if a task already exists and if we're at capacity.
func (s *scheduler) checkTaskExists(id string) error {
	if _, exists := s.tasks[id]; exists {
		return fmt.Errorf("task with ID %q already exists, use a different ID or cancel the existing task first", id)
	}
	if len(s.tasks) >= s.maxTasks {
		return fmt.Errorf("cannot schedule task: maximum number of tasks (%d) reached", s.maxTasks)
	}
	return nil
}

func (s *scheduler) Schedule(id string, task workerpool.Task, runAt time.Time) error {
	if err := s.validateTaskRequest(id, task); err != nil {
		return err
	}
	if runAt.IsZero() {
		return fmt.Errorf("task run time cannot be zero")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.checkTaskExists(id); err != nil {
		return err
	}

	s.tasks[id] = &scheduledTask{
		id:      id,
		task:    task,
		runAt:   runAt,
		created: time.Now(),
	}

	return nil
}

func (s *scheduler) ScheduleAfter(id string, task workerpool.Task, delay time.Duration) error {
	return s.Schedule(id, task, time.Now().Add(delay))
}

func (s *scheduler) ScheduleRepeating(id string, task workerpool.Task, interval time.Duration) error {
	if err := s.validateTaskRequest(id, task); err != nil {
		return err
	}
	if interval <= 0 {
		return fmt.Errorf("interval must be positive, got %v", interval)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.checkTaskExists(id); err != nil {
		return err
	}

	s.tasks[id] = &scheduledTask{
		id:       id,
		task:     task,
		runAt:    time.Now(),
		interval: interval,
		created:  time.Now(),
	}

	return nil
}

func (s *scheduler) ScheduleCron(id string, cronExpr string, task workerpool.Task) error {
	if err := s.validateTaskRequest(id, task); err != nil {
		return err
	}
	if cronExpr == "" {
		return fmt.Errorf("cron expression cannot be empty")
	}

	schedule, err := s.cronParser.Parse(cronExpr)
	if err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.checkTaskExists(id); err != nil {
		return err
	}

	now := time.Now().In(s.location)
	s.tasks[id] = &scheduledTask{
		id:           id,
		task:         task,
		runAt:        schedule.Next(now),
		cronSchedule: schedule,
		created:      time.Now(),
	}

	return nil
}

func (s *scheduler) Cancel(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[id]; exists {
		delete(s.tasks, id)
		return true
	}
	return false
}

func (s *scheduler) CancelAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tasks = make(map[string]*scheduledTask)
}

func (s *scheduler) List() []Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		tasks = append(tasks, Task{
			ID:       t.id,
			RunAt:    t.runAt,
			Interval: t.interval,
			Created:  t.created,
		})
	}

	// Sort by run time
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].RunAt.Before(tasks[j].RunAt)
	})

	return tasks
}

func (s *scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("scheduler already running, call Stop() first")
	}

	s.running = true
	s.ticker = time.NewTicker(s.tickInterval)

	go s.run()
	return nil
}

func (s *scheduler) Stop() <-chan struct{} {
	s.mu.Lock()
	if s.running {
		s.running = false
		close(s.done)
		if s.ticker != nil {
			s.ticker.Stop()
		}
	}
	s.mu.Unlock()

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		if s.ownPool {
			<-s.pool.Shutdown()
		}
	}()

	return stopped
}

func (s *scheduler) run() {
	defer func() {
		if s.ticker != nil {
			s.ticker.Stop()
		}
	}()

	for {
		select {
		case <-s.done:
			return
		case <-s.ticker.C:
			s.safeProcessTasks()
		}
	}
}

// safeProcessTasks processes ready tasks with panic recovery.
func (s *scheduler) safeProcessTasks() {
	defer func() {
		if r := recover(); r != nil {
			// Log panic but continue running scheduler
			// In production, you'd want proper logging here
			_ = r // Acknowledge we're intentionally ignoring the panic value
		}
	}()
	s.processReadyTasks()
}

func (s *scheduler) processReadyTasks() {
	now := time.Now()

	s.mu.Lock()
	readyTasks := s.findReadyTasks(now)
	s.mu.Unlock()

	// Execute ready tasks outside the lock
	s.submitTasks(readyTasks)
}

// findReadyTasks finds and reschedules ready tasks (must be called with lock held).
func (s *scheduler) findReadyTasks(now time.Time) []*scheduledTask {
	if len(s.tasks) == 0 {
		return nil
	}

	readyTasks := make([]*scheduledTask, 0, len(s.tasks))

	for id, task := range s.tasks {
		if s.isTaskReady(task, now) {
			readyTasks = append(readyTasks, task)
			s.rescheduleOrRemoveTask(id, task, now)
		}
	}

	return readyTasks
}

// isTaskReady checks if a task is ready to execute.
func (s *scheduler) isTaskReady(task *scheduledTask, now time.Time) bool {
	return now.After(task.runAt) || now.Equal(task.runAt)
}

// rescheduleOrRemoveTask handles task rescheduling or removal.
func (s *scheduler) rescheduleOrRemoveTask(id string, task *scheduledTask, now time.Time) {
	switch {
	case task.interval > 0:
		// Repeating task
		task.runAt = now.Add(task.interval)
	case task.cronSchedule != nil:
		// Cron task
		task.runAt = task.cronSchedule.Next(now.In(s.location))
	default:
		// One-time task
		delete(s.tasks, id)
	}
}

// submitTasks submits tasks to the worker pool.
func (s *scheduler) submitTasks(tasks []*scheduledTask) {
	for _, task := range tasks {
		if err := s.pool.Submit(task.task); err != nil {
			// Task submission failed, but continue processing other tasks
			continue
		}
	}
}
