# Spec: ospfv3-ext-3 -- OSPFv3 Virtual Links (RFC 5340 §4.2)

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-ospfv3-0-umbrella.md (delivered) |
| Phase | - |
| Updated | 2026-06-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `rfc/short/rfc5340.md` -- OSPFv3: §2.9 transport (virtual-link packets use GLOBAL IPv6 unicast, not link-local; routed through the transit area, hop limit > 1), §4.2 virtual links, §3.5 area data structure (TransitCapable, virtual-link membership in the backbone), App A.4.3 Router-LSA (V-bit `RouterFlagV`, virtual-link link record `RouterLinkTypeVirtual`), App A.4.8 Link-LSA NOT applicable on virtual links, §C.2 virtual-link configurable parameters
4. `plan/spec-ospfv3-0-umbrella.md` -- the delivered OSPFv3 base; "Out of Scope" row "Virtual links" (this spec closes it), the package layout, the Instance-ID / Interface-ID interface model, and the FIB-install-via-Loc-RIB path
5. `internal/plugins/ospf/v3/transport/transport.go` -- `SendPacket` binds `LinkLocalSource()` and the per-interface socket; the `InterfaceHandle.Send(dst, src, payload)` seam. THE virtual-link gap: a routed unicast send with a GLOBAL source, NOT bound to one link
6. `internal/plugins/ospf/v3/packet/lsa_router.go` -- `RouterFlagV = 0x04`, `RouterLinkTypeVirtual = 4`, `RouterLink{Type, Metric, InterfaceID, NeighborInterfaceID, NeighborRouterID}` (already defined, not yet emitted)
7. `internal/plugins/ospf/origination_v6.go` -- `v6OriginateSelf`, `v6RouterLSABody`, `v6OriginateRouter`; where the virtual-link Router-LSA record and the backbone (area 0) Router-LSA must be added
8. `internal/plugins/ospf/afstrategy_v6.go` -- `v6RouterLinks` maps Router-LSA links into the shared SPF graph; `v6NextHop` resolves next-hops from the neighbor table (`AddressOf`); virtual links join the backbone graph
9. `internal/plugins/ospf/spf/spf.go` -- the shared Dijkstra already treats `RouterLinkTypeVirtual` like a P2P link (lines ~183/266/332); the virtual-link transit-area path cost feeds the link metric
10. `internal/plugins/ospf/config.go` -- `ospfConfig`, `areaConfig`, `interfaceConfig`, `applyTree`; the `V6 *ospfConfig` IPv6 family sub-config that carries its own areas; where the `virtual-link` config list lands
11. `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- the `areas/area` list; where the per-area `virtual-link` list (remote-router-id key + timers) is added
12. `internal/plugins/ospf/neighbor/table.go` -- `AddressOf` (next-hop), `FloodNeighbors`, the adjacency table virtual neighbors register into; `internal/plugins/ospf/iface/ism.go` -- the ISM (`StatePointToPoint`, no DR on a virtual link)

## Task

Implement **OSPFv3 virtual links** (RFC 5340 §4.2) in the native OSPFv3 engine
(`internal/plugins/ospf/` with the `v3/` codec/transport and the `*_v6.go` engine
files; the OSPFv3 base umbrella `spec-ospfv3-0-umbrella.md` deferred virtual links
and this spec closes that row). A virtual link is a logical point-to-point link
through a **transit area** that repairs or extends the backbone (area `0.0.0.0`):
it lets an Area Border Router that has no physical backbone interface still join
the backbone flooding domain, and it stitches a partitioned backbone back
together. It is configured as a pair (one at each end) under the **transit area**,
naming the **virtual neighbor's Router ID**; the two endpoints must both be ABRs
of the transit area.

The defining OSPFv3 difference from OSPFv2 (RFC 5340 §2.9, §4.2): a virtual-link
adjacency does **NOT** use the interface link-local address and is **NOT** bound
to a single link. OSPFv3 virtual-link packets are sent to the virtual neighbor's
**GLOBAL IPv6 address**, resolved from the transit area's intra-area SPF result,
and routed normally through the transit area (IPv6 hop limit large enough to
traverse it, not 1). The local source is the router's own **global IPv6 address**
reachable in the transit area, not a link-local. Because the packets are routed,
the virtual interface has no fixed kernel ifindex and no multicast group; it sends
unicast through the transit area's next hop.

This spec delivers: the OSPFv3 virtual-link config surface (per transit area, keyed
by the remote Router ID, with the RFC C.2 configurable timers); **global-address
endpoint resolution** (the local + remote global IPv6 addresses, derived from the
transit area's Intra-Area-Prefix-LSAs and intra-area SPF reachability to the
virtual neighbor); a **virtual interface** that runs the ISM as a point-to-point
interface in the **backbone** area (no DR/BDR, no Network-LSA, no Link-LSA) and the
NSM that forms the virtual adjacency to Full over the routed transit path; the
**Router-LSA virtual-link advertisement** (the V-bit `RouterFlagV` in the backbone
Router-LSA plus a `RouterLinkTypeVirtual` link record naming the virtual neighbor,
the cost being the transit-area path cost); and **SPF integration** so the virtual
link participates in the backbone graph and inter-area route computation treats the
endpoint as backbone-attached.

The work reuses the delivered base: the same NSM/DD/LSREQ adjacency machinery
(`internal/plugins/ospf/neighbor/`), the same LSDB origination seam
(`OriginateSelf` via `v6OriginateRouter`), the same shared SPF
(`internal/plugins/ospf/spf/`, which already classifies `RouterLinkTypeVirtual` as
a P2P-like link), and the same v6 codec. It adds the routed-unicast transport send
path, the transit-area endpoint resolver, the virtual-interface ISM driver, and the
backbone Router-LSA virtual-link record.

### In scope (this spec)

| Item | Detail |
|------|--------|
| Config surface | A per-area `virtual-link` list under `areas/area` (the **transit area**) keyed by `remote-router-id`, with RFC C.2 timers (hello-interval, dead-interval, retransmit-interval, transmit-delay) and optional authentication inheriting the transit area; resolved into a new `virtualLinkConfig` slice on `ospfConfig` / its `V6` family |
| Global-address endpoint resolution | Resolve the virtual neighbor's reachable **global IPv6 address** and the local **global IPv6 source** from the transit area's intra-area SPF result + Intra-Area-Prefix-LSAs; recompute on every transit-area SPF run; mark the virtual link Down while the neighbor is unreachable in the transit area (RFC 5340 §4.2, §3.5) |
| Virtual interface + ISM/NSM | A synthetic point-to-point interface in the **backbone** (area 0): ISM `StatePointToPoint` (no DR/BDR, no Hello multicast -- unicast to the resolved global address), the NSM forming the adjacency to Full, using the transit-area Hello/Dead intervals from config (RFC 5340 §C.2 / §C.3) |
| Routed unicast transport | A v3 transport send path that targets the resolved **global** destination with the **global** source and a routed hop limit (> 1), NOT `LinkLocalSource()` and NOT bound to one ifindex (RFC 5340 §2.9) |
| Router-LSA advertisement | The backbone Router-LSA sets `RouterFlagV` when this router is a virtual-link endpoint and carries a `RouterLinkTypeVirtual` link record (Interface ID = the virtual-interface ID, Neighbor Router ID = the virtual neighbor, metric = the transit-area path cost) (RFC 5340 App A.4.3) |
| SPF integration | The virtual link appears in the **backbone** SPF graph as a P2P-like link (the shared `spf/` already handles `RouterLinkTypeVirtual`); inter-area route computation treats a router with a working virtual link as backbone-attached; the virtual next-hop resolves to the transit-area next hop, not the virtual neighbor's address |
| Observability | `show ipv6 ospf6 interface` lists the virtual interface and its state; `show ipv6 ospf6 neighbor` lists the virtual neighbor; the metric series owned below |

### Out of scope (noted so it is not silently assumed done)

| Item | Where |
|------|-------|
| OSPFv2 virtual links | OSPFv2 (`plan/spec-ospf-0-umbrella.md` follow-ups); the `VirtualLinkEndpoint` field already exists in `lsdb/origination.go` but its v2 driver is separate |
| NBMA + point-to-multipoint network types | `spec-ospfv3-ext-7` (the umbrella ext-7 row) |
| Authentication on the virtual link beyond inheriting the transit area's trailer config (RFC 7166) | the delivered base auth path; this spec inherits, it does not add a new auth surface |
| Multi-AF (RFC 5838) virtual links | ext-1 / future; this spec is IPv6-unicast only |
| Stub/NSSA transit areas (virtual links MUST NOT transit a stub/NSSA area, RFC 5340 §4.2) | enforced as a config rejection here, not implemented as a feature |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `docs/research/ospf-implementation-guide.md` "Network Types and Interface Model" (~line 480 table) + §6f "Virtual link" (~line 455) + the transport notes (~lines 91-95, 1634)
  -> Decision: model the virtual link as a sixth network type ("Virtual Link": no DR, explicit static peer Router ID, unicast routed through the transit area, purpose = backbone repair); it reuses the point-to-point ISM/NSM, not a new state machine
  -> Constraint: the guide's transport note is load-bearing -- "Virtual link packets are unicast globally-routable IPv6" and "Virtual links are the exception: they use normal routed IP so they can traverse a transit area" (TTL/hop-limit > 1); the link-local + ff02::5 path of every other interface does NOT apply
  -> Constraint: the guide explicitly lists "virtual links inherit their authentication from the transit area, not from the virtual area" -- the backbone-area virtual interface uses the transit area's auth config
- [ ] `plan/spec-ospfv3-0-umbrella.md` "Out of Scope" (Virtual links row) + "Interface model" + the Loc-RIB FIB path
  -> Constraint: this spec closes the umbrella's deferred "Virtual links" row; the umbrella "Out of Scope" table must be updated when this lands
  -> Decision: virtual links reuse the umbrella's Interface-ID model (the virtual interface gets its own 32-bit Interface ID) and the address-free LSA model; the Router-LSA virtual record carries Interface IDs + Router ID, no IP address (RFC 5340 App A.4.3)
- [ ] `ai/rules/plugin-self-containment.md` -- the virtual-link feature lives entirely inside the OSPF edge plugin
  -> Constraint: config, schema, CLI rows, doctor (none new), and metrics for virtual links register through the OSPF plugin's own surfaces; no virtual-link spelling leaks into generic/central packages
- [ ] `ai/rules/buffer-first.md` + `ai/rules/no-sprintf-alloc.md` -- the Router-LSA virtual record and the routed send are buffer-first
  -> Constraint: the virtual-link Router-LSA record is emitted through the existing `RouterLSA.WriteTo(buf, off)` (already buffer-first); any virtual-link CLI rendering uses `textbuf`/`AppendTo`, never `fmt`/`+`

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc5340.md` -- OSPFv3 base
  -> Constraint: §2.9 -- OSPFv3 normally uses link-local source + `ff02::5`/`ff02::6`; virtual-link packets are the exception: unicast to the virtual neighbor's GLOBAL IPv6 address, routed through the transit area (hop limit > 1), source = the local GLOBAL address reachable in the transit area
  -> Constraint: §4.2 -- a virtual link is a configured point-to-point link through a transit (non-backbone, non-stub, non-NSSA) area; it belongs to the backbone (area 0); both endpoints are ABRs of the transit area; the link is up only while the virtual neighbor is reachable by an intra-area path in the transit area
  -> Constraint: §3.5 -- a router that is the endpoint of a fully adjacent virtual link is considered to have an interface to the backbone; this is what makes inter-area routing treat it as backbone-attached
  -> Constraint: App A.4.3 -- the Router-LSA V-bit (`RouterFlagV`, 0x04) is set by a virtual-link endpoint; the virtual link is a Router-LSA link record of type `RouterLinkTypeVirtual` (4) with Neighbor Router ID = the virtual neighbor and metric = the transit-area path cost; there is no Link-LSA on a virtual link
  -> Constraint: §C.2 -- the configurable virtual-link parameters are the transit area, the virtual neighbor Router ID, and the interface timers (RxmtInterval, InfTransDelay, HelloInterval, RouterDeadInterval); the cost is NOT configured -- it is the transit-area SPF path cost (RFC 5340 §4.2, RFC 2328 §15)

