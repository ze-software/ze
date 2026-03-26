.PHONY: all build ze chaos test analyse clean fmt vet tidy generate help
.PHONY: ze-lint ze-unit-test ze-unit-test-cover ze-functional-test ze-exabgp-test ze-fuzz-test ze-fuzz-one ze-test ze-verify ze-ci
.PHONY: ze-encode-test ze-plugin-test ze-decode-test ze-parse-test ze-reload-test ze-ui-test ze-editor-test
.PHONY: ze-chaos-lint ze-chaos-unit-test ze-chaos-functional-test ze-chaos-web-test ze-chaos-test ze-chaos-verify
.PHONY: ze-all ze-all-test
.PHONY: ze-interop-test
.PHONY: ze-perf ze-perf-bench ze-perf-report ze-perf-track
.PHONY: ze-spec-status ze-spec-status-json ze-inventory ze-inventory-json ze-validate-commands ze-validate-commands-json ze-doc-drift
.PHONY: check

# Environment: keep build caches within CURDIR (not TMPDIR - breaks Unix socket tests)
export GOCACHE := $(CURDIR)/tmp/go-cache
export GOLANGCI_LINT_CACHE := $(CURDIR)/tmp/golangci-lint-cache

# Go compiler: override with GO=tinygo for smaller binaries
# TinyGo finds go via PATH, so we prepend Go 1.25 when GO=tinygo
GO ?= go
ifeq ($(GO),tinygo)
export PATH := /opt/homebrew/opt/go@1.25/bin:$(PATH)
endif

# Version: YY.MM.DD from current date, injected via ldflags.
ZE_VERSION := $(shell date +%y.%m.%d)
ZE_BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
ZE_LDFLAGS := -X main.version=$(ZE_VERSION) -X main.buildDate=$(ZE_BUILD_DATE)

# CPU limit: use 50% of available cores for tests (minimum 1)
GO_TEST_PROCS := $(shell n=$$(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4); p=$$(( n / 2 )); [ $$p -lt 1 ] && p=1; echo $$p)
GO_TEST = GOMAXPROCS=$(GO_TEST_PROCS) go test

# Packages
ZE_PACKAGES = $$(go list ./... | grep -v /cmd/ze-chaos)
CHAOS_PACKAGES = ./cmd/ze-chaos/...

# Default target
.DEFAULT_GOAL := help

all: ze-lint ze-unit-test build

# Generate code (plugin imports, etc.)
generate:
	@go run scripts/gen-plugin-imports.go

# Build all binaries
build: generate bin/ze bin/ze-test bin/ze-chaos bin/ze-analyse docs/comparison.html
	@echo "All binaries built"

# Regenerate comparison HTML when markdown changes
docs/comparison.html: docs/comparison.md scripts/comparison-html.py
	@python3 scripts/comparison-html.py

ze:
	@mkdir -p bin
	$(GO) build -ldflags "$(ZE_LDFLAGS)" -o bin/ze ./cmd/ze

chaos:
	@mkdir -p bin
	$(GO) build -o bin/ze-chaos ./cmd/ze-chaos

test:
	@mkdir -p bin
	$(GO) build -o bin/ze-test ./cmd/ze-test

analyse:
	@mkdir -p bin
	$(GO) build -o bin/ze-analyse ./cmd/ze-analyse

# Individual binary targets
bin/ze: $(shell find cmd/ze internal -name '*.go' 2>/dev/null)
	@echo "Building ze..."
	@mkdir -p bin
	$(GO) build -ldflags "$(ZE_LDFLAGS)" -o bin/ze ./cmd/ze

bin/ze-test: $(shell find cmd/ze-test internal -name '*.go' 2>/dev/null)
	@echo "Building ze-test..."
	@mkdir -p bin
	$(GO) build -o bin/ze-test ./cmd/ze-test

bin/ze-chaos: $(shell find cmd/ze-chaos internal -name '*.go' 2>/dev/null)
	@echo "Building ze-chaos..."
	@mkdir -p bin
	$(GO) build -o bin/ze-chaos ./cmd/ze-chaos

bin/ze-analyse: $(shell find cmd/ze-analyse -name '*.go' 2>/dev/null)
	@echo "Building ze-analyse..."
	@mkdir -p bin
	$(GO) build -o bin/ze-analyse ./cmd/ze-analyse

# ─── Ze tests ────────────────────────────────────────────────────────────────

