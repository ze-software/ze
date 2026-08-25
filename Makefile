.PHONY: all build ze-build ze-appliance-build ze-setup-build ze-stripped-build ze-chaos-build ze-test-build ze-analyze-build clean clean-all fmt vet tidy generate help
.PHONY: ze-docker-build ze-docker-lab-build
.PHONY: ze-lint _ze-lint-impl ze-evidence-vet ze-unit-reactor-test-race ze-unit-linux-test ze-functional-exabgp-test ze-dependency-vulnerability-check
.PHONY: ze-standard-test ze-precommit-verify ze-precommit-verify-changed ze-precommit-verify-list ze-repository-check ze-repository-tree-check ze-smoke-verify ze-ci-verify ze-verify-all ze-test-all
.PHONY: ze-lint-changed ze-unit-test-changed ze-scratch-clean ze-session-clean ze-session-reap ze-unit-hook-test
.PHONY: ze-tier-check ze-iface-resolution-check ze-plugin-boundary-check ze-config-coercion-check ze-fs-persistence-check ze-dash-stdio-check ze-port-defaults-check ze-yang-leaf-mentions-report ze-platform-vet ze-ci-dispatch-check
.PHONY: ze-test-sensitivity-check ze-test-health-update ze-test-health-check ze-test-health-record ze-test-weakened-check
.PHONY: ze-staticcheck-feature-matrix-check ze-repository-tracked-build-check ze-verify-scope-selector ze-verify-debt-clear
.PHONY: ze-iso-build-full ze-iso-initialize ze-iso-build ze-iso-check ze-pxe-build
.PHONY: ze-vendor-web-sync ze-vendor-web-check ze-vendor-web-update-report ze-htmx-upgrade-check ze-htmx-upgrade-report ze-ai-skills-sync ze-ai-instructions-generate ze-ai-sync-check
.PHONY: ze-proto-generate ze-plugin-snapshot-update ze-plugin-imports-check ze-fuzz-targets-check ze-yang-glue-check ze-feature-tags-check ze-web-assets-check ze-templ-orphan-check ze-templ-output-check ze-generated-files-update ze-generated-files-reconcile ze-generated-files-check ze-arch-map-update ze-arch-map-check
.PHONY: ze-web-golden-check ze-web-golden-update ze-templ-port-check ze-chaos-golden-update ze-doc-links-check ze-site-generate
.PHONY: check
.PHONY: help-test help-deploy help-dev
.PHONY: ze-rfc-check ze-rfc-index-update ze-rfc-reseal ze-rfc-extraction-create ze-rfc-extraction-status

# Environment: keep build caches within CURDIR (not TMPDIR - breaks Unix socket tests)
# GOCACHE is on the durable side (cache/ -> ~/.cache/ze), not disposable tmp/, so it
# survives a scratch wipe. Still inside CURDIR, so the socket-path note above holds.
export GOCACHE := $(CURDIR)/cache/go-cache
export GOLANGCI_LINT_CACHE := $(CURDIR)/tmp/golangci-lint-cache
export CGO_ENABLED := 0

# The toolchain every target builds and lints with, READ FROM go.mod rather than
# written here. golangci-lint is a separate binary linked against one version of
# go/types, and it decodes the export data the ambient toolchain writes. When the
# ambient Go is newer than the one the linter was built with, every package fails
# to typecheck with "export data version N is greater than maximum supported
# version M", the linter prints "0 issues" and exits non-zero. Measured 2026-08-22
# with ambient Go 1.27.0 against golangci-lint built with 1.26.6.
#
# A warm GOCACHE hides this: the entries were written by whatever toolchain ran
# first, so the break appears only on a cold cache, which is when nobody is
# looking for a toolchain fault.
#
# Derived, never copied. A literal here and the version in go.mod are two records
# of one fact, and the one nothing compares drifts (plan/journal/stale-spec-claims-done.md).
# A go.mod with no toolchain line yields the empty string and leaves the ambient
# toolchain untouched, which is the pre-2026-08-22 behavior.
ZE_GO_TOOLCHAIN := $(shell awk '$$1 == "toolchain" { print $$2; exit }' $(CURDIR)/go.mod)
ifneq ($(ZE_GO_TOOLCHAIN),)
export GOTOOLCHAIN := $(ZE_GO_TOOLCHAIN)
endif

# Ensure the tmp/ and cache/ symlinks point at their out-of-tree targets before any target
# writes scratch. This replaces the old tmp/go.mod nested-module sentinel: `go list ./...`
# skips a directory SYMLINK named tmp/ (verified), so no marker file is needed.
# See scripts/dev/ensure-links.py and plan/spec-relocate-scratch-and-cache.md.
.PHONY: ze-scratch-links-ensure ze-scratch-migrate
ze-scratch-links-ensure:
	@python3 scripts/dev/ensure-links.py --quiet

ze-scratch-migrate:
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
# the rest; a fully hardened build with no ssh either is `CGO_ENABLED=0 go build -tags ze_core`
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

# CPU limit: NO go process or test run gets more than a QUARTER of the cores.
# Used as GOMAXPROCS for tests. Unit tests exercise the shipped default-on
# feature set; GO_TEST_CORE runs bare ze_core compile-out checks.
#
# Owner ruling, 2026-08-17. The previous sizing was "n - 3, floored at half the
# cores", which hands 29 of 32 cores to ONE run. Several agents share this
# checkout, so the machine was oversubscribed many times over: load average
# reached 79 on 32 cores, a run was OOM-killed, and functional suites began
# failing their wall-clock budgets on load rather than on defects. A single run
# tuned to own the whole box is the wrong default when runs are concurrent by
# construction.
#
# A quarter is a CEILING, not a target, and it is deliberately conservative:
# four concurrent runs fit, and a fifth still degrades gracefully instead of
# thrashing. Runs take longer. That is the accepted trade.
#
# `go test -p` defaults to GOMAXPROCS, so this also caps how many PACKAGES
# compile and run at once, which is where the real memory pressure lives.
#
# The floor is 1 and it is load-bearing on a small runner. Two known effects
# there, both slowness rather than incorrectness:
#   - all ~450 packages run one at a time (a cached unit stage took 22 minutes
#     on GitHub's hosted 4-vCPU runner).
#   - internal/component/cli/testing's headless model races a command goroutine
#     against 10 runtime.Gosched() yields and falls back to a 900ms wait when
#     the goroutine has not finished (headless.go:152-191). With one P the
#     command cannot run in parallel, so the .et suite pays the 900ms far more
#     often (CI run 30219943935). GO_TEST_TIMEOUT below absorbs it.
# n=4 -> 1, n=8 -> 2, n=16 -> 4, n=32 -> 8.
GO_TEST_PROCS := $(shell n=$$(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4); p=$$(( n / 4 )); [ $$p -lt 1 ] && p=1; echo $$p)

# ─── Job admission ──────────────────────────────────────────────────────────
# Several agents share this checkout and this machine, and each decides on its
# own to test or lint. Every heavy target is sized for a share of the box, so
# nothing stops eight of them starting at once: on 2026-08-17 that froze the
# machine (plan/spec-shared-machine-job-admission.md).
#
# So a heavy target is a PAIR. The public half hands the work to
# scripts/dev/ze-run.sh, which runs it now, queues it behind the jobs already in
# flight, or attaches it to an equivalent run; `_<target>-impl` holds the work
# and is what the wrapper calls back into. A target admitted this way is listed
# in the `.PHONY: _<target>-impl` block at the end of the file that defines it.
#
# A target is admitted when its recipe starts a Go test binary, the `ze-test`
# runner, golangci-lint, govulncheck, Docker or QEMU. A target that only reads
# source text is not: it is single-threaded and finishes in seconds. Neither are
# the interactive ones (ze-qemu-shell, ze-qemu-debug, ze-gokrazy-run), because
# the wrapper pipes a job's output through `tee` and that loses the terminal.
#
# Nesting costs nothing: the wrapper exports ZE_RUN_JOB, and a job started
# inside another job's slot execs straight through instead of queueing behind
# its own parent. That is how every stage of ze-precommit-verify runs.
#
# How many admitted jobs scripts/dev/ze-run.sh runs at once. DERIVED from the
# ceiling above, not guessed: every heavy job in this repository is already
# sized at a quarter of the cores -- GO_TEST_PROCS for a Go test run,
# `run.concurrency: 8` in .golangci.yml for the linter -- so cores divided by
# that ceiling is exactly how many fit, which is the ruling stated above:
# "four concurrent runs fit, and a fifth still degrades gracefully".
#
# The quantity being divided is CPU, and that is measured rather than assumed.
# plan/spec-shared-machine-job-admission.md broke its own assumption A-1 with a
# paired measurement: capping the linter cut CPU 1978% to 798% and peak RSS only
# 4.55 to 3.96 GiB. On this 32-core, 31 GiB box four jobs are about 32 runnable
# threads and 16 GiB, so CPU binds first and memory has headroom.
#
# One slot for everything would be stricter than that evidence: eight sessions
# would queue for a box running at a quarter of its capacity, and a queue nobody
# believes in is the thing agents route around. Take it down for one run with
# `make <target> ZE_RUN_SLOTS=1`, or up when the jobs in flight are known light.
ZE_RUN_SLOTS ?= $(shell n=$$(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4); s=$$(( n / $(GO_TEST_PROCS) )); [ $$s -lt 1 ] && s=1; echo $$s)
export ZE_RUN_SLOTS

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
# Race-detector test commands are the sole CGO-enabled Go compilation path.
# They run with -race on both Linux and Darwin. Their test binaries never ship
# or serve as release/build evidence. Every non-race command inherits the
# top-level CGO_ENABLED=0 export.
GO_TEST_RACE_LABEL = race-instrumented
GO_TEST_RACE = CGO_ENABLED=1 GOMAXPROCS=$(GO_TEST_PROCS) go test -timeout $(GO_TEST_TIMEOUT) -tags '$(GO_TEST_TAGS)' -race
GO_TEST_CORE_RACE = CGO_ENABLED=1 GOMAXPROCS=$(GO_TEST_PROCS) go test -timeout $(GO_TEST_TIMEOUT) -tags '$(GO_TEST_CORE_TAGS)' -race
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
# than on ze -- scripts/le/install_test.py checks for brew or apt, and Alpine has neither.
# The rest shell out to gate binaries they compile on the fly, which over the 9p
# mount blows their own 60s timeouts. They cost ~33 minutes of the VM run and
# every failure was environmental. They still run in full under `make ze-precommit-verify`
# on the host, so nothing is uncovered by skipping them here.
ZE_PACKAGES_EXCLUDE ?=
ZE_PACKAGES = $$(go list ./... | grep -v '^github.com/ze-software/ze$$'$(if $(ZE_PACKAGES_EXCLUDE), | grep -vE '$(ZE_PACKAGES_EXCLUDE)',))

# Default target
.DEFAULT_GOAL := help

# ─── Include split Makefile modules ─────────────────────────────────────────
# Session-scoped binary directory (ZEBIN_*, ZE_BIN_DIR). Must come FIRST: every
# include below refers to the binaries through these variables.
include mk/helper-session.mk
include mk/test-unit.mk
include mk/test-functional.mk
include mk/test-fuzz.mk
include mk/test-chaos.mk
include mk/test-integration.mk
include mk/test-release.mk
include mk/perf-bench.mk
include mk/test-alloc.mk
include mk/check-docs.mk
include mk/check-cli.mk
include mk/check-rules.mk
include mk/report-inventory.mk
include mk/helper-verify.mk
include mk/build-gokrazy.mk
include mk/test-mutation.mk
include mk/build-appliance.mk

include mk/build-terminal-demo.mk
include mk/schedule-cadence.mk
# ─── Build ──────────────────────────────────────────────────────────────────

all: ze-lint ze-unit-test build

