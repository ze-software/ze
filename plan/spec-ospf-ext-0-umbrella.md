# Spec: ospf-ext-0 -- OSPF Extensions (Umbrella, both address families)

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-ospf-0-umbrella.md (closed, learned 1114), spec-ospfv3-0-umbrella.md (retired, unified into the ospf namespace, commit ee1f6ddbe, no own learned summary) |
| Phase | - |
| Updated | 2026-06-24 |

Roadmap status (2026-07-22 plan review): this authoring umbrella is 15/16
delivered -- children ext-1..ext-16 all have learned summaries EXCEPT ext-13
(`spec-ospf-ext-13-l3vpn-dn-bit.md`, the only child spec still on disk,
correctly recorded as VRF-blocked for its AC-7..AC-12 half). Candidate for
closure once ext-13's ownership is settled (close the umbrella and let ext-13
stand alone, or keep the umbrella open for it).

> Umbrellas are living tracking documents, not implementable specs. This file
> coordinates the OSPF extension follow-ups for the SINGLE unified `ospf` engine
> across BOTH its address families (IPv4 / OSPFv2 RFC 2328 and IPv6 / OSPFv3 RFC
> 5340). It carries NO acceptance criteria and NO feature code of its own. Each
> child spec listed below is the implementable unit. The "implementable-spec"
> sections below (Current Behavior, Data Flow, Wiring Test, TDD Test Plan,
> Checklist) are framed at umbrella level -- they describe the child set, not a
> feature this file implements. Do not mark this umbrella "done": it closes only
> when every child it tracks is closed (or explicitly re-rested).
>
> **This umbrella SUPERSEDES the retired `spec-ospfv3-ext-0-umbrella.md`.** OSPF
> is ONE engine with address families, exactly like `bgp` is one engine spanning
> address families: there is no `bgpv4`, and there is no separate `ospfv3`
> plugin. IPv4 and IPv6 are two ADDRESS FAMILIES of the one OSPF. The features
> below are features of OSPF; each row records which address family it covers.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/learned/972-ospf-af-unify.md` -- the delivered unified-engine decision: ONE `ospf` engine with address-family-aware seams (Transport, Codec, AFPrefixStrategy). The FSM, LSDB sequencing, flooding, DR election, and SPF are AF-NEUTRAL and SHARED between IPv4 and IPv6; only the wire format, LSA registry, prefix model, and transport are AF-specific. Do NOT create a second OSPFv3 engine; there is no separate `ospfv3` plugin (no `ospfv3` directory under `internal/plugins/`)
4. `plan/spec-ospf-0-umbrella.md` -- the DELIVERED IPv4 (OSPFv2) base umbrella; its "Shared Contracts", "Out of scope (future...)" table, the LSA inventory, the metric contract, and the FIB-install-vs-redistribution split. The base is the stable foundation; every IPv4 extension builds on it without re-opening it
5. `plan/spec-ospfv3-0-umbrella.md` -- the DELIVERED IPv6 (OSPFv3) base umbrella; the RFC 5340 codec/transport leaves under `internal/plugins/ospf/v3/{types,packet,transport}`, the scope-aware LSDB, the reserved Instance-ID plumbing, and the RFC 7166 auth trailer
6. `plan/spec-ospf-ext-1-opaque-framework.md` -- the first WRITTEN child (RFC 5250 opaque carrier, IPv4); the ext-family voice and the registration API the IPv4 opaque consumers (ext-2/ext-3/ext-4) and ext-9 (IPv4 GR) plug into
7. `docs/research/ospf-implementation-guide.md` §14 (FRR feature catalogue) and §15 (OSPFv3 differences, REVISED: unified address-family-aware engine) -- the extension landscape (opaque, TE, RI, SR, TI-LFA, GR, BFD, LDP-IGP sync, multi-instance, L3VPN DN-bit, multi-AF, IPsec) and the rested/deferred rationale this umbrella records
8. `internal/plugins/ospf/` -- the delivered unified engine the extensions extend: the shared AF-neutral subsystems (`lsdb/`, `spf/`, `neighbor/`, `iface/`, `transport/`), the IPv4 `packet/` codec, the IPv6 `_v6` strategy files (`afstrategy_v6.go`, `codec_v6.go`, `encoder_v6.go`, `origination_v6*.go`, `nssa.go`), the IPv6 codec/transport leaves under `internal/plugins/ospf/v3/{types,packet,transport}`, the RFC 7166 auth trailer (`auth_keystore.go`, `auth_wiring.go`), the reserved Instance-ID demux (`dispatcher.go`), and the lifecycle (`register.go`, `instance.go`)

## Task

Coordinate the OSPF extension follow-ups that the two delivered base umbrellas
(`plan/spec-ospf-0-umbrella.md` for IPv4 and `plan/spec-ospfv3-0-umbrella.md`
for IPv6) deliberately rested. Both bases are delivered inside the SINGLE unified
`ospf` engine (`internal/plugins/ospf/`): the IPv4 base delivers a complete,
interoperable OSPFv2 (multi-area / ABR / stub / NSSA, broadcast + point-to-point,
the full §13 flooding and §16 SPF, redistribution, authentication AuType
0/1/2/3); the IPv6 base delivers IPv6-unicast OSPFv3 (RFC 5340 codec and IPv6
transport in the `internal/plugins/ospf/v3/{types,packet,transport}` leaves,
scope-aware LSDB, ISM/NSM, SPF with prefix attachment, inter-area / external /
stub / NSSA, the RFC 7166 auth trailer, FRR `ospf6d` interop). The FSM, flooding,
DR election, SPF, and LSDB sequencing are the SHARED, AF-neutral engine
machinery; only the wire format, LSA registry, prefix model, and transport are
address-family-specific.

Neither base delivered: the IPv4 opaque-LSA carrier or any extension that rides
it (TE, Router Information, Extended Link/Prefix, IPv4 SR); the AF-neutral
forwarding features (TI-LFA/LFA, BFD, LDP-IGP sync) or the AF-neutral protocol
features (virtual links, NBMA/point-to-multipoint); Graceful Restart in either
family; IPv4 Multi-Instance; the IPv4 L3VPN PE-CE DN bit; the debug/introspection
surface; nor the two IPv6-only items the IPv6 base rested (RFC 5838 multiple
address families and RFC 4552 IPsec AH/ESP authentication).

This umbrella enumerates those follow-ups as a coordinated child set, fixes the
build order between them (the IPv4 opaque carrier is a hard prerequisite for
several IPv4 children; IPv4 SR sits on RI + Extended Link/Prefix; TI-LFA sits on
SR + SPF; IPv6 SR adds the v3 Router Information + RFC 8362 extended LSAs itself,
since there is no opaque carrier in v3), and records -- in the "Out of scope
(rested)" table -- the OSPF features that were considered and deliberately NOT
scheduled, each with its rationale, so a future agent does not silently assume
they are done or quietly re-add them.

### Scope boundary vs the delivered bases

| Concern | Owned by | This umbrella |
|---------|----------|---------------|
| IPv4 (OSPFv2) base protocol (areas, ABR/ASBR, stub/NSSA, §13 flooding, §16 SPF, broadcast + P2P, redistribution, auth 0/1/2/3) | `plan/spec-ospf-0-umbrella.md` (delivered) | STABLE foundation; not re-opened. IPv4 extensions attach at documented seams (LSDB stores, flooding chokepoints, SPF route table, NSM/DD, registry) |
| IPv6 (OSPFv3) base protocol (RFC 5340 codec + IPv6 transport leaves, scope-aware LSDB, ISM/NSM, SPF + prefix attachment, inter-area / external / stub / NSSA, RFC 7166 auth trailer, FRR interop) | `plan/spec-ospfv3-0-umbrella.md` (delivered) | STABLE foundation; not re-opened. IPv6 extensions attach at the `_v6` strategies, the scope-aware LSDB, flooding, SPF, ISM/NSM, the IPv6 transport leaf, the reserved Instance-ID field, the auth path |
| Shared AF-neutral engine machinery (FSM, flooding, DR election, SPF, LSDB sequencing, lifecycle) | both bases, unified per `plan/learned/972-ospf-af-unify.md` | STABLE; both-AF extensions (TI-LFA, BFD, LDP-IGP sync, virtual links, NBMA/P2MP, debug) attach here once and serve both families |
| IPv4 opaque-LSA carrier + the extensions that ride it (TE, RI, Extended Link/Prefix, IPv4 SR) | this umbrella (ext-1, ext-2, ext-3, ext-4, ext-5/IPv4 half) | Tracked + ordered here. IPv4-specific: v3 has no opaque carrier |
| AF-neutral / both-AF protocol + forwarding features (TI-LFA/LFA, virtual links, NBMA/P2MP, BFD, LDP-IGP sync, debug) | this umbrella (ext-6, ext-7, ext-8, ext-10, ext-11, ext-14) | Tracked + ordered here; the shared engine means one implementation covers both AFs |
| Graceful Restart (IPv4 RFC 3623 + IPv6 RFC 5187) | this umbrella (ext-9) | Tracked here; IPv4 Grace-LSA is a Type 9 opaque LSA (needs ext-1), IPv6 Grace-LSA is a native link-scope v3 LSA (no opaque carrier) |
| IPv6-only items the v3 base rested (RFC 5838 multi-AF, RFC 4552 IPsec) | this umbrella (ext-15, ext-16) | Promoted in scope here; both consume IPv6 base seams |
| IPv4 Multi-Instance (RFC 6549) | this umbrella (ext-12) | Tracked here; the OSPFv2 Instance ID field, distinct from the IPv6 Instance-ID demux the v3 base already reserved |
| IPv4 L3VPN PE-CE DN bit (RFC 4576/4577) | this umbrella (ext-13) | Tracked here; GATED on future MPLS-L3VPN/VRF infrastructure (blocking dependency, recorded below) |

### Target scope / decisions

| Lever | Decision | Effect on the child set |
|-------|----------|-------------------------|
| One engine, two address families | **No child forks a second engine; IPv4 and IPv6 are address families of the one `ospf`** | Per `plan/learned/972-ospf-af-unify.md`, the FSM, flooding, DR election, SPF, and LSDB sequencing are AF-neutral and shared. AF-specific wire/LSA/prefix code lives in the IPv4 `packet/` codec, the `_v6` strategies, and the `internal/plugins/ospf/v3/{types,packet,transport}` leaves. There is no separate `ospfv3` plugin directory |
| IPv4 opaque carrier first | **ext-1 (RFC 5250) is the IPv4 opaque foundation** | IPv4 TE (ext-2), IPv4 Router Information (ext-3), IPv4 Extended Link/Prefix (ext-4), the IPv4 Grace-LSA half of GR (ext-9), and the IPv4 opaque-decode half of debug (ext-14) all depend on the IPv4 opaque carrier. It is written first (Status:ready) and unblocks the IPv4 opaque chain |
| SR builds on the IGP advertisement layer | **ext-5 (IPv4) depends on ext-3 + ext-4; ext-5 (IPv6) adds the v3 RI + RFC 8362 extended LSAs itself** | RFC 8665 (IPv4) reuses the Router Information (RFC 7770) and Extended (RFC 7684) LSAs carried over the opaque framework. RFC 8666/8362 (IPv6) has no opaque carrier: SR adds the v3 Router Information LSA and the OSPFv3 Extended-LSA TLV containers (RFC 8362) as native scope-aware LSAs in the `v3/packet` leaf before it can compute labels |
| TI-LFA needs SR + SPF reachable | **ext-6 depends on ext-5 + the shared SPF** | TI-LFA / LFA (RFC 5286 + the TI-LFA draft) computes repair paths over the SR label stack and the shared SPF; once the shared SPF is wired, the repair logic serves both AFs |
| Both-AF features parallelise on the shared engine | **ext-6, ext-7, ext-8, ext-10, ext-11, ext-14 attach to shared, AF-neutral seams** | TI-LFA, virtual links, NBMA/P2MP, BFD, LDP-IGP sync, and debug touch the shared ISM/NSM/SPF/interface model; one implementation serves both families (with AF-specific record/transport differences where RFC 2328 and RFC 5340 diverge) |
| IPv4 Multi-Instance vs IPv6 Instance-ID | **ext-12 adds the OSPFv2 Instance ID field (IPv4); it is distinct from the v3 Instance-ID demux** | RFC 6549 adds an Instance ID to the OSPFv2 common header for per-instance demux on a shared interface. The IPv6 base already reserved its own Instance-ID field (consumed by ext-15 for multi-AF); ext-12 is the IPv4 analogue, not a re-use |
| L3VPN DN bit is VRF-gated | **ext-13 last, blocked on VRF infra (IPv4)** | The PE-CE DN-bit loop prevention (RFC 4576/4577) requires MPLS-L3VPN / VRF infrastructure Ze does not yet have; ext-13 is recorded as BLOCKED until that lands, not merely "later" |
| Multi-AF uses the reserved IPv6 Instance-ID | **ext-15 (RFC 5838) consumes the v3 base's Instance-ID plumbing** | The IPv6 base reserved an explicit, validated Instance-ID field precisely so multi-AF could attach later. ext-15 maps AF to the Instance-ID ranges (RFC 5838 §2.4) and spawns one engine instance per AF without re-opening the RFC 5340 codec |
| IPsec auth is independent of the trailer | **ext-16 (RFC 4552) is a separate auth path from the delivered RFC 7166 trailer (IPv6)** | RFC 4552 IPsec AH/ESP is a kernel SA/SP mechanism, structurally distinct from the in-packet RFC 7166 Authentication Trailer (`auth_keystore.go` / `auth_wiring.go`); it is its own child, not an extension of the trailer code |
| Debug folds in ospfclient | **ext-14 replaces the standalone ospfclient daemon (both AFs)** | The genuinely useful capability of FRR's `ospfclient` (inject/observe LSAs for testing) is delivered as introspection tooling inside ext-14, decoding IPv4 opaque/TE/RI/Extended LSAs and IPv6 RI/extended/SR/Grace LSAs; no separate Unix-socket external-injection daemon ships |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
- [ ] `plan/learned/972-ospf-af-unify.md` -- the delivered unified-engine decision
  -> Decision: ONE `ospf` engine with Transport / Codec / AFPrefixStrategy seams; FSM, flooding, DR election, SPF, and LSDB sequencing are AF-neutral and shared; only the wire format, LSA registry, prefix model, and transport are AF-specific
  -> Constraint: do NOT create a second OSPFv3 engine. There is no separate `ospfv3` plugin directory. Future IPv6 work belongs in the `_v6` strategies, link-scope LSDB handling, or the `internal/plugins/ospf/v3/{types,packet,transport}` leaves. Scope-typed OSPFv3 LS Types MUST be classified through helpers (`ASExternal`, `NSSA`, `InterAreaRouter`), never OSPFv2 numeric constants
- [ ] `plan/spec-ospf-0-umbrella.md` -- the DELIVERED IPv4 (OSPFv2) base umbrella the IPv4 children extend
  -> Decision: IPv4 extensions attach at the documented seams (LSDB stores, flooding chokepoints, SPF route table, NSM/DD, the registration model); the base is not re-opened
  -> Constraint: the base's "Out of scope (future...)" table is the authoritative source list of the IPv4 follow-ups this umbrella tracks -- do not add a child that is not derived from it (or explicitly record a new resting decision)
- [ ] `plan/spec-ospfv3-0-umbrella.md` -- the DELIVERED IPv6 (OSPFv3) base umbrella the IPv6 children extend
  -> Decision: IPv6 extensions attach at the `_v6` strategies, the scope-aware LSDB, flooding, SPF, ISM/NSM, the IPv6 transport leaf, the reserved Instance-ID field, and the RFC 7166 auth path; the base is not re-opened
  -> Constraint: RFC 5838 multi-AF and RFC 4552 IPsec, listed out-of-scope in the v3 base, are IN SCOPE here as ext-15 and ext-16. There is no opaque carrier in v3 (extensions are native LSAs), so the IPv4 ext-1 opaque framework has no v3 analogue
- [ ] `plan/spec-ospf-ext-1-opaque-framework.md` -- the written IPv4 foundation child (RFC 5250)
  -> Decision: ext-2/ext-3/ext-4 plug into the ext-1 consumer registry (`RegisterOpaqueConsumer`); the carrier interprets no body, each consumer owns its TLVs. ext-9 (IPv4 GR) and ext-14 (IPv4 debug) also ride it
  -> Constraint: opaque LSAs never enter SPF; consumers that affect forwarding (SR, TI-LFA) read the route table, they do not make opaque LSAs vertices
- [ ] `docs/research/ospf-implementation-guide.md` §14 (FRR extension feature catalogue) and §15 (OSPFv3 differences, unified address-family-aware engine)
  -> Constraint: the guide's rested/deferred rationale (TOS, SNMP MIB for both v2 and v3, multi-area adjacencies, QoS routing, Flood Reduction, ospfclient, forking a second engine) is the basis for this umbrella's "Out of scope (rested)" table
- [ ] The ext-2..ext-16 child specs (this batch) -- each child's own Required Reading governs its implementation; the umbrella points at them but does not duplicate their detail

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc5250.md` (IPv4 opaque framework, consumed by ext-1), `rfc/short/rfc2328.md` (IPv4 base, ext-7/ext-8 IPv4 records), `rfc/short/rfc5340.md` (IPv6 base, ext-7/ext-8 IPv6 records), `rfc/short/rfc5838.md` (multi-AF, ext-15), `rfc/short/rfc4552.md` (IPsec, ext-16) -- the pre-existing OSPF-extension summaries
  -> Constraint: each remaining child creates its own RFC summary via `/ze-rfc` before that child is implemented; the umbrella tracks the mapping (see RFC Coverage) but enforces no RFC text itself

