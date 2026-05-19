# Spec: ipsec-2 -- VTI and XFRM Interfaces

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | ipsec-1 (soft) |
| Phase | - |
| Updated | 2026-05-19 |

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

Add VTI (Virtual Tunnel Interface) and XFRM interface types to Ze's iface component.
These are route-based IPsec tunnel interfaces where traffic is routed into the
interface and the kernel XFRM subsystem handles encryption/decryption transparently.

VTI is the older mechanism (Linux 3.6+), using XFRM marks to bind SAs to interfaces.
XFRM interfaces (Linux 4.19+) are the modern replacement, using if_id instead of marks,
and are the kernel-maintainer-recommended path forward. Both must be supported: VTI for
compatibility with existing deployments (the production `home.conf` uses VTI), XFRM
interfaces for new configurations.

The reference configuration from `../home.conf` shows a VTI bound to an IPsec peer:

| Field | Value |
|-------|-------|
| Interface name | `vti0` |
| Address | `FC00::0:0000:0400:2/112` (IPv6) |
| Description | Management Tunnel VTI |
| Bound to | `vpn ipsec site-to-site peer management-bridge` via `vti { bind vti0 }` |

VTI and XFRM are semantically different from GRE/IPIP tunnels: they have no explicit
remote endpoint in the interface config (the SA peer is defined in IPsec config) and
they are bound to the kernel XFRM subsystem via mark or if_id. This is why they are
separate interface types and NOT additional TunnelKind values.

## Required Reading

### Architecture Docs
- [ ] `docs/features/interfaces.md` -- interface capability table, tunnel/wireguard patterns
  -> Decision: VTI/XFRM follow the WireguardSpec pattern (separate spec struct, separate Backend method)
  -> Constraint: new interface kinds must extend Backend interface and register in YANG schema
- [ ] `docs/architecture/core-design.md` -- component lifecycle, registration pattern
  -> Constraint: parsing follows existing parse*Entry pattern; reconciliation follows apply* pattern
- [ ] `plan/learned/557-iface-tunnel.md` -- tunnel implementation lessons
  -> Decision: recreate-on-reconcile for VTI/XFRM parameter changes, matching tunnel pattern
  -> Constraint: YANG choice/case flattening already works; VTI/XFRM can use it if needed
- [ ] `plan/learned/567-iface-tunnel-mac-per-case.md` -- L3 tunnels have no MAC
  -> Constraint: VTI and XFRM are L3 interfaces with no MAC address (use interface-common, not interface-l2)

