# Spec: iface-bridge-vlan-filtering

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release: VLAN-aware bridges (VyOS `enable-vlan`, per-member allowed and native VLANs). Non-VLAN-aware bridges stay shipped and required |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An operator must be able to make a Ze bridge VLAN-aware: per member, the
allowed (tagged) VLANs and the native (untagged, PVID) VLAN, and a VLAN
interface on the bridge for each routed VLAN. VyOS has it (`interfaces bridge
<br> enable-vlan`, `member interface <if> allowed-vlan`, `native-vlan`). Ze's
bridge has no VLAN filtering, access or trunk mode, which the NetBox umbrella
also records (`plan/post-release/features/spec-netbox-0-umbrella.md`: "Ze has
no bridge VLAN filtering or QinQ model"). The anycast gateway case of
`spec-iface-macvlan-kind.md` (AC-6) needs it.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/iface/logical-name-resolution.md` - bridge members are enslaved in Phase 2a on resolved devices
  → Constraint: VLAN membership is set on the resolved member device, after enslave

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/iface/netlink/bridge_linux.go` - `BridgeAddPort`, `BridgeDelPort`, `BridgeSetSTP` (IFLA_BR_STP_STATE over rtnetlink); no IFLA_BR_VLAN_FILTERING, no per-port VLAN entries
- [ ] `internal/component/iface/config.go` - `bridgeEntry{STP, Members}`
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - `list bridge` with `stp` and members; no VLAN leaves

**Behavior to preserve:**
- Non-VLAN-aware bridges, STP, member enslave.

**Behavior to change:**
- A VLAN-filtering switch on the bridge, per-member allowed and native VLANs, and bridge VLAN self entries for routed VLANs.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Bridge STP over rtnetlink | `bridge_linux.go` `BridgeSetSTP`, `bridgeSTPRequest` | Sets IFLA_BR_STP_STATE by changelink, never sysfs | Extends: IFLA_BR_VLAN_FILTERING uses the same changelink request shape |
| Bridge member enslave | `BridgeAddPort`, Phase 2a | Enslaves resolved members | Extends: VLAN entries follow the enslave on the same device |
| VLAN units on a bridge | the unit loop (`CreateVLAN`) | Creates `br0.<id>` 802.1Q uppers | Extends: on a VLAN-aware bridge, a routed VLAN also needs the bridge self VLAN entry |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: a VLAN-aware switch on `interface bridge <name>`, allowed and native VLANs per member (shape settled at design).

### Transformation Path
1. Verify: VLAN ids 1..4094; native VLAN in range; a member's VLANs only when the bridge is VLAN-aware.
2. Apply: set VLAN filtering; per member, add allowed VLANs tagged and the native VLAN as PVID untagged; remove VLAN 1 default where not configured; add self entries for routed units.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| iface to kernel | rtnetlink IFLA_BR_VLAN_FILTERING; RTM_SETLINK AF_BRIDGE with IFLA_BRIDGE_VLAN_INFO | No |

### Changed-Wire Census

**Search evidence:** owed at design.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [format] | [role] | [source site] | [finding] | [disposition] |

### Integration Points
- `bridge_linux.go`, the Phase 2a member loop.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A VLAN-aware bridge with one trunk and one access member committed | → | VLAN filtering and per-port VLAN entries | `test/plugin/iface-bridge-vlan-filtering.ci` (`option=needs-linux`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | VLAN-aware br0, eth1 allowed 10,20, eth2 native 10 | The kernel bridge has VLAN filtering on; eth1 carries 10 and 20 tagged; eth2 has PVID 10 untagged; a frame tagged 30 on eth1 is dropped | None |
| AC-2 | Routed unit vlan 10 on br0 | br0 itself is a member of VLAN 10 (self), and the unit's address answers hosts on VLAN 10 | None |
| AC-3 | VLAN filtering removed | Bridge forwards as before, per-port VLAN entries gone | None |
| AC-4 | A member VLAN list on a non-VLAN-aware bridge | Verify refuses naming the leaf | None |
| AC-5 | `backend vpp` | Commit refused with the YANG path, unless VPP's bridge-domain VLAN tag rewrite can match exactly | None |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Builds a trunk and an access port | config, bridge apply | `test/plugin/iface-bridge-vlan-filtering.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBridgeVLANVerify` | `internal/component/iface/config_bridge_vlan_test.go` | AC-4, id range | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| VLAN id | 1..4094 | 4094 | 0 | 4095 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `iface-bridge-vlan-filtering` | `test/plugin/` | AC-1, AC-2, AC-3 | |

## Files to Modify
- `internal/component/iface/yang/ze-iface-conf.yang`, `internal/component/iface/config.go`, `internal/plugins/iface/netlink/bridge_linux.go`

## Files to Create
- `test/plugin/iface-bridge-vlan-filtering.ci`

## Implementation Steps

1. **Phase: Wiring** - leaves, parse, failing `.ci`.
2. **Phase: Netlink** - filtering switch, per-port entries, self entries.

Proof: `./le test unit plugins`, `./le test functional plugin` for the named `.ci` file, `./le test qemu all`.

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- 802.1ad bridge VLAN protocol is not in this spec.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason): N-A, no protocol peer
