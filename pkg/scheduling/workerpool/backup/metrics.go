package workerpool

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/vnykmshr/goflow/pkg/metrics"
)

// MetricsPool wraps a worker Pool with Prometheus metrics collection.
type MetricsPool struct {
	pool     Pool
	name     string
	registry *metrics.Registry
	enabled  bool
}

// NewWithMetrics creates a new worker pool with metrics enabled.
func NewWithMetrics(workerCount int, name string) Pool {
	// Use a separate registry for each metrics-enabled component to avoid conflicts
	registry := prometheus.NewRegistry()
	config := metrics.Config{
		Enabled:  true,
		Registry: registry,
	}
	
	return NewWithConfigAndMetrics(Config{
		WorkerCount: workerCount,
		QueueSize:   0, // Unbuffered by default
	}, name, config)
}

// NewWithConfigAndMetrics creates a new worker pool with custom config and metrics.
func NewWithConfigAndMetrics(config Config, name string, metricsConfig metrics.Config) Pool {
	basePool := NewWithConfig(config)
	
	if !metricsConfig.Enabled {
		return basePool
	}

	registry := metrics.DefaultRegistry
	if metricsConfig.Registry != nil {
		// Create custom registry if provided
		registry = metrics.NewRegistry(metricsConfig.Registry)
	}

	mp := &MetricsPool{
		pool:     basePool,
		name:     name,
		registry: registry,
		enabled:  true,
	}

	// Initialize metrics
	mp.updateMetrics()
	
	return mp
}

// updateMetrics updates the current state metrics.
func (mp *MetricsPool) updateMetrics() {
	if !mp.enabled {
		return
	}
	
	mp.registry.WorkerPoolSize.WithLabelValues(mp.name).Set(float64(mp.pool.Size()))
	mp.registry.WorkerPoolActive.WithLabelValues(mp.name).Set(float64(mp.pool.ActiveWorkers()))
	mp.registry.WorkerPoolQueued.WithLabelValues(mp.name).Set(float64(mp.pool.QueueSize()))
}

// Submit adds a task to the pool for execution.
func (mp *MetricsPool) Submit(task Task) error {
	return mp.SubmitWithContext(context.Background(), task)
}

// SubmitWithTimeout submits a task with a timeout for queuing.
func (mp *MetricsPool) SubmitWithTimeout(task Task, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return mp.SubmitWithContext(ctx, task)
}

// SubmitWithContext submits a task with a context for cancellation.
func (mp *MetricsPool) SubmitWithContext(ctx context.Context, task Task) error {
	// Wrap the task to collect metrics
	wrappedTask := &metricsTask{
		original: task,
		pool:     mp,
		submitTime: time.Now(),
	}
	
	err := mp.pool.SubmitWithContext(ctx, wrappedTask)
	
	if mp.enabled {
		mp.updateMetrics()
	}
	
	return err
}

// metricsTask wraps a Task to collect execution metrics.
type metricsTask struct {
	original   Task
	pool       *MetricsPool
	submitTime time.Time
}

// Execute runs the original task and records metrics.
func (mt *metricsTask) Execute(ctx context.Context) error {
	start := time.Now()
	queueTime := start.Sub(mt.submitTime)
	
	if mt.pool.enabled {
		// Record queue wait time using task execution histogram
		mt.pool.registry.TaskExecutionDuration.WithLabelValues(mt.pool.name).Observe(queueTime.Seconds())
	}
	
	err := mt.original.Execute(ctx)
	
	if mt.pool.enabled {
		executionTime := time.Since(start)
		
		// Record execution duration
		mt.pool.registry.TaskExecutionDuration.WithLabelValues(mt.pool.name).Observe(executionTime.Seconds())
		
		// Update counters
		mt.pool.registry.TasksExecuted.WithLabelValues(mt.pool.name).Inc()
		
		if err != nil {
			mt.pool.registry.TasksFailed.WithLabelValues(mt.pool.name).Inc()
		} else {
			mt.pool.registry.TasksCompleted.WithLabelValues(mt.pool.name).Inc()
		}
		
		// Update current metrics
		mt.pool.updateMetrics()
	}
	
	return err
}

// Results returns a channel of task results.
func (mp *MetricsPool) Results() <-chan Result {
	return mp.pool.Results()
}

// Shutdown initiates graceful shutdown of the pool.
func (mp *MetricsPool) Shutdown() <-chan struct{} {
	return mp.pool.Shutdown()
}

// ShutdownWithTimeout shuts down the pool with a timeout.
func (mp *MetricsPool) ShutdownWithTimeout(timeout time.Duration) <-chan struct{} {
	return mp.pool.ShutdownWithTimeout(timeout)
}

// Size returns the current number of workers.
func (mp *MetricsPool) Size() int {
	return mp.pool.Size()
}

// QueueSize returns the current number of queued tasks.
func (mp *MetricsPool) QueueSize() int {
	queueSize := mp.pool.QueueSize()
	
	if mp.enabled {
		mp.registry.WorkerPoolQueued.WithLabelValues(mp.name).Set(float64(queueSize))
	}
	
	return queueSize
}

// ActiveWorkers returns the number of workers currently executing tasks.
func (mp *MetricsPool) ActiveWorkers() int {
	activeWorkers := mp.pool.ActiveWorkers()
	
	if mp.enabled {
		mp.registry.WorkerPoolActive.WithLabelValues(mp.name).Set(float64(activeWorkers))
	}
	
	return activeWorkers
}

// TotalSubmitted returns the total number of tasks submitted.
func (mp *MetricsPool) TotalSubmitted() int64 {
	return mp.pool.TotalSubmitted()
}

// TotalCompleted returns the total number of tasks completed.
func (mp *MetricsPool) TotalCompleted() int64 {
	return mp.pool.TotalCompleted()
}

// EnableMetrics enables metrics collection.
func (mp *MetricsPool) EnableMetrics(config metrics.Config) error {
	mp.enabled = config.Enabled
	
	if config.Registry != nil {
		mp.registry = metrics.NewRegistry(config.Registry)
	}
	
	if mp.enabled {
		mp.updateMetrics()
	}
	
	return nil
}

// DisableMetrics disables metrics collection.
func (mp *MetricsPool) DisableMetrics() {
	mp.enabled = false
}

// MetricsEnabled returns true if metrics are currently enabled.
func (mp *MetricsPool) MetricsEnabled() bool {
	return mp.enabled
}