# The templ line deletes a *_templ.go whose .templ source is gone. That is
# correct regeneration on a write path: the source produces the file, so the
# source's removal removes it. The removal is a tracked-file deletion, so it
# shows in `git status` and in review.
#
# No gate under `make ze-precommit-verify` names that deletion in the same cycle. Do NOT
# claim ze-generated-files-reconcile covers it: nothing runs that target, and
# TestVerifyStagesGuardGeneratedFiles asserts it must stay unwired. The real
# catch is ze-templ-orphan-check on a fresh checkout, one cycle later, where the
# file is on disk with no source.
#
# The vendor sync belongs here for the same reason the others do: a consumer
# asset copy is a GENERATED file. //go:embed cannot reach outside its own
# package, so one library is vendored once per consumer, and a copy that
# nothing regenerates diverges from third_party/web/ without a sound.
# ze-vendor-web-check is its read-only twin, below.
generate:
	@go run scripts/codegen/yang_glue.go
	@go run scripts/codegen/plugin_imports.go
	@go run scripts/codegen/feature_tags.go
	@go run scripts/codegen/web_assets.go
	@go run github.com/a-h/templ/cmd/templ generate -path internal
	@python3 scripts/dev/fuzz-targets.py
	@go run scripts/vendor/sync_web.go

# Regenerate api/proto/*.pb.go from api/proto/ze.proto. Deliberately NOT part of
# `generate`: it needs protoc on PATH, while the .pb.go files are checked in so a
# normal build never calls it. The two codegen plugins are built from vendor/,
# so their versions are pinned by go.mod; only the `protoc vN` header comment
# reflects the locally installed protoc. protoc-gen-go applies json_name to
# protojson descriptors but not encoding/json struct tags, so the post-step
# derives those tags from the same explicit options in ze.proto.
#
# Needed after any change to the module path: the embedded rawDesc carries
# go_package as a LENGTH-PREFIXED field, so a textual rewrite of a
# different-length path compiles and decodes to garbage. rename_module_path.py
# refuses .pb.go for exactly this reason and points here.
ze-proto-generate:
	@command -v protoc >/dev/null || { echo "protoc not found -- install it (brew install protobuf)"; exit 1; }
	@CGO_ENABLED=0 go build -mod=vendor -o bin/protoc-gen-go google.golang.org/protobuf/cmd/protoc-gen-go
	@CGO_ENABLED=0 go build -mod=vendor -o bin/protoc-gen-go-grpc google.golang.org/grpc/cmd/protoc-gen-go-grpc
	@PATH="$(CURDIR)/bin:$$PATH" protoc \
		--go_out=. --go_opt=module=$(ZE_MODULE) \
		--go-grpc_out=. --go-grpc_opt=module=$(ZE_MODULE) \
		api/proto/ze.proto
	@python3 scripts/codegen/proto_json_tags.py
	@echo "Regenerated api/proto/ze.pb.go api/proto/ze_grpc.pb.go"

ze-plugin-imports-check:
	@go run scripts/codegen/plugin_imports.go --check

ze-fuzz-targets-check:
	@python3 scripts/dev/fuzz-targets.py --check

ze-yang-glue-check:
	@go run scripts/codegen/yang_glue.go --check

ze-feature-tags-check:
	@go run scripts/codegen/feature_tags.go --check

# Refuse a page_assets.go that disagrees with the markup its package renders.
#
# The generator walks the templ component graph from each page and collects the
# htmx attributes that page reaches, so a component that gains hx-ext="sse"
# changes the set of the pages reaching it. A stale file then leaves a page
# loading an extension it now needs nothing of, or missing one it does need. The
# second is invisible everywhere but the browser: the page renders and does
# nothing.
ze-web-assets-check:
	@go run scripts/codegen/web_assets.go --check

# Report a *_templ.go whose .templ source is gone, and a .templ outside the
# walk. It writes nothing and it deletes nothing.
#
# -keep-orphaned-files makes templ SILENT about an orphan, not only harmless:
# HandleEvent (vendor/github.com/a-h/templ/cmd/templ/generatecmd/eventhandler.go)
# returns an empty result before any writer is consulted, so the check writer
# never sees the file. A generated file whose source was deleted is real
# staleness, and this target is what finds it. The tests live in
# scripts/dev/templ_orphan_check_test.py.
ze-templ-orphan-check:
	@python3 scripts/dev/templ_orphan_check.py

# Refuse a *_templ.go that its .templ source no longer produces. templ has its
# own -check mode, which reports a stale file rather than rewriting it, so this
# is the ze-plugin-imports-check shape with no diff of its own.
#
# -keep-orphaned-files IS WHAT MAKES THIS TARGET READ-ONLY. Without it templ
# deletes an orphaned *_templ.go in -check mode as readily as in generate mode:
# HandleEvent (vendor/github.com/a-h/templ/cmd/templ/generatecmd/eventhandler.go)
# calls os.Remove before any writer is consulted, and only keepOrphanedFiles
# gates that call. With the flag the run writes nothing and deletes nothing, and
# the orphan is reported by ze-templ-orphan-check above instead.
#
# THE WALK IS SCOPED TO internal/, AND ze-templ-orphan-check KEEPS THAT SCOPE
# HONEST. Every renderer in ze lives under internal/, so a .templ outside it is
# generated by nobody and reported by that target. A walk from the repo root
# would descend into gokrazy/ and tmp/, which hold module and build caches.
#
# A deleted .templ whose generated file survives is drift, which is the thing
# R-1 of spec-web-templ-migration asks this target to catch.
#
# -path internal IS LOAD-BEARING FOR FILE CONTENT, not only for the walk.
# FSEventHandler.generate computes each file name with filepath.Rel against the
# walk root and hands it to generator.WithFileName, so the root is written into
# the generated Go. A bare `templ generate`, which is what an editor-on-save
# integration runs, rewrites every *_templ.go and reds this gate.
#
# vendor/ is out of the population, and it is not empty: go mod vendor copies
# two .templ files that templ's own CLI carries. templ skips a vendor directory
# on its walk, so those files belong to the dependency and never to ze.
ze-templ-output-check: ze-templ-orphan-check
	@go run github.com/a-h/templ/cmd/templ generate -check -keep-orphaned-files -path internal

# require-go-test fails when no test in package $(2) is named exactly $(1).
# `go test -run <name>` exits 0 over an empty selection, so a target that only
# runs a named test passes after that test is renamed or deleted. That is the
# cheapest route from red to green, and it is the move a ratchet exists to
# refuse. `-list` prints the names that match; an empty print fails here before
# the run. A build failure exits with go's own status, so a broken package
# never reports as a missing test.
define require-go-test
names="$$($(GO_TEST) -list '^$(1)$$' $(2))" || exit $$?; \
	echo "$$names" | grep -qx '$(1)' || { echo "error: $(2) holds no test named $(1); this target cannot pass over zero tests"; exit 1; }
endef

# Compare the rendered bytes of every web and lg template against the fixtures
# in each package's testdata/golden/. The suites that assert on HTML use
# strings.Contains, which cannot see a byte-level change that keeps the asserted
# substring, so this is the only check that proves the rendered output is
# unchanged. Both tests also run under ze-unit-test; this target names them so a
# rendering change can be checked on its own.
#
# THREE captures run here and none proves another. The TEMPLATE capture
# executes one parsed template against data the test writes. The HANDLER capture
# issues an HTTP request and records the response, so it covers the view model
# the handler builds and the wrappers it renders through -- RenderFragment,
# RenderConfigToHTML, RenderField and RenderL2TPTemplate each discard the
# execution error and return "", which is why the template capture bypasses them.
# The MARKUP capture renders the builders that write HTML in Go, over input it
# fixes: no template holds that markup, and the handler capture normalizes it
# because the live input is whatever host.Detect finds on the machine.
ze-web-golden-check:
	@scripts/dev/ze-run.sh ze-web-golden-check $(MAKE) --no-print-directory _ze-web-golden-check-impl

_ze-web-golden-check-impl:
	@$(call require-go-test,TestWebGoldenOutput,./internal/component/web/)
	@$(GO_TEST) -run 'TestWebGoldenOutput' ./internal/component/web/
	@$(call require-go-test,TestLGGoldenOutput,./internal/component/lg/)
	@$(GO_TEST) -run 'TestLGGoldenOutput' ./internal/component/lg/
	@$(call require-go-test,TestWebHandlerGoldenOutput,./internal/component/web/)
	@$(GO_TEST) -run 'TestWebHandlerGoldenOutput' ./internal/component/web/
	@$(call require-go-test,TestLGHandlerGoldenOutput,./internal/component/lg/)
	@$(GO_TEST) -run 'TestLGHandlerGoldenOutput' ./internal/component/lg/
	@$(call require-go-test,TestWebMarkupGoldenOutput,./internal/component/web/)
	@$(GO_TEST) -run 'TestWebMarkupGoldenOutput' ./internal/component/web/

# Compare every captured unit against the bytes it held BEFORE the templ port,
# read out of git at REF, through golden.AssertPortFidelity. It is the evidence for
# AC-2 of spec-web-templ-migration.
#
#   make ze-templ-port-check REF=80f0b8b57
#
# REF is empty by default, and an empty REF means golden.PrePortRef, which is
# the commit the wiring phase captured every fixture at. Passing it is how the
# instrument outlives this port: a later port names its own commit.
#
# The two tests also run under ze-unit-test, because neither is gated and
# neither skips. This target names them so the comparison can be run against
# another ref on its own.
#
# It answers a different question from ze-web-golden-check. That target proves
# the fixtures ARE the current render. This one proves the current render says
# what the pre-port render said. Once a fixture is recaptured, the golden check
# compares the port against itself, and only this one still reads the bytes the
# port was supposed to preserve.
REF ?=
ze-templ-port-check:
	@scripts/dev/ze-run.sh ze-templ-port-check $(MAKE) --no-print-directory _ze-templ-port-check-impl

_ze-templ-port-check-impl:
	@$(call require-go-test,TestWebTemplPortFidelity,./internal/component/web/)
	@$(GO_TEST) -run 'TestWebTemplPortFidelity' ./internal/component/web/ -port-ref='$(REF)'
	@$(call require-go-test,TestLGTemplPortFidelity,./internal/component/lg/)
	@$(GO_TEST) -run 'TestLGTemplPortFidelity' ./internal/component/lg/ -port-ref='$(REF)'

# Recapture the golden fixtures after a DELIBERATE markup change. Read the diff
# before committing it: every byte this rewrites is a byte an operator receives.
ze-web-golden-update:
	@scripts/dev/ze-run.sh ze-web-golden-update $(MAKE) --no-print-directory _ze-web-golden-update-impl

_ze-web-golden-update-impl:
	@$(call require-go-test,TestWebGoldenOutput,./internal/component/web/)
	@$(GO_TEST) -run 'TestWebGoldenOutput' ./internal/component/web/ -update-golden
	@$(call require-go-test,TestLGGoldenOutput,./internal/component/lg/)
	@$(GO_TEST) -run 'TestLGGoldenOutput' ./internal/component/lg/ -update-golden
	@$(call require-go-test,TestWebHandlerGoldenOutput,./internal/component/web/)
	@$(GO_TEST) -run 'TestWebHandlerGoldenOutput' ./internal/component/web/ -update-golden
	@$(call require-go-test,TestLGHandlerGoldenOutput,./internal/component/lg/)
	@$(GO_TEST) -run 'TestLGHandlerGoldenOutput' ./internal/component/lg/ -update-golden
	@$(call require-go-test,TestWebMarkupGoldenOutput,./internal/component/web/)
	@$(GO_TEST) -run 'TestWebMarkupGoldenOutput' ./internal/component/web/ -update-golden
	@echo "Updated internal/component/web/testdata/ and internal/component/lg/testdata/"

