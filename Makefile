.PHONY: all build ze ze-appliance ze-setup-bin chaos test analyse clean clean-all fmt vet tidy generate help
.PHONY: ze-docker
.PHONY: ze-lint ze-vet-evidence ze-race-reactor ze-linux-test ze-exabgp-test ze-vulncheck
.PHONY: ze-test ze-verify ze-verify-changed ze-verify-list ze-validate ze-validate-tree ze-smoke ze-ci ze-all ze-all-test
.PHONY: ze-lint-changed ze-unit-test-changed ze-clean-tmp ze-hook-test
.PHONY: ze-tier-check ze-iface-resolution-check ze-plugin-boundary-check ze-config-coercion-check ze-fs-persistence-check ze-dash-stdio-check ze-port-defaults-check ze-yang-leaf-mentions ze-platform-vet ze-ci-dispatch-check
.PHONY: ze-test-sensitivity-check ze-test-health ze-test-health-check ze-test-health-record
.PHONY: ze-tracked-build-check
.PHONY: ze-iso ze-iso-init ze-iso-build ze-iso-check ze-pxe
.PHONY: ze-sync-vendor-web ze-check-vendor-web ze-ai-sync ze-ai-instructions
.PHONY: ze-plugin-imports-check ze-fuzz-targets-check ze-yang-glue-check ze-feature-tags-check ze-regen ze-regen-check ze-regen-check-readonly ze-arch-map ze-arch-map-check
.PHONY: check ze-setup
.PHONY: help-test help-deploy help-dev

# Environment: keep build caches within CURDIR (not TMPDIR - breaks Unix socket tests)
# GOCACHE is on the durable side (cache/ -> ~/.cache/ze), not disposable tmp/, so it
# survives a scratch wipe. Still inside CURDIR, so the socket-path note above holds.
export GOCACHE := $(CURDIR)/cache/go-cache
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

# Where Homebrew is, asked of the machine rather than written down. It is under
# /opt/homebrew on Apple Silicon and /usr/local on Intel, and `brew` itself sits
# at <prefix>/bin/brew, so its own location answers for both. Empty on a host
# with no Homebrew, which is every Linux one, so each user below guards on it.
# The same resolution in Go is brewPrefixes (internal/appliance/homebrew.go) and
# in Python brew_prefixes (scripts/evidence/homebrew.py). They are held together
# by scripts/dev/homebrew_prefix_test.py.
BREW_BIN      := $(shell command -v brew 2>/dev/null)
BREW_FROM_BIN := $(if $(BREW_BIN),$(patsubst %/,%,$(dir $(patsubst %/,%,$(dir $(BREW_BIN))))))
# The defaults on macOS only: /usr/local exists on every Linux box and is not
# Homebrew's there. Same rule as brewPrefixes and brew_prefixes.
BREW_DEFAULTS := $(if $(filter Darwin,$(shell uname -s)),/opt/homebrew /usr/local)
# `wildcard` drops what does not exist. Without the defaults rung, a make run
# whose PATH has no brew (sudo resets PATH, and so does launchd) left the prefix
# empty and E2FS lost its whole Homebrew branch on a working Apple Silicon Mac.
#
# BREW_PREFIXES keeps ALL of them, in order, because that is what the Go and
# Python resolvers return and what searching them all is worth: a stale empty
# /opt/homebrew beside a real install at /usr/local otherwise wins on a single
# `firstword` and hides it. BREW_PREFIX is the first, for the callers that can
# only use one path.
# `filter-out` drops a default the earlier rungs already named. Make has no
# order-preserving uniq, and `sort` would reorder the rungs, which is the one
# thing this list must not do.
BREW_PREFIXES := $(wildcard $(HOMEBREW_PREFIX) $(BREW_FROM_BIN) \
	$(filter-out $(HOMEBREW_PREFIX) $(BREW_FROM_BIN),$(BREW_DEFAULTS)))
BREW_PREFIX   := $(firstword $(BREW_PREFIXES))

# Go compiler: override with GO=tinygo for smaller binaries
# TinyGo finds go via PATH, so we prepend Go 1.26 when GO=tinygo
GO ?= go
ifeq ($(GO),tinygo)
ifneq ($(BREW_PREFIX),)
export PATH := $(BREW_PREFIX)/opt/go@1.26/bin:$(PATH)
endif
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
# there, NOT here. See ai/rules/plugins.md.
ZE_FEATURES := $(shell awk '$$1 ~ /^ze_/ {print $$1}' $(CURDIR)/feature-gates.txt | sort -u)

# Version: YY.MM.DD from current date, injected via ldflags.
ZE_VERSION := $(shell date +%y.%m.%d)
ZE_BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
ZE_LDFLAGS := -X main.version=$(ZE_VERSION) -X main.buildDate=$(ZE_BUILD_DATE)

