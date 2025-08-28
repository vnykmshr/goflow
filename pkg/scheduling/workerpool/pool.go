package workerpool

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Task represents a unit of work that can be executed by a worker.
type Task interface {
	// Execute runs the task with the given context.
	// It should respect context cancellation and return any error encountered.
	Execute(ctx context.Context) error
}

// TaskFunc is a function type that implements the Task interface.
type TaskFunc func(ctx context.Context) error

// Execute implements the Task interface for TaskFunc.
func (f TaskFunc) Execute(ctx context.Context) error {
	return f(ctx)
}

// Result represents the result of a task execution.
type Result struct {
	// Task is the original task that was executed
	Task Task

	// Error is any error that occurred during task execution
	Error error

	// Duration is how long the task took to execute
	Duration time.Duration

	// WorkerID identifies which worker executed the task
	WorkerID int
}

// Pool represents a worker pool that can execute tasks concurrently.
type Pool interface {
	// Submit adds a task to the pool for execution.
	Submit(task Task) error

	// Results returns a channel of task results.
	Results() <-chan Result

	// Shutdown initiates a graceful shutdown of the pool.
	Shutdown() <-chan struct{}

	// Size returns the number of workers in the pool.
	Size() int

	// QueueSize returns the current number of queued tasks.
	QueueSize() int
}

// Config holds configuration options for creating a worker pool.
type Config struct {
	// WorkerCount is the number of workers in the pool.
	WorkerCount int

	// QueueSize is the maximum number of tasks that can be queued.
	// If 0, uses a reasonable default based on worker count.
	QueueSize int

	// TaskTimeout is the default timeout for individual task execution.
	// Zero means no timeout.
	TaskTimeout time.Duration
}

// workerPool implements the Pool interface.
type workerPool struct {
	config Config

	workers      []worker
	taskQueue    chan Task
	resultQueue  chan Result
	shutdownCh   chan struct{}
	shutdownOnce sync.Once

	mu         sync.RWMutex
	isShutdown bool

	workerWg sync.WaitGroup
}

// worker represents a single worker in the pool.
type worker struct {
	id      int
	pool    *workerPool
	stopCh  chan struct{}
	stopped chan struct{}
}

// New creates a new worker pool with the specified number of workers and queue size.
func New(workerCount, queueSize int) Pool {
	return NewWithConfig(Config{
		WorkerCount: workerCount,
		QueueSize:   queueSize,
	})
}

// NewWithConfig creates a new worker pool with the specified configuration.
func NewWithConfig(config Config) Pool {
	if config.WorkerCount <= 0 {
		panic("worker count must be positive, got " + fmt.Sprint(config.WorkerCount))
	}

	// Use reasonable defaults
	queueSize := config.QueueSize
	if queueSize == 0 {
		queueSize = config.WorkerCount * 2
	}

	pool := &workerPool{
		config:      config,
		taskQueue:   make(chan Task, queueSize),
		resultQueue: make(chan Result, config.WorkerCount),
		shutdownCh:  make(chan struct{}),
	}

	// Create and start workers
	pool.workers = make([]worker, config.WorkerCount)
	for i := 0; i < config.WorkerCount; i++ {
		pool.workers[i] = worker{
			id:      i,
			pool:    pool,
			stopCh:  make(chan struct{}),
			stopped: make(chan struct{}),
		}
		pool.workerWg.Add(1)
		go pool.workers[i].run()
	}

	return pool
}
