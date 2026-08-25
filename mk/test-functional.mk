# Functional tests: .ci-based suites, run against an isolated test binary set.
#
# EVERY SUITE MOVED to `le`. They now live in scripts/le/application/functional.py,
# which carries the reasons this file had nowhere to put but a comment: the
# per-suite wall-clock budget and why the plugin one is 1500s, the concurrency
# derivation and the two measurements behind its floor and its cap, and what the
# isolated binary set buys. Each target below is a shim: it hands the work to the
# admission wrapper exactly as before, and the wrapper runs `le`.
#
#   ./le functional                            the 24 gating suites, in order
#   ./le functional --list                     every suite and what it covers
#   ./le functional ze-functional-encode-test  one of them
#
# WHAT STAYED, AND WHY. A Gate is one argv run without a shell, and two things
# here are not that:
#
#   ze-functional-test-warm  $(ZE_CI_GO_TEST_PKGS) is a `grep -rhoE | grep -oE |
#                            sort -u` pipeline over test/**/*.ci, in the same
#                            class as mk/test-unit.mk's $(ZE_PACKAGES). It is
#                            expanded lazily, so `le` never pays for a tree walk
#                            it does not need.
#   the $(ZE_TEST_DEPS*) block
#                            prerequisite EDGES. In canonical mode a suite needs
#                            the session's own binaries built first, and make
#                            building a prerequisite is a make-level concern.
#                            In the default isolated mode every list is empty and
#                            `le` builds the throwaway set itself.
#
# The `_<target>-impl` halves of the ported targets went with them, as they did
# in mk/test-unit.mk: a shim is one line and needs no second half.
#
# Quick reference:
#   make ze-functional-test               All 24 gating suites (le: GATING)
#   make ze-functional-encode-test        Encoding only
#   make ze-functional-plugin-test        Plugin behavior only
#   make ze-functional-decode-test        Wire decoding only
#   make ze-functional-parse-test         Config parsing only
#   make ze-functional-reload-test        Config reload only
#   make ze-functional-ui-test            CLI/completion only
#   make ze-functional-editor-test        TUI editor (.et files)
#   make ze-functional-managed-test       Managed config only
#   make ze-functional-web-test           Web UI only
#   make ze-functional-l2tp-test          L2TP only
#   make ze-functional-firewall-test      Firewall only
#   make ze-functional-policy-test        Policy routing only
#   make ze-functional-ipsec-test         IPsec/IKEv2 only
#   make ze-functional-ldp-test           LDP only
#   make ze-functional-rsvpte-test        RSVP-TE only
#   make ze-functional-install-test       Installer/PXE/kernel config only
#   make ze-functional-appliance-test     Appliance CLI (build/iso/list/serial-login) only
#   make ze-functional-runner-test        Test-runner primitives only
#   make ze-functional-isis-test          IS-IS config/doctor tests
#   make ze-functional-ospf-test          OSPF config/doctor tests
#   make ze-functional-ospfv3-test        OSPFv3 config/doctor tests
#   make ze-functional-vrrp-test          VRRP config/show/doctor tests
#   make ze-functional-static-test        Static routes (release evidence only)
#   make ze-functional-traffic-test       Traffic control (release evidence only)
#   make ze-functional-flow-export-test   Flow export sFlow/NetFlow/IPFIX (release evidence only)
#   make ze-functional-vpp-test           VPP stub (release evidence only)
#   make ze-functional-l2tp-wire-test     L2TP wire-level (release evidence only)
#   make ze-functional-isis-wire-test     IS-IS wire-level decode (release evidence only)
#   make ze-functional-ospf-wire-test     OSPFv2 wire-level decode (release evidence only)
#
# Every target here runs against an ISOLATED test binary set by default (built
# under $(ZE_SCRATCH_DIR)/testbin-<suffix>/ and removed on exit), so a running
# suite never touches the dev $(ZEBIN_ZE) and you can keep building/editing while
# it runs. ZE_SUFFIX=<name> pins a stable, kept directory; ZE_TEST_CANONICAL=1
# opts out. ZE_SUITE_TIMEOUT, ZE_SUITE_TIMEOUT_<SUITE>, ZE_SUITE_KILL_AFTER,
# ZE_SUITE_WARN_PERCENT, ZE_SKIP_SUITES, ZE_SUITE_CORES and ZE_COVER are read
# from the environment, and a make command-line variable lands there:
#   make ze-functional-test ZE_SUITE_TIMEOUT=1200s

