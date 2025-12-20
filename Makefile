.PHONY: all build test lint clean fmt vet tidy self-check help

# Default target
all: lint test build

# Build all binaries
build:
	@echo "Building binaries..."
	go build -o bin/zebgp ./cmd/zebgp
	go build -o bin/zebgp-cli ./cmd/zebgp-cli
	go build -o bin/zebgp-decode ./cmd/zebgp-decode

# Run tests with race detector
test:
	@echo "Running tests..."
	go test -race -v ./...

# Run tests with coverage
test-cover:
	@echo "Running tests with coverage..."
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run

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

# Run all tests including self-check
test-all: test self-check
	@echo "All tests passed"

# Run self-check functional tests
self-check:
	@echo "Running self-check tests..."
	go run ./cmd/self-check --all

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
	@echo "  all          - lint, test, build (default)"
	@echo "  build        - Build all binaries"
	@echo "  test         - Run unit tests with race detector"
	@echo "  test-all     - Run unit tests + self-check functional tests"
	@echo "  test-cover   - Run tests with coverage report"
	@echo "  self-check   - Run functional tests (ExaBGP compatibility)"
	@echo "  lint         - Run golangci-lint"
	@echo "  fmt          - Format code (gofmt + goimports)"
	@echo "  vet          - Run go vet"
	@echo "  tidy         - Tidy go.mod dependencies"
	@echo "  clean        - Remove build artifacts"
	@echo "  check        - Quick check (fmt + vet)"
	@echo "  ci           - Full CI check (lint + test + build)"
	@echo "  help         - Show this help"
