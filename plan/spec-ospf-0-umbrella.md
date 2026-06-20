# Spec: ospf-0 -- OSPFv2 Link-State IGP (Umbrella)

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-06-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `docs/research/ospf-implementation-guide.md` -- clean-room OSPF protocol + architecture guide (1900 lines): packet/LSA codec, ISM/NSM, DR election, flooding (§13), SPF (§16), areas/ABR/ASBR, NSSA, auth, concurrency model, phased order, ze package layout
4. `plan/learned/926-isis-0-umbrella.md` -- the sibling IS-IS umbrella; OSPF reuses the SAME route-install / redistribution / edge-plugin / sysrib infra and the SAME spec-set conventions. Read it for the patterns OSPF copies verbatim
5. `internal/plugins/rsvpte/transport_linux.go` -- the in-tree `AF_INET SOCK_RAW` raw-IP transport on an IP protocol number (proto 46). OSPF's transport (proto 89) is this pattern plus IP multicast group membership
6. `internal/plugins/ldp/register.go` -- closest edge-plugin template (protocol engine + SDK lifecycle: RunEngine / OnConfigure / OnStarted / OnExecuteCommand)
7. `internal/core/rib/locrib/candidate.go` -- unified cross-protocol Loc-RIB and best-path selection (the FIB-install path: SPF -> `locrib.Path` -> sysrib `OnChange` -> fibkernel). The sysrib path-group ECMP expansion (`BestChangeEntry.ECMPPaths`) ALREADY EXISTS (added by IS-IS); OSPF reuses it
8. `internal/core/redistevents/events.go` -- route-change producer payload (the REDISTRIBUTION path to BGP, NOT the FIB-install path)
9. Child specs: `plan/spec-ospf-1-types.md` through `plan/spec-ospf-13-cli-diag-interop.md`
10. `plan/spec-ospfv3-0-umbrella.md` -- the deferred OSPFv3 (RFC 5340) follow-up; v3 is a separate edge plugin, not part of this set

## Task

Add native OSPFv2 (Open Shortest Path First version 2, RFC 2328, with NSSA per
RFC 3101 and cryptographic authentication per RFC 5709 / RFC 7474) to Ze as a
link-state interior gateway protocol. The goal is to let a Ze node mesh with
neighbouring routers over OSPF (in addition to BGP and IS-IS), compute shortest
paths per area, and keep the system RIB updated with OSPF-learned routes so the
kernel FIB forwards accordingly. OSPF must also interoperate with the existing
BGP engine through redistribution (OSPF routes into BGP, and BGP / connected /
static routes into OSPF as AS-External LSAs).

Ze has no OSPF today. The implementation is native Go, consistent with Ze's
philosophy of owning its protocols end to end (native BGP, native IKEv2, native
IS-IS), using FRR `ospfd` and BIRD `proto/ospf` only as clean-room references
already distilled in `docs/research/ospf-implementation-guide.md`. OSPF runs
**directly over IP** (IP protocol 89) using the AllSPFRouters (`224.0.0.5`) and
AllDRouters (`224.0.0.6`) multicast groups, not over TCP/UDP. Unlike IS-IS
(which needed a brand-new raw Layer-2 transport), OSPF's transport is a known
in-tree pattern: RSVP-TE already opens an `AF_INET SOCK_RAW` socket on an IP
protocol number (`internal/plugins/rsvpte/transport_linux.go`, proto 46); OSPF
is that pattern on proto 89 plus IP multicast group membership.

### Target scope (decided with user, 2026-06-20)

| Lever | Decision | Effect on the set |
|-------|----------|-------------------|
| Protocol version | **OSPFv2 only** | OSPFv3 (RFC 5340) is a separate edge plugin and separate follow-up umbrella (`spec-ospfv3-0-umbrella.md`). The guide is emphatic: do NOT unify v2/v3 (FRR ships two daemons). The LSA registries and wire encodings differ enough that sharing leaks detail into both |
| Area hierarchy | **Full: multi-area + ABR + stub + NSSA up front** | Backbone + non-backbone areas, ABR Type 3/4 summaries with area ranges (ospf-9), stub-area filtering + default injection and NSSA Type 7 + translator election + Type 7->5 translation (ospf-11) all in scope |
| Network types | **Broadcast (DR/BDR) + point-to-point together** | LAN broadcast with DR/BDR election and Network-LSAs is in scope (ospf-5), alongside point-to-point. NBMA and point-to-multipoint are out of scope (future) |
| Authentication | **In v1** | AuType 0 (Null) / 1 (Simple) / 2 (Cryptographic: RFC 2328 MD5 + RFC 5709 HMAC-SHA) / 3 (RFC 7474 Cryptographic with Extended Sequence Numbers) in the common-header codec (ospf-2); key management and per-packet verify/sign as a dedicated child (ospf-12) |
| Address family | **IPv4 only** | OSPFv2 is IPv4-only by definition. IPv6 is OSPFv3, deferred to the v3 umbrella |

### Reference implementations

| Project | Role | Note |
|---------|------|------|
| Internal research | `docs/research/ospf-implementation-guide.md` | Primary guide: protocol, packets, LSAs, ISM/NSM, DR election, flooding (§13), SPF (§16), areas/ABR/ASBR/NSSA, auth, checksum traps, concurrency, phased order, ze package layout |
| FRRouting `ospfd` | Feature-complete C reference | ~80 files; monolithic packet codec (`ospf_packet.c`) + monolithic LSA codec (`ospf_lsa.c`); per-file route subsystems (`ospf_ia.c`, `ospf_asbr.c`, `ospf_ase.c`, `ospf_abr.c`). Study for edge cases and route-computation split |
| BIRD `proto/ospf` | Compact Go-friendly reference | 18 files; one file per packet type (`hello.c`, `dbdes.c`, `lsreq.c`, `lsupd.c`, `lsack.c`); SPF + inter-area + external + NSSA all in `rt.c`. Study the per-packet-type file split and the single-LSDB-with-domain-filter idea |

The sibling IS-IS implementation (`internal/plugins/isis/`) is the closest
in-tree precedent for everything ABOVE the wire (component, config,
adjacency-style FSM, lazy LSDB, SPF -> Loc-RIB install, redistribution). OSPF
copies those patterns; it does NOT share code with IS-IS (guide §11 "Why not
share code with IS-IS": OSPF has network vertices for transit LANs, IS-IS uses
pseudo-node LSPs; LSA vs LSP lookup keys differ; metric semantics differ).

## Existing Foundation (ground truth from codebase exploration)

| Capability Ze already has | Location (file:line) | How OSPF uses it |
|---------------------------|----------------------|------------------|
| Raw IP socket on an IP protocol number (`AF_INET SOCK_RAW`), kernel builds the IP header, strip-IP-header-on-receive | `internal/plugins/rsvpte/transport_linux.go:24-127` | Direct model for the OSPF transport (proto 89). OSPF ADDS: IP multicast group membership (`224.0.0.5`/`224.0.0.6`) per enabled interface, TTL=1, per-interface source binding |
| Raw-socket doctor check pattern (open+close, `CAP_NET_RAW`) | `internal/plugins/rsvpte/doctor_linux.go:14`, `internal/plugins/isis/transport/doctor_linux.go` | Model for the `doctor-ospf-raw-socket` check |
| Interface up/down + address add/remove EventBus subscription | `internal/plugins/iface/netlink/monitor_linux.go:72-87`; `internal/component/iface/events` | Drive ISM Up/Down, multicast (re)join, neighbour teardown, and Router-LSA re-origination on link/address change |
| Unified Loc-RIB insertion (the FIB-install path) | `internal/core/rib/locrib/`; IS-IS example `internal/plugins/isis/spf/install.go`; BGP example `internal/component/bgp/plugins/rib/rib_bestchange.go:813` (`InsertForward`) | OSPF INSTALLS routes here via `locrib.Path{Source = OSPF ProtocolID, Instance, NextHop, AdminDistance, Metric}` (one Path per ECMP nexthop, distinct `Instance`); sysrib consumes `loc.OnChange` and programs the kernel |
| sysrib Loc-RIB path-group ECMP expansion into `BestChangeEntry.ECMPPaths` | `internal/component/sysrib/sysrib.go`; `internal/component/sysrib/sysrib_ecmp_pathgroup_test.go` | **ALREADY EXISTS** (added by IS-IS, isis-9). OSPF reuses it for free: equal-cost OSPF nexthops reach the kernel without any new sysrib work |
| Admin-distance config per protocol | `internal/component/sysrib/yang/ze-rib-conf.yang:42` (leaf `ospf`); `sysrib.go` `adminDist` map (`"ospf": 110`) | **The `ospf` admin-distance leaf already exists** (default 110). OSPF sets `AdminDistance` = 110 on `locrib.Path`. No new sysrib leaves |
| Redistribution producer payload (pooled, value-typed) | `internal/core/redistevents/events.go:36-122`; IS-IS example `internal/plugins/isis/redistribute/`; orchestrator `internal/component/bgp/plugins/redistribute_egress/redistribute.go` | Used ONLY for redistribution to other protocols; NOT the FIB-install path. OSPF registers as a source `ospf` (-> BGP) and a `RedistConsumer` (connected/static/BGP -> Type 5 AS-External LSAs) |
| Redistribution source + consumer registries | `internal/component/config/redistribute/registry.go`, `consumer.go` | OSPF registers source + consumer exactly like IS-IS (isis-11) |
| Component registration + SDK lifecycle | `internal/plugins/ldp/register.go`, `internal/plugins/isis/register.go`; `registry.Registration` | OSPF component skeleton (RunEngine, OnConfigure/OnStarted/OnExecuteCommand) |
| YANG module discovery/merge (`ze-*-conf.yang`) | `internal/component/config/yang_schema.go:203-231`; `internal/component/config/yang/loader.go` | `ze-ospf-conf.yang` auto-merged at init; `make generate` wires `all/all.go` |
| Custom config validators with `CompleteFn` | `internal/component/config/validators.go`, `validators_register.go` (IS-IS NET/system-id validators) | OSPF router-id (dotted-quad) and area-id validators |
| CLI show registration (central `ze-show`/`ze-clear`) | `internal/plugins/ldp/cmd_show.go`, `internal/plugins/isis/cmd_show.go`; `pluginserver.RegisterRPCs` | `show ip ospf neighbor/interface/database/route/border-routers/spf` |
| Doctor checks, metrics, web SSE | `ai/rules/doctor-checks.md`, `internal/core/metrics`, `internal/component/web` | `CAP_NET_RAW` check, OSPF counters, neighbour/database views |

