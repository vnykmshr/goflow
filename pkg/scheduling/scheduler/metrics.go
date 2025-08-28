package scheduler

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/vnykmshr/goflow/pkg/metrics"
	"github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

// MetricsScheduler wraps a Scheduler with Prometheus metrics collection.
type MetricsScheduler struct {
	scheduler Scheduler
	name      string
	registry  *metrics.Registry
	enabled   bool
}

// NewWithMetrics creates a new scheduler with metrics enabled.
func NewWithMetrics(name string) Scheduler {
	// Use a separate registry for each metrics-enabled component to avoid conflicts
	registry := prometheus.NewRegistry()
	config := metrics.Config{
		Enabled:  true,
		Registry: registry,
	}
	
	return NewWithConfigAndMetrics(Config{}, name, config)
}

// NewWithConfigAndMetrics creates a new scheduler with custom config and metrics.
func NewWithConfigAndMetrics(config Config, name string, metricsConfig metrics.Config) Scheduler {
	baseScheduler := NewWithConfig(config)
	
	if !metricsConfig.Enabled {
		return baseScheduler
	}

	registry := metrics.DefaultRegistry
	if metricsConfig.Registry != nil {
		// Create custom registry if provided
		registry = metrics.NewRegistry(metricsConfig.Registry)
	}

	// Wrap the config callbacks to add metrics
	originalConfig := config
	config.OnTaskScheduled = func(task ScheduledTask) {
		if metricsConfig.Enabled {
			registry.TasksScheduled.WithLabelValues(name).Inc()
		}
		if originalConfig.OnTaskScheduled != nil {
			originalConfig.OnTaskScheduled(task)
		}
	}

	config.OnTaskExecuted = func(task ScheduledTask, result workerpool.Result) {
		if metricsConfig.Enabled {
			registry.TasksExecuted.WithLabelValues(name).Inc()
			
			if result.Error != nil {
				registry.TasksFailed.WithLabelValues(name).Inc()
			} else {
				registry.TasksCompleted.WithLabelValues(name).Inc()
			}
			
			// Record execution duration
			registry.TaskExecutionDuration.WithLabelValues(name).Observe(result.Duration.Seconds())
		}
		if originalConfig.OnTaskExecuted != nil {
			originalConfig.OnTaskExecuted(task, result)
		}
	}

	// Create base scheduler with wrapped config
	instrumentedScheduler := NewWithConfig(config)

	return &MetricsScheduler{
		scheduler: instrumentedScheduler,
		name:      name,
		registry:  registry,
		enabled:   true,
	}
}

// Schedule adds a one-time task to be executed at the specified time.
func (ms *MetricsScheduler) Schedule(id string, task workerpool.Task, runAt time.Time) error {
	err := ms.scheduler.Schedule(id, task, runAt)
	return err
}

// ScheduleRepeating adds a repeating task with the given interval.
func (ms *MetricsScheduler) ScheduleRepeating(id string, task workerpool.Task, interval time.Duration, maxRuns int) error {
	err := ms.scheduler.ScheduleRepeating(id, task, interval, maxRuns)
	return err
}

// ScheduleAfter adds a one-time task to be executed after the specified delay.
func (ms *MetricsScheduler) ScheduleAfter(id string, task workerpool.Task, delay time.Duration) error {
	err := ms.scheduler.ScheduleAfter(id, task, delay)
	return err
}

// ScheduleEvery adds a repeating task that starts immediately and repeats at the given interval.
func (ms *MetricsScheduler) ScheduleEvery(id string, task workerpool.Task, interval time.Duration, maxRuns int) error {
	err := ms.scheduler.ScheduleEvery(id, task, interval, maxRuns)
	return err
}

// Cancel removes a scheduled task by ID.
func (ms *MetricsScheduler) Cancel(id string) bool {
	canceled := ms.scheduler.Cancel(id)
	return canceled
}

// CancelAll removes all scheduled tasks.
func (ms *MetricsScheduler) CancelAll() {
	ms.scheduler.CancelAll()
}

// GetTask returns information about a scheduled task.
func (ms *MetricsScheduler) GetTask(id string) (*ScheduledTask, bool) {
	return ms.scheduler.GetTask(id)
}

// ListTasks returns all currently scheduled tasks.
func (ms *MetricsScheduler) ListTasks() []ScheduledTask {
	return ms.scheduler.ListTasks()
}

// Start begins the scheduler execution loop.
func (ms *MetricsScheduler) Start() error {
	return ms.scheduler.Start()
}

// Stop gracefully shuts down the scheduler.
func (ms *MetricsScheduler) Stop() <-chan struct{} {
	return ms.scheduler.Stop()
}

// Stats returns current scheduler statistics.
func (ms *MetricsScheduler) Stats() Stats {
	return ms.scheduler.Stats()
}

// IsRunning returns true if the scheduler is currently running.
func (ms *MetricsScheduler) IsRunning() bool {
	return ms.scheduler.IsRunning()
}

// EnableMetrics enables metrics collection.
func (ms *MetricsScheduler) EnableMetrics(config metrics.Config) error {
	ms.enabled = config.Enabled
	
	if config.Registry != nil {
		ms.registry = metrics.NewRegistry(config.Registry)
	}
	
	return nil
}

// DisableMetrics disables metrics collection.
func (ms *MetricsScheduler) DisableMetrics() {
	ms.enabled = false
}

// MetricsEnabled returns true if metrics are currently enabled.
func (ms *MetricsScheduler) MetricsEnabled() bool {
	return ms.enabled
}