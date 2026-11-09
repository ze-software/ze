# RFC 9687 - Border Gateway Protocol 4 (BGP-4) Send Hold Timer

Send Hold Timer. Every requirement this repository extracted from RFC 9687, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 13 | of 13 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 13 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 13 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 13 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0 | of 13 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 8 | 8 of 35 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 13 | of 13 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 13 | of 20 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 13 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 13 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 13 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 13 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 13 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

The 9 counts marked as a part above are the whole of the 13 gated MUSTs: they add up to 13. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. It counts TAGGED UNITS, not obligations, so this count is not one of the parts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public row | Send Hold Timer |
| Enrolment | Enrolled |
| Requirements | 20 |
| Gated MUST-level | 13 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 35 |
| Tagged units | 35 |
| Recorded audit verdicts | 13 |
| Discrimination records | 8 |
| Summary | `rfc/short/rfc9687.md` |
| Requirement shard | `rfc/requirements/rfc9687.md` |
| RFC text | `rfc/full/rfc9687.txt` |

## Enrolment

Enrolled: BGP-4 Send Hold Timer: thirteen MUST-level requirements, every one proven in both polarities over the real session (internal/component/bgp/reactor/rfc9687_test.go, a Session driven to Established on a net.Pipe with Run executing and a FakeClock behind every timer). Ten of the thirteen are read from section 4.3, whose obligations are written in the indicative inside quoted NEW blocks that replace text in RFC 4271 section 8.2.2; RFC 4271 section 8 makes an implementation "support the described functionality and exhibit the same externally visible behavior", which is the same ground on which rfc/short/rfc4271.md gates eighteen rows of that section. RFC9687-4.3-1 is the OpenConfirm Event 26 arming, RFC9687-4.3-2 through -7 are the Event 29 action list (log, release resources, zero the ConnectRetryTimer, drop TCP, increment the ConnectRetryCounter, go to Idle), RFC9687-4.3-8 is the restart on every message sent, RFC9687-4.3-9 the stop when the negotiated HoldTime is zero, RFC9687-4.3-10 the stop on any transition out of Established. RFC9687-5-1 and -2 restate the close and the log as capitalised MUSTs and RFC9687-4.4-1 is the section 4.4 constraint on the attribute. Two of the thirteen were UNMET when the walk started and were fixed in the same change: startSendHoldTimer armed the timer whatever the negotiated HoldTime was, so a peering with a zero Hold Time (RFC 4271 section 4.2, which stops KEEPALIVEs in both directions) was torn down every eight minutes although it sent nothing by design; and parsePeerFromTree accepted send-hold-time 480 beside receive-hold-time 3600, an attribute section 4.4 forbids. Both fixes were proven red before green. The remaining seven rows are 1 RECOMMENDED, 2 SHOULD and 4 MAY. The section-by-section walk is recorded in rfc/extraction/rfc9687.json at register prose. Enrolled 2026-08-31.

## What the public ledger says

**Area:** Send Hold Timer

**Proof:** 3 of 13 MUST proven

