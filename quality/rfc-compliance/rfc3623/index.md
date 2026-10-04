# RFC 3623 - Graceful OSPF Restart

Experimental. Every requirement this repository extracted from RFC 3623, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 30.8% | 4 of 13 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 38.5% | 5 of 13 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 13 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 13 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 66.7% | 18 of 27 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 13 | of 26 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 13 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 15.4% | 2 of 13 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 13 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 13 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 15.4% | 2 of 13 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 13 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 26 |
| Gated MUST-level | 13 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 2 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 27 |
| Tagged units | 27 |
| Recorded audit verdicts | 9 |
| Discrimination records | 18 |
| Summary | `rfc/short/rfc3623.md` |
| Requirement shard | `rfc/requirements/rfc3623.md` |
| RFC text | `rfc/full/rfc3623.txt` |

## Enrolment

Enrolled: Graceful OSPF Restart (RFC 3623): restarter + helper roles; 4 MET (Grace-LSA mandatory TLVs, shared-media type-3, unplanned toggle) + 5 single-polarity positive + 2 gap (virtual-link V-bit, changed-retx-list refusal) + 2 not-applicable

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

- Restarter (planned + opt-in unplanned) and helper behavior
- Grace-LSA Opaque type-3 body codec shared with RFC 5187.


**What the ledger says remains**

