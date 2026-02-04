.PHONY: all build test lint clean fmt vet tidy functional functional-all functional-encode functional-plugin functional-decode functional-parse functional-editor functional-exabgp verify help

# Environment: keep build caches within CURDIR (not TMPDIR - breaks Unix socket tests)
export GOCACHE := $(CURDIR)/tmp/go-cache
export GOLANGCI_LINT_CACHE := $(CURDIR)/tmp/golangci-lint-cache

# Default target
all: lint test build

# Build all binaries
build: bin/ze bin/ze-test bin/ze-config-reader
	@echo "All binaries built"

# Individual binary targets
bin/ze: $(shell find cmd/ze internal -name '*.go' 2>/dev/null)
	@echo "Building ze..."
	@mkdir -p bin
	go build -o bin/ze ./cmd/ze

bin/ze-test: $(shell find cmd/ze-test internal -name '*.go' 2>/dev/null)
	@echo "Building ze-test..."
	@mkdir -p bin
	go build -o bin/ze-test ./cmd/ze-test

bin/ze-config-reader: $(shell find cmd/ze-config-reader internal -name '*.go' 2>/dev/null)
	@echo "Building ze-config-reader..."
	@mkdir -p bin
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

# Run verification (use during development)
verify: lint test functional
	@echo "Verification passed"

# Run ALL tests including ExaBGP compat (use before commits)
test-all: lint test functional-all
	@echo "All tests passed"

# Run functional tests (all types, continue on failure to show all results)
functional: bin/ze bin/ze-test
	@failed=0; \
	echo "Running encode functional tests..."; \
	bin/ze-test bgp encode --all || failed=$$((failed + 1)); \
	echo ""; \
	echo "Running plugin functional tests..."; \
	bin/ze-test bgp plugin --all || failed=$$((failed + 1)); \
	echo ""; \
	echo "Running parse functional tests..."; \
	bin/ze-test bgp parse --all || failed=$$((failed + 1)); \
	echo ""; \
	echo "Running decode functional tests..."; \
	bin/ze-test bgp decode --all || failed=$$((failed + 1)); \
	echo ""; \
	echo "Running editor functional tests..."; \
	bin/ze-test editor || failed=$$((failed + 1)); \
	echo ""; \
	if [ $$failed -gt 0 ]; then \
		echo "═══════════════════════════════════════════════════════════════════════════════"; \
		echo "FUNCTIONAL TESTS: $$failed test suite(s) failed"; \
		echo "═══════════════════════════════════════════════════════════════════════════════"; \
		exit 1; \
	else \
		echo "═══════════════════════════════════════════════════════════════════════════════"; \
		echo "All functional tests passed"; \
		echo "═══════════════════════════════════════════════════════════════════════════════"; \
	fi

# Run all tests including ExaBGP compatibility (use before commits)
functional-all: functional functional-exabgp

# Run encode functional tests
functional-encode: bin/ze-test
	@echo "Running encode functional tests..."
	bin/ze-test bgp encode --all

# Run plugin functional tests
functional-plugin: bin/ze-test
	@echo "Running plugin functional tests..."
	bin/ze-test bgp plugin --all

# Run decode functional tests
functional-decode: bin/ze-test
	@echo "Running decode functional tests..."
	bin/ze-test bgp decode --all

# Run parse functional tests
functional-parse: bin/ze-test
	@echo "Running parse functional tests..."
	bin/ze-test bgp parse --all

# Run editor functional tests
functional-editor: bin/ze-test
	@echo "Running editor functional tests..."
	bin/ze-test editor

# Run ExaBGP compatibility tests (Ze encoding matches ExaBGP)
functional-exabgp:
	@echo "Running ExaBGP compatibility tests..."
	./test/exabgp-compat/bin/functional encoding --timeout 60

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
	@echo "  verify               - Quick verification: lint + test + functional (development)"
	@echo "  test                 - Run unit tests with race detector"
	@echo "  test-all             - Full verification: lint + test + functional-all (before commits)"
	@echo "  test-cover           - Run tests with coverage report"
	@echo "  functional           - Run functional tests (encode, plugin, parse, decode)"
	@echo "  functional-all       - Run all functional tests including ExaBGP compat (pre-commit)"
	@echo "  functional-encode    - Run encode functional tests only"
	@echo "  functional-plugin    - Run plugin functional tests only"
	@echo "  functional-decode    - Run decode functional tests only"
	@echo "  functional-parse     - Run parse functional tests only"
	@echo "  functional-editor    - Run editor functional tests only"
	@echo "  functional-exabgp    - Run ExaBGP compatibility tests only"
	@echo "  lint                 - Run golangci-lint"
	@echo "  fmt                  - Format code (gofmt + goimports)"
	@echo "  vet                  - Run go vet"
	@echo "  tidy                 - Tidy go.mod dependencies"
	@echo "  clean                - Remove build artifacts"
	@echo "  check                - Quick check (fmt + vet)"
	@echo "  ci                   - Full CI check (lint + test + build)"
	@echo "  help                 - Show this help"
