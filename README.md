# goflow - Go Async/IO Toolkit

[![Go Reference](https://pkg.go.dev/badge/github.com/vnykmshr/goflow.svg)](https://pkg.go.dev/github.com/vnykmshr/goflow)
[![Go Report Card](https://goreportcard.com/badge/github.com/vnykmshr/goflow)](https://goreportcard.com/report/github.com/vnykmshr/goflow)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A comprehensive Go library that provides high-level, opinionated, and powerful asynchronous patterns for building robust concurrent applications. Instead of mimicking event-driven, future-based models, goflow provides a set of idiomatic Go primitives that leverage standard concurrency features like goroutines, channels, and the context package.

## Features

### 🚥 Rate Limiting (`pkg/ratelimit`)
- **Token Bucket**: Allows burst traffic with configurable capacity and refill rate
- **Leaky Bucket**: Smooths out bursty traffic for consistent processing rate  
- **Concurrency Limiter**: Controls the number of concurrent goroutines

### ⚡ Task Scheduling (`pkg/scheduling`)
- **Worker Pool**: Reusable goroutine pool with graceful shutdown
- **Scheduler**: Time-based task execution with cron-like functionality
- **Pipeline**: Multi-stage concurrent data pipeline builder

### 🌊 Streaming & I/O (`pkg/streaming`)
- **Stream API**: High-level data stream operations (filter, map, reduce)
- **Async Writer**: Background buffering for high-volume write operations
- **Backpressure**: Flow control to prevent resource exhaustion

## Quick Start

```bash
go get github.com/vnykmshr/goflow
```

```go
package main

import (
    "context"
    "fmt"
    "github.com/vnykmshr/goflow/pkg/ratelimit/bucket"
    "github.com/vnykmshr/goflow/pkg/scheduling/pool"
)

func main() {
    ctx := context.Background()
    
    // Create a rate limiter (10 tokens, refill at 1/second)
    limiter := bucket.New(10, 1.0)
    
    // Create a worker pool with 5 workers
    workerPool := pool.New(5)
    defer workerPool.Close()
    
    // Rate-limited request processing
    if limiter.Allow(ctx) {
        task := func(ctx context.Context) error {
            fmt.Println("Processing request...")
            return nil
        }
        workerPool.Submit(ctx, task)
    }
}
```

## Design Principles

- **Idiomatic Go**: Uses standard Go concurrency primitives (goroutines, channels, context)
- **Simplicity**: Clean APIs that abstract common boilerplate and pitfalls
- **Performance**: High-performance and low overhead design
- **Modularity**: Loosely coupled packages for selective imports
- **Robustness**: Graceful failure handling with context integration

## Installation

```bash
go get github.com/vnykmshr/goflow
```

## Documentation

- [API Documentation](https://pkg.go.dev/github.com/vnykmshr/goflow)
- [Examples](./examples/)

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](./CONTRIBUTING.md) for details.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Status

### ✅ Completed Features

**Rate Limiting Module** - *Complete with 91.9-96.6% test coverage*
- [x] Token bucket rate limiter with burst capacity
- [x] Leaky bucket for consistent rate smoothing  
- [x] Concurrency limiter for resource control

**Task Scheduling Module** - *Complete with 76.6-94.4% test coverage*
- [x] Worker pool with dynamic scaling and graceful shutdown
- [x] Task scheduler with cron-like functionality
- [x] Multi-stage data processing pipeline

**Streaming & I/O Module** - *Complete with 84.6-95.0% test coverage*  
- [x] Stream API with functional operations (filter, map, reduce, etc.)
- [x] Async writer with background buffering
- [x] Backpressure channels with 4 flow control strategies

### 🚀 Future Enhancements

- [ ] Instrumentation and metrics support (Prometheus integration)
- [ ] Distributed systems integration (Redis-based rate limiting)
- [ ] Advanced scheduling patterns (cron expressions)
- [ ] Performance optimizations and benchmarking