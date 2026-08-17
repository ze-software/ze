# Performance benchmarks
#
# Quick reference:
#   make ze-perf-bench                 Run benchmarks against all DUTs (Docker)
#   make ze-perf-bench PERF_DUT=ze     Single DUT
#   make ze-perf-report                Generate comparison report
#   make ze-perf-history-record                 Update history tracking

.PHONY: ze-perf-build ze-perf-bench ze-perf-report ze-perf-history-record ze-perf-suggestion-report

PERF_DUT ?=

# One build recipe for ze-perf, and it lives in the root Makefile beside every
# other binary ($(ZEBIN_PERF): tags 'ze_perf ze_bgp' over ./cmd/ze). This target
# is the named alias, never a second copy -- the copy that used to live here
# built ./cmd/ze-perf, a directory folded into cmd/ze by eac6ec186, so
# `make ze-perf-build` and everything depending on it had been failing since.
ze-perf-build: $(ZEBIN_PERF)

ze-perf-bench: ze-perf-build
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

# Advisory: if BGP data-plane code changed since the last perf run, suggest one.
# A NUDGE, never a gate -- always exits 0. The heavy suite (ze-evidence-perf-record) needs
# Docker and minutes, so it is not run every edit; this notices when a Docker
# perf run is overdue on THIS machine. It complements the scheduled Docker-free
# regression check (.github/workflows/perf-nightly.yml): that guards the committed
# NDJSON history on every nightly, this nudges the developer to refresh it with a
# local Docker run. See scripts/dev/perf-suggest.py.
ze-perf-suggestion-report:
	@python3 scripts/dev/perf-suggest.py
