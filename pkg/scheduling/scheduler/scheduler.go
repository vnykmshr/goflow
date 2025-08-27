package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

// ScheduledTask represents a task with scheduling information.
type ScheduledTask struct {
	ID          string
	Task        workerpool.Task
	RunAt       time.Time
	Interval    time.Duration
	IsRepeating bool
	MaxRuns     int
	RunCount    int
	CreatedAt   time.Time
}

// Scheduler manages scheduled task execution.
type Scheduler interface {
	// Schedule adds a one-time task to be executed at the specified time.
	Schedule(id string, task workerpool.Task, runAt time.Time) error

	// ScheduleRepeating adds a repeating task with the given interval.
	// maxRuns <= 0 means infinite repetitions.
	ScheduleRepeating(id string, task workerpool.Task, interval time.Duration, maxRuns int) error

	// ScheduleAfter adds a one-time task to be executed after the specified delay.
	ScheduleAfter(id string, task workerpool.Task, delay time.Duration) error

	// ScheduleEvery adds a repeating task that starts immediately and repeats at the given interval.
	ScheduleEvery(id string, task workerpool.Task, interval time.Duration, maxRuns int) error

	// Cancel removes a scheduled task by ID.
	// Returns true if the task was found and canceled.
	Cancel(id string) bool

	// CancelAll removes all scheduled tasks.
	CancelAll()

	// GetTask returns information about a scheduled task.
	GetTask(id string) (*ScheduledTask, bool)

	// ListTasks returns all currently scheduled tasks.
	ListTasks() []ScheduledTask

	// Start begins the scheduler's execution loop.
	Start() error

	// Stop gracefully shuts down the scheduler.
	Stop() <-chan struct{}

	// IsRunning returns whether the scheduler is currently running.
	IsRunning() bool

	// Stats returns scheduler statistics.
	Stats() Stats
}

// Stats holds scheduler performance metrics.
type Stats struct {
	TotalScheduled   int64
	TotalExecuted    int64
	TotalCanceled    int64
	CurrentScheduled int
	UpTime           time.Duration
	StartedAt        time.Time
}

// Config holds scheduler configuration.
type Config struct {
	// WorkerPool is the worker pool for executing scheduled tasks.
	// If nil, a default pool with 4 workers will be created.
	WorkerPool workerpool.Pool

	// TickInterval determines how often the scheduler checks for ready tasks.
	// Default is 100ms.
	TickInterval time.Duration

	// TaskTimeout is the default timeout for scheduled tasks.
	// Zero means no timeout.
	TaskTimeout time.Duration

	// OnTaskScheduled is called when a task is scheduled.
	OnTaskScheduled func(task ScheduledTask)

	// OnTaskExecuted is called when a scheduled task is executed.
	OnTaskExecuted func(task ScheduledTask, result workerpool.Result)

	// OnTaskCanceled is called when a scheduled task is canceled.
	OnTaskCanceled func(task ScheduledTask)

	// OnError is called when the scheduler encounters an error.
	OnError func(err error)
}

// scheduler implements the Scheduler interface.
type scheduler struct {
	config         Config
	pool           workerpool.Pool
	ownsPool       bool
	tasks          map[string]*ScheduledTask
	mu             sync.RWMutex
	ticker         *time.Ticker
	stopCh         chan struct{}
	stopOnce       sync.Once
	isRunning      bool
	startedAt      time.Time
	totalScheduled int64
	totalExecuted  int64
	totalCancelled int64
}

// New creates a new scheduler with default configuration.
func New() Scheduler {
	return NewWithConfig(Config{})
}

// NewWithConfig creates a new scheduler with the specified configuration.
func NewWithConfig(config Config) Scheduler {
	if config.TickInterval <= 0 {
		config.TickInterval = 100 * time.Millisecond
	}

	var pool workerpool.Pool
	var ownsPool bool
	if config.WorkerPool != nil {
		pool = config.WorkerPool
		ownsPool = false
	} else {
		pool = workerpool.New(4, 100)
		ownsPool = true
	}

	return &scheduler{
		config:   config,
		pool:     pool,
		ownsPool: ownsPool,
		tasks:    make(map[string]*ScheduledTask),
		stopCh:   make(chan struct{}),
	}
}

