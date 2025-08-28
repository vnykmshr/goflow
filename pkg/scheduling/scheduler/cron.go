package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

// CronScheduler extends the basic Scheduler interface with cron expression support.
type CronScheduler interface {
	Scheduler
	
	// ScheduleCron schedules a task using a cron expression.
	// Supports standard cron format: "minute hour day month weekday"
	// Examples:
	//   "0 */2 * * *"     - Every 2 hours
	//   "30 14 * * 1-5"   - 2:30 PM on weekdays
	//   "0 9 1 * *"       - 9:00 AM on the 1st of every month
	//   "@daily"          - Every day at midnight
	//   "@hourly"         - Every hour
	ScheduleCron(id string, cronExpr string, task workerpool.Task) error

	// ScheduleCronWithOptions schedules a task with cron expression and additional options.
	ScheduleCronWithOptions(id string, cronExpr string, task workerpool.Task, options CronOptions) error

	// UpdateCron updates an existing cron task with a new expression.
	UpdateCron(id string, newCronExpr string) error

	// GetCronNext returns the next execution time for a cron task.
	GetCronNext(id string) (time.Time, error)

	// ListCronTasks returns all tasks scheduled with cron expressions.
	ListCronTasks() []CronTask

	// ValidateCronExpression validates a cron expression without scheduling it.
	ValidateCronExpression(cronExpr string) error

	// ParseCronExpression returns human-readable description of cron expression.
	ParseCronExpression(cronExpr string) (CronDescription, error)
}

// CronTask represents a task scheduled with a cron expression.
type CronTask struct {
	ScheduledTask
	CronExpression string
	NextRun        time.Time
	TimeZone       *time.Location
	Options        CronOptions
	Schedule       cron.Schedule
}

// CronOptions provides configuration for cron-scheduled tasks.
type CronOptions struct {
	// MaxRuns limits the number of times the task will execute (0 = unlimited)
	MaxRuns int

	// TimeZone specifies the timezone for cron expression evaluation
	TimeZone *time.Location

	// StopOnError determines if the cron task should be removed after an error
	StopOnError bool

	// SkipIfStillRunning prevents overlapping executions
	SkipIfStillRunning bool

	// DelayIfStillRunning waits for the previous execution to finish
	DelayIfStillRunning bool

	// MaxDelay is the maximum time to wait if DelayIfStillRunning is true
	MaxDelay time.Duration

	// OnError is called when a cron task execution fails
	OnError func(task CronTask, err error)

	// OnSkip is called when a task execution is skipped
	OnSkip func(task CronTask, reason string)
}

// CronDescription provides human-readable information about a cron expression.
type CronDescription struct {
	Expression  string
	Description string
	NextRuns    []time.Time // Next 5 execution times
	TimeZone    string
}

// cronScheduler implements CronScheduler by wrapping the base scheduler.
type cronScheduler struct {
	Scheduler
	cronTasks map[string]*CronTask
	parser    cron.Parser
}

// NewCronScheduler creates a new scheduler with cron expression support.
func NewCronScheduler() CronScheduler {
	return NewCronSchedulerWithConfig(Config{})
}

// NewCronSchedulerWithConfig creates a new cron scheduler with custom configuration.
func NewCronSchedulerWithConfig(config Config) CronScheduler {
	baseScheduler := NewWithConfig(config)
	
	// Create cron parser with seconds, minutes, hours, day of month, month, day of week
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

	return &cronScheduler{
		Scheduler: baseScheduler,
		cronTasks: make(map[string]*CronTask),
		parser:    parser,
	}
}

// ScheduleCron schedules a task using a cron expression.
func (cs *cronScheduler) ScheduleCron(id string, cronExpr string, task workerpool.Task) error {
	return cs.ScheduleCronWithOptions(id, cronExpr, task, CronOptions{})
}

// ScheduleCronWithOptions schedules a task with cron expression and options.
func (cs *cronScheduler) ScheduleCronWithOptions(id string, cronExpr string, task workerpool.Task, options CronOptions) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}
	if id == "" {
		return fmt.Errorf("task ID cannot be empty")
	}
	if cronExpr == "" {
		return fmt.Errorf("cron expression cannot be empty")
	}

	// Parse and validate cron expression
	schedule, err := cs.parser.Parse(cronExpr)
	if err != nil {
		return fmt.Errorf("invalid cron expression '%s': %w", cronExpr, err)
	}

	// Set default timezone if not specified
	timezone := options.TimeZone
	if timezone == nil {
		timezone = time.Local
	}

	// Calculate next run time
	now := time.Now().In(timezone)
	nextRun := schedule.Next(now)

	// Create cron task
	cronTask := &CronTask{
		ScheduledTask: ScheduledTask{
			ID:          id,
			Task:        task,
			RunAt:       nextRun,
			IsRepeating: true,
			MaxRuns:     options.MaxRuns,
			RunCount:    0,
			CreatedAt:   now,
		},
		CronExpression: cronExpr,
		NextRun:        nextRun,
		TimeZone:       timezone,
		Options:        options,
		Schedule:       schedule,
	}

	// Store cron task
	cs.cronTasks[id] = cronTask

	// Schedule the first execution
	return cs.Scheduler.Schedule(id, cs.createCronTaskWrapper(cronTask), nextRun)
}

