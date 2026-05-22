# Performance benchmarks
#
# Quick reference:
#   make ze-perf-bench                 Run benchmarks against all DUTs (Docker)
#   make ze-perf-bench PERF_DUT=ze     Single DUT
#   make ze-perf-report                Generate comparison report
#   make ze-perf-track                 Update history tracking

.PHONY: ze-perf ze-perf-bench ze-perf-report ze-perf-track

PERF_DUT ?=

ze-perf:
	@echo "Building ze-perf..."
	@mkdir -p bin
	$(GO) build -o bin/ze-perf ./cmd/ze-perf

ze-perf-bench: ze-perf
	@echo "Running performance benchmarks (requires Docker)..."
	@python3 test/perf/run.py --build --test $(PERF_DUT)

ze-perf-report:
	@bin/ze-perf report test/perf/results/*.json --md

ze-perf-track:
	@for f in test/perf/results/*.json; do \
		dut=$$(basename "$$f" .json); \
		bin/ze-perf track "test/perf/history/$${dut}.ndjson" --append "$$f"; \
	done
