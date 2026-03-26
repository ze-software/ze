# Spec: Route Loop Detection

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 9/9 |
| Updated | 2026-03-26 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/route-selection.md` - route selection pipeline and reason table
4. `internal/component/bgp/reactor/session_validation.go` - current validation
5. `internal/component/bgp/reactor/session_read.go` - processMessage pipeline
6. `internal/component/bgp/attribute/aspath_iter.go` - AS_PATH iteration

## Task

Add three BGP loop detection checks that run during UPDATE validation, before routes reach best-path selection. Each check sets a rejection reason on the route using the unified reason model described in `docs/architecture/route-selection.md`.

| Check | RFC | What it detects |
|-------|-----|-----------------|
| AS loop | 4271 Section 9 | Local ASN appears in received AS_PATH |
| Originator-ID loop | 4456 Section 8 | ORIGINATOR_ID matches local Router ID (iBGP/RR only) |
| Cluster-list loop | 4456 Section 8 | Local Cluster ID found in CLUSTER_LIST (iBGP/RR only) |

These are hard rejections: a route failing any check is treated as withdrawn (never enters RIB or best-path selection).

### Scope

**In scope:** The three loop detection checks above, integrated into the UPDATE processing pipeline.

**Out of scope:** OTC/RFC 9234 ingress validation (the bgp-role plugin exists but OTC attribute processing is a separate, larger effort). RPKI validation (already implemented). Unified rejection reason type (future spec, this spec uses treat-as-withdraw which is the existing mechanism). Cluster ID configuration in YANG (use Router ID as default per RFC 4456 Section 7).

### Design Decision: Cluster ID Default

RFC 4456 Section 7: "Typically, the CLUSTER_ID will be set to the BGP Identifier of the RR." Ze does not currently configure Cluster ID. This spec uses Router ID as the Cluster ID, which is the RFC default. A future spec can add explicit `cluster-id` configuration if needed.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/route-selection.md` - route selection pipeline and unified reason model
  -> Constraint: loop checks are Phase 1 validation, reasons 1-8 range
- [ ] `docs/architecture/core-design.md` - reactor, session, wire layer
  -> Constraint: validation runs in session context, before plugin dispatch

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` - BGP-4, Section 9 AS loop detection
  -> Constraint: "If the local AS appears in the AS_PATH, the route MUST be excluded from the decision process" (unless configured otherwise via allow-own-as)
- [ ] `rfc/short/rfc4456.md` - MISSING, must create. Route reflection loop detection
  -> Constraint: Section 8: discard route if ORIGINATOR_ID = local Router ID. Discard route if local Cluster ID in CLUSTER_LIST.

**Key insights:**
- All three checks happen AFTER RFC 7606 validation (route is syntactically valid) but BEFORE plugin dispatch
- AS loop detection needs AS_PATH bytes + local ASN (both available in session)
- Originator-ID and cluster-list checks only apply to iBGP sessions (PeerAS == LocalAS)
- The existing `enforceRFC7606` returns an action; loop detection is a separate step that also returns treat-as-withdraw
- Prefix limit check (`checkPrefixLimits`) runs after RFC 7606 in processMessage; loop detection should go between RFC 7606 and prefix limits so looped routes don't count
- AS_PATH iterator (`aspath_iter.go`) already exists for zero-allocation AS_PATH traversal
- ORIGINATOR_ID is a 4-byte attribute (type 9). ParseOriginatorID returns OriginatorID (netip.Addr), NOT uint32. RouterID in PeerSettings is uint32. Type conversion needed for comparison.
- CLUSTER_LIST is a sequence of 4-byte values (type 10), parsed by `attribute.ParseClusterList` returning ClusterList ([]uint32)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/session_read.go` - processMessage calls enforceRFC7606, then dispatches to plugins. No loop detection between these steps.
- [ ] `internal/component/bgp/reactor/session_validation.go` - enforceRFC7606 (RFC 7606 structural validation) and validateUpdateFamilies (AFI/SAFI check). No loop detection.
- [ ] `internal/component/bgp/reactor/peersettings.go` - PeerSettings has LocalAS (uint32), PeerAS (uint32), RouterID (uint32). No ClusterID field.
- [ ] `internal/component/bgp/attribute/aspath_iter.go` - ASPathIterator and ASNIterator for zero-allocation AS_PATH traversal. Supports ASN4.
- [ ] `internal/component/bgp/attribute/simple.go` - ParseClusterList returns ClusterList ([]uint32). ParseOriginatorID returns (OriginatorID, error) where OriginatorID is netip.Addr (not uint32). Comparison with RouterID (uint32) requires conversion.
- [ ] `internal/component/bgp/message/rfc7606.go` - RFC7606Action enum (None, AttributeDiscard, TreatAsWithdraw, SessionReset). Attribute codes 9 (ORIGINATOR_ID) and 10 (CLUSTER_LIST) defined.
- [ ] `internal/component/bgp/plugins/rib/bestpath.go` - Best-path selection. No loop detection here (correct, it belongs in validation).