# CPU limit: leave 3 cores free, but never drop below HALF the cores. Used as
# GOMAXPROCS for tests so parallel stages do not starve the system. Unit tests
# exercise the shipped default-on feature set; GO_TEST_CORE runs bare ze_core
# compile-out checks.
#
# The half-the-cores floor is not cosmetic. "n - 3" was sized for a development
# workstation and DEGENERATES on a small CI runner: GitHub's hosted 4-vCPU
# runner landed on GOMAXPROCS=1, and that is two failures, not one.
#   - `go test -p` defaults to GOMAXPROCS, so all ~450 packages ran ONE AT A
#     TIME (the cached unit stage took 22 minutes).
#   - internal/component/cli/testing's headless model races a command goroutine
#     against 10 runtime.Gosched() yields and falls back to a 900ms wait when
#     the goroutine has not finished (headless.go:152-191). With one P the
#     command cannot run in parallel, so the .et suite paid the 900ms far more
#     often and blew go test's 10-minute package default (CI run 30219943935).
# Floor: n=4 -> 2, n=8 -> 5, n=16 -> 13. Never above n-3 on a big machine.
GO_TEST_PROCS := $(shell n=$$(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4); p=$$(( n - 3 )); h=$$(( (n + 1) / 2 )); [ $$p -lt $$h ] && p=$$h; [ $$p -lt 1 ] && p=1; echo $$p)
# Per-test-binary wall-clock cap, stated rather than inherited. go test's
# implicit default is 10m, which nothing in this repo ever chose: the slowest
# package measured here (internal/component/cli/testing, the .et editor suite)
# takes ~170s on a 16-core host, so 10m was under 4x headroom and a slower
# runner crossed it. This is a HANG catcher, not a wait: no test is expected to
# approach it, and one that does is a defect to fix, not a number to raise.
# $(or …), not ?=: `?=` only skips assignment when the variable is UNDEFINED,
# so an exported-but-empty GO_TEST_TIMEOUT= would expand to `go test -timeout
# -tags …` and every target would die on `invalid value "-tags" for flag
# -timeout`. $(or …) treats empty the same as unset.
GO_TEST_TIMEOUT := $(or $(GO_TEST_TIMEOUT),20m)
# GO_TEST_CORE runs only ./cmd/ze/hub, so every absent-feature compile-out test
# (//go:build !ze_lg / !ze_ssh / !ze_web) MUST live under cmd/ze/hub. A negated
# feature-gate test placed elsewhere would be silently skipped by both passes.
GO_TEST_TAGS = ze_core $(ZE_FEATURES) $(ZE_TAGS)
# Build tools that ENUMERATE a runtime registry (families, commands, YANG
# modules) link internal/component/plugin/all and must see the same feature
# set the shipped binary has, or a gated plugin's registrations are simply
# absent and the tool reports drift against docs that are correct. GO_RUN is
# for those tools; a tool that only reads source text does not need it.
GO_RUN = go run -tags '$(GO_TEST_TAGS)'
GO_TEST_CORE_TAGS = ze_core $(ZE_TAGS)
GO_TEST = GOMAXPROCS=$(GO_TEST_PROCS) go test -timeout $(GO_TEST_TIMEOUT) -tags '$(GO_TEST_TAGS)'
GO_TEST_CORE = GOMAXPROCS=$(GO_TEST_PROCS) go test -timeout $(GO_TEST_TIMEOUT) -tags '$(GO_TEST_CORE_TAGS)'
# `go test -race` links the race runtime through cgo, so the global CGO_ENABLED=0
# (kept so release binaries stay static) has to be overridden on race targets or
# every -race run aborts with "-race requires cgo". Non-race test runs stay
# CGO-free. Use these for any -race invocation instead of `$(GO_TEST) -race`.
GO_TEST_RACE = GOMAXPROCS=$(GO_TEST_PROCS) CGO_ENABLED=1 go test -timeout $(GO_TEST_TIMEOUT) -tags '$(GO_TEST_TAGS)' -race
GO_TEST_CORE_RACE = GOMAXPROCS=$(GO_TEST_PROCS) CGO_ENABLED=1 go test -timeout $(GO_TEST_TIMEOUT) -tags '$(GO_TEST_CORE_TAGS)' -race
ZE_EXABGP_TIMEOUT ?= 180
ZE_LINUX_GO_IMAGE ?= golang:1.26-alpine
ZE_LINUX_TEST_PACKAGES ?= ./internal/plugins/traffic/vpp

# Packages
# The module path, as declared by go.mod. Rename it with
# `python3 scripts/dev/rename_module_path.py --to <new> --apply`.
ZE_MODULE = github.com/ze-software/ze
# Exclude the root module package: it only contains build-tagged tooling imports.
#
# ZE_PACKAGES_EXCLUDE is an extra grep -E pattern a caller may set to drop more
# packages. The QEMU unit phase uses it for ./scripts/... : those are host
# developer-tooling gates with no //go:build linux surface at all, so the VM adds
# no coverage, and several of them assert on the DEVELOPER's environment rather
# than on ze -- dev_setup_test.py checks for brew or apt, and Alpine has neither.
# The rest shell out to gate binaries they compile on the fly, which over the 9p
# mount blows their own 60s timeouts. They cost ~33 minutes of the VM run and
# every failure was environmental. They still run in full under `make ze-verify`
# on the host, so nothing is uncovered by skipping them here.
ZE_PACKAGES_EXCLUDE ?=
ZE_PACKAGES = $$(go list ./... | grep -v '^github.com/ze-software/ze$$'$(if $(ZE_PACKAGES_EXCLUDE), | grep -vE '$(ZE_PACKAGES_EXCLUDE)',))

# Default target
.DEFAULT_GOAL := help

