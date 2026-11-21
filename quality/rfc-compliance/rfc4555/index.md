# RFC 4555 - IKEv2 Mobility and Multihoming Protocol (MOBIKE)

MOBIKE. Every requirement listed for RFC 4555, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 14 | of 14 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 14 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 14 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 14 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 14 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 34 | 34 of 44 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 14 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 14 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 14 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 14 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 14 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 14 | of 26 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 14 mandatory requirements: they add up to 14. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | MOBIKE |
| Automatic checks | Included |
| Requirements | 26 |
| Mandatory requirements (MUST level) | 14 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 44 |
| Tests linked to requirements | 44 |
| Recorded audit verdicts | 13 |
| Fault test records | 34 |
| Summary | `rfc/short/rfc4555.md` |
| Requirement table | `rfc/requirements/rfc4555.md` |
| RFC text | `rfc/full/rfc4555.txt` |

## Automatic checks

Included in automatic checks: Ze implements MOBIKE in its native IKE engine. All thirteen MUST-level requirements remain enrolled and have production paths plus newly authored positive/negative tests. Execution and discrimination remain required; no exclusion or conformance claim is made.

## Published support status

**Area:** MOBIKE

**Test verification:** 14 of 14 MUST proven

**Evidence:** unit 14

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 14 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **14** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (14):** [`RFC4555-x-1`](#rfc4555-x-1), [`RFC4555-x-2`](#rfc4555-x-2), [`RFC4555-3.9-1`](#rfc4555-3.9-1), [`RFC4555-3.7-1`](#rfc4555-3.7-1), [`RFC4555-3.8-1`](#rfc4555-3.8-1), [`RFC4555-3.8-2`](#rfc4555-3.8-2), [`RFC4555-3.6-1`](#rfc4555-3.6-1), [`RFC4555-3.7-4`](#rfc4555-3.7-4), [`RFC4555-3.7-5`](#rfc4555-3.7-5), [`RFC4555-3.9-3`](#rfc4555-3.9-3), [`RFC4555-3.9-4`](#rfc4555-3.9-4), [`RFC4555-4.2.6-1`](#rfc4555-4.2.6-1), [`RFC4555-4.2.1-1`](#rfc4555-4.2.1-1), [`RFC4555-4.2.5-1`](#rfc4555-4.2.5-1)

## Figures

- **MUST bar**: Ze verified 14
- **Overview bar**: Ze MUST verified 14, SHOULD verified 2, Not assessed 10

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 14 |
| MUST rows kept (the MUST bar) | 14 |
| Ze verified | 14 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 12 |
| SHOULD verified | 2 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 10 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc4555.json](../../../data/rfc/rfc4555.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4555-x-1` | Implementations that wish to use MOBIKE for a particular IKE_SA MUST include a MOBIKE_SUPPORTED notification in the IKE_AUTH exchange (in case of multiple IKE_AUTH exchanges, in the message containing the SA payload). (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestMobikeAuthNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L651). **positive:** `unit/verify` [`TestRFC4555EAPOfferRidesTheIKEAuthMessageCarryingSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4555_mobike_offer_test.go#L91). **negative:** `unit/verify` [`TestMobikeAuthNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L652). **negative:** `unit/verify` [`TestRFC4555NoOwnOfferNoMOBIKE`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4555_mobike_offer_test.go#L132) |
| `RFC4555-x-2` | The addresses are taken from the IKE_AUTH request because IKEv2 requires changing from port 500 to 4500 if a NAT is discovered.  To simplify things, implementations that support both this specification and NAT Traversal MUST change to port 4500 if the correspondent also supports both, even if no NAT was detected between them (this way, there is no need to change the ports later if a NAT is detected on some other path). (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestMobikeAuthNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L654). **negative:** `unit/verify` [`TestMobikeAuthWithoutNATTSocketKeepsBaseIKE`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L712) |
| `RFC4555-3.9-1` | More specifically, when NAT Traversal is not enabled, all messages that can update the addresses associated with the IKE_SA and/or IPsec SAs (the first IKE_AUTH request and all INFORMATIONAL requests that contain any of the following notifications: UPDATE_SA_ADDRESSES, ADDITIONAL_IP4_ADDRESS, ADDITIONAL_IP6_ADDRESS, NO_ADDITIONAL_ADDRESSES) MUST also include a NO_NATS_ALLOWED notification. (NAT Prohibition, Section 3.9) | MUST | 3.9 | **positive:** `unit/verify` [`TestMobikeAdvertisementAfterSourceChange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L553). **positive:** `unit/verify` [`TestMobikeAuthRetransmitKeepsProtectedTuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1102). **positive:** `unit/verify` [`TestMobikeConfiguredNATPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1036). **negative:** `unit/verify` [`TestMobikeConfiguredNATPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1038) |
| `RFC4555-3.7-1` | The sender of an INFORMATIONAL request MAY include a COOKIE2 notification, and if included, the recipient of an INFORMATIONAL request MUST copy the notification as-is to the response. (§3.7) | MUST | 3.7 | **positive:** `unit/verify` [`TestMobikeCookie2EchoBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L234). **positive:** `unit/verify` [`TestMobikeOwnerDispatchRepliesFromArrivalSocket`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L29). **negative:** `unit/verify` [`TestMobikeCookie2EchoBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L235) |
| `RFC4555-3.8-1` | When MOBIKE is in use, the dynamic updates (specified in [IKEv2], Section 2.23), where the peer address and port are updated from the last valid authenticated packet, work in a slightly different fashion.  The host not behind a NAT MUST NOT use these dynamic updates for IKEv2 packets (§3.8) | MUST | 3.8 | **positive:** `unit/verify` [`TestMobikePathProbeRetainsTunnelEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L746). **negative:** `unit/verify` [`TestMobikePathProbeRetainsTunnelEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L748) |
| `RFC4555-3.8-2` | the NAT_DETECTION_SOURCE_IP and NAT_DETECTION_DESTINATION_IP notifications MAY be included in any INFORMATIONAL request; if the request includes them, the responder MUST also include them in the response (§3.8) | MUST | 3.8 | **positive:** `unit/verify` [`TestMobikePathProbeRetainsTunnelEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L750). **negative:** `unit/verify` [`TestMobikePathProbeRetainsTunnelEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L752) |
| `RFC4555-3.6-1` | If the request to update the addresses is retransmitted using several different source addresses, a new INFORMATIONAL request MUST be sent (Additional Addresses, Section 3.6) | MUST | 3.6 | **positive:** `unit/verify` [`TestMobikeAdvertisementAfterSourceChange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L546). **negative:** `unit/verify` [`TestMobikeAdvertisementAfterSourceChange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L548) |
| `RFC4555-3.7-4` | When processing the response, the original sender MUST verify that the value is the same one as sent. (§3.7) | MUST | 3.7 | **positive:** `unit/verify` [`TestMobikeCookie2ResponseControlsMigration`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L276). **negative:** `unit/verify` [`TestMobikeChangedPathStillChecksCookie2`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1008). **negative:** `unit/verify` [`TestMobikeCookie2ResponseControlsMigration`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L277) |
| `RFC4555-3.7-5` | If the values do not match, the IKE_SA MUST be closed. (§3.7) | MUST | 3.7 | **positive:** `unit/verify` [`TestMobikeChangedPathStillChecksCookie2`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1010). **positive:** `unit/verify` [`TestMobikeCookie2ResponseControlsMigration`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L278). **negative:** `unit/verify` [`TestMobikeCookie2ResponseControlsMigration`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L280) |
| `RFC4555-3.9-3` | The exchange responder MUST verify that the contents of the NO_NATS_ALLOWED notification match the addresses in the IP header (NAT Prohibition, Section 3.9) | MUST | 3.9 | **positive:** `unit/verify` [`TestMobikeNoNATsIPv4Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L393). **positive:** `unit/verify` [`TestMobikeNoNATsIPv6Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L507). **positive:** `unit/verify` [`TestMobikeNoNATsUpdateWaitsForRoutability`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L815). **negative:** `unit/verify` [`TestMobikeNoNATsIPv4Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L394). **negative:** `unit/verify` [`TestMobikeNoNATsIPv6Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L508) |
| `RFC4555-3.9-4` | If an UNEXPECTED_NAT_DETECTED notification is sent, the exchange responder MUST NOT use the contents of the NO_NATS_ALLOWED notification for any other purpose than possibly logging the information for troubleshooting purposes (NAT Prohibition, Section 3.9) | MUST NOT | 3.9 | **positive:** `unit/verify` [`TestMobikeNoNATsIPv4Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L396). **positive:** `unit/verify` [`TestMobikeNoNATsReplyUsesReceivedDestination`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L468). **negative:** `unit/verify` [`TestMobikeNoNATsIPv4Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L398). **negative:** `unit/verify` [`TestMobikeNoNATsUpdateWaitsForRoutability`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L817) |
| `RFC4555-4.2.6-1` | The Notify Message Type for this message is 16402. The notification data contains the IP addresses and ports from/to which the packet was sent. (§4.2.6) | MUST | 4.2.6 | **positive:** `unit/verify` [`TestMobikeNoNATsDataIsTheSendTuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1822). **negative:** `unit/verify` [`TestMobikeNoNATsDataIsTheSendTuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1826) |
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
| `RFC4555-3.6-2` | This information can be updated by sending an INFORMATIONAL exchange request message that contains either one or more ADDITIONAL_IP4_ADDRESS/ADDITIONAL_IP6_ADDRESS notifications or the NO_ADDITIONAL_ADDRESSES notification. (§3.6) | MAY | 3.6 | **positive:** `unit/verify` [`TestMobikeUpdateCarriesItsAddressList`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1791). **negative:** no negative test. **{single-polarity}:** a MAY that permits the update to carry its address list, so the one observable is that Ze's INFORMATIONAL update carries NO_ADDITIONAL_ADDRESSES (internal/component/ike/engine/mobike_test.go TestMobikeUpdateCarriesItsAddressList); an update without the list is also permitted, so there is no violating input for a negative to refuse |
| `RFC4555-3.6-3` | The initiator MAY include these notifications in the same request as UPDATE_SA_ADDRESSES. (§3.6) | MAY | 3.6 | **positive:** `unit/verify` [`TestMobikeUpdateCarriesItsAddressList`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1793). **negative:** no negative test. **{single-polarity}:** a MAY that permits the address notifications to ride in the UPDATE_SA_ADDRESSES request, so the one observable is that Ze sends NO_ADDITIONAL_ADDRESSES in that same request (internal/component/ike/engine/mobike_test.go TestMobikeUpdateCarriesItsAddressList); sending them in a separate request is also permitted, so there is no violating input for a negative to refuse |
| `RFC4555-3.7-3` | In environments where the peer is expected to be well-behaved (many corporate VPNs, for instance), or the address can be verified by some other means (e.g., a certificate issued by an authority trusted for this purpose), the return routability check MAY be omitted. (§3.7) | MAY | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4555-3.8-4` | When MOBIKE is in use, the dynamic updates (specified in [IKEv2], Section 2.23), where the peer address and port are updated from the last valid authenticated packet, work in a slightly different fashion.  The host not behind a NAT MUST NOT use these dynamic updates for IKEv2 packets, but MAY use them for ESP packets. (§3.8) | MAY | 3.8 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 4555 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC4555-x-1`](#rfc4555-x-1)

Implementations that wish to use MOBIKE for a particular IKE_SA MUST include a MOBIKE_SUPPORTED notification in the IKE_AUTH exchange (in case of multiple IKE_AUTH exchanges, in the message containing the SA payload). (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (independent judge). Clause: the MOBIKE_SUPPORTED offer is in IKE_AUTH, and with several IKE_AUTH exchanges in the message carrying the SA payload. rfc4555_mobike_offer_test.go::TestRFC4555EAPOfferRidesTheIKEAuthMessageCarryingSA drives an EAP initiator: the first IKE_AUTH request (SA, no AUTH) decrypted off the wire carries MOBIKE_SUPPORTED, the later buildEAPResponse request carries neither SA nor the offer, and the peer's answer then enables MOBIKE. TestRFC4555NoOwnOfferNoMOBIKE is the negative for THIS row: an initiator whose own IKE_AUTH carried no offer (never built, or no migrating dataplane, checked off the wire) does not enable MOBIKE on the peer's offer (acceptMobikeOffer gates on mobike.offered). Revert records observed red on mobike.go::mobikeAuthOffer and ::acceptMobikeOffer. Single-exchange placement for both roles stays on TestMobikeAuthNegotiation. The responder's multi-exchange case is not driven end to end; its only offer site is buildAuthResponse (responder.go), the SA-carrying final response, after the SA payload unconditionally, the same builder TestMobikeAuthNegotiation asserts.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestMobikeAuthNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L652) | unit/verify | revert, verified |
| negative | [`TestRFC4555NoOwnOfferNoMOBIKE`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4555_mobike_offer_test.go#L132) | unit/verify | revert, verified |
| positive | [`TestMobikeAuthNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L651) | unit/verify | revert, verified |
| positive | [`TestRFC4555EAPOfferRidesTheIKEAuthMessageCarryingSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4555_mobike_offer_test.go#L91) | unit/verify | revert, verified |

### [`RFC4555-x-2`](#rfc4555-x-2)

The addresses are taken from the IKE_AUTH request because IKEv2 requires changing from port 500 to 4500 if a NAT is discovered.  To simplify things, implementations that support both this specification and NAT Traversal MUST change to port 4500 if the correspondent also supports both, even if no NAT was detected between them (this way, there is no need to change the ports later if a NAT is detected on some other path). (§3.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-11-20 by an independent agent after 1b0198fb0d changed the negative TestMobikeAuthWithoutNATTSocketKeepsBaseIKE. RFC 4555 Section 3.3: "implementations that support both this specification and NAT Traversal MUST change to port 4500 if the correspondent also supports both, even if no NAT was detected between them". The negative gained an assertion that neither the initiator nor the responder SA sits on port 4500 without NAT-T support, beside the unchanged check that no IKE_AUTH carries MOBIKE_SUPPORTED: that is the case just outside the clause, where the correct answer is no float. The positive TestMobikeAuthNegotiation is byte-identical: through mbAuthHandshake it binds NAT-T sockets only, sends to the port-500 stand-in, fails on NATDetected and requires the IKE_AUTH on the NAT-T socket with the non-ESP marker, so a missing float delivers nothing and goes red. Verdict stands.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestMobikeAuthWithoutNATTSocketKeepsBaseIKE`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L712) | unit/verify | mutant, verified |
| positive | [`TestMobikeAuthNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L654) | unit/verify | mutant, verified |

### [`RFC4555-3.9-1`](#rfc4555-3.9-1)

More specifically, when NAT Traversal is not enabled, all messages that can update the addresses associated with the IKE_SA and/or IPsec SAs (the first IKE_AUTH request and all INFORMATIONAL requests that contain any of the following notifications: UPDATE_SA_ADDRESSES, ADDITIONAL_IP4_ADDRESS, ADDITIONAL_IP6_ADDRESS, NO_ADDITIONAL_ADDRESSES) MUST also include a NO_NATS_ALLOWED notification. (NAT Prohibition, Section 3.9)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-11-20 by an independent agent after df3221e7b8 changed TestMobikeAuthRetransmitKeepsProtectedTuple. RFC 4555 Section 3.9: "when NAT Traversal is not enabled, all messages that can update the addresses associated with the IKE_SA and/or IPsec SAs (the first IKE_AUTH request and all INFORMATIONAL requests that contain any of the following notifications: UPDATE_SA_ADDRESSES, ADDITIONAL_IP4_ADDRESS, ADDITIONAL_IP6_ADDRESS, NO_ADDITIONAL_ADDRESSES) MUST also include a NO_NATS_ALLOWED notification." The change is mechanical: the PeerSession takes its NAT-T socket through ps.sockets.Store(&sessionSockets{natt: iniTr}) instead of a natt field; the assertions are unchanged, so the retransmitted first IKE_AUTH must be byte-identical and carry NO_NATS_ALLOWED with the actual tuple. TestMobikeConfiguredNATPolicy (both tags) and TestMobikeAdvertisementAfterSourceChange are unchanged: under prohibit the IKE_AUTH and the UPDATE_SA_ADDRESSES INFORMATIONAL carry the notification, under allow and the default none do. Ze sends no ADDITIONAL_IP4 or ADDITIONAL_IP6 address. Verdict stands.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestMobikeConfiguredNATPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1038) | unit/verify | mutant, verified |
| positive | [`TestMobikeAdvertisementAfterSourceChange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L553) | unit/verify | not verified |
| positive | [`TestMobikeAuthRetransmitKeepsProtectedTuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1102) | unit/verify | not verified |
| positive | [`TestMobikeConfiguredNATPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1036) | unit/verify | mutant, verified |

### [`RFC4555-3.7-1`](#rfc4555-3.7-1)

The sender of an INFORMATIONAL request MAY include a COOKIE2 notification, and if included, the recipient of an INFORMATIONAL request MUST copy the notification as-is to the response. (§3.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: answering an INFORMATIONAL request that carries COOKIE2 without the notification, or with altered data. internal/component/ike/engine/mobike_test.go::TestMobikeCookie2EchoBounds requires, for 8- and 64-octet cookies, that the decrypted response holds only COOKIE2 with byte-identical data (mbOnlyNotify). With no COOKIE2 it requires an empty response. internal/component/ike/engine/mobike_test.go::TestMobikeOwnerDispatchRepliesFromArrivalSocket asserts bytes.Equal(echo, cookie) through the owner handoff, and asserts that a replay returns the identical cached response.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestMobikeCookie2EchoBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L235) | unit/verify | mutant, verified |
| positive | [`TestMobikeCookie2EchoBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L234) | unit/verify | mutant, verified |
| positive | [`TestMobikeOwnerDispatchRepliesFromArrivalSocket`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L29) | unit/verify | not verified |

### [`RFC4555-3.8-1`](#rfc4555-3.8-1)

When MOBIKE is in use, the dynamic updates (specified in [IKEv2], Section 2.23), where the peer address and port are updated from the last valid authenticated packet, work in a slightly different fashion.  The host not behind a NAT MUST NOT use these dynamic updates for IKEv2 packets (§3.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a host not behind a NAT moving its IKE peer address to the source of an authenticated INFORMATIONAL that carries no UPDATE_SA_ADDRESSES. internal/component/ike/engine/mobike_test.go::TestMobikePathProbeRetainsTunnelEndpoint sends such a probe from a new address and requires the next self-initiated DPD to arrive at the old endpoint (mbReceive(t, oldTr)) and no Child migration (dp.count == 0). The negative tag asserts the response still reaches the probing path. That is a conformance check rather than a violating input, and the tag prose names it as such.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestMobikePathProbeRetainsTunnelEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L748) | unit/verify | mutant, verified |
| positive | [`TestMobikePathProbeRetainsTunnelEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L746) | unit/verify | mutant, verified |

### [`RFC4555-3.8-2`](#rfc4555-3.8-2)

the NAT_DETECTION_SOURCE_IP and NAT_DETECTION_DESTINATION_IP notifications MAY be included in any INFORMATIONAL request; if the request includes them, the responder MUST also include them in the response (§3.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a response that omits NAT_DETECTION_SOURCE_IP or NAT_DETECTION_DESTINATION_IP when the request carried them. internal/component/ike/engine/mobike_test.go::TestMobikePathProbeRetainsTunnelEndpoint (natDetection=true) requires exactly the two notifications, with hashes over the observed response tuple, so an omitted or reversed hash goes red. With natDetection=false it requires an empty response, so no hashes are sent unsolicited.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestMobikePathProbeRetainsTunnelEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L752) | unit/verify | mutant, verified |
| positive | [`TestMobikePathProbeRetainsTunnelEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L750) | unit/verify | mutant, verified |

### [`RFC4555-3.6-1`](#rfc4555-3.6-1)

If the request to update the addresses is retransmitted using several different source addresses, a new INFORMATIONAL request MUST be sent (Additional Addresses, Section 3.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestMobikeAdvertisementAfterSourceChange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L548) | unit/verify | mutant, verified |
| positive | [`TestMobikeAdvertisementAfterSourceChange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L546) | unit/verify | mutant, verified |

### [`RFC4555-3.7-4`](#rfc4555-3.7-4)

When processing the response, the original sender MUST verify that the value is the same one as sent. (§3.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting a COOKIE2 response whose value differs from, or omits, the one sent. internal/component/ike/engine/mobike_test.go::TestMobikeCookie2ResponseControlsMigration drives the real startMobikeRequest producer. In the different and missing modes it requires out.reestablish, and it fails if f.dp.count != 0 (migration). In the match mode it requires migration (mbCheckMigration). internal/component/ike/engine/mobike_test.go::TestMobikeChangedPathStillChecksCookie2 repeats the mismatch after a path change.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestMobikeChangedPathStillChecksCookie2`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1008) | unit/verify | not verified |
| negative | [`TestMobikeCookie2ResponseControlsMigration`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L277) | unit/verify | mutant, verified |
| positive | [`TestMobikeCookie2ResponseControlsMigration`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L276) | unit/verify | mutant, verified |

### [`RFC4555-3.7-5`](#rfc4555-3.7-5)

If the values do not match, the IKE_SA MUST be closed. (§3.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: keeping the IKE_SA after a COOKIE2 mismatch. internal/component/ike/engine/mobike_test.go::TestMobikeCookie2ResponseControlsMigration, in the different and missing modes, requires f.local.State == StateDead and out.reestablish, and requires cleanup to remove both Child SPIs. In the match mode it requires no reestablish and a next request on the live SA. internal/component/ike/engine/mobike_test.go::TestMobikeChangedPathStillChecksCookie2 also requires teardown after a path change.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestMobikeCookie2ResponseControlsMigration`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L280) | unit/verify | mutant, verified |
| positive | [`TestMobikeChangedPathStillChecksCookie2`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1010) | unit/verify | not verified |
| positive | [`TestMobikeCookie2ResponseControlsMigration`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L278) | unit/verify | mutant, verified |

### [`RFC4555-3.9-3`](#rfc4555-3.9-3)

The exchange responder MUST verify that the contents of the NO_NATS_ALLOWED notification match the addresses in the IP header (NAT Prohibition, Section 3.9)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestMobikeNoNATsIPv4Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L394) | unit/verify | mutant, verified |
| negative | [`TestMobikeNoNATsIPv6Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L508) | unit/verify | not verified |
| positive | [`TestMobikeNoNATsIPv4Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L393) | unit/verify | mutant, verified |
| positive | [`TestMobikeNoNATsIPv6Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L507) | unit/verify | not verified |
| positive | [`TestMobikeNoNATsUpdateWaitsForRoutability`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L815) | unit/verify | not verified |

### [`RFC4555-3.9-4`](#rfc4555-3.9-4)

If an UNEXPECTED_NAT_DETECTED notification is sent, the exchange responder MUST NOT use the contents of the NO_NATS_ALLOWED notification for any other purpose than possibly logging the information for troubleshooting purposes (NAT Prohibition, Section 3.9)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestMobikeNoNATsIPv4Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L398) | unit/verify | mutant, verified |
| negative | [`TestMobikeNoNATsUpdateWaitsForRoutability`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L817) | unit/verify | not verified |
| positive | [`TestMobikeNoNATsIPv4Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L396) | unit/verify | mutant, verified |
| positive | [`TestMobikeNoNATsReplyUsesReceivedDestination`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L468) | unit/verify | not verified |

### [`RFC4555-4.2.6-1`](#rfc4555-4.2.6-1)

The Notify Message Type for this message is 16402. The notification data contains the IP addresses and ports from/to which the packet was sent. (§4.2.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-11 by an independent judge after the fix round (2691e815b1). RFC 4555 4.2.6: "The Notify Message Type for this message is 16402.  The notification data contains the IP addresses and ports from/to which the packet was sent." Type clause, now enforced: TestMobikeNoNATsDataIsTheSendTuple decrypts the NO_NATS_ALLOWED notify off a real update and requires NotifyMsgType == 16402 as a literal, so a wrong shared wire.NotifyNoNATsAllowed constant is red (mbRequireNotify finds the notify by the constant, then the literal disagrees) and a producer writing another type is red at mbRequireNotify. Data clause: the 12-octet IPv4 data equals the received packet's source address, destination address, source port and destination port field by field, and noNATsAllowedMatches accepts that tuple. Negative: noNATsAllowedMatches refuses a changed source address, a changed destination address, swapped ports, a changed source port, and now the packet's own addresses and ports written at the 16-octet width; because net.IP.Equal treats the IPv4-mapped form as equal, the length test is the only thing that refuses that last case, so it isolates the width. Revert records on mobike.go::noNATsAllowedPayload (positive) and noNATsAllowedMatches (negative). Both defects of the earlier weak verdict are resolved.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestMobikeNoNATsDataIsTheSendTuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1826) | unit/verify | revert, verified |
| positive | [`TestMobikeNoNATsDataIsTheSendTuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1822) | unit/verify | revert, verified |

### [`RFC4555-4.2.1-1`](#rfc4555-4.2.1-1)

The notification data field MUST be left empty (zero-length) when sending, and its contents (if any) MUST be ignored when this notification is received. (Payload Formats, Section 4.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: sending MOBIKE_SUPPORTED with non-empty data, or acting on (for example rejecting) received data. mbAuthOfferEmpty in internal/component/ike/engine/mobike_test.go::TestMobikeAuthNegotiation requires zero-length data, protocol 0 and no SPI on both generated IKE_AUTH offers. The request-extension-data and response-extension-data cases inject non-empty data in each direction and require that negotiation still enables COOKIE2 echo (mbOnlyNotify), which goes red if the data were not ignored.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestMobikeAuthNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L649) | unit/verify | mutant, verified |
| positive | [`TestMobikeAuthNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L647) | unit/verify | mutant, verified |

### [`RFC4555-4.2.5-1`](#rfc4555-4.2.5-1)

The data associated with this notification MUST be between 8 and 64 octets in length (inclusive), and MUST be chosen by the exchange initiator in a way that is unpredictable to the exchange responder. (§4.2.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Length: mbCookieLength in TestMobikeCookie2ResponseControlsMigration fails on a generated cookie outside 8..64, TestMobikeCookie2EchoBounds rejects received 7- and 65-octet cookies. Unpredictable: TestRFC4555Cookie2IsTheRandomSourceOutput replaces crypto/rand.Reader per startMobikeRequest call with two differently seeded streams and asserts each real MOBIKE request's COOKIE2 equals the first 32 octets of its own stream, so a counter, timestamp, SA-derived value or fixed-seed PRNG goes red. The record is a whole-producer panic (reach); discrimination read from the equality assertion.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestMobikeCookie2EchoBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L233) | unit/verify | revert, verified |
| positive | [`TestMobikeAdvertisementAfterSourceChange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L544) | unit/verify | revert, verified |
| positive | [`TestMobikeCookie2EchoBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L232) | unit/verify | revert, verified |
| positive | [`TestRFC4555Cookie2IsTheRandomSourceOutput`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4555_cookie2_test.go#L50) | unit/verify | revert, verified |

### [`RFC4555-3.6-2`](#rfc4555-3.6-2)

This information can be updated by sending an INFORMATIONAL exchange request message that contains either one or more ADDITIONAL_IP4_ADDRESS/ADDITIONAL_IP6_ADDRESS notifications or the NO_ADDITIONAL_ADDRESSES notification. (§3.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-10-11 by an independent judge. RFC 4555 3.6: "This information can be updated by sending an INFORMATIONAL exchange request message that contains either one or more ADDITIONAL_IP4_ADDRESS/ADDITIONAL_IP6_ADDRESS notifications or the NO_ADDITIONAL_ADDRESSES notification." The sentence permits a way to update the list and obliges nothing when no update is sent, so the {single-polarity: positive} annotation holds: no input is a violation for a negative to refuse. TestMobikeUpdateCarriesItsAddressList drives a real IKE_AUTH handshake, starts an update with startMobikeRequest, and requires the datagram the responder reads to be an INFORMATIONAL request (exchange type, R bit clear) whose decrypted chain carries NO_ADDITIONAL_ADDRESSES. Revert record on mobike.go::startMobikeRequest. Residual, not a finding: the 'either ... or' is not asserted as exclusive (no check that no ADDITIONAL_IP4/6_ADDRESS rides beside NO_ADDITIONAL_ADDRESSES); the producer (mobike.go) writes only NO_ADDITIONAL_ADDRESSES today.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestMobikeUpdateCarriesItsAddressList`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1791) | unit/verify | revert, verified |

### [`RFC4555-3.6-3`](#rfc4555-3.6-3)

The initiator MAY include these notifications in the same request as UPDATE_SA_ADDRESSES. (§3.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-10-11 by an independent judge. RFC 4555 3.6: "The initiator MAY include these notifications in the same request as UPDATE_SA_ADDRESSES." A MAY whose alternative (a separate request) is also permitted, so the {single-polarity: positive} annotation holds. TestMobikeUpdateCarriesItsAddressList requires both UPDATE_SA_ADDRESSES and NO_ADDITIONAL_ADDRESSES in the decrypted chain of the one INFORMATIONAL request startMobikeRequest sends, so dropping the address notification from that request turns it red. Revert record on mobike.go::startMobikeRequest.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestMobikeUpdateCarriesItsAddressList`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1793) | unit/verify | revert, verified |

## Requirement list review

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
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `1.2` | not stated | 0 | reviewed | not stated |
| `1.3` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `2.1` | not stated | 0 | reviewed | not stated |
| `2.2` | not stated | 0 | reviewed | not stated |
| `2.3` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `3.1` | not stated | 0 | reviewed | not stated |
| `3.2` | not stated | 1 | reviewed | not stated |
| `3.3` | not stated | 1 | reviewed | not stated |
| `3.4` | not stated | 0 | reviewed | not stated |
| `3.5` | not stated | 0 | reviewed | not stated |
| `3.6` | not stated | 1 | reviewed | not stated |
| `3.7` | not stated | 3 | reviewed | not stated |
| `3.8` | not stated | 2 | reviewed | not stated |
| `3.9` | not stated | 3 | reviewed | not stated |
| `3.10` | not stated | 0 | reviewed | not stated |
| `3.11` | not stated | 0 | reviewed | not stated |
| `3.12` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `4.1` | not stated | 0 | reviewed | not stated |
| `4.1.1` | not stated | 0 | reviewed | not stated |
| `4.1.2` | not stated | 0 | reviewed | not stated |
| `4.2` | not stated | 0 | reviewed | not stated |
| `4.2.1` | not stated | 1 | reviewed | not stated |
| `4.2.2` | not stated | 0 | reviewed | not stated |
| `4.2.3` | not stated | 0 | reviewed | not stated |
| `4.2.4` | not stated | 0 | reviewed | not stated |
| `4.2.5` | not stated | 2 | reviewed | not stated |
| `4.2.6` | not stated | 0 | reviewed | Row RFC4555-4.2.6-1 (added 2026-10-11) is read from the indicative sentence "The notification data contains the IP addresses and ports from/to which the packet was sent.", which carries no keyword; the section 3.9 responder check RFC4555-3.9-3 compares exactly these fields. |
| `5` | not stated | 0 | reviewed | not stated |
| `5.1` | not stated | 0 | reviewed | not stated |
| `5.2` | not stated | 0 | reviewed | not stated |
| `5.3` | not stated | 0 | reviewed | not stated |
| `5.4` | not stated | 0 | reviewed | not stated |
| `5.5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `8.1` | not stated | 0 | reviewed | not stated |
| `8.2` | not stated | 0 | reviewed | not stated |
| `A` | not stated | 0 | reviewed | not stated |
| `A.1` | not stated | 0 | reviewed | not stated |
| `A.2` | not stated | 0 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `4.2.5:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the payload-format restatement of the section 3.7 obligation already mapped at site 3.7:1: the responder copies COOKIE2 into the response | If the INFORMATIONAL request includes COOKIE2, the exchange responder MUST copy the notification to the response message. |

## Superseded

No document obsoletes RFC 4555, so its obligations are stated where they were written.
