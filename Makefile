.PHONY: all build ze chaos test analyse clean fmt vet tidy generate help
.PHONY: ze-docker
.PHONY: ze-lint ze-vet-evidence ze-race-reactor ze-linux-test ze-exabgp-test
.PHONY: ze-test ze-verify ze-verify-changed ze-smoke ze-ci ze-all ze-all-test
.PHONY: ze-lint-changed ze-unit-test-changed ze-clean-tmp
.PHONY: _ze-verify-impl _ze-verify-changed-impl
.PHONY: ze-sync-vendor-web ze-check-vendor-web ze-ai-sync ze-ai-instructions
.PHONY: ze-regen ze-regen-check
.PHONY: check ze-setup
.PHONY: help-test help-deploy help-dev

# Environment: keep build caches within CURDIR (not TMPDIR - breaks Unix socket tests)
export GOCACHE := $(CURDIR)/tmp/go-cache
export GOLANGCI_LINT_CACHE := $(CURDIR)/tmp/golangci-lint-cache
export CGO_ENABLED := 0

# Go compiler: override with GO=tinygo for smaller binaries
# TinyGo finds go via PATH, so we prepend Go 1.26 when GO=tinygo
GO ?= go
ifeq ($(GO),tinygo)
export PATH := /opt/homebrew/opt/go@1.26/bin:$(PATH)
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

# CPU limit: leave 3 cores free (minimum 1). Used as GOMAXPROCS for tests so
# parallel stages do not starve the system.
GO_TEST_PROCS := $(shell n=$$(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4); p=$$(( n - 3 )); [ $$p -lt 1 ] && p=1; echo $$p)
GO_TEST = GOMAXPROCS=$(GO_TEST_PROCS) go test
ZE_EXABGP_TIMEOUT ?= 180
ZE_LINUX_GO_IMAGE ?= golang:1.26-alpine
ZE_LINUX_TEST_PACKAGES ?= ./internal/plugins/traffic/vpp

# Packages
ZE_PACKAGES = $$(go list ./... | grep -v /cmd/ze-chaos)

# Default target
.DEFAULT_GOAL := help

# ─── Include split Makefile modules ─────────────────────────────────────────
include mk/test-unit.mk
include mk/test-functional.mk
include mk/test-fuzz.mk
include mk/test-chaos.mk
include mk/test-integration.mk
include mk/test-release.mk
include mk/perf.mk
include mk/inventory.mk
include mk/gokrazy.mk

# ─── Build ──────────────────────────────────────────────────────────────────

all: ze-lint ze-unit-test build

generate:
	@go run scripts/codegen/plugin_imports.go

build: generate bin/ze bin/ze-test bin/ze-chaos bin/ze-analyse docs/comparison.html
	@echo "All binaries built"

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

# ─── Docker ────────────────────────────────────────────────────────────────

ZE_DOCKER_IMAGE ?= ze
ZE_DOCKER_TAG ?= $(ZE_VERSION)

ze-docker:
	@command -v docker >/dev/null || { echo "error: docker not found"; exit 1; }
	docker build \
		-f docker/Dockerfile \
		--build-arg ZE_VERSION=$(ZE_VERSION) \
		--build-arg ZE_BUILD_DATE=$(ZE_BUILD_DATE) \
		$(if $(ZE_TAGS),--build-arg ZE_TAGS=$(ZE_TAGS)) \
		-t $(ZE_DOCKER_IMAGE):$(ZE_DOCKER_TAG) \
		-t $(ZE_DOCKER_IMAGE):latest \
		.

# ─── Lint and specialised test targets ──────────────────────────────────────

ze-lint:
	@echo "Running ze linter..."
	@golangci-lint run ./cmd/ze/... ./cmd/ze-test/... ./internal/... ./pkg/... ./parked/... ./test/...

ze-vet-evidence:
	@echo "Vetting evidence scripts (GOOS=linux)..."
	@GOOS=linux go vet ./scripts/evidence/...

ze-race-reactor:
	@echo "Stress race-test reactor (count=20)..."
	$(GO_TEST) -race -count=20 ./internal/component/bgp/reactor/...

