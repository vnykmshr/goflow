---
title: "Migration Guide"
weight: 30
---

# Migration Guide

## Deprecation Notices

### Leaky Bucket Constructors (v1.0.3+)

The `New()` and `NewWithConfig()` functions in the `leakybucket` package are deprecated and will be removed in v2.0.0.

**Why?** These functions panic on invalid input, which violates Go best practices and makes error handling difficult.

**Migration Path:**

```go
// Old (deprecated, will panic on error)
limiter := leakybucket.New(10, 100)

// New (recommended, returns error)
limiter, err := leakybucket.NewSafe(10, 100)
if err != nil {
    log.Fatal(err)
}
```

```go
// Old (deprecated, will panic on error)
limiter := leakybucket.NewWithConfig(leakybucket.Config{
    LeakRate: 10,
    Capacity: 100,
})

// New (recommended, returns error)
limiter, err := leakybucket.NewWithConfigSafe(leakybucket.Config{
    LeakRate: 10,
    Capacity: 100,
})
if err != nil {
    log.Fatal(err)
}
```

### Timeline

- **v1.0.3**: Deprecation notices added
- **v1.x**: Both APIs supported with deprecation warnings
- **v2.0.0**: Deprecated functions removed

## Removed Packages

### pkg/common/context (Removed in v1.0.3)

The `pkg/common/context` package provided thin wrappers around standard library functions without adding value.

**Migration Path:**

```go
// Old (removed)
import "github.com/vnykmshr/goflow/pkg/common/context"
ctx, cancel := context.WithTimeoutOrCancel(parent, 5*time.Second)

// New (use standard library directly)
import "context"
ctx, cancel := context.WithTimeout(parent, 5*time.Second)
```

**Function Mappings:**

| Removed Function | Standard Library Replacement |
|-----------------|------------------------------|
| `context.WithDeadlineOrCancel()` | `context.WithDeadline()` |
| `context.WithTimeoutOrCancel()` | `context.WithTimeout()` |
| `context.IsCanceled()` | Check `ctx.Done()` channel |
| `context.IsTimedOut()` | Check `ctx.Err() == context.DeadlineExceeded` |

**Example:**

```go
// Checking if context is canceled
// Old
if context.IsCanceled(ctx) { }

// New
select {
case <-ctx.Done():
    // Context is canceled
default:
    // Context is active
}

// Checking if context timed out
// Old
if context.IsTimedOut(ctx) { }

// New
if ctx.Err() == context.DeadlineExceeded {
    // Context timed out
}
```
