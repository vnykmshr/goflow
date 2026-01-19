.PHONY: build test lint clean install-tools fmt bench deps help check vet install-hooks

# Variables
BINARY_NAME=goflow
GO_VERSION=$(shell go version | cut -d ' ' -f 3)

# Default target
all: lint test build

# Run all checks (fmt, vet, lint, test, build)
check: fmt vet lint test build

# Run go vet
vet:
	go vet ./...

# Show help
help:
	@echo "Available targets:"
	@echo "  check         - Run all checks (fmt, vet, lint, test, build)"
	@echo "  build         - Build the project"
	@echo "  test          - Run tests with coverage"
	@echo "  lint          - Run linter"
	@echo "  fmt           - Format code"
	@echo "  vet           - Run go vet"
	@echo "  clean         - Clean build artifacts"
	@echo "  install-tools - Install development tools"
	@echo "  install-hooks - Install git pre-commit hook"
	@echo "  bench         - Run benchmarks"
	@echo "  deps          - Update dependencies"

# Install development tools
install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest

# Install git pre-commit hook
install-hooks:
	@echo "Installing pre-commit hook..."
	@mkdir -p scripts
	@cp scripts/pre-commit .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "✓ Pre-commit hook installed successfully"
	@echo "  The hook will run automatically on 'git commit'"
	@echo "  It checks for secrets, formats code, runs linter, and verifies build"

# Build the project
build:
	go build -v ./...

# Run tests
test:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Run linter
lint:
	golangci-lint run

# Clean build artifacts
clean:
	go clean
	rm -f coverage.out coverage.html

# Format code
fmt:
	goimports -w .
	gofmt -s -w .

# Run benchmarks (comprehensive suite)
benchmark:
	@echo "Running comprehensive benchmark suite..."
	@echo ""
	@echo "=== Stream Benchmarks ==="
	go test -bench=. -benchmem -run=^$$ ./internal/benchmark/... -v 2>/dev/null | grep -E "^(Benchmark|ok|---)"
	@echo ""
	@echo "=== Pipeline Benchmarks ==="
	go test -bench=. -benchmem -run=^$$ ./pkg/scheduling/pipeline/... -v 2>/dev/null | grep -E "^(Benchmark|ok|---)"
	@echo ""
	@echo "Benchmark complete. Use 'make bench-quick' for faster summary."

# Quick benchmark (less verbose)
bench:
	go test -bench=. -benchmem -run=^$$ ./...

# Update dependencies
deps:
	go mod tidy
	go mod verify

# Check Go version
version:
	@echo "Go version: $(GO_VERSION)"