# Run ze linter (excludes chaos and research packages — research excluded due to gosec v2.23.0 panic on Go 1.26)
ze-lint:
	@echo "Running ze linter..."
	@golangci-lint run ./cmd/ze/... ./cmd/ze-test/... ./internal/... ./pkg/... ./parked/... ./test/...

# Run ze unit tests with race detector (excludes chaos packages)
ze-unit-test:
	@echo "Running ze unit tests..."
	$(GO_TEST) -race $(ZE_PACKAGES)

# Run ze unit tests with coverage
ze-unit-test-cover:
	@echo "Running ze unit tests with coverage..."
	$(GO_TEST) -race -coverprofile=coverage.out $(ZE_PACKAGES)
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
	$(GO_TEST) -fuzz=FuzzParseOrigin -fuzztime=10s -timeout=60s ./internal/component/bgp/attribute/...
	$(GO_TEST) -fuzz=FuzzParseMED -fuzztime=10s -timeout=60s ./internal/component/bgp/attribute/...
	$(GO_TEST) -fuzz=FuzzParseLocalPref -fuzztime=10s -timeout=60s ./internal/component/bgp/attribute/...
	$(GO_TEST) -fuzz=FuzzParseASPath -fuzztime=10s -timeout=60s ./internal/component/bgp/attribute/...
	$(GO_TEST) -fuzz=FuzzParseCommunity -fuzztime=10s -timeout=60s ./internal/component/bgp/attribute/...
	$(GO_TEST) -fuzz=FuzzParseLargeCommunity -fuzztime=10s -timeout=60s ./internal/component/bgp/attribute/...
	$(GO_TEST) -fuzz=FuzzParseExtCommunity -fuzztime=10s -timeout=60s ./internal/component/bgp/attribute/...
	$(GO_TEST) -fuzz=FuzzRewriteASPath -fuzztime=10s -timeout=60s ./internal/component/bgp/wireu/...
	$(GO_TEST) -fuzz=FuzzParseNLRIs -fuzztime=10s -timeout=60s ./internal/component/bgp/wireu/...
	$(GO_TEST) -fuzz=FuzzParseAttributes -fuzztime=10s -timeout=60s ./internal/component/bgp/plugins/bgp-rib/storage/...
	$(GO_TEST) -fuzz=FuzzHandleRoundTrip -fuzztime=10s -timeout=60s ./internal/component/bgp/attrpool/...
	$(GO_TEST) -fuzz=FuzzInvalidHandle -fuzztime=10s -timeout=60s ./internal/component/bgp/attrpool/...
	$(GO_TEST) -fuzz=FuzzParseHeader -fuzztime=10s -timeout=60s ./internal/component/bgp/message/...
	$(GO_TEST) -fuzz=FuzzUnpackOpen -fuzztime=10s -timeout=60s ./internal/component/bgp/message/...
	$(GO_TEST) -fuzz=FuzzUnpackUpdate -fuzztime=10s -timeout=60s ./internal/component/bgp/message/...
	$(GO_TEST) -fuzz=FuzzUnpackNotification -fuzztime=10s -timeout=60s ./internal/component/bgp/message/...
	$(GO_TEST) -fuzz=FuzzUnpackRouteRefresh -fuzztime=10s -timeout=60s ./internal/component/bgp/message/...
	$(GO_TEST) -fuzz=FuzzChunkNLRI -fuzztime=10s -timeout=60s ./internal/component/bgp/message/...
	$(GO_TEST) -fuzz=FuzzParseIPv4Prefixes -fuzztime=10s -timeout=60s ./internal/component/bgp/wireu/...
	$(GO_TEST) -fuzz=FuzzParseIPv6Prefixes -fuzztime=10s -timeout=60s ./internal/component/bgp/wireu/...
	$(GO_TEST) -fuzz=FuzzParsePrefixes -fuzztime=10s -timeout=60s ./internal/component/bgp/wireu/...
	$(GO_TEST) -fuzz=FuzzParseRouteDistinguisher -fuzztime=10s -timeout=60s ./internal/component/bgp/nlri/...
	$(GO_TEST) -fuzz=FuzzParseRDString -fuzztime=10s -timeout=60s ./internal/component/bgp/nlri/...
	$(GO_TEST) -fuzz=FuzzParseLabelStack -fuzztime=10s -timeout=60s ./internal/component/bgp/nlri/...
	$(GO_TEST) -fuzz=FuzzParseVPN$$ -fuzztime=10s -timeout=60s ./internal/component/bgp/plugins/bgp-nlri-vpn/...
	$(GO_TEST) -fuzz=FuzzParseVPNAddPath -fuzztime=10s -timeout=60s ./internal/component/bgp/plugins/bgp-nlri-vpn/...
	$(GO_TEST) -fuzz=FuzzParseEVPN -fuzztime=10s -timeout=60s ./internal/component/bgp/plugins/bgp-nlri-evpn/...
	$(GO_TEST) -fuzz=FuzzParseFlowSpec$$ -fuzztime=10s -timeout=60s ./internal/component/bgp/plugins/bgp-nlri-flowspec/...
	$(GO_TEST) -fuzz=FuzzParseFlowSpecIPv6 -fuzztime=10s -timeout=60s ./internal/component/bgp/plugins/bgp-nlri-flowspec/...
	$(GO_TEST) -fuzz=FuzzParseFlowSpecVPN -fuzztime=10s -timeout=60s ./internal/component/bgp/plugins/bgp-nlri-flowspec/...
	$(GO_TEST) -fuzz=FuzzParseBGPLS$$ -fuzztime=10s -timeout=60s ./internal/component/bgp/plugins/bgp-nlri-ls/...
	$(GO_TEST) -fuzz=FuzzParseBGPLSWithRest -fuzztime=10s -timeout=60s ./internal/component/bgp/plugins/bgp-nlri-ls/...
	$(GO_TEST) -fuzz=FuzzParseMUP -fuzztime=10s -timeout=60s ./internal/component/bgp/plugins/bgp-nlri-mup/...
	$(GO_TEST) -fuzz=FuzzParseMVPN -fuzztime=10s -timeout=60s ./internal/component/bgp/plugins/bgp-nlri-mvpn/...
	$(GO_TEST) -fuzz=FuzzParseRTC -fuzztime=10s -timeout=60s ./internal/component/bgp/plugins/bgp-nlri-rtc/...
	$(GO_TEST) -fuzz=FuzzParseVPLS -fuzztime=10s -timeout=60s ./internal/component/bgp/plugins/bgp-nlri-vpls/...
	$(GO_TEST) -fuzz=FuzzScanner -fuzztime=10s -timeout=60s ./internal/component/bgp/textparse/...
	$(GO_TEST) -fuzz=FuzzConfigParser -fuzztime=10s -timeout=60s ./internal/component/config
	$(GO_TEST) -fuzz=FuzzTokenizer -fuzztime=10s -timeout=60s ./internal/component/config

