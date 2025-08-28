package scheduler

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

// AdvancedScheduler extends CronScheduler with sophisticated scheduling patterns.
type AdvancedScheduler interface {
	CronScheduler

	// ScheduleWithBackoff schedules a task with exponential backoff on failure.
	ScheduleWithBackoff(id string, task workerpool.Task, config BackoffConfig) error

	// ScheduleConditional schedules a task that only runs when a condition is met.
	ScheduleConditional(id string, task workerpool.Task, condition ConditionFunc, interval time.Duration) error

	// ScheduleChain creates a chain of tasks that execute in sequence.
	ScheduleChain(id string, tasks []ChainedTask) error

	// ScheduleBatch schedules multiple tasks to run in parallel at specific times.
	ScheduleBatch(id string, batchConfig BatchConfig) error

	// ScheduleThrottled schedules a task with rate limiting to prevent overwhelming resources.
	ScheduleThrottled(id string, task workerpool.Task, throttleConfig ThrottleConfig) error

	// ScheduleAdaptive schedules a task with adaptive timing based on system load.
	ScheduleAdaptive(id string, task workerpool.Task, adaptiveConfig AdaptiveConfig) error

	// ScheduleWindow schedules a task to run only within specific time windows.
	ScheduleWindow(id string, task workerpool.Task, windows []TimeWindow) error

	// ScheduleJittered schedules a task with random jitter to prevent thundering herd.
	ScheduleJittered(id string, task workerpool.Task, baseInterval time.Duration, jitter JitterConfig) error
}

// BackoffConfig configures exponential backoff for failed task executions.
type BackoffConfig struct {
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	Multiplier    float64
	MaxRetries    int
	ResetAfter    time.Duration // Reset backoff after successful execution
	OnMaxRetries  func(task ScheduledTask, attempts int)
}

// ConditionFunc is a function that determines if a task should execute.
type ConditionFunc func() bool

// ChainedTask represents a task in a chain of sequential tasks.
type ChainedTask struct {
	Name     string
	Task     workerpool.Task
	Timeout  time.Duration
	OnError  func(err error) ChainAction
	OnSuccess func() ChainAction
}

// ChainAction determines what happens after a chained task completes.
type ChainAction int

const (
	ChainContinue ChainAction = iota // Continue to next task
	ChainStop                        // Stop the chain
	ChainRetry                       // Retry current task
	ChainRestart                     // Restart from first task
)

// BatchConfig configures batch task execution.
type BatchConfig struct {
	Tasks       []workerpool.Task
	RunAt       time.Time
	MaxParallel int           // Max concurrent tasks (0 = unlimited)
	Timeout     time.Duration // Timeout for entire batch
	OnComplete  func(results []workerpool.Result)
}

// ThrottleConfig configures task throttling.
type ThrottleConfig struct {
	MaxConcurrent int           // Maximum concurrent executions
	RateLimit     time.Duration // Minimum time between starts
	QueueSize     int           // Max queued executions
	OnThrottled   func(task ScheduledTask)
}

// AdaptiveConfig configures adaptive task scheduling based on system metrics.
type AdaptiveConfig struct {
	BaseInterval    time.Duration
	MinInterval     time.Duration
	MaxInterval     time.Duration
	LoadThreshold   float64 // CPU/memory threshold (0.0-1.0)
	AdaptationRate  float64 // How quickly to adapt (0.0-1.0)
	LoadMetricFunc  func() float64 // Function to get current system load
}

// TimeWindow defines a time period when tasks can execute.
type TimeWindow struct {
	Start    time.Time
	End      time.Time
	TimeZone *time.Location
	Days     []time.Weekday // Days of week when window applies
}

// JitterConfig configures random jitter for task scheduling.
type JitterConfig struct {
	Type      JitterType
	Amount    time.Duration // Max jitter amount
	MinAmount time.Duration // Min jitter amount (for bounded jitter)
}

// JitterType defines the type of jitter to apply.
type JitterType int

const (
	JitterUniform JitterType = iota // Uniform distribution
	JitterNormal                    // Normal/Gaussian distribution
	JitterExponential               // Exponential distribution
)

// advancedScheduler implements AdvancedScheduler.
type advancedScheduler struct {
	CronScheduler
	backoffTasks     map[string]*BackoffState
	conditionalTasks map[string]*ConditionalTask
	chainTasks       map[string]*ChainState
	adaptiveTasks    map[string]*AdaptiveState
	mu               sync.RWMutex // Protects all task maps
}

