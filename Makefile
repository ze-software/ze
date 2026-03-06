.PHONY: all build ze chaos test clean fmt vet tidy generate help
.PHONY: ze-lint ze-unit-test ze-unit-test-cover ze-functional-test ze-exabgp-test ze-fuzz-test ze-fuzz-one ze-test ze-verify ze-ci
.PHONY: ze-encode-test ze-plugin-test ze-decode-test ze-parse-test ze-reload-test ze-ui-test ze-editor-test
.PHONY: ze-chaos-lint ze-chaos-unit-test ze-chaos-functional-test ze-chaos-web-test ze-chaos-test ze-chaos-verify
.PHONY: check

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

ze:
	@mkdir -p bin
	go build -o bin/ze ./cmd/ze

chaos:
	@mkdir -p bin
	go build -o bin/ze-chaos ./cmd/ze-chaos

test:
	@mkdir -p bin
	go build -o bin/ze-test ./cmd/ze-test

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

# Run ze linter (excludes chaos and research packages — research excluded due to gosec v2.23.0 panic on Go 1.26)
ze-lint:
	@echo "Running ze linter..."
	@golangci-lint run ./cmd/ze/... ./cmd/ze-test/... ./internal/... ./pkg/... ./parked/... ./test/...

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
	@failed=0; failed_names=""; \
	bin/ze-test bgp encode --all || { failed=$$((failed + 1)); failed_names="$${failed_names:+$$failed_names }encode"; }; \
	bin/ze-test bgp plugin --all || { failed=$$((failed + 1)); failed_names="$${failed_names:+$$failed_names }plugin"; }; \
	bin/ze-test bgp parse --all || { failed=$$((failed + 1)); failed_names="$${failed_names:+$$failed_names }parse"; }; \
	bin/ze-test bgp decode --all || { failed=$$((failed + 1)); failed_names="$${failed_names:+$$failed_names }decode"; }; \
	bin/ze-test bgp reload --all || { failed=$$((failed + 1)); failed_names="$${failed_names:+$$failed_names }reload"; }; \
	bin/ze-test ui --all || { failed=$$((failed + 1)); failed_names="$${failed_names:+$$failed_names }ui"; }; \
	bin/ze-test editor || { failed=$$((failed + 1)); failed_names="$${failed_names:+$$failed_names }editor"; }; \
	if [ $$failed -gt 0 ]; then \
		printf "\n════════════════════════════════════════\n"; \
		printf "\033[31mFAIL  %d suite(s) failed: %s\033[0m\n" $$failed "$$failed_names"; \
		printf "\n\033[33mTo run failed suites individually:\033[0m\n"; \
		for suite in $$failed_names; do \
			printf "  make ze-%s-test\n" "$$suite"; \
		done; \
		printf "\n"; \
		exit 1; \
	else \
		printf "\n════════════════════════════════════════\n"; \
		printf "\033[32mPASS  all 7 suites\033[0m\n\n"; \
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

ze-ui-test: bin/ze-test
	@bin/ze-test ui --all

ze-editor-test: bin/ze-test
	@bin/ze-test editor

