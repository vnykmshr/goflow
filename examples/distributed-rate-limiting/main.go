// Package main demonstrates distributed rate limiting across multiple application instances.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/vnykmshr/goflow/pkg/ratelimit/bucket"
	"github.com/vnykmshr/goflow/pkg/ratelimit/distributed"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "server" {
		runHTTPServer()
		return
	}

	fmt.Println("Goflow Distributed Rate Limiting Demo")
	fmt.Println("====================================")

	// Check if Redis is available
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   0,
	})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		fmt.Printf("Redis not available: %v\n", err)
		fmt.Println("Please start Redis server: redis-server")
		fmt.Println("Or run: docker run -p 6379:6379 redis")
		return
	}

	// Clear any existing test data
	rdb.FlushDB(ctx)

	// Run all demonstrations
	demonstrateBasicDistributedLimiting(rdb)
	demonstrateMultipleInstances(rdb)
	demonstrateStrategies(rdb)
	demonstrateFallback()
	demonstrateLoadTesting(rdb)

	fmt.Println("\nDemo completed! Check the examples above.")
	fmt.Println("To run HTTP server demo: go run main.go server")
}

// demonstrateBasicDistributedLimiting shows basic distributed rate limiting.
func demonstrateBasicDistributedLimiting(rdb *redis.Client) {
	fmt.Println("\n1. Basic Distributed Rate Limiting")
	fmt.Println("----------------------------------")

	config := distributed.Config{
		Redis:      rdb,
		Key:        "demo:basic",
		Rate:       3.0, // 3 requests per second
		Burst:      5,   // burst of 5
		InstanceID: "demo-instance",
	}

	limiter, err := distributed.NewLimiter(distributed.TokenBucket, config)
	if err != nil {
		log.Fatalf("Failed to create limiter: %v", err)
	}
	defer func() { _ = limiter.Close() }()

	ctx := context.Background()

	fmt.Printf("Rate: %.1f req/sec, Burst: %d\n", config.Rate, config.Burst)

	// Test burst behavior
	fmt.Println("Testing burst capacity:")
	allowed := 0
	for i := 0; i < 8; i++ {
		if limiter.Allow(ctx) {
			allowed++
			fmt.Printf("Request %d: ✓ Allowed\n", i+1)
		} else {
			fmt.Printf("Request %d: ✗ Denied\n", i+1)
		}
	}

	fmt.Printf("Burst test: %d/8 requests allowed\n", allowed)

	// Show stats
	stats, err := limiter.Stats(ctx)
	if err == nil {
		fmt.Printf("Stats: Requests=%d, Allowed=%d, Denied=%d, Instances=%v\n",
			stats.TotalRequests, stats.AllowedRequests, stats.DeniedRequests, stats.ActiveInstances)
	}

	// Clean up
	if err := limiter.Reset(ctx); err != nil {
		fmt.Printf("Warning: Failed to reset limiter: %v\n", err)
	}
}

// demonstrateMultipleInstances shows multiple instances sharing a rate limit.
func demonstrateMultipleInstances(rdb *redis.Client) {
	fmt.Println("\n2. Multiple Instances Sharing Rate Limit")
	fmt.Println("---------------------------------------")

	baseConfig := distributed.Config{
		Redis: rdb,
		Key:   "demo:multi",
		Rate:  2.0, // 2 requests per second TOTAL across all instances
		Burst: 4,   // 4 requests burst TOTAL
	}

	// Create 3 instances
	limiters := make([]distributed.Limiter, 3)
	for i := 0; i < 3; i++ {
		config := baseConfig
		config.InstanceID = fmt.Sprintf("instance-%d", i+1)

		limiter, err := distributed.NewLimiter(distributed.TokenBucket, config)
		if err != nil {
			log.Fatalf("Failed to create limiter %d: %v", i+1, err)
		}
		limiters[i] = limiter
		defer func() { _ = limiter.Close() }()
	}

	ctx := context.Background()
	fmt.Printf("3 instances sharing: %.1f req/sec, burst %d\n", baseConfig.Rate, baseConfig.Burst)

	// All instances try to make requests simultaneously
	var wg sync.WaitGroup
	results := make([][]bool, 3)

	for i, limiter := range limiters {
		wg.Add(1)
		go func(instanceID int, lim distributed.Limiter) {
			defer wg.Done()
			results[instanceID] = make([]bool, 6)

			for j := 0; j < 6; j++ {
				results[instanceID][j] = lim.Allow(ctx)
				time.Sleep(50 * time.Millisecond)
			}
		}(i, limiter)
	}

	wg.Wait()

	// Display results
	totalAllowed := 0
	for i, instanceResults := range results {
		allowed := 0
		fmt.Printf("Instance %d: ", i+1)
		for j, result := range instanceResults {
			if result {
				fmt.Printf("✓")
				allowed++
				totalAllowed++
			} else {
				fmt.Printf("✗")
			}
			if j < len(instanceResults)-1 {
				fmt.Print(" ")
			}
		}
		fmt.Printf(" (%d allowed)\n", allowed)
	}
	fmt.Printf("Total allowed across all instances: %d\n", totalAllowed)

	// Show combined stats
	stats, err := limiters[0].Stats(ctx)
	if err == nil {
		fmt.Printf("Active instances: %v\n", stats.ActiveInstances)
	}

	// Clean up
	if err := limiters[0].Reset(ctx); err != nil {
		fmt.Printf("Warning: Failed to reset limiter: %v\n", err)
	}
}