**Key insights:**
- The virtual link is two problems stitched together: (1) a routed-unicast transport that breaks the OSPFv3 link-local assumption, and (2) a synthetic backbone interface whose endpoint address is resolved from the transit area's SPF result and refreshed every transit-area SPF run.
- Everything ABOVE the transport (ISM/NSM/DD/LSREQ adjacency, Router-LSA origination, SPF graph) is already present and AF-neutral; the shared SPF already treats `RouterLinkTypeVirtual` as a P2P link and the v3 Router-LSA codec already defines `RouterFlagV` / the virtual link record. The new work is the routed send, the endpoint resolver, the virtual-interface ISM driver, and wiring the virtual record into `v6OriginateRouter` for the backbone.
- The cost is derived, not configured: it equals the transit-area intra-area cost to the virtual neighbor. A virtual link whose neighbor is unreachable in the transit area is Down and is not advertised.
- A virtual interface lives in area 0 but its packets travel the transit area; the next hop for the virtual adjacency (and thus the SPF next hop attributed to the link) is the transit-area next hop toward the virtual neighbor, never the neighbor's own global address used as the packet destination.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
- [ ] `internal/plugins/ospf/v3/transport/transport.go` -- `SendPacket(name, dst, payload)` requires `dst.Is6()`, looks up the per-interface open socket, and sends from `st.handle.LinkLocalSource()` with the interface bound; `FinalizePacketChecksum(src, dst, payload)` uses that link-local source; `InterfaceHandle.Send(dst, src, payload)` is the backend seam; `EnableInterface`/`enableInterfaceInstance` key interfaces by name + Instance ID; there is NO routed-unicast / global-source send path
  -> Constraint: the virtual-link send MUST NOT use `LinkLocalSource()`; it needs the local GLOBAL source (so the checksum pseudo-header matches) and a routed hop limit; this is a NEW transport entry point (`SendRouted` or a virtual-interface handle), not a tweak to `SendPacket`
- [ ] `internal/plugins/ospf/v3/transport/backend_linux.go` -- `resolveOSPFv3Interface` requires a link-local source (`ErrNoLinkLocal` when DAD pending); `listenNetwork = "ip6:89"`; the backend opens one raw socket per physical interface and joins `ff02::5`
  -> Constraint: a virtual link has no physical socket of its own; it must send through a socket that can reach the routed destination (a transit-area egress socket or an unbound routed socket); the hop limit must be set > 1, unlike the link-local path that uses hop limit 1 implicitly
- [ ] `internal/plugins/ospf/v3/packet/lsa_router.go` -- `RouterFlagV = 0x04`, `RouterLinkTypeVirtual = 4`, `RouterLink{Type, Metric, InterfaceID, NeighborInterfaceID, NeighborRouterID}`; `RouterLSA.WriteTo` already serializes any link type
  -> Constraint: the codec is complete; this spec only CONSTRUCTS the virtual record in origination -- no codec change
