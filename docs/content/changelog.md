---
title: "Changelog"
weight: 100
---

# Changelog

## [Unreleased]

## [v1.4.0] - 2026-01-19

### Added
- Comprehensive benchmark suite (`make benchmark`)
  - Stream benchmarks (FromSlice, Filter, Map, ToSlice, Reduce, Distinct)
  - Channel benchmarks (Send/Receive, contention, backpressure strategies)
  - WorkerPool benchmarks (submit/execute cycle, throughput, scaling)
- Performance baseline documentation in release tracker

### Changed
- Pipeline stats use atomic operations for counters, reducing lock contention
- Benchmark label formatting uses `strconv.Itoa` for all integer values

## [v1.3.0] - 2026-01-19

### Fixed
- **Critical**: Goroutine leak in worker pool - workers now properly exit on shutdown
- **Critical**: Context propagation in pipeline worker pool execution
- Race condition in pipeline stats collection
- Error handling in workerpool task submission

### Changed
- Worker pool uses `SubmitWithContext` for proper context propagation
- Improved error wrapping consistency across packages
- Enhanced goleak integration for goroutine leak detection in tests

### Added
- Comprehensive goroutine leak tests using `go.uber.org/goleak`

## [v1.2.0] - 2025-10-26

### Added
- **Test Infrastructure**: Comprehensive test suite overhaul with shared utilities
  - New `internal/testutil/mocks.go` with thread-safe MockClock and MockWriter
  - Async testing helpers: `Eventually()`, `EventuallyWithContext()`, `WaitForInt32/64()`
  - `CallbackTracker` for testing callback functionality
- **Test Coverage**: Achieved 100% coverage for critical utility packages
  - `pkg/common/errors`: 332 lines of tests (0% to 100%)
  - `pkg/common/validation`: 307 lines of tests (0% to 100%)
- **Integration Tests**: New testing framework with 10 comprehensive tests
  - Pipeline integration with rate limiting and worker pools
  - Streaming workflows (Channel + Writer + Stream processing)
  - Channel backpressure strategies (Block, Drop, DropOldest)
  - Concurrent access and context cancellation scenarios
- Common validation helpers in `pkg/common/validation` to reduce boilerplate
- Migration guide in `docs/MIGRATION.md`

### Deprecated
- `leakybucket.New()` and `leakybucket.NewWithConfig()` - use `NewSafe()` variants instead (will be removed in v2.0.0)

### Removed
- **Breaking**: `pkg/ratelimit/distributed` package (~1,800 LOC, 0% coverage)
  - Removed untested Redis-based distributed rate limiting
  - Removed Redis dependency from `go.mod`
  - Users should use specialized libraries: `github.com/go-redis/redis_rate` or `github.com/sethvargo/go-limiter`
- `pkg/common/context` package - use standard library `context` package directly

### Fixed
- **Critical**: Channel context timeout deadlock in `pkg/streaming/channel`
  - `blockingSend()` and `blockingReceive()` now properly respect context timeouts
  - Added goroutine to monitor context cancellation and wake up waiters
  - Resolves 600s timeout hangs in CI
- **Critical**: Data race in `internal/testutil` tests detected by race detector
  - Replaced plain bool with `atomic.Int32` in async test helpers
- Flaky integration test: `TestPipelineWithWorkerPool` race condition
  - Now uses `sync.WaitGroup` to ensure goroutine completion before assertions
- Flaky example test: `Example_peek` non-deterministic output ordering
- Pre-commit hook: removed obsolete `--fast` flag from golangci-lint
- GETTING_STARTED.md: corrected `workerpool.Shutdown()` usage example

### Changed
- **Code Quality**: Deduplicated ~183 lines of test code
  - Consolidated 3 duplicate MockClock implementations
  - Consolidated MockWriter implementation
  - Replaced custom `contains()` with `strings.Contains()`
- **Test Reliability**: Eliminated flaky timing tests in scheduler package
  - Replaced `time.Sleep()` with `Eventually()` pattern
  - Faster test execution with early exit
- Rate limiters now use shared validation helpers for consistent error messages
- All validation logic consolidated for better maintainability
- CI workflow: removed distributed-rate-limiting example build step

## [v1.0.2] - 2025-01-17

### Fixed
- CI/CD compilation failures in examples and tests
- Deprecated API usage across codebase
- Formatting and linting issues

### Changed
- Split CI workflow into separate lint, test, and build jobs
- Enhanced error handling in Safe API functions

## [v1.0.1] - 2025-01-16

### Changed
- **BREAKING**: Removed deprecated `New()` functions from rate limiters
- All components now use safe constructors (`NewSafe`, `NewWithConfigSafe`)

### Added
- CONTRIBUTING.md development guidelines
- Complete CHANGELOG.md

## [v1.0.0] - 2025-01-16

Initial release with production-ready components:

- Rate limiting (token bucket, leaky bucket)
- Task scheduling (worker pools, cron scheduler)
- Streaming (functional operations, backpressure)
- Prometheus metrics integration
- Comprehensive documentation and examples

[Unreleased]: https://github.com/vnykmshr/goflow/compare/v1.4.0...HEAD
[v1.4.0]: https://github.com/vnykmshr/goflow/releases/tag/v1.4.0
[v1.3.0]: https://github.com/vnykmshr/goflow/releases/tag/v1.3.0
[v1.2.0]: https://github.com/vnykmshr/goflow/releases/tag/v1.2.0
[v1.0.2]: https://github.com/vnykmshr/goflow/releases/tag/v1.0.2
[v1.0.1]: https://github.com/vnykmshr/goflow/releases/tag/v1.0.1
[v1.0.0]: https://github.com/vnykmshr/goflow/releases/tag/v1.0.0
