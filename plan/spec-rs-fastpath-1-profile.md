# Spec: rs-fastpath-1-profile -- measure the bottleneck, tune the easy knobs

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-04-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec
2. Umbrella: `spec-rs-fastpath-0-umbrella.md`
3. `test/perf/run.py`, `test/perf/configs/ze.conf`
4. `internal/component/bgp/plugins/rs/server.go` (`dispatchStructured`, `forwardLoop`)
5. `internal/component/bgp/plugins/rs/server_forward.go` (batch flush)
6. `internal/component/bgp/plugins/rs/worker.go` (per-source worker pool)
7. `internal/component/bgp/plugins/adj_rib_in/rib.go` (BART insert cost)

## Task

First child of the `rs-fastpath` umbrella. Goal: turn "ze is 16× slower than bird at 100k routes" into a named, profile-backed bottleneck. Sibling children (`-2-adjrib`, `-3-passthrough`) depend on this evidence.

Produce: (a) CPU and allocation profiles for ze at 10k/25k/50k/75k/100k routes, (b) gctrace at 100k, (c) a ranked list of top cost centres with percentages, (d) a trial of the two cheapest knobs (`forwardCh` depth, batch flush on time-or-count) with before/after numbers. Changes that survive: only those showing measurable throughput or latency improvement against the baseline. Changes that do not survive: reverted within this child. Target throughput for the umbrella's AC-1 is set once this child lands.

## Required Reading

### Architecture Docs

- [ ] `.claude/rules/design-principles.md`
- [ ] `plan/learned/417-perf.md`
- [ ] `plan/learned/424-forward-backpressure.md`, `519-fwd-auto-sizing.md`

### RFC Summaries

- [ ] `rfc/short/rfc4271.md`

**Key insights:** (filled during RESEARCH)

## Current Behavior

**Source files read:**
- [ ] `internal/component/bgp/plugins/rs/server.go` — `dispatchStructured`, `forwardLoop`, worker dispatch.
- [ ] `internal/component/bgp/plugins/rs/server_forward.go` — batch flush logic.
- [ ] `internal/component/bgp/plugins/rs/worker.go` — per-source worker pool + `BackpressureDetected`.
- [ ] `test/perf/run.py` — Docker harness, env vars (`DUT_ROUTES`, `DUT_SEED`, `DUT_REPEAT`).
- [ ] `cmd/ze/main.go` — wire up an optional pprof endpoint (check if one already exists).

**Behavior to preserve:**
- Per-source ordering of forwarded UPDATEs.
- Pause-source-on-backpressure behaviour.
- All existing `.ci` tests pass unchanged.
- Default (unflagged) ze behaviour must be byte-identical on the wire to pre-change.

**Behavior to change:**
- Batch flush: from "wait until batch full" to "flush on K routes OR T milliseconds, whichever first." K and T are the tuning knobs this child sets.
- `forwardCh` buffer depth: from current `16` to a value motivated by the in-flight-route calculation; recorded in code + spec.
- Dev-only pprof gate in `test/perf/run.py` (env-var opt-in).

## Data Flow

### Entry Point

- Benchmark sender opens BGP session to ze, streams N UPDATE messages; benchmark receiver is the second peer. Ze's rs plugin forwards. This child does not change the entry point; it measures the existing path.

### Transformation Path

1. Sender → ze TCP receive → session buffer → reactor event.
2. DirectBridge deliveryLoop → rs `dispatchStructured` (stores forwardCtx, dispatches via per-source worker).
3. Worker → `processForward` → `batchForwardUpdate` → `flushBatch` → `asyncForward` → `forwardCh` → `forwardLoop` sender (×4).
4. `updateRoute` RPC → reactor `ForwardUpdate` → `buildModifiedPayload` → fwdItem → `forward_pool` worker → TCP write to receiver.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ rs plugin | DirectBridge event (in-process) | [ ] |
| rs ↔ reactor | `updateRoute` RPC | [ ] |
| Reactor ↔ forward_pool | fwdItem channel | [ ] |

### Integration Points

- `test/perf/run.py` gains an optional `PPROF=1` env var that maps an extra port from the ze container and captures CPU + heap profiles to `tmp/perf-run/pprof/`.
- No code changes beyond rs batch-flush tuning and `forwardCh` depth, plus the benchmark-only pprof gate.

### Architectural Verification

