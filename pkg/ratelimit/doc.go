/*
Package ratelimit provides rate limiting primitives for Go applications.

This package offers three main types of rate limiters:

  - bucket: Token bucket rate limiter allowing burst traffic
  - leakybucket: Leaky bucket rate limiter for smooth traffic flow
  - concurrency: Concurrency limiter for controlling goroutine usage

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

Both limiters support:
  - Context-aware blocking operations (Wait/WaitN)
  - Reservation patterns for advance booking
  - Dynamic configuration changes
  - Comprehensive state inspection

All rate limiters are safe for concurrent use and integrate with
the context package for cancellation and timeouts.
*/
package ratelimit
