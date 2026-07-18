.PHONY: all build ze ze-appliance ze-setup-bin chaos test analyse clean clean-all fmt vet tidy generate help
.PHONY: ze-docker
.PHONY: ze-lint ze-vet-evidence ze-race-reactor ze-linux-test ze-exabgp-test
.PHONY: ze-test ze-verify ze-verify-changed ze-validate ze-smoke ze-ci ze-all ze-all-test
.PHONY: ze-lint-changed ze-unit-test-changed ze-clean-tmp ze-hook-test
.PHONY: _ze-verify-impl _ze-verify-changed-impl ze-tier-check ze-iface-resolution-check ze-plugin-boundary-check ze-config-coercion-check ze-fs-persistence-check ze-dash-stdio-check ze-port-defaults-check ze-platform-vet
.PHONY: ze-iso ze-iso-init ze-iso-build ze-iso-check ze-pxe
.PHONY: ze-sync-vendor-web ze-check-vendor-web ze-ai-sync ze-ai-instructions
.PHONY: ze-plugin-imports-check ze-yang-glue-check ze-regen ze-regen-check
.PHONY: check ze-setup
.PHONY: help-test help-deploy help-dev

# Environment: keep build caches within CURDIR (not TMPDIR - breaks Unix socket tests)
export GOCACHE := $(CURDIR)/tmp/go-cache
export GOLANGCI_LINT_CACHE := $(CURDIR)/tmp/golangci-lint-cache
export CGO_ENABLED := 0

# Ensure the tmp/ and cache/ symlinks point at their out-of-tree targets before any target
# writes scratch. This replaces the old tmp/go.mod nested-module sentinel: `go list ./...`
# skips a directory SYMLINK named tmp/ (verified), so no marker file is needed.
# See scripts/dev/ensure-links.py and plan/spec-relocate-scratch-and-cache.md.
.PHONY: ze-ensure-links ze-migrate-scratch
ze-ensure-links:
	@python3 scripts/dev/ensure-links.py --quiet

ze-migrate-scratch:
	@python3 scripts/dev/ensure-links.py --migrate

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

# Default-on per-feature compile-out tags. A service guarded by //go:build
# ze_<feature> is compiled into ze / ze-appliance iff its tag is listed here.
# ze-stripped keeps only ze_ssh (the base operator management plane) and drops
# the rest; a fully hardened build with no ssh either is `go build -tags ze_core`
# (bare) -- the no-ssh path proven by TestBuildTag_SSH_Absent + a go-tool-nm
# symbol check.
#
# DERIVED from feature-gates.txt -- the single source of truth shared with the
# generator, dep_audit.py, and the test runner. Add a gate by adding one line
# there, NOT here. See ai/rules/feature-gate-registration.md.
ZE_FEATURES := $(shell awk '$$1 ~ /^ze_/ {print $$1}' $(CURDIR)/feature-gates.txt | sort -u)

# Version: YY.MM.DD from current date, injected via ldflags.
ZE_VERSION := $(shell date +%y.%m.%d)
ZE_BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
ZE_LDFLAGS := -X main.version=$(ZE_VERSION) -X main.buildDate=$(ZE_BUILD_DATE)

