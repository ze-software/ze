# RFC 1570 - PPP LCP Extensions

Future. Every requirement this repository extracted from RFC 1570, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 22 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 22 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 22 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 22 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 22 | of 38 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (out-of-scope), so every share below reads what the summary records rather than what the gate enforces |
| Out of scope | 0 | of 22 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 22 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 22 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 22 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 100.0% | 22 of 22 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 22 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Future |
| Enrolment | Not enrolled (out-of-scope) |
| Requirements | 38 |
| Gated MUST-level | 22 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 22 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc1570.md` |
| Requirement shard | `rfc/requirements/rfc1570.md` |
| RFC text | `rfc/full/rfc1570.txt` |

## Enrolment

Not enrolled (out-of-scope, the requirements ARE extracted and the owner decided not to offer the feature for now, so the absence is a scope decision rather than a conformance gap): OUT OF SCOPE as a document by owner decision, 2026-10-26: the owner rule committed in 1e7a9dbc34 (ai/META-DECISIONS.md, Goal and scope) counts an RFC a profile lists as optional only once Thomas brings it into scope, and this summary's `Optional use cases` row lists this RFC in the optional tier of the lns profile. The extraction is COMPLETE: the source text is at rfc/full/rfc1570.txt, every capitalised MUST-level sentence is a row of this summary or an exclusion in rfc/extraction/rfc1570.json (walked 2026-10-26), and none is gated. Ze's PPP implements none of the LCP codes or options this document adds (internal/component/l2tp/ppp/lcp_options.go carries options 1, 2, 3, 5, 7 and 8 only), so an Identification or Time-Remaining packet is answered as an unknown code under RFC 1661.

## What the public ledger says

**Status:** Future

**What the ledger says is covered:**

- None. Identification (code 12), Time-Remaining (code 13) and the FCS-Alternatives, Self-Describing-Padding, Callback and Compound-Frames options are not offered
- an unknown option is rejected under RFC 1661.


**What the ledger says remains:**

Identification, Time-Remaining, FCS-Alternatives, Self-Describing-Padding, Callback and Compound-Frames. Out of scope by owner decision, 2026-10-26.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated (including scoped evidence) | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 22 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **22** | every gated MUST falls in exactly one bucket above |

**No test and no annotation (22):** [`RFC1570-1.1-1`](#rfc1570-1.1-1), [`RFC1570-1.1-2`](#rfc1570-1.1-2), [`RFC1570-1.1-3`](#rfc1570-1.1-3), [`RFC1570-1.2-1`](#rfc1570-1.2-1), [`RFC1570-1.2-2`](#rfc1570-1.2-2), [`RFC1570-1.2-3`](#rfc1570-1.2-3), [`RFC1570-2.1-1`](#rfc1570-2.1-1), [`RFC1570-2.1.1-1`](#rfc1570-2.1.1-1), [`RFC1570-2.1.1-2`](#rfc1570-2.1.1-2), [`RFC1570-2.1.1-3`](#rfc1570-2.1.1-3), [`RFC1570-2.1.2-1`](#rfc1570-2.1.2-1), [`RFC1570-2.1.2-2`](#rfc1570-2.1.2-2), [`RFC1570-2.1.2-3`](#rfc1570-2.1.2-3), [`RFC1570-2.1.2-4`](#rfc1570-2.1.2-4), [`RFC1570-2.2-1`](#rfc1570-2.2-1), [`RFC1570-2.2-2`](#rfc1570-2.2-2), [`RFC1570-2.2-3`](#rfc1570-2.2-3), [`RFC1570-2.2-4`](#rfc1570-2.2-4), [`RFC1570-2.4-1`](#rfc1570-2.4-1), [`RFC1570-2.4-2`](#rfc1570-2.4-2), [`RFC1570-2.4-3`](#rfc1570-2.4-3), [`RFC1570-2.4.1-1`](#rfc1570-2.4.1-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1570-1.1-1` | The Identifier field MUST be changed for each Identification sent. (§1.1) | MUST | 1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-1.1-2` | Until the Magic-Number Configuration Option has been successfully negotiated, the Magic- Number MUST be transmitted as zero. (§1.1) | MUST | 1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-1.1-3` | It is intended to be human readable, and MUST NOT affect operation of the protocol. (§1.1) | MUST NOT | 1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-1.2-1` | The Identifier field MUST be changed for each Time-Remaining sent. (§1.2) | MUST | 1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-1.2-2` | Until the Magic-Number Configuration Option has been successfully negotiated, the Magic- Number MUST be transmitted as zero. (§1.2) | MUST | 1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-1.2-3` | It is intended to be human readable, and MUST NOT affect operation of the protocol. (§1.2) | MUST NOT | 1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-2.1-1` | Frames sent during any other phase MUST contain the default FCS. (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-2.1.1-1` | On receipt of a LCP Configure-Request or Terminate-Request packet, the implementation MUST change to the default FCS for both transmission and reception. (§2.1.1) | MUST | 2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-2.1.1-2` | If a Request packet is received which contains a duplicate Identifier field, a new reply MUST be generated. (§2.1.1) | MUST | 2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-2.1.1-3` | If such a 48-bit FCS is used, it MUST only be used for LCP packets. (§2.1.1) | MUST | 2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-2.1.2-1` | When a configuration (LCP or NCP) or authentication packet is sent, the FCS MUST be included. (§2.1.2) | MUST | 2.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-2.1.2-2` | When a configuration (LCP or NCP) or authentication packet is received, the FCS MUST be verified. (§2.1.2) | MUST | 2.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-2.1.2-3` | The incorrect FCS MUST be passed with the rest of the data. (§2.1.2) | MUST | 2.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-2.1.2-4` | The frame MUST NOT be discarded until after demultiplexing, and only those frames that require the FCS are discarded. (§2.1.2) | MUST NOT | 2.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-2.2-1` | If the option is Rejected, the peer MUST NOT add any padding to the identified special protocols, but MAY add padding to other protocols. (§2.2) | MUST NOT | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-2.2-2` | If the option is Ack'd, the peer MUST follow the procedures for adding self-describing pads, but only to the specifically identified protocols. (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-2.2-3` | The first pad octet MUST contain the value one (1), which indicates the Padding Protocol to the Compound-Frames option. (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-2.2-4` | When no padding would otherwise be required, but the final octet of the PPP Information field contains the value 1 through MPV, at least one self-describing pad octet MUST be added to the frame. (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-2.4-1` | Therefore, this option MUST be negotiated together with the Self- Describing-Padding option. (§2.4) | MUST | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-2.4-2` | If the FCS-Alternatives option has been negotiated, self describing padding MUST always be added. (§2.4) | MUST | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-2.4-3` | That is, the final packet MUST be followed by a series of octets, the first of which contains the value one (1). (§2.4) | MUST | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-2.4.1-1` | Since the peer has agreed to send compound frames, the implementation MUST examine the remainder of the frame for additional packets. (§2.4.1) | MUST | 2.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-1.1-5` | Receipt of a Code-Reject for the Identification packet SHOULD generate the RXJ+ (permitted) event. (§1.1) | SHOULD | 1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-1.1-6` | A robust implementation SHOULD treat the Message as displayable text, and SHOULD be able to receive and display a very long Message. (§1.1) | SHOULD | 1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-1.2-4` | Receipt of a Code-Reject for the Time-Remaining packet SHOULD generate the RXJ+ (permitted) event. (§1.2) | SHOULD | 1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-2.1.1-4` | The Identifier field SHOULD NOT be incremented for each such duplicate packet. (§2.1.1) | SHOULD NOT | 2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-2.1.2-5` | The Null FCS SHOULD only be used for those network-layer and transport protocols which have an end-to-end checksum available, such as TCP/IP, or UDP/IP with the checksum enabled. (§2.1.2) | SHOULD | 2.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-2.1.2-6` | That is, the Null FCS option SHOULD be negotiated together with another non-null FCS option in a heterogeneous environment. (§2.1.2) | SHOULD | 2.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-2.2-5` | To avoid unnecessary configuration handshakes, an implementation which generates padding, and has a protocol configured which requires the padding to be known, SHOULD include this Option in its Configure-Request, and SHOULD Configure-Nak with this Option when it is not present in the peer's Request. (§2.2) | SHOULD | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-2.2-6` | Implementation Notes: If any of the pad octets contain an incorrect index value, the entire frame SHOULD be silently discarded. (§2.2) | SHOULD | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-2.3-1` | Implementation Notes: A peer which agrees to this option SHOULD request the Authentication-Protocol Configuration Option. (§2.3) | SHOULD | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-2.3-2` | Authentication SHOULD be requested in turn by the implementation when it is called back, if mutual authentication is desired. (§2.3) | SHOULD | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-2.3-3` | The actual format of the information is site or application specific, and a robust implementation SHOULD support the field as undistinguished octets. (§2.3) | SHOULD | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-2.4.1-2` | The peer SHOULD respond with a Configure-Ack, followed in a compound frame by its LCP Configure-Request, and any NCP Configure-Requests desired. (§2.4.1) | SHOULD | 2.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-2.4.1-3` | Upon receipt, the local implementation SHOULD process the Configure- Ack as usual. (§2.4.1) | SHOULD | 2.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-2.4.1-4` | If the peer also specified the Self-Describing-Padding and Compound-Frames options in its Configure-Request, the local implementation SHOULD retain its Configure-Ack, and further NCP configuration packets SHOULD be added to the return frame. (§2.4.1) | SHOULD | 2.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-1.1-4` | Identification packets MAY be sent at any time, including before LCP has reached the Opened state. (§1.1) | MAY | 1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1570-2.4-4` | When Compound-Frames is successfully negotiated, the sender MAY add additional packets to the same frame. (§2.4) | MAY | 2.4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC1570-1.1-1`](#rfc1570-1.1-1) The Identifier field MUST be changed for each Identification sent. (§1.1) | no test | no test carries this requirement id |
| [`RFC1570-1.1-2`](#rfc1570-1.1-2) Until the Magic-Number Configuration Option has been successfully negotiated, the Magic- Number MUST be transmitted as zero. (§1.1) | no test | no test carries this requirement id |
| [`RFC1570-1.1-3`](#rfc1570-1.1-3) It is intended to be human readable, and MUST NOT affect operation of the protocol. (§1.1) | no test | no test carries this requirement id |
| [`RFC1570-1.2-1`](#rfc1570-1.2-1) The Identifier field MUST be changed for each Time-Remaining sent. (§1.2) | no test | no test carries this requirement id |
| [`RFC1570-1.2-2`](#rfc1570-1.2-2) Until the Magic-Number Configuration Option has been successfully negotiated, the Magic- Number MUST be transmitted as zero. (§1.2) | no test | no test carries this requirement id |
| [`RFC1570-1.2-3`](#rfc1570-1.2-3) It is intended to be human readable, and MUST NOT affect operation of the protocol. (§1.2) | no test | no test carries this requirement id |
| [`RFC1570-2.1-1`](#rfc1570-2.1-1) Frames sent during any other phase MUST contain the default FCS. (§2.1) | no test | no test carries this requirement id |
| [`RFC1570-2.1.1-1`](#rfc1570-2.1.1-1) On receipt of a LCP Configure-Request or Terminate-Request packet, the implementation MUST change to the default FCS for both transmission and reception. (§2.1.1) | no test | no test carries this requirement id |
| [`RFC1570-2.1.1-2`](#rfc1570-2.1.1-2) If a Request packet is received which contains a duplicate Identifier field, a new reply MUST be generated. (§2.1.1) | no test | no test carries this requirement id |
| [`RFC1570-2.1.1-3`](#rfc1570-2.1.1-3) If such a 48-bit FCS is used, it MUST only be used for LCP packets. (§2.1.1) | no test | no test carries this requirement id |
| [`RFC1570-2.1.2-1`](#rfc1570-2.1.2-1) When a configuration (LCP or NCP) or authentication packet is sent, the FCS MUST be included. (§2.1.2) | no test | no test carries this requirement id |
| [`RFC1570-2.1.2-2`](#rfc1570-2.1.2-2) When a configuration (LCP or NCP) or authentication packet is received, the FCS MUST be verified. (§2.1.2) | no test | no test carries this requirement id |
| [`RFC1570-2.1.2-3`](#rfc1570-2.1.2-3) The incorrect FCS MUST be passed with the rest of the data. (§2.1.2) | no test | no test carries this requirement id |
| [`RFC1570-2.1.2-4`](#rfc1570-2.1.2-4) The frame MUST NOT be discarded until after demultiplexing, and only those frames that require the FCS are discarded. (§2.1.2) | no test | no test carries this requirement id |
| [`RFC1570-2.2-1`](#rfc1570-2.2-1) If the option is Rejected, the peer MUST NOT add any padding to the identified special protocols, but MAY add padding to other protocols. (§2.2) | no test | no test carries this requirement id |
| [`RFC1570-2.2-2`](#rfc1570-2.2-2) If the option is Ack'd, the peer MUST follow the procedures for adding self-describing pads, but only to the specifically identified protocols. (§2.2) | no test | no test carries this requirement id |
| [`RFC1570-2.2-3`](#rfc1570-2.2-3) The first pad octet MUST contain the value one (1), which indicates the Padding Protocol to the Compound-Frames option. (§2.2) | no test | no test carries this requirement id |
| [`RFC1570-2.2-4`](#rfc1570-2.2-4) When no padding would otherwise be required, but the final octet of the PPP Information field contains the value 1 through MPV, at least one self-describing pad octet MUST be added to the frame. (§2.2) | no test | no test carries this requirement id |
| [`RFC1570-2.4-1`](#rfc1570-2.4-1) Therefore, this option MUST be negotiated together with the Self- Describing-Padding option. (§2.4) | no test | no test carries this requirement id |
| [`RFC1570-2.4-2`](#rfc1570-2.4-2) If the FCS-Alternatives option has been negotiated, self describing padding MUST always be added. (§2.4) | no test | no test carries this requirement id |
| [`RFC1570-2.4-3`](#rfc1570-2.4-3) That is, the final packet MUST be followed by a series of octets, the first of which contains the value one (1). (§2.4) | no test | no test carries this requirement id |
| [`RFC1570-2.4.1-1`](#rfc1570-2.4.1-1) Since the peer has agreed to send compound frames, the implementation MUST examine the remainder of the frame for additional packets. (§2.4.1) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC1570-1.1-1`](#rfc1570-1.1-1)

The Identifier field MUST be changed for each Identification sent. (§1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1570-1.1-1, so no unit is bound to it.

### [`RFC1570-1.1-2`](#rfc1570-1.1-2)

Until the Magic-Number Configuration Option has been successfully negotiated, the Magic- Number MUST be transmitted as zero. (§1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1570-1.1-2, so no unit is bound to it.

### [`RFC1570-1.1-3`](#rfc1570-1.1-3)

It is intended to be human readable, and MUST NOT affect operation of the protocol. (§1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1570-1.1-3, so no unit is bound to it.

### [`RFC1570-1.2-1`](#rfc1570-1.2-1)

The Identifier field MUST be changed for each Time-Remaining sent. (§1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1570-1.2-1, so no unit is bound to it.

### [`RFC1570-1.2-2`](#rfc1570-1.2-2)

Until the Magic-Number Configuration Option has been successfully negotiated, the Magic- Number MUST be transmitted as zero. (§1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1570-1.2-2, so no unit is bound to it.

### [`RFC1570-1.2-3`](#rfc1570-1.2-3)

It is intended to be human readable, and MUST NOT affect operation of the protocol. (§1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1570-1.2-3, so no unit is bound to it.

### [`RFC1570-2.1-1`](#rfc1570-2.1-1)

Frames sent during any other phase MUST contain the default FCS. (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1570-2.1-1, so no unit is bound to it.

### [`RFC1570-2.1.1-1`](#rfc1570-2.1.1-1)

On receipt of a LCP Configure-Request or Terminate-Request packet, the implementation MUST change to the default FCS for both transmission and reception. (§2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1570-2.1.1-1, so no unit is bound to it.

### [`RFC1570-2.1.1-2`](#rfc1570-2.1.1-2)

If a Request packet is received which contains a duplicate Identifier field, a new reply MUST be generated. (§2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1570-2.1.1-2, so no unit is bound to it.

### [`RFC1570-2.1.1-3`](#rfc1570-2.1.1-3)

If such a 48-bit FCS is used, it MUST only be used for LCP packets. (§2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1570-2.1.1-3, so no unit is bound to it.

### [`RFC1570-2.1.2-1`](#rfc1570-2.1.2-1)

When a configuration (LCP or NCP) or authentication packet is sent, the FCS MUST be included. (§2.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1570-2.1.2-1, so no unit is bound to it.

### [`RFC1570-2.1.2-2`](#rfc1570-2.1.2-2)

When a configuration (LCP or NCP) or authentication packet is received, the FCS MUST be verified. (§2.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1570-2.1.2-2, so no unit is bound to it.

### [`RFC1570-2.1.2-3`](#rfc1570-2.1.2-3)

The incorrect FCS MUST be passed with the rest of the data. (§2.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1570-2.1.2-3, so no unit is bound to it.

### [`RFC1570-2.1.2-4`](#rfc1570-2.1.2-4)

The frame MUST NOT be discarded until after demultiplexing, and only those frames that require the FCS are discarded. (§2.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1570-2.1.2-4, so no unit is bound to it.

### [`RFC1570-2.2-1`](#rfc1570-2.2-1)

If the option is Rejected, the peer MUST NOT add any padding to the identified special protocols, but MAY add padding to other protocols. (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1570-2.2-1, so no unit is bound to it.

### [`RFC1570-2.2-2`](#rfc1570-2.2-2)

If the option is Ack'd, the peer MUST follow the procedures for adding self-describing pads, but only to the specifically identified protocols. (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1570-2.2-2, so no unit is bound to it.

### [`RFC1570-2.2-3`](#rfc1570-2.2-3)

The first pad octet MUST contain the value one (1), which indicates the Padding Protocol to the Compound-Frames option. (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1570-2.2-3, so no unit is bound to it.

### [`RFC1570-2.2-4`](#rfc1570-2.2-4)

When no padding would otherwise be required, but the final octet of the PPP Information field contains the value 1 through MPV, at least one self-describing pad octet MUST be added to the frame. (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1570-2.2-4, so no unit is bound to it.

### [`RFC1570-2.4-1`](#rfc1570-2.4-1)

Therefore, this option MUST be negotiated together with the Self- Describing-Padding option. (§2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1570-2.4-1, so no unit is bound to it.

### [`RFC1570-2.4-2`](#rfc1570-2.4-2)

If the FCS-Alternatives option has been negotiated, self describing padding MUST always be added. (§2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1570-2.4-2, so no unit is bound to it.

### [`RFC1570-2.4-3`](#rfc1570-2.4-3)

That is, the final packet MUST be followed by a series of octets, the first of which contains the value one (1). (§2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1570-2.4-3, so no unit is bound to it.

### [`RFC1570-2.4.1-1`](#rfc1570-2.4.1-1)

Since the peer has agreed to send compound frames, the implementation MUST examine the remainder of the frame for additional packets. (§2.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1570-2.4.1-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc1570.txt |
| Source fingerprint | 36be6506cdb1a944 |
| Record | rfc/extraction/rfc1570.json |
| Mapped sentences | 22 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `1` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `1.1` | not stated | 3 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `1.2` | not stated | 3 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.1` | not stated | 1 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.1.1` | not stated | 3 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.1.2` | not stated | 4 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.2` | not stated | 4 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.3` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.4` | not stated | 3 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.4.1` | not stated | 1 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `A` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `A.1` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |

### Excluded sentences

The walk over RFC 1570 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 1570, so its obligations are stated where they were written.
