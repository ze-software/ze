# Spec: ospf-ext-7 -- OSPF Virtual Links (RFC 2328 §15 / RFC 5340 §4.2)

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-ospf-ext-0-umbrella.md |
| Phase | - |
| Updated | 2026-06-24 |

> One feature, both address families. Ze runs OSPF as a single unified engine
> (`internal/plugins/ospf/`) exactly as `bgp` spans address families. IPv4
> (OSPFv2, RFC 2328) and IPv6 (OSPFv3, RFC 5340) are two **address families** of
> the one OSPF: the ISM/NSM, flooding, DR election, SPF, and LSDB sequencing are
> AF-neutral and SHARED; only the wire/LSA/prefix code differs, and that lives in
> the `_v6` strategy files plus the `internal/plugins/ospf/v3/{types,packet,transport}`
> leaves. There is NO separate `ospfv3` plugin and NO separate product. This spec
> replaces the two version-split virtual-link drafts with a single coherent
> feature spec; the shared engine behaviour is stated once and the per-AF wire/LSA
> differences are labelled with an **Address family** column or explicit IPv4/IPv6
> sub-rows.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/learned/972-ospf-af-unify.md` -- the one-engine, AF-strategy design (note: it lists a stale historical leaf path; the live AF-specific leaf path is `internal/plugins/ospf/v3/`)
4. `rfc/short/rfc2328.md` -- IPv4 family: §15 virtual links (unnumbered p2p in the backbone through a transit area), §16.1 (intra-area SPF yielding the virtual neighbour's cost + next hop), §16.3 (transit-area Summary-LSA pass + TransitCapability), §A.4.2 (Router-LSA bit V + link Type 4), §A.3.2/§A.3.3 (Hello Network Mask 0.0.0.0, DD Interface MTU 0 on virtual links), §C.4 (virtual-link parameters)
5. `rfc/short/rfc5340.md` -- IPv6 family: §2.9 (virtual-link packets use GLOBAL IPv6 unicast, routed, hop limit > 1), §4.2 virtual links, §3.5 (a fully adjacent virtual-link endpoint is backbone-attached), App A.4.3 (Router-LSA V-bit + `RouterLinkTypeVirtual` link record), App A.4.8 (no Link-LSA on virtual links), §C.2 (configurable parameters)
6. `plan/spec-ospf-ext-0-umbrella.md` -- the umbrella that defers virtual links; "Shared Contracts" (Router-LSA link records, LSDB key, SPF route table, area/interface model) and the note that virtual links are a SHOULD item layered on the stable base
7. `internal/plugins/ospf/lsdb/origination.go` -- `OriginInput.VirtualLinkEndpoint` (already sets `RouterFlagV`, currently never true) and `routerLinks(in)` (emits P2P/Transit/Stub records, never a Type-4 virtual link) -- the IPv4 origination seam
8. `internal/plugins/ospf/origination_v6.go` -- `v6OriginateSelf`, `v6RouterLSABody`, `v6OriginateRouter`; where the IPv6 virtual-link Router-LSA record and the backbone (area 0) Router-LSA must be added
9. `internal/plugins/ospf/afstrategy_v6.go` -- `v6RouterLinks` maps Router-LSA links into the shared SPF graph; `v6NextHop` resolves next-hops; the IPv6 virtual case + transit next-hop read land here
10. `internal/plugins/ospf/packet/lsa_router.go` and `internal/plugins/ospf/v3/packet/lsa_router.go` -- both codecs already define the V-bit (`RouterFlagV = 0x04`) and the virtual link record (`RouterLinkTypeVirtual = 4`) and round-trip them; no wire change in either family
11. `internal/plugins/ospf/spf/spf.go` -- the SHARED Dijkstra already treats `RouterLinkTypeVirtual` like a P2P link (`transitEdges`/`twoWayRouterLink`/`p2pNeighborAddress`, lines ~183/266/332); AF-neutral
12. `internal/plugins/ospf/spf/interarea.go` -- `IsABR` ("Virtual-link backbone repair is out of scope") and the inter-area pass; the §16.3 transit-area calculation / TransitCapability / §3.5 backbone-attachment are NOT implemented
13. `internal/plugins/ospf/spf/computer.go` -- per-area intra-area SPF then the inter-area pass; the transit-area `Result` the resolver reads
14. `internal/plugins/ospf/config.go` -- `ospfConfig`/`areaConfig`/`interfaceConfig`/`applyTree`; the `V6 *ospfConfig` IPv6 family sub-config; no virtual-link config exists yet
15. `internal/plugins/ospf/transport/transport.go` + `transport/backend_linux.go` -- IPv4 TX socket pins `IP_TTL = 1` (link-local); virtual links MUST route (TTL > 1)
16. `internal/plugins/ospf/v3/transport/transport.go` + `v3/transport/backend_linux.go` -- IPv6 `SendPacket` binds `LinkLocalSource()` per interface; virtual links MUST use a GLOBAL source + hop limit > 1, not bound to one link
17. `internal/plugins/ospf/instance.go` -- the unified `engine`: `interfaces`, `neighbors`, `lsdb`, `spf`, `openInterface`/`reconcile`, `originateSelfLSAs`, `lsdbTopology()`; both AF instances run the same engine

## Task

Add OSPF **virtual links** to the unified OSPF engine (`internal/plugins/ospf/`)
for **both address families**: IPv4 (OSPFv2, RFC 2328 §15) and IPv6 (OSPFv3,
RFC 5340 §4.2). A virtual link is a logical, unnumbered point-to-point link that
belongs to the **backbone** (Area 0.0.0.0) but runs *through* a non-backbone,
non-stub (and, for IPv6, non-NSSA) **transit area** between two Area Border
Routers. It exists to repair a partitioned backbone or to attach an ABR with no
physical backbone interface to Area 0. The virtual neighbour (configured by its
Router ID plus the transit Area ID) is reached over the transit area's intra-area
SPF shortest path; the virtual link's output cost equals that intra-area path
cost (it is computed, never configured); OSPF runs over it as a point-to-point
interface (Hellos, DD, LS Update/Ack, full adjacency) with configurable per-link
timers (HelloInterval, RouterDeadInterval, RxmtInterval, InfTransDelay); and the
endpoint advertises the link in its **backbone** Router-LSA as a virtual link
record while setting the Router-LSA V-bit.

**Shared (AF-neutral), stated once:** the ISM/NSM that forms the point-to-point
adjacency, the DD/LSReq/flooding exchange, the LSDB sequencing seam, the
intra-area SPF graph walk (already treats the virtual link record like P2P), the
transit-area-driven resolution of the virtual neighbour's reachability / cost /
next hop from the transit area's intra-area SPF `Result`, the §16.3 transit-area
Summary-LSA pass (TransitCapability) plus the §3.5 backbone-attachment condition,
and the synthetic backbone point-to-point interface lifecycle driven by SPF
results.

**Per address family (differs only in wire/LSA/transport):**

| Concern | IPv4 (OSPFv2, RFC 2328) | IPv6 (OSPFv3, RFC 5340) |
|---------|--------------------------|--------------------------|
| Endpoint addressing | the virtual neighbour's **transit-area IPv4 interface address**; LinkData = the local transit IPv4 interface address (§A.4.2) | the virtual neighbour's **global IPv6 address** + local global IPv6 source, resolved from the transit area's Intra-Area-Prefix-LSAs (§2.9, §4.2) |
| Transport send | unicast routed with **TTL > 1** on a path distinct from the TTL-1 link-local socket | unicast routed with **hop limit > 1**, GLOBAL source (not `LinkLocalSource()`), not bound to one ifindex (§2.9) |
| Router-LSA virtual record | Type-4 link record: LinkID = neighbour Router ID, LinkData = local transit address, Metric = transit cost (§A.4.2) | `RouterLinkTypeVirtual` record: Interface ID = virtual-interface ID, Neighbor Router ID = neighbour, Metric = transit cost; no IP address (App A.4.3) |
| Hello/DD field quirks | Hello Network Mask = 0.0.0.0; DD Interface MTU = 0 (§A.3.2/§A.3.3) | no Network Mask field; no Link-LSA on the virtual link (App A.4.8) |
| Inter-area effect | §16.3 transit pass improves backbone routes and resolves virtual next hops | §3.5 makes the endpoint backbone-attached; the §16.3 transit pass applies equally |
| Origination file | `internal/plugins/ospf/lsdb/origination.go` (`routerLinks`/`OriginInput`) | `internal/plugins/ospf/origination_v6.go` + `internal/plugins/ospf/afstrategy_v6.go` |
| Transport file | `internal/plugins/ospf/transport/` | `internal/plugins/ospf/v3/transport/` |

Both codecs already encode the V-bit and the virtual link record, the IPv4
origination input already carries a `VirtualLinkEndpoint` flag that flips the
V-bit (dead today), and the shared intra-area SPF graph walk already treats a
virtual link record exactly like a P2P link. What is missing in both families is
*activation*: configuration, the transit-area-driven discovery of the virtual
neighbour's reachability/cost/next hop, a synthetic backbone point-to-point
interface that forms the adjacency over routed IP, origination of the virtual
link record into the backbone Router-LSA, and the §16.3 transit pass (plus, for
IPv6, the §3.5 backbone-attachment condition) so destinations whose computed next
hop is a virtual link resolve to their real next hop.

The feature runs entirely inside the existing OSPF edge plugin and registers
through the plugin's own machinery. Removing the virtual-link configuration
removes all virtual-link behaviour in both families and leaves OSPF behaving
exactly as today.

### In scope (this spec)

| Item | Address family | Detail |
|------|----------------|--------|
| Virtual-link configuration | both | A `virtual-link` list keyed by (transit area, virtual-neighbour Router ID), with optional p2p timers (hello-interval, dead-interval, retransmit-interval, transmit-delay) and authentication inheriting from the transit area; resolved into `virtualLinkConfig` on `ospfConfig` (IPv4) and its `V6` family (IPv6) (RFC 2328 §15/§C.4, RFC 5340 §4.2/§C.2) |
| Transit-area reachability/cost/next hop | both (shared) | After the transit area's intra-area SPF, resolve each configured virtual neighbour: usable only if it is a reachable router vertex whose Router-LSA is non-MaxAge; link cost = the transit-area intra-area cost; next hop = the SPF-computed transit next hop toward the neighbour |
| Endpoint address resolution | per AF | IPv4: the local + remote transit IPv4 interface addresses. IPv6: the neighbour's reachable **global** IPv6 address (packet destination) + the local global source, from the transit Intra-Area-Prefix-LSAs |
| Synthetic backbone p2p interface | both (shared lifecycle) | A virtual interface bound to Area 0.0.0.0 running the p2p ISM/NSM (no DR/BDR, no Network-LSA, no Link-LSA), packets unicast/routed to the resolved neighbour address; IPv4 Hello Mask 0.0.0.0, DD MTU 0 |
| Routed transport send | per AF | IPv4: routed unicast TTL > 1, distinct from the TTL-1 link-local socket. IPv6: routed unicast hop limit > 1, GLOBAL source, not bound to one ifindex (§2.9) |
| Router-LSA virtual-link advertisement | per AF | When Full, originate the virtual link record into the **backbone** Router-LSA (Metric = transit cost) and set the V-bit; IPv4 Type-4 in `routerLinks`, IPv6 `RouterLinkTypeVirtual` in `v6OriginateRouter`; backbone-only in both |
| Transit-area Summary-LSA pass (§16.3) + TransitCapability | both (shared) | TransitCapability TRUE when a Type-1 / Router-LSA with the V-bit exists in the area; improve already-reachable backbone routes and resolve the *real* next hop for destinations whose next hop was a virtual link; discard unresolved virtual next hops |
| Backbone attachment | both (shared; §3.5 explicit for IPv6) | A Full virtual link to area 0 makes the endpoint backbone-attached for inter-area / ABR computation |
| Show / observability | both | IPv4: `show ospf virtual-links` + V-bit/Type-4 in `show ospf database router`. IPv6: the virtual interface in `show ipv6 ospf6 interface` and the virtual neighbour in `show ipv6 ospf6 neighbor`; virtual-link metrics in both |

### Out of scope (noted so it is not silently assumed done)

| Item | Address family | Where |
|------|----------------|-------|
| NBMA networks + the NBMA neighbour list | both | spec-ospf-ext-8 / spec-ospfv3-ext-7 (explicitly excluded here) |
| Point-to-multipoint network type | both | spec-ospf-ext-8 / spec-ospfv3-ext-7 |
| Sham links (MPLS VPN, RFC 4577) | IPv4 | not planned |
| Multi-AF (RFC 5838) virtual links | IPv6 | spec-ospfv3-ext-1; this spec is IPv6-unicast only |
| Authentication redesign | both | virtual links reuse the transit area's existing key-chain / trailer config (the existing `authStore`); no new auth machinery (IPv6 inherits the transit area's RFC 7166 trailer) |
| Stub / NSSA transit areas | both | rejected at config validation, not implemented as a feature (a virtual link MUST NOT transit a stub; for IPv6 also non-NSSA) |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
- [ ] `plan/learned/972-ospf-af-unify.md` -- the one-engine, AF-strategy design
  -> Decision: virtual links are ONE feature with shared AF-neutral engine behaviour and per-AF wire/transport seams; do NOT build a second engine or duplicate the ISM/NSM/SPF
  -> Constraint: AF-specific code lives in the `_v6` files and the `internal/plugins/ospf/v3/{types,packet,transport}` leaves; the learned doc's stale leaf path is historical -- the live path is `internal/plugins/ospf/v3/`
- [ ] `docs/research/ospf-implementation-guide.md` §6f "Virtual link" (~455) and §7 "Network Types" virtual-link row (~480, ~491) -- the model and the routed-IP requirement
  -> Decision: model a virtual link as a synthetic point-to-point interface bound to Area 0.0.0.0 whose packets traverse the transit area as normal routed IP (IPv4 TTL > 1; IPv6 hop limit > 1, global source), never as a link-local exchange
  -> Constraint: virtual links inherit authentication from the **transit** area, not from the backbone (~491); reuse the transit area's key-chain in both families
- [ ] `docs/research/ospf-implementation-guide.md` §6a SPF link-type table (~384) -- "Router-LSA link type 4 (virtual link): same as type 1 but through the transit area"
  -> Constraint: the SHARED SPF treats the virtual link record exactly like a P2P link (already true in `spf/spf.go`); the new work is *originating* the record per AF and feeding the cost/next hop from the transit-area SPF, not changing the graph walk
- [ ] `docs/research/ospf-implementation-guide.md` §13 "Implementation Roadmap" virtual-links note (~769, ~1610, ~1843) -- virtual links are a SHOULD item layered after the core is interop-green
  -> Constraint: the feature is additive in both families; a router with no `virtual-link` config behaves byte-for-byte as today (V-bit clear, no record, no §16.3 pass)
- [ ] `plan/spec-ospf-ext-0-umbrella.md` "Shared Contracts" + the deferred virtual-links row -- the contracts this feature extends and the umbrella row it closes
  -> Constraint: the LSDB key triple and the Router-LSA wire bodies are unchanged in both families; the virtual link adds one record to the existing backbone Router-LSA, no new LSA type
  -> Decision: the virtual neighbour adjacency reuses the existing neighbour state machine and LSDB flooding; only the transport path (unicast, routed) and the cost/next-hop source differ from a physical p2p interface
- [ ] `ai/rules/plugin-self-containment.md` -- the feature stays inside the OSPF plugin
  -> Constraint: virtual-link config, schema, validators, show commands, doctor checks, and metrics all live under `internal/plugins/ospf/`; no virtual-link spelling leaks into generic/central packages
- [ ] `ai/rules/no-sprintf-alloc.md` / `ai/rules/buffer-first.md` -- rendering and packet build
  -> Constraint: show output renders via `textbuf`/`AppendTo`; both virtual link records are emitted through the existing buffer-first Router-LSA `WriteTo` paths, not string concatenation

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc2328.md` §15 Virtual Links / §C.4 -- the IPv4 family
  -> Constraint: §15 -- a virtual link is an unnumbered point-to-point link belonging to the backbone, configured in BOTH endpoint ABRs by (other endpoint Router ID, transit area); its output cost and IP interface address are set DYNAMICALLY during the §16 SPF build; Router Priority is unused; the transit area MUST NOT be a stub
  -> Constraint: §A.3.2 -- a Hello on a virtual link carries Network Mask = 0.0.0.0; §A.3.3 -- a DD over a virtual link carries Interface MTU = 0
  -> Constraint: §A.4.2 -- the Router-LSA V-bit is set iff the router is an endpoint of a fully adjacent virtual link with the backbone as transit endpoint; the virtual link is a router-link of Type 4 with Link ID = the neighbour Router ID and Link Data = the local interface address used to reach it
  -> Constraint: §16.1 -- the virtual neighbour's reachability, cost, and next hop come from the transit area's intra-area shortest-path tree; an unreachable virtual neighbour means the link is DOWN
  -> Constraint: §16.3 -- ABRs attached to a transit area (TransitCapability TRUE) run a second pass over the transit area's Summary-LSAs that only IMPROVES already-reachable backbone routes and resolves real next hops for virtual next hops; unresolved virtual next hops are discarded afterward
  -> Constraint: §8.1 / IP encapsulation -- virtual-link packets are unicast and routed across the transit area (TTL large enough), unlike the TTL-1 link-local OSPF exchanges
