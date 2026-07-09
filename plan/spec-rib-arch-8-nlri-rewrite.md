# Spec: rib-arch-8 -- General NLRI-Byte Rewrite via ModAccumulator

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
4. `internal/component/bgp/filterapi/filterapi.go` - `ModAccumulator`
5. `internal/component/bgp/reactor/filter_delta.go` - delta-to-ops translation

## Task

The egress filter `ModAccumulator` (`internal/component/bgp/filterapi/filterapi.go:98`)
accumulates two kinds of per-peer modification: **attribute** ops (`ops []AttrOp`, via
`Op`/`OpCopy`, applied by `textDeltaToModOps`, `internal/component/bgp/reactor/filter_delta.go:202`)
and **announce→withdraw** conversion (`withdraw bool`, via `SetWithdraw()`,
`filterapi.go:151`).

GAP: there is no general **NLRI-byte rewrite** capability -- rewriting the NLRI prefixes
themselves (not just attributes, and not just the whole-route withdraw). Add a rewrite
field/method to `ModAccumulator` and the forward-path application so a filter can rewrite
the announced NLRI bytes for a destination peer (e.g. prefix translation / aggregation-like
substitution), symmetric to how attribute ops and withdraw are applied today.

## Required Reading

### Architecture Docs
- [ ] `internal/component/bgp/filterapi/filterapi.go` - `ModAccumulator` shape and application contract
  → Constraint: NLRI rewrite must follow the existing accumulator discipline (fresh per peer, not retained beyond the call, inline-buffer reuse where possible).
- [ ] `internal/component/bgp/reactor/filter_delta.go` - `textDeltaToModOps` (:202): how attribute deltas become ops
  → Constraint: NLRI rewrite is a new op kind or a sibling to `SetWithdraw`; wire it through the same forward application path.

**Key insights:**
- Attribute mods and announce→withdraw already flow through the accumulator; NLRI rewrite is the missing third modification kind, applied on the same egress forward path.

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; anchors verified 2026-07-08)
- [ ] `internal/component/bgp/filterapi/filterapi.go` - `ModAccumulator` (:98): `ops []AttrOp` + `withdraw bool`; `Op`/`OpCopy` (:119/:126) accumulate attribute ops; `SetWithdraw()` (:151) converts announce to withdrawal; no NLRI-rewrite field
- [ ] `internal/component/bgp/reactor/filter_delta.go` - `textDeltaToModOps` (:202): translates a text attribute delta into accumulator ops

**Behavior to preserve:**
- Attribute op and withdraw semantics; accumulator lifetime rules (fresh per peer, `MUST NOT` retain the pointer); the inline-buffer allocation-avoidance pattern.

**Behavior to change:**
- Add a general NLRI-byte rewrite modification the forward path applies when producing the per-peer UPDATE.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- An egress filter runs for a (route, destination peer) pair and requests an NLRI rewrite

### Transformation Path
1. Egress filter decides a rewrite and records it on the `ModAccumulator` (new field/method)
2. The forward path builds the per-peer UPDATE, applying attribute ops and withdraw today (`filter_delta.go`, `reactor_api_forward.go`)
3. Proposed: the same application step substitutes the rewritten NLRI bytes
4. The peer receives an UPDATE with the rewritten NLRI

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| filter → accumulator | new NLRI-rewrite field/method on `ModAccumulator` | [ ] |
| accumulator → wire | forward path substitutes NLRI bytes when emitting the per-peer UPDATE | [ ] |

### Integration Points
- `ModAccumulator` (`filterapi.go:98`) - gains the NLRI-rewrite modification
- forward application path (`filter_delta.go`, `reactor_api_forward.go`) - applies the rewrite alongside ops/withdraw

### Architectural Verification
- [ ] No bypassed layers (rewrite flows filter → accumulator → forward, like ops/withdraw)
- [ ] No unintended coupling (filters stay unaware of wire framing details)
- [ ] No duplicated functionality (reuse the accumulator + forward application, not a parallel rewrite path)
- [ ] Registration over hardcoding - filters register; no per-filter field in a core struct (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The forward path builds the per-peer UPDATE somewhere NLRI bytes can be substituted | attribute ops + withdraw already applied there | Rewrite needs a deeper encoder change | read the per-peer UPDATE build at design | unvalidated |
| A-2 | Rewriting NLRI does not break path-id / add-path or dedup invariants | inject/withdraw build NLRI with pathID=0 (`rib_bestchange.go:1180`) | Constrain rewrite to safe cases; reject others | design review of add-path interaction | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | NLRI rewrite desyncs adj-rib-out state from what was actually sent | withdraw of the wrong (original) prefix later | track the rewritten NLRI in adj-rib-out, not the original |
| R-2 | Rewrite enables a route the filter cannot faithfully encode | encode error at forward time | exact-or-reject: reject the rewrite if unencodable (`ai/rules/exact-or-reject.md`) |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Egress filter requests an NLRI rewrite for a peer | → | forward path emits an UPDATE with rewritten NLRI | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Filter rewrites NLRI for one destination peer | that peer receives the rewritten prefix; other peers unaffected |
| AC-2 | Rewritten route later withdrawn | the withdrawal references the rewritten NLRI (adj-rib-out consistent) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (define at design time) | `internal/component/bgp/filterapi/filterapi_test.go` | NLRI-rewrite accumulation and per-peer isolation | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `nlri-rewrite` (new) | `test/plugin/nlri-rewrite.ci` | a filter rewrites the announced prefix to one peer; the peer receives the rewritten NLRI | |

## Files to Modify

- `internal/component/bgp/filterapi/filterapi.go` - `ModAccumulator` NLRI-rewrite field/method
- `internal/component/bgp/reactor/filter_delta.go` - accumulate the rewrite from a filter delta
- `internal/component/bgp/reactor/reactor_api_forward.go` - apply the rewrite when building the per-peer UPDATE

## Implementation Steps

1. **Phase: design** - locate the per-peer UPDATE build (A-1); define the rewrite representation and add-path interaction (A-2).
2. **Phase: wiring** - failing test asserting a rewritten NLRI reaches one peer only.
3. **Phase: implement (TDD)** - accumulator field/method + forward application; keep adj-rib-out consistent.
4. **Functional test** - `.ci` proving the per-peer rewrite.
5. **Full verification** - `make ze-verify`.
6. **Complete spec** - audit, learned summary, two-commit closure.

## Checklist

### Goal Gates (MUST pass)
- [ ] Filter can rewrite NLRI bytes per destination peer; adj-rib-out stays consistent
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
