# OSPF LFA and TI-LFA fast reroute

Pre-computed loop-free backup next-hops (RFC 5286) and, where no base
alternate exists, a Segment Routing repair label stack along the
post-convergence path. This is a COMPUTE, CARRY-THROUGH and INSTALL feature. RFC
5286 defines no packet.

## Decisions

- **The per-neighbour shortest-path tree is the existing computation re-rooted
  at each neighbour.** No new graph code was written.
  <!-- source: internal/plugins/ospf/spf/lfa.go -- FastRerouteConfig, SRResolver -->
  <!-- source: internal/plugins/ospf/spf/computer.go -- Computer -->
- **Base LFA applies the RFC inequalities strictly**: strict loop-free and
  node-protecting comparisons, downstream measured against the SOURCE optimum
  (Errata 2323, not the alternate's neighbour), the LSInfinity gate, the
  broadcast pseudo-node rules, and the Section 3.6 preference order of
  node-and-link, then node, then link.
- **Multi-area suppression keys off the REAL ABR and ASBR counts and the
  virtual-link state**, not a stub value, so an inter-area or external prefix in
  a leakage topology gets no backup.
  <!-- source: internal/plugins/ospf/spf/lfa_multiarea.go -- suppressLFA -->
- **TI-LFA computes the post-convergence tree on a graph CLONE with the
  protected edge or vertex removed**, then derives P-space and Q-space. A single
  PQ node gives its Prefix-SID. A disjoint pair gives Prefix-SID(P),
  Adj-SID(P to q) and Prefix-SID(destination).
  <!-- source: internal/plugins/ospf/spf/tilfa.go -- buildTILFA, tilfaBackup -->
  <!-- source: internal/plugins/ospf/spf/graph.go -- Graph -->
- **A backup is PER-PRIMARY, never an ECMP sibling.** The route entry carries
  backups parallel to next-hops, the Loc-RIB path carries the backup next-hop
  and repair labels, sysrib forwards them on a dedicated list, and the kernel
  programs a link-down backup next-hop with the repair encapsulation.
  <!-- source: internal/plugins/ospf/spf/route.go -- RouteEntry -->
  <!-- source: internal/plugins/ospf/sr_tilfa.go -- srRemoteAdjSID -->

## Constraints on callers

- **The Loc-RIB path equality INCLUDES the backup fields, and the key and
  best-path selection EXCLUDE them.** A backup-only change must re-install so
  the kernel reprograms, while arbitration stays admin-distance then metric.
  Equality is used only for change detection, never in arbitration, so other
  protocols are unaffected.

## Traps

- **"S is its own P-node, push a local Adj-SID" is DEAD CODE.** Any q adjacent
  to S and in Q-space already satisfies the base-LFA inequality, because E is a
  directly connected primary and the triangle inequality chains through, so base
  LFA preempts it and TI-LFA never runs. The only case where p equals S is a
  one-way link, whose next-hop cannot be resolved. **Every reachable TI-LFA
  Adj-SID repair is therefore a REMOTE Adj-SID**, which requires decoding the
  Extended Link Adj-SIDs of other routers. Do not write the local shortcut.
- **A fake that inverts the production contract hides the whole gap.** The first
  remote-Adj-SID test passed because its fake resolver returned a value for a
  remote router while the production resolver returned none. Drive such a test
  through the REAL resolver over a real LSDB, with negative guards.
- The LSInfinity gate is dead defensive code: the metric type is 16-bit, so a
  point-to-point or transit adjacency cost cannot reach the 24-bit LSInfinity on
  real OSPFv2 input. A costed-out reverse link is already excluded by the
  two-way check.
- The Section 3.6 "prefer a primary alternate" rule is inert, because no config
  leaf selects it.
