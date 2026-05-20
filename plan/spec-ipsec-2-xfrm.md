# Spec: ipsec-2 -- XFRM Interfaces

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | ipsec-1 (soft) |
| Phase | 1/8 |
| Updated | 2026-05-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `docs/features/interfaces.md` -- interface capability table, tunnel/wireguard patterns
4. `internal/component/iface/backend.go` -- Backend interface (33 methods)
5. `internal/component/iface/tunnel.go` -- TunnelKind/TunnelSpec pattern
6. `internal/component/iface/wireguard.go` -- WireguardSpec pattern
7. `internal/component/iface/config.go` -- parse*Entry, apply* reconciliation pattern
8. `internal/component/iface/discover.go` -- zeType* constants, infoToZeType mapping
9. `spec-ipsec-0-umbrella.md` -- umbrella design decisions

## Task

Add XFRM interface type to Ze's iface component. XFRM interfaces (Linux 4.19+) are
the modern kernel mechanism for route-based IPsec: traffic is routed into the interface
and the kernel XFRM subsystem handles encryption/decryption transparently. They use
`if_id` to bind security associations to the interface.

VTI (the older mechanism using XFRM marks) is deliberately not supported. XFRM
interfaces are the kernel-maintainer-recommended replacement and there is no value in
carrying the legacy mechanism.

XFRM interfaces are semantically different from GRE/IPIP tunnels: they have no explicit
remote endpoint in the interface config (the SA peer is defined in IPsec config) and
they are bound to the kernel XFRM subsystem via if_id. This is why they are a separate
interface type and NOT an additional TunnelKind value.

**Unmanaged XFRM interfaces:** XFRM netdevs created outside Ze (by strongSwan, manual
`ip link add`, etc.) are discovered at startup and visible in operational mode commands
(`show interface`), but Ze MUST NOT modify or delete them. They MUST NOT appear in the
configuration tree (running config, candidate config, config diff). Only XFRM interfaces
declared in Ze's config are managed (created, reconciled, deleted) and appear in config.

**XFRM interface detail via netlink:** For all discovered XFRM interfaces (managed or
not), Ze queries netlink for:
- **if_id:** the XFRM interface identifier (from the Xfrmi link attributes)
- **IP addresses:** addresses assigned to the interface (standard netlink addr list)
- **XFRM policies:** policies bound to this interface's if_id (netlink XfrmPolicyList
  filtered by IfId)

This information is displayed in `show interface <name>` output, giving operators
visibility into the IPsec binding even when Ze did not create the interface.

## Required Reading

### Architecture Docs
- [ ] `docs/features/interfaces.md` -- interface capability table, tunnel/wireguard patterns
  -> Decision: XFRM follows the WireguardSpec pattern (separate spec struct, separate Backend method)
  -> Constraint: new interface kinds must extend Backend interface and register in YANG schema
- [ ] `docs/architecture/core-design.md` -- component lifecycle, registration pattern
  -> Constraint: parsing follows existing parse*Entry pattern; reconciliation follows apply* pattern
- [ ] `plan/learned/557-iface-tunnel.md` -- tunnel implementation lessons
  -> Decision: recreate-on-reconcile for XFRM parameter changes, matching tunnel pattern
  -> Constraint: YANG choice/case flattening already works; XFRM can use it if needed
- [ ] `plan/learned/567-iface-tunnel-mac-per-case.md` -- L3 tunnels have no MAC
  -> Constraint: XFRM is an L3 interface with no MAC address (use interface-common, not interface-l2)

