# Spec: llgr-2-state-machine

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-llgr-1-capability |
| Phase | - |
| Updated | 2026-03-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc9494.md` - LLGR operation timeline, timer interactions
4. `internal/component/bgp/plugins/gr/gr_state.go` - GR state machine
5. `internal/component/bgp/plugins/gr/gr.go` - event handlers, timer expiry callback

## Task

Extend the GR state machine to support the LLGR period. When the GR restart-time expires and LLGR was negotiated, transition to LLGR instead of purging. Manage per-family LLST timers. Handle session re-establishment during LLGR. Handle GR restart-time=0 (skip GR, go straight to LLGR).

Parent: `spec-llgr-0-umbrella.md`
Depends: `spec-llgr-1-capability.md` (LLGR capability stored per-peer)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - plugin event loop, timer patterns
  → Constraint: timers use time.AfterFunc with goroutine-safe callbacks
  → Decision: timer callbacks must acquire state manager mutex

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9494.md` - LLGR operation timeline and procedures
  → Constraint: LLGR begins when GR restart-time expires (or immediately if restart-time=0)
  → Constraint: per-AFI/SAFI LLST timers (independent expiry per family)
  → Constraint: session re-establishment: if F-bit clear or family missing in new LLGR cap, delete stale
  → Constraint: if both GR and LLGR caps missing on reconnect, delete all stale
  → Constraint: LLST continues running until EOR received (EOR stops LLST timer for that family)
- [ ] `rfc/short/rfc4724.md` - GR timer expiry (current behavior to extend)
  → Constraint: current behavior on timer expiry: purge all stale routes