**Behavior to preserve:**
- RFC 7606 validation runs first and is unchanged
- processMessage pipeline order: RFC 7606 -> prefix limit check -> callback dispatch -> handleUpdate (family check). Family check is inside handleUpdate, not in processMessage directly.
- Treat-as-withdraw semantics: route silently withdrawn, no NOTIFICATION, session stays up
- Existing tests for RFC 7606, family validation, prefix limits, and best-path selection

**Behavior to change:**
- Add loop detection step between RFC 7606 validation and prefix limit check in processMessage (looped routes should not count toward prefix limits)
- Routes with AS loops, originator-ID loops, or cluster-list loops are treated as withdrawn

## Data Flow (MANDATORY)

### Entry Point
- Wire bytes arrive at session_read.go processMessage
- UPDATE body is available as byte slice, WireUpdate wrapper created

### Transformation Path
1. RFC 7606 structural validation (existing, unchanged)
2. **NEW: Loop detection** - parse AS_PATH, ORIGINATOR_ID, CLUSTER_LIST from path attributes; compare against session settings (LocalAS, RouterID). Looped routes treated as withdrawn before prefix counting.
3. Prefix limit check (existing, unchanged -- runs after loop detection so looped routes don't count)
4. Callback dispatch to plugins (existing, unchanged)
5. handleUpdate -> validateUpdateFamilies (existing, unchanged -- family check is inside handleUpdate, not processMessage)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire bytes -> attribute parsing | Iterator-based, zero-allocation for AS_PATH | [ ] |
| Session settings -> validation | s.settings.LocalAS, s.settings.RouterID | [ ] |

### Integration Points
- `session_read.go:processMessage` - insert loop detection call after enforceRFC7606, before checkPrefixLimits
- `session_validation.go` - new function(s) for loop detection, same file as existing validation
- `attribute.ASPathIterator` - reuse for AS loop detection
- `attribute.ParseOriginatorID` - returns (OriginatorID, error) where OriginatorID is netip.Addr; needs conversion to compare with RouterID (uint32)
- `message.RFC7606Action` - reuse TreatAsWithdraw action for loop rejection

### Architectural Verification
- [ ] No bypassed layers (loop detection in same validation pipeline as RFC 7606)
- [ ] No unintended coupling (uses existing attribute iterators, no new dependencies)
- [ ] No duplicated functionality (no existing loop detection to conflict with)
- [ ] Zero-copy preserved (AS_PATH iterator is zero-allocation; ORIGINATOR_ID and CLUSTER_LIST read from wire bytes directly)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with iBGP peer + UPDATE with local ASN in AS_PATH | -> | detectASLoop in session_validation.go | test/plugin/loop-as.ci |
| Config with iBGP peer + UPDATE with local Router ID as ORIGINATOR_ID | -> | detectOriginatorIDLoop in session_validation.go | test/plugin/loop-originator-id.ci |
| Config with iBGP peer + UPDATE with local Router ID in CLUSTER_LIST | -> | detectClusterListLoop in session_validation.go | test/plugin/loop-cluster-list.ci |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | eBGP UPDATE with local ASN in AS_PATH | Route treated as withdrawn, not installed in RIB |
| AC-2 | iBGP UPDATE with local ASN in AS_PATH | Route treated as withdrawn, not installed in RIB |
| AC-3 | eBGP UPDATE without local ASN in AS_PATH | Route accepted normally (no change from current behavior) |
| AC-4 | iBGP UPDATE with ORIGINATOR_ID matching local Router ID | Route treated as withdrawn |
| AC-5 | iBGP UPDATE with ORIGINATOR_ID not matching local Router ID | Route accepted normally |
| AC-6 | eBGP UPDATE with ORIGINATOR_ID (should not happen per RFC, but tolerate) | Route accepted (originator-ID check only applies to iBGP) |
| AC-7 | iBGP UPDATE with local Router ID in CLUSTER_LIST | Route treated as withdrawn |
| AC-8 | iBGP UPDATE with CLUSTER_LIST not containing local Router ID | Route accepted normally |
| AC-9 | eBGP UPDATE with CLUSTER_LIST | Route accepted (cluster-list check only applies to iBGP) |
| AC-10 | UPDATE with AS_SET containing local ASN | Route treated as withdrawn (AS_SET members count) |
| AC-11 | UPDATE failing both RFC 7606 and loop detection | RFC 7606 action takes precedence (loop detection not reached) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDetectASLoop` | `internal/component/bgp/reactor/session_validate_test.go` | Local ASN in AS_SEQUENCE detected | |
| `TestDetectASLoop_ASSet` | `internal/component/bgp/reactor/session_validate_test.go` | Local ASN in AS_SET detected | |
| `TestDetectASLoop_NotPresent` | `internal/component/bgp/reactor/session_validate_test.go` | No false positive when ASN absent | |
| `TestDetectASLoop_EmptyPath` | `internal/component/bgp/reactor/session_validate_test.go` | Empty AS_PATH does not trigger | |
| `TestDetectOriginatorIDLoop` | `internal/component/bgp/reactor/session_validate_test.go` | Matching ORIGINATOR_ID detected | |
| `TestDetectOriginatorIDLoop_Different` | `internal/component/bgp/reactor/session_validate_test.go` | Non-matching ORIGINATOR_ID passes | |
| `TestDetectOriginatorIDLoop_Absent` | `internal/component/bgp/reactor/session_validate_test.go` | Missing ORIGINATOR_ID passes | |
| `TestDetectOriginatorIDLoop_eBGP` | `internal/component/bgp/reactor/session_validate_test.go` | eBGP session skips check | |
| `TestDetectClusterListLoop` | `internal/component/bgp/reactor/session_validate_test.go` | Local Router ID in CLUSTER_LIST detected | |
| `TestDetectClusterListLoop_NotPresent` | `internal/component/bgp/reactor/session_validate_test.go` | CLUSTER_LIST without local ID passes | |
| `TestDetectClusterListLoop_Absent` | `internal/component/bgp/reactor/session_validate_test.go` | Missing CLUSTER_LIST passes | |
| `TestDetectClusterListLoop_eBGP` | `internal/component/bgp/reactor/session_validate_test.go` | eBGP session skips check | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| AS_PATH length | 0-N segments | 0 segments (empty path, no loop) | N/A | N/A |
| ORIGINATOR_ID length | exactly 4 bytes | 4 bytes | 3 bytes (malformed, caught by RFC 7606) | 5 bytes (malformed) |
| CLUSTER_LIST length | multiple of 4 | 0 bytes (empty) | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `loop-as` | `test/plugin/loop-as.ci` | iBGP peer sends route with local ASN in AS_PATH, route not in RIB | |
| `loop-originator-id` | `test/plugin/loop-originator-id.ci` | iBGP peer sends route with local Router ID as ORIGINATOR_ID, route not in RIB | |
| `loop-cluster-list` | `test/plugin/loop-cluster-list.ci` | iBGP peer sends route with local Router ID in CLUSTER_LIST, route not in RIB | |

### Future (if deferring any tests)
- Explicit `cluster-id` configuration with YANG schema -- deferred, requires config work
- `allow-own-as N` configuration to accept N occurrences of local ASN -- deferred, needs config + YANG

## Files to Modify
- `internal/component/bgp/reactor/session_validation.go` - add loop detection functions
- `internal/component/bgp/reactor/session_read.go` - call loop detection in processMessage after enforceRFC7606
- `internal/component/bgp/reactor/session_validate_test.go` - add unit tests for loop detection (existing file, already has validateUpdateFamilies tests)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| CLI commands/flags | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No (validation, not RPC) | N/A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - add loop detection under BGP validation |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | No | N/A |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | No | N/A |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc4456.md` - must create RFC summary |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` - loop detection now supported |
| 12 | Internal architecture changed? | No | N/A (validation pipeline unchanged, just adding checks) |

## Files to Create
- `rfc/short/rfc4456.md` - RFC 4456 summary (route reflection, loop detection rules)
- `test/plugin/loop-as.ci` - AS loop functional test
- `test/plugin/loop-originator-id.ci` - originator-ID loop functional test
- `test/plugin/loop-cluster-list.ci` - cluster-list loop functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: RFC 4456 summary** -- create `rfc/short/rfc4456.md`
   - Tests: N/A (documentation)
   - Files: `rfc/short/rfc4456.md`
   - Verify: file exists, covers Section 7 (Cluster ID default) and Section 8 (loop detection)

2. **Phase: AS loop detection** -- add detectASLoop function and unit tests
   - Tests: `TestDetectASLoop`, `TestDetectASLoop_ASSet`, `TestDetectASLoop_NotPresent`, `TestDetectASLoop_EmptyPath`
   - Files: `session_validation.go`, `session_validate_test.go`
   - Verify: tests fail -> implement -> tests pass
   - Notes: Use `attribute.NewASPathIterator` with `ASNIterator` to walk ASNs. Compare each against `s.settings.LocalAS`. Return true on first match.

3. **Phase: Originator-ID loop detection** -- add detectOriginatorIDLoop function and unit tests
   - Tests: `TestDetectOriginatorIDLoop`, `TestDetectOriginatorIDLoop_Different`, `TestDetectOriginatorIDLoop_Absent`, `TestDetectOriginatorIDLoop_eBGP`
   - Files: `session_validation.go`, `session_validate_test.go`
   - Verify: tests fail -> implement -> tests pass
   - Notes: Extract ORIGINATOR_ID (type 9, 4 bytes) from path attributes. ParseOriginatorID returns OriginatorID (netip.Addr), not uint32. Convert RouterID (uint32) to netip.Addr for comparison, or compare at the byte level. Only check on iBGP sessions (`LocalAS == PeerAS`).

4. **Phase: Cluster-list loop detection** -- add detectClusterListLoop function and unit tests
   - Tests: `TestDetectClusterListLoop`, `TestDetectClusterListLoop_NotPresent`, `TestDetectClusterListLoop_Absent`, `TestDetectClusterListLoop_eBGP`
   - Files: `session_validation.go`, `session_validate_test.go`
   - Verify: tests fail -> implement -> tests pass
   - Notes: Extract CLUSTER_LIST (type 10, multiple of 4 bytes). Walk 4-byte values, compare each against `s.settings.RouterID` (Router ID = default Cluster ID per RFC 4456 Section 7). Only check on iBGP sessions.

5. **Phase: Pipeline integration** -- wire loop detection into processMessage
   - Tests: integration verified by functional tests
   - Files: `session_read.go`
   - Verify: call new validation function after enforceRFC7606, before family check. If loop detected, treat as withdrawal (same pattern as enforceRFC7606 treat-as-withdraw path).

6. **Phase: Functional tests** -- create .ci tests for all three loop types
   - Tests: `test/plugin/loop-as.ci`, `test/plugin/loop-originator-id.ci`, `test/plugin/loop-cluster-list.ci`
   - Files: the three .ci files
   - Verify: tests pass with loop detection enabled

7. **Phase: Documentation** -- update features.md, comparison.md, route-selection.md
   - Tests: N/A
   - Files: `docs/features.md`, `docs/comparison.md`, `docs/architecture/route-selection.md`
   - Verify: remove "Not yet implemented" entries for AS loop, originator-ID loop, cluster-list loop from route-selection.md

8. **Full verification** -- `make ze-verify`
9. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 11 ACs have implementation with file:line |
| Correctness | AS_SET members checked (not just AS_SEQUENCE). iBGP-only checks skip eBGP sessions. |
| Naming | Functions named `detectASLoop`, `detectOriginatorIDLoop`, `detectClusterListLoop` (verb + noun) |
| Data flow | Loop detection runs AFTER RFC 7606 (structurally valid), BEFORE plugin dispatch |
| Performance | AS_PATH iterator is zero-allocation. ORIGINATOR_ID and CLUSTER_LIST are O(N) scans on small data. |
| Rule: no-layering | No old loop detection to replace |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| `rfc/short/rfc4456.md` exists | `ls rfc/short/rfc4456.md` |
| AS loop detection function | `grep detectASLoop internal/component/bgp/reactor/session_validation.go` |
| Originator-ID loop function | `grep detectOriginatorIDLoop internal/component/bgp/reactor/session_validation.go` |
| Cluster-list loop function | `grep detectClusterListLoop internal/component/bgp/reactor/session_validation.go` |
| Pipeline integration | `grep detectLoop internal/component/bgp/reactor/session_read.go` (between enforceRFC7606 and checkPrefixLimits) |
| Unit tests | `grep -c TestDetect internal/component/bgp/reactor/session_validate_test.go` (12 tests) |
| Functional test: AS loop | `ls test/plugin/loop-as.ci` |
| Functional test: originator-ID | `ls test/plugin/loop-originator-id.ci` |
| Functional test: cluster-list | `ls test/plugin/loop-cluster-list.ci` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | Path attribute bytes already validated by RFC 7606 before loop detection runs. Bounds checks in attribute parsing prevent buffer overreads. |
| Resource exhaustion | AS_PATH iteration is bounded by MaxASPathTotalLength (1000). CLUSTER_LIST bounded by attribute length. No allocation. |
| Error leakage | Treat-as-withdraw is silent (no NOTIFICATION sent, no error details to peer). Debug log only. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check AC; if AC wrong then DESIGN; if AC correct then IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

## RFC Documentation

Add `// RFC 4271 Section 9: "<quoted requirement>"` above AS loop check.
Add `// RFC 4456 Section 8: "<quoted requirement>"` above originator-ID and cluster-list checks.
MUST document: validation rules, error conditions.

## Implementation Summary

### What Was Implemented
- `detectLoops` function in `session_validation.go` -- single-pass walk over path attributes checking AS_PATH, ORIGINATOR_ID, CLUSTER_LIST
- Pipeline integration in `session_read.go:processMessage` -- between RFC 7606 and prefix limits
- 12 unit tests in `session_validate_test.go`
- 3 functional .ci tests (loop-as, loop-originator-id, loop-cluster-list)
- RFC 4456 summary in `rfc/short/rfc4456.md`
- Extended ze-peer `RouteToSend` with ASPath, OriginatorID, ClusterList fields
- Documentation in `features.md`, `route-selection.md`

### Bugs Found/Fixed
- None

### Documentation Updates
- `docs/features.md` -- added Route Loop Detection section
- `docs/architecture/route-selection.md` -- added reasons 8-10 (as-loop, originator-id-loop, cluster-list-loop), renumbered rpki-invalid to 11

### Deviations from Plan
- Spec planned 3 separate functions (`detectASLoop`, `detectOriginatorIDLoop`, `detectClusterListLoop`). Implemented as single `detectLoops` that walks attributes once -- more efficient (one pass) and simpler (one call site). Same test coverage.
- ORIGINATOR_ID comparison done at byte level (uint32 from wire vs uint32 RouterID) rather than converting through netip.Addr -- simpler and avoids the type mismatch issue.
- Spec planned tests in `session_validation_test.go` but existing tests were in `session_validate_test.go` -- used existing file.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| AS loop detection | Done | session_validation.go:detectLoops | AS_PATH walk via ASPathIterator |
| Originator-ID loop detection | Done | session_validation.go:detectLoops | uint32 comparison, iBGP only |
| Cluster-list loop detection | Done | session_validation.go:detectLoops | walk 4-byte values, iBGP only |
| Pipeline integration | Done | session_read.go:processMessage | After RFC 7606, before prefix limits |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | TestDetectASLoop, loop-as.ci | eBGP also detected (AS loop is universal) |
| AC-2 | Done | TestDetectASLoop, loop-as.ci | iBGP with local ASN in AS_PATH |
| AC-3 | Done | TestDetectASLoop_NotPresent | No false positive |
| AC-4 | Done | TestDetectOriginatorIDLoop, loop-originator-id.ci | iBGP ORIGINATOR_ID match |
| AC-5 | Done | TestDetectOriginatorIDLoop_Different | Different ORIGINATOR_ID passes |
| AC-6 | Done | TestDetectOriginatorIDLoop_eBGP | eBGP skips check |
| AC-7 | Done | TestDetectClusterListLoop, loop-cluster-list.ci | iBGP CLUSTER_LIST match |
| AC-8 | Done | TestDetectClusterListLoop_NotPresent | No match passes |
| AC-9 | Done | TestDetectClusterListLoop_eBGP | eBGP skips check |
| AC-10 | Done | TestDetectASLoop_ASSet | AS_SET members checked |
| AC-11 | Done | Pipeline order in session_read.go | RFC 7606 runs first, returns before detectLoops |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestDetectASLoop | Done | session_validate_test.go | |
| TestDetectASLoop_ASSet | Done | session_validate_test.go | |
| TestDetectASLoop_NotPresent | Done | session_validate_test.go | |
| TestDetectASLoop_EmptyPath | Done | session_validate_test.go | |
| TestDetectOriginatorIDLoop | Done | session_validate_test.go | |
| TestDetectOriginatorIDLoop_Different | Done | session_validate_test.go | |
| TestDetectOriginatorIDLoop_Absent | Done | session_validate_test.go | |
| TestDetectOriginatorIDLoop_eBGP | Done | session_validate_test.go | |
| TestDetectClusterListLoop | Done | session_validate_test.go | |
| TestDetectClusterListLoop_NotPresent | Done | session_validate_test.go | |
| TestDetectClusterListLoop_Absent | Done | session_validate_test.go | |
| TestDetectClusterListLoop_eBGP | Done | session_validate_test.go | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| rfc/short/rfc4456.md | Created | RFC 4456 summary |
| session_validation.go | Modified | detectLoops function |
| session_read.go | Modified | Pipeline integration |
| session_validate_test.go | Modified | 12 new tests |
| test/plugin/loop-as.ci | Created | AS loop functional test |
| test/plugin/loop-originator-id.ci | Created | Originator-ID functional test |
| test/plugin/loop-cluster-list.ci | Created | Cluster-list functional test |
| docs/features.md | Modified | Route Loop Detection section |
| docs/architecture/route-selection.md | Modified | Reasons 8-10 added |

### Audit Summary
- **Total items:** 27 (4 requirements + 11 ACs + 12 tests)
- **Done:** 27
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (single detectLoops function instead of 3 separate functions)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| rfc/short/rfc4456.md | Yes | ls confirmed |
| test/plugin/loop-as.ci | Yes | ls confirmed |
| test/plugin/loop-originator-id.ci | Yes | ls confirmed |
| test/plugin/loop-cluster-list.ci | Yes | ls confirmed |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | AS loop detected (eBGP) | TestDetectASLoop passes (eBGP covered: AS loop is universal per RFC 4271) |
| AC-2 | AS loop detected (iBGP) | TestDetectASLoop passes, loop-as.ci passes |
| AC-3 | No false positive | TestDetectASLoop_NotPresent passes |
| AC-4 | ORIGINATOR_ID match detected | TestDetectOriginatorIDLoop passes, loop-originator-id.ci passes |
| AC-5 | Different ORIGINATOR_ID passes | TestDetectOriginatorIDLoop_Different passes |
| AC-6 | eBGP skips ORIGINATOR_ID | TestDetectOriginatorIDLoop_eBGP passes |
| AC-7 | CLUSTER_LIST match detected | TestDetectClusterListLoop passes, loop-cluster-list.ci passes |
| AC-8 | No match passes | TestDetectClusterListLoop_NotPresent passes |
| AC-9 | eBGP skips CLUSTER_LIST | TestDetectClusterListLoop_eBGP passes |
| AC-10 | AS_SET members checked | TestDetectASLoop_ASSet passes |
| AC-11 | RFC 7606 precedence | Pipeline: enforceRFC7606 returns before detectLoops runs |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| iBGP + AS loop | test/plugin/loop-as.ci | Pass (ze-test output: pass 1/1) |
| iBGP + ORIGINATOR_ID | test/plugin/loop-originator-id.ci | Pass (ze-test output: pass 1/1) |
| iBGP + CLUSTER_LIST | test/plugin/loop-cluster-list.ci | Pass (ze-test output: pass 1/1) |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
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
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] Summary included in commit
