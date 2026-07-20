# Spec: rfc7606-5-1-2-relay-shape

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc7606.md` (the RFC7606-5.1-2 `{gap}` annotation), `rfc/full/rfc7606.txt` §5.1
4. `internal/component/bgp/reactor/forward_body.go`, `internal/component/bgp/wireu/split.go`

## Task

Close the remaining half of RFC 7606 Section 5.1's second bullet: "An UPDATE message MUST
NOT contain more than one of the following: non-empty Withdrawn Routes field, non-empty
Network Layer Reachability Information field, MP_REACH_NLRI attribute, and MP_UNREACH_NLRI
attribute."

Carved out of `plan/spec-rfc7606-close-gaps.md` (closed 2026-07-20), which implemented five
of that RFC's gaps and narrowed this one. **Ze already ORIGINATES only compliant UPDATEs,
and both splitters now emit one NLRI-bearing field per message.** What remains is the relay
side, and it is a genuine trade rather than an oversight, which is why it gets its own spec
instead of being left as an unexplained annotation.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `docs/architecture/wire/mp-nlri-ordering.md` - the Section 5.1 FIRST-bullet divergence
  → Constraint: MP_UNREACH-first / MP_REACH-last ordering is deliberate and must not change.
- [ ] `ai/rules/buffer-first.md` - encoding must not allocate per message
  → Constraint: the zero-copy forward path is the thing at risk here.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7606.md` - the RFC7606-5.1-2 annotation records the current state
  → Constraint: Section 5.1 also says an implementation "MUST still be prepared to receive
    these fields in any position or combination", so RECEIVE-side tolerance must not change.
  → Constraint: the restriction exists "To facilitate the determination of the NLRI field in
    an UPDATE message with a malformed attribute" -- it protects a RECEIVER, and every
    receiver is separately required to cope with any combination anyway.

**Key insights:**
- The obligation binds the sender. Ze is the sender of bytes it relays, even when it did
  not compose them, so forwarding a received mixed shape is within scope of the MUST.
- Ze already satisfies the MUST everywhere it CONSTRUCTS an UPDATE.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/bgp/reactor/forward_body.go` - `buildFwdBody` (`:37-107`).
  → Constraint: `:51-65` is the same-context zero-copy path. When the ContextID matches and
    the UPDATE fits, `:64` does `result.rawBodies = append(result.rawBodies, peerWire.Payload())`
    -- a slice-header append, no parse, no copy. The comment at `:48-50` states it is placed
    "before any parse or re-encode" deliberately.
  → Constraint: `:99` appends a re-encoded `destUpdate` whole when it fits. That one IS ze
    constructing an UPDATE -- `fwdUpdateForDestination` (`:82`) rebuilt its sections for the
    destination context -- so it is the weaker of the two justifications for leaving it.
  → Constraint: the oversize branches already split (`:55` via `wireu.SplitWireUpdate`,
    `:93` via `fwdSplitParsedUpdate`), and both splitters are now one-field-per-message. Only
    the two FITS branches are non-compliant.
- [ ] `internal/component/bgp/wireu/split.go` - `buildCombinedUpdates` drains each component
  into its own message (done 2026-07-20).
  → Constraint: `SplitWireUpdate` returns early when the payload fits, so it does not cover
    the fits case.
- [ ] `internal/component/bgp/message/update_split.go` - `splitUpdateWithMP` likewise emits
  one field per chunk (fixed 2026-07-20 in `574e3c596`).
- [ ] Origination is already compliant: `UpdateBuilder.BuildUnicast`
  (`update_build.go:380-383`) sets NLRI without WithdrawnRoutes; withdrawals are
  withdraw-only (`peer_rib_routes.go:170-199`).

**Behavior to preserve:**
- Receive-side tolerance of any position or combination (Section 5.1 requires it).
- MP_UNREACH before MP_REACH.
- End-of-RIB handling.
- Withdrawals before announcements.
- The zero-copy append at `:64` for UPDATEs that carry a single NLRI-bearing field.

**Behavior to change:**
- A relayed UPDATE that mixes NLRI-bearing fields is split before being sent on.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A received UPDATE being forwarded to a peer, via `buildFwdBody`.

### Transformation Path
1. `buildFwdBody:51` same-context and fits → `:64` verbatim payload append. **Non-compliant
   when the received UPDATE mixed fields.**
2. `buildFwdBody:82` context mismatch → `fwdUpdateForDestination` → `:99` whole emit when it
   fits. **Non-compliant when the rebuilt UPDATE mixes fields.**

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Receive ↔ forward | `*wireu.WireUpdate` shared across peers in the forward loop | [ ] |
| Forward ↔ wire | raw payload append (`:64`), or parsed `*message.Update` (`:99`) | [ ] |

### Integration Points
- `buildFwdBody` (`reactor/forward_body.go:37`), its two fits branches.

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality (reuse the existing splitters rather than a third one)
- [ ] Zero-copy preserved where applicable — this is the crux; see R-1
- [ ] Registration over hardcoding — N/A

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Mixed-field UPDATEs are rare in practice | unmeasured | the cost is charged on every forward rather than rarely | count mixed shapes on a real feed before implementing | unvalidated |
| A-2 | The mix/no-mix decision can be made once per RECEIVED UPDATE, not once per peer | the same `*wireu.WireUpdate` pointer is shared across the forward loop and is part of the `bodySlots` cache key (`forward_rs.go:420-431`) | the cost multiplies by peer count | read the loop and the cache key | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Splitting relayed UPDATEs costs the zero-copy same-context forward | throughput regression on a route-reflector benchmark | scan once at receive and mark the WireUpdate, so only mixed ones lose zero-copy |
| R-2 | Message counts change, moving the supersede/dedup identity | `fwdSupersedeKey` hit-rate change (`forward_pool.go:926-938`) | correctness holds either way; measure the hit rate |
| R-3 | `fwdIsWithdrawal` classification changes for split items | ordering/priority treatment differs | re-check the classifier against the new shapes |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| forward a received UPDATE mixing withdrawn + NLRI | → | `buildFwdBody` fits branch | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | ze forwards a received UPDATE carrying both Withdrawn Routes and NLRI, same context, fits | each emitted UPDATE carries at most one NLRI-bearing field |
| AC-2 | Same, context mismatch (re-encoded `destUpdate`, fits) | same |
| AC-3 | ze receives an UPDATE mixing fields | still accepted, unchanged (Section 5.1 receive-side tolerance) |
| AC-4 | ze forwards an UPDATE with exactly one NLRI-bearing field | the `:64` zero-copy append is still taken, no parse, no extra allocation |
| AC-5 | End-of-RIB | unchanged |
| AC-6 | Any split | withdrawals still precede announcements, MP_UNREACH before MP_REACH |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs ze as a route reflector; a client sends a mixed-shape UPDATE | receive → buildFwdBody → split → peers | (fill) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (fill during design) | `internal/component/bgp/reactor/forward_body_test.go` | AC-1, AC-2, AC-4 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| NLRI-bearing fields per emitted UPDATE | 0..1 | 1 | N/A | 2 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rfc7606-relay-one-field.ci` | `test/plugin/rfc7606-relay-one-field.ci` | a mixed-shape UPDATE relayed through ze reaches the peer as separate compliant UPDATEs | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| relay a mixed-shape UPDATE | `test/interop/scenarios/` | FRR or BIRD | the split shape is accepted by another daemon | |

