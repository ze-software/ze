# Spec: ospf-8-spf-rib

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-ospf-7-lsdb-flooding.md |
| Phase | - |
| Updated | 2026-06-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-ospf-0-umbrella.md` - `## Shared Contracts`: "Route install vs redistribution" (single `AdminDistance` 110 on `locrib.Path`; intra<inter<E1<E2 resolved INSIDE OSPF SPF; ECMP committed, default cap 8; the sysrib path-group expansion into `BestChangeEntry.ECMPPaths` ALREADY EXISTS, added by IS-IS, and OSPF reuses it with NO sysrib code), "Next-hop derivation for SPF", "Route preference / path types", "LSA header + body layout", "Metrics (canonical)" (this spec OWNS `ze_ospf_spf_runs_total{area}`, `ze_ospf_spf_duration_seconds{area}`, `ze_ospf_routes_installed{type}`), the ospf-8 Child Specs / Dependency rows, the Wiring Test row `TestOSPFSPFRoute` + `test/ospf/ospf-route-install.ci`
4. `docs/research/ospf-implementation-guide.md` §6a (intra-area SPF, two-phase Dijkstra, two-way check), §6b (SPF throttling), §6g (route installation), traps #5 (Network-LSA LS ID = DR interface address) and #6 (two-way check)
5. `plan/learned/934-isis-9-spf-rib.md` - the IS-IS sibling (SPF + Loc-RIB install + ECMP); OSPF reuses the SAME Loc-RIB insertion / sysrib path-group infra, copied verbatim, but shares no code (network vertices vs pseudo-nodes; LSA vs LSP keys)
6. `internal/core/rib/locrib/manager.go` (`InsertForward` ~155, `Remove` ~256) + `candidate.go` - `Path{Source, Instance, NextHop, AdminDistance, Metric}` (NO path-type field) and best-path selection (the install target)
7. `internal/component/bgp/plugins/rib/rib_bestchange.go` (~813) - `r.locRIB.InsertForward(fam, pfx, locrib.Path{...})`, the install pattern to mirror
8. `internal/component/sysrib/sysrib.go` (`ecmpCollect` / `BestChangeEntry.ECMPPaths` ~464/513/598/645/791/1001) - the EXISTING path-group ECMP expansion OSPF reuses; sysrib keys `s.routes[key]` by protocol STRING and admin distance via `effectivePriority`
9. `internal/component/sysrib/yang/ze-rib-conf.yang` - the EXISTING `ospf` admin-distance leaf (default 110); no new leaves here

## Task

Add the intra-area Shortest Path First (SPF) computation and the system-RIB route
installation that together make a Ze node program OSPF-learned routes into the
kernel FIB. This is the core-goal spec of the OSPFv2 set: it delivers the
umbrella goal "keep the system RIB updated with OSPF-learned routes so the kernel
FIB forwards accordingly".

Build the intra-area shortest-path tree per area from the synced LSDB (ospf-7)
using the RFC 2328 §16.1 two-stage Dijkstra. **Stage 1** builds the tree over
router AND network (transit) vertices: Router-LSAs (Type 1) provide router
vertices and their link records (link type 1 point-to-point, type 2 transit,
type 4 virtual treated as type 1 through the transit area), and Network-LSAs
(Type 2) provide the transit-network (pseudonode) vertices keyed by the DR's
interface address. Stage 1 applies the TWO-WAY CHECK (trap #6): a link to a
vertex is usable only if that vertex's own LSA lists this router back, so SPF
never walks one direction across a half-up adjacency. The Network-LSA LS ID is
the DR's INTERFACE ADDRESS on the segment, not the network prefix (trap #5); SPF
looks the Network-LSA up by the interface address from the corresponding
Router-LSA transit-link descriptor, so getting the key wrong silently drops half
the topology. **Stage 2** attaches stub networks: for every router vertex already
in the tree, its Router-LSA stub links (link type 3) are added at their advertised
cost plus the tree distance to that router, retaining the router vertex's
next-hop set.

Derive next-hops per the umbrella "Next-hop derivation for SPF" contract: for a
destination reached across a transit network the next hop is the IP address of
the next-hop router's interface on that network, learned from that router's
Router-LSA transit-link Link Data (RFC 2328 §16.1.1); for a directly-attached
point-to-point neighbour the next hop is the neighbour's interface address. The
calculating router resolves the first hop on the SPT toward the destination, and
a child vertex inherits the parent's next-hop set during the SPT walk (a parent's
next-hops are COPIED on an equal-cost tie, not replaced).

For ECMP (committed, umbrella scope, default cap 8 paths), merge equal-cost
parents so the destination keeps every equal-cost next-hop, and emit one
`locrib.Path` per equal-cost next-hop with a distinct `Instance`. Throttle SPF
with exponential back-off (initial delay, hold, max-hold doubling per consecutive
trigger within the window, reset on a quiet window) triggered on any LSDB change
(ospf-7 notification) and debounced so a burst of LSA arrivals coalesces into one
run per area.

Maintain an OSPF route table whose entries carry a path-type marker. In this spec
only intra-area routes exist (inter-area added by ospf-9, external E1/E2 added by
ospf-10); the RFC 2328 §11 intra < inter < E1 < E2 preference is resolved INSIDE
OSPF (the route-table merge) before publishing exactly one winning `locrib.Path`
per prefix, because `locrib.Path` has no path-type field. Install the SPF result
by INSERTING routes into the unified Loc-RIB, exactly as BGP does
(`internal/component/bgp/plugins/rib/rib_bestchange.go:813`
`r.locRIB.InsertForward(fam, pfx, locrib.Path{...})`) and IS-IS does
(`internal/plugins/isis/spf/install.go`). Per umbrella Shared Contracts "Route
install vs redistribution" this is the FIB-install path, NOT `redistevents`:
`redistevents` feeds the redistribute-orchestrator (redistribution to BGP) and is
owned by ospf-10, never the FIB-install path. For each SPF prefix, SPF builds a
`locrib.Path{Source = OSPF ProtocolID, Instance, NextHop, AdminDistance = 110,
Metric}` and calls `InsertForward` on add/change and the matching forward-remove
(`loc.Remove`) on loss. sysrib then consumes the Loc-RIB via `loc.OnChange`,
applies admin distance through `effectivePriority`, recomputes the system best,
and fibkernel programs the kernel route marked `RTPROT_ZE`.

The sysrib path-group ECMP expansion into `BestChangeEntry.ECMPPaths` ALREADY
EXISTS (added by IS-IS, `internal/component/sysrib/sysrib.go`,
`sysrib_ecmp_pathgroup_test.go`). OSPF reuses it for free: equal-cost OSPF
next-hops reach the kernel as a multipath route with NO sysrib or locrib code in
this spec. The `ospf` admin-distance leaf in `ze-rib-conf.yang` ALSO already
exists (default 110), so this spec adds NO sysrib config either.

Expose `show ip ospf route` (the SPF route table with metric, next-hop(s),
interface, path-type) and `show ip ospf spf` (per-area SPF run state: last run,
duration, node count, throttle timers); the full reference rendering is owned by
ospf-13, which only scrapes/asserts the metrics this spec registers.

