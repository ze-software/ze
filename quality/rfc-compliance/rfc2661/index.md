# RFC 2661 - Layer Two Tunneling Protocol "L2TP"

Partial. Every requirement this repository extracted from RFC 2661, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 77.8% | 14 of 18 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 16.7% | 3 of 18 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 18 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 42 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 18 | of 25 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 18 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 5.6% | 1 of 18 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 18 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 25 |
| Gated MUST-level | 18 |
| Obligations that bind Ze | 18 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 42 |
| Tagged units | 42 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc2661.md` |
| Requirement shard | `rfc/requirements/rfc2661.md` |
| RFC text | `rfc/full/rfc2661.txt` |

## Enrolment

Enrolled: Layer Two Tunneling Protocol / L2TP (RFC 2661): LAC+LNS control plane. 14 MET (AVP format+reserved bits, control-message framing, ICRQ/ICRP session setup, SCCRQ/SCCRP/SCCCN tunnel handshake, unknown-mandatory-AVP teardown, reliable transport max-attempts/retransmit/duplicate-detect/window/post-teardown-ack retention, StopCCN cascade, CDN drop, SID boundary) + 3 single-polarity positive (retransmit backoff schedule, backoff cap >= 8s, control-header reserved bits zero) + 1 gap (hidden-AVP MD5 codec present but not wired: no Random Vector emitted, no H=1 encoder)

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- LNS/LAC tunnel lifecycle (answerer and **initiator**: ze dials SCCRQ, verifies SCCRP, sends SCCCN), AVP codec, hidden-AVP MD5 codec (present but not wired into message encode/decode), challenge/response, reliable control channel, HELLO, StopCCN, data sessions, **LNS-side outgoing call (OCRQ/OCRP/OCCN) via `request l2tp outgoing-call`**, dial-target config, LAC PPPoE→L2TP relay (control plane). <!-- source: [`internal/component/l2tp/tunnel_initiator.go`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_initiator.go) -- initiate/handleSCCRP
- [`internal/component/l2tp/session_initiator.go`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_initiator.go) -- placeOutgoingCall/handleOCRP -->


**What the ledger says remains**

Feature remains Partial; see L2TP guide for operational limits. One MUST gap gated in [`rfc/short/rfc2661.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc2661.md): [`RFC2661-4.3-1`](#rfc2661-4.3-1) -- the hidden-AVP MD5 cipher ([`internal/component/l2tp/hidden.go`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/hidden.go)) is implemented and unit-tested but not wired into any message encoder/decoder, so no Random Vector precedes a hidden AVP on send and precedence is not enforced on receive. Initiator tunnel interop proven vs xl2tpd (test/interop-l2tp/scenarios/03). LAC data-plane bridge (A-4) is QEMU/CAP_NET_ADMIN-gated.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 14 | one part of the gated population |
| Annotated instead of tested | 4 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **18** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (14):** [`RFC2661-4.1-1`](#rfc2661-4.1-1), [`RFC2661-4.1-2`](#rfc2661-4.1-2), [`RFC2661-4.1-3`](#rfc2661-4.1-3), [`RFC2661-4.1-4`](#rfc2661-4.1-4), [`RFC2661-5.8-3`](#rfc2661-5.8-3), [`RFC2661-5.8-4`](#rfc2661-5.8-4), [`RFC2661-5.8-5`](#rfc2661-5.8-5), [`RFC2661-5.8-6`](#rfc2661-5.8-6), [`RFC2661-5.8-7`](#rfc2661-5.8-7), [`RFC2661-24.10-1`](#rfc2661-24.10-1), [`RFC2661-24.12-1`](#rfc2661-24.12-1), [`RFC2661-10-1`](#rfc2661-10-1), [`RFC2661-9-1`](#rfc2661-9-1), [`RFC2661-10-2`](#rfc2661-10-2)

**Annotated instead of tested (4):** [`RFC2661-x-1`](#rfc2661-x-1), [`RFC2661-5.8-1`](#rfc2661-5.8-1), [`RFC2661-5.8-2`](#rfc2661-5.8-2), [`RFC2661-4.3-1`](#rfc2661-4.3-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2661-x-1` | Reserved bits 8-11 in L2TP header MUST be 0 (Wire Format) | MUST | x | **positive:** `unit/verify` [`TestWriteControlHeader`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/header_test.go#L230). **negative:** no negative test. **positive:** `functional/verify` [`rfc2661-emitted-control-shape.ci`](https://github.com/ze-software/ze/blob/main/test/l2tp/rfc2661-emitted-control-shape.ci#L27). **{single-polarity}:** every control header is the fixed constant 0xC802 and every data header is built from verL2TP\|flags, so reserved bits are always emitted zero, and ze never rejects non-zero on receive (internal/component/l2tp/header.go:26, :148-155, :172-207) |
| `RFC2661-4.1-1` | AVP reserved bits 2-5 MUST be zero on send; non-zero on receive means treat AVP as unrecognized (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestAVPCatalogRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/avp_test.go#L180). **negative:** `unit/verify` [`TestAVPIteratorReservedBits`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/avp_test.go#L107). **positive:** `functional/verify` [`rfc2661-emitted-control-shape.ci`](https://github.com/ze-software/ze/blob/main/test/l2tp/rfc2661-emitted-control-shape.ci#L29) |
| `RFC2661-4.1-2` | Message Type AVP (type 0) MUST be the first AVP in every control message. RFC 2661 Section 4.4.1: "The Message Type AVP MUST be the first AVP in a message, immediately following the control message header". The id anchor below is frozen and does NOT name Section 4.1, which is AVP Format (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestWriteICRPBody`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L940). **negative:** `unit/verify` [`TestReactor_MalformedSCCRQCreatesNoTunnel`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reactor_test.go#L256). **positive:** `functional/verify` [`rfc2661-emitted-control-shape.ci`](https://github.com/ze-software/ze/blob/main/test/l2tp/rfc2661-emitted-control-shape.ci#L24) |
| `RFC2661-4.1-3` | If M=1 and AVP is unrecognized and session-scoped, send CDN and tear down session (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestSession_IncomingLNS_ICRQ`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L121). **negative:** `unit/verify` [`TestSession_UnknownMandatoryAVP`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L554) |
| `RFC2661-4.1-4` | If M=1 and AVP is unrecognized and tunnel-scoped, send StopCCN and tear down tunnel (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestTunnelInitiatorHandshake`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_initiator_test.go#L132). **negative:** `unit/verify` [`TestTunnelSCCCNUnknownMandatoryAVP_StopCCN`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_initiator_test.go#L251) |
| `RFC2661-5.8-1` | Retransmission of control messages MUST use exponential backoff (§5.8) | MUST | 5.8 | **positive:** `unit/verify` [`TestTickBackoffSchedule`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_test.go#L374). **negative:** no negative test. **{single-polarity}:** the engine doubles the retransmit timeout on each expiry, and exponential growth is a positive behavior with no meaningful negation on a correct implementation (internal/component/l2tp/reliable.go:591-595) |
| `RFC2661-5.8-2` | Backoff cap MUST be at least 8 seconds (§5.8) | MUST | 5.8 | **positive:** `unit/verify` [`TestBackoffCapAtLeast8Seconds`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_seq_test.go#L94). **negative:** no negative test. **{single-polarity}:** the default backoff cap is the 16s constant and the reactor always constructs engines with RTimeoutCap unset, so the cap is always at least 8s with no config path or floor guard to test negatively (internal/component/l2tp/reliable_seq.go:19, reliable.go:291-292) |
| `RFC2661-5.8-3` | After exhausting retransmissions without response, tunnel and all sessions MUST be cleared (§5.8) | MUST | 5.8 | **positive:** `unit/verify` [`TestPeerTeardownWithdrawsSubscriberRoute`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reactor_test.go#L1204). **positive:** `unit/verify` [`TestTickMaxAttempts`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_test.go#L414). **negative:** `unit/verify` [`TestTickMaxAttempts`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_test.go#L417) |
| `RFC2661-5.8-4` | On each retransmit, Ns stays the same but Nr MUST be updated to current next-expected value (§5.8) | MUST | 5.8 | **positive:** `unit/verify` [`TestTickRetransmit`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_test.go#L339). **negative:** `unit/verify` [`TestTickRetransmit`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_test.go#L342) |
| `RFC2661-5.8-5` | Duplicate control messages MUST be acknowledged (via ZLB or piggyback) even though not processed by upper layer (§5.8) | MUST | 5.8 | **positive:** `unit/verify` [`TestOnReceiveDuplicate`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_test.go#L146). **negative:** `unit/verify` [`TestOnReceiveDuplicate`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_test.go#L148) |
| `RFC2661-5.8-6` | Implementations MUST accept a peer Receive Window Size of up to 4 (§5.8) | MUST | 5.8 | **positive:** `unit/verify` [`TestWindowAvailable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_window_test.go#L183). **negative:** `unit/verify` [`TestWindowPeerRWSZero`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_window_test.go#L112) |
| `RFC2661-5.8-7` | State and reliable delivery mechanisms MUST be maintained for the full retransmission interval after the final message exchange (§5.8) | MUST | 5.8 | **positive:** `unit/verify` [`TestExpired`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_test.go#L560). **positive:** `unit/verify` [`TestPostTeardownAckRetention`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_integration_test.go#L191). **negative:** `unit/verify` [`TestExpired`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_test.go#L563). **negative:** `unit/verify` [`TestPostTeardownAckRetention`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_integration_test.go#L195) |
| `RFC2661-4.3-1` | A Random Vector AVP (type 36) MUST precede any hidden AVP (H=1) in the same message (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the hidden-AVP MD5 cipher is implemented and unit-tested but is not wired into any control-message path -- no encoder sets H=1 or emits a Random Vector, and decoders skip hidden AVPs without decrypting or checking precedence (internal/component/l2tp/hidden.go:38 has no production caller; avp.go:156-168 skips hidden AVPs; AVPRandomVector avp.go:56 never emitted) |
| `RFC2661-24.10-1` | Assigned Tunnel ID of 0 in SCCRQ/SCCRP is a protocol error; reject with StopCCN. RFC 2661 Section 4.4.3: "The Assigned Tunnel ID is a 2 octet non-zero unsigned integer"; RFC 2661 Section 5.3: the value 0 "MUST NOT be used as an Assigned Session ID or Assigned Tunnel ID". The id anchor below numbers no section of RFC 2661 and is frozen (§24.10) | MUST | 24.10 | **positive:** `unit/verify` [`TestSCCRQWithNonZeroAssignedTunnelIDEstablishes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reactor_sccrq_zero_tid_test.go#L117). **positive:** `unit/verify` [`TestTunnelInitiatorHandshake`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_initiator_test.go#L135). **negative:** `unit/verify` [`TestParseSCCRP_Rejects`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_initiator_test.go#L107). **negative:** `unit/verify` [`TestSCCRQWithZeroAssignedTunnelIDIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reactor_sccrq_zero_tid_test.go#L75). **positive:** `functional/verify` [`rfc2661-sccrq-tunnel-id-zero.ci`](https://github.com/ze-software/ze/blob/main/test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci#L24). **negative:** `functional/verify` [`rfc2661-sccrq-tunnel-id-zero.ci`](https://github.com/ze-software/ze/blob/main/test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci#L21) |
| `RFC2661-24.12-1` | Unknown M=1 vendor AVP in a session context tears down the session with CDN, not the tunnel with StopCCN. RFC 2661 Section 4.1 states the session/tunnel split and RFC 2661 Section 4.2 states the consequence. The id anchor below numbers no section of RFC 2661 and is frozen (§24.12) | MUST | 24.12 | **positive:** `unit/verify` [`TestSession_UnknownMandatoryAVP`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L556). **negative:** `unit/verify` [`TestSession_UnknownMandatoryAVP`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L558) |
| `RFC2661-10-1` | CDN is valid in any non-idle session state; receiving CDN destroys the session. RFC 2661 Section 5.6 states session teardown by CDN and RFC 2661 Section 7.4.2 gives the state table. The id anchor below is frozen and does NOT name Section 10, which is IANA Considerations (§10) | MUST | 10 | **positive:** `unit/verify` [`TestSession_CDN_AnyState`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L337). **positive:** `unit/verify` [`TestSession_CDN_EstablishedSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L308). **negative:** `unit/verify` [`TestSession_CDN_UnknownSessionDropped`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L361) |
| `RFC2661-9-1` | StopCCN cascades: all sessions in a tunnel are cleared when StopCCN is received. RFC 2661 Section 5.7: an implementation "may shut down an entire tunnel and all sessions on the tunnel by sending the StopCCN". The id anchor below is frozen and does NOT name Section 9, which is Security Considerations (§9) | MUST | 9 | **positive:** `unit/verify` [`TestSession_StopCCN_CascadeSessions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L398). **negative:** `unit/verify` [`TestStopCCNQueuesAllTeardowns`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L809) |
| `RFC2661-10-2` | Session ID 0 is reserved and never assigned. RFC 2661 Section 5.3: "The value of 0 for Session ID and Tunnel ID is special and MUST NOT be used as an Assigned Session ID or Assigned Tunnel ID". The id anchor below is frozen and does NOT name Section 10, which is IANA Considerations (§10) | MUST | 10 | **positive:** `unit/verify` [`TestSession_SIDBoundary_MaxUint16`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L1036). **negative:** `unit/verify` [`TestSession_SIDBoundary_Zero`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L1050) |
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

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2661-x-1`](#rfc2661-x-1)

Reserved bits 8-11 in L2TP header MUST be 0 (Wire Format)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestWriteControlHeader`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/header_test.go#L230) | unit/verify | unproven |
| positive | [`rfc2661-emitted-control-shape.ci`](https://github.com/ze-software/ze/blob/main/test/l2tp/rfc2661-emitted-control-shape.ci#L27) | functional/verify | unproven |

### [`RFC2661-4.1-1`](#rfc2661-4.1-1)

AVP reserved bits 2-5 MUST be zero on send; non-zero on receive means treat AVP as unrecognized (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAVPIteratorReservedBits`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/avp_test.go#L107) | unit/verify | unproven |
| positive | [`TestAVPCatalogRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/avp_test.go#L180) | unit/verify | unproven |
| positive | [`rfc2661-emitted-control-shape.ci`](https://github.com/ze-software/ze/blob/main/test/l2tp/rfc2661-emitted-control-shape.ci#L29) | functional/verify | unproven |

### [`RFC2661-4.1-2`](#rfc2661-4.1-2)

Message Type AVP (type 0) MUST be the first AVP in every control message. RFC 2661 Section 4.4.1: "The Message Type AVP MUST be the first AVP in a message, immediately following the control message header". The id anchor below is frozen and does NOT name Section 4.1, which is AVP Format (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReactor_MalformedSCCRQCreatesNoTunnel`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reactor_test.go#L256) | unit/verify | unproven |
| positive | [`TestWriteICRPBody`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L940) | unit/verify | unproven |
| positive | [`rfc2661-emitted-control-shape.ci`](https://github.com/ze-software/ze/blob/main/test/l2tp/rfc2661-emitted-control-shape.ci#L24) | functional/verify | unproven |

### [`RFC2661-4.1-3`](#rfc2661-4.1-3)

If M=1 and AVP is unrecognized and session-scoped, send CDN and tear down session (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSession_UnknownMandatoryAVP`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L554) | unit/verify | unproven |
| positive | [`TestSession_IncomingLNS_ICRQ`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L121) | unit/verify | unproven |

### [`RFC2661-4.1-4`](#rfc2661-4.1-4)

If M=1 and AVP is unrecognized and tunnel-scoped, send StopCCN and tear down tunnel (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTunnelSCCCNUnknownMandatoryAVP_StopCCN`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_initiator_test.go#L251) | unit/verify | unproven |
| positive | [`TestTunnelInitiatorHandshake`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_initiator_test.go#L132) | unit/verify | unproven |

### [`RFC2661-5.8-1`](#rfc2661-5.8-1)

Retransmission of control messages MUST use exponential backoff (§5.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestTickBackoffSchedule`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_test.go#L374) | unit/verify | unproven |

### [`RFC2661-5.8-2`](#rfc2661-5.8-2)

Backoff cap MUST be at least 8 seconds (§5.8)

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

On each retransmit, Ns stays the same but Nr MUST be updated to current next-expected value (§5.8)

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

Implementations MUST accept a peer Receive Window Size of up to 4 (§5.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestWindowPeerRWSZero`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_window_test.go#L112) | unit/verify | unproven |
| positive | [`TestWindowAvailable`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reliable_window_test.go#L183) | unit/verify | unproven |

### [`RFC2661-5.8-7`](#rfc2661-5.8-7)

State and reliable delivery mechanisms MUST be maintained for the full retransmission interval after the final message exchange (§5.8)

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

### [`RFC2661-24.10-1`](#rfc2661-24.10-1)

Assigned Tunnel ID of 0 in SCCRQ/SCCRP is a protocol error; reject with StopCCN. RFC 2661 Section 4.4.3: "The Assigned Tunnel ID is a 2 octet non-zero unsigned integer"; RFC 2661 Section 5.3: the value 0 "MUST NOT be used as an Assigned Session ID or Assigned Tunnel ID". The id anchor below numbers no section of RFC 2661 and is frozen (§24.10)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSCCRQWithZeroAssignedTunnelIDIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reactor_sccrq_zero_tid_test.go#L75) | unit/verify | unproven |
| negative | [`TestParseSCCRP_Rejects`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_initiator_test.go#L107) | unit/verify | unproven |
| negative | [`rfc2661-sccrq-tunnel-id-zero.ci`](https://github.com/ze-software/ze/blob/main/test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci#L21) | functional/verify | unproven |
| positive | [`TestSCCRQWithNonZeroAssignedTunnelIDEstablishes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/reactor_sccrq_zero_tid_test.go#L117) | unit/verify | unproven |
| positive | [`TestTunnelInitiatorHandshake`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/tunnel_initiator_test.go#L135) | unit/verify | unproven |
| positive | [`rfc2661-sccrq-tunnel-id-zero.ci`](https://github.com/ze-software/ze/blob/main/test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci#L24) | functional/verify | unproven |

### [`RFC2661-24.12-1`](#rfc2661-24.12-1)

Unknown M=1 vendor AVP in a session context tears down the session with CDN, not the tunnel with StopCCN. RFC 2661 Section 4.1 states the session/tunnel split and RFC 2661 Section 4.2 states the consequence. The id anchor below numbers no section of RFC 2661 and is frozen (§24.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSession_UnknownMandatoryAVP`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L558) | unit/verify | unproven |
| positive | [`TestSession_UnknownMandatoryAVP`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L556) | unit/verify | unproven |

### [`RFC2661-10-1`](#rfc2661-10-1)

CDN is valid in any non-idle session state; receiving CDN destroys the session. RFC 2661 Section 5.6 states session teardown by CDN and RFC 2661 Section 7.4.2 gives the state table. The id anchor below is frozen and does NOT name Section 10, which is IANA Considerations (§10)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSession_CDN_UnknownSessionDropped`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L361) | unit/verify | unproven |
| positive | [`TestSession_CDN_AnyState`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L337) | unit/verify | unproven |
| positive | [`TestSession_CDN_EstablishedSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L308) | unit/verify | unproven |

### [`RFC2661-9-1`](#rfc2661-9-1)

StopCCN cascades: all sessions in a tunnel are cleared when StopCCN is received. RFC 2661 Section 5.7: an implementation "may shut down an entire tunnel and all sessions on the tunnel by sending the StopCCN". The id anchor below is frozen and does NOT name Section 9, which is Security Considerations (§9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestStopCCNQueuesAllTeardowns`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L809) | unit/verify | unproven |
| positive | [`TestSession_StopCCN_CascadeSessions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L398) | unit/verify | unproven |

### [`RFC2661-10-2`](#rfc2661-10-2)

Session ID 0 is reserved and never assigned. RFC 2661 Section 5.3: "The value of 0 for Session ID and Tunnel ID is special and MUST NOT be used as an Assigned Session ID or Assigned Tunnel ID". The id anchor below is frozen and does NOT name Section 10, which is IANA Considerations (§10)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSession_SIDBoundary_Zero`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L1050) | unit/verify | unproven |
| positive | [`TestSession_SIDBoundary_MaxUint16`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/session_fsm_test.go#L1036) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 2661, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 2661, so its obligations are stated where they were written.