**Evidence:** unit 3

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 13 | one part of the gated population |
| Annotated (including scoped evidence) | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **13** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (13):** [`RFC9687-4.3-1`](#rfc9687-4.3-1), [`RFC9687-4.3-2`](#rfc9687-4.3-2), [`RFC9687-4.3-3`](#rfc9687-4.3-3), [`RFC9687-4.3-4`](#rfc9687-4.3-4), [`RFC9687-4.3-5`](#rfc9687-4.3-5), [`RFC9687-4.3-6`](#rfc9687-4.3-6), [`RFC9687-4.3-7`](#rfc9687-4.3-7), [`RFC9687-4.3-8`](#rfc9687-4.3-8), [`RFC9687-4.3-9`](#rfc9687-4.3-9), [`RFC9687-4.3-10`](#rfc9687-4.3-10), [`RFC9687-4.4-1`](#rfc9687-4.4-1), [`RFC9687-5-1`](#rfc9687-5-1), [`RFC9687-5-2`](#rfc9687-5-2)

## Figures

- **MUST bar**: Ze proven 3, Ze correctness gap 10
- **Overview bar**: Ze MUST proven 3, SHOULD not decided 7, Ze correctness gap 10

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 13 |
| MUST rows kept (the MUST bar) | 13 |
| Ze proven | 3 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 10 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 7 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 7 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/rfc9687.json](../../../data/rfc/rfc9687.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9687-4.3-1` | starts the SendHoldTimer if the SendHoldTime is non-zero (§4.3) | MUST | 4.3 - Changes to the FSM | **positive:** `unit/verify` [`TestRFC9687SendHoldTimerArmedOnEstablished`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L336). **negative:** `unit/verify` [`TestRFC9687SendHoldTimerNotArmedBeforeEstablished`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L357) |
| `RFC9687-4.3-2` | logs an error message in the local system with the BGP Error \| Code "Send Hold Timer Expired" (§4.3) | MUST | 4.3 - Changes to the FSM | **positive:** `unit/verify` [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L171). **negative:** `unit/verify` [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L268) |
| `RFC9687-4.3-3` | releases all BGP resources (§4.3) | MUST | 4.3 - Changes to the FSM | **positive:** `unit/verify` [`TestRFC9687Event29ReleasesHoldTimerAndConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_reactor_b_test.go#L170). **positive:** `unit/verify` [`TestRFC9687Event29ReleasesTheLivePeersRIB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_rib_release_test.go#L47). **positive:** `unit/verify` [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L173). **negative:** `unit/verify` [`TestRFC9687Event29ReleasesHoldTimerAndConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_reactor_b_test.go#L171). **negative:** `unit/verify` [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L269) |
| `RFC9687-4.3-4` | sets the ConnectRetryTimer to zero (§4.3) | MUST | 4.3 - Changes to the FSM | **positive:** `unit/verify` [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L176). **negative:** `unit/verify` [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L271) |
| `RFC9687-4.3-5` | drops the TCP connection (§4.3) | MUST | 4.3 - Changes to the FSM | **positive:** `unit/verify` [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L178). **negative:** `unit/verify` [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L273) |
| `RFC9687-4.3-6` | increments the ConnectRetryCounter by 1 (§4.3) | MUST | 4.3 - Changes to the FSM | **positive:** `unit/verify` [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L180). **negative:** `unit/verify` [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L275) |
| `RFC9687-4.3-7` | changes its state to Idle (§4.3) | MUST | 4.3 - Changes to the FSM | **positive:** `unit/verify` [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L182). **negative:** `unit/verify` [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L277) |
| `RFC9687-4.3-8` | Each time the local system sends a BGP message, it restarts the \| SendHoldTimer (§4.3) | MUST | 4.3 - Changes to the FSM | **positive:** `unit/verify` [`TestRFC9687EmissionRestartsExactSendHoldDeadline`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_emission_test.go#L183). **positive:** `unit/verify` [`TestRFC9687EveryWriterRestartsTheSendHoldTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_reactor_b_test.go#L92). **positive:** `unit/verify` [`TestRFC9687RemainingWritersRestartSendHold`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_remaining_writers_test.go#L17). **positive:** `unit/verify` [`TestRFC9687SendRestartsTheSendHoldTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L401). **negative:** `unit/verify` [`TestRFC9687SilenceDoesNotRestartTheSendHoldTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L439). **negative:** `unit/verify` [`TestRFC9687SuppressedAttemptsDoNotRestartSendHold`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_emission_test.go#L69) |
| `RFC9687-4.3-9` | unless the SendHoldTime value is zero or the \| negotiated HoldTime value is zero, in which case the \| SendHoldTimer is stopped. (§4.3) | MUST | 4.3 - Changes to the FSM | **positive:** `unit/verify` [`TestRFC9687ZeroNegotiatedHoldTimeStopsTheSendHoldTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L475). **negative:** `unit/verify` [`TestRFC9687NonZeroNegotiatedHoldTimeArmsTheSendHoldTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L513) |
| `RFC9687-4.3-10` | The SendHoldTimer is stopped following any transition out of \| the Established state as part of the "release all BGP \| resources" action. (§4.3) | MUST | 4.3 - Changes to the FSM | **positive:** `unit/verify` [`TestRFC9687PeerDrivenTeardownStopsTheSendHoldTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_reactor_b_test.go#L121). **positive:** `unit/verify` [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L184). **positive:** `unit/verify` [`TestRFC9687TeardownStopsTheSendHoldTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L545). **negative:** `unit/verify` [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L279) |
| `RFC9687-4.4-1` | If SendHoldTime is non-zero, then it MUST be \| greater than the value of HoldTime; see Section 6 of [RFC9687] for \| suggested default values. (§4.4) | MUST | 4.4 - Changes to BGP Timers | **positive:** `unit/verify` [`TestRFC9687SendHoldTimeMustExceedHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L574). **negative:** `unit/verify` [`TestRFC9687SendHoldTimeAboveHoldTimeAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L615) |
| `RFC9687-5-1` | If the local system does not send any BGP messages within the period specified in SendHoldTime, then a NOTIFICATION message with the "Send Hold Timer Expired" Error Code MAY be sent and the BGP connection MUST be closed. (§5) | MUST | 5 - Send Hold Timer Expired Error Handling | **positive:** `unit/verify` [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L186). **negative:** `unit/verify` [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L281) |
| `RFC9687-5-2` | Additionally, an error MUST be logged in the local system, indicating the "Send Hold Timer Expired" Error Code. (§5) | MUST | 5 - Send Hold Timer Expired Error Handling | **positive:** `unit/verify` [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L187). **negative:** `unit/verify` [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L283) |
| `RFC9687-6-1` | Accordingly, it is RECOMMENDED that implementations of this specification enable SendHoldTimer by default, without requiring additional configuration of the BGP-speaking device. (§6) | RECOMMENDED | 6 - Implementation Considerations | **positive:** no positive test. **negative:** no negative test |
| `RFC9687-6-2` | The default value of SendHoldTime for a BGP connection SHOULD be the greater of: * 8 minutes or * 2 times the negotiated HoldTime (§6) | SHOULD | 6 - Implementation Considerations | **positive:** no positive test. **negative:** no negative test |
| `RFC9687-7-1` | Other mechanisms can be used as well, for example, BGP speakers SHOULD provide this reason ("Send Hold Timer Expired") as part of their operational state (for example, bgpPeerLastError in the BGP MIB [RFC4273]). (§7) | SHOULD | 7 - Operational Considerations | **positive:** no positive test. **negative:** no negative test |
| `RFC9687-4.3-11` | (optionally) sends a NOTIFICATION message with the BGP Error \| Code "Send Hold Timer Expired" if the local system can \| determine that doing so will not delay the following actions \| in this paragraph (§4.3) | MAY | 4.3 - Changes to the FSM | **positive:** no positive test. **negative:** no negative test |
| `RFC9687-4.3-12` | (optionally) performs peer oscillation damping if the \| DampPeerOscillations attribute is set to TRUE (§4.3) | MAY | 4.3 - Changes to the FSM | **positive:** no positive test. **negative:** no negative test |
| `RFC9687-5-3` | a NOTIFICATION message with the "Send Hold Timer Expired" Error Code MAY be sent (§5) | MAY | 5 - Send Hold Timer Expired Error Handling | **positive:** no positive test. **negative:** no negative test |
| `RFC9687-6-3` | Implementations MAY make the value of SendHoldTime configurable, either globally or on a per-peer basis, within the constraints set out in Section 4.4. (§6) | MAY | 6 - Implementation Considerations | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 9687 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9687-4.3-1`](#rfc9687-4.3-1)

starts the SendHoldTimer if the SendHoldTime is non-zero (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent stale-unit rejudgment of both current carriers from the native covers map. RFC 9687 Section 4.3, complete revised OpenConfirm Event 26 sentence (rfc/full/rfc9687.txt:199-207): 'If the local system receives a KEEPALIVE message (KeepAliveMsg (Event 26)), the local system: - restarts the HoldTimer, - starts the SendHoldTimer if the SendHoldTime is non-zero, and - changes its state to Established.' Section 4.1 (lines 161-163) additionally states: 'SendHoldTime determines how long a BGP speaker will stay in the Established state before the TCP connection is dropped because no BGP messages can be transmitted to its peer.' Retained positive assertion: internal/component/bgp/reactor/rfc9687_test.go::TestRFC9687SendHoldTimerArmedOnEstablished (lines 337-343) calls rfc9687Established with a peer HoldTime of 90 seconds; the fixture requires actual OPEN acceptance into OpenConfirm, then actual KEEPALIVE transition into Established, and the test asserts p.armed(), specifically sendHoldDeadline.Load()!=0. Retained negative assertion: TestRFC9687SendHoldTimerNotArmedBeforeEstablished (lines 358-388) supplies the same peer OPEN but no peer KEEPALIVE, requires OpenConfirm, and asserts the exact zero sendHoldDeadline. Thus the tests distinguish the arming event from connection/OPEN setup, not two names on one assertion. Their direct SendHoldTime=10s fixture attribute intentionally isolates this action from the other timers; it is not evidence for the separate configuration constraint in Section 4.4. The effective-zero SendHoldTime branch is not configurable in the current producer: session_write.go::sendHoldDuration (lines 221-226) returns a positive configured value or max(8 minutes,2*ReceiveHoldTime), with the minimum defined at session.go:184. Negotiated-zero HoldTime is a separate Section 4.3 exception, not an assertion newly claimed for these two carriers. Dispatch is explicit at session_read.go:424-425; session_handlers.go::handleKeepalive (lines 282-307) starts the timer in OpenConfirm before delivering EventKeepaliveMsg, and session_write.go::startSendHoldTimer (lines 245-257) stores the deadline and schedules the callback. The lifecycle edit replaces the detached Run launch with rfc9687Run (rfc9687_test.go:67-88), retaining the real Session.Run and its buffered error result while adding a cancel-and-join cleanup before earlier logger restoration and pipe/session cleanup. The negative test previously never consumed its local runResult; its removal removes no expectation. acceptWithReader still owns pipe and timer cleanup (session_test.go:32-55). Both protocol assertions occur before cleanup, so cleanup cannot make a prematurely armed OpenConfirm timer pass the zero assertion. [INFERENCE] Omitting the OpenConfirm arming action fails the positive assertion, while arming at connection/OPEN setup fails the negative assertion. No targeted semantic mutation or check was run for this rejudgment, and no green race execution is used as RFC truth. The inspected rfc/discrimination/rfc9687.json contains no record for this ID; records for other obligations do not supply one.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9687SendHoldTimerNotArmedBeforeEstablished`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L357) | unit/verify | unproven |
| positive | [`TestRFC9687SendHoldTimerArmedOnEstablished`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L336) | unit/verify | unproven |

### [`RFC9687-4.3-2`](#rfc9687-4.3-2)

logs an error message in the local system with the BGP Error | Code "Send Hold Timer Expired" (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 9687 Section 4.3 Event 29 requires a local error message indicating Send Hold Timer Expired. Re-read both tagged functions and the slog capture fixture: SendHoldExpiryRunsTheEvent29ActionList advances exactly SendHoldTime, now requires exact ErrSendHoldTimerExpired, and asserts the actual captured logger contains 'send hold timer expired'; NoSendHoldExpiryLeavesTheSessionIntact advances one nanosecond less and asserts that text absent while retaining Established state and resources. The fake-clock advance crosses no Hold/Keepalive/ConnectRetry deadline. sendHoldTimerExpired produces the named warning-level log; RFC text requires the error indication, not a specific logging API severity level or numeric rendering. The exact-sentinel addition excludes an unrelated Run failure and retains the log assertion. No native red/green was observed by this audit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L268) | unit/verify | unproven |
| positive | [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L171) | unit/verify | unproven |

### [`RFC9687-4.3-3`](#rfc9687-4.3-3)

releases all BGP resources (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 9687 Section 4.3 Event 29 'releases all BGP resources'. All four distinct tagged functions, including both polarities in Event29ReleasesHoldTimerAndConnection, were re-read. SendHoldExpiryRunsTheEvent29ActionList now pins exact ErrSendHoldTimerExpired and asserts stopped Keepalive/ConnectRetry/SendHold resources, Idle and closed transport; NoSendHoldExpiryLeavesTheSessionIntact keeps the resources, counter and connection before expiry. Event29ReleasesHoldTimerAndConnection independently asserts live HoldTimer, KeepaliveTimer and connection one nanosecond before expiry, then exact expiry sentinel and absence of all three on the expired fixture. Event29ReleasesTheLivePeersRIB drives the actual Peer, reactor dispatcher and registered RIB plugin, first requires both wire-originated routes present, then requires their peer storage and routes gone, the session released, Idle and nil connection, with the Event 29 NOTIFICATION on the wire. Run retires timers, closeConn releases transport, and the leave/down dispatch reaches RIBManager.handleStructuredState's peerRIB.Release/delete boundary. The existing producer-panic discrimination entry for the combined resource unit proves producer dependency only: its negative polarity is not an independently isolated mutant of the earlier live assertions. This note does not promote that record into such a proof. The source assertions remain discriminating for premature resource release versus missing release, and the exact-sentinel edits remove none. No native execution claimed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9687Event29ReleasesHoldTimerAndConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_reactor_b_test.go#L171) | unit/verify | mutant, verified |
| negative | [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L269) | unit/verify | unproven |
| positive | [`TestRFC9687Event29ReleasesHoldTimerAndConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_reactor_b_test.go#L170) | unit/verify | revert, verified |
| positive | [`TestRFC9687Event29ReleasesTheLivePeersRIB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_rib_release_test.go#L47) | unit/verify | revert, verified |
| positive | [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L173) | unit/verify | unproven |

### [`RFC9687-4.3-4`](#rfc9687-4.3-4)

sets the ConnectRetryTimer to zero (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 9687 Section 4.3 Event 29 sets ConnectRetryTimer to zero. Re-read both tagged action-list functions and their shared fixture: rfc9687Established explicitly arms and checks ConnectRetryTimer, so the positive cannot pass merely because no production retry timer was armed. SendHoldExpiryRunsTheEvent29ActionList advances exactly SendHoldTime, now requires exact ErrSendHoldTimerExpired and then asserts the retry timer stopped; NoSendHoldExpiryLeavesTheSessionIntact stops one nanosecond short and requires it still running with Established state and no terminal Run result. The isolated 10-second SendHold deadline precedes the other timer deadlines. Run's retireSessionTimers and fsm.Timers retirement/StopAll clear the retry timer. The exact error assertion adds cause isolation without replacing timer state proof. No native execution claimed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L271) | unit/verify | unproven |
| positive | [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L176) | unit/verify | unproven |

### [`RFC9687-4.3-5`](#rfc9687-4.3-5)

drops the TCP connection (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 9687 Section 4.3 Event 29 drops TCP. Both tagged action-list functions were re-read. SendHoldExpiryRunsTheEvent29ActionList advances the isolated SendHold deadline, now requires exact ErrSendHoldTimerExpired, and calls requireConnClosed after Run joins; that helper requires EOF or closed pipe and explicitly refuses deadline expiry. NoSendHoldExpiryLeavesTheSessionIntact advances one nanosecond less, retains Established and the armed timer, and fails if the transport drain reports a close. sendHoldTimerExpired invokes closeConn, backed by Run's common deferred close; a mere Run return is not the transport predicate. The new sentinel check strengthens rather than removes the existing close observation. No native test was run.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L273) | unit/verify | unproven |
| positive | [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L178) | unit/verify | unproven |

### [`RFC9687-4.3-6`](#rfc9687-4.3-6)

increments the ConnectRetryCounter by 1 (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 9687 Section 4.3 Event 29 increments ConnectRetryCounter by 1. Re-read the two tagged action-list functions and the shared fixture, which hands the session a real zero-valued fsm.ConnectRetryCounter rather than relying on the nil receiver. The expired case now requires exact ErrSendHoldTimerExpired and counter exactly 1; the one-nanosecond-short control requires exactly 0 and Established. Wrong increments, no increment and premature counting therefore contradict explicit assertions. sendHoldTimerExpired delivers EventHoldTimerExpires to the shared Established FSM branch, which calls crc.Increment and changes to Idle. No timer other than SendHold expires in the positive interval. The sentinel assertion loses no counter proof. Native execution was not performed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L275) | unit/verify | unproven |
| positive | [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L180) | unit/verify | unproven |

### [`RFC9687-4.3-7`](#rfc9687-4.3-7)

changes its state to Idle (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 9687 Section 4.3 Event 29 changes state to Idle. Re-read SendHoldExpiryRunsTheEvent29ActionList and NoSendHoldExpiryLeavesTheSessionIntact: the positive advances exactly SendHoldTime, now joins exact ErrSendHoldTimerExpired and asserts StateIdle; the negative stops one nanosecond short, rejects terminal Run completion and requires StateEstablished. The fixture establishes through real OPEN/KEEPALIVE processing and chooses a deadline preceding every other session timer. sendHoldTimerExpired delivers the event to fsm's Established HoldTimerExpires handler, whose exact transition is Idle. A missing, wrong or premature transition violates a distinct state assertion; Run completion alone does not stand in for that transition. No native execution claimed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L277) | unit/verify | unproven |
| positive | [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L182) | unit/verify | unproven |

### [`RFC9687-4.3-8`](#rfc9687-4.3-8)

Each time the local system sends a BGP message, it restarts the | SendHoldTimer (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Read RFC 9687 Section 4.3's whole restart sentence, including its zero-value exceptions, and every tagged function: SendRestartsTheSendHoldTimer, SilenceDoesNotRestartTheSendHoldTimer, EveryWriterRestartsTheSendHoldTimer, RemainingWritersRestartSendHold, EmissionRestartsExactSendHoldDeadline and SuppressedAttemptsDoNotRestartSendHold. Real RawMessage, generic KEEPALIVE, UPDATE/withdraw/raw-body, SendAnnounce, held writes, raw/parsed forward batches and dirty flush rails are exercised with nonzero negotiated hold; sending at 0.6 SendHoldTime retains Established and an armed timer past the old deadline. Exact-emission tests additionally compare wire frames and exact restarted deadline, including bufio direct writes, refresh/BoRR/EoRR, buffering without emission, and empty flush not reusing old emission. Policy suppression, duplicates, stale batches, empty operations and PATHS-LIMIT write no extra bytes, leave the exact deadline fixed and reach actual expiry. The silence unit now requires the exact ErrSendHoldTimerExpired sentinel, not just any completed Run. flushWrites consumes writePending and resets only after successful emission/flush; resetSendHoldTimer and sendHoldTimerCheck move/recheck the deadline. The zero negotiated HoldTime case is separately proven by the tagged -9 pair; effective zero SendHoldTime is unreachable because sendHoldDuration supplies a positive auto minimum. No proof loss from the sentinel addition. Source audit only; no native mutation result claimed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9687SuppressedAttemptsDoNotRestartSendHold`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_emission_test.go#L69) | unit/verify | revert, verified |
| negative | [`TestRFC9687SilenceDoesNotRestartTheSendHoldTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L439) | unit/verify | unproven |
| positive | [`TestRFC9687EmissionRestartsExactSendHoldDeadline`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_emission_test.go#L183) | unit/verify | revert, verified |
| positive | [`TestRFC9687EveryWriterRestartsTheSendHoldTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_reactor_b_test.go#L92) | unit/verify | revert, verified |
| positive | [`TestRFC9687RemainingWritersRestartSendHold`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_remaining_writers_test.go#L17) | unit/verify | revert, verified |
| positive | [`TestRFC9687SendRestartsTheSendHoldTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L401) | unit/verify | unproven |

### [`RFC9687-4.3-9`](#rfc9687-4.3-9)

unless the SendHoldTime value is zero or the | negotiated HoldTime value is zero, in which case the | SendHoldTimer is stopped. (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 9687 Section 4.3 stops SendHoldTimer when SendHoldTime or negotiated HoldTime is zero. Re-read both tagged functions and the actual duration/start producers. ZeroNegotiatedHoldTimeStopsTheSendHoldTimer negotiates peer HoldTime zero through a real OPEN, asserts effective HoldTime zero and SendHold disarmed, then advances ten SendHoldTime periods with no send and requires the session still Established. NonZeroNegotiatedHoldTimeArmsTheSendHoldTimer negotiates exactly 90 seconds, requires armed, advances SendHoldTime and now joins exact ErrSendHoldTimerExpired. startSendHoldTimer explicitly stops on zero effective duration or negotiated hold. The effective SendHoldTime-zero antecedent has no reachable case in the present capability: sendHoldDuration returns an explicitly positive configured duration or max(positive sendHoldTimerMin, 2*ReceiveHoldTime); a configured zero selects auto rather than representing effective zero. This is a producer-derived reachability boundary, not a claim that an unexercised zero-duration branch has mutant proof. The reachable zero-HoldTime exception and nonzero control have direct discriminating assertions. No native test was run.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9687NonZeroNegotiatedHoldTimeArmsTheSendHoldTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L513) | unit/verify | unproven |
| positive | [`TestRFC9687ZeroNegotiatedHoldTimeStopsTheSendHoldTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L475) | unit/verify | unproven |

### [`RFC9687-4.3-10`](#rfc9687-4.3-10)

The SendHoldTimer is stopped following any transition out of | the Established state as part of the "release all BGP | resources" action. (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 9687 Section 4.3 stops SendHoldTimer following any transition out of Established as part of resource release. Re-read all four tagged functions: SendHoldExpiryRunsTheEvent29ActionList now joins exact ErrSendHoldTimerExpired and requires timer disarmed after expiry; TeardownStopsTheSendHoldTimer starts armed, sends local Cease and requires disarmed; PeerDrivenTeardownStopsTheSendHoldTimer starts armed and drives received NOTIFICATION and peer TCP close, joins Run, then requires not Established and disarmed; NoSendHoldExpiryLeavesTheSessionIntact advances one nanosecond short of expiry and requires armed, Established and no close. The completion-only Run receive in the peer-driven cases is appropriate because the release predicates are state and timer, not a common error identity across two different terminal causes. Session.closeConn stops SendHoldTimer at the shared close boundary, and Run retires session timers on every exit. Thus the unenumerated hold/error exits share the judged stop boundary rather than independent untested stop implementations. No state or timer assertion was removed by the join comments or exact-sentinel edit. Source audit only; no native observation claimed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L279) | unit/verify | unproven |
| positive | [`TestRFC9687PeerDrivenTeardownStopsTheSendHoldTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_reactor_b_test.go#L121) | unit/verify | revert, verified |
| positive | [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L184) | unit/verify | unproven |
| positive | [`TestRFC9687TeardownStopsTheSendHoldTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L545) | unit/verify | unproven |

### [`RFC9687-4.4-1`](#rfc9687-4.4-1)

If SendHoldTime is non-zero, then it MUST be | greater than the value of HoldTime; see Section 6 of [RFC9687] for | suggested default values. (§4.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden behaviour: a non-zero SendHoldTime not greater than the HoldTime. TestRFC9687SendHoldTimeMustExceedHoldTime requires parsePeerFromTree to error on 480 below 3600 and on 600 equal to 600; TestRFC9687SendHoldTimeAboveHoldTimeAccepted requires 3601 over 3600, 480 over 90 and 480 over 0 to load, and the zero (automatic) value to be exempt. The negotiated HoldTime is min(local, peer), so the check against the local receive-hold-time bounds it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9687SendHoldTimeAboveHoldTimeAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L615) | unit/verify | unproven |
| positive | [`TestRFC9687SendHoldTimeMustExceedHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L574) | unit/verify | unproven |

### [`RFC9687-5-1`](#rfc9687-5-1)

If the local system does not send any BGP messages within the period specified in SendHoldTime, then a NOTIFICATION message with the "Send Hold Timer Expired" Error Code MAY be sent and the BGP connection MUST be closed. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 9687 Section 5 permits a Send Hold Timer Expired NOTIFICATION but requires the connection closed when no BGP message is sent within SendHoldTime. Both tagged functions were re-read. SendHoldExpiryRunsTheEvent29ActionList advances exactly the isolated SendHold deadline without sending, now requires exact ErrSendHoldTimerExpired and uses requireConnClosed to demand EOF/closed pipe, never a timeout; NoSendHoldExpiryLeavesTheSessionIntact advances one nanosecond short and requires no close and Established state. sendHoldTimerExpired closes through closeConn and Run has a common deferred close. The optional NOTIFICATION assertion in the positive unit is additional implementation behavior on a writable pipe, not a mandatory interpretation of MAY. Its sentinel addition removes no transport assertion. No native observation claimed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L281) | unit/verify | unproven |
| positive | [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L186) | unit/verify | unproven |

### [`RFC9687-5-2`](#rfc9687-5-2)

Additionally, an error MUST be logged in the local system, indicating the "Send Hold Timer Expired" Error Code. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 9687 Section 5 additionally requires a local error indicating Send Hold Timer Expired. Both tagged functions and the shared capture fixture were re-read: the exact-deadline positive now requires ErrSendHoldTimerExpired and asserts the real slog sink contains 'send hold timer expired'; the one-nanosecond-short control requires that indication absent, no Run completion, live resources and Established. sendHoldTimerExpired produces the indication before notification/close, and the fake-clock interval isolates this expiry from Hold/Keepalive/ConnectRetry events. RFC wording does not prescribe a logger API severity or require numeric code 8 when its exact error name is indicated. The exact-sentinel edit strengthens cause identification and drops no logging predicate. Source audit only; no native red or green claimed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L283) | unit/verify | unproven |
| positive | [`TestRFC9687SendHoldExpiryRunsTheEvent29ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9687_test.go#L187) | unit/verify | unproven |

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
