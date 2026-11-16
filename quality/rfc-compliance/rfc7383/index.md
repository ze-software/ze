# RFC 7383 - Internet Key Exchange Protocol Version 2 (IKEv2) Message Fragmentation

IKEv2 fragmentation. Every requirement listed for RFC 7383, the tests linked to it, and their recorded reviews. This summary is excluded from automatic compliance checks.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 25 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 25 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 25 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 25 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 0 | 0 of 0 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 25 | of 25 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 25 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 25 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 25 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 25 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 25 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level), not checked | 25 | of 40 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements listed in this summary. This RFC is excluded from automatic checks (out-of-scope), so these requirements are not checked. The counts above show what the summary records. |

The 9 counts marked as a part above are the whole of the 25 mandatory requirements: they add up to 25. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level), not checked | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | IKEv2 fragmentation |
| Automatic checks | Excluded (out-of-scope) |
| Requirements | 40 |
| Mandatory requirements (MUST level), not checked | 25 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 25 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tests linked to requirements | 0 |
| Recorded audit verdicts | 0 |
| Fault test records | 0 |
| Summary | `rfc/short/rfc7383.md` |
| Requirement table | `rfc/requirements/rfc7383.md` |
| RFC text | `rfc/full/rfc7383.txt` |

## Automatic checks

Excluded from automatic checks (out-of-scope, the requirements ARE extracted and the owner decided not to offer the feature for now, so the absence is a scope decision rather than a conformance gap): OUT OF SCOPE as a document by owner decision, 2026-10-26: the owner rule committed in 1e7a9dbc34 (ai/META-DECISIONS.md, Goal and scope) counts an RFC a profile lists as optional only once Thomas brings it into scope, and this summary's `Optional use cases` row lists this RFC in the optional tier of the vpn-endpoint and vpn-concentrator profiles. The extraction is COMPLETE: the source text is at rfc/full/rfc7383.txt and this summary declares all 25 MUST-level requirements its sites state. Ze implements none of it: internal/component/ike/wire/payload_notify.go names IKEV2_FRAGMENTATION_SUPPORTED (16430) for display only, ze never sends it, and no Encrypted Fragment payload (53) is built or reassembled.

## Published support status

**Area:** IKEv2 fragmentation

**Test verification:** 0 of 25 MUST proven

**Evidence:** -

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 25 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level), not checked** | **25** | every mandatory requirement appears in exactly one group above |

