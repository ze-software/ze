# Spec: ospf-ext-0 -- OSPFv2 Extensions (Umbrella)

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-ospf-0-umbrella.md (delivered) |
| Phase | - |
| Updated | 2026-06-24 |

> Umbrellas are living tracking documents, not implementable specs. This file
> coordinates the OSPF extension follow-ups deferred from the delivered OSPFv2
> umbrella (`plan/spec-ospf-0-umbrella.md`); it carries NO acceptance criteria
> and NO feature code of its own. Each child spec listed below is the
> implementable unit. The "implementable-spec" sections below (Current Behavior,
> Data Flow, Wiring Test, TDD Test Plan, Checklist) are framed at umbrella level
> -- they describe the child set, mirroring the delivered base umbrella, not a
> feature this file implements. Do not mark this umbrella "done": it closes only
> when every child it tracks is closed (or explicitly re-rested).

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/spec-ospf-0-umbrella.md` -- the DELIVERED OSPFv2 base umbrella; its "Shared Contracts", "Out of scope (future...)" table (the source list of every follow-up this umbrella tracks), the LSA inventory, the metrics contract, and the FIB-install-vs-redistribution split. The base is the stable foundation; every extension builds on it without re-opening it
4. `plan/spec-ospf-ext-1-opaque-framework.md` -- the first WRITTEN child (RFC 5250 opaque carrier); the ext-family voice and the registration API that ext-2/ext-3/ext-4/ext-9/ext-14 plug into
5. `docs/research/ospf-implementation-guide.md` §14 (FRR feature catalogue) -- the extension landscape (opaque, TE, RI, SR, TI-LFA, GR, BFD, LDP-IGP sync, multi-instance, L3VPN DN-bit) and the rested/deferred rationale this umbrella records
6. `internal/plugins/ospf/` -- the delivered plugin the extensions extend: `lsdb/` (stores, flooding, origination), `packet/` (codec + opaque passthrough), `spf/` (Dijkstra + route table), `neighbor/` (NSM, DD), `register.go` (registration + lifecycle)

## Task

Coordinate the OSPFv2 extension follow-ups that the delivered base umbrella
(`plan/spec-ospf-0-umbrella.md`) deliberately rested under its "Out of scope
(future, noted here so it is not silently assumed done)" table. The base
delivered a complete, interoperable OSPFv2: multi-area / ABR / stub / NSSA,
broadcast + point-to-point, the full §13 flooding procedure and §16 SPF,
redistribution, and authentication (AuType 0/1/2/3). It did NOT deliver the
opaque-LSA carrier or any extension that rides on it (TE, Router Information,
Extended Link/Prefix, Segment Routing), nor the protocol features that are
independent of opaque (virtual links, NBMA / point-to-multipoint, Graceful
Restart, BFD, LDP-IGP sync, Multi-Instance), nor the L3VPN PE-CE DN bit, nor a
dedicated debug / introspection surface for all of the above.

This umbrella enumerates those follow-ups as a coordinated child set, fixes the
build order between them (the opaque carrier is a hard prerequisite for several;
SR sits on RI + Extended Link/Prefix; TI-LFA sits on SR + SPF), and records --
in the "Out of scope (rested)" table -- the OSPF features that were considered
and deliberately NOT scheduled, each with its rationale, so a future agent does
not silently assume they are done or quietly re-add them.

### Scope boundary vs the delivered base

| Concern | Owned by | This umbrella |
|---------|----------|---------------|
| OSPFv2 base protocol (areas, ABR/ASBR, stub/NSSA, §13 flooding, §16 SPF, broadcast + P2P, redistribution, auth 0/1/2/3) | `plan/spec-ospf-0-umbrella.md` (delivered) | Treated as a STABLE foundation; not re-opened. Extensions attach at documented seams (LSDB stores, flooding chokepoints, SPF route table, NSM/DD, registry) |
| Opaque-LSA carrier + the extensions that ride it (TE, RI, Extended Link/Prefix, SR) | this umbrella (ext-1..ext-5) | Tracked + ordered here |
| Protocol features independent of opaque (virtual links, NBMA/P2MP, GR, BFD, LDP-IGP sync, Multi-Instance) | this umbrella (ext-6..ext-12, partial) | Tracked + ordered here |
| L3VPN PE-CE DN bit | this umbrella (ext-13) | Tracked here; GATED on future MPLS-L3VPN/VRF infrastructure (blocking dependency, recorded below) |
| Extension debug / introspection tooling (folds in the old standalone `ospfclient` debug/inject idea) | this umbrella (ext-14) | Tracked here; the standalone Unix-socket daemon is rested in favour of ext-14 |
| OSPFv3 (RFC 5340, IPv6) | `plan/spec-ospfv3-0-umbrella.md` | NOT this umbrella; OSPFv3 is a separate edge plugin with its own extension follow-ups |

### Target scope / decisions

| Lever | Decision | Effect on the child set |
|-------|----------|-------------------------|
| Opaque carrier first | **ext-1 (RFC 5250) is the foundation** | TE (ext-2), Router Information (ext-3), Extended Link/Prefix (ext-4), Grace-LSA / GR (ext-9), and the debug surface (ext-14) all depend on the opaque carrier. It is written first (Status:ready) and unblocks the opaque chain |
| SR builds on the IGP advertisement layer | **ext-5 depends on ext-3 + ext-4** | Segment Routing (RFC 8665) needs the Router Information LSA (SR-Algorithm, SRGB, SR-Local-Block sub-TLVs) AND the Extended Prefix/Link LSAs (Prefix-SID / Adjacency-SID). It cannot start before both land |
| TI-LFA needs SR + SPF reachable | **ext-6 depends on ext-5 + SPF** | TI-LFA / LFA (RFC 5286 + the TI-LFA draft) computes repair paths over the SR label stack and the delivered SPF; it is last in the opaque-derived chain |
| Independent protocol features parallelise | **ext-7, ext-8, ext-10, ext-11, ext-12 depend only on the delivered base** | Virtual links, NBMA/P2MP, BFD, LDP-IGP sync, and Multi-Instance touch the base protocol (areas, ISM/NSM, interface model) but NOT the opaque chain; they can proceed in parallel with ext-2..ext-4 |
| L3VPN DN bit is VRF-gated | **ext-13 last, blocked on VRF infra** | The PE-CE DN-bit loop prevention (RFC 4576/4577) requires MPLS-L3VPN / VRF infrastructure Ze does not yet have; ext-13 is recorded as BLOCKED until that lands, not merely "later" |
| Debug folds in ospfclient | **ext-14 replaces the standalone ospfclient daemon** | The genuinely useful capability of FRR's `ospfclient` (inject/observe opaque LSAs for testing) is delivered as introspection tooling inside ext-14 on the ext-1 registry; no separate Unix-socket external-injection daemon ships |
| OSPFv3 excluded | **out of this umbrella entirely** | OSPFv3 (RFC 5340) and its own extensions live under `plan/spec-ospfv3-0-umbrella.md`; this umbrella is OSPFv2-only |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
- [ ] `plan/spec-ospf-0-umbrella.md` -- the DELIVERED base umbrella this set extends
  -> Decision: extensions attach at the documented seams (LSDB stores, flooding chokepoints, SPF route table, NSM/DD, the registration model); the base is not re-opened
  -> Constraint: the base's "Out of scope (future...)" table is the authoritative source list of every follow-up this umbrella tracks -- do not add a child that is not derived from it (or explicitly record a new resting decision)
- [ ] `plan/spec-ospf-ext-1-opaque-framework.md` -- the written foundation child (RFC 5250)
  -> Decision: ext-2/ext-3/ext-4/ext-9/ext-14 plug into the ext-1 consumer registry (`RegisterOpaqueConsumer`); the carrier interprets no body, each consumer owns its TLVs
  -> Constraint: opaque LSAs never enter SPF; consumers that affect forwarding (SR, TI-LFA) read the route table, they do not make opaque LSAs vertices
- [ ] `docs/research/ospf-implementation-guide.md` §14 -- the FRR extension feature catalogue
  -> Constraint: the guide's rested/deferred rationale (TOS, SNMP MIB, multi-area adjacencies, QoS routing, Flood Reduction, ospfclient) is the basis for this umbrella's "Out of scope (rested)" table
- [ ] The ext-2..ext-14 child specs (this batch) -- each child's own Required Reading governs its implementation; the umbrella points at them but does not duplicate their detail

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc5250.md` -- the opaque framework (consumed by ext-1); the only pre-existing OSPF-extension summary (created with the base umbrella as the noted out-of-scope framework reference)
  -> Constraint: each remaining child creates its own RFC summary via `/ze-rfc` before that child is implemented; the umbrella tracks the mapping (see RFC Coverage) but enforces no RFC text itself

