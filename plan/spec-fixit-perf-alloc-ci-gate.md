# Spec: fixit-perf-alloc-ci-gate

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-07-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `scripts/status/verify_run.go` - `stagesForMode` stage registry (the CI/verify wiring point)
4. `mk/test-release.mk` - `ze-perf-gate` (the manual, Docker-gated perf check)
5. Source files in Current Behavior below

## Task

**[MEDIUM]** Performance discipline is enforced only at **authoring** time, not **integration** time.
Two gaps, both verified 2026-07-16:
- The perf-regression gate never runs in CI. `ze-perf-gate` (`mk/test-release.mk:52-60`, which runs
  `bin/ze-perf track --check`) is invoked only from `ze-release-evidence` and only via `run_if_docker`
  (`mk/test-release.mk:157`), a manual Docker-gated release step. A convergence/throughput/p99
  regression can merge undetected.
- There is NO per-op allocation regression detection anywhere. Benchmarks that would catch it already
  exist and call `b.ReportAllocs()` (`internal/component/bgp/reactor/hotpath_bench_test.go:23,35`, plus
  `forward_update_bench_test.go` and `received_update_bench_test.go`), but NO Makefile/CI target runs
  `go test -bench`/`-benchmem`. The zero-alloc doctrine is enforced only by the Claude-only pre-edit hook
  and scattered `AllocsPerRun` unit tests, neither of which binds a human commit or CI push.

Scope: wire the existing `ReportAllocs` hot-path benchmarks into a CI stage that asserts an **allocs/op
ceiling** (benchstat-style) on the forward / bufmux / EBGPWire benchmarks; and promote a lightweight
subset of the `--check` threshold logic (already at `internal/perf/cli/cmd_track.go:19-23`) into an
~~always-run or scheduled~~ **scheduled-only** pipeline. Low effort: benchmarks, thresholds, and perf tooling all already exist.
→ AUTONOMOUS DEFAULT (2026-07-17) [STAKES: scope/scheduling]: the deterministic allocs/op ceiling gate runs always (in full `ze-verify`/CI); the throughput/convergence/p99 `--check` subset runs **scheduled only**, never on push/PR. Rationale: `internal/perf/regression.go` compares ns-scale timing metrics (convergence-ms, throughput-avg, latency-p99-ms) that are machine-dependent and noise-prone across CI hosts (the Required-Reading Constraint), so gating merges on them would flap; a scheduled ratchet catches drift without blocking the inline/merge path. Thomas: override if wrong.

Related (reference, do NOT duplicate): `spec-release-evidence-gate` owns the heavy Docker perf matrix;
`spec-perf-next-*` are the optimizations this gate protects.

## Required Reading

### Architecture Docs
- [ ] `scripts/status/verify_run.go` - `stagesForMode` (:112) is the enumerated stage registry the verify runner executes; `.woodpecker/verify.yml:19` runs only `make ze-verify`.
  → Decision: add the alloc-ceiling stage by registering it in `stagesForMode` so every CI push and local `ze-verify` runs it, rather than bolting a separate CI step.
  → Constraint: the allocs/op ceiling must be machine-independent. allocs/op is deterministic across hosts; ns/op and throughput are not, so only the alloc ceiling belongs in the always-run gate.
  → AUTONOMOUS DEFAULT (2026-07-17) [STAKES: scope/scheduling]: register the alloc-gate stage in the `default` (full `ze-verify`) branch of `stagesForMode` ONLY, NOT in the `ze-verify-changed` branch. `verify_run.go:120-158` has two stage lists: `ze-verify-changed` (line 121, the fast inline per-edit dev loop) and `default`/`ze-verify` (line 138, what `.woodpecker/verify.yml:19` runs on every push/PR). Rationale: CI runs full `ze-verify`, so AC-3 ("runs on every CI push") is satisfied by the `default` branch alone; keeping the microbench stage out of `ze-verify-changed` honors the brief's "do not block the inline dev loop" and pre-adopts A-2's documented fallback ("move to `ze-verify` only (not `-changed`)") as the chosen design rather than a contingency. Thomas: override if wrong; add `mk("ze-alloc-gate")` to the `ze-verify-changed` list too if you want it inline.
