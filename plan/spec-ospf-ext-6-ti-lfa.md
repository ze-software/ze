# Spec: ospf-ext-6 -- OSPFv2 TI-LFA / LFA Fast Reroute (RFC 5286 + TI-LFA)

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-ospf-ext-5-segment-routing.md |
| Phase | - |
| Updated | 2026-06-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `rfc/short/rfc5286.md` -- LFA math: Inequality 1 loop-free (§3.1), Inequality 2 downstream (§1.1), Inequality 3 node-protecting (§3.2), Inequality 4 broadcast/PN (§3.3), §3.5 cost/overload gate, §3.6 selection order, §4/§4.1 use-and-terminate, §6.1 multi-homed-prefix, §6.3 OSPF multi-area constraints, Errata 2323 (downstream test against `D_opt(S,D)`)
4. `rfc/short/rfc8665.md` -- the SR label carriage the repair list is built from: Prefix-SID Sub-TLV (§5, V/L flags), Adj-SID Sub-TLV (§6.1, B-Flag eligible-for-FRR), SID/Label 3-octet (label in 20 rightmost bits) vs 4-octet (index) form, SRGB index->label mapping (§3.2)
5. `plan/spec-ospf-ext-5-segment-routing.md` -- the SR control plane this spec consumes: the resolved Prefix-SID -> label map, the per-adjacency Adj-SID, the SRGB/SRLB tables, and the SR FIB-install seam ext-6 extends with a repair (backup) label stack
6. `plan/spec-ospf-ext-0-umbrella.md` "ext-6 depends on ext-5 + SPF"; the contract that SR/TI-LFA READ the delivered SPF route table and install backup forwarding state through the SAME `locrib.Path` Loc-RIB seam, never a second FIB path
7. `internal/plugins/ospf/spf/spf.go` -- `Compute`/`computeWithNextHop` Dijkstra, `Result`, `NodeResult`, `NextHop`, `NextHopSource` (the AF seam), `LSInfinity`; this spec adds per-neighbour SPF roots (`SPT(N)`) and a reverse SPF
8. `internal/plugins/ospf/spf/route.go` -- `RouteEntry{AreaID,Prefix,Metric,Type,Origin,NextHops}`, `BuildRoutes`, `selectBestRoutes`, `RouteDelta`, `DiffRoutes`, `Snapshot`; the backup next-hop attaches to `RouteEntry` here
9. `internal/plugins/ospf/spf/install.go` -- `Installer`, `Apply`, `insert` (`loc.InsertForward` with `locrib.Path`); the backup install rides this seam
10. `internal/core/rib/locrib/candidate.go` -- `locrib.Path` (Source, Instance, NextHop, AdminDistance, Metric, Labels, IsEBGP); has NO backup-nexthop field today -- ext-6 must add the carry-through backup field
11. `internal/plugins/fib/kernel/richroute.go` + `nexthop_linux.go` -- `RichRoute{Prefix,NextHop,Metric,Labels,SRv6SID,ECMPPaths}`, `buildRichRoute`; supports ECMP multipath + MPLS/SRv6 encap but NO backup (RTNH_F_LINKDOWN) next-hop -- ext-6 must add the backup install path

## Task

Add OSPFv2 **fast-reroute backup next-hop computation and installation** to the
native OSPF plugin at `internal/plugins/ospf/`. After each SPF run the engine
already produces, for every prefix, a primary next-hop set (`RouteEntry.NextHops`
in `spf/route.go`). This spec computes, alongside each primary, a **pre-computed
loop-free backup next-hop** so a single local failure (link or node) can be
repaired locally the instant it is detected, before the IGP reconverges. The
backup is programmed into the FIB next to the primary; on primary-down the
forwarding plane swings to the backup with no control-plane recompute.

Two computation tiers are in scope:

1. **Base LFA (RFC 5286).** For each primary next-hop, find a directly-connected
   neighbour `N` whose own shortest path to the destination does not loop back
   through the computing router `S`. This requires the loop-free inequality
   (Inequality 1, §3.1), and, where available, the stronger downstream criterion
   (Inequality 2, §1.1) and the node-protecting criterion (Inequality 3, §3.2).
   The inputs are extra SPFs: one shortest-path tree rooted at each IGP neighbour
   (`SPT(N)`) plus the already-computed `SPT(S)`, giving `D_opt(N,D)`,
   `D_opt(N,S)`, `D_opt(N,E)`, `D_opt(E,D)`. The §3.5 cost/overload gate, the §3.6
   selection order (prefer node-and-link-protecting, then node-protecting, then
   link-protecting), the §3.3 broadcast-link pseudo-node rule (Inequality 4), the
   §6.1 multi-homed-prefix transform, and Errata 2323 (downstream measured against
   `D_opt(S,D)`) all apply.

2. **TI-LFA (topology-independent LFA).** Where base LFA finds no
   directly-connected loop-free neighbour, build an explicit **repair list** -- a
   stack of Segment Routing labels (a Prefix-SID toward a "P-node" reachable
   without crossing the failure, then optionally an Adj-SID across the protected
   link/node, per the post-convergence P-space / Q-space construction of
   `draft-ietf-rtgwg-segment-routing-ti-lfa`). The repair list is computed from
   the post-convergence SPF (the SPF of the topology with the protected resource
   removed) and the SR labels delivered by ext-5 (Prefix-SIDs and Adj-SIDs,
   RFC 8665 §5 / §6.1). TI-LFA gives 100% single-failure coverage and steers the
   repaired traffic onto the post-convergence path, avoiding transient
   micro-loops that a plain LFA could create. The repair list is installed as the
   backup next-hop's MPLS label stack.

The work runs entirely inside the existing OSPF edge plugin and the delivered
SPF / Loc-RIB / FIB seams. The backup next-hop (an address plus an optional SR
repair label stack) attaches to the existing `RouteEntry`, flows through the
existing `Installer.Apply` -> `locrib.InsertForward` seam on a new carry-through
backup field of `locrib.Path`, and is programmed by the FIB as a backup
next-hop alongside the primary. No new wire protocol behaviour is introduced:
RFC 5286 and TI-LFA are control-plane / forwarding-plane computations that change
no OSPF, LDP, or SR packet on the wire (the Adj-SID B-Flag advertised by ext-5,
RFC 8665 §6.1, is the only wire signal and it already exists). OSPFv3 (the v6
engine that runs inside this same plugin via the v6 codec) gets the same LFA/TI-LFA
computation through the existing `NextHopSource` AF seam; the SR label carriage
for OSPFv3 (RFC 8666) is out of scope and is recorded as a follow-on.

### In scope (this spec)

| Item | Detail |
|------|--------|
| Per-neighbour SPF (`SPT(N)`) | Run one extra SPF rooted at each directly-connected IGP neighbour, reusing the existing `computeWithNextHop` Dijkstra with the neighbour as root, to obtain `D_opt(N,D)` for every destination D (RFC 5286 §3) |
| Base LFA selection | For each primary next-hop, evaluate every candidate neighbour against Inequality 1 (§3.1), classify downstream (Ineq. 2), node-protecting (Ineq. 3), and link-protecting; apply the §3.6 preference order; honour the §3.5 cost/overload gate and the §3.3 broadcast pseudo-node rule |
| TI-LFA repair list | When no per-prefix LFA exists, compute the post-convergence SPF (protected link/node excluded) and build the P-space/Q-space SR repair list: Prefix-SID to the P-node + optional Adj-SID across the protected resource, sourced from ext-5's Prefix-SID/Adj-SID label maps (RFC 8665 §5/§6.1) |
| Backup on `RouteEntry` | Extend `spf/route.go` `RouteEntry` with a per-primary backup next-hop (address + optional SR repair label stack + protection class); `DiffRoutes`/`routeEqual`/`Snapshot` account for it |
| `locrib.Path` backup field | Add a carry-through backup field (backup next-hop + repair label stack) to `internal/core/rib/locrib/candidate.go` `Path`, excluded from arbitration keys (like `Labels`/`IsEBGP`); OSPF `Installer.insert` populates it |
| FIB backup install | Extend `RichRoute` + `buildRichRoute` (`internal/plugins/fib/kernel/`) to program a backup next-hop with the Linux multipath backup flag (RTNH_F_LINKDOWN-class) and the repair-list MPLS encap; the kernel forwards to the backup when the primary link is down |
| Per-prefix protection | Backup computed per primary next-hop per prefix (RFC 5286 §3.6 / §3.8), not a single shared per-next-hop alternate, so multi-homed and node-protection coverage is preserved |
| OSPF multi-area constraints | Suppress LFA where the §6.3 area-leakage conditions hold (backbone with non-meshed virtual links; inter-area route with multiple alternate ABRs; external-route multi-ASBR cases) so a wrong local-area path does not micro-loop |
| Use + terminate | Mark the backup "in use" on primary-down detection; bound how long it is used (§4 / §4.1) via the existing convergence/hold path; the backup is used only for shortest-path-routed unicast (§4) |
| CLI | `show ospf route fast-reroute` (or `... backup`) listing each prefix's primary + backup next-hop, protection class (LP/NP/SP), and repair label stack; `show ospf border-routers` unaffected |
| Config | A YANG `fast-reroute` container under `ospf` (and per-area / per-interface enable, LFA-vs-TI-LFA mode, node-protection preference) |
| Metrics | `ze_ospf_fast_reroute_*` series (protected/unprotected prefix counts, backup installs, per-class coverage) |

