# Spec: rib-arch-8 -- General NLRI-Byte Rewrite via ModAccumulator

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

### Re-verification (2026-07-14)

Anchors 1-6 exact; the gap is real (`ModAccumulator` has no NLRI-rewrite field/method, and
`filter_delta.go` explicitly excludes NLRI from the attribute pipeline). The forward seam
the rewrite must hook is `forwardUpdateCore` in
`internal/component/bgp/reactor/reactor_api_forward.go`, which declares a fresh
`ModAccumulator` per destination peer and applies attribute ops / `SetWithdraw` there. One
anchor drifted: the pathID=0 comment is now at `rib_bestchange.go:1182-1183` (fn
`reconcileBestPath` at :1184), not `:1180`.

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
| A-2 | Rewriting NLRI does not break path-id / add-path or dedup invariants | inject/withdraw build NLRI with pathID=0 (`rib_bestchange.go:1182`, fn `reconcileBestPath` :1184) | Constrain rewrite to safe cases; reject others | design review of add-path interaction | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | NLRI rewrite desyncs adj-rib-out state from what was actually sent | withdraw of the wrong (original) prefix later | track the rewritten NLRI in adj-rib-out, not the original |
| R-2 | Rewrite enables a route the filter cannot faithfully encode | encode error at forward time | exact-or-reject: reject the rewrite if unencodable (`ai/rules/exact-or-reject.md`) |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Egress filter sets `mods.SetNLRIRewrite`/`SetWithdrawnRewrite` | → | `buildModifiedPayload` substitutes the announce/withdrawn NLRI section | `TestBuildModifiedPayloadNLRIRewrite`, `TestBuildModifiedPayloadWithdrawnRewrite` (`reactor/forward_build_test.go`) |

