# RFC 1661 - The Point-to-Point Protocol (PPP)

Partial. Every requirement this repository extracted from RFC 1661, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 60.0% | 45 of 75 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 12.0% | 9 of 75 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 75 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 18.0% | 27 of 150 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 75 | of 100 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 6 | of 75 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 8.0% | 6 of 75 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 75 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 75 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 20.0% | 15 of 75 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 75 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
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
| Requirements | 100 |
| Gated MUST-level | 75 |
| Not applicable, so out of scope | 6 |
| Declared gaps | 15 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 150 |
| Tagged units | 150 |
| Recorded audit verdicts | 0 |
| Discrimination records | 27 |
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
| Positive and negative tests | 45 | one part of the gated population |
| Annotated instead of tested | 30 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **75** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (45):** [`RFC1661-2-1`](#rfc1661-2-1), [`RFC1661-6-2`](#rfc1661-6-2), [`RFC1661-3.1-2`](#rfc1661-3.1-2), [`RFC1661-3.5-1`](#rfc1661-3.5-1), [`RFC1661-3.5-3`](#rfc1661-3.5-3), [`RFC1661-3.6-1`](#rfc1661-3.6-1), [`RFC1661-3.6-3`](#rfc1661-3.6-3), [`RFC1661-4.3-1`](#rfc1661-4.3-1), [`RFC1661-5.1-1`](#rfc1661-5.1-1), [`RFC1661-5.1-2`](#rfc1661-5.1-2), [`RFC1661-5.1-3`](#rfc1661-5.1-3), [`RFC1661-5.2-1`](#rfc1661-5.2-1), [`RFC1661-5.2-2`](#rfc1661-5.2-2), [`RFC1661-5.2-3`](#rfc1661-5.2-3), [`RFC1661-5.2-4`](#rfc1661-5.2-4), [`RFC1661-5.3-1`](#rfc1661-5.3-1), [`RFC1661-5.3-2`](#rfc1661-5.3-2), [`RFC1661-5.3-3`](#rfc1661-5.3-3), [`RFC1661-5.3-5`](#rfc1661-5.3-5), [`RFC1661-5.3-6`](#rfc1661-5.3-6), [`RFC1661-5.3-7`](#rfc1661-5.3-7), [`RFC1661-5.3-8`](#rfc1661-5.3-8), [`RFC1661-5.4-1`](#rfc1661-5.4-1), [`RFC1661-5.4-2`](#rfc1661-5.4-2), [`RFC1661-5.4-3`](#rfc1661-5.4-3), [`RFC1661-5.4-4`](#rfc1661-5.4-4), [`RFC1661-5.4-5`](#rfc1661-5.4-5), [`RFC1661-5.5-1`](#rfc1661-5.5-1), [`RFC1661-5.6-1`](#rfc1661-5.6-1), [`RFC1661-5.7-1`](#rfc1661-5.7-1), [`RFC1661-5.7-4`](#rfc1661-5.7-4), [`RFC1661-5.8-1`](#rfc1661-5.8-1), [`RFC1661-5.8-2`](#rfc1661-5.8-2), [`RFC1661-6.4-1`](#rfc1661-6.4-1), [`RFC1661-6.4-2`](#rfc1661-6.4-2), [`RFC1661-6.4-3`](#rfc1661-6.4-3), [`RFC1661-4.4-2`](#rfc1661-4.4-2), [`RFC1661-5.5-2`](#rfc1661-5.5-2), [`RFC1661-5.8-4`](#rfc1661-5.8-4), [`RFC1661-5.8-5`](#rfc1661-5.8-5), [`RFC1661-6.1-1`](#rfc1661-6.1-1), [`RFC1661-6.4-5`](#rfc1661-6.4-5), [`RFC1661-6.4-6`](#rfc1661-6.4-6), [`RFC1661-6.4-7`](#rfc1661-6.4-7), [`RFC1661-6.4-8`](#rfc1661-6.4-8)

**Annotated instead of tested (30):** [`RFC1661-2-2`](#rfc1661-2-2), [`RFC1661-5-1`](#rfc1661-5-1), [`RFC1661-3.1-1`](#rfc1661-3.1-1), [`RFC1661-3.4-1`](#rfc1661-3.4-1), [`RFC1661-3.5-2`](#rfc1661-3.5-2), [`RFC1661-3.5-4`](#rfc1661-3.5-4), [`RFC1661-3.6-2`](#rfc1661-3.6-2), [`RFC1661-3.7-1`](#rfc1661-3.7-1), [`RFC1661-3.7-2`](#rfc1661-3.7-2), [`RFC1661-4.3-2`](#rfc1661-4.3-2), [`RFC1661-4.3-3`](#rfc1661-4.3-3), [`RFC1661-5.3-4`](#rfc1661-5.3-4), [`RFC1661-5.6-2`](#rfc1661-5.6-2), [`RFC1661-5.6-3`](#rfc1661-5.6-3), [`RFC1661-5.7-2`](#rfc1661-5.7-2), [`RFC1661-5.7-3`](#rfc1661-5.7-3), [`RFC1661-5.9-1`](#rfc1661-5.9-1), [`RFC1661-5.9-2`](#rfc1661-5.9-2), [`RFC1661-6.2-1`](#rfc1661-6.2-1), [`RFC1661-6.5-1`](#rfc1661-6.5-1), [`RFC1661-6.5-2`](#rfc1661-6.5-2), [`RFC1661-6.5-3`](#rfc1661-6.5-3), [`RFC1661-6.6-1`](#rfc1661-6.6-1), [`RFC1661-6.6-2`](#rfc1661-6.6-2), [`RFC1661-4.6-1`](#rfc1661-4.6-1), [`RFC1661-4.6-2`](#rfc1661-4.6-2), [`RFC1661-4.6-3`](#rfc1661-4.6-3), [`RFC1661-4.6-4`](#rfc1661-4.6-4), [`RFC1661-4.4-1`](#rfc1661-4.4-1), [`RFC1661-5.9-3`](#rfc1661-5.9-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1661-2-1` | Protocol field: LSB of least-significant octet must equal 1; LSB of most-significant octet must equal 0; frames violating these rules must be treated as unrecognized Protocol (Section 2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC1661CompliantProtocolRecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L238). **negative:** `unit/verify` [`TestRFC1661NonCompliantProtocolTreatedUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L203) |
| `RFC1661-2-2` | Information field plus Padding must fit within peer's MRU (default 1500) (Section 2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** getFrameBuf in internal/component/l2tp/ppp/session_run.go supplies MaxFrameBufLen bytes, enough for 1500 Information octets and the Protocol field. sendProtocolReject clamps to the peer MRU, but sendCodeReject still truncates only against that buffer, so a large Code-Reject can exceed a smaller peer MRU. Disclosed in docs/features/rfc-status.md |
| `RFC1661-5-1` | LCP Length must not exceed the MRU of the link (Section 5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** WriteLCPPacket in internal/component/l2tp/ppp/lcp.go backfills the packet length from its supplied payload; sendCodeReject in internal/component/l2tp/ppp/session_run.go bounds that payload by the default-sized Information field rather than a smaller negotiatedMRU. Protocol-Reject has a separate peer-MRU clamp. Disclosed in docs/features/rfc-status.md |
| `RFC1661-6-1` | A negotiable Configuration Option received in a Configure-Request with an invalid or unrecognized Length should draw a Configure-Nak carrying the desired Configuration Option with an appropriate Length and Data (Section 6) | SHOULD | 6 | **positive:** `unit/verify` [`TestRFC1661ClientInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L153). **positive:** `unit/verify` [`TestRFC1661InvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L154). **positive:** `unit/verify` [`TestRFC1661LCPInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L318). **positive:** `unit/verify` [`TestRFC1661LCPWrongLengthMagicIsNakedNotRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L404). **positive:** `unit/verify` [`TestRFC1661ReplyListsEachOptionTypeOnce`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L744). **negative:** `unit/verify` [`TestRFC1661ClientInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L159). **negative:** `unit/verify` [`TestRFC1661InvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L159). **negative:** `unit/verify` [`TestRFC1661LCPInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L325). **negative:** `unit/verify` [`TestRFC1661LCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L465). **negative:** `unit/verify` [`TestRFC1661LCPWrongLengthMagicIsNakedNotRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L408). **negative:** `unit/verify` [`TestRFC1661ReplyListsEachOptionTypeOnce`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L748) |
| `RFC1661-6-2` | A Configuration Option whose Data is indicated by its Length to extend beyond the end of the Information field must cause the entire packet to be silently discarded without affecting the automaton (Section 6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC1661ClientRequestPastEndSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L109). **positive:** `unit/verify` [`TestRFC1661LCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L584). **positive:** `unit/verify` [`TestRFC1661LCPTruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L236). **positive:** `unit/verify` [`TestRFC1661NCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L675). **positive:** `unit/verify` [`TestRFC1661TruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L75). **negative:** `unit/verify` [`TestRFC1661ClientRequestPastEndSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L116). **negative:** `unit/verify` [`TestRFC1661LCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L590). **negative:** `unit/verify` [`TestRFC1661LCPTruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L243). **negative:** `unit/verify` [`TestRFC1661NCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L680). **negative:** `unit/verify` [`TestRFC1661TruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L81) |
| `RFC1661-3.1-1` | In order to establish communications over a point-to-point link, each end of the PPP link MUST first send LCP packets to configure and test the data link. (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC1661LCPPacketsSentFirst`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L307). **negative:** no negative test. **{single-polarity}:** run (internal/component/l2tp/ppp/session_run.go:182-197) drives the synthetic Initial->Closed->ReqSent sequence whose scr action puts an LCP Configure-Request on the wire before any other traffic, and the sole branch that skips it is the RFC 2661 Section 18 proxy-LCP path where the LAC has already run LCP, so there is no case in which ze opens a link with LCP packets unsent |
| `RFC1661-3.1-2` | Then, PPP MUST send NCP packets to choose and configure one or more network-layer protocols. (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC1661NCPConfiguresEachFamilySeparately`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L335). **negative:** `unit/verify` [`TestRFC1661NoNCPPacketsWhenNoNetworkProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L374) |
| `RFC1661-3.4-1` | Any non-LCP packets received during this phase MUST be silently discarded. (Section 3.4) | MUST | 3.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handleFrame in internal/component/l2tp/ppp/session_run.go now discards IPCP/IPv6CP before the current authenticated LCP lifetime is admitted. Bounded phase and lifetime tests exist; complete proof across all protocol types remains open. |
| `RFC1661-3.5-1` | If an implementation desires that the peer authenticate with some specific authentication protocol, then it MUST request the use of that authentication protocol during Link Establishment phase. (Section 3.5) | MUST | 3.5 | **positive:** `unit/verify` [`TestRFC1661AuthProtocolRequestedDuringEstablishment`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L446). **negative:** `unit/verify` [`TestRFC1661NoAuthProtocolWhenNotDesired`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L477) |
| `RFC1661-3.5-2` | An implementation MUST NOT allow the exchange of link quality determination packets to delay authentication indefinitely. (Section 3.5) | MUST NOT | 3.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze implements no link-quality determination protocol. The LCP Quality-Protocol option (type 4) has no constant in internal/component/l2tp/ppp/lcp_options.go:14-21 and negotiatePeerOption (internal/component/l2tp/ppp/lcp_options.go) Configure-Rejects it as an unknown type; a grep for LQR, 0xC025 and Quality-Protocol across internal/ matches only that lcp_options.go comment naming type 4 as unimplemented |
| `RFC1661-3.5-3` | Advancement from the Authentication phase to the Network-Layer Protocol phase MUST NOT occur until authentication has completed. (Section 3.5) | MUST NOT | 3.5 | **positive:** `unit/verify` [`TestRFC1661NoNetworkPhaseUntilAuthCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L504). **negative:** `unit/verify` [`TestRFC1661NetworkPhaseRunsAfterAuthCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L537) |
| `RFC1661-3.5-4` | All other packets received during this phase MUST be silently discarded. (Section 3.5) | MUST | 3.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handleFrame in internal/component/l2tp/ppp/session_run.go now discards IPCP/IPv6CP before authentication completes rather than retaining early frames. Bounded phase and lifetime tests exist; complete proof across all protocol types remains open. |
| `RFC1661-3.6-1` | Once PPP has finished the previous phases, each network-layer protocol (such as IP, IPX, or AppleTalk) MUST be separately configured by the appropriate Network Control Protocol (NCP). (Section 3.6) | MUST | 3.6 | **positive:** `unit/verify` [`TestRFC1661NCPConfiguresEachFamilySeparately`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L339). **negative:** `unit/verify` [`TestRFC1661NCPStatesAreIndependent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L391) |
| `RFC1661-3.6-2` | Any supported network-layer protocol packets received when the corresponding NCP is not in the Opened state MUST be silently discarded. (Section 3.6) | MUST | 3.6 | **positive:** `unit/verify` [`TestRFC1661NetworkLayerPacketDiscardedBeforeNCPOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L410). **negative:** no negative test. **{single-polarity}:** handleFrame (internal/component/l2tp/ppp/session_run.go) dispatches only the three control protocols, and rejectUnsupportedProtocol discards an IPv4 (0x0021) or IPv6 (0x0057) frame whose NCP ze supports in every NCP state, so no state exists in which a network-layer packet is processed in userspace and there is no accepting counterpart to assert |
| `RFC1661-3.6-3` | While LCP is in the Opened state, any protocol packet which is unsupported by the implementation MUST be returned in a Protocol- Reject (described later). (Section 3.6) | MUST | 3.6 | **positive:** `unit/verify` [`TestRFC1661UnsupportedProtocolRejectedInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L46). **negative:** `unit/verify` [`TestRFC1661UnsupportedProtocolNotRejectedBeforeOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L94) |
| `RFC1661-3.7-1` | Receiver of Terminate-Request must not disconnect until at least one Restart time after sending Terminate-Ack (Section 3.7) | MUST | 3.7 | **positive:** `unit/verify` [`TestRFC1661TerminateAckSentAndLinkHeld`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L569). **negative:** no negative test. **{single-polarity}:** the Opened+RTR edge (internal/component/l2tp/ppp/ppp_fsm.go:393-394) lands in Stopping and handleLCPPacket emits EventSessionDown only for Closed or Stopped (internal/component/l2tp/ppp/session_run.go), so after sending a Terminate-Ack ze holds the link; there is no early-disconnect branch to assert |
| `RFC1661-3.7-2` | Any non-LCP packets received during this phase MUST be silently discarded. (Section 3.7) | MUST | 3.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handleFrame and performAction in internal/component/l2tp/ppp/session_run.go now gate NCP on the current LCP lifetime and clear admission when that lifetime ends. Bounded lifecycle tests exist; complete requirement-level proof remains open. |
| `RFC1661-4.3-1` | The implementation MUST be prepared to immediately renegotiate the Configuration Options. (Section 4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC1661RenegotiateOnConfigureRequestInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L629). **negative:** `unit/verify` [`TestRFC1661EchoDoesNotRenegotiate`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L662) |
| `RFC1661-4.3-2` | The implementation MUST be prepared to receive a new Configure-Request without network administrator intervention. (Section 4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC1661NewConfigureRequestAcceptedAfterTerminateRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L682). **negative:** no negative test. **{single-polarity}:** every post-RTR state in LCPDoTransition accepts a fresh RCR+ (internal/component/l2tp/ppp/ppp_fsm.go:295-296, :327-328, :359-360) and no code path consults an administrative flag before doing so, so there is no refusing counterpart to assert |
| `RFC1661-4.3-3` | The implementation MUST stop sending the offending packet type. (Section 4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** codeToEvent and handleLCPPacket in internal/component/l2tp/ppp/session_run.go handle received rejects without retaining an offending-code suppression set. The Opened RXJ+ transition can send an Echo-Reply; it does not prevent later transmission of the rejected packet type. |
| `RFC1661-5.1-1` | An implementation wishing to open a connection MUST transmit a Configure-Request. (Section 5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestRFC1661OpenTransmitsConfigureRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L708). **negative:** `unit/verify` [`TestRFC1661UpWithoutOpenSendsNoConfigureRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L739) |
| `RFC1661-5.1-2` | Upon reception of a Configure-Request, an appropriate reply MUST be transmitted. (Section 5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestRFC1661ConfigureAckEchoesOptionsVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L757). **negative:** `unit/verify` [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L821) |
| `RFC1661-5.1-3` | The Identifier field MUST be changed whenever the contents of the Options field changes, and whenever a valid reply has been received for a previous request. (Section 5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L80). **positive:** `unit/verify` [`TestLCPConfigureRequestIdentifierLifecycle`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L492). **negative:** `unit/verify` [`TestClientLCPNakChangesIdentifierAndDropsStaleReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L145). **negative:** `unit/verify` [`TestLCPConfigureRequestIdentifierLifecycle`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L493) |
| `RFC1661-5.2-1` | If every Configuration Option received in a Configure-Request is recognizable and all values are acceptable, then the implementation MUST transmit a Configure-Ack. (Section 5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestRFC1661ConfigureAckEchoesOptionsVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L761). **negative:** `unit/verify` [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L825) |
| `RFC1661-5.2-2` | Acknowledged Configuration Options must not be reordered or modified (Section 5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestRFC1661ConfigureAckEchoesOptionsVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L764). **negative:** `unit/verify` [`TestRFC1661ConfigureAckDoesNotReorderOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L790) |
| `RFC1661-5.2-3` | On reception of a Configure-Ack, the Identifier field MUST match that of the last transmitted Configure-Request. (Section 5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L18). **positive:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L399). **negative:** `unit/verify` [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L19). **negative:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L400) |
| `RFC1661-5.2-4` | Additionally, the Configuration Options in a Configure-Ack MUST exactly match those of the last transmitted Configure-Request. (Section 5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L20). **positive:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L401). **negative:** `unit/verify` [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L21). **negative:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L402) |
| `RFC1661-5.3-1` | If all options recognized but some values unacceptable, must transmit Configure-Nak (Section 5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC1661ConfigureNakSuggestsAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L911). **negative:** `unit/verify` [`TestRFC1661NoNakForAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L946) |
| `RFC1661-5.3-2` | Options which have no value fields (boolean options) MUST use the Configure-Reject reply instead. (Section 5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC1661BooleanOptionsUseRejectNotNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L976). **negative:** `unit/verify` [`TestRFC1661ValuedOptionUsesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L998) |
| `RFC1661-5.3-3` | Each Configuration Option which is allowed only a single instance MUST be modified to a value acceptable to the Configure-Nak sender. (Section 5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC1661ConfigureNakSuggestsAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L915). **positive:** `unit/verify` [`TestRFC1661RepeatedOptionDrawsOneNakEntry`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L345). **negative:** `unit/verify` [`TestRFC1661NoNakForAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L949) |
| `RFC1661-5.3-4` | When a particular type of Configuration Option can be listed more than once with different values, the Configure-Nak MUST include a list of all values for that option which are acceptable to the Configure-Nak sender. (Section 5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the requirement is conditional on an option type that "can be listed more than once with different values", and RFC 1661 Section 6 says of its own set: "(None of the Configuration Options in this specification can be listed more than once.)" Ze implements types 1, 2, 3, 5, 7 and 8 (internal/component/l2tp/ppp/lcp_options.go:14-21), none of them multi-instance, so no input gives ze a list of acceptable values to send. The earlier reason here read that vacuity off NegotiatePeerOptions (internal/component/l2tp/ppp/lcp_options.go), which emits one Nak entry per received OPTION rather than per Type; keeping the reply to one entry per Type is appendUnlessListed (internal/component/l2tp/ppp/session_run.go), not this row |
| `RFC1661-5.3-5` | Any value fields for the option MUST indicate values acceptable to the Configure-Nak sender. (Section 5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC1661NakValueIsAcceptable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1015). **negative:** `unit/verify` [`TestRFC1661RejectedValueStaysUnacceptable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1037) |
| `RFC1661-5.3-6` | Options from Configure-Request must not be reordered in Configure-Nak (Section 5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC1661NakPreservesRequestOrder`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1054). **negative:** `unit/verify` [`TestRFC1661NakOrderFollowsRequestNotAFixedOrder`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1082) |
| `RFC1661-5.3-7` | On reception of a Configure-Nak, the Identifier field MUST match that of the last transmitted Configure-Request. (Section 5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestClientLCPNakChangesIdentifierAndDropsStaleReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L144). **positive:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L403). **negative:** `unit/verify` [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L22). **negative:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L404) |
| `RFC1661-5.3-8` | Since the Nak'd Option has been modified by the peer, the implementation MUST be able to handle an Option length which is different from the original Configure-Request. (Section 5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC1661NakHandlesDifferentOptionLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1103). **negative:** `unit/verify` [`TestRFC1661NakTooShortOptionNotDecoded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1125) |
| `RFC1661-5.4-1` | If some Configuration Options received in a Configure-Request are not recognizable or are not acceptable for negotiation (as configured by a network administrator), then the implementation MUST transmit a Configure-Reject. (Section 5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestRFC1661ClientUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L202). **positive:** `unit/verify` [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L828). **positive:** `unit/verify` [`TestRFC1661LCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L459). **positive:** `unit/verify` [`TestRFC1661NCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L526). **negative:** `unit/verify` [`TestRFC1661ClientAcceptsServerAuthProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L359). **negative:** `unit/verify` [`TestRFC1661ClientUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L207). **negative:** `unit/verify` [`TestRFC1661NCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L531). **negative:** `unit/verify` [`TestRFC1661NoConfigureRejectWhenAllOptionsRecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L859) |
| `RFC1661-5.4-2` | Configure-Reject options must not be reordered or modified (Section 5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestRFC1661ClientRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L238). **positive:** `unit/verify` [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L831). **positive:** `unit/verify` [`TestRFC1661LCPRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L828). **negative:** `unit/verify` [`TestRFC1661ClientRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L250). **negative:** `unit/verify` [`TestRFC1661ConfigureRejectDoesNotReorderOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L877). **negative:** `unit/verify` [`TestRFC1661LCPRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L839) |
| `RFC1661-5.4-3` | On reception of a Configure-Reject, the Identifier field MUST match that of the last transmitted Configure-Request. (Section 5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L76). **positive:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L405). **negative:** `unit/verify` [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L23). **negative:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L406) |
| `RFC1661-5.4-4` | Additionally, the Configuration Options in a Configure-Reject MUST be a proper subset of those in the last transmitted Configure- Request. (Section 5.4, Errata 543) | MUST | 5.4 | **positive:** `unit/verify` [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L77). **positive:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L407). **negative:** `unit/verify` [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L24). **negative:** `unit/verify` [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L408) |
| `RFC1661-5.4-5` | Next Configure-Request must not include any rejected options (Section 5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L78). **positive:** `unit/verify` [`TestLCPRejectedOptionsStayRemoved`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L533). **negative:** `unit/verify` [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L79). **negative:** `unit/verify` [`TestLCPRejectedOptionsStayRemoved`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L534) |
| `RFC1661-5.5-1` | Upon reception of a Terminate-Request, a Terminate-Ack MUST be transmitted. (Section 5.5) | MUST | 5.5 | **positive:** `unit/verify` [`TestRFC1661TerminateAckSentAndLinkHeld`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L574). **negative:** `unit/verify` [`TestRFC1661NoTerminateAckForTerminateAck`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L608) |
| `RFC1661-5.6-1` | This MUST be reported back to the sender of the unknown Code by transmitting a Code- Reject. (Section 5.6) | MUST | 5.6 | **positive:** `unit/verify` [`TestRFC1661CodeRejectForUnknownCode`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1147). **negative:** `unit/verify` [`TestRFC1661NoCodeRejectForKnownCode`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1179) |
| `RFC1661-5.6-2` | The Identifier field MUST be changed for each Code-Reject sent. (Section 5.6) | MUST | 5.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** sendCodeReject (internal/component/l2tp/ppp/session_run.go) reuses the offending packet's Identifier for the Code-Reject instead of allocating a fresh one, so the Identifier does not change per Code-Reject sent. Disclosed in docs/features/rfc-status.md |
| `RFC1661-5.6-3` | The Rejected-Packet MUST be truncated to comply with the peer's established MRU. (Section 5.6) | MUST | 5.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** sendCodeReject in internal/component/l2tp/ppp/session_run.go truncates the Rejected-Packet against the 1500-octet Information field in getFrameBuf's MaxFrameBufLen buffer, not against a smaller negotiatedMRU. Disclosed in docs/features/rfc-status.md |
| `RFC1661-5.7-1` | If the LCP automaton is in the Opened state, then this MUST be reported back to the peer by transmitting a Protocol-Reject. (Section 5.7) | MUST | 5.7 | **positive:** `unit/verify` [`TestRFC1661UnsupportedProtocolRejectedInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L40). **negative:** `unit/verify` [`TestRFC1661UnsupportedProtocolNotRejectedBeforeOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L89) |
| `RFC1661-5.7-2` | Upon reception of a Protocol-Reject, the implementation MUST stop sending packets of the indicated protocol at the earliest opportunity. (Section 5.7) | MUST | 5.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handleLCPPacket in internal/component/l2tp/ppp/session_run.go maps the rejection to RXJ+ but records no rejected-protocol set. The FSM transition does not implement suppression of later packets of that protocol. |
| `RFC1661-5.7-3` | The Identifier field MUST be changed for each Protocol-Reject sent. (Section 5.7) | MUST | 5.7 | **positive:** `unit/verify` [`TestRFC1661ProtocolRejectIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L137). **negative:** no negative test. **{single-polarity}:** sendProtocolReject (internal/component/l2tp/ppp/session_run.go) advances protocolRejectID before every packet it writes, and no input asks ze to hold the Identifier still, so the obligation has no refusing counterpart to assert |
| `RFC1661-5.7-4` | The Rejected-Information MUST be truncated to comply with the peer's established MRU. (Section 5.7) | MUST | 5.7 | **positive:** `unit/verify` [`TestRFC1661ProtocolRejectInformationTruncatedToMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L166). **negative:** `unit/verify` [`TestRFC1661ProtocolRejectInformationKeptWhenItFits`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L201) |
| `RFC1661-5.8-1` | Upon reception of an Echo-Request in the LCP Opened state, an Echo-Reply MUST be transmitted. (Section 5.8) | MUST | 5.8 | **positive:** `unit/verify` [`TestRFC1661EchoReplyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1200). **negative:** `unit/verify` [`TestRFC1661NoEchoOutsideOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1242) |
| `RFC1661-5.8-2` | Echo-Request and Echo-Reply packets MUST only be sent in the LCP Opened state. (Section 5.8) | MUST | 5.8 | **positive:** `unit/verify` [`TestRFC1661NoEchoOutsideOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1246). **negative:** `unit/verify` [`TestClientLCPEchoBeforeOpenDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_transition_test.go#L115). **negative:** `unit/verify` [`TestRFC1661EchoReplyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1203) |
| `RFC1661-5.9-1` | Discard-Request packets MUST only be sent in the LCP Opened state. (Section 5.9) | MUST | 5.9 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never transmits a Discard-Request. A grep for LCPDiscardRequest across internal/ matches only the constant (internal/component/l2tp/ppp/lcp.go:28), LCPCodeName (internal/component/l2tp/ppp/lcp.go:125) and the receive-side codeToEvent mapping (internal/component/l2tp/ppp/session_run.go:705) |
| `RFC1661-5.9-2` | On reception, the receiver MUST silently discard any Discard- Request that it receives. (Section 5.9) | MUST | 5.9 | **positive:** `unit/verify` [`TestRFC1661DiscardRequestSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1268). **negative:** no negative test. **{single-polarity}:** the obligation covers ANY Discard-Request, so no conforming Discard-Request exists that must instead draw a reply, and no input can make the required silence wrong. The discrimination against a receiver that answers nothing at all is carried by TestRFC1661EchoReplyInOpened, which is tagged for the Echo requirements it actually drives (RFC1661-5.8-1, RFC1661-5.8-2), not for this one |
| `RFC1661-6.2-1` | An implementation MUST NOT include multiple Authentication- Protocol Configuration Options in its Configure-Request packets. (Section 6.2) | MUST NOT | 6.2 | **positive:** `unit/verify` [`TestRFC1661SingleAuthProtocolOptionInRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1300). **negative:** no negative test. **{single-polarity}:** LCPOptions.AuthProto is a single uint16 (internal/component/l2tp/ppp/lcp_options.go) and BuildLocalConfigRequest appends the option once (internal/component/l2tp/ppp/lcp_options.go), so no input produces two Authentication-Protocol options and there is no violating case to assert |
| `RFC1661-6.4-1` | If an implementation does transmit a Configure-Request with a Magic-Number Configuration Option, then it MUST NOT respond with a Configure-Reject when it receives a Configure-Request with a Magic-Number Configuration Option. (Section 6.4) | MUST NOT | 6.4 | **positive:** `unit/verify` [`TestRFC1661ZeroMagicNumberRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1378). **negative:** `unit/verify` [`TestRFC1661UnknownOptionRejectedWhileMagicIsNot`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1350) |
| `RFC1661-6.4-2` | If Magic-Number has been successfully negotiated, an implementation MUST transmit these packets with the Magic-Number field set to its negotiated Magic-Number. (Section 6.4) | MUST | 6.4 | **positive:** `unit/verify` [`TestRFC1661EchoReplyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1213). **negative:** `unit/verify` [`TestRFC1661EchoReplyDoesNotMirrorPeerMagic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1417) |
| `RFC1661-6.4-3` | A Magic-Number of zero is illegal and MUST always be Nak'd, if it is not Rejected outright. (Section 6.4) | MUST | 6.4 | **positive:** `unit/verify` [`TestRFC1661ClientNaksZeroMagicInAWellFormedRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_client_negotiation_test.go#L44). **positive:** `unit/verify` [`TestRFC1661ClientNaksZeroMagicWithAValueOfItsOwn`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L407). **positive:** `unit/verify` [`TestRFC1661ZeroMagicNumberRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1370). **negative:** `unit/verify` [`TestRFC1661ClientAcksANonZeroMagic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_client_negotiation_test.go#L82). **negative:** `unit/verify` [`TestRFC1661ClientNaksZeroMagicWithAValueOfItsOwn`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L413). **negative:** `unit/verify` [`TestRFC1661PeerMagicNumberAcked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1331) |
| `RFC1661-6.5-1` | By default, all implementations MUST transmit packets with two octet PPP Protocol fields. (Section 6.5) | MUST | 6.5 | **positive:** `unit/verify` [`TestRFC1661TwoOctetProtocolField`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L259). **negative:** no negative test. **{single-polarity}:** this is a transmit obligation and WriteFrame has no compressed branch at all -- WriteFrame always writes the Protocol with binary.BigEndian.PutUint16 (internal/component/l2tp/ppp/frame.go:81-85), so no configuration or negotiated option produces a single-octet transmit to assert against. The positive test drives both PFC settings to show the option cannot change the encoder; the receive-side refusal of a one-octet Protocol is the separate RFC1661-6.5-3 |
| `RFC1661-6.5-2` | Compressed Protocol fields MUST NOT be transmitted unless this Configuration Option has been negotiated. (Section 6.5) | MUST NOT | 6.5 | **positive:** `unit/verify` [`TestRFC1661TwoOctetProtocolField`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L263). **negative:** no negative test. **{single-polarity}:** WriteFrame (internal/component/l2tp/ppp/frame.go:81-85) has no compressed-Protocol branch, so ze transmits an uncompressed Protocol field whether or not the option is negotiated and there is no compressed-transmit case to contrast |
| `RFC1661-6.5-3` | When PFC negotiated, must accept both single-octet and double-octet Protocol fields (Section 6.5) | MUST | 6.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** whole-stack reception remains unverified. ParseFrame in internal/component/l2tp/ppp/frame.go reads a two-octet Protocol field, but that alone does not establish a failure after applicable directional negotiation and Linux PPP receive normalization. The negotiated kernel boundary needs proof. |
| `RFC1661-6.6-1` | By default, all implementations MUST transmit frames with Address and Control fields appropriate to the link framing. (Section 6.6) | MUST | 6.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze performs no HDLC-like framing. It writes protocol-plus-payload frames to a /dev/ppp channel fd (WriteFrame, internal/component/l2tp/ppp/frame.go:81-85) and the kernel PPP driver supplies the Address and Control octets; a grep for 0xFF03 and HDLC across internal/ matches only two comments in internal/component/l2tp/ppp/lcp_options.go, one on desiredLCPOption and one on negotiatePeerOption, each recording that the kernel does the framing |
| `RFC1661-6.6-2` | The Address and Control fields MUST NOT be compressed when sending any LCP packet. (Section 6.6) | MUST NOT | 6.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze emits no Address or Control field on any packet, LCP included: WriteFrame (internal/component/l2tp/ppp/frame.go:81-85) writes only the Protocol field and payload, and a grep for 0xFF03 and HDLC across internal/ matches only two comments in internal/component/l2tp/ppp/lcp_options.go, one on desiredLCPOption and one on negotiatePeerOption, each recording that the kernel supplies the framing |
| `RFC1661-4.6-1` | Restart timer must be configurable (Section 4.6) | MUST | 4.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** armRestartTimer in internal/component/l2tp/ppp/session_run.go uses the fixed defaultRestartTimer of three seconds; StartSession in internal/component/l2tp/ppp/start_session.go carries no restart-timer field and no YANG leaf sets one. Disclosed in docs/features/rfc-status.md |
| `RFC1661-4.6-2` | Max-Terminate must be configurable (Section 4.6) | MUST | 4.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** applyTransition in internal/component/l2tp/ppp/session_run.go initializes restartCount from the fixed defaultMaxTerminate of two transmissions, with no session or YANG setting for that limit. Disclosed in docs/features/rfc-status.md |
| `RFC1661-4.6-3` | Max-Configure must be configurable (Section 4.6) | MUST | 4.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** applyTransition in internal/component/l2tp/ppp/session_run.go initializes restartCount from the fixed defaultMaxConfigure of ten transmissions, with no session or YANG setting for that limit. Disclosed in docs/features/rfc-status.md |
| `RFC1661-4.6-4` | Max-Failure must be configurable (Section 4.6) | MUST | 4.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze counts no Configure-Naks sent; sendConfigureNakOrReject (internal/component/l2tp/ppp/session_run.go) picks Nak or Reject from the LCPNakOrReject verdict over NegotiatePeerOptions output on each request, so there is no Max-Failure value to configure and no threshold that converts a Nak into a Reject. Disclosed in docs/features/rfc-status.md |
| `RFC1661-4.4-1` | In addition to setting the Restart counter, the implementation MUST set the timeout period to the initial value when Restart timer backoff is used. (Section 4.4) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze applies no Restart timer backoff. internal/component/l2tp/ppp/session_run.go resets its session Restart timer to the fixed defaultRestartTimer, so the condition "when Restart timer backoff is used" never holds |
| `RFC1661-4.4-2` | In addition to zeroing the Restart counter, the implementation MUST set the timeout period to an appropriate value. (Section 4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestLCPPeerTerminateRestartTimer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_restart_counter_test.go#L135). **negative:** `unit/verify` [`TestLCPPeerTerminateRestartTimer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_restart_counter_test.go#L136) |
| `RFC1661-5.5-2` | On transmission of a Terminate-Request or Terminate-Ack, "the Identifier field MUST be changed whenever the content of the Data field changes, and whenever a valid reply has been received for a previous request" (Section 5.5) | MUST | 5.5 | **positive:** `unit/verify` [`TestTerminateRequestIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_echo_rfc1661_test.go#L40). **negative:** `unit/verify` [`TestTerminateRequestIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_echo_rfc1661_test.go#L41) |
| `RFC1661-5.8-4` | On transmission of an Echo-Request or Echo-Reply, "the Identifier field MUST be changed whenever the content of the Data field changes, and whenever a valid reply has been received for a previous request" (Section 5.8) | MUST | 5.8 | **positive:** `unit/verify` [`TestEchoRequestIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_echo_rfc1661_test.go#L70). **negative:** `unit/verify` [`TestEchoRequestIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_echo_rfc1661_test.go#L71) |
| `RFC1661-5.8-5` | In Echo-Request, Echo-Reply and Discard-Request packets, "Until the Magic-Number Configuration Option has been successfully negotiated, the Magic-Number MUST be transmitted as zero" (Section 5.8) | MUST | 5.8 | **positive:** `unit/verify` [`TestMagicNumberZeroUntilNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_echo_rfc1661_test.go#L110). **negative:** `unit/verify` [`TestMagicNumberZeroUntilNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_echo_rfc1661_test.go#L111) |
| `RFC1661-5.9-3` | The Identifier field MUST be changed for each Discard-Request sent. (Section 5.9) | MUST | 5.9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze sends no Discard-Request, so no Identifier is drawn for one; plan/spec-ppp-discard-request-sender.md |
| `RFC1661-6.1-1` | If smaller packets are requested, an implementation MUST still be able to receive the full 1500 octet information field in case link synchronization is lost. (Section 6.1) | MUST | 6.1 | **positive:** `unit/verify` [`TestFullInformationFieldReceived`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_echo_rfc1661_test.go#L258). **negative:** `unit/verify` [`TestFullInformationFieldReceived`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_echo_rfc1661_test.go#L259) |
| `RFC1661-6.4-5` | Before this Configuration Option is requested, an implementation MUST choose its Magic-Number. (Section 6.4) | MUST | 6.4 | **positive:** `unit/verify` [`TestMagicNumberChosenBeforeRequested`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_choice_rfc1661_test.go#L20). **negative:** `unit/verify` [`TestMagicNumberChosenBeforeRequested`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_choice_rfc1661_test.go#L21) |
| `RFC1661-6.4-6` | When a Configure-Request carries a Magic-Number equal to the one last sent to the peer, "a Configure-Nak MUST be sent specifying a different Magic-Number value" (Section 6.4) | MUST | 6.4 | **positive:** `unit/verify` [`TestEqualMagicNumberIsNaked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_choice_rfc1661_test.go#L66). **negative:** `unit/verify` [`TestEqualMagicNumberIsNaked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_choice_rfc1661_test.go#L67) |
| `RFC1661-6.4-7` | If the Magic-Number is equal to the one sent in the last Configure-Nak, the possibility of a looped-back link is increased, and a new Magic-Number MUST be chosen. (Section 6.4) | MUST | 6.4 | **positive:** `unit/verify` [`TestClientLCPMagicNakRedrawsAndUsesNegotiatedValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L185). **positive:** `unit/verify` [`TestMagicNumberRedrawnOnNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_echo_rfc1661_test.go#L168). **negative:** `unit/verify` [`TestClientLCPMagicNakRedrawsAndUsesNegotiatedValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L186). **negative:** `unit/verify` [`TestMagicNumberRedrawnOnNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_echo_rfc1661_test.go#L169) |
| `RFC1661-6.4-8` | All received Magic-Number fields MUST be equal to either zero or the peer's unique Magic-Number, depending on whether or not the peer negotiated a Magic-Number. (Section 6.4) | MUST | 6.4 | **positive:** `unit/verify` [`TestReceivedMagicNumberMustBePeers`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_echo_rfc1661_test.go#L206). **negative:** `unit/verify` [`TestReceivedMagicNumberMustBePeers`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_echo_rfc1661_test.go#L207) |
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
| [`RFC1661-2-2`](#rfc1661-2-2) Information field plus Padding must fit within peer's MRU (default 1500) (Section 2) | {gap}, no test | getFrameBuf in internal/component/l2tp/ppp/session_run.go supplies MaxFrameBufLen bytes, enough for 1500 Information octets and the Protocol field. sendProtocolReject clamps to the peer MRU, but sendCodeReject still truncates only against that buffer, so a large Code-Reject can exceed a smaller peer MRU. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-5-1`](#rfc1661-5-1) LCP Length must not exceed the MRU of the link (Section 5) | {gap}, no test | WriteLCPPacket in internal/component/l2tp/ppp/lcp.go backfills the packet length from its supplied payload; sendCodeReject in internal/component/l2tp/ppp/session_run.go bounds that payload by the default-sized Information field rather than a smaller negotiatedMRU. Protocol-Reject has a separate peer-MRU clamp. Disclosed in docs/features/rfc-status.md |
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
| [`RFC1661-6.5-3`](#rfc1661-6.5-3) When PFC negotiated, must accept both single-octet and double-octet Protocol fields (Section 6.5) | {gap}, no test | whole-stack reception remains unverified. ParseFrame in internal/component/l2tp/ppp/frame.go reads a two-octet Protocol field, but that alone does not establish a failure after applicable directional negotiation and Linux PPP receive normalization. The negotiated kernel boundary needs proof. |
| [`RFC1661-6.6-1`](#rfc1661-6.6-1) By default, all implementations MUST transmit frames with Address and Control fields appropriate to the link framing. (Section 6.6) | no test | no test carries this requirement id; annotated {not-applicable}: ze performs no HDLC-like framing. It writes protocol-plus-payload frames to a /dev/ppp channel fd (WriteFrame, internal/component/l2tp/ppp/frame.go:81-85) and the kernel PPP driver supplies the Address and Control octets; a grep for 0xFF03 and HDLC across internal/ matches only two comments in internal/component/l2tp/ppp/lcp_options.go, one on desiredLCPOption and one on negotiatePeerOption, each recording that the kernel does the framing |
| [`RFC1661-6.6-2`](#rfc1661-6.6-2) The Address and Control fields MUST NOT be compressed when sending any LCP packet. (Section 6.6) | no test | no test carries this requirement id; annotated {not-applicable}: ze emits no Address or Control field on any packet, LCP included: WriteFrame (internal/component/l2tp/ppp/frame.go:81-85) writes only the Protocol field and payload, and a grep for 0xFF03 and HDLC across internal/ matches only two comments in internal/component/l2tp/ppp/lcp_options.go, one on desiredLCPOption and one on negotiatePeerOption, each recording that the kernel supplies the framing |
| [`RFC1661-4.6-1`](#rfc1661-4.6-1) Restart timer must be configurable (Section 4.6) | {gap}, no test | armRestartTimer in internal/component/l2tp/ppp/session_run.go uses the fixed defaultRestartTimer of three seconds; StartSession in internal/component/l2tp/ppp/start_session.go carries no restart-timer field and no YANG leaf sets one. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-4.6-2`](#rfc1661-4.6-2) Max-Terminate must be configurable (Section 4.6) | {gap}, no test | applyTransition in internal/component/l2tp/ppp/session_run.go initializes restartCount from the fixed defaultMaxTerminate of two transmissions, with no session or YANG setting for that limit. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-4.6-3`](#rfc1661-4.6-3) Max-Configure must be configurable (Section 4.6) | {gap}, no test | applyTransition in internal/component/l2tp/ppp/session_run.go initializes restartCount from the fixed defaultMaxConfigure of ten transmissions, with no session or YANG setting for that limit. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-4.6-4`](#rfc1661-4.6-4) Max-Failure must be configurable (Section 4.6) | {gap}, no test | ze counts no Configure-Naks sent; sendConfigureNakOrReject (internal/component/l2tp/ppp/session_run.go) picks Nak or Reject from the LCPNakOrReject verdict over NegotiatePeerOptions output on each request, so there is no Max-Failure value to configure and no threshold that converts a Nak into a Reject. Disclosed in docs/features/rfc-status.md |
| [`RFC1661-4.4-1`](#rfc1661-4.4-1) In addition to setting the Restart counter, the implementation MUST set the timeout period to the initial value when Restart timer backoff is used. (Section 4.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze applies no Restart timer backoff. internal/component/l2tp/ppp/session_run.go resets its session Restart timer to the fixed defaultRestartTimer, so the condition "when Restart timer backoff is used" never holds |
| [`RFC1661-5.9-3`](#rfc1661-5.9-3) The Identifier field MUST be changed for each Discard-Request sent. (Section 5.9) | {gap}, no test | Ze sends no Discard-Request, so no Identifier is drawn for one; plan/spec-ppp-discard-request-sender.md |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC1661-2-1`](#rfc1661-2-1)

Protocol field: LSB of least-significant octet must equal 1; LSB of most-significant octet must equal 0; frames violating these rules must be treated as unrecognized Protocol (Section 2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NonCompliantProtocolTreatedUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L203) | unit/verify | revert, verified |
| positive | [`TestRFC1661CompliantProtocolRecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L238) | unit/verify | unproven |

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
| negative | [`TestRFC1661ReplyListsEachOptionTypeOnce`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L748) | unit/verify | unproven |
| negative | [`TestRFC1661ClientInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L159) | unit/verify | unproven |
| positive | [`TestRFC1661InvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L154) | unit/verify | unproven |
| positive | [`TestRFC1661LCPInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L318) | unit/verify | unproven |
| positive | [`TestRFC1661LCPWrongLengthMagicIsNakedNotRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L404) | unit/verify | unproven |
| positive | [`TestRFC1661ReplyListsEachOptionTypeOnce`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L744) | unit/verify | unproven |
| positive | [`TestRFC1661ClientInvalidOptionLengthDrawsNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L153) | unit/verify | unproven |

### [`RFC1661-6-2`](#rfc1661-6-2)

A Configuration Option whose Data is indicated by its Length to extend beyond the end of the Information field must cause the entire packet to be silently discarded without affecting the automaton (Section 6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661LCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L590) | unit/verify | unproven |
| negative | [`TestRFC1661LCPTruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L243) | unit/verify | unproven |
| negative | [`TestRFC1661NCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L680) | unit/verify | unproven |
| negative | [`TestRFC1661TruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L81) | unit/verify | unproven |
| negative | [`TestRFC1661ClientRequestPastEndSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L116) | unit/verify | unproven |
| positive | [`TestRFC1661LCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L584) | unit/verify | unproven |
| positive | [`TestRFC1661LCPTruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L236) | unit/verify | unproven |
| positive | [`TestRFC1661NCPReplyWithOptionsPastEndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L675) | unit/verify | unproven |
| positive | [`TestRFC1661TruncatedOptionSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L75) | unit/verify | unproven |
| positive | [`TestRFC1661ClientRequestPastEndSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L109) | unit/verify | unproven |

### [`RFC1661-3.1-1`](#rfc1661-3.1-1)

In order to establish communications over a point-to-point link, each end of the PPP link MUST first send LCP packets to configure and test the data link. (Section 3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1661LCPPacketsSentFirst`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L307) | unit/verify | unproven |

### [`RFC1661-3.1-2`](#rfc1661-3.1-2)

Then, PPP MUST send NCP packets to choose and configure one or more network-layer protocols. (Section 3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NoNCPPacketsWhenNoNetworkProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L374) | unit/verify | unproven |
| positive | [`TestRFC1661NCPConfiguresEachFamilySeparately`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L335) | unit/verify | unproven |

### [`RFC1661-3.4-1`](#rfc1661-3.4-1)

Any non-LCP packets received during this phase MUST be silently discarded. (Section 3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-3.4-1, so no unit is bound to it.

### [`RFC1661-3.5-1`](#rfc1661-3.5-1)

If an implementation desires that the peer authenticate with some specific authentication protocol, then it MUST request the use of that authentication protocol during Link Establishment phase. (Section 3.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NoAuthProtocolWhenNotDesired`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L477) | unit/verify | unproven |
| positive | [`TestRFC1661AuthProtocolRequestedDuringEstablishment`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L446) | unit/verify | unproven |

### [`RFC1661-3.5-2`](#rfc1661-3.5-2)

An implementation MUST NOT allow the exchange of link quality determination packets to delay authentication indefinitely. (Section 3.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-3.5-2, so no unit is bound to it.

### [`RFC1661-3.5-3`](#rfc1661-3.5-3)

Advancement from the Authentication phase to the Network-Layer Protocol phase MUST NOT occur until authentication has completed. (Section 3.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NetworkPhaseRunsAfterAuthCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L537) | unit/verify | unproven |
| positive | [`TestRFC1661NoNetworkPhaseUntilAuthCompletes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L504) | unit/verify | unproven |

### [`RFC1661-3.5-4`](#rfc1661-3.5-4)

All other packets received during this phase MUST be silently discarded. (Section 3.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-3.5-4, so no unit is bound to it.

### [`RFC1661-3.6-1`](#rfc1661-3.6-1)

Once PPP has finished the previous phases, each network-layer protocol (such as IP, IPX, or AppleTalk) MUST be separately configured by the appropriate Network Control Protocol (NCP). (Section 3.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NCPStatesAreIndependent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L391) | unit/verify | unproven |
| positive | [`TestRFC1661NCPConfiguresEachFamilySeparately`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L339) | unit/verify | unproven |

### [`RFC1661-3.6-2`](#rfc1661-3.6-2)

Any supported network-layer protocol packets received when the corresponding NCP is not in the Opened state MUST be silently discarded. (Section 3.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1661NetworkLayerPacketDiscardedBeforeNCPOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L410) | unit/verify | revert, verified |

### [`RFC1661-3.6-3`](#rfc1661-3.6-3)

While LCP is in the Opened state, any protocol packet which is unsupported by the implementation MUST be returned in a Protocol- Reject (described later). (Section 3.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661UnsupportedProtocolNotRejectedBeforeOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L94) | unit/verify | revert, verified |
| positive | [`TestRFC1661UnsupportedProtocolRejectedInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L46) | unit/verify | revert, verified |

### [`RFC1661-3.7-1`](#rfc1661-3.7-1)

Receiver of Terminate-Request must not disconnect until at least one Restart time after sending Terminate-Ack (Section 3.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1661TerminateAckSentAndLinkHeld`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L569) | unit/verify | revert, verified |

### [`RFC1661-3.7-2`](#rfc1661-3.7-2)

Any non-LCP packets received during this phase MUST be silently discarded. (Section 3.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-3.7-2, so no unit is bound to it.

### [`RFC1661-4.3-1`](#rfc1661-4.3-1)

The implementation MUST be prepared to immediately renegotiate the Configuration Options. (Section 4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661EchoDoesNotRenegotiate`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L662) | unit/verify | unproven |
| positive | [`TestRFC1661RenegotiateOnConfigureRequestInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L629) | unit/verify | unproven |

### [`RFC1661-4.3-2`](#rfc1661-4.3-2)

The implementation MUST be prepared to receive a new Configure-Request without network administrator intervention. (Section 4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1661NewConfigureRequestAcceptedAfterTerminateRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L682) | unit/verify | unproven |

### [`RFC1661-4.3-3`](#rfc1661-4.3-3)

The implementation MUST stop sending the offending packet type. (Section 4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-4.3-3, so no unit is bound to it.

### [`RFC1661-5.1-1`](#rfc1661-5.1-1)

An implementation wishing to open a connection MUST transmit a Configure-Request. (Section 5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661UpWithoutOpenSendsNoConfigureRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L739) | unit/verify | unproven |
| positive | [`TestRFC1661OpenTransmitsConfigureRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L708) | unit/verify | unproven |

### [`RFC1661-5.1-2`](#rfc1661-5.1-2)

Upon reception of a Configure-Request, an appropriate reply MUST be transmitted. (Section 5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L821) | unit/verify | unproven |
| positive | [`TestRFC1661ConfigureAckEchoesOptionsVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L757) | unit/verify | unproven |

### [`RFC1661-5.1-3`](#rfc1661-5.1-3)

The Identifier field MUST be changed whenever the contents of the Options field changes, and whenever a valid reply has been received for a previous request. (Section 5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLCPConfigureRequestIdentifierLifecycle`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L493) | unit/verify | unproven |
| negative | [`TestClientLCPNakChangesIdentifierAndDropsStaleReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L145) | unit/verify | unproven |
| positive | [`TestLCPConfigureRequestIdentifierLifecycle`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L492) | unit/verify | unproven |
| positive | [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L80) | unit/verify | unproven |

### [`RFC1661-5.2-1`](#rfc1661-5.2-1)

If every Configuration Option received in a Configure-Request is recognizable and all values are acceptable, then the implementation MUST transmit a Configure-Ack. (Section 5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L825) | unit/verify | unproven |
| positive | [`TestRFC1661ConfigureAckEchoesOptionsVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L761) | unit/verify | unproven |

### [`RFC1661-5.2-2`](#rfc1661-5.2-2)

Acknowledged Configuration Options must not be reordered or modified (Section 5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661ConfigureAckDoesNotReorderOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L790) | unit/verify | unproven |
| positive | [`TestRFC1661ConfigureAckEchoesOptionsVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L764) | unit/verify | unproven |

### [`RFC1661-5.2-3`](#rfc1661-5.2-3)

On reception of a Configure-Ack, the Identifier field MUST match that of the last transmitted Configure-Request. (Section 5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L400) | unit/verify | unproven |
| negative | [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L19) | unit/verify | unproven |
| positive | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L399) | unit/verify | unproven |
| positive | [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L18) | unit/verify | unproven |

### [`RFC1661-5.2-4`](#rfc1661-5.2-4)

Additionally, the Configuration Options in a Configure-Ack MUST exactly match those of the last transmitted Configure-Request. (Section 5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L402) | unit/verify | unproven |
| negative | [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L21) | unit/verify | unproven |
| positive | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L401) | unit/verify | unproven |
| positive | [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L20) | unit/verify | unproven |

### [`RFC1661-5.3-1`](#rfc1661-5.3-1)

If all options recognized but some values unacceptable, must transmit Configure-Nak (Section 5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NoNakForAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L946) | unit/verify | unproven |
| positive | [`TestRFC1661ConfigureNakSuggestsAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L911) | unit/verify | unproven |

### [`RFC1661-5.3-2`](#rfc1661-5.3-2)

Options which have no value fields (boolean options) MUST use the Configure-Reject reply instead. (Section 5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661ValuedOptionUsesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L998) | unit/verify | unproven |
| positive | [`TestRFC1661BooleanOptionsUseRejectNotNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L976) | unit/verify | unproven |

### [`RFC1661-5.3-3`](#rfc1661-5.3-3)

Each Configuration Option which is allowed only a single instance MUST be modified to a value acceptable to the Configure-Nak sender. (Section 5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NoNakForAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L949) | unit/verify | unproven |
| positive | [`TestRFC1661RepeatedOptionDrawsOneNakEntry`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L345) | unit/verify | unproven |
| positive | [`TestRFC1661ConfigureNakSuggestsAcceptableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L915) | unit/verify | unproven |

### [`RFC1661-5.3-4`](#rfc1661-5.3-4)

When a particular type of Configuration Option can be listed more than once with different values, the Configure-Nak MUST include a list of all values for that option which are acceptable to the Configure-Nak sender. (Section 5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-5.3-4, so no unit is bound to it.

### [`RFC1661-5.3-5`](#rfc1661-5.3-5)

Any value fields for the option MUST indicate values acceptable to the Configure-Nak sender. (Section 5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661RejectedValueStaysUnacceptable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1037) | unit/verify | unproven |
| positive | [`TestRFC1661NakValueIsAcceptable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1015) | unit/verify | unproven |

### [`RFC1661-5.3-6`](#rfc1661-5.3-6)

Options from Configure-Request must not be reordered in Configure-Nak (Section 5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NakOrderFollowsRequestNotAFixedOrder`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1082) | unit/verify | unproven |
| positive | [`TestRFC1661NakPreservesRequestOrder`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1054) | unit/verify | unproven |

### [`RFC1661-5.3-7`](#rfc1661-5.3-7)

On reception of a Configure-Nak, the Identifier field MUST match that of the last transmitted Configure-Request. (Section 5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L404) | unit/verify | unproven |
| negative | [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L22) | unit/verify | unproven |
| positive | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L403) | unit/verify | unproven |
| positive | [`TestClientLCPNakChangesIdentifierAndDropsStaleReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L144) | unit/verify | unproven |

### [`RFC1661-5.3-8`](#rfc1661-5.3-8)

Since the Nak'd Option has been modified by the peer, the implementation MUST be able to handle an Option length which is different from the original Configure-Request. (Section 5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NakTooShortOptionNotDecoded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1125) | unit/verify | unproven |
| positive | [`TestRFC1661NakHandlesDifferentOptionLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1103) | unit/verify | unproven |

### [`RFC1661-5.4-1`](#rfc1661-5.4-1)

If some Configuration Options received in a Configure-Request are not recognizable or are not acceptable for negotiation (as configured by a network administrator), then the implementation MUST transmit a Configure-Reject. (Section 5.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L531) | unit/verify | unproven |
| negative | [`TestRFC1661NoConfigureRejectWhenAllOptionsRecognized`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L859) | unit/verify | unproven |
| negative | [`TestRFC1661ClientAcceptsServerAuthProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L359) | unit/verify | unproven |
| negative | [`TestRFC1661ClientUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L207) | unit/verify | unproven |
| positive | [`TestRFC1661LCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L459) | unit/verify | unproven |
| positive | [`TestRFC1661NCPUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L526) | unit/verify | unproven |
| positive | [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L828) | unit/verify | unproven |
| positive | [`TestRFC1661ClientUnrecognizedTypeOutranksInvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L202) | unit/verify | unproven |

### [`RFC1661-5.4-2`](#rfc1661-5.4-2)

Configure-Reject options must not be reordered or modified (Section 5.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661LCPRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L839) | unit/verify | unproven |
| negative | [`TestRFC1661ConfigureRejectDoesNotReorderOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L877) | unit/verify | unproven |
| negative | [`TestRFC1661ClientRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L250) | unit/verify | unproven |
| positive | [`TestRFC1661LCPRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_option_length_test.go#L828) | unit/verify | unproven |
| positive | [`TestRFC1661ConfigureRejectForUnrecognizedOption`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L831) | unit/verify | unproven |
| positive | [`TestRFC1661ClientRejectEchoesTheRefusedOptionUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L238) | unit/verify | unproven |

### [`RFC1661-5.4-3`](#rfc1661-5.4-3)

On reception of a Configure-Reject, the Identifier field MUST match that of the last transmitted Configure-Request. (Section 5.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L406) | unit/verify | unproven |
| negative | [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L23) | unit/verify | unproven |
| positive | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L405) | unit/verify | unproven |
| positive | [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L76) | unit/verify | unproven |

### [`RFC1661-5.4-4`](#rfc1661-5.4-4)

Additionally, the Configuration Options in a Configure-Reject MUST be a proper subset of those in the last transmitted Configure- Request. (Section 5.4, Errata 543)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L408) | unit/verify | unproven |
| negative | [`TestClientLCPReplyCorrelation`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L24) | unit/verify | unproven |
| positive | [`TestLCPRepliesMatchOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L407) | unit/verify | unproven |
| positive | [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L77) | unit/verify | unproven |

### [`RFC1661-5.4-5`](#rfc1661-5.4-5)

Next Configure-Request must not include any rejected options (Section 5.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLCPRejectedOptionsStayRemoved`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L534) | unit/verify | unproven |
| negative | [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L79) | unit/verify | unproven |
| positive | [`TestLCPRejectedOptionsStayRemoved`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_test.go#L533) | unit/verify | unproven |
| positive | [`TestClientLCPRejectRemovesOnlyRejectedOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L78) | unit/verify | unproven |

### [`RFC1661-5.5-1`](#rfc1661-5.5-1)

Upon reception of a Terminate-Request, a Terminate-Ack MUST be transmitted. (Section 5.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NoTerminateAckForTerminateAck`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L608) | unit/verify | unproven |
| positive | [`TestRFC1661TerminateAckSentAndLinkHeld`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L574) | unit/verify | revert, verified |

### [`RFC1661-5.6-1`](#rfc1661-5.6-1)

This MUST be reported back to the sender of the unknown Code by transmitting a Code- Reject. (Section 5.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NoCodeRejectForKnownCode`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1179) | unit/verify | unproven |
| positive | [`TestRFC1661CodeRejectForUnknownCode`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1147) | unit/verify | unproven |

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

Audit verdict: not audited: no reader has judged these tests

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

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1661ProtocolRejectIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L137) | unit/verify | revert, verified |

### [`RFC1661-5.7-4`](#rfc1661-5.7-4)

The Rejected-Information MUST be truncated to comply with the peer's established MRU. (Section 5.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661ProtocolRejectInformationKeptWhenItFits`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L201) | unit/verify | revert, verified |
| positive | [`TestRFC1661ProtocolRejectInformationTruncatedToMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_protocol_reject_test.go#L166) | unit/verify | revert, verified |

### [`RFC1661-5.8-1`](#rfc1661-5.8-1)

Upon reception of an Echo-Request in the LCP Opened state, an Echo-Reply MUST be transmitted. (Section 5.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661NoEchoOutsideOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1242) | unit/verify | unproven |
| positive | [`TestRFC1661EchoReplyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1200) | unit/verify | unproven |

### [`RFC1661-5.8-2`](#rfc1661-5.8-2)

Echo-Request and Echo-Reply packets MUST only be sent in the LCP Opened state. (Section 5.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661EchoReplyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1203) | unit/verify | unproven |
| negative | [`TestClientLCPEchoBeforeOpenDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_transition_test.go#L115) | unit/verify | unproven |
| positive | [`TestRFC1661NoEchoOutsideOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1246) | unit/verify | unproven |

### [`RFC1661-5.9-1`](#rfc1661-5.9-1)

Discard-Request packets MUST only be sent in the LCP Opened state. (Section 5.9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-5.9-1, so no unit is bound to it.

### [`RFC1661-5.9-2`](#rfc1661-5.9-2)

On reception, the receiver MUST silently discard any Discard- Request that it receives. (Section 5.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1661DiscardRequestSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1268) | unit/verify | unproven |

### [`RFC1661-6.2-1`](#rfc1661-6.2-1)

An implementation MUST NOT include multiple Authentication- Protocol Configuration Options in its Configure-Request packets. (Section 6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1661SingleAuthProtocolOptionInRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1300) | unit/verify | unproven |

### [`RFC1661-6.4-1`](#rfc1661-6.4-1)

If an implementation does transmit a Configure-Request with a Magic-Number Configuration Option, then it MUST NOT respond with a Configure-Reject when it receives a Configure-Request with a Magic-Number Configuration Option. (Section 6.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661UnknownOptionRejectedWhileMagicIsNot`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1350) | unit/verify | unproven |
| positive | [`TestRFC1661ZeroMagicNumberRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1378) | unit/verify | unproven |

### [`RFC1661-6.4-2`](#rfc1661-6.4-2)

If Magic-Number has been successfully negotiated, an implementation MUST transmit these packets with the Magic-Number field set to its negotiated Magic-Number. (Section 6.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661EchoReplyDoesNotMirrorPeerMagic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1417) | unit/verify | unproven |
| positive | [`TestRFC1661EchoReplyInOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1213) | unit/verify | unproven |

### [`RFC1661-6.4-3`](#rfc1661-6.4-3)

A Magic-Number of zero is illegal and MUST always be Nak'd, if it is not Rejected outright. (Section 6.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1661PeerMagicNumberAcked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1331) | unit/verify | unproven |
| negative | [`TestRFC1661ClientAcksANonZeroMagic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_client_negotiation_test.go#L82) | unit/verify | unproven |
| negative | [`TestRFC1661ClientNaksZeroMagicWithAValueOfItsOwn`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L413) | unit/verify | unproven |
| positive | [`TestRFC1661ZeroMagicNumberRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L1370) | unit/verify | unproven |
| positive | [`TestRFC1661ClientNaksZeroMagicInAWellFormedRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_client_negotiation_test.go#L44) | unit/verify | unproven |
| positive | [`TestRFC1661ClientNaksZeroMagicWithAValueOfItsOwn`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_option_length_test.go#L407) | unit/verify | unproven |

### [`RFC1661-6.5-1`](#rfc1661-6.5-1)

By default, all implementations MUST transmit packets with two octet PPP Protocol fields. (Section 6.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1661TwoOctetProtocolField`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L259) | unit/verify | unproven |

### [`RFC1661-6.5-2`](#rfc1661-6.5-2)

Compressed Protocol fields MUST NOT be transmitted unless this Configuration Option has been negotiated. (Section 6.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1661TwoOctetProtocolField`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_test.go#L263) | unit/verify | unproven |

### [`RFC1661-6.5-3`](#rfc1661-6.5-3)

When PFC negotiated, must accept both single-octet and double-octet Protocol fields (Section 6.5)

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

In addition to setting the Restart counter, the implementation MUST set the timeout period to the initial value when Restart timer backoff is used. (Section 4.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-4.4-1, so no unit is bound to it.

### [`RFC1661-4.4-2`](#rfc1661-4.4-2)

In addition to zeroing the Restart counter, the implementation MUST set the timeout period to an appropriate value. (Section 4.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLCPPeerTerminateRestartTimer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_restart_counter_test.go#L136) | unit/verify | unproven |
| positive | [`TestLCPPeerTerminateRestartTimer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_restart_counter_test.go#L135) | unit/verify | unproven |

### [`RFC1661-5.5-2`](#rfc1661-5.5-2)

On transmission of a Terminate-Request or Terminate-Ack, "the Identifier field MUST be changed whenever the content of the Data field changes, and whenever a valid reply has been received for a previous request" (Section 5.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTerminateRequestIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_echo_rfc1661_test.go#L41) | unit/verify | revert, verified |
| positive | [`TestTerminateRequestIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_echo_rfc1661_test.go#L40) | unit/verify | revert, verified |

### [`RFC1661-5.8-4`](#rfc1661-5.8-4)

On transmission of an Echo-Request or Echo-Reply, "the Identifier field MUST be changed whenever the content of the Data field changes, and whenever a valid reply has been received for a previous request" (Section 5.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEchoRequestIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_echo_rfc1661_test.go#L71) | unit/verify | revert, verified |
| positive | [`TestEchoRequestIdentifierChanges`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_echo_rfc1661_test.go#L70) | unit/verify | revert, verified |

### [`RFC1661-5.8-5`](#rfc1661-5.8-5)

In Echo-Request, Echo-Reply and Discard-Request packets, "Until the Magic-Number Configuration Option has been successfully negotiated, the Magic-Number MUST be transmitted as zero" (Section 5.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMagicNumberZeroUntilNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_echo_rfc1661_test.go#L111) | unit/verify | revert, verified |
| positive | [`TestMagicNumberZeroUntilNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_echo_rfc1661_test.go#L110) | unit/verify | revert, verified |

### [`RFC1661-5.9-3`](#rfc1661-5.9-3)

The Identifier field MUST be changed for each Discard-Request sent. (Section 5.9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1661-5.9-3, so no unit is bound to it.

### [`RFC1661-6.1-1`](#rfc1661-6.1-1)

If smaller packets are requested, an implementation MUST still be able to receive the full 1500 octet information field in case link synchronization is lost. (Section 6.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFullInformationFieldReceived`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_echo_rfc1661_test.go#L259) | unit/verify | revert, verified |
| positive | [`TestFullInformationFieldReceived`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_echo_rfc1661_test.go#L258) | unit/verify | revert, verified |

### [`RFC1661-6.4-5`](#rfc1661-6.4-5)

Before this Configuration Option is requested, an implementation MUST choose its Magic-Number. (Section 6.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMagicNumberChosenBeforeRequested`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_choice_rfc1661_test.go#L21) | unit/verify | revert, verified |
| positive | [`TestMagicNumberChosenBeforeRequested`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_choice_rfc1661_test.go#L20) | unit/verify | revert, verified |

### [`RFC1661-6.4-6`](#rfc1661-6.4-6)

When a Configure-Request carries a Magic-Number equal to the one last sent to the peer, "a Configure-Nak MUST be sent specifying a different Magic-Number value" (Section 6.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEqualMagicNumberIsNaked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_choice_rfc1661_test.go#L67) | unit/verify | revert, verified |
| positive | [`TestEqualMagicNumberIsNaked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_choice_rfc1661_test.go#L66) | unit/verify | revert, verified |

### [`RFC1661-6.4-7`](#rfc1661-6.4-7)

If the Magic-Number is equal to the one sent in the last Configure-Nak, the possibility of a looped-back link is increased, and a new Magic-Number MUST be chosen. (Section 6.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMagicNumberRedrawnOnNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_echo_rfc1661_test.go#L169) | unit/verify | revert, verified |
| negative | [`TestClientLCPMagicNakRedrawsAndUsesNegotiatedValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L186) | unit/verify | unproven |
| positive | [`TestMagicNumberRedrawnOnNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_echo_rfc1661_test.go#L168) | unit/verify | revert, verified |
| positive | [`TestClientLCPMagicNakRedrawsAndUsesNegotiatedValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/lcp_reply_test.go#L185) | unit/verify | unproven |

### [`RFC1661-6.4-8`](#rfc1661-6.4-8)

All received Magic-Number fields MUST be equal to either zero or the peer's unique Magic-Number, depending on whether or not the peer negotiated a Magic-Number. (Section 6.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReceivedMagicNumberMustBePeers`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_echo_rfc1661_test.go#L207) | unit/verify | revert, verified |
| positive | [`TestReceivedMagicNumberMustBePeers`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/magic_echo_rfc1661_test.go#L206) | unit/verify | revert, verified |

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
