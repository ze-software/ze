# Spec: ospfv3-ext-6 -- OSPFv3 Segment Routing (RFC 8666)

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
3. `rfc/short/rfc8666.md` -- the OSPFv3 SR wire spec: SID/Label sub-TLV (§3.1, type 7), Extended Prefix Range TLV (§5, type 9), Prefix-SID sub-TLV + NP/M/E/V/L flags (§6, type 4), Adj-SID (§7.1, type 5) + LAN-Adj-SID (§7.2, type 6) sub-TLVs, SR capabilities carried in the OSPFv3 RI Opaque LSA (§4, reuse RFC 8665 TLVs), inter-area Prefix-SID propagation (§8.2), Adj-SID withdraw on adjacency < 2-Way (§8.4.1)
4. `rfc/short/rfc8665.md` -- the SR capability TLV definitions reused unchanged by RFC 8666 §4: SR-Algorithm (type 8), SID/Label Range / SRGB (type 9), SR Local Block / SRLB (type 14), SRMS-Preference (type 15), SID/Label sub-TLV, label computation from SRGB index (§3.2)
5. `rfc/short/rfc5340.md` -- OSPFv3 base: 20-byte LSA header with a 16-bit scope-encoded LS Type (§A.4.2.1), IPv6 PrefixLength/PrefixOptions/padded-32-bit-word prefix encoding (§A.4.1), and the base LSA registry this spec extends with RFC 8362 Extended LSAs
6. `plan/spec-ospfv3-0-umbrella.md` -- the delivered OSPFv3 umbrella: package layout (`internal/plugins/ospf/v3/`), the explicit "Opaque/TE/SR/GR/BFD" out-of-scope row this spec now closes, and the IPv6 Loc-RIB install seam
7. `plan/spec-ospf-ext-5-segment-routing.md` -- the OSPFv2 SR sibling (NO shared code): mirror its label-computation, SRGB index resolution, NP/E/M push/swap/PHP decision, and `mpls-fib` install design, re-expressed against the OSPFv3 Extended-LSA carriage
8. `internal/plugins/ospf/v3/packet/lsa.go` -- the OSPFv3 `LSA` struct (typed-body union + verbatim `RawBytes` passthrough), `DecodeLSA`, `WriteTo`, `LSAIterator`; the seam where Extended-LSA and RI-LSA typed bodies attach
9. `internal/plugins/ospf/v3/types/lsa.go` -- `LSType uint16` with scope in the high bits (`0x2001` area, `0x4005` AS, `0x0008` link-local), `Scope()`, the base-type constants; where the new Extended-LSA and RI function codes are added
10. `internal/plugins/ospf/lsdb/origination.go` -- `OriginateSelf(area, key, body, SelfLSAEncoder)` + `SelfLSAEncoder`, the AF-neutral self-LSA origination seam the RI LSA and Extended-LSAs originate through
11. `internal/plugins/ospf/afstrategy_v6.go` + `internal/plugins/ospf/origination_v6.go` -- the v6 SPF strategy (`BuildRoutes`/`ComputeInterArea`/`OriginateSummaries`) and the v6 self-LSA origination (`v6OriginateSelf`, `v6OriginateRouter`, `v6OriginHeader`) this spec hooks for Prefix-SID/Adj-SID install and SR-LSA origination
12. `internal/core/mplsfib/events.go` -- the `mpls-fib` bus: `Entry{Op: Push/Swap/Pop, Action: Add/Remove, InLabel, FEC, OutLabels, NextHop, Source}`; fib-kernel is the single netlink owner. `internal/plugins/ldp/fib.go` -- the `ProgramPush`/`ProgramPop` + label-pool pattern to reuse

## Task

Add OSPFv3 Segment Routing (RFC 8666) to the existing native OSPFv3 address
family inside the OSPF edge plugin at `internal/plugins/ospf/` (the v6 engine
instance over `internal/plugins/ospf/v3/`). SR is the first OSPFv3 consumer that
programs the MPLS data plane: it advertises SR capabilities and per-prefix /
per-adjacency SIDs, computes MPLS labels from advertised Prefix-SID indices
against the originator's SRGB, and installs label-switched forwarding entries for
Prefix-SIDs (node SIDs) and Adjacency-SIDs through the existing `mpls-fib` bus,
the same seam LDP and RSVP-TE use.

**Critical dependency note (stated after exploring the source):** RFC 8666
carries SR information in two structures the base OSPFv3 implementation does NOT
yet have. (1) The SR capability TLVs (SR-Algorithm, SRGB, SRLB, SRMS-Preference)
ride in the **OSPFv3 Router Information (RI) Opaque LSA** (RFC 7770, OSPFv3
function code 12, area scope `0xA00C` / AS scope `0xC00C`). (2) The Prefix-SID,
Adj-SID, LAN-Adj-SID, and Extended Prefix Range TLVs ride inside the **RFC 8362
Extended LSAs** (E-Router-LSA, E-Intra-Area-Prefix-LSA, E-Inter-Area-Prefix-LSA,
E-AS-External-LSA, E-Type-7-LSA). The OSPFv3 codec
(`internal/plugins/ospf/v3/packet/lsa.go`) today defines ONLY the RFC 5340 base
LSAs (`0x2001` Router, `0x2002` Network, `0x2003`/`0x2004` Inter-Area,
`0x4005` AS-External, `0x2007` NSSA, `0x0008` Link, `0x2009` Intra-Area-Prefix);
there is no RI LSA and no Extended-LSA family. **Therefore this spec INCLUDES
adding the OSPFv3 RI Opaque LSA and the RFC 8362 Extended-LSA carriage required
to host the SR TLVs.** It adds exactly the Extended-LSA types and TLVs RFC 8666
needs (E-Router-LSA for Adj-SID/LAN-Adj-SID; the E-prefix LSAs for Prefix-SID and
Extended Prefix Range); a full standalone RFC 8362 / RFC 7770 implementation
(every Extended-LSA optional TLV, multi-instance RI consumers beyond SR) is NOT
in scope -- only the carriage SR rides on, plus verbatim flood/passthrough for
unknown TLVs so non-SR Extended-LSA content from peers is not corrupted.

Origination: when SR is enabled on the v6 family, this node advertises its
SR-Algorithm, SRGB (one or more SID/Label Range TLVs), SRLB, and optionally
SRMS-Preference top-level TLVs in the OSPFv3 RI Opaque LSA; a Prefix-SID sub-TLV
under the Intra-Area Prefix TLV (in the E-Intra-Area-Prefix-LSA / E-Router-LSA
prefix carriage) for each configured node prefix (typically the loopback); and an
Adj-SID sub-TLV (plus LAN-Adj-SID on broadcast/NBMA) under the Router-Link TLV in
the E-Router-LSA for each adjacency in state 2-Way or higher.

Reception + forwarding: when an RI LSA from a remote router carries SR TLVs, this
node records that router's SR-Algorithm and SRGB (the ordered concatenation of
its ranges). When an Extended prefix LSA carries a Prefix-SID for a prefix the
v6 SPF route table can reach, this node computes the outgoing MPLS label from the
originator's SRGB (`label = SRGB_base + index`, resolving across ranges in
advertised order), honours the next-hop router's NP/E/M flags to decide
push/swap/PHP, and installs an MPLS push (ingress) or swap (transit) entry toward
the v6 SPF next-hop. When an E-Router-LSA carries an Adj-SID/LAN-Adj-SID this node
(as the advertiser) installs the corresponding pop/forward entry keyed by the
local label it allocated for that adjacency.

SRGB/SRLB management: the SRGB is a configured contiguous global label range this
node owns; the SRLB is a configured local label range from which Adj-SIDs are
allocated. Allocation reuses the LDP label-pool pattern (a bounded 20-bit
allocator) rather than inventing a new mechanism.

The feature is self-contained inside the OSPF plugin's v6 family: it adds the
v3 RI-LSA / Extended-LSA codecs, the SR TLV codecs, the SR origination hooks off
`v6OriginateSelf`, the SR reception/label-install hooks off the v6 SPF result,
config under `ospf { address-family ipv6 { segment-routing { ... } } }`, and SR
CLI/metrics. Removing the SR config disables origination and label install;
removing the SR code removes the SR TLVs, the RI/Extended carriage they ride in,
and the `mpls-fib` SR entries. It shares NO code with OSPFv2 SR (ext-5): the
type codes, LSA carriage, and prefix encoding differ (RFC 8666 vs RFC 8665), and
the two families run as independent engine instances.

### In scope (this spec)

| Item | Detail |
|------|--------|
| OSPFv3 RI Opaque LSA | RFC 7770 Router Information LSA for OSPFv3 (function code 12; area scope `0xA00C`, AS scope `0xC00C`); originate this node's RI LSA carrying the SR capability TLVs; decode received RI LSAs to extract SR capabilities; reuse the `OriginateSelf` seam and verbatim flood; unknown RI TLVs are stored and reflooded, never interpreted |
| RFC 8362 Extended-LSA carriage (SR subset) | E-Router-LSA (`0x2021`) Router-Link TLV (host for Adj-SID/LAN-Adj-SID); E-Intra-Area-Prefix-LSA (`0x2029`), E-Inter-Area-Prefix-LSA (`0x2023`), E-AS-External-LSA (`0x4025`), E-Type-7-LSA (`0x2027`) prefix TLVs (host for Prefix-SID + Extended Prefix Range); 4-byte-aligned top-level-TLV + sub-TLV iteration/emission; verbatim passthrough for unknown TLVs |
| SR capability TLVs | SR-Algorithm (8), SID/Label Range / SRGB (9), SR Local Block / SRLB (14), SRMS-Preference (15) top-level TLVs of the RI LSA, plus the SID/Label sub-TLV (RFC 8666 §3.1, type 7); encode/decode reusing the RFC 8665 layouts (§4 reuses them unchanged) |
| Prefix-SID sub-TLV | RFC 8666 §6, type 4; NP/M/E/V/L flags, Algorithm, SID/Index/Label sized by V/L; emit for configured node prefixes; parse on received E-prefix LSAs |
| Adj-SID / LAN-Adj-SID sub-TLV | RFC 8666 §7.1 (type 5) / §7.2 (type 6); B/V/L/G/P flags, Weight, (LAN) Neighbor ID; emit per adjacency; withdraw on adjacency < 2-Way (§8.4.1) |
| Extended Prefix Range TLV | RFC 8666 §5, type 9; AF, Prefix Length, Range Size, IPv6 padded prefix; parse + Prefix-SID-per-range starting-value semantics (decode/store; mapping-server origination is a non-goal, see out of scope) |
| SRGB / SRLB allocation | configured global SRGB + local SRLB ranges; a bounded 20-bit label allocator (LDP pool pattern); ordered range concatenation for index->label resolution |
| MPLS label computation + install | `label = SRGB_base + index` across the originator's ranges in advertised order; NP/E/M -> push/swap/PHP decision (§6); install Prefix-SID push/swap and Adj-SID pop entries via the `mpls-fib` bus toward the v6 SPF next-hop |
| Inter-area Prefix-SID propagation | when the v6 ABR re-advertises a prefix between areas, include the Prefix-SID per §8.2 (best path in source area, else backbone), NP set / E clear unless directly attached |
| Config / CLI / metrics | `segment-routing` config under the v6 family; `show ipv6 ospf segment-routing` (capabilities, SRGB/SRLB, prefix-SIDs, adj-SIDs); SR Prometheus series |