// BackoffState tracks exponential backoff state for a task.
type BackoffState struct {
	Config        BackoffConfig
	Attempts      int
	CurrentDelay  time.Duration
	LastExecution time.Time
	LastSuccess   time.Time
}

// ConditionalTask tracks a conditionally executed task.
type ConditionalTask struct {
	Task      workerpool.Task
	Condition ConditionFunc
	Interval  time.Duration
	LastCheck time.Time
}

// ChainState tracks the state of a task chain.
type ChainState struct {
	Tasks        []ChainedTask
	CurrentIndex int
	StartTime    time.Time
}

// AdaptiveState tracks adaptive scheduling state.
type AdaptiveState struct {
	Config          AdaptiveConfig
	CurrentInterval time.Duration
	LoadHistory     []float64
	LastAdaptation  time.Time
}

// NewAdvancedScheduler creates a new advanced scheduler with sophisticated patterns.
func NewAdvancedScheduler() AdvancedScheduler {
	return NewAdvancedSchedulerWithConfig(Config{})
}

// NewAdvancedSchedulerWithConfig creates an advanced scheduler with custom configuration.
func NewAdvancedSchedulerWithConfig(config Config) AdvancedScheduler {
	cronScheduler := NewCronSchedulerWithConfig(config)
	
	return &advancedScheduler{
		CronScheduler:    cronScheduler,
		backoffTasks:     make(map[string]*BackoffState),
		conditionalTasks: make(map[string]*ConditionalTask),
		chainTasks:       make(map[string]*ChainState),
		adaptiveTasks:    make(map[string]*AdaptiveState),
	}
}

// ScheduleWithBackoff schedules a task with exponential backoff on failure.
func (as *advancedScheduler) ScheduleWithBackoff(id string, task workerpool.Task, config BackoffConfig) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	// Initialize backoff state
	backoffState := &BackoffState{
		Config:       config,
		CurrentDelay: config.InitialDelay,
	}
	as.mu.Lock()
	as.backoffTasks[id] = backoffState
	as.mu.Unlock()

	// Schedule initial execution
	return as.Schedule(id, as.createBackoffWrapper(id, task, config), time.Now().Add(config.InitialDelay))
}

// createBackoffWrapper creates a wrapper task with backoff logic.
func (as *advancedScheduler) createBackoffWrapper(id string, task workerpool.Task, config BackoffConfig) workerpool.Task {
	return workerpool.TaskFunc(func(ctx context.Context) error {
		as.mu.RLock()
		state := as.backoffTasks[id]
		as.mu.RUnlock()
		if state == nil {
			return fmt.Errorf("backoff state not found for task %s", id)
		}

		state.LastExecution = time.Now()
		err := task.Execute(ctx)

		if err != nil {
			state.Attempts++
			
			// Check max retries
			if config.MaxRetries > 0 && state.Attempts >= config.MaxRetries {
				if config.OnMaxRetries != nil {
					config.OnMaxRetries(ScheduledTask{ID: id, Task: task}, state.Attempts)
				}
				as.mu.Lock()
				delete(as.backoffTasks, id)
				as.mu.Unlock()
				return fmt.Errorf("max retries exceeded: %w", err)
			}

			// Calculate next delay with exponential backoff
			nextDelay := time.Duration(float64(state.CurrentDelay) * config.Multiplier)
			if nextDelay > config.MaxDelay {
				nextDelay = config.MaxDelay
			}
			state.CurrentDelay = nextDelay

			// Schedule retry with unique ID
			retryID := fmt.Sprintf("%s-retry-%d", id, state.Attempts)
			as.ScheduleAfter(retryID, as.createBackoffWrapper(id, task, config), nextDelay)
			return err
		}

		// Success - reset backoff
		state.LastSuccess = time.Now()
		state.Attempts = 0
		
		// Reset delay if enough time has passed
		if config.ResetAfter > 0 && time.Since(state.LastSuccess) > config.ResetAfter {
			state.CurrentDelay = config.InitialDelay
		}

		return nil
	})
}

