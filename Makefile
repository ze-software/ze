.PHONY: all build clean fmt vet tidy generate help
.PHONY: ze-lint ze-unit-test ze-unit-test-cover ze-functional-test ze-exabgp-test ze-fuzz-test ze-fuzz-one ze-test ze-verify ze-ci
.PHONY: ze-encode-test ze-plugin-test ze-decode-test ze-parse-test ze-reload-test ze-editor-test
.PHONY: chaos-lint chaos-unit-test chaos-functional-test chaos-web-test chaos-test chaos-verify
.PHONY: test-all check

# Environment: keep build caches within CURDIR (not TMPDIR - breaks Unix socket tests)
export GOCACHE := $(CURDIR)/tmp/go-cache
export GOLANGCI_LINT_CACHE := $(CURDIR)/tmp/golangci-lint-cache

# Packages
ZE_PACKAGES = $$(go list ./... | grep -v /cmd/ze-chaos)
CHAOS_PACKAGES = ./cmd/ze-chaos/...

# Default target
all: ze-lint ze-unit-test build

# Generate code (plugin imports, etc.)
generate:
	@go run scripts/gen-plugin-imports.go

# Build all binaries
build: generate bin/ze bin/ze-test bin/ze-chaos
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

bin/ze-chaos: $(shell find cmd/ze-chaos internal -name '*.go' 2>/dev/null)
	@echo "Building ze-chaos..."
	@mkdir -p bin
	go build -o bin/ze-chaos ./cmd/ze-chaos

# ─── Ze tests ────────────────────────────────────────────────────────────────

# Run ze linter (excludes chaos packages)
ze-lint:
	@echo "Running ze linter..."
	@golangci-lint run ./cmd/ze/... ./cmd/ze-test/... ./internal/... ./pkg/... ./parked/... ./research/... ./test/...

# Run ze unit tests with race detector (excludes chaos packages)
ze-unit-test:
	@echo "Running ze unit tests..."
	go test -race $(ZE_PACKAGES)

# Run ze unit tests with coverage
ze-unit-test-cover:
	@echo "Running ze unit tests with coverage..."
	go test -race -coverprofile=coverage.out $(ZE_PACKAGES)
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run ze functional tests (all types, continue on failure to show all results)
ze-functional-test: bin/ze bin/ze-test
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

# Run ze functional test suites individually
ze-encode-test: bin/ze-test
	@bin/ze-test bgp encode --all

ze-plugin-test: bin/ze-test
	@bin/ze-test bgp plugin --all

ze-decode-test: bin/ze-test
	@bin/ze-test bgp decode --all

ze-parse-test: bin/ze-test
	@bin/ze-test bgp parse --all

ze-reload-test: bin/ze-test
	@bin/ze-test bgp reload --all

ze-editor-test: bin/ze-test
	@bin/ze-test editor

# Run ze fuzz tests (all targets, 10s each)
ze-fuzz-test:
	@echo "Running ze fuzz tests..."
	go test -fuzz=. -fuzztime=10s ./internal/plugins/bgp/attribute/...
	go test -fuzz=. -fuzztime=10s ./internal/plugins/bgp/wireu/...
	go test -fuzz=. -fuzztime=10s ./internal/plugins/bgp-rib/storage/...
	go test -fuzz=. -fuzztime=10s ./internal/pool/...

# Run a single fuzz target for longer (usage: make ze-fuzz-one FUZZ=FuzzParseNLRIs PKG=./internal/plugins/bgp/wireu/... TIME=30s)
FUZZ ?= FuzzParseNLRIs
PKG  ?= ./internal/plugins/bgp/wireu/...
TIME ?= 30s

ze-fuzz-one:
	go test -fuzz=$(FUZZ) -fuzztime=$(TIME) $(PKG)

# Run ExaBGP compatibility tests (Ze encoding matches ExaBGP)
ze-exabgp-test: bin/ze
	@echo "Running ExaBGP compatibility tests..."
	./test/exabgp-compat/bin/functional encoding --timeout 60

# Run all ze tests
ze-test: ze-unit-test ze-functional-test ze-exabgp-test ze-fuzz-test
	@echo "All ze tests passed"

# Ze verification (use during development)
ze-verify: ze-lint ze-unit-test ze-functional-test
	@echo "Ze verification passed"

# Ze CI check
ze-ci: ze-lint ze-unit-test build
	@echo "Ze CI check passed"

# ─── Chaos tests ─────────────────────────────────────────────────────────────

