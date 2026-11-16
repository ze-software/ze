# RFC 1570 - PPP LCP Extensions

PPP LCP extensions. Every requirement listed for RFC 1570, the tests linked to it, and their recorded reviews. This summary is excluded from automatic compliance checks.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 22 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 22 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 22 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 22 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 0 | 0 of 0 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 22 | of 22 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 22 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 22 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 22 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 22 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 22 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level), not checked | 22 | of 38 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements listed in this summary. This RFC is excluded from automatic checks (out-of-scope), so these requirements are not checked. The counts above show what the summary records. |

The 9 counts marked as a part above are the whole of the 22 mandatory requirements: they add up to 22. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Public row | PPP LCP extensions |
| Automatic checks | Excluded (out-of-scope) |
| Requirements | 38 |
| Mandatory requirements (MUST level), not checked | 22 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 22 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tests linked to requirements | 0 |
| Recorded audit verdicts | 0 |
| Fault test records | 0 |
| Summary | `rfc/short/rfc1570.md` |
| Requirement table | `rfc/requirements/rfc1570.md` |
| RFC text | `rfc/full/rfc1570.txt` |

## Automatic checks

Excluded from automatic checks (out-of-scope, the requirements ARE extracted and the owner decided not to offer the feature for now, so the absence is a scope decision rather than a conformance gap): OUT OF SCOPE as a document by owner decision, 2026-10-26: the owner rule committed in 1e7a9dbc34 (ai/META-DECISIONS.md, Goal and scope) counts an RFC a profile lists as optional only once Thomas brings it into scope, and this summary's `Optional use cases` row lists this RFC in the optional tier of the lns profile. The extraction is COMPLETE: the source text is at rfc/full/rfc1570.txt, every capitalised MUST-level sentence is a row of this summary or an exclusion in rfc/extraction/rfc1570.json (walked 2026-10-26), and none is gated. Ze's PPP implements none of the LCP codes or options this document adds (internal/component/l2tp/ppp/lcp_options.go carries options 1, 2, 3, 5, 7 and 8 only), so an Identification or Time-Remaining packet is answered as an unknown code under RFC 1661.

## Published support status

**Area:** PPP LCP extensions

**Test verification:** 0 of 22 MUST proven

**Evidence:** -

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 22 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level), not checked** | **22** | every mandatory requirement appears in exactly one group above |

