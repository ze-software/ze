# 1051 - OSPF TI-LFA / LFA Fast Reroute (RFC 5286 + TI-LFA draft + RFC 8665)

## Context

Pre-computed loop-free backup next-hops for OSPF so a single local failure repairs
locally before the IGP reconverges. Two tiers: base LFA (RFC 5286, a
directly-connected loop-free alternate) and TI-LFA (an explicit SR repair label
stack along the post-convergence path when no base LFA exists). This is a
COMPUTE + CARRY-THROUGH + INSTALL feature, not a wire feature: RFC 5286 defines no
packet; the only wire signal (the Adj-SID B-Flag) is ext-5's. Both address
families get base-LFA next-hop selection through the AF seam; SR repair labels are
IPv4-only (OSPFv3 SR carriage, RFC 8666, is out of scope / A-13). Depends on ext-5
(SR label maps).

## Decisions

- Per-neighbour SPF (`SPT(N)`) is the existing `computeWithNextHop` re-rooted at
  each neighbour; `D_opt(N,*) = Result.Nodes[*].Metric`. No new graph code.
- Base LFA: strict `<` Inequality 1 (loop-free) and Inequality 3 (node-protect);
  downstream measured against `D_opt(S,D)` (Errata 2323, NOT the alternate's
  neighbour); §3.5 LSInfinity gate; §3.3 broadcast pseudo-node (Inequality 4 +
  S->N-avoids-PN); §3.6 order (node+link > node > link).
- §6.3 multi-area suppression keyed off REAL ABR/ASBR counts + virtual-link state
  (not a stub), so inter-area/external prefixes in leakage topologies get no backup.
- TI-LFA: post-convergence SPF via a graph CLONE with the protected edge/vertex
  removed (`Graph.Clone`/`excludeRouter`/`excludeLink`); P-space/Q-space; Case A
  (single PQ node -> its Prefix-SID) / Case B (disjoint -> Prefix-SID(P) +
  Adj-SID(P->q) + Prefix-SID(dest)). SR labels read from ext-5's resolved maps.
- Carry-through: `RouteEntry.Backups` is PER-PRIMARY (parallel to NextHops), never
  an ECMP sibling; `locrib.Path` gains `BackupNextHop`/`BackupRepairLabels`;
  `sysrib` forwards it via a dedicated `backupPaths` list (never `ecmpCollect`);
  `fib/kernel` programs an RTNH_F_LINKDOWN backup next-hop with the repair MPLS encap.

## Consequences

- `locrib.Path.Equal` INCLUDES the backup fields (a backup-only change must
  re-install so the kernel reprograms, AC-12) but `key()`/`selectBest` EXCLUDE them
  (arbitration stays AdminDistance-then-Metric). `Equal` is used only for
  change-detection (`manager.go`), never in arbitration, so BGP/IS-IS best-path is
  unaffected. This is the correct reading of the spec's "excluded from key/Equal"
  (the real R-10 risk is arbitration, not change-detection).
- ext-5's `spf.NextHop.Router` (the first-hop router id) is what LFA keys the
  primary's protected neighbour E on.

## Gotchas

- LANDMINE (TI-LFA Adj-SID reachability): the intuitive "S is its own P-node, push
  S's local Adj-SID" repair is DEAD CODE. Theorem: any q adjacent to S AND in
  Q-space already satisfies base-LFA Inequality 1 (E is a directly-connected
  primary, so `D_opt(S,D) >= D_opt(S,E)+D_opt(E,D)`; the triangle inequality chains
  to `D_opt(q,D) < D_opt(q,S)+D_opt(S,D)`), so base LFA preempts it and TI-LFA never
  runs. The only case p==S forms is a ONE-WAY link, whose next-hop is unresolvable.
  Therefore EVERY reachable TI-LFA Adj-SID repair is a REMOTE-node Adj-SID (p != S),
  and AC-9 REQUIRES decoding remote routers' Extended Link Adj-SIDs
  (`(*engine).srRemoteAdjSID` scans Type-8 Ext-Link LSAs, matches the P2P Link ID or
  the LAN-Adj-SID Neighbor ID, returns the advertising router's absolute RFC 8665
  §6.1 local label verbatim). Do NOT write the p==S shortcut (wiring-completeness
  violation).
- LANDMINE (misleading fake): AC-9's first test passed only because its fake
  resolver returned a remote Adj-SID for `from != self` -- the OPPOSITE of the
  production resolver (which returned false for remote). A fake that inverts the
  production contract hides the whole gap. Test the AC via the REAL resolver over a
  real LSDB (flood the remote Ext-Link LSA through `ReceiveUpdate`), with negative
  guards (wrong neighbour must NOT resolve; index-form Adj-SID must NOT resolve).
- §3.5 LSInfinity gate is dead defensive code: `types.Metric` is uint16, so a
  P2P/transit adjacency cost can never reach `LSInfinity` (0x00ffffff) on real
  OSPFv2 input. Harmless; a costed-out reverse link is pre-excluded by the
  two-way-adjacency check.
- §3.6 rule-4 "prefer a primary alternate" is inert (no `prefer-primary` YANG leaf);
  no AC impact.
- QEMU-deferred (darwin cannot run): the RTNH_F_LINKDOWN kernel failover and the SR
  label-capture interop; the LFA/TI-LFA math is fully unit-proven.

## Files

- `internal/plugins/ospf/spf/{lfa,tilfa,lfa_multiarea}.go` + `spf/{route,graph,computer,install}.go` (+ tests)
- `internal/plugins/ospf/sr_tilfa.go` (SRResolver: PrefixSIDLabel + local/remote AdjSIDLabel + srRemoteAdjSID) (+ sr_tilfa_test.go)
- `internal/core/rib/locrib/candidate.go` (Backup carry-through, Equal vs key)
- `internal/component/sysrib/{sysrib,ecmp}.go` + `sysrib/events/events.go` + `bgp/plugins/rib/events/events.go` (non-ECMP backup forward)
- `internal/plugins/fib/kernel/{richroute,fibkernel,nexthop_linux}.go` (RTNH_F_LINKDOWN backup + repair encap)
- `internal/plugins/ospf/{config,spf_wiring,register,cmd_show,instance_snapshots}.go`, `yang/ze-ospf-{conf,cmd}.yang`
- `test/ospf/ospf-lfa-*.ci`, `ospf-ti-lfa-compute.ci`, `test/interop/scenarios/{ospf-lfa-frr,ospf-ti-lfa-frr}/`
- `docs/guide/ospf.md`, `command-reference.md`, `configuration.md`, `features.md`