- [ ] `mk/perf.mk`, `mk/test-release.mk` - `ze-perf`, `ze-perf-bench`, `ze-perf-gate` targets.
  → Constraint: `ze-perf-gate` needs Docker (`run_if_docker`) because it runs the throughput/latency matrix; the new alloc gate must NOT require Docker so it can run in the standard CI image.
- [ ] `internal/perf/regression.go`, `internal/perf/cli/cmd_track.go` - `Thresholds`/`CheckHistory` and `--check`.
  → Decision: reuse the existing threshold/regression comparison shape for the promoted lightweight `--check`; do not invent a second regression engine.
- [ ] `ai/rules/memory-architecture.md`, `ai/rules/buffer-first.md` - the zero-alloc doctrine this gate protects.
  → Constraint: the gate asserts a ceiling, not zero, for benchmarks whose steady-state allocs are a known small constant.

**Key insights:**
- allocs/op from `-benchmem` is stable and reproducible; a per-benchmark integer ceiling is a robust regression signal without a stored baseline machine.
- The gate is a **registration** point: a new hot-path benchmark opts in by being listed in the gate's benchmark set, mirroring how fuzz targets register in `mk/test-fuzz.mk`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/hotpath_bench_test.go` - `BenchmarkBufMuxGetReturn` (:21), `...Parallel` (:33), `BenchmarkFwdPoolTryDispatch` (:138); all call `b.ReportAllocs()` (:23,:35,:73,:92,:153).
  → Constraint: these are the bufmux/forward-pool hot-path benchmarks; they already report allocs/op but nothing consumes it.
- [ ] `internal/component/bgp/reactor/forward_update_bench_test.go` - `BenchmarkForwardDirect`; header comment says "Not a CI gate; run on demand".
  → Constraint: this benchmark's own doc-comment names the gap this spec closes.
- [ ] `internal/component/bgp/reactor/received_update_bench_test.go` - `BenchmarkEBGPWireCacheHitParallel` (:19), `b.ReportAllocs()` (:39).
  → Constraint: EBGPWire cache-hit is the RS fan-out hot path (one UPDATE to N eBGP peers).
- [ ] `scripts/status/verify_run.go` - `stagesForMode` (:112) enumerates stages for `ze-verify` / `ze-verify-changed`; no bench/alloc stage exists.
  → Constraint: adding a stage here makes it run in CI (`.woodpecker/verify.yml` calls `make ze-verify`) and locally.
- [ ] `mk/test-release.mk` - `ze-perf-gate` (:52-60) runs `bin/ze-perf track --check`; wired only at `:157` under `run_if_docker perf`.
- [ ] `internal/perf/regression.go` - `Thresholds` (:10), `CheckHistory` (:87); `internal/perf/cli/cmd_track.go` - `--check` thresholds (:19-23).

**Behavior to preserve:**
- Every existing benchmark signature and `ze-perf-gate`/`ze-release-evidence` behavior (the Docker matrix stays where it is).
- `ze-verify` stays the fast pre-commit gate; the new stage must be bounded (short benchtime) so it does not blow the 4-10 min budget.

**Behavior to change:**
- Add an always-run alloc-ceiling gate over the named hot-path benchmarks, registered in the verify stage set; ~~optionally~~ **and (AC-5)** promote a Docker-free `--check` subset into a **scheduled-only** pipeline.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Real:** a CI push/PR event runs `.woodpecker/verify.yml:19` → `make ze-verify` → `verify_run.go` `runVerify` iterates `stagesForMode(...)`. Today that list has no allocation stage, so a per-UPDATE heap allocation reintroduced by a non-Claude edit produces zero signal.

