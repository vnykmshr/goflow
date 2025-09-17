package bucket_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/vnykmshr/goflow/pkg/ratelimit/bucket"
)

// Example demonstrates basic usage of the token bucket rate limiter
func Example() {
	// Create a rate limiter that allows 10 requests per second with a burst of 5
	limiter, err := bucket.NewSafe(10, 5)
	if err != nil {
		panic(fmt.Sprintf("Failed to create limiter: %v", err))
	}

	// Check if a request is allowed (non-blocking)
	if limiter.Allow() {
		fmt.Println("Request allowed")
	} else {
		fmt.Println("Request denied")
	}

	// Output: Request allowed
}

// Example_wait demonstrates blocking until tokens are available
func Example_wait() {
	// Create a slow rate limiter (1 request per second, burst of 1)
	limiter, err := bucket.NewSafe(1, 1)
	if err != nil {
		panic(fmt.Sprintf("Failed to create limiter: %v", err))
	}

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
	// Create a rate limiter (2 requests per second, burst of 3)
	limiter, err := bucket.NewSafe(2, 3)
	if err != nil {
		panic(fmt.Sprintf("Failed to create limiter: %v", err))
	}

	// Use all burst tokens
	for i := 0; i < 3; i++ {
		limiter.Allow()
	}

	// Make a reservation for the next request
	reservation := limiter.Reserve()
	if reservation.OK() {
		delay := reservation.Delay()
		fmt.Printf("Need to wait %v before next request\n", delay.Round(time.Millisecond))

		// Cancel the reservation if we don't want to wait
		reservation.Cancel()
		fmt.Println("Reservation canceled")
	}

	// Output:
	// Need to wait 500ms before next request
	// Reservation canceled
}

// Example_multipleTokens demonstrates consuming multiple tokens at once
func Example_multipleTokens() {
	// Create a rate limiter (10 tokens per second, burst of 20)
	limiter, err := bucket.NewSafe(10, 20)
	if err != nil {
		panic(fmt.Sprintf("Failed to create limiter: %v", err))
	}

	// Try to consume 5 tokens at once
	if limiter.AllowN(5) {
		fmt.Println("Bulk operation allowed (5 tokens)")
	}

	// Check remaining tokens
	remaining := limiter.Tokens()
	fmt.Printf("Tokens remaining: %.0f\n", remaining)

	// Output:
	// Bulk operation allowed (5 tokens)
	// Tokens remaining: 15
}

// Example_configuration demonstrates advanced configuration
func Example_configuration() {
	// Create with specific configuration
	config := bucket.Config{
		Rate:          bucket.Every(100 * time.Millisecond), // 1 token every 100ms
		Burst:         5,
		InitialTokens: 2, // Start with 2 tokens instead of full burst
	}

	limiter, err := bucket.NewWithConfigSafe(config)
	if err != nil {
		panic(fmt.Sprintf("Failed to create limiter: %v", err))
	}

	fmt.Printf("Initial tokens: %.0f\n", limiter.Tokens())
	fmt.Printf("Rate limit: %.1f/sec\n", limiter.Limit())
	fmt.Printf("Burst capacity: %d\n", limiter.Burst())

	// Output:
	// Initial tokens: 2
	// Rate limit: 10.0/sec
	// Burst capacity: 5
}

// Example_dynamicConfiguration demonstrates changing limits at runtime
func Example_dynamicConfiguration() {
	limiter, err := bucket.NewSafe(5, 10)
	if err != nil {
		panic(fmt.Sprintf("Failed to create limiter: %v", err))
	}

	fmt.Printf("Original rate: %.0f/sec, burst: %d\n", limiter.Limit(), limiter.Burst())

	// Increase the rate limit during high traffic
	limiter.SetLimit(20)
	fmt.Printf("Updated rate: %.0f/sec, burst: %d\n", limiter.Limit(), limiter.Burst())

	// Reduce burst size for stricter limiting
	limiter.SetBurst(5)
	fmt.Printf("Final rate: %.0f/sec, burst: %d\n", limiter.Limit(), limiter.Burst())

	// Output:
	// Original rate: 5/sec, burst: 10
	// Updated rate: 20/sec, burst: 10
	// Final rate: 20/sec, burst: 5
}