**Key insights:**
- XFRM is NOT a TunnelKind. XFRM has if_id and a parent dev; no endpoints.
- Follows the WireguardSpec pattern: separate type, separate Backend method, separate YANG list.
- L3 only: no MAC address leaf (use interface-common grouping, not interface-l2).
- Recreate-on-reconcile: XFRM netlink parameters cannot be changed in place.
- VTI deliberately not supported. XFRM interfaces are the modern replacement.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/backend.go` -- Backend interface: CreateTunnel(TunnelSpec), CreateWireguardDevice(name), ConfigureWireguardDevice(WireguardSpec). No XFRM methods exist.
  -> Constraint: new methods must follow existing signature pattern (take spec struct, return error)
  -> Constraint: add both CreateXFRM(XFRMSpec) and GetXFRMInfo(name) to Backend. GetXFRMInfo returns if_id and associated XFRM policies for display.
- [ ] `internal/component/iface/tunnel.go` -- TunnelKind enum (8 values), TunnelSpec struct with *Set boolean sentinels. tunnelKindNames/tunnelKindByName maps. ParseTunnelKind function.
  -> Decision: XFRM is NOT added to TunnelKind. Separate type.
- [ ] `internal/component/iface/wireguard.go` -- WireguardSpec with Name, PrivateKey, ListenPort, Peers. WireguardPeerSpec. Uses wgctrl types. Separate from tunnel types.
  -> Decision: XFRMSpec follows the same standalone-struct pattern
- [ ] `internal/component/iface/config.go` -- ifaceConfig struct has Tunnel []tunnelEntry, Wireguard []wireguardEntry. parseTunnelEntry and parseWireguardEntry parse from YANG tree. applyTunnels and applyWireguards reconcile with recreate-on-change.
  -> Constraint: add XFRM []xfrmEntry to ifaceConfig. Add parseXFRMEntry, applyXFRMs following identical patterns.
- [ ] `internal/component/iface/discover.go` -- zeType constants (ethernet, bridge, veth, dummy, loopback, tunnel, wireguard). kernelTunnelKinds map. infoToZeType maps kernel link type string to zeType.
  -> Constraint: add zeTypeXFRM = "xfrm". Map kernel type "xfrm" to the new zeType. Add to SupportedTypes().
- [ ] `internal/plugins/ifacenetlink/tunnel_linux.go` -- per-kind builder using vishvananda/netlink Gretun/Gretap/Iptun/Sittun/Ip6tnl types. LinkAdd for creation.
  -> Decision: XFRM uses netlink Xfrmi.
- [ ] `internal/plugins/ifacenetlink/backend_linux.go` -- implements all Backend methods. Each create method calls netlink.LinkAdd.
  -> Constraint: CreateXFRM follows same pattern (build link struct, LinkAdd)
- [ ] `internal/plugins/ifacenetlink/backend_other.go` -- stubBackend returns "not supported on GOOS" for every method
  -> Constraint: CreateXFRM must be stubbed here too

**Behavior to preserve:**
- All existing Backend interface methods unchanged
- Tunnel parsing and reconciliation unchanged
- WireGuard parsing and reconciliation unchanged
- Interface discovery for all existing types unchanged
- SupportedTypes() must continue to return all existing types

**Behavior to change:**
- Backend interface extended with CreateXFRM(XFRMSpec) and GetXFRMInfo(name string) (XFRMInfo, error)
- XFRMInfo struct: IfID uint32, Policies []XFRMPolicyInfo (src/dst selectors, dir, proto, mode)
- ifaceConfig gains XFRM entry slice
- YANG schema gains `list xfrm` under interface container
- discover.go gains zeTypeXFRM and kernel type mapping
- SupportedTypes() includes "xfrm"
- Startup discovers existing XFRM kernel netdevs (not `ze init`)
- `show interface <xfrm-name>` displays if_id, IP addresses, and XFRM policies from netlink
- docs/features/interfaces.md capability table updated

## Data Flow (MANDATORY)

### Entry Point
- Config load/reload: YANG tree contains `interface { xfrm <name> { ... } }`
- Ze startup: discovers existing XFRM kernel netdevs and classifies them

### Transformation Path
1. Config parser walks YANG tree, finds `xfrm` list entries
2. `parseXFRMEntry` extracts fields into XFRMSpec
3. Parsed entries stored in ifaceConfig.XFRM
4. `applyXFRMs` compares desired spec to previous spec (by name)
5. Unchanged entries: skip creation, reconcile MTU/addresses in later phases
6. Changed/new entries: Backend.CreateXFRM(spec) via netlink
7. Removed entries: deleted by Phase 4 deletion of unmanaged kinds (existing mechanism)
8. Addresses assigned via existing unit model (phases 2-3 of config_apply.go)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG tree to Go struct | parseXFRMEntry | [ ] |
| Go struct to kernel netdev | Backend.CreateXFRM via netlink LinkAdd | [ ] |
| Kernel netdev to Ze discovery | infoToZeType maps kernel "xfrm" at startup | [ ] |
| IPsec config to XFRM binding | ipsec-4 spec resolves XFRM if_id | [ ] |

### Integration Points
- `iface.Backend` -- new CreateXFRM method
- `iface.ifaceConfig` -- new XFRM entry slice
- `iface.SupportedTypes()` -- includes "xfrm"
- `iface.DiscoverInterfaces()` -- classifies XFRM kernel netdevs at startup
- `config_apply.go` -- reconciliation phases apply to XFRM entries
- `spec-ipsec-8-ikev2-child-xfrm.md` -- IKE engine binds Child SA to XFRM interface (if_id)

### Architectural Verification
- [ ] No bypassed layers (XFRM created via Backend, not raw netlink from ipsec component)
- [ ] No unintended coupling (XFRM is a pure interface type; no knowledge of IPsec config)
- [ ] No duplicated functionality (uses existing unit model for addresses, existing Phase 4 for deletion)
- [ ] Zero-copy preserved where applicable (config parsing uses tree walker, no intermediate copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config load with `interface { xfrm xfrm0 { ... } }` | -> | parseXFRMEntry produces XFRMSpec, applyXFRMs calls Backend.CreateXFRM | `test/reload/test-tx-iface-xfrm-create.ci` |
| Config remove XFRM entry and reload | -> | Phase 4 deletes unmanaged XFRM netdev | `test/reload/test-tx-iface-xfrm-remove.ci` |
| `show interface` with XFRM present | -> | XFRM appears in interface list with type "xfrm" | `test/reload/test-tx-iface-xfrm-create.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config has `interface { xfrm xfrm0 { if-id 42 dev eth0 } }` | XFRM interface created via netlink with if_id=42 bound to parent dev eth0 |
| AC-2 | XFRM config omits `dev` leaf | XFRM interface created without parent dev binding (unbound, uses routing table) |
| AC-3 | XFRM config includes unit with addresses | Addresses assigned via existing unit model (same as tunnel/wireguard) |
| AC-4 | Config reload changes XFRM if-id from 42 to 99 | XFRM netdev deleted and recreated with new if_id. Addresses re-applied. |
| AC-5 | Config reload removes an XFRM entry | Phase 4 deletes the unmanaged XFRM netdev (existing deletion mechanism) |
| AC-6 | Ze startup on a system with existing XFRM netdevs | Discovered interfaces include XFRM with correct ze type classification |
| AC-7 | `show interface` with XFRM present | XFRM appears in output with type "xfrm" and correct address/state |
| AC-8 | XFRM config without if-id | Rejected by parser (if-id is mandatory) |
| AC-9 | XFRM config with if-id=0 | Rejected by parser (0 means unset, invalid) |
| AC-10 | XFRM netdev exists in kernel but not in Ze config | Visible in `show interface` (operational mode) with type "xfrm", if_id, addresses, and policies. Not present in config tree. Not modified or deleted by Ze. |
| AC-11 | `show interface xfrm0` on any XFRM interface (managed or unmanaged) | Displays if_id, IP addresses, and XFRM policies (src/dst selectors, direction, protocol) queried from netlink |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseXFRMEntry` | `internal/component/iface/config_test.go` | XFRM config parsing (name, if-id, dev, description) | |
| `TestParseXFRMEntryNoDev` | `internal/component/iface/config_test.go` | XFRM without dev leaf: unbound interface | |
| `TestParseXFRMEntryMissingIfId` | `internal/component/iface/config_test.go` | XFRM without if-id rejected (mandatory field) | |
| `TestParseXFRMEntryZeroIfId` | `internal/component/iface/config_test.go` | XFRM with if-id=0 rejected (0 means unset) | |
| `TestXFRMSpecEqual` | `internal/component/iface/config_test.go` | Spec equality for reconciliation | |
| `TestDiscoverXFRM` | `internal/component/iface/discover_test.go` | Kernel type "xfrm" maps to zeTypeXFRM | |
| `TestSupportedTypesIncludesXFRM` | `internal/component/iface/discover_test.go` | SupportedTypes() includes "xfrm" | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| XFRM if_id | 1-4294967295 | 4294967295 (uint32 max) | 0 (invalid, 0 means unset) | 4294967296 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-tx-iface-xfrm-create` | `test/reload/test-tx-iface-xfrm-create.ci` | Config with XFRM entry loaded, XFRM netdev created | |
| `test-tx-iface-xfrm-remove` | `test/reload/test-tx-iface-xfrm-remove.ci` | XFRM entry removed from config, netdev deleted on reload | |
| `iface-xfrm-invalid-missing-ifid` | `test/parse/iface-xfrm-invalid-missing-ifid.ci` | XFRM config without if-id rejected by parser | |