**Key insights:** (minimal context to resume after compaction)
- ext-1 is the single foundation for the opaque chain (ext-2, ext-3, ext-4, ext-9, ext-14); ext-5 needs ext-3 + ext-4; ext-6 needs ext-5 + SPF.
- ext-7, ext-8, ext-10, ext-11, ext-12 are base-only and parallelise; ext-13 is VRF-gated and last.
- The "rested" items (TOS, SNMP MIB, multi-area adjacencies, QoS routing, Flood Reduction/DoNotAge, standalone ospfclient) are deliberately absent and need a fresh decision to revive.

## Child Decomposition

Each child is an independent, implementable spec. ext-1 is already written
(Status:ready); ext-2..ext-14 are written as Status:ready specs in this same
batch. The umbrella owns the coordination, not the implementation.

| Child | Title | RFC(s) | Depends on | One-line scope |
|-------|-------|--------|------------|----------------|
| ext-1 | Opaque-LSA framework | RFC 5250 | base | The generic opaque carrier: scope-correct flooding (Type 9/10/11), the LS-ID Opaque Type/ID split, O-bit DD negotiation, generic TLV helpers, and the consumer registry that ext-2/3/4/9/14 plug into. No consumer semantics. **[WRITTEN]** |
| ext-2 | Traffic Engineering LSA | RFC 3630, RFC 5392 | ext-1 | Type 10 TE Opaque LSA: Router Address + Link TLV with sub-TLVs (link type, IDs, metrics, bandwidths, admin groups); RFC 5392 inter-AS TE links. Advertisement + TED build, no CSPF |
| ext-3 | Router Information LSA | RFC 7770 | ext-1 | The RI Opaque LSA carrying router-wide capabilities (Informational Capabilities TLV) plus the SR-Algorithm / SRGB / SR-Local-Block TLVs that SR consumes |
| ext-4 | Extended Link / Extended Prefix LSA | RFC 7684 | ext-1 | The Extended Prefix and Extended Link Opaque LSAs and their sub-TLV containers (the carriers for Prefix-SID / Adjacency-SID and prefix attributes) |
| ext-5 | Segment Routing | RFC 8665 | ext-3, ext-4 | OSPF SR control plane: SRGB/SRLB from RI (ext-3), Prefix-SID / Adjacency-SID from Extended Prefix/Link (ext-4), MPLS label computation and FIB programming for SR paths |
| ext-6 | TI-LFA / LFA fast reroute | RFC 5286, TI-LFA draft (draft-ietf-rtgwg-segment-routing-ti-lfa) | ext-5, SPF | Loop-free alternate + topology-independent LFA repair paths computed over SPF and the SR label stack; pre-computed backup nexthops in the FIB |
| ext-7 | Virtual links | RFC 2328 §15 | base | Virtual-link adjacencies across a transit area to repair a partitioned or non-contiguous backbone; virtual-interface ISM/NSM and Type 1 virtual-link records |
| ext-8 | NBMA + point-to-multipoint | RFC 2328 (network types) | base | The NBMA and point-to-multipoint network types: static neighbour config, NBMA DR/BDR eligibility, P2MP per-neighbour Hellos and host-route origination |
| ext-9 | Graceful Restart | RFC 3623 | ext-1 | Grace-LSA (a Type 9 link-scope opaque LSA) origination + the GR helper (and restarter) so a neighbour's restart does not tear down forwarding |
| ext-10 | BFD for OSPF | RFC 5880, RFC 5881 | base | Integrate Ze's existing BFD engine: register OSPF adjacencies as BFD clients, drive NSM down on a BFD session failure for sub-second failure detection |
| ext-11 | LDP-IGP synchronisation | RFC 5443, RFC 6138 | base | Hold an OSPF link at max-metric until LDP signalling is up (plus the RFC 6138 unnumbered/LFA refinement) so traffic does not use a link whose LSP is not ready |
| ext-12 | Multi-Instance OSPF | RFC 6549 | base | The Instance ID field in the OSPF common header so multiple OSPF instances share an interface; per-instance packet demultiplexing |
| ext-13 | L3VPN PE-CE DN bit | RFC 4576, RFC 4577 | base, **future VRF/MPLS-L3VPN infra (BLOCKING)** | The Down (DN) bit + VPN Route Tag loop prevention for OSPF as a PE-CE protocol; requires per-VRF OSPF instances that Ze's routing infrastructure does not yet support |
| ext-14 | Debug & introspection tooling | (no new RFC; tooling over the RFC 5250 carrier) | ext-1 | Extension-wide debug/introspection: decode + inspect opaque/TE/RI/SR/Extended LSAs, inject test opaque LSAs via the ext-1 registry, and the show/diagnostic surface for every extension above. Folds in the useful `ospfclient` inject/observe capability |

