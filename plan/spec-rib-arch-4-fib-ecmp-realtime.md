# Spec: rib-arch-4 -- Atomic N-Nexthop Best-Change Event Delivery (Realtime ECMP)

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-rib-arch-0-umbrella.md` - set context
4. `internal/component/bgp/plugins/rib/bestpath.go` - `SelectMultipath`
5. `internal/component/bgp/plugins/rib/events/events.go` - `BestChangeEntry` / `BestChangeBatch`

## Task

Equal-cost multipath selection exists: `SelectMultipath`
(`internal/component/bgp/plugins/rib/bestpath.go:157`) returns a primary plus siblings and
is called from the best-path pipeline (`internal/component/bgp/plugins/rib/rib_pipeline_best.go:98`).

GAP: the realtime best-change event delivers a **single** best next-hop, not the full
N-nexthop ECMP set atomically. Today `BestChangeEntry.BackupNextHop`
(`internal/component/bgp/plugins/rib/events/events.go:82`) is forwarded "as a DEDICATED
backup next-hop, never an ECMP sibling" -- so FIB consumers receiving best-change events
cannot install an ECMP set from the event alone. Deliver all N equal-cost next-hops
atomically in the best-change event so FIB consumers can program ECMP in realtime.

STALE-ish ANCHOR (verified 2026-07-08): the 2026-07-06 triage said `SelectMultipath` is
"show-path only"; it is now wired into the pipeline (`rib_pipeline_best.go:98`). Re-verify
at design time exactly what the realtime event carries versus what `show` computes, so this
does not re-implement an already-delivered path.

## Required Reading

### Architecture Docs
- [ ] `internal/component/bgp/plugins/rib/bestpath.go` - `SelectMultipath` (:157): primary + equal-cost siblings
  → Constraint: reuse this selection; do not recompute multipath in the event path.
- [ ] `internal/component/bgp/plugins/rib/events/events.go` - `BestChangeEntry` / `BestChangeBatch`
  → Constraint: the event's JSON contract is consumed by forked plugins; adding N next-hops must stay compatible or version explicitly.
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline_best.go` - `SelectMultipath` call site (:98)
  → Constraint: verify whether siblings already reach the event before adding them.

**Key insights:**
- Selection already yields siblings; the gap is carrying them through the realtime event atomically, not a new selection algorithm.

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; anchors verified 2026-07-08)
- [ ] `internal/component/bgp/plugins/rib/bestpath.go` - `SelectMultipath` (:157): `primary, siblings := ...` equal-cost multipath
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline_best.go` - calls `SelectMultipath(candidates, multipathMax, relaxASPath)` (:98)
- [ ] `internal/component/bgp/plugins/rib/events/events.go` - `BestChangeEntry` carries a single best plus `BackupNextHop` (:82) which is "a DEDICATED backup next-hop, never an ECMP sibling"

**Behavior to preserve:**
- Single-best consumers keep working; the `BestChangeBatch` JSON contract for forked plugins; `BackupNextHop` backup semantics (distinct from ECMP).

**Behavior to change:**
- The best-change event carries the full equal-cost next-hop set so FIB consumers can install ECMP atomically from one event.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Best-path recomputation in the RIB pipeline after a route change

### Transformation Path
1. `SelectMultipath` computes primary + equal-cost siblings (`bestpath.go:157`, called at `rib_pipeline_best.go:98`)
2. A `BestChangeEntry` is built for the prefix (`events.go`)
3. `BestChangeBatch` is emitted to consumers (FIB, redistribute, ...)
4. Proposed: the entry carries all N equal-cost next-hops so FIB installs ECMP without a second lookup

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| best-path → event | siblings from `SelectMultipath` populate the `BestChangeEntry` | [ ] |
| event → FIB | FIB consumer installs N next-hops atomically | [ ] |

### Integration Points
- `SelectMultipath` (`bestpath.go:157`) - the sibling source
- `BestChangeEntry` / `BestChangeBatch` (`events.go`) - the event payload
- FIB consumers (`internal/plugins/fib/*`) - the realtime installers

### Architectural Verification
- [ ] No bypassed layers (siblings flow best-path → event → FIB, no side channel)
- [ ] No unintended coupling (FIB stays generic; no per-family ECMP special-case)
- [ ] No duplicated functionality (reuse `SelectMultipath`; do not recompute in the event path)
- [ ] Registration over hardcoding - FIB consumers register; no per-consumer field in a core struct (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The realtime event does not already carry ECMP siblings | `BackupNextHop` is "never an ECMP sibling" (`events.go:82`) | Item already done; close as stale | read the event build path at design | unvalidated |
| A-2 | Adding N next-hops keeps the `BestChangeBatch` JSON contract compatible | `events.go` MarshalJSON is contract-stable | Needs explicit versioning | design review of MarshalJSON | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Non-atomic delivery causes transient FIB blackholes during an ECMP change | traffic loss on multipath churn | deliver the full set in one event; FIB replaces atomically |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Best-path change with N equal-cost paths | → | best-change event carries N next-hops; FIB installs ECMP | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Prefix with 3 equal-cost paths becomes best | one best-change event carries all 3 next-hops |
| AC-2 | ECMP set shrinks from 3 to 2 | one atomic event; FIB never transiently holds an incomplete set |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (define at design time) | `internal/component/bgp/plugins/rib/rib_bestchange_test.go` | best-change entry carries all equal-cost next-hops | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fib-ecmp-realtime` (new) | `test/plugin/fib-ecmp-realtime.ci` | multipath best-path change installs an ECMP FIB entry in realtime | |

## Files to Modify

- `internal/component/bgp/plugins/rib/events/events.go` - `BestChangeEntry` next-hop set
- `internal/component/bgp/plugins/rib/rib_pipeline_best.go` - populate siblings into the event
- FIB consumers (`internal/plugins/fib/*`) - install the N next-hops atomically

## Implementation Steps

1. **Phase: design** - re-verify what the event carries today (A-1); define the N-nexthop event shape.
2. **Phase: wiring** - failing test asserting N next-hops in the event.
3. **Phase: implement (TDD)** - carry siblings through the event; FIB installs ECMP atomically.
4. **Functional test** - `.ci` proving realtime ECMP.
5. **Full verification** - `make ze-verify`.
6. **Complete spec** - audit, learned summary, two-commit closure.

## Checklist

### Goal Gates (MUST pass)
- [ ] Best-change event carries N equal-cost next-hops atomically
- [ ] Wiring Test table complete (concrete test names, none deferred)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Notes
- Skeleton = captured intent, not a designed spec (`ai/rules/deferral-tracking.md`). Moves to `design` when picked up.
- Umbrella / siblings: `spec-rib-arch-0-umbrella.md`.
