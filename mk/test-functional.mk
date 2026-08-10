# Functional tests: .ci-based suites run via $(ZEBIN_TEST)
#
# Quick reference:
#   make ze-functional-test    All 24 gating suites (the all_suites list below)
#   make ze-encode-test        Encoding only
#   make ze-plugin-test        Plugin behavior only
#   make ze-decode-test        Wire decoding only
#   make ze-parse-test         Config parsing only
#   make ze-reload-test        Config reload only
#   make ze-ui-test            CLI/completion only
#   make ze-editor-test        TUI editor (.et files)
#   make ze-managed-test       Managed config only
#   make ze-web-test           Web UI only
#   make ze-l2tp-test          L2TP only
#   make ze-firewall-test      Firewall only
#   make ze-policy-test        Policy routing only
#   make ze-appliance-test     Appliance CLI (build/iso/list/serial-login) only
#   make ze-ospf-test          OSPF config/doctor tests
#   make ze-vrrp-test          VRRP config/show/doctor tests
#   make ze-static-test        Static routes (release evidence only)
#   make ze-traffic-test       Traffic control (release evidence only)
#   make ze-flow-export-test   Flow export sFlow/NetFlow/IPFIX (release evidence only)
#   make ze-vpp-test           VPP stub (release evidence only)
#   make ze-l2tp-wire-test     L2TP wire-level (release evidence only)
#   make ze-isis-wire-test     IS-IS wire-level decode (release evidence only)
#   make ze-ospf-wire-test     OSPFv2 wire-level decode (release evidence only)
#
# Every target here runs against an ISOLATED test binary set by default (built
# under tmp/testbin-<suffix>/ and removed on exit), so a running suite never
# touches the dev $(ZEBIN_ZE) and you can keep building/editing while it runs.
# ZE_SUFFIX=<name> pins a stable, kept directory; ZE_TEST_CANONICAL=1 opts out.
# See the isolated-binary block below.

.PHONY: ze-functional-test
.PHONY: ze-encode-test ze-plugin-test ze-decode-test ze-parse-test ze-reload-test
.PHONY: ze-ui-test ze-editor-test ze-web-test ze-managed-test
.PHONY: ze-l2tp-test ze-firewall-test ze-policy-test ze-appliance-test ze-runner-test
.PHONY: ze-static-test ze-traffic-test ze-flow-export-test ze-vpp-test ze-l2tp-wire-test ze-isis-wire-test ze-ospf-wire-test ze-isis-test ze-ospf-test ze-ospfv3-test ze-vrrp-test

# Per-suite wall-clock cap. A stuck subprocess that holds an output pipe open
# can make ze-test's own cmd.Wait() block indefinitely after SIGKILL; `timeout`
# runs the suite in its own process group and signals the whole group on
# expiry, so leaked grandchildren (ze daemons, tacacs-mocks) die with it.
# Exit code 124 from timeout is treated as a suite failure like any other.
# Override: make ze-functional-test ZE_SUITE_TIMEOUT=1200s
ZE_SUITE_TIMEOUT ?= 600s
ZE_SUITE_KILL_AFTER ?= 10s
ZE_ENCODE_PARALLEL ?= 8
ZE_PLUGIN_PARALLEL ?= 8
SUITE_RUN = timeout --kill-after=$(ZE_SUITE_KILL_AFTER) $(ZE_SUITE_TIMEOUT)

