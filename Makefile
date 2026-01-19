.PHONY: all build test lint clean fmt vet tidy functional functional-encoding functional-plugin functional-decoding functional-parsing help

# Environment: keep build caches within CURDIR (not TMPDIR - breaks Unix socket tests)
export GOCACHE := $(CURDIR)/tmp/go-cache
export GOLANGCI_LINT_CACHE := $(CURDIR)/tmp/golangci-lint-cache

# Default target
all: lint test build

# Build all binaries
build:
	@echo "Building binaries..."
	go build -o bin/zebgp ./cmd/zebgp
	go build -o bin/zebgp-peer ./cmd/zebgp-peer
	go build -o bin/zebgp-test ./cmd/zebgp-test

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
functional: functional-encoding functional-plugin functional-parsing functional-decoding
	@echo "All functional tests passed"

# Run encoding functional tests
functional-encoding:
	@echo "Running encoding functional tests..."
	go run ./cmd/zebgp-test run encoding --all

# Run plugin functional tests
functional-plugin:
	@echo "Running plugin functional tests..."
	go run ./cmd/zebgp-test run plugin --all

# Run decoding functional tests (may fail - JSON format alignment WIP)
functional-decoding:
	@echo "Running decoding functional tests..."
	go run ./cmd/zebgp-test run decoding --all

# Run parsing functional tests
functional-parsing:
	@echo "Running parsing functional tests..."
	go run ./cmd/zebgp-test run parsing --all

# Quick check (fast feedback during development)
check: fmt vet
	@echo "Quick check passed"

# Full CI check
ci: lint test build
	@echo "CI check passed"

# Help
help:
	@echo "ZeBGP Makefile targets:"
	@echo ""
	@echo "  all                  - lint, test, build (default)"
	@echo "  build                - Build all binaries"
	@echo "  test                 - Run unit tests with race detector"
	@echo "  test-all             - Run unit tests + functional tests"
	@echo "  test-cover           - Run tests with coverage report"
	@echo "  functional           - Run all functional tests"
	@echo "  functional-encoding  - Run encoding functional tests only"
	@echo "  functional-plugin    - Run plugin functional tests only"
	@echo "  functional-decoding  - Run decoding functional tests only"
	@echo "  functional-parsing   - Run parsing functional tests only"
	@echo "  lint                 - Run golangci-lint"
	@echo "  fmt                  - Format code (gofmt + goimports)"
	@echo "  vet                  - Run go vet"
	@echo "  tidy                 - Tidy go.mod dependencies"
	@echo "  clean                - Remove build artifacts"
	@echo "  check                - Quick check (fmt + vet)"
	@echo "  ci                   - Full CI check (lint + test + build)"
	@echo "  help                 - Show this help"
