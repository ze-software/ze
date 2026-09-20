# RFC 5036 - LDP Specification

Experimental. Every requirement this repository extracted from RFC 5036, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 53.3% | 8 of 15 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 15 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 15 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 18.8% | 3 of 16 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 15 | of 19 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 15 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 15 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 15 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 15 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 46.7% | 7 of 15 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 15 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 19 |
| Gated MUST-level | 15 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 7 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 16 |
| Tagged units | 16 |
| Recorded audit verdicts | 0 |
| Discrimination records | 3 |
| Summary | `rfc/short/rfc5036.md` |
| Requirement shard | `rfc/requirements/rfc5036.md` |
| RFC text | `rfc/full/rfc5036.txt` |

## Enrolment

Enrolled: LDP Specification

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

Basic Discovery, active-only TCP session FSM with Initialization/KeepAlive negotiation, label information base, downstream-unsolicited Label Mapping advertisement and reception, kernel MPLS integration.

**What the ledger says remains**

Seven MUST gaps gated in [`rfc/short/rfc5036.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc5036.md).

- **Messages ze emits on one path only:** [`RFC5036-2.5.3-2`](#rfc5036-2.5.3-2) and [`RFC5036-3.5.1-1`](#rfc5036-3.5.1-1) (a Notification NAKs an unacceptable Initialization, and every other fatal error still closes the session silently), [`RFC5036-2.6.1.3-1`](#rfc5036-2.6.1.3-1) (no Label Release on a received withdraw), [`RFC5036-2.6.1.2-1`](#rfc5036-2.6.1.2-1) (SendLabelWithdraw at session.go has no production caller, so a local binding is never withdrawn on the wire).
- **Session checks ze omits:** [`RFC5036-2.5.1-4`](#rfc5036-2.5.1-4) (handleInit at session.go overwrites the expected peer LSR ID instead of comparing it), [`RFC5036-2.5.1-3`](#rfc5036-2.5.1-3) (the establishment KeepAlive at register.go is unconditional, not a response to an accepted Initialization).
- **Forwarding:** [`RFC5036-2.7-1`](#rfc5036-2.7-1) (ProgramPush at fib.go imposes labels without checking that MPLS forwarding is enabled on the interface).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 8 | one part of the gated population |
| Annotated instead of tested | 7 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **15** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (8):** [`RFC5036-x-1`](#rfc5036-x-1), [`RFC5036-x-2`](#rfc5036-x-2), [`RFC5036-x-3`](#rfc5036-x-3), [`RFC5036-2.5.1-1`](#rfc5036-2.5.1-1), [`RFC5036-2.5.3-1`](#rfc5036-2.5.3-1), [`RFC5036-2.5.1-2`](#rfc5036-2.5.1-2), [`RFC5036-3.5.3-1`](#rfc5036-3.5.3-1), [`RFC5036-3.5.2-1`](#rfc5036-3.5.2-1)

**Annotated instead of tested (7):** [`RFC5036-2.6.1.2-1`](#rfc5036-2.6.1.2-1), [`RFC5036-2.6.1.3-1`](#rfc5036-2.6.1.3-1), [`RFC5036-2.5.1-3`](#rfc5036-2.5.1-3), [`RFC5036-2.5.1-4`](#rfc5036-2.5.1-4), [`RFC5036-2.5.3-2`](#rfc5036-2.5.3-2), [`RFC5036-2.7-1`](#rfc5036-2.7-1), [`RFC5036-3.5.1-1`](#rfc5036-3.5.1-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5036-x-1` | Version field in PDU header must be 1 (Wire Format) | MUST | x | **positive:** `unit/verify` [`TestRFC5036PDUVersionOneAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L126). **negative:** `unit/verify` [`TestRFC5036PDUVersionOtherRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L153) |
| `RFC5036-x-2` | Reserved bits in Common Hello Parameters TLV must be zero (Discovery) | MUST | x | **positive:** `unit/verify` [`TestRFC5036HelloReservedBitsZeroOnTransmit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L178). **negative:** `unit/verify` [`TestRFC5036HelloReservedBitsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L200) |
| `RFC5036-x-3` | Protocol Version in Common Session Parameters must be 1 (Sessions) | MUST | x | **positive:** `unit/verify` [`TestRFC5036InitProtocolVersionOne`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L223). **negative:** `unit/verify` [`TestRFC5036InitProtocolVersionOtherRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L257) |
| `RFC5036-2.5.1-1` | An LSR MUST send the Initialization message to start a session (§2.5.1) | MUST | 2.5.1 | **positive:** `unit/verify` [`TestRFC5036SessionSendsInitializationFirst`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L403). **negative:** `unit/verify` [`TestRFC5036SessionNotOperationalWithoutOwnInit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L426) |
| `RFC5036-2.5.3-1` | An LSR MUST periodically send KeepAlive messages on established sessions (§2.5.3) | MUST | 2.5.3 | **positive:** `unit/verify` [`TestRFC5036KeepalivesSentPeriodically`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L514). **negative:** `unit/verify` [`TestRFC5036KeepalivesNotSentContinuously`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L540) |
| `RFC5036-2.6.1.2-1` | An LSR MUST send a Label Withdraw message when a previously advertised binding is no longer valid (§2.6.1.2) | MUST | 2.6.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the encoder and sender exist (internal/plugins/ldp/session.go:326 SendLabelWithdraw, internal/plugins/ldp/wire.go:499 EncodeLabelWithdraw) but nothing invokes them -- a local binding is created once in OnStarted (internal/plugins/ldp/register.go:312) and released only by RemovePop at engine exit (internal/plugins/ldp/register.go:380), so no Label Withdraw ever reaches the wire |
| `RFC5036-2.6.1.3-1` | An LSR MUST send a Label Release message when it no longer needs a label (§2.6.1.3) | MUST | 2.6.1.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the withdraw handler drops the binding and reconciles forwarding without replying (internal/plugins/ldp/register.go:821 the onWithdraw callback, internal/plugins/ldp/fib.go:48 withdrawRemoteBinding); MsgTypeLabelRelease has no encoder and an inbound one is discarded at internal/plugins/ldp/session.go:463 |
| `RFC5036-2.5.1-2` | An LSR MUST accept the lower of the two proposed KeepAlive Timer values during negotiation (§2.5.1) | MUST | 2.5.1 | **positive:** `unit/verify` [`TestRFC5036KeepaliveNegotiationAdoptsLower`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L459). **negative:** `unit/verify` [`TestRFC5036KeepaliveNegotiationRefusesHigher`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L475) |
| `RFC5036-3.5.3-1` | The Common Session Parameters KeepAlive Time is a two octet unsigned NON ZERO integer, so an Initialization proposing 0 MUST be rejected with the Session Rejected/Bad KeepAlive Time Notification (0x00000018) and the session MUST NOT be established (§3.5.3, §3.9) | MUST | 3.5.3 | **positive:** `unit/verify` [`TestRFC5036InitNonZeroKeepaliveTimeAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L318). **negative:** `unit/verify` [`TestRFC5036InitZeroKeepaliveTimeRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L355) |
| `RFC5036-2.5.1-3` | An LSR MUST respond to a received Initialization with a KeepAlive message if parameters are acceptable (§2.5.1) | MUST | 2.5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handleInit (internal/plugins/ldp/session.go:504) applies the negotiated parameters and advances the FSM without emitting anything; the only establishment KeepAlive is the unconditional one at internal/plugins/ldp/register.go:758, sent right after ze's own Initialization and before the peer's arrives, so no KeepAlive is conditioned on receiving and accepting an Initialization |
| `RFC5036-3.5.2-1` | A Common Hello Parameters Hold Time of 0 means use the default hold time -- 15 seconds for Link Hellos, 45 seconds for Targeted Hellos -- and the adjacency is kept, not removed (§3.5.2) | MUST | 3.5.2 | **positive:** `unit/verify` [`TestRFC5036HelloHoldTimeZeroUsesDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L579). **negative:** `unit/verify` [`TestRFC5036HelloHoldTimeNonZeroNotDefaulted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L619) |
| `RFC5036-2.5.1-4` | The LSR MUST check that the LSR ID matches what was expected in the Initialization (§2.5.1) | MUST | 2.5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handleInit overwrites the expected peer LSR ID with whatever the PDU header carried (internal/plugins/ldp/session.go:508 `s.peerLSRID = peerLSRID`) instead of comparing it to the value learned from the Hello, and the Receiver LSR ID decoded from the Common Session Parameters TLV (internal/plugins/ldp/wire.go:338) is never compared to the local LSR ID |
| `RFC5036-2.5.3-2` | An LSR MUST send a Notification message for fatal errors that require session teardown (§2.5.3) | MUST | 2.5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** partly met. An unacceptable Initialization IS now NAK'd -- rejectInit (internal/plugins/ldp/session.go) sends the Notification through encodeNotification (internal/plugins/ldp/wire.go) for Bad Protocol Version and Session Rejected/Bad KeepAlive Time -- but every other fatal error still returns the error and closes the connection with no Notification: keepalive expiry and decode failure in ReadLoop and processMessages (internal/plugins/ldp/session.go), and the session-ended log in register.go |
| `RFC5036-2.7-1` | An LSR MUST NOT send labeled packets on a link until MPLS forwarding has been enabled on that interface (§2.7) | MUST NOT | 2.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ProgramPush (internal/plugins/ldp/fib.go:128) emits the label-imposition entry for every accepted binding with no check that MPLS forwarding is enabled on the outgoing interface; enabling it is operator config carried by iface (internal/component/iface/config_sysctl.go:71 net.mpls.conf.<iface>.input) and LDP reads no such state |
| `RFC5036-3.5.1-1` | An LSR MUST NOT process any further messages after sending a fatal Notification (§3.5.1) | MUST NOT | 3.5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** partly met. On the Initialization path the obligation now holds end to end -- rejectInit (internal/plugins/ldp/session.go) sends the fatal Notification and returns the cause, processMessages stops on it and ReadLoop propagates it, so no further message from that peer is processed. Every other fatal error still sends no Notification, so the obligation's trigger has no producer there; it is unmet for the same reason as RFC5036-2.5.3-2 |
| `RFC5036-2.9-1` | An LSR SHOULD use TCP MD5 Authentication for session protection (§2.9) | SHOULD | 2.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC5036-2.6.1.1-1` | An LSR SHOULD advertise labels for all FECs in downstream unsolicited mode (§2.6.1.1) | SHOULD | 2.6.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5036-2.8-1` | An LSR MAY use loop detection mechanisms (hop count, path vector) (§2.8) | MAY | 2.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC5036-2.4.1-1` | An LSR MAY use Extended Discovery for non-adjacent peers (§2.4.1) | MAY | 2.4.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5036-2.6.1.2-1`](#rfc5036-2.6.1.2-1) An LSR MUST send a Label Withdraw message when a previously advertised binding is no longer valid (§2.6.1.2) | {gap}, no test | the encoder and sender exist (internal/plugins/ldp/session.go:326 SendLabelWithdraw, internal/plugins/ldp/wire.go:499 EncodeLabelWithdraw) but nothing invokes them -- a local binding is created once in OnStarted (internal/plugins/ldp/register.go:312) and released only by RemovePop at engine exit (internal/plugins/ldp/register.go:380), so no Label Withdraw ever reaches the wire |
| [`RFC5036-2.6.1.3-1`](#rfc5036-2.6.1.3-1) An LSR MUST send a Label Release message when it no longer needs a label (§2.6.1.3) | {gap}, no test | the withdraw handler drops the binding and reconciles forwarding without replying (internal/plugins/ldp/register.go:821 the onWithdraw callback, internal/plugins/ldp/fib.go:48 withdrawRemoteBinding); MsgTypeLabelRelease has no encoder and an inbound one is discarded at internal/plugins/ldp/session.go:463 |
| [`RFC5036-2.5.1-3`](#rfc5036-2.5.1-3) An LSR MUST respond to a received Initialization with a KeepAlive message if parameters are acceptable (§2.5.1) | {gap}, no test | handleInit (internal/plugins/ldp/session.go:504) applies the negotiated parameters and advances the FSM without emitting anything; the only establishment KeepAlive is the unconditional one at internal/plugins/ldp/register.go:758, sent right after ze's own Initialization and before the peer's arrives, so no KeepAlive is conditioned on receiving and accepting an Initialization |
| [`RFC5036-2.5.1-4`](#rfc5036-2.5.1-4) The LSR MUST check that the LSR ID matches what was expected in the Initialization (§2.5.1) | {gap}, no test | handleInit overwrites the expected peer LSR ID with whatever the PDU header carried (internal/plugins/ldp/session.go:508 `s.peerLSRID = peerLSRID`) instead of comparing it to the value learned from the Hello, and the Receiver LSR ID decoded from the Common Session Parameters TLV (internal/plugins/ldp/wire.go:338) is never compared to the local LSR ID |
| [`RFC5036-2.5.3-2`](#rfc5036-2.5.3-2) An LSR MUST send a Notification message for fatal errors that require session teardown (§2.5.3) | {gap}, no test | partly met. An unacceptable Initialization IS now NAK'd -- rejectInit (internal/plugins/ldp/session.go) sends the Notification through encodeNotification (internal/plugins/ldp/wire.go) for Bad Protocol Version and Session Rejected/Bad KeepAlive Time -- but every other fatal error still returns the error and closes the connection with no Notification: keepalive expiry and decode failure in ReadLoop and processMessages (internal/plugins/ldp/session.go), and the session-ended log in register.go |
| [`RFC5036-2.7-1`](#rfc5036-2.7-1) An LSR MUST NOT send labeled packets on a link until MPLS forwarding has been enabled on that interface (§2.7) | {gap}, no test | ProgramPush (internal/plugins/ldp/fib.go:128) emits the label-imposition entry for every accepted binding with no check that MPLS forwarding is enabled on the outgoing interface; enabling it is operator config carried by iface (internal/component/iface/config_sysctl.go:71 net.mpls.conf.<iface>.input) and LDP reads no such state |
| [`RFC5036-3.5.1-1`](#rfc5036-3.5.1-1) An LSR MUST NOT process any further messages after sending a fatal Notification (§3.5.1) | {gap}, no test | partly met. On the Initialization path the obligation now holds end to end -- rejectInit (internal/plugins/ldp/session.go) sends the fatal Notification and returns the cause, processMessages stops on it and ReadLoop propagates it, so no further message from that peer is processed. Every other fatal error still sends no Notification, so the obligation's trigger has no producer there; it is unmet for the same reason as RFC5036-2.5.3-2 |

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

An LSR MUST send the Initialization message to start a session (§2.5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036SessionNotOperationalWithoutOwnInit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L426) | unit/verify | unproven |
| positive | [`TestRFC5036SessionSendsInitializationFirst`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L403) | unit/verify | unproven |

### [`RFC5036-2.5.3-1`](#rfc5036-2.5.3-1)

An LSR MUST periodically send KeepAlive messages on established sessions (§2.5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036KeepalivesNotSentContinuously`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L540) | unit/verify | unproven |
| positive | [`TestRFC5036KeepalivesSentPeriodically`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L514) | unit/verify | unproven |

### [`RFC5036-2.6.1.2-1`](#rfc5036-2.6.1.2-1)

An LSR MUST send a Label Withdraw message when a previously advertised binding is no longer valid (§2.6.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.6.1.2-1, so no unit is bound to it.

### [`RFC5036-2.6.1.3-1`](#rfc5036-2.6.1.3-1)

An LSR MUST send a Label Release message when it no longer needs a label (§2.6.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.6.1.3-1, so no unit is bound to it.

### [`RFC5036-2.5.1-2`](#rfc5036-2.5.1-2)

An LSR MUST accept the lower of the two proposed KeepAlive Timer values during negotiation (§2.5.1)

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

An LSR MUST respond to a received Initialization with a KeepAlive message if parameters are acceptable (§2.5.1)

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

The LSR MUST check that the LSR ID matches what was expected in the Initialization (§2.5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.5.1-4, so no unit is bound to it.

### [`RFC5036-2.5.3-2`](#rfc5036-2.5.3-2)

An LSR MUST send a Notification message for fatal errors that require session teardown (§2.5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.5.3-2, so no unit is bound to it.

### [`RFC5036-2.7-1`](#rfc5036-2.7-1)

An LSR MUST NOT send labeled packets on a link until MPLS forwarding has been enabled on that interface (§2.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.7-1, so no unit is bound to it.

### [`RFC5036-3.5.1-1`](#rfc5036-3.5.1-1)

An LSR MUST NOT process any further messages after sending a fatal Notification (§3.5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.5.1-1, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 5036, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 5036, so its obligations are stated where they were written.
