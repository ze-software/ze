.PHONY: all build test lint clean fmt vet tidy generate functional functional-all functional-encode functional-plugin functional-decode functional-parse functional-reload functional-editor functional-exabgp chaos verify help

# Environment: keep build caches within CURDIR (not TMPDIR - breaks Unix socket tests)
export GOCACHE := $(CURDIR)/tmp/go-cache
export GOLANGCI_LINT_CACHE := $(CURDIR)/tmp/golangci-lint-cache

# Default target
all: lint test build

# Generate code (plugin imports, etc.)
generate:
	@go run scripts/gen-plugin-imports.go

# Build all binaries
build: generate bin/ze bin/ze-test
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
	rm -rf bin/ tmp/
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
	bin/ze-test bgp encode --all || failed=$$((failed + 1)); \
	bin/ze-test bgp plugin --all || failed=$$((failed + 1)); \
	bin/ze-test bgp parse --all || failed=$$((failed + 1)); \
	bin/ze-test bgp decode --all || failed=$$((failed + 1)); \
	bin/ze-test bgp reload --all || failed=$$((failed + 1)); \
	bin/ze-test editor || failed=$$((failed + 1)); \
	if [ $$failed -gt 0 ]; then \
		printf "\n\033[31m═══ FAIL  %d suite(s) failed\033[0m\n\n" $$failed; \
		exit 1; \
	else \
		printf "\n\033[32m═══ PASS  all suites\033[0m\n\n"; \
	fi

# Run all tests including ExaBGP compatibility (use before commits)
functional-all: functional functional-exabgp

# Run encode functional tests
functional-encode: bin/ze-test
	@bin/ze-test bgp encode --all

# Run plugin functional tests
functional-plugin: bin/ze-test
	@bin/ze-test bgp plugin --all

# Run decode functional tests
functional-decode: bin/ze-test
	@bin/ze-test bgp decode --all

# Run parse functional tests
functional-parse: bin/ze-test
	@bin/ze-test bgp parse --all

# Run reload functional tests
functional-reload: bin/ze-test
	@bin/ze-test bgp reload --all

# Run editor functional tests
functional-editor: bin/ze-test
	@bin/ze-test editor

# Run ExaBGP compatibility tests (Ze encoding matches ExaBGP)
functional-exabgp: bin/ze
	@echo "Running ExaBGP compatibility tests..."
	./test/exabgp-compat/bin/functional encoding --timeout 60

# Chaos testing: in-process BGP chaos simulation with virtual clock.
# Seed is random by default (printed for reproduction). Override:
#   make chaos CHAOS_SEED=12345 CHAOS_DURATION=60s CHAOS_PEERS=8
CHAOS_SEED     ?= 0
CHAOS_DURATION ?= 30s
CHAOS_PEERS    ?= 4
CHAOS_ROUTES   ?= 10

chaos: bin/ze-bgp-chaos
	@bin/ze-bgp-chaos --in-process --duration $(CHAOS_DURATION) \
		--peers $(CHAOS_PEERS) --routes $(CHAOS_ROUTES) \
		--seed $(CHAOS_SEED) --quiet

bin/ze-bgp-chaos: $(shell find cmd/ze-bgp-chaos internal -name '*.go' 2>/dev/null)
	@echo "Building ze-bgp-chaos..."
	@mkdir -p bin
	go build -o bin/ze-bgp-chaos ./cmd/ze-bgp-chaos

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
	@echo "  functional           - Run functional tests (encode, plugin, parse, decode, reload)"
	@echo "  functional-all       - Run all functional tests including ExaBGP compat (pre-commit)"
	@echo "  functional-encode    - Run encode functional tests only"
	@echo "  functional-plugin    - Run plugin functional tests only"
	@echo "  functional-decode    - Run decode functional tests only"
	@echo "  functional-parse     - Run parse functional tests only"
	@echo "  functional-reload    - Run reload functional tests only"
	@echo "  functional-editor    - Run editor functional tests only"
	@echo "  functional-exabgp    - Run ExaBGP compatibility tests only"
	@echo "  chaos                - Run in-process chaos test (override: CHAOS_SEED, CHAOS_DURATION, CHAOS_PEERS)"
	@echo "  lint                 - Run golangci-lint"
	@echo "  fmt                  - Format code (gofmt + goimports)"
	@echo "  vet                  - Run go vet"
	@echo "  tidy                 - Tidy go.mod dependencies"
	@echo "  clean                - Remove build artifacts"
	@echo "  check                - Quick check (fmt + vet)"
	@echo "  ci                   - Full CI check (lint + test + build)"
	@echo "  help                 - Show this help"