- [ ] `internal/plugins/ospf/origination_v6.go` -- `v6OriginateSelf` builds per-area Router-LSAs from the topology snapshot; `v6RouterLSABody` emits P2P + transit links per interface; `v6IsAreaBorderRouter(areas)` = `len>=2 && contains(BackboneArea)`; there is NO virtual-link record and the V-bit is never set
  -> Constraint: the virtual link is advertised in the **backbone** Router-LSA, not the transit area's; `v6OriginateSelf` must learn the active virtual links (Full adjacency + resolved neighbor) and inject a virtual record into area 0's body, setting `RouterFlagV`; a router with only a virtual link to area 0 still counts as backbone-attached for `v6IsAreaBorderRouter` / inter-area computation
- [ ] `internal/plugins/ospf/afstrategy_v6.go` -- `v6RouterLinks` translates P2P + transit links into the shared SPF graph but DROPS any other link type (the `switch` has no `RouterLinkTypeVirtual` case); `v6NextHop.P2PNextHop` resolves via `neighbors.AddressOf`
  -> Constraint: `v6RouterLinks` must add a `RouterLinkTypeVirtual` case (map it onto the shared `packet.RouterLinkTypeVirtual`, which Dijkstra already treats as P2P); the virtual next hop must resolve to the transit-area next hop toward the neighbor, not the neighbor's global packet address
- [ ] `internal/plugins/ospf/spf/spf.go` -- the shared Dijkstra already accepts `packet.RouterLinkTypeVirtual` alongside `RouterLinkTypeP2P` in graph build + two-way check (lines ~183, 266, 332)
  -> Constraint: no SPF core change is needed for the backbone graph; the work is feeding the virtual link into the backbone graph (origination side) and resolving its next hop (strategy side)
- [ ] `internal/plugins/ospf/spf/interarea.go` -- inter-area computation comments "Virtual-link backbone repair is out of scope" and computes the backbone-attached condition from real area membership
  -> Constraint: the backbone-attached condition must now also be true when a Full virtual link to area 0 exists; this is the §3.5 rule and it must be threaded into the inter-area / ABR-eligibility logic
- [ ] `internal/plugins/ospf/config.go` -- `areaConfig` (AreaID, AreaType, Ranges, NSSA...), `interfaceConfig`, `ospfConfig` with `V6 *ospfConfig`; `applyTree` parses `areas/area`; `area.AreaType` enum is normal/stub/nssa; there is NO virtual-link config
  -> Constraint: virtual links are configured UNDER the transit area (`areas/area/virtual-link`), keyed by remote Router ID; parsing rejects a transit area that is stub or NSSA (RFC 5340 §4.2) and rejects area 0 as a transit area; resolved into a `virtualLinkConfig` slice on the area or on the `V6` family config