**Key insights:** (minimal context to resume after compaction)
- One unified `ospf` engine; IPv4 and IPv6 are address families. No separate `ospfv3` plugin directory; the IPv6 wire code lives in `internal/plugins/ospf/v3/{types,packet,transport}` plus the `_v6` strategies.
- IPv4 opaque chain: ext-1 (carrier) -> ext-2/ext-3/ext-4 (consumers) -> ext-5 (IPv4 SR) -> ext-6 (TI-LFA). IPv4-specific.
- IPv6 SR (ext-5 IPv6 half) adds the v3 Router Information + RFC 8362 extended LSAs itself; no opaque carrier in v3.
- Both-AF features (ext-6 TI-LFA, ext-7 virtual links, ext-8 NBMA/P2MP, ext-10 BFD, ext-11 LDP-IGP sync, ext-14 debug) attach to the shared AF-neutral engine and serve both families.
- ext-9 (GR) is per-AF: IPv4 Grace-LSA is a Type 9 opaque LSA (needs ext-1); IPv6 Grace-LSA is a native v3 link-scope LSA.
- ext-15 (multi-AF) consumes the v3 reserved Instance-ID; ext-16 (IPsec) is a separate v3 auth path; ext-13 (L3VPN DN bit) is VRF-gated and last.
- The "rested" items (SNMP MIB v2 RFC 4750 + v3 RFC 5643, TOS/QoS/multi-area-adjacency, Flood-Reduction/demand-circuits, a second separate OSPF engine / forking by version) are deliberately absent and need a fresh decision to revive.

## Child Decomposition

Each child is an independent, implementable spec. ext-1 is already written
(Status:ready); ext-2..ext-16 are written as Status:ready specs in this same
batch. The umbrella owns the coordination, not the implementation. Every feature
is a feature of the one OSPF; the "Address family" column records which families
it covers.