# Recapture the chaos dashboard fixtures after a DELIBERATE markup change. It is
# ze-web-golden-update's twin: the chaos capture holds 52 units and the same
# rule applies to every one of them. Read the diff before committing it.
#
# The check needs no target of its own. TestChaosGoldenOutput is a plain Go
# test, so ze-unit-test runs it; only the recapture needs a name.
ze-chaos-golden-update:
	@scripts/dev/ze-run.sh ze-chaos-golden-update $(MAKE) --no-print-directory _ze-chaos-golden-update-impl

_ze-chaos-golden-update-impl:
	@$(call require-go-test,TestChaosGoldenOutput,./internal/chaos/web/)
	@$(GO_TEST) -run 'TestChaosGoldenOutput' ./internal/chaos/web/ -update-golden
	@echo "Updated internal/chaos/web/testdata/golden/"

# Regenerate plugin/all registry snapshots (testdata/*.snapshot) from the live
# registry after adding or removing a plugin. ze-unit-test fails with a clear
# "unexpected/missing" message and points here, so the lists never silently
# drift from all.go. Review the diff before committing.
ze-plugin-snapshot-update:
	@scripts/dev/ze-run.sh ze-plugin-snapshot-update $(MAKE) --no-print-directory _ze-plugin-snapshot-update-impl

_ze-plugin-snapshot-update-impl:
	@$(GO_TEST) -run 'TestRegisteredPluginNames|TestRegisteredWireMethods|TestYANGSchemaProviders' ./internal/component/plugin/all/ -update
	@echo "Updated internal/component/plugin/all/testdata/*.snapshot"

build: generate $(ZEBIN_ZE) $(ZEBIN_APPLIANCE) $(ZEBIN_SETUP) $(ZEBIN_STRIPPED) $(ZEBIN_TEST) $(ZEBIN_CHAOS) $(ZEBIN_PERF) $(ZEBIN_ANALYZE)
	@echo "All binaries built"
ZE_SITE_OUTPUT ?= $(CURDIR)/../gh-pages
ZE_SITE_DEMO_OUTPUT ?= $(ZE_SITE_OUTPUT)/assets/demos

ze-site-generate: TERMINAL_DEMO_OUTPUT=$(ZE_SITE_DEMO_OUTPUT)
ze-site-generate: $(ZEBIN_ZE)
	@test -e "$(ZE_SITE_OUTPUT)/.git" || { echo "error: ZE_SITE_OUTPUT must be a git worktree: $(ZE_SITE_OUTPUT)"; exit 1; }
	@ZE_TERMINAL_DEMO_OUTPUT="$(ZE_SITE_DEMO_OUTPUT)" python3 demos/terminal/render.py --all --stamp-definition-hashes || true
	@ZE_TERMINAL_DEMO_OUTPUT="$(ZE_SITE_DEMO_OUTPUT)" python3 demos/terminal/render.py --all --check-definition || $(MAKE) ze-terminal-demo-release-render-all TERMINAL_DEMO_OUTPUT="$(ZE_SITE_DEMO_OUTPUT)"
	uv run --with pytest --with markdown python3 -m pytest -q website/tools/test_*.py
	ZE_SITE_OUTPUT="$(ZE_SITE_OUTPUT)" ZE_TERMINAL_DEMO_SOURCE="$(ZE_SITE_DEMO_OUTPUT)" website/update-website.sh


ze-build:
	@mkdir -p $(ZE_BIN_DIR)
	CGO_ENABLED=0 $(GO) build -tags 'ze_core ze_distro $(ZE_FEATURES) $(ZE_TAGS)' -ldflags "$(ZE_LDFLAGS)" -o $(ZEBIN_ZE) ./cmd/ze
	$(call ZE_SEED_SESSION_STORE,$(ZEBIN_ZE))

ze-appliance-build:
	@mkdir -p $(ZE_BIN_DIR)
	CGO_ENABLED=0 $(GO) build -tags 'ze_core ze_appliance $(ZE_FEATURES) $(ZE_TAGS)' -ldflags "$(ZE_LDFLAGS)" -o $(ZEBIN_APPLIANCE) ./cmd/ze
	$(call ZE_SEED_SESSION_STORE,$(ZEBIN_APPLIANCE))

ze-setup-build:
	@mkdir -p $(ZE_BIN_DIR)
	CGO_ENABLED=0 $(GO) build -tags 'ze_setup $(ZE_TAGS)' -ldflags "$(ZE_LDFLAGS)" -o $(ZEBIN_SETUP) ./cmd/ze

ze-stripped-build:
	@mkdir -p $(ZE_BIN_DIR)
	CGO_ENABLED=0 $(GO) build -tags 'ze_core ze_ssh $(ZE_TAGS)' -ldflags "$(ZE_LDFLAGS)" -o $(ZEBIN_STRIPPED) ./cmd/ze
	$(call ZE_SEED_SESSION_STORE,$(ZEBIN_STRIPPED))

# ze-chaos and ze-perf drive an in-process BGP reactor, so they force ze_bgp on:
# their own tags (ze_chaos / ze_perf) do not include ZE_FEATURES, and without it
# the BGP plugins would silently register nothing rather than fail to build.
ze-chaos-build:
	@mkdir -p $(ZE_BIN_DIR)
	CGO_ENABLED=0 $(GO) build -tags 'ze_chaos ze_bgp' -o $(ZEBIN_CHAOS) ./cmd/ze

ze-test-build:
	@mkdir -p $(ZE_BIN_DIR)
	CGO_ENABLED=0 $(GO) build -tags 'ze_test $(ZE_FEATURES) $(ZE_TAGS)' -o $(ZEBIN_TEST) ./cmd/ze

ze-analyze-build:
	@mkdir -p $(ZE_BIN_DIR)
	CGO_ENABLED=0 $(GO) build -tags ze_analyze -o $(ZEBIN_ANALYZE) ./cmd/ze


$(ZEBIN_ZE): $(shell find cmd/ze internal -name '*.go' 2>/dev/null)
	@echo "Building ze..."
	@mkdir -p $(ZE_BIN_DIR)
	CGO_ENABLED=0 $(GO) build -tags 'ze_core ze_distro $(ZE_FEATURES) $(ZE_TAGS)' -ldflags "$(ZE_LDFLAGS)" -o $(ZEBIN_ZE) ./cmd/ze
	$(call ZE_SEED_SESSION_STORE,$(ZEBIN_ZE))

$(ZEBIN_APPLIANCE): $(shell find cmd/ze internal -name '*.go' 2>/dev/null)
	@echo "Building ze-appliance..."
	@mkdir -p $(ZE_BIN_DIR)
	CGO_ENABLED=0 $(GO) build -tags 'ze_core ze_appliance $(ZE_FEATURES) $(ZE_TAGS)' -ldflags "$(ZE_LDFLAGS)" -o $(ZEBIN_APPLIANCE) ./cmd/ze
	$(call ZE_SEED_SESSION_STORE,$(ZEBIN_APPLIANCE))

$(ZEBIN_SETUP): $(shell find cmd/ze internal -name '*.go' 2>/dev/null)
	@echo "Building ze-setup..."
	@mkdir -p $(ZE_BIN_DIR)
	CGO_ENABLED=0 $(GO) build -tags 'ze_setup $(ZE_TAGS)' -ldflags "$(ZE_LDFLAGS)" -o $(ZEBIN_SETUP) ./cmd/ze

$(ZEBIN_STRIPPED): $(shell find cmd/ze internal -name '*.go' 2>/dev/null)
	@echo "Building ze-stripped..."
	@mkdir -p $(ZE_BIN_DIR)
	CGO_ENABLED=0 $(GO) build -tags 'ze_core ze_ssh $(ZE_TAGS)' -ldflags "$(ZE_LDFLAGS)" -o $(ZEBIN_STRIPPED) ./cmd/ze
	$(call ZE_SEED_SESSION_STORE,$(ZEBIN_STRIPPED))
$(ZEBIN_TEST): $(shell find cmd/ze internal -name '*.go' 2>/dev/null)
	@echo "Building ze-test..."
	@mkdir -p $(ZE_BIN_DIR)
	CGO_ENABLED=0 $(GO) build -tags 'ze_test $(ZE_FEATURES) $(ZE_TAGS)' -o $(ZEBIN_TEST) ./cmd/ze

$(ZEBIN_CHAOS): $(shell find cmd/ze internal -name '*.go' 2>/dev/null)
	@echo "Building ze-chaos..."
	@mkdir -p $(ZE_BIN_DIR)
	CGO_ENABLED=0 $(GO) build -tags 'ze_chaos ze_bgp' -o $(ZEBIN_CHAOS) ./cmd/ze

$(ZEBIN_ANALYZE): $(shell find cmd/ze internal -name '*.go' 2>/dev/null)
	@echo "Building ze-analyze..."
	@mkdir -p $(ZE_BIN_DIR)
	CGO_ENABLED=0 $(GO) build -tags ze_analyze -o $(ZEBIN_ANALYZE) ./cmd/ze

$(ZEBIN_PERF): $(shell find cmd/ze internal -name '*.go' 2>/dev/null)
	@echo "Building ze-perf..."
	@mkdir -p $(ZE_BIN_DIR)
	CGO_ENABLED=0 $(GO) build -tags 'ze_perf ze_bgp' -o $(ZEBIN_PERF) ./cmd/ze

# ─── Docker ────────────────────────────────────────────────────────────────

ZE_DOCKER_IMAGE ?= ze
ZE_DOCKER_TAG ?= $(ZE_VERSION)

# The lab image is a SECOND image, not a replacement: docker/Dockerfile stays a
# static binary on scratch for deployments, and this one adds the shell and
# iproute2 netlab and containerlab exec inside a node. Distinct names, so one is
# never mistaken for the other.
ZE_LAB_IMAGE ?= netlab/ze
ZE_LAB_TAG ?= latest

# Both images derive their default-on feature tags from feature-gates.txt inside
# the Dockerfile, so `docker compose build` and a bare `docker build` get the same
# binary make does. ZE_TAGS is passed as EXTRA tags, its meaning everywhere else
# in this file.
ze-docker-build:
	@command -v docker >/dev/null || { echo "error: docker not found"; exit 1; }
	docker build \
		-f docker/Dockerfile \
		--build-arg ZE_VERSION=$(ZE_VERSION) \
		--build-arg ZE_BUILD_DATE=$(ZE_BUILD_DATE) \
		--build-arg ZE_TAGS='$(ZE_TAGS)' \
		-t $(ZE_DOCKER_IMAGE):$(ZE_DOCKER_TAG) \
		-t $(ZE_DOCKER_IMAGE):latest \
		.

ze-docker-lab-build:
	@command -v docker >/dev/null || { echo "error: docker not found"; exit 1; }
	docker build \
		-f docker/Dockerfile.lab \
		--build-arg ZE_VERSION=$(ZE_VERSION) \
		--build-arg ZE_BUILD_DATE=$(ZE_BUILD_DATE) \
		--build-arg ZE_TAGS='$(ZE_TAGS)' \
		-t $(ZE_LAB_IMAGE):$(ZE_LAB_TAG) \
		.

# ─── Lint and specialised test targets ──────────────────────────────────────