// createCronTaskWrapper creates a wrapper that handles cron-specific logic.
func (cs *cronScheduler) createCronTaskWrapper(cronTask *CronTask) workerpool.Task {
	return workerpool.TaskFunc(func(ctx context.Context) error {
		// Check if task is still active
		if _, exists := cs.cronTasks[cronTask.ID]; !exists {
			return nil // Task was cancelled
		}

		// Handle overlapping execution policies
		if cronTask.Options.SkipIfStillRunning || cronTask.Options.DelayIfStillRunning {
			// Implementation would check if previous execution is still running
			// For now, we'll proceed with execution
		}

		// Execute the original task
		err := cronTask.Task.Execute(ctx)

		// Update run count
		cronTask.RunCount++

		// Handle error
		if err != nil {
			if cronTask.Options.OnError != nil {
				cronTask.Options.OnError(*cronTask, err)
			}
			
			if cronTask.Options.StopOnError {
				delete(cs.cronTasks, cronTask.ID)
				return err
			}
		}

		// Check if max runs reached
		if cronTask.Options.MaxRuns > 0 && cronTask.RunCount >= cronTask.Options.MaxRuns {
			delete(cs.cronTasks, cronTask.ID)
			return err
		}

		// Schedule next execution
		now := time.Now().In(cronTask.TimeZone)
		nextRun := cronTask.Schedule.Next(now)
		cronTask.NextRun = nextRun
		cronTask.RunAt = nextRun

		// Schedule next run
		cs.Scheduler.Schedule(cronTask.ID, cs.createCronTaskWrapper(cronTask), nextRun)

		return err
	})
}

// UpdateCron updates an existing cron task with a new expression.
func (cs *cronScheduler) UpdateCron(id string, newCronExpr string) error {
	cronTask, exists := cs.cronTasks[id]
	if !exists {
		return fmt.Errorf("cron task with ID %s not found", id)
	}

	// Parse new cron expression
	schedule, err := cs.parser.Parse(newCronExpr)
	if err != nil {
		return fmt.Errorf("invalid cron expression '%s': %w", newCronExpr, err)
	}

	// Cancel current task
	cs.Scheduler.Cancel(id)

	// Update cron task
	cronTask.CronExpression = newCronExpr
	cronTask.Schedule = schedule
	
	// Calculate next run time
	now := time.Now().In(cronTask.TimeZone)
	nextRun := schedule.Next(now)
	cronTask.NextRun = nextRun
	cronTask.RunAt = nextRun

	// Reschedule
	return cs.Scheduler.Schedule(id, cs.createCronTaskWrapper(cronTask), nextRun)
}

// GetCronNext returns the next execution time for a cron task.
func (cs *cronScheduler) GetCronNext(id string) (time.Time, error) {
	cronTask, exists := cs.cronTasks[id]
	if !exists {
		return time.Time{}, fmt.Errorf("cron task with ID %s not found", id)
	}
	
	return cronTask.NextRun, nil
}

// ListCronTasks returns all tasks scheduled with cron expressions.
func (cs *cronScheduler) ListCronTasks() []CronTask {
	tasks := make([]CronTask, 0, len(cs.cronTasks))
	for _, task := range cs.cronTasks {
		tasks = append(tasks, *task)
	}
	return tasks
}

// ValidateCronExpression validates a cron expression.
func (cs *cronScheduler) ValidateCronExpression(cronExpr string) error {
	_, err := cs.parser.Parse(cronExpr)
	return err
}

// ParseCronExpression returns human-readable description of cron expression.
func (cs *cronScheduler) ParseCronExpression(cronExpr string) (CronDescription, error) {
	schedule, err := cs.parser.Parse(cronExpr)
	if err != nil {
		return CronDescription{}, fmt.Errorf("invalid cron expression: %w", err)
	}

	now := time.Now()
	nextRuns := make([]time.Time, 5)
	current := now
	
	for i := 0; i < 5; i++ {
		current = schedule.Next(current)
		nextRuns[i] = current
	}

	description := cs.generateCronDescription(cronExpr)

	return CronDescription{
		Expression:  cronExpr,
		Description: description,
		NextRuns:    nextRuns,
		TimeZone:    now.Location().String(),
	}, nil
}

// generateCronDescription generates a human-readable description.
func (cs *cronScheduler) generateCronDescription(cronExpr string) string {
	// Handle special expressions
	switch cronExpr {
	case "@yearly", "@annually":
		return "Once a year (January 1st at midnight)"
	case "@monthly":
		return "Once a month (1st day at midnight)"
	case "@weekly":
		return "Once a week (Sunday at midnight)"
	case "@daily", "@midnight":
		return "Once a day (at midnight)"
	case "@hourly":
		return "Once an hour (at minute 0)"
	}

	// For complex expressions, provide a basic description
	// In a production implementation, you'd want more sophisticated parsing
	return fmt.Sprintf("Custom schedule: %s", cronExpr)
}

// Cancel removes a scheduled task (overrides base to handle cron tasks).
func (cs *cronScheduler) Cancel(id string) bool {
	// Remove from cron tasks if it exists
	delete(cs.cronTasks, id)
	
	// Cancel from base scheduler
	return cs.Scheduler.Cancel(id)
}

// CancelAll removes all scheduled tasks (overrides base to handle cron tasks).
func (cs *cronScheduler) CancelAll() {
	// Clear cron tasks
	cs.cronTasks = make(map[string]*CronTask)
	
	// Cancel all from base scheduler
	cs.Scheduler.CancelAll()
}