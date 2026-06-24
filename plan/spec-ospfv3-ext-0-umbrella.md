# Spec: ospfv3-ext-0 -- OSPFv3 Extensions (Umbrella)

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-ospfv3-0-umbrella.md |
| Phase | - |
| Updated | 2026-06-24 |

> Umbrellas are living tracking documents, not implementable specs. This file
> coordinates the OSPFv3 extension follow-ups deferred from the OSPFv3 base
> umbrella (`plan/spec-ospfv3-0-umbrella.md`); it carries NO acceptance criteria
> and NO feature code of its own. Each child spec listed below is the
> implementable unit. The "implementable-spec" sections below (Current Behavior,
> Data Flow, Wiring Test, TDD Test Plan, Checklist) are framed at umbrella level
> -- they describe the child set, mirroring the delivered OSPFv2-extension
> umbrella (`plan/spec-ospf-ext-0-umbrella.md`), not a feature this file
> implements. Do not mark this umbrella "done": it closes only when every child
> it tracks is closed (or explicitly re-rested).

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/spec-ospfv3-0-umbrella.md` -- the OSPFv3 base umbrella; its "Out of Scope" table (the source list of every follow-up this umbrella tracks: RFC 5838 multi-AF, RFC 4552 IPsec, virtual links, opaque/TE/SR/GR/BFD, SNMP MIB), the package layout under `internal/plugins/ospfv3/`, the scope-aware LSA model, the Instance-ID plumbing the base reserved, and the FIB-install-via-Loc-RIB path
4. `plan/spec-ospf-ext-0-umbrella.md` -- the DELIVERED OSPFv2-extension umbrella; the umbrella VOICE and structure this file mirrors (scope-boundary table, child decomposition, dependency / build order, "Out of scope (rested)" table). Reference for shared design PATTERNS ONLY -- never shared code: RFC 5340 mandates version-specific packet / LSA / SPF for v3, distinct from the v2 ext set
5. `docs/research/ospf-implementation-guide.md` §15 (OSPFv3 differences, do-not-unify) and the SR / RI / Extended-LSA sections -- the extension landscape and the v3-vs-v2 separation rationale this umbrella records
6. `internal/plugins/ospfv3/` (delivered base via the OSPFv3 base umbrella's child set) -- the plugin the extensions extend: scope-aware LSDB, RFC 5340 codec, IPv6 transport, ISM/NSM, SPF, and the reserved Instance-ID field ext-1 (multi-AF) consumes

## Task

Coordinate the OSPFv3 extension follow-ups that the OSPFv3 base umbrella
(`plan/spec-ospfv3-0-umbrella.md`) deliberately rested under its "Out of Scope"
table. The base delivers IPv6 unicast OSPFv3: RFC 5340 packet / LSA codec, raw
IPv6 transport, the scope-aware LSDB, ISM / NSM adjacency, SPF with prefix
attachment, inter-area / external / stub / NSSA behaviour, the RFC 7166
authentication trailer, and FRR `ospf6d` interop. It did NOT deliver multiple
address families, IPsec AH/ESP authentication, virtual links, Graceful Restart,
BFD integration, Segment Routing, the NBMA / point-to-multipoint network types,
or a dedicated debug / introspection surface for OSPFv3.

This umbrella enumerates those follow-ups as a coordinated child set, fixes the
build order between them (all children build on the delivered OSPFv3 base; SR
needs the v3 Router Information and extended LSAs added first; multi-AF consumes
the reserved Instance-ID plumbing), and records -- in the "Out of scope
(rested)" table -- the OSPFv3 features and management surfaces that were
considered and deliberately NOT scheduled, each with its rationale, so a future
agent does not silently assume they are done or quietly re-add them.

### Scope boundary vs the delivered OSPFv3 base

| Concern | Owned by | This umbrella |
|---------|----------|---------------|
| OSPFv3 base protocol (RFC 5340 codec, IPv6 transport, scope-aware LSDB, ISM/NSM, SPF + prefix attachment, inter-area / external / stub / NSSA, RFC 7166 auth trailer, FRR interop) | `plan/spec-ospfv3-0-umbrella.md` (delivered via its child set) | Treated as a STABLE foundation; not re-opened. Extensions attach at documented seams (LSDB scopes, flooding, SPF route table, ISM/NSM, the IPv6 transport, the reserved Instance-ID field, the auth path) |
| Multiple address families + IPsec auth (the two items the base explicitly listed as out of scope but IN SCOPE here) | this umbrella (ext-1, ext-2) | Tracked + ordered here |
| Protocol features the base rested (virtual links, GR, BFD, SR, NBMA/P2MP) | this umbrella (ext-3..ext-7) | Tracked + ordered here |
| Extension debug / introspection tooling for OSPFv3 | this umbrella (ext-8) | Tracked here |
| OSPFv2 and its extensions (RFC 5250 opaque carrier, TE, RI, Extended Link/Prefix, SR, TI-LFA, virtual links, NBMA, GR, BFD, LDP-IGP sync, Multi-Instance, L3VPN DN bit) | `plan/spec-ospf-0-umbrella.md` + `plan/spec-ospf-ext-0-umbrella.md` | NOT this umbrella; OSPFv2 is a separate edge plugin. v3 reuses v2 PATTERNS only, never v2 packet / LSA / SPF code (RFC 5340 mandate) |

### Target scope / decisions

| Lever | Decision | Effect on the child set |
|-------|----------|-------------------------|
| No shared v2/v3 wire code | **Every child lives under `internal/plugins/ospfv3/`** | RFC 5340 gives OSPFv3 a different header, LSA registry, flooding-scope model, prefix encoding, and checksum. No child shares packet / LSA / SPF code with the OSPFv2 ext set; version-specific branches would leak into both implementations. The v2 ext umbrella is a PATTERN reference, not a code dependency |
| Multi-AF uses the reserved Instance-ID | **ext-1 (RFC 5838) consumes the base's Instance-ID plumbing** | The OSPFv3 base reserved an explicit, validated Instance-ID field precisely so multi-AF could attach later without re-opening the codec. ext-1 maps AF to the Instance-ID ranges (RFC 5838 §2.4) and adds per-AF topologies; it does not redefine the header |
| SR needs the v3 advertisement layer first | **ext-6 (RFC 8666) requires adding v3 Router Information + extended LSAs first** | OSPFv3 Segment Routing rides the v3 Router Information LSA (SR-Algorithm / SRGB / SRLB) and the OSPFv3 Extended-LSA TLV containers (E-Intra-Area-Prefix / E-Router / E-Inter-Area-Prefix, RFC 8362) carrying Prefix-SID / Adjacency-SID. The ext-6 child states it must add the v3 RI + extended LSAs before SR can compute labels -- there is no opaque carrier in v3 (RFC 5340 carries extensions as native LSAs) |
| IPsec auth is independent of the trailer | **ext-2 (RFC 4552) is a separate auth path from the delivered RFC 7166 trailer** | The base delivered RFC 7166 (the in-packet Authentication Trailer). RFC 4552 IPsec AH/ESP is a distinct mechanism that needs kernel IPsec policy (SA/SP) wiring; it is its own child, not an extension of the trailer code |
| Base-independent features parallelise | **ext-2, ext-3, ext-4, ext-5, ext-7 depend only on the delivered base** | IPsec auth, virtual links, GR, BFD, and NBMA/P2MP touch the base protocol (LSDB, ISM/NSM, transport, auth path) but not the SR advertisement chain; they can proceed in parallel with ext-1 |
| OSPFv2 excluded | **out of this umbrella entirely** | OSPFv2 and its extension set live under `plan/spec-ospf-0-umbrella.md` + `plan/spec-ospf-ext-0-umbrella.md`; this umbrella is OSPFv3-only and shares no wire code with them |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
- [ ] `plan/spec-ospfv3-0-umbrella.md` -- the OSPFv3 base umbrella this set extends
  -> Decision: extensions attach at the documented seams (scope-aware LSDB, flooding, SPF route table, ISM/NSM, the IPv6 transport, the reserved Instance-ID field, the RFC 7166 auth path); the base is not re-opened
  -> Constraint: the base's "Out of Scope" table is the authoritative source list of every follow-up this umbrella tracks -- do not add a child that is not derived from it (or explicitly record a new resting decision). RFC 5838 multi-AF and RFC 4552 IPsec, listed out-of-scope in the base, are IN SCOPE here as ext-1 and ext-2
- [ ] `plan/spec-ospf-ext-0-umbrella.md` -- the delivered OSPFv2-extension umbrella
  -> Decision: mirror its umbrella structure and voice (scope-boundary table, child decomposition, dependency / build order, rested table); reference it for shared design PATTERNS only
  -> Constraint: NEVER share code with the v2 ext set. RFC 5340 mandates version-specific packet / LSA / SPF for OSPFv3; there is no opaque carrier in v3 (extensions are native LSAs), so the v2 ext-1 opaque framework has no v3 analogue
- [ ] `docs/research/ospf-implementation-guide.md` §15 (OSPFv3 differences + do-not-unify) and the SR / RI / Extended-LSA sections
  -> Constraint: FRR ships `ospfd` and `ospf6d` as separate daemons; Ze follows that separation. The guide's SR-needs-RI-and-extended-LSAs note is the basis for the ext-6 build-order gate
- [ ] The ext-1..ext-8 child specs (this batch) -- each child's own Required Reading governs its implementation; the umbrella points at them but does not duplicate their detail

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc5838.md` (multi-AF) and `rfc/short/rfc4552.md` (IPsec) -- noted in the base umbrella as deferred references; consumed by ext-1 and ext-2 respectively
  -> Constraint: each remaining child creates its own RFC summary via `/ze-rfc` before that child is implemented (RFC 5187 GR, RFC 5881 BFD, RFC 8666 SR, RFC 8362 v3 extended LSAs for ext-6, RFC 5340 §4.2 virtual links / network types for ext-3 and ext-7); the umbrella tracks the mapping (see RFC Coverage) but enforces no RFC text itself

