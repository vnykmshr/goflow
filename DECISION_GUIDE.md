# Goflow Decision Guide

This guide helps you choose the right goflow components for your specific use case.

## 🎯 Quick Decision Tree

### Rate Limiting

```
Do you need to limit request rates?
├── YES → What type of traffic pattern?
│   ├── Bursty (APIs, user requests) → bucket.Limiter
│   ├── Steady (processing queues) → leakybucket.Limiter  
│   └── Multi-instance deployment → distributed.Limiter
└── NO → Skip rate limiting
```

### Concurrency Control

```
Do you need to limit concurrent operations?
├── YES → What are you protecting?
│   ├── Database connections → concurrency.Limiter
│   ├── Memory usage → concurrency.Limiter
│   ├── CPU-intensive tasks → concurrency.Limiter
│   └── External API calls → bucket.Limiter (rate) + concurrency.Limiter
└── NO → Skip concurrency limiting
```

### Background Processing

```
Do you need background task processing?
├── YES → What type of tasks?
│   ├── Dynamic/queue-based → workerpool.Pool
│   ├── Time-based/scheduled → scheduler.Scheduler
│   └── Both → Use both components together
└── NO → Skip background processing
```

### Data Processing

```
Do you need to process data?
├── YES → What pattern?
│   ├── Multi-stage workflow → pipeline.Pipeline
│   ├── Functional transformations → stream.Stream
│   └── Async buffered writing → writer.Writer
└── NO → Skip data processing
```

## 🔍 Detailed Use Case Matrix

| Your Need | Recommended Component | Alternative | When to Choose |
|-----------|----------------------|-------------|----------------|
| **API Rate Limiting** | `bucket.Limiter` | `leakybucket.Limiter` | Bucket for burst tolerance, leaky for strict rate |
| **Database Connection Limiting** | `concurrency.Limiter` | - | Always use for DB connection pools |
| **CPU-Intensive Task Control** | `concurrency.Limiter` | - | Prevent CPU saturation |
| **External API Rate Limiting** | `bucket.Limiter` | `distributed.Limiter` | Distributed for multi-instance |
| **Background Job Processing** | `workerpool.Pool` | `scheduler.Scheduler` | Pool for dynamic, scheduler for recurring |
| **Data Validation Pipeline** | `pipeline.Pipeline` | `stream.Stream` | Pipeline for stages, stream for functional |
| **Multi-Instance Rate Limiting** | `distributed.Limiter` | - | Required for distributed systems |
| **Periodic Maintenance Tasks** | `scheduler.Scheduler` | `workerpool.Pool` | Scheduler for time-based triggers |
| **Real-time Data Processing** | `stream.Stream` | `pipeline.Pipeline` | Stream for transformations, pipeline for workflows |
| **Async File Writing** | `writer.Writer` | - | For buffered, async write operations |

## 🏗️ Architecture Patterns

### Pattern 1: Web API Service
**Components Needed:**
- `bucket.Limiter` - API endpoint rate limiting
- `concurrency.Limiter` - Database connection limiting  
- `workerpool.Pool` - Background task processing
- `scheduler.Scheduler` - Periodic cleanup tasks

**Example Architecture:**
```
HTTP Request → Rate Limiter → Handler → Concurrency Limiter → Database
                                   ↓
                              Worker Pool → Background Tasks
                                   ↓
                              Scheduler → Periodic Tasks
```

### Pattern 2: Data Processing Pipeline
**Components Needed:**
- `pipeline.Pipeline` - Multi-stage data processing
- `stream.Stream` - Data transformations
- `concurrency.Limiter` - Resource protection
- `writer.Writer` - Output buffering

**Example Architecture:**
```
Input Data → Pipeline (Validate→Transform→Enrich) → Stream Processing → Writer → Output
                ↓
         Concurrency Limiter (protect resources)
```

### Pattern 3: Distributed Microservice
**Components Needed:**
- `distributed.Limiter` - Cross-instance rate limiting
- `concurrency.Limiter` - Local resource protection
- `workerpool.Pool` - Service-specific processing
- `metrics.Registry` - Monitoring

**Example Architecture:**
```
Load Balancer → [Instance 1, Instance 2, Instance N]
                      ↓
               Distributed Rate Limiter (Redis)
                      ↓
               Local Concurrency Limiters
                      ↓
               Worker Pools + Metrics
```

### Pattern 4: Batch Processing System
**Components Needed:**
- `scheduler.Scheduler` - Job scheduling
- `workerpool.Pool` - Parallel processing
- `pipeline.Pipeline` - Data processing stages
- `concurrency.Limiter` - Resource management

**Example Architecture:**
```
Scheduler → Job Queue → Worker Pool → Pipeline Processing → Output
              ↓              ↓            ↓
          Concurrency  Concurrency  Concurrency
           (Queue)     (Workers)   (Resources)
```

## 🎮 Configuration Decision Guide

### Rate Limiter Configuration

```go
// High-traffic API (allows bursts)
bucket.NewSafe(1000, 2000) // 1000 RPS, burst 2000

// Steady processing (no bursts)  
leakybucket.New(100) // 100 per second, smooth

// External API calls (respect their limits)
bucket.NewSafe(10, 20) // 10 RPS, small burst buffer

// Distributed API (total rate across instances)
distributed.Config{Rate: 5000, Burst: 10000} // 5K RPS total
```

### Concurrency Limiter Configuration

```go
// Database connections (match pool size)
concurrency.NewSafe(20) // Match your DB pool size

// CPU-intensive tasks (match CPU cores)
concurrency.NewSafe(runtime.NumCPU())

// Memory-intensive tasks (based on available memory)
concurrency.NewSafe(availableMemoryGB / taskMemoryGB)

// External service calls (based on service limits)
concurrency.NewSafe(50) // Service allows 50 concurrent connections
```

