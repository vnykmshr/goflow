// Package metrics provides Prometheus instrumentation for goflow components.
//
// This package enables comprehensive monitoring and observability for goflow's
// rate limiting, task scheduling, and streaming components through Prometheus metrics.
//
// # Overview
//
// The metrics package provides automatic instrumentation for:
//   - Rate limiting operations (requests, allows, denies, wait times)
//   - Concurrency control (active operations, waiting operations)
//   - Task scheduling (scheduled, executed, completed, failed tasks)
//   - Worker pools (pool size, active workers, queued tasks)
//   - Streaming operations (items processed, errors, buffer usage)
//
// # Quick Start
//
// Enable metrics by using the metrics-enabled constructors:
//
//	// Rate limiter with metrics
//	limiter := bucket.NewWithMetrics(10, 20, "api_requests")
//	
//	// Worker pool with metrics
//	pool := workerpool.NewWithMetrics(5, "task_pool")
//	
//	// Scheduler with metrics
//	scheduler := scheduler.NewWithMetrics("job_scheduler")
//
// Then expose metrics via HTTP:
//
//	http.Handle("/metrics", promhttp.Handler())
//	log.Fatal(http.ListenAndServe(":8080", nil))
//
// # Custom Registry
//
// Use a custom Prometheus registry for isolation:
//
//	registry := prometheus.NewRegistry()
//	config := metrics.Config{
//		Enabled:  true,
//		Registry: registry,
//	}
//	
//	limiter := bucket.NewWithConfigAndMetrics(
//		bucket.Config{Rate: 5, Burst: 10},
//		"custom_limiter",
//		config,
//	)
//
// # Available Metrics
//
// ## Rate Limiting Metrics
//
//   - goflow_ratelimit_requests_total: Total number of rate limit requests
//   - goflow_ratelimit_allowed_total: Total number of allowed requests  
//   - goflow_ratelimit_denied_total: Total number of denied requests
//   - goflow_ratelimit_wait_duration_seconds: Time spent waiting for rate limit approval
//   - goflow_ratelimit_tokens_available: Number of tokens currently available
//
// ## Concurrency Metrics
//
//   - goflow_concurrency_active: Number of active concurrent operations
//   - goflow_concurrency_waiting: Number of operations waiting for concurrency slot
//
// ## Task Scheduling Metrics
//
//   - goflow_scheduler_tasks_scheduled_total: Total number of tasks scheduled
//   - goflow_scheduler_tasks_executed_total: Total number of tasks executed
//   - goflow_scheduler_tasks_completed_total: Total number of tasks completed successfully
//   - goflow_scheduler_tasks_failed_total: Total number of tasks that failed
//   - goflow_scheduler_task_duration_seconds: Time spent executing tasks
//
// ## Worker Pool Metrics
//
//   - goflow_workerpool_size: Current worker pool size
//   - goflow_workerpool_active_workers: Number of active workers
//   - goflow_workerpool_queued_tasks: Number of queued tasks
//
// ## Streaming Metrics
//
//   - goflow_stream_operations_total: Total number of stream operations
//   - goflow_stream_items_processed_total: Total number of items processed by streams
//   - goflow_stream_errors_total: Total number of stream processing errors
//   - goflow_stream_buffer_size: Stream buffer capacity
//   - goflow_stream_buffer_usage: Current stream buffer usage
//   - goflow_backpressure_events_total: Total number of backpressure events
//   - goflow_writer_flushes_total: Total number of writer flushes
//   - goflow_writer_bytes_written_total: Total bytes written
//
// # Labels
//
// Metrics include relevant labels for filtering and aggregation:
//
//   - limiter_type: "token_bucket", "leaky_bucket", or "concurrency"
//   - limiter_name: User-provided name for the limiter instance
//   - scheduler_name: User-provided name for the scheduler instance
//   - pool_name: User-provided name for the worker pool instance
//   - stream_name: User-provided name for the stream instance
//   - operation: Type of stream operation (e.g., "filter", "map", "reduce")
//   - strategy: Backpressure strategy (e.g., "block", "drop", "error")
//
// # Configuration
//
// Metrics can be configured globally or per-component:
//
//	config := metrics.Config{
//		Enabled:   true,                           // Enable/disable metrics
//		Registry:  prometheus.DefaultRegisterer,   // Custom registry
//		Namespace: "myapp",                        // Override default "goflow" 
//		Labels:    prometheus.Labels{"version": "1.0"}, // Additional labels
//	}
//
// # Runtime Control
//
// Components implementing the Instrumentable interface support runtime control:
//
//	limiter := bucket.NewWithMetrics(10, 20, "api")
//	limiter.DisableMetrics()           // Stop collecting metrics
//	limiter.EnableMetrics(config)      // Re-enable with new config
//	enabled := limiter.MetricsEnabled() // Check current state
//
// # Performance
//
// Metrics collection is designed for minimal overhead:
//   - Metrics are updated only when operations occur
//   - No background goroutines or timers
//   - Efficient label handling with pre-computed label values
//   - Conditional metrics updates based on enabled state
//
// # Examples
//
// See the example tests for comprehensive usage patterns:
//   - Example_comprehensiveMetrics: All components together
//   - Example_customRegistry: Using custom Prometheus registries  
//   - Example_metricsServer: Setting up HTTP metrics endpoint
//   - Example_metricsLifecycle: Runtime enable/disable patterns
package metrics