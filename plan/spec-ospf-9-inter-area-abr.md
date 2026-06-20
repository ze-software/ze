# Spec: ospf-9-inter-area-abr

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-ospf-8-spf-rib.md |
| Phase | - |
| Updated | 2026-06-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-ospf-0-umbrella.md` - `## Shared Contracts`: "LSA inventory" (Type 3 Summary-Network, Type 4 Summary-ASBR rows), "LSA header + body layout" (Summary-LSA Type 3/4 = Network Mask + TOS + 3-byte Metric; Link State ID = network for Type 3, ASBR Router ID for Type 4), "Route preference / path types" (intra-area > inter-area > external, resolved INSIDE OSPF SPF, one `locrib.Path` per prefix, AdminDistance 110), "Metrics (canonical)" rows owned here (`ze_ospf_abr`, `ze_ospf_summary_lsas{area}`), and the ospf-9 Child-Spec / Dependency-Graph rows
4. `docs/research/ospf-implementation-guide.md` §6c Inter-area Route Computation (~403-411), §6e ABR and ASBR Behaviour (~424-431), trap #8 "ABR Acceptance Rule" (~1476-1478)
5. `plan/spec-ospf-8-spf-rib.md` - the intra-area SPF route table, path types, and the Loc-RIB install path this spec extends (inter-area routes are inserted through the SAME `locrib.Path` install seam ospf-8 owns)
6. `plan/spec-ospf-7-lsdb-flooding.md` - the LSDB store + self-LSA origination + §13 flooding this spec drives (Type 3/4 Summary-LSAs are originated INTO the LSDB and flooded by the ospf-7 machinery)
7. RFC 2328 §16.2 (inter-area route calc), §16.3 (ABR examines backbone summaries only), §3.3 (area data structure), §12.4.3 (Summary-LSA origination)

## Task

Make a Ze OSPF node behave correctly as an **Area Border Router (ABR)** and
compute **inter-area** routes, per RFC 2328 §16.2 / §16.3 and the
`docs/research/ospf-implementation-guide.md` §6c/§6e ABR behaviour. This is the
multi-area spec: ospf-8 delivered intra-area SPF and the Loc-RIB install seam for
a single area; this spec adds the Type 3 / Type 4 Summary-LSA origination that
glues areas together and the inter-area route computation that consumes them.

A router is an **ABR** when it has OSPF-enabled interfaces in two or more areas,
at least one of which is the backbone (area `0.0.0.0`). On becoming an ABR the
router sets the **B-bit** (bit 0x01) in the flags byte of its Router-LSA in every
attached area (the Router-LSA body layout is the umbrella "LSA header + body
layout" V/E/B flags byte; this spec only flips the B-bit, ospf-7 owns the
Router-LSA re-origination it triggers), and maintains the `ze_ospf_abr` gauge (1
when ABR, 0 otherwise).

As an ABR the router **originates Type 3 (Summary-Network) LSAs**: into each
attached area it summarises the networks it can reach in OTHER areas. Per
RFC 2328 §12.4.3, for every intra-area network reachable in some area X the ABR
originates a Type 3 into every OTHER attached area, with Link State ID = the
network address, the body's Network Mask = the network mask, and the Metric = the
ABR's own intra-area cost to that network (the §16.1 route-table cost). When the
ABR generates a Type 3 into a **non-backbone** area it ALSO re-advertises the
inter-area networks it learned from the backbone (so spoke areas see remote-area
prefixes); when it generates into the **backbone** it advertises only its
intra-area networks of non-backbone areas (the backbone re-distribution to other
areas is then done by those areas' ABRs). A network that becomes unreachable is
withdrawn by re-originating its Type 3 at LS Age MaxAge (3600) so the §14 / §13
flush propagates and neighbours purge it.

As an ABR the router **originates Type 4 (Summary-ASBR) LSAs**: when an ASBR
exists in some area X (a Router-LSA with the E-bit set, located in the §16.1
intra-area route table or, on the backbone, the inter-area ASBR route), the ABR
advertises reachability to that ASBR into its OTHER attached areas, with Link
State ID = the ASBR's Router ID, the body's Network Mask = `0.0.0.0`, and the
Metric = the ABR's cost to the ASBR. This is what lets routers in a remote area
resolve the ASBR a Type 5 / Type 7 external points at (the actual external route
computation that consumes the Type 4 is ospf-10 §16.4).

The router also performs **inter-area route computation** (RFC 2328 §16.2): it
examines the Type 3 Summary-LSAs in its attached areas; for each, the candidate
inter-area cost is `cost-to-the-advertising-ABR (from the §16.1 intra-area route
table) + the summary's advertised metric`; the resulting route is installed as an
**inter-area** route (path type below intra-area, above external; the umbrella
"Route preference / path types" contract resolves this INSIDE OSPF SPF and
publishes one winning `locrib.Path` per prefix with AdminDistance 110 regardless
of path type). The Type 4 summaries are used the same way to build a route to the
ASBR so ospf-10 can later resolve externals. **Trap #8 (ABR acceptance rule,
RFC 2328 §16.3):** when the calculating router is ITSELF an ABR it considers ONLY
the Type 3 / Type 4 summaries it received **over the backbone (area `0.0.0.0`)**
for inter-area route computation; summaries received through a non-backbone area
are stored in the LSDB (for re-flooding) but are NOT used to compute routes. This
prevents inter-area loops when a non-backbone area has multiple ABRs advertising
the same prefix at different metrics.

**Area ranges** (RFC 2328 §3.5 / §12.4.3): the operator can configure address
ranges per area (the umbrella "Area + interface config model" `areas/area/ranges/range`
already in the ospf-4 schema). When a range with `advertise` covers one or more
component intra-area prefixes that the ABR would otherwise summarise, the ABR
originates a SINGLE Type 3 for the aggregate range (LS ID = the range
prefix/mask) instead of one Type 3 per component; the aggregate metric is the
configured range cost if set, else the maximum cost among the covered components
(RFC 2328 §12.4.3: "the cost ... is the largest cost of any of the component
networks"). A range with `not-advertise` SUPPRESSES the summary entirely (no
Type 3 for the range or its components). Component prefixes outside any range are
summarised individually as before.

**Backbone-attachment requirement:** RFC 2328 §16.2/§16.3 inter-area routing is
correct only when every ABR is attached to the backbone. Virtual links
(RFC 2328 §15), which repair a partitioned/detached backbone, are OUT OF SCOPE in
v1 (umbrella "Out of scope" table). This spec therefore requires that an ABR have
a real backbone interface; an ABR whose only path to area 0 would be a virtual
link is a known limitation (documented, not implemented). The B-bit / ABR
detection follows the §3.3 area-data-structure rule literally: "actively attached
to two or more areas" with one being the backbone.