### Worker Pool Configuration

```go
// I/O bound tasks (higher worker count)
workerpool.Config{
    WorkerCount: runtime.NumCPU() * 4, // 4x CPU cores
    QueueSize:   10000,
    TaskTimeout: 30 * time.Second,
}

// CPU bound tasks (match CPU cores)
workerpool.Config{
    WorkerCount: runtime.NumCPU(),
    QueueSize:   1000,
    TaskTimeout: 60 * time.Second,
}

// Mixed workload (balanced)
workerpool.Config{
    WorkerCount: runtime.NumCPU() * 2,
    QueueSize:   5000,
    TaskTimeout: 30 * time.Second,
}
```

## 🚀 Performance Guidelines

### When to Use Each Rate Limiting Strategy

| Traffic Pattern | Component | Reasoning |
|-----------------|-----------|-----------|
| Steady 100 RPS | `leakybucket.Limiter` | Smooth, predictable rate |
| Bursty web traffic | `bucket.Limiter` | Handles traffic spikes well |
| Multiple app instances | `distributed.Limiter` | Coordinates across instances |
| Mixed patterns | Multiple limiters | Layer different strategies |

### Concurrency Limits Based on Resources

| Resource Type | Recommended Limit | Formula |
|---------------|-------------------|---------|
| Database connections | Connection pool size | Match your DB pool |
| CPU-intensive tasks | CPU cores | `runtime.NumCPU()` |
| Memory-intensive tasks | Available memory | `totalMemory / taskMemory` |
| Network calls | Service capacity | Based on external limits |

### Worker Pool Sizing

| Task Type | Workers | Queue Size | Timeout |
|-----------|---------|------------|---------|
| Fast I/O | 4 * CPU cores | 10,000+ | 5-10s |
| Slow I/O | 2 * CPU cores | 5,000+ | 30-60s |
| CPU-bound | CPU cores | 1,000+ | 60s+ |
| Mixed | 2 * CPU cores | 5,000+ | 30s |

## ⚡ Performance Optimization

### Rate Limiter Optimization
```go
// For high-throughput scenarios
limiter, _ := bucket.NewWithConfigSafe(bucket.Config{
    Rate:          10000,    // High rate
    Burst:         20000,    // High burst
    InitialTokens: 20000,    // Start with full capacity
})
```

### Memory Optimization
```go
// For memory-constrained environments
workers, _ := workerpool.NewWithConfig(workerpool.Config{
    WorkerCount: 2,          // Fewer workers
    QueueSize:   100,        // Smaller queue
    TaskTimeout: 10 * time.Second,
})
```

### Latency Optimization
```go
// For low-latency requirements
limiter, _ := concurrency.NewWithConfigSafe(concurrency.Config{
    Capacity:         100,   // High capacity
    InitialAvailable: 100,   // All permits available
})
```

## 🔧 Integration Patterns

### HTTP Middleware Integration
```go
func rateLimitMiddleware(limiter bucket.Limiter) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if !limiter.Allow() {
                http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

### gRPC Interceptor Integration
```go
func rateLimitInterceptor(limiter bucket.Limiter) grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
        if !limiter.Allow() {
            return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded")
        }
        return handler(ctx, req)
    }
}
```

### Database Integration
```go
type DB struct {
    conn    *sql.DB
    limiter concurrency.Limiter
}

func (db *DB) Query(query string, args ...interface{}) (*sql.Rows, error) {
    if !db.limiter.Acquire() {
        return nil, errors.New("too many concurrent database operations")
    }
    defer db.limiter.Release()
    
    return db.conn.Query(query, args...)
}
```

## 📊 Monitoring Integration

### Basic Monitoring
```go
// Health check function
func (s *Service) Health() map[string]interface{} {
    return map[string]interface{}{
        "rate_limiter": map[string]interface{}{
            "tokens_remaining": s.rateLimiter.Tokens(),
            "rate":            s.rateLimiter.Limit(),
        },
        "concurrency": map[string]interface{}{
            "available": s.concLimiter.Available(),
            "capacity":  s.concLimiter.Capacity(),
        },
        "workers": s.workers.Stats(),
    }
}
```

### Metrics Collection
```go
// Enable metrics for all components
metricsConfig := metrics.Config{
    Enabled:  true,
    Registry: prometheus.DefaultRegisterer,
}

limiter := bucket.NewWithConfigAndMetrics(config, "api_limiter", metricsConfig)
workers := workerpool.NewWithConfig(workerpool.Config{
    EnableMetrics: true,
    MetricsConfig: metricsConfig,
})
```

## 🎯 Quick Reference

### "I want to..." → "Use this component"

| Goal | Component | Quick Example |
|------|-----------|---------------|
| Limit API requests | `bucket.Limiter` | `bucket.NewSafe(100, 200)` |
| Control DB connections | `concurrency.Limiter` | `concurrency.NewSafe(20)` |
| Process background jobs | `workerpool.Pool` | `workerpool.New(10, 1000)` |
| Schedule recurring tasks | `scheduler.Scheduler` | `scheduler.New().ScheduleRepeating(...)` |
| Build data pipeline | `pipeline.Pipeline` | `pipeline.New().AddStageFunc(...)` |
| Transform data streams | `stream.Stream` | `stream.FromSlice(...).Map(...).Filter(...)` |
| Rate limit across servers | `distributed.Limiter` | `distributed.NewLimiter(strategy, config)` |
| Buffer async writes | `writer.Writer` | `writer.New(output)` |

This decision guide should help you choose the right goflow components for your specific needs. When in doubt, start simple and add complexity as needed!