**Key insights:**
- VTI is NOT a TunnelKind. Tunnels have explicit local/remote endpoints; VTI has a mark.
- XFRM is NOT a TunnelKind. XFRM has if_id and a parent dev; no endpoints.
- Both follow the WireguardSpec pattern: separate type, separate Backend method, separate YANG list.
- L3 only: no MAC address leaf (use interface-common grouping, not interface-l2).
- Recreate-on-reconcile: VTI/XFRM netlink parameters cannot be changed in place.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/backend.go` -- Backend interface: CreateTunnel(TunnelSpec), CreateWireguardDevice(name), ConfigureWireguardDevice(WireguardSpec). No VTI or XFRM methods exist.
  -> Constraint: new methods must follow existing signature pattern (take spec struct, return error)
- [ ] `internal/component/iface/tunnel.go` -- TunnelKind enum (8 values), TunnelSpec struct with *Set boolean sentinels. tunnelKindNames/tunnelKindByName maps. ParseTunnelKind function.
  -> Decision: VTI/XFRM are NOT added to TunnelKind. Separate types.
- [ ] `internal/component/iface/wireguard.go` -- WireguardSpec with Name, PrivateKey, ListenPort, Peers. WireguardPeerSpec. Uses wgctrl types. Separate from tunnel types.
  -> Decision: VTISpec and XFRMSpec follow the same standalone-struct pattern
- [ ] `internal/component/iface/config.go` -- ifaceConfig struct has Tunnel []tunnelEntry, Wireguard []wireguardEntry. parseTunnelEntry and parseWireguardEntry parse from YANG tree. applyTunnels and applyWireguards reconcile with recreate-on-change.
  -> Constraint: add VTI []vtiEntry and XFRM []xfrmEntry to ifaceConfig. Add parseVTIEntry, parseXFRMEntry, applyVTIs, applyXFRMs following identical patterns.
- [ ] `internal/component/iface/discover.go` -- zeType constants (ethernet, bridge, veth, dummy, loopback, tunnel, wireguard). kernelTunnelKinds map. infoToZeType maps kernel link type string to zeType.
  -> Constraint: add zeTypeVTI = "vti", zeTypeXFRM = "xfrm". Map kernel types "vti", "ip_vti", "ip6_vti", "xfrm" to the new zeTypes. Add to SupportedTypes().
- [ ] `internal/plugins/ifacenetlink/tunnel_linux.go` -- per-kind builder using vishvananda/netlink Gretun/Gretap/Iptun/Sittun/Ip6tnl types. LinkAdd for creation.
  -> Decision: VTI uses netlink Vti (ip_vti) or Ip6tnl with vti mode. XFRM uses netlink Xfrmi.
- [ ] `internal/plugins/ifacenetlink/backend_linux.go` -- implements all Backend methods. Each create method calls netlink.LinkAdd.
  -> Constraint: CreateVTI and CreateXFRM follow same pattern (build link struct, LinkAdd)
- [ ] `internal/plugins/ifacenetlink/backend_other.go` -- stubBackend returns "not supported on GOOS" for every method
  -> Constraint: CreateVTI and CreateXFRM must be stubbed here too

**Behavior to preserve:**
- All existing Backend interface methods unchanged
- Tunnel parsing and reconciliation unchanged
- WireGuard parsing and reconciliation unchanged
- Interface discovery for all existing types unchanged
- SupportedTypes() must continue to return all existing types

**Behavior to change:**
- Backend interface extended with CreateVTI(VTISpec) and CreateXFRM(XFRMSpec)
- ifaceConfig gains VTI and XFRM entry slices
- YANG schema gains `list vti` and `list xfrm` under interface container
- discover.go gains zeTypeVTI, zeTypeXFRM, and kernel type mapping
- SupportedTypes() includes "vti" and "xfrm"
- docs/features/interfaces.md capability table updated

## Data Flow (MANDATORY)

### Entry Point
- Config load/reload: YANG tree contains `interface { vti <name> { ... } }` or `interface { xfrm <name> { ... } }`
- `ze init`: discovers existing VTI/XFRM kernel netdevs and generates config

### Transformation Path
1. Config parser walks YANG tree, finds `vti`/`xfrm` list entries
2. `parseVTIEntry` / `parseXFRMEntry` extracts fields into VTISpec / XFRMSpec
3. Parsed entries stored in ifaceConfig.VTI / ifaceConfig.XFRM
4. `applyVTIs` / `applyXFRMs` compares desired spec to previous spec (by name)
5. Unchanged entries: skip creation, reconcile MTU/addresses in later phases
6. Changed/new entries: Backend.CreateVTI(spec) / Backend.CreateXFRM(spec) via netlink
7. Removed entries: deleted by Phase 4 deletion of unmanaged kinds (existing mechanism)
8. Addresses assigned via existing unit model (phases 2-3 of config_apply.go)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG tree to Go struct | parseVTIEntry / parseXFRMEntry | [ ] |
| Go struct to kernel netdev | Backend.CreateVTI / CreateXFRM via netlink LinkAdd | [ ] |
| Kernel netdev to Ze discovery | infoToZeType maps kernel "vti"/"ip_vti"/"ip6_vti"/"xfrm" | [ ] |
| IPsec config to VTI binding | ipsec-4 spec resolves `vti { bind <name> }` to XFRM mark/if_id | [ ] |

### Integration Points
- `iface.Backend` -- new CreateVTI, CreateXFRM methods
- `iface.ifaceConfig` -- new VTI, XFRM entry slices
- `iface.SupportedTypes()` -- includes "vti" and "xfrm"
- `iface.DiscoverInterfaces()` -- classifies VTI/XFRM kernel netdevs
- `config_apply.go` -- reconciliation phases apply to VTI/XFRM entries
- `spec-ipsec-4-strongswan.md` -- strongSwan binds IPsec SA to VTI (mark) or XFRM (if_id)

### Architectural Verification
- [ ] No bypassed layers (VTI/XFRM created via Backend, not raw netlink from ipsec component)
- [ ] No unintended coupling (VTI/XFRM are pure interface types; no knowledge of IPsec config)
- [ ] No duplicated functionality (uses existing unit model for addresses, existing Phase 4 for deletion)
- [ ] Zero-copy preserved where applicable (config parsing uses tree walker, no intermediate copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config load with `interface { vti vti0 { ... } }` | -> | parseVTIEntry produces VTISpec, applyVTIs calls Backend.CreateVTI | `test/reload/test-tx-iface-vti-create.ci` |
| Config load with `interface { xfrm xfrm0 { ... } }` | -> | parseXFRMEntry produces XFRMSpec, applyXFRMs calls Backend.CreateXFRM | `test/reload/test-tx-iface-xfrm-create.ci` |
| Config remove VTI entry and reload | -> | Phase 4 deletes unmanaged VTI netdev | `test/reload/test-tx-iface-vti-remove.ci` |
| `show interface` with VTI present | -> | VTI appears in interface list with type "vti" | `test/reload/test-tx-iface-vti-create.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config has `interface { vti vti0 { ... } }` with IPv4 local/remote | VTI netdev created via netlink as type `ip_vti` with correct local, remote, and key (mark) |
| AC-2 | Config has `interface { vti vti0 { ... } }` with IPv6 local/remote | VTI netdev created as type `ip6_vti` based on address family of local/remote |
| AC-3 | VTI config includes `key 42` | XFRM mark set to 42 on the VTI interface (o_key and i_key in netlink) |
| AC-4 | VTI config includes unit with addresses | Addresses assigned via existing unit model (same as tunnel/wireguard) |
| AC-5 | Config has `interface { xfrm xfrm0 { if-id 42 dev pppoe0 } }` | XFRM interface created via netlink with if_id=42 bound to parent dev pppoe0 |
| AC-6 | XFRM config omits `dev` leaf | XFRM interface created without parent dev binding (unbound, uses routing table) |
| AC-7 | Config reload changes VTI key from 42 to 99 | VTI netdev deleted and recreated with new key. Addresses re-applied. |
| AC-8 | Config reload removes a VTI entry | Phase 4 deletes the unmanaged VTI netdev (existing deletion mechanism) |
| AC-9 | `ze init` on a system with existing VTI and XFRM netdevs | Discovered interfaces include VTI and XFRM with correct ze type classification |
| AC-10 | `show interface` with VTI and XFRM present | Both appear in output with type "vti" / "xfrm" and correct address/state |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseVTIEntry` | `internal/component/iface/config_test.go` | VTI config parsing from YANG tree (name, local, remote, key, description) | |
| `TestParseVTIEntryIPv6` | `internal/component/iface/config_test.go` | VTI with IPv6 local/remote parsed correctly, v6 flag set | |
| `TestParseVTIEntryNoKey` | `internal/component/iface/config_test.go` | VTI without key leaf: key defaults to 0 (no mark) | |
| `TestParseXFRMEntry` | `internal/component/iface/config_test.go` | XFRM config parsing (name, if-id, dev, description) | |
| `TestParseXFRMEntryNoDev` | `internal/component/iface/config_test.go` | XFRM without dev leaf: unbound interface | |
| `TestParseXFRMEntryMissingIfId` | `internal/component/iface/config_test.go` | XFRM without if-id rejected (mandatory field) | |
| `TestVTISpecEqual` | `internal/component/iface/config_test.go` | Spec equality for reconciliation (same = no recreate) | |
| `TestXFRMSpecEqual` | `internal/component/iface/config_test.go` | Spec equality for reconciliation | |
| `TestDiscoverVTI` | `internal/component/iface/discover_test.go` | Kernel type "vti" / "ip_vti" / "ip6_vti" maps to zeTypeVTI | |
| `TestDiscoverXFRM` | `internal/component/iface/discover_test.go` | Kernel type "xfrm" maps to zeTypeXFRM | |
| `TestSupportedTypesIncludesVTIXFRM` | `internal/component/iface/discover_test.go` | SupportedTypes() includes "vti" and "xfrm" | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| VTI key (XFRM mark) | 0-4294967295 | 4294967295 (uint32 max) | N/A (0 is valid, means no mark) | 4294967296 |
| XFRM if_id | 1-4294967295 | 4294967295 (uint32 max) | 0 (invalid, 0 means unset) | 4294967296 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-tx-iface-vti-create` | `test/reload/test-tx-iface-vti-create.ci` | Config with VTI entry loaded, VTI netdev created, address assigned | |
| `test-tx-iface-xfrm-create` | `test/reload/test-tx-iface-xfrm-create.ci` | Config with XFRM entry loaded, XFRM netdev created | |
| `test-tx-iface-vti-remove` | `test/reload/test-tx-iface-vti-remove.ci` | VTI entry removed from config, netdev deleted on reload | |
| `iface-vti-invalid-missing-local` | `test/parse/iface-vti-invalid-missing-local.ci` | VTI config without local address rejected by parser | |
| `iface-xfrm-invalid-missing-ifid` | `test/parse/iface-xfrm-invalid-missing-ifid.ci` | XFRM config without if-id rejected by parser | |

