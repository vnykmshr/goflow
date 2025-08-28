# Getting Started with Goflow

Goflow is a comprehensive async/IO toolkit for Go that provides rate limiting, concurrency control, scheduling, streaming, and pipeline processing capabilities. This guide will help you get started quickly and choose the right components for your needs.

## Quick Start

### Installation
```bash
go get github.com/vnykmshr/goflow
```

### Basic Usage Example
```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/vnykmshr/goflow/pkg/ratelimit/bucket"
    "github.com/vnykmshr/goflow/pkg/ratelimit/concurrency"
    "github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

func main() {
    // Create a rate limiter (10 requests per second, burst of 20)
    rateLimiter, err := bucket.NewSafe(10, 20)
    if err != nil {
        log.Fatal(err)
    }
    
    // Create a concurrency limiter (max 5 concurrent operations)
    concLimiter, err := concurrency.NewSafe(5)
    if err != nil {
        log.Fatal(err)
    }
    
    // Create a worker pool (3 workers, queue size 100)
    workers, err := workerpool.New(3, 100)
    if err != nil {
        log.Fatal(err)
    }
    defer workers.Shutdown(context.Background())
    
    // Example: Process requests with rate limiting and concurrency control
    for i := 0; i < 50; i++ {
        if rateLimiter.Allow() {
            if concLimiter.Acquire() {
                // Submit work to background pool
                workers.Submit(context.Background(), workerpool.TaskFunc(func(ctx context.Context) (interface{}, error) {
                    // Your work here
                    fmt.Printf("Processing request %d\n", i)
                    return fmt.Sprintf("result-%d", i), nil
                }))
                
                concLimiter.Release()
            } else {
                fmt.Printf("Request %d: Too many concurrent operations\n", i)
            }
        } else {
            fmt.Printf("Request %d: Rate limited\n", i)
        }
    }
}
```

## Choosing the Right Components

### 🚦 Rate Limiting: When and Which?

**When to use:**
- Protecting APIs from abuse
- Controlling resource consumption
- Implementing fair usage policies
- Managing external API call rates

**Which rate limiter to choose:**

| Use Case | Component | Configuration | Best For |
|----------|-----------|---------------|----------|
| API endpoints | `bucket.Limiter` | `bucket.NewSafe(rps, burst)` | Variable traffic with bursts |
| Steady processing | `leakybucket.Limiter` | `leakybucket.New(rate)` | Smooth, constant rate |
| Distributed services | `distributed.Limiter` | `distributed.NewLimiter(strategy, config)` | Multi-instance applications |

**Example: API Rate Limiting**
```go
// Allow 100 RPS with bursts up to 200
limiter, err := bucket.NewSafe(100, 200)
if err != nil {
    log.Fatal(err)
}

// In HTTP handler
if !limiter.Allow() {
    http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
    return
}
```

### 👥 Concurrency Control: When and How?

**When to use:**
- Limiting database connections
- Controlling CPU-intensive operations  
- Managing memory usage
- Preventing resource exhaustion

**Example: Database Connection Limiting**
```go
// Limit to 20 concurrent database operations
dbLimiter, err := concurrency.NewSafe(20)
if err != nil {
    log.Fatal(err)
}

func queryDatabase() {
    if !dbLimiter.Acquire() {
        return errors.New("too many concurrent database operations")
    }
    defer dbLimiter.Release()
    
    // Perform database operation
}
```

### 🏭 Background Processing: Worker Pools vs Schedulers?

**Worker Pools** - For dynamic, queue-based work:
```go
// Create worker pool
pool, err := workerpool.New(10, 1000) // 10 workers, 1000 queue size
if err != nil {
    log.Fatal(err)
}

// Submit dynamic tasks
result := pool.Submit(ctx, workerpool.TaskFunc(func(ctx context.Context) (interface{}, error) {
    return processData(), nil
}))
```

**Schedulers** - For time-based, recurring tasks:
```go
// Create scheduler
sched := scheduler.New()
sched.Start()

// Schedule recurring task
_, err := sched.ScheduleRepeating("cleanup", 1*time.Hour, scheduler.TaskFunc(func(ctx context.Context) error {
    return performCleanup()
}))
```

### 🔄 Data Processing: Pipelines vs Streams?