# ─── Include split Makefile modules ─────────────────────────────────────────
# Session-scoped binary names (ZEBIN_*, ZE_BIN_SUFFIX). Must come FIRST: every
# include below refers to the binaries through these variables.
include mk/session.mk
include mk/test-unit.mk
include mk/test-functional.mk
include mk/test-fuzz.mk
include mk/test-chaos.mk
include mk/test-integration.mk
include mk/test-release.mk
include mk/perf.mk
include mk/alloc-gate.mk
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
	@go run scripts/codegen/feature_tags.go
	@python3 scripts/dev/fuzz-targets.py

# Regenerate api/proto/*.pb.go from api/proto/ze.proto. Deliberately NOT part of
# `generate`: it needs protoc on PATH, while the .pb.go files are checked in so a
# normal build never calls it. The two codegen plugins are built from vendor/, so
# their versions are pinned by go.mod; only the `protoc vN` header comment
# reflects the locally installed protoc.
#
# Needed after any change to the module path: the embedded rawDesc carries
# go_package as a LENGTH-PREFIXED field, so a textual rewrite of a
# different-length path compiles and decodes to garbage. rename_module_path.py
# refuses .pb.go for exactly this reason and points here.
ze-proto-gen:
	@command -v protoc >/dev/null || { echo "protoc not found -- install it (brew install protobuf)"; exit 1; }
	@go build -mod=vendor -o bin/protoc-gen-go google.golang.org/protobuf/cmd/protoc-gen-go
	@go build -mod=vendor -o bin/protoc-gen-go-grpc google.golang.org/grpc/cmd/protoc-gen-go-grpc
	@PATH="$(CURDIR)/bin:$$PATH" protoc \
		--go_out=. --go_opt=module=$(ZE_MODULE) \
		--go-grpc_out=. --go-grpc_opt=module=$(ZE_MODULE) \
		api/proto/ze.proto
	@echo "Regenerated api/proto/ze.pb.go api/proto/ze_grpc.pb.go"

ze-plugin-imports-check:
	@go run scripts/codegen/plugin_imports.go --check

ze-fuzz-targets-check:
	@python3 scripts/dev/fuzz-targets.py --check

ze-yang-glue-check:
	@go run scripts/codegen/yang_glue.go --check

ze-feature-tags-check:
	@go run scripts/codegen/feature_tags.go --check

# Regenerate plugin/all registry snapshots (testdata/*.snapshot) from the live
# registry after adding or removing a plugin. ze-unit-test fails with a clear
# "unexpected/missing" message and points here, so the lists never silently
# drift from all.go. Review the diff before committing.
ze-plugin-snapshot:
	@$(GO_TEST) -run 'TestRegisteredPluginNames|TestRegisteredWireMethods|TestYANGSchemaProviders' ./internal/component/plugin/all/ -update
	@echo "Updated internal/component/plugin/all/testdata/*.snapshot"

build: generate $(ZEBIN_ZE) $(ZEBIN_APPLIANCE) $(ZEBIN_SETUP) $(ZEBIN_STRIPPED) $(ZEBIN_TEST) $(ZEBIN_CHAOS) $(ZEBIN_PERF) $(ZEBIN_ANALYZE)
	@echo "All binaries built"

ze:
	@mkdir -p bin
	$(GO) build -tags 'ze_core ze_distro $(ZE_FEATURES) $(ZE_TAGS)' -ldflags "$(ZE_LDFLAGS)" -o $(ZEBIN_ZE) ./cmd/ze

ze-appliance:
	@mkdir -p bin
	$(GO) build -tags 'ze_core ze_appliance $(ZE_FEATURES) $(ZE_TAGS)' -ldflags "$(ZE_LDFLAGS)" -o $(ZEBIN_APPLIANCE) ./cmd/ze

ze-setup-bin:
	@mkdir -p bin
	$(GO) build -tags 'ze_setup $(ZE_TAGS)' -ldflags "$(ZE_LDFLAGS)" -o $(ZEBIN_SETUP) ./cmd/ze

ze-stripped:
	@mkdir -p bin
	$(GO) build -tags 'ze_core ze_ssh $(ZE_TAGS)' -ldflags "$(ZE_LDFLAGS)" -o $(ZEBIN_STRIPPED) ./cmd/ze

# ze-chaos and ze-perf drive an in-process BGP reactor, so they force ze_bgp on:
# their own tags (ze_chaos / ze_perf) do not include ZE_FEATURES, and without it
# the BGP plugins would silently register nothing rather than fail to build.
chaos:
	@mkdir -p bin
	$(GO) build -tags 'ze_chaos ze_bgp' -o $(ZEBIN_CHAOS) ./cmd/ze

test:
	@mkdir -p bin
	$(GO) build -tags 'ze_test $(ZE_FEATURES) $(ZE_TAGS)' -o $(ZEBIN_TEST) ./cmd/ze

analyze:
	@mkdir -p bin
	$(GO) build -tags ze_analyze -o $(ZEBIN_ANALYZE) ./cmd/ze

perf:
	@mkdir -p bin
	$(GO) build -tags 'ze_perf ze_bgp' -o $(ZEBIN_PERF) ./cmd/ze

$(ZEBIN_ZE): $(shell find cmd/ze internal -name '*.go' 2>/dev/null)
	@echo "Building ze..."
	@mkdir -p bin
	$(GO) build -tags 'ze_core ze_distro $(ZE_FEATURES) $(ZE_TAGS)' -ldflags "$(ZE_LDFLAGS)" -o $(ZEBIN_ZE) ./cmd/ze

