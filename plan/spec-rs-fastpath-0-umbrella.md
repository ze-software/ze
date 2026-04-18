# Spec: rs-fastpath-0-umbrella -- restore route-server forwarding throughput

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-04-18 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec (umbrella)
2. Child specs: `spec-rs-fastpath-1-profile.md`, `spec-rs-fastpath-2-adjrib.md`, `spec-rs-fastpath-3-passthrough.md`
3. `.claude/rules/planning.md` — workflow rules (Spec Sets section)
4. `.claude/rules/design-principles.md` — zero-copy, copy-on-modify, encapsulation onion
5. `test/perf/run.py` and `test/perf/configs/ze.conf` — benchmark harness
6. `plan/learned/068-spec-remove-adjrib-integration.md` — prior adj-rib decoupling (don't repeat)
7. `plan/learned/417-perf.md` — ze-perf tool design

## Task

Ze's route-server forwarding throughput has regressed relative to pre-RIB baseline. At 10k IPv4 routes ze sustains ~204k rps with 2 ms first-route; at 100k routes it drops to ~49k rps with ~2.5 s first-route — 16× behind bird (~780k rps, 2 ms first-route) on the same Docker harness, with superlinear degradation above ~75k routes. User constraint: **the RIB must not slow down packet forwarding.**

Goal of this umbrella: coordinate the investigation and the structural changes that restore per-route forwarding cost to the pre-RIB level (no scaling cliff, no RIB on the hot path) while preserving all existing semantics (filters, AS-PATH rewrite, NEXT_HOP policy, flow control, replay-on-new-peer).

Numeric throughput target is **deferred** to Phase 1 child (`spec-rs-fastpath-1-profile.md`). Target is set into this umbrella's Acceptance Criteria once profiling evidence names the achievable ceiling on the benchmark hardware.

## Child Specs

| Order | File | Scope |
|-------|------|-------|
| 1 | `spec-rs-fastpath-1-profile.md` | Instrument (pprof + GC trace). Run benchmark sweep. Identify top cost centres with profile-backed evidence. Cheap experiments (forwardCh depth, flush timing). Sets target numbers into this umbrella. |
| 2 | `spec-rs-fastpath-2-adjrib.md` | Move adj-rib-in off the forwarding hot path (side-subscriber, async storage). Relax `bgp-rs → bgp-adj-rib-in` from hard dependency to soft/optional. Replay on new peer works when adj-rib-in is present; forward works without it. |
| 3 | `spec-rs-fastpath-3-passthrough.md` | Zero-copy pass-through fast path. When no per-destination modification is required, the received buffer reaches each destination's `forward_pool` worker without a per-UPDATE rs-plugin RPC round-trip and without a copy. Copy-on-modify remains for AS-PATH rewrite, next-hop policy, etc. |

Children run in order — each depends on the previous. Each child produces its own learned summary on completion; this umbrella's learned summary lands last and folds in the results of all three.

## Measurement Evidence (2026-04-17, captured in spec-rs-fastpath-1-profile)

| Routes  | first-route | convergence | throughput (rps) | p99      |
|---------|-------------|-------------|------------------|----------|
| 10,000  | 2 ms        | 49 ms       | 204,081          | 48 ms    |
| 25,000  | ~0 (noise)  | 203 ms      | 123,152          | 201 ms   |
| 50,000  | 3 ms        | 311 ms      | 160,771          | 297 ms   |
| 75,000  | 330 ms      | 806 ms      | 93,052           | 751 ms   |
| 100,000 | 1,388 ms    | 2,044 ms    | 48,923           | 1,971 ms |

| DUT  | convergence | throughput  | p99      | first-route |
|------|-------------|-------------|----------|-------------|
| bird | 128 ms      | 781,250 rps | 73 ms    | 2 ms        |
| ze   | 3,082 ms    | 32,446 rps  | 3,020 ms | 2,474 ms    |

## Required Reading

### Architecture Docs

- [ ] `.claude/rules/design-principles.md`
  → Constraint: "Zero-copy, copy-on-modify." Source buffer allocated once from the Incoming Peer Pool on receive, shared read-only across destinations, copied only when a destination's egress filter rewrites attributes (into that peer's Outgoing Peer Pool). Released when all destinations have sent or copied. Any new forwarding path must refcount the source buffer and release it on the last send, not copy it.
  → Constraint: "No `make([]byte)` where pools exist" on wire-facing paths. Fast-path fwdItem must reference the existing pool buffer, not a new allocation. `make` is OK only in pool `New` funcs and in one-shot startup.
  → Constraint: "Pool strategy by goroutine shape." reactor `forward_pool` is a single-backing ring with three-index-capped sub-slices (one reactor consumer goroutine); must NOT be switched to `sync.Pool`. Per learned/275.
  → Constraint: "Lazy over eager." Consumer walks raw bytes via iterators; never wrap a raw buffer in a new struct with accessor methods on the hot path.