ze-linux-test:
	@command -v docker >/dev/null || { echo "error: docker not found"; exit 1; }
	@mkdir -p tmp/linux-go-cache tmp/linux-gomodcache
	docker run --rm \
		--user "$$(id -u):$$(id -g)" \
		-v "$(CURDIR):/src" \
		-w /src \
		-e HOME=/tmp \
		-e GOCACHE=/src/tmp/linux-go-cache \
		-e GOMODCACHE=/src/tmp/linux-gomodcache \
		$(ZE_LINUX_GO_IMAGE) \
		go test $(ZE_LINUX_TEST_PACKAGES) -count=1

ze-exabgp-test: bin/ze
	@echo "Running ExaBGP compatibility tests..."
	uv run --with psutil --with paramiko ./test/exabgp-compat/bin/functional encoding --timeout $(ZE_EXABGP_TIMEOUT)

# ─── Scoped targets (parallel-safe) ────────────────────────────────────────

ze-lint-changed:
	@pkgs=$$({ git diff --name-only -- '*.go'; git diff --cached --name-only -- '*.go'; git ls-files --others --exclude-standard -- '*.go'; } 2>/dev/null \
		| sort -u | xargs -n1 dirname 2>/dev/null | sort -u | sed 's|^|./|'); \
	if [ -z "$$pkgs" ]; then echo "No changed Go packages to lint"; exit 0; fi; \
	echo "Linting changed packages: $$pkgs"; \
	golangci-lint run $$pkgs

ze-unit-test-changed:
	@pkgs=$$({ git diff --name-only -- '*.go'; git diff --cached --name-only -- '*.go'; git ls-files --others --exclude-standard -- '*.go'; } 2>/dev/null \
		| sort -u | xargs -n1 dirname 2>/dev/null | sort -u | sed 's|^|./|'); \
	if [ -z "$$pkgs" ]; then echo "No changed Go packages to test"; exit 0; fi; \
	echo "Testing changed packages: $$pkgs"; \
	$(GO_TEST) -race $$pkgs

# ─── Composite verification targets ────────────────────────────────────────

ze-test: ze-lint ze-unit-test ze-functional-test ze-exabgp-test ze-fuzz-test
	@echo "All ze tests passed"

ZE_VERIFY_LOG ?= tmp/ze-verify.log

ze-verify:
	@scripts/dev/verify-lock.sh ze-verify $(MAKE) --no-print-directory _ze-verify-impl

_ze-verify-impl: ze-lint ze-vet-evidence ze-unit-test-cached ze-unit-test-race-changed ze-functional-test ze-exabgp-test
	@echo "Ze verification passed"

ze-verify-changed:
	@scripts/dev/verify-lock.sh ze-verify-changed $(MAKE) --no-print-directory _ze-verify-changed-impl

_ze-verify-changed-impl: ze-lint-changed ze-unit-test-changed ze-functional-test ze-exabgp-test
	@echo "Ze verification (changed) passed"

ze-all: ze-verify ze-chaos-verify
	@echo "All verification passed (ze + chaos)"

ze-all-test: ze-test ze-chaos-verify
	@echo "All tests passed (ze + chaos + fuzz)"

ze-smoke: ze-lint ze-unit-test build
	@echo "Ze smoke check passed"

ze-ci: ze-smoke

# ─── Utilities ──────────────────────────────────────────────────────────────

fmt:
	@echo "Formatting code..."
	gofmt -w .
	goimports -w .

vet:
	@echo "Running go vet..."
	go vet ./...

tidy:
	@echo "Tidying dependencies..."
	go mod tidy

ze-sync-vendor-web:
	@go run scripts/vendor/sync_web.go

ze-check-vendor-web:
	@go run scripts/vendor/check_web.go

ze-ai-instructions:
	@sed 's/{{TOOL}}/Claude/' ai/INSTRUCTIONS.md > CLAUDE.md
	@sed 's/{{TOOL}}/Codex/' ai/INSTRUCTIONS.md > AGENTS.md

ze-ai-sync:
	@scripts/dev/skill_sync.sh

ze-regen: generate ze-ai-instructions ze-ai-sync ze-doc-index
	@echo "All generated files updated"

ze-regen-check: ze-regen
	@if ! git diff --quiet -- CLAUDE.md AGENTS.md .claude/skills/ .codex/skills/ .agents/skills/ ai/CODE-TO-DOCS.md internal/component/plugin/all/all.go 2>/dev/null; then \
		echo "ERROR: Generated files are stale. Run 'make ze-regen' and commit the result." >&2; \
		git diff --stat -- CLAUDE.md AGENTS.md .claude/skills/ .codex/skills/ .agents/skills/ ai/CODE-TO-DOCS.md internal/component/plugin/all/all.go; \
		exit 1; \
	fi
	@python3 scripts/dev/code_to_docs.py --check
	@echo "All generated files are up to date"

