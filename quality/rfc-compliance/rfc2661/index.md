# RFC 2661 - Layer Two Tunneling Protocol "L2TP"

Partial. Every requirement this repository extracted from RFC 2661, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 68.0% | 66 of 97 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 21.6% | 21 of 97 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 97 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 76.6% | 128 of 167 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 97 | of 105 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 97 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 97 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 1.0% | 1 of 97 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 97 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 9.3% | 9 of 97 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 97 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 105 |
| Gated MUST-level | 97 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 9 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 167 |
| Tagged units | 167 |
| Recorded audit verdicts | 0 |
| Discrimination records | 128 |
| Summary | `rfc/short/rfc2661.md` |
| Requirement shard | `rfc/requirements/rfc2661.md` |
| RFC text | `rfc/full/rfc2661.txt` |

## Enrolment

Enrolled: Layer Two Tunneling Protocol / L2TP (RFC 2661): Ze implements the LAC/LNS control plane. The checklist records its requirement inventory; current execution and producer discrimination determine verified coverage, not a fixed count in this paragraph. Hidden-AVP integration remains a gap. Proxy authentication and new-AVP configure-off behavior remain conditional backlog, not features selected by the authorization to finish already-started work.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- LNS/LAC tunnel lifecycle (answerer and **initiator**: ze dials SCCRQ, verifies SCCRP, sends SCCCN), AVP codec, hidden-AVP MD5 codec (present but not wired into message encode/decode), challenge/response, reliable control channel, HELLO, StopCCN, data sessions, **LNS-side outgoing call (OCRQ/OCRP/OCCN) via `request l2tp outgoing-call`**, dial-target config, LAC PPPoE→L2TP relay (control plane). <!-- source: [`internal/component/l2tp/tunnel_initiator.go`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_initiator.go) -- initiate/handleSCCRP
- [`internal/component/l2tp/session_initiator.go`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_initiator.go) -- placeOutgoingCall/handleOCRP -->


**What the ledger says remains**

