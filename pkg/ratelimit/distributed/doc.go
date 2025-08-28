// Package distributed provides distributed rate limiting using Redis as the coordination backend.
//
// This package enables rate limiting across multiple application instances, which is essential
// for microservices architectures and distributed systems where you need to enforce a global
// rate limit rather than per-instance limits.
//
// # Overview
//
// The distributed package implements three rate limiting strategies:
//
//   - TokenBucket: Distributed token bucket allowing burst traffic
//   - SlidingWindow: Smooth rate limiting with sliding time windows
//   - FixedWindow: Simple fixed time window counters
//
// All strategies use Redis for coordination and provide fallback to local rate limiting
// when Redis is unavailable.
//
// # Quick Start
//
// Basic distributed rate limiting:
//
//	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
//
//	config := distributed.Config{
//		Redis:      rdb,
//		Key:        "api_limiter",
//		Rate:       100.0, // 100 requests per second
//		Burst:      200,   // burst up to 200
//		InstanceID: "server-1",
//	}
//
//	limiter, err := distributed.NewLimiter(distributed.TokenBucket, config)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer limiter.Close()
//
//	ctx := context.Background()
//	if limiter.Allow(ctx) {
//		// Process request
//	}
//
// # Multiple Instances
//
// Multiple application instances can share the same rate limit:
//
//	// Instance 1
//	config1 := config
//	config1.InstanceID = "server-1"
//	limiter1, _ := distributed.NewLimiter(distributed.TokenBucket, config1)
//
//	// Instance 2
//	config2 := config
//	config2.InstanceID = "server-2"
//	limiter2, _ := distributed.NewLimiter(distributed.TokenBucket, config2)
//
//	// Both instances share the same global rate limit
//
// # Fallback Strategy
//
// Enable local fallback when Redis is unavailable:
//
//	localLimiter := bucket.New(bucket.Limit(100), 200)
//
//	config := distributed.Config{
//		Redis:           rdb,
//		Key:            "api_limiter",
//		Rate:           100.0,
//		Burst:          200,
//		FallbackToLocal: true,
//		LocalLimiter:    localLimiter,
//	}
//
// # Rate Limiting Strategies
//
// ## Token Bucket
//
// The token bucket strategy allows for burst traffic while maintaining an average rate:
//
//	limiter, _ := distributed.NewLimiter(distributed.TokenBucket, config)
//
//	// Allows bursts up to 'Burst' size, then sustains 'Rate' requests/second
//
// Features:
//   - Supports burst traffic
//   - Smooth rate limiting over time
//   - Atomic Redis operations via Lua scripts
//   - Automatic token refill based on elapsed time
//
// ## Sliding Window
//
// The sliding window strategy provides smooth rate limiting without fixed window boundaries:
//
//	limiter, _ := distributed.NewLimiter(distributed.SlidingWindow, config)
//
//	// Maintains exactly 'Rate' requests per second over any 1-second period
//
// Features:
//   - No thundering herd effect at window boundaries
//   - Precise rate limiting over sliding time periods
//   - Uses Redis sorted sets for efficient time-based operations
//   - Automatic cleanup of expired entries
//
// ## Fixed Window
//
// The fixed window strategy uses discrete time windows for simple rate limiting:
//
//	limiter, _ := distributed.NewLimiter(distributed.FixedWindow, config)
//
//	// Allows 'Rate' requests per fixed 1-second window
//
// Features:
//   - Simple and efficient implementation
//   - Predictable reset behavior
//   - Memory efficient
//   - Can allow brief bursts at window boundaries
//
// # Configuration
//
// Comprehensive configuration options:
//
//	config := distributed.Config{
//		Redis:           rdb,                    // Required: Redis client
//		Key:            "my_limiter",           // Required: Redis key prefix
//		Rate:           50.0,                   // Required: Requests per second
//		Burst:          100,                    // Required: Burst capacity
//		InstanceID:      "server-1",            // Auto-generated if empty
//		FallbackToLocal: true,                  // Enable local fallback
//		LocalLimiter:    bucket.New(50, 100),   // Local fallback limiter
//		RedisTimeout:    500 * time.Millisecond, // Redis operation timeout
//		RefreshInterval: 100 * time.Millisecond, // Refresh frequency for Wait operations
//		KeyTTL:         time.Hour,              // Redis key expiration
//	}
//
// # Operations
//
// ## Basic Operations
//
//	// Non-blocking check
//	if limiter.Allow(ctx) {
//		// Request allowed
//	}
//
//	// Check for N requests
//	if limiter.AllowN(ctx, 5) {
//		// 5 requests allowed
//	}
//
// ## Blocking Operations
//
//	// Wait for request to be allowed
//	err := limiter.Wait(ctx)
//	if err == nil {
//		// Request now allowed
//	}
//
//	// Wait for N requests
//	err := limiter.WaitN(ctx, 3)
//
// ## Advanced Operations
//
//	// Reserve capacity and get timing information
//	reservation, err := limiter.Reserve(ctx, 2)
//	if err == nil && reservation.OK {
//		if reservation.Delay > 0 {
//			time.Sleep(reservation.Delay)
//		}
//		// Process 2 requests
//	}
//
// ## Runtime Configuration
//
//	// Change rate limit across all instances
//	limiter.SetRate(ctx, 150.0)
//
//	// Change burst capacity
//	limiter.SetBurst(ctx, 300)
//
//	// Get current statistics
//	stats, err := limiter.Stats(ctx)
//	if err == nil {
//		fmt.Printf("Rate: %.1f, Tokens: %.1f, Instances: %v\n",
//			stats.Rate, stats.Tokens, stats.ActiveInstances)
//	}
//
// # Redis Data Structures
//
// The package uses efficient Redis data structures:
//
//   - Token Bucket: String values for tokens and timestamps, Hash for config/stats
//   - Sliding Window: Sorted sets with time-based scores for efficient windowing
//   - Fixed Window: String counters with automatic expiration
//   - All strategies: Sets for instance tracking, Hashes for configuration
//
// # Error Handling
//
// The package provides specific error types:
//
//	if err != nil {
//		var configErr *distributed.ConfigError
//		var redisErr *distributed.RedisError
//
//		switch {
//		case errors.As(err, &configErr):
//			// Configuration error
//		case errors.As(err, &redisErr):
//			// Redis operation error - may fall back to local limiter
//		default:
//			// Other error
//		}
//	}
//
// # Performance Characteristics
//
// ## Token Bucket
//   - Redis operations: 1 Lua script execution per request
//   - Memory usage: O(1) per limiter
//   - Precision: High, with automatic token refill calculation
//
// ## Sliding Window
//   - Redis operations: 1 Lua script + sorted set operations
//   - Memory usage: O(requests in window)
//   - Precision: Highest, exact sliding window behavior
//
// ## Fixed Window
//   - Redis operations: 1 Lua script execution per request
//   - Memory usage: O(1) per active window
//   - Precision: Good within window boundaries
//
// # Production Considerations
//
// ## Redis High Availability
//
// For production use, consider Redis clustering or sentinel:
//
//	rdb := redis.NewClusterClient(&redis.ClusterOptions{
//		Addrs: []string{"redis1:6379", "redis2:6379", "redis3:6379"},
//	})
//
// ## Monitoring
//
// Monitor distributed rate limiter performance:
//
//	stats, err := limiter.Stats(ctx)
//	if err == nil {
//		// Export to metrics system
//		metrics.Rate.Set(stats.Rate)
//		metrics.TokensAvailable.Set(stats.Tokens)
//		metrics.ActiveInstances.Set(float64(len(stats.ActiveInstances)))
//	}
//
// ## Graceful Shutdown
//
//	// Remove instance from active set
//	limiter.Close()
//
// ## Key Management
//
// Use meaningful key prefixes to avoid conflicts:
//
//	config := distributed.Config{
//		Key: "myapp:api:v1:limiter",  // Clear key hierarchy
//		// ... other config
//	}
//
// # Examples
//
// See the example tests for comprehensive usage patterns:
//   - Example_basicUsage: Simple distributed rate limiting
//   - Example_multipleInstances: Multiple app instances sharing limits
//   - Example_fallbackToLocal: Local fallback when Redis unavailable
//   - Example_slidingWindow: Sliding window rate limiting
//   - Example_fixedWindow: Fixed window rate limiting
//   - Example_waitAndReserve: Advanced Wait and Reserve operations
package distributed
