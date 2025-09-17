# Changelog

All notable changes to goflow will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v1.0.2] - 2025-01-17

### Fixed
- **CI/CD Issues**: Resolved compilation failures in examples and test files
- **Build Reliability**: Fixed all deprecated API usage across the codebase
- **Code Quality**: Resolved formatting and linting issues causing CI failures

### Changed
- **CI Workflow**: Split into separate lint, test, and build jobs for better parallelization
- **Linting Configuration**: More practical rules with appropriate exclusions for tests and examples
- **Error Handling**: Enhanced error propagation in all Safe API functions

### Improved
- **Developer Experience**: Faster CI feedback with fail-fast linting
- **Example Verification**: Added build verification for example applications
- **Code Formatting**: Standardized formatting across entire codebase

## [v1.0.1] - 2025-01-16

### Changed
- **BREAKING**: Removed deprecated `New()` and `NewWithConfig()` functions from rate limiters
- Updated all internal usage to safe constructors (`NewSafe`, `NewWithConfigSafe`)
- Enhanced error handling in metrics integration
- Updated benchmark tests to use safe constructors

### Added
- Comprehensive CONTRIBUTING.md with development guidelines
- Complete CHANGELOG.md following Keep a Changelog format

### Improved
- Production safety by eliminating panic paths in constructors
- Error propagation and handling across components

## [v1.0.0] - 2025-01-16

### Added
- Production-ready rate limiting with token bucket and leaky bucket implementations
- Advanced concurrency limiters with resource protection
- Enterprise-grade task scheduling with cron support and advanced patterns
- High-performance streaming with functional operations and backpressure control
- Comprehensive Prometheus metrics integration
- Distributed rate limiting with Redis coordination
- Multi-stage pipeline processing
- Extensive benchmarking and performance optimization
- Complete documentation and examples

### Features
- **Rate Limiting**: Token bucket, leaky bucket, and distributed limiters
- **Concurrency Control**: Resource-aware concurrency limiting
- **Scheduling**: Worker pools, schedulers with cron support
- **Streaming**: Functional stream processing with backpressure
- **Observability**: Full Prometheus metrics and monitoring
- **Production Ready**: Comprehensive error handling and graceful shutdown

### Performance
- Scheduler operations: ~1,636 ns/op
- Rate limiting: Sub-microsecond decisions
- Worker pools: >100K tasks/second throughput

[v1.0.2]: https://github.com/vnykmshr/goflow/releases/tag/v1.0.2
[v1.0.1]: https://github.com/vnykmshr/goflow/releases/tag/v1.0.1
[v1.0.0]: https://github.com/vnykmshr/goflow/releases/tag/v1.0.0