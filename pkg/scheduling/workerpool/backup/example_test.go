package workerpool_test

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

// Example demonstrates basic usage of the worker pool
func Example() {
	// Create a worker pool with 3 workers and queue size of 10
	pool := workerpool.New(3, 10)
	defer pool.Shutdown()

	// Submit a simple task
	task := workerpool.TaskFunc(func(ctx context.Context) error {
		fmt.Println("Task executed")
		return nil
	})

	if err := pool.Submit(task); err != nil {
		log.Printf("Failed to submit task: %v", err)
		return
	}

	// Wait for result
	result := <-pool.Results()
	if result.Error != nil {
		log.Printf("Task failed: %v", result.Error)
	}

	// Output: Task executed
}

// Example_webCrawler demonstrates using worker pool for web crawling
func Example_webCrawler() {
	// Create worker pool for concurrent crawling
	pool := workerpool.New(5, 20)
	defer pool.Shutdown()

	urls := []string{
		"https://example.com",
		"https://google.com",
		"https://github.com",
	}

	var completed int32

	// Submit crawl tasks
	for _, url := range urls {
		url := url // Capture loop variable
		task := workerpool.TaskFunc(func(ctx context.Context) error {
			// Simulate web crawling
			time.Sleep(10 * time.Millisecond)
			return nil
		})

		if err := pool.Submit(task); err != nil {
			log.Printf("Failed to submit crawl task for %s: %v", url, err)
		}
	}

	// Collect results
	go func() {
		for result := range pool.Results() {
			if result.Error != nil {
				log.Printf("Crawl failed: %v", result.Error)
			}
			atomic.AddInt32(&completed, 1)
		}
	}()

	// Wait for all tasks to complete
	for atomic.LoadInt32(&completed) < int32(len(urls)) {
		time.Sleep(time.Millisecond)
	}

	fmt.Printf("Completed crawling %d URLs\n", len(urls))

	// Output: Completed crawling 3 URLs
}

// Example_imageProcessing demonstrates batch image processing
func Example_imageProcessing() {
	// Configure pool with callbacks for monitoring
	config := workerpool.Config{
		WorkerCount: 4,
		QueueSize:   50,
		OnTaskComplete: func(workerID int, result workerpool.Result) {
			if result.Error != nil {
				fmt.Printf("Worker %d failed to process image: %v\n", workerID, result.Error)
			}
		},
	}

	pool := workerpool.NewWithConfig(config)
	defer pool.Shutdown()

	// Simulate processing multiple images
	imageFiles := []string{"img1.jpg", "img2.jpg", "img3.jpg", "img4.jpg"}

	for range imageFiles {
		task := workerpool.TaskFunc(func(ctx context.Context) error {
			// Simulate image processing work
			time.Sleep(20 * time.Millisecond)
			return nil
		})

		pool.Submit(task)
	}

	// Monitor results
	processed := 0
	for range pool.Results() {
		processed++
		if processed >= len(imageFiles) {
			break
		}
	}

	fmt.Printf("Processed %d images\n", processed)

	// Output: Processed 4 images
}

