# Spec: fib-depth-4-srv6 -- SRv6 Encapsulation

| Field | Value |
|-------|-------|
| Status | blocked |
| Depends | spec-fib-depth, bgp-nlri-srv6 |
| Phase | - |
| Updated | 2026-05-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `internal/plugins/sysrib/events/events.go` -- SRv6SID field on BestChangeEntry
3. `internal/plugins/fib/kernel/nexthop_linux.go` -- placeholder for seg6 encap
4. `internal/plugins/fib/vpp/mpls.go` -- pattern for encap backend extension

## Task

Implement SRv6 encapsulation in both kernel (Linux seg6) and VPP (SR policy)
FIB backends. The SRv6SID field already exists on BestChangeEntry. This spec
implements the backend programming that consumes it.

**Blocked on:** bgp-nlri-srv6 family must be fully wired so that SRv6 SIDs
flow through the system to reach the FIB layer.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- FIB plugin pattern

### RFC Summaries
- [ ] `rfc/short/rfc8986.md` -- SRv6 Network Programming (if exists)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/sysrib/events/events.go` -- `SRv6SID netip.Addr` field exists, omitzero
- [ ] `internal/plugins/fib/kernel/nexthop_linux.go` -- no seg6 encap code yet
- [ ] `internal/plugins/fib/vpp/mpls.go` -- MPLS encap pattern (model for SRv6)

**Behavior to preserve:**
- Existing MPLS encap path unchanged
- SRv6SID field with omitzero preserves backwards compatibility

**Behavior to change:**
- Kernel backend: when SRv6SID is valid, set route.Encap to SEG6 encap
- VPP backend: when SRv6SID is valid, program SR policy via VPP SR steer API

## Data Flow (MANDATORY)

### Entry Point
- BGP UPDATE with SRv6 Prefix SID attribute (bgp-prefix-sid-srv6)

### Transformation Path
1. bgp-nlri-srv6 parses SRv6 SID from UPDATE
2. RIB stores route with SRv6 SID
3. sysrib emits BestChangeEntry with SRv6SID field populated
4. FIB backend programs encapsulation

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| sysrib -> fib-kernel | EventBus with SRv6SID | [ ] |
| fib-kernel -> Linux | netlink SEG6 encap | [ ] |
| fib-vpp -> VPP | GoVPP SR steer API | [ ] |

### Integration Points
- `netlink.SEG6Encap` type for Linux seg6 lwtunnel
- VPP `sr_steer_add_del` binary API message

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| BestChangeEntry with SRv6SID | -> | kernel seg6 encap | `TestKernelSRv6Encap` |
| BestChangeEntry with SRv6SID | -> | VPP SR steer | `TestVPPSRv6Steer` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | BestChangeEntry with valid SRv6SID (kernel) | Route installed with SEG6 encap pointing to SID |
| AC-2 | BestChangeEntry with valid SRv6SID (VPP) | SR steer policy created |
| AC-3 | SRv6 route withdrawn | Encap/policy removed |
| AC-4 | SRv6SID zero (not set) | No encap added (plain route) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestKernelSRv6Encap` | `internal/plugins/fib/kernel/fibkernel_test.go` | SEG6 encap built correctly | |
| `TestVPPSRv6Steer` | `internal/plugins/fib/vpp/fibvpp_test.go` | SR steer API called | |
| `TestSRv6Withdraw` | `internal/plugins/fib/kernel/fibkernel_test.go` | Encap removed on withdraw | |
| `TestSRv6NotSetSkipped` | `internal/plugins/fib/kernel/fibkernel_test.go` | Zero SID means no encap | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-fib-srv6` | `test/plugin/fib-srv6.ci` | SRv6 route installed with encap | |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `srv6-frr` | `test/interop/scenarios/srv6-frr/` | FRR | SRv6 SID exchanged and installed | |

## Files to Modify
- `internal/plugins/fib/kernel/nexthop_linux.go` -- add seg6 encap in buildRichRoute
- `internal/plugins/fib/kernel/fibkernel.go` -- detect SRv6SID in hasRichFields
- `internal/plugins/fib/vpp/fibvpp.go` -- dispatch to SRv6 backend

## Files to Create
- `internal/plugins/fib/vpp/srv6.go` -- VPP SR steer API wrapper
- `test/plugin/fib-srv6.ci` -- functional test

## Implementation Steps

### Implementation Phases

1. **Phase: Kernel seg6 encap** -- build SEG6Encap when SRv6SID is valid
   - Tests: TestKernelSRv6Encap, TestSRv6NotSetSkipped
   - Files: nexthop_linux.go, fibkernel.go
   - Verify: unit tests pass

2. **Phase: VPP SR steer** -- call sr_steer_add_del via GoVPP
   - Tests: TestVPPSRv6Steer
   - Files: srv6.go, fibvpp.go
   - Verify: mock records SR steer call

3. **Phase: Withdrawal** -- remove encap/policy on withdraw
   - Tests: TestSRv6Withdraw
   - Verify: clean removal

4. **Phase: Functional + interop tests**
   - Tests: fib-srv6.ci, srv6-frr scenario

### Critical Review Checklist
| Check | What to verify |
|-------|---------------|
| Completeness | All 4 ACs implemented |
| SID validation | Only valid (non-zero) SIDs trigger encap |
| Withdrawal | Encap removed cleanly, no orphans |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | SRv6SID is netip.Addr (already validated by type); check Is6() |
| Resource | One encap per route, bounded by prefix count |

## Implementation Summary

### What Was Implemented
- [pending]

## Implementation Audit

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

## Review Gate

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features