.PHONY: ze-functional-test
.PHONY: ze-functional-encode-test ze-functional-plugin-test ze-functional-decode-test ze-functional-parse-test ze-functional-reload-test
.PHONY: ze-functional-ui-test ze-functional-editor-test ze-functional-web-test ze-functional-managed-test
.PHONY: ze-functional-l2tp-test ze-functional-firewall-test ze-functional-policy-test ze-functional-ipsec-test ze-functional-appliance-test ze-functional-runner-test
.PHONY: ze-functional-ldp-test ze-functional-rsvpte-test ze-functional-install-test
.PHONY: ze-functional-static-test ze-functional-traffic-test ze-functional-flow-export-test ze-functional-vpp-test ze-functional-l2tp-wire-test ze-functional-isis-wire-test ze-functional-ospf-wire-test ze-functional-isis-test ze-functional-ospf-test ze-functional-ospfv3-test ze-functional-vrrp-test

# ─── Prerequisite edges for the canonical (opt-out) mode ────────────────────
# ZE_TEST_CANONICAL=1 makes a suite run the session's own $(ZEBIN_TEST) in place
# and lets the runner rebuild it, which is what release and CI reproducibility
# ask for. Those binaries are make's to build, so the edges live here. In the
# DEFAULT isolated mode every list is empty: `le` builds ze, ze-test and
# ze-stripped into a throwaway directory of its own and freezes the runner
# against them with ZE_TEST_NO_BUILD=1.
ZE_TEST_CANONICAL ?=
ifeq ($(ZE_TEST_CANONICAL),)
  ZE_TEST_DEPS :=
  ZE_TEST_DEPS_STRIPPED :=
  ZE_TEST_DEPS_ZE :=
  ZE_TEST_DEPS_ALL :=
  ZE_WEB_CHAOS_DEP :=
else
  ZE_TEST_DEPS := $(ZEBIN_TEST)
  ZE_TEST_DEPS_STRIPPED := $(ZEBIN_TEST) $(ZEBIN_STRIPPED)
  ZE_TEST_DEPS_ZE := $(ZEBIN_ZE) $(ZEBIN_TEST)
  ZE_TEST_DEPS_ALL := $(ZEBIN_ZE) $(ZEBIN_STRIPPED) $(ZEBIN_TEST)
  ZE_WEB_CHAOS_DEP := $(ZEBIN_CHAOS)
endif

# The admission wrapper, plus the one value `le` must not re-derive: this
# session's own directory. make resolves it (mk/helper-session.mk) and hands it
# over, so the throwaway binary set and $(ZEBIN_TEST) cannot land under two
# different roots. Run outside make, `le` asks scripts/dev/session-scratch.sh
# for the same answer.
ZE_FUNCTIONAL_RUN = ZE_SCRATCH_DIR='$(ZE_SCRATCH_DIR)' scripts/dev/ze-run.sh

# Packages that `.ci` tests shell out to with `exec=go test ...`. Derived from
# the `.ci` files themselves so the list cannot drift from what they invoke.
ZE_CI_GO_TEST_PKGS = $(shell grep -rhoE 'exec=go test [^:]*' test/ --include='*.ci' 2>/dev/null | grep -oE '\./[a-zA-Z0-9_/.-]+' | sort -u)