# CPU limit: leave 3 cores free (minimum 1). Used as GOMAXPROCS for tests so
# parallel stages do not starve the system. Unit tests exercise the shipped
# default-on feature set; GO_TEST_CORE runs bare ze_core compile-out checks.
GO_TEST_PROCS := $(shell n=$$(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4); p=$$(( n - 3 )); [ $$p -lt 1 ] && p=1; echo $$p)
# GO_TEST_CORE runs only ./cmd/ze/hub, so every absent-feature compile-out test
# (//go:build !ze_lg / !ze_ssh / !ze_web) MUST live under cmd/ze/hub. A negated
# feature-gate test placed elsewhere would be silently skipped by both passes.
GO_TEST_TAGS = ze_core $(ZE_FEATURES) $(ZE_TAGS)
GO_TEST_CORE_TAGS = ze_core $(ZE_TAGS)
GO_TEST = GOMAXPROCS=$(GO_TEST_PROCS) go test -tags '$(GO_TEST_TAGS)'
GO_TEST_CORE = GOMAXPROCS=$(GO_TEST_PROCS) go test -tags '$(GO_TEST_CORE_TAGS)'
# `go test -race` links the race runtime through cgo, so the global CGO_ENABLED=0
# (kept so release binaries stay static) has to be overridden on race targets or
# every -race run aborts with "-race requires cgo". Non-race test runs stay
# CGO-free. Use these for any -race invocation instead of `$(GO_TEST) -race`.
GO_TEST_RACE = GOMAXPROCS=$(GO_TEST_PROCS) CGO_ENABLED=1 go test -tags '$(GO_TEST_TAGS)' -race
GO_TEST_CORE_RACE = GOMAXPROCS=$(GO_TEST_PROCS) CGO_ENABLED=1 go test -tags '$(GO_TEST_CORE_TAGS)' -race
ZE_EXABGP_TIMEOUT ?= 180
ZE_LINUX_GO_IMAGE ?= golang:1.26-alpine
ZE_LINUX_TEST_PACKAGES ?= ./internal/plugins/traffic/vpp

# Packages
# Exclude the root module package: it only contains build-tagged tooling imports.
ZE_PACKAGES = $$(go list ./... | grep -v '^codeberg.org/thomas-mangin/ze$$')

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
include mk/test-mutation.mk
include mk/appliance.mk

include mk/terminal-demo.mk
# ─── Build ──────────────────────────────────────────────────────────────────

all: ze-lint ze-unit-test build

generate:
	@go run scripts/codegen/yang_glue.go
	@go run scripts/codegen/plugin_imports.go

ze-plugin-imports-check:
	@go run scripts/codegen/plugin_imports.go --check

ze-yang-glue-check:
	@go run scripts/codegen/yang_glue.go --check

# Regenerate plugin/all registry snapshots (testdata/*.snapshot) from the live
# registry after adding or removing a plugin. ze-unit-test fails with a clear
# "unexpected/missing" message and points here, so the lists never silently
# drift from all.go. Review the diff before committing.
ze-plugin-snapshot:
	@$(GO_TEST) -run 'TestRegisteredPluginNames|TestRegisteredWireMethods|TestYANGSchemaProviders' ./internal/component/plugin/all/ -update
	@echo "Updated internal/component/plugin/all/testdata/*.snapshot"

build: generate bin/ze bin/ze-appliance bin/ze-setup bin/ze-stripped bin/ze-test bin/ze-chaos bin/ze-perf bin/ze-analyze
	@echo "All binaries built"

ze:
	@mkdir -p bin
	$(GO) build -tags 'ze_core ze_distro $(ZE_FEATURES) $(ZE_TAGS)' -ldflags "$(ZE_LDFLAGS)" -o bin/ze ./cmd/ze

ze-appliance:
	@mkdir -p bin
	$(GO) build -tags 'ze_core ze_appliance $(ZE_FEATURES) $(ZE_TAGS)' -ldflags "$(ZE_LDFLAGS)" -o bin/ze-appliance ./cmd/ze

ze-setup-bin:
	@mkdir -p bin
	$(GO) build -tags 'ze_setup $(ZE_TAGS)' -ldflags "$(ZE_LDFLAGS)" -o bin/ze-setup ./cmd/ze

ze-stripped:
	@mkdir -p bin
	$(GO) build -tags 'ze_core ze_ssh $(ZE_TAGS)' -ldflags "$(ZE_LDFLAGS)" -o bin/ze-stripped ./cmd/ze

chaos:
	@mkdir -p bin
	$(GO) build -tags ze_chaos -o bin/ze-chaos ./cmd/ze

test:
	@mkdir -p bin
	$(GO) build -tags ze_test -o bin/ze-test ./cmd/ze

analyze:
	@mkdir -p bin
	$(GO) build -tags ze_analyze -o bin/ze-analyze ./cmd/ze

perf:
	@mkdir -p bin
	$(GO) build -tags ze_perf -o bin/ze-perf ./cmd/ze