Two MUST gaps gated in [`rfc/short/rfc3623.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc3623.md): the helper does not preserve the transit-area V-bit when helping over a virtual link ([`RFC3623-3-1`](#rfc3623-3-1)), and helper entry does not refuse on a changed retransmission-list LSA ([`RFC3623-3.1-1`](#rfc3623-3.1-1), onGraceReceived is hardcoded permissive, mitigated by the Section 3.2 strict-LSA-checking exit). Experimental pending deployment hardening.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated (including scoped evidence) | 9 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **13** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC3623-A-2`](#rfc3623-a-2), [`RFC3623-A-3`](#rfc3623-a-3), [`RFC3623-A-4`](#rfc3623-a-4), [`RFC3623-5-1`](#rfc3623-5-1)

**Annotated (including scoped evidence) (9):** [`RFC3623-A-1`](#rfc3623-a-1), [`RFC3623-A-5`](#rfc3623-a-5), [`RFC3623-2.1-1`](#rfc3623-2.1-1), [`RFC3623-5-2`](#rfc3623-5-2), [`RFC3623-5-3`](#rfc3623-5-3), [`RFC3623-5-4`](#rfc3623-5-4), [`RFC3623-3-1`](#rfc3623-3-1), [`RFC3623-3.1-1`](#rfc3623-3.1-1), [`RFC3623-3.2-1`](#rfc3623-3.2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3623-A-1` | Additional Grace-LSA TLVs must be described in an Internet Draft and will be subject to the expert review of the OSPF Working Group. (§A) | MUST | A | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze originates only the three RFC-defined TLVs (types 1/2/3) and adds none, and its decoder ignores unrecognized types, so the IETF-process obligation for additional TLVs binds no ze behavior (internal/plugins/ospf/packet/grace_lsa.go:44, :64) |
| `RFC3623-A-2` | Grace Period (Type=1, length=4). The number of seconds that the router's neighbors should continue to advertise the router as fully adjacent, regardless of the state of database synchronization between the router and its neighbors. Since this time period began when grace-LSA's LS age was equal to 0, the grace period terminates when either: a) the LS age of the grace-LSA exceeds the value of a Grace Period or b) the grace-LSA is flushed. See Section 3.2 for other conditions that terminate graceful restart. This TLV must always appear in a grace-LSA. (§A) | MUST | A | **positive:** `unit/verify` [`TestGraceLSARoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc3623_grace_lsa_test.go#L13). **positive:** `unit/verify` [`TestRFC3623OriginatedGraceLSATLVOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_tlv_octets_test.go#L41). **negative:** `unit/verify` [`TestGraceLSADecodeMissingMandatory`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc3623_grace_lsa_test.go#L62). **negative:** `unit/verify` [`TestRFC3623HelperIgnoresGraceLSAMissingMandatoryTLV`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_tlv_octets_test.go#L82) |
| `RFC3623-A-3` | Graceful restart reason (Type=2, length=1). Encodes the reason for the router restart as one of the following: 0 (unknown), 1 (software restart), 2 (software reload/upgrade) or 3 (switch to redundant control processor). This TLV must always appear in a grace-LSA. (§A) | MUST | A | **positive:** `unit/verify` [`TestGraceLSARoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc3623_grace_lsa_test.go#L17). **positive:** `unit/verify` [`TestRFC3623OriginatedGraceLSATLVOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_tlv_octets_test.go#L44). **negative:** `unit/verify` [`TestGraceLSADecodeMissingMandatory`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc3623_grace_lsa_test.go#L59). **negative:** `unit/verify` [`TestRFC3623HelperIgnoresGraceLSAMissingMandatoryTLV`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_tlv_octets_test.go#L87) |
| `RFC3623-A-4` | IP interface address (Type=3, length=4). The router's IP interface address on the subnet associated with the grace-LSA. Required on broadcast, NBMA and Point-to-MultiPoint segments, where the helper uses the IP interface address to identify the restarting router (see Section 3.1). (§A) | MUST | A | **positive:** `unit/verify` [`TestGraceLSAv4BodyBuild`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_lsa_test.go#L18). **positive:** `unit/verify` [`TestRFC3623OriginatedGraceLSATLVOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_tlv_octets_test.go#L48). **positive:** `unit/verify` [`TestRFC3623SharedMediaGraceLSACarriesInterfaceAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_origination_test.go#L58). **negative:** `unit/verify` [`TestGraceLSAv4BodyBuild`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_lsa_test.go#L22) |
| `RFC3623-A-5` | DoNotAge is never set in a grace-LSA, even if the grace-LSA is flooded over a demand circuit [7]. (§A) | MUST | A | **positive:** `unit/verify` [`TestGraceLSANeverSetsDoNotAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L62). **negative:** no negative test. **{single-polarity}:** Grace-LSAs originate through OriginateOpaque, whose input struct has no DoNotAge field and starts LS age at 0 with normal aging, so the bit is never set and no negative behavior exists to test (internal/plugins/ospf/lsdb/opaque_as.go:73, gr_restarter.go:314, lsdb/entry.go:87) |
| `RFC3623-2.1-1` | In preparation for the graceful restart, Router X must perform the following actions before its software is restarted/reloaded: (Note that common OSPF shutdown procedures are *not* performed, since we want the other OSPF routers to act as if Router X remains in continuous service. For example, Router X does not flush its locally originated LSAs, since we want them to remain in other routers' link-state databases throughout the restart period.) 1) Router X must ensure that its forwarding table(s) is/are up- to-date and will remain in place across the restart. (§2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestPrepareRestartRetainsFIB`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L91). **positive:** `unit/verify` [`TestRFC3623PrepareInstallsPendingSPFAndKeepsIt`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_prepare_fib_test.go#L106). **positive:** `unit/verify` [`TestRFC3623PreparedRestartKeepsRoutesSPFNoLongerComputes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_install_wiring_test.go#L85). **negative:** no negative test. **{single-polarity}:** prepareRestart raises gracefulStop so suppressInstall makes the ensuing engine stop skip RemoveAll and retain the pre-restart FIB; the meaningful assertion is retention, with no negative behavior the requirement forbids (internal/plugins/ospf/gr_restarter.go:49, gr.go:235) |
| `RFC3623-5-1` | In any event, implementors providing the option to recover gracefully from unplanned outages must allow a network operator to turn the option off. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC3623OperatorTurnsUnplannedRecoveryOff`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_origination_test.go#L119). **positive:** `unit/verify` [`TestUnplannedGraceBeforeHello`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_unplanned_test.go#L40). **negative:** `unit/verify` [`TestRFC3623OperatorTurnsUnplannedRecoveryOff`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_origination_test.go#L122). **negative:** `unit/verify` [`TestUnplannedDisabledByDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_unplanned_test.go#L16) |
| `RFC3623-5-2` | The grace-LSAs must be originated and be sent *before* the restarted router sends any OSPF Hello Packets. On broadcast networks, this LSA must be flooded to the AllSPFRouters multicast address (224.0.0.5) since the restarting router is not aware of its previous DR state. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC3623RestartGraceLSAFloodedToAllSPFRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3623_restart_grace_flood_test.go#L45). **positive:** `unit/verify` [`TestRFC3623UnplannedColdStartOriginatesGraceLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_unplanned_cold_start_test.go#L131). **positive:** `unit/verify` [`TestUnplannedGraceLSAFloodsToAllSPFRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L151). **negative:** no negative test. **{single-polarity}:** maybeUnplannedRestart enters in-restart then originates one Grace-LSA per interface before interface Hellos begin, and link-local LSAs flood to AllSPFRouters via the standard link-scope path (internal/plugins/ospf/gr_restarter.go:105, lsdb_flooding_test.go:47) |
| `RFC3623-5-3` | The grace-LSAs are encapsulated in Link State Update Packets and sent out to all interfaces, even though the restarted router has no adjacencies and no knowledge of previous adjacencies. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC3623RestartGraceLSAFloodedToAllSPFRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3623_restart_grace_flood_test.go#L43). **positive:** `unit/verify` [`TestUnplannedGraceLSAPerActiveInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L193). **negative:** no negative test. **{single-polarity}:** grOriginateGraceLSAs walks every active (non-passive) interface and floods each Grace-LSA via OriginateOpaque's standard LSU flooding (internal/plugins/ospf/gr_restarter.go:296, :315) |
| `RFC3623-5-4` | o The restart reason in the grace-LSAs must be set to 0 (unknown) or 3 (switch to redundant control processor). (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC3623UnplannedGraceLSACarriesUnplannedReason`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_origination_test.go#L80). **positive:** `unit/verify` [`TestUnplannedGraceBeforeHello`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_unplanned_test.go#L58). **negative:** no negative test. **{single-polarity}:** grUnplannedReason returns the constant 3 (redundant control processor), an in-range unplanned reason, and there is no receive-side reason rejection to test negatively (internal/plugins/ospf/gr_restarter.go:88, gr.go:36) |
| `RFC3623-3-1` | When helping over a virtual link, the helper must also continue to set bit V in its router-LSA for the virtual link's transit area (Section 12.4.1 of [1]). (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the helper role keeps X advertised in Router/Network-LSAs but neither gr_helper.go nor gr.go has any virtual-link/transit-area handling, so the transit-area V-bit is not preserved while helping over a virtual link (internal/plugins/ospf/gr_helper.go:279) |
| `RFC3623-3.1-1` | Router Y examines the link-state retransmission list for X over the associated network segment. - If there are any LSAs with LS types 1-5,7 on the list, then they all must be periodic refreshes. - If there are instead LSAs on the list whose contents have changed (see Section 3.3 of [7]), Y must refuse to enter helper mode. (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the helper entry decision has an lsdb-changed branch, but the production caller hardcodes lsdbUnchanged=true, so no production path refuses entry on a changed retransmission-list LSA; the permissive entry is only mitigated by the Section 3.2 strict-checking exit (internal/plugins/ospf/gr_helper.go:40, :128) |
| `RFC3623-3.2-1` | If Router Y aggregated adjacencies with Router X when entering helper mode (as described in section 3.1), it must also exit helper mode for all adjacencies with Router X when any one of the exit events occurs for an adjacency with Router X. (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze tracks one helper session per (interface, router) and never aggregates adjacencies across segments, so the aggregated exit-all obligation binds a mode ze does not play (internal/plugins/ospf/gr.go:71) |
| `RFC3623-2.1-2` | In order to avoid the restarting router's LSAs from aging out, the grace period should not exceed LSRefreshTime (1800 second) [1]. (§2.1) | SHOULD NOT | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-2.1-3` | If Router X wants to ensure that its neighbors receive the grace- LSAs, it should retransmit the grace-LSAs until they are acknowledged (i.e., perform standard OSPF reliable flooding of the grace-LSAs). (§2.1) | SHOULD | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-2.1-4` | After the grace-LSAs have been sent, the router should store the fact that it is performing graceful restart along with the length of the requested grace period in non-volatile storage. (§2.1) | SHOULD | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-2.3-1` | The router should reoriginate its router-LSAs for all attached areas in order to make sure they have the correct contents. (§2.3) | SHOULD | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-2.3-2` | The router should reoriginate network-LSAs on all segments where it is the Designated Router. (§2.3) | SHOULD | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-2.3-3` | Any remnant entries in the system forwarding table that were installed before the restart, but that are no longer valid, should be removed. (§2.3) | SHOULD | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-2.3-4` | Any received self-originated LSAs that are no longer valid should be flushed. (§2.3) | SHOULD | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-2.3-5` | Any grace-LSAs that the router originated should be flushed. (§2.3) | SHOULD | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-2.1-5` | The router may need to preserve the cryptographic sequence numbers being used on each interface in non-volatile storage. An alternative is to use the router's clock for cryptographic sequence number generation and ensure that the clock is preserved across restarts (either on the same or redundant route processors). (§2.1) | MAY | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-5-5` | To improve the probability that grace-LSAs will be delivered, an implementation may send them multiple times (see for example the Robustness Variable in [8]). (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-3.1-2` | Router Y may optionally disallow graceful restart with Router X on other network segments. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-3.1-3` | Alternately, Router Y may choose to enter helper mode when a grace- LSA is received and the above checks pass for all adjacencies with Router X. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-3.2-2` | An implementation MAY provide a configuration option to disable link-state database options from terminating graceful restart. Such an option will, however, increase the risk of transient routing loops and black holes. (§3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC3623-A-1`](#rfc3623-a-1) Additional Grace-LSA TLVs must be described in an Internet Draft and will be subject to the expert review of the OSPF Working Group. (§A) | no test | no test carries this requirement id; annotated {not-applicable}: ze originates only the three RFC-defined TLVs (types 1/2/3) and adds none, and its decoder ignores unrecognized types, so the IETF-process obligation for additional TLVs binds no ze behavior (internal/plugins/ospf/packet/grace_lsa.go:44, :64) |
| [`RFC3623-3-1`](#rfc3623-3-1) When helping over a virtual link, the helper must also continue to set bit V in its router-LSA for the virtual link's transit area (Section 12.4.1 of [1]). (§3) | {gap}, no test | the helper role keeps X advertised in Router/Network-LSAs but neither gr_helper.go nor gr.go has any virtual-link/transit-area handling, so the transit-area V-bit is not preserved while helping over a virtual link (internal/plugins/ospf/gr_helper.go:279) |
| [`RFC3623-3.1-1`](#rfc3623-3.1-1) Router Y examines the link-state retransmission list for X over the associated network segment. - If there are any LSAs with LS types 1-5,7 on the list, then they all must be periodic refreshes. - If there are instead LSAs on the list whose contents have changed (see Section 3.3 of [7]), Y must refuse to enter helper mode. (§3.1) | {gap}, no test | the helper entry decision has an lsdb-changed branch, but the production caller hardcodes lsdbUnchanged=true, so no production path refuses entry on a changed retransmission-list LSA; the permissive entry is only mitigated by the Section 3.2 strict-checking exit (internal/plugins/ospf/gr_helper.go:40, :128) |
| [`RFC3623-3.2-1`](#rfc3623-3.2-1) If Router Y aggregated adjacencies with Router X when entering helper mode (as described in section 3.1), it must also exit helper mode for all adjacencies with Router X when any one of the exit events occurs for an adjacency with Router X. (§3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze tracks one helper session per (interface, router) and never aggregates adjacencies across segments, so the aggregated exit-all obligation binds a mode ze does not play (internal/plugins/ospf/gr.go:71) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC3623-A-1`](#rfc3623-a-1)

Additional Grace-LSA TLVs must be described in an Internet Draft and will be subject to the expert review of the OSPF Working Group. (§A)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3623-A-1, so no unit is bound to it.

### [`RFC3623-A-2`](#rfc3623-a-2)

Grace Period (Type=1, length=4). The number of seconds that the router's neighbors should continue to advertise the router as fully adjacent, regardless of the state of database synchronization between the router and its neighbors. Since this time period began when grace-LSA's LS age was equal to 0, the grace period terminates when either: a) the LS age of the grace-LSA exceeds the value of a Grace Period or b) the grace-LSA is flushed. See Section 3.2 for other conditions that terminate graceful restart. This TLV must always appear in a grace-LSA. (§A)

Audit verdict: enforced (the tests do what the requirement demands), fresh. c23 re-judge (independent judge). Positive: TestRFC3623OriginatedGraceLSATLVOctets drives grOriginateGraceLSAs (the OSPFv2 origination, grV4Body) over running p2p eth0 and broadcast eth1 and compares the installed link-scope body with literal octets 00 01 00 04 00 00 00 78 (Type 1, Length 4, 120 s), read without the codec; an overlay GraceTLVPeriod=4 reds it while TestGraceLSARoundTrip and TestGraceLSAv4* stay green (scratch/c23/o1). Negative under owner ruling 2 (RFC 3623 prescribes no handling; the helper cannot run a window it was not given, so ignore): TestRFC3623HelperIgnoresGraceLSAMissingMandatoryTLV feeds a reason-only body through graceOnReceive while helping X and asserts the session survives with an unchanged grace end; the control with both TLVs moves it to now+600, so the input is isolated to the missing TLV. Decoder overlay without the hasPeriod check reds only missing-grace-period (o3). Both units hold observed-red revert records (EncodeGraceLSA, DecodeGraceLSA). The HEAD codec units stay supplementary.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGraceLSADecodeMissingMandatory`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc3623_grace_lsa_test.go#L62) | unit/verify | revert, verified |
| negative | [`TestRFC3623HelperIgnoresGraceLSAMissingMandatoryTLV`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_tlv_octets_test.go#L82) | unit/verify | revert, verified |
| positive | [`TestGraceLSARoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc3623_grace_lsa_test.go#L13) | unit/verify | revert, verified |
| positive | [`TestRFC3623OriginatedGraceLSATLVOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_tlv_octets_test.go#L41) | unit/verify | revert, verified |

### [`RFC3623-A-3`](#rfc3623-a-3)

Graceful restart reason (Type=2, length=1). Encodes the reason for the router restart as one of the following: 0 (unknown), 1 (software restart), 2 (software reload/upgrade) or 3 (switch to redundant control processor). This TLV must always appear in a grace-LSA. (§A)

Audit verdict: enforced (the tests do what the requirement demands), fresh. c23 re-judge (independent judge). Positive: TestRFC3623OriginatedGraceLSATLVOctets asserts the originated OSPFv2 Grace-LSA carries 00 02 00 01 02 00 00 00 (Type 2, Length 1, reason 2, padded) at octets 8..15 on both p2p and broadcast, and the p2p body is exactly these 16 octets; overlay GraceTLVReason=5 reds it, old units green (o2). Negative under owner ruling 2: a period-only body (600) through graceOnReceive is ignored, same session, same grace end, control with both TLVs applied; decoder overlay without the hasReason check reds only missing-restart-reason (o4). Observed-red revert records on EncodeGraceLSA and DecodeGraceLSA. HEAD codec units supplementary.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGraceLSADecodeMissingMandatory`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc3623_grace_lsa_test.go#L59) | unit/verify | revert, verified |
| negative | [`TestRFC3623HelperIgnoresGraceLSAMissingMandatoryTLV`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_tlv_octets_test.go#L87) | unit/verify | revert, verified |
| positive | [`TestGraceLSARoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc3623_grace_lsa_test.go#L17) | unit/verify | revert, verified |
| positive | [`TestRFC3623OriginatedGraceLSATLVOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_tlv_octets_test.go#L44) | unit/verify | revert, verified |

### [`RFC3623-A-4`](#rfc3623-a-4)

IP interface address (Type=3, length=4). The router's IP interface address on the subnet associated with the grace-LSA. Required on broadcast, NBMA and Point-to-MultiPoint segments, where the helper uses the IP interface address to identify the restarting router (see Section 3.1). (§A)

Audit verdict: enforced (the tests do what the requirement demands), fresh. c23 re-judge under the main-thread ruling (R46 / owner ruling 5 precedent): judged on its positive. TestRFC3623OriginatedGraceLSATLVOctets: the Grace-LSA grOriginateGraceLSAs installs on a running broadcast interface is 24 octets and octets 16..19 are the literal type-3 TLV header 00 03 00 04; TestRFC3623SharedMediaGraceLSACarriesInterfaceAddress covers broadcast, NBMA and P2MP and decodes the address value. Both hold observed-red revert records on grSharedMedia. No legitimate refusal exists: Ze's helper keys a restarting neighbour by advertising Router ID (onGraceReceived, hasFullNeighbor) and never reads the TLV, which RFC 3623 sec A does not forbid. The HEAD negative tag on TestGraceLSAv4BodyBuild (#2) is true as written (grV4Body with sharedMedia=false emits no type-3 TLV, asserted by !g2.HasInterfaceAddr) and stays supplementary: it does not violate this requirement, which forbids nothing on point-to-point. Observation for the main thread, outside this row: RFC 3623 sec 3.1 item 1 says on broadcast/NBMA/P2MP the neighbour 'is identified by the IP interface address in the body of the grace-LSA'; Ze identifies by Router ID (no gated row carries that sentence).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGraceLSAv4BodyBuild`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_lsa_test.go#L22) | unit/verify | unproven |
| positive | [`TestGraceLSAv4BodyBuild`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_lsa_test.go#L18) | unit/verify | unproven |
| positive | [`TestRFC3623SharedMediaGraceLSACarriesInterfaceAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_origination_test.go#L58) | unit/verify | revert, verified |
| positive | [`TestRFC3623OriginatedGraceLSATLVOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_tlv_octets_test.go#L48) | unit/verify | revert, verified |

### [`RFC3623-A-5`](#rfc3623-a-5)

DoNotAge is never set in a grace-LSA, even if the grace-LSA is flooded over a demand circuit [7]. (§A)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden behaviour (a): a grace-LSA whose LS age has the DoNotAge bit set. (b) TestGraceLSANeverSetsDoNotAge drives OriginateOpaque with the restarter's body and asserts grace.Header.Age.DoNotAge() is false and RawBytes[0]&0x80 == 0, so an origination that set the bit goes red. The demand-circuit clause has no path in ze: no production code sets DoNotAgeBit (only types/lsage.go declares and reads it) and flooding copies the installed RawBytes, so the asserted bytes are the flooded bytes. Single-polarity marker holds.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestGraceLSANeverSetsDoNotAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L62) | unit/verify | unproven |

### [`RFC3623-2.1-1`](#rfc3623-2.1-1)

In preparation for the graceful restart, Router X must perform the following actions before its software is restarted/reloaded: (Note that common OSPF shutdown procedures are *not* performed, since we want the other OSPF routers to act as if Router X remains in continuous service. For example, Router X does not flush its locally originated LSAs, since we want them to remain in other routers' link-state databases throughout the restart period.) 1) Router X must ensure that its forwarding table(s) is/are up- to-date and will remain in place across the restart. (§2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (independent judge, c12). Section 2.1: 'Router X must ensure that its forwarding table(s) is/are up-to-date and will remain in place across the restart.' Up-to-date clause: TestRFC3623PrepareInstallsPendingSPFAndKeepsIt (c10, grRefreshFIB). Remain-in-place clause, the c11 gap (initSPF SetInstallSuppress wiring unproven): TestRFC3623PreparedRestartKeepsRoutesSPFNoLongerComputes drives the e.spf newEngine built through initSPF (not rebuilt), SPF installs 203.0.113.128/26, prepareRestart, the peer re-originates without the stub, Run: the route stays in the Loc-RIB. Author overlay removing only e.spf.SetInstallSuppress(e.gr.suppressInstall) from spf_wiring.go turns it red ('withdrew a route from the retained forwarding table'; judge read the overlay diff and the log) while the c10 unit stays green. Recorded red is the initSPF revert halt. {single-polarity: positive} kept: the requirement forbids nothing beyond failing to retain.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC3623PreparedRestartKeepsRoutesSPFNoLongerComputes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_install_wiring_test.go#L85) | unit/verify | revert, verified |
| positive | [`TestPrepareRestartRetainsFIB`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L91) | unit/verify | unproven |
| positive | [`TestRFC3623PrepareInstallsPendingSPFAndKeepsIt`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_prepare_fib_test.go#L106) | unit/verify | revert, verified |

### [`RFC3623-5-1`](#rfc3623-5-1)

In any event, implementors providing the option to recover gracefully from unplanned outages must allow a network operator to turn the option off. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29 (independent judge). The operator surface is now driven: TestRFC3623OperatorTurnsUnplannedRecoveryOff parses 'graceful-restart restarter support' from configuration text and runs maybeUnplannedRestart: planned-and-unplanned enters in-restart, planned and disabled do not. A parser mapping every value to planned-and-unplanned reddens the negative (revert records on parseGRSupport).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestUnplannedDisabledByDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_unplanned_test.go#L16) | unit/verify | unproven |
| negative | [`TestRFC3623OperatorTurnsUnplannedRecoveryOff`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_origination_test.go#L122) | unit/verify | revert, verified |
| positive | [`TestUnplannedGraceBeforeHello`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_unplanned_test.go#L40) | unit/verify | unproven |
| positive | [`TestRFC3623OperatorTurnsUnplannedRecoveryOff`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_origination_test.go#L119) | unit/verify | revert, verified |

### [`RFC3623-5-2`](#rfc3623-5-2)

The grace-LSAs must be originated and be sent *before* the restarted router sends any OSPF Hello Packets. On broadcast networks, this LSA must be flooded to the AllSPFRouters multicast address (224.0.0.5) since the restarting router is not aware of its previous DR state. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (continuation 7, judge): D-8 fixed per R32 and proven. openInterfaces sets helloHold, opens and registers every enrolled interface (startInterfaceLocked defers rt.Start into helloHeld), calls maybeUnplannedRestart, then releaseHelloHold starts the held runtimes under e.mu; no return path between hold and release, a runtime replaced or deleted during the hold is looked up by name at release. TestRFC3623UnplannedColdStartOriginatesGraceLSA drives the real cold start over a recording transport: in-restart, Grace-LSA held on eth0, the FIRST packet sent is an LSU carrying the Grace-LSA to 224.0.0.5, the first Hello strictly after it. Revert record on openInterfaces observed red (HEAD's order originated zero Grace-LSAs). Single-polarity positive per annotation (no refusal path); the annotation's file:line references are dated but its claim now holds.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC3623RestartGraceLSAFloodedToAllSPFRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3623_restart_grace_flood_test.go#L45) | unit/verify | revert, verified |
| positive | [`TestUnplannedGraceLSAFloodsToAllSPFRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L151) | unit/verify | unproven |
| positive | [`TestRFC3623UnplannedColdStartOriginatesGraceLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_unplanned_cold_start_test.go#L131) | unit/verify | revert, verified |

### [`RFC3623-5-3`](#rfc3623-5-3)

The grace-LSAs are encapsulated in Link State Update Packets and sent out to all interfaces, even though the restarted router has no adjacencies and no knowledge of previous adjacencies. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Producer change re-read: floodLink now sends floodCopy(lsa, TransmitDelay) (RFC 2328 13.3 (5) age bump) on the restart branch instead of the database copy; the branch condition and destination (allSPFRoutersFor) are unchanged. floodLink sends a Grace-LSA originated in restart in a Link State Update even when no neighbor qualified for the retransmit list. TestRFC3623RestartGraceLSAFloodedToAllSPFRouters asserts one send with no neighbor while restarting and none outside restart; revert record on lsdb/link_scope.go::floodLink re-recorded against the new body and observed red. TestUnplannedGraceLSAPerActiveInterface pins one Grace-LSA per active interface. Single-polarity positive per the row annotation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC3623RestartGraceLSAFloodedToAllSPFRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3623_restart_grace_flood_test.go#L43) | unit/verify | revert, verified |
| positive | [`TestUnplannedGraceLSAPerActiveInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L193) | unit/verify | unproven |

### [`RFC3623-5-4`](#rfc3623-5-4)

o The restart reason in the grace-LSAs must be set to 0 (unknown) or 3 (switch to redundant control processor). (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29 (independent judge). TestRFC3623UnplannedGraceLSACarriesUnplannedReason configures planned-and-unplanned, runs maybeUnplannedRestart on a running interface and decodes the installed Grace-LSA: reason 3, equal to the recorded e.gr.reason (revert record on maybeUnplannedRestart). The {single-polarity} annotation stands: the unplanned reason is a constant and there is no receive-side reason rejection.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestUnplannedGraceBeforeHello`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_unplanned_test.go#L58) | unit/verify | unproven |
| positive | [`TestRFC3623UnplannedGraceLSACarriesUnplannedReason`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_origination_test.go#L80) | unit/verify | revert, verified |

### [`RFC3623-3-1`](#rfc3623-3-1)

When helping over a virtual link, the helper must also continue to set bit V in its router-LSA for the virtual link's transit area (Section 12.4.1 of [1]). (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3623-3-1, so no unit is bound to it.

### [`RFC3623-3.1-1`](#rfc3623-3.1-1)

Router Y examines the link-state retransmission list for X over the associated network segment. - If there are any LSAs with LS types 1-5,7 on the list, then they all must be periodic refreshes. - If there are instead LSAs on the list whose contents have changed (see Section 3.3 of [7]), Y must refuse to enter helper mode. (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3623-3.1-1, so no unit is bound to it.

### [`RFC3623-3.2-1`](#rfc3623-3.2-1)

If Router Y aggregated adjacencies with Router X when entering helper mode (as described in section 3.1), it must also exit helper mode for all adjacencies with Router X when any one of the exit events occurs for an adjacency with Router X. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3623-3.2-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc3623.txt |
| Source fingerprint | 8c64b96d2a23301e |
| Record | rfc/extraction/rfc3623.json |
| Mapped sentences | 12 |
| Declined as scope | 8 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 1 | walked | not stated |
| `2` | not stated | 1 | walked | not stated |
| `2.1` | not stated | 2 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `2.3` | not stated | 0 | walked | not stated |
| `3` | not stated | 1 | walked | not stated |
| `3.1` | not stated | 2 | walked | not stated |
| `3.2` | not stated | 1 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 6 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 1 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 0 | walked | not stated |
| `9.2` | not stated | 0 | walked | not stated |
| `A` | not stated | 4 | walked | not stated |
| `B` | not stated | 0 | walked | not stated |
| `B.1` | not stated | 0 | walked | not stated |
| `B.2` | not stated | 1 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Introduction sentence describing what the mechanism needs from the neighbors ('which must cooperate in order for the restart to be graceful'); the helper obligations themselves are in Section 3. | Then there are the router's neighbors, which must cooperate in order for the restart to be graceful. |
| `2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Overview sentence announcing that Sections 2.1-2.3 follow ('it must change its OSPF processing somewhat'); it names no specific behavior. | After the router restarts/reloads, it must change its OSPF processing somewhat until it re-establishes full adjacencies with all its former fully-adjacent neighbors. |
| `2.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Lead-in to the numbered list that follows ('must perform the following actions'); the obligations are the list items, decided below. | In preparation for the graceful restart, Router X must perform the following actions before its software is restarted/reloaded: |
| `3.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | First half of the helper's retransmission-list check: RFC3623-3.1-1 states the same condition as its refusal ('LSAs whose contents have changed' rather than periodic refreshes). | - If there are any LSAs with LS types 1-5,7 on the list, then they all must be periodic refreshes. |
| `5:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Lead-in to the bullet list that follows ('The following points must be observed'); the obligations are the bullets, decided below. | The following points must be observed during this grace-LSA origination. |
| `5:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Second sentence of the same bullet; RFC3623-5-2 carries both the before-Hellos ordering and the AllSPFRouters (224.0.0.5) flood target. | On broadcast networks, this LSA must be flooded to the AllSPFRouters multicast address (224.0.0.5) since the restarting router is not aware of its previous DR state. |
| `8:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | RFC boilerplate: the IETF's standard intellectual-property notice inviting parties to disclose patents. It binds no implementation. | The IETF invites any interested party to bring to its attention any copyrights, patents or patent applications, or other proprietary rights which may cover technology that may be required to practice this standard. |
| `B.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | RFC Editor full-copyright statement ('this document itself may not be modified in any way'); it is a licence term, not a protocol requirement. | However, this document itself may not be modified in any way, such as by removing the copyright notice or references to the Internet Society or other Internet organizations, except as needed for the purpose of developing Internet standards in which case the procedures for copyrights defined in the Internet Standards process must be followed, or as required to translate it into languages other than English. |

## Superseded

No document obsoletes RFC 3623, so its obligations are stated where they were written.
