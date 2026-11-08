# Spec: iface-bond-lacp

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release (link aggregation). No shipped behavior depends on it |
| RFC gaps | - |
| Depends | `plan/release/fixes/spec-iface-restore-permanent-mac.md` (a released member gets its permanent MAC back) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An operator must be able to aggregate ethernet ports into one bond interface,
with IEEE 802.3ad (802.1AX) LACP or a static mode, carrying addresses, units
(VLANs), MTU and every other interface setting, so that a router has a
redundant and wider uplink to a switch. VyOS offers `interfaces bonding`
with modes, hash policy, LACP rate, min-links and primary member. Ze has no
aggregation at all.

| # | Operator need |
|---|---------------|
| 1 | Create `bond0` from two or more ethernet entries, mode 802.3ad, LACP rate slow or fast, transmit hash policy layer2, layer2+3 or layer3+4 |
| 2 | Static modes: active-backup (with primary), balance-xor, broadcast, balance-rr |
| 3 | `min-links`: the bond stays down below N active members |
| 4 | Units, addresses, MTU and MAC on the bond, never on a member |
| 5 | `show interface bond0` lists members, their LACP state (actor and partner) and which are active |
| 6 | Add or remove a member without disturbing the bond MAC or the other members |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/iface/logical-name-resolution.md` - selectors and aggregators
  → Constraint: a bond wears a member's address, so `mac match` must exclude the bond and read membership from IFLA_MASTER (`devicesWithMAC`, `aggregatingDevices`); members stay selectable by their permanent address
  → Constraint: a member named by a selector is enslaved after the selector map exists (the bridge member pattern, Phase 2a), never by logical name
- [ ] `docs/architecture/core-design.md` section 14a - `ze:backend`
  → Constraint: a backend that cannot build a bond exactly refuses at commit

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - interface kinds are ethernet, dummy, veth, bridge, tunnel, wireguard, xfrm, pppoe-client, loopback; no bond or LAG kind
- [ ] `internal/component/iface/backend.go` - `Backend` has `CreateBridge`, `BridgeAddPort`, `BridgeDelPort`, `BridgeSetSTP`; no bond create or member call
- [ ] `internal/plugins/iface/netlink/bridge_linux.go` - `BridgeAddPort` enslaves by `LinkSetMaster` after checking the master is a bridge; `BridgeDelPort` by `LinkSetNoMaster`
- [ ] `internal/component/iface/config.go` - `bridgeEntry` embeds `ifaceEntry` and carries `Members`
- [ ] `internal/component/iface/config_apply.go` - Phase 2 entry loop sets MTU and MAC on every bound ethernet entry, then offloads and units
- [ ] `internal/component/iface/doctor_linux.go` - `checkEthernetInterfaces` reports missing, down and selector verdicts per ethernet entry

**Behavior to preserve:**
- Bridges and their members behave as today.
- `mac match` keeps excluding aggregators and stacked devices.

**Behavior to change:**
- A new bond kind with members, modes and LACP parameters, on the netlink backend, and on VPP when A-2 holds.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Bridge kind and member enslave | `bridgeEntry`, `BridgeAddPort`, Phase 2a member loop | Creates a bridge and enslaves resolved members | Extends the pattern: bond members are enslaved in the same phase on the resolved device. A shared "member of an aggregator" helper is a design question, not a second copy |
| Selector exclusion of aggregators | `config_apply.go` `devicesWithMAC`, `aggregatingDevices`; doctor `hasLowerDevice` | A bridge or bond wearing a member's MAC never answers that MAC | Extends: already covers a bond; this spec adds the test that proves it for a bond (VyOS `ccac4ac56`) |
| Backend capability gate | `ze:backend`, `validateBackendGate` | Refuses unsupported nodes at commit | Extends: the bond kind declares which backends implement it |
| VPP interface-in-use check | `plan/release/fixes/spec-vpp-interface-in-use.md` | Notes "no VPP bond" as a known limitation | Extends: a VPP bond member becomes a member role there, if A-2 holds |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: a new interface list for bonds (name settled at design: VyOS `bonding`, Linux `bond`) with a member leaf-list naming ethernet entries.

### Transformation Path
1. YANG parse and verify: a member is an ethernet entry; a member carries no address, unit, MTU, MAC override or DHCP; one member belongs to one aggregator (bond or bridge).
2. Phase 1: create the bond with its mode parameters.
3. Phase 2a: release removed members, then enslave new members on their resolved devices.
4. Phase 2: MTU, MAC, units, addresses on the bond.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| iface to kernel | rtnetlink bond link (IFLA_BOND_*), `LinkSetMaster` | No |
| iface to VPP | GoVPP bond and LACP API (A-2) | No |

### Changed-Wire Census

**Search evidence:** to do at design: `show interface` JSON gains bond fields; interface events carry the new kind.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [format] | [role] | [source site] | [finding] | [disposition] |

### Integration Points
- The bridge member phase; the owned-device and selector code.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | iface kinds are read by literal key in `parseIfaceConfig` (noted in `spec-traffic-ingress-shaping.md`); adding a kind there is the enumeration the principles forbid unless the design derives it |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Using the vendored `vishvananda/netlink` bond link type is a new use of an approved module and needs Thomas's approval before code (`ai/META-DECISIONS.md`, third-party module rule) | META-DECISIONS, "Code and dependencies" | Code written without approval | Ask Thomas at design, with what it is for | unvalidated |
| A-2 | VPP's bond and LACP plugins (`bond_create2`, `bond_add_member`, `lacp`) can build an 802.3ad bond exactly | VPP has a bond API | VPP refuses the kind | Read VPP source, quote the functions | unvalidated |
| A-3 | Recent kernels no longer copy the first member's MAC to the bond | VyOS T7571 and T9269 | Bond MAC handling differs | Integration test reads the bond MAC after the first enslave | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Enslaving the management NIC drops the session | SSH lost on commit | Commit confirm; doctor warns when a member carries the management address |
| R-2 | A member's own settings fight the bond (MTU, MAC) | Jumbo frames drop silently | AC-5, AC-6 |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Traffic over the aggregated uplink; a wrong MAC or MTU drops frames with no log |
| How is it reverted? | Remove the bond: members are released and get their own settings back |
| Who else touches this path? | Bridge membership, selectors, DHCP client on members, VRRP (macvlan parent), the MAC-restore fix spec |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config with a bond of two ethernet entries, mode 802.3ad | → | bond create and member enslave in the iface apply | `test/plugin/iface-bond-lacp-apply.ci` (`option=needs-linux`, two veth pairs, the far side a kernel bond in another netns) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | bond0, mode 802.3ad, lacp-rate fast, members eth1 eth2, far end a kernel 802.3ad bond | Both members show LACP state collecting and distributing; traffic passes; `show interface bond0` reports actor and partner state per member | None |
| AC-2 | Each static mode, with its options (primary for active-backup, hash policy for xor and 802.3ad) | The kernel bond reports the configured mode and options | None |
| AC-3 | min-links 2, one member link down | bond0 carrier down; restored when the member returns | None |
| AC-4 | An address, unit, MTU or MAC override configured on a member | Verify refuses, naming the member and the offending option in CLI spelling (for example `unit 10 vlan-id`), never an internal field name (VyOS T9454) | None |
| AC-5 | Bond MTU 9000; a member's ethernet entry is then deleted while the member stays in the bond | The member keeps MTU 9000 while enslaved; only after release does it return to its own or default MTU (VyOS `32f664077`) | None |
| AC-6 | A third member appended to a running bond | The bond MAC does not change (VyOS `395338fe0`) | None |
| AC-7 | bond0 MAC explicitly configured | The bond carries it, and every member sends with it | None |
| AC-8 | A member bound by `mac match` | Stays bound after enslave, by its permanent address; the bond never answers that selector; `ze doctor` does not report the member missing or ambiguous (VyOS `ccac4ac56`) | None |
| AC-9 | A member removed from the bond | It is released, its MTU and permanent MAC are restored, and the other members stay collecting | None |
| AC-10 | One ethernet entry named as a member of two aggregators (two bonds, or a bond and a bridge) | Verify refuses, naming both | None |
| AC-11 | Bond removed entirely | Bond deleted, every member released and restored | None |
| AC-12 | `backend vpp` | Either AC-1 to AC-11 hold on VPP (A-2), or the commit refuses the bond kind with the YANG path and the backend | None |
| AC-13 | Daemon restart with the same config | No member flaps out and back; the existing bond is adopted | None |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Builds an LACP uplink | config, verify, create, enslave, LACP | `test/plugin/iface-bond-lacp-apply.ci` |
| 2 | Puts an address on a member by mistake | verify refusal | `test/plugin/iface-bond-member-refused.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBondMemberOptionsRefused` | `internal/component/iface/config_bond_test.go` | AC-4, AC-10 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| min-links | 0..number of members | members | N/A | members + 1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `iface-bond-lacp-apply` | `test/plugin/` | AC-1, AC-3 | |
| `iface-bond-member-refused` | `test/plugin/` | AC-4 | |
| `iface-bond-member-mtu-mac` | `test/plugin/` | AC-5, AC-6, AC-9 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bond-lacp-linux-peer` | `test/interop/scenarios/` | a Linux kernel bond in 802.3ad mode | LACP negotiation with another implementation | |

