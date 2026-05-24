# Functional tests: .ci-based suites run via bin/ze-test
#
# Quick reference:
#   make ze-functional-test    All 12 gating suites
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
#   make ze-static-test        Static routes (release evidence only)
#   make ze-traffic-test       Traffic control (release evidence only)
#   make ze-vpp-test           VPP stub (release evidence only)
#   make ze-l2tp-wire-test     L2TP wire-level (release evidence only)

.PHONY: ze-functional-test
.PHONY: ze-encode-test ze-plugin-test ze-decode-test ze-parse-test ze-reload-test
.PHONY: ze-ui-test ze-editor-test ze-web-test ze-managed-test
.PHONY: ze-l2tp-test ze-firewall-test ze-policy-test
.PHONY: ze-static-test ze-traffic-test ze-vpp-test ze-l2tp-wire-test

# Per-suite wall-clock cap. A stuck subprocess that holds an output pipe open
# can make ze-test's own cmd.Wait() block indefinitely after SIGKILL; `timeout`
# runs the suite in its own process group and signals the whole group on
# expiry, so leaked grandchildren (ze daemons, tacacs-mocks) die with it.
# Exit code 124 from timeout is treated as a suite failure like any other.
# Override: make ze-functional-test ZE_SUITE_TIMEOUT=1200s
ZE_SUITE_TIMEOUT ?= 600s
ZE_SUITE_KILL_AFTER ?= 10s
ZE_ENCODE_PARALLEL ?= 8
ZE_PLUGIN_PARALLEL ?= 2
SUITE_RUN = timeout --kill-after=$(ZE_SUITE_KILL_AFTER) $(ZE_SUITE_TIMEOUT)

# Run ze functional tests (all types, continue on failure to show all results)
# Release evidence matrix: encode, plugin, parse, decode, reload, ui, editor,
# managed, l2tp, firewall, policy, web. Suites not in this list (static,
# traffic, vpp, l2tp-wire, chaos-web) have runners but need platform deps
# or infra.
# ZE_SKIP_SUITES: comma-separated list of suites to skip (e.g. firewall,web
# for Docker environments without agent-browser or native process control).
ZE_SKIP_SUITES ?=
ze-functional-test: bin/ze bin/ze-test
	@failed=0; failed_names=""; skipped_names=""; total=0; \
	run_suite() { \
		suite="$$1"; shift; \
		if echo ",$(ZE_SKIP_SUITES)," | grep -q ",$$suite,"; then \
			skipped_names="$${skipped_names:+$$skipped_names }$$suite"; \
			return 0; \
		fi; \
		total=$$((total + 1)); \
		"$$@" || { failed=$$((failed + 1)); failed_names="$${failed_names:+$$failed_names }$$suite"; }; \
	}; \
	run_suite encode $(SUITE_RUN) bin/ze-test bgp encode --all -p $(ZE_ENCODE_PARALLEL); \
	run_suite plugin $(SUITE_RUN) bin/ze-test bgp plugin --all -p $(ZE_PLUGIN_PARALLEL); \
	run_suite parse $(SUITE_RUN) bin/ze-test bgp parse --all; \
	run_suite decode $(SUITE_RUN) bin/ze-test bgp decode --all; \
	run_suite reload $(SUITE_RUN) bin/ze-test bgp reload --all -p 1; \
	run_suite ui $(SUITE_RUN) bin/ze-test ui --all; \
	run_suite editor $(SUITE_RUN) bin/ze-test editor; \
	run_suite managed $(SUITE_RUN) bin/ze-test managed --all -p 1; \
	run_suite l2tp $(SUITE_RUN) bin/ze-test l2tp --all; \
	run_suite firewall $(SUITE_RUN) bin/ze-test firewall --all; \
	run_suite policy $(SUITE_RUN) bin/ze-test policy --all; \
	run_suite web $(SUITE_RUN) bin/ze-test web --all; \
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

ze-encode-test: bin/ze-test
	@$(SUITE_RUN) bin/ze-test bgp encode --all -p $(ZE_ENCODE_PARALLEL)

ze-plugin-test: bin/ze-test
	@$(SUITE_RUN) bin/ze-test bgp plugin --all -p $(ZE_PLUGIN_PARALLEL)

ze-decode-test: bin/ze-test
	@$(SUITE_RUN) bin/ze-test bgp decode --all

ze-parse-test: bin/ze-test
	@$(SUITE_RUN) bin/ze-test bgp parse --all

ze-reload-test: bin/ze-test
	@$(SUITE_RUN) bin/ze-test bgp reload --all -p 1

ze-ui-test: bin/ze-test
	@$(SUITE_RUN) bin/ze-test ui --all

ze-editor-test: bin/ze-test
	@$(SUITE_RUN) bin/ze-test editor

ze-web-test: bin/ze bin/ze-test
	@$(SUITE_RUN) bin/ze-test web

ze-managed-test: bin/ze-test
	@$(SUITE_RUN) bin/ze-test managed --all -p 1

ze-l2tp-test: bin/ze-test
	@$(SUITE_RUN) bin/ze-test l2tp --all

ze-firewall-test: bin/ze-test
	@$(SUITE_RUN) bin/ze-test firewall --all

ze-policy-test: bin/ze-test
	@$(SUITE_RUN) bin/ze-test policy --all

# ─── Non-gated functional test suites ───────────────────────────────────────
# These suites are shipped but not in the default ze-verify gate. They require
# platform-specific tooling or separate fixture setup.

ze-static-test: bin/ze-test
	@$(SUITE_RUN) bin/ze-test static --all

ze-traffic-test: bin/ze-test
	@$(SUITE_RUN) bin/ze-test traffic --all

ze-vpp-test: bin/ze-test
	@$(SUITE_RUN) bin/ze-test vpp --all

ze-l2tp-wire-test: bin/ze-test
	@$(SUITE_RUN) bin/ze-test l2tp-wire --all
