package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

// ContextAwareScheduler extends scheduling with sophisticated context handling.
type ContextAwareScheduler interface {
	Scheduler
	
	// ScheduleWithContext schedules a task with a specific context
	ScheduleWithContext(ctx context.Context, id string, task workerpool.Task, runAt time.Time) error
	
	// ScheduleWithTimeout schedules a task with a timeout context
	ScheduleWithTimeout(id string, task workerpool.Task, runAt time.Time, timeout time.Duration) error
	
	// ScheduleWithDeadline schedules a task with a deadline context
	ScheduleWithDeadline(id string, task workerpool.Task, runAt time.Time, deadline time.Time) error
	
	// ScheduleWithCancellation schedules a task that can be cancelled via context
	ScheduleWithCancellation(id string, task workerpool.Task, runAt time.Time) (context.CancelFunc, error)
	
	// ScheduleRepeatingWithContext schedules a repeating task with context management
	ScheduleRepeatingWithContext(ctx context.Context, id string, task workerpool.Task, interval time.Duration, maxRuns int) error
	
	// SetGlobalTimeout sets a default timeout for all scheduled tasks
	SetGlobalTimeout(timeout time.Duration)
	
	// SetContextPropagation enables context value propagation to scheduled tasks
	SetContextPropagation(enabled bool, keys []interface{})
	
	// GetTaskContext returns the context associated with a running task
	GetTaskContext(taskID string) (context.Context, bool)
	
	// CancelTaskContext cancels a specific task's context
	CancelTaskContext(taskID string) bool
	
	// GetContextStats returns statistics about context usage
	GetContextStats() ContextStats
}

// ContextStats provides metrics about context usage in the scheduler.
type ContextStats struct {
	ActiveContexts       int
	CancelledTasks       int64
	TimeoutTasks         int64
	ContextPropagations  int64
	AverageTaskDuration  time.Duration
	LongestRunningTask   time.Duration
}

// TaskContext holds context information for a scheduled task.
type TaskContext struct {
	ID            string
	Context       context.Context
	CancelFunc    context.CancelFunc
	StartTime     time.Time
	Timeout       time.Duration
	IsCancelled   bool
	PropagatedKeys []interface{}
}

// contextAwareScheduler implements ContextAwareScheduler.
type contextAwareScheduler struct {
	Scheduler
	taskContexts      map[string]*TaskContext
	globalTimeout     time.Duration
	propagationEnabled bool
	propagationKeys   []interface{}
	contextStats      ContextStats
	mu               sync.RWMutex
}

// NewContextAwareScheduler creates a new context-aware scheduler.
func NewContextAwareScheduler() ContextAwareScheduler {
	return NewContextAwareSchedulerWithConfig(Config{})
}

// NewContextAwareSchedulerWithConfig creates a context-aware scheduler with custom configuration.
func NewContextAwareSchedulerWithConfig(config Config) ContextAwareScheduler {
	baseScheduler := NewWithConfig(config)
	
	return &contextAwareScheduler{
		Scheduler:     baseScheduler,
		taskContexts:  make(map[string]*TaskContext),
		globalTimeout: 30 * time.Second, // Default global timeout
	}
}

// ScheduleWithContext schedules a task with a specific context.
func (cas *contextAwareScheduler) ScheduleWithContext(ctx context.Context, id string, task workerpool.Task, runAt time.Time) error {
	// Create task context
	taskCtx, cancel := context.WithCancel(ctx)
	
	// Store context information
	cas.mu.Lock()
	cas.taskContexts[id] = &TaskContext{
		ID:         id,
		Context:    taskCtx,
		CancelFunc: cancel,
		StartTime:  time.Now(),
	}
	cas.mu.Unlock()
	
	// Wrap task with context handling
	wrappedTask := cas.wrapTaskWithContext(id, task, taskCtx)
	
	return cas.Scheduler.Schedule(id, wrappedTask, runAt)
}

// ScheduleWithTimeout schedules a task with a timeout context.
func (cas *contextAwareScheduler) ScheduleWithTimeout(id string, task workerpool.Task, runAt time.Time, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	
	// Store context with timeout information
	cas.mu.Lock()
	cas.taskContexts[id] = &TaskContext{
		ID:         id,
		Context:    ctx,
		CancelFunc: cancel,
		StartTime:  time.Now(),
		Timeout:    timeout,
	}
	cas.mu.Unlock()
	
	wrappedTask := cas.wrapTaskWithContext(id, task, ctx)
	return cas.Scheduler.Schedule(id, wrappedTask, runAt)
}

// ScheduleWithDeadline schedules a task with a deadline context.
func (cas *contextAwareScheduler) ScheduleWithDeadline(id string, task workerpool.Task, runAt time.Time, deadline time.Time) error {
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	
	cas.mu.Lock()
	cas.taskContexts[id] = &TaskContext{
		ID:         id,
		Context:    ctx,
		CancelFunc: cancel,
		StartTime:  time.Now(),
		Timeout:    time.Until(deadline),
	}
	cas.mu.Unlock()
	
	wrappedTask := cas.wrapTaskWithContext(id, task, ctx)
	return cas.Scheduler.Schedule(id, wrappedTask, runAt)
}

