# Spec: llgr-4-readvertisement

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-llgr-3-rib-integration |
| Phase | - |
| Updated | 2026-03-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc9494.md` - LLGR_STALE readvertisement rules, partial deployment
4. `internal/component/bgp/plugins/rib/rib_commands.go` - route forwarding, sendRoutes
5. `internal/component/bgp/reactor/` - forward path, UPDATE building
6. Forward pool architecture (if exists)

## Task

When LLGR begins, stale routes with LLGR_STALE attached must be re-advertised to LLGR-capable peers and withdrawn from non-LLGR peers. For IBGP partial deployment, routes may be sent with NO_EXPORT and LOCAL_PREF=0.

Parent: `spec-llgr-0-umbrella.md`
Depends: `spec-llgr-3-rib-integration.md` (LLGR_STALE attached, depreference working)

**Note:** This is the most research-dependent phase. The forward path and UPDATE building pipeline must be understood before detailed design. This spec will be refined during research.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - forward path, UPDATE building
  → Decision: (to be filled during research)
  → Constraint: (to be filled during research)
- [ ] `docs/architecture/update-building.md` - how UPDATEs are constructed for peers
  → Decision: (to be filled during research)
- [ ] Forward pool docs (if they exist)
  → Decision: (to be filled during research)

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9494.md` - LLGR readvertisement rules
  → Constraint: LLGR_STALE routes SHOULD NOT be advertised to peers without LLGR capability
  → Constraint: MUST NOT remove LLGR_STALE community when readvertising
  → Constraint: partial deployment (IBGP): attach NO_EXPORT, set LOCAL_PREF=0
  → Constraint: only used as last resort (no better route available)

**Key insights:**
- Readvertisement requires triggering UPDATE building for affected routes
- Peer capability awareness needed in the forwarding path (which peers support LLGR)
- Partial deployment rules apply only to IBGP peers without LLGR
- The mechanism to trigger readvertisement from RIB needs investigation

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go:sendRoutes` - replays routes to a peer using formatRouteCommand + updateRoute. Used by outboundResendJSON for rib clear out.
- [ ] Forward path files (to be identified during research)

**Behavior to preserve:**
- Normal route advertisement path unchanged
- UPDATE building for non-LLGR routes unchanged
- outboundResendJSON behavior for rib clear out

**Behavior to change:**
- After enter-llgr: trigger readvertisement of LLGR_STALE routes
- Forward path: suppress LLGR_STALE routes toward non-LLGR peers
- Partial deployment: attach NO_EXPORT + LOCAL_PREF=0 for IBGP non-LLGR peers (if configured)

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- `rib enter-llgr` completes (from spec-llgr-3) -> triggers readvertisement
- Best-path recomputation selects LLGR-stale route as best -> forward to eligible peers

### Transformation Path
1. enter-llgr attaches LLGR_STALE to stale routes (spec-llgr-3)
2. Readvertisement trigger: RIB signals that affected routes need re-forwarding
3. Forward path checks each destination peer's LLGR capability
4. LLGR-capable peers: receive UPDATE with LLGR_STALE in community attribute
5. Non-LLGR peers: route withdrawn (or sent with NO_EXPORT + LOCAL_PREF=0 for IBGP partial deployment)
6. UPDATE building includes LLGR_STALE in community attribute (MUST NOT remove)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| RIB -> Forward path | Route re-advertisement trigger (mechanism TBD) | [ ] |
| Forward path -> Peer capability | Check LLGR cap per destination peer | [ ] |
| Forward path -> UPDATE building | Include LLGR_STALE in community attribute | [ ] |

### Integration Points
- Forward path: per-peer capability check for LLGR
- UPDATE building: community attribute includes LLGR_STALE
- RIB: trigger mechanism for readvertisement
- (Specific files to be identified during research)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| LLGR readvertisement trigger | -> | UPDATE with LLGR_STALE to capable peer | `test/plugin/llgr-readvertise.ci` |
| LLGR route to non-LLGR peer | -> | Route suppressed | Unit test `TestForward_LLGRStale_NonLLGRPeer` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | LLGR_STALE route, destination peer has LLGR capability | Route advertised with LLGR_STALE community included |
| AC-2 | LLGR_STALE route, destination peer lacks LLGR capability | Route NOT advertised (withdrawn if previously sent) |
| AC-3 | LLGR_STALE route readvertised | LLGR_STALE community NOT removed from attributes |
| AC-4 | Partial deployment: IBGP peer without LLGR | Route sent with NO_EXPORT + LOCAL_PREF=0 (if partial deployment enabled) |
| AC-5 | Session re-established, routes become non-stale | Routes re-advertised normally (without LLGR_STALE) to all peers |
| AC-6 | Multiple LLGR-stale routes for same prefix | Only best among them forwarded (depreference already applied) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestForward_LLGRStale_LLGRPeer` | TBD (forward path test file) | LLGR_STALE route forwarded to LLGR-capable peer | |
| `TestForward_LLGRStale_NonLLGRPeer` | TBD (forward path test file) | LLGR_STALE route suppressed for non-LLGR peer | |
| `TestForward_LLGRStale_CommunityPreserved` | TBD (forward path test file) | LLGR_STALE not removed during forwarding | |
| `TestForward_PartialDeployment_IBGP` | TBD (forward path test file) | NO_EXPORT + LOCAL_PREF=0 for IBGP non-LLGR peer | |
| `TestReadvertisement_AfterEnterLLGR` | TBD | Routes re-advertised after enter-llgr | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (no new numeric inputs in this phase) | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `llgr-readvertise` | `test/plugin/llgr-readvertise.ci` | LLGR_STALE route forwarded to capable peer, suppressed to others | |

