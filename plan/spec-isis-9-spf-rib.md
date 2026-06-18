# Spec: isis-9-spf-rib

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-isis-7-flooding.md, spec-isis-8-dis-broadcast.md |
| Phase | - |
| Updated | 2026-06-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-isis-0-umbrella.md` - umbrella `## Shared Contracts` "Route install vs redistribution" (admin distance: single `AdminDistance` 115 on `locrib.Path`, L1-over-L2 resolved inside SPF; ECMP committed) + "TLV 135 / 236 entry layout" (up/down bit in the control octet) + "Next-hop derivation for SPF", row isis-9, assumptions A-2/A-3 (authoritative)
4. `docs/research/isis-implementation-guide.md` section 5 "SPF" (~408-451) and section 12 item 9 "Route Leaking L1/L2 up/down bit" (~902-907)
5. `internal/core/rib/locrib/candidate.go` + `entry.go` (~74-96) - `Path{Source, Instance, NextHop, AdminDistance, Metric}` (NO protoType/level field) and best-path selection (the install target)
6. `internal/component/bgp/plugins/rib/rib_bestchange.go` (~813) - `r.locRIB.InsertForward(fam, pfx, locrib.Path{...})`, the install pattern to mirror
7. `internal/plugins/sysrib/sysrib.go` (~255-325 ingest, ~778-820 `OnChange`/snapshot, ~997 `replayPath`, ~225 `effectivePriority`, ~1017-1029 `bgpProtocolTypeFromPath` returns Unspecified for non-BGP) - sysrib consumes the Loc-RIB, keys `s.routes[key]` by protocol STRING
8. `internal/plugins/sysrib/yang/ze-rib-conf.yang` - admin-distance config; the existing single `isis` leaf (default 115) is used; no per-level admin-distance leaves are added

## Task

Add the Shortest Path First (SPF) computation and the system-RIB route
installation that together make a Ze node program IS-IS-learned routes into the
kernel FIB. This is the core-goal spec of the IS-IS set: it delivers the
umbrella goal "sys-rib updated from IS-IS" (umbrella Goal Validation row).

Build a directed graph per level from the synced LSDB (spec-isis-7), with System
IDs and pseudo-nodes (spec-isis-8) as vertices and Extended IS Reachability
(TLV 22) adjacencies as wide-metric-weighted edges; pseudo-node edges carry
metric 0. The IS-reachability metric (TLV 22) is 24-bit (0..16777215); the
IP/IPv6 prefix metric (TLV 135/236) is 32-bit (0..4294967295), so SPF reads the
full 32-bit prefix metric and does not cap it at 24-bit. The up/down bit lives in
the TLV 135 control octet (TLV 236 flags octet) per the umbrella canonical
"TLV 135 / 236 entry layout", not in the metric value; SPF reads the control
octet for the up/down bit and the 32-bit field as the pure metric. Accumulate
path cost in a wide-enough accumulator (a sum of 32-bit prefix metrics plus
24-bit edge metrics can exceed 32 bits); apply the RFC 5305 MAX_PATH_METRIC
(0xFE000000) handling, or accumulate in 64 bits and clamp, so overflow never
wraps. Run Dijkstra rooted at self for L1 and for L2, producing for each
reachable IP prefix a metric, one or more equal-cost next-hops (ECMP), and the
outgoing interface. Honour the overload bit (a node with overload set is usable
as a destination but not as transit). Perform L1<->L2 route leaking with the
RFC 2966 up/down bit so leaked routes never loop back up, and apply the
RFC 5308 sec 5 up/down-aware path-preference order when the same prefix is
reachable at more than one level/up-down state on an L1L2 router (best to
worst: L1 up, L2 up, L2 down, L1 down; ties broken by metric). The same order
governs IPv4 (RFC 2966 / RFC 5302) and IPv6 (RFC 5308); note an L1 DOWN prefix
is LESS preferred than an L2 prefix, so a plain "L1 always beats L2" rule is
wrong. Trigger SPF on LSDB change through a short debounce (a few hundred ms)
to avoid thrashing.

Install the SPF result by INSERTING routes into the unified Loc-RIB, exactly as
BGP does (`internal/component/bgp/plugins/rib/rib_bestchange.go:813`
`r.locRIB.InsertForward(fam, pfx, locrib.Path{...})`). Per umbrella Shared
Contracts "Route install vs redistribution", this is NOT `redistevents`:
`redistevents` feeds the redistribute-orchestrator (redistribution to other
protocols) and is owned by spec-isis-11, never the FIB-install path. For each
SPF prefix, SPF builds a `locrib.Path{Source = IS-IS ProtocolID, Instance,
NextHop, AdminDistance, Metric}` and calls `InsertForward` on add/change and the
matching forward-remove on loss. sysrib then consumes the Loc-RIB via
`loc.OnChange` (`internal/plugins/sysrib/sysrib.go:778-820`), applies admin
distance through `effectivePriority`, recomputes the system best, and fibkernel
programs the kernel route marked `RTPROT_ZE`.

`locrib.Path` has NO protoType/level field (confirmed in
`internal/core/rib/locrib/candidate.go`) and `bgpProtocolTypeFromPath` returns
Unspecified for non-BGP paths (`sysrib.go:1017-1029`), so per-level admin distance
is NOT implementable in v1. IS-IS sets a SINGLE `AdminDistance` (115) on the
`locrib.Path`, looked up by sysrib `effectivePriority` from the EXISTING
`rib.admin-distance.isis` leaf in `internal/plugins/sysrib/yang/ze-rib-conf.yang`.
The `ze-rib-conf.yang` already has the `isis` leaf, so this spec adds NO per-level
admin-distance leaves. The L1-over-L2 (intra-area over
inter-area) preference is resolved INSIDE IS-IS SPF, before publishing exactly one
`locrib.Path` per prefix. Per-level admin distance vs other protocols is future
work that would require adding a protoType/level field to `locrib.Path` (see the
Assumptions table).

For ECMP (committed, in umbrella scope), emit one `locrib.Path` per equal-cost
next-hop with a distinct `Instance`. Because sysrib keys `s.routes[key]` by
protocol string (`sysrib.go:286-298`) and the Loc-RIB snapshot/`replayPath` path
(`sysrib.go:799`, `sysrib.go:997`) plus the `OnChange` `changeToBatch` path carry
only the single best Path today, multiple IS-IS Paths for one prefix would collapse
to one next-hop. This spec therefore MUST extend sysrib/locrib to expand a Loc-RIB
path-group into `BestChangeEntry.ECMPPaths` so the equal-cost next-hops survive to
the kernel as a multipath route. This is a committed deliverable (Files to Modify
lists `internal/plugins/sysrib/sysrib.go` and `internal/core/rib/locrib/`), not an
optional limitation; see Key Design Decisions and A-2. Expose a `show isis route`
snapshot API (rendered in spec-isis-13).

