# Functional tests: .ci-based suites run via $(ZEBIN_TEST)
#
# Quick reference:
#   make ze-functional-test    All 24 gating suites (the all_suites list below)
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
#   make ze-functional-ldp-test           LDP only
#   make ze-functional-rsvpte-test        RSVP-TE only
#   make ze-functional-install-test       Installer/PXE/kernel config only
#   make ze-functional-appliance-test     Appliance CLI (build/iso/list/serial-login) only
#   make ze-functional-ospf-test          OSPF config/doctor tests
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
# under tmp/testbin-<suffix>/ and removed on exit), so a running suite never
# touches the dev $(ZEBIN_ZE) and you can keep building/editing while it runs.
# ZE_SUFFIX=<name> pins a stable, kept directory; ZE_TEST_CANONICAL=1 opts out.
# See the isolated-binary block below.

.PHONY: ze-functional-test
.PHONY: ze-functional-encode-test ze-functional-plugin-test ze-functional-decode-test ze-functional-parse-test ze-functional-reload-test
.PHONY: ze-functional-ui-test ze-functional-editor-test ze-functional-web-test ze-functional-managed-test
.PHONY: ze-functional-l2tp-test ze-functional-firewall-test ze-functional-policy-test ze-functional-ipsec-test ze-functional-appliance-test ze-functional-runner-test
.PHONY: ze-functional-ldp-test ze-functional-rsvpte-test ze-functional-install-test
.PHONY: ze-functional-static-test ze-functional-traffic-test ze-functional-flow-export-test ze-functional-vpp-test ze-functional-l2tp-wire-test ze-functional-isis-wire-test ze-functional-ospf-wire-test ze-functional-isis-test ze-functional-ospf-test ze-functional-ospfv3-test ze-functional-vrrp-test

# Per-suite wall-clock cap. A stuck subprocess that holds an output pipe open
# can make ze-standard-test's own cmd.Wait() block indefinitely after SIGKILL; `timeout`
# runs the suite in its own process group and signals the whole group on
# expiry, so leaked grandchildren (ze daemons, tacacs-mocks) die with it.
# Exit code 124 from timeout is a suite failure, and run_suite reports it as a
# budget expiry so the tests the kill interrupted are not read as product defects.
# Override: make ze-functional-test ZE_SUITE_TIMEOUT=1200s
ZE_SUITE_TIMEOUT ?= 600s
ZE_SUITE_KILL_AFTER ?= 10s
# A suite that uses this percentage of its budget is reported as a warning while
# it is still green. A suite whose runtime reaches the cap fails as a kill, and
# that kill reads as N broken tests unless somebody watched the number climb
# toward it. This is the watching: raising a cap is not a fix on its own.
ZE_SUITE_WARN_PERCENT ?= 80
# Budget override for one suite. A suite named here reads ZE_SUITE_TIMEOUT_<SUITE>
# in place of the shared cap above, so one slow suite does not take the margin
# from the other 23. Each name owes three things in this file, and
# scripts/dev/functional_suite_test.py refuses a name that is missing one: a
# SUITE_RUN_<SUITE> below, an arm in run_suite's budget case, and that
# SUITE_RUN_<SUITE> on both the run_suite line and the suite's own -impl target.
#
# plugin: 663 tests in one suite. Measured on 2026-08-19 at 855s (spec
# verify-scope-4, A-1), on a box carrying five other sessions at a load average
# rising 6.6 -> 18.7 across 32 cores. The budget is derived from that number
# rather than picked: the ZE_SUITE_WARN_PERCENT point must sit 40% above the
# measured runtime, or a contended box warns on every run and the warning stops
# meaning anything. 855 * 1.40 / 0.80 = 1496s, rounded up to the whole minute.
# The kill then lands at 1.75x the measurement, which is a wedge and not a
# busy box. Lower it when the suite is split or made faster.
ZE_SUITE_TIMEOUT_PLUGIN ?= 1500s