// ScheduleConditional schedules a task that only runs when a condition is met.
func (as *advancedScheduler) ScheduleConditional(id string, task workerpool.Task, condition ConditionFunc, interval time.Duration) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}
	if condition == nil {
		return fmt.Errorf("condition function cannot be nil")
	}

	conditionalTask := &ConditionalTask{
		Task:      task,
		Condition: condition,
		Interval:  interval,
		LastCheck: time.Now(),
	}
	as.conditionalTasks[id] = conditionalTask

	// Schedule initial check
	return as.Schedule(id, as.createConditionalWrapper(id), time.Now().Add(interval))
}

// createConditionalWrapper creates a wrapper for conditional task execution.
func (as *advancedScheduler) createConditionalWrapper(id string) workerpool.Task {
	return workerpool.TaskFunc(func(ctx context.Context) error {
		condTask := as.conditionalTasks[id]
		if condTask == nil {
			return nil // Task was cancelled
		}

		condTask.LastCheck = time.Now()

		// Check condition
		if !condTask.Condition() {
			// Condition not met, reschedule check with unique ID
			checkID := fmt.Sprintf("%s-check-%d", id, time.Now().UnixNano())
			as.ScheduleAfter(checkID, as.createConditionalWrapper(id), condTask.Interval)
			return nil
		}

		// Condition met, execute task
		err := condTask.Task.Execute(ctx)
		
		// Reschedule next check with unique ID
		checkID := fmt.Sprintf("%s-check-%d", id, time.Now().UnixNano())
		as.ScheduleAfter(checkID, as.createConditionalWrapper(id), condTask.Interval)
		
		return err
	})
}

// ScheduleChain creates a chain of tasks that execute in sequence.
func (as *advancedScheduler) ScheduleChain(id string, tasks []ChainedTask) error {
	if len(tasks) == 0 {
		return fmt.Errorf("chain must contain at least one task")
	}

	chainState := &ChainState{
		Tasks:        tasks,
		CurrentIndex: 0,
		StartTime:    time.Now(),
	}
	as.chainTasks[id] = chainState

	// Start chain execution
	return as.Schedule(id, as.createChainWrapper(id), time.Now())
}

// createChainWrapper creates a wrapper for chain task execution.
func (as *advancedScheduler) createChainWrapper(id string) workerpool.Task {
	return workerpool.TaskFunc(func(ctx context.Context) error {
		return as.executeChainStep(id, ctx)
	})
}

// executeChainStep executes the next step in a task chain.
func (as *advancedScheduler) executeChainStep(chainID string, ctx context.Context) error {
	chain := as.chainTasks[chainID]
	if chain == nil {
		return fmt.Errorf("chain state not found for %s", chainID)
	}

	if chain.CurrentIndex >= len(chain.Tasks) {
		// Chain completed
		delete(as.chainTasks, chainID)
		return nil
	}

	currentTask := chain.Tasks[chain.CurrentIndex]
	
	// Execute current task with timeout if specified
	var err error
	if currentTask.Timeout > 0 {
		taskCtx, cancel := context.WithTimeout(ctx, currentTask.Timeout)
		defer cancel()
		err = currentTask.Task.Execute(taskCtx)
	} else {
		err = currentTask.Task.Execute(ctx)
	}

	// Determine next action
	var action ChainAction
	if err != nil && currentTask.OnError != nil {
		action = currentTask.OnError(err)
	} else if err == nil && currentTask.OnSuccess != nil {
		action = currentTask.OnSuccess()
	} else {
		action = ChainContinue
	}

	// Handle chain action
	switch action {
	case ChainContinue:
		chain.CurrentIndex++
	case ChainStop:
		delete(as.chainTasks, chainID)
		return err
	case ChainRetry:
		// Keep current index, will retry same task
	case ChainRestart:
		chain.CurrentIndex = 0
	}

	// Schedule next step with unique ID
	stepID := fmt.Sprintf("%s-step-%d", chainID, time.Now().UnixNano())
	as.Schedule(stepID, as.createChainWrapper(chainID), time.Now())
	return err
}

// ScheduleJittered schedules a task with random jitter to prevent thundering herd.
func (as *advancedScheduler) ScheduleJittered(id string, task workerpool.Task, baseInterval time.Duration, jitter JitterConfig) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	// Schedule initial execution with jitter
	initialJitter := as.calculateJitter(jitter)
	return as.Schedule(id, as.createJitteredWrapper(id, task, baseInterval, jitter), time.Now().Add(baseInterval+initialJitter))
}

