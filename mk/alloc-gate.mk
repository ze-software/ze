# Allocation-ceiling gate: assert per-op allocation ceilings on hot-path
# benchmarks (spec-fixit-perf-alloc-ci-gate).
#
# Docker-free and deterministic (allocs/op counts allocations, not time, so it
# is machine-independent). Registered as a ze-precommit-verify stage in
# scripts/status/verify_run.go, so every full `make ze-precommit-verify` and CI push runs
# it. Kept OUT of ze-precommit-verify-changed to avoid slowing the inline dev loop.
#
# Registration over hardcoding: a hot-path benchmark opts into the gate by
# adding one entry to perf.AllocCeilings (internal/perf/allocgate.go), NOT by
# editing this file. This target runs every reactor benchmark with -benchmem;
# the checker (TestAllocGateEnforce) enforces ceilings only for registered
# names and fails closed if a registered benchmark is absent from the output.
#
# NOTE (Docker-free boundary): keep this in its own file, separate from
# mk/perf.mk whose ze-perf-bench requires Docker (test/perf/run.py). The
# Docker throughput/p99 matrix stays in `make ze-perf-evidence-update-check`.

.PHONY: ze-alloc-check

# Bounded, count-based benchtime keeps allocs/op stable and the stage fast.
# The log path is absolute: `go test` runs the enforce check with its working
# directory set to the package source dir (internal/perf), not the repo root.
ALLOC_GATE_BENCHTIME ?= 300x
ALLOC_GATE_BENCH_LOG := $(CURDIR)/tmp/verify/alloc-gate-bench.txt

ze-alloc-check:
	@mkdir -p $(CURDIR)/tmp/verify
	@echo "Running reactor hot-path benchmarks (-benchmem) for the alloc-ceiling gate..."
	$(GO_TEST) -run '^$$' -bench '.' -benchmem -benchtime=$(ALLOC_GATE_BENCHTIME) ./internal/component/bgp/reactor/... | tee $(ALLOC_GATE_BENCH_LOG)
	@echo "Enforcing per-benchmark allocs/op ceilings (perf.AllocCeilings)..."
	@ZE_ALLOC_GATE_BENCH=$(ALLOC_GATE_BENCH_LOG) $(GO_TEST) -run '^TestAllocGateEnforce$$' -count=1 ./internal/perf/