- [ ] `rfc/short/rfc5340.md` §2.9 / §3.5 / §4.2 / App A.4.3 / App A.4.8 / §C.2 -- the IPv6 family
  -> Constraint: §2.9 -- OSPFv3 normally uses link-local source + `ff02::5`/`ff02::6`; virtual-link packets are the exception: unicast to the neighbour's GLOBAL IPv6 address, routed through the transit area (hop limit > 1), source = the local GLOBAL address reachable in the transit area
  -> Constraint: §4.2 -- a virtual link is a configured point-to-point link through a transit (non-backbone, non-stub, non-NSSA) area; it belongs to the backbone; both endpoints are ABRs of the transit area; it is up only while the neighbour is reachable by an intra-area path in the transit area
  -> Constraint: §3.5 -- a router that is the endpoint of a fully adjacent virtual link is considered to have an interface to the backbone (this is what makes inter-area routing treat it as backbone-attached)
  -> Constraint: App A.4.3 -- the Router-LSA V-bit (`RouterFlagV`, 0x04) is set by a virtual-link endpoint; the virtual link is a `RouterLinkTypeVirtual` (4) record with Neighbor Router ID = the neighbour and metric = the transit-area path cost; App A.4.8 -- there is no Link-LSA on a virtual link
  -> Constraint: §C.2 -- the configurable parameters are the transit area, the neighbour Router ID, and the interface timers; the cost is NOT configured -- it is the transit-area SPF path cost (mirrors RFC 2328 §15)

**Key insights:** (minimal context to resume after compaction)
- Both codecs already encode the V-bit and the virtual link record; the IPv4 `OriginInput.VirtualLinkEndpoint` already flips the V-bit; the SHARED SPF already treats the record as P2P. The gap in BOTH families is *activation*: config, the synthetic interface, the cost/next-hop source (transit-area SPF), the per-AF record emission, the §16.3 transit pass, and (IPv6) §3.5 backbone attachment.
- A virtual link is "a p2p interface in Area 0 whose cost and next hop are computed, not configured, and whose packets are routed rather than link-local." The neighbour state machine, DD exchange, flooding, and SPF graph walk are reused unchanged across both families.
- The genuinely AF-specific, genuinely-absent pieces are the two routed transports: IPv4 (the TX socket pins `IP_TTL = 1`) and IPv6 (the send binds `LinkLocalSource()` per interface). Everything above the transport is shared or already present.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)

Shared / AF-neutral:
- [ ] `internal/plugins/ospf/spf/spf.go` -- `transitEdges` (~183), `twoWayRouterLink` (~266), `p2pNeighborAddress` (~332) already match `RouterLinkTypeVirtual` alongside `RouterLinkTypeP2P`; the virtual neighbour becomes a reachable router vertex the moment the record exists
  -> Constraint: the intra-area graph walk needs NO change in either family; the §16.3 transit pass is a NEW stage, not an edit to `transitEdges`
- [ ] `internal/plugins/ospf/spf/interarea.go` -- `IsABR` carries "Virtual-link backbone repair is out of scope"; the inter-area pass computes IA routes from backbone Summary-LSAs but has no §16.3 transit pass, no TransitCapability, no §3.5 virtual backbone-attachment
  -> Constraint: add the §16.3 transit pass here (or a sibling `transitarea.go`); update the comment; TransitCapability per area = a Router-LSA in that area has the V-bit; thread the §3.5 condition (a Full virtual link to area 0 = backbone-attached) into ABR/inter-area logic for both families
- [ ] `internal/plugins/ospf/spf/computer.go` -- `Computer` runs per-area intra-area SPF then the inter-area pass; the transit-area `Result` is the read source for virtual-neighbour resolution; the §16.3 pass runs after inter-area, before install
  -> Constraint: virtual-neighbour resolution (reachability/cost/next hop) is a read of the transit area's `Result`; expose it so the engine can drive the synthetic interface
- [ ] `internal/plugins/ospf/config.go` -- `ospfConfig`/`areaConfig`/`interfaceConfig`/`applyTree`; the `V6 *ospfConfig` IPv6 family sub-config; there is NO virtual-link parsing and no virtual-link config type
  -> Constraint: add a `virtualLinkConfig` type and a `VirtualLinks []virtualLinkConfig` field surfaced on `ospfConfig` (IPv4) and its `V6` family (IPv6); validate the transit area exists, is not a stub (IPv6 also non-NSSA, non-backbone), the router is an ABR, and the neighbour Router ID is not self
- [ ] `internal/plugins/ospf/instance.go` -- the unified `engine` owns `interfaces`, `neighbors`, `lsdb`, `spf`; `openInterface`/`reconcile` open physical interfaces; `lsdbTopology()` builds the topology snapshot; `originateSelfLSAs()` regenerates self-LSAs; both AF instances run this engine
  -> Constraint: a virtual link is a synthetic interface the engine creates/destroys on reachability change (not a netlink interface); the engine drives its lifecycle from SPF results and triggers `originateSelfLSAs` on Full/Down change; the synthetic-interface lifecycle is shared, the transport adapter is AF-specific
- [ ] `internal/plugins/ospf/neighbor/` + `internal/plugins/ospf/iface/` -- the adjacency table keyed by (interface name, Router ID); `AddressOf` returns the neighbour's reachable address; the ISM has a point-to-point state with no DR election; flooding is per interface
  -> Constraint: the virtual interface registers under a synthetic name and runs the standard point-to-point adjacency; the recorded neighbour "address" is the resolved unicast destination (IPv4 transit address / IPv6 global address), distinct from the SPF transit next hop

IPv4 family:
- [ ] `internal/plugins/ospf/packet/lsa_router.go` -- `RouterFlagV = 0x04`, `RouterLinkTypeVirtual = 4`; `RouterLSA.WriteTo`/`DecodeRouterLSA` round-trip the V-bit and Type-4 link record
  -> Constraint: the IPv4 codec is complete for virtual links; do NOT touch the wire format -- feed it correct values