# ─── Suite concurrency (the two suites measured for it) ────────────────────
# encode and plugin run through the bgp runner, and their -p used to be the
# constant 8 on every host. 8 is what GitHub's 4-vCPU hosted runner survives,
# so a 32-core workstation was running the 665-test plugin suite at a quarter
# of the width it can carry. Derive it instead: floor at what the smallest
# supported host runs today, cap at the core count.
#
# The FLOOR keeps CI byte-identical: on 4 cores the floor wins and both suites
# still get 8. It mirrors runner.SuiteConcurrencyFloor
# (internal/test/runner/parallel.go), and scripts/dev/functional_suite_test.py
# holds the two numbers equal so neither can drift alone.
#
# The CAP is the core count, and it is measured rather than picked. On this
# 32-core box the plugin suite runs at 96% parallel efficiency at 8, 88% at 16,
# 74% at 32 and 36% at 64; 64 lands inside the two-run spread at 32, buys no
# measurable wall clock, and costs pass rate. Neither figure transfers to the
# 22 registerCIRoot suites, which keep DefaultSuiteConcurrency's 2x CPUs: this
# measurement never covered them.
#
# ZE_SUITE_CORES exists so a test can drive a small host without owning one
# (scripts/dev/functional_suite_test.py). A core count that is missing or not a
# number falls back to the floor rather than to an empty -p.
ZE_SUITE_PARALLEL_FLOOR := 8
ZE_SUITE_CORES ?= $(shell nproc 2>/dev/null || getconf _NPROCESSORS_ONLN 2>/dev/null || echo $(ZE_SUITE_PARALLEL_FLOOR))
# No parenthesis may appear inside this $(shell ...): make's own expansion
# counts them, so a `case` pattern would close the function early.
ZE_SUITE_PARALLEL := $(shell n='$(ZE_SUITE_CORES)'; f='$(ZE_SUITE_PARALLEL_FLOOR)'; if [ "$$n" -gt "$$f" ] 2>/dev/null; then echo "$$n"; else echo "$$f"; fi)
ZE_ENCODE_PARALLEL ?= $(ZE_SUITE_PARALLEL)
ZE_PLUGIN_PARALLEL ?= $(ZE_SUITE_PARALLEL)
SUITE_RUN = timeout --kill-after=$(ZE_SUITE_KILL_AFTER) $(ZE_SUITE_TIMEOUT)
SUITE_RUN_PLUGIN = timeout --kill-after=$(ZE_SUITE_KILL_AFTER) $(ZE_SUITE_TIMEOUT_PLUGIN)

# ─── Isolated test binary set (automatic; the default for every suite) ──────
# By DEFAULT every functional target in this file builds its OWN throwaway
# binary set under tmp/testbin-<suffix>/ (ze, ze-test, ze-stripped) and runs
# frozen against it (ZE_TEST_NO_BUILD=1). This keeps testing and development on
# separate binaries:
#   - the legacy path had each $(ZEBIN_TEST) invocation recompile ze + ze-test
#     from the working tree (internal/test/runner Build), so `make ze-build` or an
#     edit made while a suite ran clobbered the dev $(ZEBIN_ZE), and half-edited
#     source leaked into later suites;
#   - now each target builds the set at the start of its recipe and
#     ZE_TEST_NO_BUILD=1 stops the runner recompiling mid-run, so $(ZEBIN_ZE) is
#     never touched by a test and you can keep building/editing it while a
#     suite runs.
# In auto mode the dir is tmp/testbin-pid-<make-PID>-<target>/: unique per make
# invocation AND per target, so chaining suites on one command line (even under
# -j) never lets one target's cleanup delete another's binaries. The dir is
# removed when the target exits (trap). ze-precommit-verify inherits this because it just
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