### Out of scope (noted so it is not silently assumed done)

| Item | Reason / where |
|------|---------------|
| OSPFv2 SR (RFC 8665) | spec-ospf-ext-5; different carriage and type codes; NO shared code |
| SRv6 (IPv6 data plane SIDs) | RFC 8666 §1 explicitly defers the IPv6 data plane to a separate document; only the MPLS data plane is in scope |
| TI-LFA / topology-independent fast reroute | separate spec; this spec sets the Adj-SID B-flag eligibility only, no backup-path computation |
| SR Mapping Server origination (SRMS server role) | this node decodes received Extended Prefix Range TLVs + SRMS-Preference and honours mapping-server SIDs (M-flag) on reception, but does NOT act as a mapping server (no Range-TLV origination, no NU-bit prefix injection) |
| Full RFC 8362 Extended-LSA feature set | only the Extended-LSA types + TLVs RFC 8666 SR rides on are added; other Extended-LSA optional TLVs are passed through verbatim, not interpreted |
| Full RFC 7770 RI consumer framework | only SR capability TLVs are interpreted in the RI LSA; other RI TLVs (e.g. hostname, GR capability) are passed through verbatim |
| Strict-SPF algorithm path computation (Algorithm 1) | SR-Algorithm 1 may be advertised/recorded, but Ze computes only Algorithm 0 paths; a Prefix-SID for an unsupported algorithm is recorded but no separate SPF is run |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `docs/research/ospf-implementation-guide.md` (grep "Segment Routing", "Router Information", "Extended Prefix", ~lines 53, 192-193, 1526-1536, 1869-1895) -- the SR / RI / Extended-LSA landscape and the "defer until you need SR" note
  -> Decision: model SR exactly as the guide and the OSPFv2 SR sibling do -- SR is a *consumer* that programs the MPLS data plane; it does NOT add a new top-level component; the RI LSA and Extended LSAs are LSA carriage, not new subsystems
  -> Constraint: RFC 8666 SR rides on RFC 8362 Extended LSAs and the RFC 7770 RI LSA, both of which the base v3 codec lacks today; adding that carriage is part of this spec, scoped to the SR subset
- [ ] `plan/spec-ospfv3-0-umbrella.md` "Out of Scope" + "Package Layout" + "Data Flow" -- the umbrella that deferred SR and defined the v6 package layout and IPv6 Loc-RIB seam
  -> Constraint: the umbrella lists "Opaque/TE/SR/GR/BFD" out of scope "until a stable base LSDB and interop"; that base now exists (v3 packet/types/transport/lsdb/spf delivered), so this spec closes the SR row; all SR code stays under the OSPF plugin's v6 family, no shared OSPFv2 wire package
  -> Decision: OSPFv3 runs as a second engine instance over `v6Codec{}` (see `register.go` `eng6`); SR origination/reception attach to that instance's `v6Originate*` and v6 SPF result, not to the v4 engine
- [ ] `ai/rules/plugin-self-containment.md` -- SR is self-contained inside the OSPF plugin
  -> Constraint: removing the SR config disables it; removing the SR code removes the SR TLVs, the RI/Extended carriage, the `mpls-fib` SR entries, the SR config leaves, the SR show command, and the SR metrics; no SR spelling appears in generic mplsfib/sysrib/locrib code (the `mpls-fib` `Source` tag is the only marker)
- [ ] `ai/rules/buffer-first.md`, `ai/rules/memory-architecture.md` -- TLV emit, RI/Extended-LSA encode, and label-stack build are buffer-first
  -> Constraint: the SR TLV builder and the RI/Extended-LSA bodies write into a caller-owned buffer via `WriteTo(buf, off) int`; TLV iterators return views over caller bytes (zero-copy); the IPv6 prefix 32-bit-word pad is written, never produced by slice concatenation
