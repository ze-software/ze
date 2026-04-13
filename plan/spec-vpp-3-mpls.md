# Spec: vpp-3-mpls — MPLS Label Operations

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-vpp-2-fib |
| Phase | - |
| Updated | 2026-04-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `plan/spec-vpp-0-umbrella.md` — parent spec
4. `plan/spec-vpp-2-fib.md` — fib-vpp plugin this extends
5. `internal/plugins/fibvpp/` — files from vpp-2

## Task

Extend fib-vpp to program MPLS label push/swap/pop in VPP's FIB based on BGP next-hop labels
(RFC 3107, RFC 8277). Ze's BGP implementation already parses MPLS labels from UPDATE messages.
This spec wires those labels through sysRIB events into GoVPP MPLS API calls.

This is what differentiates ze from IPng (which uses FRR LDP for MPLS) and VyOS (which does
not expose MPLS through VPP config). Direct label programming from BGP to VPP FIB is a
unique capability.

### Reference

- RFC 3107: Carrying Label Information in BGP-4
- RFC 8277: Using BGP to Bind MPLS Labels to Address Prefixes
- IPng.ch blog: MPLS label stack operations in VPP
- GoVPP mpls binapi: MplsRouteAddDel, MplsTableAddDel

## Required Reading

### Architecture Docs
- [ ] `internal/plugins/fibvpp/` — fib-vpp plugin from spec-vpp-2
  → Constraint: MPLS extends existing backend interface and event processing
- [ ] `internal/plugins/bgp-nlri-labeled/` — BGP labeled unicast NLRI parser
  → Constraint: ze already parses MPLS labels from BGP UPDATE messages
  → Decision: labels flow through sysRIB events, not separate channel
- [ ] `docs/architecture/core-design.md` — event payload format
  → Constraint: sysRIB event payload needs labels field extension

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc3107.md` — Carrying Label Information in BGP-4
  → Constraint: label encoded in NLRI, 20-bit label value, bottom-of-stack bit
- [ ] `rfc/short/rfc8277.md` — Using BGP to Bind MPLS Labels to Address Prefixes
  → Constraint: label binding procedures, withdraw semantics, label stack encoding

**Key insights:**
- Ze already parses MPLS labels from BGP UPDATE NLRI (labeled unicast family)
- Labels need to flow through sysRIB best-change events to reach fibvpp
- Three MPLS operations: push (PE ingress), swap (P transit), pop (PE egress)
- VPP MPLS uses IPRouteAddDel with label stack in FibPath for push, MplsRouteAddDel for swap/pop
- MPLS must be enabled per-interface in VPP before label operations work

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/fibvpp/fibvpp.go` — fib-vpp event processing (from spec-vpp-2)
  → Constraint: extend processEvent to handle labels field in payload
- [ ] `internal/plugins/fibvpp/backend.go` — vppBackend interface (from spec-vpp-2)
  → Constraint: extend with MPLS methods: addMPLSRoute, delMPLSRoute, enableMPLS, disableMPLS
- [ ] `internal/plugins/bgp-nlri-labeled/` — labeled unicast NLRI parsing
  → Constraint: labels already extracted from BGP UPDATE wire format
- [ ] sysRIB event payload format — current format has action, prefix, next-hop
  → Constraint: needs labels field added for MPLS label stack

**Behavior to preserve:**
- fib-vpp IPv4/IPv6 route programming unchanged (no labels = same behavior)
- sysRIB event format backward compatible (labels field optional)
- BGP labeled unicast parsing unchanged

**Behavior to change:**
- sysRIB best-change event payload gains optional `labels` field (array of uint32)
- fib-vpp backend gains MPLS methods
- fib-vpp processEvent checks for labels and dispatches to MPLS backend methods

## Data Flow (MANDATORY)

### Entry Point
- BGP reactor receives UPDATE with labeled unicast NLRI containing MPLS label stack
- sysRIB selects best route, emits (system-rib, best-change) event with labels in payload

### Transformation Path
1. BGP UPDATE parsed, NLRI contains prefix + label stack (RFC 3107/8277 encoding)
2. Label stack stored with route in protocol RIB
3. sysRIB selects best route, emits event with labels field in JSON payload
4. fibvpp processEvent detects labels in payload
5. If labels present:
   - PE ingress (push): call GoVPP IPRouteAddDel with label stack in FibPath.LabelStack
   - P transit (swap): call GoVPP MplsRouteAddDel with in-label, out-label, next-hop
   - PE egress (pop): call GoVPP MplsRouteAddDel with in-label, pop action
