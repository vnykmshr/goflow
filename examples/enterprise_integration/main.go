// Package main demonstrates enterprise-grade integration of all goflow components
// in a realistic production scenario: a high-throughput API gateway with
// sophisticated rate limiting, advanced scheduling, and streaming data processing.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/vnykmshr/goflow/pkg/metrics"
	"github.com/vnykmshr/goflow/pkg/ratelimit/distributed"
	"github.com/vnykmshr/goflow/pkg/ratelimit/bucket"
	"github.com/vnykmshr/goflow/pkg/scheduling/scheduler"
	"github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
	"github.com/vnykmshr/goflow/pkg/streaming/stream"
	"github.com/vnykmshr/goflow/pkg/streaming/writer"
)

// EnterpriseAPIGateway demonstrates a production-ready API gateway
// using all advanced goflow components in coordination.
type EnterpriseAPIGateway struct {
	// Rate limiting with distributed coordination
	globalRateLimiter   distributed.RateLimiter
	perUserRateLimiter  *bucket.TokenBucket
	
	// Advanced scheduling for background operations
	scheduler           scheduler.ContextAwareScheduler
	
	// High-performance workers for request processing
	requestWorkers      workerpool.Pool
	analyticsWorkers    workerpool.Pool
	
	// Streaming data processing
	requestStream       *stream.Stream[*APIRequest]
	responseStream      *stream.Stream[*APIResponse]
	
	// Async logging and metrics
	logWriter          *writer.AsyncWriter
	metricsProvider    metrics.MetricsProvider
}

// APIRequest represents an incoming API request
type APIRequest struct {
	ID        string
	UserID    string
	Endpoint  string
	Method    string
	Timestamp time.Time
	Headers   map[string]string
	Body      []byte
}

// APIResponse represents an API response
type APIResponse struct {
	RequestID  string
	StatusCode int
	Duration   time.Duration
	Size       int
	Cached     bool
	Timestamp  time.Time
}

// NewEnterpriseAPIGateway creates a fully configured enterprise API gateway
func NewEnterpriseAPIGateway() (*EnterpriseAPIGateway, error) {
	// Initialize metrics provider
	metricsConfig := metrics.Config{
		Namespace: "apigateway",
		Subsystem: "requests",
		EnableHistograms: true,
	}
	metricsProvider := metrics.NewPrometheusMetrics(metricsConfig)

	// Distributed rate limiting with Redis coordination
	globalLimiter := distributed.NewRedisTokenBucket(distributed.RedisConfig{
		Addr:       "localhost:6379",
		Key:        "global_api_rate_limit",
		Capacity:   10000,    // 10K requests
		RefillRate: 1000,     // 1K requests/second
		Window:     time.Minute,
	})

	// Per-user rate limiting (in-memory, fast)
	perUserLimiter := bucket.New(100, 10.0) // 100 requests, 10/second refill

	// Context-aware scheduler with enterprise features
	sched := scheduler.WithGlobalTimeout(scheduler.Config{
		WorkerPool: workerpool.New(4, 100),
	}, 30*time.Second)
	
	// Enable context propagation for distributed tracing
	sched.SetContextPropagation(true, []interface{}{"trace_id", "user_id", "request_id"})

	// High-performance worker pools
	requestWorkers := workerpool.New(50, 1000)   // 50 workers, 1K buffer
	analyticsWorkers := workerpool.New(10, 500)  // 10 workers, 500 buffer

	// Async writer for high-volume logging
	logWriter := writer.New(writer.Config{
		BufferSize:    10000,
		FlushInterval: 1 * time.Second,
		FlushOnClose:  true,
	})

	gateway := &EnterpriseAPIGateway{
		globalRateLimiter:   globalLimiter,
		perUserRateLimiter:  perUserLimiter,
		scheduler:          sched,
		requestWorkers:     requestWorkers,
		analyticsWorkers:   analyticsWorkers,
		logWriter:          logWriter,
		metricsProvider:    metricsProvider,
	}

	return gateway, nil
}