# Warm the Go build cache for those packages BEFORE any suite runs.
#
# Each such `.ci` test spends its per-test budget on `go test`, which COMPILES
# the package first. On a quiet host the compile is cached and invisible; in a
# full parallel run it is not, and a test then times out waiting on the
# compiler rather than on the behavior it asserts -- test/ospf/ospf-neighbor.ci
# (20s exec budget) did exactly that while its own Go test takes 0.01s.
# Compiling once here takes compilation out of every per-test budget instead of
# hiding the problem behind a larger timeout (ai/rules/completion.md: a
# generous timeout is a synonym for an unknown one).
#
# No tags, deliberately: the `.ci` commands invoke bare `go test`, and the build
# cache is keyed by tag set, so warming with tags would populate a different
# cache entry and warm nothing.
.PHONY: ze-functional-test-warm
ze-functional-test-warm:
	@scripts/dev/ze-run.sh ze-functional-test-warm $(MAKE) --no-print-directory _ze-functional-test-warm-impl

_ze-functional-test-warm-impl:
	@printf 'Warming build cache for %s .ci-invoked package(s)...\n' '$(words $(ZE_CI_GO_TEST_PKGS))'
	@$(GO) test -run '^$$' -count=1 $(ZE_CI_GO_TEST_PKGS) >/dev/null

# ─── The gating run ─────────────────────────────────────────────────────────
# The suites it runs, the order it runs them in, and the progress denominator
# are one list, `GATING` in scripts/le/application/functional.py. A suite that
# ships and does not gate has a target below and no place in that list.
ze-functional-test: ze-functional-test-warm $(ZE_TEST_DEPS_ALL) $(ZE_WEB_CHAOS_DEP)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-test $(CURDIR)/le functional

# ─── Individual functional test suites ──────────────────────────────────────
# Each target runs the same suite, with the same flags and the same wall-clock
# budget the gating run gives it, so a stuck suite invoked directly from the CLI
# is process-group-killed rather than wedging indefinitely. What it does NOT get
# is the budget REPORT: the progress line, the runtime line and the creep
# warning belong to the run that has 23 other suites waiting behind this one.

ze-functional-encode-test: $(ZE_TEST_DEPS)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-encode-test $(CURDIR)/le functional ze-functional-encode-test

ze-functional-plugin-test: $(ZE_TEST_DEPS)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-plugin-test $(CURDIR)/le functional ze-functional-plugin-test

ze-functional-decode-test: $(ZE_TEST_DEPS)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-decode-test $(CURDIR)/le functional ze-functional-decode-test

ze-functional-parse-test: $(ZE_TEST_DEPS)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-parse-test $(CURDIR)/le functional ze-functional-parse-test

ze-functional-reload-test: $(ZE_TEST_DEPS)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-reload-test $(CURDIR)/le functional ze-functional-reload-test

ze-functional-ui-test: $(ZE_TEST_DEPS_STRIPPED)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-ui-test $(CURDIR)/le functional ze-functional-ui-test

ze-functional-editor-test: $(ZE_TEST_DEPS)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-editor-test $(CURDIR)/le functional ze-functional-editor-test

ze-functional-web-test: $(ZE_TEST_DEPS_ZE) $(ZE_WEB_CHAOS_DEP)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-web-test $(CURDIR)/le functional ze-functional-web-test

ze-functional-managed-test: $(ZE_TEST_DEPS)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-managed-test $(CURDIR)/le functional ze-functional-managed-test

ze-functional-l2tp-test: $(ZE_TEST_DEPS)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-l2tp-test $(CURDIR)/le functional ze-functional-l2tp-test

ze-functional-firewall-test: $(ZE_TEST_DEPS)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-firewall-test $(CURDIR)/le functional ze-functional-firewall-test

ze-functional-policy-test: $(ZE_TEST_DEPS)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-policy-test $(CURDIR)/le functional ze-functional-policy-test

ze-functional-ipsec-test: $(ZE_TEST_DEPS)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-ipsec-test $(CURDIR)/le functional ze-functional-ipsec-test

ze-functional-appliance-test: $(ZE_TEST_DEPS)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-appliance-test $(CURDIR)/le functional ze-functional-appliance-test