### Transformation Path
1. A developer edits a hot path (forward / bufmux / EBGPWire) and reintroduces a per-op allocation.
2. The existing `ReportAllocs` benchmark now measures a higher allocs/op.
3. The new alloc-gate target runs `go test -run=^$ -bench=<set> -benchmem` at a bounded benchtime.
4. It parses the `allocs/op` column and compares each benchmark against a registered integer ceiling.
5. On any benchmark exceeding its ceiling, the stage exits non-zero and the verify run fails.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Commit/CI push ↔ verify runner | `make ze-verify` → `stagesForMode` stage list | [ ] |
| Benchmark ↔ alloc gate | `-benchmem` allocs/op parsed and compared to a ceiling | [ ] |
| Verify stage ↔ failure routing | new stage name classified by `verify_run.go` | [ ] |

### Integration Points
- `scripts/status/verify_run.go` (`stagesForMode` registration), a new `mk/alloc-gate.mk` alloc-gate target, the named benchmark set, and ~~optionally~~ **(AC-5)** a scheduled `.woodpecker/perf-nightly.yml` job for the Docker-free `--check` subset. Registration over hardcoding: a new hot-path benchmark joins the gate by being listed in the gate's benchmark set, not by editing central logic.

### Architectural Verification
- [ ] No bypassed layers (gate drives the real `ReportAllocs` benchmarks, not a reimplementation).
- [ ] No duplicated functionality (reuse `internal/perf` regression shape for the promoted `--check`; do not fork a second engine).
- [ ] Registration over hardcoding — the stage is added to the `stagesForMode` registry and the benchmark set is an enumerated list.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | allocs/op is deterministic across CI hosts, so an integer ceiling is stable | Go `-benchmem` semantics; allocs/op counts allocations, not time | ceiling flaps; fall back to a tolerance band or scheduled-only | run the gate on CI image + laptop, compare allocs/op | unvalidated |
| A-2 | The named benchmarks run fast enough at a bounded benchtime to fit the ze-verify budget | benchmarks are microbench loops; `-benchtime=<small>` bounds them | stage too slow; move to `ze-verify` only (not `-changed`) or scheduled | time the stage during implement | unvalidated |
| A-3 | The Docker throughput/p99 `--check` cannot run in the standard CI image | `ze-perf-gate` is `run_if_docker`; perf matrix needs Docker DUTs | promote more of `--check` to always-run | inspect `test/perf/run.py` deps | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Ceiling set too tight, gate flaps on legitimate allocs | first CI run red on unchanged code | seed ceilings from a measured run + small headroom; document each ceiling's source |
| R-2 | Stage slows `ze-verify` past its budget | verify wall-clock grows | bounded `-benchtime`, restrict to the smallest hot-path set, or `ze-verify` (full) only |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `make ze-verify` runs the alloc-gate stage | -> | alloc-gate target parses allocs/op and compares to ceiling | `TestAllocGateCeiling` (gate parser unit test) |
| a benchmark exceeds its ceiling | -> | gate exits non-zero, stage fails | `TestAllocGateRegressionFails` |
| `stagesForMode("ze-verify")` includes the stage | -> | `verify_run.go` stage registry | `TestStagesIncludeAllocGate` |

**No `.ci` functional test; internal test-infra opt-out (N/A, no user-facing behavior):** the entry point is `make ze-verify` (a CI/verify stage), not an operator-visible command or wire behavior, so the wiring is proven by the Go unit tests above rather than a `.ci` functional test.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `make ze-verify` on a clean tree | the alloc-gate stage runs the forward/bufmux/EBGPWire benchmarks with `-benchmem` and passes |
| AC-2 | a benchmark's allocs/op exceeds its registered ceiling | the gate exits non-zero and the verify run fails, naming the benchmark |
| AC-3 | `stagesForMode("ze-verify")` and CI | the stage is registered and runs on every CI push (no Docker required) |
| AC-4 | a new hot-path benchmark added to the gate's set | it is gated by adding one list entry (registration), not by editing gate logic |
| AC-5 | Docker-free `--check` subset | a lightweight regression check is promoted to a ~~always-run or~~ **scheduled** pipeline (`.woodpecker/perf-nightly.yml`, `cron` event), referencing (not duplicating) `ze-perf-gate` |

