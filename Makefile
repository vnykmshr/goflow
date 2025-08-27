.PHONY: build test lint clean install-tools fmt bench deps help

# Variables
BINARY_NAME=goflow
GO_VERSION=$(shell go version | cut -d ' ' -f 3)

# Default target
all: lint test build

# Show help
help:
	@echo "Available targets:"
	@echo "  build         - Build the project"
	@echo "  test          - Run tests with coverage"
	@echo "  lint          - Run linter"
	@echo "  fmt           - Format code"
	@echo "  clean         - Clean build artifacts"
	@echo "  install-tools - Install development tools"
	@echo "  bench         - Run benchmarks"
	@echo "  deps          - Update dependencies"

# Install development tools
install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest

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

# Run benchmarks
bench:
	go test -bench=. -benchmem ./...

# Update dependencies
deps:
	go mod tidy
	go mod verify

# Check Go version
version:
	@echo "Go version: $(GO_VERSION)"