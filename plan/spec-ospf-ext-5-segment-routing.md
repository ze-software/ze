# Spec: OSPF Segment Routing

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-ospf-ext-0-umbrella.md, spec-ospf-ext-3-router-information.md (IPv4 RI carrier), spec-ospf-ext-4-extended-link-prefix.md (IPv4 Extended-Prefix/Link carriers) |
| Phase | - |
| Updated | 2026-06-24 |

> Single feature across BOTH OSPF address families. Ze implements OSPF as ONE
> unified engine (`internal/plugins/ospf/`), exactly as `bgp` is one engine across
> address families: there is NO separate `ospfv3` plugin. The IPv4 family
> (OSPFv2) and the IPv6 family (OSPFv3, RFC 5340) run as two instances of the same
> engine over the same AF-neutral FSM/flooding/DR/SPF/LSDB machinery. SR's control
> logic (SRGB/SRLB management, index->label arithmetic, NP/E/M push/swap/PHP
> decision, `mpls-fib` install) is SHARED; only the wire carriage differs by
> address family: the IPv4 family rides RFC 8665 TLVs in the opaque RI / Extended
> Prefix / Extended Link LSAs (ext-3/ext-4 carriers), the IPv6 family rides RFC
> 8666 TLVs in the OSPFv3 RI Opaque LSA and the RFC 8362 Extended LSAs (added by
> this feature, since OSPFv3 has no opaque carrier yet). Per-AF differences are
> labelled with an **Address family** column or explicit IPv4 / IPv6 sub-rows.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/learned/972-ospf-af-unify.md` -- the OSPF AF-unification: one engine, two AF instances, AF-neutral FSM/flooding/DR/SPF/LSDB, AF-specific wire in the `_v6` strategy files + `internal/plugins/ospf/v3/{packet,types,transport}`
4. `rfc/short/rfc8665.md` -- the IPv4 (OSPFv2) SR wire spec: SR-Algorithm / SID-Label-Range(SRGB) / SRLB / SRMS-Pref TLVs in the RI LSA (§3), Extended Prefix Range TLV (§4), Prefix-SID Sub-TLV + V/L/NP/E/M flags (§5), Adj-SID (§6.1) + LAN-Adj-SID (§6.2) Sub-TLVs, the SID/Label Sub-TLV (§2.1, 3-octet label vs 4-octet index), label computation from SRGB index (§3.2), Adj-SID withdraw on adjacency < 2-Way (§7.4.1)
5. `rfc/short/rfc8666.md` -- the IPv6 (OSPFv3) SR wire spec: SID/Label sub-TLV (§3.1, type 7), Extended Prefix Range TLV (§5, type 9), Prefix-SID sub-TLV + NP/M/E/V/L flags (§6, type 4), Adj-SID (§7.1, type 5) + LAN-Adj-SID (§7.2, type 6) sub-TLVs, SR capabilities carried in the OSPFv3 RI Opaque LSA (§4, reuses the RFC 8665 capability TLVs unchanged), inter-area Prefix-SID propagation (§8.2), Adj-SID withdraw on adjacency < 2-Way (§8.4.1)
6. `rfc/short/rfc7770.md` -- the IPv4 RI LSA carrier (Opaque Type 4) holding the SR-Algorithm/SRGB/SRLB/SRMS top-level TLVs; first-instance rules; multi-instance tie-break (§3)
7. `rfc/short/rfc7684.md` -- the IPv4 Extended Prefix (Opaque Type 7) and Extended Link (Opaque Type 8) Opaque LSAs that hold the Prefix-SID and Adj-SID/LAN-Adj-SID sub-TLVs
8. `rfc/short/rfc5340.md` -- OSPFv3 base: 20-byte LSA header with a 16-bit scope-encoded LS Type (§A.4.2.1), IPv6 PrefixLength/PrefixOptions/padded-32-bit-word prefix encoding (§A.4.1)
9. `plan/spec-ospf-ext-0-umbrella.md` -- the OSPF extension umbrella (carrier delivery order, SR placement after ext-3/ext-4)
10. `internal/core/mplsfib/events.go` -- the `(mpls-fib, entry)` bus: `Entry{Op: Push/Swap/Pop, Action: Add/Remove, InLabel, FEC, OutLabels, NextHop, Source}`, `EntryChange.Emit`; fib-kernel is the single netlink owner
11. `internal/plugins/ldp/fib.go` + `internal/plugins/ldp/lib.go` -- the model for label-pool allocation (`allocateLabelLocked`, `nextLabel`, `MaxLabel`) and MPLS install via the mpls-fib bus (`ProgramPush`/`ProgramPop`)
12. `internal/plugins/ospf/spf/computer.go` + `spf/route.go` + `spf/graph.go` -- the AF-neutral SPF `Computer.Run()` that yields `[]RouteEntry` and the graph exposing per-vertex next-hops and neighbour adjacencies (shared by both AFs)
13. `internal/plugins/ospf/afstrategy_v6.go` + `internal/plugins/ospf/origination_v6.go` -- the IPv6-family SPF strategy and self-LSA origination (`v6OriginateSelf`, `v6OriginateRouter`, `v6OriginHeader`)
14. `internal/plugins/ospf/v3/packet/lsa.go` + `internal/plugins/ospf/v3/types/lsa.go` -- the OSPFv3 `LSA` typed-body union + verbatim `RawBytes` passthrough; the `LSType uint16` scope-encoding; the seams the new RI / Extended-LSA bodies attach to

## Task

Add Segment Routing to the unified OSPF engine at `internal/plugins/ospf/`,
covering BOTH address families with one feature: the IPv4 family per RFC 8665 and
the IPv6 family per RFC 8666. SR is the first OSPF consumer that programs the MPLS
data plane: it computes MPLS labels from advertised Prefix-SID indices against the
originator's SRGB and installs label-switched forwarding entries for Prefix-SIDs
(node SIDs) and Adjacency-SIDs through the existing `mpls-fib` bus, the same seam
LDP and RSVP-TE use.

**One engine, two address families.** The control plane is identical for both
AFs: enabling SR advertises the node's SR-Algorithm/SRGB/SRLB (and optionally
SRMS-Preference) capabilities, a Prefix-SID for each configured node prefix
(typically the loopback), and an Adj-SID (plus LAN-Adj-SID on broadcast/NBMA) for
each adjacency in state 2-Way or higher. On reception, the node records remote
SR-Algorithm and SRGB, computes the outgoing label from the originator's SRGB
(`label = SRGB_base + index`, resolving across ranges in advertised order),
applies the next-hop router's NP/E/M flags to decide push/swap/PHP, and installs
an MPLS push (ingress) or swap (transit) entry toward the SPF next-hop; for its
own Adj-SIDs it installs a pop/forward entry keyed by the SRLB local label it
allocated. The SRGB is a configured contiguous global label range the node owns;
the SRLB is a configured local label range from which Adj-SIDs are allocated. All
of this -- SRGB/SRLB management, the bounded 20-bit allocator, label arithmetic,
the NP/E/M truth table, and the `mpls-fib` install -- is SHARED between the two
address families.

**Only the wire carriage differs by address family.** Per the AF-unify design
(`plan/learned/972-ospf-af-unify.md`), the AF-specific code lives in the `_v6`
strategy files and `internal/plugins/ospf/v3/{packet,types}`:

| Address family | SR capability carrier | Prefix-SID carrier | Adj-SID / LAN-Adj-SID carrier | RFC / type codes |
|----------------|----------------------|--------------------|-------------------------------|------------------|
| IPv4 (OSPFv2) | RI Opaque LSA (Opaque Type 4), top-level TLVs 8/9/14/15 -- delivered by ext-3 | Prefix-SID Sub-TLV (type 2) under the Extended Prefix TLV of the Extended Prefix Opaque LSA (Opaque Type 7) -- delivered by ext-4 | Adj-SID (type 2) / LAN-Adj-SID (type 3) Sub-TLVs under the Extended Link TLV of the Extended Link Opaque LSA (Opaque Type 8) -- delivered by ext-4 | RFC 8665, carriers RFC 7770 / RFC 7684 |
| IPv6 (OSPFv3) | OSPFv3 RI Opaque LSA (RFC 7770, function code 12; area `0xA00C` / AS `0xC00C`) -- **added by this feature** | Prefix-SID sub-TLV (type 4) under the prefix carriage of an RFC 8362 Extended prefix LSA -- **added by this feature** | Adj-SID (type 5) / LAN-Adj-SID (type 6) sub-TLVs under the Router-Link TLV of the E-Router-LSA (`0x2021`) -- **added by this feature** | RFC 8666, capability TLVs reused from RFC 8665; carriage RFC 7770 / RFC 8362 |

For the IPv4 family the three carriers already exist (the RFC 5250 opaque
framework ext-1, the RFC 7770 RI LSA ext-3, the RFC 7684 Extended Prefix/Link
ext-4); SR registers TLV emitters/parsers with them and re-implements nothing.
For the IPv6 family the OSPFv3 codec today defines ONLY the RFC 5340 base LSAs and
has no opaque/RI/Extended carriage; therefore this feature ALSO adds the OSPFv3 RI
Opaque LSA and the RFC 8362 Extended-LSA subset RFC 8666 needs (E-Router-LSA for
Adj-SID/LAN-Adj-SID; the E-prefix LSAs for Prefix-SID and Extended Prefix Range),
with verbatim flood/passthrough for unknown TLVs. The RFC 8666 type codes are the
OSPFv3 Extended-LSA registry values (Prefix-SID 4, Adj-SID 5, LAN-Adj-SID 6,
SID/Label 7, Extended Prefix Range 9), NOT the OSPFv2 RFC 8665 numbers.

The feature is self-contained inside the OSPF engine: it registers the SR TLV
emitters/parsers, the SR config leaves (`ospf { segment-routing { ... } }` for the
IPv4 family and `ospf { address-family { ipv6 { segment-routing { ... } } } }` for
the IPv6 family), the `show ospf segment-routing` and `show ospf ipv6
segment-routing` CLI, the doctor checks, and the SR metrics. Removing the SR code
removes all SR behaviour for both AFs; OSPF and the opaque carriers behave exactly
as before.

### In scope (this spec)

Shared (both address families) unless an Address-family column narrows it.

| Item | Detail | Address family |
|------|--------|----------------|
| SR-Algorithm TLV origination + reception | advertise Algorithm 0 (SPF) when SR enabled; record remote routers' algorithms; area-scoped | both |
| SRGB (SID/Label Range TLV) origination + reception | MAY appear multiple times; each range carries exactly one SID/Label Sub-TLV; receiver concatenates ranges in advertised order to map index -> label | both |
| SRLB (SR Local Block TLV) origination + reception | the local label range from which Adj-SIDs are allocated | both |
| SRMS-Preference TLV origination + reception | optional; first-occurrence + narrowest-scope tie-break | both |
| SID/Label Sub-TLV codec | 3-octet (20-bit MPLS label) or 4-octet (32-bit SID) forms; width inferred from V/L; shared between SRGB/SRLB and the prefix/adj sub-TLVs | both (sub-TLV type 1 in v2, type 7 in v3) |
| Prefix-SID Sub-TLV origination + reception | NP/M/E/V/L flags; index (4-octet, V=0/L=0) or local label (3-octet, V=1/L=1); algorithm; topology | both (type 2 in v2 under Extended Prefix TLV; type 4 in v3 under the E-prefix carriage) |
| Extended Prefix Range TLV origination + reception | range advertisement; decode/store; Prefix-SID-per-range starting-value semantics | both (IPv4: type 2 top-level of Extended Prefix Opaque LSA, IA-Flag set by ABR; IPv6: type 9, AF field + IPv6 padded prefix) |
| Adj-SID Sub-TLV origination + reception | B/V/L/G/P flags; weight; allocated from the SRLB; withdrawn when adjacency drops below 2-Way | both (type 2 in v2 under Extended Link TLV; type 5 in v3 under the E-Router Router-Link TLV) |
| LAN-Adj-SID Sub-TLV origination + reception | carries the Neighbor ID; broadcast/NBMA only | both (type 3 in v2; type 6 in v3) |
| SRGB/SRLB label-range management | configured SRGB (global) and SRLB (local) ranges this node owns; a bounded 20-bit allocator for Adj-SID local labels reusing the LDP `nextLabel`/`MaxLabel` pattern | both (shared) |
| Label computation from index | `label = SRGB_base + index` resolved across the originator's advertised ranges in order; reject out-of-range index; honour V=1/L=1 absolute local-label form | both (shared) |
| MPLS forwarding install for Prefix-SID | Push (ingress) or Swap (transit) toward the SPF next-hop via `mpls-fib`; NP=0 -> PHP; E=1 -> Explicit NULL; M-flag ignores NP/E | both (shared logic; IPv4 Explicit NULL label 0, IPv6 Explicit NULL label 2) |
| MPLS forwarding install for Adj-SID | Pop/forward entry keyed by the local Adj-SID label this node allocated; forwarded to the specific adjacency (bypassing SPF) | both (shared) |
| OSPFv3 RI Opaque LSA carriage | RFC 7770 RI LSA for OSPFv3 (function code 12; area `0xA00C`, AS `0xC00C`); originate / decode; verbatim flood; unknown RI TLVs stored + reflooded, never interpreted | IPv6 only (added by this feature) |
| RFC 8362 Extended-LSA carriage (SR subset) | E-Router-LSA (`0x2021`) Router-Link TLV; E-Intra-Area-Prefix-LSA (`0x2029`), E-Inter-Area-Prefix-LSA (`0x2023`), E-AS-External-LSA (`0x4025`), E-Type-7-LSA (`0x2027`) prefix TLVs; 4-byte-aligned TLV/sub-TLV iteration; verbatim passthrough for unknown TLVs | IPv6 only (added by this feature) |
| Inter-area Prefix-SID propagation | when the ABR re-advertises a prefix across areas, include the Prefix-SID (best path in source area else backbone), NP set / E clear unless directly attached | both (IPv4 §4 IA-Flag; IPv6 §8.2) |
| CLI + metrics | `show ospf segment-routing` (IPv4) and `show ospf ipv6 segment-routing` (IPv6); `ze_ospf_sr_*` counters/gauges | both |

### Out of scope (noted so it is not silently assumed done)

| Item | Where / reason |
|------|---------------|
| TI-LFA / topology-independent loop-free alternates | `spec-ospf-ext-6-ti-lfa.md` (the Adj-SID B-Flag is advertised but no backup path is computed here) |
| SR-TE policies (segment lists, binding SIDs) | BGP SR-Policy is a separate subsystem; OSPF SR carries only the building-block SIDs |
| SRv6 (IPv6 data-plane SIDs) | RFC 8666 §1 and RFC 8665 §1 explicitly defer the IPv6 data plane; only the MPLS data plane is in scope for both AFs |
| Full RFC 8661 SR Mapping Server server role | only the wire carriage (SRMS-Pref TLV, Extended Prefix Range TLV, M-Flag) is implemented; this node honours received mapping-server SIDs but does not originate Range TLVs or inject NU-bit prefixes |
| Strict-SPF (Algorithm 1) path computation | the SR-Algorithm TLV records Algorithm 1 if a peer advertises it, but Ze computes only Algorithm 0; a Prefix-SID for an algorithm Ze does not compute is recorded but not installed |
| Full RFC 8362 Extended-LSA feature set / full RFC 7770 RI consumer framework (IPv6) | only the Extended-LSA types + TLVs and RI capability TLVs SR rides on are interpreted; other TLVs are passed through verbatim, not understood |
| The opaque carrier / RI LSA codec / Extended Prefix/Link LSA codec for the IPv4 family | ext-1 / ext-3 / ext-4 (this spec consumes them; it does NOT re-implement TLV iteration, RI origination, or Extended LSA origination for IPv4) |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `plan/learned/972-ospf-af-unify.md` -- one OSPF engine, two AF instances; AF-neutral FSM/flooding/DR/SPF/LSDB; AF-specific wire in `_v6` files + `internal/plugins/ospf/v3`
  -> Decision: SR's control logic (SRGB/SRLB, index->label, NP/E/M, install) is written once and shared; only the TLV codec + LSA carriage are AF-specific
  -> Constraint: NEVER add a separate `ospfv3` plugin; the IPv6 SR work lands in `internal/plugins/ospf/v3/{packet,types}` and the `_v6` origination/strategy files, with shared SR control logic in the engine
- [ ] `docs/research/ospf-implementation-guide.md` §14 ("Segment Routing", ~1526-1537) + the Router-Information / Extended-Prefix landscape -- FRR "advertises prefix-SIDs and adjacency-SIDs, allocates SRGB/SRLB, integrates with the MPLS forwarding plane"
  -> Decision: model SR as a consumer (like FRR's `ospf_sr.c` / `ospf6_*`), layered on the RI and Extended carriers; the only OSPF-core touch points are reading the shared SPF route table + adjacency set for next-hops/neighbour IDs
  -> Constraint: SR "integrates with the MPLS forwarding plane" -- install-only through the existing `mpls-fib` bus; OSPF SR never touches netlink (fib-kernel owns it)
- [ ] `internal/plugins/ldp/fib.go` (the MPLS install model) -- `ProgramPush`/`ProgramPop`/`Remove` emit `mplsfibevents.Entry` with a source tag; `mplsSourceLDP=2`, RSVP-TE uses 1
  -> Decision: SR allocates distinct source tags (`mplsSourceOSPFSR` for IPv4, `mplsSourceOSPFv3SR` for IPv6); SR programs push (ingress) and swap (transit) for prefix-SIDs and pop (egress) for adj-SIDs, mirroring LDP's three operations
  -> Constraint: implicit-null (3) signals PHP -- a Prefix-SID with NP=0 means the penultimate hop pops, so a directly-attached SR egress forwards as plain IP (no push), the same rule LDP applies to implicit-null
- [ ] `internal/plugins/ldp/lib.go` (the label-pool model) -- `LIB.allocateLabelLocked`/`nextLabel`/`MaxLabel` is a bounded 20-bit allocator skipping used labels
  -> Decision: the SRLB Adj-SID allocator reuses this bounded-allocator shape (seeded at the SRLB base, capped at the SRLB end), NOT a new allocator abstraction; one allocator type serves both AFs
  -> Constraint: the SRGB is NOT dynamically allocated -- it is a configured contiguous range this node owns and advertises verbatim; only the SRLB drives per-adjacency allocation
- [ ] `internal/plugins/ospf/v3/packet/lsa.go` + `internal/plugins/ospf/v3/types/lsa.go` -- the OSPFv3 `LSA` typed-body union with verbatim `RawBytes` passthrough; `LSType uint16` scope-encoding (`0x2001` area, `0x4005` AS, `0x0008` link-local), `Scope()`, base constants
  -> Constraint: ADD typed bodies for the RI LSA and the SR-relevant Extended LSAs to the union and the `WriteTo`/`bodyLen`/`hasTypedBody` switches; ADD the RI/Extended LS-Type constants and widen `Known()`; scope falls out of the high bits; do NOT rebuild the passthrough machinery
- [ ] `ai/rules/plugin-self-containment.md` -- SR is self-contained inside the OSPF engine
  -> Constraint: no SR spelling (Prefix-SID, Adj-SID, SRGB) appears in ext-1/ext-3/ext-4, the v3 base codec, or the OSPF core; SR registers its TLV emitters/parsers, config, CLI, doctor, and metrics from its own registration; removing the SR code removes all SR behaviour for both AFs; the `mpls-fib` `Source` tag is the only marker in generic code
- [ ] `ai/rules/buffer-first.md` -- SR TLV emit and RI/Extended-LSA encode are buffer-first
  -> Constraint: SR sub-TLV/TLV emission uses the carrier's TLV builder (`WriteTo(buf, off) int`, 4-octet pad written explicitly); the SID/Label field (3 or 4 octets) is written into the caller buffer; the IPv6 prefix 32-bit-word pad is written, never produced by slice concatenation; index-to-label is integer arithmetic, no allocation
- [ ] `ai/rules/no-sprintf-alloc.md` -- no `fmt`/`+` on the wire or hot path
  -> Constraint: `show ... ospf segment-routing` rendering uses `textbuf`/`AppendTo`; the label/index arithmetic on the SPF/forwarding hot path allocates nothing
- [ ] `ai/rules/memory-architecture.md` -- value-typed cross-boundary payloads
  -> Constraint: the SR forwarding entries handed to `mpls-fib` are value-typed (`mplsfibevents.Entry`, fixed-size fields, an owned `OutLabels` slice), carrying no pointer into SR state; SR copies any slice it retains; TLV iterators return views over caller bytes

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc8665.md` -- the IPv4 SR wire + behaviour spec (and the capability TLVs reused by RFC 8666 §4)
  -> Constraint: §3.1 -- if the SR-Algorithm TLV is advertised it MUST include Algorithm 0; multiple SR-Algorithm TLVs resolve to the first occurrence, area-scoped over AS-scoped, then smallest Instance ID; SR-Algorithm/SRGB/SRLB area-scoped
  -> Constraint: §3.2 -- Range Size MUST be > 0; exactly one SID/Label Sub-TLV per range (ignore the range TLV otherwise); ranges MUST NOT overlap; the receiver MUST build the index->label map by concatenating ranges in advertised order; the originator MUST keep range order stable across graceful restart
  -> Constraint: §3.3 SRLB Range Size > 0, Adj-SIDs from the SRLB; §3.4 SRMS-Pref first-occurrence/narrowest-scope/smallest-Instance-ID
  -> Constraint: §4 -- all prefix ranges in one Extended Prefix Opaque LSA share flooding scope; an ABR propagating the Extended Prefix Range TLV between areas MUST set the IA-Flag
  -> Constraint: §5 -- only V=0/L=0 (4-octet index) and V=1/L=1 (3-octet local label) are valid; any other combination MUST cause the SID Advertisement to be ignored; a Prefix-SID whose algorithm is not in the originator's SR-Algorithm TLV MUST be ignored; multiple Prefix-SIDs for the same prefix/topology/algorithm MUST all be ignored; the outgoing label MUST honour the next-hop router's NP/E/M flags (M set -> ignore NP/E); NP set + E clear for ABR/ASBR non-attached prefix-SIDs
  -> Constraint: §6.1 Adj-SID flags B/V/L/G/P; reserved bits zero; §7.4.1 a P2P adjacency below 2-Way MUST withdraw the Adj-SID
  -> Constraint: §9/§10 -- an invalid TLV/sub-TLV length means the LSA is malformed and MUST be ignored; MUST NOT crash; reception SHOULD be counted/logged with rate limiting
