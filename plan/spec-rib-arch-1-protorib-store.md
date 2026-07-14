# Spec: rib-arch-1 -- Central Per-Protocol RIB Store vs Event-Bus Delta Model (Design Question)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-07-14 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-rib-arch-0-umbrella.md` - set context
4. `internal/component/bgp/plugins/rib/events/events.go` - `BestChangeBatch` delta event
5. `plan/learned/634-bgp-redistribute.md`, `plan/learned/685-redist-producers.md`

## Task

DESIGN QUESTION, not a mechanical task. Today per-protocol route consumption flows
through an **event-bus delta model**: the RIB emits `BestChangeBatch` events
(`internal/component/bgp/plugins/rib/events/events.go:90`, one batch per (protocol,
family)) and `EmitBestChange` publishes them on the event bus
(`internal/component/bgp/redistribute/producer.go:40`); consumers such as the
bgp-redistribute plugin (`internal/component/bgp/plugins/redistribute_egress`,
`redistribute_ingress`) subscribe and rebuild their own view from the delta stream.

The alternative is an **engine-owned central per-protocol store** that consumers query
directly instead of maintaining a private replica from deltas. Decide store-vs-delta;
do not build a store speculatively. The trigger recorded at triage: move only when a
**second** consumer beyond bgp-redistribute makes the delta model painful (each new
consumer re-implements delta accumulation and full-table replay handling).

Scope of this spec is the DECISION plus, if store is chosen, the migration. Capture the
decision and its rationale in this spec; if the answer is "keep deltas", close as a
recorded design decision (learned summary), not code.

### Re-verification (2026-07-14): the second-consumer trigger has FIRED

The premise above -- bgp-redistribute is "the sole current delta consumer" and the move
happens only at a hypothetical *second* consumer -- is now false against live code. Two
further production subscribers of the `BestChangeBatch` delta already exist, each
re-implementing per-prefix accumulation + full-table-replay handling:
- **flowexport**: `internal/plugins/flowexport/enrichbgp.go:107`
  (`ribevents.BestChange.Subscribe(eb, b.applyBatch)`), wired at
  `internal/plugins/flowexport/register.go:270`.
- **forked sysrib** (conditional, forked deployments only): the `else` branch at
  `internal/component/sysrib/sysrib.go:889` (`ribevents.BestChange.Subscribe`); the
  in-process path uses `locrib` `OnChange` instead.

Assumption A-1 is therefore INVALID, not merely unvalidated, and this spec is no longer a
speculative "revisit at second consumer" item: the recorded trigger has fired, and
store-vs-delta can be decided against real duplication evidence.

Anchor precision (2026-07-14): `EmitBestChange` (`producer.go:40`) does NOT itself publish
on the event bus. It is a direct in-process call from `rib_bestchange.go:1221`
(`bgpredist.EmitBestChange(eb, batch)`) that converts each entry to a generic
`redistevents.RouteChangeBatch`. The RIB's own bus publish of the `BestChangeBatch` delta
is `ribevents.BestChange.Emit` at `rib_bestchange.go:1218`.

## Required Reading

### Architecture Docs
- [ ] `internal/component/bgp/plugins/rib/events/events.go` - the delta event contract
  → Constraint: any central store must preserve the replay/incremental distinction (`IsReplay`, `ReplayID`) and the `FromLocRIB` arbitration hint.
- [ ] `plan/learned/634-bgp-redistribute.md` - why redistribute consumes deltas
  → Decision: the second-consumer trigger comes from here; verify it still holds.
- [ ] `plan/learned/685-redist-producers.md` - producer wiring
  → Constraint: verify current behaviour against this before designing.

**Key insights:**
- Every delta consumer today re-implements accumulation + full-table replay. A store would centralise that; the cost is engine-owned per-protocol memory and a query API.

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; anchors verified 2026-07-08)
- [ ] `internal/component/bgp/plugins/rib/events/events.go` - `BestChangeBatch` (:90): per-(protocol,family) delta batch; `IsReplay()` (:114) distinguishes full-table replay from incremental; `FromLocRIB` (:109) marks Loc-RIB-arbitrated batches
- [ ] `internal/component/bgp/redistribute/producer.go` - `EmitBestChange` (:40): publishes a batch on the event bus
- [ ] `internal/component/bgp/plugins/redistribute_egress`, `redistribute_ingress` - the sole current delta consumer