- [ ] `ai/rules/no-sprintf-alloc.md` -- no `fmt`/`+` on the wire or hot path
  -> Constraint: SR-LSA rendering (`show ipv6 ospf segment-routing`) uses `textbuf`/`AppendTo`; label computation and install are alloc-free on the per-route hot path

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc8666.md` -- the OSPFv3 SR wire spec
  -> Constraint: §3.1 SID/Label sub-TLV (type 7) is 3 octets (20-bit label, V=1/L=1) or 4 octets (32-bit SID, V=0/L=0); width is implied by V/L, not a length field; invalid V/L combination MUST be ignored
  -> Constraint: §6 Prefix-SID flags NP/M/E/V/L (bits 1-5); a Prefix-SID whose Algorithm is not in the originator's SR-Algorithm TLV MUST be ignored; multiple Prefix-SIDs for the same prefix/topology/algorithm MUST all be ignored; M-flag set -> ignore NP and E on reception
  -> Constraint: §6 outgoing-label computation MUST take the next-hop router's NP/E/M flags into account (regardless of best-path membership): NP=0 -> upstream pops (PHP); NP=1,E=0 -> keep on top of stack; NP=1,E=1 -> replace with Explicit NULL (IPv6 label 2); ABR/ASBR-propagated prefixes set NP, clear E unless directly attached
  -> Constraint: §7.1/§7.2 Adj-SID/LAN-Adj-SID flags B/V/L/G/P; P-flag set -> SID MUST be persistent; if a P2P adjacency drops below 2-Way the Adj-SID advertisement MUST be withdrawn (§8.4.1)
  -> Constraint: §5 Extended Prefix Range TLV (type 9): AF 0=IPv4/1=IPv6, IPv6 prefix consumes `((PrefixLength+31)/32)` 32-bit words zero-padded; duplicate Range TLV -> smallest Instance ID wins; the carried Prefix-SID is the *starting* value (Nth prefix gets start+N)
  -> Constraint: §10/§11 invalid TLV/sub-TLV length -> the whole LSA is malformed and MUST be ignored; malformed TLVs MUST NOT crash the router; SHOULD count/log rate-limited
  -> Constraint: type codes are the OSPFv3 Extended-LSA registry values (Prefix-SID 4, Adj-SID 5, LAN-Adj-SID 6, SID/Label 7, Extended Prefix Range 9), NOT the OSPFv2 values; reusing OSPFv2 numbers is a bug
- [ ] `rfc/short/rfc8665.md` -- the SR capability TLVs reused by RFC 8666 §4
  -> Constraint: the SR-Algorithm (8), SID/Label Range/SRGB (9), SRLB (14), SRMS-Preference (15) top-level TLVs and the SID/Label sub-TLV carry into the OSPFv3 RI LSA unchanged; SRGB Range Size > 0; exactly one SID/Label sub-TLV per range else the range TLV MUST be ignored; ranges MUST NOT overlap; receiver concatenates ranges in advertised order to map index->label; if SR-Algorithm advertised it MUST include Algorithm 0
- [ ] `rfc/short/rfc5340.md` -- the OSPFv3 base this extends
  -> Constraint: the 20-byte LSA header carries a 16-bit scope-encoded LS Type (U|S2|S1|function); the new RI/Extended types use the same scope encoding (RI area `0xA00C`, E-Router area `0x2021`, E-AS-External AS `0x4025`, E-Link link-local `0x0028`); IPv6 prefixes use PrefixLength + PrefixOptions + padded 32-bit words, the same encoding the Extended Prefix Range TLV reuses

**Key insights:** (minimal context to resume after compaction)
- OSPFv3 SR is a *consumer + carriage* problem: the wire codec lacks the RI LSA and Extended LSAs RFC 8666 rides on, so this spec adds that carriage (SR subset only) plus the SR TLVs plus label install.
- The v6 family already runs as a second engine instance (`eng6` in `register.go`) over `v6Codec{}`; SR hooks attach to `v6OriginateSelf`/`afstrategy_v6` and the v6 SPF result, NOT to OSPFv2.
- Label computation mirrors OSPFv2 SR exactly (`SRGB_base + index`, NP/E/M -> push/swap/PHP) but the carriage, type codes, and IPv6 prefix encoding are RFC 8666-specific; no code is shared with ext-5.
- The `LSA` struct already does verbatim `RawBytes` passthrough for unknown types, so unknown RI/Extended TLVs reflood byte-for-byte without interpretation; the gap is typed bodies + scope routing for the new LS types and the SR TLV codecs.
- MPLS install is via the `mpls-fib` bus (fib-kernel is the single netlink owner); allocation reuses the LDP 20-bit label-pool pattern.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
- [ ] `internal/plugins/ospf/v3/packet/lsa.go` -- the OSPFv3 `LSA` struct: a typed-body union (`Router`, `Network`, `InterAreaPfx`, `InterAreaRtr`, `External`, `Link`, `IntraAreaPfx`) plus `Body`/`RawBytes`; `DecodeLSA` retains raw bytes and lazily decodes; `WriteTo` re-emits `RawBytes` verbatim when no typed body is set (opaque/unknown passthrough already works); `hasTypedBody()` gates re-marshal; `LSAIterator` is bound-checked and never panics
  -> Constraint: the codec already round-trips unknown LSAs verbatim and recomputes Length + Fletcher checksum for constructed ones; ADD new typed bodies (`RouterInfo *RouterInfoLSA`, the Extended-LSA bodies) to the union and the `WriteTo`/`bodyLen`/`hasTypedBody` switches; do NOT rebuild the passthrough machinery
- [ ] `internal/plugins/ospf/v3/types/lsa.go` -- `LSType uint16` with scope in the high two bits (`lsTypeScopeMask 0x6000`), `Scope()`, `Function()`; base constants `LSTypeRouter 0x2001` ... `LSTypeIntraAreaPrefix 0x2009`; `Known()` enumerates the base set
  -> Constraint: ADD the RI LSA type (`LSTypeRouterInfo` area `0xA00C` / AS `0xC00C`) and the Extended-LSA types (`0x2021` E-Router, `0x2029` E-Intra-Area-Prefix, `0x2023` E-Inter-Area-Prefix, `0x4025` E-AS-External, `0x2027` E-Type-7, `0x0028` E-Link); scope falls out of the high bits automatically; widen `Known()` so the LSDB stores + floods them by scope (U-bit handling for the rest is already correct)
- [ ] `internal/plugins/ospf/v3/packet/lsa_intraarea_prefix.go`, `lsa_router.go`, `lsa_external.go`, `prefix.go` -- the base RFC 5340 Router-LSA (graph links only), Intra-Area-Prefix-LSA (PrefixLength/PrefixOptions/padded words), External-LSA bodies, and the IPv6 prefix codec (`((PrefixLength+31)/32)` words)
  -> Constraint: the Extended-LSA prefix carriage REUSES the existing IPv6 `Prefix` codec for the Address-Prefix field; the Extended Prefix Range TLV reuses it too; do NOT reimplement IPv6 prefix encoding
- [ ] `internal/plugins/ospf/lsdb/origination.go` -- `OriginateSelf(area, key, body, SelfLSAEncoder)` is the AF-neutral self-LSA origination seam (sequencing, MinLSInterval, install, flood); `OriginateFromTopology`, `OriginateRouter`, `OriginateSummary`, `OriginateExternal`, `installOriginated`
  -> Constraint: the RI LSA and the self-originated Extended-LSAs originate through `OriginateSelf` with a `SelfLSAEncoder` that emits the RI/Extended body; sequencing/age/flood/withdraw (MaxAge) are inherited; no new origination path
- [ ] `internal/plugins/ospf/afstrategy_v6.go` -- the v6 SPF strategy: `BuildGraph`, `BuildRoutes`/`v6BuildRoutes` (yields `[]ospfspf.RouteEntry` from Intra-Area-Prefix-LSAs), `ComputeInterArea`, `ComputeExternal`, `OriginateSummaries`/`v6OriginateSummaries`, `NextHopSource` (`P2PNextHop`/`TransitNextHop` -> IPv6 link-local next-hop)
  -> Constraint: Prefix-SID label install consumes the v6 `RouteEntry` set (prefix, metric, origin RouterID, next-hops) + the per-next-hop neighbour RouterID; Adj-SID install consumes the per-adjacency neighbour identity; the next-hop comes from `NextHopSource`, not a new SPF
- [ ] `internal/plugins/ospf/origination_v6.go` -- `v6OriginateSelf(router, maxMetric)`, `v6OriginateRouter`, `v6OriginateIntraAreaPrefix`, `v6OriginateNetwork`, `v6OriginHeader(t, lsid, router, seq, purge)`
  -> Constraint: SR origination hangs off `v6OriginateSelf` (called on topology change + tick): originate/refresh the RI LSA (SR capabilities), the E-Router-LSA (Adj-SIDs), and the Prefix-SID-bearing E-prefix LSA when SR is enabled; `v6OriginHeader` builds the LSA header for the new types
- [ ] `internal/plugins/ospf/register.go` (~lines 222-325) -- the v6 engine instance (`eng6 := newEngineWithCodec(ospfv3transport.New(...), v6Codec{})`), driven by `cfg.V6` (`ospf { address-family ipv6 }`); `consumer.SetV6Injector(eng6)` for IPv6 redistribution; v6 interfaces open only when `cfg.V6.Present()`
  -> Constraint: SR config resolves into `cfg.V6` (a v6-family `segment-routing` block); SR runtime state lives on `eng6`; the v4 engine is untouched; the SR show command + metrics register alongside the existing v6 surfaces
- [ ] `internal/core/mplsfib/events.go` -- the `mpls-fib` bus: `Entry{Op: Push/Swap/Pop, Action: Add/Remove, InLabel, FEC netip.Prefix, OutLabels []uint32, NextHop netip.Addr, Source uint16}`; fib-kernel is the single netlink owner; value-typed, no pointer into producer memory
  -> Constraint: SR installs through this bus exactly as LDP/RSVP-TE do; a new `Source` tag (`mplsSourceOSPFv3SR`) distinguishes SR entries for diagnostics + stale-sweep; no direct netlink, no sysrib best-path abuse for label-keyed swap/pop entries
- [ ] `internal/plugins/ldp/fib.go` + `internal/plugins/ldp/lib.go` -- `ProgramPush(fec, label, nextHop)`, `ProgramPop(fec, inLabel)`, `mplsSourceLDP`, and the bounded 20-bit label allocator (`allocateLabelLocked`, `nextLabel`, `MaxLabel`)
  -> Constraint: the SRGB/SRLB allocator and the SR push/pop install reuse this proven pattern (a bounded allocator + `mpls-fib` emit); do NOT invent a second allocator or a second netlink path

**Behavior to preserve:**
- The OSPFv3 base codec's verbatim passthrough (`LSA.WriteTo` re-emits `RawBytes`), the LSDB key triple, the scope-encoded LS Type, the IPv6 `Prefix` codec, and `OriginateSelf`/`SelfLSAEncoder`.
- All existing OSPFv3 functional/interop tests: a v6 router with SR disabled behaves exactly as today (it does originate the new RI LSA only if a future RI consumer enables it; with SR off no RI/Extended SR LSA is originated, and received RI/Extended LSAs are stored + reflooded but produce no label install).
- OSPFv2 (the v4 engine) and OSPFv2 SR (ext-5) are completely untouched; no shared package gains a v3 branch.
- The `mpls-fib` bus contract and fib-kernel's single-owner netlink role.

**Behavior to change:** (all RFC-8666-required, not discretionary)
- `internal/plugins/ospf/v3/types/lsa.go`: add the RI + Extended-LSA type constants and widen `Known()`.
- `internal/plugins/ospf/v3/packet/lsa.go`: add typed bodies for the RI LSA and the SR-relevant Extended LSAs to the union + `WriteTo`/`bodyLen`/`hasTypedBody`.
- v6 origination: originate the RI LSA (SR capabilities), Adj-SIDs in the E-Router-LSA, and Prefix-SIDs in the E-prefix LSA when SR is enabled.
- v6 reception: record peer SR capabilities, compute labels, install push/swap/pop via `mpls-fib`.
- New `segment-routing` config under the v6 family, a new show command, and new SR metrics.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Config:** `ospf { address-family ipv6 { segment-routing { enable; srgb ...; srlb ...; prefix-sid ... } } }` resolves into `cfg.V6` and enables SR on `eng6`.
- **Origination:** a v6 topology/adjacency/prefix change triggers `v6OriginateSelf` -> SR origination of the RI LSA + E-Router-LSA (Adj-SIDs) + Prefix-SID-bearing E-prefix LSA.
- **Reception:** a v6 LS Update carrying an RI LSA or an Extended LSA arrives -> stored by scope in the LSDB -> after install, the SR consumer records capabilities (RI) or computes + installs labels (Extended prefix / E-Router).
- **SPF result:** a v6 SPF run completes -> the SR consumer re-derives Prefix-SID labels for reachable prefixes and Adj-SID entries for local adjacencies.

### Transformation Path
1. **Decode (mostly existing + new typed bodies):** `packet.DecodeLSA` returns an `LSA` whose `Header.Type` is now possibly RI (`0xA00C`/`0xC00C`) or an Extended type (`0x2021`/`0x2029`/...); `VerifyChecksum` (Fletcher) validates it; the SR consumer lazily decodes the RI / Extended body via new `Decode*` methods.
2. **Scope route (existing):** the LSDB routes by the scope bits of the LS Type -- RI area-scope and Extended area-scope LSAs land in the per-area store; RI AS-scope (`0xC00C`) and E-AS-External (`0x4025`) land in the AS-wide store; E-Link (`0x0028`) is link-local. No new store; the scope-encoded type drives the existing routing.
3. **TLV parse (new):** the SR consumer walks the RI LSA's top-level TLVs (SR-Algorithm, SRGB, SRLB, SRMS-Pref) and the Extended-LSA's TLVs/sub-TLVs (Prefix-SID, Adj-SID, LAN-Adj-SID, Extended Prefix Range), bound-checked, ignoring malformed/invalid-V-L/unknown-algorithm advertisements per §6/§10.
4. **Capability record (new):** a peer's SR-Algorithm + SRGB (ordered range concatenation) is stored keyed by Advertising Router; the SRGB maps an advertised index to an absolute label.
5. **Label computation (new):** for each reachable prefix carrying a Prefix-SID, `label = peer_SRGB_base + index` (resolving across the originating-router's ranges in order when V=0); when V=1 the label is taken directly; the next-hop router's NP/E/M flags decide push (ingress, this node originates traffic) vs swap (transit) vs PHP/Explicit-NULL.
6. **Install (new, via existing bus):** Prefix-SID -> `mpls-fib` Push (ingress: FEC=prefix, OutLabels=[remote label], NextHop=v6 SPF next-hop) or Swap (transit: InLabel=local-SRGB label for the prefix, OutLabels=[remote label or pop], NextHop); Adj-SID -> `mpls-fib` Pop (this node allocated the SRLB local label; pop and forward toward the neighbour). `Source = mplsSourceOSPFv3SR`.
7. **Origination (new, reuses existing seams):** when SR is enabled, `v6OriginateSelf` builds: the RI LSA body (SR-Algorithm + one SID/Label Range TLV per SRGB range + SRLB) via a `SelfLSAEncoder` -> `OriginateSelf` (area scope); the E-Router-LSA Router-Link TLVs each carrying an Adj-SID (and LAN-Adj-SID on broadcast/NBMA) for adjacencies >= 2-Way; the E-prefix LSA carrying a Prefix-SID sub-TLV for each configured node prefix. The LSDB owns sequence/age/flood/withdraw. Adj-SID withdraw on adjacency < 2-Way re-originates the E-Router-LSA without that Adj-SID (§8.4.1).
8. **Inter-area (new, hooks v6 ABR):** `OriginateSummaries`/`v6OriginateSummaries` gains the §8.2 Prefix-SID propagation -- when re-advertising a prefix across areas, include the Prefix-SID from the best path in the source (or backbone) area, NP set / E clear unless directly attached.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire <-> RI/Extended LSA | new typed bodies on the existing `LSA` union; verbatim `RawBytes` passthrough for unknown TLVs | [ ] |
| LSA <-> store | existing scope-encoded LS-Type routing (RI/Extended types fall into area/AS/link stores by their scope bits) | [ ] |
| RI/Extended LSA <-> SR consumer | new TLV iterators yield SR capability + SID records (value-typed, no cross-boundary pointers) | [ ] |
| SR consumer <-> v6 SPF | read-only: reachable prefixes, per-next-hop neighbour RouterID, next-hop addr from `NextHopSource` | [ ] |
| SR consumer <-> mpls-fib | `Entry{Op,Action,InLabel,FEC,OutLabels,NextHop,Source=mplsSourceOSPFv3SR}` emitted on the `mpls-fib` bus; fib-kernel programs netlink | [ ] |
| SR allocator <-> label space | bounded 20-bit SRGB/SRLB allocator (LDP pool pattern); ranges non-overlapping | [ ] |
| Config <-> SR runtime | `cfg.V6.SegmentRouting` -> `eng6` SR state; CLI/metrics read immutable snapshots | [ ] |

### Integration Points
- `internal/plugins/ospf/v3/types` -- new RI/Extended LS-Type constants + `Known()`; scope falls out of the high bits.
- `internal/plugins/ospf/v3/packet` -- new RI-LSA + Extended-LSA typed bodies on the `LSA` union; the SR TLV/sub-TLV codecs; reuse the IPv6 `Prefix` codec and `LSAIterator`.
- `internal/plugins/ospf/lsdb` -- `OriginateSelf` reused for RI/Extended self-origination; existing scope routing stores the new types; no new store.
- `internal/plugins/ospf` (v6 family) -- SR config resolution into `cfg.V6`, SR origination hooks off `v6OriginateSelf`/`v6OriginateSummaries`, SR reception + label install off the v6 SPF result, SR runtime state on `eng6`.
- `internal/plugins/ospf/afstrategy_v6.go`, `spf/` -- READ ONLY: reachable prefixes, next-hops, neighbour identity for Prefix-SID/Adj-SID install; §8.2 Prefix-SID propagation in `OriginateSummaries`.
- `internal/core/mplsfib` -- the `mpls-fib` bus for label install (new `Source` tag).
- `internal/plugins/ldp` -- pattern source for the label allocator and the push/pop install (NOT a dependency; the pattern is re-expressed in OSPF).

### Architectural Verification
- [ ] No bypassed layers (SR LSAs flow wire -> codec -> scope-store -> SR consumer -> mpls-fib -> fib-kernel; labels never bypass fib-kernel)
- [ ] No unintended coupling (SR names no foreign plugin; the only cross-boundary marker is the `mpls-fib` Source tag; OSPFv2 and the v4 engine are untouched)
- [ ] No duplicated functionality (reuses the `LSA` passthrough, `OriginateSelf`, the IPv6 `Prefix` codec, `LSAIterator`, the `mpls-fib` bus, the LDP allocator pattern; adds only the RI/Extended typed bodies, the SR TLV codecs, label computation, and the SR origination/reception hooks)
- [ ] Zero-copy preserved (TLV iterators return views; RI/Extended bodies + SR TLVs encode buffer-first; verbatim reflood of unknown TLVs; `mpls-fib` entries are value-typed)

## Risks & Assumptions

<!-- LIVE -- written during RESEARCH/DESIGN, statuses updated during implementation. -->

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The base v3 codec lacks the RI LSA and RFC 8362 Extended LSAs, so this spec must add that carriage (SR subset) | `internal/plugins/ospf/v3/packet/lsa.go` union has only the 8 base bodies; `v3/types/lsa.go` `Known()` enumerates only base types | scope balloons or shrinks; if a hidden RI carrier exists, reuse it instead | grep for `RouterInfo`/`Extended`/`0x2021`/`0xA00C` in `internal/plugins/ospf/v3` returns nothing today | unvalidated |
| A-2 | The `LSA` union + verbatim `RawBytes` passthrough extends cleanly to new typed bodies without rebuilding the codec | `lsa.go` `hasTypedBody`/`WriteTo`/`bodyLen` switch on a body union; `WriteTo` re-emits `RawBytes` when none set | a new codec layer is needed; larger change | `TestOSPFv3RILSARoundTrip`, `TestOSPFv3ERouterLSARoundTrip` (decode->re-encode byte-for-byte) | unvalidated |
| A-3 | The new RI/Extended LS types route to the correct LSDB store purely by their scope bits, with no new store | `v3/types/lsa.go` `Scope()` from `lsTypeScopeMask`; LSDB routes by scope | a new store/key is needed | `TestOSPFv3SRLSAScopeRouting` (RI area/AS, E-Link link-local, E-AS-External AS) | unvalidated |
| A-4 | RI/Extended self-origination works through `OriginateSelf` + a `SelfLSAEncoder` with no new sequencing/flooding | `lsdb/origination.go` `OriginateSelf`/`SelfLSAEncoder`; `origination_v6.go` v6 self-LSA precedent | a new origination path is needed | `TestOSPFv3OriginateRILSA`, `TestOSPFv3OriginateAdjSID`, `TestOSPFv3OriginatePrefixSID` | unvalidated |
| A-5 | The `mpls-fib` bus + the LDP label-pool pattern suffice for SR install + SRGB/SRLB allocation (no new netlink, no new allocator type) | `mplsfib/events.go` Entry/Op; `ldp/fib.go` ProgramPush/Pop + 20-bit allocator | a new label install path or allocator is needed | `TestOSPFv3PrefixSIDInstallsPush`, `TestOSPFv3AdjSIDInstallsPop`, `TestOSPFv3SRGBAllocation`; QEMU `mpls -ls` shows the entries | unvalidated |
| A-6 | The v6 SPF result (`afstrategy_v6` `BuildRoutes` + `NextHopSource`) exposes reachable prefixes, per-next-hop neighbour RouterID, and IPv6 next-hop sufficient for label install | `afstrategy_v6.go` `v6BuildRoutes`, `NextHopSource`, `RouteEntry` | the SPF result must be widened to expose next-hop neighbour identity | `TestOSPFv3LabelFromSRGBIndex` (uses a v6 SPF fixture) | unvalidated |
| A-7 | SR runs entirely on the v6 engine instance (`eng6`); the v4 engine and OSPFv2 SR (ext-5) are untouched and share no code | `register.go` `eng6`/`cfg.V6`; ext-5 lives on the v4 path with RFC 8665 codes | accidental coupling or a shared wire struct leaks version branches | `TestOSPFv2Unaffected` (existing v4 + ext-5 suites green); grep shows no shared SR type | unvalidated |
| A-8 | The IPv6 `Prefix` codec (`v3/packet/prefix.go`) is reusable for the Extended Prefix Range TLV Address-Prefix field and the Prefix-SID parent prefix carriage | `prefix.go` `((PrefixLength+31)/32)` word encoding; RFC 8666 §5 uses the same | a separate prefix codec is needed for the Range TLV | `TestOSPFv3ExtPrefixRangeTLVRoundTrip` (default route + /64 + /128) | unvalidated |
| A-9 | SR LSAs are not SPF vertices: SR affects only label install, never the IPv6 SPF topology graph | OSPFv2 SR precedent (ext-5); RFC 8666 SR is data-plane, not topology | SR data corrupts the v6 SPF graph | `TestOSPFv3SRLSANotInSPFGraph` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Wrong type codes (OSPFv2 RFC 8665 values reused instead of the RFC 8666 OSPFv3 Extended-LSA registry values) | FRR `ospf6d` rejects Ze's SR TLVs or mis-parses them | pin the OSPFv3 codes (Prefix-SID 4, Adj-SID 5, LAN-Adj-SID 6, SID/Label 7, Ext-Prefix-Range 9; RI `0xA00C`, E-Router `0x2021`); `TestOSPFv3SRTypeCodes` asserts each against the RFC; interop with `ospf6d` |
| R-2 | SID/Index/Label width misparsed (assumes a fixed width instead of inferring 3 vs 4 octets from V/L) | a label is off by a byte; install programs the wrong label | width inferred from V/L per §3.1/§6; `TestOSPFv3SIDWidthFromVL` covers V=0/L=0 (4) and V=1/L=1 (3) and rejects invalid combinations |
| R-3 | SRGB index->label resolution wrong across multiple ranges (concatenation order ignored) | traffic to a remote node SID gets the wrong label and is mis-switched | resolve across ranges in advertised order (§3.2); `TestOSPFv3SRGBMultiRangeIndex` with two ranges asserts the boundary index maps into the second range |
| R-4 | NP/E/M decision wrong -> PHP where it must not happen, or no Explicit-NULL where required (IPv6 label 2) | the penultimate hop pops a no-PHP SID; the egress drops labelled traffic | implement the §6 decision table exactly (NP=0 PHP; NP=1/E=0 keep; NP=1/E=1 Explicit-NULL 2; M -> ignore NP/E); `TestOSPFv3LabelOpFromFlags` covers all combinations |
| R-5 | Adj-SID not withdrawn when an adjacency drops below 2-Way (§8.4.1) -> stale local label and a black-holed pop entry | a neighbour goes down but the Adj-SID pop entry lingers | re-originate the E-Router-LSA without the Adj-SID and remove the `mpls-fib` pop entry on the adjacency-down event; `TestOSPFv3AdjSIDWithdrawOnDown` |
| R-6 | Malformed RI/Extended TLV from a peer crashes the decoder (untrusted input) | fuzz crash; panic on a truncated sub-TLV | every TLV/sub-TLV iterator is bound-checked and never panics; an invalid length marks the LSA malformed and ignores it (§10); extend the existing `v3/packet` fuzz target with RI/Extended bodies; `TestOSPFv3SRTLVMalformed` |
| R-7 | A duplicate / invalid-algorithm / multiple Prefix-SID is honoured instead of ignored (§6) -> a wrong label installed | a Prefix-SID for an algorithm the peer never advertised installs a label | enforce the §6 ignore rules (algorithm-not-advertised, multiple-for-same-prefix, invalid V/L); `TestOSPFv3PrefixSIDIgnoreRules` |
| R-8 | SRGB/SRLB overlap or exhaustion mis-allocates a label or collides with LDP/RSVP-TE labels | two sources program the same in-label; fib-kernel logs a conflict | the SR allocator owns a configured disjoint range; bounds-checked; reject overlapping configured ranges at validation; `TestOSPFv3SRGBExhaustion`, doctor check for SRGB/SRLB/LDP overlap |
| R-9 | Adding new LS types to `Known()` changes flooding/store behaviour for non-SR Extended LSAs and breaks an existing v3 test | an existing v3 flooding/origination test fails | the new types are additive and scope-routed; with SR off nothing originates; run the full v3 suite after the type addition; `TestOSPFv3BaseLSAsUnchanged` |
| R-10 | The MPLS data plane is Linux-only; SR install cannot be unit-tested end-to-end | install logic looks right but never programs a real kernel | QEMU integration test (`ai/rules/qemu-testing.md`) asserts `mpls -ls` shows the SR push/swap/pop entries; the install decision is unit-tested behind a `mpls-fib` fake |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ospf { address-family ipv6 { segment-routing { enable; srgb 16000 8000 } } }` resolves | -> | v6 SR config enabled on `eng6`; SRGB allocated | `TestOSPFv3SRConfigEnables` (unit) + `test/ospfv3/ospfv3-sr-config.ci` |
| v6 `v6OriginateSelf` runs with SR enabled | -> | RI LSA (SR-Algorithm+SRGB+SRLB) + E-Router-LSA (Adj-SIDs) + Prefix-SID E-prefix LSA originated and flooded | `test/ospfv3/ospfv3-sr-originate.ci` |
| a v6 LS Update carrying a peer RI LSA + Prefix-SID Extended LSA arrives for a reachable prefix | -> | record peer SRGB -> compute label -> `mpls-fib` Push/Swap toward the v6 SPF next-hop | `test/ospfv3/ospfv3-sr-receive.ci` |
| a v6 adjacency reaches 2-Way/Full | -> | Adj-SID allocated from SRLB, advertised in E-Router-LSA, `mpls-fib` Pop installed | `TestOSPFv3OriginateAdjSID` (unit) + `ospfv3-sr-frr` interop |
| `show ipv6 ospf segment-routing` invoked | -> | snapshot lists SR-Algorithm, SRGB/SRLB, prefix-SIDs, adj-SIDs | `test/ospfv3/ospfv3-sr-show.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | SR enabled on the v6 family with `srgb 16000 8000` | this node originates an OSPFv3 RI Opaque LSA (area scope `0xA00C`) carrying SR-Algorithm (incl. Algorithm 0), one SID/Label Range TLV for the SRGB, and an SRLB TLV; the LSA floods area-wide and FRR `ospf6d` accepts it |
| AC-2 | A peer RI LSA carries SR-Algorithm + SRGB (two ranges) | this node records the peer's SR-Algorithm and builds its SRGB as the ordered concatenation of the two ranges; index N beyond range 0 maps into range 1 |
| AC-3 | A configured node prefix (loopback) with `prefix-sid index 5` | a Prefix-SID sub-TLV (type 4, flags per config, Algorithm 0, index 5) is emitted under the Intra-Area prefix carriage of a self-originated Extended LSA; FRR resolves it to label SRGB_base+5 |
| AC-4 | A received Extended prefix LSA with a Prefix-SID (index) for a prefix reachable in the v6 route table | this node computes `label = peer_SRGB_base + index` and installs an `mpls-fib` Push (ingress) toward the v6 SPF next-hop; an absolute-value (V=1) SID installs that label directly |
| AC-5 | The next-hop router advertised NP=0 for the Prefix-SID | this node applies PHP: the upstream pops the SID (no swap to a remote label, or pop+forward); NP=1/E=0 keeps the SID; NP=1/E=1 swaps to IPv6 Explicit NULL (label 2); M-flag set ignores NP and E (§6) |
| AC-6 | A v6 adjacency in state 2-Way or higher with SR enabled | an Adj-SID is allocated from the SRLB and advertised as an Adj-SID sub-TLV (type 5) under the Router-Link TLV in the E-Router-LSA; an `mpls-fib` Pop entry is installed for that local label toward the neighbour |
| AC-7 | A broadcast/NBMA v6 network with a non-DR neighbour | a LAN-Adj-SID sub-TLV (type 6, with the neighbour Router ID) is advertised under the Router-Link TLV for that neighbour |
| AC-8 | A v6 P2P adjacency transitions below 2-Way | the Adj-SID advertisement is withdrawn (the E-Router-LSA is re-originated without it) and the `mpls-fib` Pop entry is removed (§8.4.1) |
| AC-9 | A SID advertisement with an invalid V/L combination, or a Prefix-SID whose Algorithm the originator never advertised, or multiple Prefix-SIDs for the same prefix/topology/algorithm | each is ignored; no label is installed for it (§6); other valid SIDs are unaffected |
| AC-10 | An RI or Extended LSA with an invalid TLV/sub-TLV length, or a malformed/truncated SR TLV | the LSA is treated as malformed and ignored; the decoder never panics; a malformed-TLV counter increments (§10/§11) |
| AC-11 | A v6 ABR re-advertises an intra-area prefix into another area | the Inter-Area Prefix-SID is included per §8.2 (best path in source/backbone area), NP set / E clear unless the prefix is directly attached to the ABR |
| AC-12 | An Extended Prefix Range TLV (AF=1 IPv6) with a starting Prefix-SID and Range Size 4 | the four covered prefixes receive consecutive SIDs (start, start+1, ...); the IPv6 prefix decodes with `((PrefixLength+31)/32)` words; a duplicate Range TLV resolves to the smallest Instance ID |
| AC-13 | An RI / Extended LSA carrying an unknown (non-SR) TLV | the LSA is stored and reflooded byte-for-byte; the unknown TLV is not interpreted and does not block the SR TLVs in the same LSA |
| AC-14 | SR disabled (config removed) | no RI/Extended SR LSA is originated; existing SR-learned `mpls-fib` entries are withdrawn; the OSPFv3 base behaviour is exactly as before SR; the v4 engine and OSPFv2 SR are unaffected |
| AC-15 | Any RI / Extended SR LSA in any store | it never appears as a vertex in the v6 SPF graph and never changes the IPv6 route table directly (SR is data-plane only) |
| AC-16 | Configured SRGB or SRLB overlaps another range or the LDP/RSVP-TE label space | config validation rejects it; `ze doctor` reports the overlap before runtime install |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables OSPFv3 SR with an SRGB and a loopback prefix-SID | config -> `eng6` SR enabled -> `v6OriginateSelf` -> RI LSA + Prefix-SID E-prefix LSA -> flood; peer `show ipv6 ospf database` / FRR shows them | `test/ospfv3/ospfv3-sr-originate.ci` |
| 2 | Receives SR LSAs from FRR `ospf6d` and forwards via SR labels | wire -> v6 codec -> scope-store -> SR consumer -> label compute -> `mpls-fib` Push/Swap; `mpls -ls` shows the entry | `test/ospfv3/ospfv3-sr-receive.ci` + `ospfv3-sr-frr` interop |
| 3 | Brings up an adjacency and gets an Adj-SID | adjacency >= 2-Way -> SRLB alloc -> E-Router-LSA Adj-SID -> `mpls-fib` Pop; `show ipv6 ospf segment-routing` lists it | `ospfv3-sr-frr` interop (Adj-SID exchange) |
| 4 | Runs `show ipv6 ospf segment-routing` | CLI -> v6 SR snapshot -> SR-Algorithm, SRGB/SRLB, prefix-SIDs (with computed labels), adj-SIDs | `test/ospfv3/ospfv3-sr-show.ci` |
| 5 | Interops with FRR: both advertise SR, exchange Prefix-SIDs/Adj-SIDs, and program matching MPLS forwarding | DD/flood SR LSAs both ways -> both compute labels -> both program `mpls-fib`; end-to-end labelled ping over the SR path | `test/interop/scenarios/ospfv3-sr-frr/check.py` |
| 6 | Disables SR | config removed -> RI/Extended SR LSAs withdrawn -> `mpls-fib` SR entries removed; OSPFv3 base + OSPFv2 unaffected | `test/ospfv3/ospfv3-sr-disable.ci` + existing v3/v4 suites green |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOSPFv3SRTypeCodes` | `internal/plugins/ospf/v3/packet/sr_tlv_test.go` | R-1/AC-1: RFC 8666 OSPFv3 codes (Prefix-SID 4, Adj-SID 5, LAN-Adj-SID 6, SID/Label 7, Ext-Prefix-Range 9; RI `0xA00C`, E-Router `0x2021`) | |
| `TestOSPFv3SIDWidthFromVL` | `internal/plugins/ospf/v3/packet/sr_tlv_test.go` | R-2/AC-9: SID width 4 (V=0/L=0) vs 3 (V=1/L=1); invalid V/L ignored | |
| `TestOSPFv3SRTLVMalformed` | `internal/plugins/ospf/v3/packet/sr_tlv_test.go` | R-6/AC-10: truncated/over-length TLV never panics, marks LSA malformed | |
| `TestOSPFv3RILSARoundTrip` | `internal/plugins/ospf/v3/packet/lsa_routerinfo_test.go` | A-2/AC-1: RI LSA (SR-Algorithm+SRGB+SRLB) decode->re-encode byte-for-byte | |
| `TestOSPFv3ERouterLSARoundTrip` | `internal/plugins/ospf/v3/packet/lsa_extended_test.go` | A-2/AC-6: E-Router-LSA with Adj-SID/LAN-Adj-SID round-trips | |
| `TestOSPFv3ExtPrefixRangeTLVRoundTrip` | `internal/plugins/ospf/v3/packet/sr_tlv_test.go` | A-8/AC-12: Extended Prefix Range TLV (default/64/128) IPv6 word encoding round-trips; smallest Instance ID on duplicate | |
| `TestOSPFv3PrefixSIDCodec` | `internal/plugins/ospf/v3/packet/sr_tlv_test.go` | AC-3: Prefix-SID sub-TLV flags/algorithm/SID encode+decode | |
| `TestOSPFv3SRLSAScopeRouting` | `internal/plugins/ospf/lsdb/lsdb_linkscope_test.go` (or a v3-scope test) | A-3/AC-15: RI area/AS, E-Link link-local, E-AS-External AS route to the right store | |
| `TestOSPFv3SRGBMultiRangeIndex` | `internal/plugins/ospf/sr_v6_label_test.go` | R-3/AC-2: index resolves across two ranges in advertised order | |
| `TestOSPFv3LabelFromSRGBIndex` | `internal/plugins/ospf/sr_v6_label_test.go` | A-6/AC-4: `label = base + index` from a v6 SPF fixture; absolute V=1 direct | |
| `TestOSPFv3LabelOpFromFlags` | `internal/plugins/ospf/sr_v6_label_test.go` | R-4/AC-5: NP/E/M -> push/swap/PHP/Explicit-NULL(2) decision table | |
| `TestOSPFv3PrefixSIDIgnoreRules` | `internal/plugins/ospf/sr_v6_recv_test.go` | R-7/AC-9: algorithm-not-advertised, multiple-same-prefix, invalid-V/L ignored | |
| `TestOSPFv3PrefixSIDInstallsPush` / `TestOSPFv3AdjSIDInstallsPop` | `internal/plugins/ospf/sr_v6_install_test.go` | A-5/AC-4/AC-6: `mpls-fib` Push/Pop emitted with the right InLabel/OutLabels/NextHop/Source | |
| `TestOSPFv3OriginateRILSA` / `TestOSPFv3OriginatePrefixSID` / `TestOSPFv3OriginateAdjSID` | `internal/plugins/ospf/sr_v6_originate_test.go` | A-4/AC-1/AC-3/AC-6: RI/Prefix-SID/Adj-SID origination via `OriginateSelf` | |
| `TestOSPFv3AdjSIDWithdrawOnDown` | `internal/plugins/ospf/sr_v6_originate_test.go` | R-5/AC-8: adjacency < 2-Way withdraws the Adj-SID + removes the pop entry | |
| `TestOSPFv3InterAreaPrefixSID` | `internal/plugins/ospf/sr_v6_interarea_test.go` | AC-11: §8.2 propagation, NP set / E clear unless directly attached | |
| `TestOSPFv3SRGBAllocation` / `TestOSPFv3SRGBExhaustion` | `internal/plugins/ospf/sr_v6_alloc_test.go` | A-5/R-8/AC-16: bounded SRGB/SRLB allocation; exhaustion + overlap handling | |
| `TestOSPFv3UnknownRITLVReflooded` | `internal/plugins/ospf/v3/packet/lsa_routerinfo_test.go` | AC-13: unknown RI/Extended TLV stored + reflooded verbatim, not interpreted | |
| `TestOSPFv3SRLSANotInSPFGraph` | `internal/plugins/ospf/spf/spf_test.go` | A-9/AC-15: RI/Extended SR LSAs never become SPF vertices | |
| `TestOSPFv3BaseLSAsUnchanged` / `TestOSPFv2Unaffected` | `internal/plugins/ospf/v3/types/lsa_test.go`, `internal/plugins/ospf/instance_v6_test.go` | R-9/A-7/AC-14: adding the new types is additive; v4 + base v3 behaviour unchanged | |
| `TestOSPFv3SRConfigEnables` | `internal/plugins/ospf/config_test.go` | wiring/AC-1: `segment-routing` config resolves and enables SR on `eng6` | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| SID/Label sub-TLV value length | {3,4} octets | 4 | 2 (rejected) | 5 (rejected) |
| SID/Index/Label (label form, 20-bit) | 0-1048575 | 1048575 | N/A | a value > 20 bits is masked/rejected |
| Prefix-SID index | 0-(SRGB size-1) | SRGB size-1 | N/A | index >= total SRGB size -> no label (out of range) |
| SRGB / SRLB Range Size | 1-16777215 (24-bit) | 16777215 | 0 (MUST be > 0, rejected) | 2^24 (rejected) |
| MPLS label value (computed) | 16-1048575 (reserved 0-15) | 1048575 | 15 (reserved) | >20 bits |
| Extended Prefix Range Size | 1-(prefixes satisfiable by PrefixLength) | per PrefixLength | 0 | exceeds capacity (malformed) |
| IPv6 PrefixLength (Ext-Prefix-Range) | 0-128 | 128 | N/A | 129 |
| Prefix-SID Algorithm | 0-255 (only advertised values honoured) | 255 | N/A | unadvertised -> ignored |
| Adj-SID Weight | 0-255 | 255 | N/A | N/A (1 byte) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospfv3-sr-config` | `test/ospfv3/ospfv3-sr-config.ci` | `segment-routing` config validates; SR appears in `show ipv6 ospf` | |
| `ospfv3-sr-originate` | `test/ospfv3/ospfv3-sr-originate.ci` | RI LSA + Prefix-SID + Adj-SID originated; visible in `show ipv6 ospf database` | |
| `ospfv3-sr-receive` | `test/ospfv3/ospfv3-sr-receive.ci` | a received Prefix-SID installs an `mpls-fib` entry; `show ipv6 ospf segment-routing` lists the computed label | |
| `ospfv3-sr-show` | `test/ospfv3/ospfv3-sr-show.ci` | `show ipv6 ospf segment-routing` renders SR-Algorithm, SRGB/SRLB, prefix-SIDs, adj-SIDs | |
| `ospfv3-sr-disable` | `test/ospfv3/ospfv3-sr-disable.ci` | removing SR config withdraws the SR LSAs + `mpls-fib` entries; base OSPFv3 unaffected | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospfv3-sr-frr` | `test/interop/scenarios/ospfv3-sr-frr/` | FRR `ospf6d` with `segment-routing-srv6`/SR-MPLS enabled | Ze originates valid RFC 8666 RI + Extended SR LSAs FRR accepts; Ze parses FRR's SR LSAs, computes the same labels, and both program matching MPLS forwarding for an end-to-end labelled path | |

