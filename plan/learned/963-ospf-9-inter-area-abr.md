# 963 - OSPFv2 inter-area routing and ABR

## Context

Completed `plan/spec-ospf-9-inter-area-abr.md`: ABR detection, Type 3 (network) and Type 4 (ASBR) Summary-LSA origination into each attached area, RFC 2328 §16.2/§16.3 inter-area route computation, area ranges (aggregate/not-advertise), `show ospf border-routers`, and the `ze_ospf_abr` / `ze_ospf_summary_lsas{area}` metrics. Implementation was inherited code-complete; this session ran the `/ze-review` gate (3 parallel review agents + maintainer reads), fixed the findings, and closed the spec.

## Decisions

- Inter-area is a producer/consumer pair over the ABR's own §16.1 route table, not a new graph algorithm: the ABR's intra-area costs ARE both the Type 3/4 metrics it advertises and the cost-to-ABR input to §16.2. Append inter-area candidates to the existing route table; `selectBestRoutes` resolves intra > inter; one `locrib.Path` per prefix, AdminDistance 110.
- Hand Type 3/4 LSAs to the ospf-7 LSDB store and reuse §13 flooding + the MaxAge walker; withdraw a summary by re-originating at MaxAge, never by local delete (so neighbours purge consistently).
- Trap #8 (§16.3) backbone-only acceptance at an ABR is the one load-bearing loop-freedom rule: collect border-router reachability across all areas, but compute inter-area routes ONLY from backbone (area 0) summaries.

## Gotchas

- **Transit/broadcast-LAN subnets were silently dropped (latent spec-8 gap).** Both `spf/route.go BuildRoutes` and `spf/summary.go collectSummaryNetworks` derived networks from Router-LSA stub links ONLY, never from `VertexNetwork` (Network-LSA) vertices. A remote broadcast LAN advertises transit links (the subnet lives only in the Network-LSA), so its prefix reached no route table and no Type 3. RFC 2328 §16.1 step (4) requires installing a route per transit vertex (LS ID masked with the Network-LSA mask). Fix: emit/collect transit-network routes. Asymmetry that matters: for INSTALL, skip directly-connected LANs (empty SPF next-hop -> owned by the connected source); for SUMMARY, INCLUDE them (an ABR summarizes its own LANs). The root's directly-connected network vertex IS present in `res.Nodes` with metric = interface cost (confirmed by `TestOSPFTransitNetworkSPF`).
- **LS-ID collision uses increment-until-free, not RFC 2328 Appendix E host-bits.** `nextFreeLSID` only checks the already-assigned `used` set, so a bumped entry can steal a later entry's natural LS ID, forcing it to bump too. Output is always collision-free (the safety property §12.4.3 cares about), but wire LS-IDs can differ from FRR in multi-collision cases. Left as a documented Known Limitation; FRR interop (ospf-13) will confirm or motivate Appendix E.
- **Verify agent claims against source.** A review agent reported `LSInfinity = 65535`; it is `0x00ff_ffff` (16777215, RFC-correct). The whole "metric ceiling mismatch" finding was a false positive built on a misread constant. Always grep the constant before acting on a numeric finding.
- **"WEAK test" is the local form of the count-only defect.** Several spec-9 tests checked count + identity but not the load-bearing field (snapshot metric/next-hop, body<->LS-ID binding, Type 4 LS ID/TOS). They pass even if that field is corrupted. Strengthened to assert the values.

## Verification anchors

- `TestOSPFTransitNetworkRoute` - a remote /25 broadcast LAN is installed as an intra-area route via the next-hop router (RFC 2328 §16.1(4)); a /25 also proves the prefix length comes from the Network-LSA, not a hard-coded /24.
- `TestOSPFTransitNetworkSummary` - an ABR's own LAN subnet is summarized into the backbone as a Type 3.
- `TestOSPFInterAreaLSInfinityDropped` - a composed cost saturating at LSInfinity installs no route (no wrap).
- `TestOSPFABRBackboneOnlyAcceptance` - trap #8 loop-freedom: a cheaper non-backbone summary is ignored at an ABR.
- `TestOSPFSummaryLSIDCollision` - body<->LS-ID binding after host-bit/increment disambiguation.
- FRR `ospfd` multi-area interop, metric scrape, central `ze-show` RPC, and tab-completion remain owned by spec-ospf-13.

## Files

None recorded.
