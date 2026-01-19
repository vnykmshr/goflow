---
title: "Building goflow"
weight: 1
---

# Building Concurrent Systems in Go

Modern applications demand concurrency to handle thousands of simultaneous operations. Rate limiting prevents resource exhaustion. Worker pools manage background tasks. Stream processing handles data pipelines. Each requires careful coordination of goroutines, channels, and context cancellation.

goflow provides production-ready concurrency primitives that handle the difficult edge cases: graceful shutdown, context propagation, backpressure, and resource protection. Rather than reimplementing token buckets or worker pools for each project, developers compose these primitives into reliable concurrent systems.

## Problem Space

Three concurrency challenges plague Go applications:

**Resource Protection**: API endpoints need rate limiting to prevent abuse. Database connections require concurrency limits to avoid exhaustion. CPU-intensive operations must be throttled to maintain system responsiveness.

**Background Processing**: Web services queue background tasks. Batch jobs process data asynchronously. Scheduled maintenance runs periodically. Each requires worker coordination, timeout handling, and graceful shutdown.

**Data Pipelines**: Streaming data flows through validation, enrichment, and persistence stages. Each stage must handle errors, apply backpressure, and process elements efficiently without blocking.

Implementing these patterns correctly requires managing goroutine lifecycles, handling cancellation propagation, coordinating shutdown sequences, and preventing resource leaks. Most implementations get these details wrong.

## Architecture

goflow provides three core modules that compose together:

```mermaid
graph TB
    subgraph "Rate Limiting"
        TB[Token Bucket<br/>Bursty Traffic]
        LB[Leaky Bucket<br/>Smooth Rate]
        CL[Concurrency Limiter<br/>Resource Protection]
    end

    subgraph "Task Scheduling"
        WP[Worker Pool<br/>Background Tasks]
        SC[Scheduler<br/>Cron/Interval]
        PL[Pipeline<br/>Multi-Stage Processing]
    end

    subgraph "Streaming"
        ST[Stream<br/>Functional Operations]
        WR[Writer<br/>Async Buffering]
        CH[Channel Utils<br/>Coordination]
    end

    API[API Endpoints] --> TB
    API --> CL
    API --> WP

    WP --> PL
    SC --> WP

    DATA[Data Sources] --> ST
    ST --> WR

    TB -.-> |Tokens/Second| API
    CL -.-> |Max Concurrent| API
    WP -.-> |Task Queue| PL
```

### Token Bucket Rate Limiter

The token bucket algorithm allows burst traffic while enforcing average rate limits. Tokens accumulate at a fixed rate up to a burst capacity. Each operation consumes tokens. When tokens are available, requests proceed immediately. When exhausted, requests either wait or fail.

```mermaid
sequenceDiagram
    participant Client
    participant TokenBucket
    participant Resource

    Note over TokenBucket: Initial: 20 tokens<br/>Rate: 10/sec, Burst: 20

    Client->>TokenBucket: Allow(1)
    TokenBucket->>TokenBucket: Check tokens: 20 >= 1
    TokenBucket->>TokenBucket: Consume 1 token (19 left)
    TokenBucket->>Client: true
    Client->>Resource: Access granted

    Note over TokenBucket: After burst: 0 tokens

    Client->>TokenBucket: WaitN(ctx, 5)
    TokenBucket->>TokenBucket: Calculate wait: 0.5s
    TokenBucket-->>Client: Wait 0.5s
    Note over TokenBucket: Tokens refill...
    TokenBucket->>Client: Proceed
    Client->>Resource: Access granted
```

### Worker Pool Architecture

Worker pools manage a fixed number of goroutines processing tasks from a bounded queue. This prevents goroutine explosions while providing backpressure through queue capacity.

```mermaid
graph LR
    subgraph "Task Submission"
        S1[Submit Task 1]
        S2[Submit Task 2]
        S3[Submit Task N]
    end

    subgraph "Worker Pool"
        Q[Task Queue<br/>Buffered Channel]

        subgraph "Workers"
            W1[Worker 1]
            W2[Worker 2]
            W3[Worker N]
        end

        Q --> W1
        Q --> W2
        Q --> W3
    end

    subgraph "Results"
        R[Result Channel]
    end

    S1 --> Q
    S2 --> Q
    S3 --> Q

    W1 --> R
    W2 --> R
    W3 --> R

    style Q fill:#e1f5ff
    style R fill:#e1ffe1
```

