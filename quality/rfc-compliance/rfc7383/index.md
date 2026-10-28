# RFC 7383 - Internet Key Exchange Protocol Version 2 (IKEv2) Message Fragmentation

Future. Every requirement this repository extracted from RFC 7383, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 25 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 25 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 25 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 25 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 25 | of 40 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (out-of-scope), so every share below reads what the summary records rather than what the gate enforces |
| Out of scope | 0 | of 25 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 25 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 25 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 25 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |
| Linux cannot do this | 0.0% | 0 of 25 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and remains unmet; it stays in every denominator and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 100.0% | 25 of 25 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 shares marked as a part above are the whole of the 25 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| MUSTs declared | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Future |
| Enrolment | Not enrolled (out-of-scope) |
| Requirements | 40 |
| Gated MUST-level | 25 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 25 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc7383.md` |
| Requirement shard | `rfc/requirements/rfc7383.md` |
| RFC text | `rfc/full/rfc7383.txt` |

## Enrolment

Not enrolled (out-of-scope, the requirements ARE extracted and the owner decided not to offer the feature for now, so the absence is a scope decision rather than a conformance gap): OUT OF SCOPE as a document by owner decision, 2026-10-26: the owner rule committed in 1e7a9dbc34 (ai/META-DECISIONS.md, Goal and scope) counts an RFC a profile lists as optional only once Thomas brings it into scope, and this summary's `Optional use cases` row lists this RFC in the optional tier of the vpn-endpoint and vpn-concentrator profiles. The extraction is COMPLETE: the source text is at rfc/full/rfc7383.txt and this summary declares all 25 MUST-level requirements its sites state. Ze implements none of it: internal/component/ike/wire/payload_notify.go names IKEV2_FRAGMENTATION_SUPPORTED (16430) for display only, ze never sends it, and no Encrypted Fragment payload (53) is built or reassembled.

## What the public ledger says

**Status:** Future

**What the ledger says is covered:**

None. Ze does not announce IKEV2_FRAGMENTATION_SUPPORTED, so a conforming peer never fragments toward it (Section 2.4).

**What the ledger says remains:**

IKEV2_FRAGMENTATION_SUPPORTED negotiation, the Encrypted Fragment payload, PMTU probing and reassembly. Out of scope by owner decision, 2026-10-26.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated (including scoped evidence) | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 25 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **25** | every gated MUST falls in exactly one bucket above |

**No test and no annotation (25):** [`RFC7383-2.3-1`](#rfc7383-2.3-1), [`RFC7383-2.3-2`](#rfc7383-2.3-2), [`RFC7383-2.3-3`](#rfc7383-2.3-3), [`RFC7383-2.4-1`](#rfc7383-2.4-1), [`RFC7383-2.5-1`](#rfc7383-2.5-1), [`RFC7383-2.5-2`](#rfc7383-2.5-2), [`RFC7383-2.5-3`](#rfc7383-2.5-3), [`RFC7383-2.5-4`](#rfc7383-2.5-4), [`RFC7383-2.5-5`](#rfc7383-2.5-5), [`RFC7383-2.5-6`](#rfc7383-2.5-6), [`RFC7383-2.5-7`](#rfc7383-2.5-7), [`RFC7383-2.5.2-1`](#rfc7383-2.5.2-1), [`RFC7383-2.5.2-2`](#rfc7383-2.5.2-2), [`RFC7383-2.5.2-3`](#rfc7383-2.5.2-3), [`RFC7383-2.5.2-4`](#rfc7383-2.5.2-4), [`RFC7383-2.5.2-5`](#rfc7383-2.5.2-5), [`RFC7383-2.5.3-1`](#rfc7383-2.5.3-1), [`RFC7383-2.6-1`](#rfc7383-2.6-1), [`RFC7383-2.6-2`](#rfc7383-2.6-2), [`RFC7383-2.6-3`](#rfc7383-2.6-3), [`RFC7383-2.6-4`](#rfc7383-2.6-4), [`RFC7383-2.6-5`](#rfc7383-2.6-5), [`RFC7383-2.6.1-1`](#rfc7383-2.6.1-1), [`RFC7383-2.6.1-2`](#rfc7383-2.6.1-2), [`RFC7383-3-1`](#rfc7383-3-1)

## Attainable ceiling

Mandatory ceiling: 25 of 25 (100.0%). Optional ceiling: 15 of 15 (100.0%). Every RFC shown with measured rows can reach 100% under current classifications. Lowest mandatory ceiling: rfc7383 25 of 25 (100.0%). Lowest optional ceiling: rfc7383 15 of 15 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7383-2.3-1` | Protocol ID (1 octet) - MUST be 0. (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.3-2` | SPI Size (1 octet) - MUST be 0, meaning no Security Parameter Index (SPI) is present. (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.3-3` | Notify Message Type (2 octets) - MUST be 16430, the value assigned for the IKEV2_FRAGMENTATION_SUPPORTED notification. (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.4-1` | IKE fragmentation MUST NOT be used unless both peers have indicated their support for it. (§2.4) | MUST NOT | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.5-1` | As is the case for the Encrypted payload, the Encrypted Fragment payload, if present in a message, MUST be the last payload in the message. (§2.5) | MUST | 2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.5-2` | Next Payload (1 octet) - in the very first fragment (with Fragment Number equal to 1), this field MUST be set to the payload type of the first inner payload (the same as for the Encrypted payload). (§2.5) | MUST | 2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.5-3` | In the rest of the Fragment messages (with Fragment Number greater than 1), this field MUST be set to zero. (§2.5) | MUST | 2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.5-4` | This field MUST be less than or equal to the next field (Total Fragments). (§2.5) | MUST | 2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.5-5` | This field MUST NOT be zero. (§2.5) | MUST NOT | 2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.5-6` | This field MUST NOT be zero. (§2.5) | MUST NOT | 2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.5-7` | When prepending the IKE header to the IKE Fragment messages, it MUST be taken intact from the original message, except for the Length and Next Payload fields. (§2.5) | MUST | 2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.5.2-1` | Implementations MUST be prepared to probe in every exchange that utilizes IKE fragmentation to deal with possible changes in path MTU over time. (§2.5.2) | MUST | 2.5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.5.2-2` | While doing probes, it MUST start from larger values and refragment the original message, using the next smaller value of the threshold if it did not receive a response in a reasonable time after several retransmissions. (§2.5.2) | MUST | 2.5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.5.2-3` | When starting a new probe, the node MUST reset its retransmission timers so that if it employs exponential back-off the timers will start over. (§2.5.2) | MUST | 2.5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.5.2-4` | After reaching the smallest allowed value for the fragmentation threshold, an implementation MUST continue retransmitting until the exchange either completes or times out using some timeout interval as discussed in Section 2.4 of [RFC7296]. (§2.5.2) | MUST | 2.5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.5.2-5` | When selecting the next smaller value for the fragmentation threshold, the sender MUST ensure that the value in the Total Fragments field is really increased. (§2.5.2) | MUST | 2.5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.5.3-1` | If some future IKEv2 extension defines such a message and it needs to be fragmented, all unprotected payloads MUST be placed in the first fragment (with the Fragment Number field equal to 1), along with the Encrypted Fragment payload, which MUST be present in every IKE Fragment message and be the last payload in it. (§2.5.3) | MUST | 2.5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.6-1` | If any of these tests fail, the message MUST be silently discarded. (§2.6) | MUST | 2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.6-2` | If an IKE Fragment message with the same Message ID, Fragment Number, and Total Fragments fields is already present in the reassembling queue, this message is considered a replay and MUST be silently discarded. (§2.6) | MUST | 2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.6-3` | If the ICV check fails, the message MUST be silently discarded. (§2.6) | MUST | 2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.6-4` | If reassembling is not finished yet and the Total Fragments field in the received fragment is greater than the Total Fragments field in those fragments that are in the reassembling queue, the receiver MUST discard all received fragments and start the reassembly process over with just the received IKE Fragment message. (§2.6) | MUST | 2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.6-5` | If the receiver does not get all IKE fragments needed to reassemble the original message within a timeout interval, it MUST discard all IKE Fragment messages received so far for the exchange. (§2.6) | MUST | 2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.6.1-1` | If an incoming message contains an Encrypted Fragment payload, the values of the Fragment Number and Total Fragments fields MUST be used along with the Message ID to detect retransmissions and replays. (§2.6.1) | MUST | 2.6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.6.1-2` | If the responder receives a retransmitted fragment of a request when it has already processed that request and has sent back a response, that event MUST only trigger a retransmission of the response message (fragmented or not) if the Fragment Number field in the received fragment is set to 1; otherwise, it MUST be ignored. (§2.6.1) | MUST | 2.6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-3-1` | When implementing IKE fragmentation along with [RFC6311], IKE Message ID Synchronization messages MUST NOT be sent fragmented, to simplify the receiver's task of detecting replays. (§3) | MUST NOT | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.4-2` | If either peer has information that a part of the transaction is likely to be fragmented at the IP layer, causing interference with the IKE exchange, that peer SHOULD use IKE fragmentation. (§2.4) | SHOULD | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.4-3` | If either peer knows that IKE fragmentation has been used in a previous exchange in the context of the current IKE SA, that peer SHOULD continue to use IKE fragmentation for the messages that are larger than the current fragmentation threshold (§2.4) | SHOULD | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.4-4` | IKE fragmentation SHOULD NOT be used in cases where IP-layer fragmentation of both the request and response messages is unlikely. (§2.4) | SHOULD NOT | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.4-5` | If none of the above apply, the responder SHOULD respond in the same form (fragmented or not) as the request message to which it is responding. (§2.4) | SHOULD | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.5.1-1` | When splitting the content of an Encrypted payload into chunks, the sender SHOULD choose their size so that the resulting IP datagrams will be smaller than some fragmentation threshold. (§2.5.1) | SHOULD | 2.5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.5.1-2` | If the sender has information about the PMTU size, it SHOULD use it. (§2.5.1) | SHOULD | 2.5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.5.1-3` | Otherwise, for messages to be sent over IPv6, it is RECOMMENDED that a value of 1280 bytes as a maximum IP datagram size be used ([RFC2460]). (§2.5.1) | RECOMMENDED | 2.5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.5.1-4` | For messages to be sent over IPv4, it is RECOMMENDED that a value of 576 bytes as a maximum IP datagram size be used. (§2.5.1) | RECOMMENDED | 2.5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.5.2-6` | For the reasons articulated above, implementing PMTU discovery in IKE is OPTIONAL. (§2.5.2) | OPTIONAL | 2.5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.5.2-7` | then the initiator SHOULD stop retransmitting the probe and SHOULD select a new value for the fragmentation threshold (§2.5.2) | SHOULD | 2.5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-2.6-6` | It is RECOMMENDED that the reassembling timeout for the responder be equal to the time interval that the implementation waits before completely giving up when acting as the initiator of an exchange. (§2.6) | RECOMMENDED | 2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-4-1` | When using IKE fragmentation, implementations MAY use longer timeouts and do more retransmits than usual before considering the peer dead. (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-5-1` | To mitigate the impact of this attack, it is RECOMMENDED that the receiver limit the number of fragments it stores in the reassembling queue (§5) | RECOMMENDED | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-5-2` | In either case, the receiver SHOULD drop the connection and discard all the received fragments. (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7383-5-3` | It is NOT RECOMMENDED for this value to exceed 64 KB because any IKE message before fragmentation would likely be shorter than that. (§5) | NOT RECOMMENDED | 5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7383-2.3-1`](#rfc7383-2.3-1) Protocol ID (1 octet) - MUST be 0. (§2.3) | no test | no test carries this requirement id |
| [`RFC7383-2.3-2`](#rfc7383-2.3-2) SPI Size (1 octet) - MUST be 0, meaning no Security Parameter Index (SPI) is present. (§2.3) | no test | no test carries this requirement id |
| [`RFC7383-2.3-3`](#rfc7383-2.3-3) Notify Message Type (2 octets) - MUST be 16430, the value assigned for the IKEV2_FRAGMENTATION_SUPPORTED notification. (§2.3) | no test | no test carries this requirement id |
| [`RFC7383-2.4-1`](#rfc7383-2.4-1) IKE fragmentation MUST NOT be used unless both peers have indicated their support for it. (§2.4) | no test | no test carries this requirement id |
| [`RFC7383-2.5-1`](#rfc7383-2.5-1) As is the case for the Encrypted payload, the Encrypted Fragment payload, if present in a message, MUST be the last payload in the message. (§2.5) | no test | no test carries this requirement id |
| [`RFC7383-2.5-2`](#rfc7383-2.5-2) Next Payload (1 octet) - in the very first fragment (with Fragment Number equal to 1), this field MUST be set to the payload type of the first inner payload (the same as for the Encrypted payload). (§2.5) | no test | no test carries this requirement id |
| [`RFC7383-2.5-3`](#rfc7383-2.5-3) In the rest of the Fragment messages (with Fragment Number greater than 1), this field MUST be set to zero. (§2.5) | no test | no test carries this requirement id |
| [`RFC7383-2.5-4`](#rfc7383-2.5-4) This field MUST be less than or equal to the next field (Total Fragments). (§2.5) | no test | no test carries this requirement id |
| [`RFC7383-2.5-5`](#rfc7383-2.5-5) This field MUST NOT be zero. (§2.5) | no test | no test carries this requirement id |
| [`RFC7383-2.5-6`](#rfc7383-2.5-6) This field MUST NOT be zero. (§2.5) | no test | no test carries this requirement id |
| [`RFC7383-2.5-7`](#rfc7383-2.5-7) When prepending the IKE header to the IKE Fragment messages, it MUST be taken intact from the original message, except for the Length and Next Payload fields. (§2.5) | no test | no test carries this requirement id |
| [`RFC7383-2.5.2-1`](#rfc7383-2.5.2-1) Implementations MUST be prepared to probe in every exchange that utilizes IKE fragmentation to deal with possible changes in path MTU over time. (§2.5.2) | no test | no test carries this requirement id |
| [`RFC7383-2.5.2-2`](#rfc7383-2.5.2-2) While doing probes, it MUST start from larger values and refragment the original message, using the next smaller value of the threshold if it did not receive a response in a reasonable time after several retransmissions. (§2.5.2) | no test | no test carries this requirement id |
| [`RFC7383-2.5.2-3`](#rfc7383-2.5.2-3) When starting a new probe, the node MUST reset its retransmission timers so that if it employs exponential back-off the timers will start over. (§2.5.2) | no test | no test carries this requirement id |
| [`RFC7383-2.5.2-4`](#rfc7383-2.5.2-4) After reaching the smallest allowed value for the fragmentation threshold, an implementation MUST continue retransmitting until the exchange either completes or times out using some timeout interval as discussed in Section 2.4 of [RFC7296]. (§2.5.2) | no test | no test carries this requirement id |
| [`RFC7383-2.5.2-5`](#rfc7383-2.5.2-5) When selecting the next smaller value for the fragmentation threshold, the sender MUST ensure that the value in the Total Fragments field is really increased. (§2.5.2) | no test | no test carries this requirement id |
| [`RFC7383-2.5.3-1`](#rfc7383-2.5.3-1) If some future IKEv2 extension defines such a message and it needs to be fragmented, all unprotected payloads MUST be placed in the first fragment (with the Fragment Number field equal to 1), along with the Encrypted Fragment payload, which MUST be present in every IKE Fragment message and be the last payload in it. (§2.5.3) | no test | no test carries this requirement id |
| [`RFC7383-2.6-1`](#rfc7383-2.6-1) If any of these tests fail, the message MUST be silently discarded. (§2.6) | no test | no test carries this requirement id |
| [`RFC7383-2.6-2`](#rfc7383-2.6-2) If an IKE Fragment message with the same Message ID, Fragment Number, and Total Fragments fields is already present in the reassembling queue, this message is considered a replay and MUST be silently discarded. (§2.6) | no test | no test carries this requirement id |
| [`RFC7383-2.6-3`](#rfc7383-2.6-3) If the ICV check fails, the message MUST be silently discarded. (§2.6) | no test | no test carries this requirement id |
| [`RFC7383-2.6-4`](#rfc7383-2.6-4) If reassembling is not finished yet and the Total Fragments field in the received fragment is greater than the Total Fragments field in those fragments that are in the reassembling queue, the receiver MUST discard all received fragments and start the reassembly process over with just the received IKE Fragment message. (§2.6) | no test | no test carries this requirement id |
| [`RFC7383-2.6-5`](#rfc7383-2.6-5) If the receiver does not get all IKE fragments needed to reassemble the original message within a timeout interval, it MUST discard all IKE Fragment messages received so far for the exchange. (§2.6) | no test | no test carries this requirement id |
| [`RFC7383-2.6.1-1`](#rfc7383-2.6.1-1) If an incoming message contains an Encrypted Fragment payload, the values of the Fragment Number and Total Fragments fields MUST be used along with the Message ID to detect retransmissions and replays. (§2.6.1) | no test | no test carries this requirement id |
| [`RFC7383-2.6.1-2`](#rfc7383-2.6.1-2) If the responder receives a retransmitted fragment of a request when it has already processed that request and has sent back a response, that event MUST only trigger a retransmission of the response message (fragmented or not) if the Fragment Number field in the received fragment is set to 1; otherwise, it MUST be ignored. (§2.6.1) | no test | no test carries this requirement id |
| [`RFC7383-3-1`](#rfc7383-3-1) When implementing IKE fragmentation along with [RFC6311], IKE Message ID Synchronization messages MUST NOT be sent fragmented, to simplify the receiver's task of detecting replays. (§3) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7383-2.3-1`](#rfc7383-2.3-1)

Protocol ID (1 octet) - MUST be 0. (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7383-2.3-1, so no unit is bound to it.

### [`RFC7383-2.3-2`](#rfc7383-2.3-2)

SPI Size (1 octet) - MUST be 0, meaning no Security Parameter Index (SPI) is present. (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7383-2.3-2, so no unit is bound to it.

### [`RFC7383-2.3-3`](#rfc7383-2.3-3)

Notify Message Type (2 octets) - MUST be 16430, the value assigned for the IKEV2_FRAGMENTATION_SUPPORTED notification. (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7383-2.3-3, so no unit is bound to it.

### [`RFC7383-2.4-1`](#rfc7383-2.4-1)

IKE fragmentation MUST NOT be used unless both peers have indicated their support for it. (§2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7383-2.4-1, so no unit is bound to it.

### [`RFC7383-2.5-1`](#rfc7383-2.5-1)

As is the case for the Encrypted payload, the Encrypted Fragment payload, if present in a message, MUST be the last payload in the message. (§2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7383-2.5-1, so no unit is bound to it.

### [`RFC7383-2.5-2`](#rfc7383-2.5-2)

Next Payload (1 octet) - in the very first fragment (with Fragment Number equal to 1), this field MUST be set to the payload type of the first inner payload (the same as for the Encrypted payload). (§2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7383-2.5-2, so no unit is bound to it.

### [`RFC7383-2.5-3`](#rfc7383-2.5-3)

In the rest of the Fragment messages (with Fragment Number greater than 1), this field MUST be set to zero. (§2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7383-2.5-3, so no unit is bound to it.

### [`RFC7383-2.5-4`](#rfc7383-2.5-4)

This field MUST be less than or equal to the next field (Total Fragments). (§2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7383-2.5-4, so no unit is bound to it.

### [`RFC7383-2.5-5`](#rfc7383-2.5-5)

This field MUST NOT be zero. (§2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7383-2.5-5, so no unit is bound to it.

### [`RFC7383-2.5-6`](#rfc7383-2.5-6)

This field MUST NOT be zero. (§2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7383-2.5-6, so no unit is bound to it.

### [`RFC7383-2.5-7`](#rfc7383-2.5-7)

When prepending the IKE header to the IKE Fragment messages, it MUST be taken intact from the original message, except for the Length and Next Payload fields. (§2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7383-2.5-7, so no unit is bound to it.

### [`RFC7383-2.5.2-1`](#rfc7383-2.5.2-1)

Implementations MUST be prepared to probe in every exchange that utilizes IKE fragmentation to deal with possible changes in path MTU over time. (§2.5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7383-2.5.2-1, so no unit is bound to it.

### [`RFC7383-2.5.2-2`](#rfc7383-2.5.2-2)

While doing probes, it MUST start from larger values and refragment the original message, using the next smaller value of the threshold if it did not receive a response in a reasonable time after several retransmissions. (§2.5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7383-2.5.2-2, so no unit is bound to it.

### [`RFC7383-2.5.2-3`](#rfc7383-2.5.2-3)

When starting a new probe, the node MUST reset its retransmission timers so that if it employs exponential back-off the timers will start over. (§2.5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7383-2.5.2-3, so no unit is bound to it.

### [`RFC7383-2.5.2-4`](#rfc7383-2.5.2-4)

After reaching the smallest allowed value for the fragmentation threshold, an implementation MUST continue retransmitting until the exchange either completes or times out using some timeout interval as discussed in Section 2.4 of [RFC7296]. (§2.5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7383-2.5.2-4, so no unit is bound to it.

### [`RFC7383-2.5.2-5`](#rfc7383-2.5.2-5)

When selecting the next smaller value for the fragmentation threshold, the sender MUST ensure that the value in the Total Fragments field is really increased. (§2.5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7383-2.5.2-5, so no unit is bound to it.

### [`RFC7383-2.5.3-1`](#rfc7383-2.5.3-1)

If some future IKEv2 extension defines such a message and it needs to be fragmented, all unprotected payloads MUST be placed in the first fragment (with the Fragment Number field equal to 1), along with the Encrypted Fragment payload, which MUST be present in every IKE Fragment message and be the last payload in it. (§2.5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7383-2.5.3-1, so no unit is bound to it.

### [`RFC7383-2.6-1`](#rfc7383-2.6-1)

If any of these tests fail, the message MUST be silently discarded. (§2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7383-2.6-1, so no unit is bound to it.

### [`RFC7383-2.6-2`](#rfc7383-2.6-2)

If an IKE Fragment message with the same Message ID, Fragment Number, and Total Fragments fields is already present in the reassembling queue, this message is considered a replay and MUST be silently discarded. (§2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7383-2.6-2, so no unit is bound to it.

### [`RFC7383-2.6-3`](#rfc7383-2.6-3)

If the ICV check fails, the message MUST be silently discarded. (§2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7383-2.6-3, so no unit is bound to it.

### [`RFC7383-2.6-4`](#rfc7383-2.6-4)

If reassembling is not finished yet and the Total Fragments field in the received fragment is greater than the Total Fragments field in those fragments that are in the reassembling queue, the receiver MUST discard all received fragments and start the reassembly process over with just the received IKE Fragment message. (§2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7383-2.6-4, so no unit is bound to it.

### [`RFC7383-2.6-5`](#rfc7383-2.6-5)

If the receiver does not get all IKE fragments needed to reassemble the original message within a timeout interval, it MUST discard all IKE Fragment messages received so far for the exchange. (§2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7383-2.6-5, so no unit is bound to it.

### [`RFC7383-2.6.1-1`](#rfc7383-2.6.1-1)

If an incoming message contains an Encrypted Fragment payload, the values of the Fragment Number and Total Fragments fields MUST be used along with the Message ID to detect retransmissions and replays. (§2.6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7383-2.6.1-1, so no unit is bound to it.

### [`RFC7383-2.6.1-2`](#rfc7383-2.6.1-2)

If the responder receives a retransmitted fragment of a request when it has already processed that request and has sent back a response, that event MUST only trigger a retransmission of the response message (fragmented or not) if the Fragment Number field in the received fragment is set to 1; otherwise, it MUST be ignored. (§2.6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7383-2.6.1-2, so no unit is bound to it.

### [`RFC7383-3-1`](#rfc7383-3-1)

When implementing IKE fragmentation along with [RFC6311], IKE Message ID Synchronization messages MUST NOT be sent fragmented, to simplify the receiver's task of detecting replays. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7383-3-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude (ze-rfc phase-1 agent, session 45f03892) |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc7383.txt |
| Source fingerprint | e5b43a1f70355e76 |
| Record | rfc/extraction/rfc7383.json |
| Mapped sentences | 25 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | title, status and table of contents | 0 | skipped (front-matter) | title, status and table of contents |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `1.3` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `2.3` | not stated | 3 | walked | not stated |
| `2.4` | not stated | 1 | walked | not stated |
| `2.5` | not stated | 7 | walked | not stated |
| `2.5.1` | not stated | 0 | walked | not stated |
| `2.5.2` | not stated | 5 | walked | not stated |
| `2.5.3` | not stated | 1 | walked | not stated |
| `2.6` | not stated | 5 | walked | not stated |
| `2.6.1` | not stated | 2 | walked | not stated |
| `3` | not stated | 1 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | IANA considerations | 0 | skipped (iana) | IANA considerations |
| `7` | references | 0 | skipped (references) | references |
| `7.1` | normative references | 0 | skipped (references) | normative references |
| `7.2` | informative references | 0 | skipped (references) | informative references |
| `A` | design rationale | 0 | skipped (appendix-non-normative) | design rationale |
| `B` | datagram size arithmetic | 0 | skipped (appendix-non-normative) | datagram size arithmetic |

### Excluded sentences

The walk over RFC 7383 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 7383, so its obligations are stated where they were written.