// createJitteredWrapper creates a wrapper for jittered task execution.
func (as *advancedScheduler) createJitteredWrapper(id string, task workerpool.Task, baseInterval time.Duration, jitter JitterConfig) workerpool.Task {
	return workerpool.TaskFunc(func(ctx context.Context) error {
		// Execute task
		err := task.Execute(ctx)
		
		// Calculate jittered next interval
		jitterAmount := as.calculateJitter(jitter)
		nextInterval := baseInterval + jitterAmount
		
		// Schedule next execution with unique ID
		nextID := fmt.Sprintf("%s-next-%d", id, time.Now().UnixNano())
		as.ScheduleAfter(nextID, as.createJitteredWrapper(id, task, baseInterval, jitter), nextInterval)
		
		return err
	})
}

// calculateJitter calculates jitter amount based on configuration.
func (as *advancedScheduler) calculateJitter(config JitterConfig) time.Duration {
	// Simplified jitter calculation - in production you'd use proper random distributions
	switch config.Type {
	case JitterUniform:
		// Simple uniform jitter for demo
		return time.Duration(float64(config.Amount) * 0.5) // 0-50% of amount
	case JitterNormal:
		// Normal distribution approximation
		return time.Duration(float64(config.Amount) * 0.3) // ~30% of amount
	case JitterExponential:
		// Exponential distribution approximation  
		return time.Duration(float64(config.Amount) * 0.7) // ~70% of amount
	default:
		return 0
	}
}

// ScheduleBatch schedules multiple tasks to run in parallel at specific times.
func (as *advancedScheduler) ScheduleBatch(id string, batchConfig BatchConfig) error {
	if len(batchConfig.Tasks) == 0 {
		return fmt.Errorf("batch must contain at least one task")
	}
	if batchConfig.RunAt.IsZero() {
		return fmt.Errorf("batch run time cannot be zero")
	}

	batchTask := workerpool.TaskFunc(func(ctx context.Context) error {
		// Create context for the entire batch with timeout if specified
		batchCtx := ctx
		if batchConfig.Timeout > 0 {
			var cancel context.CancelFunc
			batchCtx, cancel = context.WithTimeout(ctx, batchConfig.Timeout)
			defer cancel()
		}

		// Determine parallelism
		maxParallel := batchConfig.MaxParallel
		if maxParallel <= 0 {
			maxParallel = len(batchConfig.Tasks)
		}

		// Create semaphore for controlling parallelism
		sem := make(chan struct{}, maxParallel)
		results := make([]workerpool.Result, len(batchConfig.Tasks))
		errors := make([]error, len(batchConfig.Tasks))

		// Execute tasks in parallel with controlled concurrency
		var wg sync.WaitGroup
		for i, task := range batchConfig.Tasks {
			wg.Add(1)
			go func(index int, t workerpool.Task) {
				defer wg.Done()
				
				// Acquire semaphore slot
				sem <- struct{}{}
				defer func() { <-sem }()

				start := time.Now()
				err := t.Execute(batchCtx)
				duration := time.Since(start)

				errors[index] = err
				results[index] = workerpool.Result{
					Task:     t,
					Error:    err,
					Duration: duration,
				}
			}(i, task)
		}

		// Wait for all tasks to complete
		wg.Wait()

		// Call completion callback if provided
		if batchConfig.OnComplete != nil {
			batchConfig.OnComplete(results)
		}

		// Return first error encountered, if any
		for _, err := range errors {
			if err != nil {
				return err
			}
		}

		return nil
	})

	// Schedule the batch execution
	return as.Schedule(id, batchTask, batchConfig.RunAt)
}