| Child | Title | Address family | RFC(s) | Depends on | One-line scope |
|-------|-------|----------------|--------|------------|----------------|
| ext-1 | Opaque-LSA framework | IPv4 | RFC 5250 | base (IPv4) | The generic opaque carrier: scope-correct flooding (Type 9/10/11), the LS-ID Opaque Type/ID split, O-bit DD negotiation, generic TLV helpers, and the consumer registry that ext-2/3/4, ext-9 (IPv4), ext-14 (IPv4) plug into. No consumer semantics. **[WRITTEN]** |
| ext-2 | Traffic Engineering LSA | IPv4 | RFC 3630, RFC 5392 | ext-1 | Type 10 TE Opaque LSA: Router Address + Link TLV with sub-TLVs (link type, IDs, metrics, bandwidths, admin groups); RFC 5392 inter-AS TE links. Advertisement + TED build, no CSPF |
| ext-3 | Router Information LSA | IPv4 | RFC 7770 | ext-1 | The RI Opaque LSA carrying router-wide capabilities (Informational Capabilities TLV) plus the SR-Algorithm / SRGB / SR-Local-Block TLVs that IPv4 SR consumes |
| ext-4 | Extended Link / Extended Prefix LSA | IPv4 | RFC 7684 | ext-1 | The Extended Prefix and Extended Link Opaque LSAs and their sub-TLV containers (the carriers for Prefix-SID / Adjacency-SID and prefix attributes) |
| ext-5 | Segment Routing | IPv4 (RFC 8665) + IPv6 (RFC 8666/8362) | RFC 8665, RFC 8666, RFC 8362 | ext-3 + ext-4 (IPv4 half); base (IPv6 half) | OSPF SR control plane for both AFs. IPv4: SRGB/SRLB from RI (ext-3), Prefix-SID / Adjacency-SID from Extended Prefix/Link (ext-4). IPv6: add the v3 Router Information LSA + the OSPFv3 Extended-LSA containers (RFC 8362) as native `v3/packet` bodies first, then map SR labels. MPLS label computation + FIB programming through the shared Loc-RIB seam for both |
| ext-6 | TI-LFA / LFA fast reroute | both | RFC 5286, TI-LFA draft (draft-ietf-rtgwg-segment-routing-ti-lfa) | ext-5, shared SPF | Loop-free alternate + topology-independent LFA repair paths over the shared SPF and the SR label stack; pre-computed backup nexthops in the FIB for both AFs |
| ext-7 | Virtual links | both | RFC 2328 §15 (IPv4), RFC 5340 §4.2 (IPv6) | base (both) | Virtual-link adjacencies across a transit area to repair a partitioned or non-contiguous backbone; virtual-interface ISM/NSM on the shared engine, with AF-specific virtual-link records (Type 1 for IPv4, the v3 equivalent for IPv6) |
| ext-8 | NBMA + point-to-multipoint | both | RFC 2328 (IPv4 network types), RFC 5340 (IPv6 network types) | base (both) | The NBMA and point-to-multipoint network types: static neighbour config, NBMA DR/BDR eligibility (shared DR election), P2MP per-neighbour Hellos and host-route origination for both AFs |
| ext-9 | Graceful Restart | IPv4 (RFC 3623) + IPv6 (RFC 5187) | RFC 3623, RFC 5187 | ext-1 (IPv4 Grace-LSA); base (IPv6 Grace-LSA) | Grace-LSA origination + the GR helper (and restarter) so a neighbour's restart does not tear down forwarding. IPv4 Grace-LSA is a Type 9 link-scope opaque LSA (needs ext-1); IPv6 Grace-LSA is a native link-scope v3 LSA (no opaque carrier) added as a `v3/packet` body |
| ext-10 | BFD for OSPF | both | RFC 5880, RFC 5881 | base (both) | Integrate Ze's existing BFD engine: register OSPF adjacencies (IPv4 and IPv6 single-hop) as BFD clients, drive the shared NSM down on a BFD session failure for sub-second failure detection |
| ext-11 | LDP-IGP synchronisation | both | RFC 5443, RFC 6138 | base (both) | Hold an OSPF link at max-metric until LDP signalling is up (plus the RFC 6138 unnumbered/LFA refinement) so traffic does not use a link whose LSP is not ready; serves both AFs through the shared interface model |
| ext-12 | Multi-Instance OSPF | IPv4 | RFC 6549 | base (IPv4) | The Instance ID field in the OSPFv2 common header so multiple OSPF instances share an interface; per-instance packet demultiplexing. Distinct from the IPv6 Instance-ID demux the v3 base reserved (consumed by ext-15) |
| ext-13 | L3VPN PE-CE DN bit | IPv4 | RFC 4576, RFC 4577 | base (IPv4), **future VRF/MPLS-L3VPN infra (BLOCKING)** | The Down (DN) bit + VPN Route Tag loop prevention for OSPF as a PE-CE protocol; requires per-VRF OSPF instances that Ze's routing infrastructure does not yet support |
| ext-14 | Debug & introspection tooling | both | (no new RFC; tooling over the LSDB / codecs) | ext-1 (IPv4 opaque decode); base (IPv6 decode) | Extension-wide debug/introspection for both AFs: decode + inspect IPv4 opaque/TE/RI/Extended LSAs and IPv6 RI/extended/SR/Grace LSAs, inject test LSAs (IPv4 via the ext-1 registry, IPv6 via the v3 LSDB), and the show/diagnostic surface for every extension above. Folds in the useful `ospfclient` inject/observe capability |
| ext-15 | Multiple address families | IPv6 | RFC 5838 | base (IPv6) | Map address families to OSPFv3 Instance-ID ranges (§2.4) using the v3 base's reserved Instance-ID plumbing; spawn one unified-engine instance per AF; per-AF topologies and route install; AF-aware prefix strategy without re-opening the RFC 5340 codec |
| ext-16 | IPsec AH/ESP authentication | IPv6 | RFC 4552 | base (IPv6) | OSPFv3 manual-keyed IPsec AH/ESP as a distinct auth path from the delivered RFC 7166 trailer; kernel IPsec SA/SP policy wiring for the OSPFv3 IPv6 transport leaf, per-interface SPI/key config |

## Dependency / Build Order

The children split into one IPv4-specific dependency chain rooted at the opaque
carrier, a set of both-AF features that attach to the shared engine, and a small
IPv6-only set on the reserved Instance-ID / IPv6 transport seams.

| Child | Address family | Depends on |
|-------|----------------|-----------|
| ext-1 (opaque carrier) | IPv4 | base (IPv4) only |
| ext-2 (TE) | IPv4 | ext-1 |
| ext-3 (Router Information) | IPv4 | ext-1 |
| ext-4 (Extended Link/Prefix) | IPv4 | ext-1 |
| ext-5 (Segment Routing) | both | ext-3 + ext-4 (IPv4 half); base + self-added v3 RI/RFC 8362 LSAs (IPv6 half) |
| ext-6 (TI-LFA / LFA) | both | ext-5, shared SPF |
| ext-7 (Virtual links) | both | base (both) only |
| ext-8 (NBMA + P2MP) | both | base (both) only |
| ext-9 (Graceful Restart) | IPv4 + IPv6 | ext-1 (IPv4 Grace-LSA is a Type 9 opaque LSA); base (IPv6 Grace-LSA is a native v3 LSA) |
| ext-10 (BFD) | both | base (both) only |
| ext-11 (LDP-IGP sync) | both | base (both) only |
| ext-12 (Multi-Instance) | IPv4 | base (IPv4) only |
| ext-13 (L3VPN DN bit) | IPv4 | base (IPv4) + future VRF/MPLS-L3VPN infra (BLOCKING) |
| ext-14 (Debug & introspection) | both | ext-1 (IPv4 opaque decode); base (IPv6 decode) |
| ext-15 (Multiple address families) | IPv6 | base (IPv6); consumes the reserved Instance-ID plumbing |
| ext-16 (IPsec AH/ESP auth) | IPv6 | base (IPv6) only |

**IPv4 opaque chain:** ext-1 is the single IPv4 foundation for ext-2, ext-3,
ext-4, the IPv4 Grace-LSA half of ext-9, and the IPv4 opaque-decode half of
ext-14. The IPv4 half of ext-5 (SR) cannot start until ext-3 AND ext-4 land;
ext-6 (TI-LFA) cannot start until ext-5 lands and reuses the shared SPF. This
chain is IPv4-specific: RFC 5340 carries IPv6 extensions as native LSAs, so the
opaque carrier has no v3 analogue.

**IPv6 SR is self-contained:** the IPv6 half of ext-5 does NOT depend on the
opaque carrier; instead it adds the OSPFv3 Router Information LSA and the OSPFv3
Extended-LSA TLV containers (RFC 8362) -- as new `internal/plugins/ospf/v3/packet`
bodies plus `_v6` origination strategies -- before it can carry SRGB/SRLB and
Prefix-SID/Adjacency-SID. That advertisement layer is part of ext-5, not a
separate child.

**Both-AF (shared-engine) set:** ext-6 (TI-LFA), ext-7 (virtual links), ext-8
(NBMA/P2MP), ext-10 (BFD), ext-11 (LDP-IGP sync), and ext-14 (debug) attach to
the shared, AF-neutral seams (ISM/NSM, DR election, SPF, the interface model);
one implementation serves both families, with AF-specific records/transport where
RFC 2328 and RFC 5340 diverge. ext-7, ext-8, ext-10, ext-11 are base-only;
ext-6 waits on ext-5; ext-14's IPv4 half waits on ext-1.

**IPv6-only set:** ext-15 (multi-AF) consumes the reserved Instance-ID and spawns
one engine instance per AF; ext-16 (IPsec) is a separate auth path on the IPv6
transport leaf. Both are base-only and parallelise.

**Gated:** ext-13 (IPv4) additionally depends on MPLS-L3VPN / VRF infrastructure
that does not yet exist in Ze; it is BLOCKED until that lands and must not be
scheduled before it.

**Recommended build order:**

1. **ext-1** (IPv4 opaque carrier) -- unblocks the IPv4 opaque chain.
2. In parallel after ext-1, and independently the base-only set: **ext-2, ext-3,
   ext-4** (IPv4 opaque consumers); **ext-7, ext-8** (virtual links, NBMA/P2MP,
   both AFs); **ext-10, ext-11** (BFD, LDP-IGP sync, both AFs); **ext-9** (GR,
   IPv4 half waits on ext-1, IPv6 half base-only); **ext-12** (IPv4
   Multi-Instance); **ext-15, ext-16** (IPv6 multi-AF, IPsec); **ext-14** (debug,
   IPv4 half waits on ext-1, IPv6 half base-only).
