# Spec: iface-qinq-802-1ad

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release (IEEE 802.1ad stacked VLANs). Single-tag 802.1Q units stay shipped and required |
| RFC gaps | - |
| Depends | spec-iface-delete-parent-before-vlan-rolls-back.md |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Providers hand off customer VLANs inside an outer service tag (IEEE 802.1ad,
ethertype 0x88A8, "QinQ"), and an access router terminates or builds such
links. An operator must be able to configure a unit that carries an outer
S-VLAN tag and an inner C-VLAN tag, with addresses, DHCP and every other unit
setting on the inner one, and to remove it cleanly. VyOS offers `vif-s <id>
vif-c <id>`; Ze offers one 802.1Q tag per unit only.

| # | Operator need |
|---|---------------|
| 1 | eth0, S-tag 100 (0x88A8), C-tag 200 (0x8100): addresses on the C-VLAN |
| 2 | Several C-VLANs under one S-VLAN |
| 3 | DHCP client on a C-VLAN |
| 4 | Delete one C-VLAN, the S-VLAN, or the whole parent entry, in any order, with no stranded device |
| 5 | Choose 0x8100 for the outer tag where a provider uses double 802.1Q |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/iface/logical-name-resolution.md` - units on a selector-bound parent
  → Constraint: a VLAN sub-interface inherits its parent's MAC and must not answer the parent's `mac match`; a second level inherits too
- [ ] `docs/architecture/iface/vlan-qos-map.md` - per-unit PCP maps
  → Constraint: QoS maps apply to the tag the unit creates; which tag (outer or inner) carries the map is a design decision
- [ ] `plan/release/fixes/spec-iface-delete-parent-before-vlan-rolls-back.md` - deleting a parent with VLAN units fails by map order
  → Constraint: QinQ deepens the same ordering problem; teardown must be deepest first, and that fix lands first

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/backend.go` - `VLANSpec{Parent, VLANID, IngressQoSMap, EgressQoSMap}`; `CreateVLAN` documents "an 802.1Q sub-interface named <Parent>.<VLANID>"; no protocol field
- [ ] `internal/component/iface/config_apply.go` - the unit loop: `unitOSName(device, u)`, existence read before create, `CreateVLAN` with the undo deleting only a VLAN this step made, QoS map reprogramming on an existing VLAN
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - `grouping interface-unit`, `list unit` with one `vlan-id`; no outer tag, no protocol
- [ ] `internal/component/iface/config.go` - `ifaceEntry.Units []unitEntry`

**Behavior to preserve:**
- Every single-tag unit, its naming, QoS maps and rollback.

**Behavior to change:**
- A unit can carry an outer tag with a protocol (802.1ad default, 802.1Q allowed) and an inner tag.
- Teardown removes inner devices before outer and outer before the parent, stopping each device's DHCP client before its device goes.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| VLAN unit create | `config_apply.go` unit loop, `Backend.CreateVLAN`, `VLANSpec` | Creates `<parent>.<id>` 802.1Q, journaled | Extends: `VLANSpec` gains the tag protocol; the S-VLAN is a VLAN on the parent and the C-VLAN a VLAN on the S-VLAN |
| Unit device naming | `config_apply.go` `unitOSName` | Composes the kernel name of a unit | Extends: two-level names must fit IFNAMSIZ (15); `enp3s0f1.4094.4094` does not |
| VLAN QoS maps | `UpdateVLANQoSMap`, `vlan-qos-map.md` | PCP maps per unit | Extends to the chosen tag |
| Parent-before-VLAN teardown fix | `spec-iface-delete-parent-before-vlan-rolls-back.md` | Skeleton: ordering defect for one level | Depends on it; this spec adds the second level |
| DHCP client per unit | `internal/plugins/iface/dhcp` | Runs a client per configured unit device | Extends: stopped before its device is deleted |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: a unit with an outer tag and an inner tag (shape settled at design: nested inner units under an outer unit, or one unit with two ids).

### Transformation Path
1. Verify: inner tag requires an outer tag; ids 1..4094; protocol 802.1ad or 802.1Q; composed names fit 15 characters or a naming rule applies.
2. Apply: create the S-VLAN (protocol set), then each C-VLAN on it; addresses and DHCP on the C-VLAN.
3. Removal: stop DHCP on each C-VLAN, delete C-VLANs, stop DHCP on the S-VLAN, delete the S-VLAN, then the parent's own changes.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| iface to kernel | rtnetlink VLAN link with IFLA_VLAN_PROTOCOL | No |
| iface to VPP | dot1ad sub-interface create (A-2) | No |

### Changed-Wire Census

**Search evidence:** owed at design.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [format] | [role] | [source site] | [finding] | [disposition] |

