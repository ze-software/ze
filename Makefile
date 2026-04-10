.PHONY: all build ze chaos test analyse clean fmt vet tidy generate help
.PHONY: ze-lint ze-unit-test ze-unit-test-cover ze-functional-test ze-exabgp-test ze-fuzz-test ze-fuzz-one ze-test ze-verify ze-ci
.PHONY: ze-lint-changed ze-unit-test-changed ze-verify-changed
.PHONY: ze-encode-test ze-plugin-test ze-decode-test ze-parse-test ze-reload-test ze-ui-test ze-editor-test ze-managed-test
.PHONY: ze-chaos-lint ze-chaos-unit-test ze-chaos-functional-test ze-chaos-web-test ze-chaos-test ze-chaos-verify
.PHONY: ze-all ze-all-test
.PHONY: ze-interop-test ze-stress-test ze-stress-bird-test ze-stress-profile ze-live-test ze-live-rpki-test
.PHONY: ze-integration-test ze-integration-iface-test ze-integration-fib-test
.PHONY: ze-perf ze-perf-bench ze-perf-report ze-perf-track
.PHONY: ze-spec-status ze-spec-status-json ze-inventory ze-inventory-json ze-command-list ze-command-list-json ze-validate-commands ze-validate-commands-json ze-doc-drift ze-doc-test
.PHONY: ze-sync-vendor-web ze-check-vendor-web
.PHONY: check ze-setup
.PHONY: ze-gokrazy ze-gokrazy-deps ze-gokrazy-run

# Environment: keep build caches within CURDIR (not TMPDIR - breaks Unix socket tests)
export GOCACHE := $(CURDIR)/tmp/go-cache
export GOLANGCI_LINT_CACHE := $(CURDIR)/tmp/golangci-lint-cache

# Go compiler: override with GO=tinygo for smaller binaries
# TinyGo finds go via PATH, so we prepend Go 1.25 when GO=tinygo
GO ?= go
ifeq ($(GO),tinygo)
export PATH := /opt/homebrew/opt/go@1.25/bin:$(PATH)
endif

# Build tags: optional compile-time features (e.g. ZE_TAGS=maprib)
#   maprib  - Use Go map for RIB storage (default: BART trie)
ZE_TAGS ?=
ifneq ($(ZE_TAGS),)
ZE_TAGFLAG := -tags $(ZE_TAGS)
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
	@go run scripts/codegen/plugin_imports.go

# Build all binaries
build: generate bin/ze bin/ze-test bin/ze-chaos bin/ze-analyse docs/comparison.html
	@echo "All binaries built"

# Regenerate comparison HTML when markdown changes
docs/comparison.html: docs/comparison.md scripts/codegen/comparison_html.go
	@go run scripts/codegen/comparison_html.go

ze:
	@mkdir -p bin
	$(GO) build $(ZE_TAGFLAG) -ldflags "$(ZE_LDFLAGS)" -o bin/ze ./cmd/ze

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
	$(GO) build $(ZE_TAGFLAG) -ldflags "$(ZE_LDFLAGS)" -o bin/ze ./cmd/ze

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
	bin/ze-test managed --all || { failed=$$((failed + 1)); failed_names="$${failed_names:+$$failed_names }managed"; }; \
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
		printf "\033[32mPASS  all 8 suites\033[0m\n\n"; \
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

ze-web-test: bin/ze bin/ze-test
	@bin/ze-test web

ze-managed-test: bin/ze-test
	@bin/ze-test managed --all

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

# All tests except fuzz (ze only -- use during development)
ze-verify: ze-lint ze-unit-test ze-functional-test ze-exabgp-test
	@echo "Ze verification passed"

# --- Scoped targets (parallel-safe: only lint/test packages with changed .go files) ---

# Lint only packages containing modified .go files (unstaged + staged + untracked)
ze-lint-changed:
	@pkgs=$$({ git diff --name-only -- '*.go'; git diff --cached --name-only -- '*.go'; git ls-files --others --exclude-standard -- '*.go'; } 2>/dev/null \
		| sort -u | xargs -n1 dirname 2>/dev/null | sort -u | sed 's|^|./|'); \
	if [ -z "$$pkgs" ]; then echo "No changed Go packages to lint"; exit 0; fi; \
	echo "Linting changed packages: $$pkgs"; \
	golangci-lint run $$pkgs

