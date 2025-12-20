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

# Run encoding tests (ExaBGP compatibility)
test-encoding:
	@echo "Running encoding tests..."
	go test -v ./testdata/encoding/...

# Run decoding tests (ExaBGP compatibility)
test-decoding:
	@echo "Running decoding tests..."
	go test -v ./testdata/decoding/...

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
	@echo "  test         - Run tests with race detector"
	@echo "  test-cover   - Run tests with coverage report"
	@echo "  lint         - Run golangci-lint"
	@echo "  fmt          - Format code (gofmt + goimports)"
	@echo "  vet          - Run go vet"
	@echo "  tidy         - Tidy go.mod dependencies"
	@echo "  clean        - Remove build artifacts"
	@echo "  self-check   - Run functional tests (ExaBGP compatibility)"
	@echo "  check        - Quick check (fmt + vet)"
	@echo "  ci           - Full CI check (lint + test + build)"
	@echo "  help         - Show this help"
