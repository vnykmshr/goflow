---
title: "Task Scheduling"
weight: 2
---

# Task Scheduling

goflow provides worker pools for dynamic task execution and a scheduler for time-based jobs.

## Worker Pool

Worker pools manage a fixed number of goroutines that process submitted tasks.

```go
import "github.com/vnykmshr/goflow/pkg/scheduling/workerpool"

// 5 workers, queue capacity of 100 tasks
pool, err := workerpool.NewSafe(5, 100)
if err != nil {
    log.Fatal(err)
}
defer func() { <-pool.Shutdown() }()

// Submit a task
pool.Submit(workerpool.TaskFunc(func(ctx context.Context) error {
    // Do work
    return nil
}))
```

### Configuration

```go
pool, err := workerpool.NewWithConfigSafe(workerpool.Config{
    WorkerCount: 10,
    QueueSize:   1000,
    TaskTimeout: 30 * time.Second,
})
```

| Parameter | Description |
|-----------|-------------|
| WorkerCount | Number of concurrent workers |
| QueueSize | Maximum pending tasks |
| TaskTimeout | Default timeout per task |

### Graceful Shutdown

The pool supports graceful shutdown, completing in-flight tasks:

```go
// Returns a channel that closes when shutdown completes
done := pool.Shutdown()
<-done
```

### When to Use

- Background job processing
- Parallel task execution
- Bounded concurrency for resource protection

## Scheduler

The scheduler runs tasks at specified times or intervals using cron expressions.

```go
import "github.com/vnykmshr/goflow/pkg/scheduling/scheduler"

sched := scheduler.New()
sched.Start()
defer sched.Stop()

// Schedule with cron expression
sched.ScheduleCron("cleanup", "0 2 * * *", func(ctx context.Context) error {
    // Runs at 2 AM daily
    return nil
})

// Schedule at fixed intervals
sched.ScheduleInterval("health-check", 30*time.Second, func(ctx context.Context) error {
    return nil
})
```

### Cron Expressions

Standard cron format with optional seconds:

| Expression | Description |
|------------|-------------|
| `@hourly` | Every hour |
| `@daily` | Every day at midnight |
| `@weekly` | Every Sunday at midnight |
| `0 * * * *` | Every hour |
| `*/5 * * * *` | Every 5 minutes |
| `0 2 * * *` | Daily at 2 AM |

### When to Use

- Scheduled maintenance tasks
- Periodic data synchronization
- Report generation
- Health checks

## Pipeline

Pipelines process data through multiple stages.

```go
import "github.com/vnykmshr/goflow/pkg/scheduling/pipeline"

p := pipeline.New()

// Add stages using function syntax
p.AddStageFunc("validate", func(ctx context.Context, input interface{}) (interface{}, error) {
    // Validate input
    return input, nil
})
p.AddStageFunc("process", func(ctx context.Context, input interface{}) (interface{}, error) {
    // Process data
    return input, nil
})
p.AddStageFunc("store", func(ctx context.Context, input interface{}) (interface{}, error) {
    // Store result
    return input, nil
})

// Execute pipeline
result, err := p.Execute(ctx, inputData)
```

### When to Use

- ETL workflows
- Multi-stage data processing
- Assembly-line style operations

## Context Cancellation

All scheduling components respect context cancellation:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

pool.SubmitWithContext(ctx, task)
```

## API Reference

See [pkg.go.dev/github.com/vnykmshr/goflow/pkg/scheduling](https://pkg.go.dev/github.com/vnykmshr/goflow/pkg/scheduling) for complete API documentation.

## Examples

- [Worker Pool Example](https://github.com/vnykmshr/goflow/tree/main/examples/worker_pool)