// ScheduleWithCancellation schedules a task that can be cancelled via context.
func (cas *contextAwareScheduler) ScheduleWithCancellation(id string, task workerpool.Task, runAt time.Time) (context.CancelFunc, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	cas.mu.Lock()
	cas.taskContexts[id] = &TaskContext{
		ID:         id,
		Context:    ctx,
		CancelFunc: cancel,
		StartTime:  time.Now(),
	}
	cas.mu.Unlock()
	
	wrappedTask := cas.wrapTaskWithContext(id, task, ctx)
	err := cas.Scheduler.Schedule(id, wrappedTask, runAt)
	
	return cancel, err
}

// ScheduleRepeatingWithContext schedules a repeating task with context management.
func (cas *contextAwareScheduler) ScheduleRepeatingWithContext(ctx context.Context, id string, task workerpool.Task, interval time.Duration, maxRuns int) error {
	taskCtx, cancel := context.WithCancel(ctx)
	
	cas.mu.Lock()
	cas.taskContexts[id] = &TaskContext{
		ID:         id,
		Context:    taskCtx,
		CancelFunc: cancel,
		StartTime:  time.Now(),
	}
	cas.mu.Unlock()
	
	// Create repeating wrapper that respects context
	repeatingTask := workerpool.TaskFunc(func(execCtx context.Context) error {
		// Check if parent context is cancelled
		select {
		case <-taskCtx.Done():
			return taskCtx.Err()
		default:
		}
		
		// Execute the task with merged context
		mergedCtx := cas.mergeContexts(execCtx, taskCtx)
		return task.Execute(mergedCtx)
	})
	
	return cas.Scheduler.ScheduleRepeating(id, repeatingTask, interval, maxRuns)
}

// SetGlobalTimeout sets a default timeout for all scheduled tasks.
func (cas *contextAwareScheduler) SetGlobalTimeout(timeout time.Duration) {
	cas.mu.Lock()
	cas.globalTimeout = timeout
	cas.mu.Unlock()
}

// SetContextPropagation enables context value propagation to scheduled tasks.
func (cas *contextAwareScheduler) SetContextPropagation(enabled bool, keys []interface{}) {
	cas.mu.Lock()
	cas.propagationEnabled = enabled
	cas.propagationKeys = keys
	cas.mu.Unlock()
}

// GetTaskContext returns the context associated with a running task.
func (cas *contextAwareScheduler) GetTaskContext(taskID string) (context.Context, bool) {
	cas.mu.RLock()
	taskCtx, exists := cas.taskContexts[taskID]
	cas.mu.RUnlock()
	
	if !exists {
		return nil, false
	}
	
	return taskCtx.Context, true
}

// CancelTaskContext cancels a specific task's context.
func (cas *contextAwareScheduler) CancelTaskContext(taskID string) bool {
	cas.mu.Lock()
	taskCtx, exists := cas.taskContexts[taskID]
	if exists {
		taskCtx.CancelFunc()
		taskCtx.IsCancelled = true
		cas.contextStats.CancelledTasks++
	}
	cas.mu.Unlock()
	
	return exists
}

// GetContextStats returns statistics about context usage.
func (cas *contextAwareScheduler) GetContextStats() ContextStats {
	cas.mu.RLock()
	defer cas.mu.RUnlock()
	
	stats := cas.contextStats
	stats.ActiveContexts = len(cas.taskContexts)
	
	// Calculate duration statistics
	var totalDuration time.Duration
	var longestDuration time.Duration
	activeCount := 0
	
	for _, taskCtx := range cas.taskContexts {
		if !taskCtx.IsCancelled {
			duration := time.Since(taskCtx.StartTime)
			totalDuration += duration
			activeCount++
			
			if duration > longestDuration {
				longestDuration = duration
			}
		}
	}
	
	if activeCount > 0 {
		stats.AverageTaskDuration = totalDuration / time.Duration(activeCount)
	}
	stats.LongestRunningTask = longestDuration
	
	return stats
}

// wrapTaskWithContext wraps a task with context handling and propagation.
func (cas *contextAwareScheduler) wrapTaskWithContext(id string, task workerpool.Task, ctx context.Context) workerpool.Task {
	return workerpool.TaskFunc(func(execCtx context.Context) error {
		start := time.Now()
		
		// Apply global timeout if no specific timeout is set
		finalCtx := ctx
		if cas.globalTimeout > 0 {
			if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > cas.globalTimeout {
				var cancel context.CancelFunc
				finalCtx, cancel = context.WithTimeout(ctx, cas.globalTimeout)
				defer cancel()
			}
		}
		
		// Propagate context values if enabled
		if cas.propagationEnabled {
			finalCtx = cas.propagateContextValues(execCtx, finalCtx)
		}
		
		// Execute task with final context
		err := task.Execute(finalCtx)
		
		// Update statistics
		cas.updateTaskStats(id, start, err)
		
		// Clean up context if task completed
		if err != nil || !cas.isRepeatingTask(id) {
			cas.cleanupTaskContext(id)
		}
		
		return err
	})
}

