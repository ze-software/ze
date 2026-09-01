# RFC 5883 - Bidirectional Forwarding Detection (BFD) for Multihop Paths

Partial. Every requirement this repository extracted from RFC 5883, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 75.0% | 6 of 8 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 8 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 8 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 12 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 8 | of 9 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 8 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 25.0% | 2 of 8 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 8 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 9 |
| Gated MUST-level | 8 |
| Obligations that bind Ze | 8 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 2 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 12 |
| Tagged units | 12 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc5883.md` |
| Requirement shard | `rfc/requirements/rfc5883.md` |
| RFC text | `rfc/full/rfc5883.txt` |

## Enrolment

Enrolled: BFD for Multihop Paths: eight MUST-level requirements. Six are met with new positive+negative tests in internal/component/bfd asserting producer decisions: 5-1 (multihop uses UDP destination port 4784), 5-2 (single-hop 3784 and multihop 4784 use separate ports), 4.3-1 (a default session takes the Active role and arms its transmit timer), 4.3-2 (a Passive session stays silent until it receives a packet), and 3-1 / x-1 (BFD Echo is rejected on a multihop path). 7-1 and 7-2 (congestion detection and congestion-triggered transmit-rate reduction) are {gap}: ze has no BFD congestion-control code path (only slow-start and jitter). Disclosed in the docs/features/rfc-status.md RFC 5883 row.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

- Multi-hop UDP 4784 sessions, single-hop/multihop port separation (3784/4784), Active/Passive roles, echo-on-multihop rejection, and min-TTL floor
- tests bound per requirement in [`rfc/requirements/rfc5883.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc5883.md).


**What the ledger says remains:**

