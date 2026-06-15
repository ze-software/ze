# Spec: srv6-ebgp-egress-filter -- Suppress Prefix-SID on EBGP Egress

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
3. `rfc/short/rfc8669.md` - Section 4/5/8: EBGP propagation rules
4. `internal/component/bgp/reactor/` - egress UPDATE building

## Task

RFC 8669 Section 4: "A BGP speaker receiving a BGP Prefix-SID attribute from an
External BGP (EBGP) neighbor residing outside the boundaries of the SR domain
MUST discard the attribute unless it is configured to accept the attribute from
the EBGP neighbor."

Ze implements the ingress side (`accept-srv6-prefix-sid` config option). But on
egress, Ze does not suppress Prefix-SID when advertising to EBGP peers outside
the SR domain.

RFC 8669 Section 8: "The propagation to other ASes MUST be explicitly configured."
This is a SHOULD-level concern (Section 5 says "SHOULD NOT advertise...outside an
AS unless explicitly configured"), but proper domain boundary enforcement needs an
egress policy knob.

### Design questions

1. Should the default be "strip Prefix-SID on EBGP egress" (safe default) or
   "propagate" (current behavior, requires explicit suppression)?
2. Config surface: a per-peer boolean under `session` (like `accept-srv6-prefix-sid`)
   or a filter action in the export policy chain?
3. Should this also cover SRv6 TLV types 5/6 specifically, or all Prefix-SID TLVs?

### Key source files

- `internal/component/bgp/reactor/` - egress UPDATE path
- `internal/component/bgp/config/resolve.go` - peer config resolution
- `internal/component/bgp/plugins/` - export filter plugins

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component isolation
- [ ] `plan/learned/776-srv6-prefix-sid.md` - SRv6 design decisions

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc8669.md` - Section 4/5/8: EBGP propagation rules
  -> Constraint: "propagation to other ASes MUST be explicitly configured"
  -> Constraint: SHOULD NOT advertise outside AS unless explicitly configured
- [ ] `rfc/short/rfc9252.md` - Section 3.3: propagation rules for SRv6 TLVs

**Key insights:**
- (fill during design)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/config/resolve.go` - `AcceptSRv6PrefixSID` ingress filter
- [ ] `internal/component/bgp/reactor/` - egress path, no Prefix-SID suppression

**Behavior to preserve:**
- Ingress filtering via `accept-srv6-prefix-sid`
- iBGP propagation of Prefix-SID (within SR domain)

**Behavior to change:**
- Add egress suppression of Prefix-SID on EBGP sessions by default
- Add config knob to explicitly enable EBGP egress propagation

## Data Flow (MANDATORY)

### Entry Point
- Egress UPDATE building in reactor for EBGP peers

### Transformation Path
1. Best-path selected, route has PrefixSID attribute
2. Egress UPDATE built for each peer
3. Gap: no check for EBGP + Prefix-SID suppression
4. UPDATE sent with Prefix-SID to EBGP peer

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> reactor | peer session config read at UPDATE build time | [ ] |
| RIB -> egress | attribute bytes forwarded or modified | [ ] |

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
| AC-1 | Route with Prefix-SID, EBGP peer, no explicit propagation config | Prefix-SID attribute stripped from egress UPDATE |
| AC-2 | Route with Prefix-SID, EBGP peer, propagation explicitly enabled | Prefix-SID attribute included in egress UPDATE |
| AC-3 | Route with Prefix-SID, iBGP peer | Prefix-SID attribute included (unchanged behavior) |

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
| YANG schema | Yes | new leaf for egress propagation config |
| CLI commands/flags | No | |
| Functional test | Yes | test/encode/ or test/plugin/ |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features/srv6.md` |
| 2 | Config syntax changed? | Yes | config docs for new leaf |

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
- [ ] Write learned summary to `plan/learned/NNN-srv6-ebgp-egress-filter.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-srv6-ebgp-egress-filter.md`