Finally, this spec adds **`show ip ospf border-routers`** (the route-table entries
for ABRs and ASBRs reachable from this router, the standard OSPF diagnostic) and
owns the two Prometheus gauges `ze_ospf_abr` and `ze_ospf_summary_lsas{area}`.

Package: `internal/component/ospf/spf/` (the SAME package as ospf-8; this spec
adds the inter-area files `interarea.go` + `summary.go` and extends the route
table and install seam), with origination driving the ospf-7 LSDB store.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `docs/research/ospf-implementation-guide.md` §6c Inter-area Route Computation (~403-411) - the IA calc and the ABR-only-backbone rule
  → Decision: inter-area routes use `abr_cost + summary_metric`; origination is symmetric (every intra-area prefix in area X → a Type 3 into every other attached area, subject to stub/NSSA filters and ranges)
  → Constraint: at an ABR, Type 3 LSAs from non-backbone areas are IGNORED for route computation; ABRs compute IA routes from the BACKBONE's Type 3 LSAs only (trap #8)
- [ ] `docs/research/ospf-implementation-guide.md` §6e ABR and ASBR Behaviour (~424-431) - the ABR definition and responsibility list
  → Constraint: an ABR is a router with interfaces in more than one area, at least one of which is the backbone; the B flag in the Type 1 Router-LSA marks it; per intra-area prefix → Type 3 into every other area, per known ASBR → Type 4 into every other area (NSSA Type 7→5 translation is ospf-11, NOT here)
- [ ] `docs/research/ospf-implementation-guide.md` trap #8 "ABR Acceptance Rule" (~1476-1478) - loop prevention
  → Constraint: only summaries received THROUGH area 0 are used at an ABR; non-backbone summaries are stored for re-flooding but not used for route computation; prevents loops when a non-backbone area has multiple ABRs
- [ ] `plan/spec-ospf-0-umbrella.md` `## Shared Contracts` - "LSA inventory", "LSA header + body layout", "Route preference / path types", "Area + interface config model", "Metrics (canonical)"
  → Constraint: Type 3 LS ID = network address, Type 4 LS ID = ASBR Router ID; Summary body = Network Mask (4; 0.0.0.0 for Type 4) + TOS (1, 0) + 3-byte Metric; intra-area > inter-area > external preference resolved INSIDE OSPF SPF, one `locrib.Path` per prefix, AdminDistance 110; this spec OWNS exactly `ze_ospf_abr` (gauge, no labels) and `ze_ospf_summary_lsas` (gauge, label `area`)
- [ ] `ai/rules/buffer-first.md`, `ai/rules/memory-architecture.md` - zero-copy, lazy parse, no-alloc origination
  → Constraint: Type 3/4 origination encodes buffer-first into the LSDB store (ospf-7); inter-area computation reads LSDB raw bytes via the lazy accessors, no per-run re-alloc on the hot path

### RFC Summaries (MUST for protocol work; created via `/ze-rfc` at implementation time)
- [ ] `rfc/short/rfc2328.md` - OSPF Version 2: §3.3 area data structure (ABR definition), §3.5 area ranges, §12.4.3 Summary-LSA origination + range aggregation, §16.2 inter-area route calc, §16.3 ABR examines backbone summaries
  → Constraint: §16.3 "if the router is an area border router ... only consider summary-LSAs in the area 0.0.0.0 link-state database"; §12.4.3 aggregate metric = largest component cost (unless a configured range cost overrides); withdraw by re-originating at MaxAge

**Key insights:** (minimal context to resume after compaction)
- ABR = interfaces actively in ≥2 areas, one being the backbone (`0.0.0.0`); set the Router-LSA B-bit (ospf-7 re-originates); maintain `ze_ospf_abr`.
- Origination is symmetric to computation: per intra-area prefix in area X → Type 3 (LS ID = network, metric = §16.1 cost) into every OTHER attached area; into non-backbone areas ALSO re-advertise backbone-learned inter-area prefixes; per known ASBR → Type 4 (LS ID = ASBR Router ID, mask 0.0.0.0). Withdraw via MaxAge re-origination.
- Inter-area route cost = `cost-to-ABR (§16.1) + summary metric`; installed as inter-area (below intra-area). Trap #8: at an ABR, ONLY backbone (area 0) summaries are used for route computation; non-backbone ones are stored but ignored.
- Area ranges aggregate components into one Type 3 (configured cost or max component cost); `not-advertise` suppresses; v1 requires a real backbone interface (no virtual links).
- Preference resolved inside OSPF SPF → one `locrib.Path` per prefix, AdminDistance 110 (ospf-8 install seam reused, no new sysrib work).

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec; ospf-8 is the dependency and owns the package this spec extends)
<!-- Same rule: never tick [ ] to [x]. Write → Constraint: annotations instead. -->
- [ ] `plan/spec-ospf-8-spf-rib.md` - intra-area two-stage Dijkstra (§16.1) over Router/Network-LSAs, the per-area route table with path types, and the Loc-RIB install seam (`locrib.Path{Source = OSPF ProtocolID, Instance per ECMP next-hop, NextHop, AdminDistance 110, Metric}` → `InsertForward`; ECMP path-group expansion reused from sysrib)
  → Constraint: this spec does NOT re-implement intra-area SPF or the install seam; it READS the §16.1 intra-area route table (the source of "cost to a network" and "cost to an ABR/ASBR") and APPENDS inter-area route entries before the single per-prefix `locrib.Path` is published; the intra > inter preference is resolved by the route-table candidate compare ospf-8 owns
- [ ] `plan/spec-ospf-7-lsdb-flooding.md` - per-area LSDB store (lazy raw bytes + metadata), self-LSA origination, §13 flooding, MaxAge walker, the Router-LSA origination (whose flags byte carries the B-bit)
  → Constraint: Type 3/4 origination hands a built LSA to the ospf-7 LSDB store, which floods it via §13; withdrawal re-originates at MaxAge so the existing §14/§13 flush + purge runs; this spec does NOT add a second flooding path
- [ ] `plan/spec-ospf-0-umbrella.md` `## Shared Contracts` - the canonical contracts listed above (LSA layout, route preference, area config model, metrics)
  → Constraint: reference by name; never redefine the Summary-LSA body, the preference order, the area-range config leaves, or the metric series
