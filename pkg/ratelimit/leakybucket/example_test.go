package leakybucket_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/vnykmshr/goflow/pkg/ratelimit/bucket"
	"github.com/vnykmshr/goflow/pkg/ratelimit/leakybucket"
)

// Example demonstrates basic usage of the leaky bucket rate limiter
func Example() {
	// Create a leaky bucket that leaks 5 requests per second with capacity of 10
	limiter := leakybucket.New(5, 10)

	// Check if a request is allowed (non-blocking)
	if limiter.Allow() {
		fmt.Println("Request allowed")
	} else {
		fmt.Println("Request denied")
	}

	// Output: Request allowed
}

// Example_trafficShaping demonstrates smooth traffic flow characteristics
func Example_trafficShaping() {
	// Create a leaky bucket for smooth traffic shaping (2 requests/sec, capacity 4)
	limiter := leakybucket.New(2, 4)

	fmt.Printf("Initial level: %.0f/%.0f\n", limiter.Level(), float64(limiter.Capacity()))

	// Send burst of requests
	for i := 0; i < 4; i++ {
		limiter.Allow()
	}

	fmt.Printf("After burst: %.0f/%.0f\n", limiter.Level(), float64(limiter.Capacity()))
	fmt.Printf("Available space: %.0f\n", limiter.Available())

	// Output:
	// Initial level: 0/4
	// After burst: 4/4
	// Available space: 0
}

// Example_wait demonstrates blocking until space is available
func Example_wait() {
	// Create a slow leaky bucket (1 request per second, capacity 1)
	limiter := leakybucket.New(1, 1)

	ctx := context.Background()

	// First request succeeds immediately
	if err := limiter.Wait(ctx); err != nil {
		log.Fatal(err)
	}
	fmt.Println("First request processed")

	// Second request would need to wait, but we'll use a timeout
	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	if err := limiter.Wait(ctx); err != nil {
		fmt.Printf("Second request failed: %v\n", err)
	}

	// Output:
	// First request processed
	// Second request failed: context deadline exceeded
}

// Example_reservation demonstrates the reservation pattern
func Example_reservation() {
	// Create a leaky bucket (3 requests per second, capacity 5)
	limiter := leakybucket.New(3, 5)

	// Fill bucket to capacity
	for i := 0; i < 5; i++ {
		limiter.Allow()
	}

	// Make a reservation for the next request
	reservation := limiter.Reserve()
	if reservation.OK() {
		delay := reservation.Delay()
		fmt.Printf("Need to wait %v before next request\n", delay.Round(time.Millisecond*100))

		// Cancel the reservation if we don't want to wait
		reservation.Cancel()
		fmt.Println("Reservation canceled")
	}

	// Output:
	// Need to wait 300ms before next request
	// Reservation canceled
}

// Example_multipleRequests demonstrates handling multiple requests at once
func Example_multipleRequests() {
	// Create a leaky bucket (10 requests per second, capacity 20)
	limiter := leakybucket.New(10, 20)

	// Try to handle 5 requests at once
	if limiter.AllowN(5) {
		fmt.Println("Batch operation allowed (5 requests)")
	}

	// Check current state
	fmt.Printf("Current level: %.0f\n", limiter.Level())
	fmt.Printf("Available space: %.0f\n", limiter.Available())

	// Output:
	// Batch operation allowed (5 requests)
	// Current level: 5
	// Available space: 15
}

// Example_configuration demonstrates advanced configuration
func Example_configuration() {
	// Create with specific configuration
	config := leakybucket.Config{
		LeakRate:     bucket.Every(200 * time.Millisecond), // 1 request every 200ms = 5/sec
		Capacity:     8,
		InitialLevel: 3, // Start with 3 requests in bucket
	}

	limiter := leakybucket.NewWithConfig(config)

	fmt.Printf("Initial level: %.0f\n", limiter.Level())
	fmt.Printf("Leak rate: %.1f/sec\n", limiter.LeakRate())
	fmt.Printf("Capacity: %d\n", limiter.Capacity())
	fmt.Printf("Available space: %.0f\n", limiter.Available())

	// Output:
	// Initial level: 3
	// Leak rate: 5.0/sec
	// Capacity: 8
	// Available space: 5
}

// Example_dynamicConfiguration demonstrates changing limits at runtime
func Example_dynamicConfiguration() {
	limiter := leakybucket.New(5, 10)

	fmt.Printf("Original rate: %.0f/sec, capacity: %d\n", limiter.LeakRate(), limiter.Capacity())

	// Increase the leak rate for faster processing
	limiter.SetLeakRate(15)
	fmt.Printf("Updated rate: %.0f/sec, capacity: %d\n", limiter.LeakRate(), limiter.Capacity())

	// Increase capacity for larger bursts
	limiter.SetCapacity(20)
	fmt.Printf("Final rate: %.0f/sec, capacity: %d\n", limiter.LeakRate(), limiter.Capacity())

	// Output:
	// Original rate: 5/sec, capacity: 10
	// Updated rate: 15/sec, capacity: 10
	// Final rate: 15/sec, capacity: 20
}

// Example_comparison demonstrates differences from token bucket
func Example_comparison() {
	// Both limiters allow same sustained rate but different burst behavior
	tokenBucket := bucket.New(5, 10)      // 5 tokens/sec, burst of 10
	leakyBucket := leakybucket.New(5, 10) // 5 leaks/sec, capacity of 10

	fmt.Println("=== Token Bucket (allows bursts) ===")
	fmt.Printf("Initial tokens: %.0f\n", tokenBucket.Tokens())

	// Token bucket starts full - allows immediate burst
	burstCount := 0
	for i := 0; i < 15; i++ { // Try more than capacity
		if tokenBucket.Allow() {
			burstCount++
		}
	}
	fmt.Printf("Burst requests allowed: %d\n", burstCount)

	fmt.Println("\n=== Leaky Bucket (smooth flow) ===")
	fmt.Printf("Initial level: %.0f\n", leakyBucket.Level())

	// Leaky bucket starts empty - builds up gradually
	allowedCount := 0
	for i := 0; i < 15; i++ { // Try more than capacity
		if leakyBucket.Allow() {
			allowedCount++
		}
	}
	fmt.Printf("Requests allowed before full: %d\n", allowedCount)
	fmt.Printf("Current level: %.0f\n", leakyBucket.Level())

	// Output:
	// === Token Bucket (allows bursts) ===
	// Initial tokens: 10
	// Burst requests allowed: 10
	//
	// === Leaky Bucket (smooth flow) ===
	// Initial level: 0
	// Requests allowed before full: 10
	// Current level: 10
}
