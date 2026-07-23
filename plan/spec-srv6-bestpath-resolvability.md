# Spec: srv6-bestpath-resolvability -- SRv6 SID Resolvability in Best-Path Selection

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-06-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc9252.md` - Section 5: SID resolvability requirement
4. `internal/component/bgp/plugins/rib/bestpath.go` - gatherCandidatesLocked
5. `internal/plugins/sysrib/sysrib.go` - current SID resolvability enforcement

## Task

RFC 9252 Section 5: "The ingress PE MUST perform a resolvability check for the
SRv6 Service SID before considering the received prefix for the BGP best path
computation."

Currently, SID resolvability is checked in sysrib (post best-path), not in
`gatherCandidatesLocked` (during best-path). This means a route with an
unresolvable SID can win best-path, only to be suppressed at the FIB layer.
The route is still advertised to peers even though it cannot be used locally.

### Why it was deferred (from spec-srv6-review-fixes)

Moving resolvability into `gatherCandidatesLocked` creates a circular
dependency: best-path selection queries Loc-RIB for SID resolvability, but
Loc-RIB is populated from best-path results. The SID's covering route may
itself be subject to best-path selection.

### Approaches to investigate

1. **Two-pass best-path:** first pass selects ignoring SID resolvability,
   second pass filters. Loc-RIB populated between passes.
2. **Lazy resolvability via NH resolver Track/Resolve:** current sysrib
   approach but promoted into the RIB layer with a re-evaluation trigger
   when the SID's covering route changes.
3. **Document as known limitation:** Ze already filters at sysrib. The RFC
   requirement is about not forwarding traffic, which is satisfied. The
   difference is whether the route is advertised to peers.

### Key source files

- `internal/component/bgp/plugins/rib/bestpath.go` - `gatherCandidatesLocked`
- `internal/component/bgp/plugins/rib/rib_bestchange.go` - `IsSRv6Ineligible`
- `internal/plugins/sysrib/sysrib.go` - `srv6SIDResolvable`, `selectBest`
- `internal/core/rib/locrib/locrib.go` - Loc-RIB LPM lookup

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component isolation, registration pattern
- [ ] `plan/learned/776-srv6-prefix-sid.md` - original SRv6 design decisions
- [ ] `plan/learned/906-srv6-review-fixes.md` - state machine fix: store derived state after all gates

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9252.md` - Section 5: resolvability requirement for best-path
  -> Constraint: "MUST perform a resolvability check for the SRv6 Service SID before considering the received prefix"

**Key insights:**
- (fill during design)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rib/bestpath.go` - gatherCandidatesLocked filters via IsSRv6Ineligible (no valid SID), but does NOT check SID resolvability
- [ ] `internal/plugins/sysrib/sysrib.go` - selectBest checks srv6SIDResolvable after best-path; suppresses route from FIB but route is still in Loc-RIB

**Behavior to preserve:**
- IsSRv6Ineligible filtering (no valid SID = ineligible)
- sysrib SID suppression (defense in depth even if RIB-level check added)
- NH resolver Track/Resolve mechanism for SID resolution changes

**Behavior to change:**
- Routes with unresolvable SRv6 SIDs should not win best-path selection

## Data Flow (MANDATORY)

### Entry Point
- Best-path evaluation in `gatherCandidatesLocked` after INSERT/REMOVE

### Transformation Path
1. UPDATE arrives, PrefixSID attribute stored in OtherAttrs
2. `gatherCandidatesLocked` collects candidates, filters via `IsSRv6Ineligible`
3. Best-path winner selected, emitted via `checkBestPathChange`
4. sysrib `selectBest` checks `srv6SIDResolvable`, suppresses if unresolvable
5. Gap: step 3 advertises the route to peers before step 4 can suppress it

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| RIB best-path -> sysrib | bestChangeEntry via EventBus | [ ] |
| RIB best-path -> Loc-RIB | InsertForward in checkBestPathChange | [ ] |
| Loc-RIB -> NH resolver | LPM lookup for SID covering route | [ ] |

### Integration Points
- `gatherCandidatesLocked` (`internal/component/bgp/plugins/rib/bestpath.go`) - candidate filtering; where the resolvability check would be added (already calls `IsSRv6Ineligible`)
- `IsSRv6Ineligible` (`internal/component/bgp/plugins/rib/rib_bestchange.go`) - existing SRv6 eligibility gate the new check extends
- `srv6SIDResolvable` / `selectBest` (`internal/plugins/sysrib/sysrib.go`) - current post-best-path enforcement, preserved as defense in depth
- Loc-RIB LPM lookup (`internal/core/rib/locrib/locrib.go`) - the resolvability query target (source of the circular dependency)
- NH resolver Track/Resolve - existing re-evaluation trigger candidate for approach 2

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling (circular dependency is the core challenge)
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| (fill during design) | | | |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Route with valid SRv6 SID, SID not resolvable | Route excluded from best-path candidates |
| AC-2 | Route with valid SRv6 SID, SID becomes resolvable | Route re-evaluated and selected if best |
| AC-3 | Route without SRv6 SID | No change in behavior |

## 🧪 TDD Test Plan

### Unit Tests
<!-- Planned names derived from ACs; refine during design (spec is skeleton). -->
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBestPathSkipsUnresolvableSRv6SID` | `internal/component/bgp/plugins/rib/bestpath_test.go` | AC-1: route with unresolvable SID excluded from candidates | planned |
| `TestBestPathReevaluatesOnSIDResolvable` | `internal/component/bgp/plugins/rib/bestpath_test.go` | AC-2: route re-evaluated and selected once SID resolvable | planned |
| `TestBestPathNonSRv6RouteUnaffected` | `internal/component/bgp/plugins/rib/bestpath_test.go` | AC-3: route without SRv6 SID sees no behavior change | planned |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | | | | |

### Functional Tests
<!-- Planned names derived from ACs; refine during design (spec is skeleton). -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `srv6-bestpath-unresolvable` | `test/plugin/srv6-bestpath-unresolvable.ci` | AC-1: route with unresolvable SRv6 SID is not selected best / not advertised | planned |
| `srv6-bestpath-resolvable` | `test/plugin/srv6-bestpath-resolvable.ci` | AC-2: SID's covering route appears, route re-evaluated and advertised | planned |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| (fill during design) | | | | |

## Files to Modify
- (fill during design)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| (fill during design) | | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| (fill during design) | | | |

## Files to Create
- (fill during design)

## Implementation Steps

(fill during design)

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
- [ ] AC-1..AC-3 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated (or N/A confirmed)
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-srv6-bestpath-resolvability.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-srv6-bestpath-resolvability.md`