**No test and no annotation (22):** [`RFC1570-1.1-1`](#rfc1570-1.1-1), [`RFC1570-1.1-2`](#rfc1570-1.1-2), [`RFC1570-1.1-3`](#rfc1570-1.1-3), [`RFC1570-1.2-1`](#rfc1570-1.2-1), [`RFC1570-1.2-2`](#rfc1570-1.2-2), [`RFC1570-1.2-3`](#rfc1570-1.2-3), [`RFC1570-2.1-1`](#rfc1570-2.1-1), [`RFC1570-2.1.1-1`](#rfc1570-2.1.1-1), [`RFC1570-2.1.1-2`](#rfc1570-2.1.1-2), [`RFC1570-2.1.1-3`](#rfc1570-2.1.1-3), [`RFC1570-2.1.2-1`](#rfc1570-2.1.2-1), [`RFC1570-2.1.2-2`](#rfc1570-2.1.2-2), [`RFC1570-2.1.2-3`](#rfc1570-2.1.2-3), [`RFC1570-2.1.2-4`](#rfc1570-2.1.2-4), [`RFC1570-2.2-1`](#rfc1570-2.2-1), [`RFC1570-2.2-2`](#rfc1570-2.2-2), [`RFC1570-2.2-3`](#rfc1570-2.2-3), [`RFC1570-2.2-4`](#rfc1570-2.2-4), [`RFC1570-2.4-1`](#rfc1570-2.4-1), [`RFC1570-2.4-2`](#rfc1570-2.4-2), [`RFC1570-2.4-3`](#rfc1570-2.4-3), [`RFC1570-2.4.1-1`](#rfc1570-2.4.1-1)

## Figures

- **MUST bar**: Not assessed 22
- **Overview bar**: Not assessed 38

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 22 |
| MUST rows kept (the MUST bar) | 22 |
| Ze verified | 0 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 22 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 16 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 16 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc1570.json](../../../data/rfc/rfc1570.json).

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

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC1570-1.1-1`](#rfc1570-1.1-1)

The Identifier field MUST be changed for each Identification sent. (§1.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1570-1.1-1, so there is no linked test.

### [`RFC1570-1.1-2`](#rfc1570-1.1-2)

Until the Magic-Number Configuration Option has been successfully negotiated, the Magic- Number MUST be transmitted as zero. (§1.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1570-1.1-2, so there is no linked test.

### [`RFC1570-1.1-3`](#rfc1570-1.1-3)

It is intended to be human readable, and MUST NOT affect operation of the protocol. (§1.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1570-1.1-3, so there is no linked test.

### [`RFC1570-1.2-1`](#rfc1570-1.2-1)

The Identifier field MUST be changed for each Time-Remaining sent. (§1.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1570-1.2-1, so there is no linked test.

### [`RFC1570-1.2-2`](#rfc1570-1.2-2)

Until the Magic-Number Configuration Option has been successfully negotiated, the Magic- Number MUST be transmitted as zero. (§1.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1570-1.2-2, so there is no linked test.

### [`RFC1570-1.2-3`](#rfc1570-1.2-3)

It is intended to be human readable, and MUST NOT affect operation of the protocol. (§1.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1570-1.2-3, so there is no linked test.

### [`RFC1570-2.1-1`](#rfc1570-2.1-1)

Frames sent during any other phase MUST contain the default FCS. (§2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1570-2.1-1, so there is no linked test.

### [`RFC1570-2.1.1-1`](#rfc1570-2.1.1-1)

On receipt of a LCP Configure-Request or Terminate-Request packet, the implementation MUST change to the default FCS for both transmission and reception. (§2.1.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1570-2.1.1-1, so there is no linked test.

### [`RFC1570-2.1.1-2`](#rfc1570-2.1.1-2)

If a Request packet is received which contains a duplicate Identifier field, a new reply MUST be generated. (§2.1.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1570-2.1.1-2, so there is no linked test.

### [`RFC1570-2.1.1-3`](#rfc1570-2.1.1-3)

If such a 48-bit FCS is used, it MUST only be used for LCP packets. (§2.1.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1570-2.1.1-3, so there is no linked test.

### [`RFC1570-2.1.2-1`](#rfc1570-2.1.2-1)

When a configuration (LCP or NCP) or authentication packet is sent, the FCS MUST be included. (§2.1.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1570-2.1.2-1, so there is no linked test.

### [`RFC1570-2.1.2-2`](#rfc1570-2.1.2-2)

When a configuration (LCP or NCP) or authentication packet is received, the FCS MUST be verified. (§2.1.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1570-2.1.2-2, so there is no linked test.

### [`RFC1570-2.1.2-3`](#rfc1570-2.1.2-3)

The incorrect FCS MUST be passed with the rest of the data. (§2.1.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1570-2.1.2-3, so there is no linked test.

### [`RFC1570-2.1.2-4`](#rfc1570-2.1.2-4)

The frame MUST NOT be discarded until after demultiplexing, and only those frames that require the FCS are discarded. (§2.1.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1570-2.1.2-4, so there is no linked test.

### [`RFC1570-2.2-1`](#rfc1570-2.2-1)

If the option is Rejected, the peer MUST NOT add any padding to the identified special protocols, but MAY add padding to other protocols. (§2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1570-2.2-1, so there is no linked test.

### [`RFC1570-2.2-2`](#rfc1570-2.2-2)

If the option is Ack'd, the peer MUST follow the procedures for adding self-describing pads, but only to the specifically identified protocols. (§2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1570-2.2-2, so there is no linked test.

### [`RFC1570-2.2-3`](#rfc1570-2.2-3)

The first pad octet MUST contain the value one (1), which indicates the Padding Protocol to the Compound-Frames option. (§2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1570-2.2-3, so there is no linked test.

### [`RFC1570-2.2-4`](#rfc1570-2.2-4)

When no padding would otherwise be required, but the final octet of the PPP Information field contains the value 1 through MPV, at least one self-describing pad octet MUST be added to the frame. (§2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1570-2.2-4, so there is no linked test.

### [`RFC1570-2.4-1`](#rfc1570-2.4-1)

Therefore, this option MUST be negotiated together with the Self- Describing-Padding option. (§2.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1570-2.4-1, so there is no linked test.

### [`RFC1570-2.4-2`](#rfc1570-2.4-2)

If the FCS-Alternatives option has been negotiated, self describing padding MUST always be added. (§2.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1570-2.4-2, so there is no linked test.

### [`RFC1570-2.4-3`](#rfc1570-2.4-3)

That is, the final packet MUST be followed by a series of octets, the first of which contains the value one (1). (§2.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1570-2.4-3, so there is no linked test.

### [`RFC1570-2.4.1-1`](#rfc1570-2.4.1-1)

Since the peer has agreed to send compound frames, the implementation MUST examine the remainder of the frame for additional packets. (§2.4.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1570-2.4.1-1, so there is no linked test.

## Requirement list review

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
| `front` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `1` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `1.1` | not stated | 3 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `1.2` | not stated | 3 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.1` | not stated | 1 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.1.1` | not stated | 3 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.1.2` | not stated | 4 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.2` | not stated | 4 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.3` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.4` | not stated | 3 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.4.1` | not stated | 1 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `A` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `A.1` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |

### Excluded sentences

The review of RFC 1570 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 1570, so its obligations are stated where they were written.