// mergeContexts merges execution context with task context.
func (cas *contextAwareScheduler) mergeContexts(execCtx, taskCtx context.Context) context.Context {
	// If task context is cancelled, return it immediately
	select {
	case <-taskCtx.Done():
		return taskCtx
	default:
	}
	
	// Create a context that cancels when either parent is cancelled
	merged, cancel := context.WithCancel(execCtx)
	
	go func() {
		defer cancel()
		select {
		case <-execCtx.Done():
		case <-taskCtx.Done():
		}
	}()
	
	return merged
}

// propagateContextValues propagates selected values from execution context to task context.
func (cas *contextAwareScheduler) propagateContextValues(from, to context.Context) context.Context {
	cas.mu.RLock()
	keys := cas.propagationKeys
	cas.mu.RUnlock()
	
	result := to
	for _, key := range keys {
		if value := from.Value(key); value != nil {
			result = context.WithValue(result, key, value)
			
			cas.mu.Lock()
			cas.contextStats.ContextPropagations++
			cas.mu.Unlock()
		}
	}
	
	return result
}

// updateTaskStats updates context-related statistics for a task.
func (cas *contextAwareScheduler) updateTaskStats(taskID string, start time.Time, err error) {
	cas.mu.Lock()
	defer cas.mu.Unlock()
	
	if taskCtx, exists := cas.taskContexts[taskID]; exists {
		duration := time.Since(start)
		
		// Check for timeout errors
		if err == context.DeadlineExceeded {
			cas.contextStats.TimeoutTasks++
		}
		
		// Update task duration
		if cas.contextStats.AverageTaskDuration == 0 {
			cas.contextStats.AverageTaskDuration = duration
		} else {
			cas.contextStats.AverageTaskDuration = (cas.contextStats.AverageTaskDuration + duration) / 2
		}
		
		// Update longest running task
		totalDuration := time.Since(taskCtx.StartTime)
		if totalDuration > cas.contextStats.LongestRunningTask {
			cas.contextStats.LongestRunningTask = totalDuration
		}
	}
}

// isRepeatingTask checks if a task is configured as repeating.
func (cas *contextAwareScheduler) isRepeatingTask(taskID string) bool {
	// Check if task exists in base scheduler and is repeating
	if scheduledTask, exists := cas.Scheduler.GetTask(taskID); exists {
		return scheduledTask.IsRepeating
	}
	return false
}

// cleanupTaskContext removes context information for completed tasks.
func (cas *contextAwareScheduler) cleanupTaskContext(taskID string) {
	cas.mu.Lock()
	if taskCtx, exists := cas.taskContexts[taskID]; exists {
		taskCtx.CancelFunc()
		delete(cas.taskContexts, taskID)
	}
	cas.mu.Unlock()
}

// Schedule overrides base scheduler to apply global timeout and context management.
func (cas *contextAwareScheduler) Schedule(id string, task workerpool.Task, runAt time.Time) error {
	if cas.globalTimeout > 0 {
		return cas.ScheduleWithTimeout(id, task, runAt, cas.globalTimeout)
	}
	_, err := cas.ScheduleWithCancellation(id, task, runAt)
	return err
}

// ScheduleRepeating overrides base scheduler to apply context management.
func (cas *contextAwareScheduler) ScheduleRepeating(id string, task workerpool.Task, interval time.Duration, maxRuns int) error {
	return cas.ScheduleRepeatingWithContext(context.Background(), id, task, interval, maxRuns)
}

// Stop overrides base scheduler to clean up all contexts.
func (cas *contextAwareScheduler) Stop() <-chan struct{} {
	// Cancel all active task contexts
	cas.mu.Lock()
	for _, taskCtx := range cas.taskContexts {
		taskCtx.CancelFunc()
		taskCtx.IsCancelled = true
	}
	cas.taskContexts = make(map[string]*TaskContext)
	cas.mu.Unlock()
	
	return cas.Scheduler.Stop()
}

// WithContextPropagation is a helper to create context-aware scheduler with value propagation.
func WithContextPropagation(config Config, keys ...interface{}) ContextAwareScheduler {
	scheduler := NewContextAwareSchedulerWithConfig(config)
	scheduler.SetContextPropagation(true, keys)
	return scheduler
}

// WithGlobalTimeout is a helper to create context-aware scheduler with global timeout.
func WithGlobalTimeout(config Config, timeout time.Duration) ContextAwareScheduler {
	scheduler := NewContextAwareSchedulerWithConfig(config)
	scheduler.SetGlobalTimeout(timeout)
	return scheduler
}