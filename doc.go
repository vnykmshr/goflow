/*
Package goflow provides a comprehensive Go Async/IO Toolkit that brings high-level,
opinionated, and powerful asynchronous patterns for building robust concurrent applications.

Instead of mimicking event-driven, future-based models, goflow provides a set of
idiomatic Go primitives that leverage standard concurrency features like goroutines,
channels, and the context package.

The toolkit consists of three main modules:

Rate Limiting (pkg/ratelimit):
  - bucket: Token bucket-based rate limiter allowing burst traffic
  - leakybucket: Leaky bucket-based rate limiter for smooth traffic flow
  - concurrency: Concurrency limiter to control goroutine usage

Task Scheduling (pkg/scheduling):
  - pool: Worker pool implementation with graceful shutdown
  - scheduler: Goroutine scheduler for time-based task execution
  - pipeline: Data pipeline builder for multi-stage concurrent processing

Streaming & I/O (pkg/streaming):
  - stream: High-level data stream API with common operations
  - writer: Asynchronous writer with background buffering
  - backpressure: Backpressure-aware channels for flow control

All components are designed to be:
  - Idiomatic: Uses standard Go concurrency primitives
  - Simple: Clean APIs that abstract common boilerplate
  - Performant: High-performance and low overhead design
  - Modular: Loosely coupled packages for selective imports
  - Robust: Handles failures gracefully with context integration

Example usage:

	import (
		"context"
		"github.com/vnykmshr/goflow/pkg/ratelimit/bucket"
		"github.com/vnykmshr/goflow/pkg/scheduling/pool"
	)

	// Create a rate limiter (10 tokens, refill at 1/second)
	limiter := bucket.New(10, 1.0)

	// Create a worker pool
	workerPool := pool.New(5)

	// Use with context
	ctx := context.Background()
	if limiter.Allow(ctx) {
		// Process request
		workerPool.Submit(ctx, task)
	}
*/
package goflow
