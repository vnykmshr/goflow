/*
Package distributed provides distributed rate limiting using Redis for coordination across multiple instances.

Enable global rate limiting for microservices and distributed systems.

# Quick Start

	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

	config := distributed.Config{
		Redis:      rdb,
		Key:        "api_limiter",
		Rate:       100.0, // 100 requests per second
		Burst:      200,
		InstanceID: "server-1",
	}

	limiter, err := distributed.NewLimiter(distributed.TokenBucket, config)
	if err != nil {
		log.Fatal(err)
	}
	defer limiter.Close()

	if limiter.Allow(context.Background()) {
		// Process request
	}

# Strategies

Token Bucket (burst traffic allowed):

	limiter, _ := distributed.NewLimiter(distributed.TokenBucket, config)

Sliding Window (smooth rate limiting):

	limiter, _ := distributed.NewLimiter(distributed.SlidingWindow, config)

Fixed Window (simple counters):

	limiter, _ := distributed.NewLimiter(distributed.FixedWindow, config)

# Configuration

	config := distributed.Config{
		Redis:           rdb,                    // Required
		Key:            "my_limiter",           // Required
		Rate:           50.0,                   // Required
		Burst:          100,                    // Required
		InstanceID:      "server-1",            // Auto-generated if empty
		FallbackToLocal: true,                  // Enable local fallback
		LocalLimiter:    localLimiter,          // Fallback limiter
		RedisTimeout:    500 * time.Millisecond,
		KeyTTL:         time.Hour,
	}

# Operations

	// Non-blocking
	if limiter.Allow(ctx) { ... }
	if limiter.AllowN(ctx, 5) { ... }

	// Blocking
	err := limiter.Wait(ctx)
	err := limiter.WaitN(ctx, 3)

	// Reservations
	reservation, err := limiter.Reserve(ctx, 2)
	if reservation.OK && reservation.Delay > 0 {
		time.Sleep(reservation.Delay)
	}

	// Runtime changes
	limiter.SetRate(ctx, 150.0)
	limiter.SetBurst(ctx, 300)

	// Statistics
	stats, err := limiter.Stats(ctx)

# Multiple Instances

	// Instance 1
	config1 := config
	config1.InstanceID = "server-1"
	limiter1, _ := distributed.NewLimiter(distributed.TokenBucket, config1)

	// Instance 2
	config2 := config
	config2.InstanceID = "server-2"
	limiter2, _ := distributed.NewLimiter(distributed.TokenBucket, config2)

	// Both share the same global rate limit

# Fallback Strategy

	localLimiter, _ := bucket.NewSafe(bucket.Limit(100), 200)

	config := distributed.Config{
		Redis:           rdb,
		FallbackToLocal: true,
		LocalLimiter:    localLimiter,
	}

# Error Handling

	if err != nil {
		var configErr *distributed.ConfigError
		var redisErr *distributed.RedisError

		switch {
		case errors.As(err, &configErr):
			// Configuration error
		case errors.As(err, &redisErr):
			// Redis error - may fall back to local
		}
	}

# Production

Redis High Availability:

	rdb := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs: []string{"redis1:6379", "redis2:6379", "redis3:6379"},
	})

Monitoring:

	stats, err := limiter.Stats(ctx)
	if err == nil {
		metrics.Rate.Set(stats.Rate)
		metrics.ActiveInstances.Set(float64(len(stats.ActiveInstances)))
	}

Graceful Shutdown:

	limiter.Close() // Removes instance from active set

# Security Considerations

WARNING: PRODUCTION DEPLOYMENTS MUST SECURE REDIS CONNECTIONS

## Redis Authentication

	// Enable Redis authentication
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: os.Getenv("REDIS_PASSWORD"), // Never hardcode passwords
		DB:       0,
	})

## TLS Encryption

	// Use TLS for production
	import "crypto/tls"

	rdb := redis.NewClient(&redis.Options{
		Addr: "redis.example.com:6380",
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	})

## Network Isolation

	// Restrict Redis access via firewall rules:
	// - Only allow application servers to connect
	// - Use private networks (VPC) when possible
	// - Never expose Redis to public internet

## Multi-Tenant Isolation

	// Use environment-specific key prefixes
	config := distributed.Config{
		Key: fmt.Sprintf("%s:ratelimit:api", environment), // prod:ratelimit:api
	}

## Fallback Behavior Warning

CRITICAL: When FallbackToLocal=true and Redis fails, rate limits become
per-instance instead of global:

	// If you have N instances with limit R:
	// - Normal: R total requests/sec globally
	// - Redis down: N × R requests/sec (each instance allows R)

	// Example with 10 instances, 100 RPS limit:
	// - Normal: 100 RPS total (distributed)
	// - Redis down: 1000 RPS total (10 × 100)

	// For security-critical limits, disable fallback:
	config := distributed.Config{
		FallbackToLocal: false, // Deny requests if Redis unavailable
	}

# Thread Safety

All operations are safe for concurrent use from multiple goroutines.

See example tests for more usage patterns.
*/
package distributed