bin/ze: $(shell find cmd/ze internal -name '*.go' 2>/dev/null)
	@echo "Building ze..."
	@mkdir -p bin
	$(GO) build -tags 'ze_core ze_distro $(ZE_FEATURES) $(ZE_TAGS)' -ldflags "$(ZE_LDFLAGS)" -o bin/ze ./cmd/ze

bin/ze-appliance: $(shell find cmd/ze internal -name '*.go' 2>/dev/null)
	@echo "Building ze-appliance..."
	@mkdir -p bin
	$(GO) build -tags 'ze_core ze_appliance $(ZE_FEATURES) $(ZE_TAGS)' -ldflags "$(ZE_LDFLAGS)" -o bin/ze-appliance ./cmd/ze

bin/ze-setup: $(shell find cmd/ze internal -name '*.go' 2>/dev/null)
	@echo "Building ze-setup..."
	@mkdir -p bin
	$(GO) build -tags 'ze_setup $(ZE_TAGS)' -ldflags "$(ZE_LDFLAGS)" -o bin/ze-setup ./cmd/ze

bin/ze-stripped: $(shell find cmd/ze internal -name '*.go' 2>/dev/null)
	@echo "Building ze-stripped..."
	@mkdir -p bin
	$(GO) build -tags 'ze_core ze_ssh $(ZE_TAGS)' -ldflags "$(ZE_LDFLAGS)" -o bin/ze-stripped ./cmd/ze
bin/ze-test: $(shell find cmd/ze internal -name '*.go' 2>/dev/null)
	@echo "Building ze-test..."
	@mkdir -p bin
	$(GO) build -tags ze_test -o bin/ze-test ./cmd/ze

bin/ze-chaos: $(shell find cmd/ze internal -name '*.go' 2>/dev/null)
	@echo "Building ze-chaos..."
	@mkdir -p bin
	$(GO) build -tags ze_chaos -o bin/ze-chaos ./cmd/ze

bin/ze-analyze: $(shell find cmd/ze internal -name '*.go' 2>/dev/null)
	@echo "Building ze-analyze..."
	@mkdir -p bin
	$(GO) build -tags ze_analyze -o bin/ze-analyze ./cmd/ze

bin/ze-perf: $(shell find cmd/ze internal -name '*.go' 2>/dev/null)
	@echo "Building ze-perf..."
	@mkdir -p bin
	$(GO) build -tags ze_perf -o bin/ze-perf ./cmd/ze

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
	@golangci-lint run ./cmd/ze/... ./internal/... ./pkg/... ./test/...

ze-vet-evidence:
	@echo "Vetting evidence scripts (GOOS=linux)..."
	@GOOS=linux go vet ./scripts/evidence/...

ze-race-reactor:
	@echo "Stress race-test reactor (count=20)..."
	$(GO_TEST_RACE) -count=20 ./internal/component/bgp/reactor/...

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

ze-exabgp-test: bin/ze bin/ze-test
	@echo "Running ExaBGP compatibility tests..."
	uv run --with paramiko bin/ze-test exabgp --all --timeout $(ZE_EXABGP_TIMEOUT)s

# ─── Scoped targets (parallel-safe) ────────────────────────────────────────

# Changed-package set = uncommitted .go changes + packages committed since
# the last green verify (scripts/dev/changed-pkgs.sh). The committed-since
# term closes a gap where a regression committed before verifying left the
# working-tree diff and was silently skipped by scoped verify.
ze-lint-changed:
	@pkgs=$$(scripts/dev/changed-pkgs.sh); \
	if [ -z "$$pkgs" ]; then echo "No changed Go packages to lint"; exit 0; fi; \
	echo "Linting changed packages: $$pkgs"; \
	golangci-lint run $$pkgs

ze-unit-test-changed: ze-ensure-links
	@pkgs=$$(scripts/dev/changed-pkgs.sh); \
	if [ -z "$$pkgs" ]; then echo "No changed Go packages to test"; exit 0; fi; \
	echo "Testing changed packages: $$pkgs"; \
	$(GO_TEST_RACE) $$pkgs
	@echo "Unit tests: bare ze_core compile-out checks..."
	$(GO_TEST_CORE_RACE) ./cmd/ze/hub