## Files to Modify
- `internal/component/iface/yang/ze-iface-conf.yang` - bond kind
- `internal/component/iface/backend.go` - bond create and member calls
- `internal/component/iface/config.go`, `internal/component/iface/config_apply.go` - parse, verify, apply
- `internal/plugins/iface/netlink/bridge_linux.go` - shared enslave helper, or a sibling bond file
- `internal/component/iface/doctor_linux.go` - member checks

## Files to Create
- `internal/plugins/iface/netlink/bond_linux.go`
- `test/plugin/iface-bond-lacp-apply.ci`, `test/plugin/iface-bond-member-refused.ci`, `test/plugin/iface-bond-member-mtu-mac.ci`

## Implementation Steps

1. **Phase: Wiring** - YANG kind, parse, failing `.ci`, approval for A-1.
2. **Phase: Verify** - member option refusals, one aggregator per member.
3. **Phase: Netlink apply** - create, enslave, release, MTU and MAC rules.
4. **Phase: Show, doctor, VPP** - LACP state, member checks, VPP or refusal.

Proof: `./le test unit plugins`, `./le test functional plugin` for the named `.ci` files, `./le test qemu all` for the Linux integration tests.

## VyOS reference

VyOS checkout `~/Code/github.com/vyos/vyos-1x`, read 2026-11-08 at `611550d23`.

| Ticket / commit | Lesson for Ze |
|-----------------|---------------|
| T3871, `32f664077` "ethernet: leave a bond member's MTU alone while it is still enslaved" | Deleting a jumbo bond member's ethernet config dropped its MTU while it stayed the active member: jumbo frames lost both ways, nothing logged. AC-5 |
| T9269, `395338fe0` "bond: keep the bond MAC when appending a member interface" | A flag re-ran the "copy first member MAC" step on every append and gave the bond the new member's MAC. AC-6 |
| T9454, `f1be2782b` "ethernet: show CLI option names in bond member errors and warnings" | Errors named internal keys (`vif_s 10 vif_c 100 address`). AC-4 |
| T3871, `ccac4ac56` "interfaces: keep bridge and bond members discoverable" | Any device with a master was skipped by NIC naming, so in-use members looked absent. AC-8 |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- MLAG (multi-chassis) is not in this spec.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)