## Dependency / Build Order

The children split into one dependency chain rooted at the opaque carrier and a
set of base-only features that parallelise alongside it.

| Child | Depends on |
|-------|-----------|
| ext-1 (opaque carrier) | base only |
| ext-2 (TE) | ext-1 |
| ext-3 (Router Information) | ext-1 |
| ext-4 (Extended Link/Prefix) | ext-1 |
| ext-5 (Segment Routing) | ext-3, ext-4 |
| ext-6 (TI-LFA / LFA) | ext-5, SPF (delivered base) |
| ext-7 (Virtual links) | base only |
| ext-8 (NBMA + P2MP) | base only |
| ext-9 (Graceful Restart) | ext-1 (Grace-LSA is a Type 9 opaque LSA) |
| ext-10 (BFD) | base only |
| ext-11 (LDP-IGP sync) | base only |
| ext-12 (Multi-Instance) | base only |
| ext-13 (L3VPN DN bit) | base + future VRF/MPLS-L3VPN infra (BLOCKING) |
| ext-14 (Debug & introspection) | ext-1 (decodes/injects opaque + the extension bodies) |

**Opaque chain:** ext-1 is the single foundation for ext-2, ext-3, ext-4,
ext-9, and ext-14. ext-5 (SR) cannot start until ext-3 AND ext-4 land. ext-6
(TI-LFA) cannot start until ext-5 lands and reuses the delivered SPF.

**Base-only (parallel) set:** ext-7, ext-8, ext-10, ext-11, ext-12 depend only
on the delivered base umbrella, not on the opaque chain; they can proceed
concurrently with ext-2..ext-4.

**Gated:** ext-13 additionally depends on MPLS-L3VPN / VRF infrastructure that
does not yet exist in Ze; it is BLOCKED until that lands and must not be
scheduled before it.

**Recommended build order:**

1. **ext-1** (opaque carrier) -- unblocks the opaque chain.
2. In parallel after ext-1: **ext-2, ext-3, ext-4** (opaque consumers) and,
   independently of ext-1, **ext-9, ext-10, ext-11, ext-12, ext-7, ext-8,
   ext-14** (ext-9 and ext-14 still wait on ext-1; the rest are base-only).
3. **ext-5** (Segment Routing) -- once ext-3 and ext-4 are both done.
4. **ext-6** (TI-LFA) -- once ext-5 is done.
5. **ext-13** (L3VPN DN bit) -- LAST, gated on VRF/MPLS-L3VPN infrastructure.

Condensed: `ext-1 -> {ext-2, ext-3, ext-4, ext-9, ext-10, ext-11, ext-12,
ext-7, ext-8, ext-14} -> ext-5 -> ext-6`; `ext-13` last (gated on VRF).