# ─── Agent-guard hook tests ────────────────────────────────────────────────

# Regression + behavioural tests for the Claude agent-guard hooks. parity-check
# locks the consolidated dispatchers' exit codes against their golden table;
# fixture-check drives the hooks whose behaviour the golden table cannot isolate
# (c_format_alloc, validate-spec.sh, the commit_helper.py commit-time gates, and
# session-id agreement between lib/session-id.sh and pretool-writeedit.py -- the
# shell WRITES the markers Python READS, and a mismatch fails CLOSED).
# See ai/rules/hook-mapping.md.
ze-hook-test:
	@echo "Hook dispatcher parity (golden exit codes)..."
	@python3 scripts/dev/hook-parity-check.py
	@echo "Hook behavioural fixtures (format-alloc / validate-spec / commit-gate)..."
	@python3 scripts/dev/hook-fixture-check.py

# ─── Composite verification targets ────────────────────────────────────────

ze-test: ze-lint ze-unit-test ze-functional-test ze-exabgp-test ze-fuzz-test
	@echo "All ze tests passed"

ZE_VERIFY_LOG ?= tmp/ze-verify.log

ze-verify:
	@scripts/dev/verify-lock.sh ze-verify env ZE_VERIFY_MAKE="$(MAKE)" $(GO) run ./scripts/status/verify_run.go ze-verify

# NOT the live verify path: `ze-verify` above runs scripts/status/verify_run.go,
# which has its OWN hardcoded stage list in stagesForMode() -- these two _impl
# targets have zero callers anywhere in the repo (confirmed via grep across
# Makefile, mk/*.mk, scripts/, .woodpecker/) and silently drifted out of sync
# with stagesForMode for an unknown period before a /ze-review pass caught it
# (plan/learned/1045-plugin-process-boundary.md). A new gate added HERE will
# NOT run under `make ze-verify`/`ze-verify-changed` or CI -- add it to
# scripts/status/verify_run.go's stagesForMode() instead, in BOTH branches.
_ze-verify-impl: ze-lint ze-tier-check ze-iface-resolution-check ze-plugin-boundary-check ze-config-coercion-check ze-fs-persistence-check ze-dash-stdio-check ze-port-defaults-check ze-platform-vet ze-cli-grammar-check ze-verify-wiring-docs ze-vet-evidence ze-unit-test-cached ze-unit-test-race-changed ze-functional-test ze-exabgp-test
	@echo "Ze verification passed"

ze-verify-changed:
	@scripts/dev/verify-lock.sh ze-verify-changed env ZE_VERIFY_MAKE="$(MAKE)" $(GO) run ./scripts/status/verify_run.go ze-verify-changed

# See the _ze-verify-impl comment above: not the live path either.
_ze-verify-changed-impl: ze-lint-changed ze-tier-check ze-iface-resolution-check ze-plugin-boundary-check ze-config-coercion-check ze-fs-persistence-check ze-dash-stdio-check ze-port-defaults-check ze-platform-vet ze-cli-grammar-check ze-verify-wiring-docs ze-unit-test-changed ze-functional-test ze-exabgp-test
	@echo "Ze verification (changed) passed"

# Module-tier placement gate (ai/rules/module-tiers.md): a config-driven engine
# must live in internal/component/ if a feature depends on it, else internal/plugins/.
# --selftest runs the gate's own isolated fixtures (engine placement + the B-1
# wired-vs-core classification) before checking the live tree.
ze-tier-check:
	@python3 scripts/dev/dep_audit.py --selftest
	@python3 scripts/dev/dep_audit.py --check
	@python3 scripts/dev/protocol_skeleton_report.py

# RFC requirement coverage gate (plan/spec-rfc-requirement-coverage.md): every MUST-level
# requirement of an ENROLLED RFC (rfc/enrolled.txt) must be bound to a positive AND a
# negative test via an `RFC requirement: <ID> <polarity>` tag, or carry a reasoned
# annotation. Requirement text lives in rfc/short/*.md; the test links are derived from the
# tags, never hand-written (ai/rules/derive-not-hardcode.md).
# --selftest proves the gate against its own fixtures before it judges the live tree.
#
# Wired into `make ze-verify` via stagesForMode() in scripts/status/verify_run.go (BOTH
# branches) -- NOT _ze-verify-impl, which has zero callers (see the warning above ze-verify).
ze-rfc-check:
	@python3 scripts/dev/rfc_requirements.py --selftest
	@python3 scripts/dev/rfc_requirements.py --check