### Out of scope (recorded so it is not silently assumed done)

| Item | Where / why |
|------|-------------|
| Micro-loop-avoidance timers (`[MICROLOOP]` / `[ORDERED-FIB]` ordered-FIB convergence) | Explicitly excluded by the task; RFC 5286 §4.1 references them as alternative termination schemes -- not implemented; the basic §4.1(a/b/c) termination is used |
| Non-SR remote-LFA tunnels (targeted-LDP rLFA, RFC 7490) | Explicitly excluded; the only remote-repair mechanism here is the SR repair list (TI-LFA). A topology with no LFA and no SR coverage is left unprotected for that prefix |
| OSPFv3 SR label carriage (RFC 8666) | OSPFv3 gets LFA next-hop selection through the AF seam, but its SR repair-list labels need RFC 8666 (a separate spec); recorded as A-13 / a follow-on |
| LDP label-FRR following LFAs (RFC 5286 §5 LDP config) | Ze has no LDP-FEC FRR install for OSPF LFAs in this spec; SR is the label plane used |
| BGP next-hop inheritance of the IGP alternate (§6.4) | The Loc-RIB/sysrib resolves BGP next-hops over IGP routes already; whether the backup propagates to recursively-resolved BGP routes is a sysrib concern recorded as A-12, not implemented here |
| Multicast RPF (§6.5) | RFC 5286 §6.5: alternates MUST NOT be used for multicast RPF; Ze does no OSPF multicast |
| SRLG protection (§3, §3.3) | Local-SRLG membership config is not modelled; SRLG-protecting alternates are recorded as a known limitation, not implemented (the §3.6 SRLG steps degrade to "no SRLG info") |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -->
- [ ] `docs/research/ospf-implementation-guide.md` 1550-1553 ("TI-LFA (Fast Reroute)") and §14 (FRR feature catalogue) -- the feature's place in the OSPF extension chain
  -> Decision: TI-LFA is the last link in the opaque-derived chain (ext-5 SR first); model it exactly as the guide records FRR's `ospf_ti_lfa.c` -- compute loop-free backup paths over SPF and program them as pre-computed FIB backups, sitting on top of the delivered SR control plane
  -> Constraint: this is a compute-and-install feature; it touches `spf/` (extra SPF roots + selection) and the Loc-RIB/FIB seam, never the LSDB/flooding/codec
- [ ] `plan/spec-ospf-ext-0-umbrella.md` (ext-6 row, dependency graph, "extensions install through the SAME Loc-RIB path") -- the contract this spec must not violate
  -> Constraint: SR/TI-LFA forwarding state installs through the delivered `locrib.Path` seam (`Installer.insert`), never a second FIB path; backup state is carry-through metadata, not a new arbitration input
  -> Decision: ext-6 READS the delivered SPF route table and the ext-5 label maps; it does not feed anything into the SPF graph build or the opaque LSDB
- [ ] `ai/rules/buffer-first.md` -- repair-list label-stack build and any backup encode are buffer-first
  -> Constraint: the SR repair label stack is built into a caller-owned buffer / a `[]uint32` produced once per best-path change and shared (not mutated) into `locrib.Path`, mirroring the existing `Labels` field contract in `candidate.go`
- [ ] `ai/rules/no-sprintf-alloc.md` -- no `fmt`/`+` on the SPF hot path or in the `show` render
  -> Constraint: the per-neighbour SPF and LFA selection run on every topology change; they must allocate as little as the base SPF does; the `show ospf route fast-reroute` render uses `textbuf`/`AppendTo`
