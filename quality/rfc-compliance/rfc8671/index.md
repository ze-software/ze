# RFC 8671 - Support for Adj-RIB-Out in the BGP Monitoring Protocol (BMP)

Supported within BMP sender scope. Every requirement this repository extracted from RFC 8671, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 70.0% | 7 of 10 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 20.0% | 2 of 10 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 10 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 10 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 5.6% | 1 of 18 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 10 | of 12 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 10 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 10 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 10 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 10.0% | 1 of 10 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 7 shares marked as a part above are the whole of the 10 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Supported within BMP sender scope |
| Enrolment | Enrolled |
| Requirements | 12 |
| Gated MUST-level | 10 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 18 |
| Tagged units | 18 |
| Recorded audit verdicts | 0 |
| Discrimination records | 1 |
| Summary | `rfc/short/rfc8671.md` |
| Requirement shard | `rfc/requirements/rfc8671.md` |
| RFC text | `rfc/full/rfc8671.txt` |

## Enrolment

Enrolled: Support for Adj-RIB-Out in BMP: three MUSTs over the BMP Per-Peer Header (internal/component/bgp/plugins/bmp). RFC8671-x-1 (O flag is bit 4) both polarities: PeerFlagO == 1<<4 and a flags byte with bit 4 set decodes as Adj-RIB-Out while bit 3 does not (TestRFC8671OFlagBit4). RFC8671-x-2 (when O=1 the L flag is also set) both polarities: the sent direction sets O and L (TestPeerHeaderFromEventAdjRIBOut), the received direction sets neither (TestPeerHeaderFromEventAdjRIBIn). RFC8671-x-3 (Adj-RIB-Out Peer Up carries the same OPENs as Adj-RIB-In) is {single-polarity: positive}: ze sources both OPENs from the per-peer openCache (bmp.go:757-772) independent of the O flag, proven round-tripping in TestBMPPeerUpRoundTrip. Ledger row unchanged (no gap).

## What the public ledger says

**Status:** Supported within BMP sender scope

**What the ledger says is covered:**

Adj-RIB-Out direction flag handling and sent-route monitoring.

**What the ledger says remains**

