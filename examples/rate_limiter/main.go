// Rate limiting example demonstrating different types of limiters
package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/vnykmshr/goflow/pkg/ratelimit/bucket"
	"github.com/vnykmshr/goflow/pkg/ratelimit/concurrency"
	"github.com/vnykmshr/goflow/pkg/ratelimit/leakybucket"
)

func main() {
	// Example 1: Token bucket for API rate limiting (allows bursts)
	apiLimiter, err := bucket.NewSafe(10, 20) // 10 RPS, burst 20
	if err != nil {
		log.Fatal(err)
	}

	// Example 2: Leaky bucket for smooth processing (no bursts)
	smoothLimiter, err := leakybucket.NewSafe(5, 5) // 5 per second, capacity 5
	if err != nil {
		log.Fatal(err)
	}

	// Example 3: Concurrency limiter for resource protection
	dbLimiter, err := concurrency.NewSafe(3) // max 3 concurrent operations
	if err != nil {
		log.Fatal(err)
	}

	// HTTP server with rate limiting
	http.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		if !apiLimiter.Allow() {
			http.Error(w, "Rate limited", http.StatusTooManyRequests)
			return
		}
		fmt.Fprintf(w, "API request processed at %v\n", time.Now())
	})

	http.HandleFunc("/process", func(w http.ResponseWriter, r *http.Request) {
		if !smoothLimiter.Allow() {
			http.Error(w, "Processing queue full", http.StatusTooManyRequests)
			return
		}
		fmt.Fprintf(w, "Processing request at %v\n", time.Now())
	})

	http.HandleFunc("/db", func(w http.ResponseWriter, r *http.Request) {
		if !dbLimiter.Acquire() {
			http.Error(w, "Too many DB operations", http.StatusServiceUnavailable)
			return
		}
		defer dbLimiter.Release()

		// Simulate database operation
		time.Sleep(100 * time.Millisecond)
		fmt.Fprintf(w, "Database query completed at %v\n", time.Now())
	})

	fmt.Println("Rate limiter examples running on :8080")
	fmt.Println("Try these endpoints:")
	fmt.Println("  /api     - Token bucket (10 RPS, burst 20)")
	fmt.Println("  /process - Leaky bucket (5 RPS, smooth)")
	fmt.Println("  /db      - Concurrency limit (max 3)")

	log.Fatal(http.ListenAndServe(":8080", nil))
}