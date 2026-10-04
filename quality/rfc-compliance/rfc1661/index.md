# RFC 1661 - The Point-to-Point Protocol (PPP)

Partial. Every requirement this repository extracted from RFC 1661, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 60.5% | 46 of 76 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 11.8% | 9 of 76 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 76 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 76 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 49.7% | 90 of 181 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 76 | of 101 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 6 | of 76 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 7.9% | 6 of 76 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 76 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 76 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 19.7% | 15 of 76 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 76 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
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
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 101 |
| Gated MUST-level | 76 |
| Not applicable, so out of scope | 6 |
| Declared gaps | 15 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 181 |
| Tagged units | 181 |
| Recorded audit verdicts | 56 |
| Discrimination records | 90 |
| Summary | `rfc/short/rfc1661.md` |
| Requirement shard | `rfc/requirements/rfc1661.md` |
| RFC text | `rfc/full/rfc1661.txt` |

## Enrolment

Enrolled: PPP LCP under L2TP and PPPoE. Ze implements framing, option negotiation, authentication phase selection, restart and termination actions, Code-Reject and Protocol-Reject, and Echo keepalive. The checklist records remaining behavior and evidence gaps; the requirement index binds the tagged tests.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

The full ten-state RFC 1661 Section 4.1 option-negotiation automaton, LCP packet and option codecs, Configure-Request/Ack/Nak/Reject negotiation with Reject-over-Nak-over-Ack precedence and verbatim option echo, Terminate-Request/Ack, Code-Reject, Echo keepalive with Magic-Number, two-octet Protocol framing, and the common NCP structure reused by IPCP and IPv6CP under L2TP and PPPoE. Tests bound per requirement in [`rfc/requirements/rfc1661.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc1661.md).

**What the ledger says remains**

Partial for L2TP and PPPoE. Fifteen MUST rows carry {gap}. Remaining gaps include the Code-Reject peer-MRU clamp and fresh Identifiers, RXJ+ suppression, and configurable restart limits. IPCP/IPv6CP admission and LCP-down reset are implemented; bounded lifecycle and native IPv4 restart evidence exists. Compressed Protocol reception remains unverified at the Linux framing boundary. Configure-Ack/Nak/Reject now validate the retained request before FSM or authentication changes; request Identifiers advance on changed options or valid replies, and rejected options stay removed. The buffer holds a 1500-octet Information field plus the two-octet Protocol field. Tests exercise reply correlation, real Restart-timer grace expiry, Echo/Terminate Identifiers, Magic-Number negotiation and full-size reception; validation and discrimination must be run before reporting those changes verified. No Discard-Request sender is implemented; its existing spec records a proposed optional feature, not an approval to build it.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 46 | one part of the gated population |
| Annotated (including scoped evidence) | 30 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **76** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (46):** [`RFC1661-2-1`](#rfc1661-2-1), [`RFC1661-5-2`](#rfc1661-5-2), [`RFC1661-6-2`](#rfc1661-6-2), [`RFC1661-3.1-2`](#rfc1661-3.1-2), [`RFC1661-3.5-1`](#rfc1661-3.5-1), [`RFC1661-3.5-3`](#rfc1661-3.5-3), [`RFC1661-3.6-1`](#rfc1661-3.6-1), [`RFC1661-3.6-3`](#rfc1661-3.6-3), [`RFC1661-4.3-1`](#rfc1661-4.3-1), [`RFC1661-5.1-1`](#rfc1661-5.1-1), [`RFC1661-5.1-2`](#rfc1661-5.1-2), [`RFC1661-5.1-3`](#rfc1661-5.1-3), [`RFC1661-5.2-1`](#rfc1661-5.2-1), [`RFC1661-5.2-2`](#rfc1661-5.2-2), [`RFC1661-5.2-3`](#rfc1661-5.2-3), [`RFC1661-5.2-4`](#rfc1661-5.2-4), [`RFC1661-5.3-1`](#rfc1661-5.3-1), [`RFC1661-5.3-2`](#rfc1661-5.3-2), [`RFC1661-5.3-3`](#rfc1661-5.3-3), [`RFC1661-5.3-5`](#rfc1661-5.3-5), [`RFC1661-5.3-6`](#rfc1661-5.3-6), [`RFC1661-5.3-7`](#rfc1661-5.3-7), [`RFC1661-5.3-8`](#rfc1661-5.3-8), [`RFC1661-5.4-1`](#rfc1661-5.4-1), [`RFC1661-5.4-2`](#rfc1661-5.4-2), [`RFC1661-5.4-3`](#rfc1661-5.4-3), [`RFC1661-5.4-4`](#rfc1661-5.4-4), [`RFC1661-5.4-5`](#rfc1661-5.4-5), [`RFC1661-5.5-1`](#rfc1661-5.5-1), [`RFC1661-5.6-1`](#rfc1661-5.6-1), [`RFC1661-5.7-1`](#rfc1661-5.7-1), [`RFC1661-5.7-4`](#rfc1661-5.7-4), [`RFC1661-5.8-1`](#rfc1661-5.8-1), [`RFC1661-5.8-2`](#rfc1661-5.8-2), [`RFC1661-6.4-1`](#rfc1661-6.4-1), [`RFC1661-6.4-2`](#rfc1661-6.4-2), [`RFC1661-6.4-3`](#rfc1661-6.4-3), [`RFC1661-4.4-2`](#rfc1661-4.4-2), [`RFC1661-5.5-2`](#rfc1661-5.5-2), [`RFC1661-5.8-4`](#rfc1661-5.8-4), [`RFC1661-5.8-5`](#rfc1661-5.8-5), [`RFC1661-6.1-1`](#rfc1661-6.1-1), [`RFC1661-6.4-5`](#rfc1661-6.4-5), [`RFC1661-6.4-6`](#rfc1661-6.4-6), [`RFC1661-6.4-7`](#rfc1661-6.4-7), [`RFC1661-6.4-8`](#rfc1661-6.4-8)

**Annotated (including scoped evidence) (30):** [`RFC1661-2-2`](#rfc1661-2-2), [`RFC1661-5-1`](#rfc1661-5-1), [`RFC1661-3.1-1`](#rfc1661-3.1-1), [`RFC1661-3.4-1`](#rfc1661-3.4-1), [`RFC1661-3.5-2`](#rfc1661-3.5-2), [`RFC1661-3.5-4`](#rfc1661-3.5-4), [`RFC1661-3.6-2`](#rfc1661-3.6-2), [`RFC1661-3.7-1`](#rfc1661-3.7-1), [`RFC1661-3.7-2`](#rfc1661-3.7-2), [`RFC1661-4.3-2`](#rfc1661-4.3-2), [`RFC1661-4.3-3`](#rfc1661-4.3-3), [`RFC1661-5.3-4`](#rfc1661-5.3-4), [`RFC1661-5.6-2`](#rfc1661-5.6-2), [`RFC1661-5.6-3`](#rfc1661-5.6-3), [`RFC1661-5.7-2`](#rfc1661-5.7-2), [`RFC1661-5.7-3`](#rfc1661-5.7-3), [`RFC1661-5.9-1`](#rfc1661-5.9-1), [`RFC1661-5.9-2`](#rfc1661-5.9-2), [`RFC1661-6.2-1`](#rfc1661-6.2-1), [`RFC1661-6.5-1`](#rfc1661-6.5-1), [`RFC1661-6.5-2`](#rfc1661-6.5-2), [`RFC1661-6.5-3`](#rfc1661-6.5-3), [`RFC1661-6.6-1`](#rfc1661-6.6-1), [`RFC1661-6.6-2`](#rfc1661-6.6-2), [`RFC1661-4.6-1`](#rfc1661-4.6-1), [`RFC1661-4.6-2`](#rfc1661-4.6-2), [`RFC1661-4.6-3`](#rfc1661-4.6-3), [`RFC1661-4.6-4`](#rfc1661-4.6-4), [`RFC1661-4.4-1`](#rfc1661-4.4-1), [`RFC1661-5.9-3`](#rfc1661-5.9-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1661-2-1` | All Protocols MUST be odd; the least significant bit of the least significant octet MUST equal "1".  Also, all Protocols MUST be assigned such that the least significant bit of the most significant octet equals "0".  Frames received which don't comply with these rules MUST be treated as having an unrecognized Protocol. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC1661CompliantProtocolRecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L238). **positive:** `unit/verify` [`TestRFC1661ProtocolWithOddHighOctetTreatedUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L48). **negative:** `unit/verify` [`TestRFC1661NonCompliantProtocolTreatedUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L203). **negative:** `unit/verify` [`TestRFC1661ProtocolWithOddHighOctetTreatedUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L49) |
| `RFC1661-2-2` | The maximum length for the Information field, including Padding, but not including the Protocol field, is termed the Maximum Receive Unit (MRU), which defaults to 1500 octets. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** getFrameBuf in internal/component/l2tp/ppp/session_run.go supplies MaxFrameBufLen bytes, enough for 1500 Information octets and the Protocol field. sendProtocolReject clamps to the peer MRU, but sendCodeReject still truncates only against that buffer, so a large Code-Reject can exceed a smaller peer MRU. Disclosed in docs/features/rfc-status.md |
| `RFC1661-5-1` | The Length MUST NOT exceed the MRU of the link. (§5) | MUST NOT | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** WriteLCPPacket in internal/component/l2tp/ppp/lcp.go backfills the packet length from its supplied payload; sendCodeReject in internal/component/l2tp/ppp/session_run.go bounds that payload by the default-sized Information field rather than a smaller negotiatedMRU. Protocol-Reject has a separate peer-MRU clamp. Disclosed in docs/features/rfc-status.md |
| `RFC1661-5-2` | When a packet is received with an invalid Length field, the packet is silently discarded without affecting the automaton. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestIPCPFrameRejectsNonSinglePacket`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/frame_test.go#L188). **positive:** `unit/verify` [`TestRFC1661InvalidLengthPacketSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_packet_length_test.go#L36). **negative:** `unit/verify` [`TestRFC1661ValidLengthWithPaddingIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_packet_length_test.go#L77) |
| `RFC1661-6-1` | If a negotiable Configuration Option is received in a Configure- Request, but with an invalid or unrecognized Length, a Configure- Nak SHOULD be transmitted which includes the desired Configuration Option with an appropriate Length and Data. (§6) | SHOULD | 6 | **positive:** `unit/verify` [`TestRFC1661ClientInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L167). **positive:** `unit/verify` [`TestRFC1661InvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L154). **positive:** `unit/verify` [`TestRFC1661LCPInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L318). **positive:** `unit/verify` [`TestRFC1661LCPWrongLengthMagicIsNakedNotRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L404). **positive:** `unit/verify` [`TestRFC1661ReplyListsEachOptionTypeOnce`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L744). **negative:** `unit/verify` [`TestRFC1661ClientInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L173). **negative:** `unit/verify` [`TestRFC1661InvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L159). **negative:** `unit/verify` [`TestRFC1661LCPInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L325). **negative:** `unit/verify` [`TestRFC1661LCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L465). **negative:** `unit/verify` [`TestRFC1661LCPWrongLengthMagicIsNakedNotRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L408). **negative:** `unit/verify` [`TestRFC1661ReplyListsEachOptionTypeOnce`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L748) |
| `RFC1661-6-2` | When the Data field is indicated by the Length to extend beyond the end of the Information field, the entire packet is silently discarded without affecting the automaton. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC1661ClientRequestPastEndSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L123). **positive:** `unit/verify` [`TestRFC1661LCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L584). **positive:** `unit/verify` [`TestRFC1661LCPTruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L236). **positive:** `unit/verify` [`TestRFC1661NCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L675). **positive:** `unit/verify` [`TestRFC1661TruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L75). **negative:** `unit/verify` [`TestRFC1661ClientRequestPastEndSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L130). **negative:** `unit/verify` [`TestRFC1661LCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L590). **negative:** `unit/verify` [`TestRFC1661LCPTruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L243). **negative:** `unit/verify` [`TestRFC1661NCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L680). **negative:** `unit/verify` [`TestRFC1661TruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L81) |
| `RFC1661-3.1-1` | In order to establish communications over a point-to-point link, each end of the PPP link MUST first send LCP packets to configure and test the data link. (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC1661RunSendsLCPBeforeAnyOtherProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L51). **negative:** no negative test. **{single-polarity}:** run (internal/component/l2tp/ppp/session_run.go:182-197) drives the synthetic Initial->Closed->ReqSent sequence whose scr action puts an LCP Configure-Request on the wire before any other traffic, and the sole branch that skips it is the RFC 2661 Section 18 proxy-LCP path where the LAC has already run LCP, so there is no case in which ze opens a link with LCP packets unsent |
| `RFC1661-3.1-2` | Then, PPP MUST send NCP packets to choose and configure one or more network-layer protocols. (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC1661NCPConfiguresEachFamilySeparately`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L331). **positive:** `unit/verify` [`TestRFC1661RunSendsNCPAfterLCPOpens`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L66). **negative:** `unit/verify` [`TestRFC1661NoNCPPacketsWhenNoNetworkProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L370) |
| `RFC1661-3.4-1` | Any non-LCP packets received during this phase MUST be silently discarded. (Section 3.4) | MUST | 3.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handleFrame in internal/component/l2tp/ppp/session_run.go now discards IPCP/IPv6CP before the current authenticated LCP lifetime is admitted. Bounded phase and lifetime tests exist; complete proof across all protocol types remains open. |
| `RFC1661-3.5-1` | If an implementation desires that the peer authenticate with some specific authentication protocol, then it MUST request the use of that authentication protocol during Link Establishment phase. (Section 3.5) | MUST | 3.5 | **positive:** `unit/verify` [`TestRFC1661AuthProtocolRequestedDuringEstablishment`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L442). **negative:** `unit/verify` [`TestRFC1661NoAuthProtocolWhenNotDesired`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L473) |
| `RFC1661-3.5-2` | An implementation MUST NOT allow the exchange of link quality determination packets to delay authentication indefinitely. (Section 3.5) | MUST NOT | 3.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze implements no link-quality determination protocol. The LCP Quality-Protocol option (type 4) has no constant in internal/component/l2tp/ppp/lcp_options.go:14-21 and negotiatePeerOption (internal/component/l2tp/ppp/lcp_options.go) Configure-Rejects it as an unknown type; a grep for LQR, 0xC025 and Quality-Protocol across internal/ matches only that lcp_options.go comment naming type 4 as unimplemented |
| `RFC1661-3.5-3` | Advancement from the Authentication phase to the Network-Layer Protocol phase MUST NOT occur until authentication has completed. (Section 3.5) | MUST NOT | 3.5 | **positive:** `unit/verify` [`TestRFC1661NoNetworkPhaseBeforeAuthenticationCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L85). **positive:** `unit/verify` [`TestRFC1661NoNetworkPhaseUntilAuthCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L500). **negative:** `unit/verify` [`TestRFC1661NetworkPhaseRunsAfterAuthCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L533). **negative:** `unit/verify` [`TestRFC1661NoNetworkPhaseBeforeAuthenticationCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L84) |
| `RFC1661-3.5-4` | All other packets received during this phase MUST be silently discarded. (Section 3.5) | MUST | 3.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handleFrame in internal/component/l2tp/ppp/session_run.go now discards IPCP/IPv6CP before authentication completes rather than retaining early frames. Bounded phase and lifetime tests exist; complete proof across all protocol types remains open. |
| `RFC1661-3.6-1` | Once PPP has finished the previous phases, each network-layer protocol (such as IP, IPX, or AppleTalk) MUST be separately configured by the appropriate Network Control Protocol (NCP). (Section 3.6) | MUST | 3.6 | **positive:** `unit/verify` [`TestRFC1661NCPConfiguresEachFamilySeparately`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L335). **negative:** `unit/verify` [`TestRFC1661NCPStatesAreIndependent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L387) |
| `RFC1661-3.6-2` | Any supported network-layer protocol packets received when the corresponding NCP is not in the Opened state MUST be silently discarded. (Section 3.6) | MUST | 3.6 | **positive:** `unit/verify` [`TestRFC1661NetworkLayerPacketDiscardedBeforeNCPOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L406). **negative:** no negative test. **{single-polarity}:** handleFrame (internal/component/l2tp/ppp/session_run.go) dispatches only the three control protocols, and rejectUnsupportedProtocol discards an IPv4 (0x0021) or IPv6 (0x0057) frame whose NCP ze supports in every NCP state, so no state exists in which a network-layer packet is processed in userspace and there is no accepting counterpart to assert |
| `RFC1661-3.6-3` | While LCP is in the Opened state, any protocol packet which is unsupported by the implementation MUST be returned in a Protocol- Reject (described later). (Section 3.6) | MUST | 3.6 | **positive:** `unit/verify` [`TestRFC1661UnsupportedProtocolRejectedInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L46). **negative:** `unit/verify` [`TestRFC1661UnsupportedProtocolNotRejectedBeforeOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L94) |
| `RFC1661-3.7-1` | The receiver of a Terminate-Request SHOULD wait for the peer to disconnect, and MUST NOT disconnect until at least one Restart time has passed after sending a Terminate-Ack. (§3.7) | MUST NOT | 3.7 | **positive:** `unit/verify` [`TestRFC1661TerminateAckHoldsLinkForRestartTime`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L372). **positive:** `unit/verify` [`TestRFC1661TerminateAckSentAndLinkHeld`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L565). **negative:** no negative test. **{single-polarity}:** the Opened+RTR edge (internal/component/l2tp/ppp/ppp_fsm.go:393-394) lands in Stopping and handleLCPPacket emits EventSessionDown only for Closed or Stopped (internal/component/l2tp/ppp/session_run.go), so after sending a Terminate-Ack ze holds the link; there is no early-disconnect branch to assert |
| `RFC1661-3.7-2` | Any non-LCP packets received during this phase MUST be silently discarded. (Section 3.7) | MUST | 3.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handleFrame and performAction in internal/component/l2tp/ppp/session_run.go now gate NCP on the current LCP lifetime and clear admission when that lifetime ends. Bounded lifecycle tests exist; complete requirement-level proof remains open. |
| `RFC1661-4.3-1` | The implementation MUST be prepared to immediately renegotiate the Configuration Options. (Section 4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC1661ConfigureRequestInOpenedRenegotiates`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L216). **positive:** `unit/verify` [`TestRFC1661RenegotiateOnConfigureRequestInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L625). **negative:** `unit/verify` [`TestRFC1661EchoDoesNotRenegotiate`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L658) |
| `RFC1661-4.3-2` | The implementation MUST be prepared to receive a new Configure-Request without network administrator intervention. (Section 4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC1661NewConfigureRequestAcceptedAfterTerminateRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L678). **positive:** `unit/verify` [`TestRFC1661StoppedAfterPeerTerminateTakesNewConfigureRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L257). **negative:** no negative test. **{single-polarity}:** every post-RTR state in LCPDoTransition accepts a fresh RCR+ (internal/component/l2tp/ppp/ppp_fsm.go:295-296, :327-328, :359-360) and no code path consults an administrative flag before doing so, so there is no refusing counterpart to assert |
| `RFC1661-4.3-3` | The implementation MUST stop sending the offending packet type. (Section 4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** codeToEvent and handleLCPPacket in internal/component/l2tp/ppp/session_run.go handle received rejects without retaining an offending-code suppression set. The Opened RXJ+ transition can send an Echo-Reply; it does not prevent later transmission of the rejected packet type. |
| `RFC1661-5.1-1` | An implementation wishing to open a connection MUST transmit a Configure-Request. (Section 5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestRFC1661OpenTransmitsConfigureRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L704). **positive:** `unit/verify` [`TestRFC1661OpenTransmitsConfigureRequestOnWire`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L420). **negative:** `unit/verify` [`TestRFC1661OpenTransmitsConfigureRequestOnWire`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L421). **negative:** `unit/verify` [`TestRFC1661UpWithoutOpenSendsNoConfigureRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L735) |
| `RFC1661-5.1-2` | Upon reception of a Configure-Request, an appropriate reply MUST be transmitted. (Section 5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestRFC1661ConfigureAckEchoesOptionsVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L753). **negative:** `unit/verify` [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L817) |
| `RFC1661-5.1-3` | The Identifier field MUST be changed whenever the contents of the Options field changes, and whenever a valid reply has been received for a previous request. (Section 5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L80). **positive:** `unit/verify` [`TestLCPConfigureRequestIdentifierLifecycle`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L492). **negative:** `unit/verify` [`TestClientLCPNakChangesIdentifierAndDropsStaleReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L146). **negative:** `unit/verify` [`TestLCPConfigureRequestIdentifierLifecycle`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L493) |
| `RFC1661-5.2-1` | If every Configuration Option received in a Configure-Request is recognizable and all values are acceptable, then the implementation MUST transmit a Configure-Ack. (Section 5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestRFC1661ConfigureAckEchoesOptionsVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L757). **negative:** `unit/verify` [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L821). **negative:** `unit/verify` [`TestRFC1877ConfigureAckEchoesAcceptable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_dns_options_test.go#L18) |
| `RFC1661-5.2-2` | The acknowledged Configuration Options MUST NOT be reordered or modified in any way. (§5.2) | MUST NOT | 5.2 | **positive:** `unit/verify` [`TestRFC1661ConfigureAckEchoesOptionsVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L760). **positive:** `unit/verify` [`TestRFC1877ConfigureAckEchoesAcceptable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_dns_options_test.go#L14). **negative:** `unit/verify` [`TestRFC1661ConfigureAckDoesNotReorderOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L786) |
| `RFC1661-5.2-3` | On reception of a Configure-Ack, the Identifier field MUST match that of the last transmitted Configure-Request. (Section 5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L18). **positive:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L399). **negative:** `unit/verify` [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L19). **negative:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L400) |
| `RFC1661-5.2-4` | Additionally, the Configuration Options in a Configure-Ack MUST exactly match those of the last transmitted Configure-Request. (Section 5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L20). **positive:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L401). **negative:** `unit/verify` [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L21). **negative:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L402) |
| `RFC1661-5.3-1` | If every instance of the received Configuration Options is recognizable, but some values are not acceptable, then the implementation MUST transmit a Configure-Nak. (§5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC1661ConfigureNakSuggestsAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L907). **negative:** `unit/verify` [`TestRFC1661NoNakForAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L942) |
| `RFC1661-5.3-2` | Options which have no value fields (boolean options) MUST use the Configure-Reject reply instead. (Section 5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC1661BooleanOptionsUseRejectNotNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L972). **positive:** `unit/verify` [`TestRFC1661WellFormedBooleanDeclinedWithReject`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L92). **negative:** `unit/verify` [`TestRFC1661ValuedOptionUsesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L994). **negative:** `unit/verify` [`TestRFC1661WellFormedBooleanDeclinedWithReject`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L93) |
| `RFC1661-5.3-3` | Each Configuration Option which is allowed only a single instance MUST be modified to a value acceptable to the Configure-Nak sender. (Section 5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC1661ConfigureNakSuggestsAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L911). **positive:** `unit/verify` [`TestRFC1661RepeatedOptionDrawsOneNakEntry`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L345). **negative:** `unit/verify` [`TestRFC1661NoNakForAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L945) |
| `RFC1661-5.3-4` | When a particular type of Configuration Option can be listed more than once with different values, the Configure-Nak MUST include a list of all values for that option which are acceptable to the Configure-Nak sender. (Section 5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the requirement is conditional on an option type that "can be listed more than once with different values", and RFC 1661 Section 6 says of its own set: "(None of the Configuration Options in this specification can be listed more than once.)" Ze implements types 1, 2, 3, 5, 7 and 8 (internal/component/l2tp/ppp/lcp_options.go:14-21), none of them multi-instance, so no input gives ze a list of acceptable values to send. The earlier reason here read that vacuity off NegotiatePeerOptions (internal/component/l2tp/ppp/lcp_options.go), which emits one Nak entry per received OPTION rather than per Type; keeping the reply to one entry per Type is appendUnlessListed (internal/component/l2tp/ppp/session_run.go), not this row |
| `RFC1661-5.3-5` | Any value fields for the option MUST indicate values acceptable to the Configure-Nak sender. (Section 5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC1661NakValueIsAcceptable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1011). **negative:** `unit/verify` [`TestRFC1661RejectedValueStaysUnacceptable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1033) |
| `RFC1661-5.3-6` | All acceptable Configuration Options are filtered out of the Configure-Nak, but otherwise the Configuration Options from the Configure-Request MUST NOT be reordered. (§5.3) | MUST NOT | 5.3 | **positive:** `unit/verify` [`TestRFC1661NakFiltersAcceptableOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L138). **positive:** `unit/verify` [`TestRFC1661NakPreservesRequestOrder`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1050). **negative:** `unit/verify` [`TestRFC1661NakFiltersAcceptableOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L139). **negative:** `unit/verify` [`TestRFC1661NakOrderFollowsRequestNotAFixedOrder`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1078) |
| `RFC1661-5.3-7` | On reception of a Configure-Nak, the Identifier field MUST match that of the last transmitted Configure-Request. (Section 5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestClientLCPNakChangesIdentifierAndDropsStaleReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L145). **positive:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L403). **negative:** `unit/verify` [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L22). **negative:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L404) |
| `RFC1661-5.3-8` | Since the Nak'd Option has been modified by the peer, the implementation MUST be able to handle an Option length which is different from the original Configure-Request. (Section 5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC1661NakHandlesDifferentOptionLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1099). **negative:** `unit/verify` [`TestRFC1661NakTooShortOptionNotDecoded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1121) |
| `RFC1661-5.4-1` | If some Configuration Options received in a Configure-Request are not recognizable or are not acceptable for negotiation (as configured by a network administrator), then the implementation MUST transmit a Configure-Reject. (Section 5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestRFC1661ClientUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L216). **positive:** `unit/verify` [`TestRFC1661ConfigureRejectForOptionRefusedByConfiguration`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L169). **positive:** `unit/verify` [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L824). **positive:** `unit/verify` [`TestRFC1661LCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L459). **positive:** `unit/verify` [`TestRFC1661NCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L526). **negative:** `unit/verify` [`TestRFC1661ClientAcceptsServerAuthProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L373). **negative:** `unit/verify` [`TestRFC1661ClientUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L221). **negative:** `unit/verify` [`TestRFC1661ConfigureRejectForOptionRefusedByConfiguration`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L170). **negative:** `unit/verify` [`TestRFC1661NCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L531). **negative:** `unit/verify` [`TestRFC1661NoConfigureRejectWhenAllOptionsRecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L855). **negative:** `unit/verify` [`TestRFC1877ConfigureRejectEchoesUnsupportedOnly`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_dns_options_test.go#L70) |
| `RFC1661-5.4-2` | All recognizable and negotiable Configuration Options are filtered out of the Configure-Reject, but otherwise the Configuration Options MUST NOT be reordered or modified in any way. (§5.4) | MUST NOT | 5.4 | **positive:** `unit/verify` [`TestRFC1661ClientRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L252). **positive:** `unit/verify` [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L827). **positive:** `unit/verify` [`TestRFC1661LCPRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L828). **positive:** `unit/verify` [`TestRFC1877ConfigureRejectEchoesUnsupportedOnly`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_dns_options_test.go#L67). **negative:** `unit/verify` [`TestRFC1661ClientRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L264). **negative:** `unit/verify` [`TestRFC1661ConfigureRejectDoesNotReorderOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L873). **negative:** `unit/verify` [`TestRFC1661LCPRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L839) |
| `RFC1661-5.4-3` | On reception of a Configure-Reject, the Identifier field MUST match that of the last transmitted Configure-Request. (Section 5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L76). **positive:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L405). **negative:** `unit/verify` [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L23). **negative:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L406) |
| `RFC1661-5.4-4` | Additionally, the Configuration Options in a Configure-Reject MUST be a subset of those in the last transmitted Configure- Request. (§5.4, erratum 543) | MUST | 5.4 | **positive:** `unit/verify` [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L77). **positive:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L407). **negative:** `unit/verify` [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L24). **negative:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L408) |
| `RFC1661-5.4-5` | Reception of a valid Configure-Reject indicates that when a new Configure-Request is sent, it MUST NOT include any of the Configuration Options listed in the Configure-Reject. (§5.4) | MUST NOT | 5.4 | **positive:** `unit/verify` [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L78). **positive:** `unit/verify` [`TestLCPRejectedOptionsStayRemoved`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L533). **negative:** `unit/verify` [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L79). **negative:** `unit/verify` [`TestLCPRejectedOptionsStayRemoved`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L534) |
| `RFC1661-5.5-1` | Upon reception of a Terminate-Request, a Terminate-Ack MUST be transmitted. (Section 5.5) | MUST | 5.5 | **positive:** `unit/verify` [`TestRFC1661TerminateAckSentAndLinkHeld`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L570). **negative:** `unit/verify` [`TestRFC1661NoTerminateAckForTerminateAck`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L604) |
| `RFC1661-5.6-1` | This MUST be reported back to the sender of the unknown Code by transmitting a Code- Reject. (Section 5.6) | MUST | 5.6 | **positive:** `unit/verify` [`TestRFC1661CodeRejectForUnknownCode`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1143). **negative:** `unit/verify` [`TestRFC1661NoCodeRejectForKnownCode`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1175) |
| `RFC1661-5.6-2` | The Identifier field MUST be changed for each Code-Reject sent. (Section 5.6) | MUST | 5.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** sendCodeReject (internal/component/l2tp/ppp/session_run.go) reuses the offending packet's Identifier for the Code-Reject instead of allocating a fresh one, so the Identifier does not change per Code-Reject sent. Disclosed in docs/features/rfc-status.md |
| `RFC1661-5.6-3` | The Rejected-Packet MUST be truncated to comply with the peer's established MRU. (Section 5.6) | MUST | 5.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** sendCodeReject in internal/component/l2tp/ppp/session_run.go truncates the Rejected-Packet against the 1500-octet Information field in getFrameBuf's MaxFrameBufLen buffer, not against a smaller negotiatedMRU. Disclosed in docs/features/rfc-status.md |
| `RFC1661-5.7-1` | If the LCP automaton is in the Opened state, then this MUST be reported back to the peer by transmitting a Protocol-Reject. (Section 5.7) | MUST | 5.7 | **positive:** `unit/verify` [`TestRFC1661UnsupportedProtocolRejectedInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L40). **negative:** `unit/verify` [`TestRFC1661UnsupportedProtocolNotRejectedBeforeOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L89) |
| `RFC1661-5.7-2` | Upon reception of a Protocol-Reject, the implementation MUST stop sending packets of the indicated protocol at the earliest opportunity. (Section 5.7) | MUST | 5.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handleLCPPacket in internal/component/l2tp/ppp/session_run.go maps the rejection to RXJ+ but records no rejected-protocol set. The FSM transition does not implement suppression of later packets of that protocol. |
| `RFC1661-5.7-3` | The Identifier field MUST be changed for each Protocol-Reject sent. (Section 5.7) | MUST | 5.7 | **positive:** `unit/verify` [`TestRFC1661ProtocolRejectIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L137). **negative:** no negative test. **{single-polarity}:** sendProtocolReject (internal/component/l2tp/ppp/session_run.go) advances protocolRejectID before every packet it writes, and no input asks ze to hold the Identifier still, so the obligation has no refusing counterpart to assert |
| `RFC1661-5.7-4` | The Rejected-Information MUST be truncated to comply with the peer's established MRU. (Section 5.7) | MUST | 5.7 | **positive:** `unit/verify` [`TestRFC1661ProtocolRejectInformationTruncatedToMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L166). **negative:** `unit/verify` [`TestRFC1661ProtocolRejectInformationKeptWhenItFits`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L201) |
| `RFC1661-5.8-1` | Upon reception of an Echo-Request in the LCP Opened state, an Echo-Reply MUST be transmitted. (Section 5.8) | MUST | 5.8 | **positive:** `unit/verify` [`TestRFC1661EchoReplyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1196). **negative:** `unit/verify` [`TestRFC1661NoEchoOutsideOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1237) |
| `RFC1661-5.8-2` | Echo-Request and Echo-Reply packets MUST only be sent in the LCP Opened state. (Section 5.8) | MUST | 5.8 | **positive:** `unit/verify` [`TestRFC1661EchoReplyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1199). **positive:** `unit/verify` [`TestRFC1661KeepaliveEchoOnlyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L125). **negative:** `unit/verify` [`TestClientLCPEchoBeforeOpenDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_transition_test.go#L115). **negative:** `unit/verify` [`TestRFC1661KeepaliveEchoOnlyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L126). **negative:** `unit/verify` [`TestRFC1661NoEchoOutsideOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1241) |
| `RFC1661-5.9-1` | Discard-Request packets MUST only be sent in the LCP Opened state. (Section 5.9) | MUST | 5.9 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never transmits a Discard-Request. A grep for LCPDiscardRequest across internal/ matches only the constant (internal/component/l2tp/ppp/lcp.go:28), LCPCodeName (internal/component/l2tp/ppp/lcp.go:125) and the receive-side codeToEvent mapping (internal/component/l2tp/ppp/session_run.go:705) |
| `RFC1661-5.9-2` | On reception, the receiver MUST silently discard any Discard- Request that it receives. (Section 5.9) | MUST | 5.9 | **positive:** `unit/verify` [`TestRFC1661DiscardRequestSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1266). **negative:** no negative test. **{single-polarity}:** the obligation covers ANY Discard-Request, so no conforming Discard-Request exists that must instead draw a reply, and no input can make the required silence wrong. The discrimination against a receiver that answers nothing at all is carried by TestRFC1661EchoReplyInOpened, which is tagged for the Echo requirements it actually drives (RFC1661-5.8-1, RFC1661-5.8-2), not for this one |
| `RFC1661-6.2-1` | An implementation MUST NOT include multiple Authentication- Protocol Configuration Options in its Configure-Request packets. (Section 6.2) | MUST NOT | 6.2 | **positive:** `unit/verify` [`TestRFC1661SingleAuthProtocolOptionInRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1298). **negative:** no negative test. **{single-polarity}:** LCPOptions.AuthProto is a single uint16 (internal/component/l2tp/ppp/lcp_options.go) and BuildLocalConfigRequest appends the option once (internal/component/l2tp/ppp/lcp_options.go), so no input produces two Authentication-Protocol options and there is no violating case to assert |
| `RFC1661-6.4-1` | If an implementation does transmit a Configure-Request with a Magic-Number Configuration Option, then it MUST NOT respond with a Configure-Reject when it receives a Configure-Request with a Magic-Number Configuration Option. (Section 6.4) | MUST NOT | 6.4 | **positive:** `unit/verify` [`TestRFC1661ZeroMagicNumberRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1376). **negative:** `unit/verify` [`TestRFC1661UnknownOptionRejectedWhileMagicIsNot`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1348) |
| `RFC1661-6.4-2` | If Magic-Number has been successfully negotiated, an implementation MUST transmit these packets with the Magic-Number field set to its negotiated Magic-Number. (Section 6.4) | MUST | 6.4 | **positive:** `unit/verify` [`TestEchoRequestCarriesNegotiatedMagic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L418). **positive:** `unit/verify` [`TestRFC1661EchoReplyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1208). **negative:** `unit/verify` [`TestEchoRequestCarriesNegotiatedMagic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L419). **negative:** `unit/verify` [`TestRFC1661EchoReplyDoesNotMirrorPeerMagic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1415) |
| `RFC1661-6.4-3` | A Magic-Number of zero is illegal and MUST always be Nak'd, if it is not Rejected outright. (Section 6.4) | MUST | 6.4 | **positive:** `unit/verify` [`TestRFC1661ClientNaksZeroMagicInAWellFormedRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_client_negotiation_test.go#L44). **positive:** `unit/verify` [`TestRFC1661ClientNaksZeroMagicWithAValueOfItsOwn`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L421). **positive:** `unit/verify` [`TestRFC1661ZeroMagicNumberRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1368). **negative:** `unit/verify` [`TestRFC1661ClientAcksANonZeroMagic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_client_negotiation_test.go#L82). **negative:** `unit/verify` [`TestRFC1661ClientNaksZeroMagicWithAValueOfItsOwn`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L427). **negative:** `unit/verify` [`TestRFC1661PeerMagicNumberAcked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1329) |
| `RFC1661-6.5-1` | By default, all implementations MUST transmit packets with two octet PPP Protocol fields. (Section 6.5) | MUST | 6.5 | **positive:** `unit/verify` [`TestRFC1661TwoOctetProtocolField`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L259). **negative:** no negative test. **{single-polarity}:** this is a transmit obligation and WriteFrame has no compressed branch at all -- WriteFrame always writes the Protocol with binary.BigEndian.PutUint16 (internal/component/l2tp/ppp/frame.go:81-85), so no configuration or negotiated option produces a single-octet transmit to assert against. The positive test drives both PFC settings to show the option cannot change the encoder; the receive-side refusal of a one-octet Protocol is the separate RFC1661-6.5-3 |
| `RFC1661-6.5-2` | Compressed Protocol fields MUST NOT be transmitted unless this Configuration Option has been negotiated. (Section 6.5) | MUST NOT | 6.5 | **positive:** `unit/verify` [`TestRFC1661TwoOctetProtocolField`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L263). **negative:** no negative test. **{single-polarity}:** WriteFrame (internal/component/l2tp/ppp/frame.go:81-85) has no compressed-Protocol branch, so ze transmits an uncompressed Protocol field whether or not the option is negotiated and there is no compressed-transmit case to contrast |
| `RFC1661-6.5-3` | When negotiated, PPP implementations MUST accept PPP packets with either double-octet or single-octet Protocol fields, and MUST NOT distinguish between them. (§6.5) | MUST | 6.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** whole-stack reception remains unverified. ParseFrame in internal/component/l2tp/ppp/frame.go reads a two-octet Protocol field, but that alone does not establish a failure after applicable directional negotiation and Linux PPP receive normalization. The negotiated kernel boundary needs proof. |
| `RFC1661-6.6-1` | By default, all implementations MUST transmit frames with Address and Control fields appropriate to the link framing. (Section 6.6) | MUST | 6.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze performs no HDLC-like framing. It writes protocol-plus-payload frames to a /dev/ppp channel fd (WriteFrame, internal/component/l2tp/ppp/frame.go:81-85) and the kernel PPP driver supplies the Address and Control octets; a grep for 0xFF03 and HDLC across internal/ matches only two comments in internal/component/l2tp/ppp/lcp_options.go, one on desiredLCPOption and one on negotiatePeerOption, each recording that the kernel does the framing |
| `RFC1661-6.6-2` | The Address and Control fields MUST NOT be compressed when sending any LCP packet. (Section 6.6) | MUST NOT | 6.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze emits no Address or Control field on any packet, LCP included: WriteFrame (internal/component/l2tp/ppp/frame.go:81-85) writes only the Protocol field and payload, and a grep for 0xFF03 and HDLC across internal/ matches only two comments in internal/component/l2tp/ppp/lcp_options.go, one on desiredLCPOption and one on negotiatePeerOption, each recording that the kernel supplies the framing |
| `RFC1661-4.6-1` | The Restart timer MUST be configurable (§4.6) | MUST | 4.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** armRestartTimer in internal/component/l2tp/ppp/session_run.go uses the fixed defaultRestartTimer of three seconds; StartSession in internal/component/l2tp/ppp/start_session.go carries no restart-timer field and no YANG leaf sets one. Disclosed in docs/features/rfc-status.md |
| `RFC1661-4.6-2` | Max-Terminate MUST be configurable (§4.6) | MUST | 4.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** applyTransition in internal/component/l2tp/ppp/session_run.go initializes restartCount from the fixed defaultMaxTerminate of two transmissions, with no session or YANG setting for that limit. Disclosed in docs/features/rfc-status.md |
| `RFC1661-4.6-3` | Max-Configure MUST be configurable (§4.6) | MUST | 4.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** applyTransition in internal/component/l2tp/ppp/session_run.go initializes restartCount from the fixed defaultMaxConfigure of ten transmissions, with no session or YANG setting for that limit. Disclosed in docs/features/rfc-status.md |
| `RFC1661-4.6-4` | Max-Failure MUST be configurable (§4.6) | MUST | 4.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze counts no Configure-Naks sent; sendConfigureNakOrReject (internal/component/l2tp/ppp/session_run.go) picks Nak or Reject from the LCPNakOrReject verdict over NegotiatePeerOptions output on each request, so there is no Max-Failure value to configure and no threshold that converts a Nak into a Reject. Disclosed in docs/features/rfc-status.md |
| `RFC1661-4.4-1` | In addition to setting the Restart counter, the implementation MUST set the timeout period to the initial value when Restart timer backoff is used. (Section 4.4) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze applies no Restart timer backoff. internal/component/l2tp/ppp/session_run.go resets its session Restart timer to the fixed defaultRestartTimer, so the condition "when Restart timer backoff is used" never holds |
| `RFC1661-4.4-2` | In addition to zeroing the Restart counter, the implementation MUST set the timeout period to an appropriate value. (Section 4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestLCPPeerTerminateRestartTimer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_restart_counter_test.go#L135). **negative:** `unit/verify` [`TestLCPPeerTerminateRestartTimer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_restart_counter_test.go#L136) |
| `RFC1661-5.5-2` | On transmission, the Identifier field MUST be changed whenever the content of the Data field changes, and whenever a valid reply has been received for a previous request. (§5.5) | MUST | 5.5 | **positive:** `unit/verify` [`TestTerminateRequestIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L59). **negative:** `unit/verify` [`TestTerminateRequestIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L60) |
| `RFC1661-5.8-4` | On transmission, the Identifier field MUST be changed whenever the content of the Data field changes, and whenever a valid reply has been received for a previous request. (§5.8) | MUST | 5.8 | **positive:** `unit/verify` [`TestEchoRequestIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L89). **negative:** `unit/verify` [`TestEchoRequestIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L90) |
| `RFC1661-5.8-5` | Until the Magic-Number Configuration Option has been successfully negotiated, the Magic- Number MUST be transmitted as zero. (§5.8) | MUST | 5.8 | **positive:** `unit/verify` [`TestMagicNumberZeroUntilNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L134). **negative:** `unit/verify` [`TestMagicNumberZeroUntilNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L135) |
| `RFC1661-5.9-3` | The Identifier field MUST be changed for each Discard-Request sent. (Section 5.9) | MUST | 5.9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze sends no Discard-Request, so no Identifier is drawn for one; plan/spec-ppp-discard-request-sender.md |
| `RFC1661-6.1-1` | If smaller packets are requested, an implementation MUST still be able to receive the full 1500 octet information field in case link synchronization is lost. (Section 6.1) | MUST | 6.1 | **positive:** `unit/verify` [`TestFullInformationFieldReceived`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L282). **negative:** `unit/verify` [`TestFullInformationFieldReceived`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L283) |
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
| `RFC1661-3.5-6` | If authentication fails, the authenticator SHOULD proceed instead to the Link Termination phase. (§3.5) | SHOULD | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-3.5-7` | An implementation SHOULD NOT fail authentication simply due to timeout or lack of response. (§3.5) | SHOULD NOT | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-3.6-4` | Because an implementation may initially use a significant amount of time for link quality determination, implementations SHOULD avoid fixed timeouts when waiting for their peers to configure a NCP. (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-3.7-3` | After the exchange of Terminate packets, the implementation SHOULD signal the physical-layer to disconnect in order to enforce the termination of the link, particularly in the case of an authentication failure. (§3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-3.7-4` | The sender of the Terminate-Request SHOULD disconnect after receiving a Terminate-Ack, or after the Restart counter expires. (§3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-3.7-5` | The receiver of a Terminate-Request SHOULD wait for the peer to disconnect (§3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-5.1-4` | Configuration Options SHOULD NOT be included with default values. (§5.1) | SHOULD NOT | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-5.6-4` | Upon reception of the Code-Reject of a code which is fundamental to this version of the protocol, the implementation SHOULD report the problem and drop the connection, since it is unlikely that the situation can be rectified automatically. (§5.6) | SHOULD | 5.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-5.7-5` | Protocol-Reject packets received in any state other than the LCP Opened state SHOULD be silently discarded. (§5.7) | SHOULD | 5.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-5.8-3` | Echo-Request and Echo-Reply packets received in any state other than the LCP Opened state SHOULD be silently discarded. (§5.8) | SHOULD | 5.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC1661-6.2-2` | Instead, it SHOULD attempt to configure the most desirable protocol first.  If that protocol is Configure-Nak'd, then the implementation SHOULD attempt the next most desirable protocol in the next Configure-Request. (§6.2) | SHOULD | 6.2 | **positive:** no positive test. **negative:** no negative test |
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
| [`RFC1661-2-2`](#rfc1661-2-2) The maximum length for the Information field, including Padding, but not including the Protocol field, is termed the Maximum Receive Unit (MRU), which defaults to 1500 octets. (§2) | {gap}, no test | getFrameBuf in internal/component/l2tp/ppp/session_run.go supplies MaxFrameBufLen bytes, enough for 1500 Information octets and the Protocol field. sendProtocolReject clamps to the peer MRU, but sendCodeReject still truncates only against that buffer, so a large Code-Reject can exceed a smaller peer MRU. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-5-1`](#rfc1661-5-1) The Length MUST NOT exceed the MRU of the link. (§5) | {gap}, no test | WriteLCPPacket in internal/component/l2tp/ppp/lcp.go backfills the packet length from its supplied payload; sendCodeReject in internal/component/l2tp/ppp/session_run.go bounds that payload by the default-sized Information field rather than a smaller negotiatedMRU. Protocol-Reject has a separate peer-MRU clamp. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-3.4-1`](#rfc1661-3.4-1) Any non-LCP packets received during this phase MUST be silently discarded. (Section 3.4) | {gap}, no test | handleFrame in internal/component/l2tp/ppp/session_run.go now discards IPCP/IPv6CP before the current authenticated LCP lifetime is admitted. Bounded phase and lifetime tests exist; complete proof across all protocol types remains open. |
| [`RFC1661-3.5-2`](#rfc1661-3.5-2) An implementation MUST NOT allow the exchange of link quality determination packets to delay authentication indefinitely. (Section 3.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze implements no link-quality determination protocol. The LCP Quality-Protocol option (type 4) has no constant in internal/component/l2tp/ppp/lcp_options.go:14-21 and negotiatePeerOption (internal/component/l2tp/ppp/lcp_options.go) Configure-Rejects it as an unknown type; a grep for LQR, 0xC025 and Quality-Protocol across internal/ matches only that lcp_options.go comment naming type 4 as unimplemented |
| [`RFC1661-3.5-4`](#rfc1661-3.5-4) All other packets received during this phase MUST be silently discarded. (Section 3.5) | {gap}, no test | handleFrame in internal/component/l2tp/ppp/session_run.go now discards IPCP/IPv6CP before authentication completes rather than retaining early frames. Bounded phase and lifetime tests exist; complete proof across all protocol types remains open. |
| [`RFC1661-3.7-2`](#rfc1661-3.7-2) Any non-LCP packets received during this phase MUST be silently discarded. (Section 3.7) | {gap}, no test | handleFrame and performAction in internal/component/l2tp/ppp/session_run.go now gate NCP on the current LCP lifetime and clear admission when that lifetime ends. Bounded lifecycle tests exist; complete requirement-level proof remains open. |
| [`RFC1661-4.3-3`](#rfc1661-4.3-3) The implementation MUST stop sending the offending packet type. (Section 4.3) | {gap}, no test | codeToEvent and handleLCPPacket in internal/component/l2tp/ppp/session_run.go handle received rejects without retaining an offending-code suppression set. The Opened RXJ+ transition can send an Echo-Reply; it does not prevent later transmission of the rejected packet type. |
| [`RFC1661-5.3-4`](#rfc1661-5.3-4) When a particular type of Configuration Option can be listed more than once with different values, the Configure-Nak MUST include a list of all values for that option which are acceptable to the Configure-Nak sender. (Section 5.3) | no test | no test carries this requirement id; annotated {not-applicable}: the requirement is conditional on an option type that "can be listed more than once with different values", and RFC 1661 Section 6 says of its own set: "(None of the Configuration Options in this specification can be listed more than once.)" Ze implements types 1, 2, 3, 5, 7 and 8 (internal/component/l2tp/ppp/lcp_options.go:14-21), none of them multi-instance, so no input gives ze a list of acceptable values to send. The earlier reason here read that vacuity off NegotiatePeerOptions (internal/component/l2tp/ppp/lcp_options.go), which emits one Nak entry per received OPTION rather than per Type; keeping the reply to one entry per Type is appendUnlessListed (internal/component/l2tp/ppp/session_run.go), not this row |
| [`RFC1661-5.6-2`](#rfc1661-5.6-2) The Identifier field MUST be changed for each Code-Reject sent. (Section 5.6) | {gap}, no test | sendCodeReject (internal/component/l2tp/ppp/session_run.go) reuses the offending packet's Identifier for the Code-Reject instead of allocating a fresh one, so the Identifier does not change per Code-Reject sent. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-5.6-3`](#rfc1661-5.6-3) The Rejected-Packet MUST be truncated to comply with the peer's established MRU. (Section 5.6) | {gap}, no test | sendCodeReject in internal/component/l2tp/ppp/session_run.go truncates the Rejected-Packet against the 1500-octet Information field in getFrameBuf's MaxFrameBufLen buffer, not against a smaller negotiatedMRU. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-5.7-2`](#rfc1661-5.7-2) Upon reception of a Protocol-Reject, the implementation MUST stop sending packets of the indicated protocol at the earliest opportunity. (Section 5.7) | {gap}, no test | handleLCPPacket in internal/component/l2tp/ppp/session_run.go maps the rejection to RXJ+ but records no rejected-protocol set. The FSM transition does not implement suppression of later packets of that protocol. |
| [`RFC1661-5.9-1`](#rfc1661-5.9-1) Discard-Request packets MUST only be sent in the LCP Opened state. (Section 5.9) | no test | no test carries this requirement id; annotated {not-applicable}: ze never transmits a Discard-Request. A grep for LCPDiscardRequest across internal/ matches only the constant (internal/component/l2tp/ppp/lcp.go:28), LCPCodeName (internal/component/l2tp/ppp/lcp.go:125) and the receive-side codeToEvent mapping (internal/component/l2tp/ppp/session_run.go:705) |
| [`RFC1661-6.5-3`](#rfc1661-6.5-3) When negotiated, PPP implementations MUST accept PPP packets with either double-octet or single-octet Protocol fields, and MUST NOT distinguish between them. (§6.5) | {gap}, no test | whole-stack reception remains unverified. ParseFrame in internal/component/l2tp/ppp/frame.go reads a two-octet Protocol field, but that alone does not establish a failure after applicable directional negotiation and Linux PPP receive normalization. The negotiated kernel boundary needs proof. |
| [`RFC1661-6.6-1`](#rfc1661-6.6-1) By default, all implementations MUST transmit frames with Address and Control fields appropriate to the link framing. (Section 6.6) | no test | no test carries this requirement id; annotated {not-applicable}: ze performs no HDLC-like framing. It writes protocol-plus-payload frames to a /dev/ppp channel fd (WriteFrame, internal/component/l2tp/ppp/frame.go:81-85) and the kernel PPP driver supplies the Address and Control octets; a grep for 0xFF03 and HDLC across internal/ matches only two comments in internal/component/l2tp/ppp/lcp_options.go, one on desiredLCPOption and one on negotiatePeerOption, each recording that the kernel does the framing |
| [`RFC1661-6.6-2`](#rfc1661-6.6-2) The Address and Control fields MUST NOT be compressed when sending any LCP packet. (Section 6.6) | no test | no test carries this requirement id; annotated {not-applicable}: ze emits no Address or Control field on any packet, LCP included: WriteFrame (internal/component/l2tp/ppp/frame.go:81-85) writes only the Protocol field and payload, and a grep for 0xFF03 and HDLC across internal/ matches only two comments in internal/component/l2tp/ppp/lcp_options.go, one on desiredLCPOption and one on negotiatePeerOption, each recording that the kernel supplies the framing |
| [`RFC1661-4.6-1`](#rfc1661-4.6-1) The Restart timer MUST be configurable (§4.6) | {gap}, no test | armRestartTimer in internal/component/l2tp/ppp/session_run.go uses the fixed defaultRestartTimer of three seconds; StartSession in internal/component/l2tp/ppp/start_session.go carries no restart-timer field and no YANG leaf sets one. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-4.6-2`](#rfc1661-4.6-2) Max-Terminate MUST be configurable (§4.6) | {gap}, no test | applyTransition in internal/component/l2tp/ppp/session_run.go initializes restartCount from the fixed defaultMaxTerminate of two transmissions, with no session or YANG setting for that limit. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-4.6-3`](#rfc1661-4.6-3) Max-Configure MUST be configurable (§4.6) | {gap}, no test | applyTransition in internal/component/l2tp/ppp/session_run.go initializes restartCount from the fixed defaultMaxConfigure of ten transmissions, with no session or YANG setting for that limit. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-4.6-4`](#rfc1661-4.6-4) Max-Failure MUST be configurable (§4.6) | {gap}, no test | ze counts no Configure-Naks sent; sendConfigureNakOrReject (internal/component/l2tp/ppp/session_run.go) picks Nak or Reject from the LCPNakOrReject verdict over NegotiatePeerOptions output on each request, so there is no Max-Failure value to configure and no threshold that converts a Nak into a Reject. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-4.4-1`](#rfc1661-4.4-1) In addition to setting the Restart counter, the implementation MUST set the timeout period to the initial value when Restart timer backoff is used. (Section 4.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze applies no Restart timer backoff. internal/component/l2tp/ppp/session_run.go resets its session Restart timer to the fixed defaultRestartTimer, so the condition "when Restart timer backoff is used" never holds |
| [`RFC1661-5.9-3`](#rfc1661-5.9-3) The Identifier field MUST be changed for each Discard-Request sent. (Section 5.9) | {gap}, no test | Ze sends no Discard-Request, so no Identifier is drawn for one; plan/spec-ppp-discard-request-sender.md |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC1661-2-1`](#rfc1661-2-1)

All Protocols MUST be odd; the least significant bit of the least significant octet MUST equal "1".  Also, all Protocols MUST be assigned such that the least significant bit of the most significant octet equals "0".  Frames received which don't comply with these rules MUST be treated as having an unrecognized Protocol. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Receive, low-octet rule: TestRFC1661NonCompliantProtocolTreatedUnrecognized (0xC020 -> Protocol-Reject). Receive, high-octet rule: TestRFC1661ProtocolWithOddHighOctetTreatedUnrecognized sends an Echo-Request under 0xC121 in Opened, red unless exactly one Protocol-Reject naming 0xC121, no Echo-Reply, state Opened. Transmit side: the same unit checks both parity bits on every Protocol constant ze writes (LCP, PAP, CHAP, IPCP, IPV6CP, IPv4, IPv6).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661ProtocolWithOddHighOctetTreatedUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L49) | unit/verify | revert, verified |
| negative | [`TestRFC1661NonCompliantProtocolTreatedUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L203) | unit/verify | revert, verified |
| positive | [`TestRFC1661ProtocolWithOddHighOctetTreatedUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L48) | unit/verify | revert, verified |
| positive | [`TestRFC1661CompliantProtocolRecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L238) | unit/verify | revert, verified |

### [`RFC1661-2-2`](#rfc1661-2-2)

The maximum length for the Information field, including Padding, but not including the Protocol field, is termed the Maximum Receive Unit (MRU), which defaults to 1500 octets. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-2-2, so no unit is bound to it.

### [`RFC1661-5-1`](#rfc1661-5-1)

The Length MUST NOT exceed the MRU of the link. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-5-1, so no unit is bound to it.

### [`RFC1661-5-2`](#rfc1661-5-2)

When a packet is received with an invalid Length field, the packet is silently discarded without affecting the automaton. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge ppp4 2026-09-30. Row text is the verbatim RFC 1661 Section 5 sentence (rfc/full/rfc1661.txt lines 1582-1584), MUST under D-3, listed in extraction section 5 unsourced-ids. TestRFC1661InvalidLengthPacketSilentlyDiscarded (+): an Opened session with a matching peer Magic, through handleFrame, gets an Echo-Request that would draw a reply, with Length 3 (below the header) and Length 16 (past the 8 octets present): no frame written, still Opened, no lifecycle event; the buffer is otherwise valid, so only the Length decides. TestRFC1661ValidLengthWithPaddingIsAnswered (-): Length 8 plus 4 padding octets is answered (Echo-Reply Id 0x62, still Opened), so an over-eager discard turns it red. TestIPCPFrameRejectsNonSinglePacket (+) proves ParseLCPPacket's errLCPLengthMismatch detection on an IPCP packet; its claim names only the detection. Records: ParseLCPPacket revert, observed red, all three units.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661ValidLengthWithPaddingIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_packet_length_test.go#L77) | unit/verify | revert, verified |
| positive | [`TestIPCPFrameRejectsNonSinglePacket`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/frame_test.go#L188) | unit/verify | revert, verified |
| positive | [`TestRFC1661InvalidLengthPacketSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_packet_length_test.go#L36) | unit/verify | revert, verified |

### [`RFC1661-6-1`](#rfc1661-6-1)

If a negotiable Configuration Option is received in a Configure- Request, but with an invalid or unrecognized Length, a Configure- Nak SHOULD be transmitted which includes the desired Configuration Option with an appropriate Length and Data. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Sentence: a negotiable option received with an invalid or unrecognized Length SHOULD draw a Configure-Nak carrying the desired option with an appropriate Length and Data. (a) Rejecting or ignoring it, or Naking with a bad Length or value: TestRFC1661LCPInvalidOptionLengthDrawsNak (Length 3, 1, 0) goes red on onlyLCPNak, ParseLCPOptions, len(opts[0].Data) != 2 and value != 1500; TestRFC1661LCPWrongLengthMagicIsNakedNotRejected checks Magic at 4 octets, non-zero; TestRFC1661InvalidOptionLengthDrawsNak and TestRFC1661ClientInvalidOptionLengthDrawsNak cover IPCP and the PPPoE client. Negative: valid Length draws an Ack, unrecognized Type draws a Reject.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661InvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L159) | unit/verify | unproven |
| negative | [`TestRFC1661LCPInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L325) | unit/verify | unproven |
| negative | [`TestRFC1661LCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L465) | unit/verify | unproven |
| negative | [`TestRFC1661LCPWrongLengthMagicIsNakedNotRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L408) | unit/verify | unproven |
| negative | [`TestRFC1661ReplyListsEachOptionTypeOnce`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L748) | unit/verify | unproven |
| negative | [`TestRFC1661ClientInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L173) | unit/verify | unproven |
| positive | [`TestRFC1661InvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L154) | unit/verify | unproven |
| positive | [`TestRFC1661LCPInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L318) | unit/verify | unproven |
| positive | [`TestRFC1661LCPWrongLengthMagicIsNakedNotRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L404) | unit/verify | unproven |
| positive | [`TestRFC1661ReplyListsEachOptionTypeOnce`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L744) | unit/verify | unproven |
| positive | [`TestRFC1661ClientInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L167) | unit/verify | unproven |

### [`RFC1661-6-2`](#rfc1661-6-2)

When the Data field is indicated by the Length to extend beyond the end of the Information field, the entire packet is silently discarded without affecting the automaton. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Sentence: when an option Data is indicated by its Length to extend beyond the Information field, the entire packet is silently discarded without affecting the automaton. (a) Replying: TestRFC1661LCPTruncatedOptionSilentlyDiscarded goes red on rec.count() != 0 for three layouts. (b) Moving the automaton: the same test starts in ReqSent and AckSent and asserts currentState() == start, then that a valid request is still Acked. TestRFC1661TruncatedOptionSilentlyDiscarded (IPCP, IPv6CP), the reply-direction units and TestRFC1661ClientRequestPastEndSilentlyDiscarded extend it. Negative: a contained unacceptable option still draws a Nak.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661LCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L590) | unit/verify | unproven |
| negative | [`TestRFC1661LCPTruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L243) | unit/verify | unproven |
| negative | [`TestRFC1661NCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L680) | unit/verify | unproven |
| negative | [`TestRFC1661TruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L81) | unit/verify | unproven |
| negative | [`TestRFC1661ClientRequestPastEndSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L130) | unit/verify | unproven |
| positive | [`TestRFC1661LCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L584) | unit/verify | unproven |
| positive | [`TestRFC1661LCPTruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L236) | unit/verify | unproven |
| positive | [`TestRFC1661NCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L675) | unit/verify | unproven |
| positive | [`TestRFC1661TruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L75) | unit/verify | unproven |
| positive | [`TestRFC1661ClientRequestPastEndSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L123) | unit/verify | unproven |

### [`RFC1661-3.1-1`](#rfc1661-3.1-1)

In order to establish communications over a point-to-point link, each end of the PPP link MUST first send LCP packets to configure and test the data link. (Section 3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-29. TestRFC1661RunSendsLCPBeforeAnyOtherProtocol drives run() from Initial with IPCP enabled (the input pushed toward a violation) and readLifecyclePacket fails on any first frame other than LCP Configure-Request (one frame read, exact protocol/code). Observed red with sendConfigureRequest broken; a run() writing an NCP or auth frame first turns it red by construction. Tag removed from TestRFC1661LCPPacketsSentFirst (performAction only); no orphan record left. single-polarity: positive marker stands.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1661RunSendsLCPBeforeAnyOtherProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L51) | unit/verify | revert, verified |

### [`RFC1661-3.1-2`](#rfc1661-3.1-2)

Then, PPP MUST send NCP packets to choose and configure one or more network-layer protocols. (Section 3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-29. Positive TestRFC1661RunSendsNCPAfterLCPOpens drives run(): LCP Opened, no-auth accepted, then an address request and an IPCP (0x8021) Configure-Request on the wire, else red; observed red with runNCPPhase broken. HEAD negative (both NCPs disabled, runNCPPhase writes 0 frames) is the boundary where nothing is to be chosen: it refuses NCP packets for a protocol not configured, the complement of the positive rather than a violating input, which the obligation (send NCP for the protocols to configure) has no other form of.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NoNCPPacketsWhenNoNetworkProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L370) | unit/verify | revert, verified |
| positive | [`TestRFC1661RunSendsNCPAfterLCPOpens`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L66) | unit/verify | revert, verified |
| positive | [`TestRFC1661NCPConfiguresEachFamilySeparately`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L331) | unit/verify | revert, verified |

### [`RFC1661-3.4-1`](#rfc1661-3.4-1)

Any non-LCP packets received during this phase MUST be silently discarded. (Section 3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-3.4-1, so no unit is bound to it.

### [`RFC1661-3.5-1`](#rfc1661-3.5-1)

If an implementation desires that the peer authenticate with some specific authentication protocol, then it MUST request the use of that authentication protocol during Link Establishment phase. (Section 3.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: desired auth protocol not requested in the Link Establishment Configure-Request. TestRFC1661AuthProtocolRequestedDuringEstablishment goes red on a missing option (!found) or a wrong protocol (!= 0xC223) in the LCP Configure-Request sendConfigureRequest writes; the negative TestRFC1661NoAuthProtocolWhenNotDesired goes red if the option is sent for AuthMethodNone.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NoAuthProtocolWhenNotDesired`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L473) | unit/verify | unproven |
| positive | [`TestRFC1661AuthProtocolRequestedDuringEstablishment`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L442) | unit/verify | unproven |

### [`RFC1661-3.5-2`](#rfc1661-3.5-2)

An implementation MUST NOT allow the exchange of link quality determination packets to delay authentication indefinitely. (Section 3.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-3.5-2, so no unit is bound to it.

### [`RFC1661-3.5-3`](#rfc1661-3.5-3)

Advancement from the Authentication phase to the Network-Layer Protocol phase MUST NOT occur until authentication has completed. (Section 3.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-29. TestRFC1661NoNetworkPhaseBeforeAuthenticationCompletes (IPCP enabled, CHAP): Challenge unanswered for 1 s -> no address request (-), rejected -> still none (-), accepted -> address request and IPCP Configure-Request (+). Isolated: IPCP enabled so runNCPPhase is observable. Targeted swap (runNCPPhase before runAuthPhase) observed red (ppp3-swap.log); revert records on runNCPPhase/runAuthPhase.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NoNetworkPhaseBeforeAuthenticationCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L84) | unit/verify | revert, verified |
| negative | [`TestRFC1661NetworkPhaseRunsAfterAuthCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L533) | unit/verify | revert, verified |
| positive | [`TestRFC1661NoNetworkPhaseBeforeAuthenticationCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L85) | unit/verify | revert, verified |
| positive | [`TestRFC1661NoNetworkPhaseUntilAuthCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L500) | unit/verify | revert, verified |

### [`RFC1661-3.5-4`](#rfc1661-3.5-4)

All other packets received during this phase MUST be silently discarded. (Section 3.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-3.5-4, so no unit is bound to it.

### [`RFC1661-3.6-1`](#rfc1661-3.6-1)

Once PPP has finished the previous phases, each network-layer protocol (such as IP, IPX, or AppleTalk) MUST be separately configured by the appropriate Network Control Protocol (NCP). (Section 3.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden (a): a family configured by the wrong NCP -- TestRFC1661NCPConfiguresEachFamilySeparately goes red on frames[0].Proto != 0x8021 or frames[1].Proto != 0x8057; (b) NCPs not separate -- TestRFC1661NCPStatesAreIndependent goes red when opening IPCP moves IPv6CP off Initial.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NCPStatesAreIndependent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L387) | unit/verify | unproven |
| positive | [`TestRFC1661NCPConfiguresEachFamilySeparately`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L335) | unit/verify | unproven |

### [`RFC1661-3.6-2`](#rfc1661-3.6-2)

Any supported network-layer protocol packets received when the corresponding NCP is not in the Opened state MUST be silently discarded. (Section 3.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. {single-polarity} justified. Forbidden: answering or acting on an IPv4/IPv6 packet whose (enabled) NCP is not Opened. TestRFC1661NetworkLayerPacketDiscardedBeforeNCPOpened goes red on any written frame (rec.count() != 0), on session termination, or on an LCP state change.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1661NetworkLayerPacketDiscardedBeforeNCPOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L406) | unit/verify | revert, verified |

### [`RFC1661-3.6-3`](#rfc1661-3.6-3)

While LCP is in the Opened state, any protocol packet which is unsupported by the implementation MUST be returned in a Protocol- Reject (described later). (Section 3.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: silence for an unsupported protocol in Opened. TestRFC1661UnsupportedProtocolRejectedInOpened goes red unless exactly one LCP Protocol-Reject naming 0x8281 with the Information field is written; the negative TestRFC1661UnsupportedProtocolNotRejectedBeforeOpened goes red on any frame in ReqSent/AckSent/Stopped.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661UnsupportedProtocolNotRejectedBeforeOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L94) | unit/verify | revert, verified |
| positive | [`TestRFC1661UnsupportedProtocolRejectedInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L46) | unit/verify | revert, verified |

### [`RFC1661-3.7-1`](#rfc1661-3.7-1)

The receiver of a Terminate-Request SHOULD wait for the peer to disconnect, and MUST NOT disconnect until at least one Restart time has passed after sending a Terminate-Ack. (§3.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity accepted: no early-disconnect branch exists after sca in Opened+RTR. TestRFC1661TerminateAckSentAndLinkHeld pins the instant of receipt; TestRFC1661TerminateAckHoldsLinkForRestartTime adds the time bound: after the Terminate-Ack the Restart timer is running (armed by zrc via armRestartTimer, which uses defaultRestartTimer, asserted >= 3 s), and a Configure-Request and an Echo-Request meanwhile keep Stopping with no end and no EventSessionDown. Residual: the armed duration is asserted through the constant, not read off the timer.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1661TerminateAckHoldsLinkForRestartTime`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L372) | unit/verify | revert, verified |
| positive | [`TestRFC1661TerminateAckSentAndLinkHeld`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L565) | unit/verify | revert, verified |

### [`RFC1661-3.7-2`](#rfc1661-3.7-2)

Any non-LCP packets received during this phase MUST be silently discarded. (Section 3.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-3.7-2, so no unit is bound to it.

### [`RFC1661-4.3-1`](#rfc1661-4.3-1)

The implementation MUST be prepared to immediately renegotiate the Configuration Options. (Section 4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC1661ConfigureRequestInOpenedRenegotiates drives handleLCPPacket with an acceptable Configure-Request in Opened: red unless ze writes its own Configure-Request and a Configure-Ack, lands in Ack-Sent, returns no end and emits no EventSessionDown. The table-level units (Opened+RCR has scr; RXR does not) stay as the specificity control.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661EchoDoesNotRenegotiate`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L658) | unit/verify | revert, verified |
| positive | [`TestRFC1661ConfigureRequestInOpenedRenegotiates`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L216) | unit/verify | revert, verified |
| positive | [`TestRFC1661RenegotiateOnConfigureRequestInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L625) | unit/verify | revert, verified |

### [`RFC1661-4.3-2`](#rfc1661-4.3-2)

The implementation MUST be prepared to receive a new Configure-Request without network administrator intervention. (Section 4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-29 (R16). RFC 1661 4.3 RTR note plus 4.4 tlf read. TestRFC1661StoppedAfterPeerTerminateTakesNewConfigureRequest: Opened + Terminate-Request, Timeout -> run not ended, Stopped, exactly one EventSessionDown (User Request, the tlf signal), then a Configure-Request with no Open event draws ze's CONFREQ and an Ack echoing Id 0x32, Ack-Sent, no end, no second down. Red if Stopped ends the session (sessionEndedAt) or if tlf is not signalled (recorded against signalLayerFinished). Producer checked: signalLayerFinished runs only on a Stopped sessionEndedAt kept; sendEvent's at-most-once drop cannot swallow a legitimate down because every other emitter (fail, auth, NCP, run exit paths, ncp.go) ends the session and was the first down; after tlf a renegotiation ends at Opened (TestRFC1661StoppedAfterPeerTerminateEndsOnOpened) or at a later Stopped/Closed, so the wait is bounded by the CDN/PADT teardown ze started (L2TP teardownSession -> kernel worker closes the PPPoX socket; PPPoE handleSessionDown closes the transport). Valid single-polarity: no refusing counterpart; marker still true.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1661StoppedAfterPeerTerminateTakesNewConfigureRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L257) | unit/verify | revert, verified |
| positive | [`TestRFC1661NewConfigureRequestAcceptedAfterTerminateRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L678) | unit/verify | revert, verified |

### [`RFC1661-4.3-3`](#rfc1661-4.3-3)

The implementation MUST stop sending the offending packet type. (Section 4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-4.3-3, so no unit is bound to it.

### [`RFC1661-5.1-1`](#rfc1661-5.1-1)

An implementation wishing to open a connection MUST transmit a Configure-Request. (Section 5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC1661OpenTransmitsConfigureRequestOnWire runs applyTransition (the same path run() uses for its synthetic Open) for Closed+Open and Starting+Up: red unless exactly one LCP Configure-Request reaches the wire and state is Req-Sent. Initial+Up (no Open) writes nothing and lands in Closed. The run() entry itself is RFC1661-3.1-1's, still open.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661OpenTransmitsConfigureRequestOnWire`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L421) | unit/verify | revert, verified |
| negative | [`TestRFC1661UpWithoutOpenSendsNoConfigureRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L735) | unit/verify | revert, verified |
| positive | [`TestRFC1661OpenTransmitsConfigureRequestOnWire`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L420) | unit/verify | revert, verified |
| positive | [`TestRFC1661OpenTransmitsConfigureRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L704) | unit/verify | revert, verified |

### [`RFC1661-5.1-2`](#rfc1661-5.1-2)

Upon reception of a Configure-Request, an appropriate reply MUST be transmitted. (Section 5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: no reply, or the wrong reply, to a Configure-Request. TestRFC1661ConfigureAckEchoesOptionsVerbatim goes red when no Configure-Ack (Identifier 0x21) is written for an acceptable request; TestRFC1661ConfigureRejectForUnrecognizedOption goes red on an Ack or a missing Configure-Reject for an unrecognized option. Driven in ReqSent through handleLCPPacket.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L817) | unit/verify | unproven |
| positive | [`TestRFC1661ConfigureAckEchoesOptionsVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L753) | unit/verify | unproven |

### [`RFC1661-5.1-3`](#rfc1661-5.1-3)

The Identifier field MUST be changed whenever the contents of the Options field changes, and whenever a valid reply has been received for a previous request. (Section 5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudge ppp8 2026-09-30: pppoeclient TestClientLCPRejectRemovesOnlyRejectedOptions changed only to read frameLog through the locked snapshot() (frameLog race fix); every assertion, input and comparison is unchanged, so the verdict stands. (a) options change -> new Identifier: TestLCPConfigureRequestIdentifierLifecycle goes red when changed.Identifier == retry.Identifier; (b) valid reply -> new Identifier: red when afterAck.Identifier == changed.Identifier (server), and the client units go red when the post-Nak / post-Reject request keeps first.Identifier. Negative: timeout retransmission keeps the request, stale Ack does not advance.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLCPConfigureRequestIdentifierLifecycle`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L493) | unit/verify | unproven |
| negative | [`TestClientLCPNakChangesIdentifierAndDropsStaleReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L146) | unit/verify | unproven |
| positive | [`TestLCPConfigureRequestIdentifierLifecycle`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L492) | unit/verify | unproven |
| positive | [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L80) | unit/verify | unproven |

### [`RFC1661-5.2-1`](#rfc1661-5.2-1)

If every Configuration Option received in a Configure-Request is recognizable and all values are acceptable, then the implementation MUST transmit a Configure-Ack. (Section 5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: not Acking a fully acceptable request. TestRFC1661ConfigureAckEchoesOptionsVerbatim goes red on no Configure-Ack; TestRFC1877ConfigureAckEchoesAcceptable goes red when an acceptable IPCP request draws no Ack echoing it. Negatives: an unrecognized option (no Ack) and an unacceptable IP-Address (Nak).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1877ConfigureAckEchoesAcceptable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_dns_options_test.go#L18) | unit/verify | revert, verified |
| negative | [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L821) | unit/verify | unproven |
| positive | [`TestRFC1661ConfigureAckEchoesOptionsVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L757) | unit/verify | unproven |

### [`RFC1661-5.2-2`](#rfc1661-5.2-2)

The acknowledged Configuration Options MUST NOT be reordered or modified in any way. (§5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Sentence: the acknowledged Configuration Options MUST NOT be reordered or modified in any way. (a) Reordering: TestRFC1661ConfigureAckDoesNotReorderOptions sends MRU,Magic and Magic,MRU and bytes.Equal(pkt.Data, data) goes red on a normalized order. (b) Modifying: TestRFC1661ConfigureAckEchoesOptionsVerbatim bytes.Equal(pkt.Data, data) goes red on any changed value or length; TestRFC1877ConfigureAckEchoesAcceptable carries the same check for IPCP.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661ConfigureAckDoesNotReorderOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L786) | unit/verify | unproven |
| positive | [`TestRFC1877ConfigureAckEchoesAcceptable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_dns_options_test.go#L14) | unit/verify | revert, verified |
| positive | [`TestRFC1661ConfigureAckEchoesOptionsVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L760) | unit/verify | unproven |

### [`RFC1661-5.2-3`](#rfc1661-5.2-3)

On reception of a Configure-Ack, the Identifier field MUST match that of the last transmitted Configure-Request. (Section 5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudge ppp8 2026-09-30: pppoeclient TestClientLCPReplyCorrelation changed only to read frameLog through the locked snapshot() (frameLog race fix); every assertion, input and comparison is unchanged, so the verdict stands. Forbidden: accepting a Configure-Ack whose Identifier differs from the last request. TestLCPRepliesMatchOutstandingRequest ("Ack wrong Identifier") goes red on any state or wire change; TestClientLCPReplyCorrelation goes red if Identifier+1 completes LCP. Positives: the matching Ack reaches Ack-Rcvd / completes negotiation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L400) | unit/verify | unproven |
| negative | [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L19) | unit/verify | unproven |
| positive | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L399) | unit/verify | unproven |
| positive | [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L18) | unit/verify | unproven |

### [`RFC1661-5.2-4`](#rfc1661-5.2-4)

Additionally, the Configuration Options in a Configure-Ack MUST exactly match those of the last transmitted Configure-Request. (Section 5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudge ppp8 2026-09-30: pppoeclient TestClientLCPReplyCorrelation changed only to read frameLog through the locked snapshot() (frameLog race fix); every assertion, input and comparison is unchanged, so the verdict stands. Forbidden: accepting an Ack whose options differ from the request. TestLCPRepliesMatchOutstandingRequest goes red when a changed value, a missing, extra or reordered option changes the FSM or wire; TestClientLCPReplyCorrelation goes red if a nil, modified or reordered Ack completes LCP. The exact Ack advances in both.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L402) | unit/verify | unproven |
| negative | [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L21) | unit/verify | unproven |
| positive | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L401) | unit/verify | unproven |
| positive | [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L20) | unit/verify | unproven |

### [`RFC1661-5.3-1`](#rfc1661-5.3-1)

If every instance of the received Configuration Options is recognizable, but some values are not acceptable, then the implementation MUST transmit a Configure-Nak. (§5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Sentence: if every instance of the received options is recognizable but some values are not acceptable, the implementation MUST transmit a Configure-Nak. (a) Answering an unacceptable MRU 2000 with Ack, Reject or nothing: TestRFC1661ConfigureNakSuggestsAcceptableValue findCode(LCPConfigureNak) fails. (b) Nak where all values are acceptable: TestRFC1661NoNakForAcceptableValue findCode(LCPConfigureNak) ok goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NoNakForAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L942) | unit/verify | unproven |
| positive | [`TestRFC1661ConfigureNakSuggestsAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L907) | unit/verify | unproven |

### [`RFC1661-5.3-2`](#rfc1661-5.3-2)

Options which have no value fields (boolean options) MUST use the Configure-Reject reply instead. (Section 5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC1661WellFormedBooleanDeclinedWithReject removes the length-fault confound: a well-formed Length-2 ACFC declined on PPPoE draws a Configure-Reject whose options are exactly that ACFC and no Nak; beside a Nak-earning MRU, NegotiatePeerOptions puts the ACFC in rejects not naks and the wire reply is the Reject naming only ACFC. Red on a Nak for the boolean.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661WellFormedBooleanDeclinedWithReject`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L93) | unit/verify | revert, verified |
| negative | [`TestRFC1661ValuedOptionUsesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L994) | unit/verify | revert, verified |
| positive | [`TestRFC1661WellFormedBooleanDeclinedWithReject`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L92) | unit/verify | revert, verified |
| positive | [`TestRFC1661BooleanOptionsUseRejectNotNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L972) | unit/verify | revert, verified |

### [`RFC1661-5.3-3`](#rfc1661-5.3-3)

Each Configuration Option which is allowed only a single instance MUST be modified to a value acceptable to the Configure-Nak sender. (Section 5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Nak that repeats an unacceptable single-instance value. TestRFC1661ConfigureNakSuggestsAcceptableValue goes red unless the Nak carries one MRU at 1500 for a requested 2000; TestRFC1661RepeatedOptionDrawsOneNakEntry goes red on more than one Magic entry; TestRFC1661NoNakForAcceptableValue goes red if an acceptable MRU is Nak'd or modified.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NoNakForAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L945) | unit/verify | unproven |
| positive | [`TestRFC1661RepeatedOptionDrawsOneNakEntry`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L345) | unit/verify | unproven |
| positive | [`TestRFC1661ConfigureNakSuggestsAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L911) | unit/verify | unproven |

### [`RFC1661-5.3-4`](#rfc1661-5.3-4)

When a particular type of Configuration Option can be listed more than once with different values, the Configure-Nak MUST include a list of all values for that option which are acceptable to the Configure-Nak sender. (Section 5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-5.3-4, so no unit is bound to it.

### [`RFC1661-5.3-5`](#rfc1661-5.3-5)

Any value fields for the option MUST indicate values acceptable to the Configure-Nak sender. (Section 5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Nak value the Nak sender would not accept. TestRFC1661NakValueIsAcceptable re-offers the Nak'd MRU (from 2000 and 32) and goes red unless it draws ack=1 nak=0 rej=0; TestRFC1661RejectedValueStaysUnacceptable goes red if the original 2000 is accepted. Only the MRU option is exercised.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661RejectedValueStaysUnacceptable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1033) | unit/verify | unproven |
| positive | [`TestRFC1661NakValueIsAcceptable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1011) | unit/verify | unproven |

### [`RFC1661-5.3-6`](#rfc1661-5.3-6)

All acceptable Configuration Options are filtered out of the Configure-Nak, but otherwise the Configuration Options from the Configure-Request MUST NOT be reordered. (§5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Reordering: TestRFC1661NakPreservesRequestOrder / TestRFC1661NakOrderFollowsRequestNotAFixedOrder. Filtering: TestRFC1661NakFiltersAcceptableOptions puts an acceptable ACCM between Magic 0 and MRU 2000 in both orders and reads the wire Configure-Nak: red unless its Types are exactly the two Nak-earning ones in request order, so a copied ACCM or a reorder both fail.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NakFiltersAcceptableOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L139) | unit/verify | revert, verified |
| negative | [`TestRFC1661NakOrderFollowsRequestNotAFixedOrder`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1078) | unit/verify | revert, verified |
| positive | [`TestRFC1661NakFiltersAcceptableOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L138) | unit/verify | revert, verified |
| positive | [`TestRFC1661NakPreservesRequestOrder`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1050) | unit/verify | revert, verified |

### [`RFC1661-5.3-7`](#rfc1661-5.3-7)

On reception of a Configure-Nak, the Identifier field MUST match that of the last transmitted Configure-Request. (Section 5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudge ppp8 2026-09-30: pppoeclient TestClientLCPReplyCorrelation changed only to read frameLog through the locked snapshot() (frameLog race fix); every assertion, input and comparison is unchanged, so the verdict stands. Forbidden: acting on a Nak whose Identifier differs from the last request. TestLCPRepliesMatchOutstandingRequest ("Nak wrong Identifier") goes red on any state/wire change; TestClientLCPReplyCorrelation goes red if Identifier+1 Nak emits a frame. Positives: matching Nak yields a new request carrying MRU 1400 (server) and a new request (client).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L404) | unit/verify | unproven |
| negative | [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L22) | unit/verify | unproven |
| positive | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L403) | unit/verify | unproven |
| positive | [`TestClientLCPNakChangesIdentifierAndDropsStaleReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L145) | unit/verify | unproven |

### [`RFC1661-5.3-8`](#rfc1661-5.3-8)

Since the Nak'd Option has been modified by the peer, the implementation MUST be able to handle an Option length which is different from the original Configure-Request. (Section 5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: mishandling a Nak option of a different Length. TestRFC1661NakHandlesDifferentOptionLength goes red unless a 4-octet PAP Nak against ze's 5-octet CHAP option switches the method to PAP; TestRFC1661NakTooShortOptionNotDecoded goes red if a 1-octet value is decoded as a protocol.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NakTooShortOptionNotDecoded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1121) | unit/verify | unproven |
| positive | [`TestRFC1661NakHandlesDifferentOptionLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1099) | unit/verify | unproven |

### [`RFC1661-5.4-1`](#rfc1661-5.4-1)

If some Configuration Options received in a Configure-Request are not recognizable or are not acceptable for negotiation (as configured by a network administrator), then the implementation MUST transmit a Configure-Reject. (Section 5.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Clause (a) unrecognized: TestRFC1661ConfigureRejectForUnrecognizedOption and the OutranksInvalidLength units. Clause (b) refused by configuration: TestRFC1661ConfigureRejectForOptionRefusedByConfiguration sends Auth-Protocol (any session) and ACCM (PPPoE), red unless the Reject carries exactly that option and no Ack is written; the same ACCM on L2TP, which configuration accepts, must be Acked with no Reject.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661ConfigureRejectForOptionRefusedByConfiguration`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L170) | unit/verify | revert, verified |
| negative | [`TestRFC1877ConfigureRejectEchoesUnsupportedOnly`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_dns_options_test.go#L70) | unit/verify | revert, verified |
| negative | [`TestRFC1661NCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L531) | unit/verify | revert, verified |
| negative | [`TestRFC1661NoConfigureRejectWhenAllOptionsRecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L855) | unit/verify | revert, verified |
| negative | [`TestRFC1661ClientAcceptsServerAuthProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L373) | unit/verify | revert, verified |
| negative | [`TestRFC1661ClientUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L221) | unit/verify | revert, verified |
| positive | [`TestRFC1661ConfigureRejectForOptionRefusedByConfiguration`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_clauses_test.go#L169) | unit/verify | revert, verified |
| positive | [`TestRFC1661LCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L459) | unit/verify | revert, verified |
| positive | [`TestRFC1661NCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L526) | unit/verify | revert, verified |
| positive | [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L824) | unit/verify | revert, verified |
| positive | [`TestRFC1661ClientUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L216) | unit/verify | revert, verified |

### [`RFC1661-5.4-2`](#rfc1661-5.4-2)

All recognizable and negotiable Configuration Options are filtered out of the Configure-Reject, but otherwise the Configuration Options MUST NOT be reordered or modified in any way. (§5.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Sentence: all recognizable and negotiable options are filtered out of the Configure-Reject, but otherwise the options MUST NOT be reordered or modified in any way. (a) Keeping a recognizable option: TestRFC1661ConfigureRejectForUnrecognizedOption sends MRU(1400)+type 99 and bytes.Equal(pkt.Data, optStream(unknown)) goes red if MRU stays. (b) Reordering: TestRFC1661ConfigureRejectDoesNotReorderOptions compares Types in both orders. (c) Modifying: the same test compares each Data, and TestRFC1661LCPRejectEchoesTheRefusedOptionUnmodified / TestRFC1661ClientRejectEchoesTheRefusedOptionUnmodified check the echoed bytes.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661LCPRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L839) | unit/verify | unproven |
| negative | [`TestRFC1661ConfigureRejectDoesNotReorderOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L873) | unit/verify | unproven |
| negative | [`TestRFC1661ClientRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L264) | unit/verify | unproven |
| positive | [`TestRFC1877ConfigureRejectEchoesUnsupportedOnly`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_dns_options_test.go#L67) | unit/verify | revert, verified |
| positive | [`TestRFC1661LCPRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L828) | unit/verify | unproven |
| positive | [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L827) | unit/verify | unproven |
| positive | [`TestRFC1661ClientRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L252) | unit/verify | unproven |

### [`RFC1661-5.4-3`](#rfc1661-5.4-3)

On reception of a Configure-Reject, the Identifier field MUST match that of the last transmitted Configure-Request. (Section 5.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudge ppp8 2026-09-30: pppoeclient TestClientLCPRejectRemovesOnlyRejectedOptions and TestClientLCPReplyCorrelation changed only to read frameLog through the locked snapshot() (frameLog race fix); every assertion, input and comparison is unchanged, so the verdict stands. Forbidden: acting on a Reject whose Identifier differs. TestLCPRepliesMatchOutstandingRequest ("Reject wrong Identifier") and TestClientLCPReplyCorrelation go red on any state/wire change; positives: the matching Reject yields a new request without MRU (server) and a new-Identifier request without the rejected options (TestClientLCPRejectRemovesOnlyRejectedOptions).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L406) | unit/verify | unproven |
| negative | [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L23) | unit/verify | unproven |
| positive | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L405) | unit/verify | unproven |
| positive | [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L76) | unit/verify | unproven |

### [`RFC1661-5.4-4`](#rfc1661-5.4-4)

Additionally, the Configuration Options in a Configure-Reject MUST be a subset of those in the last transmitted Configure- Request. (§5.4, erratum 543)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudge ppp8 2026-09-30: pppoeclient TestClientLCPRejectRemovesOnlyRejectedOptions and TestClientLCPReplyCorrelation changed only to read frameLog through the locked snapshot() (frameLog race fix); every assertion, input and comparison is unchanged, so the verdict stands. Sentence (erratum 543): the options in a Configure-Reject MUST be a subset of those in the last transmitted Configure-Request; invalid packets are silently discarded. (a) Accepting a Reject naming an option never requested, or with a changed value: TestLCPRepliesMatchOutstandingRequest cases Reject unrequested option and Reject changed value go red (state or frame count changes); TestClientLCPReplyCorrelation covers modified. (b) Refusing a Reject equal to the whole request, the case the erratum restores: TestClientLCPRejectRemovesOnlyRejectedOptions subtest all goes red if the replacement request is not emitted; a proper-subset check in ppp would also redden TestLCPRejectedOptionsStayRemoved (all=true), which carries the 5.4-5 tag only.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L408) | unit/verify | unproven |
| negative | [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L24) | unit/verify | unproven |
| positive | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L407) | unit/verify | unproven |
| positive | [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L77) | unit/verify | unproven |

### [`RFC1661-5.4-5`](#rfc1661-5.4-5)

Reception of a valid Configure-Reject indicates that when a new Configure-Request is sent, it MUST NOT include any of the Configuration Options listed in the Configure-Reject. (§5.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudge ppp8 2026-09-30: pppoeclient TestClientLCPRejectRemovesOnlyRejectedOptions changed only to read frameLog through the locked snapshot() (frameLog race fix); every assertion, input and comparison is unchanged, so the verdict stands. Sentence: when a new Configure-Request is sent after a valid Configure-Reject, it MUST NOT include any of the rejected options. (a) Re-including a rejected option: TestLCPRejectedOptionsStayRemoved (only MRU, and all options) and TestClientLCPRejectRemovesOnlyRejectedOptions (MRU, Magic, all; across a retransmission and a later Nak) go red on bytes.Equal(next.Data, want) / len(next.Data) != 0. Negative: un-rejected options stay byte-identical.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLCPRejectedOptionsStayRemoved`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L534) | unit/verify | unproven |
| negative | [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L79) | unit/verify | unproven |
| positive | [`TestLCPRejectedOptionsStayRemoved`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_test.go#L533) | unit/verify | unproven |
| positive | [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_reply_test.go#L78) | unit/verify | unproven |

### [`RFC1661-5.5-1`](#rfc1661-5.5-1)

Upon reception of a Terminate-Request, a Terminate-Ack MUST be transmitted. (Section 5.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: no Terminate-Ack for a Terminate-Request. TestRFC1661TerminateAckSentAndLinkHeld goes red on no Terminate-Ack or Identifier != 0x77 (driven in Opened only); TestRFC1661NoTerminateAckForTerminateAck goes red if a Terminate-Ack draws one.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NoTerminateAckForTerminateAck`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L604) | unit/verify | unproven |
| positive | [`TestRFC1661TerminateAckSentAndLinkHeld`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L570) | unit/verify | revert, verified |

### [`RFC1661-5.6-1`](#rfc1661-5.6-1)

This MUST be reported back to the sender of the unknown Code by transmitting a Code- Reject. (Section 5.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: not Code-Rejecting an unknown Code. TestRFC1661CodeRejectForUnknownCode goes red on no Code-Reject or a Rejected-Packet not carrying Code 99, Identifier 0x41, Length 6, Data aabb; TestRFC1661NoCodeRejectForKnownCode goes red if a Terminate-Request is Code-Rejected.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NoCodeRejectForKnownCode`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1175) | unit/verify | unproven |
| positive | [`TestRFC1661CodeRejectForUnknownCode`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1143) | unit/verify | unproven |

### [`RFC1661-5.6-2`](#rfc1661-5.6-2)

The Identifier field MUST be changed for each Code-Reject sent. (Section 5.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-5.6-2, so no unit is bound to it.

### [`RFC1661-5.6-3`](#rfc1661-5.6-3)

The Rejected-Packet MUST be truncated to comply with the peer's established MRU. (Section 5.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-5.6-3, so no unit is bound to it.

### [`RFC1661-5.7-1`](#rfc1661-5.7-1)

If the LCP automaton is in the Opened state, then this MUST be reported back to the peer by transmitting a Protocol-Reject. (Section 5.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: no Protocol-Reject for an unrecognized protocol in Opened. TestRFC1661UnsupportedProtocolRejectedInOpened goes red unless one LCP code-8 frame names 0x8281; TestRFC1661UnsupportedProtocolNotRejectedBeforeOpened goes red on any frame outside Opened.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661UnsupportedProtocolNotRejectedBeforeOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L89) | unit/verify | revert, verified |
| positive | [`TestRFC1661UnsupportedProtocolRejectedInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L40) | unit/verify | revert, verified |

### [`RFC1661-5.7-2`](#rfc1661-5.7-2)

Upon reception of a Protocol-Reject, the implementation MUST stop sending packets of the indicated protocol at the earliest opportunity. (Section 5.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-5.7-2, so no unit is bound to it.

### [`RFC1661-5.7-3`](#rfc1661-5.7-3)

The Identifier field MUST be changed for each Protocol-Reject sent. (Section 5.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. {single-polarity} justified. Forbidden: two Protocol-Rejects with one Identifier. TestRFC1661ProtocolRejectIdentifierChanges goes red when the two consecutive Protocol-Rejects carry equal Identifiers.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1661ProtocolRejectIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L137) | unit/verify | revert, verified |

### [`RFC1661-5.7-4`](#rfc1661-5.7-4)

The Rejected-Information MUST be truncated to comply with the peer's established MRU. (Section 5.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: Rejected-Information exceeding the peer's MRU. TestRFC1661ProtocolRejectInformationTruncatedToMRU goes red unless the frame is frameLen(100) and the kept data is 94 octets from the head; TestRFC1661ProtocolRejectInformationKeptWhenItFits goes red if a fitting 40-octet packet is cut.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661ProtocolRejectInformationKeptWhenItFits`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L201) | unit/verify | revert, verified |
| positive | [`TestRFC1661ProtocolRejectInformationTruncatedToMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L166) | unit/verify | revert, verified |

### [`RFC1661-5.8-1`](#rfc1661-5.8-1)

Upon reception of an Echo-Request in the LCP Opened state, an Echo-Reply MUST be transmitted. (Section 5.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge ppp4 2026-09-30. TestRFC1661EchoReplyInOpened: Opened, Echo-Request with the peer's Magic-Number draws an Echo-Reply with Identifier 0x51 and ze's Magic (+). TestRFC1661NoEchoOutsideOpened (-): now sets peerMagic 0x99887766 and sends that Magic, so the Magic-Number filter in handleLCPPacket passes and the FSM state alone decides; at HEAD the Data equalled the local magic with peerMagic 0 and the packet was dropped before the FSM, a vacuous negative that is now fixed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NoEchoOutsideOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1237) | unit/verify | unproven |
| positive | [`TestRFC1661EchoReplyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1196) | unit/verify | unproven |

### [`RFC1661-5.8-2`](#rfc1661-5.8-2)

Echo-Request and Echo-Reply packets MUST only be sent in the LCP Opened state. (Section 5.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudge ppp8 2026-09-30: pppoeclient TestClientLCPEchoBeforeOpenDiscarded changed only to read frameLog through the locked snapshot() (frameLog race fix); every assertion, input and comparison is unchanged, so the verdict stands. Judge ppp4 2026-09-30. Polarity labels corrected (D-15): TestRFC1661EchoReplyInOpened is now the positive (reply sent in Opened, recorded red on sendEchoReply) and TestRFC1661NoEchoOutsideOpened the negative (Echo-Request in Req-Sent/Ack-Sent/Ack-Rcvd/Stopped draws neither Echo-Reply nor Echo-Request, recorded red on LCPDoTransition). The negative is no longer vacuous: the peer Magic-Number now matches peerMagic, so only the state blocks the reply, and an FSM that gave RXR a ser edge outside Opened turns it red. Echo-Request half unchanged: TestRFC1661KeepaliveEchoOnlyInOpened (run(), echo in Opened; after the peer CONFREQ the next frame is the CONFREQ retransmission, no Echo-Request). pppoeclient TestClientLCPEchoBeforeOpenDiscarded kept (client role, no record).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661KeepaliveEchoOnlyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L126) | unit/verify | revert, verified |
| negative | [`TestRFC1661NoEchoOutsideOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1241) | unit/verify | revert, verified |
| negative | [`TestClientLCPEchoBeforeOpenDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_lcp_transition_test.go#L115) | unit/verify | revert, verified |
| positive | [`TestRFC1661KeepaliveEchoOnlyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L125) | unit/verify | revert, verified |
| positive | [`TestRFC1661EchoReplyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1199) | unit/verify | revert, verified |

### [`RFC1661-5.9-1`](#rfc1661-5.9-1)

Discard-Request packets MUST only be sent in the LCP Opened state. (Section 5.9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-5.9-1, so no unit is bound to it.

### [`RFC1661-5.9-2`](#rfc1661-5.9-2)

On reception, the receiver MUST silently discard any Discard- Request that it receives. (Section 5.9)

Audit verdict: enforced (the tests do what the requirement demands), fresh. {single-polarity} justified. Forbidden: any reply or effect for a Discard-Request. TestRFC1661DiscardRequestSilentlyDiscarded goes red on any written frame, a state change or a lifecycle event.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1661DiscardRequestSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1266) | unit/verify | unproven |

### [`RFC1661-6.2-1`](#rfc1661-6.2-1)

An implementation MUST NOT include multiple Authentication- Protocol Configuration Options in its Configure-Request packets. (Section 6.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. {single-polarity} justified. Forbidden: two Authentication-Protocol options in a request. TestRFC1661SingleAuthProtocolOptionInRequest goes red when BuildLocalConfigRequest emits a count != 1 for PAP, CHAP-MD5 or MS-CHAPv2.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1661SingleAuthProtocolOptionInRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1298) | unit/verify | unproven |

### [`RFC1661-6.4-1`](#rfc1661-6.4-1)

If an implementation does transmit a Configure-Request with a Magic-Number Configuration Option, then it MUST NOT respond with a Configure-Reject when it receives a Configure-Request with a Magic-Number Configuration Option. (Section 6.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: Configure-Rejecting a peer Magic-Number while ze sends one. TestRFC1661ZeroMagicNumberRefused goes red on any reject of a zero Magic; TestRFC1661UnknownOptionRejectedWhileMagicIsNot goes red unless the Magic is Acked while Type 99 in the same request is Rejected (proving the reject path is live).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661UnknownOptionRejectedWhileMagicIsNot`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1348) | unit/verify | unproven |
| positive | [`TestRFC1661ZeroMagicNumberRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1376) | unit/verify | unproven |

### [`RFC1661-6.4-2`](#rfc1661-6.4-2)

If Magic-Number has been successfully negotiated, an implementation MUST transmit these packets with the Magic-Number field set to its negotiated Magic-Number. (Section 6.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge ppp4 2026-09-30. Re-read after the comment-only edit to TestRFC1661EchoReplyInOpened: it still asserts the Echo-Reply Magic-Number equals the local negotiated value (+); negatives and the Echo-Request half (TestEchoRequestCarriesNegotiatedMagic) unchanged. ze sends no Discard-Request (no producer).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEchoRequestCarriesNegotiatedMagic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L419) | unit/verify | revert, verified |
| negative | [`TestRFC1661EchoReplyDoesNotMirrorPeerMagic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1415) | unit/verify | revert, verified |
| positive | [`TestEchoRequestCarriesNegotiatedMagic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L418) | unit/verify | revert, verified |
| positive | [`TestRFC1661EchoReplyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1208) | unit/verify | revert, verified |

### [`RFC1661-6.4-3`](#rfc1661-6.4-3)

A Magic-Number of zero is illegal and MUST always be Nak'd, if it is not Rejected outright. (Section 6.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting a zero Magic-Number. TestRFC1661ZeroMagicNumberRefused (server) and TestRFC1661ClientNaksZeroMagicInAWellFormedRequest / ...WithAValueOfItsOwn (client) go red unless a zero Magic draws a Nak carrying a 4-octet non-zero value; negatives Ack a non-zero Magic (TestRFC1661PeerMagicNumberAcked, TestRFC1661ClientAcksANonZeroMagic).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661PeerMagicNumberAcked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1329) | unit/verify | unproven |
| negative | [`TestRFC1661ClientAcksANonZeroMagic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_client_negotiation_test.go#L82) | unit/verify | unproven |
| negative | [`TestRFC1661ClientNaksZeroMagicWithAValueOfItsOwn`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L427) | unit/verify | unproven |
| positive | [`TestRFC1661ZeroMagicNumberRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1368) | unit/verify | unproven |
| positive | [`TestRFC1661ClientNaksZeroMagicInAWellFormedRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_client_negotiation_test.go#L44) | unit/verify | unproven |
| positive | [`TestRFC1661ClientNaksZeroMagicWithAValueOfItsOwn`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L421) | unit/verify | unproven |

### [`RFC1661-6.5-1`](#rfc1661-6.5-1)

By default, all implementations MUST transmit packets with two octet PPP Protocol fields. (Section 6.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. {single-polarity} justified. Forbidden: a one-octet Protocol field on transmit. TestRFC1661TwoOctetProtocolField goes red when WriteFrame writes off != 2 or octets != c021.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1661TwoOctetProtocolField`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L259) | unit/verify | unproven |

### [`RFC1661-6.5-2`](#rfc1661-6.5-2)

Compressed Protocol fields MUST NOT be transmitted unless this Configuration Option has been negotiated. (Section 6.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. {single-polarity} justified: WriteFrame has no compressed branch. Forbidden: a compressed Protocol without negotiation. TestRFC1661TwoOctetProtocolField goes red if WriteFrame writes one octet. Its pfc loop is decorative (the built option list never reaches WriteFrame); the assertion that discriminates is the off != 2 check.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1661TwoOctetProtocolField`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L263) | unit/verify | unproven |

### [`RFC1661-6.5-3`](#rfc1661-6.5-3)

When negotiated, PPP implementations MUST accept PPP packets with either double-octet or single-octet Protocol fields, and MUST NOT distinguish between them. (§6.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-6.5-3, so no unit is bound to it.

### [`RFC1661-6.6-1`](#rfc1661-6.6-1)

By default, all implementations MUST transmit frames with Address and Control fields appropriate to the link framing. (Section 6.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-6.6-1, so no unit is bound to it.

### [`RFC1661-6.6-2`](#rfc1661-6.6-2)

The Address and Control fields MUST NOT be compressed when sending any LCP packet. (Section 6.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-6.6-2, so no unit is bound to it.

### [`RFC1661-4.6-1`](#rfc1661-4.6-1)

The Restart timer MUST be configurable (§4.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-4.6-1, so no unit is bound to it.

### [`RFC1661-4.6-2`](#rfc1661-4.6-2)

Max-Terminate MUST be configurable (§4.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-4.6-2, so no unit is bound to it.

### [`RFC1661-4.6-3`](#rfc1661-4.6-3)

Max-Configure MUST be configurable (§4.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-4.6-3, so no unit is bound to it.

### [`RFC1661-4.6-4`](#rfc1661-4.6-4)

Max-Failure MUST be configurable (§4.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-4.6-4, so no unit is bound to it.

### [`RFC1661-4.4-1`](#rfc1661-4.4-1)

In addition to setting the Restart counter, the implementation MUST set the timeout period to the initial value when Restart timer backoff is used. (Section 4.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-4.4-1, so no unit is bound to it.

### [`RFC1661-4.4-2`](#rfc1661-4.4-2)

In addition to zeroing the Restart counter, the implementation MUST set the timeout period to an appropriate value. (Section 4.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read after TestLCPPeerTerminateRestartTimer changed its end-of-grace assertion to the RFC 1661 Section 4.3 behaviour (no session end, Stopped). The 4.4-2 assertions are unchanged: Opened+RTR arms the real Restart timer at defaultRestartTimer, the session stays Stopping for the grace period and the timer event reaches Stopped; the Closed case arms no timer. Both polarities observed red under recorded breaks (armRestartTimer, applyTransition).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLCPPeerTerminateRestartTimer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_restart_counter_test.go#L136) | unit/verify | revert, verified |
| positive | [`TestLCPPeerTerminateRestartTimer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_lcp_restart_counter_test.go#L135) | unit/verify | revert, verified |

### [`RFC1661-5.5-2`](#rfc1661-5.5-2)

On transmission, the Identifier field MUST be changed whenever the content of the Data field changes, and whenever a valid reply has been received for a previous request. (§5.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent re-judge (R10). TestTerminateRequestIdentifierChanges: Terminate-Request, a Terminate-Ack with its Identifier accepted as a valid reply (state Closed asserted), a second Terminate-Request; red when the second reuses the Identifier, and only the two requests are written. Data is empty on both and asserted so: sendTerminateRequest always writes nil Data, so the Data-change clause has no reachable case. The negative tag reads the same Identifier assertion as the positive; the send-side MUST has no refusing input, so this is in substance a single-polarity positive (a marker on the row would say so). The recorded break panics sendTerminateRequest; the reply-clause break (dropping terminateID++) is caught by reading the assertion.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTerminateRequestIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L60) | unit/verify | revert, verified |
| positive | [`TestTerminateRequestIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L59) | unit/verify | revert, verified |

### [`RFC1661-5.8-4`](#rfc1661-5.8-4)

On transmission, the Identifier field MUST be changed whenever the content of the Data field changes, and whenever a valid reply has been received for a previous request. (§5.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestEchoRequestIdentifierChanges isolates both triggers: a valid Echo-Reply (echoOutstanding cleared) between two Echo-Requests with the same Data (asserted equal), red on a reused Identifier; then magicNegotiated flips with no reply, Data asserted changed, red on a reused Identifier. sendEchoReply Identifier copy also pinned.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEchoRequestIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L90) | unit/verify | revert, verified |
| positive | [`TestEchoRequestIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L89) | unit/verify | revert, verified |

### [`RFC1661-5.8-5`](#rfc1661-5.8-5)

Until the Magic-Number Configuration Option has been successfully negotiated, the Magic- Number MUST be transmitted as zero. (§5.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Sentence: until the Magic-Number option has been successfully negotiated, the Magic-Number MUST be transmitted as zero. (a) Sending the chosen Magic-Number before negotiation: TestMagicNumberZeroUntilNegotiated goes red on req != 0 || rep != 0 before any Ack, and again after a Configure-Ack that carries no option 5. Positive: after an Ack with option 5 both packets carry the value. Echo-Request and Echo-Reply both covered; the identical Discard-Request sentence is Section 5.9 and ze sends no Discard-Request.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMagicNumberZeroUntilNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L135) | unit/verify | revert, verified |
| positive | [`TestMagicNumberZeroUntilNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L134) | unit/verify | revert, verified |

### [`RFC1661-5.9-3`](#rfc1661-5.9-3)

The Identifier field MUST be changed for each Discard-Request sent. (Section 5.9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-5.9-3, so no unit is bound to it.

### [`RFC1661-6.1-1`](#rfc1661-6.1-1)

If smaller packets are requested, an implementation MUST still be able to receive the full 1500 octet information field in case link synchronization is lost. (Section 6.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: refusing a 1500-octet Information field. TestFullInformationFieldReceived goes red if ParseFrame refuses 1502 octets, returns fewer than 1500, if getFrameBuf (the readFrames buffer, MRU-independent) is shorter than the frame, or ParseLCPPacket refuses Length 1500.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFullInformationFieldReceived`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L283) | unit/verify | revert, verified |
| positive | [`TestFullInformationFieldReceived`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L282) | unit/verify | revert, verified |

### [`RFC1661-6.4-5`](#rfc1661-6.4-5)

Before this Configuration Option is requested, an implementation MUST choose its Magic-Number. (Section 6.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: requesting option 5 before a Magic-Number is chosen. TestMagicNumberChosenBeforeRequested goes red if BuildLocalConfigRequest emits option 5 with no chosen value, or if the chosen request carries a value other than the generated non-zero one.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMagicNumberChosenBeforeRequested`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_choice_test.go#L21) | unit/verify | revert, verified |
| positive | [`TestMagicNumberChosenBeforeRequested`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_choice_test.go#L20) | unit/verify | revert, verified |

### [`RFC1661-6.4-6`](#rfc1661-6.4-6)

If the two Magic-Numbers are equal, then it is possible, but not certain, that the link is looped-back and that this Configure-Request is actually the one last sent.  To determine this, a Configure-Nak MUST be sent specifying a different Magic-Number value. (§6.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Sentence: if the received Magic-Number equals the one last sent, a Configure-Nak MUST be sent specifying a different Magic-Number value. (a) Acking or Rejecting the equal value, or Naking with the same value: TestEqualMagicNumberIsNaked goes red on len(acks) != 0, len(rejects) != 0, offered == localMagic (and offered == 0). Negative: a different value is Acked unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEqualMagicNumberIsNaked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_choice_test.go#L67) | unit/verify | revert, verified |
| positive | [`TestEqualMagicNumberIsNaked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_choice_test.go#L66) | unit/verify | revert, verified |

### [`RFC1661-6.4-7`](#rfc1661-6.4-7)

If the Magic-Number is equal to the one sent in the last Configure-Nak, the possibility of a looped-back link is increased, and a new Magic-Number MUST be chosen. (Section 6.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge ppp4 2026-09-30. The row's condition is now set up for real: rfc1661_magic_loop_test.go playMagicLoop sends ze's CONFREQ, loops it back, ze answers with a Configure-Nak naming a different Magic, and that Nak is looped back. TestRFC1661LoopedNakMagicChoosesNewMagic (+): new Magic differs from the held one, is non-zero and is carried in the next CONFREQ. TestRFC1661LoopedNakMagicRefusesRepeatedDraw (-, R1 b forced input): crypto/rand.Reader returns the old Magic first; the session must take the second draw; the targeted break dropping the `mag == s.magic` continue in redrawMagicOnNak turns it red (ppp4-tb-6.4-7.log), and adopting the looped Nak value also fails its exact equality. Old tags on TestMagicNumberRedrawnOnNak kept as supplementary: that unit's Nak names ze's request value, not the value of ze's last Nak, so it does not set up this row's condition and does not carry the verdict; its negative (Nak without a Magic option leaves Magic unchanged) proves no other RFC 1661 row, so there is no row to move it to. pppoeclient TestClientLCPMagicNakRedrawsAndUsesNegotiatedValue sets up the loop and goes red if generateDifferentMagic is omitted (client role, no record).

| Polarity | Test | Kind and tier | Proof state |
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

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReceivedMagicNumberMustBePeers`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L231) | unit/verify | revert, verified |
| negative | [`TestReceivedMagicNumberZeroWhenPeerNegotiatedNone`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L362) | unit/verify | revert, verified |
| positive | [`TestReceivedMagicNumberMustBePeers`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L230) | unit/verify | revert, verified |
| positive | [`TestReceivedMagicNumberZeroWhenPeerNegotiatedNone`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_magic_echo_test.go#L361) | unit/verify | revert, verified |

## Extraction sign-off

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
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 3 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `2` | not stated | 4 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 2 | walked | not stated |
| `3.2` | not stated | 1 | walked | not stated |
| `3.3` | not stated | 0 | walked | not stated |
| `3.4` | not stated | 1 | walked | not stated |
| `3.5` | not stated | 4 | walked | not stated |
| `3.6` | not stated | 3 | walked | not stated |
| `3.7` | not stated | 2 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 0 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `4.3` | not stated | 3 | walked | not stated |
| `4.4` | not stated | 2 | walked | not stated |
| `4.5` | not stated | 0 | walked | not stated |
| `4.6` | not stated | 4 | walked | not stated |
| `5` | not stated | 1 | walked | not stated |
| `5.1` | not stated | 3 | walked | not stated |
| `5.2` | not stated | 4 | walked | not stated |
| `5.3` | not stated | 8 | walked | not stated |
| `5.4` | not stated | 5 | walked | not stated |
| `5.5` | not stated | 2 | walked | not stated |
| `5.6` | not stated | 3 | walked | not stated |
| `5.7` | not stated | 4 | walked | not stated |
| `5.8` | not stated | 4 | walked | not stated |
| `5.9` | not stated | 4 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 1 | walked | not stated |
| `6.2` | not stated | 1 | walked | not stated |
| `6.3` | not stated | 0 | walked | not stated |
| `6.4` | not stated | 8 | walked | not stated |
| `6.5` | not stated | 3 | walked | not stated |
| `6.6` | not stated | 2 | walked | not stated |

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