- [ ] `rfc/short/rfc8666.md` -- the IPv6 (OSPFv3) SR wire spec
  -> Constraint: §3.1 SID/Label sub-TLV (type 7) is 3 octets (V=1/L=1) or 4 octets (V=0/L=0); width implied by V/L; invalid V/L combination MUST be ignored
  -> Constraint: §6 Prefix-SID flags NP/M/E/V/L; algorithm-not-advertised MUST be ignored; multiple Prefix-SIDs for one prefix/topology/algorithm MUST all be ignored; M-flag -> ignore NP and E on reception; the outgoing label MUST honour the next-hop router's NP/E/M (NP=0 PHP; NP=1/E=0 keep; NP=1/E=1 Explicit NULL = IPv6 label 2); ABR/ASBR-propagated prefixes set NP, clear E unless directly attached
  -> Constraint: §7.1/§7.2 Adj-SID/LAN-Adj-SID flags B/V/L/G/P; P-flag -> persistent; §8.4.1 a P2P adjacency below 2-Way MUST withdraw the Adj-SID
  -> Constraint: §5 Extended Prefix Range TLV (type 9): AF 0=IPv4/1=IPv6, IPv6 prefix consumes `((PrefixLength+31)/32)` 32-bit words zero-padded; duplicate Range TLV -> smallest Instance ID wins; the carried Prefix-SID is the *starting* value (Nth prefix gets start+N)
  -> Constraint: §10/§11 invalid TLV/sub-TLV length -> the whole LSA is malformed and MUST be ignored; MUST NOT crash; SHOULD count/log rate-limited
  -> Constraint: the type codes are the OSPFv3 Extended-LSA registry values (Prefix-SID 4, Adj-SID 5, LAN-Adj-SID 6, SID/Label 7, Extended Prefix Range 9), NOT the OSPFv2 RFC 8665 values; reusing the OSPFv2 numbers is a bug
- [ ] `rfc/short/rfc7770.md` -- the RI LSA carrier (Opaque Type 4 for IPv4 via ext-3; function code 12 for IPv6 added here)
  -> Constraint: §2.1/§2.7 -- the SR top-level TLVs ride the RI LSA at area scope (SRMS-Pref MAY use AS scope); the multi-instance tie-break (§3, smallest Instance ID) governs SR-Algorithm and SRMS-Pref when a router floods more than one RI LSA
- [ ] `rfc/short/rfc7684.md` -- the IPv4 Extended Prefix/Link carriers (ext-4 dependency)
  -> Constraint: §2.1 Prefix-SID rides as a sub-TLV under the Extended Prefix TLV (Type 1) of the Extended Prefix Opaque LSA (Type 7); §3.1 Adj-SID/LAN-Adj-SID ride under the Extended Link TLV (Type 1) of the Extended Link Opaque LSA (Type 8, area scope only); §5 a malformed TLV/sub-TLV makes the whole LSA malformed: MUST NOT be stored, acked, or reflooded; lowest-Opaque-ID instance wins
- [ ] `rfc/short/rfc5340.md` -- the OSPFv3 base this extends for the IPv6 carriage
  -> Constraint: the 20-byte LSA header carries a 16-bit scope-encoded LS Type (U|S2|S1|function); the new RI/Extended types use the same scope encoding (RI area `0xA00C`, E-Router area `0x2021`, E-AS-External AS `0x4025`, E-Link link-local `0x0028`); IPv6 prefixes use PrefixLength + PrefixOptions + padded 32-bit words, which the Extended Prefix Range TLV reuses

**Key insights:**
- SR is a *consumer with a data-plane tail* across both AFs: the shared work is the SRGB/SRLB management, the index->label arithmetic, the NP/E/M push/swap/PHP decision, and the `mpls-fib` install. The AF-specific work is the TLV/sub-TLV codec and the LSA carriage.
- IPv4 SR plugs into delivered carriers (ext-1/ext-3/ext-4); it never parses an LSA header or floods anything itself. IPv6 SR additionally adds the OSPFv3 RI Opaque LSA and the RFC 8362 Extended-LSA subset, since the base OSPFv3 codec has neither.
- The single most error-prone shared piece is the multi-range SRGB index->label arithmetic (range ordering); it gets dedicated boundary tests and an FRR label-agreement interop for each AF.
- The shared SPF (`spf.Computer`) already gives the next-hop and route type per prefix; the graph gives neighbour Router IDs per adjacency. SR reads both (read-only) and never changes SPF. SR LSAs are never SPF vertices.
- Adj-SID lifecycle is driven by adjacency state: allocate from the SRLB at 2-Way, withdraw (and free the label) below 2-Way (RFC 8665 §7.4.1 / RFC 8666 §8.4.1).

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)

Shared engine / data plane (both AFs):
- [ ] `internal/core/mplsfib/events.go` -- `Entry{Action, Op (Push/Swap/Pop), InLabel, FEC, OutLabels, NextHop, Source}`; `EntryBatch`; `EntryChange = events.Register[*EntryBatch]("mpls-fib","entry")`; producers Emit, fib-kernel Subscribes
  -> Constraint: this is the ONLY way SR programs forwarding for either AF; SR emits an `EntryBatch` per change, value-typed, with a distinct `Source` tag; no direct netlink, no sysrib best-path abuse for label-keyed swap/pop entries
- [ ] `internal/plugins/ldp/fib.go` -- `ProgramPush(fec,label,nextHop)` (ingress), `ProgramPop(fec,inLabel)` (egress), `Remove(fec)` (idempotent, tracks `pushed`); `mplsSourceLDP=2`; implicit-null (3) => forward as plain IP, no push
  -> Constraint: SR mirrors this for both AFs: a per-FEC pushed-set for idempotent removal; an Adj-SID pop keyed by InLabel; PHP (NP=0) handled like LDP's implicit-null
- [ ] `internal/plugins/ldp/lib.go` -- `LIB.allocateLabelLocked()` walks `nextLabel` from 16, skips `usedLabels`, wraps at `MaxLabel` (20-bit); `EnsureLocal`; `AllocateLabel`
  -> Constraint: the SRLB Adj-SID allocator reuses this bounded-pool shape seeded/bounded by the configured SRLB range; the SRGB is configured, not allocated; one allocator implementation serves both AFs
- [ ] `internal/plugins/ospf/spf/computer.go` -- AF-neutral `Computer.Run()` produces `selected []RouteEntry`; `SetOnChange(fn)` fires after each run with a `RouteDelta` (redistribution trigger, NOT the FIB path); `Routes()`/`Snapshot()` expose the table; per-area `Result`s carry `Nodes` with next-hop sets
  -> Constraint: SR hooks a post-run callback (a sibling to `SetOnChange`) to recompute prefix-SID labels when the route table changes; SR reads `Routes()` for next-hop + route type per Prefix-SID prefix; SR does NOT install IP routes -- it installs LABEL entries; the post-run hook fires AFTER the IP-route install
- [ ] `internal/plugins/ospf/spf/route.go` -- `RouteEntry{AreaID, Prefix, Metric, Type (intra/inter/ext1/ext2), Origin (RouterID), NextHops []NextHop}`; `NextHop{Addr, Interface}` (AF-neutral)
  -> Constraint: the Prefix-SID install needs the prefix's `NextHops` (push toward each ECMP next-hop) and `Origin`/`Type` (to apply the NP/E inter-area/external rules); reuse `RouteEntry`, do not recompute reachability