// demonstrateStrategies compares different rate limiting strategies.
func demonstrateStrategies(rdb *redis.Client) {
	fmt.Println("\n3. Rate Limiting Strategies Comparison")
	fmt.Println("------------------------------------")

	strategies := []struct {
		name     string
		strategy distributed.Strategy
	}{
		{"Token Bucket", distributed.TokenBucket},
		{"Sliding Window", distributed.SlidingWindow},
		{"Fixed Window", distributed.FixedWindow},
	}

	ctx := context.Background()

	for _, s := range strategies {
		fmt.Printf("\nTesting %s:\n", s.name)

		config := distributed.Config{
			Redis:      rdb,
			Key:        "demo:" + s.name,
			Rate:       3.0, // 3 requests per second
			Burst:      3,
			InstanceID: "strategy-test",
		}

		limiter, err := distributed.NewLimiter(s.strategy, config)
		if err != nil {
			fmt.Printf("Failed to create %s limiter: %v\n", s.name, err)
			continue
		}

		// Test pattern: burst then wait then more requests
		fmt.Print("Burst phase: ")
		for i := 0; i < 5; i++ {
			if limiter.Allow(ctx) {
				fmt.Print("✓")
			} else {
				fmt.Print("✗")
			}
		}

		fmt.Print("\nWaiting 1 second...\n")
		time.Sleep(1 * time.Second)

		fmt.Print("Recovery phase: ")
		for i := 0; i < 4; i++ {
			if limiter.Allow(ctx) {
				fmt.Print("✓")
			} else {
				fmt.Print("✗")
			}
			time.Sleep(200 * time.Millisecond)
		}
		fmt.Println()

		if err := limiter.Close(); err != nil {
			fmt.Printf("Warning: Failed to close limiter: %v\n", err)
		}
		if err := limiter.Reset(ctx); err != nil {
			fmt.Printf("Warning: Failed to reset limiter: %v\n", err)
		}
	}
}

// demonstrateFallback shows fallback to local rate limiting.
func demonstrateFallback() {
	fmt.Println("\n4. Fallback to Local Rate Limiting")
	fmt.Println("---------------------------------")

	// Create Redis client that will fail
	rdb := redis.NewClient(&redis.Options{
		Addr:        "localhost:9999", // Non-existent server
		DialTimeout: 100 * time.Millisecond,
	})
	defer func() { _ = rdb.Close() }()

	// Create local fallback limiter
	localLimiter, err := bucket.NewSafe(bucket.Limit(2), 5)
	if err != nil {
		log.Fatalf("Failed to create local limiter: %v", err)
	}

	config := distributed.Config{
		Redis:           rdb,
		Key:             "demo:fallback",
		Rate:            5.0,
		Burst:           10,
		FallbackToLocal: true,
		LocalLimiter:    localLimiter,
	}

	fmt.Println("Attempting to create distributed limiter with unreachable Redis...")
	limiter, err := distributed.NewLimiter(distributed.TokenBucket, config)
	if err != nil {
		fmt.Printf("Expected error: %v\n", err)
		fmt.Println("Falling back to local rate limiter (2 req/sec, burst 5)")

		fmt.Print("Local limiter test: ")
		for i := 0; i < 8; i++ {
			if localLimiter.Allow() {
				fmt.Print("✓")
			} else {
				fmt.Print("✗")
			}
		}
		fmt.Println()
		return
	}
	defer func() { _ = limiter.Close() }()

	// If we somehow get here, test the limiter
	fmt.Print("Distributed limiter test: ")
	for i := 0; i < 6; i++ {
		if limiter.Allow(context.Background()) {
			fmt.Print("✓")
		} else {
			fmt.Print("✗")
		}
	}
	fmt.Println()
}