Package: `internal/plugins/ospf/spf/`, which both computes intra-area SPF and
inserts the resulting `locrib.Path` values into the Loc-RIB. Inter-area (ospf-9),
external (ospf-10), and stub/NSSA (ospf-11) route computation extend this package
behind the route-table and install seams created here.

## Required Reading

### Architecture Docs
- [ ] `docs/research/ospf-implementation-guide.md` §6a "Intra-area SPF (Dijkstra)" (~371-393) - two-phase Dijkstra, vertex types, two-way check, next-hop computation
  → Decision: Phase 1 builds the tree over router + network (transit) vertices from Router-LSAs and Network-LSAs; Phase 2 attaches Router-LSA stub links to already-in-tree router vertices
  → Constraint: the two-way check is mandatory - when visiting a target vertex via a link, the target's own LSA MUST list a reciprocal link back to the current vertex, else the link is unusable (trap #6)
  → Constraint: equal-cost parents are MERGED to support ECMP; a parent's next-hops are COPIED on an equal-cost tie, not replaced
- [ ] `docs/research/ospf-implementation-guide.md` §6b "SPF Throttling" (~394-401) - exponential back-off
  → Constraint: three knobs - initial delay (batch first run), hold time (next run within the window), max-hold (hold doubles per consecutive trigger, capped); hold resets to initial when a window passes with no trigger
- [ ] `docs/research/ospf-implementation-guide.md` §6g "Route Installation" (~457-466) - what each installed route carries; SPF replaces the old table via a diff handed to the FIB
  → Constraint: each route carries prefix, metric, admin distance (110), a path-type marker, and an ECMP next-hop set; between runs the new table replaces the old via add/change/remove deltas, the FIB is not reloaded from scratch
- [ ] `docs/research/ospf-implementation-guide.md` traps #5 (~1460-1464) and #6 (~1466-1470) - Network-LSA LS ID confusion, two-way check in SPF
  → Constraint: a Network-LSA's LS ID is the DR's INTERFACE ADDRESS on the segment, not the prefix; SPF looks it up by the interface address from the Router-LSA transit-link descriptor (the network mask is in the LSA body). Manufacture a one-way LSDB (A mentions B, B does not mention A) and verify SPF installs no route through the broken adjacency
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` (~813) - the install pattern to mirror
  → Constraint: BGP mirrors its best path into the shared Loc-RIB via `r.locRIB.InsertForward(fam, pfx, locrib.Path{Source, Instance, NextHop, AdminDistance, Metric}, forward)`; OSPF install copies this exactly, NOT a redistevents emit
- [ ] `internal/plugins/isis/spf/install.go` + `internal/plugins/isis/spf/route.go` - the IS-IS sibling install + route-table pattern to mirror
  → Constraint: one `locrib.Path` per equal-cost next-hop (distinct `Instance`), `InsertForward` on add/change, `loc.Remove` on loss; preference (L1-over-L2 for IS-IS; intra<inter<E1<E2 for OSPF) resolved inside the protocol before one Path is published; ProtocolID registered once at startup
- [ ] `internal/core/rib/locrib/manager.go` (`InsertForward` ~155, `Remove` ~256) + `candidate.go` - cross-protocol best path (the install target)
  → Constraint: Loc-RIB selects lower AdminDistance first, then lower Metric, then first-seen (stable); OSPF supplies `Source`/`Instance`/`NextHop`/`AdminDistance`/`Metric`; value-typed, no cross-boundary pointers; `Path` has no path-type field, so OSPF resolves intra<inter<E1<E2 internally and publishes one Path per prefix
- [ ] `internal/component/sysrib/sysrib.go` (`ecmpCollect`, `BestChangeEntry.ECMPPaths` ~464/513/598/645/791/1001, `effectivePriority`) - sysrib consumes the Loc-RIB and expands path-groups
  → Constraint: the path-group ECMP expansion into `BestChangeEntry.ECMPPaths` ALREADY EXISTS (added by IS-IS); OSPF reuses it with NO sysrib change. sysrib keys `s.routes[key]` by protocol STRING and admin distance comes from `effectivePriority` -> `rib.admin-distance.*`; `redistevents` is NOT used on this path
- [ ] `internal/component/sysrib/yang/ze-rib-conf.yang` - the existing `ospf` admin-distance leaf (default 110)
  → Constraint: the `ospf` leaf already exists; this spec uses it as-is and adds NO admin-distance leaves (`locrib.Path` has no path-type field, so per-path-type distance vs other protocols is future work)

### RFC Summaries (MUST for protocol work; existing, read before implementation)
- [ ] `rfc/short/rfc2328.md` - OSPF Version 2: §16.1 intra-area SPF (two-stage Dijkstra, two-way check, §16.1.1 next-hop), §11 routing-table-entry preference (intra < inter < E1 < E2)
  → Constraint: §16.1 stage 1 over router + transit vertices with the two-way check; stage 2 attaches Router-LSA stub links; §16.1.1 next-hop is the next-hop router's interface address on the transit network (Router-LSA Link Data) or the P2P neighbour's interface address; §11 preference resolved before install

**Key insights:**
- SPF is the only genuinely OSPF-specific compute in the route path; everything downstream of the inserted `locrib.Path` (Loc-RIB best-path -> sysrib -> fibkernel `RTPROT_ZE`) is shared, protocol-agnostic machinery that already installs static / connected / BGP / IS-IS routes.
- Route install is Loc-RIB INSERTION (`InsertForward`, mirror BGP `rib_bestchange.go:813` and IS-IS `spf/install.go`), NOT `redistevents`. `redistevents` feeds the redistribute-orchestrator (redistribution to BGP, ospf-10) and would route OSPF routes to consumers instead of the kernel.
- OSPF adds NO sysrib/locrib code: the `BestChangeEntry.ECMPPaths` path-group expansion and the `ospf` admin-distance leaf (default 110) BOTH already exist. OSPF inserts one `locrib.Path` per equal-cost next-hop (distinct `Instance`) and gets kernel ECMP for free; default ECMP cap 8.
- The two correctness traps are OSPF-specific and concentrate the risk: the Network-LSA LS ID is the DR interface address (trap #5), and the two-way check (trap #6) prevents phantom routes over half-up adjacencies. Both are tested on hand-built LSDBs before any on-wire run.
- `locrib.Path` has no path-type field, so the RFC 2328 §11 intra < inter < E1 < E2 preference is resolved INSIDE the OSPF route table; OSPF publishes one winning Path per prefix with a single AdminDistance 110. Per-path-type distance vs other protocols is future work.
- Next-hop derivation: transit-network next hop from the next-hop router's Router-LSA Link Data; P2P next hop from the neighbour interface address; child vertices inherit the parent's next-hop set, copied (not replaced) on an equal-cost tie (RFC 2328 §16.1.1).

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
- [ ] `internal/core/rib/locrib/manager.go` (`InsertForward` ~155, `Remove` ~256) + `candidate.go` - `Path{Source, Instance, NextHop, AdminDistance, Metric}` is the value-typed install unit with NO path-type field; best-path selection is lower AdminDistance, then lower Metric, then first-seen
  → Constraint: OSPF INSTALLS by building `locrib.Path` and inserting it; with no path-type field, per-path-type admin distance is not possible, so OSPF sets a single `AdminDistance` (110) and resolves intra<inter<E1<E2 inside its route table; see A-3
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` (~813) - the install pattern: `r.locRIB.InsertForward(fam, pfx, locrib.Path{Source: bgpProtocolID, Instance: pathID, NextHop, AdminDistance, Metric}, forward)`
  → Constraint: copy this exact insertion shape; `Source` = OSPF ProtocolID, `Instance` distinguishes the ECMP next-hop; NO redistevents emit on the install path