# ZE_COVER=1 records which Go packages each suite EXECUTES.
#
# The DUT binaries (ze, ze-stripped) are compiled with `go build -cover`, and
# run_suite gives each suite its own GOCOVERDIR. childEnv
# (internal/test/runner/runner_exec_util.go) returns os.Environ() plus its own
# entries, so the variable reaches every process a .ci starts with no per-test
# plumbing. After the run, `go tool covdata percent -i=<suite dir>` reduces the
# directory to the packages that suite reached.
#
# OFF BY DEFAULT, and off means byte-identical: ZE_COVER_FLAG is empty, so the
# build command is the one it always was, and ZE_COVER_ROOT is empty, so
# run_suite's guard is a single false test.
#
# ze-test is deliberately NOT instrumented. It is the harness, not the subject:
# its execution says what the RUNNER touched, which is not what the map is
# about.
#
# THE PATH MUST BE ABSOLUTE. A .ci that declares tmpfs= runs its process with
# proc.Dir set to the per-test directory ((*Runner).runOrchestrated in
# internal/test/runner/runner_exec.go), so a relative GOCOVERDIR resolves
# against THAT directory, the emit fails, and the Go runtime prints "coverage
# meta-data emit failed" on the child's stderr. That is silent data loss AND a
# stderr change a .ci can assert on. Measured on 2026-08-19: with a relative
# root, editor, managed, policy, ipsec, ldp and rsvpte recorded zero files and
# parse recorded one process out of hundreds.
ZE_COVER ?=
ifeq ($(ZE_COVER),)
  ZE_COVER_FLAG :=
  ZE_COVER_ROOT :=
else
  ZE_COVER_FLAG := -cover
  ZE_COVER_ROOT := $(abspath $(ZE_SCRATCH_DIR)/scratch/covdata)
endif

ZE_SUFFIX ?=
ZE_TEST_CANONICAL ?=
ifeq ($(ZE_TEST_CANONICAL),)
  ifeq ($(ZE_SUFFIX),)
    # Auto: := fixes the PID once (stable within a run); $@ (recursive =) scopes
    # the dir per target so chaining suites on one command line
    # (make ze-functional-encode-test ze-functional-plugin-test, even under -j) never lets one
    # target's cleanup trap delete another target's binaries. Throwaway, rm on
    # exit.
    # $@ is the `_<suite>-impl` half of the admission pair, because that is the
    # target whose recipe runs (see the job-admission block in the Makefile).
    # One name per suite either way, which is all this scoping asks for.
    # $(ZE_SCRATCH_DIR) is tmp/ off-session and this session's own dated
    # directory under an AI session (mk/helper-session.mk), so the throwaway set is
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
  # `ze show version` prints "ze dev" (test/parse/cli-version-show.ci).
  # ze-stripped tags match the $(ZEBIN_STRIPPED) Makefile rule.
  ZE_ALT_BUILD = { mkdir -p $(ZE_ALT_BIN) && printf 'Building isolated test binaries in %s/ (ze, ze-test, ze-stripped)...\n' '$(ZE_ALT_BIN)' && CGO_ENABLED=0 $(GO) build $(ZE_COVER_FLAG) -tags 'ze_core ze_distro ze_setup zetest $(ZE_FEATURES) $(ZE_TAGS)' -o $(ZE_ALT_BIN)/ze ./cmd/ze && CGO_ENABLED=0 $(GO) build $(ZE_COVER_FLAG) -tags 'ze_core ze_ssh $(ZE_TAGS)' -o $(ZE_ALT_BIN)/ze-stripped ./cmd/ze && CGO_ENABLED=0 $(GO) build -tags 'ze_test $(ZE_FEATURES) $(ZE_TAGS)' -o $(ZE_ALT_BIN)/ze-test ./cmd/ze ; } || exit 1;
  ZE_TEST_DEPS :=
  ZE_TEST_DEPS_STRIPPED :=
  ZE_TEST_DEPS_ZE :=
  ZE_TEST_DEPS_ALL :=
  ZE_TEST_RUN = env ZE_TEST_NO_BUILD=1 ZE_BIN=$(ZE_ALT_BIN)/ze ZE_TEST_BIN=$(ZE_ALT_BIN)/ze-test $(ZE_ALT_BIN)/ze-test
  # The chaos dashboard is a second compile of cmd/ze under different tags, and
  # only the .wb suite starts it (option=server:kind=chaos). It is built BESIDE
  # the ze binary the run uses, which is where cmd_web.go looks for it.
  ZE_ALT_CHAOS_BUILD = { CGO_ENABLED=0 $(GO) build -tags 'ze_chaos ze_bgp' -o $(ZE_ALT_BIN)/ze-chaos ./cmd/ze ; } || exit 1;
  ZE_WEB_CHAOS_DEP :=
