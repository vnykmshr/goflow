---
title: "Getting Started"
weight: 10
---

# Getting Started

## Installation

```bash
go get github.com/vnykmshr/goflow
```

## Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/vnykmshr/goflow/pkg/ratelimit/bucket"
    "github.com/vnykmshr/goflow/pkg/ratelimit/concurrency"
    "github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

func main() {
    rateLimiter, err := bucket.NewSafe(10, 20) // 10 RPS, burst 20
    if err != nil {
        log.Fatal(err)
    }

    concLimiter, err := concurrency.NewSafe(5) // max 5 concurrent
    if err != nil {
        log.Fatal(err)
    }

    workers, err := workerpool.NewSafe(3, 100) // 3 workers, queue 100
    if err != nil {
        log.Fatal(err)
    }
    defer func() { <-workers.Shutdown() }()

    for i := 0; i < 10; i++ {
        if rateLimiter.Allow() && concLimiter.Acquire() {
            workers.Submit(workerpool.TaskFunc(func(ctx context.Context) error {
                fmt.Printf("Processing request %d\n", i)
                concLimiter.Release()
                return nil
            }))
        }
    }
}
```

## Components

**Rate Limiting**

```go
// Token bucket - allows bursts
limiter, _ := bucket.NewSafe(100, 200) // 100 RPS, burst 200
if limiter.Allow() {
    // process request
}

// Concurrency limiting
conc, _ := concurrency.NewSafe(10) // max 10 concurrent
if conc.Acquire() {
    defer conc.Release()
    // do work
}
```

**Task Scheduling**

```go
// Worker pool
pool, _ := workerpool.NewSafe(5, 100) // 5 workers, queue size 100
pool.Submit(workerpool.TaskFunc(func(ctx context.Context) error {
    return processTask()
}))

// Cron scheduler
sched := scheduler.New()
sched.Start()
sched.ScheduleCron("backup", "@daily", taskFunc)
```

**Streaming**

```go
// Functional data processing
result, _ := stream.FromSlice([]int{1, 2, 3, 4}).
    Filter(func(x int) bool { return x > 2 }).
    Map(func(x int) int { return x * 2 }).
    ToSlice(context.Background()) // [6, 8]
```

## Error Handling

Always use safe constructors and check errors:

```go
limiter, err := bucket.NewSafe(10, 20)
if err != nil {
    log.Fatal(err)
}

// Context-aware operations
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
err = limiter.Wait(ctx)
```

## More Information

- [API Documentation](https://pkg.go.dev/github.com/vnykmshr/goflow)
- [Examples](https://github.com/vnykmshr/goflow/tree/main/examples)
- [Source Code](https://github.com/vnykmshr/goflow)