**OSPF introduces NO genuinely new low-level capability.** The raw-IP transport
already exists (RSVP-TE), the FIB-install path already exists (Loc-RIB/sysrib,
including ECMP expansion), the admin-distance leaf already exists, and the
component/config/redistribution machinery is protocol-agnostic. The only new I/O
wrinkle is IP multicast group membership on the raw socket. The risk and novelty
are entirely in protocol logic: the 5 packet types + LSA registry codec (two
distinct checksums), ISM/NSM correctness, the §13 flooding procedure, the §16
SPF with the two-way check, and multi-area ABR/ASBR/NSSA route computation.

## Design Principles

| Principle | Detail |
|-----------|--------|
| Native Go | Implement OSPF entirely in Ze, no FRR/bird subprocess. Consistent with native BGP/IKEv2/IS-IS. FRR/BIRD are references for edge cases only |
| Layered packages, leaf-first | `types` (leaf) <- `packet` codec <- `instance`/`area`/`iface`/`neighbor` runtime, mirroring BIRD's clean layering and Ze's own component conventions. `packet` never imports runtime |
| Lazy / buffer-first LSDB | Store received LSAs as raw bytes plus parsed metadata (LSA key, sequence, age, checksum); parse the body on demand. Matches Ze's zero-copy philosophy (`ai/rules/buffer-first.md`) and lets unknown opaque LSAs re-flood verbatim |
| IP transport modelled on RSVP-TE | Reuse the proven `AF_INET SOCK_RAW` pattern; isolate the raw-socket backend behind an interface so a future BSD/VPP backend can drop in. ADD only multicast membership |
| Install via Loc-RIB insertion | OSPF does not invent route installation: SPF results are INSERTED into the Loc-RIB (`locrib.Path`, like IS-IS `spf/install.go` and BGP `rib_bestchange.go:813`); sysrib + fibkernel arbitrate and program the kernel. `redistevents` is a SEPARATE path used only for redistribution to BGP (ospf-10), never for FIB install |
| Edge plugin, not component dir | Lives in `internal/plugins/ospf/` like LDP, RSVP-TE, and IS-IS. OSPF is a config-driven protocol engine with no reverse dependencies, so `ai/rules/module-tiers.md` places it in `internal/plugins/`, not `internal/component/` |
| Per-interface goroutine split | RX / TX / timers per interface; ISM and NSM as independent event-driven engines; LSDB guarded by a single writer; SPF debounced and event-driven (guide §9 recommended model) |
| One file per packet type, one file per LSA-type family | Group the codec by packet type (`hello`, `dbdesc`, `lsreq`, `lsupdate`, `lsack`) and by LSA-type family (`lsa_router`, `lsa_network`, `lsa_summary`, `lsa_external`), the BIRD-style middle path between FRR's two monoliths and a file-per-field scatter |
| Per-area LSDB | Each area keeps its own LSDB (FRR model); Type 5 AS-External LSAs live in an AS-wide store shared across areas. Simpler than BIRD's single-LSDB-with-domain-filter for a first pass |
| Separate from IS-IS | No shared SPF/LSDB engine in v1 (guide §11). Refactor a common core out only after both work independently and the duplication proves mechanical |

## Scope

### In scope (this spec set)

| Area | Child spec |
|------|-----------|
| Domain types (RouterID, AreaID, LSAKey, LSSequenceNumber, LSAge, Metric, OSPF Options) + Fletcher-16 (LSA) and IP one's-complement (packet) checksums | ospf-1 |
| Packet + LSA wire codec: 24-byte common header (AuType 0/1/2/3), 5 packet types (Hello, DD, LS Request, LS Update, LS Ack), 20-byte LSA header, LSA bodies (Router 1, Network 2, Summary 3/4, AS-External 5, NSSA 7); fuzz | ospf-2 |
| Raw IP transport (`AF_INET SOCK_RAW` proto 89, multicast `224.0.0.5`/`224.0.0.6` join/leave per interface, TTL=1, per-interface RX/TX, IP-header strip, `CAP_NET_RAW` doctor) | ospf-3 |
| Component registration, `ze-ospf-conf.yang`, config resolution, instance/area/interface scaffolding, lifecycle wiring | ospf-4 |
| Interface State Machine (8 states), Hello send/receive + validation, DR/BDR election (RFC 2328 §9.4 incl. sticky rule) | ospf-5 |
| Neighbor State Machine (8 states), DD master/slave exchange (I/M/MS bits, MTU check), LS Request list population + drain to Full | ospf-6 |
| Per-area LSDB (lazy raw bytes + metadata), self-LSA origination (Router-LSA, Network-LSA as DR), the §13 flooding procedure (per-neighbour retransmit lists, delayed acks, MinLSArrival/MinLSInterval), MaxAge walker, LSRefresh, purge; `show ip ospf database` | ospf-7 |
| Intra-area SPF (RFC 2328 §16.1 two-stage Dijkstra, two-way check), ECMP, SPF throttle, route table with path types, FIB install via Loc-RIB insertion; `show ip ospf route` | ospf-8 |
| Inter-area routing: Type 3 (network) and Type 4 (ASBR) Summary-LSA origination at the ABR, inter-area route computation (§16.2/§16.3), area ranges (aggregate / not-advertise) | ospf-9 |
| AS-External routing: Type 5 AS-External-LSA origination at the ASBR, external route computation (§16.4) with E1/E2 semantics + forwarding address, `default-information originate`, redistribution source (OSPF -> BGP) and consumer (connected/static/BGP -> Type 5) | ospf-10 |
| Stub areas (no Type 5; ABR-injected default Type 3) and NSSA (RFC 3101): Type 7 origination + flooding within the NSSA, translator election, Type 7 -> Type 5 translation at the elected ABR | ospf-11 |
| Authentication: AuType 1 (Simple), AuType 2 (Cryptographic: RFC 2328 MD5 + RFC 5709 HMAC-SHA), and AuType 3 (RFC 7474 Cryptographic with Extended Sequence Numbers), per-interface keys, key rotation, verify-on-receive / sign-on-send | ospf-12 |
| CLI completeness (`show ip ospf`, `neighbor`, `interface`, `database`, `route`, `border-routers`, `spf`), web neighbour/database views, Prometheus metrics, doctor checks, FRR `ospfd` interop scenarios | ospf-13 |
| Stub Router Advertisement (RFC 3137 / RFC 6987, "max-metric router-lsa") origination | ospf-7 (origination) / ospf-13 (config + CLI) |

### Out of scope (future, noted here so it is not silently assumed done)

| Area | Reason |
|------|--------|
| OSPFv3 (RFC 5340, IPv6) | Separate edge plugin + separate follow-up umbrella (`spec-ospfv3-0-umbrella.md`). Different wire format and LSA registry; guide §15 says do NOT unify |
| Opaque-LSA framework (RFC 5250) and consumers (TE RFC 3630, Router Information RFC 7770, Extended Link/Prefix RFC 7684) | Plumbing is a SHOULD; consumers are large extensions on a stable base. Future umbrella |
| Segment Routing (RFC 8665), TI-LFA / LFA (RFC 5286) | Depend on opaque framework + stable base. Future |
| Virtual links (RFC 2328 §15) | Advanced backbone-repair feature; add once the backbone is solid (guide §10 decision) |
| NBMA and point-to-multipoint network types | Broadcast + P2P only in v1; NBMA neighbour config + P2MP are future network types |
| Graceful Restart restarter + helper (RFC 3623) | Helper mode is a SHOULD; defer to a later child. Restarter side later still |
| BFD for OSPF (RFC 5880 / RFC 5881) | Ze has a BFD engine; integration is a later child |
| Demand circuits / Flood Reduction (RFC 1793 / RFC 7715), Multi-Instance (RFC 6549), L3VPN DN bit (RFC 4576), TOS routing | Niche or deprecated; out per guide §14 |
| SNMP OSPF MIB (RFC 4750), ospfclient external LSA API | Ze uses gNMI/Prometheus, not SNMP; no external-injection socket |

## Architecture (package layout)

Modelled on guide §11 and the IS-IS component layout.

| Path | Concern | Spec |
|------|---------|------|
| `internal/plugins/ospf/register.go` | component registration + RunEngine + SDK lifecycle | ospf-4 |
| `config.go` | config parse/resolve from YANG tree | ospf-4 |
| `instance.go` | top-level OSPF instance: router-id, area map, AS-external store, timers, goroutine lifecycle, packet receive dispatcher | ospf-4 |
| `area.go` | per-area state: LSDB, interface set, SPF trigger | ospf-4 / ospf-8 |
| `events.go` | event namespace + types | ospf-4 |
| `types/` | RouterID, AreaID, LSAKey, LSSequenceNumber, LSAge, Metric, Options; Fletcher-16 + IP checksum | ospf-1 |
| `packet/` | common header, hello, dbdesc, lsreq, lsupdate, lsack, lsa header, lsa_router, lsa_network, lsa_summary, lsa_external, auth | ospf-2, ospf-12 |
| `transport/` | raw IP socket backend, multicast membership, send/receive | ospf-3 |
| `iface/` | OSPF-enabled interface + ISM + Hello + DR/BDR election | ospf-5 |
| `neighbor/` | neighbour state + NSM + DD exchange + LS Request drain | ospf-6 |
| `lsdb/` | per-area LSDB store, origination, aging, flooding | ospf-7 |
| `spf/` | graph build, two-stage Dijkstra, route output, inter-area, external, install | ospf-8, ospf-9, ospf-10, ospf-11 |
| `redistribute/` | redistevents producer + RedistConsumer (Type 5) | ospf-10 |
| `cmd_show.go` | CLI show RPC registration | ospf-13 |
| `yang/` | `ze-ospf-conf.yang` (config, ospf-4), `ze-ospf-cmd.yang` (show/clear command tree binding central `ze-show:`/`ze-clear:`, ospf-13) + generated register.go/embed.go. No `ze-ospf-api.yang`: show/clear RPCs live in the central `ze-show`/`ze-clear` namespaces (Go-registered), LDP/IS-IS style | ospf-4, ospf-13 |

