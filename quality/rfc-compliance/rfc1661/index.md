# RFC 1661 - The Point-to-Point Protocol (PPP)

Partial. Every requirement this repository extracted from RFC 1661, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 44.8% | 26 of 58 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 13.8% | 8 of 58 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 58 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 91 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 66 | of 91 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 8 | of 66 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 41.4% | 24 of 58 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 58 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 91 |
| Gated MUST-level | 66 |
| Obligations that bind Ze | 58 |
| Not applicable, so out of scope | 8 |
| Declared gaps | 24 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 91 |
| Tagged units | 91 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc1661.md` |
| Requirement shard | `rfc/requirements/rfc1661.md` |
| RFC text | `rfc/full/rfc1661.txt` |

## Enrolment

Enrolled: PPP LCP (RFC 1661): 27 MET (Protocol-field parity, LCP-first bring-up, per-family NCP configuration, auth requested in Link Establishment, no network phase before auth completes, Configure-Request replies, verbatim Configure-Ack echo, Nak value substitution and ordering, Configure-Reject contents and ordering, differing Nak option length, Terminate-Ack, Code-Reject, Echo-Reply only in Opened, silent Discard-Request, Magic-Number accept/zero-refuse/echo, two-octet Protocol field) + 6 single-polarity positive (LCP sent first, network-layer packets dropped before the NCP opens, no disconnect after Terminate-Ack, new Configure-Request accepted after RTR, one Auth-Protocol option, never compresses the Protocol field) + 24 gap (no send-side negotiated-MRU clamp, no LCP-phase gate on the frame dispatcher, no Protocol-Reject sender or RXJ+ suppression, constant Configure-Request Identifier and no last-sent-request record so Ack/Nak/Reject Identifier and option matching are unchecked, Code-Reject Identifier echoed, no single-octet Protocol parsing, no configurable Restart timer / Max-Terminate / Max-Configure / Max-Failure, zrc no-op) + 8 not-applicable (no link-quality protocol, no multi-instance LCP option, no Protocol-Reject or Discard-Request sender, no HDLC Address/Control framing, no Restart-timer backoff)

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

The full ten-state RFC 1661 Section 4.1 option-negotiation automaton, LCP packet and option codecs, Configure-Request/Ack/Nak/Reject negotiation with Reject-over-Nak-over-Ack precedence and verbatim option echo, Terminate-Request/Ack, Code-Reject, Echo keepalive with Magic-Number, two-octet Protocol framing, and the common NCP structure reused by IPCP and IPv6CP under L2TP and PPPoE. Tests bound per requirement in [`rfc/requirements/rfc1661.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc1661.md).

**What the ledger says remains**