# ─── Isolated test binary set (automatic; the default for every suite) ──────
# By DEFAULT every functional target in this file builds its OWN throwaway
# binary set under tmp/testbin-<suffix>/ (ze, ze-test, ze-stripped) and runs
# frozen against it (ZE_TEST_NO_BUILD=1). This keeps testing and development on
# separate binaries:
#   - the legacy path had each $(ZEBIN_TEST) invocation recompile ze + ze-test
#     from the working tree (internal/test/runner Build), so `make ze` or an
#     edit made while a suite ran clobbered the dev $(ZEBIN_ZE), and half-edited
#     source leaked into later suites;
#   - now each target builds the set at the start of its recipe and
#     ZE_TEST_NO_BUILD=1 stops the runner recompiling mid-run, so $(ZEBIN_ZE) is
#     never touched by a test and you can keep building/editing it while a
#     suite runs.
# In auto mode the dir is tmp/testbin-pid-<make-PID>-<target>/: unique per make
# invocation AND per target, so chaining suites on one command line (even under
# -j) never lets one target's cleanup delete another's binaries. The dir is
# removed when the target exits (trap). ze-verify inherits this because it just
# runs `make ze-functional-test`. Each target rebuilds all three binaries
# (including the full ze), a deliberate cost for a uniform isolated set.
#
# .ci tests exec `ze` / `ze-stripped` by bare name; the binaries carry those
# canonical names inside the dir, ZE_BIN points there, and the runner puts that
# dir first on PATH (internal/test/runner/runner_exec.go).
#
# Overrides:
#   ZE_SUFFIX=<name>     pin a stable name (tmp/testbin-<name>/), KEPT on exit
#                        -- run a named suite and keep developing against it.
#   ZE_TEST_CANONICAL=1  opt out entirely: the runner rebuilds $(ZEBIN_ZE) +
#                        $(ZEBIN_TEST) in place (release/CI reproducibility).
# Shared residue in every mode: tmp/test-timings.json (display baseline) is
# merged by sample count on save (internal/test/runner/timing.go Save), with a
# residual reload->rename race, so a concurrent ze-test invocation's samples are
# no longer clobbered wholesale.
ZE_SUFFIX ?=
ZE_TEST_CANONICAL ?=
ifeq ($(ZE_TEST_CANONICAL),)
  ifeq ($(ZE_SUFFIX),)
    # Auto: := fixes the PID once (stable within a run); $@ (recursive =) scopes
    # the dir per target so chaining suites on one command line
    # (make ze-encode-test ze-plugin-test, even under -j) never lets one
    # target's cleanup trap delete another target's binaries. Throwaway, rm on
    # exit.
    # $(ZE_SCRATCH_DIR) is tmp/ off-session and this session's own dated
    # directory under an AI session (mk/session.mk), so the throwaway set is
    # owned by the session and lands beside its binaries even if this trap never
    # fires (crash, kill -9). The
    # pid-<PPID>-<target> scoping stays INSIDE that root: it still separates two
    # concurrent make invocations of the same target within one session.
    ZE_RUN_SUFFIX := pid-$(shell echo $$PPID)
    ZE_ALT_DIR = $(ZE_SCRATCH_DIR)/testbin-$(ZE_RUN_SUFFIX)-$@
    ZE_ALT_TRAP = rm -rf $(ZE_ALT_DIR)
  else
    # Explicit name: stable, shared across this run's targets, KEPT on exit.
    # The trap is a no-op, so sharing the dir is safe for CLEANUP.
    #
    # CONCURRENCY: an explicit ZE_SUFFIX is not isolated WITHIN one session. Two
    # runs that pick the same name -- `make -j <A> <B> ZE_SUFFIX=x` -- share one
    # testbin-<name>/, so they race on the build AND (worse) share the throwaway
    # root's etc/ze: one run's test DB/config writes then corrupt the other's.
    # The default (omit ZE_SUFFIX) gives per-invocation auto dirs,
    # pid-<make-PID>-<target>, that never collide -- prefer it for concurrent
    # work, and reserve ZE_SUFFIX=<name> for a single serial run you want kept
    # for inspection.
    #
    # ACROSS sessions this is now safe: $(ZE_SCRATCH_DIR) is per-session, so two
    # sessions that both pick ZE_SUFFIX=x get different roots and no longer
    # corrupt each other's throwaway etc/ze.
    ZE_RUN_SUFFIX := $(ZE_SUFFIX)
    ZE_ALT_DIR = $(ZE_SCRATCH_DIR)/testbin-$(ZE_RUN_SUFFIX)
    ZE_ALT_TRAP := true
  endif
  # The binaries live in a `bin/` SUBDIR of the throwaway root. ze derives its
  # config/DB directory from its own location and only recognises a parent dir
  # named bin/sbin (internal/core/paths/paths.go isBinDir); a binary directly in
  # tmp/testbin-<suffix>/ yields "cannot determine database location" and breaks
  # commands like `ze config archive` (test/parse/cli-config-archive.ci). With
  # the bin/ subdir the derived config dir is the throwaway root's etc/ze, so any
  # DB a test writes is isolated and swept with the dir.
  ZE_ALT_BIN = $(ZE_ALT_DIR)/bin
  # Build the isolated set inline at the top of each recipe (NOT via a shared
  # phony prereq, which would build ONE dir for every target in the invocation
  # and let the per-recipe cleanup traps collide). The trap is armed before this
  # runs, so a failed build is cleaned up too; `|| exit 1` aborts the recipe if
  # any build fails. Canonical names ze/ze-test/ze-stripped are what .ci tests
  # exec by. The DUT build mirrors runner.TestBuildTags()
  # (internal/test/runner/runner.go): zetest test plugins + full command surface
  # (ze_core ze_distro ze_setup) + default feature gates, NO version ldflags so
  # `ze show version` prints "ze dev" (test/parse/cli-show-version.ci).
  # ze-stripped tags match the $(ZEBIN_STRIPPED) Makefile rule.
  ZE_ALT_BUILD = { mkdir -p $(ZE_ALT_BIN) && printf 'Building isolated test binaries in %s/ (ze, ze-test, ze-stripped)...\n' '$(ZE_ALT_BIN)' && $(GO) build -tags 'ze_core ze_distro ze_setup zetest $(ZE_FEATURES) $(ZE_TAGS)' -o $(ZE_ALT_BIN)/ze ./cmd/ze && $(GO) build -tags 'ze_core ze_ssh $(ZE_TAGS)' -o $(ZE_ALT_BIN)/ze-stripped ./cmd/ze && $(GO) build -tags 'ze_test $(ZE_FEATURES) $(ZE_TAGS)' -o $(ZE_ALT_BIN)/ze-test ./cmd/ze ; } || exit 1;
  ZE_TEST_DEPS :=
  ZE_TEST_DEPS_STRIPPED :=
  ZE_TEST_DEPS_ZE :=
  ZE_TEST_DEPS_ALL :=
  ZE_TEST_RUN = env ZE_TEST_NO_BUILD=1 ZE_BIN=$(ZE_ALT_BIN)/ze ZE_TEST_BIN=$(ZE_ALT_BIN)/ze-test $(ZE_ALT_BIN)/ze-test