**Key insights:** (minimal context to resume after compaction)
- All eight children build on the delivered OSPFv3 base; there is no opaque carrier in v3 (RFC 5340 carries extensions as native LSAs), so the chain shape differs from the v2 ext set.
- ext-1 (multi-AF) consumes the reserved Instance-ID plumbing; ext-6 (SR) must add the v3 Router Information + extended LSAs (RFC 8362) before it can compute SR labels.
- ext-2, ext-3, ext-4, ext-5, ext-7 are base-only and parallelise alongside ext-1.
- The "rested" items (SNMP OSPFV3-MIB, any shared v2/v3 packet or LSA package, the v3 equivalents of the v2-rested niche items) are deliberately absent and need a fresh decision to revive.

## Child Decomposition

Each child is an independent, implementable spec written as Status:ready in this
same batch. The umbrella owns the coordination, not the implementation.

| Child | Title | RFC(s) | Depends on | One-line scope |
|-------|-------|--------|------------|----------------|
| ext-1 | Multiple address families | RFC 5838 | base | Map address families to OSPFv3 Instance-ID ranges (§2.4) using the base's reserved Instance-ID plumbing; per-AF topologies and route install; AF-aware LSDB / SPF without re-opening the RFC 5340 codec |
| ext-2 | IPsec AH/ESP authentication | RFC 4552 | base | OSPFv3 manual-keyed IPsec AH/ESP as a distinct auth path from the delivered RFC 7166 trailer; kernel IPsec SA/SP policy wiring for the OSPFv3 transport, per-interface SPI/key config |
| ext-3 | Virtual links | RFC 5340 §4.2 | base | Virtual-link adjacencies across a transit area to repair a partitioned or non-contiguous backbone; virtual-interface ISM/NSM and the OSPFv3 virtual-link records over IPv6 transport |
| ext-4 | Graceful Restart | RFC 5187 | base | OSPFv3 Grace-LSA origination + the GR helper (and restarter) so a neighbour's restart does not tear down forwarding; the v3 Grace-LSA is a native link-scope LSA (no opaque carrier in v3) |
| ext-5 | BFD for OSPFv3 | RFC 5881 (IPv6 single-hop) | base | Integrate Ze's existing BFD engine: register OSPFv3 adjacencies as BFD clients over IPv6 single-hop, drive NSM down on a BFD session failure for sub-second failure detection |
| ext-6 | Segment Routing | RFC 8666 | base, **adds v3 Router Information + extended LSAs first** | OSPFv3 SR control plane: add the v3 Router Information LSA (SR-Algorithm / SRGB / SRLB) and the OSPFv3 Extended-LSA containers (RFC 8362) carrying Prefix-SID / Adjacency-SID, then MPLS label computation and FIB programming for SR paths |
| ext-7 | NBMA + point-to-multipoint | RFC 5340 (network types) | base | The OSPFv3 NBMA and point-to-multipoint network types: static neighbour config, NBMA DR/BDR eligibility, P2MP per-neighbour Hellos and host-route origination over IPv6 transport |
| ext-8 | Debug & introspection tooling | (no new RFC; tooling over the v3 LSDB / codec) | base | Extension-wide debug/introspection for OSPFv3: decode + inspect v3 LSAs (including the RI / extended / SR / Grace LSAs added by ext-4/ext-6), inject test LSAs for functional testing, and the show/diagnostic surface for every v3 extension above |

