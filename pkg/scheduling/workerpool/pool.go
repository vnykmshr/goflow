package workerpool

import (
	"context"
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
	// Returns an error if the pool is shut down or if the task cannot be queued.
	Submit(task Task) error

	// SubmitWithTimeout submits a task with a timeout for queuing.
	// If the task cannot be queued within the timeout, it returns an error.
	SubmitWithTimeout(task Task, timeout time.Duration) error

	// SubmitWithContext submits a task with a context for cancellation.
	// The context applies to the queuing operation, not the task execution itself.
	SubmitWithContext(ctx context.Context, task Task) error

	// Results returns a channel of task results.
	// The channel is closed when the pool is shut down and all tasks are complete.
	Results() <-chan Result

	// Shutdown initiates a graceful shutdown of the pool.
	// No new tasks will be accepted, but queued tasks will be completed.
	// Returns a channel that closes when shutdown is complete.
	Shutdown() <-chan struct{}

	// ShutdownWithTimeout shuts down the pool with a timeout.
	// If shutdown doesn't complete within the timeout, remaining tasks are canceled.
	ShutdownWithTimeout(timeout time.Duration) <-chan struct{}

	// Size returns the number of workers in the pool.
	Size() int

	// QueueSize returns the current number of queued tasks waiting for execution.
	QueueSize() int

	// ActiveWorkers returns the number of workers currently executing tasks.
	ActiveWorkers() int

	// TotalSubmitted returns the total number of tasks submitted to the pool.
	TotalSubmitted() int64

	// TotalCompleted returns the total number of tasks completed by the pool.
	TotalCompleted() int64
}

// Config holds configuration options for creating a worker pool.
type Config struct {
	// WorkerCount is the number of workers in the pool.
	// Must be greater than 0.
	WorkerCount int

	// QueueSize is the maximum number of tasks that can be queued.
	// If 0, an unbounded queue is used (not recommended for production).
	// If -1, tasks are executed synchronously (no queueing).
	QueueSize int

	// WorkerTimeout is the maximum time a worker will wait for a new task
	// before being considered idle. Zero means workers wait indefinitely.
	WorkerTimeout time.Duration

	// TaskTimeout is the default timeout for individual task execution.
	// Zero means no timeout. Can be overridden per task.
	TaskTimeout time.Duration

	// BufferedResults determines if results should be buffered.
	// If true, results are sent to a buffered channel to prevent blocking.
	// Buffer size equals worker count.
	BufferedResults bool

	// PanicHandler is called when a worker panics during task execution.
	// If nil, panics are recovered and logged as errors.
	PanicHandler func(task Task, recovered interface{})

	// OnWorkerStart is called when a worker starts.
	// Useful for per-worker initialization (e.g., database connections).
	OnWorkerStart func(workerID int)

	// OnWorkerStop is called when a worker stops.
	// Useful for per-worker cleanup.
	OnWorkerStop func(workerID int)

	// OnTaskStart is called before a task begins execution.
	OnTaskStart func(workerID int, task Task)

	// OnTaskComplete is called after a task completes (success or failure).
	OnTaskComplete func(workerID int, result Result)
}

// workerPool implements the Pool interface.
type workerPool struct {
	config Config

	// Core pool state
	workers      []worker
	taskQueue    chan Task
	resultQueue  chan Result
	shutdownCh   chan struct{}
	shutdownOnce sync.Once

	// State tracking
	mu             sync.RWMutex
	isShutdown     bool
	activeWorkers  int
	totalSubmitted int64
	totalCompleted int64

	// Worker management
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
		panic("worker count must be positive")
	}

	if config.QueueSize < -1 {
		panic("queue size must be >= -1")
	}

	var taskQueue chan Task
	if config.QueueSize == -1 {
		// Synchronous execution - no queueing
		taskQueue = make(chan Task)
	} else if config.QueueSize == 0 {
		// Unbounded queue
		taskQueue = make(chan Task)
	} else {
		// Bounded queue
		taskQueue = make(chan Task, config.QueueSize)
	}

	var resultQueue chan Result
	if config.BufferedResults {
		resultQueue = make(chan Result, config.WorkerCount)
	} else {
		resultQueue = make(chan Result)
	}

	pool := &workerPool{
		config:      config,
		taskQueue:   taskQueue,
		resultQueue: resultQueue,
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
