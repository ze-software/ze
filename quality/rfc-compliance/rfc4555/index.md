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
| No test at all | 0.0% | 0 of 13 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 37 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 13 | of 23 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 13 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 13 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 13 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 13 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

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
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 37 |
| Tagged units | 37 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
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
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **13** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (13):** [`RFC4555-x-1`](#rfc4555-x-1), [`RFC4555-x-2`](#rfc4555-x-2), [`RFC4555-3.9-1`](#rfc4555-3.9-1), [`RFC4555-3.7-1`](#rfc4555-3.7-1), [`RFC4555-3.8-1`](#rfc4555-3.8-1), [`RFC4555-3.8-2`](#rfc4555-3.8-2), [`RFC4555-3.6-1`](#rfc4555-3.6-1), [`RFC4555-3.7-4`](#rfc4555-3.7-4), [`RFC4555-3.7-5`](#rfc4555-3.7-5), [`RFC4555-3.9-3`](#rfc4555-3.9-3), [`RFC4555-3.9-4`](#rfc4555-3.9-4), [`RFC4555-4.2.1-1`](#rfc4555-4.2.1-1), [`RFC4555-4.2.5-1`](#rfc4555-4.2.5-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4555-x-1` | Both peers MUST include N(MOBIKE_SUPPORTED) in IKE_AUTH to enable MOBIKE for that IKE SA (Capability Negotiation, Sections 3.1-3.2) | MUST | x | **positive:** `unit/verify` [`TestMobikeAuthNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L651). **negative:** `unit/verify` [`TestMobikeAuthNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L652) |
| `RFC4555-x-2` | Implementations supporting both MOBIKE and NAT Traversal MUST switch to port 4500 during IKE_AUTH even if no NAT is detected (Capability Negotiation, Sections 3.1-3.2) | MUST | x | **positive:** `unit/verify` [`TestMobikeAuthNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L654). **negative:** `unit/verify` [`TestMobikeAuthWithoutNATTSocketKeepsBaseIKE`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L712) |
| `RFC4555-3.9-1` | More specifically, when NAT Traversal is not enabled, all messages that can update the addresses associated with the IKE_SA and/or IPsec SAs (the first IKE_AUTH request and all INFORMATIONAL requests that contain any of the following notifications: UPDATE_SA_ADDRESSES, ADDITIONAL_IP4_ADDRESS, ADDITIONAL_IP6_ADDRESS, NO_ADDITIONAL_ADDRESSES) MUST also include a NO_NATS_ALLOWED notification. (NAT Prohibition, Section 3.9) | MUST | 3.9 | **positive:** `unit/verify` [`TestMobikeAdvertisementAfterSourceChange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L553). **positive:** `unit/verify` [`TestMobikeAuthRetransmitKeepsProtectedTuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1092). **positive:** `unit/verify` [`TestMobikeConfiguredNATPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1026). **negative:** `unit/verify` [`TestMobikeConfiguredNATPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1028) |
| `RFC4555-3.7-1` | The exchange responder MUST copy COOKIE2 verbatim into the response (Return Routability Check, Section 3.7) | MUST | 3.7 | **positive:** `unit/verify` [`TestMobikeCookie2EchoBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L234). **positive:** `unit/verify` [`TestMobikeOwnerDispatchRepliesFromArrivalSocket`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L29). **negative:** `unit/verify` [`TestMobikeCookie2EchoBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L235) |
| `RFC4555-3.8-1` | When MOBIKE is active, the host not behind a NAT MUST NOT use dynamic IKEv2 address updates for IKE packets (NAT Mapping Changes, Section 3.8) | MUST | 3.8 | **positive:** `unit/verify` [`TestMobikePathProbeRetainsTunnelEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L736). **negative:** `unit/verify` [`TestMobikePathProbeRetainsTunnelEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L738) |
| `RFC4555-3.8-2` | The responder MUST echo NAT_DETECTION_SOURCE_IP and NAT_DETECTION_DESTINATION_IP if present in any INFORMATIONAL request (NAT Mapping Changes, Section 3.8) | MUST | 3.8 | **positive:** `unit/verify` [`TestMobikePathProbeRetainsTunnelEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L740). **negative:** `unit/verify` [`TestMobikePathProbeRetainsTunnelEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L742) |
| `RFC4555-3.6-1` | If the request to update the addresses is retransmitted using several different source addresses, a new INFORMATIONAL request MUST be sent (Additional Addresses, Section 3.6) | MUST | 3.6 | **positive:** `unit/verify` [`TestMobikeAdvertisementAfterSourceChange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L546). **negative:** `unit/verify` [`TestMobikeAdvertisementAfterSourceChange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L548) |
| `RFC4555-3.7-4` | When processing the response, the original sender of COOKIE2 MUST verify that the value is the same one as sent (Return Routability Check, Section 3.7) | MUST | 3.7 | **positive:** `unit/verify` [`TestMobikeCookie2ResponseControlsMigration`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L276). **negative:** `unit/verify` [`TestMobikeChangedPathStillChecksCookie2`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L998). **negative:** `unit/verify` [`TestMobikeCookie2ResponseControlsMigration`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L277) |
| `RFC4555-3.7-5` | If the COOKIE2 values do not match, the IKE_SA MUST be closed (Return Routability Check, Section 3.7) | MUST | 3.7 | **positive:** `unit/verify` [`TestMobikeChangedPathStillChecksCookie2`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1000). **positive:** `unit/verify` [`TestMobikeCookie2ResponseControlsMigration`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L278). **negative:** `unit/verify` [`TestMobikeCookie2ResponseControlsMigration`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L280) |
| `RFC4555-3.9-3` | The exchange responder MUST verify that the contents of the NO_NATS_ALLOWED notification match the addresses in the IP header (NAT Prohibition, Section 3.9) | MUST | 3.9 | **positive:** `unit/verify` [`TestMobikeNoNATsIPv4Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L393). **positive:** `unit/verify` [`TestMobikeNoNATsIPv6Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L507). **positive:** `unit/verify` [`TestMobikeNoNATsUpdateWaitsForRoutability`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L805). **negative:** `unit/verify` [`TestMobikeNoNATsIPv4Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L394). **negative:** `unit/verify` [`TestMobikeNoNATsIPv6Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L508) |
| `RFC4555-3.9-4` | If an UNEXPECTED_NAT_DETECTED notification is sent, the exchange responder MUST NOT use the contents of the NO_NATS_ALLOWED notification for any other purpose than possibly logging the information for troubleshooting purposes (NAT Prohibition, Section 3.9) | MUST NOT | 3.9 | **positive:** `unit/verify` [`TestMobikeNoNATsIPv4Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L396). **positive:** `unit/verify` [`TestMobikeNoNATsReplyUsesReceivedDestination`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L468). **negative:** `unit/verify` [`TestMobikeNoNATsIPv4Tuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L398). **negative:** `unit/verify` [`TestMobikeNoNATsUpdateWaitsForRoutability`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L807) |
| `RFC4555-4.2.1-1` | The notification data field MUST be left empty (zero-length) when sending, and its contents (if any) MUST be ignored when this notification is received. (Payload Formats, Section 4.2.1) | MUST | 4.2.1 | **positive:** `unit/verify` [`TestMobikeAuthNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L647). **negative:** `unit/verify` [`TestMobikeAuthNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L649) |
| `RFC4555-4.2.5-1` | The data associated with the COOKIE2 notification MUST be between 8 and 64 octets in length (inclusive), and MUST be chosen by the exchange initiator in a way that is unpredictable to the exchange responder (Payload Formats, Section 4.2.5) | MUST | 4.2.5 | **positive:** `unit/verify` [`TestMobikeAdvertisementAfterSourceChange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L544). **positive:** `unit/verify` [`TestMobikeCookie2EchoBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L232). **negative:** `unit/verify` [`TestMobikeCookie2EchoBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L233) |
| `RFC4555-3.7-2` | Return routability check SHOULD be performed by default (Return Routability Check, Section 3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4555-3.8-3` | The initiator behind a NAT SHOULD include NAT detection payloads in DPD messages and compare with previous values (NAT Mapping Changes, Section 3.8) | SHOULD | 3.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC4555-3.9-2` | The initiator SHOULD retry several times on UNEXPECTED_NAT_DETECTED (NAT Prohibition, Section 3.9) | SHOULD | 3.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC4555-3.11-1` | The responder SHOULD use long timeout intervals (at least 5 minutes for retransmission) to give the initiator time to detect problems and switch paths (Failure Recovery, Section 3.11) | SHOULD | 3.11 | **positive:** no positive test. **negative:** no negative test |
| `RFC4555-3.12-1` | MOBIKE nodes SHOULD verify that incoming IPsec packets use expected addresses (Dead Peer Detection, Section 3.12) | SHOULD | 3.12 | **positive:** no positive test. **negative:** no negative test |
| `RFC4555-3.12-2` | Packets from stale addresses SHOULD NOT count as evidence that the peer is alive and synchronized (Dead Peer Detection, Section 3.12) | SHOULD NOT | 3.12 | **positive:** no positive test. **negative:** no negative test |
| `RFC4555-x-3` | SPD cache links to SAD entries should NOT use (remote IP, remote SPI) as the key (Implementation Considerations, Appendix A) | SHOULD NOT | x | **positive:** no positive test. **negative:** no negative test |
| `RFC4555-x-4` | Both peers MAY advertise extra addresses in IKE_AUTH and later in INFORMATIONAL requests (Additional Addresses, Sections 3.4, 3.6) | MAY | x | **positive:** no positive test. **negative:** no negative test |
| `RFC4555-3.7-3` | Return routability check MAY be omitted in trusted environments (Return Routability Check, Section 3.7) | MAY | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4555-3.8-4` | The host not behind a NAT MAY use dynamic IKEv2 address updates for ESP packets when MOBIKE is active (NAT Mapping Changes, Section 3.8) | MAY | 3.8 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 4555 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4555-x-1`](#rfc4555-x-1)

Both peers MUST include N(MOBIKE_SUPPORTED) in IKE_AUTH to enable MOBIKE for that IKE SA (Capability Negotiation, Sections 3.1-3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMobikeAuthNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L652) | unit/verify | unproven |
| positive | [`TestMobikeAuthNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L651) | unit/verify | unproven |

### [`RFC4555-x-2`](#rfc4555-x-2)

Implementations supporting both MOBIKE and NAT Traversal MUST switch to port 4500 during IKE_AUTH even if no NAT is detected (Capability Negotiation, Sections 3.1-3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMobikeAuthWithoutNATTSocketKeepsBaseIKE`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L712) | unit/verify | unproven |
| positive | [`TestMobikeAuthNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L654) | unit/verify | unproven |

### [`RFC4555-3.9-1`](#rfc4555-3.9-1)

More specifically, when NAT Traversal is not enabled, all messages that can update the addresses associated with the IKE_SA and/or IPsec SAs (the first IKE_AUTH request and all INFORMATIONAL requests that contain any of the following notifications: UPDATE_SA_ADDRESSES, ADDITIONAL_IP4_ADDRESS, ADDITIONAL_IP6_ADDRESS, NO_ADDITIONAL_ADDRESSES) MUST also include a NO_NATS_ALLOWED notification. (NAT Prohibition, Section 3.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMobikeConfiguredNATPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1028) | unit/verify | unproven |
| positive | [`TestMobikeAdvertisementAfterSourceChange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L553) | unit/verify | unproven |
| positive | [`TestMobikeAuthRetransmitKeepsProtectedTuple`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1092) | unit/verify | unproven |
| positive | [`TestMobikeConfiguredNATPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L1026) | unit/verify | unproven |

### [`RFC4555-3.7-1`](#rfc4555-3.7-1)

The exchange responder MUST copy COOKIE2 verbatim into the response (Return Routability Check, Section 3.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMobikeCookie2EchoBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L235) | unit/verify | unproven |
| positive | [`TestMobikeCookie2EchoBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L234) | unit/verify | unproven |
| positive | [`TestMobikeOwnerDispatchRepliesFromArrivalSocket`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L29) | unit/verify | unproven |

### [`RFC4555-3.8-1`](#rfc4555-3.8-1)

When MOBIKE is active, the host not behind a NAT MUST NOT use dynamic IKEv2 address updates for IKE packets (NAT Mapping Changes, Section 3.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMobikePathProbeRetainsTunnelEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L738) | unit/verify | unproven |
| positive | [`TestMobikePathProbeRetainsTunnelEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L736) | unit/verify | unproven |

### [`RFC4555-3.8-2`](#rfc4555-3.8-2)

The responder MUST echo NAT_DETECTION_SOURCE_IP and NAT_DETECTION_DESTINATION_IP if present in any INFORMATIONAL request (NAT Mapping Changes, Section 3.8)

Audit verdict: not audited: no reader has judged these tests

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

When processing the response, the original sender of COOKIE2 MUST verify that the value is the same one as sent (Return Routability Check, Section 3.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMobikeChangedPathStillChecksCookie2`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L998) | unit/verify | unproven |
| negative | [`TestMobikeCookie2ResponseControlsMigration`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L277) | unit/verify | unproven |
| positive | [`TestMobikeCookie2ResponseControlsMigration`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L276) | unit/verify | unproven |

### [`RFC4555-3.7-5`](#rfc4555-3.7-5)

If the COOKIE2 values do not match, the IKE_SA MUST be closed (Return Routability Check, Section 3.7)

Audit verdict: not audited: no reader has judged these tests

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

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMobikeAuthNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L649) | unit/verify | unproven |
| positive | [`TestMobikeAuthNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L647) | unit/verify | unproven |

### [`RFC4555-4.2.5-1`](#rfc4555-4.2.5-1)

The data associated with the COOKIE2 notification MUST be between 8 and 64 octets in length (inclusive), and MUST be chosen by the exchange initiator in a way that is unpredictable to the exchange responder (Payload Formats, Section 4.2.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMobikeCookie2EchoBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L233) | unit/verify | unproven |
| positive | [`TestMobikeAdvertisementAfterSourceChange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L544) | unit/verify | unproven |
| positive | [`TestMobikeCookie2EchoBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L232) | unit/verify | unproven |

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