Each worker runs independently, pulling tasks from the shared queue. Context cancellation propagates to all workers for coordinated shutdown. Panics are recovered and reported as errors.

### Pipeline Processing

Pipelines chain stages together, passing output from one stage as input to the next. Each stage can transform data, perform validation, or trigger side effects.

```mermaid
graph LR
    I[Input Data] --> S1[Stage 1<br/>Validate]
    S1 --> S2[Stage 2<br/>Enrich]
    S2 --> S3[Stage 3<br/>Transform]
    S3 --> S4[Stage 4<br/>Persist]
    S4 --> O[Output Result]

    S1 -.-> M1[Metrics]
    S2 -.-> M2[Metrics]
    S3 -.-> M3[Metrics]
    S4 -.-> M4[Metrics]

    M1 --> Stats[Pipeline Statistics]
    M2 --> Stats
    M3 --> Stats
    M4 --> Stats

    style S1 fill:#fff3cd
    style S2 fill:#cff4fc
    style S3 fill:#d1e7dd
    style S4 fill:#f8d7da
```

Pipelines track execution time and errors for each stage, enabling debugging slow stages and understanding failure modes.

### Stream Processing

Streams provide functional operations (map, filter, reduce) over data sources. Operations are lazy, executing only when terminal operations consume the stream.

```mermaid
graph TB
    Source[Stream Source<br/>Slice/Channel/Generator]

    subgraph "Intermediate Operations (Lazy)"
        Filter[Filter<br/>Predicate Selection]
        Map[Map<br/>Transformation]
        Distinct[Distinct<br/>Deduplication]
        Sorted[Sorted<br/>Ordering]
    end

    subgraph "Terminal Operations (Eager)"
        ForEach[ForEach<br/>Side Effects]
        ToSlice[ToSlice<br/>Collection]
        Reduce[Reduce<br/>Aggregation]
        Count[Count<br/>Enumeration]
    end

    Source --> Filter
    Filter --> Map
    Map --> Distinct
    Distinct --> Sorted

    Sorted --> ForEach
    Sorted --> ToSlice
    Sorted --> Reduce
    Sorted --> Count

    style Source fill:#e1f5ff
    style Filter fill:#fff3cd
    style Map fill:#fff3cd
    style Distinct fill:#fff3cd
    style Sorted fill:#fff3cd
    style ForEach fill:#d1e7dd
    style ToSlice fill:#d1e7dd
    style Reduce fill:#d1e7dd
    style Count fill:#d1e7dd
```

## Code Examples

### Rate Limiting API Endpoints

```go
package main

import (
    "net/http"
    "github.com/vnykmshr/goflow/pkg/ratelimit/bucket"
)

func main() {
    // 100 requests per second, burst of 200
    limiter, _ := bucket.NewSafe(100, 200)

    http.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
        if !limiter.Allow() {
            http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
            return
        }
        w.Write([]byte("Success"))
    })

    http.ListenAndServe(":8080", nil)
}
```

The token bucket handles traffic spikes gracefully. Burst capacity allows 200 requests immediately, then refills at 100/second.

### Background Task Processing

```go
package main

import (
    "context"
    "log"
    "time"
    "github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

func main() {
    pool, _ := workerpool.NewWithConfigSafe(workerpool.Config{
        WorkerCount: 10,
        QueueSize:   1000,
        TaskTimeout: 30 * time.Second,
    })
    defer func() { <-pool.Shutdown() }()

    task := workerpool.TaskFunc(func(ctx context.Context) error {
        select {
        case <-time.After(5 * time.Second):
            log.Println("Task completed")
            return nil
        case <-ctx.Done():
            return ctx.Err()
        }
    })

    if err := pool.Submit(task); err != nil {
        log.Printf("Failed to submit: %v", err)
    }
}
```

Worker pools prevent goroutine explosions. Bounded queues provide backpressure, rejecting tasks when overloaded.

### Multi-Stage Data Pipeline