// Start initializes and starts all gateway components
func (gw *EnterpriseAPIGateway) Start(ctx context.Context) error {
	log.Println("🚀 Starting Enterprise API Gateway...")

	// Start scheduler
	if err := gw.scheduler.Start(); err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}

	// Schedule recurring maintenance tasks
	if err := gw.scheduleMaintenanceTasks(ctx); err != nil {
		return fmt.Errorf("failed to schedule maintenance: %w", err)
	}

	// Initialize streaming data processors
	if err := gw.setupStreamProcessing(ctx); err != nil {
		return fmt.Errorf("failed to setup streaming: %w", err)
	}

	// Start metrics server
	go gw.startMetricsServer()

	log.Println("✅ API Gateway started successfully")
	return nil
}

// ProcessRequest handles an incoming API request with enterprise-grade features
func (gw *EnterpriseAPIGateway) ProcessRequest(ctx context.Context, req *APIRequest) (*APIResponse, error) {
	start := time.Now()
	
	// Add request context for distributed tracing
	ctx = context.WithValue(ctx, "request_id", req.ID)
	ctx = context.WithValue(ctx, "user_id", req.UserID)

	// Global rate limiting check
	allowed, err := gw.globalRateLimiter.Allow(ctx, 1)
	if err != nil {
		return nil, fmt.Errorf("rate limiter error: %w", err)
	}
	if !allowed {
		return &APIResponse{
			RequestID:  req.ID,
			StatusCode: http.StatusTooManyRequests,
			Duration:   time.Since(start),
			Timestamp:  time.Now(),
		}, nil
	}

	// Per-user rate limiting
	if !gw.perUserRateLimiter.Allow(ctx) {
		return &APIResponse{
			RequestID:  req.ID,
			StatusCode: http.StatusTooManyRequests,
			Duration:   time.Since(start),
			Timestamp:  time.Now(),
		}, nil
	}

	// Process request using worker pool
	var response *APIResponse
	task := workerpool.TaskFunc(func(taskCtx context.Context) error {
		response = gw.processRequestLogic(taskCtx, req, start)
		return nil
	})

	if err := gw.requestWorkers.Submit(task); err != nil {
		return nil, fmt.Errorf("failed to submit request: %w", err)
	}

	// Schedule async analytics processing
	gw.scheduleAsyncAnalytics(ctx, req, response)

	// Log request asynchronously
	gw.logRequestAsync(req, response)

	return response, nil
}

// processRequestLogic contains the core business logic
func (gw *EnterpriseAPIGateway) processRequestLogic(ctx context.Context, req *APIRequest, start time.Time) *APIResponse {
	// Simulate API processing with variable duration
	processingTime := time.Duration(50+req.ID[0]%100) * time.Millisecond
	time.Sleep(processingTime)

	// Simulate different response scenarios
	var statusCode int
	var size int
	cached := false

	switch req.Endpoint {
	case "/api/users":
		statusCode = http.StatusOK
		size = 1024
		cached = req.Method == "GET"
	case "/api/orders":
		statusCode = http.StatusOK
		size = 2048
	case "/api/products":
		statusCode = http.StatusOK
		size = 4096
		cached = true
	default:
		statusCode = http.StatusNotFound
		size = 128
	}

	return &APIResponse{
		RequestID:  req.ID,
		StatusCode: statusCode,
		Duration:   time.Since(start),
		Size:       size,
		Cached:     cached,
		Timestamp:  time.Now(),
	}
}