3. **ext-5** (Segment Routing) -- IPv4 half once ext-3 and ext-4 are both done;
   IPv6 half once its self-added v3 RI + RFC 8362 LSAs land.
4. **ext-6** (TI-LFA, both AFs) -- once ext-5 is done.
5. **ext-13** (IPv4 L3VPN DN bit) -- LAST, gated on VRF/MPLS-L3VPN infrastructure.

Condensed: `ext-1 -> {ext-2, ext-3, ext-4}` (IPv4 opaque); `{ext-7, ext-8,
ext-9, ext-10, ext-11, ext-12, ext-14, ext-15, ext-16}` parallel on the
bases/shared engine; `ext-3 + ext-4 -> ext-5(IPv4)`, `base -> ext-5(IPv6)`;
`ext-5 -> ext-6`; `ext-13` last (gated on VRF).

## Out of scope (rested, noted here so it is not silently assumed done)

These OSPF features (both address families) were considered for this extension
set and deliberately NOT scheduled. Each is recorded with its rationale so a
future agent does not assume it is implemented, nor quietly re-add it without
revisiting the reasoning. A "rested" item differs from a "deferred" child:
rested items are not part of any ext-N child and need a fresh decision (and
likely a new spec) to revive.

> NOTE: RFC 5838 multiple address families and RFC 4552 IPsec authentication are
> IN SCOPE here as ext-15 and ext-16 respectively; they are NOT rested. The IPv6
> (OSPFv3) base umbrella listed them as out of scope, and this umbrella picks
> them up.

| Rested item | Address family | RFC(s) | Rationale for resting |
|-------------|----------------|--------|-----------------------|
| TOS (Type-of-Service) routing | both | RFC 2328 §16.9 (IPv4, and earlier RFC 1583); equivalent v3 metric model | Deprecated by later RFCs; no production OSPF implementation advertises or computes per-TOS metrics. The #TOS field stays 0 in originated LSAs (already the base behaviour, both AFs). Reviving it has no interop value |
| SNMP OSPF-MIB | both | RFC 4750 (OSPFv2-MIB), RFC 5643 (OSPFV3-MIB) | Ze's management plane is YANG / gNMI / CLI / web, not SNMP. There is no SNMP agent to host either MIB; OSPF state is already exposed through `show ospf` / `show ospf ipv6` and Prometheus metrics. Adding SNMP would duplicate the existing surface against a transport Ze does not speak |
| Multi-area adjacencies | IPv4 (and v3 analogue) | RFC 5185 | Niche feature (a single link in multiple areas via a point-to-point logical adjacency). FRR does not implement it; demand is minimal. Virtual links (ext-7) cover the backbone-repair use case that overlaps with it |
| QoS routing | both | RFC 2676 | Experimental; effectively nobody implements it. The metric model and flooding extensions it needs are speculative with no interop partner, in either AF |
| OSPF Flood Reduction + demand-circuit DoNotAge | both | RFC 7715, RFC 1793 (DoNotAge) | A pure optimisation for very large, very stable LSDBs (suppressing periodic LSA refresh via the DoNotAge bit). It has subtle failure modes around stale-LSA retention and topology-change re-flooding. DEFERRED, not rejected -- it may be revisited if a deployment ever needs it; until then the shared LSRefresh/MaxAge behaviour is correct and safe for both AFs |
| Standalone `ospfclient` Unix-socket daemon | both | (FRR tooling, no RFC) | The genuinely useful capability (inject/observe LSAs for testing) is folded into **ext-14** (covering IPv4 opaque LSAs and IPv6 native LSAs) instead. A separate external-injection daemon with its own socket and trust boundary is unnecessary surface; ext-14 delivers the same debug value in-process |
| A second, separate OSPF engine (forking the engine per protocol version) | n/a (architecture) | (the delivered unified-engine design, `plan/learned/972-ospf-af-unify.md`) | FORBIDDEN. OSPF is one engine; IPv4 and IPv6 are address families that share the AF-neutral FSM, flooding, DR election, SPF, and LSDB sequencing. Forking a second engine (e.g. a separate `ospfv3` plugin directory) would duplicate that machinery and reintroduce the v2/v3 drift the unification removed. AF-specific parts (v3 header, LSA registry, flooding-scope model, prefix encoding, checksum -- RFC 5340 vs RFC 2328) already live in the IPv4 `packet/` codec, the `_v6` strategies, and the `internal/plugins/ospf/v3/{types,packet,transport}` leaves; extensions ADD to those seams |

## Current Behavior (MANDATORY)

**Source files read:** (architecture survey; each child spec reads its own targets)
<!-- Same rule: never tick [ ] to [x]. Write -> Constraint: annotations instead. -->
- [ ] `internal/plugins/ospf/lsdb/` -- the shared, AF-neutral LSDB: per-area / link / AS-external stores and routing, origination, flooding, aging, ack handling, sequencing; the seam the IPv4 opaque carrier (ext-1) and the IPv4 opaque consumers (ext-2/3/4), the GR Grace-LSAs (ext-9, both AFs), and the IPv6 RI / extended / SR LSAs (ext-5 IPv6) extend
  -> Constraint: the LSDB sequencing/flooding machinery is AF-neutral and shared; extensions ADD stores/routing (IPv4) or scope-aware v3 LSA types (IPv6, classified via `ASExternal` / `NSSA` / `InterAreaRouter` helpers, never v2 numeric constants) at this chokepoint; they do not rewrite the delivered routing
- [ ] `internal/plugins/ospf/packet/` -- the delivered IPv4 codec; it already retains opaque (Type 9/10/11) bodies verbatim and classifies them via `IsOpaque()`
  -> Constraint: ext-1 adds only the Opaque Type/ID split and generic TLV helpers; ext-2/3/4 add typed bodies on top, they do not re-open the verbatim passthrough
- [ ] `internal/plugins/ospf/v3/packet/` + `internal/plugins/ospf/v3/types/` -- the delivered RFC 5340 codec leaves: 16-byte header, IPv6 pseudo-header checksum, scope-aware LS types, topology/prefix separation, padded prefix encoding, the 24-bit Options field
  -> Constraint: the IPv6 half of ext-5 adds the v3 Router Information + Extended-LSA (RFC 8362) bodies, and ext-9 adds the IPv6 Grace-LSA body, into the `v3/packet` leaf on top of the existing scope-aware decode; they do not re-open the base codec. ext-15 keeps the Instance-ID field as the base defined it
- [ ] `internal/plugins/ospf/spf/` -- the shared, AF-neutral two-stage Dijkstra + route table with path types and the ASBR-reachability used by Type 5 (IPv4) and the v3 external path (IPv6)
  -> Constraint: opaque LSAs never become SPF vertices; SR (ext-5) and TI-LFA (ext-6) READ the route table and add label/repair computation for both AFs, they do not feed opaque or extension bodies into the graph as vertices unless RFC 5340 defines them as such
- [ ] `internal/plugins/ospf/afstrategy_v6.go`, `codec_v6.go`, `encoder_v6.go`, `origination_v6*.go`, `nssa.go` -- the IPv6 `_v6` strategy seams (prefix/next-hop strategy, v6 codec glue, v6 LSA origination) consumed by the shared engine
  -> Constraint: the IPv6 halves of ext-5 (SR origination), ext-7/ext-8 (v3 network types), ext-9 (IPv6 Grace-LSA), and ext-15 (AF-aware prefix width) extend these strategy files; they do not duplicate the shared engine
- [ ] `internal/plugins/ospf/neighbor/` + `internal/plugins/ospf/iface/` -- the shared NSM + DD exchange and the ISM (Hello, DR/BDR); the seam ext-1 uses for IPv4 O-bit negotiation, ext-7/ext-8 use for virtual links and network types, ext-10 touches for BFD client registration, and ext-12 touches for IPv4 Instance-ID demux
  -> Constraint: extensions extend ISM/NSM behaviour additively for both AFs; the delivered adjacency state machine and DR election are preserved
- [ ] `internal/plugins/ospf/v3/transport/` -- the delivered raw IPv6 proto 89 transport leaf; ext-16 wires kernel IPsec SA/SP policy onto it
  -> Constraint: ext-16 (IPsec) sits alongside the transport leaf as a kernel SA/SP path; the leaf itself is preserved
- [ ] `internal/plugins/ospf/auth_keystore.go` + `internal/plugins/ospf/auth_wiring.go` -- the delivered RFC 7166 Authentication Trailer (AT-bit, SA config, sign/verify, 64-bit sequence anti-replay) for IPv6, and the IPv4 AuType 0/1/2/3 path
  -> Constraint: ext-16 (RFC 4552 IPsec) is a SEPARATE auth path (kernel IPsec SA/SP), not an extension of the trailer code; the trailer remains the in-packet mechanism
- [ ] `internal/plugins/ospf/register.go` + `internal/plugins/ospf/dispatcher.go` + `internal/plugins/ospf/instance.go` -- the delivered registration + SDK lifecycle (`register.go` spawns the IPv4 and IPv6 engine instances; `dispatcher.go` holds the Instance-ID demux); ext-1 adds the IPv4 opaque consumer registry here; ext-15 consumes the reserved IPv6 Instance-ID; later extensions register their own commands/schema/doctor checks
  -> Constraint: each extension is self-contained (`ai/rules/plugins.md`); removing a child removes all its registration cleanly. No v3 extension spelling appears in any generic/central package; no child forks a second engine