## Shared Contracts (canonical)

Single source of truth for cross-spec interfaces. Child specs reference this
section rather than redefining (and contradicting) these contracts.

### Route install vs redistribution (two distinct paths)
- **FIB install (ospf-8):** insert SPF routes into the Loc-RIB via `locrib.Path{Source = OSPF ProtocolID, Instance, NextHop, AdminDistance, Metric}` (model IS-IS `spf/install.go`, BGP `rib_bestchange.go:813`). sysrib consumes `loc.OnChange` and programs fibkernel as `RTPROT_ZE`. This is NOT `redistevents`.
- **Redistribution (ospf-10):** `redistevents` producer feeds the redistribute-orchestrator which dispatches to `RedistConsumer`s (export OSPF to BGP); OSPF also implements `RedistConsumer` to import connected/static/BGP. `redistevents` NEVER installs to the FIB.
- **Redistribution source (ospf-10):** OSPF registers a SINGLE protocol/source named `ospf` (`redistevents.RegisterProtocol("ospf")` + `RegisterProducer`, plus `configredist.RegisterSource(RouteSource{Name: "ospf", Protocol: "ospf", ...})` wrapped in a `sync.Once` `mustRegister`, exactly like IS-IS). SPF route changes are emitted as `RouteChangeBatch{Protocol = the ospf ProtocolID}`. Per-area or per-path-type source names are NOT used: the orchestrator derives the source purely from `ProtocolName(b.Protocol)`, and the generic loop-prevention check (`route.Origin == importingProtocol`) keeps OSPF self-import auto-rejected. A single `ospf` source matches the single admin distance.
- **Admin distance:** OSPF sets a single `AdminDistance` (110) on `locrib.Path` (config `rib.admin-distance.ospf`, the EXISTING leaf, default 110). `locrib.Path` has no path-type field, so the RFC 2328 §11 intra < inter < E1 < E2 preference is resolved INSIDE OSPF SPF, which publishes one winning Path per prefix; per-path-type distance vs other protocols is future work that would need a `locrib.Path` protoType field. Loc-RIB `Source` = OSPF ProtocolID; `Instance` distinguishes ECMP nexthops.
- **ECMP (in scope, committed):** OSPF inserts one `locrib.Path` per equal-cost nexthop (distinct `Instance`). The sysrib path-group expansion into `BestChangeEntry.ECMPPaths` ALREADY EXISTS (added by IS-IS, `sysrib_ecmp_pathgroup_test.go`). ospf-8 reuses it; it does NOT re-add sysrib ECMP support. Default ECMP cap is 8 paths (guide §10 decision).

### Packet receive dispatcher (owner: ospf-4 `instance.go`)
- ospf-3 transport delivers `(ifindex, src netip.Addr, payload []byte)` after stripping the IP header.
- ospf-4 owns a dispatcher keyed by the common-header `Type` field (1 Hello, 2 DD, 3 LS Request, 4 LS Update, 5 LS Ack). Hello -> ISM/neighbour discovery (ospf-5); DD / LS Request -> NSM database exchange (ospf-6); LS Update / LS Ack -> LSDB/flooding (ospf-7). Before dispatch the instance validates: version == 2, Area ID matches the receiving interface's area, checksum, and authentication (ospf-12). Handlers register at startup; transport holds no protocol switch.

### Frame addressing + transport (owner: ospf-3)
- OSPF packets are sent to `224.0.0.5` (AllSPFRouters) by all routers, and to `224.0.0.6` (AllDRouters) by DROther routers addressing the DR/BDR; on point-to-point links the destination is `224.0.0.5`. Unicast is used for some retransmissions and for DD/LS Request to a specific neighbour (implementation choice on point-to-point). IP TTL is 1 (link-local). The socket joins `224.0.0.5` on every enabled interface and `224.0.0.6` when the router is DR or BDR on that interface. The kernel builds the outgoing IP header; on receive the raw socket delivers the full datagram and ospf-3 strips the IP header (IHL) before handing `(src, payload)` up. Transport MUST NOT parse or alter OSPF payload bytes.

### Two distinct checksums (owner: ospf-1 algorithms; ospf-2 application)
- **OSPF packet checksum:** the standard IP one's-complement checksum (RFC 1071) computed over the ENTIRE OSPF packet EXCLUDING the 64-bit Authentication field (bytes 16..23 of the common header), with the Checksum field itself zeroed during computation. For AuType 2 and AuType 3 (Cryptographic) the packet Checksum field is set to 0 and the digest is appended after the packet body instead (ospf-12); see trap "Authentication with Zeroed Checksum".
- **LSA checksum:** the ISO Fletcher-16 checksum (RFC 905 Annex B / ISO 8473) computed over the LSA starting at the Options field (i.e., EXCLUDING the first two bytes, the LS Age field, which mutates in flight). The LS Checksum field is part of the covered range and is treated as zero during the forward computation. This is the SAME Fletcher algorithm as IS-IS; the covered range differs.
- Both are owned by ospf-1 (the algorithms, with RFC 905 and RFC 1071 vector tests) and applied by ospf-2 (codec) / ospf-7 (LSA re-origination refreshes the Fletcher checksum).

### LSA inventory (codec owner ospf-2; originators noted)
| LS Type | Name | LS ID semantics | Adv. Router | Scope | Originated by |
|---------|------|-----------------|-------------|-------|---------------|
| 1 | Router-LSA | originating router's Router ID | self | area | ospf-7 (self), ospf-5 feeds link list |
| 2 | Network-LSA | IP interface address of the DR | DR | area | ospf-7 (when DR, ospf-5 election) |
| 3 | Summary-LSA (network) | the summarised network address | ABR self | area | ospf-9 (ABR) |
| 4 | Summary-LSA (ASBR) | the ASBR's Router ID | ABR self | area | ospf-9 (ABR, when an ASBR exists) |
| 5 | AS-External-LSA | the external network address | ASBR self | AS (not flooded into stub/NSSA) | ospf-10 (ASBR) |
| 7 | NSSA-LSA | the external network address | NSSA ASBR self | area (NSSA only) | ospf-11 |
| 9/10/11 | Opaque (link/area/AS) | type+instance | self | per scope | OUT OF SCOPE v1 (lazy passthrough only if a framework is added later) |

### LSA header + body layout (resolves cross-spec detail; owner ospf-2)
- **LSA common header (20 bytes):** LS Age (2, MaxAge 3600, DoNotAge bit 0x8000), Options (1), LS Type (1), Link State ID (4), Advertising Router (4), LS Sequence Number (4, signed, InitialSequenceNumber 0x80000001, MaxSequenceNumber 0x7FFFFFFF), LS Checksum (2, Fletcher), Length (2, includes the 20-byte header).
- **Router-LSA (Type 1):** flags byte (V/E/B bits for virtual-endpoint / ASBR / ABR), then a link count and that many link records: Link ID (4), Link Data (4), Type (1: 1 p2p, 2 transit, 3 stub, 4 virtual), #TOS (1, 0 in v1), metric (2). The two-way check in SPF (ospf-8) depends on the transit-link encoding (Link ID = DR interface address, Link Data = own interface address).
- **Network-LSA (Type 2):** Network Mask (4), then the list of attached routers' Router IDs (4 each, INCLUDING the DR itself).
- **Summary-LSA (Type 3/4):** Network Mask (4) [for Type 4 this field is 0.0.0.0], then TOS (1, 0) + 3-byte Metric. Link State ID is the network (Type 3) or the ASBR Router ID (Type 4).
- **AS-External-LSA (Type 5) / NSSA-LSA (Type 7):** Network Mask (4), then per-TOS: E-bit (0x80 of the first metric byte: 0 = E1, 1 = E2) + 3-byte Metric, Forwarding Address (4), External Route Tag (4). Type 7 carries the NSSA P-bit in the LSA-header Options field.
- This single layout is used by ospf-2 (codec), ospf-7 (origination), ospf-8/9/10/11 (SPF read + origination).

### Next-hop derivation for SPF (owner ospf-8)
- For a destination reached across a transit network, the next hop is the IP address of the next-hop router's interface ON that network, learned from the Router-LSA Link Data of the link that attaches it to the network (RFC 2328 §16.1.1). For a directly-attached point-to-point neighbour the next hop is the neighbour's interface address (Hello source / Router-LSA Link Data). The calculating router resolves the first hop on the SPT toward the destination.

### Route preference / path types (owner ospf-8, consumed by ospf-9/10/11)
- Intra-area routes are preferred over inter-area, which are preferred over external; within external, E1 (cost = path-to-ASBR + advertised metric) is preferred over E2 (cost = advertised metric only, tie-broken by path-to-forwarding-address). RFC 2328 §16.4.1 / §11. OSPF resolves this preference INTERNALLY and publishes one winning `locrib.Path` per prefix with `AdminDistance` = 110 regardless of path type.

