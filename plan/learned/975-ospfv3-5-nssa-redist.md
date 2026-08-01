# 975 -- ospfv3-5-nssa-redist

## Context
The unified OSPF engine (spec-ospf-af-unify) shipped v6 redistribution, but the v6 path always
originated Type-5 AS-External-LSAs (0x4005) into the AS-wide store. An OSPFv3 ASBR sitting inside
an NSSA could therefore not inject externals, because an NSSA blocks Type-5. The goal was RFC 3101
NSSA behavior for IPv6: originate Type-7 NSSA-LSAs (0x2007) into the NSSA with correct
P-bit/forwarding-address, and translate them to Type-5 at the elected NSSA ABR for other areas.
A second goal (Part B) was to make the v6 redistribution interop install a real route in FRR
instead of skip-passing, which was blocked because the redistribution framework required a
configured BGP reactor and was not feeding best-path changes to the producer.

## Decisions
- Reuse the v4 NSSA policy (translator election, P-bit boundary rule, source preference) and vary
  only the wire encode, chosen over a parallel v6 NSSA engine because RFC 3101 semantics are
  AF-independent.
- Encode the v6 NSSA-LSA by reusing `ExternalLSA.WriteTo`: the NSSA-LSA body is byte-identical to
  AS-External (RFC 5340 §A.4.8); they differ only in LS Type and flooding scope. No separate v6
  NSSA WriteTo was needed (assumption A-1 resolved in code's favor).
- Carry the P-bit in the prefix's PrefixOptions (`OptPrefixP`), not a header Options bit, per OSPFv3.
- Part B: add a real GoBGP eBGP peering as the interop route source, chosen over registering a fake
  `static` source or decoupling redistribute from the BGP-reactor requirement, because a BGP peering
  is the realistic deployment and keeps the framework intact; the alternatives were flagged as a
  separate framework follow-up.
- Bridge BGP best-path changes into the generic redistribution producer (`bgpredist.EmitBestChange`
  from rib_bestchange) rather than faking route events.

## Consequences
- An OSPFv3 ASBR in an NSSA now injects externals as Type-7 and a normal-area ASBR keeps Type-5
  AS-wide; the scope decision lives in the engine (`v6InjectExternal` / `externalScopeV6`), keeping
  the redistribution framework AF-generic.
- `import bgp` now matches `ibgp`/`ebgp` umbrella sub-sources while still rejecting loops
  (`route.Origin == importingProtocol`), enabling BGP-sourced redistribution end to end.
- The redistribution framework still couples to a configured BGP reactor; registering non-BGP
  sources / decoupling remains a separate follow-up.

## Gotchas
- A redistribution/translation TOCTOU race: `translateNSSAV6` snapshots `redistV6` into a keep-set
  then flushes stale self-LSAs; a just-injected Type-5 must be in that set. Fixed by sharing
  `nssaMu` between `v6InjectExternal` and `translateNSSAV6` (lock order nssaMu -> e.mu); guarded by
  `TestOSPFv6InjectExternalSurvivesNSSATranslation`.
- FRR rejected redistributed OSPFv3 routes until Ze drained Loading -> Full and advertised a
  reachable Router-LSA; the real fix lived in the Link-LSA/DD/LSReq work (spec-ospfv3-4-link-lsa),
  not in faking Router-LSA links.
- A non-candidate ABR with the Nt-bit clear must not wedge translation off for a willing lower-
  Router-ID candidate (`TestOSPFv6NSSANonCandidateDoesNotWedge`).
- Cross-area Type-7 -> Type-5 translation is unit/in-process validated this pass; the FRR scenario
  proves NSSA-internal install + Type-5 non-leak, not the multi-area translated route. The two
  interop scenarios (`ospf-v6-redist-frr`, `ospf-v6-nssa-redist-frr`) are real end-to-end assertions
  gated on a live Docker/FRR/GoBGP environment (CI), pending final rerun.

## Files
- `internal/plugins/ospf/origination_v6_nssa.go` (created) -- v6 Type-7 origination, `externalScopeV6`, P-bit/FA boundary
- `internal/plugins/ospf/origination_v6_external.go` -- Type-5/Type-7 scope decision + withdrawal keep-set
- `internal/plugins/ospf/nssa.go` -- `translateNSSAV6` (AF-aware translation reusing v4 election)
- `internal/plugins/ospf/v3/packet/lsa_nssa.go` -- v6 NSSA-LSA body reuses ExternalLSA; `NSSAPropagate`/`OptPrefixP`
- `internal/component/bgp/redistribute/{bgp.go,producer.go}` + `internal/component/bgp/plugins/rib/rib_bestchange.go` -- BGP source registration + best-path producer bridge
- `internal/component/config/redistribute/route.go` -- umbrella-origin import match with loop prevention
- `test/interop/scenarios/{ospf-v6-redist-frr,ospf-v6-nssa-redist-frr}/` -- real GoBGP-sourced route-install assertions
- `internal/plugins/ospf/origination_v6_nssa_test.go`, `internal/component/bgp/redistribute/producer_test.go` -- unit tests