No BFD congestion control or congestion-triggered transmit-rate reduction (RFC 5883 / RFC 5880 Section 7); IPv6 dual-bind and wider deployment proof are tracked with BFD.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 6 | one part of the gated population |
| Annotated instead of tested | 2 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **8** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (6):** [`RFC5883-3-1`](#rfc5883-3-1), [`RFC5883-4.3-1`](#rfc5883-4.3-1), [`RFC5883-4.3-2`](#rfc5883-4.3-2), [`RFC5883-5-1`](#rfc5883-5-1), [`RFC5883-5-2`](#rfc5883-5-2), [`RFC5883-x-1`](#rfc5883-x-1)

**Annotated instead of tested (2):** [`RFC5883-7-1`](#rfc5883-7-1), [`RFC5883-7-2`](#rfc5883-7-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5883-3-1` | Echo function must not be used over multihop paths (§3) | MUST NOT | 3 | **positive:** `unit/verify` [`TestRFC5883SingleHopEchoAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5883_test.go#L114). **negative:** `unit/verify` [`TestRFC5883MultiHopEchoRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5883_test.go#L100) |
| `RFC5883-4.3-1` | Unidirectional Sender must operate in the Active role (§4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC5883DefaultSessionActiveArmsTx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5883_test.go#L39). **negative:** `unit/verify` [`TestRFC5883PassiveSessionDoesNotArmTx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5883_test.go#L53) |
| `RFC5883-4.3-2` | Unidirectional Receiver must operate in the Passive role (§4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC5883PassiveSessionSilentUntilRx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5883_test.go#L68). **negative:** `unit/verify` [`TestRFC5883PassiveSessionTransmitsAfterRx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5883_test.go#L79) |
| `RFC5883-5-1` | UDP destination port must be 4784 for multihop BFD Control packets (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC5883MultiHopControlPort`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5883_test.go#L20). **negative:** `unit/verify` [`TestRFC5883MultiHopControlPortNotSingleHop`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5883_test.go#L35) |
| `RFC5883-5-2` | Implementations must bind RX sockets to the correct port per session type (single-hop 3784 vs multihop 4784) (Pitfalls, §5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC5883SingleHopControlPort`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5883_test.go#L50). **negative:** `unit/verify` [`TestRFC5883SeparatePortsPerMode`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5883_test.go#L65) |
| `RFC5883-x-1` | Multihop implementations must reject attempts to enable Echo on a multihop session (Pitfalls) | MUST | x | **positive:** `unit/verify` [`TestRFC5883SingleHopEchoAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5883_test.go#L118). **negative:** `unit/verify` [`TestRFC5883MultiHopEchoRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5883_test.go#L104) |
| `RFC5883-7-1` | Congestion control must be implemented for multihop deployments (§7 of RFC 5880, referenced in Interop) | MUST | 7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no BFD congestion detection or congestion-triggered transmit-rate reduction (internal/component/bfd/ has only slow-start and jitter); the RFC 5883 / RFC 5880 Section 7 congestion-control obligation is unmet |
| `RFC5883-7-2` | When congestion is detected, TX rate must be reduced (§7 of RFC 5880, referenced in Interop) | MUST | 7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no BFD congestion detection or congestion-triggered transmit-rate reduction (internal/component/bfd/ has only slow-start and jitter); the RFC 5883 / RFC 5880 Section 7 congestion-control obligation is unmet |
| `RFC5883-6-1` | Cryptographic authentication should be used for all multihop sessions (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5883-7-1`](#rfc5883-7-1) Congestion control must be implemented for multihop deployments (§7 of RFC 5880, referenced in Interop) | {gap}, no test | ze implements no BFD congestion detection or congestion-triggered transmit-rate reduction (internal/component/bfd/ has only slow-start and jitter); the RFC 5883 / RFC 5880 Section 7 congestion-control obligation is unmet |
| [`RFC5883-7-2`](#rfc5883-7-2) When congestion is detected, TX rate must be reduced (§7 of RFC 5880, referenced in Interop) | {gap}, no test | ze implements no BFD congestion detection or congestion-triggered transmit-rate reduction (internal/component/bfd/ has only slow-start and jitter); the RFC 5883 / RFC 5880 Section 7 congestion-control obligation is unmet |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5883-3-1`](#rfc5883-3-1)

Echo function must not be used over multihop paths (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5883MultiHopEchoRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5883_test.go#L100) | unit/verify | unproven |
| positive | [`TestRFC5883SingleHopEchoAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5883_test.go#L114) | unit/verify | unproven |

### [`RFC5883-4.3-1`](#rfc5883-4.3-1)

Unidirectional Sender must operate in the Active role (§4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5883PassiveSessionDoesNotArmTx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5883_test.go#L53) | unit/verify | unproven |
| positive | [`TestRFC5883DefaultSessionActiveArmsTx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5883_test.go#L39) | unit/verify | unproven |

### [`RFC5883-4.3-2`](#rfc5883-4.3-2)

Unidirectional Receiver must operate in the Passive role (§4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5883PassiveSessionTransmitsAfterRx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5883_test.go#L79) | unit/verify | unproven |
| positive | [`TestRFC5883PassiveSessionSilentUntilRx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5883_test.go#L68) | unit/verify | unproven |

### [`RFC5883-5-1`](#rfc5883-5-1)

UDP destination port must be 4784 for multihop BFD Control packets (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5883MultiHopControlPortNotSingleHop`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5883_test.go#L35) | unit/verify | unproven |
| positive | [`TestRFC5883MultiHopControlPort`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5883_test.go#L20) | unit/verify | unproven |

### [`RFC5883-5-2`](#rfc5883-5-2)

Implementations must bind RX sockets to the correct port per session type (single-hop 3784 vs multihop 4784) (Pitfalls, §5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5883SeparatePortsPerMode`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5883_test.go#L65) | unit/verify | unproven |
| positive | [`TestRFC5883SingleHopControlPort`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5883_test.go#L50) | unit/verify | unproven |

### [`RFC5883-x-1`](#rfc5883-x-1)

Multihop implementations must reject attempts to enable Echo on a multihop session (Pitfalls)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5883MultiHopEchoRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5883_test.go#L104) | unit/verify | unproven |
| positive | [`TestRFC5883SingleHopEchoAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5883_test.go#L118) | unit/verify | unproven |

### [`RFC5883-7-1`](#rfc5883-7-1)

Congestion control must be implemented for multihop deployments (§7 of RFC 5880, referenced in Interop)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5883-7-1, so no unit is bound to it.

### [`RFC5883-7-2`](#rfc5883-7-2)

When congestion is detected, TX rate must be reduced (§7 of RFC 5880, referenced in Interop)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5883-7-2, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 5883, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 5883, so its obligations are stated where they were written.