## Files to Modify
- `internal/component/iface/backend.go` -- add CreateVTI(VTISpec) error and CreateXFRM(XFRMSpec) error to Backend interface
- `internal/component/iface/config.go` -- add vtiEntry, xfrmEntry types; parseVTIEntry, parseXFRMEntry; applyVTIs, applyXFRMs; vtiSpecEqual, xfrmSpecEqual
- `internal/component/iface/discover.go` -- add zeTypeVTI, zeTypeXFRM constants; extend kernelTunnelKinds or add kernelVTIKinds/kernelXFRMKinds; extend infoToZeType; extend SupportedTypes
- `internal/component/iface/register.go` -- register VTI/XFRM section parsing in parseIfaceSections
- `internal/component/iface/schema/ze-iface-conf.yang` -- add `list vti` and `list xfrm` under interface container with appropriate groupings
- `internal/plugins/ifacenetlink/backend_linux.go` -- implement CreateVTI, CreateXFRM
- `internal/plugins/ifacenetlink/backend_other.go` -- stub CreateVTI, CreateXFRM returning unsupported
- `internal/component/iface/config_test.go` -- add VTI/XFRM parsing unit tests
- `internal/component/iface/discover_test.go` -- add VTI/XFRM discovery tests
- `docs/features/interfaces.md` -- update capability table: VTI "have", XFRM "have"