> Interop is required: this changes wire behaviour (new RI/Extended LSAs, SR TLVs)
> and programs the MPLS data plane. The raw-IPv6 / multicast / MPLS paths are
> Linux-only and run as QEMU integration tests (`ai/rules/qemu-testing.md`),
> consistent with the rest of the OSPFv3 interop set; the MPLS forwarding
> assertion uses `mpls -ls` inside the QEMU guest.

### Future (if deferring any tests)
- None. Every AC maps to a unit, functional, or interop test above. SRv6, TI-LFA, and the SR mapping-server role are out of scope (not deferred tests).

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*) -->
- `internal/plugins/ospf/v3/types/lsa.go` -- add the RI (`LSTypeRouterInfo` area `0xA00C`, AS `0xC00C`) and Extended-LSA type constants (`0x2021` E-Router, `0x2029` E-Intra-Area-Prefix, `0x2023` E-Inter-Area-Prefix, `0x4025` E-AS-External, `0x2027` E-Type-7, `0x0028` E-Link); widen `Known()`
- `internal/plugins/ospf/v3/packet/lsa.go` -- add `RouterInfo *RouterInfoLSA` and the Extended-LSA typed bodies to the union, `WriteTo`/`bodyLen`/`hasTypedBody` switches, and `Decode*` accessors
- `internal/plugins/ospf/origination_v6.go` -- SR origination off `v6OriginateSelf`: RI LSA (SR capabilities), E-Router-LSA Adj-SIDs, Prefix-SID E-prefix LSA; extend `v6OriginHeader` for the new types
- `internal/plugins/ospf/afstrategy_v6.go` -- §8.2 Inter-Area Prefix-SID propagation in `OriginateSummaries`/`v6OriginateSummaries`; expose per-next-hop neighbour identity if not already
- `internal/plugins/ospf/config.go` -- resolve the v6 `segment-routing` block (enable, SRGB, SRLB, per-prefix prefix-SID, flags) into `cfg.V6`
- `internal/plugins/ospf/register.go` -- start/stop SR on `eng6` from `cfg.V6.SegmentRouting`; register the SR show command + metrics
- `internal/plugins/ospf/cmd_show.go` -- `show ipv6 ospf segment-routing` backing data
- `internal/plugins/ospf/doctor.go` -- SRGB/SRLB/LDP-RSVP label-range overlap check (a new runtime dependency: the MPLS label space)
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- the `segment-routing` container under `address-family ipv6` (enable, srgb, srlb, prefix-sid list, flags)
- `internal/plugins/ospf/yang/ze-ospf-cmd.yang` -- the `show ipv6 ospf segment-routing` command binding
- `internal/core/diagnostic/codes.go` -- a diagnostic code for the SRGB/SRLB overlap doctor check

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- `segment-routing` under `address-family ipv6`; read `ai/rules/config-surface.md` + `ai/rules/config-naming.md` |
| YANG validation constraints | [ ] yes | SRGB/SRLB base = `uint32` `range "16..1048575"`, size = `uint32` `range "1..1048575"`; prefix-sid index `range "0..1048575"`; flags `boolean`; enable `boolean` |
| YANG custom validators | [ ] yes | a `ze:validate` for SRGB/SRLB non-overlap (with each other and with the LDP/RSVP-TE ranges); register in `validators_register.go` |
| CLI commands/flags | [ ] yes | `show ipv6 ospf segment-routing` in `ze-ospf-cmd.yang` + `cmd_show.go` |
| CLI grammar (action before identifier) | [ ] yes | `ai/rules/cli-grammar.md` -- `show ipv6 ospf segment-routing` |
| Editor autocomplete | [ ] yes | automatic for the YANG leaves + the new show subcommand |
| Functional test for new RPC/API | [ ] yes | `test/ospfv3/ospfv3-sr-*.ci` |
| Pipe completeness | [ ] yes | `show ipv6 ospf segment-routing` routes through `ApplyPipes` like the other show outputs |
| Env var registration | [ ] no | SR is operational config, not an `environment/` leaf |
| Doctor check for runtime dependencies | [ ] yes | the MPLS label space is a runtime dependency: a doctor check for SRGB/SRLB overlap with LDP/RSVP-TE and for `CONFIG_MPLS_ROUTING`; `internal/core/diagnostic/codes.go` + unit + functional test (`ai/rules/doctor-checks.md`) |
| Prometheus counters/metrics | [ ] yes | see the metrics rows below |