$(ZEBIN_APPLIANCE): $(shell find cmd/ze internal -name '*.go' 2>/dev/null)
	@echo "Building ze-appliance..."
	@mkdir -p bin
	$(GO) build -tags 'ze_core ze_appliance $(ZE_FEATURES) $(ZE_TAGS)' -ldflags "$(ZE_LDFLAGS)" -o $(ZEBIN_APPLIANCE) ./cmd/ze

$(ZEBIN_SETUP): $(shell find cmd/ze internal -name '*.go' 2>/dev/null)
	@echo "Building ze-setup..."
	@mkdir -p bin
	$(GO) build -tags 'ze_setup $(ZE_TAGS)' -ldflags "$(ZE_LDFLAGS)" -o $(ZEBIN_SETUP) ./cmd/ze

$(ZEBIN_STRIPPED): $(shell find cmd/ze internal -name '*.go' 2>/dev/null)
	@echo "Building ze-stripped..."
	@mkdir -p bin
	$(GO) build -tags 'ze_core ze_ssh $(ZE_TAGS)' -ldflags "$(ZE_LDFLAGS)" -o $(ZEBIN_STRIPPED) ./cmd/ze
$(ZEBIN_TEST): $(shell find cmd/ze internal -name '*.go' 2>/dev/null)
	@echo "Building ze-test..."
	@mkdir -p bin
	$(GO) build -tags 'ze_test $(ZE_FEATURES) $(ZE_TAGS)' -o $(ZEBIN_TEST) ./cmd/ze

$(ZEBIN_CHAOS): $(shell find cmd/ze internal -name '*.go' 2>/dev/null)
	@echo "Building ze-chaos..."
	@mkdir -p bin
	$(GO) build -tags 'ze_chaos ze_bgp' -o $(ZEBIN_CHAOS) ./cmd/ze

$(ZEBIN_ANALYZE): $(shell find cmd/ze internal -name '*.go' 2>/dev/null)
	@echo "Building ze-analyze..."
	@mkdir -p bin
	$(GO) build -tags ze_analyze -o $(ZEBIN_ANALYZE) ./cmd/ze

$(ZEBIN_PERF): $(shell find cmd/ze internal -name '*.go' 2>/dev/null)
	@echo "Building ze-perf..."
	@mkdir -p bin
	$(GO) build -tags 'ze_perf ze_bgp' -o $(ZEBIN_PERF) ./cmd/ze

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

ze-exabgp-test: $(ZEBIN_ZE) $(ZEBIN_TEST)
	@echo "Running ExaBGP compatibility tests..."
	uv run --with paramiko $(ZEBIN_TEST) exabgp --all --timeout $(ZE_EXABGP_TIMEOUT)s

# Software-composition analysis (SCA): govulncheck (golang.org/x/vuln) scans the
# module's dependency graph against the Go vulnerability database (vuln.go.dev)
# and reports only vulnerabilities reachable from ze's own call graph.
#
# Deliberately ON-DEMAND, NOT a stagesForMode entry and NOT wired into
# `make ze-verify`: it needs a network fetch of the vuln DB, and a transient fetch
# failure or a newly published advisory must never wedge the inline pre-commit /
# merge loop (spec-fixit-supply-chain-hardening AC-1, SCHEDULED default). It runs
# on a cron via .github/workflows/govulncheck.yml -- which calls THIS target, the
# single source of truth for the invocation -- and stays runnable by hand.
#
# `@latest` runs the tool from outside the main module, so there is no go.mod /
# vendor churn: this repo vendors (vendor/), and adding x/vuln as a module `tool`
# dependency would force vendoring govulncheck's large analysis tree (x/tools SSA,
# callgraph, ...) into vendor/, a heavy and build-fragile change for a CI-only tool.
ze-vulncheck:
	@echo "Running govulncheck (SCA: module deps vs vuln.go.dev)..."
	$(GO) run golang.org/x/vuln/cmd/govulncheck@latest ./...

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
# See ai/rules/repo-maintenance.md.
ze-hook-test:
	@echo "Hook dispatcher parity (golden exit codes)..."
	@python3 scripts/dev/hook-parity-check.py
	@echo "Hook behavioural fixtures (format-alloc / validate-spec / design-gate / commit-gate)..."
	@python3 scripts/dev/hook-fixture-check.py

# ─── Composite verification targets ────────────────────────────────────────

ze-test: ze-lint ze-unit-test ze-functional-test ze-exabgp-test ze-fuzz-test
	@echo "All ze tests passed"

ZE_VERIFY_LOG ?= tmp/ze-verify.log

# NOTE on `make -n`/-t/-q: the refusal lives in verify_run.go (makeDryRun), which
# is reached only AFTER verify-lock.sh has taken the lock. So a no-execute
# invocation still waits on a concurrent verify and still appends a 0-second row
# to tmp/.ze-verify-duration.txt before refusing. That is deliberate: guarding
# earlier means re-implementing the MAKEFLAGS parse in shell or a make
# conditional, and the parse is subtle (a naive `findstring n,$(MAKEFLAGS)`
# false-positives on --no-print-directory, refusing every real verify). One
# tested implementation beats two, and nothing is forged either way.
ze-verify:
	@scripts/dev/verify-lock.sh ze-verify env ZE_VERIFY_MAKE="$(MAKE)" $(GO) run ./scripts/status/verify_run.go ze-verify

