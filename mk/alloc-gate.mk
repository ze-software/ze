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
# editing this file. This target runs every benchmark of ALLOC_GATE_PACKAGES
# with -benchmem; the checker (TestAllocGateEnforce) enforces ceilings only for
# registered names and fails closed if a registered benchmark is absent from the
# output.
#
# ALLOC_GATE_PACKAGES is the one thing a NEW package costs, and it is a package
# list rather than a benchmark list so the registry stays the registration
# point. A package holding a registered benchmark and missing from this list is
# a permanent red, not a silent pass: the enforce check reports the name as
# absent. TestAllocGateCoversRecordPath (internal/perf/allocgate_test.go) reads
# the variable below and asserts the record path's package is covered, so the
# two files cannot drift apart in silence.
#
# NOTE (Docker-free boundary): keep this in its own file, separate from
# mk/perf.mk whose ze-perf-bench requires Docker (test/perf/run.py). The
# Docker throughput/p99 matrix stays in `make ze-evidence-perf-record`.

.PHONY: ze-alloc-check

# Bounded, count-based benchtime keeps allocs/op stable and the stage fast.
# The log path is absolute: `go test` runs the enforce check with its working
# directory set to the package source dir (internal/perf), not the repo root.
ALLOC_GATE_BENCHTIME ?= 300x
ALLOC_GATE_BENCH_LOG := $(CURDIR)/tmp/verify/alloc-gate-bench.txt

# The packages whose benchmarks the gate runs. internal/component/plugin is
# named without `/...` on purpose: the record-path benchmark lives in that
# package, and its server/ subpackage benchmarks spawn plugin processes, which
# is a suite rather than an allocation measurement.
ALLOC_GATE_PACKAGES ?= ./internal/component/bgp/reactor/... ./internal/component/plugin

ze-alloc-check:
	@scripts/dev/ze-run.sh ze-alloc-check $(MAKE) --no-print-directory _ze-alloc-check-impl

_ze-alloc-check-impl:
	@mkdir -p $(CURDIR)/tmp/verify
	@echo "Running hot-path benchmarks (-benchmem) for the alloc-ceiling gate..."
	$(GO_TEST) -run '^$$' -bench '.' -benchmem -benchtime=$(ALLOC_GATE_BENCHTIME) $(ALLOC_GATE_PACKAGES) | tee $(ALLOC_GATE_BENCH_LOG)
	@echo "Enforcing per-benchmark allocs/op ceilings (perf.AllocCeilings)..."
	@ZE_ALLOC_GATE_BENCH=$(ALLOC_GATE_BENCH_LOG) $(GO_TEST) -run '^TestAllocGateEnforce$$' -count=1 ./internal/perf/

# The `_<target>-impl` half of every admitted pair defined in this file.
# The public half calls the admission wrapper and this half holds the work;
# see the job-admission block above ZE_RUN_SLOTS in the Makefile.
.PHONY: _ze-alloc-check-impl
