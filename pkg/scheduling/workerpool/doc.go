// Package workerpool provides a worker pool for executing tasks concurrently
// with a fixed number of goroutines and bounded queue.
//
// Basic usage:
//
//	pool := workerpool.New(4, 100)
//	defer func() { <-pool.Shutdown() }()
//
//	task := workerpool.TaskFunc(func(ctx context.Context) error {
//		// Do work here
//		return nil
//	})
//
//	pool.Submit(task)
//
//	// Process results
//	result := <-pool.Results()
//	if result.Error != nil {
//		log.Printf("Task failed: %v", result.Error)
//	}
package workerpool