## Out of scope (rested, noted here so it is not silently assumed done)

These OSPF features were considered for this extension set and deliberately NOT
scheduled. Each is recorded with its rationale so a future agent does not assume
it is implemented, nor quietly re-add it without revisiting the reasoning. A
"rested" item differs from a "deferred" child: rested items are not part of any
ext-N child and need a fresh decision (and likely a new spec) to revive.

| Rested item | RFC(s) | Rationale for resting |
|-------------|--------|-----------------------|
| TOS (Type-of-Service) routing | RFC 2328 §16.9 (and earlier RFC 1583) | Deprecated by later RFCs; no production OSPF implementation advertises or computes per-TOS metrics. The #TOS field stays 0 in originated LSAs (already the base behaviour). Reviving it has no interop value |
| SNMP OSPF-MIB | RFC 4750 | Ze's management plane is YANG / gNMI / CLI / web, not SNMP. There is no SNMP agent to host the MIB; OSPF state is already exposed through `show ip ospf` and Prometheus metrics. Adding SNMP would duplicate the existing surface against a transport Ze does not speak |
| Multi-area adjacencies | RFC 5185 | Niche feature (a single link in multiple areas via a point-to-point logical adjacency). FRR does not implement it; demand is minimal. Virtual links (ext-7) cover the backbone-repair use case that overlaps with it |
| QoS routing | RFC 2676 | Experimental; effectively nobody implements it. The metric model and flooding extensions it needs are speculative with no interop partner |
| OSPF Flood Reduction + demand-circuit DoNotAge | RFC 7715, RFC 1793 (DoNotAge) | A pure optimisation for very large, very stable LSDBs (suppressing periodic LSA refresh via the DoNotAge bit). It has subtle failure modes around stale-LSA retention and topology-change re-flooding. DEFERRED, not rejected -- it may be revisited if a deployment ever needs it; until then the base's normal LSRefresh/MaxAge behaviour is correct and safe |
| Standalone `ospfclient` Unix-socket daemon | (FRR tooling, no RFC) | The genuinely useful capability (inject/observe opaque LSAs for testing) is folded into **ext-14** on the ext-1 registry instead. A separate external-injection daemon with its own socket and trust boundary is unnecessary surface; ext-14 delivers the same debug value in-process |

## Current Behavior (MANDATORY)

**Source files read:** (architecture survey; each child spec reads its own targets)
<!-- Same rule: never tick [ ] to [x]. Write -> Constraint: annotations instead. -->
- [ ] `internal/plugins/ospf/lsdb/lsdb.go` -- the delivered LSDB: per-area stores, `asExternal`, `links`, store routing in `dbForLocked`; the seam opaque (ext-1) and the opaque consumers (ext-2/3/4/9) extend
  -> Constraint: extensions ADD stores/routing at this chokepoint; they do not rewrite the delivered Types 1-7 routing
- [ ] `internal/plugins/ospf/packet/lsa.go`, `internal/plugins/ospf/packet/lsa_opaque.go` -- the delivered codec already retains opaque (Type 9/10/11) bodies verbatim; `IsOpaque()` classifies them
  -> Constraint: ext-1 adds only the Opaque Type/ID split and generic TLV helpers; ext-2/3/4 add typed bodies on top, they do not re-open the verbatim passthrough
- [ ] `internal/plugins/ospf/spf/` -- the delivered two-stage Dijkstra + route table with path types and the ASBR-reachability used by Type 5
  -> Constraint: opaque LSAs never become SPF vertices; SR (ext-5) and TI-LFA (ext-6) READ the route table and add label/repair computation, they do not feed opaque bodies into the graph
- [ ] `internal/plugins/ospf/neighbor/` -- the delivered NSM + DD exchange (the seam ext-1 uses for O-bit negotiation and ext-10/ext-12 touch for BFD client registration and Instance-ID demux)
  -> Constraint: extensions extend NSM/DD behaviour additively; the delivered adjacency state machine is preserved
- [ ] `internal/plugins/ospf/register.go` -- the delivered registration + SDK lifecycle; ext-1 adds the opaque consumer registry here; later extensions register their own commands/schema/doctor checks
  -> Constraint: each extension is self-contained (`ai/rules/plugin-self-containment.md`); removing a child removes all its registration cleanly

**Behavior to preserve:** (the delivered base is a stable foundation)
- The delivered OSPFv2 wire codec, LSDB key triple, the three stores (link/per-area/AS-external), `OriginateSelf`/`OriginateLinkSelf`/`OriginateExternal`, and the §13 flooding + §16 SPF behaviour.
- All existing OSPFv2 functional and FRR interop tests: every extension is additive; a router with no extension enabled behaves exactly as the delivered base.
- The FIB-install-via-Loc-RIB path and the redistribution-via-redistevents path (the two distinct paths the base umbrella documents); extensions that affect forwarding (SR/TI-LFA) install through the SAME Loc-RIB seam.
- The canonical OSPF metric set and the command-YANG ownership model; each extension adds its own `ze_ospf_<ext>_*` series and its own show subcommands, it does not rename existing ones.

**Behavior to change:** (this umbrella changes NONE directly)
- None -- the umbrella implements nothing. Each child changes behaviour additively, documented in that child's own "Behavior to change". The umbrella only coordinates ordering and records the rested set.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

(Umbrella-level: the data paths each child plugs into. Each child carries its own detailed Data Flow.)

