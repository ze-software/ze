# RFC 8671 - Support for Adj-RIB-Out in the BGP Monitoring Protocol (BMP)

BMP Adj-RIB-Out. Every requirement listed for RFC 8671, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 9 | of 11 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 2 | of 11 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 11 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 11 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 11 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 22 | 22 of 24 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 11 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 11 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 11 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 11 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 11 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 11 | of 12 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 11 mandatory requirements: they add up to 11. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | BMP Adj-RIB-Out |
| Automatic checks | Included |
| Requirements | 12 |
| Mandatory requirements (MUST level) | 11 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 24 |
| Tests linked to requirements | 24 |
| Recorded audit verdicts | 9 |
| Fault test records | 22 |
| Summary | `rfc/short/rfc8671.md` |
| Requirement table | `rfc/requirements/rfc8671.md` |
| RFC text | `rfc/full/rfc8671.txt` |

## Automatic checks

Included in automatic checks: Support for Adj-RIB-Out in BMP: three MUSTs over the BMP Per-Peer Header (internal/component/bgp/plugins/bmp). RFC8671-x-1 (O flag is bit 4) both polarities: PeerFlagO == 1<<4 and a flags byte with bit 4 set decodes as Adj-RIB-Out while bit 3 does not (TestRFC8671OFlagBit4). RFC8671-x-2 (when O=1 the L flag is also set) both polarities: the sent direction sets O and L (TestPeerHeaderFromEventAdjRIBOut), the received direction sets neither (TestPeerHeaderFromEventAdjRIBIn). RFC8671-x-3 (Adj-RIB-Out Peer Up carries the same OPENs as Adj-RIB-In) is {single-polarity: positive}: ze sources both OPENs from the per-peer openCache (cacheOpenPDU, recordPeerUp in bmp_events.go) independent of the O flag, proven round-tripping in TestBMPPeerUpRoundTrip. Ledger row unchanged (no gap).

## Published support status

**Area:** BMP Adj-RIB-Out

**Test verification:** 11 of 11 MUST proven

