# Component Selection Guide

## When to Use What

**Rate Limiting**
- `bucket` - API endpoints, bursty traffic
- `leakybucket` - Steady processing, smooth rates
- `concurrency` - Database connections, resource protection
- `distributed` - Multi-instance applications

**Task Processing**
- `workerpool` - Dynamic background tasks
- `scheduler` - Time-based, recurring jobs
- `pipeline` - Multi-stage data processing

**Data Processing**
- `stream` - Functional operations on data
- `writer` - Async buffered writing

## Common Use Cases

**Web API Service**
- `bucket` for API rate limiting
- `concurrency` for database connections
- `workerpool` for background tasks

**Data Processing System**
- `stream` for transformations
- `pipeline` for multi-stage workflows
- `writer` for async output

**Job Processing**
- `scheduler` for cron jobs
- `workerpool` for dynamic tasks
- `concurrency` for resource control
## Configuration Examples

**API Rate Limiting**
```go
bucket.NewSafe(100, 200) // 100 RPS with burst 200
```

**Database Connections**
```go
concurrency.NewSafe(20) // Max 20 concurrent connections
```

**Worker Pool**
```go
workerpool.Config{
    WorkerCount: 10,
    QueueSize:   1000,
    TaskTimeout: 30 * time.Second,
}
```