# golangci-lint analyses ONE build: the host GOOS, with the build-tags
# .golangci.yml lists (ze_core + every feature gate, GENERATED by
# scripts/codegen/feature_tags.go). Two file populations fall outside that build
# and were reported on by nothing, in CI or locally:
#
#   - //go:build integration     82 tracked files on 2026-08-23, 38 packages, the
#                                mandatory home for kernel-facing tests
#                                (ai/rules/platform-linux.md). That count grows,
#                                and the coverage does not need re-earning: the
#                                pass keys on the TAG, never on a file list
#   - //go:build linux           invisible on a non-Linux host, so every netlink,
#                                nftables and XFRM file on a dev machine
#
# The second pass supplies the two axes the config cannot express: GOOS, and a
# tag that is NOT a feature gate. `--build-tags` ADDS to the config list rather
# than replacing it (measured, golangci-lint v2.10.1), so the generated gate
# tags still apply and nothing is duplicated here. That is what keeps
# .golangci.yml generated and untouched (ai/rules/repo-maintenance.md) and keeps
# `integration` out of feature-gates.txt, which holds feature gates only
# (ai/rules/plugins.md).
#
# Adding `integration` to the generated tag list instead would have changed
# nothing on a dev machine: 80 of the 82 files are `integration && linux`, and
# measured on darwin the tag alone reaches 0 of them.
#
# What these two passes still do NOT reach is every other build tag, and every
# other GOOS and GOARCH. The personality tags among them (ze_installer,
# ze_distro, ze_appliance, ze_setup) cannot share one pass, because each names a
# mutually exclusive build. scripts/dev/lint_flavors.py runs one pass for each,
# derives its package set from the tree, and reports what no pass reaches. See
# plan/journal/gate-excludes-part-of-its-population.md.
#
# The package pattern is ./... rather than four roots. 48 files under scripts/
# carried no build constraint at all and were unlinted purely because nothing
# pointed the linter at them -- the repository's own gates among them, including
# the test that pins the integration pass above. ZE_PACKAGES (the unit-test
# population) has always been ./..., and the two now agree.
ZE_LINT_PKGS := ./...

# Memory half of the linter ceiling; the worker half is ZE_LINT_RUN's `-j`
# below. golangci-lint v2.10.1 accepts no memory setting of its own, so the Go
# runtime env var is the only place this can go: `golangci-lint config verify`
# rejects memory, memory-limit, mem-limit, gomemlimit, max-memory and
# memory-ceiling as unknown keys, and `golangci-lint run -h` lists no memory
# flag. The binary honors GOMEMLIMIT, which its runtime reads at startup.
#
# GOMEMLIMIT is a soft limit: the GC works harder as the heap approaches it, and
# a run whose live heap exceeds it gets slow rather than killed. That is the
# failure this sizing exists to avoid, and a fixed number walked into it.
#
# An EIGHTH of the machine's RAM, floored at 4 GiB, DERIVED for the same reason
# the worker count is: Ze is developed on machines of different sizes. It was a
# flat 4 GiB until 2026-08-21, measured on a 31 GiB box where it was about an
# eighth. On the 64 GiB machine it is 6%, and the linter ran pinned at 3.97 GiB
# against it -- GC-thrashing to save 0.55 GiB, since
# plan/spec-shared-machine-job-admission.md measured the uncapped peak RSS at
# 4.55 GiB. The floor keeps the measured 4 GiB on the box it was measured on.
#
# ZE_RUN_SLOTS jobs run at once, so the worst case is a slots-many multiple of
# this. At four slots that is half the RAM on the 64 GiB box and 16 GiB on the
# 31 GiB one, which is the headroom the admission spec asked for.
#
# Raise it for one run with `make ze-lint ZE_LINT_MEMLIMIT=16GiB`.
# See plan/spec-shared-machine-job-admission.md, AC-1.
ZE_LINT_MEMLIMIT ?= $(shell g=$$(awk '/MemTotal/{printf "%d", $$2/1048576}' /proc/meminfo 2>/dev/null); [ -z "$$g" ] && g=$$(( $$(sysctl -n hw.memsize 2>/dev/null || echo 0) / 1073741824 )); m=$$(( g / 8 )); [ $$m -lt 4 ] && m=4; echo $${m}GiB)
ZE_LINT := GOMEMLIMIT=$(ZE_LINT_MEMLIMIT) golangci-lint

# The worker half of the ceiling, DERIVED from the core count exactly as
# GO_TEST_PROCS is, because Ze is developed on machines of different sizes and a
# hardcoded number is a quarter of the box on one of them and a half on another.
# It lived in .golangci.yml as `concurrency: 8` until 2026-08-21; that file
# cannot divide, so the number could not follow the machine. See the comment
# there, and "Job admission" above: ZE_RUN_SLOTS divides the box by this same
# share to decide how many jobs run at once, so a linter taking more than its
# declared share breaks that arithmetic rather than just running hot.
#
# Every `golangci-lint run` in this repository goes through ZE_LINT_RUN. A raw
# call reaches the linter with neither ceiling; `check_raw_test_invocation` in
# .claude/hooks/pretool-bash.py refuses one from an agent.
ZE_LINT_RUN := $(ZE_LINT) run -j $(GO_TEST_PROCS)

# The flavor driver runs golangci-lint itself, once per build that the two
# passes below cannot reach, so it carries the SAME two ceilings rather than a
# second set: GOMEMLIMIT through the environment, `-j` through the flag it
# passes on to every run. TestLintFlavorDriverCarriesTheLinterCeilings pins the
# pair against ZE_LINT_RUN's.
ZE_LINT_FLAVOR_RUN := GOMEMLIMIT=$(ZE_LINT_MEMLIMIT) python3 scripts/dev/lint_flavors.py -j $(GO_TEST_PROCS)

# Two full golangci-lint passes over the tree, each sized for the whole box,
# then one SCOPED pass for each build neither of them analyses. Measured
# 2026-08-17, the two full passes are about 18 minutes of a 20-minute full
# verify, so this is the heaviest single job in the repository and the one that
# froze the machine when two sessions started it at once. It goes through the
# shared admission point for that reason
# (plan/spec-shared-machine-job-admission.md). The flavor passes add about a
# minute: each lints only the packages holding a file the two above do not load,
# 1 to 59 packages rather than 649 (measured 2026-08-24).
# Run as a stage of ze-precommit-verify it inherits that job's slot instead of
# queueing behind it, which is what ZE_RUN_JOB is for.
ze-lint:
	@scripts/dev/ze-run.sh ze-lint $(MAKE) --no-print-directory _ze-lint-impl

_ze-lint-impl:
	@echo "Running ze linter..."
	@$(ZE_LINT_RUN) $(ZE_LINT_PKGS)
	@echo "Running ze linter (GOOS=linux, integration tag)..."
	@GOOS=linux $(ZE_LINT_RUN) --build-tags integration $(ZE_LINT_PKGS)
	@$(ZE_LINT_FLAVOR_RUN)

ze-evidence-vet:
	@echo "Vetting evidence scripts (GOOS=linux)..."
	@GOOS=linux go vet ./scripts/evidence/...

ze-unit-reactor-test-race:
	@scripts/dev/ze-run.sh ze-unit-reactor-test-race $(MAKE) --no-print-directory _ze-unit-reactor-test-race-impl

_ze-unit-reactor-test-race-impl:
	@echo "Stress-testing reactor (count=20, $(GO_TEST_RACE_LABEL))..."
	$(GO_TEST_RACE) -count=20 ./internal/component/bgp/reactor/...

ze-unit-linux-test:
	@scripts/dev/ze-run.sh ze-unit-linux-test $(MAKE) --no-print-directory _ze-unit-linux-test-impl

_ze-unit-linux-test-impl:
	@command -v docker >/dev/null || { echo "error: docker not found"; exit 1; }
	@mkdir -p tmp/linux-go-cache tmp/linux-gomodcache
	docker run --rm \
		--user "$$(id -u):$$(id -g)" \
		-v "$(CURDIR):/src" \
		-w /src \
		-e HOME=/tmp \
		-e CGO_ENABLED=0 \
		-e GOCACHE=/src/tmp/linux-go-cache \
		-e GOMODCACHE=/src/tmp/linux-gomodcache \
		$(ZE_LINUX_GO_IMAGE) \
		go test $(ZE_LINUX_TEST_PACKAGES) -count=1

ze-functional-exabgp-test:
	@scripts/dev/ze-run.sh ze-functional-exabgp-test $(MAKE) --no-print-directory _ze-functional-exabgp-test-impl

_ze-functional-exabgp-test-impl: $(ZEBIN_ZE) $(ZEBIN_TEST)
	@echo "Running ExaBGP compatibility tests..."
	uv run --with paramiko $(ZEBIN_TEST) exabgp --all --timeout $(ZE_EXABGP_TIMEOUT)s

# Software-composition analysis (SCA): govulncheck (golang.org/x/vuln) scans the
# module's dependency graph against the Go vulnerability database (vuln.go.dev)
# and reports only vulnerabilities reachable from ze's own call graph.
#
# Both normal verification modes run this target after hook tests and before unit
# tests. The dedicated scheduled and manual workflow also runs it daily to catch
# advisories published after a commit.
#
# The `@latest` tool and vuln.go.dev database are live network inputs. The outer
# `go run` stays host-native; `-exec` gives only the scanner process the
# Linux/amd64 analysis environment.
#
# `@latest` runs the tool from outside the main module, so there is no go.mod or
# vendor churn. This repo vendors dependencies, and adding x/vuln as a module
# `tool` dependency would vendor its large analysis tree (x/tools SSA, callgraph,
# and related packages).
ze-dependency-vulnerability-check:
	@scripts/dev/ze-run.sh ze-dependency-vulnerability-check $(MAKE) --no-print-directory _ze-dependency-vulnerability-check-impl

_ze-dependency-vulnerability-check-impl:
	@echo "Running govulncheck (SCA: module deps vs vuln.go.dev)..."
	$(GO) run -exec='env GOOS=linux GOARCH=amd64' golang.org/x/vuln/cmd/govulncheck@latest ./...

# ─── Scoped targets (parallel-safe) ────────────────────────────────────────

# The changed-package set comes from ONE producer,
# scripts/checks/verify_scope_selector.go, reached through
# scripts/dev/changed-pkgs.sh (see ze-verify-scope-selector below). It is the
# uncommitted .go changes, plus the packages committed since the last green
# verify, plus the packages that IMPORT either at up to two levels, with the
# build tags on -- so a //go:build ze_ssh importer is visible, which the old
# untagged expansion could not see. The committed-since term closes a gap where
# a regression committed before verifying left the working-tree diff and was
# silently skipped by scoped verify.
#
# Inside `make ze-precommit-verify-changed` the answer is computed ONCE, before
# the first stage, and both recipes below read the file the run published
# (scripts/status/verify_run.go, ZE_VERIFY_SCOPE_PACKAGES). Run either target on
# its own and it selects its own set, at the selector's measured 2.4 to 2.9s.
#
# `./...` is a legitimate answer and means the change reaches everything: a
# dependency moved, or a changed path could not be classified. Both recipes must
# pass it through unchanged -- treating it as "nothing selected" would verify
# nothing and report success.
# The second pass and the flavor driver mirror ze-lint's (see the comment above
# that target): without them a new //go:build integration test, or a new
# //go:build ze_installer file, lands unlinted -- the changed-file gate is the
# only lint most edits ever face. `&&` keeps it fail-closed -- with `;` a
# pass-1 failure would be masked by a clean pass 2.
#
# The driver takes the changed packages as its --scope, so it derives each
# flavor's package set from that set rather than from the whole tree. It asserts
# coverage only for a whole-tree run: over a scoped set a missing file is
# missing because the caller said so.
ze-lint-changed:
	@scripts/dev/ze-run.sh ze-lint-changed $(MAKE) --no-print-directory _ze-lint-changed-impl

_ze-lint-changed-impl:
	@pkgs=$$(scripts/dev/changed-pkgs.sh); \
	if [ -z "$$pkgs" ]; then echo "No changed Go packages to lint"; exit 0; fi; \
	echo "Linting changed packages: $$pkgs"; \
	$(ZE_LINT_RUN) $$pkgs && \
	echo "Linting changed packages (GOOS=linux, integration tag): $$pkgs" && \
	GOOS=linux $(ZE_LINT_RUN) --build-tags integration $$pkgs && \
	$(ZE_LINT_FLAVOR_RUN) --scope "$$pkgs"

ze-unit-test-changed:
	@scripts/dev/ze-run.sh ze-unit-test-changed $(MAKE) --no-print-directory _ze-unit-test-changed-impl