Package: `internal/component/isis/spf/`, which both computes SPF and inserts the
resulting `locrib.Path` values into the Loc-RIB.

## Required Reading

### Architecture Docs
- [ ] `docs/research/isis-implementation-guide.md` section 5 "SPF" (~408-451) - graph build, Dijkstra, ECMP, triggering, output
  -> Decision: graph vertices are System IDs and pseudo-node IDs; edges from Extended IS Reachability TLV 22, pseudo-node edges metric 0
  -> Decision: SPF is debounced and event-driven on LSDB change, with a periodic safety re-run
  -> Constraint: SPF output is prefix -> {metric, next-hop(s), interface}; ECMP is multiple equal-cost next-hops
- [ ] `docs/research/isis-implementation-guide.md` section 12 item 9 "Route Leaking" (~902-907) - RFC 2966 up/down bit
  -> Constraint: routes learned from L2 and re-advertised into L1 MUST set the up/down bit; an L1 route carrying the up/down bit set MUST NOT be re-leaked up into L2 (loop prevention)
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` (~813) - the install pattern to mirror
  -> Constraint: BGP mirrors its best path into the shared Loc-RIB via `r.locRIB.InsertForward(fam, pfx, locrib.Path{Source, Instance, NextHop, AdminDistance, Metric}, forward)`; IS-IS install copies this exactly, NOT a redistevents emit
- [ ] `internal/core/rib/locrib/entry.go` + `candidate.go` - cross-protocol best path (the install target)
  -> Constraint: Loc-RIB selects lower AdminDistance first, then lower Metric, then first-seen (stable); IS-IS supplies `Source`/`Instance`/`NextHop`/`AdminDistance`/`Metric`; value-typed, no cross-boundary pointers
  -> Constraint: a `PathGroup` keeps one best Path; multiple ECMP Paths for the same prefix do not survive to sysrib without a path-group enhancement (see A-2)
- [ ] `internal/plugins/sysrib/sysrib.go` (~255-325, ~778-820, ~997, ~225, ~1017-1029) - sysrib consumes the Loc-RIB and recomputes best
  -> Constraint: sysrib subscribes via `loc.OnChange` and snapshots via `loc.Iterate` -> `replayPath`; it keys `s.routes[key]` by protocol STRING and admin distance comes from `effectivePriority(protoType, ...)` -> `rib.admin-distance.*`; deterministic tiebreak by protocol name
  -> Constraint: `bgpProtocolTypeFromPath` (`sysrib.go:1017-1029`) returns Unspecified for non-BGP paths, so there is no per-level protoType to key on; IS-IS supplies a single `AdminDistance` (115) on `locrib.Path`
  -> Constraint: `redistevents` is NOT used on this path; it belongs to redistribution (isis-11)
- [ ] `internal/plugins/sysrib/yang/ze-rib-conf.yang` - admin distance per protocol
  -> Constraint: the existing single `isis` leaf (default 115) is used as-is; this spec adds NO per-level admin-distance leaves (`locrib.Path` has no protoType/level field, so per-level distance is not implementable in v1)

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc2966.md` - Domain-wide prefix distribution, up/down bit (CREATED per umbrella)
  -> Constraint: the up/down bit lives in the TLV 135 CONTROL octet (RFC 5305 sec 4.1), not in the metric value; it is set when a prefix is leaked from L2 down into L1, and a prefix with the bit set is not re-advertised up into L2 (TLV 236 carries the U bit in its flags octet)
- [ ] `rfc/short/rfc5305.md` - wide metrics, Extended IS Reachability TLV 22, Extended IP Reachability TLV 135 (CREATED per umbrella)
  -> Constraint: IS-reachability wide metric is 24-bit (TLV 22, range 0..16777215); the IP/IPv6 prefix metric (TLV 135/236) is a 32-bit field (0..4294967295) read in full, with the up/down bit in a separate control/flags octet (umbrella "TLV 135 / 236 entry layout"); accumulate path cost in a wide-enough accumulator and apply MAX_PATH_METRIC 0xFE000000 handling so sums never wrap
- [ ] `rfc/short/rfc3787.md` - overload bit (CREATED per umbrella)
  -> Constraint: a node advertising the overload bit is reachable as a destination but MUST NOT be used as a transit (path-through) node in SPF

