# Spec: ospfv3-0 -- OSPFv3 for IPv6 (Follow-up Umbrella)

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-ospf-0-umbrella.md |
| Phase | follow-up |
| Updated | 2026-06-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file.
2. `plan/spec-ospf-0-umbrella.md` for the OSPFv2 pattern and scope boundary.
3. `docs/research/ospf-implementation-guide.md` §15 for the OSPFv3 separation decision.
4. `ai/rules/module-tiers.md` for edge-plugin placement.
5. `rfc/short/rfc5340.md`, `rfc/short/rfc7166.md`, `rfc/short/rfc5838.md`, and `rfc/short/rfc4552.md`.

## Task

Implement OSPFv3 for IPv6 as a follow-up to the OSPFv2 spec set. OSPFv3 is
not delivered by `spec-ospf-0-umbrella.md`: it has a different packet header,
LSA registry, flooding-scope model, prefix encoding, checksum, authentication
mechanism, and transport address family. Ze will implement it as a separate
edge plugin under `internal/plugins/ospfv3/`, using the OSPFv2 implementation
as a pattern source but sharing no packet, LSA, LSDB, auth, or SPF data
structures in the first pass.

The deliverable is IPv6 unicast OSPFv3 with point-to-point and broadcast
adjacency, area 0 and non-backbone areas, SPF route install into IPv6 Loc-RIB,
inter-area summaries, external redistribution, stub/NSSA behavior, RFC 7166
authentication trailer support, CLI/metrics/doctor/web surfaces, and FRR
`ospf6d` interoperability.

## Scope

### In Scope

| Area | Requirement |
|------|-------------|
| Placement | New edge plugin at `internal/plugins/ospfv3/`; do not place it under the component tree |
| Transport | Raw IPv6 protocol 89 with link-local source addresses and multicast `ff02::5` / `ff02::6` |
| Packet codec | RFC 5340 16-byte common header and packet types 1 through 5 |
| LSAs | RFC 5340 scope-aware LSA type parsing and base LSAs `0x2001`, `0x2002`, `0x2003`, `0x2004`, `0x4005`, `0x2007`, `0x0008`, `0x2009` |
| Prefixes | IPv6 PrefixLength, PrefixOptions, and padded 32-bit-word prefix encoding |
| Interface model | Instance ID, Interface ID, link-local neighbor source, Hello parameters, DR/BDR election |
| Neighbor model | OSPFv3 NSM, DD exchange, LS Request, LS Update, LS Ack, retransmit lists |
| SPF | Topology from Router-LSAs and Network-LSAs, prefix attachment from Intra-Area-Prefix-LSAs |
| Route install | IPv6 `locrib.Path` insertion with admin distance 110 and existing sysrib/fibkernel fanout |
| Redistribution | Source `ospfv3` and IPv6 redistribution consumer through `redistevents` |
| Areas | Backbone, non-backbone, ABR summaries, stub areas, NSSA Type 7 and Type 7 to Type 5 translation |
| Authentication | RFC 7166 Authentication Trailer with HMAC-SHA-1/256/384/512, AT-bit, 64-bit sequence anti-replay |
| Observability | CLI, metrics, doctor, web status, functional tests, FRR interop scenarios |
| Coexistence | OSPFv2 and OSPFv3 can run on the same node without shared mutable state or config ambiguity |

### Out of Scope

| Area | Reason |
|------|--------|
| RFC 5838 multiple address families | Base IPv6 unicast first; Instance ID plumbing must not block later AF support |
| RFC 4552 IPsec AH/ESP | RFC 7166 trailer is the first auth path; IPsec needs separate kernel policy work |
| Virtual links | Same deferral as OSPFv2; add after backbone behavior is stable |
| Opaque/TE/SR/GR/BFD | Large extension families; require stable base LSDB and interop first |
| Shared OSPFv2/OSPFv3 packet or LSA package | Different wire contracts would leak version-specific branches into both implementations |
| SNMP MIB | Not part of Ze's current management plane |

## Required Reading

### Repo and Architecture

- [ ] `ai/rules/module-tiers.md` - edge protocols without reverse dependencies belong under `internal/plugins/`
  -> Constraint: use `internal/plugins/ospfv3/`, not the component tree.