## Files to Create
- `internal/component/iface/vti.go` -- VTISpec struct (Name, LocalAddress, RemoteAddress, Key uint32, KeySet bool, IsIPv6 bool)
- `internal/component/iface/xfrm.go` -- XFRMSpec struct (Name, IfID uint32, PhysicalDev string)
- `internal/plugins/ifacenetlink/vti_linux.go` -- CreateVTI implementation: build netlink.Vti (ipv4) or netlink.Ip6tnl in VTI mode (ipv6), set IKey/OKey from spec.Key, LinkAdd
- `internal/plugins/ifacenetlink/xfrm_linux.go` -- CreateXFRM implementation: build netlink.Xfrmi with Ifid from spec.IfID and ParentIndex from spec.PhysicalDev, LinkAdd
- `test/reload/test-tx-iface-vti-create.ci` -- functional test: VTI creation on config load
- `test/reload/test-tx-iface-xfrm-create.ci` -- functional test: XFRM creation on config load
- `test/reload/test-tx-iface-vti-remove.ci` -- functional test: VTI deletion on config reload
- `test/parse/iface-vti-invalid-missing-local.ci` -- parser rejection test
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

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- register entry points, write failing wiring tests
   - Tests: `test/reload/test-tx-iface-vti-create.ci`, `test/reload/test-tx-iface-xfrm-create.ci`
   - Files: `vti.go`, `xfrm.go` (spec structs), `backend.go` (interface extension), `register.go` (section parsing)
   - Verify: entry point exists; wiring test fails because create method is a stub

