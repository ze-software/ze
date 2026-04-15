# Spec: vpp-4-iface — VPP Interface Backend

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-vpp-1-lifecycle |
| Phase | - |
| Updated | 2026-04-14 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `plan/spec-vpp-0-umbrella.md` — parent spec
4. `internal/component/iface/backend.go` — Backend interface to implement
5. `internal/plugins/ifacenetlink/` — existing netlink backend to mirror

## Task

Implement `iface.Backend` for VPP via GoVPP. Ze creates/configures VPP interfaces directly,
not through Linux netlink. The backend implements all 30+ Backend interface methods using
GoVPP binary API calls. Also manages LCP pairs so control-plane interfaces (BGP TCP) exist
as Linux TAPs, and maintains a bidirectional naming map between ze short names and VPP long names.

With this spec, `ze config edit` becomes the single configuration authority for VPP interfaces.
Without it, VPP interfaces must be managed separately via vppcfg or vppctl.

### Reference

- Internal: `internal/component/iface/backend.go` (Backend interface definition)
- Internal: `internal/plugins/ifacenetlink/` (netlink backend, same pattern)
- GoVPP interface binapi: SwInterfaceDump, SwInterfaceSetFlags, SwInterfaceAddDelAddress
- IPng.ch blog: LCP pair management, interface naming in VPP

## Required Reading

### Architecture Docs
- [ ] `internal/component/iface/backend.go` — Backend interface: 30+ methods
  → Constraint: ifacevpp must implement every method or return meaningful error for unsupported ops
  → Decision: RegisterBackend("vpp", factory) in init()
- [ ] `internal/plugins/ifacenetlink/register.go` — netlink backend registration
  → Constraint: same pattern: init() calls iface.RegisterBackend
- [ ] `internal/plugins/ifacenetlink/ifacenetlink.go` — netlink backend implementation
  → Constraint: reference implementation, all methods map to netlink calls
- [ ] `internal/component/iface/types.go` — InterfaceInfo, InterfaceStats, RouteInfo, TunnelSpec types
  → Constraint: ifacevpp returns same types, populated from GoVPP responses
- [ ] `docs/architecture/core-design.md` — component/plugin split
  → Constraint: iface is a component, ifacevpp is a plugin

### RFC Summaries (MUST for protocol work)

Not protocol work. No RFCs apply.