# Print the stage list without running anything.
#   make ze-verify-list
#   make ze-verify-list ZE_VERIFY_MODE=ze-verify-changed
#
# Use THIS, never `make -n ze-verify` (nor -t / -q): the ze-verify recipe above
# contains $(MAKE), so make executes it even in those no-execute modes while
# every stage sub-make does nothing and "passes" -- the runner would then write
# a FRESH tmp/ze-verify.status for a tree nothing actually verified.
# verify_run.go refuses to run under them for that reason.
#
# The variable is ZE_VERIFY_MODE, not MODE: make imports the environment, and
# MODE is a common enough name that a stray `export MODE=...` would silently
# make this print the wrong list.
ze-verify-list:
	@$(GO) run ./scripts/status/verify_run.go --list $(or $(ZE_VERIFY_MODE),ze-verify)

ze-verify-changed:
	@scripts/dev/verify-lock.sh ze-verify-changed env ZE_VERIFY_MAKE="$(MAKE)" $(GO) run ./scripts/status/verify_run.go ze-verify-changed; rc=$$?; python3 scripts/dev/perf-suggest.py || true; exit $$rc

# Module-tier placement gate (ai/rules/architecture.md): a config-driven engine
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
# tags, never hand-written (ai/rules/evidence.md).
# --selftest proves the gate against its own fixtures before it judges the live tree.
#
# Wired into `make ze-verify` via stagesForMode() in scripts/status/verify_run.go (BOTH
# branches) -- that function is the only live stage list; nothing in this Makefile
# enumerates verify stages any more.
ze-rfc-check:
	@python3 scripts/dev/rfc_requirements.py --selftest
	@python3 scripts/dev/rfc_requirements.py --check

# Regenerate ai/RFC-REQUIREMENTS.md (requirement -> enforcing tests).
ze-rfc-index:
	@python3 scripts/dev/rfc_requirements.py --write

# Re-stamp the audit verdicts a mechanical edit staled (plan/spec-rfcgate-3-audit-teeth.md).
# `make ze-rfc-check` reports a verdict as SHIFTED when the tagged unit -- the enclosing
# top-level function of each tagged test -- is byte-identical and only the file around it moved:
# a line shift, a sibling test, an import rewrite. Nothing was re-judged, so no human should be
# asked to re-read; this rewrites the file-level fingerprints and nothing else.
#
# Deliberately its OWN target. It is the only thing that writes rfc/audit/ without a human
# editing it, and folding it into ze-rfc-check (a check that writes cannot be trusted to
# report) or ze-rfc-index (which runs routinely for reasons unrelated to any audit) would
# automate the blind re-stamp reflex the spec exists to remove. Owner ruling 2026-07-29.
#
# A verdict whose unit, cited producer code, or requirement text moved is REFUSED and stays
# stale: that one needs /ze-rfc-audit <rfc>. Run `make ze-rfc-index` afterwards.
ze-rfc-reseal:
	@python3 scripts/dev/rfc_requirements.py --reseal

# Write an UNCLASSIFIED extraction skeleton for one RFC
# (plan/spec-rfcgate-1-extraction.md): every normative site and every section of
# rfc/full/<stem>.txt, with each disposition null. A reviewer then classifies each one by
# hand in rfc/extraction/<stem>.json. Generation alone can never produce a passing
# sign-off -- an unclassified site FAILS `make ze-rfc-check` -- so mass-generating
# skeletons makes the gate redder, never greener.
#   make ze-rfc-extract STEM=rfc7296
ze-rfc-extract:
	@test -n "$(STEM)" || { echo "usage: make ze-rfc-extract STEM=<rfc-stem>"; exit 2; }
	@python3 scripts/dev/rfc_requirements.py --extract-skeleton $(STEM)

# The machine-readable extraction counts the umbrella's drain quota consumes
# (plan/spec-rfcgate-0-umbrella.md, "Where the counter lives"): signed and enrolled
# counts, the per-register split, and the unsigned backlog. Always JSON -- that envelope
# is the mode's only consumer.
ze-rfc-extraction-status:
	@python3 scripts/dev/rfc_requirements.py --extraction-status --json

# No-direct-resolution gate (plan/spec-iface-resolve-0-umbrella.md AC-U1,
# sub-spec 7): interface consumers must resolve logical names via the shared
# iface resolver, not the kernel directly. scripts/checks/iface_resolution.go
# owns the allowlist of legitimate direct-resolution sites.
ze-iface-resolution-check:
	@$(GO) run scripts/checks/iface_resolution.go

# Plugin process-boundary gate (ai/rules/plugins.md): a
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

# Dispatch-command call-site gate: every command string the repo SENDS must
# still resolve. GO_RUN (not GO run): this enumerates the live command registry,
# so it needs the same feature tags the shipped binary has or gated plugins'
# commands are absent and every use of them reports as dead.
ze-ci-dispatch-check:
	@$(GO_RUN) scripts/checks/ci_dispatch_commands.go --selftest
	@$(GO_RUN) scripts/checks/ci_dispatch_commands.go

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