- [ ] `internal/plugins/ospf/lsdb/origination.go` -- `OriginInput.VirtualLinkEndpoint bool` already sets `flags |= packet.RouterFlagV` in `OriginateRouter`; `routerLinks(in)` emits only P2P, Transit, and Stub records -- never a Type-4 virtual record
  -> Constraint: `OriginInput` is the seam: add virtual-link records so `routerLinks` emits Type-4 records and `VirtualLinkEndpoint` is set when any virtual link is Full; backbone Router-LSA only
- [ ] `internal/plugins/ospf/transport/transport.go` + `transport/backend_linux.go` -- `SendPacket(name, dst, payload)` sends to a unicast `dst`; the TX socket sets `IP_TTL = 1` (~215) for link-local OSPF
  -> Constraint: virtual-link packets MUST go out routed (TTL > 1). The current TX socket pins TTL 1, so add a virtual-link send path (a separate routed socket or a per-send TTL override) bound to the egress transit interface toward the neighbour's transit address

IPv6 family:
- [ ] `internal/plugins/ospf/v3/packet/lsa_router.go` -- `RouterFlagV = 0x04`, `RouterLinkTypeVirtual = 4`, `RouterLink{Type, Metric, InterfaceID, NeighborInterfaceID, NeighborRouterID}`; `RouterLSA.WriteTo` already serializes any link type
  -> Constraint: the IPv6 codec is complete; this spec only CONSTRUCTS the virtual record in origination -- no codec change
- [ ] `internal/plugins/ospf/origination_v6.go` -- `v6OriginateSelf` builds per-area Router-LSAs; `v6RouterLSABody` emits P2P + transit links; `v6IsAreaBorderRouter(areas)`; there is NO virtual-link record and the V-bit is never set
  -> Constraint: the virtual link is advertised in the **backbone** Router-LSA; `v6OriginateSelf` learns active virtual links (Full + resolved neighbour) and injects a `RouterLinkTypeVirtual` record into area 0's body, setting `RouterFlagV`; a router with only a virtual link to area 0 still counts as backbone-attached for `v6IsAreaBorderRouter`
- [ ] `internal/plugins/ospf/afstrategy_v6.go` -- `v6RouterLinks` translates P2P + transit links into the shared SPF graph but DROPS other link types (no `RouterLinkTypeVirtual` case); `v6NextHop.P2PNextHop` resolves via `neighbors.AddressOf`
  -> Constraint: add a `RouterLinkTypeVirtual` case to `v6RouterLinks` (the shared Dijkstra already treats it as P2P); the virtual next hop must resolve to the transit-area next hop toward the neighbour, not the neighbour's global packet address
- [ ] `internal/plugins/ospf/v3/transport/transport.go` + `v3/transport/backend_linux.go` + `v3/transport/backend_other.go` -- `SendPacket` requires `dst.Is6()`, looks up the per-interface socket, sends from `LinkLocalSource()` with the interface bound; `FinalizePacketChecksum(src, dst, payload)` uses that link-local source; `resolveOSPFv3Interface` requires a link-local source; `listenNetwork = "ip6:89"`
  -> Constraint: the virtual-link send MUST NOT use `LinkLocalSource()`; it needs the local GLOBAL source (so the checksum pseudo-header matches) and a routed hop limit > 1; this is a NEW transport entry point (routed send / virtual-interface handle), not a tweak to `SendPacket`

