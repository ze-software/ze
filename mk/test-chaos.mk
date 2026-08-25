# Chaos tests: fault-injection simulation via ze-chaos.
#
# THE LINT AND UNIT GATES MOVED to `le`. They now live in
# scripts/le/application/chaos.py, which carries the reasons this file had
# nowhere to put but a comment. Each one below is a shim: it hands the work to
# the admission wrapper exactly as before, and the wrapper runs `le`.
#
#   ./le test-chaos                  every check
#   ./le test-chaos --list           what each one is for
#   ./le test-chaos ze-chaos-lint    one of them
#
# ze-chaos-unit-test forwards to TWO gates, because its recipe was two runs:
# the simulator's own packages, then the orchestrator CLI that only a ze_chaos
# build compiles. The second is named ze-chaos-cli-unit-test and it can now be
# run on its own, which it could not be as half a recipe.
#
# WHAT STAYED, AND WHY:
#
#   ze-chaos-functional-test, -integration-test, -web-test
#                        they run $(ZEBIN_CHAOS) / $(ZEBIN_TEST), session-scoped
#                        paths resolved by the glob-then-create rule that
#                        mk/helper-session.mk keeps to three implementations
#                        (make, Go, shell). `le` must not become a fourth
#   ze-chaos-test        prerequisite aggregation; the edges ARE the target
#   ze-chaos-verify      the same, under scripts/dev/verify-lock.sh
#
# Quick reference:
#   make ze-chaos-test              All chaos tests (unit + functional + integration + web)
#   make ze-chaos-verify            Lint + all chaos tests
#   make ze-chaos-functional-test   In-process chaos simulation
#   make ze-chaos-integration-test  End-to-end: Ze + chaos peers (.ci tests)
#   make ze-chaos-web-test          Chaos web dashboard HTTP checks

.PHONY: ze-chaos-lint ze-chaos-unit-test ze-chaos-functional-test ze-chaos-integration-test ze-chaos-web-test ze-chaos-test ze-chaos-verify
.PHONY: _ze-chaos-verify-impl

# Chaos simulation parameters. Seed is random by default (printed for reproduction).
# Override: make ze-chaos-functional-test CHAOS_SEED=12345 CHAOS_DURATION=60s CHAOS_PEERS=8
CHAOS_SEED     ?= 0
CHAOS_DURATION ?= 30s
CHAOS_PEERS    ?= 4
CHAOS_ROUTES   ?= 10

ze-chaos-lint:
	@scripts/dev/ze-run.sh ze-chaos-lint $(CURDIR)/le test-chaos ze-chaos-lint

ze-chaos-unit-test:
	@scripts/dev/ze-run.sh ze-chaos-unit-test $(CURDIR)/le test-chaos ze-chaos-unit-test ze-chaos-cli-unit-test

ze-chaos-functional-test: $(ZEBIN_CHAOS)
	@$(ZEBIN_CHAOS) --in-process --duration $(CHAOS_DURATION) \
		--peers $(CHAOS_PEERS) --routes $(CHAOS_ROUTES) \
		--seed $(CHAOS_SEED) --quiet

ze-chaos-integration-test:
	@scripts/dev/ze-run.sh ze-chaos-integration-test $(MAKE) --no-print-directory _ze-chaos-integration-test-impl

_ze-chaos-integration-test-impl: $(ZEBIN_TEST)
	@$(ZEBIN_TEST) bgp chaos --all -t 40s

ze-chaos-web-test:
	@scripts/dev/ze-run.sh ze-chaos-web-test $(MAKE) --no-print-directory _ze-chaos-web-test-impl

_ze-chaos-web-test-impl: $(ZEBIN_TEST)
	@$(ZEBIN_TEST) bgp chaos-web --all

ze-chaos-test: ze-chaos-unit-test ze-chaos-functional-test ze-chaos-integration-test ze-chaos-web-test
	@echo "All chaos tests passed"

# Wrapped in the shared verify lock (see ze-precommit-verify) because chaos tests
# run $(ZEBIN_ZE) instances that would contend with a concurrent ze-precommit-verify.
ze-chaos-verify:
	@scripts/dev/verify-lock.sh ze-chaos-verify $(MAKE) --no-print-directory _ze-chaos-verify-impl

_ze-chaos-verify-impl: ze-chaos-lint ze-chaos-unit-test ze-chaos-functional-test ze-chaos-integration-test ze-chaos-web-test
	@echo "Chaos verification passed"

# The `_<target>-impl` half of every admitted pair defined in this file.
# The public half calls the admission wrapper and this half holds the work;
# see the job-admission block above ZE_RUN_SLOTS in the Makefile.
.PHONY: _ze-chaos-integration-test-impl _ze-chaos-web-test-impl