// ScheduleThrottled schedules a task with rate limiting to prevent overwhelming resources.
func (as *advancedScheduler) ScheduleThrottled(id string, task workerpool.Task, throttleConfig ThrottleConfig) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}
	if throttleConfig.RateLimit <= 0 {
		return fmt.Errorf("rate limit must be positive")
	}

	// Create throttling state
	throttleState := &struct {
		lastExecution time.Time
		concurrent    int32
		queue         chan struct{}
	}{
		queue: make(chan struct{}, throttleConfig.QueueSize),
	}

	wrappedTask := workerpool.TaskFunc(func(ctx context.Context) error {
		// Check concurrent limit
		if throttleConfig.MaxConcurrent > 0 {
			current := atomic.LoadInt32(&throttleState.concurrent)
			if int(current) >= throttleConfig.MaxConcurrent {
				if throttleConfig.OnThrottled != nil {
					throttleConfig.OnThrottled(ScheduledTask{ID: id, Task: task})
				}
				return fmt.Errorf("max concurrent executions reached")
			}
		}

		// Enforce rate limiting
		now := time.Now()
		if !throttleState.lastExecution.IsZero() {
			elapsed := now.Sub(throttleState.lastExecution)
			if elapsed < throttleConfig.RateLimit {
				waitTime := throttleConfig.RateLimit - elapsed
				select {
				case <-time.After(waitTime):
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}

		// Increment concurrent counter
		atomic.AddInt32(&throttleState.concurrent, 1)
		defer atomic.AddInt32(&throttleState.concurrent, -1)

		// Update last execution time
		throttleState.lastExecution = time.Now()

		// Execute task
		return task.Execute(ctx)
	})

	// Schedule initial execution
	return as.Schedule(id, wrappedTask, time.Now())
}

// ScheduleAdaptive schedules a task with adaptive timing based on system load.
func (as *advancedScheduler) ScheduleAdaptive(id string, task workerpool.Task, adaptiveConfig AdaptiveConfig) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}
	if adaptiveConfig.BaseInterval <= 0 {
		return fmt.Errorf("base interval must be positive")
	}
	if adaptiveConfig.LoadMetricFunc == nil {
		return fmt.Errorf("load metric function cannot be nil")
	}

	adaptiveState := &AdaptiveState{
		Config:          adaptiveConfig,
		CurrentInterval: adaptiveConfig.BaseInterval,
		LoadHistory:     make([]float64, 0, 10),
		LastAdaptation:  time.Now(),
	}
	as.adaptiveTasks[id] = adaptiveState

	// Schedule initial execution
	return as.Schedule(id, as.createAdaptiveWrapper(id, task), time.Now().Add(adaptiveConfig.BaseInterval))
}

// createAdaptiveWrapper creates a wrapper for adaptive task execution.
func (as *advancedScheduler) createAdaptiveWrapper(id string, task workerpool.Task) workerpool.Task {
	return workerpool.TaskFunc(func(ctx context.Context) error {
		state := as.adaptiveTasks[id]
		if state == nil {
			return fmt.Errorf("adaptive state not found for task %s", id)
		}

		// Measure current system load
		currentLoad := state.Config.LoadMetricFunc()
		
		// Update load history
		state.LoadHistory = append(state.LoadHistory, currentLoad)
		if len(state.LoadHistory) > 10 {
			state.LoadHistory = state.LoadHistory[1:]
		}

		// Calculate average load
		var avgLoad float64
		for _, load := range state.LoadHistory {
			avgLoad += load
		}
		avgLoad /= float64(len(state.LoadHistory))

		// Execute the task
		err := task.Execute(ctx)

		// Adapt interval based on system load
		if time.Since(state.LastAdaptation) > time.Minute {
			if avgLoad > state.Config.LoadThreshold {
				// High load - increase interval
				newInterval := time.Duration(float64(state.CurrentInterval) * (1.0 + state.Config.AdaptationRate))
				if newInterval > state.Config.MaxInterval {
					newInterval = state.Config.MaxInterval
				}
				state.CurrentInterval = newInterval
			} else {
				// Low load - decrease interval
				newInterval := time.Duration(float64(state.CurrentInterval) * (1.0 - state.Config.AdaptationRate))
				if newInterval < state.Config.MinInterval {
					newInterval = state.Config.MinInterval
				}
				state.CurrentInterval = newInterval
			}
			state.LastAdaptation = time.Now()
		}

		// Schedule next execution with adapted interval and unique ID
		nextID := fmt.Sprintf("%s-adaptive-%d", id, time.Now().UnixNano())
		as.ScheduleAfter(nextID, as.createAdaptiveWrapper(id, task), state.CurrentInterval)

		return err
	})
}

// ScheduleWindow schedules a task to run only within specific time windows.
func (as *advancedScheduler) ScheduleWindow(id string, task workerpool.Task, windows []TimeWindow) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}
	if len(windows) == 0 {
		return fmt.Errorf("at least one time window must be specified")
	}

	// Find initial window
	nextWindow := as.findNextWindow(time.Now(), windows)
	if nextWindow.IsZero() {
		return fmt.Errorf("no valid time windows found")
	}

	return as.Schedule(id, as.createWindowWrapper(id, task, windows), nextWindow)
}

