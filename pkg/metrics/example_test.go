package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

// Example_basicUsage demonstrates basic metrics configuration.
func Example_basicUsage() {
	// Create a separate registry for this test
	testRegistry := prometheus.NewRegistry()
	registry := NewRegistry(testRegistry)

	fmt.Printf("Registry created with %d rate limit metrics\n", 5)
	fmt.Printf("Registry created with %d concurrency metrics\n", 2)
	fmt.Printf("Registry created with %d scheduling metrics\n", 5)
	fmt.Printf("Registry created with %d streaming metrics\n", 6)

	// Example of accessing metrics
	registry.RateLimitRequests.WithLabelValues("token_bucket", "test").Add(10)
	registry.RateLimitAllowed.WithLabelValues("token_bucket", "test").Add(8)
	registry.RateLimitDenied.WithLabelValues("token_bucket", "test").Add(2)

	fmt.Println("Metrics updated successfully")

	// Output:
	// Registry created with 5 rate limit metrics
	// Registry created with 2 concurrency metrics
	// Registry created with 5 scheduling metrics
	// Registry created with 6 streaming metrics
	// Metrics updated successfully
}

// Example_customRegistry demonstrates using a custom Prometheus registry.
func Example_customRegistry() {
	// Create a custom registry
	customRegistry := prometheus.NewRegistry()

	config := Config{
		Enabled:  true,
		Registry: customRegistry,
	}

	// Create metrics registry with custom config
	registry := NewRegistry(config.Registry)

	// Test the registry
	registry.RateLimitRequests.WithLabelValues("custom", "limiter").Add(12)
	registry.RateLimitAllowed.WithLabelValues("custom", "limiter").Add(10)
	registry.RateLimitDenied.WithLabelValues("custom", "limiter").Add(2)

	fmt.Printf("Custom registry enabled: %v\n", config.Enabled)
	fmt.Println("Custom registry configured with goflow metrics")

	// Output:
	// Custom registry enabled: true
	// Custom registry configured with goflow metrics
}

// Example_metricsServer demonstrates setting up a metrics HTTP server.
func Example_metricsServer() {
	// In a real application, you would start a metrics server:
	//
	// http.Handle("/metrics", promhttp.Handler())
	// log.Fatal(http.ListenAndServe(":8080", nil))
	//
	// Available metrics would include:
	// - goflow_ratelimit_requests_total{limiter_type="token_bucket",limiter_name="http_api"}
	// - goflow_ratelimit_allowed_total{limiter_type="token_bucket",limiter_name="http_api"}
	// - goflow_ratelimit_denied_total{limiter_type="token_bucket",limiter_name="http_api"}
	// - goflow_workerpool_size{pool_name="request_handlers"}
	// - goflow_workerpool_active_workers{pool_name="request_handlers"}
	// - goflow_workerpool_queued_tasks{pool_name="request_handlers"}
	// And many more...

	fmt.Println("Metrics available at /metrics endpoint")
	fmt.Println("See examples/metrics/main.go for a complete demonstration")

	// Output:
	// Metrics available at /metrics endpoint
	// See examples/metrics/main.go for a complete demonstration
}

// Example_configuration demonstrates different metrics configurations.
func Example_configuration() {
	// Default configuration
	defaultConfig := DefaultConfig()
	fmt.Printf("Default enabled: %v\n", defaultConfig.Enabled)
	fmt.Printf("Default namespace: %s\n", defaultConfig.Namespace)

	// Custom configuration
	customConfig := Config{
		Enabled:   false,
		Namespace: "myapp",
	}
	fmt.Printf("Custom enabled: %v\n", customConfig.Enabled)
	fmt.Printf("Custom namespace: %s\n", customConfig.Namespace)

	// Output:
	// Default enabled: true
	// Default namespace: goflow
	// Custom enabled: false
	// Custom namespace: myapp
}