else
  ZE_ALT_TRAP := true
  ZE_ALT_BUILD :=
  ZE_TEST_DEPS := $(ZEBIN_TEST)
  ZE_TEST_DEPS_STRIPPED := $(ZEBIN_TEST) $(ZEBIN_STRIPPED)
  ZE_TEST_DEPS_ZE := $(ZEBIN_ZE) $(ZEBIN_TEST)
  ZE_TEST_DEPS_ALL := $(ZEBIN_ZE) $(ZEBIN_STRIPPED) $(ZEBIN_TEST)
  ZE_TEST_RUN := $(ZEBIN_TEST)
  ZE_ALT_CHAOS_BUILD :=
  ZE_WEB_CHAOS_DEP := $(ZEBIN_CHAOS)
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
.PHONY: ze-functional-test-warm
ze-functional-test-warm:
	@scripts/dev/ze-run.sh ze-functional-test-warm $(MAKE) --no-print-directory _ze-functional-test-warm-impl

_ze-functional-test-warm-impl:
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
ze-functional-test:
	@scripts/dev/ze-run.sh ze-functional-test $(MAKE) --no-print-directory _ze-functional-test-impl

_ze-functional-test-impl: ze-functional-test-warm $(ZE_TEST_DEPS_ALL) $(ZE_WEB_CHAOS_DEP)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(ZE_ALT_CHAOS_BUILD) \
	failed=0; failed_names=""; skipped_names=""; total=0; suite_index=0; \
	expired_names=""; warned_names=""; runtimes=""; \
	cover_root='$(ZE_COVER_ROOT)'; \
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
		if [ -n "$$cover_root" ]; then \
			suite_covdir="$$cover_root/$$suite"; \
			rm -rf "$$suite_covdir"; mkdir -p "$$suite_covdir"; \
			set -- env GOCOVERDIR="$$suite_covdir" "$$@"; \
		fi; \
		printf "\n[%d/%d] suite %s\n" "$$suite_index" "$$suite_total" "$$suite"; \
		suite_started=$$(date +%s); \
		"$$@"; suite_status=$$?; \
		suite_seconds=$$(($$(date +%s) - suite_started)); \
		case "$$suite" in \
			plugin) budget="$(ZE_SUITE_TIMEOUT_PLUGIN)"; budget_var=ZE_SUITE_TIMEOUT_PLUGIN ;; \
			*) budget="$(ZE_SUITE_TIMEOUT)"; budget_var=ZE_SUITE_TIMEOUT ;; \
		esac; \
		budget_number="$${budget%[smhd]}"; \
		case "$${budget#$$budget_number}" in \
			""|s) budget_scale=1 ;; \
			m) budget_scale=60 ;; \
			h) budget_scale=3600 ;; \
			d) budget_scale=86400 ;; \
			*) budget_scale=0 ;; \
		esac; \
		case "$$budget_number" in \
			""|*[!0-9]*) budget_seconds=0 ;; \
			*) budget_seconds=$$((budget_number * budget_scale)) ;; \
		esac; \
		if [ "$$budget_seconds" -gt 0 ]; then \
			suite_percent=$$((suite_seconds * 100 / budget_seconds)); \
			printf "      suite %s took %ss of its %s budget (%s%%)\n" "$$suite" "$$suite_seconds" "$$budget" "$$suite_percent"; \
			runtimes="$$runtimes  $$suite $${suite_seconds}s of $$budget ($$suite_percent%)\n"; \
		else \
			suite_percent=0; \
			printf "      suite %s took %ss (budget %s is not a duration this report can measure against)\n" "$$suite" "$$suite_seconds" "$$budget"; \
			runtimes="$$runtimes  $$suite $${suite_seconds}s of $$budget (unmeasurable budget)\n"; \
		fi; \
		if [ "$$suite_status" -eq 124 ]; then \
			printf "\033[31mBUDGET EXPIRED  suite %s reached its %s wall-clock budget (%s) and was killed. The test failures above are that kill, not the product.\033[0m\n" "$$suite" "$$budget" "$$budget_var"; \
			printf 'VERIFY FAILURE GROUP: {"stage":"%s","group-id":"suite-budget:%s","kind":"timeout","related":["%s"],"summary":"suite %s reached its %s wall-clock budget (%s) and was killed","rerun":"make ze-functional-%s-test","parallel":"stage"}\n' "$$suite" "$$suite" "$$suite" "$$suite" "$$budget" "$$budget_var" "$$suite"; \
			expired_names="$${expired_names:+$$expired_names }$$suite"; \
		elif [ "$$budget_seconds" -gt 0 ] && [ "$$suite_percent" -ge $(ZE_SUITE_WARN_PERCENT) ]; then \
			printf "\033[33mBUDGET WARNING  suite %s used %s%% of its %s budget, and the warning level is %s%%. Make the suite faster or raise %s before it becomes a kill.\033[0m\n" "$$suite" "$$suite_percent" "$$budget" "$(ZE_SUITE_WARN_PERCENT)" "$$budget_var"; \
			warned_names="$${warned_names:+$$warned_names }$$suite"; \
		fi; \
		if [ -n "$$cover_root" ]; then \
			printf '%s %s %s\n' "$$suite" "$$(find "$$suite_covdir" -type f | wc -l)" "$$(du -sk "$$suite_covdir" | cut -f1)" >> "$$cover_root/raw-size.txt"; \
			$(GO) tool covdata percent -i="$$suite_covdir" > "$$cover_root/$$suite.percent" 2>&1 || printf 'covdata percent failed for suite %s\n' "$$suite"; \
			rm -rf "$$suite_covdir"; \
		fi; \
		[ "$$suite_status" -eq 0 ] || { failed=$$((failed + 1)); failed_names="$${failed_names:+$$failed_names }$$suite"; }; \
	}; \
	run_suite encode $(SUITE_RUN) $(ZE_TEST_RUN) bgp encode --all -p $(ZE_ENCODE_PARALLEL); \
	run_suite plugin $(SUITE_RUN_PLUGIN) $(ZE_TEST_RUN) bgp plugin --all -p $(ZE_PLUGIN_PARALLEL); \
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
	printf "\n──── suite runtimes (default budget %s, warning level %s%%) ────\n" "$(ZE_SUITE_TIMEOUT)" "$(ZE_SUITE_WARN_PERCENT)"; \
	printf "%b" "$$runtimes"; \
	if [ -n "$$warned_names" ]; then \
		printf "\033[33mBUDGET WARNING  suite(s) near their budget: %s\033[0m\n" "$$warned_names"; \
	fi; \
	if [ -n "$$expired_names" ]; then \
		printf "\033[31mBUDGET EXPIRED  suite(s) killed at their budget: %s\033[0m\n" "$$expired_names"; \
	fi; \
	if [ -n "$$skipped_names" ]; then \
		printf "\n\033[33mSKIPPED suites (ZE_SKIP_SUITES): %s\033[0m\n" "$$skipped_names"; \
	fi; \
	if [ $$failed -gt 0 ]; then \
		printf "\n════════════════════════════════════════\n"; \
		printf "\033[31mFAIL  %d suite(s) failed: %s\033[0m\n" $$failed "$$failed_names"; \
		printf "\n\033[33mTo run failed suites individually:\033[0m\n"; \
		for suite in $$failed_names; do \
			printf "  make ze-functional-%s-test\n" "$$suite"; \
		done; \
		printf "\n"; \
		exit 1; \
	else \
		printf "\n════════════════════════════════════════\n"; \
		printf "\033[32mPASS  all $$total suites\033[0m\n\n"; \
	fi