// Example_dataProcessingPipeline demonstrates processing data with timeouts
func Example_dataProcessingPipeline() {
	// Create pool with task timeout
	config := workerpool.Config{
		WorkerCount: 2,
		QueueSize:   10,
		TaskTimeout: 100 * time.Millisecond,
	}

	pool := workerpool.NewWithConfig(config)
	defer pool.Shutdown()

	// Simulate data records with varying processing times
	processingTimes := []time.Duration{
		50 * time.Millisecond,  // Fast
		150 * time.Millisecond, // Slow (will timeout)
		30 * time.Millisecond,  // Fast
	}

	for _, duration := range processingTimes {
		duration := duration // Capture loop variable
		task := workerpool.TaskFunc(func(ctx context.Context) error {
			select {
			case <-time.After(duration):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})

		pool.Submit(task)
	}

	// Collect results
	for i := 0; i < len(processingTimes); i++ {
		<-pool.Results()
	}

	fmt.Println("Data processing pipeline completed")

	// Output: Data processing pipeline completed
}

// Example_errorHandling demonstrates error handling and panic recovery
func Example_errorHandling() {
	// Configure pool with custom panic handler
	config := workerpool.Config{
		WorkerCount: 2,
		QueueSize:   5,
		PanicHandler: func(task workerpool.Task, recovered interface{}) {
			// Custom panic handling
		},
	}

	pool := workerpool.NewWithConfig(config)
	defer pool.Shutdown()

	// Submit various types of tasks
	tasks := []workerpool.Task{
		workerpool.TaskFunc(func(ctx context.Context) error {
			return nil // Success
		}),
		workerpool.TaskFunc(func(ctx context.Context) error {
			return fmt.Errorf("task error") // Error
		}),
		workerpool.TaskFunc(func(ctx context.Context) error {
			panic("task panic") // Panic
		}),
	}

	for _, task := range tasks {
		pool.Submit(task)
	}

	// Process results
	for i := 0; i < len(tasks); i++ {
		<-pool.Results()
	}

	fmt.Println("Error handling example completed")

	// Output: Error handling example completed
}

// Example_dynamicWorkload demonstrates handling dynamic workloads
func Example_dynamicWorkload() {
	pool := workerpool.New(3, 100)
	defer pool.Shutdown()

	var tasksSubmitted, tasksCompleted int32

	// Simulate dynamic task submission
	go func() {
		for i := 0; i < 10; i++ {
			task := workerpool.TaskFunc(func(ctx context.Context) error {
				// Simulate work
				time.Sleep(10 * time.Millisecond)
				return nil
			})

			if err := pool.Submit(task); err != nil {
				log.Printf("Failed to submit task: %v", err)
			} else {
				atomic.AddInt32(&tasksSubmitted, 1)
			}

			time.Sleep(5 * time.Millisecond) // Stagger submissions
		}
	}()

	// Process results as they come
	go func() {
		for range pool.Results() {
			atomic.AddInt32(&tasksCompleted, 1)
		}
	}()

	// Wait for completion
	for atomic.LoadInt32(&tasksCompleted) < 10 {
		time.Sleep(5 * time.Millisecond)
	}

	fmt.Println("Dynamic workload processing completed")

	// Output: Dynamic workload processing completed
}

// Example_gracefulShutdown demonstrates graceful shutdown handling
func Example_gracefulShutdown() {
	pool := workerpool.New(2, 5)

	// Submit some long-running tasks
	for i := 0; i < 3; i++ {
		task := workerpool.TaskFunc(func(ctx context.Context) error {
			select {
			case <-time.After(100 * time.Millisecond):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		pool.Submit(task)
	}

	// Process some results
	go func() {
		for range pool.Results() {
			// Process results
		}
	}()

	// Wait a bit then shutdown
	time.Sleep(50 * time.Millisecond)

	// Graceful shutdown waits for running tasks to complete
	shutdownComplete := pool.Shutdown()
	<-shutdownComplete

	fmt.Println("Graceful shutdown completed")

	// Output: Graceful shutdown completed
}

// Example_loadBalancing demonstrates how tasks are distributed among workers
func Example_loadBalancing() {
	pool := workerpool.New(3, 10)
	defer pool.Shutdown()

	workerStats := make(map[int]int)
	var mu sync.Mutex

	// Submit tasks that track which worker executes them
	for i := 0; i < 9; i++ {
		task := workerpool.TaskFunc(func(ctx context.Context) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		})
		pool.Submit(task)
	}

	// Track worker distribution
	for i := 0; i < 9; i++ {
		result := <-pool.Results()
		mu.Lock()
		workerStats[result.WorkerID]++
		mu.Unlock()
	}

	fmt.Printf("Load balancing completed with %d workers\n", len(workerStats))

	// Output: Load balancing completed with 3 workers
}
