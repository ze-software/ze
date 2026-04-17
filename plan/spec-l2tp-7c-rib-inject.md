# Spec: l2tp-7c -- L2TP Redistribute RIB injection

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-l2tp-7-subsystem |
| Phase | - |
| Updated | 2026-04-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-l2tp-7-subsystem.md` -- parent spec
3. `plan/deferrals.md` 2026-04-17 row pointing here
4. `internal/component/l2tp/route_observer.go` -- current observer
5. `internal/component/bgp/plugins/rib/rib.go` -- `bgp rib inject` handler

## Task

Wire the L2TP RouteObserver's in-memory subscriber-route tracking into
the protocol RIB so that a session reaching `Established` with an
assigned peer IP produces an actual BGP UPDATE (or equivalent protocol
injection) carrying the `/32` (IPv4) or `/128` (IPv6) with source
`l2tp`. Today the observer records the lifecycle in memory and logs;
the RIB side of the wire is a no-op because no programmatic inject path
exists outside the `bgp rib inject` CLI command.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- redistribute registry
- [ ] `internal/component/bgp/plugins/rib/rib.go` -- `InjectRoute` + `handleCommand` entry
- [ ] `internal/component/bgp/redistribute/` -- BGP source registration precedent

### RFC Summaries (MUST for protocol work)
- [ ] RFC 4271 -- BGP UPDATE message

**Key insights:** (filled during RESEARCH phase)

## Current Behavior (MANDATORY)

**Source files read:** (to be filled during RESEARCH phase)

**Behavior to preserve:** existing `bgp rib inject` CLI continues working.

**Behavior to change:** add a programmatic inject path callable from
non-BGP subsystems (L2TP first; future connected / static / OSPF likely
consumers).

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- L2TP RouteObserver.OnSessionIPUp / OnSessionDown

### Transformation Path
1. Observer hands (session-id, username, addr, family) to an
   injector interface.
2. Injector converts to the RIB's inject shape (peer=`l2tp`,
   family=`ipv4/unicast` or `ipv6/unicast`, prefix=`addr/32` or
   `addr/128`, source=`l2tp`, origin=`incomplete`, nhop=self).
3. RIB accepts the inject; routes flow out to configured BGP peers
   according to redistribute import rules.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| L2TP -> RIB | A programmatic inject API (new) | Functional test in spec-l2tp-7b |

### Integration Points
- `bgp rib inject` becomes one caller of the new injector
- L2TP subscribes a second caller

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Session IPCP completes on LNS | -> | L2TP observer injects `/32` via new injector | `test/plugin/redistribute-inject.ci` |
| Session torn down | -> | L2TP observer withdraws the `/32` | `test/plugin/redistribute-withdraw.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | IPCP completes on a session with `redistribute l2tp` configured | `/32` appears in protocol RIB with source=`l2tp` |
| AC-2 | Session ends (CDN or StopCCN) | `/32` removed from RIB |
| AC-3 | Injector called with an IPv6 address | `/128` with correct family |
| AC-4 | No redistribute rule configured | Inject is a no-op (source registration present, routes not advertised) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TBD | `internal/component/bgp/plugins/rib/injector_test.go` | Programmatic inject from external caller | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| n/a | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| redistribute-inject | `test/plugin/redistribute-inject.ci` | Subscriber connects, /32 in RIB | |
| redistribute-withdraw | `test/plugin/redistribute-withdraw.ci` | Subscriber leaves, /32 gone | |

### Future (if deferring any tests)
- None

## Files to Modify
- `internal/component/bgp/plugins/rib/rib.go` -- expose programmatic `InjectRoute` + `WithdrawRoute`
- `internal/component/l2tp/route_observer.go` -- call the injector interface
- `internal/component/l2tp/subsystem.go` -- plumb the injector handle through Start

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| RIB injector API | [ ] | `internal/component/bgp/plugins/rib/` |
| L2TP wiring | [ ] | `internal/component/l2tp/route_observer.go` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | |
| 3 | CLI command added/changed? | [ ] | |
| 4 | API/RPC added/changed? | [ ] | |
| 5 | Plugin added/changed? | [ ] | |
| 6 | Has a user guide page? | [ ] | `docs/guide/l2tp.md` |
| 7 | Wire format changed? | [ ] | |
| 8 | Plugin SDK/protocol changed? | [ ] | |
| 9 | RFC behavior implemented? | [ ] | |
| 10 | Test infrastructure changed? | [ ] | |
| 11 | Affects daemon comparison? | [ ] | |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/l2tp.md` |

## Files to Create
- `internal/component/bgp/plugins/rib/injector.go`
- `internal/component/bgp/plugins/rib/injector_test.go`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create |
| 3. Implement (TDD) | RIB injector first, then L2TP wiring |
| 4. Full verification | `make ze-verify-fast` |
| 5-12 | Standard flow |

### Implementation Phases
1. Factor the existing `bgp rib inject` handler into a programmatic API.
2. Wire L2TP RouteObserver through that API.
3. Land functional tests.

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation |
| Correctness | Refactor does not change `bgp rib inject` behavior |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| Programmatic injector API | `grep -n 'func.*InjectRoute' internal/component/bgp/plugins/rib/` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | Addresses validated before inject |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
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

RFC 4271 S4.3 (UPDATE), RFC 5492 (capabilities) cited in inject code.

## Implementation Summary

### What Was Implemented
- (to be filled)

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
- [ ] AC-N all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete

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
- [ ] Write learned summary
- [ ] Summary included in commit