# Run ze fuzz tests (all targets, 15s each)
# Note: multiple fuzz tests per package require individual enumeration (-fuzz=. fails with "matches more than one").
# Config package uses exact path (no ...) because sub-packages would trigger "multiple packages" error.
ze-fuzz-test:
	@echo "Running ze fuzz tests..."
	go test -fuzz=FuzzParseOrigin -fuzztime=10s -timeout=60s ./internal/component/bgp/attribute/...
	go test -fuzz=FuzzParseMED -fuzztime=10s -timeout=60s ./internal/component/bgp/attribute/...
	go test -fuzz=FuzzParseLocalPref -fuzztime=10s -timeout=60s ./internal/component/bgp/attribute/...
	go test -fuzz=FuzzParseASPath -fuzztime=10s -timeout=60s ./internal/component/bgp/attribute/...
	go test -fuzz=FuzzParseCommunity -fuzztime=10s -timeout=60s ./internal/component/bgp/attribute/...
	go test -fuzz=FuzzParseLargeCommunity -fuzztime=10s -timeout=60s ./internal/component/bgp/attribute/...
	go test -fuzz=FuzzParseExtCommunity -fuzztime=10s -timeout=60s ./internal/component/bgp/attribute/...
	go test -fuzz=FuzzRewriteASPath -fuzztime=10s -timeout=60s ./internal/component/bgp/wireu/...
	go test -fuzz=FuzzParseNLRIs -fuzztime=10s -timeout=60s ./internal/component/bgp/wireu/...
	go test -fuzz=FuzzParseAttributes -fuzztime=10s -timeout=60s ./internal/component/bgp/plugins/bgp-rib/storage/...
	go test -fuzz=FuzzHandleRoundTrip -fuzztime=10s -timeout=60s ./internal/component/bgp/attrpool/...
	go test -fuzz=FuzzInvalidHandle -fuzztime=10s -timeout=60s ./internal/component/bgp/attrpool/...
	go test -fuzz=FuzzParseHeader -fuzztime=10s -timeout=60s ./internal/component/bgp/message/...
	go test -fuzz=FuzzUnpackOpen -fuzztime=10s -timeout=60s ./internal/component/bgp/message/...
	go test -fuzz=FuzzUnpackUpdate -fuzztime=10s -timeout=60s ./internal/component/bgp/message/...
	go test -fuzz=FuzzUnpackNotification -fuzztime=10s -timeout=60s ./internal/component/bgp/message/...
	go test -fuzz=FuzzUnpackRouteRefresh -fuzztime=10s -timeout=60s ./internal/component/bgp/message/...
	go test -fuzz=FuzzChunkNLRI -fuzztime=10s -timeout=60s ./internal/component/bgp/message/...
	go test -fuzz=FuzzParseIPv4Prefixes -fuzztime=10s -timeout=60s ./internal/component/bgp/wireu/...
	go test -fuzz=FuzzParseIPv6Prefixes -fuzztime=10s -timeout=60s ./internal/component/bgp/wireu/...
	go test -fuzz=FuzzParsePrefixes -fuzztime=10s -timeout=60s ./internal/component/bgp/wireu/...
	go test -fuzz=FuzzParseRouteDistinguisher -fuzztime=10s -timeout=60s ./internal/component/bgp/nlri/...
	go test -fuzz=FuzzParseRDString -fuzztime=10s -timeout=60s ./internal/component/bgp/nlri/...
	go test -fuzz=FuzzParseLabelStack -fuzztime=10s -timeout=60s ./internal/component/bgp/nlri/...
	go test -fuzz=FuzzParseVPN$$ -fuzztime=10s -timeout=60s ./internal/component/bgp/plugins/bgp-nlri-vpn/...
	go test -fuzz=FuzzParseVPNAddPath -fuzztime=10s -timeout=60s ./internal/component/bgp/plugins/bgp-nlri-vpn/...
	go test -fuzz=FuzzParseEVPN -fuzztime=10s -timeout=60s ./internal/component/bgp/plugins/bgp-nlri-evpn/...
	go test -fuzz=FuzzParseFlowSpec$$ -fuzztime=10s -timeout=60s ./internal/component/bgp/plugins/bgp-nlri-flowspec/...
	go test -fuzz=FuzzParseFlowSpecIPv6 -fuzztime=10s -timeout=60s ./internal/component/bgp/plugins/bgp-nlri-flowspec/...
	go test -fuzz=FuzzParseFlowSpecVPN -fuzztime=10s -timeout=60s ./internal/component/bgp/plugins/bgp-nlri-flowspec/...
	go test -fuzz=FuzzParseBGPLS$$ -fuzztime=10s -timeout=60s ./internal/component/bgp/plugins/bgp-nlri-ls/...
	go test -fuzz=FuzzParseBGPLSWithRest -fuzztime=10s -timeout=60s ./internal/component/bgp/plugins/bgp-nlri-ls/...
	go test -fuzz=FuzzParseMUP -fuzztime=10s -timeout=60s ./internal/component/bgp/plugins/bgp-nlri-mup/...
	go test -fuzz=FuzzParseMVPN -fuzztime=10s -timeout=60s ./internal/component/bgp/plugins/bgp-nlri-mvpn/...
	go test -fuzz=FuzzParseRTC -fuzztime=10s -timeout=60s ./internal/component/bgp/plugins/bgp-nlri-rtc/...
	go test -fuzz=FuzzParseVPLS -fuzztime=10s -timeout=60s ./internal/component/bgp/plugins/bgp-nlri-vpls/...
	go test -fuzz=FuzzScanner -fuzztime=10s -timeout=60s ./internal/component/bgp/textparse/...
	go test -fuzz=FuzzConfigParser -fuzztime=10s -timeout=60s ./internal/component/config
	go test -fuzz=FuzzTokenizer -fuzztime=10s -timeout=60s ./internal/component/config

# Run a single fuzz target for longer (usage: make ze-fuzz-one FUZZ=FuzzParseNLRIs PKG=./internal/component/bgp/wireu/... TIME=30s)
FUZZ ?= FuzzParseNLRIs
PKG  ?= ./internal/component/bgp/wireu/...
TIME ?= 30s

ze-fuzz-one:
	go test -fuzz=$(FUZZ) -fuzztime=$(TIME) $(PKG)

# Run ExaBGP compatibility tests (Ze encoding matches ExaBGP)
# Uses uv to auto-install psutil dependency
ze-exabgp-test: bin/ze
	@echo "Running ExaBGP compatibility tests..."
	uv run --with psutil ./test/exabgp-compat/bin/functional encoding --timeout 60