→ AUTONOMOUS DEFAULT (2026-07-17) [STAKES: scope/scheduling]: AC-5's promoted `--check` is **scheduled-only** (a new `.woodpecker/perf-nightly.yml` triggered on `cron`, not `push`/`pull_request`). The repo currently has a single CI file, `.woodpecker/verify.yml` (push + pull_request → `make ze-verify`); the scheduled job is additive and Docker-free (it runs the timing `--check` over stored history, not the Docker DUT matrix, which stays in `ze-perf-gate`/`spec-release-evidence-gate`). Thomas: override if wrong.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAllocGateCeiling` | `internal/perf/allocgate_test.go` | AC-1: parses allocs/op, passes within ceiling | |
| `TestAllocGateRegressionFails` | `internal/perf/allocgate_test.go` | AC-2: over-ceiling exits non-zero | |
| `TestStagesIncludeAllocGate` | `scripts/status/verify_run_test.go` | AC-3: stage registered in `stagesForMode` | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| allocs/op vs ceiling | 0..ceiling | `allocs == ceiling` | N/A | `allocs == ceiling+1` (fails) |

### Functional Tests
Test infrastructure only; no user-facing features. The gate is a CI/verify stage exercised by `make ze-verify`; no `.ci` functional test applies.

## Files to Modify
- `scripts/status/verify_run.go` - register the alloc-gate stage in `stagesForMode` **`default`/`ze-verify` branch (:138) only, not the `ze-verify-changed` branch (:121)** (and add a classifier case if useful).
- ~~`mk/perf.mk` (or a new `mk/alloc-gate.mk`)~~ **`mk/alloc-gate.mk` (new)** - add the alloc-ceiling gate target (`ze-alloc-gate`) that runs the benchmark set with `-benchmem` and enforces per-benchmark ceilings.
- `Makefile` - add `include mk/alloc-gate.mk` (the root Makefile uses explicit per-file includes, `Makefile:81-93`).
- ~~`.woodpecker/` - optional scheduled job~~ **`.woodpecker/perf-nightly.yml` (new) - `cron`-triggered scheduled job** for the Docker-free `--check` subset (AC-5).
- `docs/functional-tests.md` - document the new gate (discovery update).

→ AUTONOMOUS DEFAULT (2026-07-17) [STAKES: scope/arch]: pin the make target to a **new `mk/alloc-gate.mk`** rather than folding it into `mk/perf.mk`. Rationale: `mk/perf.mk`'s targets (`ze-perf-bench`, `mk/perf.mk:18-20`) require Docker (`test/perf/run.py --build --test`); the alloc gate MUST be Docker-free (Required-Reading Constraint) so keeping it in a dedicated file makes the Docker-free boundary structural and prevents accidental coupling, mirroring `mk/test-fuzz.mk`. Cost: one `include` line in the root `Makefile`. Thomas: override if wrong.

## Files to Create
- ~~gate helper (parser + ceiling comparison) under `scripts/status/` or `internal/perf/`~~ **`internal/perf/allocgate.go` + `internal/perf/allocgate_test.go`** - the `-benchmem` `allocs/op` parser plus the per-benchmark integer ceiling registry (a map keyed by benchmark name = the registration list) and the compare-against-ceiling check.
- `.woodpecker/perf-nightly.yml` - the scheduled Docker-free `--check` job (AC-5).

→ AUTONOMOUS DEFAULT (2026-07-17) [STAKES: arch]: locate the parser + ceiling registry in **`internal/perf/`** (not `scripts/status/`). Rationale: it is a perf-domain concern and reuses the existing `internal/perf` regression shape (`regression.go`, per the Required-Reading Decision "do not invent a second regression engine"); `scripts/status/` (`package main`) keeps only the stage registration + `TestStagesIncludeAllocGate` in `verify_run_test.go`. Thomas: override if wrong.