# Run chaos linter
chaos-lint:
	@echo "Running chaos linter..."
	@golangci-lint run $(CHAOS_PACKAGES)

# Run chaos unit tests with race detector
chaos-unit-test:
	@echo "Running chaos unit tests..."
	go test -race $(CHAOS_PACKAGES)

# Chaos functional testing: in-process BGP chaos simulation with virtual clock.
# Seed is random by default (printed for reproduction). Override:
#   make chaos-functional-test CHAOS_SEED=12345 CHAOS_DURATION=60s CHAOS_PEERS=8
CHAOS_SEED     ?= 0
CHAOS_DURATION ?= 30s
CHAOS_PEERS    ?= 4
CHAOS_ROUTES   ?= 10

chaos-functional-test: bin/ze-chaos
	@bin/ze-chaos --in-process --duration $(CHAOS_DURATION) \
		--peers $(CHAOS_PEERS) --routes $(CHAOS_ROUTES) \
		--seed $(CHAOS_SEED) --quiet

# Chaos web dashboard tests: HTTP endpoint checks against --in-process --web.
chaos-web-test: bin/ze-test
	@bin/ze-test bgp chaos-web --all

# Run all chaos tests
chaos-test: chaos-unit-test chaos-functional-test chaos-web-test
	@echo "All chaos tests passed"

# Chaos verification
chaos-verify: chaos-lint chaos-unit-test chaos-functional-test chaos-web-test
	@echo "Chaos verification passed"

# ─── Aggregates ──────────────────────────────────────────────────────────────

# Run ALL ze tests (use before commits)
test-all: ze-lint ze-test
	@echo "All tests passed"

# ─── Utilities ───────────────────────────────────────────────────────────────

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

# Quick check (fast feedback during development)
check: fmt vet
	@echo "Quick check passed"

# ─── Help ────────────────────────────────────────────────────────────────────

help:
	@echo "Ze BGP Makefile targets:"
	@echo ""
	@echo "  all                   - ze-lint, ze-unit-test, build (default)"
	@echo "  build                 - Build all binaries"
	@echo ""
	@echo "  Ze tests:"
	@echo "  ze-lint               - Run linter on ze packages"
	@echo "  ze-unit-test          - Run ze unit tests with race detector"
	@echo "  ze-unit-test-cover    - Run ze unit tests with coverage report"
	@echo "  ze-functional-test    - Run ze functional tests (encode, plugin, parse, decode, reload, editor)"
	@echo "  ze-encode-test        - Run encode functional tests only"
	@echo "  ze-plugin-test        - Run plugin functional tests only"
	@echo "  ze-decode-test        - Run decode functional tests only"
	@echo "  ze-parse-test         - Run parse functional tests only"
	@echo "  ze-reload-test        - Run reload functional tests only"
	@echo "  ze-editor-test        - Run editor functional tests only"
	@echo "  ze-exabgp-test        - Run ExaBGP compatibility tests only"
	@echo "  ze-fuzz-test          - Run all fuzz tests (10s per package)"
	@echo "  ze-fuzz-one           - Run single fuzz target (FUZZ=name PKG=path TIME=30s)"
	@echo "  ze-test               - All ze tests (unit + functional + exabgp + fuzz)"
	@echo "  ze-verify             - ze-lint + ze-unit-test + ze-functional-test (development)"
	@echo "  ze-ci                 - ze-lint + ze-unit-test + build"
	@echo ""
	@echo "  Chaos tests:"
	@echo "  chaos-lint            - Run linter on chaos packages"
	@echo "  chaos-unit-test       - Run chaos unit tests with race detector"
	@echo "  chaos-functional-test - Run in-process chaos simulation"
	@echo "  chaos-web-test        - Run chaos web dashboard HTTP tests"
	@echo "  chaos-test            - All chaos tests (unit + functional + web)"
	@echo "  chaos-verify          - chaos-lint + chaos-unit-test + chaos-functional-test"
	@echo ""
	@echo "  Aggregates:"
	@echo "  test-all              - ze-lint + ze-test (before commits)"
	@echo ""
	@echo "  Utilities:"
	@echo "  fmt                   - Format code (gofmt + goimports)"
	@echo "  vet                   - Run go vet"
	@echo "  tidy                  - Tidy go.mod dependencies"
	@echo "  clean                 - Remove build artifacts"
	@echo "  check                 - Quick check (fmt + vet)"
	@echo "  help                  - Show this help"
