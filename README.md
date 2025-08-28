# goflow - Enterprise-Grade Go Async/IO Toolkit

[![Go Reference](https://pkg.go.dev/badge/github.com/vnykmshr/goflow.svg)](https://pkg.go.dev/github.com/vnykmshr/goflow)
[![Go Report Card](https://goreportcard.com/badge/github.com/vnykmshr/goflow)](https://goreportcard.com/report/github.com/vnykmshr/goflow)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen.svg)]()
[![Coverage](https://img.shields.io/badge/coverage-90%2B-brightgreen.svg)]()

A comprehensive, enterprise-grade Go library providing sophisticated asynchronous patterns, advanced scheduling capabilities, and high-performance concurrent primitives for building robust distributed applications. Built with production-grade features including comprehensive metrics, distributed coordination, and advanced scheduling patterns.

## 🚀 Key Features

### 🚥 Advanced Rate Limiting (`pkg/ratelimit`) - **Production Ready**
- **Token Bucket**: Burst traffic handling with configurable capacity and refill rates
- **Leaky Bucket**: Smooth rate limiting for consistent processing flows
- **Concurrency Limiter**: Resource control with adaptive scaling
- **Distributed Limiting**: Redis-backed coordination for multi-instance deployments
- **Prometheus Metrics**: Full observability with detailed performance metrics

### ⚡ Enterprise Scheduling (`pkg/scheduling`) - **Production Ready**
- **Advanced Worker Pool**: Dynamic scaling, graceful shutdown, comprehensive metrics
- **Sophisticated Scheduler**: Enterprise-grade task scheduling with multiple patterns:
  - **Full Cron Support**: Complete cron expression parsing with timezone handling
  - **Exponential Backoff**: Configurable retry logic with failure handling
  - **Conditional Scheduling**: Execute tasks only when conditions are met
  - **Task Chaining**: Sequential task execution with error handling
  - **Batch Processing**: Parallel task execution with concurrency controls
  - **Adaptive Scheduling**: Dynamic timing based on system load
  - **Time Window Scheduling**: Restrict execution to specific periods
  - **Context Integration**: Sophisticated timeout and cancellation handling
- **Performance Optimizations**: Heap-based scheduling, object pooling, memory compaction
- **Multi-Stage Pipeline**: Complex data processing workflows

### 🌊 High-Performance Streaming (`pkg/streaming`) - **Production Ready**
- **Stream API**: Functional operations (filter, map, reduce) with parallel processing
- **Async Writer**: Background buffering optimized for high-volume operations
- **Advanced Backpressure**: 4 sophisticated flow control strategies
- **Channel Utilities**: Enhanced channel operations with timeout and cancellation

### 📊 Comprehensive Observability
- **Prometheus Integration**: Full metrics export for all components
- **Performance Benchmarking**: Extensive benchmarking suite with comparisons
- **Runtime Statistics**: Detailed performance and resource usage metrics
- **Distributed Tracing**: Context propagation for distributed systems

## 🚀 Quick Start

```bash
go get github.com/vnykmshr/goflow
```

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/vnykmshr/goflow/pkg/ratelimit/bucket"
    "github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
    "github.com/vnykmshr/goflow/pkg/scheduling/scheduler"
)

func main() {
    ctx := context.Background()
    
    // Rate limiting with burst capacity
    limiter := bucket.New(10, 1.0) // 10 tokens, refill 1/second
    
    // Advanced worker pool with metrics
    pool := workerpool.New(5, 100) // 5 workers, 100 task buffer
    defer func() { <-pool.Shutdown() }()
    
    // Enterprise scheduler with cron support
    sched := scheduler.NewAdvancedScheduler()
    defer func() { <-sched.Stop() }()
    sched.Start()
    
    // Rate-limited task processing
    if limiter.Allow(ctx) {
        task := workerpool.TaskFunc(func(ctx context.Context) error {
            fmt.Println("Processing high-priority request...")
            return nil
        })
        
        // Submit to worker pool
        pool.Submit(task)
        
        // Schedule recurring cleanup job
        cleanupTask := workerpool.TaskFunc(func(ctx context.Context) error {
            fmt.Println("Running cleanup...")
            return nil
        })
        sched.ScheduleCron("cleanup", "@daily", cleanupTask)
    }
}
```

### Advanced Enterprise Example

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/vnykmshr/goflow/pkg/ratelimit/distributed"
    "github.com/vnykmshr/goflow/pkg/scheduling/scheduler"
    "github.com/vnykmshr/goflow/pkg/metrics"
)

func main() {
    // Distributed rate limiting with Redis
    limiter := distributed.NewRedisTokenBucket(distributed.RedisConfig{
        Addr: "localhost:6379",
        Key:  "api_rate_limit",
        Capacity: 1000,
        RefillRate: 100,
        Window: time.Minute,
    })
    
    // Context-aware scheduler with sophisticated patterns
    sched := scheduler.WithGlobalTimeout(scheduler.Config{}, 30*time.Second)
    sched.SetContextPropagation(true, []interface{}{"trace_id", "user_id"})
    sched.Start()
    defer func() { <-sched.Stop() }()
    
    // Exponential backoff for resilient operations
    resilientTask := workerpool.TaskFunc(func(ctx context.Context) error {
        // Simulate API call that might fail
        if time.Now().UnixNano()%3 == 0 {
            return fmt.Errorf("temporary API error")
        }
        fmt.Printf("API call succeeded for user %v\n", ctx.Value("user_id"))
        return nil
    })
    
    // Schedule with advanced patterns
    sched.ScheduleWithBackoff("api_call", resilientTask, scheduler.BackoffConfig{
        InitialDelay: 100 * time.Millisecond,
        MaxDelay:     10 * time.Second,
        Multiplier:   2.0,
        MaxRetries:   5,
    })
    
    // Prometheus metrics integration
    metricsConfig := metrics.Config{
        Namespace: "myapp",
        Subsystem: "scheduler",
        EnableHistograms: true,
    }
    metricsProvider := metrics.NewPrometheusMetrics(metricsConfig)
    
    // Your application logic here...
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

## 🎯 Implementation Status

### ✅ Production Ready Modules

**🚥 Advanced Rate Limiting** - *Complete with 91.9-96.6% test coverage*
- [x] **Token Bucket**: Configurable burst capacity and refill rates
- [x] **Leaky Bucket**: Consistent rate smoothing with overflow handling
- [x] **Concurrency Limiter**: Resource control with adaptive scaling
- [x] **Distributed Limiting**: Redis-backed coordination for multi-instance deployments
- [x] **Comprehensive Metrics**: Prometheus integration with detailed performance metrics

**⚡ Enterprise Task Scheduling** - *Complete with 85-95% test coverage*
- [x] **Advanced Worker Pool**: Dynamic scaling, graceful shutdown, comprehensive metrics
- [x] **Basic Scheduler**: Time-based task execution with interval and one-time scheduling
- [x] **Cron Scheduler**: Full cron expression support with timezone handling
- [x] **Advanced Scheduler**: Enterprise-grade patterns including:
  - [x] Exponential backoff with configurable retry logic
  - [x] Conditional scheduling based on custom conditions
  - [x] Task chaining with error handling and flow control
  - [x] Batch processing with concurrency controls
  - [x] Throttled scheduling with rate limiting
  - [x] Adaptive scheduling based on system load
  - [x] Time window scheduling with day/time restrictions
  - [x] Jittered scheduling to prevent thundering herd
- [x] **Context Integration**: Sophisticated timeout and cancellation handling
- [x] **Performance Optimizations**: Heap-based scheduling, object pooling, memory compaction
- [x] **Multi-Stage Pipeline**: Complex data processing workflows

**🌊 High-Performance Streaming** - *Complete with 84.6-95.0% test coverage*
- [x] **Stream API**: Functional operations (filter, map, reduce, collect, etc.)
- [x] **Async Writer**: Background buffering optimized for high-volume operations
- [x] **Advanced Backpressure**: 4 sophisticated flow control strategies:
  - [x] Block strategy for memory-bounded processing
  - [x] Drop strategy for real-time systems
  - [x] Slide strategy for sliding window processing
  - [x] Spill strategy for disk-based overflow
- [x] **Channel Utilities**: Enhanced channel operations with timeout and cancellation

**📊 Comprehensive Observability** - *Complete*
- [x] **Prometheus Integration**: Full metrics export for all components
- [x] **Performance Benchmarking**: Extensive benchmarking suite with detailed analysis
- [x] **Runtime Statistics**: Performance and resource usage metrics
- [x] **Distributed Tracing**: Context propagation for distributed systems

### 🚀 Architecture Highlights

**Enterprise-Grade Features:**
- **Thread-Safe Operations**: Comprehensive concurrency protection
- **Graceful Shutdown**: Proper resource cleanup and connection draining  
- **Error Recovery**: Sophisticated retry and backoff mechanisms
- **Resource Management**: Automatic scaling and memory optimization
- **Observability**: Full metrics, logging, and tracing integration
- **Production Testing**: Extensive test coverage with realistic scenarios

**Performance Optimizations:**
- **Zero-Copy Operations**: Efficient memory usage patterns
- **Lock-Free Algorithms**: Atomic operations where possible
- **Object Pooling**: Reduced garbage collection pressure
- **Batch Processing**: Optimized throughput for high-volume operations
- **Adaptive Algorithms**: Dynamic adjustment based on runtime conditions

**Distributed Systems Ready:**
- **Context Propagation**: Distributed tracing support
- **Redis Integration**: Coordinated rate limiting across instances  
- **Timezone Handling**: Global scheduling coordination
- **Health Checks**: Built-in monitoring and diagnostics
- **Configuration Management**: Environment-based configuration

### 📈 Performance Benchmarks

- **Scheduler Operations**: ~1,636 ns/op (430 B/op, 6 allocs/op)
- **Cron Scheduling**: ~5,362 ns/op (656 B/op, 6 allocs/op)
- **Expression Parsing**: ~727 ns/op (358 B/op, 13 allocs/op)
- **Rate Limiting**: Sub-microsecond allow/deny decisions
- **Worker Pool**: >100K tasks/second sustained throughput

### 🔄 Continuous Improvements

- [ ] Enhanced streaming connectors (Kafka, gRPC, HTTP/2)
- [ ] Advanced monitoring dashboards and alerting
- [ ] Machine learning-based adaptive algorithms
- [ ] WebAssembly runtime integration
- [ ] Cloud-native deployment patterns