## Dependency / Build Order

Every child builds on the delivered OSPFv3 base. Unlike the OSPFv2 extension set
there is NO opaque carrier root: RFC 5340 carries extensions as native LSAs, so
the only intra-extension dependency is SR's need for the v3 advertisement layer,
which ext-6 adds itself.

| Child | Depends on |
|-------|-----------|
| ext-1 (multiple address families) | base (consumes the reserved Instance-ID plumbing) |
| ext-2 (IPsec AH/ESP auth) | base only |
| ext-3 (virtual links) | base only |
| ext-4 (Graceful Restart) | base only |
| ext-5 (BFD) | base only |
| ext-6 (Segment Routing) | base + adds v3 Router Information + extended LSAs (RFC 8362) as a prerequisite within the child |
| ext-7 (NBMA + P2MP) | base only |
| ext-8 (debug & introspection) | base (and decodes the LSAs ext-4 / ext-6 add) |

**No opaque chain:** OSPFv3 (RFC 5340) carries extension state as native,
scope-aware LSAs, not as opaque Type 9/10/11 LSAs. The v2 ext-1 opaque framework
therefore has no v3 analogue, and none of these children depend on an opaque
carrier.

**SR self-contained prerequisite:** ext-6 (Segment Routing) does NOT depend on
another ext-N child; instead it states, within its own scope, that it must first
add the OSPFv3 Router Information LSA and the OSPFv3 Extended-LSA TLV containers
(RFC 8362) before it can carry SRGB/SRLB and Prefix-SID/Adjacency-SID. That
advertisement layer is part of ext-6, not a separate child.

**Multi-AF uses reserved plumbing:** ext-1 (RFC 5838) attaches to the
Instance-ID field the base deliberately reserved and validated; it adds per-AF
topologies without re-opening the RFC 5340 codec.

**Base-only (parallel) set:** ext-2, ext-3, ext-4, ext-5, ext-7 depend only on
the delivered base; they can proceed concurrently with ext-1 and ext-6.

**Recommended build order:**

1. Any base-only child first, in parallel: **ext-1** (multi-AF, on the reserved
   Instance-ID), **ext-2** (IPsec), **ext-3** (virtual links), **ext-4** (GR),
   **ext-5** (BFD), **ext-7** (NBMA/P2MP).
2. **ext-6** (Segment Routing) -- adds the v3 RI + extended LSAs as its first
   internal phase, then the SR control plane.
3. **ext-8** (debug & introspection) -- best last so it can decode the LSAs that
   ext-4 (Grace) and ext-6 (RI / extended / SR) add; the core decode surface is
   base-only and can start any time.

Condensed: `base -> {ext-1, ext-2, ext-3, ext-4, ext-5, ext-7}` in parallel;
`ext-6` adds its own v3 RI + extended-LSA layer then SR; `ext-8` last (decodes
the ext-4 / ext-6 LSAs).

## Out of scope (rested, noted here so it is not silently assumed done)

These OSPFv3 features and management surfaces were considered for this extension
set and deliberately NOT scheduled. Each is recorded with its rationale so a
future agent does not assume it is implemented, nor quietly re-add it without
revisiting the reasoning. A "rested" item differs from a "deferred" child:
rested items are not part of any ext-N child and need a fresh decision (and
likely a new spec) to revive.

> NOTE: RFC 4552 IPsec authentication and RFC 5838 multiple address families are
> IN SCOPE here as ext-2 and ext-1 respectively; they are NOT rested. The base
> OSPFv3 umbrella listed them as out of scope, and this umbrella picks them up.

| Rested item | RFC(s) | Rationale for resting |
|-------------|--------|-----------------------|
| SNMP OSPFV3-MIB | RFC 5643 | Wrong management plane. Ze's management plane is YANG / gNMI / CLI / web, not SNMP. There is no SNMP agent to host the MIB; OSPFv3 state is already exposed through `show ipv6 ospf` and Prometheus metrics. Adding SNMP would duplicate the existing surface against a transport Ze does not speak |
| Any shared OSPFv2 / OSPFv3 packet or LSA package | (architecture decision; RFC 5340 vs RFC 2328 wire contracts) | FORBIDDEN. OSPFv2 and OSPFv3 have different headers, LSA registries, flooding-scope models, prefix encodings, and checksums (RFC 5340 vs RFC 2328). A shared package would force version-specific branches into both implementations, leaking v2 concerns into v3 code and vice-versa. FRR keeps `ospfd` and `ospf6d` separate for the same reason; Ze follows that separation. The v2 ext umbrella is a PATTERN reference only |
| OSPFv3 equivalents of the v2-rested niche items (TOS/QoS routing, OSPFv3 Flood Reduction / demand-circuit DoNotAge) | (per-item; deprecated / experimental / optimisation) | The same rationale that rested these for OSPFv2 applies to OSPFv3: TOS routing is deprecated with no interop value; QoS routing is experimental with no interop partner; Flood Reduction / DoNotAge is a pure optimisation for very large stable LSDBs with subtle stale-LSA failure modes. None has an OSPFv3 child; the base's normal LSRefresh/MaxAge behaviour is correct and safe. Reviving any needs a fresh decision |

