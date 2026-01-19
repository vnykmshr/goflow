---
title: "Rate Limiting"
weight: 1
---

# Rate Limiting

goflow provides two rate limiting strategies: token bucket for bursty workloads and leaky bucket for smooth, consistent rates.

## Token Bucket

The token bucket algorithm allows bursts up to a configured capacity while maintaining an average rate.

```go
import "github.com/vnykmshr/goflow/pkg/ratelimit/bucket"

// 100 requests per second with burst capacity of 200
limiter, err := bucket.NewSafe(100, 200)
if err != nil {
    log.Fatal(err)
}

if limiter.Allow() {
    // Process request
}
```

### When to Use

- API rate limiting where occasional bursts are acceptable
- User-facing endpoints with variable traffic
- External API calls with rate limits

### Configuration

| Parameter | Description |
|-----------|-------------|
| Rate | Tokens added per second |
| Capacity | Maximum tokens (burst size) |

## Leaky Bucket

The leaky bucket enforces a strict, constant rate without allowing bursts.

```go
import "github.com/vnykmshr/goflow/pkg/ratelimit/leakybucket"

// Steady 10 requests per second, queue up to 100
limiter, err := leakybucket.NewSafe(10, 100)
if err != nil {
    log.Fatal(err)
}

if limiter.Allow() {
    // Process request
}
```

### When to Use

- Background job processing at a consistent rate
- Protecting downstream services that require steady traffic
- Scenarios where burst traffic causes problems

## Concurrency Limiter

For controlling the number of concurrent operations rather than rate:

```go
import "github.com/vnykmshr/goflow/pkg/ratelimit/concurrency"

// Maximum 10 concurrent operations
limiter, err := concurrency.NewSafe(10)
if err != nil {
    log.Fatal(err)
}

if limiter.Acquire() {
    defer limiter.Release()
    // Do work
}
```

### When to Use

- Database connection pooling
- Limiting concurrent file operations
- Protecting shared resources

## Context Support

All limiters support context-aware waiting:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

err := limiter.Wait(ctx)
if err != nil {
    // Timeout or cancellation
}
```

## API Reference

See [pkg.go.dev/github.com/vnykmshr/goflow/pkg/ratelimit](https://pkg.go.dev/github.com/vnykmshr/goflow/pkg/ratelimit) for complete API documentation.

## Examples

- [Rate Limiter Example](https://github.com/vnykmshr/goflow/tree/main/examples/rate_limiter)
- [Distributed Rate Limiting](https://github.com/vnykmshr/goflow/tree/main/examples/distributed-rate-limiting)