# Regenerate ai/RFC-REQUIREMENTS.md (requirement -> enforcing tests).
ze-rfc-index:
	@python3 scripts/dev/rfc_requirements.py --write

# No-direct-resolution gate (plan/spec-iface-resolve-0-umbrella.md AC-U1,
# sub-spec 7): interface consumers must resolve logical names via the shared
# iface resolver, not the kernel directly. scripts/checks/iface_resolution.go
# owns the allowlist of legitimate direct-resolution sites.
ze-iface-resolution-check:
	@$(GO) run scripts/checks/iface_resolution.go

# Plugin process-boundary gate (ai/rules/plugin-process-boundary.md): a
# plugin calling another in-process package's same-process-effect function
# directly (bypassing DirectBridge/DispatchCommand) must guard with
# IsInternal()/warnIfExternal() so it does not silently no-op when run as an
# external subprocess. scripts/checks/plugin_process_boundary.go owns the
# dangerous-pattern list and allowlist.
ze-plugin-boundary-check:
	@$(GO) run scripts/checks/plugin_process_boundary.go --selftest
	@$(GO) run scripts/checks/plugin_process_boundary.go

# Config value-coercion gate: the framework delivers YANG leaf values as JSON
# strings, so a config.go coercing them with a native-type assertion (v.(bool),
# a numeric/bool type switch with no `case string:` arm) silently ignores the
# value and reverts to the default (a bool `enabled` gate disables the feature --
# how ddos-detect never fired). scripts/checks/config_string_coercion.go owns it.
ze-config-coercion-check:
	@$(GO) run scripts/checks/config_string_coercion.go --selftest
	@$(GO) run scripts/checks/config_string_coercion.go

ze-fs-persistence-check:
	@$(GO) run scripts/checks/direct_fs_persistence.go --selftest
	@$(GO) run scripts/checks/direct_fs_persistence.go

# CLI "-" = stdin/stdout gate: a filename-accepting command must read/write a
# user-supplied path through internal/core/cliio (so "-" works), never a raw os
# call. --selftest first proves the AST taint detector fires on the pre-migration
# shapes; then the live scan asserts the tree is clean.
ze-dash-stdio-check:
	@$(GO) run scripts/checks/cli_dash_stdio.go --selftest
	@$(GO) run scripts/checks/cli_dash_stdio.go

# Listener port-default gate (spec-followup-subsystem AC-11): the hand-maintained
# Go table (internal/component/config/listener_defaults.go) must match each
# service's YANG `refine port { default N }`, since the YANG compiler does not
# propagate refine defaults. scripts/checks/port_defaults.go owns the mapping.
ze-port-defaults-check:
	@$(GO) run scripts/checks/port_defaults.go --selftest
	@$(GO) run scripts/checks/port_defaults.go

# Cross-platform vet gate (spec-followup-subsystem AC-7): the interface plugins
# ship non-Linux stubs (default_other.go, backend_other.go, host/platform_other.go)
# so the tree builds under GOOS=darwin and GOOS=freebsd. Nothing in the default
# host-GOOS build exercises those files, so a stub that stops compiling on a
# non-Linux platform (e.g. an int64-vs-uint64 syscall.Rlimit drift) rots
# silently. This gate vets the iface + host trees under both non-Linux targets.
ze-platform-vet:
	@echo "Vetting iface/host trees (GOOS=darwin)..."
	@GOOS=darwin go vet ./internal/component/host/... ./internal/component/iface/... ./internal/plugins/iface/...
	@echo "Vetting iface/host trees (GOOS=freebsd)..."
	@GOOS=freebsd go vet ./internal/component/host/... ./internal/component/iface/... ./internal/plugins/iface/...

ze-validate:
	@python3 scripts/dev/validate.py --root .

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