# Run all ze tests (use before commits)
ze-test: ze-lint ze-chaos-lint ze-unit-test ze-functional-test ze-exabgp-test ze-chaos-test ze-fuzz-test
	@echo "All ze tests passed"

# All tests except fuzz (use during development)
ze-verify: ze-lint ze-chaos-lint ze-unit-test ze-functional-test ze-exabgp-test ze-chaos-test
	@echo "Ze verification passed"

# Ze CI check
ze-ci: ze-lint ze-unit-test build
	@echo "Ze CI check passed"

# ─── Chaos tests ─────────────────────────────────────────────────────────────

# Run chaos linter
ze-chaos-lint:
	@echo "Running chaos linter..."
	@golangci-lint run $(CHAOS_PACKAGES)

# Run chaos unit tests with race detector
ze-chaos-unit-test:
	@echo "Running chaos unit tests..."
	go test -race $(CHAOS_PACKAGES)

# Chaos functional testing: in-process BGP chaos simulation with virtual clock.
# Seed is random by default (printed for reproduction). Override:
#   make ze-chaos-functional-test CHAOS_SEED=12345 CHAOS_DURATION=60s CHAOS_PEERS=8
CHAOS_SEED     ?= 0
CHAOS_DURATION ?= 30s
CHAOS_PEERS    ?= 4
CHAOS_ROUTES   ?= 10

ze-chaos-functional-test: bin/ze-chaos
	@bin/ze-chaos --in-process --duration $(CHAOS_DURATION) \
		--peers $(CHAOS_PEERS) --routes $(CHAOS_ROUTES) \
		--seed $(CHAOS_SEED) --quiet

# Chaos web dashboard tests: HTTP endpoint checks against --in-process --web.
ze-chaos-web-test: bin/ze-test
	@bin/ze-test bgp chaos-web --all

# Run all chaos tests
ze-chaos-test: ze-chaos-unit-test ze-chaos-functional-test ze-chaos-web-test
	@echo "All chaos tests passed"

# Chaos verification
ze-chaos-verify: ze-chaos-lint ze-chaos-unit-test ze-chaos-functional-test ze-chaos-web-test
	@echo "Chaos verification passed"

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
	@echo "  build                 - Build all binaries (bin/ze, bin/ze-test, bin/ze-chaos)"
	@echo "  ze                    - Build bin/ze"
	@echo "  chaos                 - Build bin/ze-chaos"
	@echo "  test                  - Build bin/ze-test"
	@echo ""
	@echo "  Ze tests:"
	@echo "  ze-lint               - Run linter on ze packages"
	@echo "  ze-unit-test          - Run ze unit tests with race detector"
	@echo "  ze-unit-test-cover    - Run ze unit tests with coverage report"
	@echo "  ze-functional-test    - Run ze functional tests (encode, plugin, parse, decode, reload, ui, editor)"
	@echo "  ze-encode-test        - Run encode functional tests only"
	@echo "  ze-plugin-test        - Run plugin functional tests only"
	@echo "  ze-decode-test        - Run decode functional tests only"
	@echo "  ze-parse-test         - Run parse functional tests only"
	@echo "  ze-reload-test        - Run reload functional tests only"
	@echo "  ze-ui-test            - Run UI functional tests only (completion)"
	@echo "  ze-editor-test        - Run editor functional tests only"
	@echo "  ze-exabgp-test        - Run ExaBGP compatibility tests only"
	@echo "  ze-fuzz-test          - Run all fuzz tests (15s per target)"
	@echo "  ze-fuzz-one           - Run single fuzz target (FUZZ=name PKG=path TIME=30s)"
	@echo "  ze-test               - All tests: lint + unit + functional + exabgp + chaos + fuzz (before commits)"
	@echo "  ze-verify             - All tests except fuzz (development)"
	@echo "  ze-ci                 - ze-lint + ze-unit-test + build"
	@echo ""
	@echo "  Chaos tests:"
	@echo "  ze-chaos-lint            - Run linter on chaos packages"
	@echo "  ze-chaos-unit-test       - Run chaos unit tests with race detector"
	@echo "  ze-chaos-functional-test - Run in-process chaos simulation"
	@echo "  ze-chaos-web-test        - Run chaos web dashboard HTTP tests"
	@echo "  ze-chaos-test            - All chaos tests (unit + functional + web)"
	@echo "  ze-chaos-verify          - ze-chaos-lint + all chaos tests"
	@echo ""
	@echo "  Utilities:"
	@echo "  fmt                   - Format code (gofmt + goimports)"
	@echo "  vet                   - Run go vet"
	@echo "  tidy                  - Tidy go.mod dependencies"
	@echo "  clean                 - Remove build artifacts"
	@echo "  check                 - Quick check (fmt + vet)"
	@echo "  help                  - Show this help"
