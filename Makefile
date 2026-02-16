.PHONY: all build unit-test lint clean fmt vet tidy generate functional-test encode-test plugin-test decode-test parse-test reload-test editor-test exabgp-test fuzz-test chaos-test verify help

# Environment: keep build caches within CURDIR (not TMPDIR - breaks Unix socket tests)
export GOCACHE := $(CURDIR)/tmp/go-cache
export GOLANGCI_LINT_CACHE := $(CURDIR)/tmp/golangci-lint-cache

# Default target
all: lint unit-test build

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

# Run unit tests with race detector
unit-test:
	@echo "Running unit tests..."
	go test -race ./...

# Run unit tests with coverage
unit-test-cover:
	@echo "Running unit tests with coverage..."
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
verify: lint unit-test functional-test
	@echo "Verification passed"

# Run ALL tests including ExaBGP compat (use before commits)
test-all: lint unit-test functional-test exabgp-test
	@echo "All tests passed"

# Run functional tests (all types, continue on failure to show all results)
functional-test: bin/ze bin/ze-test
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

# Run encode functional tests
encode-test: bin/ze-test
	@bin/ze-test bgp encode --all

# Run plugin functional tests
plugin-test: bin/ze-test
	@bin/ze-test bgp plugin --all

# Run decode functional tests
decode-test: bin/ze-test
	@bin/ze-test bgp decode --all

# Run parse functional tests
parse-test: bin/ze-test
	@bin/ze-test bgp parse --all

# Run reload functional tests
reload-test: bin/ze-test
	@bin/ze-test bgp reload --all

# Run editor functional tests
editor-test: bin/ze-test
	@bin/ze-test editor

# Run fuzz tests
fuzz-test:
	@echo "Running fuzz tests..."
	go test -fuzz=FuzzParseNLRI -fuzztime=30s ./internal/bgp/nlri/...
	go test -fuzz=. -fuzztime=10s ./internal/bgp/...

# Run ExaBGP compatibility tests (Ze encoding matches ExaBGP)
exabgp-test: bin/ze
	@echo "Running ExaBGP compatibility tests..."
	./test/exabgp-compat/bin/functional encoding --timeout 60

# Chaos testing: in-process BGP chaos simulation with virtual clock.
# Seed is random by default (printed for reproduction). Override:
#   make chaos-test CHAOS_SEED=12345 CHAOS_DURATION=60s CHAOS_PEERS=8
CHAOS_SEED     ?= 0
CHAOS_DURATION ?= 30s
CHAOS_PEERS    ?= 4
CHAOS_ROUTES   ?= 10

chaos-test: bin/ze-bgp-chaos
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
ci: lint unit-test build
	@echo "CI check passed"

# Help
help:
	@echo "Ze BGP Makefile targets:"
	@echo ""
	@echo "  all                  - lint, unit-test, build (default)"
	@echo "  build                - Build all binaries"
	@echo "  verify               - Quick verification: lint + unit-test + functional-test (development)"
	@echo "  unit-test            - Run unit tests with race detector"
	@echo "  test-all             - Full verification: lint + unit-test + functional-test + exabgp-test (before commits)"
	@echo "  unit-test-cover      - Run unit tests with coverage report"
	@echo "  functional-test      - Run functional tests (encode, plugin, parse, decode, reload, editor)"
	@echo "  encode-test          - Run encode functional tests only"
	@echo "  plugin-test          - Run plugin functional tests only"
	@echo "  decode-test          - Run decode functional tests only"
	@echo "  parse-test           - Run parse functional tests only"
	@echo "  reload-test          - Run reload functional tests only"
	@echo "  editor-test          - Run editor functional tests only"
	@echo "  exabgp-test          - Run ExaBGP compatibility tests only"
	@echo "  fuzz-test            - Run fuzz tests (NLRI 30s + all 10s)"
	@echo "  chaos-test           - Run in-process chaos test (override: CHAOS_SEED, CHAOS_DURATION, CHAOS_PEERS)"
	@echo "  lint                 - Run golangci-lint"
	@echo "  fmt                  - Format code (gofmt + goimports)"
	@echo "  vet                  - Run go vet"
	@echo "  tidy                 - Tidy go.mod dependencies"
	@echo "  clean                - Remove build artifacts"
	@echo "  check                - Quick check (fmt + vet)"
	@echo "  ci                   - Full CI check (lint + unit-test + build)"
	@echo "  help                 - Show this help"
