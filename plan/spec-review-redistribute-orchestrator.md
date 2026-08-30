# Spec: Redistribute orchestrator producer/consumer symmetry and dead skip machinery (DESIGN-REVIEW finding 5)

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-unify-replay (ReplayID leaf concern, adjacent; closed, learned 1081) |
| Phase | - |
| Updated | 2026-07-06 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. ~~`DESIGN-REVIEW.md` finding 5 ("Protocol-agnostic core carrying protocol-specific shape")~~ (2026-07-22: ephemeral session artifact, never committed; the finding is restated inline in this spec)
4. `internal/component/bgp/plugins/redistribute_egress/redistribute.go`,
   `internal/core/redistevents/events.go`

## Task

Close the two concrete, uncovered defects in DESIGN-REVIEW finding 5. The redistribute
orchestrator treats producers and consumers asymmetrically and threads a startup snapshot
(`skipIDs`) through three function signatures that does no real work.

Two verified defects in scope:

1. **Producer/consumer registration asymmetry (latent silent drop).** The orchestrator
   enumerates producers exactly once at startup via `redistevents.Producers()`
   (`redistribute.go`, call at `:142`) and then blocks on `<-ctx.Done()`
   (`:136`), so the subscription set is fixed. But consumers are read live on every event
   via `configredist.ConsumerNames()` (`redistribute.go`). A producer that registers its
   ProtocolID after the orchestrator's `run` has started is never subscribed to and its
   route-change events are silently dropped, with no log and no metric.

2. **`skipIDs` is dead machinery.** `skipIDs` is computed at startup
   (`redistribute.go`), threaded through `subscribe`, the subscription closure
   (`:152`), and `handleBatch`, but its only use is a debug log
   (`redistribute.go`); it skips nothing. The real skip (do not dispatch a batch back
   to the consumer of its own source protocol) is a live per-consumer recompute at
   `redistribute.go` (`redistevents.ProtocolIDOf(cname) == b.Protocol`) that never
   consults `skipIDs`. So the parameter is both dead-for-skipping and, because it is a startup
   snapshot while consumers are read live, potentially stale for its one logging use.

Goal: (1) remove the silent-drop failure mode so a late-registered producer is either handled
or loudly surfaced (metric + warn), making producer handling as observable as consumer
handling; (2) delete `skipIDs` from the `run` -> `subscribe` -> closure -> `handleBatch`
chain, deriving the heads-up debug log (if kept) from the live skip, with byte-identical
dispatch behavior.

**Explicitly out of scope (referenced, not duplicated):**
- Removing `ReplayID`/`ReplayRequest` (BGP-peer semantics) from the `redistevents` leaf
  payload (`events.go,161-168`): adjacent to `spec-unify-replay.md`, which converges the
  replay-request vocabulary. This spec cross-references it and does not redesign replay. See
  Known Limitations for the framing (the leaf stays type-clean via an opaque value token; the
  coupling is semantic, not structural).
- The lossy route-change event bridge and per-entry OriginAS: owned by `spec-unify-route-events.md`.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `docs/architecture/core-design.md` (redistribute orchestrator section) - the
  producer -> EventBus -> orchestrator -> consumer path
  → Decision: the orchestrator is the single EventBus subscriber turning producer route-change events into consumer dispatches.
  → Constraint: producers and consumers each build LOCAL typed handles; no handle pointer crosses a plugin boundary.
- [ ] `ai/rules/plugins.md` - registration-over-hardcoding for producers/consumers
  → Constraint: producer discovery must go through the registry; do not hardcode a producer list.
- [ ] `ai/rules/architecture.md` - required for the Data Flow section
  → Constraint: no silent drop of a registered producer; a gap must be observable (log + metric).