**Key insights:**
- GR and LLGR timers are serial, not parallel: GR first, then LLGR
- LLST is per-family (unlike GR restart-time which is global)
- On reconnect during LLGR, check both GR and LLGR capabilities
- EOR during LLGR stops the LLST timer for that family (same as GR behavior for restart timer)
- GR restart-time=0 + LLST>0: skip GR, immediate LLGR on session drop

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/gr/gr_state.go` - grStateManager with peers map. onSessionDown creates grPeerState with staleFamilies map and restartTimer. onSessionReestablished validates new GR cap, purges non-forwarding families. onEORReceived purges stale per-family. handleTimerExpired calls clearPeerLocked + onTimerExpired callback.
- [ ] `internal/component/bgp/plugins/gr/gr.go:onTimerExpired` - calls releaseRoutes -> DispatchCommand("rib release-routes <peer>"). This is the transition point: currently releases, will instead check for LLGR.
- [ ] `internal/component/bgp/plugins/gr/gr.go:handleStateEvent` - on state=down: calls state.onSessionDown, if activated, sends 3-step sequence (purge-stale, retain-routes, mark-stale). On state=up: calls state.onSessionReestablished, purges returned families.

**Behavior to preserve:**
- GR activation on TCP failure for GR-capable peers (not NOTIFICATION)
- 3-step session-down sequence: purge-stale -> retain-routes -> mark-stale
- EOR handling: purge stale for received EOR family
- Session reestablishment: validate new cap, purge non-forwarding families
- Consecutive restart handling (clearPeerLocked before new state)

**Behavior to change:**
- handleTimerExpired: instead of always releasing, check if LLGR negotiated and transition
- grPeerState: add LLGR fields (llstTimers, inLLGR flag, llgrFamilies, llgrCap)
- onSessionDown: if restart-time=0 + LLGR, enter LLGR immediately
- onSessionReestablished: during LLGR, check both GR and LLGR caps
- onEORReceived: during LLGR, stop LLST timer for that family

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- GR restart timer fires (time.AfterFunc callback)
- Session drops with restart-time=0 and LLGR negotiated

### Transformation Path
1. GR timer fires -> handleTimerExpired -> check if LLGR negotiated for any family
2. If LLGR: per-family with nonzero LLST: start LLST timer, mark inLLGR=true
3. For families with LLST=0: purge stale (same as GR expiry)
4. Dispatch `rib enter-llgr <peer> <family> <llst>` for each LLGR family
5. LLST timer fires -> purge stale for that family, if last family -> release routes
6. Reconnect during LLGR -> validate new GR + LLGR caps, purge as needed

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| bgp-gr state -> bgp-rib | DispatchCommand("rib enter-llgr ...") | [ ] |
| bgp-gr state -> bgp-rib | DispatchCommand("rib purge-stale ...") on LLST expiry | [ ] |
| bgp-gr state -> bgp-rib | DispatchCommand("rib release-routes ...") when all LLGR done | [ ] |

### Integration Points
- `gr_state.go:handleTimerExpired` - transition point (GR -> LLGR or purge)
- `gr_state.go:grPeerState` - extended with LLGR fields
- `gr_state.go:onSessionReestablished` - extended for LLGR reconnect
- `gr_state.go:onEORReceived` - extended for LLGR EOR
- `gr.go:onTimerExpired` - callback now checks LLGR before releasing
- `gr.go:grPlugin` - stores LLGR peer cap alongside GR peer cap

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| GR timer expiry with LLGR negotiated | -> | LLGR state transition | `test/plugin/llgr-transition.ci` |
| Session drop with restart-time=0, LLST>0 | -> | Immediate LLGR entry | Unit test `TestOnSessionDown_SkipGR_DirectLLGR` |
| Reconnect during LLGR period | -> | LLGR reconnect validation | Unit test `TestOnSessionReestablished_DuringLLGR` |
| EOR during LLGR | -> | LLST timer stop for family | Unit test `TestOnEORReceived_DuringLLGR` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | GR restart-time expires, LLGR negotiated with LLST>0 for some families | State transitions to LLGR: per-family LLST timers started, dispatches `rib enter-llgr` per family |
| AC-2 | GR restart-time expires, no LLGR negotiated | Existing behavior: purge all stale, release routes |
| AC-3 | LLST timer expires for one family (others still active) | Stale routes purged for that family only; other families remain in LLGR |
| AC-4 | Last LLST timer expires | Stale routes purged, routes released, GR/LLGR state cleaned up |
| AC-5 | GR restart-time=0, LLST>0 | LLGR entered immediately on session drop (no GR period) |
| AC-6 | Session re-established during LLGR, new OPEN has both GR + LLGR caps | Stop LLST timers; validate F-bits; purge families with F=0 or missing |
| AC-7 | Session re-established during LLGR, new OPEN has GR but no LLGR cap | Delete all stale routes (LLGR not renegotiated) |
| AC-8 | Session re-established during LLGR, new OPEN has neither GR nor LLGR | Delete all stale routes |
| AC-9 | EOR received during LLGR for a family | Stop LLST timer for that family, purge stale for that family |
| AC-10 | LLGR negotiated for some families but not others on GR timer expiry | Only families with LLST>0 enter LLGR; others purged on GR timer expiry |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOnTimerExpired_WithLLGR` | `internal/.../gr/gr_state_test.go` | GR timer expiry transitions to LLGR for families with LLST>0 | |
| `TestOnTimerExpired_WithoutLLGR` | `internal/.../gr/gr_state_test.go` | GR timer expiry without LLGR purges all (existing behavior) | |
| `TestOnTimerExpired_MixedFamilies` | `internal/.../gr/gr_state_test.go` | Some families have LLST, others don't; correct per-family behavior | |
| `TestLLSTTimerExpiry_SingleFamily` | `internal/.../gr/gr_state_test.go` | One LLST timer fires, others continue | |
| `TestLLSTTimerExpiry_LastFamily` | `internal/.../gr/gr_state_test.go` | Last LLST timer fires, state cleaned up | |
| `TestOnSessionDown_SkipGR_DirectLLGR` | `internal/.../gr/gr_state_test.go` | restart-time=0, LLST>0: LLGR entered immediately | |
| `TestOnSessionDown_ZeroGR_ZeroLLST` | `internal/.../gr/gr_state_test.go` | restart-time=0, LLST=0: no GR or LLGR | |
| `TestOnSessionReestablished_DuringLLGR` | `internal/.../gr/gr_state_test.go` | Reconnect during LLGR with valid caps; LLST timers stopped | |
| `TestOnSessionReestablished_DuringLLGR_NoLLGRCap` | `internal/.../gr/gr_state_test.go` | Reconnect during LLGR, no LLGR in new OPEN: purge all | |
| `TestOnSessionReestablished_DuringLLGR_NoCaps` | `internal/.../gr/gr_state_test.go` | Reconnect during LLGR, no GR/LLGR: purge all | |
| `TestOnEORReceived_DuringLLGR` | `internal/.../gr/gr_state_test.go` | EOR during LLGR stops LLST timer for that family | |
| `TestOnEORReceived_DuringLLGR_LastFamily` | `internal/.../gr/gr_state_test.go` | EOR for last LLGR family completes LLGR | |
| `TestConsecutiveRestart_DuringLLGR` | `internal/.../gr/gr_state_test.go` | New session drop while in LLGR: clean old state, start fresh | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| LLST | 0-16777215 | 16777215 seconds (~194 days) | N/A | N/A (wire is 24-bit) |
| restart-time | 0-4095 | 0 (special: skip GR) | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `llgr-transition` | `test/plugin/llgr-transition.ci` | GR timer expires, LLGR activates, `rib enter-llgr` dispatched | |

### Future (if deferring any tests)
- None; all state machine tests are in this phase

## Files to Modify

