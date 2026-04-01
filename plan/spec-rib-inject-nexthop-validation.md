# Spec: rib-inject-nexthop-validation

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-rib-inject |
| Phase | - |
| Updated | 2026-04-01 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/bgp/context/context.go` - EncodingContext, ExtendedNextHopFor
4. `internal/component/bgp/plugins/rib/rib_commands.go` - current injectRoute

## Task

Replace the hardcoded IPv4-only next-hop validation in `rib inject` with capability-aware validation. Use the peer's negotiated OPEN capabilities (ExtendedNextHop, RFC 8950) to decide what next-hop formats are valid. For injected routes where no session exists, use a configurable default.

## Required Reading

### Architecture Docs
- [ ] `internal/component/bgp/context/context.go` - EncodingContext, ExtendedNextHopFor
  -> Constraint: Returns 0 if extended next-hop not negotiated for the family
  -> Constraint: Uses `c.encoding.ExtendedNextHop[f]` map lookup
- [ ] `internal/component/bgp/context/registry.go` - ContextRegistry, global Registry
  -> Constraint: Registry.Get(id) returns nil if ID not registered
  -> Decision: Registry keyed by ContextID, not peer address
- [ ] `internal/component/bgp/capability/encoding.go` - EncodingCaps.ExtendedNextHop
  -> Constraint: map[Family]AFI, key is NLRI family, value is next-hop AFI
- [ ] `internal/component/bgp/capability/negotiated.go` - ExtendedNextHop negotiation
  -> Constraint: Tuple negotiated only if both peers advertise same (NLRI_AFI, NLRI_SAFI, NH_AFI)
- [ ] `internal/component/bgp/reactor/peer_static_routes.go` - existing capability check pattern
  -> Constraint: sendCtx.ExtendedNextHopFor(family) != 0 gates UseExtendedNextHop flag

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc5549.md` - RFC 8950 (obsoletes 5549) extended next-hop encoding
  -> Constraint: MUST only advertise IPv4 NLRI with IPv6 NH if capability negotiated (Section 4)

**Key insights:**
- EncodingContext.ExtendedNextHopFor(family) is the single query point for capability check
- ContextRegistry is keyed by ContextID, not peer address
- RIB plugin receives ContextID via WireUpdate.SourceCtxID() in structured events
- RIB plugin's peerMeta stores PeerASN/LocalASN but not ContextID

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `rib_commands.go:233-241` - nhop validation: net.ParseIP, To4, reject if nil
- [ ] `context.go:190-198` - ExtendedNextHopFor returns AFI for family, 0 if not negotiated
- [ ] `registry.go:74-78` - Registry.Get(id) returns EncodingContext or nil
- [ ] `peer_static_routes.go:68-75` - checks sendCtx.ExtendedNextHopFor before UseExtendedNextHop
- [ ] `rib_structured.go:62` - already reads SourceCtxID: `ctx := bgpctx.Registry.Get(wu.SourceCtxID())`

**Behavior to preserve:**
- IPv4 next-hop always valid for any peer
- IPv4-mapped IPv6 (::ffff:x.x.x.x) treated as IPv4
- Invalid IP strings rejected
- All other inject validation unchanged

**Behavior to change:**
- IPv6 next-hop currently always rejected
- No per-peer capability lookup for real peers

## Data Flow (MANDATORY)

### Entry Point
- `rib inject <peer> <family> <prefix> nhop <ip>` arrives at injectRoute

### Transformation Path
1. Parse IP via net.ParseIP
2. To4() non-nil -> accept (IPv4 or mapped), done
3. To4() nil (real IPv6) -> capability lookup:
   a. Check peerMeta[peer].ContextID (populated from structured events for real peers)
   b. If ContextID exists: bgpctx.Registry.Get(id).ExtendedNextHopFor(family) != 0 -> accept
   c. If no ContextID (injected peer, no session): check for `extended-nexthop` flag -> accept/reject
4. Build attribute: if IPv6 accepted, store in MP_REACH_NLRI (future), for now store raw in Builder

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| RIB plugin -> bgpctx.Registry | Read-only Get(ContextID) | [ ] |
| Structured event -> peerMeta | ContextID stored on first event | [ ] |

### Integration Points
- `bgpctx.Registry.Get(ContextID)` - existing global registry, read-only
- `peerMeta` map in RIBManager - add ContextID field
- `rib_structured.go` handleReceivedStructured - already reads SourceCtxID

