# Spec: pim-sm-multicast-routing

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release: IP multicast routing with PIM-SM (RFC 7761) and the host-side group membership protocols it needs. No release profile lists these RFCs, so no summary is written or enrolled until Thomas brings them into scope (`ai/META-DECISIONS.md`). |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

## Task

Ze forwards no routed multicast. An operator who needs IPTV, market data or
any other multicast service across routed links needs PIM Sparse Mode toward
the other routers, IGMP (IPv4) and MLD (IPv6) toward the receivers' LANs, and
the kernel's multicast forwarding cache programmed from the result. Source
Specific Multicast (PIM-SSM, RFC 7761 Section 4.8, with IGMPv3 and MLDv2) is
the smallest useful subset; Any Source Multicast adds the RP, registers and
the shared tree.

### VyOS reference

VyOS `protocols pim`, `protocols pim6`, `protocols igmp-proxy` and
`interfaces ... ip igmp`, on top of FRR pimd and pim6d. BIRD has no PIM, so FRR
is the interop peer.

### RFC text read (`rfc/full/rfc7761.txt`, table of contents and Section 4 titles)

- Section 4.3: Designated Routers and Hello messages (4.3.1 sending, 4.3.2 DR election).
- Section 4.4: Register messages between the DR and the RP. Section 4.5: Join/Prune for (*,G), (S,G) and (S,G,rpt), each with its state machine. Section 4.6: Assert. Section 4.7: Bootstrap and RP discovery (RFC 5059 for BSR). Section 4.8: Source-Specific Multicast and PIM-SSM-only routers. Section 4.9: packet formats.
- The design phase reads each MUST of Section 4 and of the membership protocol RFCs (IGMPv2 RFC 2236, IGMPv3 RFC 3376, MLDv2 RFC 3810) and fetches the missing texts into `rfc/full/` first.

## Required Reading

- [ ] `rfc/full/rfc7761.txt` Section 4 in full
- [ ] `docs/architecture/protocol-skeleton.md` - the layout a new protocol follows (BFD as the reference)
- [ ] `docs/architecture/core-design.md`, section 16 - the FIB pipeline, which has no multicast path today
- [ ] `ai/doors/routing-protocols.md`, `ai/doors/forwarding.md`

## Current Behavior

- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - the interface unit carries no IGMP, MLD or PIM setting; no plugin under `internal/plugins/` speaks PIM, IGMP or MLD, and no FIB writer programs the kernel multicast forwarding cache.

## Existing Mechanisms

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| Protocol plugin skeleton | BFD layout, `docs/architecture/protocol-skeleton.md` | The reference shape for a protocol engine | Reused: PIM is an edge plugin owning its codec, FSMs, timers and state |
| Unicast RIB for RPF | Loc-RIB and sysrib | Selects unicast routes | Reused read-only: RPF lookups toward sources and the RP; the design states how a forked plugin reads it |
| FIB writers | `fib-kernel`, `fib-vpp` | Program unicast routes and MPLS labels; one writer per data plane | Extends: a multicast route event and its kernel writer (ipmr `MRT_*` socket, which one process per table owns); VPP refuses at verify until it has a writer |
| Interface name resolution and events | `internal/component/iface`, EventBus interface events | Interface up, down, addresses | Reused for PIM and IGMP interface state |
| BFD client | `internal/component/bfd` | Liveness for BGP, OSPF, static | Optional later: PIM BFD (RFC 7761 does not require it) |
| Firewall and CoPP | `internal/plugins/copp` | Protects the control plane | Extends: PIM, IGMP and MLD must pass CoPP |

## Data Flow

### Entry Point
Config enabling PIM and IGMP/MLD per interface; received Hello, Join/Prune, Register, Assert, Bootstrap; IGMP and MLD reports.

### Transformation Path
1. Membership state per interface from IGMP or MLD.
2. PIM state machines per (*,G), (S,G), (S,G,rpt) with RPF from the unicast RIB.
3. The outgoing interface list per (S,G) is published as a multicast route event and written to the kernel MFC by the data-plane writer.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| PIM plugin to the multicast FIB writer | a registered event type, never netlink from the protocol (`ai/doors/routing-protocols.md`) | No |
| Kernel upcalls (NOCACHE, WRONGVIF, WHOLEPKT) to the plugin | the writer that owns the mroute socket forwards them | No |

### Integration Points
- New plugin, new event type, kernel writer, CoPP, `show` commands on every command surface.

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| PIM-SSM config, receiver joins (S,G) by IGMPv3 | → | (S,G) Join upstream and MFC entry | [QEMU integration test reading the kernel MFC, named at design] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | PIM on two interfaces | Hellos sent per Section 4.3.1; neighbors and DR elected per Section 4.3.2; `show pim neighbor` |
| AC-2 | IGMPv3 report for (S,G) in the SSM range on a LAN where Ze is DR | Ze sends an (S,G) Join toward S along the unicast RPF path |
| AC-3 | Traffic from S arrives on the RPF interface | The kernel MFC holds (S,G) with the right outgoing interfaces; packets reach the receiver LAN (QEMU) |
| AC-4 | Receiver leaves | Prune upstream and MFC entry removed after the timers of Section 4.10 |
| AC-5 | ASM group with a static RP | Register from the DR, (*,G) Join toward the RP, switchover to the SPT (Section 4.2.1) |
| AC-6 | Two routers forward onto one LAN | Assert elects one forwarder (Section 4.6) |
| AC-7 | IPv6 with MLDv2 | The same as AC-2 to AC-4 |
| AC-8 | Interop: Ze and FRR pimd in a three-router lab | SSM and ASM trees build across both; traffic flows; proven red with the Join send reverted |
| AC-9 | Another process already owns the kernel mroute socket for the table | Ze refuses to start PIM with an error naming the owner and the fix; `ze doctor` reports it |

## Risks & Assumptions

| ID | Risk | Mitigation |
|----|------|------------|
| R-1 | The kernel allows one mroute socket owner per table | AC-9, and the writer is the only owner |
| R-2 | Scope is large | Split at design into SSM first, then ASM with RP and BSR, each its own child spec |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [named at design] | the new PIM plugin's packet package | Section 4.9 encode and decode | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [named at design] | a new `test/pim/` suite | Config parse, Hello exchange in a netns | |

## Files to Modify

- New plugin `internal/plugins/pim` (and IGMP/MLD, placement decided at design)
- `internal/plugins/fib/` multicast writer
- `internal/component/iface/yang/ze-iface-conf.yang` or the plugin's own augment
- New guide `docs/guide/multicast.md`, `feature-gates.txt` entry

## Implementation Steps

1. Design: split into child specs (SSM, ASM, BSR, IPv6), fetch the membership RFC texts.
2. Codec and Hello, then IGMPv3 and SSM, then the MFC writer.
3. FRR interop scenario.

### Integration Checklist
- [ ] Compiles out with its own feature gate

### Documentation Update Checklist
- [ ] New multicast guide

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before the change
- [ ] Tests PASS after the change
- [ ] `./le test qemu all` for the MFC test, `./le test integration interop` with the new scenario