else
  ZE_ALT_TRAP := true
  ZE_ALT_BUILD :=
  ZE_TEST_DEPS := $(ZEBIN_TEST)
  ZE_TEST_DEPS_STRIPPED := $(ZEBIN_TEST) $(ZEBIN_STRIPPED)
  ZE_TEST_DEPS_ZE := $(ZEBIN_ZE) $(ZEBIN_TEST)
  ZE_TEST_DEPS_ALL := $(ZEBIN_ZE) $(ZEBIN_STRIPPED) $(ZEBIN_TEST)
  ZE_TEST_RUN := $(ZEBIN_TEST)
endif

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
.PHONY: ze-functional-warm
ze-functional-warm:
	@printf 'Warming build cache for %s .ci-invoked package(s)...\n' '$(words $(ZE_CI_GO_TEST_PKGS))'
	@$(GO) test -run '^$$' -count=1 $(ZE_CI_GO_TEST_PKGS) >/dev/null

# Run ze functional tests (all types, continue on failure to show all results)
# Release evidence matrix: every suite in the all_suites line below. That line is the
# single source of truth, so this comment names none of them and cannot drift from it.
# A suite that has a runner and no all_suites entry (static, traffic, flow-export, vpp,
# vrrp, chaos-web) carries a platform dependency or infrastructure this target does not
# set up. Add a suite to all_suites and to a run_suite line together, because the first
# sets the progress denominator and the second runs it.
# ZE_SKIP_SUITES: comma-separated list of suites to skip (e.g. firewall,web
# for Docker environments without agent-browser or native process control).
ZE_SKIP_SUITES ?=
ze-functional-test: ze-functional-warm $(ZE_TEST_DEPS_ALL)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) \
	failed=0; failed_names=""; skipped_names=""; total=0; suite_index=0; \
	all_suites="encode plugin parse decode reload ui editor managed l2tp firewall policy ipsec ldp rsvpte isis ospf ospfv3 web install appliance l2tp-wire isis-wire ospf-wire runner"; \
	suite_total=0; \
	for suite in $$all_suites; do \
		case ",$(ZE_SKIP_SUITES)," in *,$$suite,*) ;; *) suite_total=$$((suite_total + 1));; esac; \
	done; \
	run_suite() { \
		suite="$$1"; shift; \
		case ",$(ZE_SKIP_SUITES)," in \
			*,$$suite,*) skipped_names="$${skipped_names:+$$skipped_names }$$suite"; return 0 ;; \
		esac; \
		total=$$((total + 1)); suite_index=$$((suite_index + 1)); \
		printf "\n[%d/%d] suite %s\n" "$$suite_index" "$$suite_total" "$$suite"; \
		"$$@" || { failed=$$((failed + 1)); failed_names="$${failed_names:+$$failed_names }$$suite"; }; \
	}; \
	run_suite encode $(SUITE_RUN) $(ZE_TEST_RUN) bgp encode --all -p $(ZE_ENCODE_PARALLEL); \
	run_suite plugin $(SUITE_RUN) $(ZE_TEST_RUN) bgp plugin --all -p $(ZE_PLUGIN_PARALLEL); \
	run_suite parse $(SUITE_RUN) $(ZE_TEST_RUN) bgp parse --all; \
	run_suite decode $(SUITE_RUN) $(ZE_TEST_RUN) bgp decode --all; \
	run_suite reload $(SUITE_RUN) $(ZE_TEST_RUN) bgp reload --all -p 1; \
	run_suite ui $(SUITE_RUN) $(ZE_TEST_RUN) ui --all; \
	run_suite editor $(SUITE_RUN) $(ZE_TEST_RUN) editor --all; \
	run_suite managed $(SUITE_RUN) $(ZE_TEST_RUN) managed --all -p 1; \
	run_suite l2tp $(SUITE_RUN) $(ZE_TEST_RUN) l2tp --all; \
	run_suite firewall $(SUITE_RUN) $(ZE_TEST_RUN) firewall --all; \
	run_suite policy $(SUITE_RUN) $(ZE_TEST_RUN) policy --all; \
	run_suite ipsec $(SUITE_RUN) $(ZE_TEST_RUN) ipsec --all; \
	run_suite ldp $(SUITE_RUN) $(ZE_TEST_RUN) ldp --all; \
	run_suite rsvpte $(SUITE_RUN) $(ZE_TEST_RUN) rsvpte --all; \
	run_suite isis $(SUITE_RUN) $(ZE_TEST_RUN) isis --all; \
	run_suite ospf $(SUITE_RUN) $(ZE_TEST_RUN) ospf --all; \
	run_suite ospfv3 $(SUITE_RUN) $(ZE_TEST_RUN) ospfv3 --all; \
	run_suite web $(SUITE_RUN) $(ZE_TEST_RUN) web --all; \
	run_suite install $(SUITE_RUN) $(ZE_TEST_RUN) install --all; \
	run_suite appliance $(SUITE_RUN) $(ZE_TEST_RUN) appliance --all; \
	run_suite l2tp-wire $(SUITE_RUN) $(ZE_TEST_RUN) l2tp-wire --all; \
	run_suite isis-wire $(SUITE_RUN) $(ZE_TEST_RUN) isis-wire --all; \
	run_suite ospf-wire $(SUITE_RUN) $(ZE_TEST_RUN) ospf-wire --all; \
	run_suite runner $(SUITE_RUN) $(ZE_TEST_RUN) runner --all; \
	if [ -n "$$skipped_names" ]; then \
		printf "\n\033[33mSKIPPED suites (ZE_SKIP_SUITES): %s\033[0m\n" "$$skipped_names"; \
	fi; \
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
		printf "\033[32mPASS  all $$total suites\033[0m\n\n"; \
	fi

