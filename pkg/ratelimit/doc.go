/*
Package ratelimit provides rate limiting primitives for Go applications.

This package offers three main types of rate limiters:

  - bucket: Token bucket rate limiter allowing burst traffic
  - leakybucket: Leaky bucket rate limiter for smooth traffic flow
  - concurrency: Concurrency limiter for controlling goroutine usage

Basic usage:

	limiter := bucket.New(10, 1.0) // 10 tokens, 1 per second
	if limiter.Allow(ctx) {
	    // Process request
	}

All rate limiters are safe for concurrent use and integrate with
the context package for cancellation and timeouts.
*/
package ratelimit