#### Metrics (new series owned by this spec)
| Metric | Type | Labels |
|--------|------|--------|
| `ze_ospfv3_sr_prefix_sids` | gauge | `algorithm` |
| `ze_ospfv3_sr_adj_sids` | gauge | `interface` |
| `ze_ospfv3_sr_labels_installed` | gauge | `op` (push/swap/pop) |
| `ze_ospfv3_sr_originations_total` | counter | `lsa` (ri/e-router/e-prefix) |
| `ze_ospfv3_sr_malformed_tlvs_total` | counter | `tlv` |
| `ze_ospfv3_sr_srgb_free` | gauge | (none) |

> These extend the umbrella's OSPFv3 metric set (`ze_ospfv3_*`); they are
> registered by this spec's SR code, not by the base v3 telemetry.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` -- OSPFv3 Segment Routing |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` -- the v6 `segment-routing` block |
| 3 | CLI command added/changed? | [ ] yes | `docs/guide/command-catalogue.md` -- `show ipv6 ospf segment-routing` |
| 4 | API/RPC added/changed? | [ ] no | the show RPC documents under the command catalogue |
| 5 | Plugin added/changed? | [ ] yes | `docs/guide/plugins.md` -- OSPF gains an OSPFv3 SR consumer + MPLS install |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/ospfv3.md` -- a Segment Routing section |
| 7 | Wire format changed? | [ ] yes | `docs/architecture/wire/ospfv3.md` -- RI LSA + Extended LSAs + SR TLVs |
| 8 | Plugin SDK/protocol changed? | [ ] no | no SDK change; the `mpls-fib` bus contract is unchanged (a new Source tag only) |
| 9 | RFC behavior implemented? | [ ] yes | `rfc/short/rfc8666.md` (+ `rfc8665.md` capability TLVs) -- flip the compliance-checklist items implemented |
| 10 | Test infrastructure changed? | [ ] yes (interop scenario added) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` -- OSPFv3 SR parity with FRR |
| 12 | Internal architecture changed? | [ ] yes | the OSPF subsystem doc -- v3 RI/Extended LSA carriage + SR label install |
| 13 | Route metadata keys added/changed? | [ ] no | SR installs `mpls-fib` label entries, not Loc-RIB prefix metadata |
| 14 | Prometheus counters added/changed? | [ ] yes | the OSPF telemetry doc -- the six `ze_ospfv3_sr_*` series |
| 15 | Registered plugin/event/command/capability inventory changed? | [ ] yes | `docs/plugin-overview.md` + the OSPFv3 metrics table |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors into the changed v3 packet/types/origination files |
| 17 | Existing docs show examples for this area? | [ ] check | verify any OSPFv3 config/CLI examples against the new `segment-routing` block |

