// Package main demonstrates comprehensive metrics collection across all goflow components.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/vnykmshr/goflow/pkg/ratelimit/bucket"
	"github.com/vnykmshr/goflow/pkg/ratelimit/concurrency"
	"github.com/vnykmshr/goflow/pkg/scheduling/scheduler"
	"github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

func main() {
	fmt.Println("Starting goflow metrics demonstration...")

	// Start metrics HTTP server
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Println("Metrics server starting on :8080/metrics")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Printf("Metrics server failed: %v", err)
		}
	}()

	demonstrateMetrics()
	
	// Keep server running for a bit to examine metrics
	fmt.Println("Metrics server running at http://localhost:8080/metrics")
	fmt.Println("Press Ctrl+C to exit...")
	time.Sleep(30 * time.Second)
}

func demonstrateMetrics() {
	// Create rate limiter with metrics
	rateLimiter := bucket.NewWithMetrics(5, 10, "api_limiter")
	
	// Create concurrency limiter with metrics  
	concurrencyLimiter := concurrency.NewWithMetrics(3, "concurrent_operations")
	
	// Create worker pool with metrics
	pool := workerpool.NewWithMetrics(2, "task_pool")
	defer func() { <-pool.Shutdown() }()
	
	// Create scheduler with metrics
	sched := scheduler.NewWithMetrics("task_scheduler")
	defer func() { <-sched.Stop() }()
	sched.Start()

	_ = context.Background()

	fmt.Println("=== Testing Rate Limiter ===")
	// Test rate limiter - some will be allowed, some denied
	for i := 0; i < 15; i++ {
		if rateLimiter.Allow() {
			fmt.Printf("Request %d: Allowed\n", i+1)
		} else {
			fmt.Printf("Request %d: Denied\n", i+1)
		}
	}

	fmt.Println("\n=== Testing Concurrency Limiter ===")
	// Test concurrency limiter
	for i := 0; i < 5; i++ {
		if concurrencyLimiter.Acquire() {
			fmt.Printf("Operation %d: Started\n", i+1)
			go func(id int) {
				time.Sleep(100 * time.Millisecond)
				concurrencyLimiter.Release()
				fmt.Printf("Operation %d: Completed\n", id)
			}(i + 1)
		} else {
			fmt.Printf("Operation %d: Rejected\n", i+1)
		}
	}

	fmt.Println("\n=== Testing Worker Pool ===")
	// Test worker pool
	for i := 0; i < 8; i++ {
		task := workerpool.TaskFunc(func(ctx context.Context) error {
			time.Sleep(50 * time.Millisecond)
			return nil
		})
		
		err := pool.Submit(task)
		if err != nil {
			fmt.Printf("Task %d: Failed to submit - %v\n", i+1, err)
		} else {
			fmt.Printf("Task %d: Submitted\n", i+1)
		}
	}

	fmt.Println("\n=== Testing Scheduler ===")
	// Test scheduler
	schedulerTask := workerpool.TaskFunc(func(ctx context.Context) error {
		fmt.Println("Scheduled task executed")
		return nil
	})
	
	sched.ScheduleAfter("test_task", schedulerTask, 200*time.Millisecond)
	sched.ScheduleRepeating("repeat_task", schedulerTask, 300*time.Millisecond, 3)

	// Wait for operations to complete
	time.Sleep(2 * time.Second)

	fmt.Printf("\nFinal Status:\n")
	fmt.Printf("Rate Limiter tokens remaining: %.1f\n", rateLimiter.Tokens())
	fmt.Printf("Concurrency limiter in use: %d\n", concurrencyLimiter.InUse())
	fmt.Printf("Worker pool - Size: %d, Active: %d, Queued: %d\n", 
		pool.Size(), pool.ActiveWorkers(), pool.QueueSize())
	fmt.Printf("Scheduler stats: %+v\n", sched.Stats())

	fmt.Println("\nAll metrics are now available at http://localhost:8080/metrics")
}