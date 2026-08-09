# OSPF inter-area routing and the ABR

ABR detection, Type 3 network and Type 4 ASBR Summary-LSA origination into each
attached area, the RFC 2328 Section 16.2 and 16.3 inter-area computation, and
area ranges.

## Decisions

- **Inter-area is a producer and consumer pair over the ABR's own Section 16.1
  route table, not a new graph algorithm.** The ABR's intra-area costs ARE both
  the Type 3 and Type 4 metrics it advertises and the cost-to-ABR input to
  Section 16.2. Inter-area candidates are appended to the existing route table,
  `selectBestRoutes` resolves intra above inter, and one `locrib.Path` per
  prefix is published at AdminDistance 110.
  <!-- source: internal/plugins/ospf/spf/interarea.go -- ComputeInterArea, IsABR -->
  <!-- source: internal/plugins/ospf/spf/summary.go -- collectSummaryNetworks -->
- **Summary LSAs go into the shared LSDB store and reuse the Section 13 flooding
  and the MaxAge walker.** A summary is withdrawn by re-originating at MaxAge,
  never by a local delete, so neighbours purge consistently.
- **A backbone-only acceptance rule at an ABR is the one load-bearing
  loop-freedom rule (Section 16.3).** Border-router reachability is collected
  across all areas, and inter-area routes are computed ONLY from backbone
  (area 0) summaries.

## Traps

- **A transit or broadcast LAN subnet lives only in the Network-LSA.** Deriving
  networks from Router-LSA stub links alone drops it from the route table and
  from every Type 3. RFC 2328 Section 16.1 step 4 installs a route per transit
  vertex, with the LS ID masked by the Network-LSA mask. The asymmetry matters:
  for INSTALL, skip a directly-connected LAN, whose next-hop is empty and whose
  route the connected source owns. For SUMMARY, include it, because an ABR
  summarizes its own LANs.
- **LS-ID collision resolution increments until free. It does not implement RFC
  2328 Appendix E host bits.** The result is always collision-free, which is the
  property Section 12.4.3 requires, and a wire LS-ID can differ from FRR in a
  multi-collision case. This is a known limitation.
- **Verify a numeric claim against the constant before acting on it.** A review
  reported `LSInfinity = 65535`. It is `0x00ffffff`. The whole finding was built
  on the misread value.
- **A count-and-identity assertion is a weak test.** Several spec-9 tests
  checked the count and the identity but not the load-bearing field, so they
  passed with a corrupted metric, next-hop or LS-ID binding.
