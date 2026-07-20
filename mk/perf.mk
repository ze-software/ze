# Performance benchmarks
#
# Quick reference:
#   make ze-perf-bench                 Run benchmarks against all DUTs (Docker)
#   make ze-perf-bench PERF_DUT=ze     Single DUT
#   make ze-perf-report                Generate comparison report
#   make ze-perf-track                 Update history tracking

.PHONY: ze-perf ze-perf-bench ze-perf-report ze-perf-track ze-perf-suggest

PERF_DUT ?=

ze-perf:
	@echo "Building ze-perf..."
	@mkdir -p bin
	$(GO) build -o bin/ze-perf ./cmd/ze-perf

ze-perf-bench: ze-perf
	@echo "Running performance benchmarks (requires Docker)..."
	@python3 test/perf/run.py --build --test $(PERF_DUT)
	@python3 scripts/dev/perf-suggest.py --record

ze-perf-report:
	@bin/ze-perf report test/perf/results/*.json --md

ze-perf-track:
	@for f in test/perf/results/*.json; do \
		dut=$$(basename "$$f" .json); \
		bin/ze-perf track "test/perf/history/$${dut}.ndjson" --append "$$f"; \
	done
	@python3 scripts/dev/perf-suggest.py --record

# Advisory: if BGP data-plane code changed since the last perf run, suggest one.
# A NUDGE, never a gate -- always exits 0. The heavy suite (ze-perf-gate) needs
# Docker and minutes, so it is not run every edit; this notices when a run is
# overdue. Deliberately local: it replaced a nightly Woodpecker pipeline, which
# ran a heavy sweep on Codeberg's donated runners to catch something reproducible
# on the developer's own machine. See scripts/dev/perf-suggest.py.
ze-perf-suggest:
	@python3 scripts/dev/perf-suggest.py