**Pipelines** - For multi-stage processing:
```go
pipeline := pipeline.New()

pipeline.AddStageFunc("validate", func(ctx context.Context, input interface{}) (interface{}, error) {
    // Validation logic
    return validatedData, nil
})

pipeline.AddStageFunc("transform", func(ctx context.Context, input interface{}) (interface{}, error) {
    // Transformation logic  
    return transformedData, nil
})

result, err := pipeline.Execute(ctx, inputData)
```

**Streams** - For functional-style data processing:
```go
result := stream.FromSlice([]int{1, 2, 3, 4, 5}).
    Filter(func(x int) bool { return x%2 == 0 }).
    Map(func(x int) int { return x * 2 }).
    Collect()
// Result: [4, 8]
```

## Common Patterns

### Pattern 1: HTTP Service with Rate Limiting
```go
func main() {
    // Create rate limiter
    limiter, err := bucket.NewSafe(100, 200) // 100 RPS, burst 200
    if err != nil {
        log.Fatal(err)
    }
    
    // Middleware function
    rateLimitMiddleware := func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if !limiter.Allow() {
                http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
    
    // Apply to routes
    http.Handle("/api/", rateLimitMiddleware(http.HandlerFunc(apiHandler)))
}
```

### Pattern 2: Background Job Processing
```go
func setupJobProcessor() {
    // Worker pool for job processing
    workers, err := workerpool.New(5, 100)
    if err != nil {
        log.Fatal(err)
    }
    
    // Scheduler for periodic jobs
    sched := scheduler.New()
    sched.Start()
    
    // Schedule cleanup job every hour
    sched.ScheduleRepeating("cleanup", time.Hour, scheduler.TaskFunc(func(ctx context.Context) error {
        return performCleanup()
    }))
    
    // Process jobs from queue
    for job := range jobQueue {
        workers.Submit(context.Background(), workerpool.TaskFunc(func(ctx context.Context) (interface{}, error) {
            return processJob(job), nil
        }))
    }
}
```

### Pattern 3: Multi-Stage Data Pipeline
```go
func createDataPipeline() pipeline.Pipeline {
    p := pipeline.New()
    
    // Stage 1: Input validation
    p.AddStageFunc("validate", func(ctx context.Context, input interface{}) (interface{}, error) {
        data := input.(UserData)
        if err := validateUser(data); err != nil {
            return nil, err
        }
        return data, nil
    })
    
    // Stage 2: Data enrichment
    p.AddStageFunc("enrich", func(ctx context.Context, input interface{}) (interface{}, error) {
        data := input.(UserData)
        enrichedData := enrichUserData(data)
        return enrichedData, nil
    })
    
    // Stage 3: Persistence
    p.AddStageFunc("save", func(ctx context.Context, input interface{}) (interface{}, error) {
        data := input.(EnrichedUserData)
        return saveToDatabase(data), nil
    })
    
    return p
}
```

### Pattern 4: Resource-Protected Operations
```go
func setupResourceManagement() {
    // Limit database connections
    dbLimiter, _ := concurrency.NewSafe(10)
    
    // Limit API calls to external service
    apiLimiter, _ := bucket.NewSafe(5, 10) // 5 RPS, burst 10
    
    // Protected database operation
    dbOperation := func() error {
        if !dbLimiter.Acquire() {
            return errors.New("database connection limit exceeded")
        }
        defer dbLimiter.Release()
        
        // Database operation
        return performDBOperation()
    }
    
    // Protected API call
    apiCall := func() error {
        if !apiLimiter.Allow() {
            return errors.New("API rate limit exceeded")
        }
        
        // External API call
        return callExternalAPI()
    }
}
```

## Error Handling Best Practices

### 1. Use Safe Constructors
```go
// ✅ Good: Returns error instead of panicking
limiter, err := bucket.NewSafe(10, 20)
if err != nil {
    return fmt.Errorf("failed to create rate limiter: %w", err)
}

// ⚠️  Deprecated: Can panic on invalid config (kept for compatibility)
limiter := bucket.New(10, 20)
```

### 2. Check Error Types
```go
import "github.com/vnykmshr/goflow/pkg/common/errors"

limiter, err := bucket.NewSafe(-1, 20)
if err != nil {
    if errors.IsValidationError(err) {
        // Handle configuration error with helpful message
        log.Printf("Configuration error: %v", err)
    }
    return err
}
```