**Key insights:** producers are enumerated once and subscribed; consumers are looked up live.
The `skipIDs` snapshot duplicates (staler) information the live path already computes.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/bgp/plugins/redistribute_egress/redistribute.go` - `run` snapshots
  `skipIDs` (line 127) and calls `subscribe`; `subscribe` enumerates `Producers()` once (142)
  and subscribes each (150-153); `handleBatch` reads `ConsumerNames()` live (204); the real
  self-consumer skip is at 215; `skipIDs` is used only at the debug log 186-188.
- [ ] `internal/core/redistevents/events.go` - `Producers()`/`ProtocolIDOf`/`ProtocolName`
  registry surface; `RouteChangeBatch` payload; `ReplayID`/`ReplayRequest` (BGP-peer semantics,
  out of scope here) at 64-68, 161-168.

**Behavior to preserve:** (unless user explicitly said to change)
- Actual dispatch outcome unchanged: a batch is never dispatched back to the consumer of its
  own source protocol; all other consumers still receive it (BGP best-path into OSPF,
  etc.), subject to the evaluator `Accept` check.
- Metric names and semantics for `filteredProtocolTotal`, `filteredRuleTotal`,
  `eventsReceived`, `announcements`, `withdrawals`, `replayTotal` unchanged.
- Replay path (nonzero `ReplayID`) unchanged (owned by `spec-unify-replay.md`).
- All existing redistribution `.ci` tests (ospf/isis redist) stay green.

**Behavior to change:** (only if user explicitly requested)
- A producer that registers after orchestrator startup is no longer silently dropped: it is
  either subscribed or surfaced via a warn + a new metric.
- `skipIDs` is removed from the function-signature chain; the debug log (if retained) is
  derived live, not from a startup snapshot.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A protocol producer (L2TP, connected, static, OSPF, ...) emits a
  `redistevents.RouteChangeBatch` on the EventBus under its own protocol namespace.
- Format at entry: a pooled value-typed `*RouteChangeBatch` delivered as `any` by the bus.

### Transformation Path
1. At startup `run` computes `skipIDs := consumerProtocolIDs()` (`redistribute.go`).
2. `subscribe` enumerates `redistevents.Producers()` ONCE and registers one local
   handle + subscription per producer; `run` then blocks on ctx.
3. Per event, `handleBatch` validates the batch, branches replay vs incremental, then
   reads consumers live via `configredist.ConsumerNames()`.
4. For each consumer it applies the real self-consumer skip live and the evaluator
   `Accept` check, then dispatches each entry.
5. `skipIDs` is consulted only to emit a debug log; it drives no dispatch.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Producer ↔ orchestrator | per-protocol EventBus subscription, fixed at startup | [ ] |
| Consumer registry ↔ orchestrator | `ConsumerNames`/`LookupConsumer` read live per event | [ ] |
| Redistevents registry ↔ orchestrator | `Producers`/`ProtocolIDOf`/`ProtocolName` lookups | [ ] |

### Integration Points
- `redistevents.Producers()` and the `RegisterProtocol` registry - producer discovery.
- `redistributeegress.run` / `subscribe` / `handleBatch` - the asymmetry and `skipIDs` chain.
- `configredist.ConsumerNames` / `LookupConsumer` - the live consumer side.
- Telemetry registry - for a new "producer without subscription" observability metric.

### Architectural Verification
- [ ] No bypassed layers (producer discovery stays via the registry, not a hardcoded list)
- [ ] No unintended coupling (no new cross-plugin handle pointer)
- [ ] No duplicated functionality (`skipIDs` snapshot removed; single live skip)
- [ ] Zero-copy preserved where applicable (payload handling unchanged)
- [ ] Registration over hardcoding — late producers are handled via the registry seam or
  loudly surfaced; no hardcoded producer list is introduced (`ai/rules/plugins.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | All current producers register their ProtocolID at `init()`, before the orchestrator `run` starts | producer packages register via `RegisterProtocol` at init | The silent drop is an active bug today, not latent; fix priority rises | grep producer registration call sites; startup-order trace | unvalidated |