**Behavior to preserve:** (the delivered bases are a stable foundation)
- The unified engine: IPv4 (OSPFv2) and IPv6 (OSPFv3) SHARE the AF-neutral FSM / flooding / DR election / SPF / LSDB-sequencing machinery; every extension is additive and must not fork a second engine or leak v3 LS-type constants into the IPv4 path.
- The delivered IPv4 wire codec, the LSDB key triple, the three stores (link/per-area/AS-external), `OriginateSelf`/`OriginateLinkSelf`/`OriginateExternal`, and the §13 flooding + §16 SPF behaviour.
- The delivered RFC 5340 codec leaves (`internal/plugins/ospf/v3/{packet,types}`), the scope-aware LSDB, the SPF + prefix-attachment behaviour, the IPv6 raw-transport leaf, the ISM/NSM, the reserved Instance-ID field, and the RFC 7166 auth trailer.
- All existing OSPFv2 and OSPFv3 functional and FRR (`ospfd` / `ospf6d`) interop tests: every extension is additive; a router with no extension enabled behaves exactly as the delivered base, in either AF.
- The FIB-install-via-Loc-RIB path and the redistribution-via-redistevents path; extensions that affect forwarding (SR/TI-LFA) install through the SAME shared Loc-RIB seam for both AFs.
- The canonical OSPF metric set and the command-YANG ownership model; each extension adds its own `ze_ospf_<ext>_*` (IPv4) or `ze_ospfv3_<ext>_*` (IPv6) series and its own `show ospf <noun>` / `show ospf ipv6 <noun>` subcommands, it does not rename existing ones.

**Behavior to change:** (this umbrella changes NONE directly)
- None -- the umbrella implements nothing. Each child changes behaviour additively, documented in that child's own "Behavior to change". The umbrella only coordinates ordering and records the rested set.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

(Umbrella-level: the data paths each child plugs into. Each child carries its own detailed Data Flow.)

### Entry Point
- Children are selected and implemented in the build order above; the umbrella's "input" is the set of follow-ups from the two base umbrellas' out-of-scope tables (with RFC 5838 and RFC 4552 promoted in scope), plus the resting decisions captured here.
- At runtime, every extension enters through the unified OSPF engine's existing entry points: IPv4 opaque LSAs via `lsdb.ReceiveUpdate` (the ext-1 carrier path), IPv6 LSAs via the scope-aware LSDB receive path, IPv6 datagrams via the raw transport leaf, config via the YANG tree (`ospf { address-family { ipv6 { ... } } }`), BFD/LDP-sync via the existing engine integration seams, the Instance-ID demux.

### Transformation Path
1. **Bases delivered:** IPv4 OSPFv2 (areas, ABR/ASBR, stub/NSSA, §13 flooding, §16 SPF, redistribution, auth) and IPv6 OSPFv3 (RFC 5340 codec + IPv6 transport leaves, scope-aware LSDB, ISM/NSM, SPF + prefix attachment, RFC 7166 trailer) are complete and stable inside the one engine (the source of every seam below).
2. **IPv4 opaque carrier:** ext-1 (RFC 5250) lands on the shared LSDB/flooding/neighbor seams; ext-2 (TE), ext-3 (RI), ext-4 (Extended Link/Prefix) register Opaque Types on it and add typed bodies.
3. **SR control plane:** ext-5 IPv4 half reads RI (ext-3) + Extended Prefix/Link (ext-4); ext-5 IPv6 half adds the v3 RI + RFC 8362 extended LSAs itself; both compute SR labels and program SR paths via the shared Loc-RIB seam.
4. **Repair paths:** ext-6 (TI-LFA/LFA) reads the shared SPF + the ext-5 label stack to pre-compute backup nexthops for both AFs.
5. **Both-AF protocol features:** ext-7 (virtual links), ext-8 (NBMA/P2MP), ext-10 (BFD), ext-11 (LDP-IGP sync) extend the shared ISM/NSM/interface model directly; ext-9 (GR) adds Grace-LSAs (IPv4 opaque via ext-1, IPv6 native via the v3 LSDB).
6. **IPv4-only:** ext-12 (Multi-Instance) adds the OSPFv2 Instance ID demux.
7. **IPv6-only:** ext-15 (multi-AF) maps AF to the reserved Instance-ID and spawns one engine instance per AF; ext-16 (IPsec) adds a kernel SA/SP path on the IPv6 transport leaf.
8. **Debug:** ext-14 decodes/injects IPv4 opaque and IPv6 native extension LSAs.
9. **VRF-gated:** ext-13 (IPv4 L3VPN DN bit) lands LAST, once MPLS-L3VPN/VRF infrastructure exists.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| IPv4 base <-> opaque carrier | ext-1 attaches at `dbForLocked` / flooding chokepoints / DD Options; the IPv4 codec already carries opaque verbatim | [ ] |
| IPv4 carrier <-> consumer | ext-2/3/4 register Opaque Types via the ext-1 registry; value-typed payloads, no cross-boundary pointers | [ ] |
| IPv6 base <-> native extension LSAs | ext-5 (IPv6) and ext-9 (IPv6) add native scope-aware v3 LSA types in the `v3/packet` leaf with `_v6` origination | [ ] |
| RI/Extended <-> SR | ext-5 reads ext-3 SRGB/SRLB + ext-4 Prefix-SID/Adjacency-SID (IPv4) or its own v3 RI/RFC 8362 LSAs (IPv6) -- read-only consumption of advertised state | [ ] |
| SR/SPF <-> repair | ext-6 reads the shared SPF route table + the ext-5 label stack to emit backup nexthops for both AFs | [ ] |
| Extension <-> Loc-RIB (FIB) | SR/TI-LFA forwarding state installs through the SAME shared `locrib.Path` seam (IPv4 and IPv6), never a second FIB path | [ ] |
| Engine <-> BFD / LDP | ext-10 registers OSPF adjacencies (both AFs) as BFD clients; ext-11 reads LDP signalling state; both via existing engine integration | [ ] |
| IPv6 transport leaf <-> IPsec | ext-16 wires kernel IPsec SA/SP policy onto the raw IPv6 transport leaf; distinct from the RFC 7166 trailer path | [ ] |
| Reserved Instance-ID <-> multi-AF | ext-15 attaches at the reserved Instance-ID field; spawns one engine instance per AF without a codec change | [ ] |
| OSPF <-> VRF (ext-13) | DN bit / VPN Route Tag loop prevention across per-VRF OSPF instances (future infra, IPv4) | [ ] |

### Integration Points
- `internal/plugins/ospf/lsdb` (shared stores + flooding + origination + sequencing) -- ext-1 and every IPv4 opaque consumer, plus the IPv6 ext-5/ext-9 native LSAs, attach here.
- `internal/plugins/ospf/packet` (IPv4 codec + TLV helpers) -- ext-1 TLV helpers; ext-2/3/4 typed bodies.
- `internal/plugins/ospf/v3/packet` + `internal/plugins/ospf/v3/types` (RFC 5340 codec leaves) -- ext-5 (IPv6) adds the v3 RI + Extended-LSA (RFC 8362) bodies; ext-9 (IPv6) adds the Grace-LSA body; ext-15 keeps the Instance-ID field.
- `internal/plugins/ospf/spf` (shared route table, reachability) -- read by ext-5/ext-6 (both AFs); never receives opaque vertices.
- `internal/plugins/ospf/afstrategy_v6.go` + the `_v6` strategy files -- the IPv6 prefix/next-hop strategy and v6 LSA origination; extended by the IPv6 halves of ext-5/ext-7/ext-8/ext-9/ext-15.
- `internal/plugins/ospf/neighbor` + `internal/plugins/ospf/iface` (shared NSM/DD + ISM) -- ext-1 IPv4 O-bit; ext-7/ext-8 virtual links + network types; ext-10 BFD client; ext-12 IPv4 Instance-ID demux.
- `internal/plugins/ospf/v3/transport` (IPv6 transport leaf) -- ext-16 IPsec SA/SP policy.
- `internal/plugins/ospf/auth_keystore.go` + `internal/plugins/ospf/auth_wiring.go` (RFC 7166 trailer + IPv4 auth) -- ext-16 sits alongside as a distinct auth path.
- `internal/plugins/ospf/register.go` + `internal/plugins/ospf/dispatcher.go` + `internal/plugins/ospf/instance.go` (registration + lifecycle + Instance-ID demux) -- the ext-1 consumer registry; the reserved Instance-ID for ext-15; each child's own commands/schema/doctor.
- The shared Loc-RIB / sysrib FIB-install seam -- SR/TI-LFA forwarding state (both AFs).
- Ze's existing BFD engine (ext-10), LDP signalling (ext-11), and kernel IPsec SA/SP infrastructure (ext-16).
- Future MPLS-L3VPN / VRF infrastructure (ext-13, BLOCKING).

### Architectural Verification
- [ ] No bypassed layers (extensions attach at delivered seams; SR/TI-LFA install through the existing Loc-RIB path for both AFs, not a new FIB path)
- [ ] No unintended coupling (the IPv4 opaque carrier names no consumer; consumers depend on the carrier, not vice-versa; base-only features do not depend on the opaque chain; no child forks a second engine or leaks v3 LS-type constants into the IPv4 path)
- [ ] No duplicated functionality (extensions reuse the shared stores, flooding, SPF, DR election, NSM, and FIB-install seams; the v3 wire/LSA/prefix code stays in the `internal/plugins/ospf/v3/{packet,types}` leaves)
- [ ] Zero-copy preserved (opaque and v3 LSA bodies retained as views; TLV iterator zero-copy; buffer-first encode -- enforced per child)

## Wiring Test (MANDATORY -- NOT deferrable)

(Umbrella-level: proves the child set is reachable as a coordinated whole. Each child carries its own detailed, executable Wiring Test.)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| ext-1 IPv4 opaque carrier present | -> | a consumer registers an Opaque Type and the engine discovers it | `TestOpaqueConsumerRegistered` (ext-1) + `test/ospf/ospf-opaque-register.ci` |
| ext-3 RI + ext-4 Extended Prefix/Link present (IPv4) | -> | ext-5 IPv4 SR reads SRGB/SRLB + Prefix-SID/Adjacency-SID and programs an SR path | `test/ospf/ospf-sr-install.ci` (ext-5 IPv4) |
| IPv6 SR configured with SRGB + Prefix-SID | -> | ext-5 adds the v3 RI + RFC 8362 extended LSAs, computes SR labels, and programs an SR path | `test/ospfv3/ospfv3-sr-install.ci` (ext-5 IPv6) |
| ext-5 SR present (either AF) | -> | ext-6 computes a TI-LFA backup nexthop over the SR label stack | `test/ospf/ospf-tilfa.ci`, `test/ospfv3/ospfv3-tilfa.ci` (ext-6) |
| base only (either AF) | -> | ext-10 drives the shared NSM down on a BFD session failure | `test/ospf/ospf-bfd.ci`, `test/ospfv3/ospfv3-bfd.ci` (ext-10) |
| Two address families configured | -> | ext-15 maps each AF to an Instance-ID range and installs per-AF routes | `test/ospfv3/ospfv3-multiaf.ci` (ext-15) |
| IPsec AH/ESP configured on an OSPFv3 interface | -> | ext-16 installs kernel IPsec SA/SP and the adjacency reaches Full under IPsec protection | `test/ospfv3/ospfv3-ipsec.ci` (ext-16) |
| future VRF infra present | -> | ext-13 sets/honours the DN bit for a PE-CE OSPF instance | `test/ospf/ospf-dnbit.ci` (ext-13, when unblocked) |