## Implementation Steps
1. **Phase: Wiring (MANDATORY FIRST)** - add the `ze-alloc-gate` make target (in new `mk/alloc-gate.mk`, `include`d from `Makefile`) driving the existing `ReportAllocs` benchmarks (forward/bufmux/EBGPWire) with a bounded `-benchmem`; register the stage in the `default`/`ze-verify` branch of `stagesForMode` **only** (not `ze-verify-changed`). Confirm `make ze-verify` runs it and `make ze-verify-changed` does not.
2. **Phase: ceilings** - measure current allocs/op for each benchmark, set integer ceilings + small headroom, record each ceiling's source (R-1).
3. **Phase: regression proof** - a deliberately-allocating variant trips the gate (AC-2); assert the stage names the offending benchmark.
4. **Phase: promote `--check`** - wire a Docker-free subset of `ze-perf-gate`'s `--check` into a **scheduled-only** pipeline (new `.woodpecker/perf-nightly.yml`, `cron` event), referencing `spec-release-evidence-gate` (AC-5). Do NOT add it to push/pull_request events.
5. **Discovery update** - `docs/functional-tests.md` documents the gate; note the new stage.
6. **Full verification** - `make ze-verify`; complete spec (audit tables, `plan/learned/NNN-<name>.md`, two-commit closure).

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Alloc-gate stage runs in `ze-verify` and in CI without Docker
- [ ] Registration over hardcoding respected (benchmark set is a list; stage registered in `stagesForMode`)
- [ ] Discovery update done (`docs/functional-tests.md`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary case (allocs == ceiling passes, ceiling+1 fails) present

## Review Gate

Independent review recorded: `tmp/review/fixit-perf-alloc-ci-gate-58c51aab-79d8-400d-b779-2c0cf322a274.md`
(reviewer: fresh general-purpose subagent, verified against the real reactor
benchmark files, ran the benchmarks, tests, and lint).

| Severity | Count | Disposition |
|----------|-------|-------------|
| BLOCKER | 0 | — |
| ISSUE | 0 | — |
| NIT | 3 | all acknowledged, none required (see below) |

Verdict: **CLEAN**. 0 BLOCKER, 0 ISSUE. Loop complete.

NIT disposition (each explicitly "not required" by the reviewer):
- **Pipe masks `go test` exit code** (`mk/alloc-gate.mk`): `... | tee LOG` takes
  tee's exit under POSIX `sh`. Safely backstopped by the tested fail-closed
  missing-benchmark guard (a masked build/run failure emits no benchmark lines →
  every registered benchmark flagged Missing → gate fails). Kept: `set -o pipefail`
  is not portable to the default `sh`, and the guard is the real protection.
  Documented in `plan/learned/1198-*.md`.
- **Exported symbols consumed only by in-package tests** (`allocgate.go`):
  `AllocCeilings` is the documented registration point (AC-4), intentionally
  exported as the public opt-in API; `golangci-lint` reports 0 issues. Kept exported.
- **allocs/op integer truncation is a sensitivity floor**: go truncates
  `netallocs/b.N`; only ever under-reports, so no false fails and a real per-op
  regression still trips. Design is sound; noted in the learned file.

Not editing source to address NITs: all are doc-only or accepted-as-designed, and
an edit would invalidate the recorded review hashes for zero functional gain.

## Notes
- Skeleton captured from the 2026-07-16 repository audit (MEDIUM). Benchmarks, thresholds, and perf tooling
  all already exist; the work is wiring, not building. allocs/op belongs in the always-run gate (deterministic);
  throughput/p99 stay in the Docker `ze-perf-gate` (`spec-release-evidence-gate`). Optimizations this gate
  protects live in `spec-perf-next-*` — reference, do not duplicate.