- [ ] `internal/core/rib/locrib/candidate.go` - the `locrib.Path{Source, Instance, NextHop, AdminDistance, Metric}` value type and the best-path candidate compare (lower AdminDistance, then lower Metric, then first-seen) that the ospf-8 install seam targets
  → Constraint: inter-area routes are published as the SAME `locrib.Path` shape (AdminDistance 110, one Path per ECMP next-hop, distinct `Instance`); there is NO path-type field on `locrib.Path`, so the intra > inter > external preference is resolved INSIDE OSPF SPF before publishing one winning Path per prefix
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` (~813 `InsertForward`) - the Loc-RIB insertion pattern the ospf-8 seam mirrors and this spec reuses unchanged for inter-area routes
  → Constraint: this spec does NOT add a new `InsertForward` call site; it appends inter-area candidates to the ospf-8 route table, which then publishes through the existing seam

**Behavior to preserve:** (unless user explicitly said to change)
- The intra-area §16.1 SPF, the per-area route table, and the Loc-RIB install seam (ospf-8) are unchanged in shape; this spec appends inter-area entries through the same route-table candidate compare and the same single-`locrib.Path`-per-prefix publish.
- The ospf-7 LSDB store, §13 flooding, MaxAge walker, and Router-LSA origination are reused unchanged; Type 3/4 LSAs are just more self-originated LSAs flowing through that machinery, and the B-bit is one flag in the existing Router-LSA flags byte.
- The umbrella "Route preference / path types" contract: OSPF resolves intra > inter > external INTERNALLY and publishes one winning `locrib.Path` per prefix with AdminDistance 110; this spec adds inter-area candidates to that internal resolution and does NOT introduce a second admin distance or a `locrib.Path` path-type field.
- No sysrib / locrib / `ze-rib-conf.yang` change (the existing `rib.admin-distance.ospf` leaf, default 110, and the existing ECMP path-group expansion are reused exactly as ospf-8 set up).

**Behavior to change:** (only if user explicitly requested)
- New inter-area files in `internal/component/ospf/spf/` (`interarea.go`, `summary.go`) plus an ABR-detection hook in the instance/area runtime.
- The Router-LSA flags byte gains the B-bit when the router becomes an ABR (origination owned by ospf-7; this spec supplies the ABR predicate that drives it).
- New self-originated Type 3 / Type 4 Summary-LSAs flow into the ospf-7 LSDB store; new inter-area route-table entries flow into the ospf-8 route table.
- New `show ip ospf border-routers` snapshot RPC (rendered/registered with the other show RPCs; the command YANG binding is owned by ospf-13 per the umbrella command-ownership contract).
- New metrics `ze_ospf_abr` and `ze_ospf_summary_lsas{area}` (owned and registered here).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- An intra-area SPF run completes (ospf-8) for some area, producing/refreshing the §16.1 intra-area route table → triggers Type 3/4 (re-)origination and the inter-area route computation.
- An LSDB change to a Type 3 / Type 4 Summary-LSA in an attached area (received via ospf-7 flooding) → triggers re-running the inter-area route computation.
- A config change to areas (an area added/removed, a `ranges/range` added/changed/removed) → re-evaluates the ABR predicate and re-originates / withdraws summaries.

### Transformation Path
1. **ABR detection:** evaluate the predicate "OSPF-enabled interfaces actively attached to ≥2 areas, one being `0.0.0.0`" (RFC 2328 §3.3). On a transition, set/clear the Router-LSA B-bit (drives ospf-7 Router-LSA re-origination) and update `ze_ospf_abr`.
2. **Source set:** read the ospf-8 §16.1 intra-area route table for every attached area: the reachable networks (with their intra-area cost) and the located ASBRs (Router-LSA E-bit, with cost to reach them).
3. **Range aggregation:** for each attached area's source networks, fold component prefixes that fall inside a configured `advertise` range into one aggregate (metric = configured range cost, else max covered-component cost); drop components covered by a `not-advertise` range; leave the rest individual.
4. **Type 3 origination:** for each attached area A, for every network reachable in a DIFFERENT area (intra-area in area X≠A, plus, when A is non-backbone, the inter-area networks learned from the backbone), build a Type 3 (LS ID = network address, body Network Mask + TOS 0 + 3-byte Metric = ABR cost) and hand it to the ospf-7 LSDB store for area A → §13 flooding. Increment `ze_ospf_summary_lsas{area=A}`.
5. **Type 4 origination:** for each attached area A, for every known ASBR in a DIFFERENT area, build a Type 4 (LS ID = ASBR Router ID, body Network Mask = `0.0.0.0` + TOS 0 + 3-byte Metric = ABR cost to the ASBR) and hand it to the ospf-7 LSDB store for area A.
6. **Withdraw:** a summary whose underlying network/ASBR became unreachable (or whose range turned `not-advertise`, or area removed) is re-originated at LS Age MaxAge (3600) into the ospf-7 store → §14/§13 flush; neighbours purge.
7. **Inter-area computation (§16.2):** examine the Type 3 summaries in attached areas. For a NON-ABR calculating router: for each Type 3, candidate cost = `cost-to-advertising-ABR (§16.1) + summary metric`; the next-hop is the §16.1 next-hop toward that ABR. For an ABR calculating router (trap #8 / §16.3): consider ONLY Type 3/4 summaries in the **backbone** (area `0.0.0.0`) LSDB; ignore non-backbone summaries for computation (they remain stored for re-flooding).
8. **Route-table merge (preference):** append each inter-area result as an inter-area candidate to the ospf-8 per-prefix route table; the route-table candidate compare (owned by ospf-8) prefers intra-area over inter-area over external, so the single winning `locrib.Path` per prefix carries the right path type's next-hop/metric. Type 4 results build a route to the ASBR (consumed by ospf-10 §16.4, not installed as a forwarding route here).
9. **Install:** the ospf-8 install seam publishes one `locrib.Path{Source = OSPF ProtocolID, Instance, NextHop, AdminDistance 110, Metric}` per prefix (one per ECMP next-hop) via `InsertForward`; inter-area routes reach the kernel exactly as intra-area ones do. No new install code.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| ospf-8 intra-area route table ↔ this spec | in-process read of the §16.1 route table (cost-to-network, cost-to-ABR/ASBR, next-hops); append inter-area entries (same package) | [ ] |
| this spec ↔ ospf-7 LSDB store | hand a built Type 3/4 LSA (buffer-first bytes) to the per-area store → §13 flooding; withdraw via MaxAge re-origination | [ ] |
| this spec ↔ ospf-7 Router-LSA origination | supply the ABR predicate that flips the B-bit in the Router-LSA flags byte | [ ] |
| OSPF SPF ↔ Loc-RIB (FIB install) | reuse the ospf-8 `locrib.Path` install seam; one Path per prefix, AdminDistance 110 (no new code) | [ ] |
| engine ↔ CLI | `show ip ospf border-routers` snapshot RPC (registration in Go; command YANG owned by ospf-13) | [ ] |

### Integration Points
- `internal/component/ospf/spf/` (extends the ospf-8 package): inter-area computation + Summary-LSA origination read the §16.1 route table and write the per-area route table + the ospf-7 LSDB store.
- The ospf-7 LSDB store + §13 flooding + MaxAge walker (origination/withdrawal of Type 3/4).
- The ospf-8 Loc-RIB install seam (`InsertForward`, AdminDistance 110) reused unchanged for inter-area routes.
- The ospf-4 area/range config (`areas/area/ranges/range`, the umbrella "Area + interface config model"); no new config leaves are added here (ranges already modelled in ospf-4).
- `show ip ospf border-routers` snapshot RPC (rendered + command-YANG-bound by ospf-13).
- Prometheus gauges `ze_ospf_abr` and `ze_ospf_summary_lsas{area}` owned and registered HERE per the umbrella canonical table; ospf-13 only scrapes/asserts them.

### Architectural Verification
- [ ] No bypassed layers (route table → ospf-8 install seam → sysrib → fibkernel; origination → ospf-7 LSDB store → §13 flooding; no second FIB path, no second flooding path)
- [ ] No unintended coupling (inter-area logic reads the ospf-8 route table + ospf-7 LSDB; no direct netlink; no redistevents on this path; OSPF independent of IS-IS)
- [ ] No duplicated functionality (intra-area SPF, install seam, flooding, MaxAge walker all reused; this spec adds only inter-area computation + Summary-LSA origination)
- [ ] Zero-copy preserved (Type 3/4 origination is buffer-first into the LSDB store; inter-area computation reads LSDB raw bytes via lazy accessors)

## Risks & Assumptions

<!-- LIVE -- written during RESEARCH/DESIGN, statuses updated during implementation. -->

### Assumptions
<!-- Status: unvalidated → confirmed | broken. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The ospf-8 §16.1 intra-area route table exposes, per attached area, the reachable networks with their intra-area cost AND the located ASBRs (Router-LSA E-bit) with cost-to-reach, sufficient to source Type 3/4 origination and to provide cost-to-ABR for §16.2 | `plan/spec-ospf-8-spf-rib.md` route-table-with-path-types design; RFC 2328 §16.1 | Need to extend the ospf-8 route table to expose ASBR vertices and per-area network costs | `TestOSPFType3Origination` + `TestOSPFInterAreaRoute` reading a hand-built route table | unvalidated |
| A-2 | Inter-area routes install through the SAME ospf-8 `locrib.Path` seam (AdminDistance 110, one Path per prefix); the intra > inter preference is resolved by the ospf-8 route-table candidate compare, so this spec adds candidates rather than a second install path | umbrella "Route preference / path types"; `plan/spec-ospf-8-spf-rib.md` install seam | A separate install path or a `locrib.Path` path-type field would be needed (out of v1 scope) | `TestOSPFInterAreaPreference` (intra wins over inter for the same prefix) + `ospf-inter-area.ci` | unvalidated |
| A-3 | Type 3/4 origination can hand a buffer-first LSA to the ospf-7 per-area LSDB store and rely on §13 flooding + the MaxAge walker for distribution and withdrawal (no new flooding code) | `plan/spec-ospf-7-lsdb-flooding.md` self-LSA origination + flooding + MaxAge | Need a Summary-LSA-specific origination/flood path | `TestOSPFSummaryFlood` + `TestOSPFSummaryWithdraw` (MaxAge re-origination) | unvalidated |
| A-4 | The Router-LSA flags byte (V/E/B) is owned by ospf-7 origination and this spec only supplies the ABR predicate to set/clear the B-bit; toggling it re-originates the Router-LSA in every attached area | umbrella "LSA header + body layout" (Router-LSA flags byte); `plan/spec-ospf-7-lsdb-flooding.md` | Need to add B-bit handling to the Router-LSA codec/origination here | `TestOSPFABRBitSet` (B-bit set when ABR, cleared when not) | unvalidated |
| A-5 | The `areas/area/ranges/range` config (prefix + `advertise`/`not-advertise` + optional cost) from ospf-4 is sufficient for range aggregation; no new config leaves are needed in this spec | umbrella "Area + interface config model"; `plan/spec-ospf-4-component-config.md` schema | Need to add range leaves to `ze-ospf-conf.yang` (would be an ospf-4 change) | `TestOSPFAreaRangeAggregate` + `TestOSPFAreaRangeNotAdvertise` reading the resolved config | unvalidated |

### Risks
<!-- From Failure Mode Analysis. Surviving risks copy forward to the Executive Summary + learned summary. -->
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Trap #8 omitted: an ABR uses non-backbone summaries for route computation → inter-area loop when a non-backbone area has multiple ABRs advertising the same prefix | a route oscillates / loops in a two-ABR non-backbone topology; suboptimal IA path | Enforce §16.3 in step 7 (ABR considers ONLY area-0 summaries); `TestOSPFABRBackboneOnlyAcceptance` with a two-ABR non-backbone topology |
| R-2 | Summary not withdrawn when the underlying network/ASBR becomes unreachable → stale inter-area route persists | prefix still in peers' RIBs after the originating network dies | Re-originate the Type 3/4 at MaxAge on loss (step 6); `TestOSPFSummaryWithdraw` + the withdraw step of `ospf-inter-area.ci` |
| R-3 | Range aggregate metric wrong (not the max component cost, or ignores the configured range cost) → suboptimal or black-holed inter-area routing | aggregate metric lower/higher than every component; traffic mis-steered | §12.4.3 aggregate-metric rule in step 3; `TestOSPFAreaRangeAggregate` asserts max-component (and configured-cost override) |
| R-4 | `not-advertise` range fails to suppress (a Type 3 still leaks for a covered component) | a suppressed prefix appears in a remote area | Drop covered components in step 3 before origination; `TestOSPFAreaRangeNotAdvertise` |
| R-5 | Type 3 LS ID collision: two networks with the same address but different masks would collide on LS ID = network address (RFC 2328 §12.4.3 note) | one summary overwrites another; a prefix disappears | Follow the §12.4.3 LS-ID-collision handling (set host bits in the LS ID to disambiguate); `TestOSPFSummaryLSIDCollision` |
| R-6 | ABR predicate wrong (e.g. treats a router with two non-backbone areas and no backbone as an ABR, or misses a loopback-only backbone) → wrong B-bit, wrong summaries | B-bit set on a non-ABR; summaries originated by a non-ABR | RFC 2328 §3.3 predicate (≥2 active areas, one being `0.0.0.0`) in step 1; `TestOSPFABRDetection` covers the no-backbone and backbone-only cases |
| R-7 | Inter-area cost uses the wrong base (cost to the network instead of cost to the advertising ABR) → wrong IA metric | IA metric off by the intra-area component | §16.2 `cost-to-ABR + summary metric` in step 7; `TestOSPFInterAreaRoute` asserts the composed cost |

## Wiring Test (MANDATORY — NOT deferrable)

<!-- Proves the feature is reachable from its intended entry point. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| router has interfaces in area 0 + a non-backbone area | → | ABR predicate true → Router-LSA B-bit set, `ze_ospf_abr`=1 | `TestOSPFABRDetection` |
| intra-area SPF completes on an ABR | → | Type 3 (and Type 4 when an ASBR exists) originated into every OTHER area, handed to the ospf-7 LSDB store | `TestOSPFType3Origination`, `TestOSPFType4Origination` |
| Type 3 summaries present in an attached area | → | inter-area route computed (`abr_cost + metric`), appended to the ospf-8 route table, installed via the `locrib.Path` seam | `TestOSPFInterAreaRoute`, `test/ospf/ospf-inter-area.ci` |
| `show ip ospf border-routers` invoked | → | snapshot lists reachable ABRs/ASBRs with area, cost, next-hop | `test/ospf/ospf-inter-area.ci` (border-routers step) |

## Acceptance Criteria

<!-- Each row is a testable assertion. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Router with interfaces in area `0.0.0.0` and a non-backbone area | ABR predicate true; the Router-LSA B-bit is set in every attached area; `ze_ospf_abr`=1. A router with only non-backbone areas (no area 0) is NOT an ABR; `ze_ospf_abr`=0 |
| AC-2 | An ABR with an intra-area network in area X | A Type 3 (LS ID = network address, body Network Mask, TOS 0, 3-byte Metric = the ABR's §16.1 cost to the network) is originated into every OTHER attached area, flooded via the ospf-7 §13 machinery; `ze_ospf_summary_lsas{area}` increments |
| AC-3 | An ASBR (Router-LSA E-bit) located in area X, ABR also attached to area Y | A Type 4 (LS ID = ASBR Router ID, body Network Mask `0.0.0.0`, TOS 0, 3-byte Metric = cost to the ASBR) is originated into area Y |
| AC-4 | A NON-ABR router sees a Type 3 for a remote-area prefix | An inter-area route is installed with cost = `cost-to-advertising-ABR + summary metric`, next-hop toward the ABR, tagged inter-area; appears in the kernel FIB as OSPF (`RTPROT_ZE`, AdminDistance 110) |
| AC-5 | The same prefix is reachable both intra-area and inter-area | The intra-area route wins (preference resolved inside OSPF SPF); exactly one `locrib.Path` is published for the prefix |
| AC-6 | This router is an ABR; the same prefix is summarised into a non-backbone area by two ABRs and is also present on the backbone | Only the BACKBONE (area `0.0.0.0`) summary is used for inter-area route computation (trap #8 / §16.3); the non-backbone summaries are stored but not used; no loop forms |
| AC-7 | An area `advertise` range covers two component intra-area prefixes | A SINGLE aggregate Type 3 (LS ID = range prefix/mask) is originated with metric = the configured range cost if set, else the maximum component cost; the components are not advertised individually |
| AC-8 | An area `not-advertise` range covers a component prefix | No Type 3 is originated for the range or its covered components (suppressed); uncovered components are still summarised individually |
| AC-9 | A summarised network (or ASBR) becomes unreachable | Its Type 3 (or Type 4) is re-originated at LS Age MaxAge (3600); the §14/§13 flush propagates and the inter-area route is withdrawn from the FIB |
| AC-10 | `show ip ospf border-routers` invoked | The snapshot lists each reachable ABR and ASBR with its Router ID, the area the route was computed in, the cost, and the next-hop |
| AC-11 | An ABR has no real backbone interface (would need a virtual link) | The ABR is treated per the v1 limitation: inter-area routing assumes a real backbone attachment; the virtual-link repair is documented as out of scope, not silently mis-computed |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Joins two areas with an ABR and expects each area to see the other's prefixes | intra-area SPF (ospf-8) → Type 3 origination (this spec) → ospf-7 §13 flooding → peer's inter-area computation → `locrib.Path` install → kernel | `test/ospf/ospf-inter-area.ci` |
| 2 | Configures an area `advertise` range and expects one aggregate route | resolved config → range fold (this spec) → single aggregate Type 3 → peer sees one inter-area route for the range | `test/ospf/ospf-inter-area.ci` (range step) |
| 3 | Loses a network behind the ABR and expects the inter-area route withdrawn | network unreachable → Type 3 MaxAge re-origination → §14/§13 flush → peer withdraws → route removed from the kernel | `test/ospf/ospf-inter-area.ci` (withdraw step) |
| 4 | Runs `show ip ospf border-routers` | CLI → RPC → border-router snapshot (this spec; rendering owned by ospf-13) | `test/ospf/ospf-inter-area.ci` (border-routers step) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOSPFABRDetection` | `internal/component/ospf/spf/interarea_test.go` | ABR predicate: ≥2 active areas one being `0.0.0.0`; no-backbone case is NOT an ABR; transition updates `ze_ospf_abr` | |
| `TestOSPFABRBitSet` | `internal/component/ospf/spf/interarea_test.go` | the ABR predicate flips the Router-LSA B-bit (set when ABR, cleared when not) | |
| `TestOSPFType3Origination` | `internal/component/ospf/spf/summary_test.go` | per intra-area network in area X → Type 3 into every other area; LS ID = network, body Network Mask + TOS 0 + 3-byte metric = §16.1 cost | |
| `TestOSPFType4Origination` | `internal/component/ospf/spf/summary_test.go` | per known ASBR in area X → Type 4 into every other area; LS ID = ASBR Router ID, body Network Mask `0.0.0.0` | |
| `TestOSPFSummaryLSIDCollision` | `internal/component/ospf/spf/summary_test.go` | two networks colliding on LS ID = network address are disambiguated per §12.4.3 (host bits in the LS ID) | |
| `TestOSPFAreaRangeAggregate` | `internal/component/ospf/spf/summary_test.go` | an `advertise` range folds components into one Type 3 (LS ID = range); metric = configured cost, else max component cost | |
| `TestOSPFAreaRangeNotAdvertise` | `internal/component/ospf/spf/summary_test.go` | a `not-advertise` range suppresses the range and its covered components; uncovered components still summarised | |
| `TestOSPFSummaryFlood` | `internal/component/ospf/spf/summary_test.go` | an originated Type 3/4 is handed to the ospf-7 LSDB store and enters §13 flooding | |
| `TestOSPFSummaryWithdraw` | `internal/component/ospf/spf/summary_test.go` | a vanished network/ASBR re-originates its summary at MaxAge (3600), driving the §14/§13 flush | |
| `TestOSPFInterAreaRoute` | `internal/component/ospf/spf/interarea_test.go` | a NON-ABR's Type 3 yields an inter-area route, cost = `cost-to-ABR + summary metric`, next-hop toward the ABR | |
| `TestOSPFInterAreaPreference` | `internal/component/ospf/spf/interarea_test.go` | when a prefix is both intra- and inter-area, intra wins; exactly one `locrib.Path` is published | |
| `TestOSPFABRBackboneOnlyAcceptance` | `internal/component/ospf/spf/interarea_test.go` | trap #8: at an ABR, only area-0 summaries are used for computation; non-backbone summaries stored but ignored; a two-ABR non-backbone topology does not loop | |
| `TestOSPFType4RouteToASBR` | `internal/component/ospf/spf/interarea_test.go` | a Type 4 builds a route to the ASBR (cost-to-ABR + Type 4 metric) consumed by external computation (ospf-10), not installed as a forwarding route | |
| `TestOSPFBorderRouterSnapshot` | `internal/component/ospf/spf/interarea_test.go` | `show ip ospf border-routers` snapshot lists ABRs/ASBRs with Router ID, area, cost, next-hop | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Summary-LSA Metric (3-byte field) | 0..16777215 | 16777215 | N/A | N/A (3-byte field; range cost > 0xFFFFFF rejected at config) |
| Inter-area total cost (cost-to-ABR + summary metric) | 0..0xFFFFFF (LSInfinity 0xFFFFFF = unreachable) | 0xFFFFFE | N/A | 0xFFFFFF treated as unreachable (route not installed), no wrap |
| Area ID (dotted-quad; `0.0.0.0` = backbone) | 0.0.0.0..255.255.255.255 | 255.255.255.255 | N/A | N/A (4-byte field) |
| Active-area count for ABR predicate | 0..N | N | 0/1 (not an ABR) | N/A |
| `ze_ospf_abr` gauge | {0,1} | 1 | N/A | N/A |