### Entry Point
- Children are selected and implemented in the build order above; the umbrella's "input" is the set of follow-ups from the base umbrella's out-of-scope table, plus the resting decisions captured here.
- At runtime, every extension enters through the delivered OSPF plugin's existing entry points: opaque LSAs via `lsdb.ReceiveUpdate` (the ext-1 carrier path), config via the YANG tree, BFD/LDP-sync via the existing engine integration seams.

### Transformation Path
1. **Base delivered:** OSPFv2 (areas, ABR/ASBR, stub/NSSA, §13 flooding, §16 SPF, redistribution, auth) is complete and stable (the source of every seam below).
2. **ext-1 carrier:** the opaque-LSA framework (scope flooding, O-bit DD, LS-ID split, TLV helpers, consumer registry) lands on the delivered LSDB/flooding/neighbor seams.
3. **Opaque consumers:** ext-2 (TE), ext-3 (RI), ext-4 (Extended Link/Prefix), ext-9 (Grace-LSA), ext-14 (debug) register Opaque Types on the ext-1 carrier and add typed bodies.
4. **SR control plane:** ext-5 reads RI (ext-3) + Extended Prefix/Link (ext-4) to compute SR labels and program SR paths via the delivered Loc-RIB seam.
5. **Repair paths:** ext-6 (TI-LFA/LFA) reads the delivered SPF + ext-5 label stack to pre-compute backup nexthops.
6. **Base-only protocol features:** ext-7 (virtual links), ext-8 (NBMA/P2MP), ext-10 (BFD), ext-11 (LDP-IGP sync), ext-12 (Multi-Instance) extend the delivered ISM/NSM/interface model directly, independent of the opaque chain.
7. **VRF-gated:** ext-13 (L3VPN DN bit) lands LAST, once MPLS-L3VPN/VRF infrastructure exists.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Base <-> opaque carrier | ext-1 attaches at `dbForLocked` / flooding chokepoints / DD Options; the base codec already carries opaque verbatim | [ ] |
| Carrier <-> consumer | ext-2/3/4/9/14 register Opaque Types via the ext-1 registry; value-typed payloads, no cross-boundary pointers | [ ] |
| RI/Extended <-> SR | ext-5 reads ext-3 SRGB/SRLB + ext-4 Prefix-SID/Adjacency-SID (read-only consumption of advertised state) | [ ] |
| SR/SPF <-> repair | ext-6 reads the delivered SPF route table + the ext-5 label stack to emit backup nexthops | [ ] |
| Extension <-> Loc-RIB (FIB) | SR/TI-LFA forwarding state installs through the SAME delivered `locrib.Path` seam, never a second FIB path | [ ] |
| Engine <-> BFD / LDP | ext-10 registers OSPF adjacencies as BFD clients; ext-11 reads LDP signalling state; both via existing engine integration | [ ] |
| OSPF <-> VRF (ext-13) | DN bit / VPN Route Tag loop prevention across per-VRF OSPF instances (future infra) | [ ] |

### Integration Points
- `internal/plugins/ospf/lsdb` (stores + flooding + origination) -- ext-1 and every opaque consumer attach here.
- `internal/plugins/ospf/packet` (codec + TLV helpers) -- ext-1 TLV helpers; ext-2/3/4 typed bodies.
- `internal/plugins/ospf/spf` (route table, reachability) -- read by ext-5/ext-6; never receives opaque vertices.
- `internal/plugins/ospf/neighbor` (NSM/DD) -- ext-1 O-bit; ext-10 BFD client; ext-12 Instance-ID demux.
- `internal/plugins/ospf/register.go` (registration + lifecycle) -- the ext-1 consumer registry; each child's own commands/schema/doctor.
- The delivered Loc-RIB / sysrib FIB-install seam -- SR/TI-LFA forwarding state.
- Ze's existing BFD engine (ext-10) and LDP signalling (ext-11).
- Future MPLS-L3VPN / VRF infrastructure (ext-13, BLOCKING).

### Architectural Verification
- [ ] No bypassed layers (extensions attach at delivered seams; SR/TI-LFA install through the existing Loc-RIB path, not a new FIB path)
- [ ] No unintended coupling (the opaque carrier names no consumer; consumers depend on the carrier, not vice-versa; base-only features do not depend on the opaque chain)
- [ ] No duplicated functionality (extensions reuse the delivered stores, flooding, SPF, NSM, and FIB-install seams)
- [ ] Zero-copy preserved (opaque bodies retained as views; TLV iterator zero-copy; buffer-first encode -- enforced per child)

## Wiring Test (MANDATORY -- NOT deferrable)

(Umbrella-level: proves the child set is reachable as a coordinated whole. Each child carries its own detailed, executable Wiring Test.)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| ext-1 opaque carrier present | -> | a consumer registers an Opaque Type and the engine discovers it | `TestOpaqueConsumerRegistered` (ext-1) + `test/ospf/ospf-opaque-register.ci` |
| ext-3 RI + ext-4 Extended Prefix/Link present | -> | ext-5 SR reads SRGB/SRLB + Prefix-SID/Adjacency-SID and programs an SR path | `test/ospf/ospf-sr-install.ci` (ext-5) |
| ext-5 SR present | -> | ext-6 computes a TI-LFA backup nexthop over the SR label stack | `test/ospf/ospf-tilfa.ci` (ext-6) |
| base only | -> | ext-10 drives NSM down on a BFD session failure | `test/ospf/ospf-bfd.ci` (ext-10) |
| future VRF infra present | -> | ext-13 sets/honours the DN bit for a PE-CE OSPF instance | `test/ospf/ospf-dnbit.ci` (ext-13, when unblocked) |