- [ ] `internal/plugins/ospf/spf/graph.go` -- `Graph.Routers map[RouterID]*RouterVertex`; `RouterVertex{ID, Links}`; `Result.Nodes` keyed by `VertexID{Kind, Router, Network}` (AF-neutral)
  -> Constraint: the Adj-SID/LAN-Adj-SID install + the per-neighbour next-hop come from the adjacency set; SR maps a local adjacency (interface + neighbour Router ID) to the neighbour's next-hop for the pop/forward entry
- [ ] `internal/plugins/ospf/iface/ism.go` + `iface/iface.go` -- interface/neighbour state machine; adjacency states; transitions
  -> Constraint: the Adj-SID lifecycle (allocate at 2-Way, withdraw below 2-Way) is driven by these adjacency-state transitions for both AFs; SR subscribes to (or is polled on) adjacency change, allocating/freeing an SRLB label and re-originating the carrier LSA
- [ ] `internal/plugins/ospf/register.go` -- `registerOSPF()`, `runOSPFEngine`, the v4 and v6 engine instances (`eng6` over `v6Codec{}` driven by `cfg.V6`), metrics + snapshot dispatch; consumers wired in `OnStarted`
  -> Constraint: the SR consumer's metrics + snapshot + post-run hook + adjacency subscription are wired here for both AFs; `cfg.V6` carries the IPv6 SR block; `show ospf segment-routing` and `show ospf ipv6 segment-routing` add dispatch keys returning the per-AF SR snapshot
- [ ] `internal/plugins/ospf/spf/install.go` -- the `Installer` inserts `locrib.Path` per ECMP next-hop, AdminDistance 110
  -> Constraint: SR does NOT touch the Installer (IP routes); SR's output is the MPLS push that rides ON TOP of the IP route the Installer already created (fib-kernel attaches the MPLS encap, as LDP push works)

IPv4 family carriers (delivered):
- [ ] ext-1 opaque framework + ext-3 RI LSA + ext-4 Extended Prefix/Link (per `spec-ospf-ext-1-opaque-framework.md`, `spec-ospf-ext-3-router-information.md`, `spec-ospf-ext-4-extended-link-prefix.md`) -- the RFC 5250 carrier with the generic TLV iterator/builder, the RI originator + RI TLV codec (Opaque Type 4), and the Extended Prefix (Type 7) + Extended Link (Type 8) originators/decoders
  -> Constraint: SR registers TLV emitters/parsers with ext-3 (RI top-level TLVs 8/9/14/15) and ext-4 (Prefix-SID sub-TLV 2, Adj-SID sub-TLV 2, LAN-Adj-SID sub-TLV 3); SR adds TLV types, not new carriers; no SR spelling in ext-1/ext-3/ext-4

IPv6 family carriers (added by this feature):
- [ ] `internal/plugins/ospf/v3/packet/lsa.go` -- the OSPFv3 `LSA` struct: a typed-body union (`Router`, `Network`, `InterAreaPfx`, `InterAreaRtr`, `External`, `Link`, `IntraAreaPfx`) plus `Body`/`RawBytes`; `DecodeLSA` retains raw bytes and lazily decodes; `WriteTo` re-emits `RawBytes` verbatim when no typed body is set; `hasTypedBody()` gates re-marshal; `LSAIterator` is bound-checked and never panics
  -> Constraint: ADD `RouterInfo` + the Extended-LSA typed bodies to the union and the `WriteTo`/`bodyLen`/`hasTypedBody` switches; reuse the verbatim passthrough; do NOT rebuild the passthrough machinery
- [ ] `internal/plugins/ospf/v3/types/lsa.go` -- `LSType uint16` with scope in the high two bits, `Scope()`, `Function()`; base constants `LSTypeRouter 0x2001` ... `LSTypeIntraAreaPrefix 0x2009`; `Known()` enumerates the base set
  -> Constraint: ADD the RI LSA type (`LSTypeRouterInfo` area `0xA00C` / AS `0xC00C`) and the Extended-LSA types (`0x2021` E-Router, `0x2029` E-Intra-Area-Prefix, `0x2023` E-Inter-Area-Prefix, `0x4025` E-AS-External, `0x2027` E-Type-7, `0x0028` E-Link); scope falls out of the high bits; widen `Known()` so the shared LSDB stores + floods them by scope
- [ ] `internal/plugins/ospf/v3/packet/lsa_intraarea_prefix.go`, `lsa_router.go`, `lsa_external.go`, `prefix.go` -- the base RFC 5340 Router-LSA (graph links only), Intra-Area-Prefix-LSA, External-LSA bodies, and the IPv6 prefix codec (`((PrefixLength+31)/32)` words)
  -> Constraint: the Extended-LSA prefix carriage and the Extended Prefix Range TLV REUSE the existing IPv6 `Prefix` codec; do NOT reimplement IPv6 prefix encoding
- [ ] `internal/plugins/ospf/lsdb/origination.go` -- `OriginateSelf(area, key, body, SelfLSAEncoder)` is the AF-neutral self-LSA origination seam (sequencing, MinLSInterval, install, flood); `OriginateRouter`, `OriginateSummary`, `OriginateExternal`
  -> Constraint: the IPv6 RI LSA and the self-originated Extended-LSAs originate through `OriginateSelf` with a `SelfLSAEncoder`; sequencing/age/flood/withdraw (MaxAge) are inherited; no new origination path
- [ ] `internal/plugins/ospf/afstrategy_v6.go` -- the v6 SPF strategy: `BuildGraph`, `BuildRoutes`/`v6BuildRoutes`, `ComputeInterArea`, `ComputeExternal`, `OriginateSummaries`/`v6OriginateSummaries`, `NextHopSource` (`P2PNextHop`/`TransitNextHop` -> IPv6 link-local next-hop)
  -> Constraint: IPv6 Prefix-SID label install consumes the v6 `RouteEntry` set + per-next-hop neighbour RouterID; Adj-SID install consumes the per-adjacency neighbour identity; the next-hop comes from `NextHopSource`, not a new SPF; §8.2 propagation hooks `OriginateSummaries`
- [ ] `internal/plugins/ospf/origination_v6.go` -- `v6OriginateSelf(router, maxMetric)`, `v6OriginateRouter`, `v6OriginateIntraAreaPrefix`, `v6OriginateNetwork`, `v6OriginHeader(t, lsid, router, seq, purge)`
  -> Constraint: IPv6 SR origination hangs off `v6OriginateSelf`: originate/refresh the RI LSA (SR capabilities), the E-Router-LSA (Adj-SIDs), and the Prefix-SID-bearing E-prefix LSA when SR is enabled; `v6OriginHeader` builds the header for the new types

**Behavior to preserve:**
- The shared OSPF SPF route table, `RouteEntry`/`NextHop` shapes, the `Installer` IP-route install, the `SetOnChange` redistribution trigger, and all existing OSPF behaviour for both AFs. A router without SR enabled behaves exactly as today.
- The `mpls-fib` bus contract (`Entry`/`EntryBatch`), fib-kernel as the single netlink owner, the LDP/RSVP-TE source tags (SR takes two new, distinct tags).
- IPv4: the ext-1 TLV iterator/builder, the ext-3 RI LSA origination/codec, and the ext-4 Extended Prefix/Link origination/codec are consumed unchanged; SR adds TLV types, not new carriers.
- IPv6: the OSPFv3 base codec's verbatim passthrough (`LSA.WriteTo` re-emits `RawBytes`), the LSDB key triple, the scope-encoded LS Type, the IPv6 `Prefix` codec, and `OriginateSelf`/`SelfLSAEncoder`. With SR off no RI/Extended SR LSA is originated, and received RI/Extended LSAs are stored + reflooded but produce no label install. All existing v3 functional/interop tests stay green.

**Behavior to change:** (all RFC-required, not discretionary)
- When SR is enabled (per AF): the RI LSA gains the SR-Algorithm/SRGB/SRLB(/SRMS) top-level TLVs; the node originates Prefix-SID-bearing prefix LSAs and Adj-SID/LAN-Adj-SID-bearing link LSAs in the AF-appropriate carrier.
- On receiving SR TLVs: the node records remote SRGBs/algorithms and installs MPLS forwarding for reachable prefix-SIDs and for its own adj-SIDs.
- Adjacency transitions below 2-Way withdraw the corresponding Adj-SID (RFC 8665 §7.4.1 / RFC 8666 §8.4.1) and free its SRLB label.
- IPv6 additionally: `v3/types/lsa.go` gains the RI + Extended-LSA type constants and a widened `Known()`; `v3/packet/lsa.go` gains the RI + Extended typed bodies.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Config:** `ospf { segment-routing { ... } }` (IPv4) and `ospf { address-family { ipv6 { segment-routing { ... } } } }` (IPv6) resolve into the engine SR config (the IPv6 block into `cfg.V6`).
- **Origination:** SR enabled -> on the next self-LSA origination the AF carrier emits the SR TLVs. IPv4: SR emitters registered with ext-3 (RI) and ext-4 (Ext-Prefix/Ext-Link) are called by the carriers. IPv6: a v6 topology/adjacency/prefix change triggers `v6OriginateSelf` -> the RI LSA + E-Router-LSA (Adj-SIDs) + Prefix-SID-bearing E-prefix LSA.
- **Reception:** an RI / prefix / link LSA carrying SR TLVs arrives -> the AF carrier decodes -> the shared SR parser records the originator's SR-Algorithm + ordered SRGB (RI) or stores a Prefix-SID / Adj-SID entry.
- **Forwarding trigger:** a (shared) SPF run completes (post-run hook) OR a remote SR LSA changes OR a local adjacency changes -> SR recomputes labels and emits `mpls-fib` entries.
- **Adjacency lifecycle:** an adjacency reaches/leaves 2-Way -> SR allocates/frees an SRLB label and re-originates the AF link carrier LSA.

### Transformation Path
1. **SR config resolve (shared):** the SR YANG leaves (`enable`, `srgb` range(s), `srlb` range, per-prefix `prefix-sid` index, `node` flag) resolve into the engine's per-AF SR config.
2. **Emitter registration / origination hook (AF-specific):** IPv4 -- SR registers TLV emitters with ext-3 (RI top-level TLVs 8/9/14/15) and ext-4 (Prefix-SID sub-TLV 2, Adj-SID 2, LAN-Adj-SID 3). IPv6 -- SR origination hangs off `v6OriginateSelf`/`v6OriginateSummaries` building the RI LSA + E-Router-LSA + E-prefix LSA bodies.
3. **SR codec (AF-specific type codes, shared shapes):** the SID/Label Sub-TLV (3-octet label / 4-octet index), the SR-Algorithm/SRGB/SRLB/SRMS bodies, the Prefix-SID body (flags + topology + algorithm + SID/Index/Label), the Adj-SID/LAN-Adj-SID bodies (flags + weight + [neighbour] + SID/Index/Label), the Extended Prefix Range TLV -- encode + decode with V/L validation. IPv4 uses the RFC 8665 codes (Prefix-SID 2 / Adj-SID 2 / LAN-Adj-SID 3 / SID-Label 1 / Ext-Prefix-Range 2); IPv6 uses the RFC 8666 codes (4 / 5 / 6 / 7 / 9).
4. **Reception parse (shared validation, AF carrier):** the SR parser validates (RFC 8665 §3/§5 or RFC 8666 §3/§6 rules), records the originator's SR-Algorithm + ordered SRGB, and stores prefix-SID / adj-SID entries.
5. **Label computation (shared):** for a received Prefix-SID with index I and originator R, find R's range covering I, compute `label = range_base + (I - cumulative_prior)`; reject if I exceeds the total range size or the algorithm is one Ze does not compute; for V=1/L=1 use the absolute local label directly.
6. **Forwarding decision (shared truth table):** read the SR route table entry for the prefix (next-hop, type). If the next-hop router advertised the Prefix-SID with M set, ignore NP/E; else NP=0 -> PHP (forward as plain IP toward a directly-attached SR egress); NP=1/E=0 -> keep/swap the label; NP=1/E=1 -> Explicit NULL (IPv4 label 0, IPv6 label 2); otherwise push (ingress) or swap (transit).
7. **MPLS install (shared bus, per-AF source tag):** emit `mplsfibevents.Entry` with `Source = mplsSourceOSPFSR` (IPv4) or `mplsSourceOSPFv3SR` (IPv6): Push (FEC=prefix, OutLabels=[label], NextHop) for ingress; Swap (InLabel=local node-SID label, OutLabels=[remote label], NextHop) for transit; Pop (InLabel=local Adj-SID label) for the advertiser's adjacency forwarding. Removal mirrors LDP's idempotent per-key tracking.
8. **Adj-SID origination (shared trigger, AF carrier):** on adjacency >= 2-Way, allocate an SRLB local label, originate the AF link carrier LSA carrying the Adj-SID (and LAN-Adj-SID on broadcast), and install the pop/forward entry; on adjacency < 2-Way, withdraw the LSA and free the label.
9. **Inter-area (AF carrier):** when re-advertising a prefix across areas, include the Prefix-SID (best path in source area else backbone), NP set / E clear unless directly attached. IPv4 sets the IA-Flag on the Extended Prefix Range TLV (§4); IPv6 hooks `v6OriginateSummaries` (§8.2).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| SR config <-> engine | YANG SR leaves -> resolved per-AF SR config (SRGB/SRLB ranges, prefix-SID index) | [ ] |
| SR <-> IPv4 RI LSA (ext-3) | registered emitter/parser for RI top-level TLVs 8/9/14/15; value-typed TLV bodies | [ ] |
| SR <-> IPv4 Extended Prefix/Link (ext-4) | registered emitter/parser for Prefix-SID (2) and Adj-SID/LAN-Adj-SID (2/3) | [ ] |
| SR <-> IPv6 RI/Extended LSA (v3 codec) | new typed bodies on the existing `LSA` union; verbatim `RawBytes` passthrough for unknown TLVs | [ ] |
| SR <-> TLV builder/iterator | SR TLV bytes written/read via the carrier's 4-octet-aligned helpers (buffer-first, zero-copy) | [ ] |
| SR <-> shared SPF (read-only) | `Computer.Routes()` for prefix next-hop/type; the graph for adjacency neighbour IDs; a post-run hook | [ ] |
| SR <-> mpls-fib bus | `EntryChange.Emit(EntryBatch)` with the per-AF Source tag; fib-kernel programs netlink | [ ] |
| SR <-> adjacency state | allocate/free SRLB label + re-originate link carrier LSA on 2-Way up/down | [ ] |
| SR allocator <-> label space | bounded 20-bit SRGB/SRLB allocator (LDP pool pattern); ranges non-overlapping | [ ] |