clean:
	@echo "Cleaning..."
	rm -rf bin/ tmp/
	rm -f coverage.out coverage.html

ze-clean-tmp:
	@echo "Cleaning tmp/ scratch files older than 24h..."
	@find tmp/ -maxdepth 1 -type f -mmin +1440 -delete 2>/dev/null || true
	@find tmp/ -maxdepth 1 -type d -not -name tmp -not -name session \
		-not -name go-cache -not -name golangci-lint-cache -not -name kernel \
		-mmin +1440 -exec rm -rf {} + 2>/dev/null || true
	@find tmp/session/ -maxdepth 1 -type f -mmin +1440 -delete 2>/dev/null || true
	@echo "Done. $$(ls -1 tmp/ 2>/dev/null | wc -l | tr -d ' ') entries remain."

check: fmt vet
	@echo "Quick check passed"

# ─── Setup ──────────────────────────────────────────────────────────────────

ze-setup:
	@echo "Vendoring Go dependencies (includes tools from tools.go)..."
	go mod tidy
	go mod vendor
	@echo ""
	@echo "Installing golangci-lint..."
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1
	@echo ""
	@echo "Installing system packages..."
ifeq ($(shell uname -s),Darwin)
	brew install protobuf jq pipx
else
	@echo "Run: sudo apt install -y protobuf-compiler jq pipx"
	@echo "(requires sudo -- not run automatically)"
endif
	@echo ""
	@echo "Installing ruff (Python linter) via pipx..."
	@command -v pipx >/dev/null 2>&1 || { echo "pipx missing -- install it first (see above), then re-run 'make ze-setup'"; exit 1; }
	pipx install --force ruff
	@echo ""
	@echo "Setup complete. Verify with: make ze-smoke"

# ─── Help ───────────────────────────────────────────────────────────────────

help:
	@echo "Ze Network OS -- Build & Test"
	@echo ""
	@echo "  Start here (new contributor):"
	@echo "    make ze-setup             Install dev tools (one-time)"
	@echo "    make ze-smoke             Verify setup: lint + unit + build (~2 min)"
	@echo ""
	@echo "  Daily development:"
	@echo "    make ze-test-bgp          Unit tests for one component group (-race)"
	@echo "                              Also: ze-test-core, ze-test-plugins, ze-test-config, ze-test-cli, ze-test-rest"
	@echo "    make ze-encode-test       Single functional suite (encode, plugin, decode, parse, reload, ...)"
	@echo "    make ze-verify            Pre-commit gate: lint + unit + functional + exabgp (~2 min)"
	@echo "    make ze-verify-changed    Scoped verify: only changed packages, then full functional"
	@echo ""
	@echo "  Build:"
	@echo "    make build                All binaries (ze, ze-test, ze-chaos, ze-analyse)"
	@echo "    make ze                   Just bin/ze"
	@echo ""
	@echo "  More help:"
	@echo "    make help-test            All test targets (unit, functional, fuzz, chaos, interop, ...)"
	@echo "    make help-deploy          Gokrazy appliance, kernel builds, deployment evidence"
	@echo "    make help-dev             Inventory, doc validation, spec status, utilities"
	@echo ""
	@echo "  See also: docs/contributing/testing.md"