_ze-unit-test-changed-impl: ze-scratch-links-ensure
	@pkgs=$$(scripts/dev/changed-pkgs.sh); \
	if [ -z "$$pkgs" ]; then echo "No changed Go packages to test"; exit 0; fi; \
	echo "Testing changed packages ($(GO_TEST_RACE_LABEL)): $$pkgs"; \
	$(GO_TEST_RACE) $$pkgs
	@echo "Unit tests: bare ze_core compile-out checks ($(GO_TEST_RACE_LABEL))..."
	$(GO_TEST_CORE_RACE) ./cmd/ze/hub

# ─── Agent-guard hook tests ────────────────────────────────────────────────

# Regression + behavioural tests for the Claude agent-guard hooks. parity-check
# locks the consolidated dispatchers' exit codes against their golden table;
# fixture-check drives the hooks whose behaviour the golden table cannot isolate
# (c_format_alloc, validate-spec.sh, the commit_helper.py commit-time gates, and
# session-id agreement between lib/session-id.sh and pretool-writeedit.py -- the
# shell WRITES the markers Python READS, and a mismatch fails CLOSED).
# See ai/rules/repo-maintenance.md.
ze-unit-hook-test:
	@scripts/dev/ze-run.sh ze-unit-hook-test $(MAKE) --no-print-directory _ze-unit-hook-test-impl

_ze-unit-hook-test-impl:
	@echo "Hook dispatcher parity (golden exit codes)..."
	@python3 scripts/dev/hook-parity-check.py
	@echo "Hook behavioural fixtures (format-alloc / validate-spec / design-gate / commit-gate)..."
	@python3 scripts/dev/hook-fixture-check.py

# ─── Composite verification targets ────────────────────────────────────────

ze-standard-test: ze-lint ze-unit-test ze-functional-test ze-functional-exabgp-test ze-fuzz-test
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
ze-precommit-verify:
	@scripts/dev/verify-lock.sh ze-precommit-verify env ZE_VERIFY_MAKE="$(MAKE)" $(GO) run ./scripts/status/verify_run.go ze-precommit-verify

# Clear verification debt by RUNNING the gate, never by editing the table.
# Every open row in plan/verification-debt/ names a gate that had not run green
# over the commit that owed it, and `commit_helper.py create --push` refuses
# while one is open. This re-runs each named gate once and writes `cleared` only
# on exit 0; a red gate leaves its rows open and prints its output, and a gate
# no command produces (an independent review) says so. It runs whatever the
# named gates cost, `make ze-precommit-verify` included, so it is an explicit
# invocation and no other target depends on it.
ze-verify-debt-clear:
	@python3 scripts/dev/commit_helper.py debt-clear

# Print the stage list without running anything.
#   make ze-precommit-verify-list
#   make ze-precommit-verify-list ZE_VERIFY_MODE=ze-precommit-verify-changed
#
# Use THIS, never `make -n ze-precommit-verify` (nor -t / -q): the ze-precommit-verify recipe above
# contains $(MAKE), so make executes it even in those no-execute modes while
# every stage sub-make does nothing and "passes" -- the runner would then write
# a FRESH tmp/ze-verify.status for a tree nothing actually verified.
# verify_run.go refuses to run under them for that reason.
#
# The variable is ZE_VERIFY_MODE, not MODE: make imports the environment, and
# MODE is a common enough name that a stray `export MODE=...` would silently
# make this print the wrong list.
ze-precommit-verify-list:
	@$(GO) run ./scripts/status/verify_run.go --list $(or $(ZE_VERIFY_MODE),ze-precommit-verify)

ze-precommit-verify-changed:
	@scripts/dev/verify-lock.sh ze-precommit-verify-changed env ZE_VERIFY_MAKE="$(MAKE)" $(GO) run ./scripts/status/verify_run.go ze-precommit-verify-changed; rc=$$?; python3 scripts/dev/perf-suggest.py || true; exit $$rc

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
# Wired into `make ze-precommit-verify` via stagesForMode() in scripts/status/verify_run.go (BOTH
# branches) -- that function is the only live stage list; nothing in this Makefile
# enumerates verify stages any more.
ze-rfc-check:
	@python3 scripts/dev/rfc_requirements.py --selftest
	@python3 scripts/dev/rfc_requirements.py --check

# Regenerate the RFC requirement ledger (requirement -> enforcing tests): the index
# ai/RFC-REQUIREMENTS.md, and one file per RFC stem under rfc/requirements/ holding that
# RFC's rows. It also deletes a shard whose stem no longer renders.
ze-rfc-index-update:
	@python3 scripts/dev/rfc_requirements.py --write

# Re-stamp the audit verdicts a mechanical edit staled (plan/spec-rfcgate-3-audit-teeth.md).
# `make ze-rfc-check` reports a verdict as SHIFTED when the tagged unit -- the enclosing
# top-level function of each tagged test -- is byte-identical and only the file around it moved:
# a line shift, a sibling test, an import rewrite. Nothing was re-judged, so no human should be
# asked to re-read; this rewrites the file-level fingerprints and nothing else.
#
# Deliberately its OWN target. It is the only thing that writes rfc/audit/ without a human
# editing it, and folding it into ze-rfc-check (a check that writes cannot be trusted to
# report) or ze-rfc-index-update (which runs routinely for reasons unrelated to any audit) would
# automate the blind re-stamp reflex the spec exists to remove. Owner ruling 2026-07-29.
#
# A verdict whose unit, cited producer code, or requirement text moved is REFUSED and stays
# stale: that one needs /ze-rfc-audit <rfc>. Run `make ze-rfc-index-update` afterwards.
ze-rfc-reseal:
	@python3 scripts/dev/rfc_requirements.py --reseal

# Write an UNCLASSIFIED extraction skeleton for one RFC
# (plan/spec-rfcgate-1-extraction.md): every normative site and every section of
# rfc/full/<stem>.txt, with each disposition null. A reviewer then classifies each one by
# hand in rfc/extraction/<stem>.json. Generation alone can never produce a passing
# sign-off -- an unclassified site FAILS `make ze-rfc-check` -- so mass-generating
# skeletons makes the gate redder, never greener.
#   make ze-rfc-extraction-create STEM=rfc7296
ze-rfc-extraction-create:
	@test -n "$(STEM)" || { echo "usage: make ze-rfc-extraction-create STEM=<rfc-stem>"; exit 2; }
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
ze-yang-leaf-mentions-report:
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

# Weakened-test record (plan/spec-weakened-per-commit.md): a commit that weakens a
# test carries the reason in test/weakened.md, and scripts/dev/commit_helper.py
# refuses the commit when a weakening has no row. Whether a commit is covered is a
# question about THAT commit's paths, and a verify stage has none, so this stage
# checks the one thing that is true for every session in a shared checkout: the
# file the commit gate depends on still parses. A header that drifted would leave
# the gate reading no rows.
# --selftest first proves the checker still refuses a weakening with no row, and
# accepts the same weakening once a row names it, on a fixture repository whose
# answer is known.
ze-test-weakened-check:
	@python3 scripts/dev/check_weakened_tests.py --selftest
	@python3 scripts/dev/check_weakened_tests.py

# Entry point for the Staticcheck feature-tag matrix gate.
#
# ARGS passes the checker's own flags:
#   ARGS=--print-matrix    print the rows this invocation would judge, and stop
#   ARGS=--deadline=D      bound the one Staticcheck process
#
# ZE_VERIFY_SCOPE_TAGS names the feature-tag answer a verify run published
# (scripts/status/verify_run.go), and scopes the rows to the ones that change
# set can move. Unset -- a developer running this target on its own -- judges
# every row the manifest implies.
ze-staticcheck-feature-matrix-check:
	@$(GO) run scripts/checks/staticcheck_feature_matrix.go $(ARGS)

# The change-set selector (plan/spec-verify-scope-2-change-set-selector.md): one
# answer for "what must this change retest, and which features can it reach".
# The reverse import graph is built with every feature tag on, so a gated
# importer such as cmd/ze/hub's //go:build ze_ssh files is visible.
#
# ARGS passes the selector's own flags:
#   ARGS=--print=tags      the feature-tag answer alone
#   ARGS=--print=both      both answers, in one sectioned document
#   ARGS=--depth=1         a narrower reverse walk than the default 2
#   ARGS=--drop-log=FILE   record what the depth bound dropped
ze-verify-scope-selector:
	@$(GO) run scripts/checks/verify_scope_selector.go $(ARGS)

# Tracked-build gate (docs/architecture/testing/tracked-build-gate.md): compile
# the tree GIT HOLDS, which is the one population no other check compiles. Every
# other gate builds the working tree, so a consumer committed without its
# producer is green for its author and broken for anybody who builds the commit
# -- four commits broke `make ze-build` at HEAD that way on 2026-08-04.
#
# --selftest first proves the two vacuity guards still fire: `go build ./...`
# exits 0 over a pattern that matched nothing buildable, so a flavor that
# compiled zero packages would otherwise report success.
#
# REV=<commit-ish> judges another commit (`make ze-repository-tracked-build-check REV=7abe8a07e`).
# The extracted tree is removed at the end; add ARGS=--keep to inspect it.
ze-repository-tracked-build-check:
	@$(GO) run scripts/checks/tracked_build.go --selftest
	@$(GO) run scripts/checks/tracked_build.go $(if $(REV),--rev=$(REV)) $(ARGS)

# Regenerate the testing-state page (docs/features/test-health.md), its structured
# sibling test/health/latest.json, and the ratchet baseline. Output is a pure
# function of committed state -- no wall-clock value -- so ze-test-health-check can
# gate it for staleness the way every other generated file here is gated.
ze-test-health-update:
	@python3 scripts/dev/testing_health.py --write

# Staleness gate for the above; a prerequisite of ze-generated-files-check.
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

ze-repository-check:
	@python3 scripts/dev/validate.py --root .

# The half of ze-repository-check that `make ze-precommit-verify` runs (stagesForMode,
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
# the checker run by hand (spec-session-scoped-build-artifacts: 10
# pre-existing findings pulled into one session's scope), and inside ze-precommit-verify
# it would red a run whose author changed none of it. Same reasoning as
# ze-ste-check, which stays out of ze-doc-verify for the same reason (mk/check-docs.mk).
#
# Declaring an EMPTY changed set is what selects the tree-wide three: both
# changed-file checks return no findings before reading anything, and the other
# three take --root and are untouched. Run `make ze-repository-check` to get all five
# over your own tree.
ze-repository-tree-check:
	@python3 scripts/dev/validate.py --root . --changed-file ''

ze-verify-all: ze-precommit-verify ze-chaos-verify
	@echo "All verification passed (ze + chaos)"

ze-test-all: ze-standard-test ze-chaos-verify
	@echo "All tests passed (ze + chaos + fuzz)"

ze-smoke-verify: ze-lint ze-unit-test build
	@echo "Ze smoke check passed"

ze-ci-verify: ze-smoke-verify

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

ze-vendor-web-sync:
	@go run scripts/vendor/sync_web.go

# The staleness gate for the vendor sync in `generate`, and a `ze-precommit-verify` stage.
# It reads two directory trees and no network, so it runs in an offline CI and
# in an offline checkout. `ze-vendor-web-update-report` is where the npm
# registry query lives.
ze-vendor-web-check:
	@go run scripts/vendor/check_web.go

ze-vendor-web-update-report:
	@go run scripts/vendor/check_web.go --updates

# htmx 2 -> htmx 4 upgrade gate (plan/spec-web-htmx4-cutover, AC-13). It runs
# htmx's OWN scanner, vendored at third_party/web/htmx-upgrade-check.py, over
# every package that embeds htmx, and refuses an issue no row of
# scripts/dev/htmx-upgrade-explained.txt accounts for.
#
# The scanner builds a DOM, so it reads the inheritance carriers a text search
# cannot: an attribute htmx 2 inherited down the tree needs the :inherited
# suffix in htmx 4, and only a parser knows whether a DESCENDANT issues a
# request. --report prints every issue, explained or not, and exits 0.
ze-htmx-upgrade-check:
	@python3 scripts/dev/htmx_upgrade_check.py --check