```go
package main

import (
    "context"
    "fmt"
    "github.com/vnykmshr/goflow/pkg/scheduling/pipeline"
)

func main() {
    p := pipeline.New()

    p.AddStageFunc("validate", func(ctx context.Context, input interface{}) (interface{}, error) {
        data := input.(map[string]interface{})
        if data["id"] == nil {
            return nil, fmt.Errorf("missing id field")
        }
        return data, nil
    })

    p.AddStageFunc("enrich", func(ctx context.Context, input interface{}) (interface{}, error) {
        data := input.(map[string]interface{})
        data["enriched"] = true
        return data, nil
    })

    p.AddStageFunc("transform", func(ctx context.Context, input interface{}) (interface{}, error) {
        data := input.(map[string]interface{})
        data["processed"] = true
        return data, nil
    })

    result, err := p.Execute(context.Background(), map[string]interface{}{
        "id":    123,
        "value": "test",
    })

    if err != nil {
        fmt.Printf("Pipeline failed: %v\n", err)
    } else {
        fmt.Printf("Pipeline completed in %v\n", result.Duration)
    }
}
```

### Functional Stream Processing

```go
package main

import (
    "context"
    "fmt"
    "strings"
    "github.com/vnykmshr/goflow/pkg/streaming/stream"
)

func main() {
    data := []string{"apple", "banana", "cherry", "date", "elderberry"}

    result, _ := stream.FromSlice(data).
        Filter(func(s string) bool {
            return len(s) > 5
        }).
        Map(func(s string) string {
            return strings.ToUpper(s)
        }).
        Sorted(func(a, b string) int {
            return strings.Compare(a, b)
        }).
        ToSlice(context.Background())

    fmt.Println(result) // [BANANA CHERRY ELDERBERRY]
}
```

Streams are lazy, processing elements only when terminal operations execute.

### Complete Web Service Integration

```go
package main

import (
    "context"
    "net/http"
    "time"
    "github.com/vnykmshr/goflow/pkg/ratelimit/bucket"
    "github.com/vnykmshr/goflow/pkg/ratelimit/concurrency"
    "github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
    "github.com/vnykmshr/goflow/pkg/scheduling/pipeline"
)

type WebService struct {
    apiLimiter   bucket.Limiter
    dbLimiter    concurrency.Limiter
    workers      workerpool.Pool
    dataPipeline pipeline.Pipeline
}

func NewWebService() (*WebService, error) {
    apiLimiter, _ := bucket.NewSafe(100, 200)
    dbLimiter, _ := concurrency.NewSafe(20)

    workers, _ := workerpool.NewWithConfigSafe(workerpool.Config{
        WorkerCount: 10,
        QueueSize:   1000,
        TaskTimeout: 30 * time.Second,
    })

    dataPipeline := pipeline.New()

    return &WebService{
        apiLimiter:   apiLimiter,
        dbLimiter:    dbLimiter,
        workers:      workers,
        dataPipeline: dataPipeline,
    }, nil
}

func (ws *WebService) handleRequest(w http.ResponseWriter, r *http.Request) {
    if !ws.apiLimiter.Allow() {
        http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
        return
    }

    if !ws.dbLimiter.Acquire() {
        http.Error(w, "Service busy", http.StatusServiceUnavailable)
        return
    }
    defer ws.dbLimiter.Release()

    result, err := ws.dataPipeline.Execute(r.Context(), getData(r))
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    task := workerpool.TaskFunc(func(ctx context.Context) error {
        return processAsync(result.Output)
    })
    ws.workers.Submit(task)

    w.Write([]byte("Success"))
}

func getData(r *http.Request) map[string]interface{} {
    return map[string]interface{}{}
}

func processAsync(data interface{}) error {
    return nil
}
```

This demonstrates composing primitives into a production system with proper resource management and graceful degradation.

## Design Decisions

### Context-Aware Operations

All operations accept `context.Context` parameters, enabling cancellation propagation and deadline enforcement:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

err := limiter.WaitN(ctx, 10)
result, _ := stream.ToSlice(ctx)
```

This prevents resource leaks and enables coordinated shutdown.

### Graceful Shutdown

Worker pools and schedulers support graceful shutdown, draining in-flight tasks before terminating:

```go
done := pool.Shutdown()
<-done
```

Shutdown does not accept new tasks but completes queued work. This prevents data loss during restart.

### Panic Recovery

Worker tasks run in isolated goroutines with panic recovery:

```go
task := workerpool.TaskFunc(func(ctx context.Context) error {
    panic("unexpected error")
})
```

Panics are converted to errors and returned through result channels, preventing worker pool corruption.

### Thread Safety

All limiters and pools are thread-safe with fine-grained locking. Multiple goroutines can safely call methods concurrently without external synchronization.

### Zero Dependencies

The core library has zero external dependencies beyond the Go standard library. This minimizes supply chain risk and simplifies maintenance.

### Metrics and Observability

Pipelines track execution statistics automatically:

```go
stats := pipeline.Stats()
fmt.Printf("Total executions: %d\n", stats.TotalExecutions)
fmt.Printf("Average duration: %v\n", stats.AverageDuration)
```

## Real-World Applications

### API Gateway Rate Limiting

```go
clientLimiters := make(map[string]bucket.Limiter)