help-test:
	@echo "Ze Test Targets"
	@echo ""
	@echo "  Unit tests (Go test with -race):"
	@echo "    ze-unit-test              All packages (~5 min)"
	@echo "    ze-unit-test-cover        All packages with coverage report"
	@echo "    ze-test-bgp               BGP component group (~1:30)"
	@echo "    ze-test-core              Core libraries (~30s)"
	@echo "    ze-test-plugins           Plugins (~40s)"
	@echo "    ze-test-config            Config component (~20s)"
	@echo "    ze-test-cli               CLI component (~10s)"
	@echo "    ze-test-rest              Everything else (~1:00)"
	@echo "    ze-race-reactor           Stress race-test reactor (-race -count=20)"
	@echo ""
	@echo "  Functional tests (.ci suites via bin/ze-test):"
	@echo "    ze-functional-test        All 12 gating suites"
	@echo "    ze-encode-test            BGP wire encoding"
	@echo "    ze-plugin-test            Plugin behavior"
	@echo "    ze-decode-test            Wire decoding"
	@echo "    ze-parse-test             Config parsing"
	@echo "    ze-reload-test            Config reload"
	@echo "    ze-ui-test                CLI/completion"
	@echo "    ze-editor-test            TUI editor (.et files)"
	@echo "    ze-managed-test           Managed config"
	@echo "    ze-web-test               Web UI (browser)"
	@echo "    ze-l2tp-test              L2TP"
	@echo "    ze-firewall-test          Firewall"
	@echo "    ze-policy-test            Policy routing"
	@echo "    ---"
	@echo "    ze-static-test            Static routes (release evidence only)"
	@echo "    ze-traffic-test           Traffic control (release evidence only)"
	@echo "    ze-vpp-test               VPP stub (release evidence only)"
	@echo "    ze-l2tp-wire-test         L2TP wire-level (release evidence only)"
	@echo ""
	@echo "  Fuzz:"
	@echo "    ze-fuzz-test              All fuzz targets (10s each)"
	@echo "    ze-fuzz-one               Single target (FUZZ=name PKG=path TIME=30s)"
	@echo ""
	@echo "  Chaos:"
	@echo "    ze-chaos-test              Unit + functional + integration + web"
	@echo "    ze-chaos-verify            Lint + all chaos tests"
	@echo "    ze-chaos-functional-test   In-process BGP chaos simulation"
	@echo "                               Options: CHAOS_SEED=N CHAOS_DURATION=60s CHAOS_PEERS=8"
	@echo "    ze-chaos-integration-test  End-to-end: Ze + chaos peers (.ci tests)"
	@echo ""
	@echo "  ExaBGP compatibility:"
	@echo "    ze-exabgp-test            Ze encoding matches ExaBGP"
	@echo ""
	@echo "  Interop (Docker):"
	@echo "    ze-interop-test           FRR/BIRD interop (INTEROP_SCENARIO=name for one)"
	@echo "    ze-ipsec-interop-test     strongSwan IKEv2 (IPSEC_INTEROP_SCENARIO=name)"
	@echo ""
	@echo "  Stress (Linux, root, netns):"
	@echo "    ze-stress-test            BGP stress with ze-test peer injector"
	@echo "    ze-stress-bird-test       BIRD baseline comparison"
	@echo "    ze-stress-profile         1M route profiling (saves pprof to tmp/)"
	@echo ""
	@echo "  Integration (CAP_NET_ADMIN / root):"
	@echo "    ze-integration-test       All netns integration tests"
	@echo "    ze-integration-iface-test     iface netlink"
	@echo "    ze-integration-fib-test       FIB kernel routes"
	@echo "    ze-integration-firewall-test  nftables"
	@echo "    ze-integration-traffic-test   tc qdisc"
	@echo ""
	@echo "  QEMU (macOS-friendly, no Docker Desktop kernel limits):"
	@echo "    ze-qemu-integration-test  Integration tests in QEMU Alpine VM"
	@echo "    ze-qemu-l2tp-ppp-test     L2TP PPP/NCP in QEMU with gokrazy kernel"
	@echo "    ze-qemu-ldp-frr-test      LDP interop against FRR ldpd in QEMU"
	@echo ""
	@echo "  Live (Docker + internet):"
	@echo "    ze-live-test              All live tests"
	@echo "    ze-live-rpki-test         RPKI with real-world data (stayrtr)"
	@echo ""
	@echo "  Linux-only Go tests:"
	@echo "    ze-linux-test             Run in Docker (ZE_LINUX_TEST_PACKAGES=...)"
	@echo ""
	@echo "  Performance benchmarks (Docker):"
	@echo "    ze-perf-bench             Run against all DUTs (PERF_DUT=name for one)"
	@echo "    ze-perf-report            Generate comparison report"
	@echo "    ze-perf-track             Update history tracking"
	@echo ""
	@echo "  Composite targets:"
	@echo "    ze-smoke                  Lint + unit + build (~2 min)"
	@echo "    ze-verify                 Pre-commit gate: lint + unit (two-pass) + functional + exabgp"
	@echo "    ze-verify-changed         Scoped: changed packages only + functional + exabgp"
	@echo "    ze-test                   All ze tests including fuzz"
	@echo "    ze-all                    ze-verify + chaos-verify"
	@echo "    ze-all-test               ze-test + chaos-verify"
	@echo ""
	@echo "  Release evidence (external infra required):"
	@echo "    ze-release-evidence-preflight  Check required tooling (Docker, QEMU)"
	@echo "    ze-release-evidence            Full matrix: interop + chaos + fuzz + perf + QEMU + deploy"
	@echo "    ze-perf-gate                   Perf bench (ze DUT) + regression check"
	@echo ""
	@echo "  Escalation: single test -> package -> component group -> ze-verify"
	@echo "  See docs/contributing/testing.md for the full workflow."