- [ ] `docs/architecture/core-design.md` — not re-read this session; RESEARCH pending when child 2 or 3 is selected.

### Learned Summaries (prior work in this area)

- [ ] `plan/learned/068-spec-remove-adjrib-integration.md`
- [ ] `plan/learned/275-spec-forward-pool.md`
- [ ] `plan/learned/269-rr-serial-forward.md`, `277-rr-ebgp-forward.md`, `289-rr-per-family-forward.md`
- [ ] `plan/learned/392-forward-congestion-phase1.md`, `394-forward-congestion-phase3.md`, `445-forward-congestion-phase4-5.md`, `457-forward-congestion-phase2.md`
- [ ] `plan/learned/424-forward-backpressure.md`, `519-fwd-auto-sizing.md`
- [ ] `plan/learned/417-perf.md`

### RFC Summaries

- [ ] `rfc/short/rfc4271.md` — BGP-4 UPDATE processing.
- [ ] RFC 7947 (BGP route-server) — summary not yet written. Child 1 creates the short summary via `/ze-rfc 7947`.

**Key insights** (umbrella-level, captured 2026-04-17):

- **Hot-path goroutine chain:** incoming UPDATE traverses engine deliveryLoop → DirectBridge → `rs.dispatchStructured` (`server.go:599`) → `workers.Dispatch` → per-source worker goroutine → `processForward` → `batchForwardUpdate` → `flushBatch` → `asyncForward` → `forwardCh` (buffered 16) → `forwardLoop` sender (N=4) → `updateRoute` RPC → `reactor.ForwardUpdate` → `buildModifiedPayload` → fwdItem channel → `forward_pool` TCP-writer. 6+ hops, with one `updateRoute` RPC marshal per route even when rs and reactor are in-process.
- **adj-rib-in is a hard dependency.** `internal/component/bgp/plugins/rs/register.go:17` declares `Dependencies: ["bgp-adj-rib-in"]`, forcing auto-load. adj-rib-in subscribes as a regular (synchronous) event subscriber, so BART insert latency sits on the hot-path delivery goroutine.
- **`waitForAPISync` is NOT the 1.4–2.5s first-route stall.** `reactor/peer.go:313` `ResetAPISync(expectedCount)` sets `apiSyncExpected` from `ProcessBindings` where `SendUpdate=true` (`peer_run.go:312-318`). The benchmark config has no `process { send update }` block (config parsed by `reactor/config.go:632 parseProcessBindingsFromTree`), so `apiSyncExpected=0` and the 500 ms sleep + 2 s `waitForAPISync` at `peer_initial_sync.go:171-178` does NOT fire. The 1.4–2.5 s stall is elsewhere.
- **Replay on new peer.** `rs/server_handlers.go:68-82` `handleStateUp` sets `Replaying=true`, spawns `replayForPeer`. `server_handlers.go:90-156` dispatches `adj-rib-in replay <peer> 0`, then a convergent delta loop (`replayConvergenceMax=10` iterations × `replayConvergenceDelay=20 ms`). On empty cache returns immediately; on filled cache sequences through the iterations.
- **Scaling cliff is at ~75k-100k routes, not present at 10k-50k.** Benchmark evidence is in this spec's Measurement Evidence table. Consistent with forwardCh (depth 16) saturation plus per-UPDATE `updateRoute` RPC overhead accumulating.
- **Config shape matters.** Peer config uses `connection { local { ip, port, connect }, remote { ip, port, accept } }` + `session { asn { local, remote }, router-id, family { ... } }` per `internal/component/bgp/schema/ze-bgp-conf.yang:125-141` + `peer-fields` grouping. `port` at peer-level is invalid; must be nested under `connection/local` or `connection/remote`. Benchmark's `test/perf/configs/ze.conf` was stale; fixed 2026-04-17 in this session.

