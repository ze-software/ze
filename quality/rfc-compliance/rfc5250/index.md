# RFC 5250 - The OSPF Opaque LSA Option

Experimental. Every requirement this repository extracted from RFC 5250, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 84.6% | 11 of 13 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 15.4% | 2 of 13 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 13 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 13 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 22.2% | 6 of 27 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 13 | of 15 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 13 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 13 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 13 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 13 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Audit verdicts | 10 | of 13 gated MUSTs judged | 6 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 13 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
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
| Requirements | 15 |
| Gated MUST-level | 13 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 27 |
| Tagged units | 27 |
| Recorded audit verdicts | 10 |
| Discrimination records | 6 |
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
| Positive and negative tests | 11 | one part of the gated population |
| Annotated instead of tested | 2 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **13** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (11):** [`RFC5250-3-1`](#rfc5250-3-1), [`RFC5250-3.1-1`](#rfc5250-3.1-1), [`RFC5250-3.1-2`](#rfc5250-3.1-2), [`RFC5250-3.1-3`](#rfc5250-3.1-3), [`RFC5250-3.1-4`](#rfc5250-3.1-4), [`RFC5250-3.1-5`](#rfc5250-3.1-5), [`RFC5250-5-1`](#rfc5250-5-1), [`RFC5250-3.2-1`](#rfc5250-3.2-1), [`RFC5250-3.2-2`](#rfc5250-3.2-2), [`RFC5250-3.2-3`](#rfc5250-3.2-3), [`RFC5250-3.2-4`](#rfc5250-3.2-4)

**Annotated instead of tested (2):** [`RFC5250-3-2`](#rfc5250-3-2), [`RFC5250-5-2`](#rfc5250-5-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5250-3-1` | Opaque LSAs are types 9, 10, and 11 link state advertisements. Opaque LSAs consist of a standard LSA header followed by a 32-bit aligned application-specific information field. Standard link-state database flooding mechanisms are used for distribution of Opaque LSAs. The range of topological distribution (i.e., the flooding scope) of an Opaque LSA is identified by its link-state type. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestLSTypeKnownValues`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/lstype_test.go#L38). **positive:** `unit/verify` [`TestOpaqueScopeRouting`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L73). **negative:** `unit/verify` [`TestLSTypeKnownValues`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/lstype_test.go#L25). **negative:** `unit/verify` [`TestOpaqueScopeRouting`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L88) |
| `RFC5250-3.1-1` | o If the Opaque LSA is type-9 (the flooding scope is link-local) and the interface that the LSA was received on is not the same as the target interface (e.g., the interface associated with a particular target neighbor), the Opaque LSA MUST be discarded and not acknowledged. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestOpaqueType9WrongInterfaceDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L116). **negative:** `unit/verify` [`TestOpaqueType9WrongInterfaceDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L120) |
| `RFC5250-3.1-2` | If the Opaque LSA is type-10 (the flooding scope is area-local) and the area associated with the Opaque LSA (as identified during origination or from a received LSA's associated OSPF packet header) is not the same as the area associated with the target interface, the Opaque LSA MUST be discarded and not acknowledged. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestOpaqueScopeRouting`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L63). **positive:** `unit/verify` [`TestOpaqueType10ConfinedToItsArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L203). **negative:** `unit/verify` [`TestOpaqueType10ConfinedToItsArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L207) |
| `RFC5250-3.1-3` | If the Opaque LSA is type-11 (the LSA is flooded throughout the AS) and the target interface is associated with a stub area or NSSA, the Opaque LSA MUST NOT be flooded out the interface. A type-11 Opaque LSA that is received on an interface associated with a stub area or NSSA MUST be discarded and not acknowledged (the neighboring router has flooded the LSA in error). (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestOpaqueType11StubDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L161). **negative:** `unit/verify` [`TestOpaqueType11StubDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L152) |
| `RFC5250-3.1-4` | Opaque LSAs are only flooded to opaque-capable neighbors. To be more precise, in Section 13.3 of [OSPF], Opaque LSAs MUST be placed on the link-state retransmission lists of opaque-capable neighbors and MUST NOT be placed on the link-state retransmission lists of non-opaque-capable neighbors. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestOpaqueFloodOnlyToOpaqueNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_flood_test.go#L38). **negative:** `unit/verify` [`TestOpaqueFloodOnlyToOpaqueNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_flood_test.go#L42) |
| `RFC5250-3-2` | The link-state ID of the Opaque LSA is divided into an Opaque type field (the first 8 bits) and a type-specific ID (the remaining 24 bits). (§3) | MUST | 3 | **positive:** `unit/verify` [`TestOpaqueLinkStateIDSplit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/lsa_opaque_test.go#L19). **negative:** no negative test. **{single-polarity}:** the Link State ID split is a pure bit operation (high octet is the Opaque Type, low 24 bits the Opaque ID) with no validation or reject path, so there is no negative behavior to drive |
| `RFC5250-3.1-5` | the O-bit SHOULD NOT be set and MUST be ignored when received in packets other than Database Description packets. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestOpaqueBitIgnoredOutsideDD`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/dd_opaque_test.go#L74). **negative:** `unit/verify` [`TestOpaqueBitIgnoredOutsideDD`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/dd_opaque_test.go#L63) |
| `RFC5250-5-1` | (2) When processing a received type-11 Opaque LSA, the router MUST look up the routing table entries (potentially one per attached area) for the ASBR that originated the LSA. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestOpaqueType11UnreachableOriginatorNotUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/opaque_reachability_test.go#L51). **negative:** `unit/verify` [`TestOpaqueType11UnreachableOriginatorNotUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/opaque_reachability_test.go#L44) |
| `RFC5250-5-2` | It also MUST discontinue using all Opaque LSAs injected into the network by the same originator whenever it is detected that the originator is unreachable. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestOpaqueType11UnreachableOriginatorNotUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/opaque_reachability_test.go#L52). **negative:** no negative test. **{single-polarity}:** reachability is recomputed from a live SPF seam on every opaque delivery (internal/plugins/ospf/opaque.go:126,243) and never cached, so a now-unreachable originator yields not-usable on the next evaluation; there is no stale-cache code path to drive a negative |
| `RFC5250-3.2-1` | The router MUST list the contents of its entire area link-state database in the neighbor Database summary list. The area link-state database consists of the Router LSAs, Network LSAs, Summary LSAs, type-9 Opaque LSAs, and type-10 Opaque LSAs contained in the area structure, along with AS External and type-11 Opaque LSAs contained in the global structure. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC5250SummaryListsOpaqueAndGlobalLSAs`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/dd_summary_rfc5250_test.go#L24). **negative:** `unit/verify` [`TestRFC5250SummaryOmitsGlobalLSAsInStubAndNSSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/dd_summary_rfc5250_test.go#L58) |
| `RFC5250-3.2-2` | AS External and type-11 Opaque LSAs MUST be omitted from a virtual neighbor's Database summary list (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC5250VirtualDatabaseExchangeOmitsASScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5250_virtual_exchange_test.go#L68). **negative:** `unit/verify` [`TestRFC5250PhysicalDatabaseExchangeRetainsASScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5250_virtual_exchange_test.go#L85) |
| `RFC5250-3.2-3` | AS External LSAs and type-11 Opaque LSAs MUST be omitted from the Database summary list if the area has been configured as a stub area or NSSA (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC5250SummaryOmitsGlobalLSAsInStubAndNSSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/dd_summary_rfc5250_test.go#L54). **negative:** `unit/verify` [`TestRFC5250SummaryListsOpaqueAndGlobalLSAs`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/dd_summary_rfc5250_test.go#L27) |
| `RFC5250-3.2-4` | Type-9 Opaque LSAs MUST be omitted from the Database summary list if the interface associated with the neighbor is not the interface associated with the Opaque LSA (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC5250SummaryListsOwnInterfaceType9`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/dd_summary_rfc5250_test.go#L33). **negative:** `unit/verify` [`TestRFC5250SummaryOmitsOtherInterfaceType9`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/dd_summary_rfc5250_test.go#L61) |
| `RFC5250-3.1-6` | A neighbor is opaque-capable if and only if it sets the O-bit in the Options field of its Database Description packets; the O-bit SHOULD NOT be set and MUST be ignored when received in packets other than Database Description packets. (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5250-8-1` | The frequency at which new LSA instances may be originated is set equal to once every MinLSInterval seconds, whose value is 5 seconds (see Section 12.4 of [OSPF]). The frequency at which new LSA instances are accepted during flooding is once every MinLSArrival seconds, whose value is set to 1 (see Section 13, Appendix B, and G.5 of [OSPF]). (§8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 5250 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5250-3-1`](#rfc5250-3-1)

Opaque LSAs are types 9, 10, and 11 link state advertisements. Opaque LSAs consist of a standard LSA header followed by a 32-bit aligned application-specific information field. Standard link-state database flooding mechanisms are used for distribution of Opaque LSAs. The range of topological distribution (i.e., the flooding scope) of an Opaque LSA is identified by its link-state type. (§3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Types 9/10/11 as Opaque and scope-by-type are enforced: lstype_test.go fatals if a standard type IsOpaque or an opaque type is not; TestOpaqueScopeRouting fatals if Type 11 is not AS-wide or a Type 9 is accepted into an area store. The quoted span also states that Opaque LSAs consist of a standard LSA header followed by a 32-bit aligned body and that standard flooding mechanisms distribute them; no tagged unit asserts either.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpaqueScopeRouting`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L88) | unit/verify | unproven |
| negative | [`TestLSTypeKnownValues`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/lstype_test.go#L25) | unit/verify | unproven |
| positive | [`TestOpaqueScopeRouting`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L73) | unit/verify | unproven |
| positive | [`TestLSTypeKnownValues`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/lstype_test.go#L38) | unit/verify | unproven |

### [`RFC5250-3.1-1`](#rfc5250-3.1-1)

o If the Opaque LSA is type-9 (the flooding scope is link-local) and the interface that the LSA was received on is not the same as the target interface (e.g., the interface associated with a particular target neighbor), the Opaque LSA MUST be discarded and not acknowledged. (§3.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Discard is enforced: TestOpaqueType9WrongInterfaceDiscarded fatals if a Type 9 received on eth0 is found in the eth1 link store, flooded as an LSUpdate out eth1, or queued for retransmission to the eth1 neighbor. The "not acknowledged" clause has no assertion: the send loop checks only s.pkt.LSUpdate, so an LSAck out eth1 passes.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpaqueType9WrongInterfaceDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L120) | unit/verify | unproven |
| positive | [`TestOpaqueType9WrongInterfaceDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L116) | unit/verify | unproven |

### [`RFC5250-3.1-2`](#rfc5250-3.1-2)

If the Opaque LSA is type-10 (the flooding scope is area-local) and the area associated with the Opaque LSA (as identified during origination or from a received LSA's associated OSPF packet header) is not the same as the area associated with the target interface, the Opaque LSA MUST be discarded and not acknowledged. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Type 10 flooded to or read from an area other than its own. TestOpaqueType10ConfinedToItsArea fatals if the LSA is readable (LookupLSA or Lookup) from area 0.0.0.2, if floodExcept emits anything but exactly one send, on eth0, which also refuses any acknowledgement out the other area interface, or if the eth1 neighbor is queued to retransmit it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpaqueType10ConfinedToItsArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L207) | unit/verify | unproven |
| positive | [`TestOpaqueScopeRouting`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L63) | unit/verify | unproven |
| positive | [`TestOpaqueType10ConfinedToItsArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L203) | unit/verify | unproven |

### [`RFC5250-3.1-3`](#rfc5250-3.1-3)

If the Opaque LSA is type-11 (the LSA is flooded throughout the AS) and the target interface is associated with a stub area or NSSA, the Opaque LSA MUST NOT be flooded out the interface. A type-11 Opaque LSA that is received on an interface associated with a stub area or NSSA MUST be discarded and not acknowledged (the neighboring router has flooded the LSA in error). (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Clause 1 forbidden: a Type 11 flooded out a stub interface; TestOpaqueType11StubDiscarded fatals if floodExcept sends on eth0 (stub). Clause 2 forbidden: a Type 11 received on a stub interface installed or acknowledged; the same test fatals if it is found in the store or if any packet at all is sent (len(tx.sends) != 0, which covers an LSAck).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpaqueType11StubDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L152) | unit/verify | unproven |
| positive | [`TestOpaqueType11StubDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L161) | unit/verify | unproven |

### [`RFC5250-3.1-4`](#rfc5250-3.1-4)

Opaque LSAs are only flooded to opaque-capable neighbors. To be more precise, in Section 13.3 of [OSPF], Opaque LSAs MUST be placed on the link-state retransmission lists of opaque-capable neighbors and MUST NOT be placed on the link-state retransmission lists of non-opaque-capable neighbors. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an opaque LSA placed on the retransmission list of a non-opaque-capable neighbor, or missing from an opaque-capable one. The opaque_flood_test.go unit fatals if db.retransmit lacks the Type 11 for 2.2.2.2 (OpaqueCapable) or holds it for 3.3.3.3 (not capable), and shows a Router LSA still reaches both.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpaqueFloodOnlyToOpaqueNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_flood_test.go#L42) | unit/verify | unproven |
| positive | [`TestOpaqueFloodOnlyToOpaqueNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_flood_test.go#L38) | unit/verify | unproven |

### [`RFC5250-3-2`](#rfc5250-3-2)

The link-state ID of the Opaque LSA is divided into an Opaque type field (the first 8 bits) and a type-specific ID (the remaining 24 bits). (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Link State ID split other than 8-bit type and 24-bit ID. TestOpaqueLinkStateIDSplit fatals if OpaqueTypeOf(AA BB CC DD) != 0xAA or OpaqueIDOf != 0x00BBCCDD, on a failed round trip, on a high id byte leaking into the type, and at the 0xFF/0xFFFFFF boundary. Row carries {single-polarity: positive}.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestOpaqueLinkStateIDSplit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/lsa_opaque_test.go#L19) | unit/verify | unproven |

### [`RFC5250-3.1-5`](#rfc5250-3.1-5)

the O-bit SHOULD NOT be set and MUST be ignored when received in packets other than Database Description packets. (§3.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The negative in dd_opaque_test.go asserts a neighbor is not opaque-capable after Hellos, but HelloInput carries no Options field and the Hellos carry no O-bit, so no non-DD packet with the O-bit set is ever received: a NAS that read the O-bit from a Hello is not driven. The quoted "SHOULD NOT be set" clause (no O-bit sent in non-DD packets) has no assertion either.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpaqueBitIgnoredOutsideDD`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/dd_opaque_test.go#L63) | unit/verify | unproven |
| positive | [`TestOpaqueBitIgnoredOutsideDD`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/dd_opaque_test.go#L74) | unit/verify | unproven |

### [`RFC5250-5-1`](#rfc5250-5-1)

(2) When processing a received type-11 Opaque LSA, the router MUST look up the routing table entries (potentially one per attached area) for the ASBR that originated the LSA. (§5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The unit replaces eng.opaqueReachableFn with a stub keyed on router ID and checks deliverOpaque reports Reachable accordingly. The sentence obliges a lookup of the routing table entries for the originating ASBR; the seam is faked, so a predicate that never consulted the routing table (or looked up the router rather than the ASBR entry) stays green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpaqueType11UnreachableOriginatorNotUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/opaque_reachability_test.go#L44) | unit/verify | unproven |
| positive | [`TestOpaqueType11UnreachableOriginatorNotUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/opaque_reachability_test.go#L51) | unit/verify | unproven |

### [`RFC5250-5-2`](#rfc5250-5-2)

It also MUST discontinue using all Opaque LSAs injected into the network by the same originator whenever it is detected that the originator is unreachable. (§5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: continuing to use Opaque LSAs already delivered from an originator once it becomes unreachable. The tagged positive delivers one LSA from an unreachable and one from a reachable originator; no unit delivers from an originator, flips it unreachable, and asserts its earlier LSAs stop being used, so a consumer holding stale Reachable=true data stays green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestOpaqueType11UnreachableOriginatorNotUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/opaque_reachability_test.go#L52) | unit/verify | unproven |

### [`RFC5250-3.2-1`](#rfc5250-3.2-1)

The router MUST list the contents of its entire area link-state database in the neighbor Database summary list. The area link-state database consists of the Router LSAs, Network LSAs, Summary LSAs, type-9 Opaque LSAs, and type-10 Opaque LSAs contained in the area structure, along with AS External and type-11 Opaque LSAs contained in the global structure. (§3.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The normal-area summary is asserted to list type-10, type-5 and type-11 LSAs (TestRFC5250SummaryListsOpaqueAndGlobalLSAs). The quoted list also names type-9 Opaque LSAs and Router, Network and Summary LSAs, and no tagged unit asserts any of them in the Database summary list; the tagged negative is the stub/NSSA omission of RFC5250-3.2-3.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5250SummaryOmitsGlobalLSAsInStubAndNSSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/dd_summary_rfc5250_test.go#L58) | unit/verify | revert, verified |
| positive | [`TestRFC5250SummaryListsOpaqueAndGlobalLSAs`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/dd_summary_rfc5250_test.go#L24) | unit/verify | revert, verified |

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
| negative | [`TestRFC5250SummaryListsOpaqueAndGlobalLSAs`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/dd_summary_rfc5250_test.go#L27) | unit/verify | revert, verified |
| positive | [`TestRFC5250SummaryOmitsGlobalLSAsInStubAndNSSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/dd_summary_rfc5250_test.go#L54) | unit/verify | revert, verified |

### [`RFC5250-3.2-4`](#rfc5250-3.2-4)

Type-9 Opaque LSAs MUST be omitted from the Database summary list if the interface associated with the neighbor is not the interface associated with the Opaque LSA (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5250SummaryOmitsOtherInterfaceType9`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/dd_summary_rfc5250_test.go#L61) | unit/verify | revert, verified |
| positive | [`TestRFC5250SummaryListsOwnInterfaceType9`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/dd_summary_rfc5250_test.go#L33) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc5250.txt |
| Source fingerprint | e2fca6f42f48f835 |
| Record | rfc/extraction/rfc5250.json |
| Mapped sentences | 12 |
| Declined as scope | 10 |
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
| `5:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The obligation named is 'the requirements related to setting of the Options field E-bit in OSPF LSA headers as specified in [OSPF]', which is RFC 2328's ASBR E-bit rule. | (1) An OSPF router that is configured to originate AS-scope opaque LSAs will advertise itself as an ASBR and MUST follow the requirements related to setting of the Options field E-bit in OSPF LSA headers as specified in [OSPF]. |
| `5:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Same numbered procedure (2): RFC5250-5-1 states both the routing-table lookup and 'if unreachable, do nothing with the LSA'. | If no entries exist for the ASBR (i.e., the ASBR is unreachable), the router MUST do nothing with this LSA. |
| `A.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Appendix A.2 restates the link-local flooding scope that Section 3/3.1 defines and RFC5250-3-1 carries. | Opaque LSAs with a link-local scope MUST NOT be flooded beyond the local (sub)network. |
| `A.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Appendix A.2 restates the area-local flooding scope that Section 3/3.1 defines and RFC5250-3-1 carries. | Opaque LSAs with an area-local scope MUST NOT be flooded beyond their area of origin. |
| `A.2:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Appendix A.2 restates the type-11 stub/NSSA flooding ban that Section 3.1 states and RFC5250-3.1-3 carries. | Opaque LSAs with AS-wide scope MUST NOT be flooded into stub areas or NSSAs. |

## Superseded

No document obsoletes RFC 5250, so its obligations are stated where they were written.