### Future (if deferring any tests)
- Partial deployment IBGP tests (may require multi-peer functional test infrastructure)

## Files to Modify

- (To be determined during research phase -- forward path files)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] | N/A |
| RPC count in architecture docs | [ ] | N/A |
| CLI commands/flags | [ ] | N/A |
| CLI usage/help text | [ ] | N/A |
| API commands doc | [ ] | N/A |
| Plugin SDK docs | [ ] | N/A |
| Editor autocomplete | [ ] | N/A |
| Functional test for new RPC/API | [x] | `test/plugin/llgr-readvertise.ci` |

## Files to Create

- `test/plugin/llgr-readvertise.ci` - readvertisement functional test

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

1. **Phase: Research** -- trace forward path and UPDATE building pipeline
   - Tests: none (research only)
   - Files: identify all files involved in route forwarding
   - Verify: can name 3+ files in forward path, understand trigger mechanism

2. **Phase: Readvertisement Trigger** -- mechanism for re-advertising after enter-llgr
   - Tests: `TestReadvertisement_AfterEnterLLGR`
   - Files: (TBD from research)
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Peer Capability Filter** -- suppress LLGR_STALE to non-LLGR peers
   - Tests: `TestForward_LLGRStale_LLGRPeer`, `TestForward_LLGRStale_NonLLGRPeer`
   - Files: (TBD from research)
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Community Preservation** -- LLGR_STALE not removed during forwarding
   - Tests: `TestForward_LLGRStale_CommunityPreserved`
   - Files: (TBD from research)
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Partial Deployment** -- IBGP non-LLGR peers get NO_EXPORT + LOCAL_PREF=0
   - Tests: `TestForward_PartialDeployment_IBGP`
   - Files: (TBD from research)
   - Verify: tests fail -> implement -> tests pass

6. **Functional tests** -- create .ci file

7. **RFC refs** -- add `// RFC 9494 Section X.Y` comments

8. **Full verification** -- `make ze-verify`

9. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-6 has implementation with file:line |
| Correctness | LLGR_STALE never stripped during forwarding; suppression is correct per-peer |
| Naming | Consistent with existing forward path naming conventions |
| Data flow | Readvertisement uses existing forward path (not a parallel mechanism) |
| Rule: no-layering | Peer filter extends existing output filter (not a separate layer) |
| Rule: buffer-first | Any UPDATE building uses existing WriteTo patterns |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Readvertisement trigger | grep for readvertise/re-advertise in modified files |
| Peer LLGR capability filter | grep for LLGR in forward path files |
| Functional test | ls `test/plugin/llgr-readvertise.ci` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Community integrity | LLGR_STALE not accidentally stripped or duplicated |
| Partial deployment | NO_EXPORT prevents route leaking beyond AS boundary |

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

Add `// RFC 9494 Section 4.5: "<quoted requirement>"` above peer filtering code.
MUST document: readvertisement trigger, LLGR_STALE preservation, non-LLGR suppression, partial deployment rules.

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
- [ ] AC-1..AC-6 all demonstrated
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
- [ ] Write learned summary to `plan/learned/NNN-llgr-4-readvertisement.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