# Unit-test only packages containing modified .go files
ze-unit-test-changed:
	@pkgs=$$({ git diff --name-only -- '*.go'; git diff --cached --name-only -- '*.go'; git ls-files --others --exclude-standard -- '*.go'; } 2>/dev/null \
		| sort -u | xargs -n1 dirname 2>/dev/null | sort -u | sed 's|^|./|'); \
	if [ -z "$$pkgs" ]; then echo "No changed Go packages to test"; exit 0; fi; \
	echo "Testing changed packages: $$pkgs"; \
	$(GO_TEST) -race $$pkgs

# Scoped verify: lint + test changed packages, then full functional + exabgp
ze-verify-changed: ze-lint-changed ze-unit-test-changed ze-functional-test ze-exabgp-test
	@echo "Ze verification (changed) passed"

# Everything: ze + chaos (no fuzz)
ze-all: ze-verify ze-chaos-verify
	@echo "All verification passed (ze + chaos)"

# Everything including fuzz: ze + chaos
ze-all-test: ze-test ze-chaos-verify
	@echo "All tests passed (ze + chaos + fuzz)"

# Codebase consistency checks (naming, structure, cross-refs, file sizes)
ze-consistency:
	@echo "Running consistency checks..."
	@go run scripts/lint/consistency.go .

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

# ─── Stress tests (BNG Blaster) ────────────────────────────────────────────

# Run BGP stress tests using BNG Blaster (requires Linux, root, BNG Blaster installed).
# Install BNG Blaster first: sudo python3 test/stress/setup.py
# Run single scenario: make ze-stress-test STRESS_SCENARIO=01-bulk-ipv4
STRESS_SCENARIO ?=

ze-stress-test: bin/ze
	@echo "Running stress tests with BNG Blaster (requires root + netns)..."
	@sudo ZE_BINARY=$(CURDIR)/bin/ze VERBOSE=$(VERBOSE) SESSION_TIMEOUT=$(SESSION_TIMEOUT) \
		python3 test/stress/run.py $(STRESS_SCENARIO)

# Run BIRD baseline stress test (requires bird2 installed).
ze-stress-bird-test:
	@echo "Running BIRD baseline stress test (requires root + bird2 + netns)..."
	@sudo VERBOSE=$(VERBOSE) SESSION_TIMEOUT=$(SESSION_TIMEOUT) \
		python3 test/stress/run.py 04-bulk-ipv4-bird

# Run 1M route profiling stress test (captures CPU/heap/goroutine profiles).
# Profiles saved to tmp/stress-profile-{cpu,heap,goroutine}.pb.gz
# Analyze: go tool pprof -http=:8080 tmp/stress-profile-cpu.pb.gz
ze-stress-profile: bin/ze
	@echo "Running 1M profile stress test (requires root + netns)..."
	@sudo ZE_BINARY=$(CURDIR)/bin/ze ZE_PPROF=1 VERBOSE=$(VERBOSE) \
		python3 test/stress/run.py 05-profile-1m

# ─── Live tests ────────────────────────────────────────────────────────────

# Run all live integration tests (requires Docker + internet).
# Tests connect to real external infrastructure (e.g., RPKI cache servers).
ze-live-test: ze-live-rpki-test

# Run RPKI live test (stayrtr container with real-world RPKI data).
ze-live-rpki-test:
	@echo "Running RPKI live test (requires Docker + internet)..."
	$(GO) test -v -tags live -timeout 180s -count=1 ./internal/component/bgp/plugins/rpki/... -run TestLive

# ─── Integration tests (network namespace) ──────────────────────────────────

# Run iface integration tests (requires CAP_NET_ADMIN / root).
# These tests create ephemeral network namespaces and exercise netlink operations.
ze-integration-iface-test:
	@echo "Running iface integration tests (requires CAP_NET_ADMIN)..."
	$(GO) test -tags integration -count=1 -race -timeout 120s ./internal/component/iface/...

# Run FIB kernel integration tests (requires CAP_NET_ADMIN / root).
# Tests actual netlink route programming in isolated network namespaces.
ze-integration-fib-test:
	@echo "Running FIB kernel integration tests (requires CAP_NET_ADMIN)..."
	$(GO) test -tags integration -count=1 -race -timeout 120s ./internal/plugins/fibkernel/...

# Run all integration tests (requires CAP_NET_ADMIN / root).
ze-integration-test: ze-integration-iface-test ze-integration-fib-test

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
	@go run scripts/status/spec_status.go

# Show spec inventory as JSON
ze-spec-status-json:
	@go run scripts/status/spec_status.go --json

# ─── Inventory ──────────────────────────────────────────────────────────

# Generate project inventory (plugins, YANG, RPCs, tests, packages)
ze-inventory:
	@go run scripts/inventory/inventory.go

# Generate project inventory as JSON
ze-inventory-json:
	@go run scripts/inventory/inventory.go --json

