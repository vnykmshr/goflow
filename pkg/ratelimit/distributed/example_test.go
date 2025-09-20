package distributed

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/vnykmshr/goflow/pkg/ratelimit/bucket"
)

// Example_basicUsage demonstrates basic distributed rate limiting.
func Example_basicUsage() {
	// Create a Redis client (in real usage, use your Redis connection)
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Use a test database
	})
	defer func() { _ = rdb.Close() }()

	// Test Redis connection
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		fmt.Println("Redis not available, skipping example")
		return
	}

	// Create distributed token bucket limiter
	config := Config{
		Redis:      rdb,
		Key:        "api_limiter",
		Rate:       5.0, // 5 requests per second
		Burst:      10,  // burst of 10
		InstanceID: "example_instance_1",
	}

	limiter, err := NewLimiter(TokenBucket, config)
	if err != nil {
		log.Fatalf("Failed to create limiter: %v", err)
	}
	defer func() { _ = limiter.Close() }()

	fmt.Println("Testing distributed rate limiting:")

	// Test multiple requests
	for i := 0; i < 12; i++ {
		if limiter.Allow(ctx) {
			fmt.Printf("Request %d: Allowed\n", i+1)
		} else {
			fmt.Printf("Request %d: Denied\n", i+1)
		}
	}

	// Show stats
	stats, err := limiter.Stats(ctx)
	if err == nil {
		fmt.Printf("Total requests: %d, Allowed: %d, Denied: %d\n",
			stats.TotalRequests, stats.AllowedRequests, stats.DeniedRequests)
		fmt.Printf("Active instances: %v\n", stats.ActiveInstances)
	}

	// Clean up
	_ = limiter.Reset(ctx)

	// Output varies based on timing, but should show some allowed and some denied
}

// Example_multipleInstances demonstrates multiple application instances sharing a rate limit.
func Example_multipleInstances() {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1,
	})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		fmt.Println("Redis not available, skipping example")
		return
	}

	// Create two instances sharing the same rate limit
	baseConfig := Config{
		Redis: rdb,
		Key:   "shared_limiter",
		Rate:  3.0, // 3 requests per second total
		Burst: 5,
	}

	// Instance 1
	config1 := baseConfig
	config1.InstanceID = "instance_1"
	limiter1, err := NewLimiter(TokenBucket, config1)
	if err != nil {
		log.Fatalf("Failed to create limiter1: %v", err)
	}
	defer func() { _ = limiter1.Close() }()

	// Instance 2
	config2 := baseConfig
	config2.InstanceID = "instance_2"
	limiter2, err := NewLimiter(TokenBucket, config2)
	if err != nil {
		log.Fatalf("Failed to create limiter2: %v", err)
	}
	defer func() { _ = limiter2.Close() }()

	fmt.Println("Testing multiple instances:")

	// Both instances try to make requests
	for i := 0; i < 8; i++ {
		allowed1 := limiter1.Allow(ctx)
		allowed2 := limiter2.Allow(ctx)

		fmt.Printf("Round %d - Instance1: %v, Instance2: %v\n",
			i+1, allowed1, allowed2)

		time.Sleep(100 * time.Millisecond)
	}

	// Show combined stats
	stats, err := limiter1.Stats(ctx)
	if err == nil {
		fmt.Printf("Combined stats - Instances: %v\n", stats.ActiveInstances)
	}

	// Clean up
	_ = limiter1.Reset(ctx)
}

// Example_fallbackToLocal demonstrates fallback to local rate limiting.
func Example_fallbackToLocal() {
	// Create a Redis client that will fail to connect
	rdb := redis.NewClient(&redis.Options{
		Addr:        "localhost:9999", // Non-existent Redis server
		DialTimeout: 100 * time.Millisecond,
	})
	defer func() { _ = rdb.Close() }()

	// Create a local fallback limiter
	localLimiter, err := bucket.NewSafe(bucket.Limit(2), 5)
	if err != nil {
		panic(fmt.Sprintf("Failed to create local limiter: %v", err))
	}

	config := Config{
		Redis:           rdb,
		Key:             "test_limiter",
		Rate:            10.0,
		Burst:           20,
		FallbackToLocal: true,
		LocalLimiter:    localLimiter,
	}

	limiter, err := NewLimiter(TokenBucket, config)
	if err != nil {
		// If Redis is not available, we'd still get an error during creation
		// In practice, you might handle this differently
		fmt.Printf("Failed to create distributed limiter: %v\n", err)
		fmt.Println("Using local limiter fallback")

		for i := 0; i < 8; i++ {
			if localLimiter.Allow() {
				fmt.Printf("Local request %d: Allowed\n", i+1)
			} else {
				fmt.Printf("Local request %d: Denied\n", i+1)
			}
		}
		return
	}
	defer func() { _ = limiter.Close() }()

	ctx := context.Background()

	// Test requests - these should fallback to local limiter
	fmt.Println("Testing with fallback (requests will use local limiter):")
	for i := 0; i < 8; i++ {
		if limiter.Allow(ctx) {
			fmt.Printf("Request %d: Allowed (fallback)\n", i+1)
		} else {
			fmt.Printf("Request %d: Denied (fallback)\n", i+1)
		}
	}
}

