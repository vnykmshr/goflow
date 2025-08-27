/*
Package ratelimit provides rate limiting primitives for Go applications.

This package offers three main types of rate limiters:

  - bucket: Token bucket rate limiter allowing burst traffic
  - leakybucket: Leaky bucket rate limiter for smooth traffic flow
  - concurrency: Concurrency limiter for controlling concurrent operations

Rate Limiter Types:

Token Bucket vs Leaky Bucket:

Token bucket allows controlled bursts and is ideal for interactive applications:

	tokenLimiter := bucket.New(10, 5) // 10 tokens/sec, burst of 5
	if tokenLimiter.Allow() {
		// Process request (allows immediate burst)
	}

Leaky bucket enforces smooth flow and is ideal for traffic shaping:

	leakyLimiter := leakybucket.New(10, 5) // 10 requests/sec, capacity 5
	if leakyLimiter.Allow() {
		// Process request (smooth flow, no bursts)
	}

Concurrency limiter controls the number of simultaneous operations:

	concLimiter := concurrency.New(10) // Max 10 concurrent operations
	if concLimiter.Acquire() {
		defer concLimiter.Release()
		// Process operation (limited by concurrency, not time)
	}

All rate limiters support:
  - Context-aware blocking operations (Wait/WaitN)
  - Dynamic configuration changes
  - Comprehensive state inspection
  - Safe concurrent access

Rate vs Concurrency Limiting:
  - Rate limiters control operations per unit of time
  - Concurrency limiters control simultaneous operations
  - Rate limiters include reservation patterns for advance booking
  - Concurrency limiters provide semaphore-like functionality

All rate limiters integrate with the context package for cancellation and timeouts.
*/
package ratelimit
