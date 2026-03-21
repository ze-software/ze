# Spec: llgr-3-rib-integration

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-llgr-2-state-machine |
| Phase | - |
| Updated | 2026-03-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc9494.md` - LLGR_STALE community, depreference, NO_LLGR deletion
4. `internal/component/bgp/plugins/rib/rib_commands.go` - RIB command handlers
5. `internal/component/bgp/plugins/rib/rib.go` - peerGRState, RIBManager
6. `internal/component/bgp/plugins/rib/storage/` - RouteEntry, PeerRIB stale methods
7. `internal/component/bgp/plugins/rib/bestpath.go` - SelectBest, Candidate

## Task

Implement RIB-side LLGR support: new `rib enter-llgr` and `rib depreference-stale` commands, LLGR_STALE community attachment to stale routes, NO_LLGR route deletion, LLGRStale flag on RouteEntry, and best-path depreference for LLGR-stale routes.

Parent: `spec-llgr-0-umbrella.md`
Depends: `spec-llgr-2-state-machine.md` (state machine dispatches `rib enter-llgr`)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/plugin/rib-storage-design.md` - pool storage, attribute handles
  → Constraint: attributes stored as pool handles; community is a deduplicated pool entry
  → Decision: community attachment = read old handle, create new set, store new handle, release old
- [ ] `docs/architecture/core-design.md` - inter-plugin command protocol
  → Constraint: commands are text strings dispatched via SDK DispatchCommand

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9494.md` - LLGR RIB procedures
  → Constraint: LLGR_STALE (0xFFFF0006) attached when entering LLGR period
  → Constraint: routes with NO_LLGR (0xFFFF0007) deleted immediately on LLGR entry
  → Constraint: LLGR_STALE routes treated as least preferred in route selection
  → Constraint: between two LLGR_STALE routes, normal tiebreaking applies

**Key insights:**
- Community attachment requires modifying stored routes (pool handle update)
- NO_LLGR deletion is per-family (only for the family entering LLGR)
- Depreference is a route selection concern, not an attribute mutation
- LLGRStale flag on RouteEntry avoids scanning communities in best-path (performance)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` - handleCommand switch dispatches: rib status, rib show, rib clear in/out, rib retain-routes, rib release-routes, rib mark-stale, rib purge-stale, rib best, rib help, rib command list, rib event list. markStaleCommand takes peer + restart-time, marks all routes stale, starts expiry timer. purgeStaleCommand takes peer + optional family.
- [ ] `internal/component/bgp/plugins/rib/rib.go` - peerGRState has StaleAt, RestartTime, ExpiresAt, expiryTimer. RIBManager has ribInPool (per-peer PeerRIB), grState map, retainedPeers set. autoExpireStale purges all stale and cleans up grState.
- [ ] `internal/component/bgp/plugins/rib/bestpath.go` - SelectBest compares Candidate structs: LOCAL_PREF (higher wins), AS_PATH length (shorter wins), ORIGIN (lower wins), MED (lower wins), eBGP>iBGP, ORIGINATOR_ID tiebreak. Candidate has PeerAddr, LocalPref, ASPathLen, Origin, MED, PeerASN, LocalASN, FirstAS, OriginatorID.
- [ ] `internal/component/bgp/plugins/rib/storage/` - RouteEntry has Stale bool, pool handles for attributes. PeerRIB has per-family FamilyRIB. FamilyRIB has MarkAllStale, PurgeAllStale, PurgeFamilyStale, StaleCount methods.

**Behavior to preserve:**
- All existing RIB commands (status, show, clear, retain, release, mark-stale, purge-stale, best)
- Best-path selection algorithm steps (LOCAL_PREF, AS_PATH, ORIGIN, MED, eBGP/iBGP, ORIGINATOR_ID)
- Stale flag behavior (MarkAllStale, PurgeAllStale, PurgeFamilyStale)
- peerGRState and autoExpireStale for GR (not LLGR)

**Behavior to change:**
- handleCommand: add `rib enter-llgr` and `rib depreference-stale` commands
- storage.RouteEntry: add LLGRStale bool field
- bestpath.go: add LLGR-stale depreference as first comparison step in SelectBest
- Community pool: enter-llgr attaches LLGR_STALE to stale routes
- FamilyRIB: method to delete routes with NO_LLGR community
- ribCommands list: add new commands with help text

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- `rib enter-llgr <peer> <family> <llst>` command from bgp-gr plugin
- `rib depreference-stale <peer>` command from bgp-gr plugin