**Behavior to preserve:**
- The Router-LSA wire formats and the LSDB key triple in both families; physical-interface origination, flooding, and intra-area SPF; the inter-area pass (§16.2); the existing OSPFv2 and OSPFv3 functional and interop tests.
- A router with no `virtual-link` config in either family: the V-bit stays clear, no virtual record is originated, no §16.3 pass runs, and the existing send paths (IPv4 TTL-1 link-local; IPv6 link-local source + `ff02::5`/`ff02::6`) are unchanged.
- The neighbour state machine, DD exchange (only the IPv4 MTU=0 / Mask=0 fields differ on virtual links), and the `authStore` (virtual links reuse the transit area's key-chain / RFC 7166 trailer).
- The shared Dijkstra (already virtual-aware) and the v6 next-hop resolution for normal P2P/transit links.

**Behavior to change:** (all RFC-required, not discretionary)
- Origination (per AF): emit a virtual link record and set the V-bit when a virtual link to the backbone is Full -- IPv4 `routerLinks`/`OriginInput`, IPv6 `v6OriginateSelf`/`v6RouterLSABody`.
- SPF (shared): add the §16.3 transit-area Summary-LSA pass and TransitCapability; expose virtual-neighbour reachability/cost/next hop from the transit-area intra-area result; thread the §3.5 backbone-attachment condition.
- SPF strategy (IPv6): `v6RouterLinks` gains the `RouterLinkTypeVirtual` case + the transit next-hop resolution.
- Transport (per AF): IPv4 routed (TTL > 1) unicast send path; IPv6 routed (hop limit > 1) unicast send path with a global source.
- DD/Hello on a virtual link (IPv4): Interface MTU = 0, Network Mask = 0.0.0.0.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Config:** a `virtual-link` entry (transit area + virtual-neighbour Router ID + optional timers) -> `applyTree` -> `ospfConfig.VirtualLinks` (IPv4) / the `V6` family's `VirtualLinks` (IPv6).
- **SPF result:** the transit area's intra-area SPF (§16.1) yields whether the virtual neighbour is reachable, its cost, and its next hop -> drives the synthetic interface up/down and the link cost (shared).
- **Reception:** OSPF packets from the virtual neighbour arrive unicast/routed on the transit egress and are demultiplexed to the synthetic virtual interface by source Router ID + Area 0.0.0.0 (shared logic; IPv4 vs IPv6 socket families differ).

### Transformation Path
1. **Config resolve (new, shared shape):** `applyTree` parses `virtual-link`; validation rejects a stub (IPv6 also NSSA/backbone) transit area, an absent transit area, a non-ABR, and a self Router ID.
2. **Transit-area SPF read (new, shared):** after the transit area's intra-area SPF, the engine reads each virtual neighbour's `(reachable, cost, transitNextHop, neighbourAddress, localAddress)` from the transit-area `Result`. IPv4 addresses are transit interface addresses; IPv6 addresses are global addresses from the Intra-Area-Prefix-LSAs.
3. **Synthetic interface lifecycle (new, shared):** on `reachable` true, the engine creates/keeps a synthetic backbone p2p `Interface` (NetworkType p2p, timers from config; IPv4 Mask 0.0.0.0 / MTU 0) whose Sender routes to the neighbour address; on `reachable` false, it tears the interface (and adjacency) down.
4. **Adjacency (reused, shared):** the synthetic interface runs the p2p neighbour state machine; the adjacency reaches Full via the normal Hello/DD/LSReq/flooding path over routed IP.
5. **Routed send (new, per AF):** outgoing virtual-link packets go through the AF routed-unicast path -- IPv4 TTL > 1; IPv6 global source + hop limit > 1, checksum pseudo-header against the global source.
6. **Router-LSA origination (changed, per AF):** the backbone area's origination includes virtual links -- IPv4 `routerLinks` emits a Type-4 record + sets `VirtualLinkEndpoint`; IPv6 `v6OriginateSelf` injects a `RouterLinkTypeVirtual` record + sets `RouterFlagV`; Metric = transit cost; backbone-only; both via the existing buffer-first `WriteTo`.
7. **Backbone SPF (reused, shared):** the backbone intra-area SPF graph walk treats the virtual record like P2P; the virtual neighbour becomes a reachable backbone router vertex, repairing the backbone. For IPv6, `v6RouterLinks` maps the record in and resolves the transit next hop.
8. **§16.3 transit pass + §3.5 (new, shared):** for each transit area with TransitCapability, a second pass over its Summary-LSAs improves already-reachable backbone routes and rewrites virtual-link next hops to the real transit next hop; unresolved virtual next hops are discarded; a Full virtual link to area 0 marks the endpoint backbone-attached.
9. **Install (reused, shared):** the route delta installs through the existing Loc-RIB seam.
10. **Show (new, per AF):** IPv4 `show ospf virtual-links`; IPv6 the virtual interface/neighbour rows in `show ipv6 ospf6 interface`/`neighbor`.

### Boundaries Crossed
| Boundary | How | Address family | Verified |
|----------|-----|----------------|----------|
| Config <-> engine | `virtual-link` list -> `ospfConfig.VirtualLinks` / `V6` family; reconcile creates/destroys synthetic interfaces | both | [ ] |
| Transit-area SPF <-> virtual link | read `(reachable, cost, nextHop, neighbourAddr, localAddr)` from the transit-area intra-area `Result` (read-only) | both (shared) | [ ] |
| Engine <-> synthetic interface | a backbone p2p `Interface` with a routing Sender bound to the transit egress | both (shared) | [ ] |
| Synthetic interface <-> neighbour/LSDB | reuses the neighbour state machine, DD, and flooding unchanged | both (shared) | [ ] |
| Virtual link <-> Router-LSA | IPv4 `OriginInput` -> `routerLinks` Type-4 + V-bit; IPv6 `v6OriginateSelf` -> `RouterLinkTypeVirtual` + V-bit; backbone-only | per AF | [ ] |
| Backbone SPF <-> §16.3 transit pass + §3.5 | new transit-area Summary-LSA pass + TransitCapability + backbone-attachment; rewrites/discards virtual next hops | both (shared) | [ ] |
| Transport <-> routed IP | IPv4 routed TTL > 1 path; IPv6 routed hop-limit > 1 global-source path | per AF | [ ] |

### Integration Points
- `internal/plugins/ospf/config.go` -- `virtualLinkConfig`, `VirtualLinks` on `ospfConfig` and the `V6` family, parse + validate (shared shape).
- `internal/plugins/ospf/lsdb/origination.go` -- IPv4 `OriginInput` virtual records; `routerLinks` Type-4 emission; `VirtualLinkEndpoint` set from Full state.
- `internal/plugins/ospf/origination_v6.go` + `internal/plugins/ospf/afstrategy_v6.go` -- IPv6 backbone Router-LSA virtual record + V-bit; the `v6RouterLinks` virtual case + transit next-hop resolution; backbone attachment via `v6IsAreaBorderRouter`.
- `internal/plugins/ospf/spf/` -- the §16.3 transit pass + TransitCapability + §3.5 (`transitarea.go` or extend `interarea.go`); the virtual-neighbour resolution exposed from the transit-area `Result` (shared).
- `internal/plugins/ospf/iface` + `internal/plugins/ospf/neighbor` -- reused for the synthetic interface and the standard p2p adjacency (shared).
- `internal/plugins/ospf/transport/` (IPv4) and `internal/plugins/ospf/v3/transport/` (IPv6) -- the two routed send paths.
- `internal/plugins/ospf/instance.go` -- the synthetic-interface lifecycle and virtual-link manager driven by SPF results; backbone origination input; virtual-link packet demux (shared lifecycle, AF transport adapter).
- `internal/plugins/ospf/cmd_show.go` -- IPv4 `show ospf virtual-links` and V-bit/Type-4 in `show ospf database router`; IPv6 virtual interface/neighbour rows.

### Architectural Verification
- [ ] No bypassed layers (virtual-link packets flow config -> synthetic interface -> neighbour SM -> LSDB/flooding -> SPF, the same spine as a physical p2p interface in both families; only the cost/next-hop source and the routed transport differ)
- [ ] No unintended coupling (virtual links read the transit-area SPF result read-only; the transit area is unaware of the backbone repair; SPF stays AF-neutral)
- [ ] No duplicated functionality (reuses the per-AF codec V-bit/virtual record, the shared neighbour state machine, DD, flooding, and the shared intra-area SPF graph walk; adds only config, the synthetic interface, the two routed send paths, the per-AF record emission, and the shared §16.3/§3.5 logic)
- [ ] Zero-copy / buffer-first preserved (both virtual records emitted through the existing Router-LSA `WriteTo`; show output via `textbuf`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Address family | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|----------------|--------------------------------|----------|--------------|--------|
| A-1 | Both codecs round-trip the V-bit and the virtual link record, so no wire-format work is needed | both | `packet/lsa_router.go`, `v3/packet/lsa_router.go` `RouterFlagV`/`RouterLinkTypeVirtual` | wire work + scope creep | `TestRouterLSAVirtualLinkRoundTrip` (v4) + `TestV6RouterLSAVirtualRecordRoundTrip` (v6) | unvalidated |
| A-2 | The shared intra-area SPF reaches a virtual neighbour as a router vertex with no change, because the graph walk already matches `RouterLinkTypeVirtual` | both (shared) | `spf/spf.go` lines ~183/266/332 | the graph walk needs a virtual branch | `TestBackboneSPFReachesVirtualNeighbor` (v4) + `TestV6BackboneGraphIncludesVirtualLink` (v6) | unvalidated |
| A-3 | The transit area's intra-area SPF `Result` already contains the virtual neighbour's cost and next hop, so resolution is a read, not a recompute | both (shared) | `spf/computer.go` per-area `Result`; `spf/route.go` node results | a separate transit-area shortest-path computation is needed | `TestVirtualNeighborResolvedFromTransitSPF` | unvalidated |
| A-4 | The IPv4 `OriginInput.VirtualLinkEndpoint` flipping the V-bit is correct and sufficient; only the Type-4 emission in `routerLinks` is missing | IPv4 | `lsdb/origination.go` `OriginateRouter` ORs `RouterFlagV` | the V-bit semantics differ | `TestBackboneRouterLSAHasVBitWhenVLFull` | unvalidated |
| A-5 | The IPv6 global address of the virtual neighbour is resolvable from the transit-area SPF result + its Intra-Area-Prefix-LSA | IPv6 | `afstrategy_v6.go` `v6BuildRoutes`, `v6InterfacePrefixes` | a separate address-learning mechanism is needed | `TestVirtualEndpointResolvesGlobalAddress` | unvalidated |
| A-6 | A virtual link reuses the neighbour state machine and DD/flooding unchanged by presenting as a p2p interface | both (shared) | `iface/`+`neighbor/` p2p path; §A.3.2/§A.3.3 (v4) | the neighbour/DD code needs virtual-link special cases beyond Mask/MTU | `TestVirtualLinkAdjacencyReachesFull` (v4) + `TestVirtualAdjacencyReachesFull` (v6) | unvalidated |
| A-7 | A routed IPv4 (TTL > 1) send path can be added without disturbing the TTL-1 link-local path | IPv4 | `transport/backend_linux.go` sets `IP_TTL = 1` | the two paths conflict; a deeper transport refactor | `TestVirtualLinkSendUsesRoutedTTL` | unvalidated |
| A-8 | A routed IPv6 (hop limit > 1) send with a non-link-local global source is possible via the existing `golang.org/x/net/ipv6` control-message path | IPv6 | `v3/transport/backend_linux.go` uses `golang.org/x/net/ipv6`; `Send(dst, src, payload)` takes an explicit src | a new socket type / kernel feature is required | `TestRoutedSendUsesGlobalSourceAndHopLimit` | unvalidated |
| A-9 | The §16.3 transit pass only improves already-reachable routes and resolves virtual next hops; it never makes new destinations reachable | both (shared) | RFC 2328 §16.3; guide §6f | over-broad route changes / loops | `TestTransitAreaPassOnlyImprovesReachable`, `TestVirtualNextHopResolvedOrDiscarded` | unvalidated |
| A-10 | The transit area for a virtual link must not be a stub (IPv6 also NSSA/backbone), the router must be an ABR, and the neighbour Router ID must not be self; these are config-time rules | both | RFC 2328 §15 / RFC 5340 §4.2; `config.go` validate path | invalid configs accepted; runtime breakage | `TestVirtualLinkRejectStubTransit`, `TestVirtualLinkRejectNonABR`, `TestVirtualLinkRejectsSelfRouterID` | unvalidated |
| A-11 | Virtual links inherit the transit area's authentication via the existing `authStore`, with no new key surface (IPv6 reuses the transit area's RFC 7166 trailer) | both | guide §7 (~491); `instance.go` `authStore`; the v6 auth path | a separate virtual-link key surface is needed | `TestVirtualLinkUsesTransitAreaAuth` | unvalidated |
| A-12 | A Full virtual link to area 0 makes the endpoint backbone-attached for inter-area/ABR computation (§3.5) without breaking the real-backbone path | both (shared; §3.5 explicit IPv6) | `spf/interarea.go`, `v6IsAreaBorderRouter` | partitioned-backbone routes never appear; §3.5 unmet | `TestVirtualLinkBackboneAttachment` | unvalidated |
| A-13 | The virtual next hop (for route install) is the transit-area next hop toward the neighbour, distinct from the neighbour's unicast packet destination | both (shared) | `afstrategy_v6.go` `v6NextHop`; RFC 2328 §16.1 / RFC 5340 §4.2 | routes point at an unreachable address | `TestVirtualNextHopIsTransitNextHop` | unvalidated |

### Risks
| ID | Risk | Address family | Early signal | Mitigation / fallback |
|----|------|----------------|--------------|----------------------|
| R-1 | The link-local send path drops virtual-link packets (they need routing across the transit area), so the adjacency never forms | both | the virtual neighbour stays in ExStart/Down; FRR shows no virtual-link Hello | dedicated routed send paths (IPv4 TTL > 1; IPv6 global source + hop limit > 1); the per-AF interop forms Full |
| R-2 | The IPv6 routed send copies the link-local source, so the checksum pseudo-header mismatches and FRR drops the packet | IPv6 | FRR logs "bad checksum"; adjacency stuck in Init | the routed send takes an explicit global source and finalizes the checksum against it; `TestRoutedSendUsesGlobalSourceAndHopLimit` + interop |
| R-3 | The §16.3 transit pass makes new destinations reachable or creates a routing loop (violates "improve only") | both | a destination reachable only via the transit pass; traffic loops in the transit area | strictly gate the pass to already-reachable backbone routes; `TestTransitAreaPassOnlyImprovesReachable` |
| R-4 | A destination whose next hop is a virtual link is installed with the unroutable virtual next hop instead of the real transit next hop | both | the FIB has a route pointing at the virtual neighbour Router ID, not a real address | resolve the real next hop in the §16.3 pass; discard unresolved virtual next hops; `TestVirtualNextHopResolvedOrDiscarded` |
| R-5 | The virtual record is originated into a non-backbone Router-LSA (it must be backbone-only) | both | the V-bit / virtual record appears in a transit-area Router-LSA; FRR rejects | emit virtual records only into the Area 0.0.0.0 origination input; `TestVirtualLinkBackboneOnly` (v4) + `TestVirtualRecordInBackboneRouterLSA` (v6) |
| R-6 | The endpoint resolver caches a stale address after the transit topology changes, so packets go to a dead destination | both (IPv6 acute) | adjacency flaps after an unrelated transit SPF run | resolve on EVERY transit-area SPF run; mark Down when unreachable; `TestVirtualEndpointReresolvesOnSPF`, `TestVirtualLinkCostTracksTransitTopology` |
| R-7 | The virtual link flaps when the transit cost changes, churning the backbone Router-LSA and SPF | both | frequent Router-LSA reoriginations; SPF storms | drive the synthetic interface from settled SPF results; reuse the SPF throttle + MinLSInterval; `TestVirtualLinkCostUpdateNoFlap` |
| R-8 | A virtual link counts as backbone attachment even when not Full, so a half-formed link wrongly makes the router an ABR | both | spurious B-bit / inter-area routes during bring-up | backbone-attachment requires Full; `TestVirtualLinkBackboneAttachment` gates on state |
| R-9 | Demux ambiguity: a routed packet from the virtual neighbour matches the wrong physical interface | both | the adjacency forms on the wrong interface or not at all | demux by source Router ID + backbone Area ID to the synthetic interface; `TestVirtualLinkPacketDemux` |
| R-10 | Two virtual links share a transit egress and collide on the routed socket / next hop, or the synthetic name/ID clashes with a real interface | both | one link's packets go to the other neighbour; a virtual interface shadows a real one | key the synthetic interface and send path by (transit area, neighbour) from a reserved namespace; `TestTwoVirtualLinksSameTransit`, `TestVirtualInterfaceNameReserved` |
| R-11 | An interop mismatch: FRR's OSPFv3 virtual link expects a specific Interface-ID allocation or unnumbered behaviour Ze does not match | IPv6 | FRR shows the virtual neighbour stuck in ExStart / DD mismatch | follow FRR's virtual Interface-ID handling; key the two-way check on Neighbor Router ID; the `ospfv3-vlink-frr` scenario validates against `ospf6d` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Address family | Test |
|-------------|---|--------------|----------------|------|
| A `virtual-link` config entry (transit area + neighbour Router ID) | -> | `applyTree` resolves `VirtualLinks` on `ospfConfig` / the `V6` family; reconcile creates a synthetic interface when the neighbour is reachable | both | `TestParseVirtualLinkConfig` (unit) + `test/ospf/ospf-virtual-link-config.ci` + `test/ospfv3/ospfv3-vlink-config.ci` |
| The transit area's SPF settles with the virtual neighbour reachable | -> | the engine resolves cost/next hop and brings the synthetic backbone interface up | both (shared) | `TestVirtualNeighborResolvedFromTransitSPF` (unit) + `test/ospf/ospf-virtual-link-up.ci` + `test/ospfv3/ospfv3-vlink.ci` |
| The virtual-link adjacency reaches Full | -> | the backbone Router-LSA gets the virtual record + V-bit | IPv4 | `TestBackboneRouterLSAHasVBitWhenVLFull` (unit) + observed in `ospf-virtual-link-frr` |
| The virtual-link adjacency reaches Full | -> | `v6OriginateSelf` emits the backbone Router-LSA with `RouterFlagV` + the `RouterLinkTypeVirtual` record | IPv6 | `TestVirtualRecordInBackboneRouterLSA` (unit) + `ospfv3-vlink-frr` |
| The backbone SPF runs with the virtual record present | -> | the virtual neighbour becomes a reachable backbone vertex; the §16.3 pass resolves virtual next hops; §3.5 backbone-attachment holds | both (shared) | `TestBackboneSPFReachesVirtualNeighbor`, `TestV6BackboneGraphIncludesVirtualLink`, `TestVirtualNextHopResolvedOrDiscarded` (unit) + `test/ospf/ospf-virtual-link-route.ci` |
| An outgoing virtual-link packet | -> | IPv4 routed TTL > 1 / IPv6 routed global-source hop limit > 1 send, not the link-local path | per AF | `TestVirtualLinkSendUsesRoutedTTL` (v4), `TestRoutedSendUsesGlobalSourceAndHopLimit` (v6) |
| `show ospf virtual-links` / `show ipv6 ospf6 interface` | -> | the engine renders virtual-link / virtual-interface state | per AF | `test/ospf/ospf-virtual-link-show.ci`, `test/ospfv3/ospfv3-vlink.ci` |

## Acceptance Criteria

| AC ID | Address family | Input / Condition | Expected Behavior |
|-------|----------------|-------------------|-------------------|
| AC-1 | both | A `virtual-link` entry naming a transit area and a virtual-neighbour Router ID | parsed into `VirtualLinks` (IPv4 on `ospfConfig`, IPv6 on the `V6` family); optional hello/dead/retransmit/transmit-delay timers resolved with documented defaults (RFC 2328 §C.4 / RFC 5340 §C.2) |
| AC-2 | both | A `virtual-link` whose transit area is a stub (IPv6 also NSSA or backbone 0.0.0.0), absent, or whose neighbour Router ID equals this router's own | rejected at config validation with a clear error; no virtual link is created (RFC 2328 §15 / RFC 5340 §4.2) |
| AC-3 | both | A `virtual-link` on a router that is not an ABR | rejected at config validation |
| AC-4 | both (shared) | The virtual neighbour is reachable as a router vertex in the transit area's intra-area SPF | the virtual link's cost = the transit-area intra-area cost and its next hop = the transit-area next hop toward the neighbour; the synthetic backbone interface is brought up |
| AC-5 | both (shared) | The virtual neighbour is not reachable in the transit area | the virtual link stays Down; no virtual record is originated; no adjacency is attempted; it is re-resolved on the next transit SPF |
| AC-6 | IPv4 | The virtual-link adjacency over the transit area | reaches Full via the p2p neighbour state machine; Hellos carry Network Mask 0.0.0.0 and DD packets carry Interface MTU 0 (§A.3.2/§A.3.3); packets are unicast and routed (TTL > 1) to the neighbour's transit address |
| AC-7 | IPv6 | The virtual-link adjacency over the transit area | reaches Full via the standard point-to-point DD/LSREQ exchange (no DR/BDR, no Network-LSA, no Link-LSA); packets go to the neighbour's GLOBAL IPv6 address from the local GLOBAL source with hop limit > 1, routed (not bound to one ifindex), the checksum pseudo-header using the global source (§2.9, §4.2, App A.4.8) |
| AC-8 | IPv4 | A virtual link is Full | the **backbone** Router-LSA contains a Type-4 link record (LinkID = neighbour Router ID, LinkData = local transit address, Metric = transit cost) and the V-bit is set; no virtual record or V-bit appears in any non-backbone Router-LSA (§A.4.2) |
| AC-9 | IPv6 | A virtual link is Full | the **backbone** Router-LSA sets `RouterFlagV` and carries a `RouterLinkTypeVirtual` record (Interface ID = virtual-interface ID, Neighbor Router ID = neighbour, Metric = transit cost); backbone-only (App A.4.3, §4.2) |
| AC-10 | both | The backbone Router-LSA now contains the virtual record | the backbone intra-area SPF reaches the virtual neighbour as a router vertex (two-way check honoured via the reciprocal record), repairing a partitioned backbone or attaching the remote area to Area 0 |
| AC-11 | both | An ABR whose only path to area 0 is a Full virtual link | it is treated as backbone-attached: it participates in inter-area route computation and inter-area routes that depend on the repaired backbone appear (RFC 5340 §3.5; RFC 2328 §15) |
| AC-12 | both (shared) | A transit area has a Router-LSA with the V-bit (TransitCapability TRUE) | the §16.3 transit-area Summary-LSA pass runs for that area: it only improves already-reachable backbone routes and never makes a new destination reachable |
| AC-13 | both (shared) | A destination whose §16 next hop is a virtual link | the §16.3 pass rewrites it to the real transit next hop; if it cannot be resolved, the destination is discarded (not installed with a virtual next hop) |
| AC-14 | both | The transit-area cost to the virtual neighbour changes | the next SPF run updates the virtual link's cost, reoriginates the backbone Router-LSA virtual metric, and recomputes backbone routes; no flap when the cost is unchanged |
| AC-15 | IPv4 | `show ospf virtual-links` | lists each configured virtual link with transit area, neighbour Router ID, adjacency state, computed cost, and next hop; `show ospf database router` shows the V-bit + Type-4 link |
| AC-16 | IPv6 | `show ipv6 ospf6 interface` / `show ipv6 ospf6 neighbor` while a virtual link is Full | the virtual interface and the virtual neighbour are listed with state and the resolved global address; a Down virtual link shows Down |
| AC-17 | both | No `virtual-link` is configured | OSPF behaves byte-for-byte as today in both families: V-bit clear, no virtual record, no §16.3 pass, the existing link-local send paths unchanged |
| AC-18 | both | A virtual link's authentication | uses the transit area's configured key-chain / RFC 7166 trailer (the existing `authStore`), not a separate virtual-area key |

## End-to-End User Stories (MANDATORY for new features)

| # | Address family | User does | Path through system | Test proving it works |
|---|----------------|-----------|--------------------|-----------------------|
| 1 | IPv4 | Configures a virtual link through a transit area to attach a remote area to Area 0 | config -> `ospfConfig.VirtualLinks` -> transit-area SPF resolves -> synthetic interface up -> adjacency Full -> Type-4 in backbone Router-LSA -> backbone SPF reaches the neighbour | `test/ospf/ospf-virtual-link-up.ci` + `ospf-virtual-link-frr` |
| 2 | IPv6 | Configures a virtual link under a transit area on both endpoints | config -> `V6` family `VirtualLinks` -> resolver -> virtual interface -> NSM Full; `show ipv6 ospf6 neighbor` shows the virtual neighbour Full | `test/ospfv3/ospfv3-vlink.ci` |
| 3 | IPv4 | Forms a virtual-link adjacency with FRR `ospfd` across a transit area | unicast routed Hello/DD/LSU (TTL > 1) -> p2p neighbour SM -> Full; FRR's `show ip ospf virtual-links` shows the link up | `ospf-virtual-link-frr` interop |
| 4 | IPv6 | Forms a virtual-link adjacency with FRR `ospf6d` across a transit area | routed-unicast OSPFv3 packets (global src/dst, hop limit > 1) traverse the transit area -> point-to-point NSM -> Full; FRR shows Ze's backbone Router-LSA with the V-bit | `ospfv3-vlink-frr` interop (multi-hop QEMU) |
| 5 | both | Repairs a partitioned backbone / an ABR with no physical backbone interface | two backbone fragments joined only through a transit area -> virtual link Full -> virtual records both ways -> backbone SPF treats the fragments as connected; a destination in one fragment is reachable from the other | `test/ospf/ospf-virtual-link-route.ci` + `test/ospfv3/ospfv3-vlink-backbone-repair.ci` + both interop scenarios |
| 6 | both | Inspects the virtual link | CLI -> `show ospf virtual-links` (IPv4) / `show ipv6 ospf6 interface`+`neighbor` (IPv6) -> engine state (transit area, neighbour, state, cost, next hop) | `test/ospf/ospf-virtual-link-show.ci` + `test/ospfv3/ospfv3-vlink.ci` |
| 7 | both | Removes the virtual-link config | reconcile tears down the synthetic interface and adjacency; the backbone Router-LSA loses the virtual record and the V-bit; OSPF otherwise unchanged | `TestVirtualLinkRemovalWithdrawsRecord` + existing OSPF suites still green |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Address family | Validates | Status |
|------|------|----------------|-----------|--------|
| `TestParseVirtualLinkConfig` | `internal/plugins/ospf/config_test.go` | both | AC-1: `virtual-link` list parsed into `VirtualLinks` (IPv4 + `V6`) with timer defaults | |
| `TestVirtualLinkRejectStubTransit` | `internal/plugins/ospf/config_test.go` | both | AC-2, A-10: a stub (IPv6 also NSSA/backbone) transit area is rejected | |
| `TestVirtualLinkRejectNonABR` | `internal/plugins/ospf/config_test.go` | both | AC-3, A-10: a non-ABR / absent transit area is rejected | |
| `TestVirtualLinkRejectsSelfRouterID` | `internal/plugins/ospf/config_test.go` | both | AC-2, A-10: a self neighbour Router ID is rejected | |
| `TestRouterLSAVirtualLinkRoundTrip` | `internal/plugins/ospf/packet/lsa_router_test.go` | IPv4 | A-1: V-bit + Type-4 record round-trip byte-for-byte | |
| `TestV6RouterLSAVirtualRecordRoundTrip` | `internal/plugins/ospf/v3/packet/lsa_router_test.go` | IPv6 | A-1: virtual record + V-bit decode/re-encode byte-for-byte | |
| `TestRouterLinksEmitsVirtualType4` | `internal/plugins/ospf/lsdb/origination_test.go` | IPv4 | AC-8: `routerLinks` emits a Type-4 record for a Full virtual link with the correct LinkID/LinkData/Metric | |
| `TestBackboneRouterLSAHasVBitWhenVLFull` | `internal/plugins/ospf/lsdb/origination_test.go` | IPv4 | AC-8, A-4: the backbone Router-LSA sets the V-bit; non-backbone Router-LSAs do not | |
| `TestVirtualLinkBackboneOnly` | `internal/plugins/ospf/lsdb/origination_test.go` | IPv4 | R-5: no Type-4 record or V-bit in any non-backbone Router-LSA | |
| `TestVirtualRecordInBackboneRouterLSA` | `internal/plugins/ospf/origination_v6_test.go` | IPv6 | AC-9, R-5: V-bit + virtual record land in the BACKBONE Router-LSA only | |
| `TestVirtualLinkCostEqualsTransitCost` | `internal/plugins/ospf/origination_v6_test.go` | IPv6 | AC-14, A-13: advertised metric = transit-area path cost | |
| `TestVirtualLinkWithdrawnWhenDown` | `internal/plugins/ospf/origination_v6_test.go` | IPv6 | AC-5: a Down virtual link withdraws the record + V-bit | |
| `TestVirtualNeighborResolvedFromTransitSPF` | `internal/plugins/ospf/spf/transitarea_test.go` | both (shared) | AC-4, A-3: reachability/cost/next hop read from the transit-area intra-area `Result` | |
| `TestVirtualLinkNeighborUnreachableStaysDown` | `internal/plugins/ospf/spf/transitarea_test.go` | both (shared) | AC-5, R-6: an unreachable virtual neighbour yields a Down link, no virtual record | |
| `TestBackboneSPFReachesVirtualNeighbor` | `internal/plugins/ospf/spf/spf_test.go` | IPv4 | AC-10, A-2: the backbone graph reaches the virtual neighbour via the Type-4 link (two-way check honoured) | |
| `TestV6BackboneGraphIncludesVirtualLink` | `internal/plugins/ospf/afstrategy_v6_test.go` | IPv6 | AC-10, A-2, R-11: the backbone graph + two-way check include the virtual link | |
| `TestTransitCapabilitySetByVBit` | `internal/plugins/ospf/spf/transitarea_test.go` | both (shared) | AC-12: TransitCapability TRUE iff a Router-LSA in the area has the V-bit | |
| `TestTransitAreaPassOnlyImprovesReachable` | `internal/plugins/ospf/spf/transitarea_test.go` | both (shared) | AC-12, R-3, A-9: the §16.3 pass improves only already-reachable routes | |
| `TestVirtualNextHopResolvedOrDiscarded` | `internal/plugins/ospf/spf/transitarea_test.go` | both (shared) | AC-13, R-4: a virtual next hop is rewritten to the real transit next hop or discarded | |
| `TestVirtualNextHopIsTransitNextHop` | `internal/plugins/ospf/afstrategy_v6_test.go` | both (shared) | AC-13, A-13: SPF next hop is the transit next hop, not the neighbour's packet destination | |
| `TestVirtualLinkBackboneAttachment` | `internal/plugins/ospf/afstrategy_v6_test.go` | both (shared; §3.5) | AC-11, A-12, R-8: a Full virtual link makes the endpoint backbone-attached (gated on Full) | |
| `TestVirtualLinkCostTracksTransitTopology` | `internal/plugins/ospf/spf/transitarea_test.go` | both (shared) | AC-14, R-6: a transit-cost change updates the virtual-link cost | |
| `TestVirtualLinkCostUpdateNoFlap` | `internal/plugins/ospf/spf/transitarea_test.go` | both (shared) | R-7: an unchanged cost does not reoriginate the backbone Router-LSA | |
| `TestVirtualEndpointResolvesGlobalAddress` | `internal/plugins/ospf/virtuallink_v6_test.go` | IPv6 | AC-7, A-5: resolve the neighbour's global dest from the transit SPF + Intra-Area-Prefix-LSA | |
| `TestVirtualEndpointReresolvesOnSPF` | `internal/plugins/ospf/virtuallink_v6_test.go` | IPv6 | AC-5, R-6: re-resolve on each transit SPF; Down when unreachable | |
| `TestVirtualLinkAdjacencyReachesFull` | `internal/plugins/ospf/virtual_link_test.go` | IPv4 | AC-6, A-6: the synthetic p2p interface reaches Full; DD MTU 0, Hello Mask 0.0.0.0 | |
| `TestVirtualAdjacencyReachesFull` | `internal/plugins/ospf/instance_v6_test.go` | IPv6 | AC-7, A-6: the virtual NSM reaches Full over a fake routed transport | |
| `TestVirtualLinkSendUsesRoutedTTL` | `internal/plugins/ospf/transport/transport_test.go` | IPv4 | AC-6, A-7, R-1: the virtual-link send path uses a routed TTL (> 1), not the TTL-1 socket | |
| `TestRoutedSendUsesGlobalSourceAndHopLimit` | `internal/plugins/ospf/v3/transport/transport_test.go` | IPv6 | AC-7, A-8, R-1, R-2: the routed send uses a global source + hop limit > 1, not link-local | |
| `TestVirtualLinkPacketDemux` | `internal/plugins/ospf/virtual_link_test.go` | both | R-9: a routed packet from the virtual neighbour demuxes to the synthetic interface by source Router ID + backbone Area ID | |
| `TestTwoVirtualLinksSameTransit` | `internal/plugins/ospf/virtual_link_test.go` | both | R-10: two virtual links sharing a transit egress are keyed by (transit area, neighbour) and do not collide | |
| `TestVirtualInterfaceNameReserved` | `internal/plugins/ospf/instance_v6_test.go` | both | R-10: the synthetic virtual-interface name/ID does not clash with a real interface | |
| `TestVirtualLinkUsesTransitAreaAuth` | `internal/plugins/ospf/virtual_link_test.go` | both | AC-18, A-11: the virtual link signs/verifies with the transit area's key-chain / trailer | |
| `TestVirtualLinkRemovalWithdrawsRecord` | `internal/plugins/ospf/virtual_link_test.go` | both | story 7: removing config tears down the interface and withdraws the virtual record + V-bit | |
| `TestNoVirtualLinkBehaviorUnchanged` | `internal/plugins/ospf/virtual_link_test.go` | both | AC-17: with no virtual link configured, V-bit clear, no record, no §16.3 pass | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Address family | Range | Last Valid | Invalid Below | Invalid Above |
|-------|----------------|-------|------------|---------------|---------------|
| hello-interval (seconds) | both | 1-65535 | 65535 | 0 | N/A (uint16) |
| dead-interval (seconds) | both | 1-65535 | 65535 | 0 | N/A (uint16) |
| retransmit-interval (seconds) | both | 1-65535 | 65535 | 0 | N/A (uint16) |
| transmit-delay (InfTransDelay, seconds) | both | 1-3600 | 3600 | 0 (must be > 0) | 3601 |
| virtual-link cost (computed from transit SPF) | both | 1-65534 | 65534 | N/A (>= 1 by SPF) | LSInfinity (0xffff) means unreachable -> link Down |
| transit area-id | both | 0.0.0.1-255.255.255.255 | any non-zero | 0.0.0.0 (backbone cannot be a transit area) | N/A |
| remote-router-id | both | dotted-quad / uint32 | 255.255.255.255 | N/A | self Router ID rejected (AC-2) |
| IPv4 send TTL | IPv4 | 2-255 | 255 | 1 (would not route) | N/A (uint8) |
| IPv6 hop limit on virtual send | IPv6 | 2-255 | 255 | 1 (would not route) | N/A (uint8) |

### Functional Tests
| Test | Location | Address family | End-User Scenario | Status |
|------|----------|----------------|-------------------|--------|
| `ospf-virtual-link-config` | `test/ospf/ospf-virtual-link-config.ci` | IPv4 | a `virtual-link` entry parses and shows in `show ospf`; stub-transit/non-ABR configs are rejected | |
| `ospf-virtual-link-up` | `test/ospf/ospf-virtual-link-up.ci` | IPv4 | the virtual neighbour is resolved over the transit area and the link comes up to Full | |
| `ospf-virtual-link-route` | `test/ospf/ospf-virtual-link-route.ci` | IPv4 | a destination reachable only across the repaired backbone is installed with the real transit next hop | |
| `ospf-virtual-link-show` | `test/ospf/ospf-virtual-link-show.ci` | IPv4 | `show ospf virtual-links` lists state/cost/next hop; `show ospf database router` shows the V-bit + Type-4 link | |
| `ospfv3-vlink-config` | `test/ospfv3/ospfv3-vlink-config.ci` | IPv6 | virtual-link config parses; a stub/NSSA/backbone transit area or self RID is rejected | |
| `ospfv3-vlink` | `test/ospfv3/ospfv3-vlink.ci` | IPv6 | two Ze routers form a virtual-link adjacency to Full across a transit area; `show ipv6 ospf6 neighbor` shows the virtual neighbour | |
| `ospfv3-vlink-backbone-repair` | `test/ospfv3/ospfv3-vlink-backbone-repair.ci` | IPv6 | an ABR with no physical backbone interface becomes backbone-attached; inter-area routes appear | |
| `ospfv3-vlink-reresolve` | `test/ospfv3/ospfv3-vlink-reresolve.ci` | IPv6 | a transit topology change updates the metric/next hop; an unreachable neighbour drives the link Down and withdraws the record | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Address family | Peer Daemon | What It Proves | Status |
|----------|-----------|----------------|-------------|----------------|--------|
| `ospf-virtual-link-frr` | `test/interop/scenarios/ospf-virtual-link-frr/` | IPv4 | FRR `ospfd` (3-area topology: Area 0 fragment, transit Area, remote fragment; FRR configures `area <transit> virtual-link <ze-rid>`) | Ze forms a Full virtual-link adjacency over the transit area (routed IP, TTL > 1), originates the V-bit + Type-4 record into its backbone Router-LSA, repairs the backbone, and FRR's `show ip ospf virtual-links` shows the link up | |
| `ospfv3-vlink-frr` | `test/interop/scenarios/ospfv3-vlink-frr/` | IPv6 | FRR `ospf6d` (virtual link at the far end, two-hop transit area) | Ze sends routed-unicast OSPFv3 (global src/dst, hop limit > 1), forms a Full virtual adjacency across the transit area, advertises the backbone Router-LSA with the V-bit + virtual record, and inter-area reachability is restored through the virtual link | |

> Interop is required in both families: virtual links add wire behaviour (IPv4
> Hello Mask 0 / DD MTU 0 / routed TTL > 1 / V-bit + Type-4; IPv6 routed unicast
> with a global source / hop limit > 1 / V-bit + virtual record). The raw-IP /
> routed transit paths are Linux-only and run as QEMU integration tests
> (`ai/rules/qemu-testing.md`), consistent with the rest of the OSPF interop set;
> the IPv6 transit area requires a multi-hop QEMU topology so the routed send is
> genuinely exercised (not a single-hop shortcut).

### Future (if deferring any tests)
- None. Every AC is covered by a unit, functional, or interop test above. NBMA / P2MP and multi-AF virtual links are separate specs (out of scope), not deferred tests of this spec.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*) -->