- [ ] `plan/spec-ospf-0-umbrella.md` - OSPFv2 sibling and implementation sequence
  -> Constraint: reuse patterns and verification shape, not code or wire structs.
- [ ] `docs/research/ospf-implementation-guide.md` §15 - OSPFv3 differences and the do-not-unify recommendation
  -> Constraint: FRR has `ospfd` and `ospf6d`; Ze follows that separation.
- [ ] `docs/guide/command-catalogue.md` - existing OSPFv3 CLI naming row
  -> Constraint: user-facing command should be `show ipv6 ospf` unless a later CLI review changes the catalogue.
- [ ] `internal/analyze/statistics.go`, `internal/mrt/types.go`, `internal/component/bgp/plugins/nlri/ls/types.go`
  -> Constraint: existing OSPFv3 strings/constants are MRT or BGP-LS metadata only, not an OSPFv3 routing engine.

### RFC Summaries

- [ ] `rfc/short/rfc5340.md` - OSPFv3 base protocol
  -> Constraint: 16-byte header, IPv6 checksum pseudo-header, scope-aware LS types, topology/prefix separation.
- [ ] `rfc/short/rfc7166.md` - OSPFv3 Authentication Trailer
  -> Constraint: AT-bit, trailer outside OSPFv3 packet length, 64-bit sequence per neighbor and packet type, HMAC-SHA algorithms.
- [ ] `rfc/short/rfc5838.md` - multi-address-family OSPFv3
  -> Constraint: out of scope, but keep Instance ID explicit and validated.
- [ ] `rfc/short/rfc4552.md` - OSPFv3 IPsec AH/ESP
  -> Constraint: out of scope; do not require kernel IPsec policy for base OSPFv3.

## Current Behavior

**Source files read or searched:**
- `internal/analyze/statistics.go` maps MRT OSPFv3 type names for analysis output.
- `internal/mrt/types.go` defines MRT OSPFv3 type constants.
- `internal/component/bgp/plugins/nlri/ls/types.go` defines BGP-LS protocol ID `ProtoOSPFv3`.
- `docs/guide/command-catalogue.md` has an OSPFv3 row for `show ipv6 ospf`.
- No `internal/plugins/ospfv3/` routing engine, config module, transport, LSDB, or OSPFv3 tests exist today.

**Behavior to preserve:**
- MRT and BGP-LS OSPFv3 constants continue to mean external data formats, not the new routing engine.
- Existing IPv6 route installation through Loc-RIB, sysrib, and fibkernel remains the only FIB path.
- OSPFv2 remains IPv4-only and independent.

**Behavior to change:**
- Add `ospfv3` config and lifecycle.
- Open raw IPv6 OSPF sockets on configured interfaces.
- Form OSPFv3 adjacencies and install IPv6 routes.
- Expose OSPFv3 status through CLI, web, metrics, doctor, and functional tests.

## Architecture

### Package Layout

| Path | Purpose |
|------|---------|
| `internal/plugins/ospfv3/register.go` | Plugin registration, YANG embed hook, lifecycle entry |
| `internal/plugins/ospfv3/config.go` | Typed config resolution, defaults, validation glue |
| `internal/plugins/ospfv3/instance.go` | Instance lifecycle, goroutines, socket ownership, route/redistribution wiring |
| `internal/plugins/ospfv3/area.go` | Area state, LSDB ownership, ABR/NSSA flags |
| `internal/plugins/ospfv3/types/` | Router ID, Area ID, Instance ID, Interface ID, metrics, prefix options, LSA keys |
| `internal/plugins/ospfv3/packet/` | RFC 5340 packet and LSA encode/decode |
| `internal/plugins/ospfv3/transport/` | Raw IPv6 proto 89, multicast membership, link-local source selection |
| `internal/plugins/ospfv3/iface/` | Interface state machine, Hello, DR/BDR, timers |
| `internal/plugins/ospfv3/neighbor/` | Neighbor state machine, DD, LS Request, retransmission lists |
| `internal/plugins/ospfv3/lsdb/` | Scope-aware LSDB, origination, flooding, aging, ack handling |
| `internal/plugins/ospfv3/spf/` | SPF graph, prefix attachment, IPv6 route candidate generation |
| `internal/plugins/ospfv3/redistribute/` | IPv6 external origination and redistevents consumer |
| `internal/plugins/ospfv3/auth/` | RFC 7166 Security Associations, trailer sign/verify, sequence state |
| `internal/plugins/ospfv3/yang/` | `ze-ospfv3-conf.yang`, `ze-ospfv3-cmd.yang`, generated glue |
| `internal/plugins/ospfv3/cmd_show.go` | CLI and RPC backing data |
| `www/api/ospfv3*`, `www/src/pages/ospfv3*` | Web API and status page |