# Generate command inventory (all registered commands, classified by verb)
ze-command-list:
	@go run scripts/inventory/commands.go

# Generate command inventory as JSON
ze-command-list-json:
	@go run scripts/inventory/commands.go --json

# Check documentation drift against live registry and filesystem
ze-doc-drift:
	@go run scripts/docvalid/doc_drift.go

# Cross-check YANG command tree against registered handlers
ze-validate-commands:
	@go run scripts/docvalid/commands.go

# Cross-check YANG command tree (JSON output)
ze-validate-commands-json:
	@go run scripts/docvalid/commands.go --json

# Run all documentation tests: drift check + YANG/handler contract.
# Each tool runs independently so the user sees ALL issues, not just the first
# tool that fails. Returns non-zero if any tool reports drift.
# See docs/contributing/documentation-testing.md for the workflow.
ze-doc-test:
	@echo "Running documentation tests..."
	@FAIL=0; \
	echo ""; \
	echo "  -> Documentation drift (DESIGN.md, comparison.md vs registry)..."; \
	go run scripts/docvalid/doc_drift.go || FAIL=1; \
	echo ""; \
	echo "  -> YANG/handler contract (validate-commands)..."; \
	go run scripts/docvalid/commands.go || FAIL=1; \
	echo ""; \
	if [ $$FAIL -ne 0 ]; then \
		echo "Documentation tests FAILED -- see output above."; \
		echo "See docs/contributing/documentation-testing.md for how to fix."; \
		exit 1; \
	fi; \
	echo "Documentation tests PASSED"

# Sync vendored web assets to consumer directories
ze-sync-vendor-web:
	@go run scripts/vendor/sync_web.go

# Check vendored web assets for newer versions
ze-check-vendor-web:
	@go run scripts/vendor/check_web.go

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

# ─── Gokrazy VM appliance ────────────────────────────────────────────────────
#
# Builds a bootable x86_64 VM image with Ze baked in.
# Everything is vendored: gok tool source in gokrazy/tools/vendor/,
# dependency pins in gokrazy/ze/builddir/*/go.mod.
# After cloning, run `make ze-gokrazy-deps` once to populate the Go module cache
# for the gokrazy system packages (kernel, init). After that, builds work offline.
#
# Requires: e2fsprogs (brew install e2fsprogs)
#           qemu (brew install qemu) -- for ze-gokrazy-run only
#
# The image contains:
#   - Linux kernel + gokrazy init (process supervisor, DHCP, NTP, web UI)
#   - Ze binary with all internal plugins compiled in
#   - Seed config at /etc/ze/ze.conf (read-only root)
#   - SSH credentials in /perm/ze/database.zefs (persistent partition)
#
# Gokrazy web UI: http://gokrazy:<password>@localhost:18080/
# Ze web UI:      http://localhost:28080/
# Ze SSH CLI:     ssh -p 2222 <user>@localhost
#
# Usage:
#   make ze-gokrazy-deps                    -- one-time: download gokrazy system packages
#   make ze-gokrazy USER=admin PASS=secret  -- build image with SSH credentials
#   make ze-gokrazy-run                     -- boot image in QEMU

GOKRAZY_INSTANCE   := ze
GOKRAZY_DIR        := gokrazy
GOKRAZY_IMG        := tmp/gokrazy/ze.img
GOKRAZY_IMG_SIZE   := 2147483648
GOKRAZY_PERM_OFF   := 1157627904
GOKRAZY_PERM_BLK   := 966639
GOKRAZY_PERM_4K    := 241660
GOKRAZY_PERM_SKIP  := 282624
E2FS               := /opt/homebrew/Cellar/e2fsprogs/1.47.4/sbin

# Build ze-gok from vendored source (gokrazy/tools/).
# ze-gok wraps gok with a repo-local GOMODCACHE so all module resolution
# stays under gokrazy/modcache/ (committed Go source, .gitignored kernel).
bin/gok:
	@echo "Building ze-gok from vendored source..."
	@mkdir -p bin
	go build -C $(GOKRAZY_DIR)/tools -mod=vendor -o ../../bin/gok ./cmd/ze-gok

# Download gokrazy system packages into the repo-local module cache.
# Only fetches gokrazy's own packages (kernel, init, serial-busybox).
# Ze's dependencies are already in the repo's vendor/ directory.
GOMODCACHE_LOCAL := $(CURDIR)/$(GOKRAZY_DIR)/modcache

