# RFC 5250 - The OSPF Opaque LSA Option

Experimental. Every requirement this repository extracted from RFC 5250, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 92.9% | 13 of 14 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 7.1% | 1 of 14 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 14 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 14 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 14 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 65.1% | 28 of 43 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 14 | of 16 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 14 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 14 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 14 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 14 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 14 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 16 |
| Gated MUST-level | 14 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 43 |
| Tagged units | 43 |
| Recorded audit verdicts | 11 |
| Discrimination records | 28 |
| Summary | `rfc/short/rfc5250.md` |
| Requirement shard | `rfc/requirements/rfc5250.md` |
| RFC text | `rfc/full/rfc5250.txt` |

## Enrolment

Enrolled: OSPFv2 opaque LSAs: scope-specific storage/flooding, O-bit negotiation, opaque identity, originator reachability and Database Exchange filtering, including virtual neighbours.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

Opaque LSA framework and retention.

**What the ledger says remains:**

Same OSPF experimental status.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 13 | one part of the gated population |
| Annotated (including scoped evidence) | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **14** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (13):** [`RFC5250-3-1`](#rfc5250-3-1), [`RFC5250-3.1-1`](#rfc5250-3.1-1), [`RFC5250-3.1-2`](#rfc5250-3.1-2), [`RFC5250-3.1-3`](#rfc5250-3.1-3), [`RFC5250-3.1-4`](#rfc5250-3.1-4), [`RFC5250-3.1-5`](#rfc5250-3.1-5), [`RFC5250-5-1`](#rfc5250-5-1), [`RFC5250-5-2`](#rfc5250-5-2), [`RFC5250-5-3`](#rfc5250-5-3), [`RFC5250-3.2-1`](#rfc5250-3.2-1), [`RFC5250-3.2-2`](#rfc5250-3.2-2), [`RFC5250-3.2-3`](#rfc5250-3.2-3), [`RFC5250-3.2-4`](#rfc5250-3.2-4)

**Annotated (including scoped evidence) (1):** [`RFC5250-3-2`](#rfc5250-3-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5250-3-1` | Opaque LSAs are types 9, 10, and 11 link state advertisements. Opaque LSAs consist of a standard LSA header followed by a 32-bit aligned application-specific information field. Standard link-state database flooding mechanisms are used for distribution of Opaque LSAs. The range of topological distribution (i.e., the flooding scope) of an Opaque LSA is identified by its link-state type. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestLSTypeKnownValues`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/rfc5250_lstype_test.go#L38). **positive:** `unit/verify` [`TestOpaqueScopeRouting`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_opaque_scope_test.go#L73). **positive:** `unit/verify` [`TestRFC5250AlignedOpaqueStoredAndFlooded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_opaque_format_flood_test.go#L68). **negative:** `unit/verify` [`TestLSTypeKnownValues`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/rfc5250_lstype_test.go#L25). **negative:** `unit/verify` [`TestOpaqueScopeRouting`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_opaque_scope_test.go#L88). **negative:** `unit/verify` [`TestRFC5250DuplicateOpaqueNotReflooded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_opaque_format_flood_test.go#L146). **negative:** `unit/verify` [`TestRFC5250UnalignedOpaqueBodyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_opaque_format_flood_test.go#L112) |
| `RFC5250-3.1-1` | o If the Opaque LSA is type-9 (the flooding scope is link-local) and the interface that the LSA was received on is not the same as the target interface (e.g., the interface associated with a particular target neighbor), the Opaque LSA MUST be discarded and not acknowledged. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestOpaqueType9WrongInterfaceDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_opaque_scope_test.go#L116). **positive:** `unit/verify` [`TestRFC5250Type9AcknowledgedOnArrivalInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_type9_ack_test.go#L57). **negative:** `unit/verify` [`TestOpaqueType9WrongInterfaceDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_opaque_scope_test.go#L120). **negative:** `unit/verify` [`TestRFC5250Type9NotAcknowledgedOnOtherInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_type9_ack_test.go#L69) |
| `RFC5250-3.1-2` | If the Opaque LSA is type-10 (the flooding scope is area-local) and the area associated with the Opaque LSA (as identified during origination or from a received LSA's associated OSPF packet header) is not the same as the area associated with the target interface, the Opaque LSA MUST be discarded and not acknowledged. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestOpaqueScopeRouting`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_opaque_scope_test.go#L63). **positive:** `unit/verify` [`TestOpaqueType10ConfinedToItsArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_opaque_scope_test.go#L203). **negative:** `unit/verify` [`TestOpaqueType10ConfinedToItsArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_opaque_scope_test.go#L207) |
| `RFC5250-3.1-3` | If the Opaque LSA is type-11 (the LSA is flooded throughout the AS) and the target interface is associated with a stub area or NSSA, the Opaque LSA MUST NOT be flooded out the interface. A type-11 Opaque LSA that is received on an interface associated with a stub area or NSSA MUST be discarded and not acknowledged (the neighboring router has flooded the LSA in error). (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestOpaqueType11StubDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_opaque_scope_test.go#L161). **negative:** `unit/verify` [`TestOpaqueType11StubDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_opaque_scope_test.go#L152) |
| `RFC5250-3.1-4` | Opaque LSAs are only flooded to opaque-capable neighbors. To be more precise, in Section 13.3 of [OSPF], Opaque LSAs MUST be placed on the link-state retransmission lists of opaque-capable neighbors and MUST NOT be placed on the link-state retransmission lists of non-opaque-capable neighbors. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestOpaqueFloodOnlyToOpaqueNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_opaque_flood_test.go#L38). **negative:** `unit/verify` [`TestOpaqueFloodOnlyToOpaqueNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_opaque_flood_test.go#L42) |
| `RFC5250-3-2` | The link-state ID of the Opaque LSA is divided into an Opaque type field (the first 8 bits) and a type-specific ID (the remaining 24 bits). (§3) | MUST | 3 | **positive:** `unit/verify` [`TestOpaqueLinkStateIDSplit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc5250_lsa_opaque_test.go#L19). **negative:** no negative test. **{single-polarity}:** the Link State ID split is a pure bit operation (high octet is the Opaque Type, low 24 bits the Opaque ID) with no validation or reject path, so there is no negative behavior to drive |
| `RFC5250-3.1-5` | the O-bit SHOULD NOT be set and MUST be ignored when received in packets other than Database Description packets. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestOpaqueBitIgnoredOutsideDD`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc5250_dd_opaque_test.go#L74). **positive:** `unit/verify` [`TestRFC5250HelloSentWithoutOBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5250_obit_hello_test.go#L77). **negative:** `unit/verify` [`TestOpaqueBitIgnoredOutsideDD`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc5250_dd_opaque_test.go#L63). **negative:** `unit/verify` [`TestRFC5250HelloOBitIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5250_obit_hello_test.go#L94) |
| `RFC5250-5-1` | (2) When processing a received type-11 Opaque LSA, the router MUST look up the routing table entries (potentially one per attached area) for the ASBR that originated the LSA. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestOpaqueType11UnreachableOriginatorNotUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5250_opaque_reachability_test.go#L51). **positive:** `unit/verify` [`TestRFC5250Type11LooksUpOriginatorRoutingEntry`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5250_asbr_lookup_test.go#L65). **negative:** `unit/verify` [`TestOpaqueType11UnreachableOriginatorNotUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5250_opaque_reachability_test.go#L44). **negative:** `unit/verify` [`TestRFC5250Type11LooksUpOriginatorRoutingEntry`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5250_asbr_lookup_test.go#L49) |
| `RFC5250-5-2` | It also MUST discontinue using all Opaque LSAs injected into the network by the same originator whenever it is detected that the originator is unreachable. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestOpaqueType11UnreachableOriginatorNotUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5250_opaque_reachability_test.go#L52). **positive:** `unit/verify` [`TestRFC5250DiscontinueOpaqueFromUnreachableOriginator`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5250_discontinue_test.go#L51). **negative:** `unit/verify` [`TestRFC5250DiscontinueOpaqueFromUnreachableOriginator`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5250_discontinue_test.go#L64) |
| `RFC5250-5-3` | (1) An OSPF router that is configured to originate AS-scope opaque LSAs will advertise itself as an ASBR and MUST follow the requirements related to setting of the Options field E-bit in OSPF LSA headers as specified in [OSPF]. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC5250TwoRoutersType11OriginatorReachable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5250_asbr_originator_test.go#L115). **positive:** `unit/verify` [`TestRFC5250Type11OriginatorAdvertisesASBR`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_asbr_ebit_test.go#L59). **negative:** `unit/verify` [`TestRFC5250Type11OriginatorAdvertisesASBR`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_asbr_ebit_test.go#L45) |
| `RFC5250-3.2-1` | The router MUST list the contents of its entire area link-state database in the neighbor Database summary list. The area link-state database consists of the Router LSAs, Network LSAs, Summary LSAs, type-9 Opaque LSAs, and type-10 Opaque LSAs contained in the area structure, along with AS External and type-11 Opaque LSAs contained in the global structure. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC5250SummaryListsEntireAreaDatabase`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc5250_summary_entire_area_test.go#L111). **positive:** `unit/verify` [`TestRFC5250SummaryListsOpaqueAndGlobalLSAs`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_dd_summary_test.go#L24). **negative:** `unit/verify` [`TestRFC5250SummaryOmitsGlobalLSAsInStubAndNSSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_dd_summary_test.go#L58). **negative:** `unit/verify` [`TestRFC5250SummaryOmitsOtherAreaAndLinkLSAs`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc5250_summary_entire_area_test.go#L124) |
| `RFC5250-3.2-2` | AS External and type-11 Opaque LSAs MUST be omitted from a virtual neighbor's Database summary list (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC5250VirtualDatabaseExchangeOmitsASScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5250_virtual_exchange_test.go#L68). **negative:** `unit/verify` [`TestRFC5250PhysicalDatabaseExchangeRetainsASScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5250_virtual_exchange_test.go#L85) |
| `RFC5250-3.2-3` | AS External LSAs and type-11 Opaque LSAs MUST be omitted from the Database summary list if the area has been configured as a stub area or NSSA (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC5250SummaryOmitsGlobalLSAsInStubAndNSSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_dd_summary_test.go#L54). **negative:** `unit/verify` [`TestRFC5250SummaryListsOpaqueAndGlobalLSAs`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_dd_summary_test.go#L27) |
| `RFC5250-3.2-4` | Type-9 Opaque LSAs MUST be omitted from the Database summary list if the interface associated with the neighbor is not the interface associated with the Opaque LSA (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC5250SummaryListsOwnInterfaceType9`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc5250_dd_summary_test.go#L33). **negative:** `unit/verify` [`TestRFC5250SummaryOmitsOtherInterfaceType9`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc5250_dd_summary_test.go#L61) |
| `RFC5250-3.1-6` | A neighbor is opaque-capable if and only if it sets the O-bit in the Options field of its Database Description packets; the O-bit SHOULD NOT be set and MUST be ignored when received in packets other than Database Description packets. (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5250-8-1` | The frequency at which new LSA instances may be originated is set equal to once every MinLSInterval seconds, whose value is 5 seconds (see Section 12.4 of [OSPF]). The frequency at which new LSA instances are accepted during flooding is once every MinLSArrival seconds, whose value is set to 1 (see Section 13, Appendix B, and G.5 of [OSPF]). (§8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 5250 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5250-3-1`](#rfc5250-3-1)

Opaque LSAs are types 9, 10, and 11 link state advertisements. Opaque LSAs consist of a standard LSA header followed by a 32-bit aligned application-specific information field. Standard link-state database flooding mechanisms are used for distribution of Opaque LSAs. The range of topological distribution (i.e., the flooding scope) of an Opaque LSA is identified by its link-state type. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Every clause both polarities. LS types 9/10/11 as Opaque: lstype_test.go TestLSTypeKnownValues (+ opaque types IsOpaque, - standard types not), revert records on IsOpaque. Scope by LS type: TestOpaqueScopeRouting (+ Type 11 AS-wide, - Type 9 refused an area store), revert records on Install. 32-bit aligned body: TestRFC5250AlignedOpaqueStoredAndFlooded (+ 8-octet body stored, flooded out eth1 at length 28 with body intact, on the retransmission list) and TestRFC5250UnalignedOpaqueBodyDiscarded (- 5-octet body with a valid checksum not stored, acked or flooded; aligned companion in the same update stored), red on the pre-fix ReceiveUpdate (D-8 fixed at lsdb/flooding.go ReceiveUpdate). Standard flooding: the aligned unit plus TestRFC5250DuplicateOpaqueNotReflooded (- the same instance after MinLSArrival is acked, not reflooded), red under an equal-copies-reflooded overlay. Revert records on ReceiveUpdate for all three new units.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5250DuplicateOpaqueNotReflooded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_opaque_format_flood_test.go#L146) | unit/verify | revert, verified |
| negative | [`TestRFC5250UnalignedOpaqueBodyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_opaque_format_flood_test.go#L112) | unit/verify | revert, verified |
| negative | [`TestOpaqueScopeRouting`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_opaque_scope_test.go#L88) | unit/verify | revert, verified |
| negative | [`TestLSTypeKnownValues`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/rfc5250_lstype_test.go#L25) | unit/verify | revert, verified |
| positive | [`TestRFC5250AlignedOpaqueStoredAndFlooded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_opaque_format_flood_test.go#L68) | unit/verify | revert, verified |
| positive | [`TestOpaqueScopeRouting`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_opaque_scope_test.go#L73) | unit/verify | revert, verified |
| positive | [`TestLSTypeKnownValues`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/rfc5250_lstype_test.go#L38) | unit/verify | revert, verified |

### [`RFC5250-3.1-1`](#rfc5250-3.1-1)

o If the Opaque LSA is type-9 (the flooding scope is link-local) and the interface that the LSA was received on is not the same as the target interface (e.g., the interface associated with a particular target neighbor), the Opaque LSA MUST be discarded and not acknowledged. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (independent judge, ospf c27). The 'not acknowledged' clause is now asserted: lsdb/rfc5250_type9_ack_test.go receives a Type 9 from 2.2.2.2 on eth0 through ReceiveUpdate (eth0, eth1 in area 0, Full neighbors), flushes both interfaces' delayed acks and decodes the sent packets. + TestRFC5250Type9AcknowledgedOnArrivalInterface: an LSAck carrying the header leaves eth0 (same-interface case); - TestRFC5250Type9NotAcknowledgedOnOtherInterface: no LSAck and no LS Update carrying it leaves eth1. Judge replayed overlays on flooding.go ackForReceive: ack queued on eth1 -> + and - red ('acknowledged out eth1'); ack dropped -> only + red. Discard clause: TestOpaqueType9WrongInterfaceDiscarded (not in the eth1 store, not flooded, not retransmit-queued), now with + and - revert records on link_scope.go installLinkLocked. All four records observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpaqueType9WrongInterfaceDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_opaque_scope_test.go#L120) | unit/verify | revert, verified |
| negative | [`TestRFC5250Type9NotAcknowledgedOnOtherInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_type9_ack_test.go#L69) | unit/verify | revert, verified |
| positive | [`TestOpaqueType9WrongInterfaceDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_opaque_scope_test.go#L116) | unit/verify | revert, verified |
| positive | [`TestRFC5250Type9AcknowledgedOnArrivalInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_type9_ack_test.go#L57) | unit/verify | revert, verified |

### [`RFC5250-3.1-2`](#rfc5250-3.1-2)

If the Opaque LSA is type-10 (the flooding scope is area-local) and the area associated with the Opaque LSA (as identified during origination or from a received LSA's associated OSPF packet header) is not the same as the area associated with the target interface, the Opaque LSA MUST be discarded and not acknowledged. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Type 10 flooded to or read from an area other than its own. TestOpaqueType10ConfinedToItsArea fatals if the LSA is readable (LookupLSA or Lookup) from area 0.0.0.2, if floodExcept emits anything but exactly one send, on eth0, which also refuses any acknowledgement out the other area interface, or if the eth1 neighbor is queued to retransmit it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpaqueType10ConfinedToItsArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_opaque_scope_test.go#L207) | unit/verify | unproven |
| positive | [`TestOpaqueScopeRouting`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_opaque_scope_test.go#L63) | unit/verify | unproven |
| positive | [`TestOpaqueType10ConfinedToItsArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_opaque_scope_test.go#L203) | unit/verify | unproven |

### [`RFC5250-3.1-3`](#rfc5250-3.1-3)

If the Opaque LSA is type-11 (the LSA is flooded throughout the AS) and the target interface is associated with a stub area or NSSA, the Opaque LSA MUST NOT be flooded out the interface. A type-11 Opaque LSA that is received on an interface associated with a stub area or NSSA MUST be discarded and not acknowledged (the neighboring router has flooded the LSA in error). (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Clause 1 forbidden: a Type 11 flooded out a stub interface; TestOpaqueType11StubDiscarded fatals if floodExcept sends on eth0 (stub). Clause 2 forbidden: a Type 11 received on a stub interface installed or acknowledged; the same test fatals if it is found in the store or if any packet at all is sent (len(tx.sends) != 0, which covers an LSAck).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpaqueType11StubDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_opaque_scope_test.go#L152) | unit/verify | unproven |
| positive | [`TestOpaqueType11StubDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_opaque_scope_test.go#L161) | unit/verify | unproven |

### [`RFC5250-3.1-4`](#rfc5250-3.1-4)

Opaque LSAs are only flooded to opaque-capable neighbors. To be more precise, in Section 13.3 of [OSPF], Opaque LSAs MUST be placed on the link-state retransmission lists of opaque-capable neighbors and MUST NOT be placed on the link-state retransmission lists of non-opaque-capable neighbors. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an opaque LSA placed on the retransmission list of a non-opaque-capable neighbor, or missing from an opaque-capable one. The opaque_flood_test.go unit fatals if db.retransmit lacks the Type 11 for 2.2.2.2 (OpaqueCapable) or holds it for 3.3.3.3 (not capable), and shows a Router LSA still reaches both.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpaqueFloodOnlyToOpaqueNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_opaque_flood_test.go#L42) | unit/verify | unproven |
| positive | [`TestOpaqueFloodOnlyToOpaqueNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_opaque_flood_test.go#L38) | unit/verify | unproven |

### [`RFC5250-3-2`](#rfc5250-3-2)

The link-state ID of the Opaque LSA is divided into an Opaque type field (the first 8 bits) and a type-specific ID (the remaining 24 bits). (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Link State ID split other than 8-bit type and 24-bit ID. TestOpaqueLinkStateIDSplit fatals if OpaqueTypeOf(AA BB CC DD) != 0xAA or OpaqueIDOf != 0x00BBCCDD, on a failed round trip, on a high id byte leaking into the type, and at the 0xFF/0xFFFFFF boundary. Row carries {single-polarity: positive}.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestOpaqueLinkStateIDSplit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc5250_lsa_opaque_test.go#L19) | unit/verify | unproven |

### [`RFC5250-3.1-5`](#rfc5250-3.1-5)

the O-bit SHOULD NOT be set and MUST be ignored when received in packets other than Database Description packets. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (independent judge, ospf c28). New rfc5250_obit_hello_test.go: real engine, opaque on, p2p eth0, recording transport; a wire Hello from 10.0.0.2 with Options E|O dispatched through eng.dispatch, waits for a sent Hello and a sent DD (both required, so not vacuous). + TestRFC5250HelloSentWithoutOBit (SHOULD NOT be set): every sent Hello has O clear while the DD has O set; - TestRFC5250HelloOBitIgnoredOnReceipt (MUST be ignored): neighbor 10.0.0.2 not OpaqueCapable after the O-bit Hello with no DD from it. Author overlay logs read (c28/o6, o7): Hello built with O -> + red only ('options 0x42'); neighbor Options get O on Hello -> - red only. HelloInput carries no Options, so today the received O-bit is dropped at the engine boundary; the end-to-end negative catches any future path that carries it into the neighbor. HEAD dd_opaque_test units (DD O-bit learned / Hello exchange) stay as supplementary. Revert records on buildHelloPacketLocked and neighbor/table.go hello observed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpaqueBitIgnoredOutsideDD`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc5250_dd_opaque_test.go#L63) | unit/verify | unproven |
| negative | [`TestRFC5250HelloOBitIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5250_obit_hello_test.go#L94) | unit/verify | revert, verified |
| positive | [`TestOpaqueBitIgnoredOutsideDD`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc5250_dd_opaque_test.go#L74) | unit/verify | unproven |
| positive | [`TestRFC5250HelloSentWithoutOBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5250_obit_hello_test.go#L77) | unit/verify | revert, verified |

### [`RFC5250-5-1`](#rfc5250-5-1)

(2) When processing a received type-11 Opaque LSA, the router MUST look up the routing table entries (potentially one per attached area) for the ASBR that originated the LSA. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (independent judge, c12) after the D-8 fix. Section 5 (2): 'the router MUST look up the routing table entries (potentially one per attached area) for the ASBR that originated the LSA. If no entries exist for the ASBR (i.e., the ASBR is unreachable), the router MUST do nothing with this LSA.' Producer: spf/reachability.go ASBRReachable reads the SPF border entries of Kind asbr (intra-area Router-LSA E-bit, interarea.go intraAreaBorderRouters; inter-area Type-4 summaries, interarea.go:157), finite metric and a next hop, the same entries the Type-5 calculation reads (external.go); Computer.ASBRReachable gates on the config generation; opaque.go spfASBRReachable is the only production caller (grep: no other RouterReachable user changed; bgpls_export and nssa keep ReachabilitySnapshot.RouterReachable/RouterReachableAny). TestRFC5250Type11LooksUpOriginatorRoutingEntry over the engine's own seam: - pending SPF, reached router without the E-bit, unreached 10.0.0.9 all delivered not usable; + the same router re-originated with the E-bit is usable. Judge overlay (go test -overlay) restoring the old RouterReachableAny answer in Computer.ASBRReachable: red at the no-E-bit negative (scratch/judge-ospf-5250mut.log); author's red at HEAD agrees. Records +/- revert on ASBRReachable observed. The inter-area Type-4 source is not driven by this unit; it feeds the same border slice. opaque_reachability_test.go tags are supplementary (injected predicate). Related defect reported to the main thread: Ze does not set its own E-bit when it originates Type-11 LSAs (lsdb selfIsASBRLocked counts only Type-5/7), against Section 5 (1), so another Ze router now treats those LSAs as unusable.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5250Type11LooksUpOriginatorRoutingEntry`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5250_asbr_lookup_test.go#L49) | unit/verify | revert, verified |
| negative | [`TestOpaqueType11UnreachableOriginatorNotUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5250_opaque_reachability_test.go#L44) | unit/verify | unproven |
| positive | [`TestRFC5250Type11LooksUpOriginatorRoutingEntry`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5250_asbr_lookup_test.go#L65) | unit/verify | revert, verified |
| positive | [`TestOpaqueType11UnreachableOriginatorNotUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5250_opaque_reachability_test.go#L51) | unit/verify | unproven |

### [`RFC5250-5-2`](#rfc5250-5-2)

It also MUST discontinue using all Opaque LSAs injected into the network by the same originator whenever it is detected that the originator is unreachable. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (independent judge, c11). D-8 fixed at the producer: the Extended Prefix receiver cached usable at delivery (extRecvEntry.usable), so a Type-11 entry stayed usable in `show` after its originator became unreachable; the entry now stores scope only and ext_prefix.go::extPrefixUsable (RFC quote above the return) judges it at every read, snapshot deriving it after releasing the receiver lock. TestRFC5250DiscontinueOpaqueFromUnreachableOriginator: + two Type-11 LSAs (Opaque IDs 1, 2) from 2.2.2.2 usable while reachable; - the seam flips unreachable, no new LSA, both rows unusable through extOpaqueDecode (the show path). An overlay mutants (go test -overlay, tree untouched, scratch/ospf-judge-mut) making extPrefixUsable return true for AS scope turns the negative red (and TestExtPrefixType11UnreachableUnusable). Revert records on extPrefixUsable observed red. The row's false {single-polarity} annotation ('never cached') was removed; the quote is unchanged. The TED consumer already judged at read time (te_ted.go usableLocked).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5250DiscontinueOpaqueFromUnreachableOriginator`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5250_discontinue_test.go#L64) | unit/verify | revert, verified |
| positive | [`TestRFC5250DiscontinueOpaqueFromUnreachableOriginator`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5250_discontinue_test.go#L51) | unit/verify | revert, verified |
| positive | [`TestOpaqueType11UnreachableOriginatorNotUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5250_opaque_reachability_test.go#L52) | unit/verify | unproven |

### [`RFC5250-5-3`](#rfc5250-5-3)

(1) An OSPF router that is configured to originate AS-scope opaque LSAs will advertise itself as an ASBR and MUST follow the requirements related to setting of the Options field E-bit in OSPF LSA headers as specified in [OSPF]. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-09-30 (independent judge, ospf c13). Section 5 (1): 'An OSPF router that is configured to originate AS-scope opaque LSAs will advertise itself as an ASBR and MUST follow the requirements related to setting of the Options field E-bit in OSPF LSA headers as specified in [OSPF].' RFC 2328 Section 12.1.2: the E-bit 'should also be set in all AS-external-LSAs'. Producers: lsdb/origination.go selfIsASBRLocked now scans d.asOpaque (self, non-purged); lsdb/opaque_as.go OriginateOpaque ORs OptionE into every Type-11 header. Production refreshes the Router-LSA on every origination tick (instance.go ticker -> originateSelfLSAs -> OriginateFromTopology), so the E-bit follows a Type-11 origination or withdrawal within one tick. lsdb TestRFC5250Type11OriginatorAdvertisesASBR: - Router-LSA E clear with nothing, with Type-10 only, and after the only Type-11 is withdrawn; + Type-11 header Options carries E although the caller passed only O, and Router-LSA E set. Two-router TestRFC5250TwoRoutersType11OriginatorReachable: router A's own originated Router-LSA installed on engine B, B's SPF runs, deliverOpaque judges A's Type-11: unusable before A originates it, usable after (the end-to-end consequence of 5 (1) against the 5 (2) receiver). Judge overlays observed red for the right reason: dropping !e.purged from the asOpaque scan reds the withdrawn negative ('E-bit still set after the only Type-11 opaque LSA was withdrawn'); dropping the OptionE set reds the header assertion ('Options 0x40 lack the E-bit'). Recorded revert breaks on selfIsASBRLocked are panic halts. The Router-LSA is carried by Install, not over the wire: the encode/flood path is not exercised.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5250Type11OriginatorAdvertisesASBR`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_asbr_ebit_test.go#L45) | unit/verify | revert, verified |
| positive | [`TestRFC5250Type11OriginatorAdvertisesASBR`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_asbr_ebit_test.go#L59) | unit/verify | revert, verified |
| positive | [`TestRFC5250TwoRoutersType11OriginatorReachable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5250_asbr_originator_test.go#L115) | unit/verify | revert, verified |

### [`RFC5250-3.2-1`](#rfc5250-3.2-1)

The router MUST list the contents of its entire area link-state database in the neighbor Database summary list. The area link-state database consists of the Router LSAs, Network LSAs, Summary LSAs, type-9 Opaque LSAs, and type-10 Opaque LSAs contained in the area structure, along with AS External and type-11 Opaque LSAs contained in the global structure. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Production path databaseSummaryLocked over a real lsdb.LSDB at ExStart: positive lists all eight named types (Router, Network, Summary 3 and 4, type-9 on its interface, type-10, AS External, type-11); negative shows area 0.0.0.4's Router and type-10 and eth9's type-9 absent and the list exactly 8. Judge overlays: dropping link LSAs, Router or Network LSAs turns the positive red; appending another area's Router LSA turns only the negative red. Records observed red on databaseSummaryLocked; the lsdb units stay as supplementary.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5250SummaryOmitsGlobalLSAsInStubAndNSSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_dd_summary_test.go#L58) | unit/verify | revert, verified |
| negative | [`TestRFC5250SummaryOmitsOtherAreaAndLinkLSAs`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc5250_summary_entire_area_test.go#L124) | unit/verify | revert, verified |
| positive | [`TestRFC5250SummaryListsOpaqueAndGlobalLSAs`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_dd_summary_test.go#L24) | unit/verify | revert, verified |
| positive | [`TestRFC5250SummaryListsEntireAreaDatabase`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc5250_summary_entire_area_test.go#L111) | unit/verify | revert, verified |

### [`RFC5250-3.2-2`](#rfc5250-3.2-2)

AS External and type-11 Opaque LSAs MUST be omitted from a virtual neighbor's Database summary list (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5250PhysicalDatabaseExchangeRetainsASScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5250_virtual_exchange_test.go#L85) | unit/verify | unproven |
| positive | [`TestRFC5250VirtualDatabaseExchangeOmitsASScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5250_virtual_exchange_test.go#L68) | unit/verify | unproven |

### [`RFC5250-3.2-3`](#rfc5250-3.2-3)

AS External LSAs and type-11 Opaque LSAs MUST be omitted from the Database summary list if the area has been configured as a stub area or NSSA (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5250SummaryListsOpaqueAndGlobalLSAs`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_dd_summary_test.go#L27) | unit/verify | revert, verified |
| positive | [`TestRFC5250SummaryOmitsGlobalLSAsInStubAndNSSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5250_dd_summary_test.go#L54) | unit/verify | revert, verified |

### [`RFC5250-3.2-4`](#rfc5250-3.2-4)

Type-9 Opaque LSAs MUST be omitted from the Database summary list if the interface associated with the neighbor is not the interface associated with the Opaque LSA (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5250SummaryOmitsOtherInterfaceType9`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc5250_dd_summary_test.go#L61) | unit/verify | revert, verified |
| positive | [`TestRFC5250SummaryListsOwnInterfaceType9`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc5250_dd_summary_test.go#L33) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc5250.txt |
| Source fingerprint | e2fca6f42f48f835 |
| Record | rfc/extraction/rfc5250.json |
| Mapped sentences | 13 |
| Declined as scope | 9 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 9 | walked | not stated |
| `3.2` | not stated | 6 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 0 | walked | not stated |
| `5` | not stated | 4 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `10.1` | not stated | 0 | walked | not stated |
| `10.2` | not stated | 0 | walked | not stated |
| `A` | not stated | 0 | walked | not stated |
| `A.1` | not stated | 0 | walked | not stated |
| `A.2` | not stated | 3 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3.1:2` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The sentence cites Section 13 of [OSPF] (RFC 2328) and binds this document's reader to that RFC's flooding procedure; the opaque-specific modifications follow it and are mapped separately. | Those procedures MUST be followed as defined except where modified in this section. |
| `3.1:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Second half of the same type-11 bullet: RFC5250-3.1-3 states both the flood-out ban and the discard of a type-11 LSA received on a stub/NSSA interface. | A type-11 Opaque LSA that is received on an interface associated with a stub area or NSSA MUST be discarded and not acknowledged (the neighboring router has flooded the LSA in error). |
| `3.1:8` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Editorial note explaining why the preceding sentence chose SHOULD NOT over MUST NOT ('to remain compatible with earlier specifications'); it states no obligation of its own. | The setting of the O-bit is a "SHOULD NOT" rather than a "MUST NOT" to remain compatible with earlier specifications. |
| `3.2:5` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | Section 3.2 reproduces the ExStart/NegotiationDone action of Section 10.3 of [OSPF]; the MaxAge omission sentence is unmodified RFC 2328 text carrying no opaque-specific content. | Any advertisement whose age is equal to MaxAge MUST be omitted from the Database summary list. |
| `3.2:6` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | Same reproduced RFC 2328 Section 10.3 paragraph: the MaxAge advertisement goes on the neighbor's retransmission list, unmodified base text with no opaque content. | It MUST instead be added to the neighbor's link-state retransmission list. |
| `5:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Same numbered procedure (2): RFC5250-5-1 states both the routing-table lookup and 'if unreachable, do nothing with the LSA'. | If no entries exist for the ASBR (i.e., the ASBR is unreachable), the router MUST do nothing with this LSA. |
| `A.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Appendix A.2 restates the link-local flooding scope that Section 3/3.1 defines and RFC5250-3-1 carries. | Opaque LSAs with a link-local scope MUST NOT be flooded beyond the local (sub)network. |
| `A.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Appendix A.2 restates the area-local flooding scope that Section 3/3.1 defines and RFC5250-3-1 carries. | Opaque LSAs with an area-local scope MUST NOT be flooded beyond their area of origin. |
| `A.2:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Appendix A.2 restates the type-11 stub/NSSA flooding ban that Section 3.1 states and RFC5250-3.1-3 carries. | Opaque LSAs with AS-wide scope MUST NOT be flooded into stub areas or NSSAs. |

## Superseded

No document obsoletes RFC 5250, so its obligations are stated where they were written.