### Future (if deferring any tests)
- None.

## Files to Modify
- `internal/component/bgp/reactor/forward_body.go` - both fits branches
- `internal/component/bgp/wireu/split.go` or `internal/component/bgp/message/update_split.go` - reuse, do not add a third splitter
- `rfc/short/rfc7606.md` - remove the RFC7606-5.1-2 `{gap}` once met
- `rfc/audit/rfc7606.json`, `docs/features/rfc-status.md` - gaps 3 → 2

### BGP Family Checklist (if new SAFI / capability / attribute)
N/A — no new family, capability or attribute.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | no config surface |
| CLI commands/flags | No | - |
| Doctor check | No | - |
| Prometheus counters | Maybe | a split-on-relay counter would make R-1 measurable in production |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 7 | Wire format changed? | Yes | `docs/architecture/wire/` |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc7606.md`, `docs/features/rfc-status.md` |
| others | | No | |

## Files to Create
- `test/plugin/rfc7606-relay-one-field.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-verify-changed` |
| 6-9. Review + fix | Critical Review Checklist |
| 10-12. Deliverables, security, docs | below |
| 13. /ze-review gate | Review Gate |
| 14. Present + close | two-commit closure |

### Implementation Phases

1. **Phase: measure first** — count mixed-shape UPDATEs on a representative feed and record
   the number here. A-1 is unvalidated and the whole cost/benefit turns on it.
2. **Phase: receive-side decision** — determine once per received UPDATE whether it mixes
   fields, cached on the WireUpdate, so the per-peer loop pays nothing in the common case.
3. **Phase: split on relay** — reuse `SplitWireUpdate` / `message.Splitter`.
4. **Phase: disclosure** — drop the `{gap}`, update the rfc-status row and the audit verdict.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Receive tolerance | Section 5.1's "MUST still be prepared to receive" is untouched |
| Ordering | withdrawals before announcements; MP_UNREACH before MP_REACH |
| Zero-copy | the single-field common case still takes the `:64` append with no parse |
| No third splitter | the existing two are reused |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Relay emits one field per UPDATE | the new `.ci` plus a reactor unit test |
| Gate reflects it | `grep -c "{gap" rfc/short/rfc7606.md` returns 2 |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Amplification | a peer sending mixed-shape UPDATEs forces ze to split on every relay; bound the cost |
| Allocation | splitting must not allocate per peer |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Throughput regression | back to DESIGN: the receive-side decision is not doing its job |
| Existing forward test turns red | STOP: a relay regression is a blocker |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

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

- The MUST protects a receiver's ability to locate the NLRI field in an UPDATE with a
  malformed attribute. Every receiver is independently required to cope with any
  combination, so the practical benefit of splitting on relay is smaller than the letter
  suggests. That is an argument about priority, not about whether the obligation applies.

## Core Insight

Ze is the sender of bytes it relays, even when it did not compose them. "We only forward
what we received" is a policy choice, not an exemption from a sender-side MUST.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Carved into its own spec rather than left as an unexplained annotation | leaving the `{gap}` in place with no home | the trade is real and needs measuring first; a spec is where that belongs |

## Known Limitations
- Until this lands, `rfc/short/rfc7606.md` carries the RFC7606-5.1-2 `{gap}` and
  `docs/features/rfc-status.md` discloses it.

## RFC Documentation

Add `// RFC 7606 Section 5.1: "<quoted requirement>"` above the enforcing code.

## Implementation Summary

### What Was Implemented
- (fill during implementation)

### Bugs Found/Fixed
- (fill during implementation)

### Documentation Updates
- (fill during implementation)

### Deviations from Plan
- (fill during implementation)

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Relayed UPDATEs carry one NLRI-bearing field | functional `.ci` + interop | (fill) |
| The zero-copy path survives for single-field UPDATEs | benchmark or allocation assertion | (fill) |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | (fill) | file:line | (fill) |

### Fixes applied
- (fill)

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

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Risks & Assumptions: every A-N confirmed or broken

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only