// demonstrateLoadTesting shows performance under load.
func demonstrateLoadTesting(rdb *redis.Client) {
	fmt.Println("\n5. Load Testing")
	fmt.Println("---------------")

	config := distributed.Config{
		Redis:      rdb,
		Key:        "demo:load",
		Rate:       50.0, // 50 req/sec
		Burst:      100,
		InstanceID: "load-test",
	}

	limiter, err := distributed.NewLimiter(distributed.TokenBucket, config)
	if err != nil {
		log.Fatalf("Failed to create limiter: %v", err)
	}
	defer func() { _ = limiter.Close() }()

	ctx := context.Background()

	// Concurrent load test
	const numWorkers = 10
	const requestsPerWorker = 20

	fmt.Printf("Load test: %d workers, %d requests each\n", numWorkers, requestsPerWorker)

	start := time.Now()
	var wg sync.WaitGroup
	totalAllowed := int64(0)
	var mu sync.Mutex

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(_ int) {
			defer wg.Done()
			allowed := 0

			for j := 0; j < requestsPerWorker; j++ {
				if limiter.Allow(ctx) {
					allowed++
				}
			}

			mu.Lock()
			totalAllowed += int64(allowed)
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	fmt.Printf("Results: %d/%d requests allowed in %v\n",
		totalAllowed, numWorkers*requestsPerWorker, elapsed)
	fmt.Printf("Throughput: %.1f req/sec\n",
		float64(numWorkers*requestsPerWorker)/elapsed.Seconds())

	// Final stats
	stats, err := limiter.Stats(ctx)
	if err == nil {
		fmt.Printf("Final stats: Total=%d, Allowed=%d, Denied=%d\n",
			stats.TotalRequests, stats.AllowedRequests, stats.DeniedRequests)
	}

	if err := limiter.Reset(ctx); err != nil {
		fmt.Printf("Warning: Failed to reset limiter: %v\n", err)
	}
}

// runHTTPServer runs a simple HTTP server with distributed rate limiting.
func runHTTPServer() {
	fmt.Println("Starting HTTP server with distributed rate limiting...")

	// Connect to Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   0,
	})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Redis not available: %v", err)
	}

	// Create distributed rate limiter
	config := distributed.Config{
		Redis:      rdb,
		Key:        "http:api",
		Rate:       10.0, // 10 requests per second globally
		Burst:      20,   // burst of 20
		InstanceID: fmt.Sprintf("server-%d", os.Getpid()),
	}

	limiter, err := distributed.NewLimiter(distributed.TokenBucket, config)
	if err != nil {
		log.Fatalf("Failed to create limiter: %v", err)
	}
	defer func() { _ = limiter.Close() }()

	// HTTP handler with rate limiting
	http.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow(r.Context()) {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		// Simulate API work
		time.Sleep(100 * time.Millisecond)

		w.Header().Set("Content-Type", "application/json")
		if _, err := fmt.Fprintf(w, `{"message": "Hello from %s!", "timestamp": "%s"}`,
			config.InstanceID, time.Now().Format(time.RFC3339)); err != nil {
			log.Printf("Error writing response: %v", err)
		}
	})

	// Stats endpoint
	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		stats, err := limiter.Stats(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := fmt.Fprintf(w, `{
	"rate": %.1f,
	"burst": %d,
	"tokens": %.1f,
	"total_requests": %d,
	"allowed_requests": %d,
	"denied_requests": %d,
	"active_instances": %d
}`, stats.Rate, stats.Burst, stats.Tokens, stats.TotalRequests,
			stats.AllowedRequests, stats.DeniedRequests, len(stats.ActiveInstances)); err != nil {
			log.Printf("Error writing response: %v", err)
		}
	})

	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		if _, err := strconv.Atoi(p); err == nil {
			port = p
		}
	}

	fmt.Printf("Server starting on port %s\n", port)
	fmt.Printf("Instance ID: %s\n", config.InstanceID)
	fmt.Printf("Rate limit: %.1f req/sec (burst %d) - SHARED across all instances\n",
		config.Rate, config.Burst)
	fmt.Println()
	fmt.Printf("Try: curl http://localhost:%s/api\n", port)
	fmt.Printf("Stats: curl http://localhost:%s/stats\n", port)
	fmt.Println()
	fmt.Println("Start multiple instances to see distributed rate limiting:")
	fmt.Printf("PORT=8081 go run main.go server\n")
	fmt.Printf("PORT=8082 go run main.go server\n")

	server := &http.Server{
		Addr:         ":" + port,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}