Partial. Nine MUST rows carry {gap}. The hidden-AVP MD5 codec is not wired into message encoding or decoding, so Random Vector precedence is not enforced. The existing proxy-authentication and new-AVP specs describe conditional features that have not been selected or declined. Control-plane tests exercise tunnel/session establishment, mandatory AVPs, header fields and unknown mandatory Message Types. Linux-boundary tests check session-setup timing, sequencing/LNS-mode flags and UDP checksum settings; they do not by themselves demonstrate kernel sequence adaptation or PPP data-plane forwarding. LNS/LAC interoperability, the LAC bridge and kernel behavior still need the corresponding privileged interop/QEMU runs before this batch is reported verified.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 66 | one part of the gated population |
| Annotated instead of tested | 31 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 1 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **97** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (66):** [`RFC2661-4.1-1`](#rfc2661-4.1-1), [`RFC2661-4.1-2`](#rfc2661-4.1-2), [`RFC2661-4.1-3`](#rfc2661-4.1-3), [`RFC2661-4.1-4`](#rfc2661-4.1-4), [`RFC2661-5.8-3`](#rfc2661-5.8-3), [`RFC2661-5.8-4`](#rfc2661-5.8-4), [`RFC2661-5.8-5`](#rfc2661-5.8-5), [`RFC2661-5.8-6`](#rfc2661-5.8-6), [`RFC2661-5.8-7`](#rfc2661-5.8-7), [`RFC2661-6.1-1`](#rfc2661-6.1-1), [`RFC2661-6.2-1`](#rfc2661-6.2-1), [`RFC2661-24.10-1`](#rfc2661-24.10-1), [`RFC2661-24.12-1`](#rfc2661-24.12-1), [`RFC2661-10-1`](#rfc2661-10-1), [`RFC2661-9-1`](#rfc2661-9-1), [`RFC2661-10-2`](#rfc2661-10-2), [`RFC2661-3.1-2`](#rfc2661-3.1-2), [`RFC2661-3.1-5`](#rfc2661-3.1-5), [`RFC2661-3.1-6`](#rfc2661-3.1-6), [`RFC2661-4.1-5`](#rfc2661-4.1-5), [`RFC2661-4.4.1-1`](#rfc2661-4.4.1-1), [`RFC2661-4.4.3-1`](#rfc2661-4.4.3-1), [`RFC2661-4.4.3-3`](#rfc2661-4.4.3-3), [`RFC2661-4.4.3-4`](#rfc2661-4.4.3-4), [`RFC2661-4.4.3-5`](#rfc2661-4.4.3-5), [`RFC2661-4.4.3-6`](#rfc2661-4.4.3-6), [`RFC2661-4.4.3-7`](#rfc2661-4.4.3-7), [`RFC2661-4.4.3-8`](#rfc2661-4.4.3-8), [`RFC2661-4.4.3-9`](#rfc2661-4.4.3-9), [`RFC2661-4.4.4-1`](#rfc2661-4.4.4-1), [`RFC2661-4.4.4-2`](#rfc2661-4.4.4-2), [`RFC2661-4.4.4-3`](#rfc2661-4.4.4-3), [`RFC2661-4.4.4-4`](#rfc2661-4.4.4-4), [`RFC2661-4.4.4-5`](#rfc2661-4.4.4-5), [`RFC2661-5.0-1`](#rfc2661-5.0-1), [`RFC2661-5.0-2`](#rfc2661-5.0-2), [`RFC2661-5.1.1-1`](#rfc2661-5.1.1-1), [`RFC2661-5.1.1-2`](#rfc2661-5.1.1-2), [`RFC2661-5.1.1-3`](#rfc2661-5.1.1-3), [`RFC2661-5.3-1`](#rfc2661-5.3-1), [`RFC2661-5.3-2`](#rfc2661-5.3-2), [`RFC2661-5.4-1`](#rfc2661-5.4-1), [`RFC2661-5.4-2`](#rfc2661-5.4-2), [`RFC2661-5.4-3`](#rfc2661-5.4-3), [`RFC2661-5.7-1`](#rfc2661-5.7-1), [`RFC2661-5.8-10`](#rfc2661-5.8-10), [`RFC2661-6.3-1`](#rfc2661-6.3-1), [`RFC2661-6.4-1`](#rfc2661-6.4-1), [`RFC2661-6.6-1`](#rfc2661-6.6-1), [`RFC2661-6.7-1`](#rfc2661-6.7-1), [`RFC2661-6.8-1`](#rfc2661-6.8-1), [`RFC2661-6.9-1`](#rfc2661-6.9-1), [`RFC2661-6.9-2`](#rfc2661-6.9-2), [`RFC2661-6.10-1`](#rfc2661-6.10-1), [`RFC2661-6.11-1`](#rfc2661-6.11-1), [`RFC2661-6.12-1`](#rfc2661-6.12-1), [`RFC2661-6.12-2`](#rfc2661-6.12-2), [`RFC2661-6.13-1`](#rfc2661-6.13-1), [`RFC2661-6.14-1`](#rfc2661-6.14-1), [`RFC2661-6.14-2`](#rfc2661-6.14-2), [`RFC2661-7.2.1-1`](#rfc2661-7.2.1-1), [`RFC2661-7.2.1-2`](#rfc2661-7.2.1-2), [`RFC2661-7.2.1-3`](#rfc2661-7.2.1-3), [`RFC2661-7.5-1`](#rfc2661-7.5-1), [`RFC2661-7.5.1-1`](#rfc2661-7.5.1-1), [`RFC2661-8.1-3`](#rfc2661-8.1-3)

**Annotated instead of tested (31):** [`RFC2661-x-1`](#rfc2661-x-1), [`RFC2661-5.8-1`](#rfc2661-5.8-1), [`RFC2661-5.8-2`](#rfc2661-5.8-2), [`RFC2661-4.3-1`](#rfc2661-4.3-1), [`RFC2661-3.1-1`](#rfc2661-3.1-1), [`RFC2661-3.1-3`](#rfc2661-3.1-3), [`RFC2661-3.1-4`](#rfc2661-3.1-4), [`RFC2661-3.1-7`](#rfc2661-3.1-7), [`RFC2661-4.2-2`](#rfc2661-4.2-2), [`RFC2661-4.3-3`](#rfc2661-4.3-3), [`RFC2661-4.4.1-2`](#rfc2661-4.4.1-2), [`RFC2661-4.4-3`](#rfc2661-4.4-3), [`RFC2661-4.4-1`](#rfc2661-4.4-1), [`RFC2661-4.4.2-1`](#rfc2661-4.4.2-1), [`RFC2661-4.4.3-2`](#rfc2661-4.4.3-2), [`RFC2661-4.4-2`](#rfc2661-4.4-2), [`RFC2661-4.4.5-1`](#rfc2661-4.4.5-1), [`RFC2661-4.4.5-2`](#rfc2661-4.4.5-2), [`RFC2661-4.4.5-3`](#rfc2661-4.4.5-3), [`RFC2661-4.4.5-4`](#rfc2661-4.4.5-4), [`RFC2661-4.4.5-5`](#rfc2661-4.4.5-5), [`RFC2661-4.4.5-6`](#rfc2661-4.4.5-6), [`RFC2661-4.4.6-1`](#rfc2661-4.4.6-1), [`RFC2661-6.5-1`](#rfc2661-6.5-1), [`RFC2661-6.5-2`](#rfc2661-6.5-2), [`RFC2661-6.5-3`](#rfc2661-6.5-3), [`RFC2661-7.5.1-2`](#rfc2661-7.5.1-2), [`RFC2661-8.1-1`](#rfc2661-8.1-1), [`RFC2661-8.1-2`](#rfc2661-8.1-2), [`RFC2661-8.2-1`](#rfc2661-8.2-1), [`RFC2661-9.5-2`](#rfc2661-9.5-2)

**Derived from other rows (1):** [`RFC2661-6.0-1`](#rfc2661-6.0-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2661-x-1` | Reserved bits 8-11 in L2TP header MUST be 0 (Wire Format) | MUST | x | **positive:** `unit/verify` [`TestWriteControlHeader`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/header_test.go#L230). **negative:** no negative test. **positive:** `functional/verify` [`rfc2661-emitted-control-shape.ci`](https://github.com/ze-software/ze/blob/main/test/l2tp/rfc2661-emitted-control-shape.ci#L27). **{single-polarity}:** every control header is the fixed constant 0xC802 and every data header is built from verL2TP\|flags, so reserved bits are always emitted zero, and ze never rejects non-zero on receive (internal/component/l2tp/header.go:26, :148-155, :172-207) |
| `RFC2661-4.1-1` | AVP reserved bits 2-5 MUST be zero on send; non-zero on receive means treat AVP as unrecognized (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestAVPCatalogRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/avp_test.go#L180). **negative:** `unit/verify` [`TestAVPIteratorReservedBits`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/avp_test.go#L107). **positive:** `functional/verify` [`rfc2661-emitted-control-shape.ci`](https://github.com/ze-software/ze/blob/main/test/l2tp/rfc2661-emitted-control-shape.ci#L29) |
| `RFC2661-4.1-2` | Message Type AVP (type 0) MUST be the first AVP in every control message. RFC 2661 Section 4.4.1: "The Message Type AVP MUST be the first AVP in a message, immediately following the control message header". The id anchor below is frozen and does NOT name Section 4.1, which is AVP Format (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestWriteICRPBody`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L945). **negative:** `unit/verify` [`TestReactor_MalformedSCCRQCreatesNoTunnel`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reactor_test.go#L256). **positive:** `functional/verify` [`rfc2661-emitted-control-shape.ci`](https://github.com/ze-software/ze/blob/main/test/l2tp/rfc2661-emitted-control-shape.ci#L24) |
| `RFC2661-4.1-3` | If M=1 and AVP is unrecognized and session-scoped, send CDN and tear down session (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestSession_IncomingLNS_ICRQ`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L126). **negative:** `unit/verify` [`TestSession_UnknownMandatoryAVP`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L559) |
| `RFC2661-4.1-4` | If M=1 and AVP is unrecognized and tunnel-scoped, send StopCCN and tear down tunnel (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestTunnelInitiatorHandshake`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_initiator_test.go#L134). **negative:** `unit/verify` [`TestTunnelSCCCNUnknownMandatoryAVP_StopCCN`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_initiator_test.go#L253) |
| `RFC2661-5.8-1` | Each subsequent retransmission of a message MUST employ an exponential backoff interval. (§5.8) | MUST | 5.8 | **positive:** `unit/verify` [`TestTickBackoffSchedule`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_test.go#L374). **negative:** no negative test. **{single-polarity}:** the engine doubles the retransmit timeout on each expiry, and exponential growth is a positive behavior with no meaningful negation on a correct implementation (internal/component/l2tp/reliable.go:591-595) |
| `RFC2661-5.8-2` | This cap MUST be no less than 8 seconds per retransmission. (§5.8) | MUST | 5.8 | **positive:** `unit/verify` [`TestBackoffCapAtLeast8Seconds`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_seq_test.go#L94). **negative:** no negative test. **{single-polarity}:** the default backoff cap is the 16s constant and the reactor always constructs engines with RTimeoutCap unset, so the cap is always at least 8s with no config path or floor guard to test negatively (internal/component/l2tp/reliable_seq.go:19, reliable.go:291-292) |
| `RFC2661-5.8-3` | After exhausting retransmissions without response, tunnel and all sessions MUST be cleared (§5.8) | MUST | 5.8 | **positive:** `unit/verify` [`TestPeerTeardownWithdrawsSubscriberRoute`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reactor_test.go#L1204). **positive:** `unit/verify` [`TestTickMaxAttempts`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_test.go#L414). **negative:** `unit/verify` [`TestTickMaxAttempts`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_test.go#L417) |
| `RFC2661-5.8-4` | The retransmitted message contains the same Ns value, but the Nr value MUST be updated with the sequence number of the next expected message. (§5.8) | MUST | 5.8 | **positive:** `unit/verify` [`TestTickRetransmit`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_test.go#L339). **negative:** `unit/verify` [`TestTickRetransmit`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_test.go#L342) |
| `RFC2661-5.8-5` | Duplicate control messages MUST be acknowledged (via ZLB or piggyback) even though not processed by upper layer (§5.8) | MUST | 5.8 | **positive:** `unit/verify` [`TestOnReceiveDuplicate`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_test.go#L146). **negative:** `unit/verify` [`TestOnReceiveDuplicate`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_test.go#L148) |
| `RFC2661-5.8-6` | An implementation may support a receive window of only 1 (i.e., by sending out a Receive Window Size AVP with a value of 1), but MUST accept a window of up to 4 from its peer (e.g. have the ability to send 4 messages before backing off). (§5.8) | MUST | 5.8 | **positive:** `unit/verify` [`TestWindowAvailable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_window_test.go#L183). **negative:** `unit/verify` [`TestWindowPeerRWSZero`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_window_test.go#L112) |
| `RFC2661-5.8-7` | When a tunnel is being shut down for reasons other than loss of connectivity, the state and reliable delivery mechanisms MUST be maintained and operated for the full retransmission interval after the final message exchange has occurred. (§5.8) | MUST | 5.8 | **positive:** `unit/verify` [`TestExpired`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_test.go#L560). **positive:** `unit/verify` [`TestPostTeardownAckRetention`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_integration_test.go#L191). **negative:** `unit/verify` [`TestExpired`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_test.go#L563). **negative:** `unit/verify` [`TestPostTeardownAckRetention`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_integration_test.go#L195) |
| `RFC2661-4.3-1` | A Random Vector AVP (type 36) MUST precede any hidden AVP (H=1) in the same message (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the hidden-AVP MD5 cipher is implemented and unit-tested but is not wired into any control-message path -- no encoder sets H=1 or emits a Random Vector, and decoders skip hidden AVPs without decrypting or checking precedence (internal/component/l2tp/hidden.go:38 has no production caller; avp.go:156-168 skips hidden AVPs; AVPRandomVector avp.go:56 never emitted) |
| `RFC2661-6.1-1` | Every AVP RFC 2661 Section 6.1 makes mandatory in an SCCRQ is required, and an SCCRQ missing one is answered with StopCCN rather than dropped in silence. RFC 2661 Section 6.1: "The following AVPs MUST be present in the SCCRQ: Message Type AVP, Protocol Version, Host Name, Framing Capabilities, Assigned Tunnel ID"; RFC 2661 Section 7.1: "Examples of a malformed control message include ... a message that is missing a required AVP", and receipt of one "should be logged appropriately and the control connection cleared to ensure recovery to a known state" (§6.1) | MUST | 6.1 | **positive:** `unit/verify` [`TestSCCRQWithEveryMandatoryAVPEstablishes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reactor_sccrq_mandatory_avp_test.go#L108). **negative:** `unit/verify` [`TestSCCRQMissingMandatoryAVPIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reactor_sccrq_mandatory_avp_test.go#L45). **negative:** `unit/verify` [`TestSCCRQWithShortFramingCapabilitiesIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reactor_sccrq_mandatory_avp_test.go#L149). **positive:** `functional/verify` [`rfc2661-sccrq-mandatory-avp.ci`](https://github.com/ze-software/ze/blob/main/test/l2tp/rfc2661-sccrq-mandatory-avp.ci#L28). **negative:** `functional/verify` [`rfc2661-sccrq-mandatory-avp.ci`](https://github.com/ze-software/ze/blob/main/test/l2tp/rfc2661-sccrq-mandatory-avp.ci#L24) |
| `RFC2661-6.2-1` | Every AVP RFC 2661 Section 6.2 makes mandatory in an SCCRP is required, and an SCCRP missing one tears the dialed tunnel down with StopCCN rather than establishing it. RFC 2661 Section 6.2: "The following AVPs MUST be present in the SCCRP: Message Type, Protocol Version, Framing Capabilities, Host Name, Assigned Tunnel ID"; RFC 2661 Section 7.2.1 gives wait-ctl-reply the row "Receive SCCRP, not acceptable \| Send StopCCN, Clean up \| idle" (§6.2) | MUST | 6.2 | **positive:** `unit/verify` [`TestSCCRPWithEveryMandatoryAVPEstablishes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_initiator_sccrp_mandatory_avp_test.go#L108). **negative:** `unit/verify` [`TestSCCRPMissingMandatoryAVPTearsTheTunnelDown`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_initiator_sccrp_mandatory_avp_test.go#L58) |
| `RFC2661-24.10-1` | Assigned Tunnel ID of 0 in SCCRQ/SCCRP is a protocol error; reject with StopCCN. RFC 2661 Section 4.4.3: "The Assigned Tunnel ID is a 2 octet non-zero unsigned integer"; RFC 2661 Section 5.3: the value 0 "MUST NOT be used as an Assigned Session ID or Assigned Tunnel ID". The id anchor below numbers no section of RFC 2661 and is frozen (§24.10) | MUST | 24.10 | **positive:** `unit/verify` [`TestSCCRQWithNonZeroAssignedTunnelIDEstablishes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reactor_sccrq_zero_tid_test.go#L117). **positive:** `unit/verify` [`TestTunnelInitiatorHandshake`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_initiator_test.go#L137). **negative:** `unit/verify` [`TestParseSCCRP_Rejects`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_initiator_test.go#L109). **negative:** `unit/verify` [`TestSCCRQWithZeroAssignedTunnelIDIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reactor_sccrq_zero_tid_test.go#L75). **positive:** `functional/verify` [`rfc2661-sccrq-tunnel-id-zero.ci`](https://github.com/ze-software/ze/blob/main/test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci#L24). **negative:** `functional/verify` [`rfc2661-sccrq-tunnel-id-zero.ci`](https://github.com/ze-software/ze/blob/main/test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci#L21) |
| `RFC2661-24.12-1` | Unknown M=1 vendor AVP in a session context tears down the session with CDN, not the tunnel with StopCCN. RFC 2661 Section 4.1 states the session/tunnel split and RFC 2661 Section 4.2 states the consequence. The id anchor below numbers no section of RFC 2661 and is frozen (§24.12) | MUST | 24.12 | **positive:** `unit/verify` [`TestSession_UnknownMandatoryAVP`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L561). **negative:** `unit/verify` [`TestSession_UnknownMandatoryAVP`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L563) |
| `RFC2661-10-1` | CDN is valid in any non-idle session state; receiving CDN destroys the session. RFC 2661 Section 5.6 states session teardown by CDN and RFC 2661 Section 7.4.2 gives the state table. The id anchor below is frozen and does NOT name Section 10, which is IANA Considerations (§10) | MUST | 10 | **positive:** `unit/verify` [`TestSession_CDN_AnyState`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L342). **positive:** `unit/verify` [`TestSession_CDN_EstablishedSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L313). **negative:** `unit/verify` [`TestSession_CDN_UnknownSessionDropped`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L366) |
| `RFC2661-9-1` | StopCCN cascades: all sessions in a tunnel are cleared when StopCCN is received. RFC 2661 Section 5.7: an implementation "may shut down an entire tunnel and all sessions on the tunnel by sending the StopCCN". The id anchor below is frozen and does NOT name Section 9, which is Security Considerations (§9) | MUST | 9 | **positive:** `unit/verify` [`TestSession_StopCCN_CascadeSessions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L403). **negative:** `unit/verify` [`TestStopCCNQueuesAllTeardowns`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L814) |
| `RFC2661-10-2` | Session ID 0 is reserved and never assigned. RFC 2661 Section 5.3: "The value of 0 for Session ID and Tunnel ID is special and MUST NOT be used as an Assigned Session ID or Assigned Tunnel ID". The id anchor below is frozen and does NOT name Section 10, which is IANA Considerations (§10) | MUST | 10 | **positive:** `unit/verify` [`TestSession_SIDBoundary_MaxUint16`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L1041). **negative:** `unit/verify` [`TestSession_SIDBoundary_Zero`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L1055) |
| `RFC2661-3.1-1` | This bit MUST be set to 1 for control messages (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC2661HeaderFlagWord`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L19). **negative:** no negative test. **{single-polarity}:** a received word with T=0 is a data message by definition (header.go::ParseMessageHeader sets IsControl false and reactor.go::handle drops it), so no violating control input exists |
| `RFC2661-3.1-2` | The S bit MUST be set to 1 for control messages (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC2661HeaderFlagWord`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L21). **negative:** `unit/verify` [`TestRFC2661ControlHeaderWithoutSequenceRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L77) |
| `RFC2661-3.1-3` | The O bit MUST be set to 0 (zero) for control messages (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC2661HeaderFlagWord`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L23). **negative:** no negative test. **{single-polarity}:** the RFC states a sender obligation and no receiver refusal, so the only assertable behavior is the O=0 header.go::WriteControlHeader stamps |
| `RFC2661-3.1-4` | The P bit MUST be set to 0 for all control messages (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC2661HeaderFlagWord`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L25). **negative:** no negative test. **{single-polarity}:** the RFC states a sender obligation and no receiver refusal, so the only assertable behavior is the P=0 header.go::WriteControlHeader stamps |
| `RFC2661-3.1-5` | Ver MUST be 2, indicating the version of the L2TP data message header described in this document (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC2661HeaderFlagWord`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L27). **negative:** `unit/verify` [`TestRFC2661UnknownVersionRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L95) |
| `RFC2661-3.1-6` | Packets received with an unknown Ver field MUST be discarded (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC2661UnknownVersionRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L97). **negative:** `unit/verify` [`TestRFC2661KnownVersionAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L114) |
| `RFC2661-3.1-7` | In data messages, Nr is reserved and, if present (as indicated by the S-bit), MUST be ignored upon receipt (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux l2tp_ppp; internal/component/l2tp/kernel_linux.go::pppSetupReal creates the pppol2tp session the data plane runs on and installs no value the module reads to ignore Nr; reactor.go::handle drops every data message before reading Ns or Nr, so no value Ze writes decides this field |
| `RFC2661-4.1-5` | If the M bit is not set, an unrecognized AVP MUST be ignored (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC2661UnrecognizedOptionalAVPIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L637). **negative:** `unit/verify` [`TestRFC2661UnrecognizedOptionalAVPIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L641) |
| `RFC2661-4.2-2` | Use of the M-bit with new AVPs (those not defined in this document) MUST provide the ability to configure the associated feature off, such that the AVP is either not sent, or sent with the M-bit not set (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze defines no AVP outside this document, so no feature exists to configure off; plan/spec-l2tp-new-avp-definitions.md |
| `RFC2661-4.3-3` | The H bit MUST only be set if a shared secret exists between the LAC and LNS (§4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC2661WrittenAVPFlags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L211). **negative:** no negative test. **{single-polarity}:** no writer sets H=1 until hiding is wired (RFC2661-4.3-1), so there is no hidden AVP to refuse; every body writer emits H=0 |
| `RFC2661-4.4.1-1` | Thus, if the M-bit is set within the Message Type AVP and the Message Type is unknown to the implementation, the tunnel MUST be cleared (§4.4.1) | MUST | 4.4.1 | **positive:** `unit/verify` [`TestUnknownMessageTypeClearsTunnelWhenMandatory`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_type_rfc2661_test.go#L30). **negative:** `unit/verify` [`TestUnknownMessageTypeClearsTunnelWhenMandatory`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_type_rfc2661_test.go#L31) |
| `RFC2661-4.4.1-2` | The M-bit MUST be set to 1 for all message types defined in this document (§4.4.1) | MUST | 4.4.1 | **positive:** `unit/verify` [`TestRFC2661WrittenAVPFlags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L202). **negative:** no negative test. **{single-polarity}:** a sender obligation on the eleven message writers. reliable.go::makeRecvEntry preserves the received M-bit so handleMessage can reject unknown mandatory types, but the RFC states no receiver refusal of a known type with M=0 |
| `RFC2661-4.4-3` | Every AVP whose Section 4.4 definition states "This AVP MUST NOT be hidden (the H-bit MUST be 0)" MUST be sent with H=0 (§4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestRFC2661WrittenAVPFlags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L209). **negative:** no negative test. **{single-polarity}:** a sender obligation; avp.go::skipHiddenAVP refuses every hidden AVP on receipt, so no receive-side input distinguishes the MUST-NOT-hide set |
| `RFC2661-4.4-1` | Every mandatory AVP definition in Section 4.4 states "The M-bit for this AVP MUST be set to 1", so the sender MUST set M=1 on each of those AVPs (§4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestRFC2661WrittenAVPFlags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L204). **negative:** no negative test. **{single-polarity}:** a sender obligation; the RFC states no receiver refusal of a mandatory AVP received with M=0 |
| `RFC2661-4.4.2-1` | Human readable text in all error messages MUST be provided in the UTF-8 charset using the Default Language [RFC2277] (§4.4.2) | MUST | 4.4.2 | **positive:** `unit/verify` [`TestRFC2661ResultCodeMessageUTF8`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L664). **negative:** no negative test. **{single-polarity}:** a sender obligation; tunnel_fsm.go::parseStopCCN refuses no message text and the RFC states no receiver refusal of invalid UTF-8 |
| `RFC2661-4.4.3-1` | A peer MUST NOT request an incoming or outgoing call with a Framing Type AVP specifying a value not advertised in the Framing Capabilities AVP it received during control connection establishment (§4.4.3) | MUST NOT | 4.4.3 | **positive:** `unit/verify` [`TestRFC2661CallCapabilitiesChecked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L431). **negative:** `unit/verify` [`TestRFC2661CallCapabilitiesChecked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L433) |
| `RFC2661-4.4.3-2` | This AVP MUST be present if the sender can place outgoing calls when requested (§4.4.3) | MUST | 4.4.3 | **positive:** `unit/verify` [`TestRFC2661BearerCapabilitiesEmitted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L477). **negative:** no negative test. **{single-polarity}:** a sender obligation on tunnel_initiator.go::writeSCCRQBody and tunnel_fsm.go::writeSCCRPBody, which always write the AVP; the receive side of an absent AVP is RFC2661-6.9-1 |
| `RFC2661-4.4.3-3` | The lower value "wins", and the "loser" MUST silently discard its tunnel (§4.4.3) | MUST | 4.4.3 | **positive:** `unit/verify` [`TestTieBreakerLoserDiscardsItsTunnel`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tiebreaker_rfc2661_test.go#L32). **negative:** `unit/verify` [`TestTieBreakerWinnerKeepsItsTunnel`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tiebreaker_rfc2661_test.go#L45) |
| `RFC2661-4.4.3-4` | In the case where a tie breaker is present on both sides, and the value is equal, both sides MUST discard their tunnels (§4.4.3) | MUST | 4.4.3 | **positive:** `unit/verify` [`TestTieBreakerEqualDiscardsBoth`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tiebreaker_rfc2661_test.go#L59). **negative:** `unit/verify` [`TestTieBreakerUnequalKeepsOneTunnel`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tiebreaker_rfc2661_test.go#L72) |
| `RFC2661-4.4-2` | Every optional AVP definition in Section 4.4 states "The M-bit for this AVP MUST be set to 0", so the sender MUST set M=0 on each of those AVPs (§4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestRFC2661WrittenAVPFlags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L207). **negative:** no negative test. **{single-polarity}:** a sender obligation on the one M=0 AVP Ze emits (Tie Breaker); the RFC states no receiver refusal of an optional AVP received with M=1 |
| `RFC2661-4.4.3-5` | The Host Name is of arbitrary length, but MUST be at least 1 octet (§4.4.3) | MUST | 4.4.3 | **positive:** `unit/verify` [`TestRFC2661HostNameAtLeastOneOctet`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L440). **negative:** `unit/verify` [`TestRFC2661HostNameAtLeastOneOctet`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L442) |
| `RFC2661-4.4.3-6` | The L2TP peer MUST place this value in the Tunnel ID header field of all control and data messages that it subsequently transmits over the associated tunnel (§4.4.3) | MUST | 4.4.3 | **positive:** `unit/verify` [`TestRFC2661HeaderTunnelIDBeforeAndAfterAssignment`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L183). **negative:** `unit/verify` [`TestRFC2661HeaderTunnelIDNeverWrong`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L211) |
| `RFC2661-4.4.3-7` | Before the Assigned Tunnel ID AVP is received from a peer, messages MUST be sent to that peer with a Tunnel ID value of 0 in the header of all control messages (§4.4.3) | MUST | 4.4.3 | **positive:** `unit/verify` [`TestRFC2661HeaderTunnelIDBeforeAndAfterAssignment`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L177). **negative:** `unit/verify` [`TestRFC2661HeaderTunnelIDNeverWrong`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L206) |
| `RFC2661-4.4.3-8` | In the StopCCN control message, the Assigned Tunnel ID AVP MUST be the same as the Assigned Tunnel ID AVP first sent to the receiving peer, permitting the peer to identify the appropriate tunnel even if a StopCCN is sent before an Assigned Tunnel ID AVP is received (§4.4.3) | MUST | 4.4.3 | **positive:** `unit/verify` [`TestRFC2661StopCCNRepeatsFirstAssignedTunnelID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L241). **negative:** `unit/verify` [`TestRFC2661StopCCNRepeatsFirstAssignedTunnelID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L243) |
| `RFC2661-4.4.3-9` | This AVP MUST be present in an SCCRP or SCCCN if a challenge was received in the preceding SCCRQ or SCCRP (§4.4.3) | MUST | 4.4.3 | **positive:** `unit/verify` [`TestRFC2661InitiatorAnswersChallenge`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L260). **negative:** `unit/verify` [`TestRFC2661AnsweringSideVerifiesChallengeResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L316) |
| `RFC2661-4.4.4-1` | The L2TP peer MUST place this value in the Session ID header field of all control and data messages that it subsequently transmits over the tunnel that belong to this session (§4.4.4) | MUST | 4.4.4 | **positive:** `unit/verify` [`TestRFC2661HeaderSessionIDBeforeAndAfterAssignment`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L363). **negative:** `unit/verify` [`TestRFC2661HeaderSessionIDNeverWrong`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L395) |
| `RFC2661-4.4.4-2` | Before the Assigned Session ID AVP is received from a peer, messages MUST be sent to that peer with a Session ID of 0 in the header of all control messages (§4.4.4) | MUST | 4.4.4 | **positive:** `unit/verify` [`TestRFC2661HeaderSessionIDBeforeAndAfterAssignment`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L357). **negative:** `unit/verify` [`TestRFC2661HeaderSessionIDNeverWrong`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L391) |
| `RFC2661-4.4.4-3` | Bits in the Value field of this AVP MUST only be set by the LNS for an OCRQ if it was set in the Bearer Capabilities AVP received from the LAC during control connection establishment (§4.4.4) | MUST | 4.4.4 | **positive:** `unit/verify` [`TestRFC2661CallCapabilitiesChecked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L423). **negative:** `unit/verify` [`TestRFC2661CallCapabilitiesChecked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L425) |
| `RFC2661-4.4.4-4` | Bits in the Value field of this AVP MUST only be set by the LNS for an OCRQ if it was set in the Framing Capabilities AVP received from the LAC during control connection establishment (§4.4.4) | MUST | 4.4.4 | **positive:** `unit/verify` [`TestRFC2661CallCapabilitiesChecked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L427). **negative:** `unit/verify` [`TestRFC2661CallCapabilitiesChecked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L429) |
| `RFC2661-4.4.4-5` | The Sequencing Required AVP, Attribute Type 39, indicates to the LNS that Sequence Numbers MUST always be present on the data channel (§4.4.4) | MUST | 4.4.4 | **positive:** `unit/verify` [`TestSequencingRequiredReachesKernelSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/lower_layer_rfc2661_linux_test.go#L99). **negative:** `unit/verify` [`TestSequencingRequiredReachesKernelSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/lower_layer_rfc2661_linux_test.go#L100) |
| `RFC2661-4.4.5-1` | This AVP MUST be present if proxy authentication is to be utilized (§4.4.5) | MUST | 4.4.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze offers no proxy authentication in either role, so the AVP is never emitted and never consumed; plan/spec-l2tp-proxy-authentication.md |
| `RFC2661-4.4.5-2` | This AVP MUST be present in messages containing a Proxy Authen Type AVP with an Authen Type of 1, 2, 3 or 5 (§4.4.5) | MUST | 4.4.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze offers no proxy authentication in either role, so the AVP is never emitted and never consumed; plan/spec-l2tp-proxy-authentication.md |
| `RFC2661-4.4.5-3` | This AVP MUST be present for Proxy Authen Types 2 and 5 (§4.4.5) | MUST | 4.4.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze offers no proxy authentication in either role, so the AVP is never emitted and never consumed; plan/spec-l2tp-proxy-authentication.md |
| `RFC2661-4.4.5-4` | ID is a 2 octet unsigned integer, the most significant octet MUST be 0 (§4.4.5) | MUST | 4.4.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** avp_compound.go::writeAVPProxyAuthenID writes the octet as 0 but has no non-test caller, and readProxyAuthenID ignores it; plan/spec-l2tp-proxy-authentication.md |
| `RFC2661-4.4.5-5` | The Proxy Authen ID AVP MUST be present for Proxy authen types 2, 3 and 5 (§4.4.5) | MUST | 4.4.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze offers no proxy authentication in either role, so the AVP is never emitted and never consumed; plan/spec-l2tp-proxy-authentication.md |
| `RFC2661-4.4.5-6` | This AVP MUST be present for Proxy authen types 1, 2, 3 and 5 (§4.4.5) | MUST | 4.4.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze offers no proxy authentication in either role, so the AVP is never emitted and never consumed; plan/spec-l2tp-proxy-authentication.md |
| `RFC2661-4.4.6-1` | Reserved - Not used, MUST be 0 CRC Errors - Number of PPP frames received with CRC errors since call was established Framing Errors - Number of improperly framed PPP packets received Hardware Overruns - Number of receive buffer over-runs since call was established Buffer Overruns - Number of buffer over-runs detected since call was established Time-out Errors - Number of time-outs since call was established Alignment Errors - Number of alignment errors since call was established (§4.4.6) | MUST | 4.4.6 | **positive:** `unit/verify` [`TestRFC2661CallErrorsReservedZero`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L459). **negative:** no negative test. **{single-polarity}:** a sender obligation on avp_compound.go::writeAVPCallErrors; the RFC states no receiver refusal of a non-zero reserved field and readCallErrors ignores it |
| `RFC2661-5.0-1` | The Tunnel and corresponding Control Connection MUST be established before an incoming or outgoing call is initiated (§5.0) | MUST | 5.0 | **positive:** `unit/verify` [`TestRFC2661CallNeedsEstablishedTunnel`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L489). **negative:** `unit/verify` [`TestRFC2661CallNeedsEstablishedTunnel`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L491) |
| `RFC2661-5.0-2` | An L2TP Session MUST be established before L2TP can begin to tunnel PPP frames (§5.0) | MUST | 5.0 | **positive:** `unit/verify` [`TestKernelSessionRequestedOnlyOnceEstablished`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/lower_layer_rfc2661_linux_test.go#L71). **negative:** `unit/verify` [`TestKernelSessionRequestedOnlyOnceEstablished`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/lower_layer_rfc2661_linux_test.go#L72) |
| `RFC2661-5.1.1-1` | If a Challenge AVP is received in an SCCRQ or SCCRP, a Challenge Response AVP MUST be sent in the following SCCRP or SCCCN, respectively (§5.1.1) | MUST | 5.1.1 | **positive:** `unit/verify` [`TestRFC2661InitiatorAnswersChallenge`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L263). **negative:** `unit/verify` [`TestRFC2661AnsweringSideVerifiesChallengeResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L319) |
| `RFC2661-5.1.1-2` | If the expected response and response received from a peer does not match, establishment of the tunnel MUST be disallowed (§5.1.1) | MUST | 5.1.1 | **positive:** `unit/verify` [`TestRFC2661AnsweringSideVerifiesChallengeResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L311). **negative:** `unit/verify` [`TestRFC2661AnsweringSideVerifiesChallengeResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L313) |
| `RFC2661-5.1.1-3` | To participate in tunnel authentication, a single shared secret MUST exist between the LAC and LNS (§5.1.1) | MUST | 5.1.1 | **positive:** `unit/verify` [`TestRFC2661InitiatorAnswersChallenge`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L265). **negative:** `unit/verify` [`TestRFC2661InitiatorWithoutSecretRefusesChallenge`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L282) |
| `RFC2661-5.3-1` | For the cases where a Session ID has not yet been assigned by the peer (i.e., during establishment of a new session or tunnel), the Session ID field MUST be sent as 0, and the Assigned Session ID AVP within the message MUST be used to identify the session (§5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC2661HeaderSessionIDBeforeAndAfterAssignment`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L360). **negative:** `unit/verify` [`TestRFC2661HeaderSessionIDNeverWrong`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L393) |
| `RFC2661-5.3-2` | Similarly, for cases where the Tunnel ID has not yet been assigned from the peer, the Tunnel ID MUST be sent as 0 and Assigned Tunnel ID AVP used to identify the tunnel (§5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC2661HeaderTunnelIDBeforeAndAfterAssignment`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L180). **negative:** `unit/verify` [`TestRFC2661HeaderTunnelIDNeverWrong`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L209) |
| `RFC2661-5.4-1` | If this AVP is present during session setup, sequence numbers MUST be present at all times (§5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestSequencingRequiredReachesKernelSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/lower_layer_rfc2661_linux_test.go#L101). **negative:** `unit/verify` [`TestSequencingRequiredReachesKernelSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/lower_layer_rfc2661_linux_test.go#L102) |
| `RFC2661-5.4-2` | Thus, if the LAC receives a data message without sequence numbers present, it MUST stop sending sequence numbers in future data messages (§5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestLACKernelSessionFollowsPeerSequencing`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/lower_layer_rfc2661_linux_test.go#L138). **negative:** `unit/verify` [`TestLACKernelSessionFollowsPeerSequencing`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/lower_layer_rfc2661_linux_test.go#L139) |
| `RFC2661-5.4-3` | If the LAC receives a data message with sequence numbers present, it MUST begin sending sequence numbers in future outgoing data messages (§5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestLACKernelSessionFollowsPeerSequencing`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/lower_layer_rfc2661_linux_test.go#L140). **negative:** `unit/verify` [`TestLACKernelSessionFollowsPeerSequencing`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/lower_layer_rfc2661_linux_test.go#L141) |
| `RFC2661-5.7-1` | The receiver of a StopCCN MUST send a ZLB ACK to acknowledge receipt of the message and maintain enough control connection state to properly accept StopCCN retransmissions over at least a full retransmission cycle (in case the ZLB ACK is lost) (§5.7) | MUST | 5.7 | **positive:** `unit/verify` [`TestRFC2661StopCCNAcknowledged`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L515). **negative:** `unit/verify` [`TestRFC2661StopCCNAcknowledged`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L519) |
| `RFC2661-5.8-10` | A peer MUST NOT withhold acknowledgment of messages as a technique for flow controlling control messages (§5.8) | MUST NOT | 5.8 | **positive:** `unit/verify` [`TestRFC2661AckNotWithheldUnderFlowControl`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L599). **negative:** `unit/verify` [`TestRFC2661AckNotWithheldUnderFlowControl`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L602) |
| `RFC2661-6.0-1` | Any "reserved" or "empty" fields MUST be sent as 0 values to allow for protocol extensibility (§6.0) | MUST | 6.0 | **positive:** no positive test. **negative:** no negative test. **{rollup}:** RFC2661-x-1, RFC2661-4.1-1, RFC2661-4.4.6-1; the sentence restates the reserved-field sender obligations those rows carry and asserts nothing of its own. **derived:** met |
| `RFC2661-6.3-1` | The following AVP MUST be present in the SCCCN: (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestRFC2661MandatoryAVPSetsEmitted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L285). **negative:** `unit/verify` [`TestRFC2661MandatoryAVPSetsParsed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L376) |
| `RFC2661-6.4-1` | The following AVPs MUST be present in the StopCCN: (§6.4) | MUST | 6.4 | **positive:** `unit/verify` [`TestRFC2661MandatoryAVPSetsEmitted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L287). **negative:** `unit/verify` [`TestRFC2661MandatoryAVPSetsParsed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L379) |
| `RFC2661-6.5-1` | A peer MUST NOT expect HELLO messages at any time or interval (§6.5) | MUST NOT | 6.5 | **positive:** `unit/verify` [`TestDeadPeerZLBAckKeepsTunnelUp`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reactor_test.go#L1317). **negative:** no negative test. **{single-polarity}:** the violation would be a teardown keyed on the absence of a peer HELLO, and reactor.go::handleTick keys teardown on lastLiveness alone, so no timer exists to drive |
| `RFC2661-6.5-2` | The Session ID in a HELLO message MUST be 0 (§6.5) | MUST | 6.5 | **positive:** `unit/verify` [`TestRFC2661HelloShape`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L489). **negative:** no negative test. **{single-polarity}:** a sender obligation on tunnel_fsm.go::handleHelloTimer; the RFC states no receiver refusal of a HELLO whose Session ID is not 0 |
| `RFC2661-6.5-3` | The Following AVP MUST be present in the HELLO message: (§6.5) | MUST | 6.5 | **positive:** `unit/verify` [`TestRFC2661HelloShape`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L491). **negative:** no negative test. **{single-polarity}:** a HELLO without a Message Type AVP is a malformed control message handled generically (tunnel_fsm.go::handleMessage), so no HELLO-specific negative exists |
| `RFC2661-6.6-1` | The following AVPs MUST be present in the ICRQ: (§6.6) | MUST | 6.6 | **positive:** `unit/verify` [`TestRFC2661MandatoryAVPSetsEmitted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L289). **negative:** `unit/verify` [`TestRFC2661MandatoryAVPSetsParsed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L382) |
| `RFC2661-6.7-1` | The following AVPs MUST be present in the ICRP: (§6.7) | MUST | 6.7 | **positive:** `unit/verify` [`TestRFC2661MandatoryAVPSetsEmitted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L291). **negative:** `unit/verify` [`TestRFC2661MandatoryAVPSetsParsed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L386) |
| `RFC2661-6.8-1` | The following AVPs MUST be present in the ICCN: (§6.8) | MUST | 6.8 | **positive:** `unit/verify` [`TestRFC2661MandatoryAVPSetsEmitted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L293). **negative:** `unit/verify` [`TestRFC2661MandatoryAVPSetsParsed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L389) |
| `RFC2661-6.9-1` | An LNS MUST have received a Bearer Capabilities AVP during tunnel establishment from an LAC in order to request an outgoing call to that LAC (§6.9) | MUST | 6.9 | **positive:** `unit/verify` [`TestRFC2661CallCapabilitiesChecked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L419). **negative:** `unit/verify` [`TestRFC2661CallCapabilitiesChecked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L421) |
| `RFC2661-6.9-2` | The following AVPs MUST be present in the OCRQ: (§6.9) | MUST | 6.9 | **positive:** `unit/verify` [`TestRFC2661MandatoryAVPSetsEmitted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L295). **negative:** `unit/verify` [`TestRFC2661MandatoryAVPSetsParsed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L393) |
| `RFC2661-6.10-1` | The following AVPs MUST be present in the OCRP: (§6.10) | MUST | 6.10 | **positive:** `unit/verify` [`TestRFC2661MandatoryAVPSetsEmitted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L297). **negative:** `unit/verify` [`TestRFC2661MandatoryAVPSetsParsed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L396) |
| `RFC2661-6.11-1` | The following AVPs MUST be present in the OCCN: (§6.11) | MUST | 6.11 | **positive:** `unit/verify` [`TestRFC2661MandatoryAVPSetsParsed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L402). **negative:** `unit/verify` [`TestRFC2661MandatoryAVPSetsParsed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L399) |
| `RFC2661-6.12-1` | The peer MUST clean up any resources, and does not send back any indication of success or failure for such cleanup (§6.12) | MUST | 6.12 | **positive:** `unit/verify` [`TestRFC2661CDNCleansUpSilently`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L701). **negative:** `unit/verify` [`TestRFC2661CDNCleansUpSilently`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L704) |
| `RFC2661-6.12-2` | The following AVPs MUST be present in the CDN: (§6.12) | MUST | 6.12 | **positive:** `unit/verify` [`TestRFC2661MandatoryAVPSetsEmitted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L299). **negative:** `unit/verify` [`TestRFC2661MandatoryAVPSetsParsed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L404) |
| `RFC2661-6.13-1` | The following AVPs MUST be present in the WEN: (§6.13) | MUST | 6.13 | **positive:** `unit/verify` [`TestRFC2661MandatoryAVPSetsParsed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L409). **negative:** `unit/verify` [`TestRFC2661MandatoryAVPSetsParsed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L407) |
| `RFC2661-6.14-1` | These options can change at any time during the life of the call, thus the LAC MUST be able to update its internal call information and behavior on an active PPP session (§6.14) | MUST | 6.14 | **positive:** `unit/verify` [`TestRFC2661SLIUpdatesACCM`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L766). **negative:** `unit/verify` [`TestRFC2661SLIUpdatesACCM`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L768) |
| `RFC2661-6.14-2` | The following AVPs MUST be present in the SLI: (§6.14) | MUST | 6.14 | **positive:** `unit/verify` [`TestRFC2661MandatoryAVPSetsParsed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L413). **negative:** `unit/verify` [`TestRFC2661MandatoryAVPSetsParsed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L411) |
| `RFC2661-7.2.1-1` | If the version is earlier and not supported, a StopCCN MUST be sent to the peer and the originator cleans up and terminates the tunnel (§7.2.1) | MUST | 7.2.1 | **positive:** `unit/verify` [`TestSCCRQAtAnUnsupportedProtocolVersionIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/rfc2661_protocol_version_test.go#L37). **negative:** `unit/verify` [`TestSCCRQAtTheSupportedProtocolVersionEstablishes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/rfc2661_protocol_version_test.go#L93) |
| `RFC2661-7.2.1-2` | In the event of a local termination, the originator MUST send a Stop-Control-Connection-Notification and clean up the tunnel (§7.2.1) | MUST | 7.2.1 | **positive:** `unit/verify` [`TestRFC2661LocalTerminationSendsStopCCN`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L536). **negative:** `unit/verify` [`TestRFC2661LocalTerminationSendsStopCCN`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L539) |
| `RFC2661-7.2.1-3` | If the originator receives a Stop-Control-Connection-Notification it MUST also clean up the tunnel (§7.2.1) | MUST | 7.2.1 | **positive:** `unit/verify` [`TestRFC2661OriginatorCleansUpOnStopCCN`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L573). **negative:** `unit/verify` [`TestRFC2661OriginatorCleansUpOnStopCCN`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L575) |
| `RFC2661-7.5-1` | The LAC MUST respond to the Outgoing-Call-Request message with an Outgoing-Call-Reply message once the LAC determines that the proper facilities exist to place the call and the call is administratively authorized (§7.5) | MUST | 7.5 | **positive:** `unit/verify` [`TestRFC2661OCRQAnsweredWithOCRP`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L795). **negative:** `unit/verify` [`TestRFC2661OCRQAnsweredWithOCRP`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L798) |
| `RFC2661-7.5.1-1` | established If a Call-Disconnect-Notify is received by the LAC, the telco call MUST be released via appropriate mechanisms and the session cleaned up (§7.5.1) | MUST | 7.5.1 | **positive:** `unit/verify` [`TestRFC2661CDNCleansUpSilently`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L706). **negative:** `unit/verify` [`TestRFC2661CDNCleansUpSilently`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L709) |
| `RFC2661-7.5.1-2` | If the call is disconnected by the client or the called interface, a Call-Disconnect-Notify message MUST be sent to the LNS (§7.5.1) | MUST | 7.5.1 | **positive:** `unit/verify` [`TestRFC2661LACSendsCDNOnDisconnect`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L742). **negative:** no negative test. **{single-polarity}:** session_fsm.go::teardownSession takes the session pointer and emits the CDN; there is no violating input to refuse at the tunnel level |
| `RFC2661-8.1-1` | Once the source and destination ports and addresses are established, they MUST remain static for the life of the tunnel (§8.1) | MUST | 8.1 | **positive:** `unit/verify` [`TestRFC2661LocalTerminationSendsStopCCN`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L541). **negative:** no negative test. **{single-polarity}:** tunnel.go::newTunnel fixes peerAddr and every sendRequest names it; the RFC states no receiver refusal of a peer that moves |
| `RFC2661-8.1-2` | The default for any L2TP implementation is that UDP checksums MUST be enabled for both control and data messages (§8.1) | MUST | 8.1 | **positive:** `unit/verify` [`TestControlSocketKeepsUDPChecksums`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/lower_layer_rfc2661_linux_test.go#L167). **negative:** no negative test. **{single-polarity}:** Ze never sets SO_NO_CHECK on the socket listener.go::newUDPListener opens, so the only assertable state is the checksum-enabled default it reads back |
| `RFC2661-8.1-3` | An L2TP implementation running on a system which does not support L2F MUST silently discard all L2F packets (§8.1) | MUST | 8.1 | **positive:** `unit/verify` [`TestRFC2661UnknownVersionRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L100). **negative:** `unit/verify` [`TestRFC2661KnownVersionAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L116) |
| `RFC2661-8.2-1` | When operating in IP environments, L2TP MUST offer the UDP encapsulation described in 8.1 as its default configuration for IP operation (§8.2) | MUST | 8.2 | **positive:** `unit/verify` [`TestRFC2661UDPEncapsulationOffered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L829). **negative:** no negative test. **{single-polarity}:** UDP is the only transport listener.go::newUDPListener offers, so there is nothing to refuse |
| `RFC2661-9.5-2` | If the LNS chooses to implement proxy authentication, it MUST be able to be configured off, requiring a new round a PPP authentication initiated by the LNS (which may or may not include a new round of LCP negotiation) (§9.5) | MUST | 9.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no LNS proxy authentication exists to configure off; plan/spec-l2tp-proxy-authentication.md |
| `RFC2661-5.8-8` | Retransmission count SHOULD be configurable (recommended 5) (§5.8) | SHOULD | 5.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC2661-x-2` | Slow start and congestion avoidance SHOULD be implemented (CWND/SSTHRESH per Appendix A) (Appendix A) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC2661-15-1` | HELLO keepalive SHOULD be sent when no control messages received for a configurable period (recommended 60 seconds). RFC 2661 Section 5.5 states the keepalive and RFC 2661 Section 6.5 the message. The id anchor below numbers no section of RFC 2661 and is frozen (§15) | SHOULD | 15 | **positive:** no positive test. **negative:** no negative test |
| `RFC2661-4.2-1` | Both peers MAY independently challenge each other during tunnel establishment. RFC 2661 Section 5.1.1 states tunnel authentication and RFC 2661 Section 4.4.3 defines the AVPs. The id anchor below is frozen and does NOT name Section 4.2, which is Mandatory AVPs (§4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2661-4.3-2` | Multiple hidden AVPs MAY share a single Random Vector AVP (§4.3) | MAY | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2661-5.8-9` | Out-of-order control messages MAY be queued or discarded (§5.8) | MAY | 5.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC2661-9.5-1` | Tie Breaker AVP MAY be included in SCCRQ for simultaneous-open resolution. RFC 2661 Section 4.4.3 defines the AVP and its resolution rule; RFC 2661 Section 7.2 names the collision. The id anchor below is frozen and does NOT name Section 9.5, which is Proxy PPP Authentication (§9.5) | MAY | 9.5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2661-4.3-1`](#rfc2661-4.3-1) A Random Vector AVP (type 36) MUST precede any hidden AVP (H=1) in the same message (§4.3) | {gap}, no test | the hidden-AVP MD5 cipher is implemented and unit-tested but is not wired into any control-message path -- no encoder sets H=1 or emits a Random Vector, and decoders skip hidden AVPs without decrypting or checking precedence (internal/component/l2tp/hidden.go:38 has no production caller; avp.go:156-168 skips hidden AVPs; AVPRandomVector avp.go:56 never emitted) |
| [`RFC2661-3.1-7`](#rfc2661-3.1-7) In data messages, Nr is reserved and, if present (as indicated by the S-bit), MUST be ignored upon receipt (§3.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux l2tp_ppp; internal/component/l2tp/kernel_linux.go::pppSetupReal creates the pppol2tp session the data plane runs on and installs no value the module reads to ignore Nr; reactor.go::handle drops every data message before reading Ns or Nr, so no value Ze writes decides this field |
| [`RFC2661-4.2-2`](#rfc2661-4.2-2) Use of the M-bit with new AVPs (those not defined in this document) MUST provide the ability to configure the associated feature off, such that the AVP is either not sent, or sent with the M-bit not set (§4.2) | {gap}, no test | Ze defines no AVP outside this document, so no feature exists to configure off; plan/spec-l2tp-new-avp-definitions.md |
| [`RFC2661-4.4.5-1`](#rfc2661-4.4.5-1) This AVP MUST be present if proxy authentication is to be utilized (§4.4.5) | {gap}, no test | Ze offers no proxy authentication in either role, so the AVP is never emitted and never consumed; plan/spec-l2tp-proxy-authentication.md |
| [`RFC2661-4.4.5-2`](#rfc2661-4.4.5-2) This AVP MUST be present in messages containing a Proxy Authen Type AVP with an Authen Type of 1, 2, 3 or 5 (§4.4.5) | {gap}, no test | Ze offers no proxy authentication in either role, so the AVP is never emitted and never consumed; plan/spec-l2tp-proxy-authentication.md |
| [`RFC2661-4.4.5-3`](#rfc2661-4.4.5-3) This AVP MUST be present for Proxy Authen Types 2 and 5 (§4.4.5) | {gap}, no test | Ze offers no proxy authentication in either role, so the AVP is never emitted and never consumed; plan/spec-l2tp-proxy-authentication.md |
| [`RFC2661-4.4.5-4`](#rfc2661-4.4.5-4) ID is a 2 octet unsigned integer, the most significant octet MUST be 0 (§4.4.5) | {gap}, no test | avp_compound.go::writeAVPProxyAuthenID writes the octet as 0 but has no non-test caller, and readProxyAuthenID ignores it; plan/spec-l2tp-proxy-authentication.md |
| [`RFC2661-4.4.5-5`](#rfc2661-4.4.5-5) The Proxy Authen ID AVP MUST be present for Proxy authen types 2, 3 and 5 (§4.4.5) | {gap}, no test | Ze offers no proxy authentication in either role, so the AVP is never emitted and never consumed; plan/spec-l2tp-proxy-authentication.md |
| [`RFC2661-4.4.5-6`](#rfc2661-4.4.5-6) This AVP MUST be present for Proxy authen types 1, 2, 3 and 5 (§4.4.5) | {gap}, no test | Ze offers no proxy authentication in either role, so the AVP is never emitted and never consumed; plan/spec-l2tp-proxy-authentication.md |
| [`RFC2661-6.0-1`](#rfc2661-6.0-1) Any "reserved" or "empty" fields MUST be sent as 0 values to allow for protocol extensibility (§6.0) | no test | no test carries this requirement id; annotated {rollup}: RFC2661-x-1, RFC2661-4.1-1, RFC2661-4.4.6-1; the sentence restates the reserved-field sender obligations those rows carry and asserts nothing of its own |
| [`RFC2661-9.5-2`](#rfc2661-9.5-2) If the LNS chooses to implement proxy authentication, it MUST be able to be configured off, requiring a new round a PPP authentication initiated by the LNS (which may or may not include a new round of LCP negotiation) (§9.5) | {gap}, no test | no LNS proxy authentication exists to configure off; plan/spec-l2tp-proxy-authentication.md |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2661-x-1`](#rfc2661-x-1)

Reserved bits 8-11 in L2TP header MUST be 0 (Wire Format)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestWriteControlHeader`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/header_test.go#L230) | unit/verify | unproven |
| positive | [`rfc2661-emitted-control-shape.ci`](https://github.com/ze-software/ze/blob/main/test/l2tp/rfc2661-emitted-control-shape.ci#L27) | functional/verify | revert, verified |

### [`RFC2661-4.1-1`](#rfc2661-4.1-1)

AVP reserved bits 2-5 MUST be zero on send; non-zero on receive means treat AVP as unrecognized (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAVPIteratorReservedBits`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/avp_test.go#L107) | unit/verify | unproven |
| positive | [`TestAVPCatalogRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/avp_test.go#L180) | unit/verify | unproven |
| positive | [`rfc2661-emitted-control-shape.ci`](https://github.com/ze-software/ze/blob/main/test/l2tp/rfc2661-emitted-control-shape.ci#L29) | functional/verify | revert, verified |

### [`RFC2661-4.1-2`](#rfc2661-4.1-2)

Message Type AVP (type 0) MUST be the first AVP in every control message. RFC 2661 Section 4.4.1: "The Message Type AVP MUST be the first AVP in a message, immediately following the control message header". The id anchor below is frozen and does NOT name Section 4.1, which is AVP Format (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReactor_MalformedSCCRQCreatesNoTunnel`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reactor_test.go#L256) | unit/verify | unproven |
| positive | [`TestWriteICRPBody`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L945) | unit/verify | unproven |
| positive | [`rfc2661-emitted-control-shape.ci`](https://github.com/ze-software/ze/blob/main/test/l2tp/rfc2661-emitted-control-shape.ci#L24) | functional/verify | revert, verified |

### [`RFC2661-4.1-3`](#rfc2661-4.1-3)

If M=1 and AVP is unrecognized and session-scoped, send CDN and tear down session (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSession_UnknownMandatoryAVP`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L559) | unit/verify | unproven |
| positive | [`TestSession_IncomingLNS_ICRQ`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L126) | unit/verify | unproven |

### [`RFC2661-4.1-4`](#rfc2661-4.1-4)

If M=1 and AVP is unrecognized and tunnel-scoped, send StopCCN and tear down tunnel (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTunnelSCCCNUnknownMandatoryAVP_StopCCN`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_initiator_test.go#L253) | unit/verify | unproven |
| positive | [`TestTunnelInitiatorHandshake`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_initiator_test.go#L134) | unit/verify | unproven |

### [`RFC2661-5.8-1`](#rfc2661-5.8-1)

Each subsequent retransmission of a message MUST employ an exponential backoff interval. (§5.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestTickBackoffSchedule`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_test.go#L374) | unit/verify | unproven |

### [`RFC2661-5.8-2`](#rfc2661-5.8-2)

This cap MUST be no less than 8 seconds per retransmission. (§5.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBackoffCapAtLeast8Seconds`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_seq_test.go#L94) | unit/verify | unproven |

### [`RFC2661-5.8-3`](#rfc2661-5.8-3)

After exhausting retransmissions without response, tunnel and all sessions MUST be cleared (§5.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTickMaxAttempts`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_test.go#L417) | unit/verify | unproven |
| positive | [`TestPeerTeardownWithdrawsSubscriberRoute`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reactor_test.go#L1204) | unit/verify | unproven |
| positive | [`TestTickMaxAttempts`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_test.go#L414) | unit/verify | unproven |

### [`RFC2661-5.8-4`](#rfc2661-5.8-4)

The retransmitted message contains the same Ns value, but the Nr value MUST be updated with the sequence number of the next expected message. (§5.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTickRetransmit`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_test.go#L342) | unit/verify | unproven |
| positive | [`TestTickRetransmit`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_test.go#L339) | unit/verify | unproven |

### [`RFC2661-5.8-5`](#rfc2661-5.8-5)

Duplicate control messages MUST be acknowledged (via ZLB or piggyback) even though not processed by upper layer (§5.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOnReceiveDuplicate`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_test.go#L148) | unit/verify | unproven |
| positive | [`TestOnReceiveDuplicate`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_test.go#L146) | unit/verify | unproven |

### [`RFC2661-5.8-6`](#rfc2661-5.8-6)

An implementation may support a receive window of only 1 (i.e., by sending out a Receive Window Size AVP with a value of 1), but MUST accept a window of up to 4 from its peer (e.g. have the ability to send 4 messages before backing off). (§5.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestWindowPeerRWSZero`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_window_test.go#L112) | unit/verify | unproven |
| positive | [`TestWindowAvailable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_window_test.go#L183) | unit/verify | unproven |

### [`RFC2661-5.8-7`](#rfc2661-5.8-7)

When a tunnel is being shut down for reasons other than loss of connectivity, the state and reliable delivery mechanisms MUST be maintained and operated for the full retransmission interval after the final message exchange has occurred. (§5.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPostTeardownAckRetention`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_integration_test.go#L195) | unit/verify | unproven |
| negative | [`TestExpired`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_test.go#L563) | unit/verify | unproven |
| positive | [`TestPostTeardownAckRetention`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_integration_test.go#L191) | unit/verify | unproven |
| positive | [`TestExpired`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_test.go#L560) | unit/verify | unproven |

### [`RFC2661-4.3-1`](#rfc2661-4.3-1)

A Random Vector AVP (type 36) MUST precede any hidden AVP (H=1) in the same message (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2661-4.3-1, so no unit is bound to it.

### [`RFC2661-6.1-1`](#rfc2661-6.1-1)

Every AVP RFC 2661 Section 6.1 makes mandatory in an SCCRQ is required, and an SCCRQ missing one is answered with StopCCN rather than dropped in silence. RFC 2661 Section 6.1: "The following AVPs MUST be present in the SCCRQ: Message Type AVP, Protocol Version, Host Name, Framing Capabilities, Assigned Tunnel ID"; RFC 2661 Section 7.1: "Examples of a malformed control message include ... a message that is missing a required AVP", and receipt of one "should be logged appropriately and the control connection cleared to ensure recovery to a known state" (§6.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSCCRQMissingMandatoryAVPIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reactor_sccrq_mandatory_avp_test.go#L45) | unit/verify | revert, verified |
| negative | [`TestSCCRQWithShortFramingCapabilitiesIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reactor_sccrq_mandatory_avp_test.go#L149) | unit/verify | revert, verified |
| negative | [`rfc2661-sccrq-mandatory-avp.ci`](https://github.com/ze-software/ze/blob/main/test/l2tp/rfc2661-sccrq-mandatory-avp.ci#L24) | functional/verify | revert, verified |
| positive | [`TestSCCRQWithEveryMandatoryAVPEstablishes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reactor_sccrq_mandatory_avp_test.go#L108) | unit/verify | revert, verified |
| positive | [`rfc2661-sccrq-mandatory-avp.ci`](https://github.com/ze-software/ze/blob/main/test/l2tp/rfc2661-sccrq-mandatory-avp.ci#L28) | functional/verify | revert, verified |

### [`RFC2661-6.2-1`](#rfc2661-6.2-1)

Every AVP RFC 2661 Section 6.2 makes mandatory in an SCCRP is required, and an SCCRP missing one tears the dialed tunnel down with StopCCN rather than establishing it. RFC 2661 Section 6.2: "The following AVPs MUST be present in the SCCRP: Message Type, Protocol Version, Framing Capabilities, Host Name, Assigned Tunnel ID"; RFC 2661 Section 7.2.1 gives wait-ctl-reply the row "Receive SCCRP, not acceptable | Send StopCCN, Clean up | idle" (§6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSCCRPMissingMandatoryAVPTearsTheTunnelDown`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_initiator_sccrp_mandatory_avp_test.go#L58) | unit/verify | revert, verified |
| positive | [`TestSCCRPWithEveryMandatoryAVPEstablishes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_initiator_sccrp_mandatory_avp_test.go#L108) | unit/verify | revert, verified |

### [`RFC2661-24.10-1`](#rfc2661-24.10-1)

Assigned Tunnel ID of 0 in SCCRQ/SCCRP is a protocol error; reject with StopCCN. RFC 2661 Section 4.4.3: "The Assigned Tunnel ID is a 2 octet non-zero unsigned integer"; RFC 2661 Section 5.3: the value 0 "MUST NOT be used as an Assigned Session ID or Assigned Tunnel ID". The id anchor below numbers no section of RFC 2661 and is frozen (§24.10)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSCCRQWithZeroAssignedTunnelIDIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reactor_sccrq_zero_tid_test.go#L75) | unit/verify | unproven |
| negative | [`TestParseSCCRP_Rejects`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_initiator_test.go#L109) | unit/verify | unproven |
| negative | [`rfc2661-sccrq-tunnel-id-zero.ci`](https://github.com/ze-software/ze/blob/main/test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci#L21) | functional/verify | unproven |
| positive | [`TestSCCRQWithNonZeroAssignedTunnelIDEstablishes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reactor_sccrq_zero_tid_test.go#L117) | unit/verify | unproven |
| positive | [`TestTunnelInitiatorHandshake`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_initiator_test.go#L137) | unit/verify | unproven |
| positive | [`rfc2661-sccrq-tunnel-id-zero.ci`](https://github.com/ze-software/ze/blob/main/test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci#L24) | functional/verify | unproven |

### [`RFC2661-24.12-1`](#rfc2661-24.12-1)

Unknown M=1 vendor AVP in a session context tears down the session with CDN, not the tunnel with StopCCN. RFC 2661 Section 4.1 states the session/tunnel split and RFC 2661 Section 4.2 states the consequence. The id anchor below numbers no section of RFC 2661 and is frozen (§24.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSession_UnknownMandatoryAVP`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L563) | unit/verify | unproven |
| positive | [`TestSession_UnknownMandatoryAVP`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L561) | unit/verify | unproven |

### [`RFC2661-10-1`](#rfc2661-10-1)

CDN is valid in any non-idle session state; receiving CDN destroys the session. RFC 2661 Section 5.6 states session teardown by CDN and RFC 2661 Section 7.4.2 gives the state table. The id anchor below is frozen and does NOT name Section 10, which is IANA Considerations (§10)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSession_CDN_UnknownSessionDropped`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L366) | unit/verify | unproven |
| positive | [`TestSession_CDN_AnyState`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L342) | unit/verify | unproven |
| positive | [`TestSession_CDN_EstablishedSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L313) | unit/verify | unproven |

### [`RFC2661-9-1`](#rfc2661-9-1)

StopCCN cascades: all sessions in a tunnel are cleared when StopCCN is received. RFC 2661 Section 5.7: an implementation "may shut down an entire tunnel and all sessions on the tunnel by sending the StopCCN". The id anchor below is frozen and does NOT name Section 9, which is Security Considerations (§9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestStopCCNQueuesAllTeardowns`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L814) | unit/verify | unproven |
| positive | [`TestSession_StopCCN_CascadeSessions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L403) | unit/verify | unproven |

### [`RFC2661-10-2`](#rfc2661-10-2)

Session ID 0 is reserved and never assigned. RFC 2661 Section 5.3: "The value of 0 for Session ID and Tunnel ID is special and MUST NOT be used as an Assigned Session ID or Assigned Tunnel ID". The id anchor below is frozen and does NOT name Section 10, which is IANA Considerations (§10)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSession_SIDBoundary_Zero`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L1055) | unit/verify | unproven |
| positive | [`TestSession_SIDBoundary_MaxUint16`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L1041) | unit/verify | unproven |

### [`RFC2661-3.1-1`](#rfc2661-3.1-1)

This bit MUST be set to 1 for control messages (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2661HeaderFlagWord`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L19) | unit/verify | revert, verified |

### [`RFC2661-3.1-2`](#rfc2661-3.1-2)

The S bit MUST be set to 1 for control messages (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661ControlHeaderWithoutSequenceRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L77) | unit/verify | revert, verified |
| positive | [`TestRFC2661HeaderFlagWord`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L21) | unit/verify | revert, verified |

### [`RFC2661-3.1-3`](#rfc2661-3.1-3)

The O bit MUST be set to 0 (zero) for control messages (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2661HeaderFlagWord`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L23) | unit/verify | revert, verified |

### [`RFC2661-3.1-4`](#rfc2661-3.1-4)

The P bit MUST be set to 0 for all control messages (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2661HeaderFlagWord`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L25) | unit/verify | revert, verified |

### [`RFC2661-3.1-5`](#rfc2661-3.1-5)

Ver MUST be 2, indicating the version of the L2TP data message header described in this document (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661UnknownVersionRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L95) | unit/verify | revert, verified |
| positive | [`TestRFC2661HeaderFlagWord`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L27) | unit/verify | revert, verified |

### [`RFC2661-3.1-6`](#rfc2661-3.1-6)

Packets received with an unknown Ver field MUST be discarded (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661KnownVersionAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L114) | unit/verify | revert, verified |
| positive | [`TestRFC2661UnknownVersionRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L97) | unit/verify | revert, verified |

### [`RFC2661-3.1-7`](#rfc2661-3.1-7)

In data messages, Nr is reserved and, if present (as indicated by the S-bit), MUST be ignored upon receipt (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2661-3.1-7, so no unit is bound to it.

### [`RFC2661-4.1-5`](#rfc2661-4.1-5)

If the M bit is not set, an unrecognized AVP MUST be ignored (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661UnrecognizedOptionalAVPIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L641) | unit/verify | revert, verified |
| positive | [`TestRFC2661UnrecognizedOptionalAVPIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L637) | unit/verify | revert, verified |

### [`RFC2661-4.2-2`](#rfc2661-4.2-2)

Use of the M-bit with new AVPs (those not defined in this document) MUST provide the ability to configure the associated feature off, such that the AVP is either not sent, or sent with the M-bit not set (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2661-4.2-2, so no unit is bound to it.

### [`RFC2661-4.3-3`](#rfc2661-4.3-3)

The H bit MUST only be set if a shared secret exists between the LAC and LNS (§4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2661WrittenAVPFlags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L211) | unit/verify | revert, verified |

### [`RFC2661-4.4.1-1`](#rfc2661-4.4.1-1)

Thus, if the M-bit is set within the Message Type AVP and the Message Type is unknown to the implementation, the tunnel MUST be cleared (§4.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestUnknownMessageTypeClearsTunnelWhenMandatory`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_type_rfc2661_test.go#L31) | unit/verify | revert, verified |
| positive | [`TestUnknownMessageTypeClearsTunnelWhenMandatory`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_type_rfc2661_test.go#L30) | unit/verify | revert, verified |

### [`RFC2661-4.4.1-2`](#rfc2661-4.4.1-2)

The M-bit MUST be set to 1 for all message types defined in this document (§4.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2661WrittenAVPFlags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L202) | unit/verify | revert, verified |

### [`RFC2661-4.4-3`](#rfc2661-4.4-3)

Every AVP whose Section 4.4 definition states "This AVP MUST NOT be hidden (the H-bit MUST be 0)" MUST be sent with H=0 (§4.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2661WrittenAVPFlags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L209) | unit/verify | revert, verified |

### [`RFC2661-4.4-1`](#rfc2661-4.4-1)

Every mandatory AVP definition in Section 4.4 states "The M-bit for this AVP MUST be set to 1", so the sender MUST set M=1 on each of those AVPs (§4.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2661WrittenAVPFlags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L204) | unit/verify | revert, verified |

### [`RFC2661-4.4.2-1`](#rfc2661-4.4.2-1)

Human readable text in all error messages MUST be provided in the UTF-8 charset using the Default Language [RFC2277] (§4.4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2661ResultCodeMessageUTF8`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L664) | unit/verify | revert, verified |

### [`RFC2661-4.4.3-1`](#rfc2661-4.4.3-1)

A peer MUST NOT request an incoming or outgoing call with a Framing Type AVP specifying a value not advertised in the Framing Capabilities AVP it received during control connection establishment (§4.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661CallCapabilitiesChecked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L433) | unit/verify | revert, verified |
| positive | [`TestRFC2661CallCapabilitiesChecked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L431) | unit/verify | revert, verified |

### [`RFC2661-4.4.3-2`](#rfc2661-4.4.3-2)

This AVP MUST be present if the sender can place outgoing calls when requested (§4.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2661BearerCapabilitiesEmitted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L477) | unit/verify | revert, verified |

### [`RFC2661-4.4.3-3`](#rfc2661-4.4.3-3)

The lower value "wins", and the "loser" MUST silently discard its tunnel (§4.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTieBreakerWinnerKeepsItsTunnel`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tiebreaker_rfc2661_test.go#L45) | unit/verify | revert, verified |
| positive | [`TestTieBreakerLoserDiscardsItsTunnel`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tiebreaker_rfc2661_test.go#L32) | unit/verify | revert, verified |

### [`RFC2661-4.4.3-4`](#rfc2661-4.4.3-4)

In the case where a tie breaker is present on both sides, and the value is equal, both sides MUST discard their tunnels (§4.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTieBreakerUnequalKeepsOneTunnel`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tiebreaker_rfc2661_test.go#L72) | unit/verify | revert, verified |
| positive | [`TestTieBreakerEqualDiscardsBoth`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tiebreaker_rfc2661_test.go#L59) | unit/verify | revert, verified |

### [`RFC2661-4.4-2`](#rfc2661-4.4-2)

Every optional AVP definition in Section 4.4 states "The M-bit for this AVP MUST be set to 0", so the sender MUST set M=0 on each of those AVPs (§4.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2661WrittenAVPFlags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L207) | unit/verify | revert, verified |

### [`RFC2661-4.4.3-5`](#rfc2661-4.4.3-5)

The Host Name is of arbitrary length, but MUST be at least 1 octet (§4.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661HostNameAtLeastOneOctet`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L442) | unit/verify | revert, verified |
| positive | [`TestRFC2661HostNameAtLeastOneOctet`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L440) | unit/verify | revert, verified |

### [`RFC2661-4.4.3-6`](#rfc2661-4.4.3-6)

The L2TP peer MUST place this value in the Tunnel ID header field of all control and data messages that it subsequently transmits over the associated tunnel (§4.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661HeaderTunnelIDNeverWrong`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L211) | unit/verify | revert, verified |
| positive | [`TestRFC2661HeaderTunnelIDBeforeAndAfterAssignment`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L183) | unit/verify | revert, verified |

### [`RFC2661-4.4.3-7`](#rfc2661-4.4.3-7)

Before the Assigned Tunnel ID AVP is received from a peer, messages MUST be sent to that peer with a Tunnel ID value of 0 in the header of all control messages (§4.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661HeaderTunnelIDNeverWrong`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L206) | unit/verify | revert, verified |
| positive | [`TestRFC2661HeaderTunnelIDBeforeAndAfterAssignment`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L177) | unit/verify | revert, verified |

### [`RFC2661-4.4.3-8`](#rfc2661-4.4.3-8)

In the StopCCN control message, the Assigned Tunnel ID AVP MUST be the same as the Assigned Tunnel ID AVP first sent to the receiving peer, permitting the peer to identify the appropriate tunnel even if a StopCCN is sent before an Assigned Tunnel ID AVP is received (§4.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661StopCCNRepeatsFirstAssignedTunnelID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L243) | unit/verify | revert, verified |
| positive | [`TestRFC2661StopCCNRepeatsFirstAssignedTunnelID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L241) | unit/verify | revert, verified |

### [`RFC2661-4.4.3-9`](#rfc2661-4.4.3-9)

This AVP MUST be present in an SCCRP or SCCCN if a challenge was received in the preceding SCCRQ or SCCRP (§4.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661AnsweringSideVerifiesChallengeResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L316) | unit/verify | revert, verified |
| positive | [`TestRFC2661InitiatorAnswersChallenge`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L260) | unit/verify | revert, verified |

### [`RFC2661-4.4.4-1`](#rfc2661-4.4.4-1)

The L2TP peer MUST place this value in the Session ID header field of all control and data messages that it subsequently transmits over the tunnel that belong to this session (§4.4.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661HeaderSessionIDNeverWrong`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L395) | unit/verify | revert, verified |
| positive | [`TestRFC2661HeaderSessionIDBeforeAndAfterAssignment`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L363) | unit/verify | revert, verified |

### [`RFC2661-4.4.4-2`](#rfc2661-4.4.4-2)

Before the Assigned Session ID AVP is received from a peer, messages MUST be sent to that peer with a Session ID of 0 in the header of all control messages (§4.4.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661HeaderSessionIDNeverWrong`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L391) | unit/verify | revert, verified |
| positive | [`TestRFC2661HeaderSessionIDBeforeAndAfterAssignment`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L357) | unit/verify | revert, verified |

### [`RFC2661-4.4.4-3`](#rfc2661-4.4.4-3)

Bits in the Value field of this AVP MUST only be set by the LNS for an OCRQ if it was set in the Bearer Capabilities AVP received from the LAC during control connection establishment (§4.4.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661CallCapabilitiesChecked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L425) | unit/verify | revert, verified |
| positive | [`TestRFC2661CallCapabilitiesChecked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L423) | unit/verify | revert, verified |

### [`RFC2661-4.4.4-4`](#rfc2661-4.4.4-4)

Bits in the Value field of this AVP MUST only be set by the LNS for an OCRQ if it was set in the Framing Capabilities AVP received from the LAC during control connection establishment (§4.4.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661CallCapabilitiesChecked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L429) | unit/verify | revert, verified |
| positive | [`TestRFC2661CallCapabilitiesChecked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L427) | unit/verify | revert, verified |

### [`RFC2661-4.4.4-5`](#rfc2661-4.4.4-5)

The Sequencing Required AVP, Attribute Type 39, indicates to the LNS that Sequence Numbers MUST always be present on the data channel (§4.4.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSequencingRequiredReachesKernelSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/lower_layer_rfc2661_linux_test.go#L100) | unit/verify | revert, verified |
| positive | [`TestSequencingRequiredReachesKernelSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/lower_layer_rfc2661_linux_test.go#L99) | unit/verify | revert, verified |

### [`RFC2661-4.4.5-1`](#rfc2661-4.4.5-1)

This AVP MUST be present if proxy authentication is to be utilized (§4.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2661-4.4.5-1, so no unit is bound to it.

### [`RFC2661-4.4.5-2`](#rfc2661-4.4.5-2)

This AVP MUST be present in messages containing a Proxy Authen Type AVP with an Authen Type of 1, 2, 3 or 5 (§4.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2661-4.4.5-2, so no unit is bound to it.

### [`RFC2661-4.4.5-3`](#rfc2661-4.4.5-3)

This AVP MUST be present for Proxy Authen Types 2 and 5 (§4.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2661-4.4.5-3, so no unit is bound to it.

### [`RFC2661-4.4.5-4`](#rfc2661-4.4.5-4)

ID is a 2 octet unsigned integer, the most significant octet MUST be 0 (§4.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2661-4.4.5-4, so no unit is bound to it.

### [`RFC2661-4.4.5-5`](#rfc2661-4.4.5-5)

The Proxy Authen ID AVP MUST be present for Proxy authen types 2, 3 and 5 (§4.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2661-4.4.5-5, so no unit is bound to it.

### [`RFC2661-4.4.5-6`](#rfc2661-4.4.5-6)

This AVP MUST be present for Proxy authen types 1, 2, 3 and 5 (§4.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2661-4.4.5-6, so no unit is bound to it.

### [`RFC2661-4.4.6-1`](#rfc2661-4.4.6-1)

Reserved - Not used, MUST be 0 CRC Errors - Number of PPP frames received with CRC errors since call was established Framing Errors - Number of improperly framed PPP packets received Hardware Overruns - Number of receive buffer over-runs since call was established Buffer Overruns - Number of buffer over-runs detected since call was established Time-out Errors - Number of time-outs since call was established Alignment Errors - Number of alignment errors since call was established (§4.4.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2661CallErrorsReservedZero`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L459) | unit/verify | revert, verified |

### [`RFC2661-5.0-1`](#rfc2661-5.0-1)

The Tunnel and corresponding Control Connection MUST be established before an incoming or outgoing call is initiated (§5.0)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661CallNeedsEstablishedTunnel`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L491) | unit/verify | revert, verified |
| positive | [`TestRFC2661CallNeedsEstablishedTunnel`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L489) | unit/verify | revert, verified |

### [`RFC2661-5.0-2`](#rfc2661-5.0-2)

An L2TP Session MUST be established before L2TP can begin to tunnel PPP frames (§5.0)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestKernelSessionRequestedOnlyOnceEstablished`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/lower_layer_rfc2661_linux_test.go#L72) | unit/verify | revert, verified |
| positive | [`TestKernelSessionRequestedOnlyOnceEstablished`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/lower_layer_rfc2661_linux_test.go#L71) | unit/verify | revert, verified |

### [`RFC2661-5.1.1-1`](#rfc2661-5.1.1-1)

If a Challenge AVP is received in an SCCRQ or SCCRP, a Challenge Response AVP MUST be sent in the following SCCRP or SCCCN, respectively (§5.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661AnsweringSideVerifiesChallengeResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L319) | unit/verify | revert, verified |
| positive | [`TestRFC2661InitiatorAnswersChallenge`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L263) | unit/verify | revert, verified |

### [`RFC2661-5.1.1-2`](#rfc2661-5.1.1-2)

If the expected response and response received from a peer does not match, establishment of the tunnel MUST be disallowed (§5.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661AnsweringSideVerifiesChallengeResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L313) | unit/verify | revert, verified |
| positive | [`TestRFC2661AnsweringSideVerifiesChallengeResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L311) | unit/verify | revert, verified |

### [`RFC2661-5.1.1-3`](#rfc2661-5.1.1-3)

To participate in tunnel authentication, a single shared secret MUST exist between the LAC and LNS (§5.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661InitiatorWithoutSecretRefusesChallenge`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L282) | unit/verify | revert, verified |
| positive | [`TestRFC2661InitiatorAnswersChallenge`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L265) | unit/verify | revert, verified |

### [`RFC2661-5.3-1`](#rfc2661-5.3-1)

For the cases where a Session ID has not yet been assigned by the peer (i.e., during establishment of a new session or tunnel), the Session ID field MUST be sent as 0, and the Assigned Session ID AVP within the message MUST be used to identify the session (§5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661HeaderSessionIDNeverWrong`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L393) | unit/verify | revert, verified |
| positive | [`TestRFC2661HeaderSessionIDBeforeAndAfterAssignment`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L360) | unit/verify | revert, verified |

### [`RFC2661-5.3-2`](#rfc2661-5.3-2)

Similarly, for cases where the Tunnel ID has not yet been assigned from the peer, the Tunnel ID MUST be sent as 0 and Assigned Tunnel ID AVP used to identify the tunnel (§5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661HeaderTunnelIDNeverWrong`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L209) | unit/verify | revert, verified |
| positive | [`TestRFC2661HeaderTunnelIDBeforeAndAfterAssignment`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L180) | unit/verify | revert, verified |

### [`RFC2661-5.4-1`](#rfc2661-5.4-1)

If this AVP is present during session setup, sequence numbers MUST be present at all times (§5.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSequencingRequiredReachesKernelSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/lower_layer_rfc2661_linux_test.go#L102) | unit/verify | revert, verified |
| positive | [`TestSequencingRequiredReachesKernelSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/lower_layer_rfc2661_linux_test.go#L101) | unit/verify | revert, verified |

### [`RFC2661-5.4-2`](#rfc2661-5.4-2)

Thus, if the LAC receives a data message without sequence numbers present, it MUST stop sending sequence numbers in future data messages (§5.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLACKernelSessionFollowsPeerSequencing`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/lower_layer_rfc2661_linux_test.go#L139) | unit/verify | revert, verified |
| positive | [`TestLACKernelSessionFollowsPeerSequencing`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/lower_layer_rfc2661_linux_test.go#L138) | unit/verify | revert, verified |

### [`RFC2661-5.4-3`](#rfc2661-5.4-3)

If the LAC receives a data message with sequence numbers present, it MUST begin sending sequence numbers in future outgoing data messages (§5.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLACKernelSessionFollowsPeerSequencing`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/lower_layer_rfc2661_linux_test.go#L141) | unit/verify | revert, verified |
| positive | [`TestLACKernelSessionFollowsPeerSequencing`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/lower_layer_rfc2661_linux_test.go#L140) | unit/verify | revert, verified |

### [`RFC2661-5.7-1`](#rfc2661-5.7-1)

The receiver of a StopCCN MUST send a ZLB ACK to acknowledge receipt of the message and maintain enough control connection state to properly accept StopCCN retransmissions over at least a full retransmission cycle (in case the ZLB ACK is lost) (§5.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661StopCCNAcknowledged`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L519) | unit/verify | revert, verified |
| positive | [`TestRFC2661StopCCNAcknowledged`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L515) | unit/verify | revert, verified |

### [`RFC2661-5.8-10`](#rfc2661-5.8-10)

A peer MUST NOT withhold acknowledgment of messages as a technique for flow controlling control messages (§5.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661AckNotWithheldUnderFlowControl`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L602) | unit/verify | revert, verified |
| positive | [`TestRFC2661AckNotWithheldUnderFlowControl`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L599) | unit/verify | revert, verified |

### [`RFC2661-6.0-1`](#rfc2661-6.0-1)

Any "reserved" or "empty" fields MUST be sent as 0 values to allow for protocol extensibility (§6.0)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2661-6.0-1, so no unit is bound to it.

### [`RFC2661-6.3-1`](#rfc2661-6.3-1)

The following AVP MUST be present in the SCCCN: (§6.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661MandatoryAVPSetsParsed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L376) | unit/verify | revert, verified |
| positive | [`TestRFC2661MandatoryAVPSetsEmitted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L285) | unit/verify | revert, verified |

### [`RFC2661-6.4-1`](#rfc2661-6.4-1)

The following AVPs MUST be present in the StopCCN: (§6.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661MandatoryAVPSetsParsed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L379) | unit/verify | revert, verified |
| positive | [`TestRFC2661MandatoryAVPSetsEmitted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L287) | unit/verify | revert, verified |

### [`RFC2661-6.5-1`](#rfc2661-6.5-1)

A peer MUST NOT expect HELLO messages at any time or interval (§6.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestDeadPeerZLBAckKeepsTunnelUp`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reactor_test.go#L1317) | unit/verify | revert, verified |

### [`RFC2661-6.5-2`](#rfc2661-6.5-2)

The Session ID in a HELLO message MUST be 0 (§6.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2661HelloShape`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L489) | unit/verify | revert, verified |

### [`RFC2661-6.5-3`](#rfc2661-6.5-3)

The Following AVP MUST be present in the HELLO message: (§6.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2661HelloShape`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L491) | unit/verify | revert, verified |

### [`RFC2661-6.6-1`](#rfc2661-6.6-1)

The following AVPs MUST be present in the ICRQ: (§6.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661MandatoryAVPSetsParsed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L382) | unit/verify | revert, verified |
| positive | [`TestRFC2661MandatoryAVPSetsEmitted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L289) | unit/verify | revert, verified |

### [`RFC2661-6.7-1`](#rfc2661-6.7-1)

The following AVPs MUST be present in the ICRP: (§6.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661MandatoryAVPSetsParsed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L386) | unit/verify | revert, verified |
| positive | [`TestRFC2661MandatoryAVPSetsEmitted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L291) | unit/verify | revert, verified |

### [`RFC2661-6.8-1`](#rfc2661-6.8-1)

The following AVPs MUST be present in the ICCN: (§6.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661MandatoryAVPSetsParsed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L389) | unit/verify | revert, verified |
| positive | [`TestRFC2661MandatoryAVPSetsEmitted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L293) | unit/verify | revert, verified |

### [`RFC2661-6.9-1`](#rfc2661-6.9-1)

An LNS MUST have received a Bearer Capabilities AVP during tunnel establishment from an LAC in order to request an outgoing call to that LAC (§6.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661CallCapabilitiesChecked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L421) | unit/verify | revert, verified |
| positive | [`TestRFC2661CallCapabilitiesChecked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L419) | unit/verify | revert, verified |

### [`RFC2661-6.9-2`](#rfc2661-6.9-2)

The following AVPs MUST be present in the OCRQ: (§6.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661MandatoryAVPSetsParsed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L393) | unit/verify | revert, verified |
| positive | [`TestRFC2661MandatoryAVPSetsEmitted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L295) | unit/verify | revert, verified |

### [`RFC2661-6.10-1`](#rfc2661-6.10-1)

The following AVPs MUST be present in the OCRP: (§6.10)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661MandatoryAVPSetsParsed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L396) | unit/verify | revert, verified |
| positive | [`TestRFC2661MandatoryAVPSetsEmitted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L297) | unit/verify | revert, verified |

### [`RFC2661-6.11-1`](#rfc2661-6.11-1)

The following AVPs MUST be present in the OCCN: (§6.11)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661MandatoryAVPSetsParsed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L399) | unit/verify | revert, verified |
| positive | [`TestRFC2661MandatoryAVPSetsParsed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L402) | unit/verify | revert, verified |

### [`RFC2661-6.12-1`](#rfc2661-6.12-1)

The peer MUST clean up any resources, and does not send back any indication of success or failure for such cleanup (§6.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661CDNCleansUpSilently`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L704) | unit/verify | revert, verified |
| positive | [`TestRFC2661CDNCleansUpSilently`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L701) | unit/verify | revert, verified |

### [`RFC2661-6.12-2`](#rfc2661-6.12-2)

The following AVPs MUST be present in the CDN: (§6.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661MandatoryAVPSetsParsed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L404) | unit/verify | revert, verified |
| positive | [`TestRFC2661MandatoryAVPSetsEmitted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L299) | unit/verify | revert, verified |

### [`RFC2661-6.13-1`](#rfc2661-6.13-1)

The following AVPs MUST be present in the WEN: (§6.13)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661MandatoryAVPSetsParsed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L407) | unit/verify | revert, verified |
| positive | [`TestRFC2661MandatoryAVPSetsParsed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L409) | unit/verify | revert, verified |

### [`RFC2661-6.14-1`](#rfc2661-6.14-1)

These options can change at any time during the life of the call, thus the LAC MUST be able to update its internal call information and behavior on an active PPP session (§6.14)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661SLIUpdatesACCM`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L768) | unit/verify | revert, verified |
| positive | [`TestRFC2661SLIUpdatesACCM`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L766) | unit/verify | revert, verified |

### [`RFC2661-6.14-2`](#rfc2661-6.14-2)

The following AVPs MUST be present in the SLI: (§6.14)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661MandatoryAVPSetsParsed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L411) | unit/verify | revert, verified |
| positive | [`TestRFC2661MandatoryAVPSetsParsed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L413) | unit/verify | revert, verified |

### [`RFC2661-7.2.1-1`](#rfc2661-7.2.1-1)

If the version is earlier and not supported, a StopCCN MUST be sent to the peer and the originator cleans up and terminates the tunnel (§7.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSCCRQAtTheSupportedProtocolVersionEstablishes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/rfc2661_protocol_version_test.go#L93) | unit/verify | revert, verified |
| positive | [`TestSCCRQAtAnUnsupportedProtocolVersionIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/rfc2661_protocol_version_test.go#L37) | unit/verify | revert, verified |

### [`RFC2661-7.2.1-2`](#rfc2661-7.2.1-2)

In the event of a local termination, the originator MUST send a Stop-Control-Connection-Notification and clean up the tunnel (§7.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661LocalTerminationSendsStopCCN`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L539) | unit/verify | revert, verified |
| positive | [`TestRFC2661LocalTerminationSendsStopCCN`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L536) | unit/verify | revert, verified |

### [`RFC2661-7.2.1-3`](#rfc2661-7.2.1-3)

If the originator receives a Stop-Control-Connection-Notification it MUST also clean up the tunnel (§7.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661OriginatorCleansUpOnStopCCN`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L575) | unit/verify | revert, verified |
| positive | [`TestRFC2661OriginatorCleansUpOnStopCCN`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L573) | unit/verify | revert, verified |

### [`RFC2661-7.5-1`](#rfc2661-7.5-1)

The LAC MUST respond to the Outgoing-Call-Request message with an Outgoing-Call-Reply message once the LAC determines that the proper facilities exist to place the call and the call is administratively authorized (§7.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661OCRQAnsweredWithOCRP`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L798) | unit/verify | revert, verified |
| positive | [`TestRFC2661OCRQAnsweredWithOCRP`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L795) | unit/verify | revert, verified |

### [`RFC2661-7.5.1-1`](#rfc2661-7.5.1-1)

established If a Call-Disconnect-Notify is received by the LAC, the telco call MUST be released via appropriate mechanisms and the session cleaned up (§7.5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661CDNCleansUpSilently`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L709) | unit/verify | revert, verified |
| positive | [`TestRFC2661CDNCleansUpSilently`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L706) | unit/verify | revert, verified |

### [`RFC2661-7.5.1-2`](#rfc2661-7.5.1-2)

If the call is disconnected by the client or the called interface, a Call-Disconnect-Notify message MUST be sent to the LNS (§7.5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2661LACSendsCDNOnDisconnect`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L742) | unit/verify | revert, verified |

### [`RFC2661-8.1-1`](#rfc2661-8.1-1)

Once the source and destination ports and addresses are established, they MUST remain static for the life of the tunnel (§8.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2661LocalTerminationSendsStopCCN`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L541) | unit/verify | revert, verified |

### [`RFC2661-8.1-2`](#rfc2661-8.1-2)

The default for any L2TP implementation is that UDP checksums MUST be enabled for both control and data messages (§8.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestControlSocketKeepsUDPChecksums`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/lower_layer_rfc2661_linux_test.go#L167) | unit/verify | revert, verified |

### [`RFC2661-8.1-3`](#rfc2661-8.1-3)

An L2TP implementation running on a system which does not support L2F MUST silently discard all L2F packets (§8.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2661KnownVersionAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L116) | unit/verify | revert, verified |
| positive | [`TestRFC2661UnknownVersionRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/message_rfc2661_test.go#L100) | unit/verify | revert, verified |

### [`RFC2661-8.2-1`](#rfc2661-8.2-1)

When operating in IP environments, L2TP MUST offer the UDP encapsulation described in 8.1 as its default configuration for IP operation (§8.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2661UDPEncapsulationOffered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_rfc2661_test.go#L829) | unit/verify | revert, verified |

### [`RFC2661-9.5-2`](#rfc2661-9.5-2)

If the LNS chooses to implement proxy authentication, it MUST be able to be configured off, requiring a new round a PPP authentication initiated by the LNS (which may or may not include a new round of LCP negotiation) (§9.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2661-9.5-2, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc2661.txt |
| Source fingerprint | 5e39a4a0368bb9a6 |
| Record | rfc/extraction/rfc2661.json |
| Mapped sentences | 94 |
| Declined as scope | 49 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1.0` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `2.0` | not stated | 0 | walked | not stated |
| `3.0` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 8 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `4.0` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 5 | walked | not stated |
| `4.2` | not stated | 1 | walked | not stated |
| `4.3` | not stated | 2 | walked | not stated |
| `4.4` | not stated | 0 | walked | not stated |
| `4.4.1` | not stated | 7 | walked | not stated |
| `4.4.2` | not stated | 3 | walked | not stated |
| `4.4.3` | not stated | 25 | walked | not stated |
| `4.4.4` | not stated | 22 | walked | not stated |
| `4.4.5` | not stated | 14 | walked | not stated |
| `4.4.6` | not stated | 3 | walked | not stated |
| `5.0` | not stated | 2 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
| `5.1.1` | not stated | 3 | walked | not stated |
| `5.2` | not stated | 0 | walked | not stated |
| `5.2.1` | not stated | 0 | walked | not stated |
| `5.2.2` | not stated | 0 | walked | not stated |
| `5.3` | not stated | 3 | walked | not stated |
| `5.4` | not stated | 3 | walked | not stated |
| `5.5` | not stated | 0 | walked | not stated |
| `5.6` | not stated | 0 | walked | not stated |
| `5.7` | not stated | 1 | walked | not stated |
| `5.8` | not stated | 8 | walked | not stated |
| `6.0` | not stated | 1 | walked | not stated |
| `6.1` | not stated | 1 | walked | not stated |
| `6.2` | not stated | 1 | walked | not stated |
| `6.3` | not stated | 1 | walked | not stated |
| `6.4` | not stated | 1 | walked | not stated |
| `6.5` | not stated | 3 | walked | not stated |
| `6.6` | not stated | 1 | walked | not stated |
| `6.7` | not stated | 1 | walked | not stated |
| `6.8` | not stated | 1 | walked | not stated |
| `6.9` | not stated | 2 | walked | not stated |
| `6.10` | not stated | 1 | walked | not stated |
| `6.11` | not stated | 1 | walked | not stated |
| `6.12` | not stated | 2 | walked | not stated |
| `6.13` | not stated | 1 | walked | not stated |
| `6.14` | not stated | 2 | walked | not stated |
| `7.0` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 1 | walked | not stated |
| `7.2` | not stated | 0 | walked | not stated |
| `7.2.1` | not stated | 3 | walked | not stated |
| `7.3` | not stated | 0 | walked | not stated |
| `7.4` | not stated | 0 | walked | not stated |
| `7.4.1` | not stated | 0 | walked | not stated |
| `7.4.2` | not stated | 0 | walked | not stated |
| `7.5` | not stated | 1 | walked | not stated |
| `7.5.1` | not stated | 2 | walked | not stated |
| `7.5.2` | not stated | 0 | walked | not stated |
| `7.6` | not stated | 0 | walked | not stated |
| `8.0` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 3 | walked | not stated |
| `8.2` | not stated | 1 | walked | not stated |
| `9.0` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 1 | walked | not stated |
| `9.2` | not stated | 0 | walked | not stated |
| `9.3` | not stated | 0 | walked | not stated |
| `9.4` | not stated | 0 | walked | not stated |
| `9.5` | not stated | 1 | walked | not stated |
| `10.0` | not stated | 0 | walked | not stated |
| `10.1` | not stated | 0 | walked | not stated |
| `10.2` | not stated | 0 | walked | not stated |
| `10.3` | not stated | 0 | walked | not stated |
| `10.3.1` | not stated | 0 | walked | not stated |
| `10.3.2` | not stated | 0 | walked | not stated |
| `10.4` | not stated | 0 | walked | not stated |
| `10.5` | not stated | 0 | walked | not stated |
| `10.6` | not stated | 0 | walked | not stated |
| `11.0` | not stated | 0 | walked | not stated |
| `12.0` | not stated | 0 | walked | not stated |
| `13.0` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `4.1:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the reserved-bit handling the same paragraph states, which RFC2661-4.1-1 carries: a reserved bit set to 1 makes the AVP unrecognized. | An AVP received with a reserved bit set to 1 MUST be treated as an unrecognized AVP. |
| `4.4.1:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the Random Vector ordering Section 4.3 states, which RFC2661-4.3-1 carries: the Random Vector AVP precedes the first AVP with the H bit set. | This AVP MUST precede the first AVP with the H bit set. |
| `4.4.1:7` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "This AVP MUST NOT be hidden (the H-bit MUST be 0)." once per AVP that may not be hidden; RFC2661-4.4-3 carries that obligation. | This AVP MUST NOT be hidden (the H-bit MUST be 0). |
| `4.4.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "This AVP MUST NOT be hidden (the H-bit MUST be 0)." once per AVP that may not be hidden; RFC2661-4.4-3 carries that obligation. | This AVP MUST NOT be hidden (the H-bit MUST be 0). |
| `4.4.2:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 1." once per mandatory AVP definition; RFC2661-4.4-1 carries that obligation. | The M-bit for this AVP MUST be set to 1. |
| `4.4.3:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "This AVP MUST NOT be hidden (the H-bit MUST be 0)." once per AVP that may not be hidden; RFC2661-4.4-3 carries that obligation. | This AVP MUST NOT be hidden (the H-bit MUST be 0). |
| `4.4.3:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 1." once per mandatory AVP definition; RFC2661-4.4-1 carries that obligation. | The M-bit for this AVP MUST be set to 1. |
| `4.4.3:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 1." once per mandatory AVP definition; RFC2661-4.4-1 carries that obligation. | The M-bit for this AVP MUST be set to 1. |
| `4.4.3:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 1." once per mandatory AVP definition; RFC2661-4.4-1 carries that obligation. | The M-bit for this AVP MUST be set to 1. |
| `4.4.3:9` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "This AVP MUST NOT be hidden (the H-bit MUST be 0)." once per AVP that may not be hidden; RFC2661-4.4-3 carries that obligation. | This AVP MUST NOT be hidden (the H-bit MUST be 0). |
| `4.4.3:11` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 0." once per optional AVP definition; RFC2661-4.4-2 carries that obligation. | The M-bit for this AVP MUST be set to 0. |
| `4.4.3:13` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "This AVP MUST NOT be hidden (the H-bit MUST be 0)." once per AVP that may not be hidden; RFC2661-4.4-3 carries that obligation. | This AVP MUST NOT be hidden (the H-bit MUST be 0). |
| `4.4.3:14` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 1." once per mandatory AVP definition; RFC2661-4.4-1 carries that obligation. | The M-bit for this AVP MUST be set to 1. |
| `4.4.3:15` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the UTF-8 Default Language obligation for human readable text that RFC2661-4.4.2-1 carries. | Human readable text for this AVP MUST be provided in the UTF-8 charset using the Default Language [RFC2277]. |
| `4.4.3:16` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 0." once per optional AVP definition; RFC2661-4.4-2 carries that obligation. | The M-bit for this AVP MUST be set to 0. |
| `4.4.3:20` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 1." once per mandatory AVP definition; RFC2661-4.4-1 carries that obligation. | The M-bit for this AVP MUST be set to 1. |
| `4.4.3:21` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "This AVP MUST NOT be hidden (the H-bit MUST be 0)." once per AVP that may not be hidden; RFC2661-4.4-3 carries that obligation. | This AVP MUST NOT be hidden (the H-bit MUST be 0). |
| `4.4.3:22` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 1." once per mandatory AVP definition; RFC2661-4.4-1 carries that obligation. | The M-bit for this AVP MUST be set to 1. |
| `4.4.3:23` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 1." once per mandatory AVP definition; RFC2661-4.4-1 carries that obligation. | The M-bit for this AVP MUST be set to 1. |
| `4.4.3:25` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 1." once per mandatory AVP definition; RFC2661-4.4-1 carries that obligation. | The M-bit for this AVP MUST be set to 1. |
| `4.4.4:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "This AVP MUST NOT be hidden (the H-bit MUST be 0)." once per AVP that may not be hidden; RFC2661-4.4-3 carries that obligation. | This AVP MUST NOT be hidden (the H-bit MUST be 0). |
| `4.4.4:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 1." once per mandatory AVP definition; RFC2661-4.4-1 carries that obligation. | The M-bit for this AVP MUST be set to 1. |
| `4.4.4:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 1." once per mandatory AVP definition; RFC2661-4.4-1 carries that obligation. | The M-bit for this AVP MUST be set to 1. |
| `4.4.4:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 1." once per mandatory AVP definition; RFC2661-4.4-1 carries that obligation. | The M-bit for this AVP MUST be set to 1. |
| `4.4.4:7` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 1." once per mandatory AVP definition; RFC2661-4.4-1 carries that obligation. | The M-bit for this AVP MUST be set to 1. |
| `4.4.4:8` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 1." once per mandatory AVP definition; RFC2661-4.4-1 carries that obligation. | The M-bit for this AVP MUST be set to 1. |
| `4.4.4:10` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 1." once per mandatory AVP definition; RFC2661-4.4-1 carries that obligation. | The M-bit for this AVP MUST be set to 1. |
| `4.4.4:12` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 1." once per mandatory AVP definition; RFC2661-4.4-1 carries that obligation. | The M-bit for this AVP MUST be set to 1. |
| `4.4.4:13` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 1." once per mandatory AVP definition; RFC2661-4.4-1 carries that obligation. | The M-bit for this AVP MUST be set to 1. |
| `4.4.4:14` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 1." once per mandatory AVP definition; RFC2661-4.4-1 carries that obligation. | The M-bit for this AVP MUST be set to 1. |
| `4.4.4:15` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 1." once per mandatory AVP definition; RFC2661-4.4-1 carries that obligation. | The M-bit for this AVP MUST be set to 1. |
| `4.4.4:16` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 1." once per mandatory AVP definition; RFC2661-4.4-1 carries that obligation. | The M-bit for this AVP MUST be set to 1. |
| `4.4.4:17` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 0." once per optional AVP definition; RFC2661-4.4-2 carries that obligation. | The M-bit for this AVP MUST be set to 0. |
| `4.4.4:18` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 0." once per optional AVP definition; RFC2661-4.4-2 carries that obligation. | The M-bit for this AVP MUST be set to 0. |
| `4.4.4:19` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 0." once per optional AVP definition; RFC2661-4.4-2 carries that obligation. | The M-bit for this AVP MUST be set to 0. |
| `4.4.4:21` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "This AVP MUST NOT be hidden (the H-bit MUST be 0)." once per AVP that may not be hidden; RFC2661-4.4-3 carries that obligation. | This AVP MUST NOT be hidden (the H-bit MUST be 0). |
| `4.4.4:22` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 1." once per mandatory AVP definition; RFC2661-4.4-1 carries that obligation. | The M-bit for this AVP MUST be set to 1. |
| `4.4.5:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 0." once per optional AVP definition; RFC2661-4.4-2 carries that obligation. | The M-bit for this AVP MUST be set to 0. |
| `4.4.5:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 0." once per optional AVP definition; RFC2661-4.4-2 carries that obligation. | The M-bit for this AVP MUST be set to 0. |
| `4.4.5:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 0." once per optional AVP definition; RFC2661-4.4-2 carries that obligation. | The M-bit for this AVP MUST be set to 0. |
| `4.4.5:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 0." once per optional AVP definition; RFC2661-4.4-2 carries that obligation. | The M-bit for this AVP MUST be set to 0. |
| `4.4.5:7` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 0." once per optional AVP definition; RFC2661-4.4-2 carries that obligation. | The M-bit for this AVP MUST be set to 0. |
| `4.4.5:9` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 0." once per optional AVP definition; RFC2661-4.4-2 carries that obligation. | The M-bit for this AVP MUST be set to 0. |
| `4.4.5:12` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 0." once per optional AVP definition; RFC2661-4.4-2 carries that obligation. | The M-bit for this AVP MUST be set to 0. |
| `4.4.5:14` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 0." once per optional AVP definition; RFC2661-4.4-2 carries that obligation. | The M-bit for this AVP MUST be set to 0. |
| `4.4.6:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 1." once per mandatory AVP definition; RFC2661-4.4-1 carries that obligation. | The M-bit for this AVP MUST be set to 1. |
| `4.4.6:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.4 repeats the sentence "The M-bit for this AVP MUST be set to 1." once per mandatory AVP definition; RFC2661-4.4-1 carries that obligation. | The M-bit for this AVP MUST be set to 1. |
| `7.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A reading instruction for Section 7.1, not an obligation on the wire: it tells the reader that the section's tolerant receive handling is not permission to send malformed AVPs. No sender or receiver behavior is constrained by it. | This MUST NOT be considered a license to send malformed AVPs, but simply a guide towards how to handle an improperly formatted message if one is received. |
| `9.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the shared-secret requirement for tunnel authentication that Section 5.1.1 states and RFC2661-5.1.1-3 carries. | For authentication to occur, the LAC and LNS MUST share a single secret. |

## Superseded

No document obsoletes RFC 2661, so its obligations are stated where they were written.