2. **Phase: VTI Spec and Parsing** -- VTISpec type, parseVTIEntry, YANG list
   - Tests: `TestParseVTIEntry`, `TestParseVTIEntryIPv6`, `TestParseVTIEntryNoKey`
   - Files: `vti.go`, `config.go` (parseVTIEntry, vtiEntry), `ze-iface-conf.yang` (list vti)
   - Verify: tests fail -> implement -> tests pass

3. **Phase: XFRM Spec and Parsing** -- XFRMSpec type, parseXFRMEntry, YANG list
   - Tests: `TestParseXFRMEntry`, `TestParseXFRMEntryNoDev`, `TestParseXFRMEntryMissingIfId`
   - Files: `xfrm.go`, `config.go` (parseXFRMEntry, xfrmEntry), `ze-iface-conf.yang` (list xfrm)
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Netlink Backend** -- CreateVTI and CreateXFRM implementations
   - Tests: integration tests in `internal/plugins/ifacenetlink/`
   - Files: `vti_linux.go`, `xfrm_linux.go`, `backend_linux.go`, `backend_other.go`
   - Verify: netlink creation produces correct kernel netdev types

5. **Phase: Reconciliation** -- applyVTIs, applyXFRMs, spec equality checks
   - Tests: `TestVTISpecEqual`, `TestXFRMSpecEqual`, `test-tx-iface-vti-remove.ci`
   - Files: `config.go` (applyVTIs, applyXFRMs, equality), `config_apply.go` (call sites)
   - Verify: unchanged specs skip creation; changed specs recreate; removed entries deleted

6. **Phase: Discovery** -- kernel type classification for VTI and XFRM
   - Tests: `TestDiscoverVTI`, `TestDiscoverXFRM`, `TestSupportedTypesIncludesVTIXFRM`
   - Files: `discover.go` (zeType constants, infoToZeType extension)
   - Verify: `ze init` on a system with VTI/XFRM classifies them correctly

7. **Functional tests** -- Create after feature works
8. **Full verification** -- `make ze-verify`
9. **Complete spec** -- Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-10 has implementation with file:line |
| Correctness | VTI ipv4 vs ipv6 auto-detection correct. XFRM if_id=0 rejected. |
| Naming | zeTypeVTI = "vti", zeTypeXFRM = "xfrm". YANG list keys use kebab-case. |
| Data flow | VTI/XFRM created via Backend only, never raw netlink from ipsec component |
| Rule: no-layering | VTI/XFRM are NOT added to TunnelKind enum |
| Rule: derive-not-hardcode | SupportedTypes() includes VTI/XFRM from constants, not hardcoded list |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| VTISpec type exists | `grep -rn 'type VTISpec struct' internal/component/iface/` |
| XFRMSpec type exists | `grep -rn 'type XFRMSpec struct' internal/component/iface/` |
| Backend.CreateVTI method | `grep -rn 'CreateVTI' internal/component/iface/backend.go` |
| Backend.CreateXFRM method | `grep -rn 'CreateXFRM' internal/component/iface/backend.go` |
| Netlink VTI creation | `grep -rn 'CreateVTI' internal/plugins/ifacenetlink/vti_linux.go` |
| Netlink XFRM creation | `grep -rn 'CreateXFRM' internal/plugins/ifacenetlink/xfrm_linux.go` |
| YANG list vti | `grep -rn 'list vti' internal/component/iface/schema/` |
| YANG list xfrm | `grep -rn 'list xfrm' internal/component/iface/schema/` |
| VTI in SupportedTypes | `grep -rn 'zeTypeVTI' internal/component/iface/discover.go` |
| Functional test exists | `ls test/reload/test-tx-iface-vti-create.ci` |
| docs updated | `grep -n 'VTI' docs/features/interfaces.md` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | VTI key must be valid uint32. XFRM if_id must be non-zero uint32. Interface names validated by existing iface name validator. |
| Resource exhaustion | Unbounded VTI/XFRM creation limited by kernel netdev count (same as tunnels). No Ze-side limit needed beyond kernel. |
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
- [ ] AC-1..AC-10 all demonstrated
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
