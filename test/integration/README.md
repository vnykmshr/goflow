# Integration Tests

This directory contains integration tests that verify cross-package functionality and real-world scenarios.

## Purpose

While unit tests verify individual components in isolation, integration tests ensure that different components work together correctly. These tests:

- Test **real interactions** between multiple packages
- Use **actual timing** (not mocked time) to verify behavior
- Validate **error propagation** across component boundaries
- Ensure **concurrent access** works correctly in realistic scenarios
- Test **resource cleanup** and proper shutdown sequences

## Test Organization

Integration tests are organized by the primary interaction being tested:

- `pipeline_ratelimit_test.go` - Pipeline + Rate Limiting + Worker Pool interactions

## Running Integration Tests

```bash
# Run all integration tests
go test ./test/integration/...

# Run with verbose output
go test ./test/integration/... -v

# Run specific test
go test ./test/integration -run TestPipelineWithRateLimiting
```

## Writing Integration Tests

Integration tests should:

1. **Test Real Scenarios**: Use realistic workflows that users would encounter
2. **Avoid Mocks**: Use real implementations, not mocks (unless testing failure scenarios)
3. **Use Timeouts**: Always set reasonable timeouts to prevent hanging tests
4. **Be Thorough**: Test both success and failure paths
5. **Document Intent**: Include comments explaining what's being tested and why

### Example

```go
func TestPipelineWithRateLimiting(t *testing.T) {
    // Setup: Create real instances
    limiter, _ := bucket.NewSafe(10, 5)
    p := pipeline.New()

    // Test: Execute realistic workflow
    p.AddStageFunc("rate-limited", func(ctx context.Context, input interface{}) (interface{}, error) {
        limiter.Wait(ctx) // Real rate limiting
        return processInput(input), nil
    })

    // Verify: Check actual behavior
    result, err := p.Execute(context.Background(), testData)
    assert.NoError(t, err)
    assert.Equal(t, expected, result.Output)
}
```

## Best Practices

### Use Helper Utilities

Leverage `internal/testutil` helpers for cleaner tests:

```go
// Instead of time.Sleep() and manual checking
testutil.Eventually(t, func() bool {
    return atomic.LoadInt32(&counter) == expected
}, timeout, interval)

// Instead of manual callback tracking
tracker := testutil.NewCallbackTracker()
tracker.AssertCalled(t)
tracker.AssertCallCount(t, 3)
```

### Test Concurrency

When testing concurrent scenarios:

```go
const goroutines = 10
done := make(chan bool, goroutines)

for i := 0; i < goroutines; i++ {
    go func() {
        defer func() { done <- true }()
        // Test concurrent behavior
    }()
}

for i := 0; i < goroutines; i++ {
    <-done
}
```

### Proper Cleanup

Always clean up resources:

```go
func TestWithResources(t *testing.T) {
    pool := workerpool.New(5, 10)
    defer func() { <-pool.Shutdown() }()

    limiter, _ := bucket.NewSafe(10, 5)
    // limiter doesn't need cleanup

    // Run tests...
}
```

## Coverage Goals

Integration tests should focus on:

- ✅ **Cross-package workflows** (not covered by unit tests)
- ✅ **Real timing and concurrency** (not mocked)
- ✅ **Error propagation** between components
- ✅ **Resource lifecycle** (creation, usage, cleanup)
- ❌ **Not**: Duplicating unit test coverage
- ❌ **Not**: Testing internal implementation details

## Performance Considerations

Integration tests may take longer than unit tests:

- **Rate limiting tests**: Need real time delays
- **Concurrent tests**: Need goroutine scheduling
- **Pipeline tests**: Multiple stages with actual execution

Keep tests as fast as possible while still testing realistic behavior:
- Use shorter timeouts where appropriate
- Use smaller data sets
- Run concurrent operations in parallel