# ─── Individual functional test suites ──────────────────────────────────────
# Each target applies the same wall-clock budget the combined ze-functional-test
# target gives that suite (see ZE_SUITE_TIMEOUT and the ZE_SUITE_TIMEOUT_<SUITE>
# overrides above), so a stuck suite invoked directly from the CLI also gets
# process-group-killed instead of wedging indefinitely.

ze-functional-encode-test:
	@scripts/dev/ze-run.sh ze-functional-encode-test $(MAKE) --no-print-directory _ze-functional-encode-test-impl

_ze-functional-encode-test-impl: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) bgp encode --all -p $(ZE_ENCODE_PARALLEL)

ze-functional-plugin-test:
	@scripts/dev/ze-run.sh ze-functional-plugin-test $(MAKE) --no-print-directory _ze-functional-plugin-test-impl

_ze-functional-plugin-test-impl: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN_PLUGIN) $(ZE_TEST_RUN) bgp plugin --all -p $(ZE_PLUGIN_PARALLEL)

ze-functional-decode-test:
	@scripts/dev/ze-run.sh ze-functional-decode-test $(MAKE) --no-print-directory _ze-functional-decode-test-impl

_ze-functional-decode-test-impl: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) bgp decode --all

ze-functional-parse-test:
	@scripts/dev/ze-run.sh ze-functional-parse-test $(MAKE) --no-print-directory _ze-functional-parse-test-impl

