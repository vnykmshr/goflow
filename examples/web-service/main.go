// Package main demonstrates a complete web service using multiple goflow modules:
// - Rate limiting for API endpoints
// - Worker pools for background tasks
// - Concurrency limiting for resource protection
// - Pipeline processing for data transformation
// - Scheduled tasks for maintenance
// - Integrated metrics for monitoring
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/vnykmshr/goflow/pkg/metrics"
	"github.com/vnykmshr/goflow/pkg/ratelimit/bucket"
	"github.com/vnykmshr/goflow/pkg/ratelimit/concurrency"
	"github.com/vnykmshr/goflow/pkg/scheduling/pipeline"
	"github.com/vnykmshr/goflow/pkg/scheduling/scheduler"
	"github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

// WebService demonstrates a production-ready web service using goflow
type WebService struct {
	// Rate limiting
	apiRateLimiter    bucket.Limiter
	uploadRateLimiter bucket.Limiter

	// Resource protection
	dbConnectionLimiter concurrency.Limiter
	cpuIntensiveLimiter concurrency.Limiter

	// Background processing
	backgroundWorkers workerpool.Pool
	taskScheduler     scheduler.Scheduler

	// Data processing
	dataPipeline pipeline.Pipeline

	// Metrics
	metricsRegistry *metrics.Registry
	httpServer      *http.Server
}

// NewWebService creates a new web service with properly configured goflow components
func NewWebService(port string) (*WebService, error) {
	// Create metrics registry
	promRegistry := prometheus.NewRegistry()
	metricsRegistry := metrics.NewRegistry(promRegistry)

	// Create rate limiters with safe constructors and helpful error messages
	apiRateLimiter, err := bucket.NewSafe(100, 200) // 100 RPS, burst 200
	if err != nil {
		return nil, fmt.Errorf("failed to create API rate limiter: %w", err)
	}

	uploadRateLimiter, err := bucket.NewSafe(10, 50) // 10 RPS, burst 50 for uploads
	if err != nil {
		return nil, fmt.Errorf("failed to create upload rate limiter: %w", err)
	}

	// Create concurrency limiters for resource protection
	dbLimiter, err := concurrency.NewSafe(20) // Max 20 concurrent DB operations
	if err != nil {
		return nil, fmt.Errorf("failed to create DB connection limiter: %w", err)
	}

	cpuLimiter, err := concurrency.NewSafe(4) // Max 4 CPU-intensive tasks
	if err != nil {
		return nil, fmt.Errorf("failed to create CPU limiter: %w", err)
	}

	// Create worker pool for background tasks
	workerConfig := workerpool.Config{
		WorkerCount: 10,
		QueueSize:   1000,
		TaskTimeout: 30 * time.Second,
	}

	workers := workerpool.NewWithConfig(workerConfig)

	// Create task scheduler
	sched := scheduler.New()

	// Create data processing pipeline
	dataPipeline := pipeline.New()
	setupDataPipeline(dataPipeline)

	// Setup HTTP server
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	ws := &WebService{
		apiRateLimiter:      apiRateLimiter,
		uploadRateLimiter:   uploadRateLimiter,
		dbConnectionLimiter: dbLimiter,
		cpuIntensiveLimiter: cpuLimiter,
		backgroundWorkers:   workers,
		taskScheduler:       sched,
		dataPipeline:        dataPipeline,
		metricsRegistry:     metricsRegistry,
		httpServer:          server,
	}

	// Setup HTTP routes
	ws.setupRoutes(mux)

	return ws, nil
}

// setupDataPipeline configures a multi-stage data processing pipeline
func setupDataPipeline(p pipeline.Pipeline) {
	// Stage 1: Data validation and sanitization
	p.AddStageFunc("validate", func(ctx context.Context, input interface{}) (interface{}, error) {
		data := input.(map[string]interface{})

		// Simulate validation
		if data["id"] == nil {
			return nil, fmt.Errorf("missing required field: id")
		}

		data["validated"] = true
		data["validated_at"] = time.Now()
		return data, nil
	})

	// Stage 2: Data enrichment
	p.AddStageFunc("enrich", func(ctx context.Context, input interface{}) (interface{}, error) {
		data := input.(map[string]interface{})

		// Simulate external API call for data enrichment
		data["enriched"] = true
		data["country"] = "US" // Mock geo-location
		data["enriched_at"] = time.Now()

		return data, nil
	})

	// Stage 3: Data transformation
	p.AddStageFunc("transform", func(ctx context.Context, input interface{}) (interface{}, error) {
		data := input.(map[string]interface{})

		// Simulate business logic transformation
		if userID, ok := data["id"].(float64); ok {
			data["user_tier"] = getUserTier(int(userID))
		}

		data["transformed"] = true
		data["transformed_at"] = time.Now()

		return data, nil
	})

	// Stage 4: Data persistence
	p.AddStageFunc("persist", func(ctx context.Context, input interface{}) (interface{}, error) {
		data := input.(map[string]interface{})

		// Simulate database write
		data["persisted"] = true
		data["persisted_at"] = time.Now()

		return data, nil
	})
}

