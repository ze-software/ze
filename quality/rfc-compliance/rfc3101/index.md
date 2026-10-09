# RFC 3101 - The OSPF Not-So-Stubby Area (NSSA) Option

Experimental. Every requirement this repository extracted from RFC 3101, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 92.3% | 24 of 26 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 26 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 26 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 26 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 47.6% | 49 of 103 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 26 | of 32 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 26 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 26 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 26 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 26 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 7.7% | 2 of 26 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 28 | of 26 gated MUSTs judged | 3 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 26 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 32 |
| Gated MUST-level | 26 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 2 |
| Declared gaps a test demonstrates | 2 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 108 |
| Tagged units | 103 |
| Recorded audit verdicts | 28 |
| Discrimination records | 49 |
| Summary | `rfc/short/rfc3101.md` |
| Requirement shard | `rfc/requirements/rfc3101.md` |
| RFC text | `rfc/full/rfc3101.txt` |

## Enrolment

Enrolled: Ze implements NSSA area operation, Type-7 external routing and border-router translation. The checklist includes forwarding-address lifecycle, per-area ASBR and forwarding-address eligibility, and the Appendix D configuration requirements.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered**

Type-7 origination/flooding, N/E-bit Hello negotiation, redistribution into NSSA, translator election and Type-7 to Type-5 translation, summary import policy and mandatory border-router defaults. The external stage retains per-area ASBR reachability and enforces Type-5/Type-7 forwarding-address eligibility. Per-source `nssa-propagate` defaults to false, and interface lifecycle reconciliation replaces forwarding addresses for retained imports. The shared external calculation and origination policies serve OSPFv2 and the RFC 5838 OSPFv3 address families.

**What the ledger says remains**