### Child Specs

| Child | Name | Depends | Owns |
|-------|------|---------|------|
| `spec-ospfv3-1-types.md` | types and constants | umbrella | Router ID, Area ID, Instance ID, Interface ID, prefix options, LSA key types |
| `spec-ospfv3-2-wire.md` | packet and LSA codec | ospfv3-1 | Header, packet types, LSA header, base LSAs, prefix encoding, checksum |
| `spec-ospfv3-3-ipv6-transport.md` | raw IPv6 transport | ospfv3-2 | IPv6 proto 89 sockets, multicast groups, link-local source, interface receive loop |
| `spec-ospfv3-4-plugin-config.md` | plugin registration and YANG | ospfv3-1, ospfv3-2, ospfv3-3 | Config tree, instance lifecycle, generated YANG glue |
| `spec-ospfv3-5-interface-ism.md` | interface FSM and Hello | ospfv3-4 | Hello send/receive, DR/BDR election, Interface ID handling |
| `spec-ospfv3-6-neighbor-nsm.md` | neighbor FSM | ospfv3-5 | DD exchange, LS Request, adjacency Full |
| `spec-ospfv3-7-lsdb-flooding.md` | LSDB and flooding | ospfv3-6 | Scope-aware LSDB, Link-LSA, Intra-Area-Prefix-LSA origination, flood/ack |
| `spec-ospfv3-8-spf-rib.md` | SPF and IPv6 route install | ospfv3-7 | Intra-area SPF, prefix attachment, Loc-RIB install |
| `spec-ospfv3-9-inter-area-abr.md` | inter-area ABR | ospfv3-8 | Inter-Area-Prefix-LSA, Inter-Area-Router-LSA, area ranges |
| `spec-ospfv3-10-as-external-asbr.md` | AS external and redistribution | ospfv3-8, ospfv3-9 | AS-External-LSA, IPv6 redist consumer, E1/E2 selection |
| `spec-ospfv3-11-stub-nssa.md` | stub and NSSA | ospfv3-7, ospfv3-8, ospfv3-9, ospfv3-10 | Stub filtering, NSSA-LSA, translator election, Type 7 to 5 |
| `spec-ospfv3-12-auth.md` | RFC 7166 auth trailer | ospfv3-2, ospfv3-3, ospfv3-4, ospfv3-5, ospfv3-6, ospfv3-7 | AT-bit, SA config, sign/verify, sequence anti-replay |
| `spec-ospfv3-13-cli-diag-interop.md` | CLI, metrics, web, doctor, FRR interop | ospfv3-1 through ospfv3-12 | User surfaces and interop matrix |

### Follow-up Spec Added

| Spec | Status | Purpose |
|------|--------|---------|
| `plan/spec-ospfv3-1-types.md` | design | First implementation child; creates the OSPFv3 leaf type package consumed by all later children |

The next expansion target after `ospfv3-1-types` is `spec-ospfv3-2-wire.md`, the packet and LSA codec.

### Dependency Graph

| Stage | Specs | Why |
|-------|-------|-----|
| Foundation | 1, 2, 3, 4 | Types, codec, transport, and config must exist before runtime behavior |
| Adjacency | 5, 6 | Hello and NSM create Full neighbors |
| Database | 7 | Flooding needs Full neighbors and packet handlers |
| Routing | 8, 9, 10, 11 | SPF first, then summaries, externals, and area type constraints |
| Security | 12 | Auth wraps all packet paths after they exist |
| Product surface | 13 | CLI, metrics, doctor, web, and FRR interop after behavior exists |

## Data Flow

### Entry Points

- Config: `ospfv3` subtree from YANG-validated config.
- RX: raw IPv6 datagrams with Next Header 89 on enabled interfaces.
- TX: OSPFv3 packet bytes emitted through raw IPv6 transport.
- Redistribution: IPv6 route events from `redistevents` into external LSA origination.
- User surface: CLI, web, metrics, doctor, and functional tests.