// Example_slidingWindow demonstrates sliding window rate limiting.
func Example_slidingWindow() {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1,
	})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		fmt.Println("Redis not available, skipping example")
		return
	}

	config := Config{
		Redis:      rdb,
		Key:        "sliding_window_limiter",
		Rate:       3.0, // 3 requests per second
		Burst:      3,   // Same as rate for sliding window
		InstanceID: "sliding_example",
	}

	limiter, err := NewLimiter(SlidingWindow, config)
	if err != nil {
		log.Fatalf("Failed to create sliding window limiter: %v", err)
	}
	defer func() { _ = limiter.Close() }()

	fmt.Println("Testing sliding window rate limiting:")

	// Make requests over time to see sliding window behavior
	for i := 0; i < 10; i++ {
		if limiter.Allow(ctx) {
			fmt.Printf("Request %d: Allowed\n", i+1)
		} else {
			fmt.Printf("Request %d: Denied\n", i+1)
		}

		// Wait between requests to demonstrate sliding window
		if i == 4 {
			fmt.Println("Waiting 1 second...")
			time.Sleep(1 * time.Second)
		} else {
			time.Sleep(200 * time.Millisecond)
		}
	}

	// Clean up
	_ = limiter.Reset(ctx)
}

// Example_fixedWindow demonstrates fixed window rate limiting.
func Example_fixedWindow() {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1,
	})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		fmt.Println("Redis not available, skipping example")
		return
	}

	config := Config{
		Redis:      rdb,
		Key:        "fixed_window_limiter",
		Rate:       4.0, // 4 requests per second window
		Burst:      4,
		InstanceID: "fixed_example",
	}

	limiter, err := NewLimiter(FixedWindow, config)
	if err != nil {
		log.Fatalf("Failed to create fixed window limiter: %v", err)
	}
	defer func() { _ = limiter.Close() }()

	fmt.Println("Testing fixed window rate limiting:")

	// Make requests to demonstrate fixed window behavior
	for i := 0; i < 10; i++ {
		if limiter.Allow(ctx) {
			fmt.Printf("Request %d: Allowed\n", i+1)
		} else {
			fmt.Printf("Request %d: Denied\n", i+1)
		}

		// Wait after 5th request to enter new window
		if i == 4 {
			fmt.Println("Waiting for new window...")
			time.Sleep(1 * time.Second)
		} else {
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Clean up
	_ = limiter.Reset(ctx)
}

// Example_waitAndReserve demonstrates Wait and Reserve operations.
func Example_waitAndReserve() {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1,
	})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		fmt.Println("Redis not available, skipping example")
		return
	}

	config := Config{
		Redis:      rdb,
		Key:        "wait_reserve_limiter",
		Rate:       2.0, // 2 requests per second
		Burst:      3,
		InstanceID: "wait_example",
	}

	limiter, err := NewLimiter(TokenBucket, config)
	if err != nil {
		log.Fatalf("Failed to create limiter: %v", err)
	}
	defer func() { _ = limiter.Close() }()

	fmt.Println("Testing Wait and Reserve operations:")

	// Use up the burst capacity
	for i := 0; i < 3; i++ {
		limiter.Allow(ctx)
		fmt.Printf("Used burst capacity: %d/3\n", i+1)
	}

	// Try to reserve - should show delay needed
	reservation, err := limiter.Reserve(ctx, 1)
	if err == nil {
		fmt.Printf("Reservation: OK=%v, Delay=%v\n",
			reservation.OK, reservation.Delay)
	}

	// Wait for a request (with timeout)
	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	start := time.Now()
	err = limiter.Wait(waitCtx)
	elapsed := time.Since(start)

	if err == nil {
		fmt.Printf("Wait succeeded after %v\n", elapsed)
	} else {
		fmt.Printf("Wait failed: %v\n", err)
	}

	// Clean up
	_ = limiter.Reset(ctx)
}
