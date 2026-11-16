# RFC 7607 - Codification of AS 0 Processing

Codification of AS 0 processing. Every requirement listed for RFC 7607, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 5 | of 5 mandatory requirements | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 5 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 5 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 5 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 5 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 19 | 19 of 34 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests counts as proof only after this check has demonstrated that they can detect a fault |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 5 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 5 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 5 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 5 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 5 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no proof credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 5 | of 5 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 5 mandatory requirements: they add up to 5. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | Codification of AS 0 processing |
| Automatic checks | Included |
| Requirements | 5 |
| Mandatory requirements (MUST level) | 5 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 34 |
| Tests linked to requirements | 34 |
| Recorded audit verdicts | 2 |
| Fault test records | 19 |
| Summary | `rfc/short/rfc7607.md` |
| Requirement table | `rfc/requirements/rfc7607.md` |
| RFC text | `rfc/full/rfc7607.txt` |

## Automatic checks

Included in automatic checks: Five MUST-level requirements in Section 2. The receive validators in message/rfc7606.go and message/rfc7607.go apply the distinct RFC 7606 and RFC 6793 malformed-attribute procedures. The final outbound Session.writeUpdateBody guard (reactor/session_rfc7607.go) judges all four attributes after policy on originated, raw-injected and both relay rails, withdrawing offending routes without examining internal AS-zero sentinels or NLRI descriptors. rfc7607_egress_test.go reads actual Session output at both negotiated ASN widths and pairs each forbidden input with its real-AS control. Session.validateOpenPeerAS refuses peer AS zero on both OPEN rails with NOTIFICATION 2/2, and Session.sendOpen plus the YANG ASN range refuse a local AS-zero claim. Verified discrimination records determine the proven share; receive-only tests do not establish the complete outbound prohibition.

## Published support status

**Area:** Codification of AS 0 processing

**Proof:** 5 of 5 MUST proven