**Key insights:**
- Backend is a 30+ method interface covering lifecycle, addressing, routes, link state, properties, query, bridge, mirror, monitor
- ifacenetlink registers via iface.RegisterBackend("netlink", factory) in init(). NOT via registry.Register(). No plugin SDK.
- ifacevpp follows same: iface.RegisterBackend("vpp", factory). Blank import triggers init().
- GoVPP connection via vpp.GetActiveConnector().NewChannel()
- GoVPP binapi packages: interface, l2, vxlan, gre, lcp, tapv2
- VPP interface names are long (TenGigabitEthernet3/0/0); ze uses short names (xe0)
- LCP pairs create Linux TAPs for VPP interfaces so BGP TCP sessions can bind
- Unsupported ops (CreateVeth, Wireguard without plugin, Mirror on old VPP) return descriptive errors

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/backend.go` — Backend interface definition, RegisterBackend, LoadBackend, GetBackend
  → Constraint: ifacevpp implements this interface
- [ ] `internal/component/iface/types.go` — InterfaceInfo, InterfaceStats, RouteInfo, TunnelSpec, WireguardSpec
  → Constraint: ifacevpp returns these types populated from GoVPP data
- [ ] `internal/plugins/ifacenetlink/ifacenetlink.go` — netlink Backend implementation
  → Constraint: reference for method-by-method mapping
- [ ] `internal/plugins/ifacenetlink/register.go` — init() → iface.RegisterBackend("netlink", factory)
  → Constraint: ifacevpp uses iface.RegisterBackend("vpp", factory)

**Behavior to preserve:**
- ifacenetlink continues to work for Linux interfaces
- Backend interface unchanged
- iface component selects backend by config leaf
- All existing types (InterfaceInfo, etc.) unchanged

**Behavior to change:**
- New ifacevpp plugin implements Backend for VPP
- Backend "vpp" available for selection in config
- VPP interfaces managed via GoVPP instead of netlink

## Data Flow (MANDATORY)

### Entry Point
- YANG config specifies backend: "vpp" for VPP interfaces
- iface component loads ifacevpp backend via LoadBackend("vpp")
- iface component calls Backend methods for interface operations

### Transformation Path
1. iface component receives config with interface definitions
2. Component selects "vpp" backend based on config leaf
3. Component calls Backend methods (CreateDummy, AddAddress, SetAdminUp, etc.)
4. ifacevpp translates each method to GoVPP binary API call(s)
5. GoVPP sends binary message to VPP via api-socket
6. VPP creates/modifies interface
7. If LCP enabled: ifacevpp also creates LCP pair (Linux TAP) for control-plane access

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| iface component → ifacevpp | Backend interface method calls | [ ] |
| ifacevpp → GoVPP | Binary API via VPPConn from vpp component | [ ] |
| GoVPP → VPP | Unix socket binary protocol | [ ] |
| ifacevpp → LCP | GoVPP LCP pair creation (linux_cp binapi) | [ ] |

### Integration Points
- `internal/component/iface/backend.go` — Backend interface that ifacevpp implements
- `internal/component/vpp/conn.go` — shared VPPConn provides interface/LCP API clients
- `internal/plugins/ifacenetlink/` — coexists, selected by config

### Architectural Verification
- [ ] No bypassed layers (iface component → Backend interface → GoVPP → VPP)
- [ ] No unintended coupling (ifacevpp depends on vpp component for connection only)
- [ ] No duplicated functionality (parallels ifacenetlink for different dataplane)
- [ ] Zero-copy preserved where applicable (GoVPP handles binary encoding)

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| iface YANG config with backend "vpp" | → | ifacevpp CreateDummy/CreateBridge/CreateVLAN | `test/vpp/006-iface-create.ci` |
| iface component AddAddress | → | ifacevpp SwInterfaceAddDelAddress via GoVPP | `test/vpp/006-iface-create.ci` |
| iface component SetAdminUp | → | ifacevpp SwInterfaceSetFlags via GoVPP | `test/vpp/006-iface-create.ci` |
| LCP enabled in config | → | ifacevpp creates LCP pair for VPP interface | `test/vpp/006-iface-create.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Backend "vpp" configured | ifacevpp loaded, all Backend methods available |
| AC-2 | CreateDummy("loop0") | VPP loopback interface created (CreateLoopback) |
| AC-3 | CreateBridge("br0") | VPP bridge domain created (BridgeDomainAddDelV2) |
| AC-4 | CreateVLAN("xe0", 100) | VPP sub-interface created (CreateSubif with dot1q) |
| AC-5 | CreateTunnel(vxlan spec) | VPP VXLAN tunnel created (VxlanAddDelTunnelV3) |
| AC-6 | CreateTunnel(gre spec) | VPP GRE tunnel created (GreTunnelAddDel) |
| AC-7 | AddAddress("xe0", "10.0.0.1/24") | VPP interface has address (SwInterfaceAddDelAddress) |
| AC-8 | SetAdminUp("xe0") | VPP interface admin up (SwInterfaceSetFlags with ADMIN_UP) |
| AC-9 | SetMTU("xe0", 9000) | VPP interface MTU set (SwInterfaceSetMtu) |
| AC-10 | ListInterfaces() | All VPP interfaces returned with correct names and state |
| AC-11 | GetStats("xe0") | Interface rx/tx/drop counters returned |
| AC-12 | LCP enabled, interface created | Linux TAP exists for VPP interface |
| AC-13 | Ze short name "xe0" used | Maps to VPP long name (e.g., TenGigabitEthernet3/0/0) |
| AC-14 | DeleteInterface("loop0") | VPP loopback deleted (DeleteLoopback) |
| AC-15 | BridgeAddPort("br0", "xe0") | VPP interface added to bridge domain (SwInterfaceSetL2Bridge) |
| AC-16 | StartMonitor(eventbus) | VPP interface events received (WantInterfaceEvents) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCreateDummy` | `internal/plugins/ifacevpp/ifacevpp_test.go` | CreateLoopback called via mock GoVPP | |
| `TestCreateBridge` | `internal/plugins/ifacevpp/ifacevpp_test.go` | BridgeDomainAddDelV2 called | |
| `TestCreateVLAN` | `internal/plugins/ifacevpp/ifacevpp_test.go` | CreateSubif with correct VLAN ID | |
| `TestCreateTunnelVXLAN` | `internal/plugins/ifacevpp/ifacevpp_test.go` | VxlanAddDelTunnelV3 called | |
| `TestCreateTunnelGRE` | `internal/plugins/ifacevpp/ifacevpp_test.go` | GreTunnelAddDel called | |
| `TestAddAddress` | `internal/plugins/ifacevpp/ifacevpp_test.go` | SwInterfaceAddDelAddress with correct params | |
| `TestRemoveAddress` | `internal/plugins/ifacevpp/ifacevpp_test.go` | SwInterfaceAddDelAddress with IsAdd=false | |
| `TestSetAdminUp` | `internal/plugins/ifacevpp/ifacevpp_test.go` | SwInterfaceSetFlags with ADMIN_UP | |
| `TestSetAdminDown` | `internal/plugins/ifacevpp/ifacevpp_test.go` | SwInterfaceSetFlags with flags=0 | |
| `TestSetMTU` | `internal/plugins/ifacevpp/ifacevpp_test.go` | SwInterfaceSetMtu called | |
| `TestSetMACAddress` | `internal/plugins/ifacevpp/ifacevpp_test.go` | SwInterfaceSetMacAddress called | |
| `TestListInterfaces` | `internal/plugins/ifacevpp/ifacevpp_test.go` | SwInterfaceDump results converted to InterfaceInfo | |
| `TestGetInterface` | `internal/plugins/ifacevpp/ifacevpp_test.go` | SwInterfaceDump with name filter | |
| `TestGetStats` | `internal/plugins/ifacevpp/ifacevpp_test.go` | Stats API returns correct counters | |
| `TestBridgeAddPort` | `internal/plugins/ifacevpp/ifacevpp_test.go` | SwInterfaceSetL2Bridge called | |
| `TestBridgeDelPort` | `internal/plugins/ifacevpp/ifacevpp_test.go` | SwInterfaceSetL2Bridge with enable=false | |
| `TestDeleteInterface` | `internal/plugins/ifacevpp/ifacevpp_test.go` | Correct delete API called per interface type | |
| `TestNameMapping` | `internal/plugins/ifacevpp/naming_test.go` | Ze name to VPP name bidirectional mapping | |
| `TestNameMappingPopulate` | `internal/plugins/ifacevpp/naming_test.go` | Map populated from SwInterfaceDump at startup | |
| `TestLCPPairCreation` | `internal/plugins/ifacevpp/ifacevpp_test.go` | LCP pair created for new interfaces when enabled | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| VLAN ID | 1-4094 | 4094 | 0 | 4095 |
| MTU | 68-65535 | 65535 | 67 | 65536 |
| Bridge domain ID | 1-16777215 | 16777215 | 0 | 16777216 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-iface-create` | `test/vpp/006-iface-create.ci` | VPP interface config, interface exists with address and admin up | |