ze-arch-map:
	@python3 scripts/dev/arch_map.py

ze-ai-instructions: ze-arch-map
	@sed 's/{{TOOL}}/Claude/' ai/INSTRUCTIONS.md > CLAUDE.md
	@sed 's/{{TOOL}}/Codex/' ai/INSTRUCTIONS.md > AGENTS.md

ze-ai-sync:
	@scripts/dev/skill_sync.sh

# CLAUDE.md / AGENTS.md / skills mirrors are gitignored: git diff can NEVER
# show drift for them. ze-ai-check compares content against a fresh
# generation instead. The session-start hook runs it and warns when stale.
ze-ai-check:
	@scripts/dev/skill_sync.sh --check

ze-regen: generate ze-ai-instructions ze-ai-sync ze-doc-index ze-rules-index ze-discovery-index
	@echo "All generated files updated"

ze-regen-check: ze-regen
	@if ! git diff --quiet -- ai/CODE-TO-DOCS.md ai/rules/INDEX.md ai/PACKAGE-MAP.md ai/DOCS-TO-CODE.md ai/LEARNED-FULL-INDEX.md internal/component/plugin/all/all.go 2>/dev/null; then \
		echo "ERROR: Generated files are stale. Run 'make ze-regen' and commit the result." >&2; \
		git diff --stat -- ai/CODE-TO-DOCS.md ai/rules/INDEX.md ai/PACKAGE-MAP.md ai/DOCS-TO-CODE.md ai/LEARNED-FULL-INDEX.md internal/component/plugin/all/all.go; \
		exit 1; \
	fi
	@python3 scripts/dev/code_to_docs.py --check
	@python3 scripts/dev/rules_index.py --check
	@python3 scripts/dev/arch_map.py --check
	@python3 scripts/dev/package_map.py --check
	@python3 scripts/dev/docs_to_code.py --check
	@python3 scripts/dev/learned_index.py --check
	@python3 scripts/dev/learned_numbers.py --check
	@scripts/dev/skill_sync.sh --check
	@python3 scripts/dev/check_doc_links.py --md-only
	@echo "All generated files are up to date"

ze-doc-links:
	@python3 scripts/dev/check_doc_links.py

# clean removes bin/, coverage, and THIS session's scratch (tmp/s/<session-id>/, via
# scripts/dev/session-scratch.sh --clean), leaving the shared Go build caches and every
# other concurrent session's scratch/state intact, so a sibling session keeps its work --
# though bin/ and coverage are shared, rebuildable build outputs it does remove. For the
# full per-checkout wipe (bin/ + ALL of tmp/, shared caches
# included) use `make clean-all`. See plan/spec-relocate-scratch-and-cache.md.
clean:
	@echo "Cleaning this session (bin/, coverage, tmp/s/<session>)..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	@scripts/dev/session-scratch.sh --clean 2>/dev/null || true
	@python3 scripts/dev/ensure-links.py --quiet

# clean-all is the full per-checkout wipe: bin/ + the SCRATCH contents (the tmp/ symlink
# target, which includes the shared Go build caches AND every session's state), then
# re-ensures the symlinks. It NEVER touches the durable cache/ (not scratch). Destructive
# under concurrency: it removes sibling sessions' scratch and the shared caches -- use
# `make clean` for the everyday, session-safe clean.
clean-all:
	@echo "Cleaning EVERYTHING (bin/ + all of tmp/, shared caches included)..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	@if [ -e tmp ]; then find tmp/ -mindepth 1 -maxdepth 1 -exec rm -rf {} + 2>/dev/null || true; fi
	@python3 scripts/dev/ensure-links.py --quiet

ze-clean-tmp:
	@echo "Cleaning tmp/ scratch files older than 24h..."
	@find tmp/ -maxdepth 1 -type f -mmin +1440 -delete 2>/dev/null || true
	@find tmp/ -maxdepth 1 -type d -not -name session -not -name kernel \
		-mmin +1440 -exec rm -rf {} + 2>/dev/null || true
	@find tmp/session/ -maxdepth 1 -type f -mmin +1440 -delete 2>/dev/null || true
	@echo "Done. $$(ls -1 tmp/ 2>/dev/null | wc -l | tr -d ' ') entries remain."

