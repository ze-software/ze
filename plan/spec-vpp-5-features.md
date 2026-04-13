# Spec: vpp-5-features — VPP-Specific Features

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-vpp-4-iface |
| Phase | - |
| Updated | 2026-04-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `plan/spec-vpp-0-umbrella.md` — parent spec
4. `internal/component/vpp/` — VPP component from vpp-1
5. `internal/plugins/ifacevpp/` — VPP interface backend from vpp-4

## Task

Expose VPP-native features that have no kernel equivalent through YANG config and GoVPP API.
Each feature is self-contained: YANG container + Go handler + GoVPP calls. Features can be
implemented independently in any order, prioritized by user need.

This spec is a menu of features, not a single implementation. Each feature can be its own
implementation cycle within the spec.

### Features

| Feature | YANG container | GoVPP API | Priority |
|---------|---------------|-----------|----------|
| L2 cross-connect | `vpp l2xc` | SwInterfaceSetL2Xconnect | High (simple, useful) |
| Bridge domain with BVI | `vpp bridge-domain` | BridgeDomainAddDelV2 + BVI interface | High |
| VXLAN tunnel (dynamic) | `vpp vxlan` | VxlanAddDelTunnelV3 | High |
| Policer | `vpp policer` | PolicerAddDel | Medium |
| ACL | `vpp acl` | AclAddReplace, AclInterfaceSetAclList | Medium (also in spec-fw-6) |
| SRv6 policy | `vpp srv6` | SrPolicyAdd, SrSteeringAddDel | Medium |
| sFlow | `vpp sflow` | SflowEnableDisable, SflowSamplingRateSet | Low |

### Reference

- IPng.ch blog: L2 cross-connect, bridge domains, VXLAN configuration in VPP
- VyOS: VPP feature configuration patterns
- VPP 25.02 documentation: API reference for each feature module

## Required Reading

### Architecture Docs
- [ ] `internal/component/vpp/` — VPP component from vpp-1
  → Constraint: features use shared GoVPP connection from vpp component
- [ ] `internal/plugins/ifacevpp/` — VPP interface backend from vpp-4
  → Constraint: some features reference VPP interfaces by SwIfIndex
- [ ] `.claude/patterns/config-option.md` — config option pattern
  → Constraint: each feature adds YANG container under vpp
- [ ] `rules/config-design.md` — YANG rules
  → Constraint: fail on unknown keys, no version numbers

### RFC Summaries (MUST for protocol work)

SRv6 work references:
- [ ] `rfc/short/rfc8986.md` — SRv6 Network Programming (if exists, create if not)