### Integration Points
- The unit loop and the prune step in `config_apply.go`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The kernel deletes child VLANs when their lower device is deleted (cascade) | VyOS T9313 `41fac0cfb` | Teardown order matters less | Integration test | unvalidated |
| A-2 | VPP builds an 802.1ad outer plus 802.1Q inner sub-interface exactly (`create_subif` with dot1ad and inner tag flags) | VPP sub-interface API | VPP refuses QinQ | Read VPP source, quote the function | unvalidated |
| A-3 | Setting `VlanProtocol` on the vendored netlink VLAN link needs no further approval: every package and feature of `vishvananda/netlink` may be used, and the per-use approval for third-party modules applies only to experimental crypto modules such as Cloudflare's `circl` | Owner standing rule, recorded with the other code and dependency rules; vendored `vishvananda/netlink` `Vlan.VlanProtocol` | - | Owner rule | validated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The parent MTU does not leave room for a second tag | Large frames lost | Verify warns or refuses when C-VLAN MTU plus 8 exceeds the parent MTU, per design |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Customer traffic on the handoff; a stranded device breaks later commits |
| How is it reverted? | Remove the outer unit; teardown removes both levels |
| Who else touches this path? | DHCP client, RA, VRRP (VLAN tag carried in group extraction), CoS maps, the parent-before-VLAN fix |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| eth0 with S-tag 100 and C-tag 200 carrying an address | → | two-level VLAN create in the unit loop | `test/plugin/iface-qinq-apply.ci` (`option=needs-linux`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | S-tag 100 (default protocol), C-tag 200 with 192.0.2.1/24 | The S-VLAN reports protocol 802.1ad and id 100, the C-VLAN protocol 802.1Q and id 200 on it; a frame sent to 192.0.2.2 leaves eth0 with outer 0x88A8/100 and inner 0x8100/200 | None |
| AC-2 | Outer protocol set to 802.1Q | The S-VLAN reports protocol 802.1Q | None |
| AC-3 | Delete the whole outer unit while a DHCP client runs on the C-VLAN | The DHCP client stops before its device is deleted; the C-VLAN is removed before the S-VLAN; no device is re-created; the next commit succeeds (VyOS T9313) | None |
| AC-4 | Delete the parent entry with S and C VLANs under it | All removed deepest first; no rollback, in any map order | None |
| AC-5 | A C-VLAN was already removed by the kernel cascade when the prune reaches it | The prune treats it as gone, never re-creates it, and reports no error | None |
| AC-6 | Composed device name longer than 15 characters | Refused at verify naming the unit and the limit, or named by the design's rule; never truncated silently | None |
| AC-7 | Inner tag without outer tag, or an id outside 1..4094 | Refused naming the leaf | None |
| AC-8 | `mac match` on the parent with QinQ units | The parent stays the only device answering the selector | None |
| AC-9 | `backend vpp` | QinQ works on VPP (A-2) or the commit refuses it with the YANG path | None |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Terminates a QinQ handoff | config, unit loop, kernel | `test/plugin/iface-qinq-apply.ci` |
| 2 | Removes it with DHCP running | prune, DHCP stop, delete | `test/plugin/iface-qinq-teardown.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestQinQTeardownOrder` | `internal/component/iface/config_qinq_test.go` | AC-3, AC-4, AC-5 against the fake backend | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| outer and inner id | 1..4094 | 4094 | 0 | 4095 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `iface-qinq-apply` | `test/plugin/` | AC-1 with a packet capture on the peer veth | |
| `iface-qinq-teardown` | `test/plugin/` | AC-3, AC-4 | |

## Files to Modify
- `internal/component/iface/yang/ze-iface-conf.yang` - outer tag and protocol
- `internal/component/iface/backend.go` - `VLANSpec` protocol
- `internal/component/iface/config.go`, `internal/component/iface/config_apply.go` - parse, create, prune order

## Files to Create
- `test/plugin/iface-qinq-apply.ci`, `test/plugin/iface-qinq-teardown.ci`

## Implementation Steps

1. **Phase: Wiring** - YANG, parse, failing `.ci`.
2. **Phase: Create** - protocol on `VLANSpec`, two-level create.
3. **Phase: Teardown** - deepest-first prune with DHCP stop.
4. **Phase: VPP** - dot1ad sub-interface or refusal.

Proof: `./le test unit plugins`, `./le test functional plugin` for the named `.ci` files, `./le test qemu all`.

## VyOS reference

VyOS checkout `~/Code/github.com/vyos/vyos-1x`, read 2026-11-08 at `611550d23`.

| Ticket / commit | Lesson for Ze |
|-----------------|---------------|
| T9313, `41fac0cfb` "ifconfig: fix removal of QinQ sub-interfaces" | The removal list held the S-VLAN and its C-VLANs; deleting the S-VLAN cascaded, and the loop then re-created the gone C-VLAN with a bare `ip link add` with no type, which failed and stranded a device that broke every later test. AC-3, AC-5 |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- Bridge VLAN filtering and QinQ on bridge ports are not in this spec.

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
- [ ] Interop tests for protocol features (or N-A with a reason): N-A, IEEE 802.1ad framing is proven by packet capture