**Key insights:**
- Route install is solved infra, but the path is Loc-RIB INSERTION, not redistevents: SPF builds `locrib.Path` and calls `InsertForward` (mirror BGP `rib_bestchange.go:813`); sysrib consumes `loc.OnChange` and fibkernel programs `RTPROT_ZE`. `redistevents` is the separate redistribution path (isis-11) and must not be used here.
- Admin distance for IS-IS is a single 115 on `locrib.Path.AdminDistance`, looked up by sysrib `effectivePriority` from the EXISTING `rib.admin-distance.isis` leaf; `locrib.Path` has no protoType/level field, so per-level admin distance is not implementable in v1 and L1-over-L2 is resolved inside IS-IS SPF before publishing one Path per prefix.
- The up/down bit (RFC 2966) is the loop-prevention mechanism for L1<->L2 leaking and lives in the TLV 135 control octet (TLV 236 flags octet), not in the metric. Path preference among levels follows the RFC 5308 sec 5 up/down-aware order (L1 up > L2 up > L2 down > L1 down, then metric), NOT a flat "L1 over L2"; an L1-down prefix is the least preferred.
- Metric widths differ: TLV 22 IS-reachability metric is 24-bit; TLV 135/236 prefix metric is 32-bit (read in full); accumulate in a wide-enough accumulator with MAX_PATH_METRIC 0xFE000000 handling.
- Next-hop derivation: SPF resolves the first hop toward the originating neighbour and uses that neighbour's advertised address from TLV 132 (IPv4) / TLV 232 (IPv6), per Shared Contracts "Next-hop derivation for SPF".
- ECMP (committed, umbrella scope) is one `locrib.Path` per equal-cost next-hop (distinct `Instance`); sysrib currently collapses intra-protocol ECMP to one next-hop, so this spec MUST expand a Loc-RIB path-group into `BestChangeEntry.ECMPPaths` (umbrella A-2), a committed deliverable.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
- [ ] `internal/core/rib/locrib/candidate.go` + `entry.go` (~74-96) - `Path{Source, Instance, NextHop, AdminDistance, Metric}` is the value-typed install unit with NO protoType/level field; `PathGroup` keeps one best Path (`selectBest`: lower AdminDistance, then lower Metric, then first-seen)
  -> Constraint: IS-IS INSTALLS by building `locrib.Path` and inserting it; with no protoType field, per-level admin distance is not possible, so IS-IS sets a single `AdminDistance` (115) and resolves L1-over-L2 inside SPF; see umbrella A-3
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` (~813) - the install pattern: `r.locRIB.InsertForward(fam, pfx, locrib.Path{Source: bgpProtocolID, Instance: pathID, NextHop, AdminDistance, Metric}, forward)`
  -> Constraint: copy this exact insertion shape; `Source` = IS-IS ProtocolID, `Instance` distinguishes ECMP next-hop; NO redistevents emit on the install path
- [ ] `internal/plugins/sysrib/sysrib.go` (~255-325 ingest, ~778-820 `OnChange`/snapshot, ~997 `replayPath`, ~225 `effectivePriority`, ~1017-1029 `bgpProtocolTypeFromPath`) - sysrib consumes the Loc-RIB
  -> Constraint: sysrib subscribes via `loc.OnChange`, snapshots via `loc.Iterate` -> `replayPath`, keys `s.routes[key]` by protocol STRING, and looks up admin distance via `effectivePriority(c.ProtocolType.String(), ...)`; `bgpProtocolTypeFromPath` (`sysrib.go:1017-1029`) returns Unspecified for non-BGP, so there is no per-level key; intra-protocol ECMP collapses because only the single best Path is replayed/changed (`sysrib.go:286-298,799,997`)
- [ ] `internal/plugins/sysrib/yang/ze-rib-conf.yang` - `rib.admin-distance` container; has a single `isis` leaf (default 115) which this spec uses as-is; NO per-level leaves are added (no protoType field on `locrib.Path`)
- [ ] `internal/plugins/fib/kernel/backend_linux.go` (~26) + `internal/core/rtproto/rtproto.go` - kernel routes are tagged `RTPROT_ZE` (`rtproto.FIBKernel`); the FIB programming path is protocol-agnostic

**Behavior to preserve:**
- The protocol-agnostic FIB-install path (Loc-RIB insertion -> `sysrib` `OnChange` -> `fibkernel`) is unchanged; IS-IS plugs in as a Loc-RIB source like BGP.
- Loc-RIB best-path semantics (AdminDistance then Metric then first-seen) and sysrib `recomputeBest`/`effectivePriority` semantics are unchanged.
- Existing Loc-RIB sources (static, connected, L2TP, BGP) keep working unchanged.
- The existing `rib.admin-distance.isis` leaf is used as-is; no per-level admin-distance leaves are added.
- The Loc-RIB path-group ECMP expansion into `BestChangeEntry.ECMPPaths` is additive: single-Path protocols (static, connected, BGP best) are unaffected; only multi-Path prefixes gain extra next-hops.
- `redistevents` and the redistribute-orchestrator are not touched here; redistribution to BGP is spec-isis-11.

**Behavior to change:**
- New `internal/component/isis/spf/` package that computes SPF and INSERTS the resulting `locrib.Path` values into the Loc-RIB.
- IS-IS becomes a Loc-RIB source identified by its registered ProtocolID; it sets a single `AdminDistance` (115) on each Path (no per-level protoType, which `locrib.Path` does not model).
- sysrib admin-distance is unchanged: the existing `isis` leaf (default 115) is used; no new leaves.
- sysrib/locrib gains a path-group expansion into `BestChangeEntry.ECMPPaths` so intra-protocol equal-cost next-hops survive to the kernel (committed; see A-2).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- LSDB change notification from spec-isis-7 (LSP added/updated/purged, adjacency up/down) on the relevant level (L1 and/or L2).
- A periodic safety timer also enqueues an SPF run even with no change.

### Transformation Path
1. **Debounce:** an LSDB change marks the level dirty and arms a short timer (a few hundred ms); coalesces a burst of LSP arrivals into one SPF run per level.
2. **Graph build:** read the synced LSDB for the level; vertices are System IDs and pseudo-node IDs; edges come from Extended IS Reachability TLV 22 (wide metric), pseudo-node edges metric 0; mark vertices that advertise the overload bit as transit-excluded.
3. **Dijkstra:** rooted at self; relax edges; ECMP keeps every equal-cost predecessor so multiple next-hops survive; resolve next-hop to the first hop toward the originating neighbour, using that neighbour's advertised address from TLV 132 (IPv4) / TLV 232 (IPv6) and the outgoing circuit, per Shared Contracts "Next-hop derivation for SPF".
4. **Prefix attach:** attach Extended IP Reachability (TLV 135) prefixes from each reached node, choosing the minimum total metric and the ECMP next-hop set.
5. **L1<->L2 leak:** on an L1L2 router, leak L2-derived prefixes into L1 with the RFC 2966 up/down bit set; do not re-leak prefixes that carry the up/down bit upward into L2. When the same prefix is reachable at more than one level/up-down state, select per the RFC 5308 sec 5 order L1-up > L2-up > L2-down > L1-down (then metric); this is NOT a flat "L1 over L2" rule -- an L1-down (leaked) prefix is the least preferred.
6. **Diff:** compare the new per-prefix result set against the previously installed set; produce add / change / remove deltas.
7. **Install (Loc-RIB insertion):** for each added/changed prefix, build `locrib.Path{Source = IS-IS ProtocolID, Instance (ECMP next-hop discriminator), NextHop, AdminDistance (single 115), Metric}` and call `locRIB.InsertForward(fam, pfx, path, forward)` (one Path per equal-cost next-hop, distinct `Instance`); for lost prefixes call the matching forward-remove. This mirrors BGP `rib_bestchange.go:813`. No `redistevents` emit (that path is redistribution, isis-11). L1-over-L2 preference is resolved inside SPF before this step, so exactly one prefix result set is published.
8. **Arbitrate + program:** Loc-RIB best-path (single admin distance 115, then metric, then first-seen) -> sysrib consumes `loc.OnChange` / snapshot `replayPath` -> `recomputeBest` -> fibkernel netlink -> kernel route tagged `RTPROT_ZE`. Intra-protocol ECMP survives via the committed Loc-RIB path-group expansion into `BestChangeEntry.ECMPPaths` (A-2), so the kernel receives a multipath route.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| LSDB <-> SPF | in-process change notification + LSDB read (same component) | [ ] |
| SPF/engine <-> Loc-RIB | `locrib.Path{Source, Instance, NextHop, AdminDistance, Metric}` via `InsertForward` (value-typed) | [ ] |
| Loc-RIB <-> sys-rib | `loc.OnChange` + snapshot `replayPath`; best selection by (AdminDistance, Metric); path-group expansion into `BestChangeEntry.ECMPPaths` for equal-cost next-hops | [ ] |
| sys-rib <-> kernel | existing best-change -> fibkernel netlink (`RTPROT_ZE`) | [ ] |

### Integration Points
- New package `internal/component/isis/spf/` (graph, Dijkstra, route output, diff, Loc-RIB insertion).
- Loc-RIB insertion via `InsertForward` (mirror BGP `rib_bestchange.go:813`); IS-IS is a new Loc-RIB source, not a redistevents producer.
- The existing `rib.admin-distance.isis` leaf in `internal/plugins/sysrib/yang/ze-rib-conf.yang` (no schema change here).
- Committed sysrib/locrib path-group ECMP expansion (`internal/plugins/sysrib/sysrib.go`, `internal/core/rib/locrib/`) into `BestChangeEntry.ECMPPaths` (A-2).
- `show isis route` snapshot API (rendered in spec-isis-13).
- Prometheus SPF counters (`ze_isis_spf_*`, `ze_isis_routes_installed`) are owned and registered HERE per the umbrella canonical table; isis-13 only scrapes/surfaces them.

### Architectural Verification
- [ ] No bypassed layers (LSDB -> SPF -> Loc-RIB insertion -> sysrib `OnChange` -> fibkernel -> kernel)
- [ ] No unintended coupling (SPF reads the LSDB; no second FIB path; no direct netlink from IS-IS; no redistevents on the install path)
- [ ] No duplicated functionality (route install reuses Loc-RIB + sysrib; arbitration reuses Loc-RIB best-path)
- [ ] Value-typed boundary preserved (`locrib.Path` fields are value types; no cross-boundary pointers)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-2 | FIB install is via Loc-RIB insertion (`locrib.Path` + `InsertForward`), not redistevents (redistevents feeds the redistribute-orchestrator, isis-11); ECMP needs one Path per next-hop plus a committed sysrib/locrib path-group expansion into `BestChangeEntry.ECMPPaths` because sysrib keys `s.routes[key]` by protocol string and only the single best Path is replayed (`sysrib.go:286-298,799,997`) | `internal/component/bgp/plugins/rib/rib_bestchange.go:813`, `internal/plugins/sysrib/sysrib.go:255-325,778-820`; mirrors umbrella A-2 | The path-group expansion is mandatory; if it cannot be built, ECMP install fails and the umbrella ECMP goal is not met | `test/isis/isis-route-install.ci` end-to-end to kernel with multiple next-hops | unvalidated |
| A-3 | A single IS-IS admin distance (115) on `locrib.Path.AdminDistance` (existing `rib.admin-distance.isis` leaf) plus Loc-RIB best-path (AdminDistance then Metric) expresses cross-protocol arbitration; L1-over-L2 preference is resolved inside IS-IS SPF before publishing one Path (`locrib.Path` has no protoType/level field, `sysrib.go:1017-1029` returns Unspecified for non-BGP) | `internal/core/rib/locrib/candidate.go`, `internal/plugins/sysrib/sysrib.go:225,1017-1029`; mirrors umbrella A-3 | Per-level admin distance vs OTHER protocols (not just intra-IS-IS) is future work needing a protoType/level field on `locrib.Path` | `TestISISLeakUpDownBit` (intra-SPF L1-over-L2) + multi-source functional test | unvalidated |
| A-3b | Per-level admin distance vs other protocols (L1 and L2 at different distances against static/BGP) is NOT implementable in v1 and is deferred as future work | `internal/core/rib/locrib/candidate.go` has no protoType/level field; `sysrib.go:1017-1029` `bgpProtocolTypeFromPath` returns Unspecified for non-BGP | If a deployment needs distinct per-level distances vs other protocols, add a protoType/level field to `locrib.Path` and per-level admin-distance leaves to `ze-rib-conf.yang` | future spec when the field is added | deferred (future work) |
| A-9 | The LSDB exposes per-level change notification and a read API sufficient to build the graph without re-parsing raw bytes on the hot path more than once per SPF run | spec-isis-6/7 LSDB design | SPF must add its own parsed-topology cache | SPF unit test on a hand-built LSDB | unvalidated |
| A-10 | First-hop next-hop resolution (neighbour interface address + outgoing circuit) is derivable from the adjacency table populated in spec-isis-5/8 | spec-isis-5 neighbour table, spec-isis-8 pseudo-node | Need extra TLV 132/232 interface-address lookup | `TestISISSPFRoute` next-hop assertion | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | L1<->L2 leaking without the up/down bit creates a routing loop | loop / churn in a mixed L1L2 topology | Enforce RFC 2966 up/down bit in step 5; `TestISISLeakUpDownBit`; interop test in spec-isis-13 |
| R-2 | Overload bit ignored, transit traffic blackholed through an overloaded node | traffic via an overloaded node in tests | Exclude overloaded nodes from transit in graph build; `TestISISOverloadTransit` |
| R-3 | SPF thrash on a flapping link starves the engine | high SPF run rate counter | Debounce timer (a few hundred ms) coalesces bursts; `TestISISSPFDebounce` |
| R-4 | Stale routes left in the kernel after neighbour loss | prefix still in FIB after hold-down | Diff against previously installed set and forward-remove the lost `locrib.Path`; `isis-route-install.ci` withdraw step |
| R-5 | Intra-protocol ECMP collapses to one next-hop because sysrib keys by protocol string and only the best Path is replayed (`sysrib.go:286-298,799,997`) | single next-hop in kernel when two expected | Expand a Loc-RIB path-group into `BestChangeEntry.ECMPPaths` (committed sysrib/locrib change in Files to Modify); validate A-2 end-to-end with a multi-next-hop kernel route |
| R-6 | The Loc-RIB path-group expansion into `BestChangeEntry.ECMPPaths` regresses existing single-Path sources (static, connected, BGP best) by changing the snapshot/`replayPath`/`changeToBatch` shape (`sysrib.go:286-298,799,997`) | existing route tests fail; duplicate or missing next-hops for non-IS-IS routes | Make the expansion additive (single-Path prefixes keep one next-hop); cover with existing static/BGP route functional tests plus the new IS-IS ECMP test before claiming done |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| LSDB change notification on a level | -> | debounce arms, SPF runs, route set produced | `TestISISSPFRoute` |
| SPF result change | -> | SPF inserts `locrib.Path` (Source=IS-IS) via `InsertForward` | `TestISISSPFRoute` (asserts the inserted Path) |
| SPF route inserted, two/three nodes | -> | Loc-RIB -> sysrib `OnChange` -> fibkernel -> kernel route (`RTPROT_ZE`, IS-IS source) | `test/isis/isis-route-install.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Hand-built LSDB with a known topology | Dijkstra produces the shortest-path metric and next-hop that matches the hand-computed result for every prefix |
| AC-2 | Two/three nodes, a remote prefix originated | The remote prefix appears in the local kernel FIB with the IS-IS-derived next-hop, tagged `RTPROT_ZE` |
| AC-3 | Two equal-cost paths to the same prefix | Both next-hops are installed (ECMP) via one `locrib.Path` per next-hop (distinct `Instance`); the Loc-RIB path-group expansion carries both into `BestChangeEntry.ECMPPaths` |
| AC-11 | Two equal-cost IS-IS paths, end to end | The kernel route for the prefix is a multipath route with both IS-IS next-hops (`RTPROT_ZE`), proving the sysrib/locrib `BestChangeEntry.ECMPPaths` expansion reaches fibkernel |
| AC-4 | A neighbour is lost (adjacency / LSP purge) | Affected prefixes are removed from the Loc-RIB (forward-remove) and withdrawn from the kernel |
| AC-5 | L1 area + L2 backbone on an L1L2 router | L2 prefixes leak into L1 with the up/down bit set; a prefix with the up/down bit set is not re-leaked up into L2; no loop forms |
| AC-6 | A node advertises the overload bit | The node is reachable as a destination but is not used as a transit node in SPF |
| AC-7 | Same prefix reachable at multiple levels/up-down states | Path selection follows the RFC 5308 sec 5 order L1-up > L2-up > L2-down > L1-down (then metric): an L1-up prefix beats an L2-up prefix, AND an L1-down (leaked) prefix is LESS preferred than an L2 prefix (proving the order is up/down-aware, not flat "L1 over L2") |
| AC-8 | Same prefix also present from static / BGP with a lower admin distance | The lower-admin-distance source wins in sysrib; IS-IS (115) loses; raising the other source above 115 makes IS-IS win |
| AC-9 | A burst of LSP arrivals within the debounce window | SPF runs once per level for the burst, not once per LSP |
| AC-10 | `show isis route` invoked | The snapshot lists per-level prefixes with metric, next-hop(s), and interface |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Expects remote prefixes in the kernel FIB | LSDB change -> debounce -> SPF -> `locrib.Path` `InsertForward` -> Loc-RIB -> sysrib `OnChange` -> fibkernel -> kernel (`RTPROT_ZE`) | `test/isis/isis-route-install.ci` |
| 2 | Expects ECMP installed for equal-cost paths | SPF keeps both next-hops -> one `locrib.Path` per next-hop (distinct `Instance`) -> Loc-RIB path-group expansion into `BestChangeEntry.ECMPPaths` -> sysrib ECMP collect -> kernel multipath | `test/isis/isis-route-install.ci` (ECMP step) |
| 3 | Expects routes withdrawn when a neighbour dies | adjacency/LSP loss -> SPF diff -> forward-remove of the lost `locrib.Path` -> sysrib withdraw -> kernel route removed | `test/isis/isis-route-install.ci` (withdraw step) |
| 4 | Runs `show isis route` | CLI -> RPC -> SPF snapshot (rendering in spec-isis-13) | `test/isis/isis-show.ci` (spec-isis-13) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestISISGraphBuild` | `internal/component/isis/spf/graph_test.go` | vertices from System IDs + pseudo-nodes; TLV 22 edges; pseudo-node edges metric 0 | |
| `TestISISSPFShortestPath` | `internal/component/isis/spf/spf_test.go` | Dijkstra on a hand-built LSDB matches hand-computed metric/next-hop | |
| `TestISISSPFECMP` | `internal/component/isis/spf/spf_test.go` | two equal-cost paths yield two next-hops | |
| `TestISISOverloadTransit` | `internal/component/isis/spf/spf_test.go` | overloaded node reachable as destination, excluded as transit | |
| `TestISISLeakUpDownBit` | `internal/component/isis/spf/route_test.go` | L2->L1 leak sets the up/down bit in the TLV 135 control octet (not the metric); up/down-set prefix not re-leaked up; multi-level selection follows the RFC 5308 sec 5 order (L1-up > L2-up > L2-down > L1-down), so an L1-down prefix loses to an L2 prefix (not flat "L1 over L2") | |
| `TestISISMetricWidth` | `internal/component/isis/spf/spf_test.go` | 32-bit TLV 135 prefix metric read in full (not capped at 24-bit); path-cost accumulation does not wrap; MAX_PATH_METRIC 0xFE000000 treated as unreachable | |
| `TestSysribECMPPathGroup` | `internal/plugins/sysrib/sysrib_test.go` | a Loc-RIB path-group with two equal-cost next-hops expands into `BestChangeEntry.ECMPPaths`; single-Path prefixes unchanged | |
| `TestISISRouteDiff` | `internal/component/isis/spf/route_test.go` | add/change/remove deltas computed correctly between runs | |
| `TestISISSPFDebounce` | `internal/component/isis/spf/spf_test.go` | a burst of LSDB changes coalesces into one SPF run per level | |
| `TestISISInstallPath` | `internal/component/isis/spf/install_test.go` | SPF result -> `locrib.Path{Source=IS-IS, Instance, NextHop, AdminDistance, Metric}` with one Path per ECMP next-hop -> `InsertForward`/forward-remove | |
| `TestISISSPFRoute` | `internal/component/isis/spf/install_test.go` | LSDB change -> SPF -> Loc-RIB insertion (wiring) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Wide IS metric (TLV 22 edge, 24-bit) | 0..16777215 | 16777215 | N/A | 16777216 |
| Prefix metric (TLV 135/236, 32-bit, up/down bit in control octet not the metric) | 0..4294967295 | 4294967295 | N/A | N/A (full 32-bit field) |
| Total path metric (wide accumulator, MAX_PATH_METRIC) | 0..0xFE000000 | 0xFE000000 | N/A | > 0xFE000000 treated as unreachable (no wrap; 64-bit accumulator) |
| Admin distance (isis, single leaf) | 0..255 | 255 | N/A | 256 |
| ECMP next-hop count per prefix | 1..N (config cap) | N | 0 (no route) | > N (truncate) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `isis-route-install` | `test/isis/isis-route-install.ci` | two/three nodes: remote prefix appears in kernel with IS-IS source; ECMP installs multiple next-hops; neighbour loss withdraws | |
| `isis-redist-arbitration` | `test/isis/isis-redist-arbitration.ci` | same prefix from IS-IS and static/BGP arbitrated by admin distance | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| (deferred to spec-isis-13) | `test/interop/scenarios/` | FRR isisd | SPF convergence and route install validated against FRR in `isis-p2p-frr` / `isis-lan-dis-frr` | |

### Future (if deferring any tests)
- FRR interop for SPF/convergence is owned by spec-isis-13 (`isis-p2p-frr`, `isis-lan-dis-frr`); this spec proves SPF + install with Ze-to-Ze functional tests. IPv6 SPF/install is spec-isis-12.

## Files to Modify
- `internal/plugins/sysrib/sysrib.go` - expand a Loc-RIB path-group into `BestChangeEntry.ECMPPaths` so intra-protocol equal-cost next-hops survive to the kernel; today sysrib keys `s.routes[key]` by protocol string (`sysrib.go:286-298`) and the snapshot/`replayPath`/`changeToBatch` paths carry only the single best Path (`sysrib.go:799,997`). Additive: single-Path prefixes unchanged
- `internal/core/rib/locrib/` - expose the equal-cost path group from the Loc-RIB so sysrib can read all next-hops for one prefix (path-group -> `BestChangeEntry.ECMPPaths`); no change to the value-typed `locrib.Path` shape
- NOTE: NO change to `internal/plugins/sysrib/yang/ze-rib-conf.yang`; the existing `isis` admin-distance leaf (default 115) is used and no per-level leaves are added (`locrib.Path` has no protoType/level field, so per-level distance is not implementable in v1)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | No | uses existing `rib.admin-distance.isis` leaf; no per-level leaves added (`locrib.Path` has no protoType field) |
| YANG validation constraints | No | existing `isis` leaf already `type uint8`, `default 115` |
| YANG custom validators | No | not needed |
| CLI commands/flags | Yes | `show isis route` snapshot RPC (rendered in spec-isis-13) |
| CLI grammar (action before identifier) | Yes | `show isis route` follows `ai/rules/cli-grammar.md` |
| Editor autocomplete | No | no new config leaves (the existing `rib.admin-distance.isis` leaf is reused; no per-level leaves added); `show isis route` completion is YANG-driven and owned by isis-13 |
| Functional test for new RPC/API | Yes | `test/isis/isis-route-install.ci`, `test/isis/isis-redist-arbitration.ci` |
| Pipe completeness | Yes | `show isis route` output through `ApplyPipes`/`ProcessPipes` (spec-isis-13) |
| Doctor check for runtime dependencies | No | install path reuses fibkernel; no new runtime dependency here |
| Prometheus counters/metrics | Yes | this spec OWNS and registers `ze_isis_spf_runs_total{level}`, `ze_isis_spf_duration_seconds{level}`, `ze_isis_spf_nodes{level}`, `ze_isis_routes_installed{level,afi}` (per the umbrella "Metrics (canonical)" table). Per-owner registration here; isis-13 only scrapes/asserts |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` (IS-IS route install row) |
| 2 | Config syntax changed? | No | existing `rib.admin-distance.isis` leaf reused; no new config |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (`show isis route`, in spec-isis-13) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (`show isis route`) |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/isis.md` (route install + leaking) |
| 7 | Wire format changed? | No | up/down bit codec belongs to spec-isis-2 |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc2966.md`, `rfc/short/rfc3787.md`, `rfc/short/rfc5305.md` |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (new `test/isis/` cases) |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` (IS-IS SPF/install row) |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` (IS-IS as a Loc-RIB source via `InsertForward`, like BGP) |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` (`ze_isis_spf_*`, `ze_isis_routes_installed` owned and registered HERE per the canonical table; surfaced in spec-isis-13) |
| 15 | Registered plugin/event/command/capability changed? | Yes | `docs/plugin-overview.md` (IS-IS registered as a Loc-RIB source ProtocolID) |
| 16 | Changed files referenced by doc source anchors? | No | grep at completion |
| 17 | Existing docs show examples for this area? | No | grep at completion |

## Files to Create
- `internal/component/isis/spf/spf.go` - Dijkstra per level, ECMP, overload handling, debounce trigger
- `internal/component/isis/spf/graph.go` - directed graph build from the LSDB (System IDs, pseudo-nodes, TLV 22 edges)
- `internal/component/isis/spf/route.go` - prefix attach, L1<->L2 leaking (RFC 2966 up/down bit), diff against installed set, `show isis route` snapshot
- `internal/component/isis/spf/install.go` - build `locrib.Path{Source = IS-IS ProtocolID, Instance (per ECMP next-hop), NextHop, AdminDistance (single 115), Metric}` per (prefix, next-hop) and call `locRIB.InsertForward` on add/change, forward-remove on loss (mirror BGP `rib_bestchange.go:813`); the IS-IS ProtocolID is registered once at startup; L1-over-L2 is already resolved in `route.go` so one prefix result set is published
- `internal/component/isis/spf/graph_test.go`, `spf_test.go`, `route_test.go`, `install_test.go` - unit tests (install_test asserts the inserted `locrib.Path` and forward-remove)
- `test/isis/isis-route-install.ci` - end-to-end route install (add, ECMP, withdraw)
- `test/isis/isis-redist-arbitration.ci` - admin-distance arbitration against static/BGP

Note: `internal/component/isis/redistribute/` (the `redistevents` producer + `RedistConsumer`) is owned by spec-isis-11, NOT this spec. This spec installs to the FIB only via Loc-RIB insertion.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8-14. | Standard flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** - register the IS-IS ProtocolID, wire the Loc-RIB insertion stub, and write the failing route-install test
   - Tests: `TestISISSPFRoute`, `test/isis/isis-route-install.ci` (fails: no SPF result yet)
   - Files: `internal/component/isis/spf/install.go` (register the IS-IS ProtocolID, hold the Loc-RIB handle, `InsertForward` entry point), an SPF entry-point stub in `spf/spf.go`
   - Verify: the IS-IS ProtocolID registers, the Loc-RIB handle is held, the wiring test fails because SPF returns no routes (no redistevents anywhere)
2. **Phase: Graph build** - vertices and edges from the LSDB
   - Tests: `TestISISGraphBuild`
   - Files: `spf/graph.go`
   - Verify: System IDs + pseudo-nodes as vertices, TLV 22 edges weighted by wide metric, pseudo-node edges metric 0, overloaded nodes flagged transit-excluded
3. **Phase: Dijkstra + ECMP + overload** - per-level shortest path
   - Tests: `TestISISSPFShortestPath`, `TestISISSPFECMP`, `TestISISOverloadTransit`
   - Files: `spf/spf.go`
   - Verify: metric/next-hop match hand-computed; equal-cost yields multiple next-hops; overloaded node excluded as transit
4. **Phase: Prefix attach + leaking + diff** - route output
   - Tests: `TestISISLeakUpDownBit`, `TestISISRouteDiff`
   - Files: `spf/route.go`
   - Verify: TLV 135 prefixes attached at min metric; L2->L1 leak sets up/down bit; up/down-set prefix not re-leaked up; multi-level selection follows the RFC 5308 sec 5 order (L1-up > L2-up > L2-down > L1-down), not flat "L1 over L2"; diff yields add/change/remove
5. **Phase: Debounce + install** - trigger and Loc-RIB insertion
   - Tests: `TestISISSPFDebounce`, `TestISISInstallPath`, `test/isis/isis-route-install.ci`
   - Files: `spf/spf.go` (debounce), `spf/install.go` (one `locrib.Path` per next-hop -> `InsertForward`; forward-remove on loss)
   - Verify: burst coalesces to one run per level; `locrib.Path` inserted with Source=IS-IS; route appears in kernel as `RTPROT_ZE`; ECMP installs multiple next-hops via the path-group expansion; neighbour loss removes
6. **Phase: Admin distance + arbitration + ECMP expansion** - cross-protocol selection and the committed intra-protocol ECMP path-group expansion
   - Tests: `test/isis/isis-redist-arbitration.ci`, `TestISISInstallPath` (ECMP), `TestSysribECMPPathGroup`
   - Files: `internal/plugins/sysrib/sysrib.go` and `internal/core/rib/locrib/` (path-group -> `BestChangeEntry.ECMPPaths`); NO change to `ze-rib-conf.yang` (existing single `isis` leaf, default 115, used as-is)
   - Verify: IS-IS at 115 (single admin distance) loses to lower-admin-distance sources and wins when raised; L1-over-L2 holds (resolved inside SPF); intra-protocol ECMP installs multiple next-hops in the kernel via `BestChangeEntry.ECMPPaths`; existing single-Path sources (static, connected, BGP) unaffected
7. **Phase: snapshot + metrics** - `show isis route` snapshot API and SPF counters
   - Files: `spf/route.go` (snapshot); the `ze_isis_spf_*`/`ze_isis_routes_installed` series are registered HERE (per-owner); isis-13 only renders/scrape-asserts them
   - Verify: snapshot lists per-level prefixes with metric/next-hop/interface; counters increment
8. **Functional tests** - finalise `test/isis/*.ci`
9. **RFC refs** - add `// RFC 2966 ...`, `// RFC 3787 ...`, `// RFC 5305 ...` comments above enforcing code
10. **Full verification** - `make ze-verify`
11. **Complete spec** - fill audit tables, write learned summary, two commits

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Feature completeness | Every End-to-End User Story has a working path (install, ECMP, withdraw, snapshot) |
| Correctness | Dijkstra matches hand-computed paths; up/down bit per RFC 2966; overload per RFC 3787 |
| Naming | admin-distance leaf `isis` (single, existing); Loc-RIB `Source` = IS-IS ProtocolID; CLI `show isis route`; metrics `ze_isis_*` |
| Data flow | Routes flow LSDB -> SPF -> Loc-RIB insertion (`InsertForward`) -> sysrib `OnChange` -> fibkernel; no bypass; no second FIB path; no redistevents on the install path |
| Rule: plugin-self-containment | All SPF/install/snapshot code under `internal/component/isis/spf/` |
| Rule: memory-architecture | `locrib.Path` value-typed; no cross-boundary pointers in the inserted Path |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| SPF package | `ls internal/component/isis/spf/` |
| Loc-RIB insertion | `grep -r 'InsertForward' internal/component/isis/spf/` |
| ECMP path-group expansion | `grep -rn 'ECMPPaths' internal/plugins/sysrib/ internal/core/rib/locrib/` |
| Functional tests | `ls test/isis/isis-route-install.ci test/isis/isis-redist-arbitration.ci` |
| Kernel route tagged RTPROT_ZE | functional test asserts IS-IS source in kernel FIB |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | LSDB-derived metrics and prefixes bounds-checked before graph build. Metric widths differ: the IS-reachability (TLV 22) metric is 24-bit (0..16777215); the IP/IPv6 prefix metric (TLV 135/236) is a 32-bit field (0..4294967295) read in full. Do NOT cap the prefix metric at 24-bit; instead, per RFC 5305 sec 4 / RFC 5308 sec 2, exclude a prefix whose metric is >= MAX_PATH_METRIC (0xFE000000) from normal SPF, and clamp the accumulated path cost at MAX_PATH_METRIC so sums never wrap |
| Resource exhaustion | SPF run rate bounded by debounce; ECMP next-hop count capped; graph size bounded by LSDB cap |
| Loop prevention | RFC 2966 up/down bit enforced on L1<->L2 leak; no re-leak of up/down-set prefixes |
| Error leakage | SPF failures logged, not panicked; a malformed LSP excludes one node, not the whole run |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test setup |
| Test fails behavior mismatch | Re-read RFC summary / Current Behavior |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to the relevant phase and implement |
| 3 fix attempts fail | STOP; report all 3 approaches; ask user |

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
- The "install IS-IS routes" requirement is mostly existing infra, but the path is Loc-RIB INSERTION, not redistevents: IS-IS becomes a Loc-RIB source (like BGP at `rib_bestchange.go:813`) and SPF only decides which `locrib.Path` values to insert. The novelty is SPF correctness (graph, Dijkstra, ECMP, leaking, overload), not the install path.
- `redistevents` was the wrong mechanism: it feeds the redistribute-orchestrator (redistribution to other protocols), which is spec-isis-11, not the FIB. Using it for FIB install would have routed IS-IS routes to redistribution consumers instead of the kernel.
- ECMP shape is constrained downstream: sysrib keys `s.routes[key]` by protocol string and only the single best Path is replayed/changed, so multiple IS-IS `locrib.Path` for one prefix would collapse. Because ECMP is in umbrella scope, this spec commits to expanding a Loc-RIB path-group into `BestChangeEntry.ECMPPaths` (no single-next-hop fallback). Validating this end to end is umbrella assumption A-2.
- Admin distance is constrained by the data model: `locrib.Path` has no protoType/level field and `bgpProtocolTypeFromPath` returns Unspecified for non-BGP, so per-level admin distance cannot be modelled in v1. IS-IS sets one distance (115) and resolves L1-over-L2 inside SPF; a per-level field is deferred future work (A-3b).

## Core Insight
SPF is the only genuinely protocol-specific compute in the route path; everything
downstream of the inserted `locrib.Path` is shared, protocol-agnostic
machinery (Loc-RIB best-path -> sysrib -> fibkernel) that already installs static,
connected, and BGP routes.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Install via Loc-RIB insertion (`locrib.Path` + `InsertForward`), not redistevents | Emit a `redistevents` route-change batch as a producer | redistevents feeds the redistribute-orchestrator (isis-11), not the FIB; Loc-RIB insertion is the actual FIB-install path, mirroring BGP `rib_bestchange.go:813` |
| One `locrib.Path` per equal-cost next-hop (distinct `Instance`) for ECMP | A single Path with a next-hop slice | Keep the value-typed `Path` unchanged; the next-hop multiplicity is expressed by distinct `Instance`; downstream path-group expansion carries them through |
| Intra-protocol ECMP: expand a Loc-RIB path-group into `BestChangeEntry.ECMPPaths` (committed) | Leave it silently collapsed; document a single-next-hop limitation | ECMP is in umbrella scope, so the expansion is a committed deliverable; sysrib keys by protocol string and replays only the best Path today (`sysrib.go:286-298,799,997`), so without the expansion the umbrella ECMP goal would not be met |
| Single IS-IS admin distance (115) on `locrib.Path`; reuse the existing `isis` leaf; L1-over-L2 resolved inside SPF | Add per-level admin-distance leaves and a protoType on the Path | `locrib.Path` has NO protoType/level field (confirmed `locrib/candidate.go`) and `bgpProtocolTypeFromPath` returns Unspecified for non-BGP (`sysrib.go:1017-1029`); per-level admin distance is not implementable in v1, so IS-IS publishes one Path per prefix after resolving L1-over-L2 internally. Per-level distance vs other protocols is deferred future work (A-3b) |
| Read the up/down bit from the TLV 135 control octet (TLV 236 flags octet) | Read it from the high bit of the metric | The up/down bit lives in the control/flags octet per RFC 5305 sec 4.1 / RFC 2966 and the umbrella canonical layout; the metric is a full 32-bit field |
| Read the full 32-bit prefix metric; accumulate path cost in a 64-bit accumulator with MAX_PATH_METRIC 0xFE000000 handling | Cap prefix metric at 24-bit and sum in 32 bits | TLV 135/236 prefix metric is 32-bit; sums of 32-bit prefix and 24-bit edge metrics can exceed 32 bits, so a 24-bit cap or 32-bit sum would wrap |
| Debounce SPF on LSDB change | Run SPF per LSP arrival | Avoid thrash on bursts/flaps; one run per level per burst |
| RFC 2966 up/down bit for leaking | Leak without loop marking | Prevent L1<->L2 loops as the RFC mandates |

## Known Limitations
- IPv4 only here; IPv6 SPF/install is spec-isis-12.
- FRR interop for convergence is owned by spec-isis-13.
- Wide metrics only (narrow metrics decoded for interop, not used as SPF edges); single-topology only.
- IS-IS sets a single admin distance (115) on `locrib.Path`; per-level (L1 vs L2) admin distance against OTHER protocols is not implementable in v1 because `locrib.Path` has no protoType/level field (`locrib/candidate.go`) and `bgpProtocolTypeFromPath` returns Unspecified for non-BGP (`sysrib.go:1017-1029`). L1-over-L2 is resolved inside IS-IS SPF. Distinct per-level distances vs other protocols are deferred future work that would need a protoType/level field on `locrib.Path` (A-3b).

## RFC Documentation
Add `// RFC 2966 Section X.Y: "<quoted up/down bit requirement>"` and
`// RFC 5305 Section 4.1: "<quoted up/down bit in the control octet requirement>"`
above the leak code (the up/down bit is read/written in the TLV 135 control octet,
TLV 236 flags octet, not the metric),
`// RFC 3787 Section X.Y: "<quoted overload transit requirement>"` above the
overload-exclusion code, and
`// RFC 5305 Section X.Y: "<quoted wide-metric requirement>"` above the
metric-width / MAX_PATH_METRIC handling (24-bit TLV 22 edge metric, 32-bit
TLV 135/236 prefix metric, wide accumulator).

## Implementation Summary

### What Was Implemented
- [To be filled]

### Bugs Found/Fixed
- [To be filled]

### Documentation Updates
- [To be filled]

### Deviations from Plan
- [To be filled]

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
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| SPF shortest path correct | unit test | `TestISISSPFShortestPath` vs hand-computed |
| sys-rib updated from IS-IS (route in kernel) | functional test | `test/isis/isis-route-install.ci` |
| ECMP installed | functional test | `test/isis/isis-route-install.ci` (ECMP step) |
| L1<->L2 leak with up/down bit, no loop | unit test | `TestISISLeakUpDownBit` |
| Overload bit honoured | unit test | `TestISISOverloadTransit` |
| Admin-distance arbitration | functional test | `test/isis/isis-redist-arbitration.ci` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [To be filled]

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
- [ ] AC-1..AC-11 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete - every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled - 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/component/isis/spf/`, including `spf/install.go` Loc-RIB insertion)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Critical Review passes
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features (out-of-scope honoured: no IPv6, no SR/TE/MT)
- [ ] Single responsibility per file (graph / spf / route / producer)
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification - owned by spec-isis-13)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-isis-9-spf-rib.md`
- [ ] Summary included in commit

## Related Specs
- `plan/spec-isis-7-flooding.md` - provides the synced LSDB this SPF reads (dependency)
- `plan/spec-isis-8-dis-broadcast.md` - provides pseudo-nodes used as graph vertices (dependency)
- `plan/spec-isis-11-redistribution.md` - IS-IS as a redistribution source/consumer (builds on this producer)
- `plan/spec-isis-12-ipv6.md` - IPv6 SPF and install (extends this for TLV 232/236)
- `plan/spec-isis-13-cli-diag-interop.md` - renders `show isis route`, scrape-asserts SPF metrics, FRR convergence interop