Shared / AF-neutral:
- `internal/plugins/ospf/config.go` -- `virtualLinkConfig` type; `VirtualLinks` surfaced on `ospfConfig` (IPv4) and the `V6` family (IPv6); `parseVirtualLink`; `validateConfig` stub/NSSA/backbone-transit, non-ABR, and self-RID rejection
- `internal/plugins/ospf/spf/interarea.go` -- update the `IsABR` "out of scope" comment; invoke the §16.3 transit-area pass; thread the §3.5 backbone-attachment condition
- `internal/plugins/ospf/spf/computer.go` -- run the transit-area pass after inter-area; expose virtual-neighbour resolution from the transit-area `Result`
- `internal/plugins/ospf/instance.go` -- the synthetic virtual-interface lifecycle and virtual-link manager driven by SPF results; backbone origination input; virtual-link packet demux (shared lifecycle; AF transport adapter)
- `internal/plugins/ospf/cmd_show.go` -- IPv4 `show ospf virtual-links` and V-bit/Type-4 in `show ospf database router`; IPv6 virtual interface/neighbour rows
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- the `virtual-link` list with native constraints (shared schema, used by both families)
- `internal/plugins/ospf/yang/ze-ospf-cmd.yang` -- the `show ospf virtual-links` command and the IPv6 show virtual rows
- `internal/plugins/ospf/register.go` -- register the new `ospf-virtual-link` config validator
- `internal/plugins/ospf/doctor.go` -- a doctor check only if a new runtime dependency is introduced (a routed send socket); add one if so

