# Changelog

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

- Rate limiting (token bucket, leaky bucket, distributed)
- Task scheduling (worker pools, cron scheduler)
- Streaming (functional operations, backpressure)
- Prometheus metrics integration
- Comprehensive documentation and examples

[v1.0.2]: https://github.com/vnykmshr/goflow/releases/tag/v1.0.2
[v1.0.1]: https://github.com/vnykmshr/goflow/releases/tag/v1.0.1
[v1.0.0]: https://github.com/vnykmshr/goflow/releases/tag/v1.0.0