### Functional Tests
<!-- Verify the feature works from the end-user perspective -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-inter-area` | `test/ospf/ospf-inter-area.ci` | two areas + an ABR: each area sees the other's prefixes as inter-area (Type 3) with the ABR as next-hop; an `advertise` range collapses to one aggregate; `not-advertise` suppresses; a lost network withdraws the inter-area route; `show ip ospf border-routers` lists the ABR/ASBR | |

### Interop Tests (MANDATORY for protocol features)
<!-- FRR ospfd interop for multi-area is owned by ospf-13 per the umbrella test wiring contract. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| (deferred to ospf-13) | `test/interop/scenarios/` | FRR `ospfd` | multi-area Type 3/4 summary exchange, area ranges, and inter-area convergence validated against FRR in the `ospf-multi-area-frr` scenario | |

### Future (if deferring any tests)
- FRR `ospfd` multi-area interop (Type 3/4 exchange, ranges, trap #8 two-ABR loop-freedom, convergence) is owned by ospf-13 (`ospf-multi-area-frr`); this spec proves ABR behaviour + inter-area routing with Ze-to-Ze unit and functional tests. Raw-IP / multicast end-to-end runs as a QEMU integration test (`ai/rules/qemu-testing.md`), Linux-only.
- Virtual-link backbone repair (RFC 2328 §15) is out of scope v1 (umbrella "Out of scope"); no test here.
- NSSA Type 7 handling and stub-area summary filtering (no-summary) are owned by ospf-11; this spec originates Type 3/4 for NORMAL areas only and leaves the stub/NSSA filtering hooks for ospf-11.

## Files to Modify
<!-- MUST include feature code, not only test files -->
- `internal/component/ospf/spf/` - EXTENDED with the inter-area files below; the ospf-8 route-table candidate compare gains an inter-area path-type candidate (the preference resolution itself is ospf-8-owned; this spec supplies inter-area candidates to it)
- `internal/component/ospf/instance.go` / `area.go` (ospf-4-owned scaffolding) - the ABR predicate hook is invoked on area/interface config change and on SPF completion; the result drives the ospf-7 Router-LSA B-bit and `ze_ospf_abr`. NOTE: NO change to `ze-ospf-conf.yang` (the `areas/area/ranges/range` leaves already exist from ospf-4) and NO change to `ze-rib-conf.yang` / sysrib / locrib (the existing `rib.admin-distance.ospf` leaf default 110 and the existing ECMP path-group expansion are reused)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | `areas/area/ranges/range` already in `ze-ospf-conf.yang` (ospf-4); the `show ip ospf border-routers` command YANG binding is owned by ospf-13 (`ze-ospf-cmd.yang`) |
| YANG validation constraints | No | range leaves (prefix, advertise/not-advertise, cost) are constrained in ospf-4 |
| YANG custom validators | No | not needed here |
| CLI commands/flags | Yes | `show ip ospf border-routers` snapshot RPC registered in Go (central `ze-show:ospf-border-routers`); command-YANG binding + render owned by ospf-13 |
| CLI grammar (action before identifier) | Yes | `show ip ospf border-routers` follows `ai/rules/cli-grammar.md` |
| Editor autocomplete | No | no new config leaves; `show ip ospf border-routers` completion is YANG-driven and owned by ospf-13 |
| Functional test for new RPC/API | Yes | `test/ospf/ospf-inter-area.ci` |
| Pipe completeness | Yes | `show ip ospf border-routers` output through `ApplyPipes`/`ProcessPipes` (render owned by ospf-13) |
| Env var registration | No | no env-only settings |
| Doctor check for runtime dependencies | No | install path reuses the ospf-8 seam + fibkernel; flooding reuses ospf-7; no new runtime dependency |
| Prometheus counters/metrics | Yes | this spec OWNS and registers `ze_ospf_abr` (gauge, no labels) and `ze_ospf_summary_lsas` (gauge, label `area`) per the umbrella "Metrics (canonical)" table. Per-owner registration here; ospf-13 only scrapes/asserts |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` (OSPF ABR / inter-area routing row) |
| 2 | Config syntax changed? | No | `areas/area/ranges/range` already documented by ospf-4 |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (`show ip ospf border-routers`, render owned by ospf-13) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (`show ip ospf border-routers`) |
| 5 | Plugin added/changed? | No | OSPF is a component, registered by ospf-4 |
| 6 | Has a user guide page? | Yes | `docs/guide/ospf.md` (ABR + inter-area + area ranges section) |
| 7 | Wire format changed? | No | the Summary-LSA codec is owned by ospf-2; this spec originates/consumes it |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc2328.md` (§3.3 ABR, §3.5 ranges, §12.4.3 summary origination, §16.2/§16.3 inter-area) |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (new `test/ospf/ospf-inter-area.ci`) |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` (OSPF multi-area / ABR row) |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` (OSPF inter-area computation appends to the route table before the single `locrib.Path` publish) |
| 13 | Route metadata keys added/changed? | No | inter-area routes use the existing OSPF route metadata |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` (`ze_ospf_abr`, `ze_ospf_summary_lsas{area}` owned and registered here; surfaced in ospf-13) |
| 15 | Registered plugin/event/command/capability changed? | Yes | `docs/plugin-overview.md` (`show ip ospf border-routers` show RPC) |
| 16 | Changed files referenced by doc source anchors? | No | grep at completion |
| 17 | Existing docs show examples for this area? | No | grep at completion |