# YANG leaf mention report (spec-improve-7 AC-8). ADVISORY, and deliberately in
# NO verify stage: the signal is a heuristic (a leaf name that appears in no
# string literal of the owning package is PROBABLY never read), so it reports
# and exits 0. The blocking half of that spec is the root-claim gate in
# internal/component/plugin/all/config_claims_test.go, which runs under
# ze-unit-test. --selftest first proves the scan fires on a fixture whose
# answer is known; TestYANGLeafMentionReport (scripts/checks) runs the same.
ze-yang-leaf-mentions:
	@$(GO) run scripts/checks/yang_leaf_mentions.go --selftest
	@$(GO) run scripts/checks/yang_leaf_mentions.go

# Test-sensitivity ratchet (spec-test-health-dashboard AC-10/AC-11): a test that
# cannot fail, and a test file no build tag reaches, both read as coverage while
# providing none. Neither is detectable by any count of tests, which is why the
# published totals grew for years without this. Counts may only go DOWN; the
# floors live in test/health/sensitivity-baseline.json and are lowered in the same
# change that improves the number (the test/.ci-sleep-baseline convention).
# --selftest first proves both AST detectors fire on known-bad fixtures.
ze-test-sensitivity-check:
	@$(GO) run scripts/checks/inert_tests.go --selftest
	@$(GO) run scripts/checks/inert_tests.go --check

# Tracked-build gate (docs/architecture/testing/tracked-build-gate.md): compile
# the tree GIT HOLDS, which is the one population no other check compiles. Every
# other gate builds the working tree, so a consumer committed without its
# producer is green for its author and broken for anybody who builds the commit
# -- four commits broke `make ze` at HEAD that way on 2026-08-04.
#
# --selftest first proves the two vacuity guards still fire: `go build ./...`
# exits 0 over a pattern that matched nothing buildable, so a flavor that
# compiled zero packages would otherwise report success.
#
# REV=<commit-ish> judges another commit (`make ze-tracked-build-check REV=7abe8a07e`).
# The extracted tree is removed at the end; add ARGS=--keep to inspect it.
ze-tracked-build-check:
	@$(GO) run scripts/checks/tracked_build.go --selftest
	@$(GO) run scripts/checks/tracked_build.go $(if $(REV),--rev=$(REV)) $(ARGS)

# Regenerate the testing-state page (docs/features/test-health.md), its structured
# sibling test/health/latest.json, and the ratchet baseline. Output is a pure
# function of committed state -- no wall-clock value -- so ze-test-health-check can
# gate it for staleness the way every other generated file here is gated.
ze-test-health:
	@python3 scripts/dev/testing_health.py --write

# Staleness gate for the above; a prerequisite of ze-regen-check-readonly.
ze-test-health-check:
	@python3 scripts/dev/testing_health.py --check

# Append one KPI row to test/health/history.ndjson. Run after a mutation or verify
# run; the page renders trends from the committed history, never from live output.
ze-test-health-record:
	@python3 scripts/dev/testing_health.py --record

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

# The half of ze-validate that `make ze-verify` runs (stagesForMode,
# scripts/status/verify_run.go). Three of the five checks read the tree:
# check_source_anchor_line_numbers and check_source_anchor_stale_paths walk
# docs/**/*.md, check_spec_ac_completeness walks plan/spec-*.md. They judge the
# same working tree every other verify stage judges.
#
# The other two scope themselves to `git diff HEAD` plus untracked files
# (changed_files in scripts/dev/validate.py). Several sessions share this
# checkout, so that list is mostly OTHER sessions' half-written files, and both
# checks hold their subject to a completeness standard that a file in the middle
# of an edit cannot meet: check_cross_package_wiring wants a cross-package
# caller for every new exported symbol, check_cli_handler_coverage wants a .ci
# test naming every newly registered command. That already happened once with
# the checker run by hand (plan/spec-session-scoped-build-artifacts.md: 10
# pre-existing findings pulled into one session's scope), and inside ze-verify
# it would red a run whose author changed none of it. Same reasoning as
# ze-ste-check, which stays out of ze-doc-test for the same reason (mk/inventory.mk).
#
# Declaring an EMPTY changed set is what selects the tree-wide three: both
# changed-file checks return no findings before reading anything, and the other
# three take --root and are untouched. Run `make ze-validate` to get all five
# over your own tree.
ze-validate-tree:
	@python3 scripts/dev/validate.py --root . --changed-file ''

ze-all: ze-verify ze-chaos-verify
	@echo "All verification passed (ze + chaos)"

ze-all-test: ze-test ze-chaos-verify
	@echo "All tests passed (ze + chaos + fuzz)"

ze-smoke: ze-lint ze-unit-test build
	@echo "Ze smoke check passed"

ze-ci: ze-smoke

# ─── Utilities ──────────────────────────────────────────────────────────────

# gofmt and goimports take FILESYSTEM paths, not the `./...` package pattern, so
# a bare `.` walks into vendor/ and rewrites third-party code -- it churned 79
# vendored files (//go:build tags added beside legacy // +build, doc comments
# reflowed, imports regrouped) purely because the vendored code predates the
# current toolchain's formatting rules. Enumerate our own Go files instead. The
# root tools.go is included, which a list of source directories would miss.
ZE_GO_SRC = find . -name '*.go' \
	-not -path './vendor/*' -not -path './.git/*' -not -path './tmp/*' -print0

fmt:
	@echo "Formatting code..."
	$(ZE_GO_SRC) | xargs -0 -r gofmt -w
	$(ZE_GO_SRC) | xargs -0 -r goimports -w -local $(ZE_MODULE)

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