ze-htmx-upgrade-report:
	@python3 scripts/dev/htmx_upgrade_check.py --report

ze-arch-map-update:
	@python3 scripts/dev/arch_map.py

ze-ai-instructions-generate: ze-arch-map-update
	@sed 's/{{TOOL}}/Claude/' ai/INSTRUCTIONS.md > CLAUDE.md
	@sed 's/{{TOOL}}/Codex/' ai/INSTRUCTIONS.md > AGENTS.md

ze-ai-skills-sync:
	@scripts/dev/skill_sync.sh

# CLAUDE.md / AGENTS.md / skills mirrors are gitignored: git diff can NEVER
# show drift for them. ze-ai-sync-check compares content against a fresh
# generation instead. The session-start hook runs it and warns when stale.
ze-ai-sync-check:
	@scripts/dev/skill_sync.sh --check

ze-generated-files-update: generate ze-rules-render-update ze-rules-condensed-update ze-ai-instructions-generate ze-ai-skills-sync ze-doc-index-update ze-rules-index-update ze-discovery-index-update ze-test-health-update
	@echo "All generated files updated"

# Write-safe twin of ze-generated-files-reconcile, and the ONLY one wired into verify
# (stagesForMode in scripts/status/verify_run.go, both branches).
#
# ze-generated-files-reconcile below cannot go into verify: its `ze-generated-files-update` prerequisite
# REWRITES every generated file and only then diffs, so `make ze-precommit-verify` would
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
# `generate` / `ze-generated-files-update` recipes and fails when a generator gains no check.
#
# Generator -> output -> covering prerequisite:
#   plugin_imports.go -> plugin/all/all.go                   -> ze-plugin-imports-check
#   yang_glue.go      -> yang/*/register.go, embed.go        -> ze-yang-glue-check
#   feature_tags.go   -> .golangci.yml, gokrazy/ze/config.json,
#                        docs/guide/quickstart.md            -> ze-feature-tags-check
#   templ generate    -> internal/**/*_templ.go              -> ze-templ-output-check
#   fuzz-targets.py   -> mk/test-fuzz-targets.mk             -> ze-fuzz-targets-check
#   sync_web.go       -> internal/**/assets/<vendored file>  -> ze-vendor-web-check
#   web_assets.go     -> internal/**/page_assets.go           -> ze-web-assets-check
#   code_to_docs.py   -> docs/ `<!-- source: -->` anchors    -> ze-doc-index-check
#                        (the anchors, NOT ai/CODE-TO-DOCS.md: see exclusion 3)
#   rules_points.py   -> ai/rules/<rule>.md (from ai/rules/points/) -> ze-rules-render-check
#   rules_index.py    -> ai/rules/INDEX.md                   -> ze-rules-index-check
#   rules_condensed.py-> ai/rules/TRIGGERS.md, ai/rules/CORE.md -> ze-rules-condensed-check
#   arch_map.py       -> arch lists in ai/INSTRUCTIONS.md    -> ze-arch-map-check
#   package_map.py    -> ai/PACKAGE-MAP.md                   -> ze-discovery-index-check
#
# THREE DELIBERATE EXCLUSIONS. The first two would break CI or duplicate an
# earlier stage; the third has nothing left to check:
#
#   skill_sync.sh --check (CLAUDE.md, AGENTS.md, .claude|.codex|.agents/skills).
#   Every one of its targets is GITIGNORED (.gitignore), so they do not exist at
#   all in the fresh checkout CI runs `make ze-precommit-verify` against, and the check
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
#   check_doc_links.py --md-only, which ze-generated-files-reconcile's recipe ends with. It
#   checks references, not generated-file staleness, and ze-doc-links-check runs the
#   FULL check (a strict superset) in the stage slot immediately before this one.
#
#   The freshness of ai/DOCS-TO-CODE.md and ai/CODE-TO-DOCS.md. Both are now
#   GITIGNORED, for the same reason the skill mirrors above are: nothing
#   committed can drift, so there is nothing for CI to catch, and the check
#   would instead fail on a fresh checkout where the derived file was never
#   generated. They are 8900 lines of pure derivation that no code reads,
#   they rebuild in under 3 seconds, and tracking them put a diff on 13% of
#   commits and a rebase conflict on every concurrent append.
#   `ze-generated-files-update` still regenerates both, and .claude/hooks/session-start.sh
#   builds them when missing. What ze-doc-index-check still enforces is the part
#   a generated file cannot answer for itself: that every `<!-- source: -->`
#   anchor under docs/ points at a real path and names a symbol that file
#   declares. Do not "fix" this by wiring the freshness half back in.
ze-arch-map-check:
	@python3 scripts/dev/arch_map.py --check

ze-generated-files-check: ze-plugin-imports-check ze-yang-glue-check ze-feature-tags-check ze-templ-output-check ze-fuzz-targets-check ze-vendor-web-check ze-web-assets-check ze-doc-index-check ze-rules-render-check ze-rules-index-check ze-rules-condensed-check ze-rules-lint ze-arch-map-check ze-discovery-index-check ze-test-health-check
	@echo "All generated files are up to date"

ze-generated-files-reconcile: ze-generated-files-update
	@if ! git diff --quiet -- ai/CODE-TO-DOCS.md ':(glob)ai/rules/*.md' ai/PACKAGE-MAP.md ai/DOCS-TO-CODE.md internal/component/plugin/all/all.go .golangci.yml gokrazy/ze/config.json docs/guide/quickstart.md mk/test-fuzz-targets.mk ':(glob)internal/**/*_templ.go' 2>/dev/null; then \
		echo "ERROR: Generated files are stale. Run 'make ze-generated-files-update' and commit the result." >&2; \
		git diff --stat -- ai/CODE-TO-DOCS.md ':(glob)ai/rules/*.md' ai/PACKAGE-MAP.md ai/DOCS-TO-CODE.md internal/component/plugin/all/all.go .golangci.yml gokrazy/ze/config.json docs/guide/quickstart.md mk/test-fuzz-targets.mk ':(glob)internal/**/*_templ.go'; \
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

ze-doc-links-check:
	@python3 scripts/dev/check_doc_links.py

# clean removes bin/, coverage, THIS session's WHOLE directory
# tmp/session/<YYYY-MM-DD>-<session-id>/ (via scripts/dev/session-scratch.sh --clean),
# the directories of sessions that are no longer running (ze-session-reap), and both
# Go build caches.
#
# --clean takes all three subdirectories and not only scratch/: the session's binaries
# and the etc/ze store they resolve, its scratch/, and its state/ per-spec digest.
# Losing the digest costs the spec handoff .claude/rules/post-compaction.md reads, so
# run `make clean` between specs rather than inside one.
#
# A CONCURRENT session keeps its own directory: ze-session-reap removes a directory
# only when it can show the session that owns it exited, and it errs toward keeping
# (scripts/dev/session-reap.py). bin/, coverage and the caches are shared, rebuildable
# outputs, so those go whoever runs this.
#
# Two Go caches are emptied because there are two: GOCACHE is overridden to
# $(CURDIR)/cache/go-cache at the top of this file, so a plain `go clean -cache`
# reaches that one and never the default user cache. `env -u GOCACHE` drops the
# override so the second run reaches ~/.cache/go-build. mk/schedule-cadence.mk runs the same
# pair every morning.
#
# For the full per-checkout wipe (bin/ + ALL of tmp/, every session's directory
# included, running or not) use `make clean-all`. See
# plan/spec-relocate-scratch-and-cache.md.
clean:
	@echo "Cleaning this session (bin/, coverage, this session's whole directory: bin/, scratch/, state/)..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	@scripts/dev/session-scratch.sh --clean 2>/dev/null || true
	@$(MAKE) --no-print-directory ze-session-reap
	@echo "Emptying both Go build caches (repository override, then the default user cache)..."
	go clean -cache
	env -u GOCACHE go clean -cache
	@python3 scripts/dev/ensure-links.py --quiet

# clean-all is the full per-checkout wipe: bin/ + the SCRATCH contents (all of tmp/,
# which is every session's state) + both Go build caches, then it re-ensures the
# symlinks.
#
# The durable cache/ IS emptied here (owner directive, 2026-08-20). It holds GOCACHE
# (line 20: cache/ -> ~/.cache/ze), and that is the one tree in this checkout that
# grows without a bound: it reached 261G of a 295G device and turned eight verify
# stages red with `no space left on device`, twice
# (plan/journal/full-disk-false-red.md, fourth occurrence). A full wipe that leaves
# the only directory able to fill a disk is not a full wipe.
#
# Destructive under concurrency: it removes sibling sessions' scratch and the shared
# caches -- use `make clean` for the everyday, session-safe clean.
clean-all:
	@echo "Cleaning EVERYTHING (bin/ + all of tmp/, both Go build caches)..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	@if [ -e tmp ]; then find tmp/ -mindepth 1 -maxdepth 1 -exec rm -rf {} + 2>/dev/null || true; fi
	@echo "Emptying both Go build caches (repository override, then the default user cache)..."
	go clean -cache
	env -u GOCACHE go clean -cache
	@python3 scripts/dev/ensure-links.py --quiet

# go.mod is EXCLUDED from the file reap below: tmp/go.mod is the TRACKED sentinel
# that keeps `go list ./...` out of the caches under tmp/ (SENTINEL in
# scripts/dev/ensure-links.py). Being committed, it is always older than 24h, so
# an unqualified reap silently deletes a tracked file. Keep this exclusion in
# sync with the identical one in .claude/hooks/session-start.sh.
# -mindepth 1 on the directory sweep is load-bearing, not decoration. `find tmp/`
# yields tmp/ ITSELF at depth 0, and its basename is `tmp`, so neither -not -name
# excludes it: without the bound, a tmp/ whose own mtime is older than 24h is a
# match and `rm -rf` takes the entire scratch tree, tmp/session/ and every
# session's binaries, seeded store and state/ digest with it. The exclusions
# below only ever protect tmp/session/ and tmp/kernel/ from being removed as
# CHILDREN. Reproduced in a fixture, 2026-08-10.
ze-scratch-clean:
	@echo "Cleaning tmp/ scratch files older than 24h..."
	@find tmp/ -maxdepth 1 -type f -not -name go.mod -mmin +1440 -delete 2>/dev/null || true
	@find tmp/ -mindepth 1 -maxdepth 1 -type d -not -name session -not -name kernel \
		-mmin +1440 -exec rm -rf {} + 2>/dev/null || true
	@echo "Done. $$(ls -1 tmp/ 2>/dev/null | wc -l | tr -d ' ') entries remain."

