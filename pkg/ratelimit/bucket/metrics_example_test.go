package bucket

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/vnykmshr/goflow/pkg/metrics"
)

// Example_metricsBasic demonstrates basic metrics collection for token bucket rate limiter.
func Example_metricsBasic() {
	// Create a rate limiter with metrics (5 tokens per second, burst of 10)
	limiter := NewWithMetrics(5, 10, "api_requests")

	ctx := context.Background()

	// Make some requests
	for i := 0; i < 15; i++ {
		if limiter.Allow() {
			fmt.Printf("Request %d: Allowed\n", i+1)
		} else {
			fmt.Printf("Request %d: Denied\n", i+1)
		}
	}

	// Wait for some tokens to refill
	time.Sleep(100 * time.Millisecond)
	
	// Try one more request
	err := limiter.Wait(ctx)
	if err == nil {
		fmt.Println("Final request: Allowed after wait")
	}

	fmt.Printf("Remaining tokens: %.1f\n", limiter.Tokens())

	// Output:
	// Request 1: Allowed
	// Request 2: Allowed
	// Request 3: Allowed
	// Request 4: Allowed
	// Request 5: Allowed
	// Request 6: Allowed
	// Request 7: Allowed
	// Request 8: Allowed
	// Request 9: Allowed
	// Request 10: Allowed
	// Request 11: Denied
	// Request 12: Denied
	// Request 13: Denied
	// Request 14: Denied
	// Request 15: Denied
	// Final request: Allowed after wait
	// Remaining tokens: 0.0
}

// Example_metricsCustomRegistry demonstrates using a custom Prometheus registry.
func Example_metricsCustomRegistry() {
	// Create custom registry
	customRegistry := prometheus.NewRegistry()
	
	metricsConfig := metrics.Config{
		Enabled:  true,
		Registry: customRegistry,
	}

	// Create limiter with custom metrics registry
	limiter := NewWithConfigAndMetrics(Config{
		Rate:          Limit(2), // 2 tokens per second
		Burst:         5,
		InitialTokens: 3, // Start with 3 tokens
	}, "custom_limiter", metricsConfig)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Test various operations
	fmt.Printf("Initial tokens: %.0f\n", limiter.Tokens())
	
	// Allow operations
	fmt.Printf("Allow(): %v\n", limiter.Allow())
	fmt.Printf("Allow(): %v\n", limiter.Allow())
	fmt.Printf("Allow(): %v\n", limiter.Allow())
	fmt.Printf("Allow(): %v\n", limiter.Allow()) // Should be denied
	
	// Wait operation (should timeout due to context)
	err := limiter.Wait(ctx)
	if err != nil {
		fmt.Printf("Wait failed: %v\n", err)
	}

	fmt.Printf("Final tokens: %.0f\n", limiter.Tokens())

	// Output:
	// Initial tokens: 3
	// Allow(): true
	// Allow(): true
	// Allow(): true
	// Allow(): false
	// Wait failed: context deadline exceeded
	// Final tokens: 0
}

// Example_metricsHTTPServer demonstrates exposing metrics via HTTP.
func Example_metricsHTTPServer() {
	// Create rate limiter with metrics
	limiter := NewWithMetrics(10, 20, "http_requests")

	// Simulate API requests
	for i := 0; i < 25; i++ {
		if limiter.Allow() {
			// Simulate successful request processing
			time.Sleep(1 * time.Millisecond)
		}
	}

	// In a real application, you would start an HTTP server like this:
	//
	// http.Handle("/metrics", promhttp.Handler())
	// log.Fatal(http.ListenAndServe(":8080", nil))
	//
	// This would expose metrics at http://localhost:8080/metrics

	fmt.Printf("Tokens remaining: %.1f\n", limiter.Tokens())
	fmt.Println("Metrics server would be available at /metrics endpoint")

	// Output:
	// Tokens remaining: 0.0
	// Metrics server would be available at /metrics endpoint
}

// Example_metricsConfiguration demonstrates different metrics configurations.
func Example_metricsConfiguration() {
	// Limiter with metrics disabled
	disabledConfig := metrics.Config{
		Enabled: false,
	}
	limiterDisabled := NewWithConfigAndMetrics(Config{
		Rate:  5,
		Burst: 10,
	}, "disabled_limiter", disabledConfig)

	// Limiter with metrics enabled (default)
	enabledConfig := metrics.DefaultConfig()
	limiterEnabled := NewWithConfigAndMetrics(Config{
		Rate:  5, 
		Burst: 10,
	}, "enabled_limiter", enabledConfig)

	// Test both limiters
	fmt.Printf("Disabled limiter allows: %v\n", limiterDisabled.Allow())
	fmt.Printf("Enabled limiter allows: %v\n", limiterEnabled.Allow())

	// Check metrics status (if implementing Instrumentable interface)
	if ml, ok := limiterEnabled.(*MetricsLimiter); ok {
		fmt.Printf("Enabled limiter has metrics: %v\n", ml.MetricsEnabled())
	}
	
	if ml, ok := limiterDisabled.(*MetricsLimiter); ok {
		fmt.Printf("Disabled limiter has metrics: %v\n", ml.MetricsEnabled())
	} else {
		fmt.Println("Disabled limiter has metrics: false")
	}

	// Output:
	// Disabled limiter allows: true
	// Enabled limiter allows: true  
	// Enabled limiter has metrics: true
	// Disabled limiter has metrics: false
}

// startMetricsServer starts a basic HTTP server for metrics (helper for examples).
func startMetricsServer(port string) {
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		if err := http.ListenAndServe(port, nil); err != nil {
			fmt.Printf("Metrics server failed: %v\n", err)
		}
	}()
}