## Files to Create
- `internal/plugins/ospf/v3/packet/lsa_routerinfo.go` -- the OSPFv3 RI Opaque LSA body + top-level TLV iterator (SR-Algorithm/SRGB/SRLB/SRMS) + verbatim passthrough for unknown RI TLVs
- `internal/plugins/ospf/v3/packet/lsa_extended.go` -- the RFC 8362 Extended-LSA bodies (E-Router-LSA Router-Link TLVs; the E-prefix LSAs' prefix TLVs) carrying SR sub-TLVs; verbatim passthrough for unknown TLVs
- `internal/plugins/ospf/v3/packet/sr_tlv.go` -- the SR TLV/sub-TLV codecs: SID/Label, Prefix-SID, Adj-SID, LAN-Adj-SID, Extended Prefix Range; V/L width inference; bound-checked iteration
- `internal/plugins/ospf/sr_v6.go` -- the v6 SR engine glue: enable/disable on `eng6`, peer-capability store, the origination trigger, the reception/install trigger off the v6 SPF result
- `internal/plugins/ospf/sr_v6_label.go` -- SRGB index->label resolution + the NP/E/M push/swap/PHP decision
- `internal/plugins/ospf/sr_v6_alloc.go` -- the bounded SRGB/SRLB 20-bit label allocator (LDP pool pattern)
- `internal/plugins/ospf/sr_v6_install.go` -- the `mpls-fib` Push/Swap/Pop emit (`mplsSourceOSPFv3SR`)
- `internal/plugins/ospf/v3/packet/sr_tlv_test.go`, `lsa_routerinfo_test.go`, `lsa_extended_test.go`
- `internal/plugins/ospf/sr_v6_label_test.go`, `sr_v6_recv_test.go`, `sr_v6_install_test.go`, `sr_v6_originate_test.go`, `sr_v6_interarea_test.go`, `sr_v6_alloc_test.go`
- `test/ospfv3/ospfv3-sr-config.ci`, `ospfv3-sr-originate.ci`, `ospfv3-sr-receive.ci`, `ospfv3-sr-show.ci`, `ospfv3-sr-disable.ci`
- `test/interop/scenarios/ospfv3-sr-frr/` -- `ze.conf`, `frr.conf`, `check.py`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan -- confirm the base v3 codec, `OriginateSelf`, the `mpls-fib` bus, and the LDP allocator pattern exist; confirm NO RI/Extended LSA exists yet |
| 3. Wiring phase | Wiring Test table -- SR config + RI/Extended type stubs + failing wiring tests |
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

<!-- Phase 1 is ALWAYS wiring: create the entry point and a failing wiring test. -->
Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- SR config + entry points + failing wiring tests
   - Tests: `TestOSPFv3SRConfigEnables`, `test/ospfv3/ospfv3-sr-config.ci`
   - Files: `yang/ze-ospf-conf.yang` (`segment-routing` block), `config.go` (resolve into `cfg.V6`), `register.go` (start/stop SR on `eng6`), `sr_v6.go` (SR engine skeleton), the new LS-type constants in `v3/types/lsa.go` as stubs
   - Verify: SR config resolves and toggles SR state on `eng6`; origination/reception/install are stubs so the deeper tests still fail
2. **Phase: RI + Extended LSA carriage + SR TLV codecs** -- the wire primitives
   - Tests: `TestOSPFv3SRTypeCodes`, `TestOSPFv3SIDWidthFromVL`, `TestOSPFv3SRTLVMalformed`, `TestOSPFv3RILSARoundTrip`, `TestOSPFv3ERouterLSARoundTrip`, `TestOSPFv3ExtPrefixRangeTLVRoundTrip`, `TestOSPFv3PrefixSIDCodec`, `TestOSPFv3UnknownRITLVReflooded`, `TestOSPFv3SRLSAScopeRouting`, `TestOSPFv3BaseLSAsUnchanged`
   - Files: `v3/types/lsa.go` (types + `Known()`), `v3/packet/lsa.go` (union + switches), `v3/packet/lsa_routerinfo.go`, `lsa_extended.go`, `sr_tlv.go`
   - Verify: RI/Extended LSAs round-trip; SR TLVs encode/decode with V/L width inference; malformed input never panics; new types scope-route correctly; base LSAs unchanged
3. **Phase: SRGB/SRLB allocation + label computation** -- the SR math
   - Tests: `TestOSPFv3SRGBAllocation`, `TestOSPFv3SRGBExhaustion`, `TestOSPFv3SRGBMultiRangeIndex`, `TestOSPFv3LabelFromSRGBIndex`, `TestOSPFv3LabelOpFromFlags`
   - Files: `sr_v6_alloc.go`, `sr_v6_label.go`
   - Verify: bounded allocation; index->label across ranges in order; NP/E/M decision table exact (Explicit-NULL = IPv6 label 2)
4. **Phase: Reception + install** -- peer capability + `mpls-fib`
   - Tests: `TestOSPFv3PrefixSIDIgnoreRules`, `TestOSPFv3PrefixSIDInstallsPush`, `TestOSPFv3AdjSIDInstallsPop`, `ospfv3-sr-receive.ci`
   - Files: `sr_v6.go` (capability store + reception trigger off the v6 SPF result), `sr_v6_install.go`
   - Verify: peer SRGB recorded; §6 ignore rules; Push/Swap/Pop emitted with the right fields + Source
5. **Phase: Origination + withdraw** -- RI/Adj-SID/Prefix-SID
   - Tests: `TestOSPFv3OriginateRILSA`, `TestOSPFv3OriginatePrefixSID`, `TestOSPFv3OriginateAdjSID`, `TestOSPFv3AdjSIDWithdrawOnDown`, `ospfv3-sr-originate.ci`
   - Files: `origination_v6.go` (hooks off `v6OriginateSelf`, `v6OriginHeader`), `sr_v6.go`
   - Verify: RI/Adj-SID/Prefix-SID originate via `OriginateSelf`; Adj-SID withdraws on adjacency down + removes the pop entry
6. **Phase: Inter-area Prefix-SID propagation** -- §8.2
   - Tests: `TestOSPFv3InterAreaPrefixSID`, `TestOSPFv3SRLSANotInSPFGraph`
   - Files: `afstrategy_v6.go` (`OriginateSummaries`), `sr_v6.go`
   - Verify: Prefix-SID carried across areas with NP set / E clear unless directly attached; SR LSAs never enter SPF
7. **Phase: CLI + doctor + metrics + disable** -- user surface
   - Tests: `ospfv3-sr-show.ci`, `ospfv3-sr-disable.ci`, the doctor unit/functional test
   - Files: `cmd_show.go`, `yang/ze-ospf-cmd.yang`, `doctor.go`, `internal/core/diagnostic/codes.go`, metric registration, `register.go`
   - Verify: `show ipv6 ospf segment-routing`; SRGB/SRLB overlap doctor check; SR disable withdraws LSAs + entries; six metric series
8. **Functional tests** -> the five `.ci` cover the user-visible behaviour
9. **RFC refs** -> add `// RFC 8666 Section X` / `// RFC 8665 Section X` comments on the type codes, V/L width, NP/E/M decision, Adj-SID withdraw, and §8.2 propagation
10. **Interop** -> `ospfv3-sr-frr` QEMU scenario (SR LSA exchange + matching MPLS forwarding via `mpls -ls`)
11. **Full verification** -> `make ze-verify`
12. **Complete spec** -> audit tables + learned summary; two commits (A: code+spec+learned, B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has file:line implementation |
| Feature completeness | each user story has a working path; SR parity with FRR `ospf6d` SR-MPLS (capabilities + Prefix-SID + Adj-SID + label install); SRv6/TI-LFA/mapping-server excluded by design |
| Correctness | RFC 8666 OSPFv3 type codes (not RFC 8665 values); V/L width inference; SRGB multi-range index; NP/E/M -> push/swap/PHP/Explicit-NULL(2); Adj-SID withdraw < 2-Way; §8.2 propagation; §6 ignore rules |
| Naming | `ze_ospfv3_sr_*` metrics; YANG `segment-routing`/`srgb`/`srlb`/`prefix-sid` kebab-case; `mplsSourceOSPFv3SR` |
| Data flow | SR touches v3 codec + LSDB store (by scope) + the SR consumer + `mpls-fib`; SPF read-only; no foreign-plugin name in SR code; v4 engine untouched |
| CLI grammar | `show ipv6 ospf segment-routing` action-before-identifier |
| Doctor checks | SRGB/SRLB/LDP-RSVP overlap + MPLS routing capability registered |
| YANG validation | every SR leaf has native range/boolean constraints; the non-overlap custom validator present |
| Prometheus counters | the six `ze_ospfv3_sr_*` series defined, registered, listed |
| Rule: plugin-self-containment | SR removal removes the TLVs, the RI/Extended carriage, the `mpls-fib` entries, the config, the show command, the metrics; no SR spelling in generic mplsfib/sysrib code |
| Rule: buffer-first | SR TLV builder + RI/Extended encode write into caller buffers; iterators zero-copy; IPv6 word pad written |
| Rule: no shared code with ext-5 | grep confirms no OSPFv2-SR type/struct is imported by the v3 SR code, and vice-versa |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| RI + Extended LSA carriage | `go test ./internal/plugins/ospf/v3/packet -run 'RILSA|ExtendedLSA|SRTLV'` |
| SR type codes are RFC 8666 OSPFv3 values | `go test ./internal/plugins/ospf/v3/packet -run TestOSPFv3SRTypeCodes` |
| Label computation + decision table | `go test ./internal/plugins/ospf -run 'SRGB|LabelOp|LabelFromSRGB'` |
| `mpls-fib` install | `go test ./internal/plugins/ospf -run 'InstallsPush|InstallsPop'` + QEMU `mpls -ls` |
| SR origination/withdraw | `go test ./internal/plugins/ospf -run 'OriginateRI|OriginateAdjSID|AdjSIDWithdraw'` |
| Six metric series registered | `grep -rn 'ze_ospfv3_sr_' internal/plugins/ospf` |
| Doctor overlap check | `go test ./internal/plugins/ospf -run Doctor` + `ze doctor` output |
| Interop scenario present | `ls test/interop/scenarios/ospfv3-sr-frr/` |
| Functional tests present | `ls test/ospfv3/ospfv3-sr-*.ci` |
| No shared code with OSPFv2 SR | `grep -rn 'rfc8665\|ext-5\|v2.*PrefixSID' internal/plugins/ospf/sr_v6*.go internal/plugins/ospf/v3` returns nothing |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | RI/Extended/SR TLV iteration bound-checked; invalid length -> LSA ignored, never panic; the v3 fuzz target extended with RI/Extended bodies |
| Resource exhaustion | the SR per-router capability store + the SRGB/SRLB allocator are bounded; a flood of SR LSAs cannot grow memory unbounded or exhaust labels beyond the configured ranges |
| MPLS data-plane integrity | SR programs forwarding directly (no LDP/RSVP signalling), so a misrouted SID misroutes traffic and is hard to detect (§11); only labels from a configured SRGB/SRLB are installed; the doctor check guards against range overlap with LDP/RSVP-TE |
| Trust boundary | SR honours the existing OSPFv3 RFC 7166 authentication; an SR-capable router accepts SIDs only for prefixes the v6 SPF already reaches; no new auth surface |
| Error leakage | malformed-TLV handling counts `ze_ospfv3_sr_malformed_tlvs_total` and logs rate-limited, not per-packet (§11 DoS avoidance) |

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
OSPFv3 SR is a *carriage + consumer* problem layered on a base codec that lacks
the carriage. Unlike OSPFv2 SR (which had delivered RI and Extended-Prefix/Link
carriers to plug into), OSPFv3 has neither the RFC 7770 RI LSA nor the RFC 8362
Extended LSAs, so this spec adds exactly the SR subset of both, then mirrors the
OSPFv2 SR label-computation and `mpls-fib` install logic against the new
RFC-8666-specific carriage. The base v3 `LSA` verbatim-passthrough and the
scope-encoded LS Type mean the *carriage* is mostly new typed bodies + scope
routing that the existing LSDB already honours, and the *install* reuses the
`mpls-fib` bus and the LDP allocator pattern unchanged.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Add the SR subset of RFC 7770 RI + RFC 8362 Extended LSAs in this spec | a separate RI/Extended-LSA spec first, then SR on top | OSPFv3 has no other consumer of the RI/Extended carriage yet; bundling avoids a carrier with no user; scope is bounded to what SR needs, with verbatim passthrough for the rest |
| Run SR on the v6 engine instance, no shared code with OSPFv2 SR | a shared SR core across v2/v3 | RFC 8666 vs RFC 8665 differ in type codes, carriage, and prefix encoding; sharing would leak version branches; the two families already run as independent engine instances |
| Install via the `mpls-fib` bus + the LDP allocator pattern | a new OSPF-owned netlink path / allocator | fib-kernel is the single netlink owner; LDP/RSVP-TE already prove the bus + 20-bit allocator; reuse keeps stale-sweep, metrics, and conflict detection uniform |
| New typed bodies on the existing `LSA` union, not a new codec | a parallel Extended-LSA codec | the union + verbatim passthrough already handle unknown LSAs; adding bodies is additive and inherits flood/aging/scope routing |
| SR is data-plane only, never an SPF vertex | feed SR into path computation | RFC 8666 SR programs forwarding labels for paths SPF already computed; topology stays RFC-5340-driven (Algorithm 0) |

## Known Limitations
- SRv6 (IPv6 data-plane SIDs) is out of scope; RFC 8666 §1 defers it to a separate document. Only the MPLS data plane is programmed.
- TI-LFA / backup-path computation is out of scope; the Adj-SID B-flag may be set for eligibility but no backup path is computed.
- The SR mapping-server *server* role is out of scope: Ze honours received mapping-server SIDs (M-flag, Extended Prefix Range TLV) but does not originate Range TLVs or inject NU-bit prefixes.
- Only the SR subset of the RI / Extended-LSA frameworks is interpreted; other RI/Extended TLVs are passed through verbatim, not understood.
- Only SR-Algorithm 0 paths are computed; Algorithm 1 (Strict-SPF) SIDs may be recorded but no separate SPF is run for them.

## RFC Documentation

Add `// RFC 8666 Section X.Y: "<quoted requirement>"` (and `// RFC 8665 Section X`
for the reused capability TLVs) above the enforcing code.
MUST document: the OSPFv3 type codes vs OSPFv2; V/L width inference; the NP/E/M
push/swap/PHP/Explicit-NULL decision; the §6 ignore rules; the Adj-SID withdraw
on adjacency < 2-Way; the §8.2 inter-area Prefix-SID propagation; the malformed-TLV
no-crash requirement.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered -- add test for each]

### Documentation Updates
- [Docs updated, with source anchors named, or "None" with grep evidence]
- [If docs were changed: `make ze-doc-test` result]

### Deviations from Plan
- [Differences from original plan and why]

## Implementation Audit

<!-- BLOCKING: Complete BEFORE writing learned summary. See rules/implementation-audit.md -->

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
| Advertise OSPFv3 SR capabilities (SR-Algorithm/SRGB/SRLB) in the RI LSA | interop test | `ospfv3-sr-frr` -- FRR `ospf6d` accepts Ze's RI SR TLVs |
| Carry Prefix-SID / Adj-SID / LAN-Adj-SID in the Extended LSAs | interop + unit | `ospfv3-sr-frr` SID exchange; `TestOSPFv3SRTypeCodes`, round-trip tests |
| Compute MPLS labels from advertised Prefix-SID indices against the originator SRGB | unit + functional | `TestOSPFv3LabelFromSRGBIndex`, `TestOSPFv3SRGBMultiRangeIndex`, `ospfv3-sr-receive.ci` |
| Program SR forwarding (push/swap/pop) | QEMU interop | `ospfv3-sr-frr` `mpls -ls` shows the SR entries; end-to-end labelled path |
| Allocate SRGB/SRLB | unit | `TestOSPFv3SRGBAllocation`, `TestOSPFv3SRGBExhaustion` |
| CLI + metrics | functional | `ospfv3-sr-show.ci`; `ze_ospfv3_sr_*` series present |
| No shared code with OSPFv2 SR; v4 untouched | grep + suites | deliverables grep; `TestOSPFv2Unaffected` + existing v4/v3 suites green |

## Review Gate

<!-- BLOCKING (rules/planning.md Completion Checklist step 7): -->
<!-- Run /ze-review BEFORE the final testing/verify step. Record the findings here. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

<!-- BLOCKING: Do NOT trust the audit above. Re-verify everything independently. -->

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
- [ ] AC-1..AC-16 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` -- no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-ospfv3-ext-6-segment-routing.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospfv3-ext-6-segment-routing.md` only (preserves the edited spec in git history from commit A)
