# RFC 9494 - Long-Lived Graceful Restart for BGP

Long-Lived Graceful Restart. Every requirement listed for RFC 9494, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 23 | of 25 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 25 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 25 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 25 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 103 | 103 of 115 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 2 | of 25 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Audit verdicts | 20 | of 25 mandatory requirements reviewed | 3 weak, wrong or unimplemented, 3 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 25 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 25 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 25 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 25 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 25 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 25 | of 36 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 25 mandatory requirements: they add up to 25. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | Long-Lived Graceful Restart |
| Automatic checks | Included |
| Requirements | 36 |
| Mandatory requirements (MUST level) | 25 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 2 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 115 |
| Tests linked to requirements | 115 |
| Recorded audit verdicts | 20 |
| Fault test records | 103 |
| Summary | `rfc/short/rfc9494.md` |
| Requirement table | `rfc/requirements/rfc9494.md` |
| RFC text | `rfc/full/rfc9494.txt` |

## Automatic checks

Included in automatic checks: Long-Lived Graceful Restart for BGP

## Published support status

**Area:** Long-Lived Graceful Restart

**Test verification:** 23 of 25 MUST proven

**Evidence:** functional 2, unit 21

**Unmet:** RFC9494-4.7.2-1, RFC9494-4.7.2-2


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 23 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 2 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **25** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (23):** [`RFC9494-3.1-1`](#rfc9494-3.1-1), [`RFC9494-4.1-1`](#rfc9494-4.1-1), [`RFC9494-3.1-2`](#rfc9494-3.1-2), [`RFC9494-4-1`](#rfc9494-4-1), [`RFC9494-4.2-1`](#rfc9494-4.2-1), [`RFC9494-4.2-2`](#rfc9494-4.2-2), [`RFC9494-4.2-3`](#rfc9494-4.2-3), [`RFC9494-4.2-4`](#rfc9494-4.2-4), [`RFC9494-4.2-5`](#rfc9494-4.2-5), [`RFC9494-4.2-6`](#rfc9494-4.2-6), [`RFC9494-4.2-7`](#rfc9494-4.2-7), [`RFC9494-4.2-8`](#rfc9494-4.2-8), [`RFC9494-4.2-9`](#rfc9494-4.2-9), [`RFC9494-4.3-1`](#rfc9494-4.3-1), [`RFC9494-4.3-2`](#rfc9494-4.3-2), [`RFC9494-4.4-1`](#rfc9494-4.4-1), [`RFC9494-4.5-1`](#rfc9494-4.5-1), [`RFC9494-4.6-1`](#rfc9494-4.6-1), [`RFC9494-4.6-2`](#rfc9494-4.6-2), [`RFC9494-4.6-3`](#rfc9494-4.6-3), [`RFC9494-5-1`](#rfc9494-5-1), [`RFC9494-5-2`](#rfc9494-5-2), [`RFC9494-4.2-10`](#rfc9494-4.2-10)

**Annotated (including scoped evidence) (2):** [`RFC9494-4.7.2-1`](#rfc9494-4.7.2-1), [`RFC9494-4.7.2-2`](#rfc9494-4.7.2-2)

## Figures

- **MUST bar**: Ze verified 23, Not built 2
- **Overview bar**: Ze MUST verified 23, SHOULD verified 1, Not assessed 10, Not built 2

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 25 |
| MUST rows kept (the MUST bar) | 25 |
| Ze verified | 23 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 2 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 11 |
| SHOULD verified | 1 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 10 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc9494.json](../../../data/rfc/rfc9494.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9494-3.1-1` | If the LLGR capability is advertised, the Graceful Restart capability [RFC4724] MUST also be advertised; see Section 4.1. (§3.1, §4.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC9494_LLGRCapDeclaredWithGRCap`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L29). **negative:** `unit/verify` [`TestRFC9494_NoLLGRCapWithoutGRContainer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L53) |
| `RFC9494-4.1-1` | If it is not so advertised, the LLGR Capability MUST be disregarded. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestHandleStructuredOpenLLGRNoGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_event_test.go#L610). **negative:** `unit/verify` [`TestHandleStructuredOpenGRPlusLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_event_test.go#L689) |
| `RFC9494-3.1-2` | The remaining bits are reserved and MUST be set to zero by the sender and ignored by the receiver. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC9494_FlagsReservedBitsZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L71). **negative:** `unit/verify` [`TestRFC9494_FlagsReservedBitsIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L91) |
| `RFC9494-4-1` | If a BGP speaker is configured to support the procedures of this document, it MUST use BGP Capabilities Advertisement [RFC5492] to advertise the Long-Lived Graceful Restart Capability. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestExtractLLGRCapabilities_Basic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_test.go#L673). **negative:** `unit/verify` [`TestExtractLLGRCapabilities_NoLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_test.go#L700) |
| `RFC9494-4.2-1` | After the session goes down, and before the session is re- established, the stale routes for an AFI/SAFI MUST be retained. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestLLGROmittedGRFamilyReadvertisementDoesNotDelaySiblingExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L211). **positive:** `unit/verify` [`TestLLGROmittedGRFamilyRetainsRealRIB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L27). **positive:** `unit/verify` [`TestLLGROmittedGRFamilyTimerSurvivesConventionalEntry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L110). **positive:** `unit/verify` [`TestLLSTTimerExpiry_LastFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L708). **positive:** `unit/verify` [`TestRFC9494DelayedDownPreservesLLSTDeadlines`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L176). **positive:** `unit/verify` [`TestRFC9494RestartTimeThenLongLivedStaleTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L137). **positive:** `unit/verify` [`TestRFC9494ZeroRestartTimeRetainsThroughTheLLGRPeriod`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L67). **negative:** `unit/verify` [`TestLLGROmittedGRFamilyWithoutLLSTIsRemoved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L164). **negative:** `unit/verify` [`TestOnTimerExpired_WithoutLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L606). **negative:** `unit/verify` [`TestRFC9494BothTimesZeroRetainsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L101) |
| `RFC9494-4.2-2` | * For each AFI/SAFI for which it has received a nonzero Long-Lived Stale Time, the helper router MUST start a timer for that Long- Lived Stale Time. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestLLGROmittedGRFamilyReadvertisementDoesNotDelaySiblingExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L212). **positive:** `unit/verify` [`TestLLGROmittedGRFamilyRetainsRealRIB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L28). **positive:** `unit/verify` [`TestLLGROmittedGRFamilyTimerSurvivesConventionalEntry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L111). **positive:** `unit/verify` [`TestOnTimerExpired_WithLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L571). **positive:** `unit/verify` [`TestRFC9494RestartTimeThenLongLivedStaleTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L138). **negative:** `unit/verify` [`TestLLGROmittedGRFamilyWithoutLLSTIsRemoved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L165). **negative:** `unit/verify` [`TestOnSessionDown_ZeroGR_ZeroLLST`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L770). **negative:** `unit/verify` [`TestRFC9494BothTimesZeroRetainsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L102) |
| `RFC9494-4.2-3` | If the timer for the Long-Lived Stale Time for a given AFI/SAFI expires before the session is re-established, the helper MUST delete all stale routes of that AFI/SAFI from the neighbor that it is retaining. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestLLSTTimerExpiry_SingleFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L664). **positive:** `unit/verify` [`TestRFC9494ConsecutiveRestartExpiryDeletesEveryConnectionsRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_consecutive_rib_test.go#L168). **positive:** `unit/verify` [`TestRFC9494DelayedDownPreservesLLSTDeadlines`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L177). **positive:** `unit/verify` [`TestRFC9494LLSTExpiryDeletesTheFamilysStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L207). **negative:** `unit/verify` [`TestRFC9494ConsecutiveRestartExpiryDeletesEveryConnectionsRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_consecutive_rib_test.go#L169). **negative:** `unit/verify` [`TestRFC9494DelayedDownPreservesLLSTDeadlines`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L178) |
| `RFC9494-4.2-4` | * The helper router MUST attach the LLGR_STALE community to the stale routes being retained. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestAttachCommunity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_rib_gr_test.go#L621). **positive:** `unit/verify` [`TestRFC9494ConsecutiveRestartMarksTheLaterConnectionsRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_consecutive_rib_test.go#L148). **positive:** `unit/verify` [`TestRFC9494ReconnectDuringLLGREntryStillAttachesLLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_reconnect_entry_test.go#L109). **positive:** `unit/verify` [`TestRFC9494ZeroRestartTimeRetainsThroughTheLLGRPeriod`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L68). **negative:** `unit/verify` [`TestRFC9494ReconnectDuringLLGREntryLeavesTheNewConnectionsRouteFresh`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_reconnect_entry_test.go#L152). **negative:** `unit/verify` [`TestRFC9494_FreshRoutesDoNotGetLLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L34) |
| `RFC9494-4.2-5` | * If any of the routes from the peer have been marked with the NO_LLGR community, either as sent by the peer or as the result of a configured policy, they MUST NOT be retained and MUST be removed as per the normal operation of [RFC4271]. (§4.2) | MUST NOT | 4.2 | **positive:** `unit/verify` [`TestDeleteWithCommunity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_rib_gr_test.go#L662). **positive:** `unit/verify` [`TestRFC9494NoLLGRSweepWithdrawsTheBestPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_no_llgr_withdraw_test.go#L80). **positive:** `unit/verify` [`TestRFC9494ReconnectDuringLLGREntryStillRemovesNoLLGRRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_reconnect_entry_test.go#L132). **positive:** `unit/verify` [`TestRFC9494ZeroRestartTimeRetainsThroughTheLLGRPeriod`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L69). **negative:** `unit/verify` [`TestRFC9494SweepKeepsTheBestPathWithoutNoLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_no_llgr_withdraw_test.go#L100). **negative:** `unit/verify` [`TestRFC9494_StaleRouteWithoutNoLLGRRetained`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L63). **positive:** `functional/verify` [`llgr-import-no-llgr.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/llgr-import-no-llgr.ci#L6). **negative:** `functional/verify` [`llgr-import-no-llgr.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/llgr-import-no-llgr.ci#L7) |
| `RFC9494-4.2-6` | * The helper router MUST perform the procedures listed in Section 4.3. (§4.2, §4.3) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC9494_LLGRStaleRouteIsLeastPreferred`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L137). **negative:** `unit/verify` [`TestRFC9494_RouteWithoutLLGRStaleNotLeastPreferred`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L175) |
| `RFC9494-4.2-7` | However, in the case of consecutive restarts, the previously marked stale routes MUST NOT be deleted before the timer for the Long-Lived Stale Time expires. (§4.2) | MUST NOT | 4.2 | **positive:** `unit/verify` [`TestRFC9494ConsecutiveRestartKeepsTheEarlierConnectionsStaleRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_consecutive_rib_test.go#L125). **positive:** `unit/verify` [`TestRFC9494ConsecutiveRestartPreservesLLGRStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_consecutive_test.go#L32). **negative:** `unit/verify` [`TestRFC9494ConsecutiveRestartPreservesLLGRStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_consecutive_test.go#L36) |
| `RFC9494-4.2-8` | Similar to [RFC4724], once the LLGR Period begins, the Helper MUST immediately remove all the stale routes from the peer that it is retaining for that address family if any of the following occur: * the F bit for a specific address family is not set in the newly received LLGR Capability, or * a specific address family is not included in the newly received LLGR Capability, or * the LLGR and accompanying GR Capability are not received in the re-established session at all. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestOnSessionReestablished_DuringLLGR_NoCaps`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L849). **positive:** `unit/verify` [`TestOnSessionReestablished_FamilyAbsentFromLLGRPurges`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_reconnect_test.go#L65). **positive:** `unit/verify` [`TestOnSessionReestablished_LLGRFBitClearPurges`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_reconnect_test.go#L40). **positive:** `unit/verify` [`TestOnSessionReestablished_NoLLGRCapPurgesLLGRPeriodFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_reconnect_test.go#L93). **negative:** `unit/verify` [`TestOnSessionReestablished_BothFBitsSetKeepsLLGRPeriodFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_reconnect_test.go#L133). **negative:** `unit/verify` [`TestOnSessionReestablished_DuringLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L798). **negative:** `unit/verify` [`TestOnSessionReestablished_GRPeriodFamilyIgnoresLLGRBullets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_reconnect_test.go#L156) |
| `RFC9494-4.2-9` | If a Long-Lived Stale Time timer is running for routes with a given AFI/SAFI received from a peer, it MUST NOT be updated (other than by manual operator intervention) until the peer has established and synchronized a new session. (§4.2) | MUST NOT | 4.2 | **positive:** `unit/verify` [`TestRFC9494LLSTTimerNotUpdatedUntilSynchronized`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_consecutive_test.go#L157). **negative:** `unit/verify` [`TestRFC9494LLSTTimerNotUpdatedUntilSynchronized`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_consecutive_test.go#L160) |
| `RFC9494-4.3-1` | A BGP speaker that has advertised the Long-Lived Graceful Restart Capability to a neighbor MUST perform the following upon receiving a route from that neighbor with the LLGR_STALE community or upon attaching the LLGR_STALE community itself per Section 4.2: * Treat the route as the least preferred in route selection (see below). (§4.3, §4.4) | MUST | 4.3 | **positive:** `unit/verify` [`TestComparePair_LLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L820). **positive:** `unit/verify` [`TestLLGRIngressFilter_MarksReceivedStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_ingress_test.go#L72). **positive:** `unit/verify` [`TestRFC9494LLGRStaleLosesEveryLaterTieBreak`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_least_preferred_test.go#L37). **positive:** `unit/verify` [`TestRFC9494NegotiateWithKeysOnAdvertisedLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9494_llgr_trigger_test.go#L46). **positive:** `unit/verify` [`TestRFC9494ReceivedEventCarriesIngressStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9494_llgr_ingress_meta_test.go#L101). **positive:** `unit/verify` [`TestRFC9494ReceivedLLGRStaleIsLeastPreferred`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_received_stale_test.go#L125). **positive:** `unit/verify` [`TestRFC9494SelfAttachedLLGRStaleIsLeastPreferred`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_self_attach_test.go#L95). **negative:** `unit/verify` [`TestComparePair_GRStaleCompetesNormally`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L856). **negative:** `unit/verify` [`TestLLGRIngressFilter_NoLLGRSentLeavesFresh`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_ingress_test.go#L91). **negative:** `unit/verify` [`TestRFC9494NegotiateWithKeysOnAdvertisedLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9494_llgr_trigger_test.go#L48). **negative:** `unit/verify` [`TestRFC9494ReceivedEventNoLLGRSessionStaysFresh`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9494_llgr_ingress_meta_test.go#L117). **negative:** `unit/verify` [`TestRFC9494ReceivedLLGRStaleFromNonLLGRNeighborCompetesNormally`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_received_stale_test.go#L198). **negative:** `unit/verify` [`TestRFC9494ReceivedOtherCommunityCompetesNormally`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_received_stale_test.go#L146). **negative:** `unit/verify` [`TestRFC9494SelfAttachSkipsRouteLearnedAfterLoss`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_self_attach_test.go#L114). **positive:** `functional/verify` [`llgr-stale-received-depreferenced.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/llgr-stale-received-depreferenced.ci#L10). **negative:** `functional/verify` [`llgr-stale-received-depreferenced.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/llgr-stale-received-depreferenced.ci#L11) |
| `RFC9494-4.3-2` | The LLGR_STALE community MUST NOT be removed when the route is further advertised (§4.3) | MUST NOT | 4.3 | **positive:** `unit/verify` [`TestRFC9494LLGRStaleNotRemovedOnEgress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_community/rfc9494_llgr_stale_preserve_test.go#L26). **negative:** `unit/verify` [`TestRFC9494LLGRStaleNotRemovedOnEgress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_community/rfc9494_llgr_stale_preserve_test.go#L30) |
| `RFC9494-4.4-1` | A least preferred route MUST be treated as less preferred than any other route that is not also least preferred. (§4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestSelectBest_LLGRStaleDepreference`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L755). **negative:** `unit/verify` [`TestSelectBest_BothLLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L783) |
| `RFC9494-4.5-1` | If the LLGR Capability is received without an accompanying GR Capability, the LLGR Capability MUST be ignored, that is, the implementation MUST behave as though no LLGR Capability has been received. (§4.5) | MUST | 4.5 | **positive:** `unit/verify` [`TestHandleEventOpenLLGR_NoGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_event_test.go#L301). **negative:** `unit/verify` [`TestHandleEventOpenLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_event_test.go#L266) |
| `RFC9494-4.6-1` | The neighbors MUST be internal (Internal BGP (IBGP) or Confederation) neighbors. (§4.6) | MUST | 4.6 | **positive:** `unit/verify` [`TestLLGREgressFilter_IBGPPartial`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L429). **negative:** `unit/verify` [`TestLLGREgressFilter_EBGPNonLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L399) |
| `RFC9494-4.6-2` | * The NO_EXPORT community [RFC1997] MUST be attached to the stale routes. (§4.6) | MUST | 4.6 | **positive:** `unit/verify` [`TestLLGREgressFilter_IBGPPartial`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L433). **positive:** `unit/verify` [`TestLLGREgressFilter_NilStateDepreferencesIBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L182). **negative:** `unit/verify` [`TestLLGREgressFilter_LLGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L369) |
| `RFC9494-4.6-3` | * The stale routes MUST have their LOCAL_PREF set to zero. (§4.6) | MUST | 4.6 | **positive:** `unit/verify` [`TestLLGREgressFilter_IBGPPartial`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L436). **positive:** `unit/verify` [`TestLLGREgressFilter_NilStateDepreferencesIBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L186). **negative:** `unit/verify` [`TestLLGREgressFilter_LLGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L373) |
| `RFC9494-4.7.2-1` | In addition to including the path attributes derived from the ATTR_SET attribute in the advertised route as per [RFC6368], the PE router MUST also include the LLGR_STALE community if it is present in the path attributes of the imported route, even if it is not present in the ATTR_SET attribute. (§4.7.2) | MUST | 4.7.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze fills the PE role on the control plane (it originates and reflects VPN routes) but implements no ATTR_SET attribute and no RFC 6368 PE-CE model, and no owner decision declines them: the attribute code table (internal/core/bgp/attribute/attribute.go) holds no code 128 and no VRF imports a VPN route into a CE session, so no imported route's LLGR_STALE is carried |
| `RFC9494-4.7.2-2` | In addition to including in the VPN route the ATTR_SET derived from the path attributes as per [RFC6368], the PE router MUST also include the LLGR_STALE community in the VPN route if it is present in the path attributes of the route as received from the CE. (§4.7.2) | MUST | 4.7.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no VRF export of a CE route into a VPN address family and no owner decision declines it: route-target handling is only the extended community and the RTC NLRI codec (internal/component/bgp/plugins/nlri/rtc), so no CE route's LLGR_STALE reaches a VPN route |
| `RFC9494-5-1` | Implementations MUST NOT enable these procedures by default. (§5) | MUST NOT | 5 | **positive:** `unit/verify` [`TestRFC9494HelperProceduresOnPerConfiguredFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_helper_default_red_test.go#L117). **positive:** `unit/verify` [`TestRFC9494_LLGRNotEnabledByDefault`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L119). **negative:** `unit/verify` [`TestRFC9494HelperProceduresOffWithoutLocalConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_helper_default_red_test.go#L81). **negative:** `unit/verify` [`TestRFC9494_LLGREnabledByExplicitConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L144) |
| `RFC9494-5-2` | They MUST require affirmative configuration per AFI/SAFI in order to enable them. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC9494_PerFamilyLLGRConfigPositive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_per_family_test.go#L48). **negative:** `unit/verify` [`TestRFC9494_PerFamilyLLGRConfigNegative`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_per_family_test.go#L100) |
| `RFC9494-4.2-10` | If the session subsequently resets prior to becoming synchronized, any remaining routes (for the AFI/SAFI whose LLST timer expired) MUST be removed immediately. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC9494ResetAfterSyncLLSTExpiryRemovesImmediately`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_consecutive_test.go#L259). **negative:** `unit/verify` [`TestRFC9494ResetAfterSyncLLSTExpiryRemovesImmediately`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_consecutive_test.go#L262) |
| `RFC9494-4.2-11` | The timers received in the Long-Lived Graceful Restart Capability SHOULD be modifiable by local configuration, which may impose an upper bound, a lower bound, or both on their respective values. (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9494-4.3-3` | The route SHOULD NOT be advertised to any neighbor from which the Long-Lived Graceful Restart Capability has not been received. The exception is described in Section 4.6. (§4.3) | SHOULD NOT | 4.3 | **positive:** `unit/verify` [`TestBuildRelayUpdateMarksRebuilt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9494_relay_rebuilt_test.go#L69). **positive:** `unit/verify` [`TestLLGREgressFilter_NilStateWithdrawsEBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L156). **positive:** `unit/verify` [`TestLLGREgressFilter_RebuiltRecoversReceivedStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_rails_test.go#L106). **positive:** `unit/verify` [`TestLLGREgressFilter_StateLoadedWithdrawsEBGPNonLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_rails_test.go#L46). **positive:** `unit/verify` [`TestPeerUpReplayReportsSourceIngressLevel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_replay_stale_test.go#L131). **positive:** `unit/verify` [`TestRFC9494CachedForwardWithholdsReceivedStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9494_llgr_cached_forward_test.go#L275). **positive:** `unit/verify` [`TestRFC9494RouteServerForwardWithholdsReceivedStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9494_llgr_cached_forward_test.go#L308). **positive:** `unit/verify` [`TestRIBReplayMetaReadsSourceIngressLevel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_replay_stale_test.go#L177). **negative:** `unit/verify` [`TestBuildRelayUpdateMarksRebuilt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9494_relay_rebuilt_test.go#L73). **negative:** `unit/verify` [`TestLLGREgressFilter_GRStaleLevelOnePassesThrough`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_rails_test.go#L73). **negative:** `unit/verify` [`TestLLGREgressFilter_RebuiltRecoversReceivedStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_rails_test.go#L110). **negative:** `unit/verify` [`TestLLGREgressFilter_StateLoadedStillAdvertisesToLLGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L340). **negative:** `unit/verify` [`TestPeerUpReplayReportsSourceIngressLevel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_replay_stale_test.go#L135). **negative:** `unit/verify` [`TestRFC9494CachedForwardWithholdsReceivedStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9494_llgr_cached_forward_test.go#L277). **negative:** `unit/verify` [`TestRFC9494RouteServerForwardWithholdsReceivedStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9494_llgr_cached_forward_test.go#L310). **negative:** `unit/verify` [`TestRIBReplayMetaReadsSourceIngressLevel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_replay_stale_test.go#L182). **positive:** `functional/verify` [`llgr-egress-state-unloaded.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/llgr-egress-state-unloaded.ci#L23) |
| `RFC9494-4.7.1-1` | Finally, if this exception is used, the implementation SHOULD, by default, attach the NO_EXPORT community to the routes in question, as an additional protection against stale routes spreading without limit. (§4.7.1) | SHOULD | 4.7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9494-3.2-1` | An implementation MAY allow users to configure policies that accept, reject, or modify routes based on the presence or absence of this community. (§3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9494-3.3-1` | An implementation MAY allow users to configure policies that accept, reject, or modify routes based on the presence or absence of this community. (§3.3) | MAY | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9494-4.2-12` | The value of a Long-Lived Stale Time in the capability received from a neighbor MAY be reduced by local configuration. (§4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9494-4.6-4` | However, to facilitate incremental deployment, stale routes MAY be advertised to neighbors that have not advertised the Long-Lived Graceful Restart Capability under the following conditions: * The neighbors MUST be internal (Internal BGP (IBGP) or Confederation) neighbors. (§4.6) | MAY | 4.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9494-4.7.1-2` | For this reason, an implementation MAY advertise stale routes over a PE-CE session, when explicitly configured to do so. (§4.7.1) | MAY | 4.7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9494-4.7.1-3` | an implementation MAY advertise stale routes over a PE-CE session, when explicitly configured to do so. That is, the second rule listed in Section 4.3 MAY be disregarded in such cases. (§4.7.1) | MAY | 4.7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9494-4.7.1-4` | Attachment of the NO_EXPORT community MAY be disabled by explicit configuration in order to accommodate exceptional cases. (§4.7.1) | MAY | 4.7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9494-4.7.2-3` | If the CE router does not support the procedures of this document: Then the optional procedures of Section 4.6 MAY be followed, attaching the NO_EXPORT community and setting the value of LOCAL_PREF to zero, overriding the value found in the ATTR_SET. (§4.7.2) | MAY | 4.7.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9494-4.7.2-1`](#rfc9494-4.7.2-1) In addition to including the path attributes derived from the ATTR_SET attribute in the advertised route as per [RFC6368], the PE router MUST also include the LLGR_STALE community if it is present in the path attributes of the imported route, even if it is not present in the ATTR_SET attribute. (§4.7.2) | {gap}, no test | ze fills the PE role on the control plane (it originates and reflects VPN routes) but implements no ATTR_SET attribute and no RFC 6368 PE-CE model, and no owner decision declines them: the attribute code table (internal/core/bgp/attribute/attribute.go) holds no code 128 and no VRF imports a VPN route into a CE session, so no imported route's LLGR_STALE is carried |
| [`RFC9494-4.7.2-2`](#rfc9494-4.7.2-2) In addition to including in the VPN route the ATTR_SET derived from the path attributes as per [RFC6368], the PE router MUST also include the LLGR_STALE community in the VPN route if it is present in the path attributes of the route as received from the CE. (§4.7.2) | {gap}, no test | ze implements no VRF export of a CE route into a VPN address family and no owner decision declines it: route-target handling is only the extended community and the RTC NLRI codec (internal/component/bgp/plugins/nlri/rtc), so no CE route's LLGR_STALE reaches a VPN route |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC9494-3.1-1`](#rfc9494-3.1-1)

If the LLGR capability is advertised, the Graceful Restart capability [RFC4724] MUST also be advertised; see Section 4.1. (§3.1, §4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent re-audit against RFC 9494 Sections 3.1 and 4.1 ('If the LLGR capability is advertised, the Graceful Restart capability [RFC4724] MUST also be advertised') after splitting graceful-restart into short-lived and long-lived > family. parseLLGRCapValue requires capMap['graceful-restart'] to be a map, parseGRCapValue emits a non-empty code-64 payload ('0000' when short-lived is omitted, or header + tuples when present) for every peer with that container, and both share grCapabilityFor for peer/group precedence. Positive TestRFC9494_LLGRCapDeclaredWithGRCap asserts both code 71 and code 64 are declared for the same peer under configWithLLST. Negative TestRFC9494_NoLLGRCapWithoutGRContainer places long-lived > family outside graceful-restart and asserts neither code 71 nor code 64 is declared. Bounded to the plugin capability-declaration boundary; reactor OPEN framing is not asserted in these units. Note: in rfc/discrimination/rfc9494/RFC9494-3.1-1.json, the positive extractLLGRCapabilities revert record still carries prior producer-sha 5e2ede5080554e41 (vs current f3f24d396d6d7969) and must be renewed by main before ./le rfc check.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9494_NoLLGRCapWithoutGRContainer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L53) | unit/verify | revert, verified |
| positive | [`TestRFC9494_LLGRCapDeclaredWithGRCap`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L29) | unit/verify | revert, verified |

### [`RFC9494-4.1-1`](#rfc9494-4.1-1)

If it is not so advertised, the LLGR Capability MUST be disregarded. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent current-source rejudgment, not authored by this judge. Read the complete normative Sections4.1-4.6 and5 context, current requirement/derived ledger, every whole tagged question and actual helpers/producers for this row. Preserve the existing verdict and scope, without claiming whole-RFC conformance. Section4.1 requires disregarding LLGR received without GR. Both complete structured OPEN tests use correctly framed capability TLVs: absent code64 leaves neither capability in the owner701 map; the conforming code64+71 input preserves two families, F bits, RestartTime240 and exact received LLST7200/3600. These are exact absent/present capability outcomes, not malformed OPEN rejection. The captured original connection41 comes from grEventFixture and matches the stored capabilities. No new machine record exists for these two tags; the enforced verdict is the source assertion judgment, not a new runtime claim.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestHandleStructuredOpenGRPlusLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_event_test.go#L689) | unit/verify | revert, verified |
| positive | [`TestHandleStructuredOpenLLGRNoGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_event_test.go#L610) | unit/verify | revert, verified |

### [`RFC9494-3.1-2`](#rfc9494-3.1-2)

The remaining bits are reserved and MUST be set to zero by the sender and ignored by the receiver. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Two clauses, both asserted. Sender: TestRFC9494_FlagsReservedBitsZeroOnSend fails unless the encoded Flags octet is exactly 0x80 (any reserved bit set goes red). Receiver: TestRFC9494_FlagsReservedBitsIgnoredOnReceive fails if a 0xFF Flags octet errors or decodes differently from 0x80, and if 0x7F leaks into the F bit.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9494_FlagsReservedBitsIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L91) | unit/verify | revert, verified |
| positive | [`TestRFC9494_FlagsReservedBitsZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L71) | unit/verify | revert, verified |

### [`RFC9494-4-1`](#rfc9494-4-1)

If a BGP speaker is configured to support the procedures of this document, it MUST use BGP Capabilities Advertisement [RFC5492] to advertise the Long-Lived Graceful Restart Capability. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent re-audit against RFC 9494 Section 4 ('If a BGP speaker is configured to support the procedures of this document, it MUST use BGP Capabilities Advertisement [RFC5492] to advertise the Long-Lived Graceful Restart Capability') after updating test inputs to graceful-restart > short-lived and long-lived > family. Positive TestExtractLLGRCapabilities_Basic drives extractLLGRCapabilities (wired into grCapabilities via DeclareCapabilitiesFrom) with ipv4/unicast stale-time 3600 and asserts one CapabilityDecl with Code 71, hex encoding, peer 192.168.1.1, and payload decoding to one family with LLST 3600. Negative TestExtractLLGRCapabilities_NoLLGR passes short-lived-only config and asserts an empty declaration slice. Both units carry current extractLLGRCapabilities revert records (producer-sha f3f24d396d6d7969). Bounded to the plugin capability-declaration boundary; OPEN wire framing is not asserted in these units.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestExtractLLGRCapabilities_NoLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_test.go#L700) | unit/verify | revert, verified |
| positive | [`TestExtractLLGRCapabilities_Basic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_test.go#L673) | unit/verify | revert, verified |

### [`RFC9494-4.2-1`](#rfc9494-4.2-1)

After the session goes down, and before the session is re- established, the stale routes for an AFI/SAFI MUST be retained. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent re-audit after 26d70dfa0d against RFC 9494 Section 4.2 full text. Single restart: retainPeerFamilies now reads the retain-routes answer (dispatchSourceAnswer, recordEarlierRetained) but sends the same retain-routes and mark-stale commands; with no earlier connection retainedConnections answers the source alone, so onLLGRFamilyExpired and onLLGRComplete still send exactly one purge-stale and one release-routes to that connection. The real-RIB helper newGRWithRealRIB now routes every command through answerHook, so the tagged real-RIB roots (TestRFC9494ZeroRestartTimeRetainsThroughTheLLGRPeriod, TestRFC9494RestartTimeThenLongLivedStaleTime, TestRFC9494DelayedDownPreservesLLSTDeadlines, the four omitted-GR-family roots) exercise the new answer-reading path and still require level-1 then level-2 retention with 1ns boundaries before deletion; negatives TestOnTimerExpired_WithoutLLGR, TestRFC9494BothTimesZeroRetainsNothing and TestLLGROmittedGRFamilyWithoutLLSTIsRemoved still require removal. The RIB change (retainPhysicalStateLocked no longer refuses while an earlier connection owns routes) repairs the consecutive-restart retention, whose own row RFC9494-4.2-7 tags it. The new reconnect revocation in llgrFamilyActive acts only after re-establishment, outside this row's down interval.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestOnTimerExpired_WithoutLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L606) | unit/verify | not verified |
| negative | [`TestRFC9494BothTimesZeroRetainsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L101) | unit/verify | revert, verified |
| negative | [`TestLLGROmittedGRFamilyWithoutLLSTIsRemoved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L164) | unit/verify | revert, verified |
| positive | [`TestLLSTTimerExpiry_LastFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L708) | unit/verify | not verified |
| positive | [`TestRFC9494DelayedDownPreservesLLSTDeadlines`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L176) | unit/verify | revert, verified |
| positive | [`TestRFC9494RestartTimeThenLongLivedStaleTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L137) | unit/verify | revert, verified |
| positive | [`TestRFC9494ZeroRestartTimeRetainsThroughTheLLGRPeriod`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L67) | unit/verify | revert, verified |
| positive | [`TestLLGROmittedGRFamilyReadvertisementDoesNotDelaySiblingExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L211) | unit/verify | revert, verified |
| positive | [`TestLLGROmittedGRFamilyRetainsRealRIB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L27) | unit/verify | revert, verified |
| positive | [`TestLLGROmittedGRFamilyTimerSurvivesConventionalEntry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L110) | unit/verify | revert, verified |

### [`RFC9494-4.2-2`](#rfc9494-4.2-2)

* For each AFI/SAFI for which it has received a nonzero Long-Lived Stale Time, the helper router MUST start a timer for that Long- Lived Stale Time. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent re-audit after 26d70dfa0d. Per-family LLST timers are still armed only in enterLLGRLocked (unchanged) at the GR deadline or immediately for omitted-GR families from onSessionDownPlan, whose only change adds preservedLLGRFams; restalePreservedLLGR arms no new timer for a family whose period runs, as RFC9494-4.2-9 requires. The new llgrFamilyActive reconnect guard sits after the timer is armed, so it cannot suppress the timer. Tagged roots unchanged and now run through the RIB's real answers: TestOnTimerExpired_WithLLGR, TestRFC9494RestartTimeThenLongLivedStaleTime and the three omitted-family positives require the per-family timer and its exact 1ns expiry; negatives TestOnSessionDown_ZeroGR_ZeroLLST, TestRFC9494BothTimesZeroRetainsNothing and TestLLGROmittedGRFamilyWithoutLLSTIsRemoved require no timer and no retention for a zero or unexchanged LLST.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestOnSessionDown_ZeroGR_ZeroLLST`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L770) | unit/verify | not verified |
| negative | [`TestRFC9494BothTimesZeroRetainsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L102) | unit/verify | revert, verified |
| negative | [`TestLLGROmittedGRFamilyWithoutLLSTIsRemoved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L165) | unit/verify | revert, verified |
| positive | [`TestOnTimerExpired_WithLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L571) | unit/verify | not verified |
| positive | [`TestRFC9494RestartTimeThenLongLivedStaleTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L138) | unit/verify | revert, verified |
| positive | [`TestLLGROmittedGRFamilyReadvertisementDoesNotDelaySiblingExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L212) | unit/verify | revert, verified |
| positive | [`TestLLGROmittedGRFamilyRetainsRealRIB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L28) | unit/verify | revert, verified |
| positive | [`TestLLGROmittedGRFamilyTimerSurvivesConventionalEntry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L111) | unit/verify | revert, verified |

### [`RFC9494-4.2-3`](#rfc9494-4.2-3)

If the timer for the Long-Lived Stale Time for a given AFI/SAFI expires before the session is re-established, the helper MUST delete all stale routes of that AFI/SAFI from the neighbor that it is retaining. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent re-audit after 26d70dfa0d. onLLGRFamilyExpired and onLLGRComplete now send purge-stale and release-routes through dispatchRetained to every connection the retained cycle holds, earlier ones first; for a single restart that is the source alone, so TestLLSTTimerExpiry_SingleFamily, TestRFC9494LLSTExpiryDeletesTheFamilysStaleRoutes (exact IPv4 purge, no IPv6 purge) and TestRFC9494DelayedDownPreservesLLSTDeadlines (both polarities, real RIB, 1ns either side of each family's deadline) keep their full force. The commit adds both polarities on TestRFC9494ConsecutiveRestartExpiryDeletesEveryConnectionsRoutes: after a second restart during LLGR, the real RIB holds all three routes of connections 41 and 42 at level 2 with LLGR_STALE one nanosecond before the original deadline and none at it, so reverting dispatchRetained to the latest connection leaves 41's route and goes red. The RIB side (earlierRetainedLocked, retainsEarlier keeping an emptied earlier connection known) answers a zero purge rather than refusing. The whole sentence, every stale route of the family from the neighbor, is covered for both the single and the consecutive case.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9494ConsecutiveRestartExpiryDeletesEveryConnectionsRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_consecutive_rib_test.go#L169) | unit/verify | revert, verified |
| negative | [`TestRFC9494DelayedDownPreservesLLSTDeadlines`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L178) | unit/verify | revert, verified |
| positive | [`TestLLSTTimerExpiry_SingleFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L664) | unit/verify | not verified |
| positive | [`TestRFC9494LLSTExpiryDeletesTheFamilysStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L207) | unit/verify | not verified |
| positive | [`TestRFC9494ConsecutiveRestartExpiryDeletesEveryConnectionsRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_consecutive_rib_test.go#L168) | unit/verify | revert, verified |
| positive | [`TestRFC9494DelayedDownPreservesLLSTDeadlines`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L177) | unit/verify | revert, verified |

### [`RFC9494-4.2-4`](#rfc9494-4.2-4)

* The helper router MUST attach the LLGR_STALE community to the stale routes being retained. (§4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), stale-unit: internal/component/bgp/plugins/gr/gr_consecutive.go::grPlugin.restalePreservedLLGR, internal/component/bgp/plugins/gr/gr_state.go::grStateManager.llgrFamilyActive moved. Independent re-audit after 26d70dfa0d. The tagged roots still prove attachment on the normal paths: the zero-GR real-RIB root requires literal 0xffff0006 at level 2, TestAttachCommunity the pooled bytes, the fresh-route negative no attachment to fresh routes, and the new TestRFC9494ConsecutiveRestartMarksTheLaterConnectionsRoutes requires LLGR_STALE at level 2 on the routes connection 42 delivered for a family already in LLGR (restalePreservedLLGR). FINDING, single restart: the same commit made llgrFamilyActive return false once reestablishedConnectionID is set, and wireStateCallbacks' onLLGREnter checks it before NO_LLGR removal, before attach-community and before the level-2 mark. When the successor's UP lands while a Restart-Time-expiry LLGR entry is dispatching, the LLGR period has begun (its LLST timer keeps running through synchronization) but the retained stale routes keep level 1 and no LLGR_STALE until End-of-RIB, LLST expiry or a later DOWN; nothing completes the entry on reconnect (only restalePreservedLLGR at the next DOWN), and the RIB safety timer armed by the DOWN mark-stale stays armed. Section 4.2 says these procedures MUST be performed once the Restart Time period ends. The untagged TestRFC9494ConsecutiveRestartCompletesARevokedLLGREntry asserts that undecorated state at lines 276-278 as expected, and no tagged root can fail on it. Before this commit llgrFamilyActive kept true after reconnect, so the entry completed. Suspected code defect, not verified by a run.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9494ReconnectDuringLLGREntryLeavesTheNewConnectionsRouteFresh`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_reconnect_entry_test.go#L152) | unit/verify | revert, verified |
| negative | [`TestRFC9494_FreshRoutesDoNotGetLLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L34) | unit/verify | revert, verified |
| positive | [`TestRFC9494ConsecutiveRestartMarksTheLaterConnectionsRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_consecutive_rib_test.go#L148) | unit/verify | revert, verified |
| positive | [`TestRFC9494ZeroRestartTimeRetainsThroughTheLLGRPeriod`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L68) | unit/verify | revert, verified |
| positive | [`TestRFC9494ReconnectDuringLLGREntryStillAttachesLLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_reconnect_entry_test.go#L109) | unit/verify | revert, verified |
| positive | [`TestAttachCommunity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_rib_gr_test.go#L621) | unit/verify | not verified |

### [`RFC9494-4.2-5`](#rfc9494-4.2-5)

* If any of the routes from the peer have been marked with the NO_LLGR community, either as sent by the peer or as the result of a configured policy, they MUST NOT be retained and MUST be removed as per the normal operation of [RFC4271]. (§4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), stale-unit: internal/component/bgp/plugins/gr/gr_consecutive.go::grPlugin.restalePreservedLLGR, internal/component/bgp/plugins/gr/gr_state.go::grStateManager.llgrFamilyActive, internal/component/bgp/plugins/rib/rib_source_mutation.go::removePhysicalSource moved. Independent re-audit after 26d70dfa0d. On the normal paths the tags still hold: the zero-GR real-RIB root requires the received NO_LLGR route absent while its sibling stays at level 2 with LLGR_STALE; TestDeleteWithCommunity and the LLGR_STALE-only negative pin exact community matching; the best-path pair requires the Withdraw; llgr-import-no-llgr.ci covers the policy-marked case on a real daemon. removeNoLLGR now wraps the same delete-with-community command, and restalePreservedLLGR repeats it on every retained connection at a consecutive DOWN; removePhysicalSource only gains forgetEarlier on a complete release. FINDING, single restart: llgrFamilyActive now returns false once a successor re-established, and onLLGREnter checks it before step 1. If the successor's UP is handled after the Restart Time expired but before the entry callback dispatches NO_LLGR removal, the LLGR period has begun yet NO_LLGR routes stay retained as stale through synchronization, until End-of-RIB, LLST expiry or a later DOWN. Section 4.2 says they MUST NOT be retained once that period begins. No tagged root can fail on this; before the commit the guard read only the family's timer and the removal ran. Suspected code defect, not verified by a run.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9494SweepKeepsTheBestPathWithoutNoLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_no_llgr_withdraw_test.go#L100) | unit/verify | revert, verified |
| negative | [`TestRFC9494_StaleRouteWithoutNoLLGRRetained`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L63) | unit/verify | not verified |
| negative | [`llgr-import-no-llgr.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/llgr-import-no-llgr.ci#L7) | functional/verify | revert, verified |
| positive | [`TestRFC9494ZeroRestartTimeRetainsThroughTheLLGRPeriod`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L69) | unit/verify | revert, verified |
| positive | [`TestRFC9494ReconnectDuringLLGREntryStillRemovesNoLLGRRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_reconnect_entry_test.go#L132) | unit/verify | revert, verified |
| positive | [`TestRFC9494NoLLGRSweepWithdrawsTheBestPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_no_llgr_withdraw_test.go#L80) | unit/verify | revert, verified |
| positive | [`TestDeleteWithCommunity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_rib_gr_test.go#L662) | unit/verify | not verified |
| positive | [`llgr-import-no-llgr.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/llgr-import-no-llgr.ci#L6) | functional/verify | revert, verified |

### [`RFC9494-4.2-6`](#rfc9494-4.2-6)

* The helper router MUST perform the procedures listed in Section 4.3. (§4.2, §4.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Independent producer-change maintenance rejudgment (2026-11-22): preserve WEAK with no upgrade. Inspected bestpath.go::compareBeforeMED against HEAD: only the RFC 7611 Section 3 post-LOCAL_PREF compareAcceptOwn(a, b) call was inserted after Step 1 (LOCAL_PREF); Step 0 (stale-level depreference at storage.DepreferenceThreshold) is byte-identical and executes first, so LLGR stale depreference is completely unaffected. Both tagged tests in internal/component/bgp/plugins/rib/rfc9494_test.go are unchanged and still prove only depreference (attach-community raises StaleLevel to DepreferenceThreshold, marked LOCAL_PREF 500 loses to fresh LOCAL_PREF 100 in ComparePair/SelectBest, and an unmarked fresh candidate competes normally); they still do not assert all Section 4.3 received/self-attachment triggers, advertisement restrictions, and LLGR_STALE preservation on further advertisement. No machine discrimination record exists for these two tags, and no full-row credit is claimed.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9494_RouteWithoutLLGRStaleNotLeastPreferred`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L175) | unit/verify | revert, verified |
| positive | [`TestRFC9494_LLGRStaleRouteIsLeastPreferred`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L137) | unit/verify | revert, verified |

### [`RFC9494-4.2-7`](#rfc9494-4.2-7)

However, in the case of consecutive restarts, the previously marked stale routes MUST NOT be deleted before the timer for the Long-Lived Stale Time expires. (§4.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9494ConsecutiveRestartPreservesLLGRStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_consecutive_test.go#L36) | unit/verify | revert, verified |
| positive | [`TestRFC9494ConsecutiveRestartKeepsTheEarlierConnectionsStaleRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_consecutive_rib_test.go#L125) | unit/verify | revert, verified |
| positive | [`TestRFC9494ConsecutiveRestartPreservesLLGRStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_consecutive_test.go#L32) | unit/verify | revert, verified |

### [`RFC9494-4.2-8`](#rfc9494-4.2-8)

Similar to [RFC4724], once the LLGR Period begins, the Helper MUST immediately remove all the stale routes from the peer that it is retaining for that address family if any of the following occur: * the F bit for a specific address family is not set in the newly received LLGR Capability, or * a specific address family is not included in the newly received LLGR Capability, or * the LLGR and accompanying GR Capability are not received in the re-established session at all. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent re-audit against RFC 9494 Section 4.2 and current gr.go/gr_state.go (post-18b491ae23). Section 4.2 requires immediate removal of retained stale routes for an LLGR-period family on re-establishment when the new LLGR capability clears F, omits the family, or neither GR nor LLGR is received. Commit 18b491ae23 changed onSessionReestablished so surviving families keep their running LLST timers until EoR (as Section 4.2 requires), while families failing reconnectKeepsFamily (or all families when newCap==nil) still have their LLST timers stopped and are returned in purged for immediate purge-stale dispatch in handleStructuredState. All 7 tagged units verified: positives TestOnSessionReestablished_LLGRFBitClearPurges, TestOnSessionReestablished_FamilyAbsentFromLLGRPurges, TestOnSessionReestablished_NoLLGRCapPurgesLLGRPeriodFamily, and TestOnSessionReestablished_DuringLLGR_NoCaps cover every bullet; negatives TestOnSessionReestablished_DuringLLGR, TestOnSessionReestablished_BothFBitsSetKeepsLLGRPeriodFamily, and TestOnSessionReestablished_GRPeriodFamilyIgnoresLLGRBullets verify keeping both-F-set LLGR families and confining LLGR bullets to families whose LLGR period has begun. Note: TestOnSessionReestablished_DuringLLGR carries a stale doc comment claiming reconnect stops LLST timers, though its assertion only checks purged is empty.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestOnSessionReestablished_DuringLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L798) | unit/verify | revert, verified |
| negative | [`TestOnSessionReestablished_BothFBitsSetKeepsLLGRPeriodFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_reconnect_test.go#L133) | unit/verify | revert, verified |
| negative | [`TestOnSessionReestablished_GRPeriodFamilyIgnoresLLGRBullets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_reconnect_test.go#L156) | unit/verify | revert, verified |
| positive | [`TestOnSessionReestablished_DuringLLGR_NoCaps`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L849) | unit/verify | revert, verified |
| positive | [`TestOnSessionReestablished_FamilyAbsentFromLLGRPurges`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_reconnect_test.go#L65) | unit/verify | revert, verified |
| positive | [`TestOnSessionReestablished_LLGRFBitClearPurges`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_reconnect_test.go#L40) | unit/verify | revert, verified |
| positive | [`TestOnSessionReestablished_NoLLGRCapPurgesLLGRPeriodFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_reconnect_test.go#L93) | unit/verify | revert, verified |

### [`RFC9494-4.2-9`](#rfc9494-4.2-9)

If a Long-Lived Stale Time timer is running for routes with a given AFI/SAFI received from a peer, it MUST NOT be updated (other than by manual operator intervention) until the peer has established and synchronized a new session. (§4.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9494LLSTTimerNotUpdatedUntilSynchronized`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_consecutive_test.go#L160) | unit/verify | revert, verified |
| positive | [`TestRFC9494LLSTTimerNotUpdatedUntilSynchronized`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_consecutive_test.go#L157) | unit/verify | revert, verified |

### [`RFC9494-4.3-1`](#rfc9494-4.3-1)

A BGP speaker that has advertised the Long-Lived Graceful Restart Capability to a neighbor MUST perform the following upon receiving a route from that neighbor with the LLGR_STALE community or upon attaching the LLGR_STALE community itself per Section 4.2: * Treat the route as the least preferred in route selection (see below). (§4.3, §4.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent re-audit against RFC 9494 Sections 4.3-4.4 after updating LLGR config syntax in test/plugin/llgr-stale-received-depreferenced.ci (line 51: short-lived { restart-time 120; } long-lived { family { ipv4/unicast 3600; } }) and gr_filter.go llgrConfig path (session > capability > graceful-restart > long-lived > family). Verified insertForPeer/resolvedPath (filter_insertion.go) matches llgrConfig on peer llgr (inserting bgp-gr:llgr-ingress) and does not match peer plain (graceful-restart disable), while evaluateLLGRIngress and llgrStaleFromSource are byte-identical and all other 14 tagged Go test units and all 9 cited producer functions are unchanged by this GR/LLGR config refactor. The .ci test's BGP UPDATE wire hex, peer topology, and exact stderr assertions (10.0.0.0/8 best ::1 by stale-level positive; 10.1.0.0/16 best ::1 by local-preference negative) are unchanged. Bounded scope and prior limitations remain in full (no tests or discrimination replays were run by this auditor; test/plugin/llgr-stale-received-depreferenced.ci unit-sha and any prior producer-sha entries in rfc/discrimination/rfc9494/RFC9494-4.3-1.json require renewal by main before ./le rfc check).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestLLGRIngressFilter_NoLLGRSentLeavesFresh`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_ingress_test.go#L91) | unit/verify | revert, verified |
| negative | [`TestComparePair_GRStaleCompetesNormally`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L856) | unit/verify | revert, verified |
| negative | [`TestRFC9494ReceivedLLGRStaleFromNonLLGRNeighborCompetesNormally`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_received_stale_test.go#L198) | unit/verify | revert, verified |
| negative | [`TestRFC9494ReceivedOtherCommunityCompetesNormally`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_received_stale_test.go#L146) | unit/verify | revert, verified |
| negative | [`TestRFC9494SelfAttachSkipsRouteLearnedAfterLoss`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_self_attach_test.go#L114) | unit/verify | revert, verified |
| negative | [`TestRFC9494ReceivedEventNoLLGRSessionStaysFresh`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9494_llgr_ingress_meta_test.go#L117) | unit/verify | revert, verified |
| negative | [`TestRFC9494NegotiateWithKeysOnAdvertisedLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9494_llgr_trigger_test.go#L48) | unit/verify | revert, verified |
| negative | [`llgr-stale-received-depreferenced.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/llgr-stale-received-depreferenced.ci#L11) | functional/verify | revert, verified |
| positive | [`TestLLGRIngressFilter_MarksReceivedStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_ingress_test.go#L72) | unit/verify | revert, verified |
| positive | [`TestComparePair_LLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L820) | unit/verify | revert, verified |
| positive | [`TestRFC9494LLGRStaleLosesEveryLaterTieBreak`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_least_preferred_test.go#L37) | unit/verify | revert, verified |
| positive | [`TestRFC9494ReceivedLLGRStaleIsLeastPreferred`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_received_stale_test.go#L125) | unit/verify | revert, verified |
| positive | [`TestRFC9494SelfAttachedLLGRStaleIsLeastPreferred`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_self_attach_test.go#L95) | unit/verify | revert, verified |
| positive | [`TestRFC9494ReceivedEventCarriesIngressStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9494_llgr_ingress_meta_test.go#L101) | unit/verify | revert, verified |
| positive | [`TestRFC9494NegotiateWithKeysOnAdvertisedLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9494_llgr_trigger_test.go#L46) | unit/verify | revert, verified |
| positive | [`llgr-stale-received-depreferenced.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/llgr-stale-received-depreferenced.ci#L10) | functional/verify | revert, verified |

### [`RFC9494-4.3-2`](#rfc9494-4.3-2)

The LLGR_STALE community MUST NOT be removed when the route is further advertised (§4.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9494LLGRStaleNotRemovedOnEgress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_community/rfc9494_llgr_stale_preserve_test.go#L30) | unit/verify | revert, verified |
| positive | [`TestRFC9494LLGRStaleNotRemovedOnEgress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_community/rfc9494_llgr_stale_preserve_test.go#L26) | unit/verify | revert, verified |

### [`RFC9494-4.4-1`](#rfc9494-4.4-1)

A least preferred route MUST be treated as less preferred than any other route that is not also least preferred. (§4.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a least-preferred route beating one that is not. TestSelectBest_LLGRStaleDepreference gives the stale candidate LOCAL_PREF 300 against 100 and fails, in both input orders, unless the normal route wins, so removing Step 0 goes red. Negative: TestSelectBest_BothLLGRStale fails unless two stale candidates fall through to LOCAL_PREF, so the rule orders only across the least-preferred boundary.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestSelectBest_BothLLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L783) | unit/verify | revert, verified |
| positive | [`TestSelectBest_LLGRStaleDepreference`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L755) | unit/verify | revert, verified |

### [`RFC9494-4.5-1`](#rfc9494-4.5-1)

If the LLGR Capability is received without an accompanying GR Capability, the LLGR Capability MUST be ignored, that is, the implementation MUST behave as though no LLGR Capability has been received. (§4.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent current-source rejudgment, not authored by this judge. Read the complete normative Sections4.1-4.6 and5 context, current requirement/derived ledger, every whole tagged question and actual helpers/producers for this row. Preserve the existing verdict and scope, without claiming whole-RFC conformance. Section4.5 requires behaving as though LLGR was absent when GR is absent. Both whole JSON OPEN tests read. A valid code71-only OPEN leaves no GR or LLGR map entry; the valid code64+71 control keeps actual decoded family/F-bit/LLST3600 data and RestartTime120. The event carries owner701/connection41, and handleOpenEvent ends in forgetGRCapability when code64 was not found, so its removal is conditional on missing GR, not universal capability suppression. No new machine record exists for these tags; this is current source assertion judgment, not a new runtime observation.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestHandleEventOpenLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_event_test.go#L266) | unit/verify | revert, verified |
| positive | [`TestHandleEventOpenLLGR_NoGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_event_test.go#L301) | unit/verify | revert, verified |

### [`RFC9494-4.6-1`](#rfc9494-4.6-1)

The neighbors MUST be internal (Internal BGP (IBGP) or Confederation) neighbors. (§4.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent current-source rejudgment, not authored by this judge. Read the complete normative Sections4.1-4.6 and5 context, current requirement/derived ledger, every whole tagged question and actual helpers/producers for this row. Preserve the existing verdict and scope, without claiming whole-RFC conformance. Section4.6 permits partial delivery only to an internal neighbor. Both whole tests read. The external non-LLGR input requires mods.IsWithdraw; the internal non-LLGR input requires delivered, not withdrawn output modifications including exact NO_EXPORT0xffffff01 and LOCAL_PREF0. Current production uses dest.IsIBGP, the captured session classification, rather than comparing AS numbers; the migrated internal fixture explicitly sets it. Existing owner/physical-connection capability lookup prevents another destination capability from granting this branch. No new machine record or actual socket-wire observation exists for these filter-unit tags.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestLLGREgressFilter_EBGPNonLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L399) | unit/verify | revert, verified |
| positive | [`TestLLGREgressFilter_IBGPPartial`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L429) | unit/verify | revert, verified |

### [`RFC9494-4.6-2`](#rfc9494-4.6-2)

* The NO_EXPORT community [RFC1997] MUST be attached to the stale routes. (§4.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent current-source rejudgment, not authored by this judge. Read the complete normative Sections4.1-4.6 and5 context, current requirement/derived ledger, every whole tagged question and actual helpers/producers for this row. Preserve the existing verdict and scope, without claiming whole-RFC conformance. Section4.6 requires NO_EXPORT on the partial-deployment stale delivery. All3whole roots read. Loaded-state and unloaded-state internal non-LLGR inputs require a COMMUNITIES Add operation with exactly four bytes0xffffff01; presence is required independently of the loop, preventing an empty operations list from passing. The LLGR-capable peer is a valid contrast with mods.Len0. Current LLGREgressFilter selects partial delivery by captured IsIBGP and applies exact communityNoExport bytes. These are exact filter-operation assertions, not a newly observed destination wire encoding or machine mutant record.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestLLGREgressFilter_LLGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L369) | unit/verify | revert, verified |
| positive | [`TestLLGREgressFilter_IBGPPartial`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L433) | unit/verify | not verified |
| positive | [`TestLLGREgressFilter_NilStateDepreferencesIBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L182) | unit/verify | revert, verified |

### [`RFC9494-4.6-3`](#rfc9494-4.6-3)

* The stale routes MUST have their LOCAL_PREF set to zero. (§4.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent current-source rejudgment, not authored by this judge. Read the complete normative Sections4.1-4.6 and5 context, current requirement/derived ledger, every whole tagged question and actual helpers/producers for this row. Preserve the existing verdict and scope, without claiming whole-RFC conformance. Section4.6 requires LOCAL_PREF0 on partial-deployment stale delivery. All3whole roots read. Loaded-state and unloaded-state internal non-LLGR inputs require a LOCAL_PREF Set operation carrying exactly four zero bytes, plus explicit presence flags. The LLGR-capable peer must receive no modifications. Source owner/connection55 fixtures match their capability snapshots, and LLGREgressFilter uses captured IsIBGP for the internal exception. No new machine record or complete socket-level partial-deployment experiment is claimed.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestLLGREgressFilter_LLGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L373) | unit/verify | revert, verified |
| positive | [`TestLLGREgressFilter_IBGPPartial`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L436) | unit/verify | not verified |
| positive | [`TestLLGREgressFilter_NilStateDepreferencesIBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L186) | unit/verify | revert, verified |

### [`RFC9494-4.7.2-1`](#rfc9494-4.7.2-1)

In addition to including the path attributes derived from the ATTR_SET attribute in the advertised route as per [RFC6368], the PE router MUST also include the LLGR_STALE community if it is present in the path attributes of the imported route, even if it is not present in the ATTR_SET attribute. (§4.7.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9494-4.7.2-1, so there is no linked test.

### [`RFC9494-4.7.2-2`](#rfc9494-4.7.2-2)

In addition to including in the VPN route the ATTR_SET derived from the path attributes as per [RFC6368], the PE router MUST also include the LLGR_STALE community in the VPN route if it is present in the path attributes of the route as received from the CE. (§4.7.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9494-4.7.2-2, so there is no linked test.

### [`RFC9494-5-1`](#rfc9494-5-1)

Implementations MUST NOT enable these procedures by default. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent re-audit against RFC 9494 Section 5 ('Implementations MUST NOT enable these procedures by default') after adding diagnostic field 'code', codeDoctorBGPGRCapability to the six logger().Warn calls inside parseLLGRCapValue (commit 3c2a055fd9). The change is logging-only with zero effect on control flow, return values, or wire encoding: absent graceful-restart or long-lived containers still return '', llgrFamilyMax = 36 and !nlrisplit.Supported(f) remain strictly restrictive, and all 4 tagged units are unchanged (TestRFC9494_LLGRNotEnabledByDefault, TestRFC9494_LLGREnabledByExplicitConfig, TestRFC9494HelperProceduresOffWithoutLocalConfig, TestRFC9494HelperProceduresOnPerConfiguredFamily). Note: the two parseLLGRCapValue revert records in rfc/discrimination/rfc9494/RFC9494-5-1.json still carry prior producer-sha 317009a6c8051332 from before the warn-code edit and need renewal by main before ./le rfc check; whole-body panics prove producer dependency only, with semantic scope supplied by healthy assertions and source inspection.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9494HelperProceduresOffWithoutLocalConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_helper_default_red_test.go#L81) | unit/verify | revert, verified |
| negative | [`TestRFC9494_LLGREnabledByExplicitConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L144) | unit/verify | revert, verified |
| positive | [`TestRFC9494HelperProceduresOnPerConfiguredFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_helper_default_red_test.go#L117) | unit/verify | revert, verified |
| positive | [`TestRFC9494_LLGRNotEnabledByDefault`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L119) | unit/verify | revert, verified |

### [`RFC9494-5-2`](#rfc9494-5-2)

They MUST require affirmative configuration per AFI/SAFI in order to enable them. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent re-audit against RFC 9494 Section 5 ('They MUST require affirmative configuration per AFI/SAFI in order to enable them') after adding diagnostic field 'code', codeDoctorBGPGRCapability to the six logger().Warn calls inside parseLLGRCapValue (commit 3c2a055fd9). The change is logging-only with zero effect on per-AFI/SAFI filtering, 24-bit LLST encoding, or helper gating. Both tagged units in rfc9494_llgr_per_family_test.go are unchanged: positive TestRFC9494_PerFamilyLLGRConfigPositive drives LoadConfig -> ToPluginMap -> ExtractConfigSubtree -> refuseUncarriedGRFamilies -> extractLLGRCapabilities, proving dual-family single-entry exclusion of ipv6/unicast and exact boundary LLST hex 0001018000000000020180ffffff (0 and 16777215); negative TestRFC9494_PerFamilyLLGRConfigNegative proves short-lived-only and empty long-lived { } emit no code 71, and unconfigured ipv6/unicast is omitted from code 71 and purged (no ffff0006) on structured helper session drop even when remote advertises both families. Note: the two parseLLGRCapValue revert records in rfc/discrimination/rfc9494/RFC9494-5-2.json still carry prior producer-sha 317009a6c8051332 from before the warn-code edit and need renewal by main before ./le rfc check (exchangedLLGRLocked discrimination is recorded under RFC9494-5-1). Scope boundary: config-to-declaration and structured helper event path; reactor OPEN assembly and JSON event path are not asserted in these two units.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9494_PerFamilyLLGRConfigNegative`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_per_family_test.go#L100) | unit/verify | revert, verified |
| positive | [`TestRFC9494_PerFamilyLLGRConfigPositive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_per_family_test.go#L48) | unit/verify | revert, verified |

### [`RFC9494-4.2-10`](#rfc9494-4.2-10)

If the session subsequently resets prior to becoming synchronized, any remaining routes (for the AFI/SAFI whose LLST timer expired) MUST be removed immediately. (§4.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9494ResetAfterSyncLLSTExpiryRemovesImmediately`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_consecutive_test.go#L262) | unit/verify | revert, verified |
| positive | [`TestRFC9494ResetAfterSyncLLSTExpiryRemovesImmediately`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_consecutive_test.go#L259) | unit/verify | revert, verified |

### [`RFC9494-4.3-3`](#rfc9494-4.3-3)

The route SHOULD NOT be advertised to any neighbor from which the Long-Lived Graceful Restart Capability has not been received. The exception is described in Section 4.6. (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/reactor/forward_rs.go::reactorForwardRSSectionResult, internal/component/bgp/reactor/reactor_api_relay.go::buildRelayUpdate, internal/component/bgp/reactor/reactor_notify.go::Reactor.notifyMessageReceiverFrom, internal/component/bgp/reactor/reactor_notify.go::Reactor.notifyOneMessage moved. Independent final source rejudgment, 2026-11-19. Read RFC 9494 Sections 4.3 and 4.6, the current requirement and old verdict, all seventeen tagged bindings in eleven distinct roots, their fixture boundaries, the current producing boundaries, sixteen discrimination records, and the renewed Main receipts. Verdict remains enforced for this requirement, with the existing SHOULD NOT strength and Section 4.6 exception preserved. This is not a whole-RFC support claim. Section 4.3 requires that the route SHOULD NOT be advertised to a neighbor from which LLGR has not been received and explains that this implies withdrawal; Section 4.6 permits advertisement to internal neighbors only with NO_EXPORT and LOCAL_PREF zero.

The direct GR roots assert the actual withdrawal decision toward an incapable external destination under both nil state and a nonempty loaded capability map. The capable-destination contrasts require no withdrawal and no modification; the conventional-GR level-one contrast likewise requires unchanged advertisement while level two is withdrawn. Runtime capabilities are keyed by original destination owner and connection. The rebuilt-route root requires the rebuilt key, source LLGR offer and LLGR_STALE wire community together, checks withdrawal versus a capable destination, and removes each of the three prerequisites separately with exact unchanged-route controls. These are outcome assertions, not merely handled/no-panic checks.

The two RIB roots feed the actual structured received handler with an original owner, connection and receipt, then create a destination sent copy. They assert the source's received level two survives a copy at level zero or one through grouped peer-up, resend and refresh, and reaches exactly one dispatched route command as stale=2. The fresh contrasts require level zero and no stale key. Refresh after a new source MessageID still requires level two even though the older sent revision is no longer current. The four meaningful current mutants in job-advertiser-llgr-replay-and-reactor-proofs-82710851.log have healthy baselines and intended assertion failures: forcing fresh grouped replay to level two fails the fresh control; omitting received stale from grouped replay fails the stale control; forcing fresh refresh stale fails the fresh refresh assertion; copying only the sent retention mark fails stale refresh and the new-MessageID assertion. These show the named source-level transport obligations, not a new end-to-end daemon grant proof.

The cached and RS reactor roots now capture a real original Session receipt after publishing generation and encoding, enter through its source callback, require retention and exact dispatched owner/connection/receipt, then exercise the native cache/general or RS forwarding rail. Their ingress and external export members are explicit stand-ins for GR. They require the export probe to see exactly level two and the forwarding pool to receive only the exact prefix withdrawal, versus no stale field and exactly one announcement for the source-no-offer control. They do not negotiate a real OPEN here and do not observe a final recipient socket. The rebuilt relay root separately asserts external export fields rebuilt=true in both cases, source-llgr-advertised exactly true/false and the source address. Its operator relay source lookup is not a broader learned-source ownership proof. Current notifyMessageReceiverFrom/notifyOneMessage preserve route metadata through continuation, received event and cache; source receipt bookkeeping supplements it.

The promoted llgr-egress-state-unloaded.ci retains the original source UPDATE, configured peers, unchanged next hop, capability contrast and first-prefix/second-NO_EXPORT wire oracle. Its observer installs a normalized static terminal /32 and waits for the real selected, resolved nonrecursive route before the best-route wait. This repairs reachability of the original useful route; it does not alter source bytes or skip selection. The destination is internal, has no recorded LLGR capability, and receives the Section 4.6 exception. The meaningful quiet cut in job-advertiser-promoted-llgr-egress-cut-and-record-f497f32b.log passes clean at line 8, fails at line 18, and at line 73 reports the missing NO_EXPORT on a useful second UPDATE that still carries the same prefix, next hop and LOCAL_PREF zero. It therefore distinguishes unchanged or incompletely modified advertisement from the required wire treatment. Its assertion is the NO_EXPORT substring, not a decoded LOCAL_PREF-zero assertion; the latter is not newly claimed for this .ci carrier.

Keep the discrimination evidence classes separate. The five current GR-local body-panic retakes in job-advertiser-llgr-local-record-renewal-b4406954.log and five refreshed reactor body-panic retakes in 82710851 have named healthy runs and named broken runs, but establish producer reach only. In particular negative-labelled multi-case records can stop at the first advertised case, so they are not credited as independently observed negative-leaf semantic mutants. The rebuilt negative resolveRelaySource record remains its existing source-reach witness. The later promoted daemon body-panic replay in f497f32b has clean PASS at line 175, broken FAIL at line 190 and native record at line 309; its actual failure is the observer reporting that readvertisement never arrived. That historical halt record is not the earlier quiet missing-NO_EXPORT proof. The seventeen original bindings, compositional source assertions and their stated limits remain; no polarity, annotation, RFC scope or conformance count is upgraded.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestLLGREgressFilter_GRStaleLevelOnePassesThrough`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_rails_test.go#L73) | unit/verify | revert, verified |
| negative | [`TestLLGREgressFilter_RebuiltRecoversReceivedStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_rails_test.go#L110) | unit/verify | revert, verified |
| negative | [`TestLLGREgressFilter_StateLoadedStillAdvertisesToLLGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L340) | unit/verify | not verified |
| negative | [`TestPeerUpReplayReportsSourceIngressLevel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_replay_stale_test.go#L135) | unit/verify | mutant, verified |
| negative | [`TestRIBReplayMetaReadsSourceIngressLevel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_replay_stale_test.go#L182) | unit/verify | mutant, verified |
| negative | [`TestRFC9494CachedForwardWithholdsReceivedStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9494_llgr_cached_forward_test.go#L277) | unit/verify | revert, verified |
| negative | [`TestRFC9494RouteServerForwardWithholdsReceivedStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9494_llgr_cached_forward_test.go#L310) | unit/verify | revert, verified |
| negative | [`TestBuildRelayUpdateMarksRebuilt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9494_relay_rebuilt_test.go#L73) | unit/verify | revert, verified |
| positive | [`TestLLGREgressFilter_RebuiltRecoversReceivedStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_rails_test.go#L106) | unit/verify | revert, verified |
| positive | [`TestLLGREgressFilter_StateLoadedWithdrawsEBGPNonLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_rails_test.go#L46) | unit/verify | revert, verified |
| positive | [`TestLLGREgressFilter_NilStateWithdrawsEBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L156) | unit/verify | revert, verified |
| positive | [`TestPeerUpReplayReportsSourceIngressLevel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_replay_stale_test.go#L131) | unit/verify | mutant, verified |
| positive | [`TestRIBReplayMetaReadsSourceIngressLevel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_replay_stale_test.go#L177) | unit/verify | mutant, verified |
| positive | [`TestRFC9494CachedForwardWithholdsReceivedStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9494_llgr_cached_forward_test.go#L275) | unit/verify | revert, verified |
| positive | [`TestRFC9494RouteServerForwardWithholdsReceivedStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9494_llgr_cached_forward_test.go#L308) | unit/verify | revert, verified |
| positive | [`TestBuildRelayUpdateMarksRebuilt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9494_relay_rebuilt_test.go#L69) | unit/verify | revert, verified |
| positive | [`llgr-egress-state-unloaded.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/llgr-egress-state-unloaded.ci#L23) | functional/verify | revert, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc9494.txt |
| Source fingerprint | c6e4d6c4836c4b4e |
| Record | rfc/extraction/rfc9494.json |
| Mapped sentences | 25 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `2.1` | not stated | 0 | reviewed | not stated |
| `2.2` | not stated | 0 | reviewed | not stated |
| `2.3` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `3.1` | not stated | 2 | reviewed | not stated |
| `3.2` | not stated | 0 | reviewed | not stated |
| `3.3` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 1 | reviewed | not stated |
| `4.1` | not stated | 2 | reviewed | not stated |
| `4.2` | not stated | 12 | reviewed | not stated |
| `4.3` | not stated | 2 | reviewed | not stated |
| `4.4` | not stated | 1 | reviewed | not stated |
| `4.5` | not stated | 1 | reviewed | not stated |
| `4.6` | not stated | 3 | reviewed | not stated |
| `4.7` | not stated | 0 | reviewed | not stated |
| `4.7.1` | not stated | 0 | reviewed | not stated |
| `4.7.2` | not stated | 2 | reviewed | not stated |
| `5` | not stated | 2 | reviewed | not stated |
| `5.1` | not stated | 0 | reviewed | not stated |
| `5.2` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `9.1` | not stated | 0 | reviewed | not stated |
| `9.2` | not stated | 0 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `4.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.1 restates the Section 3.1 obligation verbatim; site 3.1:1 carries it and cites Section 4.1 itself ("see Section 4.1"). | If the LLGR Capability is advertised, the Graceful Restart capability MUST also be advertised. |
| `4.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A quotation of superseded RFC 4724 text, introduced by "The following text in Section 4.2 of [RFC4724] no longer applies" and followed by "and the following procedures are specified instead". The block is set off with change bars and states no obligation of this document. | \| If the session does not get re-established within the "Restart \| Time" that the peer advertised previously, the Receiving Speaker \| MUST delete all the stale routes from the peer that it is \| retaining. |
| `4.2:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The chapeau introducing the LLGR-period bullet list. Each enumerated procedure carries its own MUST and is mapped separately (sites 4.2:4 through 4.2:8); the chapeau's own content, that the procedures begin once the Restart Time period ends, is carried in RFC9494-4.2-2. | Once the Restart Time period ends (including the case in which the Restart Time is zero), the LLGR period is said to have begun and the following procedures MUST be performed: |

## Superseded

No document obsoletes RFC 9494, so its obligations are stated where they were written.