- [ ] `internal/plugins/isis/spf/install.go` + `route.go` - the IS-IS sibling: one `locrib.Path` per equal-cost next-hop, `InsertForward` on add/change, `loc.Remove` on loss, ProtocolID registered once, preference resolved in the route table before publishing one Path
  → Constraint: OSPF mirrors this above-the-wire pattern but shares NO code (OSPF has network vertices for transit LANs; IS-IS uses pseudo-node LSPs; LSA vs LSP lookup keys and metric semantics differ - umbrella §11)
- [ ] `internal/component/sysrib/sysrib.go` (`ecmpCollect`, `BestChangeEntry.ECMPPaths` ~464/513/598/645/791/1001, `effectivePriority`, `s.routes[key]` by protocol string) - sysrib consumes the Loc-RIB and EXPANDS path-groups into `BestChangeEntry.ECMPPaths`
  → Constraint: the path-group ECMP expansion ALREADY EXISTS (added by IS-IS); OSPF reuses it with NO change; admin distance via `effectivePriority` -> `rib.admin-distance.ospf`; `redistevents` is NOT on this path
- [ ] `internal/component/sysrib/yang/ze-rib-conf.yang` - `rib.admin-distance` container; has an `ospf` leaf (default 110) used as-is; NO new leaves here
- [ ] `internal/plugins/fib/kernel/backend_linux.go` + `internal/core/rtproto/rtproto.go` - kernel routes are tagged `RTPROT_ZE` (`rtproto.FIBKernel`); the FIB programming path is protocol-agnostic

**Behavior to preserve:**
- The protocol-agnostic FIB-install path (Loc-RIB insertion -> `sysrib` `OnChange` -> `fibkernel`) is unchanged; OSPF plugs in as a Loc-RIB source like BGP and IS-IS.
- Loc-RIB best-path semantics (AdminDistance then Metric then first-seen) and sysrib `recomputeBest` / `effectivePriority` semantics are unchanged.
- Existing Loc-RIB sources (static, connected, L2TP, BGP, IS-IS) keep working unchanged.
- The existing `rib.admin-distance.ospf` leaf (default 110) is used as-is; no admin-distance leaves are added.
- The sysrib/locrib `BestChangeEntry.ECMPPaths` path-group expansion already exists and is reused unchanged; OSPF adds no sysrib/locrib code.
- `redistevents` and the redistribute-orchestrator are not touched here; redistribution to BGP is ospf-10.

**Behavior to change:**
- New `internal/plugins/ospf/spf/` package that computes intra-area SPF and INSERTS the resulting `locrib.Path` values into the Loc-RIB.
- OSPF becomes a Loc-RIB source identified by its registered ProtocolID; it sets a single `AdminDistance` (110) on each Path (no path-type field on `locrib.Path`).
- A per-area SPF route table with a path-type marker (intra-area only here; inter-area/E1/E2 added by ospf-9/10), resolving intra<inter<E1<E2 internally before publishing one Path per prefix.
- An exponential-back-off SPF throttle triggered on LSDB change.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- LSDB change notification from ospf-7 (Router-LSA / Network-LSA added, updated, or purged; adjacency Full/Down) on the relevant area.
- A periodic safety timer also enqueues an SPF run for an area even with no change.

