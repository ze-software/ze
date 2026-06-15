# Spec: srv6-labeled-unicast -- SRv6 Support for Labeled Unicast (SAFIMPLSLabel)

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
3. `rfc/short/rfc9252.md` - SRv6 service TLV scope
4. `internal/component/bgp/plugins/rib/rib_bestchange.go` - SRv6 SID lookup gated on SAFI

## Task

Ze skips SRv6 SID lookup for SAFIMPLSLabel (labeled unicast, SAFI 4). The
current code in `checkBestPathChange` explicitly excludes it:

```go
if fam.SAFI != family.SAFIMPLSLabel {
    srv6SID = r.lookupSRv6SIDForBest(fam, nlriBytes, newBest.PeerIP)
}
```

RFC 9252 scope includes "IPv6 unicast with labels" alongside VPN and EVPN.
Supporting SRv6 with labeled unicast is uncommon in deployments but is part
of the RFC scope.

### Design questions

1. Is there real-world demand for SAFIMPLSLabel + SRv6? (IPv6 labeled unicast
   with SRv6 SIDs is unusual; most deployments use VPN or EVPN.)
2. What label width applies? MPLS labels are 20 bits regardless of SAFI.
3. Does the existing transposition logic need changes, or just removing the
   SAFI gate?

### Key source files

- `internal/component/bgp/plugins/rib/rib_bestchange.go:716-721` - SAFI gate
- `internal/component/bgp/plugins/rib/rib_bestchange.go:887-889` - needsTransposition
- `internal/component/bgp/plugins/rib/pool/srv6sid.go` - extraction and transposition

## Required Reading

### Architecture Docs
- [ ] `plan/learned/776-srv6-prefix-sid.md` - original SRv6 design decisions
- [ ] `plan/learned/906-srv6-review-fixes.md` - recent fixes

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9252.md` - scope includes labeled unicast
- [ ] `rfc/short/rfc8669.md` - Prefix-SID attribute for labeled unicast

**Key insights:**
- (fill during design)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` - SRv6 SID lookup skipped for SAFIMPLSLabel

**Behavior to preserve:**
- SRv6 SID handling for VPN and EVPN SAFIs
- Label lookup for SAFIMPLSLabel (existing, separate from SRv6)

**Behavior to change:**
- Enable SRv6 SID lookup for SAFIMPLSLabel
- Determine if transposition applies (likely yes, same as VPN)

## Data Flow (MANDATORY)

### Entry Point
- Best-path change for labeled unicast prefix with PrefixSID attribute

### Transformation Path
1. UPDATE with PrefixSID attribute arrives for labeled unicast family
2. Currently: SRv6 SID lookup skipped due to SAFI gate
3. Desired: SRv6 SID extracted, transposition applied if SID Structure present
4. SID passed through to sysrib for FIB installation

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| RIB best-path -> sysrib | bestChangeEntry.SRv6SID via EventBus | [ ] |

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| (fill during design) | | | |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Labeled unicast route with SRv6 Prefix-SID | SRv6 SID extracted and passed to sysrib |
| AC-2 | Labeled unicast route with SRv6 SID + SID Structure | Transposition applied correctly |
| AC-3 | Labeled unicast route without Prefix-SID | No change in behavior |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (fill during design) | | | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| (fill during design) | | | |

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
| 1 | New user-facing feature? | Yes | `docs/features/srv6.md` |

## Files to Create
- (fill during design)

## Implementation Steps

(fill during design)

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
- [ ] Write learned summary to `plan/learned/NNN-srv6-labeled-unicast.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-srv6-labeled-unicast.md`