### Architectural Verification
- [ ] No bypassed layers (uses existing context registry)
- [ ] No unintended coupling (read-only access to registry)
- [ ] No duplicated functionality (reuses ExtendedNextHopFor)
- [ ] Zero-copy preserved (no new allocations in hot path)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `rib inject ... nhop <ipv6>` with real peer context | -> | injectRoute capability lookup | TestInjectRoute_IPv6NhopWithExtendedCapability |
| `rib inject ... nhop <ipv6>` without capability | -> | injectRoute rejects | TestInjectRoute_IPv6NhopWithoutExtendedCapability |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | IPv4 nhop, any peer | Accepted |
| AC-2 | IPv4-mapped IPv6 nhop (::ffff:x.x.x.x) | Accepted (treated as IPv4) |
| AC-3 | IPv6 nhop, real peer with ExtendedNextHop negotiated for family | Accepted |
| AC-4 | IPv6 nhop, real peer without ExtendedNextHop for family | Rejected with error naming the missing capability |
| AC-5 | IPv6 nhop, unknown peer (no session seen) | Accepted (fallback: any valid IP) |
| AC-6 | Invalid IP string, any peer | Rejected |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestInjectRoute_IPv4NhopAlwaysValid` | `rib_test.go` | AC-1 | existing (TestInjectRoute_AllAttributes) |
| `TestInjectRoute_IPv4MappedAlwaysValid` | `rib_test.go` | AC-2 | existing (TestInjectRoute_IPv4MappedIPv6NextHop) |
| `TestInjectRoute_IPv6NhopWithExtendedCapability` | `rib_test.go` | AC-3 | |
| `TestInjectRoute_IPv6NhopWithoutExtendedCapability` | `rib_test.go` | AC-4 | |
| `TestInjectRoute_IPv6NhopInjectedPeerDefault` | `rib_test.go` | AC-5 | existing (TestInjectRoute_IPv6NextHopRejected) |
| `TestInjectRoute_IPv6NhopInjectedPeerOverride` | `rib_test.go` | AC-6 | |
| `TestInjectRoute_ErrorMessageNamesCapability` | `rib_test.go` | AC-7 | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A - no new numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Covered by existing api-rib-inject.ci | test/plugin/api-rib-inject.ci | IPv4 inject path | existing |

### Future
- Functional test with real peer + extended-nexthop capability negotiated (requires full session setup in .ci)

## Files to Modify
- `internal/component/bgp/plugins/rib/rib_commands.go` - replace IPv4-only nhop check with capability-aware validation
- `internal/component/bgp/plugins/rib/rib.go` - add ContextID to PeerMeta struct
- `internal/component/bgp/plugins/rib/rib_structured.go` - store ContextID in peerMeta on first event
- `internal/component/bgp/plugins/rib/rib_test.go` - new tests for AC-3, AC-4, AC-6, AC-7

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | |
| CLI commands/flags | Yes | `extended-nexthop` flag added to inject syntax |
| Editor autocomplete | No | YANG unchanged |
| Functional test for new RPC/API | No | existing .ci covers IPv4 path |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` - add `extended-nexthop` flag |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` - update inject attrs |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | Yes | Verify `rfc/short/rfc8950.md` exists |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |

## Files to Create
- None (modifying existing files only)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan |
| 3. Implement (TDD) | Phases below |
| 4. Full verification | `make ze-verify` |
| 5. Critical review | Critical Review Checklist |
| 6. Fix issues | Fix findings |
| 7. Re-verify | `make ze-verify` |
| 8. Repeat 5-7 | Max 2 passes |
| 9. Deliverables review | Deliverables Checklist |
| 10. Security review | Security Review Checklist |
| 11. Re-verify | `make ze-verify` |
| 12. Present summary | Executive Summary |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Store ContextID in peerMeta** -- add ContextID field to PeerMeta, store from SourceCtxID in handleReceivedStructured
   - Tests: existing structured event tests should still pass
   - Files: `rib.go` (PeerMeta struct), `rib_structured.go` (store ContextID)
   - Verify: `go test -run TestHandleReceived`

2. **Phase: Capability-aware nhop validation** -- replace To4-only check with Registry lookup
   - Tests: TestInjectRoute_IPv6NhopWithExtendedCapability, TestInjectRoute_IPv6NhopWithoutExtendedCapability, TestInjectRoute_ErrorMessageNamesCapability
   - Files: `rib_commands.go` (injectRoute nhop block)
   - Verify: new tests fail -> implement -> pass

3. **Phase: Override flag for injected peers** -- add `extended-nexthop` keyword to inject args
   - Tests: TestInjectRoute_IPv6NhopInjectedPeerOverride
   - Files: `rib_commands.go` (attr parsing loop + knownAttrs)
   - Verify: test fails -> implement -> pass

4. **Phase: Docs** -- update command-reference.md and api/commands.md
   - Files: docs
   - Verify: content matches implementation

5. **Full verification** -- `make ze-verify`

6. **Complete spec** -- fill audit, write learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1 through AC-7 all have tests |
| Correctness | ContextID stored correctly from structured events, not stale after session reset |
| Naming | `extended-nexthop` flag matches RFC 8950 terminology |
| Data flow | Read-only access to bgpctx.Registry, no writes from RIB plugin |
| Rule: design-principles | No premature abstraction - simple if/else, not interface |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| ContextID in PeerMeta | `grep ContextID rib.go` |
| ContextID stored from events | `grep ContextID rib_structured.go` |
| Capability lookup in injectRoute | `grep ExtendedNextHopFor rib_commands.go` |
| Override flag parsed | `grep extended-nexthop rib_commands.go` |
| 4 new tests pass | `go test -run IPv6Nhop` |
| Docs updated | `grep extended-nexthop docs/guide/command-reference.md` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | `extended-nexthop` flag is boolean (no value), can't inject data |
| Registry access | Read-only, no mutation of global state |
| Stale context | ContextID from a reset session could point to wrong capabilities |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Design Decisions Needed

| # | Question | Status |
|---|----------|--------|
| 1 | How does RIB plugin get per-peer capability? | Proposed: store ContextID in peerMeta from SourceCtxID in structured events |
| 2 | Override syntax for injected peers? | Proposed: `extended-nexthop` boolean flag in attr args |
| 3 | Stale ContextID after session reset? | Open: ContextID may reference old capabilities after session bounce |

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

Add `// RFC 8950 Section 4` above the capability check in injectRoute.

## Implementation Summary

### What Was Implemented

### Bugs Found/Fixed

### Documentation Updates

### Deviations from Plan

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
- **Partial:**
- **Skipped:**
- **Changed:**

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
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven
- [ ] Docs updated
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
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-rib-inject-nexthop-validation.md`
- [ ] Summary included in commit