6. If no labels: standard IP route programming (existing behavior)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| BGP NLRI → protocol RIB | Label stack extracted during NLRI parsing | [ ] |
| protocol RIB → sysRIB | Best route selection includes label info | [ ] |
| sysRIB → fibvpp | JSON payload with optional labels array | [ ] |
| fibvpp → GoVPP | IPRouteAddDel (push) or MplsRouteAddDel (swap/pop) | [ ] |

### Integration Points
- `internal/plugins/bgp-nlri-labeled/` — source of label information
- `internal/plugins/fibvpp/` — extended backend and event processing
- sysRIB event payload — labels field added
- GoVPP mpls binapi — MplsRouteAddDel, MplsTableAddDel

### Architectural Verification
- [ ] No bypassed layers (labels flow through sysRIB events, not direct BGP-to-VPP)
- [ ] No unintended coupling (label info carried in existing event payload, not separate channel)
- [ ] No duplicated functionality (extends existing fibvpp, not parallel implementation)
- [ ] Zero-copy preserved where applicable (label stack is small fixed array)

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| BGP labeled unicast UPDATE → sysRIB event with labels | → | fibvpp MPLS push via IPRouteAddDel with LabelStack | `test/vpp/005-mpls-push.ci` |
| sysRIB event with labels, transit role | → | fibvpp MPLS swap via MplsRouteAddDel | `test/vpp/005-mpls-push.ci` |
| sysRIB event with labels, withdraw | → | fibvpp MPLS route deletion | `test/vpp/005-mpls-push.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | BGP labeled unicast prefix with label stack received | VPP FIB has IP route with MPLS label push (LabelStack populated in FibPath) |
| AC-2 | MPLS transit: in-label mapped to out-label + next-hop | VPP MPLS FIB has swap entry (MplsRouteAddDel) |
| AC-3 | MPLS egress: in-label with pop action | VPP MPLS FIB has pop entry |
| AC-4 | BGP withdraws labeled unicast prefix | VPP MPLS route removed |
| AC-5 | MPLS interface enable | VPP interface has MPLS enabled (MplsInterfaceEnableDisable) |
| AC-6 | No labels in sysRIB event | Standard IP route programming (existing behavior unchanged) |
| AC-7 | VPP restart with MPLS routes | Replay repopulates both IP and MPLS routes |
| AC-8 | Label value 0-1048575 (20-bit) | Valid labels programmed, out-of-range rejected |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestProcessEventWithLabels` | `internal/plugins/fibvpp/mpls_test.go` | Event with labels field → MPLS backend methods called | |
| `TestProcessEventWithoutLabels` | `internal/plugins/fibvpp/mpls_test.go` | Event without labels → standard IP route (no MPLS) | |
| `TestMPLSPush` | `internal/plugins/fibvpp/mpls_test.go` | Label push: IPRouteAddDel with LabelStack in FibPath | |
| `TestMPLSSwap` | `internal/plugins/fibvpp/mpls_test.go` | Label swap: MplsRouteAddDel with in/out labels | |
| `TestMPLSPop` | `internal/plugins/fibvpp/mpls_test.go` | Label pop: MplsRouteAddDel with pop action | |
| `TestMPLSDelete` | `internal/plugins/fibvpp/mpls_test.go` | MPLS route deletion | |
| `TestMPLSInterfaceEnable` | `internal/plugins/fibvpp/mpls_test.go` | MPLS enabled on VPP interface | |
| `TestMPLSLabelRange` | `internal/plugins/fibvpp/mpls_test.go` | Label 0-1048575 accepted, >1048575 rejected | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| MPLS label | 0-1048575 (20-bit) | 1048575 | N/A | 1048576 |
| Label stack depth | 1-16 | 16 | 0 (no labels = IP route) | 17 (VPP FibPath limit) |
| TTL | 1-255 | 255 | 0 | 256 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-mpls-push` | `test/vpp/005-mpls-push.ci` | BGP labeled unicast → MPLS push in VPP FIB | |

### Future (if deferring any tests)
- Multi-label stack (stacked LSPs) deferred until MPLS VPN spec
- ECMP with unequal labels deferred

## Files to Modify

- `internal/plugins/fibvpp/fibvpp.go` — extend processEvent for labels
- `internal/plugins/fibvpp/backend.go` — extend vppBackend interface with MPLS methods

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | MPLS config is part of fib-vpp YANG from spec-vpp-2 |
| CLI commands/flags | No | - |
| Editor autocomplete | No | - |
| Functional test | Yes | `test/vpp/005-mpls-push.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` — add MPLS label programming |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` — update fib-vpp with MPLS |
| 6 | Has a user guide page? | Yes | `docs/guide/vpp.md` — MPLS section |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc3107.md`, `rfc/short/rfc8277.md` |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` — MPLS from BGP |
| 12 | Internal architecture changed? | No | - |