Carries the L2TP and PPPoE Partial status. Twenty-four MUST gaps gated in [`rfc/short/rfc1661.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc1661.md): (1) no send-side clamp to a negotiated peer MRU, so frame and LCP Length bounds come from the fixed 1500-octet buffer ([`RFC1661-2-2`](#rfc1661-2-2), 5-1, 5.6-3); (2) the frame dispatcher has no LCP-phase gate and buffers early NCP frames instead of discarding out-of-phase packets ([`RFC1661-3.4-1`](#rfc1661-3.4-1), 3.5-4, 3.7-2); (3) no Protocol-Reject sender and no suppression of a packet type on RXJ+ ([`RFC1661-3.6-3`](#rfc1661-3.6-3), 4.3-3, 5.7-1, 5.7-2); (4) Configure-Request carries a constant Identifier and ze keeps no record of its last transmitted request, so Configure-Ack/Nak/Reject Identifier and option matching are unchecked and a rejected MRU or Magic-Number option reappears ([`RFC1661-5.1-3`](#rfc1661-5.1-3), 5.2-3, 5.2-4, 5.3-7, 5.4-3, 5.4-4, 5.4-5); (5) Code-Reject echoes the offending Identifier ([`RFC1661-5.6-2`](#rfc1661-5.6-2)); (6) a single-octet Protocol field is refused even with PFC negotiated ([`RFC1661-6.5-3`](#rfc1661-6.5-3)); (7) the Restart timer, Max-Terminate, Max-Configure and Max-Failure are not configurable and zrc is a no-op ([`RFC1661-4.6-1`](#rfc1661-4.6-1), 4.6-2, 4.6-3, 4.6-4, 4.4-2).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 26 | one part of the gated population |
| Annotated instead of tested | 40 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **66** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (26):** [`RFC1661-2-1`](#rfc1661-2-1), [`RFC1661-6-2`](#rfc1661-6-2), [`RFC1661-3.1-2`](#rfc1661-3.1-2), [`RFC1661-3.5-1`](#rfc1661-3.5-1), [`RFC1661-3.5-3`](#rfc1661-3.5-3), [`RFC1661-3.6-1`](#rfc1661-3.6-1), [`RFC1661-4.3-1`](#rfc1661-4.3-1), [`RFC1661-5.1-1`](#rfc1661-5.1-1), [`RFC1661-5.1-2`](#rfc1661-5.1-2), [`RFC1661-5.2-1`](#rfc1661-5.2-1), [`RFC1661-5.2-2`](#rfc1661-5.2-2), [`RFC1661-5.3-1`](#rfc1661-5.3-1), [`RFC1661-5.3-2`](#rfc1661-5.3-2), [`RFC1661-5.3-3`](#rfc1661-5.3-3), [`RFC1661-5.3-5`](#rfc1661-5.3-5), [`RFC1661-5.3-6`](#rfc1661-5.3-6), [`RFC1661-5.3-8`](#rfc1661-5.3-8), [`RFC1661-5.4-1`](#rfc1661-5.4-1), [`RFC1661-5.4-2`](#rfc1661-5.4-2), [`RFC1661-5.5-1`](#rfc1661-5.5-1), [`RFC1661-5.6-1`](#rfc1661-5.6-1), [`RFC1661-5.8-1`](#rfc1661-5.8-1), [`RFC1661-5.8-2`](#rfc1661-5.8-2), [`RFC1661-6.4-1`](#rfc1661-6.4-1), [`RFC1661-6.4-2`](#rfc1661-6.4-2), [`RFC1661-6.4-3`](#rfc1661-6.4-3)

**Annotated instead of tested (40):** [`RFC1661-2-2`](#rfc1661-2-2), [`RFC1661-5-1`](#rfc1661-5-1), [`RFC1661-3.1-1`](#rfc1661-3.1-1), [`RFC1661-3.4-1`](#rfc1661-3.4-1), [`RFC1661-3.5-2`](#rfc1661-3.5-2), [`RFC1661-3.5-4`](#rfc1661-3.5-4), [`RFC1661-3.6-2`](#rfc1661-3.6-2), [`RFC1661-3.6-3`](#rfc1661-3.6-3), [`RFC1661-3.7-1`](#rfc1661-3.7-1), [`RFC1661-3.7-2`](#rfc1661-3.7-2), [`RFC1661-4.3-2`](#rfc1661-4.3-2), [`RFC1661-4.3-3`](#rfc1661-4.3-3), [`RFC1661-5.1-3`](#rfc1661-5.1-3), [`RFC1661-5.2-3`](#rfc1661-5.2-3), [`RFC1661-5.2-4`](#rfc1661-5.2-4), [`RFC1661-5.3-4`](#rfc1661-5.3-4), [`RFC1661-5.3-7`](#rfc1661-5.3-7), [`RFC1661-5.4-3`](#rfc1661-5.4-3), [`RFC1661-5.4-4`](#rfc1661-5.4-4), [`RFC1661-5.4-5`](#rfc1661-5.4-5), [`RFC1661-5.6-2`](#rfc1661-5.6-2), [`RFC1661-5.6-3`](#rfc1661-5.6-3), [`RFC1661-5.7-1`](#rfc1661-5.7-1), [`RFC1661-5.7-2`](#rfc1661-5.7-2), [`RFC1661-5.7-3`](#rfc1661-5.7-3), [`RFC1661-5.7-4`](#rfc1661-5.7-4), [`RFC1661-5.9-1`](#rfc1661-5.9-1), [`RFC1661-5.9-2`](#rfc1661-5.9-2), [`RFC1661-6.2-1`](#rfc1661-6.2-1), [`RFC1661-6.5-1`](#rfc1661-6.5-1), [`RFC1661-6.5-2`](#rfc1661-6.5-2), [`RFC1661-6.5-3`](#rfc1661-6.5-3), [`RFC1661-6.6-1`](#rfc1661-6.6-1), [`RFC1661-6.6-2`](#rfc1661-6.6-2), [`RFC1661-4.6-1`](#rfc1661-4.6-1), [`RFC1661-4.6-2`](#rfc1661-4.6-2), [`RFC1661-4.6-3`](#rfc1661-4.6-3), [`RFC1661-4.6-4`](#rfc1661-4.6-4), [`RFC1661-4.4-1`](#rfc1661-4.4-1), [`RFC1661-4.4-2`](#rfc1661-4.4-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1661-2-1` | Protocol field: LSB of least-significant octet must equal 1; LSB of most-significant octet must equal 0; frames violating these rules must be treated as unrecognized Protocol (Section 2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC1661CompliantProtocolRecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L212). **negative:** `unit/verify` [`TestRFC1661NonCompliantProtocolTreatedUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L187) |
| `RFC1661-2-2` | Information field plus Padding must fit within peer's MRU (default 1500) (Section 2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze sizes every frame it emits from the fixed 1500-octet frameBufPool buffer and never consults the peer's negotiated MRU -- getFrameBuf (internal/component/l2tp/ppp/session_run.go:59-65) hands out MaxFrameLen bytes and sendCodeReject (internal/component/l2tp/ppp/session_run.go) truncates only against that buffer -- so a Code-Reject echoing a large packet exceeds a peer MRU negotiated below 1500. Disclosed in docs/features/rfc-status.md |
| `RFC1661-5-1` | LCP Length must not exceed the MRU of the link (Section 5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the LCP Length ze writes is backfilled from what fits the fixed 1500-octet buffer (WriteLCPPacket, internal/component/l2tp/ppp/lcp.go:91-99, fed by getFrameBuf at internal/component/l2tp/ppp/session_run.go:59-65); no send path reads s.negotiatedMRU, so the Length is bounded by the default MRU rather than by a smaller MRU the peer negotiated. Disclosed in docs/features/rfc-status.md |
| `RFC1661-6-1` | A negotiable Configuration Option received in a Configure-Request with an invalid or unrecognized Length should draw a Configure-Nak carrying the desired Configuration Option with an appropriate Length and Data (Section 6) | SHOULD | 6 | **positive:** `unit/verify` [`TestRFC1661ClientInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L148). **positive:** `unit/verify` [`TestRFC1661InvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L154). **positive:** `unit/verify` [`TestRFC1661LCPInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L318). **positive:** `unit/verify` [`TestRFC1661LCPWrongLengthMagicIsNakedNotRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L404). **positive:** `unit/verify` [`TestRFC1661ReplyListsEachOptionTypeOnce`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L724). **negative:** `unit/verify` [`TestRFC1661ClientInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L154). **negative:** `unit/verify` [`TestRFC1661InvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L159). **negative:** `unit/verify` [`TestRFC1661LCPInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L325). **negative:** `unit/verify` [`TestRFC1661LCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L465). **negative:** `unit/verify` [`TestRFC1661LCPWrongLengthMagicIsNakedNotRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L408). **negative:** `unit/verify` [`TestRFC1661ReplyListsEachOptionTypeOnce`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L728) |
| `RFC1661-6-2` | A Configuration Option whose Data is indicated by its Length to extend beyond the end of the Information field must cause the entire packet to be silently discarded without affecting the automaton (Section 6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC1661ClientRequestPastEndSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L104). **positive:** `unit/verify` [`TestRFC1661LCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L584). **positive:** `unit/verify` [`TestRFC1661LCPTruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L236). **positive:** `unit/verify` [`TestRFC1661NCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L655). **positive:** `unit/verify` [`TestRFC1661TruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L75). **negative:** `unit/verify` [`TestRFC1661ClientRequestPastEndSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L111). **negative:** `unit/verify` [`TestRFC1661LCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L590). **negative:** `unit/verify` [`TestRFC1661LCPTruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L243). **negative:** `unit/verify` [`TestRFC1661NCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L660). **negative:** `unit/verify` [`TestRFC1661TruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L81) |
| `RFC1661-3.1-1` | Each end must first send LCP packets to configure and test the data link (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC1661LCPPacketsSentFirst`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L280). **negative:** no negative test. **{single-polarity}:** run (internal/component/l2tp/ppp/session_run.go:182-197) drives the synthetic Initial->Closed->ReqSent sequence whose scr action puts an LCP Configure-Request on the wire before any other traffic, and the sole branch that skips it is the RFC 2661 Section 18 proxy-LCP path where the LAC has already run LCP, so there is no case in which ze opens a link with LCP packets unsent |
| `RFC1661-3.1-2` | PPP must send NCP packets to choose and configure network-layer protocols (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC1661NCPConfiguresEachFamilySeparately`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L308). **negative:** `unit/verify` [`TestRFC1661NoNCPPacketsWhenNoNetworkProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L347) |
| `RFC1661-3.4-1` | Non-LCP packets received during Link Establishment phase must be silently discarded (Section 3.4) | MUST | 3.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handleFrame (internal/component/l2tp/ppp/session_run.go:628-684) dispatches purely on the PPP Protocol field with no LCP-phase gate; an IPCP frame arriving during Link Establishment is copied into earlyNCPFrames (internal/component/l2tp/ppp/session_run.go:645-651) and replayed by runNCPPhase (internal/component/l2tp/ppp/ncp.go) instead of being silently discarded. Disclosed in docs/features/rfc-status.md |
| `RFC1661-3.5-1` | If peer authentication is desired, must request Authentication-Protocol during Link Establishment (Section 3.5) | MUST | 3.5 | **positive:** `unit/verify` [`TestRFC1661AuthProtocolRequestedDuringEstablishment`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L409). **negative:** `unit/verify` [`TestRFC1661NoAuthProtocolWhenNotDesired`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L440) |
| `RFC1661-3.5-2` | Exchange of link quality determination packets must not delay authentication indefinitely (Section 3.5) | MUST NOT | 3.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze implements no link-quality determination protocol. The LCP Quality-Protocol option (type 4) has no constant in internal/component/l2tp/ppp/lcp_options.go:14-21 and negotiatePeerOption (internal/component/l2tp/ppp/lcp_options.go) Configure-Rejects it as an unknown type; a grep for LQR, 0xC025 and Quality-Protocol across internal/ matches only that lcp_options.go comment naming type 4 as unimplemented |
| `RFC1661-3.5-3` | Advancement from Authentication to Network-Layer Protocol phase must not occur until authentication completes (Section 3.5) | MUST NOT | 3.5 | **positive:** `unit/verify` [`TestRFC1661NoNetworkPhaseUntilAuthCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L467). **negative:** `unit/verify` [`TestRFC1661NetworkPhaseRunsAfterAuthCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L500) |
| `RFC1661-3.5-4` | All other packets during Authentication phase must be silently discarded (Section 3.5) | MUST | 3.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** during the authentication wait waitCHAPLike (internal/component/l2tp/ppp/auth.go:288-306) hands every non-auth frame to handleFrame, which has no phase gate (internal/component/l2tp/ppp/session_run.go:628-684), so an NCP frame received in the Authentication phase is buffered into earlyNCPFrames and replayed rather than silently discarded. Disclosed in docs/features/rfc-status.md |
| `RFC1661-3.6-1` | Each network-layer protocol must be separately configured by appropriate NCP (Section 3.6) | MUST | 3.6 | **positive:** `unit/verify` [`TestRFC1661NCPConfiguresEachFamilySeparately`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L312). **negative:** `unit/verify` [`TestRFC1661NCPStatesAreIndependent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L364) |
| `RFC1661-3.6-2` | Network-layer packets received when corresponding NCP is not Opened must be silently discarded (Section 3.6) | MUST | 3.6 | **positive:** `unit/verify` [`TestRFC1661NetworkLayerPacketDiscardedBeforeNCPOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L382). **negative:** no negative test. **{single-polarity}:** handleFrame (internal/component/l2tp/ppp/session_run.go:637-683) dispatches only the three control protocols and drops IPv4 (0x0021) and IPv6 (0x0057) frames in every state, so no NCP state exists in which a network-layer packet is processed in userspace and there is no accepting counterpart to assert |
| `RFC1661-3.6-3` | Unsupported Protocol in Opened state must be returned in Protocol-Reject (Section 3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze has no Protocol-Reject sender. handleFrame (internal/component/l2tp/ppp/session_run.go:681-683) logs an unsupported PPP Protocol at debug level and drops the frame; LCPProtocolReject (internal/component/l2tp/ppp/lcp.go:25) appears only in LCPCodeName and in the receive-side codeToEvent mapping. Disclosed in docs/features/rfc-status.md |
| `RFC1661-3.7-1` | Receiver of Terminate-Request must not disconnect until at least one Restart time after sending Terminate-Ack (Section 3.7) | MUST | 3.7 | **positive:** `unit/verify` [`TestRFC1661TerminateAckSentAndLinkHeld`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L532). **negative:** no negative test. **{single-polarity}:** the Opened+RTR edge (internal/component/l2tp/ppp/ppp_fsm.go:393-394) lands in Stopping and handleLCPPacket emits EventSessionDown only for Closed or Stopped (internal/component/l2tp/ppp/session_run.go), so after sending a Terminate-Ack ze holds the link; there is no early-disconnect branch to assert |
| `RFC1661-3.7-2` | Non-LCP packets during Link Termination phase must be silently discarded (Section 3.7) | MUST | 3.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handleFrame (internal/component/l2tp/ppp/session_run.go:637-680) carries no LCP-state guard, so an IPCP or IPv6CP packet arriving while LCP sits in Closing or Stopping is still passed to handleNCPPacket (internal/component/l2tp/ppp/ncp.go) rather than silently discarded. Disclosed in docs/features/rfc-status.md |
| `RFC1661-4.3-1` | Implementation must be prepared to immediately renegotiate Configuration Options on RCR+/RCR- in Opened (Section 4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC1661RenegotiateOnConfigureRequestInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L587). **negative:** `unit/verify` [`TestRFC1661EchoDoesNotRenegotiate`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L620) |
| `RFC1661-4.3-2` | Implementation must be prepared to receive new Configure-Request without admin intervention after RTR (Section 4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC1661NewConfigureRequestAcceptedAfterTerminateRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L640). **negative:** no negative test. **{single-polarity}:** every post-RTR state in LCPDoTransition accepts a fresh RCR+ (internal/component/l2tp/ppp/ppp_fsm.go:295-296, :327-328, :359-360) and no code path consults an administrative flag before doing so, so there is no refusing counterpart to assert |
| `RFC1661-4.3-3` | Implementation must stop sending the offending packet type on RXJ+ (Section 4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** codeToEvent (internal/component/l2tp/ppp/session_run.go:703-704) maps a received Code-Reject or Protocol-Reject to RXJ+, and every RXJ+ edge in LCPDoTransition (internal/component/l2tp/ppp/ppp_fsm.go:227, :305, :337, :369, :399) carries no action; ze holds no per-packet-type suppression state, so it keeps sending the offending packet type. Disclosed in docs/features/rfc-status.md |
| `RFC1661-5.1-1` | An implementation wishing to open a connection must transmit a Configure-Request (Section 5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestRFC1661OpenTransmitsConfigureRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L666). **negative:** `unit/verify` [`TestRFC1661UpWithoutOpenSendsNoConfigureRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L697) |
| `RFC1661-5.1-2` | Upon reception of Configure-Request, an appropriate reply must be transmitted (Section 5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestRFC1661ConfigureAckEchoesOptionsVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L715). **negative:** `unit/verify` [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L779) |
| `RFC1661-5.1-3` | Identifier field must be changed whenever Options field changes or a valid reply is received (Section 5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** sendConfigureRequest (internal/component/l2tp/ppp/session_run.go) writes a constant Identifier of 1 into every LCP Configure-Request, so the Identifier changes neither when the option list changes nor after a valid reply arrives. Disclosed in docs/features/rfc-status.md |
| `RFC1661-5.2-1` | If all options recognizable and acceptable, must transmit Configure-Ack (Section 5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestRFC1661ConfigureAckEchoesOptionsVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L719). **negative:** `unit/verify` [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L783) |
| `RFC1661-5.2-2` | Acknowledged Configuration Options must not be reordered or modified (Section 5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestRFC1661ConfigureAckEchoesOptionsVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L722). **negative:** `unit/verify` [`TestRFC1661ConfigureAckDoesNotReorderOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L748) |
| `RFC1661-5.2-3` | On reception of Configure-Ack, Identifier must match last transmitted Configure-Request (Section 5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handleLCPPacket (internal/component/l2tp/ppp/session_run.go) feeds a Configure-Ack straight through codeToEvent (internal/component/l2tp/ppp/session_run.go:695-696) as RCA without comparing pkt.Identifier against the last transmitted Configure-Request; ze stores no last-sent Identifier. Disclosed in docs/features/rfc-status.md |
| `RFC1661-5.2-4` | Configure-Ack options must exactly match last transmitted Configure-Request (Section 5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze keeps no copy of the options it last sent -- sendConfigureRequest rebuilds them from s.maxMRU, s.magic and s.configuredAuthMethod on each call (internal/component/l2tp/ppp/session_run.go) -- so handleLCPPacket accepts a Configure-Ack without comparing its option list to the last transmitted Configure-Request. Disclosed in docs/features/rfc-status.md |
| `RFC1661-5.3-1` | If all options recognized but some values unacceptable, must transmit Configure-Nak (Section 5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC1661ConfigureNakSuggestsAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L869). **negative:** `unit/verify` [`TestRFC1661NoNakForAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L904) |
| `RFC1661-5.3-2` | Boolean options (no value) must use Configure-Reject instead of Configure-Nak (Section 5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC1661BooleanOptionsUseRejectNotNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L934). **negative:** `unit/verify` [`TestRFC1661ValuedOptionUsesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L956) |
| `RFC1661-5.3-3` | Single-instance option in Nak must be modified to acceptable value (Section 5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC1661ConfigureNakSuggestsAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L873). **negative:** `unit/verify` [`TestRFC1661NoNakForAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L907) |
| `RFC1661-5.3-4` | Multi-instance option in Nak must list all acceptable values (Section 5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the requirement is conditional on an option type that "can be listed more than once with different values", and RFC 1661 Section 6 says of its own set: "(None of the Configuration Options in this specification can be listed more than once.)" Ze implements types 1, 2, 3, 5, 7 and 8 (internal/component/l2tp/ppp/lcp_options.go:14-21), none of them multi-instance, so no input gives ze a list of acceptable values to send. The earlier reason here read that vacuity off NegotiatePeerOptions (internal/component/l2tp/ppp/lcp_options.go), which emits one Nak entry per received OPTION rather than per Type; keeping the reply to one entry per Type is appendUnlessListed (internal/component/l2tp/ppp/session_run.go), not this row |
| `RFC1661-5.3-5` | Nak option value fields must indicate values acceptable to the sender (Section 5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC1661NakValueIsAcceptable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L973). **negative:** `unit/verify` [`TestRFC1661RejectedValueStaysUnacceptable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L995) |
| `RFC1661-5.3-6` | Options from Configure-Request must not be reordered in Configure-Nak (Section 5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC1661NakPreservesRequestOrder`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1012). **negative:** `unit/verify` [`TestRFC1661NakOrderFollowsRequestNotAFixedOrder`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1031) |
| `RFC1661-5.3-7` | On reception of Configure-Nak, Identifier must match last transmitted Configure-Request (Section 5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a Configure-Nak reaches codeToEvent as RCN (internal/component/l2tp/ppp/session_run.go:697-698) with no Identifier comparison, and adjustAuthOnNakOrReject (internal/component/l2tp/ppp/auth.go:38-46) parses the option list without checking pkt.Identifier against the last transmitted Configure-Request. Disclosed in docs/features/rfc-status.md |
| `RFC1661-5.3-8` | Implementation must handle option length different from original Configure-Request (Section 5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC1661NakHandlesDifferentOptionLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1049). **negative:** `unit/verify` [`TestRFC1661NakTooShortOptionNotDecoded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1071) |
| `RFC1661-5.4-1` | If options not recognized or not acceptable for negotiation, must transmit Configure-Reject (Section 5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestRFC1661ClientUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L197). **positive:** `unit/verify` [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L786). **positive:** `unit/verify` [`TestRFC1661LCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L459). **positive:** `unit/verify` [`TestRFC1661NCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L526). **negative:** `unit/verify` [`TestRFC1661ClientAcceptsServerAuthProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L354). **negative:** `unit/verify` [`TestRFC1661ClientUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L202). **negative:** `unit/verify` [`TestRFC1661NCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L531). **negative:** `unit/verify` [`TestRFC1661NoConfigureRejectWhenAllOptionsRecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L817) |
| `RFC1661-5.4-2` | Configure-Reject options must not be reordered or modified (Section 5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestRFC1661ClientRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L233). **positive:** `unit/verify` [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L789). **positive:** `unit/verify` [`TestRFC1661LCPRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L808). **negative:** `unit/verify` [`TestRFC1661ClientRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L245). **negative:** `unit/verify` [`TestRFC1661ConfigureRejectDoesNotReorderOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L835). **negative:** `unit/verify` [`TestRFC1661LCPRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L819) |
| `RFC1661-5.4-3` | On reception of Configure-Reject, Identifier must match last transmitted Configure-Request (Section 5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a Configure-Reject reaches codeToEvent as RCN (internal/component/l2tp/ppp/session_run.go:697-698) with no Identifier comparison, and adjustAuthOnNakOrReject (internal/component/l2tp/ppp/auth.go:38-56) acts on it without checking pkt.Identifier against the last transmitted Configure-Request. Disclosed in docs/features/rfc-status.md |
| `RFC1661-5.4-4` | Configure-Reject options must be a subset of last transmitted Configure-Request (Section 5.4, Errata 543) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze retains no record of the options in its last transmitted Configure-Request (sendConfigureRequest rebuilds them per call, internal/component/l2tp/ppp/session_run.go), so a Configure-Reject is acted on without verifying its options are a subset of that request. Disclosed in docs/features/rfc-status.md |
| `RFC1661-5.4-5` | Next Configure-Request must not include any rejected options (Section 5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** only the Authentication-Protocol option is absorbed on a peer Configure-Reject -- adjustAuthOnNakOrReject clears configuredAuthMethod (internal/component/l2tp/ppp/auth.go:52-56) -- while sendConfigureRequest (internal/component/l2tp/ppp/session_run.go) rebuilds the MRU and Magic-Number options from s.maxMRU and s.magic on every call, so a rejected MRU or Magic-Number option reappears in the next Configure-Request. Disclosed in docs/features/rfc-status.md |
| `RFC1661-5.5-1` | Upon reception of Terminate-Request, a Terminate-Ack must be transmitted (Section 5.5) | MUST | 5.5 | **positive:** `unit/verify` [`TestRFC1661TerminateAckSentAndLinkHeld`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L537). **negative:** `unit/verify` [`TestRFC1661NoTerminateAckForTerminateAck`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L566) |
| `RFC1661-5.6-1` | Unknown code must be reported via Code-Reject (Section 5.6) | MUST | 5.6 | **positive:** `unit/verify` [`TestRFC1661CodeRejectForUnknownCode`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1093). **negative:** `unit/verify` [`TestRFC1661NoCodeRejectForKnownCode`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1125) |
| `RFC1661-5.6-2` | Identifier must be changed for each Code-Reject sent (Section 5.6) | MUST | 5.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** sendCodeReject (internal/component/l2tp/ppp/session_run.go) reuses the offending packet's Identifier for the Code-Reject instead of allocating a fresh one, so the Identifier does not change per Code-Reject sent. Disclosed in docs/features/rfc-status.md |
| `RFC1661-5.6-3` | Rejected-Packet in Code-Reject must be truncated to peer's MRU (Section 5.6) | MUST | 5.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** sendCodeReject (internal/component/l2tp/ppp/session_run.go) truncates the Rejected-Packet only against the fixed 1500-octet buffer from getFrameBuf (internal/component/l2tp/ppp/session_run.go:59-65) and never reads s.negotiatedMRU, so the copy is not bounded by a peer MRU negotiated below 1500. Disclosed in docs/features/rfc-status.md |
| `RFC1661-5.7-1` | In Opened state, unsupported Protocol must be reported via Protocol-Reject (Section 5.7) | MUST | 5.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze has no Protocol-Reject sender. In the Opened state an unsupported PPP Protocol reaches the drop path in handleFrame (internal/component/l2tp/ppp/session_run.go:681-683); no code writes an LCPProtocolReject packet (internal/component/l2tp/ppp/lcp.go:25 is referenced only by LCPCodeName and codeToEvent). Disclosed in docs/features/rfc-status.md |
| `RFC1661-5.7-2` | On reception of Protocol-Reject, must stop sending packets of indicated protocol (Section 5.7) | MUST | 5.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** codeToEvent (internal/component/l2tp/ppp/session_run.go:703-704) turns a received Protocol-Reject into RXJ+, whose FSM edge in Opened carries no action (internal/component/l2tp/ppp/ppp_fsm.go:399-400); ze records no rejected-protocol set, so it keeps sending packets of the indicated protocol. Disclosed in docs/features/rfc-status.md |
| `RFC1661-5.7-3` | Identifier must be changed for each Protocol-Reject sent (Section 5.7) | MUST | 5.7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never transmits a Protocol-Reject. A grep for LCPProtocolReject across internal/ matches only the constant (internal/component/l2tp/ppp/lcp.go:25), its name in LCPCodeName (internal/component/l2tp/ppp/lcp.go:119) and the receive-side codeToEvent mapping (internal/component/l2tp/ppp/session_run.go:703); no send path exists, so no Identifier is allocated for one |
| `RFC1661-5.7-4` | Rejected-Information in Protocol-Reject must be truncated to peer's MRU (Section 5.7) | MUST | 5.7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never transmits a Protocol-Reject, so no Rejected-Information field is built. A grep for LCPProtocolReject across internal/ matches only internal/component/l2tp/ppp/lcp.go:25, lcp.go:119 and the receive-side session_run.go:703 |
| `RFC1661-5.8-1` | On Echo-Request in Opened state, Echo-Reply must be transmitted (Section 5.8) | MUST | 5.8 | **positive:** `unit/verify` [`TestRFC1661EchoReplyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1146). **negative:** `unit/verify` [`TestRFC1661NoEchoOutsideOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1187) |
| `RFC1661-5.8-2` | Echo-Request and Echo-Reply must only be sent in Opened state (Section 5.8) | MUST | 5.8 | **positive:** `unit/verify` [`TestRFC1661NoEchoOutsideOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1191). **negative:** `unit/verify` [`TestRFC1661EchoReplyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1149) |
| `RFC1661-5.9-1` | Discard-Request must only be sent in Opened state (Section 5.9) | MUST | 5.9 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never transmits a Discard-Request. A grep for LCPDiscardRequest across internal/ matches only the constant (internal/component/l2tp/ppp/lcp.go:28), LCPCodeName (internal/component/l2tp/ppp/lcp.go:125) and the receive-side codeToEvent mapping (internal/component/l2tp/ppp/session_run.go:705) |
| `RFC1661-5.9-2` | Receiver must silently discard any Discard-Request (Section 5.9) | MUST | 5.9 | **positive:** `unit/verify` [`TestRFC1661DiscardRequestSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1213). **negative:** no negative test. **{single-polarity}:** the obligation covers ANY Discard-Request, so no conforming Discard-Request exists that must instead draw a reply, and no input can make the required silence wrong. The discrimination against a receiver that answers nothing at all is carried by TestRFC1661EchoReplyInOpened, which is tagged for the Echo requirements it actually drives (RFC1661-5.8-1, RFC1661-5.8-2), not for this one |
| `RFC1661-6.2-1` | Multiple Authentication-Protocol options must not be included in a single Configure-Request (Section 6.2) | MUST NOT | 6.2 | **positive:** `unit/verify` [`TestRFC1661SingleAuthProtocolOptionInRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1245). **negative:** no negative test. **{single-polarity}:** LCPOptions.AuthProto is a single uint16 (internal/component/l2tp/ppp/lcp_options.go) and BuildLocalConfigRequest appends the option once (internal/component/l2tp/ppp/lcp_options.go), so no input produces two Authentication-Protocol options and there is no violating case to assert |
| `RFC1661-6.4-1` | If implementation transmits Configure-Request with Magic-Number, must not Configure-Reject peer's Magic-Number option (Section 6.4) | MUST NOT | 6.4 | **positive:** `unit/verify` [`TestRFC1661ZeroMagicNumberRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1323). **negative:** `unit/verify` [`TestRFC1661UnknownOptionRejectedWhileMagicIsNot`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1295) |
| `RFC1661-6.4-2` | If Magic-Number negotiated, Echo/Discard packets must carry negotiated Magic-Number (Section 6.4) | MUST | 6.4 | **positive:** `unit/verify` [`TestRFC1661EchoReplyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1159). **negative:** `unit/verify` [`TestRFC1661EchoReplyDoesNotMirrorPeerMagic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1362) |
| `RFC1661-6.4-3` | Magic-Number of zero must always be Nak'd if not Rejected (Section 6.4) | MUST | 6.4 | **positive:** `unit/verify` [`TestRFC1661ClientNaksZeroMagicWithAValueOfItsOwn`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L402). **positive:** `unit/verify` [`TestRFC1661ZeroMagicNumberRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1315). **negative:** `unit/verify` [`TestRFC1661ClientNaksZeroMagicWithAValueOfItsOwn`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L408). **negative:** `unit/verify` [`TestRFC1661PeerMagicNumberAcked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1276) |
| `RFC1661-6.5-1` | All implementations must transmit packets with two-octet PPP Protocol fields by default (Section 6.5) | MUST | 6.5 | **positive:** `unit/verify` [`TestRFC1661TwoOctetProtocolField`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L232). **negative:** no negative test. **{single-polarity}:** this is a transmit obligation and WriteFrame has no compressed branch at all -- WriteFrame always writes the Protocol with binary.BigEndian.PutUint16 (internal/component/l2tp/ppp/frame.go:81-85), so no configuration or negotiated option produces a single-octet transmit to assert against. The positive test drives both PFC settings to show the option cannot change the encoder; the receive-side refusal of a one-octet Protocol is the separate RFC1661-6.5-3 |
| `RFC1661-6.5-2` | Compressed Protocol fields must not be transmitted unless PFC option negotiated (Section 6.5) | MUST NOT | 6.5 | **positive:** `unit/verify` [`TestRFC1661TwoOctetProtocolField`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L236). **negative:** no negative test. **{single-polarity}:** WriteFrame (internal/component/l2tp/ppp/frame.go:81-85) has no compressed-Protocol branch, so ze transmits an uncompressed Protocol field whether or not the option is negotiated and there is no compressed-transmit case to contrast |
| `RFC1661-6.5-3` | When PFC negotiated, must accept both single-octet and double-octet Protocol fields (Section 6.5) | MUST | 6.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ParseFrame (internal/component/l2tp/ppp/frame.go:59-67) accepts only the two-octet Protocol form, a choice documented at internal/component/l2tp/ppp/frame.go:44-58, so a single-octet Protocol field is rejected as a malformed frame even once Protocol-Field-Compression is negotiated. Disclosed in docs/features/rfc-status.md |
| `RFC1661-6.6-1` | All implementations must transmit frames with Address and Control fields appropriate to link framing (Section 6.6) | MUST | 6.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze performs no HDLC-like framing. It writes protocol-plus-payload frames to a /dev/ppp channel fd (WriteFrame, internal/component/l2tp/ppp/frame.go:81-85) and the kernel PPP driver supplies the Address and Control octets; a grep for 0xFF03 and HDLC across internal/ matches only two comments in internal/component/l2tp/ppp/lcp_options.go, one on desiredLCPOption and one on negotiatePeerOption, each recording that the kernel does the framing |
| `RFC1661-6.6-2` | Address and Control fields must not be compressed when sending LCP packets (Section 6.6) | MUST NOT | 6.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze emits no Address or Control field on any packet, LCP included: WriteFrame (internal/component/l2tp/ppp/frame.go:81-85) writes only the Protocol field and payload, and a grep for 0xFF03 and HDLC across internal/ matches only two comments in internal/component/l2tp/ppp/lcp_options.go, one on desiredLCPOption and one on negotiatePeerOption, each recording that the kernel supplies the framing |
| `RFC1661-4.6-1` | Restart timer must be configurable (Section 4.6) | MUST | 4.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Configure-Request retransmission timer is a fixed 3-second ticker created inline in run (internal/component/l2tp/ppp/session_run.go:217); StartSession (internal/component/l2tp/ppp/start_session.go) carries no restart-timer field and no YANG leaf sets one. Disclosed in docs/features/rfc-status.md |
| `RFC1661-4.6-2` | Max-Terminate must be configurable (Section 4.6) | MUST | 4.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze holds no Restart counter -- performAction treats LCPActIRC and LCPActZRC as no-ops (internal/component/l2tp/ppp/session_run.go) -- so there is no Max-Terminate value to configure; sendTerminateRequest (internal/component/l2tp/ppp/session_run.go) fires only from FSM edges. Disclosed in docs/features/rfc-status.md |
| `RFC1661-4.6-3` | Max-Configure must be configurable (Section 4.6) | MUST | 4.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze bounds LCP negotiation with the fixed 30-second defaultNegoTimeout (internal/component/l2tp/ppp/session_run.go, the constant and its use in run) rather than a Configure-Request transmission count, and the Restart counter LCPActIRC would initialize is a no-op (internal/component/l2tp/ppp/session_run.go, performAction), so no Max-Configure value exists to configure. Disclosed in docs/features/rfc-status.md |
| `RFC1661-4.6-4` | Max-Failure must be configurable (Section 4.6) | MUST | 4.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze counts no Configure-Naks sent; sendConfigureNakOrReject (internal/component/l2tp/ppp/session_run.go) picks Nak or Reject from the LCPNakOrReject verdict over NegotiatePeerOptions output on each request, so there is no Max-Failure value to configure and no threshold that converts a Nak into a Reject. Disclosed in docs/features/rfc-status.md |
| `RFC1661-4.4-1` | On irc action, timeout period must be reset to initial value when backoff is used (Section 4.4) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze applies no Restart timer backoff. The retransmission timer is a fixed-interval ticker, time.NewTicker(3 * time.Second) at internal/component/l2tp/ppp/session_run.go:217, with no growing timeout value, so the condition "when Restart timer backoff is used" never holds |
| `RFC1661-4.4-2` | On zrc action, timeout period must be set to appropriate value (Section 4.4) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** performAction treats LCPActZRC as a no-op (internal/component/l2tp/ppp/session_run.go), so the Opened+RTR edge that prescribes zrc (internal/component/l2tp/ppp/ppp_fsm.go:393-394) neither zeroes a Restart counter nor sets a timeout period. Disclosed in docs/features/rfc-status.md |
| `RFC1661-4.6-5` | Restart timer should default to 3 seconds (Section 4.6) | SHOULD | 4.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-4.6-6` | Max-Terminate should default to 2 transmissions (Section 4.6) | SHOULD | 4.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-4.6-7` | Max-Configure should default to 10 transmissions (Section 4.6) | SHOULD | 4.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-4.6-8` | Max-Failure should default to 5 transmissions (Section 4.6) | SHOULD | 4.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-1.2-1` | Provide capability of logging silently discarded packets and record in statistics counter (Section 1.2) | SHOULD | 1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-3.5-5` | Authentication should take place as soon as possible after link establishment (Section 3.5) | SHOULD | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-3.5-6` | If authentication fails, proceed to Link Termination phase (Section 3.5) | SHOULD | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-3.5-7` | Should not fail authentication simply due to timeout or lack of response (Section 3.5) | SHOULD NOT | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-3.6-4` | Avoid fixed timeouts when waiting for peers to configure NCP (Section 3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-3.7-3` | Signal physical-layer to disconnect on termination, especially on auth failure (Section 3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-3.7-4` | Sender of Terminate-Request should disconnect after Terminate-Ack or Restart counter expires (Section 3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-3.7-5` | Receiver of Terminate-Request should wait for peer to disconnect (Section 3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-5.1-4` | Configuration Options should not be included with default values in Configure-Request (Section 5.1) | SHOULD | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-5.6-4` | Upon Code-Reject of fundamental code, report problem and drop connection (Section 5.6) | SHOULD | 5.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-5.7-5` | Protocol-Reject received outside Opened state should be silently discarded (Section 5.7) | SHOULD | 5.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-5.8-3` | Echo-Request/Reply received outside Opened state should be silently discarded (Section 5.8) | SHOULD | 5.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-6.2-2` | Attempt most desirable authentication protocol first; if Nak'd, try next (Section 6.2) | SHOULD | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-6.4-4` | Magic-Number should be chosen in most random manner possible (Section 6.4) | SHOULD | 6.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-4.2-1` | Passive option should not be used on switched circuits (Section 4.2) | SHOULD NOT | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-3.5-8` | Link quality determination may occur concurrently with authentication (Section 3.5) | MAY | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-4.6-9` | Restart timer may use exponential backoff; each value should be at least 2x previous (Section 4.6) | MAY | 4.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-5.1-5` | Identifier may remain unchanged for retransmissions (Section 5.1) | MAY | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-5.3-9` | On Configure-Nak, options may be modified as specified (Section 5.3) | MAY | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-5.3-10` | Responder may append desired options to Configure-Nak to prompt peer (Section 5.3) | MAY | 5.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC1661-2-2`](#rfc1661-2-2) Information field plus Padding must fit within peer's MRU (default 1500) (Section 2) | {gap}, no test | ze sizes every frame it emits from the fixed 1500-octet frameBufPool buffer and never consults the peer's negotiated MRU -- getFrameBuf (internal/component/l2tp/ppp/session_run.go:59-65) hands out MaxFrameLen bytes and sendCodeReject (internal/component/l2tp/ppp/session_run.go) truncates only against that buffer -- so a Code-Reject echoing a large packet exceeds a peer MRU negotiated below 1500. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-5-1`](#rfc1661-5-1) LCP Length must not exceed the MRU of the link (Section 5) | {gap}, no test | the LCP Length ze writes is backfilled from what fits the fixed 1500-octet buffer (WriteLCPPacket, internal/component/l2tp/ppp/lcp.go:91-99, fed by getFrameBuf at internal/component/l2tp/ppp/session_run.go:59-65); no send path reads s.negotiatedMRU, so the Length is bounded by the default MRU rather than by a smaller MRU the peer negotiated. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-3.4-1`](#rfc1661-3.4-1) Non-LCP packets received during Link Establishment phase must be silently discarded (Section 3.4) | {gap}, no test | handleFrame (internal/component/l2tp/ppp/session_run.go:628-684) dispatches purely on the PPP Protocol field with no LCP-phase gate; an IPCP frame arriving during Link Establishment is copied into earlyNCPFrames (internal/component/l2tp/ppp/session_run.go:645-651) and replayed by runNCPPhase (internal/component/l2tp/ppp/ncp.go) instead of being silently discarded. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-3.5-2`](#rfc1661-3.5-2) Exchange of link quality determination packets must not delay authentication indefinitely (Section 3.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze implements no link-quality determination protocol. The LCP Quality-Protocol option (type 4) has no constant in internal/component/l2tp/ppp/lcp_options.go:14-21 and negotiatePeerOption (internal/component/l2tp/ppp/lcp_options.go) Configure-Rejects it as an unknown type; a grep for LQR, 0xC025 and Quality-Protocol across internal/ matches only that lcp_options.go comment naming type 4 as unimplemented |
| [`RFC1661-3.5-4`](#rfc1661-3.5-4) All other packets during Authentication phase must be silently discarded (Section 3.5) | {gap}, no test | during the authentication wait waitCHAPLike (internal/component/l2tp/ppp/auth.go:288-306) hands every non-auth frame to handleFrame, which has no phase gate (internal/component/l2tp/ppp/session_run.go:628-684), so an NCP frame received in the Authentication phase is buffered into earlyNCPFrames and replayed rather than silently discarded. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-3.6-3`](#rfc1661-3.6-3) Unsupported Protocol in Opened state must be returned in Protocol-Reject (Section 3.6) | {gap}, no test | ze has no Protocol-Reject sender. handleFrame (internal/component/l2tp/ppp/session_run.go:681-683) logs an unsupported PPP Protocol at debug level and drops the frame; LCPProtocolReject (internal/component/l2tp/ppp/lcp.go:25) appears only in LCPCodeName and in the receive-side codeToEvent mapping. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-3.7-2`](#rfc1661-3.7-2) Non-LCP packets during Link Termination phase must be silently discarded (Section 3.7) | {gap}, no test | handleFrame (internal/component/l2tp/ppp/session_run.go:637-680) carries no LCP-state guard, so an IPCP or IPv6CP packet arriving while LCP sits in Closing or Stopping is still passed to handleNCPPacket (internal/component/l2tp/ppp/ncp.go) rather than silently discarded. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-4.3-3`](#rfc1661-4.3-3) Implementation must stop sending the offending packet type on RXJ+ (Section 4.3) | {gap}, no test | codeToEvent (internal/component/l2tp/ppp/session_run.go:703-704) maps a received Code-Reject or Protocol-Reject to RXJ+, and every RXJ+ edge in LCPDoTransition (internal/component/l2tp/ppp/ppp_fsm.go:227, :305, :337, :369, :399) carries no action; ze holds no per-packet-type suppression state, so it keeps sending the offending packet type. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-5.1-3`](#rfc1661-5.1-3) Identifier field must be changed whenever Options field changes or a valid reply is received (Section 5.1) | {gap}, no test | sendConfigureRequest (internal/component/l2tp/ppp/session_run.go) writes a constant Identifier of 1 into every LCP Configure-Request, so the Identifier changes neither when the option list changes nor after a valid reply arrives. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-5.2-3`](#rfc1661-5.2-3) On reception of Configure-Ack, Identifier must match last transmitted Configure-Request (Section 5.2) | {gap}, no test | handleLCPPacket (internal/component/l2tp/ppp/session_run.go) feeds a Configure-Ack straight through codeToEvent (internal/component/l2tp/ppp/session_run.go:695-696) as RCA without comparing pkt.Identifier against the last transmitted Configure-Request; ze stores no last-sent Identifier. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-5.2-4`](#rfc1661-5.2-4) Configure-Ack options must exactly match last transmitted Configure-Request (Section 5.2) | {gap}, no test | ze keeps no copy of the options it last sent -- sendConfigureRequest rebuilds them from s.maxMRU, s.magic and s.configuredAuthMethod on each call (internal/component/l2tp/ppp/session_run.go) -- so handleLCPPacket accepts a Configure-Ack without comparing its option list to the last transmitted Configure-Request. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-5.3-4`](#rfc1661-5.3-4) Multi-instance option in Nak must list all acceptable values (Section 5.3) | no test | no test carries this requirement id; annotated {not-applicable}: the requirement is conditional on an option type that "can be listed more than once with different values", and RFC 1661 Section 6 says of its own set: "(None of the Configuration Options in this specification can be listed more than once.)" Ze implements types 1, 2, 3, 5, 7 and 8 (internal/component/l2tp/ppp/lcp_options.go:14-21), none of them multi-instance, so no input gives ze a list of acceptable values to send. The earlier reason here read that vacuity off NegotiatePeerOptions (internal/component/l2tp/ppp/lcp_options.go), which emits one Nak entry per received OPTION rather than per Type; keeping the reply to one entry per Type is appendUnlessListed (internal/component/l2tp/ppp/session_run.go), not this row |
| [`RFC1661-5.3-7`](#rfc1661-5.3-7) On reception of Configure-Nak, Identifier must match last transmitted Configure-Request (Section 5.3) | {gap}, no test | a Configure-Nak reaches codeToEvent as RCN (internal/component/l2tp/ppp/session_run.go:697-698) with no Identifier comparison, and adjustAuthOnNakOrReject (internal/component/l2tp/ppp/auth.go:38-46) parses the option list without checking pkt.Identifier against the last transmitted Configure-Request. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-5.4-3`](#rfc1661-5.4-3) On reception of Configure-Reject, Identifier must match last transmitted Configure-Request (Section 5.4) | {gap}, no test | a Configure-Reject reaches codeToEvent as RCN (internal/component/l2tp/ppp/session_run.go:697-698) with no Identifier comparison, and adjustAuthOnNakOrReject (internal/component/l2tp/ppp/auth.go:38-56) acts on it without checking pkt.Identifier against the last transmitted Configure-Request. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-5.4-4`](#rfc1661-5.4-4) Configure-Reject options must be a subset of last transmitted Configure-Request (Section 5.4, Errata 543) | {gap}, no test | ze retains no record of the options in its last transmitted Configure-Request (sendConfigureRequest rebuilds them per call, internal/component/l2tp/ppp/session_run.go), so a Configure-Reject is acted on without verifying its options are a subset of that request. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-5.4-5`](#rfc1661-5.4-5) Next Configure-Request must not include any rejected options (Section 5.4) | {gap}, no test | only the Authentication-Protocol option is absorbed on a peer Configure-Reject -- adjustAuthOnNakOrReject clears configuredAuthMethod (internal/component/l2tp/ppp/auth.go:52-56) -- while sendConfigureRequest (internal/component/l2tp/ppp/session_run.go) rebuilds the MRU and Magic-Number options from s.maxMRU and s.magic on every call, so a rejected MRU or Magic-Number option reappears in the next Configure-Request. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-5.6-2`](#rfc1661-5.6-2) Identifier must be changed for each Code-Reject sent (Section 5.6) | {gap}, no test | sendCodeReject (internal/component/l2tp/ppp/session_run.go) reuses the offending packet's Identifier for the Code-Reject instead of allocating a fresh one, so the Identifier does not change per Code-Reject sent. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-5.6-3`](#rfc1661-5.6-3) Rejected-Packet in Code-Reject must be truncated to peer's MRU (Section 5.6) | {gap}, no test | sendCodeReject (internal/component/l2tp/ppp/session_run.go) truncates the Rejected-Packet only against the fixed 1500-octet buffer from getFrameBuf (internal/component/l2tp/ppp/session_run.go:59-65) and never reads s.negotiatedMRU, so the copy is not bounded by a peer MRU negotiated below 1500. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-5.7-1`](#rfc1661-5.7-1) In Opened state, unsupported Protocol must be reported via Protocol-Reject (Section 5.7) | {gap}, no test | ze has no Protocol-Reject sender. In the Opened state an unsupported PPP Protocol reaches the drop path in handleFrame (internal/component/l2tp/ppp/session_run.go:681-683); no code writes an LCPProtocolReject packet (internal/component/l2tp/ppp/lcp.go:25 is referenced only by LCPCodeName and codeToEvent). Disclosed in docs/features/rfc-status.md |
| [`RFC1661-5.7-2`](#rfc1661-5.7-2) On reception of Protocol-Reject, must stop sending packets of indicated protocol (Section 5.7) | {gap}, no test | codeToEvent (internal/component/l2tp/ppp/session_run.go:703-704) turns a received Protocol-Reject into RXJ+, whose FSM edge in Opened carries no action (internal/component/l2tp/ppp/ppp_fsm.go:399-400); ze records no rejected-protocol set, so it keeps sending packets of the indicated protocol. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-5.7-3`](#rfc1661-5.7-3) Identifier must be changed for each Protocol-Reject sent (Section 5.7) | no test | no test carries this requirement id; annotated {not-applicable}: ze never transmits a Protocol-Reject. A grep for LCPProtocolReject across internal/ matches only the constant (internal/component/l2tp/ppp/lcp.go:25), its name in LCPCodeName (internal/component/l2tp/ppp/lcp.go:119) and the receive-side codeToEvent mapping (internal/component/l2tp/ppp/session_run.go:703); no send path exists, so no Identifier is allocated for one |
| [`RFC1661-5.7-4`](#rfc1661-5.7-4) Rejected-Information in Protocol-Reject must be truncated to peer's MRU (Section 5.7) | no test | no test carries this requirement id; annotated {not-applicable}: ze never transmits a Protocol-Reject, so no Rejected-Information field is built. A grep for LCPProtocolReject across internal/ matches only internal/component/l2tp/ppp/lcp.go:25, lcp.go:119 and the receive-side session_run.go:703 |
| [`RFC1661-5.9-1`](#rfc1661-5.9-1) Discard-Request must only be sent in Opened state (Section 5.9) | no test | no test carries this requirement id; annotated {not-applicable}: ze never transmits a Discard-Request. A grep for LCPDiscardRequest across internal/ matches only the constant (internal/component/l2tp/ppp/lcp.go:28), LCPCodeName (internal/component/l2tp/ppp/lcp.go:125) and the receive-side codeToEvent mapping (internal/component/l2tp/ppp/session_run.go:705) |
| [`RFC1661-6.5-3`](#rfc1661-6.5-3) When PFC negotiated, must accept both single-octet and double-octet Protocol fields (Section 6.5) | {gap}, no test | ParseFrame (internal/component/l2tp/ppp/frame.go:59-67) accepts only the two-octet Protocol form, a choice documented at internal/component/l2tp/ppp/frame.go:44-58, so a single-octet Protocol field is rejected as a malformed frame even once Protocol-Field-Compression is negotiated. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-6.6-1`](#rfc1661-6.6-1) All implementations must transmit frames with Address and Control fields appropriate to link framing (Section 6.6) | no test | no test carries this requirement id; annotated {not-applicable}: ze performs no HDLC-like framing. It writes protocol-plus-payload frames to a /dev/ppp channel fd (WriteFrame, internal/component/l2tp/ppp/frame.go:81-85) and the kernel PPP driver supplies the Address and Control octets; a grep for 0xFF03 and HDLC across internal/ matches only two comments in internal/component/l2tp/ppp/lcp_options.go, one on desiredLCPOption and one on negotiatePeerOption, each recording that the kernel does the framing |
| [`RFC1661-6.6-2`](#rfc1661-6.6-2) Address and Control fields must not be compressed when sending LCP packets (Section 6.6) | no test | no test carries this requirement id; annotated {not-applicable}: ze emits no Address or Control field on any packet, LCP included: WriteFrame (internal/component/l2tp/ppp/frame.go:81-85) writes only the Protocol field and payload, and a grep for 0xFF03 and HDLC across internal/ matches only two comments in internal/component/l2tp/ppp/lcp_options.go, one on desiredLCPOption and one on negotiatePeerOption, each recording that the kernel supplies the framing |
| [`RFC1661-4.6-1`](#rfc1661-4.6-1) Restart timer must be configurable (Section 4.6) | {gap}, no test | the Configure-Request retransmission timer is a fixed 3-second ticker created inline in run (internal/component/l2tp/ppp/session_run.go:217); StartSession (internal/component/l2tp/ppp/start_session.go) carries no restart-timer field and no YANG leaf sets one. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-4.6-2`](#rfc1661-4.6-2) Max-Terminate must be configurable (Section 4.6) | {gap}, no test | ze holds no Restart counter -- performAction treats LCPActIRC and LCPActZRC as no-ops (internal/component/l2tp/ppp/session_run.go) -- so there is no Max-Terminate value to configure; sendTerminateRequest (internal/component/l2tp/ppp/session_run.go) fires only from FSM edges. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-4.6-3`](#rfc1661-4.6-3) Max-Configure must be configurable (Section 4.6) | {gap}, no test | ze bounds LCP negotiation with the fixed 30-second defaultNegoTimeout (internal/component/l2tp/ppp/session_run.go, the constant and its use in run) rather than a Configure-Request transmission count, and the Restart counter LCPActIRC would initialize is a no-op (internal/component/l2tp/ppp/session_run.go, performAction), so no Max-Configure value exists to configure. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-4.6-4`](#rfc1661-4.6-4) Max-Failure must be configurable (Section 4.6) | {gap}, no test | ze counts no Configure-Naks sent; sendConfigureNakOrReject (internal/component/l2tp/ppp/session_run.go) picks Nak or Reject from the LCPNakOrReject verdict over NegotiatePeerOptions output on each request, so there is no Max-Failure value to configure and no threshold that converts a Nak into a Reject. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-4.4-1`](#rfc1661-4.4-1) On irc action, timeout period must be reset to initial value when backoff is used (Section 4.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze applies no Restart timer backoff. The retransmission timer is a fixed-interval ticker, time.NewTicker(3 * time.Second) at internal/component/l2tp/ppp/session_run.go:217, with no growing timeout value, so the condition "when Restart timer backoff is used" never holds |
| [`RFC1661-4.4-2`](#rfc1661-4.4-2) On zrc action, timeout period must be set to appropriate value (Section 4.4) | {gap}, no test | performAction treats LCPActZRC as a no-op (internal/component/l2tp/ppp/session_run.go), so the Opened+RTR edge that prescribes zrc (internal/component/l2tp/ppp/ppp_fsm.go:393-394) neither zeroes a Restart counter nor sets a timeout period. Disclosed in docs/features/rfc-status.md |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC1661-2-1`](#rfc1661-2-1)

Protocol field: LSB of least-significant octet must equal 1; LSB of most-significant octet must equal 0; frames violating these rules must be treated as unrecognized Protocol (Section 2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NonCompliantProtocolTreatedUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L187) | unit/verify | unproven |
| positive | [`TestRFC1661CompliantProtocolRecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L212) | unit/verify | unproven |

### [`RFC1661-2-2`](#rfc1661-2-2)

Information field plus Padding must fit within peer's MRU (default 1500) (Section 2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-2-2, so no unit is bound to it.

### [`RFC1661-5-1`](#rfc1661-5-1)

LCP Length must not exceed the MRU of the link (Section 5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-5-1, so no unit is bound to it.

### [`RFC1661-6-1`](#rfc1661-6-1)

A negotiable Configuration Option received in a Configure-Request with an invalid or unrecognized Length should draw a Configure-Nak carrying the desired Configuration Option with an appropriate Length and Data (Section 6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661InvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L159) | unit/verify | unproven |
| negative | [`TestRFC1661LCPInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L325) | unit/verify | unproven |
| negative | [`TestRFC1661LCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L465) | unit/verify | unproven |
| negative | [`TestRFC1661LCPWrongLengthMagicIsNakedNotRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L408) | unit/verify | unproven |
| negative | [`TestRFC1661ReplyListsEachOptionTypeOnce`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L728) | unit/verify | unproven |
| negative | [`TestRFC1661ClientInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L154) | unit/verify | unproven |
| positive | [`TestRFC1661InvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L154) | unit/verify | unproven |
| positive | [`TestRFC1661LCPInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L318) | unit/verify | unproven |
| positive | [`TestRFC1661LCPWrongLengthMagicIsNakedNotRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L404) | unit/verify | unproven |
| positive | [`TestRFC1661ReplyListsEachOptionTypeOnce`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L724) | unit/verify | unproven |
| positive | [`TestRFC1661ClientInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L148) | unit/verify | unproven |

### [`RFC1661-6-2`](#rfc1661-6-2)

A Configuration Option whose Data is indicated by its Length to extend beyond the end of the Information field must cause the entire packet to be silently discarded without affecting the automaton (Section 6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661LCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L590) | unit/verify | unproven |
| negative | [`TestRFC1661LCPTruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L243) | unit/verify | unproven |
| negative | [`TestRFC1661NCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L660) | unit/verify | unproven |
| negative | [`TestRFC1661TruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L81) | unit/verify | unproven |
| negative | [`TestRFC1661ClientRequestPastEndSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L111) | unit/verify | unproven |
| positive | [`TestRFC1661LCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L584) | unit/verify | unproven |
| positive | [`TestRFC1661LCPTruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L236) | unit/verify | unproven |
| positive | [`TestRFC1661NCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L655) | unit/verify | unproven |
| positive | [`TestRFC1661TruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L75) | unit/verify | unproven |
| positive | [`TestRFC1661ClientRequestPastEndSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L104) | unit/verify | unproven |

### [`RFC1661-3.1-1`](#rfc1661-3.1-1)

Each end must first send LCP packets to configure and test the data link (Section 3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1661LCPPacketsSentFirst`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L280) | unit/verify | unproven |

### [`RFC1661-3.1-2`](#rfc1661-3.1-2)

PPP must send NCP packets to choose and configure network-layer protocols (Section 3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NoNCPPacketsWhenNoNetworkProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L347) | unit/verify | unproven |
| positive | [`TestRFC1661NCPConfiguresEachFamilySeparately`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L308) | unit/verify | unproven |

### [`RFC1661-3.4-1`](#rfc1661-3.4-1)

Non-LCP packets received during Link Establishment phase must be silently discarded (Section 3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-3.4-1, so no unit is bound to it.

### [`RFC1661-3.5-1`](#rfc1661-3.5-1)

If peer authentication is desired, must request Authentication-Protocol during Link Establishment (Section 3.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NoAuthProtocolWhenNotDesired`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L440) | unit/verify | unproven |
| positive | [`TestRFC1661AuthProtocolRequestedDuringEstablishment`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L409) | unit/verify | unproven |

### [`RFC1661-3.5-2`](#rfc1661-3.5-2)

Exchange of link quality determination packets must not delay authentication indefinitely (Section 3.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-3.5-2, so no unit is bound to it.

### [`RFC1661-3.5-3`](#rfc1661-3.5-3)

Advancement from Authentication to Network-Layer Protocol phase must not occur until authentication completes (Section 3.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NetworkPhaseRunsAfterAuthCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L500) | unit/verify | unproven |
| positive | [`TestRFC1661NoNetworkPhaseUntilAuthCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L467) | unit/verify | unproven |

### [`RFC1661-3.5-4`](#rfc1661-3.5-4)

All other packets during Authentication phase must be silently discarded (Section 3.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-3.5-4, so no unit is bound to it.

### [`RFC1661-3.6-1`](#rfc1661-3.6-1)

Each network-layer protocol must be separately configured by appropriate NCP (Section 3.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NCPStatesAreIndependent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L364) | unit/verify | unproven |
| positive | [`TestRFC1661NCPConfiguresEachFamilySeparately`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L312) | unit/verify | unproven |

### [`RFC1661-3.6-2`](#rfc1661-3.6-2)

Network-layer packets received when corresponding NCP is not Opened must be silently discarded (Section 3.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1661NetworkLayerPacketDiscardedBeforeNCPOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L382) | unit/verify | unproven |

### [`RFC1661-3.6-3`](#rfc1661-3.6-3)

Unsupported Protocol in Opened state must be returned in Protocol-Reject (Section 3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-3.6-3, so no unit is bound to it.

### [`RFC1661-3.7-1`](#rfc1661-3.7-1)

Receiver of Terminate-Request must not disconnect until at least one Restart time after sending Terminate-Ack (Section 3.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1661TerminateAckSentAndLinkHeld`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L532) | unit/verify | unproven |

### [`RFC1661-3.7-2`](#rfc1661-3.7-2)

Non-LCP packets during Link Termination phase must be silently discarded (Section 3.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-3.7-2, so no unit is bound to it.

### [`RFC1661-4.3-1`](#rfc1661-4.3-1)

Implementation must be prepared to immediately renegotiate Configuration Options on RCR+/RCR- in Opened (Section 4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661EchoDoesNotRenegotiate`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L620) | unit/verify | unproven |
| positive | [`TestRFC1661RenegotiateOnConfigureRequestInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L587) | unit/verify | unproven |

### [`RFC1661-4.3-2`](#rfc1661-4.3-2)

Implementation must be prepared to receive new Configure-Request without admin intervention after RTR (Section 4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1661NewConfigureRequestAcceptedAfterTerminateRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L640) | unit/verify | unproven |

### [`RFC1661-4.3-3`](#rfc1661-4.3-3)

Implementation must stop sending the offending packet type on RXJ+ (Section 4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-4.3-3, so no unit is bound to it.

### [`RFC1661-5.1-1`](#rfc1661-5.1-1)

An implementation wishing to open a connection must transmit a Configure-Request (Section 5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661UpWithoutOpenSendsNoConfigureRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L697) | unit/verify | unproven |
| positive | [`TestRFC1661OpenTransmitsConfigureRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L666) | unit/verify | unproven |

### [`RFC1661-5.1-2`](#rfc1661-5.1-2)

Upon reception of Configure-Request, an appropriate reply must be transmitted (Section 5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L779) | unit/verify | unproven |
| positive | [`TestRFC1661ConfigureAckEchoesOptionsVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L715) | unit/verify | unproven |

### [`RFC1661-5.1-3`](#rfc1661-5.1-3)

Identifier field must be changed whenever Options field changes or a valid reply is received (Section 5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-5.1-3, so no unit is bound to it.

### [`RFC1661-5.2-1`](#rfc1661-5.2-1)

If all options recognizable and acceptable, must transmit Configure-Ack (Section 5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L783) | unit/verify | unproven |
| positive | [`TestRFC1661ConfigureAckEchoesOptionsVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L719) | unit/verify | unproven |

### [`RFC1661-5.2-2`](#rfc1661-5.2-2)

Acknowledged Configuration Options must not be reordered or modified (Section 5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661ConfigureAckDoesNotReorderOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L748) | unit/verify | unproven |
| positive | [`TestRFC1661ConfigureAckEchoesOptionsVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L722) | unit/verify | unproven |

### [`RFC1661-5.2-3`](#rfc1661-5.2-3)

On reception of Configure-Ack, Identifier must match last transmitted Configure-Request (Section 5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-5.2-3, so no unit is bound to it.

### [`RFC1661-5.2-4`](#rfc1661-5.2-4)

Configure-Ack options must exactly match last transmitted Configure-Request (Section 5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-5.2-4, so no unit is bound to it.

### [`RFC1661-5.3-1`](#rfc1661-5.3-1)

If all options recognized but some values unacceptable, must transmit Configure-Nak (Section 5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NoNakForAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L904) | unit/verify | unproven |
| positive | [`TestRFC1661ConfigureNakSuggestsAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L869) | unit/verify | unproven |

### [`RFC1661-5.3-2`](#rfc1661-5.3-2)

Boolean options (no value) must use Configure-Reject instead of Configure-Nak (Section 5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661ValuedOptionUsesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L956) | unit/verify | unproven |
| positive | [`TestRFC1661BooleanOptionsUseRejectNotNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L934) | unit/verify | unproven |

### [`RFC1661-5.3-3`](#rfc1661-5.3-3)

Single-instance option in Nak must be modified to acceptable value (Section 5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NoNakForAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L907) | unit/verify | unproven |
| positive | [`TestRFC1661ConfigureNakSuggestsAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L873) | unit/verify | unproven |

### [`RFC1661-5.3-4`](#rfc1661-5.3-4)

Multi-instance option in Nak must list all acceptable values (Section 5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-5.3-4, so no unit is bound to it.

### [`RFC1661-5.3-5`](#rfc1661-5.3-5)

Nak option value fields must indicate values acceptable to the sender (Section 5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661RejectedValueStaysUnacceptable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L995) | unit/verify | unproven |
| positive | [`TestRFC1661NakValueIsAcceptable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L973) | unit/verify | unproven |

### [`RFC1661-5.3-6`](#rfc1661-5.3-6)

Options from Configure-Request must not be reordered in Configure-Nak (Section 5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NakOrderFollowsRequestNotAFixedOrder`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1031) | unit/verify | unproven |
| positive | [`TestRFC1661NakPreservesRequestOrder`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1012) | unit/verify | unproven |

### [`RFC1661-5.3-7`](#rfc1661-5.3-7)

On reception of Configure-Nak, Identifier must match last transmitted Configure-Request (Section 5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-5.3-7, so no unit is bound to it.

### [`RFC1661-5.3-8`](#rfc1661-5.3-8)

Implementation must handle option length different from original Configure-Request (Section 5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NakTooShortOptionNotDecoded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1071) | unit/verify | unproven |
| positive | [`TestRFC1661NakHandlesDifferentOptionLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1049) | unit/verify | unproven |

### [`RFC1661-5.4-1`](#rfc1661-5.4-1)

If options not recognized or not acceptable for negotiation, must transmit Configure-Reject (Section 5.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L531) | unit/verify | unproven |
| negative | [`TestRFC1661NoConfigureRejectWhenAllOptionsRecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L817) | unit/verify | unproven |
| negative | [`TestRFC1661ClientAcceptsServerAuthProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L354) | unit/verify | unproven |
| negative | [`TestRFC1661ClientUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L202) | unit/verify | unproven |
| positive | [`TestRFC1661LCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L459) | unit/verify | unproven |
| positive | [`TestRFC1661NCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L526) | unit/verify | unproven |
| positive | [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L786) | unit/verify | unproven |
| positive | [`TestRFC1661ClientUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L197) | unit/verify | unproven |

### [`RFC1661-5.4-2`](#rfc1661-5.4-2)

Configure-Reject options must not be reordered or modified (Section 5.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661LCPRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L819) | unit/verify | unproven |
| negative | [`TestRFC1661ConfigureRejectDoesNotReorderOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L835) | unit/verify | unproven |
| negative | [`TestRFC1661ClientRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L245) | unit/verify | unproven |
| positive | [`TestRFC1661LCPRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L808) | unit/verify | unproven |
| positive | [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L789) | unit/verify | unproven |
| positive | [`TestRFC1661ClientRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L233) | unit/verify | unproven |

### [`RFC1661-5.4-3`](#rfc1661-5.4-3)

On reception of Configure-Reject, Identifier must match last transmitted Configure-Request (Section 5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-5.4-3, so no unit is bound to it.

### [`RFC1661-5.4-4`](#rfc1661-5.4-4)

Configure-Reject options must be a subset of last transmitted Configure-Request (Section 5.4, Errata 543)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-5.4-4, so no unit is bound to it.

### [`RFC1661-5.4-5`](#rfc1661-5.4-5)

Next Configure-Request must not include any rejected options (Section 5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-5.4-5, so no unit is bound to it.

### [`RFC1661-5.5-1`](#rfc1661-5.5-1)

Upon reception of Terminate-Request, a Terminate-Ack must be transmitted (Section 5.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NoTerminateAckForTerminateAck`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L566) | unit/verify | unproven |
| positive | [`TestRFC1661TerminateAckSentAndLinkHeld`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L537) | unit/verify | unproven |

### [`RFC1661-5.6-1`](#rfc1661-5.6-1)

Unknown code must be reported via Code-Reject (Section 5.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NoCodeRejectForKnownCode`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1125) | unit/verify | unproven |
| positive | [`TestRFC1661CodeRejectForUnknownCode`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1093) | unit/verify | unproven |

### [`RFC1661-5.6-2`](#rfc1661-5.6-2)

Identifier must be changed for each Code-Reject sent (Section 5.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-5.6-2, so no unit is bound to it.

### [`RFC1661-5.6-3`](#rfc1661-5.6-3)

Rejected-Packet in Code-Reject must be truncated to peer's MRU (Section 5.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-5.6-3, so no unit is bound to it.

### [`RFC1661-5.7-1`](#rfc1661-5.7-1)

In Opened state, unsupported Protocol must be reported via Protocol-Reject (Section 5.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-5.7-1, so no unit is bound to it.

### [`RFC1661-5.7-2`](#rfc1661-5.7-2)

On reception of Protocol-Reject, must stop sending packets of indicated protocol (Section 5.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-5.7-2, so no unit is bound to it.

### [`RFC1661-5.7-3`](#rfc1661-5.7-3)

Identifier must be changed for each Protocol-Reject sent (Section 5.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-5.7-3, so no unit is bound to it.

### [`RFC1661-5.7-4`](#rfc1661-5.7-4)

Rejected-Information in Protocol-Reject must be truncated to peer's MRU (Section 5.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-5.7-4, so no unit is bound to it.

### [`RFC1661-5.8-1`](#rfc1661-5.8-1)

On Echo-Request in Opened state, Echo-Reply must be transmitted (Section 5.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NoEchoOutsideOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1187) | unit/verify | unproven |
| positive | [`TestRFC1661EchoReplyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1146) | unit/verify | unproven |

### [`RFC1661-5.8-2`](#rfc1661-5.8-2)

Echo-Request and Echo-Reply must only be sent in Opened state (Section 5.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661EchoReplyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1149) | unit/verify | unproven |
| positive | [`TestRFC1661NoEchoOutsideOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1191) | unit/verify | unproven |

### [`RFC1661-5.9-1`](#rfc1661-5.9-1)

Discard-Request must only be sent in Opened state (Section 5.9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-5.9-1, so no unit is bound to it.

### [`RFC1661-5.9-2`](#rfc1661-5.9-2)

Receiver must silently discard any Discard-Request (Section 5.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1661DiscardRequestSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1213) | unit/verify | unproven |

### [`RFC1661-6.2-1`](#rfc1661-6.2-1)

Multiple Authentication-Protocol options must not be included in a single Configure-Request (Section 6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1661SingleAuthProtocolOptionInRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1245) | unit/verify | unproven |

### [`RFC1661-6.4-1`](#rfc1661-6.4-1)

If implementation transmits Configure-Request with Magic-Number, must not Configure-Reject peer's Magic-Number option (Section 6.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661UnknownOptionRejectedWhileMagicIsNot`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1295) | unit/verify | unproven |
| positive | [`TestRFC1661ZeroMagicNumberRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1323) | unit/verify | unproven |

### [`RFC1661-6.4-2`](#rfc1661-6.4-2)

If Magic-Number negotiated, Echo/Discard packets must carry negotiated Magic-Number (Section 6.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661EchoReplyDoesNotMirrorPeerMagic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1362) | unit/verify | unproven |
| positive | [`TestRFC1661EchoReplyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1159) | unit/verify | unproven |

### [`RFC1661-6.4-3`](#rfc1661-6.4-3)

Magic-Number of zero must always be Nak'd if not Rejected (Section 6.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661PeerMagicNumberAcked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1276) | unit/verify | unproven |
| negative | [`TestRFC1661ClientNaksZeroMagicWithAValueOfItsOwn`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L408) | unit/verify | unproven |
| positive | [`TestRFC1661ZeroMagicNumberRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1315) | unit/verify | unproven |
| positive | [`TestRFC1661ClientNaksZeroMagicWithAValueOfItsOwn`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L402) | unit/verify | unproven |

### [`RFC1661-6.5-1`](#rfc1661-6.5-1)

All implementations must transmit packets with two-octet PPP Protocol fields by default (Section 6.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1661TwoOctetProtocolField`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L232) | unit/verify | unproven |

### [`RFC1661-6.5-2`](#rfc1661-6.5-2)

Compressed Protocol fields must not be transmitted unless PFC option negotiated (Section 6.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1661TwoOctetProtocolField`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L236) | unit/verify | unproven |

### [`RFC1661-6.5-3`](#rfc1661-6.5-3)

When PFC negotiated, must accept both single-octet and double-octet Protocol fields (Section 6.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-6.5-3, so no unit is bound to it.

### [`RFC1661-6.6-1`](#rfc1661-6.6-1)

All implementations must transmit frames with Address and Control fields appropriate to link framing (Section 6.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-6.6-1, so no unit is bound to it.

### [`RFC1661-6.6-2`](#rfc1661-6.6-2)

Address and Control fields must not be compressed when sending LCP packets (Section 6.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-6.6-2, so no unit is bound to it.

### [`RFC1661-4.6-1`](#rfc1661-4.6-1)

Restart timer must be configurable (Section 4.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-4.6-1, so no unit is bound to it.

### [`RFC1661-4.6-2`](#rfc1661-4.6-2)

Max-Terminate must be configurable (Section 4.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-4.6-2, so no unit is bound to it.

### [`RFC1661-4.6-3`](#rfc1661-4.6-3)

Max-Configure must be configurable (Section 4.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-4.6-3, so no unit is bound to it.

### [`RFC1661-4.6-4`](#rfc1661-4.6-4)

Max-Failure must be configurable (Section 4.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-4.6-4, so no unit is bound to it.

### [`RFC1661-4.4-1`](#rfc1661-4.4-1)

On irc action, timeout period must be reset to initial value when backoff is used (Section 4.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-4.4-1, so no unit is bound to it.

### [`RFC1661-4.4-2`](#rfc1661-4.4-2)

On zrc action, timeout period must be set to appropriate value (Section 4.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-4.4-2, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 1661, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 1661, so its obligations are stated where they were written.