**Evidence:** unit 5

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 5 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **5** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (5):** [`RFC7607-2-1`](#rfc7607-2-1), [`RFC7607-2-2`](#rfc7607-2-2), [`RFC7607-2-3`](#rfc7607-2-3), [`RFC7607-2-4`](#rfc7607-2-4), [`RFC7607-2-5`](#rfc7607-2-5)

## Figures

- **MUST bar**: Ze proven 5
- **Overview bar**: Ze MUST proven 5

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 5 |
| MUST rows kept (the MUST bar) | 5 |
| Ze proven | 5 |
| OS verified | 0 |
| Proof pending | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 0 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 0 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc7607.json](../../../data/rfc/rfc7607.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7607-2-1` | A BGP speaker MUST NOT originate or propagate a route with an AS number of zero in the AS_PATH, AS4_PATH, AGGREGATOR, or AS4_AGGREGATOR attributes (§2) | MUST NOT | 2 - Behavior | **positive:** `unit/verify` [`TestRFC7607ASZeroNeverLeavesOriginatedOrRelayedRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_egress_test.go#L31). **positive:** `unit/verify` [`TestRFC7607ASZeroReplacementWithdrawsMixedFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_egress_boundaries_test.go#L24). **positive:** `unit/verify` [`TestRFC7607AggregatorASZeroIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L88). **positive:** `unit/verify` [`TestRFC7607UpdateWithASZeroIsNotAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L48). **negative:** `unit/verify` [`TestRFC7607AggregatorRealASIsKept`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L104). **negative:** `unit/verify` [`TestRFC7607RealASLeavesOriginatedAndRelayedRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_egress_test.go#L39). **negative:** `unit/verify` [`TestRFC7607UpdateWithRealASIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L68) |
| `RFC7607-2-2` | An UPDATE message that contains the AS number of zero in the AS_PATH or AGGREGATOR attribute MUST be considered as malformed and be handled by the procedures specified in [RFC7606] (§2) | MUST | 2 - Behavior | **positive:** `unit/verify` [`TestRFC7607ASPathZeroTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L29). **positive:** `unit/verify` [`TestRFC7607AggregatorASZeroIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L90). **positive:** `unit/verify` [`TestRFC7607AggregatorZeroDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L65). **positive:** `unit/verify` [`TestRFC7607CompleteUpdateASPathZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L181). **positive:** `unit/verify` [`TestRFC7607ReachesTheAttributeWalk`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L246). **positive:** `unit/verify` [`TestRFC7607UpdateWithASZeroIsNotAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L50). **negative:** `unit/verify` [`TestRFC7607ASPathNonZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L50). **negative:** `unit/verify` [`TestRFC7607AggregatorNonZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L92). **negative:** `unit/verify` [`TestRFC7607CompleteUpdateAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L194). **negative:** `unit/verify` [`TestRFC7607UpdateWithRealASIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L70) |
| `RFC7607-2-3` | An UPDATE message that contains the AS number of zero in the AS4_PATH or AS4_AGGREGATOR attribute MUST be considered as malformed and be handled by the procedures specified in [RFC6793] (§2) | MUST | 2 - Behavior | **positive:** `unit/verify` [`TestRFC7607AS4AggregatorASZeroIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L124). **positive:** `unit/verify` [`TestRFC7607AS4AggregatorZeroDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L140). **positive:** `unit/verify` [`TestRFC7607AS4PathReachesTheAttributeWalk`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L268). **positive:** `unit/verify` [`TestRFC7607AS4PathZeroDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L113). **positive:** `unit/verify` [`TestRFC7607CompleteUpdateAS4PathZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L210). **negative:** `unit/verify` [`TestRFC7607AS4AggregatorNonZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L156). **negative:** `unit/verify` [`TestRFC7607AS4AggregatorRealASIsKept`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L138). **negative:** `unit/verify` [`TestRFC7607AS4PathNonZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L130). **negative:** `unit/verify` [`TestRFC7607CompleteUpdateAS4PathAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L226) |
| `RFC7607-2-4` | If a BGP speaker receives zero as the peer AS in an OPEN message, it MUST abort the connection and send a NOTIFICATION with Error Code "OPEN Message Error" and subcode "Bad Peer AS" (§2) | MUST | 2 - Behavior | **positive:** `unit/verify` [`TestHandleOpenRejectsPeerASZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_session_open_as_test.go#L48). **positive:** `unit/verify` [`TestProcessOpenRejectsPeerASZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_session_open_as_test.go#L108). **positive:** `unit/verify` [`TestRFC7607ZeroASOpenAbortsTCP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_open_abort_tcp_test.go#L31). **negative:** `unit/verify` [`TestHandleOpenRejectsPeerASZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_session_open_as_test.go#L51). **negative:** `unit/verify` [`TestProcessOpenRejectsPeerASZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_session_open_as_test.go#L110). **negative:** `unit/verify` [`TestRFC7607RealASOpenRetainsTCP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_open_abort_tcp_test.go#L56) |
| `RFC7607-2-5` | A router MUST NOT initiate a connection claiming to be AS 0 (§2) | MUST NOT | 2 - Behavior | **positive:** `unit/verify` [`TestSendOpenRefusesLocalASZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_session_open_as_test.go#L206). **negative:** `unit/verify` [`TestSendOpenRefusesLocalASZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_session_open_as_test.go#L209) |

## Gaps and untested MUSTs

RFC 7607 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Proof state

A test reads unproven when no record shows it failing after a deliberate fault. Until then, its claim to check the requirement remains unproven.

### [`RFC7607-2-1`](#rfc7607-2-1)

A BGP speaker MUST NOT originate or propagate a route with an AS number of zero in the AS_PATH, AS4_PATH, AGGREGATOR, or AS4_AGGREGATOR attributes (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7607RealASLeavesOriginatedAndRelayedRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_egress_test.go#L39) | unit/verify | mutant, verified |
| negative | [`TestRFC7607AggregatorRealASIsKept`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L104) | unit/verify | unproven |
| negative | [`TestRFC7607UpdateWithRealASIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L68) | unit/verify | unproven |
| positive | [`TestRFC7607ASZeroReplacementWithdrawsMixedFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_egress_boundaries_test.go#L24) | unit/verify | mutant, verified |
| positive | [`TestRFC7607ASZeroNeverLeavesOriginatedOrRelayedRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_egress_test.go#L31) | unit/verify | mutant, verified |
| positive | [`TestRFC7607AggregatorASZeroIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L88) | unit/verify | unproven |
| positive | [`TestRFC7607UpdateWithASZeroIsNotAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L48) | unit/verify | unproven |

### [`RFC7607-2-2`](#rfc7607-2-2)

An UPDATE message that contains the AS number of zero in the AS_PATH or AGGREGATOR attribute MUST be considered as malformed and be handled by the procedures specified in [RFC7606] (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7607ASPathNonZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L50) | unit/verify | mutant, verified |
| negative | [`TestRFC7607AggregatorNonZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L92) | unit/verify | mutant, verified |
| negative | [`TestRFC7607CompleteUpdateAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L194) | unit/verify | unproven |
| negative | [`TestRFC7607UpdateWithRealASIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L70) | unit/verify | unproven |
| positive | [`TestRFC7607ASPathZeroTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L29) | unit/verify | mutant, verified |
| positive | [`TestRFC7607AggregatorZeroDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L65) | unit/verify | mutant, verified |
| positive | [`TestRFC7607CompleteUpdateASPathZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L181) | unit/verify | unproven |
| positive | [`TestRFC7607ReachesTheAttributeWalk`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L246) | unit/verify | unproven |
| positive | [`TestRFC7607AggregatorASZeroIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L90) | unit/verify | unproven |
| positive | [`TestRFC7607UpdateWithASZeroIsNotAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L50) | unit/verify | unproven |

### [`RFC7607-2-3`](#rfc7607-2-3)

An UPDATE message that contains the AS number of zero in the AS4_PATH or AS4_AGGREGATOR attribute MUST be considered as malformed and be handled by the procedures specified in [RFC6793] (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7607AS4AggregatorNonZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L156) | unit/verify | mutant, verified |
| negative | [`TestRFC7607AS4PathNonZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L130) | unit/verify | mutant, verified |
| negative | [`TestRFC7607CompleteUpdateAS4PathAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L226) | unit/verify | unproven |
| negative | [`TestRFC7607AS4AggregatorRealASIsKept`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L138) | unit/verify | unproven |
| positive | [`TestRFC7607AS4AggregatorZeroDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L140) | unit/verify | mutant, verified |
| positive | [`TestRFC7607AS4PathReachesTheAttributeWalk`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L268) | unit/verify | unproven |
| positive | [`TestRFC7607AS4PathZeroDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L113) | unit/verify | mutant, verified |
| positive | [`TestRFC7607CompleteUpdateAS4PathZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L210) | unit/verify | unproven |
| positive | [`TestRFC7607AS4AggregatorASZeroIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L124) | unit/verify | unproven |

### [`RFC7607-2-4`](#rfc7607-2-4)

If a BGP speaker receives zero as the peer AS in an OPEN message, it MUST abort the connection and send a NOTIFICATION with Error Code "OPEN Message Error" and subcode "Bad Peer AS" (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Read the complete RFC 7607 and every tagged unit for this row. Section 2 says: If a BGP speaker receives zero as the peer AS in an OPEN message, it MUST abort the connection and send a NOTIFICATION with Error Code "OPEN Message Error" and subcode "Bad Peer AS" (see Section 6 of [RFC4271]). The older TestHandleOpenRejectsPeerASZero covers My AS zero and AS_TRANS with a zero Four-octet AS capability, exact notification 2/2 and no OpenConfirm; matching configured real-AS controls reach OpenConfirm with no notification. TestProcessOpenRejectsPeerASZero supplies the pre-parsed collision consumer with My AS zero versus a real AS and checks the exact notification/error versus success. Those older tests alone do not observe TCP abort. The new TestRFC7607ZeroASOpenAbortsTCP supplies both field encodings on real loopback TCP through ordinary ReadAndProcess/handleOpen and pre-read acceptWithOpen/processOpen, requires the complete notification payload [2,2], then a zero-byte read with io.EOF and Idle. It starts no Session.Run, so outer teardown cannot conceal a missing producer close, and cleanup runs after these assertions. TestRFC7607RealASOpenRetainsTCP pairs each case with a matching real AS, reaches Established through the actual OPEN/KEEPALIVE exchange and sends a further KEEPALIVE on the same TCP connection. The parent baseline log names all eight cases passing. Existing native records remove the notification while preserving close and force valid AS rejection; the queued new records must additionally observe closure omission and valid-AS refusal before stamping. The collision test exercises the winner consumer and handoff, not the separate identifier election. This supersedes the unstamped weak proposal based only on the older units and makes no judgment about RFC7607-2-1, -2-2 or -2-3.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7607RealASOpenRetainsTCP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_open_abort_tcp_test.go#L56) | unit/verify | mutant, verified |
| negative | [`TestHandleOpenRejectsPeerASZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_session_open_as_test.go#L51) | unit/verify | mutant, verified |
| negative | [`TestProcessOpenRejectsPeerASZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_session_open_as_test.go#L110) | unit/verify | mutant, verified |
| positive | [`TestRFC7607ZeroASOpenAbortsTCP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_open_abort_tcp_test.go#L31) | unit/verify | mutant, verified |
| positive | [`TestHandleOpenRejectsPeerASZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_session_open_as_test.go#L48) | unit/verify | mutant, verified |
| positive | [`TestProcessOpenRejectsPeerASZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_session_open_as_test.go#L108) | unit/verify | mutant, verified |

### [`RFC7607-2-5`](#rfc7607-2-5)

A router MUST NOT initiate a connection claiming to be AS 0 (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Read the complete RFC 7607 and every tagged occurrence of this row. Section 2 says: A router MUST NOT initiate a connection claiming to be AS 0. TestSendOpenRefusesLocalASZero reaches connectionEstablished and sendOpen through Session.Accept on an installed net.Pipe; it is not an unconnected helper invocation that could fail with ErrNotConnected. The AS-zero case requires ErrLocalASZero and searches the captured socket writes for an OPEN, which must be absent. Its real-AS twin requires successful Accept, OpenSent and an actual OPEN carrying My AS 65001, so unconditional OPEN suppression or guard rejection fails. Both tags reside in this one table unit. The source connects the outgoing Connect rail and incoming Accept rail through the same connectionEstablished/sendOpen producer; sendOpen rejects LocalAS zero before preparing or writing any OPEN. The YANG ASN range also excludes zero, but the tagged proof rests on the runtime sender boundary, not on claiming a schema test was run. The two native records must verify zero-guard removal and overbroad refusal of a real local AS before stamping. No separate outbound-dial integration run or full RFC 7607 coverage is claimed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSendOpenRefusesLocalASZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_session_open_as_test.go#L209) | unit/verify | mutant, verified |
| positive | [`TestSendOpenRefusesLocalASZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_session_open_as_test.go#L206) | unit/verify | mutant, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | ze-implement agent, spec-rfcgate-6-supported-extraction-signoff |
| Signed off | 2026-08-30 |
| Register | rfc2119 |
| Source | rfc/full/rfc7607.txt |
| Source fingerprint | 1d694dc81b42f157 |
| Record | rfc/extraction/rfc7607.json |
| Mapped sentences | 5 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of This Memo, copyright notice, Abstract and Table of Contents. The Abstract states the document's scope and binds nobody. |
| `1` | Introduction | 0 | reviewed | Introduction. Explains why AS 0 is proscribed, citing the RFC 6491 ROA signal, and states that this document updates the error handling of RFC 4271 sections 6.2 and 6.3. Its one obligation-shaped sentence, 'requires that BGP implementations not accept or propagate routes containing AS 0', is a rationale for section 2 and carries no RFC 2119 keyword; section 2 states it normatively as 2:1. |
| `1.1` | Requirements Notation | 0 | reviewed | Requirements Notation. The RFC 2119 key-words paragraph. It tells a reader how to read section 2 and binds no speaker, which is why the derivation excludes it from the site inventory. |
| `2` | Behavior | 5 | reviewed | Behavior. The whole normative content of the document: five sentences, all five mapped, none excluded. Its closing paragraph, encouraging authors of future protocol extensions to keep AS 0 in mind, binds a document author rather than an implementation and carries no keyword, so the derivation raises no site for it. |
| `3` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records that IANA changed the AS 0 entry in the 16-bit Autonomous System Numbers registry to read 'Reserved'. A completed registry action, not an obligation on a speaker. |
| `4` | Security Considerations | 0 | reviewed | Security Considerations. Explains what the section 2 rules buy: a resource holder can declare an address resource unused, and a malicious party cannot hijack it by announcing AS 0. Rationale for the obligations already captured, with no keyword and no obligation of its own. |
| `5` | References | 0 | skipped (references) | References. The section heading only; its entries are in 5.1 and 5.2. |
| `5.1` | Normative References | 0 | skipped (references) | Normative References. Bibliography entries for IANA.AS_Numbers, RFC 2119, RFC 4271, RFC 6793 and RFC 7606. |
| `5.2` | Informative References | 0 | skipped (references) | Informative References. The bibliography entry for RFC 6491, plus the Acknowledgements and Authors' Addresses that follow it. |

### Excluded sentences

The review of RFC 7607 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 7607, so its obligations are stated where they were written.
