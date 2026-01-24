.PHONY: all build test lint clean fmt vet tidy functional functional-encode functional-plugin functional-decode functional-parse help

# Environment: keep build caches within CURDIR (not TMPDIR - breaks Unix socket tests)
export GOCACHE := $(CURDIR)/tmp/go-cache
export GOLANGCI_LINT_CACHE := $(CURDIR)/tmp/golangci-lint-cache

# Default target
all: lint test build

# Build all binaries
build:
	@echo "Building binaries..."
	go build -o bin/ze ./cmd/ze
	go build -o bin/ze-peer ./cmd/ze-peer
	go build -o bin/ze-test ./cmd/ze-test
	go build -o bin/ze-config-reader ./cmd/ze-config-reader

# Run tests with race detector
test:
	@echo "Running tests..."
	go test -race ./...

# Run tests with coverage
test-cover:
	@echo "Running tests with coverage..."
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run linter
lint:
	@echo "Running linter..."
	@golangci-lint run

# Format code
fmt:
	@echo "Formatting code..."
	gofmt -w .
	goimports -w .

# Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	go mod tidy

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html

# Run all tests including functional tests
test-all: test functional
	@echo "All tests passed"

# Run functional tests (all types)
functional: functional-encode functional-plugin functional-parse functional-decode
	@echo "All functional tests passed"

# Run encode functional tests
functional-encode:
	@echo "Running encode functional tests..."
	go run ./cmd/ze-test bgp encode --all

# Run plugin functional tests
functional-plugin:
	@echo "Running plugin functional tests..."
	go run ./cmd/ze-test bgp plugin --all

# Run decode functional tests
functional-decode:
	@echo "Running decode functional tests..."
	go run ./cmd/ze-test bgp decode --all

# Run parse functional tests
functional-parse:
	@echo "Running parse functional tests..."
	go run ./cmd/ze-test bgp parse --all

# Quick check (fast feedback during development)
check: fmt vet
	@echo "Quick check passed"

# Full CI check
ci: lint test build
	@echo "CI check passed"

# Help
help:
	@echo "Ze BGP Makefile targets:"
	@echo ""
	@echo "  all                  - lint, test, build (default)"
	@echo "  build                - Build all binaries"
	@echo "  test                 - Run unit tests with race detector"
	@echo "  test-all             - Run unit tests + functional tests"
	@echo "  test-cover           - Run tests with coverage report"
	@echo "  functional           - Run all functional tests"
	@echo "  functional-encode    - Run encode functional tests only"
	@echo "  functional-plugin    - Run plugin functional tests only"
	@echo "  functional-decode    - Run decode functional tests only"
	@echo "  functional-parse     - Run parse functional tests only"
	@echo "  lint                 - Run golangci-lint"
	@echo "  fmt                  - Format code (gofmt + goimports)"
	@echo "  vet                  - Run go vet"
	@echo "  tidy                 - Tidy go.mod dependencies"
	@echo "  clean                - Remove build artifacts"
	@echo "  check                - Quick check (fmt + vet)"
	@echo "  ci                   - Full CI check (lint + test + build)"
	@echo "  help                 - Show this help"