// Schedule adds a one-time task to be executed at the specified time.
func (s *scheduler) Schedule(id string, task workerpool.Task, runAt time.Time) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}
	if id == "" {
		return fmt.Errorf("task ID cannot be empty")
	}

	scheduledTask := &ScheduledTask{
		ID:          id,
		Task:        task,
		RunAt:       runAt,
		IsRepeating: false,
		MaxRuns:     1,
		RunCount:    0,
		CreatedAt:   time.Now(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[id]; exists {
		return fmt.Errorf("task with ID %s already exists", id)
	}

	s.tasks[id] = scheduledTask
	s.totalScheduled++

	if s.config.OnTaskScheduled != nil {
		s.config.OnTaskScheduled(*scheduledTask)
	}

	return nil
}

// ScheduleRepeating adds a repeating task with the given interval.
func (s *scheduler) ScheduleRepeating(id string, task workerpool.Task, interval time.Duration, maxRuns int) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}
	if id == "" {
		return fmt.Errorf("task ID cannot be empty")
	}
	if interval <= 0 {
		return fmt.Errorf("interval must be positive")
	}

	scheduledTask := &ScheduledTask{
		ID:          id,
		Task:        task,
		RunAt:       time.Now().Add(interval),
		Interval:    interval,
		IsRepeating: true,
		MaxRuns:     maxRuns,
		RunCount:    0,
		CreatedAt:   time.Now(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[id]; exists {
		return fmt.Errorf("task with ID %s already exists", id)
	}

	s.tasks[id] = scheduledTask
	s.totalScheduled++

	if s.config.OnTaskScheduled != nil {
		s.config.OnTaskScheduled(*scheduledTask)
	}

	return nil
}

// ScheduleAfter adds a one-time task to be executed after the specified delay.
func (s *scheduler) ScheduleAfter(id string, task workerpool.Task, delay time.Duration) error {
	return s.Schedule(id, task, time.Now().Add(delay))
}

// ScheduleEvery adds a repeating task that starts immediately and repeats at the given interval.
func (s *scheduler) ScheduleEvery(id string, task workerpool.Task, interval time.Duration, maxRuns int) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}
	if id == "" {
		return fmt.Errorf("task ID cannot be empty")
	}
	if interval <= 0 {
		return fmt.Errorf("interval must be positive")
	}

	scheduledTask := &ScheduledTask{
		ID:          id,
		Task:        task,
		RunAt:       time.Now(),
		Interval:    interval,
		IsRepeating: true,
		MaxRuns:     maxRuns,
		RunCount:    0,
		CreatedAt:   time.Now(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[id]; exists {
		return fmt.Errorf("task with ID %s already exists", id)
	}

	s.tasks[id] = scheduledTask
	s.totalScheduled++

	if s.config.OnTaskScheduled != nil {
		s.config.OnTaskScheduled(*scheduledTask)
	}

	return nil
}

// Cancel removes a scheduled task by ID.
func (s *scheduler) Cancel(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[id]
	if !exists {
		return false
	}

	delete(s.tasks, id)
	s.totalCancelled++

	if s.config.OnTaskCanceled != nil {
		s.config.OnTaskCanceled(*task)
	}

	return true
}

// CancelAll removes all scheduled tasks.
func (s *scheduler) CancelAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, task := range s.tasks {
		if s.config.OnTaskCanceled != nil {
			s.config.OnTaskCanceled(*task)
		}
		delete(s.tasks, id)
		s.totalCancelled++
	}
}

// GetTask returns information about a scheduled task.
func (s *scheduler) GetTask(id string) (*ScheduledTask, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[id]
	if !exists {
		return nil, false
	}

	// Return a copy to avoid external mutation
	taskCopy := *task
	return &taskCopy, true
}

// ListTasks returns all currently scheduled tasks.
func (s *scheduler) ListTasks() []ScheduledTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]ScheduledTask, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, *task)
	}

	return tasks
}

// Start begins the scheduler's execution loop.
func (s *scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		return fmt.Errorf("scheduler is already running")
	}

	s.isRunning = true
	s.startedAt = time.Now()
	s.ticker = time.NewTicker(s.config.TickInterval)

	go s.run()

	return nil
}