### Future (if deferring any tests)
- Wireguard device methods may return "not supported" if VPP wireguard plugin not loaded
- Mirror/SPAN support depends on VPP version

## Files to Modify

- `internal/component/iface/backend.go` — no changes, reference only

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | Backend selection is existing config leaf, no new YANG |
| CLI commands/flags | No | iface CLI already exists |
| Editor autocomplete | No | - |
| Functional test | Yes | `test/vpp/006-iface-create.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` — VPP interface backend |
| 2 | Config syntax changed? | No | Backend selection already exists |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` — add iface-vpp |
| 6 | Has a user guide page? | Yes | `docs/guide/vpp.md` — interface section |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` — unified config authority |
| 12 | Internal architecture changed? | No | Follows existing Backend pattern |

## Files to Create

- `internal/plugins/ifacevpp/ifacevpp.go` — VPP Backend implementation (all 30+ methods)
- `internal/plugins/ifacevpp/naming.go` — Ze name to VPP name bidirectional mapping
- `internal/plugins/ifacevpp/register.go` — RegisterBackend("vpp", factory)
- `internal/plugins/ifacevpp/ifacevpp_test.go` — Backend method tests with mock GoVPP
- `internal/plugins/ifacevpp/naming_test.go` — Naming tests
- `test/vpp/006-iface-create.ci` — Interface creation functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella |
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

