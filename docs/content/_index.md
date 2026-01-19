---
title: "goflow"
type: docs
---

# goflow

Go library for building concurrent applications with rate limiting, task scheduling, and streaming.

> **New to goflow?** Read [Why goflow](docs/building-goflow/) to understand the problem space and architecture.

## Installation

```bash
go get github.com/vnykmshr/goflow
```

## Quick Example

```go
package main

import (
    "context"
    "log"

    "github.com/vnykmshr/goflow/pkg/ratelimit/bucket"
    "github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

func main() {
    limiter, _ := bucket.NewSafe(100, 200)
    pool, _ := workerpool.NewSafe(5, 100)
    defer func() { <-pool.Shutdown() }()

    if limiter.Allow() {
        pool.Submit(workerpool.TaskFunc(func(ctx context.Context) error {
            log.Println("Processing request")
            return nil
        }))
    }
}
```

## Documentation

**Guides**
- [Getting Started]({{< relref "/docs/guides/getting-started" >}}) — Installation and basic usage
- [Decision Guide]({{< relref "/docs/guides/decision-guide" >}}) — Choosing the right components
- [Migration]({{< relref "/docs/guides/migration" >}}) — Upgrading from previous versions

**Features**
- [Rate Limiting]({{< relref "/docs/features/rate-limiting" >}}) — Token bucket, leaky bucket, concurrency
- [Task Scheduling]({{< relref "/docs/features/task-scheduling" >}}) — Worker pools, cron scheduler
- [Streaming]({{< relref "/docs/features/streaming" >}}) — Functional data processing

**Reference**
- [Contributing]({{< relref "/docs/reference/contributing" >}}) — How to contribute
- [Security]({{< relref "/docs/reference/security" >}}) — Security policy
- [Changelog]({{< relref "/docs/reference/changelog" >}}) — Release history

## Resources

- [API Reference](https://pkg.go.dev/github.com/vnykmshr/goflow) — Complete API documentation
- [Examples](https://github.com/vnykmshr/goflow/tree/main/examples) — Working code examples
- [GitHub Repository](https://github.com/vnykmshr/goflow) — Source code and issues