### 3. Context-Aware Operations
```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

err := limiter.Wait(ctx)
if errors.Is(err, context.DeadlineExceeded) {
    // Handle timeout
    return errors.New("operation timed out")
}
```

## Monitoring and Metrics

### Enable Metrics Collection
```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/vnykmshr/goflow/pkg/metrics"
)

// Create metrics registry
registry := prometheus.NewRegistry()
metricsConfig := metrics.Config{
    Enabled:  true,
    Registry: registry,
}

// Create components with metrics
limiter := bucket.NewWithConfigAndMetrics(bucket.Config{
    Rate:  100,
    Burst: 200,
}, "api_limiter", metricsConfig)

// Expose metrics endpoint
http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
```

## Advanced Configuration

### Custom Rate Limiting Strategies
```go
// Token bucket for bursty traffic
tokenBucket, _ := bucket.NewSafe(100, 500)

// Leaky bucket for smooth processing  
leakyBucket := leakybucket.New(50) // 50 per second, no burst

// Distributed rate limiting across instances
config := distributed.Config{
    Redis:      redisClient,
    Key:        "api_rate_limit",
    Rate:       1000, // Global rate across all instances
    Burst:      2000,
    InstanceID: "server-1",
}
distributedLimiter, _ := distributed.NewLimiter(distributed.TokenBucket, config)
```

### Pipeline Configuration with Error Handling
```go
config := pipeline.Config{
    ErrorStrategy: pipeline.ContinueOnError, // Don't stop on stage errors
    Timeout:       30 * time.Second,         // Overall pipeline timeout
    
    // Callbacks for monitoring
    OnStageStart: func(stageName string, input interface{}) {
        log.Printf("Starting stage: %s", stageName)
    },
    OnStageComplete: func(result pipeline.StageResult) {
        log.Printf("Stage %s completed in %v", result.StageName, result.Duration)
    },
}

p := pipeline.NewWithConfig(config)
```

## Migration Guide

### From Standard Libraries

**From `golang.org/x/time/rate`:**
```go
// Old: stdlib rate limiter
import "golang.org/x/time/rate"
stdLimiter := rate.NewLimiter(rate.Limit(10), 20)

// New: goflow rate limiter with better error handling and metrics
import "github.com/vnykmshr/goflow/pkg/ratelimit/bucket"
goflowLimiter, err := bucket.NewSafe(10, 20)
if err != nil {
    log.Fatal(err)
}
```

**From Custom Worker Pools:**
```go
// Old: Custom worker pool implementation
type Job struct { /* fields */ }
jobs := make(chan Job, 100)
for i := 0; i < 10; i++ {
    go worker(jobs)
}

// New: goflow worker pool with timeouts and metrics
workers, err := workerpool.New(10, 100)
if err != nil {
    log.Fatal(err)
}
defer workers.Shutdown(context.Background())
```

## Troubleshooting

### Common Issues

1. **"Rate limiter creation failed"** - Check that rate and burst values are positive
2. **"Concurrency limit exceeded"** - Increase capacity or implement waiting with `Wait(ctx)`
3. **"Pipeline timeout"** - Increase timeout or optimize stage processing
4. **"Worker pool full"** - Increase queue size or add backpressure handling

### Debug Information

Enable debug logging and monitoring:
```go
// Health check function
func checkHealth() map[string]interface{} {
    return map[string]interface{}{
        "rate_limiter_tokens": rateLimiter.Tokens(),
        "concurrency_available": concLimiter.Available(), 
        "worker_stats": workerPool.Stats(),
        "pipeline_stats": pipeline.Stats(),
    }
}
```

## Next Steps

1. **Explore Examples**: Check out the `/examples` directory for complete applications
2. **Read Module Documentation**: Each module has comprehensive documentation and examples
3. **Performance Testing**: Use the benchmark tests to understand performance characteristics
4. **Production Deployment**: Review the web service example for production patterns

## Getting Help

- **Documentation**: Each package has detailed documentation with examples
- **Examples**: Complete working examples in the `/examples` directory  
- **Issues**: Report bugs and request features on the GitHub repository
- **Performance**: Benchmark tests show expected performance characteristics

Start with the basic patterns above and gradually incorporate more advanced features as your application grows!