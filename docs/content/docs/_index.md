---
title: "Documentation"
weight: 1
bookCollapseSection: false
---

# goflow Documentation

Welcome to the goflow documentation. goflow is a Go library for building concurrent applications with rate limiting, task scheduling, and streaming capabilities.

> **New to goflow?** Start with [Why goflow](../building-goflow/) to understand the problem space and architecture.

## Sections

{{< columns >}}
### [Guides]({{< relref "/docs/guides" >}})

Get started quickly with tutorials and guides.

- [Getting Started]({{< relref "/docs/guides/getting-started" >}})
- [Decision Guide]({{< relref "/docs/guides/decision-guide" >}})
- [Migration]({{< relref "/docs/guides/migration" >}})

<--->

### [Features]({{< relref "/docs/features" >}})

Learn about goflow's core features.

- [Rate Limiting]({{< relref "/docs/features/rate-limiting" >}})
- [Task Scheduling]({{< relref "/docs/features/task-scheduling" >}})
- [Streaming]({{< relref "/docs/features/streaming" >}})

{{< /columns >}}

## Quick Start

```bash
go get github.com/vnykmshr/goflow
```

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

## Resources

- [API Reference](https://pkg.go.dev/github.com/vnykmshr/goflow) — Complete API documentation
- [Examples](https://github.com/vnykmshr/goflow/tree/main/examples) — Working code examples
- [GitHub](https://github.com/vnykmshr/goflow) — Source code and issues
