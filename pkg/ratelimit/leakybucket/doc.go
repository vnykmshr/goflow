/*
Package leakybucket provides leaky bucket rate limiting for Go applications.

A leaky bucket enforces a constant smooth output rate by "leaking" requests
at a fixed interval, unlike token bucket which allows controlled bursts.
This makes it ideal for traffic shaping and ensuring predictable resource
consumption.

Basic usage:

	limiter := leakybucket.New(5, 10) // 5 requests/sec leak rate, capacity 10
	if limiter.Allow() {
		// Process request
	}

Key Characteristics:

The leaky bucket algorithm provides smooth traffic flow by:
  - Accepting requests up to capacity when space is available
  - "Leaking" requests at a constant rate regardless of input patterns
  - Buffering burst traffic and processing it at a steady rate
  - Preventing downstream systems from being overwhelmed

Comparison with Token Bucket:

	// Token Bucket: Allows bursts, starts with full tokens
	tokenLimiter := bucket.New(5, 10) // Allows immediate burst of 10

	// Leaky Bucket: Smooth flow, starts empty
	leakyLimiter := leakybucket.New(5, 10) // Builds up to capacity gradually

Use Cases:

Leaky bucket is ideal for:
  - Video streaming and media processing
  - Network traffic shaping
  - Database write operations
  - Any scenario requiring predictable resource usage

Token bucket is better for:
  - Interactive web applications
  - API rate limiting with burst tolerance
  - Variable load handling

Configuration Options:

	config := leakybucket.Config{
		LeakRate:     10,    // Requests per second
		Capacity:     20,    // Maximum requests to buffer
		InitialLevel: 5,     // Start with some requests in bucket
		Clock:        clock, // Custom time source (for testing)
	}
	limiter := leakybucket.NewWithConfig(config)

Advanced Features:

The limiter supports context-aware operations:

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := limiter.Wait(ctx); err != nil {
		// Handle timeout or cancellation
	}

Reservation pattern for advance booking:

	reservation := limiter.Reserve()
	if reservation.OK() {
		delay := reservation.Delay()
		// Wait for delay or cancel reservation
		reservation.Cancel()
	}

Dynamic configuration changes:

	limiter.SetLeakRate(20)    // Change processing rate
	limiter.SetCapacity(50)    // Change buffer size

State inspection:

	level := limiter.Level()         // Current fill level
	available := limiter.Available() // Available space
	rate := limiter.LeakRate()       // Current leak rate
	capacity := limiter.Capacity()   // Maximum capacity

Thread Safety:

All operations are safe for concurrent use. The limiter uses mutex-based
synchronization to protect internal state while maintaining good performance
for high-throughput scenarios.
*/
package leakybucket
