# RFC 4555 - IKEv2 Mobility and Multihoming Protocol (MOBIKE)

Partial. Every requirement this repository extracted from RFC 4555, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 13 of 13 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 13 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 13 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 13 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 13 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 20.0% | 8 of 40 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 13 | of 23 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 13 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 13 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 13 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 13 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

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
| Requirements | 23 |
| Gated MUST-level | 13 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 40 |
| Tagged units | 40 |
| Recorded audit verdicts | 10 |
| Discrimination records | 8 |
| Summary | `rfc/short/rfc4555.md` |
| Requirement shard | `rfc/requirements/rfc4555.md` |
| RFC text | `rfc/full/rfc4555.txt` |

## Enrolment

Enrolled: Ze implements MOBIKE in its native IKE engine. All thirteen MUST-level requirements remain enrolled and have production paths plus newly authored positive/negative tests. Execution and discrimination remain required; no exclusion or conformance claim is made.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Tunnel-mode negotiation
- UDP 4500 source selection
- explicit address updates
- COOKIE2 generation, echo, comparison and failure teardown
- NO_NATS_ALLOWED sending and full receiver validation under the per-peer NAT policy
- NAT detection
- sequence-preserving live XFRM migration when the kernel supports XFRM_MSG_MIGRATE_STATE. Newly authored tests remain unrun.


**What the ledger says remains:**