## Acceptance Criteria

(Umbrella-level coordination criteria; each child carries its own detailed, testable ACs.)

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The child set is written | Every child ext-1..ext-16 exists, cross-references its dependencies, records its address-family coverage, and is consistent with the unified-engine layout and the two bases' Shared Contracts |
| AC-2 | The build order is followed | No IPv4 opaque consumer (ext-2/3/4, IPv4 ext-9, IPv4 ext-14) is scheduled before ext-1; IPv4 SR (ext-5) not before ext-3 + ext-4; IPv6 SR adds its v3 RI + RFC 8362 LSAs first; TI-LFA (ext-6) not before ext-5 |
| AC-3 | A both-AF feature is implemented | ext-6/7/8/10/11/14 attach to shared, AF-neutral seams and serve both families; ext-7/8/10/11 build on the bases without depending on the opaque chain |
| AC-4 | An IPv6-only item is considered | RFC 5838 / RFC 4552 are scheduled as ext-15 / ext-16 (in scope), NOT left in the rested table |
| AC-5 | ext-13 is considered | It is recorded as BLOCKED on VRF/MPLS-L3VPN infra (IPv4) and is not implemented before that infra lands |
| AC-6 | A rested item is encountered | It is found in the "Out of scope (rested)" table with a rationale (SNMP MIB v2 RFC 4750 + v3 RFC 5643, TOS/QoS/multi-area-adjacency, Flood-Reduction/DoNotAge, ospfclient, a second separate OSPF engine); reviving it requires a fresh decision and a new spec, not a quiet add to a child |
| AC-7 | Any child is implemented | Its code lives within the unified `ospf` engine (`internal/plugins/ospf/...` plus the `internal/plugins/ospf/v3/{types,packet,transport}` leaves for IPv6); it does not fork a second engine and never references a separate `ospfv3` plugin directory |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables OSPF opaque + a consumer (IPv4) | ext-1 carrier -> consumer registry -> origination/flooding/delivery | `test/ospf/ospf-opaque-*.ci` (ext-1) |
| 2 | Enables OSPF Segment Routing (IPv4) | ext-3 RI + ext-4 Extended Prefix/Link -> ext-5 SR label computation -> Loc-RIB SR path -> kernel | `test/ospf/ospf-sr-install.ci` (ext-5 IPv4) |
| 3 | Enables OSPF Segment Routing (IPv6) | ext-5 v3 RI + RFC 8362 extended LSAs -> SR label computation -> IPv6 Loc-RIB SR path -> kernel | `test/ospfv3/ospfv3-sr-install.ci` (ext-5 IPv6) |
| 4 | Enables TI-LFA fast reroute (either AF) | ext-5 SR + shared SPF -> ext-6 backup nexthop -> FIB | `test/ospf/ospf-tilfa.ci`, `test/ospfv3/ospfv3-tilfa.ci` (ext-6) |
| 5 | Enables BFD for OSPF (either AF) | ext-10 BFD client registration -> sub-second shared-NSM down on failure | `test/ospf/ospf-bfd.ci`, `test/ospfv3/ospfv3-bfd.ci` (ext-10) |
| 6 | Runs OSPFv3 with two address families | ext-15 Instance-ID mapping -> per-AF engine instance -> per-AF Loc-RIB install | `test/ospfv3/ospfv3-multiaf.ci` (ext-15) |
| 7 | Protects OSPFv3 with IPsec AH/ESP | ext-16 kernel IPsec SA/SP -> protected transport leaf -> Full adjacency | `test/ospfv3/ospfv3-ipsec.ci` (ext-16) |
| 8 | Runs OSPF as a PE-CE protocol (future, IPv4) | ext-13 DN bit / VPN Route Tag over a per-VRF OSPF instance | `test/ospf/ospf-dnbit.ci` (ext-13, when unblocked) |

## 🧪 TDD Test Plan

### Unit Tests
(Per child; the umbrella aggregates, it does not own unit tests.)

| Test | File | Validates | Status |
|------|------|-----------|--------|
| (per child) | `internal/plugins/ospf/...` and `internal/plugins/ospf/v3/...` (per ext-N) | see each child spec ext-1..ext-16 | |

### Boundary Tests (MANDATORY for numeric inputs)
(Per child; the umbrella introduces no new numeric wire fields of its own.)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Opaque Type (LS-ID high byte, IPv4) | 0-255 | 255 | N/A | N/A (1 byte) -- owned by ext-1 |
| Opaque ID (LS-ID low 24 bits, IPv4) | 0-16777215 | 16777215 | N/A | N/A (masked) -- owned by ext-1 |
| Instance ID (IPv6 AF mapping) | 0-255 | 255 | N/A | N/A (1 byte) -- AF ranges owned by ext-15 (RFC 5838 §2.4) |
| Instance ID (IPv4 Multi-Instance) | 0-255 | 255 | N/A | N/A (1 byte) -- owned by ext-12 (RFC 6549) |
| (other numeric fields) | per child | -- | -- | per ext-N boundary tables |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| (per child, IPv4) | `test/ospf/ospf-<ext>-*.ci` | per ext-N functional scenarios | |
| (per child, IPv6) | `test/ospfv3/ospfv3-<ext>-*.ci` | per ext-N functional scenarios | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| (per child, IPv4) | `test/interop/scenarios/ospf-<ext>-frr/` | FRR `ospfd` | per ext-N wire-behaviour interop (opaque/TE/RI/SR/GR/BFD as applicable) | |
| (per child, IPv6) | `test/interop/scenarios/ospfv3-<ext>-frr/` | FRR `ospf6d` | per ext-N wire-behaviour interop (multi-AF/IPsec/vlink/GR/BFD/SR/P2MP as applicable) | |

### Future (if deferring any tests)
- All extension tests are owned by their children; ext-13 interop is deferred with the feature until VRF infrastructure exists.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files -->
- `internal/plugins/ospf/` -- the unified OSPF engine each child extends: the shared AF-neutral subsystems (`lsdb/`, `spf/`, `neighbor/`, `iface/`), the IPv4 `packet/` codec, the IPv6 `_v6` strategy files (`afstrategy_v6.go`, `codec_v6.go`, `encoder_v6.go`, `origination_v6*.go`, `nssa.go`), the auth trailer (`auth_keystore.go`, `auth_wiring.go`), and the lifecycle (`register.go`, `dispatcher.go`, `instance.go`); the umbrella names no single file edit of its own
- `internal/plugins/ospf/v3/{types,packet,transport}/` -- the OSPFv3 codec / transport leaves each IPv6 protocol-feature child adds LSA bodies / options / transport handling to
- `plan/spec-ospf-ext-1-opaque-framework.md` .. `plan/spec-ospf-ext-16-*.md` -- the child specs this umbrella coordinates (authoring deliverable)
- `docs/comparison.md`, `docs/features.md` -- OSPF-extension parity rows for both AFs (per child, as each lands)
- NOTE: the umbrella itself modifies no feature code; each child lists its own `internal/plugins/ospf/...` and `internal/plugins/ospf/v3/...` edits

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | Per child | each ext-N adds its own leaves to `internal/plugins/ospf/yang/ze-ospf-conf.yang` (the single unified OSPF schema; IPv6 leaves under `ospf { address-family { ipv6 { ... } } }`) |
| CLI commands/flags | Per child | each ext-N adds `show ospf <noun>` (IPv4) / `show ospf ipv6 <noun>` (IPv6) subcommands in `internal/plugins/ospf/yang/ze-ospf-cmd.yang` |
| Doctor check for runtime dependencies | Per child | ext-10 (BFD), ext-11 (LDP), ext-16 (kernel IPsec) and any new runtime dependency get their own check |
| Prometheus counters/metrics | Per child | each ext-N owns its `ze_ospf_<ext>_*` (IPv4) / `ze_ospfv3_<ext>_*` (IPv6) series; the umbrella metrics mapping is updated as children land |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Per child | `docs/features.md` (per ext-N) |
| 5 | Plugin added/changed? | Per child | `docs/guide/plugins.md`, `docs/plugin-overview.md` (per ext-N) |
| 6 | Has a user guide page? | Per child | `docs/guide/ospf.md` (per ext-N) |
| 9 | RFC behavior implemented? | Per child | `rfc/short/rfcNNNN.md` (created by each ext-N via `/ze-rfc`) |
| 11 | Affects daemon comparison? | Per child | `docs/comparison.md` (OSPF-extension parity rows, both AFs) |
| 12 | Internal architecture changed? | Per child | `docs/architecture/wire/ospfv3.md` and the OSPF subsystem docs (per ext-N) |
| -- | Umbrella-level | This file | keep the Child Decomposition, Dependency / Build Order, and RFC Coverage tables current as children land or rest |

## Files to Create
- `plan/spec-ospf-ext-2-*.md` .. `plan/spec-ospf-ext-16-*.md` -- the child specs (ext-1 already written)
- (no feature files at the umbrella level) -- each child creates its own `internal/plugins/ospf/...` / `internal/plugins/ospf/v3/...` files and `test/ospf/*.ci` / `test/ospfv3/*.ci` / `test/interop/scenarios/ospf-*-frr/` / `test/interop/scenarios/ospfv3-*-frr/`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + the selected child |
| 2. Audit | Per-child Files/TDD |
| 3. Wiring phase | Per-child Wiring Test |
| 4. Implement (TDD) | Per-child Implementation Phases |
| 5. /ze-review gate | Per-child Review Gate |
| 6-14. | Standard flow per child |