_ze-functional-parse-test-impl: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) bgp parse --all

ze-functional-reload-test:
	@scripts/dev/ze-run.sh ze-functional-reload-test $(MAKE) --no-print-directory _ze-functional-reload-test-impl

_ze-functional-reload-test-impl: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) bgp reload --all -p 1

ze-functional-ui-test:
	@scripts/dev/ze-run.sh ze-functional-ui-test $(MAKE) --no-print-directory _ze-functional-ui-test-impl

_ze-functional-ui-test-impl: $(ZE_TEST_DEPS_STRIPPED)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) ui --all

ze-functional-editor-test:
	@scripts/dev/ze-run.sh ze-functional-editor-test $(MAKE) --no-print-directory _ze-functional-editor-test-impl

_ze-functional-editor-test-impl: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) editor --all

ze-functional-web-test:
	@scripts/dev/ze-run.sh ze-functional-web-test $(MAKE) --no-print-directory _ze-functional-web-test-impl

_ze-functional-web-test-impl: $(ZE_TEST_DEPS_ZE) $(ZE_WEB_CHAOS_DEP)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(ZE_ALT_CHAOS_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) web --all

ze-functional-managed-test:
	@scripts/dev/ze-run.sh ze-functional-managed-test $(MAKE) --no-print-directory _ze-functional-managed-test-impl

_ze-functional-managed-test-impl: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) managed --all -p 1

ze-functional-l2tp-test:
	@scripts/dev/ze-run.sh ze-functional-l2tp-test $(MAKE) --no-print-directory _ze-functional-l2tp-test-impl

_ze-functional-l2tp-test-impl: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) l2tp --all

ze-functional-firewall-test:
	@scripts/dev/ze-run.sh ze-functional-firewall-test $(MAKE) --no-print-directory _ze-functional-firewall-test-impl

_ze-functional-firewall-test-impl: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) firewall --all

