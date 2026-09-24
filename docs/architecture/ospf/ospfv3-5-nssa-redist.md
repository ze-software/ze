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
  The same per-source `nssa-propagate` leaf controls it for IPv6 and the
  RFC 5838 IPv4 address families. Its default is false. If propagation is
  requested without a usable forwarding address, no LSA is originated and an
  existing LSA is withdrawn. Clearing P and advertising the route locally would
  discard the source's propagation policy (RFC 3101 Section 2.3).
  <!-- source: internal/plugins/ospf/redist_wiring.go -- externalPropagate -->
  <!-- source: internal/plugins/ospf/origination_v6_nssa.go -- v6OriginateNSSALSA -->
- **The NSSA default route reuses the same producer, at a reserved Link State
  ID.** RFC 5340 Section 4.4.3.7 maps RFC 3101 Section 2.4 onto OSPFv3
  unchanged, so the default is an ordinary NSSA-LSA with a zero-length prefix.
  It takes LSID 0, which redistribution can never allocate because
  `v6InjectExternal` pre-increments its counter, and its key joins the
  withdrawal keep-set so an unrelated redistribution withdrawal cannot sweep it.
  <!-- source: internal/plugins/ospf/origination_v6_nssa.go -- v6OriginateNSSADefault, v6NSSADefaultLSID -->
  Default injection and withdrawal validate the prefix against the engine's
  address family. IPv4 unicast and multicast accept `0.0.0.0/0` with the OSPFv3
  codec; IPv6 families accept `::/0`.
  <!-- source: internal/plugins/ospf/default.go -- defaultRoute, checkDefaultFamily -->
  <!-- source: internal/plugins/ospf/origination_v6_external.go -- v6WithdrawExternal -->
- **One forwarding-address seam serves both families.** RFC 3101 Section 2.4
  makes a usable forwarding address a precondition for a P-set Type 7 LSA, so a
  unit test that cannot supply one cannot reach any rule that comes after it.
  The engine holds a single nil-by-default lookup that both the OSPFv2 and the
  OSPFv3 helper consult; production leaves it nil and reads the live interface.
  <!-- source: internal/plugins/ospf/origination_v6_nssa.go -- forwardingAddressForAF -->
  <!-- source: internal/plugins/ospf/redist_wiring.go -- nssaIPv4Address -->
- **The scope decision lives in the engine**, so the redistribution framework
  stays address-family generic. An ASBR in an NSSA injects Type-7, and a
  normal-area ASBR keeps Type-5 AS-wide.
  <!-- source: internal/plugins/ospf/origination_v6_external.go -- v6InjectExternal -->
- **Forwarding-address lifetime follows the interface state.** Imports retain
  their source policy, metric inputs and assigned Link State ID. A link-down
  event reselects the forwarding address and re-originates under the same ID;
  a lost final address flushes a requested P-set LSA. The periodic replay
  retries changes delayed by MinLSInterval.
  <!-- source: internal/plugins/ospf/redist_wiring.go -- reconcileExternalImports, externalInterfaces -->
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