**Key insights:**
- Each feature is self-contained: YANG + handler + GoVPP, no dependencies between features
- Features share GoVPP connection from vpp component
- ACL feature overlaps with spec-fw-6 (firewallvpp); coordinate to avoid duplication
- L2XC and bridge domains use SwIfIndex references, need naming module from vpp-4

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/vpp/` — VPP component provides connection and config
  → Constraint: features register as sub-handlers within vpp component or as separate plugins
- [ ] `internal/plugins/ifacevpp/naming.go` — Ze to VPP name mapping
  → Constraint: features reference VPP interfaces via naming module
- [ ] No existing VPP feature code in ze

**Behavior to preserve:**
- VPP component lifecycle unchanged
- Interface backend unchanged
- FIB plugin unchanged

**Behavior to change:**
- YANG module extended with feature containers under `vpp`
- Feature handlers added to vpp component or as sub-plugins

## Data Flow (MANDATORY)

### Entry Point
- YANG config contains feature-specific containers under `vpp` (e.g., `vpp { l2xc { ... } }`)
- VPP component parses feature config during OnConfigApply

### Transformation Path
1. Config component parses YANG tree, `vpp` subtree contains feature containers
2. VPP component iterates feature containers
3. For each feature: translate config to GoVPP API call(s)
4. GoVPP dispatches to VPP via binary API

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config → VPP component | YANG tree parsing, OnConfigApply | [ ] |
| VPP component → GoVPP | Feature-specific binary API calls | [ ] |
| GoVPP → VPP | Unix socket binary protocol | [ ] |

### Integration Points
- `internal/component/vpp/conn.go` — shared GoVPP connection
- `internal/plugins/ifacevpp/naming.go` — interface name resolution for SwIfIndex
- `internal/component/vpp/schema/ze-vpp-conf.yang` — extended with feature containers

### Architectural Verification
- [ ] No bypassed layers (config → handler → GoVPP → VPP)
- [ ] No unintended coupling (features independent of each other)
- [ ] No duplicated functionality (ACL feature coordinates with spec-fw-6)
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `vpp { l2xc { ... } }` YANG config | → | SwInterfaceSetL2Xconnect via GoVPP | `test/vpp/008-l2xc.ci` |
| `vpp { bridge-domain { ... } }` YANG config | → | BridgeDomainAddDelV2 via GoVPP | `test/vpp/009-bridge.ci` |
| `vpp { vxlan { ... } }` YANG config | → | VxlanAddDelTunnelV3 via GoVPP | `test/vpp/010-vxlan.ci` |
| `vpp { policer { ... } }` YANG config | → | PolicerAddDel via GoVPP | `test/vpp/011-policer.ci` |

## Acceptance Criteria

Per-feature AC. Each feature has its own set.

### L2 Cross-Connect
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-L2XC-1 | L2XC config with two interfaces | VPP L2 cross-connect established between them |
| AC-L2XC-2 | L2XC config removed | Cross-connect torn down |

### Bridge Domain
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-BD-1 | Bridge domain config with member interfaces | VPP bridge domain created with ports |
| AC-BD-2 | BVI interface configured | BVI created and added to bridge domain |
| AC-BD-3 | Bridge domain removed | Domain and BVI deleted |

### VXLAN
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-VX-1 | VXLAN tunnel config with src, dst, VNI | VPP VXLAN tunnel created |
| AC-VX-2 | VXLAN tunnel removed | Tunnel deleted |

### Policer
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-POL-1 | Policer config with CIR/EIR | VPP policer created |
| AC-POL-2 | Policer applied to interface | Traffic policed per config |

### SRv6
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-SR-1 | SRv6 policy config | VPP SR policy created |
| AC-SR-2 | SRv6 steering config | Traffic steered into SR policy |

### sFlow
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-SF-1 | sFlow enabled on interface | VPP sFlow sampling active |
| AC-SF-2 | sFlow sampling rate configured | Rate applied |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestL2XCApply` | `internal/component/vpp/features_test.go` | L2XC config → SwInterfaceSetL2Xconnect | |
| `TestBridgeDomainApply` | `internal/component/vpp/features_test.go` | BD config → BridgeDomainAddDelV2 + BVI | |
| `TestVXLANApply` | `internal/component/vpp/features_test.go` | VXLAN config → VxlanAddDelTunnelV3 | |
| `TestPolicerApply` | `internal/component/vpp/features_test.go` | Policer config → PolicerAddDel | |
| `TestSRv6Apply` | `internal/component/vpp/features_test.go` | SR policy → SrPolicyAdd + SrSteeringAddDel | |
| `TestSflowApply` | `internal/component/vpp/features_test.go` | sFlow → SflowEnableDisable | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| VXLAN VNI | 1-16777215 | 16777215 | 0 | 16777216 |
| Policer CIR | 1+ kbps | 1 | 0 | N/A |
| Policer EIR | >= CIR | CIR value | CIR-1 | N/A |
| sFlow sampling rate | 1+ | 1 | 0 | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-l2xc` | `test/vpp/008-l2xc.ci` | L2XC config, cross-connect active | |
| `test-bridge` | `test/vpp/009-bridge.ci` | Bridge domain with BVI, members active | |
| `test-vxlan` | `test/vpp/010-vxlan.ci` | VXLAN tunnel created with VNI | |
| `test-policer` | `test/vpp/011-policer.ci` | Policer applied to interface | |

### Future (if deferring any tests)
- SRv6 and sFlow tests deferred to when those features are implemented
- ACL tests coordinated with spec-fw-6

## Files to Modify

- `internal/component/vpp/schema/ze-vpp-conf.yang` — add feature containers
- `internal/component/vpp/config.go` — parse feature config sections

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | Yes | `internal/component/vpp/schema/ze-vpp-conf.yang` |
| CLI commands/flags | Yes | VPP feature show commands (per feature) |
| Editor autocomplete | Yes | YANG-driven (automatic) |
| Functional test | Yes | Per-feature .ci tests |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` — VPP features |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` — VPP feature sections |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` — per feature |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | Extends existing VPP component |
| 6 | Has a user guide page? | Yes | `docs/guide/vpp.md` — per feature |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | Conditional | SRv6: RFC 8986 |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` |
| 12 | Internal architecture changed? | No | - |

## Files to Create

Per-feature files (created as each feature is implemented):

- `internal/component/vpp/l2xc.go` — L2 cross-connect handler
- `internal/component/vpp/bridge.go` — Bridge domain handler
- `internal/component/vpp/vxlan.go` — VXLAN tunnel handler
- `internal/component/vpp/policer.go` — Policer handler
- `internal/component/vpp/srv6.go` — SRv6 policy handler
- `internal/component/vpp/sflow.go` — sFlow handler
- `internal/component/vpp/features_test.go` — Feature tests
- `test/vpp/008-l2xc.ci` through `test/vpp/011-policer.ci` — Functional tests

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella |
| 2. Audit | Files to Modify, Files to Create |
| 3. Implement (TDD) | Per-feature phases below |
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

Each feature is a self-contained phase. Order by priority. Each ends with Self-Critical Review.

1. **Phase: L2 cross-connect** — simplest feature, validates pattern
   - Tests: `TestL2XCApply`
   - Files: l2xc.go, YANG extension, .ci test
   - Verify: tests fail → implement → tests pass

2. **Phase: Bridge domain** — BVI integration
   - Tests: `TestBridgeDomainApply`
   - Files: bridge.go, YANG extension, .ci test
   - Verify: tests fail → implement → tests pass

3. **Phase: VXLAN** — dynamic tunnel endpoints
   - Tests: `TestVXLANApply`
   - Files: vxlan.go, YANG extension, .ci test
   - Verify: tests fail → implement → tests pass

4. **Phase: Policer** — traffic policing
   - Tests: `TestPolicerApply`
   - Files: policer.go, YANG extension, .ci test
   - Verify: tests fail → implement → tests pass

5. **Phase: SRv6** — segment routing
   - Tests: `TestSRv6Apply`
   - Files: srv6.go, YANG extension
   - Verify: tests fail → implement → tests pass

6. **Phase: sFlow** — sampling
   - Tests: `TestSflowApply`
   - Files: sflow.go, YANG extension
   - Verify: tests fail → implement → tests pass

7. **Full verification** → `make ze-verify` after each feature
8. **Complete spec** → Fill audit tables after all prioritized features done

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every implemented feature's AC demonstrated |
| Correctness | GoVPP API calls match VPP 25.02 for each feature |
| Naming | YANG containers follow ze conventions |
| Data flow | Config → handler → GoVPP → VPP per feature |
| Rule: no-layering | Direct GoVPP calls, no abstraction between features |
| Independence | Features do not depend on each other |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| Feature handler files | `ls internal/component/vpp/{l2xc,bridge,vxlan,policer,srv6,sflow}.go` |
| YANG containers | `grep "container" internal/component/vpp/schema/ze-vpp-conf.yang` |
| Feature tests | `go test -run Test.*Apply internal/component/vpp/` |
| Functional tests | `ls test/vpp/008-l2xc.ci test/vpp/009-bridge.ci` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| VXLAN VNI | Range validated (1-16777215) |
| Policer rates | Positive values, EIR >= CIR |
| SRv6 SID | Valid IPv6 address format |
| sFlow rate | Positive integer |
| Interface references | Validated via naming module before GoVPP calls |

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

## YANG Config Shape (per feature)

### L2 Cross-Connect
| Container | Leaf | Type | Description |
|-----------|------|------|-------------|
| vpp/l2xc (list, key: name) | name | string | Cross-connect name |
| vpp/l2xc | interface-a | leafref to interface | First interface |
| vpp/l2xc | interface-b | leafref to interface | Second interface |

### Bridge Domain
| Container | Leaf | Type | Description |
|-----------|------|------|-------------|
| vpp/bridge-domain (list, key: name) | name | string | Bridge domain name |
| vpp/bridge-domain | bvi-interface | string | BVI loopback name |
| vpp/bridge-domain/member (list) | interface | leafref | Member interface |

### VXLAN Tunnel
| Container | Leaf | Type | Description |
|-----------|------|------|-------------|
| vpp/vxlan (list, key: name) | name | string | Tunnel name |
| vpp/vxlan | source-address | ip-address | Local endpoint |
| vpp/vxlan | destination-address | ip-address | Remote endpoint |
| vpp/vxlan | vni | uint32 (1-16777215) | VXLAN Network Identifier |

### Policer
| Container | Leaf | Type | Description |
|-----------|------|------|-------------|
| vpp/policer (list, key: name) | name | string | Policer name |
| vpp/policer | cir | uint32 | Committed Information Rate (kbps) |
| vpp/policer | eir | uint32 | Excess Information Rate (kbps) |
| vpp/policer | interface | leafref | Interface to police |

### sFlow
| Container | Leaf | Type | Description |
|-----------|------|------|-------------|
| vpp/sflow | enabled | boolean | Enable sFlow |
| vpp/sflow | sampling-rate | uint32 | Packet sampling rate |
| vpp/sflow/interface (list) | name | leafref | Interfaces to sample |

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
- [ ] Implemented feature ACs all demonstrated
- [ ] Wiring Test table complete for implemented features
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
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
- [ ] Write learned summary to `plan/learned/NNN-vpp-5-features.md`
- [ ] Summary included in commit