ze-regen: generate ze-rules-render ze-rules-condensed ze-ai-instructions ze-ai-sync ze-doc-index ze-rules-index ze-discovery-index ze-test-health
	@echo "All generated files updated"

# Write-safe twin of ze-regen-check, and the ONLY one wired into verify
# (stagesForMode in scripts/status/verify_run.go, both branches).
#
# ze-regen-check below cannot go into verify: its `ze-regen` prerequisite
# REWRITES every generated file and only then diffs, so `make ze-verify` would
# leave a dirty working tree whenever anything was stale. This target reaches
# the same verdict without writing: each generator's own --check regenerates in
# memory and diffs.
#
# Composed of PREREQUISITE TARGETS, never re-typed recipes. Three of these
# (ze-yang-glue-check, ze-feature-tags-check, ze-fuzz-targets-check) had zero
# callers before this target existed. Spelling their commands out here would
# have made a fifth copy of "how to check yang glue" -- the same duplication
# this spec deleted from the stage list. TestRegenCheckReadonlyCoversGenerators
# (scripts/status/verify_run_test.go) derives the required set from the
# `generate` / `ze-regen` recipes and fails when a generator gains no check.
#
# Generator -> output -> covering prerequisite:
#   plugin_imports.go -> plugin/all/all.go                   -> ze-plugin-imports-check
#   yang_glue.go      -> yang/*/register.go, embed.go        -> ze-yang-glue-check
#   feature_tags.go   -> .golangci.yml, gokrazy/ze/config.json,
#                        docs/guide/quickstart.md            -> ze-feature-tags-check
#   fuzz-targets.py   -> mk/test-fuzz-targets.mk             -> ze-fuzz-targets-check
#   code_to_docs.py   -> ai/CODE-TO-DOCS.md                  -> ze-doc-check-stale
#   rules_points.py   -> ai/rules/<rule>.md (from ai/rules/points/) -> ze-rules-render-check
#   rules_index.py    -> ai/rules/INDEX.md                   -> ze-rules-index-check
#   rules_condensed.py-> ai/rules/TRIGGERS.md, ai/rules/CORE.md -> ze-rules-condensed-check
#   arch_map.py       -> arch lists in ai/INSTRUCTIONS.md    -> ze-arch-map-check
#   package_map.py    -> ai/PACKAGE-MAP.md                   \
#   docs_to_code.py   -> ai/DOCS-TO-CODE.md                   > ze-discovery-index-check
#
# TWO DELIBERATE EXCLUSIONS, both would break CI or duplicate an earlier stage:
#
#   skill_sync.sh --check (CLAUDE.md, AGENTS.md, .claude|.codex|.agents/skills).
#   Every one of its targets is GITIGNORED (.gitignore), so they do not exist at
#   all in the fresh checkout CI runs `make ze-verify` against, and the check
#   exits 1 there: "No such file or directory" on all three mirror dirs, "stale"
#   on both .md files. Wiring it in would red EVERY CI run.
#   To reproduce, give the test tree its own git root -- skill_sync.sh starts by
#   cd'ing to `git rev-parse --show-toplevel`, so a `git archive HEAD` tree
#   unpacked inside this repo resolves back here and passes, falsely.
#   Nothing committed can drift here (that is what gitignored means), so CI has
#   nothing to catch; the guard belongs where a generated tree exists, and it
#   already lives there as the warn-only .claude/hooks/session-start.sh check.
#   Do not "fix" this by wiring it in.
#
#   check_doc_links.py --md-only, which ze-regen-check's recipe ends with. It
#   checks references, not generated-file staleness, and ze-doc-links runs the
#   FULL check (a strict superset) in the stage slot immediately before this one.
ze-arch-map-check:
	@python3 scripts/dev/arch_map.py --check

ze-regen-check-readonly: ze-plugin-imports-check ze-yang-glue-check ze-feature-tags-check ze-fuzz-targets-check ze-doc-check-stale ze-rules-render-check ze-rules-index-check ze-rules-condensed-check ze-rules-lint ze-arch-map-check ze-discovery-index-check ze-test-health-check
	@echo "All generated files are up to date"

ze-regen-check: ze-regen
	@if ! git diff --quiet -- ai/CODE-TO-DOCS.md ':(glob)ai/rules/*.md' ai/PACKAGE-MAP.md ai/DOCS-TO-CODE.md internal/component/plugin/all/all.go .golangci.yml gokrazy/ze/config.json docs/guide/quickstart.md mk/test-fuzz-targets.mk 2>/dev/null; then \
		echo "ERROR: Generated files are stale. Run 'make ze-regen' and commit the result." >&2; \
		git diff --stat -- ai/CODE-TO-DOCS.md ':(glob)ai/rules/*.md' ai/PACKAGE-MAP.md ai/DOCS-TO-CODE.md internal/component/plugin/all/all.go .golangci.yml gokrazy/ze/config.json docs/guide/quickstart.md mk/test-fuzz-targets.mk; \
		exit 1; \
	fi
	@python3 scripts/dev/code_to_docs.py --check
	@python3 scripts/dev/rules_points.py render --check
	@python3 scripts/dev/rules_index.py --check
	@python3 scripts/dev/rules_condensed.py --check
	@python3 scripts/dev/rules_lint.py --quiet
	@python3 scripts/dev/arch_map.py --check
	@python3 scripts/dev/package_map.py --check
	@python3 scripts/dev/docs_to_code.py --check
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

