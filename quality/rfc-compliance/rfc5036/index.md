# RFC 5036 - LDP Specification

Experimental. Every requirement this repository extracted from RFC 5036, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 26.5% | 22 of 83 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 3.6% | 3 of 83 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 83 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 72.3% | 34 of 47 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 83 | of 87 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 83 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 83 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 83 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 83 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 69.9% | 58 of 83 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 83 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 87 |
| Gated MUST-level | 83 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 59 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 47 |
| Tagged units | 47 |
| Recorded audit verdicts | 0 |
| Discrimination records | 34 |
| Summary | `rfc/short/rfc5036.md` |
| Requirement shard | `rfc/requirements/rfc5036.md` |
| RFC text | `rfc/full/rfc5036.txt` |

## Enrolment

Enrolled: LDP Specification

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered**

Basic Discovery, active-only TCP session FSM with Initialization/KeepAlive negotiation, label information base, downstream-unsolicited Label Mapping advertisement and reception, kernel MPLS integration. The 2026-09-21 walk widened the checklist to the whole document, so this list now covers a small part of the row set rather than most of it.

**What the ledger says remains**

The 2026-09-21 extraction walk read all 87 normative sites in [`rfc/full/rfc5036.txt`](https://github.com/ze-software/ze/blob/main/rfc/full/rfc5036.txt) and grew the checklist from 19 rows to 86.

- **Messages ze emits on one path only:** [`RFC5036-2.5.3-2`](#rfc5036-2.5.3-2) and [`RFC5036-3.5.1-1`](#rfc5036-3.5.1-1) (a Notification NAKs an unacceptable Initialization, and every other fatal error still closes the session silently), [`RFC5036-2.6.1.3-1`](#rfc5036-2.6.1.3-1) (no Label Release on a received withdraw), [`RFC5036-2.6.1.2-1`](#rfc5036-2.6.1.2-1) (SendLabelWithdraw at session.go has no production caller, so a local binding is never withdrawn on the wire).
- **Session checks ze omits:** [`RFC5036-2.5.1-4`](#rfc5036-2.5.1-4) (handleInit at session.go overwrites the expected peer LSR ID instead of comparing it), [`RFC5036-2.5.1-3`](#rfc5036-2.5.1-3) (the establishment KeepAlive at register.go is unconditional, not a response to an accepted Initialization). The walk corrected four rows against the document: [`RFC5036-2.9-1`](#rfc5036-2.9-1) rose to MUST, [`RFC5036-2.6.1.1-1`](#rfc5036-2.6.1.1-1) fell to MAY, [`RFC5036-3.5.1-1`](#rfc5036-3.5.1-1) fell to SHOULD, and [`RFC5036-2.7-1`](#rfc5036-2.7-1) lost a prohibition on sending labeled packets before MPLS forwarding is enabled, which no sentence of RFC 5036 states. Ze still imposes labels without that check, at ProgramPush in [`internal/plugins/ldp/fib.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/fib.go), and the walk removed the row that recorded it, so nothing on this ledger gates it now.
- **The 67 rows the walk added carry no test:** the Loop Detection procedures, the ATM and Frame Relay label and session parameters, the Downstream on Demand Label Request, Abort and Release procedures, the vendor-private U-bit configuration interfaces, and the session parameter negotiations.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 22 | one part of the gated population |
| Annotated instead of tested | 61 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **83** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (22):** [`RFC5036-x-1`](#rfc5036-x-1), [`RFC5036-x-2`](#rfc5036-x-2), [`RFC5036-x-3`](#rfc5036-x-3), [`RFC5036-2.5.1-1`](#rfc5036-2.5.1-1), [`RFC5036-2.5.3-1`](#rfc5036-2.5.3-1), [`RFC5036-2.5.1-2`](#rfc5036-2.5.1-2), [`RFC5036-3.5.3-1`](#rfc5036-3.5.3-1), [`RFC5036-3.5.2-1`](#rfc5036-3.5.2-1), [`RFC5036-2.5.3-3`](#rfc5036-2.5.3-3), [`RFC5036-2.5.3-4`](#rfc5036-2.5.3-4), [`RFC5036-2.5.2-1`](#rfc5036-2.5.2-1), [`RFC5036-2.6.1.2-2`](#rfc5036-2.6.1.2-2), [`RFC5036-3.1-1`](#rfc5036-3.1-1), [`RFC5036-3.3-1`](#rfc5036-3.3-1), [`RFC5036-3.4.4.1-1`](#rfc5036-3.4.4.1-1), [`RFC5036-3.4.4.1-2`](#rfc5036-3.4.4.1-2), [`RFC5036-3.5.3-3`](#rfc5036-3.5.3-3), [`RFC5036-3.5.3-5`](#rfc5036-3.5.3-5), [`RFC5036-3.5.3-6`](#rfc5036-3.5.3-6), [`RFC5036-3.5.3-7`](#rfc5036-3.5.3-7), [`RFC5036-3.5.3-9`](#rfc5036-3.5.3-9), [`RFC5036-3.5.4.1-1`](#rfc5036-3.5.4.1-1)

**Annotated instead of tested (61):** [`RFC5036-2.6.1.2-1`](#rfc5036-2.6.1.2-1), [`RFC5036-2.6.1.3-1`](#rfc5036-2.6.1.3-1), [`RFC5036-2.5.1-3`](#rfc5036-2.5.1-3), [`RFC5036-2.5.1-4`](#rfc5036-2.5.1-4), [`RFC5036-2.5.3-2`](#rfc5036-2.5.3-2), [`RFC5036-2.7-1`](#rfc5036-2.7-1), [`RFC5036-2.9-1`](#rfc5036-2.9-1), [`RFC5036-2.8.1-1`](#rfc5036-2.8.1-1), [`RFC5036-2.8.1-2`](#rfc5036-2.8.1-2), [`RFC5036-2.8.1-3`](#rfc5036-2.8.1-3), [`RFC5036-2.8.1-4`](#rfc5036-2.8.1-4), [`RFC5036-2.8.1-5`](#rfc5036-2.8.1-5), [`RFC5036-2.8.1-6`](#rfc5036-2.8.1-6), [`RFC5036-2.8.1-7`](#rfc5036-2.8.1-7), [`RFC5036-2.8.2-1`](#rfc5036-2.8.2-1), [`RFC5036-2.8.2-2`](#rfc5036-2.8.2-2), [`RFC5036-2.8.2-3`](#rfc5036-2.8.2-3), [`RFC5036-2.8.2-4`](#rfc5036-2.8.2-4), [`RFC5036-2.8.2-5`](#rfc5036-2.8.2-5), [`RFC5036-2.8.2-6`](#rfc5036-2.8.2-6), [`RFC5036-2.8.2-7`](#rfc5036-2.8.2-7), [`RFC5036-2.8.2-8`](#rfc5036-2.8.2-8), [`RFC5036-2.8.2-9`](#rfc5036-2.8.2-9), [`RFC5036-2.8.2-10`](#rfc5036-2.8.2-10), [`RFC5036-2.8.2-11`](#rfc5036-2.8.2-11), [`RFC5036-2.8.2-12`](#rfc5036-2.8.2-12), [`RFC5036-2.8.2-13`](#rfc5036-2.8.2-13), [`RFC5036-3.4.2.2-1`](#rfc5036-3.4.2.2-1), [`RFC5036-3.4.2.2-2`](#rfc5036-3.4.2.2-2), [`RFC5036-3.4.2.2-3`](#rfc5036-3.4.2.2-3), [`RFC5036-3.4.2.3-1`](#rfc5036-3.4.2.3-1), [`RFC5036-3.4.4.1-3`](#rfc5036-3.4.4.1-3), [`RFC5036-3.4.5.1.1-1`](#rfc5036-3.4.5.1.1-1), [`RFC5036-3.4.5.1.1-2`](#rfc5036-3.4.5.1.1-2), [`RFC5036-3.4.5.1.1-3`](#rfc5036-3.4.5.1.1-3), [`RFC5036-3.4.5.1.2-1`](#rfc5036-3.4.5.1.2-1), [`RFC5036-3.4.5.1.2-2`](#rfc5036-3.4.5.1.2-2), [`RFC5036-3.4.5.1.2-3`](#rfc5036-3.4.5.1.2-3), [`RFC5036-3.5-1`](#rfc5036-3.5-1), [`RFC5036-3.5.3-2`](#rfc5036-3.5.3-2), [`RFC5036-3.5.3-4`](#rfc5036-3.5.3-4), [`RFC5036-3.5.3-8`](#rfc5036-3.5.3-8), [`RFC5036-3.5.3-10`](#rfc5036-3.5.3-10), [`RFC5036-3.5.3-11`](#rfc5036-3.5.3-11), [`RFC5036-3.5.3-12`](#rfc5036-3.5.3-12), [`RFC5036-3.5.3-13`](#rfc5036-3.5.3-13), [`RFC5036-3.5.3-14`](#rfc5036-3.5.3-14), [`RFC5036-3.5.7-1`](#rfc5036-3.5.7-1), [`RFC5036-3.5.8.1-1`](#rfc5036-3.5.8.1-1), [`RFC5036-3.5.8.1-2`](#rfc5036-3.5.8.1-2), [`RFC5036-3.5.8.1-3`](#rfc5036-3.5.8.1-3), [`RFC5036-3.5.8.1-4`](#rfc5036-3.5.8.1-4), [`RFC5036-3.5.9.1-1`](#rfc5036-3.5.9.1-1), [`RFC5036-3.5.9.1-2`](#rfc5036-3.5.9.1-2), [`RFC5036-3.5.9.1-3`](#rfc5036-3.5.9.1-3), [`RFC5036-3.5.11.1-1`](#rfc5036-3.5.11.1-1), [`RFC5036-3.6.1.1-1`](#rfc5036-3.6.1.1-1), [`RFC5036-3.6.1.2-1`](#rfc5036-3.6.1.2-1), [`RFC5036-A.1.2-1`](#rfc5036-a.1.2-1), [`RFC5036-A.1.7-1`](#rfc5036-a.1.7-1), [`RFC5036-A.1-1`](#rfc5036-a.1-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5036-x-1` | Version field in PDU header must be 1 (Wire Format) | MUST | x | **positive:** `unit/verify` [`TestRFC5036PDUVersionOneAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L126). **negative:** `unit/verify` [`TestRFC5036PDUVersionOtherRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L153) |
| `RFC5036-x-2` | Reserved bits in Common Hello Parameters TLV must be zero (Discovery) | MUST | x | **positive:** `unit/verify` [`TestRFC5036HelloReservedBitsZeroOnTransmit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L178). **negative:** `unit/verify` [`TestRFC5036HelloReservedBitsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L200) |
| `RFC5036-x-3` | Protocol Version in Common Session Parameters must be 1 (Sessions) | MUST | x | **positive:** `unit/verify` [`TestRFC5036InitProtocolVersionOne`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L223). **negative:** `unit/verify` [`TestRFC5036InitProtocolVersionOtherRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L257) |
| `RFC5036-2.5.1-1` | An LSR MUST send the Initialization message to start a session (§2.5.1). The cited section describes session establishment and states no obligation; the sentence is in 2.5.3 and is indicative: "if LSR1 is playing the active role, it initiates negotiation of session parameters by sending an Initialization message to LSR2" | MUST | 2.5.1 | **positive:** `unit/verify` [`TestRFC5036SessionSendsInitializationFirst`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L403). **negative:** `unit/verify` [`TestRFC5036SessionNotOperationalWithoutOwnInit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L426) |
| `RFC5036-2.5.3-1` | An LSR MUST periodically send KeepAlive messages on established sessions (§2.5.3). The sentence is in 3.5.4.1: "in circumstances where no other LDP protocol messages have been sent within the period, a KeepAlive message MUST be sent" | MUST | 2.5.3 | **positive:** `unit/verify` [`TestRFC5036KeepalivesSentPeriodically`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L514). **negative:** `unit/verify` [`TestRFC5036KeepalivesNotSentContinuously`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L540) |
| `RFC5036-2.6.1.2-1` | An LSR MUST send a Label Withdraw message when a previously advertised binding is no longer valid (§2.6.1.2). The cited section is about ordered control and states no such obligation; the sentence is in Appendix A: "Whenever an LSR breaks the binding between a label and a FEC, it MUST withdraw the FEC label mapping from all LDP peers to which it has previously sent the mapping" | MUST | 2.6.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the encoder and sender exist (internal/plugins/ldp/session.go:326 SendLabelWithdraw, internal/plugins/ldp/wire.go:499 EncodeLabelWithdraw) but nothing invokes them -- a local binding is created once in OnStarted (internal/plugins/ldp/register.go:312) and released only by RemovePop at engine exit (internal/plugins/ldp/register.go:380), so no Label Withdraw ever reaches the wire |
| `RFC5036-2.6.1.3-1` | An LSR MUST send a Label Release message when it no longer needs a label (§2.6.1.3). The cited section states no such obligation; the sentence is in 3.5.10.1: "An LSR that receives a Label Withdraw message MUST respond with a Label Release message" | MUST | 2.6.1.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the withdraw handler drops the binding and reconciles forwarding without replying (internal/plugins/ldp/register.go:821 the onWithdraw callback, internal/plugins/ldp/fib.go:48 withdrawRemoteBinding); MsgTypeLabelRelease has no encoder and an inbound one is discarded at internal/plugins/ldp/session.go:463 |
| `RFC5036-2.5.1-2` | An LSR MUST accept the lower of the two proposed KeepAlive Timer values during negotiation (§2.5.1). The sentence is in 3.5.3: "The receiving LSR MUST calculate the value of the KeepAlive Timer by using the smaller of its proposed KeepAlive Time and the KeepAlive Time received in the PDU" | MUST | 2.5.1 | **positive:** `unit/verify` [`TestRFC5036KeepaliveNegotiationAdoptsLower`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L459). **negative:** `unit/verify` [`TestRFC5036KeepaliveNegotiationRefusesHigher`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L475) |
| `RFC5036-3.5.3-1` | The Common Session Parameters KeepAlive Time is a two octet unsigned NON ZERO integer, so an Initialization proposing 0 MUST be rejected with the Session Rejected/Bad KeepAlive Time Notification (0x00000018) and the session MUST NOT be established (§3.5.3, §3.9) | MUST | 3.5.3 | **positive:** `unit/verify` [`TestRFC5036InitNonZeroKeepaliveTimeAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L318). **negative:** `unit/verify` [`TestRFC5036InitZeroKeepaliveTimeRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L355) |
| `RFC5036-2.5.1-3` | An LSR MUST respond to a received Initialization with a KeepAlive message if parameters are acceptable (§2.5.1). The sentence is in 2.5.3 and is indicative: "LSR1 replies with an Initialization message of its own to propose the parameters it wishes to use and a KeepAlive message to signal acceptance of LSR2's parameters" | MUST | 2.5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handleInit (internal/plugins/ldp/session.go:504) applies the negotiated parameters and advances the FSM without emitting anything; the only establishment KeepAlive is the unconditional one at internal/plugins/ldp/register.go:758, sent right after ze's own Initialization and before the peer's arrives, so no KeepAlive is conditioned on receiving and accepting an Initialization |
| `RFC5036-3.5.2-1` | A Common Hello Parameters Hold Time of 0 means use the default hold time -- 15 seconds for Link Hellos, 45 seconds for Targeted Hellos -- and the adjacency is kept, not removed (§3.5.2) | MUST | 3.5.2 | **positive:** `unit/verify` [`TestRFC5036HelloHoldTimeZeroUsesDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L579). **negative:** `unit/verify` [`TestRFC5036HelloHoldTimeNonZeroNotDefaulted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L619) |
| `RFC5036-2.5.1-4` | The LSR MUST check that the LSR ID matches what was expected in the Initialization (§2.5.1). The cited section states no such check; 2.5.3 says the passive LSR "attempts to match the LDP Identifier carried by the message PDU with a Hello adjacency", and the enforcing MUST is in 3.5.3, recorded as RFC5036-3.5.3-9 | MUST | 2.5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handleInit overwrites the expected peer LSR ID with whatever the PDU header carried (internal/plugins/ldp/session.go:508 `s.peerLSRID = peerLSRID`) instead of comparing it to the value learned from the Hello, and the Receiver LSR ID decoded from the Common Session Parameters TLV (internal/plugins/ldp/wire.go:338) is never compared to the local LSR ID |
| `RFC5036-2.5.3-2` | An LSR MUST send a Notification message for fatal errors that require session teardown (§2.5.3) | MUST | 2.5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** partly met. An unacceptable Initialization IS now NAK'd -- rejectInit (internal/plugins/ldp/session.go) sends the Notification through encodeNotification (internal/plugins/ldp/wire.go) for Bad Protocol Version and Session Rejected/Bad KeepAlive Time -- but every other fatal error still returns the error and closes the connection with no Notification: keepalive expiry and decode failure in ReadLoop and processMessages (internal/plugins/ldp/session.go), and the session-ended log in register.go |
| `RFC5036-2.7-1` | To enable LSRs to map between a peer LDP Identifier and the peer's addresses, an LSR must be able to map the next hop address for a prefix to an LDP Identifier and an LDP Identifier to the peer's addresses, and LSRs advertise their addresses using LDP Address and Address Withdraw messages (§2.7) | MUST | 2.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze decodes Address messages but never sends one; plan/pre-release/spec-ldp-address-advertisement.md |
| `RFC5036-3.5.1-1` | After sending a Notification message whose Status Code indicates a fatal error, the LSR SHOULD terminate the LDP session by closing the session TCP connection and discard all state associated with the session, including all label-FEC bindings learned via the session. The sentence is in subsection 3.5.1.1 and states SHOULD, not MUST NOT (§3.5.1) | SHOULD | 3.5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** partly met. On the Initialization path the obligation now holds end to end -- rejectInit (internal/plugins/ldp/session.go) sends the fatal Notification and returns the cause, processMessages stops on it and ReadLoop propagates it, so no further message from that peer is processed. Every other fatal error still sends no Notification, so the obligation's trigger has no producer there; it is unmet for the same reason as RFC5036-2.5.3-2 |
| `RFC5036-2.9-1` | The use of the TCP MD5 Signature Option mechanism that protects against spoofed TCP segments in LDP session connection streams MUST be supported as a configurable option (§2.9) | MUST | 2.9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze reads no LDP password and sets no TCP_MD5SIG; plan/pre-release/spec-ldp-md5-authentication.md |
| `RFC5036-2.6.1.1-1` | Under independent LSP control an LSR may advertise label mappings to its neighbors at any time it desires, and when operating in independent Downstream Unsolicited mode it may advertise a label mapping for a FEC to its neighbors whenever it is prepared to label-switch that FEC. The cited section states no obligation to advertise labels for all FECs (§2.6.1.1) | MAY | 2.6.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5036-2.8-1` | An LSR MAY use loop detection mechanisms (hop count, path vector) (§2.8) | MAY | 2.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC5036-2.4.1-1` | An LSR MAY use Extended Discovery for non-adjacent peers (§2.4.1). The cited section covers Basic Discovery; Extended Discovery is described in 2.4.2, indicatively: "LDP sessions between non-directly connected LSRs are supported by LDP Extended Discovery" | MAY | 2.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5036-2.5.3-3` | An LSR MUST throttle its session setup retry attempts with an exponential backoff in situations where Initialization messages are being NAK'd (§2.5.3) | MUST | 2.5.3 | **positive:** `unit/verify` [`TestRFC5036SetupRetryBackoffGrows`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L322). **negative:** `unit/verify` [`TestRFC5036SetupRetryRefusedInsideDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L346) |
| `RFC5036-2.5.3-4` | The session establishment setup attempt following a NAK'd Initialization message MUST be delayed no less than 15 seconds, and subsequent delays MUST grow to a maximum delay of no less than 2 minutes (§2.5.3) | MUST | 2.5.3 | **positive:** `unit/verify` [`TestRFC5036SetupRetryBackoffGrows`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L324). **negative:** `unit/verify` [`TestRFC5036SetupRetryRefusedInsideDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L348) |
| `RFC5036-2.5.2-1` | An LSR MUST advertise the same transport address in all Hellos that advertise the same label space (§2.5.2) | MUST | 2.5.2 | **positive:** `unit/verify` [`TestRFC5036HellosCarryOneTransportAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L483). **negative:** `unit/verify` [`TestRFC5036HelloTransportAddressNeverFollowsTheSocket`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L501) |
| `RFC5036-2.6.1.2-2` | For each FEC for which the LSR is not the egress and no mapping exists, the LSR MUST wait until a label from a downstream LSR is received before mapping the FEC and passing corresponding labels to upstream LSRs. (§2.6.1.2) | MUST | 2.6.1.2 | **positive:** `unit/verify` [`TestRFC5036EgressFECMappedWithoutWaiting`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L432). **negative:** `unit/verify` [`TestRFC5036NonEgressFECNotMappedBeforeDownstreamLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L461) |
| `RFC5036-2.8.1-1` | - The Label Request message MUST include a Hop Count TLV. (§2.8.1) | MUST | 2.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.1-2` | - If R is sending the Label Request because it is a FEC ingress, it MUST include a Hop Count TLV with hop count value 1. (§2.8.1) | MUST | 2.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.1-3` | - If R is sending the Label Request as a result of having received a Label Request from an upstream LSR, and if the received Label Request contains a Hop Count TLV, R MUST increment the received hop count value by 1 and MUST pass the resulting value in a Hop Count TLV to its next hop along with the Label Request message. (§2.8.1) | MUST | 2.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.1-4` | - If R is sending the Label Request because it is a FEC ingress, then if R is non-merge capable, it MUST include a Path Vector TLV of length 1 containing its own LSR Id. (§2.8.1) | MUST | 2.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.1-5` | R MUST add its own LSR Id to the Path Vector, and MUST pass the resulting Path Vector to its next hop along with the Label Request message. (§2.8.1) | MUST | 2.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.1-6` | If the Label Request contains no Path Vector TLV, R MUST include a Path Vector TLV of length 1 containing its own LSR Id. (§2.8.1) | MUST | 2.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.1-7` | When R detects a loop, it MUST send a Loop Detected Notification message to the source of the Label Request message and drop the Label Request message. (§2.8.1) | MUST | 2.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.2-1` | If configured for Loop Detection, R MUST include a Hop Count TLV in the Label Mapping message (§2.8.2) | MUST | 2.8.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.2-2` | - If R is the egress, the hop count value MUST be 1. (§2.8.2) | MUST | 2.8.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.2-3` | If configured for Loop Detection and the Label Mapping message propagates one received from the next hop to an upstream peer, the hop count value MUST be determined by the two rules that follow it (§2.8.2) | MUST | 2.8.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.2-4` | o If R is a member of the edge set of an LSR domain whose LSRs do not perform 'TTL-decrement' (e.g., an ATM LSR domain or a Frame Relay LSR domain) and the upstream peer is within that domain, R MUST reset the hop count to 1 before propagating the message. (§2.8.2) | MUST | 2.8.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.2-5` | If configured for Loop Detection and the preceding case does not hold, R MUST increment the hop count received from the next hop before propagating the message (§2.8.2) | MUST | 2.8.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.2-6` | - If the Label Mapping message is not being sent to propagate a Label Mapping message, the hop count value MUST be the result of incrementing R's current knowledge of the hop count learned from previous Label Mapping messages. (§2.8.2) | MUST | 2.8.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.2-7` | - If R is sending the Label Mapping message to propagate a Label Mapping message received from the next hop to an upstream peer, then: o If R is merge capable and if R has not previously sent a Label Mapping message to the upstream peer, then it MUST include a Path Vector TLV. (§2.8.2) | MUST | 2.8.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.2-8` | o If the received message contains an unknown hop count, then R MUST include a Path Vector TLV. (§2.8.2) | MUST | 2.8.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.2-9` | o If R has previously sent a Label Mapping message to the upstream peer, then it MUST include a Path Vector TLV if the received message reports an LSP hop count increase, a change in hop count from unknown to known, or a change from known to unknown. (§2.8.2) | MUST | 2.8.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.2-10` | o If the received Label Mapping message included a Path Vector, the Path Vector sent upstream MUST be the result of adding R's LSR Id to the received Path Vector. (§2.8.2) | MUST | 2.8.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.2-11` | o If the received message had no Path Vector, the Path Vector sent upstream MUST be a Path Vector of length 1 containing R's LSR Id. (§2.8.2) | MUST | 2.8.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.2-12` | - If the Label Mapping message is not being sent to propagate a received message upstream, the Label Mapping message MUST include a Path Vector of length 1 containing R's LSR Id. (§2.8.2) | MUST | 2.8.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.2-13` | When R detects a loop, it MUST stop using the label for forwarding, drop the Label Mapping message, and signal Loop Detected status to the source of the Label Mapping message. (§2.8.2) | MUST | 2.8.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-3.1-1` | The first four octets identify the LSR and MUST be a globally unique value. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC5036DistinctLSRIDFormsAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L373). **negative:** `unit/verify` [`TestRFC5036OwnLSRIDFormsNoAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L391) |
| `RFC5036-3.3-1` | Upon receipt of an unknown TLV, if U is clear (=0), a notification MUST be returned to the message originator and the entire message MUST be ignored; if U is set (=1), the unknown TLV MUST be silently ignored and the rest of the message processed as if the unknown TLV did not exist. (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestRFC5036UnknownTLVWithUBitSetIsSkipped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L99). **negative:** `unit/verify` [`TestRFC5036UnknownTLVWithUBitClearIsRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L128) |
| `RFC5036-3.4.2.2-1` | It MUST be set to zero on transmission and MUST be ignored on receipt. (§3.4.2.2) | MUST | 3.4.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| `RFC5036-3.4.2.2-2` | If the VCI is less than 16 bits, the preceding bits of the VCI field MUST be set to 0 (§3.4.2.2) | MUST | 3.4.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| `RFC5036-3.4.2.2-3` | If Virtual Path switching is indicated in the V-bits field, then this field MUST be ignored by the receiver and set to 0 by the sender. (§3.4.2.2) | MUST | 3.4.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| `RFC5036-3.4.2.3-1` | The Reserved field of the Frame Relay Label TLV MUST be set to zero on transmission and MUST be ignored on receipt (§3.4.2.3) | MUST | 3.4.2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| `RFC5036-3.4.4.1-1` | If an LSR receives a message containing a Hop Count TLV, it MUST check the hop count value to determine whether the hop count has exceeded its configured maximum allowable value (§3.4.4.1) | MUST | 3.4.4.1 | **positive:** `unit/verify` [`TestRFC5036HopCountWithinMaximumIsApplied`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L174). **negative:** `unit/verify` [`TestRFC5036HopCountAboveMaximumDrawsLoopDetected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L203) |
| `RFC5036-3.4.4.1-2` | If so, it MUST behave as if the containing message has traversed a loop by sending a Notification message signaling Loop Detected in reply to the sender of the message. (§3.4.4.1) | MUST | 3.4.4.1 | **positive:** `unit/verify` [`TestRFC5036HopCountAboveMaximumDrawsLoopDetected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L206). **negative:** `unit/verify` [`TestRFC5036HopCountWithinMaximumIsApplied`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L177) |
| `RFC5036-3.4.4.1-3` | If Loop Detection is configured, the LSR MUST follow the procedures specified in Section "Loop Detection". (§3.4.4.1) | MUST | 3.4.4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-3.4.5.1.1-1` | An LSR that receives a Path Vector in a Label Request message MUST perform the procedures described in Section "Loop Detection". (§3.4.5.1.1) | MUST | 3.4.5.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-3.4.5.1.1-2` | If the LSR detects a loop, it MUST reject the Label Request message. (§3.4.5.1.1) | MUST | 3.4.5.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-3.4.5.1.1-3` | On rejecting a looping Label Request the LSR MUST perform the steps the section lists: send a Notification message signaling Loop Detected to the LSR that sent the message, and abandon processing of it (§3.4.5.1.1) | MUST | 3.4.5.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-3.4.5.1.2-1` | An LSR that receives a Path Vector in a Label Mapping message MUST perform the procedures described in Section "Loop Detection". (§3.4.5.1.2) | MUST | 3.4.5.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-3.4.5.1.2-2` | If the LSR detects a loop, it MUST reject the Label Mapping message in order to prevent a forwarding loop. (§3.4.5.1.2) | MUST | 3.4.5.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-3.4.5.1.2-3` | On rejecting a looping Label Mapping the LSR MUST perform the steps the section lists: send a Notification message signaling Loop Detected to the LSR that sent the message, and abandon processing of it (§3.4.5.1.2) | MUST | 3.4.5.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-3.5-1` | For messages that have required parameters, the required parameters MUST appear in the order specified by the individual message specifications (§3.5) | MUST | 3.5 | **positive:** `unit/verify` [`TestRFC5036RequiredParametersInSpecifiedOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L330). **negative:** no negative test. **{single-polarity}:** the obligation binds the sender and ze's encoders take no input that could reorder the mandatory parameters, so no violating input exists to refuse |
| `RFC5036-3.5.3-2` | If the session is for a label-controlled ATM link or a label-controlled Frame Relay link, then Downstream on Demand MUST be used (§3.5.3) | MUST | 3.5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| `RFC5036-3.5.3-3` | Otherwise, Downstream Unsolicited MUST be used (§3.5.3) | MUST | 3.5.3 | **positive:** `unit/verify` [`TestRFC5036InitProposesDownstreamUnsolicited`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L97). **negative:** `unit/verify` [`TestRFC5036InitOnDemandProposalKeepsDownstreamUnsolicited`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L117) |
| `RFC5036-3.5.3-4` | If the label advertisement discipline determined in this way is unacceptable to an LSR, it MUST send a Session Rejected/Parameters Advertisement Mode Notification message in response to the Initialization message and not establish the session (§3.5.3) | MUST | 3.5.3 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC5036InitOnDemandProposalKeepsDownstreamUnsolicited`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L121). **{single-polarity}:** Downstream Unsolicited is the only discipline ze runs and it is always acceptable, so no unacceptable advertisement mode exists to reject |
| `RFC5036-3.5.3-5` | The Path Vector Limit of the Common Session Parameters MUST be 0 if Loop Detection is disabled (D = 0) (§3.5.3) | MUST | 3.5.3 | **positive:** `unit/verify` [`TestRFC5036InitPathVectorLimitZeroWithoutLoopDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L163). **negative:** `unit/verify` [`TestRFC5036EncodeInitDropsPathVectorLimitWithoutLoopDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L179) |
| `RFC5036-3.5.3-6` | A Reserved field of the Common, ATM or Frame Relay Session Parameters TLV MUST be set to zero on transmission and ignored on receipt (§3.5.3) | MUST | 3.5.3 | **positive:** `unit/verify` [`TestRFC5036InitReservedBitsZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L215). **negative:** `unit/verify` [`TestRFC5036InitReservedBitsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L229) |
| `RFC5036-3.5.3-7` | The receiving LSR MUST calculate the maximum PDU length for the session by using the smaller of its and its peer's proposals for Max PDU Length (§3.5.3) | MUST | 3.5.3 | **positive:** `unit/verify` [`TestRFC5036InitMaxPDULengthTakesTheSmallerProposal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L266). **negative:** `unit/verify` [`TestRFC5036InitMaxPDULengthNeverRaisedAboveOwnProposal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L305) |
| `RFC5036-3.5.3-8` | If the maximum PDU length determined this way is unacceptable to an LSR, it MUST send a Session Rejected/Parameters Max PDU Length Notification message in response to the Initialization message and not establish the session (§3.5.3) | MUST | 3.5.3 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC5036InitMaxPDULengthTakesTheSmallerProposal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L269). **{single-polarity}:** ze accepts every Max PDU Length by taking the smaller of the two proposals, so no unacceptable value exists to reject |
| `RFC5036-3.5.3-9` | If there is no matching Hello adjacency, the LSR MUST send a Session Rejected/No Hello Notification message in response to the Initialization message and not establish the session. (§3.5.3) | MUST | 3.5.3 | **positive:** `unit/verify` [`TestRFC5036InitFromHelloAdjacencyAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L249). **negative:** `unit/verify` [`TestRFC5036InitWithoutHelloAdjacencyRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L270) |
| `RFC5036-3.5.3-10` | A receiving LSR MUST calculate the intersection between the received range and its own supported label range. (§3.5.3) | MUST | 3.5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| `RFC5036-3.5.3-11` | LSRs MUST NOT establish a session with neighbors for which the intersection of ranges is NULL. (§3.5.3) | MUST NOT | 3.5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| `RFC5036-3.5.3-12` | In this case, the LSR MUST send a Session Rejected/Parameters Label Range Notification message in response to the Initialization message and not establish the session. (§3.5.3) | MUST | 3.5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| `RFC5036-3.5.3-13` | The Reserved field that precedes the ATM label range MUST be set to zero on transmission and MUST be ignored on receipt (§3.5.3) | MUST | 3.5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| `RFC5036-3.5.3-14` | When peer LSRs are connected indirectly by means of an ATM VP, the receiving LSR MUST ignore the Minimum and Maximum VPI fields (§3.5.3) | MUST | 3.5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| `RFC5036-3.5.4.1-1` | An LSR MUST arrange that its peer receive an LDP message from it at least every KeepAlive Time period (§3.5.4.1) | MUST | 3.5.4.1 | **positive:** `unit/verify` [`TestRFC5036PeerHearsFromZeWithinKeepaliveTime`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L375). **negative:** `unit/verify` [`TestRFC5036KeepalivePacingFollowsNegotiatedTime`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L403) |
| `RFC5036-3.5.7-1` | Label Request Message ID If this Label Mapping message is a response to a Label Request message, it MUST include the Label Request Message ID optional parameter. (§3.5.7) | MUST | 3.5.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| `RFC5036-3.5.8.1-1` | Unless its routing table includes an entry that exactly matches the requested Prefix, the LSR MUST respond with a No Route Notification message. (§3.5.8.1) | MUST | 3.5.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| `RFC5036-3.5.8.1-2` | When the receiving LSR responds with a Label Mapping message, the mapping message MUST include a Label Request/Returned Message ID TLV optional parameter that includes the message ID of the Label Request message. (§3.5.8.1) | MUST | 3.5.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| `RFC5036-3.5.8.1-3` | When resources become available, the LSR MUST notify the requesting LSR by sending a Notification message with the Label Resources Available Status Code. (§3.5.8.1) | MUST | 3.5.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| `RFC5036-3.5.8.1-4` | An LSR that receives a No Label Resources response to a Label Request message MUST NOT issue further Label Request messages until it receives a Notification message with the Label Resources Available Status Code (§3.5.8.1) | MUST NOT | 3.5.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| `RFC5036-3.5.9.1-1` | When an LSR receives a Label Abort Request message, if it has not previously responded to the Label Request being aborted with a Label Mapping message or some other Notification message, it MUST acknowledge the abort by responding with a Label Request Aborted Notification message (§3.5.9.1) | MUST | 3.5.9.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| `RFC5036-3.5.9.1-2` | The Notification MUST include a Label Request Message ID TLV that carries the message ID of the aborted Label Request message. (§3.5.9.1) | MUST | 3.5.9.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| `RFC5036-3.5.9.1-3` | An LSR receiving a Label Abort Request message MUST process it immediately, regardless of the downstream state of the LSP, responding with a Label Request Aborted Notification or ignoring it, as appropriate (§3.5.9.1) | MUST | 3.5.9.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| `RFC5036-3.5.11.1-1` | An LSR MUST transmit a Label Release message under any of the conditions the section lists (§3.5.11.1) | MUST | 3.5.11.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| `RFC5036-3.6.1.1-1` | Implementations that support vendor-private TLVs MUST support a user-accessible configuration interface that causes the U-bit to be set on all transmitted vendor-private TLVs; this requirement MAY be satisfied by a user-accessible configuration interface that prevents transmission of all vendor-private TLVs for which the U- bit is clear. (§3.6.1.1) | MUST | 3.6.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze encodes no vendor-private TLV or message, so no U-bit configuration interface exists; plan/spec-ldp-vendor-private-tlvs.md |
| `RFC5036-3.6.1.2-1` | Implementations that support Vendor-Private messages MUST support a user-accessible configuration interface that causes the U-bit to be set on all transmitted Vendor-Private messages; this requirement MAY be satisfied by a user-accessible configuration interface that prevents transmission of all Vendor-Private messages for which the U-bit is clear. (§3.6.1.2) | MUST | 3.6.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze encodes no vendor-private TLV or message, so no U-bit configuration interface exists; plan/spec-ldp-vendor-private-tlvs.md |
| `RFC5036-A.1.2-1` | An LSR operating in Downstream Unsolicited mode MUST process any Label Request messages it receives (§A.1.2) | MUST | A.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| `RFC5036-A.1.7-1` | Regardless of the Label Request procedure in use by the LSR, it MUST send a label request if the conditions in NH.13 hold (§A.1.7) | MUST | A.1.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| `RFC5036-A.1-1` | The requirement on an LDP implementation is that its event handling must have the effect specified by the label distribution algorithms of this appendix; an implementation need not follow exactly the steps they specify (§A.1) | MUST | A.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements the Downstream Unsolicited, liberal-retention, independent-control subset and none of the Label Request, Release or Abort algorithms; plan/spec-ldp-label-request-path.md |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5036-2.6.1.2-1`](#rfc5036-2.6.1.2-1) An LSR MUST send a Label Withdraw message when a previously advertised binding is no longer valid (§2.6.1.2). The cited section is about ordered control and states no such obligation; the sentence is in Appendix A: "Whenever an LSR breaks the binding between a label and a FEC, it MUST withdraw the FEC label mapping from all LDP peers to which it has previously sent the mapping" | {gap}, no test | the encoder and sender exist (internal/plugins/ldp/session.go:326 SendLabelWithdraw, internal/plugins/ldp/wire.go:499 EncodeLabelWithdraw) but nothing invokes them -- a local binding is created once in OnStarted (internal/plugins/ldp/register.go:312) and released only by RemovePop at engine exit (internal/plugins/ldp/register.go:380), so no Label Withdraw ever reaches the wire |
| [`RFC5036-2.6.1.3-1`](#rfc5036-2.6.1.3-1) An LSR MUST send a Label Release message when it no longer needs a label (§2.6.1.3). The cited section states no such obligation; the sentence is in 3.5.10.1: "An LSR that receives a Label Withdraw message MUST respond with a Label Release message" | {gap}, no test | the withdraw handler drops the binding and reconciles forwarding without replying (internal/plugins/ldp/register.go:821 the onWithdraw callback, internal/plugins/ldp/fib.go:48 withdrawRemoteBinding); MsgTypeLabelRelease has no encoder and an inbound one is discarded at internal/plugins/ldp/session.go:463 |
| [`RFC5036-2.5.1-3`](#rfc5036-2.5.1-3) An LSR MUST respond to a received Initialization with a KeepAlive message if parameters are acceptable (§2.5.1). The sentence is in 2.5.3 and is indicative: "LSR1 replies with an Initialization message of its own to propose the parameters it wishes to use and a KeepAlive message to signal acceptance of LSR2's parameters" | {gap}, no test | handleInit (internal/plugins/ldp/session.go:504) applies the negotiated parameters and advances the FSM without emitting anything; the only establishment KeepAlive is the unconditional one at internal/plugins/ldp/register.go:758, sent right after ze's own Initialization and before the peer's arrives, so no KeepAlive is conditioned on receiving and accepting an Initialization |
| [`RFC5036-2.5.1-4`](#rfc5036-2.5.1-4) The LSR MUST check that the LSR ID matches what was expected in the Initialization (§2.5.1). The cited section states no such check; 2.5.3 says the passive LSR "attempts to match the LDP Identifier carried by the message PDU with a Hello adjacency", and the enforcing MUST is in 3.5.3, recorded as RFC5036-3.5.3-9 | {gap}, no test | handleInit overwrites the expected peer LSR ID with whatever the PDU header carried (internal/plugins/ldp/session.go:508 `s.peerLSRID = peerLSRID`) instead of comparing it to the value learned from the Hello, and the Receiver LSR ID decoded from the Common Session Parameters TLV (internal/plugins/ldp/wire.go:338) is never compared to the local LSR ID |
| [`RFC5036-2.5.3-2`](#rfc5036-2.5.3-2) An LSR MUST send a Notification message for fatal errors that require session teardown (§2.5.3) | {gap}, no test | partly met. An unacceptable Initialization IS now NAK'd -- rejectInit (internal/plugins/ldp/session.go) sends the Notification through encodeNotification (internal/plugins/ldp/wire.go) for Bad Protocol Version and Session Rejected/Bad KeepAlive Time -- but every other fatal error still returns the error and closes the connection with no Notification: keepalive expiry and decode failure in ReadLoop and processMessages (internal/plugins/ldp/session.go), and the session-ended log in register.go |
| [`RFC5036-2.7-1`](#rfc5036-2.7-1) To enable LSRs to map between a peer LDP Identifier and the peer's addresses, an LSR must be able to map the next hop address for a prefix to an LDP Identifier and an LDP Identifier to the peer's addresses, and LSRs advertise their addresses using LDP Address and Address Withdraw messages (§2.7) | {gap}, no test | ze decodes Address messages but never sends one; plan/pre-release/spec-ldp-address-advertisement.md |
| [`RFC5036-3.5.1-1`](#rfc5036-3.5.1-1) After sending a Notification message whose Status Code indicates a fatal error, the LSR SHOULD terminate the LDP session by closing the session TCP connection and discard all state associated with the session, including all label-FEC bindings learned via the session. The sentence is in subsection 3.5.1.1 and states SHOULD, not MUST NOT (§3.5.1) | {gap} | partly met. On the Initialization path the obligation now holds end to end -- rejectInit (internal/plugins/ldp/session.go) sends the fatal Notification and returns the cause, processMessages stops on it and ReadLoop propagates it, so no further message from that peer is processed. Every other fatal error still sends no Notification, so the obligation's trigger has no producer there; it is unmet for the same reason as RFC5036-2.5.3-2 |
| [`RFC5036-2.9-1`](#rfc5036-2.9-1) The use of the TCP MD5 Signature Option mechanism that protects against spoofed TCP segments in LDP session connection streams MUST be supported as a configurable option (§2.9) | {gap}, no test | ze reads no LDP password and sets no TCP_MD5SIG; plan/pre-release/spec-ldp-md5-authentication.md |
| [`RFC5036-2.8.1-1`](#rfc5036-2.8.1-1) - The Label Request message MUST include a Hop Count TLV. (§2.8.1) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.1-2`](#rfc5036-2.8.1-2) - If R is sending the Label Request because it is a FEC ingress, it MUST include a Hop Count TLV with hop count value 1. (§2.8.1) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.1-3`](#rfc5036-2.8.1-3) - If R is sending the Label Request as a result of having received a Label Request from an upstream LSR, and if the received Label Request contains a Hop Count TLV, R MUST increment the received hop count value by 1 and MUST pass the resulting value in a Hop Count TLV to its next hop along with the Label Request message. (§2.8.1) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.1-4`](#rfc5036-2.8.1-4) - If R is sending the Label Request because it is a FEC ingress, then if R is non-merge capable, it MUST include a Path Vector TLV of length 1 containing its own LSR Id. (§2.8.1) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.1-5`](#rfc5036-2.8.1-5) R MUST add its own LSR Id to the Path Vector, and MUST pass the resulting Path Vector to its next hop along with the Label Request message. (§2.8.1) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.1-6`](#rfc5036-2.8.1-6) If the Label Request contains no Path Vector TLV, R MUST include a Path Vector TLV of length 1 containing its own LSR Id. (§2.8.1) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.1-7`](#rfc5036-2.8.1-7) When R detects a loop, it MUST send a Loop Detected Notification message to the source of the Label Request message and drop the Label Request message. (§2.8.1) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.2-1`](#rfc5036-2.8.2-1) If configured for Loop Detection, R MUST include a Hop Count TLV in the Label Mapping message (§2.8.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.2-2`](#rfc5036-2.8.2-2) - If R is the egress, the hop count value MUST be 1. (§2.8.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.2-3`](#rfc5036-2.8.2-3) If configured for Loop Detection and the Label Mapping message propagates one received from the next hop to an upstream peer, the hop count value MUST be determined by the two rules that follow it (§2.8.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.2-4`](#rfc5036-2.8.2-4) o If R is a member of the edge set of an LSR domain whose LSRs do not perform 'TTL-decrement' (e.g., an ATM LSR domain or a Frame Relay LSR domain) and the upstream peer is within that domain, R MUST reset the hop count to 1 before propagating the message. (§2.8.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.2-5`](#rfc5036-2.8.2-5) If configured for Loop Detection and the preceding case does not hold, R MUST increment the hop count received from the next hop before propagating the message (§2.8.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.2-6`](#rfc5036-2.8.2-6) - If the Label Mapping message is not being sent to propagate a Label Mapping message, the hop count value MUST be the result of incrementing R's current knowledge of the hop count learned from previous Label Mapping messages. (§2.8.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.2-7`](#rfc5036-2.8.2-7) - If R is sending the Label Mapping message to propagate a Label Mapping message received from the next hop to an upstream peer, then: o If R is merge capable and if R has not previously sent a Label Mapping message to the upstream peer, then it MUST include a Path Vector TLV. (§2.8.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.2-8`](#rfc5036-2.8.2-8) o If the received message contains an unknown hop count, then R MUST include a Path Vector TLV. (§2.8.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.2-9`](#rfc5036-2.8.2-9) o If R has previously sent a Label Mapping message to the upstream peer, then it MUST include a Path Vector TLV if the received message reports an LSP hop count increase, a change in hop count from unknown to known, or a change from known to unknown. (§2.8.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.2-10`](#rfc5036-2.8.2-10) o If the received Label Mapping message included a Path Vector, the Path Vector sent upstream MUST be the result of adding R's LSR Id to the received Path Vector. (§2.8.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.2-11`](#rfc5036-2.8.2-11) o If the received message had no Path Vector, the Path Vector sent upstream MUST be a Path Vector of length 1 containing R's LSR Id. (§2.8.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.2-12`](#rfc5036-2.8.2-12) - If the Label Mapping message is not being sent to propagate a received message upstream, the Label Mapping message MUST include a Path Vector of length 1 containing R's LSR Id. (§2.8.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.2-13`](#rfc5036-2.8.2-13) When R detects a loop, it MUST stop using the label for forwarding, drop the Label Mapping message, and signal Loop Detected status to the source of the Label Mapping message. (§2.8.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-3.4.2.2-1`](#rfc5036-3.4.2.2-1) It MUST be set to zero on transmission and MUST be ignored on receipt. (§3.4.2.2) | {gap}, no test | ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| [`RFC5036-3.4.2.2-2`](#rfc5036-3.4.2.2-2) If the VCI is less than 16 bits, the preceding bits of the VCI field MUST be set to 0 (§3.4.2.2) | {gap}, no test | ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| [`RFC5036-3.4.2.2-3`](#rfc5036-3.4.2.2-3) If Virtual Path switching is indicated in the V-bits field, then this field MUST be ignored by the receiver and set to 0 by the sender. (§3.4.2.2) | {gap}, no test | ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| [`RFC5036-3.4.2.3-1`](#rfc5036-3.4.2.3-1) The Reserved field of the Frame Relay Label TLV MUST be set to zero on transmission and MUST be ignored on receipt (§3.4.2.3) | {gap}, no test | ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| [`RFC5036-3.4.4.1-3`](#rfc5036-3.4.4.1-3) If Loop Detection is configured, the LSR MUST follow the procedures specified in Section "Loop Detection". (§3.4.4.1) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-3.4.5.1.1-1`](#rfc5036-3.4.5.1.1-1) An LSR that receives a Path Vector in a Label Request message MUST perform the procedures described in Section "Loop Detection". (§3.4.5.1.1) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-3.4.5.1.1-2`](#rfc5036-3.4.5.1.1-2) If the LSR detects a loop, it MUST reject the Label Request message. (§3.4.5.1.1) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-3.4.5.1.1-3`](#rfc5036-3.4.5.1.1-3) On rejecting a looping Label Request the LSR MUST perform the steps the section lists: send a Notification message signaling Loop Detected to the LSR that sent the message, and abandon processing of it (§3.4.5.1.1) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-3.4.5.1.2-1`](#rfc5036-3.4.5.1.2-1) An LSR that receives a Path Vector in a Label Mapping message MUST perform the procedures described in Section "Loop Detection". (§3.4.5.1.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-3.4.5.1.2-2`](#rfc5036-3.4.5.1.2-2) If the LSR detects a loop, it MUST reject the Label Mapping message in order to prevent a forwarding loop. (§3.4.5.1.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-3.4.5.1.2-3`](#rfc5036-3.4.5.1.2-3) On rejecting a looping Label Mapping the LSR MUST perform the steps the section lists: send a Notification message signaling Loop Detected to the LSR that sent the message, and abandon processing of it (§3.4.5.1.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-3.5.3-2`](#rfc5036-3.5.3-2) If the session is for a label-controlled ATM link or a label-controlled Frame Relay link, then Downstream on Demand MUST be used (§3.5.3) | {gap}, no test | ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| [`RFC5036-3.5.3-10`](#rfc5036-3.5.3-10) A receiving LSR MUST calculate the intersection between the received range and its own supported label range. (§3.5.3) | {gap}, no test | ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| [`RFC5036-3.5.3-11`](#rfc5036-3.5.3-11) LSRs MUST NOT establish a session with neighbors for which the intersection of ranges is NULL. (§3.5.3) | {gap}, no test | ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| [`RFC5036-3.5.3-12`](#rfc5036-3.5.3-12) In this case, the LSR MUST send a Session Rejected/Parameters Label Range Notification message in response to the Initialization message and not establish the session. (§3.5.3) | {gap}, no test | ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| [`RFC5036-3.5.3-13`](#rfc5036-3.5.3-13) The Reserved field that precedes the ATM label range MUST be set to zero on transmission and MUST be ignored on receipt (§3.5.3) | {gap}, no test | ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| [`RFC5036-3.5.3-14`](#rfc5036-3.5.3-14) When peer LSRs are connected indirectly by means of an ATM VP, the receiving LSR MUST ignore the Minimum and Maximum VPI fields (§3.5.3) | {gap}, no test | ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| [`RFC5036-3.5.7-1`](#rfc5036-3.5.7-1) Label Request Message ID If this Label Mapping message is a response to a Label Request message, it MUST include the Label Request Message ID optional parameter. (§3.5.7) | {gap}, no test | ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| [`RFC5036-3.5.8.1-1`](#rfc5036-3.5.8.1-1) Unless its routing table includes an entry that exactly matches the requested Prefix, the LSR MUST respond with a No Route Notification message. (§3.5.8.1) | {gap}, no test | ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| [`RFC5036-3.5.8.1-2`](#rfc5036-3.5.8.1-2) When the receiving LSR responds with a Label Mapping message, the mapping message MUST include a Label Request/Returned Message ID TLV optional parameter that includes the message ID of the Label Request message. (§3.5.8.1) | {gap}, no test | ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| [`RFC5036-3.5.8.1-3`](#rfc5036-3.5.8.1-3) When resources become available, the LSR MUST notify the requesting LSR by sending a Notification message with the Label Resources Available Status Code. (§3.5.8.1) | {gap}, no test | ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| [`RFC5036-3.5.8.1-4`](#rfc5036-3.5.8.1-4) An LSR that receives a No Label Resources response to a Label Request message MUST NOT issue further Label Request messages until it receives a Notification message with the Label Resources Available Status Code (§3.5.8.1) | {gap}, no test | ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| [`RFC5036-3.5.9.1-1`](#rfc5036-3.5.9.1-1) When an LSR receives a Label Abort Request message, if it has not previously responded to the Label Request being aborted with a Label Mapping message or some other Notification message, it MUST acknowledge the abort by responding with a Label Request Aborted Notification message (§3.5.9.1) | {gap}, no test | ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| [`RFC5036-3.5.9.1-2`](#rfc5036-3.5.9.1-2) The Notification MUST include a Label Request Message ID TLV that carries the message ID of the aborted Label Request message. (§3.5.9.1) | {gap}, no test | ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| [`RFC5036-3.5.9.1-3`](#rfc5036-3.5.9.1-3) An LSR receiving a Label Abort Request message MUST process it immediately, regardless of the downstream state of the LSP, responding with a Label Request Aborted Notification or ignoring it, as appropriate (§3.5.9.1) | {gap}, no test | ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| [`RFC5036-3.5.11.1-1`](#rfc5036-3.5.11.1-1) An LSR MUST transmit a Label Release message under any of the conditions the section lists (§3.5.11.1) | {gap}, no test | ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| [`RFC5036-3.6.1.1-1`](#rfc5036-3.6.1.1-1) Implementations that support vendor-private TLVs MUST support a user-accessible configuration interface that causes the U-bit to be set on all transmitted vendor-private TLVs; this requirement MAY be satisfied by a user-accessible configuration interface that prevents transmission of all vendor-private TLVs for which the U- bit is clear. (§3.6.1.1) | {gap}, no test | ze encodes no vendor-private TLV or message, so no U-bit configuration interface exists; plan/spec-ldp-vendor-private-tlvs.md |
| [`RFC5036-3.6.1.2-1`](#rfc5036-3.6.1.2-1) Implementations that support Vendor-Private messages MUST support a user-accessible configuration interface that causes the U-bit to be set on all transmitted Vendor-Private messages; this requirement MAY be satisfied by a user-accessible configuration interface that prevents transmission of all Vendor-Private messages for which the U-bit is clear. (§3.6.1.2) | {gap}, no test | ze encodes no vendor-private TLV or message, so no U-bit configuration interface exists; plan/spec-ldp-vendor-private-tlvs.md |
| [`RFC5036-A.1.2-1`](#rfc5036-a.1.2-1) An LSR operating in Downstream Unsolicited mode MUST process any Label Request messages it receives (§A.1.2) | {gap}, no test | ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| [`RFC5036-A.1.7-1`](#rfc5036-a.1.7-1) Regardless of the Label Request procedure in use by the LSR, it MUST send a label request if the conditions in NH.13 hold (§A.1.7) | {gap}, no test | ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| [`RFC5036-A.1-1`](#rfc5036-a.1-1) The requirement on an LDP implementation is that its event handling must have the effect specified by the label distribution algorithms of this appendix; an implementation need not follow exactly the steps they specify (§A.1) | {gap}, no test | ze implements the Downstream Unsolicited, liberal-retention, independent-control subset and none of the Label Request, Release or Abort algorithms; plan/spec-ldp-label-request-path.md |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5036-x-1`](#rfc5036-x-1)

Version field in PDU header must be 1 (Wire Format)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036PDUVersionOtherRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L153) | unit/verify | unproven |
| positive | [`TestRFC5036PDUVersionOneAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L126) | unit/verify | unproven |

### [`RFC5036-x-2`](#rfc5036-x-2)

Reserved bits in Common Hello Parameters TLV must be zero (Discovery)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036HelloReservedBitsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L200) | unit/verify | unproven |
| positive | [`TestRFC5036HelloReservedBitsZeroOnTransmit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L178) | unit/verify | unproven |

### [`RFC5036-x-3`](#rfc5036-x-3)

Protocol Version in Common Session Parameters must be 1 (Sessions)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036InitProtocolVersionOtherRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L257) | unit/verify | revert, verified |
| positive | [`TestRFC5036InitProtocolVersionOne`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L223) | unit/verify | unproven |

### [`RFC5036-2.5.1-1`](#rfc5036-2.5.1-1)

An LSR MUST send the Initialization message to start a session (§2.5.1). The cited section describes session establishment and states no obligation; the sentence is in 2.5.3 and is indicative: "if LSR1 is playing the active role, it initiates negotiation of session parameters by sending an Initialization message to LSR2"

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036SessionNotOperationalWithoutOwnInit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L426) | unit/verify | unproven |
| positive | [`TestRFC5036SessionSendsInitializationFirst`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L403) | unit/verify | unproven |

### [`RFC5036-2.5.3-1`](#rfc5036-2.5.3-1)

An LSR MUST periodically send KeepAlive messages on established sessions (§2.5.3). The sentence is in 3.5.4.1: "in circumstances where no other LDP protocol messages have been sent within the period, a KeepAlive message MUST be sent"

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036KeepalivesNotSentContinuously`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L540) | unit/verify | unproven |
| positive | [`TestRFC5036KeepalivesSentPeriodically`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L514) | unit/verify | unproven |

### [`RFC5036-2.6.1.2-1`](#rfc5036-2.6.1.2-1)

An LSR MUST send a Label Withdraw message when a previously advertised binding is no longer valid (§2.6.1.2). The cited section is about ordered control and states no such obligation; the sentence is in Appendix A: "Whenever an LSR breaks the binding between a label and a FEC, it MUST withdraw the FEC label mapping from all LDP peers to which it has previously sent the mapping"

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.6.1.2-1, so no unit is bound to it.

### [`RFC5036-2.6.1.3-1`](#rfc5036-2.6.1.3-1)

An LSR MUST send a Label Release message when it no longer needs a label (§2.6.1.3). The cited section states no such obligation; the sentence is in 3.5.10.1: "An LSR that receives a Label Withdraw message MUST respond with a Label Release message"

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.6.1.3-1, so no unit is bound to it.

### [`RFC5036-2.5.1-2`](#rfc5036-2.5.1-2)

An LSR MUST accept the lower of the two proposed KeepAlive Timer values during negotiation (§2.5.1). The sentence is in 3.5.3: "The receiving LSR MUST calculate the value of the KeepAlive Timer by using the smaller of its proposed KeepAlive Time and the KeepAlive Time received in the PDU"

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036KeepaliveNegotiationRefusesHigher`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L475) | unit/verify | unproven |
| positive | [`TestRFC5036KeepaliveNegotiationAdoptsLower`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L459) | unit/verify | unproven |

### [`RFC5036-3.5.3-1`](#rfc5036-3.5.3-1)

The Common Session Parameters KeepAlive Time is a two octet unsigned NON ZERO integer, so an Initialization proposing 0 MUST be rejected with the Session Rejected/Bad KeepAlive Time Notification (0x00000018) and the session MUST NOT be established (§3.5.3, §3.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036InitZeroKeepaliveTimeRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L355) | unit/verify | revert, verified |
| positive | [`TestRFC5036InitNonZeroKeepaliveTimeAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L318) | unit/verify | revert, verified |

### [`RFC5036-2.5.1-3`](#rfc5036-2.5.1-3)

An LSR MUST respond to a received Initialization with a KeepAlive message if parameters are acceptable (§2.5.1). The sentence is in 2.5.3 and is indicative: "LSR1 replies with an Initialization message of its own to propose the parameters it wishes to use and a KeepAlive message to signal acceptance of LSR2's parameters"

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.5.1-3, so no unit is bound to it.

### [`RFC5036-3.5.2-1`](#rfc5036-3.5.2-1)

A Common Hello Parameters Hold Time of 0 means use the default hold time -- 15 seconds for Link Hellos, 45 seconds for Targeted Hellos -- and the adjacency is kept, not removed (§3.5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036HelloHoldTimeNonZeroNotDefaulted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L619) | unit/verify | unproven |
| positive | [`TestRFC5036HelloHoldTimeZeroUsesDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L579) | unit/verify | unproven |

### [`RFC5036-2.5.1-4`](#rfc5036-2.5.1-4)

The LSR MUST check that the LSR ID matches what was expected in the Initialization (§2.5.1). The cited section states no such check; 2.5.3 says the passive LSR "attempts to match the LDP Identifier carried by the message PDU with a Hello adjacency", and the enforcing MUST is in 3.5.3, recorded as RFC5036-3.5.3-9

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.5.1-4, so no unit is bound to it.

### [`RFC5036-2.5.3-2`](#rfc5036-2.5.3-2)

An LSR MUST send a Notification message for fatal errors that require session teardown (§2.5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.5.3-2, so no unit is bound to it.

### [`RFC5036-2.7-1`](#rfc5036-2.7-1)

To enable LSRs to map between a peer LDP Identifier and the peer's addresses, an LSR must be able to map the next hop address for a prefix to an LDP Identifier and an LDP Identifier to the peer's addresses, and LSRs advertise their addresses using LDP Address and Address Withdraw messages (§2.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.7-1, so no unit is bound to it.

### [`RFC5036-2.9-1`](#rfc5036-2.9-1)

The use of the TCP MD5 Signature Option mechanism that protects against spoofed TCP segments in LDP session connection streams MUST be supported as a configurable option (§2.9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.9-1, so no unit is bound to it.

### [`RFC5036-2.5.3-3`](#rfc5036-2.5.3-3)

An LSR MUST throttle its session setup retry attempts with an exponential backoff in situations where Initialization messages are being NAK'd (§2.5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036SetupRetryRefusedInsideDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L346) | unit/verify | revert, verified |
| positive | [`TestRFC5036SetupRetryBackoffGrows`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L322) | unit/verify | revert, verified |

### [`RFC5036-2.5.3-4`](#rfc5036-2.5.3-4)

The session establishment setup attempt following a NAK'd Initialization message MUST be delayed no less than 15 seconds, and subsequent delays MUST grow to a maximum delay of no less than 2 minutes (§2.5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036SetupRetryRefusedInsideDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L348) | unit/verify | revert, verified |
| positive | [`TestRFC5036SetupRetryBackoffGrows`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L324) | unit/verify | revert, verified |

### [`RFC5036-2.5.2-1`](#rfc5036-2.5.2-1)

An LSR MUST advertise the same transport address in all Hellos that advertise the same label space (§2.5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036HelloTransportAddressNeverFollowsTheSocket`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L501) | unit/verify | revert, verified |
| positive | [`TestRFC5036HellosCarryOneTransportAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L483) | unit/verify | revert, verified |

### [`RFC5036-2.6.1.2-2`](#rfc5036-2.6.1.2-2)

For each FEC for which the LSR is not the egress and no mapping exists, the LSR MUST wait until a label from a downstream LSR is received before mapping the FEC and passing corresponding labels to upstream LSRs. (§2.6.1.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036NonEgressFECNotMappedBeforeDownstreamLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L461) | unit/verify | revert, verified |
| positive | [`TestRFC5036EgressFECMappedWithoutWaiting`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L432) | unit/verify | revert, verified |

### [`RFC5036-2.8.1-1`](#rfc5036-2.8.1-1)

- The Label Request message MUST include a Hop Count TLV. (§2.8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.8.1-1, so no unit is bound to it.

### [`RFC5036-2.8.1-2`](#rfc5036-2.8.1-2)

- If R is sending the Label Request because it is a FEC ingress, it MUST include a Hop Count TLV with hop count value 1. (§2.8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.8.1-2, so no unit is bound to it.

### [`RFC5036-2.8.1-3`](#rfc5036-2.8.1-3)

- If R is sending the Label Request as a result of having received a Label Request from an upstream LSR, and if the received Label Request contains a Hop Count TLV, R MUST increment the received hop count value by 1 and MUST pass the resulting value in a Hop Count TLV to its next hop along with the Label Request message. (§2.8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.8.1-3, so no unit is bound to it.

### [`RFC5036-2.8.1-4`](#rfc5036-2.8.1-4)

- If R is sending the Label Request because it is a FEC ingress, then if R is non-merge capable, it MUST include a Path Vector TLV of length 1 containing its own LSR Id. (§2.8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.8.1-4, so no unit is bound to it.

### [`RFC5036-2.8.1-5`](#rfc5036-2.8.1-5)

R MUST add its own LSR Id to the Path Vector, and MUST pass the resulting Path Vector to its next hop along with the Label Request message. (§2.8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.8.1-5, so no unit is bound to it.

### [`RFC5036-2.8.1-6`](#rfc5036-2.8.1-6)

If the Label Request contains no Path Vector TLV, R MUST include a Path Vector TLV of length 1 containing its own LSR Id. (§2.8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.8.1-6, so no unit is bound to it.

### [`RFC5036-2.8.1-7`](#rfc5036-2.8.1-7)

When R detects a loop, it MUST send a Loop Detected Notification message to the source of the Label Request message and drop the Label Request message. (§2.8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.8.1-7, so no unit is bound to it.

### [`RFC5036-2.8.2-1`](#rfc5036-2.8.2-1)

If configured for Loop Detection, R MUST include a Hop Count TLV in the Label Mapping message (§2.8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.8.2-1, so no unit is bound to it.

### [`RFC5036-2.8.2-2`](#rfc5036-2.8.2-2)

- If R is the egress, the hop count value MUST be 1. (§2.8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.8.2-2, so no unit is bound to it.

### [`RFC5036-2.8.2-3`](#rfc5036-2.8.2-3)

If configured for Loop Detection and the Label Mapping message propagates one received from the next hop to an upstream peer, the hop count value MUST be determined by the two rules that follow it (§2.8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.8.2-3, so no unit is bound to it.

### [`RFC5036-2.8.2-4`](#rfc5036-2.8.2-4)

o If R is a member of the edge set of an LSR domain whose LSRs do not perform 'TTL-decrement' (e.g., an ATM LSR domain or a Frame Relay LSR domain) and the upstream peer is within that domain, R MUST reset the hop count to 1 before propagating the message. (§2.8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.8.2-4, so no unit is bound to it.

### [`RFC5036-2.8.2-5`](#rfc5036-2.8.2-5)

If configured for Loop Detection and the preceding case does not hold, R MUST increment the hop count received from the next hop before propagating the message (§2.8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.8.2-5, so no unit is bound to it.

### [`RFC5036-2.8.2-6`](#rfc5036-2.8.2-6)

- If the Label Mapping message is not being sent to propagate a Label Mapping message, the hop count value MUST be the result of incrementing R's current knowledge of the hop count learned from previous Label Mapping messages. (§2.8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.8.2-6, so no unit is bound to it.

### [`RFC5036-2.8.2-7`](#rfc5036-2.8.2-7)

- If R is sending the Label Mapping message to propagate a Label Mapping message received from the next hop to an upstream peer, then: o If R is merge capable and if R has not previously sent a Label Mapping message to the upstream peer, then it MUST include a Path Vector TLV. (§2.8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.8.2-7, so no unit is bound to it.

### [`RFC5036-2.8.2-8`](#rfc5036-2.8.2-8)

o If the received message contains an unknown hop count, then R MUST include a Path Vector TLV. (§2.8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.8.2-8, so no unit is bound to it.

### [`RFC5036-2.8.2-9`](#rfc5036-2.8.2-9)

o If R has previously sent a Label Mapping message to the upstream peer, then it MUST include a Path Vector TLV if the received message reports an LSP hop count increase, a change in hop count from unknown to known, or a change from known to unknown. (§2.8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.8.2-9, so no unit is bound to it.

### [`RFC5036-2.8.2-10`](#rfc5036-2.8.2-10)

o If the received Label Mapping message included a Path Vector, the Path Vector sent upstream MUST be the result of adding R's LSR Id to the received Path Vector. (§2.8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.8.2-10, so no unit is bound to it.

### [`RFC5036-2.8.2-11`](#rfc5036-2.8.2-11)

o If the received message had no Path Vector, the Path Vector sent upstream MUST be a Path Vector of length 1 containing R's LSR Id. (§2.8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.8.2-11, so no unit is bound to it.

### [`RFC5036-2.8.2-12`](#rfc5036-2.8.2-12)

- If the Label Mapping message is not being sent to propagate a received message upstream, the Label Mapping message MUST include a Path Vector of length 1 containing R's LSR Id. (§2.8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.8.2-12, so no unit is bound to it.

### [`RFC5036-2.8.2-13`](#rfc5036-2.8.2-13)

When R detects a loop, it MUST stop using the label for forwarding, drop the Label Mapping message, and signal Loop Detected status to the source of the Label Mapping message. (§2.8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.8.2-13, so no unit is bound to it.

### [`RFC5036-3.1-1`](#rfc5036-3.1-1)

The first four octets identify the LSR and MUST be a globally unique value. (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036OwnLSRIDFormsNoAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L391) | unit/verify | revert, verified |
| positive | [`TestRFC5036DistinctLSRIDFormsAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L373) | unit/verify | revert, verified |

### [`RFC5036-3.3-1`](#rfc5036-3.3-1)

Upon receipt of an unknown TLV, if U is clear (=0), a notification MUST be returned to the message originator and the entire message MUST be ignored; if U is set (=1), the unknown TLV MUST be silently ignored and the rest of the message processed as if the unknown TLV did not exist. (§3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036UnknownTLVWithUBitClearIsRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L128) | unit/verify | revert, verified |
| positive | [`TestRFC5036UnknownTLVWithUBitSetIsSkipped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L99) | unit/verify | revert, verified |

### [`RFC5036-3.4.2.2-1`](#rfc5036-3.4.2.2-1)

It MUST be set to zero on transmission and MUST be ignored on receipt. (§3.4.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.4.2.2-1, so no unit is bound to it.

### [`RFC5036-3.4.2.2-2`](#rfc5036-3.4.2.2-2)

If the VCI is less than 16 bits, the preceding bits of the VCI field MUST be set to 0 (§3.4.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.4.2.2-2, so no unit is bound to it.

### [`RFC5036-3.4.2.2-3`](#rfc5036-3.4.2.2-3)

If Virtual Path switching is indicated in the V-bits field, then this field MUST be ignored by the receiver and set to 0 by the sender. (§3.4.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.4.2.2-3, so no unit is bound to it.

### [`RFC5036-3.4.2.3-1`](#rfc5036-3.4.2.3-1)

The Reserved field of the Frame Relay Label TLV MUST be set to zero on transmission and MUST be ignored on receipt (§3.4.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.4.2.3-1, so no unit is bound to it.

### [`RFC5036-3.4.4.1-1`](#rfc5036-3.4.4.1-1)

If an LSR receives a message containing a Hop Count TLV, it MUST check the hop count value to determine whether the hop count has exceeded its configured maximum allowable value (§3.4.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036HopCountAboveMaximumDrawsLoopDetected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L203) | unit/verify | revert, verified |
| positive | [`TestRFC5036HopCountWithinMaximumIsApplied`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L174) | unit/verify | revert, verified |

### [`RFC5036-3.4.4.1-2`](#rfc5036-3.4.4.1-2)

If so, it MUST behave as if the containing message has traversed a loop by sending a Notification message signaling Loop Detected in reply to the sender of the message. (§3.4.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036HopCountWithinMaximumIsApplied`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L177) | unit/verify | revert, verified |
| positive | [`TestRFC5036HopCountAboveMaximumDrawsLoopDetected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L206) | unit/verify | revert, verified |

### [`RFC5036-3.4.4.1-3`](#rfc5036-3.4.4.1-3)

If Loop Detection is configured, the LSR MUST follow the procedures specified in Section "Loop Detection". (§3.4.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.4.4.1-3, so no unit is bound to it.

### [`RFC5036-3.4.5.1.1-1`](#rfc5036-3.4.5.1.1-1)

An LSR that receives a Path Vector in a Label Request message MUST perform the procedures described in Section "Loop Detection". (§3.4.5.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.4.5.1.1-1, so no unit is bound to it.

### [`RFC5036-3.4.5.1.1-2`](#rfc5036-3.4.5.1.1-2)

If the LSR detects a loop, it MUST reject the Label Request message. (§3.4.5.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.4.5.1.1-2, so no unit is bound to it.

### [`RFC5036-3.4.5.1.1-3`](#rfc5036-3.4.5.1.1-3)

On rejecting a looping Label Request the LSR MUST perform the steps the section lists: send a Notification message signaling Loop Detected to the LSR that sent the message, and abandon processing of it (§3.4.5.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.4.5.1.1-3, so no unit is bound to it.

### [`RFC5036-3.4.5.1.2-1`](#rfc5036-3.4.5.1.2-1)

An LSR that receives a Path Vector in a Label Mapping message MUST perform the procedures described in Section "Loop Detection". (§3.4.5.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.4.5.1.2-1, so no unit is bound to it.

### [`RFC5036-3.4.5.1.2-2`](#rfc5036-3.4.5.1.2-2)

If the LSR detects a loop, it MUST reject the Label Mapping message in order to prevent a forwarding loop. (§3.4.5.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.4.5.1.2-2, so no unit is bound to it.

### [`RFC5036-3.4.5.1.2-3`](#rfc5036-3.4.5.1.2-3)

On rejecting a looping Label Mapping the LSR MUST perform the steps the section lists: send a Notification message signaling Loop Detected to the LSR that sent the message, and abandon processing of it (§3.4.5.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.4.5.1.2-3, so no unit is bound to it.

### [`RFC5036-3.5-1`](#rfc5036-3.5-1)

For messages that have required parameters, the required parameters MUST appear in the order specified by the individual message specifications (§3.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5036RequiredParametersInSpecifiedOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L330) | unit/verify | revert, verified |

### [`RFC5036-3.5.3-2`](#rfc5036-3.5.3-2)

If the session is for a label-controlled ATM link or a label-controlled Frame Relay link, then Downstream on Demand MUST be used (§3.5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.5.3-2, so no unit is bound to it.

### [`RFC5036-3.5.3-3`](#rfc5036-3.5.3-3)

Otherwise, Downstream Unsolicited MUST be used (§3.5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036InitOnDemandProposalKeepsDownstreamUnsolicited`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L117) | unit/verify | revert, verified |
| positive | [`TestRFC5036InitProposesDownstreamUnsolicited`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L97) | unit/verify | revert, verified |

### [`RFC5036-3.5.3-4`](#rfc5036-3.5.3-4)

If the label advertisement discipline determined in this way is unacceptable to an LSR, it MUST send a Session Rejected/Parameters Advertisement Mode Notification message in response to the Initialization message and not establish the session (§3.5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036InitOnDemandProposalKeepsDownstreamUnsolicited`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L121) | unit/verify | revert, verified |

### [`RFC5036-3.5.3-5`](#rfc5036-3.5.3-5)

The Path Vector Limit of the Common Session Parameters MUST be 0 if Loop Detection is disabled (D = 0) (§3.5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036EncodeInitDropsPathVectorLimitWithoutLoopDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L179) | unit/verify | revert, verified |
| positive | [`TestRFC5036InitPathVectorLimitZeroWithoutLoopDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L163) | unit/verify | revert, verified |

### [`RFC5036-3.5.3-6`](#rfc5036-3.5.3-6)

A Reserved field of the Common, ATM or Frame Relay Session Parameters TLV MUST be set to zero on transmission and ignored on receipt (§3.5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036InitReservedBitsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L229) | unit/verify | revert, verified |
| positive | [`TestRFC5036InitReservedBitsZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L215) | unit/verify | revert, verified |

### [`RFC5036-3.5.3-7`](#rfc5036-3.5.3-7)

The receiving LSR MUST calculate the maximum PDU length for the session by using the smaller of its and its peer's proposals for Max PDU Length (§3.5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036InitMaxPDULengthNeverRaisedAboveOwnProposal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L305) | unit/verify | revert, verified |
| positive | [`TestRFC5036InitMaxPDULengthTakesTheSmallerProposal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L266) | unit/verify | revert, verified |

### [`RFC5036-3.5.3-8`](#rfc5036-3.5.3-8)

If the maximum PDU length determined this way is unacceptable to an LSR, it MUST send a Session Rejected/Parameters Max PDU Length Notification message in response to the Initialization message and not establish the session (§3.5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036InitMaxPDULengthTakesTheSmallerProposal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L269) | unit/verify | revert, verified |

### [`RFC5036-3.5.3-9`](#rfc5036-3.5.3-9)

If there is no matching Hello adjacency, the LSR MUST send a Session Rejected/No Hello Notification message in response to the Initialization message and not establish the session. (§3.5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036InitWithoutHelloAdjacencyRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L270) | unit/verify | revert, verified |
| positive | [`TestRFC5036InitFromHelloAdjacencyAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/procedures_rfc5036_test.go#L249) | unit/verify | revert, verified |

### [`RFC5036-3.5.3-10`](#rfc5036-3.5.3-10)

A receiving LSR MUST calculate the intersection between the received range and its own supported label range. (§3.5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.5.3-10, so no unit is bound to it.

### [`RFC5036-3.5.3-11`](#rfc5036-3.5.3-11)

LSRs MUST NOT establish a session with neighbors for which the intersection of ranges is NULL. (§3.5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.5.3-11, so no unit is bound to it.

### [`RFC5036-3.5.3-12`](#rfc5036-3.5.3-12)

In this case, the LSR MUST send a Session Rejected/Parameters Label Range Notification message in response to the Initialization message and not establish the session. (§3.5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.5.3-12, so no unit is bound to it.

### [`RFC5036-3.5.3-13`](#rfc5036-3.5.3-13)

The Reserved field that precedes the ATM label range MUST be set to zero on transmission and MUST be ignored on receipt (§3.5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.5.3-13, so no unit is bound to it.

### [`RFC5036-3.5.3-14`](#rfc5036-3.5.3-14)

When peer LSRs are connected indirectly by means of an ATM VP, the receiving LSR MUST ignore the Minimum and Maximum VPI fields (§3.5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.5.3-14, so no unit is bound to it.

### [`RFC5036-3.5.4.1-1`](#rfc5036-3.5.4.1-1)

An LSR MUST arrange that its peer receive an LDP message from it at least every KeepAlive Time period (§3.5.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036KeepalivePacingFollowsNegotiatedTime`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L403) | unit/verify | revert, verified |
| positive | [`TestRFC5036PeerHearsFromZeWithinKeepaliveTime`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/sessionparams_rfc5036_test.go#L375) | unit/verify | revert, verified |

### [`RFC5036-3.5.7-1`](#rfc5036-3.5.7-1)

Label Request Message ID If this Label Mapping message is a response to a Label Request message, it MUST include the Label Request Message ID optional parameter. (§3.5.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.5.7-1, so no unit is bound to it.

### [`RFC5036-3.5.8.1-1`](#rfc5036-3.5.8.1-1)

Unless its routing table includes an entry that exactly matches the requested Prefix, the LSR MUST respond with a No Route Notification message. (§3.5.8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.5.8.1-1, so no unit is bound to it.

### [`RFC5036-3.5.8.1-2`](#rfc5036-3.5.8.1-2)

When the receiving LSR responds with a Label Mapping message, the mapping message MUST include a Label Request/Returned Message ID TLV optional parameter that includes the message ID of the Label Request message. (§3.5.8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.5.8.1-2, so no unit is bound to it.

### [`RFC5036-3.5.8.1-3`](#rfc5036-3.5.8.1-3)

When resources become available, the LSR MUST notify the requesting LSR by sending a Notification message with the Label Resources Available Status Code. (§3.5.8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.5.8.1-3, so no unit is bound to it.

### [`RFC5036-3.5.8.1-4`](#rfc5036-3.5.8.1-4)

An LSR that receives a No Label Resources response to a Label Request message MUST NOT issue further Label Request messages until it receives a Notification message with the Label Resources Available Status Code (§3.5.8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.5.8.1-4, so no unit is bound to it.

### [`RFC5036-3.5.9.1-1`](#rfc5036-3.5.9.1-1)

When an LSR receives a Label Abort Request message, if it has not previously responded to the Label Request being aborted with a Label Mapping message or some other Notification message, it MUST acknowledge the abort by responding with a Label Request Aborted Notification message (§3.5.9.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.5.9.1-1, so no unit is bound to it.

### [`RFC5036-3.5.9.1-2`](#rfc5036-3.5.9.1-2)

The Notification MUST include a Label Request Message ID TLV that carries the message ID of the aborted Label Request message. (§3.5.9.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.5.9.1-2, so no unit is bound to it.

### [`RFC5036-3.5.9.1-3`](#rfc5036-3.5.9.1-3)

An LSR receiving a Label Abort Request message MUST process it immediately, regardless of the downstream state of the LSP, responding with a Label Request Aborted Notification or ignoring it, as appropriate (§3.5.9.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.5.9.1-3, so no unit is bound to it.

### [`RFC5036-3.5.11.1-1`](#rfc5036-3.5.11.1-1)

An LSR MUST transmit a Label Release message under any of the conditions the section lists (§3.5.11.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.5.11.1-1, so no unit is bound to it.

### [`RFC5036-3.6.1.1-1`](#rfc5036-3.6.1.1-1)

Implementations that support vendor-private TLVs MUST support a user-accessible configuration interface that causes the U-bit to be set on all transmitted vendor-private TLVs; this requirement MAY be satisfied by a user-accessible configuration interface that prevents transmission of all vendor-private TLVs for which the U- bit is clear. (§3.6.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.6.1.1-1, so no unit is bound to it.

### [`RFC5036-3.6.1.2-1`](#rfc5036-3.6.1.2-1)

Implementations that support Vendor-Private messages MUST support a user-accessible configuration interface that causes the U-bit to be set on all transmitted Vendor-Private messages; this requirement MAY be satisfied by a user-accessible configuration interface that prevents transmission of all Vendor-Private messages for which the U-bit is clear. (§3.6.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.6.1.2-1, so no unit is bound to it.

### [`RFC5036-A.1.2-1`](#rfc5036-a.1.2-1)

An LSR operating in Downstream Unsolicited mode MUST process any Label Request messages it receives (§A.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-A.1.2-1, so no unit is bound to it.

### [`RFC5036-A.1.7-1`](#rfc5036-a.1.7-1)

Regardless of the Label Request procedure in use by the LSR, it MUST send a label request if the conditions in NH.13 hold (§A.1.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-A.1.7-1, so no unit is bound to it.

### [`RFC5036-A.1-1`](#rfc5036-a.1-1)

The requirement on an LDP implementation is that its event handling must have the effect specified by the label distribution algorithms of this appendix; an implementation need not follow exactly the steps they specify (§A.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-A.1-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc5036.txt |
| Source fingerprint | 4e8e1f1d87e26ed1 |
| Record | rfc/extraction/rfc5036.json |
| Mapped sentences | 74 |
| Declined as scope | 13 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | the RFC title block, status-of-memo and copyright notice | 0 | skipped (front-matter) | the RFC title block, status-of-memo and copyright notice |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `1.3` | not stated | 0 | walked | not stated |
| `1.4` | not stated | 0 | walked | not stated |
| `1.5` | not stated | 0 | walked | not stated |
| `1.6` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `2.2.1` | not stated | 0 | walked | not stated |
| `2.2.2` | not stated | 0 | walked | not stated |
| `2.2.3` | not stated | 0 | walked | not stated |
| `2.2.4` | not stated | 0 | walked | not stated |
| `2.3` | not stated | 0 | walked | not stated |
| `2.4` | not stated | 0 | walked | not stated |
| `2.4.1` | not stated | 0 | walked | not stated |
| `2.4.2` | not stated | 0 | walked | not stated |
| `2.5` | not stated | 0 | walked | not stated |
| `2.5.1` | not stated | 0 | walked | not stated |
| `2.5.2` | not stated | 1 | walked | not stated |
| `2.5.3` | not stated | 3 | walked | not stated |
| `2.5.4` | not stated | 0 | walked | not stated |
| `2.5.5` | not stated | 0 | walked | not stated |
| `2.5.6` | not stated | 0 | walked | not stated |
| `2.6` | not stated | 0 | walked | not stated |
| `2.6.1` | not stated | 0 | walked | not stated |
| `2.6.1.1` | not stated | 0 | walked | not stated |
| `2.6.1.2` | not stated | 1 | walked | not stated |
| `2.6.2` | not stated | 0 | walked | not stated |
| `2.6.2.1` | not stated | 0 | walked | not stated |
| `2.6.2.2` | not stated | 0 | walked | not stated |
| `2.6.3` | not stated | 0 | walked | not stated |
| `2.7` | not stated | 0 | walked | not stated |
| `2.8` | not stated | 2 | walked | not stated |
| `2.8.1` | not stated | 7 | walked | not stated |
| `2.8.2` | not stated | 13 | walked | not stated |
| `2.8.3` | not stated | 0 | walked | not stated |
| `2.9` | not stated | 1 | walked | not stated |
| `2.9.1` | not stated | 0 | walked | not stated |
| `2.9.2` | not stated | 0 | walked | not stated |
| `2.10` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 1 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `3.3` | not stated | 1 | walked | not stated |
| `3.4` | not stated | 0 | walked | not stated |
| `3.4.1` | not stated | 0 | walked | not stated |
| `3.4.1.1` | not stated | 0 | walked | not stated |
| `3.4.2` | not stated | 0 | walked | not stated |
| `3.4.2.1` | not stated | 0 | walked | not stated |
| `3.4.2.2` | not stated | 3 | walked | not stated |
| `3.4.2.3` | not stated | 1 | walked | not stated |
| `3.4.3` | not stated | 0 | walked | not stated |
| `3.4.4` | not stated | 0 | walked | not stated |
| `3.4.4.1` | not stated | 6 | walked | not stated |
| `3.4.5` | not stated | 0 | walked | not stated |
| `3.4.5.1` | not stated | 0 | walked | not stated |
| `3.4.5.1.1` | not stated | 3 | walked | not stated |
| `3.4.5.1.2` | not stated | 3 | walked | not stated |
| `3.4.6` | not stated | 0 | walked | not stated |
| `3.5` | not stated | 1 | walked | not stated |
| `3.5.1` | not stated | 0 | walked | not stated |
| `3.5.1.1` | not stated | 0 | walked | not stated |
| `3.5.1.2` | not stated | 0 | walked | not stated |
| `3.5.1.2.1` | not stated | 0 | walked | not stated |
| `3.5.1.2.2` | not stated | 0 | walked | not stated |
| `3.5.1.2.3` | not stated | 0 | walked | not stated |
| `3.5.1.2.4` | not stated | 0 | walked | not stated |
| `3.5.1.2.5` | not stated | 0 | walked | not stated |
| `3.5.1.2.6` | not stated | 0 | walked | not stated |
| `3.5.1.2.7` | not stated | 0 | walked | not stated |
| `3.5.1.2.8` | not stated | 0 | walked | not stated |
| `3.5.2` | not stated | 1 | walked | not stated |
| `3.5.2.1` | not stated | 0 | walked | not stated |
| `3.5.3` | not stated | 20 | walked | not stated |
| `3.5.3.1` | not stated | 0 | walked | not stated |
| `3.5.4` | not stated | 0 | walked | not stated |
| `3.5.4.1` | not stated | 2 | walked | not stated |
| `3.5.5` | not stated | 0 | walked | not stated |
| `3.5.5.1` | not stated | 0 | walked | not stated |
| `3.5.6` | not stated | 0 | walked | not stated |
| `3.5.6.1` | not stated | 0 | walked | not stated |
| `3.5.7` | not stated | 1 | walked | not stated |
| `3.5.7.1` | not stated | 0 | walked | not stated |
| `3.5.7.1.1` | not stated | 0 | walked | not stated |
| `3.5.7.1.2` | not stated | 0 | walked | not stated |
| `3.5.7.1.3` | not stated | 0 | walked | not stated |
| `3.5.7.1.4` | not stated | 0 | walked | not stated |
| `3.5.8` | not stated | 0 | walked | not stated |
| `3.5.8.1` | not stated | 4 | walked | not stated |
| `3.5.9` | not stated | 0 | walked | not stated |
| `3.5.9.1` | not stated | 3 | walked | not stated |
| `3.5.10` | not stated | 0 | walked | not stated |
| `3.5.10.1` | not stated | 1 | walked | not stated |
| `3.5.11` | not stated | 0 | walked | not stated |
| `3.5.11.1` | not stated | 1 | walked | not stated |
| `3.6` | not stated | 0 | walked | not stated |
| `3.6.1` | not stated | 0 | walked | not stated |
| `3.6.1.1` | not stated | 2 | walked | not stated |
| `3.6.1.2` | not stated | 1 | walked | not stated |
| `3.6.2` | not stated | 0 | walked | not stated |
| `3.7` | not stated | 0 | walked | not stated |
| `3.8` | not stated | 0 | walked | not stated |
| `3.9` | not stated | 0 | walked | not stated |
| `3.10` | not stated | 0 | walked | not stated |
| `3.10.1` | not stated | 0 | walked | not stated |
| `3.10.2` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | skipped (iana) | IANA Considerations: name space management guidelines addressed to IANA |
| `4.1` | message type name space allocation policy | 0 | skipped (iana) | message type name space allocation policy |
| `4.2` | TLV type name space allocation policy | 0 | skipped (iana) | TLV type name space allocation policy |
| `4.3` | FEC type name space allocation policy | 0 | skipped (iana) | FEC type name space allocation policy |
| `4.4` | status code name space allocation policy | 0 | skipped (iana) | status code name space allocation policy |
| `4.5` | experiment ID name space allocation policy | 0 | skipped (iana) | experiment ID name space allocation policy |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
| `5.2` | not stated | 0 | walked | not stated |
| `5.3` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | Acknowledgments | 0 | skipped (acknowledgements) | Acknowledgments |
| `9` | References | 0 | skipped (references) | References |
| `9.1` | Normative References | 0 | skipped (references) | Normative References |
| `9.2` | Informative References | 0 | skipped (references) | Informative References |
| `A` | not stated | 1 | walked | not stated |
| `A.1` | not stated | 0 | walked | not stated |
| `A.1.1` | not stated | 0 | walked | not stated |
| `A.1.2` | not stated | 1 | walked | not stated |
| `A.1.3` | not stated | 0 | walked | not stated |
| `A.1.4` | not stated | 0 | walked | not stated |
| `A.1.5` | not stated | 0 | walked | not stated |
| `A.1.6` | not stated | 0 | walked | not stated |
| `A.1.7` | not stated | 1 | walked | not stated |
| `A.1.8` | not stated | 0 | walked | not stated |
| `A.1.9` | not stated | 0 | walked | not stated |
| `A.1.10` | not stated | 0 | walked | not stated |
| `A.1.11` | not stated | 0 | walked | not stated |
| `A.1.12` | not stated | 0 | walked | not stated |
| `A.1.13` | not stated | 0 | walked | not stated |
| `A.1.14` | not stated | 1 | walked | not stated |
| `A.1.15` | not stated | 0 | walked | not stated |
| `A.2` | not stated | 0 | walked | not stated |
| `A.2.1` | not stated | 0 | walked | not stated |
| `A.2.2` | not stated | 0 | walked | not stated |
| `A.2.3` | not stated | 0 | walked | not stated |
| `A.2.4` | not stated | 0 | walked | not stated |
| `A.2.5` | not stated | 0 | walked | not stated |
| `A.2.6` | not stated | 0 | walked | not stated |
| `A.2.7` | not stated | 0 | walked | not stated |
| `A.2.8` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `2.8:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | a definition of the document's own keyword usage: it redefines MUST for the Loop Detection paragraphs and states no behavior of its own | For these paragraphs, and only these paragraphs, "MUST" is redefined to mean "MUST if configured for Loop Detection". |
| `2.8:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | an announcement of what the paragraphs that follow specify, not an obligation; the obligations are the paragraphs themselves | The paragraphs specify messages that MUST carry Path Vector and Hop Count TLVs. |
| `3.4.4.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Hop Count TLV section restates the Loop Detection increment rule for the Label Request message that site 2.8.1:3 maps | - If the message is a Label Request message, R MUST increment the received hop count; - If the message is a Label Mapping message, R determines the hop count as follows: |
| `3.4.4.1:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates the edge-set reset-to-1 rule for a propagated Label Mapping that site 2.8.2:4 maps | o If R is a member of the edge set of an LSR domain whose LSRs do not perform 'TTL-decrement' and the upstream peer is within that domain, R MUST reset the hop count to 1 before propagating the message. |
| `3.4.4.1:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates the otherwise-increment rule for a propagated Label Mapping that site 2.8.2:5 maps | o Otherwise, R MUST increment the received hop count. |
| `3.5.3:10` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | another Reserved field of the session parameter TLVs carrying the identical zero-on-transmit, ignore-on-receipt obligation | It MUST be set to zero on transmission and ignored on receipt. |
| `3.5.3:16` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | another Reserved field of the session parameter TLVs carrying the identical zero-on-transmit, ignore-on-receipt obligation | It MUST be set to zero on transmission and ignored on receipt. |
| `3.5.3:17` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Frame Relay Session Parameters repeat the ATM label range intersection sentence word for word | A receiving LSR MUST calculate the intersection between the received range and its own supported label range. |
| `3.5.3:18` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Frame Relay Session Parameters repeat the NULL-intersection prohibition word for word | LSRs MUST NOT establish a session with neighbors for which the intersection of ranges is NULL. |
| `3.5.3:19` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Frame Relay Session Parameters repeat the Session Rejected/Parameters Label Range sentence word for word | In this case, the LSR MUST send a Session Rejected/Parameters Label Range Notification message in response to the Initialization message and not establish the session. |
| `3.5.3:20` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | another Reserved field of the session parameter TLVs carrying the identical zero-on-transmit, ignore-on-receipt obligation | It MUST be set to zero on transmission and ignored on receipt. |
| `3.6.1.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the vendor-private TLV section repeats the unknown-TLV U-bit rule of the TLV encoding section word for word | Upon receipt of an unknown TLV, if U is clear (=0), a notification MUST be returned to the message originator and the entire message MUST be ignored; if U is set (=1), the unknown TLV is silently ignored and the rest of the message is processed as if the unknown TLV did not exist. |
| `A.1.14:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the detect-change-in-FEC-next-hop algorithm restates the Appendix A obligation to withdraw a broken FEC label binding from the peers it was sent to | An LSR that does so MUST send a Label Withdraw message for the FEC to the peer. |

## Superseded

No document obsoletes RFC 5036, so its obligations are stated where they were written.