IPv4 family:
- `internal/plugins/ospf/lsdb/origination.go` -- carry virtual-link records in `OriginInput`; emit a Type-4 record in `routerLinks`; set `VirtualLinkEndpoint` (V-bit) when Full; backbone Router-LSA only
- `internal/plugins/ospf/transport/transport.go` + `internal/plugins/ospf/transport/backend_linux.go` -- a routed (TTL > 1) unicast send path for virtual-link packets, distinct from the TTL-1 link-local socket

IPv6 family:
- `internal/plugins/ospf/origination_v6.go` -- inject the `RouterLinkTypeVirtual` record + `RouterFlagV` into the backbone Router-LSA; treat a Full virtual link as backbone attachment for `v6IsAreaBorderRouter`; withdraw on Down
- `internal/plugins/ospf/afstrategy_v6.go` -- the `RouterLinkTypeVirtual` case in `v6RouterLinks`; the virtual next-hop resolution (transit next hop, not the global dest)
- `internal/plugins/ospf/v3/transport/transport.go` -- a routed-unicast send path (global source, hop limit > 1, not bound to one ifindex); checksum finalized against the global source
- `internal/plugins/ospf/v3/transport/backend_linux.go` + `internal/plugins/ospf/v3/transport/backend_other.go` -- backend support for the routed send (hop-limit control, explicit global source); `internal/plugins/ospf/transport_iface.go` if the engine-side transport adapter needs the new entry point

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- `virtual-link` list; read `ai/rules/config-surface.md` + `ai/rules/config-naming.md` |
| YANG validation constraints | [ ] yes | native `range`/`pattern` on area-id, neighbour Router ID, and the four timers; reject backbone (0.0.0.0) as transit area |
| YANG custom validators | [ ] yes | `ze:validate "ospf-virtual-link"` for the not-a-stub (IPv6 also NSSA/backbone) / ABR / non-self rules with `ValidateFn` + `CompleteFn`; register in `register.go` |
| CLI commands/flags | [ ] yes | IPv4 `show ospf virtual-links`; IPv6 virtual rows in `show ipv6 ospf6 interface`/`neighbor` in `ze-ospf-cmd.yang` + `cmd_show.go` |
| CLI grammar (action before identifier) | [ ] yes | `ai/rules/cli-grammar.md` -- `show ospf virtual-links`; show subcommands unchanged in shape |
| Editor autocomplete | [ ] yes | automatic for the YANG list + the new show subcommand; `CompleteFn` offers configured transit areas / known area IDs |
| Functional test for new RPC/API | [ ] yes | `test/ospf/ospf-virtual-link-*.ci` + `test/ospfv3/ospfv3-vlink*.ci` |
| Pipe completeness | [ ] yes | all virtual show outputs route through `ApplyPipes` like the other show outputs |
| Env var registration | [ ] no | virtual links are operational config, not an `environment/` leaf |
| Doctor check for runtime dependencies | [ ] check | IPv4: if a routed send socket is opened, add an owning-package doctor check + `internal/core/diagnostic/codes.go` entry per `ai/rules/doctor-checks.md`. IPv6: none expected (the routed send reuses the existing raw IPv6 / proto-89 socket family) |
| Prometheus counters/metrics | [ ] yes | see the metrics rows below |

