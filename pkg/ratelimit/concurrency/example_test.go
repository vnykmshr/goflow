package concurrency_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/vnykmshr/goflow/pkg/ratelimit/concurrency"
)

// Example demonstrates basic usage of the concurrency limiter
func Example() {
	// Create a limiter that allows 3 concurrent operations
	limiter, err := concurrency.NewSafe(3)
	if err != nil {
		panic(fmt.Sprintf("Failed to create limiter: %v", err))
	}

	// Try to acquire a permit (non-blocking)
	if limiter.Acquire() {
		fmt.Println("Operation permitted")
		// Do work...
		limiter.Release() // Don't forget to release!
	} else {
		fmt.Println("Operation denied - at capacity")
	}

	// Output: Operation permitted
}

// Example_workerPool demonstrates using concurrency limiter for a worker pool
func Example_workerPool() {
	// Limit concurrent workers to 2
	limiter, err := concurrency.NewSafe(2)
	if err != nil {
		panic(fmt.Sprintf("Failed to create limiter: %v", err))
	}

	tasks := []string{"task1", "task2", "task3", "task4", "task5"}
	var wg sync.WaitGroup

	for _, task := range tasks {
		wg.Add(1)
		go func(taskName string) {
			defer wg.Done()

			// Wait for permit (blocks if necessary)
			ctx := context.Background()
			if err := limiter.Wait(ctx); err != nil {
				fmt.Printf("Failed to acquire permit for %s: %v\n", taskName, err)
				return
			}
			defer limiter.Release()

			// Simulate work
			time.Sleep(100 * time.Millisecond)
		}(task)
	}

	wg.Wait()
	fmt.Println("All tasks completed")

	// Output: All tasks completed
}

// Example_databaseConnections demonstrates limiting database connections
func Example_databaseConnections() {
	// Limit to 3 concurrent database connections
	dbLimiter, err := concurrency.NewSafe(3)
	if err != nil {
		panic(fmt.Sprintf("Failed to create limiter: %v", err))
	}

	fmt.Printf("Database limiter capacity: %d\n", dbLimiter.Capacity())
	fmt.Printf("Available connections: %d\n", dbLimiter.Available())

	// Simulate acquiring connections
	for i := 1; i <= 4; i++ {
		if dbLimiter.Acquire() {
			fmt.Printf("Connection %d acquired. Available: %d, In use: %d\n",
				i, dbLimiter.Available(), dbLimiter.InUse())
		} else {
			fmt.Printf("Connection %d denied. Available: %d, In use: %d\n",
				i, dbLimiter.Available(), dbLimiter.InUse())
		}
	}

	// Release a connection
	dbLimiter.Release()
	fmt.Printf("Connection released. Available: %d, In use: %d\n",
		dbLimiter.Available(), dbLimiter.InUse())

	// Output:
	// Database limiter capacity: 3
	// Available connections: 3
	// Connection 1 acquired. Available: 2, In use: 1
	// Connection 2 acquired. Available: 1, In use: 2
	// Connection 3 acquired. Available: 0, In use: 3
	// Connection 4 denied. Available: 0, In use: 3
	// Connection released. Available: 1, In use: 2
}

// Example_withTimeout demonstrates using timeouts with concurrency limiter
func Example_withTimeout() {
	// Small limiter to demonstrate blocking
	limiter, err := concurrency.NewSafe(1)
	if err != nil {
		panic(fmt.Sprintf("Failed to create limiter: %v", err))
	}

	// Fill the limiter
	limiter.Acquire()

	// Try to acquire with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err = limiter.Wait(ctx)
	if err != nil {
		fmt.Printf("Failed to acquire permit: %v\n", err)
	} else {
		fmt.Println("Permit acquired")
		limiter.Release()
	}

	// Output: Failed to acquire permit: context deadline exceeded
}