ze-gokrazy-deps: bin/gok
	@echo "Downloading gokrazy dependencies into $(GOKRAZY_DIR)/modcache/..."
	@for d in $$(find $(GOKRAZY_DIR)/$(GOKRAZY_INSTANCE)/builddir -name go.mod -exec dirname {} \;); do \
		echo "  $$d"; \
		(cd "$$d" && GOMODCACHE=$(GOMODCACHE_LOCAL) go mod download all) || exit 1; \
	done
	@echo "Done. Builds now work offline."

# Build a bootable VM image with SSH credentials baked in.
ze-gokrazy: ze bin/gok
	@test -n "$(USER)" || { echo "Usage: make ze-gokrazy USER=admin PASS=secret"; exit 1; }
	@test -n "$(PASS)" || { echo "Usage: make ze-gokrazy USER=admin PASS=secret"; exit 1; }
	@test -f $(E2FS)/mkfs.ext4 || { echo "error: e2fsprogs not found (brew install e2fsprogs)"; exit 1; }
	@mkdir -p tmp/gokrazy/init
	@echo "--- Creating SSH credentials ---"
	@printf '%s\n' "$(USER)" "$(PASS)" "0.0.0.0" "22" "ze" | \
		env ze.config.dir=tmp/gokrazy/init bin/ze init --force 2>&1
	@echo "--- Building gokrazy image ---"
	GOARCH=amd64 bin/gok --parent_dir $(GOKRAZY_DIR) -i $(GOKRAZY_INSTANCE) overwrite \
		--full $(GOKRAZY_IMG) \
		--target_storage_bytes $(GOKRAZY_IMG_SIZE)
	@echo "--- Formatting /perm partition ---"
	$(E2FS)/mkfs.ext4 -q -F -E offset=$(GOKRAZY_PERM_OFF) $(GOKRAZY_IMG) $(GOKRAZY_PERM_BLK)
	@echo "--- Injecting credentials into /perm ---"
	@dd if=$(GOKRAZY_IMG) of=tmp/gokrazy/perm.img bs=4096 skip=$(GOKRAZY_PERM_SKIP) count=$(GOKRAZY_PERM_4K) 2>/dev/null
	@$(E2FS)/debugfs -w -R "mkdir ze" tmp/gokrazy/perm.img 2>/dev/null
	@$(E2FS)/debugfs -w -R "write tmp/gokrazy/init/database.zefs ze/database.zefs" tmp/gokrazy/perm.img 2>/dev/null
	@dd if=tmp/gokrazy/perm.img of=$(GOKRAZY_IMG) bs=4096 seek=$(GOKRAZY_PERM_SKIP) conv=notrunc 2>/dev/null
	@rm -f tmp/gokrazy/perm.img
	@echo ""
	@echo "Image ready: $(GOKRAZY_IMG)"
	@echo "Run: make ze-gokrazy-run"

# Boot the VM image in QEMU with port forwarding.
ze-gokrazy-run:
	@test -f $(GOKRAZY_IMG) || { echo "error: $(GOKRAZY_IMG) not found (run: make ze-gokrazy USER=admin PASS=secret)"; exit 1; }
	@command -v qemu-system-x86_64 >/dev/null || { echo "error: qemu not found (brew install qemu)"; exit 1; }
	@echo "Booting Ze gokrazy appliance..."
	@echo "  Gokrazy web: http://gokrazy:$$(python3 -c "import json; print(json.load(open('$(GOKRAZY_DIR)/$(GOKRAZY_INSTANCE)/config.json'))['Update']['HTTPPassword'])" 2>/dev/null || echo 'see config')@localhost:18080/"
	@echo "  Ze web:      http://localhost:28080/"
	@echo "  Ze SSH:      ssh -p 2222 <user>@localhost"
	@echo "  Quit:        Ctrl-A X"
	@echo ""
	qemu-system-x86_64 \
		-machine accel=tcg \
		-smp 2 -m 512 \
		-drive file=$(GOKRAZY_IMG),format=raw \
		-nographic -serial mon:stdio \
		-nic user,model=e1000,hostfwd=tcp::18080-:80,hostfwd=tcp::28080-:8080,hostfwd=tcp::2222-:22

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/ tmp/
	rm -f coverage.out coverage.html

# Quick check (fast feedback during development)
check: fmt vet
	@echo "Quick check passed"

# ─── Setup ───────────────────────────────────────────────────────────────────

# Install development tools. Run once after cloning.
# Go tools (goimports, protoc plugins) are vendored via tools.go and
# used with "go run" -- no "go install" needed.
# golangci-lint is installed separately (large dependency tree).
# System packages (protoc, jq) require the OS package manager.
ze-setup:
	@echo "Vendoring Go dependencies (includes tools from tools.go)..."
	go mod tidy
	go mod vendor
	@echo ""
	@echo "Installing golangci-lint..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo ""
	@echo "Installing system packages..."
