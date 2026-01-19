---
title: "Streaming"
weight: 3
---

# Streaming

goflow provides functional stream processing and channel utilities for data pipelines.

## Stream Processing

Streams enable functional-style operations on collections.

```go
import (
    "context"
    "github.com/vnykmshr/goflow/pkg/streaming/stream"
)

result, _ := stream.FromSlice([]int{1, 2, 3, 4, 5}).
    Filter(func(x int) bool { return x > 2 }).
    Map(func(x int) int { return x * 2 }).
    ToSlice(context.Background()) // [6, 8, 10]
```

### Available Operations

**Transformations**

| Operation | Description |
|-----------|-------------|
| `Map` | Transform each element |
| `Filter` | Keep elements matching predicate |
| `FlatMap` | Transform and flatten |
| `Distinct` | Remove duplicates |
| `Limit` | Keep first N elements |
| `Skip` | Skip first N elements |

**Terminal Operations**

| Operation | Description |
|-----------|-------------|
| `ToSlice` | Gather results into slice |
| `Reduce` | Combine elements into single value |
| `ForEach` | Apply function to each element |
| `Count` | Count elements |
| `FindFirst` | Get first element |
| `AnyMatch` | Check if any match predicate |
| `AllMatch` | Check if all match predicate |

### Examples

```go
ctx := context.Background()

// Sum of squares of even numbers
sum, _ := stream.FromSlice([]int{1, 2, 3, 4, 5}).
    Filter(func(x int) bool { return x%2 == 0 }).
    Map(func(x int) int { return x * x }).
    Reduce(ctx, 0, func(a, b int) int { return a + b }) // 20

// Find first matching element
first, ok, _ := stream.FromSlice(users).
    Filter(func(u User) bool { return u.Active }).
    FindFirst(ctx)

// Check if any element matches
hasAdmin, _ := stream.FromSlice(users).
    AnyMatch(ctx, func(u User) bool { return u.Role == "admin" })
```

## Channel Operations

The channel package provides utilities for channel-based communication with backpressure support.

```go
import "github.com/vnykmshr/goflow/pkg/streaming/channel"

ch, err := channel.NewSafe(channel.Config{
    Capacity:          100,
    BackpressureMode:  channel.Block,
})
if err != nil {
    log.Fatal(err)
}

// Send with context
ctx, cancel := context.WithTimeout(context.Background(), time.Second)
defer cancel()
err = ch.Send(ctx, value)

// Receive
value, err := ch.Receive(ctx)
```

### Backpressure Strategies

| Strategy | Behavior |
|----------|----------|
| `Block` | Block sender until space available |
| `Drop` | Drop new items when full |
| `DropOldest` | Drop oldest items to make room |

### When to Use

- Producer-consumer patterns
- Bounded buffering between goroutines
- Backpressure handling

## Async Writer

The writer package provides buffered, async writing.

```go
import "github.com/vnykmshr/goflow/pkg/streaming/writer"

w, err := writer.NewSafe(writer.Config{
    BufferSize:    1000,
    FlushInterval: time.Second,
    Output:        outputWriter,
})
if err != nil {
    log.Fatal(err)
}
defer w.Close()

w.Write(data)
```

### When to Use

- High-throughput logging
- Batched database writes
- Network buffering

## Combining Components

Stream processing works with channels for concurrent pipelines:

```go
// Producer
go func() {
    for item := range input {
        ch.Send(ctx, item)
    }
    ch.Close()
}()

// Consumer with stream processing
for {
    batch, err := ch.ReceiveBatch(ctx, 100)
    if err != nil {
        break
    }

    results, _ := stream.FromSlice(batch).
        Filter(validate).
        Map(transform).
        ToSlice(ctx)

    processBatch(results)
}
```

## Context Support

All streaming operations respect context cancellation:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

err := ch.Send(ctx, value)
if err == context.DeadlineExceeded {
    // Handle timeout
}
```

## API Reference

See [pkg.go.dev/github.com/vnykmshr/goflow/pkg/streaming](https://pkg.go.dev/github.com/vnykmshr/goflow/pkg/streaming) for complete API documentation.

## Examples

- [Streaming Example](https://github.com/vnykmshr/goflow/tree/main/examples/streaming)