1. **Phase: Naming module** — bidirectional name map, populate from SwInterfaceDump
   - Tests: `TestNameMapping`, `TestNameMappingPopulate`
   - Files: naming.go, naming_test.go
   - Verify: tests fail → implement → tests pass

2. **Phase: Core lifecycle methods** — Create*, Delete*, SetAdmin*, registration
   - Tests: `TestCreateDummy`, `TestCreateBridge`, `TestCreateVLAN`, `TestDeleteInterface`, `TestSetAdminUp`, `TestSetAdminDown`
   - Files: ifacevpp.go, register.go, ifacevpp_test.go
   - Verify: tests fail → implement → tests pass

3. **Phase: Address and property methods** — Add/RemoveAddress, SetMTU, SetMAC, Get*
   - Tests: `TestAddAddress`, `TestRemoveAddress`, `TestSetMTU`, `TestSetMACAddress`, `TestGetStats`, `TestListInterfaces`, `TestGetInterface`
   - Files: ifacevpp.go, ifacevpp_test.go
   - Verify: tests fail → implement → tests pass

4. **Phase: Bridge and tunnel methods** — BridgeAdd/DelPort, CreateTunnel, StartMonitor
   - Tests: `TestBridgeAddPort`, `TestBridgeDelPort`, `TestCreateTunnelVXLAN`, `TestCreateTunnelGRE`
   - Files: ifacevpp.go, ifacevpp_test.go
   - Verify: tests fail → implement → tests pass

5. **Phase: LCP pair management** — create TAP pairs for VPP interfaces
   - Tests: `TestLCPPairCreation`
   - Files: ifacevpp.go, ifacevpp_test.go
   - Verify: tests fail → implement → tests pass

6. **Functional tests** → `test/vpp/006-iface-create.ci`
7. **Full verification** → `make ze-verify`
8. **Complete spec** → Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every Backend method implemented or returns clear "not supported" error |
| Correctness | GoVPP API calls match VPP 25.02 interface binapi |
| Naming | Bidirectional name map consistent, populated at startup |
| Data flow | iface component → Backend method → GoVPP → VPP |
| Rule: no-layering | No kernel intermediary for VPP interface management |
| Rule: single-responsibility | ifacevpp.go = Backend methods, naming.go = name map |
| LCP | TAP pairs created when LCP enabled, BGP can bind on Linux side |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| ifacevpp plugin directory | `ls internal/plugins/ifacevpp/` |
| Backend registration | `grep "RegisterBackend" internal/plugins/ifacevpp/register.go` |
| All Backend methods | `grep -c "func.*ifacevpp.*Backend" internal/plugins/ifacevpp/ifacevpp.go` — should be 30+ |
| Naming module | `ls internal/plugins/ifacevpp/naming.go` |
| Tests | `go test internal/plugins/ifacevpp/` |
| Functional test | `ls test/vpp/006-iface-create.ci` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Interface names | Validate interface names before GoVPP calls (no injection) |
| VLAN ID | Range checked (1-4094) |
| MAC address | Format validated before SetMACAddress |
| Address CIDR | Prefix parsed and validated before AddAddress |
| LCP netns | Network namespace name validated (alphanumeric only) |

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

