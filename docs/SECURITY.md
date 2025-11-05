# Security Policy

## Supported Versions

We release patches for security vulnerabilities in the following versions:

| Version | Supported |
| ------- | --------- |
| 1.0.x   | Yes       |
| < 1.0   | No        |

## Reporting a Vulnerability

We take the security of goflow seriously. If you believe you have found a security vulnerability, please report it to us as described below.

### How to Report a Security Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, please report them via one of the following methods:

1. **Preferred**: Use GitHub's [Security Advisories](https://github.com/vnykmshr/goflow/security/advisories/new) feature
2. **Alternative**: Email the maintainers directly (check the repository for contact information)

### What to Include in Your Report

Please include as much of the following information as possible:

- Type of vulnerability (e.g., buffer overflow, SQL injection, cross-site scripting, etc.)
- Full paths of source file(s) related to the vulnerability
- Location of the affected source code (tag/branch/commit or direct URL)
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if possible)
- Impact of the vulnerability, including how an attacker might exploit it

### What to Expect

- **Acknowledgment**: We will acknowledge receipt of your vulnerability report within 48 hours
- **Assessment**: We will assess the vulnerability and determine its impact and severity
- **Fix Development**: We will work on a fix and keep you informed of our progress
- **Disclosure**: Once a fix is available, we will:
  - Release a security advisory
  - Credit you for the discovery (unless you prefer to remain anonymous)
  - Release a patched version

### Security Update Policy

- **Critical vulnerabilities**: Patches released within 7 days
- **High severity**: Patches released within 14 days
- **Medium/Low severity**: Patches included in the next regular release

## Security Best Practices

When using goflow in production:

### Rate Limiting

- Use appropriate rate limits based on your infrastructure capacity
- Monitor rate limiter behavior to detect unusual patterns
- Consider the burst parameter carefully - too high allows traffic spikes

### Task Scheduling
- Set reasonable timeouts for tasks to prevent resource exhaustion
- Validate all input data before processing
- Use context cancellation to handle timeouts gracefully
- Monitor worker pool metrics

### Streaming
- Set appropriate buffer sizes to prevent memory exhaustion
- Implement backpressure mechanisms for high-volume streams
- Use context timeouts for long-running operations
- Handle errors appropriately to prevent resource leaks

### General
- Keep goflow and all dependencies up to date
- Use Go modules for dependency management
- Run security scans regularly (we use gosec in CI)
- Follow the principle of least privilege for service accounts

## Known Security Considerations

### Concurrency
- This library uses goroutines extensively. Ensure your application doesn't exceed system limits
- Always use context cancellation for long-running operations
- Properly close streams and shutdown worker pools to prevent goroutine leaks

### Resource Management
- Rate limiters and worker pools consume memory based on configuration
- Monitor memory usage in production environments
- Use appropriate queue sizes to prevent unbounded memory growth

### Dependencies
We regularly scan dependencies for vulnerabilities using:
- GitHub Dependency Review
- Gosec security scanner
- Weekly automated security scans

See our [CI workflows](.github/workflows/security.yml) for details.

## Security Scanning

This project includes automated security scanning:

- **Weekly Security Scans**: Runs every Monday at 6 AM UTC
- **PR Dependency Review**: Blocks PRs with moderate+ vulnerabilities
- **License Compliance**: Rejects GPL-2.0 and GPL-3.0 licenses

## Contact

For general security questions or concerns, please open a discussion in the repository.

For reporting vulnerabilities, please use the methods described above.

---

**Last Updated**: 2025-01-26