| A-2 | The real skip at `:215` fully subsumes any correctness `skipIDs` was intended to provide | `skipIDs` is only read at the debug log `:186-188` | Removing `skipIDs` changes dispatch | Before/after dispatch parity test | unvalidated |
| A-3 | redistevents exposes (or can cheaply expose) a registration seam to subscribe late producers, or init-ordering can be enforced | `redistevents` registry (`Producers`, `RegisterProtocol`) | Late-producer handling needs a larger registry change | Read `redistevents/registry.go`; design decision | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Removing `skipIDs` subtly changes the debug-log condition operators rely on | log diff in redistribute scenarios | Derive the same condition live from the `:215` skip; keep message text |
| R-2 | Reactive late-producer subscription introduces a subscribe race | `-race` failures on subscribe/teardown | Prefer enforce-init-ordering + loud detection over dynamic resubscribe if the race cost is high |
| R-3 | New metric noise if a producer legitimately has no routes | metric climbs with no problem | Count only producers with no subscription, not producers with zero routes |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a source protocol that is also a consumer emits a batch | → | live self-consumer skip fires; no `skipIDs` param | `test/plugin/redistribute-consumer-skip.ci` |
| a producer ProtocolID registered after `run` starts emits a batch | → | subscribed-or-surfaced, never silently dropped | `TestLateProducerNotSilentlyDropped` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A producer registers its ProtocolID after the orchestrator `run` has started, then emits a batch | The batch is either dispatched (producer subscribed) or the missing subscription is surfaced by a warn + a dedicated metric; it is never silently dropped |
| AC-2 | Normal run with `skipIDs` removed from `run`/`subscribe`/closure/`handleBatch` | Dispatch outcome byte-identical to before; grep shows no `skipIDs` parameter threaded for logging only |
| AC-3 | A source protocol that is also a consumer emits a batch | That consumer is skipped, all others still receive it, `filteredProtocolTotal` increments exactly as before |
| AC-4 | The debug log "source protocol has a consumer" case | If retained, the log is derived from the live skip, not a startup snapshot (no staleness when a consumer registers after startup) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | configures BGP-to-OSPF redistribution where the source is also a consumer | producer -> orchestrator -> live skip -> other consumers | `test/plugin/redistribute-consumer-skip.ci` |
| 2 | (internal) a producer registers late | registry -> orchestrator subscribe/detect | `TestLateProducerNotSilentlyDropped` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLateProducerNotSilentlyDropped` | `internal/component/bgp/plugins/redistribute_egress/redistribute_test.go` | late producer subscribed or surfaced, not silently dropped | |
| `TestSkipIDsRemovedDispatchParity` | `internal/component/bgp/plugins/redistribute_egress/redistribute_test.go` | dispatch identical after removing `skipIDs` | |
| `TestSelfConsumerSkipLive` | `internal/component/bgp/plugins/redistribute_egress/redistribute_test.go` | self-consumer skip + `filteredProtocolTotal` unchanged | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ProtocolID | 1 .. 2^16-1 | 65535 | 0 (ProtocolUnspecified, rejected) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `redistribute-consumer-skip` | `test/plugin/redistribute-consumer-skip.ci` | source-is-also-consumer skipped, others still get routes | |

### Interop Tests (MANDATORY for protocol features)
Not applicable: this is an internal orchestrator refactor with no peer-facing wire change.
Existing OSPF/IS-IS redistribution `.ci` scenarios are the regression gate.

### Future (if deferring any tests)
- A chaos test injecting late producer registration under load is a strong follow-on.

## Files to Modify
- `internal/component/bgp/plugins/redistribute_egress/redistribute.go` - remove `skipIDs` from
  `run`/`subscribe`/closure/`handleBatch`; derive the debug log live; handle-or-surface late
  producers; add the "producer without subscription" metric.
- `internal/core/redistevents/registry.go` - (conditional) expose a registration seam or a
  post-startup detection hook if the design picks reactive subscription over enforced ordering.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| Prometheus counter (producer without subscription) | [ ] | redistribute telemetry + `docs/plugin-development/metrics.md` |
| Functional test for skip behavior | [ ] | `test/plugin/redistribute-consumer-skip.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` redistribute orchestrator section |
| 14 | Prometheus counters added/changed? | [ ] | `docs/plugin-development/metrics.md` (new producer-gap metric) |
| 16 | Changed source referenced by doc anchors? | [ ] | grep `docs/` for `source: .../redistribute_egress/redistribute.go` |

## Files to Create
- `test/plugin/redistribute-consumer-skip.ci` - proves the self-consumer skip and that other
  consumers still receive the routes.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate |
| 6. Full verification | `./le verify current mode full` |

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** — add `TestLateProducerNotSilentlyDropped` asserting a
   late producer is not silently dropped (fails now); add the producer-gap metric skeleton.
   - Files: `redistribute.go`, `redistribute_test.go`
   - Verify: test fails because the current snapshot silently drops the late producer.
2. **Phase: Producer symmetry** — subscribe late producers or surface the gap (warn + metric);
   pick the mechanism (Key Design Decisions) after validating A-3.
   - Tests: `TestLateProducerNotSilentlyDropped` passes.
3. **Phase: Remove skipIDs** — delete `skipIDs` from all four sites; derive the debug log live
   from the `:215` skip; prove dispatch parity.
   - Tests: `TestSkipIDsRemovedDispatchParity`, `TestSelfConsumerSkipLive`, `redistribute-consumer-skip.ci`.
4. **Functional tests** → `redistribute-consumer-skip.ci`.
5. **Full verification** → `./le verify current mode full`.
6. **Complete spec** → learned summary `plan/learned/NNN-redistribute-orchestrator.md`; two commits.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Dispatch outcome byte-identical; metric semantics preserved |
| Data flow | No silent producer drop; gap is observable |
| Registration over hardcoding | Producer discovery via registry; no hardcoded producer list |
| Rule: no-layering | `skipIDs` fully removed, not left as a dormant unused parameter |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| skipIDs removed | grep `skipIDs` in `redistribute.go` returns nothing (or only a live-derived local) |
| Late producer handled | `TestLateProducerNotSilentlyDropped` passes; metric present |
| Dispatch parity | `redistribute-consumer-skip.ci` + `TestSkipIDsRemovedDispatchParity` pass |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | Reactive subscription (if chosen) cannot be driven to unbounded subscriptions by a misbehaving producer |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Dispatch differs after skipIDs removal | Re-derive the live condition (Phase 3, A-2) |
| Subscribe race on late producer | Prefer enforced init-ordering + detection (Phase 2, R-2) |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
<!-- LIVE — write IMMEDIATELY when you learn something -->

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Handle-or-surface late producers | Silently keep the startup snapshot | Silent drop is the defect; observability is the minimum bar |
| Enforce init-ordering + loud detection vs reactive resubscribe | Dynamic resubscribe on every RegisterProtocol | Pick after A-3; enforcement avoids subscribe races if reactive is costly |
| Remove skipIDs entirely | Make skipIDs drive the real skip | The live `:215` check already exists and stays fresh; the snapshot is redundant and can go stale |

## Known Limitations
- `ReplayID`/`ReplayRequest` carrying BGP-peer semantics in the `redistevents` leaf
  (`events.go,161-168`) is NOT resolved here. It is a deliberate, documented trade-off:
  the token is opaque and value-typed and the orchestrator alone holds the token->peer map, so
  the leaf stays type-clean; the coupling is semantic. Vocabulary convergence is owned by
  `spec-unify-replay.md`; a fuller redesign (orchestrator-side correlation without a payload
  field) is a separate, larger effort not attempted here.

## Implementation Summary

### What Was Implemented
- [filled during /implement]

### Bugs Found/Fixed
- [silent producer drop, if A-1 shows it is active rather than latent]

### Documentation Updates
- [core-design.md redistribute section + metrics doc, or "None" with grep evidence]

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `./le verify worktree` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (reuse the existing registry/live-skip)
- [ ] No speculative features (two named defects only)
- [ ] Explicit > implicit behavior (observable producer gap)
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (ProtocolID)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests N/A (internal change; existing redistribution interop is the regression gate)

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Write learned summary to `plan/learned/NNN-redistribute-orchestrator.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-review-redistribute-orchestrator.md`