### Implementation Phases

This umbrella is implemented by selecting and completing child specs in the
dependency order above. Per the spec-set rule, select children individually when
implementing; keep the umbrella pointed-to but do not implement the umbrella
directly.

1. **Phase: IPv4 opaque carrier** -- ext-1 (RFC 5250); the foundation that unblocks the IPv4 opaque chain.
2. **Phase: IPv4 opaque consumers (parallel after ext-1)** -- ext-2 (TE), ext-3 (RI), ext-4 (Extended Link/Prefix); plus the IPv4 halves of ext-9 (Grace-LSA) and ext-14 (debug).
3. **Phase: Both-AF + base-only features (parallel)** -- ext-7 (virtual links), ext-8 (NBMA/P2MP), ext-10 (BFD), ext-11 (LDP-IGP sync) on the shared engine; ext-12 (IPv4 Multi-Instance); ext-9 IPv6 half (native Grace-LSA); the IPv6 halves of ext-14 (debug).
4. **Phase: IPv6-only features (parallel)** -- ext-15 (multi-AF, on the reserved Instance-ID), ext-16 (IPsec).
5. **Phase: Segment Routing** -- ext-5; IPv4 half once ext-3 + ext-4 are done, IPv6 half adding its own v3 RI + RFC 8362 LSAs first.
6. **Phase: TI-LFA / LFA** -- ext-6 (both AFs), once ext-5 is done.
7. **Phase: L3VPN DN bit (gated)** -- ext-13 (IPv4), LAST, only once MPLS-L3VPN/VRF infrastructure exists.
8. **Per-child verification + interop** -- `make ze-verify` + FRR `ospfd` / `ospf6d` scenarios, owned by each child.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this umbrella |
|-------|----------------------------------|
| Completeness | Every child ext-1..ext-16 exists, cross-references its dependencies, records its address-family coverage, and matches the unified-engine layout and the two bases' Shared Contracts |
| Correctness | The dependency / build order is honoured (IPv4 opaque carrier first; IPv4 SR after RI + Extended; IPv6 SR adds its v3 RI + RFC 8362 LSAs first; TI-LFA after SR; ext-13 VRF-gated) |
| Naming | Each extension uses `ze_ospf_<ext>_*` (IPv4) / `ze_ospfv3_<ext>_*` (IPv6) metrics and `show ospf <noun>` / `show ospf ipv6 <noun>` subcommands; no existing series/command renamed |
| Data flow | Extensions attach at delivered seams; SR/TI-LFA install through the existing Loc-RIB path (both AFs); opaque LSAs never enter SPF; no second engine |
| Rule: plugin-self-containment | Each child's schema/help/doctor/commands live within the unified `ospf` plugin (`internal/plugins/ospf/...`); no v3 extension spelling leaks into a generic/central package or the IPv4 path; no child references a separate `ospfv3` plugin directory |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Umbrella + 16 child specs | `ls plan/spec-ospf-ext-*.md` |
| ext-1 written (Status:ready) | `grep -m1 '| Status |' plan/spec-ospf-ext-1-opaque-framework.md` |
| Each child cross-references its dependency | grep each child for its "Depends" row |
| Each child records its address family | grep each child for its "Address family" coverage |
| Rested set recorded | `grep -A20 'Out of scope (rested' plan/spec-ospf-ext-0-umbrella.md` |
| No second engine; IPv6 wire code in the v3 leaves | each child's files are under `internal/plugins/ospf/...` (no separate `ospfv3` plugin directory) |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Each extension's wire decode is bound-checked (IPv4 opaque TLVs, TE/RI/Extended sub-TLVs; IPv6 RI / extended / SR sub-TLVs, Grace-LSA body, multi-AF Instance-ID ranges); per child |
| Trust boundary | Opaque/extension LSAs flooded only to capable neighbours; received LSAs rely on the delivered OSPF authentication (IPv4 AuType, IPv6 RFC 7166 trailer or RFC 4552 IPsec); no new unauthenticated surface; ext-16 IPsec keys/SPIs handled by the kernel SA/SP path with no secret leakage to logs |
| Resource exhaustion | Extension stores share the delivered LSDB caps; a flood of extension LSAs cannot grow memory unbounded; per child |
| Consumer isolation | A consumer callback panic is recovered and counted (ext-1 contract); a bad extension cannot crash OSPF; a debug/inject path (ext-14) cannot corrupt the live LSDB |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the child that introduced it |
| Test fails wrong reason | Fix test setup |
| Test fails behavior mismatch | Re-read the child's RFC summary / Current Behavior |
| Build-order violation (IPv4 consumer before carrier) | STOP; reorder; the IPv4 carrier (ext-1) must land first |
| Build-order violation (SR labels before the v3 RI + extended LSAs, IPv6) | STOP; reorder; the IPv6 half of ext-5 must add its advertisement layer first |
| A second OSPF engine introduced (forking by version, e.g. a separate `ospfv3` plugin directory) | STOP; the delivered design is one unified engine; keep IPv6-specific wire/LSA code in the `_v6` strategies + the `internal/plugins/ospf/v3/{types,packet,transport}` leaves |
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
- The extension set has one IPv4-specific dependency root (ext-1, the opaque carrier), one self-contained IPv6 SR advertisement layer (ext-5 adds the v3 RI + RFC 8362 LSAs itself), and a wide fringe of both-AF features on the shared engine; capturing this split up front prevents scheduling SR or TE before its carrier, or forking a second engine for v3.
- Organising the umbrella by FEATURE (each row carrying an address-family column) rather than by protocol version matches the delivered reality: OSPF is one engine, like BGP, and most forwarding/protocol features (TI-LFA, BFD, LDP-IGP sync, virtual links, NBMA/P2MP, debug) are implemented once on the shared AF-neutral machinery and serve both families.
- "Rested" is a distinct status from "deferred child": deferred children are scheduled work, rested items are deliberately-absent decisions that need re-opening. Conflating them is how a rejected feature (or a forbidden second engine) silently creeps back.

## Core Insight
The OSPF extension landscape is a single feature set on ONE unified engine with
two address families. The IPv4 features form a dependency tree rooted at the RFC
5250 opaque carrier; the IPv6 features attach as native LSAs (SR adds its own RI +
RFC 8362 layer); and a broad set of forwarding/protocol features lives once on the
shared AF-neutral machinery and serves both families. The umbrella's job is to
make that ordering and the address-family coverage explicit, to keep every
extension attached at the shared/AF-specific seams (never forking a second
engine), and to record -- with rationale -- the features deliberately NOT
scheduled (including a second OSPF engine), so neither the build order nor the
resting decisions are re-litigated by accident.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One feature-organized umbrella for both AFs | Two umbrellas (separate ospfv3-ext) | OSPF is one engine with address families (the BGP model: no bgpv4, no separate ospfv3). A single umbrella matches the delivered unified engine and stops the v2/v3 drift the unification removed. This umbrella SUPERSEDES the retired `spec-ospfv3-ext-0-umbrella.md` |
| IPv4 opaque carrier (ext-1) is the single IPv4 foundation | Per-extension opaque handling | One generic RFC 5250 carrier keeps flooding/scope/O-bit logic in one place; IPv4 consumers stay self-contained |
| IPv6 SR (ext-5) adds the v3 RI + RFC 8362 LSAs itself | A separate opaque carrier for v3, or separate RI/extended children | OSPFv3 has no opaque carrier; the v3 RI and Extended LSAs (RFC 8362) exist only to carry SR state, so they are scoped inside ext-5 as native `v3/packet` bodies |
| SR depends on RI + Extended Prefix/Link (IPv4) | SR carries its own advertisements | RFC 8665 explicitly reuses the RI (RFC 7770) and Extended (RFC 7684) LSAs; duplicating them would diverge from FRR/interop |
| ext-13 recorded as VRF-gated/BLOCKED (IPv4) | Schedule it "later" with the rest | It has a hard external dependency (MPLS-L3VPN/VRF) absent from Ze; marking it merely "later" risks premature, unwireable work |
| ext-15 (multi-AF) on the reserved IPv6 Instance-ID | Re-open the RFC 5340 codec to add AF awareness | The IPv6 base reserved a validated Instance-ID field precisely for this; ext-15 maps AF to Instance-ID ranges (RFC 5838 §2.4) and spawns one engine instance per AF without a codec change |
| ext-16 (IPsec) is a separate auth path | Extend the delivered RFC 7166 trailer code | RFC 4552 IPsec AH/ESP is a kernel SA/SP mechanism, structurally distinct from the in-packet trailer; folding it in would conflate two auth models |
| Debug folds in ospfclient (ext-14, both AFs) | Standalone ospfclient Unix-socket daemon | The useful inject/observe capability fits in-process (IPv4 via the ext-1 registry, IPv6 via the v3 LSDB); a separate daemon adds a socket and trust boundary for no benefit |
| A second OSPF engine rested (forbidden) | Fork the engine per protocol version | The delivered design (`plan/learned/972-ospf-af-unify.md`) is one engine: the FSM, flooding, DR, SPF, and LSDB sequencing are AF-neutral and shared. Forking reintroduces drift |