### Packet Receive Path

1. Raw IPv6 socket receives proto 89 datagram on an enabled interface.
2. Transport validates link-local source, destination group or unicast target, hop limit, interface, and Instance ID.
3. Packet codec validates version 3, type, length, area, checksum, and optional RFC 7166 trailer.
4. Dispatcher routes Hello to ISM, DD/LS Request to NSM, LS Update/LS Ack to LSDB.
5. LSDB validates LSA scope and keys, stores newer LSAs, schedules floods and acks.
6. SPF invalidation queues recomputation for affected areas.
7. SPF produces IPv6 Loc-RIB candidates.
8. Loc-RIB, sysrib, and fibkernel handle arbitration and FIB changes.

### Packet Transmit Path

1. Runtime builds a packet from typed state and raw LSA bytes.
2. Codec writes the OSPFv3 header, body, and checksum or RFC 7166 trailer.
3. Transport sends through IPv6 proto 89 using the interface link-local source.
4. Multicast destination is `ff02::5` or `ff02::6`; unicast retransmission uses the neighbor link-local source address learned from Hello.

### Redistribution Path

1. `redistevents` emits IPv6 candidates selected by policy.
2. `ospfv3` redistribution consumer filters by configured route maps and route family.
3. ASBR logic originates AS-External-LSAs or NSSA-LSAs.
4. LSDB floods them with AS or area scope.
5. SPF/route selection ranks intra-area, inter-area, E1/E2, and NSSA translated candidates inside OSPFv3 before exporting one best path per prefix to Loc-RIB.

### Boundaries Crossed