## Acceptance Criteria

(Umbrella-level coordination criteria; each child carries its own detailed, testable ACs.)

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The child set is written | Every child ext-1..ext-14 exists as a spec, cross-references its dependencies, and is consistent with the base umbrella's Shared Contracts |
| AC-2 | The build order is followed | No opaque consumer (ext-2/3/4/9/14) is scheduled before ext-1; SR (ext-5) not before ext-3 + ext-4; TI-LFA (ext-6) not before ext-5 |
| AC-3 | A base-only feature is implemented | ext-7/8/10/11/12 build on the delivered base without depending on the opaque chain |
| AC-4 | ext-13 is considered | It is recorded as BLOCKED on VRF/MPLS-L3VPN infra and is not implemented before that infra lands |
| AC-5 | A rested item is encountered | It is found in the "Out of scope (rested)" table with a rationale; reviving it requires a fresh decision and a new spec, not a quiet add to a child |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables OSPF opaque + a consumer | ext-1 carrier -> consumer registry -> origination/flooding/delivery | `test/ospf/ospf-opaque-*.ci` (ext-1) |
| 2 | Enables OSPF Segment Routing | ext-3 RI + ext-4 Extended Prefix/Link -> ext-5 SR label computation -> Loc-RIB SR path -> kernel | `test/ospf/ospf-sr-install.ci` (ext-5) |
| 3 | Enables TI-LFA fast reroute | ext-5 SR + delivered SPF -> ext-6 backup nexthop -> FIB | `test/ospf/ospf-tilfa.ci` (ext-6) |
| 4 | Enables BFD for OSPF | ext-10 BFD client registration -> sub-second NSM down on failure | `test/ospf/ospf-bfd.ci` (ext-10) |
| 5 | Runs OSPF as a PE-CE protocol (future) | ext-13 DN bit / VPN Route Tag over a per-VRF OSPF instance | `test/ospf/ospf-dnbit.ci` (ext-13, when unblocked) |

## 🧪 TDD Test Plan

### Unit Tests
(Per child; the umbrella aggregates, it does not own unit tests.)

| Test | File | Validates | Status |
|------|------|-----------|--------|
| (per child) | `internal/plugins/ospf/...` (per ext-N) | see each child spec ext-1..ext-14 | |

### Boundary Tests (MANDATORY for numeric inputs)
(Per child; the umbrella introduces no new numeric wire fields of its own.)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Opaque Type (LS-ID high byte) | 0-255 | 255 | N/A | N/A (1 byte) -- owned by ext-1 |
| Opaque ID (LS-ID low 24 bits) | 0-16777215 | 16777215 | N/A | N/A (masked) -- owned by ext-1 |
| (other numeric fields) | per child | -- | -- | per ext-N boundary tables |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| (per child) | `test/ospf/ospf-<ext>-*.ci` | per ext-N functional scenarios | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| (per child) | `test/interop/scenarios/ospf-<ext>-frr/` | FRR `ospfd` | per ext-N wire-behaviour interop (opaque/TE/RI/SR/GR/BFD as applicable) | |

### Future (if deferring any tests)
- All extension tests are owned by their children; ext-13 interop is deferred with the feature until VRF infrastructure exists.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files -->
- `internal/plugins/ospf/` -- the delivered OSPF edge plugin each child extends (lsdb/packet/spf/neighbor/register.go); the umbrella names no single file edit of its own
- `plan/spec-ospf-ext-1-opaque-framework.md` .. `plan/spec-ospf-ext-14-*.md` -- the child specs this umbrella coordinates (authoring deliverable)
- `docs/comparison.md`, `docs/features.md` -- OSPF-extension parity rows (per child, as each lands)
- NOTE: the umbrella itself modifies no feature code; each child lists its own `internal/plugins/ospf/...` edits

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | Per child | each ext-N adds its own leaves to `internal/plugins/ospf/yang/ze-ospf-conf.yang` |
| CLI commands/flags | Per child | each ext-N adds `show ip ospf <noun>` subcommands in `ze-ospf-cmd.yang` |
| Doctor check for runtime dependencies | Per child | ext-10 (BFD), ext-11 (LDP) and any new runtime dependency get their own check |
| Prometheus counters/metrics | Per child | each ext-N owns its `ze_ospf_<ext>_*` series; the umbrella metrics mapping is updated as children land |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Per child | `docs/features.md` (per ext-N) |
| 5 | Plugin added/changed? | Per child | `docs/guide/plugins.md`, `docs/plugin-overview.md` (per ext-N) |
| 9 | RFC behavior implemented? | Per child | `rfc/short/rfcNNNN.md` (created by each ext-N via `/ze-rfc`) |
| 11 | Affects daemon comparison? | Per child | `docs/comparison.md` (OSPF-extension parity rows) |
| 12 | Internal architecture changed? | Per child | the OSPF subsystem doc (per ext-N) |
| -- | Umbrella-level | This file | keep the Child Decomposition, Dependency / Build Order, and RFC Coverage tables current as children land or rest |

