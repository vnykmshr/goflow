# Goflow Web Service Example

This example demonstrates how to build a production-ready web service using multiple goflow modules working together:

## Features Demonstrated

### 🚦 **Rate Limiting**
- API endpoints protected with token bucket rate limiting
- Different limits for different endpoint types (API vs uploads)
- Graceful rate limit error handling

### 👥 **Concurrency Control**
- Database connection limiting to prevent resource exhaustion
- CPU-intensive task limiting to maintain server responsiveness
- Semaphore-based permit management

### 🏭 **Background Processing**
- Worker pool for async task processing
- Configurable worker count and queue size
- Task timeout and error handling

### 🔄 **Data Processing Pipelines**
- Multi-stage data processing with validation, enrichment, transformation
- Context-aware processing with timeouts
- Pipeline error handling and recovery

### ⏰ **Task Scheduling**
- Periodic maintenance tasks
- Health check monitoring
- Configurable scheduling intervals

### 📊 **Integrated Metrics**
- Prometheus metrics collection
- HTTP metrics endpoint
- Component-specific metrics

## Running the Example

### Prerequisites
- Go 1.19 or later
- Basic understanding of HTTP services

### Quick Start
```bash
# Clone the repository
git clone <repository-url>
cd goflow/examples/web-service

# Run the service
go run main.go

# Or with custom port
PORT=9090 go run main.go
```

The service will start on http://localhost:8080 (or your specified port).

## API Endpoints

### Core Endpoints
- `GET /api/users` - List users (rate limited: 100 RPS)
- `POST /api/users` - Create user (rate limited: 100 RPS)
- `POST /api/data` - Process data through pipeline (rate limited: 100 RPS)
- `POST /api/upload` - File upload (rate limited: 10 RPS)
- `GET /api/db/users` - Database query (concurrency limited: 20)
- `POST /api/process` - CPU-intensive task (concurrency limited: 4)
- `POST /api/tasks` - Submit background task

### Monitoring Endpoints
- `GET /health` - Health check with component status
- `GET /metrics` - Prometheus metrics

## Testing the Service

### Rate Limiting
Test API rate limiting (should start failing after 100 requests):
```bash
# Fast burst test
for i in {1..150}; do curl -s http://localhost:8080/api/users & done; wait

# Upload rate limiting (should start failing after 10 requests)
for i in {1..20}; do curl -X POST -s http://localhost:8080/api/upload & done; wait
```

### Concurrency Limiting
Test concurrency limiting:
```bash
# Database concurrency (should handle 20 concurrent, queue others)
for i in {1..30}; do curl -s http://localhost:8080/api/db/users & done; wait

# CPU concurrency (should handle 4 concurrent, reject others)
for i in {1..10}; do curl -X POST -s http://localhost:8080/api/process & done; wait
```

### Data Pipeline
Test the data processing pipeline:
```bash
curl -X POST http://localhost:8080/api/data
# Returns: {"message": "Data processed successfully", "stages": 4, "duration": "..."}
```

### Background Tasks
Submit background tasks:
```bash
curl -X POST http://localhost:8080/api/tasks
# Returns: {"message": "Task submitted", "task_id": "..."}
```

### Health Monitoring
Check service health:
```bash
curl http://localhost:8080/health
# Returns comprehensive health information
```

### Metrics
View Prometheus metrics:
```bash
curl http://localhost:8080/metrics
# Returns Prometheus-format metrics
```

## Architecture Overview

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────────┐
│   HTTP Client   │────│  Rate Limiters   │────│   HTTP Handlers     │
└─────────────────┘    │  • API: 100 RPS  │    │  • /api/users       │
                       │  • Upload: 10RPS │    │  • /api/data        │
                       └──────────────────┘    │  • /api/upload      │
                                              └─────────────────────┘
                                                         │
                ┌─────────────────────────────────────────────────────┐
                │
                ▼