# tmp/session/ is EXCLUDED above and swept by nothing else. Nothing under that
# root is ever removed automatically -- not at session end, not on an age timer,
# not by a hook (owner decision, 2026-08-03). It holds a session's binaries, its
# scratch, and the markers that carry a spec claim across a compaction, and an
# automatic sweep of any of them deletes the operator's own data unasked.
#
# ze-session-clean is the operator's route, and BEFORE is what makes it one: a
# date the person types, never a default and never "now minus a window". The
# YYYY-MM-DD- prefix every session directory carries turns "older than" into an
# integer comparison, so the target needs no clock and no stat.
#
# Three properties bound what it can remove, and scripts/dev/session_bin_dir_test.py
# drives the target itself to prove each one:
#   1. The glob is $(ZE_SESSION_ROOT)/????-??-??-*, so the DATED SHAPE is what
#      bounds a candidate. A flat marker file beside them (.sid-by-pid-*,
#      .closure-ack-*, .session-*) matches neither the shape nor the -d test, and
#      `.` and `..` cannot match the shape at all. The shape is the bound, not
#      the root: ZE_SESSION_ROOT is a plain `:=`, so a caller can point it
#      elsewhere, and session_bin_dir_test.py's CleanSessionsCase does exactly
#      that to drive the real target against a temporary tree. Read property 1 as
#      "only a dated directory inside the root it is given".
#   2. Without BEFORE the target exits 2 having removed nothing, so a bare
#      invocation, or a typo that empties the variable, deletes no session.
#   3. The comparison is strict, so a directory dated exactly BEFORE survives.
#
# BEFORE reaches the recipe through the ENVIRONMENT ($$BEFORE), never spliced
# into the shell text as $(BEFORE). Interpolating it put the operator's typing
# inside a double-quoted shell literal BEFORE the format check could see it, so
# `make ze-session-clean 'BEFORE=";touch pwn;x"'` ran that touch and only then
# reported a bad date. mk/helper-session.mk refuses a quote in the session id for this
# exact reason one file away. `export` is what makes $$BEFORE readable here.
#
# That closes the SHELL layer. Make's own layer sits above it and is not closed:
# `export` expands the value, so `BEFORE=$(shell …)` runs at that point, before
# any check. mk/helper-session.mk records the same residual for ZE_SESSION_ID, and the
# same reason applies -- BEFORE is the operator's own command line, so it crosses
# no privilege boundary. $(ZE_SESSION_ROOT) below is spliced for the same reason:
# it is a test seam a caller sets deliberately.
export BEFORE
ze-session-clean:
	@if [ -z "$$BEFORE" ]; then \
		echo "ze-session-clean: BEFORE=<YYYY-MM-DD> is required, and removes session directories dated strictly before it"; \
		echo "  example: make ze-session-clean BEFORE=$$(date +%Y-%m-01)"; \
		exit 2; \
	fi
	@case "$$BEFORE" in \
		[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]) ;; \
		*) echo "ze-session-clean: BEFORE must be YYYY-MM-DD, got '$$BEFORE'"; exit 2 ;; \
	esac
	@cutoff=$$(echo "$$BEFORE" | tr -d -); removed=0; \
	for d in $(ZE_SESSION_ROOT)/????-??-??-*; do \
		[ -d "$$d" ] || continue; \
		stamp=$$(basename "$$d" | cut -c1-10 | tr -d -); \
		[ "$$stamp" -lt "$$cutoff" ] 2>/dev/null || continue; \
		rm -rf "$$d"; \
		removed=$$((removed + 1)); \
	done; \
	echo "Removed $$removed session director$$([ $$removed -eq 1 ] && echo y || echo ies) dated before $$BEFORE."

# ze-session-reap asks the question ze-session-clean cannot: is the session that
# owns this directory still RUNNING? A date says nothing about that, which is why
# the age timer was refused in the first place -- a directory dated last week can
# belong to a session still open in a terminal. A directory whose owner exited
# holds nothing anybody can return to, so this one needs no date from the operator.
#
# It keeps on four independent signals and removes only when every one of them is
# silent, so a session it cannot classify survives. `make clean` runs it, and
# DRY=1 names what would go without removing it. The signals, the proof that a
# running session always clears them, and the fail-closed case are in
# scripts/dev/session-reap.py.
ze-session-reap:
	@python3 scripts/dev/session-reap.py $(if $(DRY),--dry-run)

check: fmt vet
	@echo "Quick check passed"

# ─── Setup ──────────────────────────────────────────────────────────────────

# ─── Help ───────────────────────────────────────────────────────────────────

help:
	@echo "Ze Network OS -- Build & Test"
	@echo ""
	@echo "  Start here (new contributor):"
	@echo "    ./le setup                                 Full dev setup: build deps, linters, appliance tools (one-time)"
	@echo "    ./le setup --check                         Probe only: list missing tools, exit nonzero if any required"
	@echo "    ./le lint                                  Lint and type-check the Python half of the tree"
	@echo "    make ze-smoke-verify                       Verify setup: lint + unit + build (~2 min)"
	@echo ""
	@echo "  Daily development:"
	@echo "    make ze-unit-bgp-test                      Unit tests for one component group ($(GO_TEST_RACE_LABEL))"
	@echo "                                               Also: ze-unit-core-test, ze-unit-plugins-test, ze-unit-config-test, ze-unit-cli-test, ze-unit-rest-test"
	@echo "    make ze-functional-encode-test             Single functional suite (encode, plugin, decode, parse, reload, ...)"
	@echo "    make ze-precommit-verify                   Pre-commit gate: lint + wiring/docs + SCA + unit + functional + exabgp (4-10 min)"
	@echo "    make ze-precommit-verify-changed           Scoped verify: changed packages + wiring/docs + SCA, then full functional"
	@echo ""
	@echo "  Cadence -- the checks ze-precommit-verify and the nightly workflows do NOT run:"
	@echo "    make ze-cadence-daily-run                  Seconds. Run this one every morning"
	@echo "    make ze-cadence-weekly-run                 Minutes. Takes the verify lock; not beside a verify"
	@echo "    make ze-cadence-monthly-run                Docker/QEMU/root. Members report rather than gate"
	@echo ""
	@echo "  Build:"
	@echo "    make build                                 All binaries (ze, ze-stripped, ze-test, ze-chaos, ze-perf, ze-analyze)"
	@echo "    make ze-build                              Just $(ZEBIN_ZE)"
	@echo "    make ze-stripped-build                     Just $(ZEBIN_STRIPPED)"
	@echo "    make ze-session-binary-path                Print this session's ze path (never hardcode bin/ze)"
	@echo ""
	@echo "  More help:"
	@echo "    make help-test                             Common test targets (unit, functional, fuzz, chaos, interop, ...)"
	@echo "    make help-deploy                           Gokrazy appliance, kernel builds, deployment evidence"
	@echo "    make help-dev                              Inventory, doc validation, spec status, utilities"
	@echo ""
	@echo "  See also: docs/contributing/testing.md"

help-test:
	@echo "Ze Test Targets"
	@echo ""
	@echo "  Unit tests ($(GO_TEST_RACE_LABEL)):"
	@echo "    ze-unit-test                               All packages (~5 min)"
	@echo "    ze-unit-test-coverage                      All packages with coverage report"
	@echo "    ze-unit-bgp-test                           BGP component group (~1:30)"
	@echo "    ze-unit-core-test                          Core libraries (~30s)"
	@echo "    ze-unit-plugins-test                       Plugins (~40s)"
	@echo "    ze-unit-config-test                        Config component (~20s)"
	@echo "    ze-unit-cli-test                           CLI component (~10s)"
	@echo "    ze-unit-rest-test                          Everything else (~1:00)"
	@echo "    ze-unit-reactor-test-race                  Stress-test reactor ($(GO_TEST_RACE_LABEL), count=20)"
	@echo ""
	@echo "  Functional tests (.ci suites via $(ZEBIN_TEST)):"
	@echo "    ze-functional-test                         All 24 gating suites"
	@echo "    ze-functional-encode-test                  BGP wire encoding"
	@echo "    ze-functional-plugin-test                  Plugin behavior"
	@echo "    ze-functional-decode-test                  Wire decoding"
	@echo "    ze-functional-parse-test                   Config parsing"
	@echo "    ze-functional-reload-test                  Config reload"
	@echo "    ze-functional-ui-test                      CLI/completion"
	@echo "    ze-functional-editor-test                  TUI editor (.et files)"
	@echo "    ze-functional-managed-test                 Managed config"
	@echo "    ze-functional-web-test                     Web UI (browser)"
	@echo "    ze-functional-l2tp-test                    L2TP"
	@echo "    ze-functional-firewall-test                Firewall"
	@echo "    ze-functional-policy-test                  Policy routing"
	@echo "    ze-functional-ospf-test                    OSPF config and doctor"
	@echo "    ---"
	@echo "    ze-functional-static-test                  Static routes (release evidence only)"
	@echo "    ze-functional-traffic-test                 Traffic control (release evidence only)"
	@echo "    ze-functional-vpp-test                     VPP stub (release evidence only)"
	@echo "    ze-functional-l2tp-wire-test               L2TP wire-level (release evidence only)"
	@echo "    ze-functional-isis-wire-test               IS-IS wire-level decode (release evidence only)"
	@echo "    ze-functional-ospf-wire-test               OSPFv2 wire-level decode (release evidence only)"
	@echo ""
	@echo "  Functional tests run against an ISOLATED binary set by default"
	@echo "  (tmp/testbin-<pid>/, removed on exit), so a running suite never touches"
	@echo "  $(ZEBIN_ZE) and you can keep building/editing while it runs."
	@echo "    ZE_SUFFIX=<name>          Pin a stable, kept dir (tmp/testbin-<name>/)"
	@echo "    ZE_TEST_CANONICAL=1                        Opt out: rebuild $(ZEBIN_ZE) + $(ZEBIN_TEST) in place"
	@echo ""
	@echo "  Fuzz:"
	@echo "    ze-fuzz-test                               All fuzz targets (10s each)"
	@echo "    ze-fuzz-test-one                           Single target (FUZZ=name PKG=path TIME=30s)"
	@echo ""
	@echo "  Chaos:"
	@echo "    ze-chaos-test                              Unit + functional + integration + web"
	@echo "    ze-chaos-verify                            Lint + all chaos tests"
	@echo "    ze-chaos-functional-test                   In-process BGP chaos simulation"
	@echo "                                               Options: CHAOS_SEED=N CHAOS_DURATION=60s CHAOS_PEERS=8"
	@echo "    ze-chaos-integration-test                  End-to-end: Ze + chaos peers (.ci tests)"
	@echo ""
	@echo "  ExaBGP compatibility:"
	@echo "    ze-functional-exabgp-test                  Ze encoding matches ExaBGP (ZE_EXABGP_TIMEOUT=180)"
	@echo ""
	@echo "  Interop (Docker):"
	@echo "    ze-interop-test                            FRR/BIRD interop (INTEROP_SCENARIO=name for one)"
	@echo "    ze-interop-ipsec-test                      strongSwan IKEv2 (IPSEC_INTEROP_SCENARIO=name)"
	@echo ""
	@echo "  netlab (needs netlab installed):"
	@echo "    ze-netlab-render-check                     contrib/netlab renders to golden (ARGS=--update, NETLAB=path)"
	@echo ""
	@echo "  Stress (Linux, root, netns):"
	@echo "    ze-stress-test                             BGP stress with ze-test peer injector"
	@echo "    ze-stress-bird-test                        BIRD baseline comparison"
	@echo "    ze-stress-profile                          1M route profiling (saves pprof to tmp/)"
	@echo ""
	@echo "  Integration (CAP_NET_ADMIN / root):"
	@echo "    ze-integration-test                        All netns integration tests"
	@echo "    ze-integration-iface-test                  iface netlink"
	@echo "    ze-integration-fib-test                    FIB kernel routes"
	@echo "    ze-integration-firewall-test               nftables"
	@echo "    ze-integration-traffic-test                tc qdisc"
	@echo "    ze-netns-test                              firewall/policy/ospf/ospfv3 .ci in a per-test netns"
	@echo "    ze-netns-plugin-test                       plugin .ci needing CAP_SYSLOG (/dev/kmsg)"
	@echo ""
	@echo "  QEMU (macOS-friendly, no Docker Desktop kernel limits):"
	@echo "    ze-qemu-test-all                           FULL suite in QEMU: functional + unit + integration (host-compiled)"
	@echo "    ze-qemu-debug RUN=...                      Run specific tests verbosely in QEMU (RUN='bin/ze-test-linux-arm64 bgp parse 91 -v')"
	@echo "    ze-qemu-shell                              Persistent QEMU VM for interactive debugging (run in background)"
	@echo "    ze-qemu-integration-test                   Integration tests in QEMU Alpine VM"
	@echo "    ze-qemu-l2tp-ppp-test                      L2TP PPP/NCP in QEMU with gokrazy kernel"
	@echo "    ze-qemu-pppoe-accel-test                   PPPoE client vs accel-ppp AC in QEMU"
	@echo "    ze-qemu-pppoe-test                         PPPoE access-concentrator .ci suite in QEMU (netns + runtime kernel)"
	@echo "    ze-qemu-traffic-usage-test                 traffic-usage eBPF TCX accounting in QEMU"
	@echo "    ze-qemu-ldp-frr-test                       LDP interop against FRR ldpd in QEMU"
	@echo "    ze-qemu-isis-frr-test                      IS-IS interop against FRR isisd in QEMU"
	@echo "    ze-qemu-vrrp-keepalived-test               VRRP interop against keepalived in QEMU"
	@echo ""
	@echo "  Live (Docker + internet):"
	@echo "    ze-live-test                               All live tests"
	@echo "    ze-live-rpki-test                          RPKI with real-world data (stayrtr)"
	@echo ""
	@echo "  Linux-only Go tests:"
	@echo "    ze-unit-linux-test                         Run in Docker (ZE_LINUX_TEST_PACKAGES=...)"
	@echo ""
	@echo "  Performance benchmarks (Docker):"
	@echo "    ze-perf-bench                              Run against all DUTs (PERF_DUT=name for one)"
	@echo "    ze-perf-report                             Generate comparison report"
	@echo "    ze-perf-history-record                     Update history tracking"
	@echo ""
	@echo "  Test health (would a regression be caught, not how many tests exist):"
	@echo "    ze-test-health-update                      Regenerate docs/features/test-health.md + test/health/"
	@echo "    ze-test-health-check                       Fail if a structural fact drifted (in ze-precommit-verify)"
	@echo "    ze-test-health-record                      Append one KPI sample to the committed history"
	@echo "    ze-test-sensitivity-check                  Ratchet: tests that cannot fail, files no target runs"
	@echo "    ze-test-weakened-check                     Check test/weakened.md parses for the commit gate"
	@echo ""
	@echo "  Composite targets:"
	@echo "    ze-smoke-verify                            Lint + unit + build (~2 min)"
	@echo "    ze-precommit-verify                        Pre-commit gate: lint + wiring/docs + SCA + unit (two-pass) + functional + exabgp"
	@echo "    ze-precommit-verify-changed                Scoped: changed packages + wiring/docs + SCA + functional + exabgp"
	@echo "    ze-repository-check                        All five checks: source anchors, wiring, spec completeness (~0.2s)"
	@echo "    ze-repository-tree-check                   The three tree-wide checks of ze-repository-check; runs inside ze-precommit-verify"
	@echo "    ze-standard-test                           All standard Ze tests including fuzz"
	@echo "    ze-verify-all                              ze-precommit-verify + ze-chaos-verify"
	@echo "    ze-test-all                                ze-standard-test + ze-chaos-verify"
	@echo ""
	@echo "  Release evidence (external infra required):"
	@echo "    ze-evidence-functional-test                Run non-gating functional suites as release evidence"
	@echo "    ze-evidence-perf-record                    Record perf evidence and gate regressions"
	@echo "    ze-evidence-release-candidate-check        Clean release-candidate verification in Docker"
	@echo "    ze-evidence-release-preflight              Check required tooling (Docker, QEMU)"
	@echo "    ze-evidence-release-verify                 Full matrix: interop + chaos + fuzz + perf + QEMU + deploy"
	@echo "    ze-release-assets-update                   Rebuild every release-owned website asset"
	@echo "    ze-terminal-demo-release-render-all        Re-record all terminal demos for this release"
	@echo ""
	@echo "  Escalation: single test -> package -> component group -> ze-precommit-verify"
	@echo "  See docs/contributing/testing.md for the full workflow."