Atomic migration requires Linux 7.2's API and CONFIG_XFRM_MIGRATE in the appliance build. Runtime verification, discrimination and interoperability results remain outstanding.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 13 | one part of the gated population |
| Annotated (including scoped evidence) | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **13** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (13):** [`RFC4555-x-1`](#rfc4555-x-1), [`RFC4555-x-2`](#rfc4555-x-2), [`RFC4555-3.9-1`](#rfc4555-3.9-1), [`RFC4555-3.7-1`](#rfc4555-3.7-1), [`RFC4555-3.8-1`](#rfc4555-3.8-1), [`RFC4555-3.8-2`](#rfc4555-3.8-2), [`RFC4555-3.6-1`](#rfc4555-3.6-1), [`RFC4555-3.7-4`](#rfc4555-3.7-4), [`RFC4555-3.7-5`](#rfc4555-3.7-5), [`RFC4555-3.9-3`](#rfc4555-3.9-3), [`RFC4555-3.9-4`](#rfc4555-3.9-4), [`RFC4555-4.2.1-1`](#rfc4555-4.2.1-1), [`RFC4555-4.2.5-1`](#rfc4555-4.2.5-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4555-x-1` | Implementations that wish to use MOBIKE for a particular IKE_SA MUST include a MOBIKE_SUPPORTED notification in the IKE_AUTH exchange (in case of multiple IKE_AUTH exchanges, in the message containing the SA payload). (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestMobikeAuthNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L651). **positive:** `unit/verify` [`TestRFC4555EAPOfferRidesTheIKEAuthMessageCarryingSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4555_mobike_offer_test.go#L91). **negative:** `unit/verify` [`TestMobikeAuthNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L652). **negative:** `unit/verify` [`TestRFC4555NoOwnOfferNoMOBIKE`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4555_mobike_offer_test.go#L132) |
| `RFC4555-x-2` | The addresses are taken from the IKE_AUTH request because IKEv2 requires changing from port 500 to 4500 if a NAT is discovered.  To simplify things, implementations that support both this specification and NAT Traversal MUST change to port 4500 if the correspondent also supports both, even if no NAT was detected between them (this way, there is no need to change the ports later if a NAT is detected on some other path). (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestMobikeAuthNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L654). **negative:** `unit/verify` [`TestMobikeAuthWithoutNATTSocketKeepsBaseIKE`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L712) |
| `RFC4555-3.9-1` | More specifically, when NAT Traversal is not enabled, all messages that can update the addresses associated with the IKE_SA and/or IPsec SAs (the first IKE_AUTH request and all INFORMATIONAL requests that contain any of the following notifications: UPDATE_SA_ADDRESSES, ADDITIONAL_IP4_ADDRESS, ADDITIONAL_IP6_ADDRESS, NO_ADDITIONAL_ADDRESSES) MUST also include a NO_NATS_ALLOWED notification. (NAT Prohibition, Section 3.9) | MUST | 3.9 | **positive:** `unit/verify` [`TestMobikeAdvertisementAfterSourceChange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L553). **positive:** `unit/verify` [`TestMobikeAuthRetransmitKeepsProtectedTuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1092). **positive:** `unit/verify` [`TestMobikeConfiguredNATPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1026). **negative:** `unit/verify` [`TestMobikeConfiguredNATPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1028) |
| `RFC4555-3.7-1` | The sender of an INFORMATIONAL request MAY include a COOKIE2 notification, and if included, the recipient of an INFORMATIONAL request MUST copy the notification as-is to the response. (§3.7) | MUST | 3.7 | **positive:** `unit/verify` [`TestMobikeCookie2EchoBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L234). **positive:** `unit/verify` [`TestMobikeOwnerDispatchRepliesFromArrivalSocket`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L29). **negative:** `unit/verify` [`TestMobikeCookie2EchoBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L235) |
| `RFC4555-3.8-1` | When MOBIKE is in use, the dynamic updates (specified in [IKEv2], Section 2.23), where the peer address and port are updated from the last valid authenticated packet, work in a slightly different fashion.  The host not behind a NAT MUST NOT use these dynamic updates for IKEv2 packets (§3.8) | MUST | 3.8 | **positive:** `unit/verify` [`TestMobikePathProbeRetainsTunnelEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L736). **negative:** `unit/verify` [`TestMobikePathProbeRetainsTunnelEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L738) |
| `RFC4555-3.8-2` | the NAT_DETECTION_SOURCE_IP and NAT_DETECTION_DESTINATION_IP notifications MAY be included in any INFORMATIONAL request; if the request includes them, the responder MUST also include them in the response (§3.8) | MUST | 3.8 | **positive:** `unit/verify` [`TestMobikePathProbeRetainsTunnelEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L740). **negative:** `unit/verify` [`TestMobikePathProbeRetainsTunnelEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L742) |
| `RFC4555-3.6-1` | If the request to update the addresses is retransmitted using several different source addresses, a new INFORMATIONAL request MUST be sent (Additional Addresses, Section 3.6) | MUST | 3.6 | **positive:** `unit/verify` [`TestMobikeAdvertisementAfterSourceChange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L546). **negative:** `unit/verify` [`TestMobikeAdvertisementAfterSourceChange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L548) |
| `RFC4555-3.7-4` | When processing the response, the original sender MUST verify that the value is the same one as sent. (§3.7) | MUST | 3.7 | **positive:** `unit/verify` [`TestMobikeCookie2ResponseControlsMigration`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L276). **negative:** `unit/verify` [`TestMobikeChangedPathStillChecksCookie2`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L998). **negative:** `unit/verify` [`TestMobikeCookie2ResponseControlsMigration`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L277) |
| `RFC4555-3.7-5` | If the values do not match, the IKE_SA MUST be closed. (§3.7) | MUST | 3.7 | **positive:** `unit/verify` [`TestMobikeChangedPathStillChecksCookie2`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1000). **positive:** `unit/verify` [`TestMobikeCookie2ResponseControlsMigration`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L278). **negative:** `unit/verify` [`TestMobikeCookie2ResponseControlsMigration`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L280) |
| `RFC4555-3.9-3` | The exchange responder MUST verify that the contents of the NO_NATS_ALLOWED notification match the addresses in the IP header (NAT Prohibition, Section 3.9) | MUST | 3.9 | **positive:** `unit/verify` [`TestMobikeNoNATsIPv4Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L393). **positive:** `unit/verify` [`TestMobikeNoNATsIPv6Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L507). **positive:** `unit/verify` [`TestMobikeNoNATsUpdateWaitsForRoutability`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L805). **negative:** `unit/verify` [`TestMobikeNoNATsIPv4Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L394). **negative:** `unit/verify` [`TestMobikeNoNATsIPv6Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L508) |
| `RFC4555-3.9-4` | If an UNEXPECTED_NAT_DETECTED notification is sent, the exchange responder MUST NOT use the contents of the NO_NATS_ALLOWED notification for any other purpose than possibly logging the information for troubleshooting purposes (NAT Prohibition, Section 3.9) | MUST NOT | 3.9 | **positive:** `unit/verify` [`TestMobikeNoNATsIPv4Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L396). **positive:** `unit/verify` [`TestMobikeNoNATsReplyUsesReceivedDestination`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L468). **negative:** `unit/verify` [`TestMobikeNoNATsIPv4Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L398). **negative:** `unit/verify` [`TestMobikeNoNATsUpdateWaitsForRoutability`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L807) |
| `RFC4555-4.2.1-1` | The notification data field MUST be left empty (zero-length) when sending, and its contents (if any) MUST be ignored when this notification is received. (Payload Formats, Section 4.2.1) | MUST | 4.2.1 | **positive:** `unit/verify` [`TestMobikeAuthNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L647). **negative:** `unit/verify` [`TestMobikeAuthNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L649) |
| `RFC4555-4.2.5-1` | The data associated with this notification MUST be between 8 and 64 octets in length (inclusive), and MUST be chosen by the exchange initiator in a way that is unpredictable to the exchange responder. (§4.2.5) | MUST | 4.2.5 | **positive:** `unit/verify` [`TestMobikeAdvertisementAfterSourceChange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L544). **positive:** `unit/verify` [`TestMobikeCookie2EchoBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L232). **positive:** `unit/verify` [`TestRFC4555Cookie2IsTheRandomSourceOutput`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4555_cookie2_test.go#L50). **negative:** `unit/verify` [`TestMobikeCookie2EchoBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L233) |
| `RFC4555-3.7-2` | By default, this "return routability check" SHOULD be performed. (§3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4555-3.8-3` | When the initiator is behind a NAT (as detected earlier using the NAT_DETECTION_SOURCE_IP and NAT_DETECTION_DESTINATION_IP notifications), it SHOULD include these notifications in DPD messages and compare the received NAT_DETECTION_DESTINATION_IP notifications with the value from the previous UPDATE_SA_ADDRESSES response (or the IKE_SA_INIT response). (§3.8) | SHOULD | 3.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC4555-3.9-2` | If the exchange initiator receives an UNEXPECTED_NAT_DETECTED notification in response to its INFORMATIONAL request, it SHOULD retry the operation several times using new INFORMATIONAL requests.  Similarly, if the initiator receives UNEXPECTED_NAT_DETECTED in the IKE_AUTH exchange, it SHOULD retry IKE_SA establishment several times, starting from a new IKE_SA_INIT request. (§3.9) | SHOULD | 3.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC4555-3.11-1` | To give the initiator enough time to detect the error, the responder SHOULD use relatively long timeout intervals when, for instance, retransmitting IKEv2 requests or deciding whether to initiate Dead Peer Detection.  While no specific timeout lengths are required, it is suggested that responders continue retransmitting IKEv2 requests for at least five minutes before giving up. (§3.11) | SHOULD | 3.11 | **positive:** no positive test. **negative:** no negative test |
| `RFC4555-3.12-1` | This means that when there are incoming IPsec packets, MOBIKE nodes SHOULD inspect the addresses used in those packets and determine that they correspond to those that should be employed. (§3.12) | SHOULD | 3.12 | **positive:** no positive test. **negative:** no negative test |
| `RFC4555-3.12-2` | If they do not, such packets SHOULD NOT be used as evidence that the peer is able to communicate with this node and or that the peer has received all address updates. (§3.12) | SHOULD NOT | 3.12 | **positive:** no positive test. **negative:** no negative test |
| `RFC4555-x-3` | In particular, simply storing the (remote tunnel header IP address, remote SPI) pair in the SPD cache is not sufficient, since the pair does not always uniquely identify a single SAD entry. (§A.1) | SHOULD NOT | A.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4555-x-4` | Both the initiator and responder MAY include one or more ADDITIONAL_IP4_ADDRESS and/or ADDITIONAL_IP6_ADDRESS notifications in the IKE_AUTH exchange (in case of multiple IKE_AUTH exchanges, in the message containing the SA payload). (§3.4) | MAY | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4555-3.7-3` | In environments where the peer is expected to be well-behaved (many corporate VPNs, for instance), or the address can be verified by some other means (e.g., a certificate issued by an authority trusted for this purpose), the return routability check MAY be omitted. (§3.7) | MAY | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4555-3.8-4` | When MOBIKE is in use, the dynamic updates (specified in [IKEv2], Section 2.23), where the peer address and port are updated from the last valid authenticated packet, work in a slightly different fashion.  The host not behind a NAT MUST NOT use these dynamic updates for IKEv2 packets, but MAY use them for ESP packets. (§3.8) | MAY | 3.8 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 4555 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4555-x-1`](#rfc4555-x-1)

Implementations that wish to use MOBIKE for a particular IKE_SA MUST include a MOBIKE_SUPPORTED notification in the IKE_AUTH exchange (in case of multiple IKE_AUTH exchanges, in the message containing the SA payload). (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (independent judge). Clause: the MOBIKE_SUPPORTED offer is in IKE_AUTH, and with several IKE_AUTH exchanges in the message carrying the SA payload. rfc4555_mobike_offer_test.go::TestRFC4555EAPOfferRidesTheIKEAuthMessageCarryingSA drives an EAP initiator: the first IKE_AUTH request (SA, no AUTH) decrypted off the wire carries MOBIKE_SUPPORTED, the later buildEAPResponse request carries neither SA nor the offer, and the peer's answer then enables MOBIKE. TestRFC4555NoOwnOfferNoMOBIKE is the negative for THIS row: an initiator whose own IKE_AUTH carried no offer (never built, or no migrating dataplane, checked off the wire) does not enable MOBIKE on the peer's offer (acceptMobikeOffer gates on mobike.offered). Revert records observed red on mobike.go::mobikeAuthOffer and ::acceptMobikeOffer. Single-exchange placement for both roles stays on TestMobikeAuthNegotiation. The responder's multi-exchange case is not driven end to end; its only offer site is buildAuthResponse (responder.go), the SA-carrying final response, after the SA payload unconditionally, the same builder TestMobikeAuthNegotiation asserts.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMobikeAuthNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L652) | unit/verify | revert, verified |
| negative | [`TestRFC4555NoOwnOfferNoMOBIKE`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4555_mobike_offer_test.go#L132) | unit/verify | revert, verified |
| positive | [`TestMobikeAuthNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L651) | unit/verify | revert, verified |
| positive | [`TestRFC4555EAPOfferRidesTheIKEAuthMessageCarryingSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4555_mobike_offer_test.go#L91) | unit/verify | revert, verified |

### [`RFC4555-x-2`](#rfc4555-x-2)

The addresses are taken from the IKE_AUTH request because IKEv2 requires changing from port 500 to 4500 if a NAT is discovered.  To simplify things, implementations that support both this specification and NAT Traversal MUST change to port 4500 if the correspondent also supports both, even if no NAT was detected between them (this way, there is no need to change the ports later if a NAT is detected on some other path). (§3.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a MOBIKE plus NAT-T implementation staying on port 500 for IKE_AUTH when no NAT is detected. internal/component/ike/engine/mobike_test.go::TestMobikeAuthNegotiation (through mbAuthHandshake) binds only NAT-T sockets, targets the port-500 stand-in at respTr-1, fails if ini.NATDetected, and requires the IKE_AUTH to arrive on respTr (mbReceive) with the non-ESP marker. Without the float the packet reaches no listener and the test goes red. Negative: internal/component/ike/engine/mobike_test.go::TestMobikeAuthWithoutNATTSocketKeepsBaseIKE shows that with no NAT-T support the condition does not hold and no MOBIKE offer is made.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMobikeAuthWithoutNATTSocketKeepsBaseIKE`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L712) | unit/verify | unproven |
| positive | [`TestMobikeAuthNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L654) | unit/verify | unproven |

### [`RFC4555-3.9-1`](#rfc4555-3.9-1)

More specifically, when NAT Traversal is not enabled, all messages that can update the addresses associated with the IKE_SA and/or IPsec SAs (the first IKE_AUTH request and all INFORMATIONAL requests that contain any of the following notifications: UPDATE_SA_ADDRESSES, ADDITIONAL_IP4_ADDRESS, ADDITIONAL_IP6_ADDRESS, NO_ADDITIONAL_ADDRESSES) MUST also include a NO_NATS_ALLOWED notification. (NAT Prohibition, Section 3.9)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an address-updating message sent without NO_NATS_ALLOWED when NAT is prohibited. Ze sends two kinds, the first IKE_AUTH and the INFORMATIONAL carrying UPDATE_SA_ADDRESSES and NO_ADDITIONAL_ADDRESSES. It never sends ADDITIONAL_IP4/IP6 (startMobikeRequest, mobike.go). First IKE_AUTH: mbAuthHandshake under policy prohibit (from internal/component/ike/engine/mobike_test.go::TestMobikeConfiguredNATPolicy) requires NO_NATS_ALLOWED with the actual tuple, and internal/component/ike/engine/mobike_test.go::TestMobikeAuthRetransmitKeepsProtectedTuple covers the retransmission. The INFORMATIONAL update is covered by TestMobikeConfiguredNATPolicy (notification == nil goes red) and by internal/component/ike/engine/mobike_test.go::TestMobikeAdvertisementAfterSourceChange (Linux only) for original and replacement updates. Negative: allow and default policies send none.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMobikeConfiguredNATPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1028) | unit/verify | unproven |
| positive | [`TestMobikeAdvertisementAfterSourceChange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L553) | unit/verify | unproven |
| positive | [`TestMobikeAuthRetransmitKeepsProtectedTuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1092) | unit/verify | unproven |
| positive | [`TestMobikeConfiguredNATPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1026) | unit/verify | unproven |

### [`RFC4555-3.7-1`](#rfc4555-3.7-1)

The sender of an INFORMATIONAL request MAY include a COOKIE2 notification, and if included, the recipient of an INFORMATIONAL request MUST copy the notification as-is to the response. (§3.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: answering an INFORMATIONAL request that carries COOKIE2 without the notification, or with altered data. internal/component/ike/engine/mobike_test.go::TestMobikeCookie2EchoBounds requires, for 8- and 64-octet cookies, that the decrypted response holds only COOKIE2 with byte-identical data (mbOnlyNotify). With no COOKIE2 it requires an empty response. internal/component/ike/engine/mobike_test.go::TestMobikeOwnerDispatchRepliesFromArrivalSocket asserts bytes.Equal(echo, cookie) through the owner handoff, and asserts that a replay returns the identical cached response.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMobikeCookie2EchoBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L235) | unit/verify | unproven |
| positive | [`TestMobikeCookie2EchoBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L234) | unit/verify | unproven |
| positive | [`TestMobikeOwnerDispatchRepliesFromArrivalSocket`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L29) | unit/verify | unproven |

### [`RFC4555-3.8-1`](#rfc4555-3.8-1)

When MOBIKE is in use, the dynamic updates (specified in [IKEv2], Section 2.23), where the peer address and port are updated from the last valid authenticated packet, work in a slightly different fashion.  The host not behind a NAT MUST NOT use these dynamic updates for IKEv2 packets (§3.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a host not behind a NAT moving its IKE peer address to the source of an authenticated INFORMATIONAL that carries no UPDATE_SA_ADDRESSES. internal/component/ike/engine/mobike_test.go::TestMobikePathProbeRetainsTunnelEndpoint sends such a probe from a new address and requires the next self-initiated DPD to arrive at the old endpoint (mbReceive(t, oldTr)) and no Child migration (dp.count == 0). The negative tag asserts the response still reaches the probing path. That is a conformance check rather than a violating input, and the tag prose names it as such.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMobikePathProbeRetainsTunnelEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L738) | unit/verify | unproven |
| positive | [`TestMobikePathProbeRetainsTunnelEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L736) | unit/verify | unproven |

### [`RFC4555-3.8-2`](#rfc4555-3.8-2)

the NAT_DETECTION_SOURCE_IP and NAT_DETECTION_DESTINATION_IP notifications MAY be included in any INFORMATIONAL request; if the request includes them, the responder MUST also include them in the response (§3.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a response that omits NAT_DETECTION_SOURCE_IP or NAT_DETECTION_DESTINATION_IP when the request carried them. internal/component/ike/engine/mobike_test.go::TestMobikePathProbeRetainsTunnelEndpoint (natDetection=true) requires exactly the two notifications, with hashes over the observed response tuple, so an omitted or reversed hash goes red. With natDetection=false it requires an empty response, so no hashes are sent unsolicited.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMobikePathProbeRetainsTunnelEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L742) | unit/verify | unproven |
| positive | [`TestMobikePathProbeRetainsTunnelEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L740) | unit/verify | unproven |

### [`RFC4555-3.6-1`](#rfc4555-3.6-1)

If the request to update the addresses is retransmitted using several different source addresses, a new INFORMATIONAL request MUST be sent (Additional Addresses, Section 3.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMobikeAdvertisementAfterSourceChange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L548) | unit/verify | unproven |
| positive | [`TestMobikeAdvertisementAfterSourceChange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L546) | unit/verify | unproven |

### [`RFC4555-3.7-4`](#rfc4555-3.7-4)

When processing the response, the original sender MUST verify that the value is the same one as sent. (§3.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting a COOKIE2 response whose value differs from, or omits, the one sent. internal/component/ike/engine/mobike_test.go::TestMobikeCookie2ResponseControlsMigration drives the real startMobikeRequest producer. In the different and missing modes it requires out.reestablish, and it fails if f.dp.count != 0 (migration). In the match mode it requires migration (mbCheckMigration). internal/component/ike/engine/mobike_test.go::TestMobikeChangedPathStillChecksCookie2 repeats the mismatch after a path change.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMobikeChangedPathStillChecksCookie2`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L998) | unit/verify | unproven |
| negative | [`TestMobikeCookie2ResponseControlsMigration`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L277) | unit/verify | unproven |
| positive | [`TestMobikeCookie2ResponseControlsMigration`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L276) | unit/verify | unproven |

### [`RFC4555-3.7-5`](#rfc4555-3.7-5)

If the values do not match, the IKE_SA MUST be closed. (§3.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: keeping the IKE_SA after a COOKIE2 mismatch. internal/component/ike/engine/mobike_test.go::TestMobikeCookie2ResponseControlsMigration, in the different and missing modes, requires f.local.State == StateDead and out.reestablish, and requires cleanup to remove both Child SPIs. In the match mode it requires no reestablish and a next request on the live SA. internal/component/ike/engine/mobike_test.go::TestMobikeChangedPathStillChecksCookie2 also requires teardown after a path change.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMobikeCookie2ResponseControlsMigration`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L280) | unit/verify | unproven |
| positive | [`TestMobikeChangedPathStillChecksCookie2`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1000) | unit/verify | unproven |
| positive | [`TestMobikeCookie2ResponseControlsMigration`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L278) | unit/verify | unproven |

### [`RFC4555-3.9-3`](#rfc4555-3.9-3)

The exchange responder MUST verify that the contents of the NO_NATS_ALLOWED notification match the addresses in the IP header (NAT Prohibition, Section 3.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMobikeNoNATsIPv4Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L394) | unit/verify | unproven |
| negative | [`TestMobikeNoNATsIPv6Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L508) | unit/verify | unproven |
| positive | [`TestMobikeNoNATsIPv4Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L393) | unit/verify | unproven |
| positive | [`TestMobikeNoNATsIPv6Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L507) | unit/verify | unproven |
| positive | [`TestMobikeNoNATsUpdateWaitsForRoutability`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L805) | unit/verify | unproven |

### [`RFC4555-3.9-4`](#rfc4555-3.9-4)

If an UNEXPECTED_NAT_DETECTED notification is sent, the exchange responder MUST NOT use the contents of the NO_NATS_ALLOWED notification for any other purpose than possibly logging the information for troubleshooting purposes (NAT Prohibition, Section 3.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMobikeNoNATsIPv4Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L398) | unit/verify | unproven |
| negative | [`TestMobikeNoNATsUpdateWaitsForRoutability`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L807) | unit/verify | unproven |
| positive | [`TestMobikeNoNATsIPv4Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L396) | unit/verify | unproven |
| positive | [`TestMobikeNoNATsReplyUsesReceivedDestination`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L468) | unit/verify | unproven |

### [`RFC4555-4.2.1-1`](#rfc4555-4.2.1-1)

The notification data field MUST be left empty (zero-length) when sending, and its contents (if any) MUST be ignored when this notification is received. (Payload Formats, Section 4.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: sending MOBIKE_SUPPORTED with non-empty data, or acting on (for example rejecting) received data. mbAuthOfferEmpty in internal/component/ike/engine/mobike_test.go::TestMobikeAuthNegotiation requires zero-length data, protocol 0 and no SPI on both generated IKE_AUTH offers. The request-extension-data and response-extension-data cases inject non-empty data in each direction and require that negotiation still enables COOKIE2 echo (mbOnlyNotify), which goes red if the data were not ignored.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMobikeAuthNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L649) | unit/verify | unproven |
| positive | [`TestMobikeAuthNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L647) | unit/verify | unproven |

### [`RFC4555-4.2.5-1`](#rfc4555-4.2.5-1)

The data associated with this notification MUST be between 8 and 64 octets in length (inclusive), and MUST be chosen by the exchange initiator in a way that is unpredictable to the exchange responder. (§4.2.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Length: mbCookieLength in TestMobikeCookie2ResponseControlsMigration fails on a generated cookie outside 8..64, TestMobikeCookie2EchoBounds rejects received 7- and 65-octet cookies. Unpredictable: TestRFC4555Cookie2IsTheRandomSourceOutput replaces crypto/rand.Reader per startMobikeRequest call with two differently seeded streams and asserts each real MOBIKE request's COOKIE2 equals the first 32 octets of its own stream, so a counter, timestamp, SA-derived value or fixed-seed PRNG goes red. The record is a whole-producer panic (reach); discrimination read from the equality assertion.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMobikeCookie2EchoBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L233) | unit/verify | revert, verified |
| positive | [`TestMobikeAdvertisementAfterSourceChange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L544) | unit/verify | revert, verified |
| positive | [`TestMobikeCookie2EchoBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L232) | unit/verify | revert, verified |
| positive | [`TestRFC4555Cookie2IsTheRandomSourceOutput`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4555_cookie2_test.go#L50) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc4555.txt |
| Source fingerprint | 67a9bc6952c1ff68 |
| Record | rfc/extraction/rfc4555.json |
| Mapped sentences | 13 |
| Declined as scope | 1 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `1.3` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `2.3` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 1 | walked | not stated |
| `3.3` | not stated | 1 | walked | not stated |
| `3.4` | not stated | 0 | walked | not stated |
| `3.5` | not stated | 0 | walked | not stated |
| `3.6` | not stated | 1 | walked | not stated |
| `3.7` | not stated | 3 | walked | not stated |
| `3.8` | not stated | 2 | walked | not stated |
| `3.9` | not stated | 3 | walked | not stated |
| `3.10` | not stated | 0 | walked | not stated |
| `3.11` | not stated | 0 | walked | not stated |
| `3.12` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 0 | walked | not stated |
| `4.1.1` | not stated | 0 | walked | not stated |
| `4.1.2` | not stated | 0 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `4.2.1` | not stated | 1 | walked | not stated |
| `4.2.2` | not stated | 0 | walked | not stated |
| `4.2.3` | not stated | 0 | walked | not stated |
| `4.2.4` | not stated | 0 | walked | not stated |
| `4.2.5` | not stated | 2 | walked | not stated |
| `4.2.6` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
| `5.2` | not stated | 0 | walked | not stated |
| `5.3` | not stated | 0 | walked | not stated |
| `5.4` | not stated | 0 | walked | not stated |
| `5.5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |
| `8.2` | not stated | 0 | walked | not stated |
| `A` | not stated | 0 | walked | not stated |
| `A.1` | not stated | 0 | walked | not stated |
| `A.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `4.2.5:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the payload-format restatement of the section 3.7 obligation already mapped at site 3.7:1: the responder copies COOKIE2 into the response | If the INFORMATIONAL request includes COOKIE2, the exchange responder MUST copy the notification to the response message. |

## Superseded

No document obsoletes RFC 4555, so its obligations are stated where they were written.