## Files to Modify
- `internal/component/iface/backend.go` -- add CreateXFRM(XFRMSpec) error and GetXFRMInfo(name string) (XFRMInfo, error) to Backend interface
- `internal/component/iface/config.go` -- add xfrmEntry type; parseXFRMEntry; applyXFRMs; xfrmSpecEqual
- `internal/component/iface/discover.go` -- add zeTypeXFRM constant; extend infoToZeType; extend SupportedTypes
- `internal/component/iface/register.go` -- register XFRM section parsing in parseIfaceSections
- `internal/component/iface/schema/ze-iface-conf.yang` -- add `list xfrm` under interface container
- `internal/plugins/ifacenetlink/backend_linux.go` -- implement CreateXFRM, GetXFRMInfo
- `internal/plugins/ifacenetlink/backend_other.go` -- stub CreateXFRM, GetXFRMInfo returning unsupported
- `internal/component/iface/config_test.go` -- add XFRM parsing unit tests
- `internal/component/iface/discover_test.go` -- add XFRM discovery tests
- `docs/features/interfaces.md` -- update capability table: XFRM "have"

## Files to Create
- `internal/component/iface/xfrm.go` -- XFRMSpec struct (Name, IfID uint32, PhysicalDev string); XFRMInfo struct (IfID uint32, Policies []XFRMPolicyInfo); XFRMPolicyInfo struct (Src, Dst selector CIDRs, Dir, Proto, Mode)
- `internal/plugins/ifacenetlink/xfrm_linux.go` -- CreateXFRM: build netlink.Xfrmi with Ifid/ParentIndex, LinkAdd. GetXFRMInfo: read Xfrmi link attrs for if_id, call netlink.XfrmPolicyList and filter by IfId, return XFRMInfo.
- `test/reload/test-tx-iface-xfrm-create.ci` -- functional test: XFRM creation on config load
- `test/reload/test-tx-iface-xfrm-remove.ci` -- functional test: XFRM deletion on config reload
- `test/parse/iface-xfrm-invalid-missing-ifid.ci` -- parser rejection test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

