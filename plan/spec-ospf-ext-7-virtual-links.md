# Spec: ospf-ext-7 -- OSPFv2 Virtual Links (RFC 2328 §15)

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-ospf-0-umbrella.md (delivered) |
| Phase | - |
| Updated | 2026-06-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `rfc/short/rfc2328.md` -- §15 virtual links (unnumbered p2p in the backbone through a transit area), §16.1 (intra-area SPF that yields the virtual neighbour's cost + next hop), §16.3 (transit-area Summary-LSA pass + TransitCapability), §A.4.2 (Router-LSA bit V + link Type 4), §A.3.2/§A.3.3 (Hello Network Mask 0.0.0.0, DD Interface MTU 0 on virtual links), §C.4 (virtual-link parameters)
4. `plan/spec-ospf-0-umbrella.md` -- delivered OSPFv2 umbrella; "Shared Contracts" (Router-LSA link records, LSDB key, SPF route table, area/interface model) and the note that virtual links are a SHOULD item layered on the stable base
5. `internal/plugins/ospf/lsdb/origination.go` -- `OriginInput.VirtualLinkEndpoint` (already sets `RouterFlagV`, currently never true) and `routerLinks(in)` (emits P2P/Transit/Stub links, never a Type-4 virtual link)
6. `internal/plugins/ospf/packet/lsa_router.go` -- `RouterFlagV`, `RouterLinkTypeVirtual = 4` (codec already round-trips the virtual link record and the V-bit)
7. `internal/plugins/ospf/spf/spf.go` -- `transitEdges`/`twoWayRouterLink`/`p2pNeighborAddress` already treat `RouterLinkTypeVirtual` exactly like `RouterLinkTypeP2P`; the virtual neighbour is reachable as a router vertex once the link record is originated
8. `internal/plugins/ospf/spf/interarea.go` -- `IsABR` ("Virtual-link backbone repair is out of scope") and the inter-area pass; §16.3 transit-area calculation + TransitCapability are NOT implemented
9. `internal/plugins/ospf/config.go` -- `areaConfig`/`interfaceConfig`/`ospfConfig` resolution; no virtual-link config exists yet
10. `internal/plugins/ospf/transport/backend_linux.go` -- the TX raw socket sets `IP_TTL = 1` (RFC 2328 App D.3 link-local), so virtual-link packets (which MUST traverse the transit area as routed IP, TTL > 1) cannot reuse the per-interface socket unchanged
11. `internal/plugins/ospf/instance.go` -- `lsdbTopology()`, `originateSelfLSAs()`, `openInterface`/`reconcile`, the engine that owns interfaces, neighbours, LSDB and SPF wiring

## Task

Add OSPFv2 **virtual links** (RFC 2328 §15) to the native OSPFv2 plugin at
`internal/plugins/ospf/`. A virtual link is a logical, unnumbered
point-to-point link that belongs to the **backbone** (Area 0.0.0.0) but runs
*through* a non-backbone, non-stub **transit area** between two Area Border
Routers. It exists to repair a partitioned backbone or to attach a remote area
to Area 0 when that area has no physical backbone interface. The virtual
neighbour (configured by its Router ID plus the transit Area ID) is reached over
the transit area's intra-area SPF shortest path; the virtual link's output cost
equals that intra-area path cost; OSPF runs over it as a point-to-point
interface (Hellos, DD, LS Update/Ack, full adjacency) with configurable per-link
timers (HelloInterval, RouterDeadInterval, RxmtInterval, InfTransDelay), and the
endpoint advertises the link in its **backbone** Type 1 Router-LSA as a Type-4
(virtual) link record while setting the Router-LSA V-bit.

The umbrella delivered OSPFv2 with virtual links deliberately out of scope, yet
left the carrier seams in place: the codec already encodes the V-bit
(`packet.RouterFlagV`) and the Type-4 link record (`packet.RouterLinkTypeVirtual`),
the Router-LSA origination input already carries a `VirtualLinkEndpoint` flag
that flips the V-bit (dead today, nothing sets it), and the intra-area SPF graph
walk already treats a Type-4 link exactly like a Type-1 P2P link. What is
missing is the *active* feature: configuration of virtual links, the
transit-area-driven discovery of the virtual neighbour's reachability/cost/next
hop, a synthetic backbone point-to-point interface that forms the adjacency over
routed IP (TTL > 1) across the transit area, origination of the Type-4 link
record into the backbone Router-LSA, and the §16.3 transit-area Summary-LSA pass
(TransitCapability) so destinations whose computed next hop is a virtual link
resolve to their real next hop.

The feature runs entirely inside the existing OSPF edge plugin and registers
through the plugin's own machinery. Removing the virtual-link configuration
removes all virtual-link behaviour and leaves OSPFv2 behaving exactly as today.

### In scope (this spec)

| Item | Detail |
|------|--------|
| Virtual-link configuration | A `virtual-link` list under the `ospf` container keyed by (transit area, virtual-neighbour Router ID), with optional p2p timers (hello-interval, dead-interval, retransmit-interval, transmit-delay) and authentication that inherits from the transit area (RFC 2328 §15 / §C.4) |
| Transit-area reachability/cost/next hop | After the transit area's intra-area SPF (§16.1), resolve each configured virtual neighbour: usable only if it is a reachable router vertex in the transit area whose Router-LSA is non-MaxAge; the link cost = the transit-area intra-area cost; the next hop = the SPF-computed next hop toward the virtual neighbour over the transit area |
| Synthetic backbone p2p interface | A virtual interface bound to Area 0.0.0.0 that runs the p2p interface/neighbour state machine with the virtual neighbour, sending OSPF packets **unicast to the virtual neighbour's transit address with TTL > 1** (routed IP, not the TTL-1 per-interface socket); Hello Network Mask = 0.0.0.0, DD Interface MTU = 0 (§A.3.2/§A.3.3) |
| Router-LSA virtual-link advertisement | When a virtual link's adjacency is Full, originate a Type-4 (virtual) link record into the **backbone** Router-LSA (LinkID = virtual neighbour Router ID, LinkData = the local transit-area interface address used to reach it, Metric = transit-area cost) and set the Router-LSA V-bit (`VirtualLinkEndpoint`) (§A.4.2) |
| Transit-area Summary-LSA pass (§16.3) | A new transit-area SPF stage (TransitCapability TRUE when a Type-1 with the V-bit exists in the area): improve already-reachable backbone routes and resolve the *real* next hop for destinations whose §16.2/§16.1 next hop was a virtual link; discard unresolved virtual next hops afterward |
| Show / observability | `show ip ospf virtual-links` (state, transit area, virtual neighbour, cost, next hop), the V-bit and Type-4 link surfaced in `show ip ospf database router`, and virtual-link metrics |

### Out of scope (noted so it is not silently assumed done)

| Item | Where |
|------|-------|
| NBMA networks and the NBMA neighbour list | spec-ospf-ext-8 (explicitly excluded here) |
| OSPFv3 virtual links (RFC 5340 §4.4.3.5 carries them as Type-4 router links with the V-bit, but over global IPv6 and a separate Instance-ID surface) | a future OSPFv3 spec; this spec is OSPFv2-only |
| Sham links (MPLS VPN, RFC 4577) | not planned |
| Point-to-multipoint network type | not planned |
| Authentication redesign | virtual links reuse the transit area's existing key-chain via the existing `authStore`; no new auth machinery |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -->
- [ ] `docs/research/ospf-implementation-guide.md` §6f "Virtual link" (~455) and §7 "Network Types" virtual-link row (~480, ~491) -- the model and the routed-IP requirement
  → Decision: model a virtual link as a synthetic point-to-point interface bound to Area 0.0.0.0 whose packets traverse the transit area as normal routed IPv4 (TTL > 1), never as a TTL-1 link-local exchange
  → Constraint: virtual links inherit authentication from the **transit** area, not from the backbone (~491); reuse the transit area's key-chain, do not invent a virtual-area key
- [ ] `docs/research/ospf-implementation-guide.md` §6a SPF link-type table (~384) -- "Router-LSA link type 4 (virtual link): same as type 1 but through the transit area"
  → Constraint: SPF treats the Type-4 link exactly like a Type-1 P2P link (already true in `spf.go`); the new work is *originating* the link record and feeding the cost/next hop from the transit-area SPF, not changing the graph walk
- [ ] `docs/research/ospf-implementation-guide.md` §13 "Implementation Roadmap" virtual-links note (~769, ~1610, ~1843) -- virtual links are a SHOULD item layered after the core is interop-green
  → Constraint: the feature is additive; a router with no `virtual-link` config must behave byte-for-byte as today (the V-bit stays clear, no Type-4 link is originated, no §16.3 pass runs unless TransitCapability is set)
- [ ] `plan/spec-ospf-0-umbrella.md` "Shared Contracts" (Router-LSA link records, LSDB key, SPF route table, area/interface model) -- the contracts this feature extends
  → Constraint: the LSDB key triple and the Router-LSA wire body are unchanged; the virtual link adds one Type-4 link record to the existing backbone Router-LSA, no new LSA type
  → Decision: the virtual neighbour adjacency reuses the existing neighbour state machine and LSDB flooding; only the transport path (unicast, TTL > 1) and the cost/next-hop source differ from a physical p2p interface
- [ ] `ai/rules/plugin-self-containment.md` -- the feature stays inside the OSPF plugin
  → Constraint: virtual-link config, schema, validators, show command, doctor checks, and metrics all live under `internal/plugins/ospf/`; no virtual-link spelling leaks into generic/central packages
- [ ] `ai/rules/no-sprintf-alloc.md` / `ai/rules/buffer-first.md` -- rendering and packet build
  → Constraint: `show ip ospf virtual-links` renders via `textbuf`/`AppendTo`; the Type-4 link record is emitted through the existing buffer-first Router-LSA `WriteTo` path, not string concatenation

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc2328.md` §15 Virtual Links / §C.4 -- the feature spec
  → Constraint: §15 -- a virtual link is an unnumbered point-to-point link belonging to the backbone, configured in BOTH endpoint ABRs by (other endpoint Router ID, transit area); its output cost and IP interface address are set DYNAMICALLY during the §16 SPF build; Router Priority is unused; the transit area MUST NOT be a stub
  → Constraint: §A.3.2 -- a Hello sent on a virtual link carries Network Mask = 0.0.0.0; §A.3.3 -- a DD packet sent over a virtual link carries Interface MTU = 0
  → Constraint: §A.4.2 -- the Router-LSA V-bit is set iff the router is an endpoint of a fully adjacent virtual link with THIS area (the backbone) as transit endpoint; the virtual link is encoded as a router-link of Type 4 with Link ID = the virtual neighbour's Router ID and Link Data = the router's own interface address used to reach it
  → Constraint: §16.1 -- the virtual neighbour's reachability, cost, and next hop come from the transit area's intra-area shortest-path tree (the calculating router roots the tree in the transit area, finds the virtual neighbour router vertex); a virtual link whose neighbour is not reachable in the transit area is DOWN
  → Constraint: §16.3 -- ABRs attached to a transit area (TransitCapability TRUE, set when a Type-1 in the area has the V-bit) run a second pass over the transit area's Summary-LSAs that only IMPROVES already-reachable backbone routes (never makes new destinations reachable) and resolves real next hops for destinations whose next hop was a virtual link; afterward, any destination still pointing at an unresolved virtual next hop is discarded
  → Constraint: §8.1 / IP encapsulation -- virtual-link packets are unicast and routed across the transit area (TTL large enough to reach the far end), in contrast to the TTL-1 link-local OSPF exchanges on physical interfaces

**Key insights:**
- The codec work is already done: `RouterFlagV` and `RouterLinkTypeVirtual` exist, the Router-LSA round-trips them, and `OriginInput.VirtualLinkEndpoint` already flips the V-bit. The gap is *activation*: config, the synthetic interface, the cost/next-hop source (transit-area SPF), the Type-4 link emission in `routerLinks`, and the §16.3 transit pass.
- A virtual link is "a p2p interface in Area 0 whose cost and next hop are computed, not configured, and whose packets are routed (TTL > 1) rather than link-local (TTL 1)." The neighbour state machine, DD exchange, flooding, and SPF graph walk are all reused unchanged.
- The single hardest piece is the §16.3 transit-area calculation (TransitCapability) plus the routed-IP send path (the existing TX socket pins `IP_TTL = 1`), because both are genuinely absent today.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
- [ ] `internal/plugins/ospf/packet/lsa_router.go` -- defines `RouterFlagV = 0x04`, `RouterLinkTypeVirtual = 4`; `RouterLSA.WriteTo`/`DecodeRouterLSA` round-trip the V-bit and Type-4 link record verbatim
  → Constraint: the codec is complete for virtual links; do NOT touch the wire format -- the work is feeding it correct values
- [ ] `internal/plugins/ospf/lsdb/origination.go` -- `OriginInput.VirtualLinkEndpoint bool` already sets `flags |= packet.RouterFlagV` in `OriginateRouter`; `routerLinks(in)` emits only P2P (from `NetworkPointToPoint` Full neighbours), Transit (broadcast DR), and Stub records -- it never emits a Type-4 virtual record
  → Constraint: `OriginInput` is the seam: add the virtual-link records to `Interfaces`/a new field so `routerLinks` emits Type-4 records and `VirtualLinkEndpoint` is set when any virtual link is Full; backbone Router-LSA only
- [ ] `internal/plugins/ospf/spf/spf.go` -- `transitEdges` (line ~183), `twoWayRouterLink` (line ~266), `p2pNeighborAddress` (line ~332) already match `RouterLinkTypeVirtual` alongside `RouterLinkTypeP2P`; the virtual neighbour becomes a reachable router vertex with the Type-4 link's metric and next hop the moment the link record exists
  → Constraint: the intra-area graph walk needs NO change; the §16.3 transit pass is a NEW stage, not an edit to `transitEdges`
- [ ] `internal/plugins/ospf/spf/interarea.go` -- `IsABR` carries the comment "Virtual-link backbone repair is out of scope"; the inter-area pass computes IA routes from backbone Summary-LSAs but has no §16.3 transit-area pass and no TransitCapability tracking
  → Constraint: add the §16.3 transit-area Summary-LSA pass here (or a sibling `transitarea.go`); update the comment; TransitCapability per area = a Type-1 Router-LSA in that area has the V-bit
- [ ] `internal/plugins/ospf/spf/computer.go` -- `Computer` runs per-area intra-area SPF then the inter-area pass; holds `areas`, `areaOptions`, `areaRanges`, route table, `onChange`; `Run` produces a `RouteDelta`
  → Constraint: the virtual-neighbour resolution (reachability/cost/next hop) is a read of the transit area's `Result` after intra-area SPF; expose it so the engine can drive the synthetic interface; the §16.3 pass runs after inter-area, before install
- [ ] `internal/plugins/ospf/config.go` -- `ospfConfig`/`areaConfig`/`interfaceConfig` resolution; `applyTree` parses `areas`, `interfaces`, `key-chains`, etc.; there is NO virtual-link parsing and no virtual-link config type
  → Constraint: add a `virtualLinkConfig` type and a `VirtualLinks []virtualLinkConfig` field on `ospfConfig`, parsed from a new `virtual-link` keyed list; validate transit area exists and is not a stub, and that the router is an ABR
- [ ] `internal/plugins/ospf/instance.go` -- `engine` owns `interfaces map[string]*ospfiface.Interface`, `neighbors`, `lsdb`, `spf`; `openInterface`/`reconcile` open physical interfaces; `lsdbTopology()` builds `InterfaceInfo` from running physical interfaces; `originateSelfLSAs()` calls `OriginateFromTopology`
  → Constraint: a virtual link is a synthetic interface the engine creates/destroys on reachability change, not a netlink interface; its `InterfaceInfo`-equivalent feeds the backbone Router-LSA; the engine drives its lifecycle from SPF results
- [ ] `internal/plugins/ospf/iface/iface.go` -- the `Interface` state machine; `Config` has `NetworkType`, `NetworkMask`, `InterfaceAddress`, `InterfaceMTU`, timers; `Sender.SendPacket(name, dst, payload)` is the send seam; p2p networks skip DR election and form full adjacency
  → Constraint: a virtual link is `NetworkType = "point-to-point"` with `NetworkMask = 0.0.0.0` and a Sender that routes (TTL > 1); the interface machine is reused, only the Config and the send path differ
- [ ] `internal/plugins/ospf/transport/transport.go` + `backend_linux.go` -- `SendPacket(name, dst, payload)` sends to a unicast `dst`; the TX socket sets `IP_TTL = 1` (line ~215) for link-local OSPF
  → Constraint: virtual-link packets MUST go out with a routed TTL (> 1). The current TX socket pins TTL 1, so the spec needs a virtual-link send path: a separate routed-IP socket (or a per-send TTL override) bound to the egress transit interface toward the virtual neighbour's transit address
- [ ] `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- the `ospf` container; no `virtual-link` list
  → Constraint: add a `virtual-link` list keyed by transit area + neighbour Router ID with native range/pattern constraints and a custom validator for the not-a-stub / ABR rule
- [ ] `internal/plugins/ospf/yang/ze-ospf-cmd.yang` + `cmd_show.go` -- the `show ip ospf database router|network|...` subtree; no `virtual-links` show command
  → Constraint: add `show ip ospf virtual-links`; surface the V-bit and Type-4 link in the existing `show ip ospf database router` rendering

**Behavior to preserve:**
- The Router-LSA wire format and the LSDB key triple; physical-interface origination, flooding, and intra-area SPF; the inter-area pass (§16.2); the existing OSPFv2 functional and interop tests.
- A router with no `virtual-link` config: the V-bit stays clear, no Type-4 record is originated, no §16.3 pass runs, the TTL-1 link-local send path is unchanged.
- The neighbour state machine, DD exchange (only the MTU=0 / Mask=0 fields differ on virtual links), and the `authStore` (virtual links reuse the transit area's key-chain).

**Behavior to change:** (all RFC-2328-§15-required, not discretionary)
- `routerLinks` / `OriginInput`: emit a Type-4 link record and set the V-bit when a virtual link to the backbone is Full.
- SPF: add the §16.3 transit-area Summary-LSA pass and TransitCapability; expose virtual-neighbour reachability/cost/next hop from the transit-area intra-area result.
- Transport: add a routed-IP (TTL > 1) unicast send path for virtual-link packets.
- DD/Hello on a virtual link: Interface MTU = 0, Network Mask = 0.0.0.0.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Config:** a `virtual-link` entry under `ospf` (transit area + virtual-neighbour Router ID + optional timers) -> `parseOSPFConfig` -> `ospfConfig.VirtualLinks`.
- **SPF result:** the transit area's intra-area SPF (§16.1) yields whether the virtual neighbour is reachable, its cost, and its next hop -> drives the synthetic interface up/down and the link cost.
- **Reception:** OSPF packets from the virtual neighbour arrive (unicast, routed) on the transit egress interface and are demultiplexed to the synthetic virtual interface by source Router ID + Area 0.0.0.0.

### Transformation Path
1. **Config resolve (new):** `applyTree` parses `virtual-link`; `validateConfig` rejects a transit area that is a stub or absent, and rejects virtual links on a non-ABR.
2. **Transit-area SPF read (new):** after the transit area's intra-area SPF, the engine reads each virtual neighbour's `(reachable, cost, nextHop, localTransitAddr)` from the transit-area `Result`.
3. **Synthetic interface lifecycle (new):** on `reachable` true, the engine creates/keeps a synthetic backbone p2p `Interface` (NetworkType p2p, Mask 0.0.0.0, MTU 0, timers from config) whose Sender routes (TTL > 1) to the virtual neighbour's transit address; on `reachable` false, it tears the interface (and its adjacency) down.
4. **Adjacency (reused):** the synthetic interface runs the p2p neighbour state machine; the adjacency reaches Full via the normal Hello/DD/LSReq/flooding path over routed IP.
5. **Router-LSA origination (changed):** `OriginateFromTopology` now includes virtual links in the backbone area's `OriginInput`; `routerLinks` emits a Type-4 record (LinkID = neighbour Router ID, LinkData = local transit address, Metric = transit cost) and sets `VirtualLinkEndpoint` (V-bit) when the link is Full.
6. **Backbone SPF (reused):** the backbone intra-area SPF graph walk already treats the Type-4 link like P2P; the virtual neighbour becomes a reachable backbone router vertex, repairing the backbone.
7. **§16.3 transit pass (new):** for each transit area with TransitCapability, a second pass over its Summary-LSAs improves already-reachable backbone routes and rewrites virtual-link next hops to the real transit next hop; unresolved virtual next hops are discarded.
8. **Install (reused):** the route delta installs through the existing Loc-RIB seam.
9. **Show (new):** `show ip ospf virtual-links` reads the engine's virtual-link state.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config <-> engine | new `virtual-link` list -> `ospfConfig.VirtualLinks`; reconcile creates/destroys synthetic interfaces | [ ] |
| Transit-area SPF <-> virtual link | read `(reachable, cost, nextHop, localAddr)` from the transit-area intra-area `Result` (read-only) | [ ] |
| Engine <-> synthetic interface | a backbone p2p `Interface` with a routing (TTL>1) Sender bound to the transit egress | [ ] |
| Synthetic interface <-> neighbour/LSDB | reuses the neighbour state machine, DD (MTU 0), and flooding unchanged | [ ] |
| Virtual link <-> Router-LSA | `OriginInput` virtual records -> `routerLinks` Type-4 emission + V-bit; backbone Router-LSA only | [ ] |
| Backbone SPF <-> §16.3 transit pass | new transit-area Summary-LSA pass + TransitCapability; rewrites/discards virtual next hops | [ ] |
| Transport <-> routed IP | a routed (TTL>1) unicast send path distinct from the TTL-1 per-interface socket | [ ] |

### Integration Points
- `internal/plugins/ospf/config.go` -- `virtualLinkConfig`, `ospfConfig.VirtualLinks`, parse + validate.
- `internal/plugins/ospf/lsdb/origination.go` -- `OriginInput` virtual-link records; `routerLinks` Type-4 emission; `VirtualLinkEndpoint` set from Full state (codec already supports it).
- `internal/plugins/ospf/spf/` -- the §16.3 transit pass + TransitCapability (`transitarea.go` or extend `interarea.go`); the virtual-neighbour resolution exposed from the transit-area `Result`.
- `internal/plugins/ospf/iface` -- reused for the synthetic interface (Config with Mask 0.0.0.0, MTU 0, p2p).
- `internal/plugins/ospf/neighbor` -- reused unchanged (the virtual neighbour is a normal p2p neighbour).
- `internal/plugins/ospf/transport` -- a routed (TTL>1) virtual-link send path.
- `internal/plugins/ospf/instance.go` -- the synthetic-interface lifecycle driven by SPF results; backbone Router-LSA origination input.
- `internal/plugins/ospf/cmd_show.go` -- `show ip ospf virtual-links`; V-bit/Type-4 in `show ip ospf database router`.

### Architectural Verification
- [ ] No bypassed layers (virtual-link packets flow config -> synthetic interface -> neighbour SM -> LSDB/flooding -> SPF, the same spine as a physical p2p interface, only the cost/next-hop source and the TTL differ)
- [ ] No unintended coupling (virtual links read the transit-area SPF result read-only; the transit area is unaware of the backbone repair)
- [ ] No duplicated functionality (reuses the codec V-bit/Type-4, the neighbour state machine, DD, flooding, and the intra-area SPF graph walk; adds only config, the synthetic interface, the routed send path, the Type-4 emission, and the §16.3 pass)
- [ ] Zero-copy / buffer-first preserved (Type-4 record emitted through the existing Router-LSA `WriteTo`; show output via `textbuf`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The codec already round-trips the V-bit and the Type-4 virtual link record, so no wire-format work is needed | `packet/lsa_router.go` `RouterFlagV`, `RouterLinkTypeVirtual`; `RouterLSA.WriteTo`/`Decode` | wire work + scope creep | `TestRouterLSAVirtualLinkRoundTrip` round-trips a V-bit + Type-4 record byte-for-byte | unvalidated |
| A-2 | The intra-area SPF graph walk reaches a virtual neighbour as a router vertex with no change, because `transitEdges`/`twoWayRouterLink`/`p2pNeighborAddress` already match `RouterLinkTypeVirtual` | `spf/spf.go` lines ~183/266/332 | the graph walk needs a virtual-link branch | `TestBackboneSPFReachesVirtualNeighbor` once the Type-4 record is originated | unvalidated |
| A-3 | The transit area's intra-area SPF `Result` already contains the virtual neighbour's cost and next hop (it is just another router vertex in that area), so resolution is a read, not a recompute | `spf/computer.go` per-area `Result`; `spf/route.go` node results | a separate transit-area shortest-path computation is needed | `TestVirtualNeighborResolvedFromTransitSPF` | unvalidated |
| A-4 | `OriginInput.VirtualLinkEndpoint` flipping the V-bit is correct and sufficient; only the Type-4 link emission in `routerLinks` is missing | `lsdb/origination.go` `OriginateRouter` already ORs `RouterFlagV` | the V-bit semantics differ (e.g. per-area) | `TestBackboneRouterLSAHasVBitWhenVLFull` | unvalidated |
| A-5 | A virtual link can reuse the neighbour state machine and DD/flooding unchanged by presenting as a p2p interface (Mask 0.0.0.0, MTU 0) | `iface/iface.go` p2p path; §A.3.2/§A.3.3 | the neighbour/DD code needs virtual-link special cases beyond Mask/MTU | `TestVirtualLinkAdjacencyReachesFull` (synthetic transport) | unvalidated |
| A-6 | A routed (TTL>1) unicast send path can be added without disturbing the TTL-1 link-local physical-interface path | `transport/backend_linux.go` sets `IP_TTL = 1` on the shared TX socket | the two paths conflict; a deeper transport refactor | `TestVirtualLinkSendUsesRoutedTTL` (the virtual-link send path does not pin TTL 1) | unvalidated |
| A-7 | The §16.3 transit pass only improves already-reachable routes and resolves virtual next hops; it never makes new destinations reachable | RFC 2328 §16.3; guide §6f | over-broad route changes / loops | `TestTransitAreaPassOnlyImprovesReachable`, `TestVirtualNextHopResolvedOrDiscarded` | unvalidated |
| A-8 | The transit area for a virtual link must not be a stub and the configuring router must be an ABR; these are config-time rules | RFC 2328 §15; `config.go` validate path | invalid configs accepted; runtime breakage | `TestVirtualLinkRejectStubTransit`, `TestVirtualLinkRejectNonABR` | unvalidated |
| A-9 | Virtual links inherit the transit area's authentication via the existing `authStore`, with no new key surface | guide §7 (~491); `instance.go` `authStore` per area | a separate virtual-link key surface is needed | `TestVirtualLinkUsesTransitAreaAuth` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The TTL-1 send socket drops virtual-link packets (they need to be routed across the transit area), so the adjacency never forms | the virtual neighbour stays in ExStart/Down; FRR shows no virtual-link Hello | dedicated routed (TTL>1) send path; `TestVirtualLinkSendUsesRoutedTTL` + `ospf-virtual-link-frr` interop forms Full |
| R-2 | The §16.3 transit pass makes new destinations reachable or creates a routing loop (violates "improve only") | a destination reachable only via the transit pass; traffic loops in the transit area | strictly gate the pass to already-reachable backbone routes; `TestTransitAreaPassOnlyImprovesReachable` |
| R-3 | A destination whose next hop is a virtual link is installed with the synthetic (unroutable) virtual next hop instead of the real transit next hop | the FIB has a route pointing at the virtual neighbour Router ID, not a real address | resolve the real next hop in the §16.3 pass; discard unresolved virtual next hops; `TestVirtualNextHopResolvedOrDiscarded` |
| R-4 | The Type-4 record is originated into a non-backbone Router-LSA (it must be backbone-only) | the V-bit / Type-4 appears in a transit-area Router-LSA; FRR rejects | emit virtual records only into the Area 0.0.0.0 `OriginInput`; `TestVirtualLinkBackboneOnly` |
| R-5 | The virtual link flaps when the transit-area cost changes, churning the backbone Router-LSA and SPF | frequent Router-LSA reoriginations; SPF storms | drive the synthetic interface from settled SPF results; reuse the existing SPF throttle and MinLSInterval; `TestVirtualLinkCostUpdateNoFlap` |
| R-6 | Cost/next-hop staleness: the virtual link uses an old transit cost after the transit topology changes | the backbone Router-LSA metric lags the transit cost | recompute the virtual-neighbour resolution every SPF run before backbone origination; `TestVirtualLinkCostTracksTransitTopology` |
| R-7 | A virtual link configured to an unreachable / non-existent neighbour wedges or busy-loops | the synthetic interface churns up/down | only create the synthetic interface when the neighbour is a reachable transit router vertex; keep it down otherwise; `TestVirtualLinkNeighborUnreachableStaysDown` |
| R-8 | Demux ambiguity: a routed packet from the virtual neighbour is matched to the wrong (physical) interface | the adjacency forms on the wrong interface or not at all | demux virtual-link packets by source Router ID + backbone Area ID to the synthetic interface; `TestVirtualLinkPacketDemux` |
| R-9 | Two virtual links share a transit egress and collide on the routed socket / next hop | one virtual link's packets are sent toward the other neighbour | key the synthetic interface and send path by (transit area, neighbour); `TestTwoVirtualLinksSameTransit` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| A `virtual-link` config entry under `ospf` | -> | `parseOSPFConfig` resolves `ospfConfig.VirtualLinks`; the engine reconcile creates a synthetic interface when the neighbour is reachable | `TestParseVirtualLinkConfig` (unit) + `test/ospf/ospf-virtual-link-config.ci` |
| The transit area's SPF settles with the virtual neighbour reachable | -> | the engine resolves cost/next hop and brings the synthetic backbone interface up | `TestVirtualNeighborResolvedFromTransitSPF` (unit) + `test/ospf/ospf-virtual-link-up.ci` |
| The virtual-link adjacency reaches Full | -> | `OriginateFromTopology` emits a Type-4 record into the backbone Router-LSA and sets the V-bit | `TestBackboneRouterLSAHasVBitWhenVLFull` (unit) + observed in `ospf-virtual-link-frr` interop |
| The backbone SPF runs with the Type-4 link present | -> | the virtual neighbour becomes a reachable backbone vertex; the §16.3 transit pass resolves virtual next hops | `TestBackboneSPFReachesVirtualNeighbor`, `TestVirtualNextHopResolvedOrDiscarded` (unit) + `test/ospf/ospf-virtual-link-route.ci` |
| `show ip ospf virtual-links` | -> | the engine renders virtual-link state (transit area, neighbour, state, cost, next hop) | `test/ospf/ospf-virtual-link-show.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A `virtual-link` entry naming a transit area and a virtual-neighbour Router ID | parsed into `ospfConfig.VirtualLinks`; optional hello/dead/retransmit/transmit-delay timers resolved with documented defaults |
| AC-2 | A `virtual-link` whose transit area is a stub area | rejected at config validation (RFC 2328 §15: a virtual link cannot run through a stub) |
| AC-3 | A `virtual-link` on a router that is not an ABR (single area, or transit area absent) | rejected at config validation |
| AC-4 | The virtual neighbour is reachable as a router vertex in the transit area's intra-area SPF | the virtual link's cost = the transit-area intra-area cost and its next hop = the transit-area next hop toward the neighbour; the synthetic backbone interface is brought up |
| AC-5 | The virtual neighbour is not reachable in the transit area | the virtual link stays Down; no Type-4 record is originated; no adjacency is attempted |
| AC-6 | The virtual-link adjacency over the transit area | reaches Full via the p2p neighbour state machine; Hellos carry Network Mask 0.0.0.0 and DD packets carry Interface MTU 0 (§A.3.2/§A.3.3); packets are unicast and routed (TTL > 1) to the neighbour's transit address |
| AC-7 | A virtual link is Full | the endpoint's **backbone** Router-LSA contains a Type-4 link record (LinkID = neighbour Router ID, LinkData = local transit address, Metric = transit cost) and the Router-LSA V-bit is set; no virtual record or V-bit appears in any non-backbone Router-LSA |
| AC-8 | The backbone Router-LSA now contains the Type-4 link | the backbone intra-area SPF reaches the virtual neighbour as a router vertex (two-way check honoured via the reciprocal Type-4 record), repairing a partitioned backbone or attaching the remote area to Area 0 |
| AC-9 | A transit area has a router whose Router-LSA has the V-bit (TransitCapability TRUE) | the §16.3 transit-area Summary-LSA pass runs for that area: it only improves already-reachable backbone intra/inter-area routes and never makes a new destination reachable |
| AC-10 | A destination whose §16 next hop is a virtual link | the §16.3 pass rewrites it to the real transit next hop; if it cannot be resolved, the destination is discarded (not installed with a virtual next hop) |
| AC-11 | The transit-area topology changes (cost to the virtual neighbour changes) | the next SPF run updates the virtual link's cost, reoriginates the backbone Router-LSA Type-4 metric, and recomputes backbone routes; no flap when the cost is unchanged |
| AC-12 | `show ip ospf virtual-links` | lists each configured virtual link with transit area, virtual-neighbour Router ID, adjacency state, computed cost, and next hop |
| AC-13 | No `virtual-link` is configured | OSPFv2 behaves byte-for-byte as today: V-bit clear, no Type-4 record, no §16.3 pass, TTL-1 link-local send path unchanged |
| AC-14 | A virtual link's authentication | uses the transit area's configured key-chain (the existing `authStore`), not a separate virtual-area key |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures a virtual link through a transit area to attach a remote area to Area 0 | config -> `ospfConfig.VirtualLinks` -> transit-area SPF resolves the neighbour -> synthetic interface up -> adjacency Full -> Type-4 in backbone Router-LSA -> backbone SPF reaches the neighbour | `test/ospf/ospf-virtual-link-up.ci` + `ospf-virtual-link-frr` interop |
| 2 | Forms a virtual-link adjacency with FRR across a transit area | unicast routed Hello/DD/LSU over the transit area -> p2p neighbour SM -> Full; FRR's `show ip ospf virtual-links` shows the link up | `ospf-virtual-link-frr` interop |
| 3 | Inspects the virtual link | CLI -> `show ip ospf virtual-links` -> engine state (transit area, neighbour, state, cost, next hop); `show ip ospf database router` shows the V-bit + Type-4 link | `test/ospf/ospf-virtual-link-show.ci` |
| 4 | Repairs a partitioned backbone | two backbone fragments joined only through a transit area -> virtual link Full -> Type-4 records both ways -> backbone SPF treats the fragments as connected; a destination in one fragment is reachable from the other | `test/ospf/ospf-virtual-link-route.ci` + `ospf-virtual-link-frr` interop |
| 5 | Removes the virtual-link config | reconcile tears down the synthetic interface and adjacency; the backbone Router-LSA loses the Type-4 record and the V-bit; OSPF otherwise unchanged | `TestVirtualLinkRemovalWithdrawsTypeFour` + existing OSPF suite still green |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseVirtualLinkConfig` | `internal/plugins/ospf/config_test.go` | AC-1: `virtual-link` list parsed into `VirtualLinks` with timer defaults | |
| `TestVirtualLinkRejectStubTransit` | `internal/plugins/ospf/config_test.go` | AC-2, A-8: a stub transit area is rejected | |
| `TestVirtualLinkRejectNonABR` | `internal/plugins/ospf/config_test.go` | AC-3, A-8: a non-ABR / absent transit area is rejected | |
| `TestRouterLSAVirtualLinkRoundTrip` | `internal/plugins/ospf/packet/lsa_router_test.go` | A-1: V-bit + Type-4 record round-trip byte-for-byte | |
| `TestRouterLinksEmitsVirtualType4` | `internal/plugins/ospf/lsdb/origination_test.go` | AC-7: `routerLinks` emits a Type-4 record for a Full virtual link with the correct LinkID/LinkData/Metric | |
| `TestBackboneRouterLSAHasVBitWhenVLFull` | `internal/plugins/ospf/lsdb/origination_test.go` | AC-7, A-4: the backbone Router-LSA sets the V-bit; non-backbone Router-LSAs do not | |
| `TestVirtualLinkBackboneOnly` | `internal/plugins/ospf/lsdb/origination_test.go` | R-4: no Type-4 record or V-bit in any non-backbone Router-LSA | |
| `TestVirtualNeighborResolvedFromTransitSPF` | `internal/plugins/ospf/spf/transitarea_test.go` | AC-4, A-3: reachability/cost/next hop read from the transit-area intra-area `Result` | |
| `TestVirtualLinkNeighborUnreachableStaysDown` | `internal/plugins/ospf/spf/transitarea_test.go` | AC-5, R-7: an unreachable virtual neighbour yields a Down link, no Type-4 record | |
| `TestBackboneSPFReachesVirtualNeighbor` | `internal/plugins/ospf/spf/spf_test.go` | AC-8, A-2: the backbone graph reaches the virtual neighbour via the Type-4 link (two-way check honoured) | |
| `TestTransitCapabilitySetByVBit` | `internal/plugins/ospf/spf/transitarea_test.go` | AC-9: TransitCapability TRUE iff a Type-1 in the area has the V-bit | |
| `TestTransitAreaPassOnlyImprovesReachable` | `internal/plugins/ospf/spf/transitarea_test.go` | AC-9, R-2, A-7: the §16.3 pass improves only already-reachable routes, never makes new ones reachable | |
| `TestVirtualNextHopResolvedOrDiscarded` | `internal/plugins/ospf/spf/transitarea_test.go` | AC-10, R-3: a virtual next hop is rewritten to the real transit next hop or discarded | |
| `TestVirtualLinkCostTracksTransitTopology` | `internal/plugins/ospf/spf/transitarea_test.go` | AC-11, R-6: a transit-cost change updates the virtual-link cost | |
| `TestVirtualLinkCostUpdateNoFlap` | `internal/plugins/ospf/spf/transitarea_test.go` | R-5: an unchanged cost does not reoriginate the backbone Router-LSA | |
| `TestVirtualLinkAdjacencyReachesFull` | `internal/plugins/ospf/virtual_link_test.go` | AC-6, A-5: the synthetic p2p interface reaches Full; DD MTU 0, Hello Mask 0.0.0.0 | |
| `TestVirtualLinkSendUsesRoutedTTL` | `internal/plugins/ospf/transport/transport_test.go` | AC-6, A-6, R-1: the virtual-link send path uses a routed TTL (> 1), not the TTL-1 link-local socket | |
| `TestVirtualLinkPacketDemux` | `internal/plugins/ospf/virtual_link_test.go` | R-8: a routed packet from the virtual neighbour demuxes to the synthetic interface by source Router ID + backbone Area ID | |
| `TestTwoVirtualLinksSameTransit` | `internal/plugins/ospf/virtual_link_test.go` | R-9: two virtual links sharing a transit egress are keyed by (transit area, neighbour) and do not collide | |
| `TestVirtualLinkUsesTransitAreaAuth` | `internal/plugins/ospf/virtual_link_test.go` | AC-14, A-9: the virtual link signs/verifies with the transit area's key-chain | |
| `TestVirtualLinkRemovalWithdrawsTypeFour` | `internal/plugins/ospf/virtual_link_test.go` | story 5: removing config tears down the interface and withdraws the Type-4 record + V-bit | |
| `TestNoVirtualLinkBehaviorUnchanged` | `internal/plugins/ospf/virtual_link_test.go` | AC-13: with no virtual link configured, V-bit clear, no Type-4, no §16.3 pass | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| hello-interval (seconds) | 1-65535 | 65535 | 0 | N/A (uint16) |
| dead-interval (seconds) | 1-65535 | 65535 | 0 | N/A (uint16) |
| retransmit-interval (seconds) | 1-65535 | 65535 | 0 | N/A (uint16) |
| transmit-delay (InfTransDelay, seconds) | 1-65535 | 65535 | 0 (RFC 2328 App C.3: must be > 0) | N/A (uint16) |
| virtual-link cost (computed from transit SPF) | 1-65534 | 65534 | N/A (>= 1 by SPF) | LSInfinity (0xffff) means unreachable -> link Down |
| transit area-id | 0.0.0.1-255.255.255.255 | any non-zero | 0.0.0.0 (backbone cannot be a transit area) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-virtual-link-config` | `test/ospf/ospf-virtual-link-config.ci` | a `virtual-link` entry parses and shows in `show ip ospf`; stub-transit/non-ABR configs are rejected | |
| `ospf-virtual-link-up` | `test/ospf/ospf-virtual-link-up.ci` | the virtual neighbour is resolved over the transit area and the link comes up to Full | |
| `ospf-virtual-link-route` | `test/ospf/ospf-virtual-link-route.ci` | a destination reachable only across the repaired backbone is installed with the real transit next hop | |
| `ospf-virtual-link-show` | `test/ospf/ospf-virtual-link-show.ci` | `show ip ospf virtual-links` lists state/cost/next hop; `show ip ospf database router` shows the V-bit + Type-4 link | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-virtual-link-frr` | `test/interop/scenarios/ospf-virtual-link-frr/` | FRR `ospfd` (a 3-area topology: Area 0 fragment, transit Area, remote Area 0 fragment / remote area; FRR configures the matching `area <transit> virtual-link <ze-rid>`) | Ze forms a Full virtual-link adjacency with FRR over the transit area (routed IP), originates the V-bit + Type-4 record into its backbone Router-LSA, repairs the backbone so a destination in one fragment is reachable from the other, and FRR's `show ip ospf virtual-links` shows the link up | |

> Interop is required: this adds wire behaviour (virtual-link Hello Mask 0,
> DD MTU 0, routed-IP exchange, V-bit + Type-4 link record). The raw-IP / routed
> transit path is Linux-only and runs as a QEMU integration test
> (`ai/rules/qemu-testing.md`), consistent with the rest of the OSPF interop set.

### Future (if deferring any tests)
- None. Every AC is covered by a unit, functional, or interop test above. OSPFv3 virtual links and NBMA are separate specs (out of scope), not deferred tests of this spec.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*) -->
- `internal/plugins/ospf/config.go` -- `virtualLinkConfig` type; `ospfConfig.VirtualLinks`; `parseVirtualLink`; `validateConfig` stub-transit / non-ABR rejection
- `internal/plugins/ospf/lsdb/origination.go` -- carry virtual-link records in `OriginInput`; emit a Type-4 record in `routerLinks`; set `VirtualLinkEndpoint` (V-bit) when Full; backbone Router-LSA only
- `internal/plugins/ospf/spf/interarea.go` -- update the `IsABR` "out of scope" comment; invoke the new §16.3 transit-area pass
- `internal/plugins/ospf/spf/computer.go` -- run the transit-area pass after inter-area; expose virtual-neighbour resolution from the transit-area `Result`
- `internal/plugins/ospf/instance.go` -- the synthetic virtual-interface lifecycle driven by SPF results; backbone `OriginInput` virtual records; virtual-link packet demux
- `internal/plugins/ospf/transport/transport.go` + `transport/backend_linux.go` -- a routed (TTL>1) unicast send path for virtual-link packets, distinct from the TTL-1 link-local socket
- `internal/plugins/ospf/cmd_show.go` -- `show ip ospf virtual-links`; V-bit + Type-4 link rendering in `show ip ospf database router`
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- the `virtual-link` list with native constraints
- `internal/plugins/ospf/yang/ze-ospf-cmd.yang` -- the `show ip ospf virtual-links` command
- `internal/plugins/ospf/register.go` -- register the new `ospf-virtual-link` config validator
- `internal/plugins/ospf/doctor.go` -- a doctor check only if a new runtime dependency is introduced (the routed send socket); add one if so

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- `virtual-link` list; read `ai/rules/config-surface.md` + `ai/rules/config-naming.md` |
| YANG validation constraints | [ ] yes | native `range`/`pattern` on area-id, neighbour Router ID, and the four timers; reject backbone (0.0.0.0) as transit area |
| YANG custom validators | [ ] yes | `ze:validate "ospf-virtual-link"` for the not-a-stub / ABR rule with `ValidateFn` + `CompleteFn`; register in `register.go` |
| CLI commands/flags | [ ] yes | `show ip ospf virtual-links` in `ze-ospf-cmd.yang` + `cmd_show.go` |
| CLI grammar (action before identifier) | [ ] yes | `ai/rules/cli-grammar.md` -- `show ip ospf virtual-links` |
| Editor autocomplete | [ ] yes | automatic for the YANG list + the new show subcommand; `CompleteFn` offers configured transit areas |
| Functional test for new RPC/API | [ ] yes | `test/ospf/ospf-virtual-link-*.ci` |
| Pipe completeness | [ ] yes | `show ip ospf virtual-links` routes through `ApplyPipes` like the other show outputs |
| Env var registration | [ ] no | virtual links are operational config, not an `environment/` leaf |
| Doctor check for runtime dependencies | [ ] yes | if a routed send socket is opened, add an owning-package doctor check + `internal/core/diagnostic/codes.go` entry per `ai/rules/doctor-checks.md` |
| Prometheus counters/metrics | [ ] yes | see the metrics rows below |

#### Metrics (new series owned by this spec)
| Metric | Type | Labels |
|--------|------|--------|
| `ze_ospf_virtual_links` | gauge | `transit_area`, `state` |
| `ze_ospf_virtual_link_cost` | gauge | `transit_area`, `neighbor` |
| `ze_ospf_virtual_link_adjacency_changes_total` | counter | `transit_area`, `neighbor` |
| `ze_ospf_transit_area_passes_total` | counter | `transit_area` |

> These extend the umbrella's canonical OSPF metric set; they use the
> `ze_ospf_virtual_*` / `ze_ospf_transit_*` prefixes and are registered by this
> spec's owner code. The umbrella "Metrics" table gains these rows when this spec lands.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` -- OSPFv2 virtual links |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` -- the `virtual-link` list |
| 3 | CLI command added/changed? | [ ] yes | `docs/guide/command-reference.md` -- `show ip ospf virtual-links` |
| 4 | API/RPC added/changed? | [ ] no | the show RPC lives in the central `ze-show` namespace; document under the command reference |
| 5 | Plugin added/changed? | [ ] yes | `docs/guide/plugins.md` -- OSPF gains virtual-link config + the synthetic interface |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/ospf.md` -- virtual-link section |
| 7 | Wire format changed? | [ ] yes | `docs/architecture/wire/ospf.md` (or equivalent) -- V-bit, Type-4 link, virtual-link Hello/DD field rules |
| 8 | Plugin SDK/protocol changed? | [ ] no | no SDK surface change |
| 9 | RFC behavior implemented? | [ ] yes | `rfc/short/rfc2328.md` -- flip the §15 / §16.3 / virtual-link compliance items to implemented |
| 10 | Test infrastructure changed? | [ ] yes (interop scenario added) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` -- OSPF virtual-link parity with FRR |
| 12 | Internal architecture changed? | [ ] yes | the OSPF subsystem doc -- the synthetic interface, the §16.3 transit pass, the routed send path |
| 13 | Route metadata keys added/changed? | [ ] no | virtual links resolve to existing route entries (no new metadata key) |
| 14 | Prometheus counters added/changed? | [ ] yes | the OSPF telemetry doc -- the four new series |
| 15 | Registered plugin/event/command/capability inventory changed? | [ ] yes | `docs/plugin-overview.md` + the umbrella metrics table (new show command, new validator) |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors into `config.go`, `origination.go`, `spf/interarea.go`, `transport.go` |
| 17 | Existing docs show examples for this area? | [ ] check | verify any OSPF config/CLI examples still parse with the new `virtual-link` list present |

## Files to Create
- `internal/plugins/ospf/virtual_link.go` -- the synthetic virtual-interface lifecycle, the routed Sender, packet demux, and the engine glue
- `internal/plugins/ospf/virtual_link_test.go` -- adjacency, demux, two-links, auth, removal, no-config-unchanged tests
- `internal/plugins/ospf/spf/transitarea.go` -- the §16.3 transit-area Summary-LSA pass, TransitCapability, and the virtual-neighbour resolution from the transit-area `Result`
- `internal/plugins/ospf/spf/transitarea_test.go` -- resolution, TransitCapability, improve-only, next-hop-resolve-or-discard, cost-tracking/no-flap tests
- `test/ospf/ospf-virtual-link-config.ci`, `ospf-virtual-link-up.ci`, `ospf-virtual-link-route.ci`, `ospf-virtual-link-show.ci`
- `test/interop/scenarios/ospf-virtual-link-frr/` -- `ze.conf`, `frr.conf`, `check.py`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan -- confirm the codec V-bit/Type-4 and the SPF virtual-link graph match already exist |
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

1. **Phase: Wiring (MANDATORY FIRST)** -- config surface + synthetic-interface skeleton + failing wiring tests
   - Tests: `TestParseVirtualLinkConfig`, `TestVirtualLinkRejectStubTransit`, `TestVirtualLinkRejectNonABR`, `test/ospf/ospf-virtual-link-config.ci`
   - Files: `config.go` (`virtualLinkConfig`, `VirtualLinks`, parse + validate), `yang/ze-ospf-conf.yang` (`virtual-link` list), `register.go` (validator), `virtual_link.go` (engine reconcile creates a stub synthetic interface)
   - Verify: the config resolves and is validated; the synthetic interface is created on reachability but the adjacency/origination are stubs, so deeper tests still fail
2. **Phase: Transit-area resolution + §16.3 pass** -- the SPF side
   - Tests: `TestVirtualNeighborResolvedFromTransitSPF`, `TestVirtualLinkNeighborUnreachableStaysDown`, `TestTransitCapabilitySetByVBit`, `TestTransitAreaPassOnlyImprovesReachable`, `TestVirtualNextHopResolvedOrDiscarded`, `TestVirtualLinkCostTracksTransitTopology`, `TestVirtualLinkCostUpdateNoFlap`
   - Files: `spf/transitarea.go`, `spf/computer.go`, `spf/interarea.go`
   - Verify: reachability/cost/next hop read from the transit-area result; the §16.3 pass improves only reachable routes and resolves/discards virtual next hops; cost tracks topology without flapping
3. **Phase: Router-LSA Type-4 origination** -- the backbone advertisement
   - Tests: `TestRouterLSAVirtualLinkRoundTrip`, `TestRouterLinksEmitsVirtualType4`, `TestBackboneRouterLSAHasVBitWhenVLFull`, `TestVirtualLinkBackboneOnly`, `TestBackboneSPFReachesVirtualNeighbor`
   - Files: `lsdb/origination.go`, `instance.go` (backbone `OriginInput` virtual records)
   - Verify: a Full virtual link emits a Type-4 record + V-bit into the backbone Router-LSA only; the backbone SPF reaches the virtual neighbour
4. **Phase: Synthetic interface + routed send path + adjacency** -- the transport/neighbour side
   - Tests: `TestVirtualLinkAdjacencyReachesFull`, `TestVirtualLinkSendUsesRoutedTTL`, `TestVirtualLinkPacketDemux`, `TestTwoVirtualLinksSameTransit`, `TestVirtualLinkUsesTransitAreaAuth`, `TestVirtualLinkRemovalWithdrawsTypeFour`, `TestNoVirtualLinkBehaviorUnchanged`
   - Files: `virtual_link.go`, `transport/transport.go`, `transport/backend_linux.go`
   - Verify: the synthetic p2p interface reaches Full over routed IP (TTL>1, Mask 0, MTU 0); packets demux correctly; two links coexist; auth inherits from the transit area; removal withdraws the Type-4 record; no-config is unchanged
5. **Phase: CLI + metrics + doctor** -- user surface
   - Tests: `ospf-virtual-link-show.ci`, `ospf-virtual-link-up.ci`, `ospf-virtual-link-route.ci`
   - Files: `cmd_show.go`, `yang/ze-ospf-cmd.yang`, metric registration, `doctor.go` (if a routed socket is introduced)
   - Verify: `show ip ospf virtual-links`, the V-bit/Type-4 in `show ip ospf database router`, the four metric series, the doctor check
6. **Functional tests** -> the four `.ci` cover the user-visible behaviour
7. **RFC refs** -> add `// RFC 2328 Section 15 / 16.3 / A.4.2` comments on the origination, transit pass, and routed-send code
8. **Interop** -> `ospf-virtual-link-frr` QEMU scenario
9. **Full verification** -> `make ze-verify`
10. **Complete spec** -> audit tables + learned summary; two commits (A: code+spec+learned, B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has file:line implementation |
| Feature completeness | each user story has a working path; virtual-link parity with FRR (backbone repair + remote-area attach + `show ip ospf virtual-links`) |
| Correctness | cost = transit intra-area cost; next hop from transit SPF; V-bit + Type-4 backbone-only; §16.3 improves-only and resolves/discards virtual next hops; Hello Mask 0, DD MTU 0; routed TTL>1 |
| Naming | `ze_ospf_virtual_*` / `ze_ospf_transit_*` metrics; YANG `virtual-link` kebab-case; show command `virtual-links` |
| Data flow | virtual links read the transit-area SPF result read-only; backbone Router-LSA is the only origination point; no virtual-link spelling in generic packages |
| CLI grammar | `show ip ospf virtual-links` action-before-identifier |
| Doctor checks | a routed-send-socket dependency, if added, has a `ze doctor` check per `ai/rules/doctor-checks.md` |
| YANG validation | the `virtual-link` list has native range/pattern + a custom validator for stub-transit / ABR rules |
| Prometheus counters | the four series defined, registered, listed; umbrella table updated |
| Rule: plugin-self-containment | all virtual-link surfaces live under `internal/plugins/ospf/`; removing the config removes the feature |
| Rule: buffer-first | Type-4 record emitted via the Router-LSA `WriteTo`; show output via `textbuf` |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `virtual-link` config parses and validates | `go test ./internal/plugins/ospf -run TestParseVirtualLinkConfig` |
| Type-4 record + V-bit originated into the backbone Router-LSA | `go test ./internal/plugins/ospf/lsdb -run 'Virtual'` |
| §16.3 transit-area pass + resolution | `go test ./internal/plugins/ospf/spf -run 'Transit|Virtual'` |
| Routed (TTL>1) virtual-link send path | `go test ./internal/plugins/ospf/transport -run TestVirtualLinkSendUsesRoutedTTL` |
| Synthetic interface reaches Full | `go test ./internal/plugins/ospf -run TestVirtualLinkAdjacencyReachesFull` |
| Four metric series registered | `grep -rn 'ze_ospf_virtual_\|ze_ospf_transit_' internal/plugins/ospf` |
| Show command present | `grep -rn 'virtual-links\|ospf-virtual-link' internal/plugins/ospf` |
| Functional tests present | `ls test/ospf/ospf-virtual-link-*.ci` |
| Interop scenario present | `ls test/interop/scenarios/ospf-virtual-link-frr/` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | virtual-link config validated (transit area exists, not stub, router is ABR); a malformed virtual-link packet is bound-checked by the existing codec |
| Resource exhaustion | the number of synthetic interfaces is bounded by configured virtual links; a flapping transit topology cannot spawn unbounded interfaces or SPF runs (reuse the SPF throttle) |
| Trust boundary | virtual-link packets are routed (TTL>1) and could arrive from any host that can reach the transit address; they MUST pass the transit area's authentication before forming an adjacency (reuse `authStore`) |
| Routed exposure | the routed send socket only sends to configured virtual-neighbour transit addresses; it does not widen the receive surface beyond the existing per-interface receive path |
| Error leakage | virtual-link resolution failures (unreachable neighbour, unresolved next hop) are logged/counted, not surfaced to peers |

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
A virtual link is not a new protocol mechanism: it is a point-to-point interface
in Area 0 whose cost and next hop are *computed from the transit area's
intra-area SPF* instead of configured, and whose packets are *routed* (TTL > 1)
instead of link-local (TTL 1). Ze's codec already encodes the V-bit and the
Type-4 link record, and the SPF graph walk already treats a Type-4 link like a
P2P link. The real work is activation: configuration, the transit-area-driven
resolution (cost/next hop/reachability), a synthetic backbone interface with a
routed send path, originating the Type-4 record into the backbone Router-LSA, and
the §16.3 transit-area pass that resolves destinations whose next hop is a
virtual link.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Model a virtual link as a synthetic p2p interface in Area 0 | a bespoke virtual-link state machine separate from interfaces | reuses the neighbour SM, DD, flooding, and the p2p path; only the cost/next-hop source and TTL differ (RFC 2328 §15) |
| Read cost/next hop from the transit-area intra-area SPF `Result` | a separate shortest-path computation just for virtual links | the virtual neighbour is already a router vertex in the transit area; a read avoids a second Dijkstra (§16.1) |
| A dedicated routed (TTL>1) send path | bumping the shared TX socket's TTL | the shared socket pins TTL 1 for link-local OSPF; bumping it would break physical-interface exchanges |
| Emit the Type-4 record only into the backbone Router-LSA | emit per area | RFC 2328 §A.4.2: the V-bit/Type-4 link belongs to the backbone (transit endpoint is Area 0) |
| Inherit auth from the transit area | a separate virtual-area key surface | guide §7 (~491): virtual links use the transit area's authentication |
| Implement §16.3 as a new transit pass gated by TransitCapability | fold it into the inter-area pass | §16.3 is a distinct, improve-only stage with its own discard-unresolved-virtual-next-hops semantics |

## Known Limitations
- OSPFv3 virtual links (RFC 5340) are out of scope; this spec is OSPFv2-only.
- NBMA networks and the NBMA neighbour list are spec-ospf-ext-8, deliberately excluded here.
- Virtual links cannot run through a stub area or be configured on a non-ABR (RFC 2328 §15); these are config-time rejections, not runtime fallbacks.
- The §16.3 transit pass only improves already-reachable backbone routes; it never makes a new destination reachable (RFC 2328 §16.3).

## RFC Documentation

Add `// RFC 2328 Section X.Y: "<quoted requirement>"` above the enforcing code:
- §15 virtual link belongs to the backbone, configured by (neighbour Router ID, transit area), cost/address set dynamically, transit area not a stub
- §A.3.2 Hello Network Mask 0.0.0.0 on a virtual link; §A.3.3 DD Interface MTU 0 on a virtual link
- §A.4.2 Router-LSA V-bit and Type-4 link record (LinkID = neighbour Router ID, LinkData = local interface address, Metric = transit cost)
- §16.1 virtual-neighbour reachability/cost/next hop from the transit-area shortest-path tree
- §16.3 transit-area Summary-LSA pass: improve-only, resolve/discard virtual next hops, TransitCapability

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
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Virtual-link config (transit area + virtual neighbour) | functional | `ospf-virtual-link-config.ci` |
| Virtual-neighbour adjacency over the transit area | interop | `ospf-virtual-link-frr` (Full adjacency over routed IP) |
| Type-1 Router-LSA virtual-link advertisement (V-bit + Type-4) | unit + interop | `TestBackboneRouterLSAHasVBitWhenVLFull`, `ospf-virtual-link-frr` |
| SPF integration (backbone repair + §16.3 resolution) | unit + functional | `TestBackboneSPFReachesVirtualNeighbor`, `TestVirtualNextHopResolvedOrDiscarded`, `ospf-virtual-link-route.ci` |

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
- [ ] AC-1..AC-14 all demonstrated
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
- [ ] RFC 2328 constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (the synthetic interface reuses the existing interface/neighbour machinery)
- [ ] No speculative features (only RFC 2328 §15/§16.3 virtual links; NBMA and OSPFv3 excluded)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (virtual links read the transit-area SPF result read-only)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (`ospf-virtual-link-frr`)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ospf-ext-7-virtual-links.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospf-ext-7-virtual-links.md`
