# RFC 8654 - Extended Message Support for BGP

Partial. Every requirement this repository extracted from RFC 8654, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 75.0% | 9 of 12 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 8.3% | 1 of 12 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 12 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 12 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 12 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 55.2% | 32 of 58 tagged units, 0 escaped and 2 lapsed | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 12 | of 16 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 12 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 16.7% | 2 of 12 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 12 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 12 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 12 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 16 |
| Gated MUST-level | 12 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 58 |
| Tagged units | 58 |
| Recorded audit verdicts | 8 |
| Discrimination records | 34 |
| Summary | `rfc/short/rfc8654.md` |
| Requirement shard | `rfc/requirements/rfc8654.md` |
| RFC text | `rfc/full/rfc8654.txt` |

## Enrolment

Enrolled: Extended Message capability and framing are implemented in internal/component/bgp. Receive permission follows local advertisement; send permission follows peer advertisement. Tests exercise accumulation, RFC7606 discard and route removal, fatal-error cleanup and replacement-best delivery. These proofs do not establish every incorporated RFC obligation or the exact final route-owner release/admission ordering.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

Directional Extended Message capability and 65535-byte receive limit after local advertisement.

**What the ledger says remains**

Correction 2026-10-06: the earlier weak classification described incomplete tests, not an absent Extended Message implementation. Native host execution and independent rejudgment now support the repaired directional, accumulation, discard/removal and fatal-error replacement assertions. Exact last-owner release versus outbound advertisement admission remains unproven; the ledger separately reports unaudited obligations and missing discrimination records. This is not a whole-RFC conformance claim.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 9 | one part of the gated population |
| Annotated (including scoped evidence) | 3 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **12** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (9):** [`RFC8654-3-1`](#rfc8654-3-1), [`RFC8654-3-3`](#rfc8654-3-3), [`RFC8654-4-1`](#rfc8654-4-1), [`RFC8654-4-2`](#rfc8654-4-2), [`RFC8654-4-3`](#rfc8654-4-3), [`RFC8654-6-1`](#rfc8654-6-1), [`RFC8654-5-1`](#rfc8654-5-1), [`RFC8654-5-2`](#rfc8654-5-2), [`RFC8654-5-4`](#rfc8654-5-4)

**Annotated (including scoped evidence) (3):** [`RFC8654-3-2`](#rfc8654-3-2), [`RFC8654-5-3`](#rfc8654-5-3), [`RFC8654-5-5`](#rfc8654-5-5)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8654-3-1` | Peers that wish to use the BGP Extended Message Capability MUST support error handling for BGP UPDATE messages per [RFC7606]. (§3) | MUST | 3 - BGP Extended Message Capability | **positive:** `unit/verify` [`TestRFC7606MPReachNLRIConsistentWithAFISAFIAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L124). **positive:** `unit/verify` [`TestRFC8654ExtendedAttributeDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go#L305). **positive:** `unit/verify` [`TestRFC8654ExtendedMessageSessionUsesRFC7606ErrorHandling`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go#L247). **positive:** `unit/verify` [`TestRFC8654TreatAsWithdrawRemovesInstalledRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_rib_recovery_test.go#L32). **negative:** `unit/verify` [`TestRFC7606AttributeLengthConflictTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L238). **negative:** `unit/verify` [`TestRFC8654ExtendedAttributeDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go#L306). **negative:** `unit/verify` [`TestRFC8654ExtendedMessageSessionUsesRFC7606ErrorHandling`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go#L249). **negative:** `unit/verify` [`TestRFC8654TreatAsWithdrawRemovesInstalledRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_rib_recovery_test.go#L33) |
| `RFC8654-3-2` | The BGP Extended Message Capability is a new BGP capability [RFC5492] defined with Capability Code 6 (§3) | MUST | 3 - BGP Extended Message Capability | **positive:** `unit/verify` [`TestCapabilityCodeConstants`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L29). **positive:** `unit/verify` [`TestRFC8654ExtendedMessageWireIsCode6Length0`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc8654_extended_message_test.go#L26). **negative:** no negative test. **{single-polarity}:** the capability-code assignment is a single fixed value, so the only falsifiable check is that CodeExtendedMessage encodes as 6; there is no distinct rejection behavior for a negative case to exercise |
| `RFC8654-3-3` | The BGP Extended Message Capability is a new BGP capability [RFC5492] defined with Capability Code 6 and Capability Length 0. (§3) | MUST | 3 - BGP Extended Message Capability | **positive:** `unit/verify` [`TestCapabilityRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L523). **positive:** `unit/verify` [`TestRFC8654ExtendedMessageWireIsCode6Length0`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc8654_extended_message_test.go#L27). **negative:** `unit/verify` [`TestRFC8654ExtendedMessageNonZeroLengthRefused`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc8654_extended_message_test.go#L52) |
| `RFC8654-4-1` | An implementation that advertises the BGP Extended Message Capability MUST be capable of receiving a message with a length up to and including 65,535 octets (§4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestRFC8654ReceiveLimitFollowsLocalOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L28). **positive:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L265). **negative:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L256). **positive:** `interop/nightly` [`checkExtendedAsymmetricFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_extended_message_scenarios.go#L13) |
| `RFC8654-4-2` | Applications generating information that might be encapsulated within BGP messages MUST limit the size of their payload to take the maximum message size into account. (§4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestBuildUnicast_MaxSize_Fits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L1613). **positive:** `unit/verify` [`TestSendPluginRoutesLeavesAFittingGroupWhole`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L366). **positive:** `unit/verify` [`TestSendPluginRoutesTracksTheNegotiatedMaximum`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L394). **positive:** `unit/verify` [`TestSendUpdateWithSplitLeavesAFittingPayloadWhole`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L187). **positive:** `unit/verify` [`TestSendUpdateWithSplitTracksTheNegotiatedMaximum`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L224). **positive:** `unit/verify` [`TestSplitUpdate_VPNChunksFitMaxMessageSize`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8654_update_split_test.go#L1416). **negative:** `unit/verify` [`TestBuildUnicast_MaxSize_TooLarge`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L1585). **negative:** `unit/verify` [`TestSendPluginRoutesBoundsAnOversizeGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L330). **negative:** `unit/verify` [`TestSendUpdateWithSplitBoundsAnOversizePayload`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L143) |
| `RFC8654-4-3` | The BGP Extended Message Capability applies to all messages except for OPEN and KEEPALIVE messages. (§4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestMaxMessageLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L321). **negative:** `unit/verify` [`TestRFC8654AdvertisementDoesNotExtendControlBounds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L259). **negative:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L241) |
| `RFC8654-6-1` | [RFC4271] states "The value of the Length field MUST always be at least 19 and no greater than 4096." This document changes the latter number to 65,535 for all messages except for OPEN and KEEPALIVE messages. Section 6.1 of [RFC4271] specifies raising an error if the length of a message is over 4,096 octets. For all messages except for OPEN and KEEPALIVE messages, if the receiver has advertised the BGP Extended Message Capability, this document raises that limit to 65,535. (§6) | MUST | 6 - Changes to RFC 4271 | **positive:** `unit/verify` [`TestRFC8654AnnouncementAccumulation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L108). **positive:** `unit/verify` [`TestRFC8654ExtendedMessageDirections`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc8654_extended_message_test.go#L69). **positive:** `unit/verify` [`TestRFC8654LengthBoundsPerTypeAndCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8654_length_bounds_test.go#L35). **positive:** `unit/verify` [`TestRFC8654ReceiveLimitFollowsLocalOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L26). **positive:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L253). **negative:** `unit/verify` [`TestRFC8654AnnouncementAccumulation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L109). **negative:** `unit/verify` [`TestRFC8654ExtendedMessageDirections`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc8654_extended_message_test.go#L70). **negative:** `unit/verify` [`TestRFC8654LengthBoundsPerTypeAndCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8654_length_bounds_test.go#L38). **negative:** `unit/verify` [`TestRFC8654ReceiveLimitFollowsLocalOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L27). **negative:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L248) |
| `RFC8654-5-1` | A BGP speaker that has the ability to use BGP Extended Messages but has not advertised the BGP Extended Message Capability, presumably due to configuration, MUST NOT accept a BGP Extended Message. (§5) | MUST NOT | 5 - Error Handling | **positive:** `unit/verify` [`TestRFC8654ReceiveLimitFollowsLocalOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L29). **positive:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L267). **negative:** `unit/verify` [`TestRFC8654ReceiveLimitFollowsLocalOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L30). **negative:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L259). **negative:** `interop/nightly` [`checkExtendedNeitherRejectFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_extended_message_scenarios.go#L76). **negative:** `interop/nightly` [`checkExtendedRemoteOnlyRejectFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_extended_message_scenarios.go#L58) |
| `RFC8654-5-2` | A speaker MUST NOT implement a more liberal policy accepting BGP Extended Messages (§5) | MUST NOT | 5 - Error Handling | **positive:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L269). **negative:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L261) |
| `RFC8654-5-3` | However, if a NOTIFICATION is to be sent to a BGP speaker that has not advertised the BGP Extended Message Capability, the size of the message MUST NOT exceed 4,096 octets. (§5) | MUST NOT | 5 - Error Handling | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never generates a NOTIFICATION near 4096 octets -- notification.go:191 sizes it as 19 + 2 + len(Data) and the Administrative Shutdown Communication is truncated to 128 octets (internal/component/bgp/message/notification.go:311-313), so there is no over-4096 NOTIFICATION code path to cap |
| `RFC8654-5-4` | Similarly, any speaker that treats an improper BGP Extended Message as a fatal error MUST follow the error-handling procedures of [RFC4271]. (§5) | MUST | 5 - Error Handling | **positive:** `unit/verify` [`TestRFC8654FatalLengthReelectsAlternateBest`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_rib_recovery_test.go#L172). **positive:** `unit/verify` [`TestRFC8654FatalLengthReleasesInstalledRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_fatal_length_cleanup_test.go#L51). **positive:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L298). **negative:** `unit/verify` [`TestRFC8654LengthBoundsPerTypeAndCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8654_length_bounds_test.go#L41). **negative:** `unit/verify` [`TestRFC8654OverLengthUpdateFollowsRFC4271ErrorHandling`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_bad_length_session_test.go#L27). **negative:** `unit/verify` [`TestRFC8654ValidLengthRetainsInstalledRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_fatal_length_cleanup_test.go#L128). **negative:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L290) |
| `RFC8654-5-5` | Future protocol specifications MUST describe how to handle peers that can only accommodate 4,096 octet messages. (§5) | MUST | 5 - Error Handling | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this obligation binds the author of a specification that defines a new BGP message type to state its extended-message eligibility; it is not a runtime behavior ze implements |
| `RFC8654-4-4` | A BGP speaker that is capable of receiving BGP Extended Messages SHOULD advertise the BGP Extended Message Capability to its peers using BGP Capabilities Advertisement [RFC5492]. (§4) | SHOULD | 4 - Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC8654-4-5` | When propagating that UPDATE onward to a neighbor that has not advertised the BGP Extended Message Capability, the speaker SHOULD try to reduce the outgoing message size by removing attributes eligible under the "attribute discard" approach of [RFC7606]. (§4) | SHOULD | 4 - Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC8654-5-6` | BGP protocol developers and implementers are conservative in their application and use of BGP Extended Messages (§5) | RECOMMENDED | 5 - Error Handling | **positive:** no positive test. **negative:** no negative test |
| `RFC8654-4-6` | A BGP speaker MAY send BGP Extended Messages to a peer only if the BGP Extended Message Capability was received from that peer (§4) | MAY | 4 - Operation | **positive:** `unit/verify` [`TestRFC8654SendLimitFollowsPeerOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L196). **negative:** `unit/verify` [`TestRFC8654SendLimitFollowsPeerOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L197). **positive:** `interop/nightly` [`checkExtendedAsymmetricFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_extended_message_scenarios.go#L17). **negative:** `interop/nightly` [`checkExtendedSendDeniedFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_extended_message_scenarios.go#L40) |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8654-5-3`](#rfc8654-5-3) However, if a NOTIFICATION is to be sent to a BGP speaker that has not advertised the BGP Extended Message Capability, the size of the message MUST NOT exceed 4,096 octets. (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze never generates a NOTIFICATION near 4096 octets -- notification.go:191 sizes it as 19 + 2 + len(Data) and the Administrative Shutdown Communication is truncated to 128 octets (internal/component/bgp/message/notification.go:311-313), so there is no over-4096 NOTIFICATION code path to cap |
| [`RFC8654-5-5`](#rfc8654-5-5) Future protocol specifications MUST describe how to handle peers that can only accommodate 4,096 octet messages. (§5) | no test | no test carries this requirement id; annotated {not-applicable}: this obligation binds the author of a specification that defines a new BGP message type to state its extended-message eligibility; it is not a runtime behavior ze implements |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8654-3-1`](#rfc8654-3-1)

Peers that wish to use the BGP Extended Message Capability MUST support error handling for BGP UPDATE messages per [RFC7606]. (§3)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/reactor/session_read.go::processMessage moved. Independent current-source renewal. RFC8654 Section 3 (rfc/full/rfc8654.txt): "Peers that wish to use the BGP Extended Message Capability MUST support error handling for BGP UPDATE messages per [RFC7606]." The complete current cover set exercises exact None and TreatAsWithdraw validator results, an extended structural overrun yielding exact NOTIFICATION 3/1 with no dispatch, and isolated malformed ATOMIC_AGGREGATE/AGGREGATOR discard versus intact valid controls through both readers. TestRFC8654TreatAsWithdrawRemovesInstalledRoutes installs two routes with an otherwise valid 4097-octet UPDATE, changes only ORIGIN to 3, requires actual Adj-RIB-In and Loc-RIB removal and recipient TCP withdrawals, then requires fresh ORIGIN=1 recovery in storage, recipient TCP and both Loc-RIB publication callbacks on the same Established session without NOTIFICATION. Its retained ForwardHandles preserve publication evidence until producer cleanup; observers do not modify routing state. Source tracing confirms enforceRFC7606 dispatches the classified action, processMessage synthesizes withdrawals, notifyMessageReceiver publishes WireUpdate.Payload rather than the original malformed body, and the RIB and RS consumers process those withdrawals. The retained semantic observation valid-origin1-recovery-treated-as-withdraw failed specifically at fresh recovery while all eight discard controls passed; that historical targeted observation is distinct from current native producer-dependence records and is not claimed freshly rerun here. This judges implemented extended-message integration with RFC7606, not an independent audit of every incorporated RFC7606 clause. The fixture has no alternate source: live-source RS withdrawal/replacement remains excluded and unimplemented by this proof; source-DOWN replacement evidence must not be substituted for it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606AttributeLengthConflictTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L238) | unit/verify | unproven |
| negative | [`TestRFC8654TreatAsWithdrawRemovesInstalledRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_rib_recovery_test.go#L33) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC8654ExtendedAttributeDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go#L306) | unit/verify | revert, verified |
| negative | [`TestRFC8654ExtendedMessageSessionUsesRFC7606ErrorHandling`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go#L249) | unit/verify | revert, verified |
| positive | [`TestRFC7606MPReachNLRIConsistentWithAFISAFIAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L124) | unit/verify | unproven |
| positive | [`TestRFC8654TreatAsWithdrawRemovesInstalledRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_rib_recovery_test.go#L32) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC8654ExtendedAttributeDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go#L305) | unit/verify | revert, verified |
| positive | [`TestRFC8654ExtendedMessageSessionUsesRFC7606ErrorHandling`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go#L247) | unit/verify | revert, verified |

### [`RFC8654-3-2`](#rfc8654-3-2)

The BGP Extended Message Capability is a new BGP capability [RFC5492] defined with Capability Code 6 (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Code 6 is the whole obligation; row single-polarity positive (a fixed code has no refusal form). TestRFC8654ExtendedMessageWireIsCode6Length0 reads the octet WriteTo puts at a non-zero offset (06), Code()==6, and Parse(06 00) yields *ExtendedMessage: a code drift in WriteTo or in Parse's dispatch goes red. The old TestCapabilityCodeConstants tag (constant only) is supplementary. Judge 2026-09-30.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCapabilityCodeConstants`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L29) | unit/verify | unproven |
| positive | [`TestRFC8654ExtendedMessageWireIsCode6Length0`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc8654_extended_message_test.go#L26) | unit/verify | revert, verified |

### [`RFC8654-3-3`](#rfc8654-3-3)

The BGP Extended Message Capability is a new BGP capability [RFC5492] defined with Capability Code 6 and Capability Length 0. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive: WriteTo writes exactly 06 00 with Len 2 and nothing beyond, Parse(06 00) accepted. Negative: TestRFC8654ExtendedMessageNonZeroLengthRefused sends code 6 with length 1 and 4 and requires ErrInvalidLength and no capability. Judge break: parseZeroLengthCapability accepting a non-zero length (len>8 guard) turned the negative red. Single-polarity marker removal is correct: a refusal path exists.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8654ExtendedMessageNonZeroLengthRefused`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc8654_extended_message_test.go#L52) | unit/verify | revert, verified |
| positive | [`TestCapabilityRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L523) | unit/verify | unproven |
| positive | [`TestRFC8654ExtendedMessageWireIsCode6Length0`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc8654_extended_message_test.go#L27) | unit/verify | revert, verified |

### [`RFC8654-4-1`](#rfc8654-4-1)

An implementation that advertises the BGP Extended Message Capability MUST be capable of receiving a message with a length up to and including 65,535 octets (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L256) | unit/verify | unproven |
| positive | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L265) | unit/verify | unproven |
| positive | [`TestRFC8654ReceiveLimitFollowsLocalOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L28) | unit/verify | revert, verified |
| positive | [`checkExtendedAsymmetricFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_extended_message_scenarios.go#L13) | interop/nightly | revert, verified |

### [`RFC8654-4-2`](#rfc8654-4-2)

Applications generating information that might be encapsulated within BGP messages MUST limit the size of their payload to take the maximum message size into account. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Non-compliant: an UPDATE on the wire larger than the negotiated maximum. TestSendUpdateWithSplitBoundsAnOversizePayload drives Peer.sendUpdateWithSplit with an over-4096 payload to a non-extended peer and asserts every wire frame length <= 4096 with the NLRI preserved; TestSendUpdateWithSplitTracksTheNegotiatedMaximum asserts the bound is 65535 when negotiated; BuildUnicastWithMaxSize refuses an oversize build (ErrUpdateTooLarge). Both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildUnicast_MaxSize_TooLarge`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L1585) | unit/verify | unproven |
| negative | [`TestSendPluginRoutesBoundsAnOversizeGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L330) | unit/verify | unproven |
| negative | [`TestSendUpdateWithSplitBoundsAnOversizePayload`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L143) | unit/verify | unproven |
| positive | [`TestSplitUpdate_VPNChunksFitMaxMessageSize`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8654_update_split_test.go#L1416) | unit/verify | revert, verified |
| positive | [`TestBuildUnicast_MaxSize_Fits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L1613) | unit/verify | unproven |
| positive | [`TestSendPluginRoutesLeavesAFittingGroupWhole`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L366) | unit/verify | unproven |
| positive | [`TestSendPluginRoutesTracksTheNegotiatedMaximum`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L394) | unit/verify | unproven |
| positive | [`TestSendUpdateWithSplitLeavesAFittingPayloadWhole`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L187) | unit/verify | unproven |
| positive | [`TestSendUpdateWithSplitTracksTheNegotiatedMaximum`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L224) | unit/verify | unproven |

### [`RFC8654-4-3`](#rfc8654-4-3)

The BGP Extended Message Capability applies to all messages except for OPEN and KEEPALIVE messages. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Prior independent judgment retained after scope comparison, not gratuitously re-audited. The complete Section 4 exception sentence and tagged units are unchanged: TestMaxMessageLength, TestValidateLengthWithMax, and TestRFC8654AdvertisementDoesNotExtendControlBounds. The directional test file gained an import and sibling accumulation unit; its control-bound unit is unchanged in the scoped diff. OPEN 4097 and KEEPALIVE 20 still require exact NOTIFICATION 1/2 in both readers, while valid controls and all supported type maxima are pinned. No new foreign execution or canonical fingerprint computation is claimed. The parent stamping tool must compute current unit/file hashes.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L241) | unit/verify | unproven |
| negative | [`TestRFC8654AdvertisementDoesNotExtendControlBounds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L259) | unit/verify | revert, verified |
| positive | [`TestMaxMessageLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L321) | unit/verify | unproven |

### [`RFC8654-6-1`](#rfc8654-6-1)

[RFC4271] states "The value of the Length field MUST always be at least 19 and no greater than 4096." This document changes the latter number to 65,535 for all messages except for OPEN and KEEPALIVE messages. Section 6.1 of [RFC4271] specifies raising an error if the length of a message is over 4,096 octets. For all messages except for OPEN and KEEPALIVE messages, if the receiver has advertised the BGP Extended Message Capability, this document raises that limit to 65,535. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent current-source renewal. RFC8654 Section 6 (rfc/full/rfc8654.txt): "This document changes the latter number to 65,535 for all messages except for OPEN and KEEPALIVE messages." Also: "For all messages except for OPEN and KEEPALIVE messages, if the receiver has advertised the BGP Extended Message Capability, this document raises that limit to 65,535." The complete row retains both paragraphs, including the common 19-octet floor, raised upper bound, receiver-advertisement condition and OPEN/KEEPALIVE exceptions. Current tagged header carriers assert accepted per-type minima, invalid 0/18 lengths, standard and extended maxima, OPEN rejection at 4097/65535 and KEEPALIVE rejection at 20. Capability tests assert separate receive/send fields in Negotiated and EncodingCaps for all four combinations. Actual-OPEN reader tests cover both readers at 4096/4097/65535 with exact accepted payload or rejected-length notification and no dispatch. AnnouncementAccumulation uses individually standard-sized announcements to isolate accumulation from header admission: it asserts exact consumer batches beyond 4096 through exactly 65535, overflow flush, KEEPALIVE flush, no duplicate empty flush and standard-sized batching without local advertisement. Production receive-buffer selection, both header gates and coalescer body capacity use the local receive permission; negotiateWith's moved computation preserves that projection. Retained targeted observations separately changed coalescer capacity and coalesced header admission to bilateral gating: the former failed only local=true/peer=false accumulation at both prefix counts, and the latter failed only that direction's coalesced 4097/65535 admission while prescribed controls passed. Those historical semantic observations are not a mutation claim for the ordinary reader and are distinct from renewed native whole-producer panic dependence. No new execution or foreign scenario verification is claimed here.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L248) | unit/verify | unproven |
| negative | [`TestRFC8654LengthBoundsPerTypeAndCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8654_length_bounds_test.go#L38) | unit/verify | revert, verified |
| negative | [`TestRFC8654AnnouncementAccumulation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L109) | unit/verify | revert, verified |
| negative | [`TestRFC8654ReceiveLimitFollowsLocalOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L27) | unit/verify | revert, verified |
| negative | [`TestRFC8654ExtendedMessageDirections`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc8654_extended_message_test.go#L70) | unit/verify | revert, verified |
| positive | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L253) | unit/verify | unproven |
| positive | [`TestRFC8654LengthBoundsPerTypeAndCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8654_length_bounds_test.go#L35) | unit/verify | revert, verified |
| positive | [`TestRFC8654AnnouncementAccumulation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L108) | unit/verify | revert, verified |
| positive | [`TestRFC8654ReceiveLimitFollowsLocalOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L26) | unit/verify | revert, verified |
| positive | [`TestRFC8654ExtendedMessageDirections`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc8654_extended_message_test.go#L69) | unit/verify | revert, verified |

### [`RFC8654-5-1`](#rfc8654-5-1)

A BGP speaker that has the ability to use BGP Extended Messages but has not advertised the BGP Extended Message Capability, presumably due to configuration, MUST NOT accept a BGP Extended Message. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent current-source renewal. RFC8654 Section 5 (rfc/full/rfc8654.txt): "A BGP speaker that has the ability to use BGP Extended Messages but has not advertised the BGP Extended Message Capability, presumably due to configuration, MUST NOT accept a BGP Extended Message." TestRFC8654ReceiveLimitFollowsLocalOPEN exchanges actual OPENs for all four local/peer capability combinations and exercises both readers at Length 4096, 4097 and 65535. Without local advertisement it requires an error, zero consumer dispatch and exact NOTIFICATION 1/2 carrying the offending Length; local-only advertisement supplies the distinct accepting control with exact complete payload and Established continuity. Header tests independently pin rejection and permitted extended lengths. Both current FRR rejection carriers reach checkExtendedMessages and checkExtendedReceiveRejection: they distinguish Ze's original OPEN from the relay-delivered OPEN, require a real FRR oversized UPDATE, exact offending-length NOTIFICATION and closure of the original FRR session generation. Their current execution is parent-owned and is not inferred from source inspection. handleOpen derives local capabilities from the actual local OPEN; Negotiate retains local receive permission independently of peer send permission; both reader admission gates consume the receive permission before reading the body. negotiateWith now computes Negotiate under mu before taking writeMu, while pointer publication, initPathsLimit and writeBuf.Resize remain writer-locked. Prompt failure under a whole-body Negotiate panic establishes producer dependence and cleanup progress, not a semantic admission counterfactual.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L259) | unit/verify | unproven |
| negative | [`TestRFC8654ReceiveLimitFollowsLocalOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L30) | unit/verify | revert, verified |
| negative | [`checkExtendedNeitherRejectFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_extended_message_scenarios.go#L76) | interop/nightly | revert, verified |
| negative | [`checkExtendedRemoteOnlyRejectFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_extended_message_scenarios.go#L58) | interop/nightly | revert, verified |
| positive | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L267) | unit/verify | unproven |
| positive | [`TestRFC8654ReceiveLimitFollowsLocalOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L29) | unit/verify | revert, verified |

### [`RFC8654-5-2`](#rfc8654-5-2)

A speaker MUST NOT implement a more liberal policy accepting BGP Extended Messages (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L261) | unit/verify | unproven |
| positive | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L269) | unit/verify | unproven |

### [`RFC8654-5-3`](#rfc8654-5-3)

However, if a NOTIFICATION is to be sent to a BGP speaker that has not advertised the BGP Extended Message Capability, the size of the message MUST NOT exceed 4,096 octets. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8654-5-3, so no unit is bound to it.

### [`RFC8654-5-4`](#rfc8654-5-4)

Similarly, any speaker that treats an improper BGP Extended Message as a fatal error MUST follow the error-handling procedures of [RFC4271]. (§5)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/plugins/rib/rib.go::handleStructuredState, internal/component/bgp/reactor/relay_recovery.go::recoverNLRIBatch moved. Independent semantic rejudgment: RFC8654 Section 5: "Similarly, any speaker that treats an improper BGP Extended Message as a fatal error MUST follow the error-handling procedures of [RFC4271]." RFC4271 Section 6 says: "Before the invalid routes are deleted from the system, it advertises, to its peers, either withdraws for the routes marked as invalid, or the new best routes before the invalid routes are deleted from the system." Current fatal-length controls observe exact Notification 1/2, EOF/session loss, Adj-RIB-In/Loc-RIB removal and real downstream withdrawal. The formerly failing TestRFC8654FatalLengthReelectsAlternateBest now passes in job-enum-recovery-causal-race-941bd345.log with original-best preboundary, surviving source attributes on actual TCP, no transient withdrawal of the replaced route, and no-survivor withdrawal. Source retains the detached RS withdrawal inventory in a joined DOWN lifecycle, and recovery uses actual sent ownership/generation with applied-delivery drain before lookup. Omitting that drain now fails the newer-target ownership consumer assertion on both direct and external transports; forgetting prior delivery failure fails strict-drain assertions on both, while legitimate success controls pass. The race log also passes final-worker re-resolution, reconnect/generation refusal, full native Add-Path/pruning and hard-failure destination retirement. This replaces the obsolete failing-TCP objection with observed consumer evidence; canonical and foreign carrier proof remains separate. These are inspected scratch semantic executions, not canonical discrimination records. Current native record renewal/stamping remains parent-owned; no pending FRR carrier run, SRPM behavior, dataplane behavior, or full-RFC acceptance is inferred.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L290) | unit/verify | unproven |
| negative | [`TestRFC8654LengthBoundsPerTypeAndCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8654_length_bounds_test.go#L41) | unit/verify | revert, verified |
| negative | [`TestRFC8654OverLengthUpdateFollowsRFC4271ErrorHandling`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_bad_length_session_test.go#L27) | unit/verify | revert, verified |
| negative | [`TestRFC8654ValidLengthRetainsInstalledRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_fatal_length_cleanup_test.go#L128) | unit/verify | revert, verified |
| positive | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L298) | unit/verify | unproven |
| positive | [`TestRFC8654FatalLengthReleasesInstalledRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_fatal_length_cleanup_test.go#L51) | unit/verify | revert, verified |
| positive | [`TestRFC8654FatalLengthReelectsAlternateBest`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_rib_recovery_test.go#L172) | unit/verify | revert, verified |

### [`RFC8654-5-5`](#rfc8654-5-5)

Future protocol specifications MUST describe how to handle peers that can only accommodate 4,096 octet messages. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8654-5-5, so no unit is bound to it.

### [`RFC8654-4-6`](#rfc8654-4-6)

A BGP speaker MAY send BGP Extended Messages to a peer only if the BGP Extended Message Capability was received from that peer (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8654SendLimitFollowsPeerOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L197) | unit/verify | revert, verified |
| negative | [`checkExtendedSendDeniedFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_extended_message_scenarios.go#L40) | interop/nightly | revert, verified |
| positive | [`TestRFC8654SendLimitFollowsPeerOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L196) | unit/verify | revert, verified |
| positive | [`checkExtendedAsymmetricFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_extended_message_scenarios.go#L17) | interop/nightly | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6 phase 5, rfc8654 |
| Signed off | 2026-08-31 |
| Register | prose |
| Source | rfc/full/rfc8654.txt |
| Source fingerprint | b14e6e2fbf6eafd8 |
| Record | rfc/extraction/rfc8654.json |
| Mapped sentences | 9 |
| Declined as scope | 5 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | skipped (front-matter) | Title block, Abstract, Status of This Memo, Copyright Notice and Table of Contents. The Abstract restates section 1. Its one site is the IETF Trust Legal Provisions sentence, excluded below. |
| `1` | Introduction | 0 | walked | Introduction. Indicative prose: RFC 4271 mandates 4,096 octets, new AFIs, SAFIs and capabilities need more, and this document raises the limit to 65,535 octets for every message except OPEN and KEEPALIVE. No sentence directs a speaker. |
| `1.1` | Requirements Language | 0 | walked | Requirements Language. The BCP 14 key-words paragraph, which states that the key words bind when, and only when, they appear in all capitals. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. It is also what makes the lowercase modals in sections 4 and 8 non-normative in this document. |
| `2` | BGP Extended Message | 0 | walked | BGP Extended Message. Definitions only: a message over 4,096 octets is a BGP Extended Message, extended messages have a maximum size of 65,535 octets, and the smallest message is a 19-octet KEEPALIVE. Stated indicatively, so under section 1.1 it carries no RFC 2119 level. The 65,535 and 19 octet bounds are carried normatively by site 6:1, which maps RFC8654-6-1, and by the Wire Formats and Constants tables of rfc/short/rfc8654.md. |
| `3` | BGP Extended Message Capability | 1 | walked | BGP Extended Message Capability. Its one capitalised site is the RFC 7606 error-handling prerequisite, mapped below to RFC8654-3-1. The rest is value assignment and description: Capability Code 6, Capability Length 0, how a speaker advertises it with RFC 5492, and what advertising conveys. Those are indicative, so the two wire-value obligations the summary declares from them are the unsourced ids below. |
| `4` | Operation | 5 | walked | Operation. Two capitalised MUST-level sites, mapped below to RFC8654-4-1 and RFC8654-4-2. Its three remaining sites carry lowercase modals that section 1.1 gives no normative level, and each is excluded below. The section's other directives are the SHOULD to advertise the capability, the SHOULD to try attribute discard when propagating to a peer that did not advertise it, the MAY that permits sending an extended message only to a peer the capability was received from, and the scoping sentence that the capability applies to all messages except OPEN and KEEPALIVE; the summary declares those as the unsourced ids below. The sentence stating that a listener which has not advertised the capability will generate a NOTIFICATION with Bad Message Length is written in the indicative future and cites RFC 4271 Section 6.1 for the subcode, so the site scan cannot see it; rfc/short/rfc8654.md carries it in its Errors list and in the Error Handling table. |
| `5` | Error Handling | 5 | walked | Error Handling. The document's densest normative section: five capitalised MUST-level sites, all mapped below to RFC8654-5-1 through RFC8654-5-5. Its one advisory, the RECOMMENDED that developers and implementers are conservative in their use of extended messages, is the unsourced id below. The sentence stating that UPDATE error handling per RFC 4271 Section 6.3 is unchanged is indicative and adds no obligation. |
| `6` | Changes to RFC 4271 | 1 | walked | Changes to RFC 4271. Its one site quotes the RFC 4271 Length-field MUST and states the change to 65,535, mapped below to RFC8654-6-1. Its second paragraph restates the same change against RFC 4271 Section 6.1 in the indicative and adds no separate obligation. |
| `7` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records the allocation of value 6, "BGP Extended Message", in the "Capability Codes" registry. Binds IANA, not a speaker; the wire value itself is RFC8654-3-2. |
| `8` | Security Considerations | 1 | walked | Security Considerations. States that the extension does not change BGP's underlying security issues, that buffering 65,535-octet messages increases exposure to resource exhaustion, and lists three consequences of reducing an outgoing message for a peer that does not support extended messages. Its one site is the RFC 7606 eligibility criterion for attribute discard, excluded below. No countermeasure is directed at a speaker. |
| `9` | References heading | 0 | skipped (references) | References heading. |
| `9.1` | not stated | 0 | skipped (references) | Normative References: RFC 2119, RFC 4271, RFC 5492, RFC 7606, RFC 8174. |
| `9.2` | Informative References: RFC 4272, RFC 7752, RFC 8205 | 0 | skipped (references) | Informative References: RFC 4272, RFC 7752, RFC 8205. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Boilerplate the extractor did not strip: the IETF Trust Legal Provisions paragraph of the Copyright Notice. It states a licensing condition on code extracted from the document text and directs no protocol behavior at a BGP speaker. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `4:3` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The obligation belongs to RFC 4271 Section 9.2, which the sentence cites as its authority. RFC 8654 raises the Length ceiling only for a neighbor that advertised the capability, so for a neighbor that did not, RFC 4271's own 4,096-octet limit is what forbids the send. The modal is lowercase, which section 1.1 gives no RFC 2119 level in this document. | If the message is still too big, then it must not be sent to the neighbor ([RFC4271], Section 9.2). |
| `4:4` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The obligation belongs to RFC 4271 Section 9.1.3, which the sentence cites: withdrawing a route that can no longer be advertised is the Update-Send Process's own rule, not a new one this document states. The modal is lowercase, which section 1.1 gives no RFC 2119 level in this document. | Additionally, if the NLRI was previously advertised to that peer, it must be withdrawn from service ([RFC4271], Section 9.1.3). |
| `4:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AS operator, not a BGP speaker. The paragraph's own next sentences name that role: a consistent view is guaranteed only if all the iBGP speakers advertise the capability, and if that is not the case "the operator should consider whether or not" to advertise it to external peers. It is a deployment judgement about which speakers are configured to advertise, and no code path in a speaker can carry it. The role is the AS operator, so no producer could act as it. Ze CONSUMES the operator's decision: the capability is negotiated per session in the reactor (`internal/component/bgp/reactor`), which advertises what it is configured to and decides no AS-wide policy. | If an Autonomous System (AS) has multiple internal BGP speakers and also has multiple external BGP neighbors, care must be taken to ensure a consistent view within the AS in order to present a consistent external view. |
| `8:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | Eligibility for attribute discard is defined by RFC 7606, which the sentence cites. This sentence restates that criterion inside a security note about the consequence of discarding, and its modal is lowercase, which section 1.1 gives no RFC 2119 level in this document. | The attributes eligible under the "attribute discard" approach must have no effect on route selection or installation [RFC7606]. |

## Superseded

No document obsoletes RFC 8654, so its obligations are stated where they were written.