- [ ] `ai/rules/qemu-testing.md` -- the FIB backup-install path is Linux-only (netlink RTNH backup flag); it MUST be validated under QEMU
  -> Constraint: the backup next-hop programming and the actual kernel failover are QEMU integration tests, never skipped for "needs hardware"; LFA/TI-LFA selection math is unit-testable without a kernel

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc5286.md` -- the LFA computation spec
  -> Constraint: §3.1 Inequality 1 -- a neighbour N is a loop-free alternate iff `D_opt(N,D) < D_opt(N,S) + D_opt(S,D)`, STRICT `<`; equality is not loop-free. Every installed backup MUST satisfy at least this
  -> Constraint: §3.2 Inequality 3 -- node protection requires `D_opt(N,D) < D_opt(N,E) + D_opt(E,D)` STRICT; on equality assume NO node protection
  -> Constraint: §1.1 Inequality 2 (downstream) -- `D_opt(N,D) < D_opt(S,D)`; Errata 2323 fixes §3.6 step 16 to measure against `D_opt(S,D)`, NOT `D_opt(P_i.neighbor,D)`
  -> Constraint: §3.5 -- MUST NOT use an alternate via a link whose cost or reverse cost is LSInfinity (`0x00ffffff`); a neighbour reachable only over a costed-out reverse link is unusable
  -> Constraint: §3.3 -- on a broadcast/NBMA primary link a node-protecting LFA is NOT automatically link-protecting; the alternate must be loop-free wrt the pseudo-node (Inequality 4) AND S's own path to N must avoid that PN
  -> Constraint: §3.6 selection order -- SHOULD prefer node-protecting; among those prefer node-and-link-protecting; with multiple primaries prefer another primary or a node-protecting alternate; SHOULD attempt at least one LFA per primary next-hop
  -> Constraint: §4 -- the alternate MUST be used only for shortest-path-routed traffic; §4.1 -- a router MUST bound how long the alternate is used and SHOULD terminate per (a) the new primary was loop-free before the change, (b) a configured hold-down expires, or (c) an unrelated topology change is notified
  -> Constraint: §6.1 -- a multi-homed prefix is modelled as a node with unidirectional links from each advertising router and no outgoing links; the alternate SHOULD consider paths via all advertising routers (a router MAY simplify to the pre-failure attachment point)
  -> Constraint: §6.3 -- in OSPF, LFAs are unsafe in the listed multi-area leakage cases (backbone with non-meshed virtual links; inter-area route with multiple alternate ABRs; external routes via multiple non-backbone ASBRs); suppress LFA there
- [ ] `rfc/short/rfc8665.md` -- the SR labels the repair list is built from (consumed via ext-5)
  -> Constraint: §5 Prefix-SID -- the repair segment toward a P-node is the P-node's Prefix-SID; V=1/L=1 means a 3-octet local label (20 rightmost bits), V=0/L=0 means a 4-octet index resolved through the originator's SRGB (§3.2). The TI-LFA repair list must resolve indices to labels via the advertised SRGB
  -> Constraint: §6.1 Adj-SID -- the repair segment across the protected link/node is an Adj-SID; the B-Flag marks an adjacency eligible for FRR protection. The TI-LFA Q-segment uses an Adj-SID, not a Prefix-SID, when the post-convergence path must cross a specific adjacency
  -> Constraint: SID/Label encoding -- 3-octet form carries the MPLS label in the 20 rightmost bits; 4-octet form is a 32-bit index. The repair stack pushed into `locrib.Path.Labels`/backup is the resolved 20-bit MPLS label sequence (outermost first), the same form the SR primary path uses

**Key insights:**
- This is an **algorithm + install** feature, not a wire feature: RFC 5286 defines no packet/TLV; the one wire signal (Adj-SID B-Flag) is already advertised by ext-5. The whole spec lives in `spf/` (computation) and the Loc-RIB/FIB seam (install).
- The base LFA needs **per-neighbour SPFs** (`SPT(N)`): the existing `computeWithNextHop(g, root, ...)` already takes an arbitrary root, so `SPT(N)` is the same Dijkstra rooted at each neighbour. No new graph code; new orchestration that runs N+1 SPFs and indexes `D_opt(N,D)`.
- The backup attaches to the **existing `RouteEntry`** and flows through the **existing install seam**; the only new plumbing in the shared core is one carry-through field on `locrib.Path` and the FIB backup-nexthop programming. Both are recorded as assumptions because the field does not exist today.
- TI-LFA's repair list is an **SR label stack**, so it depends on ext-5 having resolved Prefix-SIDs and Adj-SIDs to labels; ext-6 reads those maps, it does not parse the SR TLVs itself.
- The §6.3 OSPF multi-area constraints are the subtle correctness risk: a node-protecting LFA computed from the local-area SPF can micro-loop when the real path leaves and re-enters the area; the spec must suppress LFA in those cases.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
- [ ] `internal/plugins/ospf/spf/spf.go` -- `Compute(g, root, maxPaths)` -> `computeWithNextHop(g, root, maxPaths, v4NextHop{})` runs RFC 2328 §16.1 stage-1 Dijkstra and returns `*Result{Nodes map[VertexID]*NodeResult}`; `NodeResult{Metric, NextHops}`; `NextHop{Addr, Interface}`; `NextHopSource` is the AF seam (`P2PNextHop`/`TransitNextHop`); `LSInfinity = 0x00ffffff`; `clampMetric` saturates at LSInfinity
  -> Constraint: `computeWithNextHop` already accepts an ARBITRARY root, so `SPT(N)` is the same call with the neighbour's RouterID as root; the per-neighbour SPF reuses this verbatim. The result gives `D_opt(root, vertex)` = `Nodes[vertex].Metric`, which is exactly the `D_opt(N,*)` LFA needs
- [ ] `internal/plugins/ospf/spf/route.go` -- `RouteEntry{AreaID, Prefix, Metric, Type, Origin, NextHops []NextHop}`; `BuildRoutes` produces per-prefix entries; `selectBestRoutes` resolves preference + ECMP merge; `RouteType` is intra/inter/ext1/ext2; `DiffRoutes`/`routeEqual` diff for install; `Snapshot`/`RouteSnapshotEntry` render `show ospf route`
  -> Constraint: the backup next-hop is a NEW per-primary field on `RouteEntry`; `routeEqual` and `DiffRoutes` MUST compare it so a backup-only change re-installs; `Snapshot` MUST surface it for the new CLI
- [ ] `internal/plugins/ospf/spf/route.go` `RouteEntry.NextHops` carries the primary ECMP set; the backup is keyed per primary next-hop (each primary may have a distinct backup), so the backup is a parallel slice or a per-next-hop wrapper, not a single route-level value
  -> Constraint: per-prefix-per-primary protection (§3.6/§3.8) means the backup CANNOT be a single route-scalar; model it as one backup per primary next-hop
- [ ] `internal/plugins/ospf/spf/install.go` -- `Installer.Apply(cur)` diffs against `installed`, calls `insert`/`remove`; `insert` builds one `locrib.Path` per primary next-hop via `loc.InsertForward(fam, prefix, locrib.Path{Source, Instance, NextHop, AdminDistance, Metric}, nil)`; `loc` may be nil in a forked subprocess (install is a no-op, snapshot still tracks)
  -> Constraint: the backup rides `insert`: each `locrib.Path` for a primary gains its backup next-hop + repair labels; the nil-`loc` no-op path must remain (snapshot tracks the backup even when not installed)
- [ ] `internal/plugins/ospf/spf/graph.go` -- `BuildGraph(src, area)` decodes Router-LSAs + Network-LSAs into `Graph{Routers, Networks}`; `Source` is the narrow LSDB read API (`Summary`, `LookupLSA`); a malformed LSA excludes one vertex, not the run
  -> Constraint: the post-convergence SPF (TI-LFA) needs a graph with the protected link/node REMOVED; build it by cloning the area `Graph` and dropping the protected edge/vertex, then re-running `Compute` -- no new decode, a graph transform
- [ ] `internal/plugins/ospf/spf/computer.go` -- `Computer` orchestrates per-area SPF on a debounced trigger (`Run` -> per-area `Compute`/`BuildRoutes`/`Installer.Apply`); holds `installer *Installer`, `maxPaths`, `onChange`; `SetMaxPaths`/`SetTimers`/`SetOnChange`; metrics `mRuns`/`mDuration`
  -> Constraint: the LFA/TI-LFA computation hooks into `Computer.Run` AFTER the primary `BuildRoutes` and BEFORE `Installer.Apply`, so the backup is attached to the same `RouteEntry` set that is installed; the extra per-neighbour SPFs run inside `Run` under the same lock/debounce
- [ ] `internal/core/rib/locrib/candidate.go` -- `Path{Source, Instance, NextHop, AdminDistance, Metric, Labels []uint32, IsEBGP bool}`; `Labels` and `IsEBGP` are carry-through metadata EXCLUDED from arbitration keys; `Labels` is built once per best-path change and shared (not copied) to the FIB
  -> Constraint: the backup next-hop + repair labels are a NEW carry-through field, modelled exactly like `Labels` (excluded from `Equal`/key, shared not copied); arbitration stays AdminDistance-then-Metric and is unaffected
- [ ] `internal/plugins/fib/kernel/richroute.go` + `nexthop_linux.go` -- `RichRoute{Prefix, NextHop, RouteType, Metric, TableID, Labels, SRv6SID, ECMPPaths}`; `buildRichRoute` sets `route.Gw`/`route.MultiPath`, MPLS/SRv6 `route.Encap`; there is NO backup-next-hop / RTNH_F_LINKDOWN handling
  -> Constraint: the FIB backup install is NEW: `RichRoute` gains a backup next-hop (+ repair labels); `buildRichRoute` must emit the backup as a lower-priority / link-down-flagged multipath next-hop with the repair MPLS encap so the kernel forwards to it when the primary link goes down
- [ ] `internal/component/sysrib/ecmp.go` + `sysrib.go` -- sysrib folds equal-cost siblings into `ECMPPaths` at best-change; `ecmpCollect` gathers same-priority/metric routes; the backup is NOT equal-cost (it is strictly worse) so it MUST bypass `ecmpCollect` and travel as the dedicated backup field, not as an ECMP sibling
  -> Constraint: the backup must NOT be confused with an ECMP path: ECMP next-hops are equal-cost and load-shared; a backup is used only on primary failure. The carry-through backup field keeps them distinct end-to-end
- [ ] `internal/plugins/ospf/register.go` -- `registerOSPF()` builds the `registry.Registration` (YANG `ZeOSPFConfYANG`, `RunEngine`, config verifier, doctor); `runOSPFEngine` constructs the `Computer`/`Installer`; the v6 (OSPFv3) engine runs as a second instance over the v6 codec
  -> Constraint: the `fast-reroute` config leaf is resolved in `config.go` and threaded into the `Computer`; OSPFv3 reuses the same `Computer`/LFA path through the v6 `NextHopSource`, so LFA selection is AF-neutral and the v6 engine gets it for free (SR labels excepted)
- [ ] `internal/plugins/ospf/yang/ze-ospf-conf.yang` + `ze-ospf-cmd.yang` -- the OSPF config + command schemas; `fast-reroute` is a new container; `show ospf route fast-reroute` is a new command leaf
  -> Constraint: the new config + command surface is added here; native YANG constraints (boolean enable, enum mode) per `ai/rules/config-surface.md`

**Behavior to preserve:**
- The base SPF (`Compute`/`BuildRoutes`/`selectBestRoutes`) primary-path output and ECMP merge -- LFA is additive; with fast-reroute disabled the route set and install are byte-for-byte as today.
- The `Installer.Apply` -> `locrib.InsertForward` seam and the nil-`loc` subprocess no-op.
- `locrib.Path` arbitration (AdminDistance then Metric); the backup field is carry-through only, excluded from `Equal`/key exactly like `Labels`/`IsEBGP`.
- The existing `RichRoute` ECMP/MPLS/SRv6 programming; backup is an added path, not a change to primary or ECMP programming.
- All existing OSPF unit/functional/interop tests (a router with fast-reroute off behaves exactly as today).

**Behavior to change:** (all RFC-5286 / TI-LFA-required when fast-reroute is enabled)
- `Computer.Run`: after primary `BuildRoutes`, run per-neighbour SPFs and attach a backup next-hop to each `RouteEntry` primary.
- `RouteEntry`, `routeEqual`, `DiffRoutes`, `Snapshot`: carry and diff the backup.
- `locrib.Path`: add the carry-through backup field.
- `Installer.insert`: populate the backup field on each primary's `Path`.
- `RichRoute` + `buildRichRoute`: program the backup next-hop with the link-down/backup flag and repair MPLS encap.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Trigger:** a topology change (LSDB change / neighbour up-down) fires the existing SPF debounce -> `Computer.Run`. With `fast-reroute` enabled, `Run` performs the base SPF (as today) and then the LFA/TI-LFA pass.
- **Config:** the `fast-reroute` YANG container (enable, mode lfa|ti-lfa, node-protection preference) resolved in `config.go` and threaded into the `Computer`.
- **SR input:** ext-5's resolved Prefix-SID label map + per-adjacency Adj-SID map, read read-only when building a TI-LFA repair list.

### Transformation Path
1. **Base SPF (existing):** `Compute(g, S, maxPaths)` -> `Result` with `D_opt(S, *)`; `BuildRoutes` -> primary `RouteEntry` set.
2. **Per-neighbour SPF (new):** for each directly-connected neighbour `N` (a root-attached vertex in `Result`), run `Compute(g, N, maxPaths)` -> `Result_N` giving `D_opt(N, *)`. Index `D_opt(N,D)`, `D_opt(N,S)`, `D_opt(N,E)`. A reverse-distance lookup (`D_opt(E,D)`) reuses `Result` / `Result_N` distances.
3. **Base LFA selection (new):** for each primary next-hop `P_i` of each prefix D, walk candidate neighbours; apply the §3.5 cost/overload gate, Inequality 1 (loop-free), classify downstream (Ineq. 2), node-protecting (Ineq. 3), link-protecting (different link; broadcast pseudo-node check Ineq. 4); pick per the §3.6 order. Record the winning backup next-hop + protection class on `P_i`.
4. **TI-LFA fallback (new):** when step 3 finds no per-prefix LFA, build the post-convergence graph (clone `Graph`, drop the protected link or node), run `Compute` on it, compute P-space (nodes reachable from S without the failure) and Q-space (nodes that reach D without the failure), pick the P-node and Q-node, and assemble the SR repair list: P-node Prefix-SID (resolved through ext-5's SRGB map) + optional Adj-SID across the protected resource. Record the repair label stack as the backup.
5. **Multi-area suppression (new):** if the prefix is inter-area/external and the §6.3 leakage conditions hold, drop the computed backup (leave the prefix unprotected) to avoid a micro-loop.
6. **Attach + diff (extends existing):** the backup (address + repair labels + class) attaches to the `RouteEntry` primary; `DiffRoutes`/`routeEqual` include it so a backup-only delta re-installs.
7. **Install (extends existing):** `Installer.insert` sets the new carry-through backup field on each primary's `locrib.Path`; `InsertForward` carries it through Loc-RIB unchanged (excluded from arbitration); sysrib forwards it to the FIB as a dedicated backup, NOT an ECMP sibling.
8. **FIB program (new, Linux):** `buildRichRoute` emits the backup as a link-down-flagged backup next-hop with the repair-list MPLS encap; the kernel swings to it the instant the primary link is detected down (§4), bounded in time by the next SPF reconverge (§4.1).
9. **CLI (new):** `show ospf route fast-reroute` renders each prefix's primary + backup + class + repair stack from the route snapshot.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| LSDB graph <-> per-neighbour SPF | `Compute(g, N, ...)` reuses the existing Dijkstra with the neighbour as root (read-only over the same `Graph`) | [ ] |
| Base SPF <-> LFA selection | `Result`/`Result_N` distances feed the §3.1/§3.2/§1.1 inequalities; no graph mutation | [ ] |
| SPF <-> TI-LFA | a cloned `Graph` with the protected resource removed, re-run through `Compute`; P-space/Q-space derived from the two trees | [ ] |
| ext-5 SR maps <-> repair list | read-only lookup of Prefix-SID label / Adj-SID label; SRGB index resolution; no SR TLV parsing here | [ ] |
| `RouteEntry` <-> Loc-RIB | backup carried on `RouteEntry`, set into the new `locrib.Path` backup field in `Installer.insert` | [ ] |
| Loc-RIB <-> sysrib <-> FIB | backup is carry-through metadata (excluded from arbitration), forwarded as a dedicated backup next-hop, not an ECMP path | [ ] |
| sysrib/FIB <-> kernel | `buildRichRoute` programs the backup with the RTNH link-down/backup flag + repair MPLS encap (Linux-only, QEMU-validated) | [ ] |

### Integration Points
- `internal/plugins/ospf/spf` -- `Compute`/`computeWithNextHop` (reused for `SPT(N)`), `Result`/`NodeResult`/`NextHop` (distances), `RouteEntry` (backup field), `BuildRoutes`/`selectBestRoutes` (primary set), `Computer.Run` (orchestration), `graph.go` (post-convergence clone).
- `internal/core/rib/locrib/candidate.go` -- the new carry-through backup field on `Path`.
- `internal/component/sysrib` -- forward the backup distinct from ECMP siblings.
- `internal/plugins/fib/kernel` -- `RichRoute` + `buildRichRoute` backup-next-hop programming.
- ext-5 SR control plane (`plan/spec-ospf-ext-5-segment-routing.md`) -- READ ONLY: Prefix-SID / Adj-SID label maps + SRGB for the repair list.
- `internal/plugins/ospf` (engine) -- `config.go` (`fast-reroute` resolution), `register.go` (thread into `Computer`), `cmd_show.go` (the new CLI), metrics.

### Architectural Verification
- [ ] No bypassed layers (backup flows SPF -> `RouteEntry` -> `Installer` -> `locrib.Path` -> sysrib -> FIB, the SAME spine as the primary; the FIB backup is a programmed next-hop, not a side channel)
- [ ] No unintended coupling (ext-6 reads ext-5's label maps and the delivered SPF; it adds nothing to the LSDB/flooding/codec and names no SR TLV internals)
- [ ] No duplicated functionality (reuses `Compute` for `SPT(N)` and the post-convergence SPF; reuses `Installer`/`InsertForward`; adds only LFA selection, the repair-list builder, one `locrib.Path` field, and the FIB backup path)
- [ ] Zero-copy preserved (repair label stack built once per best-path change and shared into `locrib.Path` like `Labels`; per-neighbour SPF reuses the existing allocation profile; `show` render is buffer-first)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `computeWithNextHop(g, root, maxPaths, nh)` can be re-rooted at any neighbour to yield `SPT(N)` and `D_opt(N,D)=Nodes[D].Metric` without modification | `spf/spf.go` -- `Compute` already passes an arbitrary `root`; `Result.Nodes[id].Metric` is `D_opt(root,id)` | the LFA needs a new SPF variant; larger change | `TestPerNeighborSPFDistances` (D_opt(N,*) matches a hand-built tree) | unvalidated |
| A-2 | `locrib.Path` has no backup-nexthop field today and a carry-through field excluded from arbitration (like `Labels`/`IsEBGP`) is the correct place for it | `internal/core/rib/locrib/candidate.go` -- `Path` fields are Source/Instance/NextHop/AdminDistance/Metric/Labels/IsEBGP; arbitration is AdminDistance-then-Metric | a different RIB carrier (or a second Path) is needed; touches BGP/IS-IS | `grep` confirms no backup field; `TestLocribPathBackupCarriedNotArbitrated` | unvalidated |
| A-3 | The FIB (`RichRoute`/`buildRichRoute`) has NO backup-next-hop install today and a Linux RTNH link-down/backup multipath next-hop is the right primitive | `fib/kernel/richroute.go` + `nexthop_linux.go` -- only `Gw`/`MultiPath`/`Encap`, no RTNH_F flags | the FIB cannot express a backup; the feature is compute-only (degraded) | `grep` confirms no RTNH_F; QEMU `ospf-lfa-frr` shows kernel failover | unvalidated |
| A-4 | ext-5 exposes a resolved Prefix-SID->label map and a per-adjacency Adj-SID->label map (with SRGB index resolution) that ext-6 can read to build a repair list | `plan/spec-ospf-ext-0-umbrella.md` ext-5 row; RFC 8665 §5/§6.1; ext-5 is the stated Depends | TI-LFA cannot build a repair list; only base LFA works | ext-5 spec defines the maps; `TestTILFARepairListFromSRMaps` | unvalidated |
| A-5 | The post-convergence SPF can be produced by cloning the area `Graph` and removing the protected edge/vertex, then re-running `Compute` | `spf/graph.go` -- `Graph{Routers,Networks}` is a plain map structure; `Compute` reads it | a dedicated incremental SPF is needed for performance | `TestPostConvergenceSPFExcludesResource` | unvalidated |
| A-6 | Per-neighbour SPFs (N+1 SPF runs per area per topology change) are within the SPF debounce budget for the umbrella's target topology size | `spf/computer.go` debounce (`delay`/`hold`); base SPF is already debounced | LFA recompute stalls convergence; need throttling/caching | benchmark in `Computer.Run`; `ze_ospf_fast_reroute_compute_seconds` metric within budget | unvalidated |
| A-7 | The backup next-hop is keyed per primary next-hop (not per route), so multi-primary/ECMP prefixes get one backup per primary per §3.6/§3.8 | RFC 5286 §3.6/§3.8; `RouteEntry.NextHops` is the primary set | a route-scalar backup loses per-primary protection | `TestBackupPerPrimaryNextHop` | unvalidated |
| A-8 | Errata 2323 applies: the downstream test (§3.6 step 16) measures against `D_opt(S,D)`, not `D_opt(P_i.neighbor,D)` | `rfc/short/rfc5286.md` Errata 2323 (Verified) | downstream classification is wrong; some alternates misclassified, possible micro-loop | `TestDownstreamCriterionAgainstS` pins both forms | unvalidated |
| A-9 | OSPFv3 (v6 engine in the same plugin) gets LFA next-hop selection through the existing `NextHopSource` AF seam unchanged; only SR repair labels are v4-only here | `spf/spf.go` `NextHopSource`; `register.go` runs a second v6 engine instance | the v6 engine needs separate LFA wiring | `TestLFAv6NextHopSelection` (v6 graph, base LFA) | unvalidated |
| A-10 | The §6.3 OSPF multi-area leakage cases are detectable from the existing ABR/area state (number of alternate ABRs, virtual-link config, ASBR area membership) the inter-area/external SPF already tracks | `spf/interarea.go`/`spf/external.go` compute inter-area/external routes and know the advertising ABR/ASBR | the suppression rule cannot be evaluated; risk of micro-loop in multi-area | `TestLFASuppressedMultiAreaLeakage` | unvalidated |
| A-11 | A broadcast/NBMA primary link's pseudo-node and S's path to the alternate neighbour are derivable from the existing Network-LSA / transit-vertex graph for the Inequality 4 check | `spf/graph.go` `NetworkVertex`; `spf/spf.go` transit-edge handling | broadcast-link LFAs are wrongly classified link-protecting | `TestBroadcastPseudoNodeRule` | unvalidated |
| A-12 | sysrib forwarding the backup as a dedicated (non-ECMP) next-hop does not perturb recursive BGP next-hop resolution over the OSPF route | `sysrib/ecmp.go` `ecmpCollect` (equal-cost only); §6.4 (BGP inherits IGP next-hop) | BGP routes resolve onto the backup or the ECMP merge breaks | `TestBackupNotFoldedIntoECMP` + existing sysrib ECMP tests green | unvalidated |
| A-13 | OSPFv3 SR label carriage (RFC 8666) is genuinely out of scope and base-LFA-only v6 protection is acceptable for this spec | task "OUT OF SCOPE" + umbrella ext-5/8666 split | v6 TI-LFA silently missing where expected | documented as Known Limitation; `ospf-v6` suite unaffected | unvalidated |
| A-14 | `RouteType` (intra/inter/ext1/ext2) is sufficient to gate which prefixes are LFA-eligible and which need the §6.3 suppression, with no new route classification | `spf/route.go` `RouteType` | a new route attribute is needed to drive suppression | `TestLFAEligibilityByRouteType` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Inequality 1 implemented with `<=` instead of strict `<` installs a looping backup | a backup forwards back through S in an interop topology; ping loops on failover | strict `<` enforced; `TestLoopFreeStrictInequality` pins equality as NOT loop-free; FRR interop failover ping has no loop |
| R-2 | Errata 2323 not applied: downstream test against `D_opt(P_i.neighbor,D)` misclassifies alternates | downstream-only topologies pick a non-downstream backup that micro-loops under node failure | implement against `D_opt(S,D)`; `TestDownstreamCriterionAgainstS`; RFC comment cites Errata 2323 |
| R-3 | TI-LFA repair list pushes a wrong SR label (index vs label form, or wrong SRGB resolution) so the repaired packet is mis-forwarded | QEMU failover drops or mis-routes; label stack mismatch vs FRR | resolve V/L per RFC 8665 §5; SRGB index->label via ext-5's map; `TestTILFARepairListFromSRMaps`; QEMU label-capture compare vs FRR |
| R-4 | Backup folded into ECMP (treated as equal-cost) load-shares onto the worse path even with no failure | traffic uses the backup in steady state; throughput/latency anomaly | backup is a dedicated carry-through field, never in `ecmpCollect`; `TestBackupNotFoldedIntoECMP` |
| R-5 | N+1 per-neighbour SPFs blow the convergence budget on large topologies | SPF duration metric spikes; convergence interop test times out | benchmark + `ze_ospf_fast_reroute_compute_seconds`; degrade to link-protecting-only or per-next-hop simplification (§3.8) under a configured cap |
| R-6 | §6.3 multi-area leakage not suppressed: an inter-area/external LFA micro-loops where the real path leaves the area | multi-area interop failover loops | implement the §6.3 suppression set; `TestLFASuppressedMultiAreaLeakage`; multi-area QEMU interop |
| R-7 | FIB backup install uses the wrong RTNH flag and the kernel never fails over (or always uses the backup) | QEMU primary-down does not swing traffic, or steady-state uses backup | use the link-down/backup multipath primitive; QEMU asserts both steady-state (primary) and post-failure (backup) forwarding |
| R-8 | Node-protecting classified on Inequality 3 equality (should assume NO node protection) over-claims protection | a node failure is not actually protected though reported NP | strict `<` for Ineq. 3; equality -> not node-protecting; `TestNodeProtectionStrictInequality` |
| R-9 | Broadcast-link LFA claimed link-protecting though S's own path to N crosses the same pseudo-node (§3.3) | a LAN link failure is not repaired despite an installed "link-protecting" backup | implement Inequality 4 + the S->N-avoids-PN check; `TestBroadcastPseudoNodeRule` |
| R-10 | `locrib.Path` backup field accidentally enters the arbitration key, changing best-path selection for BGP/IS-IS | unrelated protocol route flaps; sysrib churn | exclude from `Equal`/key exactly like `Labels`/`IsEBGP`; `TestLocribPathBackupCarriedNotArbitrated` + full sysrib suite green |
| R-11 | Backup not re-diffed: a backup-only change (same primary) is not re-installed | failover uses a stale backup after a topology change that did not move the primary | `routeEqual`/`DiffRoutes` include the backup; `TestBackupOnlyChangeReinstalls` |
| R-12 | Repair label stack mutated after sharing into `locrib.Path` corrupts a concurrent reader | intermittent wrong label under load (race) | build once per best-path change, never mutate (the `Labels` contract); race test under `-race` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `fast-reroute { enable }` config resolved + threaded into `Computer` | -> | `config.go` resolves the leaf; `Computer` runs the LFA pass in `Run` | `TestFastRerouteConfigEnablesLFAPass` (unit) + `test/ospf/ospf-lfa-config.ci` |
| An SPF run with a topology that has a base LFA | -> | `Computer.Run` -> per-neighbour SPF -> LFA selection -> backup on `RouteEntry` -> `Installer.insert` sets `locrib.Path` backup | `test/ospf/ospf-lfa-compute.ci` |
| An SPF run with NO base LFA but SR coverage | -> | TI-LFA fallback builds the repair list from ext-5 maps -> backup label stack on `RouteEntry` | `test/ospf/ospf-ti-lfa-compute.ci` |
| An installed route with a backup | -> | `buildRichRoute` programs the backup next-hop + repair encap into the kernel | `ospf-lfa-frr` QEMU interop (kernel failover observed) |
| `show ospf route fast-reroute` | -> | `cmd_show.go` renders primary + backup + class + repair stack from the snapshot | `test/ospf/ospf-lfa-show.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `fast-reroute { enable }` set; a triangle topology where a neighbour is a loop-free alternate | after SPF the protected prefix's `RouteEntry` carries a backup next-hop satisfying Inequality 1 (`D_opt(N,D) < D_opt(N,S) + D_opt(S,D)`, strict); with fast-reroute off the route is unchanged from today |
| AC-2 | A candidate neighbour N where `D_opt(N,D) == D_opt(N,S) + D_opt(S,D)` (equality) | N is NOT selected as an LFA (strict `<`, §3.1) |
| AC-3 | A topology with a downstream neighbour (`D_opt(N,D) < D_opt(S,D)`) and a non-downstream LFA | the downstream alternate is preferred and classified downstream; the test is measured against `D_opt(S,D)` (Errata 2323) |
| AC-4 | A topology where a neighbour avoids the primary node E (`D_opt(N,D) < D_opt(N,E) + D_opt(E,D)`, strict) | the backup is classified node-protecting; on Inequality 3 equality it is NOT node-protecting (§3.2) |
| AC-5 | A neighbour reachable only over a link with cost or reverse cost `LSInfinity` | that neighbour is NOT used as an alternate (§3.5) |
| AC-6 | A broadcast/NBMA primary link with a candidate alternate whose S->N path crosses the same pseudo-node | the alternate is NOT classified link-protecting unless Inequality 4 + the S->N-avoids-PN check pass (§3.3) |
| AC-7 | Multiple candidate alternates (node-and-link-protecting, node-only, link-only) | selection follows §3.6: node-and-link-protecting preferred, then node-protecting, then link-protecting; at least one LFA per primary next-hop is attempted |
| AC-8 | A prefix with no base LFA but a P-node reachable via SR | a TI-LFA repair list is built (P-node Prefix-SID resolved via the ext-5 SRGB map, + Adj-SID across the protected resource where needed) and stored as the backup label stack |
| AC-9 | A TI-LFA repair where the post-convergence path crosses a specific adjacency | the Q-segment uses an Adj-SID (RFC 8665 §6.1), not a Prefix-SID; the label form (3-octet local label, 20 rightmost bits) matches the SR primary path |
| AC-10 | A prefix with both an ECMP primary set and a backup | each primary next-hop has its own backup (per §3.6/§3.8); the backup is NOT merged into the ECMP set |
| AC-11 | An installed OSPF route with a backup, on Linux | the FIB programs the backup as a link-down/backup multipath next-hop with the repair MPLS encap; the kernel forwards to it when the primary link is down and to the primary otherwise |
| AC-12 | A backup-only change (the primary next-hop is unchanged but the backup changes) | `DiffRoutes`/`routeEqual` detect it and re-install the route so the new backup is programmed |
| AC-13 | The `locrib.Path` backup field on an OSPF path | it is carry-through metadata excluded from arbitration (AdminDistance-then-Metric unchanged); BGP/IS-IS best-path selection is unaffected |
| AC-14 | An inter-area or external prefix in a §6.3 leakage topology (multiple alternate ABRs / multi-ASBR / non-meshed virtual links) | LFA is suppressed for that prefix (no backup installed) to avoid a micro-loop |
| AC-15 | `show ospf route fast-reroute` | each protected prefix lists primary + backup next-hop, protection class (LP/NP/downstream), and the repair label stack; unprotected prefixes are shown as unprotected |
| AC-16 | OSPFv3 (v6 engine) with fast-reroute enabled and a base-LFA topology | the v6 route set carries a base LFA backup next-hop via the AF seam (SR repair labels are out of scope for v6 and absent) |
| AC-17 | A primary-down event while a backup is installed | the backup is used only for shortest-path-routed unicast (§4) and its use is bounded in time (§4.1) -- the next SPF reconverge replaces it; no permanent backup pinning |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables `fast-reroute` on an OSPF router with a triangle topology, then fails the primary link | config -> `Computer.Run` -> per-neighbour SPF -> LFA -> backup on `RouteEntry` -> `Installer` -> `locrib.Path` -> FIB backup -> kernel swings to the backup on link-down | `ospf-lfa-frr` QEMU interop (ping survives the failover) |
| 2 | Enables `fast-reroute` mode `ti-lfa` where no directly-connected LFA exists but SR is deployed | TI-LFA fallback -> post-convergence SPF -> repair list from ext-5 SR maps -> backup label stack -> FIB MPLS backup encap | `test/ospf/ospf-ti-lfa-compute.ci` + `ospf-ti-lfa-frr` QEMU interop |
| 3 | Runs `show ospf route fast-reroute` | CLI -> route snapshot -> render primary + backup + class + repair stack | `test/ospf/ospf-lfa-show.ci` |
| 4 | Enables fast-reroute on a multi-area router where a remote-area prefix is reachable via two alternate ABRs | §6.3 suppression -> that prefix gets no backup; intra-area prefixes still get backups | `test/ospf/ospf-lfa-multiarea.ci` + `ospf-multiarea-frr` failover |
| 5 | Leaves fast-reroute disabled | the LFA pass is skipped; the route set + install + `show ospf route` are byte-for-byte as today | existing OSPF suite green + `TestFastRerouteDisabledNoBackup` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPerNeighborSPFDistances` | `internal/plugins/ospf/spf/lfa_spf_test.go` | A-1: `Compute` re-rooted at N gives correct `D_opt(N,*)` | |
| `TestLoopFreeStrictInequality` | `internal/plugins/ospf/spf/lfa_select_test.go` | AC-1/AC-2, R-1: Inequality 1 strict `<`; equality rejected | |
| `TestDownstreamCriterionAgainstS` | `internal/plugins/ospf/spf/lfa_select_test.go` | AC-3, A-8, R-2: downstream test against `D_opt(S,D)` (Errata 2323) | |
| `TestNodeProtectionStrictInequality` | `internal/plugins/ospf/spf/lfa_select_test.go` | AC-4, R-8: Inequality 3 strict; equality not node-protecting | |
| `TestCostOverloadGate` | `internal/plugins/ospf/spf/lfa_select_test.go` | AC-5: LSInfinity cost/reverse-cost neighbour excluded (§3.5) | |
| `TestBroadcastPseudoNodeRule` | `internal/plugins/ospf/spf/lfa_select_test.go` | AC-6, A-11, R-9: Inequality 4 + S->N-avoids-PN on broadcast links | |
| `TestSelectionPreferenceOrder` | `internal/plugins/ospf/spf/lfa_select_test.go` | AC-7: §3.6 node+link > node > link; one LFA per primary | |
| `TestBackupPerPrimaryNextHop` | `internal/plugins/ospf/spf/lfa_select_test.go` | AC-10, A-7: per-primary backup, not folded into ECMP | |
| `TestLFAEligibilityByRouteType` | `internal/plugins/ospf/spf/lfa_select_test.go` | A-14: intra/inter/ext gating | |
| `TestPostConvergenceSPFExcludesResource` | `internal/plugins/ospf/spf/tilfa_test.go` | A-5: cloned graph drops the protected edge/vertex; SPF re-run correct | |
| `TestTILFAPQSpace` | `internal/plugins/ospf/spf/tilfa_test.go` | AC-8: P-space/Q-space P-node + Q-node selection | |
| `TestTILFARepairListFromSRMaps` | `internal/plugins/ospf/spf/tilfa_test.go` | AC-8/AC-9, A-4, R-3: repair list = Prefix-SID (SRGB-resolved) + Adj-SID, correct label form | |
| `TestLFASuppressedMultiAreaLeakage` | `internal/plugins/ospf/spf/lfa_multiarea_test.go` | AC-14, A-10, R-6: §6.3 suppression set | |
| `TestBackupOnlyChangeReinstalls` | `internal/plugins/ospf/spf/route_backup_test.go` | AC-12, R-11: `routeEqual`/`DiffRoutes` include the backup | |
| `TestRouteEntryBackupSnapshot` | `internal/plugins/ospf/spf/route_backup_test.go` | AC-15: `Snapshot` surfaces backup + class + repair stack | |
| `TestFastRerouteDisabledNoBackup` | `internal/plugins/ospf/spf/computer_lfa_test.go` | story 5: disabled -> route set identical to today | |
| `TestFastRerouteConfigEnablesLFAPass` | `internal/plugins/ospf/spf/computer_lfa_test.go` | wiring: config enables the pass in `Run` | |
| `TestLFAv6NextHopSelection` | `internal/plugins/ospf/spf/lfa_v6_test.go` | AC-16, A-9: v6 base LFA via the AF seam | |
| `TestLocribPathBackupCarriedNotArbitrated` | `internal/core/rib/locrib/candidate_test.go` | AC-13, A-2, R-10: backup carry-through, excluded from key/`Equal` | |
| `TestInstallerSetsBackupPath` | `internal/plugins/ospf/spf/install_backup_test.go` | AC-11: `Installer.insert` populates the `locrib.Path` backup | |
| `TestRichRouteBackupNextHop` | `internal/plugins/fib/kernel/richroute_test.go` | AC-11, A-3, R-7: `RichRoute` carries the backup; `buildRichRoute` emits the flagged backup + repair encap (Linux build) | |
| `TestBackupNotFoldedIntoECMP` | `internal/component/sysrib/sysrib_test.go` | AC-10, A-12, R-4: backup is not an ECMP sibling | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| LFA path cost (`D_opt`) | 0..LSInfinity-1 | `0x00fffffe` | N/A | `>= 0x00ffffff` rejected (§3.5) |
| Inequality 1 comparison | strict `<` | N-1 vs equality | equality NOT loop-free | N/A |
| Repair-list label (3-octet SR) | 0..0x0fffff (20 bits) | `0xfffff` | N/A | `> 0xfffff` invalid label |
| SRGB index (4-octet form) | 0..range-size-1 | range-1 | N/A | `>= range-size` out of range (ignored) |
| Repair-list depth | 0..max-stack | implementation cap | N/A | beyond the FIB MPLS stack cap rejected |
| Per-neighbour SPF count | 0..(neighbours) | all neighbours | N/A | capped by the §3.8 simplification under a configured limit |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-lfa-config` | `test/ospf/ospf-lfa-config.ci` | `fast-reroute { enable }` accepted; shows enabled in `show ospf` | |
| `ospf-lfa-compute` | `test/ospf/ospf-lfa-compute.ci` | a triangle topology yields a base LFA backup on the protected prefix | |
| `ospf-ti-lfa-compute` | `test/ospf/ospf-ti-lfa-compute.ci` | no base LFA + SR coverage -> a TI-LFA repair label stack | |
| `ospf-lfa-show` | `test/ospf/ospf-lfa-show.ci` | `show ospf route fast-reroute` lists primary + backup + class + repair stack | |
| `ospf-lfa-multiarea` | `test/ospf/ospf-lfa-multiarea.ci` | §6.3 suppression: a multi-ABR remote prefix gets no backup; intra-area prefixes do | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-lfa-frr` | `test/interop/scenarios/ospf-lfa-frr/` | FRR `ospfd` (LFA enabled) | Ze computes the same base LFA backup FRR does; on a primary-link-down the kernel fails over to the backup with no loop; ping survives the failure window | |
| `ospf-ti-lfa-frr` | `test/interop/scenarios/ospf-ti-lfa-frr/` | FRR `ospfd` + SR (segment-routing on) | Ze's TI-LFA repair label stack matches FRR's post-convergence path; the repaired packet carries the correct SR label(s) (captured + compared); kernel failover steers onto the post-convergence path | |

> Interop is required: although RFC 5286 adds no wire format, the SR Adj-SID
> B-Flag (RFC 8665 §6.1) and the repair-list label encoding are wire-observable,
> and the FIB failover behaviour must match FRR. The raw-IP / netlink backup-install
> paths are Linux-only and run as QEMU integration tests (`ai/rules/qemu-testing.md`),
> consistent with the rest of the OSPF interop set. `ospf6d` LFA interop is deferred
> with OSPFv3 SR (A-13); v6 base-LFA selection is unit-tested (`TestLFAv6NextHopSelection`).

### Future (if deferring any tests)
- OSPFv3 SR (RFC 8666) TI-LFA interop with `ospf6d` -- deferred with A-13 (needs RFC 8666 SR carriage). v6 base-LFA selection is covered now by `TestLFAv6NextHopSelection`.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*) -->
- `internal/plugins/ospf/spf/route.go` -- add the per-primary backup (next-hop + repair labels + protection class) to `RouteEntry`; include it in `routeEqual`/`DiffRoutes` and `Snapshot`/`RouteSnapshotEntry`
- `internal/plugins/ospf/spf/computer.go` -- run the LFA/TI-LFA pass in `Run` after `BuildRoutes` and before `Installer.Apply`, gated on the `fast-reroute` config; the per-neighbour-SPF orchestration under the same debounce/lock; the new compute-duration metric
- `internal/plugins/ospf/spf/install.go` -- `Installer.insert` populates the new `locrib.Path` backup field per primary next-hop; the nil-`loc` no-op preserved
- `internal/core/rib/locrib/candidate.go` -- add the carry-through backup field (backup next-hop + repair label stack) to `Path`, excluded from `Equal`/key (documented like `Labels`/`IsEBGP`)
- `internal/component/sysrib/sysrib.go` + `internal/component/sysrib/ecmp.go` -- forward the backup as a dedicated next-hop on the best-change, never via `ecmpCollect`
- `internal/component/sysrib/events/events.go` -- carry the backup next-hop (+ repair labels) on the FIB-install event alongside `ECMPPath`
- `internal/plugins/fib/kernel/richroute.go` -- add the backup next-hop (+ repair labels) to `RichRoute`
- `internal/plugins/fib/kernel/nexthop_linux.go` -- `buildRichRoute` emits the backup as a link-down/backup multipath next-hop with the repair MPLS encap (Linux-only)
- `internal/plugins/fib/kernel/fibkernel.go` -- thread the backup from the sysrib event into `RichRoute`
- `internal/plugins/ospf/config.go` -- resolve the `fast-reroute` container into the engine/`Computer` config
- `internal/plugins/ospf/register.go` -- thread `fast-reroute` config into the `Computer`; ensure the v6 engine instance gets the same LFA pass
- `internal/plugins/ospf/cmd_show.go` -- `show ospf route fast-reroute` render (primary + backup + class + repair stack)
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- the `fast-reroute` container (enable boolean, mode enum lfa|ti-lfa, node-protection-preference boolean; per-area/per-interface enable)
- `internal/plugins/ospf/yang/ze-ospf-cmd.yang` -- the `route fast-reroute` show subcommand

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- `fast-reroute` container; read `ai/rules/config-surface.md` + `ai/rules/config-naming.md` |
| YANG validation constraints | [ ] yes | `enable` boolean (native), `mode` enumeration {lfa, ti-lfa}, `node-protection` boolean; no bare `type string` |
| YANG custom validators | [ ] no | native boolean/enum suffice |
| CLI commands/flags | [ ] yes | `show ospf route fast-reroute` in `ze-ospf-cmd.yang` + `cmd_show.go` |
| CLI grammar (action before identifier) | [ ] yes | `ai/rules/cli-grammar.md` -- `show ospf route fast-reroute` |
| Editor autocomplete | [ ] yes | automatic for the YANG enum/boolean + the new show subcommand |
| Functional test for new RPC/API | [ ] yes | `test/ospf/ospf-lfa-*.ci` |
| Pipe completeness | [ ] yes | `show ospf route fast-reroute` routes through `ApplyPipes` like the other show outputs |
| Env var registration | [ ] no | `fast-reroute` is operational config, not an `environment/` leaf |
| Doctor check for runtime dependencies | [ ] no | no new socket/port/binary/cert; reuses the existing OSPF raw socket and FIB netlink (the FIB doctor already covers netlink) |
| Prometheus counters/metrics | [ ] yes | see the metrics rows below |

#### Metrics (new series owned by this spec)
| Metric | Type | Labels |
|--------|------|--------|
| `ze_ospf_fast_reroute_protected_prefixes` | gauge | `area`, `class` (link/node/downstream) |
| `ze_ospf_fast_reroute_unprotected_prefixes` | gauge | `area`, `reason` (no-lfa/no-sr/suppressed) |
| `ze_ospf_fast_reroute_backups_installed` | gauge | `kind` (lfa/ti-lfa) |
| `ze_ospf_fast_reroute_compute_seconds` | histogram | `area` |
| `ze_ospf_fast_reroute_ti_lfa_repair_labels` | gauge | `area` |

> These extend the umbrella's canonical OSPF metric set with the
> `ze_ospf_fast_reroute_*` prefix, registered by this spec's owner code. The
> umbrella "Metrics" table gains these rows when this spec lands.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` -- OSPF LFA / TI-LFA fast reroute |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` -- the `fast-reroute` container |
| 3 | CLI command added/changed? | [ ] yes | `docs/guide/command-reference.md` -- `show ospf route fast-reroute` |
| 4 | API/RPC added/changed? | [ ] no | the show RPC lives in the central `ze-show` namespace; documented under the command reference |
| 5 | Plugin added/changed? | [ ] yes | `docs/guide/plugins.md` -- OSPF gains fast-reroute compute + FIB backup install |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/ospf.md` -- a fast-reroute section (LFA + TI-LFA) |
| 7 | Wire format changed? | [ ] no | RFC 5286 adds no wire format; the Adj-SID B-Flag is ext-5's; note the dependency only |
| 8 | Plugin SDK/protocol changed? | [ ] yes | `ai/rules/plugin-design.md` / the FIB doc -- the `locrib.Path` backup field + the `RichRoute` backup next-hop are a cross-plugin contract change |
| 9 | RFC behavior implemented? | [ ] yes | `rfc/short/rfc5286.md` -- flip the LFA compliance items implemented; note TI-LFA draft coverage |
| 10 | Test infrastructure changed? | [ ] yes (interop scenarios added) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` -- OSPF LFA/TI-LFA parity with FRR |
| 12 | Internal architecture changed? | [ ] yes | the OSPF subsystem doc + the FIB/sysrib doc -- per-neighbour SPF, backup carry-through, FIB backup install |
| 13 | Route metadata keys added/changed? | [ ] yes | `docs/architecture/meta/README.md` -- the backup-next-hop / repair-list carry-through on `locrib.Path` |
| 14 | Prometheus counters added/changed? | [ ] yes | the OSPF telemetry doc -- the five `ze_ospf_fast_reroute_*` series |
| 15 | Registered plugin/event/command/capability inventory changed? | [ ] yes | `docs/plugin-overview.md` + the umbrella metrics table (the new show command + metrics) |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors into `spf/route.go`, `spf/install.go`, `locrib/candidate.go`, `fib/kernel/*` |
| 17 | Existing docs show examples for this area? | [ ] check | verify OSPF route / FIB examples against the new backup field |

## Files to Create
- `internal/plugins/ospf/spf/lfa.go` -- the base LFA selection engine: per-neighbour-SPF orchestration, the §3.1/§1.1/§3.2/§3.3 inequalities, the §3.5 cost/overload gate, the §3.6 selection order, the §6.1 multi-homed transform
- `internal/plugins/ospf/spf/tilfa.go` -- the TI-LFA repair-list builder: post-convergence graph clone, P-space/Q-space, repair-list assembly from ext-5 SR maps (Prefix-SID + Adj-SID, SRGB resolution)
- `internal/plugins/ospf/spf/lfa_multiarea.go` -- the §6.3 OSPF multi-area LFA suppression rules
- `internal/plugins/ospf/spf/lfa_spf_test.go`, `lfa_select_test.go`, `tilfa_test.go`, `lfa_multiarea_test.go`, `route_backup_test.go`, `computer_lfa_test.go`, `install_backup_test.go`, `lfa_v6_test.go`
- `internal/core/rib/locrib/candidate_test.go` -- (extend if present) `TestLocribPathBackupCarriedNotArbitrated`
- `internal/plugins/fib/kernel/richroute_test.go` -- (extend if present) `TestRichRouteBackupNextHop`
- `test/ospf/ospf-lfa-config.ci`, `ospf-lfa-compute.ci`, `ospf-ti-lfa-compute.ci`, `ospf-lfa-show.ci`, `ospf-lfa-multiarea.ci`
- `test/interop/scenarios/ospf-lfa-frr/` -- `ze.conf`, `frr.conf`, `check.py`
- `test/interop/scenarios/ospf-ti-lfa-frr/` -- `ze.conf`, `frr.conf`, `check.py`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan -- confirm `Compute` re-roots, `locrib.Path` has no backup field, `RichRoute` has no backup |
| 3. Wiring phase | Wiring Test table -- config -> `Computer.Run` LFA pass + failing wiring tests |
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

1. **Phase: Wiring (MANDATORY FIRST)** -- config seam + the LFA pass hook + failing wiring tests
   - Tests: `TestFastRerouteConfigEnablesLFAPass`, `test/ospf/ospf-lfa-config.ci`
   - Files: `yang/ze-ospf-conf.yang` (`fast-reroute` container), `config.go` (resolve), `register.go` (thread into `Computer`), `computer.go` (call a stub LFA pass in `Run` when enabled), `spf/lfa.go` (stub returning no backup)
   - Verify: enabling the leaf calls the LFA pass; the stub returns no backup so the deeper tests still fail
2. **Phase: Carry-through plumbing** -- `RouteEntry` backup + `locrib.Path` backup + diff/snapshot
   - Tests: `TestRouteEntryBackupSnapshot`, `TestBackupOnlyChangeReinstalls`, `TestLocribPathBackupCarriedNotArbitrated`, `TestInstallerSetsBackupPath`
   - Files: `spf/route.go`, `spf/install.go`, `locrib/candidate.go`
   - Verify: a backup attaches to `RouteEntry`, survives diff, sets the `locrib.Path` field, and does NOT change arbitration
3. **Phase: Per-neighbour SPF + base LFA selection** -- the RFC 5286 math
   - Tests: `TestPerNeighborSPFDistances`, `TestLoopFreeStrictInequality`, `TestDownstreamCriterionAgainstS`, `TestNodeProtectionStrictInequality`, `TestCostOverloadGate`, `TestBroadcastPseudoNodeRule`, `TestSelectionPreferenceOrder`, `TestBackupPerPrimaryNextHop`, `TestLFAEligibilityByRouteType`, `ospf-lfa-compute.ci`
   - Files: `spf/lfa.go`
   - Verify: every inequality + the §3.6 order + the §3.5 gate + the broadcast PN rule hold; per-primary backups attach
4. **Phase: Multi-area suppression** -- §6.3
   - Tests: `TestLFASuppressedMultiAreaLeakage`, `ospf-lfa-multiarea.ci`
   - Files: `spf/lfa_multiarea.go`, wired into `spf/lfa.go`
   - Verify: inter-area/external prefixes in leakage topologies get no backup
5. **Phase: TI-LFA repair list** -- post-convergence + SR labels
   - Tests: `TestPostConvergenceSPFExcludesResource`, `TestTILFAPQSpace`, `TestTILFARepairListFromSRMaps`, `ospf-ti-lfa-compute.ci`
   - Files: `spf/tilfa.go`, `spf/graph.go` (clone/exclude helper)
   - Verify: post-convergence SPF correct; P/Q-space repair list built from ext-5 maps with the right label form
6. **Phase: FIB backup install** -- sysrib forward + Linux netlink
   - Tests: `TestBackupNotFoldedIntoECMP`, `TestRichRouteBackupNextHop`, `ospf-lfa-frr` QEMU
   - Files: `sysrib/sysrib.go`, `sysrib/ecmp.go`, `sysrib/events/events.go`, `fib/kernel/richroute.go`, `fib/kernel/nexthop_linux.go`, `fib/kernel/fibkernel.go`
   - Verify: the backup reaches the kernel as a flagged backup next-hop with the repair encap; kernel fails over under QEMU
7. **Phase: CLI + metrics + v6** -- user surface
   - Tests: `ospf-lfa-show.ci`, `TestLFAv6NextHopSelection`, `TestFastRerouteDisabledNoBackup`
   - Files: `cmd_show.go`, `yang/ze-ospf-cmd.yang`, metric registration in `computer.go`/`install.go`, the v6 engine wiring in `register.go`
   - Verify: `show ospf route fast-reroute`; the five metric series; v6 base LFA; disabled-path identical to today
8. **Functional tests** -> the five `.ci` cover the user-visible behaviour
9. **RFC refs** -> add `// RFC 5286 Section X` (inequalities, §3.5, §3.6, §4.1) and `// RFC 8665 Section 5/6.1` (repair-list SID forms) comments on the enforcing code
10. **Interop** -> `ospf-lfa-frr` + `ospf-ti-lfa-frr` QEMU scenarios
11. **Full verification** -> `make ze-verify`
12. **Complete spec** -> audit tables + learned summary; two commits (A: code+spec+learned, B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has file:line implementation |
| Feature completeness | each user story has a working path; base-LFA + TI-LFA parity with FRR's `ospf_ti_lfa.c` (loop-free backup + SR repair list + FIB install) |
| Correctness | Inequality 1/3 STRICT `<`; Errata 2323 downstream against `D_opt(S,D)`; §3.5 LSInfinity gate; §3.3 broadcast PN rule; §3.6 selection order; repair-list SID label form (3-octet 20-bit); §6.3 suppression |
| Naming | `ze_ospf_fast_reroute_*` metrics; YANG `fast-reroute` kebab-case; protection class names (link/node/downstream) |
| Data flow | backup flows SPF -> `RouteEntry` -> `Installer` -> `locrib.Path` (carry-through) -> sysrib (non-ECMP) -> FIB; ext-5 read-only; no LSDB/codec change |
| CLI grammar | `show ospf route fast-reroute` action-before-identifier |
| Doctor checks | none added (reuses existing OSPF socket + FIB netlink doctor) -- confirm |
| YANG validation | `enable` boolean, `mode` enum, `node-protection` boolean; no bare string |
| Prometheus counters | the five series defined, registered, listed; umbrella table updated |
| Rule: plugin-self-containment | the `locrib.Path` backup field + FIB backup path are generic (no OSPF spelling); removing OSPF leaves them unused but valid |
| Rule: buffer-first | repair label stack built once and shared into `locrib.Path` like `Labels`; `show` render buffer-first |
| Rule: qemu-testing | the FIB backup-install + kernel failover are QEMU integration tests, not skipped |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Base LFA selection | `go test ./internal/plugins/ospf/spf -run 'LFA'` |
| TI-LFA repair list | `go test ./internal/plugins/ospf/spf -run 'TILFA'` |
| `locrib.Path` backup carry-through | `go test ./internal/core/rib/locrib -run Backup` + `grep -n backup internal/core/rib/locrib/candidate.go` |
| FIB backup next-hop | `go test ./internal/plugins/fib/kernel -run Backup` (Linux) |
| Backup not folded into ECMP | `go test ./internal/component/sysrib -run Backup` |
| `fast-reroute` config + CLI | `grep -n fast-reroute internal/plugins/ospf/yang/*.yang`; `ls test/ospf/ospf-lfa-*.ci` |
| Five metric series registered | `grep -rn 'ze_ospf_fast_reroute_' internal/plugins/ospf` |
| Interop scenarios present | `ls test/interop/scenarios/ospf-lfa-frr/ test/interop/scenarios/ospf-ti-lfa-frr/` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | the SR labels feeding the repair list come from ext-5's already-validated maps; ext-6 re-checks label/index range before pushing (no out-of-range MPLS label) |
| Resource exhaustion | per-neighbour SPF count is bounded (the §3.8 simplification cap); the repair-list depth is capped to the FIB MPLS stack limit; a crafted topology cannot make SPF run unbounded |
| Forwarding correctness | a wrong backup can misroute traffic on failover; strict inequalities + interop label-capture compare guard against installing a looping/mis-labelled backup |
| Trust boundary | LFA/TI-LFA consume LSDB + SR state already authenticated by OSPF; no new untrusted input or auth surface |
| Error leakage | LFA-compute failures degrade the prefix to unprotected (counted in `ze_ospf_fast_reroute_unprotected_prefixes{reason}`), never crash the engine or surface to peers |

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
TI-LFA in Ze is a *compute + carry-through + install* feature, not a wire feature:
RFC 5286 defines no packet, the per-neighbour SPF is the existing Dijkstra re-rooted
at each neighbour, and the post-convergence SPF is the same Dijkstra over a graph
clone with the protected resource removed. The only new plumbing in the shared core
is one carry-through backup field on `locrib.Path` (modelled exactly like `Labels`)
and a Linux FIB backup-next-hop programming path. Everything else lives in `spf/`
and reads ext-5's SR label maps; the LSDB, flooding, and codec are untouched.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Reuse `Compute` re-rooted at each neighbour for `SPT(N)` | a bespoke reverse-SPF / inward-SPF | `computeWithNextHop` already takes an arbitrary root; `D_opt(N,*)` falls straight out of `Result.Nodes[*].Metric` with no new graph code |
| Backup as a carry-through field on `locrib.Path` (excluded from arbitration) | a second `Path`, or an out-of-band backup table | matches the existing `Labels`/`IsEBGP` carry-through contract; keeps AdminDistance-then-Metric arbitration intact and the install seam single |
| Backup is per-primary-next-hop, never an ECMP sibling | a single per-route backup; folding into ECMP | RFC 5286 §3.6/§3.8 require per-primary protection; a backup is strictly worse-cost and must not load-share, so it cannot ride the ECMP path |
| Post-convergence SPF via a graph clone with the resource removed | incremental SPF / LFA-only | reuses the verbatim Dijkstra; correctness over micro-optimisation for the first implementation; A-6 tracks the perf budget |
| SR repair labels read from ext-5's resolved maps | re-parse the SR TLVs in ext-6 | plugin-self-containment + the umbrella contract: ext-5 owns SR decode/resolution; ext-6 consumes labels |
| §6.3 multi-area LFA suppression rather than attempting correct multi-area LFA | full multi-area LFA computation | the local-area SPF cannot see the real inter-area path; RFC 5286 §6.3 explicitly says to suppress; correctness first |

## Known Limitations
- Micro-loop-avoidance timers and ordered-FIB convergence (RFC 5286 §4.1 alternatives) are not implemented; only the basic §4.1(a/b/c) termination is used.
- Non-SR remote-LFA tunnels (RFC 7490) are not implemented; a prefix with no LFA and no SR coverage is left unprotected.
- SRLG-protecting alternates (RFC 5286 §3) are not implemented (no local-SRLG config); the §3.6 SRLG steps degrade to "no SRLG info."
- OSPFv3 gets base-LFA next-hop selection but no SR repair labels (RFC 8666 SR carriage is a separate spec, A-13).
- BGP recursive next-hop inheritance of the IGP alternate (§6.4) is a sysrib concern, not implemented here (A-12).

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code:
- RFC 5286 §3.1 Inequality 1 (strict loop-free) above the loop-free gate
- RFC 5286 §1.1 / §3.6 step 16 + Errata 2323 (downstream against `D_opt(S,D)`) above the downstream classifier
- RFC 5286 §3.2 Inequality 3 (strict node-protecting) above the node-protection classifier
- RFC 5286 §3.5 (LSInfinity cost/reverse-cost/overload gate) above the candidate filter
- RFC 5286 §3.3 Inequality 4 (broadcast pseudo-node) above the broadcast-link path
- RFC 5286 §3.6 (selection order) above the selection loop
- RFC 5286 §4 / §4.1 (use only for shortest-path traffic; bounded use) above the install/terminate path
- RFC 5286 §6.3 (OSPF multi-area suppression) above the suppression rule
- RFC 8665 §5 / §6.1 (Prefix-SID / Adj-SID label form) above the repair-list assembly

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
| Base LFA loop-free backup per RFC 5286 | unit + interop | `TestLoopFreeStrictInequality`, `ospf-lfa-frr` |
| TI-LFA SR repair list from P/Q-space | unit + interop | `TestTILFARepairListFromSRMaps`, `ospf-ti-lfa-frr` |
| Primary + backup programmed into the FIB | interop (QEMU) | `ospf-lfa-frr` kernel failover (ping survives) |
| Per-prefix protection (not shared per-next-hop) | unit | `TestBackupPerPrimaryNextHop` |
| `show ospf route fast-reroute` CLI | functional | `ospf-lfa-show.ci` |
| §6.3 multi-area suppression | unit + functional | `TestLFASuppressedMultiAreaLeakage`, `ospf-lfa-multiarea.ci` |

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
- [ ] Feature code integrated (`internal/plugins/ospf/*`, `internal/core/rib/locrib/*`, `internal/plugins/fib/kernel/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass)
- [ ] RFC 5286 + RFC 8665 constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (LFA + TI-LFA are both needed; the carry-through field serves the single install seam)
- [ ] No speculative features (no SRLG, no rLFA, no micro-loop timers -- all out of scope)
- [ ] Single responsibility per component (`lfa.go` selection, `tilfa.go` repair list, suppression in `lfa_multiarea.go`)
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (ext-6 reads ext-5 maps + delivered SPF; carrier field generic)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (`ospf-lfa-frr`, `ospf-ti-lfa-frr`)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ospf-ext-6-ti-lfa.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospf-ext-6-ti-lfa.md`