## Current Behavior (MANDATORY)

**Source files read:** (architecture survey; each child spec reads its own targets)
<!-- Same rule: never tick [ ] to [x]. Write -> Constraint: annotations instead. -->
- [ ] `internal/plugins/ospfv3/lsdb/` (delivered via the base umbrella's child set) -- the scope-aware LSDB: link-local / area / AS scopes, origination, flooding, aging, ack handling; the seam the v3 extensions extend (Grace-LSA for ext-4, RI / extended / SR LSAs for ext-6)
  -> Constraint: extensions ADD scope-aware LSA types and origination at this seam; they do not rewrite the delivered base-LSA routing
- [ ] `internal/plugins/ospfv3/packet/` -- the delivered RFC 5340 codec: 16-byte header, IPv6 pseudo-header checksum, scope-aware LS types, topology/prefix separation, padded prefix encoding
  -> Constraint: ext-6 adds the v3 Router Information + Extended-LSA (RFC 8362) bodies on top of the existing scope-aware decode; it does not re-open the base codec. ext-1 keeps the Instance-ID field as the base defined it
- [ ] `internal/plugins/ospfv3/spf/` -- the delivered SPF over Router-LSAs / Network-LSAs with prefix attachment from Intra-Area-Prefix-LSAs and the IPv6 Loc-RIB install
  -> Constraint: SR (ext-6) READS the SPF route table and adds label/FIB computation; ext-1 (multi-AF) adds per-AF topologies; neither feeds extension LSAs into the SPF graph as topology vertices unless RFC 5340 defines them as such
- [ ] `internal/plugins/ospfv3/transport/` + `iface/` + `neighbor/` -- the delivered raw IPv6 proto 89 transport, the ISM (Hello, DR/BDR, Interface ID) and the NSM (DD, LS Request, adjacency Full); the seams ext-3 (virtual links), ext-5 (BFD client registration), ext-7 (NBMA/P2MP network types) extend
  -> Constraint: extensions extend ISM/NSM and the network-type handling additively; the delivered adjacency state machine and IPv6 transport are preserved
- [ ] `internal/plugins/ospfv3/auth/` -- the delivered RFC 7166 Authentication Trailer (AT-bit, SA config, sign/verify, 64-bit sequence anti-replay)
  -> Constraint: ext-2 (RFC 4552 IPsec) is a SEPARATE auth path (kernel IPsec SA/SP), not an extension of the trailer code; the trailer remains the in-packet mechanism
- [ ] `internal/plugins/ospfv3/register.go` + the reserved Instance-ID plumbing -- the delivered registration + lifecycle; ext-1 consumes the reserved, validated Instance-ID field; later extensions register their own commands / schema / doctor checks
  -> Constraint: each extension is self-contained (`ai/rules/plugin-self-containment.md`); removing a child removes all its registration cleanly. No v3 extension spelling appears in the OSPFv2 plugin or any generic/central package

**Behavior to preserve:** (the delivered OSPFv3 base is a stable foundation)
- The delivered RFC 5340 wire codec, the scope-aware LSDB (link-local / area / AS), the SPF + prefix-attachment behaviour, the IPv6 raw-transport, the ISM/NSM, and the RFC 7166 auth trailer.
- All existing OSPFv3 functional and FRR `ospf6d` interop tests: every extension is additive; a router with no extension enabled behaves exactly as the delivered base.
- The reserved, validated Instance-ID field (ext-1 attaches to it without redefining the header) and the IPv6 Loc-RIB FIB-install path (SR installs through the SAME seam).
- The canonical OSPFv3 metric set and the command-YANG ownership model; each extension adds its own `ze_ospfv3_<ext>_*` series and its own `show ipv6 ospf <noun>` subcommands, it does not rename existing ones.

**Behavior to change:** (this umbrella changes NONE directly)
- None -- the umbrella implements nothing. Each child changes behaviour additively, documented in that child's own "Behavior to change". The umbrella only coordinates ordering and records the rested set.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

(Umbrella-level: the data paths each child plugs into. Each child carries its own detailed Data Flow.)

### Entry Point
- Children are selected and implemented in the build order above; the umbrella's "input" is the set of follow-ups from the base OSPFv3 umbrella's out-of-scope table (with RFC 5838 and RFC 4552 promoted in scope), plus the resting decisions captured here.
- At runtime, every extension enters through the delivered OSPFv3 plugin's existing entry points: LSAs via the scope-aware LSDB receive path, IPv6 datagrams via the raw transport, config via the YANG tree, BFD via the existing engine integration seam, the Instance-ID demux for multi-AF.

### Transformation Path
1. **Base delivered:** OSPFv3 (RFC 5340 codec, IPv6 transport, scope-aware LSDB, ISM/NSM, SPF + prefix attachment, inter-area / external / stub / NSSA, RFC 7166 trailer) is complete and stable (the source of every seam below).
2. **Multi-AF:** ext-1 (RFC 5838) maps address families to Instance-ID ranges using the reserved field, adding per-AF topologies and route install without re-opening the codec.
3. **IPsec auth:** ext-2 (RFC 4552) adds a kernel-IPsec SA/SP path for the OSPFv3 transport, distinct from the delivered RFC 7166 trailer.
4. **Base-only protocol features:** ext-3 (virtual links), ext-4 (Graceful Restart, native Grace-LSA), ext-5 (BFD), ext-7 (NBMA/P2MP network types) extend the delivered ISM/NSM/transport/LSDB directly.
5. **SR advertisement + control plane:** ext-6 (RFC 8666) adds the v3 Router Information LSA + the Extended-LSA TLV containers (RFC 8362) for SRGB/SRLB and Prefix-SID/Adjacency-SID, then reads the SPF route table to compute SR labels and program SR paths via the delivered IPv6 Loc-RIB seam.
6. **Debug surface:** ext-8 decodes and injects v3 LSAs (including the RI / extended / SR / Grace LSAs the other children add) for introspection and functional testing.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Base <-> multi-AF | ext-1 attaches at the reserved Instance-ID field; per-AF LSDB/SPF topologies without a codec change | [ ] |
| Base transport <-> IPsec | ext-2 wires kernel IPsec SA/SP policy onto the OSPFv3 raw IPv6 transport; distinct from the RFC 7166 trailer path | [ ] |
| Base ISM/NSM <-> virtual link / NBMA / P2MP | ext-3 and ext-7 add network-type and virtual-interface handling at the delivered ISM/NSM seam | [ ] |
| Base LSDB <-> Grace / RI / extended / SR LSAs | ext-4 and ext-6 add native scope-aware LSA types and origination at the delivered LSDB seam | [ ] |
| SR / SPF <-> Loc-RIB (FIB) | ext-6 SR forwarding state installs through the SAME delivered IPv6 `locrib.Path` seam, never a second FIB path | [ ] |
| Engine <-> BFD | ext-5 registers OSPFv3 adjacencies as BFD clients over IPv6 single-hop via the existing BFD engine | [ ] |

### Integration Points
- `internal/plugins/ospfv3/lsdb` (scopes + flooding + origination) -- ext-4 (Grace-LSA) and ext-6 (RI / extended / SR LSAs) attach here.
- `internal/plugins/ospfv3/packet` (RFC 5340 codec) -- ext-6 adds the v3 RI + Extended-LSA (RFC 8362) bodies; ext-1 keeps the Instance-ID field.
- `internal/plugins/ospfv3/spf` (route table, reachability) -- read by ext-6 (SR); per-AF topologies added by ext-1.
- `internal/plugins/ospfv3/transport` + `iface` + `neighbor` (IPv6 transport, ISM/NSM) -- ext-2 (IPsec policy), ext-3 (virtual links), ext-5 (BFD client), ext-7 (NBMA/P2MP network types).
- `internal/plugins/ospfv3/auth` (RFC 7166 trailer) -- ext-2 sits alongside it as a distinct auth path, not an extension of it.
- `internal/plugins/ospfv3/register.go` + the reserved Instance-ID field -- ext-1 consumes the Instance-ID plumbing; each child registers its own commands / schema / doctor.
- The delivered IPv6 Loc-RIB / sysrib FIB-install seam -- ext-6 SR forwarding state.
- Ze's existing BFD engine (ext-5) and kernel IPsec SA/SP infrastructure (ext-2).

### Architectural Verification
- [ ] No bypassed layers (extensions attach at delivered seams; SR installs through the existing IPv6 Loc-RIB path, not a new FIB path)
- [ ] No unintended coupling (base-only features do not depend on each other; no v3 extension depends on the OSPFv2 plugin or any v2 wire code)
- [ ] No duplicated functionality (extensions reuse the delivered LSDB, flooding, SPF, ISM/NSM, transport, and FIB-install seams; no shared v2/v3 packet or LSA package)
- [ ] Zero-copy preserved (LSA bodies retained as views; buffer-first encode -- enforced per child)

## Wiring Test (MANDATORY -- NOT deferrable)

(Umbrella-level: proves the child set is reachable as a coordinated whole. Each child carries its own detailed, executable Wiring Test.)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Two address families configured on one OSPFv3 instance | -> | ext-1 maps each AF to an Instance-ID range and installs per-AF routes | `test/ospfv3/ospfv3-multiaf.ci` (ext-1) |
| IPsec AH/ESP configured on an OSPFv3 interface | -> | ext-2 installs kernel IPsec SA/SP and the adjacency reaches Full under IPsec protection | `test/ospfv3/ospfv3-ipsec.ci` (ext-2) |
| A transit area with a virtual link configured | -> | ext-3 forms a virtual-link adjacency and repairs the backbone | `test/ospfv3/ospfv3-vlink.ci` (ext-3) |
| A neighbour restarts with GR helper enabled | -> | ext-4 Grace-LSA keeps forwarding while the restarter recovers | `test/ospfv3/ospfv3-gr.ci` (ext-4) |
| A link fails with BFD enabled | -> | ext-5 drives NSM down sub-second on a BFD session failure | `test/ospfv3/ospfv3-bfd.ci` (ext-5) |
| SR configured with SRGB + Prefix-SID | -> | ext-6 adds the v3 RI + extended LSAs, computes SR labels, and programs an SR path | `test/ospfv3/ospfv3-sr-install.ci` (ext-6) |
| An NBMA / point-to-multipoint interface configured | -> | ext-7 forms adjacencies and originates host routes for the network type | `test/ospfv3/ospfv3-p2mp.ci` (ext-7) |
| Operator inspects / injects an OSPFv3 extension LSA | -> | ext-8 decodes the RI / extended / SR / Grace LSA and lists it | `test/ospfv3/ospfv3-debug.ci` (ext-8) |

## Acceptance Criteria

(Umbrella-level coordination criteria; each child carries its own detailed, testable ACs.)

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The child set is written | Every child ext-1..ext-8 exists, cross-references its dependencies, and is consistent with the OSPFv3 base umbrella's package layout and scope-aware LSA model |
| AC-2 | The build order is followed | ext-1 attaches to the reserved Instance-ID plumbing; ext-6 (SR) adds the v3 RI + extended LSAs before computing labels; ext-8 can decode the LSAs ext-4 / ext-6 add |
| AC-3 | A base-only feature is implemented | ext-2/3/4/5/7 build on the delivered base without depending on any other ext-N child or on the OSPFv2 plugin |
| AC-4 | RFC 5838 / RFC 4552 are considered | They are scheduled as ext-1 / ext-2 (in scope), NOT left in the rested table |
| AC-5 | A rested item is encountered | It is found in the "Out of scope (rested)" table with a rationale (SNMP OSPFV3-MIB, shared v2/v3 wire package, v3 equivalents of the v2-rested niche items); reviving it requires a fresh decision and a new spec, not a quiet add to a child |
| AC-6 | Any child is implemented | Its code lives under `internal/plugins/ospfv3/` and shares no packet / LSA / SPF code with the OSPFv2 ext set (RFC 5340 mandate) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs OSPFv3 with two address families | ext-1 Instance-ID mapping -> per-AF topology -> per-AF Loc-RIB install | `test/ospfv3/ospfv3-multiaf.ci` (ext-1) |
| 2 | Protects OSPFv3 with IPsec AH/ESP | ext-2 kernel IPsec SA/SP -> protected transport -> Full adjacency | `test/ospfv3/ospfv3-ipsec.ci` (ext-2) |
| 3 | Enables OSPFv3 Segment Routing | ext-6 v3 RI + extended LSAs -> SR label computation -> IPv6 Loc-RIB SR path -> kernel | `test/ospfv3/ospfv3-sr-install.ci` (ext-6) |
| 4 | Enables BFD for OSPFv3 | ext-5 BFD client registration -> sub-second NSM down on failure | `test/ospfv3/ospfv3-bfd.ci` (ext-5) |
| 5 | Repairs a partitioned backbone | ext-3 virtual-link adjacency across a transit area | `test/ospfv3/ospfv3-vlink.ci` (ext-3) |

## 🧪 TDD Test Plan

### Unit Tests
(Per child; the umbrella aggregates, it does not own unit tests.)

| Test | File | Validates | Status |
|------|------|-----------|--------|
| (per child) | `internal/plugins/ospfv3/...` (per ext-N) | see each child spec ext-1..ext-8 | |

### Boundary Tests (MANDATORY for numeric inputs)
(Per child; the umbrella introduces no new numeric wire fields of its own.)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Instance ID (AF mapping) | 0-255 | 255 | N/A | N/A (1 byte) -- AF ranges owned by ext-1 (RFC 5838 §2.4) |
| (other numeric fields) | per child | -- | -- | per ext-N boundary tables (SR label ranges, BFD timers, virtual-link cost) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| (per child) | `test/ospfv3/ospfv3-<ext>-*.ci` | per ext-N functional scenarios | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| (per child) | `test/interop/scenarios/ospfv3-<ext>-frr/` | FRR `ospf6d` | per ext-N wire-behaviour interop (multi-AF / IPsec / vlink / GR / BFD / SR / P2MP as applicable) | |

### Future (if deferring any tests)
- All extension tests are owned by their children; the umbrella defers none of its own.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files -->
- `internal/plugins/ospfv3/` -- the delivered OSPFv3 edge plugin each child extends (lsdb / packet / spf / transport / iface / neighbor / auth / register.go); the umbrella names no single file edit of its own
- `plan/spec-ospfv3-ext-1-*.md` .. `plan/spec-ospfv3-ext-8-*.md` -- the child specs this umbrella coordinates (authoring deliverable)
- `docs/comparison.md`, `docs/features.md` -- OSPFv3-extension parity rows (per child, as each lands)
- NOTE: the umbrella itself modifies no feature code; each child lists its own `internal/plugins/ospfv3/...` edits

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | Per child | each ext-N adds its own leaves to `internal/plugins/ospfv3/yang/ze-ospfv3-conf.yang` |
| CLI commands/flags | Per child | each ext-N adds `show ipv6 ospf <noun>` subcommands in `ze-ospfv3-cmd.yang` |
| Doctor check for runtime dependencies | Per child | ext-2 (kernel IPsec), ext-5 (BFD) and any new runtime dependency get their own check |
| Prometheus counters/metrics | Per child | each ext-N owns its `ze_ospfv3_<ext>_*` series; the umbrella metrics mapping is updated as children land |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Per child | `docs/features.md` (per ext-N) |
| 5 | Plugin added/changed? | Per child | `docs/guide/plugins.md`, `docs/plugin-overview.md` (per ext-N) |
| 9 | RFC behavior implemented? | Per child | `rfc/short/rfcNNNN.md` (created by each ext-N via `/ze-rfc`) |
| 11 | Affects daemon comparison? | Per child | `docs/comparison.md` (OSPFv3-extension parity rows) |
| 12 | Internal architecture changed? | Per child | the OSPFv3 subsystem doc (per ext-N) |
| -- | Umbrella-level | This file | keep the Child Decomposition, Dependency / Build Order, and RFC Coverage tables current as children land or rest |

## Files to Create
- `plan/spec-ospfv3-ext-1-*.md` .. `plan/spec-ospfv3-ext-8-*.md` -- the child specs (this batch)
- (no feature files at the umbrella level) -- each child creates its own `internal/plugins/ospfv3/...` files and `test/ospfv3/*.ci` / `test/interop/scenarios/ospfv3-*-frr/`

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

1. **Phase: Base-only extensions (parallel)** -- ext-1 (multi-AF, on the reserved Instance-ID), ext-2 (IPsec), ext-3 (virtual links), ext-4 (GR), ext-5 (BFD), ext-7 (NBMA/P2MP).
2. **Phase: Segment Routing** -- ext-6, adding the v3 Router Information + extended LSAs (RFC 8362) as its first internal phase, then the SR control plane and FIB programming.
3. **Phase: Debug & introspection** -- ext-8, last so it can decode the LSAs ext-4 (Grace) and ext-6 (RI / extended / SR) add; the core decode surface is base-only and can start any time.
4. **Per-child verification + interop** -- `make ze-verify` + FRR `ospf6d` scenarios, owned by each child.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this umbrella |
|-------|----------------------------------|
| Completeness | Every child ext-1..ext-8 exists, cross-references its dependencies, and matches the OSPFv3 base umbrella's package layout and scope-aware LSA model |
| Correctness | The dependency / build order is honoured (ext-1 on the reserved Instance-ID; ext-6 adds the v3 RI + extended LSAs before SR labels; ext-8 decodes the ext-4 / ext-6 LSAs) |
| Naming | Each extension uses `ze_ospfv3_<ext>_*` metrics and `show ipv6 ospf <noun>` subcommands; no existing series/command renamed |
| Data flow | Extensions attach at delivered seams; SR installs through the existing IPv6 Loc-RIB path; no shared v2/v3 wire code |
| Rule: plugin-self-containment | Each child's schema/help/doctor/commands live under `internal/plugins/ospfv3/`; no v3 extension spelling in the OSPFv2 plugin or any generic/central package |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Umbrella + 8 child specs | `ls plan/spec-ospfv3-ext-*.md` |
| Each child Status:ready | `grep -m1 '| Status |' plan/spec-ospfv3-ext-1-*.md` (and siblings) |
| Each child cross-references its dependency | grep each child for its "Depends" row |
| Rested set recorded | `grep -A20 'Out of scope (rested' plan/spec-ospfv3-ext-0-umbrella.md` |
| No shared v2/v3 wire package | each child's files are under `internal/plugins/ospfv3/` only |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Each extension's wire decode is bound-checked (multi-AF Instance-ID ranges, RI / extended / SR sub-TLVs, Grace-LSA body); per child |
| Trust boundary | ext-2 IPsec keys/SPIs handled by the kernel SA/SP path with no secret leakage to logs; received extension LSAs rely on the delivered RFC 7166 trailer or RFC 4552 IPsec; no new unauthenticated surface |
| Resource exhaustion | Extension LSAs share the delivered LSDB caps; a flood of extension LSAs cannot grow memory unbounded; per child |
| Consumer isolation | A debug/inject path (ext-8) cannot corrupt the live LSDB; injected LSAs follow the normal validation path |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the child that introduced it |
| Test fails wrong reason | Fix test setup |
| Test fails behavior mismatch | Re-read the child's RFC summary / Current Behavior |
| Build-order violation (SR labels before the v3 RI + extended LSAs) | STOP; reorder; ext-6 must add its advertisement layer first |
| Shared v2/v3 wire code introduced | STOP; RFC 5340 forbids it; keep the code under `internal/plugins/ospfv3/` |
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
- OSPFv3 has no opaque carrier: RFC 5340 carries extension state as native, scope-aware LSAs, so the v3 extension set has a flatter dependency graph than the OSPFv2 set (no single opaque root). The only intra-extension ordering is SR's self-contained need for the v3 Router Information + extended LSAs.
- The base umbrella's reserved Instance-ID field is the deliberate hook for multi-AF (ext-1); recording this up front prevents a future agent re-opening the RFC 5340 codec to add address-family support.
- "Rested" is a distinct status from "deferred child": deferred children are scheduled work, rested items are deliberately-absent decisions that need re-opening. RFC 5838 and RFC 4552 are NOT rested here -- they are promoted into scope as ext-1 and ext-2 -- which is exactly the kind of decision the rested table exists to make explicit.

## Core Insight
The OSPFv3 extension landscape is a flat set of follow-ups on a stable RFC 5340
base, with no opaque carrier root (extensions are native LSAs) and exactly one
self-contained ordering constraint (SR adds the v3 Router Information + extended
LSAs before it can compute labels). The umbrella's job is to make that ordering
explicit, to keep all v3 code version-separate from the OSPFv2 ext set (RFC 5340
mandate), and to record -- with rationale -- the management surfaces and niche
features deliberately NOT scheduled, while promoting RFC 5838 multi-AF and
RFC 4552 IPsec from the base's out-of-scope list into this umbrella's in-scope
child set.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| No shared OSPFv2/OSPFv3 wire code | A unified packet / LSA package across v2 and v3 | RFC 5340 vs RFC 2328 have different headers, LSA registries, scopes, prefix encodings, and checksums; sharing leaks version-specific branches into both. FRR keeps `ospfd`/`ospf6d` separate; Ze follows |
| Multi-AF (ext-1) on the reserved Instance-ID | Re-open the RFC 5340 codec to add AF awareness | The base reserved a validated Instance-ID field precisely for this; ext-1 maps AF to Instance-ID ranges (RFC 5838 §2.4) without a codec change |
| IPsec (ext-2) is a separate auth path | Extend the delivered RFC 7166 trailer code | RFC 4552 IPsec AH/ESP is a kernel SA/SP mechanism, structurally distinct from the in-packet trailer; folding it into the trailer would conflate two auth models |
| SR (ext-6) adds the v3 RI + extended LSAs itself | Make the v3 RI / extended LSAs separate ext-N children | OSPFv3 has no opaque carrier; the RI (RFC 7770-equivalent v3 form) and Extended LSAs (RFC 8362) exist only to carry SR state here, so they are scoped inside ext-6 rather than as standalone children |
| RFC 5838 + RFC 4552 promoted in scope | Leave them rested as the base umbrella did | They are concrete, interop-valuable features with FRR partners; they become ext-1 and ext-2 rather than rested items |
| SNMP OSPFV3-MIB rested | Schedule an OSPFV3-MIB child | Wrong management plane; Ze uses YANG/gNMI/CLI/web, has no SNMP agent, and already exposes state via `show ipv6 ospf` + metrics |

## Known Limitations
- This umbrella tracks OSPFv3 extensions ONLY. OSPFv2 (RFC 2328) and its extension follow-ups are a separate edge plugin under `plan/spec-ospf-0-umbrella.md` + `plan/spec-ospf-ext-0-umbrella.md`; nothing here shares wire code with v2.
- The umbrella is a coordination document: it has no feature code, no tests, and no acceptance criteria that it implements itself. Completion is defined by its children, not by this file. It is never marked "done" while a tracked child is open.
- OSPFv3 has no opaque-LSA carrier (RFC 5340 carries extensions as native LSAs); the OSPFv2 ext-1 opaque framework has no v3 analogue, and ext-6 (SR) must add the v3 Router Information + Extended LSAs (RFC 8362) itself before it can compute SR labels.
- The "rested" items (SNMP OSPFV3-MIB, any shared v2/v3 packet or LSA package, the v3 equivalents of the v2-rested niche items) are deliberately absent from the child set. Reviving any of them requires a fresh design decision and a new spec, not a quiet add to an existing child. RFC 5838 multi-AF and RFC 4552 IPsec are explicitly NOT rested -- they are ext-1 and ext-2.
- Sharing any OSPFv2/OSPFv3 packet, LSA, or SPF package is forbidden (RFC 5340 mandate); a child that introduces such sharing must be rejected at planning time.

## RFC Documentation

Per-RFC implementation summaries (the `/ze-rfc` deep output) and the short
house-format summaries under `rfc/short/` are produced by each CHILD spec at its
own implementation time, for the RFCs whose normative detail that child's code
enforces. This umbrella adds no RFC enforcement code and therefore carries no
`// RFC NNNN Section X.Y` annotations itself; it only records the RFC-to-child
mapping in "RFC Coverage" below. `rfc/short/rfc5838.md` and `rfc/short/rfc4552.md`
are the pre-existing summaries noted by the base OSPFv3 umbrella as deferred
references, consumed by ext-1 and ext-2.

### RFC Coverage (per child)
| Child | RFC(s) | Summary status |
|-------|--------|----------------|
| ext-1 | RFC 5838 | noted by the base umbrella; refreshed by ext-1 via `/ze-rfc` |
| ext-2 | RFC 4552 | noted by the base umbrella; refreshed by ext-2 via `/ze-rfc` |
| ext-3 | RFC 5340 §4.2 (virtual links) | covered by the base `rfc/short/rfc5340.md` (delivered); ext-3-specific detail per child |
| ext-4 | RFC 5187 | created by ext-4 |
| ext-5 | RFC 5881 (IPv6 single-hop) | created by ext-5 |
| ext-6 | RFC 8666 + RFC 8362 (v3 extended LSAs) + the v3 Router Information form | created by ext-6 |
| ext-7 | RFC 5340 (network types) | covered by the base `rfc/short/rfc5340.md` (delivered); ext-7-specific detail per child |
| ext-8 | (tooling; no new RFC) | n/a |

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
| Enumerate the OSPFv3 extension follow-ups as a coordinated child set | Done | Child Decomposition table | ext-1..ext-8 |
| Fix the dependency / build order | Done | Dependency / Build Order | base-only set + ext-1 on reserved Instance-ID + ext-6 self-contained SR advertisement layer |
| Record the rested set with rationale | Done | Out of scope (rested) table | SNMP OSPFV3-MIB, shared v2/v3 wire package, v3 equivalents of the v2-rested niche items |
| Keep OSPFv3 separate from OSPFv2 (no shared wire code) | Done | Scope boundary + Target scope/decisions + Known Limitations | RFC 5340 mandate recorded; v2 ext umbrella is a pattern reference only |
| Per-child implementation | (pending) | each `plan/spec-ospfv3-ext-N-*.md` | downstream |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1..AC-6 | (coordination) | this umbrella's tables | child ACs are detailed per child |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| (per child) | (pending) | `internal/plugins/ospfv3/...`, `test/ospfv3/...` | per ext-N |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `plan/spec-ospfv3-ext-0-umbrella.md` | Written | this file (Status:design) |
| `plan/spec-ospfv3-ext-1-*.md` .. `plan/spec-ospfv3-ext-8-*.md` | (this batch) | Status:ready |
| `internal/plugins/ospfv3/` (extensions) | (pending) | per child |

### Audit Summary
- **Total items:** umbrella coordination (this deliverable) + downstream per-child implementation
- **Done:** child decomposition, dependency / build order, rested set, v2/v3 separation decision
- **Partial:** 0
- **Skipped:** 0
- **Changed:** per-child implementation is downstream, tracked per child

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| OSPFv3 extension child set exists and is internally consistent | spec files + cross-references | `ls plan/spec-ospfv3-ext-*.md`; dependency / build-order table; each child's "Depends" row |
| The build order is captured | this file | Dependency / Build Order section (base-only set + ext-1 Instance-ID + ext-6 self-contained SR layer) |
| The rested set is recorded with rationale | this file | Out of scope (rested) table |
| RFC 5838 + RFC 4552 promoted in scope, not rested | this file | ext-1 / ext-2 rows in Child Decomposition; the NOTE above the rested table |
| Per-child implementation | unit + functional + interop | (filled during implementation per child) |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | pending | `/ze-review` on the umbrella + child set has not run after authoring | `plan/spec-ospfv3-ext-*.md` | run before implementation begins |

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
| `plan/spec-ospfv3-ext-0-umbrella.md` | (verify) | `ls plan/spec-ospfv3-ext-0-umbrella.md` |
| `plan/spec-ospfv3-ext-1-*.md` .. `plan/spec-ospfv3-ext-8-*.md` | (verify) | `ls plan/spec-ospfv3-ext-*.md` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | each child exists + cross-references | `ls plan/spec-ospfv3-ext-*.md`; grep each child's Depends row |
| AC-2 | build order honoured | Dependency / Build Order table |
| AC-6 | code under `internal/plugins/ospfv3/`, no shared v2/v3 wire | per child Files to Create/Modify |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| (downstream) | `test/ospfv3/*.ci` (per ext-N) | filled during implementation |

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
- [ ] All 8 child specs written and cross-referenced (ext-1..ext-8 this batch)
- [ ] Dependency / build order captured and consistent
- [ ] Out-of-scope (rested) set recorded with rationale
- [ ] RFC 5838 + RFC 4552 promoted in scope (ext-1 / ext-2), not rested
- [ ] AC-1..AC-6 demonstrated by the umbrella's tables
- [ ] End-to-End User Stories each map to a child + a downstream test
- [ ] Wiring Test table complete (umbrella-level; detailed wiring per child)
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes (downstream, per child)
- [ ] Feature code integrated (`internal/plugins/ospfv3/`) (downstream, per child)
- [ ] Documentation Update Checklist answered (per child as each lands)
- [ ] Critical Review passes

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added (downstream, per child)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (no extension built before its prerequisite)
- [ ] No speculative features (rested table honoured)
- [ ] Single responsibility per child
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (base-only features independent of each other and of OSPFv2)

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
- [ ] Write learned summary to `plan/learned/NNN-ospfv3-ext-0-umbrella.md` (at set completion)
- [ ] Summary included in commit