help-deploy:
	@echo "Ze Deployment Targets"
	@echo ""
	@echo "  Appliance installer ISO (from JSON config):"
	@echo "    ze-iso-build-full                          Full build from config: init + kernel + initrd + image + ISO"
	@echo "                                               CONFIG=prod.json SSH_PASSWORD='...'"
	@echo "    ze-iso-build                               Rebuild (appliance already initialized)"
	@echo "                                               NAME=prod APPLIANCE_BUILDER=docker"
	@echo "    ze-iso-check                               Check ISO build prerequisites"
	@echo "    ze-pxe-build                               Build iPXE + TFTP for PXE boot"
	@echo "                                               NAME=prod PXE_DIR=build/pxe"
	@echo ""
	@echo "  Docker:"
	@echo "    ze-docker-build                            Build deployment image, scratch base (ZE_DOCKER_IMAGE=ze ZE_DOCKER_TAG=...)"
	@echo "    ze-docker-lab-build                        Build lab image for netlab/containerlab, alpine base (ZE_LAB_IMAGE=netlab/ze ZE_LAB_TAG=latest)"
	@echo ""
	@echo "  Gokrazy VM appliance (see docs/guide/appliance.md):"
	@echo "    ze-gokrazy-deps-download                   One-time: download gokrazy system packages"
	@echo "    ze-gokrazy-build USER=x PASS=y             Build bootable VM image"
	@echo "    ze-gokrazy-run                             Boot in QEMU (Ctrl-A X to quit)"
	@echo "                                               GOKRAZY_ARCH=arm64 for Apple Silicon"
	@echo ""
	@echo "  Custom kernel:"
	@echo "    ze-kernel-build KERNEL_ARCH=amd64          Build/materialize the runtime kernel (L2TP/PPP built in; ~30 min cold via KERNEL_BUILDER=docker, instant from cache)"
	@echo "    ze-kernel-vmlinuz-stage KERNEL_ARCH=amd64  Stage tmp/kernel/vmlinuz only (what the QEMU targets need; no gokrazy module cache)"
	@echo "    ze-kernel-clean                            Restore pinned rtr7/kernel"
	@echo ""
	@echo "  Deployment evidence:"
	@echo "    ze-deployment-vpp-test                     Real VPP daemon in Docker"
	@echo "    ze-deployment-l2tp-test                    External L2TP peer in Docker"
	@echo "    ze-deployment-l2tp-ppp-test                Full PPP/NCP in Linux netns"
	@echo "    ze-deployment-docker-l2tp-ppp-test         PPP/NCP Docker lab (Ze LNS + LAC + FRR)"
	@echo "    ze-deployment-docker-pppoe-accel-test      PPPoE Docker lab (Ze client + accel-ppp AC)"
	@echo "    ze-deployment-gokrazy-l2tp-ppp-test        PPP against QEMU appliance"
	@echo "    ze-evidence-docker-run EVIDENCE_SCRIPT=... EVIDENCE_PACKAGES=..."
	@echo "    ze-deployment-preflight                    Check deployment tooling availability"

help-dev:
	@echo "Ze Development Tools"
	@echo ""
	@echo "  Inventory:"
	@echo "    ze-inventory                               Plugins, YANG, RPCs, tests, packages"
	@echo "    ze-inventory-json                          Same as JSON"
	@echo "    ze-command-list                            All registered commands by verb"
	@echo "    ze-command-list-json                       Same as JSON"
	@echo ""
	@echo "  Wiki:"
	@echo "    ze-wiki-update                             Regenerate all auto-generated wiki pages"
	@echo "    ze-wiki-commands-update                    Regenerate wiki/command-reference.md"
	@echo ""
	@echo "  Documentation validation:"
	@echo "    ze-doc-verify                              All doc checks (drift + anchors + YANG/handler contract)"
	@echo "    ze-doc-drift-check                         Docs claims vs live registry/Makefile/filesystem"
	@echo "    ze-doc-index-update                        Regenerate ai/CODE-TO-DOCS.md (code->docs reverse index)"
	@echo "    ze-rules-index-update                      Regenerate ai/rules/INDEX.md (one-line overview of every rule)"
	@echo "    ze-command-contract-check                  YANG command tree vs registered handlers"
	@echo "    ze-consistency-check                       Code/doc consistency: design refs, cross-refs, stale refs"
	@echo "    ze-doc-wiring-check                        Changed-file-aware wiring, docs, command, and inventory gate"
	@echo ""
	@echo "  Commit integrity:"
	@echo "    ze-staticcheck-feature-matrix-check        Type-check the working tree across feature-tag configurations"
	@echo "    ze-repository-tracked-build-check          Compile the tree GIT HOLDS (REV=<sha> for another commit)"
	@echo "    ze-verify-scope-selector                   Packages to retest and features reached by the change set"
	@echo ""
	@echo "  Spec management:"
	@echo "    ze-spec-status                             Spec inventory with progress status"
	@echo "    ze-spec-status-json                        Same as JSON"
	@echo ""
	@echo "  Module / protobuf:"
	@echo "    ze-proto-generate                          Regenerate api/proto/*.pb.go from ze.proto (needs protoc)"
	@echo "    (rename the module path)  python3 scripts/dev/rename_module_path.py --to <module> --apply"
	@echo ""
	@echo "  Code:"
	@echo "    fmt                                        Format code (gofmt + goimports, excludes vendor/)"
	@echo "    vet                                        Run go vet"
	@echo "    tidy                                       Tidy go.mod"
	@echo "    check                                      Quick check (fmt + vet)"
	@echo ""
	@echo "  Supply chain (SCA):"
	@echo "    ze-dependency-vulnerability-check          govulncheck: live tool and vuln.go.dev DB; verification plus daily CI"
	@echo ""
	@echo "  Generated files:"
	@echo "    ze-generated-files-update                  Regenerate all generated files"
	@echo "    ze-generated-files-reconcile               Regenerate tracked outputs, then fail if any were stale"
	@echo "    ze-generated-files-check                   Same verdict, writes nothing (this is what ze-precommit-verify runs)"
	@echo "    ze-precommit-verify-list                   Print the ze-precommit-verify stage list (never use make -n ze-precommit-verify)"
	@echo "    ze-plugin-imports-check                    Verify generated plugin blank imports are current"
	@echo "    ze-templ-output-check                      Verify templ outputs without rewriting them"
	@echo "    ze-web-assets-check                        Gate: each page's generated asset set matches its markup"
	@echo "    ze-site-generate                           Generate the public website into ../gh-pages"
	@echo "    ze-ai-instructions-generate                Generate CLAUDE.md and AGENTS.md"
	@echo "    ze-ai-skills-sync                          Sync canonical skills to tool directories"
	@echo "    ze-ai-sync-check                           Check generated agent files match canonical sources"
	@echo "    ze-doc-index-update                        Regenerate ai/CODE-TO-DOCS.md"
	@echo "    ze-rules-index-update                      Regenerate ai/rules/INDEX.md"
	@echo "    ze-vendor-web-sync                         Sync vendored web assets (also part of 'generate')"
	@echo "    ze-vendor-web-check                        Gate: each consumer asset copy matches third_party/web/"
	@echo "    ze-vendor-web-update-report                Ask the npm registry for newer web asset versions"
	@echo "    ze-htmx-upgrade-check                      Gate: htmx own scanner reports no unexplained htmx 4 issue"
	@echo "    ze-htmx-upgrade-report                     Print every htmx 4 upgrade issue, explained or not"
	@echo ""
	@echo "  Cleanup:"
	@echo "    clean                                      bin/, coverage, this session's scratch, exited sessions, both Go caches"
	@echo "    clean-all                                  Full wipe: bin/ + ALL of tmp/ (shared caches, all sessions, running or not)"
	@echo "    ze-scratch-clean                           Remove tmp/ scratch files older than 24h"
	@echo "    ze-session-clean                           Remove session dirs dated before BEFORE=<YYYY-MM-DD>"
	@echo "    ze-session-reap                            Remove session dirs whose Claude session exited (DRY=1 to list)"

# The `_<target>-impl` half of every admitted pair defined in this file.
# The public half calls the admission wrapper and this half holds the work;
# see the job-admission block above ZE_RUN_SLOTS in the Makefile.
.PHONY: _ze-web-golden-check-impl _ze-templ-port-check-impl _ze-web-golden-update-impl _ze-chaos-golden-update-impl _ze-plugin-snapshot-update-impl _ze-unit-reactor-test-race-impl _ze-unit-linux-test-impl _ze-functional-exabgp-test-impl _ze-dependency-vulnerability-check-impl _ze-lint-changed-impl _ze-unit-test-changed-impl _ze-unit-hook-test-impl