### Transformation Path
1. **Throttle/debounce:** an LSDB change marks the area dirty and arms the exponential-back-off timer (initial delay; hold doubles per consecutive trigger within the window, capped at max-hold; resets after a quiet window); coalesces a burst of LSA arrivals into one SPF run per area.
2. **Graph read:** read the synced LSDB for the area; Router-LSAs (Type 1) give router vertices + link records; Network-LSAs (Type 2) give transit-network vertices keyed by the DR interface address (= the Network-LSA LS ID, trap #5).
3. **Stage 1 Dijkstra (transit):** seed the candidate list with self (root, cost 0); extract the minimum, mark it in the SPT, and for each of its Router-LSA links (type 1 p2p -> router vertex by neighbour Router ID; type 2 transit -> network vertex by DR interface address; type 4 virtual -> as type 1) or Network-LSA attached-routers (cost 0 from the pseudonode) apply the TWO-WAY CHECK (trap #6: the target's own LSA must list a reciprocal link back), compute the candidate distance, and add/merge it; equal-cost parents are merged for ECMP. Resolve the next-hop: transit-network next hop = the next-hop router's interface address from its Router-LSA transit-link Link Data (§16.1.1); P2P next hop = the neighbour interface address; child vertices inherit the parent's next-hop set (copied, not replaced, on an equal-cost tie).
4. **Stage 2 (stub attach):** for every router vertex in the SPT, add its Router-LSA stub links (link type 3) at tree-distance + stub cost, retaining the router vertex's next-hop set; record each as an intra-area route.
5. **Route-table merge:** assemble the per-area intra-area routes into the OSPF route table with a path-type marker; resolve intra < inter < E1 < E2 internally (only intra-area exists here; inter/external added by ospf-9/10) so exactly one winning result per prefix is chosen.
6. **Diff:** compare the new per-prefix result set against the previously installed set; produce add / change / remove deltas.
7. **Install (Loc-RIB insertion):** for each added/changed prefix, build `locrib.Path{Source = OSPF ProtocolID, Instance (ECMP next-hop discriminator), NextHop, AdminDistance (single 110), Metric}` and call `locRIB.InsertForward(fam, pfx, path, forward)` (one Path per equal-cost next-hop, distinct `Instance`, default cap 8); for lost prefixes / shrinking ECMP sets call the matching `loc.Remove`. This mirrors BGP `rib_bestchange.go:813` and IS-IS `spf/install.go`. No `redistevents` emit (that path is redistribution, ospf-10).
8. **Arbitrate + program:** Loc-RIB best-path (single admin distance 110, then metric, then first-seen) -> sysrib consumes `loc.OnChange` / snapshot `replayPath` -> `recomputeBest` -> fibkernel netlink -> kernel route tagged `RTPROT_ZE`. Intra-protocol ECMP survives via the EXISTING Loc-RIB path-group expansion into `BestChangeEntry.ECMPPaths` (added by IS-IS), so the kernel receives a multipath route with no sysrib code here.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| LSDB ↔ SPF | in-process change notification + LSDB read (same component) | [ ] |
| SPF/engine ↔ Loc-RIB | `locrib.Path{Source, Instance, NextHop, AdminDistance, Metric}` via `InsertForward` (value-typed) | [ ] |
| Loc-RIB ↔ sys-rib | `loc.OnChange` + snapshot `replayPath`; best by (AdminDistance, Metric); EXISTING path-group expansion into `BestChangeEntry.ECMPPaths` for equal-cost next-hops | [ ] |
| sys-rib ↔ kernel | existing best-change -> fibkernel netlink (`RTPROT_ZE`) | [ ] |

### Integration Points
- New package `internal/plugins/ospf/spf/` (graph build, two-stage Dijkstra, next-hop derivation, route table, diff, Loc-RIB insertion).
- Loc-RIB insertion via `InsertForward` (mirror BGP `rib_bestchange.go:813`, IS-IS `spf/install.go`); OSPF is a new Loc-RIB source, not a redistevents producer.
- The existing `rib.admin-distance.ospf` leaf in `ze-rib-conf.yang` (no schema change here).
- The EXISTING sysrib/locrib path-group ECMP expansion into `BestChangeEntry.ECMPPaths` (added by IS-IS); reused unchanged.
- `show ip ospf route` / `show ip ospf spf` snapshot APIs (rendered in ospf-13).
- Prometheus SPF metrics (`ze_ospf_spf_runs_total{area}`, `ze_ospf_spf_duration_seconds{area}`, `ze_ospf_routes_installed{type}`) are owned and registered HERE per the umbrella canonical table; ospf-13 only scrapes/surfaces them.

### Architectural Verification
- [ ] No bypassed layers (LSDB -> SPF -> Loc-RIB insertion -> sysrib `OnChange` -> fibkernel -> kernel)
- [ ] No unintended coupling (SPF reads the LSDB; no second FIB path; no direct netlink from OSPF; no redistevents on the install path; no IS-IS coupling)
- [ ] No duplicated functionality (route install reuses Loc-RIB + sysrib; arbitration reuses Loc-RIB best-path; ECMP reuses the existing path-group expansion; no new sysrib code)
- [ ] Value-typed boundary preserved (`locrib.Path` fields are value types; no cross-boundary pointers)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The synced LSDB (ospf-7) exposes a per-area read API and change notification sufficient to build the SPF graph (Router-LSA link records, Network-LSA attached-routers, by `(Type, LS ID, Adv Router)` key) without re-parsing raw bytes more than once per run | ospf-7 LSDB design; umbrella "LSA header + body layout" | SPF must add its own parsed-topology cache | `TestOSPFGraphBuild` on a hand-built LSDB | unvalidated |
| A-2 | FIB install is via Loc-RIB insertion (`locrib.Path` + `InsertForward`), not redistevents; ECMP reuses the EXISTING sysrib/locrib path-group expansion into `BestChangeEntry.ECMPPaths` (added by IS-IS) with NO sysrib code | `internal/component/bgp/plugins/rib/rib_bestchange.go:813`, `internal/component/sysrib/sysrib.go` `ecmpCollect`; umbrella ECMP note + A-2 | If the existing expansion does not generalise to a second IGP, ECMP install fails | `test/ospf/ospf-route-install.ci` end-to-end to kernel with multiple next-hops | unvalidated |
| A-3 | A single OSPF admin distance (110) on `locrib.Path.AdminDistance` (existing `rib.admin-distance.ospf` leaf) plus Loc-RIB best-path (AdminDistance then Metric) expresses cross-protocol arbitration; intra<inter<E1<E2 is resolved inside the OSPF route table before publishing one Path (`locrib.Path` has no path-type field) | `internal/core/rib/locrib/candidate.go`, `ze-rib-conf.yang`; umbrella A-3 | Per-path-type distance vs OTHER protocols needs a path-type field on `locrib.Path` | multi-source functional test + intra-area route test | unvalidated |
| A-3b | Per-path-type admin distance vs other protocols (intra-area vs external at different distances against static/BGP) is NOT implementable in v1 and is deferred as future work | `internal/core/rib/locrib/candidate.go` has no path-type field | A deployment needing distinct per-path-type distances would add a path-type field to `locrib.Path` and per-type leaves to `ze-rib-conf.yang` | future spec when the field is added | deferred (future work) |
| A-4 | The Network-LSA can be located during the SPT walk by the DR interface address taken from the Router-LSA transit-link descriptor (LS ID = DR interface address, trap #5), keyed `(Type 2, LS ID, Adv Router)` in the LSDB | RFC 2328 §16.1; guide trap #5 | SPF drops transit-LAN topology | `TestOSPFTransitNetworkSPF` (LAN with a Network-LSA) | unvalidated |
| A-5 | First-hop next-hop resolution (transit-network = next-hop router's Router-LSA Link Data; P2P = neighbour interface address; parent inheritance) yields the correct outgoing next-hop without consulting the live interface table | RFC 2328 §16.1.1; umbrella "Next-hop derivation for SPF" | Need an extra interface/adjacency lookup for the local outgoing interface | `TestOSPFNextHop` next-hop assertion | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Two-way check omitted -> phantom routes over a half-up adjacency (trap #6) | routes via a neighbour that does not list us back | Enforce the two-way check in stage 1; `TestOSPFTwoWayCheck` on a one-way LSDB; FRR convergence interop (ospf-13) |
| R-2 | Network-LSA looked up by the prefix instead of the DR interface address (trap #5) -> half the transit topology silently dropped | LAN destinations missing from the route table | Key the Network-LSA by `(Type 2, LS ID = DR interface address, Adv Router)`; `TestOSPFTransitNetworkSPF` |
| R-3 | Next-hop inheritance wrong (replaced instead of copied on an equal-cost tie) -> wrong or lost ECMP next-hops | ECMP next-hop count wrong vs hand-computed | Copy the parent next-hop set on an equal-cost tie per §16.1.1; `TestOSPFNextHop`, `TestOSPFSPFECMP` |
| R-4 | SPF thrash on a flapping link starves the engine | high `ze_ospf_spf_runs_total` rate | Exponential-back-off throttle coalesces bursts; `TestOSPFSPFThrottle` |
| R-5 | Stale routes left in the kernel after adjacency loss | prefix still in FIB after dead-interval | Diff against the previously installed set and forward-remove (`loc.Remove`) the lost `locrib.Path`; `ospf-route-install.ci` withdraw step |
| R-6 | Intra-protocol ECMP does not survive to the kernel | single next-hop in kernel when two expected | Reuse the EXISTING `BestChangeEntry.ECMPPaths` path-group expansion (added by IS-IS); validate A-2 end-to-end with a multi-next-hop kernel route; no sysrib code added |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| LSDB change notification on an area | → | throttle arms, SPF runs, route set produced | `TestOSPFSPFRoute` |
| SPF result change | → | SPF inserts `locrib.Path` (Source=OSPF) via `InsertForward` | `TestOSPFSPFRoute` (asserts the inserted Path) |
| SPF route inserted, two/three nodes | → | Loc-RIB -> sysrib `OnChange` -> fibkernel -> kernel route (`RTPROT_ZE`, OSPF source) | `test/ospf/ospf-route-install.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Hand-built LSDB with a known router/transit topology (Router-LSAs + a Network-LSA) | Two-stage Dijkstra produces the shortest-path metric and next-hop that matches the hand-computed result for every prefix |
| AC-2 | LSDB where router A's Router-LSA lists B but B's Router-LSA does not list A | The two-way check rejects the link; SPF installs no route through the broken adjacency (trap #6) |
| AC-3 | Broadcast LAN with a Network-LSA (LS ID = DR interface address) | The transit-network vertex is found by the DR interface address from the Router-LSA transit link; all routers behind the LAN are reached (trap #5) |
| AC-4 | A destination reached across a transit network | The installed next-hop is the next-hop router's interface address from its Router-LSA Link Data (§16.1.1), not the DR or the prefix |
| AC-5 | Two/three nodes, a remote prefix originated | The remote prefix appears in the local kernel FIB with the OSPF-derived next-hop, tagged `RTPROT_ZE` |
| AC-6 | Two equal-cost intra-area paths to the same prefix | Both next-hops are installed (ECMP) via one `locrib.Path` per next-hop (distinct `Instance`, cap 8); the EXISTING path-group expansion carries both into `BestChangeEntry.ECMPPaths` |
| AC-7 | Two equal-cost OSPF paths, end to end | The kernel route for the prefix is a multipath route with both OSPF next-hops (`RTPROT_ZE`), proving the existing `BestChangeEntry.ECMPPaths` expansion reaches fibkernel with no sysrib change |
| AC-8 | An adjacency is lost (Router-LSA re-origination / LSA purge) | Affected prefixes are removed from the Loc-RIB (`loc.Remove`) and withdrawn from the kernel |
| AC-9 | Same prefix also present from static / BGP with a lower admin distance | The lower-admin-distance source wins in sysrib; OSPF (110) loses; raising the other source above 110 makes OSPF win |
| AC-10 | A burst of LSA arrivals within the throttle window | SPF runs once per area for the burst, not once per LSA; the hold time backs off exponentially and resets after a quiet window |
| AC-11 | `show ip ospf route` and `show ip ospf spf` invoked | `route` lists per-area prefixes with metric, next-hop(s), interface, and path-type (intra-area); `spf` lists per-area last-run time, duration, node count, and throttle state |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Expects remote prefixes in the kernel FIB | LSDB change -> throttle -> two-stage SPF -> `locrib.Path` `InsertForward` -> Loc-RIB -> sysrib `OnChange` -> fibkernel -> kernel (`RTPROT_ZE`) (NOT `redistevents`) | `test/ospf/ospf-route-install.ci` |
| 2 | Expects ECMP installed for equal-cost paths | SPF merges equal-cost parents -> one `locrib.Path` per next-hop (distinct `Instance`, cap 8) -> EXISTING path-group expansion into `BestChangeEntry.ECMPPaths` -> sysrib ECMP collect -> kernel multipath | `test/ospf/ospf-route-install.ci` (ECMP step) |
| 3 | Expects routes withdrawn when an adjacency dies | adjacency/LSA loss -> SPF diff -> `loc.Remove` of the lost `locrib.Path` -> sysrib withdraw -> kernel route removed | `test/ospf/ospf-route-install.ci` (withdraw step) |
| 4 | Runs `show ip ospf route` / `show ip ospf spf` | CLI -> RPC -> SPF route-table / run-state snapshot (rendering in ospf-13) | `test/ospf/ospf-show.ci` (ospf-13) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOSPFGraphBuild` | `internal/plugins/ospf/spf/graph_test.go` | router vertices + link records from Router-LSAs; transit-network vertices from Network-LSAs keyed by DR interface address; stub links recorded for stage 2 | |
| `TestOSPFSPFShortestPath` | `internal/plugins/ospf/spf/spf_test.go` | two-stage Dijkstra on a hand-built LSDB matches hand-computed metric/next-hop for every prefix | |
| `TestOSPFTwoWayCheck` | `internal/plugins/ospf/spf/spf_test.go` | a one-way LSDB (A lists B, B does not list A) installs no route through the broken adjacency (trap #6) | |
| `TestOSPFTransitNetworkSPF` | `internal/plugins/ospf/spf/spf_test.go` | a LAN Network-LSA (LS ID = DR interface address) is found by the Router-LSA transit-link address; all routers behind the LAN reached (trap #5) | |
| `TestOSPFNextHop` | `internal/plugins/ospf/spf/spf_test.go` | transit-network next-hop = next-hop router's Router-LSA Link Data; P2P next-hop = neighbour interface address; parent next-hops copied (not replaced) on an equal-cost tie (§16.1.1) | |
| `TestOSPFSPFECMP` | `internal/plugins/ospf/spf/spf_test.go` | equal-cost parent merge yields multiple next-hops, capped at 8 | |
| `TestOSPFStubAttach` | `internal/plugins/ospf/spf/spf_test.go` | stage 2 attaches Router-LSA stub links at tree-distance + stub cost, retaining the router vertex next-hop set | |
| `TestOSPFRouteTablePreference` | `internal/plugins/ospf/spf/route_test.go` | intra-area path-type marker set; intra < inter < E1 < E2 resolution publishes one winning result per prefix (intra-area only present here) | |
| `TestOSPFRouteDiff` | `internal/plugins/ospf/spf/route_test.go` | add/change/remove deltas computed correctly between runs | |
| `TestOSPFSPFThrottle` | `internal/plugins/ospf/spf/spf_test.go` | exponential back-off: a burst coalesces to one run per area; hold doubles per consecutive trigger, caps at max-hold, resets after a quiet window | |
| `TestOSPFInstallPath` | `internal/plugins/ospf/spf/install_test.go` | SPF result -> `locrib.Path{Source=OSPF, Instance, NextHop, AdminDistance 110, Metric}` with one Path per ECMP next-hop -> `InsertForward`; `loc.Remove` on loss/shrink | |
| `TestOSPFSPFRoute` | `internal/plugins/ospf/spf/install_test.go` | LSDB change -> SPF -> Loc-RIB insertion (wiring) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Router-LSA / Network link metric (16-bit) | 0..65535 | 65535 | N/A | 65536 |
| Stub-link metric (16-bit) | 0..65535 | 65535 | N/A | 65536 |
| Total path metric (sum of 16-bit link costs) | 0..0xFFFF total per LSInfinity convention | LSInfinity (0xFFFFFF) treated as unreachable | N/A | metric ≥ LSInfinity excluded from the tree |
| ECMP next-hop count per prefix | 1..8 (default cap) | 8 | 0 (no route) | > 8 (truncate) |
| Admin distance (ospf, single existing leaf) | 0..255 | 255 | N/A | 256 |
| SPF throttle initial delay / hold / max-hold (ms) | 0..max-hold | max-hold | N/A | clamp at max-hold |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-route-install` | `test/ospf/ospf-route-install.ci` | two/three nodes: remote prefix appears in kernel with OSPF source; ECMP installs multiple next-hops; adjacency loss withdraws | |
| `ospf-redist-arbitration` | `test/ospf/ospf-redist-arbitration.ci` | same prefix from OSPF and static/BGP arbitrated by admin distance (OSPF 110) | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| (deferred to spec-ospf-13) | `test/interop/scenarios/` | FRR ospfd | SPF convergence and route install validated against FRR in `ospf-p2p-frr` / `ospf-broadcast-frr` | |

### Future (if deferring any tests)
- FRR interop for SPF/convergence is owned by spec-ospf-13 (`ospf-p2p-frr`, `ospf-broadcast-frr`); this spec proves intra-area SPF + install with Ze-to-Ze functional tests. Inter-area (ospf-9), external (ospf-10), and stub/NSSA (ospf-11) route computation extend this package and carry their own tests.

## Files to Modify
- NOTE: NO change to `internal/component/sysrib/sysrib.go` or `internal/core/rib/locrib/` - the `BestChangeEntry.ECMPPaths` path-group expansion ALREADY EXISTS (added by IS-IS) and is reused unchanged.
- NOTE: NO change to `internal/component/sysrib/yang/ze-rib-conf.yang` - the existing `ospf` admin-distance leaf (default 110) is used and no per-path-type leaves are added (`locrib.Path` has no path-type field, so per-type distance is not implementable in v1).
- `internal/plugins/ospf/instance.go` (or `area.go`) - wire the SPF Computer to the per-area LSDB change notification (the `triggerSPF` arming point); held by ospf-4's scaffolding

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | No | uses existing `rib.admin-distance.ospf` leaf; ECMP cap default 8 lives in the `ospf` config surface owned by ospf-4 (this spec reads it) |
| YANG validation constraints | No | existing `ospf` leaf already `type uint8`, `default 110` |
| YANG custom validators | No | not needed |
| CLI commands/flags | Yes | `show ip ospf route` and `show ip ospf spf` snapshot RPCs (rendered in ospf-13) |
| CLI grammar (action before identifier) | Yes | `show ip ospf route` / `spf` follow `ai/rules/cli-grammar.md` |
| Editor autocomplete | No | no new config leaves; `show ip ospf` completion is YANG-driven and owned by ospf-13 |
| Functional test for new RPC/API | Yes | `test/ospf/ospf-route-install.ci`, `test/ospf/ospf-redist-arbitration.ci` |
| Pipe completeness | Yes | `show ip ospf route` / `spf` output through `ApplyPipes`/`ProcessPipes` (ospf-13) |
| Doctor check for runtime dependencies | No | install path reuses fibkernel; no new runtime dependency here |
| Prometheus counters/metrics | Yes | this spec OWNS and registers `ze_ospf_spf_runs_total{area}`, `ze_ospf_spf_duration_seconds{area}` (histogram), `ze_ospf_routes_installed{type}` (gauge) per the umbrella "Metrics (canonical)" table. Per-owner registration here; ospf-13 only scrapes/asserts |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` (OSPF route install row) |
| 2 | Config syntax changed? | No | existing `rib.admin-distance.ospf` leaf reused; no new config |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (`show ip ospf route` / `spf`, in ospf-13) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (`show ip ospf route` / `spf`) |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/ospf.md` (SPF + route install) |
| 7 | Wire format changed? | No | LSA codec belongs to ospf-2 |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc2328.md` (§16.1 SPF, §16.1.1 next-hop, §11 preference) |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (new `test/ospf/` cases) |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` (OSPF SPF/install row) |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` (OSPF as a Loc-RIB source via `InsertForward`, like BGP and IS-IS) |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` (`ze_ospf_spf_runs_total`, `ze_ospf_spf_duration_seconds`, `ze_ospf_routes_installed` owned and registered HERE per the canonical table; surfaced in ospf-13) |
| 15 | Registered plugin/event/command/capability changed? | Yes | `docs/plugin-overview.md` (OSPF registered as a Loc-RIB source ProtocolID) |
| 16 | Changed files referenced by doc source anchors? | No | grep at completion |
| 17 | Existing docs show examples for this area? | No | grep at completion |

## Files to Create
- `internal/plugins/ospf/spf/graph.go` - per-area graph build from the LSDB: router vertices + link records from Router-LSAs; transit-network vertices from Network-LSAs keyed by DR interface address (trap #5); stub links recorded for stage 2
- `internal/plugins/ospf/spf/spf.go` - the RFC 2328 §16.1 two-stage Dijkstra: stage 1 over router + transit vertices with the two-way check (trap #6) and §16.1.1 next-hop derivation + parent inheritance + equal-cost merge (ECMP, cap 8); stage 2 stub attach
- `internal/plugins/ospf/spf/route.go` - per-area route table with path-type marker, intra<inter<E1<E2 internal resolution (intra-area only here), diff against the installed set, `show ip ospf route` snapshot
- `internal/plugins/ospf/spf/computer.go` - exponential-back-off SPF throttle / debounce orchestration; owns and registers `ze_ospf_spf_runs_total{area}`, `ze_ospf_spf_duration_seconds{area}`; `show ip ospf spf` run-state snapshot
- `internal/plugins/ospf/spf/install.go` - build `locrib.Path{Source = OSPF ProtocolID, Instance (per ECMP next-hop), NextHop, AdminDistance (single 110), Metric}` per (prefix, next-hop) and call `locRIB.InsertForward` on add/change, `loc.Remove` on loss/shrink (mirror BGP `rib_bestchange.go:813`, IS-IS `spf/install.go`); register the OSPF ProtocolID once at startup; owns and registers `ze_ospf_routes_installed{type}`; preference already resolved in `route.go` so one result set per prefix is published
- `internal/plugins/ospf/spf/graph_test.go`, `spf_test.go`, `route_test.go`, `install_test.go` - unit tests (install_test asserts the inserted `locrib.Path` and the forward-remove)
- `test/ospf/ospf-route-install.ci` - end-to-end route install (add, ECMP, withdraw)
- `test/ospf/ospf-redist-arbitration.ci` - admin-distance arbitration against static/BGP

Note: `internal/plugins/ospf/redistribute/` (the `redistevents` producer + `RedistConsumer`) is owned by spec-ospf-10, NOT this spec. This spec installs to the FIB only via Loc-RIB insertion. Inter-area (ospf-9), external (ospf-10), and stub/NSSA (ospf-11) route computation extend this package behind the route-table and install seams.

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

1. **Phase: Wiring (MANDATORY FIRST)** - register the OSPF ProtocolID, wire the Loc-RIB insertion stub, and write the failing route-install test
   - Tests: `TestOSPFSPFRoute`, `test/ospf/ospf-route-install.ci` (fails: no SPF result yet)
   - Files: `internal/plugins/ospf/spf/install.go` (register the OSPF ProtocolID, hold the Loc-RIB handle, `InsertForward` entry point), an SPF entry-point stub in `spf/spf.go`; arm the trigger from `instance.go`/`area.go`
   - Verify: the OSPF ProtocolID registers, the Loc-RIB handle is held, the wiring test fails because SPF returns no routes (no redistevents anywhere)
2. **Phase: Graph build** - vertices and edges from the LSDB
   - Tests: `TestOSPFGraphBuild`
   - Files: `spf/graph.go`
   - Verify: router vertices + link records from Router-LSAs; transit-network vertices from Network-LSAs keyed by the DR interface address (trap #5); stub links recorded for stage 2
3. **Phase: Stage 1 Dijkstra + two-way check + next-hop + ECMP** - shortest-path tree over router + transit vertices
   - Tests: `TestOSPFSPFShortestPath`, `TestOSPFTwoWayCheck`, `TestOSPFTransitNetworkSPF`, `TestOSPFNextHop`, `TestOSPFSPFECMP`
   - Files: `spf/spf.go`
   - Verify: metric/next-hop match hand-computed; one-way adjacency rejected (trap #6); transit-LAN reached via the Network-LSA (trap #5); next-hop per §16.1.1 with parent inheritance (copied on tie); equal-cost parents merged, capped at 8
4. **Phase: Stage 2 stub attach + route table + diff** - route output
   - Tests: `TestOSPFStubAttach`, `TestOSPFRouteTablePreference`, `TestOSPFRouteDiff`
   - Files: `spf/spf.go` (stub attach), `spf/route.go` (route table, preference, diff, snapshot)
   - Verify: stub links attached at tree-distance + stub cost; intra-area path-type marker; intra<inter<E1<E2 resolution publishes one result per prefix (intra only here); diff yields add/change/remove
5. **Phase: Throttle + install** - trigger and Loc-RIB insertion
   - Tests: `TestOSPFSPFThrottle`, `TestOSPFInstallPath`, `test/ospf/ospf-route-install.ci`
   - Files: `spf/computer.go` (exponential back-off throttle), `spf/install.go` (one `locrib.Path` per next-hop -> `InsertForward`; `loc.Remove` on loss/shrink)
   - Verify: burst coalesces to one run per area; hold backs off and resets; `locrib.Path` inserted with Source=OSPF; route appears in kernel as `RTPROT_ZE`; ECMP installs multiple next-hops via the EXISTING path-group expansion; adjacency loss removes
6. **Phase: Arbitration + ECMP reuse verification** - cross-protocol selection and the reused intra-protocol path-group expansion
   - Tests: `test/ospf/ospf-redist-arbitration.ci`, `TestOSPFInstallPath` (ECMP)
   - Files: none in sysrib/locrib (reuse the EXISTING `BestChangeEntry.ECMPPaths` expansion and the existing `ospf` admin-distance leaf, default 110); only `internal/plugins/ospf/spf/install.go`
   - Verify: OSPF at 110 (single admin distance) loses to lower-admin-distance sources and wins when raised; intra<inter<E1<E2 holds (resolved inside SPF); intra-protocol ECMP installs multiple next-hops in the kernel via `BestChangeEntry.ECMPPaths`; existing single-Path sources (static, connected, BGP, IS-IS) unaffected (no sysrib change)
7. **Phase: snapshot + metrics** - `show ip ospf route` / `show ip ospf spf` snapshots and SPF metrics
   - Files: `spf/route.go` (route snapshot), `spf/computer.go` (run-state snapshot); the `ze_ospf_spf_runs_total`/`ze_ospf_spf_duration_seconds`/`ze_ospf_routes_installed` series are registered HERE (per-owner); ospf-13 only renders/scrape-asserts them
   - Verify: route snapshot lists per-area prefixes with metric/next-hop/interface/path-type; spf snapshot lists per-area run state; counters increment
8. **Functional tests** - finalise `test/ospf/*.ci`
9. **RFC refs** - add `// RFC 2328 Section 16.1 ...` comments above the two-way check, the next-hop derivation, and the stage boundaries
10. **Full verification** - `make ze-verify`
11. **Complete spec** - fill audit tables, write learned summary, two commits

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Feature completeness | Every End-to-End User Story has a working path (install, ECMP, withdraw, snapshot) |
| Correctness | Two-stage Dijkstra matches hand-computed paths; two-way check (trap #6) rejects one-way links; Network-LSA looked up by DR interface address (trap #5); next-hop per §16.1.1 (parent inheritance copied on tie) |
| Naming | admin-distance leaf `ospf` (single, existing, 110); Loc-RIB `Source` = OSPF ProtocolID; CLI `show ip ospf route` / `spf`; metrics `ze_ospf_spf_runs_total`/`_duration_seconds`/`ze_ospf_routes_installed` (exact owned names only) |
| Data flow | Routes flow LSDB -> SPF -> Loc-RIB insertion (`InsertForward`) -> sysrib `OnChange` -> fibkernel; no bypass; no second FIB path; no redistevents on the install path; no sysrib/locrib code added |
| Rule: plugin-self-containment | All SPF/route/install/snapshot code under `internal/plugins/ospf/spf/` |
| Rule: memory-architecture | `locrib.Path` value-typed; no cross-boundary pointers in the inserted Path |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| SPF package | `ls internal/plugins/ospf/spf/` |
| Two-stage Dijkstra + two-way check | `grep -rn 'two-way\|twoWay\|transit' internal/plugins/ospf/spf/` |
| Loc-RIB insertion | `grep -r 'InsertForward' internal/plugins/ospf/spf/` |
| ECMP reuse (no sysrib change) | `git diff --name-only` shows NO `internal/component/sysrib/` or `internal/core/rib/locrib/` files |
| Functional tests | `ls test/ospf/ospf-route-install.ci test/ospf/ospf-redist-arbitration.ci` |
| Kernel route tagged RTPROT_ZE | functional test asserts OSPF source in kernel FIB |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | LSDB-derived metrics, link records, and prefixes bounds-checked before graph build; link counts and Network-LSA attached-router lists validated against the LSA Length before slicing; a metric ≥ LSInfinity excluded from the tree |
| Resource exhaustion | SPF run rate bounded by the exponential-back-off throttle; ECMP next-hop count capped at 8; graph size bounded by the per-area LSDB cap |
| Loop prevention | The two-way check (trap #6) prevents one-way phantom paths; inter-area/external loop rules belong to ospf-9/10/11 |
| Error leakage | SPF failures logged, not panicked; a malformed LSA excludes one vertex, not the whole run |

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
- The "install OSPF routes" requirement is mostly existing infra, but the path is Loc-RIB INSERTION, not redistevents: OSPF becomes a Loc-RIB source (like BGP at `rib_bestchange.go:813` and IS-IS at `spf/install.go`) and SPF only decides which `locrib.Path` values to insert. The novelty is SPF correctness (two-stage Dijkstra, the two-way check, transit-network handling, next-hop derivation, ECMP), not the install path.
- `redistevents` is the wrong mechanism here: it feeds the redistribute-orchestrator (redistribution to BGP, ospf-10), not the FIB. Using it for FIB install would route OSPF routes to redistribution consumers instead of the kernel.
- ECMP costs OSPF zero new sysrib/locrib code: the `BestChangeEntry.ECMPPaths` path-group expansion already exists (added by IS-IS). OSPF inserts one `locrib.Path` per equal-cost next-hop (distinct `Instance`, cap 8) and the kernel gets a multipath route for free.
- Admin distance is constrained by the data model: `locrib.Path` has no path-type field, so per-path-type admin distance cannot be modelled in v1. OSPF sets one distance (110) and resolves intra<inter<E1<E2 inside its route table; a per-path-type field is deferred future work (A-3b).

## Core Insight
The intra-area two-stage Dijkstra (with the two-way check and the transit-network
vertex keyed by the DR interface address) is the only genuinely OSPF-specific
compute in the route path; everything downstream of the inserted `locrib.Path` is
shared, protocol-agnostic machinery (Loc-RIB best-path -> sysrib path-group
expansion -> fibkernel) that already installs static, connected, BGP, and IS-IS
routes. OSPF therefore adds a route table and a SPF package, and zero new
infrastructure below the inserted Path.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Install via Loc-RIB insertion (`locrib.Path` + `InsertForward`), not redistevents | Emit a `redistevents` route-change batch as a producer | redistevents feeds the redistribute-orchestrator (ospf-10), not the FIB; Loc-RIB insertion is the actual FIB-install path, mirroring BGP `rib_bestchange.go:813` and IS-IS `spf/install.go` |
| One `locrib.Path` per equal-cost next-hop (distinct `Instance`), reuse the EXISTING `BestChangeEntry.ECMPPaths` expansion | A single Path with a next-hop slice; or re-add an ECMP path-group | Keep the value-typed `Path` unchanged; the path-group expansion already exists (added by IS-IS), so OSPF reuses it with NO sysrib/locrib code; default cap 8 |
| Single OSPF admin distance (110) on `locrib.Path`; reuse the existing `ospf` leaf; intra<inter<E1<E2 resolved inside the route table | Add per-path-type admin-distance leaves and a path-type field on the Path | `locrib.Path` has NO path-type field, so per-type distance is not implementable in v1; OSPF publishes one Path per prefix after resolving preference internally. Per-type distance vs other protocols is deferred future work (A-3b) |
| Two-way check enforced in stage 1 | Trust the LSDB and walk links unconditionally | RFC 2328 §16.1 mandates the reciprocal-link check (trap #6); without it SPF walks one direction across a half-up adjacency and installs non-forwarding routes |
| Network-LSA keyed/looked up by the DR interface address (= LS ID) | Key by the network prefix | RFC 2328 / trap #5: a Network-LSA's LS ID is the DR interface address, not the prefix (the mask is in the body); SPF looks it up by the Router-LSA transit-link address, else it silently drops half the topology |
| Exponential back-off SPF throttle on LSDB change | Run SPF per LSA arrival; or a flat fixed debounce | Avoid thrash on bursts/flaps; the guide §6b model (initial delay, hold doubling, max-hold, reset) is what every production implementation uses |

## Known Limitations
- Intra-area routing only here; inter-area (Type 3/4) is spec-ospf-9, AS-external (Type 5, E1/E2) is spec-ospf-10, stub/NSSA (Type 7) is spec-ospf-11. The route table and install seams created here are extended by those specs.
- IPv4 only (OSPFv2 is IPv4-only by definition; IPv6 is OSPFv3, a separate component).
- FRR interop for convergence is owned by spec-ospf-13.
- OSPF sets a single admin distance (110) on `locrib.Path`; per-path-type (intra vs inter vs external) admin distance against OTHER protocols is not implementable in v1 because `locrib.Path` has no path-type field (`locrib/candidate.go`). Intra<inter<E1<E2 is resolved inside OSPF. Distinct per-type distances vs other protocols are deferred future work that would need a path-type field on `locrib.Path` (A-3b).

## RFC Documentation
Add `// RFC 2328 Section 16.1: "<quoted two-stage Dijkstra requirement>"` above the
stage-1/stage-2 boundaries, `// RFC 2328 Section 16.1: "<quoted two-way check
requirement>"` above the reciprocal-link check (the target vertex's LSA must list a
link back to the current vertex), `// RFC 2328 Section 16.1.1: "<quoted next-hop
calculation requirement>"` above the transit-network / P2P next-hop derivation and
the parent-inheritance (copy-on-tie) logic, and `// RFC 2328 Section 11: "<quoted
routing-table-entry preference requirement>"` above the intra<inter<E1<E2
route-table resolution.

## Implementation Summary

### What Was Implemented
- Pending: no implementation has run for this design spec; fill with observed implementation evidence during /ze-implement.

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
| Intra-area SPF shortest path correct | unit test | `TestOSPFSPFShortestPath` vs hand-computed |
| Two-way check / transit-network correctness | unit test | `TestOSPFTwoWayCheck`, `TestOSPFTransitNetworkSPF` |
| System RIB updated with OSPF routes (route in Loc-RIB, then kernel) | unit + wiring + functional | `TestOSPFSPFRoute` / `TestOSPFInstallPath` + `test/ospf/ospf-route-install.ci` |
| ECMP installed (reusing existing expansion) | unit + functional | `TestOSPFSPFECMP` + `test/ospf/ospf-route-install.ci` (ECMP step) |
| Admin-distance arbitration | unit + functional | sysrib cross-protocol selection + `test/ospf/ospf-redist-arbitration.ci` |
| SPF throttle | unit test | `TestOSPFSPFThrottle` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | pending | `/ze-review` not run yet for this design spec | this spec | run during implementation; record concrete findings here |

### Fixes applied
- Pending: record concrete fixes after `/ze-review` reports BLOCKER or ISSUE findings.

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
- [ ] Feature code integrated (`internal/plugins/ospf/spf/`, including `spf/install.go` Loc-RIB insertion)
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
- [ ] No speculative features (out-of-scope honoured: intra-area only; no inter-area/external/stub/NSSA here)
- [ ] Single responsibility per file (graph / spf / route / computer / install)
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification - owned by spec-ospf-13)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ospf-8-spf-rib.md`
- [ ] Summary included in commit

## Related Specs
- `plan/spec-ospf-0-umbrella.md` - umbrella; Shared Contracts (route install vs redistribution, next-hop derivation, route preference, metrics) this spec references
- `plan/spec-ospf-7-lsdb-flooding.md` - provides the synced LSDB this SPF reads (dependency)
- `plan/spec-ospf-9-inter-area-abr.md` - inter-area (Type 3/4) route computation extending this route table/install
- `plan/spec-ospf-10-as-external-asbr.md` - AS-external (Type 5, E1/E2) route computation + OSPF as a redistribution source/consumer
- `plan/spec-ospf-11-stub-nssa.md` - stub/NSSA (Type 7) route computation
- `plan/spec-ospf-13-cli-diag-interop.md` - renders `show ip ospf route` / `spf`, scrape-asserts SPF metrics, FRR convergence interop
