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
| Proven by a recorded break | 13.3% | 2 of 15 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

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
| Audit verdicts | 9 | of 13 gated MUSTs judged | 5 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 13 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
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
| Requirements | 26 |
| Gated MUST-level | 13 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 2 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 15 |
| Tagged units | 15 |
| Recorded audit verdicts | 9 |
| Discrimination records | 2 |
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
| Annotated instead of tested | 9 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **13** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC3623-A-2`](#rfc3623-a-2), [`RFC3623-A-3`](#rfc3623-a-3), [`RFC3623-A-4`](#rfc3623-a-4), [`RFC3623-5-1`](#rfc3623-5-1)

**Annotated instead of tested (9):** [`RFC3623-A-1`](#rfc3623-a-1), [`RFC3623-A-5`](#rfc3623-a-5), [`RFC3623-2.1-1`](#rfc3623-2.1-1), [`RFC3623-5-2`](#rfc3623-5-2), [`RFC3623-5-3`](#rfc3623-5-3), [`RFC3623-5-4`](#rfc3623-5-4), [`RFC3623-3-1`](#rfc3623-3-1), [`RFC3623-3.1-1`](#rfc3623-3.1-1), [`RFC3623-3.2-1`](#rfc3623-3.2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3623-A-1` | Additional Grace-LSA TLVs must be described in an Internet Draft and will be subject to the expert review of the OSPF Working Group. (§A) | MUST | A | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze originates only the three RFC-defined TLVs (types 1/2/3) and adds none, and its decoder ignores unrecognized types, so the IETF-process obligation for additional TLVs binds no ze behavior (internal/plugins/ospf/packet/grace_lsa.go:44, :64) |
| `RFC3623-A-2` | Grace Period (Type=1, length=4). The number of seconds that the router's neighbors should continue to advertise the router as fully adjacent, regardless of the state of database synchronization between the router and its neighbors. Since this time period began when grace-LSA's LS age was equal to 0, the grace period terminates when either: a) the LS age of the grace-LSA exceeds the value of a Grace Period or b) the grace-LSA is flushed. See Section 3.2 for other conditions that terminate graceful restart. This TLV must always appear in a grace-LSA. (§A) | MUST | A | **positive:** `unit/verify` [`TestGraceLSARoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/grace_lsa_test.go#L20). **negative:** `unit/verify` [`TestGraceLSADecodeMissingMandatory`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/grace_lsa_test.go#L75) |
| `RFC3623-A-3` | Graceful restart reason (Type=2, length=1). Encodes the reason for the router restart as one of the following: 0 (unknown), 1 (software restart), 2 (software reload/upgrade) or 3 (switch to redundant control processor). This TLV must always appear in a grace-LSA. (§A) | MUST | A | **positive:** `unit/verify` [`TestGraceLSARoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/grace_lsa_test.go#L24). **negative:** `unit/verify` [`TestGraceLSADecodeMissingMandatory`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/grace_lsa_test.go#L72) |
| `RFC3623-A-4` | IP interface address (Type=3, length=4). The router's IP interface address on the subnet associated with the grace-LSA. Required on broadcast, NBMA and Point-to-MultiPoint segments, where the helper uses the IP interface address to identify the restarting router (see Section 3.1). (§A) | MUST | A | **positive:** `unit/verify` [`TestGraceLSAv4BodyBuild`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/gr_lsa_test.go#L18). **negative:** `unit/verify` [`TestGraceLSAv4BodyBuild`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/gr_lsa_test.go#L22) |
| `RFC3623-A-5` | DoNotAge is never set in a grace-LSA, even if the grace-LSA is flooded over a demand circuit [7]. (§A) | MUST | A | **positive:** `unit/verify` [`TestGraceLSANeverSetsDoNotAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L62). **negative:** no negative test. **{single-polarity}:** Grace-LSAs originate through OriginateOpaque, whose input struct has no DoNotAge field and starts LS age at 0 with normal aging, so the bit is never set and no negative behavior exists to test (internal/plugins/ospf/lsdb/opaque_as.go:73, gr_restarter.go:314, lsdb/entry.go:87) |
| `RFC3623-2.1-1` | In preparation for the graceful restart, Router X must perform the following actions before its software is restarted/reloaded: (Note that common OSPF shutdown procedures are *not* performed, since we want the other OSPF routers to act as if Router X remains in continuous service. For example, Router X does not flush its locally originated LSAs, since we want them to remain in other routers' link-state databases throughout the restart period.) 1) Router X must ensure that its forwarding table(s) is/are up- to-date and will remain in place across the restart. (§2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestPrepareRestartRetainsFIB`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L91). **negative:** no negative test. **{single-polarity}:** prepareRestart raises gracefulStop so suppressInstall makes the ensuing engine stop skip RemoveAll and retain the pre-restart FIB; the meaningful assertion is retention, with no negative behavior the requirement forbids (internal/plugins/ospf/gr_restarter.go:49, gr.go:235) |
| `RFC3623-5-1` | In any event, implementors providing the option to recover gracefully from unplanned outages must allow a network operator to turn the option off. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestUnplannedGraceBeforeHello`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/gr_unplanned_test.go#L40). **negative:** `unit/verify` [`TestUnplannedDisabledByDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/gr_unplanned_test.go#L16) |
| `RFC3623-5-2` | The grace-LSAs must be originated and be sent *before* the restarted router sends any OSPF Hello Packets. On broadcast networks, this LSA must be flooded to the AllSPFRouters multicast address (224.0.0.5) since the restarting router is not aware of its previous DR state. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC3623RestartGraceLSAFloodedToAllSPFRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3623_restart_grace_flood_test.go#L45). **positive:** `unit/verify` [`TestUnplannedGraceLSAFloodsToAllSPFRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L151). **negative:** no negative test. **{single-polarity}:** maybeUnplannedRestart enters in-restart then originates one Grace-LSA per interface before interface Hellos begin, and link-local LSAs flood to AllSPFRouters via the standard link-scope path (internal/plugins/ospf/gr_restarter.go:105, lsdb_flooding_test.go:47) |
| `RFC3623-5-3` | The grace-LSAs are encapsulated in Link State Update Packets and sent out to all interfaces, even though the restarted router has no adjacencies and no knowledge of previous adjacencies. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC3623RestartGraceLSAFloodedToAllSPFRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3623_restart_grace_flood_test.go#L43). **positive:** `unit/verify` [`TestUnplannedGraceLSAPerActiveInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L193). **negative:** no negative test. **{single-polarity}:** grOriginateGraceLSAs walks every active (non-passive) interface and floods each Grace-LSA via OriginateOpaque's standard LSU flooding (internal/plugins/ospf/gr_restarter.go:296, :315) |
| `RFC3623-5-4` | o The restart reason in the grace-LSAs must be set to 0 (unknown) or 3 (switch to redundant control processor). (§5) | MUST | 5 | **positive:** `unit/verify` [`TestUnplannedGraceBeforeHello`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/gr_unplanned_test.go#L58). **negative:** no negative test. **{single-polarity}:** grUnplannedReason returns the constant 3 (redundant control processor), an in-range unplanned reason, and there is no receive-side reason rejection to test negatively (internal/plugins/ospf/gr_restarter.go:88, gr.go:36) |
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

Audit verdict: enforced (the tests do what the requirement demands), fresh. The quote describes the Grace Period TLV and ends with its MUST: it always appears in a grace-LSA. Forbidden behaviour (a): a grace-LSA emitted without the type-1 TLV. (b) TestGraceLSARoundTrip asserts len(body) == opaqueTLVsLen(period, reason, addr) and a round-trip equal to the input, so an EncodeGraceLSA that dropped type 1 goes red; the negative TestGraceLSADecodeMissingMandatory asserts DecodeGraceLSA errors on the reasonOnly body. EncodeGraceLSA emits type 1 unconditionally. The descriptive clauses on the TLV value (neighbors keep advertising, grace-period end) are helper behaviour stated in sec 3 rows, not this row's obligation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGraceLSADecodeMissingMandatory`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/grace_lsa_test.go#L75) | unit/verify | unproven |
| positive | [`TestGraceLSARoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/grace_lsa_test.go#L20) | unit/verify | unproven |

### [`RFC3623-A-3`](#rfc3623-a-3)

Graceful restart reason (Type=2, length=1). Encodes the reason for the router restart as one of the following: 0 (unknown), 1 (software restart), 2 (software reload/upgrade) or 3 (switch to redundant control processor). This TLV must always appear in a grace-LSA. (§A)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden behaviour (a): a grace-LSA emitted without the type-2 Graceful restart reason TLV. (b) TestGraceLSARoundTrip asserts the body length includes the 1-octet reason TLV padded to 4 and the round-trip returns Reason=2, so omission goes red; TestGraceLSADecodeMissingMandatory asserts DecodeGraceLSA errors on the periodOnly body. The reason value range is a descriptive list, the MUST is presence.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGraceLSADecodeMissingMandatory`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/grace_lsa_test.go#L72) | unit/verify | unproven |
| positive | [`TestGraceLSARoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/grace_lsa_test.go#L24) | unit/verify | unproven |

### [`RFC3623-A-4`](#rfc3623-a-4)

IP interface address (Type=3, length=4). The router's IP interface address on the subnet associated with the grace-LSA. Required on broadcast, NBMA and Point-to-MultiPoint segments, where the helper uses the IP interface address to identify the restarting router (see Section 3.1). (§A)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The TLV is required on broadcast, NBMA and Point-to-MultiPoint segments. TestGraceLSAv4BodyBuild passes sharedMedia=true or false straight into grV4Body, so it proves only that the flag adds the TLV. The mapping from segment type to that flag, grSharedMedia (gr_lsa.go), which the restarter calls, is asserted nowhere: grSharedMedia returning false for NBMA or Point-to-MultiPoint (or broadcast) keeps both tagged units green. The negative (p2p omits the TLV) asserts behaviour the RFC does not forbid.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGraceLSAv4BodyBuild`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/gr_lsa_test.go#L22) | unit/verify | unproven |
| positive | [`TestGraceLSAv4BodyBuild`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/gr_lsa_test.go#L18) | unit/verify | unproven |

### [`RFC3623-A-5`](#rfc3623-a-5)

DoNotAge is never set in a grace-LSA, even if the grace-LSA is flooded over a demand circuit [7]. (§A)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden behaviour (a): a grace-LSA whose LS age has the DoNotAge bit set. (b) TestGraceLSANeverSetsDoNotAge drives OriginateOpaque with the restarter's body and asserts grace.Header.Age.DoNotAge() is false and RawBytes[0]&0x80 == 0, so an origination that set the bit goes red. The demand-circuit clause has no path in ze: no production code sets DoNotAgeBit (only types/lsage.go declares and reads it) and flooding copies the installed RawBytes, so the asserted bytes are the flooded bytes. Single-polarity marker holds.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestGraceLSANeverSetsDoNotAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L62) | unit/verify | unproven |

### [`RFC3623-2.1-1`](#rfc3623-2.1-1)

In preparation for the graceful restart, Router X must perform the following actions before its software is restarted/reloaded: (Note that common OSPF shutdown procedures are *not* performed, since we want the other OSPF routers to act as if Router X remains in continuous service. For example, Router X does not flush its locally originated LSAs, since we want them to remain in other routers' link-state databases throughout the restart period.) 1) Router X must ensure that its forwarding table(s) is/are up- to-date and will remain in place across the restart. (§2.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The quote requires the forwarding table to be up to date and to remain in place across the restart. TestPrepareRestartRetainsFIB asserts only that suppressInstall() turns true after prepareRestart. No tagged unit drives the engine stop and observes that the FIB entries survive (an engine stop that ignored the flag and ran RemoveAll stays green), and nothing asserts the table is brought up to date before the reload (a pending SPF result left uninstalled stays green).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestPrepareRestartRetainsFIB`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L91) | unit/verify | unproven |

### [`RFC3623-5-1`](#rfc3623-5-1)

In any event, implementors providing the option to recover gracefully from unplanned outages must allow a network operator to turn the option off. (§5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The obligation is that the operator can turn unplanned recovery off. The tagged units set gracefulRestartConfig directly (grEnableEngine / configure) and assert the engine gate: off means no restart, planned-and-unplanned means restart. The operator surface, the restarter-support leaf parsed in config.go (case planned-and-unplanned), is not exercised, so a parser that mapped every value to planned-and-unplanned, leaving the operator no off switch, keeps both units green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestUnplannedDisabledByDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/gr_unplanned_test.go#L16) | unit/verify | unproven |
| positive | [`TestUnplannedGraceBeforeHello`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/gr_unplanned_test.go#L40) | unit/verify | unproven |

### [`RFC3623-5-2`](#rfc3623-5-2)

The grace-LSAs must be originated and be sent *before* the restarted router sends any OSPF Hello Packets. On broadcast networks, this LSA must be flooded to the AllSPFRouters multicast address (224.0.0.5) since the restarting router is not aware of its previous DR state. (§5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. floodLink sends a Grace-LSA originated in restart (isGraceLSA, selfFlushSuppressed) to AllSPFRouters on every interface type (allSPFRoutersFor). TestRFC3623RestartGraceLSAFloodedToAllSPFRouters asserts a broadcast DROther interface sends it to 224.0.0.5, not AllDRouters. Weak: the 'before any Hello' ordering clause is not asserted by this unit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC3623RestartGraceLSAFloodedToAllSPFRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3623_restart_grace_flood_test.go#L45) | unit/verify | revert, verified |
| positive | [`TestUnplannedGraceLSAFloodsToAllSPFRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L151) | unit/verify | unproven |

### [`RFC3623-5-3`](#rfc3623-5-3)

The grace-LSAs are encapsulated in Link State Update Packets and sent out to all interfaces, even though the restarted router has no adjacencies and no knowledge of previous adjacencies. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. floodLink sends a Grace-LSA originated in restart in a Link State Update even when no neighbor qualified for the retransmit list. TestRFC3623RestartGraceLSAFloodedToAllSPFRouters asserts one send with no neighbor while restarting and none outside restart; red observed with the restart branch disabled (scratch DF-OSPF-B/red-gr.log).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC3623RestartGraceLSAFloodedToAllSPFRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3623_restart_grace_flood_test.go#L43) | unit/verify | revert, verified |
| positive | [`TestUnplannedGraceLSAPerActiveInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L193) | unit/verify | unproven |

### [`RFC3623-5-4`](#rfc3623-5-4)

o The restart reason in the grace-LSAs must be set to 0 (unknown) or 3 (switch to redundant control processor). (§5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The obligation is on the reason carried IN the grace-LSAs. The tagged unit asserts grUnplannedReason() and e.gr.reason are 0 or 3, but never decodes an originated grace-LSA (the engine in TestUnplannedGraceBeforeHello has no running interface, so none is originated). maybeUnplannedRestart passing another reason to grOriginateGraceLSAs than it records keeps the unit green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestUnplannedGraceBeforeHello`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/gr_unplanned_test.go#L58) | unit/verify | unproven |

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