**Note:** "ze startup" means the normal iface component startup path that calls
`DiscoverInterfaces()`, NOT `ze init`. XFRM netdevs present in the kernel at boot
are discovered and classified via the same `infoToZeType` mapping used for all
interface types during the standard startup discovery pass.

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- register entry points, write failing wiring tests
   - Tests: `test/reload/test-tx-iface-xfrm-create.ci`
   - Files: `xfrm.go` (spec struct), `backend.go` (interface extension), `register.go` (section parsing)
   - Verify: entry point exists; wiring test fails because create method is a stub

2. **Phase: XFRM Spec and Parsing** -- XFRMSpec type, parseXFRMEntry, YANG list
   - Tests: `TestParseXFRMEntry`, `TestParseXFRMEntryNoDev`, `TestParseXFRMEntryMissingIfId`, `TestParseXFRMEntryZeroIfId`
   - Files: `xfrm.go`, `config.go` (parseXFRMEntry, xfrmEntry), `ze-iface-conf.yang` (list xfrm)
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Netlink Backend** -- CreateXFRM implementation
   - Tests: integration tests in `internal/plugins/ifacenetlink/`
   - Files: `xfrm_linux.go`, `backend_linux.go`, `backend_other.go`
   - Verify: netlink creation produces correct kernel netdev type

4. **Phase: Reconciliation** -- applyXFRMs, spec equality check
   - Tests: `TestXFRMSpecEqual`, `test/reload/test-tx-iface-xfrm-remove.ci`
   - Files: `config.go` (applyXFRMs, equality), `config_apply.go` (call site)
   - Verify: unchanged specs skip creation; changed specs recreate; removed entries deleted

5. **Phase: Discovery** -- kernel type classification for XFRM at startup
   - Tests: `TestDiscoverXFRM`, `TestSupportedTypesIncludesXFRM`
   - Files: `discover.go` (zeType constant, infoToZeType extension)
   - Verify: Ze startup on a system with XFRM interfaces classifies them correctly

6. **Functional tests** -- Create after feature works
7. **Full verification** -- `make ze-verify`
8. **Complete spec** -- Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-11 has implementation with file:line |
| Correctness | XFRM if_id=0 rejected. Unbound (no dev) works. |
| Naming | zeTypeXFRM = "xfrm". YANG list key uses kebab-case. |
| Data flow | XFRM created via Backend only, never raw netlink from ipsec component |
| Rule: no-layering | XFRM is NOT added to TunnelKind enum |
| Rule: derive-not-hardcode | SupportedTypes() includes XFRM from constant, not hardcoded list |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| XFRMSpec type exists | `grep -rn 'type XFRMSpec struct' internal/component/iface/` |
| Backend.CreateXFRM method | `grep -rn 'CreateXFRM' internal/component/iface/backend.go` |
| Netlink XFRM creation | `grep -rn 'CreateXFRM' internal/plugins/ifacenetlink/xfrm_linux.go` |
| YANG list xfrm | `grep -rn 'list xfrm' internal/component/iface/schema/` |
| XFRM in SupportedTypes | `grep -rn 'zeTypeXFRM' internal/component/iface/discover.go` |
| Functional test exists | `ls test/reload/test-tx-iface-xfrm-create.ci` |
| docs updated | `grep -n 'XFRM' docs/features/interfaces.md` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | XFRM if_id must be non-zero uint32. Interface names validated by existing iface name validator. |
| Resource exhaustion | Unbounded XFRM creation limited by kernel netdev count (same as tunnels). No Ze-side limit needed beyond kernel. |
| Privilege | Netlink interface creation requires CAP_NET_ADMIN (same as existing tunnel creation). No new privilege requirements. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural then DESIGN phase |
| Functional test fails | Check AC; if AC wrong then DESIGN; if AC correct then IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] Summary included in commit