ze-functional-ldp-test: $(ZE_TEST_DEPS)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-ldp-test $(CURDIR)/le functional ze-functional-ldp-test

ze-functional-rsvpte-test: $(ZE_TEST_DEPS)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-rsvpte-test $(CURDIR)/le functional ze-functional-rsvpte-test

ze-functional-install-test: $(ZE_TEST_DEPS)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-install-test $(CURDIR)/le functional ze-functional-install-test

ze-functional-runner-test: $(ZE_TEST_DEPS)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-runner-test $(CURDIR)/le functional ze-functional-runner-test

ze-functional-isis-test: $(ZE_TEST_DEPS)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-isis-test $(CURDIR)/le functional ze-functional-isis-test

# ospf and ospfv3 warm the build cache first: their .ci tests are the ones that
# shell out to `go test`, and a cold compile inside a 20s per-test budget times
# out on the compiler rather than on the behavior asserted.
ze-functional-ospf-test: ze-functional-test-warm $(ZE_TEST_DEPS)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-ospf-test $(CURDIR)/le functional ze-functional-ospf-test

ze-functional-ospfv3-test: ze-functional-test-warm $(ZE_TEST_DEPS)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-ospfv3-test $(CURDIR)/le functional ze-functional-ospfv3-test

ze-functional-l2tp-wire-test: $(ZE_TEST_DEPS)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-l2tp-wire-test $(CURDIR)/le functional ze-functional-l2tp-wire-test

ze-functional-isis-wire-test: $(ZE_TEST_DEPS)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-isis-wire-test $(CURDIR)/le functional ze-functional-isis-wire-test

ze-functional-ospf-wire-test: $(ZE_TEST_DEPS)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-ospf-wire-test $(CURDIR)/le functional ze-functional-ospf-wire-test

# ─── Non-gated functional test suites ───────────────────────────────────────
# These suites are shipped but not in the default ze-precommit-verify gate: each
# needs platform-specific tooling or separate fixture setup. They are absent
# from `GATING`, so a `.ci` here earns no verify tier.

ze-functional-static-test: $(ZE_TEST_DEPS)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-static-test $(CURDIR)/le functional ze-functional-static-test

ze-functional-traffic-test: $(ZE_TEST_DEPS)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-traffic-test $(CURDIR)/le functional ze-functional-traffic-test

ze-functional-flow-export-test: $(ZE_TEST_DEPS)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-flow-export-test $(CURDIR)/le functional ze-functional-flow-export-test

ze-functional-vpp-test: $(ZE_TEST_DEPS)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-vpp-test $(CURDIR)/le functional ze-functional-vpp-test

ze-functional-vrrp-test: $(ZE_TEST_DEPS)
	@$(ZE_FUNCTIONAL_RUN) ze-functional-vrrp-test $(CURDIR)/le functional ze-functional-vrrp-test

# Fail-open call-site ratchet over this suite's own Python. It runs as a PAIR:
# --selftest proves every verdict fires on a known fixture, so the scan that
# follows cannot pass vacuously. Both halves are gates in the functional area.
#
# The floor is enforced on the verify path WITHOUT this target: TestRepoRatchet
# in scripts/dev/docker_exec_checked_test.py runs the real scan, and
# scripts/dev/python_tests_test.go globs every *_test.py, so `make ze-unit-test`
# already refuses a rise.
#
# It lives here rather than in mk/check-docs.mk because its whole population is
# test/**/*.py, the functional harness this file owns.
.PHONY: ze-functional-docker-exec-check
ze-functional-docker-exec-check:
	@$(CURDIR)/le functional ze-functional-docker-exec-selftest ze-functional-docker-exec-check

# The `_<target>-impl` half of every admitted pair defined in this file.
# The public half calls the admission wrapper and this half holds the work;
# see the job-admission block above ZE_RUN_SLOTS in the Makefile.
.PHONY: _ze-functional-test-warm-impl