## Files to Create
- `plan/spec-ospf-ext-2-*.md` .. `plan/spec-ospf-ext-14-*.md` -- the child specs (ext-1 already written)
- (no feature files at the umbrella level) -- each child creates its own `internal/plugins/ospf/...` files and `test/ospf/*.ci` / `test/interop/scenarios/ospf-*-frr/`

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

1. **Phase: Opaque carrier** -- ext-1 (RFC 5250); the foundation that unblocks the opaque chain.
2. **Phase: Opaque consumers (parallel after ext-1)** -- ext-2 (TE), ext-3 (RI), ext-4 (Extended Link/Prefix), ext-9 (Grace-LSA), ext-14 (debug surface).
3. **Phase: Base-only protocol features (parallel)** -- ext-7 (virtual links), ext-8 (NBMA/P2MP), ext-10 (BFD), ext-11 (LDP-IGP sync), ext-12 (Multi-Instance).
4. **Phase: Segment Routing** -- ext-5, once ext-3 + ext-4 are done.
5. **Phase: TI-LFA / LFA** -- ext-6, once ext-5 is done.
6. **Phase: L3VPN DN bit (gated)** -- ext-13, LAST, only once MPLS-L3VPN/VRF infrastructure exists.
7. **Per-child verification + interop** -- `make ze-verify` + FRR scenarios, owned by each child.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this umbrella |
|-------|----------------------------------|
| Completeness | Every child ext-1..ext-14 exists, cross-references its dependencies, and matches the base umbrella's Shared Contracts |
| Correctness | The dependency / build order is honoured (opaque carrier first; SR after RI + Extended; TI-LFA after SR; ext-13 VRF-gated) |
| Naming | Each extension uses `ze_ospf_<ext>_*` metrics and `show ip ospf <noun>` subcommands; no existing series/command renamed |
| Data flow | Extensions attach at delivered seams; SR/TI-LFA install through the existing Loc-RIB path; opaque LSAs never enter SPF |
| Rule: plugin-self-containment | Each child's schema/help/doctor/commands live under `internal/plugins/ospf/`; the carrier names no consumer |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Umbrella + 14 child specs | `ls plan/spec-ospf-ext-*.md` |
| ext-1 written (Status:ready) | `grep -m1 '| Status |' plan/spec-ospf-ext-1-opaque-framework.md` |
| Each child cross-references its dependency | grep each child for its "Depends" row |
| Rested set recorded | `grep -A20 'Out of scope (rested' plan/spec-ospf-ext-0-umbrella.md` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Each extension's wire decode is bound-checked (opaque TLVs, TE/RI/Extended sub-TLVs); per child |
| Trust boundary | Opaque/extension LSAs flooded only to capable neighbours; received LSAs rely on the delivered OSPF authentication; no new unauthenticated surface |
| Resource exhaustion | Extension stores share the delivered LSDB caps; a flood of extension LSAs cannot grow memory unbounded; per child |
| Consumer isolation | A consumer callback panic is recovered and counted (ext-1 contract); a bad extension cannot crash OSPF |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the child that introduced it |
| Test fails wrong reason | Fix test setup |
| Test fails behavior mismatch | Re-read the child's RFC summary / Current Behavior |
| Build-order violation (consumer before carrier) | STOP; reorder; the carrier (ext-1) must land first |
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
- The extension set has exactly one hard root (ext-1, the opaque carrier) and a wide base-only fringe; capturing this split up front prevents the common error of scheduling SR or TE before the carrier exists.
- "Rested" is a distinct status from "deferred child": deferred children are scheduled work, rested items are deliberately-absent decisions that need re-opening. Conflating them is how a rejected feature silently creeps back.

## Core Insight
The OSPFv2 extension landscape is a single dependency tree rooted at the
RFC 5250 opaque carrier, plus a set of base-only protocol features that
parallelise it. The umbrella's job is to make that ordering explicit and to
record, with rationale, the features that were considered and deliberately
NOT scheduled -- so neither the build order nor the resting decisions are
re-litigated by accident.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Opaque carrier (ext-1) is the single foundation | Per-extension opaque handling | One generic RFC 5250 carrier keeps flooding/scope/O-bit logic in one place; consumers stay self-contained |
| SR depends on RI + Extended Prefix/Link | SR carries its own advertisements | RFC 8665 explicitly reuses the RI (RFC 7770) and Extended (RFC 7684) LSAs; duplicating them would diverge from FRR/interop |
| ext-13 recorded as VRF-gated/BLOCKED | Schedule it "later" with the rest | It has a hard external dependency (MPLS-L3VPN/VRF) absent from Ze; marking it merely "later" risks premature, unwireable work |
| Debug folds in ospfclient (ext-14) | Standalone ospfclient Unix-socket daemon | The useful inject/observe capability fits in-process on the ext-1 registry; a separate daemon adds a socket and trust boundary for no benefit |
| Flood Reduction/DoNotAge rested (deferred), not a child | Schedule it now / reject it outright | A pure optimisation with subtle failure modes; record as revisitable rather than build speculatively |