**Behavior to preserve:**
- The `BestChangeBatch` JSON wire contract (external forked-plugin consumers decode it); the replay marker semantics; per-protocol arbitration by admin distance for non-Loc-RIB batches.

**Behavior to change:**
- Only if "store" is chosen: consumers query a central store instead of rebuilding from deltas. Otherwise: none (record the decision).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Best-path change inside the RIB pipeline produces a `BestChangeBatch`

### Transformation Path
1. RIB best-path selection produces per-prefix best changes
2. `EmitBestChange` publishes a `BestChangeBatch` on the event bus (`producer.go:40`)
3. Each consumer subscribes and accumulates its own per-protocol view from the delta stream
4. Proposed: an engine-owned store accumulates once; consumers query it instead

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| RIB → consumer | event-bus `BestChangeBatch` delta (today) vs central-store query (proposed) | [ ] |
| in-process ↔ forked plugin | JSON contract of `BestChangeBatch` (must survive either model) | [ ] |

### Integration Points
- `EmitBestChange` (`producer.go:40`) - the publish point
- `BestChangeBatch` (`events.go:90`) - the delta payload / store input

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (a store must not leak per-protocol internals to consumers)
- [ ] No duplicated functionality (a store replaces per-consumer accumulation, not adds to it -- see `ai/rules/no-layering.md`)
- [ ] Registration over hardcoding - consumers/producers register; no per-protocol switch in a core package (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | bgp-redistribute is still the only production delta consumer | 2026-07-08 split verification | The second-consumer trigger already fired; re-prioritise | grep for `OnChange`/best-change subscribers at design time | **INVALID (2026-07-14)**: flowexport (`enrichbgp.go:107`) + forked sysrib (`sysrib.go:889`) already consume the delta; trigger has fired |
| A-2 | The delta contract can be preserved by a store without a wire change | `events.go` MarshalJSON keeps a stable JSON tag | Store needs its own contract; larger change | design review | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Building a store speculatively (premature abstraction) | only one consumer at design time | Keep deltas; record the decision; revisit at second consumer |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Best-path change with N consumers | → | central store queried once vs N delta rebuilds (if store chosen) | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Design decision recorded | Store-vs-delta chosen with rationale in the learned summary |
| AC-2 | (if store chosen) second consumer added | queries the store; no private delta accumulation |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (define at design time) | (define at design time) | store query returns current best set (if store chosen) | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A at skeleton stage - internal RIB store architecture; no new user-facing behaviour. Existing redistribute best-change `.ci` suites regression-guard. Define a `.ci` at design if a user-facing surface emerges | per design | redistribute still works end-to-end | |

## Files to Modify

- `internal/component/bgp/plugins/rib/events/events.go` - delta contract (unchanged if deltas kept; store input if store chosen)
- `internal/component/bgp/redistribute/producer.go` - publish point / store write

## Implementation Steps

1. **Phase: design** - re-verify the second-consumer trigger (A-1); decide store vs delta; record rationale.
2. **Phase: implement (only if store chosen, TDD)** - store type, query API, migrate the consumer, delete per-consumer accumulation (`ai/rules/no-layering.md`).
3. **Full verification** - `make ze-verify`.
4. **Complete spec** - learned summary (records the decision either way), two-commit closure.

## Checklist

### Goal Gates (MUST pass)
- [ ] Store-vs-delta decision recorded with rationale
- [ ] Wiring Test table complete if code is written (concrete test names)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Decision (2026-07-14): Keep the event-bus delta model

**Decision:** Keep the `BestChangeBatch` event-bus delta model. Do NOT build an
engine-owned central per-protocol RIB store. No production code changes; this spec
closes as a recorded design decision (AC-1), per the Task section's own guidance
("if the answer is 'keep deltas', close as a recorded design decision, not code").

**Trigger status — A-1 is BROKEN.** The second-consumer trigger recorded at triage
HAS fired. Assumption A-1 ("bgp-redistribute is still the only production delta
consumer") is false as of 2026-07-14. Three production consumers of the RIB
best-change stream now exist (verified by reading each producer, not its caller):

| Consumer | Verified file:line | What it does with the delta |
|----------|--------------------|------------------------------|
| bgp-redistribute (`redistribute_egress`) | subscribes the transformed `RouteChange` via `EmitBestChange` (`internal/component/bgp/redistribute/producer.go:40`, `convertBestChange` :61) | transform-and-forward; keeps no full materialized table |
| flowexport `bgpEnrichBuilder` | `internal/plugins/flowexport/enrichbgp.go:63` `applyBatch`, `:107` `Subscribe` | accumulates its own `table map[netip.Prefix]enrich.ASEntry` and rebuilds an immutable prefix→AS radix tree, debounced 5s (`:86` `rebuild`) |
| sysrib | `internal/component/sysrib/sysrib.go:852` (`loc.OnChange`), `:889` (Stream A fallback) | maintains its own arbitrated route table; re-emits arbitrated Stream B to FIB installers |

**Why keep deltas despite the trigger firing:**
1. A central per-protocol store would NOT remove the expensive work. flowexport's
   cost is the O(N) radix rebuild (`enrichbgp.go:86-100`), not the cheap incremental
   map; a store snapshot would still require that rebuild. The store saves only the
   cheap `table` accumulation.
2. The store's payload would be identical to `BestChangeEntry` — it must carry the
   BGP-specific `OriginAS`/`ASPath`/`NextHop` that flowexport reads
   (`enrichbgp.go:75-79`). It is not a cleaner abstraction, just a shared cache of the
   same delta payload.
3. The engine-owned central arbitrated store already exists: the Loc-RIB
   (`internal/core/rib/locrib`), consumed by sysrib and re-emitted as arbitrated
   Stream B for FIB installers. Consumers wanting a protocol-agnostic materialized
   best-path view query that path. Consumers needing BGP-specific attributes
   (flowexport) legitimately consume the BGP-RIB delta stream because the arbitrated
   Loc-RIB does not carry AS_PATH / communities.
4. The two BGP-RIB delta consumers have DIFFERENT query shapes (prefix→AS radix vs
   transform-and-forward). One store fits neither cleanly — premature abstraction
   (R-1 confirmed).
5. The delta model preserves the `BestChangeBatch` JSON wire contract for forked
   plugins; a store query API would need its own contract.

**Revised revisit trigger:** build a central per-protocol store only when ≥2 consumers
need the SAME materialized query shape over BGP-specific attributes (e.g. two consumers
both wanting "current best-path set with AS_PATH, point-queryable"). Today's consumers
do not. flowexport's own optimisation (incremental/persistent-trie rebuild,
`plan/learned/819`) is per-consumer, reinforcing that the pain is consumer-specific,
not delta-model-wide.

## Implementation Summary

- No production code changed. This is a recorded design decision (see above).
- AC-1 satisfied: store-vs-delta decided (keep deltas) with rationale, captured here
  and in the learned summary `plan/learned/1120-rib-arch-1-store-vs-delta.md`.
- AC-2 not applicable: store not chosen, so no second-consumer store migration.
- Deviation from skeleton assumption: A-1 moved `unvalidated → broken` (the
  second-consumer trigger fired via flowexport); the decision explains why the store is
  still not warranted.

## Review Gate

No production code diff (decision-only spec). Nothing to review.
Findings: 0 BLOCKER, 0 ISSUE, 0 NOTE.

## Pre-Commit Verification

Re-verified from scratch on 2026-07-14:

| Item | Evidence |
|------|----------|
| AC-1 decision recorded | Decision section above + `plan/learned/1120-rib-arch-1-store-vs-delta.md` |
| No code changed | `git diff` touches only `plan/` (spec + learned + counter) |
| A-1 resolved | `broken` — flowexport is a second production delta consumer (`internal/plugins/flowexport/enrichbgp.go:63,107`), read directly |
| A-2 resolved | not exercised — store not built; `BestChangeBatch` JSON contract untouched |
| Producers read (not callers) | `producer.go:40` `EmitBestChange`, `enrichbgp.go:63` `applyBatch`, `events.go:54` `BestChangeEntry` all read this session |

## Notes
- Skeleton = captured intent, not a designed spec (`ai/rules/deferral-tracking.md`). Moves to `design` when picked up.
- Umbrella / siblings: `spec-rib-arch-0-umbrella.md`.