### Area + interface config model (schema owner ospf-4)
- Top-level `ospf` container: `router-id` (dotted-quad), `reference-bandwidth` (auto-cost), `redistribute` wiring, `default-information originate`.
- Per-area under `areas/area`: `area-id` (dotted-quad or integer), `area-type` (enum `normal`/`stub`/`nssa`; `stub`/`nssa` may set `no-summary` for totally-stubby/totally-NSSA), `default-cost` (ABR's injected default into stub/NSSA), `ranges/range` (prefix + `advertise`/`not-advertise` + optional cost).
- Per-interface enrolment under `interfaces/interface` (or area-scoped): `area` binding (per-interface, NOT FRR `network <prefix> area` matching -- guide §10 decision), `network-type` (enum `broadcast`/`point-to-point`), `cost`, `hello-interval` (default 10), `dead-interval` (default 40), `priority` (DR election, default 1), `passive` (originate but no Hellos), `mtu-ignore`, `authentication` (per-interface, with an `inherit` option for the area key -- guide §10), `retransmit-interval`, `transmit-delay`.
- ECMP is enabled by default, cap 8 (guide §10).

### Authentication config model (schema owner ospf-4, semantics ospf-12)
- Key chains, not bare strings: each key has key-id, algorithm (enum `simple`/`md5`/`hmac-sha-1`/`hmac-sha-256`/...), secret (`$9$`-encoded), optional send/accept lifetimes (hitless rotation). Per-interface chains (with an area-level default via `inherit`). AuType codes on the wire (four schemes; see `rfc/short/rfc2328.md`, `rfc/short/rfc5709.md`, `rfc/short/rfc7474.md`):
  - **0 Null** -- no authentication; the 8-byte field is unused.
  - **1 Simple** -- 8-byte cleartext password in the auth field.
  - **2 Cryptographic** (RFC 2328 Appendix D, extended by RFC 5709) -- the 8-byte auth field carries Reserved(2)=0 + Key ID(1) + Auth Data Length(1) + a 32-bit Cryptographic Sequence Number; the digest (Keyed-MD5 per RFC 2328, or HMAC-SHA-1/256/384/512 per RFC 5709) is appended after the packet body; Packet Length excludes the digest; the packet Checksum field is set to 0.
  - **3 Cryptographic with Extended Sequence Numbers** (RFC 7474) -- a DISTINCT AuType, not a variant of 2: the 8-byte auth field is restructured to Reserved(3 octets, was 2)=0 + Key ID(4 octets, was 1, in the former sequence-number position) + Auth Data Length(1), and a 64-bit Cryptographic Sequence Number is appended BEFORE the digest, giving replay protection across reboots; the HMAC-SHA algorithm set is still RFC 5709; the packet Checksum field is set to 0.
- RFC 5709 supplies the HMAC-SHA algorithms used by BOTH AuType 2 and AuType 3; RFC 7474's contribution is the AuType-3 restructured field plus the 64-bit sequence number (NOT a new algorithm). The ospf-12 child owns the verify/sign semantics; ospf-2 owns the AuType-0/1/2/3 field framing.

### Command + API YANG (owner ospf-13; enforced by command-ownership check)
- show/clear commands require owner command YANG. OSPF ships ONE command YANG, `ze-ospf-cmd.yang` (CLI tree: `show ip ospf [neighbor|interface|database|route|border-routers|spf]` binding `ze-show:ospf-*`; `clear ip ospf [process|neighbor|counters]` binding `ze-clear:ospf-*`), modelled on `ze-ldp-cmd.yang` / `ze-isis-cmd.yang`. There is NO `ze-ospf-api.yang`: both show and clear RPCs live in the CENTRAL `ze-show`/`ze-clear` namespaces and are registered in Go. `scripts/checks/command_ownership.go` enforces the command-YANG ownership.

### Metrics (canonical, owner of each series noted; surfaced by ospf-13)
Single exact and COMPLETE set of Prometheus series and labels -- the one
contract. Each owning spec registers its OWN rows (per-owner registration);
ospf-13 registers NONE, it only scrapes/asserts the full set. No bare `ospf_*`
names. `area` is the dotted-quad area id; `type` for packets is
`hello|dd|lsreq|lsupdate|lsack`; `type` for LSAs is `1|2|3|4|5|7`. A child spec
that lists metrics MUST name the exact series from this table and own only its
assigned rows.

| Metric | Type | Labels | Owner |
|--------|------|--------|-------|
| `ze_ospf_packets_sent_total` | counter | `interface`, `type` | ospf-3 |
| `ze_ospf_packets_received_total` | counter | `interface`, `type` | ospf-3 |
| `ze_ospf_packets_dropped_total` | counter | `interface`, `reason` | ospf-3 |
| `ze_ospf_sockets_open` | gauge | (none) | ospf-3 |
| `ze_ospf_interface_up` | gauge | `area`, `interface` | ospf-5 |
| `ze_ospf_dr_elections_total` | counter | `interface` | ospf-5 |
| `ze_ospf_neighbors` | gauge | `area`, `interface`, `state` | ospf-6 |
| `ze_ospf_adjacencies_full` | gauge | `area` | ospf-6 |
| `ze_ospf_nsm_events_total` | counter | `event` | ospf-6 |
| `ze_ospf_lsdb_lsas` | gauge | `area`, `type` | ospf-7 |
| `ze_ospf_lsa_originations_total` | counter | `type` | ospf-7 |
| `ze_ospf_lsa_refreshes_total` | counter | `type` | ospf-7 |
| `ze_ospf_lsa_purges_total` | counter | `type` | ospf-7 |
| `ze_ospf_lsupdates_sent_total` | counter | `interface` | ospf-7 |
| `ze_ospf_lsupdates_received_total` | counter | `interface` | ospf-7 |
| `ze_ospf_lsacks_sent_total` | counter | `interface` | ospf-7 |
| `ze_ospf_retransmissions_total` | counter | `area` | ospf-7 |
| `ze_ospf_spf_runs_total` | counter | `area` | ospf-8 |
| `ze_ospf_spf_duration_seconds` | histogram | `area` | ospf-8 |
| `ze_ospf_routes_installed` | gauge | `type` | ospf-8 |
| `ze_ospf_abr` | gauge | (none) | ospf-9 |
| `ze_ospf_summary_lsas` | gauge | `area` | ospf-9 |
| `ze_ospf_asbr` | gauge | (none) | ospf-10 |
| `ze_ospf_external_lsas` | gauge | (none) | ospf-10 |
| `ze_ospf_redist_injected_total` | counter | `source` | ospf-10 |
| `ze_ospf_redist_withdrawn_total` | counter | `source` | ospf-10 |
| `ze_ospf_nssa_translations_total` | counter | `area` | ospf-11 |
| `ze_ospf_auth_failures_total` | counter | `interface`, `reason` | ospf-12 |

### Test + interop wiring (mandatory)
- The `test/ospf` suite is registered in `internal/test/cli/register.go` and `mk/test-functional.mk` (ospf-4 adds the suite; later specs add cases). Raw-IP / multicast tests are Linux-only and run as QEMU integration tests (`ai/rules/qemu-testing.md`), not plain `.ci`. FRR `ospfd` interop is MANDATORY (not deferrable), owned by ospf-13: P2P, broadcast/DR, multi-area, stub, NSSA, redistribution, authentication, and failover/convergence. ospf-13 Goal Validation must be filled with these scenarios.

## Child Specs

| Phase | Spec | Scope summary | Depends |
|-------|------|---------------|---------|
| 1 | `spec-ospf-1-types.md` | Domain types (RouterID, AreaID, LSAKey, LSSequenceNumber, LSAge incl. MaxAge/DoNotAge, Metric, Options) with parse/format/compare/serialize; Fletcher-16 (LSA) and IP one's-complement (packet) checksums with RFC 905 / RFC 1071 vector tests; no I/O | - |
| 2 | `spec-ospf-2-wire.md` | Codec for the 24-byte common header (AuType 0/1/2/3), the 5 packet types (Hello, DD, LS Request, LS Update, LS Ack), the 20-byte LSA header, and LSA bodies (Router 1, Network 2, Summary 3/4, AS-External 5, NSSA 7); round-trip + fuzz; real-capture decode | `spec-ospf-1-types.md` |
| 3 | `spec-ospf-3-ip-transport.md` | Raw IP transport: `AF_INET SOCK_RAW` proto 89 behind an interface, IP multicast membership (`224.0.0.5`/`224.0.0.6`) per enabled interface, TTL=1, per-interface RX/TX goroutines, IP-header strip, `CAP_NET_RAW` doctor check | `spec-ospf-1-types.md` |
| 4 | `spec-ospf-4-component-config.md` | **Wiring backbone (MANDATORY before runtime specs)**: `internal/plugins/ospf/` registration, `ze-ospf-conf.yang` (router-id, areas/area-type/ranges/auth defaults, per-interface area/network-type/cost/timers/priority/passive/auth refs), config resolve to typed structs, instance/area/interface scaffolding, packet receive dispatcher using the ospf-2 common-header codec, OnConfigure/OnConfigApply/OnStarted, `make generate`, `all/all.go` | `spec-ospf-2-wire.md`, `spec-ospf-3-ip-transport.md` |
| 5 | `spec-ospf-5-interface-ism.md` | Interface State Machine (Down/Loopback/Waiting/Point-to-Point/DROther/Backup/DR) + events, Hello send/receive + header validation, DR/BDR election (RFC 2328 §9.4 incl. sticky rule), Wait timer; `show ip ospf interface` | `spec-ospf-2-wire.md`, `spec-ospf-4-component-config.md` |
| 6 | `spec-ospf-6-neighbor-nsm.md` | Neighbor State Machine (Down/Attempt/Init/2-Way/ExStart/Exchange/Loading/Full) + events, DD master/slave negotiation (I/M/MS bits, DD sequence, MTU check), LS Request list population + drain, adjacency formation rules; `show ip ospf neighbor` | `spec-ospf-5-interface-ism.md` |
| 7 | `spec-ospf-7-lsdb-flooding.md` | Per-area LSDB (lazy raw bytes + metadata), self-LSA origination (Router-LSA from interfaces/neighbours, Network-LSA as DR), the §13 flooding procedure (freshness compare §13.1, retransmit lists, delayed acks, MinLSArrival/MinLSInterval), MaxAge walker + purge, LSRefresh, sequence wraparound, stub-router (max-metric) origination; `show ip ospf database` | `spec-ospf-6-neighbor-nsm.md` |
| 8 | `spec-ospf-8-spf-rib.md` | **SPF + FIB install**: intra-area two-stage Dijkstra (§16.1) over Router/Network-LSAs with the two-way check, ECMP equal-cost parent merge, SPF throttle (exponential back-off), route table with path types, INSERT into Loc-RIB with `AdminDistance` = 110 -> sysrib `OnChange` -> fibkernel (reusing the existing ECMP path-group expansion); `show ip ospf route`, `show ip ospf spf` | `spec-ospf-7-lsdb-flooding.md` |
| 9 | `spec-ospf-9-inter-area-abr.md` | ABR detection, Type 3 (network) and Type 4 (ASBR) Summary-LSA origination into each attached area, inter-area route computation (§16.2 from summaries, §16.3 ABR examines backbone summaries), area ranges (aggregate / not-advertise), backbone-attachment rule; `show ip ospf border-routers` | `spec-ospf-8-spf-rib.md` |
| 10 | `spec-ospf-10-as-external-asbr.md` | ASBR: Type 5 AS-External-LSA origination on redistributed routes, external route computation (§16.4) with E1/E2 + forwarding-address resolution after the ospf-9 inter-area route table exists, `default-information originate`; register OSPF as redistribution source `ospf` (-> BGP) and `RedistConsumer` (connected/static/BGP -> Type 5); `redistribute` YANG wiring | `spec-ospf-8-spf-rib.md`, `spec-ospf-9-inter-area-abr.md` |
| 11 | `spec-ospf-11-stub-nssa.md` | Stub areas (suppress Type 4/5 in the area, ABR injects a Type 3 default; totally-stubby `no-summary`), NSSA (RFC 3101): Type 7 origination + flooding within the NSSA, translator election (§3.5, highest Router ID ABR with the P-bit), Type 7 -> Type 5 translation onto the backbone, NSSA default handling | `spec-ospf-9-inter-area-abr.md`, `spec-ospf-10-as-external-asbr.md` |
| 12 | `spec-ospf-12-auth.md` | Authentication: AuType 1 (Simple), AuType 2 (Cryptographic: RFC 2328 MD5 + RFC 5709 HMAC-SHA), and AuType 3 (RFC 7474 Cryptographic with Extended Sequence Numbers), per-interface key chains with area `inherit`, verify-on-receive / sign-on-send for all 5 packet types including LS Update/Ack, key rotation, packet-checksum-zeroed handling | `spec-ospf-2-wire.md`, `spec-ospf-3-ip-transport.md`, `spec-ospf-4-component-config.md`, `spec-ospf-5-interface-ism.md`, `spec-ospf-6-neighbor-nsm.md`, `spec-ospf-7-lsdb-flooding.md` |
| 13 | `spec-ospf-13-cli-diag-interop.md` | CLI completeness (`show ip ospf` + subcommands, `clear ip ospf`), web neighbour/database views, Prometheus metrics scrape/assert, doctor checks, max-metric config, FRR `ospfd` interop scenarios (P2P, broadcast/DR, multi-area, redistribution, stub/NSSA, auth, convergence) | `spec-ospf-1-types.md`, `spec-ospf-2-wire.md`, `spec-ospf-3-ip-transport.md`, `spec-ospf-4-component-config.md`, `spec-ospf-5-interface-ism.md`, `spec-ospf-6-neighbor-nsm.md`, `spec-ospf-7-lsdb-flooding.md`, `spec-ospf-8-spf-rib.md`, `spec-ospf-9-inter-area-abr.md`, `spec-ospf-10-as-external-asbr.md`, `spec-ospf-11-stub-nssa.md`, `spec-ospf-12-auth.md` |

## Dependency Graph

| Spec | Depends on |
|------|-----------|
| `spec-ospf-1-types.md` | - |
| `spec-ospf-2-wire.md` | `spec-ospf-1-types.md` |
| `spec-ospf-3-ip-transport.md` | `spec-ospf-1-types.md` |
| `spec-ospf-4-component-config.md` (wiring backbone) | `spec-ospf-2-wire.md`, `spec-ospf-3-ip-transport.md` |
| `spec-ospf-5-interface-ism.md` | `spec-ospf-2-wire.md`, `spec-ospf-4-component-config.md` |
| `spec-ospf-6-neighbor-nsm.md` | `spec-ospf-5-interface-ism.md` |
| `spec-ospf-7-lsdb-flooding.md` | `spec-ospf-6-neighbor-nsm.md` |
| `spec-ospf-8-spf-rib.md` | `spec-ospf-7-lsdb-flooding.md` |
| `spec-ospf-9-inter-area-abr.md` | `spec-ospf-8-spf-rib.md` |
| `spec-ospf-10-as-external-asbr.md` | `spec-ospf-8-spf-rib.md`, `spec-ospf-9-inter-area-abr.md` |
| `spec-ospf-11-stub-nssa.md` | `spec-ospf-9-inter-area-abr.md`, `spec-ospf-10-as-external-asbr.md` |
| `spec-ospf-12-auth.md` | `spec-ospf-2-wire.md`, `spec-ospf-3-ip-transport.md`, `spec-ospf-4-component-config.md`, `spec-ospf-5-interface-ism.md`, `spec-ospf-6-neighbor-nsm.md`, `spec-ospf-7-lsdb-flooding.md` |
| `spec-ospf-13-cli-diag-interop.md` | `spec-ospf-1-types.md`, `spec-ospf-2-wire.md`, `spec-ospf-3-ip-transport.md`, `spec-ospf-4-component-config.md`, `spec-ospf-5-interface-ism.md`, `spec-ospf-6-neighbor-nsm.md`, `spec-ospf-7-lsdb-flooding.md`, `spec-ospf-8-spf-rib.md`, `spec-ospf-9-inter-area-abr.md`, `spec-ospf-10-as-external-asbr.md`, `spec-ospf-11-stub-nssa.md`, `spec-ospf-12-auth.md` |

`spec-ospf-1-types.md`, `spec-ospf-2-wire.md`, `spec-ospf-3-ip-transport.md` are
parallelisable foundations. `spec-ospf-4-component-config.md` is the integration
backbone and must be implemented before any runtime spec (5+). The OSPF NSM
(ospf-6) depends on the ISM (ospf-5) because adjacency formation is gated on the
interface reaching DR/Backup/DROther/Point-to-Point and on DR/BDR identity.

## RFC Coverage

| RFC | Topic | Summary status |
|-----|-------|----------------|
| RFC 2328 | OSPF Version 2 (base standard) | CREATED `rfc/short/rfc2328.md` (via `/ze-rfc`, ospf-2/5/6/7/8) |
| RFC 905 | ISO Transport Protocol -- Fletcher checksum (Annex) | CREATED `rfc/short/rfc905.md` (ospf-1); the Fletcher algorithm is shared with IS-IS |
| RFC 1071 | Computing the Internet Checksum | CREATED `rfc/short/rfc1071.md` (ospf-1) |
| RFC 3101 | OSPF Not-So-Stubby Area (NSSA) Option | CREATED `rfc/short/rfc3101.md` (ospf-11) |
| RFC 5709 | OSPFv2 HMAC-SHA Cryptographic Authentication | CREATED `rfc/short/rfc5709.md` (ospf-12) |
| RFC 7474 | Security Extensions for OSPFv2 (auth trailer) | CREATED `rfc/short/rfc7474.md` (ospf-12) |
| RFC 3137 / RFC 6987 | OSPF Stub Router Advertisement | CREATED `rfc/short/rfc6987.md` (ospf-7) |
| RFC 5250 | OSPF Opaque LSA Option | CREATED `rfc/short/rfc5250.md` (referenced as out-of-scope framework) |
| RFC 9129 | YANG Data Model for OSPF | CREATED `rfc/short/rfc9129.md` (ospf-4 schema shape) |

The short RFC summaries for this set have been CREATED under `rfc/short/`
(sourced from the RFC text, house format with Meta + wire formats + Compliance
Checklist). The deeper per-RFC implementation summaries under
`docs/architecture/rfc/` (the `/ze-rfc` output) are produced at implementation
time for the RFCs whose normative detail the code must enforce (RFC 2328 first).

## Key Design Questions (Resolved)

| Question | Decision | Rationale |
|----------|----------|-----------|
| Native vs wrap FRR/bird? | Native Go | Consistent with native BGP/IKEv2/IS-IS; no subprocess on the gokrazy appliance; Ze owns the protocol. FRR/BIRD are clean-room references only |
| Edge plugin dir vs component dir? | `internal/plugins/ospf/` | Module-tier rule: engines with no feature depending on them are edge plugins. LDP, RSVP-TE, and IS-IS are in `internal/plugins/`; BGP/sysrib stay in `internal/component/` because other features depend on them |
| Share an SPF/LSDB engine with IS-IS? | No (v1) | Guide §11: OSPF has network vertices for transit LANs, IS-IS uses pseudo-node LSPs; LSA vs LSP keys and metric semantics differ. A shared abstraction leaks detail into both. Refactor later if the duplication proves mechanical |
| OSPFv2 + OSPFv3 in one plugin? | No | Guide §15: FRR ships two daemons; the LSA registries and wire formats differ enough that unification is net-negative. OSPFv3 is a separate edge plugin + umbrella |
| Transport? | `AF_INET SOCK_RAW` proto 89 + IP multicast, behind an interface | Reuse the RSVP-TE raw-IP pattern; add multicast membership; isolate so a BSD/VPP backend can be added later |
| LSDB storage model? | Lazy raw-bytes + metadata, per-area | Matches Ze buffer-first philosophy; per-area LSDBs are the FRR model and simpler than BIRD's single-LSDB-with-domain-filter for a first pass |
| Route installation mechanism? | Loc-RIB insertion (`locrib.Path`), NOT redistevents | The FIB-install path is Loc-RIB -> sysrib `OnChange` -> fibkernel, exactly as IS-IS and BGP. redistevents feeds the redistribute-orchestrator (redistribution to BGP), a different concern (ospf-10). Admin distance 110 on `locrib.Path.AdminDistance` (config `rib.admin-distance.ospf`, the EXISTING leaf) |
| ECMP at sysrib? | Reuse the existing path-group expansion | The sysrib `BestChangeEntry.ECMPPaths` expansion already exists (added by IS-IS). OSPF inserts one Path per equal-cost nexthop and gets kernel ECMP for free. Default cap 8 |
| Area binding model? | Per-interface | Guide §10: per-interface `area` binding rather than FRR's legacy `network <prefix> area <id>` matching; clearer, matches RFC 9129, avoids accidental over-matching |
| Network types in v1? | Broadcast + point-to-point | User decision: DR/BDR election and Network-LSAs in scope (ospf-5); NBMA/P2MP future |
| Area types in v1? | Normal + stub + NSSA | User decision: full hierarchy; ABR Type 3/4 (ospf-9), stub + NSSA Type 7 + translator (ospf-11) |
| Authentication timing? | v1 | User decision: AuType 0/1/2 codec in ospf-2, key mgmt + per-packet verify/sign + RFC 7474 trailer in ospf-12 |

## Required Reading

### Architecture Docs
- [ ] `docs/research/ospf-implementation-guide.md` - protocol, packets, LSAs, ISM/NSM, DR election, flooding (§13), SPF (§16), areas/ABR/ASBR/NSSA, auth, checksum traps, concurrency, phased order, ze layout
  -> Decision: follow the phased order (§16); adopt the per-packet-type + per-LSA-family file split (BIRD middle path between FRR's monoliths)
  -> Constraint: two distinct checksums (packet IP one's-complement excluding the auth field; LSA Fletcher-16 excluding LS Age); the §13.4 hard traps (Fletcher, sequence compare, MaxAge retention, MTU mismatch, Network-LSA LS ID, two-way check, E1/E2 ordering, ABR acceptance, NSSA translator election, zeroed-checksum auth, sticky DR)
- [ ] `plan/learned/926-isis-0-umbrella.md` - the sibling IS-IS umbrella; OSPF reuses the SAME route-install / redistribution / edge-plugin / sysrib infra
  -> Constraint: copy the Loc-RIB install, redistribution source/consumer, plugin lifecycle, and metrics-table conventions verbatim; do NOT couple OSPF to IS-IS code
- [ ] `docs/architecture/core-design.md` - plugin registration, event bus, lifecycle
  -> Constraint: OSPF registers like LDP/RSVP-TE/IS-IS; runtime via SDK OnConfigure/OnStarted/OnExecuteCommand
- [ ] `ai/rules/buffer-first.md`, `ai/rules/memory-architecture.md` - zero-copy, lazy parse, no-alloc hot path
  -> Constraint: LSDB stores raw LSA bytes; body parse on demand; encode is buffer-first `WriteTo(buf, off) int`
- [ ] `ai/rules/plugin-self-containment.md`, `ai/rules/registration-dispatch.md` - self-contained plugin, registration not switch
  -> Constraint: all OSPF commands/schema/help/doctor live under `internal/plugins/ospf/`
- [ ] `ai/rules/config-surface.md`, `ai/rules/config-naming.md` - YANG vs env var, kebab-case
  -> Constraint: OSPF config is YANG (`ze-ospf-conf.yang`), top-level `ospf` container, kebab-case leaves

### RFC Summaries (MUST for protocol work; existing, read before implementation)
- [ ] `rfc/short/rfc2328.md` - OSPF Version 2 (base): packets, LSAs, ISM/NSM, DR election, §13 flooding, §16 SPF/inter-area/external, stub areas, AuType 0/1/2
- [ ] `rfc/short/rfc905.md` - Fletcher-16 checksum (Annex B)
- [ ] `rfc/short/rfc1071.md` - Internet (IP) one's-complement checksum
- [ ] `rfc/short/rfc3101.md` - NSSA (Type 7, translator election, P-bit)
- [ ] `rfc/short/rfc5709.md`, `rfc/short/rfc7474.md` - HMAC-SHA auth + auth trailer
- [ ] `rfc/short/rfc6987.md` - Stub Router Advertisement / max-metric Router-LSA (ospf-7 origination, ospf-13 config + CLI)

**Key insights:**
- OSPF runs over IP (proto 89), multicast `224.0.0.5` (AllSPFRouters) / `224.0.0.6` (AllDRouters), TTL 1; needs `CAP_NET_RAW`. The transport is the in-tree RSVP-TE raw-IP pattern PLUS multicast membership -- no new low-level capability
- FIB install = Loc-RIB insertion (`locrib.Path`) -> sysrib -> fibkernel (like IS-IS/BGP); redistevents is redistribution-to-BGP only. SPF decides what to insert. The sysrib ECMP path-group expansion already exists
- Two distinct checksums and the §13.4 hard traps are where correctness risk concentrates; test them against RFC vectors and FRR wire output early
- Multi-area / ABR / stub / NSSA and authentication are all in v1 scope; OSPFv3 is a separate edge plugin (deferred)

## Current Behavior (MANDATORY)

**Source files read:** (architecture survey; per-child specs read their own targets)
- [ ] Ze has no OSPF protocol. The closest in-tree artefacts are RSVP-TE's raw-IP transport (`internal/plugins/rsvpte/transport_linux.go`, the proto-89 model) and the IS-IS component (`internal/plugins/isis/`, the above-the-wire pattern source)
  -> Constraint: OSPF builds a new component; it copies IS-IS's above-the-wire patterns but shares no code
- [ ] FIB install path is Loc-RIB insertion -> `sysrib` `OnChange` -> `fibkernel`; the ECMP path-group expansion (`BestChangeEntry.ECMPPaths`) already exists; `redistevents` feeds the redistribute-orchestrator and is redistribution-only
  -> Constraint: OSPF inserts `locrib.Path` with `AdminDistance` 110; OSPF resolves intra/inter/E1/E2 preference internally before publishing one Path per prefix
- [ ] The `ospf` admin-distance leaf already exists in `ze-rib-conf.yang` (default 110)
  -> Constraint: OSPF reuses the existing leaf; no new sysrib config

**Behavior to preserve:**
- BGP, LDP, RSVP-TE, IS-IS, static, connected route sources remain independent and functional
- Loc-RIB / sysrib / fibkernel arbitration semantics unchanged (OSPF is just another source)
- RSVP-TE raw-socket code unchanged (OSPF adds a sibling transport, does not refactor RSVP-TE in place unless a shared raw-IP helper is extracted cleanly)

**Behavior to change:**
- New top-level `ospf` config container and `internal/plugins/ospf/` component
- A new redistribution source/consumer pair for OSPF
- No sysrib changes (admin-distance leaf and ECMP expansion already present)

## Data Flow (MANDATORY)

### Entry Point
- OSPF packets (Hello/DD/LS Request/LS Update/LS Ack) arrive on enabled interfaces via the raw IP socket joined to `224.0.0.5`/`224.0.0.6` (ospf-3)
- Config arrives as the `ospf` subtree of the YANG-validated config tree (ospf-4)
- Connected/static/BGP routes arrive via the redistribution consumer (ospf-10)

### Transformation Path
1. **Receive:** raw datagram -> strip IP header -> common-header parse + validate (version/area/checksum/auth) -> dispatch by Type (ospf-3, ospf-2, ospf-4)
2. **Interface/Hello:** Hello -> ISM events + DR/BDR election -> neighbour discovery (ospf-5)
3. **Adjacency:** NSM (DD master/slave, LS Request drain) -> Full (ospf-6)
4. **LSDB:** LS Update -> §13.1 freshness compare -> store raw + metadata -> flood + retransmit + delayed ack (ospf-7)
5. **SPF:** LSDB change -> debounce/throttle -> two-stage Dijkstra per area + inter-area + external -> routes with nexthop+metric+path-type (ospf-8/9/10/11)
6. **Install:** insert `locrib.Path` (Source = OSPF ProtocolID, Instance per ECMP nexthop, AdminDistance 110) -> Loc-RIB best-path -> sysrib `OnChange` -> fibkernel -> kernel (`RTPROT_ZE`) (ospf-8)
7. **Redistribute (separate path, not FIB install):** OSPF intra/inter routes -> `redistevents` -> redistribute-orchestrator -> BGP `RedistConsumer`; connected/static/BGP -> OSPF Type 5 AS-External LSAs via consumer (ospf-10)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire <-> OSPF engine | raw `AF_INET SOCK_RAW` proto 89 datagrams, IP multicast | [ ] |
| Engine <-> Loc-RIB (FIB install) | `locrib.Path` insertion (Source/Instance/AdminDistance/Metric) | [ ] |
| Engine <-> redistribution | `redistevents.RouteChangeBatch` (value-typed, pooled) to orchestrator | [ ] |
| sys-rib <-> kernel | existing best-change -> fibkernel netlink (`RTPROT_ZE`) | [ ] |
| OSPF <-> BGP | redistribute source/consumer registries | [ ] |
| Config tree <-> engine | SDK OnConfigure/OnConfigApply (JSON subtree) | [ ] |

### Integration Points
- New component `internal/plugins/ospf/` (ospf-4)
- Loc-RIB insertion for FIB install (ospf-8); redistevents producer + redistribute source/consumer (ospf-10)
- sysrib admin-distance (existing `ospf` leaf) + existing ECMP expansion
- iface EventBus link/address up/down (ospf-3, ospf-5, ospf-7)
- CLI/web/metrics/doctor (ospf-13)

### Architectural Verification
- [ ] No bypassed layers (datagram -> codec -> engine -> Loc-RIB insertion -> sysrib -> fib; redistevents only for redistribution)
- [ ] No unintended coupling (OSPF independent of IS-IS; transport behind an interface)
- [ ] No duplicated functionality (route install reuses sysrib; no second FIB path; ECMP reuses the existing expansion)
- [ ] Zero-copy preserved (LSDB raw bytes; buffer-first encode)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | RSVP-TE's `AF_INET SOCK_RAW` pattern generalises to OSPF proto 89 with IP multicast membership added | `rsvpte/transport_linux.go` exploration | Transport needs a different socket mechanism (e.g. `IP_HDRINCL`, `PKTINFO`, per-interface fan-out) | ospf-3 prototype multicast send/recv on veth | unvalidated |
| A-2 | FIB install via Loc-RIB insertion (`locrib.Path`) works for OSPF unchanged, and the existing sysrib ECMP path-group expansion handles OSPF equal-cost nexthops | sysrib/locrib code read; IS-IS precedent | ECMP collapses or admin distance misapplies for a second IGP | ospf-8 end-to-end kernel ECMP test | unvalidated |
| A-3 | A single OSPF admin distance (110) on `locrib.Path.AdminDistance` suffices; intra/inter/E1/E2 preference is resolved inside OSPF SPF (locrib.Path has no path-type field) | `ze-rib-conf.yang:42`, `locrib/candidate.go` | Per-path-type distance vs other protocols needs a locrib.Path protoType field (future) | ospf-8 multi-source + path-type test | unvalidated |
| A-4 | IP multicast receive on a raw socket works on Linux veth/bridge with `IP_ADD_MEMBERSHIP` and no promiscuous mode | guide §9, RSVP-TE precedent (unicast only) | Need `PACKET`-level membership or `IP_MULTICAST_ALL` tuning | ospf-3 functional multicast test | unvalidated |
| A-5 | `make generate` discovers a new `component/ospf` + `yang` package automatically | LDP/RSVP-TE/IS-IS precedent | Manual `all.go` edit needed (forbidden to hand-edit generated) | ospf-4 build after generate | unvalidated |
| A-6 | Per-area LSDBs (FRR model) are sufficient; no single-LSDB-with-domain-filter (BIRD model) is needed | guide §11 takeaways | Cross-area flooding/lookup awkwardness | ospf-7/ospf-9 multi-area tests | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Fletcher-16 (LSA) or IP one's-complement (packet) checksum implemented wrong, or wrong covered range | round-trip/interop checksum failures | RFC 905 / RFC 1071 vector tests in ospf-1 before any runtime; covered-range tests in ospf-2 |
| R-2 | LS sequence-number comparison / MaxAge purge retention mishandled causes flap or loop | soak/chaos LSA flap; routes never withdraw | Explicit §13.1 freshness + §14 MaxAge-retention tests in ospf-7 (traps #2, #3) |
| R-3 | SPF two-way check omitted -> phantom routes over half-up adjacencies | routes via a neighbour that does not list us back | Two-way-check test in ospf-8 (trap #6) |
| R-4 | DR/BDR election churn (non-sticky) on a flapping LAN | repeated Network-LSA re-origination | Sticky-DR rule + election tests in ospf-5 (trap #12) |
| R-5 | DD MTU mismatch handling wrong -> adjacency stuck in ExStart/Exchange | adjacency never reaches Full with a larger-MTU peer | MTU-field check + `mtu-ignore` option + test in ospf-6 (trap #4) |
| R-6 | NSSA translator election / Type 7->5 translation wrong -> duplicate or missing externals | duplicate Type 5 on the backbone, or NSSA externals invisible | RFC 3101 §3.5 election + P-bit tests in ospf-11 (trap #9) |
| R-7 | E1 vs E2 external ordering / forwarding-address resolution wrong | suboptimal or black-holed external routes | §16.4 ordering tests in ospf-10 (trap #7) |
| R-8 | Cryptographic auth with the packet checksum NOT zeroed | auth always fails against FRR | Zeroed-checksum handling + interop test in ospf-12 (trap #10) |
| R-9 | Scope creep into opaque/TE/SR/virtual-links | spec churn | Hard out-of-scope table above; no opaque consumers in v1 |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| config `ospf { ... }` present | -> | OSPF component starts, opens the raw socket, enrols interfaces | `TestOSPFComponentStart` (ospf-4) |
| Hello received on an interface | -> | neighbour discovered, ISM/NSM advance | `TestOSPFNeighborInit` (ospf-5), `TestOSPFAdjacencyFull` (ospf-6) |
| adjacency Full on two nodes | -> | LSAs exchanged, LSDB synced | `TestOSPFLSDBSync` (ospf-7) |
| LSDB populated | -> | SPF runs, route emitted | `TestOSPFSPFRoute` (ospf-8) |
| SPF route emitted | -> | sysrib best-change -> kernel route (`RTPROT_ZE`) | `test/ospf/ospf-route-install.ci` (ospf-8) |

## Acceptance Criteria

(Umbrella-level; each child spec carries its own detailed ACs.)

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Two Ze nodes on a point-to-point link, OSPF area 0 configured | Adjacency reaches Full; each installs the other's prefixes |
| AC-2 | Three Ze nodes on a broadcast LAN | A DR and BDR are elected; a Network-LSA represents the segment; all reach Full with the DR/BDR |
| AC-3 | Line topology with prefixes on every router | SPF converges; each node installs every prefix in the kernel with OSPF as source and the correct next-hop |
| AC-4 | Two areas joined by an ABR, prefixes in each | Each area sees the other's prefixes as inter-area routes (Type 3) with the ABR as next-hop |
| AC-5 | ASBR redistributes a static route | Every other router learns it as an AS-External route with correct E1/E2 metric and forwarding address |
| AC-6 | Stub area + NSSA configured | Stub: no Type 5, default present. NSSA: internal redistribution -> Type 7, translator converts to Type 5 on the backbone |
| AC-7 | Authentication configured | Packets without/with the wrong key are rejected; correct key forms the adjacency; key rotation is hitless |
| AC-8 | `redistribute { destination bgp { import ospf } }` | OSPF routes appear in BGP |
| AC-9 | `redistribute { destination ospf { import connected } }` | Connected prefixes appear as Type 5 AS-External LSAs and in peers' RIBs |
| AC-10 | Link/neighbour lost (dead interval) | Adjacency down, Router/Network-LSA re-originated, routes withdrawn from the kernel |
| AC-11 | Interop with FRR `ospfd` | Adjacency, LSDB sync, route convergence, multi-area, stub, NSSA, and auth all work |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures OSPF on two linked nodes | config -> component -> interface -> Hello -> ISM/NSM -> Full | `TestOSPFAdjacencyFull`, `test/ospf/ospf-adjacency.ci` |
| 2 | Expects remote prefixes in the kernel FIB | LSDB -> SPF -> Loc-RIB insertion (`locrib.Path`) -> sysrib `OnChange` -> fibkernel -> kernel (NOT `redistevents`) | `test/ospf/ospf-route-install.ci` |
| 3 | Redistributes OSPF into BGP | OSPF route -> source registry -> BGP consumer -> BGP RIB | `test/ospf/ospf-redist-bgp.ci` |
| 4 | Runs `show ip ospf neighbor` / `database` / `route` | CLI -> RPC -> engine snapshot | `test/ospf/ospf-show.ci` |
| 5 | Meshes with an FRR router across two areas | full protocol over the wire | `test/interop/scenarios/ospf-*-frr` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (per child) | `internal/plugins/ospf/...` | see child specs ospf-1..ospf-13 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Router ID / Area ID | 4 bytes (dotted-quad) | n/a | <4 | >4 |
| LS Sequence Number | 0x80000001..0x7FFFFFFF (signed) | 0x7FFFFFFF | 0x80000000 (reserved) | wraps -> flush (MaxAge) then re-originate at InitialSequenceNumber |
| LS Age | 0..3600 s (MaxAge); DoNotAge bit 0x8000 | 3600 | n/a | >3600 treated as MaxAge |
| Router-LSA / Network metric | 0..65535 (16-bit) | 65535 | n/a | 65536 |
| Summary / External metric | 0..16777215 (24-bit) | 16777215 | n/a | 16777216 |
| DR priority | 0..255 | 255 | n/a | 256 |
| Hello / Dead interval | 1..65535 s | 65535 | 0 | 65536 |
| Interface MTU (DD) | 0..65535 | 65535 | n/a | 65536 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-adjacency` | `test/ospf/ospf-adjacency.ci` | two nodes reach Full | |
| `ospf-route-install` | `test/ospf/ospf-route-install.ci` | remote prefix installed in the kernel | |
| `ospf-redist-bgp` | `test/ospf/ospf-redist-bgp.ci` | OSPF route into BGP | |
| `ospf-show` | `test/ospf/ospf-show.ci` | show commands render | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-p2p-frr` | `test/interop/scenarios/` | FRR ospfd | P2P adjacency + route convergence | |
| `ospf-broadcast-frr` | `test/interop/scenarios/` | FRR ospfd | LAN DR/BDR election + Network-LSA | |
| `ospf-multiarea-frr` | `test/interop/scenarios/` | FRR ospfd | Inter-area (Type 3/4) routing | |
| `ospf-stub-nssa-frr` | `test/interop/scenarios/` | FRR ospfd | Stub filtering + NSSA Type 7->5 | |
| `ospf-auth-frr` | `test/interop/scenarios/` | FRR ospfd | MD5 / HMAC-SHA authentication | |

### Future (if deferring any tests)
- Opaque/TE/SR/virtual-links/GR/BFD interop deferred with the corresponding out-of-scope features

## Files to Modify
- `internal/component/plugin/all/all.go` - regenerated by `make generate` to import OSPF (ospf-4)
- `internal/component/config/redistribute/...` - register OSPF source/consumer (ospf-10)
- `internal/test/cli/register.go`, `mk/test-functional.mk` - register the `test/ospf` suite (ospf-4)
- `docs/comparison.md`, `docs/features.md` - OSPF support row (ospf-13)
- NOTE: NO sysrib changes -- the `ospf` admin-distance leaf and the ECMP path-group expansion already exist

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | Yes | `internal/plugins/ospf/yang/ze-ospf-conf.yang` |
| YANG validation constraints | Yes | range/pattern/enum on every leaf (router-id/area-id dotted-quad pattern, area-type enum, network-type enum, interval ranges, priority range) |
| YANG custom validators | Yes | router-id / area-id validators with `CompleteFn` |
| CLI commands/flags | Yes | `show ip ospf [neighbor\|interface\|database\|route\|border-routers\|spf]`, `clear ip ospf` |
| CLI grammar (action before identifier) | Yes | `ai/rules/cli-grammar.md` |
| Editor autocomplete | Yes | YANG enum/type driven + custom `CompleteFn` |
| Functional test for new RPC/API | Yes | `test/ospf/*.ci` |
| Pipe completeness | Yes | show output through `ApplyPipes`/`ProcessPipes` |
| Doctor check for runtime dependencies | Yes | `CAP_NET_RAW`, raw socket open (ospf-3) |
| Prometheus counters/metrics | Yes | neighbours, LSAs, SPF runs, auth failures, flooding (canonical table above) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`, `docs/plugin-overview.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/ospf.md` |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/ospf.md` |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc2328.md` and siblings |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (new `test/ospf/`) |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` (new component) |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin/event/command/capability changed? | Yes | `docs/plugin-overview.md`, `docs/guide/status.md` |
| 16 | Changed files referenced by doc source anchors? | No | grep at completion |
| 17 | Existing docs show examples for this area? | No | grep at completion |

## Files to Create
- `internal/plugins/ospf/` - the OSPF edge plugin (subpackages per architecture layout)
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` - config schema
- `test/ospf/*.ci` - functional tests
- `test/interop/scenarios/ospf-*-frr/` - interop scenarios
- (none) OSPFv2 RFC summaries already exist under `rfc/short/`; update them only for source-verified errata
- `docs/guide/ospf.md`, `docs/architecture/wire/ospf.md` - user + wire docs
- `plan/spec-ospf-1-*.md` .. `plan/spec-ospf-13-*.md` - child specs
- `plan/spec-ospfv3-0-umbrella.md` - the deferred OSPFv3 follow-up umbrella

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + the selected child |
| 2. Audit | Per-child Files/TDD |
| 3. Wiring phase | Per-child Wiring Test |
| 4. Implement (TDD) | Per-child Implementation Phases |
| 5. /ze-review gate | Per-child Review Gate |
| 6-14. | Standard flow |

### Implementation Phases

This umbrella is implemented by selecting and completing child specs in
dependency order. Per the spec-set rule, select children individually when
implementing; keep the umbrella pointed-to but do not implement the umbrella
directly.

1. **Phase: Foundations (parallel)** - ospf-1 (types+checksums), ospf-2 (wire), ospf-3 (IP transport)
2. **Phase: Wiring backbone** - ospf-4 (plugin/config/scaffolding); MANDATORY before runtime specs and depends on both wire and transport
3. **Phase: Interface + adjacency** - ospf-5 (ISM/Hello/DR election), ospf-6 (NSM/DD)
4. **Phase: LSDB + flooding** - ospf-7
5. **Phase: SPF + sys-rib** - ospf-8 (delivers the core goal: kernel routes from OSPF)
6. **Phase: Multi-area then external** - ospf-9 (inter-area/ABR) before ospf-10 (AS-external/ASBR/redistribution)
7. **Phase: Auth and area policy** - ospf-12 after ospf-7; ospf-11 after ospf-9/10
8. **Phase: CLI/diag/interop** - ospf-13 after ospf-1 through ospf-12
9. **Full verification + interop** - `make ze-verify` + FRR scenarios

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every child spec exists and cross-references siblings |
| Correctness | Wire matches RFC 2328/3101; both checksums correct (covered ranges); the §13.4 traps handled |
| Naming | YANG kebab-case; CLI `show ip ospf <noun>`; admin-distance key `ospf` (single, existing) |
| Data flow | FIB install flows OSPF SPF -> Loc-RIB insertion (`locrib.Path`) -> sysrib `OnChange` -> fibkernel (NOT `redistevents`); redistribution flows OSPF -> `redistevents` -> orchestrator -> BGP consumer (never the FIB); no bypass and no conflation |
| Rule: plugin-self-containment | All OSPF schema/help/doctor/commands under `internal/plugins/ospf/` |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Umbrella + 13 child specs + v3 follow-up | `ls plan/spec-ospf-*.md plan/spec-ospfv3-0-umbrella.md` |
| OSPF edge plugin | `ls internal/plugins/ospf/` |
| Functional + interop tests | `ls test/ospf/ test/interop/scenarios/ospf-*` |
| RFC summaries | `ls rfc/short/rfc2328.md` and siblings |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Every packet/LSA length validated before slicing; bound checks on the receive path; LSA length vs claimed length |
| Authentication | AuType verification (ospf-12); reject on mismatch; constant-time compare; zeroed-checksum handling |
| Resource exhaustion | Max LSA/neighbour/retransmit limits; LSDB size cap; flood rate limiting (MinLSArrival/MinLSInterval) |
| Privilege | `CAP_NET_RAW` only; drop after socket open if feasible |
| Spoofing | Area-id / neighbour checks; sequence/age sanity; reject self-originated LSAs with a higher sequence than ours (flush-and-reoriginate) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test setup |
| Test fails behavior mismatch | Re-read RFC summary / Current Behavior |
| Interop mismatch | Capture with tcpdump, compare to FRR, fix codec/FSM |
| 3 fix attempts fail | STOP; report; ask user |

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
- The "ensure sys-rib is updated" requirement is solved by existing Loc-RIB/sysrib infra (the same path IS-IS uses), INCLUDING the ECMP path-group expansion IS-IS already added; OSPF adds no sysrib code.
- OSPF is unusual among Ze's protocols in introducing NO new low-level I/O capability: the raw-IP transport already exists (RSVP-TE). The novelty is entirely protocol logic.

## Core Insight
OSPF in Ze is "a large, dense protocol on top of zero new infrastructure." Every
integration primitive (raw-IP transport, Loc-RIB install + ECMP, redistribution,
component/config, admin distance) already exists in-tree. The risk and novelty
concentrate in the protocol itself: two checksums, the ISM/NSM pair, the §13
flooding procedure, the §16 two-stage SPF with the two-way check, and multi-area
ABR/ASBR/NSSA route computation. Correctness is won by testing the §13.4 hard
traps against RFC vectors and FRR wire output early.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Native Go OSPFv2 | Wrap FRR ospfd | Consistent with native BGP/IKEv2/IS-IS; no subprocess on gokrazy |
| Separate from IS-IS | Shared SPF/LSDB engine | Network vertices vs pseudo-nodes; different LSA/LSP keys and metric semantics (guide §11) |
| OSPFv2 only; v3 separate | Unified v2/v3 component | Different wire/LSA registry; FRR split into two daemons (guide §15) |
| Per-area LSDB | Single LSDB with domain filter (BIRD) | Simpler first pass; FRR model |
| Producer into existing sys-rib | New OSPF-specific FIB path | Reuse Loc-RIB/sysrib/fibkernel arbitration + existing ECMP expansion; one FIB path |
| Transport behind an interface | Inline raw socket in the engine | Future BSD/VPP backends; testability |

## Known Limitations
- v1 ships broadcast + point-to-point network types only (NBMA/P2MP future)
- Opaque-LSA framework, TE, SR, virtual links, GR, BFD, demand circuits, multi-instance, L3VPN DN bit, SNMP MIB are out of scope (future)
- OSPFv3 (IPv6) is a separate edge plugin (`spec-ospfv3-0-umbrella.md`)

## RFC Documentation
Add `// RFC 2328 Section X.Y: "<quoted requirement>"` (and RFC 3101/5709/7474/6987 as applicable) above enforcing code in each child.

## Implementation Summary

### What Was Implemented
- Pending: fill as child specs are implemented and verified.

### Bugs Found/Fixed
- Pending: fill after implementation or review produces concrete evidence.

### Documentation Updates
- Pending: fill after implementation or review produces concrete evidence.

### Deviations from Plan
- Pending: fill after implementation or review produces concrete evidence.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Native OSPFv2 link-state IGP (RFC 2328 + RFC 3101 + RFC 5709/7474) | (pending) | `internal/plugins/ospf/` | spec set authored; implementation pending |
| Mesh over OSPF (adjacency, broadcast + P2P) | (pending) | `internal/plugins/ospf/iface`, `neighbor` | |
| Compute shortest paths | (pending) | `internal/plugins/ospf/spf` | |
| Keep sys-rib updated so the kernel FIB forwards | (pending) | `internal/plugins/ospf/spf/install` | |
| Interoperate with BGP via redistribution (both directions) | (pending) | `internal/plugins/ospf/redistribute` | |
| Multi-area + ABR + stub + NSSA | (pending) | `internal/plugins/ospf/spf` (ia/ase/nssa) | |
| Authentication in v1 (AuType 1/2 + HMAC-SHA trailer) | (pending) | `internal/plugins/ospf/packet/auth` | |
| Raw IP transport (proto 89 + multicast) | (pending) | `internal/plugins/ospf/transport` | |
| Plugin registration + YANG config + lifecycle | (pending) | `internal/plugins/ospf/register.go`, `config.go`, `yang/` | |
| 13 child specs in dependency order + v3 follow-up | Done | `plan/spec-ospf-1-*.md` .. `plan/spec-ospf-13-*.md`, `plan/spec-ospfv3-0-umbrella.md` | spec-set authoring deliverable |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1..AC-11 | (pending implementation) | per child spec | umbrella ACs map to child ACs |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| (per child) | (pending) | `internal/plugins/ospf/...`, `test/ospf/...` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `plan/spec-ospf-1-*.md` .. `plan/spec-ospf-13-*.md` | (filled by authoring) | spec set |
| `plan/spec-ospfv3-0-umbrella.md` | (filled by authoring) | v3 follow-up |
| `internal/plugins/ospf/` | (pending) | implementation |

### Audit Summary
- **Total items:** spec-set authoring (this deliverable) + downstream implementation
- **Done:** spec set authored (umbrella + 13 children + v3 follow-up)
- **Partial:** 0
- **Skipped:** 0
- **Changed:** implementation is downstream work, tracked per child

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| OSPFv2 spec set exists and is internally consistent | spec files + cross-references | `ls plan/spec-ospf-*.md` (umbrella + 13); dependency graph + Shared Contracts referenced by children |
| Native OSPFv2 engine (downstream) | build + unit (`-race`) | (filled during implementation) |
| Mesh over OSPF, SPF, sys-rib install, redistribution, multi-area, NSSA, auth | unit + functional + interop | (filled during implementation per child) |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | pending | `/ze-review` on the spec set has not run after these amendments | `plan/spec-ospf-*.md` | run before implementation begins |

### Fixes applied
- Pending: fill after implementation or review produces concrete evidence.

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
| `plan/spec-ospf-0-umbrella.md` | (verify) | `ls plan/spec-ospf-0-umbrella.md` |
| `plan/spec-ospf-1-*.md` .. `plan/spec-ospf-13-*.md` | (verify) | `ls plan/spec-ospf-*.md` |
| `plan/spec-ospfv3-0-umbrella.md` | (verify) | `ls plan/spec-ospfv3-0-umbrella.md` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| (spec-set) | each child carries its own ACs | per-child AC tables |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| (downstream) | `test/ospf/*.ci` | filled during implementation |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1..A-6 | unvalidated (design-time) | validated during child implementation per the Assumptions table |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| (downstream) | docs updated per child | filled during implementation |

## Checklist

### Goal Gates (MUST pass)
- [ ] All 13 child specs written and cross-referenced
- [ ] v3 follow-up umbrella written
- [ ] AC-1..AC-11 demonstrated across children (downstream)
- [ ] End-to-End User Stories each have a working path + passing test (downstream)
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes (downstream)
- [ ] Feature code integrated (`internal/plugins/ospf/`) (downstream)
- [ ] Integration completeness proven end-to-end (downstream)
- [ ] Documentation Update Checklist answered with source evidence (downstream)
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added (downstream)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (no shared OSPF/IS-IS engine in v1)
- [ ] No speculative features (out-of-scope table honoured)
- [ ] Single responsibility per child
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written (downstream)
- [ ] Tests FAIL (paste output) (downstream)
- [ ] Tests PASS (paste output) (downstream)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features
- [ ] Goal Validation table filled

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled (downstream)
- [ ] Implementation Audit filled
- [ ] Mistake Log escalation reviewed
- [ ] Write learned summary to `plan/learned/NNN-ospf-0-umbrella.md` (at set completion)
- [ ] Summary included in commit