## Known Limitations
- This umbrella tracks OSPFv2 extensions ONLY. OSPFv3 (RFC 5340) and its extension follow-ups are a separate edge plugin under `plan/spec-ospfv3-0-umbrella.md`; nothing here applies to v3.
- The umbrella is a coordination document: it has no feature code, no tests, and no acceptance criteria that it implements itself. Completion is defined by its children, not by this file. It is never marked "done" while a tracked child is open.
- ext-13 (L3VPN PE-CE DN bit) cannot be implemented until Ze gains MPLS-L3VPN / VRF infrastructure; it is BLOCKED, not merely sequenced last. Implementing it before VRF lands is a scope error.
- The "rested" items are deliberately absent from the child set. Reviving any of them (notably Flood Reduction / DoNotAge, which is deferred rather than rejected) requires a fresh design decision and a new spec, not a quiet add to an existing child.
- The opaque chain (ext-2..ext-6, ext-9, ext-14) is hard-blocked on ext-1. Build-order violations (e.g. starting SR before RI + Extended Prefix/Link) produce specs that cannot be wired and must be rejected at planning time.

## RFC Documentation

Per-RFC implementation summaries (the `/ze-rfc` deep output under
`docs/architecture/rfc/`) and the short house-format summaries under
`rfc/short/` are produced by each CHILD spec at its own implementation time, for
the RFCs whose normative detail that child's code enforces. This umbrella adds
no RFC enforcement code and therefore carries no `// RFC NNNN Section X.Y`
annotations itself; it only records the RFC-to-child mapping in "RFC Coverage"
below. The base umbrella's `rfc/short/rfc5250.md` is the single pre-existing
summary (created as the noted out-of-scope framework reference) and is consumed
by ext-1.

### RFC Coverage (per child)
| Child | RFC(s) | Summary status |
|-------|--------|----------------|
| ext-1 | RFC 5250 | CREATED `rfc/short/rfc5250.md` (with the base umbrella) |
| ext-2 | RFC 3630, RFC 5392 | created by ext-2 |
| ext-3 | RFC 7770 | created by ext-3 |
| ext-4 | RFC 7684 | created by ext-4 |
| ext-5 | RFC 8665 | created by ext-5 |
| ext-6 | RFC 5286 + TI-LFA draft | created by ext-6 |
| ext-7 | RFC 2328 §15 | covered by `rfc/short/rfc2328.md` (delivered) |
| ext-8 | RFC 2328 (network types) | covered by `rfc/short/rfc2328.md` (delivered) |
| ext-9 | RFC 3623 | created by ext-9 |
| ext-10 | RFC 5880, RFC 5881 | created by ext-10 |
| ext-11 | RFC 5443, RFC 6138 | created by ext-11 |
| ext-12 | RFC 6549 | created by ext-12 |
| ext-13 | RFC 4576, RFC 4577 | created by ext-13 (when unblocked) |
| ext-14 | (tooling; no new RFC) | n/a |

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
| Enumerate the OSPFv2 extension follow-ups as a coordinated child set | Done | Child Decomposition table | ext-1..ext-14 |
| Fix the dependency / build order | Done | Dependency / Build Order | opaque chain + base-only set + VRF-gated ext-13 |
| Record the rested set with rationale | Done | Out of scope (rested) table | TOS, SNMP MIB, multi-area adjacencies, QoS, Flood Reduction/DoNotAge, ospfclient |
| Per-child implementation | (pending) | each `plan/spec-ospf-ext-N-*.md` | downstream |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1..AC-5 | (coordination) | this umbrella's tables | child ACs are detailed per child |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| (per child) | (pending) | `internal/plugins/ospf/...`, `test/ospf/...` | per ext-N |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `plan/spec-ospf-ext-1-opaque-framework.md` | Written | Status:ready |
| `plan/spec-ospf-ext-2-*.md` .. `plan/spec-ospf-ext-14-*.md` | (this batch) | Status:ready |
| `internal/plugins/ospf/` (extensions) | (pending) | per child |

### Audit Summary
- **Total items:** umbrella coordination (this deliverable) + downstream per-child implementation
- **Done:** child decomposition, dependency / build order, rested set
- **Partial:** 0
- **Skipped:** 0
- **Changed:** per-child implementation is downstream, tracked per child

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| OSPFv2 extension child set exists and is internally consistent | spec files + cross-references | `ls plan/spec-ospf-ext-*.md`; dependency / build-order table; each child's "Depends" row |
| The build order is captured | this file | Dependency / Build Order section (opaque chain + base-only set + ext-13 VRF gate) |
| The rested set is recorded with rationale | this file | Out of scope (rested) table |
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
| `plan/spec-ospf-ext-2-*.md` .. `plan/spec-ospf-ext-14-*.md` | (verify) | `ls plan/spec-ospf-ext-*.md` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | each child exists + cross-references | `ls plan/spec-ospf-ext-*.md`; grep each child's Depends row |
| AC-2 | build order honoured | Dependency / Build Order table |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| (downstream) | `test/ospf/*.ci` (per ext-N) | filled during implementation |

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
- [ ] All 14 child specs written and cross-referenced (ext-1 written; ext-2..ext-14 this batch)
- [ ] Dependency / build order captured and consistent
- [ ] Out-of-scope (rested) set recorded with rationale
- [ ] AC-1..AC-5 demonstrated by the umbrella's tables
- [ ] End-to-End User Stories each map to a child + a downstream test
- [ ] Wiring Test table complete (umbrella-level; detailed wiring per child)
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes (downstream, per child)
- [ ] Feature code integrated (`internal/plugins/ospf/`) (downstream, per child)
- [ ] Documentation Update Checklist answered (per child as each lands)
- [ ] Critical Review passes

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added (downstream, per child)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (no extension built before its dependency)
- [ ] No speculative features (rested table honoured)
- [ ] Single responsibility per child
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (base-only features independent of the opaque chain)

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