### Integration Points
- The SR consumer (codec, SRGB/SRLB management, label computation, forwarding install, snapshot) inside the OSPF engine -- shared control logic plus the per-AF codecs.
- IPv4: ext-3 RI LSA originator/decoder + ext-4 Extended Prefix/Link originators/decoders -- register SR TLV emitters/parsers (no SR spelling in the carriers); ext-1 opaque carrier -- the TLV iterator/builder + scope-correct flooding.
- IPv6: `internal/plugins/ospf/v3/types` (new RI/Extended LS-Type constants + `Known()`), `internal/plugins/ospf/v3/packet` (new RI + Extended typed bodies + the SR TLV codecs, reusing the IPv6 `Prefix` codec + `LSAIterator`), `internal/plugins/ospf/lsdb` (`OriginateSelf` reuse; existing scope routing stores the new types).
- Shared: `internal/plugins/ospf/spf` (READ ONLY: route table + adjacency graph for next-hops/neighbours; a post-run hook); `internal/core/mplsfib` (the forwarding-install bus, two new Source tags); the OSPF engine (`register.go`, `config.go`) for SR config resolve, metrics, snapshot dispatch, adjacency-state subscription, lifecycle.
- `internal/plugins/ldp` -- pattern source for the label allocator and push/pop install (NOT a dependency; the pattern is re-expressed in OSPF).

### Architectural Verification
- [ ] No bypassed layers (SR TLVs flow through the AF carriers; forwarding flows through `mpls-fib` -> fib-kernel; SR never floods or programs netlink directly)
- [ ] No unintended coupling (the carriers and OSPF core name nothing SR; SR depends on them, not vice-versa; the v4 and v6 SR paths share only the AF-neutral control logic)
- [ ] No duplicated functionality (reuses the TLV builders, the RI/Extended carriers, the shared SPF route table, the LDP label-pool pattern, the mpls-fib bus; SR control logic is written once; only the per-AF codec differs)
- [ ] Zero-copy preserved (TLV bodies are views; SID/Label field written into caller buffers; mpls-fib entries value-typed with an owned label slice; IPv6 unknown TLVs reflooded verbatim)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | SR's control logic (SRGB/SRLB management, index->label, NP/E/M, mpls-fib install) is AF-neutral and shared; only the TLV codec + LSA carriage are AF-specific | `plan/learned/972-ospf-af-unify.md`; the shared `spf.Computer`/`RouteEntry` and the per-AF `_v6` strategy split | SR control logic must fork per AF, doubling the error-prone arithmetic | `TestSRLabelFromIndexMultiRange` shared across both AFs; grep shows one label-compute implementation | unvalidated |
| A-2 | ext-3 exposes a registration seam for IPv4 RI LSA top-level TLV emitters/parsers (TLVs 8/9/14/15) without editing ext-3 | dependency `spec-ospf-ext-3-router-information.md`; the ext-1 consumer-registry precedent | SR must edit ext-3 directly, violating self-containment | `TestSRRegistersRITLVs` (IPv4) | unvalidated |
| A-3 | ext-4 exposes a registration seam for IPv4 Extended Prefix and Extended Link sub-TLV emitters/parsers | dependency `spec-ospf-ext-4-extended-link-prefix.md`; RFC 7684 §2.1/§3.1 | SR must edit ext-4 directly | `TestSRRegistersExtPrefixSubTLV`, `TestSRRegistersExtLinkSubTLV` (IPv4) | unvalidated |
| A-4 | The base v3 codec lacks the RI LSA and RFC 8362 Extended LSAs, so this feature must add that carriage (SR subset) | `internal/plugins/ospf/v3/packet/lsa.go` union has only the base bodies; `v3/types/lsa.go` `Known()` enumerates only base types | scope balloons/shrinks; if a hidden RI carrier exists, reuse it | grep for `RouterInfo`/`Extended`/`0x2021`/`0xA00C` in `internal/plugins/ospf/v3` returns nothing today | unvalidated |
| A-5 | The `LSA` union + verbatim `RawBytes` passthrough extends cleanly to new typed bodies without rebuilding the v3 codec | `lsa.go` `hasTypedBody`/`WriteTo`/`bodyLen` switch on a body union; `WriteTo` re-emits `RawBytes` | a new codec layer is needed | `TestOSPFv3RILSARoundTrip`, `TestOSPFv3ERouterLSARoundTrip` (decode->re-encode byte-for-byte) | unvalidated |
| A-6 | The new IPv6 RI/Extended LS types route to the correct LSDB store purely by their scope bits, with no new store | `v3/types/lsa.go` `Scope()` from `lsTypeScopeMask`; the LSDB routes by scope | a new store/key is needed | `TestOSPFv3SRLSAScopeRouting` (RI area/AS, E-Link link-local, E-AS-External AS) | unvalidated |
| A-7 | RI/Extended self-origination works through `OriginateSelf` + a `SelfLSAEncoder` with no new sequencing/flooding | `lsdb/origination.go` `OriginateSelf`; `origination_v6.go` precedent | a new origination path is needed | `TestOSPFv3OriginateRILSA`, `TestOSPFv3OriginateAdjSID`, `TestOSPFv3OriginatePrefixSID` | unvalidated |
| A-8 | The `mpls-fib` bus + the LDP label-pool pattern suffice for SR install + SRGB/SRLB allocation for both AFs (no new netlink, no new allocator type) | `mplsfib/events.go`; `ldp/fib.go` ProgramPush/Pop + 20-bit allocator | a new label install path or allocator is needed | `TestSRInstallPrefixSIDPush` (IPv4), `TestOSPFv3PrefixSIDInstallsPush` (IPv6); QEMU `mpls -ls` per AF | unvalidated |
| A-9 | The shared SPF route table (`Computer.Routes()`/`RouteEntry`) and per-next-hop neighbour identity suffice for both AFs to compute push/swap and apply NP/E rules | `spf/route.go`, `spf/computer.go`, `afstrategy_v6.go` `NextHopSource` | SR must recompute reachability or widen the SPF result | `TestSRPrefixSIDUsesSPFNextHop` (IPv4), `TestOSPFv3LabelFromSRGBIndex` (IPv6) | unvalidated |
| A-10 | Adjacency-state transitions (2-Way up/down) are observable to SR for the Adj-SID withdraw, for both AFs | `iface/ism.go`, engine adjacency-change events | SR cannot withdraw Adj-SIDs on adjacency loss; stale forwarding | `TestSRAdjSIDWithdrawnBelow2Way` (IPv4), `TestOSPFv3AdjSIDWithdrawOnDown` (IPv6) | unvalidated |
| A-11 | The SRGB is a single configured global range per node per AF (multiple ranges supported on receive but a single configured range on originate is acceptable for v1) | RFC 8665 §3.2 / RFC 8666 (multiple MAY); operational norm | originating multiple ranges is required for interop; extend config | `ospf-sr-frr` / `ospfv3-sr-frr` accept Ze's single-range SRGB | unvalidated |
| A-12 | Ze computes only Algorithm 0; a Prefix-SID for an algorithm not advertised, or Algorithm 1, is recorded but not installed (both AFs) | RFC 8665 §5 / RFC 8666 §6; strict-SPF out of scope | installing an unsupported-algorithm SID misroutes | `TestSRPrefixSIDUnknownAlgorithmIgnored` (IPv4), `TestOSPFv3PrefixSIDIgnoreRules` (IPv6) | unvalidated |
| A-13 | The IPv6 `Prefix` codec (`v3/packet/prefix.go`) is reusable for the Extended Prefix Range TLV and the Prefix-SID parent prefix carriage | `prefix.go` `((PrefixLength+31)/32)` word encoding; RFC 8666 §5 uses the same | a separate prefix codec is needed | `TestOSPFv3ExtPrefixRangeTLVRoundTrip` (default route + /64 + /128) | unvalidated |
| A-14 | SR runs on both engine instances without leaking version branches; the RFC 8665 and RFC 8666 type codes never cross | `register.go` v4 + `eng6`; the two carriers differ in type codes/carriage | accidental coupling or a shared wire struct leaks version branches | `TestOSPFv2Unaffected` + `TestOSPFv3BaseLSAsUnchanged`; grep shows no RFC-8665 code in the v3 codec and no RFC-8666 code in the IPv4 SR path | unvalidated |
| A-15 | SR LSAs are never SPF vertices for either AF: SR affects only label install, never the topology graph | OSPFv2/OSPFv3 SR is data-plane; the shared SPF builds the topology from base LSAs | SR data corrupts the SPF graph | `TestSRLSANotInSPFGraph` (covers both AFs) | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Multi-range SRGB index->label mapping computed wrong (off-by-one or wrong range order) -> traffic mislabelled and blackholed (shared bug across AFs) | a computed label disagrees with FRR for an index spanning two ranges | dedicated `TestSRLabelFromIndexMultiRange` / `TestOSPFv3SRGBMultiRangeIndex` pinning the cumulative-offset arithmetic; one shared implementation; interop label cross-check against FRR for each AF |
| R-2 | V/L flag mishandling -> a 3-octet local label parsed as a 4-octet index (or vice-versa), shifting every subsequent byte | the SID field length disagrees with the parsed form | strict validation: only V=0/L=0 (4-octet) and V=1/L=1 (3-octet) accepted; `TestSRSIDFieldVL` (IPv4) / `TestOSPFv3SIDWidthFromVL` (IPv6) cover both forms + every invalid combination |
| R-3 | NP/E/M flag misapplication -> wrong PHP/Explicit-NULL behaviour; the SR egress drops | FRR egress logs a label mismatch / the ping over the SR LSP fails at the last hop | implement the NP/E/M truth table explicitly once (M overrides NP/E; NP=0 PHP; NP=1/E=0 keep; NP=1/E=1 Explicit NULL: IPv4 label 0, IPv6 label 2); `TestSRPHPBehavior` / `TestOSPFv3LabelOpFromFlags` per combination |
| R-4 | Adj-SID not withdrawn on adjacency loss -> a stale pop entry forwards to a dead neighbour | a removed adjacency still has an `mpls-fib` pop entry | drive Adj-SID lifecycle off adjacency state; free the SRLB label + MaxAge-flush the link carrier LSA + remove the pop entry; `TestSRAdjSIDWithdrawnBelow2Way` / `TestOSPFv3AdjSIDWithdrawOnDown` |
| R-5 | SRGB/SRLB ranges overlap (own or with LDP/RSVP-TE label space) -> a label is double-claimed | config validation passes but two SIDs map to one label | YANG + resolve-time validation: SRGB/SRLB MUST NOT overlap, Range Size > 0; a doctor check for overlap with LDP/RSVP-TE; `TestSRGBSRLBNoOverlap` / `TestOSPFv3SRGBExhaustion` |
| R-6 | A malformed SR TLV/sub-TLV crashes the parser (untrusted flooded input) | fuzz crash on an SR LSA body | the SR parser is bound-checked over the carrier iterator's views; a malformed length makes the LSA malformed; extend the OSPF + v3 packet fuzz targets with SR bodies; `TestSRParserMalformed` / `TestOSPFv3SRTLVMalformed` |
| R-7 | Duplicate / invalid-algorithm / multiple Prefix-SID honoured instead of ignored -> a wrong label installed | two installed labels for one prefix | dedupe on (prefix, topology, algorithm); install none if more than one; enforce the algorithm-not-advertised ignore; `TestSRDuplicatePrefixSIDIgnored` / `TestOSPFv3PrefixSIDIgnoreRules` |
| R-8 | SR install races the IP-route install (SR push emitted before the underlying IP route exists) -> fib-kernel rejects the encap | a push entry appears with no parent IP route | order SR recompute AFTER the shared SPF Installer's `Apply` (the post-run hook fires after install); fib-kernel reasserts on the next route event; `TestSRInstallOrderingAfterRoute` |
| R-9 | The SRGB advertised order changes across re-origination/restart -> peers recompute different labels | a peer's label for an index flaps after Ze re-originates | originate ranges in a fixed, config-declared order; a stable iteration; `TestSRGBOrderStableAcrossReorigination` |
| R-10 | Wrong IPv6 type codes (RFC 8665 values reused instead of the RFC 8666 OSPFv3 Extended-LSA registry values) | FRR `ospf6d` rejects Ze's SR TLVs or mis-parses them | pin the OSPFv3 codes (Prefix-SID 4, Adj-SID 5, LAN-Adj-SID 6, SID/Label 7, Ext-Prefix-Range 9; RI `0xA00C`, E-Router `0x2021`); `TestOSPFv3SRTypeCodes` asserts each against the RFC; `ospf6d` interop |
| R-11 | Adding new LS types to the IPv6 `Known()` changes flooding/store behaviour for non-SR Extended LSAs and breaks an existing v3 test | an existing v3 flooding/origination test fails | the new types are additive and scope-routed; with SR off nothing originates; run the full v3 suite after the type addition; `TestOSPFv3BaseLSAsUnchanged` |
| R-12 | The MPLS data plane is Linux-only; SR install cannot be unit-tested end-to-end | install logic looks right but never programs a real kernel | QEMU integration test (`ai/rules/qemu-testing.md`) asserts `mpls -ls` shows the SR push/swap/pop entries per AF; the install decision is unit-tested behind a `mpls-fib` fake |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test | Address family |
|-------------|---|--------------|------|----------------|
| `ospf { segment-routing { enable; srgb ... } }` resolves with a node prefix-SID index | -> | SR registers RI + Extended-Prefix TLV emitters; on origination the RI LSA carries SR-Algorithm/SRGB and the Extended Prefix Opaque LSA carries the Prefix-SID | `TestSROriginatesRIAndPrefixSID` + `test/ospf/ospf-sr-originate.ci` | IPv4 |
| `ospf { address-family { ipv6 { segment-routing { enable; srgb 16000 8000 } } } }` resolves | -> | v6 SR config enabled on `eng6`; `v6OriginateSelf` originates the RI LSA (SR-Algorithm+SRGB+SRLB) + E-Router-LSA (Adj-SIDs) + Prefix-SID E-prefix LSA | `TestOSPFv3SRConfigEnables` + `test/ospfv3/ospfv3-sr-config.ci` + `test/ospfv3/ospfv3-sr-originate.ci` | IPv6 |
| An RI + prefix LSA carrying SR TLVs arrives from a peer for a reachable prefix | -> | SR parser records the SRGB, computes the label, emits an `mpls-fib` push toward the SPF next-hop | `TestSRReceivesAndInstallsPrefixSID` + `test/ospf/ospf-sr-receive.ci`; `TestOSPFv3PrefixSIDInstallsPush` + `test/ospfv3/ospfv3-sr-receive.ci` | both |
| An adjacency reaches 2-Way | -> | SR allocates an SRLB label, originates the AF link carrier LSA with an Adj-SID, installs the pop entry | `test/ospf/ospf-sr-adj.ci` (IPv4); `TestOSPFv3OriginateAdjSID` (IPv6) | both |
| An adjacency drops below 2-Way | -> | SR withdraws the Adj-SID LSA, frees the label, removes the pop entry | `TestSRAdjSIDWithdrawnBelow2Way` (IPv4); `TestOSPFv3AdjSIDWithdrawOnDown` (IPv6) | both |
| `show ospf segment-routing` / `show ospf ipv6 segment-routing` is run | -> | the per-AF SR snapshot dispatch returns SRGB/SRLB, prefix-SIDs, adj-SIDs, computed labels | `test/ospf/ospf-sr-show.ci`; `test/ospfv3/ospfv3-sr-show.ci` | both |

## Acceptance Criteria

Unless an Address-family column narrows it, an AC applies to both AFs (the wire
carriage differs; the observable behaviour is the same).