// Stop gracefully shuts down the scheduler.
func (s *scheduler) Stop() <-chan struct{} {
	done := make(chan struct{})

	s.stopOnce.Do(func() {
		s.mu.Lock()
		if s.isRunning {
			s.isRunning = false
			close(s.stopCh)
			if s.ticker != nil {
				s.ticker.Stop()
			}
		}
		s.mu.Unlock()

		go func() {
			if s.ownsPool {
				<-s.pool.Shutdown()
			}
			close(done)
		}()
	})

	return done
}

// IsRunning returns whether the scheduler is currently running.
func (s *scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isRunning
}

// Stats returns scheduler statistics.
func (s *scheduler) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var uptime time.Duration
	if !s.startedAt.IsZero() {
		uptime = time.Since(s.startedAt)
	}

	return Stats{
		TotalScheduled:   s.totalScheduled,
		TotalExecuted:    s.totalExecuted,
		TotalCanceled:    s.totalCancelled,
		CurrentScheduled: len(s.tasks),
		UpTime:           uptime,
		StartedAt:        s.startedAt,
	}
}

// run is the main scheduler loop that checks for ready tasks and executes them.
func (s *scheduler) run() {
	for {
		select {
		case <-s.stopCh:
			return
		case <-s.ticker.C:
			s.checkAndExecuteTasks()
		}
	}
}

// checkAndExecuteTasks checks for tasks that are ready to run and executes them.
func (s *scheduler) checkAndExecuteTasks() {
	now := time.Now()
	var tasksToExecute []*ScheduledTask

	s.mu.Lock()
	for _, task := range s.tasks {
		if now.After(task.RunAt) || now.Equal(task.RunAt) {
			tasksToExecute = append(tasksToExecute, task)
		}
	}
	s.mu.Unlock()

	for _, task := range tasksToExecute {
		s.executeTask(task)
	}
}

// executeTask executes a scheduled task and handles rescheduling if necessary.
func (s *scheduler) executeTask(scheduledTask *ScheduledTask) {
	// Create result channel for this specific task
	resultCh := make(chan workerpool.Result, 1)

	// Create a wrapper task that sends result to our channel
	wrapperTask := workerpool.TaskFunc(func(ctx context.Context) error {
		if s.config.TaskTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, s.config.TaskTimeout)
			defer cancel()
		}

		err := scheduledTask.Task.Execute(ctx)

		// Send result to our specific channel
		select {
		case resultCh <- workerpool.Result{Task: scheduledTask.Task, Error: err, Duration: 0, WorkerID: -1}:
		default:
			// Channel full, ignore (shouldn't happen with buffer of 1)
		}

		return err
	})

	// Submit task to worker pool
	if err := s.pool.Submit(wrapperTask); err != nil {
		if s.config.OnError != nil {
			s.config.OnError(fmt.Errorf("failed to submit task %s: %w", scheduledTask.ID, err))
		}
		return
	}

	// Handle task execution results in background
	go func() {
		// Wait for our specific result
		select {
		case result := <-resultCh:
			s.handleTaskResult(scheduledTask, result)
		case <-s.stopCh:
			return
		}

		// Consume the worker pool result to prevent blocking
		select {
		case <-s.pool.Results():
			// Consume the corresponding worker pool result
		case <-s.stopCh:
			return
		case <-time.After(100 * time.Millisecond):
			// Timeout to avoid blocking indefinitely
		}
	}()
}

// handleTaskResult processes the result of a scheduled task execution.
func (s *scheduler) handleTaskResult(scheduledTask *ScheduledTask, result workerpool.Result) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Update run count
	scheduledTask.RunCount++
	s.totalExecuted++

	// Call result callback
	if s.config.OnTaskExecuted != nil {
		s.config.OnTaskExecuted(*scheduledTask, result)
	}

	// Determine if task should be rescheduled
	shouldReschedule := scheduledTask.IsRepeating &&
		(scheduledTask.MaxRuns <= 0 || scheduledTask.RunCount < scheduledTask.MaxRuns)

	if shouldReschedule {
		// Schedule next run
		scheduledTask.RunAt = time.Now().Add(scheduledTask.Interval)
	} else {
		// Remove completed task
		delete(s.tasks, scheduledTask.ID)
	}
}
