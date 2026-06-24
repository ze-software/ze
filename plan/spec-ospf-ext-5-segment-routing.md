# Spec: ospf-ext-5 -- OSPFv2 Segment Routing (RFC 8665)

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-ospf-ext-3-router-information.md, spec-ospf-ext-4-extended-link-prefix.md |
| Phase | - |
| Updated | 2026-06-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `rfc/short/rfc8665.md` -- the SR wire spec: SR-Algorithm/SID-Label-Range(SRGB)/SRLB/SRMS-Pref TLVs in the RI LSA (§3), Extended Prefix Range TLV (§4), Prefix-SID Sub-TLV + V/L/NP/E/M flags (§5), Adj-SID (§6.1) + LAN-Adj-SID (§6.2) Sub-TLVs, the SID/Label Sub-TLV (§2.1, 3-octet label vs 4-octet index), label computation from SRGB index (§3.2), Adj-SID withdraw on adjacency < 2-Way (§7.4.1)
4. `rfc/short/rfc7770.md` -- the RI LSA carrier (Opaque Type 4) that holds the SR-Algorithm/SRGB/SRLB/SRMS top-level TLVs; first-instance rules; multi-instance tie-break (§3)
5. `rfc/short/rfc7684.md` -- the Extended Prefix (Opaque Type 7) and Extended Link (Opaque Type 8) Opaque LSAs that hold the Prefix-SID and Adj-SID/LAN-Adj-SID sub-TLVs; generic 4-octet-aligned TLV format
6. `plan/spec-ospf-ext-1-opaque-framework.md` -- the RFC 5250 carrier: `RegisterOpaqueConsumer`, the generic TLV iterator/builder (`packet/opaque_tlv.go`), scope-correct flooding, the Opaque Type/ID split. SR TLVs are emitted/parsed through this carrier's helpers
7. `plan/spec-ospf-ext-3-router-information.md` (dependency) -- the RI LSA originator + RI TLV codec (Opaque Type 4) into which this spec injects the SR-Algorithm/SRGB/SRLB/SRMS top-level TLVs
8. `plan/spec-ospf-ext-4-extended-link-prefix.md` (dependency) -- the Extended Prefix (Type 7) + Extended Link (Type 8) Opaque LSA originators/decoders into which this spec injects the Prefix-SID and Adj-SID/LAN-Adj-SID sub-TLVs
9. `internal/core/mplsfib/events.go` -- the `(mpls-fib, entry)` bus: `Entry{Op: Push/Swap/Pop, Action: Add/Remove, InLabel, FEC, OutLabels, NextHop, Source}`, `EntryChange.Emit`; fib-kernel is the single netlink owner
10. `internal/plugins/ldp/fib.go` + `internal/plugins/ldp/lib.go` -- the model for label-pool allocation (`allocateLabelLocked`, `nextLabel`, `MaxLabel`) and MPLS install via the mpls-fib bus (`ProgramPush`/`ProgramPop`, `mplsSourceLDP`)
11. `internal/plugins/ospf/spf/computer.go` + `spf/route.go` + `spf/graph.go` -- the SPF `Computer.Run()` that yields `[]RouteEntry` (prefix, metric, type, origin RouterID, next-hops) and the `Graph`/`RouterVertex`/`Result.Nodes` that expose per-vertex next-hops and neighbour adjacencies for Prefix-SID/Adj-SID label install

## Task

Add OSPFv2 Segment Routing (RFC 8665) to the native OSPFv2 plugin at
`internal/plugins/ospf/` as a new opaque-LSA consumer built on top of three
delivered carriers: the RFC 5250 opaque framework (ext-1), the RFC 7770 Router
Information LSA (ext-3), and the RFC 7684 Extended Prefix / Extended Link Opaque
LSAs (ext-4). SR is the first OSPF consumer that programs the MPLS data plane:
it computes MPLS labels from advertised Prefix-SID indices against the
originator's SRGB and installs label-switched forwarding entries for prefix-SIDs
(node SIDs) and adjacency-SIDs through the existing `mpls-fib` bus, the same
seam LDP and RSVP-TE use.

Origination: when SR is enabled, this node advertises its SR-Algorithm,
SRGB (one or more SID/Label Range TLVs), SRLB, and (optionally) SRMS-Preference
top-level TLVs in the RI LSA (ext-3); a Prefix-SID Sub-TLV under the Extended
Prefix TLV (ext-4) for each configured node prefix (typically the loopback); and
an Adj-SID Sub-TLV (and LAN-Adj-SID on broadcast/NBMA) under the Extended Link
TLV (ext-4) for each adjacency in state 2-Way or higher.

Reception + forwarding: when an RI LSA from a remote router carries SR TLVs,
this node records that router's SR-Algorithm and SRGB (the ordered concatenation
of its ranges). When an Extended Prefix Opaque LSA carries a Prefix-SID for a
prefix the SPF route table can reach, this node computes the outgoing MPLS label
from the originator's SRGB (`label = SRGB_base + index`, resolving across ranges
in advertised order), honours the next-hop router's NP/E/M flags to decide
push/swap/PHP, and installs an MPLS push (ingress) or swap (transit) entry toward
the SPF next-hop. When an Extended Link Opaque LSA carries an Adj-SID/LAN-Adj-SID,
this node (as the advertiser) installs the corresponding pop/forward entry keyed
by the local label it allocated for that adjacency.

SRGB/SRLB management: the SRGB is a configured contiguous global label range
this node owns; the SRLB is a configured local label range from which Adj-SIDs
are allocated. Allocation reuses the LDP label-pool pattern (a bounded 20-bit
allocator) rather than inventing a new mechanism.

The consumer is fully self-contained: it registers SR TLV emitters/parsers with
ext-3 and ext-4, registers the SR config leaves, the `show ip ospf segment-routing`
CLI, the doctor checks, and the metrics. Removing the SR consumer removes all SR
behaviour; OSPF and the opaque carriers behave exactly as before.

### In scope (this spec)

| Item | Detail |
|------|--------|
| SR-Algorithm TLV origination + reception | Type 8 top-level TLV of the RI LSA (ext-3); advertise Algorithm 0 (SPF) when SR enabled; record remote routers' algorithms; area-scoped flooding (§3.1) |
| SRGB (SID/Label Range TLV) origination + reception | Type 9 top-level TLV of the RI LSA (ext-3), MAY appear multiple times; each range carries exactly one SID/Label Sub-TLV (the first label); receiver concatenates ranges in advertised order to map index -> label (§3.2) |
| SRLB (SR Local Block TLV) origination + reception | Type 14 top-level TLV of the RI LSA (ext-3); the local label range from which Adj-SIDs are allocated (§3.3) |
| SRMS-Preference TLV origination + reception | Type 15 top-level TLV of the RI LSA (ext-3); optional; first-occurrence + narrowest-scope tie-break (§3.4) |
| Prefix-SID Sub-TLV origination + reception | Type 2 sub-TLV under the Extended Prefix TLV (ext-4) and Extended Prefix Range TLV; NP/M/E/V/L flags; index (4-octet, V=0/L=0) or local label (3-octet, V=1/L=1); algorithm; MT-ID (§5) |
| Extended Prefix Range TLV origination + reception | Type 2 top-level TLV of the Extended Prefix Opaque LSA (ext-4); IA-Flag set by ABR between areas; carries Prefix-SID sub-TLVs for SR Mapping Server / range advertisement (§4) |
| Adj-SID Sub-TLV origination + reception | Type 2 sub-TLV under the Extended Link TLV (ext-4); B/V/L/G/P flags; weight; allocated from the SRLB; withdrawn when adjacency drops below 2-Way (§6.1, §7.4.1) |
| LAN-Adj-SID Sub-TLV origination + reception | Type 3 sub-TLV under the Extended Link TLV (ext-4); carries the Neighbor ID; broadcast/NBMA only (§6.2, §7.4.2) |
| SID/Label Sub-TLV codec | Type 1; 3-octet (20-bit MPLS label) or 4-octet (32-bit SID) forms (§2.1); shared between SRGB/SRLB and the prefix/adj sub-TLVs |
| SRGB/SRLB label-range management | Configured SRGB (global) and SRLB (local) ranges this node owns; a bounded 20-bit allocator for Adj-SID local labels reusing the LDP `nextLabel`/`MaxLabel` pattern |
| Label computation from index | `label = SRGB_base + index` resolved across the originator's advertised ranges in order; reject out-of-range index; honour V=1/L=1 absolute local-label form (§3.2, §5) |
| MPLS forwarding install for Prefix-SID | Push (ingress, SR not yet on stack) or swap (transit) toward the SPF next-hop via the `mpls-fib` bus; NP=0 -> PHP (pop at penultimate), E=1 -> Explicit NULL, M-flag ignores NP/E (§5) |
| MPLS forwarding install for Adj-SID | Pop/forward entry keyed by the local Adj-SID label this node allocated; forwarded to the specific adjacency (bypassing SPF) (§6.1) |
| CLI + metrics | `show ip ospf segment-routing` (SRGB/SRLB, prefix-SIDs, adj-SIDs, computed labels); SR-specific `ze_ospf_sr_*` counters/gauges |

### Out of scope (noted so it is not silently assumed done)

