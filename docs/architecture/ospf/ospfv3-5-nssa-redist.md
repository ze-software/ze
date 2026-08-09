# OSPFv3 NSSA externals

RFC 3101 NSSA behaviour for IPv6: originate Type-7 NSSA-LSAs inside the NSSA
with the correct P-bit and forwarding address, and translate them to Type-5 at
the elected NSSA ABR. Before this, the v6 path always originated Type-5
AS-External LSAs, which an NSSA blocks, so an OSPFv3 ASBR inside an NSSA could
inject nothing.

## Decisions

- **Reuse the OSPFv2 NSSA policy and vary only the wire encode.** The translator
  election, the P-bit boundary rule and the source preference are
  address-family independent (see `ospf-11-stub-nssa.md`). A parallel v6 NSSA
  engine was rejected.
  <!-- source: internal/plugins/ospf/origination_v6_nssa.go -- externalScopeV6 -->
  <!-- source: internal/plugins/ospf/nssa.go -- translateNSSAV6 -->
- **The v6 NSSA-LSA body reuses the AS-External encoder.** RFC 5340 Appendix
  A.4.8 makes the two bodies byte-identical. They differ in LS Type and flooding
  scope only, so no separate NSSA write path exists.
  <!-- source: internal/plugins/ospf/v3/packet/lsa_nssa.go -- NSSAPropagate -->
- **The P-bit rides in the prefix options, not a header Options bit**, per
  OSPFv3.
- **The scope decision lives in the engine**, so the redistribution framework
  stays address-family generic. An ASBR in an NSSA injects Type-7, and a
  normal-area ASBR keeps Type-5 AS-wide.
  <!-- source: internal/plugins/ospf/origination_v6_external.go -- v6InjectExternal -->

## Traps

- **Redistribution and translation race over the same keep-set.** The v6
  translation snapshots the redistributed set and then flushes stale self-LSAs,
  so a just-injected Type-5 must already be in that set. Injection and
  translation share the NSSA mutex. Lock order: NSSA mutex, then engine mutex.
- **A non-candidate ABR with the Nt-bit clear must not wedge translation off**
  for a willing lower-Router-ID candidate.
- The peer rejected redistributed OSPFv3 routes until Ze drained Loading to Full
  and advertised a reachable Router-LSA. The fix belonged in the Link-LSA, DD
  and LS Request work, not in faking Router-LSA links.