# ─── Individual functional test suites ──────────────────────────────────────
# Same SUITE_RUN wall-clock cap as the combined ze-functional-test target
# (see ZE_SUITE_TIMEOUT above) so a stuck suite invoked directly from the
# CLI also gets process-group-killed instead of wedging indefinitely.

ze-encode-test: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) bgp encode --all -p $(ZE_ENCODE_PARALLEL)

ze-plugin-test: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) bgp plugin --all -p $(ZE_PLUGIN_PARALLEL)

ze-decode-test: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) bgp decode --all

ze-parse-test: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) bgp parse --all

ze-reload-test: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) bgp reload --all -p 1

ze-ui-test: $(ZE_TEST_DEPS_STRIPPED)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) ui --all

ze-editor-test: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) editor --all

ze-web-test: $(ZE_TEST_DEPS_ZE)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) web --all

ze-managed-test: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) managed --all -p 1

ze-l2tp-test: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) l2tp --all

ze-firewall-test: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) firewall --all

ze-policy-test: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) policy --all

# IPsec/IKEv2 suite (test/ipsec/*.ci). It was listed in all_suites above but had
# no run_suite line, so it counted toward the progress denominator and never ran.
# ai/rules/testing.md derives a .ci tag's verify tier from all_suites, so every
# tag in test/ipsec/ was credited a merge-gate tier it did not earn.
ze-ipsec-test: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) ipsec --all