| Item | Where |
|------|-------|
| TI-LFA / topology-independent loop-free alternates | ospf-ext-6 (backup paths; B-Flag is advertised but no backup is computed here) |
| SR-TE policies (segment lists, BSID) | BGP SR-Policy is a separate subsystem; OSPF SR carries only the building-block SIDs |
| SRv6 (IPv6 SR) | not applicable to OSPFv2 (IPv4 data plane only, RFC 8665 §1); OSPFv3 SR is RFC 8666 (separate) |
| RFC 8661 SR Mapping Server full semantics | only the wire carriage (SRMS-Pref TLV, Extended Prefix Range TLV, M-Flag) is implemented; mapping-server preference arbitration beyond first-occurrence/narrowest-scope is deferred |
| Strict-SPF (Algorithm 1) path computation | the SR-Algorithm TLV records Algorithm 1 if a peer advertises it, but Ze computes only Algorithm 0 (standard SPF); a Prefix-SID for an algorithm Ze does not compute is recorded but not installed |
| The opaque carrier, RI LSA codec, Extended Prefix/Link LSA codec | ext-1 / ext-3 / ext-4 (this spec consumes them; it does NOT re-implement TLV iteration, RI origination, or Extended LSA origination) |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -->
- [ ] `docs/research/ospf-implementation-guide.md` §14 ("Segment Routing (RFC 8665)", ~1534-1537) -- FRR "advertises prefix-SIDs and adjacency-SIDs, allocates SRGB/SRLB, integrates with the MPLS forwarding plane"
  -> Decision: model SR as an opaque-LSA consumer (like FRR's `ospf_sr.c`), layered on RI (ext-3) and Extended Prefix/Link (ext-4), NOT as a change to the OSPF core; the only OSPF-core touch point is reading the SPF route table + adjacency set for next-hops and neighbour IDs
  -> Constraint: SR "integrates with the MPLS forwarding plane" -- the integration is install-only through the existing `mpls-fib` bus; OSPF SR never touches netlink (fib-kernel owns it)
- [ ] `internal/plugins/ldp/fib.go` (the MPLS install model) -- `ProgramPush`/`ProgramPop`/`Remove` emit `mplsfibevents.Entry` with a source tag; `mplsSourceLDP=2`, RSVP-TE uses 1
  -> Decision: SR allocates a distinct `mplsSourceOSPFSR` source tag; SR programs push (ingress) and swap (transit) for prefix-SIDs and pop (egress) for adj-SIDs, exactly mirroring LDP's three operations
  -> Constraint: implicit-null (3) signals PHP -- a Prefix-SID with NP=0 means the penultimate hop pops, so the SR install for a directly-attached SR egress neighbour must forward as plain IP (no push), the same rule LDP applies to implicit-null
- [ ] `internal/plugins/ldp/lib.go` (the label-pool model) -- `LIB.allocateLabelLocked`/`nextLabel`/`MaxLabel` is a bounded 20-bit allocator skipping used labels
  -> Decision: the SRLB Adj-SID allocator reuses this bounded-allocator shape (a local label pool seeded at the SRLB base, capped at the SRLB end), NOT a new allocator abstraction
  -> Constraint: the SRGB is NOT dynamically allocated -- it is a configured contiguous range this node owns and advertises verbatim; only the SRLB drives per-adjacency allocation
- [ ] `ai/rules/plugin-self-containment.md` -- the SR consumer is self-contained
  -> Constraint: no SR-specific spelling (Prefix-SID, Adj-SID, SRGB) appears in ext-1/ext-3/ext-4 or in the OSPF core; SR registers its TLV emitters/parsers, config, CLI, doctor, and metrics from its own `init()`; removing the SR files removes all SR behaviour
- [ ] `ai/rules/buffer-first.md` -- SR TLV emit is buffer-first
  -> Constraint: SR sub-TLV/TLV emission uses the ext-1 TLV builder (`WriteTo(buf, off) int`, 4-octet pad written explicitly); the SID/Label field (3 or 4 octets) is written into the caller buffer, never via slice concatenation; the index-to-label computation is integer arithmetic, no allocation
- [ ] `ai/rules/no-sprintf-alloc.md` -- no `fmt`/`+` on the wire or hot path
  -> Constraint: `show ip ospf segment-routing` rendering uses `textbuf`/`AppendTo`; the label/index arithmetic on the SPF/forwarding hot path allocates nothing
- [ ] `ai/rules/memory-architecture.md` -- value-typed cross-boundary payloads
  -> Constraint: the SR forwarding entries handed to the `mpls-fib` bus are value-typed (`mplsfibevents.Entry`, fixed-size fields, an owned `OutLabels` slice), carrying no pointer into SR state; the SR consumer copies any slice it retains

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc8665.md` -- the SR wire + behaviour spec
  -> Constraint: §3.1 -- if the SR-Algorithm TLV is advertised it MUST include Algorithm 0; multiple SR-Algorithm TLVs from one router resolve to the first occurrence, area-scoped over AS-scoped, then smallest Instance ID; SR-Algorithm/SRGB/SRLB are area-scoped (REQUIRED)
  -> Constraint: §3.2 -- Range Size MUST be > 0; each SID/Label Range TLV MUST contain exactly one SID/Label Sub-TLV (ignore the range TLV if more than one); ranges MUST NOT overlap; the receiver MUST build the index->label map by concatenating ranges in advertised order; the originator MUST keep range order stable across graceful restart
  -> Constraint: §3.3 -- SRLB Range Size MUST be > 0; exactly one SID/Label Sub-TLV; SRLB ranges MUST NOT overlap; Adj-SIDs are allocated from the SRLB
  -> Constraint: §3.4 -- SRMS-Preference: first occurrence, narrowest flooding scope, then smallest Instance ID; SHOULD use AS-scoped flooding
  -> Constraint: §4 -- all prefix ranges in one Extended Prefix Opaque LSA MUST share flooding scope; an ABR propagating the Extended Prefix Range TLV between areas MUST set the IA-Flag; Range Size MUST NOT exceed the prefixes satisfiable by Prefix Length excluding 224.0.0.0/3
  -> Constraint: §5 -- only V=0/L=0 (4-octet index) and V=1/L=1 (3-octet local label) are valid; any other V/L combination MUST cause the SID Advertisement to be ignored; a Prefix-SID whose algorithm is not in the originator's SR-Algorithm TLV MUST be ignored; multiple Prefix-SIDs for the same prefix/topology/algorithm MUST all be ignored; the outgoing label MUST honour the next-hop router's NP/E/M flags (M set -> ignore NP/E); NP MUST be set + E clear for ABR inter-area and ASBR redistributed prefix-SIDs unless directly attached
  -> Constraint: §6.1 -- Adj-SID flags B/V/L/G/P; reserved bits MUST be zero; P set -> persistent; an Adj-SID MAY be advertised for any P2P adjacency at 2-Way or higher
  -> Constraint: §7.4.1 -- when a P2P adjacency transitions below 2-Way the Adj-SID Advertisement MUST be withdrawn from the area
  -> Constraint: §9/§10 -- an invalid TLV/sub-TLV length means the LSA is malformed and MUST be ignored; malformed TLVs MUST NOT crash the router; reception SHOULD be counted/logged with rate limiting
- [ ] `rfc/short/rfc7770.md` -- the RI LSA carrier (ext-3 dependency)
  -> Constraint: §2.1/§2.7 -- the SR top-level TLVs ride the RI LSA (Opaque Type 4) at area scope (9/10/11 select scope; SR uses area scope 10, SRMS-Pref MAY use AS scope 11); the multi-instance tie-break (§3, smallest Instance ID) governs SR-Algorithm and SRMS-Pref when a router floods more than one RI LSA
- [ ] `rfc/short/rfc7684.md` -- the Extended Prefix/Link carriers (ext-4 dependency)
  -> Constraint: §2.1 -- the Prefix-SID rides as a sub-TLV under the Extended Prefix TLV (Type 1) of the Extended Prefix Opaque LSA (Opaque Type 7); §3.1 -- the Adj-SID/LAN-Adj-SID ride as sub-TLVs under the Extended Link TLV (Type 1) of the Extended Link Opaque LSA (Opaque Type 8, area scope only); §5 -- a malformed TLV/sub-TLV makes the whole LSA malformed: MUST NOT be stored, acked, or reflooded; the lowest-Opaque-ID instance wins for a duplicated prefix/link

**Key insights:**
- SR is a *consumer*, not a carrier: it never parses an LSA header or floods anything itself. It registers TLV emitters/parsers with ext-3 (RI) and ext-4 (Extended Prefix/Link) and receives decoded TLV bodies. The work is (a) the SR TLV/sub-TLV codec, (b) SRGB/SRLB management + index->label arithmetic, (c) the forwarding install through `mpls-fib`.
- The MPLS data-plane integration already exists as a clean seam: `internal/core/mplsfib` (`Entry{Op,Action,InLabel,FEC,OutLabels,NextHop,Source}`) with fib-kernel as the single netlink owner. SR is the third producer (after RSVP-TE source=1, LDP source=2).
- Label computation is pure arithmetic over the originator's ordered SRGB ranges: index N maps to the (N - sum-of-prior-range-sizes)-th label of the range that contains it. This is the single most error-prone piece (multi-range ordering) and gets dedicated boundary tests.
- The SPF route table (ext-8/9, `spf.Computer`) already gives the next-hop and the route type per prefix; the SPF graph gives the neighbour Router IDs per adjacency. SR reads both (read-only) to resolve push/swap next-hops and Adj-SID neighbours; it does NOT change SPF.
- Adj-SID lifecycle is driven by adjacency state (`iface/ism.go`): allocate from the SRLB when an adjacency reaches 2-Way, withdraw (and free the label) when it drops below 2-Way (§7.4.1).

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
- [ ] `internal/core/mplsfib/events.go` -- `Entry{Action, Op (Push/Swap/Pop), InLabel, FEC, OutLabels, NextHop, Source}`; `EntryBatch`; `EntryChange = events.Register[*EntryBatch]("mpls-fib","entry")`; producers Emit, fib-kernel Subscribes
  -> Constraint: this is the ONLY way SR programs forwarding; SR emits an `EntryBatch` per change, value-typed, with a distinct `Source` tag; no direct netlink, no sysrib best-path abuse for label-keyed swap/pop entries
- [ ] `internal/plugins/ldp/fib.go` -- `ldpFIB.ProgramPush(fec,label,nextHop)` (ingress), `ProgramPop(fec,inLabel)` (egress), `Remove(fec)` (idempotent, tracks `pushed`); `mplsSourceLDP=2`; implicit-null (3) => forward as plain IP, no push
  -> Constraint: SR mirrors this exactly: a per-FEC pushed-set for idempotent removal; an Adj-SID pop keyed by InLabel; PHP (NP=0) handled like LDP's implicit-null (no push when the SR egress neighbour wants no label)
- [ ] `internal/plugins/ldp/lib.go` -- `LIB.allocateLabelLocked()` walks `nextLabel` from 16, skips `usedLabels`, wraps at `MaxLabel` (20-bit); `EnsureLocal` allocates per-FEC; `AllocateLabel`
  -> Constraint: the SRLB Adj-SID allocator reuses this bounded-pool shape but seeded/bounded by the configured SRLB range (not the full 16..MaxLabel space); the SRGB is configured, not allocated
- [ ] `internal/plugins/ospf/spf/computer.go` -- `Computer.Run()` produces `selected []RouteEntry`; `SetOnChange(fn)` fires after each run with a `RouteDelta` (the redistribution trigger, NOT the FIB path); `Routes()`/`Snapshot()` expose the table; per-area `Result`s built in `Run` carry `Nodes` with next-hop sets
  -> Constraint: SR hooks `SetOnChange` (or a sibling SR-specific post-run callback) to recompute prefix-SID labels when the route table changes; SR reads `Routes()` for the next-hop + route type of each Prefix-SID's prefix; SR does NOT install IP routes (the Installer owns that) -- it installs LABEL entries
- [ ] `internal/plugins/ospf/spf/route.go` -- `RouteEntry{AreaID, Prefix, Metric, Type (intra/inter/ext1/ext2), Origin (RouterID), NextHops []NextHop}`; `NextHop{Addr, Interface}`
  -> Constraint: the Prefix-SID install needs the prefix's `NextHops` (push toward each ECMP next-hop) and `Origin`/`Type` (to apply §5 NP/E rules for inter-area/external prefixes); reuse `RouteEntry`, do not recompute reachability
- [ ] `internal/plugins/ospf/spf/graph.go` -- `Graph.Routers map[RouterID]*RouterVertex`; `RouterVertex{ID, Links}`; `Result.Nodes` keyed by `VertexID{Kind, Router, Network}`
  -> Constraint: the Adj-SID/LAN-Adj-SID install + the per-neighbour next-hop for an Adj-SID label come from the adjacency set; SR maps a local adjacency (interface + neighbour Router ID) to the neighbour's next-hop IP for the pop/forward entry
- [ ] `internal/plugins/ospf/iface/ism.go` + `iface/iface.go` -- interface/neighbour state machine; `StateDown`, adjacency states; `iface.go` transitions
  -> Constraint: the Adj-SID lifecycle (allocate at 2-Way, withdraw below 2-Way per §7.4.1) is driven by these adjacency-state transitions; SR subscribes to (or is polled on) adjacency change, allocating/freeing an SRLB label and re-originating the Extended Link Opaque LSA
- [ ] `internal/plugins/ospf/register.go` -- `registerOSPF()`, `runOSPFEngine`, `SetMetrics`, the snapshot dispatch (`databaseSnapshot`, `routeSnapshot`, `spfSnapshot`); consumers wired in `OnStarted`
  -> Constraint: the SR consumer's metrics + snapshot + post-run hook are wired here alongside the existing consumers; `show ip ospf segment-routing` adds a dispatch key returning the SR snapshot
- [ ] `internal/plugins/ospf/spf/install.go` -- `Installer` inserts `locrib.Path` per ECMP next-hop tagged `ospfProtocolID`, AdminDistance 110
  -> Constraint: SR does NOT touch the Installer (IP routes); SR's forwarding output is the MPLS push that rides ON TOP of the IP route the Installer already created (fib-kernel attaches the MPLS encap to the ze-owned IP route, exactly as LDP push works)

**Behavior to preserve:**
- The OSPFv2 SPF route table, `RouteEntry`/`NextHop` shapes, the `Installer` IP-route install, the `SetOnChange` redistribution trigger, and all existing OSPF + opaque-carrier behaviour. A router without SR enabled behaves exactly as today.
- The `mpls-fib` bus contract (`Entry`/`EntryBatch`), fib-kernel as the single netlink owner, the LDP/RSVP-TE source tags (SR takes a new, distinct tag).
- The ext-1 TLV iterator/builder, the ext-3 RI LSA origination/codec, and the ext-4 Extended Prefix/Link origination/codec are consumed unchanged; SR adds TLV types, not new carriers.

**Behavior to change:** (all RFC-8665-required, not discretionary)
- When SR is enabled: the RI LSA gains the SR-Algorithm/SRGB/SRLB(/SRMS) top-level TLVs; the node originates Extended Prefix Opaque LSAs with Prefix-SID sub-TLVs and Extended Link Opaque LSAs with Adj-SID/LAN-Adj-SID sub-TLVs.
- On receiving SR TLVs: the node records remote SRGBs/algorithms and installs MPLS forwarding for reachable prefix-SIDs and for its own adj-SIDs.
- Adjacency transitions below 2-Way withdraw the corresponding Adj-SID (§7.4.1) and free its SRLB label.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Origination:** SR config enabled (SRGB/SRLB ranges, node prefix-SID index) -> the SR consumer registers TLV emitters with ext-3 (RI) and ext-4 (Ext-Prefix/Ext-Link) -> on the next self-LSA origination the carriers call the SR emitters -> SR TLV bytes are written via the ext-1 TLV builder -> flooded by the carrier.
- **Reception:** an RI / Extended Prefix / Extended Link Opaque LSA arrives -> ext-1 carrier -> ext-3/ext-4 decode -> the SR parser hook receives the decoded TLV body (SR-Algorithm/SRGB/SRLB/SRMS, or Prefix-SID, or Adj-SID/LAN-Adj-SID) -> SR state is updated.
- **Forwarding trigger:** an SPF run completes (`Computer` post-run hook) OR a remote SR LSA changes OR a local adjacency changes -> SR recomputes labels and emits `mpls-fib` entries.
- **Adjacency lifecycle:** an adjacency reaches/leaves 2-Way -> SR allocates/frees an SRLB label and re-originates the Extended Link Opaque LSA.

### Transformation Path
1. **SR config resolve:** YANG SR leaves (`enable`, `srgb` range(s), `srlb` range, per-prefix `prefix-sid` index, `node` flag) resolve into the engine's SR config.
2. **Emitter registration (origination):** SR registers TLV emitters: with ext-3 for the RI LSA top-level TLVs (8/9/14/15), with ext-4 for the Extended Prefix Prefix-SID sub-TLV (Type 2) and the Extended Link Adj-SID/LAN-Adj-SID sub-TLVs (Type 2/3). Each emitter writes its TLV via the ext-1 builder.
3. **SR codec:** the SID/Label Sub-TLV (Type 1, 3-octet label / 4-octet index), the SR-Algorithm/SRGB/SRLB/SRMS bodies, the Prefix-SID body (flags + MT-ID + algorithm + SID/Index/Label), the Adj-SID/LAN-Adj-SID bodies (flags + weight + [neighbour] + SID/Index/Label) -- encode + decode, with §5 V/L validation.
4. **Reception parse:** the SR parser hook (registered with ext-3/ext-4) is handed the decoded TLV body + originator Router ID + flooding scope; it validates (§3/§5 rules), records the originator's SR-Algorithm + ordered SRGB, and stores prefix-SID / adj-SID entries.
5. **Label computation:** for a received Prefix-SID with index I and originator R, look up R's ordered SRGB ranges, find the range covering I, compute `label = range_base + (I - cumulative_prior)`; reject if I exceeds the total range size or the algorithm is one Ze does not compute; for V=1/L=1 use the absolute local label directly.
6. **Forwarding decision (§5):** read the SR route table entry for the prefix (next-hop, type). If the next-hop router advertised a Prefix-SID for the same prefix with M set, ignore NP/E; else if NP=0 -> the penultimate hop pops (forward as plain IP toward a directly-attached SR egress, like LDP implicit-null); if E=1 -> push/swap to Explicit NULL (0); otherwise push (ingress) or swap (transit) the computed label.
7. **MPLS install:** emit `mplsfibevents.Entry` with `Source = mplsSourceOSPFSR`: Push (FEC=prefix, OutLabels=[label], NextHop) for ingress; Swap (InLabel=local-node-SID-label, OutLabels=[remote label], NextHop) for transit; Pop (InLabel=local Adj-SID label) for the advertiser's adjacency forwarding. Removal mirrors LDP's idempotent per-key tracking.
8. **Adj-SID origination:** on adjacency >= 2-Way, allocate an SRLB local label, originate an Extended Link Opaque LSA carrying the Adj-SID (and LAN-Adj-SID on broadcast), and install the pop/forward entry; on adjacency < 2-Way, withdraw the LSA (carrier MaxAge flush) and free the label (§7.4.1).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| SR config <-> engine | YANG SR leaves -> resolved SR config (SRGB/SRLB ranges, prefix-SID index) | [ ] |
| SR <-> RI LSA (ext-3) | registered emitter/parser for RI top-level TLVs 8/9/14/15; value-typed TLV bodies | [ ] |
| SR <-> Extended Prefix/Link (ext-4) | registered emitter/parser for Prefix-SID (sub-TLV 2) and Adj-SID/LAN-Adj-SID (sub-TLV 2/3) | [ ] |
| SR <-> ext-1 TLV builder/iterator | SR TLV bytes written/read via the carrier's 4-octet-aligned helpers (buffer-first, zero-copy) | [ ] |
| SR <-> SPF (read-only) | `Computer.Routes()` for prefix next-hop/type; the SPF graph for adjacency neighbour IDs; a post-run hook | [ ] |
| SR <-> mpls-fib bus | `EntryChange.Emit(EntryBatch)` with `Source=mplsSourceOSPFSR`; fib-kernel programs netlink | [ ] |
| SR <-> adjacency state | allocate/free SRLB label + re-originate Ext-Link LSA on 2-Way up/down (§7.4.1) | [ ] |

### Integration Points
- `internal/plugins/ospf/sr/` (new package) -- the SR consumer: codec, SRGB/SRLB management, label computation, forwarding install, snapshot.
- ext-3 RI LSA originator/decoder -- registers SR top-level TLV emitters/parsers (no SR spelling in ext-3).
- ext-4 Extended Prefix/Link originators/decoders -- registers SR sub-TLV emitters/parsers (no SR spelling in ext-4).
- ext-1 opaque carrier -- uses the TLV iterator/builder; SR LSAs flood via the carrier's scope rules.
- `internal/plugins/ospf/spf` -- READ ONLY: route table + adjacency graph for next-hops/neighbours; a post-run hook to recompute labels.
- `internal/core/mplsfib` -- the forwarding-install bus (Push/Swap/Pop).
- `internal/plugins/ospf` (engine) -- SR config resolve, metrics, snapshot dispatch, adjacency-state subscription, the SR consumer lifecycle.
- `internal/component/mpls` -- READ ONLY for `show mpls forwarding` (SR entries appear there once fib-kernel programs them); no SR code added here.

### Architectural Verification
- [ ] No bypassed layers (SR TLVs flow through ext-1/ext-3/ext-4 carriers; forwarding flows through `mpls-fib` -> fib-kernel; SR never floods or programs netlink directly)
- [ ] No unintended coupling (ext-1/ext-3/ext-4 and OSPF core name nothing SR; SR depends on them, not vice-versa)
- [ ] No duplicated functionality (reuses the TLV builder, the RI/Extended carriers, the SPF route table, the LDP label-pool pattern, the mpls-fib bus; adds only the SR codec, SRGB/SRLB management, label arithmetic, and the SR install glue)
- [ ] Zero-copy preserved (TLV bodies are views; SID/Label field written into caller buffers; mpls-fib entries value-typed with an owned label slice)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | ext-3 exposes a registration seam for RI LSA top-level TLV emitters/parsers (so SR can inject TLVs 8/9/14/15 without editing ext-3) | dependency `spec-ospf-ext-3-router-information.md`; the ext-1 consumer-registry precedent (`RegisterOpaqueConsumer`) | SR must edit ext-3 directly, violating self-containment; coordinate a seam change | `TestSRRegistersRITLVs` exercises the ext-3 seam | unvalidated |
| A-2 | ext-4 exposes a registration seam for Extended Prefix and Extended Link sub-TLV emitters/parsers | dependency `spec-ospf-ext-4-extended-link-prefix.md`; RFC 7684 §2.1/§3.1 sub-TLV registries | SR must edit ext-4 directly; coordinate a seam change | `TestSRRegistersExtPrefixSubTLV`, `TestSRRegistersExtLinkSubTLV` | unvalidated |
| A-3 | The `mpls-fib` bus (`mplsfibevents.Entry`, Push/Swap/Pop) is sufficient to install SR prefix-SID and adj-SID forwarding with no fib-kernel change | `internal/core/mplsfib/events.go`; `internal/plugins/ldp/fib.go` ProgramPush/ProgramPop | fib-kernel needs an SR-aware path; larger change | `TestSRInstallPrefixSIDPush`, interop `ospf-sr-frr` shows the kernel entry | unvalidated |
| A-4 | The SPF route table (`Computer.Routes()`/`RouteEntry`) gives the next-hop, route type, and origin needed to compute the prefix-SID push/swap and apply §5 NP/E rules | `spf/route.go`, `spf/computer.go` | SR must recompute reachability; duplicates SPF | `TestSRPrefixSIDUsesSPFNextHop` | unvalidated |
| A-5 | The SPF graph + adjacency set expose neighbour Router IDs + next-hop IPs so an Adj-SID maps to a concrete (interface, next-hop) | `spf/graph.go` `RouterVertex.Links`, `iface/iface.go` neighbour state | the Adj-SID forwarding next-hop is unavailable; need new neighbour plumbing | `TestSRAdjSIDForwardsToNeighbor` | unvalidated |
| A-6 | A bounded 20-bit label allocator (the LDP `nextLabel`/`MaxLabel` shape) seeded by the configured SRLB range is sufficient for Adj-SID allocation | `internal/plugins/ldp/lib.go` | a more elaborate allocator is needed (persistence, ranges) | `TestSRLBAllocatorBounds`, `TestSRLBAllocatorExhaustion` | unvalidated |
| A-7 | Adjacency-state transitions (2-Way up/down) are observable to the SR consumer for the §7.4.1 Adj-SID withdraw | `iface/ism.go`, engine adjacency-change events | SR cannot withdraw Adj-SIDs on adjacency loss; stale forwarding | `TestSRAdjSIDWithdrawnBelow2Way` | unvalidated |
| A-8 | The SRGB is a single configured global range per node (multiple ranges supported on receive but a single configured range on originate is acceptable for v1) | RFC 8665 §3.2 (multiple MAY); operational norm (one SRGB block) | originating multiple ranges is required for interop; extend config | `ospf-sr-frr` interop accepts Ze's single-range SRGB | unvalidated |
| A-9 | Ze computes only Algorithm 0 (SPF); a received Prefix-SID for an algorithm not in the originator's SR-Algorithm TLV, or for Algorithm 1, is recorded but not installed | §3.1, §5 ("MUST ignore the Prefix-SID Sub-TLV" if algorithm not advertised); out-of-scope strict-SPF | installing an unsupported-algorithm SID misroutes | `TestSRPrefixSIDUnknownAlgorithmIgnored` | unvalidated |
| A-10 | SR LSAs use area flooding scope (LS Type 10) for SR-Algorithm/SRGB/SRLB and Extended Link; the carriers honour scope (ext-1) | RFC 8665 §3.1 (area-scoped REQUIRED); RFC 7684 §3 (Ext-Link area only) | mis-scoped SR TLVs leak or fail to reach; interop break | `TestSRTLVsAreaScoped` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Multi-range SRGB index->label mapping computed wrong (off-by-one or wrong range order) -> traffic mislabelled and blackholed | a computed label disagrees with FRR for an index spanning two ranges | dedicated `TestSRLabelFromIndexMultiRange` pinning the cumulative-offset arithmetic; concatenate ranges in advertised order (§3.2); interop label cross-check against FRR |
| R-2 | V/L flag mishandling -> a 3-octet local label parsed as a 4-octet index (or vice-versa), shifting every subsequent byte | the SID field length (7 vs 8, 11 vs 12) disagrees with the parsed form | strict §5 validation: only V=0/L=0 (4-octet) and V=1/L=1 (3-octet) accepted, Length must match; `TestSRSIDFieldVL` for both forms + every invalid combination |
| R-3 | NP/E/M flag misapplication -> wrong PHP/Explicit-NULL behaviour; the SR egress sees an unexpected label and drops | FRR egress logs a label mismatch / the ping over the SR LSP fails at the last hop | implement the §5 truth table explicitly (M overrides NP/E; NP=0 -> PHP; NP=1,E=0 -> keep; NP=1,E=1 -> Explicit NULL); `TestSRPHPBehavior` per combination |
| R-4 | Adj-SID not withdrawn on adjacency loss (§7.4.1) -> a stale pop entry forwards to a dead neighbour | a removed adjacency still has an `mpls-fib` pop entry / `show ip ospf segment-routing` lists it | drive Adj-SID lifecycle off adjacency state; free the SRLB label + MaxAge-flush the Ext-Link LSA + remove the pop entry; `TestSRAdjSIDWithdrawnBelow2Way` |
| R-5 | SRGB/SRLB ranges overlap (own or between own SRGB and SRLB) -> a label is double-claimed | config validation passes but two SIDs map to one label | YANG + resolve-time validation: SRGB and SRLB MUST NOT overlap, Range Size > 0; `TestSRGBSRLBNoOverlap` |
| R-6 | A malformed SR TLV/sub-TLV crashes the parser (untrusted flooded input) | fuzz crash on an SR LSA body | the SR parser is bound-checked over the ext-1 iterator's views; a malformed length makes the LSA malformed (ext-4 drops it); extend the OSPF packet fuzz target with SR bodies; `TestSRParserMalformed` |
| R-7 | Duplicate Prefix-SID for the same prefix/topology/algorithm not detected -> all-MUST-be-ignored rule (§5) violated, a wrong label installed | two installed labels for one prefix | dedupe on (prefix, MT-ID, algorithm); if more than one, install none and count a metric; `TestSRDuplicatePrefixSIDIgnored` |
| R-8 | SR install races the IP-route install (SR push emitted before the underlying IP route exists) -> fib-kernel rejects the encap | a push entry appears with no parent IP route; kernel error | order SR recompute AFTER the SPF Installer's `Apply` (the post-run hook fires after install); fib-kernel reasserts on the next route event; `TestSRInstallOrderingAfterRoute` |
| R-9 | The SRGB advertised order changes across re-origination/restart -> peers recompute different labels (§3.2 MUST keep order stable) | a peer's label for an index flaps after Ze re-originates | originate ranges in a fixed, config-declared order; a stable iteration; `TestSRGBOrderStableAcrossReorigination` |
| R-10 | SR enabled but no SR-Algorithm TLV from a peer -> that peer is (correctly) treated as non-SR, but Ze still tries to install its prefix-SIDs | a label installed toward a non-SR next-hop that cannot switch it | gate install on the next-hop router advertising SR (SR-Algorithm present) and the prefix's originator advertising the prefix-SID; `TestSRNoInstallForNonSRNextHop` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| SR config enabled with an SRGB and a node prefix-SID index | -> | SR registers RI + Extended-Prefix TLV emitters; on origination the RI LSA carries SR-Algorithm/SRGB and the Extended Prefix Opaque LSA carries the Prefix-SID | `TestSROriginatesRIAndPrefixSID` (unit) + `test/ospf/ospf-sr-originate.ci` |
| An RI + Extended Prefix Opaque LSA carrying SR TLVs arrives from a peer for a reachable prefix | -> | SR parser records the SRGB, computes the label, emits an `mpls-fib` push toward the SPF next-hop | `TestSRReceivesAndInstallsPrefixSID` (unit) + `test/ospf/ospf-sr-receive.ci` |
| An adjacency reaches 2-Way | -> | SR allocates an SRLB label, originates the Extended Link Opaque LSA with an Adj-SID, installs the pop entry | `test/ospf/ospf-sr-adj.ci` |
| An adjacency drops below 2-Way | -> | SR withdraws the Adj-SID LSA, frees the label, removes the pop entry (§7.4.1) | `TestSRAdjSIDWithdrawnBelow2Way` (unit) |
| `show ip ospf segment-routing` is run | -> | the SR snapshot dispatch returns SRGB/SRLB, prefix-SIDs, adj-SIDs, computed labels | `test/ospf/ospf-sr-show.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | SR enabled with an SRGB range | the RI LSA carries an SR-Algorithm TLV including Algorithm 0 and one SID/Label Range TLV (Range Size > 0) with exactly one SID/Label Sub-TLV; area-scoped (§3.1, §3.2) |
| AC-2 | SR enabled with an SRLB range | the RI LSA carries an SRLB TLV (Range Size > 0, one SID/Label Sub-TLV); SRLB and SRGB MUST NOT overlap (§3.3) |
| AC-3 | SR enabled with a node prefix-SID index for a loopback | an Extended Prefix Opaque LSA carries an Extended Prefix TLV with a Prefix-SID Sub-TLV (index, V=0/L=0, NP per directly-attached rule); the N-Flag is set on the host loopback prefix (§5, RFC 7684 §2.1) |
| AC-4 | A received SID/Label Range TLV with more than one SID/Label Sub-TLV | the range TLV is ignored (§3.2) |
| AC-5 | A received RI LSA with overlapping SRGB ranges | handled per RFC 8660 (the receiver does not double-map); SR records a non-overlapping ordered SRGB (§3.2) |
| AC-6 | A received Prefix-SID with index I, originator R whose SRGB is ranges in advertised order | the computed label is `range_base + (I - cumulative_prior)` for the range covering I; an index beyond the total range size is rejected and no label installed (§3.2) |
| AC-7 | A received Prefix-SID with V=1/L=1 | the 3-octet SID/Index/Label field is read as an absolute 20-bit local label; V=0/L=0 reads a 4-octet index; any other V/L combination causes the SID Advertisement to be ignored (§5) |
| AC-8 | A received Prefix-SID for a reachable prefix whose next-hop router advertises SR | an `mpls-fib` push (ingress) or swap (transit) entry is emitted toward the SPF next-hop with the computed label; the entry carries `Source=mplsSourceOSPFSR` |
| AC-9 | A received Prefix-SID, next-hop router flags NP=0 | the penultimate hop pops: SR forwards as plain IP toward a directly-attached SR egress (no push), mirroring implicit-null; NP=1,E=0 keeps the label; NP=1,E=1 uses Explicit NULL (0); M set ignores NP/E (§5) |
| AC-10 | A received Prefix-SID whose algorithm is not in the originator's SR-Algorithm TLV, or Algorithm 1 (which Ze does not compute) | the Prefix-SID is recorded but NOT installed (§5, §3.1) |
| AC-11 | A router advertises multiple Prefix-SIDs for the same prefix/topology/algorithm | all of them are ignored and a metric is incremented (§5) |
| AC-12 | An adjacency reaches state 2-Way or higher on a P2P link | an Adj-SID Sub-TLV is advertised in an Extended Link Opaque LSA, allocated from the SRLB; a pop/forward entry is installed toward that neighbour (§6.1, §7.4.1) |
| AC-13 | A P2P adjacency transitions below 2-Way | the Adj-SID Advertisement is withdrawn from the area, the SRLB label is freed, and the pop entry is removed (§7.4.1) |
| AC-14 | A broadcast/NBMA adjacency to a non-DR neighbour | a LAN-Adj-SID Sub-TLV carrying the Neighbor ID is advertised (§6.2, §7.4.2) |
| AC-15 | An ABR propagates an Extended Prefix Range TLV between areas | the IA-Flag is set (§4, §7.1) |
| AC-16 | A malformed SR TLV/sub-TLV (bad length, truncated SID field) | the LSA is treated as malformed and not installed; the parser does not panic; reception is counted (§9, §10) |
| AC-17 | `show ip ospf segment-routing` | renders the configured SRGB/SRLB, this node's prefix-SIDs and adj-SIDs, and per-remote computed labels with their forwarding action (push/swap/pop) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables SR on a Ze node (SRGB, node prefix-SID) and a peer's loopback becomes reachable | SR config -> RI + Ext-Prefix emitters -> origination; peer's Ext-Prefix LSA received -> label computed -> `mpls-fib` push -> fib-kernel programs the push; `show mpls forwarding` lists it | `test/ospf/ospf-sr-originate.ci` + `test/ospf/ospf-sr-receive.ci` + `ospf-sr-frr` interop |
| 2 | Pings a remote SR loopback over the SR LSP (label-switched) | SR push at ingress -> transit swap -> PHP/Explicit-NULL at the egress per the remote's NP/E flags -> packet delivered | `ospf-sr-frr` interop (label-switched reachability + NP/E behaviour) |
| 3 | Brings an SR adjacency up then down | adjacency 2-Way -> SRLB label allocated -> Ext-Link LSA with Adj-SID -> pop entry; adjacency down -> Adj-SID withdrawn, label freed, pop removed | `test/ospf/ospf-sr-adj.ci` |
| 4 | Inspects SR state via CLI | `show ip ospf segment-routing` -> SR snapshot (SRGB/SRLB, prefix-SIDs, adj-SIDs, computed labels + actions) | `test/ospf/ospf-sr-show.ci` |
| 5 | Runs Ze SR against FRR ospfd with SR enabled | DD/flood exchange; both originate SR TLVs; Ze installs FRR's prefix-SIDs and FRR installs Ze's; labels agree for multi-range SRGB | `ospf-sr-frr` interop |
| 6 | Removes the SR consumer (build without it) | the SR TLV emitters/parsers, config, CLI, and metrics vanish; OSPF + the opaque carriers behave as before; no SR forwarding | `TestOSPFBuildsWithoutSR` + existing OSPF suite still green |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSIDLabelSubTLVRoundTrip` | `internal/plugins/ospf/sr/codec_test.go` | AC-7: SID/Label Sub-TLV 3-octet label vs 4-octet index encode/decode (§2.1) | |
| `TestSRAlgorithmTLVRoundTrip` | `internal/plugins/ospf/sr/codec_test.go` | AC-1: SR-Algorithm TLV includes Algorithm 0; decode rejects absence (§3.1) | |
| `TestSRGBRangeTLVRoundTrip` | `internal/plugins/ospf/sr/codec_test.go` | AC-1/AC-4: SID/Label Range TLV; exactly one SID/Label Sub-TLV; Range Size > 0 (§3.2) | |
| `TestSRLBTLVRoundTrip` | `internal/plugins/ospf/sr/codec_test.go` | AC-2: SRLB TLV encode/decode (§3.3) | |
| `TestSRMSPrefTLVRoundTrip` | `internal/plugins/ospf/sr/codec_test.go` | SRMS-Pref TLV; first-occurrence/narrowest-scope tie-break (§3.4) | |
| `TestPrefixSIDSubTLVRoundTrip` / `TestSRSIDFieldVL` | `internal/plugins/ospf/sr/codec_test.go` | AC-7, R-2: Prefix-SID flags + V/L sizing; every invalid V/L combination ignored (§5) | |
| `TestAdjSIDSubTLVRoundTrip` / `TestLANAdjSIDSubTLVRoundTrip` | `internal/plugins/ospf/sr/codec_test.go` | AC-12/AC-14: Adj-SID + LAN-Adj-SID flags/weight/neighbour (§6.1, §6.2) | |
| `TestExtPrefixRangeTLVRoundTrip` | `internal/plugins/ospf/sr/codec_test.go` | AC-15: Extended Prefix Range TLV; IA-Flag (§4) | |
| `TestSRParserMalformed` | `internal/plugins/ospf/sr/codec_test.go` | AC-16, R-6: malformed TLV/sub-TLV never panics; LSA marked malformed (§9) | |
| `TestSRLabelFromIndexSingleRange` / `TestSRLabelFromIndexMultiRange` | `internal/plugins/ospf/sr/srgb_test.go` | AC-6, R-1: index->label across one and multiple ordered ranges; cumulative offset | |
| `TestSRLabelIndexOutOfRange` | `internal/plugins/ospf/sr/srgb_test.go` | AC-6: index beyond total range size rejected, no install | |
| `TestSRGBOrderStableAcrossReorigination` | `internal/plugins/ospf/sr/srgb_test.go` | R-9: advertised range order stable (§3.2) | |
| `TestSRGBSRLBNoOverlap` | `internal/plugins/ospf/sr/config_test.go` | AC-2, R-5: SRGB/SRLB non-overlap, Range Size > 0 validation | |
| `TestSRLBAllocatorBounds` / `TestSRLBAllocatorExhaustion` | `internal/plugins/ospf/sr/srlb_test.go` | A-6: bounded SRLB allocator within range; exhaustion handled | |
| `TestSRPrefixSIDUsesSPFNextHop` | `internal/plugins/ospf/sr/install_test.go` | AC-8, A-4: push/swap toward the SPF next-hop from `RouteEntry` | |
| `TestSRInstallPrefixSIDPush` / `TestSRInstallPrefixSIDSwap` | `internal/plugins/ospf/sr/install_test.go` | AC-8: `mpls-fib` push (ingress) and swap (transit) entries with `Source=mplsSourceOSPFSR` | |
| `TestSRPHPBehavior` | `internal/plugins/ospf/sr/install_test.go` | AC-9, R-3: NP/E/M truth table (PHP, keep, Explicit NULL, M override) (§5) | |
| `TestSRPrefixSIDUnknownAlgorithmIgnored` | `internal/plugins/ospf/sr/install_test.go` | AC-10, A-9: algorithm-not-advertised / Algorithm 1 recorded, not installed (§5) | |
| `TestSRDuplicatePrefixSIDIgnored` | `internal/plugins/ospf/sr/install_test.go` | AC-11, R-7: multiple Prefix-SIDs same prefix/topology/algorithm all ignored (§5) | |
| `TestSRNoInstallForNonSRNextHop` | `internal/plugins/ospf/sr/install_test.go` | R-10: no label installed toward a non-SR next-hop | |
| `TestSRInstallOrderingAfterRoute` | `internal/plugins/ospf/sr/install_test.go` | R-8: SR push emitted after the IP route exists | |
| `TestSRAdjSIDForwardsToNeighbor` | `internal/plugins/ospf/sr/adjsid_test.go` | AC-12, A-5: Adj-SID pop forwards to the specific neighbour next-hop | |
| `TestSRAdjSIDWithdrawnBelow2Way` | `internal/plugins/ospf/sr/adjsid_test.go` | AC-13, R-4: Adj-SID withdrawn + label freed + pop removed below 2-Way (§7.4.1) | |
| `TestSRRegistersRITLVs` | `internal/plugins/ospf/sr/register_test.go` | A-1: SR top-level TLV emitter/parser registered with ext-3 | |
| `TestSRRegistersExtPrefixSubTLV` / `TestSRRegistersExtLinkSubTLV` | `internal/plugins/ospf/sr/register_test.go` | A-2: SR sub-TLV emitter/parser registered with ext-4 | |
| `TestSROriginatesRIAndPrefixSID` | `internal/plugins/ospf/sr/origination_test.go` | AC-1/AC-3: RI + Ext-Prefix originate SR TLVs when enabled | |
| `TestSRTLVsAreaScoped` | `internal/plugins/ospf/sr/origination_test.go` | AC, A-10: SR-Algorithm/SRGB/SRLB + Ext-Link area-scoped (§3.1) | |
| `TestSRSnapshot` | `internal/plugins/ospf/sr/snapshot_test.go` | AC-17: `show ip ospf segment-routing` snapshot rows | |
| `TestOSPFBuildsWithoutSR` | `internal/plugins/ospf/sr/register_test.go` | self-containment: removing SR leaves OSPF + carriers intact | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| MPLS label (SID/Label, 3-octet form) | 16-1048575 (20-bit) | 1048575 | 0-15 reserved (0 = Explicit NULL only) | >1048575 rejected |
| SID index (Prefix-SID, V=0/L=0) | 0-4294967295 (32-bit) | within total SRGB range size | N/A | index >= total range size rejected (§3.2) |
| Range Size (SRGB/SRLB, 3-octet) | 1-16777215 | 16777215 | 0 invalid (MUST be > 0, §3.2/§3.3) | N/A (3 octets) |
| Range Size (Extended Prefix Range, 2-octet) | 1-65535 | bounded by Prefix Length capacity excl. 224.0.0.0/3 | 0 | exceeding capacity rejected (§4) |
| SR-Algorithm value | 0-255 | 255 | N/A | Algorithm 0 MUST be present when TLV advertised (§3.1) |
| Prefix-SID Length (V-Flag) | 7 or 8 | 8 | N/A | other lengths malformed (§5) |
| Adj-SID Length (V-Flag) | 7 or 8 | 8 | N/A | other lengths malformed (§6.1) |
| LAN-Adj-SID Length (V-Flag) | 11 or 12 | 12 | N/A | other lengths malformed (§6.2) |
| SRMS Preference | 0-255 | 255 | N/A | N/A (1 octet) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-sr-originate` | `test/ospf/ospf-sr-originate.ci` | SR enabled; RI LSA shows SR-Algorithm/SRGB, Ext-Prefix shows the node prefix-SID | |
| `ospf-sr-receive` | `test/ospf/ospf-sr-receive.ci` | a received prefix-SID computes a label and installs an `mpls-fib` push; `show mpls forwarding` lists it | |
| `ospf-sr-adj` | `test/ospf/ospf-sr-adj.ci` | an adjacency advertises an Adj-SID; dropping it withdraws the Adj-SID and removes the pop entry | |
| `ospf-sr-show` | `test/ospf/ospf-sr-show.ci` | `show ip ospf segment-routing` renders SRGB/SRLB, prefix-SIDs, adj-SIDs, computed labels | |
| `ospf-sr-php` | `test/ospf/ospf-sr-php.ci` | NP/E flags drive PHP / keep / Explicit-NULL on the installed label | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-sr-frr` | `test/interop/scenarios/ospf-sr-frr/` | FRR `ospfd` with `segment-routing on` (SRGB/SRLB, prefix-SIDs, adj-SIDs) | Ze and FRR exchange SR TLVs, agree on index->label for a multi-range SRGB, install matching MPLS forwarding, and a label-switched ping over the SR LSP succeeds with correct NP/E (PHP / Explicit-NULL) behaviour | |

> Interop is required: this changes wire behaviour (new RI + Extended Prefix/Link
> TLVs) and programs the MPLS data plane. The raw-IP / multicast / AF_MPLS paths
> are Linux-only and run as QEMU integration tests (`ai/rules/qemu-testing.md`),
> consistent with the rest of the OSPF + MPLS interop set.

### Future (if deferring any tests)
- None. Every AC maps to a unit, functional, or interop test above. TI-LFA backup-path tests and strict-SPF (Algorithm 1) path-install tests are out of scope (ext-6 / deferred), not deferred-but-in-scope.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*) -->
- `internal/plugins/ospf/register.go` -- wire the SR consumer: metrics, the `show ip ospf segment-routing` snapshot dispatch key, the SR post-SPF-run hook, and adjacency-change subscription for Adj-SID lifecycle
- `internal/plugins/ospf/config.go` -- resolve the SR YANG leaves (enable, SRGB/SRLB ranges, node prefix-SID index) into the engine SR config
- `internal/plugins/ospf/spf/computer.go` -- expose a read-only post-run SR hook (a sibling to `SetOnChange`) and ensure it fires AFTER the Installer `Apply` (R-8) so SR pushes ride existing IP routes
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- the `segment-routing` config container (enable, srgb, srlb, prefix-sid index, node flag)
- `internal/plugins/ospf/yang/ze-ospf-cmd.yang` -- the `show ip ospf segment-routing` command
- `internal/plugins/ospf/doctor.go` -- a doctor check for SR config sanity (SRGB/SRLB present + non-overlapping when SR enabled, MPLS forwarding available)
- ext-3 RI LSA originator/decoder (per `spec-ospf-ext-3-router-information.md`) -- a registration seam for SR top-level TLV emitters/parsers (no SR spelling added to ext-3 code)
- ext-4 Extended Prefix/Link originators/decoders (per `spec-ospf-ext-4-extended-link-prefix.md`) -- a registration seam for SR sub-TLV emitters/parsers (no SR spelling added to ext-4 code)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- `segment-routing` container; read `ai/rules/config-surface.md` + `ai/rules/config-naming.md` |
| YANG validation constraints | [ ] yes | SRGB/SRLB ranges: `range` on label values (16..1048575), `range` on size (>0); `enable` boolean; prefix-sid index `range`; custom validator for SRGB/SRLB non-overlap |
| YANG custom validators | [ ] yes | SRGB/SRLB overlap + capacity validation (`ze:validate` + `ValidateFn`); `CompleteFn` for label-range hints; register in the OSPF validators register |
| CLI commands/flags | [ ] yes | `show ip ospf segment-routing` in `ze-ospf-cmd.yang` + the register dispatch |
| CLI grammar (action before identifier) | [ ] yes | `ai/rules/cli-grammar.md` -- `show ip ospf segment-routing` |
| Editor autocomplete | [ ] yes | automatic for the YANG enum/boolean leaves + the new show subcommand |
| Functional test for new RPC/API | [ ] yes | `test/ospf/ospf-sr-*.ci` |
| Pipe completeness | [ ] yes | `show ip ospf segment-routing` routes through `ApplyPipes` like the other show outputs |
| Env var registration | [ ] no | SR is operational config, not an `environment/` leaf |
| Doctor check for runtime dependencies | [ ] yes | SR programs the MPLS data plane: a doctor check that AF_MPLS forwarding is available and SRGB/SRLB are sane; `internal/core/diagnostic/codes.go` + unit + functional test (`ai/rules/doctor-checks.md`) |
| Prometheus counters/metrics | [ ] yes | see the metrics rows below |

#### Metrics (new series owned by this spec)
| Metric | Type | Labels |
|--------|------|--------|
| `ze_ospf_sr_enabled` | gauge | (none) |
| `ze_ospf_sr_prefix_sids` | gauge | `direction` (originated/received) |
| `ze_ospf_sr_adj_sids` | gauge | (none) |
| `ze_ospf_sr_labels_installed` | gauge | `op` (push/swap/pop) |
| `ze_ospf_sr_label_compute_errors_total` | counter | `reason` (index-out-of-range / unknown-algorithm / duplicate / bad-vl) |
| `ze_ospf_sr_srlb_labels_in_use` | gauge | (none) |
| `ze_ospf_sr_malformed_tlvs_total` | counter | `tlv` |

> These extend the canonical OSPF metric set with the `ze_ospf_sr_*` prefix,
> registered by this spec's owner code (not by the OSPF metrics core). The OSPF
> telemetry doc gains these rows when this spec lands.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` -- OSPFv2 Segment Routing |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` -- the `segment-routing` container |
| 3 | CLI command added/changed? | [ ] yes | `docs/guide/command-reference.md` -- `show ip ospf segment-routing` |
| 4 | API/RPC added/changed? | [ ] no | the show RPC lives in the central `ze-show` namespace; document under the command reference |
| 5 | Plugin added/changed? | [ ] yes | `docs/guide/plugins.md` -- OSPF gains an SR consumer |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/ospf.md` -- a Segment Routing section (SRGB/SRLB, prefix-SID, adj-SID) |
| 7 | Wire format changed? | [ ] yes | `docs/architecture/wire/ospf.md` (or equivalent) -- the SR TLVs/sub-TLVs |
| 8 | Plugin SDK/protocol changed? | [ ] yes | document the ext-3/ext-4 SR registration seam for future SR-adjacent consumers |
| 9 | RFC behavior implemented? | [ ] yes | `rfc/short/rfc8665.md` -- tick the implemented Compliance Checklist items |
| 10 | Test infrastructure changed? | [ ] yes (interop scenario added) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` -- OSPF SR parity with FRR |
| 12 | Internal architecture changed? | [ ] yes | the OSPF subsystem doc + the MPLS forwarding doc -- SR as a third mpls-fib producer |
| 13 | Route metadata keys added/changed? | [ ] no | SR installs label entries via mpls-fib, not route metadata; confirm no meta key added |
| 14 | Prometheus counters added/changed? | [ ] yes | the OSPF telemetry doc -- the `ze_ospf_sr_*` series |
| 15 | Registered plugin/event/command/capability inventory changed? | [ ] yes | `docs/plugin-overview.md` -- SR consumer + the `show ip ospf segment-routing` command + the mpls-fib SR source tag |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors into the changed OSPF/mpls files |
| 17 | Existing docs show examples for this area? | [ ] check | verify any OSPF/MPLS config/CLI examples against the new SR leaves |

## Files to Create
- `internal/plugins/ospf/sr/codec.go` -- the SR TLV/sub-TLV codec: SID/Label Sub-TLV, SR-Algorithm, SID/Label Range (SRGB), SRLB, SRMS-Pref, Extended Prefix Range, Prefix-SID, Adj-SID, LAN-Adj-SID (encode + decode + §5 V/L validation) using the ext-1 TLV builder/iterator
- `internal/plugins/ospf/sr/srgb.go` -- SRGB representation (ordered ranges) + the index->label computation across ranges
- `internal/plugins/ospf/sr/srlb.go` -- the bounded SRLB local-label allocator (LDP `nextLabel`/`MaxLabel` pattern, seeded by the configured SRLB range)
- `internal/plugins/ospf/sr/config.go` -- the resolved SR config + SRGB/SRLB non-overlap/capacity validation
- `internal/plugins/ospf/sr/origination.go` -- the SR emitters: RI top-level TLVs (via ext-3 seam) + Prefix-SID / Adj-SID / LAN-Adj-SID sub-TLVs (via ext-4 seam); area scoping
- `internal/plugins/ospf/sr/reception.go` -- the SR parsers: record remote SR-Algorithm + ordered SRGB; store prefix-SID / adj-SID entries; §3/§5 validation
- `internal/plugins/ospf/sr/install.go` -- the forwarding install: label computation per route, the §5 NP/E/M truth table, `mpls-fib` push/swap/pop emission with `mplsSourceOSPFSR`, idempotent per-key removal (LDP `pushed`-set pattern)
- `internal/plugins/ospf/sr/adjsid.go` -- the Adj-SID lifecycle driven by adjacency state (allocate at 2-Way, withdraw below 2-Way, §7.4.1)
- `internal/plugins/ospf/sr/register.go` -- registers the SR emitters/parsers with ext-3/ext-4, the SR metrics, the config resolve, and the snapshot dispatch; the `mplsSourceOSPFSR` source tag
- `internal/plugins/ospf/sr/snapshot.go` -- the `show ip ospf segment-routing` snapshot rows
- `internal/plugins/ospf/sr/codec_test.go`, `srgb_test.go`, `srlb_test.go`, `config_test.go`, `install_test.go`, `adjsid_test.go`, `register_test.go`, `origination_test.go`, `snapshot_test.go`
- `test/ospf/ospf-sr-originate.ci`, `ospf-sr-receive.ci`, `ospf-sr-adj.ci`, `ospf-sr-show.ci`, `ospf-sr-php.ci`
- `test/interop/scenarios/ospf-sr-frr/` -- `ze.conf`, `frr.conf`, `check.py`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan -- confirm ext-1/ext-3/ext-4 carriers + the mpls-fib bus exist and expose the needed seams |
| 3. Wiring phase | Wiring Test table -- register SR emitters/parsers + failing wiring tests |
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

1. **Phase: Wiring (MANDATORY FIRST)** -- register SR emitters/parsers with ext-3/ext-4 + failing wiring tests
   - Tests: `TestSRRegistersRITLVs`, `TestSRRegistersExtPrefixSubTLV`, `TestSRRegistersExtLinkSubTLV`, `TestOSPFBuildsWithoutSR`, `test/ospf/ospf-sr-show.ci` (empty SR snapshot)
   - Files: `sr/register.go` (registration + `mplsSourceOSPFSR` + metrics + snapshot dispatch in `ospf/register.go`), `sr/snapshot.go` (stub), a minimal `sr/config.go`
   - Verify: SR registers with the carriers and the engine discovers it; origination/reception/install are stubs so the deeper tests still fail
2. **Phase: SR codec + SID/Label arithmetic** -- the wire primitives
   - Tests: `TestSIDLabelSubTLVRoundTrip`, `TestSRAlgorithmTLVRoundTrip`, `TestSRGBRangeTLVRoundTrip`, `TestSRLBTLVRoundTrip`, `TestSRMSPrefTLVRoundTrip`, `TestPrefixSIDSubTLVRoundTrip`, `TestSRSIDFieldVL`, `TestAdjSIDSubTLVRoundTrip`, `TestLANAdjSIDSubTLVRoundTrip`, `TestExtPrefixRangeTLVRoundTrip`, `TestSRParserMalformed`
   - Files: `sr/codec.go`
   - Verify: every SR TLV/sub-TLV round-trips via the ext-1 builder/iterator; V/L validation holds; malformed input never panics
3. **Phase: SRGB/SRLB management** -- index->label + the local allocator
   - Tests: `TestSRLabelFromIndexSingleRange`, `TestSRLabelFromIndexMultiRange`, `TestSRLabelIndexOutOfRange`, `TestSRGBOrderStableAcrossReorigination`, `TestSRLBAllocatorBounds`, `TestSRLBAllocatorExhaustion`, `TestSRGBSRLBNoOverlap`
   - Files: `sr/srgb.go`, `sr/srlb.go`, `sr/config.go`
   - Verify: the multi-range arithmetic is exact; range order stable; SRLB allocation bounded; config non-overlap enforced
4. **Phase: Origination** -- advertise SR TLVs when enabled
   - Tests: `TestSROriginatesRIAndPrefixSID`, `TestSRTLVsAreaScoped`
   - Files: `sr/origination.go`, `config.go` resolve, `yang/ze-ospf-conf.yang`
   - Verify: RI carries SR-Algorithm/SRGB/SRLB; Ext-Prefix carries the node prefix-SID; area-scoped
5. **Phase: Reception + label computation** -- record remote SR, compute labels
   - Tests: `TestSRPrefixSIDUsesSPFNextHop`, `TestSRPrefixSIDUnknownAlgorithmIgnored`, `TestSRDuplicatePrefixSIDIgnored`, `TestSRNoInstallForNonSRNextHop`
   - Files: `sr/reception.go`, `sr/install.go` (compute side)
   - Verify: remote SRGB/algorithm recorded; labels computed; §5 ignore rules hold
6. **Phase: MPLS forwarding install** -- push/swap/pop via mpls-fib
   - Tests: `TestSRInstallPrefixSIDPush`, `TestSRInstallPrefixSIDSwap`, `TestSRPHPBehavior`, `TestSRInstallOrderingAfterRoute`, `ospf-sr-receive.ci`, `ospf-sr-php.ci`
   - Files: `sr/install.go`, `spf/computer.go` (post-run hook ordering)
   - Verify: push/swap/pop emitted with `mplsSourceOSPFSR`; NP/E/M truth table correct; install after the IP route
7. **Phase: Adj-SID lifecycle** -- allocate/withdraw on adjacency state
   - Tests: `TestSRAdjSIDForwardsToNeighbor`, `TestSRAdjSIDWithdrawnBelow2Way`, `ospf-sr-adj.ci`
   - Files: `sr/adjsid.go`, `ospf/register.go` (adjacency-change subscription)
   - Verify: Adj-SID advertised at 2-Way, withdrawn + freed below 2-Way (§7.4.1); LAN-Adj-SID on broadcast
8. **Phase: CLI + metrics + doctor** -- user surface
   - Tests: `TestSRSnapshot`, `ospf-sr-originate.ci`, `ospf-sr-show.ci`
   - Files: `sr/snapshot.go`, `yang/ze-ospf-cmd.yang`, `doctor.go`, metric registration
   - Verify: `show ip ospf segment-routing`, the SR metrics, the SR doctor check
9. **Functional tests** -> the five `.ci` cover the user-visible behaviour
10. **RFC refs** -> add `// RFC 8665 Section X` comments on the V/L validation, index->label, NP/E/M, and §7.4.1 enforcement
11. **Interop** -> `ospf-sr-frr` QEMU scenario (label agreement + label-switched ping)
12. **Full verification** -> `make ze-verify`
13. **Complete spec** -> audit tables + learned summary; two commits (A: code+spec+learned, B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has file:line implementation |
| Feature completeness | each user story has a working path; SR parity with FRR (prefix-SID + adj-SID origination/reception, SRGB/SRLB, MPLS install); TI-LFA / SR-TE excluded by design |
| Correctness | multi-range index->label arithmetic; V/L sizing; NP/E/M truth table; area scoping; Adj-SID withdraw below 2-Way; SRGB/SRLB non-overlap; duplicate/unknown-algorithm ignore rules |
| Naming | `ze_ospf_sr_*` metrics; YANG `segment-routing`/`srgb`/`srlb` kebab-case; `mplsSourceOSPFSR` |
| Data flow | SR TLVs via ext-1/ext-3/ext-4 only; forwarding via mpls-fib only; SPF read-only; no SR spelling in carriers/core |
| CLI grammar | `show ip ospf segment-routing` action-before-identifier |
| Doctor checks | the SR/MPLS doctor check registered per `ai/rules/doctor-checks.md` |
| YANG validation | SRGB/SRLB ranges have `range` constraints + the non-overlap custom validator; no bare `type string` |
| Prometheus counters | the seven `ze_ospf_sr_*` series defined, registered, listed |
| Rule: plugin-self-containment | carriers/core name nothing SR; removing the `sr/` package removes all SR behaviour |
| Rule: buffer-first | SR TLV emit uses the ext-1 builder; mpls-fib entries value-typed |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| SR codec round-trips every TLV/sub-TLV | `go test ./internal/plugins/ospf/sr -run 'RoundTrip'` |
| Multi-range index->label correct | `go test ./internal/plugins/ospf/sr -run 'TestSRLabelFromIndex'` |
| MPLS install via mpls-fib | `grep -rn 'mplsSourceOSPFSR' internal/plugins/ospf/sr` + `go test ./internal/plugins/ospf/sr -run 'Install'` |
| Adj-SID withdraw on adjacency loss | `go test ./internal/plugins/ospf/sr -run 'TestSRAdjSIDWithdrawn'` |
| Seven SR metric series registered | `grep -rn 'ze_ospf_sr_' internal/plugins/ospf` |
| SR snapshot + CLI | `ls test/ospf/ospf-sr-*.ci` + `grep -rn 'segment-routing' internal/plugins/ospf/yang` |
| Interop scenario present | `ls test/interop/scenarios/ospf-sr-frr/` |
| Self-contained | build without the `sr/` package; OSPF suite green (`TestOSPFBuildsWithoutSR`) |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | SR TLV/sub-TLV parsing bound-checked over the ext-1 iterator; malformed lengths/SID fields make the LSA malformed (ext-4 drops it); the OSPF packet fuzz target extended with SR bodies |
| Data-plane safety | SR programs the MPLS data plane: a wrong label misroutes traffic. Validate every label/index against the originator's advertised SRGB; never install a label for an unsupported algorithm or a non-SR next-hop; gate on reachability |
| Resource exhaustion | the SRLB allocator is bounded by the configured range and reports exhaustion; a flood of remote prefix-SIDs cannot grow SR state unbounded (bounded by the LSDB cap the carriers enforce) |
| Trust boundary | received SR TLVs ride the existing OSPF authentication (RFC 7474 SHOULD be used, §10); no new auth surface; SR install never bypasses fib-kernel ownership |
| Error leakage | label-compute / malformed-TLV errors are counted (`ze_ospf_sr_*_total`), rate-limited in logs (§9/§10), not surfaced to peers |

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
OSPF SR is a *consumer with a data-plane tail*: the control plane is pure TLV
codec over three delivered carriers (ext-1/ext-3/ext-4), and the only genuinely
new mechanism is the index->label arithmetic plus the install through the
existing `mpls-fib` bus. The MPLS integration is not new plumbing -- SR is
simply the third producer (after RSVP-TE and LDP) on a seam fib-kernel already
owns. The two error-prone pieces are the multi-range SRGB mapping and the
NP/E/M PHP truth table; both get exhaustive boundary tests and an FRR
label-agreement interop.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| SR is a new `internal/plugins/ospf/sr/` consumer, not OSPF-core changes | bake SR into the RI/Extended carriers | plugin-self-containment + RFC 8665's layering: SR owns its TLVs and forwarding; carriers stay SR-agnostic; removing `sr/` removes SR |
| Forwarding installs through the existing `mpls-fib` bus | a new SR-specific FIB path / direct netlink | fib-kernel is the single netlink owner; SR is the third producer (RSVP-TE=1, LDP=2) with a distinct source tag; no duplicated forwarding code |
| The SRLB Adj-SID allocator reuses the LDP bounded-pool pattern | a new allocator with persistence | the LDP `nextLabel`/`MaxLabel` shape already solves bounded 20-bit allocation; persistence (P-Flag) is a follow-up |
| The SRGB is a single configured range on originate; multiple on receive | dynamic SRGB allocation | operational norm is one configured SRGB block; the receive path MUST handle multiple ranges (interop) but originate stays simple (A-8) |
| Ze computes only Algorithm 0; unsupported-algorithm prefix-SIDs are recorded but not installed | install all advertised SIDs | §5 MUST ignore a Prefix-SID whose algorithm is not advertised / not computed; strict-SPF (Alg 1) is out of scope |
| The SR install fires from a post-SPF-run hook AFTER the IP-route Installer | recompute SR on every LSDB change | SR pushes ride existing IP routes; installing after `Apply` avoids the push-before-route race (R-8) |

## Known Limitations
- TI-LFA / backup paths are not computed (the Adj-SID B-Flag is advertised but no protection path is installed) -- ext-6.
- SR-TE policies (segment lists, binding SIDs) are not part of OSPF SR -- a separate subsystem.
- SRv6 is not applicable to OSPFv2; OSPFv3 SR (RFC 8666) is separate.
- Strict-SPF (Algorithm 1) paths are not computed; such prefix-SIDs are recorded but not installed.
- SR Mapping Server arbitration beyond first-occurrence/narrowest-scope (full RFC 8661) is deferred; only the wire carriage is implemented.
- The SRGB is originated as a single configured range (multiple ranges are accepted on receive).

## RFC Documentation
Add `// RFC 8665 Section X.Y: "<quoted requirement>"` above the enforcing code:
- §3.1 SR-Algorithm MUST include Algorithm 0; first-occurrence/area-scope/Instance-ID tie-break
- §3.2 Range Size > 0; exactly one SID/Label Sub-TLV; ranges concatenated in advertised order; stable order across restart
- §3.3 SRLB Range Size > 0; Adj-SIDs from the SRLB
- §3.4 SRMS-Pref first-occurrence / narrowest-scope tie-break
- §4 all prefix ranges in one LSA share scope; ABR sets the IA-Flag between areas
- §5 V/L validation (only 0/0 and 1/1); algorithm-not-advertised ignore; duplicate prefix/topology/algorithm all ignored; NP/E/M outgoing-label truth table; NP set + E clear for ABR/ASBR non-attached prefix-SIDs
- §6.1/§6.2 Adj-SID/LAN-Adj-SID flags; P-Flag persistence
- §7.4.1 withdraw the Adj-SID when the adjacency drops below 2-Way
- §9/§10 malformed TLV/sub-TLV -> LSA malformed, no crash, count/log rate-limited
Also `// RFC 7770` (RI LSA carriage, area scope) and `// RFC 7684` (Extended Prefix/Link sub-TLV carriage, malformed-LSA drop) where SR rides those carriers.

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
| Advertise SR-Algorithm/SRGB/SRLB/SRMS in the RI LSA | unit + functional | `TestSRAlgorithmTLVRoundTrip`, `TestSRGBRangeTLVRoundTrip`, `ospf-sr-originate.ci` |
| Prefix-SID in Extended Prefix; Adj-SID/LAN-Adj-SID in Extended Link | unit + interop | `TestPrefixSIDSubTLVRoundTrip`, `TestAdjSIDSubTLVRoundTrip`, `ospf-sr-frr` |
| Compute MPLS labels from prefix-SID index against the SRGB | unit | `TestSRLabelFromIndexMultiRange`, `TestSRLabelIndexOutOfRange` |
| Program SR forwarding into the MPLS plane | functional + interop | `ospf-sr-receive.ci` (mpls-fib push), `ospf-sr-frr` (label-switched ping) |
| Adj-SID lifecycle on adjacency state | unit + functional | `TestSRAdjSIDWithdrawnBelow2Way`, `ospf-sr-adj.ci` |
| CLI + metrics | functional | `ospf-sr-show.ci`, `ze_ospf_sr_*` series |

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
- [ ] AC-1..AC-17 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/plugins/ospf/sr/*`, `internal/plugins/ospf/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass)
- [ ] RFC 8665 / 7770 / 7684 constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (SR is the first MPLS-programming OSPF consumer; the codec/install split is justified by the test surface)
- [ ] No speculative features (no TI-LFA, no SR-TE, no SRv6, no strict-SPF install)
- [ ] Single responsibility per component (codec / srgb / srlb / install / adjsid separated)
- [ ] Explicit > implicit behavior (the NP/E/M truth table is explicit, not inferred)
- [ ] Minimal coupling (carriers/core name nothing SR)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (`ospf-sr-frr`)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ospf-ext-5-segment-routing.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospf-ext-5-segment-routing.md`