No MUST is a gap. [`RFC8671-6.2-1`](#rfc8671-6.2-1), the O flag zero on a Statistics Report, was one until the `statistics-timeout` timer was built (spec-bmp-statistics-timeout-sends-no-report, 2026-09-06): ze now emits a periodic Statistics Report per established peer, so the obligation is exercised and TestRFC8671StatisticsReportOnTheWireClearsTheOFlag reads the flag off a report that path produced. One feature is absent by decision, and it is not a conformance gap: ze exports the post-policy Adj-RIB-Out view only, and the pre-policy Adj-RIB-Out view is not built. RFC 7854 Section 5, which this document updates, leaves that choice to the implementation, so the two obligations conditional on the pre-policy view do not bind ze. The first is [`RFC8671-5.2-1`](#rfc8671-5.2-1), the L flag set to 0 to indicate pre-policy, annotated {feature-declined} on its checklist line in [`rfc/short/rfc8671.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc8671.md). The second, that a mandatory attribute unknown at the pre-policy phase completion is zero or empty, is not declared as a requirement id: it is excluded at site 5.2:1 of [`rfc/extraction/rfc8671.json`](https://github.com/ze-software/ze/blob/main/rfc/extraction/rfc8671.json), with the kind feature-out-of-scope and the same reason. A later scope decision can revisit it and build the pre-policy view.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 7 | one part of the gated population |
| Annotated instead of tested | 3 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **10** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (7):** [`RFC8671-x-1`](#rfc8671-x-1), [`RFC8671-x-2`](#rfc8671-x-2), [`RFC8671-4-1`](#rfc8671-4-1), [`RFC8671-5.1-1`](#rfc8671-5.1-1), [`RFC8671-6.1-1`](#rfc8671-6.1-1), [`RFC8671-6.3.1-1`](#rfc8671-6.3.1-1), [`RFC8671-7.2-1`](#rfc8671-7.2-1)

**Annotated instead of tested (3):** [`RFC8671-x-3`](#rfc8671-x-3), [`RFC8671-5.2-1`](#rfc8671-5.2-1), [`RFC8671-6.2-1`](#rfc8671-6.2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8671-x-1` | O flag uses bit 4 of the Per-Peer Header Flags field (Key Constraints) | MUST | x | **positive:** `unit/verify` [`TestRFC8671OFlagBit4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L219). **negative:** `unit/verify` [`TestRFC8671OFlagBit4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L222) |
| `RFC8671-x-2` | When O=1 (Adj-RIB-Out), the L flag (bit 6, post-policy) is also set (Peer Up Behavior) | MUST | x | **positive:** `unit/verify` [`TestPeerHeaderFromEventAdjRIBOut`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L57). **negative:** `unit/verify` [`TestPeerHeaderFromEventAdjRIBIn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L80) |
| `RFC8671-x-3` | Peer Up message for Adj-RIB-Out (O=1) carries the same sent and received OPEN messages as for Adj-RIB-In (Peer Up Behavior) | MUST | x | **positive:** `unit/verify` [`TestBMPPeerUpRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/msg_test.go#L103). **negative:** no negative test. **{single-polarity}:** ze builds every Peer Up from the peer's cached sent and received OPEN messages (internal/component/bgp/plugins/bmp/bmp.go:757-772, pair.sent/pair.received) regardless of the O flag, so an Adj-RIB-Out Peer Up carries the same OPENs as an Adj-RIB-In one by construction; there is no "different OPENs for Adj-RIB-Out" case to assert as a negative. The positive (a Peer Up round-trips its sent/received OPENs) is proven in TestBMPPeerUpRoundTrip |
| `RFC8671-x-4` | A single BGP session MAY produce two Peer Up messages: one for Adj-RIB-In (O=0) and one for Adj-RIB-Out (O=1) (Peer Up Behavior) | MAY | x | **positive:** no positive test. **negative:** no negative test |
| `RFC8671-x-5` | Pre-policy Adj-RIB-Out (L=0, O=1) is valid but uncommon (Key Constraints) | MAY | x | **positive:** no positive test. **negative:** no negative test |
| `RFC8671-4-1` | The Per-Peer Header Flags bits reserved for future use MUST be transmitted as 0, and their values MUST be ignored on receipt (§4) | MUST | 4 - Per-Peer Header | **positive:** `unit/verify` [`TestRFC8671ReservedPeerFlagsTransmittedAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L66). **negative:** `unit/verify` [`TestRFC8671ReservedPeerFlagsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L108) |
| `RFC8671-5.1-1` | Post-policy Adj-RIB-Out MUST convey to the BMP receiver what is actually transmitted to the peer (§5.1) | MUST | 5.1 - Post-policy | **positive:** `unit/verify` [`TestRFC8671PostPolicyConveysTransmittedBytes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L164). **negative:** `unit/verify` [`TestRFC8671AdjRIBOutConveysNoUntransmittedUpdate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L229). **negative:** `unit/verify` [`TestRFC8671PostPolicyConveysUnknownAttributeUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L194) |
| `RFC8671-5.2-1` | The L flag MUST be set to 0 to indicate pre-policy (§5.2) | MUST | 5.2 - Pre-policy | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "Similar to Adj-RIB-In policy validation, pre-policy Adj-RIB-Out can be used to validate and audit outbound policies."; the L flag rule binds a speaker that sends the pre-policy Adj-RIB-Out view, and ze declined that view. RFC 8671 states no obligation to offer it, and RFC 7854 Section 5, which this document updates, leaves the choice to the implementation: 'A BMP speaker may send pre-policy routes, post-policy routes, or both.' internal/component/bgp/plugins/bmp/bmp_events.go::peerHeaderFromEvent is the only producer that sets the O flag, and it sets PeerFlagO and PeerFlagL in one statement for a sent-direction event, so no ze message carries O=1 with L=0. The route-monitoring-policy leaf (internal/component/bgp/plugins/bmp/yang/ze-bmp-conf.yang) offers pre-policy (Adj-RIB-In), post-policy (Adj-RIB-Out) and all, with no pre-policy Adj-RIB-Out choice |
| `RFC8671-6.1-1` | The O flag MUST be set accordingly to indicate if the Route Monitoring or Route Mirroring message conveys Adj-RIB-In or Adj-RIB-Out (§6.1) | MUST | 6.1 - Route Monitoring and Route Mirroring | **positive:** `unit/verify` [`TestRFC8671OFlagSetOnAdjRIBOutMessages`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L268). **negative:** `unit/verify` [`TestRFC8671OFlagClearOnAdjRIBInMessages`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L306) |
| `RFC8671-6.2-1` | Statistics Report messages are not specific to Adj-RIB-In or Adj-RIB-Out and MUST have the O flag set to zero (§6.2) | MUST | 6.2 - Statistics Report | **positive:** `unit/verify` [`TestRFC8671StatisticsReportOnTheWireClearsTheOFlag`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/statistics_test.go#L530). **negative:** no negative test. **{single-polarity}:** the only production encoder of a Statistics Report is `senderSession.writeStatisticsReport` (internal/component/bgp/plugins/bmp/sender.go), which clears PeerFlagO on the header its caller hands in, so there is no valid ze Statistics Report carrying the O flag to reject and no negative case to construct. The positive is TestRFC8671StatisticsReportOnTheWireClearsTheOFlag (internal/component/bgp/plugins/bmp/statistics_test.go), which reports on a peer monitored for Adj-RIB-Out, reads the message off the collector socket after `sendStatisticsReports` produced it, and asserts the L flag survives so a cleared flags byte fails. It was recorded as a gap until 2026-09-06, for the reason the 2026-08-31 owner ruling gave: a green encoder test proves nothing about a requirement no production path exercises, and `statistics.go` is that path |
| `RFC8671-6.3.1-1` | When multiple Admin Labels are included, the BMP receiver MUST preserve their order (§6.3.1) | MUST | 6.3.1 - Peer Up Information | **positive:** `unit/verify` [`TestRFC8671AdminLabelOrderPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L459). **negative:** `unit/verify` [`TestRFC8671AdminLabelReversedOrderPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L476) |
| `RFC8671-7.2-1` | A change that alters the behavior of an existing BMP session MUST bounce that session with a Peer Down/Peer Up sequence; ze keeps the BMP session up and sends a Peer Down (reason 5, configuration reasons) then a Peer Up for every established peer reported on it. The bounce is owed to a CHANGE, so ze compares the parsed sender configuration against the one in force and acts only when a leaf deciding what the session carries has moved. The change arrives on the plugin config-apply callback, and the tests drive that callback rather than the function behind it (§7.2) | MUST | 7.2 - Changes to Existing BMP Session | **positive:** `unit/verify` [`TestRFC8671BehaviorChangeBouncesEachPeerAndKeepsTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L825). **negative:** `unit/verify` [`TestRFC8671RemovingEveryCollectorBouncesTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L989). **negative:** `unit/verify` [`TestRFC8671UnrelatedBGPChangeBouncesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L934) |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8671-5.2-1`](#rfc8671-5.2-1) The L flag MUST be set to 0 to indicate pre-policy (§5.2) | no test | no test carries this requirement id; annotated {feature-declined}: "Similar to Adj-RIB-In policy validation, pre-policy Adj-RIB-Out can be used to validate and audit outbound policies."; the L flag rule binds a speaker that sends the pre-policy Adj-RIB-Out view, and ze declined that view. RFC 8671 states no obligation to offer it, and RFC 7854 Section 5, which this document updates, leaves the choice to the implementation: 'A BMP speaker may send pre-policy routes, post-policy routes, or both.' internal/component/bgp/plugins/bmp/bmp_events.go::peerHeaderFromEvent is the only producer that sets the O flag, and it sets PeerFlagO and PeerFlagL in one statement for a sent-direction event, so no ze message carries O=1 with L=0. The route-monitoring-policy leaf (internal/component/bgp/plugins/bmp/yang/ze-bmp-conf.yang) offers pre-policy (Adj-RIB-In), post-policy (Adj-RIB-Out) and all, with no pre-policy Adj-RIB-Out choice |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8671-x-1`](#rfc8671-x-1)

O flag uses bit 4 of the Per-Peer Header Flags field (Key Constraints)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8671OFlagBit4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L222) | unit/verify | unproven |
| positive | [`TestRFC8671OFlagBit4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L219) | unit/verify | unproven |

### [`RFC8671-x-2`](#rfc8671-x-2)

When O=1 (Adj-RIB-Out), the L flag (bit 6, post-policy) is also set (Peer Up Behavior)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPeerHeaderFromEventAdjRIBIn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L80) | unit/verify | unproven |
| positive | [`TestPeerHeaderFromEventAdjRIBOut`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L57) | unit/verify | unproven |

### [`RFC8671-x-3`](#rfc8671-x-3)

Peer Up message for Adj-RIB-Out (O=1) carries the same sent and received OPEN messages as for Adj-RIB-In (Peer Up Behavior)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBMPPeerUpRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/msg_test.go#L103) | unit/verify | unproven |

### [`RFC8671-4-1`](#rfc8671-4-1)

The Per-Peer Header Flags bits reserved for future use MUST be transmitted as 0, and their values MUST be ignored on receipt (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8671ReservedPeerFlagsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L108) | unit/verify | unproven |
| positive | [`TestRFC8671ReservedPeerFlagsTransmittedAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L66) | unit/verify | unproven |

### [`RFC8671-5.1-1`](#rfc8671-5.1-1)

Post-policy Adj-RIB-Out MUST convey to the BMP receiver what is actually transmitted to the peer (§5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8671AdjRIBOutConveysNoUntransmittedUpdate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L229) | unit/verify | unproven |
| negative | [`TestRFC8671PostPolicyConveysUnknownAttributeUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L194) | unit/verify | unproven |
| positive | [`TestRFC8671PostPolicyConveysTransmittedBytes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L164) | unit/verify | unproven |

### [`RFC8671-5.2-1`](#rfc8671-5.2-1)

The L flag MUST be set to 0 to indicate pre-policy (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8671-5.2-1, so no unit is bound to it.

### [`RFC8671-6.1-1`](#rfc8671-6.1-1)

The O flag MUST be set accordingly to indicate if the Route Monitoring or Route Mirroring message conveys Adj-RIB-In or Adj-RIB-Out (§6.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8671OFlagClearOnAdjRIBInMessages`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L306) | unit/verify | unproven |
| positive | [`TestRFC8671OFlagSetOnAdjRIBOutMessages`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L268) | unit/verify | unproven |

### [`RFC8671-6.2-1`](#rfc8671-6.2-1)

Statistics Report messages are not specific to Adj-RIB-In or Adj-RIB-Out and MUST have the O flag set to zero (§6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8671StatisticsReportOnTheWireClearsTheOFlag`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/statistics_test.go#L530) | unit/verify | revert, verified |

### [`RFC8671-6.3.1-1`](#rfc8671-6.3.1-1)

When multiple Admin Labels are included, the BMP receiver MUST preserve their order (§6.3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8671AdminLabelReversedOrderPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L476) | unit/verify | unproven |
| positive | [`TestRFC8671AdminLabelOrderPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L459) | unit/verify | unproven |

### [`RFC8671-7.2-1`](#rfc8671-7.2-1)

A change that alters the behavior of an existing BMP session MUST bounce that session with a Peer Down/Peer Up sequence; ze keeps the BMP session up and sends a Peer Down (reason 5, configuration reasons) then a Peer Up for every established peer reported on it. The bounce is owed to a CHANGE, so ze compares the parsed sender configuration against the one in force and acts only when a leaf deciding what the session carries has moved. The change arrives on the plugin config-apply callback, and the tests drive that callback rather than the function behind it (§7.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8671RemovingEveryCollectorBouncesTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L989) | unit/verify | unproven |
| negative | [`TestRFC8671UnrelatedBGPChangeBouncesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L934) | unit/verify | unproven |
| positive | [`TestRFC8671BehaviorChangeBouncesEachPeerAndKeepsTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L825) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-fix agent, false public conformance claim on rfc8671, section 5.2 re-walked 2026-09-05 |
| Signed off | 2026-09-05 |
| Register | rfc2119 |
| Source | rfc/full/rfc8671.txt |
| Source fingerprint | aed5ce37b74c3a4d |
| Record | rfc/extraction/rfc8671.json |
| Mapped sentences | 8 |
| Declined as scope | 2 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Abstract, Status of This Memo, Copyright Notice and Table of Contents. The Abstract restates section 1 and states no obligation. |
| `1` | Introduction | 0 | walked | Introduction. Indicative prose: what BMP monitors today, why an operator cannot see what it advertises, and that this document updates the per-peer header of RFC 7854 Section 4.2 with a new flag. No sentence directs a speaker. |
| `2` | Terminology | 0 | walked | Terminology. The BCP 14 key-words paragraph, which binds the key words only when they appear in all capitals. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. |
| `3` | Definitions | 1 | walked | Definitions. Three terms: Adj-RIB-Out quoted from RFC 4271, pre-policy Adj-RIB-Out and post-policy Adj-RIB-Out. The one capitalised keyword sits in the post-policy definition and repeats section 5.1 word for word, so it is excluded below as a duplicate. |
| `4` | Per-Peer Header | 1 | walked | Per-Peer Header. Adds the O flag to the RFC 7854 Section 4.2 flags field and redefines Peer Address, Peer AS, Peer BGP ID and Timestamp for O=1. The bit position and the meaning of the flag are stated indicatively ('The O flag indicates Adj-RIB-In if set to 0 and Adj-RIB-Out if set to 1') and in the bit diagram, so section 2 gives them no RFC 2119 level and the site scan cannot see them; RFC8671-x-1 is read from there and is listed as unsourced. The one site is the reserved-bits sentence, mapped below. The four redefined fields are value definitions carried by the O Flag table of rfc/short/rfc8671.md, not directives. |
| `5` | Adj-RIB-Out | 0 | walked | Adj-RIB-Out. A heading with no body text: 5.1 and 5.2 carry the whole section. |
| `5.1` | Post-policy | 2 | walked | Post-policy. States the primary use case, then two capitalised MUSTs, both mapped below. The rest is indicative: which attributes are set at transmission time, and what post-policy reflects. |
| `5.2` | Pre-policy | 2 | walked | Pre-policy. Two capitalised MUSTs. Site 5.2:2 is mapped to RFC8671-5.2-1 and site 5.2:1 is excluded below. Each is conditional on the pre-policy Adj-RIB-Out view, an OPTIONAL feature ze declined, and the RFC8671-5.2-1 line of rfc/short/rfc8671.md carries the {feature-declined} annotation that records the decision. Until 2026-09-05 both sites read excluded as binding a role ze does not implement, which the presumed-wrong ruling on binds-another-role bans for a role ze fills: ze IS the BMP sender. The remaining prose is indicative: what the candidate route set holds for each peering session type, and that a null or loopback next hop is common before transmission. RFC8671-x-5 records that pre-policy Adj-RIB-Out is permitted and uncommon; that is read from this section's indicative prose, so it is listed as unsourced. |
| `6` | BMP Messages | 0 | walked | BMP Messages. Two sentences. The first says some messages carrying a per-peer header are not applicable to the Adj-RIB-In or Adj-RIB-Out distinction, and names Peer Up and Peer Down. The second, 'Unless otherwise defined, the O flag should be set to 0 in the per-peer header in BMP messages', uses a lowercase 'should', so under section 2 it carries no RFC 2119 level and the site scan does not see it. ze meets it anyway: peerHeaderFromEvent sets the O flag only for a sent-direction event, so a Peer Up, a Peer Down and a Loc-RIB message all carry O=0. |
| `6.1` | Route Monitoring and Route Mirroring | 1 | walked | Route Monitoring and Route Mirroring. One sentence, one capitalised MUST, mapped below. |
| `6.2` | Statistics Report | 1 | walked | Statistics Report. One capitalised MUST, excluded below, then the four new Stat Types 14 to 17 as value definitions. Its second sentence, 'The O flag SHOULD be ignored by the BMP receiver', is a SHOULD and never gates; ze's receiver meets it, because decodeStatisticsReport reads the per-peer header and processStatisticsReport stores the entries without consulting the O flag. |
| `6.3` | Peer Up and Down Notifications | 0 | walked | Peer Up and Down Notifications. No capitalised MUST-level keyword. Two indicative sentences and one SHOULD: the peering state a Peer Up or Peer Down conveys 'is independent of whether or not route monitoring or route mirroring messages will be sent for Adj-RIB-In, Adj-RIB-Out, or both', and a receiver SHOULD ignore the O flag on these two messages. RFC8671-x-3, which records that an Adj-RIB-Out Peer Up carries the same sent and received OPEN messages as an Adj-RIB-In one, is read from that independence sentence, so it is listed as unsourced. RFC8671-x-4 is NOT listed: it records that one BGP session MAY produce two Peer Up messages, and no sentence of RFC 8671 says that. It is a MAY, so it gates nothing and the reverse arithmetic asks no home for it. |
| `6.3.1` | Peer Up Information | 1 | walked | Peer Up Information. Defines Peer Up Information TLV type 4, Admin Label: a free-form UTF-8 string, administratively assigned, with no terminator required. One capitalised MUST on the order of multiple labels, mapped below. The type number is a value definition and the last sentence, 'The Admin Label is optional', removes any obligation to send one. |
| `7` | Other Considerations | 0 | walked | Other Considerations. A heading with no body text. |
| `7.1` | Peer and Update Groups | 0 | walked | Peer and Update Groups. Indicative prose on why a BMP sender cannot publish a simple peer group name, and why the Admin Label of section 6.3.1 answers the same need. Its last sentence puts label configuration and assignment outside the document. No directive. |
| `7.2` | Changes to Existing BMP Session | 1 | walked | Changes to Existing BMP Session. One sentence, one capitalised MUST, mapped below to RFC8671-7.2-1. |
| `8` | Security Considerations | 0 | walked | Security Considerations. Imports Section 11 of RFC 7854, states that an implementation SHOULD require sessions with authorized and trusted monitoring devices, and states that this document adds no further consideration. The SHOULD is advisory and never gates. |
| `9` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Names the registry the three subsections write to. |
| `9.1` | Addition to the BMP Peer Flags registry: flag 3, the O flag | 0 | skipped (iana) | Addition to the BMP Peer Flags registry: flag 3, the O flag. Binds IANA, not a speaker. |
| `9.2` | not stated | 0 | skipped (iana) | Additions to the BMP Statistics Types registry: types 14 to 17. Binds IANA, not a speaker. |
| `9.3` | not stated | 0 | skipped (iana) | Addition to the BMP Initiation Message TLVs registry: type 4, Admin Label. Binds IANA, not a speaker. |
| `10` | Normative References: RFC 2119, RFC 4271, RFC 7854, RFC 8174 | 0 | skipped (references) | Normative References: RFC 2119, RFC 4271, RFC 7854, RFC 8174. The section also absorbs the Acknowledgements, Contributors and Authors' Addresses blocks, none of which states an obligation. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Definitions entry for post-policy Adj-RIB-Out closes with the same sentence section 5.1 states: 'This MUST convey to the BMP receiver what is actually transmitted to the peer.' Site 5.1:1 maps that obligation to RFC8671-5.1-1. One obligation, written twice. | This MUST convey to the BMP receiver what is actually transmitted to the peer. |
| `5.2:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | The obligation is CONDITIONAL on the pre-policy Adj-RIB-Out view, an OPTIONAL feature ze decided not to offer, so it is excluded rather than declared. The sentence binds a speaker only at 'the pre-policy phase completion', which ze never reaches. RFC 8671 states no obligation to offer the pre-policy view, and RFC 7854 Section 5, which this document updates, leaves the choice to the implementation: 'A BMP speaker may send pre-policy routes, post-policy routes, or both.' peerHeaderFromEvent (internal/component/bgp/plugins/bmp/bmp_events.go) is the only producer that sets the O flag, and it sets PeerFlagO and PeerFlagL in one statement for a sent-direction event, so O=1 with L=0 is unreachable; the route-monitoring-policy leaf (yang/ze-bmp-conf.yang) offers pre-policy (Adj-RIB-In), post-policy (Adj-RIB-Out) and all, with no pre-policy Adj-RIB-Out choice. ze holds no route at a pre-policy phase for which it would have to zero or empty a mandatory attribute. This is a SCOPE DECISION and not outstanding work. The absent feature is disclosed on the summary's Support status and Support remaining rows, which name this site, and never as a conformance gap. It is NOT declared as a requirement id: rfc8671 derives 10 capitalised keyword sites and its summary already declares 10 gated requirements, two of them read from indicative prose and recorded in unsourced-ids, so an eleventh declaration would push DeriveRegister (internal/le/rfc/inventory.go) off the rfc2119 grade this source supports. | All mandatory attributes, such as next hop, MUST be either zero or have an empty length if they are unknown at the pre-policy phase completion. |

## Superseded

No document obsoletes RFC 8671, so its obligations are stated where they were written.