**No test and no annotation (25):** [`RFC7383-2.3-1`](#rfc7383-2.3-1), [`RFC7383-2.3-2`](#rfc7383-2.3-2), [`RFC7383-2.3-3`](#rfc7383-2.3-3), [`RFC7383-2.4-1`](#rfc7383-2.4-1), [`RFC7383-2.5-1`](#rfc7383-2.5-1), [`RFC7383-2.5-2`](#rfc7383-2.5-2), [`RFC7383-2.5-3`](#rfc7383-2.5-3), [`RFC7383-2.5-4`](#rfc7383-2.5-4), [`RFC7383-2.5-5`](#rfc7383-2.5-5), [`RFC7383-2.5-6`](#rfc7383-2.5-6), [`RFC7383-2.5-7`](#rfc7383-2.5-7), [`RFC7383-2.5.2-1`](#rfc7383-2.5.2-1), [`RFC7383-2.5.2-2`](#rfc7383-2.5.2-2), [`RFC7383-2.5.2-3`](#rfc7383-2.5.2-3), [`RFC7383-2.5.2-4`](#rfc7383-2.5.2-4), [`RFC7383-2.5.2-5`](#rfc7383-2.5.2-5), [`RFC7383-2.5.3-1`](#rfc7383-2.5.3-1), [`RFC7383-2.6-1`](#rfc7383-2.6-1), [`RFC7383-2.6-2`](#rfc7383-2.6-2), [`RFC7383-2.6-3`](#rfc7383-2.6-3), [`RFC7383-2.6-4`](#rfc7383-2.6-4), [`RFC7383-2.6-5`](#rfc7383-2.6-5), [`RFC7383-2.6.1-1`](#rfc7383-2.6.1-1), [`RFC7383-2.6.1-2`](#rfc7383-2.6.1-2), [`RFC7383-3-1`](#rfc7383-3-1)

## Figures

- **MUST bar**: Not assessed 25
- **Overview bar**: Not assessed 40

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 25 |
| MUST rows kept (the MUST bar) | 25 |
| Ze verified | 0 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 25 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 15 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 15 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc7383.json](../../../data/rfc/rfc7383.json).

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

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC7383-2.3-1`](#rfc7383-2.3-1)

Protocol ID (1 octet) - MUST be 0. (§2.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7383-2.3-1, so there is no linked test.

### [`RFC7383-2.3-2`](#rfc7383-2.3-2)

SPI Size (1 octet) - MUST be 0, meaning no Security Parameter Index (SPI) is present. (§2.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7383-2.3-2, so there is no linked test.

### [`RFC7383-2.3-3`](#rfc7383-2.3-3)

Notify Message Type (2 octets) - MUST be 16430, the value assigned for the IKEV2_FRAGMENTATION_SUPPORTED notification. (§2.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7383-2.3-3, so there is no linked test.

### [`RFC7383-2.4-1`](#rfc7383-2.4-1)

IKE fragmentation MUST NOT be used unless both peers have indicated their support for it. (§2.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7383-2.4-1, so there is no linked test.

### [`RFC7383-2.5-1`](#rfc7383-2.5-1)

As is the case for the Encrypted payload, the Encrypted Fragment payload, if present in a message, MUST be the last payload in the message. (§2.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7383-2.5-1, so there is no linked test.

### [`RFC7383-2.5-2`](#rfc7383-2.5-2)

Next Payload (1 octet) - in the very first fragment (with Fragment Number equal to 1), this field MUST be set to the payload type of the first inner payload (the same as for the Encrypted payload). (§2.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7383-2.5-2, so there is no linked test.

### [`RFC7383-2.5-3`](#rfc7383-2.5-3)

In the rest of the Fragment messages (with Fragment Number greater than 1), this field MUST be set to zero. (§2.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7383-2.5-3, so there is no linked test.

### [`RFC7383-2.5-4`](#rfc7383-2.5-4)

This field MUST be less than or equal to the next field (Total Fragments). (§2.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7383-2.5-4, so there is no linked test.

### [`RFC7383-2.5-5`](#rfc7383-2.5-5)

This field MUST NOT be zero. (§2.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7383-2.5-5, so there is no linked test.

### [`RFC7383-2.5-6`](#rfc7383-2.5-6)

This field MUST NOT be zero. (§2.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7383-2.5-6, so there is no linked test.

### [`RFC7383-2.5-7`](#rfc7383-2.5-7)

When prepending the IKE header to the IKE Fragment messages, it MUST be taken intact from the original message, except for the Length and Next Payload fields. (§2.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7383-2.5-7, so there is no linked test.

### [`RFC7383-2.5.2-1`](#rfc7383-2.5.2-1)

Implementations MUST be prepared to probe in every exchange that utilizes IKE fragmentation to deal with possible changes in path MTU over time. (§2.5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7383-2.5.2-1, so there is no linked test.

### [`RFC7383-2.5.2-2`](#rfc7383-2.5.2-2)

While doing probes, it MUST start from larger values and refragment the original message, using the next smaller value of the threshold if it did not receive a response in a reasonable time after several retransmissions. (§2.5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7383-2.5.2-2, so there is no linked test.

### [`RFC7383-2.5.2-3`](#rfc7383-2.5.2-3)

When starting a new probe, the node MUST reset its retransmission timers so that if it employs exponential back-off the timers will start over. (§2.5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7383-2.5.2-3, so there is no linked test.

### [`RFC7383-2.5.2-4`](#rfc7383-2.5.2-4)

After reaching the smallest allowed value for the fragmentation threshold, an implementation MUST continue retransmitting until the exchange either completes or times out using some timeout interval as discussed in Section 2.4 of [RFC7296]. (§2.5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7383-2.5.2-4, so there is no linked test.

### [`RFC7383-2.5.2-5`](#rfc7383-2.5.2-5)

When selecting the next smaller value for the fragmentation threshold, the sender MUST ensure that the value in the Total Fragments field is really increased. (§2.5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7383-2.5.2-5, so there is no linked test.

### [`RFC7383-2.5.3-1`](#rfc7383-2.5.3-1)

If some future IKEv2 extension defines such a message and it needs to be fragmented, all unprotected payloads MUST be placed in the first fragment (with the Fragment Number field equal to 1), along with the Encrypted Fragment payload, which MUST be present in every IKE Fragment message and be the last payload in it. (§2.5.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7383-2.5.3-1, so there is no linked test.

### [`RFC7383-2.6-1`](#rfc7383-2.6-1)

If any of these tests fail, the message MUST be silently discarded. (§2.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7383-2.6-1, so there is no linked test.

### [`RFC7383-2.6-2`](#rfc7383-2.6-2)

If an IKE Fragment message with the same Message ID, Fragment Number, and Total Fragments fields is already present in the reassembling queue, this message is considered a replay and MUST be silently discarded. (§2.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7383-2.6-2, so there is no linked test.

### [`RFC7383-2.6-3`](#rfc7383-2.6-3)

If the ICV check fails, the message MUST be silently discarded. (§2.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7383-2.6-3, so there is no linked test.

### [`RFC7383-2.6-4`](#rfc7383-2.6-4)

If reassembling is not finished yet and the Total Fragments field in the received fragment is greater than the Total Fragments field in those fragments that are in the reassembling queue, the receiver MUST discard all received fragments and start the reassembly process over with just the received IKE Fragment message. (§2.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7383-2.6-4, so there is no linked test.

### [`RFC7383-2.6-5`](#rfc7383-2.6-5)

If the receiver does not get all IKE fragments needed to reassemble the original message within a timeout interval, it MUST discard all IKE Fragment messages received so far for the exchange. (§2.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7383-2.6-5, so there is no linked test.

### [`RFC7383-2.6.1-1`](#rfc7383-2.6.1-1)

If an incoming message contains an Encrypted Fragment payload, the values of the Fragment Number and Total Fragments fields MUST be used along with the Message ID to detect retransmissions and replays. (§2.6.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7383-2.6.1-1, so there is no linked test.

### [`RFC7383-2.6.1-2`](#rfc7383-2.6.1-2)

If the responder receives a retransmitted fragment of a request when it has already processed that request and has sent back a response, that event MUST only trigger a retransmission of the response message (fragmented or not) if the Fragment Number field in the received fragment is set to 1; otherwise, it MUST be ignored. (§2.6.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7383-2.6.1-2, so there is no linked test.

### [`RFC7383-3-1`](#rfc7383-3-1)

When implementing IKE fragmentation along with [RFC6311], IKE Message ID Synchronization messages MUST NOT be sent fragmented, to simplify the receiver's task of detecting replays. (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7383-3-1, so there is no linked test.

## Requirement list review

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
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `1.2` | not stated | 0 | reviewed | not stated |
| `1.3` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `2.1` | not stated | 0 | reviewed | not stated |
| `2.2` | not stated | 0 | reviewed | not stated |
| `2.3` | not stated | 3 | reviewed | not stated |
| `2.4` | not stated | 1 | reviewed | not stated |
| `2.5` | not stated | 7 | reviewed | not stated |
| `2.5.1` | not stated | 0 | reviewed | not stated |
| `2.5.2` | not stated | 5 | reviewed | not stated |
| `2.5.3` | not stated | 1 | reviewed | not stated |
| `2.6` | not stated | 5 | reviewed | not stated |
| `2.6.1` | not stated | 2 | reviewed | not stated |
| `3` | not stated | 1 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | IANA considerations | 0 | skipped (iana) | IANA considerations |
| `7` | references | 0 | skipped (references) | references |
| `7.1` | normative references | 0 | skipped (references) | normative references |
| `7.2` | informative references | 0 | skipped (references) | informative references |
| `A` | design rationale | 0 | skipped (appendix-non-normative) | design rationale |
| `B` | datagram size arithmetic | 0 | skipped (appendix-non-normative) | datagram size arithmetic |

### Excluded sentences

The review of RFC 7383 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 7383, so its obligations are stated where they were written.