## Files to Create
- `internal/component/ospf/spf/interarea.go` - ABR detection predicate, §16.2/§16.3 inter-area route computation (trap #8 backbone-only acceptance at an ABR), Type 4 route-to-ASBR, the `show ip ospf border-routers` snapshot
- `internal/component/ospf/spf/summary.go` - Type 3 (network) and Type 4 (ASBR) Summary-LSA origination into each attached area, area-range aggregation (`advertise` max-component / configured cost; `not-advertise` suppression), LS-ID-collision disambiguation, MaxAge withdrawal; hands built LSAs to the ospf-7 LSDB store; owns `ze_ospf_summary_lsas{area}` (and `interarea.go` owns `ze_ospf_abr`)
- `internal/component/ospf/spf/interarea_test.go`, `internal/component/ospf/spf/summary_test.go` - unit tests (built against a hand-built §16.1 route table + a hand-built LSDB)
- `test/ospf/ospf-inter-area.ci` - end-to-end: two areas + ABR, Type 3/4 exchange, area ranges, withdrawal, `show ip ospf border-routers`

Note: the Summary-LSA wire codec is owned by ospf-2; the LSDB store + §13 flooding + MaxAge walker are owned by ospf-7; the intra-area route table + the `locrib.Path` install seam are owned by ospf-8. This spec adds only the inter-area computation and the Summary-LSA origination on top of those.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan — check what ospf-7/ospf-8 already expose |
| 3. Wiring phase | Wiring Test table — ABR predicate hook + failing inter-area route test |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8-14. | Standard flow per `ai/rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** — ABR predicate hook + inter-area route stub + failing tests
   - Tests: `TestOSPFABRDetection`, `test/ospf/ospf-inter-area.ci` (fails: no summaries / no inter-area routes yet)
   - Files: `spf/interarea.go` (ABR predicate, register `ze_ospf_abr`, an inter-area compute entry-point stub), the instance/area hook that invokes it on config change + SPF completion
   - Verify: the ABR predicate runs, `ze_ospf_abr` registers and toggles, the Router-LSA B-bit flips; the inter-area test fails because no Type 3 is originated and no inter-area route is computed
2. **Phase: Type 3 / Type 4 origination** — Summary-LSA origination into each attached area
   - Tests: `TestOSPFType3Origination`, `TestOSPFType4Origination`, `TestOSPFSummaryLSIDCollision`, `TestOSPFSummaryFlood`
   - Files: `spf/summary.go` (read the ospf-8 §16.1 route table; build Type 3/4; hand to the ospf-7 LSDB store; register `ze_ospf_summary_lsas{area}`)
   - Verify: per intra-area network → Type 3 into every other area with the right LS ID / mask / metric; per ASBR → Type 4; LS-ID collision disambiguated; the LSA enters §13 flooding
3. **Phase: Area ranges** — aggregation + suppression
   - Tests: `TestOSPFAreaRangeAggregate`, `TestOSPFAreaRangeNotAdvertise`
   - Files: `spf/summary.go` (fold components into a configured `advertise` range with metric = configured cost else max component; drop `not-advertise`-covered components)
   - Verify: one aggregate Type 3 per `advertise` range with the §12.4.3 metric; `not-advertise` suppresses; uncovered components still individual
4. **Phase: Inter-area route computation (§16.2/§16.3)** — consume Type 3/4, append inter-area candidates
   - Tests: `TestOSPFInterAreaRoute`, `TestOSPFInterAreaPreference`, `TestOSPFABRBackboneOnlyAcceptance`, `TestOSPFType4RouteToASBR`
   - Files: `spf/interarea.go` (cost = cost-to-ABR + summary metric; next-hop toward the ABR; trap #8 backbone-only at an ABR; Type 4 → route-to-ASBR)
   - Verify: NON-ABR computes IA routes from attached-area summaries; ABR uses ONLY area-0 summaries (two-ABR non-backbone topology does not loop); intra beats inter for the same prefix; one `locrib.Path` per prefix
5. **Phase: Withdrawal + border-routers** — MaxAge re-origination + the show snapshot
   - Tests: `TestOSPFSummaryWithdraw`, `TestOSPFBorderRouterSnapshot`, `test/ospf/ospf-inter-area.ci`
   - Files: `spf/summary.go` (MaxAge re-origination on loss), `spf/interarea.go` (`show ip ospf border-routers` snapshot)
   - Verify: a vanished network/ASBR re-originates at MaxAge and the inter-area route withdraws from the kernel; the snapshot lists ABRs/ASBRs with area/cost/next-hop
6. **Functional tests** — finalise `test/ospf/ospf-inter-area.ci` (add, range, withdraw, border-routers)
7. **RFC refs** — add `// RFC 2328 Section 16.2 / 16.3 / 12.4.3 / 3.3 / 3.5 ...` comments above the enforcing code
8. **Full verification** — `make ze-verify`
9. **Complete spec** — fill audit tables, write learned summary to `plan/learned/NNN-ospf-9-inter-area-abr.md`, two commits

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Feature completeness | Every End-to-End User Story has a working path (origination, inter-area compute, range, withdraw, border-routers); ABR/inter-area parity with the FRR `ospf_abr.c` / `ospf_ia.c` split |
| Correctness | §16.2 cost = cost-to-ABR + summary metric; trap #8 backbone-only acceptance at an ABR; §12.4.3 range metric = configured or max component; withdraw via MaxAge re-origination; LS-ID collision handling |
| Naming | Type 3 LS ID = network address, Type 4 LS ID = ASBR Router ID; metrics `ze_ospf_abr`, `ze_ospf_summary_lsas{area}`; CLI `show ip ospf border-routers` |
| Data flow | inter-area routes append to the ospf-8 route table → single `locrib.Path` publish → sysrib → fibkernel; origination → ospf-7 LSDB store → §13 flooding; no second FIB/flooding path; no redistevents |
| CLI grammar | `show ip ospf border-routers`: action before identifier per `ai/rules/cli-grammar.md` |
| Prometheus counters | `ze_ospf_abr` and `ze_ospf_summary_lsas{area}` defined, registered here (per-owner), and only these two |
| Rule: plugin-self-containment | All inter-area/summary code under `internal/component/ospf/spf/`; no OSPF spelling in generic packages |
| Rule: memory-architecture | Type 3/4 origination buffer-first into the LSDB store; LSDB read via lazy accessors; no cross-boundary pointers in the inserted `locrib.Path` |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Inter-area + summary package files | `ls internal/component/ospf/spf/interarea.go internal/component/ospf/spf/summary.go` |
| Type 3/4 origination into the LSDB store | `grep -rn 'LSType.*3\|LSType.*4\|Summary' internal/component/ospf/spf/summary.go` |
| Trap #8 backbone-only acceptance | `grep -rn 'backbone\|0\.0\.0\.0\|AreaBackbone' internal/component/ospf/spf/interarea.go` |
| Metrics owned here | `grep -rn 'ze_ospf_abr\|ze_ospf_summary_lsas' internal/component/ospf/spf/` |
| Functional test | `ls test/ospf/ospf-inter-area.ci` |
| Inter-area route reaches the kernel | functional test asserts an inter-area prefix in the kernel FIB as OSPF (`RTPROT_ZE`) via the ABR |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Summary-LSA metric is a 3-byte field (0..16777215); the configured range cost is bounds-checked against it; inter-area total cost clamps at LSInfinity (0xFFFFFF = unreachable, no wrap); the advertised Network Mask is validated before forming a prefix |
| Resource exhaustion | Origination is bounded by the route-table size and the area count; one Type 3 per network per other-area, deduplicated; range aggregation reduces, never multiplies, the LSA count |
| Loop prevention | Trap #8 (§16.3) backbone-only acceptance at an ABR is mandatory; without it a two-ABR non-backbone topology loops |
| Error leakage | A malformed received Summary-LSA excludes one route, not the whole inter-area run; failures logged, not panicked |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read RFC 2328 §16.2/§16.3/§12.4.3 / Current Behavior |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
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
<!-- LIVE — write IMMEDIATELY when you learn something -->

## Core Insight
<!-- Optional: the single most important design revelation from this work. -->
Inter-area routing is mostly bookkeeping on top of intra-area SPF: the ABR's own
§16.1 route table is BOTH the source of Type 3/4 origination (the costs it
advertises) AND the input to §16.2 computation (cost-to-ABR), so the inter-area
spec is a producer/consumer pair over a single existing artifact, not a new graph
algorithm. The only genuinely subtle rule is trap #8 (an ABR trusts only backbone
summaries), which exists purely for loop-freedom.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Append inter-area candidates to the ospf-8 route table; let its candidate compare resolve intra > inter; reuse the single `locrib.Path` install seam | A separate inter-area install path; a `locrib.Path` path-type field | The umbrella "Route preference / path types" resolves preference INSIDE OSPF SPF and publishes one Path per prefix with AdminDistance 110; a second install path or a path-type field is out of v1 scope and would duplicate the seam |
| Hand Type 3/4 LSAs to the ospf-7 LSDB store and rely on §13 flooding + the MaxAge walker | A Summary-LSA-specific origination/flood path | Summary-LSAs are just more self-originated LSAs; reusing the ospf-7 store keeps a single flooding path and a single MaxAge withdrawal mechanism |
| Trap #8: at an ABR, use ONLY area-0 summaries for route computation (store non-backbone ones for re-flooding) | Use all attached-area summaries | RFC 2328 §16.3 mandates it; without it a two-ABR non-backbone area loops (guide trap #8) |
| Area-range aggregate metric = configured range cost if set, else max component cost | min or sum of component costs | RFC 2328 §12.4.3: the aggregate cost is the largest component cost (unless the operator sets a range cost) |
| Withdraw a summary by re-originating at LS Age MaxAge | Delete the LSA from the local store directly | RFC 2328 §14: MaxAge re-origination drives the §13 flush so neighbours purge consistently; a local delete would not propagate |
| No virtual links in v1; require a real backbone interface | Implement RFC 2328 §15 virtual links now | Umbrella "Out of scope": virtual links are a backbone-repair feature added once the backbone is solid; documented as a limitation here |