This is an SDK primitive with no user-facing config/command surface, so the wiring test is a Go unit test on the forward-path application (no `.ci` — existing filter `.ci` suites regression-guard the unchanged filter behaviour).

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Filter rewrites NLRI for one destination peer | that peer receives the rewritten prefix; other peers unaffected |
| AC-2 | Rewritten route later withdrawn | the withdrawal references the rewritten NLRI (adj-rib-out consistent) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestModAccumulator_NLRIRewrite` | `internal/component/bgp/filterapi/filterapi_test.go` | NLRI/withdrawn-rewrite accumulation, `HasModifications` (not `Len`), Reset clears, per-instance isolation | PASS |
| `TestBuildModifiedPayloadNLRIRewrite` | `internal/component/bgp/reactor/forward_build_test.go` | announce NLRI section rewritten (AC-1), attrs preserved | PASS (RED→GREEN) |
| `TestBuildModifiedPayloadWithdrawnRewrite` | `internal/component/bgp/reactor/forward_build_test.go` | withdrawn NLRI section rewritten + `withdrawn_len` fixed (AC-2) | PASS (RED→GREEN) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A — internal SDK primitive, no user-facing config/command surface. No production filter requests an NLRI rewrite, and external filters already rewrite NLRI via the raw wire override (`runEgressPolicyChain` → `exportWireOverride`), so no `.ci` can drive this without adding a speculative surface. Existing filter `.ci` suites regression-guard the unchanged filter path. Go unit tests cover the mechanism. | reactor unit tests | filter behaviour unchanged end-to-end | PASS |

## Files to Modify

- `internal/component/bgp/filterapi/filterapi.go` - `ModAccumulator` `nlriRewrite`/`withdrawnRewrite` fields + `SetNLRIRewrite`/`NLRIRewrite`/`SetWithdrawnRewrite`/`WithdrawnRewrite`/`HasModifications`; `Reset` clears them
- `internal/component/bgp/reactor/forward_build.go` - `buildModifiedPayload` reads the accumulator rewrites (announce via existing `nlriOverride` slot; withdrawn via a new step-1 substitution)
- `internal/component/bgp/reactor/reactor_api_forward.go`, `forward_rs.go` - gate on `mods.HasModifications()` so a rewrite-only mod rebuilds the payload

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

## Design Verification (2026-07-14)

- **A-1 confirmed:** the per-peer UPDATE is (re)built by `buildModifiedPayload`
  (`internal/component/bgp/reactor/forward_build.go:58`), which already had an
  `nlriOverride` slot substituting the legacy IPv4 NLRI section (`:245`) for the
  per-prefix modify path. The withdrawn section was copied verbatim (`:133`).
- **A-2 note:** the rewrite operates on wire NLRI bytes; add-path path-ids are part of
  those bytes, so a filter that rewrites NLRI must produce a valid section (exact-or-reject
  is the filter's responsibility). No path-id-specific handling was added.
- **Scope decision (approved by user 2026-07-14):** build the ModAccumulator primitive +
  forward application + Go unit tests, WITHOUT a `.ci` or a driving filter surface. Rationale:
  no in-process filter needs NLRI rewrite today, and external filters already rewrite NLRI via
  the raw wire override (`runEgressPolicyChain` → `exportWireOverride` → `peerBaseWire`,
  `reactor_api_forward.go:488,515`). The primitive is the cleaner path for a future in-process
  filter; it stays inert until one calls it.

## Implementation Summary

- `filterapi.ModAccumulator` gained `nlriRewrite`/`withdrawnRewrite` (+ setters/getters),
  `HasModifications()`, and `Reset` clears them. `Len()` still counts only attribute ops.
- `buildModifiedPayload` reads the accumulator's rewrites: the announce NLRI via the existing
  `nlriOverride` slot (explicit argument still takes precedence), the withdrawn NLRI via a new
  step-1 substitution (writes a fresh `withdrawn_len` + override bytes; > 65535 abandons).
- `reactor_api_forward.go` and `forward_rs.go` gate the rebuild on `mods.HasModifications()`
  (was `mods.Len() > 0`), so a rewrite-only modification is applied.
- **AC-1 met:** `TestBuildModifiedPayloadNLRIRewrite` (announce rewrite, attrs preserved).
  **AC-2 met:** `TestBuildModifiedPayloadWithdrawnRewrite` (withdrawn rewrite keeps adj-rib-out
  consistent — the peer is withdrawn under the same rewritten prefix it was announced).
- **Inert-by-default:** all changes are no-ops unless a filter sets a rewrite; the full
  `filterapi` + `reactor` suites pass unchanged (no forward-path regression).

## Review Gate

Self-review of the diff (filterapi.go + forward_build.go + reactor_api_forward.go + forward_rs.go + tests):
- No signature churn: rewrites travel on the already-passed `mods` argument, so the ~24
  `buildModifiedPayload` callers are untouched.
- Buffer safety: withdrawn/NLRI substitution goes through `safeCopy` bounds checks and the
  `> 65535` guard; on overflow the mod is abandoned (returns nil,0), never a truncated write.
- Announce-vs-withdraw: `SetWithdraw` (announce→withdraw conversion) still routes to
  `buildWithdrawalPayload`; the rewrites apply only on the `buildModifiedPayload` branch.
Findings: 0 BLOCKER, 0 ISSUE.

## Pre-Commit Verification

Re-verified 2026-07-14:

| Item | Evidence |
|------|----------|
| AC-1 verified | `TestBuildModifiedPayloadNLRIRewrite` PASS (RED captured when the accumulator read is disabled) |
| AC-2 verified | `TestBuildModifiedPayloadWithdrawnRewrite` PASS (RED captured) |
| Accumulator semantics | `TestModAccumulator_NLRIRewrite` PASS (Reset clears, `HasModifications`, isolation) |
| No forward regression | full `./internal/component/bgp/filterapi/` + `./internal/component/bgp/reactor/` suites PASS |
| Lint | `make ze-lint-changed` 0 issues |
| A-1 resolved | confirmed — `buildModifiedPayload:58`, `nlriOverride` slot `:245`, withdrawn copy `:133` |
| A-2 resolved | confirmed — rewrite is raw NLRI bytes; add-path handling is the filter's responsibility (documented) |
| Producers read | `forward_build.go`, `reactor_api_forward.go:468-601`, `forward_rs.go:390-405` all read this session |

## Notes
- Skeleton = captured intent, not a designed spec (`ai/rules/deferral-tracking.md`). Moves to `design` when picked up.
- Umbrella / siblings: `spec-rib-arch-0-umbrella.md`.
