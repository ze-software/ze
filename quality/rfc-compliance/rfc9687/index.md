# RFC 9687 - Border Gateway Protocol 4 (BGP-4) Send Hold Timer

Supported. Every requirement this repository extracted from RFC 9687, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 13 of 13 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 13 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 13 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 13 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 27 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 13 | of 20 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 13 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 13 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 20 |
| Gated MUST-level | 13 |
| Obligations that bind Ze | 13 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 27 |
| Tagged units | 27 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc9687.md` |
| Requirement shard | `rfc/requirements/rfc9687.md` |
| RFC text | `rfc/full/rfc9687.txt` |

## Enrolment

Enrolled: BGP-4 Send Hold Timer: thirteen MUST-level requirements, every one proven in both polarities over the real session (internal/component/bgp/reactor/rfc9687_test.go, a Session driven to Established on a net.Pipe with Run executing and a FakeClock behind every timer). Ten of the thirteen are read from section 4.3, whose obligations are written in the indicative inside quoted NEW blocks that replace text in RFC 4271 section 8.2.2; RFC 4271 section 8 makes an implementation "support the described functionality and exhibit the same externally visible behavior", which is the same ground on which rfc/short/rfc4271.md gates eighteen rows of that section. RFC9687-4.3-1 is the OpenConfirm Event 26 arming, RFC9687-4.3-2 through -7 are the Event 29 action list (log, release resources, zero the ConnectRetryTimer, drop TCP, increment the ConnectRetryCounter, go to Idle), RFC9687-4.3-8 is the restart on every message sent, RFC9687-4.3-9 the stop when the negotiated HoldTime is zero, RFC9687-4.3-10 the stop on any transition out of Established. RFC9687-5-1 and -2 restate the close and the log as capitalised MUSTs and RFC9687-4.4-1 is the section 4.4 constraint on the attribute. Two of the thirteen were UNMET when the walk started and were fixed in the same change: startSendHoldTimer armed the timer whatever the negotiated HoldTime was, so a peering with a zero Hold Time (RFC 4271 section 4.2, which stops KEEPALIVEs in both directions) was torn down every eight minutes although it sent nothing by design; and parsePeerFromTree accepted send-hold-time 480 beside receive-hold-time 3600, an attribute section 4.4 forbids. Both fixes were proven red before green. The remaining seven rows are 1 RECOMMENDED, 2 SHOULD and 4 MAY. The section-by-section walk is recorded in rfc/extraction/rfc9687.json at register prose. Enrolled 2026-08-31.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered:**

Send Hold Timer, auto duration `max(8min, 2x hold-time)`, NOTIFICATION code 8 on expiry.

**What the ledger says remains:**

No tracked gap in current source anchors.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 13 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **13** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (13):** [`RFC9687-4.3-1`](#rfc9687-4.3-1), [`RFC9687-4.3-2`](#rfc9687-4.3-2), [`RFC9687-4.3-3`](#rfc9687-4.3-3), [`RFC9687-4.3-4`](#rfc9687-4.3-4), [`RFC9687-4.3-5`](#rfc9687-4.3-5), [`RFC9687-4.3-6`](#rfc9687-4.3-6), [`RFC9687-4.3-7`](#rfc9687-4.3-7), [`RFC9687-4.3-8`](#rfc9687-4.3-8), [`RFC9687-4.3-9`](#rfc9687-4.3-9), [`RFC9687-4.3-10`](#rfc9687-4.3-10), [`RFC9687-4.4-1`](#rfc9687-4.4-1), [`RFC9687-5-1`](#rfc9687-5-1), [`RFC9687-5-2`](#rfc9687-5-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9687-4.3-1` | In OpenConfirm on KeepAliveMsg (Event 26), the local system "starts the SendHoldTimer if the SendHoldTime is non-zero" (§4.3) | MUST | 4.3 - Changes to the FSM | **positive:** `unit/verify` [`TestRFC9687SendHoldTimerArmedOnEstablished`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L309). **negative:** `unit/verify` [`TestRFC9687SendHoldTimerNotArmedBeforeEstablished`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L330) |
| `RFC9687-4.3-2` | On SendHoldTimer_Expires (Event 29) the local system "logs an error message in the local system with the BGP Error Code 'Send Hold Timer Expired'" (§4.3) | MUST | 4.3 - Changes to the FSM | **positive:** `unit/verify` [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L145). **negative:** `unit/verify` [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L241) |
| `RFC9687-4.3-3` | On Event 29 the local system "releases all BGP resources" (§4.3) | MUST | 4.3 - Changes to the FSM | **positive:** `unit/verify` [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L147). **negative:** `unit/verify` [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L242) |
| `RFC9687-4.3-4` | On Event 29 the local system "sets the ConnectRetryTimer to zero" (§4.3) | MUST | 4.3 - Changes to the FSM | **positive:** `unit/verify` [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L150). **negative:** `unit/verify` [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L244) |
| `RFC9687-4.3-5` | On Event 29 the local system "drops the TCP connection" (§4.3) | MUST | 4.3 - Changes to the FSM | **positive:** `unit/verify` [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L152). **negative:** `unit/verify` [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L246) |
| `RFC9687-4.3-6` | On Event 29 the local system "increments the ConnectRetryCounter by 1" (§4.3) | MUST | 4.3 - Changes to the FSM | **positive:** `unit/verify` [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L154). **negative:** `unit/verify` [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L248) |
| `RFC9687-4.3-7` | On Event 29 the local system "changes its state to Idle" (§4.3) | MUST | 4.3 - Changes to the FSM | **positive:** `unit/verify` [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L156). **negative:** `unit/verify` [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L250) |
| `RFC9687-4.3-8` | "Each time the local system sends a BGP message, it restarts the SendHoldTimer" (§4.3) | MUST | 4.3 - Changes to the FSM | **positive:** `unit/verify` [`TestRFC9687SendRestartsTheSendHoldTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L376). **negative:** `unit/verify` [`TestRFC9687SilenceDoesNotRestartTheSendHoldTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L414) |
| `RFC9687-4.3-9` | The SendHoldTimer is stopped "unless the SendHoldTime value is zero or the negotiated HoldTime value is zero, in which case the SendHoldTimer is stopped" (§4.3) | MUST | 4.3 - Changes to the FSM | **positive:** `unit/verify` [`TestRFC9687ZeroNegotiatedHoldTimeStopsTheSendHoldTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L449). **negative:** `unit/verify` [`TestRFC9687NonZeroNegotiatedHoldTimeArmsTheSendHoldTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L487) |
| `RFC9687-4.3-10` | "The SendHoldTimer is stopped following any transition out of the Established state as part of the 'release all BGP resources' action" (§4.3) | MUST | 4.3 - Changes to the FSM | **positive:** `unit/verify` [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L158). **positive:** `unit/verify` [`TestRFC9687TeardownStopsTheSendHoldTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L518). **negative:** `unit/verify` [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L252) |
| `RFC9687-4.4-1` | "If SendHoldTime is non-zero, then it MUST be greater than the value of HoldTime" (§4.4) | MUST | 4.4 - Changes to BGP Timers | **positive:** `unit/verify` [`TestRFC9687SendHoldTimeMustExceedHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L547). **negative:** `unit/verify` [`TestRFC9687SendHoldTimeAboveHoldTimeAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L588) |
| `RFC9687-5-1` | "If the local system does not send any BGP messages within the period specified in SendHoldTime, then ... the BGP connection MUST be closed" (§5) | MUST | 5 - Send Hold Timer Expired Error Handling | **positive:** `unit/verify` [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L160). **negative:** `unit/verify` [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L254) |
| `RFC9687-5-2` | "Additionally, an error MUST be logged in the local system, indicating the 'Send Hold Timer Expired' Error Code" (§5) | MUST | 5 - Send Hold Timer Expired Error Handling | **positive:** `unit/verify` [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L161). **negative:** `unit/verify` [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L256) |
| `RFC9687-6-1` | "it is RECOMMENDED that implementations of this specification enable SendHoldTimer by default, without requiring additional configuration of the BGP-speaking device" (§6) | RECOMMENDED | 6 - Implementation Considerations | **positive:** no positive test. **negative:** no negative test |
| `RFC9687-6-2` | "The default value of SendHoldTime for a BGP connection SHOULD be the greater of: 8 minutes or 2 times the negotiated HoldTime" (§6) | SHOULD | 6 - Implementation Considerations | **positive:** no positive test. **negative:** no negative test |
| `RFC9687-7-1` | "BGP speakers SHOULD provide this reason ('Send Hold Timer Expired') as part of their operational state (for example, bgpPeerLastError in the BGP MIB [RFC4273])" (§7) | SHOULD | 7 - Operational Considerations | **positive:** no positive test. **negative:** no negative test |
| `RFC9687-4.3-11` | On Event 29 the local system "(optionally) sends a NOTIFICATION message with the BGP Error Code 'Send Hold Timer Expired' if the local system can determine that doing so will not delay the following actions in this paragraph" (§4.3) | MAY | 4.3 - Changes to the FSM | **positive:** no positive test. **negative:** no negative test |
| `RFC9687-4.3-12` | On Event 29 the local system "(optionally) performs peer oscillation damping if the DampPeerOscillations attribute is set to TRUE" (§4.3) | MAY | 4.3 - Changes to the FSM | **positive:** no positive test. **negative:** no negative test |
| `RFC9687-5-3` | "a NOTIFICATION message with the 'Send Hold Timer Expired' Error Code MAY be sent" (§5) | MAY | 5 - Send Hold Timer Expired Error Handling | **positive:** no positive test. **negative:** no negative test |
| `RFC9687-6-3` | "Implementations MAY make the value of SendHoldTime configurable, either globally or on a per-peer basis, within the constraints set out in Section 4.4" (§6) | MAY | 6 - Implementation Considerations | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 9687 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9687-4.3-1`](#rfc9687-4.3-1)

In OpenConfirm on KeepAliveMsg (Event 26), the local system "starts the SendHoldTimer if the SendHoldTime is non-zero" (§4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9687SendHoldTimerNotArmedBeforeEstablished`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L330) | unit/verify | unproven |
| positive | [`TestRFC9687SendHoldTimerArmedOnEstablished`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L309) | unit/verify | unproven |

### [`RFC9687-4.3-2`](#rfc9687-4.3-2)

On SendHoldTimer_Expires (Event 29) the local system "logs an error message in the local system with the BGP Error Code 'Send Hold Timer Expired'" (§4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L241) | unit/verify | unproven |
| positive | [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L145) | unit/verify | unproven |

### [`RFC9687-4.3-3`](#rfc9687-4.3-3)

On Event 29 the local system "releases all BGP resources" (§4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L242) | unit/verify | unproven |
| positive | [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L147) | unit/verify | unproven |

### [`RFC9687-4.3-4`](#rfc9687-4.3-4)

On Event 29 the local system "sets the ConnectRetryTimer to zero" (§4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L244) | unit/verify | unproven |
| positive | [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L150) | unit/verify | unproven |

### [`RFC9687-4.3-5`](#rfc9687-4.3-5)

On Event 29 the local system "drops the TCP connection" (§4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L246) | unit/verify | unproven |
| positive | [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L152) | unit/verify | unproven |

### [`RFC9687-4.3-6`](#rfc9687-4.3-6)

On Event 29 the local system "increments the ConnectRetryCounter by 1" (§4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L248) | unit/verify | unproven |
| positive | [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L154) | unit/verify | unproven |

### [`RFC9687-4.3-7`](#rfc9687-4.3-7)

On Event 29 the local system "changes its state to Idle" (§4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L250) | unit/verify | unproven |
| positive | [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L156) | unit/verify | unproven |

### [`RFC9687-4.3-8`](#rfc9687-4.3-8)

"Each time the local system sends a BGP message, it restarts the SendHoldTimer" (§4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9687SilenceDoesNotRestartTheSendHoldTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L414) | unit/verify | unproven |
| positive | [`TestRFC9687SendRestartsTheSendHoldTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L376) | unit/verify | unproven |

### [`RFC9687-4.3-9`](#rfc9687-4.3-9)

The SendHoldTimer is stopped "unless the SendHoldTime value is zero or the negotiated HoldTime value is zero, in which case the SendHoldTimer is stopped" (§4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9687NonZeroNegotiatedHoldTimeArmsTheSendHoldTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L487) | unit/verify | unproven |
| positive | [`TestRFC9687ZeroNegotiatedHoldTimeStopsTheSendHoldTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L449) | unit/verify | unproven |

### [`RFC9687-4.3-10`](#rfc9687-4.3-10)

"The SendHoldTimer is stopped following any transition out of the Established state as part of the 'release all BGP resources' action" (§4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L252) | unit/verify | unproven |
| positive | [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L158) | unit/verify | unproven |
| positive | [`TestRFC9687TeardownStopsTheSendHoldTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L518) | unit/verify | unproven |

### [`RFC9687-4.4-1`](#rfc9687-4.4-1)

"If SendHoldTime is non-zero, then it MUST be greater than the value of HoldTime" (§4.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9687SendHoldTimeAboveHoldTimeAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L588) | unit/verify | unproven |
| positive | [`TestRFC9687SendHoldTimeMustExceedHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L547) | unit/verify | unproven |

### [`RFC9687-5-1`](#rfc9687-5-1)

"If the local system does not send any BGP messages within the period specified in SendHoldTime, then ... the BGP connection MUST be closed" (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L254) | unit/verify | unproven |
| positive | [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L160) | unit/verify | unproven |

### [`RFC9687-5-2`](#rfc9687-5-2)

"Additionally, an error MUST be logged in the local system, indicating the 'Send Hold Timer Expired' Error Code" (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L256) | unit/verify | unproven |
| positive | [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L161) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6 phase 7 pilot, rfc9687 |
| Signed off | 2026-08-31 |
| Register | prose |
| Source | rfc/full/rfc9687.txt |
| Source fingerprint | 4d3d8fc283139135 |
| Record | rfc/extraction/rfc9687.json |
| Mapped sentences | 3 |
| Declined as scope | 1 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | walked | Title block, Status of This Memo, Copyright Notice, Abstract and Table of Contents. Walked rather than skipped because the site scan attributes one site here, the IETF Trust Legal Provisions boilerplate, excluded below. The Abstract restates section 1: the document defines the SendHoldTimer and the SendHoldTimer_Expires event for the BGP FSM and updates RFC 4271. Nothing before section 1 binds a BGP speaker. |
| `1` | Introduction | 0 | walked | Introduction. Indicative prose: what the document defines, why a blocked connection harms the inter-domain routing system, and that speakers following the specification close blocked connections locally instead of relying on the remote system. Its one near-directive, 'This specification intends to improve this situation by requiring that BGP connections be terminated', announces the obligation that sections 4.3 and 5 state; it adds none of its own. |
| `2` | Requirements Language | 0 | walked | Requirements Language. The BCP 14 key-words paragraph, which also states that the key words bind only when they appear in all capitals. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. |
| `3` | Example of a Problematic Scenario | 0 | walked | Example of a Problematic Scenario. Describes the fault this document addresses: a remote speaker advertising a TCP Receive Window of zero, BGP's lack of visibility into the socket, and the stale routing information that results. Every sentence is descriptive and none directs a speaker. |
| `4` | Changes to RFC 4271 - SendHoldTimer | 0 | walked | Changes to RFC 4271 - SendHoldTimer. One paragraph naming what the four subsections add: a BGP timer, SendHoldTimer, and updates to the BGP FSM. No directive. |
| `4.1` | Session Attributes | 0 | walked | Session Attributes. Adds SendHoldTimer and SendHoldTime as optional session attributes 14 and 15 of RFC 4271 section 8, then defines SendHoldTime: it 'determines how long a BGP speaker will stay in the Established state before the TCP connection is dropped because no BGP messages can be transmitted to its peer', and 'A BGP speaker can configure the value of the SendHoldTime for each peer independently'. Both are definitions of the attribute rather than directives; the permission to make it configurable is the section 6 MAY, RFC9687-6-3, and the constraint on its value is the section 4.4 MUST, RFC9687-4.4-1. The attribute is carried by the Timers table of rfc/short/rfc9687.md. |
| `4.2` | Timer Event: SendHoldTimer_Expires | 0 | walked | Timer Event: SendHoldTimer_Expires. Adds Event 29 to RFC 4271 section 8.1.3 with a definition ('An event generated when the SendHoldTimer expires') and a status ('Optional'). A registry entry for the FSM's event list, not a directive: what a speaker DOES on Event 29 is section 4.3. The word Optional is title case, so section 2 gives it no RFC 2119 meaning; it marks the event as one an implementation need not have, and ze has it. |
| `4.3` | Changes to the FSM | 0 | walked | Changes to the FSM. The document's main normative section, and the reason the register derives 'prose': every obligation it states sits inside a quoted NEW block that replaces text in RFC 4271 section 8.2.2, written in the indicative with no modal at all, so no scan attributes a site here. All ten gated ids are declared unsourced. RFC 4271 section 8 is what makes them MUST-level -- an implementation 'MUST support the described functionality and exhibit the same externally visible behavior' -- and rfc/short/rfc4271.md already gates eighteen rows of section 8.2.2 on that ground. RFC9687-4.3-1 is the revised OpenConfirm KeepAliveMsg (Event 26) action list, 'starts the SendHoldTimer if the SendHoldTime is non-zero', produced by handleKeepalive calling startSendHoldTimer (internal/component/bgp/reactor/session_handlers.go). RFC9687-4.3-2 through RFC9687-4.3-7 are the six non-optional items of the Event 29 action list added to the Established state, in the order the RFC lists them: log the error, release all BGP resources, set the ConnectRetryTimer to zero, drop the TCP connection, increment the ConnectRetryCounter by 1, change state to Idle. Their producer is sendHoldTimerExpired (session_write.go), which logs, sends the optional NOTIFICATION, fires the FSM event that increments the counter and moves to Idle (internal/component/bgp/fsm/fsm.go), and closes the connection, with Run's exit StopAll releasing the timers. RFC9687-4.3-8 and RFC9687-4.3-9 split the restart sentence: every message sent restarts the timer (resetSendHoldTimer, called after each successful flush), and a zero SendHoldTime or a zero negotiated HoldTime stops it instead (startSendHoldTimer reads Timers.HoldTime). RFC9687-4.3-10 is the closing sentence, the stop on any transition out of Established, produced by closeConn. Two sentences the scans cannot see are not obligations: the OLD block quotes the RFC 4271 text being replaced, and the two optional items of the action list are the MAY rows RFC9687-4.3-11 and RFC9687-4.3-12, which are not gated. |
| `4.4` | Changes to BGP Timers | 1 | walked | Changes to BGP Timers. Adds SendHoldTimer to the RFC 4271 section 10 timer summary. Its one site carries the section's only capitalised keyword and is mapped below to RFC9687-4.4-1. The surrounding sentence, 'SendHoldTime is an FSM attribute that stores the initial value for the SendHoldTimer', is a definition already carried by section 4.1 and by the Timers table of rfc/short/rfc9687.md. |
| `5` | Send Hold Timer Expired Error Handling | 2 | walked | Send Hold Timer Expired Error Handling. Two sites, both mapped below. The first sentence carries a MAY and a MUST: the MUST is RFC9687-5-1 and the MAY is the ungated RFC9687-5-3, which restates the optional NOTIFICATION of section 4.3. The second is RFC9687-5-2, the log obligation. Both restate items of the section 4.3 Event 29 action list at capitalised strength, which is why the summary declares them as their own ids rather than folding them into the 4.3 rows: they are separately quotable obligations at a level the scans can see, and both carry their own tags. |
| `6` | Implementation Considerations | 0 | walked | Implementation Considerations. Three advisory statements and one value definition, none gated. The RECOMMENDED default-on is RFC9687-6-1, the 'greater of 8 minutes or 2 times the negotiated HoldTime' default is the SHOULD of RFC9687-6-2, and the permission to make the value configurable is RFC9687-6-3. The closing paragraph fixes the NOTIFICATION subcode at 0 with no Data; it is a value assignment, carried by the Wire Formats and Constants sections of rfc/short/rfc9687.md and asserted by the Event 29 test rather than declared as a requirement of its own. |
| `7` | Operational Considerations | 0 | walked | Operational Considerations. Explains why the NOTIFICATION usually cannot be delivered and asks that the attempt still be made, which is the section 4.3 MAY. Its one capitalised keyword is the SHOULD of RFC9687-7-1, that a speaker provide the reason as part of its operational state; not gated. The modal scan attributes no site to this section. |
| `8` | Security Considerations | 0 | walked | Security Considerations. States that the specification does not change BGP's security characteristics and that terminating connections with malfunctioning peers enhances resilience. No countermeasure is directed at a speaker. |
| `9` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records the registration of value 8, 'Send Hold Timer Expired', in the BGP Error (Notification) Codes registry. Binds IANA, not a speaker. The value is carried by the Constants table of rfc/short/rfc9687.md. |
| `10` | References, the heading of 10.1 and 10.2 | 0 | skipped (references) | References, the heading of 10.1 and 10.2. |
| `10.1` | Normative References: RFC 2119, RFC 4271, RFC 8174, RFC 9293 | 0 | skipped (references) | Normative References: RFC 2119, RFC 4271, RFC 8174, RFC 9293. |
| `10.2` | not stated | 0 | skipped (references) | Informative References: the RIPE Labs BGP Zombies article and RFC 4273. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The IETF Trust Legal Provisions boilerplate of the Copyright Notice. Its 'must' is lower case, which section 2 puts outside the normative set, and it binds a person who reuses Code Components from the document, never a BGP speaker on the wire. | Code Components extracted from this document must include Revised BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Revised BSD License. |

## Superseded

No document obsoletes RFC 9687, so its obligations are stated where they were written.