#### Metrics (new series owned by this spec)
| Metric | Address family | Type | Labels |
|--------|----------------|------|--------|
| `ze_ospf_virtual_links` | IPv4 | gauge | `transit_area`, `state` |
| `ze_ospf_virtual_link_cost` | IPv4 | gauge | `transit_area`, `neighbor` |
| `ze_ospf_virtual_link_adjacency_changes_total` | IPv4 | counter | `transit_area`, `neighbor` |
| `ze_ospf_transit_area_passes_total` | both (shared §16.3) | counter | `transit_area` |
| `ze_ospfv3_virtual_links` | IPv6 | gauge | `transit_area`, `state` |
| `ze_ospfv3_virtual_link_cost` | IPv6 | gauge | `transit_area`, `remote_router_id` |
| `ze_ospfv3_virtual_link_reresolves_total` | IPv6 | counter | `transit_area` |

> These extend the umbrella's canonical OSPF metric set with the
> `ze_ospf_virtual_*` / `ze_ospf_transit_*` (IPv4 + shared) and
> `ze_ospfv3_virtual_link*` (IPv6) prefixes, registered by this spec's owner
> code. The umbrella "Metrics" table gains these rows when this spec lands.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` -- OSPF virtual links (both address families) |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` -- the `virtual-link` list |
| 3 | CLI command added/changed? | [ ] yes | `docs/guide/command-reference.md` -- `show ospf virtual-links` + the IPv6 show virtual rows |
| 4 | API/RPC added/changed? | [ ] no | the show RPCs live in the central `ze-show` namespace; documented under the command reference |
| 5 | Plugin added/changed? | [ ] yes | `docs/guide/plugins.md` -- OSPF gains virtual-link config + the synthetic interface + routed transport |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/ospf.md` -- virtual-link section covering both address families |
| 7 | Wire format changed? | [ ] yes | `docs/architecture/wire/ospf.md` and `docs/architecture/wire/ospfv3.md` -- V-bit, virtual link record, virtual-link Hello/DD field rules, routed unicast |
| 8 | Plugin SDK/protocol changed? | [ ] no | no SDK surface change |
| 9 | RFC behavior implemented? | [ ] yes | `rfc/short/rfc2328.md` (§15 / §16.3 / virtual-link items) and `rfc/short/rfc5340.md` (§4.2 / §3.5 / §2.9) -- mark implemented |
| 10 | Test infrastructure changed? | [ ] yes (two interop scenarios, multi-hop QEMU) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` -- OSPF virtual-link parity with FRR in both families |
| 12 | Internal architecture changed? | [ ] yes | the OSPF subsystem doc -- the synthetic interface, the §16.3 transit pass, the two routed send paths |
| 13 | Route metadata keys added/changed? | [ ] no | virtual links resolve to existing route entries (no new metadata key) |
| 14 | Prometheus counters added/changed? | [ ] yes | the OSPF telemetry doc -- the new series |
| 15 | Registered plugin/event/command/capability inventory changed? | [ ] yes | `docs/plugin-overview.md` + the umbrella metrics table (new show command, new validator) |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors into `config.go`, `lsdb/origination.go`, `origination_v6.go`, `afstrategy_v6.go`, `spf/interarea.go`, `transport/transport.go`, `v3/transport/transport.go` |
| 17 | Existing docs show examples for this area? | [ ] check | verify OSPF config/CLI examples still parse with the new `virtual-link` list present, in both families |