## Backend Method Mapping

| Backend Method | GoVPP API | Notes |
|---------------|-----------|-------|
| CreateDummy | CreateLoopback | Returns SwIfIndex |
| CreateVeth | N/A | VPP uses memif/TAP, not veth. Return "not supported for VPP". |
| CreateBridge | BridgeDomainAddDelV2 | Returns BD ID |
| CreateVLAN | CreateSubif (dot1q) | Parent SwIfIndex + VLAN ID |
| CreateTunnel (vxlan) | VxlanAddDelTunnelV3 | Src/dst IP, VNI |
| CreateTunnel (gre) | GreTunnelAddDel | Src/dst IP |
| CreateTunnel (ipip) | IpipAddTunnel | Src/dst IP |
| CreateWireguardDevice | WireguardInterfaceCreate | If WG plugin loaded |
| DeleteInterface | DeleteLoopback / DeleteSubif / set admin down | Depends on type |
| AddAddress | SwInterfaceAddDelAddress (IsAdd=true) | Parse CIDR to VPP prefix |
| RemoveAddress | SwInterfaceAddDelAddress (IsAdd=false) | |
| ReplaceAddressWithLifetime | SwInterfaceAddDelAddress | VPP does not support lifetime directly |
| AddRoute | IPRouteAddDel | Interface route |
| RemoveRoute | IPRouteAddDel (IsAdd=false) | |
| ListRoutes | IPRouteDump with filter | |
| SetAdminUp | SwInterfaceSetFlags (IF_STATUS_API_FLAG_ADMIN_UP) | |
| SetAdminDown | SwInterfaceSetFlags (flags: 0) | |
| SetMTU | SwInterfaceSetMtu (HwInterfaceSetMtu for L2) | |
| SetMACAddress | SwInterfaceSetMacAddress | |
| GetMACAddress | SwInterfaceDump + filter | Extract from InterfaceDetails |
| GetStats | Stats API via shared memory | |
| ListInterfaces | SwInterfaceDump | Convert to InterfaceInfo |
| GetInterface | SwInterfaceDump + name filter | |
| BridgeAddPort | SwInterfaceSetL2Bridge (enable=true) | |
| BridgeDelPort | SwInterfaceSetL2Bridge (enable=false) | |
| BridgeSetSTP | BridgeDomainSetMacAge or similar | VPP STP support varies |
| SetupMirror | SpanEnableDisableL2 | |
| RemoveMirror | SpanEnableDisableL2 (enable=false) | |
| StartMonitor | WantInterfaceEvents | Async notifications |
| StopMonitor | WantInterfaceEvents (enable=false) | |
| Close | Release all clients | |

## Interface Naming

Ze uses short names (xe0, e1, loop0). VPP uses long names (TenGigabitEthernet3/0/0,
GigabitEthernet5/0/1, loop0).

The naming module maintains a bidirectional map:

| Direction | Lookup |
|-----------|--------|
| Ze to VPP | Short name → VPP SwIfIndex + VPP long name |
| VPP to Ze | SwIfIndex → ze short name |

Map populated at startup from SwInterfaceDump + LCP pair list. Updated on interface
create/delete. DPDK interface names mapped from PCI address (from vpp-1 config).

## LCP Pair Management

When LCP is enabled (vpp-1 config), creating a VPP interface also creates an LCP pair:
- VPP side: hardware interface (e.g., TenGigabitEthernet3/0/0)
- Linux side: TAP interface in configured netns

This is required because ze's BGP reactor binds TCP sockets on Linux interfaces. Without
LCP pairs, BGP cannot establish sessions over VPP-managed interfaces.

LCP pair created via GoVPP LinuxCP binapi (LcpItfPairAddDel).

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
- [ ] AC-1..AC-16 all demonstrated
- [ ] Wiring Test table complete
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
- [ ] Write learned summary to `plan/learned/NNN-vpp-4-iface.md`
- [ ] Summary included in commit