func rateLimit(clientID string) bool {
    limiter, exists := clientLimiters[clientID]
    if !exists {
        limiter, _ = bucket.NewSafe(100, 200)
        clientLimiters[clientID] = limiter
    }
    return limiter.Allow()
}
```

Burst capacity handles legitimate spikes while preventing sustained abuse.

### Background Job Processing

```go
emailPool, _ := workerpool.NewSafe(5, 100)
reportPool, _ := workerpool.NewSafe(2, 50)

emailPool.Submit(sendEmailTask)
reportPool.Submit(generateReportTask)
```

Separate pools prevent one job type from starving others.

### ETL Pipelines

```go
etl := pipeline.New()
etl.AddStageFunc("extract", extractFromSource)
etl.AddStageFunc("validate", validateRecords)
etl.AddStageFunc("transform", applyBusinessRules)
etl.AddStageFunc("load", writeToWarehouse)
```

Per-stage metrics identify bottlenecks in the pipeline.

### Database Connection Pooling

```go
dbLimiter, _ := concurrency.NewSafe(20)

func queryDatabase(query string) ([]Row, error) {
    if !dbLimiter.Acquire() {
        return nil, errors.New("too many concurrent queries")
    }
    defer dbLimiter.Release()
    return db.Query(query)
}
```

This prevents thundering herd problems during traffic spikes.

### Stream Processing Systems

```go
events := stream.FromChannel(eventChannel).
    Filter(isImportantEvent).
    Map(enrichWithMetadata).
    Distinct().
    Peek(sendToMetrics)

events.ForEach(context.Background(), processEvent)
```

Lazy evaluation ensures efficient memory usage for large streams.

## When to Use goflow

**Use goflow when:**

- Building web services that need rate limiting and background processing
- Processing data through multi-stage pipelines with error tracking
- Managing resource pools with bounded concurrency
- Requiring production-ready concurrency primitives with proper shutdown handling
- Needing functional stream operations over Go data structures

**Consider alternatives when:**

- Building simple CLI tools with no concurrency requirements
- Requiring distributed rate limiting across multiple servers (use Redis-based limiters)
- Needing advanced stream processing features like windowing (use Apache Flink or similar)
- Working with extremely high-throughput scenarios where lock contention becomes a bottleneck
- Requiring dynamic worker pool scaling based on load

## When Not to Use goflow

- Trivial applications where standard library channels and goroutines suffice
- Distributed systems requiring coordination across servers (goflow operates within a single process)
- Real-time systems with microsecond latency requirements
- Applications already using mature frameworks like Temporal or Cadence for workflow orchestration

## Conclusion

Building reliable concurrent systems requires handling cancellation, shutdown, backpressure, and error recovery correctly. goflow provides production-tested primitives that solve these challenges, enabling developers to focus on business logic rather than concurrency mechanics.

The library composes naturally: rate limiters protect resources, worker pools process tasks, pipelines coordinate stages, and streams transform data. Each primitive handles edge cases correctly, preventing the subtle bugs that plague hand-rolled concurrency code.

For Go applications that need concurrent request processing, background task execution, or data pipeline coordination, goflow provides the building blocks for robust systems.

## API Reference

See [pkg.go.dev/github.com/vnykmshr/goflow](https://pkg.go.dev/github.com/vnykmshr/goflow) for complete API documentation.

## Examples

- [Rate Limiter](https://github.com/vnykmshr/goflow/tree/main/examples/rate_limiter)
- [Worker Pool](https://github.com/vnykmshr/goflow/tree/main/examples/worker_pool)
- [Streaming](https://github.com/vnykmshr/goflow/tree/main/examples/streaming)
- [Web Service](https://github.com/vnykmshr/goflow/tree/main/examples/web-service)
