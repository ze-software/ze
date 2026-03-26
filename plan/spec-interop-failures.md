# Spec: interop-failures

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-26 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/testing/interop.md` - scenario inventory and results
4. Interop test results table below (Category 1-3)

## Task

Investigate and fix Ze bugs discovered by interop scenarios 22-32. The test infrastructure is correct (validated by 4 passing scenarios with all 3 vendors). Failures are Ze bugs in EVPN, VPN, FlowSpec address family handling and IPv6 route delivery to BIRD.

### Interop Test Results (2026-03-26)

| Scenario | Feature | Vendor | Result | Failure |
|----------|---------|--------|--------|---------|
| 22-evpn-frr | EVPN | FRR | FAIL | l2vpn/evpn family not negotiated |
| 23-vpn-frr | VPN | FRR | FAIL | Routes not received (session up) |
| 24-flowspec-frr | FlowSpec | FRR | FAIL | Rules not received (session up) |
| 25-ipv6-ebgp-bird | IPv6 | BIRD | FAIL | Routes not received (session up) |
| 26-ipv6-ebgp-gobgp | IPv6 | GoBGP | PASS | |
| 27-multihop-ebgp-frr | Multihop | FRR | PASS | |
| 28-evpn-gobgp | EVPN | GoBGP | FAIL | Routes not received |
| 29-vpn-gobgp | VPN | GoBGP | FAIL | Session never established |
| 30-flowspec-gobgp | FlowSpec | GoBGP | FAIL | Rules not received |
| 31-multihop-ebgp-bird | Multihop | BIRD | PASS | |
| 32-multihop-ebgp-gobgp | Multihop | GoBGP | PASS | |

### Triangulation

| Feature | FRR | BIRD | GoBGP | Conclusion |
|---------|-----|------|-------|------------|
| IPv4 unicast | PASS (27) | PASS (31) | PASS (32) | Works |
| IPv6 unicast | existing-10 | FAIL (25) | PASS (26) | Ze or BIRD-specific |
| EVPN | FAIL (22) | N/A | FAIL (28) | Ze bug |
| VPN | FAIL (23) | N/A | FAIL (29) | Ze bug |
| FlowSpec | FAIL (24) | N/A | FAIL (30) | Ze bug |
| Multihop | PASS (27) | PASS (31) | PASS (32) | Works |

### Failure Categories

**Category 1: EVPN capability not negotiated (scenarios 22, 28)**
Session establishes but l2vpn/evpn is not in OPEN capabilities. The `bgp-nlri-evpn` plugin registers `l2vpn/evpn` but Ze may not include it in the outgoing OPEN.

**Category 2: Routes not delivered (scenarios 23, 24, 25, 30)**
Session up, family may be negotiated, but routes from process plugin never reach peer. Either text command rejected, encoding fails, or UPDATE not forwarded.

**Category 3: Session fails (scenario 29)**
VPN with GoBGP -- session never establishes. Possible GoBGP afi-safi name mismatch (`ipv4-vpnv4unicast` may be wrong).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/interop.md` - scenario inventory and test framework
  -> Constraint: scenarios use config `remote { ip; as; }` + `local { ip; as; }` + `family { ... { prefix { maximum N; } } }` format

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4760.md` - Multiprotocol Extensions (MP_REACH_NLRI capability negotiation)
  -> Constraint: each address family must be announced via capability code 1 in OPEN

**Key insights:**
- EVPN/VPN/FlowSpec fail with BOTH FRR and GoBGP, confirming Ze bugs (not vendor issues)
- IPv4 unicast and multihop work with all 3 vendors (control group)
- Process plugin delivery works for IPv4/IPv6 unicast (proven by scenarios 26, 27, 31, 32)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/session.go` - session OPEN construction, capability building
- [ ] `internal/component/bgp/plugins/cmd/update/update_text.go` - text command parsing
- [ ] `internal/component/bgp/plugins/nlri/evpn/register.go` - EVPN plugin registration (families: l2vpn/evpn)
- [ ] `internal/component/bgp/plugins/nlri/vpn/register.go` - VPN plugin registration (families: ipv4/vpn, ipv6/vpn)
- [ ] `internal/component/bgp/plugins/nlri/flowspec/register.go` - FlowSpec plugin registration

**Behavior to preserve:**
- All 4 passing scenarios (26, 27, 31, 32) continue passing
- Existing 21 scenarios (01-21) behavior unchanged

**Behavior to change:**
- Ze must negotiate EVPN/VPN/FlowSpec capabilities in OPEN
- Ze must deliver routes for non-unicast families from process plugins to peers

## Data Flow (MANDATORY)

### Entry Point
- Ze config file with non-unicast family declarations
- Process plugin flush() commands with EVPN/VPN/FlowSpec NLRI

### Transformation Path
1. Config parsing: `family { l2vpn/evpn { ... } }` -> family set for peer
2. Capability building: family set -> MP_REACH capability in OPEN message
3. Plugin command: `flush('peer * update text ... nlri l2vpn/evpn add ...')` -> text parser -> UPDATE builder
4. UPDATE forwarding: reactor -> peer session -> wire encoding -> TCP

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Capabilities | Family set -> OPEN message capability list | [ ] |
| Plugin -> Reactor | Text command IPC -> parsed route -> announce batch | [ ] |
| Reactor -> Wire | UPDATE builder -> MP_REACH_NLRI encoding | [ ] |

