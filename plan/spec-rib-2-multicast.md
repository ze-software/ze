# Spec: Multicast RIB Support (SAFI 2)

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-03-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. RFC 4760, RFC 2858 - multiprotocol extensions and multicast
4. `internal/component/bgp/message/family.go` - family constants
5. `internal/component/bgp/plugins/rib/` - current RIB implementation

## Task

Ze currently carries multicast NLRI (SAFI 2) at the wire level -- the encoding is identical
to unicast and capability negotiation works. However, Ze does not maintain a separate Multicast
RIB (MRIB) distinct from the unicast RIB, and does not support RPF (Reverse Path Forwarding)
lookups that multicast routing protocols (PIM, IGMP) require.

This spec covers adding proper multicast support:
- Separate MRIB storage per address family (IPv4/IPv6 multicast)
- RPF lookup interface for external multicast daemons
- Best-path selection within the MRIB
- Plugin API for multicast route events

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - RIB architecture
- [ ] `docs/comparison.md` - multicast support across implementations

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4760.md` - Multiprotocol Extensions for BGP-4
- [ ] `rfc/short/rfc2858.md` - Multiprotocol Extensions for BGP-4 (multicast)
- [ ] `rfc/short/rfc7911.md` - ADD-PATH (applies to multicast SAFI too)

**Key insights:**
- To be filled during design phase

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/message/family.go` - defines FamilyIPv4Multicast, FamilyIPv6Multicast constants
- [ ] `internal/component/bgp/plugins/rib/` - current RIB implementation (unicast-oriented)
- [ ] `internal/component/bgp/reactor/` - how families are negotiated and dispatched

**Behavior to preserve:**
- Wire-level multicast NLRI parsing (already works, same encoding as unicast)
- Capability negotiation for SAFI 2 (already works)
- JSON event format for multicast routes

**Behavior to change:**
- Multicast routes currently go into the same storage as unicast (if RIB plugin is loaded)
- No separate MRIB exists
- No RPF lookup interface

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Wire bytes: multicast NLRI in MP_REACH/MP_UNREACH with SAFI 2
- Config: static multicast routes (ipv4/multicast, ipv6/multicast families)

### Transformation Path
1. Wire parsing - same as unicast, distinguished by SAFI in MP attribute
2. Route dispatch to RIB plugin - currently no MRIB separation
3. To be designed: separate MRIB storage and RPF interface

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine -> Plugin | JSON event with family "ipv4/multicast" | [ ] |
| Plugin -> Engine | Text command with multicast family | [ ] |

### Integration Points
- To be identified during design

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| To be designed | -> | To be designed | To be designed |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | To be designed | To be designed |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| To be designed | To be designed | To be designed | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| To be designed | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| To be designed | To be designed | To be designed | |

### Future (if deferring any tests)
- None planned

## Files to Modify
- To be identified during design phase

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] | To be determined |
| RPC count in architecture docs | [ ] | |
| CLI commands/flags | [ ] | |
| CLI usage/help text | [ ] | |
| API commands doc | [ ] | |
| Plugin SDK docs | [ ] | |
| Editor autocomplete | [ ] | |
| Functional test for new RPC/API | [ ] | |

## Files to Create
- To be identified during design phase

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
<!-- Every Yes MUST name the file and what to add/change. -->
<!-- See planning.md "Documentation Update Checklist" for the full table with examples. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | [ ] | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | [ ] | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | [ ] | `ai/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented? | [ ] | `rfc/short/rfcNNNN.md` |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` or subsystem doc |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-verify` |
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

1. **Phase: Design** -- identify MRIB storage approach and RPF interface
2. **Phase: MRIB storage** -- separate multicast RIB in bgp-rib plugin
3. **Phase: RPF interface** -- lookup API for external multicast daemons
4. **Phase: Functional tests** -- .ci tests proving multicast routes stored separately
5. **Phase: Full verification** -- `make ze-verify`
6. **Phase: Complete spec** -- audit, learned summary, commit

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | MRIB separate from unicast RIB |
| Data flow | Multicast routes dispatched to MRIB, not unicast RIB |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| To be designed | To be designed |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | Multicast NLRI parsing bounds checks |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check AC |
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

Add `// RFC NNNN Section X.Y` above enforcing code.

## Implementation Summary

### What Was Implemented
- To be filled

### Bugs Found/Fixed
- To be filled

### Documentation Updates
- To be filled

### Deviations from Plan
- To be filled

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
- [ ] AC-1..AC-N all demonstrated
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
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] Summary included in commit