- `internal/component/bgp/plugins/gr/gr_state.go` - extend grPeerState, onSessionDown, onSessionReestablished, onEORReceived, handleTimerExpired
- `internal/component/bgp/plugins/gr/gr.go` - extend onTimerExpired callback, store LLGR cap per-peer, new LLGR dispatch commands

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] | N/A (no new RPCs in this phase) |
| RPC count in architecture docs | [ ] | N/A |
| CLI commands/flags | [ ] | N/A |
| CLI usage/help text | [ ] | N/A |
| API commands doc | [ ] | N/A |
| Plugin SDK docs | [ ] | N/A |
| Editor autocomplete | [ ] | N/A |
| Functional test for new RPC/API | [x] | `test/plugin/llgr-transition.ci` |

## Files to Create

- `test/plugin/llgr-transition.ci` - GR-to-LLGR transition functional test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: State Structure** -- extend grPeerState with LLGR fields
   - Tests: compilation only (fields are data)
   - Files: `gr_state.go`
   - Verify: compiles

2. **Phase: GR Timer Transition** -- handleTimerExpired checks for LLGR
   - Tests: `TestOnTimerExpired_WithLLGR`, `TestOnTimerExpired_WithoutLLGR`, `TestOnTimerExpired_MixedFamilies`
   - Files: `gr_state.go` (handleTimerExpired, new enterLLGR method)
   - Verify: tests fail -> implement -> tests pass

3. **Phase: LLST Timers** -- per-family LLST timer management
   - Tests: `TestLLSTTimerExpiry_SingleFamily`, `TestLLSTTimerExpiry_LastFamily`
   - Files: `gr_state.go` (new handleLLSTExpired method)
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Skip-GR Path** -- restart-time=0 direct LLGR entry
   - Tests: `TestOnSessionDown_SkipGR_DirectLLGR`, `TestOnSessionDown_ZeroGR_ZeroLLST`
   - Files: `gr_state.go` (onSessionDown extended)
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Reconnect During LLGR** -- extended onSessionReestablished
   - Tests: `TestOnSessionReestablished_DuringLLGR`, `TestOnSessionReestablished_DuringLLGR_NoLLGRCap`, `TestOnSessionReestablished_DuringLLGR_NoCaps`
   - Files: `gr_state.go` (onSessionReestablished extended)
   - Verify: tests fail -> implement -> tests pass

6. **Phase: EOR During LLGR** -- extended onEORReceived
   - Tests: `TestOnEORReceived_DuringLLGR`, `TestOnEORReceived_DuringLLGR_LastFamily`
   - Files: `gr_state.go` (onEORReceived extended)
   - Verify: tests fail -> implement -> tests pass

7. **Phase: Plugin Wiring** -- extend gr.go callbacks to dispatch LLGR commands
   - Tests: `TestConsecutiveRestart_DuringLLGR`
   - Files: `gr.go` (onTimerExpired, new dispatchLLGREntry)
   - Verify: tests fail -> implement -> tests pass

8. **Functional tests** -- create .ci file

9. **RFC refs** -- add `// RFC 9494 Section X.Y` comments

10. **Full verification** -- `make ze-verify`

11. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-10 has implementation with file:line |
| Correctness | Timer transitions are serial (GR then LLGR), LLST per-family not global |
| Naming | LLGR-related fields use consistent prefix (llgr/LLGR) |
| Data flow | State machine dispatches commands; never accesses RIB directly |
| Rule: no-layering | GR timer expiry path modified in-place (not duplicated) |
| Rule: goroutine-lifecycle | LLST timers use time.AfterFunc (existing pattern), no new goroutines |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| grPeerState has LLGR fields | grep for "llgr" in `gr_state.go` |
| handleTimerExpired checks LLGR | grep for "LLGR\|llgr" in `gr_state.go` handleTimerExpired |
| LLST timer management | grep for "LLST\|llst" in `gr_state.go` |
| Skip-GR path | grep for "restart.*0\|skip.*GR" in `gr_state.go` |
| Functional test | ls `test/plugin/llgr-transition.ci` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Timer overflow | LLST up to 16777215 seconds: Duration conversion safe (fits int64 nanoseconds) |
| Concurrent access | LLST timer callbacks must acquire mutex before modifying state |
| Timer leak | All LLST timers stopped on reconnect, consecutive restart, or cleanup |
| Resource exhaustion | Per-family timers bounded by number of negotiated families (small, finite) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
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

## RFC Documentation

Add `// RFC 9494 Section 4.3: "<quoted requirement>"` above LLGR transition code.
MUST document: LLGR entry conditions, LLST timer semantics, F-bit validation on reconnect, skip-GR path.

## Implementation Summary

### What Was Implemented
- (to be filled after implementation)

### Bugs Found/Fixed
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan
- (to be filled)

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
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

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-llgr-2-state-machine.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