## Current Behavior

**Source files read (overview — detail lives in each child):**
- [ ] `internal/component/bgp/plugins/rs/register.go:1-35` — declares `Dependencies: ["bgp-adj-rib-in"]` at line 17. Sets `RunEngine: RunRouteServer`.
  → Constraint: changing this dependency type (hard → soft/optional) is a plugin-registry change, not a local rs change. See child 2.
- [ ] `internal/component/bgp/plugins/rs/server.go` — `dispatchStructured(peerAddr, msg *bgptypes.RawMessage)` at line 599; stores `forwardCtx`, dispatches to per-source worker via `workers.Dispatch(workerKey{sourcePeer: peerAddr}, workItem{msgID})` (line 613). Flow-control via `workers.BackpressureDetected(key)` at line 621 pauses source.
  → Constraint: per-source worker key serialisation is the ordering guarantee; any fast path must preserve `sourcePeer → serial worker → ordered forwarding`.
- [ ] `internal/component/bgp/plugins/rs/server_handlers.go:68-156` — `handleStateUp` spawns per-peer lifecycle goroutine `replayForPeer(peerAddr, gen)`. Replay dispatches `adj-rib-in replay <peer> 0` then delta loop (max 10 × 20ms). Sets `Replaying=false` on completion.
  → Constraint: `Replaying=true` excludes peer from `selectForwardTargets`; during replay, new routes are not forwarded to that peer. Fast path must integrate with this gate.
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go`, `rib_commands.go` — not re-read this session. RESEARCH pending when child 2 is selected.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go:156` `ForwardUpdate` (RPC handler) — not re-read this session. RESEARCH pending when child 3 is selected.
- [ ] `internal/component/bgp/reactor/forward_pool.go`, `forward_build.go` — not re-read this session. RESEARCH pending when child 3 is selected.
- [ ] `internal/component/bgp/reactor/peer_initial_sync.go:171-178` — conditional 500 ms sleep + `waitForAPISync(2 * time.Second)` gate; gated on `apiSyncExpected > 0`. Verified NOT firing for benchmark config.
  → Constraint: this gate is necessary for peers that DO have `ProcessBindings{SendUpdate:true}`; must not be weakened. Benchmark is simply a case where no process bindings are configured.