# go.mod is EXCLUDED from the file reap below: tmp/go.mod is the TRACKED sentinel
# that keeps `go list ./...` out of the caches under tmp/ (SENTINEL in
# scripts/dev/ensure-links.py). Being committed, it is always older than 24h, so
# an unqualified reap silently deletes a tracked file. Keep this exclusion in
# sync with the identical one in .claude/hooks/session-start.sh.
ze-clean-tmp:
	@echo "Cleaning tmp/ scratch files older than 24h..."
	@find tmp/ -maxdepth 1 -type f -not -name go.mod -mmin +1440 -delete 2>/dev/null || true
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
	@echo "    make ze                   Just $(ZEBIN_ZE)"
	@echo "    make ze-stripped          Just $(ZEBIN_STRIPPED)"
	@echo "    make ze-path              Print this session's ze path (never hardcode bin/ze)"
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
	@echo "  Functional tests (.ci suites via $(ZEBIN_TEST)):"
	@echo "    ze-functional-test        All 24 gating suites"
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
	@echo "  $(ZEBIN_ZE) and you can keep building/editing while it runs."
	@echo "    ZE_SUFFIX=<name>          Pin a stable, kept dir (tmp/testbin-<name>/)"
	@echo "    ZE_TEST_CANONICAL=1       Opt out: rebuild $(ZEBIN_ZE) + $(ZEBIN_TEST) in place"
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
	@echo "    ze-netns-test             firewall/policy/ospf/ospfv3 .ci in a per-test netns"
	@echo "    ze-netns-plugin-test      plugin .ci needing CAP_SYSLOG (/dev/kmsg)"
	@echo ""
	@echo "  QEMU (macOS-friendly, no Docker Desktop kernel limits):"
	@echo "    ze-qemu-all-test          FULL suite in QEMU: functional + unit + integration (host-compiled)"
	@echo "    ze-qemu-debug RUN=...      Run specific tests verbosely in QEMU (RUN='bin/ze-test-linux-arm64 bgp parse 91 -v')"
	@echo "    ze-qemu-shell             Persistent QEMU VM for interactive debugging (run in background)"
	@echo "    ze-qemu-integration-test  Integration tests in QEMU Alpine VM"
	@echo "    ze-qemu-l2tp-ppp-test     L2TP PPP/NCP in QEMU with gokrazy kernel"
	@echo "    ze-qemu-pppoe-accel-test  PPPoE client vs accel-ppp AC in QEMU"
	@echo "    ze-qemu-pppoe-test        PPPoE access-concentrator .ci suite in QEMU (netns + runtime kernel)"
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
	@echo "  Test health (would a regression be caught, not how many tests exist):"
	@echo "    ze-test-health            Regenerate docs/features/test-health.md + test/health/"
	@echo "    ze-test-health-check      Fail if a structural fact drifted (in ze-verify)"
	@echo "    ze-test-health-record     Append one KPI sample to the committed history"
	@echo "    ze-test-sensitivity-check Ratchet: tests that cannot fail, files no target runs"
	@echo ""
	@echo "  Composite targets:"
	@echo "    ze-smoke                  Lint + unit + build (~2 min)"
	@echo "    ze-verify                 Pre-commit gate: lint + wiring/docs + unit (two-pass) + functional + exabgp"
	@echo "    ze-verify-changed         Scoped: changed packages + wiring/docs + functional + exabgp"
	@echo "    ze-validate               All five checks: source anchors, wiring, spec completeness (~0.2s)"
	@echo "    ze-validate-tree          The three tree-wide checks of ze-validate; runs inside ze-verify"
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
	@echo "    ze-kernel KERNEL_ARCH=amd64  Build/materialize the runtime kernel (L2TP/PPP built in; ~30 min cold via KERNEL_BUILDER=docker, instant from cache)"
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
	@echo "  Commit integrity:"
	@echo "    ze-tracked-build-check   Compile the tree GIT HOLDS (REV=<sha> for another commit)"
	@echo ""
	@echo "  Spec management:"
	@echo "    ze-spec-status           Spec inventory with progress status"
	@echo "    ze-spec-status-json      Same as JSON"
	@echo ""
	@echo "  Module / protobuf:"
	@echo "    ze-proto-gen             Regenerate api/proto/*.pb.go from ze.proto (needs protoc)"
	@echo "    (rename the module path)  python3 scripts/dev/rename_module_path.py --to <module> --apply"
	@echo ""
	@echo "  Code:"
	@echo "    fmt                      Format code (gofmt + goimports, excludes vendor/)"
	@echo "    vet                      Run go vet"
	@echo "    tidy                     Tidy go.mod"
	@echo "    check                    Quick check (fmt + vet)"
	@echo ""
	@echo "  Supply chain (SCA):"
	@echo "    ze-vulncheck             govulncheck: module deps vs vuln.go.dev (on-demand; scheduled in CI)"
	@echo ""
	@echo "  Generated files:"
	@echo "    ze-regen                 Regenerate all generated files"
	@echo "    ze-regen-check           Verify generated files are up to date (REGENERATES first)"
	@echo "    ze-regen-check-readonly  Same verdict, writes nothing (this is what ze-verify runs)"
	@echo "    ze-verify-list           Print the ze-verify stage list (never use make -n ze-verify)"
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