## Known Limitations
- This umbrella tracks OSPF extensions for BOTH address families of the one engine. It SUPERSEDES the retired `spec-ospfv3-ext-0-umbrella.md`. There is no separate OSPFv3 product or plugin; IPv4 and IPv6 are address families, exactly as BGP has no `bgpv4`.
- The umbrella is a coordination document: it has no feature code, no tests, and no acceptance criteria that it implements itself. Completion is defined by its children, not by this file. It is never marked "done" while a tracked child is open.
- The IPv4 opaque chain (ext-2, ext-3, ext-4, the IPv4 half of ext-5/ext-6, the IPv4 Grace-LSA of ext-9, the IPv4 decode half of ext-14) is hard-blocked on ext-1. Build-order violations (e.g. starting IPv4 SR before RI + Extended Prefix/Link) produce specs that cannot be wired and must be rejected at planning time.
- OSPFv3 has no opaque-LSA carrier (RFC 5340 carries extensions as native LSAs); the IPv4 ext-1 opaque framework has no v3 analogue, and the IPv6 half of ext-5 (SR) must add the v3 Router Information + Extended LSAs (RFC 8362) itself before it can compute SR labels.
- ext-13 (IPv4 L3VPN PE-CE DN bit) cannot be implemented until Ze gains MPLS-L3VPN / VRF infrastructure; it is BLOCKED, not merely sequenced last. Implementing it before VRF lands is a scope error.
- The "rested" items (SNMP MIB v2 RFC 4750 + v3 RFC 5643, TOS, QoS, multi-area adjacencies, Flood Reduction / DoNotAge, the standalone ospfclient daemon, and a second separate OSPF engine) are deliberately absent. Reviving any of them requires a fresh design decision and a new spec, not a quiet add to an existing child. RFC 5838 multi-AF and RFC 4552 IPsec are explicitly NOT rested -- they are ext-15 and ext-16. Forking a second OSPF engine is forbidden; a child that introduces one (or references a separate `ospfv3` plugin directory) must be rejected at planning time.

## RFC Documentation

Per-RFC implementation summaries (the `/ze-rfc` deep output) and the short
house-format summaries under `rfc/short/` are produced by each CHILD spec at its
own implementation time, for the RFCs whose normative detail that child's code
enforces. This umbrella adds no RFC enforcement code and therefore carries no
`// RFC NNNN Section X.Y` annotations itself; it only records the RFC-to-child
mapping in "RFC Coverage" below. The pre-existing summaries are
`rfc/short/rfc5250.md` (IPv4 opaque, consumed by ext-1), `rfc/short/rfc2328.md`
and `rfc/short/rfc5340.md` (the two bases, referenced by ext-7/ext-8 records),
and `rfc/short/rfc5838.md` / `rfc/short/rfc4552.md` (consumed by ext-15 / ext-16).

### RFC Coverage (per child)
| Child | Address family | RFC(s) | Summary status |
|-------|----------------|--------|----------------|
| ext-1 | IPv4 | RFC 5250 | CREATED `rfc/short/rfc5250.md` |
| ext-2 | IPv4 | RFC 3630, RFC 5392 | created by ext-2 |
| ext-3 | IPv4 | RFC 7770 | created by ext-3 |
| ext-4 | IPv4 | RFC 7684 | created by ext-4 |
| ext-5 | both | RFC 8665 (IPv4), RFC 8666, RFC 8362 (IPv6) | created by ext-5 |
| ext-6 | both | RFC 5286 + TI-LFA draft | created by ext-6 |
| ext-7 | both | RFC 2328 §15 (IPv4), RFC 5340 §4.2 (IPv6) | covered by `rfc/short/rfc2328.md` + `rfc/short/rfc5340.md` (delivered) |
| ext-8 | both | RFC 2328 (IPv4 network types), RFC 5340 (IPv6 network types) | covered by `rfc/short/rfc2328.md` + `rfc/short/rfc5340.md` (delivered) |
| ext-9 | IPv4 + IPv6 | RFC 3623 (IPv4), RFC 5187 (IPv6) | created by ext-9 |
| ext-10 | both | RFC 5880, RFC 5881 | created by ext-10 |
| ext-11 | both | RFC 5443, RFC 6138 | created by ext-11 |
| ext-12 | IPv4 | RFC 6549 | created by ext-12 |
| ext-13 | IPv4 | RFC 4576, RFC 4577 | created by ext-13 (when unblocked) |
| ext-14 | both | (tooling; no new RFC) | n/a |
| ext-15 | IPv6 | RFC 5838 | `rfc/short/rfc5838.md`; refreshed by ext-15 via `/ze-rfc` |
| ext-16 | IPv6 | RFC 4552 | `rfc/short/rfc4552.md`; refreshed by ext-16 via `/ze-rfc` |

## Implementation Summary

### What Was Implemented
- Pending: the umbrella authoring deliverable is the child set + coordination; per-child implementation is downstream.

### Bugs Found/Fixed
- Pending: fill as children are implemented.

### Documentation Updates
- Pending: each child updates docs as it lands; the umbrella keeps the decomposition / build-order / RFC-coverage tables current.

### Deviations from Plan
- Pending: fill if the child set or ordering changes.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Single feature-organized umbrella for both address families | Done | Child Decomposition table (Address family column) | ext-1..ext-16 |
| Supersede the retired ospfv3-ext umbrella | Done | header note + Known Limitations + Key Design Decisions | SUPERSEDES `spec-ospfv3-ext-0-umbrella.md` |
| Fix the dependency / build order | Done | Dependency / Build Order | IPv4 opaque chain + IPv6 self-contained SR + both-AF set + VRF-gated ext-13 |
| Record the rested set with rationale (merged from both umbrellas) | Done | Out of scope (rested) table | SNMP MIB v2+v3, TOS/QoS/multi-area-adjacency, Flood-Reduction/DoNotAge, ospfclient, a second separate OSPF engine |
| Per-child implementation | (pending) | each `plan/spec-ospf-ext-N-*.md` | downstream |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1..AC-7 | (coordination) | this umbrella's tables | child ACs are detailed per child |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| (per child) | (pending) | `internal/plugins/ospf/...`, `internal/plugins/ospf/v3/...`, `test/ospf/...`, `test/ospfv3/...` | per ext-N |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `plan/spec-ospf-ext-1-opaque-framework.md` | Written | Status:ready |
| `plan/spec-ospf-ext-2-*.md` .. `plan/spec-ospf-ext-16-*.md` | (this batch) | Status:ready |
| `internal/plugins/ospf/` + `internal/plugins/ospf/v3/` (extensions) | (pending) | per child |

### Audit Summary
- **Total items:** umbrella coordination (this deliverable) + downstream per-child implementation
- **Done:** child decomposition (16 children, both AFs), dependency / build order, merged rested set, supersession of the v3-ext umbrella
- **Partial:** 0
- **Skipped:** 0
- **Changed:** per-child implementation is downstream, tracked per child

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Single feature-organized OSPF extension child set (both AFs) exists and is internally consistent | spec files + cross-references | `ls plan/spec-ospf-ext-*.md`; the Child Decomposition table's Address family column; each child's "Depends" row |
| The build order is captured | this file | Dependency / Build Order section (IPv4 opaque chain + IPv6 self-contained SR + both-AF set + ext-13 VRF gate) |
| The merged rested set is recorded with rationale | this file | Out of scope (rested) table (SNMP v2+v3, TOS/QoS/multi-area, Flood-Reduction, ospfclient, second engine) |
| The umbrella supersedes the retired v3-ext umbrella and frames OSPF as one engine with address families | this file | header note + Known Limitations + Key Design Decisions |
| Per-child implementation | unit + functional + interop | (filled during implementation per child) |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | pending | `/ze-review` on the umbrella + child set has not run after authoring | `plan/spec-ospf-ext-*.md` | run before implementation begins |

### Fixes applied
- Pending: fill after `/ze-review` produces concrete findings.

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
| `plan/spec-ospf-ext-0-umbrella.md` | (verify) | `ls plan/spec-ospf-ext-0-umbrella.md` |
| `plan/spec-ospf-ext-1-opaque-framework.md` | (verify) | `ls plan/spec-ospf-ext-1-opaque-framework.md` |
| `plan/spec-ospf-ext-2-*.md` .. `plan/spec-ospf-ext-16-*.md` | (verify) | `ls plan/spec-ospf-ext-*.md` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | each child exists + cross-references + records its AF | `ls plan/spec-ospf-ext-*.md`; grep each child's Depends + Address-family rows |
| AC-2 | build order honoured | Dependency / Build Order table |
| AC-7 | code within the unified `ospf` engine; no separate `ospfv3` plugin directory | per child Files to Create/Modify |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| (downstream) | `test/ospf/*.ci`, `test/ospfv3/*.ci` (per ext-N) | filled during implementation |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| (umbrella) | n/a -- coordination doc; assumptions are validated per child | per ext-N Assumptions tables |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| (downstream) | docs updated per child | filled during implementation |

## Checklist

### Goal Gates (MUST pass)
- [ ] All 16 child specs written and cross-referenced (ext-1 written; ext-2..ext-16 this batch), each with its address-family coverage
- [ ] Dependency / build order captured and consistent (IPv4 opaque chain; IPv6 self-contained SR; both-AF shared-engine set; VRF-gated ext-13)
- [ ] Out-of-scope (rested) set recorded with rationale, merged from both umbrellas
- [ ] RFC 5838 + RFC 4552 promoted in scope (ext-15 / ext-16), not rested
- [ ] AC-1..AC-7 demonstrated by the umbrella's tables
- [ ] End-to-End User Stories each map to a child + a downstream test
- [ ] Wiring Test table complete (umbrella-level; detailed wiring per child)
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes (downstream, per child)
- [ ] Feature code integrated (`internal/plugins/ospf/` + `internal/plugins/ospf/v3/`) (downstream, per child)
- [ ] Documentation Update Checklist answered (per child as each lands)
- [ ] Critical Review passes

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added (downstream, per child)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (no extension built before its dependency)
- [ ] No speculative features (rested table honoured; no second engine)
- [ ] Single responsibility per child
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (both-AF features on the shared engine; base-only features independent of the opaque chain; no v3 LS-type leakage into the IPv4 path)

### TDD
- [ ] Tests written (downstream, per child)
- [ ] Tests FAIL (paste output) (downstream, per child)
- [ ] Tests PASS (paste output) (downstream, per child)
- [ ] Boundary tests for all numeric inputs (per child)
- [ ] Functional tests for end-to-end behavior (per child)
- [ ] Interop tests for protocol features (per child)
- [ ] Goal Validation table filled

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled (downstream)
- [ ] Implementation Audit filled
- [ ] Mistake Log escalation reviewed
- [ ] Write learned summary to `plan/learned/NNN-ospf-ext-0-umbrella.md` (at set completion)
- [ ] Summary included in commit