- [ ] `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- `areas/area` has `area-id`, `area-type`, `ranges/range`, `nssa`, `authentication`; the interface list has `network-type` (broadcast/point-to-point/loopback), timers, priority
  -> Constraint: add a `virtual-link` list under `areas/area` keyed by `remote-router-id` (dotted-quad Router ID) with the C.2 timer leaves; native YANG constraints (range/pattern) on every leaf; a custom validator (`ze:validate`) rejects stub/NSSA/backbone transit areas and a self Router ID
- [ ] `internal/plugins/ospf/neighbor/table.go` + `iface/ism.go` -- the adjacency table keyed by (interface name, Router ID); `AddressOf` returns the neighbor's reachable address; the ISM has `StatePointToPoint` with no DR election; `FloodNeighbors` drives flooding per interface
  -> Constraint: the virtual interface registers in these tables under a synthetic virtual-interface name; its NSM runs the standard point-to-point adjacency; the neighbor "address" recorded for the virtual neighbor is its resolved GLOBAL address (the packet destination), distinct from the SPF transit next hop
- [ ] `internal/plugins/ospf/instance.go` -- `engine` holds `transport`, `areas`, `interfaces`, `neighbors`, `lsdb`, `spf`; `openInterfaces`/`reconcile` open configured interfaces; `originateSelfLSAs` regenerates self-LSAs on topology change; the engine drives the v6 strategy
  -> Constraint: the engine gains a virtual-link manager: it learns virtual links from config, resolves endpoints after each transit-area SPF run, opens/closes the virtual interface, and triggers `originateSelfLSAs` when a virtual link's Full/Down state changes

**Behavior to preserve:**
- The OSPFv3 link-local + `ff02::5`/`ff02::6` transport for every normal interface (`SendPacket` unchanged); the v3 codec; the AF-neutral LSDB key; `v6OriginateSelf` behaviour for non-virtual interfaces; `v6IsAreaBorderRouter` for routers with real backbone interfaces.
- All existing OSPFv3 functional/interop tests: a router with no virtual link configured behaves exactly as today.
- The shared SPF Dijkstra (already virtual-aware) and the v6 next-hop resolution for normal P2P/transit links.

**Behavior to change:** (all RFC-5340-required, not discretionary)
- `v6RouterLinks` (`afstrategy_v6.go`): add the `RouterLinkTypeVirtual` case so the backbone graph includes virtual links.
- `v6OriginateSelf` / `v6RouterLSABody` (`origination_v6.go`): inject the virtual-link record into the backbone Router-LSA and set `RouterFlagV`; treat a Full virtual link as backbone attachment for ABR detection.
- v3 transport: add a routed-unicast send path with a global source and hop limit > 1.
- Inter-area / backbone-attached condition (`spf/interarea.go` consumers): a Full virtual link to area 0 satisfies backbone attachment (§3.5).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Config:** `areas/area/virtual-link` (under the transit area) -> `applyTree` -> `virtualLinkConfig` on `ospfConfig` / `V6` -> the engine's virtual-link manager.
- **Endpoint resolution:** after each transit-area SPF run, the manager resolves the virtual neighbor's reachable global IPv6 address (and the local global source) from the transit-area SPF result + Intra-Area-Prefix-LSAs.
- **Adjacency:** the virtual interface's NSM exchanges DD/LSREQ/LSUpdate over the routed transport to reach Full.
- **Origination:** a Full virtual link triggers `originateSelfLSAs`, which emits the backbone Router-LSA with the V-bit and the virtual record.
- **SPF:** the backbone SPF graph includes the virtual link; inter-area computation treats the endpoint as backbone-attached.

### Transformation Path
1. **Config parse (new):** the `virtual-link` list under the transit area is validated (transit area not stub/NSSA/backbone; remote Router ID not self) and resolved into `virtualLinkConfig{TransitArea, RemoteRouterID, Hello, Dead, Retransmit, TransmitDelay}`.
2. **Endpoint resolution (new):** the manager runs after the transit-area SPF; it finds the virtual neighbor's router vertex in the transit area, takes the SPF next hop (transit-area next hop) and the neighbor's advertised global prefix (from its Intra-Area-Prefix-LSA) as the packet destination; the local global source is the router's own transit-area global address; if the neighbor is unreachable, the virtual link is marked Down.
3. **Virtual interface open (new):** when an endpoint is resolved, the manager opens a synthetic backbone-area point-to-point virtual interface with the configured timers; the ISM goes to `StatePointToPoint` (no DR/BDR).
4. **Adjacency (reused):** the NSM (`neighbor/`) drives DD/LSREQ/LSUpdate to Full over the routed transport; the recorded neighbor address is the virtual neighbor's global packet destination.
5. **Routed send (new):** outgoing virtual-link packets go through the new v3 transport routed-unicast path: global source, global destination, hop limit > 1, not bound to a single ifindex; the checksum pseudo-header uses the global source.
6. **Router-LSA origination (extended):** `v6OriginateSelf` injects a `RouterLinkTypeVirtual` record into the **backbone** Router-LSA (Interface ID = virtual-interface ID, Neighbor Router ID = virtual neighbor, metric = transit-area path cost) and sets `RouterFlagV`; the LSDB owns sequencing/flood.
7. **SPF (extended):** `v6RouterLinks` maps the virtual record into the backbone graph as a P2P-like link; Dijkstra (already virtual-aware) computes the backbone tree; the virtual next hop resolves to the transit-area next hop; inter-area computation sees the endpoint as backbone-attached (§3.5).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree <-> engine | `areas/area/virtual-link` -> `virtualLinkConfig` -> virtual-link manager | [ ] |
| Transit SPF <-> endpoint resolver | transit-area SPF result + Intra-Area-Prefix-LSA -> resolved global dest + transit next hop | [ ] |
| Virtual interface <-> NSM | synthetic backbone p2p interface registers in the neighbor/ISM tables; NSM reaches Full | [ ] |
| Engine <-> v3 transport | new routed-unicast send (global src/dst, hop limit > 1) vs link-local `SendPacket` | [ ] |
| Origination <-> backbone Router-LSA | `RouterFlagV` + `RouterLinkTypeVirtual` record injected into area 0's body | [ ] |
| Router-LSA <-> SPF | `v6RouterLinks` virtual case -> shared Dijkstra (already virtual-aware) -> backbone-attached condition | [ ] |

### Integration Points
- `internal/plugins/ospf/v3/transport` -- a new routed-unicast send (global source, hop limit > 1); the existing `SendPacket`/`InterfaceHandle` are unchanged.
- `internal/plugins/ospf/v3/packet/lsa_router.go` -- `RouterFlagV`, `RouterLinkTypeVirtual`, `RouterLink` (consumed, not redefined).
- `internal/plugins/ospf/origination_v6.go` -- backbone Router-LSA virtual record + V-bit; backbone-attachment via virtual link.
- `internal/plugins/ospf/afstrategy_v6.go` -- the `v6RouterLinks` virtual case + the virtual next-hop resolution.
- `internal/plugins/ospf/spf/` -- READ-MOSTLY: the Dijkstra is already virtual-aware; the backbone-attached condition (`interarea.go`) gains a virtual-link input.
- `internal/plugins/ospf/neighbor` + `iface` -- the virtual interface's adjacency + ISM (reused point-to-point machinery under a synthetic interface name).
- `internal/plugins/ospf/config.go` + `yang/ze-ospf-conf.yang` -- the `virtual-link` config surface + validation.
- `internal/plugins/ospf/instance.go` -- the virtual-link manager (resolve, open/close, trigger origination).
- `internal/plugins/ospf/cmd_show.go` + the v3 show path -- the virtual interface/neighbor rows.

### Architectural Verification
- [ ] No bypassed layers (virtual-link packets flow engine -> routed transport -> wire and back through the normal NSM/LSDB/SPF spine; only the transport send and the endpoint resolution are new)
- [ ] No unintended coupling (the routed send is a transport API; the resolver reads the transit-area SPF result read-only; SPF stays AF-neutral)
- [ ] No duplicated functionality (reuses NSM/DD/LSREQ, `OriginateSelf`, the shared Dijkstra and its existing virtual handling, the v6 codec; adds only the routed send, the resolver, the virtual-interface driver, and the backbone virtual record)
- [ ] Zero-copy preserved (the virtual Router-LSA record is emitted through the existing buffer-first `RouterLSA.WriteTo`; the routed send reuses the existing packet buffer)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The v3 codec already supports the V-bit and the virtual-link Router-LSA record, so no codec change is needed | `v3/packet/lsa_router.go` `RouterFlagV`, `RouterLinkTypeVirtual`, `RouterLSA.WriteTo` | codec work + scope creep | `TestV6RouterLSAVirtualRecordRoundTrip` decodes/re-encodes a virtual record byte-for-byte | unvalidated |
| A-2 | The shared Dijkstra (`spf/spf.go`) already treats `RouterLinkTypeVirtual` as P2P, so the backbone graph needs no core SPF change | `spf/spf.go` lines ~183/266/332 accept `RouterLinkTypeVirtual` | SPF core change needed | `TestV6BackboneGraphIncludesVirtualLink` walks the backbone tree through a virtual link | unvalidated |
| A-3 | The transit neighbor's global IPv6 address is resolvable from the transit-area SPF result + its Intra-Area-Prefix-LSA (the same data `v6BuildRoutes` already attaches) | `afstrategy_v6.go` `v6BuildRoutes`, `v6InterfacePrefixes` | a separate address-learning mechanism is needed | `TestVirtualEndpointResolvesGlobalAddress` resolves the neighbor's global dest from a synthetic transit LSDB | unvalidated |
| A-4 | The v3 transport can send a routed unicast with a non-link-local source and hop limit > 1 through an existing/socketed path (golang.org/x/net/ipv6 supports `ControlMessage.HopLimit` and `Src`) | `v3/transport/backend_linux.go` uses `golang.org/x/net/ipv6`; `Send(dst, src, payload)` already takes an explicit src | a new socket type or kernel feature is required | `TestRoutedSendUsesGlobalSourceAndHopLimit` (fake backend asserts src/hop-limit); QEMU virtual-link interop | unvalidated |
| A-5 | The reused point-to-point NSM/DD/LSREQ machinery forms a Full adjacency over a routed unicast path with no link-local assumptions in the adjacency code | `neighbor/` table keyed by (iface,RID); `AddressOf` returns a recorded address, not necessarily link-local | the NSM hardcodes link-local somewhere; refactor needed | `TestVirtualAdjacencyReachesFull` (engine-level, fake routed transport) | unvalidated |
| A-6 | A Full virtual link to area 0 makes the endpoint backbone-attached for inter-area/ABR computation per §3.5 without breaking the existing real-backbone path | `spf/interarea.go`, `v6IsAreaBorderRouter` | partitioned-backbone routes never appear; §3.5 unmet | `TestVirtualLinkBackboneAttachment` (inter-area route via the virtual link) + interop | unvalidated |
| A-7 | The virtual-link cost is the transit-area intra-area SPF cost to the neighbor, not configured (RFC 5340 §4.2 / §C.2) | `rfc/short/rfc5340.md` §C.2; RFC 2328 §15 | the wrong metric is advertised; routing asymmetry | `TestVirtualLinkCostEqualsTransitCost` | unvalidated |
| A-8 | Configuring a virtual link with a stub/NSSA/backbone transit area or a self Router ID is rejected at parse time (RFC 5340 §4.2) | `config.go` area-type enum; YANG validator hook | invalid config forms a broken link silently | `TestVirtualLinkRejectsStubTransitArea`, `TestVirtualLinkRejectsSelfRouterID` | unvalidated |
| A-9 | The virtual next hop (for SPF route install) is the transit-area next hop toward the neighbor, distinct from the neighbor's global packet destination | `afstrategy_v6.go` `v6NextHop`; RFC 5340 §4.2 | routes point at an unreachable global address | `TestVirtualNextHopIsTransitNextHop` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The routed send uses the link-local source (copy of `SendPacket`), so the checksum pseudo-header mismatches and FRR drops the packet | FRR logs "bad checksum" on the virtual link; adjacency stuck in Init | a dedicated routed-send path takes an explicit global source and finalizes the checksum against it; `TestRoutedSendUsesGlobalSourceAndHopLimit` + interop |
| R-2 | Hop limit left at 1 so virtual-link packets die in the transit area (never routed) | the neighbor never sees Hellos; adjacency never forms across > 1 hop | the routed send sets a hop limit > 1 (use the IPv4 virtual-link convention of a large TTL); QEMU two-hop transit topology proves traversal |
| R-3 | The virtual link is advertised in the transit area's Router-LSA instead of the backbone's | the transit area's database shows a virtual record; backbone has none; inter-area routing still broken | origination injects the virtual record into area 0 ONLY; `TestVirtualRecordInBackboneRouterLSA` asserts the area |
| R-4 | The endpoint resolver caches a stale global address after the transit topology changes, so packets go to a dead destination | adjacency flaps after an unrelated transit SPF run | resolve on EVERY transit-area SPF run; mark Down when unreachable; `TestVirtualEndpointReresolvesOnSPF` |
| R-5 | A virtual link counts as backbone attachment even when not Full, so a half-formed link wrongly makes the router an ABR | spurious B-bit / inter-area routes during adjacency bring-up | backbone-attachment requires the virtual link at Full; `TestVirtualLinkBackboneAttachment` gates on state |
| R-6 | Two-way / reciprocal SPF check fails because the virtual record's Interface IDs do not match what the neighbor advertises | the backbone tree omits the virtual link even though both ends are Full | follow the v6 P2P convention (key the two-way check on Neighbor Router ID, RFC 5340 App A.4.3); `TestV6BackboneGraphIncludesVirtualLink` includes the two-way check |
| R-7 | An interop mismatch: FRR's OSPFv3 virtual link expects a specific Interface-ID allocation or unnumbered behaviour Ze does not match | FRR shows the virtual neighbor stuck in ExStart / DD mismatch | the QEMU `ospfv3-vlink-frr` scenario validates against `ospf6d`; follow FRR's virtual Interface-ID handling |
| R-8 | Resource/identity clash: the synthetic virtual-interface name or Interface ID collides with a real interface | the virtual interface shadows a real one in the tables | allocate the virtual interface name/ID from a reserved namespace; `TestVirtualInterfaceNameReserved` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `areas/area/virtual-link { remote-router-id ... }` under a transit area | -> | `applyTree` parses + validates -> `virtualLinkConfig` on the config -> the engine's virtual-link manager registers it | `TestParseVirtualLinkConfig` (unit) + `test/ospfv3/ospfv3-vlink-config.ci` |
| A transit area with a reachable virtual neighbor (after transit SPF) | -> | the endpoint resolver computes the global dest + transit next hop; the virtual interface opens; the NSM reaches Full | `test/ospfv3/ospfv3-vlink.ci` |
| A Full virtual link | -> | `v6OriginateSelf` emits the backbone Router-LSA with `RouterFlagV` + the `RouterLinkTypeVirtual` record | `TestVirtualRecordInBackboneRouterLSA` (unit) + `ospfv3-vlink-frr` interop |
| The backbone SPF run with a virtual link present | -> | `v6RouterLinks` maps the virtual record into the backbone graph; Dijkstra builds the tree; inter-area sees backbone attachment | `TestVirtualLinkBackboneAttachment` (unit) |
| An outgoing virtual-link packet | -> | the v3 routed-unicast send uses the global source + hop limit > 1, not `LinkLocalSource()` | `TestRoutedSendUsesGlobalSourceAndHopLimit` (unit) + interop |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `areas/area/virtual-link { remote-router-id R }` configured under a normal transit area | parsed into a `virtualLinkConfig`; the engine registers a pending virtual link to R through that transit area (RFC 5340 §C.2) |
| AC-2 | A virtual link configured with a stub or NSSA or backbone (0.0.0.0) transit area, or `remote-router-id` equal to this router's own Router ID | rejected at config validation with a clear error; no virtual link is created (RFC 5340 §4.2) |
| AC-3 | The virtual neighbor is reachable by an intra-area path in the transit area | the resolver computes the neighbor's GLOBAL IPv6 destination and the transit-area next hop; the virtual interface opens as a backbone point-to-point interface with the configured timers (RFC 5340 §2.9, §4.2) |
| AC-4 | The virtual neighbor becomes unreachable in the transit area (transit topology change) | the virtual link goes Down; the backbone Router-LSA virtual record + V-bit are withdrawn; the endpoint is re-resolved on the next transit SPF (RFC 5340 §4.2) |
| AC-5 | A virtual-link packet is sent | it is sent to the virtual neighbor's GLOBAL address from the local GLOBAL source with an IPv6 hop limit > 1, routed (not bound to one ifindex), and the checksum pseudo-header uses the global source (RFC 5340 §2.9) |
| AC-6 | Both endpoints are configured and reachable | the virtual interface NSM reaches Full using the standard point-to-point DD/LSREQ exchange (no DR/BDR, no Network-LSA, no Link-LSA) (RFC 5340 §4.2) |
| AC-7 | A Full virtual link | the backbone (area 0) Router-LSA sets `RouterFlagV` and carries a `RouterLinkTypeVirtual` record naming the virtual neighbor, with metric = the transit-area path cost to that neighbor (RFC 5340 App A.4.3, §4.2) |
| AC-8 | The transit-area path cost to the virtual neighbor changes | the advertised virtual-link metric in the backbone Router-LSA updates to match (RFC 5340 §4.2, §C.2: cost is the transit-area cost, not configured) |
| AC-9 | The backbone SPF runs with a Full virtual link | the backbone graph includes the virtual link as a P2P-like edge (two-way checked on Neighbor Router ID); the route through the virtual neighbor resolves its next hop to the TRANSIT-area next hop, not the neighbor's global packet destination (RFC 5340 §4.2) |
| AC-10 | An ABR whose only path to area 0 is a Full virtual link | it is treated as backbone-attached: it participates in inter-area route computation and inter-area routes that depend on the repaired backbone appear (RFC 5340 §3.5) |
| AC-11 | A partitioned backbone repaired by a virtual link between the two partitions | inter-area reachability between the partitions is restored through the virtual link (RFC 5340 §4.2) |
| AC-12 | `show ipv6 ospf6 interface` / `show ipv6 ospf6 neighbor` while a virtual link is Full | the virtual interface and the virtual neighbor are listed with their state and the resolved address; a Down virtual link shows Down |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures a virtual link under a transit area on both endpoints | config -> `applyTree` -> `virtualLinkConfig` -> manager -> resolver -> virtual interface -> NSM Full; `show ipv6 ospf6 neighbor` shows the virtual neighbor Full | `test/ospfv3/ospfv3-vlink.ci` |
| 2 | Repairs an ABR with no physical backbone interface | the virtual link to an area-0 router makes the ABR backbone-attached; inter-area routes through it appear in the RIB | `test/ospfv3/ospfv3-vlink-backbone-repair.ci` + `ospfv3-vlink-frr` interop |
| 3 | Forms a virtual-link adjacency with FRR `ospf6d` across a transit area | routed-unicast OSPFv3 packets (global src/dst, hop limit > 1) traverse the transit area; both reach Full; FRR shows Ze's backbone Router-LSA with the V-bit | `ospfv3-vlink-frr` interop (QEMU two-hop transit) |
| 4 | Misconfigures a stub transit area | config validation rejects it; no broken virtual link is created | `test/ospfv3/ospfv3-vlink-config.ci` |
| 5 | Watches a transit-area topology change move the virtual neighbor | the metric and next hop update; if the neighbor goes unreachable the virtual link goes Down and the V-bit/record are withdrawn | `test/ospfv3/ospfv3-vlink-reresolve.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseVirtualLinkConfig` | `internal/plugins/ospf/config_test.go` | AC-1: `areas/area/virtual-link` parses into `virtualLinkConfig` | |
| `TestVirtualLinkRejectsStubTransitArea` / `TestVirtualLinkRejectsSelfRouterID` / `TestVirtualLinkRejectsBackboneTransit` | `internal/plugins/ospf/config_test.go` | AC-2, A-8: config validation rejects stub/NSSA/backbone transit + self RID | |
| `TestVirtualEndpointResolvesGlobalAddress` | `internal/plugins/ospf/virtuallink_v6_test.go` | AC-3, A-3: resolve the neighbor's global dest from the transit SPF + Intra-Area-Prefix-LSA | |
| `TestVirtualEndpointReresolvesOnSPF` | `internal/plugins/ospf/virtuallink_v6_test.go` | AC-4, R-4: re-resolve on each transit SPF; Down when unreachable | |
| `TestVirtualNextHopIsTransitNextHop` | `internal/plugins/ospf/virtuallink_v6_test.go` | AC-9, A-9: SPF next hop is the transit next hop, not the global dest | |
| `TestRoutedSendUsesGlobalSourceAndHopLimit` | `internal/plugins/ospf/v3/transport/transport_test.go` | AC-5, A-4, R-1, R-2: routed send uses global src + hop limit > 1, not link-local | |
| `TestV6RouterLSAVirtualRecordRoundTrip` | `internal/plugins/ospf/v3/packet/lsa_router_test.go` | A-1: virtual record + V-bit decode/re-encode byte-for-byte | |
| `TestVirtualRecordInBackboneRouterLSA` | `internal/plugins/ospf/origination_v6_test.go` | AC-7, R-3: V-bit + virtual record land in the BACKBONE Router-LSA only | |
| `TestVirtualLinkCostEqualsTransitCost` | `internal/plugins/ospf/origination_v6_test.go` | AC-8, A-7: advertised metric = transit-area path cost | |
| `TestVirtualLinkWithdrawnWhenDown` | `internal/plugins/ospf/origination_v6_test.go` | AC-4: a Down virtual link withdraws the record + V-bit | |
| `TestV6BackboneGraphIncludesVirtualLink` | `internal/plugins/ospf/afstrategy_v6_test.go` | AC-9, A-2, R-6: backbone graph + two-way check include the virtual link | |
| `TestVirtualLinkBackboneAttachment` | `internal/plugins/ospf/afstrategy_v6_test.go` | AC-10, A-6, R-5: a Full virtual link makes the endpoint backbone-attached (gated on Full) | |
| `TestVirtualAdjacencyReachesFull` | `internal/plugins/ospf/instance_v6_test.go` | AC-6, A-5: the virtual NSM reaches Full over a fake routed transport | |
| `TestVirtualInterfaceNameReserved` | `internal/plugins/ospf/instance_v6_test.go` | R-8: the synthetic virtual interface name/ID does not clash with a real interface | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| virtual-link hello-interval (s) | 1..65535 | 65535 | 0 | N/A (uint16) |
| virtual-link dead-interval (s) | 1..65535 | 65535 | 0 | N/A (uint16) |
| virtual-link retransmit-interval (s) | 1..65535 | 65535 | 0 | N/A (uint16) |
| virtual-link transmit-delay (s) | 1..3600 | 3600 | 0 | 3601 |
| remote-router-id | dotted-quad / uint32 | 255.255.255.255 | N/A | self RID rejected (AC-2) |
| advertised virtual-link metric | 1..65534 (LSInfinity 0xffff excluded) | 65534 | 0 -> coerced to 1 | unreachable -> link Down, not advertised |
| IPv6 hop limit on virtual send | 2..255 | 255 | 1 (would not route) | N/A (uint8) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospfv3-vlink-config` | `test/ospfv3/ospfv3-vlink-config.ci` | virtual-link config parses; a stub/NSSA/backbone transit area or self RID is rejected | |
| `ospfv3-vlink` | `test/ospfv3/ospfv3-vlink.ci` | two Ze routers form a virtual-link adjacency to Full across a transit area; `show ipv6 ospf6 neighbor` shows the virtual neighbor | |
| `ospfv3-vlink-backbone-repair` | `test/ospfv3/ospfv3-vlink-backbone-repair.ci` | an ABR with no physical backbone interface becomes backbone-attached; inter-area routes appear | |
| `ospfv3-vlink-reresolve` | `test/ospfv3/ospfv3-vlink-reresolve.ci` | a transit topology change updates the metric/next hop; an unreachable neighbor drives the link Down and withdraws the record | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospfv3-vlink-frr` | `test/interop/scenarios/ospfv3-vlink-frr/` | FRR `ospf6d` (virtual link configured at the far end, two-hop transit area) | Ze sends routed-unicast OSPFv3 (global src/dst, hop limit > 1), forms a Full virtual adjacency across the transit area, advertises the backbone Router-LSA with the V-bit + virtual record, and FRR accepts it; inter-area reachability is restored through the virtual link | |

> Interop is required: virtual links change wire behaviour (routed unicast,
> global addressing, the Router-LSA V-bit + virtual record). The raw IPv6 routed
> path is Linux-only and runs as a QEMU integration test
> (`ai/rules/qemu-testing.md`), consistent with the rest of the OSPFv3 interop set;
> the transit area requires a multi-hop QEMU topology so the routed send is
> genuinely exercised (not a single-hop shortcut).

### Future (if deferring any tests)
- None. Every AC is covered by a unit, functional, or interop test above.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*) -->
- `internal/plugins/ospf/config.go` -- the `virtualLinkConfig` type; `applyTree` parsing under `areas/area/virtual-link`; validation rejecting stub/NSSA/backbone transit areas and a self Router ID; surfacing virtual links on `ospfConfig` / `V6`
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- a `virtual-link` list under `areas/area` keyed by `remote-router-id`, with the C.2 timer leaves and native range/pattern constraints
- `internal/plugins/ospf/origination_v6.go` -- inject the `RouterLinkTypeVirtual` record + `RouterFlagV` into the backbone Router-LSA; treat a Full virtual link as backbone attachment for `v6IsAreaBorderRouter`; withdraw on Down
- `internal/plugins/ospf/afstrategy_v6.go` -- the `RouterLinkTypeVirtual` case in `v6RouterLinks`; the virtual next-hop resolution (transit next hop, not the global dest)
- `internal/plugins/ospf/instance.go` -- the virtual-link manager hook: register from config, resolve endpoints after transit SPF, open/close the virtual interface, trigger `originateSelfLSAs` on state change
- `internal/plugins/ospf/v3/transport/transport.go` -- a routed-unicast send path (global source, hop limit > 1, not bound to one ifindex); checksum finalized against the global source
- `internal/plugins/ospf/v3/transport/backend_linux.go` + `backend_other.go` -- the backend support for a routed send (hop limit control, explicit global source); `transport_iface.go` if the engine-side transport adapter needs the new entry point
- `internal/plugins/ospf/spf/interarea.go` -- thread the virtual-link backbone-attachment condition into the inter-area / ABR-eligibility logic (§3.5)
- `internal/plugins/ospf/cmd_show.go` + the v3 show snapshot path -- list the virtual interface + virtual neighbor rows
- `internal/plugins/ospf/doctor.go` -- only if a runtime dependency is added; none expected (the routed send reuses the existing raw IPv6 socket family)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- the `virtual-link` list; read `ai/rules/config-surface.md` + `ai/rules/config-naming.md` |
| YANG validation constraints | [ ] yes | every timer leaf has `range`; `remote-router-id` has the dotted-quad/uint32 `pattern` (mirroring `area-id`) |
| YANG custom validators | [ ] yes | a `ze:validate` validator rejects a stub/NSSA/backbone transit area and a self Router ID (AC-2); `CompleteFn` offers known area IDs |
| CLI commands/flags | [ ] yes | `show ipv6 ospf6 interface`/`neighbor` gain virtual rows in `ze-ospf-cmd.yang` + `cmd_show.go` |
| CLI grammar (action before identifier) | [ ] yes | `ai/rules/cli-grammar.md` -- show subcommands unchanged in shape |
| Editor autocomplete | [ ] yes | automatic for the YANG leaves; `CompleteFn` for the transit-area / remote-router-id |
| Functional test for new RPC/API | [ ] yes | `test/ospfv3/ospfv3-vlink*.ci` |
| Pipe completeness | [ ] yes | the virtual rows route through `ApplyPipes` like the other show outputs |
| Env var registration | [ ] no | virtual links are operational config, not an `environment/` leaf |
| Doctor check for runtime dependencies | [ ] no | no new socket/port/binary/cert; the routed send reuses the existing raw IPv6 (proto 89) socket family |
| Prometheus counters/metrics | [ ] yes | see the metrics rows below |

#### Metrics (new series owned by this spec)
| Metric | Type | Labels |
|--------|------|--------|
| `ze_ospfv3_virtual_links` | gauge | `transit_area`, `state` (down/full) |
| `ze_ospfv3_virtual_link_cost` | gauge | `transit_area`, `remote_router_id` |
| `ze_ospfv3_virtual_link_reresolves_total` | counter | `transit_area` |

> These extend the umbrella's `ze_ospfv3_*` metric set; they are registered by
> this spec's owner code. The umbrella metrics list gains these rows when this
> spec lands.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` -- OSPFv3 virtual links |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` -- the `virtual-link` list |
| 3 | CLI command added/changed? | [ ] yes | `docs/guide/command-reference.md` -- virtual rows in `show ipv6 ospf6 interface`/`neighbor` |
| 4 | API/RPC added/changed? | [ ] no | show RPCs live in the central `ze-show` namespace; documented under the command reference |
| 5 | Plugin added/changed? | [ ] yes | `docs/guide/plugins.md` -- OSPF gains virtual-link config + a routed transport path |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/ospfv3.md` (or the OSPF guide) -- virtual-link section |
| 7 | Wire format changed? | [ ] yes | `docs/architecture/wire/ospfv3.md` (or equivalent) -- routed unicast + Router-LSA V-bit / virtual record |
| 8 | Plugin SDK/protocol changed? | [ ] no | no plugin SDK surface change |
| 9 | RFC behavior implemented? | [ ] yes | `rfc/short/rfc5340.md` -- mark §4.2 virtual links as implemented |
| 10 | Test infrastructure changed? | [ ] yes (interop + multi-hop QEMU) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` -- OSPFv3 virtual-link parity with FRR |
| 12 | Internal architecture changed? | [ ] yes | the OSPF subsystem doc -- the virtual-link manager + routed transport |
| 13 | Route metadata keys added/changed? | [ ] no | virtual links do not add route metadata keys |
| 14 | Prometheus counters added/changed? | [ ] yes | the OSPF telemetry doc -- the three `ze_ospfv3_virtual_link*` series |
| 15 | Registered plugin/event/command/capability inventory changed? | [ ] yes | `docs/plugin-overview.md` + the umbrella metrics list |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors into the changed OSPF/transport files |
| 17 | Existing docs show examples for this area? | [ ] check | verify OSPFv3 config/CLI examples against the new `virtual-link` leaf |

## Files to Create
- `internal/plugins/ospf/virtuallink_v6.go` -- the virtual-link manager: config registration, endpoint resolution (global dest + transit next hop from the transit SPF result), virtual-interface open/close, state tracking, origination trigger
- `internal/plugins/ospf/virtuallink_v6_test.go` -- the resolver, re-resolution, next-hop, and state-tracking unit tests
- `test/ospfv3/ospfv3-vlink-config.ci`, `ospfv3-vlink.ci`, `ospfv3-vlink-backbone-repair.ci`, `ospfv3-vlink-reresolve.ci`
- `test/interop/scenarios/ospfv3-vlink-frr/` -- `ze.conf`, `frr.conf`, `check.py`, and a multi-hop transit-area QEMU topology

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan -- confirm the codec V-bit/virtual record + the shared-SPF virtual handling exist |
| 3. Wiring phase | Wiring Test table -- config + manager + failing wiring tests |
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

1. **Phase: Wiring (MANDATORY FIRST)** -- config surface + the virtual-link manager skeleton + failing wiring tests
   - Tests: `TestParseVirtualLinkConfig`, `TestVirtualLinkRejectsStubTransitArea`, `TestVirtualLinkRejectsSelfRouterID`, `test/ospfv3/ospfv3-vlink-config.ci`
   - Files: `config.go` (`virtualLinkConfig` + parse + validation), `yang/ze-ospf-conf.yang` (the `virtual-link` list + validator), `virtuallink_v6.go` (manager skeleton registered from the engine), `instance.go` (manager hook)
   - Verify: config parses + validates; the manager registers a pending virtual link; resolution/adjacency are stubs so deeper tests still fail
2. **Phase: Routed-unicast transport** -- the global-source, hop-limit > 1 send
   - Tests: `TestRoutedSendUsesGlobalSourceAndHopLimit`
   - Files: `v3/transport/transport.go` (the routed send entry point), `v3/transport/backend_linux.go` + `backend_other.go` (hop limit + explicit global source), `transport_iface.go` if needed
   - Verify: the routed send uses the global source + hop limit > 1, not `LinkLocalSource()`; checksum finalized against the global source
3. **Phase: Endpoint resolution** -- global dest + transit next hop from the transit SPF result
   - Tests: `TestVirtualEndpointResolvesGlobalAddress`, `TestVirtualEndpointReresolvesOnSPF`, `TestVirtualNextHopIsTransitNextHop`, `TestVirtualInterfaceNameReserved`
   - Files: `virtuallink_v6.go` (resolver), `afstrategy_v6.go` (the transit next-hop read)
   - Verify: the neighbor's global dest + transit next hop resolve; re-resolve on each transit SPF; Down when unreachable; the virtual interface name/ID is reserved
4. **Phase: Virtual adjacency** -- open the virtual interface, run the point-to-point NSM to Full over the routed transport
   - Tests: `TestVirtualAdjacencyReachesFull`, `ospfv3-vlink.ci`
   - Files: `instance.go` (open/close the virtual interface; ISM `StatePointToPoint`, no DR), reuse `neighbor/` + `iface/`
   - Verify: the virtual NSM reaches Full; `show ipv6 ospf6 neighbor` lists the virtual neighbor
5. **Phase: Router-LSA advertisement** -- the backbone V-bit + virtual record
   - Tests: `TestV6RouterLSAVirtualRecordRoundTrip`, `TestVirtualRecordInBackboneRouterLSA`, `TestVirtualLinkCostEqualsTransitCost`, `TestVirtualLinkWithdrawnWhenDown`
   - Files: `origination_v6.go` (inject the record into area 0, set `RouterFlagV`, metric = transit cost, withdraw on Down)
   - Verify: the backbone Router-LSA carries the virtual record + V-bit; the metric tracks the transit cost; a Down link withdraws
6. **Phase: SPF integration + backbone attachment** -- backbone graph + §3.5
   - Tests: `TestV6BackboneGraphIncludesVirtualLink`, `TestVirtualLinkBackboneAttachment`, `ospfv3-vlink-backbone-repair.ci`, `ospfv3-vlink-reresolve.ci`
   - Files: `afstrategy_v6.go` (the `RouterLinkTypeVirtual` case), `spf/interarea.go` (backbone-attachment condition)
   - Verify: the backbone tree includes the virtual link (two-way checked); a Full virtual link makes the endpoint backbone-attached; inter-area routes appear
7. **Phase: CLI + metrics** -- user surface
   - Tests: the `.ci` show outputs
   - Files: `cmd_show.go` + the v3 show path, the three `ze_ospfv3_virtual_link*` metric registrations
   - Verify: virtual interface/neighbor rows render; the metric series register
8. **Functional tests** -> the four `.ci` cover the user-visible behaviour
9. **RFC refs** -> add `// RFC 5340 Section 4.2 / 2.9 / A.4.3` comments on the routed send, the endpoint resolution, the V-bit/virtual record, and the §3.5 backbone-attachment code
10. **Interop** -> `ospfv3-vlink-frr` multi-hop QEMU scenario
11. **Full verification** -> `make ze-verify`
12. **Complete spec** -> audit tables + learned summary; two commits (A: code+spec+learned, B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has file:line implementation |
| Feature completeness | each user story has a working path; parity with FRR's OSPFv3 virtual link (routed unicast, V-bit, backbone repair) |
| Correctness | routed send uses the GLOBAL source + hop limit > 1; the virtual record lands in the BACKBONE Router-LSA only; the metric = transit cost; the SPF next hop = transit next hop (not the global dest); backbone-attachment gated on Full |
| Naming | `ze_ospfv3_virtual_link*` metrics; YANG `virtual-link` / `remote-router-id` kebab-case; `virtualLinkConfig` |
| Data flow | the resolver reads the transit-area SPF result read-only; SPF stays AF-neutral; the routed send is a transport API, not engine glue |
| CLI grammar | `show ipv6 ospf6 interface`/`neighbor` virtual rows action-before-identifier |
| Doctor checks | none added (no new runtime dependency) -- confirm |
| YANG validation | every timer leaf has `range`; the transit-area / self-RID validator rejects invalid config |
| Prometheus counters | the three series defined, registered, listed; umbrella list updated |
| Rule: plugin-self-containment | virtual-link surfaces live entirely in the OSPF plugin; no leak into generic packages |
| Rule: buffer-first | the virtual record is emitted through the existing `RouterLSA.WriteTo`; the routed send reuses the packet buffer |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Virtual-link config + validation | `go test ./internal/plugins/ospf -run TestParseVirtualLinkConfig` and the reject tests |
| Routed-unicast send | `go test ./internal/plugins/ospf/v3/transport -run TestRoutedSendUsesGlobalSourceAndHopLimit` |
| Endpoint resolution + next hop | `go test ./internal/plugins/ospf -run 'TestVirtualEndpoint|TestVirtualNextHop'` |
| Backbone Router-LSA virtual record | `go test ./internal/plugins/ospf -run 'TestVirtualRecordInBackboneRouterLSA|TestVirtualLinkCost'` |
| SPF + backbone attachment | `go test ./internal/plugins/ospf -run 'TestV6BackboneGraphIncludesVirtualLink|TestVirtualLinkBackboneAttachment'` |
| Three metric series registered | `grep -rn 'ze_ospfv3_virtual_link' internal/plugins/ospf` |
| Interop scenario present | `ls test/interop/scenarios/ospfv3-vlink-frr/` |
| Functional tests present | `ls test/ospfv3/ospfv3-vlink*.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | the resolved global destination is validated (a routable global IPv6, not unspecified/multicast/link-local); a malformed transit LSDB never panics the resolver |
| Trust boundary | virtual-link packets are routed across a transit area; they rely on the transit area's inherited auth (RFC 7166 trailer); no new auth surface; do not accept virtual-link packets from an unconfigured remote Router ID |
| Resource exhaustion | the number of virtual links is bounded by config; the resolver runs O(transit routers) per transit SPF, not per packet; no unbounded retry on an unreachable neighbor (Down + wait for the next SPF) |
| Spoofing | a routed unicast from an arbitrary source must still pass the OSPFv3 header checks (version, area, Instance ID, checksum) and match a configured virtual link before any state change |
| Error leakage | resolver/adjacency errors are logged + counted, not surfaced to peers |

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
An OSPFv3 virtual link is not a new protocol surface: the codec already defines the
V-bit and the virtual record, the NSM already forms point-to-point adjacencies, and
the shared Dijkstra already treats `RouterLinkTypeVirtual` as P2P. The genuinely new
work is two layer-crossing changes: a transport send that abandons the OSPFv3
link-local assumption (routed unicast, GLOBAL source, hop limit > 1), and an
endpoint resolver that turns the transit area's SPF result into the virtual
neighbor's reachable global address (recomputed every transit SPF run). Everything
else is wiring the virtual link into the BACKBONE Router-LSA and SPF graph and
honouring §3.5 backbone attachment.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Configure virtual links under the transit area, keyed by remote Router ID | a top-level `virtual-link` list with a transit-area field | matches RFC 5340 §C.2 (the transit area is the natural parent) and FRR/standard CLI; the area is the validation anchor (reject stub/NSSA) |
| A dedicated routed-unicast transport send, not a tweak to `SendPacket` | overload `SendPacket` with a flag | the link-local + ff02::5 path is correct for every normal interface; virtual links are the documented exception (§2.9) and keeping them separate avoids regressing the common path |
| The cost is derived from the transit-area SPF, never configured | a `cost` config leaf | RFC 5340 §C.2 / RFC 2328 §15: the virtual-link cost IS the transit-area path cost; a config leaf would diverge from the standard |
| Reuse the point-to-point ISM/NSM under a synthetic backbone interface | a new virtual-link state machine | the adjacency behaviour is identical to P2P (no DR, no Network/Link-LSA); a new FSM duplicates code |
| Resolve the endpoint on every transit-area SPF run | resolve once at config time | the neighbor's reachability + next hop change with the transit topology; a stale address breaks the link (R-4) |

## Known Limitations
- IPv6 unicast only; multi-AF (RFC 5838) virtual links are out of scope.
- Virtual links cannot transit a stub or NSSA area (RFC 5340 §4.2); this is enforced as a config rejection, not implemented.
- Authentication on the virtual interface inherits the transit area's RFC 7166 trailer config; no virtual-link-specific auth surface is added.
- OSPFv2 virtual links remain a separate follow-up (`plan/spec-ospf-0-umbrella.md`).

## RFC Documentation

Add `// RFC 5340 Section X.Y: "<quoted requirement>"` above the enforcing code:
- §2.9 routed-unicast virtual-link transport (global source/dest, hop limit > 1)
- §4.2 virtual link through a transit (non-backbone/non-stub/non-NSSA) area; up only while the neighbor is reachable intra-area
- §3.5 a fully adjacent virtual-link endpoint is considered backbone-attached
- App A.4.3 the Router-LSA V-bit + the `RouterLinkTypeVirtual` record; metric = transit-area path cost
- §C.2 the configurable virtual-link parameters (transit area, remote Router ID, timers); cost is NOT configured

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
|------|--------|----------|-------|

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
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| v3 virtual-link config (transit area + virtual neighbor Router ID) | functional | `ospfv3-vlink-config.ci` |
| Global-address endpoint resolution | unit | `TestVirtualEndpointResolvesGlobalAddress`, `TestVirtualNextHopIsTransitNextHop` |
| Virtual adjacency over the transit area | functional + interop | `ospfv3-vlink.ci`, `ospfv3-vlink-frr` |
| Router-LSA virtual-link advertisement | unit + interop | `TestVirtualRecordInBackboneRouterLSA`, `ospfv3-vlink-frr` |
| SPF integration + backbone repair | unit + functional | `TestVirtualLinkBackboneAttachment`, `ospfv3-vlink-backbone-repair.ci` |

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
- [ ] AC-1..AC-12 all demonstrated
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
- [ ] RFC 5340 constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (the routed send + resolver serve only virtual links now; justified by §2.9 / §4.2)
- [ ] No speculative features (no NBMA/P2MP, no multi-AF, no configured cost)
- [ ] Single responsibility per component (resolver vs transport vs origination vs SPF)
- [ ] Explicit > implicit behavior (virtual link Down when unreachable, not silently kept)
- [ ] Minimal coupling (SPF AF-neutral; the resolver reads the transit SPF read-only)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (`ospfv3-vlink-frr`)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ospfv3-ext-3-virtual-links.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospfv3-ext-3-virtual-links.md`