## Known Limitations
- Normal areas only here; stub-area summary filtering (`no-summary` totally-stubby) and NSSA Type 7 origination / Type 7→5 translation are owned by ospf-11 (this spec leaves the stub/NSSA filter hooks for it).
- No virtual links (RFC 2328 §15): an ABR must have a real backbone interface; a detached/partitioned backbone is not repaired in v1.
- Inter-area routes carry a single AdminDistance (110) on `locrib.Path` like all OSPF routes; the intra > inter > external preference is resolved inside OSPF SPF (the umbrella contract), so per-path-type admin distance vs other protocols is future work needing a `locrib.Path` path-type field.
- FRR `ospfd` multi-area interop (Type 3/4 exchange, ranges, two-ABR loop-freedom, convergence) is owned by ospf-13; this spec proves the behaviour Ze-to-Ze.
- TOS-based summaries are not produced (single TOS 0, consistent with the umbrella codec).

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: the ABR predicate (§3.3), the Summary-LSA origination rule (§12.4.3,
incl. the aggregate-metric and LS-ID-collision rules), §16.2 inter-area cost
composition, and the §16.3 backbone-only acceptance rule (trap #8). Specifically:
`// RFC 2328 Section 3.3` above the ABR predicate, `// RFC 2328 Section 12.4.3`
above Type 3/4 origination + range aggregation, `// RFC 2328 Section 16.2` above
the inter-area cost composition, and `// RFC 2328 Section 16.3` above the
ABR-backbone-only acceptance check.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered — add test for each]

