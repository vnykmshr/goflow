---
title: "goflow"
type: docs
---

# goflow

Go library for building concurrent applications with rate limiting, task scheduling, and streaming.

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

- [Getting Started]({{< relref "/getting-started" >}})
- [Building goflow]({{< relref "/building-goflow" >}})
- [Decision Guide]({{< relref "/decision-guide" >}})
- [Rate Limiting]({{< relref "/rate-limiting" >}})
- [Task Scheduling]({{< relref "/task-scheduling" >}})
- [Streaming]({{< relref "/streaming" >}})
- [Migration Guide]({{< relref "/migration" >}})
- [Changelog]({{< relref "/changelog" >}})

## Resources

- [API Reference](https://pkg.go.dev/github.com/vnykmshr/goflow)
- [Examples](https://github.com/vnykmshr/goflow/tree/main/examples)
- [GitHub Repository](https://github.com/vnykmshr/goflow)