// Example_multiplePermits demonstrates acquiring multiple permits at once
func Example_multiplePermits() {
	limiter, err := concurrency.NewSafe(5)
	if err != nil {
		panic(fmt.Sprintf("Failed to create limiter: %v", err))
	}

	// Acquire multiple permits for batch operation
	if limiter.AcquireN(3) {
		fmt.Printf("Batch operation started. Available: %d, In use: %d\n",
			limiter.Available(), limiter.InUse())

		// Simulate batch work
		time.Sleep(10 * time.Millisecond)

		// Release all permits
		limiter.ReleaseN(3)
		fmt.Printf("Batch operation completed. Available: %d, In use: %d\n",
			limiter.Available(), limiter.InUse())
	} else {
		fmt.Println("Not enough permits available for batch operation")
	}

	// Output:
	// Batch operation started. Available: 2, In use: 3
	// Batch operation completed. Available: 5, In use: 0
}

// Example_dynamicCapacity demonstrates changing capacity at runtime
func Example_dynamicCapacity() {
	limiter, err := concurrency.NewSafe(3)
	if err != nil {
		panic(fmt.Sprintf("Failed to create limiter: %v", err))
	}

	// Use some permits
	limiter.AcquireN(2)
	fmt.Printf("Initial: capacity=%d, available=%d, in_use=%d\n",
		limiter.Capacity(), limiter.Available(), limiter.InUse())

	// Increase capacity
	limiter.SetCapacity(5)
	fmt.Printf("After increase: capacity=%d, available=%d, in_use=%d\n",
		limiter.Capacity(), limiter.Available(), limiter.InUse())

	// Decrease capacity
	limiter.SetCapacity(3)
	fmt.Printf("After decrease: capacity=%d, available=%d, in_use=%d\n",
		limiter.Capacity(), limiter.Available(), limiter.InUse())

	// Output:
	// Initial: capacity=3, available=1, in_use=2
	// After increase: capacity=5, available=3, in_use=2
	// After decrease: capacity=3, available=1, in_use=2
}

// Example_customConfiguration demonstrates advanced configuration
func Example_customConfiguration() {
	// Create limiter with custom initial state
	config := concurrency.Config{
		Capacity:         10,
		InitialAvailable: 5, // Start with only 5 available (simulating 5 in use)
	}

	limiter, err := concurrency.NewWithConfigSafe(config)
	if err != nil {
		panic(fmt.Sprintf("Failed to create limiter: %v", err))
	}

	fmt.Printf("Custom limiter: capacity=%d, available=%d, in_use=%d\n",
		limiter.Capacity(), limiter.Available(), limiter.InUse())

	// Output: Custom limiter: capacity=10, available=5, in_use=5
}

// Example_gracefulShutdown demonstrates graceful shutdown pattern
func Example_gracefulShutdown() {
	limiter, err := concurrency.NewSafe(2)
	if err != nil {
		panic(fmt.Sprintf("Failed to create limiter: %v", err))
	}

	// Start some work
	var wg sync.WaitGroup
	for i := 1; i <= 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Acquire permit with timeout for shutdown
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			if err := limiter.Wait(ctx); err != nil {
				fmt.Printf("Worker %d: shutdown timeout\n", id)
				return
			}
			defer limiter.Release()

			time.Sleep(50 * time.Millisecond)
		}(i)
	}

	wg.Wait()
	fmt.Printf("Final state: available=%d, in_use=%d\n",
		limiter.Available(), limiter.InUse())

	// Output: Final state: available=2, in_use=0
}

// Example_httpServerLimiting demonstrates HTTP request limiting pattern
func Example_httpServerLimiting() {
	// Limit concurrent HTTP requests
	requestLimiter, err := concurrency.NewSafe(100)
	if err != nil {
		panic(fmt.Sprintf("Failed to create limiter: %v", err))
	}

	// Simulate request handler
	handleRequest := func(requestID int) {
		if !requestLimiter.Acquire() {
			fmt.Printf("Request %d: Server busy, try again later\n", requestID)
			return
		}
		defer requestLimiter.Release()

		// Simulate request processing time
		time.Sleep(10 * time.Millisecond)
	}

	// Simulate burst of requests
	var wg sync.WaitGroup
	for i := 1; i <= 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			handleRequest(id)
		}(i)
	}

	wg.Wait()
	fmt.Printf("All requests processed. Server state: %d/%d in use\n",
		requestLimiter.InUse(), requestLimiter.Capacity())

	// Output: All requests processed. Server state: 0/100 in use
}