ifeq ($(shell uname -s),Darwin)
	brew install protobuf jq
else
	@echo "Run: sudo apt install -y protobuf-compiler jq"
	@echo "(requires sudo -- not run automatically)"
endif
	@echo ""
	@echo "Setup complete. Verify with: make check"

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
	@echo "  Build options:"
	@echo "    GO=tinygo              - Build with TinyGo (does not work yet -- TinyGo limitations)"
	@echo "    ZE_TAGS=maprib         - Use Go map RIB instead of BART trie (e.g. make ze ZE_TAGS=maprib)"
	@echo "    ZE_TAGS='maprib,foo'   - Multiple build tags"
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
	@echo "  ze-lint-changed       - Lint only packages with changed .go files (parallel-safe)"
	@echo "  ze-unit-test-changed  - Unit test only packages with changed .go files"
	@echo "  ze-verify-changed     - Scoped lint+test, then full functional+exabgp"
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
	@echo "  ze-stress-test           - Run BGP stress tests with BNG Blaster (Linux, root)"
	@echo "                             STRESS_SCENARIO=name to run one scenario"
	@echo "  ze-stress-bird-test      - Run BIRD baseline stress test (Linux, root, bird2)"
	@echo "  ze-stress-profile        - Run 1M profile test, saves pprof to tmp/"
	@echo ""
	@echo "  Stress test options:"
	@echo "    STRESS_SCENARIO=name   - Run single scenario (e.g. 01-bulk-ipv4)"
	@echo "    ZE_TAGS=maprib         - Build Ze with Go map RIB instead of BART trie"
	@echo "    VERBOSE=1              - Show debug output from test harness"
	@echo "    SESSION_TIMEOUT=N      - BGP session timeout in seconds (default: 120)"
	@echo ""
	@echo "  Examples:"
	@echo "    make ze-stress-test                          # all scenarios, default build"
	@echo "    make ze-stress-test ZE_TAGS=maprib           # all scenarios, Go map RIB"
	@echo "    make ze-stress-test STRESS_SCENARIO=01-bulk-ipv4  # single scenario"
	@echo "    make ze-stress-profile ZE_TAGS=maprib        # profile with Go map RIB"
	@echo ""
	@echo "  Integration tests (CAP_NET_ADMIN / root):"
	@echo "  ze-integration-test      - Run all integration tests (network namespaces)"
	@echo "  ze-integration-iface-test - Run iface integration tests"
	@echo "  ze-integration-fib-test  - Run FIB kernel netlink integration tests"
	@echo ""
	@echo "  Live tests (Docker + internet):"
	@echo "  ze-live-test             - Run all live integration tests"
	@echo "  ze-live-rpki-test        - Run RPKI live test (stayrtr + real data)"
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
	@echo "  ze-command-list       - Generate command inventory (all commands by verb)"
	@echo "  ze-command-list-json  - Generate command inventory as JSON"
	@echo "  ze-sync-vendor-web    - Sync vendored web assets to consumer directories"
	@echo "  ze-check-vendor-web   - Check vendored web assets for newer versions"
	@echo ""
	@echo "  Documentation testing:"
	@echo "  ze-doc-test           - Run all doc tests (drift + YANG/handler contract)"
	@echo "  ze-doc-drift          - Check DESIGN.md/comparison.md claims vs live registry"
	@echo "  ze-validate-commands  - Cross-check YANG ze:command vs registered RPC handlers"
	@echo "  ze-consistency        - Code/doc consistency: design refs, cross-refs, stale refs"
	@echo "  See docs/contributing/documentation-testing.md for the workflow."
	@echo ""
	@echo "  Gokrazy VM appliance (x86_64, see docs/guide/appliance.md):"
	@echo "  ze-gokrazy-deps          - One-time: download gokrazy system packages into Go module cache"
	@echo "  ze-gokrazy USER=x PASS=y - Build bootable VM image with Ze + SSH credentials"
	@echo "  ze-gokrazy-run           - Boot the VM image in QEMU (Ctrl-A X to quit)"
	@echo ""
	@echo "  Utilities:"
	@echo "  ze-setup              - Install dev tools (goimports, golangci-lint, protoc plugins)"
	@echo "  fmt                   - Format code (gofmt + goimports)"
	@echo "  vet                   - Run go vet"
	@echo "  tidy                  - Tidy go.mod dependencies"
	@echo "  clean                 - Remove build artifacts"
	@echo "  check                 - Quick check (fmt + vet)"
	@echo "  help                  - Show this help"