ze-functional-policy-test:
	@scripts/dev/ze-run.sh ze-functional-policy-test $(MAKE) --no-print-directory _ze-functional-policy-test-impl

_ze-functional-policy-test-impl: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) policy --all

# IPsec/IKEv2 suite (test/ipsec/*.ci). It was listed in all_suites above but had
# no run_suite line, so it counted toward the progress denominator and never ran.
# ai/rules/testing.md derives a .ci tag's verify tier from all_suites, so every
# tag in test/ipsec/ was credited a merge-gate tier it did not earn.
ze-functional-ipsec-test:
	@scripts/dev/ze-run.sh ze-functional-ipsec-test $(MAKE) --no-print-directory _ze-functional-ipsec-test-impl

_ze-functional-ipsec-test-impl: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) ipsec --all

ze-functional-appliance-test:
	@scripts/dev/ze-run.sh ze-functional-appliance-test $(MAKE) --no-print-directory _ze-functional-appliance-test-impl

_ze-functional-appliance-test-impl: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) appliance --all

# ldp, rsvpte and install are gating suites (they are in all_suites) that had no
# individual target, so the failure report above named `make ze-functional-ldp-test`
# and make answered "No rule to make target". A suite a run can fail on must be a
# suite a developer can re-run, and the report must be able to name it for EVERY
# member without a list of three exceptions to keep in step.
# TestEverySuiteCanBeRerun (scripts/dev/functional_suite_test.py) holds that true:
# it fails when an all_suites member has no target of this name.
#
# Same shape as the appliance target above, because the loop invokes all four the
# same way: `$(ZE_TEST_RUN) <suite> --all`, no parallelism flag, $(ZE_TEST_DEPS).
ze-functional-ldp-test:
	@scripts/dev/ze-run.sh ze-functional-ldp-test $(MAKE) --no-print-directory _ze-functional-ldp-test-impl

_ze-functional-ldp-test-impl: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) ldp --all

ze-functional-rsvpte-test:
	@scripts/dev/ze-run.sh ze-functional-rsvpte-test $(MAKE) --no-print-directory _ze-functional-rsvpte-test-impl

_ze-functional-rsvpte-test-impl: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) rsvpte --all

ze-functional-install-test:
	@scripts/dev/ze-run.sh ze-functional-install-test $(MAKE) --no-print-directory _ze-functional-install-test-impl

_ze-functional-install-test-impl: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) install --all

# Test-runner primitive suite (test/runner/*.ci). Host-safe: it spawns only
# sh/tail helpers, no ze daemon or privileged tooling, so it stays in the gating
# ze-functional-test run_suite list above.
ze-functional-runner-test:
	@scripts/dev/ze-run.sh ze-functional-runner-test $(MAKE) --no-print-directory _ze-functional-runner-test-impl

_ze-functional-runner-test-impl: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) runner --all

# ─── Non-gated functional test suites ───────────────────────────────────────
# These suites are shipped but not in the default ze-precommit-verify gate. They require
# platform-specific tooling or separate fixture setup.

ze-functional-static-test:
	@scripts/dev/ze-run.sh ze-functional-static-test $(MAKE) --no-print-directory _ze-functional-static-test-impl

_ze-functional-static-test-impl: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) static --all

ze-functional-traffic-test:
	@scripts/dev/ze-run.sh ze-functional-traffic-test $(MAKE) --no-print-directory _ze-functional-traffic-test-impl

_ze-functional-traffic-test-impl: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) traffic --all

# Flow export (sFlow v5 / NetFlow v9 / IPFIX). Like static and traffic, this
# suite needs the Linux daemon and (for packet sampling) CAP_NET_ADMIN +
# kernel psample, so it is release-evidence-only and not in the gating
# ze-functional-test run_suite list above.
ze-functional-flow-export-test:
	@scripts/dev/ze-run.sh ze-functional-flow-export-test $(MAKE) --no-print-directory _ze-functional-flow-export-test-impl