// setupRoutes configures HTTP routes with rate limiting and concurrency control
func (ws *WebService) setupRoutes(mux *http.ServeMux) {
	// API endpoint with rate limiting
	mux.HandleFunc("/api/users", ws.withRateLimit(ws.apiRateLimiter, ws.handleUsers))
	mux.HandleFunc("/api/data", ws.withRateLimit(ws.apiRateLimiter, ws.handleDataProcessing))

	// Upload endpoint with stricter rate limiting
	mux.HandleFunc("/api/upload", ws.withRateLimit(ws.uploadRateLimiter, ws.handleUpload))

	// Database operations with concurrency limiting
	mux.HandleFunc("/api/db/users", ws.withConcurrencyLimit(ws.dbConnectionLimiter, ws.handleDatabaseQuery))

	// CPU-intensive operations with concurrency limiting
	mux.HandleFunc("/api/process", ws.withConcurrencyLimit(ws.cpuIntensiveLimiter, ws.handleCPUIntensive))

	// Background task submission
	mux.HandleFunc("/api/tasks", ws.handleTaskSubmission)

	// Health check endpoint
	mux.HandleFunc("/health", ws.handleHealth)

	// Metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())
}

// withRateLimit wraps a handler with rate limiting
func (ws *WebService) withRateLimit(limiter bucket.Limiter, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		handler(w, r)
	}
}

// withConcurrencyLimit wraps a handler with concurrency limiting
func (ws *WebService) withConcurrencyLimit(limiter concurrency.Limiter, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Acquire() {
			http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		defer limiter.Release()

		handler(w, r)
	}
}

// handleUsers demonstrates basic API endpoint
func (ws *WebService) handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		fmt.Fprintf(w, `{"users": [{"id": 1, "name": "John"}, {"id": 2, "name": "Jane"}]}`)
	case "POST":
		fmt.Fprintf(w, `{"message": "User created", "id": 123}`)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleDataProcessing demonstrates pipeline integration
func (ws *WebService) handleDataProcessing(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Simulate incoming data
	inputData := map[string]interface{}{
		"id":        123,
		"timestamp": time.Now().Unix(),
		"action":    "user_signup",
	}

	// Process data through pipeline
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	result, err := ws.dataPipeline.Execute(ctx, inputData)
	if err != nil {
		http.Error(w, fmt.Sprintf("Processing failed: %v", err), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, `{"message": "Data processed successfully", "stages": %d, "duration": "%v"}`,
		len(result.StageResults), result.Duration)
}

// handleUpload demonstrates upload handling with rate limiting
func (ws *WebService) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Simulate file upload processing
	time.Sleep(100 * time.Millisecond)
	fmt.Fprintf(w, `{"message": "File uploaded successfully", "size": 1024}`)
}

// handleDatabaseQuery demonstrates database operations with concurrency limiting
func (ws *WebService) handleDatabaseQuery(w http.ResponseWriter, r *http.Request) {
	// Simulate database query
	time.Sleep(50 * time.Millisecond)
	fmt.Fprintf(w, `{"data": [{"id": 1, "value": "result1"}, {"id": 2, "value": "result2"}]}`)
}

// handleCPUIntensive demonstrates CPU-intensive operations with concurrency limiting
func (ws *WebService) handleCPUIntensive(w http.ResponseWriter, r *http.Request) {
	// Simulate CPU-intensive work
	time.Sleep(200 * time.Millisecond)
	fmt.Fprintf(w, `{"message": "Processing completed", "result": "processed_data"}`)
}