check: fmt vet
	@echo "Quick check passed"

# ─── Setup ──────────────────────────────────────────────────────────────────

ze-setup:
	python3 scripts/dev/dev-setup.py $(if $(CHECK),--check)

# ─── Help ───────────────────────────────────────────────────────────────────

help:
	@echo "Ze Network OS -- Build & Test"
	@echo ""
	@echo "  Start here (new contributor):"
	@echo "    make ze-setup             Full dev setup: build deps, linters, appliance tools (one-time)"
	@echo "    make ze-setup CHECK=1     Probe only: list missing tools, exit nonzero if any required"
	@echo "    make ze-smoke             Verify setup: lint + unit + build (~2 min)"
	@echo ""
	@echo "  Daily development:"
	@echo "    make ze-test-bgp          Unit tests for one component group (-race)"
	@echo "                              Also: ze-test-core, ze-test-plugins, ze-test-config, ze-test-cli, ze-test-rest"
	@echo "    make ze-encode-test       Single functional suite (encode, plugin, decode, parse, reload, ...)"
	@echo "    make ze-verify            Pre-commit gate: lint + wiring/docs + unit + functional + exabgp (4-10 min)"
	@echo "    make ze-verify-changed    Scoped verify: changed packages + wiring/docs, then full functional"
	@echo ""
	@echo "  Build:"
	@echo "    make build                All binaries (ze, ze-stripped, ze-test, ze-chaos, ze-perf, ze-analyze)"
	@echo "    make ze                   Just bin/ze"
	@echo "    make ze-stripped          Just bin/ze-stripped"
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
	@echo "    ze-functional-test        All 22 gating suites"
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
	@echo "    ze-ospf-test              OSPF config and doctor"
	@echo "    ---"
	@echo "    ze-static-test            Static routes (release evidence only)"
	@echo "    ze-traffic-test           Traffic control (release evidence only)"
	@echo "    ze-vpp-test               VPP stub (release evidence only)"
	@echo "    ze-l2tp-wire-test         L2TP wire-level (release evidence only)"
	@echo "    ze-isis-wire-test         IS-IS wire-level decode (release evidence only)"
	@echo "    ze-ospf-wire-test         OSPFv2 wire-level decode (release evidence only)"
	@echo ""
	@echo "  Functional tests run against an ISOLATED binary set by default"
	@echo "  (tmp/testbin-<pid>/, removed on exit), so a running suite never touches"
	@echo "  bin/ze and you can keep building/editing while it runs."
	@echo "    ZE_SUFFIX=<name>          Pin a stable, kept dir (tmp/testbin-<name>/)"
	@echo "    ZE_TEST_CANONICAL=1       Opt out: rebuild bin/ze + bin/ze-test in place"
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
	@echo "    ze-exabgp-test            Ze encoding matches ExaBGP (ZE_EXABGP_TIMEOUT=180)"
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
	@echo "    ze-qemu-all-test          FULL suite in QEMU: functional + unit + integration (host-compiled)"
	@echo "    ze-qemu-debug RUN=...      Run specific tests verbosely in QEMU (RUN='bin/ze-test-linux-arm64 bgp parse 91 -v')"
	@echo "    ze-qemu-shell             Persistent QEMU VM for interactive debugging (run in background)"
	@echo "    ze-qemu-integration-test  Integration tests in QEMU Alpine VM"
	@echo "    ze-qemu-l2tp-ppp-test     L2TP PPP/NCP in QEMU with gokrazy kernel"
	@echo "    ze-qemu-pppoe-accel-test  PPPoE client vs accel-ppp AC in QEMU"
	@echo "    ze-qemu-traffic-usage-test  traffic-usage eBPF TCX accounting in QEMU"
	@echo "    ze-qemu-ldp-frr-test      LDP interop against FRR ldpd in QEMU"
	@echo "    ze-qemu-isis-frr-test     IS-IS interop against FRR isisd in QEMU"
	@echo "    ze-qemu-vrrp-keepalived-test  VRRP interop against keepalived in QEMU"
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
	@echo "    ze-verify                 Pre-commit gate: lint + wiring/docs + unit (two-pass) + functional + exabgp"
	@echo "    ze-verify-changed         Scoped: changed packages + wiring/docs + functional + exabgp"
	@echo "    ze-validate               Post-verify: source anchors, wiring, spec completeness (~0.2s)"
	@echo "    ze-test                   All ze tests including fuzz"
	@echo "    ze-all                    ze-verify + chaos-verify"
	@echo "    ze-all-test               ze-test + chaos-verify"
	@echo ""
	@echo "  Release evidence (external infra required):"
	@echo "    ze-release-evidence-preflight  Check required tooling (Docker, QEMU)"
	@echo "    ze-release-evidence            Full matrix: interop + chaos + fuzz + perf + QEMU + deploy"
	@echo "    ze-perf-gate                   Perf bench (ze DUT) + regression check"
	@echo "    ze-release-assets             Rebuild every release-owned website asset"
	@echo "    ze-terminal-demos-release     Re-record all terminal demos for this release"
	@echo "    ze-terminal-demo-tools       Install native VHS, ffmpeg, and ttyd (macOS/Ubuntu)"
	@echo ""
	@echo "  Escalation: single test -> package -> component group -> ze-verify"
	@echo "  See docs/contributing/testing.md for the full workflow."