# Run a single fuzz target for longer (usage: make ze-fuzz-one FUZZ=FuzzParseNLRIs PKG=./internal/component/bgp/wireu/... TIME=30s)
FUZZ ?= FuzzParseNLRIs
PKG  ?= ./internal/component/bgp/wireu/...
TIME ?= 30s

ze-fuzz-one:
	$(GO_TEST) -fuzz=$(FUZZ) -fuzztime=$(TIME) $(PKG)

# Run ExaBGP compatibility tests (Ze encoding matches ExaBGP)
# Uses uv to auto-install psutil dependency
ze-exabgp-test: bin/ze
	@echo "Running ExaBGP compatibility tests..."
	uv run --with psutil --with paramiko ./test/exabgp-compat/bin/functional encoding --timeout 60

# Run all ze tests including fuzz (ze only, no chaos/perf/analyse)
ze-test: ze-lint ze-unit-test ze-functional-test ze-exabgp-test ze-fuzz-test
	@echo "All ze tests passed"

# All tests except fuzz (ze only — use during development)
ze-verify: ze-lint ze-unit-test ze-functional-test ze-exabgp-test
	@echo "Ze verification passed"

# Everything: ze + chaos (no fuzz)
ze-all: ze-verify ze-chaos-verify
	@echo "All verification passed (ze + chaos)"

# Everything including fuzz: ze + chaos
ze-all-test: ze-test ze-chaos-verify
	@echo "All tests passed (ze + chaos + fuzz)"

# Codebase consistency checks (naming, structure, cross-refs, file sizes)
ze-consistency:
	@echo "Running consistency checks..."
	@go run scripts/consistency-check.go .

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
	$(GO_TEST) -race $(CHAOS_PACKAGES)

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

# ─── Interop tests ──────────────────────────────────────────────────────────

# Run interoperability tests against FRR and BIRD (requires Docker).
# Override FRR image: make ze-interop-test FRR_IMAGE=quay.io/frrouting/frr:10.3
# Run single scenario: make ze-interop-test INTEROP_SCENARIO=01-ebgp-ipv4-frr
INTEROP_SCENARIO ?=