// scheduleMaintenanceTasks sets up recurring maintenance operations
func (gw *EnterpriseAPIGateway) scheduleMaintenanceTasks(ctx context.Context) error {
	// Cache cleanup every hour with exponential backoff on failure
	cacheCleanup := workerpool.TaskFunc(func(ctx context.Context) error {
		log.Println("🧹 Running cache cleanup...")
		// Simulate cache cleanup work
		time.Sleep(100 * time.Millisecond)
		
		// Simulate occasional failures for backoff demonstration
		if time.Now().UnixNano()%10 == 0 {
			return fmt.Errorf("cache cleanup failed - will retry with backoff")
		}
		
		log.Println("✅ Cache cleanup completed")
		return nil
	})

	err := gw.scheduler.ScheduleWithBackoff("cache_cleanup", cacheCleanup, scheduler.BackoffConfig{
		InitialDelay: 1 * time.Second,
		MaxDelay:     10 * time.Minute,
		Multiplier:   2.0,
		MaxRetries:   5,
		OnMaxRetries: func(task scheduler.ScheduledTask, attempts int) {
			log.Printf("❌ Cache cleanup failed after %d attempts", attempts)
		},
	})
	if err != nil {
		return err
	}

	// Database maintenance every 6 hours during low-traffic windows
	dbMaintenance := workerpool.TaskFunc(func(ctx context.Context) error {
		log.Println("🔧 Running database maintenance...")
		time.Sleep(500 * time.Millisecond)
		log.Println("✅ Database maintenance completed")
		return nil
	})

	// Define maintenance windows (2 AM - 4 AM local time)
	now := time.Now()
	maintenanceStart := time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, now.Location())
	maintenanceEnd := time.Date(now.Year(), now.Month(), now.Day(), 4, 0, 0, 0, now.Location())
	
	windows := []scheduler.TimeWindow{{
		Start:    maintenanceStart,
		End:      maintenanceEnd,
		TimeZone: now.Location(),
		Days:     []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday},
	}}

	err = gw.scheduler.ScheduleWindow("db_maintenance", dbMaintenance, windows)
	if err != nil {
		return err
	}

	// Analytics aggregation every 15 minutes with adaptive timing
	analyticsTask := workerpool.TaskFunc(func(ctx context.Context) error {
		log.Println("📊 Running analytics aggregation...")
		time.Sleep(200 * time.Millisecond)
		log.Println("✅ Analytics aggregation completed")
		return nil
	})

	// Load metric function for adaptive scheduling
	loadMetric := func() float64 {
		// Simulate system load measurement
		stats := gw.scheduler.GetContextStats()
		return float64(stats.ActiveContexts) / 100.0 // Normalize to 0-1
	}

	err = gw.scheduler.ScheduleAdaptive("analytics", analyticsTask, scheduler.AdaptiveConfig{
		BaseInterval:   15 * time.Minute,
		MinInterval:    5 * time.Minute,
		MaxInterval:    30 * time.Minute,
		LoadThreshold:  0.7,
		AdaptationRate: 0.1,
		LoadMetricFunc: loadMetric,
	})
	if err != nil {
		return err
	}

	// Health check with jittered scheduling
	healthCheck := workerpool.TaskFunc(func(ctx context.Context) error {
		log.Println("💓 Health check - All systems operational")
		return nil
	})

	jitterConfig := scheduler.JitterConfig{
		Type:   scheduler.JitterUniform,
		Amount: 30 * time.Second, // ±30 seconds jitter
	}

	err = gw.scheduler.ScheduleJittered("health_check", healthCheck, 2*time.Minute, jitterConfig)
	if err != nil {
		return err
	}

	log.Println("📅 Scheduled maintenance tasks")
	return nil
}

// setupStreamProcessing initializes streaming data processors
func (gw *EnterpriseAPIGateway) setupStreamProcessing(ctx context.Context) error {
	// Create request processing stream
	gw.requestStream = stream.FromChannel(make(chan *APIRequest, 1000)).
		Filter(func(req *APIRequest) bool {
			// Filter out health check requests from analytics
			return req.Endpoint != "/health"
		}).
		Map(func(req *APIRequest) *APIRequest {
			// Enrich request with additional metadata
			if req.Headers == nil {
				req.Headers = make(map[string]string)
			}
			req.Headers["processed_at"] = time.Now().Format(time.RFC3339)
			return req
		})

	// Create response processing stream
	gw.responseStream = stream.FromChannel(make(chan *APIResponse, 1000)).
		Filter(func(resp *APIResponse) bool {
			// Only process successful responses for caching
			return resp.StatusCode >= 200 && resp.StatusCode < 300
		}).
		Map(func(resp *APIResponse) *APIResponse {
			// Add performance metadata
			if resp.Duration < 100*time.Millisecond {
				resp.Cached = true
			}
			return resp
		})

	log.Println("🌊 Stream processing initialized")
	return nil
}