- [ ] `internal/component/bgp/reactor/peer.go:310-364` — `ResetAPISync(expectedCount)`, `SignalAPIReady`, `waitForAPISync`. `apiSyncExpected` is an int32 derived from `ProcessBindings.SendUpdate` count.
- [ ] `internal/component/bgp/reactor/peer_run.go:300-348` — FSM Established transition: counts `ProcessBindings{SendUpdate:true}` (line 312-318), calls `ResetAPISync`, sets `sendingInitialRoutes=1`, notifies reactor, spawns `sendInitialRoutes` goroutine.
- [ ] `internal/component/bgp/reactor/config.go:632-674` — `parseProcessBindingsFromTree`; `ProcessBindings` come from config `process { <name> { send update refresh, receive ... } }` block. Empty in benchmark config.
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang:125-141` + `peer-fields` grouping (line 213+) — peer config shape: `connection { local{ip,port,connect}, remote{ip,port,accept}, md5, bfd }` + `session { asn{local,remote}, router-id, family{list} }`. `port` at peer level is INVALID.
- [ ] `internal/component/plugin/server/dispatch.go`, `pkg/ze/eventbus.go` — not re-read this session. RESEARCH pending when child 2 is selected (side-subscriber semantics).
- [ ] `test/perf/run.py`, `test/perf/configs/ze.conf` — harness validated; ze.conf fixed 2026-04-17 for current YANG schema.

**Behavior to preserve:**
- Per-source ordering of forwarded UPDATEs.
- Per-destination egress filter pipeline (redistribution, community, prefix, AS-PATH, NEXT_HOP, role/OTC, strip-private).
- Flow control via `workers.BackpressureDetected` + pause-source.
- Replay-on-new-peer when adj-rib-in is present.
- EOR ordering on session up.
- Bounded memory. No new unbounded channels or slices.
- All existing `.ci` tests pass unchanged.

**Behavior to change (deferred to children):** listed in each child spec.

## Data Flow

Each child spec traces its own changes. This umbrella records the end-state flow expected after all three children land:

### Entry Point

- Wire bytes on TCP arrive at a peer session.

### Transformation Path (target end-state)

1. Session `Run()` reads wire bytes into its session buffer.
2. Message boundaries framed; a `WireUpdate` reference is produced (zero-copy).
3. Reactor publishes the UPDATE as an event; subscribers:
   - **Hot path** (child 3): forward dispatcher identifies destination peers and hands the raw buffer to each destination's `forward_pool` worker. Zero-copy when no modification is required; one copy into Outgoing Peer Pool when modification is required.
   - **Side path** (child 2): adj-rib-in stores the route asynchronously without gating the hot path.
4. `forward_pool` worker writes the buffer to TCP.
5. Source buffer is released to its Incoming Peer Pool once all destinations have sent (or copied).

### Boundaries Crossed

| Boundary | How | Verified in |
|----------|-----|-------------|
| Session ↔ Reactor | WireUpdate reference + event | child 3 |
| Reactor ↔ adj-rib-in | DirectBridge side-subscriber | child 2 |
| Reactor ↔ forward_pool | fwdItem channel | child 3 |
| forward_pool ↔ TCP | direct write of pool buffer | child 3 |

### Integration Points

- `pkg/ze/eventbus.go` + `internal/component/plugin/server/dispatch.go` — DirectBridge is the zero-copy event hop; child 2 adds adj-rib-in as a side-subscriber here.
- `internal/component/bgp/reactor/reactor_api_forward.go` — canonical forwarding entry; child 3 either adds a fast-path entry beside it or refactors it into a mode switch.
- `internal/component/bgp/reactor/forward_pool.go` — per-peer TCP writer; child 3 accepts pass-through fwdItem referring to the source buffer plus refcount.
- `internal/component/bgp/plugins/adj_rib_in/rib.go` — async storage path; child 2 changes the subscription mode to side-subscriber.
- `test/perf/run.py` — benchmark harness; child 1 adds an optional PPROF gate.

### Architectural Verification

- [ ] No bypassed layers.
- [ ] No unintended coupling.
- [ ] No duplicated functionality.
- [ ] Zero-copy preserved where applicable.

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Two passive peers + bgp-rs config; sender streams UPDATEs | → | reactor fast-path dispatcher | `test/plugin/bgp-rs-fastpath.ci` (created by child 3) |
| Peer connects mid-stream; adj-rib-in stores asynchronously | → | adj-rib-in side-subscriber + replay | `test/plugin/bgp-rs-replay-mid-stream.ci` (created by child 2) |
| Profile evidence of hot-path cost | → | pprof/gctrace capture hook | `tmp/perf-run/pprof/` artefacts (produced by child 1) |

## Acceptance Criteria

Umbrella-level ACs are set **after** child 1 produces profiling evidence. Until then:

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | 100k IPv4 routes via `test/perf/run.py` | Throughput ≥ 400,000 rps (≈ 50 % of bird's 780k), first-route ≤ 50 ms. Floor: ≥ 200,000 rps and ≤ 50 ms first-route if the 400k target proves unreachable after child 3. Rationale: child 1 profile (2026-04-18) identified the rs → engine text-RPC round-trip as the dominant cost (60 % of allocations, 43 % of CPU consumed by GC). Removing the RPC (child 3) + moving adj-rib-in off the hot path (child 2) should raise throughput by ≥ 10× from today's 33k rps. See `plan/spec-rs-fastpath-1-profile.md` Design Insights. |
| AC-2 | 10k IPv4 routes | Throughput and latency unchanged or better vs 2026-04-17 baseline (204k rps, 2 ms first-route, 49 ms convergence). |
| AC-3 | Scaling sweep 10k/25k/50k/75k/100k | No superlinear cliff. Throughput stays within ±25 % across the sweep. |
| AC-4 | All existing `test/*` tests | Pass unchanged. |
| AC-5 | `make ze-verify-fast` + `make ze-race-reactor` | Clean after each child merges. |

Child-specific ACs live in each child spec.

## 🧪 TDD Test Plan

Umbrella-level tests are the end-to-end validation that the three children together achieve the goal. Per-change unit and functional tests live in child specs.

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `BenchmarkForwardPassThrough` | `internal/component/bgp/reactor/forward_pool_bench_test.go` | Per-UPDATE in-process hot path meets child-3 throughput target. | |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| — | Umbrella carries no numeric inputs. Children carry their own. | — | — | — |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-rs-fastpath-e2e` | `test/plugin/bgp-rs-fastpath-e2e.ci` | Two passive peers + bgp-rs; sender streams 1000 UPDATEs; receiver delivered all 1000 with p99 below child-3 threshold. Created by child 3. | |
| `bgp-rs-replay-mid-stream` | `test/plugin/bgp-rs-replay-mid-stream.ci` | Two peers + bgp-rs; sender streams; third peer joins; third peer receives full replay; ongoing stream continues uninterrupted. Created by child 2. | |
| `ze-perf 100k` | `test/perf/run.py` (harness) | 100k routes, 3 iterations, ze vs bird on same harness. Not a `.ci` test — bench harness owned by this umbrella. | |

### Future

- None. All tests ship with their respective children.

## Files to Modify

- Umbrella modifies no code directly. Children modify:
  - `internal/component/bgp/plugins/rs/*.go` (children 2 and 3)
  - `internal/component/bgp/plugins/adj_rib_in/*.go` (child 2)
  - `internal/component/bgp/reactor/reactor_*.go`, `forward_*.go` (child 3)
  - `test/perf/run.py` (child 1, optional pprof gate)

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] | (not expected) |
| CLI commands | [ ] | — |
| Editor autocomplete | [ ] | — |
| Functional test for new behavior | [ ] | `test/plugin/bgp-rs-fastpath.ci`, `test/plugin/bgp-rs-replay-mid-stream.ci` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` if child 2 ships rs-without-adj-rib-in |
| 2 | Config syntax changed? | [ ] | — |
| 3 | CLI command added/changed? | [ ] | — |
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` if child 3 bypasses `updateRoute` on hot path |
| 5 | Plugin added/changed? | [ ] | `docs/guide/plugins.md` — rs dependency relaxed (child 2) |
| 6 | Has a user guide page? | [ ] | `docs/guide/<topic>.md` route-server section |
| 7 | Wire format changed? | [ ] | — |
| 8 | Plugin SDK/protocol changed? | [ ] | `.claude/rules/plugin-design.md` if soft-dep concept is new (child 2) |
| 9 | RFC behavior implemented? | [ ] | — |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` if new benchmarks added |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` + `docs/performance.md` — new numbers after all children |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` — forwarding fast path |

## Files to Create

- `plan/spec-rs-fastpath-1-profile.md` (child 1)
- `plan/spec-rs-fastpath-2-adjrib.md` (child 2)
- `plan/spec-rs-fastpath-3-passthrough.md` (child 3)
- `plan/learned/NNN-rs-fastpath-0-umbrella.md` (on completion of all three)

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + current child |
| 2. Audit | Child's Files to Modify / Create |
| 3. Implement (TDD) | Child's Implementation Phases |
| 4. `/ze-review` gate | Child's Review Gate |
| 5. Full verification | `make ze-test`, `make ze-verify-fast`, `make ze-race-reactor`, `make ze-perf` where applicable |
| 6–9. Critical review + fixes | Child's Critical Review Checklist |
| 10. Deliverables review | Child's Deliverables Checklist |
| 11. Security review | Child's Security Review Checklist |
| 12. Re-verify | Re-run stage 5 |
| 13. Executive Summary | Per `rules/planning.md` |

### Implementation Phases

1. **Child 1 — profile + cheap experiments.** Evidence-driven. Produces numeric targets that get folded into this umbrella's AC-1 before children 2 and 3 start.
   - Tests: child-specific.
   - Files: child spec.
   - Verify: profile artefacts captured; umbrella AC-1 updated.
2. **Child 2 — adj-rib-in off hot path.** Async side-subscriber; relax hard dep.
   - Tests: child-specific.
   - Files: child spec.
   - Verify: hot-path delivery no longer blocks on BART insert; rs without adj-rib-in works.
3. **Child 3 — zero-copy pass-through.** Structural fast path.
   - Tests: child-specific.
   - Files: child spec.
   - Verify: bench meets AC-1, all existing tests pass, race detector clean.
4. **Umbrella close.** Re-run `test/perf/run.py --test ze bird`. Update `docs/performance.md`, `docs/comparison.md`. Write learned summary.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All three children complete; AC-1 numeric target set and met. |
| Correctness | Byte-identical wire output to pre-change for pass-through case. |
| Rule: no-layering | Old hot-path code removed or refactored after child 3; no "both old and new" residue. |
| Rule: integration-completeness | Umbrella + each child have a `.ci` / benchmark that exercises their AC. |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| All three children landed | `ls plan/learned/*rs-fastpath*.md` shows umbrella + 3 children |
| `docs/performance.md` updated with new numbers | `git log -- docs/performance.md` |
| Umbrella learned summary | `ls plan/learned/*rs-fastpath-0*.md` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | No change to wire parsing across any child. |
| Resource exhaustion | All new channels bounded; all new timers stopped/drained on Stop. |
| Concurrency | `make ze-race-reactor` clean after every child merges. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Child 1 profile does not name a bottleneck | Extend instrumentation in child 1; do not start child 2 |
| Child 1 experiment regresses an AC | Revert within child 1; do not carry forward |
| Child 2 breaks replay on new peer | Fix in child 2; umbrella AC-4 must stay green |
| Child 3 byte diff vs old path for pass-through case | Fix in child 3; AC-5 must stay green |
| 3 fix attempts fail in any child | STOP. Report all 3 approaches. Ask user. |

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

<!-- LIVE — umbrella collects insights that span children -->

## RFC Documentation

- RFC 4271 — MRAI advisory (no enforcement here).
- RFC 7947 — route-server semantics; child 1 creates the short summary.

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

- [ ] All three children complete and merged
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE) on each child
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] `make ze-verify-fast` passes
- [ ] `make ze-race-reactor` passes
- [ ] `test/perf/run.py --test ze bird` recorded; AC-1 met
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass — defer with user approval)

- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design

- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit
- [ ] Minimal coupling

### TDD

- [ ] Tests written (in each child)
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)

- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-rs-fastpath-0-umbrella.md`
- [ ] Summary included in commit