| AC ID | Input / Condition | Expected Behavior | Address family |
|-------|-------------------|-------------------|----------------|
| AC-1 | SR enabled with an SRGB range | the RI LSA carries an SR-Algorithm TLV including Algorithm 0 and one SID/Label Range TLV (Range Size > 0) with exactly one SID/Label Sub-TLV; area-scoped | both (IPv4 Opaque Type 4; IPv6 RI `0xA00C`) |
| AC-2 | SR enabled with an SRLB range | the RI LSA carries an SRLB TLV (Range Size > 0, one SID/Label Sub-TLV); SRLB and SRGB MUST NOT overlap | both |
| AC-3 | SR enabled with a node prefix-SID index for a loopback | a prefix LSA carries a Prefix-SID Sub-TLV (index, V=0/L=0, NP per directly-attached rule, Algorithm 0) for the host loopback prefix; FRR resolves it to label SRGB_base+index | both (IPv4 Extended Prefix TLV, N-Flag set on the host prefix; IPv6 sub-TLV type 4 under the E-prefix carriage) |
| AC-4 | A received SID/Label Range TLV with more than one SID/Label Sub-TLV | the range TLV is ignored | both |
| AC-5 | A received RI LSA with multiple SRGB ranges | the receiver builds its SRGB as the ordered concatenation of the ranges; index N beyond range 0 maps into range 1; overlapping ranges are not double-mapped | both |
| AC-6 | A received Prefix-SID with index I, originator R whose SRGB is ranges in advertised order | the computed label is `range_base + (I - cumulative_prior)` for the range covering I; an index beyond the total range size is rejected and no label installed | both |
| AC-7 | A received Prefix-SID with V=1/L=1 vs V=0/L=0 | V=1/L=1 reads a 3-octet absolute 20-bit local label; V=0/L=0 reads a 4-octet index; any other V/L combination causes the SID Advertisement to be ignored | both (IPv4 §5; IPv6 §3.1/§6) |
| AC-8 | A received Prefix-SID for a reachable prefix whose next-hop router advertises SR | an `mpls-fib` push (ingress) or swap (transit) entry is emitted toward the SPF next-hop with the computed label and the per-AF `Source` tag | both |
| AC-9 | A received Prefix-SID, next-hop router flags NP=0 | the penultimate hop pops: SR forwards as plain IP toward a directly-attached SR egress (no push); NP=1/E=0 keeps the label; NP=1/E=1 uses Explicit NULL (IPv4 label 0, IPv6 label 2); M set ignores NP/E | both |
| AC-10 | A received Prefix-SID whose algorithm is not in the originator's SR-Algorithm TLV, or Algorithm 1 (which Ze does not compute) | the Prefix-SID is recorded but NOT installed | both |
| AC-11 | A router advertises multiple Prefix-SIDs for the same prefix/topology/algorithm | all of them are ignored and a metric is incremented | both |
| AC-12 | An adjacency reaches state 2-Way or higher on a P2P link | an Adj-SID Sub-TLV is advertised in the AF link carrier LSA, allocated from the SRLB; a pop/forward entry is installed toward that neighbour | both (IPv4 Extended Link sub-TLV 2; IPv6 E-Router Router-Link sub-TLV 5) |
| AC-13 | A P2P adjacency transitions below 2-Way | the Adj-SID Advertisement is withdrawn (the link carrier LSA re-originated without it), the SRLB label freed, and the pop entry removed | both (IPv4 §7.4.1; IPv6 §8.4.1) |
| AC-14 | A broadcast/NBMA adjacency to a non-DR neighbour | a LAN-Adj-SID Sub-TLV carrying the Neighbor ID is advertised | both (IPv4 sub-TLV 3; IPv6 sub-TLV 6) |
| AC-15 | An ABR re-advertises a prefix between areas | the Inter-Area Prefix-SID is included (best path in source/backbone area), NP set / E clear unless directly attached | both (IPv4 sets the IA-Flag on the Extended Prefix Range TLV; IPv6 §8.2) |
| AC-16 | A malformed SR TLV/sub-TLV (bad length, truncated SID field) | the LSA is treated as malformed and not installed; the parser does not panic; reception is counted | both |
| AC-17 | `show ospf segment-routing` / `show ospf ipv6 segment-routing` | renders the configured SRGB/SRLB, this node's prefix-SIDs and adj-SIDs, and per-remote computed labels with their forwarding action (push/swap/pop) | both |
| AC-18 | A received IPv6 RI / Extended LSA carrying an unknown (non-SR) TLV | the LSA is stored and reflooded byte-for-byte; the unknown TLV is not interpreted and does not block the SR TLVs in the same LSA | IPv6 |
| AC-19 | An Extended Prefix Range TLV (IPv6 AF=1) with a starting Prefix-SID and Range Size N | the N covered prefixes receive consecutive SIDs (start, start+1, ...); the IPv6 prefix decodes with `((PrefixLength+31)/32)` words; a duplicate Range TLV resolves to the smallest Instance ID | IPv6 (IPv4 Range covered by AC-15) |
| AC-20 | SR disabled (config removed) | no RI/Extended SR LSA is originated; existing SR-learned `mpls-fib` entries are withdrawn; the base OSPF behaviour for both AFs is exactly as before SR; the IPv6 type additions are inert with SR off | both |
| AC-21 | Configured SRGB or SRLB overlaps another range or the LDP/RSVP-TE label space | config validation rejects it; `ze doctor` reports the overlap before runtime install | both |
| AC-22 | Any RI / Extended SR LSA in any store | it never appears as a vertex in the SPF graph and never changes the route table directly (SR is data-plane only) | both |
| AC-23 | The IPv6 RFC 8666 type codes | are the OSPFv3 Extended-LSA registry values (Prefix-SID 4, Adj-SID 5, LAN-Adj-SID 6, SID/Label 7, Ext-Prefix-Range 9; RI `0xA00C`), NOT the OSPFv2 RFC 8665 numbers | IPv6 |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables IPv4 SR on a Ze node (SRGB, node prefix-SID) and a peer's loopback becomes reachable | SR config -> RI + Ext-Prefix emitters (ext-3/ext-4) -> origination; peer's Ext-Prefix LSA received -> label computed -> `mpls-fib` push -> fib-kernel programs it; `show mpls forwarding` lists it | `test/ospf/ospf-sr-originate.ci` + `test/ospf/ospf-sr-receive.ci` + `ospf-sr-frr` interop |
| 2 | Enables IPv6 SR (SRGB + loopback prefix-SID) and a peer's loopback becomes reachable | config -> `eng6` SR enabled -> `v6OriginateSelf` -> RI LSA + Prefix-SID E-prefix LSA -> flood; peer's E-prefix LSA received -> label computed -> `mpls-fib` push; `mpls -ls` shows it | `test/ospfv3/ospfv3-sr-originate.ci` + `test/ospfv3/ospfv3-sr-receive.ci` + `ospfv3-sr-frr` interop |
| 3 | Pings a remote SR loopback over the SR LSP (label-switched), either AF | SR push at ingress -> transit swap -> PHP/Explicit-NULL at the egress per the remote's NP/E flags -> packet delivered | `ospf-sr-frr` (IPv4) + `ospfv3-sr-frr` (IPv6) interop (label-switched reachability + NP/E behaviour) |
| 4 | Brings an SR adjacency up then down, either AF | adjacency 2-Way -> SRLB label allocated -> link carrier LSA with Adj-SID -> pop entry; adjacency down -> Adj-SID withdrawn, label freed, pop removed | `test/ospf/ospf-sr-adj.ci` (IPv4); `ospfv3-sr-frr` Adj-SID exchange (IPv6) |
| 5 | Inspects SR state via CLI, either AF | `show ospf segment-routing` / `show ospf ipv6 segment-routing` -> SR snapshot (SRGB/SRLB, prefix-SIDs, adj-SIDs, computed labels + actions) | `test/ospf/ospf-sr-show.ci`; `test/ospfv3/ospfv3-sr-show.ci` |
| 6 | Runs Ze SR against FRR ospfd (IPv4) and ospf6d (IPv6) with SR enabled | DD/flood exchange; both originate SR TLVs; Ze installs FRR's prefix-SIDs and FRR installs Ze's; labels agree for a multi-range SRGB | `ospf-sr-frr` + `ospfv3-sr-frr` interop |
| 7 | Disables SR (config removed) | the SR TLV emitters/parsers stop originating; SR `mpls-fib` entries are withdrawn; OSPF + the carriers behave as before; the IPv6 type additions are inert | `test/ospfv3/ospfv3-sr-disable.ci` + `TestOSPFBuildsWithoutSR` + existing OSPF suites green |

## 🧪 TDD Test Plan

### Unit Tests

Shared SR control logic (both AFs):
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSRLabelFromIndexSingleRange` / `TestSRLabelFromIndexMultiRange` | `internal/plugins/ospf/sr/srgb_test.go` | AC-6, R-1: index->label across one and multiple ordered ranges; cumulative offset | |
| `TestSRLabelIndexOutOfRange` | `internal/plugins/ospf/sr/srgb_test.go` | AC-6: index beyond total range size rejected, no install | |
| `TestSRGBOrderStableAcrossReorigination` | `internal/plugins/ospf/sr/srgb_test.go` | R-9: advertised range order stable | |
| `TestSRGBSRLBNoOverlap` | `internal/plugins/ospf/sr/config_test.go` | AC-2/AC-21, R-5: SRGB/SRLB non-overlap, Range Size > 0 validation | |
| `TestSRLBAllocatorBounds` / `TestSRLBAllocatorExhaustion` | `internal/plugins/ospf/sr/srlb_test.go` | A-8: bounded SRLB allocator within range; exhaustion handled | |
| `TestSRPHPBehavior` | `internal/plugins/ospf/sr/install_test.go` | AC-9, R-3: NP/E/M truth table (PHP, keep, Explicit NULL, M override) shared, AF-parameterised NULL label | |
| `TestSRPrefixSIDUnknownAlgorithmIgnored` | `internal/plugins/ospf/sr/install_test.go` | AC-10, A-12: algorithm-not-advertised / Algorithm 1 recorded, not installed | |
| `TestSRDuplicatePrefixSIDIgnored` | `internal/plugins/ospf/sr/install_test.go` | AC-11, R-7: multiple Prefix-SIDs same prefix/topology/algorithm all ignored | |
| `TestSRInstallOrderingAfterRoute` | `internal/plugins/ospf/sr/install_test.go` | R-8: SR push emitted after the IP route exists | |
| `TestSRLSANotInSPFGraph` | `internal/plugins/ospf/spf/spf_test.go` | AC-22, A-15: RI/Extended SR LSAs never become SPF vertices (both AFs) | |
| `TestOSPFBuildsWithoutSR` | `internal/plugins/ospf/sr/register_test.go` | self-containment: removing SR leaves OSPF + carriers intact | |

IPv4 (RFC 8665) wire + plumbing:
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSIDLabelSubTLVRoundTrip` | `internal/plugins/ospf/sr/codec_test.go` | AC-7: SID/Label Sub-TLV 3-octet label vs 4-octet index (§2.1) | |
| `TestSRAlgorithmTLVRoundTrip` | `internal/plugins/ospf/sr/codec_test.go` | AC-1: SR-Algorithm TLV includes Algorithm 0 (§3.1) | |
| `TestSRGBRangeTLVRoundTrip` / `TestSRLBTLVRoundTrip` / `TestSRMSPrefTLVRoundTrip` | `internal/plugins/ospf/sr/codec_test.go` | AC-1/AC-2/AC-4: SRGB/SRLB/SRMS TLVs; exactly one SID/Label Sub-TLV; Range Size > 0 | |
| `TestPrefixSIDSubTLVRoundTrip` / `TestSRSIDFieldVL` | `internal/plugins/ospf/sr/codec_test.go` | AC-7, R-2: Prefix-SID flags + V/L sizing; every invalid V/L combination ignored (§5) | |
| `TestAdjSIDSubTLVRoundTrip` / `TestLANAdjSIDSubTLVRoundTrip` | `internal/plugins/ospf/sr/codec_test.go` | AC-12/AC-14: Adj-SID + LAN-Adj-SID flags/weight/neighbour (§6.1, §6.2) | |
| `TestExtPrefixRangeTLVRoundTrip` | `internal/plugins/ospf/sr/codec_test.go` | AC-15: Extended Prefix Range TLV; IA-Flag (§4) | |
| `TestSRParserMalformed` | `internal/plugins/ospf/sr/codec_test.go` | AC-16, R-6: malformed TLV/sub-TLV never panics; LSA marked malformed (§9) | |
| `TestSRRegistersRITLVs` / `TestSRRegistersExtPrefixSubTLV` / `TestSRRegistersExtLinkSubTLV` | `internal/plugins/ospf/sr/register_test.go` | A-2/A-3: SR emitters/parsers registered with ext-3/ext-4 | |
| `TestSROriginatesRIAndPrefixSID` / `TestSRTLVsAreaScoped` | `internal/plugins/ospf/sr/origination_test.go` | AC-1/AC-3, A-? : RI + Ext-Prefix originate SR TLVs; area-scoped (§3.1) | |
| `TestSRPrefixSIDUsesSPFNextHop` / `TestSRInstallPrefixSIDPush` / `TestSRInstallPrefixSIDSwap` | `internal/plugins/ospf/sr/install_test.go` | AC-8, A-9: push/swap toward the SPF next-hop with `Source=mplsSourceOSPFSR` | |
| `TestSRNoInstallForNonSRNextHop` | `internal/plugins/ospf/sr/install_test.go` | R-? : no label installed toward a non-SR next-hop | |
| `TestSRAdjSIDForwardsToNeighbor` / `TestSRAdjSIDWithdrawnBelow2Way` | `internal/plugins/ospf/sr/adjsid_test.go` | AC-12/AC-13, R-4: Adj-SID pop to the neighbour; withdrawn + freed below 2-Way (§7.4.1) | |
| `TestSRSnapshot` | `internal/plugins/ospf/sr/snapshot_test.go` | AC-17: `show ospf segment-routing` snapshot rows | |