- [ ] No bypassed layers.
- [ ] No unintended coupling.
- [ ] No duplicated functionality.
- [ ] Zero-copy preserved where applicable.

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `PPROF=1 python3 test/perf/run.py --test ze` | → | pprof endpoint + harness port-map | `test/plugin/bgp-rs-perf-pprof.ci` |
| `ze-perf run --routes 10000` against tuned ze | → | rs batch flush + forwardCh depth | `test/plugin/bgp-rs-batch-flush.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Profile capture at 100k routes | CPU and alloc profile files exist under `tmp/perf-run/pprof/`; top-5 functions by CPU and by allocations recorded in spec's Design Insights with percentages. |
| AC-2 | gctrace at 100k routes | `GODEBUG=gctrace=1` output captured; GC pause time and frequency recorded. |
| AC-3 | Scaling sweep 10k/25k/50k/75k/100k, 3-iter | Throughput and first-route numbers recorded in spec's Design Insights; regression vs 2026-04-17 baseline flagged (better / same / worse). |
| AC-4 | Batch flush on K routes OR T milliseconds | Single UPDATE forwarded within T ms (no unbounded wait for batch fill). K and T values recorded. Verified by unit test. |
| AC-5 | forwardCh depth set to N | N documented in code as a named constant (not a magic literal) with a comment naming the formula used. |
| AC-6 | Backpressure preserved | Unit test demonstrates pause-source still fires when destination channel crosses high-water mark. |
| AC-7 | All existing `test/*` tests | Pass unchanged. |
| AC-8 | `make ze-test`, `make ze-verify-fast`, `make ze-race-reactor` | All clean. |
| AC-9 | Umbrella target set | This child updates `spec-rs-fastpath-0-umbrella.md` AC-1 row with concrete throughput and first-route numbers based on profile findings. |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRSFlushOnCountBound` | `internal/component/bgp/plugins/rs/server_forward_test.go` | After K routes arrive in a single batch, flush is triggered regardless of timer. | |
| `TestRSFlushOnTimeBound` | `internal/component/bgp/plugins/rs/server_forward_test.go` | After T ms elapse with fewer than K routes in batch, flush is triggered. | |
| `TestRSBackpressurePreserved` | `internal/component/bgp/plugins/rs/worker_test.go` | Pause-source still fires when high-water mark is crossed. | |
| `TestForwardChDepthNamed` | `internal/component/bgp/plugins/rs/server_test.go` | `forwardCh` depth is set from a named constant with documented formula. | |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Batch K | 1..max UPDATEs per TCP send | TBD set in design | 0 | TBD |
| Batch T (ms) | 0..100 | TBD set in design | — | TBD |
| forwardCh depth | bounded by in-flight budget | TBD set in design | 0 | TBD |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-rs-perf-pprof` | `test/plugin/bgp-rs-perf-pprof.ci` | `PPROF=1` env on ze container exposes pprof endpoint; a profile can be captured and parsed. | |
| `bgp-rs-batch-flush` | `test/plugin/bgp-rs-batch-flush.ci` | Two passive peers + bgp-rs; sender sends 1 route; receiver gets it within T ms of arrival (no wait for batch). | |

### Future

- None. All tests ship with this child.

## Files to Modify

- `internal/component/bgp/plugins/rs/server.go` — `forwardCh` depth constant + comment.
- `internal/component/bgp/plugins/rs/server_forward.go` — batch flush on K OR T.
- `internal/component/bgp/plugins/rs/worker.go` — no functional change (test only).
- `test/perf/run.py` — optional `PPROF=1` gate.
- `cmd/ze/main.go` or equivalent — dev-gated pprof endpoint if not already present.
- `plan/spec-rs-fastpath-0-umbrella.md` — update AC-1 row with measured target (append-only amendment, per spec rules).

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] | — |
| CLI commands | [ ] | — |
| Editor autocomplete | [ ] | — |
| Functional test for new behavior | [ ] | `test/plugin/bgp-rs-batch-flush.ci`, `test/plugin/bgp-rs-perf-pprof.ci` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | — |
| 2 | Config syntax changed? | [ ] | — |
| 3 | CLI command added/changed? | [ ] | — |
| 4 | API/RPC added/changed? | [ ] | — |
| 5 | Plugin added/changed? | [ ] | — |
| 6 | Has a user guide page? | [ ] | — |
| 7 | Wire format changed? | [ ] | — |
| 8 | Plugin SDK/protocol changed? | [ ] | — |
| 9 | RFC behavior implemented? | [ ] | — |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` if PPROF gate added to harness |
| 11 | Affects daemon comparison? | [ ] | — (umbrella owns the final numbers) |
| 12 | Internal architecture changed? | [ ] | — |

## Files to Create

- `test/plugin/bgp-rs-perf-pprof.ci`
- `test/plugin/bgp-rs-batch-flush.ci`
- `internal/component/bgp/plugins/rs/server_forward_test.go` (new test file if not present)
- `tmp/perf-run/pprof/` (profile artefacts — not committed; listed for evidence)
- `plan/learned/NNN-rs-fastpath-1-profile.md` (on completion)

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella |
| 2. Audit | Files to Modify, Files to Create |
| 3. Implement (TDD) | Phases below |
| 4. `/ze-review` gate | Review Gate |
| 5. Full verification | `make ze-test`, `make ze-verify-fast` |
| 6–9. Critical review + fixes | Critical Review Checklist |
| 10. Deliverables review | Deliverables Checklist |
| 11. Security review | Security Review Checklist |
| 12. Re-verify | Re-run stage 5 |
| 13. Executive Summary | Per `rules/planning.md` |

### Implementation Phases

1. **Phase 1 — pprof + gctrace capture.** Enable a dev-gated pprof endpoint in the ze container. Add `PPROF=1` to `test/perf/run.py`. Run sweep; save CPU + heap + gctrace artefacts.
   - Tests: `bgp-rs-perf-pprof.ci`.
   - Files: `cmd/ze/main.go` (if pprof not present), `test/perf/run.py`.
   - Verify: artefacts exist; Design Insights records top-5 CPU and alloc functions.
2. **Phase 2 — batch flush tuning.** Replace "wait for batch full" with "flush on K routes OR T ms."
   - Tests: `TestRSFlushOnCountBound`, `TestRSFlushOnTimeBound`, `bgp-rs-batch-flush.ci`, `TestRSBackpressurePreserved`.
   - Files: `internal/component/bgp/plugins/rs/server_forward.go`, `server_forward_test.go`.
   - Verify: unit tests pass; 100k-route bench re-run shows improved first-route latency.
3. **Phase 3 — forwardCh depth.** Replace literal `16` with a named constant + comment citing the formula. Sweep values against bench.
   - Tests: `TestForwardChDepthNamed`.
   - Files: `internal/component/bgp/plugins/rs/server.go`.
   - Verify: bench improves or stays equal; the winning depth is documented.
4. **Phase 4 — Set umbrella target.** With phases 1–3 baseline numbers in hand, amend `spec-rs-fastpath-0-umbrella.md` AC-1 with the concrete throughput and first-route target that children 2 and 3 must meet.
5. **Functional tests** → created in phases 1 and 2.
6. **Full verification** → `make ze-verify-fast`, `make ze-race-reactor`.
7. **Complete spec** → audit tables, learned summary.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has evidence. AC-9 landed: umbrella's AC-1 updated. |
| Correctness | Batch flush fires within T ms for a single UPDATE; K-count path unchanged. |
| Rule: no-layering | "Wait for batch full" code path fully removed, not co-existing. |
| Rule: goroutine-lifecycle | Any new timer is long-lived (`AfterFunc` or drained `NewTimer`); no per-event goroutines added. |
| Rule: buffer-first | No new `make([]byte)` on forwarding paths. |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Profile artefacts | `ls tmp/perf-run/pprof/` |
| Sweep numbers recorded | `grep -n "Scaling sweep" plan/spec-rs-fastpath-1-profile.md` returns Design Insights data |
| Umbrella AC-1 updated | `grep "AC-1" plan/spec-rs-fastpath-0-umbrella.md` shows concrete numbers |
| `.ci` tests pass | `bin/ze-test plugin -p bgp-rs-batch-flush`, `... bgp-rs-perf-pprof` |
| Learned summary | `ls plan/learned/*rs-fastpath-1-profile*.md` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | pprof endpoint must be dev-gated (env var or build tag), not enabled in release. |
| Resource exhaustion | New timer stopped/drained on Stop; new channel depth bounded. |
| Error leakage | pprof endpoint must not be reachable from non-localhost by default. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Profile inconclusive | Back to Phase 1; add targeted timing marks, not more knobs |
| Phase 2 regresses backpressure test | Fix in Phase 2; do not weaken the test |
| Phase 3 tuning gives no improvement | Document and revert; keep literal `16` with a comment explaining the bench result |
| 3 fix attempts fail | STOP. Ask user. |

## Mistake Log

### Wrong Assumptions

| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches

| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates

| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

<!-- LIVE — profile findings and target numbers land here -->

## RFC Documentation

- RFC 4271 — MRAI advisory; flush-timer T must be small (≤ 10 ms expected) and must not drift towards the 30 s advisory value.

## Implementation Summary

### What Was Implemented

### Bugs Found/Fixed

### Documentation Updates

### Deviations from Plan

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan

| File | Status | Notes |
|------|--------|-------|

### Audit Summary

- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Review Gate

### Run 1 (initial)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

### Run 2+ (re-runs until clean)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status

- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)

- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] `make ze-verify-fast` passes
- [ ] `make ze-race-reactor` passes
- [ ] Umbrella AC-1 updated with concrete target
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)

- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design

- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit
- [ ] Minimal coupling

### TDD

- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)

- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-rs-fastpath-1-profile.md`
- [ ] Summary included in commit