help-deploy:
	@echo "Ze Deployment Targets"
	@echo ""
	@echo "  Docker:"
	@echo "    ze-docker                Build Docker image (ZE_DOCKER_IMAGE=ze ZE_DOCKER_TAG=...)"
	@echo ""
	@echo "  Gokrazy VM appliance (see docs/guide/appliance.md):"
	@echo "    ze-gokrazy-deps              One-time: download gokrazy system packages"
	@echo "    ze-gokrazy USER=x PASS=y     Build bootable VM image"
	@echo "    ze-gokrazy-run               Boot in QEMU (Ctrl-A X to quit)"
	@echo "                                 GOKRAZY_ARCH=arm64 for Apple Silicon"
	@echo ""
	@echo "  Custom kernel:"
	@echo "    ze-kernel KVER=7.0           Build kernel with L2TP/PPP support (~5 min)"
	@echo "    ze-kernel-clean              Restore pinned rtr7/kernel"
	@echo ""
	@echo "  Deployment evidence:"
	@echo "    ze-deployment-vpp-test            Real VPP daemon in Docker"
	@echo "    ze-deployment-l2tp-test           External L2TP peer in Docker"
	@echo "    ze-deployment-l2tp-ppp-test       Full PPP/NCP in Linux netns"
	@echo "    ze-deployment-l2tp-ppp-docker-test  PPP/NCP Docker lab (Ze LNS + LAC + FRR)"
	@echo "    ze-deployment-gokrazy-l2tp-ppp-test  PPP against QEMU appliance"
	@echo "    ze-docker-evidence EVIDENCE_SCRIPT=... EVIDENCE_PACKAGES=..."
	@echo "    ze-deployment-preflight      Check deployment tooling availability"
	@echo "    ze-release-check             Clean release-candidate verification in Docker"

help-dev:
	@echo "Ze Development Tools"
	@echo ""
	@echo "  Inventory:"
	@echo "    ze-inventory             Plugins, YANG, RPCs, tests, packages"
	@echo "    ze-inventory-json        Same as JSON"
	@echo "    ze-command-list          All registered commands by verb"
	@echo "    ze-command-list-json     Same as JSON"
	@echo ""
	@echo "  Documentation validation:"
	@echo "    ze-doc-test              All doc checks (drift + YANG/handler contract)"
	@echo "    ze-doc-drift             Docs claims vs live registry/Makefile/filesystem"
	@echo "    ze-doc-index             Regenerate ai/CODE-TO-DOCS.md (code->docs reverse index)"
	@echo "    ze-validate-commands     YANG command tree vs registered handlers"
	@echo "    ze-consistency           Code/doc consistency: design refs, cross-refs, stale refs"
	@echo ""
	@echo "  Spec management:"
	@echo "    ze-spec-status           Spec inventory with progress status"
	@echo "    ze-spec-status-json      Same as JSON"
	@echo "    ze-learned-counter       Rebuild plan/learned/.counter (recovery)"
	@echo ""
	@echo "  Code:"
	@echo "    fmt                      Format code (gofmt + goimports)"
	@echo "    vet                      Run go vet"
	@echo "    tidy                     Tidy go.mod"
	@echo "    check                    Quick check (fmt + vet)"
	@echo ""
	@echo "  Generated files:"
	@echo "    ze-regen                 Regenerate all generated files"
	@echo "    ze-regen-check           Verify generated files are up to date"
	@echo "    ze-ai-instructions       Generate CLAUDE.md and AGENTS.md"
	@echo "    ze-ai-sync               Sync canonical skills to tool directories"
	@echo "    ze-doc-index             Regenerate ai/CODE-TO-DOCS.md"
	@echo "    ze-sync-vendor-web       Sync vendored web assets"
	@echo "    ze-check-vendor-web      Check for newer web asset versions"
	@echo ""
	@echo "  Cleanup:"
	@echo "    clean                    Remove bin/ and tmp/"
	@echo "    ze-clean-tmp             Remove tmp/ scratch files older than 24h"
