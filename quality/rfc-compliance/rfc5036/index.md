# RFC 5036 - LDP Specification

Experimental. Every requirement this repository extracted from RFC 5036, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 31.1% | 28 of 90 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 3.3% | 3 of 90 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 90 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 90 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 85.0% | 51 of 60 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 90 | of 94 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 90 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 90 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 90 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 90 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 65.6% | 59 of 90 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 26 | of 90 gated MUSTs judged | 2 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 90 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 94 |
| Gated MUST-level | 90 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 60 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 60 |
| Tagged units | 60 |
| Recorded audit verdicts | 26 |
| Discrimination records | 51 |
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

The 2026-09-21 extraction walk read all 87 normative sites in [`rfc/full/rfc5036.txt`](https://github.com/ze-software/ze/blob/main/rfc/full/rfc5036.txt) and grew the checklist from 19 rows to 86. Fatal Notification handling remains partial: [`RFC5036-2.5.3-2`](#rfc5036-2.5.3-2) and [`RFC5036-3.5.1-1`](#rfc5036-3.5.1-1) (unacceptable Initialization and unexpected message types in OPENSENT/OPENREC are NAK'd, but KeepAlive expiry and decoding errors outside the initialization type checks still close silently).

- **Other missing send paths:** [`RFC5036-2.6.1.3-1`](#rfc5036-2.6.1.3-1) (no Label Release on a received withdraw), [`RFC5036-2.6.1.2-1`](#rfc5036-2.6.1.2-1) (SendLabelWithdraw has no production caller, so a local binding is never withdrawn on the wire). Passive-role session procedures remain absent: [`RFC5036-2.5.1-4`](#rfc5036-2.5.1-4), [`RFC5036-2.5.1-3`](#rfc5036-2.5.1-3) and [`RFC5036-2.5.3-7`](#rfc5036-2.5.3-7) (ze dials every session and nothing listens on TCP port 646). The walk corrected four rows against the document: [`RFC5036-2.9-1`](#rfc5036-2.9-1) rose to MUST, [`RFC5036-2.6.1.1-1`](#rfc5036-2.6.1.1-1) fell to MAY, [`RFC5036-3.5.1-1`](#rfc5036-3.5.1-1) fell to SHOULD, and [`RFC5036-2.7-1`](#rfc5036-2.7-1) lost a prohibition on sending labeled packets before MPLS forwarding is enabled, which no sentence of RFC 5036 states. Ze still imposes labels without that check, at ProgramPush in [`internal/plugins/ldp/fib.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/fib.go), and the walk removed the row that recorded it, so nothing on this ledger gates it now. Remaining implementation gaps include Loop Detection procedures, ATM and Frame Relay label and session parameters, Downstream on Demand Label Request, Abort and Release procedures, and vendor-private U-bit configuration interfaces.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 28 | one part of the gated population |
| Annotated (including scoped evidence) | 62 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **90** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (28):** [`RFC5036-x-1`](#rfc5036-x-1), [`RFC5036-x-2`](#rfc5036-x-2), [`RFC5036-x-3`](#rfc5036-x-3), [`RFC5036-2.5.1-1`](#rfc5036-2.5.1-1), [`RFC5036-2.5.3-1`](#rfc5036-2.5.3-1), [`RFC5036-2.5.1-2`](#rfc5036-2.5.1-2), [`RFC5036-3.5.3-1`](#rfc5036-3.5.3-1), [`RFC5036-2.5.3-5`](#rfc5036-2.5.3-5), [`RFC5036-2.5.3-6`](#rfc5036-2.5.3-6), [`RFC5036-2.5.3-8`](#rfc5036-2.5.3-8), [`RFC5036-2.5.3-9`](#rfc5036-2.5.3-9), [`RFC5036-2.5.4-1`](#rfc5036-2.5.4-1), [`RFC5036-2.5.4-2`](#rfc5036-2.5.4-2), [`RFC5036-3.5.2-1`](#rfc5036-3.5.2-1), [`RFC5036-2.5.3-3`](#rfc5036-2.5.3-3), [`RFC5036-2.5.3-4`](#rfc5036-2.5.3-4), [`RFC5036-2.5.2-1`](#rfc5036-2.5.2-1), [`RFC5036-2.6.1.2-2`](#rfc5036-2.6.1.2-2), [`RFC5036-3.1-1`](#rfc5036-3.1-1), [`RFC5036-3.3-1`](#rfc5036-3.3-1), [`RFC5036-3.4.4.1-1`](#rfc5036-3.4.4.1-1), [`RFC5036-3.4.4.1-2`](#rfc5036-3.4.4.1-2), [`RFC5036-3.5.3-3`](#rfc5036-3.5.3-3), [`RFC5036-3.5.3-5`](#rfc5036-3.5.3-5), [`RFC5036-3.5.3-6`](#rfc5036-3.5.3-6), [`RFC5036-3.5.3-7`](#rfc5036-3.5.3-7), [`RFC5036-3.5.3-9`](#rfc5036-3.5.3-9), [`RFC5036-3.5.4.1-1`](#rfc5036-3.5.4.1-1)

**Annotated (including scoped evidence) (62):** [`RFC5036-2.6.1.2-1`](#rfc5036-2.6.1.2-1), [`RFC5036-2.6.1.3-1`](#rfc5036-2.6.1.3-1), [`RFC5036-2.5.1-3`](#rfc5036-2.5.1-3), [`RFC5036-2.5.3-7`](#rfc5036-2.5.3-7), [`RFC5036-2.5.1-4`](#rfc5036-2.5.1-4), [`RFC5036-2.5.3-2`](#rfc5036-2.5.3-2), [`RFC5036-2.7-1`](#rfc5036-2.7-1), [`RFC5036-2.9-1`](#rfc5036-2.9-1), [`RFC5036-2.8.1-1`](#rfc5036-2.8.1-1), [`RFC5036-2.8.1-2`](#rfc5036-2.8.1-2), [`RFC5036-2.8.1-3`](#rfc5036-2.8.1-3), [`RFC5036-2.8.1-4`](#rfc5036-2.8.1-4), [`RFC5036-2.8.1-5`](#rfc5036-2.8.1-5), [`RFC5036-2.8.1-6`](#rfc5036-2.8.1-6), [`RFC5036-2.8.1-7`](#rfc5036-2.8.1-7), [`RFC5036-2.8.2-1`](#rfc5036-2.8.2-1), [`RFC5036-2.8.2-2`](#rfc5036-2.8.2-2), [`RFC5036-2.8.2-3`](#rfc5036-2.8.2-3), [`RFC5036-2.8.2-4`](#rfc5036-2.8.2-4), [`RFC5036-2.8.2-5`](#rfc5036-2.8.2-5), [`RFC5036-2.8.2-6`](#rfc5036-2.8.2-6), [`RFC5036-2.8.2-7`](#rfc5036-2.8.2-7), [`RFC5036-2.8.2-8`](#rfc5036-2.8.2-8), [`RFC5036-2.8.2-9`](#rfc5036-2.8.2-9), [`RFC5036-2.8.2-10`](#rfc5036-2.8.2-10), [`RFC5036-2.8.2-11`](#rfc5036-2.8.2-11), [`RFC5036-2.8.2-12`](#rfc5036-2.8.2-12), [`RFC5036-2.8.2-13`](#rfc5036-2.8.2-13), [`RFC5036-3.4.2.2-1`](#rfc5036-3.4.2.2-1), [`RFC5036-3.4.2.2-2`](#rfc5036-3.4.2.2-2), [`RFC5036-3.4.2.2-3`](#rfc5036-3.4.2.2-3), [`RFC5036-3.4.2.3-1`](#rfc5036-3.4.2.3-1), [`RFC5036-3.4.4.1-3`](#rfc5036-3.4.4.1-3), [`RFC5036-3.4.5.1.1-1`](#rfc5036-3.4.5.1.1-1), [`RFC5036-3.4.5.1.1-2`](#rfc5036-3.4.5.1.1-2), [`RFC5036-3.4.5.1.1-3`](#rfc5036-3.4.5.1.1-3), [`RFC5036-3.4.5.1.2-1`](#rfc5036-3.4.5.1.2-1), [`RFC5036-3.4.5.1.2-2`](#rfc5036-3.4.5.1.2-2), [`RFC5036-3.4.5.1.2-3`](#rfc5036-3.4.5.1.2-3), [`RFC5036-3.5-1`](#rfc5036-3.5-1), [`RFC5036-3.5.3-2`](#rfc5036-3.5.3-2), [`RFC5036-3.5.3-4`](#rfc5036-3.5.3-4), [`RFC5036-3.5.3-8`](#rfc5036-3.5.3-8), [`RFC5036-3.5.3-10`](#rfc5036-3.5.3-10), [`RFC5036-3.5.3-11`](#rfc5036-3.5.3-11), [`RFC5036-3.5.3-12`](#rfc5036-3.5.3-12), [`RFC5036-3.5.3-13`](#rfc5036-3.5.3-13), [`RFC5036-3.5.3-14`](#rfc5036-3.5.3-14), [`RFC5036-3.5.7-1`](#rfc5036-3.5.7-1), [`RFC5036-3.5.8.1-1`](#rfc5036-3.5.8.1-1), [`RFC5036-3.5.8.1-2`](#rfc5036-3.5.8.1-2), [`RFC5036-3.5.8.1-3`](#rfc5036-3.5.8.1-3), [`RFC5036-3.5.8.1-4`](#rfc5036-3.5.8.1-4), [`RFC5036-3.5.9.1-1`](#rfc5036-3.5.9.1-1), [`RFC5036-3.5.9.1-2`](#rfc5036-3.5.9.1-2), [`RFC5036-3.5.9.1-3`](#rfc5036-3.5.9.1-3), [`RFC5036-3.5.11.1-1`](#rfc5036-3.5.11.1-1), [`RFC5036-3.6.1.1-1`](#rfc5036-3.6.1.1-1), [`RFC5036-3.6.1.2-1`](#rfc5036-3.6.1.2-1), [`RFC5036-A.1.2-1`](#rfc5036-a.1.2-1), [`RFC5036-A.1.7-1`](#rfc5036-a.1.7-1), [`RFC5036-A.1-1`](#rfc5036-a.1-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5036-x-1` | Two octet unsigned integer containing the version number of the protocol. This version of the specification specifies LDP protocol version 1. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC5036PDUVersionOneAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L135). **negative:** `unit/verify` [`TestRFC5036PDUVersionOtherRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L162) |
| `RFC5036-x-2` | This field is reserved. It MUST be set to zero on transmission and ignored on receipt. (§3.5.2) | MUST | 3.5.2 | **positive:** `unit/verify` [`TestRFC5036HelloReservedBitsZeroOnTransmit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L187). **negative:** `unit/verify` [`TestRFC5036HelloReservedBitsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L209) |
| `RFC5036-x-3` | Two octet unsigned integer containing the version number of the protocol. This version of the specification specifies LDP protocol version 1. (§3.5.3) | MUST | 3.5.3 | **positive:** `unit/verify` [`TestRFC5036InitProtocolVersionOne`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L232). **negative:** `unit/verify` [`TestRFC5036InitProtocolVersionOtherRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L266) |
| `RFC5036-2.5.1-1` | After the connection is established, if LSR1 is playing the active role, it initiates negotiation of session parameters by sending an Initialization message to LSR2. (§2.5.3) | MUST | 2.5.3 | **positive:** `unit/verify` [`TestRFC5036ActiveRoleInitiatesNegotiation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_establishment_test.go#L98). **negative:** `unit/verify` [`TestRFC5036SessionNotOperationalWithoutOwnInit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L436) |
| `RFC5036-2.5.3-1` | An LSR MUST arrange that its peer receive an LDP message from it at least every KeepAlive Time period. Any LDP protocol message will do but, in circumstances where no other LDP protocol messages have been sent within the period, a KeepAlive message MUST be sent. (§3.5.4.1) | MUST | 3.5.4.1 | **positive:** `unit/verify` [`TestRFC5036KeepalivesSentPeriodically`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L525). **negative:** `unit/verify` [`TestRFC5036KeepalivePacingFollowsNegotiatedTime`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L408) |
| `RFC5036-2.6.1.2-1` | Whenever an LSR breaks the binding between a label and a FEC, it MUST withdraw the FEC label mapping from all LDP peers to which it has previously sent the mapping. (§A) | MUST | A | **positive:** no positive test. **negative:** no negative test. **{gap}:** the encoder and sender exist (internal/plugins/ldp/session.go:326 SendLabelWithdraw, internal/plugins/ldp/wire.go:499 EncodeLabelWithdraw) but nothing invokes them -- a local binding is created once in OnStarted (internal/plugins/ldp/register.go:312) and released only by RemovePop at engine exit (internal/plugins/ldp/register.go:380), so no Label Withdraw ever reaches the wire |
| `RFC5036-2.6.1.3-1` | An LSR that receives a Label Withdraw message MUST respond with a Label Release message. (§3.5.10.1) | MUST | 3.5.10.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the withdraw handler drops the binding and reconciles forwarding without replying (internal/plugins/ldp/register.go:821 the onWithdraw callback, internal/plugins/ldp/fib.go:48 withdrawRemoteBinding); MsgTypeLabelRelease has no encoder and an inbound one is discarded at internal/plugins/ldp/session.go:463 |
| `RFC5036-2.5.1-2` | The receiving LSR MUST calculate the value of the KeepAlive Timer by using the smaller of its proposed KeepAlive Time and the KeepAlive Time received in the PDU. (§3.5.3) | MUST | 3.5.3 | **positive:** `unit/verify` [`TestRFC5036KeepaliveNegotiationAdoptsLower`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L469). **negative:** `unit/verify` [`TestRFC5036KeepaliveNegotiationRefusesHigher`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L485) |
| `RFC5036-3.5.3-1` | Two octet unsigned non zero integer that indicates the number of seconds that the sending LSR proposes for the value of the KeepAlive Time. (§3.5.3) | MUST | 3.5.3 | **positive:** `unit/verify` [`TestRFC5036InitNonZeroKeepaliveTimeAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L327). **negative:** `unit/verify` [`TestRFC5036InitZeroKeepaliveTimeRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L364) |
| `RFC5036-2.5.1-3` | Next LSR1 checks whether the session parameters proposed in the message are acceptable. If they are, LSR1 replies with an Initialization message of its own to propose the parameters it wishes to use and a KeepAlive message to signal acceptance of LSR2's parameters. (§2.5.3) | MUST | 2.5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze has no passive role. startSessionForAdj (internal/plugins/ldp/register.go) dials every session and nothing listens on TCP port 646, so ze never receives an Initialization before it has sent its own and never replies with one. The active role's reply, a KeepAlive to an acceptable Initialization, is RFC5036-2.5.3-5 |
| `RFC5036-2.5.3-5` | If LSR1 receives an Initialization message, it checks whether the session parameters are acceptable. If so, it replies with a KeepAlive message. (§2.5.3) | MUST | 2.5.3 | **positive:** `unit/verify` [`TestRFC5036ActiveRoleAcceptableInitAnsweredWithKeepAlive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_establishment_test.go#L129). **negative:** `unit/verify` [`TestRFC5036ActiveRoleNoKeepAliveWithoutAcceptableInit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_establishment_test.go#L159) |
| `RFC5036-2.5.3-6` | If the session parameters are unacceptable, LSR1 sends a Session Rejected/Parameters Error Notification message and closes the connection. (§2.5.3) | MUST | 2.5.3 | **positive:** `unit/verify` [`TestRFC5036ActiveRoleUnacceptableInitRejectedAndClosed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_establishment_test.go#L192). **negative:** `unit/verify` [`TestRFC5036ActiveRoleAcceptableInitKeepsConnection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_establishment_test.go#L242) |
| `RFC5036-2.5.3-7` | If the parameters are not acceptable, LSR1 responds by sending a Session Rejected/Parameters Error Notification message and closing the TCP connection. (§2.5.3) | MUST | 2.5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** this is the passive role's step, and ze has no passive role: startSessionForAdj (internal/plugins/ldp/register.go) dials every session and nothing listens on TCP port 646, so no Initialization reaches ze on a connection the peer opened. The active role's refusal is RFC5036-2.5.3-6 |
| `RFC5036-2.5.3-8` | If LSR1 receives a KeepAlive message, LSR2 has accepted its proposed session parameters. (§2.5.3) | MUST | 2.5.3 | **positive:** `unit/verify` [`TestRFC5036OpenReceivedKeepAliveEstablishesSession`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_openrec_test.go#L13). **negative:** `unit/verify` [`TestRFC5036OpenReceivedWaitsForPeerKeepAlive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_openrec_test.go#L39) |
| `RFC5036-2.5.3-9` | When LSR1 has received both an acceptable Initialization message and a KeepAlive message, the session is operational from LSR1's point of view. (§2.5.3) | MUST | 2.5.3 | **positive:** `unit/verify` [`TestRFC5036OpenReceivedKeepAliveEstablishesSession`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_openrec_test.go#L15). **negative:** `unit/verify` [`TestRFC5036KeepAliveWithoutPeerInitializationNeverEstablishes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_openrec_test.go#L56). **negative:** `unit/verify` [`TestRFC5036OpenReceivedWaitsForPeerKeepAlive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_openrec_test.go#L41) |
| `RFC5036-2.5.4-1` | OPENREC Receive KeepAlive msg OPERATIONAL Receive Any other LDP msg NON EXISTENT Action: Transmit Error Notification msg (NAK) and close transport connection (§2.5.4) | MUST | 2.5.4 | **positive:** `unit/verify` [`TestRFC5036OpenReceivedOtherMessageRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_openrec_test.go#L89). **negative:** `unit/verify` [`TestRFC5036OpenReceivedKeepAliveEstablishesSession`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_openrec_test.go#L17) |
| `RFC5036-2.5.4-2` | OPENSENT Receive acceptable OPENREC Initialization msg Action: Transmit KeepAlive msg Receive Any other LDP msg NON EXISTENT Action: Transmit Error Notification msg (NAK) and close transport connection (§2.5.4) | MUST | 2.5.4 | **positive:** `unit/verify` [`TestRFC5036OpenSentOtherMessageRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_openrec_test.go#L101). **negative:** `unit/verify` [`TestRFC5036OpenReceivedKeepAliveEstablishesSession`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_openrec_test.go#L19) |
| `RFC5036-3.5.2-1` | A value of 0 means use the default, which is 15 seconds for Link Hellos and 45 seconds for Targeted Hellos. (§3.5.2) | MUST | 3.5.2 | **positive:** `unit/verify` [`TestRFC5036HelloHoldTimeZeroUsesDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L584). **negative:** `unit/verify` [`TestRFC5036HelloHoldTimeNonZeroNotDefaulted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L624) |
| `RFC5036-2.5.1-4` | If LSR1 receives an Initialization message, it attempts to match the LDP Identifier carried by the message PDU with a Hello adjacency. (§2.5.3) | MUST | 2.5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handleInit overwrites the expected peer LSR ID with whatever the PDU header carried (internal/plugins/ldp/session.go:508 `s.peerLSRID = peerLSRID`) instead of comparing it to the value learned from the Hello, and the Receiver LSR ID decoded from the Common Session Parameters TLV (internal/plugins/ldp/wire.go:338) is never compared to the local LSR ID |
| `RFC5036-2.5.3-2` | If an LSR encounters a condition requiring it to notify its peer with advisory or error information, it sends the peer a Notification message containing a Status TLV that encodes the information and optionally additional TLVs that provide more information about the condition. If the condition is one that is a fatal error, the Status Code carried in the Notification will indicate that. (§3.5.1.1) | MUST | 3.5.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** partly met. An unacceptable Initialization is NAK'd by rejectInit, and processMessages sends fatal Shutdown for every unexpected complete message type in OPENSENT/OPENREC. KeepAlive expiry and decoding errors outside those state checks still return an error and close with no Notification in ReadLoop and processMessages (internal/plugins/ldp/session.go). The separate spec-ldp-keepalive-expiry-notification owns only the expiry cause |
| `RFC5036-2.7-1` | To retrieve the label, the LSR must be able to map the next hop address for the prefix to an LDP Identifier. Similarly, when the LSR learns a label for a prefix from an LDP peer, it must be able to determine whether that peer is currently a next hop for the prefix to determine whether it needs to start using the newly learned label when forwarding packets that match the prefix. To make that decision, the LSR must be able to map an LDP Identifier to the peer's addresses to check whether any are a next hop for the prefix. To enable LSRs to map between a peer LDP Identifier and the peer's addresses, LSRs advertise their addresses using LDP Address and Withdraw Address messages. (§2.7) | MUST | 2.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze decodes Address messages but never sends one; plan/pre-release/spec-ldp-address-advertisement.md |
| `RFC5036-3.5.1-1` | If the condition is one that is a fatal error, the Status Code carried in the Notification will indicate that. In this case, after sending the Notification message the LSR SHOULD terminate the LDP session by closing the session TCP connection and discard all state associated with the session, including all label-FEC bindings learned via the session. (§3.5.1.1) | SHOULD | 3.5.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** partly met. An unacceptable Initialization and an unexpected complete message type in OPENSENT/OPENREC receive fatal Notifications; processMessages then returns an error, ReadLoop ends and runSession closes. KeepAlive expiry and decoding errors outside the initialization type checks still send no Notification, leaving the same remaining causes as RFC5036-2.5.3-2 |
| `RFC5036-2.9-1` | This section specifies a mechanism to protect against the introduction of spoofed TCP segments into LDP session connection streams. The use of this mechanism MUST be supported as a configurable option. (§2.9) | MUST | 2.9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze reads no LDP password and sets no TCP_MD5SIG; plan/pre-release/spec-ldp-md5-authentication.md |
| `RFC5036-2.6.1.1-1` | When using independent LSP control, each LSR may advertise label mappings to its neighbors at any time it desires. For example, when operating in independent Downstream on Demand mode, an LSR may answer requests for label mappings immediately, without waiting for a label mapping from the next hop. When operating in independent Downstream Unsolicited mode, an LSR may advertise a label mapping for a FEC to its neighbors whenever it is prepared to label-switch that FEC. (§2.6.1.1) | MAY | 2.6.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5036-2.8-1` | Loop Detection is a configurable option that provides a mechanism for finding looping LSPs and for preventing Label Request messages from looping in the presence of non-merge capable LSRs. The mechanism makes use of Path Vector and Hop Count TLVs carried by Label Request and Label Mapping messages. (§2.8) | MAY | 2.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC5036-2.4.1-1` | LDP sessions between non-directly connected LSRs are supported by LDP Extended Discovery. (§2.4.2) | MAY | 2.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5036-2.5.3-3` | An LSR MUST throttle its session setup retry attempts with an exponential backoff in situations where Initialization messages are being NAK'd (§2.5.3) | MUST | 2.5.3 | **positive:** `unit/verify` [`TestRFC5036SetupRetryBackoffGrows`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L322). **negative:** `unit/verify` [`TestRFC5036SetupRetryRefusedInsideDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L346) |
| `RFC5036-2.5.3-4` | The session establishment setup attempt following a NAK'd Initialization message MUST be delayed no less than 15 seconds, and subsequent delays MUST grow to a maximum delay of no less than 2 minutes (§2.5.3) | MUST | 2.5.3 | **positive:** `unit/verify` [`TestRFC5036SetupRetryBackoffGrows`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L324). **negative:** `unit/verify` [`TestRFC5036SetupRetryRefusedInsideDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L348) |
| `RFC5036-2.5.2-1` | An LSR MUST advertise the same transport address in all Hellos that advertise the same label space (§2.5.2) | MUST | 2.5.2 | **positive:** `unit/verify` [`TestRFC5036HellosCarryOneTransportAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L491). **negative:** `unit/verify` [`TestRFC5036HelloTransportAddressNeverFollowsTheSocket`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L509) |
| `RFC5036-2.6.1.2-2` | For each FEC for which the LSR is not the egress and no mapping exists, the LSR MUST wait until a label from a downstream LSR is received before mapping the FEC and passing corresponding labels to upstream LSRs. (§2.6.1.2) | MUST | 2.6.1.2 | **positive:** `unit/verify` [`TestRFC5036EgressFECMappedWithoutWaiting`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L469). **negative:** `unit/verify` [`TestRFC5036NonEgressFECNotMappedBeforeDownstreamLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L498) |
| `RFC5036-2.8.1-1` | - The Label Request message MUST include a Hop Count TLV. (§2.8.1) | MUST | 2.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.1-2` | - If R is sending the Label Request because it is a FEC ingress, it MUST include a Hop Count TLV with hop count value 1. (§2.8.1) | MUST | 2.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.1-3` | - If R is sending the Label Request as a result of having received a Label Request from an upstream LSR, and if the received Label Request contains a Hop Count TLV, R MUST increment the received hop count value by 1 and MUST pass the resulting value in a Hop Count TLV to its next hop along with the Label Request message. (§2.8.1) | MUST | 2.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.1-4` | - If R is sending the Label Request because it is a FEC ingress, then if R is non-merge capable, it MUST include a Path Vector TLV of length 1 containing its own LSR Id. (§2.8.1) | MUST | 2.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.1-5` | R MUST add its own LSR Id to the Path Vector, and MUST pass the resulting Path Vector to its next hop along with the Label Request message. (§2.8.1) | MUST | 2.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.1-6` | If the Label Request contains no Path Vector TLV, R MUST include a Path Vector TLV of length 1 containing its own LSR Id. (§2.8.1) | MUST | 2.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.1-7` | When R detects a loop, it MUST send a Loop Detected Notification message to the source of the Label Request message and drop the Label Request message. (§2.8.1) | MUST | 2.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.2-1` | The rules that govern the use of the Hop Count TLV in Label Mapping messages sent by an LSR R when Loop Detection is enabled are the following: - R MUST include a Hop Count TLV. (§2.8.2) | MUST | 2.8.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.2-2` | - If R is the egress, the hop count value MUST be 1. (§2.8.2) | MUST | 2.8.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.2-3` | If the Label Mapping message is being sent to propagate a Label Mapping message received from the next hop to an upstream peer, the hop count value MUST be determined as follows: (§2.8.2) | MUST | 2.8.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.2-4` | o If R is a member of the edge set of an LSR domain whose LSRs do not perform 'TTL-decrement' (e.g., an ATM LSR domain or a Frame Relay LSR domain) and the upstream peer is within that domain, R MUST reset the hop count to 1 before propagating the message. (§2.8.2) | MUST | 2.8.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.2-5` | Otherwise, R MUST increment the hop count received from the next hop before propagating the message. (§2.8.2) | MUST | 2.8.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.2-6` | - If the Label Mapping message is not being sent to propagate a Label Mapping message, the hop count value MUST be the result of incrementing R's current knowledge of the hop count learned from previous Label Mapping messages. (§2.8.2) | MUST | 2.8.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.2-7` | - If R is sending the Label Mapping message to propagate a Label Mapping message received from the next hop to an upstream peer, then: o If R is merge capable and if R has not previously sent a Label Mapping message to the upstream peer, then it MUST include a Path Vector TLV. (§2.8.2) | MUST | 2.8.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.2-8` | o If the received message contains an unknown hop count, then R MUST include a Path Vector TLV. (§2.8.2) | MUST | 2.8.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.2-9` | o If R has previously sent a Label Mapping message to the upstream peer, then it MUST include a Path Vector TLV if the received message reports an LSP hop count increase, a change in hop count from unknown to known, or a change from known to unknown. (§2.8.2) | MUST | 2.8.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.2-10` | o If the received Label Mapping message included a Path Vector, the Path Vector sent upstream MUST be the result of adding R's LSR Id to the received Path Vector. (§2.8.2) | MUST | 2.8.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.2-11` | o If the received message had no Path Vector, the Path Vector sent upstream MUST be a Path Vector of length 1 containing R's LSR Id. (§2.8.2) | MUST | 2.8.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.2-12` | - If the Label Mapping message is not being sent to propagate a received message upstream, the Label Mapping message MUST include a Path Vector of length 1 containing R's LSR Id. (§2.8.2) | MUST | 2.8.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-2.8.2-13` | When R detects a loop, it MUST stop using the label for forwarding, drop the Label Mapping message, and signal Loop Detected status to the source of the Label Mapping message. (§2.8.2) | MUST | 2.8.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-3.1-1` | The first four octets identify the LSR and MUST be a globally unique value. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC5036DistinctLSRIDFormsAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L373). **negative:** `unit/verify` [`TestRFC5036OwnLSRIDFormsNoAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L439) |
| `RFC5036-3.3-1` | Upon receipt of an unknown TLV, if U is clear (=0), a notification MUST be returned to the message originator and the entire message MUST be ignored; if U is set (=1), the unknown TLV MUST be silently ignored and the rest of the message processed as if the unknown TLV did not exist. (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestRFC5036UnknownTLVWithUBitSetIsSkipped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L99). **negative:** `unit/verify` [`TestRFC5036UnknownTLVWithUBitClearIsRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L128) |
| `RFC5036-3.4.2.2-1` | It MUST be set to zero on transmission and MUST be ignored on receipt. (§3.4.2.2) | MUST | 3.4.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| `RFC5036-3.4.2.2-2` | If the VCI is less than 16-bits, it SHOULD be right justified in the field and the preceding bits MUST be set to 0. (§3.4.2.2) | MUST | 3.4.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| `RFC5036-3.4.2.2-3` | If Virtual Path switching is indicated in the V-bits field, then this field MUST be ignored by the receiver and set to 0 by the sender. (§3.4.2.2) | MUST | 3.4.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| `RFC5036-3.4.2.3-1` | This field is reserved. It MUST be set to zero on transmission and MUST be ignored on receipt. (§3.4.2.3) | MUST | 3.4.2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| `RFC5036-3.4.4.1-1` | If an LSR receives a message containing a Hop Count TLV, it MUST check the hop count value to determine whether the hop count has exceeded its configured maximum allowable value (§3.4.4.1) | MUST | 3.4.4.1 | **positive:** `unit/verify` [`TestRFC5036HopCountWithinMaximumIsApplied`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L174). **negative:** `unit/verify` [`TestRFC5036HopCountAboveMaximumDrawsLoopDetected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L203) |
| `RFC5036-3.4.4.1-2` | If so, it MUST behave as if the containing message has traversed a loop by sending a Notification message signaling Loop Detected in reply to the sender of the message. (§3.4.4.1) | MUST | 3.4.4.1 | **positive:** `unit/verify` [`TestRFC5036HopCountAboveMaximumDrawsLoopDetected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L206). **negative:** `unit/verify` [`TestRFC5036HopCountWithinMaximumIsApplied`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L177) |
| `RFC5036-3.4.4.1-3` | If Loop Detection is configured, the LSR MUST follow the procedures specified in Section "Loop Detection". (§3.4.4.1) | MUST | 3.4.4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-3.4.5.1.1-1` | An LSR that receives a Path Vector in a Label Request message MUST perform the procedures described in Section "Loop Detection". (§3.4.5.1.1) | MUST | 3.4.5.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-3.4.5.1.1-2` | If the LSR detects a loop, it MUST reject the Label Request message. (§3.4.5.1.1) | MUST | 3.4.5.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-3.4.5.1.1-3` | If the LSR detects a loop, it MUST reject the Label Request message. The LSR MUST: 1. Transmit a Notification message to the sending LSR signaling "Loop Detected". 2. Not propagate the Label Request message further. (§3.4.5.1.1) | MUST | 3.4.5.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-3.4.5.1.2-1` | An LSR that receives a Path Vector in a Label Mapping message MUST perform the procedures described in Section "Loop Detection". (§3.4.5.1.2) | MUST | 3.4.5.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-3.4.5.1.2-2` | If the LSR detects a loop, it MUST reject the Label Mapping message in order to prevent a forwarding loop. (§3.4.5.1.2) | MUST | 3.4.5.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-3.4.5.1.2-3` | If the LSR detects a loop, it MUST reject the Label Mapping message in order to prevent a forwarding loop. The LSR MUST: 1. Transmit a Label Release message carrying a Status TLV to the sending LSR to signal "Loop Detected". 2. Not propagate the message further. 3. Check whether the Label Mapping message is for an existing LSP. If so, the LSR must unsplice any upstream labels that are spliced to the downstream label for the FEC. (§3.4.5.1.2) | MUST | 3.4.5.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| `RFC5036-3.5-1` | For messages that have required parameters, the required parameters MUST appear in the order specified by the individual message specifications (§3.5) | MUST | 3.5 | **positive:** `unit/verify` [`TestRFC5036RequiredParametersInSpecifiedOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L336). **negative:** no negative test. **{single-polarity}:** the obligation binds the sender and ze's encoders take no input that could reorder the mandatory parameters, so no violating input exists to refuse |
| `RFC5036-3.5.3-2` | If the session is for a label-controlled ATM link or a label-controlled Frame Relay link, then Downstream on Demand MUST be used (§3.5.3) | MUST | 3.5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| `RFC5036-3.5.3-3` | Otherwise, Downstream Unsolicited MUST be used (§3.5.3) | MUST | 3.5.3 | **positive:** `unit/verify` [`TestRFC5036InitProposesDownstreamUnsolicited`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L85). **negative:** `unit/verify` [`TestRFC5036InitOnDemandProposalKeepsDownstreamUnsolicited`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L105) |
| `RFC5036-3.5.3-4` | If the label advertisement discipline determined in this way is unacceptable to an LSR, it MUST send a Session Rejected/Parameters Advertisement Mode Notification message in response to the Initialization message and not establish the session (§3.5.3) | MUST | 3.5.3 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC5036InitOnDemandProposalKeepsDownstreamUnsolicited`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L109). **{single-polarity}:** Downstream Unsolicited is the only discipline ze runs and it is always acceptable, so no unacceptable advertisement mode exists to reject |
| `RFC5036-3.5.3-5` | The configured maximum Path Vector length. MUST be 0 if Loop Detection is disabled (D = 0). (§3.5.3) | MUST | 3.5.3 | **positive:** `unit/verify` [`TestRFC5036InitPathVectorLimitZeroWithoutLoopDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L151). **negative:** `unit/verify` [`TestRFC5036EncodeInitDropsPathVectorLimitWithoutLoopDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L167) |
| `RFC5036-3.5.3-6` | This field is reserved. It MUST be set to zero on transmission and ignored on receipt. (§3.5.3) | MUST | 3.5.3 | **positive:** `unit/verify` [`TestRFC5036InitReservedBitsZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L203). **negative:** `unit/verify` [`TestRFC5036InitReservedBitsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L217) |
| `RFC5036-3.5.3-7` | The receiving LSR MUST calculate the maximum PDU length for the session by using the smaller of its and its peer's proposals for Max PDU Length (§3.5.3) | MUST | 3.5.3 | **positive:** `unit/verify` [`TestRFC5036InitMaxPDULengthTakesTheSmallerProposal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L254). **negative:** `unit/verify` [`TestRFC5036InitMaxPDULengthNeverRaisedAboveOwnProposal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L311) |
| `RFC5036-3.5.3-8` | If the maximum PDU length determined this way is unacceptable to an LSR, it MUST send a Session Rejected/Parameters Max PDU Length Notification message in response to the Initialization message and not establish the session (§3.5.3) | MUST | 3.5.3 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC5036InitMaxPDULengthTakesTheSmallerProposal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L257). **{single-polarity}:** ze accepts every Max PDU Length by taking the smaller of the two proposals, so no unacceptable value exists to reject |
| `RFC5036-3.5.3-9` | If there is no matching Hello adjacency, the LSR MUST send a Session Rejected/No Hello Notification message in response to the Initialization message and not establish the session. (§3.5.3) | MUST | 3.5.3 | **positive:** `unit/verify` [`TestRFC5036InitFromHelloAdjacencyAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L249). **negative:** `unit/verify` [`TestRFC5036InitWithoutHelloAdjacencyRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L270) |
| `RFC5036-3.5.3-10` | A receiving LSR MUST calculate the intersection between the received range and its own supported label range. (§3.5.3) | MUST | 3.5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| `RFC5036-3.5.3-11` | LSRs MUST NOT establish a session with neighbors for which the intersection of ranges is NULL. (§3.5.3) | MUST NOT | 3.5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| `RFC5036-3.5.3-12` | In this case, the LSR MUST send a Session Rejected/Parameters Label Range Notification message in response to the Initialization message and not establish the session. (§3.5.3) | MUST | 3.5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| `RFC5036-3.5.3-13` | This field is reserved. It MUST be set to zero on transmission and MUST be ignored on receipt. (§3.5.3) | MUST | 3.5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| `RFC5036-3.5.3-14` | When peer LSRs are connected indirectly by means of an ATM VP, the sending LSR SHOULD set the Minimum and Maximum VPI fields to 0, and the receiving LSR MUST ignore the Minimum and Maximum VPI fields. (§3.5.3) | MUST | 3.5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| `RFC5036-3.5.4.1-1` | An LSR MUST arrange that its peer receive an LDP message from it at least every KeepAlive Time period (§3.5.4.1) | MUST | 3.5.4.1 | **positive:** `unit/verify` [`TestRFC5036PeerHearsFromZeWithinKeepaliveTime`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L381). **negative:** `unit/verify` [`TestRFC5036KeepalivePacingFollowsNegotiatedTime`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L405) |
| `RFC5036-3.5.7-1` | Label Request Message ID If this Label Mapping message is a response to a Label Request message, it MUST include the Label Request Message ID optional parameter. (§3.5.7) | MUST | 3.5.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| `RFC5036-3.5.8.1-1` | Unless its routing table includes an entry that exactly matches the requested Prefix, the LSR MUST respond with a No Route Notification message. (§3.5.8.1) | MUST | 3.5.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| `RFC5036-3.5.8.1-2` | When the receiving LSR responds with a Label Mapping message, the mapping message MUST include a Label Request/Returned Message ID TLV optional parameter that includes the message ID of the Label Request message. (§3.5.8.1) | MUST | 3.5.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| `RFC5036-3.5.8.1-3` | When resources become available, the LSR MUST notify the requesting LSR by sending a Notification message with the Label Resources Available Status Code. (§3.5.8.1) | MUST | 3.5.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| `RFC5036-3.5.8.1-4` | An LSR that receives a No Label Resources response to a Label Request message MUST NOT issue further Label Request messages until it receives a Notification message with the Label Resources Available Status Code (§3.5.8.1) | MUST NOT | 3.5.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| `RFC5036-3.5.9.1-1` | When an LSR receives a Label Abort Request message, if it has not previously responded to the Label Request being aborted with a Label Mapping message or some other Notification message, it MUST acknowledge the abort by responding with a Label Request Aborted Notification message (§3.5.9.1) | MUST | 3.5.9.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| `RFC5036-3.5.9.1-2` | The Notification MUST include a Label Request Message ID TLV that carries the message ID of the aborted Label Request message. (§3.5.9.1) | MUST | 3.5.9.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| `RFC5036-3.5.9.1-3` | An LSR receiving a Label Abort Request message MUST process it immediately, regardless of the downstream state of the LSP, responding with a Label Request Aborted Notification or ignoring it, as appropriate (§3.5.9.1) | MUST | 3.5.9.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| `RFC5036-3.5.11.1-1` | An LSR transmits a Label Release message to a peer when it no longer needs a label previously received from or requested of that peer. An LSR MUST transmit a Label Release message under any of the following conditions: 1. The LSR that sent the label mapping is no longer the next hop for the mapped FEC, and the LSR is configured for conservative operation. 2. The LSR receives a label mapping from an LSR that is not the next hop for the FEC, and the LSR is configured for conservative operation. 3. The LSR receives a Label Withdraw message. (§3.5.11.1) | MUST | 3.5.11.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| `RFC5036-3.6.1.1-1` | Implementations that support vendor-private TLVs MUST support a user-accessible configuration interface that causes the U-bit to be set on all transmitted vendor-private TLVs; this requirement MAY be satisfied by a user-accessible configuration interface that prevents transmission of all vendor-private TLVs for which the U- bit is clear. (§3.6.1.1) | MUST | 3.6.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze encodes no vendor-private TLV or message, so no U-bit configuration interface exists; plan/spec-ldp-vendor-private-tlvs.md |
| `RFC5036-3.6.1.2-1` | Implementations that support Vendor-Private messages MUST support a user-accessible configuration interface that causes the U-bit to be set on all transmitted Vendor-Private messages; this requirement MAY be satisfied by a user-accessible configuration interface that prevents transmission of all Vendor-Private messages for which the U-bit is clear. (§3.6.1.2) | MUST | 3.6.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze encodes no vendor-private TLV or message, so no U-bit configuration interface exists; plan/spec-ldp-vendor-private-tlvs.md |
| `RFC5036-A.1.2-1` | An LSR operating in Downstream Unsolicited mode MUST process any Label Request messages it receives (§A.1.2) | MUST | A.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| `RFC5036-A.1.7-1` | Regardless of the Label Request procedure in use by the LSR, it MUST send a label request if the conditions in NH.13 hold (§A.1.7) | MUST | A.1.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| `RFC5036-A.1-1` | The requirement on an LDP implementation is that its event handling must have the effect specified by the algorithms. That is, an implementation need not follow exactly the steps specified by the algorithms as long as the effect is identical. (§A.1) | MUST | A.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements the Downstream Unsolicited, liberal-retention, independent-control subset and none of the Label Request, Release or Abort algorithms; plan/spec-ldp-label-request-path.md |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5036-2.6.1.2-1`](#rfc5036-2.6.1.2-1) Whenever an LSR breaks the binding between a label and a FEC, it MUST withdraw the FEC label mapping from all LDP peers to which it has previously sent the mapping. (§A) | {gap}, no test | the encoder and sender exist (internal/plugins/ldp/session.go:326 SendLabelWithdraw, internal/plugins/ldp/wire.go:499 EncodeLabelWithdraw) but nothing invokes them -- a local binding is created once in OnStarted (internal/plugins/ldp/register.go:312) and released only by RemovePop at engine exit (internal/plugins/ldp/register.go:380), so no Label Withdraw ever reaches the wire |
| [`RFC5036-2.6.1.3-1`](#rfc5036-2.6.1.3-1) An LSR that receives a Label Withdraw message MUST respond with a Label Release message. (§3.5.10.1) | {gap}, no test | the withdraw handler drops the binding and reconciles forwarding without replying (internal/plugins/ldp/register.go:821 the onWithdraw callback, internal/plugins/ldp/fib.go:48 withdrawRemoteBinding); MsgTypeLabelRelease has no encoder and an inbound one is discarded at internal/plugins/ldp/session.go:463 |
| [`RFC5036-2.5.1-3`](#rfc5036-2.5.1-3) Next LSR1 checks whether the session parameters proposed in the message are acceptable. If they are, LSR1 replies with an Initialization message of its own to propose the parameters it wishes to use and a KeepAlive message to signal acceptance of LSR2's parameters. (§2.5.3) | {gap}, no test | ze has no passive role. startSessionForAdj (internal/plugins/ldp/register.go) dials every session and nothing listens on TCP port 646, so ze never receives an Initialization before it has sent its own and never replies with one. The active role's reply, a KeepAlive to an acceptable Initialization, is RFC5036-2.5.3-5 |
| [`RFC5036-2.5.3-7`](#rfc5036-2.5.3-7) If the parameters are not acceptable, LSR1 responds by sending a Session Rejected/Parameters Error Notification message and closing the TCP connection. (§2.5.3) | {gap}, no test | this is the passive role's step, and ze has no passive role: startSessionForAdj (internal/plugins/ldp/register.go) dials every session and nothing listens on TCP port 646, so no Initialization reaches ze on a connection the peer opened. The active role's refusal is RFC5036-2.5.3-6 |
| [`RFC5036-2.5.1-4`](#rfc5036-2.5.1-4) If LSR1 receives an Initialization message, it attempts to match the LDP Identifier carried by the message PDU with a Hello adjacency. (§2.5.3) | {gap}, no test | handleInit overwrites the expected peer LSR ID with whatever the PDU header carried (internal/plugins/ldp/session.go:508 `s.peerLSRID = peerLSRID`) instead of comparing it to the value learned from the Hello, and the Receiver LSR ID decoded from the Common Session Parameters TLV (internal/plugins/ldp/wire.go:338) is never compared to the local LSR ID |
| [`RFC5036-2.5.3-2`](#rfc5036-2.5.3-2) If an LSR encounters a condition requiring it to notify its peer with advisory or error information, it sends the peer a Notification message containing a Status TLV that encodes the information and optionally additional TLVs that provide more information about the condition. If the condition is one that is a fatal error, the Status Code carried in the Notification will indicate that. (§3.5.1.1) | {gap}, no test | partly met. An unacceptable Initialization is NAK'd by rejectInit, and processMessages sends fatal Shutdown for every unexpected complete message type in OPENSENT/OPENREC. KeepAlive expiry and decoding errors outside those state checks still return an error and close with no Notification in ReadLoop and processMessages (internal/plugins/ldp/session.go). The separate spec-ldp-keepalive-expiry-notification owns only the expiry cause |
| [`RFC5036-2.7-1`](#rfc5036-2.7-1) To retrieve the label, the LSR must be able to map the next hop address for the prefix to an LDP Identifier. Similarly, when the LSR learns a label for a prefix from an LDP peer, it must be able to determine whether that peer is currently a next hop for the prefix to determine whether it needs to start using the newly learned label when forwarding packets that match the prefix. To make that decision, the LSR must be able to map an LDP Identifier to the peer's addresses to check whether any are a next hop for the prefix. To enable LSRs to map between a peer LDP Identifier and the peer's addresses, LSRs advertise their addresses using LDP Address and Withdraw Address messages. (§2.7) | {gap}, no test | ze decodes Address messages but never sends one; plan/pre-release/spec-ldp-address-advertisement.md |
| [`RFC5036-3.5.1-1`](#rfc5036-3.5.1-1) If the condition is one that is a fatal error, the Status Code carried in the Notification will indicate that. In this case, after sending the Notification message the LSR SHOULD terminate the LDP session by closing the session TCP connection and discard all state associated with the session, including all label-FEC bindings learned via the session. (§3.5.1.1) | {gap} | partly met. An unacceptable Initialization and an unexpected complete message type in OPENSENT/OPENREC receive fatal Notifications; processMessages then returns an error, ReadLoop ends and runSession closes. KeepAlive expiry and decoding errors outside the initialization type checks still send no Notification, leaving the same remaining causes as RFC5036-2.5.3-2 |
| [`RFC5036-2.9-1`](#rfc5036-2.9-1) This section specifies a mechanism to protect against the introduction of spoofed TCP segments into LDP session connection streams. The use of this mechanism MUST be supported as a configurable option. (§2.9) | {gap}, no test | ze reads no LDP password and sets no TCP_MD5SIG; plan/pre-release/spec-ldp-md5-authentication.md |
| [`RFC5036-2.8.1-1`](#rfc5036-2.8.1-1) - The Label Request message MUST include a Hop Count TLV. (§2.8.1) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.1-2`](#rfc5036-2.8.1-2) - If R is sending the Label Request because it is a FEC ingress, it MUST include a Hop Count TLV with hop count value 1. (§2.8.1) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.1-3`](#rfc5036-2.8.1-3) - If R is sending the Label Request as a result of having received a Label Request from an upstream LSR, and if the received Label Request contains a Hop Count TLV, R MUST increment the received hop count value by 1 and MUST pass the resulting value in a Hop Count TLV to its next hop along with the Label Request message. (§2.8.1) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.1-4`](#rfc5036-2.8.1-4) - If R is sending the Label Request because it is a FEC ingress, then if R is non-merge capable, it MUST include a Path Vector TLV of length 1 containing its own LSR Id. (§2.8.1) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.1-5`](#rfc5036-2.8.1-5) R MUST add its own LSR Id to the Path Vector, and MUST pass the resulting Path Vector to its next hop along with the Label Request message. (§2.8.1) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.1-6`](#rfc5036-2.8.1-6) If the Label Request contains no Path Vector TLV, R MUST include a Path Vector TLV of length 1 containing its own LSR Id. (§2.8.1) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.1-7`](#rfc5036-2.8.1-7) When R detects a loop, it MUST send a Loop Detected Notification message to the source of the Label Request message and drop the Label Request message. (§2.8.1) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.2-1`](#rfc5036-2.8.2-1) The rules that govern the use of the Hop Count TLV in Label Mapping messages sent by an LSR R when Loop Detection is enabled are the following: - R MUST include a Hop Count TLV. (§2.8.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.2-2`](#rfc5036-2.8.2-2) - If R is the egress, the hop count value MUST be 1. (§2.8.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.2-3`](#rfc5036-2.8.2-3) If the Label Mapping message is being sent to propagate a Label Mapping message received from the next hop to an upstream peer, the hop count value MUST be determined as follows: (§2.8.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.2-4`](#rfc5036-2.8.2-4) o If R is a member of the edge set of an LSR domain whose LSRs do not perform 'TTL-decrement' (e.g., an ATM LSR domain or a Frame Relay LSR domain) and the upstream peer is within that domain, R MUST reset the hop count to 1 before propagating the message. (§2.8.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.2-5`](#rfc5036-2.8.2-5) Otherwise, R MUST increment the hop count received from the next hop before propagating the message. (§2.8.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.2-6`](#rfc5036-2.8.2-6) - If the Label Mapping message is not being sent to propagate a Label Mapping message, the hop count value MUST be the result of incrementing R's current knowledge of the hop count learned from previous Label Mapping messages. (§2.8.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.2-7`](#rfc5036-2.8.2-7) - If R is sending the Label Mapping message to propagate a Label Mapping message received from the next hop to an upstream peer, then: o If R is merge capable and if R has not previously sent a Label Mapping message to the upstream peer, then it MUST include a Path Vector TLV. (§2.8.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.2-8`](#rfc5036-2.8.2-8) o If the received message contains an unknown hop count, then R MUST include a Path Vector TLV. (§2.8.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.2-9`](#rfc5036-2.8.2-9) o If R has previously sent a Label Mapping message to the upstream peer, then it MUST include a Path Vector TLV if the received message reports an LSP hop count increase, a change in hop count from unknown to known, or a change from known to unknown. (§2.8.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.2-10`](#rfc5036-2.8.2-10) o If the received Label Mapping message included a Path Vector, the Path Vector sent upstream MUST be the result of adding R's LSR Id to the received Path Vector. (§2.8.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.2-11`](#rfc5036-2.8.2-11) o If the received message had no Path Vector, the Path Vector sent upstream MUST be a Path Vector of length 1 containing R's LSR Id. (§2.8.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.2-12`](#rfc5036-2.8.2-12) - If the Label Mapping message is not being sent to propagate a received message upstream, the Label Mapping message MUST include a Path Vector of length 1 containing R's LSR Id. (§2.8.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-2.8.2-13`](#rfc5036-2.8.2-13) When R detects a loop, it MUST stop using the label for forwarding, drop the Label Mapping message, and signal Loop Detected status to the source of the Label Mapping message. (§2.8.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-3.4.2.2-1`](#rfc5036-3.4.2.2-1) It MUST be set to zero on transmission and MUST be ignored on receipt. (§3.4.2.2) | {gap}, no test | ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| [`RFC5036-3.4.2.2-2`](#rfc5036-3.4.2.2-2) If the VCI is less than 16-bits, it SHOULD be right justified in the field and the preceding bits MUST be set to 0. (§3.4.2.2) | {gap}, no test | ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| [`RFC5036-3.4.2.2-3`](#rfc5036-3.4.2.2-3) If Virtual Path switching is indicated in the V-bits field, then this field MUST be ignored by the receiver and set to 0 by the sender. (§3.4.2.2) | {gap}, no test | ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| [`RFC5036-3.4.2.3-1`](#rfc5036-3.4.2.3-1) This field is reserved. It MUST be set to zero on transmission and MUST be ignored on receipt. (§3.4.2.3) | {gap}, no test | ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| [`RFC5036-3.4.4.1-3`](#rfc5036-3.4.4.1-3) If Loop Detection is configured, the LSR MUST follow the procedures specified in Section "Loop Detection". (§3.4.4.1) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-3.4.5.1.1-1`](#rfc5036-3.4.5.1.1-1) An LSR that receives a Path Vector in a Label Request message MUST perform the procedures described in Section "Loop Detection". (§3.4.5.1.1) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-3.4.5.1.1-2`](#rfc5036-3.4.5.1.1-2) If the LSR detects a loop, it MUST reject the Label Request message. (§3.4.5.1.1) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-3.4.5.1.1-3`](#rfc5036-3.4.5.1.1-3) If the LSR detects a loop, it MUST reject the Label Request message. The LSR MUST: 1. Transmit a Notification message to the sending LSR signaling "Loop Detected". 2. Not propagate the Label Request message further. (§3.4.5.1.1) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-3.4.5.1.2-1`](#rfc5036-3.4.5.1.2-1) An LSR that receives a Path Vector in a Label Mapping message MUST perform the procedures described in Section "Loop Detection". (§3.4.5.1.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-3.4.5.1.2-2`](#rfc5036-3.4.5.1.2-2) If the LSR detects a loop, it MUST reject the Label Mapping message in order to prevent a forwarding loop. (§3.4.5.1.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-3.4.5.1.2-3`](#rfc5036-3.4.5.1.2-3) If the LSR detects a loop, it MUST reject the Label Mapping message in order to prevent a forwarding loop. The LSR MUST: 1. Transmit a Label Release message carrying a Status TLV to the sending LSR to signal "Loop Detected". 2. Not propagate the message further. 3. Check whether the Label Mapping message is for an existing LSP. If so, the LSR must unsplice any upstream labels that are spliced to the downstream label for the FEC. (§3.4.5.1.2) | {gap}, no test | loop detection is not configurable and its Path Vector procedures are unimplemented; plan/spec-ldp-loop-detection.md |
| [`RFC5036-3.5.3-2`](#rfc5036-3.5.3-2) If the session is for a label-controlled ATM link or a label-controlled Frame Relay link, then Downstream on Demand MUST be used (§3.5.3) | {gap}, no test | ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| [`RFC5036-3.5.3-10`](#rfc5036-3.5.3-10) A receiving LSR MUST calculate the intersection between the received range and its own supported label range. (§3.5.3) | {gap}, no test | ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| [`RFC5036-3.5.3-11`](#rfc5036-3.5.3-11) LSRs MUST NOT establish a session with neighbors for which the intersection of ranges is NULL. (§3.5.3) | {gap}, no test | ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| [`RFC5036-3.5.3-12`](#rfc5036-3.5.3-12) In this case, the LSR MUST send a Session Rejected/Parameters Label Range Notification message in response to the Initialization message and not establish the session. (§3.5.3) | {gap}, no test | ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| [`RFC5036-3.5.3-13`](#rfc5036-3.5.3-13) This field is reserved. It MUST be set to zero on transmission and MUST be ignored on receipt. (§3.5.3) | {gap}, no test | ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| [`RFC5036-3.5.3-14`](#rfc5036-3.5.3-14) When peer LSRs are connected indirectly by means of an ATM VP, the sending LSR SHOULD set the Minimum and Maximum VPI fields to 0, and the receiving LSR MUST ignore the Minimum and Maximum VPI fields. (§3.5.3) | {gap}, no test | ze runs LDP over IP links only, so no ATM or Frame Relay label range or session parameter exists; plan/spec-ldp-atm-frame-relay.md |
| [`RFC5036-3.5.7-1`](#rfc5036-3.5.7-1) Label Request Message ID If this Label Mapping message is a response to a Label Request message, it MUST include the Label Request Message ID optional parameter. (§3.5.7) | {gap}, no test | ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| [`RFC5036-3.5.8.1-1`](#rfc5036-3.5.8.1-1) Unless its routing table includes an entry that exactly matches the requested Prefix, the LSR MUST respond with a No Route Notification message. (§3.5.8.1) | {gap}, no test | ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| [`RFC5036-3.5.8.1-2`](#rfc5036-3.5.8.1-2) When the receiving LSR responds with a Label Mapping message, the mapping message MUST include a Label Request/Returned Message ID TLV optional parameter that includes the message ID of the Label Request message. (§3.5.8.1) | {gap}, no test | ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| [`RFC5036-3.5.8.1-3`](#rfc5036-3.5.8.1-3) When resources become available, the LSR MUST notify the requesting LSR by sending a Notification message with the Label Resources Available Status Code. (§3.5.8.1) | {gap}, no test | ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| [`RFC5036-3.5.8.1-4`](#rfc5036-3.5.8.1-4) An LSR that receives a No Label Resources response to a Label Request message MUST NOT issue further Label Request messages until it receives a Notification message with the Label Resources Available Status Code (§3.5.8.1) | {gap}, no test | ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| [`RFC5036-3.5.9.1-1`](#rfc5036-3.5.9.1-1) When an LSR receives a Label Abort Request message, if it has not previously responded to the Label Request being aborted with a Label Mapping message or some other Notification message, it MUST acknowledge the abort by responding with a Label Request Aborted Notification message (§3.5.9.1) | {gap}, no test | ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| [`RFC5036-3.5.9.1-2`](#rfc5036-3.5.9.1-2) The Notification MUST include a Label Request Message ID TLV that carries the message ID of the aborted Label Request message. (§3.5.9.1) | {gap}, no test | ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| [`RFC5036-3.5.9.1-3`](#rfc5036-3.5.9.1-3) An LSR receiving a Label Abort Request message MUST process it immediately, regardless of the downstream state of the LSP, responding with a Label Request Aborted Notification or ignoring it, as appropriate (§3.5.9.1) | {gap}, no test | ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| [`RFC5036-3.5.11.1-1`](#rfc5036-3.5.11.1-1) An LSR transmits a Label Release message to a peer when it no longer needs a label previously received from or requested of that peer. An LSR MUST transmit a Label Release message under any of the following conditions: 1. The LSR that sent the label mapping is no longer the next hop for the mapped FEC, and the LSR is configured for conservative operation. 2. The LSR receives a label mapping from an LSR that is not the next hop for the FEC, and the LSR is configured for conservative operation. 3. The LSR receives a Label Withdraw message. (§3.5.11.1) | {gap}, no test | ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| [`RFC5036-3.6.1.1-1`](#rfc5036-3.6.1.1-1) Implementations that support vendor-private TLVs MUST support a user-accessible configuration interface that causes the U-bit to be set on all transmitted vendor-private TLVs; this requirement MAY be satisfied by a user-accessible configuration interface that prevents transmission of all vendor-private TLVs for which the U- bit is clear. (§3.6.1.1) | {gap}, no test | ze encodes no vendor-private TLV or message, so no U-bit configuration interface exists; plan/spec-ldp-vendor-private-tlvs.md |
| [`RFC5036-3.6.1.2-1`](#rfc5036-3.6.1.2-1) Implementations that support Vendor-Private messages MUST support a user-accessible configuration interface that causes the U-bit to be set on all transmitted Vendor-Private messages; this requirement MAY be satisfied by a user-accessible configuration interface that prevents transmission of all Vendor-Private messages for which the U-bit is clear. (§3.6.1.2) | {gap}, no test | ze encodes no vendor-private TLV or message, so no U-bit configuration interface exists; plan/spec-ldp-vendor-private-tlvs.md |
| [`RFC5036-A.1.2-1`](#rfc5036-a.1.2-1) An LSR operating in Downstream Unsolicited mode MUST process any Label Request messages it receives (§A.1.2) | {gap}, no test | ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| [`RFC5036-A.1.7-1`](#rfc5036-a.1.7-1) Regardless of the Label Request procedure in use by the LSR, it MUST send a label request if the conditions in NH.13 hold (§A.1.7) | {gap}, no test | ze runs Downstream Unsolicited only; the Label Request, Release and Abort path is absent; plan/spec-ldp-label-request-path.md |
| [`RFC5036-A.1-1`](#rfc5036-a.1-1) The requirement on an LDP implementation is that its event handling must have the effect specified by the algorithms. That is, an implementation need not follow exactly the steps specified by the algorithms as long as the effect is identical. (§A.1) | {gap}, no test | ze implements the Downstream Unsolicited, liberal-retention, independent-control subset and none of the Label Request, Release or Abort algorithms; plan/spec-ldp-label-request-path.md |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5036-x-1`](#rfc5036-x-1)

Two octet unsigned integer containing the version number of the protocol. This version of the specification specifies LDP protocol version 1. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC5036PDUVersionOneAccepted reads the PDU header SendInit emits and checks Version 1, and decodes a Version 1 header; TestRFC5036PDUVersionOtherRejected checks versions 0, 2 and 65535 are refused by decodePDUHeader and that processDiscoveryPacket forms no adjacency from a version 2 Hello.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036PDUVersionOtherRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L162) | unit/verify | unproven |
| positive | [`TestRFC5036PDUVersionOneAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L135) | unit/verify | unproven |

### [`RFC5036-x-2`](#rfc5036-x-2)

This field is reserved. It MUST be set to zero on transmission and ignored on receipt. (§3.5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC5036HelloReservedBitsZeroOnTransmit checks the 14 reserved bits of the Common Hello Parameters TLV are zero from EncodeHello for all four T/R combinations; TestRFC5036HelloReservedBitsIgnoredOnReceipt sets every reserved bit and checks DecodeHello reads neither T nor R nor a changed hold time.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036HelloReservedBitsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L209) | unit/verify | unproven |
| positive | [`TestRFC5036HelloReservedBitsZeroOnTransmit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L187) | unit/verify | unproven |

### [`RFC5036-x-3`](#rfc5036-x-3)

Two octet unsigned integer containing the version number of the protocol. This version of the specification specifies LDP protocol version 1. (§3.5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC5036InitProtocolVersionOne reads the Initialization SendInit emits and checks Protocol Version 1, and drives processMessages to operational on version 1; TestRFC5036InitProtocolVersionOtherRejected checks versions 0, 2 and 65535 draw the Bad Protocol Version Notification, errBadVersion and no operational state. Re-judged 2026-10-02 (c29 judge): since the RULINGS R4 fix an accepted Initialization takes OPENSENT to OPENREC and only the peer's KeepAlive makes the session operational (RFC 5036 2.5.4), so the unit now appends the peer's KeepAlive to the PDU before asserting operational; the assertion is unchanged, and a rejected or unaccepted Initialization still fails it (processMessages errors before the KeepAlive, and keepaliveReceived refuses without initAccepted).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036InitProtocolVersionOtherRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L266) | unit/verify | revert, verified |
| positive | [`TestRFC5036InitProtocolVersionOne`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L232) | unit/verify | unproven |

### [`RFC5036-2.5.1-1`](#rfc5036-2.5.1-1)

After the connection is established, if LSR1 is playing the active role, it initiates negotiation of session parameters by sending an Initialization message to LSR2. (§2.5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. The positive now drives runSession, the function startSessionForAdj runs on the connection ze dialed (ze is always the active LSR), with a silent peer: the first PDU on the wire must be ze's Initialization, carrying ze's LDP Identifier in the header and the peer's as receiver. It fails if runSession stops sending the Initialization, sends something else first, or waits for the peer (observed red with runSession broken). The negative is a consequence guard rather than a violating input: a session that never sent its own Initialization does not go operational on the peer's. The passive-role clause of the same RFC sentence is outside this row and recorded as a gap on RFC5036-2.5.1-3.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036SessionNotOperationalWithoutOwnInit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L436) | unit/verify | revert, verified |
| positive | [`TestRFC5036ActiveRoleInitiatesNegotiation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_establishment_test.go#L98) | unit/verify | revert, verified |

### [`RFC5036-2.5.3-1`](#rfc5036-2.5.3-1)

An LSR MUST arrange that its peer receive an LDP message from it at least every KeepAlive Time period. Any LDP protocol message will do but, in circumstances where no other LDP protocol messages have been sent within the period, a KeepAlive message MUST be sent. (§3.5.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both clauses asserted through runSession on an operational session. Positive: KeepAlive Time 1s on both sides, empty LIB so no other message is due, the next three PDUs must each be a KeepAlive and each gap at most 1s, so a sender slower than the KeepAlive Time or one sending nothing fails. Negative (dual-tagged with RFC5036-3.5.4.1-1): ze proposes 60s, the peer 1s, the input on which a sender pacing from its own proposal leaves the peer 20s without a message; each gap must stay within the negotiated 1s. Row-quality: the first sentence duplicates RFC5036-3.5.4.1-1 verbatim.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036KeepalivePacingFollowsNegotiatedTime`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L408) | unit/verify | revert, verified |
| positive | [`TestRFC5036KeepalivesSentPeriodically`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L525) | unit/verify | revert, verified |

### [`RFC5036-2.6.1.2-1`](#rfc5036-2.6.1.2-1)

Whenever an LSR breaks the binding between a label and a FEC, it MUST withdraw the FEC label mapping from all LDP peers to which it has previously sent the mapping. (§A)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.6.1.2-1, so no unit is bound to it.

### [`RFC5036-2.6.1.3-1`](#rfc5036-2.6.1.3-1)

An LSR that receives a Label Withdraw message MUST respond with a Label Release message. (§3.5.10.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.6.1.3-1, so no unit is bound to it.

### [`RFC5036-2.5.1-2`](#rfc5036-2.5.1-2)

The receiving LSR MUST calculate the value of the KeepAlive Timer by using the smaller of its proposed KeepAlive Time and the KeepAlive Time received in the PDU. (§3.5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC5036KeepaliveNegotiationAdoptsLower (peer 20s, ours 60s gives 20s) and TestRFC5036KeepaliveNegotiationRefusesHigher (peer 180s, ours 30s keeps 30s) drive the production handleInit and assert the exact negotiated value in both directions.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036KeepaliveNegotiationRefusesHigher`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L485) | unit/verify | unproven |
| positive | [`TestRFC5036KeepaliveNegotiationAdoptsLower`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L469) | unit/verify | unproven |

### [`RFC5036-3.5.3-1`](#rfc5036-3.5.3-1)

Two octet unsigned non zero integer that indicates the number of seconds that the sending LSR proposes for the value of the KeepAlive Time. (§3.5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC5036InitNonZeroKeepaliveTimeAccepted takes a non-zero KeepAlive Time to operational with no Notification; TestRFC5036InitZeroKeepaliveTimeRejected checks a zero KeepAlive Time draws Session Rejected/Bad KeepAlive Time referring to the Initialization, errBadKeepaliveTime, no operational state and no zero reaching the timers. Re-judged 2026-10-02 (c29 judge): since the RULINGS R4 fix an accepted Initialization takes OPENSENT to OPENREC and only the peer's KeepAlive makes the session operational (RFC 5036 2.5.4), so the unit now appends the peer's KeepAlive to the PDU before asserting operational; the assertion is unchanged, and a rejected or unaccepted Initialization still fails it (processMessages errors before the KeepAlive, and keepaliveReceived refuses without initAccepted).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036InitZeroKeepaliveTimeRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L364) | unit/verify | revert, verified |
| positive | [`TestRFC5036InitNonZeroKeepaliveTimeAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L327) | unit/verify | revert, verified |

### [`RFC5036-2.5.1-3`](#rfc5036-2.5.1-3)

Next LSR1 checks whether the session parameters proposed in the message are acceptable. If they are, LSR1 replies with an Initialization message of its own to propose the parameters it wishes to use and a KeepAlive message to signal acceptance of LSR2's parameters. (§2.5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.5.1-3, so no unit is bound to it.

### [`RFC5036-2.5.3-5`](#rfc5036-2.5.3-5)

If LSR1 receives an Initialization message, it checks whether the session parameters are acceptable. If so, it replies with a KeepAlive message. (§2.5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive: through runSession in the active role, an acceptable peer Initialization (KeepAlive Time 30) is answered by a KeepAlive as the next PDU, and the peer's own KeepAlive then makes the session operational (awaitOperational; RFC 5036 2.5.4 OPENSENT -> OPENREC -> OPERATIONAL since the c29 R4 fix). Negative: nothing follows ze's Initialization for 300ms while the peer has sent none (the pre-fix unconditional establishment KeepAlive fails here, observed red against HEAD register.go), and an unacceptable Initialization (KeepAlive Time 0) draws a Notification, never a KeepAlive, and no operational state. The recorded break for the negative is rejectInit; the silence half was confirmed red by the judge with HEAD runSession overlaid. Re-judged 2026-10-02 (c29 judge): only the positive's tail changed, from an immediate operational check to the peer KeepAlive plus the same operational assertion.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036ActiveRoleNoKeepAliveWithoutAcceptableInit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_establishment_test.go#L159) | unit/verify | revert, verified |
| positive | [`TestRFC5036ActiveRoleAcceptableInitAnsweredWithKeepAlive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_establishment_test.go#L129) | unit/verify | revert, verified |

### [`RFC5036-2.5.3-6`](#rfc5036-2.5.3-6)

If the session parameters are unacceptable, LSR1 sends a Session Rejected/Parameters Error Notification message and closes the connection. (§2.5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive: through runSession, an Initialization with KeepAlive Time 0 draws Session Rejected/Bad KeepAlive Time (0x80000018, a Session Rejected parameters code of RFC 5036 section 3.9) naming the Initialization's id and type, then end of file on the peer end, and no operational state. The second case (Protocol Version 2) draws Bad Protocol Version, which section 3.5.1.2.1 names rather than a Session Rejected code; it proves the close, not this row's status code. Negative: an acceptable Initialization draws a KeepAlive and the connection stays open for a periodic KeepAlive. Only KeepAlive Time and Protocol Version are refusable today; Max PDU Length and advertisement-mode refusals are never owed by ze's acceptance rules.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036ActiveRoleAcceptableInitKeepsConnection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_establishment_test.go#L242) | unit/verify | revert, verified |
| positive | [`TestRFC5036ActiveRoleUnacceptableInitRejectedAndClosed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_establishment_test.go#L192) | unit/verify | revert, verified |

### [`RFC5036-2.5.3-7`](#rfc5036-2.5.3-7)

If the parameters are not acceptable, LSR1 responds by sending a Session Rejected/Parameters Error Notification message and closing the TCP connection. (§2.5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.5.3-7, so no unit is bound to it.

### [`RFC5036-2.5.3-8`](#rfc5036-2.5.3-8)

If LSR1 receives a KeepAlive message, LSR2 has accepted its proposed session parameters. (§2.5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC5036 section 2.5.3 item 2.c: the live OpenReceivedKeepAliveEstablishesSession test first proves OPENREC after acceptable Initialization, then requires Operational on peer KeepAlive and timeout-only wire silence. OpenReceivedWaitsForPeerKeepAlive instead withholds that message, requires timeout (not EOF), and pins OPENREC. Reviewed processMessages, handleInit, keepaliveReceived, ReadLoop and runSession; acceptance is not inferred from TCP connect.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036OpenReceivedWaitsForPeerKeepAlive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_openrec_test.go#L39) | unit/verify | revert, verified |
| positive | [`TestRFC5036OpenReceivedKeepAliveEstablishesSession`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_openrec_test.go#L13) | unit/verify | revert, verified |

### [`RFC5036-2.5.3-9`](#rfc5036-2.5.3-9)

When LSR1 has received both an acceptable Initialization message and a KeepAlive message, the session is operational from LSR1's point of view. (§2.5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. The live positive requires acceptable Initialization followed by peer KeepAlive before Operational. Independent negative controls withhold KeepAlive or send it before any peer Initialization. The latter now requires fatal Shutdown 0x8000000a referencing KeepAlive 9, EOF and no recorded SessionUp after joining the worker; it no longer blesses premature KeepAlive silence. The former requires OPENREC and timeout-only silence. These distinguish both required peer messages.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036KeepAliveWithoutPeerInitializationNeverEstablishes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_openrec_test.go#L56) | unit/verify | revert, verified |
| negative | [`TestRFC5036OpenReceivedWaitsForPeerKeepAlive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_openrec_test.go#L41) | unit/verify | revert, verified |
| positive | [`TestRFC5036OpenReceivedKeepAliveEstablishesSession`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_openrec_test.go#L15) | unit/verify | revert, verified |

### [`RFC5036-2.5.4-1`](#rfc5036-2.5.4-1)

OPENREC Receive KeepAlive msg OPERATIONAL Receive Any other LDP msg NON EXISTENT Action: Transmit Error Notification msg (NAK) and close transport connection (§2.5.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independently rejudged after the quotation-ending prose period changed TestRFC5036OpenReceivedOtherMessageRejected. RFC 5036 Section 2.5.4 says: "Note that a Shutdown message is implemented as a Notification message with a Status TLV indicating a fatal error." Its OPENREC table requires KeepAlive -> OPERATIONAL and any other LDP message -> NON EXISTENT with Error Notification and transport close (rfc/full/rfc5036.txt:884-927). Section 3.5.1.1 says: "When an LSR receives a Shutdown message during session initialization, it SHOULD transmit a Shutdown message and then close the transport connection." Section 2.5.3 also overrides message U-bit handling before establishment (lines 847-853). Q1 yes: the two tagged carriers assert these two table branches, not merely the implementation's current error return. Q2 yes by source analysis: TestRFC5036OpenReceivedOtherMessageRejected delegates to testInitializationOtherMessageRejected, whose twelve non-KeepAlive cases demand Notification/Status TLV, exact fatal Shutdown 0x8000000a, offending message ID/type, then actual EOF before test cleanup; after the worker exits they demand no SessionUp, no Operational state and no installed Mapping or Address. Removing refusal, sending another status, silently closing, or leaving the transport open fails an assertion. TestRFC5036OpenReceivedKeepAliveEstablishesSession requires OPENREC after the live Initialization exchange, then exact Operational state after a valid peer KeepAlive and timeout-only silence, which rejects data and EOF. Q3 yes: complete version-1 PDUs reach the type guard; Mapping, Address, Address Withdraw, Initialization and Shutdown have structured bodies, while other complete message headers deliberately exercise type rejection before body decoding. The guard in internal/plugins/ldp/session.go::processMessages (lines 457-495) precedes decoding and dispatch; body errors cannot satisfy the required Shutdown/referring-message/EOF combination, and unknown U=0/U=1 cannot bypass it. Q4 yes for this whole checklist row: both KeepAlive acceptance and all non-KeepAlive type classes, including Shutdown and both unknown U bits, are exercised. These are distinct polarity assertions: repository positive labels the refusal obligation's non-KeepAlive trigger, while negative is a genuinely conforming KeepAlive control, not a second malformed input. Producer chain: processMessages calls sendNotification and returns the rejection; ReadLoop propagates it; internal/plugins/ldp/register.go::runSession defers Session.Stop, whose close resets StateNonExistent. keepaliveReceived implements the accepted transition; handleInit establishes OPENREC and initAccepted. internal/plugins/ldp/wire.go::encodeNotification encodes the asserted status and offending reference. Test sources: internal/plugins/ldp/rfc5036_openrec_test.go:13-37,89-97,108-213; helpers readNotificationStatus, expectClosed, expectSilence and runSessionForTest were read. Existing discrimination records name keepaliveReceived for the negative carrier and processMessages for the positive carrier; their whole-producer panic breaks prove sensitivity only to those breaks, not each clause independently. No new execution or fingerprint calculation was performed in this audit; parent owns the already-queued positive-carrier native renewal. No whole-RFC or adjacent gap closure is claimed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036OpenReceivedKeepAliveEstablishesSession`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_openrec_test.go#L17) | unit/verify | revert, verified |
| positive | [`TestRFC5036OpenReceivedOtherMessageRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_openrec_test.go#L89) | unit/verify | revert, verified |

### [`RFC5036-2.5.4-2`](#rfc5036-2.5.4-2)

OPENSENT Receive acceptable OPENREC Initialization msg Action: Transmit KeepAlive msg Receive Any other LDP msg NON EXISTENT Action: Transmit Error Notification msg (NAK) and close transport connection (§2.5.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. OPENSENT common guard in Session.processMessages accepts only Initialization. OpenSentOtherMessageRejected uses twelve other types including KeepAlive, valid Shutdown/Mapping/Address and both unknown U bits; requires fatal Shutdown with offending reference, EOF, no SessionUp and no applied mapping/address. The live acceptable Initialization control requires a KeepAlive response and OPENREC, then peer KeepAlive reaches Operational without Notification or close (strict timeout). This judges both complete table branches, not solely non-establishment.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036OpenReceivedKeepAliveEstablishesSession`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_openrec_test.go#L19) | unit/verify | revert, verified |
| positive | [`TestRFC5036OpenSentOtherMessageRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_openrec_test.go#L101) | unit/verify | revert, verified |

### [`RFC5036-3.5.2-1`](#rfc5036-3.5.2-1)

A value of 0 means use the default, which is 15 seconds for Link Hellos and 45 seconds for Targeted Hellos. (§3.5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC5036HelloHoldTimeZeroUsesDefault checks Hold Time 0 keeps the adjacency with 15s for a Link Hello and 45s for a Targeted Hello and not expired; TestRFC5036HelloHoldTimeNonZeroNotDefaulted checks non-zero values below and above the default are used as sent.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036HelloHoldTimeNonZeroNotDefaulted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L624) | unit/verify | unproven |
| positive | [`TestRFC5036HelloHoldTimeZeroUsesDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_test.go#L584) | unit/verify | unproven |

### [`RFC5036-2.5.1-4`](#rfc5036-2.5.1-4)

If LSR1 receives an Initialization message, it attempts to match the LDP Identifier carried by the message PDU with a Hello adjacency. (§2.5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.5.1-4, so no unit is bound to it.

### [`RFC5036-2.5.3-2`](#rfc5036-2.5.3-2)

If an LSR encounters a condition requiring it to notify its peer with advisory or error information, it sends the peer a Notification message containing a Status TLV that encodes the information and optionally additional TLVs that provide more information about the condition. If the condition is one that is a fatal error, the Status Code carried in the Notification will indicate that. (§3.5.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.5.3-2, so no unit is bound to it.

### [`RFC5036-2.7-1`](#rfc5036-2.7-1)

To retrieve the label, the LSR must be able to map the next hop address for the prefix to an LDP Identifier. Similarly, when the LSR learns a label for a prefix from an LDP peer, it must be able to determine whether that peer is currently a next hop for the prefix to determine whether it needs to start using the newly learned label when forwarding packets that match the prefix. To make that decision, the LSR must be able to map an LDP Identifier to the peer's addresses to check whether any are a next hop for the prefix. To enable LSRs to map between a peer LDP Identifier and the peer's addresses, LSRs advertise their addresses using LDP Address and Withdraw Address messages. (§2.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.7-1, so no unit is bound to it.

### [`RFC5036-2.9-1`](#rfc5036-2.9-1)

This section specifies a mechanism to protect against the introduction of spoofed TCP segments into LDP session connection streams. The use of this mechanism MUST be supported as a configurable option. (§2.9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.9-1, so no unit is bound to it.

### [`RFC5036-2.5.3-3`](#rfc5036-2.5.3-3)

An LSR MUST throttle its session setup retry attempts with an exponential backoff in situations where Initialization messages are being NAK'd (§2.5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036SetupRetryRefusedInsideDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L346) | unit/verify | revert, verified |
| positive | [`TestRFC5036SetupRetryBackoffGrows`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L322) | unit/verify | revert, verified |

### [`RFC5036-2.5.3-4`](#rfc5036-2.5.3-4)

The session establishment setup attempt following a NAK'd Initialization message MUST be delayed no less than 15 seconds, and subsequent delays MUST grow to a maximum delay of no less than 2 minutes (§2.5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036SetupRetryRefusedInsideDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L348) | unit/verify | revert, verified |
| positive | [`TestRFC5036SetupRetryBackoffGrows`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L324) | unit/verify | revert, verified |

### [`RFC5036-2.5.2-1`](#rfc5036-2.5.2-1)

An LSR MUST advertise the same transport address in all Hellos that advertise the same label space (§2.5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036HelloTransportAddressNeverFollowsTheSocket`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L509) | unit/verify | revert, verified |
| positive | [`TestRFC5036HellosCarryOneTransportAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L491) | unit/verify | revert, verified |

### [`RFC5036-2.6.1.2-2`](#rfc5036-2.6.1.2-2)

For each FEC for which the LSR is not the egress and no mapping exists, the LSR MUST wait until a label from a downstream LSR is received before mapping the FEC and passing corresponding labels to upstream LSRs. (§2.6.1.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 under ruling R40. Forbidden: mapping a non-egress FEC with no existing mapping, and passing its label upstream, before a label from the downstream LSR arrives. TestRFC5036NonEgressFECNotMappedBeforeDownstreamLabel holds the negative: a non-egress FEC with only a third-party binding is not mapped and no label is sent upstream. TestRFC5036EgressFECMappedWithoutWaiting is the positive: the wait binds only non-egress FECs, so the egress FEC is mapped at once, and an implementation that refused every mapping goes red. The sentence is a prohibition; mapping a transit FEC once the downstream label arrives is not an obligation of this sentence, and transit mapping remains a Support-remaining gap of rfc5036.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036NonEgressFECNotMappedBeforeDownstreamLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L498) | unit/verify | revert, verified |
| positive | [`TestRFC5036EgressFECMappedWithoutWaiting`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L469) | unit/verify | revert, verified |

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

The rules that govern the use of the Hop Count TLV in Label Mapping messages sent by an LSR R when Loop Detection is enabled are the following: - R MUST include a Hop Count TLV. (§2.8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.8.2-1, so no unit is bound to it.

### [`RFC5036-2.8.2-2`](#rfc5036-2.8.2-2)

- If R is the egress, the hop count value MUST be 1. (§2.8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.8.2-2, so no unit is bound to it.

### [`RFC5036-2.8.2-3`](#rfc5036-2.8.2-3)

If the Label Mapping message is being sent to propagate a Label Mapping message received from the next hop to an upstream peer, the hop count value MUST be determined as follows: (§2.8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.8.2-3, so no unit is bound to it.

### [`RFC5036-2.8.2-4`](#rfc5036-2.8.2-4)

o If R is a member of the edge set of an LSR domain whose LSRs do not perform 'TTL-decrement' (e.g., an ATM LSR domain or a Frame Relay LSR domain) and the upstream peer is within that domain, R MUST reset the hop count to 1 before propagating the message. (§2.8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-2.8.2-4, so no unit is bound to it.

### [`RFC5036-2.8.2-5`](#rfc5036-2.8.2-5)

Otherwise, R MUST increment the hop count received from the next hop before propagating the message. (§2.8.2)

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Global uniqueness is an operator property; what ze owns is that every LDP Identifier it sends carries the configured lsr-id and that it refuses its own identifier from the wire. The positive now reads the header of a Hello sendHello puts on a real UDP socket and of the Initialization runSession sends on a session built by sessionConfigForAdj, both of which must carry the engine's 192.0.2.77, and a distinct-LSR Hello must form the adjacency. The negative shows a Hello carrying ze's own LSR ID forms no adjacency.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036OwnLSRIDFormsNoAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L439) | unit/verify | revert, verified |
| positive | [`TestRFC5036DistinctLSRIDFormsAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L373) | unit/verify | revert, verified |

### [`RFC5036-3.3-1`](#rfc5036-3.3-1)

Upon receipt of an unknown TLV, if U is clear (=0), a notification MUST be returned to the message originator and the entire message MUST be ignored; if U is set (=1), the unknown TLV MUST be silently ignored and the rest of the message processed as if the unknown TLV did not exist. (§3.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC5036UnknownTLVWithUBitSetIsSkipped checks an unknown TLV with U=1 is skipped, the mapping is applied and nothing is written; TestRFC5036UnknownTLVWithUBitClearIsRefused checks U=0 draws the Unknown TLV Notification referring to the Label Mapping, the mapping is not applied and the session stays up.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036UnknownTLVWithUBitClearIsRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L128) | unit/verify | revert, verified |
| positive | [`TestRFC5036UnknownTLVWithUBitSetIsSkipped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L99) | unit/verify | revert, verified |

### [`RFC5036-3.4.2.2-1`](#rfc5036-3.4.2.2-1)

It MUST be set to zero on transmission and MUST be ignored on receipt. (§3.4.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.4.2.2-1, so no unit is bound to it.

### [`RFC5036-3.4.2.2-2`](#rfc5036-3.4.2.2-2)

If the VCI is less than 16-bits, it SHOULD be right justified in the field and the preceding bits MUST be set to 0. (§3.4.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.4.2.2-2, so no unit is bound to it.

### [`RFC5036-3.4.2.2-3`](#rfc5036-3.4.2.2-3)

If Virtual Path switching is indicated in the V-bits field, then this field MUST be ignored by the receiver and set to 0 by the sender. (§3.4.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.4.2.2-3, so no unit is bound to it.

### [`RFC5036-3.4.2.3-1`](#rfc5036-3.4.2.3-1)

This field is reserved. It MUST be set to zero on transmission and MUST be ignored on receipt. (§3.4.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.4.2.3-1, so no unit is bound to it.

### [`RFC5036-3.4.4.1-1`](#rfc5036-3.4.4.1-1)

If an LSR receives a message containing a Hop Count TLV, it MUST check the hop count value to determine whether the hop count has exceeded its configured maximum allowable value (§3.4.4.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The tagged HopCountWithinMaximumIsApplied and HopCountAboveMaximumDrawsLoopDetected correctly distinguish Label Mapping hop counts 4 and 5 at configured maximum 4, including callback data/suppression and exact advisory reply. But full RFC5036 section 3.4.4.1 covers received Label Request as well as Label Mapping. Session.processMessages groups MsgTypeLabelRequest among unhandled message types, without parsing/checking Hop Count. Both tagged tests send Mapping only, so they cannot detect this uncovered receive behavior. Renewed producer-halt records prove reachability, not whole-row enforcement.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036HopCountAboveMaximumDrawsLoopDetected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L203) | unit/verify | revert, verified |
| positive | [`TestRFC5036HopCountWithinMaximumIsApplied`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L174) | unit/verify | revert, verified |

### [`RFC5036-3.4.4.1-2`](#rfc5036-3.4.4.1-2)

If so, it MUST behave as if the containing message has traversed a loop by sending a Notification message signaling Loop Detected in reply to the sender of the message. (§3.4.4.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Prior enforcement overstated full RFC5036 section 3.4.4.1. The two tagged tests correctly prove Label Mapping count 5 above maximum 4 yields advisory Loop Detected with exact offending reference, no callback and Operational state, while count 4 is applied with timeout-only wire silence. But the RFC also covers Label Request containing Hop Count; Session.processMessages treats MsgTypeLabelRequest as unhandled and never checks its hop count or sends Loop Detected. Both tests send Mapping, leaving that receive case untested. Renewed producer-halt records do not close the semantic gap.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036HopCountWithinMaximumIsApplied`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L177) | unit/verify | revert, verified |
| positive | [`TestRFC5036HopCountAboveMaximumDrawsLoopDetected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L206) | unit/verify | revert, verified |

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

If the LSR detects a loop, it MUST reject the Label Request message. The LSR MUST: 1. Transmit a Notification message to the sending LSR signaling "Loop Detected". 2. Not propagate the Label Request message further. (§3.4.5.1.1)

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

If the LSR detects a loop, it MUST reject the Label Mapping message in order to prevent a forwarding loop. The LSR MUST: 1. Transmit a Label Release message carrying a Status TLV to the sending LSR to signal "Loop Detected". 2. Not propagate the message further. 3. Check whether the Label Mapping message is for an existing LSP. If so, the LSR must unsplice any upstream labels that are spliced to the downstream label for the FEC. (§3.4.5.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.4.5.1.2-3, so no unit is bound to it.

### [`RFC5036-3.5-1`](#rfc5036-3.5-1)

For messages that have required parameters, the required parameters MUST appear in the order specified by the individual message specifications (§3.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5036RequiredParametersInSpecifiedOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L336) | unit/verify | revert, verified |

### [`RFC5036-3.5.3-2`](#rfc5036-3.5.3-2)

If the session is for a label-controlled ATM link or a label-controlled Frame Relay link, then Downstream on Demand MUST be used (§3.5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.5.3-2, so no unit is bound to it.

### [`RFC5036-3.5.3-3`](#rfc5036-3.5.3-3)

Otherwise, Downstream Unsolicited MUST be used (§3.5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. On non-ATM/non-Frame-Relay sessions, InitProposesDownstreamUnsolicited pins A=0 in the sent Common Session Parameters. InitOnDemandProposalKeepsDownstreamUnsolicited constructs A=1, confirms decoding, processes Initialization plus peer KeepAlive, requires Operational and strict timeout-only silence, then successfully sends and reads an unsolicited Mapping. handleInit retains Downstream Unsolicited regardless of peer A, and runSession advertises local bindings without a request.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036InitOnDemandProposalKeepsDownstreamUnsolicited`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L105) | unit/verify | revert, verified |
| positive | [`TestRFC5036InitProposesDownstreamUnsolicited`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L85) | unit/verify | revert, verified |

### [`RFC5036-3.5.3-4`](#rfc5036-3.5.3-4)

If the label advertisement discipline determined in this way is unacceptable to an LSR, it MUST send a Session Rejected/Parameters Advertisement Mode Notification message in response to the Initialization message and not establish the session (§3.5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Reviewed the negative-only annotation: handleInit accepts Downstream Unsolicited for all supported IP sessions and has no unacceptable advertisement-mode policy. The A=1 control requires Operational, timeout-only silence rather than EOF, and a subsequent unsolicited Mapping. Thus the tested supported negotiation cannot spuriously reject an acceptable mode; no absent ATM/Frame Relay capability is claimed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036InitOnDemandProposalKeepsDownstreamUnsolicited`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L109) | unit/verify | revert, verified |

### [`RFC5036-3.5.3-5`](#rfc5036-3.5.3-5)

The configured maximum Path Vector length. MUST be 0 if Loop Detection is disabled (D = 0). (§3.5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC5036InitPathVectorLimitZeroWithoutLoopDetection reads the Initialization ze sends and checks D=0 with PVLim 0; TestRFC5036EncodeInitDropsPathVectorLimitWithoutLoopDetection gives EncodeInit a limit of 7 and checks 0 reaches the wire beside D=0 and 7 beside D=1.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036EncodeInitDropsPathVectorLimitWithoutLoopDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L167) | unit/verify | revert, verified |
| positive | [`TestRFC5036InitPathVectorLimitZeroWithoutLoopDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L151) | unit/verify | revert, verified |

### [`RFC5036-3.5.3-6`](#rfc5036-3.5.3-6)

This field is reserved. It MUST be set to zero on transmission and ignored on receipt. (§3.5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read both reserved-bit tests and EncodeInit/DecodeInit. Transmission requires all six reserved bits zero. Reception sets all six, requires neither A nor D set, processes Initialization plus peer KeepAlive to Operational, pins negotiated 30s and now requires a deadline timeout for silence (EOF fails). The initialization guard continues to accept the conforming message types; reserved bits do not change negotiation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036InitReservedBitsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L217) | unit/verify | revert, verified |
| positive | [`TestRFC5036InitReservedBitsZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L203) | unit/verify | revert, verified |

### [`RFC5036-3.5.3-7`](#rfc5036-3.5.3-7)

The receiving LSR MUST calculate the maximum PDU length for the session by using the smaller of its and its peer's proposals for Max PDU Length (§3.5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. First judgment. Live InitMaxPDULengthTakesTheSmallerProposal preserves proposals 1000,255,100,0 and pins negotiated values 1000,4096,4096,4096, a KeepAlive response, OPENREC before peer KeepAlive, Operational afterward and timeout-only silence. Independent 8000 proposal control pins the unchanged local maximum 4096. Reviewed handleInit normalization at <=255 and minimum selection. This proves negotiation, not a separate promise about enforcing received PDU size.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036InitMaxPDULengthNeverRaisedAboveOwnProposal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L311) | unit/verify | revert, verified |
| positive | [`TestRFC5036InitMaxPDULengthTakesTheSmallerProposal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L254) | unit/verify | revert, verified |

### [`RFC5036-3.5.3-8`](#rfc5036-3.5.3-8)

If the maximum PDU length determined this way is unacceptable to an LSR, it MUST send a Session Rejected/Parameters Max PDU Length Notification message in response to the Initialization message and not establish the session (§3.5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. First judgment; negative-only annotation is supported by handleInit: no Max PDU proposal is unacceptable to this implementation after <=255 normalization and minimum selection. Four live cases require KeepAlive rather than Notification, exact negotiated maximum, OPENREC then peer-KeepAlive Operational, and a timeout rather than EOF. No fatal rejection branch for an unsupported policy is invented.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036InitMaxPDULengthTakesTheSmallerProposal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L257) | unit/verify | revert, verified |

### [`RFC5036-3.5.3-9`](#rfc5036-3.5.3-9)

If there is no matching Hello adjacency, the LSR MUST send a Session Rejected/No Hello Notification message in response to the Initialization message and not establish the session. (§3.5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC5036InitFromHelloAdjacencyAccepted checks an Initialization from the adjacency's LDP Identifier goes operational with no write; TestRFC5036InitWithoutHelloAdjacencyRejected checks another LSR ID and another label space each draw Session Rejected/No Hello referring to the Initialization and no session. Re-judged 2026-10-02 (c29 judge): since the RULINGS R4 fix an accepted Initialization takes OPENSENT to OPENREC and only the peer's KeepAlive makes the session operational (RFC 5036 2.5.4), so the unit now appends the peer's KeepAlive to the PDU before asserting operational; the assertion is unchanged, and a rejected or unaccepted Initialization still fails it (processMessages errors before the KeepAlive, and keepaliveReceived refuses without initAccepted).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036InitWithoutHelloAdjacencyRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L270) | unit/verify | revert, verified |
| positive | [`TestRFC5036InitFromHelloAdjacencyAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_procedures_test.go#L249) | unit/verify | revert, verified |

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

This field is reserved. It MUST be set to zero on transmission and MUST be ignored on receipt. (§3.5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.5.3-13, so no unit is bound to it.

### [`RFC5036-3.5.3-14`](#rfc5036-3.5.3-14)

When peer LSRs are connected indirectly by means of an ATM VP, the sending LSR SHOULD set the Minimum and Maximum VPI fields to 0, and the receiving LSR MUST ignore the Minimum and Maximum VPI fields. (§3.5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5036-3.5.3-14, so no unit is bound to it.

### [`RFC5036-3.5.4.1-1`](#rfc5036-3.5.4.1-1)

An LSR MUST arrange that its peer receive an LDP message from it at least every KeepAlive Time period (§3.5.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5036KeepalivePacingFollowsNegotiatedTime`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L405) | unit/verify | revert, verified |
| positive | [`TestRFC5036PeerHearsFromZeWithinKeepaliveTime`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5036_sessionparams_test.go#L381) | unit/verify | revert, verified |

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

An LSR transmits a Label Release message to a peer when it no longer needs a label previously received from or requested of that peer. An LSR MUST transmit a Label Release message under any of the following conditions: 1. The LSR that sent the label mapping is no longer the next hop for the mapped FEC, and the LSR is configured for conservative operation. 2. The LSR receives a label mapping from an LSR that is not the next hop for the FEC, and the LSR is configured for conservative operation. 3. The LSR receives a Label Withdraw message. (§3.5.11.1)

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

The requirement on an LDP implementation is that its event handling must have the effect specified by the algorithms. That is, an implementation need not follow exactly the steps specified by the algorithms as long as the effect is identical. (§A.1)

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