// createWindowWrapper creates a wrapper for time window task execution.
func (as *advancedScheduler) createWindowWrapper(id string, task workerpool.Task, windows []TimeWindow) workerpool.Task {
	return workerpool.TaskFunc(func(ctx context.Context) error {
		now := time.Now()
		
		// Check if current time falls within any window
		inWindow := false
		for _, window := range windows {
			if as.isTimeInWindow(now, window) {
				inWindow = true
				break
			}
		}

		if !inWindow {
			// Not in window - find next valid window and reschedule with unique ID
			nextWindow := as.findNextWindow(now, windows)
			if !nextWindow.IsZero() {
				windowID := fmt.Sprintf("%s-window-%d", id, time.Now().UnixNano())
				as.Schedule(windowID, as.createWindowWrapper(id, task, windows), nextWindow)
			}
			return fmt.Errorf("task skipped - outside time window")
		}

		// In window - execute task
		err := task.Execute(ctx)
		
		// Schedule next check with unique ID
		nextCheck := now.Add(time.Minute) // Check every minute
		nextWindow := as.findNextWindow(nextCheck, windows)
		if !nextWindow.IsZero() {
			windowID := fmt.Sprintf("%s-window-%d", id, time.Now().UnixNano())
			as.Schedule(windowID, as.createWindowWrapper(id, task, windows), nextWindow)
		}

		return err
	})
}

// isTimeInWindow checks if a time falls within a time window.
func (as *advancedScheduler) isTimeInWindow(t time.Time, window TimeWindow) bool {
	// Convert to window timezone if specified
	if window.TimeZone != nil {
		t = t.In(window.TimeZone)
	}

	// Check day of week if specified
	if len(window.Days) > 0 {
		dayMatch := false
		for _, day := range window.Days {
			if t.Weekday() == day {
				dayMatch = true
				break
			}
		}
		if !dayMatch {
			return false
		}
	}

	// Check time range
	start := window.Start
	end := window.End
	
	// Handle same-day windows
	if start.Before(end) {
		return (t.After(start) || t.Equal(start)) && t.Before(end)
	}
	
	// Handle overnight windows (end is next day)
	return t.After(start) || t.Before(end)
}

// findNextWindow finds the next time when any window will be active.
func (as *advancedScheduler) findNextWindow(from time.Time, windows []TimeWindow) time.Time {
	var earliest time.Time
	
	for _, window := range windows {
		next := as.findNextWindowTime(from, window)
		if !next.IsZero() && (earliest.IsZero() || next.Before(earliest)) {
			earliest = next
		}
	}
	
	return earliest
}

// findNextWindowTime finds the next time when a specific window will be active.
func (as *advancedScheduler) findNextWindowTime(from time.Time, window TimeWindow) time.Time {
	// Convert to window timezone
	if window.TimeZone != nil {
		from = from.In(window.TimeZone)
	}

	// If no day restrictions, find next occurrence of start time
	if len(window.Days) == 0 {
		return as.nextTimeOccurrence(from, window.Start)
	}

	// Find next valid day and time combination
	for d := 0; d < 14; d++ { // Check up to 2 weeks ahead
		candidate := from.AddDate(0, 0, d)
		
		// Check if this day matches window requirements
		dayMatch := false
		for _, day := range window.Days {
			if candidate.Weekday() == day {
				dayMatch = true
				break
			}
		}
		
		if dayMatch {
			// Set time to window start
			startTime := time.Date(
				candidate.Year(), candidate.Month(), candidate.Day(),
				window.Start.Hour(), window.Start.Minute(), window.Start.Second(),
				0, candidate.Location())
			
			if startTime.After(from) {
				return startTime
			}
		}
	}
	
	return time.Time{} // No valid window found
}

// nextTimeOccurrence finds the next occurrence of a specific time.
func (as *advancedScheduler) nextTimeOccurrence(from, target time.Time) time.Time {
	// Same day occurrence
	today := time.Date(from.Year(), from.Month(), from.Day(),
		target.Hour(), target.Minute(), target.Second(), 0, from.Location())
	
	if today.After(from) {
		return today
	}
	
	// Next day occurrence
	return today.AddDate(0, 0, 1)
}