ze-appliance-test: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) appliance --all

# Test-runner primitive suite (test/runner/*.ci). Host-safe: it spawns only
# sh/tail helpers, no ze daemon or privileged tooling, so it stays in the gating
# ze-functional-test run_suite list above.
ze-runner-test: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) runner --all

# ─── Non-gated functional test suites ───────────────────────────────────────
# These suites are shipped but not in the default ze-verify gate. They require
# platform-specific tooling or separate fixture setup.

ze-static-test: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) static --all

ze-traffic-test: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) traffic --all

# Flow export (sFlow v5 / NetFlow v9 / IPFIX). Like static and traffic, this
# suite needs the Linux daemon and (for packet sampling) CAP_NET_ADMIN +
# kernel psample, so it is release-evidence-only and not in the gating
# ze-functional-test run_suite list above.
ze-flow-export-test: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) flow-export --all

ze-vpp-test: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) vpp --all

ze-l2tp-wire-test: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) l2tp-wire --all

ze-isis-wire-test: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) isis-wire --all

ze-ospf-wire-test: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) ospf-wire --all

ze-isis-test: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) isis --all

ze-ospf-test: ze-functional-warm $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) ospf --all

ze-ospfv3-test: ze-functional-warm $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) ospfv3 --all

ze-vrrp-test: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) vrrp --all

# Fail-open call-site ratchet over this suite's own Python. `docker_exec_quiet`
# (test/interop/interop.py) returns "" on ANY non-zero exit, so a caller that does
# not test the value for emptiness turns a command that FAILED into a passing
# assertion over nothing (ai/rules/evidence.md). The flagged set is DERIVED to a
# fixpoint -- a function that returns a fail-open call is itself fail-open -- so a
# new wrapper is covered the day it is written, which is what a seed-name-only
# lint would have missed: scenarios call the 19 wrappers, not the seed. The floor
# in test/health/docker-exec-baseline.json may only go DOWN. --selftest runs
# first and proves every verdict fires on a known fixture, so the gate cannot
# pass vacuously.
#
# The floor is enforced on the verify path WITHOUT this target: TestRepoRatchet
# in scripts/dev/docker_exec_checked_test.py runs the real scan, and
# scripts/dev/python_tests_test.go globs every *_test.py, so `make ze-unit-test`
# already refuses a rise. Changed-file routing through verify_wiring_docs.py is
# written and tested but held out of this commit: another session's in-flight
# refactor has interleaved 12 lines into that file, and committing it would
# carry their half-finished work.
#
# It lives here rather than in mk/inventory.mk because its whole population is
# test/**/*.py, the functional harness this file owns.
.PHONY: ze-docker-exec-check
ze-docker-exec-check:
	@python3 scripts/dev/docker_exec_checked.py --selftest
	@python3 scripts/dev/docker_exec_checked.py
