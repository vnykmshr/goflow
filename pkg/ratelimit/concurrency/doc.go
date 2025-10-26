/*
Package concurrency provides concurrency limiting for Go applications.

A concurrency limiter controls the number of operations that can run
simultaneously, acting as a semaphore with additional features like
context support, dynamic configuration, and comprehensive state inspection.

Basic usage:

	limiter, err := concurrency.NewSafe(10) // Allow 10 concurrent operations
	if err != nil {
		log.Fatal(err)
	}

	if limiter.Acquire() {
		defer limiter.Release()
		// Do work
	}

Key Features:

The concurrency limiter provides:
  - Non-blocking permit acquisition (Acquire/AcquireN)
  - Context-aware blocking operations (Wait/WaitN)
  - Batch permit operations for efficiency
  - Dynamic capacity adjustment at runtime
  - Comprehensive state inspection
  - Graceful handling of context cancellation

Use Cases:

Concurrency limiting is ideal for:
  - Database connection pools
  - HTTP server request limiting
  - Worker pool management
  - Resource-intensive operation control
  - API rate limiting by active requests
  - Memory-bound operation throttling

Worker Pool Pattern:

	limiter, err := concurrency.NewSafe(5) // Max 5 concurrent workers
	if err != nil {
		log.Fatal(err)
	}

	var wg sync.WaitGroup
	for _, task := range tasks {
		wg.Add(1)
		go func(t Task) {
			defer wg.Done()

			// Wait for available slot
			if err := limiter.Wait(ctx); err != nil {
				return // Context canceled/timeout
			}
			defer limiter.Release()

			// Process task
			processTask(t)
		}(task)
	}
	wg.Wait()

Database Connection Limiting:

	dbLimiter, err := concurrency.NewSafe(20) // Max 20 DB connections
	if err != nil {
		log.Fatal(err)
	}

	func queryDatabase(query string) error {
		// Acquire connection permit
		if !dbLimiter.Acquire() {
			return errors.New("database busy")
		}
		defer dbLimiter.Release()

		// Execute query
		return db.Query(query)
	}

HTTP Server Limiting:

	requestLimiter, err := concurrency.NewSafe(1000) // Max 1000 concurrent requests
	if err != nil {
		log.Fatal(err)
	}

	func handler(w http.ResponseWriter, r *http.Request) {
		if !requestLimiter.Acquire() {
			http.Error(w, "Server busy", http.StatusTooManyRequests)
			return
		}
		defer requestLimiter.Release()

		// Handle request
		handleRequest(w, r)
	}

Batch Operations:

	// Acquire multiple permits for batch processing
	if limiter.AcquireN(batchSize) {
		defer limiter.ReleaseN(batchSize)

		// Process batch
		for _, item := range batch {
			processItem(item)
		}
	}

Context Integration:

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := limiter.Wait(ctx); err != nil {
		if err == context.DeadlineExceeded {
			// Handle timeout
		}
		return err
	}
	defer limiter.Release()

Dynamic Configuration:

	// Start with capacity for normal load
	limiter, err := concurrency.NewSafe(10)
	if err != nil {
		log.Fatal(err)
	}

	// Increase capacity during peak hours
	limiter.SetCapacity(50)

	// Monitor and adjust based on system metrics
	if systemLoad < 0.5 {
		limiter.SetCapacity(limiter.Capacity() + 10)
	}

State Inspection:

	capacity := limiter.Capacity()   // Maximum concurrent operations
	available := limiter.Available() // Currently available permits
	inUse := limiter.InUse()        // Currently active operations

	// Calculate utilization
	utilization := float64(inUse) / float64(capacity)

Advanced Configuration:

	config := concurrency.Config{
		Capacity:         100,
		InitialAvailable: 50, // Start with 50 operations "in use"
	}
	limiter, err := concurrency.NewWithConfigSafe(config)
	if err != nil {
		log.Fatal(err)
	}

Graceful Shutdown:

	// Allow pending operations to complete, but reject new ones
	limiter.SetCapacity(0) // Stop accepting new operations

	// Wait for active operations to complete
	for limiter.InUse() > 0 {
		time.Sleep(100 * time.Millisecond)
	}

Error Handling:

The limiter panics on:
  - Invalid capacity (zero or negative)
  - Releasing more permits than capacity
  - Invalid configuration parameters

Context errors are returned for:
  - context.Canceled: Context was canceled
  - context.DeadlineExceeded: Context timeout exceeded

Thread Safety:

All operations are safe for concurrent use. The limiter uses mutex-based
synchronization to ensure consistency while maintaining good performance
for high-throughput scenarios.

Performance Characteristics:

  - Acquire/Release: O(1) for immediate success/failure
  - Wait operations: O(n) where n is the number of waiters
  - Memory usage scales with number of waiting goroutines
  - Optimized for high-frequency acquire/release patterns

Comparison with Other Patterns:

vs Channel-based semaphores:
  - Better performance for high-frequency operations
  - More features (state inspection, dynamic capacity)
  - Context integration built-in

vs sync.WaitGroup:
  - Controls concurrency rather than coordination
  - Supports timeouts and cancellation
  - Runtime capacity adjustment

vs Worker pools:
  - More flexible - works with any operation pattern
  - Lower overhead - no goroutine pool management
  - Dynamic scaling based on demand
*/
package concurrency