┌─────────────────────┐  ┌─────────────────────┐  ┌─────────────────────┐
│ Concurrency Limiters│  │  Data Pipeline      │  │  Background Workers │
│ • DB: 20 max        │  │  • Validate         │  │  • 10 workers       │
│ • CPU: 4 max        │  │  • Enrich           │  │  • 1000 queue size  │
└─────────────────────┘  │  • Transform        │  │  • 30s timeout      │
                        │  • Persist          │  └─────────────────────┘
                        └─────────────────────┘
                                   │
                                   ▼
                        ┌─────────────────────┐
                        │   Task Scheduler    │
                        │  • Cleanup: 5min    │
                        │  • Health: 30sec    │
                        └─────────────────────┘
                                   │
                                   ▼
                        ┌─────────────────────┐
                        │  Metrics Registry   │
                        │  • Prometheus       │
                        │  • /metrics         │
                        └─────────────────────┘
```

## Key Implementation Patterns

### 1. Safe Constructor Usage
```go
// Use the new safe constructors for better error handling
apiLimiter, err := bucket.NewSafe(100, 200)
if err != nil {
    return nil, fmt.Errorf("failed to create rate limiter: %w", err)
}
```

### 2. Middleware Pattern for Rate Limiting
```go
func (ws *WebService) withRateLimit(limiter bucket.Limiter, handler http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if !limiter.Allow() {
            http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
            return
        }
        handler(w, r)
    }
}
```

### 3. Context-Aware Processing
```go
ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
defer cancel()

result, err := ws.dataPipeline.Execute(ctx, inputData)
```

### 4. Graceful Shutdown
```go
shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

ws.httpServer.Shutdown(shutdownCtx)
ws.taskScheduler.Stop()
ws.backgroundWorkers.Shutdown(shutdownCtx)
```

## Configuration

The service demonstrates various configuration patterns:

### Rate Limiting Configuration
- **API Rate Limit**: 100 requests/second, burst 200
- **Upload Rate Limit**: 10 requests/second, burst 50

### Concurrency Configuration
- **Database Connections**: Max 20 concurrent
- **CPU-Intensive Tasks**: Max 4 concurrent

### Worker Pool Configuration
- **Workers**: 10 concurrent workers
- **Queue Size**: 1000 pending tasks
- **Task Timeout**: 30 seconds

### Scheduler Configuration
- **Metrics Cleanup**: Every 5 minutes
- **Health Checks**: Every 30 seconds

## Production Considerations

1. **Monitoring**: The `/health` and `/metrics` endpoints provide comprehensive monitoring
2. **Error Handling**: All components use proper error handling and context timeouts
3. **Resource Management**: Concurrency limits prevent resource exhaustion
4. **Graceful Shutdown**: Clean shutdown of all components
5. **Configuration**: Environment-based configuration for flexibility

## Learning Objectives

After studying this example, you should understand:

- How to combine multiple goflow modules effectively
- Production-ready error handling patterns
- Resource protection strategies
- Monitoring and observability integration
- Graceful shutdown implementation
- Context-aware processing patterns

## Next Steps

- Explore individual module documentation for advanced features
- Implement custom metrics for your specific use case
- Add authentication and authorization layers
- Integrate with external services (databases, message queues)
- Deploy with container orchestration platforms

## Troubleshooting

### Common Issues

1. **Rate Limit Exceeded**: Adjust rate limits or implement retry logic with exponential backoff
2. **Service Unavailable**: Check concurrency limits and queue capacity
3. **Task Timeout**: Adjust task timeouts or optimize task implementation
4. **Memory Usage**: Monitor worker pool queue size and consider backpressure

### Debug Endpoints

- Health status: `curl localhost:8080/health`
- Current metrics: `curl localhost:8080/metrics`
- Component status available in health endpoint

This example serves as a foundation for building robust, scalable web services with proper rate limiting, concurrency control, and monitoring.