**Evidence:** unit 11

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 9 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 2 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **11** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (9):** [`RFC8671-x-1`](#rfc8671-x-1), [`RFC8671-x-2`](#rfc8671-x-2), [`RFC8671-4-1`](#rfc8671-4-1), [`RFC8671-5.1-1`](#rfc8671-5.1-1), [`RFC8671-5.2-1`](#rfc8671-5.2-1), [`RFC8671-5.2-2`](#rfc8671-5.2-2), [`RFC8671-6.1-1`](#rfc8671-6.1-1), [`RFC8671-6.3.1-1`](#rfc8671-6.3.1-1), [`RFC8671-7.2-1`](#rfc8671-7.2-1)

**Annotated (including scoped evidence) (2):** [`RFC8671-x-3`](#rfc8671-x-3), [`RFC8671-6.2-1`](#rfc8671-6.2-1)

## Figures

- **MUST bar**: Ze verified 11
- **Overview bar**: Ze MUST verified 11, Not assessed 1

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 11 |
| MUST rows kept (the MUST bar) | 11 |
| Ze verified | 11 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 1 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 1 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc8671.json](../../../data/rfc/rfc8671.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8671-x-1` | The per-peer header has the same structure and flags as defined in Section 4.2 of [RFC7854] with the addition of the O flag as shown here: 0 1 2 3 4 5 6 7 +-+-+-+-+-+-+-+-+ \|V\|L\|A\|O\| Resv \| +-+-+-+-+-+-+-+-+ *  The O flag indicates Adj-RIB-In if set to 0 and Adj-RIB-Out if set to 1. (§4) | MUST | 4 - Per-Peer Header | **positive:** `unit/verify` [`TestRFC8671OFlagBit4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L219). **negative:** `unit/verify` [`TestRFC8671OFlagBit4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L222) |
| `RFC8671-x-2` | The L flag MUST be set to 1 to indicate post-policy. (§5.1) | MUST | 5.1 - Post-policy | **positive:** `unit/verify` [`TestPeerHeaderFromEventAdjRIBOut`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L58). **negative:** `unit/verify` [`TestPeerHeaderFromEventAdjRIBIn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L81) |
| `RFC8671-x-3` | Peer Up and Down Notifications convey BGP peering session state to BMP receivers.  The state is independent of whether or not route monitoring or route mirroring messages will be sent for Adj-RIB-In, Adj-RIB-Out, or both. (§6.3) | MUST | 6.3 - Peer Up and Down Notifications | **positive:** `unit/verify` [`TestBMPPeerUpRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_msg_test.go#L104). **positive:** `unit/verify` [`TestRFC8671PeerDownDoesNotDependOnTheMonitoredRIB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_verdict_test.go#L359). **positive:** `unit/verify` [`TestRFC8671PeerUpDoesNotDependOnTheMonitoredRIB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_verdict_test.go#L414). **negative:** no negative test. **{single-polarity}:** ze builds every Peer Up from the peer's cached sent and received OPEN messages (cacheOpenPDU and recordPeerUp in internal/component/bgp/plugins/bmp/bmp_events.go, pair.sent/pair.received) regardless of the O flag, so an Adj-RIB-Out Peer Up carries the same OPENs as an Adj-RIB-In one by construction; there is no "different OPENs for Adj-RIB-Out" case to assert as a negative. The positive (a Peer Up round-trips its sent/received OPENs) is proven in TestBMPPeerUpRoundTrip, and the independence through the producer in TestRFC8671PeerUpDoesNotDependOnTheMonitoredRIB, which primes one collector per route-monitoring policy (pre-policy, post-policy, all) over the same Established peer and finds the same Peer Up on each. The Peer Down half is proven in TestRFC8671PeerDownDoesNotDependOnTheMonitoredRIB, which drives the same down event through a plugin per policy and finds the same Peer Down, header, reason and data, on each |
| `RFC8671-x-5` | Similar to Adj-RIB-In policy validation, pre-policy Adj-RIB-Out can be used to validate and audit outbound policies. (§5.2) | MAY | 5.2 - Pre-policy | **positive:** no positive test. **negative:** no negative test |
| `RFC8671-4-1` | The existing flags are defined in Section 4.2 of [RFC7854], and the remaining bits are reserved for future use.  They MUST be transmitted as 0, and their values MUST be ignored on receipt. (§4) | MUST | 4 - Per-Peer Header | **positive:** `unit/verify` [`TestRFC8671ReservedPeerFlagsTransmittedAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L66). **negative:** `unit/verify` [`TestRFC8671ReservedPeerFlagsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L108) |
| `RFC8671-5.1-1` | Post-policy Adj-RIB-Out MUST convey to the BMP receiver what is actually transmitted to the peer (§5.1) | MUST | 5.1 - Post-policy | **positive:** `unit/verify` [`TestRFC8671PostPolicyConveysTransmittedBytes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L164). **negative:** `unit/verify` [`TestRFC8671AdjRIBOutConveysNoUntransmittedUpdate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L229). **negative:** `unit/verify` [`TestRFC8671PostPolicyConveysUnknownAttributeUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L194) |
| `RFC8671-5.2-1` | The L flag MUST be set to 0 to indicate pre-policy (§5.2) | MUST | 5.2 - Pre-policy | **positive:** `unit/verify` [`TestRFC8671PrePolicyAdjRIBOutSetsLFlagZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_pre_policy_out_test.go#L89). **negative:** `unit/verify` [`TestRFC8671PrePolicyAdjRIBOutDistinguishesFromPostPolicyAndDefaultOff`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_pre_policy_out_test.go#L182) |
| `RFC8671-5.2-2` | All mandatory attributes, such as next hop, MUST be either zero or have an empty length if they are unknown at the pre-policy phase completion. (§5.2) | MUST | 5.2 - Pre-policy | **positive:** `unit/verify` [`TestRFC8671PrePolicyAdjRIBOutNormalizesUnknownMandatoryAttrs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_pre_policy_out_test.go#L239). **negative:** `unit/verify` [`TestRFC8671PrePolicyAdjRIBOutPreservesKnownMandatoryAttrsAndWithdrawals`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_pre_policy_out_test.go#L387) |
| `RFC8671-6.1-1` | The O flag MUST be set accordingly to indicate if the route monitor or route mirroring message conveys Adj-RIB-In or Adj-RIB-Out. (§6.1) | MUST | 6.1 - Route Monitoring and Route Mirroring | **positive:** `unit/verify` [`TestRFC8671OFlagSetOnAdjRIBOutMessages`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L268). **negative:** `unit/verify` [`TestRFC8671OFlagClearOnAdjRIBInMessages`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L306) |
| `RFC8671-6.2-1` | Statistics report messages are not specific to Adj-RIB-In or Adj-RIB-Out and MUST have the O flag set to zero. (§6.2) | MUST | 6.2 - Statistics Report | **positive:** `unit/verify` [`TestRFC8671StatisticsReportOnTheWireClearsTheOFlag`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/statistics_test.go#L556). **negative:** no negative test. **{single-polarity}:** the only production encoder of a Statistics Report is `senderSession.writeStatisticsReport` (internal/component/bgp/plugins/bmp/sender.go), which clears PeerFlagO on the header its caller hands in, so there is no valid ze Statistics Report carrying the O flag to reject and no negative case to construct. The positive is TestRFC8671StatisticsReportOnTheWireClearsTheOFlag (internal/component/bgp/plugins/bmp/statistics_test.go), which reports on a peer monitored for Adj-RIB-Out, reads the message off the collector socket after `sendStatisticsReports` produced it, and asserts the L flag survives so a cleared flags byte fails. It was recorded as a gap until 2026-09-06, for the reason the 2026-08-31 owner ruling gave: a green encoder test proves nothing about a requirement no production path exercises, and `statistics.go` is that path |
| `RFC8671-6.3.1-1` | When multiple Admin Labels are included, the BMP receiver MUST preserve their order (§6.3.1) | MUST | 6.3.1 - Peer Up Information | **positive:** `unit/verify` [`TestRFC8671AdminLabelOrderPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L459). **negative:** `unit/verify` [`TestRFC8671AdminLabelReversedOrderPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L476) |
| `RFC8671-7.2-1` | In case of any change that results in the alteration of behavior of an existing BMP session (i.e., changes to filtering and table names), the session MUST be bounced with a Peer Down/Peer Up sequence. (§7.2) | MUST | 7.2 - Changes to Existing BMP Session | **positive:** `unit/verify` [`TestRFC8671BehaviorChangeBouncesEachPeerAndKeepsTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L848). **negative:** `unit/verify` [`TestRFC8671RemovingEveryCollectorBouncesTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L1022). **negative:** `unit/verify` [`TestRFC8671UnrelatedBGPChangeBouncesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L967) |

## Gaps and untested MUSTs

RFC 8671 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC8671-x-1`](#rfc8671-x-1)

The per-peer header has the same structure and flags as defined in Section 4.2 of [RFC7854] with the addition of the O flag as shown here: 0 1 2 3 4 5 6 7 +-+-+-+-+-+-+-+-+ |V|L|A|O| Resv | +-+-+-+-+-+-+-+-+ *  The O flag indicates Adj-RIB-In if set to 0 and Adj-RIB-Out if set to 1. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read 2026-09-27 against the §4 quote (header layout with O in the fourth flag position; O=0 Adj-RIB-In, O=1 Adj-RIB-Out). Forbidden: O at another bit, or its meaning inverted. TestRFC8671OFlagBit4 fails if PeerFlagO != 0x10, if flags 0x10 does not decode as Adj-RIB-Out, or if 0x08 does.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8671OFlagBit4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L222) | unit/verify | revert, verified |
| positive | [`TestRFC8671OFlagBit4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L219) | unit/verify | revert, verified |

### [`RFC8671-x-2`](#rfc8671-x-2)

The L flag MUST be set to 1 to indicate post-policy. (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read 2026-10-23 against section 5.1 'The L flag MUST be set to 1 to indicate post-policy.' after peerHeaderFromEvent became (PeerHeader, error) and refuses an unknown peer AS. Both units changed only to build the AS as rpc.RemoteASOf(65001) and read the header through mustPeerHeader, which fails the test on any error, so the new refusal branch cannot satisfy or mask either assertion. TestPeerHeaderFromEventAdjRIBOut still fails if the sent (Adj-RIB-Out, post-policy only in ze) header lacks PeerFlagL; TestPeerHeaderFromEventAdjRIBIn shows L is not set unconditionally.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestPeerHeaderFromEventAdjRIBIn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L81) | unit/verify | revert, verified |
| positive | [`TestPeerHeaderFromEventAdjRIBOut`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L58) | unit/verify | revert, verified |

### [`RFC8671-x-3`](#rfc8671-x-3)

Peer Up and Down Notifications convey BGP peering session state to BMP receivers.  The state is independent of whether or not route monitoring or route mirroring messages will be sent for Adj-RIB-In, Adj-RIB-Out, or both. (§6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-23. TestRFC8671PeerDownDoesNotDependOnTheMonitoredRIB changed by one fixture line: PeerAS is now rpc.RemoteASOf(65001), the type StructuredEvent.PeerAS became, with the same known AS, so the event takes the same mirror path. The comparison of presence, per-peer header, reason and data across pre-policy, post-policy and all is unchanged and runs through handleSenderState. The other two units are unchanged. {single-polarity: positive} holds.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestBMPPeerUpRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_msg_test.go#L104) | unit/verify | revert, verified |
| positive | [`TestRFC8671PeerDownDoesNotDependOnTheMonitoredRIB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_verdict_test.go#L359) | unit/verify | revert, verified |
| positive | [`TestRFC8671PeerUpDoesNotDependOnTheMonitoredRIB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_verdict_test.go#L414) | unit/verify | revert, verified |

### [`RFC8671-4-1`](#rfc8671-4-1)

The existing flags are defined in Section 4.2 of [RFC7854], and the remaining bits are reserved for future use.  They MUST be transmitted as 0, and their values MUST be ignored on receipt. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read 2026-10-23 after peerHeaderFromEvent became (PeerHeader, error). Transmit clause: TestRFC8671ReservedPeerFlagsTransmittedAsZero, over a known AS and through mustPeerHeader (fatal on any error), still fails if peerHeaderFromEvent, locRIBPeerHeader or the byte writePeerHeader puts on the wire carries a reserved bit, for IPv4 and IPv6, both directions. Receipt clause: TestRFC8671ReservedPeerFlagsIgnoredOnReceipt fails if a header with every reserved bit set is refused or changes a defined flag, over two bases. The unit edits are the AS type change only; no assertion moved.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8671ReservedPeerFlagsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L108) | unit/verify | revert, verified |
| positive | [`TestRFC8671ReservedPeerFlagsTransmittedAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L66) | unit/verify | revert, verified |

### [`RFC8671-5.1-1`](#rfc8671-5.1-1)

Post-policy Adj-RIB-Out MUST convey to the BMP receiver what is actually transmitted to the peer (§5.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8671AdjRIBOutConveysNoUntransmittedUpdate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L229) | unit/verify | not verified |
| negative | [`TestRFC8671PostPolicyConveysUnknownAttributeUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L194) | unit/verify | revert, verified |
| positive | [`TestRFC8671PostPolicyConveysTransmittedBytes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L164) | unit/verify | revert, verified |

### [`RFC8671-5.2-1`](#rfc8671-5.2-1)

The L flag MUST be set to 0 to indicate pre-policy (§5.2)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/plugins/bmp/bmp_events.go::BMPPlugin.handleSenderPrePolicyOut moved. Independent audit against RFC 8671 Section 5.2 ('The L flag MUST be set to 0 to indicate pre-policy.') and Section 4 (O=1 indicates Adj-RIB-Out). Production code enforces O=1, L=0 across all three pre-policy Adj-RIB-Out emission paths: live Route Monitoring via peerHeaderFromEvent (EventKindUpdatePrePolicyOut branch sets PeerFlagO and clears PeerFlagL before the DirectionSent branch), cached replay routes via cacheAdjSections/peerHeaderFromEvent/writeAdjRouteLocked, and pre-policy Adj-RIB-Out End-of-RIB markers in replayPeerLocked. Positive test TestRFC8671PrePolicyAdjRIBOutSetsLFlagZero drives the real reload rail (enabling adj-rib-out-pre-policy and asserting the bounced pre-policy Adj-RIB-Out EOR carries exact flags PeerFlagO), live handleStructuredEvent over net.Pipe (asserting O=1, L=0, reserved=0 via decoded RouteMonitoring), and primeSender replay over a fresh pipe (asserting the replayed route carries O=1, L=0). Negative test TestRFC8671PrePolicyAdjRIBOutDistinguishesFromPostPolicyAndDefaultOff verifies the YANG leaf defaults to false, confirms disabled senders emit nothing for pre-policy Adj-RIB-Out events, and proves post-policy Adj-RIB-Out still carries O=1, L=1 and Adj-RIB-In carries O=0, L=0 both with adj-rib-out-pre-policy off and on. Discrimination records in rfc/discrimination/rfc8671/RFC8671-5.2-1.json prove whole-producer dependency on peerHeaderFromEvent; source inspection confirms exact L-flag bit discrimination across live, replayed, and EOR paths.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8671PrePolicyAdjRIBOutDistinguishesFromPostPolicyAndDefaultOff`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_pre_policy_out_test.go#L182) | unit/verify | revert, verified |
| positive | [`TestRFC8671PrePolicyAdjRIBOutSetsLFlagZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_pre_policy_out_test.go#L89) | unit/verify | revert, verified |

### [`RFC8671-5.2-2`](#rfc8671-5.2-2)

All mandatory attributes, such as next hop, MUST be either zero or have an empty length if they are unknown at the pre-policy phase completion. (§5.2)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/plugins/bmp/bmp_events.go::BMPPlugin.handleSenderPrePolicyOut moved. Independent audit against RFC 8671 Section 5.2 ('All mandatory attributes, such as next hop, MUST be either zero or have an empty length if they are unknown at the pre-policy phase completion.'). Production code normalizes pre-policy Adj-RIB-Out announcements in normalizePrePolicyAdjRIBOutBody (invoked both on the live handleSenderPrePolicyOut path and on cacheAdjUpdate before snapshot caching): missing ORIGIN (code 1) and AS_PATH (code 2) are emitted with empty length 0; missing, non-4-byte, or nextHopUnknown IPv4 NEXT_HOP (code 3) for inline NLRI is encoded as 0.0.0.0 (4 zero octets); MP_REACH_NLRI (code 14) with nextHopUnknown encodes Next Hop Network Address Length as 0 (empty length) via appendMPReachEmptyNextHop while preserving AFI/SAFI, reserved=0, and NLRI verbatim; withdrawal-only and End-of-RIB UPDATEs are returned unchanged. In the reactor, emitPrePolicyAdjRIBOut marks PrePolicyNextHopUnknown when transmission-time next-hop rewriting applies (nhSelfWithheld or nhMode != nhModeNone) or when an injected body already carries an unknown/zero next hop. Positive test TestRFC8671PrePolicyAdjRIBOutNormalizesUnknownMandatoryAttrs drives the live BMP sender pipe and replay primeSender across four subtests covering unknown IPv4 NEXT_HOP zeroing to 0.0.0.0 (preserving known ORIGIN/AS_PATH/NLRI), missing ORIGIN/AS_PATH/NEXT_HOP synthesis as empty/zero, unknown IPv6 MP_REACH_NLRI next hop encoded with empty length 0, and replayed snapshot normalization. Negative test TestRFC8671PrePolicyAdjRIBOutPreservesKnownMandatoryAttrsAndWithdrawals proves known IPv4 and IPv6 mandatory attributes are conveyed byte-for-byte verbatim when nextHopUnknown is false and withdrawal-only UPDATEs gain no fabricated mandatory attributes. Discrimination records in rfc/discrimination/rfc8671/RFC8671-5.2-2.json prove whole-producer dependency on normalizePrePolicyAdjRIBOutBody; source inspection confirms exact byte/length discrimination across all subtests.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8671PrePolicyAdjRIBOutPreservesKnownMandatoryAttrsAndWithdrawals`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_pre_policy_out_test.go#L387) | unit/verify | revert, verified |
| positive | [`TestRFC8671PrePolicyAdjRIBOutNormalizesUnknownMandatoryAttrs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_pre_policy_out_test.go#L239) | unit/verify | revert, verified |

### [`RFC8671-6.1-1`](#rfc8671-6.1-1)

The O flag MUST be set accordingly to indicate if the route monitor or route mirroring message conveys Adj-RIB-In or Adj-RIB-Out. (§6.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read 2026-09-27. Forbidden: O not matching the RIB conveyed on Route Monitoring or Route Mirroring. TestRFC8671OFlagSetOnAdjRIBOutMessages fails if either message for a sent UPDATE lacks O; TestRFC8671OFlagClearOnAdjRIBInMessages fails if either for a received UPDATE carries it. Both read off the collector pipe after handleStructuredEvent.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8671OFlagClearOnAdjRIBInMessages`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L306) | unit/verify | revert, verified |
| positive | [`TestRFC8671OFlagSetOnAdjRIBOutMessages`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L268) | unit/verify | revert, verified |

### [`RFC8671-6.2-1`](#rfc8671-6.2-1)

Statistics report messages are not specific to Adj-RIB-In or Adj-RIB-Out and MUST have the O flag set to zero. (§6.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read 2026-09-27. Forbidden: a Statistics Report with O=1. TestRFC8671StatisticsReportOnTheWireClearsTheOFlag fails when the report sendStatisticsReports puts on the collector pipe carries PeerFlagO for a peer whose header holds O, and asserts L survives. Single polarity is declared on the row.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC8671StatisticsReportOnTheWireClearsTheOFlag`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/statistics_test.go#L556) | unit/verify | revert, verified |

### [`RFC8671-6.3.1-1`](#rfc8671-6.3.1-1)

When multiple Admin Labels are included, the BMP receiver MUST preserve their order (§6.3.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8671AdminLabelReversedOrderPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L476) | unit/verify | revert, verified |
| positive | [`TestRFC8671AdminLabelOrderPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L459) | unit/verify | revert, verified |

### [`RFC8671-7.2-1`](#rfc8671-7.2-1)

In case of any change that results in the alteration of behavior of an existing BMP session (i.e., changes to filtering and table names), the session MUST be bounced with a Peer Down/Peer Up sequence. (§7.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read 2026-09-27. Forbidden: a behaviour-altering change (the RFC's i.e.: filtering, table names) without a Peer Down/Peer Up per peer. Filtering: TestRFC8671BehaviorChangeBouncesEachPeerAndKeepsTheSession fails unless each of two peers gets exactly one Peer Down then one Peer Up after a route-monitoring-policy change through the engine reload rail. Table names: the Loc-RIB table name is the constant locRIBTableName, so no change can occur. Negatives: TestRFC8671UnrelatedBGPChangeBouncesNothing, TestRFC8671RemovingEveryCollectorBouncesTheSession. Not asserted: that other behavior leaves (route-mirroring, loc-rib) also bounce.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8671RemovingEveryCollectorBouncesTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L1022) | unit/verify | not verified |
| negative | [`TestRFC8671UnrelatedBGPChangeBouncesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L967) | unit/verify | revert, verified |
| positive | [`TestRFC8671BehaviorChangeBouncesEachPeerAndKeepsTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L848) | unit/verify | revert, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | ze-fix agent, unowned feature-out-of-scope exclusion on rfc8671 site 5.2:1, section 5.2 re-walked 2026-10-30 |
| Signed off | 2026-10-30 |
| Register | rfc2119 |
| Source | rfc/full/rfc8671.txt |
| Source fingerprint | aed5ce37b74c3a4d |
| Record | rfc/extraction/rfc8671.json |
| Mapped sentences | 9 |
| Declined as scope | 1 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Abstract, Status of This Memo, Copyright Notice and Table of Contents. The Abstract restates section 1 and states no obligation. |
| `1` | Introduction | 0 | reviewed | Introduction. Indicative prose: what BMP monitors today, why an operator cannot see what it advertises, and that this document updates the per-peer header of RFC 7854 Section 4.2 with a new flag. No sentence directs a speaker. |
| `2` | Terminology | 0 | reviewed | Terminology. The BCP 14 key-words paragraph, which binds the key words only when they appear in all capitals. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. |
| `3` | Definitions | 1 | reviewed | Definitions. Three terms: Adj-RIB-Out quoted from RFC 4271, pre-policy Adj-RIB-Out and post-policy Adj-RIB-Out. The one capitalised keyword sits in the post-policy definition and repeats section 5.1 word for word, so it is excluded below as a duplicate. |
| `4` | Per-Peer Header | 1 | reviewed | Per-Peer Header. Adds the O flag to the RFC 7854 Section 4.2 flags field and redefines Peer Address, Peer AS, Peer BGP ID and Timestamp for O=1. The bit position and the meaning of the flag are stated indicatively ('The O flag indicates Adj-RIB-In if set to 0 and Adj-RIB-Out if set to 1') and in the bit diagram, so section 2 gives them no RFC 2119 level and the site scan cannot see them; RFC8671-x-1 is read from there and is listed as unsourced. The one site is the reserved-bits sentence, mapped below. The four redefined fields are value definitions carried by the O Flag table of rfc/short/rfc8671.md, not directives. |
| `5` | Adj-RIB-Out | 0 | reviewed | Adj-RIB-Out. A heading with no body text: 5.1 and 5.2 carry the whole section. |
| `5.1` | Post-policy | 2 | reviewed | Post-policy. States the primary use case, then two capitalised MUSTs, both mapped below. The rest is indicative: which attributes are set at transmission time, and what post-policy reflects. |
| `5.2` | Pre-policy | 2 | reviewed | Pre-policy. Two capitalised MUSTs, both conditional on the pre-policy Adj-RIB-Out view, which ze does not build and which no owner decision declines, so both are declared gaps. Site 5.2:2 is mapped to RFC8671-5.2-1 and site 5.2:1 to RFC8671-5.2-2, each carrying {gap} on its checklist line of rfc/short/rfc8671.md. Until 2026-09-05 both sites read excluded as binding a role ze does not implement, which the presumed-wrong ruling on binds-another-role bans for a role ze fills: ze IS the BMP sender; until 2026-10-30 site 5.2:1 stayed excluded as feature-out-of-scope with no owner decision behind it. The remaining prose is indicative: what the candidate route set holds for each peering session type, and that a null or loopback next hop is common before transmission. RFC8671-x-5 records that pre-policy Adj-RIB-Out is permitted and uncommon; that is read from this section's indicative prose, so it is listed as unsourced. |
| `6` | BMP Messages | 0 | reviewed | BMP Messages. Two sentences. The first says some messages carrying a per-peer header are not applicable to the Adj-RIB-In or Adj-RIB-Out distinction, and names Peer Up and Peer Down. The second, 'Unless otherwise defined, the O flag should be set to 0 in the per-peer header in BMP messages', uses a lowercase 'should', so under section 2 it carries no RFC 2119 level and the site scan does not see it. ze meets it anyway: peerHeaderFromEvent sets the O flag only for a sent-direction event, so a Peer Up, a Peer Down and a Loc-RIB message all carry O=0. |
| `6.1` | Route Monitoring and Route Mirroring | 1 | reviewed | Route Monitoring and Route Mirroring. One sentence, one capitalised MUST, mapped below. |
| `6.2` | Statistics Report | 1 | reviewed | Statistics Report. One capitalised MUST, excluded below, then the four new Stat Types 14 to 17 as value definitions. Its second sentence, 'The O flag SHOULD be ignored by the BMP receiver', is a SHOULD and never gates; ze's receiver meets it, because decodeStatisticsReport reads the per-peer header and processStatisticsReport stores the entries without consulting the O flag. |
| `6.3` | Peer Up and Down Notifications | 0 | reviewed | Peer Up and Down Notifications. No capitalised MUST-level keyword. Two indicative sentences and one SHOULD: the peering state a Peer Up or Peer Down conveys 'is independent of whether or not route monitoring or route mirroring messages will be sent for Adj-RIB-In, Adj-RIB-Out, or both', and a receiver SHOULD ignore the O flag on these two messages. RFC8671-x-3, which records that an Adj-RIB-Out Peer Up carries the same sent and received OPEN messages as an Adj-RIB-In one, is read from that independence sentence, so it is listed as unsourced. RFC8671-x-4 is NOT listed: it records that one BGP session MAY produce two Peer Up messages, and no sentence of RFC 8671 says that. It is a MAY, so it gates nothing and the reverse arithmetic asks no home for it. |
| `6.3.1` | Peer Up Information | 1 | reviewed | Peer Up Information. Defines Peer Up Information TLV type 4, Admin Label: a free-form UTF-8 string, administratively assigned, with no terminator required. One capitalised MUST on the order of multiple labels, mapped below. The type number is a value definition and the last sentence, 'The Admin Label is optional', removes any obligation to send one. |
| `7` | Other Considerations | 0 | reviewed | Other Considerations. A heading with no body text. |
| `7.1` | Peer and Update Groups | 0 | reviewed | Peer and Update Groups. Indicative prose on why a BMP sender cannot publish a simple peer group name, and why the Admin Label of section 6.3.1 answers the same need. Its last sentence puts label configuration and assignment outside the document. No directive. |
| `7.2` | Changes to Existing BMP Session | 1 | reviewed | Changes to Existing BMP Session. One sentence, one capitalised MUST, mapped below to RFC8671-7.2-1. |
| `8` | Security Considerations | 0 | reviewed | Security Considerations. Imports Section 11 of RFC 7854, states that an implementation SHOULD require sessions with authorized and trusted monitoring devices, and states that this document adds no further consideration. The SHOULD is advisory and never gates. |
| `9` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Names the registry the three subsections write to. |
| `9.1` | Addition to the BMP Peer Flags registry: flag 3, the O flag | 0 | skipped (iana) | Addition to the BMP Peer Flags registry: flag 3, the O flag. Binds IANA, not a speaker. |
| `9.2` | not stated | 0 | skipped (iana) | Additions to the BMP Statistics Types registry: types 14 to 17. Binds IANA, not a speaker. |
| `9.3` | not stated | 0 | skipped (iana) | Addition to the BMP Initiation Message TLVs registry: type 4, Admin Label. Binds IANA, not a speaker. |
| `10` | Normative References: RFC 2119, RFC 4271, RFC 7854, RFC 8174 | 0 | skipped (references) | Normative References: RFC 2119, RFC 4271, RFC 7854, RFC 8174. The section also absorbs the Acknowledgements, Contributors and Authors' Addresses blocks, none of which states an obligation. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Definitions entry for post-policy Adj-RIB-Out closes with the same sentence section 5.1 states: 'This MUST convey to the BMP receiver what is actually transmitted to the peer.' Site 5.1:1 maps that obligation to RFC8671-5.1-1. One obligation, written twice. | This MUST convey to the BMP receiver what is actually transmitted to the peer. |

## Superseded

No document obsoletes RFC 8671, so its obligations are stated where they were written.
