# Spec: rs-fastpath-3-passthrough -- zero-copy pass-through for bgp-rs forwarding

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-rs-fastpath-2-adjrib |
| Phase | - |
| Updated | 2026-04-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec
2. Umbrella: `spec-rs-fastpath-0-umbrella.md`
3. Completed siblings: `plan/learned/NNN-rs-fastpath-1-profile.md`, `plan/learned/NNN-rs-fastpath-2-adjrib.md`
4. `internal/component/bgp/reactor/reactor_api_forward.go` — `ForwardUpdate`
5. `internal/component/bgp/reactor/reactor_notify.go` — event publish
6. `internal/component/bgp/reactor/forward_pool.go` — per-peer TCP writers
7. `internal/component/bgp/reactor/forward_build.go` — modification detection
8. `internal/component/bgp/plugins/rs/server.go` — `dispatchStructured`

## Task

Third child of the `rs-fastpath` umbrella. Structural change. Goal: when no per-destination attribute modification is required, route the received source buffer directly from reactor to each destination's `forward_pool` worker — no rs-plugin RPC round-trip, no copy. When modification is required (AS-PATH prepend, next-hop rewrite, extended-community rewrite, etc.), fall back to the existing `ForwardUpdate` path, which already does a single copy per modifying destination using the destination's Outgoing Peer Pool.

This restores the "zero-copy, copy-on-modify" contract from `rules/design-principles.md` for the route-server case. It also removes the per-UPDATE `updateRoute` RPC hop that currently sits in the hot path.

Preserves: per-source ordering, per-destination egress filters, flow control (pause-source), replay-on-new-peer when adj-rib-in is present. All existing `.ci` tests unchanged.

Depends on child 2 — async adj-rib-in — so that the fast path and the side-subscriber storage do not contend.

## Required Reading

### Architecture Docs

- [ ] `.claude/rules/design-principles.md`
- [ ] `.claude/rules/buffer-first.md`
- [ ] `plan/learned/275-spec-forward-pool.md`
- [ ] `plan/learned/277-rr-ebgp-forward.md`, `269-rr-serial-forward.md`, `289-rr-per-family-forward.md`
- [ ] `plan/learned/434-apply-mods.md`

### RFC Summaries

- [ ] `rfc/short/rfc4271.md` — UPDATE message format, attribute rules.

**Key insights:** (filled during RESEARCH)

## Current Behavior