### Documentation Updates
- [Docs updated, with source anchors named, or "None" with grep evidence]

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
| ABR detection + B-bit + `ze_ospf_abr` | unit test | `TestOSPFABRDetection`, `TestOSPFABRBitSet` |
| Type 3 / Type 4 origination into each attached area | unit test | `TestOSPFType3Origination`, `TestOSPFType4Origination` |
| Inter-area route computation (§16.2, cost-to-ABR + metric) | unit + functional test | `TestOSPFInterAreaRoute`, `test/ospf/ospf-inter-area.ci` |
| Trap #8 backbone-only acceptance at an ABR (loop-freedom) | unit test | `TestOSPFABRBackboneOnlyAcceptance` |
| Area ranges (aggregate / not-advertise) | unit + functional test | `TestOSPFAreaRangeAggregate`, `TestOSPFAreaRangeNotAdvertise`, `test/ospf/ospf-inter-area.ci` (range step) |
| Summary withdrawal (MaxAge) | unit + functional test | `TestOSPFSummaryWithdraw`, `test/ospf/ospf-inter-area.ci` (withdraw step) |
| `show ip ospf border-routers` | functional test | `test/ospf/ospf-inter-area.ci` (border-routers step) |

## Review Gate

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
- [ ] AC-1..AC-N all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/component/ospf/spf/interarea.go`, `summary.go`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component (interarea vs summary)
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification — owned by ospf-13)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-ospf-9-inter-area-abr.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospf-9-inter-area-abr.md` only (preserves edited spec in git history from commit A)

## Related Specs
- `plan/spec-ospf-8-spf-rib.md` - provides the intra-area §16.1 route table and the `locrib.Path` install seam this spec extends (dependency)
- `plan/spec-ospf-7-lsdb-flooding.md` - provides the LSDB store, §13 flooding, and MaxAge walker that distribute/withdraw the Type 3/4 summaries
- `plan/spec-ospf-2-wire.md` - the Summary-LSA (Type 3/4) wire codec this spec originates and consumes
- `plan/spec-ospf-10-as-external-asbr.md` - consumes the Type 4 route-to-ASBR for §16.4 external computation
- `plan/spec-ospf-11-stub-nssa.md` - stub-area summary filtering (`no-summary`) and NSSA Type 7 build on this spec's ABR origination
- `plan/spec-ospf-13-cli-diag-interop.md` - renders `show ip ospf border-routers`, scrape-asserts `ze_ospf_abr` / `ze_ospf_summary_lsas`, and owns the FRR multi-area interop scenario