## Files to Create
- `internal/plugins/ospf/virtual_link.go` -- the synthetic virtual-interface lifecycle, the shared virtual-link manager, packet demux, and the engine glue (AF-neutral lifecycle)
- `internal/plugins/ospf/virtual_link_test.go` -- adjacency (IPv4), demux, two-links, auth, removal, no-config-unchanged tests
- `internal/plugins/ospf/virtuallink_v6.go` -- the IPv6 endpoint resolver: the global dest + transit next hop from the transit SPF result, the routed-send adapter binding, state tracking
- `internal/plugins/ospf/virtuallink_v6_test.go` -- the IPv6 resolver, re-resolution, and global-address tests
- `internal/plugins/ospf/spf/transitarea.go` -- the §16.3 transit-area Summary-LSA pass, TransitCapability, the §3.5 backbone-attachment condition, and the virtual-neighbour resolution from the transit-area `Result` (shared)
- `internal/plugins/ospf/spf/transitarea_test.go` -- resolution, TransitCapability, improve-only, next-hop-resolve-or-discard, cost-tracking/no-flap tests (shared)
- `test/ospf/ospf-virtual-link-config.ci`, `ospf-virtual-link-up.ci`, `ospf-virtual-link-route.ci`, `ospf-virtual-link-show.ci`
- `test/ospfv3/ospfv3-vlink-config.ci`, `ospfv3-vlink.ci`, `ospfv3-vlink-backbone-repair.ci`, `ospfv3-vlink-reresolve.ci`
- `test/interop/scenarios/ospf-virtual-link-frr/` -- `ze.conf`, `frr.conf`, `check.py` (IPv4)
- `test/interop/scenarios/ospfv3-vlink-frr/` -- `ze.conf`, `frr.conf`, `check.py`, multi-hop transit-area QEMU topology (IPv6)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan -- confirm both codecs' V-bit/virtual record and the shared-SPF virtual graph match already exist |
| 3. Wiring phase | Wiring Test table -- config + synthetic-interface skeleton + failing wiring tests |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist |
| 8. Fix issues | from critical review |
| 9. Re-verify | re-run stage 6 |
| 10. Repeat 7-9 | until clean |
| 11. Deliverables review | Deliverables Checklist |
| 12. Security review | Security Review Checklist |
| 13. Re-verify | re-run stage 6 |
| 14. Present summary | Executive Summary per `ai/rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- the shared config surface + synthetic-interface skeleton + failing wiring tests
   - Tests: `TestParseVirtualLinkConfig`, `TestVirtualLinkRejectStubTransit`, `TestVirtualLinkRejectNonABR`, `TestVirtualLinkRejectsSelfRouterID`, `test/ospf/ospf-virtual-link-config.ci`, `test/ospfv3/ospfv3-vlink-config.ci`
   - Files: `config.go` (`virtualLinkConfig`, `VirtualLinks` on both families, parse + validate), `yang/ze-ospf-conf.yang` (`virtual-link` list), `register.go` (validator), `virtual_link.go` (manager skeleton; reconcile creates a stub synthetic interface)
   - Verify: the config resolves and is validated in both families; the synthetic interface is created on reachability but adjacency/origination are stubs, so deeper tests still fail
2. **Phase: Transit-area resolution + §16.3 pass + §3.5 (shared SPF)** -- the AF-neutral SPF side
   - Tests: `TestVirtualNeighborResolvedFromTransitSPF`, `TestVirtualLinkNeighborUnreachableStaysDown`, `TestTransitCapabilitySetByVBit`, `TestTransitAreaPassOnlyImprovesReachable`, `TestVirtualNextHopResolvedOrDiscarded`, `TestVirtualLinkBackboneAttachment`, `TestVirtualLinkCostTracksTransitTopology`, `TestVirtualLinkCostUpdateNoFlap`
   - Files: `spf/transitarea.go`, `spf/computer.go`, `spf/interarea.go`
   - Verify: reachability/cost/next hop read from the transit-area result; the §16.3 pass improves only reachable routes and resolves/discards virtual next hops; a Full virtual link makes the endpoint backbone-attached; cost tracks topology without flapping
3. **Phase: Router-LSA virtual record origination (per AF)** -- the backbone advertisement
   - Tests: `TestRouterLSAVirtualLinkRoundTrip`, `TestRouterLinksEmitsVirtualType4`, `TestBackboneRouterLSAHasVBitWhenVLFull`, `TestVirtualLinkBackboneOnly`, `TestBackboneSPFReachesVirtualNeighbor` (IPv4); `TestV6RouterLSAVirtualRecordRoundTrip`, `TestVirtualRecordInBackboneRouterLSA`, `TestVirtualLinkCostEqualsTransitCost`, `TestVirtualLinkWithdrawnWhenDown`, `TestV6BackboneGraphIncludesVirtualLink`, `TestVirtualNextHopIsTransitNextHop` (IPv6)
   - Files: `lsdb/origination.go`, `instance.go` (IPv4 backbone `OriginInput`); `origination_v6.go`, `afstrategy_v6.go` (IPv6 record + `v6RouterLinks` case)
   - Verify: a Full virtual link emits the virtual record + V-bit into the backbone Router-LSA only in each family; the backbone SPF reaches the virtual neighbour; the metric tracks the transit cost
4. **Phase: Routed transport (per AF)** -- the two routed send paths
   - Tests: `TestVirtualLinkSendUsesRoutedTTL` (IPv4); `TestRoutedSendUsesGlobalSourceAndHopLimit` (IPv6)
   - Files: `transport/transport.go`, `transport/backend_linux.go` (IPv4 TTL > 1); `v3/transport/transport.go`, `v3/transport/backend_linux.go`, `v3/transport/backend_other.go`, `transport_iface.go` (IPv6 global source + hop limit > 1)
   - Verify: IPv4 send uses TTL > 1 not the TTL-1 socket; IPv6 send uses a global source + hop limit > 1 not `LinkLocalSource()`; the IPv6 checksum is finalized against the global source
5. **Phase: Synthetic interface + adjacency** -- the shared neighbour/interface side
   - Tests: `TestVirtualLinkAdjacencyReachesFull` (IPv4), `TestVirtualAdjacencyReachesFull` (IPv6), `TestVirtualEndpointResolvesGlobalAddress`, `TestVirtualEndpointReresolvesOnSPF` (IPv6), `TestVirtualLinkPacketDemux`, `TestTwoVirtualLinksSameTransit`, `TestVirtualInterfaceNameReserved`, `TestVirtualLinkUsesTransitAreaAuth`, `TestVirtualLinkRemovalWithdrawsRecord`, `TestNoVirtualLinkBehaviorUnchanged`
   - Files: `virtual_link.go`, `virtuallink_v6.go`, `instance.go`
   - Verify: the synthetic p2p interface reaches Full over routed IP in both families; packets demux correctly; two links coexist; the name/ID is reserved; auth inherits from the transit area; removal withdraws the record; no-config is unchanged
6. **Phase: CLI + metrics + doctor** -- user surface
   - Tests: `ospf-virtual-link-show.ci`, `ospf-virtual-link-up.ci`, `ospf-virtual-link-route.ci`, `ospfv3-vlink.ci`, `ospfv3-vlink-backbone-repair.ci`, `ospfv3-vlink-reresolve.ci`
   - Files: `cmd_show.go`, `yang/ze-ospf-cmd.yang`, metric registration, `doctor.go` (if a routed socket is introduced)
   - Verify: `show ospf virtual-links`, the IPv6 show virtual rows, the V-bit/Type-4 in `show ospf database router`, the metric series, the doctor check
7. **Functional tests** -> the eight `.ci` cover the user-visible behaviour in both families
8. **RFC refs** -> add `// RFC 2328 Section 15 / 16.3 / A.4.2` (IPv4) and `// RFC 5340 Section 4.2 / 2.9 / 3.5 / A.4.3` (IPv6) comments on the origination, transit pass, and routed-send code
9. **Interop** -> `ospf-virtual-link-frr` (IPv4) and `ospfv3-vlink-frr` (IPv6, multi-hop) QEMU scenarios
10. **Full verification** -> `make ze-verify`
11. **Complete spec** -> audit tables + learned summary; two commits (A: code+spec+learned, B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has file:line implementation in the right family |
| Feature completeness | each user story has a working path; virtual-link parity with FRR in both families (backbone repair + remote-area attach + the show commands) |
| Correctness | cost = transit intra-area cost; next hop from transit SPF (not the neighbour packet address); V-bit + virtual record backbone-only; §16.3 improves-only and resolves/discards virtual next hops; IPv4 Hello Mask 0 / DD MTU 0 / TTL > 1; IPv6 global source / hop limit > 1 / checksum against the global source |
| Naming | `ze_ospf_virtual_*` / `ze_ospf_transit_*` / `ze_ospfv3_virtual_link*` metrics; YANG `virtual-link` / `remote-router-id` kebab-case; show commands |
| Data flow | virtual links read the transit-area SPF result read-only; backbone Router-LSA is the only origination point; SPF stays AF-neutral; no virtual-link spelling in generic packages |
| CLI grammar | `show ospf virtual-links` action-before-identifier; IPv6 show rows unchanged in shape |
| Doctor checks | a routed-send-socket dependency (IPv4), if added, has a `ze doctor` check per `ai/rules/doctor-checks.md`; IPv6 confirms none added |
| YANG validation | the `virtual-link` list has native range/pattern + a custom validator for stub/NSSA/backbone-transit / ABR / non-self rules |
| Prometheus counters | the series defined, registered, listed; umbrella table updated |
| Rule: plugin-self-containment | all virtual-link surfaces live under `internal/plugins/ospf/`; removing the config removes the feature in both families |
| Rule: buffer-first | both virtual records emitted via the Router-LSA `WriteTo`; show output via `textbuf` |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `virtual-link` config parses and validates (both families) | `go test ./internal/plugins/ospf -run TestParseVirtualLinkConfig` and the reject tests |
| Type-4 record + V-bit in the IPv4 backbone Router-LSA | `go test ./internal/plugins/ospf/lsdb -run 'Virtual'` |
| `RouterLinkTypeVirtual` + V-bit in the IPv6 backbone Router-LSA | `go test ./internal/plugins/ospf -run 'TestVirtualRecordInBackboneRouterLSA|TestVirtualLinkCost'` |
| §16.3 transit-area pass + resolution + §3.5 | `go test ./internal/plugins/ospf/spf -run 'Transit|Virtual'` |
| Routed IPv4 (TTL > 1) send path | `go test ./internal/plugins/ospf/transport -run TestVirtualLinkSendUsesRoutedTTL` |
| Routed IPv6 (global source, hop limit > 1) send path | `go test ./internal/plugins/ospf/v3/transport -run TestRoutedSendUsesGlobalSourceAndHopLimit` |
| Synthetic interface reaches Full (both families) | `go test ./internal/plugins/ospf -run 'TestVirtualLinkAdjacencyReachesFull|TestVirtualAdjacencyReachesFull'` |
| Metric series registered | `grep -rn 'ze_ospf_virtual_\|ze_ospf_transit_\|ze_ospfv3_virtual_link' internal/plugins/ospf` |
| Show commands present | `grep -rn 'virtual-links\|ospf-virtual-link\|ospfv3-vlink' internal/plugins/ospf` |
| Functional tests present | `ls test/ospf/ospf-virtual-link-*.ci test/ospfv3/ospfv3-vlink*.ci` |
| Interop scenarios present | `ls test/interop/scenarios/ospf-virtual-link-frr/ test/interop/scenarios/ospfv3-vlink-frr/` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | virtual-link config validated (transit area exists, not stub/NSSA/backbone, router is ABR, neighbour not self); the resolved IPv6 global destination is a routable global address (not unspecified/multicast/link-local); a malformed virtual-link packet / transit LSDB is bound-checked by the existing codec and never panics the resolver |
| Resource exhaustion | the number of synthetic interfaces is bounded by configured virtual links; a flapping transit topology cannot spawn unbounded interfaces or SPF runs (reuse the SPF throttle); the resolver runs O(transit routers) per transit SPF, not per packet |
| Trust boundary | virtual-link packets are routed and can arrive from any host that can reach the transit/global address; they MUST pass the transit area's authentication (inherited key-chain / RFC 7166 trailer) and match a configured virtual link before any state change |
| Spoofing | a routed unicast from an arbitrary source must still pass the OSPF header checks (version, area, Instance ID for IPv6, checksum) and match a configured virtual link |
| Routed exposure | the routed send sockets only send to configured virtual-neighbour addresses; they do not widen the receive surface beyond the existing per-interface receive path |
| Error leakage | virtual-link resolution / adjacency failures are logged and counted, not surfaced to peers |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to the relevant phase and implement |
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
<!-- LIVE -->

## Core Insight
A virtual link is not a new protocol mechanism in either address family: it is a
point-to-point interface in Area 0 whose cost and next hop are *computed from the
transit area's intra-area SPF* instead of configured, and whose packets are
*routed* (IPv4 TTL > 1; IPv6 global source + hop limit > 1) instead of link-local.
Because Ze runs OSPF as one unified engine, the ISM/NSM, DD/flooding, LSDB
sequencing, and the intra-area SPF graph walk (already virtual-aware) are SHARED;
both codecs already encode the V-bit and the virtual link record. The only
genuinely AF-specific, genuinely-absent pieces are the two routed transports and
the per-AF record emission plus, for IPv6, the global-address endpoint resolver.
The real work is activation: configuration, the shared transit-area-driven
resolution, a synthetic backbone interface with two routed send paths, originating
the record into the backbone Router-LSA per family, and the shared §16.3 transit
pass (plus §3.5 backbone attachment) that resolves destinations whose next hop is
a virtual link.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One feature spec for both address families | two version-split specs | matches Ze's one-engine OSPF design; the shared engine behaviour is stated once and per-AF wire/transport differences are labelled, avoiding divergence between the IPv4 and IPv6 drafts |
| Model a virtual link as a synthetic p2p interface in Area 0 | a bespoke virtual-link state machine separate from interfaces | reuses the shared neighbour SM, DD, flooding, and the p2p path; only the cost/next-hop source and the routed transport differ (RFC 2328 §15 / RFC 5340 §4.2) |
| Read cost/next hop from the transit-area intra-area SPF `Result` | a separate shortest-path computation just for virtual links | the virtual neighbour is already a router vertex in the transit area; a read avoids a second Dijkstra (§16.1) |
| Two dedicated routed send paths, not tweaks to the existing send | bump the IPv4 TX socket TTL / overload IPv6 `SendPacket` with a flag | the IPv4 socket pins TTL 1 and the IPv6 path is correctly link-local for every normal interface; virtual links are the documented exception (§2.9), keeping them separate avoids regressing the common paths |
| The cost is derived from the transit-area SPF, never configured | a `cost` config leaf | RFC 2328 §15 / RFC 5340 §C.2: the virtual-link cost IS the transit-area path cost; a config leaf would diverge from the standard |
| Emit the virtual record only into the backbone Router-LSA | emit per area | RFC 2328 §A.4.2 / RFC 5340 App A.4.3: the V-bit/record belongs to the backbone (transit endpoint is Area 0) |
| Inherit auth from the transit area | a separate virtual-area key surface | guide §7 (~491): virtual links use the transit area's authentication (IPv6 reuses its RFC 7166 trailer) |
| Implement §16.3 as a new shared transit pass gated by TransitCapability | fold it into the inter-area pass | §16.3 is a distinct, improve-only stage with its own discard-unresolved-virtual-next-hops semantics, identical across families |

## Known Limitations
- IPv4 per RFC 2328 §15; IPv6 per RFC 5340 §4.2 (IPv6-unicast only); multi-AF (RFC 5838) virtual links are out of scope (spec-ospfv3-ext-1).
- NBMA networks / the NBMA neighbour list and the point-to-multipoint network type are spec-ospf-ext-8 / spec-ospfv3-ext-7, deliberately excluded here.
- Sham links (MPLS VPN, RFC 4577) are not planned.
- Virtual links cannot run through a stub area (IPv6 also NSSA / backbone) or be configured on a non-ABR; these are config-time rejections, not runtime fallbacks.
- The §16.3 transit pass only improves already-reachable backbone routes; it never makes a new destination reachable (RFC 2328 §16.3).
- Authentication on the virtual interface inherits the transit area's config; no virtual-link-specific auth surface is added.

## RFC Documentation

Add `// RFC <NNNN> Section X.Y: "<quoted requirement>"` above the enforcing code.

IPv4 (RFC 2328):
- §15 virtual link belongs to the backbone, configured by (neighbour Router ID, transit area), cost/address set dynamically, transit area not a stub
- §A.3.2 Hello Network Mask 0.0.0.0 on a virtual link; §A.3.3 DD Interface MTU 0 on a virtual link
- §A.4.2 Router-LSA V-bit and Type-4 link record (LinkID = neighbour Router ID, LinkData = local interface address, Metric = transit cost)
- §16.1 virtual-neighbour reachability/cost/next hop from the transit-area shortest-path tree
- §16.3 transit-area Summary-LSA pass: improve-only, resolve/discard virtual next hops, TransitCapability

IPv6 (RFC 5340):
- §2.9 routed-unicast virtual-link transport (global source/dest, hop limit > 1)
- §4.2 virtual link through a transit (non-backbone/non-stub/non-NSSA) area; up only while the neighbour is reachable intra-area
- §3.5 a fully adjacent virtual-link endpoint is considered backbone-attached
- App A.4.3 the Router-LSA V-bit + the `RouterLinkTypeVirtual` record; metric = transit-area path cost; App A.4.8 no Link-LSA on a virtual link
- §C.2 the configurable parameters (transit area, neighbour Router ID, timers); cost is NOT configured

## Implementation Summary

### What Was Implemented
- [filled at implementation time]

### Bugs Found/Fixed
- [filled at implementation time]

### Documentation Updates
- [filled at implementation time]

### Deviations from Plan
- [filled at implementation time]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|-----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Address family | Evidence Type | Concrete Evidence |
|--------------------------|----------------|---------------|-------------------|
| Virtual-link config (transit area + virtual neighbour) | both | functional | `ospf-virtual-link-config.ci`, `ospfv3-vlink-config.ci` |
| Virtual-neighbour adjacency over the transit area | IPv4 | interop | `ospf-virtual-link-frr` (Full over routed IP) |
| Virtual-neighbour adjacency over the transit area | IPv6 | functional + interop | `ospfv3-vlink.ci`, `ospfv3-vlink-frr` |
| Router-LSA virtual-link advertisement (V-bit + record) | IPv4 | unit + interop | `TestBackboneRouterLSAHasVBitWhenVLFull`, `ospf-virtual-link-frr` |
| Router-LSA virtual-link advertisement (V-bit + record) | IPv6 | unit + interop | `TestVirtualRecordInBackboneRouterLSA`, `ospfv3-vlink-frr` |
| SPF integration (backbone repair + §16.3 resolution + §3.5) | both | unit + functional | `TestBackboneSPFReachesVirtualNeighbor`, `TestVirtualLinkBackboneAttachment`, `TestVirtualNextHopResolvedOrDiscarded`, `ospf-virtual-link-route.ci`, `ospfv3-vlink-backbone-repair.ci` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE |  | file:line |  |

### Fixes applied
-

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

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

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-18 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/plugins/ospf/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass)
- [ ] RFC 2328 and RFC 5340 constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (the synthetic interface reuses the existing interface/neighbour machinery; the routed sends + resolver serve only virtual links now, justified by §2.9 / §4.2 / §15)
- [ ] No speculative features (only RFC 2328 §15/§16.3 and RFC 5340 §4.2 virtual links; NBMA/P2MP, multi-AF, sham links, configured cost excluded)
- [ ] Single responsibility per component (resolver vs transport vs origination vs SPF)
- [ ] Explicit > implicit behavior (virtual link Down when unreachable, not silently kept)
- [ ] Minimal coupling (virtual links read the transit-area SPF result read-only; SPF stays AF-neutral)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (`ospf-virtual-link-frr`, `ospfv3-vlink-frr`)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ospf-ext-7-virtual-links.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospf-ext-7-virtual-links.md`