Disclosed deviation [`RFC3101-3.1-4`](#rfc3101-3.1-4) (translator election, Section 3.1): Ze sets the Nt-bit on every NSSA border router whose translate role is not `never`, where the RFC sets it only for the Always role, and a candidate is disabled only by a reachable border router that has the Nt-bit set and a higher router ID. A higher-router-ID `translate never` router therefore cannot leave the NSSA without a translator, which the RFC's "Nt bit set or who has a higher router ID" rule allows. The row is a gap and is not counted conformant. Gap [`RFC3101-3.1-5`](#rfc3101-3.1-5) (Section 3.1): the translator list checks only that the NSSA's SPF reaches a border router, not that the router is also reachable as an ASBR over the AS's transit topology. Current producer and tagged-test changes require the parent validation run and discrimination proofs before any revised conformance claim. The existing operator-path default scenarios and FRR scenario remain part of that verification population.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 24 | one part of the gated population |
| Annotated (including scoped evidence) | 2 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **26** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (24):** [`RFC3101-2.1-1`](#rfc3101-2.1-1), [`RFC3101-2.1-2`](#rfc3101-2.1-2), [`RFC3101-x-1`](#rfc3101-x-1), [`RFC3101-2.3-1`](#rfc3101-2.3-1), [`RFC3101-2.3-2`](#rfc3101-2.3-2), [`RFC3101-2.4-1`](#rfc3101-2.4-1), [`RFC3101-2.4-2`](#rfc3101-2.4-2), [`RFC3101-2.4-3`](#rfc3101-2.4-3), [`RFC3101-2.4-4`](#rfc3101-2.4-4), [`RFC3101-2.4-5`](#rfc3101-2.4-5), [`RFC3101-2.5-1`](#rfc3101-2.5-1), [`RFC3101-2.3-3`](#rfc3101-2.3-3), [`RFC3101-2.5-2`](#rfc3101-2.5-2), [`RFC3101-2.5-3`](#rfc3101-2.5-3), [`RFC3101-2.5-4`](#rfc3101-2.5-4), [`RFC3101-2.5-5`](#rfc3101-2.5-5), [`RFC3101-3.1-1`](#rfc3101-3.1-1), [`RFC3101-2.7-1`](#rfc3101-2.7-1), [`RFC3101-3.1-2`](#rfc3101-3.1-2), [`RFC3101-3.2-1`](#rfc3101-3.2-1), [`RFC3101-x-3`](#rfc3101-x-3), [`RFC3101-x-4`](#rfc3101-x-4), [`RFC3101-x-5`](#rfc3101-x-5), [`RFC3101-2.7-3`](#rfc3101-2.7-3)

**Annotated (including scoped evidence) (2):** [`RFC3101-3.1-5`](#rfc3101-3.1-5), [`RFC3101-3.1-4`](#rfc3101-3.1-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3101-2.1-1` | To support the NSSA option an additional check must be made in the function that handles the receiving of the Hello packet to verify that both the N-bit and the E-bit found in the Hello packet's option field match the area type and ExternalRoutingCapability of the area of the receiving interface. (Section 2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc3101_hello_nssa_test.go#L46). **positive:** `unit/verify` [`TestRFC3101HelloOptionsMatchingAreaTypeAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc3101_hello_options_test.go#L38). **negative:** `unit/verify` [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc3101_hello_nssa_test.go#L68). **negative:** `unit/verify` [`TestRFC3101HelloOptionsMismatchDroppedOnEveryAreaType`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc3101_hello_options_test.go#L56) |
| `RFC3101-2.1-2` | both NSSA neighbors must agree on the setting of the "N" bit or the OSPF neighbor adjacency will not form. (§1.3) | MUST | 1.3 | **positive:** `unit/verify` [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc3101_hello_nssa_test.go#L48). **negative:** `unit/verify` [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc3101_hello_nssa_test.go#L59) |
| `RFC3101-x-1` | Therefore, if the N-bit is set in the options field, the E-bit must be clear. (§2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc3101_hello_nssa_test.go#L50). **positive:** `unit/verify` [`TestRFC3101NSSAHelloSentWithNSetAndEClear`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc3101_hello_options_test.go#L77). **negative:** `unit/verify` [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc3101_hello_nssa_test.go#L70). **negative:** `unit/verify` [`TestRFC3101NSSAHelloWithNAndESetRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc3101_hello_options_test.go#L99) |
| `RFC3101-2.3-1` | To support NSSAs the link- state database must therefore be expanded to contain Type-7 LSAs. (Section 2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestOSPFType7Origination`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_test.go#L33). **negative:** `unit/verify` [`TestOSPFType7Origination`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_test.go#L49) |
| `RFC3101-2.3-2` | Type-7 LSAs are only flooded within the originating NSSA. (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestOSPFType7FloodScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_test.go#L86). **positive:** `unit/verify` [`TestRFC3101Type7FloodedWithinItsNSSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_type7_flood_scope_test.go#L57). **negative:** `unit/verify` [`TestOSPFType7FloodScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_test.go#L92). **negative:** `unit/verify` [`TestRFC3101Type7NotFloodedIntoAnotherNSSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_type7_flood_scope_test.go#L70) |
| `RFC3101-2.4-1` | An NSSA internal AS boundary router must set the P-bit in the LSA header's option field of any Type-7 LSA whose network it wants advertised into the OSPF domain's full transit topology. (Section 2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestOSPFType7Origination`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_test.go#L35). **negative:** `unit/verify` [`TestOSPFType7Origination`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_test.go#L57) |
| `RFC3101-2.4-2` | If the P-bit is set, the forwarding address must be non-zero; otherwise it may be 0.0.0.0.  If an NSSA requires the P-bit be set and a non-zero forwarding address is unavailable, then the route's Type-7 LSA is not originated into this NSSA. (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestOSPFNSSAPBitBoundaryPolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_pbit_test.go#L32). **negative:** `unit/verify` [`TestOSPFNSSAPBitBoundaryPolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_pbit_test.go#L23). **negative:** `unit/verify` [`TestOSPFv3NSSADefaultPBitFollowsForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_origination_v6_nssa_default_test.go#L191). **negative:** `unit/verify` [`TestOSPFv3NSSAInternalRouterDefaultNeedsForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_origination_v6_nssa_default_test.go#L171) |
| `RFC3101-2.4-3` | When an NSSA border router originates both a Type-5 LSA and a Type-7 LSA for the same network, then the P-bit must be clear in the Type-7 LSA so that it isn't translated into a Type-5 LSA by another NSSA border router. (§2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestOSPFNSSAPBitBoundaryPolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_pbit_test.go#L34). **negative:** `unit/verify` [`TestOSPFNSSAPBitBoundaryPolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_pbit_test.go#L42) |
| `RFC3101-2.4-4` | The Type-7 default LSA originated by an NSSA border router must have the P-bit clear. (Section 2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestOSPFNSSABorderRouterDefaultsEveryArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_ac14_16_test.go#L38). **positive:** `unit/verify` [`TestOSPFv3NSSABorderRouterOriginatesDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_origination_v6_nssa_default_test.go#L71). **negative:** `unit/verify` [`TestRFC3101BorderRouterDefaultStaysPClearUnderPropagatingImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_border_default_pbit_test.go#L23) |
| `RFC3101-2.4-5` | NSSA border routers must originate an LSA for the default destination into all their directly attached NSSAs in order to support intra-AS routing and inter-AS routing. (Section 2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestOSPFNSSABorderRouterDefaultsEveryArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_ac14_16_test.go#L32). **positive:** `unit/verify` [`TestOSPFNSSANoSummaryDefaultInjection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_area_type_test.go#L128). **positive:** `unit/verify` [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_area_type_test.go#L103). **positive:** `unit/verify` [`TestOSPFv3NSSABorderRouterOriginatesDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_origination_v6_nssa_default_test.go#L62). **negative:** `unit/verify` [`TestOSPFNSSAInternalRouterOriginatesNoBorderDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_ac14_16_test.go#L53). **negative:** `unit/verify` [`TestOSPFv3NSSAInternalRouterDefaultNeedsForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_origination_v6_nssa_default_test.go#L173). **positive:** `interop/nightly` [`checkNSSADefault`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1255) |
| `RFC3101-2.5-1` | Else if the destination is a Type-7 default route (destination ID = DefaultDestination) and one of the following is true, then do nothing with this LSA and consider the next in the list: o The calculating router is a border router and the LSA has its P-bit clear. Appendix E describes a technique whereby an NSSA border router installs a Type-7 default LSA without propagating it. o The calculating router is a border router and is suppressing the import of summary routes as Type-3 summary-LSAs. (§2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestOSPFNSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_external_nssa_test.go#L113). **positive:** `unit/verify` [`TestOSPFv3NSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_install_gate_v6_test.go#L99). **negative:** `unit/verify` [`TestOSPFNSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_external_nssa_test.go#L96). **negative:** `unit/verify` [`TestOSPFNSSANonBorderRouterInstallsPClearDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_external_nssa_test.go#L163). **negative:** `unit/verify` [`TestOSPFv3NSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_install_gate_v6_test.go#L81). **negative:** `unit/verify` [`TestOSPFv3NSSANonBorderRouterInstallsPClearDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_install_gate_v6_test.go#L119) |
| `RFC3101-2.3-3` | When the interface whose IP address is the LSA's forwarding address transitions to a Down state (see [OSPF] Section 9.3), the router must select a new forwarding address for the LSA and then re- originate it. (Section 2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestNSSAImportReoriginatesAfterForwardingInterfaceDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_import_lifecycle_test.go#L111). **negative:** `unit/verify` [`TestNSSAImportIgnoresUnrelatedDownAndWithdrawnIntent`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_import_lifecycle_test.go#L152) |
| `RFC3101-2.5-2` | Since the flooding scope of a Type-7 LSA is restricted to the originating NSSA, the routing table entry of its ASBR must be found in the originating NSSA. (Section 2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestNSSAASBRUsesOriginatingArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_scope_test.go#L93). **positive:** `unit/verify` [`TestNSSAExternalCalculationRunsOnType7Change`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_external_nssa_recalc_test.go#L44). **negative:** `unit/verify` [`TestNSSAASBRRejectsOtherArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_scope_test.go#L112). **negative:** `unit/verify` [`TestNSSAExternalCalculationDropsLostASBR`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_external_nssa_recalc_test.go#L67) |
| `RFC3101-2.5-3` | For a Type-5 LSA the matching routing table entry must specify an intra-area or inter-area path through a Type-5 capable area. (Section 2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestExternalForwardingPreservesInternalPreferenceAndECMP`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_external_nssa_test.go#L230). **positive:** `unit/verify` [`TestExternalForwardingScopeAcceptsEligiblePaths`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_scope_test.go#L128). **negative:** `unit/verify` [`TestExternalForwardingScopeRejectsIneligiblePaths`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_scope_test.go#L148) |
| `RFC3101-2.5-4` | For a Type-7 LSA the matching routing table entry must specify an intra-area path through the LSA's originating NSSA. (Section 2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestExternalForwardingScopeAcceptsEligiblePaths`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_scope_test.go#L129). **negative:** `unit/verify` [`TestExternalForwardingScopeRejectsIneligiblePaths`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_scope_test.go#L149) |
| `RFC3101-2.5-5` | This calculation must be run when Type-7 LSAs are processed during the AS external route calculation. (§2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestNSSAExternalCalculationRunsOnType7Change`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_external_nssa_recalc_test.go#L43). **negative:** `unit/verify` [`TestNSSAExternalCalculationDropsLostASBR`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_external_nssa_recalc_test.go#L66) |
| `RFC3101-3.1-1` | All NSSA border routers must set the E-bit in the Type-1 router-LSAs of their directly attached non-stub areas, even when they are not translating. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestOSPFASBRBitFromNSSAType7`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_origination_external_test.go#L90). **positive:** `unit/verify` [`TestRFC3101NSSABorderEBitInEveryNonStubArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_border_ebit_areas_test.go#L63). **positive:** `unit/verify` [`TestRFC3101NSSABorderRouterSetsEBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_border_ebit_test.go#L36). **positive:** `unit/verify` [`TestRFC3101V6NSSABorderEBitInEveryNonStubArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_v6_nssa_border_ebit_test.go#L75). **negative:** `unit/verify` [`TestOSPFASBRBitFromNSSAType7`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_origination_external_test.go#L76). **negative:** `unit/verify` [`TestRFC3101NSSABorderRouterSetsEBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_border_ebit_test.go#L48). **negative:** `unit/verify` [`TestRFC3101NoNSSAAttachedEBitClearEverywhere`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_border_ebit_areas_test.go#L75). **negative:** `unit/verify` [`TestRFC3101V6NoNSSAAttachedEBitClearEverywhere`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_v6_nssa_border_ebit_test.go#L94) |
| `RFC3101-2.7-1` | In order for OSPF's summary routing to not be obscured by an NSSA's Type-7 AS-external-LSAs, all NSSA border router implementations must support the optional import of summary routes into NSSAs as Type-3 summary-LSAs. (§2.7) | MUST | 2.7 | **positive:** `unit/verify` [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_area_type_test.go#L85). **negative:** `unit/verify` [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_area_type_test.go#L96) |
| `RFC3101-3.1-2` | An NSSA border router whose NSSA's NSSATranslatorRole is set to Candidate must maintain a list of the NSSA's border routers that are reachable both over the NSSA (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestOSPFNSSATranslatorElection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_test.go#L64). **positive:** `unit/verify` [`TestRFC3101UnreachableBorderRouterIsNotListed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_translator_list_test.go#L47). **negative:** `unit/verify` [`TestOSPFNSSANoTranslateWhenNotElected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_test.go#L260). **negative:** `unit/verify` [`TestOSPFNSSATranslatorElection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_test.go#L67). **negative:** `unit/verify` [`TestRFC3101ReachableBorderRouterIsListed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_translator_list_test.go#L76) |
| `RFC3101-3.1-5` | An NSSA border router whose NSSA's NSSATranslatorRole is set to Candidate must maintain a list of the NSSA's border routers that are reachable both over the NSSA and as ASBRs over the AS's transit topology. (Section 3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the second membership condition, "as ASBRs over the AS's transit topology", is not implemented. electNSSATranslator (internal/plugins/ospf/nssa.go) lists every NSSA border router with the B-bit and the Nt-bit that the NSSA's completed SPF reaches (RFC3101-3.1-2), and never asks whether the router is reachable as an ASBR through the backbone or any other transit area. A higher-router-ID border router that the NSSA reaches but the transit topology does not therefore still disables the candidate. TestRFC3101BorderRouterNotReachedAsASBROverTransitIsNotListed demonstrates it; docs/architecture/ospf/ospf-11-stub-nssa.md discloses it |
| `RFC3101-3.1-4` | If there exists another border router in this list whose router-LSA has bit Nt set or who has a higher router ID, then its NSSATranslatorState is disabled. (Section 3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** disclosed deviation. Ze sets the Nt-bit on every NSSA border router whose translate role is not `never` (ntAreas, internal/plugins/ospf/instance.go), where Section 3.1 sets it only for the Always role, and a candidate is disabled only by a listed router with the Nt-bit set AND a higher router ID (nssaABRs and electNSSATranslator, internal/plugins/ospf/nssa.go). A higher-router-ID border router configured `translate never` therefore does not disable a willing candidate, which under the RFC would leave the NSSA with no translator, and a lower-router-ID Always router does not disable it either. TestOSPFNSSANonCandidateDoesNotWedge pins the chosen behaviour; docs/architecture/ospf/ospf-11-stub-nssa.md discloses it |
| `RFC3101-3.2-1` | The newly originated Type-5 LSA will describe the same network and have the same network mask, path type, metric, forwarding address and external route tag as the Type-7 LSA.  The advertising router field will be the router ID of this NSSA border router. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestOSPFNSSATranslation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_test.go#L127). **negative:** `unit/verify` [`TestOSPFNSSAPbitNotTranslated`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_test.go#L188). **negative:** `unit/verify` [`TestOSPFNSSATranslation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_test.go#L142). **negative:** `unit/verify` [`TestRFC3101TranslatedType5FollowsSourceFields`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_translation_test.go#L19) |
| `RFC3101-3.2-2` | the calculating router has the highest router ID amongst NSSA translators that have originated a functionally equivalent Type-5 LSA (i.e. same destination, cost and non-zero forwarding address) and that are reachable over area 0 and the NSSA, then a Type-5 LSA should be generated (§3.2) | SHOULD | 3.2 | **positive:** `unit/verify` [`TestOSPFHigherRIDType5Exists`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_higher_rid_test.go#L38). **positive:** `unit/verify` [`TestOSPFNSSAHigherRIDType5Suppresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_ac14_16_test.go#L119). **positive:** `unit/verify` [`TestRFC3101TranslatorYieldsOnlyToEquivalentType5`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_translator_equivalence_test.go#L60). **positive:** `unit/verify` [`TestRFC3101TranslatorYieldsOnlyToEquivalentType5V6`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_translator_equivalence_test.go#L123). **negative:** `unit/verify` [`TestOSPFHigherRIDType5Exists`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_higher_rid_test.go#L29). **negative:** `unit/verify` [`TestOSPFNSSAHigherRIDType5Suppresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_ac14_16_test.go#L135). **negative:** `unit/verify` [`TestRFC3101TranslatorYieldsOnlyToEquivalentType5`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_translator_equivalence_test.go#L54). **negative:** `unit/verify` [`TestRFC3101TranslatorYieldsOnlyToEquivalentType5V6`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_translator_equivalence_test.go#L117) |
| `RFC3101-x-3` | Implementations must provide a vehicle for setting the P-bit when external routes are imported into the NSSA as Type-7 LSAs. (§D) | MUST | D - Appendix D | **positive:** `unit/verify` [`TestNSSAPerSourcePropagationEnabled`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_import_lifecycle_test.go#L187). **negative:** `unit/verify` [`TestNSSAPerSourcePropagationCannotOverrideType5`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_import_lifecycle_test.go#L234). **negative:** `unit/verify` [`TestNSSAPerSourcePropagationDisabled`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_import_lifecycle_test.go#L212) |
| `RFC3101-x-4` | For NSSAs the ExternalRoutingCapability area configuration parameter must be set to accept Type-7 external routes. (§D) | MUST | D - Appendix D | **positive:** `unit/verify` [`TestRFC3101NSSAAreaAcceptsType7`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_area_capability_test.go#L45). **negative:** `unit/verify` [`TestRFC3101NormalAreaRejectsType7`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_area_capability_test.go#L55) |
| `RFC3101-x-5` | Additionally there must be a way of configuring the metric of the default LSA that a border router advertises into its directly attached NSSAs. (§D) | MUST | D - Appendix D | **positive:** `unit/verify` [`TestOSPFNSSADefaultCarriesConfiguredMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_default_cost_test.go#L44). **positive:** `unit/verify` [`TestRFC3101BorderRouterDefaultCarriesConfiguredMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_border_default_cost_test.go#L60). **positive:** `unit/verify` [`TestRFC3101BorderRouterType3DefaultCarriesConfiguredMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_type3_default_cost_test.go#L71). **negative:** `unit/verify` [`TestOSPFNSSADefaultMetricIsNotFixed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_default_cost_test.go#L55). **negative:** `unit/verify` [`TestRFC3101BorderRouterDefaultMetricIsNotBorrowed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_border_default_cost_test.go#L73). **negative:** `unit/verify` [`TestRFC3101BorderRouterType3DefaultMetricIsNotBorrowed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_type3_default_cost_test.go#L84) |
| `RFC3101-x-2` | If an elected translator determines its services are no longer required, it continues to perform its translation duties for the additional time interval defined by a new area configuration parameter, TranslatorStabilityInterval.  This minimizes excessive flushing of translated Type-7 LSAs and provides for a more stable translator transition.  The default value for the TranslatorStabilityInterval parameter has been defined as 40 seconds. (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3101-3.1-3` | It is not recommended that multiple NSSA border routers perform Type-7 to Type-5 translation unless it is required to route packets efficiently through Area 0 to an NSSA partitioned by Type-7 address ranges. (Section 3.1) | NOT RECOMMENDED | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3101-2.4-6` | Since Type-5 AS-external-LSAs are not flooded into NSSAs, NSSA border routers should not originate Type-4 summary- LSAs into their NSSAs. (§1.3) | SHOULD NOT | 1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3101-2.7-2` | When OSPF's summary routes are not imported, the default LSA originated by an NSSA border router into the NSSA should be a Type-3 summary-LSA. (§2.7) | SHOULD | 2.7 | **positive:** `unit/verify` [`TestOSPFNSSANoSummaryDefaultInjection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_area_type_test.go#L126). **positive:** `unit/verify` [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_area_type_test.go#L101). **positive:** `unit/verify` [`TestOSPFv3NSSANoSummaryDefaultUsesSummaryLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_origination_v6_nssa_default_test.go#L105). **negative:** `unit/verify` [`TestOSPFNSSANoSummaryDefaultInjection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_area_type_test.go#L149). **negative:** `unit/verify` [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_area_type_test.go#L90). **negative:** `unit/verify` [`TestOSPFv3NSSANoSummaryDefaultUsesSummaryLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_origination_v6_nssa_default_test.go#L99) |
| `RFC3101-2.7-3` | When summary routes are imported into the NSSA, the default LSA originated by an NSSA border router must not be a Type-3 summary-LSA (§2.7) | MUST NOT | 2.7 | **positive:** `unit/verify` [`TestOSPFNSSANoSummaryDefaultInjection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_area_type_test.go#L151). **positive:** `unit/verify` [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_area_type_test.go#L92). **positive:** `unit/verify` [`TestOSPFv3NSSANoSummaryDefaultUsesSummaryLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_origination_v6_nssa_default_test.go#L117). **negative:** `unit/verify` [`TestOSPFNSSANoSummaryDefaultInjection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_area_type_test.go#L130). **negative:** `unit/verify` [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_area_type_test.go#L105). **negative:** `unit/verify` [`TestOSPFv3NSSANoSummaryDefaultUsesSummaryLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_origination_v6_nssa_default_test.go#L108) |
| `RFC3101-2.2-1` | NSSA border routers may aggregate Type-7 routes by advertising a single Type-5 LSA for each Type-7 address range. (§2.2) | MAY | 2.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC3101-3.1-5`](#rfc3101-3.1-5) An NSSA border router whose NSSA's NSSATranslatorRole is set to Candidate must maintain a list of the NSSA's border routers that are reachable both over the NSSA and as ASBRs over the AS's transit topology. (Section 3.1) | {gap}, demonstrated by internal/plugins/ospf/rfc3101_translator_list_test.go::TestRFC3101BorderRouterNotReachedAsASBROverTransitIsNotListed | the second membership condition, "as ASBRs over the AS's transit topology", is not implemented. electNSSATranslator (internal/plugins/ospf/nssa.go) lists every NSSA border router with the B-bit and the Nt-bit that the NSSA's completed SPF reaches (RFC3101-3.1-2), and never asks whether the router is reachable as an ASBR through the backbone or any other transit area. A higher-router-ID border router that the NSSA reaches but the transit topology does not therefore still disables the candidate. TestRFC3101BorderRouterNotReachedAsASBROverTransitIsNotListed demonstrates it; docs/architecture/ospf/ospf-11-stub-nssa.md discloses it |
| [`RFC3101-3.1-4`](#rfc3101-3.1-4) If there exists another border router in this list whose router-LSA has bit Nt set or who has a higher router ID, then its NSSATranslatorState is disabled. (Section 3.1) | {gap}, demonstrated by internal/plugins/ospf/rfc3101_translator_list_test.go::TestRFC3101HigherRouterIDWithoutNtDisablesCandidate | disclosed deviation. Ze sets the Nt-bit on every NSSA border router whose translate role is not `never` (ntAreas, internal/plugins/ospf/instance.go), where Section 3.1 sets it only for the Always role, and a candidate is disabled only by a listed router with the Nt-bit set AND a higher router ID (nssaABRs and electNSSATranslator, internal/plugins/ospf/nssa.go). A higher-router-ID border router configured `translate never` therefore does not disable a willing candidate, which under the RFC would leave the NSSA with no translator, and a lower-router-ID Always router does not disable it either. TestOSPFNSSANonCandidateDoesNotWedge pins the chosen behaviour; docs/architecture/ospf/ospf-11-stub-nssa.md discloses it |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC3101-2.1-1`](#rfc3101-2.1-1)

To support the NSSA option an additional check must be made in the function that handles the receiving of the Hello packet to verify that both the N-bit and the E-bit found in the Hello packet's option field match the area type and ExternalRoutingCapability of the area of the receiving interface. (Section 2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. c22 re-judge. The quote requires the received Hello's N-bit and E-bit to match the area type of the receiving interface, any area type. iface/rfc3101_hello_options_test.go drives ReceiveHello -> validateHelloLocked. Positive: matching options accepted on normal (E), stub (none) and NSSA (N). Negative: normal N set -> options-n, normal E clear -> options-e, stub N set -> options-n, stub E set -> options-e, NSSA N clear -> options-n; each case keeps the other bit matching, so the asserted drop reason isolates the bit under test (E is checked first). The author overlay comparing N only on NSSA (the earlier finding) reds normal-n-set and stub-n-set while the old NSSA-only TestOSPFNSSANbitMismatch stays green. Both polarities carry a recorded revert of validateHelloLocked.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc3101_hello_nssa_test.go#L68) | unit/verify | unproven |
| negative | [`TestRFC3101HelloOptionsMismatchDroppedOnEveryAreaType`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc3101_hello_options_test.go#L56) | unit/verify | revert, verified |
| positive | [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc3101_hello_nssa_test.go#L46) | unit/verify | unproven |
| positive | [`TestRFC3101HelloOptionsMatchingAreaTypeAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc3101_hello_options_test.go#L38) | unit/verify | revert, verified |

### [`RFC3101-2.1-2`](#rfc3101-2.1-2)

both NSSA neighbors must agree on the setting of the "N" bit or the OSPF neighbor adjacency will not form. (§1.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an adjacency with a neighbour disagreeing on N. iface/hello_nssa_test.go TestOSPFNSSANbitMismatch sends an N-clear Hello to an NSSA interface and asserts DropReasonOptionsN (line 63); receiveHello returns on that reason before the neighbour is created. Positive: the N-set, E-clear Hello is accepted (line 54).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc3101_hello_nssa_test.go#L59) | unit/verify | unproven |
| positive | [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc3101_hello_nssa_test.go#L48) | unit/verify | unproven |

### [`RFC3101-x-1`](#rfc3101-x-1)

Therefore, if the N-bit is set in the options field, the E-bit must be clear. (§2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. c22 re-judge. Positive TestRFC3101NSSAHelloSentWithNSetAndEClear reads the Options octet at offset 30 of buildHelloPacketLocked (the payload every Hello send uses, iface.go) on an NSSA interface and asserts 0x08 set and 0x02 clear as literal bits (recorded revert of expectedOptionsLocked). Negative under OWNER RULING 2: an NSSA Hello carrying N and E together is dropped with options-e (TestRFC3101NSSAHelloWithNAndESetRefused, recorded revert of validateHelloLocked); the author overlay skipping the E check when N is set reds it while the old E-only negative stays green. RFC 3101 is OSPFv2; the Options octet checked is the OSPFv2 one.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc3101_hello_nssa_test.go#L70) | unit/verify | unproven |
| negative | [`TestRFC3101NSSAHelloWithNAndESetRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc3101_hello_options_test.go#L99) | unit/verify | revert, verified |
| positive | [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc3101_hello_nssa_test.go#L50) | unit/verify | unproven |
| positive | [`TestRFC3101NSSAHelloSentWithNSetAndEClear`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc3101_hello_options_test.go#L77) | unit/verify | revert, verified |

### [`RFC3101-2.3-1`](#rfc3101-2.3-1)

To support NSSAs the link- state database must therefore be expanded to contain Type-7 LSAs. (Section 2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an LSDB that cannot hold Type-7. lsdb/nssa_test.go TestOSPFType7Origination requires the originated route under LSTypeNSSA in the NSSA area store (require.True line 38) and asserts it is absent from the AS-wide Type-5 store (line 52). Received Type-7 storage is covered under RFC3101-x-4.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFType7Origination`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_test.go#L49) | unit/verify | unproven |
| positive | [`TestOSPFType7Origination`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_test.go#L33) | unit/verify | unproven |

### [`RFC3101-2.3-2`](#rfc3101-2.3-2)

Type-7 LSAs are only flooded within the originating NSSA. (§2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. c22 re-judge. lsdb/rfc3101_type7_flood_scope_test.go: ABR with eth0 backbone, eth1+eth4 NSSA 0.0.0.1, eth2 NSSA 0.0.0.2, all with Full neighbors; a Type-7 from 3.3.3.3 enters through ReceiveUpdate on eth1. Positive: installed in 0.0.0.1 and flooded out eth4 (same NSSA). Negative: with eth4 reach asserted first, no LS Update carries it out eth2 (a different NSSA) or eth0, no retransmission entry for 5.5.5.5 or 2.2.2.2, no copy in area 0.0.0.2. Dropping the AreaID==area comparison in eligibleInterface's NSSA case (the earlier finding) reds the new negative while TestOSPFType7FloodScope stays green. Both polarities carry a recorded revert of flooding.go::eligibleInterface.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFType7FloodScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_test.go#L92) | unit/verify | unproven |
| negative | [`TestRFC3101Type7NotFloodedIntoAnotherNSSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_type7_flood_scope_test.go#L70) | unit/verify | revert, verified |
| positive | [`TestOSPFType7FloodScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_test.go#L86) | unit/verify | unproven |
| positive | [`TestRFC3101Type7FloodedWithinItsNSSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_type7_flood_scope_test.go#L57) | unit/verify | revert, verified |

### [`RFC3101-2.4-1`](#rfc3101-2.4-1)

An NSSA internal AS boundary router must set the P-bit in the LSA header's option field of any Type-7 LSA whose network it wants advertised into the OSPF domain's full transit topology. (Section 2.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a propagate-wanted Type-7 with P clear. lsdb/nssa_test.go TestOSPFType7Origination asserts OptionNP set when propagate=true (line 39) and clear when propagate=false (line 63).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFType7Origination`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_test.go#L57) | unit/verify | unproven |
| positive | [`TestOSPFType7Origination`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_test.go#L35) | unit/verify | unproven |

### [`RFC3101-2.4-2`](#rfc3101-2.4-2)

If the P-bit is set, the forwarding address must be non-zero; otherwise it may be 0.0.0.0.  If an NSSA requires the P-bit be set and a non-zero forwarding address is unavailable, then the route's Type-7 LSA is not originated into this NSSA. (§2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (OSPF judge, continuations 3+4): TestOSPFv3NSSADefaultPBitFollowsForwardingAddress now carries a 2.4-2 negative (was a misplaced 2.4-4 tag): a P-set default keeps its non-zero forwarding address beside P and is flushed, not originated P-set without one, once the address is lost. The lsdb TestOSPFNSSAPBitBoundaryPolicy pair and TestOSPFv3NSSAInternalRouterDefaultNeedsForwardingAddress are unchanged. Prior judgement: Forbidden: a P-set Type-7 with a zero forwarding address. lsdb/nssa_pbit_test.go TestOSPFNSSAPBitBoundaryPolicy asserts the P-set, zero-FA request is not originated (LookupLSA exists -> Fatal, line 27) and a P-set, non-zero-FA request keeps P (line 37); origination_v6_nssa_default_test.go TestOSPFv3NSSAInternalRouterDefaultNeedsForwardingAddress asserts no v3 P-set default without a usable address (line 175). The 'may be 0.0.0.0' clause is a permission and has no forbidden behaviour.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFNSSAPBitBoundaryPolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_pbit_test.go#L23) | unit/verify | unproven |
| negative | [`TestOSPFv3NSSADefaultPBitFollowsForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_origination_v6_nssa_default_test.go#L191) | unit/verify | revert, verified |
| negative | [`TestOSPFv3NSSAInternalRouterDefaultNeedsForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_origination_v6_nssa_default_test.go#L171) | unit/verify | unproven |
| positive | [`TestOSPFNSSAPBitBoundaryPolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_pbit_test.go#L32) | unit/verify | unproven |

### [`RFC3101-2.4-3`](#rfc3101-2.4-3)

When an NSSA border router originates both a Type-5 LSA and a Type-7 LSA for the same network, then the P-bit must be clear in the Type-7 LSA so that it isn't translated into a Type-5 LSA by another NSSA border router. (§2.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: P set on a Type-7 whose network this router also originates as Type-5. lsdb/nssa_pbit_test.go TestOSPFNSSAPBitBoundaryPolicy asserts P clear after OriginateExternal for the same network (line 49) and P set with no self Type-5 (line 37).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFNSSAPBitBoundaryPolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_pbit_test.go#L42) | unit/verify | unproven |
| positive | [`TestOSPFNSSAPBitBoundaryPolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_pbit_test.go#L34) | unit/verify | unproven |

### [`RFC3101-2.4-4`](#rfc3101-2.4-4)

The Type-7 default LSA originated by an NSSA border router must have the P-bit clear. (Section 2.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (OSPF judge, continuations 3+4): positives TestOSPFNSSABorderRouterDefaultsEveryArea (v4) and TestOSPFv3NSSABorderRouterOriginatesDefault (v6) assert the border router's Type-7 default is P-clear; negative TestRFC3101BorderRouterDefaultStaysPClearUnderPropagatingImport pushes every P-set trigger (NSSA default-originate plus a retained default import from an nssa-propagate source) and asserts applyNSSADefaults still installs the default P-clear. The misplaced install-rule negatives moved to RFC3101-2.5-1. Judge-run overlay mutants in applyNSSADefaults: ABR taking the internal-router propagate decision (line 152 'if !isABR' -> 'if true') reds the negative; 'propagate := true' reds both positives. Four stale 2.4-4 records remain for units that lost the tag (orphans, not gate-named).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3101BorderRouterDefaultStaysPClearUnderPropagatingImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_border_default_pbit_test.go#L23) | unit/verify | revert, verified |
| positive | [`TestOSPFNSSABorderRouterDefaultsEveryArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_ac14_16_test.go#L38) | unit/verify | unproven |
| positive | [`TestOSPFv3NSSABorderRouterOriginatesDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_origination_v6_nssa_default_test.go#L71) | unit/verify | unproven |

### [`RFC3101-2.4-5`](#rfc3101-2.4-5)

NSSA border routers must originate an LSA for the default destination into all their directly attached NSSAs in order to support intra-AS routing and inter-AS routing. (Section 2.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a border router leaving an attached NSSA without a default. nssa_ac14_16_test.go TestOSPFNSSABorderRouterDefaultsEveryArea requires one self Type-7 default in the regular NSSA (line 33) on an ABR with two NSSAs; origination_v6_nssa_default_test.go TestOSPFv3NSSABorderRouterOriginatesDefault requires the v3 0x2007 default (line 66); spf/area_type_test.go TestOSPFNSSANoSummaryDefaultInjection requires the Type-3 default in the no-summary NSSA (line 133). Negatives: an internal NSSA router originates none (nssa_ac14_16_test.go line 54, v6 line 175). The interop check checkNSSADefault is also tagged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFNSSAInternalRouterOriginatesNoBorderDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_ac14_16_test.go#L53) | unit/verify | unproven |
| negative | [`TestOSPFv3NSSAInternalRouterDefaultNeedsForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_origination_v6_nssa_default_test.go#L173) | unit/verify | unproven |
| positive | [`checkNSSADefault`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1255) | interop/nightly | unproven |
| positive | [`TestOSPFNSSABorderRouterDefaultsEveryArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_ac14_16_test.go#L32) | unit/verify | unproven |
| positive | [`TestOSPFv3NSSABorderRouterOriginatesDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_origination_v6_nssa_default_test.go#L62) | unit/verify | unproven |
| positive | [`TestOSPFNSSANoSummaryDefaultInjection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_area_type_test.go#L128) | unit/verify | unproven |
| positive | [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_area_type_test.go#L103) | unit/verify | unproven |

### [`RFC3101-2.5-1`](#rfc3101-2.5-1)

Else if the destination is a Type-7 default route (destination ID = DefaultDestination) and one of the following is true, then do nothing with this LSA and consider the next in the list: o The calculating router is a border router and the LSA has its P-bit clear. Appendix E describes a technique whereby an NSSA border router installs a Type-7 default LSA without propagating it. o The calculating router is a border router and is suppressing the import of summary routes as Type-3 summary-LSAs. (§2.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (OSPF judge, continuations 3+4): both skip conditions of section 2.5 step (3) now carry tags: positives TestOSPFNSSABorderRouterDefaultPBit / TestOSPFv3NSSABorderRouterDefaultPBit skip a P-set default under summary suppression and (P-bit clear subtest) a P-clear default with summaries imported; negatives in the same units install a P-set default with summaries imported, and TestOSPFNSSANonBorderRouterInstallsPClearDefault (v4, v6) show both conditions bind a border router only. Six revert records on spf/external.go::ComputeExternalWith, observed red; the ComputeExternalWith comment now names 2.5-1 for the P-clear skip.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFv3NSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_install_gate_v6_test.go#L81) | unit/verify | revert, verified |
| negative | [`TestOSPFv3NSSANonBorderRouterInstallsPClearDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_install_gate_v6_test.go#L119) | unit/verify | revert, verified |
| negative | [`TestOSPFNSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_external_nssa_test.go#L96) | unit/verify | revert, verified |
| negative | [`TestOSPFNSSANonBorderRouterInstallsPClearDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_external_nssa_test.go#L163) | unit/verify | revert, verified |
| positive | [`TestOSPFv3NSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_install_gate_v6_test.go#L99) | unit/verify | revert, verified |
| positive | [`TestOSPFNSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_external_nssa_test.go#L113) | unit/verify | revert, verified |

### [`RFC3101-2.3-3`](#rfc3101-2.3-3)

When the interface whose IP address is the LSA's forwarding address transitions to a Down state (see [OSPF] Section 9.3), the router must select a new forwarding address for the LSA and then re- originate it. (Section 2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: keeping a Type-7 whose forwarding interface went Down. nssa_import_lifecycle_test.go TestNSSAImportReoriginatesAfterForwardingInterfaceDown asserts after onInterfaceDown the next forwarding address, Sequence.Next(), not MaxAge (line 138), and MaxAge when no address remains (line 145); TestNSSAImportIgnoresUnrelatedDownAndWithdrawnIntent asserts an unrelated Down leaves sequence and address unchanged (line 170).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNSSAImportIgnoresUnrelatedDownAndWithdrawnIntent`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_import_lifecycle_test.go#L152) | unit/verify | unproven |
| positive | [`TestNSSAImportReoriginatesAfterForwardingInterfaceDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_import_lifecycle_test.go#L111) | unit/verify | unproven |

### [`RFC3101-2.5-2`](#rfc3101-2.5-2)

Since the flooding scope of a Type-7 LSA is restricted to the originating NSSA, the routing table entry of its ASBR must be found in the originating NSSA. (Section 2.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: taking a Type-7's ASBR entry from another area. nssa_scope_test.go TestNSSAASBRUsesOriginatingArea asserts own-area cost 15 via 192.0.2.9 despite a cheaper backbone entry (line 105); TestNSSAASBRRejectsOtherArea asserts no route when the ASBR is reachable only outside the NSSA, with and without forwarding address (line 120); spf/external_nssa_recalc_test.go repeats both through Computer.Run.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNSSAASBRRejectsOtherArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_scope_test.go#L112) | unit/verify | unproven |
| negative | [`TestNSSAExternalCalculationDropsLostASBR`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_external_nssa_recalc_test.go#L67) | unit/verify | unproven |
| positive | [`TestNSSAASBRUsesOriginatingArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_scope_test.go#L93) | unit/verify | unproven |
| positive | [`TestNSSAExternalCalculationRunsOnType7Change`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_external_nssa_recalc_test.go#L44) | unit/verify | unproven |

### [`RFC3101-2.5-3`](#rfc3101-2.5-3)

For a Type-5 LSA the matching routing table entry must specify an intra-area or inter-area path through a Type-5 capable area. (Section 2.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Type-5 forwarding address resolved through an NSSA, stub, or external route. nssa_scope_test.go TestExternalForwardingScopeRejectsIneligiblePaths asserts no route for the NSSA-area, external-path-type and stub cases (line 173). Positives: inter-area in the backbone (TestExternalForwardingScopeAcceptsEligiblePaths line 140) and intra-area in two Type-5 capable areas (spf/external_nssa_test.go TestExternalForwardingPreservesInternalPreferenceAndECMP lines 238-239).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestExternalForwardingScopeRejectsIneligiblePaths`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_scope_test.go#L148) | unit/verify | unproven |
| positive | [`TestExternalForwardingScopeAcceptsEligiblePaths`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_scope_test.go#L128) | unit/verify | unproven |
| positive | [`TestExternalForwardingPreservesInternalPreferenceAndECMP`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_external_nssa_test.go#L230) | unit/verify | unproven |

### [`RFC3101-2.5-4`](#rfc3101-2.5-4)

For a Type-7 LSA the matching routing table entry must specify an intra-area path through the LSA's originating NSSA. (Section 2.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Type-7 forwarding address resolved outside an intra-area path of its own NSSA. nssa_scope_test.go TestExternalForwardingScopeRejectsIneligiblePaths asserts no route for a backbone route, an inter-area route and a stub-area route (line 173); TestExternalForwardingScopeAcceptsEligiblePaths asserts the own-NSSA intra-area route resolves (line 140).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestExternalForwardingScopeRejectsIneligiblePaths`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_scope_test.go#L149) | unit/verify | unproven |
| positive | [`TestExternalForwardingScopeAcceptsEligiblePaths`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_scope_test.go#L129) | unit/verify | unproven |

### [`RFC3101-2.5-5`](#rfc3101-2.5-5)

This calculation must be run when Type-7 LSAs are processed during the AS external route calculation. (§2.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: the external calculation skipping Type-7 LSAs. spf/external_nssa_recalc_test.go TestNSSAExternalCalculationRunsOnType7Change asserts Computer.Run adds the Type-7 route at cost 15 and changes it to 19 on a Type-7 metric change (lines 50, 61); TestNSSAExternalCalculationDropsLostASBR asserts removal when the NSSA loses the ASBR (line 84).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNSSAExternalCalculationDropsLostASBR`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_external_nssa_recalc_test.go#L66) | unit/verify | unproven |
| positive | [`TestNSSAExternalCalculationRunsOnType7Change`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_external_nssa_recalc_test.go#L43) | unit/verify | unproven |

### [`RFC3101-3.1-1`](#rfc3101-3.1-1)

All NSSA border routers must set the E-bit in the Type-1 router-LSAs of their directly attached non-stub areas, even when they are not translating. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. c23 re-judge (independent judge). OSPFv2 as recorded at c22 (rfc3101_nssa_border_ebit_areas_test.go, both polarities, records on NSSABorderEBit). OSPFv3 now proven: rfc3101_v6_nssa_border_ebit_test.go builds an OSPFv3 engine (v6Codec) with backbone, area 0.0.0.1, normal 0.0.0.2 and stub 0.0.0.3, one passive running interface each, and drives v6OriginateSelf (no hand-set nssaBorderE); the E-bit is read as the literal 0x02 of the flags octet at offset 20 of the installed Router-LSA (RFC 5340 A.4.3: 0 0 0 Nt x V E B). Positive: with 0.0.0.1 NSSA, E set in 0, 0.0.0.1 and 0.0.0.2 with no external origination, clear in the stub. Negative: with 0.0.0.1 normal, E clear in all four. Overlays on NSSABorderEBit (scratch/c23/e1..e3): backbone-only reds the positive, dropping the stub check reds it, any-active-area reds the negative; TestOSPFv6OriginateRouterLSAABRNtBits green under all three. Both new units hold observed-red revert records. TestOSPFASBRBitFromNSSAType7 (#2) stays supplementary.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3101NoNSSAAttachedEBitClearEverywhere`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_border_ebit_areas_test.go#L75) | unit/verify | revert, verified |
| negative | [`TestRFC3101NSSABorderRouterSetsEBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_border_ebit_test.go#L48) | unit/verify | revert, verified |
| negative | [`TestOSPFASBRBitFromNSSAType7`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_origination_external_test.go#L76) | unit/verify | unproven |
| negative | [`TestRFC3101V6NoNSSAAttachedEBitClearEverywhere`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_v6_nssa_border_ebit_test.go#L94) | unit/verify | revert, verified |
| positive | [`TestRFC3101NSSABorderEBitInEveryNonStubArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_border_ebit_areas_test.go#L63) | unit/verify | revert, verified |
| positive | [`TestRFC3101NSSABorderRouterSetsEBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_border_ebit_test.go#L36) | unit/verify | revert, verified |
| positive | [`TestOSPFASBRBitFromNSSAType7`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_origination_external_test.go#L90) | unit/verify | unproven |
| positive | [`TestRFC3101V6NSSABorderEBitInEveryNonStubArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_v6_nssa_border_ebit_test.go#L75) | unit/verify | revert, verified |

### [`RFC3101-2.7-1`](#rfc3101-2.7-1)

In order for OSPF's summary routing to not be obscured by an NSSA's Type-7 AS-external-LSAs, all NSSA border router implementations must support the optional import of summary routes into NSSAs as Type-3 summary-LSAs. (§2.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a border router unable to import Type-3 summaries into an NSSA. spf/area_type_test.go TestOSPFNSSAType3SummaryImport asserts a regular NSSA keeps the inter-area Type-3 (line 88) and a no-summary NSSA suppresses it (line 99), proving the import is optional and supported.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_area_type_test.go#L96) | unit/verify | unproven |
| positive | [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_area_type_test.go#L85) | unit/verify | unproven |

### [`RFC3101-3.1-2`](#rfc3101-3.1-2)

An NSSA border router whose NSSA's NSSATranslatorRole is set to Candidate must maintain a list of the NSSA's border routers that are reachable both over the NSSA (Section 3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (OSPF judge, continuation 5): the row now quotes the verbatim span through 'reachable both over the NSSA' (R3 split); the transit-topology ASBR condition is gap row RFC3101-3.1-5, so no clause of this row is untested. electNSSATranslator skips a router the NSSA's completed SPF does not reach (ReachabilitySnapshot.RouterReachable). TestRFC3101UnreachableBorderRouterIsNotListed / TestRFC3101ReachableBorderRouterIsListed are an isolated pair differing only in reachability (translated 1 vs 0); TestOSPFNSSATranslatorElection adds the unreachable-higher case; five observed-red records re-recorded after the nssa.go comment edit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFNSSANoTranslateWhenNotElected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_test.go#L260) | unit/verify | revert, verified |
| negative | [`TestOSPFNSSATranslatorElection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_test.go#L67) | unit/verify | revert, verified |
| negative | [`TestRFC3101ReachableBorderRouterIsListed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_translator_list_test.go#L76) | unit/verify | revert, verified |
| positive | [`TestOSPFNSSATranslatorElection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_test.go#L64) | unit/verify | revert, verified |
| positive | [`TestRFC3101UnreachableBorderRouterIsNotListed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_translator_list_test.go#L47) | unit/verify | revert, verified |

### [`RFC3101-3.1-5`](#rfc3101-3.1-5)

An NSSA border router whose NSSA's NSSATranslatorRole is set to Candidate must maintain a list of the NSSA's border routers that are reachable both over the NSSA and as ASBRs over the AS's transit topology. (Section 3.1)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Judged 2026-09-30 (R3 split of 3.1-2): the list's second membership condition, 'as ASBRs over the AS's transit topology', is not implemented. electNSSATranslator lists every NSSA border router with B and Nt that the NSSA's completed SPF reaches and never consults the transit topology's ASBR reachability. Gap demo TestRFC3101BorderRouterNotReachedAsASBROverTransitIsNotListed (rfcgap.Demonstrate): 10.0.6.9 is reached in the NSSA, originates no router-LSA outside it and sets no E-bit, the body asserts the RFC outcome (candidate translates, 1 Type-5) and Ze gives 0. Disclosed on the row, the Support remaining text, docs/architecture/ospf/ospf-11-stub-nssa.md and rfc/corrections/rfc3101.md. Not counted conformant.

No test carries RFC3101-3.1-5, so no unit is bound to it.

### [`RFC3101-3.1-4`](#rfc3101-3.1-4)

If there exists another border router in this list whose router-LSA has bit Nt set or who has a higher router ID, then its NSSATranslatorState is disabled. (Section 3.1)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Re-judged 2026-09-30 (continuation 5): electNSSATranslator changed only in a comment (3.1-2 quote narrowed, names gap 3.1-5); the deviation is unchanged. Section 3.1 disables a candidate when another listed border router 'has bit Nt set or who has a higher router ID', and sets Nt only when NSSATranslatorState is enabled (Always role). Ze sets Nt on every non-never NSSA border router (setConfig ntAreas, instance.go) and nssaABRs lists only B|Nt routers, so a higher-RID Nt-clear router never disables the candidate. Gap demo TestRFC3101HigherRouterIDWithoutNtDisablesCandidate (rfcgap.Demonstrate) observes Ze translating; TestOSPFNSSANonCandidateDoesNotWedge pins the chosen behaviour untagged. Disclosed deviation, not counted conformant; OWNER-GATE item.

No test carries RFC3101-3.1-4, so no unit is bound to it.

### [`RFC3101-3.2-1`](#rfc3101-3.2-1)

The newly originated Type-5 LSA will describe the same network and have the same network mask, path type, metric, forwarding address and external route tag as the Type-7 LSA.  The advertising router field will be the router ID of this NSSA border router. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29. TestOSPFNSSATranslation now asserts the network mask as well as metric, E1, tag and FA, and the advertising router through the lookup key (translator Router ID) and the network through the Link State ID. New TestRFC3101TranslatedType5FollowsSourceFields re-originates the Type-7 with a new mask, E2, metric, FA and tag (clock advanced past MinLSInterval) and reads each new value in the translated Type-5, so a translator that hardcodes, defaults or keeps stale fields goes red. Revert records on translateNSSA for both. The withdraw and P=0 negatives stay (neighbouring section 3.2 steps).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFNSSAPbitNotTranslated`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_test.go#L188) | unit/verify | unproven |
| negative | [`TestOSPFNSSATranslation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_test.go#L142) | unit/verify | unproven |
| negative | [`TestRFC3101TranslatedType5FollowsSourceFields`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_translation_test.go#L19) | unit/verify | revert, verified |
| positive | [`TestOSPFNSSATranslation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_test.go#L127) | unit/verify | revert, verified |

### [`RFC3101-3.2-2`](#rfc3101-3.2-2)

the calculating router has the highest router ID amongst NSSA translators that have originated a functionally equivalent Type-5 LSA (i.e. same destination, cost and non-zero forwarding address) and that are reachable over area 0 and the NSSA, then a Type-5 LSA should be generated (§3.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Owner decision D-12: translator = router whose Router-LSA in the NSSA carries the B-bit (nssaABRs/nssaABRsV6 with B), equivalent = same destination and mask, same metric, same non-zero forwarding address (equivalentType5, equivalentType5V6 over lsdb.HigherRIDTranslatorExternals). TestRFC3101TranslatorYieldsOnlyToEquivalentType5 and its V6 twin assert the yield for an equivalent Type-5 and translation for a different metric, forwarding address, mask, or a non-translator advertiser; TestOSPFNSSAHigherRIDType5Suppresses and lsdb TestOSPFHigherRIDType5Exists keep the higher-RID ordering. Weak because the clause 'and that are reachable over area 0 and the NSSA' is neither implemented nor tested: the translator set is read from the NSSA LSDB, not from per-area SPF reachability.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFHigherRIDType5Exists`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_higher_rid_test.go#L29) | unit/verify | revert, verified |
| negative | [`TestOSPFNSSAHigherRIDType5Suppresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_ac14_16_test.go#L135) | unit/verify | revert, verified |
| negative | [`TestRFC3101TranslatorYieldsOnlyToEquivalentType5`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_translator_equivalence_test.go#L54) | unit/verify | revert, verified |
| negative | [`TestRFC3101TranslatorYieldsOnlyToEquivalentType5V6`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_translator_equivalence_test.go#L117) | unit/verify | revert, verified |
| positive | [`TestOSPFHigherRIDType5Exists`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_nssa_higher_rid_test.go#L38) | unit/verify | revert, verified |
| positive | [`TestOSPFNSSAHigherRIDType5Suppresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_ac14_16_test.go#L119) | unit/verify | revert, verified |
| positive | [`TestRFC3101TranslatorYieldsOnlyToEquivalentType5`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_translator_equivalence_test.go#L60) | unit/verify | revert, verified |
| positive | [`TestRFC3101TranslatorYieldsOnlyToEquivalentType5V6`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_translator_equivalence_test.go#L123) | unit/verify | revert, verified |

### [`RFC3101-x-3`](#rfc3101-x-3)

Implementations must provide a vehicle for setting the P-bit when external routes are imported into the NSSA as Type-7 LSAs. (§D)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: no configuration vehicle for the P-bit. nssa_import_lifecycle_test.go TestNSSAPerSourcePropagationEnabled parses nssa-propagate true and asserts P set on the imported route and default (line 204); TestNSSAPerSourcePropagationDisabled asserts P clear for explicit false and unconfigured sources (line 226); TestNSSAPerSourcePropagationCannotOverrideType5 asserts P clear with a Type-5 twin (line 251).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNSSAPerSourcePropagationCannotOverrideType5`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_import_lifecycle_test.go#L234) | unit/verify | unproven |
| negative | [`TestNSSAPerSourcePropagationDisabled`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_import_lifecycle_test.go#L212) | unit/verify | unproven |
| positive | [`TestNSSAPerSourcePropagationEnabled`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_import_lifecycle_test.go#L187) | unit/verify | unproven |

### [`RFC3101-x-4`](#rfc3101-x-4)

For NSSAs the ExternalRoutingCapability area configuration parameter must be set to accept Type-7 external routes. (§D)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an NSSA area that discards Type-7. lsdb/area_capability_rfc3101_test.go TestRFC3101NSSAAreaAcceptsType7 fails if a received Type-7 is not installed in an NSSA area; TestRFC3101NormalAreaRejectsType7 fails if the same LSA is installed in a normal area.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3101NormalAreaRejectsType7`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_area_capability_test.go#L55) | unit/verify | revert, verified |
| positive | [`TestRFC3101NSSAAreaAcceptsType7`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3101_area_capability_test.go#L45) | unit/verify | revert, verified |

### [`RFC3101-x-5`](#rfc3101-x-5)

Additionally there must be a way of configuring the metric of the default LSA that a border router advertises into its directly attached NSSAs. (§D)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (continuation 6, judge): both halves proven on a border router through config. Type-7 half: TestRFC3101BorderRouterDefaultCarriesConfiguredMetric (continuation 5). Type-3 half (no-summary NSSA, s2.7): TestRFC3101BorderRouterType3DefaultCarriesConfiguredMetric (config JSON default-cost 77 and 5000 on two no-summary NSSAs -> setConfig -> SPF run -> the self-originated Type-3 default, mask 0, carries 77 and 5000) and negative TestRFC3101BorderRouterType3DefaultMetricIsNotBorrowed (unconfigured sibling carries DefaultAreaCost, not 77). Revert records on applyAreaTypePolicy observed red for both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3101BorderRouterDefaultMetricIsNotBorrowed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_border_default_cost_test.go#L73) | unit/verify | revert, verified |
| negative | [`TestOSPFNSSADefaultMetricIsNotFixed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_default_cost_test.go#L55) | unit/verify | revert, verified |
| negative | [`TestRFC3101BorderRouterType3DefaultMetricIsNotBorrowed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_type3_default_cost_test.go#L84) | unit/verify | revert, verified |
| positive | [`TestRFC3101BorderRouterDefaultCarriesConfiguredMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_border_default_cost_test.go#L60) | unit/verify | revert, verified |
| positive | [`TestOSPFNSSADefaultCarriesConfiguredMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_default_cost_test.go#L44) | unit/verify | revert, verified |
| positive | [`TestRFC3101BorderRouterType3DefaultCarriesConfiguredMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_nssa_type3_default_cost_test.go#L71) | unit/verify | revert, verified |

### [`RFC3101-2.7-2`](#rfc3101-2.7-2)

When OSPF's summary routes are not imported, the default LSA originated by an NSSA border router into the NSSA should be a Type-3 summary-LSA. (§2.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a no-summary NSSA default that is not a Type-3. spf/area_type_test.go TestOSPFNSSANoSummaryDefaultInjection requires the Type-3 default through OriginateSummaries (line 133); origination_v6_nssa_default_test.go TestOSPFv3NSSANoSummaryDefaultUsesSummaryLSA asserts no NSSA-LSA default in the no-summary NSSA (line 101) and the inter-area default (line 112). Negatives: a regular NSSA gets no Type-3 default (area_type_test.go lines 94, 154).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFv3NSSANoSummaryDefaultUsesSummaryLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_origination_v6_nssa_default_test.go#L99) | unit/verify | unproven |
| negative | [`TestOSPFNSSANoSummaryDefaultInjection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_area_type_test.go#L149) | unit/verify | unproven |
| negative | [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_area_type_test.go#L90) | unit/verify | unproven |
| positive | [`TestOSPFv3NSSANoSummaryDefaultUsesSummaryLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_origination_v6_nssa_default_test.go#L105) | unit/verify | unproven |
| positive | [`TestOSPFNSSANoSummaryDefaultInjection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_area_type_test.go#L126) | unit/verify | unproven |
| positive | [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_area_type_test.go#L101) | unit/verify | unproven |

### [`RFC3101-2.7-3`](#rfc3101-2.7-3)

When summary routes are imported into the NSSA, the default LSA originated by an NSSA border router must not be a Type-3 summary-LSA (§2.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Type-3 default into an NSSA that imports summaries. spf/area_type_test.go TestOSPFNSSANoSummaryDefaultInjection/regular asserts no summary default through OriginateSummaries (line 154), TestOSPFNSSAType3SummaryImport asserts it through applyAreaTypePolicy (line 94), and the v3 test asserts no inter-area default (line 121). Negatives show the ban is conditional on import (area_type_test.go lines 107, 133).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFv3NSSANoSummaryDefaultUsesSummaryLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_origination_v6_nssa_default_test.go#L108) | unit/verify | revert, verified |
| negative | [`TestOSPFNSSANoSummaryDefaultInjection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_area_type_test.go#L130) | unit/verify | revert, verified |
| negative | [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_area_type_test.go#L105) | unit/verify | revert, verified |
| positive | [`TestOSPFv3NSSANoSummaryDefaultUsesSummaryLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3101_origination_v6_nssa_default_test.go#L117) | unit/verify | revert, verified |
| positive | [`TestOSPFNSSANoSummaryDefaultInjection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_area_type_test.go#L151) | unit/verify | revert, verified |
| positive | [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc3101_area_type_test.go#L92) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-27 |
| Register | prose |
| Source | rfc/full/rfc3101.txt |
| Source fingerprint | 7061642290964ece |
| Record | rfc/extraction/rfc3101.json |
| Mapped sentences | 25 |
| Declined as scope | 15 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1.0` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 1 | walked | not stated |
| `1.2` | not stated | 1 | walked | not stated |
| `1.3` | not stated | 3 | walked | not stated |
| `2.0` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 3 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `2.3` | not stated | 5 | walked | not stated |
| `2.4` | not stated | 5 | walked | not stated |
| `2.5` | not stated | 5 | walked | not stated |
| `2.6` | not stated | 0 | walked | not stated |
| `2.7` | not stated | 2 | walked | not stated |
| `3.0` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 4 | walked | not stated |
| `3.2` | not stated | 1 | walked | not stated |
| `3.3` | not stated | 0 | walked | not stated |
| `4.0` | not stated | 1 | walked | not stated |
| `5.0` | not stated | 0 | walked | not stated |
| `6.0` | not stated | 0 | walked | not stated |
| `7.0` | not stated | 0 | walked | not stated |
| `A` | Appendix A | 1 | walked | Appendix A. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of section 7.0, where its 1 site(s) were walked; every decision is carried forward by its verbatim quote. |
| `B` | Appendix B | 1 | walked | Appendix B. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of section 7.0, where its 1 site(s) were walked; every decision is carried forward by its verbatim quote. |
| `C` | Appendix C | 2 | walked | Appendix C. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of section 7.0, where its 2 site(s) were walked; every decision is carried forward by its verbatim quote. |
| `D` | Appendix D | 3 | walked | Appendix D. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of section 7.0, where its 3 site(s) were walked; every decision is carried forward by its verbatim quote. |
| `E` | Appendix E | 0 | walked | Appendix E. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `F` | Appendix F | 0 | walked | Appendix F. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of section 7.0. Since the same day its subsections F.1 to F.6, written without a trailing dot, open their own sections, and the sites once walked here sit in them, each decision carried forward by its verbatim quote. |
| `F.1` | Appendix subsection F.1 | 1 | walked | Appendix subsection F.1. Until 2026-09-27 the heading reader did not read a column-0 appendix subsection heading written without a trailing dot, so its text was read as part of section F, where its 1 site(s) were walked; every decision is carried forward by its verbatim quote. |
| `F.2` | Appendix subsection F.2 | 0 | walked | Appendix subsection F.2. Until 2026-09-27 the heading reader did not read a column-0 appendix subsection heading written without a trailing dot, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `F.3` | Appendix subsection F.3 | 0 | walked | Appendix subsection F.3. Until 2026-09-27 the heading reader did not read a column-0 appendix subsection heading written without a trailing dot, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `F.4` | Appendix subsection F.4 | 0 | walked | Appendix subsection F.4. Until 2026-09-27 the heading reader did not read a column-0 appendix subsection heading written without a trailing dot, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `F.5` | Appendix subsection F.5 | 0 | walked | Appendix subsection F.5. Until 2026-09-27 the heading reader did not read a column-0 appendix subsection heading written without a trailing dot, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `F.6` | Appendix subsection F.6 | 1 | walked | Appendix subsection F.6. Until 2026-09-27 the heading reader did not read a column-0 appendix subsection heading written without a trailing dot, so its text was read as part of section F, where its 1 site(s) were walked; every decision is carried forward by its verbatim quote. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1.1 is the transit-network motivation: the sentence describes the deployment problem NSSA exists to solve and binds no implementation. | In order to run OSPF out to BR18, BR18 must be a member of a non-stub area or the OSPF backbone before it can import routes other than its directly connected network(s). |
| `1.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1.2 is the corporate-network motivation: the sentence states what any solution has to achieve, as a design goal, not an obligation on an implementation. | Any solution to this dilemma must also honor Area 1's path of choice to 192.168.192/20 through A0 with redundancy through B0 while at the same time honoring Area 2's path of choice to 192.168.208/20 through B0 with redundancy through A0. |
| `1.3:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 1.3 is the overview; it restates the border-router default-origination obligation Section 2.4 states. | Default routes are necessary because NSSAs do not receive full routing information and must have a default route in order to route to AS-external destinations. |
| `2.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The sentence quotes [OSPF] Section 10.5 to state the E-bit condition the Section 2.1 Hello check verifies. | As explained in [OSPF] Section 10.5, if Type-5 LSAs are not flooded into/throughout the area, the E-bit must be clear in the option field of the received Hello packets. |
| `2.1:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 2.1 restates the Appendix A rule that the E-bit is clear whenever the N-bit is set. | Therefore, if the N-bit is set in the options field, the E-bit must be clear. |
| `2.3:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The sentence restates the non-zero forwarding address obligation Section 2.4 states for a P-bit Type-7 LSA. | Those Type-7 LSAs that are to be translated into Type-5 LSAs must have their forwarding address set. |
| `2.3:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Explanatory aside: it says non-zero forwarding addresses ease translation because border routers are not required to compute them, and states no obligation. | Also the non-zero forwarding addresses of Type-7 LSAs ease the process of their translation into Type-5 LSAs, as NSSA border routers are not required to compute them. |
| `2.4:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | "The LSAs of these networks must have a valid non-zero forwarding address" restates the same obligation in the same section. | The LSAs of these networks must have a valid non-zero forwarding address. |
| `2.5:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The zero-forwarding-address case restates the rule that the ASBR entry is chosen from the originating NSSA. | Since a Type-7 LSA only has area-wide flooding scope, when its forwarding address is set to 0.0.0.0, its ASBR's routing table entry must be chosen from the originating NSSA. |
| `4.0:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 4.0 describes what the [DIGI] document contains; it is a pointer, not an obligation of this document. | [DIGI] describes the extensions to OSPF required to add digital signature authentication to Link State data and to provide a certification mechanism for router data. |
| `B:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | Appendix B describes the Type-1 router-LSA and defers outright: "For details concerning the construction of router-LSAs, see [OSPF] Section 12.4.1." The single-router-LSA rule is RFC 2328's. | All of the router's links to the area must be described in a single router-LSA. |
| `C:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Appendix C's Type-7 packet format restates the Section 2.4 P-bit origination rule. | The Options field must have the N/P bit set as described in Appendix A when the originating router desires that the external route be propagated throughout the OSPF domain. |
| `C:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Appendix C restates the Section 2.4 forwarding address rule word for word. | If the P-bit is set, the forwarding address must be non-zero, otherwise it may be 0.0.0.0. |
| `F.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Appendix F's differences list restates the Section 2.7 no-summary default rule. | When summary routes are not imported into an NSSA, the default LSA originated by its border routers must be a Type-3 summary-LSA. |
| `F.6:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Internet Society copyright boilerplate. | However, this document itself may not be modified in any way, such as by removing the copyright notice or references to the Internet Society or other Internet organizations, except as needed for the purpose of developing Internet standards in which case the procedures for copyrights defined in the Internet Standards process must be followed, or as required to translate it into languages other than English. |

## Superseded

No document obsoletes RFC 3101, so its obligations are stated where they were written.
