# RFC 1661 - The Point-to-Point Protocol (PPP)

PPP LCP. Every requirement listed for RFC 1661, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 51 | of 76 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 10 | of 76 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 76 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 76 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 140 | 140 of 216 tests linked to requirements, 0 without a fault test and 7 with outdated fault tests | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 13 | of 76 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Audit verdicts | 64 | of 76 mandatory requirements reviewed | 5 weak, wrong or unimplemented, 37 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 76 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 76 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 2 | of 76 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 76 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 76 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 76 | of 101 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 76 mandatory requirements: they add up to 76. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | PPP LCP |
| Automatic checks | Included |
| Requirements | 101 |
| Mandatory requirements (MUST level) | 76 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 13 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 2 |
| Test tags | 216 |
| Tests linked to requirements | 216 |
| Recorded audit verdicts | 64 |
| Fault test records | 147 |
| Summary | `rfc/short/rfc1661.md` |
| Requirement table | `rfc/requirements/rfc1661.md` |
| RFC text | `rfc/full/rfc1661.txt` |

## Automatic checks

Included in automatic checks: PPP LCP under L2TP and PPPoE. Ze implements framing, option negotiation, authentication phase selection, restart and termination actions, Code-Reject and Protocol-Reject, and Echo keepalive. The checklist records remaining behavior and evidence gaps; the requirement index binds the tagged tests.

## Published support status

**Area:** PPP LCP

**Test verification:** 48 of 76 MUST proven

**Evidence:** interop 4, unit 44