_ze-functional-flow-export-test-impl: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) flow-export --all

ze-functional-vpp-test:
	@scripts/dev/ze-run.sh ze-functional-vpp-test $(MAKE) --no-print-directory _ze-functional-vpp-test-impl

_ze-functional-vpp-test-impl: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) vpp --all

ze-functional-l2tp-wire-test:
	@scripts/dev/ze-run.sh ze-functional-l2tp-wire-test $(MAKE) --no-print-directory _ze-functional-l2tp-wire-test-impl

_ze-functional-l2tp-wire-test-impl: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) l2tp-wire --all

ze-functional-isis-wire-test:
	@scripts/dev/ze-run.sh ze-functional-isis-wire-test $(MAKE) --no-print-directory _ze-functional-isis-wire-test-impl

_ze-functional-isis-wire-test-impl: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) isis-wire --all

ze-functional-ospf-wire-test:
	@scripts/dev/ze-run.sh ze-functional-ospf-wire-test $(MAKE) --no-print-directory _ze-functional-ospf-wire-test-impl

_ze-functional-ospf-wire-test-impl: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) ospf-wire --all

ze-functional-isis-test:
	@scripts/dev/ze-run.sh ze-functional-isis-test $(MAKE) --no-print-directory _ze-functional-isis-test-impl

_ze-functional-isis-test-impl: $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) isis --all

ze-functional-ospf-test:
	@scripts/dev/ze-run.sh ze-functional-ospf-test $(MAKE) --no-print-directory _ze-functional-ospf-test-impl

_ze-functional-ospf-test-impl: ze-functional-test-warm $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) ospf --all

ze-functional-ospfv3-test:
	@scripts/dev/ze-run.sh ze-functional-ospfv3-test $(MAKE) --no-print-directory _ze-functional-ospfv3-test-impl

_ze-functional-ospfv3-test-impl: ze-functional-test-warm $(ZE_TEST_DEPS)
	@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD) $(SUITE_RUN) $(ZE_TEST_RUN) ospfv3 --all

ze-functional-vrrp-test:
	@scripts/dev/ze-run.sh ze-functional-vrrp-test $(MAKE) --no-print-directory _ze-functional-vrrp-test-impl

_ze-functional-vrrp-test-impl: $(ZE_TEST_DEPS)
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
.PHONY: ze-functional-docker-exec-check
ze-functional-docker-exec-check:
	@python3 scripts/dev/docker_exec_checked.py --selftest
	@python3 scripts/dev/docker_exec_checked.py

# The `_<target>-impl` half of every admitted pair defined in this file.
# The public half calls the admission wrapper and this half holds the work;
# see the job-admission block above ZE_RUN_SLOTS in the Makefile.
.PHONY: _ze-functional-ldp-test-impl _ze-functional-rsvpte-test-impl _ze-functional-install-test-impl
.PHONY: _ze-functional-test-warm-impl _ze-functional-test-impl _ze-functional-encode-test-impl _ze-functional-plugin-test-impl _ze-functional-decode-test-impl _ze-functional-parse-test-impl _ze-functional-reload-test-impl _ze-functional-ui-test-impl _ze-functional-editor-test-impl _ze-functional-web-test-impl _ze-functional-managed-test-impl _ze-functional-l2tp-test-impl _ze-functional-firewall-test-impl _ze-functional-policy-test-impl _ze-functional-ipsec-test-impl _ze-functional-appliance-test-impl _ze-functional-runner-test-impl _ze-functional-static-test-impl _ze-functional-traffic-test-impl _ze-functional-flow-export-test-impl _ze-functional-vpp-test-impl _ze-functional-l2tp-wire-test-impl _ze-functional-isis-wire-test-impl _ze-functional-ospf-wire-test-impl _ze-functional-isis-test-impl _ze-functional-ospf-test-impl _ze-functional-ospfv3-test-impl _ze-functional-vrrp-test-impl