IPv6 (RFC 8666) wire carriage + plumbing:
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOSPFv3SRTypeCodes` | `internal/plugins/ospf/v3/packet/sr_tlv_test.go` | AC-23, R-10: RFC 8666 OSPFv3 codes (Prefix-SID 4, Adj-SID 5, LAN-Adj-SID 6, SID/Label 7, Ext-Prefix-Range 9; RI `0xA00C`, E-Router `0x2021`) | |
| `TestOSPFv3SIDWidthFromVL` | `internal/plugins/ospf/v3/packet/sr_tlv_test.go` | AC-7, R-2: SID width 4 (V=0/L=0) vs 3 (V=1/L=1); invalid V/L ignored | |
| `TestOSPFv3SRTLVMalformed` | `internal/plugins/ospf/v3/packet/sr_tlv_test.go` | AC-16, R-6: truncated/over-length TLV never panics, marks LSA malformed | |
| `TestOSPFv3RILSARoundTrip` | `internal/plugins/ospf/v3/packet/lsa_routerinfo_test.go` | A-5/AC-1: RI LSA (SR-Algorithm+SRGB+SRLB) decode->re-encode byte-for-byte | |
| `TestOSPFv3ERouterLSARoundTrip` | `internal/plugins/ospf/v3/packet/lsa_extended_test.go` | A-5/AC-12: E-Router-LSA with Adj-SID/LAN-Adj-SID round-trips | |
| `TestOSPFv3ExtPrefixRangeTLVRoundTrip` | `internal/plugins/ospf/v3/packet/sr_tlv_test.go` | A-13/AC-19: Extended Prefix Range TLV (default/64/128) IPv6 word encoding; smallest Instance ID on duplicate | |
| `TestOSPFv3PrefixSIDCodec` | `internal/plugins/ospf/v3/packet/sr_tlv_test.go` | AC-3: Prefix-SID sub-TLV flags/algorithm/SID encode+decode | |
| `TestOSPFv3UnknownRITLVReflooded` | `internal/plugins/ospf/v3/packet/lsa_routerinfo_test.go` | AC-18: unknown RI/Extended TLV stored + reflooded verbatim, not interpreted | |
| `TestOSPFv3SRLSAScopeRouting` | `internal/plugins/ospf/lsdb/lsdb_linkscope_test.go` | A-6/AC-22: RI area/AS, E-Link link-local, E-AS-External AS route to the right store | |
| `TestOSPFv3SRGBMultiRangeIndex` / `TestOSPFv3LabelFromSRGBIndex` / `TestOSPFv3LabelOpFromFlags` | `internal/plugins/ospf/sr_v6_label_test.go` | R-1/R-4/AC-5/AC-9: v6 index resolution + the NP/E/M decision (Explicit-NULL label 2) | |
| `TestOSPFv3PrefixSIDIgnoreRules` | `internal/plugins/ospf/sr_v6_recv_test.go` | R-7/AC-10/AC-11: algorithm-not-advertised, multiple-same-prefix, invalid-V/L ignored | |
| `TestOSPFv3PrefixSIDInstallsPush` / `TestOSPFv3AdjSIDInstallsPop` | `internal/plugins/ospf/sr_v6_install_test.go` | A-8/AC-8/AC-12: `mpls-fib` Push/Pop with the right InLabel/OutLabels/NextHop/`Source=mplsSourceOSPFv3SR` | |
| `TestOSPFv3OriginateRILSA` / `TestOSPFv3OriginatePrefixSID` / `TestOSPFv3OriginateAdjSID` / `TestOSPFv3AdjSIDWithdrawOnDown` | `internal/plugins/ospf/sr_v6_originate_test.go` | A-7/AC-1/AC-3/AC-12/AC-13: RI/Prefix-SID/Adj-SID origination via `OriginateSelf`; withdraw on adjacency down | |
| `TestOSPFv3InterAreaPrefixSID` | `internal/plugins/ospf/sr_v6_interarea_test.go` | AC-15: §8.2 propagation, NP set / E clear unless directly attached | |
| `TestOSPFv3SRGBAllocation` / `TestOSPFv3SRGBExhaustion` | `internal/plugins/ospf/sr_v6_alloc_test.go` | A-8/R-5/AC-21: bounded allocation; exhaustion + overlap handling | |
| `TestOSPFv3BaseLSAsUnchanged` / `TestOSPFv2Unaffected` | `internal/plugins/ospf/v3/types/lsa_test.go`, `internal/plugins/ospf/instance_v6_test.go` | R-11/A-14/AC-20: additive type changes; v4 + base v3 behaviour unchanged | |
| `TestOSPFv3SRConfigEnables` | `internal/plugins/ospf/config_test.go` | wiring/AC-1: `segment-routing` config resolves and enables SR on `eng6` | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| MPLS label (SID/Label, 3-octet form) | 16-1048575 (20-bit) | 1048575 | 0-15 reserved (IPv4 NULL 0, IPv6 NULL 2) | >1048575 rejected |
| SID index (Prefix-SID, V=0/L=0) | 0-4294967295 (32-bit) | within total SRGB range size | N/A | index >= total range size rejected |
| Range Size (SRGB/SRLB) | 1-16777215 (24-bit) | 16777215 | 0 invalid (MUST be > 0) | >2^24 rejected |
| Range Size (IPv4 Extended Prefix Range, 2-octet) | 1-65535 | bounded by Prefix Length capacity excl. 224.0.0.0/3 | 0 | exceeding capacity rejected |
| Range Size (IPv6 Extended Prefix Range) | 1-(prefixes satisfiable by PrefixLength) | per PrefixLength | 0 | exceeds capacity (malformed) |
| SID/Label sub-TLV value length | {3,4} octets | 4 | 2 (rejected) | 5 (rejected) |
| SR-Algorithm value | 0-255 | 255 | N/A | Algorithm 0 MUST be present when TLV advertised |
| Prefix-SID Length (V-Flag) | 7 or 8 (IPv4 sub-TLV) | 8 | N/A | other lengths malformed |
| Adj-SID Length (V-Flag) | 7 or 8 (IPv4 sub-TLV) | 8 | N/A | other lengths malformed |
| LAN-Adj-SID Length (V-Flag) | 11 or 12 (IPv4 sub-TLV) | 12 | N/A | other lengths malformed |
| IPv6 PrefixLength (Ext-Prefix-Range) | 0-128 | 128 | N/A | 129 |
| Prefix-SID Algorithm | 0-255 (only advertised values honoured) | 255 | N/A | unadvertised -> ignored |
| Adj-SID Weight | 0-255 | 255 | N/A | N/A (1 byte) |
| SRMS Preference | 0-255 | 255 | N/A | N/A (1 octet) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-sr-originate` | `test/ospf/ospf-sr-originate.ci` | IPv4 SR enabled; RI LSA shows SR-Algorithm/SRGB, Ext-Prefix shows the node prefix-SID | |
| `ospf-sr-receive` | `test/ospf/ospf-sr-receive.ci` | a received IPv4 prefix-SID computes a label and installs an `mpls-fib` push; `show mpls forwarding` lists it | |
| `ospf-sr-adj` | `test/ospf/ospf-sr-adj.ci` | an IPv4 adjacency advertises an Adj-SID; dropping it withdraws the Adj-SID and removes the pop entry | |
| `ospf-sr-show` | `test/ospf/ospf-sr-show.ci` | `show ospf segment-routing` renders SRGB/SRLB, prefix-SIDs, adj-SIDs, computed labels | |
| `ospf-sr-php` | `test/ospf/ospf-sr-php.ci` | NP/E flags drive PHP / keep / Explicit-NULL on the installed IPv4 label | |
| `ospfv3-sr-config` | `test/ospfv3/ospfv3-sr-config.ci` | IPv6 `segment-routing` config validates; SR appears in `show ospf ipv6` | |
| `ospfv3-sr-originate` | `test/ospfv3/ospfv3-sr-originate.ci` | IPv6 RI LSA + Prefix-SID + Adj-SID originated; visible in `show ospf ipv6 database` | |
| `ospfv3-sr-receive` | `test/ospfv3/ospfv3-sr-receive.ci` | a received IPv6 Prefix-SID installs an `mpls-fib` entry; `show ospf ipv6 segment-routing` lists the computed label | |
| `ospfv3-sr-show` | `test/ospfv3/ospfv3-sr-show.ci` | `show ospf ipv6 segment-routing` renders SR-Algorithm, SRGB/SRLB, prefix-SIDs, adj-SIDs | |
| `ospfv3-sr-disable` | `test/ospfv3/ospfv3-sr-disable.ci` | removing IPv6 SR config withdraws the SR LSAs + `mpls-fib` entries; base OSPFv3 + the v4 engine unaffected | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-sr-frr` | `test/interop/scenarios/ospf-sr-frr/` | FRR `ospfd` with `segment-routing on` (SRGB/SRLB, prefix-SIDs, adj-SIDs) | Ze and FRR exchange RFC 8665 SR TLVs, agree on index->label for a multi-range SRGB, install matching MPLS forwarding, and a label-switched ping over the SR LSP succeeds with correct NP/E (PHP / Explicit-NULL) behaviour | |
| `ospfv3-sr-frr` | `test/interop/scenarios/ospfv3-sr-frr/` | FRR `ospf6d` with SR-MPLS enabled | Ze originates valid RFC 8666 RI + Extended SR LSAs FRR accepts; Ze parses FRR's SR LSAs, computes the same labels, and both program matching MPLS forwarding for an end-to-end labelled IPv6 path | |

> Interop is required for both AFs: this changes wire behaviour (new RI + Extended
> TLVs for each AF) and programs the MPLS data plane. The raw-IP/IPv6, multicast,
> and AF_MPLS paths are Linux-only and run as QEMU integration tests
> (`ai/rules/qemu-testing.md`); the MPLS forwarding assertion uses `mpls -ls`
> inside the QEMU guest.

### Future (if deferring any tests)
- None. Every AC maps to a unit, functional, or interop test above. TI-LFA backup-path tests, strict-SPF (Algorithm 1) path-install tests, the SR mapping-server server role, and SRv6 are out of scope (separate specs / deferred), not deferred-but-in-scope.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*) -->

Shared (both AFs):
- `internal/plugins/ospf/register.go` -- wire the SR consumer: metrics, the `show ospf segment-routing` and `show ospf ipv6 segment-routing` snapshot dispatch keys, the SR post-SPF-run hook, the adjacency-change subscription for Adj-SID lifecycle; start/stop IPv6 SR on `eng6` from `cfg.V6.SegmentRouting`
- `internal/plugins/ospf/config.go` -- resolve the IPv4 SR YANG leaves into the engine SR config and the IPv6 `segment-routing` block into `cfg.V6` (enable, SRGB/SRLB ranges, node prefix-SID index, flags)
- `internal/plugins/ospf/spf/computer.go` -- expose a read-only post-run SR hook (a sibling to `SetOnChange`) firing AFTER the Installer `Apply` (R-8) so SR pushes ride existing IP routes (shared by both AFs)
- `internal/plugins/ospf/doctor.go` -- a doctor check for SR config sanity (SRGB/SRLB present + non-overlapping with each other and the LDP/RSVP-TE ranges when SR enabled; MPLS forwarding available)
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- the `segment-routing` container for the IPv4 family and under `address-family ipv6` (enable, srgb, srlb, prefix-sid index, node flag)
- `internal/plugins/ospf/yang/ze-ospf-cmd.yang` -- the `show ospf segment-routing` and `show ospf ipv6 segment-routing` commands
- `internal/core/diagnostic/codes.go` -- a diagnostic code for the SRGB/SRLB overlap doctor check

IPv4 family carriers (registration seams; no SR spelling added):
- ext-3 RI LSA originator/decoder (per `spec-ospf-ext-3-router-information.md`) -- a registration seam for SR top-level TLV emitters/parsers
- ext-4 Extended Prefix/Link originators/decoders (per `spec-ospf-ext-4-extended-link-prefix.md`) -- a registration seam for SR sub-TLV emitters/parsers

IPv6 family carriage (added by this feature):
- `internal/plugins/ospf/v3/types/lsa.go` -- add the RI (`LSTypeRouterInfo` area `0xA00C`, AS `0xC00C`) and Extended-LSA type constants (`0x2021` E-Router, `0x2029` E-Intra-Area-Prefix, `0x2023` E-Inter-Area-Prefix, `0x4025` E-AS-External, `0x2027` E-Type-7, `0x0028` E-Link); widen `Known()`
- `internal/plugins/ospf/v3/packet/lsa.go` -- add `RouterInfo` + the Extended-LSA typed bodies to the union, the `WriteTo`/`bodyLen`/`hasTypedBody` switches, and `Decode*` accessors
- `internal/plugins/ospf/origination_v6.go` -- SR origination off `v6OriginateSelf`: RI LSA (SR capabilities), E-Router-LSA Adj-SIDs, Prefix-SID E-prefix LSA; extend `v6OriginHeader` for the new types
- `internal/plugins/ospf/afstrategy_v6.go` -- §8.2 Inter-Area Prefix-SID propagation in `OriginateSummaries`/`v6OriginateSummaries`; expose per-next-hop neighbour identity if not already
- `internal/plugins/ospf/cmd_show.go` -- `show ospf ipv6 segment-routing` backing data

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- `segment-routing` container (IPv4 family + under `address-family ipv6`); read `ai/rules/config-surface.md` + `ai/rules/config-naming.md` |
| YANG validation constraints | [ ] yes | SRGB/SRLB base `range "16..1048575"`, size `range "1..1048575"` (or 24-bit per RFC); prefix-sid index `range`; `enable`/flags `boolean` |
| YANG custom validators | [ ] yes | SRGB/SRLB non-overlap (with each other + LDP/RSVP-TE) + capacity validation (`ze:validate` + `ValidateFn`); `CompleteFn` for label-range hints; register in the OSPF validators register |
| CLI commands/flags | [ ] yes | `show ospf segment-routing` + `show ospf ipv6 segment-routing` in `ze-ospf-cmd.yang` + the register dispatch / `cmd_show.go` |
| CLI grammar (action before identifier) | [ ] yes | `ai/rules/cli-grammar.md` -- `show ospf segment-routing`, `show ospf ipv6 segment-routing` |
| Editor autocomplete | [ ] yes | automatic for the YANG enum/boolean leaves + the new show subcommands |
| Functional test for new RPC/API | [ ] yes | `test/ospf/ospf-sr-*.ci`, `test/ospfv3/ospfv3-sr-*.ci` |
| Pipe completeness | [ ] yes | both show commands route through `ApplyPipes` like the other show outputs |
| Env var registration | [ ] no | SR is operational config, not an `environment/` leaf |
| Doctor check for runtime dependencies | [ ] yes | SR programs the MPLS data plane: a doctor check that AF_MPLS forwarding is available and SRGB/SRLB are sane + non-overlapping with LDP/RSVP-TE; `internal/core/diagnostic/codes.go` + unit + functional test (`ai/rules/doctor-checks.md`) |
| Prometheus counters/metrics | [ ] yes | see the metrics rows below |

#### Metrics (new series owned by this spec)
| Metric | Type | Labels | Address family |
|--------|------|--------|----------------|
| `ze_ospf_sr_enabled` | gauge | `af` (ipv4/ipv6) | both |
| `ze_ospf_sr_prefix_sids` | gauge | `af`, `direction` (originated/received) | both |
| `ze_ospf_sr_adj_sids` | gauge | `af` | both |
| `ze_ospf_sr_labels_installed` | gauge | `af`, `op` (push/swap/pop) | both |
| `ze_ospf_sr_label_compute_errors_total` | counter | `af`, `reason` (index-out-of-range / unknown-algorithm / duplicate / bad-vl) | both |
| `ze_ospf_sr_srlb_labels_in_use` | gauge | `af` | both |
| `ze_ospf_sr_malformed_tlvs_total` | counter | `af`, `tlv` | both |

> The single `ze_ospf_sr_*` series namespace carries an `af` label to distinguish
> the two address families, mirroring how Ze treats OSPF as one engine. The series
> are registered by this spec's SR code (not by the OSPF metrics core). The OSPF
> telemetry doc gains these rows when this spec lands.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` -- OSPF Segment Routing (both AFs) |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` -- the `segment-routing` container (IPv4 + `address-family ipv6`) |
| 3 | CLI command added/changed? | [ ] yes | `docs/guide/command-reference.md` -- `show ospf segment-routing`, `show ospf ipv6 segment-routing` |
| 4 | API/RPC added/changed? | [ ] no | the show RPCs live in the central `ze-show` namespace; document under the command reference |
| 5 | Plugin added/changed? | [ ] yes | `docs/guide/plugins.md` -- OSPF gains an SR consumer (both AFs) |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/ospf.md` -- a Segment Routing section (SRGB/SRLB, prefix-SID, adj-SID, both AFs) |
| 7 | Wire format changed? | [ ] yes | `docs/architecture/wire/ospf.md` (IPv4 SR TLVs) + `docs/architecture/wire/ospfv3.md` (RI LSA + Extended LSAs + SR TLVs) |
| 8 | Plugin SDK/protocol changed? | [ ] yes | document the ext-3/ext-4 SR registration seam (IPv4) for future SR-adjacent consumers; the `mpls-fib` bus contract is unchanged (two new Source tags only) |
| 9 | RFC behavior implemented? | [ ] yes | `rfc/short/rfc8665.md` + `rfc/short/rfc8666.md` -- tick the implemented Compliance Checklist items |
| 10 | Test infrastructure changed? | [ ] yes (interop scenarios added) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` -- OSPF SR parity with FRR (both AFs) |
| 12 | Internal architecture changed? | [ ] yes | the OSPF subsystem doc + the MPLS forwarding doc -- SR as a third mpls-fib producer; the v3 RI/Extended carriage |
| 13 | Route metadata keys added/changed? | [ ] no | SR installs label entries via mpls-fib, not route metadata; confirm no meta key added |
| 14 | Prometheus counters added/changed? | [ ] yes | the OSPF telemetry doc -- the `ze_ospf_sr_*` series |
| 15 | Registered plugin/event/command/capability inventory changed? | [ ] yes | `docs/plugin-overview.md` -- SR consumer + the two show commands + the mpls-fib SR source tags |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors into the changed OSPF/v3/mpls files |
| 17 | Existing docs show examples for this area? | [ ] check | verify any OSPF/MPLS config/CLI examples against the new SR leaves |

## Files to Create

Shared SR consumer (both AFs):
- `internal/plugins/ospf/sr/srgb.go` -- SRGB representation (ordered ranges) + the index->label computation across ranges (shared)
- `internal/plugins/ospf/sr/srlb.go` -- the bounded SRLB local-label allocator (LDP `nextLabel`/`MaxLabel` pattern, seeded by the configured SRLB range) (shared)
- `internal/plugins/ospf/sr/config.go` -- the resolved SR config + SRGB/SRLB non-overlap/capacity validation (shared)
- `internal/plugins/ospf/sr/install.go` -- the forwarding install: label computation per route, the NP/E/M truth table (AF-parameterised Explicit-NULL label), `mpls-fib` push/swap/pop emission, idempotent per-key removal (LDP `pushed`-set pattern) (shared)
- `internal/plugins/ospf/sr/adjsid.go` -- the Adj-SID lifecycle driven by adjacency state (allocate at 2-Way, withdraw below 2-Way) (shared)
- `internal/plugins/ospf/sr/register.go` -- registers the SR metrics, config resolve, snapshot dispatch, and the per-AF `mplsSourceOSPFSR` / `mplsSourceOSPFv3SR` source tags; wires the IPv4 ext-3/ext-4 emitters and the IPv6 origination hooks
- `internal/plugins/ospf/sr/snapshot.go` -- the SR snapshot rows for both show commands

IPv4 wire (RFC 8665):
- `internal/plugins/ospf/sr/codec.go` -- the IPv4 SR TLV/sub-TLV codec: SID/Label Sub-TLV, SR-Algorithm, SID/Label Range (SRGB), SRLB, SRMS-Pref, Extended Prefix Range, Prefix-SID, Adj-SID, LAN-Adj-SID (encode + decode + V/L validation) using the ext-1 TLV builder/iterator
- `internal/plugins/ospf/sr/origination.go` -- the IPv4 SR emitters: RI top-level TLVs (via ext-3 seam) + Prefix-SID / Adj-SID / LAN-Adj-SID sub-TLVs (via ext-4 seam); area scoping
- `internal/plugins/ospf/sr/reception.go` -- the IPv4 SR parsers: record remote SR-Algorithm + ordered SRGB; store prefix-SID / adj-SID entries; §3/§5 validation

IPv6 wire carriage + glue (RFC 8666 / RFC 7770 / RFC 8362):
- `internal/plugins/ospf/v3/packet/lsa_routerinfo.go` -- the OSPFv3 RI Opaque LSA body + top-level TLV iterator (SR-Algorithm/SRGB/SRLB/SRMS) + verbatim passthrough for unknown RI TLVs
- `internal/plugins/ospf/v3/packet/lsa_extended.go` -- the RFC 8362 Extended-LSA bodies (E-Router-LSA Router-Link TLVs; the E-prefix LSAs' prefix TLVs) carrying SR sub-TLVs; verbatim passthrough for unknown TLVs
- `internal/plugins/ospf/v3/packet/sr_tlv.go` -- the IPv6 SR TLV/sub-TLV codecs: SID/Label, Prefix-SID, Adj-SID, LAN-Adj-SID, Extended Prefix Range; V/L width inference; bound-checked iteration; RFC 8666 type codes
- `internal/plugins/ospf/sr_v6.go` -- the v6 SR engine glue: enable/disable on `eng6`, peer-capability store, the origination trigger, the reception/install trigger off the v6 SPF result
- `internal/plugins/ospf/sr_v6_label.go` -- the IPv6 SRGB index->label resolution + the NP/E/M push/swap/PHP decision (delegating to the shared logic where AF-neutral)
- `internal/plugins/ospf/sr_v6_install.go` -- the IPv6 `mpls-fib` Push/Swap/Pop emit (`mplsSourceOSPFv3SR`)

Tests:
- IPv4: `internal/plugins/ospf/sr/codec_test.go`, `srgb_test.go`, `srlb_test.go`, `config_test.go`, `install_test.go`, `adjsid_test.go`, `register_test.go`, `origination_test.go`, `snapshot_test.go`
- IPv6: `internal/plugins/ospf/v3/packet/sr_tlv_test.go`, `lsa_routerinfo_test.go`, `lsa_extended_test.go`; `internal/plugins/ospf/sr_v6_label_test.go`, `sr_v6_recv_test.go`, `sr_v6_install_test.go`, `sr_v6_originate_test.go`, `sr_v6_interarea_test.go`, `sr_v6_alloc_test.go`
- Functional: `test/ospf/ospf-sr-originate.ci`, `ospf-sr-receive.ci`, `ospf-sr-adj.ci`, `ospf-sr-show.ci`, `ospf-sr-php.ci`; `test/ospfv3/ospfv3-sr-config.ci`, `ospfv3-sr-originate.ci`, `ospfv3-sr-receive.ci`, `ospfv3-sr-show.ci`, `ospfv3-sr-disable.ci`
- Interop: `test/interop/scenarios/ospf-sr-frr/` and `test/interop/scenarios/ospfv3-sr-frr/` -- each with `ze.conf`, `frr.conf`, `check.py`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan -- confirm the IPv4 ext-1/ext-3/ext-4 carriers + the mpls-fib bus exist and expose the needed seams; confirm NO RI/Extended LSA exists yet in the v3 codec |
| 3. Wiring phase | Wiring Test table -- SR config + register emitters/hooks + failing wiring tests for both AFs |
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

1. **Phase: Wiring (MANDATORY FIRST)** -- SR config + entry points + failing wiring tests for both AFs
   - Tests: `TestSRRegistersRITLVs`, `TestSRRegistersExtPrefixSubTLV`, `TestSRRegistersExtLinkSubTLV`, `TestOSPFBuildsWithoutSR` (IPv4); `TestOSPFv3SRConfigEnables` (IPv6); `test/ospf/ospf-sr-show.ci`, `test/ospfv3/ospfv3-sr-config.ci` (empty/enable)
   - Files: `sr/register.go` (registration + the two source tags + metrics + snapshot dispatch in `ospf/register.go`), `sr/snapshot.go` (stub), `sr/config.go`, `config.go` resolve into `cfg.V6`, the new LS-type constants in `v3/types/lsa.go` as stubs
   - Verify: SR registers with the IPv4 carriers and toggles SR state on both engines; origination/reception/install are stubs so the deeper tests still fail
2. **Phase: Shared SRGB/SRLB management + label arithmetic** -- the AF-neutral math
   - Tests: `TestSRLabelFromIndexSingleRange`, `TestSRLabelFromIndexMultiRange`, `TestSRLabelIndexOutOfRange`, `TestSRGBOrderStableAcrossReorigination`, `TestSRLBAllocatorBounds`, `TestSRLBAllocatorExhaustion`, `TestSRGBSRLBNoOverlap`
   - Files: `sr/srgb.go`, `sr/srlb.go`, `sr/config.go`
   - Verify: the multi-range arithmetic is exact; range order stable; SRLB allocation bounded; config non-overlap enforced; one implementation used by both AFs
3. **Phase: IPv4 SR codec** -- the RFC 8665 wire primitives over ext-1
   - Tests: `TestSIDLabelSubTLVRoundTrip`, `TestSRAlgorithmTLVRoundTrip`, `TestSRGBRangeTLVRoundTrip`, `TestSRLBTLVRoundTrip`, `TestSRMSPrefTLVRoundTrip`, `TestPrefixSIDSubTLVRoundTrip`, `TestSRSIDFieldVL`, `TestAdjSIDSubTLVRoundTrip`, `TestLANAdjSIDSubTLVRoundTrip`, `TestExtPrefixRangeTLVRoundTrip`, `TestSRParserMalformed`
   - Files: `sr/codec.go`
   - Verify: every IPv4 SR TLV/sub-TLV round-trips via the ext-1 builder/iterator; V/L validation holds; malformed input never panics
4. **Phase: IPv6 RI + Extended LSA carriage + SR TLV codecs** -- the RFC 8666 wire primitives
   - Tests: `TestOSPFv3SRTypeCodes`, `TestOSPFv3SIDWidthFromVL`, `TestOSPFv3SRTLVMalformed`, `TestOSPFv3RILSARoundTrip`, `TestOSPFv3ERouterLSARoundTrip`, `TestOSPFv3ExtPrefixRangeTLVRoundTrip`, `TestOSPFv3PrefixSIDCodec`, `TestOSPFv3UnknownRITLVReflooded`, `TestOSPFv3SRLSAScopeRouting`, `TestOSPFv3BaseLSAsUnchanged`
   - Files: `v3/types/lsa.go` (types + `Known()`), `v3/packet/lsa.go` (union + switches), `v3/packet/lsa_routerinfo.go`, `lsa_extended.go`, `sr_tlv.go`
   - Verify: RI/Extended LSAs round-trip; SR TLVs use the RFC 8666 codes with V/L width inference; malformed input never panics; new types scope-route; base LSAs unchanged
5. **Phase: IPv4 origination + reception + install** -- advertise/parse RFC 8665 SR, compute + install labels
   - Tests: `TestSROriginatesRIAndPrefixSID`, `TestSRTLVsAreaScoped`, `TestSRPrefixSIDUsesSPFNextHop`, `TestSRPrefixSIDUnknownAlgorithmIgnored`, `TestSRDuplicatePrefixSIDIgnored`, `TestSRNoInstallForNonSRNextHop`, `TestSRInstallPrefixSIDPush`, `TestSRInstallPrefixSIDSwap`, `TestSRPHPBehavior`, `TestSRInstallOrderingAfterRoute`, `ospf-sr-receive.ci`, `ospf-sr-php.ci`
   - Files: `sr/origination.go`, `sr/reception.go`, `sr/install.go`, `yang/ze-ospf-conf.yang`, `spf/computer.go` (post-run hook ordering)
   - Verify: RI carries SR-Algorithm/SRGB/SRLB; Ext-Prefix carries the node prefix-SID; push/swap/pop emitted with `mplsSourceOSPFSR`; NP/E/M correct (NULL=0); install after the IP route
6. **Phase: IPv6 reception + install + origination + withdraw** -- RFC 8666 over the new carriage
   - Tests: `TestOSPFv3PrefixSIDIgnoreRules`, `TestOSPFv3PrefixSIDInstallsPush`, `TestOSPFv3AdjSIDInstallsPop`, `TestOSPFv3LabelFromSRGBIndex`, `TestOSPFv3SRGBMultiRangeIndex`, `TestOSPFv3LabelOpFromFlags`, `TestOSPFv3OriginateRILSA`, `TestOSPFv3OriginatePrefixSID`, `TestOSPFv3OriginateAdjSID`, `TestOSPFv3AdjSIDWithdrawOnDown`, `TestOSPFv3SRGBAllocation`, `TestOSPFv3SRGBExhaustion`, `ospfv3-sr-receive.ci`, `ospfv3-sr-originate.ci`
   - Files: `sr_v6.go`, `sr_v6_label.go`, `sr_v6_install.go`, `sr_v6_alloc.go` (if a v6-specific shim is needed over the shared allocator), `origination_v6.go`, `register.go`
   - Verify: peer SRGB recorded; §6 ignore rules; Push/Pop emitted with `mplsSourceOSPFv3SR` (NULL=2); RI/Adj-SID/Prefix-SID originate via `OriginateSelf`; Adj-SID withdraws on adjacency down
7. **Phase: Adj-SID lifecycle (both AFs) + inter-area** -- adjacency-driven allocation/withdraw + §4/§8.2 propagation
   - Tests: `TestSRAdjSIDForwardsToNeighbor`, `TestSRAdjSIDWithdrawnBelow2Way` (IPv4), `TestOSPFv3InterAreaPrefixSID`, `TestSRLSANotInSPFGraph`, `ospf-sr-adj.ci`
   - Files: `sr/adjsid.go`, `afstrategy_v6.go` (`OriginateSummaries`), `register.go` (adjacency-change subscription)
   - Verify: Adj-SID advertised at 2-Way, withdrawn + freed below 2-Way (both AFs); LAN-Adj-SID on broadcast; inter-area Prefix-SID with NP set / E clear unless attached; SR LSAs never enter SPF
8. **Phase: CLI + metrics + doctor + disable** -- user surface (both AFs)
   - Tests: `TestSRSnapshot`, `ospf-sr-originate.ci`, `ospf-sr-show.ci`, `ospfv3-sr-show.ci`, `ospfv3-sr-disable.ci`, the doctor unit/functional test
   - Files: `sr/snapshot.go`, `cmd_show.go`, `yang/ze-ospf-cmd.yang`, `doctor.go`, `internal/core/diagnostic/codes.go`, metric registration
   - Verify: both show commands; the `ze_ospf_sr_*` series with the `af` label; the SRGB/SRLB overlap doctor check; SR disable withdraws LSAs + entries
9. **Functional tests** -> the ten `.ci` cover the user-visible behaviour for both AFs
10. **RFC refs** -> add `// RFC 8665 Section X` (IPv4) and `// RFC 8666 Section X` / `// RFC 8665 Section X` (IPv6 capability TLVs) comments on the V/L validation, index->label, NP/E/M, Adj-SID withdraw, type codes, and §8.2 propagation
11. **Interop** -> `ospf-sr-frr` + `ospfv3-sr-frr` QEMU scenarios (label agreement + label-switched ping per AF)
12. **Full verification** -> `make ze-verify`
13. **Complete spec** -> audit tables + learned summary; two commits (A: code+spec+learned, B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has file:line implementation for the applicable AF(s) |
| Feature completeness | each user story has a working path; SR parity with FRR `ospfd` (IPv4) and `ospf6d` (IPv6): capabilities + Prefix-SID + Adj-SID + label install; TI-LFA / SR-TE / SRv6 / mapping-server-server excluded by design |
| Correctness | multi-range index->label arithmetic (shared); V/L sizing; NP/E/M truth table (NULL=0 IPv4, NULL=2 IPv6); area scoping; Adj-SID withdraw below 2-Way; SRGB/SRLB non-overlap; duplicate/unknown-algorithm ignore rules; RFC 8666 OSPFv3 type codes (not RFC 8665 values) |
| Naming | one `ze_ospf_sr_*` metric namespace with an `af` label; YANG `segment-routing`/`srgb`/`srlb` kebab-case; `mplsSourceOSPFSR` / `mplsSourceOSPFv3SR` |
| Data flow | IPv4 SR via ext-1/ext-3/ext-4; IPv6 SR via the v3 codec + LSDB store-by-scope; forwarding via mpls-fib only; shared SPF read-only; no SR spelling in carriers/core; the two RFC code sets never cross |
| CLI grammar | `show ospf segment-routing` + `show ospf ipv6 segment-routing` action-before-identifier |
| Doctor checks | the SR/MPLS doctor check (SRGB/SRLB overlap with LDP/RSVP-TE + MPLS routing capability) registered per `ai/rules/doctor-checks.md` |
| YANG validation | SRGB/SRLB ranges have `range` constraints + the non-overlap custom validator; no bare `type string` |
| Prometheus counters | the seven `ze_ospf_sr_*` series defined, registered, listed, AF-labelled |
| Rule: plugin-self-containment | carriers/core/v3-base name nothing SR; removing the SR code removes all SR behaviour for both AFs |
| Rule: buffer-first | SR TLV emit uses the carrier builders; RI/Extended encode write into caller buffers; mpls-fib entries value-typed; IPv6 word pad written |
| Rule: shared-not-duplicated | SRGB/SRLB/label/NP-E-M logic is written once and used by both AFs; grep shows one implementation |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Shared SR control logic (label compute, allocator, NP/E/M) used by both AFs | `grep -rn 'sr/srgb\|sr/srlb\|sr/install' internal/plugins/ospf` shows one implementation imported by both paths |
| IPv4 SR codec round-trips every TLV/sub-TLV | `go test ./internal/plugins/ospf/sr -run 'RoundTrip'` |
| IPv6 RI + Extended LSA carriage + RFC 8666 type codes | `go test ./internal/plugins/ospf/v3/packet -run 'RILSA|ExtendedLSA|SRTLV|SRTypeCodes'` |
| Multi-range index->label correct (both AFs) | `go test ./internal/plugins/ospf/... -run 'LabelFromIndex|SRGBMultiRange'` |
| MPLS install via mpls-fib with the right source tags | `grep -rn 'mplsSourceOSPFSR\|mplsSourceOSPFv3SR' internal/plugins/ospf` + `go test ./internal/plugins/ospf/... -run 'Install'` |
| Adj-SID withdraw on adjacency loss (both AFs) | `go test ./internal/plugins/ospf/... -run 'AdjSIDWithdraw|AdjSIDWithdrawOnDown'` |
| Seven SR metric series registered (AF-labelled) | `grep -rn 'ze_ospf_sr_' internal/plugins/ospf` |
| SR snapshot + both CLI commands | `ls test/ospf/ospf-sr-*.ci test/ospfv3/ospfv3-sr-*.ci` + `grep -rn 'segment-routing' internal/plugins/ospf/yang` |
| Interop scenarios present | `ls test/interop/scenarios/ospf-sr-frr/ test/interop/scenarios/ospfv3-sr-frr/` |
| Self-contained | OSPF + base v3 suites green; `TestOSPFBuildsWithoutSR`, `TestOSPFv2Unaffected`, `TestOSPFv3BaseLSAsUnchanged` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | SR TLV/sub-TLV parsing bound-checked over the carrier iterators (ext-1 for IPv4, the v3 `LSAIterator` for IPv6); malformed lengths/SID fields make the LSA malformed; the OSPF + v3 packet fuzz targets extended with SR bodies |
| Data-plane safety | SR programs the MPLS data plane: a wrong label misroutes traffic. Validate every label/index against the originator's advertised SRGB; never install a label for an unsupported algorithm or a non-SR next-hop; gate on reachability; the two RFC code sets must not cross-contaminate |
| Resource exhaustion | the SRLB allocator is bounded by the configured range and reports exhaustion; the per-router SR capability store is bounded by the LSDB cap; a flood of remote prefix-SIDs cannot grow SR state unbounded |
| Trust boundary | received SR TLVs ride the existing OSPF authentication (RFC 7474 for IPv4, RFC 7166 for IPv6); no new auth surface; SR install never bypasses fib-kernel ownership |
| Error leakage | label-compute / malformed-TLV errors are counted (`ze_ospf_sr_*_total`), rate-limited in logs, not surfaced to peers |

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
OSPF SR is one feature across two address families: the control plane (SRGB/SRLB
management, the multi-range index->label arithmetic, the NP/E/M PHP truth table,
and the install through the existing `mpls-fib` bus) is AF-neutral and written
once, exactly as OSPF itself is one engine across IPv4 and IPv6. Only the wire
carriage differs: the IPv4 family plugs SR TLVs into the delivered opaque RI /
Extended Prefix / Extended Link carriers (ext-3/ext-4), while the IPv6 family must
first add the OSPFv3 RI Opaque LSA and the RFC 8362 Extended-LSA subset (since the
base OSPFv3 codec has no opaque carrier), then ride the RFC-8666-coded SR TLVs on
that new carriage. The two error-prone pieces are shared (multi-range SRGB mapping)
and per-AF (the RFC 8666 type codes must not be confused with the RFC 8665 ones);
both get exhaustive boundary tests and an FRR label-agreement interop per AF.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One SR feature across both AFs; control logic shared, only the codec/carriage per-AF | two version-split specs / two separate SR subsystems | OSPF is one engine across AFs (like `bgp`); the SRGB/SRLB/label/NP-E-M logic is identical; sharing it avoids duplicating the most error-prone arithmetic |
| SR is a consumer, not OSPF-core changes | bake SR into the RI/Extended carriers | plugin-self-containment + RFC layering: SR owns its TLVs and forwarding; carriers stay SR-agnostic; removing the SR code removes SR |
| IPv4 plugs into delivered carriers; IPv6 adds the SR subset of RFC 7770 RI + RFC 8362 Extended LSAs | a standalone RI/Extended spec first for IPv6, then SR on top | OSPFv3 has no other consumer of that carriage yet; bundling avoids a carrier with no user; scope is bounded to what SR needs, with verbatim passthrough for the rest |
| Forwarding installs through the existing `mpls-fib` bus | a new SR-specific FIB path / direct netlink | fib-kernel is the single netlink owner; SR is the third producer (RSVP-TE=1, LDP=2) with two distinct source tags; no duplicated forwarding code |
| The SRLB Adj-SID allocator reuses the LDP bounded-pool pattern | a new allocator with persistence | the LDP `nextLabel`/`MaxLabel` shape already solves bounded 20-bit allocation; one allocator serves both AFs; persistence (P-Flag) is a follow-up |
| The SRGB is a single configured range on originate; multiple on receive | dynamic SRGB allocation | operational norm is one configured SRGB block; the receive path MUST handle multiple ranges (interop) but originate stays simple (A-11) |
| Ze computes only Algorithm 0; unsupported-algorithm prefix-SIDs recorded but not installed | install all advertised SIDs | the RFCs MUST ignore a Prefix-SID whose algorithm is not advertised / not computed; strict-SPF (Alg 1) is out of scope |
| SR install fires from a post-SPF-run hook AFTER the IP-route Installer | recompute SR on every LSDB change | SR pushes ride existing IP routes; installing after `Apply` avoids the push-before-route race (R-8) |
| New typed bodies on the existing v3 `LSA` union, not a new codec | a parallel Extended-LSA codec | the union + verbatim passthrough already handle unknown LSAs; adding bodies is additive and inherits flood/aging/scope routing |

## Known Limitations
- TI-LFA / backup paths are not computed (the Adj-SID B-Flag is advertised but no protection path is installed) -- `spec-ospf-ext-6-ti-lfa.md`.
- SR-TE policies (segment lists, binding SIDs) are not part of OSPF SR -- a separate subsystem.
- SRv6 (IPv6 data-plane SIDs) is out of scope for both AFs; only the MPLS data plane is programmed.
- Strict-SPF (Algorithm 1) paths are not computed; such prefix-SIDs are recorded but not installed.
- SR Mapping Server *server* role is out of scope: Ze honours received mapping-server SIDs (M-flag, Extended Prefix Range TLV) but does not originate Range TLVs or inject NU-bit prefixes.
- The SRGB is originated as a single configured range per AF (multiple ranges are accepted on receive).
- For IPv6, only the SR subset of the RFC 7770 RI / RFC 8362 Extended-LSA frameworks is interpreted; other RI/Extended TLVs are passed through verbatim.

## RFC Documentation
Add `// RFC 8665 Section X.Y: "<quoted requirement>"` (IPv4) and `// RFC 8666
Section X.Y` / `// RFC 8665 Section X` (IPv6 capability TLVs) above the enforcing
code:
- SR-Algorithm MUST include Algorithm 0; first-occurrence/area-scope/Instance-ID tie-break (RFC 8665 §3.1 / RFC 8666 §4)
- Range Size > 0; exactly one SID/Label Sub-TLV; ranges concatenated in advertised order; stable order across restart (RFC 8665 §3.2)
- SRLB Range Size > 0; Adj-SIDs from the SRLB (RFC 8665 §3.3); SRMS-Pref tie-break (§3.4)
- V/L validation (only 0/0 and 1/1); algorithm-not-advertised ignore; duplicate prefix/topology/algorithm all ignored; NP/E/M outgoing-label truth table (NULL=0 IPv4 RFC 8665 §5, NULL=2 IPv6 RFC 8666 §6); NP set + E clear for ABR/ASBR non-attached prefix-SIDs
- Adj-SID/LAN-Adj-SID flags; P-Flag persistence (RFC 8665 §6.1/§6.2; RFC 8666 §7.1/§7.2)
- Withdraw the Adj-SID when the adjacency drops below 2-Way (RFC 8665 §7.4.1; RFC 8666 §8.4.1)
- IPv6 OSPFv3 Extended-LSA registry type codes vs the OSPFv2 values (RFC 8666); inter-area Prefix-SID propagation (RFC 8666 §8.2; RFC 8665 §4 IA-Flag)
- Malformed TLV/sub-TLV -> LSA malformed, no crash, count/log rate-limited (RFC 8665 §9/§10; RFC 8666 §10/§11)
Also `// RFC 7770` (RI LSA carriage, area scope) and `// RFC 7684` (IPv4 Extended Prefix/Link sub-TLV carriage) and `// RFC 8362` (IPv6 Extended-LSA carriage) where SR rides those carriers.

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
| Advertise SR-Algorithm/SRGB/SRLB/SRMS in the RI LSA (both AFs) | unit + functional + interop | `TestSRAlgorithmTLVRoundTrip`, `TestOSPFv3RILSARoundTrip`, `ospf-sr-originate.ci`, `ospfv3-sr-originate.ci`, `ospf-sr-frr`, `ospfv3-sr-frr` |
| Prefix-SID / Adj-SID / LAN-Adj-SID in the per-AF carriers | unit + interop | `TestPrefixSIDSubTLVRoundTrip`, `TestOSPFv3SRTypeCodes`, `TestAdjSIDSubTLVRoundTrip`, `ospf-sr-frr`, `ospfv3-sr-frr` |
| Compute MPLS labels from prefix-SID index against the SRGB (shared) | unit | `TestSRLabelFromIndexMultiRange`, `TestOSPFv3SRGBMultiRangeIndex`, `TestSRLabelIndexOutOfRange` |
| Program SR forwarding into the MPLS plane (both AFs) | functional + interop | `ospf-sr-receive.ci`, `ospfv3-sr-receive.ci` (mpls-fib push), `ospf-sr-frr`, `ospfv3-sr-frr` (label-switched ping, `mpls -ls`) |
| Adj-SID lifecycle on adjacency state (both AFs) | unit + functional | `TestSRAdjSIDWithdrawnBelow2Way`, `TestOSPFv3AdjSIDWithdrawOnDown`, `ospf-sr-adj.ci` |
| IPv6 RI + Extended LSA carriage added (SR subset) | unit | `TestOSPFv3RILSARoundTrip`, `TestOSPFv3ERouterLSARoundTrip`, `TestOSPFv3UnknownRITLVReflooded` |
| CLI + metrics (both AFs) | functional | `ospf-sr-show.ci`, `ospfv3-sr-show.ci`, `ze_ospf_sr_*` series (af-labelled) |
| One engine, no version branches leaked; v4/base-v3 unaffected | grep + suites | `TestOSPFv2Unaffected`, `TestOSPFv3BaseLSAsUnchanged`; grep shows no RFC-code crossover |

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
- [ ] AC-1..AC-23 all demonstrated (per applicable AF)
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/plugins/ospf/sr/*`, `internal/plugins/ospf/v3/*`, `internal/plugins/ospf/*`)
- [ ] Integration completeness proven end-to-end for both AFs
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass)
- [ ] RFC 8665 / 8666 / 7770 / 7684 / 8362 constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (SR is the first MPLS-programming OSPF consumer; the codec/install split is justified by the test surface)
- [ ] No speculative features (no TI-LFA, no SR-TE, no SRv6, no strict-SPF install, no mapping-server server role)
- [ ] Single responsibility per component (codec / srgb / srlb / install / adjsid / v6-carriage separated)
- [ ] Explicit > implicit behavior (the NP/E/M truth table is explicit, not inferred)
- [ ] Minimal coupling (carriers/core/v3-base name nothing SR; control logic shared, not duplicated; RFC code sets isolated)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior (both AFs)
- [ ] Interop tests for protocol features (`ospf-sr-frr`, `ospfv3-sr-frr`)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ospf-ext-5-segment-routing.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospf-ext-5-segment-routing.md`