**Source files read:**
- [ ] `internal/component/bgp/reactor/reactor_notify.go` — how incoming UPDATE is published.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` — `ForwardUpdate`: peer selection, egress filter, AS-PATH rewrite, next-hop policy, `buildModifiedPayload`, fwdItem dispatch.
- [ ] `internal/component/bgp/reactor/forward_pool.go` — per-peer worker pool, fwdItem struct, TCP write.
- [ ] `internal/component/bgp/reactor/forward_build.go` — `buildModifiedPayload`, modification detection.
- [ ] `internal/component/bgp/plugins/rs/server.go` — `dispatchStructured`; per-source worker + `updateRoute` RPC path.
- [ ] `internal/component/bgp/plugins/rs/server_forward.go` — batch flush.
- [ ] `internal/component/bgp/message/*.go` — RawMessage + refcount; Incoming/Outgoing Peer Pool hooks.

**Behavior to preserve:**
- Per-source ordering across all destinations.
- Egress filter pipeline per destination (redistribution, community, prefix, AS-PATH, NEXT_HOP, role/OTC, strip-private).
- Modification logic for destinations that need it (copy-on-modify is unchanged).
- Flow control via `workers.BackpressureDetected` + pause-source.
- Replay-on-new-peer (child 2 already made this work).
- **`Replaying=true` destination gate (`rs/server_handlers.go:68-82`):** while a destination peer is in its replay window, live forwarded routes MUST NOT be delivered to it or duplicates land on the wire (once live, once via replay). Today the gate is enforced in rs's `selectForwardTargets`; the fast path bypasses rs, so the gate state must be reachable from the reactor fast-path target selector, OR rs must continue to own target selection even on the fast path (dispatcher is called with a pre-filtered destination list). Design phase must pick one and wire it through.
- Wire bytes byte-identical to pre-change for pass-through case.

**Behavior to change:**
- Reactor exposes a fast-path dispatcher that accepts `(source, WireUpdate, destinations, no-mod flag)` and hands fwdItems directly to each destination's forward_pool worker, referencing the source buffer (refcount + 1 per destination).
- rs plugin decides "no modification for any destination" at batch level (or at peer-policy level where possible) and takes the fast path for that batch; falls back to `ForwardUpdate` otherwise.
- Source buffer is released to its Incoming Peer Pool once the final destination has sent.

## Data Flow

### Entry Point

- Inbound UPDATE bytes on TCP arrive at a peer session.

### Transformation Path

1. Session `Run()` reads wire bytes into its Incoming Peer Pool buffer.
2. Reactor framing produces a `WireUpdate` reference (zero-copy).
3. Reactor publishes UPDATE event via DirectBridge.
4. Hot-path subscriber = rs plugin.
5. rs classifies the batch: if all destinations require no modification → fast-path; else fall back to `ForwardUpdate`.
6. Fast-path: reactor fast-path dispatcher increments source buffer refcount once per destination and enqueues a pass-through fwdItem on each destination's `forward_pool` worker.
7. `forward_pool` worker writes the source buffer to TCP, then decrements the refcount. When the refcount reaches zero, the buffer is released to the Incoming Peer Pool.
8. Side-path subscriber (adj-rib-in, from child 2) stores asynchronously.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Session ↔ Reactor | WireUpdate reference | [ ] |
| Reactor ↔ rs plugin | DirectBridge event | [ ] |
| Reactor ↔ forward_pool | fwdItem channel (direct, no RPC) | [ ] |
| forward_pool ↔ TCP | direct write of pool buffer | [ ] |

### Integration Points

- `internal/component/bgp/reactor/reactor_api_forward.go` — add fast-path entry or refactor `ForwardUpdate` into a mode switch with a shared core.
- `internal/component/bgp/reactor/forward_pool.go` — accept pass-through fwdItem referring to source buffer + refcount.
- `internal/component/bgp/reactor/forward_build.go` — factor modification-detection into a pre-check so the fast path can skip `buildModifiedPayload`.
- `internal/component/bgp/plugins/rs/server.go` — batch classifier; fast-path call when all destinations are no-mod.
- `internal/component/bgp/message/rawmessage.go` (or equivalent) — refcount increment/decrement semantics.

### Architectural Verification

- [ ] No bypassed layers — rs still owns policy; fast path is invoked only when rs's classifier says "no mod."
- [ ] No unintended coupling.
- [ ] No duplicated functionality — fast path reuses existing `forward_pool` workers.
- [ ] Zero-copy preserved on pass-through; one copy per modifying destination on fall-back.

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Two passive peers + bgp-rs; sender streams UPDATEs with no per-destination mods | → | reactor fast-path dispatcher | `test/plugin/bgp-rs-fastpath.ci` |
| Two passive peers + bgp-rs with AS-PATH rewrite on one destination | → | fall-back to `ForwardUpdate` + copy-on-modify | `test/plugin/bgp-rs-mod-copy.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | 100k IPv4 route bench, ze vs bird | ze throughput ≥ umbrella AC-1 target; first-route ≤ umbrella target; convergence ≤ umbrella target. |
| AC-2 | Pass-through: sender → ze → two receivers, no per-destination mods | Source buffer refcount equals number of destinations during flight; no new buffer allocated per destination. Verified by test-mode instrumentation. |
| AC-3 | Copy-on-modify: sender → ze → two receivers, AS-PATH rewrite on receiver A | Receiver A gets rewritten UPDATE; receiver B gets unchanged UPDATE; exactly one Outgoing Peer Pool buffer allocated for A. |
| AC-4 | Per-source ordering | With N=10000, verify receiver sees UPDATEs in the same order the sender sent them. |
| AC-5 | Backpressure: one destination TCP stalls | Source paused via existing `workers.BackpressureDetected`; no unbounded queue growth; other destinations continue. |
| AC-5b | Destination is in `Replaying=true` when a live UPDATE arrives | Fast path MUST exclude that destination from the target set. Live UPDATE arrives at the destination exactly once (via replay), never twice. Verified by `.ci` test that triggers a mid-stream peer join and asserts no duplicate prefixes arrive. |
| AC-6 | Wire bytes | For pass-through case, ze's outbound bytes are byte-identical to the inbound bytes (modulo framing, modulo any pre-existing RFC 7606 sanitisation). Verified by hex comparison in `.ci` test. |
| AC-7 | All existing `.ci` tests | Pass unchanged. |
| AC-8 | `make ze-test`, `make ze-verify-fast`, `make ze-race-reactor` | All clean. |
| AC-9 | `BenchmarkForwardPassThrough` | Per-UPDATE in-process hot path ≥ 500k UPDATE/s/core (verified against Phase 1 profile baseline). |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestForwardFastPathNoCopy` | `internal/component/bgp/reactor/forward_pool_test.go` | Pass-through: source buffer refcount matches destination count; no new buffer allocated. | |
| `TestForwardModifyCopyPerDest` | `internal/component/bgp/reactor/forward_build_test.go` | Exactly one Outgoing Peer Pool buffer allocated per modifying destination. | |
| `TestForwardFastPathOrdering` | `internal/component/bgp/reactor/forward_pool_test.go` | Per-source ordering preserved across destinations. | |
| `TestForwardFastPathBackpressure` | `internal/component/bgp/reactor/forward_pool_test.go` | `BackpressureDetected` still fires when a destination channel crosses the high-water mark. | |
| `TestRSClassifierNoMod` | `internal/component/bgp/plugins/rs/server_test.go` | Classifier returns "fast path" when all destinations' policies are no-op. | |
| `TestRSClassifierWithMod` | `internal/component/bgp/plugins/rs/server_test.go` | Classifier returns "fall-back" when any destination's policy mutates attributes. | |
| `BenchmarkForwardPassThrough` | `internal/component/bgp/reactor/forward_pool_bench_test.go` | Per-UPDATE in-process hot path meets AC-9 target. | |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Source refcount | 0..N destinations | N | — | N+1 (indicates leak) |
| Destination fwdItem channel depth | inherited from forward_pool sizing | — | — | — |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-rs-fastpath` | `test/plugin/bgp-rs-fastpath.ci` | Two passive peers + bgp-rs; sender streams 1000 UPDATEs with no policy mods; receiver delivered all 1000; wire bytes byte-identical. | |
| `bgp-rs-mod-copy` | `test/plugin/bgp-rs-mod-copy.ci` | Two passive peers + bgp-rs with AS-PATH rewrite on one destination; rewrite applied to that destination, unchanged on the other. | |

### Future

- None.

## Files to Modify

- `internal/component/bgp/reactor/reactor_api_forward.go` — add fast-path entry; factor modification-decision into pre-check.
- `internal/component/bgp/reactor/reactor_notify.go` — wire rs to reactor fast-path dispatcher.
- `internal/component/bgp/reactor/forward_pool.go` — accept pass-through fwdItem; refcount decrement on send complete.
- `internal/component/bgp/reactor/forward_build.go` — refactor modification detection for reuse.
- `internal/component/bgp/plugins/rs/server.go` — classifier + fast-path invocation.
- `internal/component/bgp/plugins/rs/server_forward.go` — batch classifier hook.
- `internal/component/bgp/message/rawmessage.go` — refcount semantics (if not already present).

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] | — |
| CLI commands | [ ] | — |
| Editor autocomplete | [ ] | — |
| Functional test for new behavior | [ ] | `test/plugin/bgp-rs-fastpath.ci`, `test/plugin/bgp-rs-mod-copy.ci` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` — "zero-copy pass-through" |
| 2 | Config syntax changed? | [ ] | — |
| 3 | CLI command added/changed? | [ ] | — |
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` — `updateRoute` RPC no longer on hot path for pass-through |
| 5 | Plugin added/changed? | [ ] | — |
| 6 | Has a user guide page? | [ ] | `docs/guide/<route-server>.md` — note pass-through is zero-copy |
| 7 | Wire format changed? | [ ] | — (byte-identical) |
| 8 | Plugin SDK/protocol changed? | [ ] | — |
| 9 | RFC behavior implemented? | [ ] | — |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` if benchmark naming changes |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md`, `docs/performance.md` — umbrella updates these on close |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` — forwarding fast path + onion-slice invariants |

## Files to Create

- `test/plugin/bgp-rs-fastpath.ci`
- `test/plugin/bgp-rs-mod-copy.ci`
- `internal/component/bgp/reactor/forward_pool_bench_test.go`
- `plan/learned/NNN-rs-fastpath-3-passthrough.md` (on completion)

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella |
| 2. Audit | Files to Modify, Files to Create |
| 3. Implement (TDD) | Phases below |
| 4. `/ze-review` gate | Review Gate |
| 5. Full verification | `make ze-test`, `make ze-verify-fast`, `make ze-race-reactor`, `test/perf/run.py --test ze bird` |
| 6–9. Critical review + fixes | Critical Review Checklist |
| 10. Deliverables review | Deliverables Checklist |
| 11. Security review | Security Review Checklist |
| 12. Re-verify | Re-run stage 5 |
| 13. Executive Summary | Per `rules/planning.md` |

### Implementation Phases

1. **Phase 1 — Refcount + Incoming Peer Pool release.** Confirm or add refcount semantics on the source buffer. Teach forward_pool to decrement on send and release to the correct pool on zero.
   - Tests: `TestForwardFastPathNoCopy`.
   - Files: `message/rawmessage.go`, `forward_pool.go`.
   - Verify: unit test passes; no leaks under race.
2. **Phase 2 — Reactor fast-path dispatcher.** Factor modification-decision into a pre-check. Add reactor entry that accepts pass-through fwdItem and dispatches directly to destination forward_pool workers.
   - Tests: `TestForwardFastPathOrdering`, `TestForwardFastPathBackpressure`.
   - Files: `reactor_api_forward.go`, `reactor_notify.go`, `forward_build.go`.
   - Verify: unit tests pass; per-source ordering preserved; backpressure still fires.
3. **Phase 3 — rs classifier + fast-path call.** rs batch classifier; when no-mod for all destinations, invoke reactor fast-path; else fall back.
   - Tests: `TestRSClassifierNoMod`, `TestRSClassifierWithMod`, `bgp-rs-fastpath.ci`, `bgp-rs-mod-copy.ci`.
   - Files: `rs/server.go`, `rs/server_forward.go`.
   - Verify: functional tests pass; wire bytes byte-identical on pass-through.
4. **Phase 4 — Benchmark + soak.** `BenchmarkForwardPassThrough`; `test/perf/run.py` full run.
   - Tests: `BenchmarkForwardPassThrough`.
   - Files: `forward_pool_bench_test.go`.
   - Verify: AC-1 met; bench recorded; `make ze-race-reactor` clean.
5. **Functional tests** → in phases 2/3.
6. **Full verification** → `make ze-verify-fast`, `make ze-race-reactor`, `test/perf/run.py`.
7. **Complete spec** → audit tables, learned summary.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has test + file:line. AC-9 benchmark result pasted in spec. |
| Correctness | Wire bytes byte-identical for pass-through. Modification still correct for fall-back. |
| Rule: no-layering | Old per-UPDATE RPC path for the no-mod case removed, not co-existing. |
| Rule: goroutine-lifecycle | No new per-event goroutines; all workers long-lived. |
| Rule: buffer-first | No new `append` or `make([]byte)` on wire paths. |
| Rule: zero-copy | Pass-through case: source buffer refcount equals destination count; no new buffer allocated. |
| Rule: design-principles | Copy-on-modify is the only copy allowed. |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `BenchmarkForwardPassThrough` meets AC-9 | `go test -run=^$ -bench=BenchmarkForwardPassThrough ./internal/component/bgp/reactor/...` |
| `.ci` tests pass | `bin/ze-test plugin -p bgp-rs-fastpath`, `... bgp-rs-mod-copy` |
| ze-perf 100k meets umbrella AC-1 | `python3 test/perf/run.py --test ze bird`, read `test/perf/results/ze.json` |
| Wire bytes identical | `bgp-rs-fastpath.ci` asserts hex equality |
| Learned summary | `ls plan/learned/*rs-fastpath-3-passthrough*.md` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | No change to wire parsing; bounds checks unchanged. |
| Resource exhaustion | Refcount cannot underflow; fwdItem channel bounded. |
| Error leakage | Classifier errors log but do not crash; fall-back path is the safe default. |
| Concurrency | `make ze-race-reactor` clean; refcount uses atomic. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Wire diff on pass-through | Fix in Phase 3; classifier marked a mod-required case as no-mod |
| Ordering test fails | Fix in Phase 2; fast-path dispatcher broke per-source serialisation |
| Backpressure test fails | Fix in Phase 2; fwdItem enqueue must participate in high-water-mark check |
| Benchmark below target | Identify the next hop with pprof; do not claim done |
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

<!-- LIVE -->

## RFC Documentation

- RFC 4271 — UPDATE format; pass-through must reproduce wire bytes verbatim.
- RFC 7606 — attribute sanitisation on receive (ze already applies this pre-forward; no change here).

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
- [ ] ze-perf 100k meets umbrella AC-1
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
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
- [ ] Write learned summary to `plan/learned/NNN-rs-fastpath-3-passthrough.md`
- [ ] Summary included in commit
