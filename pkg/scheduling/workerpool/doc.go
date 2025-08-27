/*
Package workerpool provides a flexible and efficient worker pool implementation for Go applications.

A worker pool manages a fixed number of worker goroutines that execute tasks concurrently.
This pattern is essential for controlling resource usage, limiting concurrency, and achieving
predictable performance in high-throughput scenarios.

Basic usage:

	pool := workerpool.New(4, 100) // 4 workers, queue size 100
	defer pool.Shutdown()

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		// Do work
		return nil
	})

	if err := pool.Submit(task); err != nil {
		log.Printf("Failed to submit: %v", err)
	}

	// Process result
	result := <-pool.Results()
	if result.Error != nil {
		log.Printf("Task failed: %v", result.Error)
	}

Key Features:

The worker pool provides:
  - Fixed number of worker goroutines for predictable resource usage
  - Bounded task queue to prevent memory exhaustion
  - Context-aware task execution with timeout support
  - Comprehensive error handling and panic recovery
  - Graceful shutdown with completion guarantees
  - Real-time metrics and state inspection
  - Flexible configuration with lifecycle callbacks
  - Multiple submission methods for different use cases

Task Interface:

Tasks implement a simple interface:

	type Task interface {
		Execute(ctx context.Context) error
	}

The TaskFunc type provides a convenient way to create tasks from functions:

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		// Task implementation
		return nil
	})

Configuration Options:

Advanced configuration is available through the Config struct:

	config := workerpool.Config{
		WorkerCount:     8,
		QueueSize:       1000,
		TaskTimeout:     30 * time.Second,
		BufferedResults: true,
		PanicHandler: func(task Task, recovered interface{}) {
			log.Printf("Task panicked: %v", recovered)
		},
		OnTaskComplete: func(workerID int, result Result) {
			log.Printf("Worker %d completed task in %v", workerID, result.Duration)
		},
	}
	pool := workerpool.NewWithConfig(config)

Submission Methods:

Multiple ways to submit tasks:

	// Basic submission
	err := pool.Submit(task)

	// With timeout
	err := pool.SubmitWithTimeout(task, time.Second)

	// With context
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := pool.SubmitWithContext(ctx, task)

Result Processing:

Results are delivered through a channel:

	for result := range pool.Results() {
		if result.Error != nil {
			log.Printf("Task failed: %v", result.Error)
		} else {
			log.Printf("Task completed in %v by worker %d",
				result.Duration, result.WorkerID)
		}
	}

Error Handling:

The pool handles various error conditions:

	// Task errors are captured in results
	task := workerpool.TaskFunc(func(ctx context.Context) error {
		return errors.New("task failed")
	})

	// Panics are recovered and can be customized
	task := workerpool.TaskFunc(func(ctx context.Context) error {
		panic("something went wrong")
	})

	// Context cancellation is supported
	task := workerpool.TaskFunc(func(ctx context.Context) error {
		select {
		case <-time.After(time.Hour):
			return nil
		case <-ctx.Done():
			return ctx.Err() // Timeout or cancellation
		}
	})

Use Cases:

Web Crawling:

	pool := workerpool.New(10, 100)
	defer pool.Shutdown()

	for _, url := range urls {
		url := url
		task := workerpool.TaskFunc(func(ctx context.Context) error {
			return crawl(ctx, url)
		})
		pool.Submit(task)
	}

Image Processing:

	pool := workerpool.New(4, 50) // CPU-bound work
	defer pool.Shutdown()

	for _, image := range images {
		image := image
		task := workerpool.TaskFunc(func(ctx context.Context) error {
			return processImage(ctx, image)
		})
		pool.Submit(task)
	}

Database Operations:

	config := workerpool.Config{
		WorkerCount: 20,
		QueueSize:   200,
		TaskTimeout: 30 * time.Second,
		OnWorkerStart: func(workerID int) {
			// Initialize per-worker DB connection
			connections[workerID] = db.Connect()
		},
	}
	pool := workerpool.NewWithConfig(config)

API Request Processing:

	pool := workerpool.New(50, 1000)
	defer pool.Shutdown()

	http.HandleFunc("/process", func(w http.ResponseWriter, r *http.Request) {
		task := workerpool.TaskFunc(func(ctx context.Context) error {
			return processRequest(ctx, r)
		})

		if err := pool.SubmitWithTimeout(task, time.Second); err != nil {
			http.Error(w, "Server busy", http.StatusTooManyRequests)
			return
		}

		// Handle response asynchronously or wait for result
	})

Queue Configurations:

Different queue sizes serve different purposes:

	// Bounded queue - prevents memory exhaustion
	pool := workerpool.New(4, 100)

	// Unbounded queue - for bursty workloads (use with caution)
	pool := workerpool.New(4, 0)

	// Synchronous execution - no queueing
	pool := workerpool.New(4, -1)

Monitoring and Metrics:

The pool provides real-time metrics:

	fmt.Printf("Pool size: %d\n", pool.Size())
	fmt.Printf("Queue size: %d\n", pool.QueueSize())
	fmt.Printf("Active workers: %d\n", pool.ActiveWorkers())
	fmt.Printf("Total submitted: %d\n", pool.TotalSubmitted())
	fmt.Printf("Total completed: %d\n", pool.TotalCompleted())

Lifecycle Callbacks:

Monitor worker and task lifecycle:

	config := workerpool.Config{
		WorkerCount: 4,
		QueueSize:   100,
		OnWorkerStart: func(workerID int) {
			log.Printf("Worker %d started", workerID)
		},
		OnWorkerStop: func(workerID int) {
			log.Printf("Worker %d stopped", workerID)
		},
		OnTaskStart: func(workerID int, task Task) {
			log.Printf("Worker %d starting task", workerID)
		},
		OnTaskComplete: func(workerID int, result Result) {
			log.Printf("Worker %d completed task in %v", workerID, result.Duration)
		},
	}

Graceful Shutdown:

Proper shutdown ensures all tasks complete:

	// Graceful shutdown - waits for running tasks
	shutdownComplete := pool.Shutdown()
	<-shutdownComplete

	// Shutdown with timeout - forces termination after timeout
	shutdownComplete := pool.ShutdownWithTimeout(30 * time.Second)
	<-shutdownComplete

Best Practices:

1. Size worker pools based on workload characteristics:
  - CPU-bound: workers = CPU cores
  - I/O-bound: workers = 2-4x CPU cores
  - Network-bound: workers = even higher ratio

2. Choose appropriate queue sizes:
  - Bounded queues prevent memory exhaustion
  - Size based on expected burst capacity
  - Monitor queue size to detect bottlenecks

3. Handle context cancellation in long-running tasks:
  - Check ctx.Done() in loops
  - Use select statements with context
  - Return ctx.Err() when canceled

4. Use timeouts for tasks that might hang:
  - Set TaskTimeout in config
  - Implement timeouts within task logic
  - Monitor task durations

5. Implement proper error handling:
  - Process all results from Results() channel
  - Log errors appropriately
  - Consider retry logic for transient failures

Performance Characteristics:

  - Task submission: O(1) for available queue space
  - Task execution: Parallel across workers
  - Memory usage: Bounded by queue size and worker count
  - Shutdown time: Depends on longest-running task

Thread Safety:

All pool operations are safe for concurrent use from multiple goroutines.
Internal synchronization ensures consistent state without external locking.

Comparison with Alternatives:

vs Manual goroutine management:
  - Better resource control and limits
  - Built-in error handling and monitoring
  - Graceful shutdown capabilities

vs Channel-based patterns:
  - Higher-level abstraction
  - More comprehensive error handling
  - Better observability and metrics

vs sync.WaitGroup:
  - Continuous operation vs batch processing
  - Resource limiting vs unlimited concurrency
  - Result collection and error handling built-in
*/
package workerpool