help-deploy:
	@echo "Ze Deployment Targets"
	@echo ""
	@echo "  Appliance installer ISO (from JSON config):"
	@echo "    ze-iso                   Full build from config: init + kernel + initrd + image + ISO"
	@echo "                             CONFIG=prod.json SSH_PASSWORD='...'"
	@echo "    ze-iso-build             Rebuild (appliance already initialized)"
	@echo "                             NAME=prod APPLIANCE_BUILDER=docker"
	@echo "    ze-iso-check             Check ISO build prerequisites"
	@echo "    ze-pxe                   Build iPXE + TFTP for PXE boot"
	@echo "                             NAME=prod PXE_DIR=build/pxe"
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
	@echo "    ze-deployment-pppoe-accel-docker-test  PPPoE Docker lab (Ze client + accel-ppp AC)"
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
	@echo "  Wiki:"
	@echo "    ze-wiki-update           Regenerate all auto-generated wiki pages"
	@echo "    ze-wiki-commands         Regenerate wiki/command-reference.md"
	@echo ""
	@echo "  Documentation validation:"
	@echo "    ze-doc-test              All doc checks (drift + anchors + YANG/handler contract)"
	@echo "    ze-doc-drift             Docs claims vs live registry/Makefile/filesystem"
	@echo "    ze-doc-index             Regenerate ai/CODE-TO-DOCS.md (code->docs reverse index)"
	@echo "    ze-rules-index           Regenerate ai/rules/INDEX.md (one-line overview of every rule)"
	@echo "    ze-validate-commands     YANG command tree vs registered handlers"
	@echo "    ze-consistency           Code/doc consistency: design refs, cross-refs, stale refs"
	@echo "    ze-verify-wiring-docs     Changed-file-aware wiring, docs, command, and inventory gate"
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
	@echo "    ze-plugin-imports-check   Verify generated plugin blank imports are current"
	@echo "    ze-ai-instructions       Generate CLAUDE.md and AGENTS.md"
	@echo "    ze-ai-sync               Sync canonical skills to tool directories"
	@echo "    ze-ai-check              Check generated agent files match canonical sources"
	@echo "    ze-doc-index             Regenerate ai/CODE-TO-DOCS.md"
	@echo "    ze-rules-index           Regenerate ai/rules/INDEX.md"
	@echo "    ze-sync-vendor-web       Sync vendored web assets"
	@echo "    ze-check-vendor-web      Check for newer web asset versions"
	@echo ""
	@echo "  Cleanup:"
	@echo "    clean                    Session-safe: bin/, coverage, tmp/s/<session> only"
	@echo "    clean-all                Full wipe: bin/ + ALL of tmp/ (shared caches, all sessions)"
	@echo "    ze-clean-tmp             Remove tmp/ scratch files older than 24h"