| Boundary | Direction | Rule |
|----------|-----------|------|
| Config -> plugin | YANG tree to typed config | No env vars; all operator config is under `ospfv3` |
| Transport -> codec | Raw bytes to typed packet | Bounds checks before every field read |
| Codec -> auth | Packet plus optional trailer | RFC 7166 owns trailer length and HMAC verification |
| LSDB -> SPF | Scoped LSAs to graph | SPF reads immutable LSA views, not mutable flood queues |
| SPF -> Loc-RIB | IPv6 route candidates | No direct FIB writes |
| Redistribution -> LSDB | IPv6 route event to LSA | No Loc-RIB feedback loop without explicit policy |
| Runtime -> observability | State snapshots to CLI/web/metrics | No pointers to mutable engine maps exposed |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ospfv3` config with one area and one enabled interface | Plugin starts, validates config, opens raw IPv6 proto 89 socket, joins `ff02::5` |
| AC-2 | Two Ze nodes on a point-to-point IPv6 link | Hello, DD, LS Request/Update/Ack complete and neighbor reaches Full |
| AC-3 | Ze node on a broadcast LAN with three routers | DR/BDR election uses Router ID and priority; Network-LSA originated by DR |
| AC-4 | Router-LSA and Network-LSA topology with Intra-Area-Prefix-LSA prefixes | SPF installs IPv6 intra-area routes through Loc-RIB |
| AC-5 | Prefix-only change in Intra-Area-Prefix-LSA | Prefix attachment updates routes without reinterpreting topology LSAs as IPv6 addresses |
| AC-6 | Non-backbone area connected by ABR | Inter-Area-Prefix-LSAs and Inter-Area-Router-LSAs propagate summaries correctly |
| AC-7 | External IPv6 route redistributed into OSPFv3 | AS-External-LSA is originated and remote node installs the E1/E2 route |
| AC-8 | Stub area configured | External LSAs are blocked and default route injection follows configured cost |
| AC-9 | NSSA area configured | NSSA-LSA is originated, translator election is stable, Type 7 to Type 5 translation is correct |
| AC-10 | RFC 7166 auth enabled with matching SA | AT-bit is present, trailer verifies, sequence increases, adjacency reaches Full |
| AC-11 | RFC 7166 packet replayed or signed with wrong key | Packet is dropped before ISM/NSM/LSDB, auth failure metric increments |
| AC-12 | FRR `ospf6d` neighbor on p2p and broadcast links | Adjacency, LSDB sync, route convergence, and auth interop pass |
| AC-13 | OSPFv2 and OSPFv3 both configured on one node | Separate plugins run without shared state, config collision, or route-family confusion |
| AC-14 | Operator runs `show ipv6 ospf` and opens the web OSPFv3 page | Neighbors, interfaces, LSDB summary, routes, and counters match runtime state |
| AC-15 | Doctor runs on a host without raw socket capability | OSPFv3 reports a clear dependency failure before runtime loops start |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|---------------------|-----------------------|
| 1 | Enables OSPFv3 between two Ze routers on an IPv6 p2p link | config -> raw IPv6 -> Hello/DD -> LSDB -> SPF -> Loc-RIB | `test/ospfv3/ospfv3-adjacency.ci` |
| 2 | Connects Ze to FRR `ospf6d` on a p2p link | Ze transport/codec/FSM with FRR wire behavior | `test/interop/scenarios/ospfv3-p2p-frr/check.py` |
| 3 | Runs OSPFv3 on a LAN with DR/BDR | ISM DR election -> Network-LSA -> LSDB sync | `test/interop/scenarios/ospfv3-broadcast-frr/check.py` |
| 4 | Learns and installs IPv6 prefixes | Router/Network topology -> Intra-Area-Prefix-LSA -> SPF -> Loc-RIB | `test/ospfv3/ospfv3-route-install.ci` |
| 5 | Summarizes a non-backbone area | ABR logic -> Inter-Area-Prefix-LSA -> remote route | `test/interop/scenarios/ospfv3-multiarea-frr/check.py` |
| 6 | Redistributes a BGP IPv6 prefix into OSPFv3 | redistevents -> AS-External-LSA -> flood -> remote install | `test/ospfv3/ospfv3-redist-bgp.ci` and `test/interop/scenarios/ospfv3-redist-frr/check.py` |
| 7 | Enables OSPFv3 auth trailer | key chain -> RFC 7166 sign/verify -> Full adjacency | `test/ospfv3/ospfv3-auth.ci` and `test/interop/scenarios/ospfv3-auth-frr/check.py` |
| 8 | Checks OSPFv3 status | runtime snapshot -> CLI/web/metrics | `test/ospfv3/ospfv3-observability.ci` and `test/web/ospfv3-neighbor-database.wb` |

## TDD Plan

### Unit Tests

| Test | File | Proves |
|------|------|--------|
| `TestOSPFv3HeaderRoundTrip` | `internal/plugins/ospfv3/packet/header_test.go` | 16-byte header fields, Instance ID, length, reserved byte |
| `TestOSPFv3ChecksumIPv6PseudoHeader` | `internal/plugins/ospfv3/packet/checksum_test.go` | IPv6 pseudo-header checksum with Next Header 89 |
| `TestOSPFv3PrefixEncodingBoundaries` | `internal/plugins/ospfv3/packet/prefix_test.go` | `((PrefixLength + 31) / 32)` words, default route, padding validation |
| `TestOSPFv3LSATypeScope` | `internal/plugins/ospfv3/packet/lsa_test.go` | U/S2/S1/function parsing and scope selection |
| `TestOSPFv3RouterNetworkLSACodec` | `internal/plugins/ospfv3/packet/lsa_test.go` | Router-LSA and Network-LSA graph fields |
| `TestOSPFv3LinkAndIntraAreaPrefixLSA` | `internal/plugins/ospfv3/packet/lsa_test.go` | Link-LSA and Intra-Area-Prefix-LSA prefix attachment data |
| `TestOSPFv3TransportRejectsWrongInstance` | `internal/plugins/ospfv3/transport/transport_test.go` | Instance ID mismatch drop before FSM |
| `TestOSPFv3ISMFullOnP2P` | `internal/plugins/ospfv3/iface/ism_test.go` | Hello path reaches neighbor discovery on p2p |
| `TestOSPFv3BroadcastDRElection` | `internal/plugins/ospfv3/iface/dr_test.go` | Router ID and priority DR/BDR election |
| `TestOSPFv3NSMDownToFull` | `internal/plugins/ospfv3/neighbor/nsm_test.go` | DD exchange and request list path to Full |
| `TestOSPFv3FloodScope` | `internal/plugins/ospfv3/lsdb/flood_test.go` | Link-local LSAs never leave the link; area and AS LSAs flood correctly |
| `TestOSPFv3SPFPfxAttach` | `internal/plugins/ospfv3/spf/spf_test.go` | SPF topology plus Intra-Area-Prefix route attachment |
| `TestOSPFv3ABRSummary` | `internal/plugins/ospfv3/spf/abr_test.go` | Inter-Area-Prefix-LSA origination and selection |
| `TestOSPFv3ExternalE1E2` | `internal/plugins/ospfv3/redistribute/external_test.go` | AS-External-LSA E1/E2 selection and metric calculation |
| `TestOSPFv3NSSATranslation` | `internal/plugins/ospfv3/redistribute/nssa_test.go` | NSSA-LSA to AS-External-LSA translation |
| `TestOSPFv3AuthTrailerLayout` | `internal/plugins/ospfv3/auth/trailer_test.go` | RFC 7166 fixed trailer and Auth Data Len |
| `TestOSPFv3AuthSequencePerPacketType` | `internal/plugins/ospfv3/auth/trailer_test.go` | 64-bit sequence anti-replay per neighbor and packet type |
| `TestOSPFv3AuthWrongSARejected` | `internal/plugins/ospfv3/auth/trailer_test.go` | Unknown/expired SA drops before dispatcher |

### Functional Tests

| Test | Proves |
|------|--------|
| `test/ospfv3/ospfv3-adjacency.ci` | Ze-to-Ze p2p adjacency reaches Full |
| `test/ospfv3/ospfv3-route-install.ci` | IPv6 prefixes install into Loc-RIB and FIB candidate path |
| `test/ospfv3/ospfv3-redist-bgp.ci` | IPv6 BGP/static/connected redistribution into OSPFv3 |
| `test/ospfv3/ospfv3-auth.ci` | RFC 7166 auth success, wrong key drop, replay drop |
| `test/ospfv3/ospfv3-observability.ci` | CLI, metrics, and doctor outputs match runtime state |

### Interop Tests

| Scenario | Proves |
|----------|--------|
| `test/interop/scenarios/ospfv3-p2p-frr/check.py` | FRR `ospf6d` p2p adjacency and route exchange |
| `test/interop/scenarios/ospfv3-broadcast-frr/check.py` | FRR broadcast DR/BDR, Network-LSA, route exchange |
| `test/interop/scenarios/ospfv3-multiarea-frr/check.py` | ABR summaries and multi-area route selection |
| `test/interop/scenarios/ospfv3-stub-nssa-frr/check.py` | Stub and NSSA behavior against FRR |
| `test/interop/scenarios/ospfv3-redist-frr/check.py` | AS-External-LSA and redistributed IPv6 prefixes |
| `test/interop/scenarios/ospfv3-auth-frr/check.py` | RFC 7166 trailer interoperability |
| `test/interop/scenarios/ospfv3-convergence-frr/check.py` | Link failure, flood, SPF, and route withdrawal timing |

## Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Version | 3 only | 3 | 2 | 4 |
| Packet Type | 1..5 | 5 | 0 | 6 |
| Packet Length | 16..65535 | 65535 | 15 | greater than datagram length |
| Instance ID | 0..255 | 255 | n/a | wrap to 0 |
| Area ID | uint32 | 0xFFFFFFFF | n/a | n/a |
| Interface ID | uint32, non-zero for active interfaces | 0xFFFFFFFF | 0 when required | duplicate on same router |
| PrefixLength | 0..128 | 128 | n/a | 129 |
| Prefix words | `(PrefixLength + 31) / 32` | 4 words for /128 | too short | extra non-zero padding |
| LS Type scope | U/S2/S1/function | AS scope `0x4005` | reserved scope 3 rejected for base LSAs | unknown function with U=0 mishandled |
| Auth Data Len | fixed 16 plus digest length | 80 for HMAC-SHA-512 | 15 | beyond IPv6 payload |
| RFC 7166 sequence | uint64 strictly increasing | 0xFFFFFFFFFFFFFFFF | equal last accepted | wrap to 0 without key reset |
| Hello interval | 1..65535 seconds | 65535 | 0 | 65536 |
| Router dead interval | greater than hello interval | 65535 | <= hello interval | 65536 |
| Cost | 1..16777215 unless child narrows | 16777215 | 0 | 16777216 |

## Config, CLI, Web, Metrics, Doctor

| Surface | Required Change |
|---------|-----------------|
| YANG config | Add `ospfv3` container with router-id, areas, interfaces, instance-id, costs, timers, auth key chains, redistribution, stub/NSSA, area ranges |
| YANG commands | Add OSPFv3 show RPCs under the existing action-before-identifier CLI grammar |
| CLI | `show ipv6 ospf`, `show ipv6 ospf interface`, `show ipv6 ospf neighbor`, `show ipv6 ospf database`, `show ipv6 ospf route` |
| Web | OSPFv3 neighbor/database/route page and API endpoint |
| Metrics | `ze_ospfv3_neighbors`, `ze_ospfv3_spf_runs_total`, `ze_ospfv3_lsa_total`, `ze_ospfv3_floods_total`, `ze_ospfv3_auth_failures_total`, `ze_ospfv3_routes_total` |
| Doctor | Raw socket capability, IPv6 link-local address, multicast membership, config sanity, auth SA expiry, interface status |
| Logs | State transitions and auth failures with no secret material |

## Files to Create

| Path | Owner spec |
|------|------------|
| `plan/spec-ospfv3-1-types.md` | First follow-up implementation child from this umbrella |
| `plan/spec-ospfv3-2-wire.md` through `plan/spec-ospfv3-13-cli-diag-interop.md` | Later child specs from this umbrella |
| `internal/plugins/ospfv3/` | All child specs |
| `internal/plugins/ospfv3/yang/ze-ospfv3-conf.yang` | ospfv3-4 |
| `internal/plugins/ospfv3/yang/ze-ospfv3-cmd.yang` | ospfv3-13 |
| `test/ospfv3/` | child specs 4 through 13 |
| `test/interop/scenarios/ospfv3-*` | ospfv3-13 |
| `test/web/ospfv3-neighbor-database.wb` | ospfv3-13 |
| `docs/guide/ospfv3.md` | ospfv3-13 |
| `docs/architecture/wire/ospfv3.md` | ospfv3-2 |

## Files to Modify

| Path | Change |
|------|--------|
| `internal/component/plugin/all/all.go` | Register OSPFv3 edge plugin |
| `internal/component/config/validators_register.go` | Add OSPFv3 validators and completion functions if not local to plugin |
| `internal/component/rib/admin-distance` files | Add or confirm `ospfv3` admin distance default 110 |
| `internal/component/redistevents/` | Add OSPFv3 IPv6 consumer/source identifiers |
| `mk/test-functional.mk` | Add `ze-ospfv3-test` target and suite registration |
| `docs/guide/configuration.md` | Document `ospfv3` config |
| `docs/guide/command-catalogue.md` | Confirm `show ipv6 ospf` row and examples |
| `docs/guide/plugins.md`, `docs/plugin-overview.md`, `docs/guide/status.md` | Add OSPFv3 edge plugin docs |
| `docs/functional-tests.md` | Document `test/ospfv3/` and interop scenarios |
| `docs/architecture/core-design.md` | Add OSPFv3 to edge-plugin protocol list |
| `www/` API and page registries | Add OSPFv3 status page and API route |

## Implementation Steps

1. **Types follow-up first** - implement `spec-ospfv3-1-types.md`.
   - Tests: `go test ./internal/plugins/ospfv3/types` after implementation.
   - Verify: leaf types, LS Type scope helpers, prefix length math, and boundary tests exist before the wire codec starts.
2. **Wire codec** - implement OSPFv3 header, LSA header, packet bodies, base LSAs, prefix encoding, and checksum using the types package.
   - Tests: packet/LSA/prefix/checksum unit tests fail first and then pass.
3. **Transport and plugin wiring** - add raw IPv6 socket, multicast, config, lifecycle, and YANG glue.
   - Tests: plugin start and wrong Instance ID rejection.
4. **Adjacency** - implement ISM, Hello, DR/BDR, NSM, DD, LS Request, LS Update, LS Ack.
   - Tests: Ze-to-Ze adjacency functional test and unit FSM tests.
5. **LSDB and flooding** - implement scope-aware LSDB, origination, flood queues, retransmission, and acks.
   - Tests: flood-scope unit tests and FRR p2p interop smoke.
6. **Routing** - implement intra-area SPF, prefix attachment, IPv6 Loc-RIB install, ABR summaries, externals, stub/NSSA.
   - Tests: route install, multiarea, redist, stub/NSSA functional and interop tests.
7. **Authentication** - implement RFC 7166 SA config, AT-bit, trailer, HMAC-SHA, sequence anti-replay, and wrong-key drops.
   - Tests: auth unit, functional, and FRR interop tests.
8. **Product surface** - add CLI, metrics, doctor, web, docs, and interop matrix.
   - Tests: observability functional, web, and seven interop scenarios.
9. **Full verification** - run focused child tests as each spec lands, then the repo verification command required by the active Ze implementation workflow.
10. **Completion** - update implementation audit, learned summaries, and selected spec state.

## Review Gate

| Lens | Required Review Question |
|------|--------------------------|
| Protocol | Does every RFC 5340 field have one owner and one validation path? |
| Security | Does RFC 7166 verify before ISM/NSM/LSDB and avoid logging secrets? |
| Concurrency | Are LSDB, neighbor, and route snapshots immutable when exposed? |
| Routing | Are OSPFv3 route choices resolved before Loc-RIB insertion? |
| Interop | Does every FRR scenario validate wire behavior, not only local state? |
| Architecture | Is all OSPFv3 code under `internal/plugins/ospfv3/` with no OSPFv2 shared wire package? |
| Docs | Do config, commands, plugin overview, functional tests, and wire docs all match implemented behavior? |

## Pre-Commit Verification

| Command or Check | Purpose |
|------------------|---------|
| `make ze-spec-status-json` | Spec metadata and status validation |
| `go test ./internal/plugins/ospfv3/...` | OSPFv3 unit coverage after implementation exists |
| `make ze-ospfv3-test` | Functional OSPFv3 suite |
| `test/interop/scenarios/ospfv3-p2p-frr/check.py` and siblings | FRR `ospf6d` interop |
| `test/web/ospfv3-neighbor-database.wb` | Web page/API behavior |
| `make ze-doc-test` | Documentation anchors and examples after docs are updated |
| `make ze-verify-fast` | Standard final verification gate when implementation is complete |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Complete OSPFv3 umbrella | done | `plan/spec-ospfv3-0-umbrella.md` | Umbrella has scope, child graph, files, tests, ACs, data flow, and verification matrix |
| Add first follow-up implementation spec | done | `plan/spec-ospfv3-1-types.md` | First child owns OSPFv3 leaf/domain types |
| Keep OSPFv3 separate from OSPFv2 | done | Scope, Architecture | Uses `internal/plugins/ospfv3/` and no shared wire package |
| Add RFC summaries | done | `rfc/short/rfc5340.md`, `rfc/short/rfc7166.md`, `rfc/short/rfc5838.md`, `rfc/short/rfc4552.md` | Summaries created for implementation handoff |
| Implement OSPFv3 runtime | pending | child specs 1 through 13 | This umbrella and first child are implementation specs, not runtime code |

### Goal Validation

| Goal | Evidence Type | Concrete Evidence |
|------|---------------|-------------------|
| OSPFv3 not silently included in OSPFv2 | spec scope | OSPFv2 umbrella depends on this follow-up for IPv6 |
| OSPFv3 has concrete implementation path | spec content | Child specs, files, tests, ACs, data flow, and verification matrix listed above |
| First implementation child is ready | spec content | `plan/spec-ospfv3-1-types.md` has ACs, files, tests, data flow, and verification |
| RFC decisions captured | RFC summaries | RFC 5340 base, RFC 7166 auth, RFC 5838 deferral, RFC 4552 deferral |
| Correct Ze placement | architecture | `internal/plugins/ospfv3/` is the only runtime package path in this spec |

## Known Limitations

- This umbrella does not implement runtime code. It coordinates implementation child specs.
- Only `spec-ospfv3-1-types.md` exists today; child specs 2 through 13 are written as the implementation proceeds.
- RFC 5838 and RFC 4552 are explicitly deferred.
- OSPFv3 virtual links, TE, SR, GR, BFD, and SNMP MIB are deferred.

## Cross-References

- `plan/spec-ospf-0-umbrella.md` - OSPFv2 sibling and pattern source.
- `plan/spec-ospfv3-1-types.md` - first follow-up implementation spec.
- `rfc/short/rfc5340.md` - OSPFv3 base protocol.
- `rfc/short/rfc7166.md` - OSPFv3 auth trailer.
- `rfc/short/rfc5838.md` - multi-AF deferral.
- `rfc/short/rfc4552.md` - IPsec deferral.