**Unmet:** RFC1661-2-2, RFC1661-3.4-1, RFC1661-3.5-2, RFC1661-3.5-4, RFC1661-3.7-2, RFC1661-4.3-3, RFC1661-4.4-1, RFC1661-5-1, RFC1661-5.3-4, RFC1661-5.7-2, RFC1661-5.9-1, RFC1661-5.9-3, RFC1661-6.5-3


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 51 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 25 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 2 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **76** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (51):** [`RFC1661-2-1`](#rfc1661-2-1), [`RFC1661-5-2`](#rfc1661-5-2), [`RFC1661-6-2`](#rfc1661-6-2), [`RFC1661-3.1-2`](#rfc1661-3.1-2), [`RFC1661-3.5-1`](#rfc1661-3.5-1), [`RFC1661-3.5-3`](#rfc1661-3.5-3), [`RFC1661-3.6-1`](#rfc1661-3.6-1), [`RFC1661-3.6-3`](#rfc1661-3.6-3), [`RFC1661-4.3-1`](#rfc1661-4.3-1), [`RFC1661-5.1-1`](#rfc1661-5.1-1), [`RFC1661-5.1-2`](#rfc1661-5.1-2), [`RFC1661-5.1-3`](#rfc1661-5.1-3), [`RFC1661-5.2-1`](#rfc1661-5.2-1), [`RFC1661-5.2-2`](#rfc1661-5.2-2), [`RFC1661-5.2-3`](#rfc1661-5.2-3), [`RFC1661-5.2-4`](#rfc1661-5.2-4), [`RFC1661-5.3-1`](#rfc1661-5.3-1), [`RFC1661-5.3-2`](#rfc1661-5.3-2), [`RFC1661-5.3-3`](#rfc1661-5.3-3), [`RFC1661-5.3-5`](#rfc1661-5.3-5), [`RFC1661-5.3-6`](#rfc1661-5.3-6), [`RFC1661-5.3-7`](#rfc1661-5.3-7), [`RFC1661-5.3-8`](#rfc1661-5.3-8), [`RFC1661-5.4-1`](#rfc1661-5.4-1), [`RFC1661-5.4-2`](#rfc1661-5.4-2), [`RFC1661-5.4-3`](#rfc1661-5.4-3), [`RFC1661-5.4-4`](#rfc1661-5.4-4), [`RFC1661-5.4-5`](#rfc1661-5.4-5), [`RFC1661-5.5-1`](#rfc1661-5.5-1), [`RFC1661-5.6-1`](#rfc1661-5.6-1), [`RFC1661-5.6-3`](#rfc1661-5.6-3), [`RFC1661-5.7-1`](#rfc1661-5.7-1), [`RFC1661-5.7-4`](#rfc1661-5.7-4), [`RFC1661-5.8-1`](#rfc1661-5.8-1), [`RFC1661-5.8-2`](#rfc1661-5.8-2), [`RFC1661-6.4-1`](#rfc1661-6.4-1), [`RFC1661-6.4-2`](#rfc1661-6.4-2), [`RFC1661-6.4-3`](#rfc1661-6.4-3), [`RFC1661-4.6-1`](#rfc1661-4.6-1), [`RFC1661-4.6-2`](#rfc1661-4.6-2), [`RFC1661-4.6-3`](#rfc1661-4.6-3), [`RFC1661-4.6-4`](#rfc1661-4.6-4), [`RFC1661-4.4-2`](#rfc1661-4.4-2), [`RFC1661-5.5-2`](#rfc1661-5.5-2), [`RFC1661-5.8-4`](#rfc1661-5.8-4), [`RFC1661-5.8-5`](#rfc1661-5.8-5), [`RFC1661-6.1-1`](#rfc1661-6.1-1), [`RFC1661-6.4-5`](#rfc1661-6.4-5), [`RFC1661-6.4-6`](#rfc1661-6.4-6), [`RFC1661-6.4-7`](#rfc1661-6.4-7), [`RFC1661-6.4-8`](#rfc1661-6.4-8)

**Annotated (including scoped evidence) (25):** [`RFC1661-2-2`](#rfc1661-2-2), [`RFC1661-5-1`](#rfc1661-5-1), [`RFC1661-3.1-1`](#rfc1661-3.1-1), [`RFC1661-3.4-1`](#rfc1661-3.4-1), [`RFC1661-3.5-2`](#rfc1661-3.5-2), [`RFC1661-3.5-4`](#rfc1661-3.5-4), [`RFC1661-3.6-2`](#rfc1661-3.6-2), [`RFC1661-3.7-1`](#rfc1661-3.7-1), [`RFC1661-3.7-2`](#rfc1661-3.7-2), [`RFC1661-4.3-2`](#rfc1661-4.3-2), [`RFC1661-4.3-3`](#rfc1661-4.3-3), [`RFC1661-5.3-4`](#rfc1661-5.3-4), [`RFC1661-5.6-2`](#rfc1661-5.6-2), [`RFC1661-5.7-2`](#rfc1661-5.7-2), [`RFC1661-5.7-3`](#rfc1661-5.7-3), [`RFC1661-5.9-1`](#rfc1661-5.9-1), [`RFC1661-5.9-2`](#rfc1661-5.9-2), [`RFC1661-6.2-1`](#rfc1661-6.2-1), [`RFC1661-6.5-1`](#rfc1661-6.5-1), [`RFC1661-6.5-2`](#rfc1661-6.5-2), [`RFC1661-6.5-3`](#rfc1661-6.5-3), [`RFC1661-6.6-1`](#rfc1661-6.6-1), [`RFC1661-6.6-2`](#rfc1661-6.6-2), [`RFC1661-4.4-1`](#rfc1661-4.4-1), [`RFC1661-5.9-3`](#rfc1661-5.9-3)

**Evidence that runs nightly only (2):** [`RFC1661-6.6-1`](#rfc1661-6.6-1), [`RFC1661-6.6-2`](#rfc1661-6.6-2)

## Figures

- **MUST bar**: Ze verified 48, Tests awaiting verification 13, OS testing gap 2, Not built 13
- **Overview bar**: Ze MUST verified 48, SHOULD verified 2, Tests awaiting verification 14, OS testing gap 2, Not assessed 22, Not built 13

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 76 |
| MUST rows kept (the MUST bar) | 76 |
| Ze verified | 48 |
| OS verified | 0 |
| Tests awaiting verification | 13 |
| OS testing gap | 2 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 13 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 25 |
| SHOULD verified | 2 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 1 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 22 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc1661.json](../../../data/rfc/rfc1661.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1661-2-1` | All Protocols MUST be odd; the least significant bit of the least significant octet MUST equal "1".  Also, all Protocols MUST be assigned such that the least significant bit of the most significant octet equals "0".  Frames received which don't comply with these rules MUST be treated as having an unrecognized Protocol. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC1661CompliantProtocolRecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L233). **positive:** `unit/verify` [`TestRFC1661ProtocolWithOddHighOctetTreatedUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L48). **negative:** `unit/verify` [`TestRFC1661NonCompliantProtocolTreatedUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L198). **negative:** `unit/verify` [`TestRFC1661ProtocolWithOddHighOctetTreatedUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L49) |
| `RFC1661-2-2` | The maximum length for the Information field, including Padding, but not including the Protocol field, is termed the Maximum Receive Unit (MRU), which defaults to 1500 octets. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** getFrameBuf in internal/component/l2tp/ppp/session_run.go supplies MaxFrameBufLen bytes, enough for 1500 Information octets and the Protocol field. sendProtocolReject and writeCodeReject clamp to the peer MRU, but sendEchoReply (BuildLCPEchoReply in internal/component/l2tp/ppp/echo.go) echoes the request's Data with no clamp, so a peer whose Echo-Request is longer than its own established MRU draws an Echo-Reply that exceeds it. Disclosed in docs/features/rfc-status.md |
| `RFC1661-5-1` | The Length MUST NOT exceed the MRU of the link. (§5) | MUST NOT | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** WriteLCPPacket in internal/component/l2tp/ppp/lcp.go backfills the packet length from its supplied payload; sendCodeReject in internal/component/l2tp/ppp/session_run.go bounds that payload by the default-sized Information field rather than a smaller negotiatedMRU. Protocol-Reject has a separate peer-MRU clamp. Disclosed in docs/features/rfc-status.md |
| `RFC1661-5-2` | When a packet is received with an invalid Length field, the packet is silently discarded without affecting the automaton. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestIPCPFrameRejectsNonSinglePacket`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/frame_test.go#L188). **positive:** `unit/verify` [`TestRFC1661InvalidLengthPacketSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_packet_length_test.go#L36). **negative:** `unit/verify` [`TestRFC1661ValidLengthWithPaddingIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_packet_length_test.go#L77) |
| `RFC1661-6-1` | If a negotiable Configuration Option is received in a Configure- Request, but with an invalid or unrecognized Length, a Configure- Nak SHOULD be transmitted which includes the desired Configuration Option with an appropriate Length and Data. (§6) | SHOULD | 6 | **positive:** `unit/verify` [`TestRFC1661ClientInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L201). **positive:** `unit/verify` [`TestRFC1661InvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L239). **positive:** `unit/verify` [`TestRFC1661LCPInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L403). **positive:** `unit/verify` [`TestRFC1661LCPWrongLengthMagicIsNakedNotRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L489). **positive:** `unit/verify` [`TestRFC1661ReplyListsEachOptionTypeOnce`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L887). **negative:** `unit/verify` [`TestRFC1661ClientInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L207). **negative:** `unit/verify` [`TestRFC1661InvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L244). **negative:** `unit/verify` [`TestRFC1661LCPInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L410). **negative:** `unit/verify` [`TestRFC1661LCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L550). **negative:** `unit/verify` [`TestRFC1661LCPWrongLengthMagicIsNakedNotRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L493). **negative:** `unit/verify` [`TestRFC1661ReplyListsEachOptionTypeOnce`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L891) |
| `RFC1661-6-2` | When the Data field is indicated by the Length to extend beyond the end of the Information field, the entire packet is silently discarded without affecting the automaton. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC1661ClientRequestPastEndSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L122). **positive:** `unit/verify` [`TestRFC1661LCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L669). **positive:** `unit/verify` [`TestRFC1661LCPTruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L321). **positive:** `unit/verify` [`TestRFC1661NCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L760). **positive:** `unit/verify` [`TestRFC1661TruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L75). **negative:** `unit/verify` [`TestRFC1661ClientRequestPastEndSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L129). **negative:** `unit/verify` [`TestRFC1661LCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L675). **negative:** `unit/verify` [`TestRFC1661LCPTruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L328). **negative:** `unit/verify` [`TestRFC1661NCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L765). **negative:** `unit/verify` [`TestRFC1661TruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L81) |
| `RFC1661-3.1-1` | In order to establish communications over a point-to-point link, each end of the PPP link MUST first send LCP packets to configure and test the data link. (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC1661RunSendsLCPBeforeAnyOtherProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L51). **negative:** no negative test. **{single-polarity}:** run (internal/component/l2tp/ppp/session_run.go:182-197) drives the synthetic Initial->Closed->ReqSent sequence whose scr action puts an LCP Configure-Request on the wire before any other traffic, and the sole branch that skips it is the RFC 2661 Section 4.4.5 proxy-LCP path where the LAC has already run LCP and the LNS policy is `proxy-lcp accept` (under `proxy-lcp renegotiate` the scr runs as for any session), so there is no case in which ze opens a link with LCP packets unsent |
| `RFC1661-3.1-2` | Then, PPP MUST send NCP packets to choose and configure one or more network-layer protocols. (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC1661NCPConfiguresEachFamilySeparately`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L326). **positive:** `unit/verify` [`TestRFC1661RunSendsNCPAfterLCPOpens`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L66). **negative:** `unit/verify` [`TestRFC1661NoNCPPacketsWhenNoNetworkProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L365) |
| `RFC1661-3.4-1` | Any non-LCP packets received during this phase MUST be silently discarded. (Section 3.4) | MUST | 3.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handleFrame in internal/component/l2tp/ppp/session_run.go now discards IPCP/IPv6CP before the current authenticated LCP lifetime is admitted. Bounded phase and lifetime tests exist; complete proof across all protocol types remains open. |
| `RFC1661-3.5-1` | If an implementation desires that the peer authenticate with some specific authentication protocol, then it MUST request the use of that authentication protocol during Link Establishment phase. (Section 3.5) | MUST | 3.5 | **positive:** `unit/verify` [`TestRFC1661AuthProtocolRequestedDuringEstablishment`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L437). **negative:** `unit/verify` [`TestRFC1661NoAuthProtocolWhenNotDesired`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L468) |
| `RFC1661-3.5-2` | An implementation MUST NOT allow the exchange of link quality determination packets to delay authentication indefinitely. (Section 3.5) | MUST NOT | 3.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: internal/component/l2tp/ppp/lcp_options.go::negotiatePeerOption meets it, because it Configure-Rejects the Quality-Protocol option (type 4) as an unknown type, so ze exchanges no link quality determination packet that could delay authentication; no test asserts it |
| `RFC1661-3.5-3` | Advancement from the Authentication phase to the Network-Layer Protocol phase MUST NOT occur until authentication has completed. (Section 3.5) | MUST NOT | 3.5 | **positive:** `unit/verify` [`TestRFC1661NoNetworkPhaseBeforeAuthenticationCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L85). **positive:** `unit/verify` [`TestRFC1661NoNetworkPhaseUntilAuthCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L495). **negative:** `unit/verify` [`TestRFC1661NetworkPhaseRunsAfterAuthCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L527). **negative:** `unit/verify` [`TestRFC1661NoNetworkPhaseBeforeAuthenticationCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L84) |
| `RFC1661-3.5-4` | All other packets received during this phase MUST be silently discarded. (Section 3.5) | MUST | 3.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handleFrame in internal/component/l2tp/ppp/session_run.go now discards IPCP/IPv6CP before authentication completes rather than retaining early frames. Bounded phase and lifetime tests exist; complete proof across all protocol types remains open. |
| `RFC1661-3.6-1` | Once PPP has finished the previous phases, each network-layer protocol (such as IP, IPX, or AppleTalk) MUST be separately configured by the appropriate Network Control Protocol (NCP). (Section 3.6) | MUST | 3.6 | **positive:** `unit/verify` [`TestRFC1661NCPConfiguresEachFamilySeparately`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L330). **negative:** `unit/verify` [`TestRFC1661NCPStatesAreIndependent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L382) |
| `RFC1661-3.6-2` | Any supported network-layer protocol packets received when the corresponding NCP is not in the Opened state MUST be silently discarded. (Section 3.6) | MUST | 3.6 | **positive:** `unit/verify` [`TestDialRenegotiatesLCPInNetworkPhase`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc1661_pppoe_packets_integration_linux_test.go#L29). **positive:** `unit/verify` [`TestRFC1661NetworkLayerPacketDiscardedBeforeNCPOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L401). **negative:** no negative test. **{single-polarity}:** handleFrame (internal/component/l2tp/ppp/session_run.go) dispatches only the three control protocols, and rejectUnsupportedProtocol discards an IPv4 (0x0021) or IPv6 (0x0057) frame whose NCP ze supports in every NCP state, so no state exists in which a network-layer packet is processed in userspace and there is no accepting counterpart to assert |
| `RFC1661-3.6-3` | While LCP is in the Opened state, any protocol packet which is unsupported by the implementation MUST be returned in a Protocol- Reject (described later). (Section 3.6) | MUST | 3.6 | **positive:** `unit/verify` [`TestRFC1661UnsupportedProtocolRejectedInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L46). **negative:** `unit/verify` [`TestRFC1661UnsupportedProtocolNotRejectedBeforeOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L93) |
| `RFC1661-3.7-1` | The receiver of a Terminate-Request SHOULD wait for the peer to disconnect, and MUST NOT disconnect until at least one Restart time has passed after sending a Terminate-Ack. (§3.7) | MUST NOT | 3.7 | **positive:** `unit/verify` [`TestRFC1661TerminateAckHoldsLinkForRestartTime`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L372). **positive:** `unit/verify` [`TestRFC1661TerminateAckSentAndLinkHeld`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L559). **negative:** no negative test. **{single-polarity}:** the Opened+RTR edge (internal/component/l2tp/ppp/ppp_fsm.go:393-394) lands in Stopping and handleLCPPacket emits EventSessionDown only for Closed or Stopped (internal/component/l2tp/ppp/session_run.go), so after sending a Terminate-Ack ze holds the link; there is no early-disconnect branch to assert |
| `RFC1661-3.7-2` | Any non-LCP packets received during this phase MUST be silently discarded. (Section 3.7) | MUST | 3.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handleFrame and performAction in internal/component/l2tp/ppp/session_run.go now gate NCP on the current LCP lifetime and clear admission when that lifetime ends. Bounded lifecycle tests exist; complete requirement-level proof remains open. |
| `RFC1661-4.3-1` | The implementation MUST be prepared to immediately renegotiate the Configuration Options. (Section 4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC1661ClientRenegotiatesLCPDuringAuth`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_renegotiation_test.go#L126). **positive:** `unit/verify` [`TestRFC1661ClientRenegotiatesLCPDuringIPCP`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_renegotiation_test.go#L189). **positive:** `unit/verify` [`TestRFC1661ClientRenegotiatesLCPInNetworkPhase`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_renegotiation_test.go#L197). **positive:** `unit/verify` [`TestRFC1661ConfigureRequestInOpenedRenegotiates`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L216). **positive:** `unit/verify` [`TestRFC1661RenegotiateOnConfigureRequestInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L619). **negative:** `unit/verify` [`TestRFC1661EchoDoesNotRenegotiate`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L652) |
| `RFC1661-4.3-2` | The implementation MUST be prepared to receive a new Configure-Request without network administrator intervention. (Section 4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC1661NewConfigureRequestAcceptedAfterTerminateRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L672). **positive:** `unit/verify` [`TestRFC1661StoppedAfterPeerTerminateTakesNewConfigureRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L257). **negative:** no negative test. **{single-polarity}:** every post-RTR state in LCPDoTransition accepts a fresh RCR+ (internal/component/l2tp/ppp/ppp_fsm.go:295-296, :327-328, :359-360) and no code path consults an administrative flag before doing so, so there is no refusing counterpart to assert |
| `RFC1661-4.3-3` | The implementation MUST stop sending the offending packet type. (Section 4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** codeToEvent and handleLCPPacket in internal/component/l2tp/ppp/session_run.go handle received rejects without retaining an offending-code suppression set. The Opened RXJ+ transition can send an Echo-Reply; it does not prevent later transmission of the rejected packet type. |
| `RFC1661-5.1-1` | An implementation wishing to open a connection MUST transmit a Configure-Request. (Section 5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestRFC1661OpenTransmitsConfigureRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L698). **positive:** `unit/verify` [`TestRFC1661OpenTransmitsConfigureRequestOnWire`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L420). **negative:** `unit/verify` [`TestRFC1661OpenTransmitsConfigureRequestOnWire`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L421). **negative:** `unit/verify` [`TestRFC1661UpWithoutOpenSendsNoConfigureRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L729) |
| `RFC1661-5.1-2` | Upon reception of a Configure-Request, an appropriate reply MUST be transmitted. (Section 5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestRFC1661ConfigureAckEchoesOptionsVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L747). **negative:** `unit/verify` [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L811) |
| `RFC1661-5.1-3` | The Identifier field MUST be changed whenever the contents of the Options field changes, and whenever a valid reply has been received for a previous request. (Section 5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L80). **positive:** `unit/verify` [`TestLCPConfigureRequestIdentifierLifecycle`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L492). **negative:** `unit/verify` [`TestClientLCPNakChangesIdentifierAndDropsStaleReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L146). **negative:** `unit/verify` [`TestLCPConfigureRequestIdentifierLifecycle`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L493) |
| `RFC1661-5.2-1` | If every Configuration Option received in a Configure-Request is recognizable and all values are acceptable, then the implementation MUST transmit a Configure-Ack. (Section 5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestRFC1661ConfigureAckEchoesOptionsVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L751). **negative:** `unit/verify` [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L815). **negative:** `unit/verify` [`TestRFC1877ConfigureAckEchoesAcceptable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_dns_options_test.go#L18) |
| `RFC1661-5.2-2` | The acknowledged Configuration Options MUST NOT be reordered or modified in any way. (§5.2) | MUST NOT | 5.2 | **positive:** `unit/verify` [`TestRFC1661ConfigureAckEchoesOptionsVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L754). **positive:** `unit/verify` [`TestRFC1877ConfigureAckEchoesAcceptable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_dns_options_test.go#L14). **negative:** `unit/verify` [`TestRFC1661ConfigureAckDoesNotReorderOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L780) |
| `RFC1661-5.2-3` | On reception of a Configure-Ack, the Identifier field MUST match that of the last transmitted Configure-Request. (Section 5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L18). **positive:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L399). **negative:** `unit/verify` [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L19). **negative:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L400) |
| `RFC1661-5.2-4` | Additionally, the Configuration Options in a Configure-Ack MUST exactly match those of the last transmitted Configure-Request. (Section 5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L20). **positive:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L401). **negative:** `unit/verify` [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L21). **negative:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L402) |
| `RFC1661-5.3-1` | If every instance of the received Configuration Options is recognizable, but some values are not acceptable, then the implementation MUST transmit a Configure-Nak. (§5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC1661ConfigureNakSuggestsAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L901). **negative:** `unit/verify` [`TestRFC1661NoNakForAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L936) |
| `RFC1661-5.3-2` | Options which have no value fields (boolean options) MUST use the Configure-Reject reply instead. (Section 5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC1661BooleanOptionsUseRejectNotNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L966). **positive:** `unit/verify` [`TestRFC1661WellFormedBooleanDeclinedWithReject`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L92). **negative:** `unit/verify` [`TestRFC1661ValuedOptionUsesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L988). **negative:** `unit/verify` [`TestRFC1661WellFormedBooleanDeclinedWithReject`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L93) |
| `RFC1661-5.3-3` | Each Configuration Option which is allowed only a single instance MUST be modified to a value acceptable to the Configure-Nak sender. (Section 5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC1661ConfigureNakSuggestsAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L905). **positive:** `unit/verify` [`TestRFC1661RepeatedOptionDrawsOneNakEntry`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L345). **negative:** `unit/verify` [`TestRFC1661NoNakForAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L939) |
| `RFC1661-5.3-4` | When a particular type of Configuration Option can be listed more than once with different values, the Configure-Nak MUST include a list of all values for that option which are acceptable to the Configure-Nak sender. (Section 5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: the condition never holds for ze, because RFC 1661 Section 6 lists no Configuration Option that can be listed more than once and ze negotiates only types 1, 2, 3, 5, 7 and 8 (internal/component/l2tp/ppp/lcp_options.go::negotiatePeerOption); no test asserts that no input yields a multi-value Configure-Nak |
| `RFC1661-5.3-5` | Any value fields for the option MUST indicate values acceptable to the Configure-Nak sender. (Section 5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC1661NakValueIsAcceptable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1005). **negative:** `unit/verify` [`TestRFC1661RejectedValueStaysUnacceptable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1027) |
| `RFC1661-5.3-6` | All acceptable Configuration Options are filtered out of the Configure-Nak, but otherwise the Configuration Options from the Configure-Request MUST NOT be reordered. (§5.3) | MUST NOT | 5.3 | **positive:** `unit/verify` [`TestRFC1661NakFiltersAcceptableOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L138). **positive:** `unit/verify` [`TestRFC1661NakPreservesRequestOrder`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1044). **negative:** `unit/verify` [`TestRFC1661NakFiltersAcceptableOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L139). **negative:** `unit/verify` [`TestRFC1661NakOrderFollowsRequestNotAFixedOrder`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1072) |
| `RFC1661-5.3-7` | On reception of a Configure-Nak, the Identifier field MUST match that of the last transmitted Configure-Request. (Section 5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestClientLCPNakChangesIdentifierAndDropsStaleReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L145). **positive:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L403). **negative:** `unit/verify` [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L22). **negative:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L404) |
| `RFC1661-5.3-8` | Since the Nak'd Option has been modified by the peer, the implementation MUST be able to handle an Option length which is different from the original Configure-Request. (Section 5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC1661NakHandlesDifferentOptionLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1093). **negative:** `unit/verify` [`TestRFC1661NakTooShortOptionNotDecoded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1117) |
| `RFC1661-5.4-1` | If some Configuration Options received in a Configure-Request are not recognizable or are not acceptable for negotiation (as configured by a network administrator), then the implementation MUST transmit a Configure-Reject. (Section 5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestRFC1661ClientUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L250). **positive:** `unit/verify` [`TestRFC1661ConfigureRejectForOptionRefusedByConfiguration`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L169). **positive:** `unit/verify` [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L818). **positive:** `unit/verify` [`TestRFC1661LCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L544). **positive:** `unit/verify` [`TestRFC1661NCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L611). **negative:** `unit/verify` [`TestRFC1661ClientAcceptsServerAuthProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L407). **negative:** `unit/verify` [`TestRFC1661ClientUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L255). **negative:** `unit/verify` [`TestRFC1661ConfigureRejectForOptionRefusedByConfiguration`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L170). **negative:** `unit/verify` [`TestRFC1661NCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L616). **negative:** `unit/verify` [`TestRFC1661NoConfigureRejectWhenAllOptionsRecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L849). **negative:** `unit/verify` [`TestRFC1877ConfigureRejectEchoesUnsupportedOnly`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_dns_options_test.go#L70) |
| `RFC1661-5.4-2` | All recognizable and negotiable Configuration Options are filtered out of the Configure-Reject, but otherwise the Configuration Options MUST NOT be reordered or modified in any way. (§5.4) | MUST NOT | 5.4 | **positive:** `unit/verify` [`TestRFC1661ClientRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L286). **positive:** `unit/verify` [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L821). **positive:** `unit/verify` [`TestRFC1661LCPRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L971). **positive:** `unit/verify` [`TestRFC1877ConfigureRejectEchoesUnsupportedOnly`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_dns_options_test.go#L67). **negative:** `unit/verify` [`TestRFC1661ClientRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L298). **negative:** `unit/verify` [`TestRFC1661ConfigureRejectDoesNotReorderOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L867). **negative:** `unit/verify` [`TestRFC1661LCPRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L982) |
| `RFC1661-5.4-3` | On reception of a Configure-Reject, the Identifier field MUST match that of the last transmitted Configure-Request. (Section 5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L76). **positive:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L405). **negative:** `unit/verify` [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L23). **negative:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L406) |
| `RFC1661-5.4-4` | Additionally, the Configuration Options in a Configure-Reject MUST be a subset of those in the last transmitted Configure- Request. (§5.4, erratum 543) | MUST | 5.4 | **positive:** `unit/verify` [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L77). **positive:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L407). **negative:** `unit/verify` [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L24). **negative:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L408) |
| `RFC1661-5.4-5` | Reception of a valid Configure-Reject indicates that when a new Configure-Request is sent, it MUST NOT include any of the Configuration Options listed in the Configure-Reject. (§5.4) | MUST NOT | 5.4 | **positive:** `unit/verify` [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L78). **positive:** `unit/verify` [`TestLCPRejectedOptionsStayRemoved`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L533). **positive:** `unit/verify` [`TestRFC1661ClientDropsRejectedDNSOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_client_dns_test.go#L149). **negative:** `unit/verify` [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L79). **negative:** `unit/verify` [`TestLCPRejectedOptionsStayRemoved`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L534) |
| `RFC1661-5.5-1` | Upon reception of a Terminate-Request, a Terminate-Ack MUST be transmitted. (Section 5.5) | MUST | 5.5 | **positive:** `unit/verify` [`TestRFC1661TerminateAckSentAndLinkHeld`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L564). **negative:** `unit/verify` [`TestRFC1661NoTerminateAckForTerminateAck`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L598). **positive:** `interop/nightly` [`checkPPPCodeReject`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/l2tp/ppp_code_reject.go#L96) |
| `RFC1661-5.6-1` | This MUST be reported back to the sender of the unknown Code by transmitting a Code- Reject. (Section 5.6) | MUST | 5.6 | **positive:** `unit/verify` [`TestRFC1661CodeRejectForUnknownCode`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1141). **negative:** `unit/verify` [`TestRFC1661NoCodeRejectForKnownCode`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1173). **positive:** `interop/nightly` [`checkPPPCodeReject`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/l2tp/ppp_code_reject.go#L82) |
| `RFC1661-5.6-2` | The Identifier field MUST be changed for each Code-Reject sent. (Section 5.6) | MUST | 5.6 | **positive:** `unit/verify` [`TestRFC1661LCPCodeRejectIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_code_reject_test.go#L48). **positive:** `unit/verify` [`TestRFC1661NCPCodeRejectIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_code_reject_test.go#L76). **negative:** no negative test. **positive:** `interop/nightly` [`checkPPPCodeReject`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/l2tp/ppp_code_reject.go#L86). **{single-polarity}:** sendCodeReject (internal/component/l2tp/ppp/session_run.go) advances lcpCodeRejectID, and sendNCPCodeReject (internal/component/l2tp/ppp/ncp.go) advances the family's ipcpCodeRejectID or ipv6cpCodeRejectID, before every packet written, and no input asks ze to hold the Identifier still, so the obligation has no refusing counterpart to assert |
| `RFC1661-5.6-3` | The Rejected-Packet MUST be truncated to comply with the peer's established MRU. (Section 5.6) | MUST | 5.6 | **positive:** `unit/verify` [`TestRFC1661CodeRejectTruncatedToMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_code_reject_test.go#L122). **negative:** `unit/verify` [`TestRFC1661CodeRejectKeptWhenItFits`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_code_reject_test.go#L185). **positive:** `interop/nightly` [`checkPPPCodeReject`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/l2tp/ppp_code_reject.go#L91) |
| `RFC1661-5.7-1` | If the LCP automaton is in the Opened state, then this MUST be reported back to the peer by transmitting a Protocol-Reject. (Section 5.7) | MUST | 5.7 | **positive:** `unit/verify` [`TestRFC1661UnsupportedProtocolRejectedInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L40). **negative:** `unit/verify` [`TestRFC1661UnsupportedProtocolNotRejectedBeforeOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L88) |
| `RFC1661-5.7-2` | Upon reception of a Protocol-Reject, the implementation MUST stop sending packets of the indicated protocol at the earliest opportunity. (Section 5.7) | MUST | 5.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handleLCPPacket in internal/component/l2tp/ppp/session_run.go maps the rejection to RXJ+ but records no rejected-protocol set. The FSM transition does not implement suppression of later packets of that protocol. |
| `RFC1661-5.7-3` | The Identifier field MUST be changed for each Protocol-Reject sent. (Section 5.7) | MUST | 5.7 | **positive:** `unit/verify` [`TestRFC1661ProtocolRejectIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L136). **negative:** no negative test. **{single-polarity}:** sendProtocolReject (internal/component/l2tp/ppp/session_run.go) advances protocolRejectID before every packet it writes, and no input asks ze to hold the Identifier still, so the obligation has no refusing counterpart to assert |
| `RFC1661-5.7-4` | The Rejected-Information MUST be truncated to comply with the peer's established MRU. (Section 5.7) | MUST | 5.7 | **positive:** `unit/verify` [`TestRFC1661ProtocolRejectInformationTruncatedToMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L165). **negative:** `unit/verify` [`TestRFC1661ProtocolRejectInformationKeptWhenItFits`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L200) |
| `RFC1661-5.8-1` | Upon reception of an Echo-Request in the LCP Opened state, an Echo-Reply MUST be transmitted. (Section 5.8) | MUST | 5.8 | **positive:** `unit/verify` [`TestRFC1661EchoReplyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1194). **negative:** `unit/verify` [`TestRFC1661NoEchoOutsideOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1235) |
| `RFC1661-5.8-2` | Echo-Request and Echo-Reply packets MUST only be sent in the LCP Opened state. (Section 5.8) | MUST | 5.8 | **positive:** `unit/verify` [`TestRFC1661EchoReplyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1197). **positive:** `unit/verify` [`TestRFC1661KeepaliveEchoOnlyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L125). **negative:** `unit/verify` [`TestClientLCPEchoBeforeOpenDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_transition_test.go#L115). **negative:** `unit/verify` [`TestRFC1661KeepaliveEchoOnlyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L126). **negative:** `unit/verify` [`TestRFC1661NoEchoOutsideOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1239) |
| `RFC1661-5.9-1` | Discard-Request packets MUST only be sent in the LCP Opened state. (Section 5.9) | MUST | 5.9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: ze meets it by never sending a Discard-Request: LCPDiscardRequest appears only as a constant, in LCPCodeName (internal/component/l2tp/ppp/lcp.go) and in the receive-side codeToEvent mapping (internal/component/l2tp/ppp/session_run.go), and no sender builds one; no test asserts it |
| `RFC1661-5.9-2` | On reception, the receiver MUST silently discard any Discard- Request that it receives. (Section 5.9) | MUST | 5.9 | **positive:** `unit/verify` [`TestRFC1661DiscardRequestSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1264). **negative:** no negative test. **{single-polarity}:** the obligation covers ANY Discard-Request, so no conforming Discard-Request exists that must instead draw a reply, and no input can make the required silence wrong. The discrimination against a receiver that answers nothing at all is carried by TestRFC1661EchoReplyInOpened, which is tagged for the Echo requirements it actually drives (RFC1661-5.8-1, RFC1661-5.8-2), not for this one |
| `RFC1661-6.2-1` | An implementation MUST NOT include multiple Authentication- Protocol Configuration Options in its Configure-Request packets. (Section 6.2) | MUST NOT | 6.2 | **positive:** `unit/verify` [`TestRFC1661SingleAuthProtocolOptionInRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1296). **negative:** no negative test. **{single-polarity}:** LCPOptions.AuthProto is a single uint16 (internal/component/l2tp/ppp/lcp_options.go) and BuildLocalConfigRequest appends the option once (internal/component/l2tp/ppp/lcp_options.go), so no input produces two Authentication-Protocol options and there is no violating case to assert |
| `RFC1661-6.4-1` | If an implementation does transmit a Configure-Request with a Magic-Number Configuration Option, then it MUST NOT respond with a Configure-Reject when it receives a Configure-Request with a Magic-Number Configuration Option. (Section 6.4) | MUST NOT | 6.4 | **positive:** `unit/verify` [`TestRFC1661ZeroMagicNumberRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1374). **negative:** `unit/verify` [`TestRFC1661UnknownOptionRejectedWhileMagicIsNot`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1346) |
| `RFC1661-6.4-2` | If Magic-Number has been successfully negotiated, an implementation MUST transmit these packets with the Magic-Number field set to its negotiated Magic-Number. (Section 6.4) | MUST | 6.4 | **positive:** `unit/verify` [`TestEchoRequestCarriesNegotiatedMagic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L418). **positive:** `unit/verify` [`TestRFC1661EchoReplyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1206). **negative:** `unit/verify` [`TestEchoRequestCarriesNegotiatedMagic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L419). **negative:** `unit/verify` [`TestRFC1661EchoReplyDoesNotMirrorPeerMagic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1413) |
| `RFC1661-6.4-3` | A Magic-Number of zero is illegal and MUST always be Nak'd, if it is not Rejected outright. (Section 6.4) | MUST | 6.4 | **positive:** `unit/verify` [`TestRFC1661ClientNaksZeroMagicInAWellFormedRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_client_negotiation_test.go#L44). **positive:** `unit/verify` [`TestRFC1661ClientNaksZeroMagicWithAValueOfItsOwn`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L455). **positive:** `unit/verify` [`TestRFC1661ZeroMagicNumberRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1366). **negative:** `unit/verify` [`TestRFC1661ClientAcksANonZeroMagic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_client_negotiation_test.go#L82). **negative:** `unit/verify` [`TestRFC1661ClientNaksZeroMagicWithAValueOfItsOwn`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L461). **negative:** `unit/verify` [`TestRFC1661PeerMagicNumberAcked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1327) |
| `RFC1661-6.5-1` | By default, all implementations MUST transmit packets with two octet PPP Protocol fields. (Section 6.5) | MUST | 6.5 | **positive:** `unit/verify` [`TestRFC1661TwoOctetProtocolField`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L254). **negative:** no negative test. **{single-polarity}:** this is a transmit obligation and WriteFrame has no compressed branch at all -- WriteFrame always writes the Protocol with binary.BigEndian.PutUint16 (internal/component/l2tp/ppp/frame.go:81-85), so no configuration or negotiated option produces a single-octet transmit to assert against. The positive test drives both PFC settings to show the option cannot change the encoder; the receive-side refusal of a one-octet Protocol is the separate RFC1661-6.5-3 |
| `RFC1661-6.5-2` | Compressed Protocol fields MUST NOT be transmitted unless this Configuration Option has been negotiated. (Section 6.5) | MUST NOT | 6.5 | **positive:** `unit/verify` [`TestRFC1661TwoOctetProtocolField`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L258). **negative:** no negative test. **{single-polarity}:** WriteFrame (internal/component/l2tp/ppp/frame.go:81-85) has no compressed-Protocol branch, so ze transmits an uncompressed Protocol field whether or not the option is negotiated and there is no compressed-transmit case to contrast |
| `RFC1661-6.5-3` | When negotiated, PPP implementations MUST accept PPP packets with either double-octet or single-octet Protocol fields, and MUST NOT distinguish between them. (§6.5) | MUST | 6.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** whole-stack reception remains unverified. ParseFrame in internal/component/l2tp/ppp/frame.go reads a two-octet Protocol field, but that alone does not establish a failure after applicable directional negotiation and Linux PPP receive normalization. The negotiated kernel boundary needs proof. |
| `RFC1661-6.6-1` | By default, all implementations MUST transmit frames with Address and Control fields appropriate to the link framing. (Section 6.6) | MUST | 6.6 | **positive:** `interop/nightly` [`checkPPPCodeReject`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/l2tp/ppp_code_reject.go#L99). **negative:** no negative test. **{lower-layer}:** Linux PPP channels over L2TP and PPPoE (net/l2tp/l2tp_ppp.c pppol2tp_xmit, drivers/net/ppp/pppoe.c pppoe_xmit), linux:CONFIG_PPPOL2TP; internal/component/l2tp/ppp/session_run.go::writeFrame writes the Protocol field and payload that internal/component/l2tp/ppp/frame.go::WriteFrame builds to the /dev/ppp channel, and the kernel adds the Address and Control octets 0xFF 0x03 to every L2TP frame and none to a PPPoE frame, whose RFC 2516 framing carries none; ze sets no channel flag that changes this, and no VPP path carries ze's PPP sessions. **nightly-only:** every test bound to this requirement runs in the scheduled workflow alone, so these tests are not run as part of the checks before integration |
| `RFC1661-6.6-2` | The Address and Control fields MUST NOT be compressed when sending any LCP packet. (Section 6.6) | MUST NOT | 6.6 | **positive:** `interop/nightly` [`checkPPPCodeReject`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/l2tp/ppp_code_reject.go#L103). **negative:** `interop/nightly` [`checkPPPCodeReject`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/l2tp/ppp_code_reject.go#L105). **{lower-layer}:** Linux PPP channel over L2TP (net/l2tp/l2tp_ppp.c pppol2tp_xmit), linux:CONFIG_PPPOL2TP; internal/component/l2tp/ppp/session_run.go::writeFrame hands every LCP packet to the /dev/ppp channel as Protocol field and payload, and pppol2tp_xmit pushes the uncompressed 0xFF 0x03 header onto every frame whatever ACFC negotiated; ze acks a peer's ACFC (internal/component/l2tp/ppp/lcp_options.go::negotiatePeerOption, refused on PPPoE) but sets no channel compression flag, so no LCP packet leaves compressed, and no VPP path carries ze's PPP sessions. **nightly-only:** every test bound to this requirement runs in the scheduled workflow alone, so these tests are not run as part of the checks before integration |
| `RFC1661-4.6-1` | The Restart timer MUST be configurable (§4.6) | MUST | 4.6 | **positive:** `unit/verify` [`TestConfiguredLCPRestartTimer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_configured_restart_test.go#L38). **negative:** `unit/verify` [`TestConfiguredLCPRestartTimer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_configured_restart_test.go#L39) |
| `RFC1661-4.6-2` | Max-Terminate MUST be configurable (§4.6) | MUST | 4.6 | **positive:** `unit/verify` [`TestConfiguredLCPMaxTerminate`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_configured_restart_test.go#L158). **negative:** `unit/verify` [`TestConfiguredLCPMaxTerminate`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_configured_restart_test.go#L159) |
| `RFC1661-4.6-3` | Max-Configure MUST be configurable (§4.6) | MUST | 4.6 | **positive:** `unit/verify` [`TestConfiguredLCPMaxConfigure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_configured_restart_test.go#L86). **positive:** `unit/verify` [`TestProxyLCPRenegotiationUsesConfiguredRestartBudget`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_proxy_restart_budget_test.go#L20). **negative:** `unit/verify` [`TestConfiguredLCPMaxConfigure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_configured_restart_test.go#L87). **negative:** `unit/verify` [`TestProxyLCPRenegotiationAnswerStopsRestartBudget`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_proxy_restart_budget_test.go#L69) |
| `RFC1661-4.6-4` | Max-Failure MUST be configurable (§4.6) | MUST | 4.6 | **positive:** `unit/verify` [`TestConfiguredLCPMaxFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_configured_restart_test.go#L199). **positive:** `unit/verify` [`TestRFC1661ClientMaxFailureConvertsFurtherNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_max_failure_test.go#L20). **negative:** `unit/verify` [`TestConfiguredLCPMaxFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_configured_restart_test.go#L200). **negative:** `unit/verify` [`TestRFC1661ClientMaxFailureAckRenewsBudget`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_max_failure_test.go#L58) |
| `RFC1661-4.4-1` | In addition to setting the Restart counter, the implementation MUST set the timeout period to the initial value when Restart timer backoff is used. (Section 4.4) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: armRestartTimer in internal/component/l2tp/ppp/session_run.go resets the Restart timer to the configured constant period and applies no backoff, so the condition never holds; no test asserts it |
| `RFC1661-4.4-2` | In addition to zeroing the Restart counter, the implementation MUST set the timeout period to an appropriate value. (Section 4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestConfiguredLCPZeroRestartPeriod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_configured_restart_test.go#L243). **positive:** `unit/verify` [`TestLCPPeerTerminateRestartTimer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_restart_counter_test.go#L135). **negative:** `unit/verify` [`TestConfiguredLCPZeroRestartPeriod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_configured_restart_test.go#L244). **negative:** `unit/verify` [`TestLCPPeerTerminateRestartTimer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_restart_counter_test.go#L136) |
| `RFC1661-5.5-2` | On transmission, the Identifier field MUST be changed whenever the content of the Data field changes, and whenever a valid reply has been received for a previous request. (§5.5) | MUST | 5.5 | **positive:** `unit/verify` [`TestTerminateRequestIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L59). **negative:** `unit/verify` [`TestTerminateRequestIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L60) |
| `RFC1661-5.8-4` | On transmission, the Identifier field MUST be changed whenever the content of the Data field changes, and whenever a valid reply has been received for a previous request. (§5.8) | MUST | 5.8 | **positive:** `unit/verify` [`TestEchoRequestIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L89). **negative:** `unit/verify` [`TestEchoRequestIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L90) |
| `RFC1661-5.8-5` | Until the Magic-Number Configuration Option has been successfully negotiated, the Magic- Number MUST be transmitted as zero. (§5.8) | MUST | 5.8 | **positive:** `unit/verify` [`TestMagicNumberZeroUntilNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L134). **negative:** `unit/verify` [`TestMagicNumberZeroUntilNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L135) |
| `RFC1661-5.9-3` | The Identifier field MUST be changed for each Discard-Request sent. (Section 5.9) | MUST | 5.9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze sends no Discard-Request, so no Identifier is drawn for one; plan/post-release/features/spec-ppp-discard-request-sender.md |
| `RFC1661-6.1-1` | If smaller packets are requested, an implementation MUST still be able to receive the full 1500 octet information field in case link synchronization is lost. (Section 6.1) | MUST | 6.1 | **positive:** `unit/verify` [`TestFullInformationFieldReceived`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L282). **positive:** `unit/verify` [`TestReceiveFullFrameBelowConfiguredMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_proxy_policy_driver_test.go#L124). **negative:** `unit/verify` [`TestReceiveFullFrameBelowConfiguredMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_proxy_policy_driver_test.go#L125) |
| `RFC1661-6.4-5` | Before this Configuration Option is requested, an implementation MUST choose its Magic-Number. (Section 6.4) | MUST | 6.4 | **positive:** `unit/verify` [`TestMagicNumberChosenBeforeRequested`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_choice_test.go#L20). **negative:** `unit/verify` [`TestMagicNumberChosenBeforeRequested`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_choice_test.go#L21) |
| `RFC1661-6.4-6` | If the two Magic-Numbers are equal, then it is possible, but not certain, that the link is looped-back and that this Configure-Request is actually the one last sent.  To determine this, a Configure-Nak MUST be sent specifying a different Magic-Number value. (§6.4) | MUST | 6.4 | **positive:** `unit/verify` [`TestEqualMagicNumberIsNaked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_choice_test.go#L66). **negative:** `unit/verify` [`TestEqualMagicNumberIsNaked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_choice_test.go#L67) |
| `RFC1661-6.4-7` | If the Magic-Number is equal to the one sent in the last Configure-Nak, the possibility of a looped-back link is increased, and a new Magic-Number MUST be chosen. (Section 6.4) | MUST | 6.4 | **positive:** `unit/verify` [`TestClientLCPMagicNakRedrawsAndUsesNegotiatedValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L186). **positive:** `unit/verify` [`TestMagicNumberRedrawnOnNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L192). **positive:** `unit/verify` [`TestRFC1661LoopedNakMagicChoosesNewMagic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_loop_test.go#L79). **negative:** `unit/verify` [`TestClientLCPMagicNakRedrawsAndUsesNegotiatedValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L187). **negative:** `unit/verify` [`TestMagicNumberRedrawnOnNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L193). **negative:** `unit/verify` [`TestRFC1661LoopedNakMagicRefusesRepeatedDraw`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_loop_test.go#L101) |
| `RFC1661-6.4-8` | All received Magic-Number fields MUST be equal to either zero or the peer's unique Magic-Number, depending on whether or not the peer negotiated a Magic-Number. (Section 6.4) | MUST | 6.4 | **positive:** `unit/verify` [`TestReceivedMagicNumberMustBePeers`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L230). **positive:** `unit/verify` [`TestReceivedMagicNumberZeroWhenPeerNegotiatedNone`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L361). **negative:** `unit/verify` [`TestReceivedMagicNumberMustBePeers`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L231). **negative:** `unit/verify` [`TestReceivedMagicNumberZeroWhenPeerNegotiatedNone`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L362) |
| `RFC1661-4.6-5` | The Restart timer MUST be configurable, but SHOULD default to three (3) seconds. (§4.6) | SHOULD | 4.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-4.6-6` | Max-Terminate MUST be configurable, but SHOULD default to two (2) transmissions. (§4.6) | SHOULD | 4.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-4.6-7` | Max-Configure MUST be configurable, but SHOULD default to ten (10) transmissions. (§4.6) | SHOULD | 4.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-4.6-8` | Max-Failure MUST be configurable, but SHOULD default to five (5) transmissions. (§4.6) | SHOULD | 4.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-1.2-1` | The implementation SHOULD provide the capability of logging the error, including the contents of the silently discarded packet, and SHOULD record the event in a statistics counter. (§1.2) | SHOULD | 1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-3.5-5` | Authentication SHOULD take place as soon as possible after link establishment. (§3.5) | SHOULD | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-3.5-6` | If authentication fails, the authenticator SHOULD proceed instead to the Link Termination phase. (§3.5) | SHOULD | 3.5 | **positive:** `unit/verify` [`TestAuthRefusalTerminationCleanup`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_policy_driver_test.go#L148). **negative:** `unit/verify` [`TestAuthRefusalTerminationCleanup`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_policy_driver_test.go#L149) |
| `RFC1661-3.5-7` | An implementation SHOULD NOT fail authentication simply due to timeout or lack of response. (§3.5) | SHOULD NOT | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-3.6-4` | Because an implementation may initially use a significant amount of time for link quality determination, implementations SHOULD avoid fixed timeouts when waiting for their peers to configure a NCP. (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-3.7-3` | After the exchange of Terminate packets, the implementation SHOULD signal the physical-layer to disconnect in order to enforce the termination of the link, particularly in the case of an authentication failure. (§3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-3.7-4` | The sender of the Terminate-Request SHOULD disconnect after receiving a Terminate-Ack, or after the Restart counter expires. (§3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-3.7-5` | The receiver of a Terminate-Request SHOULD wait for the peer to disconnect (§3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-5.1-4` | Configuration Options SHOULD NOT be included with default values. (§5.1) | SHOULD NOT | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-5.6-4` | Upon reception of the Code-Reject of a code which is fundamental to this version of the protocol, the implementation SHOULD report the problem and drop the connection, since it is unlikely that the situation can be rectified automatically. (§5.6) | SHOULD | 5.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-5.7-5` | Protocol-Reject packets received in any state other than the LCP Opened state SHOULD be silently discarded. (§5.7) | SHOULD | 5.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-5.8-3` | Echo-Request and Echo-Reply packets received in any state other than the LCP Opened state SHOULD be silently discarded. (§5.8) | SHOULD | 5.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-6.2-2` | Instead, it SHOULD attempt to configure the most desirable protocol first.  If that protocol is Configure-Nak'd, then the implementation SHOULD attempt the next most desirable protocol in the next Configure-Request. (§6.2) | SHOULD | 6.2 | **positive:** `unit/verify` [`TestAuthAllowSelectionMatrix`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_policy_driver_test.go#L31). **negative:** `unit/verify` [`TestAuthAllowSelectionMatrix`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_policy_driver_test.go#L32) |
| `RFC1661-6.4-4` | It is recommended that the Magic- Number be chosen in the most random manner possible in order to guarantee with very high probability that an implementation will arrive at a unique number. (§6.4) | SHOULD | 6.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-4.2-1` | After the peer fails to respond to Configure-Requests, an implementation MAY wait passively for the peer to send Configure-Requests.  In this case, the This-Layer-Finished action is not used for the TO- event in states Req-Sent, Ack- Rcvd and Ack-Sent.  This option is useful for dedicated circuits, or circuits which have no status signals available, but SHOULD NOT be used for switched circuits. (§4.2) | SHOULD NOT | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-3.5-8` | Authentication SHOULD take place as soon as possible after link establishment.  However, link quality determination MAY occur concurrently. (§3.5) | MAY | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-4.6-9` | Instead of a constant value, the Restart timer MAY begin at an initial small value and increase to the configured final value.  Each successive value less than the final value SHOULD be at least twice the previous value. (§4.6) | MAY | 4.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-5.1-5` | For retransmissions, the Identifier MAY remain unchanged. (§5.1) | MAY | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-5.3-9` | Reception of a valid Configure-Nak indicates that when a new Configure-Request is sent, the Configuration Options MAY be modified as specified in the Configure-Nak. (§5.3) | MAY | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-5.3-10` | Finally, an implementation may be configured to request the negotiation of a specific Configuration Option.  If that option is not listed, then that option MAY be appended to the list of Nak'd Configuration Options, in order to prompt the peer to include that option in its next Configure-Request packet. (§5.3) | MAY | 5.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC1661-2-2`](#rfc1661-2-2) The maximum length for the Information field, including Padding, but not including the Protocol field, is termed the Maximum Receive Unit (MRU), which defaults to 1500 octets. (§2) | {gap}, no test | getFrameBuf in internal/component/l2tp/ppp/session_run.go supplies MaxFrameBufLen bytes, enough for 1500 Information octets and the Protocol field. sendProtocolReject and writeCodeReject clamp to the peer MRU, but sendEchoReply (BuildLCPEchoReply in internal/component/l2tp/ppp/echo.go) echoes the request's Data with no clamp, so a peer whose Echo-Request is longer than its own established MRU draws an Echo-Reply that exceeds it. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-5-1`](#rfc1661-5-1) The Length MUST NOT exceed the MRU of the link. (§5) | {gap}, no test | WriteLCPPacket in internal/component/l2tp/ppp/lcp.go backfills the packet length from its supplied payload; sendCodeReject in internal/component/l2tp/ppp/session_run.go bounds that payload by the default-sized Information field rather than a smaller negotiatedMRU. Protocol-Reject has a separate peer-MRU clamp. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-3.4-1`](#rfc1661-3.4-1) Any non-LCP packets received during this phase MUST be silently discarded. (Section 3.4) | {gap}, no test | handleFrame in internal/component/l2tp/ppp/session_run.go now discards IPCP/IPv6CP before the current authenticated LCP lifetime is admitted. Bounded phase and lifetime tests exist; complete proof across all protocol types remains open. |
| [`RFC1661-3.5-2`](#rfc1661-3.5-2) An implementation MUST NOT allow the exchange of link quality determination packets to delay authentication indefinitely. (Section 3.5) | {gap}, no test | gap in proof: internal/component/l2tp/ppp/lcp_options.go::negotiatePeerOption meets it, because it Configure-Rejects the Quality-Protocol option (type 4) as an unknown type, so ze exchanges no link quality determination packet that could delay authentication; no test asserts it |
| [`RFC1661-3.5-4`](#rfc1661-3.5-4) All other packets received during this phase MUST be silently discarded. (Section 3.5) | {gap}, no test | handleFrame in internal/component/l2tp/ppp/session_run.go now discards IPCP/IPv6CP before authentication completes rather than retaining early frames. Bounded phase and lifetime tests exist; complete proof across all protocol types remains open. |
| [`RFC1661-3.7-2`](#rfc1661-3.7-2) Any non-LCP packets received during this phase MUST be silently discarded. (Section 3.7) | {gap}, no test | handleFrame and performAction in internal/component/l2tp/ppp/session_run.go now gate NCP on the current LCP lifetime and clear admission when that lifetime ends. Bounded lifecycle tests exist; complete requirement-level proof remains open. |
| [`RFC1661-4.3-3`](#rfc1661-4.3-3) The implementation MUST stop sending the offending packet type. (Section 4.3) | {gap}, no test | codeToEvent and handleLCPPacket in internal/component/l2tp/ppp/session_run.go handle received rejects without retaining an offending-code suppression set. The Opened RXJ+ transition can send an Echo-Reply; it does not prevent later transmission of the rejected packet type. |
| [`RFC1661-5.3-4`](#rfc1661-5.3-4) When a particular type of Configuration Option can be listed more than once with different values, the Configure-Nak MUST include a list of all values for that option which are acceptable to the Configure-Nak sender. (Section 5.3) | {gap}, no test | gap in proof: the condition never holds for ze, because RFC 1661 Section 6 lists no Configuration Option that can be listed more than once and ze negotiates only types 1, 2, 3, 5, 7 and 8 (internal/component/l2tp/ppp/lcp_options.go::negotiatePeerOption); no test asserts that no input yields a multi-value Configure-Nak |
| [`RFC1661-5.7-2`](#rfc1661-5.7-2) Upon reception of a Protocol-Reject, the implementation MUST stop sending packets of the indicated protocol at the earliest opportunity. (Section 5.7) | {gap}, no test | handleLCPPacket in internal/component/l2tp/ppp/session_run.go maps the rejection to RXJ+ but records no rejected-protocol set. The FSM transition does not implement suppression of later packets of that protocol. |
| [`RFC1661-5.9-1`](#rfc1661-5.9-1) Discard-Request packets MUST only be sent in the LCP Opened state. (Section 5.9) | {gap}, no test | gap in proof: ze meets it by never sending a Discard-Request: LCPDiscardRequest appears only as a constant, in LCPCodeName (internal/component/l2tp/ppp/lcp.go) and in the receive-side codeToEvent mapping (internal/component/l2tp/ppp/session_run.go), and no sender builds one; no test asserts it |
| [`RFC1661-6.5-3`](#rfc1661-6.5-3) When negotiated, PPP implementations MUST accept PPP packets with either double-octet or single-octet Protocol fields, and MUST NOT distinguish between them. (§6.5) | {gap}, no test | whole-stack reception remains unverified. ParseFrame in internal/component/l2tp/ppp/frame.go reads a two-octet Protocol field, but that alone does not establish a failure after applicable directional negotiation and Linux PPP receive normalization. The negotiated kernel boundary needs proof. |
| [`RFC1661-4.4-1`](#rfc1661-4.4-1) In addition to setting the Restart counter, the implementation MUST set the timeout period to the initial value when Restart timer backoff is used. (Section 4.4) | {gap}, no test | gap in proof: armRestartTimer in internal/component/l2tp/ppp/session_run.go resets the Restart timer to the configured constant period and applies no backoff, so the condition never holds; no test asserts it |
| [`RFC1661-5.9-3`](#rfc1661-5.9-3) The Identifier field MUST be changed for each Discard-Request sent. (Section 5.9) | {gap}, no test | Ze sends no Discard-Request, so no Identifier is drawn for one; plan/post-release/features/spec-ppp-discard-request-sender.md |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC1661-2-1`](#rfc1661-2-1)

All Protocols MUST be odd; the least significant bit of the least significant octet MUST equal "1".  Also, all Protocols MUST be assigned such that the least significant bit of the most significant octet equals "0".  Frames received which don't comply with these rules MUST be treated as having an unrecognized Protocol. (§2)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661CompliantProtocolRecognized, internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661NonCompliantProtocolTreatedUnrecognized moved. RFC 1661 Section 2: "Frames received which don't comply with these rules MUST be treated as having an unrecognized Protocol." Rejudged after 83deb89129, which changed only the doc comment of TestRFC1661CompliantProtocolRecognized ('dropped' became a Protocol-Reject, matching the code). Low-octet rule: TestRFC1661NonCompliantProtocolTreatedUnrecognized sends 0xC020 and asserts no termination, exactly one Protocol-Reject naming 0xC020 and state Opened; the recorded mutant removes the final rejectUnsupportedProtocol return of handleFrame. High-octet rule: TestRFC1661ProtocolWithOddHighOctetTreatedUnrecognized sends 0xC121 and asserts one Protocol-Reject naming it, so no Echo-Reply, and state Opened. Transmit side: the same unit checks both parity bits on LCP, PAP, CHAP, IPCP, IPV6CP, IPv4 and IPv6, and the package's WriteFrame callers name LCP, PAP, CHAP and the NCP protocols; the one variable caller, session_run.go:2228, was judged earlier as writeCodeReject with ProtoLCP or ncpProto and was not re-traced here. TestRFC1661CompliantProtocolRecognized requires the Echo-Reply under 0xC021, so a blanket refusal goes red. Caveat on proof: the recorded break of the parity-constant positive panics rejectUnsupportedProtocol, which reddens the later half of the unit and never falsifies the constant loop; by reading, an even constant reaches t.Fatalf, but no record shows it.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661ProtocolWithOddHighOctetTreatedUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L49) | unit/verify | revert, verified |
| negative | [`TestRFC1661NonCompliantProtocolTreatedUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L198) | unit/verify | mutant, verified |
| positive | [`TestRFC1661ProtocolWithOddHighOctetTreatedUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L48) | unit/verify | revert, verified |
| positive | [`TestRFC1661CompliantProtocolRecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L233) | unit/verify | mutant, verified |

### [`RFC1661-2-2`](#rfc1661-2-2)

The maximum length for the Information field, including Padding, but not including the Protocol field, is termed the Maximum Receive Unit (MRU), which defaults to 1500 octets. (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1661-2-2, so there is no linked test.

### [`RFC1661-5-1`](#rfc1661-5-1)

The Length MUST NOT exceed the MRU of the link. (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1661-5-1, so there is no linked test.

### [`RFC1661-5-2`](#rfc1661-5-2)

When a packet is received with an invalid Length field, the packet is silently discarded without affecting the automaton. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge ppp4 2026-09-30. Row text is the verbatim RFC 1661 Section 5 sentence (rfc/full/rfc1661.txt lines 1582-1584), MUST under D-3, listed in extraction section 5 unsourced-ids. TestRFC1661InvalidLengthPacketSilentlyDiscarded (+): an Opened session with a matching peer Magic, through handleFrame, gets an Echo-Request that would draw a reply, with Length 3 (below the header) and Length 16 (past the 8 octets present): no frame written, still Opened, no lifecycle event; the buffer is otherwise valid, so only the Length decides. TestRFC1661ValidLengthWithPaddingIsAnswered (-): Length 8 plus 4 padding octets is answered (Echo-Reply Id 0x62, still Opened), so an over-eager discard turns it red. TestIPCPFrameRejectsNonSinglePacket (+) proves ParseLCPPacket's errLCPLengthMismatch detection on an IPCP packet; its claim names only the detection. Records: ParseLCPPacket revert, observed red, all three units.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661ValidLengthWithPaddingIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_packet_length_test.go#L77) | unit/verify | revert, verified |
| positive | [`TestIPCPFrameRejectsNonSinglePacket`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/frame_test.go#L188) | unit/verify | revert, verified |
| positive | [`TestRFC1661InvalidLengthPacketSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_packet_length_test.go#L36) | unit/verify | revert, verified |

### [`RFC1661-6-1`](#rfc1661-6-1)

If a negotiable Configuration Option is received in a Configure- Request, but with an invalid or unrecognized Length, a Configure- Nak SHOULD be transmitted which includes the desired Configuration Option with an appropriate Length and Data. (§6)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_option_length_test.go::TestRFC1661InvalidOptionLengthDrawsNak, internal/component/l2tp/ppp/rfc1661_option_length_test.go::TestRFC1661InvalidOptionLengthDrawsNak#2, internal/component/l2tp/ppp/rfc1661_option_length_test.go::TestRFC1661LCPInvalidOptionLengthDrawsNak, internal/component/l2tp/ppp/rfc1661_option_length_test.go::TestRFC1661LCPInvalidOptionLengthDrawsNak#2, internal/component/l2tp/ppp/rfc1661_option_length_test.go::TestRFC1661LCPUnrecognizedTypeOutranksInvalidLength, internal/component/l2tp/ppp/rfc1661_option_length_test.go::TestRFC1661LCPWrongLengthMagicIsNakedNotRejected, internal/component/l2tp/ppp/rfc1661_option_length_test.go::TestRFC1661LCPWrongLengthMagicIsNakedNotRejected#2, internal/component/l2tp/ppp/rfc1661_option_length_test.go::TestRFC1661ReplyListsEachOptionTypeOnce, internal/component/l2tp/ppp/rfc1661_option_length_test.go::TestRFC1661ReplyListsEachOptionTypeOnce#2, internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go::TestRFC1661ClientInvalidOptionLengthDrawsNak, internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go::TestRFC1661ClientInvalidOptionLengthDrawsNak#2 moved. Sentence: a negotiable option received with an invalid or unrecognized Length SHOULD draw a Configure-Nak carrying the desired option with an appropriate Length and Data. (a) Rejecting or ignoring it, or Naking with a bad Length or value: TestRFC1661LCPInvalidOptionLengthDrawsNak (Length 3, 1, 0) goes red on onlyLCPNak, ParseLCPOptions, len(opts[0].Data) != 2 and value != 1500; TestRFC1661LCPWrongLengthMagicIsNakedNotRejected checks Magic at 4 octets, non-zero; TestRFC1661InvalidOptionLengthDrawsNak and TestRFC1661ClientInvalidOptionLengthDrawsNak cover IPCP and the PPPoE client. Negative: valid Length draws an Ack, unrecognized Type draws a Reject.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661InvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L244) | unit/verify | not verified |
| negative | [`TestRFC1661LCPInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L410) | unit/verify | not verified |
| negative | [`TestRFC1661LCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L550) | unit/verify | not verified |
| negative | [`TestRFC1661LCPWrongLengthMagicIsNakedNotRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L493) | unit/verify | not verified |
| negative | [`TestRFC1661ReplyListsEachOptionTypeOnce`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L891) | unit/verify | not verified |
| negative | [`TestRFC1661ClientInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L207) | unit/verify | not verified |
| positive | [`TestRFC1661InvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L239) | unit/verify | not verified |
| positive | [`TestRFC1661LCPInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L403) | unit/verify | not verified |
| positive | [`TestRFC1661LCPWrongLengthMagicIsNakedNotRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L489) | unit/verify | not verified |
| positive | [`TestRFC1661ReplyListsEachOptionTypeOnce`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L887) | unit/verify | not verified |
| positive | [`TestRFC1661ClientInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L201) | unit/verify | not verified |

### [`RFC1661-6-2`](#rfc1661-6-2)

When the Data field is indicated by the Length to extend beyond the end of the Information field, the entire packet is silently discarded without affecting the automaton. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. ACTUAL LIVE SOURCE VERDICT: the final NCP-request a69f41b7 and independent B-approved client8020de7d units now match their frozen proposals in normalized body comparison, and the complete ten-tag census is confirmed. Final PPP carrier raw source pin d433ecb656ff65ffc7fa1a7de943b78e94ab543445dd096596b0cff96d0bdf5c and client carrier2b6b31a3d06cd2f7c5af9636daae782fd83b5f0220ad47a5b9070855f198ad2c are provenance only, not native audit fingerprints. No further source repair is needed for these two proof gaps; this is pending source judgment only and MAIN owns actual validation and any supported audit stamp. Read full RFC1661 section6 and the exact summary row, then every tagged unit in both carriers: PPP TestRFC1661TruncatedOptionSilentlyDiscarded, TestRFC1661LCPTruncatedOptionSilentlyDiscarded, TestRFC1661LCPReplyWithOptionsPastEndDiscarded, TestRFC1661NCPReplyWithOptionsPastEndDiscarded, and PPPoE-client TestRFC1661ClientRequestPastEndSilentlyDiscarded, ten tags total. The requirement binds whole-packet silence and an unaffected automaton when an option extends past Information; valid-contained opposite controls must still respond. Core LCP request tests isolate three malformed layouts in actual ReqSent/AckSent with exact wire/state assertions, and reply tests use correlated request identifiers with unchanged-state/wire assertions and valid controls. A independently judged B's repaired NCPReply: retained strayC1, real matching outstanding-ID truncated Reject, all-wire timeout-only silence, no lifecycle event, same-original-request completion, plus matching complete mandatory-option Reject causing exact ipv4 fatal Down. A independently judged B's NCP-request a69f: all old IPCP/IPv6CP malformed bytes and contained-unacceptable Nak cases remain; exact following Ack Data and actual original-Control family assignment/SessionUp prove AckRcvd survives, while two actual AckSent cases count every frame and require silence before only the original local request Ack completes, with no replacement peer request. The four ac4f source cuts each directly violate a named assertion: AckRcvd reset loses completion, AckSent reset loses completion, hidden CR fails all-frame silence, unconditional discard loses valid-contained Nak. A authored the client strengthening and does not self-judge it: independent B read full HEAD/current/proposal and actual custom-client producer/helpers, approved8020 and officially recorded332 row137. B's verdict covers both retained malformed layouts and two added genuine AckSent sequences: exact two total writes and original CR1/Ack0x0F Data exclude response/state-reset and replacement-request escape. These separate source judgments close both custom-client and NCP state-preservation weaknesses. No compile, runtime, observed mutant RED, proof record or native audit stamp is claimed here. MAIN owes focused named-unit canonical GREEN and behavioral RED for reviewed cuts, then supported rejudge stamp after confirming final formatting remains semantically identical; no whole verifier is requested.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661LCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L675) | unit/verify | not verified |
| negative | [`TestRFC1661LCPTruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L328) | unit/verify | not verified |
| negative | [`TestRFC1661NCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L765) | unit/verify | mutant, verified |
| negative | [`TestRFC1661TruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L81) | unit/verify | mutant, verified |
| negative | [`TestRFC1661ClientRequestPastEndSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L129) | unit/verify | mutant, verified |
| positive | [`TestRFC1661LCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L669) | unit/verify | not verified |
| positive | [`TestRFC1661LCPTruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L321) | unit/verify | not verified |
| positive | [`TestRFC1661NCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L760) | unit/verify | mutant, verified |
| positive | [`TestRFC1661TruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L75) | unit/verify | mutant, verified |
| positive | [`TestRFC1661ClientRequestPastEndSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L122) | unit/verify | mutant, verified |

### [`RFC1661-3.1-1`](#rfc1661-3.1-1)

In order to establish communications over a point-to-point link, each end of the PPP link MUST first send LCP packets to configure and test the data link. (Section 3.1)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go::TestRFC1661RunSendsLCPBeforeAnyOtherProtocol moved. Judge 2026-09-29. TestRFC1661RunSendsLCPBeforeAnyOtherProtocol drives run() from Initial with IPCP enabled (the input pushed toward a violation) and readLifecyclePacket fails on any first frame other than LCP Configure-Request (one frame read, exact protocol/code). Observed red with sendConfigureRequest broken; a run() writing an NCP or auth frame first turns it red by construction. Tag removed from TestRFC1661LCPPacketsSentFirst (performAction only); no orphan record left. single-polarity: positive marker stands.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC1661RunSendsLCPBeforeAnyOtherProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L51) | unit/verify | revert, verified |

### [`RFC1661-3.1-2`](#rfc1661-3.1-2)

Then, PPP MUST send NCP packets to choose and configure one or more network-layer protocols. (Section 3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent CURRENT-Core source rejudgment against RFC1661 Section3.1: after link establishment/authentication PPP sends NCP packets for the network protocols to configure. Complete TestRFC1661RunSendsNCPAfterLCPOpens runs the actual session from LCP negotiation, accepts the genuine no-auth request, then requires an actual IPv4 address request and IPCP Configure-Request via pendingLifecycleNCP. TestRFC1661NCPConfiguresEachFamilySeparately writes IPv4 and IPv6 NCP separately and requires exactly two frames with protocols0x8021/0x8057 and Configure-Request Codes. TestRFC1661NoNCPPacketsWhenNoNetworkProtocol exercises the distinct no-selected-protocol boundary and requires zero frames with successful NCP phase return. Read afterLCPOpen/runNCPPhase/sendNCPConfigureRequest: configured family enablement, not an unconditional writer, causes packets. The negative is the meaningful complement where no protocol is selected, not a malformed peer input. Source judgment only; existing/new native breaks are not certified here. Rejudged independently after full old/current tagged-unit and B private lint-proposal reads. The address-taking caller edits and immediate helper-local copies preserve every old input, wire assertion, state transition, timeout and cleanup join; the changed helpers still construct genuine Driver/request/Attempt capabilities. All tagged units were included, including inline tags. Producer behavior for this requirement is unchanged by the scoped lint packet. This pending source verdict claims neither a fresh compiler/runtime result nor a native discrimination observation; MAIN owns final source stamping and recorded cuts.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661NoNCPPacketsWhenNoNetworkProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L365) | unit/verify | mutant, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC1661RunSendsNCPAfterLCPOpens`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L66) | unit/verify | mutant, verified |
| positive | [`TestRFC1661NCPConfiguresEachFamilySeparately`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L326) | unit/verify | revert, verified |

### [`RFC1661-3.4-1`](#rfc1661-3.4-1)

Any non-LCP packets received during this phase MUST be silently discarded. (Section 3.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1661-3.4-1, so there is no linked test.

### [`RFC1661-3.5-1`](#rfc1661-3.5-1)

If an implementation desires that the peer authenticate with some specific authentication protocol, then it MUST request the use of that authentication protocol during Link Establishment phase. (Section 3.5)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661AuthProtocolRequestedDuringEstablishment, internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661NoAuthProtocolWhenNotDesired moved. Forbidden: desired auth protocol not requested in the Link Establishment Configure-Request. TestRFC1661AuthProtocolRequestedDuringEstablishment goes red on a missing option (!found) or a wrong protocol (!= 0xC223) in the LCP Configure-Request sendConfigureRequest writes; the negative TestRFC1661NoAuthProtocolWhenNotDesired goes red if the option is sent for AuthMethodNone.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661NoAuthProtocolWhenNotDesired`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L468) | unit/verify | not verified |
| positive | [`TestRFC1661AuthProtocolRequestedDuringEstablishment`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L437) | unit/verify | not verified |

### [`RFC1661-3.5-2`](#rfc1661-3.5-2)

An implementation MUST NOT allow the exchange of link quality determination packets to delay authentication indefinitely. (Section 3.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1661-3.5-2, so there is no linked test.

### [`RFC1661-3.5-3`](#rfc1661-3.5-3)

Advancement from the Authentication phase to the Network-Layer Protocol phase MUST NOT occur until authentication has completed. (Section 3.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent CURRENT-Core source rejudgment against RFC1661 Section3.5: advancement to Network-Layer Protocol phase must wait for completed authentication. Complete TestRFC1661NoNetworkPhaseBeforeAuthenticationCompletes holds a real CHAP exchange with IPCP enabled: after durable one-second pending observation no address request exists, rejected exchange still has none, while an accepted Response leads to a genuine address request and IPCP Configure-Request. Distinct direct afterLCPOpen tests use actual emitted no-auth requests: rejection requires false return and zero MTU/admin-up/MRU operations; acceptance requires exactly one relevant interface operation. Read afterLCPOpen and runAuthPhase/runNCPPhase: auth returns before NCP and interface activation, with failed/canceled requests returning false. Pending CHAP input is structurally valid and isolated, so silence is authentication ordering rather than invalid frame rejection; positive acceptance proves the network path live. Current immutable request replies preserve these causal gates. Fresh records remain owed. Rejudged independently after full old/current tagged-unit and B private lint-proposal reads. The address-taking caller edits and immediate helper-local copies preserve every old input, wire assertion, state transition, timeout and cleanup join; the changed helpers still construct genuine Driver/request/Attempt capabilities. All tagged units were included, including inline tags. Producer behavior for this requirement is unchanged by the scoped lint packet. This pending source verdict claims neither a fresh compiler/runtime result nor a native discrimination observation; MAIN owns final source stamping and recorded cuts.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661NoNetworkPhaseBeforeAuthenticationCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L84) | unit/verify | revert, verified |
| negative | [`TestRFC1661NetworkPhaseRunsAfterAuthCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L527) | unit/verify | mutant, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC1661NoNetworkPhaseBeforeAuthenticationCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L85) | unit/verify | mutant, verified |
| positive | [`TestRFC1661NoNetworkPhaseUntilAuthCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L495) | unit/verify | mutant, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`RFC1661-3.5-4`](#rfc1661-3.5-4)

All other packets received during this phase MUST be silently discarded. (Section 3.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1661-3.5-4, so there is no linked test.

### [`RFC1661-3.6-1`](#rfc1661-3.6-1)

Once PPP has finished the previous phases, each network-layer protocol (such as IP, IPX, or AppleTalk) MUST be separately configured by the appropriate Network Control Protocol (NCP). (Section 3.6)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661NCPConfiguresEachFamilySeparately, internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661NCPStatesAreIndependent moved. Forbidden (a): a family configured by the wrong NCP -- TestRFC1661NCPConfiguresEachFamilySeparately goes red on frames[0].Proto != 0x8021 or frames[1].Proto != 0x8057; (b) NCPs not separate -- TestRFC1661NCPStatesAreIndependent goes red when opening IPCP moves IPv6CP off Initial.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661NCPStatesAreIndependent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L382) | unit/verify | not verified |
| positive | [`TestRFC1661NCPConfiguresEachFamilySeparately`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L330) | unit/verify | not verified |

### [`RFC1661-3.6-2`](#rfc1661-3.6-2)

Any supported network-layer protocol packets received when the corresponding NCP is not in the Opened state MUST be silently discarded. (Section 3.6)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661NetworkLayerPacketDiscardedBeforeNCPOpened moved. Rejudge 2026-11-09: 65044f40ad added only the //ze:guest-root directive to TestDialRenegotiatesLCPInNetworkPhase's doc comment, which selects the uid the guest observation runs under; the body, assertions and inputs are byte-identical to the 2026-11-08 judgement. That judgement stands: client-role positive over a real PPPoE session through the kernel PPP channel, operator config through plugin delivery: IPv4 in both directions is delivered before renegotiation and after IPCP reopens, and is NOT delivered while LCP, PAP and IPCP renegotiate with each Ack held back; the inbound drop window fails on any delivered datagram, and a late one also fails the next stage's payload match. Producer: connected_session.go/ipcp_independent.go set NPModeDrop on restart; NPModePass there turns the inbound DROP checks red. {single-polarity} stands. TestRFC1661NetworkLayerPacketDiscardedBeforeNCPOpened (ppp) unchanged. Client IPv6 while IPv6CP is not Opened is not covered by any tag of this row (other units exercise setIPv6Mode, untagged).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestDialRenegotiatesLCPInNetworkPhase`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc1661_pppoe_packets_integration_linux_test.go#L29) | unit/verify | mutant, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC1661NetworkLayerPacketDiscardedBeforeNCPOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L401) | unit/verify | mutant, verified |

### [`RFC1661-3.6-3`](#rfc1661-3.6-3)

While LCP is in the Opened state, any protocol packet which is unsupported by the implementation MUST be returned in a Protocol- Reject (described later). (Section 3.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 1661 Section 3.6: "While LCP is in the Opened state, any protocol packet which is unsupported by the implementation MUST be returned in a Protocol-Reject (described later)." Rejudged statically at HEAD; the narrowed claim drops supportsProtocol, which the body never exercised. TestRFC1661UnsupportedProtocolRejectedInOpened sends MPLSCP in Opened and asserts exactly one frame, ProtoLCP, code 8, Rejected-Protocol, the Information field returned verbatim, and state Opened; the mutant at the handleFrame fall-through goes red. TestRFC1661UnsupportedProtocolNotRejectedBeforeOpened asserts zero frames in ReqSent, AckSent and Stopped; removing the Opened gate goes red by reading, though its recorded break is a producer panic that shows reachability only. One unsupported value is sufficient because handleFrame's fall-through handles every unnamed protocol. Disabled-NCP rejection added by 926f01f732 is proved by ncp_disabled_test.go, which this row does not cite.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661UnsupportedProtocolNotRejectedBeforeOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L93) | unit/verify | revert, verified |
| positive | [`TestRFC1661UnsupportedProtocolRejectedInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L46) | unit/verify | mutant, verified |

### [`RFC1661-3.7-1`](#rfc1661-3.7-1)

The receiver of a Terminate-Request SHOULD wait for the peer to disconnect, and MUST NOT disconnect until at least one Restart time has passed after sending a Terminate-Ack. (§3.7)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661TerminateAckSentAndLinkHeld moved. Single-polarity accepted: no early-disconnect branch exists after sca in Opened+RTR. TestRFC1661TerminateAckSentAndLinkHeld pins the instant of receipt; TestRFC1661TerminateAckHoldsLinkForRestartTime adds the time bound: after the Terminate-Ack the Restart timer is running (armed by zrc via armRestartTimer, which uses defaultRestartTimer, asserted >= 3 s), and a Configure-Request and an Echo-Request meanwhile keep Stopping with no end and no EventSessionDown. Residual: the armed duration is asserted through the constant, not read off the timer.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC1661TerminateAckHoldsLinkForRestartTime`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L372) | unit/verify | revert, verified |
| positive | [`TestRFC1661TerminateAckSentAndLinkHeld`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L559) | unit/verify | mutant, verified |

### [`RFC1661-3.7-2`](#rfc1661-3.7-2)

Any non-LCP packets received during this phase MUST be silently discarded. (Section 3.7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1661-3.7-2, so there is no linked test.

### [`RFC1661-4.3-1`](#rfc1661-4.3-1)

The implementation MUST be prepared to immediately renegotiate the Configuration Options. (Section 4.3)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/l2tp/pppoeclient/connected_session.go::connectedSession.restart moved. Re-read Section 4.3's immediate-renegotiation sentence, the Section 4.1 Opened RCR+/RCR- table, and all six tagged units. The server tests require exact tld/scr/sca or tld/scr/scn actions, Ack-Sent for an acceptable request, actual Configure-Request and Ack output, and no session-down event; the distinct Echo control retains Opened without scr or tld. Client Auth, IPCP and Network probes require their own Configure-Request immediately before an exact peer Ack with Identifier 0x73 and Magic 0x55667788; Auth additionally requires the second CHAP Response. The request is structurally valid, so malformed-option discard cannot satisfy these assertions. The changed network fixture and shared exchange helper now explicitly supply Max-Failure five, preserving their prior packets and assertions. Current mutant records address phase dispatch; producer-disable records establish reachability only. The semantic assertions and current reentry/phase producer chain, not the record routes alone, ground this judgment.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661EchoDoesNotRenegotiate`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L652) | unit/verify | revert, verified |
| positive | [`TestRFC1661ConfigureRequestInOpenedRenegotiates`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L216) | unit/verify | mutant, verified |
| positive | [`TestRFC1661RenegotiateOnConfigureRequestInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L619) | unit/verify | revert, verified |
| positive | [`TestRFC1661ClientRenegotiatesLCPDuringAuth`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_renegotiation_test.go#L126) | unit/verify | mutant, verified |
| positive | [`TestRFC1661ClientRenegotiatesLCPDuringIPCP`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_renegotiation_test.go#L189) | unit/verify | mutant, verified |
| positive | [`TestRFC1661ClientRenegotiatesLCPInNetworkPhase`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_renegotiation_test.go#L197) | unit/verify | mutant, verified |

### [`RFC1661-4.3-2`](#rfc1661-4.3-2)

The implementation MUST be prepared to receive a new Configure-Request without network administrator intervention. (Section 4.3)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661NewConfigureRequestAcceptedAfterTerminateRequest moved. Judge 2026-09-29 (R16). RFC 1661 4.3 RTR note plus 4.4 tlf read. TestRFC1661StoppedAfterPeerTerminateTakesNewConfigureRequest: Opened + Terminate-Request, Timeout -> run not ended, Stopped, exactly one EventSessionDown (User Request, the tlf signal), then a Configure-Request with no Open event draws ze's CONFREQ and an Ack echoing Id 0x32, Ack-Sent, no end, no second down. Red if Stopped ends the session (sessionEndedAt) or if tlf is not signalled (recorded against signalLayerFinished). Producer checked: signalLayerFinished runs only on a Stopped sessionEndedAt kept; sendEvent's at-most-once drop cannot swallow a legitimate down because every other emitter (fail, auth, NCP, run exit paths, ncp.go) ends the session and was the first down; after tlf a renegotiation ends at Opened (TestRFC1661StoppedAfterPeerTerminateEndsOnOpened) or at a later Stopped/Closed, so the wait is bounded by the CDN/PADT teardown ze started (L2TP teardownSession -> kernel worker closes the PPPoX socket; PPPoE handleSessionDown closes the transport). Valid single-polarity: no refusing counterpart; marker still true.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC1661StoppedAfterPeerTerminateTakesNewConfigureRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L257) | unit/verify | revert, verified |
| positive | [`TestRFC1661NewConfigureRequestAcceptedAfterTerminateRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L672) | unit/verify | revert, verified |

### [`RFC1661-4.3-3`](#rfc1661-4.3-3)

The implementation MUST stop sending the offending packet type. (Section 4.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1661-4.3-3, so there is no linked test.

### [`RFC1661-5.1-1`](#rfc1661-5.1-1)

An implementation wishing to open a connection MUST transmit a Configure-Request. (Section 5.1)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661OpenTransmitsConfigureRequest, internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661UpWithoutOpenSendsNoConfigureRequest moved. TestRFC1661OpenTransmitsConfigureRequestOnWire runs applyTransition (the same path run() uses for its synthetic Open) for Closed+Open and Starting+Up: red unless exactly one LCP Configure-Request reaches the wire and state is Req-Sent. Initial+Up (no Open) writes nothing and lands in Closed. The run() entry itself is RFC1661-3.1-1's, still open.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661OpenTransmitsConfigureRequestOnWire`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L421) | unit/verify | revert, verified |
| negative | [`TestRFC1661UpWithoutOpenSendsNoConfigureRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L729) | unit/verify | revert, verified |
| positive | [`TestRFC1661OpenTransmitsConfigureRequestOnWire`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L420) | unit/verify | revert, verified |
| positive | [`TestRFC1661OpenTransmitsConfigureRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L698) | unit/verify | revert, verified |

### [`RFC1661-5.1-2`](#rfc1661-5.1-2)

Upon reception of a Configure-Request, an appropriate reply MUST be transmitted. (Section 5.1)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661ConfigureAckEchoesOptionsVerbatim, internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661ConfigureRejectForUnrecognizedOption moved. Forbidden: no reply, or the wrong reply, to a Configure-Request. TestRFC1661ConfigureAckEchoesOptionsVerbatim goes red when no Configure-Ack (Identifier 0x21) is written for an acceptable request; TestRFC1661ConfigureRejectForUnrecognizedOption goes red on an Ack or a missing Configure-Reject for an unrecognized option. Driven in ReqSent through handleLCPPacket.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L811) | unit/verify | not verified |
| positive | [`TestRFC1661ConfigureAckEchoesOptionsVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L747) | unit/verify | not verified |

### [`RFC1661-5.1-3`](#rfc1661-5.1-3)

The Identifier field MUST be changed whenever the contents of the Options field changes, and whenever a valid reply has been received for a previous request. (Section 5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read Section 5.1 and all three distinct tagged units. TestLCPConfigureRequestIdentifierLifecycle separately requires a new Identifier after changed options and after a valid Ack even when the next options are unchanged. Its timeout retry preserves the outstanding bytes and Identifier, a permitted retransmission control, and a stale Ack leaves the current request unchanged. Client matching Nak and MRU/Magic/all-option Reject probes require replacement Identifiers to differ, refuse the old Ack, and complete on the replacement Ack. Their well-formed replies isolate reply processing rather than a parse failure. The client change is an explicit valid Max-Failure-five fixture with all packet and Identifier assertions retained. The current server request writer and client negotiator implement both Identifier-change conditions. The client revert records prove failure when the producer is disabled, not a targeted Identifier mutation; the exact Identifier assertions were independently read.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestLCPConfigureRequestIdentifierLifecycle`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L493) | unit/verify | not verified |
| negative | [`TestClientLCPNakChangesIdentifierAndDropsStaleReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L146) | unit/verify | revert, verified |
| positive | [`TestLCPConfigureRequestIdentifierLifecycle`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L492) | unit/verify | not verified |
| positive | [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L80) | unit/verify | revert, verified |

### [`RFC1661-5.2-1`](#rfc1661-5.2-1)

If every Configuration Option received in a Configure-Request is recognizable and all values are acceptable, then the implementation MUST transmit a Configure-Ack. (Section 5.2)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661ConfigureAckEchoesOptionsVerbatim, internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661ConfigureRejectForUnrecognizedOption moved. Forbidden: not Acking a fully acceptable request. TestRFC1661ConfigureAckEchoesOptionsVerbatim goes red on no Configure-Ack; TestRFC1877ConfigureAckEchoesAcceptable goes red when an acceptable IPCP request draws no Ack echoing it. Negatives: an unrecognized option (no Ack) and an unacceptable IP-Address (Nak).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1877ConfigureAckEchoesAcceptable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_dns_options_test.go#L18) | unit/verify | revert, verified |
| negative | [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L815) | unit/verify | not verified |
| positive | [`TestRFC1661ConfigureAckEchoesOptionsVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L751) | unit/verify | not verified |

### [`RFC1661-5.2-2`](#rfc1661-5.2-2)

The acknowledged Configuration Options MUST NOT be reordered or modified in any way. (§5.2)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661ConfigureAckDoesNotReorderOptions, internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661ConfigureAckEchoesOptionsVerbatim moved. Sentence: the acknowledged Configuration Options MUST NOT be reordered or modified in any way. (a) Reordering: TestRFC1661ConfigureAckDoesNotReorderOptions sends MRU,Magic and Magic,MRU and bytes.Equal(pkt.Data, data) goes red on a normalized order. (b) Modifying: TestRFC1661ConfigureAckEchoesOptionsVerbatim bytes.Equal(pkt.Data, data) goes red on any changed value or length; TestRFC1877ConfigureAckEchoesAcceptable carries the same check for IPCP.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661ConfigureAckDoesNotReorderOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L780) | unit/verify | not verified |
| positive | [`TestRFC1877ConfigureAckEchoesAcceptable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_dns_options_test.go#L14) | unit/verify | mutant, verified |
| positive | [`TestRFC1661ConfigureAckEchoesOptionsVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L754) | unit/verify | not verified |

### [`RFC1661-5.2-3`](#rfc1661-5.2-3)

On reception of a Configure-Ack, the Identifier field MUST match that of the last transmitted Configure-Request. (Section 5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read Section 5.2 and both server/client correlation units. An otherwise exact Ack with only its Identifier changed leaves the server FSM/wire and client completion/output unchanged; the matching exact Ack advances the server to Ack-Rcvd and completes the client whose peer direction was already acknowledged. Those positive controls reject unconditional silence. The wrong-Identifier input has the correct original options, isolating this clause from the exact-options rule. The client now explicitly supplies Max-Failure five without changing a packet or assertion. ValidateLCPReply compares Identifier before either receiving path can mutate negotiation. Current client producer-disable records establish reachability; the independently read wrong/matching Identifier assertions supply the semantic distinction.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L400) | unit/verify | not verified |
| negative | [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L19) | unit/verify | revert, verified |
| positive | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L399) | unit/verify | not verified |
| positive | [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L18) | unit/verify | revert, verified |

### [`RFC1661-5.2-4`](#rfc1661-5.2-4)

Additionally, the Configuration Options in a Configure-Ack MUST exactly match those of the last transmitted Configure-Request. (Section 5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read Section 5.2 and both correlation units. With the outstanding Identifier retained, missing, modified and reordered Ack options cannot advance either negotiation; the server also rejects an extra option. Each buffer remains an LCP Ack and the changed/reordered cases have valid option structure, so an unrelated malformed-packet branch is not the proof. Exact original options subsequently enter Ack-Rcvd on the server and open the client, providing a distinct acceptance control. Client Max-Failure-five fixture initialization changes no expected bytes or outcome. The current shared validator uses byte equality for the entire Ack options field before either receiver changes state. Producer-disable records are credited for reachability only, with the exact options assertions independently judged.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L402) | unit/verify | not verified |
| negative | [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L21) | unit/verify | revert, verified |
| positive | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L401) | unit/verify | not verified |
| positive | [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L20) | unit/verify | revert, verified |

### [`RFC1661-5.3-1`](#rfc1661-5.3-1)

If every instance of the received Configuration Options is recognizable, but some values are not acceptable, then the implementation MUST transmit a Configure-Nak. (§5.3)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661ConfigureNakSuggestsAcceptableValue, internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661NoNakForAcceptableValue moved. Sentence: if every instance of the received options is recognizable but some values are not acceptable, the implementation MUST transmit a Configure-Nak. (a) Answering an unacceptable MRU 2000 with Ack, Reject or nothing: TestRFC1661ConfigureNakSuggestsAcceptableValue findCode(LCPConfigureNak) fails. (b) Nak where all values are acceptable: TestRFC1661NoNakForAcceptableValue findCode(LCPConfigureNak) ok goes red.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661NoNakForAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L936) | unit/verify | not verified |
| positive | [`TestRFC1661ConfigureNakSuggestsAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L901) | unit/verify | not verified |

### [`RFC1661-5.3-2`](#rfc1661-5.3-2)

Options which have no value fields (boolean options) MUST use the Configure-Reject reply instead. (Section 5.3)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661BooleanOptionsUseRejectNotNak, internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661ValuedOptionUsesNak moved. TestRFC1661WellFormedBooleanDeclinedWithReject removes the length-fault confound: a well-formed Length-2 ACFC declined on PPPoE draws a Configure-Reject whose options are exactly that ACFC and no Nak; beside a Nak-earning MRU, NegotiatePeerOptions puts the ACFC in rejects not naks and the wire reply is the Reject naming only ACFC. Red on a Nak for the boolean.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661WellFormedBooleanDeclinedWithReject`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L93) | unit/verify | revert, verified |
| negative | [`TestRFC1661ValuedOptionUsesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L988) | unit/verify | revert, verified |
| positive | [`TestRFC1661WellFormedBooleanDeclinedWithReject`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L92) | unit/verify | revert, verified |
| positive | [`TestRFC1661BooleanOptionsUseRejectNotNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L966) | unit/verify | revert, verified |

### [`RFC1661-5.3-3`](#rfc1661-5.3-3)

Each Configuration Option which is allowed only a single instance MUST be modified to a value acceptable to the Configure-Nak sender. (Section 5.3)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661ConfigureNakSuggestsAcceptableValue, internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661NoNakForAcceptableValue moved. Forbidden: a Nak that repeats an unacceptable single-instance value. TestRFC1661ConfigureNakSuggestsAcceptableValue goes red unless the Nak carries one MRU at 1500 for a requested 2000; TestRFC1661RepeatedOptionDrawsOneNakEntry goes red on more than one Magic entry; TestRFC1661NoNakForAcceptableValue goes red if an acceptable MRU is Nak'd or modified.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661NoNakForAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L939) | unit/verify | not verified |
| positive | [`TestRFC1661RepeatedOptionDrawsOneNakEntry`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L345) | unit/verify | not verified |
| positive | [`TestRFC1661ConfigureNakSuggestsAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L905) | unit/verify | not verified |

### [`RFC1661-5.3-4`](#rfc1661-5.3-4)

When a particular type of Configuration Option can be listed more than once with different values, the Configure-Nak MUST include a list of all values for that option which are acceptable to the Configure-Nak sender. (Section 5.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1661-5.3-4, so there is no linked test.

### [`RFC1661-5.3-5`](#rfc1661-5.3-5)

Any value fields for the option MUST indicate values acceptable to the Configure-Nak sender. (Section 5.3)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661NakValueIsAcceptable, internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661RejectedValueStaysUnacceptable moved. Forbidden: a Nak value the Nak sender would not accept. TestRFC1661NakValueIsAcceptable re-offers the Nak'd MRU (from 2000 and 32) and goes red unless it draws ack=1 nak=0 rej=0; TestRFC1661RejectedValueStaysUnacceptable goes red if the original 2000 is accepted. Only the MRU option is exercised.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661RejectedValueStaysUnacceptable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1027) | unit/verify | not verified |
| positive | [`TestRFC1661NakValueIsAcceptable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1005) | unit/verify | not verified |

### [`RFC1661-5.3-6`](#rfc1661-5.3-6)

All acceptable Configuration Options are filtered out of the Configure-Nak, but otherwise the Configuration Options from the Configure-Request MUST NOT be reordered. (§5.3)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661NakOrderFollowsRequestNotAFixedOrder, internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661NakPreservesRequestOrder moved. Reordering: TestRFC1661NakPreservesRequestOrder / TestRFC1661NakOrderFollowsRequestNotAFixedOrder. Filtering: TestRFC1661NakFiltersAcceptableOptions puts an acceptable ACCM between Magic 0 and MRU 2000 in both orders and reads the wire Configure-Nak: red unless its Types are exactly the two Nak-earning ones in request order, so a copied ACCM or a reorder both fail.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661NakFiltersAcceptableOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L139) | unit/verify | revert, verified |
| negative | [`TestRFC1661NakOrderFollowsRequestNotAFixedOrder`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1072) | unit/verify | revert, verified |
| positive | [`TestRFC1661NakFiltersAcceptableOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L138) | unit/verify | revert, verified |
| positive | [`TestRFC1661NakPreservesRequestOrder`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1044) | unit/verify | revert, verified |

### [`RFC1661-5.3-7`](#rfc1661-5.3-7)

On reception of a Configure-Nak, the Identifier field MUST match that of the last transmitted Configure-Request. (Section 5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read Section 5.3 and all three tagged units. The server's structurally valid MRU Nak with only the Identifier changed leaves FSM, output and Magic unchanged, while its matching Nak sends a fresh request carrying MRU1400. The client wrong-Identifier Nak emits no replacement and cannot finish; the matching-Nak unit requires a fresh request Identifier, refuses the previous Ack and completes on the new Ack. The positive and negative assertions therefore distinguish the Identifier condition rather than blanket refusal. Explicit Max-Failure five preserves the old client inputs and assertions. Both receivers call ValidateLCPReply before adopting Nak options. Existing client revert records show the negotiator is exercised; their panic route is not claimed to specifically prove the Identifier guard.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L404) | unit/verify | not verified |
| negative | [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L22) | unit/verify | revert, verified |
| positive | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L403) | unit/verify | not verified |
| positive | [`TestClientLCPNakChangesIdentifierAndDropsStaleReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L145) | unit/verify | revert, verified |

### [`RFC1661-5.3-8`](#rfc1661-5.3-8)

Since the Nak'd Option has been modified by the peer, the implementation MUST be able to handle an Option length which is different from the original Configure-Request. (Section 5.3)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661NakHandlesDifferentOptionLength, internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661NakTooShortOptionNotDecoded moved. RFC 1661 Section 5.3: Since the Nak'd Option has been modified by the peer, the implementation MUST be able to handle an Option length which is different from the original Configure-Request. Rejudged TestRFC1661NakHandlesDifferentOptionLength and TestRFC1661NakTooShortOptionNotDecoded: Q1 yes, the shorter 4-octet PAP suggestion is accepted against the offered 5-octet CHAP form while a one-octet protocol value is not decoded; Q2 yes, the recorded WalkLCPOptions minimum-length mutant and adjustAuthOnNakOrReject one-byte decode mutant discriminate these exact assertions; Q3 yes, explicit permitted orders and the already-offered CHAP bit isolate length handling from authentication fallback eligibility, and the malformed value remains structurally inside its own option; Q4 yes, the variable-length acceptance obligation and unsafe-short-input boundary are covered. Existing enforced verdict retained; no product fix identified.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661NakTooShortOptionNotDecoded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1117) | unit/verify | mutant, verified |
| positive | [`TestRFC1661NakHandlesDifferentOptionLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1093) | unit/verify | mutant, verified |

### [`RFC1661-5.4-1`](#rfc1661-5.4-1)

If some Configuration Options received in a Configure-Request are not recognizable or are not acceptable for negotiation (as configured by a network administrator), then the implementation MUST transmit a Configure-Reject. (Section 5.4)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_option_length_test.go::TestRFC1661LCPUnrecognizedTypeOutranksInvalidLength, internal/component/l2tp/ppp/rfc1661_option_length_test.go::TestRFC1661NCPUnrecognizedTypeOutranksInvalidLength, internal/component/l2tp/ppp/rfc1661_option_length_test.go::TestRFC1661NCPUnrecognizedTypeOutranksInvalidLength#2, internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661ConfigureRejectForUnrecognizedOption, internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661NoConfigureRejectWhenAllOptionsRecognized, internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go::TestRFC1661ClientAcceptsServerAuthProtocol, internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go::TestRFC1661ClientUnrecognizedTypeOutranksInvalidLength, internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go::TestRFC1661ClientUnrecognizedTypeOutranksInvalidLength#2 moved. Clause (a) unrecognized: TestRFC1661ConfigureRejectForUnrecognizedOption and the OutranksInvalidLength units. Clause (b) refused by configuration: TestRFC1661ConfigureRejectForOptionRefusedByConfiguration sends Auth-Protocol (any session) and ACCM (PPPoE), red unless the Reject carries exactly that option and no Ack is written; the same ACCM on L2TP, which configuration accepts, must be Acked with no Reject.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661ConfigureRejectForOptionRefusedByConfiguration`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L170) | unit/verify | revert, verified |
| negative | [`TestRFC1877ConfigureRejectEchoesUnsupportedOnly`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_dns_options_test.go#L70) | unit/verify | revert, verified |
| negative | [`TestRFC1661NCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L616) | unit/verify | revert, verified |
| negative | [`TestRFC1661NoConfigureRejectWhenAllOptionsRecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L849) | unit/verify | mutant, verified |
| negative | [`TestRFC1661ClientAcceptsServerAuthProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L407) | unit/verify | revert, verified |
| negative | [`TestRFC1661ClientUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L255) | unit/verify | revert, verified |
| positive | [`TestRFC1661ConfigureRejectForOptionRefusedByConfiguration`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L169) | unit/verify | revert, verified |
| positive | [`TestRFC1661LCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L544) | unit/verify | mutant, verified |
| positive | [`TestRFC1661NCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L611) | unit/verify | revert, verified |
| positive | [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L818) | unit/verify | mutant, verified |
| positive | [`TestRFC1661ClientUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L250) | unit/verify | revert, verified |

### [`RFC1661-5.4-2`](#rfc1661-5.4-2)

All recognizable and negotiable Configuration Options are filtered out of the Configure-Reject, but otherwise the Configuration Options MUST NOT be reordered or modified in any way. (§5.4)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_option_length_test.go::TestRFC1661LCPRejectEchoesTheRefusedOptionUnmodified, internal/component/l2tp/ppp/rfc1661_option_length_test.go::TestRFC1661LCPRejectEchoesTheRefusedOptionUnmodified#2, internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661ConfigureRejectDoesNotReorderOptions, internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661ConfigureRejectForUnrecognizedOption, internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go::TestRFC1661ClientRejectEchoesTheRefusedOptionUnmodified, internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go::TestRFC1661ClientRejectEchoesTheRefusedOptionUnmodified#2 moved. Sentence: all recognizable and negotiable options are filtered out of the Configure-Reject, but otherwise the options MUST NOT be reordered or modified in any way. (a) Keeping a recognizable option: TestRFC1661ConfigureRejectForUnrecognizedOption sends MRU(1400)+type 99 and bytes.Equal(pkt.Data, optStream(unknown)) goes red if MRU stays. (b) Reordering: TestRFC1661ConfigureRejectDoesNotReorderOptions compares Types in both orders. (c) Modifying: the same test compares each Data, and TestRFC1661LCPRejectEchoesTheRefusedOptionUnmodified / TestRFC1661ClientRejectEchoesTheRefusedOptionUnmodified check the echoed bytes.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661LCPRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L982) | unit/verify | not verified |
| negative | [`TestRFC1661ConfigureRejectDoesNotReorderOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L867) | unit/verify | not verified |
| negative | [`TestRFC1661ClientRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L298) | unit/verify | not verified |
| positive | [`TestRFC1877ConfigureRejectEchoesUnsupportedOnly`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_dns_options_test.go#L67) | unit/verify | revert, verified |
| positive | [`TestRFC1661LCPRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L971) | unit/verify | not verified |
| positive | [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L821) | unit/verify | not verified |
| positive | [`TestRFC1661ClientRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L286) | unit/verify | not verified |

### [`RFC1661-5.4-3`](#rfc1661-5.4-3)

On reception of a Configure-Reject, the Identifier field MUST match that of the last transmitted Configure-Request. (Section 5.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read Section 5.4 and all three tagged units. A Reject carrying an unchanged requested MRU option under the wrong Identifier leaves FSM/wire or client completion/output unchanged. A matching Identifier causes the next request to omit that option, and the client covers MRU-only, Magic-only and whole-request Rejects before completing on an exact replacement Ack. The wrong-Identifier Reject contains an otherwise valid requested subset, isolating the Identifier rule. The client fixture adds only valid Max-Failure five, retaining every packet and assertion. Both current receiving paths apply the shared Identifier validator before rejection flags or requests change. Producer-disable records prove those paths are reached; the exact acceptance/discard assertions independently enforce this clause.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L406) | unit/verify | not verified |
| negative | [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L23) | unit/verify | revert, verified |
| positive | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L405) | unit/verify | not verified |
| positive | [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L76) | unit/verify | revert, verified |

### [`RFC1661-5.4-4`](#rfc1661-5.4-4)

Additionally, the Configuration Options in a Configure-Reject MUST be a subset of those in the last transmitted Configure- Request. (§5.4, erratum 543)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read Section 5.4, retained verified erratum543, and all three tagged units. The corrected requirement permits a subset including the entire original option list. Client valid MRU-only, Magic-only and all-option Rejects create the exact remaining options; modified or reordered Rejects with the current Identifier produce no replacement. The server additionally refuses an unrequested option while accepting the original ordered MRU subset. These are structurally valid option lists, so changed content, membership and ordering are isolated from a packet-length fault. Explicit Max-Failure five changes no assertion. ValidateLCPReply matches each Reject option byte-for-byte against its requested instance and preserves request order, without requiring a proper subset. Revert records establish receiving-path reachability rather than a targeted subset cut; the semantic input pairs were independently read.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L408) | unit/verify | not verified |
| negative | [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L24) | unit/verify | revert, verified |
| positive | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L407) | unit/verify | not verified |
| positive | [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L77) | unit/verify | revert, verified |

### [`RFC1661-5.4-5`](#rfc1661-5.4-5)

Reception of a valid Configure-Reject indicates that when a new Configure-Request is sent, it MUST NOT include any of the Configuration Options listed in the Configure-Reject. (§5.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read Section 5.4 and all three tagged units. Server MRU-only rejection preserves the other original bytes, while rejection of MRU/Auth/Magic yields an empty request that a valid Ack can advance. Client MRU-only, Magic-only and all-option cases assert exact surviving bytes, unchanged retransmission, and no resurrection from later appended Nak suggestions, then complete with a matching Ack. The IPCP control removes both DNS options or Secondary-DNS alone, checks the exact remaining request and completes address negotiation with no rejected DNS learned. Removal and preservation are distinct assertions, not two tags on one outcome. New client Max-Failure-five and server validated-LCP fixtures preserve all peer buffers and expectations. Current server rejection flags and client LCP/IPCP request builders retain the removal. Native DNS mutation and producer-disable records supplement these independently read assertions without being described as broader semantic proofs.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestLCPRejectedOptionsStayRemoved`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L534) | unit/verify | not verified |
| negative | [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L79) | unit/verify | revert, verified |
| positive | [`TestLCPRejectedOptionsStayRemoved`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L533) | unit/verify | not verified |
| positive | [`TestRFC1661ClientDropsRejectedDNSOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_client_dns_test.go#L149) | unit/verify | mutant, verified |
| positive | [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L78) | unit/verify | revert, verified |

### [`RFC1661-5.5-1`](#rfc1661-5.5-1)

Upon reception of a Terminate-Request, a Terminate-Ack MUST be transmitted. (Section 5.5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), shifted: internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661NoTerminateAckForTerminateAck, internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661TerminateAckSentAndLinkHeld moved. The tagged positive TestRFC1661TerminateAckSentAndLinkHeld drives LCP Opened and requires a Terminate-Ack with the request Identifier 0x77; its tagged negative receives a Terminate-Ack in Opened and refuses a Terminate-Ack response. checkPPPCodeReject assertion 6 drives IPCP Opened and requires the exact fifth injected frame IPCP 05 5a 00 04 and a matching Ze Terminate-Ack, after assertion 5 excluded prior Code 1-6 traffic. RFC 1332 Section 2 inherits this LCP exchange. These are useful and isolating proofs, and the main native guest baseline passed the current checker. They do not prove the whole Section 5.5 obligation: LCPDoTransition has separate sta edges for Stopped, Closing, Stopping, Ack-Rcvd and Ack-Sent outside these tagged tests. Closed has the untagged TestLCPPeerTerminateRestartTimer proof and IPCP Req-Sent has TestIPCPUsedCodesAreNotCodeRejected, but they do not cover the listed edges. This pre-existing gap remains owned by spec-ppp-l2tp-aaa-one-off-defects in plan/journal/requirement-proven-in-one-state-of-many.md. Weak corrects the earlier whole-row enforced verdict without reducing any test. AC-3 preserves the armed trigger and all later controls through explicit phase metadata; the added Echo precedes the original IPCP termination probe. Main native paired renewal now passed the clean current checker and observed the STA action break in performNCPAction fail the cited assertion 6 for the missing IPCP Terminate-Ack; native rfc record wrote its renewal.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661NoTerminateAckForTerminateAck`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L598) | unit/verify | not verified |
| positive | [`TestRFC1661TerminateAckSentAndLinkHeld`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L564) | unit/verify | revert, verified |
| positive | [`checkPPPCodeReject`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/l2tp/ppp_code_reject.go#L96) | interop/nightly | mutant, verified |

### [`RFC1661-5.6-1`](#rfc1661-5.6-1)

This MUST be reported back to the sender of the unknown Code by transmitting a Code- Reject. (Section 5.6)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), shifted: internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661CodeRejectForUnknownCode, internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661NoCodeRejectForKnownCode moved. The tagged positive TestRFC1661CodeRejectForUnknownCode drives LCP Opened with Code 99, Identifier 0x41 and Data aabb, and requires Code-Reject with the exact rejected header and Data; the negative drives a recognized Terminate-Request in Opened and refuses a Code-Reject. checkPPPCodeReject assertion 2 requires two LCP and two IPCP Code-Rejects for exactly four observed injected probes; assertions 3 and 4 retain exact counts, distinct response Identifiers and the rejected packet identity/prefix. The main native guest baseline passed the current checker. Those proofs isolate reporting an unknown Code in the exercised states, but LCPDoTransition has separate RUC-to-scj edges in Closing and Stopping that the tagged tests do not reach. Existing additional FSM table tests cover other RUC cells and still omit these two. This pre-existing whole-row gap is recorded under spec-ppp-l2tp-aaa-one-off-defects in plan/journal/requirement-proven-in-one-state-of-many.md. Weak corrects the earlier enforced verdict without weakening any assertion or claiming a product defect. AC-3 preserves the original arming boundary and pre-injection controls. Main native paired renewal passed the clean current checker and observed writeCodeReject changed from LCPCodeReject to LCPEchoReply fail the cited assertion 2; native rfc record wrote its renewal.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661NoCodeRejectForKnownCode`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1173) | unit/verify | not verified |
| positive | [`TestRFC1661CodeRejectForUnknownCode`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1141) | unit/verify | not verified |
| positive | [`checkPPPCodeReject`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/l2tp/ppp_code_reject.go#L82) | interop/nightly | mutant, verified |

### [`RFC1661-5.6-2`](#rfc1661-5.6-2)

The Identifier field MUST be changed for each Code-Reject sent. (Section 5.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. The paired probes deliberately share one rejected Identifier, isolating the forbidden copy. TestRFC1661LCPCodeRejectIdentifierChanges drives two LCP Code 99 packets in Opened and requires exactly two Code-Rejects with different Identifiers. TestRFC1661NCPCodeRejectIdentifierChanges does the same for IPCP and IPv6CP Code 12 in Req-Sent. checkPPPCodeReject assertion 3 requires exactly two distinct Identifiers per LCP/IPCP protocol on the live wire, and its normal native guest baseline passed. The single-polarity annotation remains justified: sendCodeReject advances the LCP counter and nextNCPCodeRejectID advances the family counter before each sent Code-Reject; no input asks the sender to retain its previous Identifier. These common producer paths and isolating probes enforce the sent-Identifier requirement. AC-3 keeps the armed trigger and later control population, original four probes and response counts. Main native paired renewal passed the clean current checker and observed sendNCPCodeReject using the rejected req.Identifier fail assertion 3 with both Code-Reject Identifiers equal to 66; native rfc record wrote its renewal.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC1661LCPCodeRejectIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_code_reject_test.go#L48) | unit/verify | mutant, verified |
| positive | [`TestRFC1661NCPCodeRejectIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_code_reject_test.go#L76) | unit/verify | mutant, verified |
| positive | [`checkPPPCodeReject`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/l2tp/ppp_code_reject.go#L86) | interop/nightly | mutant, verified |

### [`RFC1661-5.6-3`](#rfc1661-5.6-3)

The Rejected-Packet MUST be truncated to comply with the peer's established MRU. (Section 5.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC1661CodeRejectTruncatedToMRU drives 404-octet LCP and IPCP packets with negotiated MRU 100 and requires an Information field of exactly 100 octets, the original Code/Identifier/Length 404 and the first 92 Data octets. The negative TestRFC1661CodeRejectKeptWhenItFits requires all 44 octets of a fitting LCP packet, so an unconditional cut fails. checkPPPCodeReject assertion 4 with peer MRU 128 and 204-octet probes requires every observed LCP/IPCP rejection to fit MRU, retain a consistent outer Length and preserve the rejected Code/Identifier/original Length and leading Data. Assertion 3 separately keeps exactly two responses per protocol. The shared writeCodeReject producer applies the same conditional minimum to the available buffer and peer MRU; the tests isolate the truncate-versus-retain boundary. The main native guest baseline passed the current checker. AC-3 preserves the original armed control population and predicates. Main native paired renewal passed the clean current checker and observed the copied Rejected-Packet Identifier changed in writeCodeReject fail assertion 4; native rfc record wrote its renewal. That break discriminates the copy clause, alongside the existing conditional MRU unit proofs described above.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661CodeRejectKeptWhenItFits`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_code_reject_test.go#L185) | unit/verify | mutant, verified |
| positive | [`TestRFC1661CodeRejectTruncatedToMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_code_reject_test.go#L122) | unit/verify | mutant, verified |
| positive | [`checkPPPCodeReject`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/l2tp/ppp_code_reject.go#L91) | interop/nightly | mutant, verified |

### [`RFC1661-5.7-1`](#rfc1661-5.7-1)

If the LCP automaton is in the Opened state, then this MUST be reported back to the peer by transmitting a Protocol-Reject. (Section 5.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 1661 Section 5.7: "If the LCP automaton is in the Opened state, then this MUST be reported back to the peer by transmitting a Protocol-Reject." Rejudged statically at HEAD; the body is unchanged. TestRFC1661UnsupportedProtocolRejectedInOpened goes red unless one LCP code-8 frame names the received protocol. TestRFC1661UnsupportedProtocolNotRejectedBeforeOpened requires zero frames outside Opened, matching the same section: "Protocol-Reject packets can only be sent in the LCP Opened state." The mutant record on the handleFrame fall-through is semantic; the negative record is a producer panic showing reachability only, and removing the Opened gate goes red by reading.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661UnsupportedProtocolNotRejectedBeforeOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L88) | unit/verify | revert, verified |
| positive | [`TestRFC1661UnsupportedProtocolRejectedInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L40) | unit/verify | mutant, verified |

### [`RFC1661-5.7-2`](#rfc1661-5.7-2)

Upon reception of a Protocol-Reject, the implementation MUST stop sending packets of the indicated protocol at the earliest opportunity. (Section 5.7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1661-5.7-2, so there is no linked test.

### [`RFC1661-5.7-3`](#rfc1661-5.7-3)

The Identifier field MUST be changed for each Protocol-Reject sent. (Section 5.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. {single-polarity} justified. Forbidden: two Protocol-Rejects with one Identifier. TestRFC1661ProtocolRejectIdentifierChanges goes red when the two consecutive Protocol-Rejects carry equal Identifiers.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC1661ProtocolRejectIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L136) | unit/verify | mutant, verified |

### [`RFC1661-5.7-4`](#rfc1661-5.7-4)

The Rejected-Information MUST be truncated to comply with the peer's established MRU. (Section 5.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: Rejected-Information exceeding the peer's MRU. TestRFC1661ProtocolRejectInformationTruncatedToMRU goes red unless the frame is frameLen(100) and the kept data is 94 octets from the head; TestRFC1661ProtocolRejectInformationKeptWhenItFits goes red if a fitting 40-octet packet is cut.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661ProtocolRejectInformationKeptWhenItFits`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L200) | unit/verify | mutant, verified |
| positive | [`TestRFC1661ProtocolRejectInformationTruncatedToMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L165) | unit/verify | mutant, verified |

### [`RFC1661-5.8-1`](#rfc1661-5.8-1)

Upon reception of an Echo-Request in the LCP Opened state, an Echo-Reply MUST be transmitted. (Section 5.8)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661EchoReplyInOpened, internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661NoEchoOutsideOpened moved. Judge ppp4 2026-09-30. TestRFC1661EchoReplyInOpened: Opened, Echo-Request with the peer's Magic-Number draws an Echo-Reply with Identifier 0x51 and ze's Magic (+). TestRFC1661NoEchoOutsideOpened (-): now sets peerMagic 0x99887766 and sends that Magic, so the Magic-Number filter in handleLCPPacket passes and the FSM state alone decides; at HEAD the Data equalled the local magic with peerMagic 0 and the packet was dropped before the FSM, a vacuous negative that is now fixed.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661NoEchoOutsideOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1235) | unit/verify | not verified |
| positive | [`TestRFC1661EchoReplyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1194) | unit/verify | not verified |

### [`RFC1661-5.8-2`](#rfc1661-5.8-2)

Echo-Request and Echo-Reply packets MUST only be sent in the LCP Opened state. (Section 5.8)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/l2tp/ppp/session_run.go::run moved. Independent CURRENT-Core source rejudgment against RFC1661 Section5.8: Echo-Request and Echo-Reply may only be sent in LCP Opened. Complete TestRFC1661KeepaliveEchoOnlyInOpened drives actual run through accepted authentication to an emitted Echo, then peer renegotiation; after three restart intervals its next packet must be the configured request retransmission, never a stale ticker Echo. TestRFC1661EchoReplyInOpened requires the actual reply Identifier/Magic; TestRFC1661NoEchoOutsideOpened uses the same valid peer Magic in ReqSent/AckSent/AckRcvd/Stopped and forbids both Echo Codes, isolating state from Magic rejection. Complete client TestClientLCPEchoBeforeOpenDiscarded injects valid Echo Requests before completion and in AckRcvd, retaining exactly the original Configure-Request before valid final peer configuration. Read LCPDoTransition/performAction plus run ticker state transition and client negotiateLCP Echo discard: only Opened permits server emission and leaving Opened stops old ticker channels. This is assertion/producer source judgment; no new runtime/native evidence inferred.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661KeepaliveEchoOnlyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L126) | unit/verify | mutant, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC1661NoEchoOutsideOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1239) | unit/verify | revert, verified |
| negative | [`TestClientLCPEchoBeforeOpenDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_transition_test.go#L115) | unit/verify | revert, verified |
| positive | [`TestRFC1661KeepaliveEchoOnlyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L125) | unit/verify | revert, verified |
| positive | [`TestRFC1661EchoReplyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1197) | unit/verify | mutant, verified |

### [`RFC1661-5.9-1`](#rfc1661-5.9-1)

Discard-Request packets MUST only be sent in the LCP Opened state. (Section 5.9)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1661-5.9-1, so there is no linked test.

### [`RFC1661-5.9-2`](#rfc1661-5.9-2)

On reception, the receiver MUST silently discard any Discard- Request that it receives. (Section 5.9)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661DiscardRequestSilentlyDiscarded moved. {single-polarity} justified. Forbidden: any reply or effect for a Discard-Request. TestRFC1661DiscardRequestSilentlyDiscarded goes red on any written frame, a state change or a lifecycle event.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC1661DiscardRequestSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1264) | unit/verify | not verified |

### [`RFC1661-6.2-1`](#rfc1661-6.2-1)

An implementation MUST NOT include multiple Authentication- Protocol Configuration Options in its Configure-Request packets. (Section 6.2)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661SingleAuthProtocolOptionInRequest moved. {single-polarity} justified. Forbidden: two Authentication-Protocol options in a request. TestRFC1661SingleAuthProtocolOptionInRequest goes red when BuildLocalConfigRequest emits a count != 1 for PAP, CHAP-MD5 or MS-CHAPv2.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC1661SingleAuthProtocolOptionInRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1296) | unit/verify | not verified |

### [`RFC1661-6.4-1`](#rfc1661-6.4-1)

If an implementation does transmit a Configure-Request with a Magic-Number Configuration Option, then it MUST NOT respond with a Configure-Reject when it receives a Configure-Request with a Magic-Number Configuration Option. (Section 6.4)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661UnknownOptionRejectedWhileMagicIsNot, internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661ZeroMagicNumberRefused moved. Forbidden: Configure-Rejecting a peer Magic-Number while ze sends one. TestRFC1661ZeroMagicNumberRefused goes red on any reject of a zero Magic; TestRFC1661UnknownOptionRejectedWhileMagicIsNot goes red unless the Magic is Acked while Type 99 in the same request is Rejected (proving the reject path is live).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661UnknownOptionRejectedWhileMagicIsNot`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1346) | unit/verify | not verified |
| positive | [`TestRFC1661ZeroMagicNumberRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1374) | unit/verify | not verified |

### [`RFC1661-6.4-2`](#rfc1661-6.4-2)

If Magic-Number has been successfully negotiated, an implementation MUST transmit these packets with the Magic-Number field set to its negotiated Magic-Number. (Section 6.4)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661EchoReplyDoesNotMirrorPeerMagic, internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661EchoReplyInOpened moved. Judge ppp4 2026-09-30. Re-read after the comment-only edit to TestRFC1661EchoReplyInOpened: it still asserts the Echo-Reply Magic-Number equals the local negotiated value (+); negatives and the Echo-Request half (TestEchoRequestCarriesNegotiatedMagic) unchanged. ze sends no Discard-Request (no producer).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestEchoRequestCarriesNegotiatedMagic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L419) | unit/verify | revert, verified |
| negative | [`TestRFC1661EchoReplyDoesNotMirrorPeerMagic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1413) | unit/verify | revert, verified |
| positive | [`TestEchoRequestCarriesNegotiatedMagic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L418) | unit/verify | revert, verified |
| positive | [`TestRFC1661EchoReplyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1206) | unit/verify | mutant, verified |

### [`RFC1661-6.4-3`](#rfc1661-6.4-3)

A Magic-Number of zero is illegal and MUST always be Nak'd, if it is not Rejected outright. (Section 6.4)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661PeerMagicNumberAcked, internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661ZeroMagicNumberRefused, internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go::TestRFC1661ClientNaksZeroMagicWithAValueOfItsOwn, internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go::TestRFC1661ClientNaksZeroMagicWithAValueOfItsOwn#2 moved. Forbidden: accepting a zero Magic-Number. TestRFC1661ZeroMagicNumberRefused (server) and TestRFC1661ClientNaksZeroMagicInAWellFormedRequest / ...WithAValueOfItsOwn (client) go red unless a zero Magic draws a Nak carrying a 4-octet non-zero value; negatives Ack a non-zero Magic (TestRFC1661PeerMagicNumberAcked, TestRFC1661ClientAcksANonZeroMagic).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1661PeerMagicNumberAcked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1327) | unit/verify | not verified |
| negative | [`TestRFC1661ClientAcksANonZeroMagic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_client_negotiation_test.go#L82) | unit/verify | not verified |
| negative | [`TestRFC1661ClientNaksZeroMagicWithAValueOfItsOwn`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L461) | unit/verify | not verified |
| positive | [`TestRFC1661ZeroMagicNumberRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1366) | unit/verify | not verified |
| positive | [`TestRFC1661ClientNaksZeroMagicInAWellFormedRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_client_negotiation_test.go#L44) | unit/verify | not verified |
| positive | [`TestRFC1661ClientNaksZeroMagicWithAValueOfItsOwn`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L455) | unit/verify | not verified |

### [`RFC1661-6.5-1`](#rfc1661-6.5-1)

By default, all implementations MUST transmit packets with two octet PPP Protocol fields. (Section 6.5)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661TwoOctetProtocolField moved. {single-polarity} justified. Forbidden: a one-octet Protocol field on transmit. TestRFC1661TwoOctetProtocolField goes red when WriteFrame writes off != 2 or octets != c021.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC1661TwoOctetProtocolField`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L254) | unit/verify | not verified |

### [`RFC1661-6.5-2`](#rfc1661-6.5-2)

Compressed Protocol fields MUST NOT be transmitted unless this Configuration Option has been negotiated. (Section 6.5)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_test.go::TestRFC1661TwoOctetProtocolField moved. {single-polarity} justified: WriteFrame has no compressed branch. Forbidden: a compressed Protocol without negotiation. TestRFC1661TwoOctetProtocolField goes red if WriteFrame writes one octet. Its pfc loop is decorative (the built option list never reaches WriteFrame); the assertion that discriminates is the off != 2 check.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC1661TwoOctetProtocolField`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L258) | unit/verify | not verified |

### [`RFC1661-6.5-3`](#rfc1661-6.5-3)

When negotiated, PPP implementations MUST accept PPP packets with either double-octet or single-octet Protocol fields, and MUST NOT distinguish between them. (§6.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1661-6.5-3, so there is no linked test.

### [`RFC1661-6.6-1`](#rfc1661-6.6-1)

By default, all implementations MUST transmit frames with Address and Control fields appropriate to the link framing. (Section 6.6)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Implemented and observed within the L2TP scenario: checkPPPCodeReject assertion 7 reads normal datagrams from ordinal one, requires consecutive ordinals and explicit arming metadata, and applies strict ff03 parsing to every Ze data datagram before protocol filtering. Startup Configure exchanges, LCP/IPCP Code-Rejects, the correlated Echo-Reply and IP traffic are required witnesses. The main native ARM64 Ze-kernel guest paired run passed the clean checker and failed the relay log-only strip at the cited assertion 7 with missing Address and Control octets. Native rfc record wrote the current positive discrimination record for checkPPPCodeReject, breaking framingCapture.observe. This does not exercise PPPoE or every link framing covered by the whole requirement. WriteFrame builds the Protocol field and payload and pppSession.writeFrame writes them to its channel; the lower-layer attribution remains. The observed relay log-only strip discriminates the L2TP observation rather than breaking that annotated producer. It therefore cannot provide OS-verified credit, and whole-row coverage remains an OS testing gap. The recorded positive proves this bounded observation; whole-row OS and PPPoE proof remain absent, and no code defect is inferred from that missing breadth. The retained lower-layer explanation is judged here only for the exercised kernel L2TP path; concurrent PPPoE/VPP transport work receives no coverage from this capture.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`checkPPPCodeReject`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/l2tp/ppp_code_reject.go#L99) | interop/nightly | mutant, verified |

### [`RFC1661-6.6-2`](#rfc1661-6.6-2)

The Address and Control fields MUST NOT be compressed when sending any LCP packet. (Section 6.6)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Implemented and observed within the L2TP scenario: assertion 7 checks uncompressed Ze LCP output, including Configure, Code-Reject and Echo-Reply witnesses. Assertion 8 requires both acknowledged Configure exchanges to omit ACFC, injects one specifically identified compressed peer Echo-Request with its acknowledged Magic-Number, and requires a later ff03 Echo-Reply carrying the same Identifier and Data and Ze Magic, unchanged interfaces and useful traffic. Native paired records now verify both polarities of this bounded L2TP checker. The clean runs passed; the relay log-only Address/Control strip failed the cited assertion 7, and suppressing the one compressed probe failed the cited assertion 8 because the capture held zero compressed probes. Native rfc record wrote both current checkPPPCodeReject records, breaking framingCapture.observe and framingCapture.injectCompressed respectively. Section 6.6 permits silent discard; the accepting receive path is the observed implementation behavior, not a mandatory RFC receive obligation. This negative uses a violating peer input and checks Ze output independently; it does not assert that reception must accept or reject it. Negotiated-ACFC LCP transmission and PPPoE are outside this capture, so a defect confined to those paths would not be detected here. The lower-layer attribution remains, and the observed relay capture/probe breaks do not break annotated writeFrame or provide whole-row OS verification. The three new 6.6 polarity records are observed; whole-row OS and omitted-transport proof remain absent. The retained lower-layer explanation is judged here only for the exercised kernel L2TP path; concurrent PPPoE/VPP transport work receives no coverage from this capture.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`checkPPPCodeReject`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/l2tp/ppp_code_reject.go#L105) | interop/nightly | mutant, verified |
| positive | [`checkPPPCodeReject`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/l2tp/ppp_code_reject.go#L103) | interop/nightly | mutant, verified |

### [`RFC1661-4.6-1`](#rfc1661-4.6-1)

The Restart timer MUST be configurable (§4.6)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_configured_restart_test.go::TestConfiguredLCPRestartTimer, internal/component/l2tp/ppp/rfc1661_configured_restart_test.go::TestConfiguredLCPRestartTimer#2 moved. Section 4.6 requires the Restart timer to be configurable. TestConfiguredLCPRestartTimer observes SCR's exact 500 ms and 3 s periods, rejects early expiry and checks timer cancellation on Opened. A fixed period fails the configured interval. Both server transport configuration paths supply the validated session setting read by armRestartTimer. Client LCP and IPv6CP have separate schema-backed interval snapshots and one-shot request timers; the server tags here claim the shared server timer's exact expiry and cancellation, rather than proof of every client lifecycle.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestConfiguredLCPRestartTimer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_configured_restart_test.go#L39) | unit/verify | mutant, verified |
| positive | [`TestConfiguredLCPRestartTimer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_configured_restart_test.go#L38) | unit/verify | mutant, verified |

### [`RFC1661-4.6-2`](#rfc1661-4.6-2)

Max-Terminate MUST be configurable (§4.6)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_configured_restart_test.go::TestConfiguredLCPMaxTerminate, internal/component/l2tp/ppp/rfc1661_configured_restart_test.go::TestConfiguredLCPMaxTerminate#2 moved. Section 4.6 defines Max-Terminate as the number of unanswered Terminate-Requests. TestConfiguredLCPMaxTerminate drives Close with budgets of 2 and 4, requires exactly that many transmissions and Closed on exhaustion, and contrasts a matching first Terminate-Ack that closes after one request. Both branches explicitly require each emitted frame to use ProtoLCP and LCPTerminateRequest, so an unrelated LCP packet cannot satisfy the request-count claim. Both server transports configure this shared automaton; the client has no local Terminate-Request sender. Its PADT cleanup capability is documented separately, not treated as a general exemption for PPP clients.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestConfiguredLCPMaxTerminate`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_configured_restart_test.go#L159) | unit/verify | mutant, verified |
| positive | [`TestConfiguredLCPMaxTerminate`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_configured_restart_test.go#L158) | unit/verify | mutant, verified |

### [`RFC1661-4.6-3`](#rfc1661-4.6-3)

Max-Configure MUST be configurable (§4.6)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/l2tp/ppp/session_run.go::run moved. Independent CURRENT-Core source rejudgment against RFC1661 Section4.6, Max-Configure must be configurable. Complete TestConfiguredLCPMaxConfigure contrasts budgets3/5, exact Configure-Request counts, exhausted restartCount0 and Stopped, plus a matching answering peer that opens after exactly two requests. Complete proxy restart pair and helper enter accepted proxy LCP through a real Driver, observe no initial/retry packets while Opened, then require configured3/5 requests at exact500ms boundaries after peer RCR, final session-down/Stopped and matching elapsed time. The answering proxy contrast requires reopening after exactly two requests and no later event. Current run retains configured MaxConfigure for accepted proxy lifetime; applyTransition loads appropriate configured IRC value, sendConfigureRequest decrements, handleRestartTimeout dispatches TO+/TO-. Read ParseLCP parameter-to-NewLCPConfig flow and client negotiateLCP configured remaining counter: no changed shared code replaced configuration with a fixed value. Source rejudgment covers this counter, not other NCP/client settings; native evidence/stamp is not claimed.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestConfiguredLCPMaxConfigure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_configured_restart_test.go#L87) | unit/verify | mutant, verified |
| negative | [`TestProxyLCPRenegotiationAnswerStopsRestartBudget`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_proxy_restart_budget_test.go#L69) | unit/verify | mutant, verified |
| positive | [`TestConfiguredLCPMaxConfigure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_configured_restart_test.go#L86) | unit/verify | mutant, verified |
| positive | [`TestProxyLCPRenegotiationUsesConfiguredRestartBudget`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_proxy_restart_budget_test.go#L20) | unit/verify | mutant, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`RFC1661-4.6-4`](#rfc1661-4.6-4)

Max-Failure MUST be configurable (§4.6)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/l2tp/pppoeclient/dialer.go::Dialer.Dial moved. Section 4.6 counts Configure-Naks sent without sending a Configure-Ack. TestConfiguredLCPMaxFailure and TestRFC1661ClientMaxFailureConvertsFurtherNak each exercise configured limits of two and five, require Naks through the limit, and require the next reply to be a Reject carrying the original unacceptable MRU bytes. The server test and TestRFC1661ClientMaxFailureAckRenewsBudget separately require a successful Ack to renew the budget. Both transport configurations reach the shared server counter; the client schema and Dial snapshot reach negotiateLCP's counter. These assertions distinguish a fixed limit, premature rejection, substituted suggestion bytes and a missing Ack reset. This judgment covers the implemented LCP Configure-Nak senders; it makes no claim about absent IPv6CP counter configuration.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestConfiguredLCPMaxFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_configured_restart_test.go#L200) | unit/verify | mutant, verified |
| negative | [`TestRFC1661ClientMaxFailureAckRenewsBudget`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_max_failure_test.go#L58) | unit/verify | mutant, verified |
| positive | [`TestConfiguredLCPMaxFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_configured_restart_test.go#L199) | unit/verify | mutant, verified |
| positive | [`TestRFC1661ClientMaxFailureConvertsFurtherNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_max_failure_test.go#L20) | unit/verify | mutant, verified |

### [`RFC1661-4.4-1`](#rfc1661-4.4-1)

In addition to setting the Restart counter, the implementation MUST set the timeout period to the initial value when Restart timer backoff is used. (Section 4.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1661-4.4-1, so there is no linked test.

### [`RFC1661-4.4-2`](#rfc1661-4.4-2)

In addition to zeroing the Restart counter, the implementation MUST set the timeout period to an appropriate value. (Section 4.4)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1661_configured_restart_test.go::TestConfiguredLCPZeroRestartPeriod, internal/component/l2tp/ppp/rfc1661_configured_restart_test.go::TestConfiguredLCPZeroRestartPeriod#2 moved. Section 4.4 requires an appropriate timeout when zrc zeroes the counter. The existing peer-termination test preserves the default-period grace and Closed contrast. TestConfiguredLCPZeroRestartPeriod adds a configured 500 ms grace: Opened plus Terminate-Request zeroes a live counter, expires at that period into Stopped and sends only the Terminate-Ack. Closed acknowledges the same input without arming a timer. Distinct assertions reject both a missing grace and unconditional arming.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestConfiguredLCPZeroRestartPeriod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_configured_restart_test.go#L244) | unit/verify | mutant, verified |
| negative | [`TestLCPPeerTerminateRestartTimer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_restart_counter_test.go#L136) | unit/verify | revert, verified |
| positive | [`TestConfiguredLCPZeroRestartPeriod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_configured_restart_test.go#L243) | unit/verify | mutant, verified |
| positive | [`TestLCPPeerTerminateRestartTimer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_restart_counter_test.go#L135) | unit/verify | revert, verified |

### [`RFC1661-5.5-2`](#rfc1661-5.5-2)

On transmission, the Identifier field MUST be changed whenever the content of the Data field changes, and whenever a valid reply has been received for a previous request. (§5.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent re-judge (R10). TestTerminateRequestIdentifierChanges: Terminate-Request, a Terminate-Ack with its Identifier accepted as a valid reply (state Closed asserted), a second Terminate-Request; red when the second reuses the Identifier, and only the two requests are written. Data is empty on both and asserted so: sendTerminateRequest always writes nil Data, so the Data-change clause has no reachable case. The negative tag reads the same Identifier assertion as the positive; the send-side MUST has no refusing input, so this is in substance a single-polarity positive (a marker on the row would say so). The recorded break panics sendTerminateRequest; the reply-clause break (dropping terminateID++) is caught by reading the assertion.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestTerminateRequestIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L60) | unit/verify | revert, verified |
| positive | [`TestTerminateRequestIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L59) | unit/verify | revert, verified |

### [`RFC1661-5.8-4`](#rfc1661-5.8-4)

On transmission, the Identifier field MUST be changed whenever the content of the Data field changes, and whenever a valid reply has been received for a previous request. (§5.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestEchoRequestIdentifierChanges isolates both triggers: a valid Echo-Reply (echoOutstanding cleared) between two Echo-Requests with the same Data (asserted equal), red on a reused Identifier; then magicNegotiated flips with no reply, Data asserted changed, red on a reused Identifier. sendEchoReply Identifier copy also pinned.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestEchoRequestIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L90) | unit/verify | revert, verified |
| positive | [`TestEchoRequestIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L89) | unit/verify | revert, verified |

### [`RFC1661-5.8-5`](#rfc1661-5.8-5)

Until the Magic-Number Configuration Option has been successfully negotiated, the Magic- Number MUST be transmitted as zero. (§5.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Sentence: until the Magic-Number option has been successfully negotiated, the Magic-Number MUST be transmitted as zero. (a) Sending the chosen Magic-Number before negotiation: TestMagicNumberZeroUntilNegotiated goes red on req != 0 || rep != 0 before any Ack, and again after a Configure-Ack that carries no option 5. Positive: after an Ack with option 5 both packets carry the value. Echo-Request and Echo-Reply both covered; the identical Discard-Request sentence is Section 5.9 and ze sends no Discard-Request.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestMagicNumberZeroUntilNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L135) | unit/verify | revert, verified |
| positive | [`TestMagicNumberZeroUntilNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L134) | unit/verify | revert, verified |

### [`RFC1661-5.9-3`](#rfc1661-5.9-3)

The Identifier field MUST be changed for each Discard-Request sent. (Section 5.9)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1661-5.9-3, so there is no linked test.

### [`RFC1661-6.1-1`](#rfc1661-6.1-1)

If smaller packets are requested, an implementation MUST still be able to receive the full 1500 octet information field in case link synchronization is lost. (Section 6.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Read all RFC1661 Section6.1: requesting smaller packets must retain ability to receive the full1500-octet Information field; the Protocol field is outside that1500. Read every tagged body: TestFullInformationFieldReceived checks actual1502-octet frame parsing, a1500-octet control Length and reader-buffer capacity. TestReceiveFullFrameBelowConfiguredMRU drives a real Driver with requested MRU1492 and sends a structurally valid1500-octet LCP Information field through its actual reader, requiring complete receipt before the separately capped response; independently, exact fakeOps receive-MRU assertion remains1500. readFrames reads into full-frame capacity and afterLCPOpen installs MaxFrameLen receive MRU independently of the smaller negotiated request. The read-cap cut and receive-MRU cut target distinct observations; parser-only acceptance alone is not credited as whole-path proof. Rejudged independently after full old/current tagged-unit and B private lint-proposal reads. The address-taking caller edits and immediate helper-local copies preserve every old input, wire assertion, state transition, timeout and cleanup join; the changed helpers still construct genuine Driver/request/Attempt capabilities. All tagged units were included, including inline tags. Producer behavior for this requirement is unchanged by the scoped lint packet. This pending source verdict claims neither a fresh compiler/runtime result nor a native discrimination observation; MAIN owns final source stamping and recorded cuts.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestReceiveFullFrameBelowConfiguredMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_proxy_policy_driver_test.go#L125) | unit/verify | mutant, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestFullInformationFieldReceived`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L282) | unit/verify | revert, verified |
| positive | [`TestReceiveFullFrameBelowConfiguredMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_proxy_policy_driver_test.go#L124) | unit/verify | mutant, verified |

### [`RFC1661-6.4-5`](#rfc1661-6.4-5)

Before this Configuration Option is requested, an implementation MUST choose its Magic-Number. (Section 6.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: requesting option 5 before a Magic-Number is chosen. TestMagicNumberChosenBeforeRequested goes red if BuildLocalConfigRequest emits option 5 with no chosen value, or if the chosen request carries a value other than the generated non-zero one.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestMagicNumberChosenBeforeRequested`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_choice_test.go#L21) | unit/verify | revert, verified |
| positive | [`TestMagicNumberChosenBeforeRequested`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_choice_test.go#L20) | unit/verify | revert, verified |

### [`RFC1661-6.4-6`](#rfc1661-6.4-6)

If the two Magic-Numbers are equal, then it is possible, but not certain, that the link is looped-back and that this Configure-Request is actually the one last sent.  To determine this, a Configure-Nak MUST be sent specifying a different Magic-Number value. (§6.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Sentence: if the received Magic-Number equals the one last sent, a Configure-Nak MUST be sent specifying a different Magic-Number value. (a) Acking or Rejecting the equal value, or Naking with the same value: TestEqualMagicNumberIsNaked goes red on len(acks) != 0, len(rejects) != 0, offered == localMagic (and offered == 0). Negative: a different value is Acked unchanged.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestEqualMagicNumberIsNaked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_choice_test.go#L67) | unit/verify | revert, verified |
| positive | [`TestEqualMagicNumberIsNaked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_choice_test.go#L66) | unit/verify | revert, verified |

### [`RFC1661-6.4-7`](#rfc1661-6.4-7)

If the Magic-Number is equal to the one sent in the last Configure-Nak, the possibility of a looped-back link is increased, and a new Magic-Number MUST be chosen. (Section 6.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Re-read Section 6.4 and all four tagged units. playMagicLoop creates the actual condition: Ze Naks its looped-back Magic and receives that same Nak value back. The server requires a new nonzero value in the next request; its deterministic negative supplies the old value as the first entropy draw and requires the second distinct draw. The MRU-only Nak control separately leaves Magic unchanged. The client loops its own Nak back and checks a fresh proposal and subsequent Echo, but its random source never deliberately repeats the previous proposal, and its two tags share the same assertions. Removing generateDifferentMagic's rejection of magic == previous therefore normally passes that client test; the server's deterministic control exercises redrawMagicOnNak, a different implementation, and cannot catch this client regression. Existing client producer-disable records prove reachability only. The current client guard is present, but the whole implemented path lacks a deterministic repeated-draw control, so the old enforced judgment's acknowledged limitation is a real proof gap. Explicit Max-Failure five does not repair that gap. Add a client repeated-draw input and a targeted guard break before restoring enforced.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestMagicNumberRedrawnOnNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L193) | unit/verify | revert, verified |
| negative | [`TestRFC1661LoopedNakMagicRefusesRepeatedDraw`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_loop_test.go#L101) | unit/verify | revert, verified |
| negative | [`TestClientLCPMagicNakRedrawsAndUsesNegotiatedValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L187) | unit/verify | revert, verified |
| positive | [`TestMagicNumberRedrawnOnNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L192) | unit/verify | revert, verified |
| positive | [`TestRFC1661LoopedNakMagicChoosesNewMagic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_loop_test.go#L79) | unit/verify | revert, verified |
| positive | [`TestClientLCPMagicNakRedrawsAndUsesNegotiatedValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L186) | unit/verify | revert, verified |

### [`RFC1661-6.4-8`](#rfc1661-6.4-8)

All received Magic-Number fields MUST be equal to either zero or the peer's unique Magic-Number, depending on whether or not the peer negotiated a Magic-Number. (Section 6.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both cases now driven. Negotiated: TestReceivedMagicNumberMustBePeers (0, peer+1, own refused; peer's accepted). Not negotiated: TestReceivedMagicNumberZeroWhenPeerNegotiatedNone acks a Configure-Request without Magic over a stale peerMagic, then zero draws an Echo-Reply and clears echoOutstanding, while the stale value and ze's own draw no reply and leave the count. Red if handleLCPPacket kept the stale peerMagic.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestReceivedMagicNumberMustBePeers`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L231) | unit/verify | mutant, verified |
| negative | [`TestReceivedMagicNumberZeroWhenPeerNegotiatedNone`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L362) | unit/verify | mutant, verified |
| positive | [`TestReceivedMagicNumberMustBePeers`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L230) | unit/verify | mutant, verified |
| positive | [`TestReceivedMagicNumberZeroWhenPeerNegotiatedNone`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L361) | unit/verify | mutant, verified |

### [`RFC1661-3.5-6`](#rfc1661-3.5-6)

If authentication fails, the authenticator SHOULD proceed instead to the Link Termination phase. (§3.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestAuthRefusalTerminationCleanup`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_policy_driver_test.go#L149) | unit/verify | mutant, verified |
| positive | [`TestAuthRefusalTerminationCleanup`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_policy_driver_test.go#L148) | unit/verify | mutant, verified |

### [`RFC1661-6.2-2`](#rfc1661-6.2-2)

Instead, it SHOULD attempt to configure the most desirable protocol first.  If that protocol is Configure-Nak'd, then the implementation SHOULD attempt the next most desirable protocol in the next Configure-Request. (§6.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestAuthAllowSelectionMatrix`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_policy_driver_test.go#L32) | unit/verify | mutant, verified |
| positive | [`TestAuthAllowSelectionMatrix`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_policy_driver_test.go#L31) | unit/verify | mutant, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc1661.txt |
| Source fingerprint | 2547412836baf372 |
| Record | rfc/extraction/rfc1661.json |
| Mapped sentences | 73 |
| Declined as scope | 9 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 3 | reviewed | not stated |
| `1.2` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 4 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `3.1` | not stated | 2 | reviewed | not stated |
| `3.2` | not stated | 1 | reviewed | not stated |
| `3.3` | not stated | 0 | reviewed | not stated |
| `3.4` | not stated | 1 | reviewed | not stated |
| `3.5` | not stated | 4 | reviewed | not stated |
| `3.6` | not stated | 3 | reviewed | not stated |
| `3.7` | not stated | 2 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `4.1` | not stated | 0 | reviewed | not stated |
| `4.2` | not stated | 0 | reviewed | not stated |
| `4.3` | not stated | 3 | reviewed | not stated |
| `4.4` | not stated | 2 | reviewed | not stated |
| `4.5` | not stated | 0 | reviewed | not stated |
| `4.6` | not stated | 4 | reviewed | not stated |
| `5` | not stated | 1 | reviewed | not stated |
| `5.1` | not stated | 3 | reviewed | not stated |
| `5.2` | not stated | 4 | reviewed | not stated |
| `5.3` | not stated | 8 | reviewed | not stated |
| `5.4` | not stated | 5 | reviewed | not stated |
| `5.5` | not stated | 2 | reviewed | not stated |
| `5.6` | not stated | 3 | reviewed | not stated |
| `5.7` | not stated | 4 | reviewed | not stated |
| `5.8` | not stated | 4 | reviewed | not stated |
| `5.9` | not stated | 4 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `6.1` | not stated | 1 | reviewed | not stated |
| `6.2` | not stated | 1 | reviewed | not stated |
| `6.3` | not stated | 0 | reviewed | not stated |
| `6.4` | not stated | 8 | reviewed | not stated |
| `6.5` | not stated | 3 | reviewed | not stated |
| `6.6` | not stated | 2 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1.1 'Specification of Requirements' defines the keyword MUST itself. The capitalised word is the term being defined, not an obligation on an implementation. | MUST This word, or the adjective "required", means that the definition is an absolute requirement of the specification. |
| `1.1:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1.1 'Specification of Requirements' defines the phrase MUST NOT itself. The capitalised words are the term being defined, not an obligation on an implementation. | MUST NOT This phrase means that the definition is an absolute prohibition of the specification. |
| `1.1:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Part of the Section 1.1 definition of MAY: it states what the word 'optional' means for a reader of this document. The interoperation obligation it describes is discharged by the option-by-option rows, not by this definition. | An implementation which does not include this option MUST be prepared to interoperate with another implementation which does include the option. |
| `2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The second half of the same Protocol-field parity rule, which the mapped row states in full: odd least-significant octet, even most-significant octet. | Also, all Protocols MUST be assigned such that the least significant bit of the most significant octet equals "0". |
| `2:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The receive-side consequence of the same parity rule, which the mapped row carries: a frame failing the rule is treated as an unrecognized Protocol. | Frames received which don't comply with these rules MUST be treated as having an unrecognized Protocol. |
| `2:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to whoever defines a NEW PPP protocol number, as a registration action with IANA. Ze defines no new PPP protocol: internal/component/l2tp/pppoe and the LCP code speak the numbers this document and its successors already assign. The producer would be the developer of a new PPP protocol, who registers with IANA and is not a Ze code path. | Developers of new protocols MUST obtain a number from the Internet Assigned Numbers Authority (IANA), at IANA@isi.edu. |
| `3.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Introduces the phase descriptions that follow ('Not all transitions are specified in this diagram. The following semantics MUST be followed.'). It states no obligation of its own; every phase obligation is carried at its own site in Sections 3.4 to 3.7. | The following semantics MUST be followed. |
| `5.9:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The identical sentence appears under Echo-Request/Echo-Reply and under Discard-Request; the mapped row covers all three packet types. | Until the Magic-Number Configuration Option has been successfully negotiated, the Magic- Number MUST be transmitted as zero. |
| `6.4:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the preceding sentence: an implementation that uses Magic Numbers must let its peer use them, which is the ban on Configure-Reject the mapped row carries. | That is, if an implementation desires to use Magic Numbers, then it MUST also allow its peer to do so. |

## Superseded

No document obsoletes RFC 1661, so its obligations are stated where they were written.