### Integration Points
- `bgp-nlri-evpn` plugin encode/decode
- `bgp-nlri-vpn` plugin encode/decode
- `bgp-nlri-flowspec` plugin encode/decode
- `peersettings.go` family negotiation

### Architectural Verification
- [ ] Capability building includes all configured families
- [ ] Text command parser handles EVPN/VPN/FlowSpec NLRI
- [ ] UPDATE encoding works for non-unicast families
- [ ] Forward pool dispatches to peers with matching families

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| ze.conf with l2vpn/evpn | -> | OPEN capability + EVPN UPDATE | `test/interop/scenarios/22-evpn-frr/check.py` |
| ze.conf with ipv4/mpls-vpn | -> | OPEN capability + VPN UPDATE | `test/interop/scenarios/23-vpn-frr/check.py` |
| ze.conf with ipv4/flow | -> | OPEN capability + FlowSpec UPDATE | `test/interop/scenarios/24-flowspec-frr/check.py` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Ze configured with l2vpn/evpn, FRR peer | EVPN capability negotiated, Type-2 route received by FRR |
| AC-2 | Ze configured with ipv4/mpls-vpn, FRR peer | VPN capability negotiated, VPN route received by FRR |
| AC-3 | Ze configured with ipv4/flow, FRR peer | FlowSpec capability negotiated, rules received by FRR |
| AC-4 | Ze configured with ipv6/unicast, BIRD peer | IPv6 routes received by BIRD |
| AC-5 | Ze configured with l2vpn/evpn, GoBGP peer | EVPN routes received by GoBGP |
| AC-6 | Ze configured with ipv4/mpls-vpn, GoBGP peer | VPN session established, routes received |
| AC-7 | Ze configured with ipv4/flow, GoBGP peer | FlowSpec rules received by GoBGP |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| To be determined during investigation | | | |

### Boundary Tests (MANDATORY for numeric inputs)

Not applicable -- investigation spec, no new numeric inputs.

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| 22-evpn-frr | `test/interop/scenarios/22-evpn-frr/check.py` | EVPN with FRR | FAIL |
| 23-vpn-frr | `test/interop/scenarios/23-vpn-frr/check.py` | VPN with FRR | FAIL |
| 24-flowspec-frr | `test/interop/scenarios/24-flowspec-frr/check.py` | FlowSpec with FRR | FAIL |
| 25-ipv6-ebgp-bird | `test/interop/scenarios/25-ipv6-ebgp-bird/check.py` | IPv6 with BIRD | FAIL |
| 28-evpn-gobgp | `test/interop/scenarios/28-evpn-gobgp/check.py` | EVPN with GoBGP | FAIL |
| 29-vpn-gobgp | `test/interop/scenarios/29-vpn-gobgp/check.py` | VPN with GoBGP | FAIL |
| 30-flowspec-gobgp | `test/interop/scenarios/30-flowspec-gobgp/check.py` | FlowSpec with GoBGP | FAIL |

## Files to Modify

- To be determined during investigation (likely reactor, session, capability building)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | - |
| CLI commands/flags | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | No | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |

## Files to Create

- None expected (bug fixes in existing files)

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify -- trace each failure category |
| 3. Implement (TDD) | Fix bugs per category below |
| 4. Full verification | `NO_BUILD=1 make ze-interop-test` for scenarios 22-32 |
| 5. Critical review | Critical Review Checklist below |
| 6-12 | Standard flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Debug logging** -- add `environment { log { level debug } }` to failing ze.conf files, re-run, capture Ze logs to identify failure points
2. **Phase: Category 1 (EVPN capability)** -- fix OPEN capability building to include l2vpn/evpn
   - Verify: scenarios 22, 28 pass
3. **Phase: Category 2 (route delivery)** -- fix process plugin route delivery for non-unicast families
   - Verify: scenarios 23, 24, 25, 30 pass
4. **Phase: Category 3 (VPN/GoBGP session)** -- fix GoBGP afi-safi name or capability mismatch
   - Verify: scenario 29 passes
5. **Full verification** -- all 32 scenarios

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 7 failing scenarios now pass |
| Regression | All 4 previously passing scenarios still pass |
| Root cause | Each fix addresses the actual root cause, not symptoms |
| Triangulation | If Ze<->FRR passes, Ze<->GoBGP should also pass for the same feature |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Scenarios 22-32 all pass | `NO_BUILD=1 make ze-interop-test INTEROP_SCENARIO=22-*` etc. |
| No regression in passing scenarios | scenarios 26, 27, 31, 32 still pass |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Fixes don't weaken capability negotiation or NLRI validation |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Debug logging shows text command rejected | Fix text command parser |
| Debug logging shows UPDATE built but not sent | Fix forward pool family filtering |
| Debug logging shows no capability in OPEN | Fix capability building from family config |
| GoBGP rejects with NOTIFICATION | Fix capability encoding or afi-safi matching |
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

To be added during investigation.

## Implementation Summary

### What Was Implemented
- [To be filled after fixes]

### Bugs Found/Fixed
- [To be filled after fixes]

### Documentation Updates
- [To be filled after fixes]

### Deviations from Plan
- [To be filled after fixes]
