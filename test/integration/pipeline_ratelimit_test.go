// Package integration contains integration tests that verify cross-package functionality.
// These tests ensure that different components work together correctly in realistic scenarios.
package integration

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vnykmshr/goflow/internal/testutil"
	"github.com/vnykmshr/goflow/pkg/ratelimit/bucket"
	"github.com/vnykmshr/goflow/pkg/scheduling/pipeline"
	"github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

// TestPipelineWithRateLimiting verifies that a pipeline can properly integrate
// with rate limiting to control execution rate across stages.
func TestPipelineWithRateLimiting(t *testing.T) {
	// Create a rate limiter: 10 requests per second with burst of 5
	limiter, err := bucket.NewSafe(10, 5)
	if err != nil {
		t.Fatalf("failed to create rate limiter: %v", err)
	}

	var stageExecutions int32

	// Create a pipeline with a rate-limited stage
	p := pipeline.New()
	p.AddStageFunc("rate-limited-stage", func(ctx context.Context, input interface{}) (interface{}, error) {
		// Wait for rate limiter before processing
		if err := limiter.Wait(ctx); err != nil {
			return input, err
		}

		atomic.AddInt32(&stageExecutions, 1)
		return input, nil
	})

	// Execute multiple requests rapidly
	const numRequests = 20
	start := time.Now()

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			_, err := p.Execute(context.Background(), id)
			if err != nil {
				t.Errorf("execution %d failed: %v", id, err)
			}
		}(i)
	}

	// Wait for all executions to complete
	testutil.WaitForInt32(t, &stageExecutions, numRequests, 5*time.Second)

	elapsed := time.Since(start)

	// With rate limiting at 10 req/sec and burst of 5:
	// - First 5 requests complete immediately (burst)
	// - Remaining 15 requests take 1.5 seconds (at 10 req/sec)
	// Total should be around 1.5 seconds
	minExpected := 1 * time.Second // Should take at least 1 second
	maxExpected := 3 * time.Second // But not more than 3 seconds

	if elapsed < minExpected {
		t.Errorf("execution too fast: %v, rate limiting may not be working", elapsed)
	}
	if elapsed > maxExpected {
		t.Errorf("execution too slow: %v, something may be wrong", elapsed)
	}

	t.Logf("Executed %d requests in %v (rate limited to 10 req/sec)", numRequests, elapsed)
}

// TestPipelineWithWorkerPool verifies that pipeline stages can use a worker pool
// to control concurrency and process items efficiently.
func TestPipelineWithWorkerPool(t *testing.T) {
	// Create a worker pool with 3 workers
	pool := workerpool.New(3, 20) //nolint:staticcheck // OK in tests
	defer func() { <-pool.Shutdown() }()

	// Consume results in background
	go func() {
		for range pool.Results() {
			// Drain results
		}
	}()

	var processedCount int32

	// Create pipeline that uses the worker pool
	p := pipeline.New().SetWorkerPool(pool)

	p.AddStageFunc("processing-stage", func(_ context.Context, input interface{}) (interface{}, error) {
		// Simulate some processing work
		time.Sleep(10 * time.Millisecond)
		atomic.AddInt32(&processedCount, 1)
		return input, nil
	})

	// Execute multiple items
	const numItems = 10
	var wg sync.WaitGroup
	wg.Add(numItems)
	for i := 0; i < numItems; i++ {
		go func(id int) {
			defer wg.Done()
			_, err := p.Execute(context.Background(), id)
			if err != nil {
				t.Errorf("execution %d failed: %v", id, err)
			}
		}(i)
	}

	// Wait for all Execute calls to complete
	wg.Wait()

	stats := p.Stats()
	if stats.TotalExecutions != numItems {
		t.Errorf("total executions = %d, want %d", stats.TotalExecutions, numItems)
	}
	if stats.SuccessfulRuns != numItems {
		t.Errorf("successful runs = %d, want %d", stats.SuccessfulRuns, numItems)
	}

	t.Logf("Processed %d items using %d workers", numItems, pool.Size())
}

// TestConcurrentRateLimiting tests that rate limiting works correctly
// when accessed concurrently from multiple goroutines.
func TestConcurrentRateLimiting(t *testing.T) {
	limiter, err := bucket.NewSafe(100, 50) // 100 req/sec, burst 50
	if err != nil {
		t.Fatalf("failed to create rate limiter: %v", err)
	}

	const goroutines = 10
	const requestsPerGoroutine = 20
	const totalRequests = goroutines * requestsPerGoroutine

	var allowed, denied int32

	done := make(chan bool, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer func() { done <- true }()

			for j := 0; j < requestsPerGoroutine; j++ {
				if limiter.Allow() {
					atomic.AddInt32(&allowed, 1)
				} else {
					atomic.AddInt32(&denied, 1)
				}
			}
		}()
	}

	// Wait for all goroutines
	for i := 0; i < goroutines; i++ {
		<-done
	}

	totalProcessed := atomic.LoadInt32(&allowed) + atomic.LoadInt32(&denied)
	if totalProcessed != totalRequests {
		t.Errorf("total processed = %d, want %d", totalProcessed, totalRequests)
	}

	// At least burst amount should be allowed immediately
	if atomic.LoadInt32(&allowed) < 50 {
		t.Errorf("allowed = %d, expected at least 50 (burst size)", allowed)
	}

	t.Logf("Concurrent rate limiting: %d allowed, %d denied out of %d requests",
		allowed, denied, totalRequests)
}

// TestPipelineErrorHandling verifies that errors are properly handled
// when propagating through a pipeline with multiple stages.
func TestPipelineErrorHandling(t *testing.T) {
	var stage1Executed, stage2Executed, stage3Executed int32

	p := pipeline.New()

	p.AddStageFunc("stage1", func(_ context.Context, input interface{}) (interface{}, error) {
		atomic.AddInt32(&stage1Executed, 1)
		return input, nil
	})

	p.AddStageFunc("stage2-fails", func(_ context.Context, input interface{}) (interface{}, error) {
		atomic.AddInt32(&stage2Executed, 1)
		return input, context.Canceled // Simulate an error
	})

	p.AddStageFunc("stage3", func(_ context.Context, input interface{}) (interface{}, error) {
		atomic.AddInt32(&stage3Executed, 1)
		return input, nil
	})

	result, err := p.Execute(context.Background(), "test")

	// Error should propagate
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Stage 1 and 2 should execute, stage 3 should not (default StopOnError)
	if atomic.LoadInt32(&stage1Executed) != 1 {
		t.Error("stage1 should have executed")
	}
	if atomic.LoadInt32(&stage2Executed) != 1 {
		t.Error("stage2 should have executed")
	}
	if atomic.LoadInt32(&stage3Executed) != 0 {
		t.Error("stage3 should not have executed due to error in stage2")
	}

	if result == nil {
		t.Fatal("result should not be nil even with error")
	}

	// Should have executed 2 stages
	if len(result.StageResults) != 2 {
		t.Errorf("stage results = %d, want 2", len(result.StageResults))
	}

	t.Logf("Error handling test passed: pipeline stopped after stage2 error")
}