ze-interop-test:
	@echo "Running interop tests (requires Docker)..."
	@python3 test/interop/run.py $(INTEROP_SCENARIO)

# ─── Performance benchmarks ────────────────────────────────────────────────

# Build ze-perf binary
ze-perf:
	@echo "Building ze-perf..."
	@mkdir -p bin
	$(GO) build -o bin/ze-perf ./cmd/ze-perf

# Run performance benchmarks against all DUTs (requires Docker).
# Override: DUT_ROUTES=100000 DUT_SEED=42 make ze-perf-bench
# Single DUT: make ze-perf-bench PERF_DUT=ze
# Skip image builds: NO_BUILD=1 make ze-perf-bench
PERF_DUT ?=

ze-perf-bench: ze-perf
	@echo "Running performance benchmarks (requires Docker)..."
	@python3 test/perf/run.py --build --test $(PERF_DUT)

# Generate comparison report from benchmark results.
ze-perf-report:
	@bin/ze-perf report test/perf/results/*.json --md

# Update history tracking from benchmark results.
ze-perf-track:
	@for f in test/perf/results/*.json; do \
		dut=$$(basename "$$f" .json); \
		bin/ze-perf track "test/perf/history/$${dut}.ndjson" --append "$$f"; \
	done

# ─── Spec status ─────────────────────────────────────────────────────────────

# Show spec inventory with progress status
ze-spec-status:
	@bash scripts/spec-status.sh

# Show spec inventory as JSON
ze-spec-status-json:
	@bash scripts/spec-status.sh --json

# ─── Inventory ──────────────────────────────────────────────────────────

# Generate project inventory (plugins, YANG, RPCs, tests, packages)
ze-inventory:
	@go run scripts/inventory.go

# Generate project inventory as JSON
ze-inventory-json:
	@go run scripts/inventory.go --json

# Check documentation drift against live registry and filesystem
ze-doc-drift:
	@go run scripts/check-doc-drift.go

# Cross-check YANG command tree against registered handlers
ze-validate-commands:
	@go run scripts/validate-commands.go

# Cross-check YANG command tree (JSON output)
ze-validate-commands-json:
	@go run scripts/validate-commands.go --json

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
	@echo "  build                 - Build all binaries (bin/ze, bin/ze-test, bin/ze-chaos, bin/ze-analyse)"
	@echo "  ze                    - Build bin/ze"
	@echo "  chaos                 - Build bin/ze-chaos"
	@echo "  test                  - Build bin/ze-test"
	@echo "  analyse               - Build bin/ze-analyse (MRT analysis tools)"
	@echo ""
	@echo "  Use GO=tinygo to build with TinyGo (e.g. make ze GO=tinygo)"
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
	@echo "  ze-test               - Ze tests: lint + unit + functional + exabgp + fuzz"
	@echo "  ze-verify             - Ze tests except fuzz (development)"
	@echo "  ze-all                - Everything: ze-verify + ze-chaos-verify"
	@echo "  ze-all-test           - Everything + fuzz: ze-test + ze-chaos-verify"
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
	@echo "  Interop tests (Docker):"
	@echo "  ze-interop-test          - Run interop tests against FRR and BIRD"
	@echo "                             INTEROP_SCENARIO=name to run one scenario"
	@echo ""
	@echo "  Performance benchmarks (Docker):"
	@echo "  ze-perf            - Build ze-perf binary"
	@echo "  ze-perf-bench            - Run benchmarks against all DUTs"
	@echo "                             PERF_DUT=name to run one DUT"
	@echo "  ze-perf-report           - Generate comparison report from results"
	@echo "  ze-perf-track            - Update history tracking from results"
	@echo ""
	@echo "  Spec status:"
	@echo "  ze-spec-status        - Show spec inventory with progress status"
	@echo "  ze-spec-status-json   - Show spec inventory as JSON"
	@echo ""
	@echo "  Inventory:"
	@echo "  ze-inventory          - Generate project inventory (plugins, YANG, RPCs, tests)"
	@echo "  ze-inventory-json     - Generate project inventory as JSON"
	@echo "  ze-validate-commands  - Cross-check YANG command tree vs registered handlers"
	@echo ""
	@echo "  Utilities:"
	@echo "  fmt                   - Format code (gofmt + goimports)"
	@echo "  vet                   - Run go vet"
	@echo "  tidy                  - Tidy go.mod dependencies"
	@echo "  clean                 - Remove build artifacts"
	@echo "  check                 - Quick check (fmt + vet)"
	@echo "  help                  - Show this help"