## Files to Create

- `internal/plugins/fibvpp/mpls.go` — MPLS route programming via GoVPP mpls.RPCService
- `internal/plugins/fibvpp/mpls_test.go` — MPLS tests
- `test/vpp/005-mpls-push.ci` — MPLS functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella + vpp-2 |
| 2. Audit | Files to Modify, Files to Create |
| 3. Implement (TDD) | Phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: sysRIB event extension** — add optional labels field to best-change payload
   - Tests: event parsing with and without labels
   - Files: sysRIB event payload (may be in rib plugin)
   - Verify: tests fail → implement → tests pass

2. **Phase: MPLS backend methods** — extend vppBackend with MPLS operations
   - Tests: `TestMPLSPush`, `TestMPLSSwap`, `TestMPLSPop`, `TestMPLSDelete`, `TestMPLSInterfaceEnable`, `TestMPLSLabelRange`
   - Files: mpls.go, mpls_test.go
   - Verify: tests fail → implement → tests pass

3. **Phase: Event processing integration** — fibvpp processEvent handles labels
   - Tests: `TestProcessEventWithLabels`, `TestProcessEventWithoutLabels`
   - Files: fibvpp.go (extend processEvent)
   - Verify: tests fail → implement → tests pass

4. **Functional tests** → `test/vpp/005-mpls-push.ci`
5. **Full verification** → `make ze-verify`
6. **Complete spec** → Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | MPLS label operations match RFC 3107/8277 semantics |
| Naming | Label operations use VPP API naming conventions |
| Data flow | Labels flow through sysRIB events, not bypassing EventBus |
| Rule: no-layering | No separate MPLS channel, extends existing event payload |
| Backward compatibility | Events without labels still work (standard IP routes) |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| MPLS backend methods | `grep "addMPLSRoute\|delMPLSRoute\|enableMPLS" internal/plugins/fibvpp/mpls.go` |
| Labels in event processing | `grep "labels\|Labels" internal/plugins/fibvpp/fibvpp.go` |
| MPLS tests | `go test -run TestMPLS internal/plugins/fibvpp/` |
| Functional test | `ls test/vpp/005-mpls-push.ci` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Label range | MPLS labels validated to 20-bit range (0-1048575) before GoVPP call |
| Stack depth | Label stack depth bounded by VPP FibPath limit (16) |
| TTL | TTL values validated to 1-255 range |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## MPLS Operations

| Operation | When | VPP API | Parameters |
|-----------|------|---------|------------|
| Push | PE ingress: IP packet needs label encap | IPRouteAddDel | IP prefix, FibPath with LabelStack populated, TTL=64 |
| Swap | P transit: labeled packet, swap label | MplsRouteAddDel | In-label (MrLabel), out-label (in LabelStack), next-hop, EOS bit |
| Pop | PE egress: remove label, deliver IP | MplsRouteAddDel | In-label (MrLabel), no out-label, next-hop for IP delivery |
| Enable | Before any MPLS ops on interface | MplsInterfaceEnableDisable | SwIfIndex, Enable=true |
| Disable | Cleanup | MplsInterfaceEnableDisable | SwIfIndex, Enable=false |

## sysRIB Event Payload Extension

Current payload fields: family, replay, changes (array of action/prefix/next-hop).

Extended fields per change entry:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| action | string | yes | "add", "del", "replace" |
| prefix | string | yes | IP prefix (e.g., "10.0.0.0/24") |
| next-hop | string | conditional | Next-hop address (required for add/replace) |
| protocol | string | yes | Route source protocol |
| labels | array of uint32 | no | MPLS label stack (empty = pure IP route) |

Backward compatible: events without labels field are treated as pure IP routes.

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

## Implementation Summary

### What Was Implemented
- (To be filled after implementation)

### Bugs Found/Fixed
- (To be filled)

### Documentation Updates
- (To be filled)

### Deviations from Plan
- (To be filled)

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
- [ ] AC-1..AC-8 all demonstrated
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
- [ ] Write learned summary to `plan/learned/NNN-vpp-3-mpls.md`
- [ ] Summary included in commit
