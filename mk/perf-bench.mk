# Performance benchmarks -- the suggestion report MOVED to `le`.
#
# That one report now lives in scripts/le/application/perf_bench.py, and the
# target below forwards to it so every existing caller keeps working.
#
# AGENTS AND HUMANS: use `le` directly for it. It is the source of truth.
#
#   ./le perf-bench                            every check in this area
#   ./le perf-bench --list                     what each one is for
#   ./le perf-bench ze-perf-suggestion-report  one of them
#
# FOUR TARGETS DID NOT MOVE, and their recipes are still here. ze-perf-build is
# an alias for the $(ZEBIN_PERF) file target, whose one build recipe lives in
# the root Makefile. ze-perf-bench passes through the shared-machine admission
# wrapper (scripts/dev/ze-run.sh), which re-enters make for the `-impl` half,
# and it needs Docker. ze-perf-report and ze-perf-history-record expand a shell
# glob, the second inside a `for` loop. A gate is one command.
#
# Quick reference:
#   make ze-perf-bench                 Run benchmarks against all DUTs (Docker)
#   make ze-perf-bench PERF_DUT=ze     Single DUT
#   make ze-perf-report                Generate comparison report
#   make ze-perf-history-record        Update history tracking

.PHONY: ze-perf-build ze-perf-bench ze-perf-report ze-perf-history-record ze-perf-suggestion-report

PERF_DUT ?=

ze-perf-suggestion-report:
	@$(CURDIR)/le perf-bench ze-perf-suggestion-report

# --- The four that stayed --------------------------------------------------

# One build recipe for ze-perf, and it lives in the root Makefile beside every
# other binary ($(ZEBIN_PERF): tags 'ze_perf ze_bgp' over ./cmd/ze). This target
# is the named alias, never a second copy -- the copy that used to live here
# built ./cmd/ze-perf, a directory folded into cmd/ze by eac6ec186, so
# `make ze-perf-build` and everything depending on it had been failing since.
ze-perf-build: $(ZEBIN_PERF)

ze-perf-bench:
	@scripts/dev/ze-run.sh ze-perf-bench $(MAKE) --no-print-directory _ze-perf-bench-impl

_ze-perf-bench-impl: ze-perf-build
	@echo "Running performance benchmarks (requires Docker)..."
	@ZE_PERF_BIN=$(CURDIR)/$(ZEBIN_PERF) python3 test/perf/run.py --build --test $(PERF_DUT)
	@python3 scripts/dev/perf-suggest.py --record

ze-perf-report: ze-perf-build
	@$(ZEBIN_PERF) report test/perf/results/*.json --md

ze-perf-history-record: ze-perf-build
	@for f in test/perf/results/*.json; do \
		dut=$$(basename "$$f" .json); \
		$(ZEBIN_PERF) track "test/perf/history/$${dut}.ndjson" --append "$$f"; \
	done
	@python3 scripts/dev/perf-suggest.py --record

# The `_<target>-impl` half of every admitted pair defined in this file.
# The public half calls the admission wrapper and this half holds the work;
# see the job-admission block above ZE_RUN_SLOTS in the Makefile.
.PHONY: _ze-perf-bench-impl
