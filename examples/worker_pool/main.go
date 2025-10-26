// Worker pool example demonstrating background task processing
package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

func main() {
	// Create worker pool with 3 workers and queue size 10
	pool, err := workerpool.NewSafe(3, 10)
	if err != nil {
		log.Fatal(err)
	}

	// Ensure graceful shutdown
	defer func() {
		fmt.Println("Shutting down worker pool...")
		<-pool.Shutdown()
		fmt.Println("Worker pool shut down complete")
	}()

	// Example 1: Simple tasks
	fmt.Println("Submitting simple tasks...")
	for i := 1; i <= 5; i++ {
		taskID := i
		task := workerpool.TaskFunc(func(_ context.Context) error {
			// Simulate work with cryptographically secure random duration
			randNum, err := rand.Int(rand.Reader, big.NewInt(1000))
			if err != nil {
				return fmt.Errorf("failed to generate random duration: %w", err)
			}
			duration := time.Duration(randNum.Int64()+500) * time.Millisecond
			fmt.Printf("Task %d: working for %v\n", taskID, duration)
			time.Sleep(duration)
			fmt.Printf("Task %d: completed\n", taskID)
			return nil
		})

		if err := pool.Submit(task); err != nil {
			log.Printf("Failed to submit task %d: %v", taskID, err)
		}
	}

	// Example 2: Tasks with results
	fmt.Println("\nSubmitting tasks with results...")
	for i := 1; i <= 3; i++ {
		taskID := i
		task := workerpool.TaskFunc(func(_ context.Context) error {
			result := taskID * taskID
			fmt.Printf("Calculation task %d: %d^2 = %d\n", taskID, taskID, result)
			return nil
		})

		if err := pool.Submit(task); err != nil {
			log.Printf("Failed to submit calculation task %d: %v", taskID, err)
		}
	}

	// Example 3: Error handling
	fmt.Println("\nSubmitting task that will fail...")
	errorTask := workerpool.TaskFunc(func(_ context.Context) error {
		fmt.Println("Error task: simulating failure")
		return fmt.Errorf("simulated error")
	})

	if err := pool.Submit(errorTask); err != nil {
		log.Printf("Failed to submit error task: %v", err)
	}

	// Wait for all tasks to complete
	fmt.Println("\nWaiting for tasks to complete...")
	time.Sleep(3 * time.Second)

	// Note: Pool stats may vary by implementation
	fmt.Printf("\nWorker pool processing completed")

	fmt.Println("\nExample completed. Worker pool will shut down...")
}
