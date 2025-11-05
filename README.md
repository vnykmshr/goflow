# goflow

[![Go Reference](https://pkg.go.dev/badge/github.com/vnykmshr/goflow.svg)](https://pkg.go.dev/github.com/vnykmshr/goflow)
[![Go Report Card](https://goreportcard.com/badge/github.com/vnykmshr/goflow)](https://goreportcard.com/report/github.com/vnykmshr/goflow)
[![CI](https://github.com/vnykmshr/goflow/actions/workflows/ci.yml/badge.svg)](https://github.com/vnykmshr/goflow/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/vnykmshr/goflow/branch/main/graph/badge.svg)](https://codecov.io/gh/vnykmshr/goflow)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A Go library for concurrent applications with rate limiting, task scheduling, and streaming.

## Features

**Rate Limiting** (`pkg/ratelimit`)
- Token bucket and leaky bucket algorithms
- Concurrency limiting
- Prometheus metrics

**Task Scheduling** (`pkg/scheduling`)
- Worker pools with graceful shutdown
- Cron-based scheduling
- Multi-stage pipelines
- Context-aware timeouts

**Streaming** (`pkg/streaming`)
- Functional stream operations
- Background buffering
- Backpressure control
- Channel utilities

## Installation

```bash
go get github.com/vnykmshr/goflow
```

## Usage

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    
    "github.com/vnykmshr/goflow/pkg/ratelimit/bucket"
    "github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
    "github.com/vnykmshr/goflow/pkg/scheduling/scheduler"
)

func main() {
    limiter, err := bucket.NewSafe(10, 20) // 10 RPS, burst 20
    if err != nil {
        log.Fatal(err)
    }

    pool, err := workerpool.NewWithConfigSafe(workerpool.Config{
        WorkerCount: 5,
        QueueSize:   100,
        TaskTimeout: 30 * time.Second,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer func() { <-pool.Shutdown() }()

    if limiter.Allow() {
        task := workerpool.TaskFunc(func(ctx context.Context) error {
            fmt.Println("Processing request...")
            return nil
        })

        if err := pool.Submit(task); err != nil {
            log.Printf("Failed to submit task: %v", err)
        }
    }
}
```

## Components

**Rate Limiters**
- `bucket.NewSafe(rate, burst)` - Token bucket with burst capacity
- `leakybucket.New(rate)` - Smooth rate limiting
- `concurrency.NewSafe(limit)` - Concurrent operations control

**Scheduling**
- `workerpool.NewSafe(workers, queueSize)` - Background task processing
- `scheduler.New()` - Cron and interval scheduling

**Streaming**
- `stream.FromSlice(data)` - Functional data processing
- `writer.New(config)` - Async buffered writing

## Documentation

- [Getting Started](./docs/GETTING_STARTED.md)
- [Decision Guide](./docs/DECISION_GUIDE.md)
- [Migration Guide](./docs/MIGRATION.md)
- [API Reference](https://pkg.go.dev/github.com/vnykmshr/goflow)
- [Examples](./examples/)

## Development

Install the pre-commit hook for automatic code quality checks:
```bash
make install-hooks
```

The hook automatically:
- Checks for potential secrets
- Formats Go code with `goimports` and `gofmt`
- Runs `golangci-lint` on changed files
- Verifies the build succeeds

Run `make test` manually before pushing changes.

## Contributing

See [CONTRIBUTING.md](./docs/CONTRIBUTING.md) for contribution guidelines.

## License

MIT License - see [LICENSE](LICENSE) for details.