### Transformation Path
1. `rib enter-llgr` received -> parse peer, family, LLST
2. Find peer's FamilyRIB for the specified family
3. For each stale route in that family:
   a. Read existing community pool handle
   b. Check for NO_LLGR: if present, delete the route
   c. Otherwise: create new community set with LLGR_STALE appended
   d. Store new community handle, update route entry, release old handle
   e. Set LLGRStale=true on RouteEntry
4. Start LLST safety-net timer (similar to GR's autoExpireStale)
5. `rib depreference-stale` -> set LLGRStale=true on all stale routes for peer (if not already)
6. Best-path: SelectBest checks LLGRStale flag first

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| bgp-gr -> bgp-rib | DispatchCommand("rib enter-llgr ...") | [ ] |
| RIB storage -> pool | Community handle read/create/update/release | [ ] |
| RIB -> best-path | LLGRStale flag on Candidate checked in SelectBest | [ ] |

### Integration Points
- `rib_commands.go:handleCommand` - new cases for enter-llgr and depreference-stale
- `rib.go:peerGRState` - extended with LLGR fields (LLST, llgrExpiryTimer)
- `storage/routeentry.go:RouteEntry` - add LLGRStale bool
- `bestpath.go:SelectBest` - add depreference check
- `bestpath.go:Candidate` - add LLGRStale bool
- `rib_commands.go:extractCandidate` - populate LLGRStale from RouteEntry

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `rib enter-llgr` command | -> | LLGR_STALE attachment + NO_LLGR deletion | `test/plugin/llgr-rib-stale.ci` |
| Best-path with LLGR-stale candidate | -> | Depreference in SelectBest | Unit test `TestSelectBest_LLGRStaleDepreference` |
| `rib depreference-stale` command | -> | LLGRStale flag set on routes | Unit test `TestDepreferenceStale` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `rib enter-llgr 10.0.0.1 ipv4/unicast 3600` | Stale routes for that family get LLGR_STALE community, NO_LLGR routes deleted |
| AC-2 | Route with NO_LLGR community during enter-llgr | Route deleted from RIB |
| AC-3 | Route without communities during enter-llgr | LLGR_STALE community created and attached |
| AC-4 | Route with existing communities during enter-llgr | LLGR_STALE appended to existing community set |
| AC-5 | `rib depreference-stale 10.0.0.1` | All stale routes for peer have LLGRStale=true |
| AC-6 | SelectBest with normal + LLGR-stale candidates | Normal route always wins regardless of other attributes |
| AC-7 | SelectBest with two LLGR-stale candidates | Normal tiebreaking (LOCAL_PREF, AS_PATH, etc.) applies |
| AC-8 | SelectBest with only LLGR-stale candidates | Best among them selected by normal tiebreaking |
| AC-9 | `rib status` with LLGR-stale routes | Status shows LLGR-stale count |
| AC-10 | LLST safety-net timer expires | Stale routes for that family purged, LLGR state cleaned up |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEnterLLGR_Basic` | `internal/.../rib/rib_gr_test.go` | enter-llgr attaches LLGR_STALE to stale routes | |
| `TestEnterLLGR_NoLLGRDeletion` | `internal/.../rib/rib_gr_test.go` | Routes with NO_LLGR deleted on enter-llgr | |
| `TestEnterLLGR_ExistingCommunities` | `internal/.../rib/rib_gr_test.go` | LLGR_STALE appended to existing communities | |
| `TestEnterLLGR_NoCommunities` | `internal/.../rib/rib_gr_test.go` | LLGR_STALE created when route has no communities | |
| `TestDepreferenceStale` | `internal/.../rib/rib_gr_test.go` | depreference-stale sets LLGRStale on all stale routes | |
| `TestSelectBest_LLGRStaleDepreference` | `internal/.../rib/bestpath_test.go` | Normal beats LLGR-stale regardless of LOCAL_PREF | |
| `TestSelectBest_BothLLGRStale` | `internal/.../rib/bestpath_test.go` | Two LLGR-stale candidates: normal tiebreaking | |
| `TestSelectBest_OnlyLLGRStale` | `internal/.../rib/bestpath_test.go` | All LLGR-stale: best among them selected | |
| `TestRouteEntry_LLGRStaleDefault` | `internal/.../rib/storage/stale_test.go` | New RouteEntry has LLGRStale=false | |
| `TestStatusJSON_LLGRStale` | `internal/.../rib/rib_test.go` | Status includes LLGR-stale count | |
| `TestLLGRAutoExpire` | `internal/.../rib/rib_gr_test.go` | LLST safety-net timer purges stale | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| LLST in enter-llgr | 0-16777215 | 16777215 | N/A | N/A |
| Family in enter-llgr | valid family string | "ipv4/unicast" | "" (empty) | "invalid" |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `llgr-rib-stale` | `test/plugin/llgr-rib-stale.ci` | enter-llgr command attaches LLGR_STALE, deletes NO_LLGR | |

### Future (if deferring any tests)
- None; all RIB integration tests are in this phase

## Files to Modify

- `internal/component/bgp/plugins/rib/rib_commands.go` - add enter-llgr, depreference-stale handlers
- `internal/component/bgp/plugins/rib/rib.go` - peerGRState LLGR fields, LLGR status in statusJSON
- `internal/component/bgp/plugins/rib/storage/routeentry.go` - LLGRStale bool field
- `internal/component/bgp/plugins/rib/bestpath.go` - LLGR depreference in SelectBest, LLGRStale on Candidate
- `internal/component/bgp/plugins/rib/rib_commands.go` - extractCandidate populates LLGRStale

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] | N/A (commands are inter-plugin, not user-facing RPCs) |
| RPC count in architecture docs | [ ] | N/A |
| CLI commands/flags | [ ] | N/A |
| CLI usage/help text | [ ] | N/A |
| API commands doc | [x] | `docs/architecture/api/commands.md` (new rib commands) |
| Plugin SDK docs | [ ] | N/A |
| Editor autocomplete | [ ] | N/A |
| Functional test for new RPC/API | [x] | `test/plugin/llgr-rib-stale.ci` |

## Files to Create

- `test/plugin/llgr-rib-stale.ci` - LLGR RIB integration functional test

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

1. **Phase: RouteEntry Flag** -- add LLGRStale bool to RouteEntry
   - Tests: `TestRouteEntry_LLGRStaleDefault`
   - Files: `storage/routeentry.go`, `storage/stale_test.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Best-Path Depreference** -- add LLGR check to SelectBest
   - Tests: `TestSelectBest_LLGRStaleDepreference`, `TestSelectBest_BothLLGRStale`, `TestSelectBest_OnlyLLGRStale`
   - Files: `bestpath.go`, `bestpath_test.go`, `rib_commands.go` (extractCandidate)
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Enter-LLGR Command** -- community attachment and NO_LLGR deletion
   - Tests: `TestEnterLLGR_Basic`, `TestEnterLLGR_NoLLGRDeletion`, `TestEnterLLGR_ExistingCommunities`, `TestEnterLLGR_NoCommunities`
   - Files: `rib_commands.go` (new enterLLGRCommand handler)
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Depreference Command** -- rib depreference-stale handler
   - Tests: `TestDepreferenceStale`
   - Files: `rib_commands.go`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Status + Safety Timer** -- LLGR status display and LLST expiry timer
   - Tests: `TestStatusJSON_LLGRStale`, `TestLLGRAutoExpire`
   - Files: `rib.go`, `rib_commands.go`
   - Verify: tests fail -> implement -> tests pass

6. **Functional tests** -- create .ci file

7. **RFC refs** -- add `// RFC 9494 Section X.Y` comments

8. **Full verification** -- `make ze-verify`

9. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-10 has implementation with file:line |
| Correctness | Community pool handles properly released after update; no handle leak |
| Naming | Command names kebab-case ("enter-llgr"), JSON keys kebab-case ("llgr-stale") |
| Data flow | Community attachment through pool (not direct mutation); depreference via flag (not LOCAL_PREF) |
| Rule: no-layering | enter-llgr replaces (not duplicates) any prior LLGR state |
| Rule: buffer-first | No new wire encoding in this phase (community is pool-stored, not encoded here) |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| LLGRStale on RouteEntry | grep for "LLGRStale" in `storage/routeentry.go` |
| LLGRStale in SelectBest | grep for "LLGRStale" in `bestpath.go` |
| enter-llgr command | grep for "enter-llgr" in `rib_commands.go` |
| depreference-stale command | grep for "depreference-stale" in `rib_commands.go` |
| Functional test | ls `test/plugin/llgr-rib-stale.ci` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Pool handle leak | Every community pool Get() must have matching Release() on the old handle |
| Input validation | enter-llgr: validate family format, LLST range, peer exists |
| Concurrent access | Enter-LLGR iterates routes under lock; no concurrent modification |

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

Add `// RFC 9494 Section 4.2: "<quoted requirement>"` above LLGR_STALE attachment code.
MUST document: LLGR_STALE semantics, NO_LLGR deletion, depreference rules, community handling.

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
- [ ] Write learned summary to `plan/learned/NNN-llgr-3-rib-integration.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