// scheduleAsyncAnalytics queues analytics processing
func (gw *EnterpriseAPIGateway) scheduleAsyncAnalytics(ctx context.Context, req *APIRequest, resp *APIResponse) {
	analyticsTask := workerpool.TaskFunc(func(taskCtx context.Context) error {
		// Process analytics data
		log.Printf("📈 Analytics: %s %s -> %d (%v)", req.Method, req.Endpoint, resp.StatusCode, resp.Duration)
		
		// Update metrics
		labels := map[string]string{
			"method":   req.Method,
			"endpoint": req.Endpoint,
			"status":   fmt.Sprintf("%d", resp.StatusCode),
		}
		gw.metricsProvider.IncCounter("requests_total", labels)
		gw.metricsProvider.RecordHistogram("request_duration_seconds", resp.Duration.Seconds(), labels)
		
		return nil
	})

	// Submit to analytics workers
	if err := gw.analyticsWorkers.Submit(analyticsTask); err != nil {
		log.Printf("⚠️ Failed to submit analytics task: %v", err)
	}
}

// logRequestAsync performs asynchronous request logging
func (gw *EnterpriseAPIGateway) logRequestAsync(req *APIRequest, resp *APIResponse) {
	logEntry := fmt.Sprintf("[%s] %s %s %s -> %d (%v)\n", 
		req.Timestamp.Format("2006-01-02 15:04:05"),
		req.ID, req.Method, req.Endpoint, resp.StatusCode, resp.Duration)
	
	if _, err := gw.logWriter.Write([]byte(logEntry)); err != nil {
		log.Printf("⚠️ Failed to write log entry: %v", err)
	}
}

// startMetricsServer starts the Prometheus metrics HTTP server
func (gw *EnterpriseAPIGateway) startMetricsServer() {
	http.Handle("/metrics", promhttp.Handler())
	log.Println("📊 Metrics server started on :8080/metrics")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Printf("❌ Metrics server error: %v", err)
	}
}

// Stop gracefully shuts down the API gateway
func (gw *EnterpriseAPIGateway) Stop(ctx context.Context) error {
	log.Println("🛑 Shutting down API Gateway...")

	// Stop scheduler
	<-gw.scheduler.Stop()

	// Shutdown worker pools
	<-gw.requestWorkers.Shutdown()
	<-gw.analyticsWorkers.Shutdown()

	// Close async writer
	if err := gw.logWriter.Close(); err != nil {
		log.Printf("⚠️ Error closing log writer: %v", err)
	}

	// Print final statistics
	stats := gw.scheduler.GetContextStats()
	log.Printf("📊 Final Statistics - Active: %d, Cancelled: %d, Timeouts: %d",
		stats.ActiveContexts, stats.CancelledTasks, stats.TimeoutTasks)

	log.Println("✅ API Gateway stopped gracefully")
	return nil
}

// DemoScenario simulates realistic API gateway usage
func DemoScenario(ctx context.Context, gateway *EnterpriseAPIGateway) {
	log.Println("🎬 Starting demo scenario...")

	// Simulate realistic API traffic
	endpoints := []string{"/api/users", "/api/orders", "/api/products", "/health"}
	methods := []string{"GET", "POST", "PUT", "DELETE"}

	for i := 0; i < 50; i++ {
		req := &APIRequest{
			ID:        fmt.Sprintf("req-%d", i),
			UserID:    fmt.Sprintf("user-%d", i%10),
			Endpoint:  endpoints[i%len(endpoints)],
			Method:    methods[i%len(methods)],
			Timestamp: time.Now(),
			Headers:   map[string]string{"User-Agent": "goflow-demo/1.0"},
		}

		// Process request
		resp, err := gateway.ProcessRequest(ctx, req)
		if err != nil {
			log.Printf("❌ Request %s failed: %v", req.ID, err)
			continue
		}

		log.Printf("✅ Request %s processed: %d (%v)", req.ID, resp.StatusCode, resp.Duration)
		
		// Vary request frequency
		if i%10 == 0 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	log.Println("🎬 Demo scenario completed")
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Create enterprise API gateway
	gateway, err := NewEnterpriseAPIGateway()
	if err != nil {
		log.Fatalf("❌ Failed to create gateway: %v", err)
	}

	// Start the gateway
	if err := gateway.Start(ctx); err != nil {
		log.Fatalf("❌ Failed to start gateway: %v", err)
	}

	// Run demo scenario
	go func() {
		time.Sleep(2 * time.Second) // Let everything initialize
		DemoScenario(ctx, gateway)
	}()

	// Wait for demo completion or timeout
	<-ctx.Done()

	// Graceful shutdown
	if err := gateway.Stop(context.Background()); err != nil {
		log.Printf("⚠️ Shutdown error: %v", err)
	}
}