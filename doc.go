/*
Package goflow provides a Go library for concurrent applications with rate limiting,
task scheduling, and streaming.

Rate Limiting (pkg/ratelimit):
  - bucket: Token bucket rate limiter with burst capacity
  - leakybucket: Smooth rate limiting without bursts
  - concurrency: Control concurrent operations
  - distributed: Multi-instance rate limiting with Redis

Task Scheduling (pkg/scheduling):
  - workerpool: Background task processing
  - scheduler: Cron and interval-based scheduling
  - pipeline: Multi-stage data processing

Streaming (pkg/streaming):
  - stream: Functional data operations
  - writer: Async buffered writing

Example usage:

	import (
		"github.com/vnykmshr/goflow/pkg/ratelimit/bucket"
		"github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
	)

	limiter, _ := bucket.NewSafe(10, 20) // 10 RPS, burst 20
	pool := workerpool.New(5, 100) // 5 workers, queue 100

	if limiter.Allow() {
		pool.Submit(task)
	}
*/
package goflow