// handleTaskSubmission demonstrates background task submission
func (ws *WebService) handleTaskSubmission(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Submit background task
	task := workerpool.TaskFunc(func(ctx context.Context) error {
		// Simulate background work
		select {
		case <-time.After(1 * time.Second):
			log.Printf("Background task completed")
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	err := ws.backgroundWorkers.Submit(task)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to submit task: %v", err), http.StatusServiceUnavailable)
		return
	}

	// Acknowledge submission
	fmt.Fprintf(w, `{"message": "Task submitted successfully"}`)
}

// handleHealth provides health check endpoint
func (ws *WebService) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"workers": map[string]interface{}{
			"size":       ws.backgroundWorkers.Size(),
			"queue_size": ws.backgroundWorkers.QueueSize(),
		},
		"api_tokens":    fmt.Sprintf("%.1f", ws.apiRateLimiter.Tokens()),
		"upload_tokens": fmt.Sprintf("%.1f", ws.uploadRateLimiter.Tokens()),
		"db_available":  ws.dbConnectionLimiter.Available(),
		"cpu_available": ws.cpuIntensiveLimiter.Available(),
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"health": %+v}`, health)
}

// Start starts the web service and background tasks
func (ws *WebService) Start(ctx context.Context) error {
	// Start scheduled tasks
	err := ws.setupScheduledTasks()
	if err != nil {
		return fmt.Errorf("failed to setup scheduled tasks: %w", err)
	}

	// Start task scheduler
	ws.taskScheduler.Start()

	// Start HTTP server
	go func() {
		log.Printf("Starting web service on %s", ws.httpServer.Addr)
		log.Printf("API rate limit: %.1f RPS (burst %.0f)", ws.apiRateLimiter.Limit(), float64(ws.apiRateLimiter.Burst()))
		log.Printf("Upload rate limit: %.1f RPS (burst %.0f)", ws.uploadRateLimiter.Limit(), float64(ws.uploadRateLimiter.Burst()))
		log.Printf("DB concurrency limit: %d", ws.dbConnectionLimiter.Capacity())
		log.Printf("CPU concurrency limit: %d", ws.cpuIntensiveLimiter.Capacity())
		log.Printf("Background workers: %d", ws.backgroundWorkers.Size())
		log.Printf("Health check: http://localhost%s/health", ws.httpServer.Addr)
		log.Printf("Metrics: http://localhost%s/metrics", ws.httpServer.Addr)

		if err := ws.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()

	return ws.Shutdown()
}

// setupScheduledTasks configures periodic maintenance tasks
func (ws *WebService) setupScheduledTasks() error {
	// Schedule metrics cleanup task every 5 minutes
	err := ws.taskScheduler.ScheduleRepeating("metrics-cleanup", workerpool.TaskFunc(func(ctx context.Context) error {
		log.Println("Running scheduled metrics cleanup task")
		// Simulate metrics cleanup
		return nil
	}), 5*time.Minute)
	if err != nil {
		return err
	}

	// Schedule health check task every 30 seconds
	err = ws.taskScheduler.ScheduleRepeating("health-check", workerpool.TaskFunc(func(ctx context.Context) error {
		size := ws.backgroundWorkers.Size()
		queueSize := ws.backgroundWorkers.QueueSize()
		log.Printf("Health check - Workers: %d, Queued tasks: %d", size, queueSize)
		return nil
	}), 30*time.Second)
	if err != nil {
		return err
	}

	return nil
}

// Shutdown gracefully shuts down the web service
func (ws *WebService) Shutdown() error {
	log.Println("Shutting down web service...")

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := ws.httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	// Stop task scheduler
	ws.taskScheduler.Stop()

	// Shutdown worker pool
	<-ws.backgroundWorkers.Shutdown()

	log.Println("Web service shut down complete")
	return nil
}

// getUserTier simulates business logic
func getUserTier(userID int) string {
	switch {
	case userID < 100:
		return "premium"
	case userID < 1000:
		return "standard"
	default:
		return "basic"
	}
}

func main() {
	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Create web service
	service, err := NewWebService(port)
	if err != nil {
		log.Fatalf("Failed to create web service: %v", err)
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, initiating shutdown...", sig)
		cancel()
	}()

	// Start the service
	if err := service.Start(ctx); err != nil {
		log.Fatalf("Service error: %v", err)
	}
}
