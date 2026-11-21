# RFC 9494 - Long-Lived Graceful Restart for BGP

Long-Lived Graceful Restart. Every requirement listed for RFC 9494, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 18 | of 25 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 25 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 25 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 25 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 65 | 65 of 98 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 7 | of 25 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Audit verdicts | 19 | of 25 mandatory requirements reviewed | 1 weak, wrong or unimplemented, 2 no longer current. Each is named below under its own requirement id |

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
| Declared gaps | 7 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 98 |
| Tests linked to requirements | 98 |
| Recorded audit verdicts | 19 |
| Fault test records | 65 |
| Summary | `rfc/short/rfc9494.md` |
| Requirement table | `rfc/requirements/rfc9494.md` |
| RFC text | `rfc/full/rfc9494.txt` |

## Automatic checks

Included in automatic checks: Long-Lived Graceful Restart for BGP

## Published support status

**Area:** Long-Lived Graceful Restart

**Test verification:** 8 of 25 MUST proven

**Evidence:** functional 2, unit 6

**Unmet:** RFC9494-4.2-10, RFC9494-4.2-7, RFC9494-4.2-9, RFC9494-4.3-2, RFC9494-4.7.2-1, RFC9494-4.7.2-2, RFC9494-5-2


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 18 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 7 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **25** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (18):** [`RFC9494-3.1-1`](#rfc9494-3.1-1), [`RFC9494-4.1-1`](#rfc9494-4.1-1), [`RFC9494-3.1-2`](#rfc9494-3.1-2), [`RFC9494-4-1`](#rfc9494-4-1), [`RFC9494-4.2-1`](#rfc9494-4.2-1), [`RFC9494-4.2-2`](#rfc9494-4.2-2), [`RFC9494-4.2-3`](#rfc9494-4.2-3), [`RFC9494-4.2-4`](#rfc9494-4.2-4), [`RFC9494-4.2-5`](#rfc9494-4.2-5), [`RFC9494-4.2-6`](#rfc9494-4.2-6), [`RFC9494-4.2-8`](#rfc9494-4.2-8), [`RFC9494-4.3-1`](#rfc9494-4.3-1), [`RFC9494-4.4-1`](#rfc9494-4.4-1), [`RFC9494-4.5-1`](#rfc9494-4.5-1), [`RFC9494-4.6-1`](#rfc9494-4.6-1), [`RFC9494-4.6-2`](#rfc9494-4.6-2), [`RFC9494-4.6-3`](#rfc9494-4.6-3), [`RFC9494-5-1`](#rfc9494-5-1)

**Annotated (including scoped evidence) (7):** [`RFC9494-4.2-7`](#rfc9494-4.2-7), [`RFC9494-4.2-9`](#rfc9494-4.2-9), [`RFC9494-4.3-2`](#rfc9494-4.3-2), [`RFC9494-4.7.2-1`](#rfc9494-4.7.2-1), [`RFC9494-4.7.2-2`](#rfc9494-4.7.2-2), [`RFC9494-5-2`](#rfc9494-5-2), [`RFC9494-4.2-10`](#rfc9494-4.2-10)

## Figures

- **MUST bar**: Ze verified 8, Tests awaiting verification 10, Not built 7
- **Overview bar**: Ze MUST verified 8, SHOULD verified 1, Tests awaiting verification 10, Not assessed 10, Not built 7

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 25 |
| MUST rows kept (the MUST bar) | 25 |
| Ze verified | 8 |
| OS verified | 0 |
| Tests awaiting verification | 10 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 7 |
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
| `RFC9494-4-1` | If a BGP speaker is configured to support the procedures of this document, it MUST use BGP Capabilities Advertisement [RFC5492] to advertise the Long-Lived Graceful Restart Capability. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestExtractLLGRCapabilities_Basic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_test.go#L663). **negative:** `unit/verify` [`TestExtractLLGRCapabilities_NoLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_test.go#L690) |
| `RFC9494-4.2-1` | After the session goes down, and before the session is re- established, the stale routes for an AFI/SAFI MUST be retained. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestLLGROmittedGRFamilyReadvertisementDoesNotDelaySiblingExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L211). **positive:** `unit/verify` [`TestLLGROmittedGRFamilyRetainsRealRIB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L27). **positive:** `unit/verify` [`TestLLGROmittedGRFamilyTimerSurvivesConventionalEntry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L110). **positive:** `unit/verify` [`TestLLSTTimerExpiry_LastFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L708). **positive:** `unit/verify` [`TestRFC9494DelayedDownPreservesLLSTDeadlines`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L176). **positive:** `unit/verify` [`TestRFC9494RestartTimeThenLongLivedStaleTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L137). **positive:** `unit/verify` [`TestRFC9494ZeroRestartTimeRetainsThroughTheLLGRPeriod`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L67). **negative:** `unit/verify` [`TestLLGROmittedGRFamilyWithoutLLSTIsRemoved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L164). **negative:** `unit/verify` [`TestOnTimerExpired_WithoutLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L606). **negative:** `unit/verify` [`TestRFC9494BothTimesZeroRetainsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L101) |
| `RFC9494-4.2-2` | * For each AFI/SAFI for which it has received a nonzero Long-Lived Stale Time, the helper router MUST start a timer for that Long- Lived Stale Time. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestLLGROmittedGRFamilyReadvertisementDoesNotDelaySiblingExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L212). **positive:** `unit/verify` [`TestLLGROmittedGRFamilyRetainsRealRIB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L28). **positive:** `unit/verify` [`TestLLGROmittedGRFamilyTimerSurvivesConventionalEntry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L111). **positive:** `unit/verify` [`TestOnTimerExpired_WithLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L571). **positive:** `unit/verify` [`TestRFC9494RestartTimeThenLongLivedStaleTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L138). **negative:** `unit/verify` [`TestLLGROmittedGRFamilyWithoutLLSTIsRemoved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L165). **negative:** `unit/verify` [`TestOnSessionDown_ZeroGR_ZeroLLST`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L770). **negative:** `unit/verify` [`TestRFC9494BothTimesZeroRetainsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L102) |
| `RFC9494-4.2-3` | If the timer for the Long-Lived Stale Time for a given AFI/SAFI expires before the session is re-established, the helper MUST delete all stale routes of that AFI/SAFI from the neighbor that it is retaining. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestLLSTTimerExpiry_SingleFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L664). **positive:** `unit/verify` [`TestRFC9494DelayedDownPreservesLLSTDeadlines`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L177). **positive:** `unit/verify` [`TestRFC9494LLSTExpiryDeletesTheFamilysStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L207). **negative:** `unit/verify` [`TestRFC9494DelayedDownPreservesLLSTDeadlines`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L178) |
| `RFC9494-4.2-4` | * The helper router MUST attach the LLGR_STALE community to the stale routes being retained. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestAttachCommunity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_rib_gr_test.go#L621). **positive:** `unit/verify` [`TestRFC9494ZeroRestartTimeRetainsThroughTheLLGRPeriod`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L68). **negative:** `unit/verify` [`TestRFC9494_FreshRoutesDoNotGetLLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L34) |
| `RFC9494-4.2-5` | * If any of the routes from the peer have been marked with the NO_LLGR community, either as sent by the peer or as the result of a configured policy, they MUST NOT be retained and MUST be removed as per the normal operation of [RFC4271]. (§4.2) | MUST NOT | 4.2 | **positive:** `unit/verify` [`TestDeleteWithCommunity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_rib_gr_test.go#L662). **positive:** `unit/verify` [`TestRFC9494NoLLGRSweepWithdrawsTheBestPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_no_llgr_withdraw_test.go#L80). **positive:** `unit/verify` [`TestRFC9494ZeroRestartTimeRetainsThroughTheLLGRPeriod`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L69). **negative:** `unit/verify` [`TestRFC9494SweepKeepsTheBestPathWithoutNoLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_no_llgr_withdraw_test.go#L100). **negative:** `unit/verify` [`TestRFC9494_StaleRouteWithoutNoLLGRRetained`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L63). **positive:** `functional/verify` [`llgr-import-no-llgr.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/llgr-import-no-llgr.ci#L6). **negative:** `functional/verify` [`llgr-import-no-llgr.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/llgr-import-no-llgr.ci#L7) |
| `RFC9494-4.2-6` | * The helper router MUST perform the procedures listed in Section 4.3. (§4.2, §4.3) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC9494_LLGRStaleRouteIsLeastPreferred`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L137). **negative:** `unit/verify` [`TestRFC9494_RouteWithoutLLGRStaleNotLeastPreferred`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L175) |
| `RFC9494-4.2-7` | However, in the case of consecutive restarts, the previously marked stale routes MUST NOT be deleted before the timer for the Long-Lived Stale Time expires. (§4.2) | MUST NOT | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze applies the RFC 4724 Section 4.2 consecutive-restart rule unconditionally, including during an LLGR period -- every activation of a session-down dispatches "request bgp rib purge-stale <peer>" as its first step (internal/component/bgp/plugins/gr/gr.go:362, internal/component/bgp/plugins/gr/gr.go:532), and onSessionDown clears the prior peer state through clearPeerLocked (internal/component/bgp/plugins/gr/gr_state.go:125), so routes marked stale in the previous cycle are deleted at the new session drop rather than kept until their LLST timer expires |
| `RFC9494-4.2-8` | Similar to [RFC4724], once the LLGR Period begins, the Helper MUST immediately remove all the stale routes from the peer that it is retaining for that address family if any of the following occur: * the F bit for a specific address family is not set in the newly received LLGR Capability, or * a specific address family is not included in the newly received LLGR Capability, or * the LLGR and accompanying GR Capability are not received in the re-established session at all. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestOnSessionReestablished_DuringLLGR_NoCaps`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L849). **positive:** `unit/verify` [`TestOnSessionReestablished_FamilyAbsentFromLLGRPurges`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_reconnect_test.go#L65). **positive:** `unit/verify` [`TestOnSessionReestablished_LLGRFBitClearPurges`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_reconnect_test.go#L40). **positive:** `unit/verify` [`TestOnSessionReestablished_NoLLGRCapPurgesLLGRPeriodFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_reconnect_test.go#L93). **negative:** `unit/verify` [`TestOnSessionReestablished_BothFBitsSetKeepsLLGRPeriodFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_reconnect_test.go#L133). **negative:** `unit/verify` [`TestOnSessionReestablished_DuringLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L798). **negative:** `unit/verify` [`TestOnSessionReestablished_GRPeriodFamilyIgnoresLLGRBullets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_reconnect_test.go#L156) |
| `RFC9494-4.2-9` | If a Long-Lived Stale Time timer is running for routes with a given AFI/SAFI received from a peer, it MUST NOT be updated (other than by manual operator intervention) until the peer has established and synchronized a new session. (§4.2) | MUST NOT | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a consecutive session drop while the peer is already in LLGR replaces the running timer -- onSessionDown calls clearPeerLocked, which stops every LLST timer via stopLLSTTimersLocked (internal/component/bgp/plugins/gr/gr_state.go:125, :457-465, :476-481), and enterLLGRLocked then arms a fresh time.AfterFunc for the family's full LLST (internal/component/bgp/plugins/gr/gr_state.go:384-387), so the remaining Long-Lived Stale Time is reset before the peer has established and synchronized a new session |
| `RFC9494-4.3-1` | A BGP speaker that has advertised the Long-Lived Graceful Restart Capability to a neighbor MUST perform the following upon receiving a route from that neighbor with the LLGR_STALE community or upon attaching the LLGR_STALE community itself per Section 4.2: * Treat the route as the least preferred in route selection (see below). (§4.3, §4.4) | MUST | 4.3 | **positive:** `unit/verify` [`TestComparePair_LLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L820). **positive:** `unit/verify` [`TestLLGRIngressFilter_MarksReceivedStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_ingress_test.go#L72). **positive:** `unit/verify` [`TestRFC9494LLGRStaleLosesEveryLaterTieBreak`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_least_preferred_test.go#L37). **positive:** `unit/verify` [`TestRFC9494NegotiateWithKeysOnAdvertisedLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9494_llgr_trigger_test.go#L46). **positive:** `unit/verify` [`TestRFC9494ReceivedEventCarriesIngressStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9494_llgr_ingress_meta_test.go#L101). **positive:** `unit/verify` [`TestRFC9494ReceivedLLGRStaleIsLeastPreferred`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_received_stale_test.go#L125). **positive:** `unit/verify` [`TestRFC9494SelfAttachedLLGRStaleIsLeastPreferred`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_self_attach_test.go#L95). **negative:** `unit/verify` [`TestComparePair_GRStaleCompetesNormally`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L856). **negative:** `unit/verify` [`TestLLGRIngressFilter_NoLLGRSentLeavesFresh`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_ingress_test.go#L91). **negative:** `unit/verify` [`TestRFC9494NegotiateWithKeysOnAdvertisedLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9494_llgr_trigger_test.go#L48). **negative:** `unit/verify` [`TestRFC9494ReceivedEventNoLLGRSessionStaysFresh`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9494_llgr_ingress_meta_test.go#L117). **negative:** `unit/verify` [`TestRFC9494ReceivedLLGRStaleFromNonLLGRNeighborCompetesNormally`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_received_stale_test.go#L198). **negative:** `unit/verify` [`TestRFC9494ReceivedOtherCommunityCompetesNormally`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_received_stale_test.go#L146). **negative:** `unit/verify` [`TestRFC9494SelfAttachSkipsRouteLearnedAfterLoss`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_self_attach_test.go#L114). **positive:** `functional/verify` [`llgr-stale-received-depreferenced.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/llgr-stale-received-depreferenced.ci#L10). **negative:** `functional/verify` [`llgr-stale-received-depreferenced.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/llgr-stale-received-depreferenced.ci#L11) |
| `RFC9494-4.3-2` | The LLGR_STALE community MUST NOT be removed when the route is further advertised (§4.3) | MUST NOT | 4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** two egress paths strip the whole COMMUNITIES attribute with no LLGR_STALE (0xFFFF0006) exemption. applyFactsSendCommunity emits a whole-attribute suppress for code 8 whenever the peer's `send-community` is `none` or omits `standard` (internal/component/bgp/reactor/peer_forward_facts.go:249, mask set at :195-223), and it runs on both readvertise rails (internal/component/bgp/reactor/reactor_api_forward.go:516, internal/component/bgp/reactor/forward_rs.go:343). A `community-remove ffff0006` filter reaches the same attribute through AttrModRemove (internal/component/bgp/reactor/filter_delta.go:270), and removeValues drops every matching 4-octet value without checking which community it is (internal/component/bgp/plugins/filter_community/handler.go:120-131). The RIB-side attach path only appends (internal/component/bgp/plugins/rib/rib_commands_community.go:222-236), but that is not the path on which the community is lost |
| `RFC9494-4.4-1` | A least preferred route MUST be treated as less preferred than any other route that is not also least preferred. (§4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestSelectBest_LLGRStaleDepreference`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L755). **negative:** `unit/verify` [`TestSelectBest_BothLLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L783) |
| `RFC9494-4.5-1` | If the LLGR Capability is received without an accompanying GR Capability, the LLGR Capability MUST be ignored, that is, the implementation MUST behave as though no LLGR Capability has been received. (§4.5) | MUST | 4.5 | **positive:** `unit/verify` [`TestHandleEventOpenLLGR_NoGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_event_test.go#L301). **negative:** `unit/verify` [`TestHandleEventOpenLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_event_test.go#L266) |
| `RFC9494-4.6-1` | The neighbors MUST be internal (Internal BGP (IBGP) or Confederation) neighbors. (§4.6) | MUST | 4.6 | **positive:** `unit/verify` [`TestLLGREgressFilter_IBGPPartial`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L429). **negative:** `unit/verify` [`TestLLGREgressFilter_EBGPNonLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L399) |
| `RFC9494-4.6-2` | * The NO_EXPORT community [RFC1997] MUST be attached to the stale routes. (§4.6) | MUST | 4.6 | **positive:** `unit/verify` [`TestLLGREgressFilter_IBGPPartial`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L433). **positive:** `unit/verify` [`TestLLGREgressFilter_NilStateDepreferencesIBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L182). **negative:** `unit/verify` [`TestLLGREgressFilter_LLGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L369) |
| `RFC9494-4.6-3` | * The stale routes MUST have their LOCAL_PREF set to zero. (§4.6) | MUST | 4.6 | **positive:** `unit/verify` [`TestLLGREgressFilter_IBGPPartial`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L436). **positive:** `unit/verify` [`TestLLGREgressFilter_NilStateDepreferencesIBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L186). **negative:** `unit/verify` [`TestLLGREgressFilter_LLGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L373) |
| `RFC9494-4.7.2-1` | In addition to including the path attributes derived from the ATTR_SET attribute in the advertised route as per [RFC6368], the PE router MUST also include the LLGR_STALE community if it is present in the path attributes of the imported route, even if it is not present in the ATTR_SET attribute. (§4.7.2) | MUST | 4.7.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze fills the PE role on the control plane (it originates and reflects VPN routes) but implements no ATTR_SET attribute and no RFC 6368 PE-CE model, and no owner decision declines them: the attribute code table (internal/core/bgp/attribute/attribute.go) holds no code 128 and no VRF imports a VPN route into a CE session, so no imported route's LLGR_STALE is carried |
| `RFC9494-4.7.2-2` | In addition to including in the VPN route the ATTR_SET derived from the path attributes as per [RFC6368], the PE router MUST also include the LLGR_STALE community in the VPN route if it is present in the path attributes of the route as received from the CE. (§4.7.2) | MUST | 4.7.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no VRF export of a CE route into a VPN address family and no owner decision declines it: route-target handling is only the extended community and the RTC NLRI codec (internal/component/bgp/plugins/nlri/rtc), so no CE route's LLGR_STALE reaches a VPN route |
| `RFC9494-5-1` | Implementations MUST NOT enable these procedures by default. (§5) | MUST NOT | 5 | **positive:** `unit/verify` [`TestRFC9494HelperProceduresOnPerConfiguredFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_helper_default_red_test.go#L117). **positive:** `unit/verify` [`TestRFC9494_LLGRNotEnabledByDefault`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L119). **negative:** `unit/verify` [`TestRFC9494HelperProceduresOffWithoutLocalConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_helper_default_red_test.go#L81). **negative:** `unit/verify` [`TestRFC9494_LLGREnabledByExplicitConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L144) |
| `RFC9494-5-2` | They MUST require affirmative configuration per AFI/SAFI in order to enable them. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze requires affirmative configuration, but its granularity is the peer, not the AFI/SAFI -- parseLLGRCapValue reads a single long-lived-stale-time from the peer's (or group's) graceful-restart container and stamps that same LLST onto every negotiated family (internal/component/bgp/plugins/gr/gr_llgr.go:130-171), and the YANG leaf sits in the per-peer capability container with the description "Applied to all negotiated address families for the peer" (internal/component/bgp/plugins/gr/yang/ze-graceful-restart.yang), so an operator cannot enable LLGR for one address family and leave another off |
| `RFC9494-4.2-10` | If the session subsequently resets prior to becoming synchronized, any remaining routes (for the AFI/SAFI whose LLST timer expired) MUST be removed immediately. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze holds no record of an LLST that elapsed during synchronization, because no LLST timer runs then -- onSessionReestablished stops every LLST timer and clears state.inLLGR the moment the session comes back (internal/component/bgp/plugins/gr/gr_state.go:194, :231, :476-481), so the expiry this requirement keys on cannot be observed and the next session reset starts an ordinary GR cycle with a full restart timer (internal/component/bgp/plugins/gr/gr_state.go:153-156) instead of removing the family's remaining routes immediately |
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
| [`RFC9494-4.2-7`](#rfc9494-4.2-7) However, in the case of consecutive restarts, the previously marked stale routes MUST NOT be deleted before the timer for the Long-Lived Stale Time expires. (§4.2) | {gap}, no test | ze applies the RFC 4724 Section 4.2 consecutive-restart rule unconditionally, including during an LLGR period -- every activation of a session-down dispatches "request bgp rib purge-stale <peer>" as its first step (internal/component/bgp/plugins/gr/gr.go:362, internal/component/bgp/plugins/gr/gr.go:532), and onSessionDown clears the prior peer state through clearPeerLocked (internal/component/bgp/plugins/gr/gr_state.go:125), so routes marked stale in the previous cycle are deleted at the new session drop rather than kept until their LLST timer expires |
| [`RFC9494-4.2-9`](#rfc9494-4.2-9) If a Long-Lived Stale Time timer is running for routes with a given AFI/SAFI received from a peer, it MUST NOT be updated (other than by manual operator intervention) until the peer has established and synchronized a new session. (§4.2) | {gap}, no test | a consecutive session drop while the peer is already in LLGR replaces the running timer -- onSessionDown calls clearPeerLocked, which stops every LLST timer via stopLLSTTimersLocked (internal/component/bgp/plugins/gr/gr_state.go:125, :457-465, :476-481), and enterLLGRLocked then arms a fresh time.AfterFunc for the family's full LLST (internal/component/bgp/plugins/gr/gr_state.go:384-387), so the remaining Long-Lived Stale Time is reset before the peer has established and synchronized a new session |
| [`RFC9494-4.3-2`](#rfc9494-4.3-2) The LLGR_STALE community MUST NOT be removed when the route is further advertised (§4.3) | {gap}, no test | two egress paths strip the whole COMMUNITIES attribute with no LLGR_STALE (0xFFFF0006) exemption. applyFactsSendCommunity emits a whole-attribute suppress for code 8 whenever the peer's `send-community` is `none` or omits `standard` (internal/component/bgp/reactor/peer_forward_facts.go:249, mask set at :195-223), and it runs on both readvertise rails (internal/component/bgp/reactor/reactor_api_forward.go:516, internal/component/bgp/reactor/forward_rs.go:343). A `community-remove ffff0006` filter reaches the same attribute through AttrModRemove (internal/component/bgp/reactor/filter_delta.go:270), and removeValues drops every matching 4-octet value without checking which community it is (internal/component/bgp/plugins/filter_community/handler.go:120-131). The RIB-side attach path only appends (internal/component/bgp/plugins/rib/rib_commands_community.go:222-236), but that is not the path on which the community is lost |
| [`RFC9494-4.7.2-1`](#rfc9494-4.7.2-1) In addition to including the path attributes derived from the ATTR_SET attribute in the advertised route as per [RFC6368], the PE router MUST also include the LLGR_STALE community if it is present in the path attributes of the imported route, even if it is not present in the ATTR_SET attribute. (§4.7.2) | {gap}, no test | ze fills the PE role on the control plane (it originates and reflects VPN routes) but implements no ATTR_SET attribute and no RFC 6368 PE-CE model, and no owner decision declines them: the attribute code table (internal/core/bgp/attribute/attribute.go) holds no code 128 and no VRF imports a VPN route into a CE session, so no imported route's LLGR_STALE is carried |
| [`RFC9494-4.7.2-2`](#rfc9494-4.7.2-2) In addition to including in the VPN route the ATTR_SET derived from the path attributes as per [RFC6368], the PE router MUST also include the LLGR_STALE community in the VPN route if it is present in the path attributes of the route as received from the CE. (§4.7.2) | {gap}, no test | ze implements no VRF export of a CE route into a VPN address family and no owner decision declines it: route-target handling is only the extended community and the RTC NLRI codec (internal/component/bgp/plugins/nlri/rtc), so no CE route's LLGR_STALE reaches a VPN route |
| [`RFC9494-5-2`](#rfc9494-5-2) They MUST require affirmative configuration per AFI/SAFI in order to enable them. (§5) | {gap}, no test | ze requires affirmative configuration, but its granularity is the peer, not the AFI/SAFI -- parseLLGRCapValue reads a single long-lived-stale-time from the peer's (or group's) graceful-restart container and stamps that same LLST onto every negotiated family (internal/component/bgp/plugins/gr/gr_llgr.go:130-171), and the YANG leaf sits in the per-peer capability container with the description "Applied to all negotiated address families for the peer" (internal/component/bgp/plugins/gr/yang/ze-graceful-restart.yang), so an operator cannot enable LLGR for one address family and leave another off |
| [`RFC9494-4.2-10`](#rfc9494-4.2-10) If the session subsequently resets prior to becoming synchronized, any remaining routes (for the AFI/SAFI whose LLST timer expired) MUST be removed immediately. (§4.2) | {gap}, no test | ze holds no record of an LLST that elapsed during synchronization, because no LLST timer runs then -- onSessionReestablished stops every LLST timer and clears state.inLLGR the moment the session comes back (internal/component/bgp/plugins/gr/gr_state.go:194, :231, :476-481), so the expiry this requirement keys on cannot be observed and the next session reset starts an ordinary GR cycle with a full restart timer (internal/component/bgp/plugins/gr/gr_state.go:153-156) instead of removing the family's remaining routes immediately |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC9494-3.1-1`](#rfc9494-3.1-1)

If the LLGR capability is advertised, the Graceful Restart capability [RFC4724] MUST also be advertised; see Section 4.1. (§3.1, §4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: declaring code 71 without code 64. TestRFC9494_LLGRCapDeclaredWithGRCap fails unless extractGRCapabilities returns one code-64 declaration for the same peer as the code-71 one (require.Len grCaps 1, Code 64, same Peers). Negative: TestRFC9494_NoLLGRCapWithoutGRContainer places long-lived-stale-time outside the graceful-restart container and fails if either code 71 or code 64 is declared, so no config shape yields 71 alone.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9494_NoLLGRCapWithoutGRContainer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L53) | unit/verify | not verified |
| positive | [`TestRFC9494_LLGRCapDeclaredWithGRCap`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L29) | unit/verify | not verified |

### [`RFC9494-4.1-1`](#rfc9494-4.1-1)

If it is not so advertised, the LLGR Capability MUST be disregarded. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent current-source rejudgment, not authored by this judge. Read the complete normative Sections4.1-4.6 and5 context, current requirement/derived ledger, every whole tagged question and actual helpers/producers for this row. Preserve the existing verdict and scope, without claiming whole-RFC conformance. Section4.1 requires disregarding LLGR received without GR. Both complete structured OPEN tests use correctly framed capability TLVs: absent code64 leaves neither capability in the owner701 map; the conforming code64+71 input preserves two families, F bits, RestartTime240 and exact received LLST7200/3600. These are exact absent/present capability outcomes, not malformed OPEN rejection. The captured original connection41 comes from grEventFixture and matches the stored capabilities. No new machine record exists for these two tags; the enforced verdict is the source assertion judgment, not a new runtime claim.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestHandleStructuredOpenGRPlusLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_event_test.go#L689) | unit/verify | not verified |
| positive | [`TestHandleStructuredOpenLLGRNoGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_event_test.go#L610) | unit/verify | not verified |

### [`RFC9494-3.1-2`](#rfc9494-3.1-2)

The remaining bits are reserved and MUST be set to zero by the sender and ignored by the receiver. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Two clauses, both asserted. Sender: TestRFC9494_FlagsReservedBitsZeroOnSend fails unless the encoded Flags octet is exactly 0x80 (any reserved bit set goes red). Receiver: TestRFC9494_FlagsReservedBitsIgnoredOnReceive fails if a 0xFF Flags octet errors or decodes differently from 0x80, and if 0x7F leaks into the F bit.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9494_FlagsReservedBitsIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L91) | unit/verify | not verified |
| positive | [`TestRFC9494_FlagsReservedBitsZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L71) | unit/verify | not verified |

### [`RFC9494-4-1`](#rfc9494-4-1)

If a BGP speaker is configured to support the procedures of this document, it MUST use BGP Capabilities Advertisement [RFC5492] to advertise the Long-Lived Graceful Restart Capability. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 9494 Section 4: "If a BGP speaker is configured to support the procedures of this document, it MUST use BGP Capabilities Advertisement [RFC5492] to advertise the Long-Lived Graceful Restart Capability." Re-judged 2026-10-15 after the positive tag's claim was reworded (body unchanged): extractLLGRCapabilities is declared through grCapabilities, the plugin's BGPCapabilityExtractor (gr_capability.go), which the claim now names. TestExtractLLGRCapabilities_Basic fails unless extractLLGRCapabilities emits one Code 71 hex declaration for the peer whose payload decodes to LLST 3600. Negative: TestExtractLLGRCapabilities_NoLLGR asserts no declaration without long-lived-stale-time. Proven at the plugin's capability-declaration boundary; the hand-off to the OPEN is not asserted in these units.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestExtractLLGRCapabilities_NoLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_test.go#L690) | unit/verify | not verified |
| positive | [`TestExtractLLGRCapabilities_Basic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_test.go#L663) | unit/verify | not verified |

### [`RFC9494-4.2-1`](#rfc9494-4.2-1)

After the session goes down, and before the session is re- established, the stale routes for an AFI/SAFI MUST be retained. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent current-source rejudgment, not authored by this judge. Read the complete normative Sections4.1-4.6 and5 context, current requirement/derived ledger, every whole tagged question and actual helpers/producers for this row. Preserve the existing verdict and scope, without claiming whole-RFC conformance. Section4.2 retention is bounded by the received GR+LLST periods, including zero and GR-omitted families. Read all10whole roots. State-only last-family/GR-only callback tests are supplementary, not the sole evidence. Real registered-RIB roots first require actual received inventory, then GR stale-level1, LLGR stale-level2, both-zero deletion, exact family survival/removal and original absolute deadlines on JSON/structured rails. The four omitted-family roots include empty/mixed GR lists, LLST2 vs sibling GR6+LLST4, an LLST8 timer that survives sibling entry, zero/unexchanged absence and a blocked immediate-family re-advertisement while sibling deadlines still fire. Before/at-boundary inventory and stale-community assertions discriminate duration/retention, not just timer-map membership. These tests do not establish consecutive-restart or synchronization requirements outside this row. Fresh native receipt scratch/job-advertiser-llgr-local-record-renewal-b4406954.log wrote this row at lines41,131,186,243,309,377,449,527, each after its own named healthy baseline and producer-halt failure. The whole-body panic proves producer dependency only; it can occur in setup, the first event-path/subcase, or an asynchronous timer. It does not independently isolate timer duration, later skipped subcases, partial deployment, or normal-return route omission. Exact useful healthy assertions and independent source judgment supply the semantic scope; no unexecuted broken branch is credited.

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent current-source rejudgment, not authored by this judge. Read the complete normative Sections4.1-4.6 and5 context, current requirement/derived ledger, every whole tagged question and actual helpers/producers for this row. Preserve the existing verdict and scope, without claiming whole-RFC conformance. Section4.2 requires each nonzero received LLST timer, starting when that family enters LLGR. All8whole roots read. Actual inventory exists one nanosecond before and disappears at the expected absolute boundary; omitted IPv4 starts at DOWN while conventional IPv6 follows its own GR interval. Sibling GR entry does not restart the running omitted-family LLST8, and blocking immediate-family readvertisement does not postpone sibling GR1+LLST2. Zero/unexchanged family controls have no retained route and no LLST timer. Callback-only legacy units are supplemented by the exact virtual-time real-RIB tests. Source deadline arithmetic subtracts dispatch delay from the original sum rather than extending it. No consecutive reset/synchronization claim is added. Fresh native receipt scratch/job-advertiser-llgr-local-record-renewal-b4406954.log wrote this row at lines603,658,707,775,847,925, each after its own named healthy baseline and producer-halt failure. The whole-body panic proves producer dependency only; it can occur in setup, the first event-path/subcase, or an asynchronous timer. It does not independently isolate timer duration, later skipped subcases, partial deployment, or normal-return route omission. Exact useful healthy assertions and independent source judgment supply the semantic scope; no unexecuted broken branch is credited.

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent current-source rejudgment, not authored by this judge. Read the complete normative Sections4.1-4.6 and5 context, current requirement/derived ledger, every whole tagged question and actual helpers/producers for this row. Preserve the existing verdict and scope, without claiming whole-RFC conformance. Section4.2 requires deletion of every stale route of the expired family. Read all4tag bindings/3distinct roots: the legacy callback-only family expiry, actual production wired purge-command observation and delayed-DOWN real registered-RIB positive/negative deadline test. The real RIB deadline controls require exact original family/owner/connection removal at expiry while the sibling remains before its own boundary. Current wireStateCallbacks dispatches source-scoped purge-stale; purgePhysicalSourceCommand removes only stale original received entries for the named family, with source ownership checked in the physical mutation consumer. Legacy purgeStaleCommand and RemoveFamilyMatching fingerprints are preserved as prior cited consumers, supplemented by the actually used physical-source boundary. No runtime guarantee is inferred from command text alone; real inventory/deadline checks supply the storage evidence. Fresh native receipt scratch/job-advertiser-llgr-local-record-renewal-b4406954.log wrote this row at lines1015,1107, each after its own named healthy baseline and producer-halt failure. The whole-body panic proves producer dependency only; it can occur in setup, the first event-path/subcase, or an asynchronous timer. It does not independently isolate timer duration, later skipped subcases, partial deployment, or normal-return route omission. Exact useful healthy assertions and independent source judgment supply the semantic scope; no unexecuted broken branch is credited.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9494DelayedDownPreservesLLSTDeadlines`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L178) | unit/verify | revert, verified |
| positive | [`TestLLSTTimerExpiry_SingleFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L664) | unit/verify | not verified |
| positive | [`TestRFC9494LLSTExpiryDeletesTheFamilysStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L207) | unit/verify | not verified |
| positive | [`TestRFC9494DelayedDownPreservesLLSTDeadlines`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L177) | unit/verify | revert, verified |

### [`RFC9494-4.2-4`](#rfc9494-4.2-4)

* The helper router MUST attach the LLGR_STALE community to the stale routes being retained. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/plugins/rib/rfc9494_rib_gr_test.go::TestAttachCommunity moved. Independent current-source rejudgment, not authored by this judge. Read the complete normative Sections4.1-4.6 and5 context, current requirement/derived ledger, every whole tagged question and actual helpers/producers for this row. Preserve the existing verdict and scope, without claiming whole-RFC conformance. Section4.2 requires attaching LLGR_STALE to retained stale routes. All3whole tagged questions read. The zero-GR real-RIB test exercises both event rails and requires literal0xffff0006 on a retained received route at level2. TestAttachCommunity requires actual pooled community bytes equalffff0006 after generic stale marking; the fresh-route negative requires attached0, fresh level retained and no community bundle. Current physical and generic attach consumers skip fresh entries and only mutate retained original source state; wireStateCallbacks orders NO_LLGR deletion before marker attachment and level2 marking. These tags prove storage attachment, not a separately observed destination-wire dissemination guarantee or neighboring4.3-2 community-preservation obligation. Fresh native receipt scratch/job-advertiser-llgr-local-record-renewal-b4406954.log wrote this row at lines1181, each after its own named healthy baseline and producer-halt failure. The whole-body panic proves producer dependency only; it can occur in setup, the first event-path/subcase, or an asynchronous timer. It does not independently isolate timer duration, later skipped subcases, partial deployment, or normal-return route omission. Exact useful healthy assertions and independent source judgment supply the semantic scope; no unexecuted broken branch is credited.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9494_FreshRoutesDoNotGetLLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L34) | unit/verify | not verified |
| positive | [`TestRFC9494ZeroRestartTimeRetainsThroughTheLLGRPeriod`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L68) | unit/verify | revert, verified |
| positive | [`TestAttachCommunity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_rib_gr_test.go#L621) | unit/verify | not verified |

### [`RFC9494-4.2-5`](#rfc9494-4.2-5)

* If any of the routes from the peer have been marked with the NO_LLGR community, either as sent by the peer or as the result of a configured policy, they MUST NOT be retained and MUST be removed as per the normal operation of [RFC4271]. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/test/fixture/plugin_fixture_llgr_import.go::llgrImportNoLLGR moved. Independent current-source rejudgment retains the existing ENFORCED verdict and its original NO_LLGR selective-removal scope; this is not whole-RFC conformance. RFC9494 Section4.2 requires routes marked NO_LLGR either by the peer or configured policy not to be retained at LLGR entry, and to be removed through normal RFC4271 operation. Read the complete normative LLGR-entry context, current requirement and derived row, all seven tag bindings (five distinct Go roots and both polarities of llgr-import-no-llgr.ci), actual helpers and producing consumers.

The real registered-RIB GR root drives both JSON and structured original owner701/connection41 DOWN paths with actually received routes. It requires the received NO_LLGR route absent, while the unmarked useful sibling remains at stale level2 carrying literal0xffff0006. The original direct deletion root requires deleted1 and an empty peer RIB for literalffff0007. The separate LLGR_STALE-only negative requires deleted0, unchanged useful route count and the original exact prefix still present. The elected-best-path pair supplies a connected terminal next hop before election, then requires exactly one IPv4/unicast best-change Withdraw for10.0.0.0/24 after matching deletion; its control carries an actual nonmatching65001:100 community and survives without any best-change event. It therefore reaches exact community comparison, rather than merely an absent-attribute branch.

Both real-daemon .ci input UPDATEs contain no community. Only marked has the actual top-level policy modifier65535:7 and explicit accepting import final. The observer first requires one policy-marked10.0.0.0/24, one useful plain10.0.1.0/24 and NO_LLGR absent on that plain control. Only after these checks does it send real TCP markers that release both source closes. After LLGR entry, success requires marked-prefix absence together with useful plain-prefix retention carrying65535:6. The .ci stderr success fence and panic rejection preserve both outcomes. This proves received-RIB lifecycle, not a separately observed downstream recipient withdrawal socket.

Actual current source separates old operator deletion from the automatic original-physical command. wireStateCallbacks first dispatches source delete-with-communityffff0007 for the original owner/connection, then attachesffff0006 and marks level2. deletePhysicalSourceCommunityCommand validates that original source and exact family/community, skips fresh input and uses the same exact four-byte comparison. removePhysicalSource narrows removal to actual original received entries, joins output, reconciles election and settles source references; the generic operator command still removes matching stale entries, reconciles sent lifecycle and publishes best changes. Existing old comments naming the generic command as GR's automatic call are not the evidence for current dispatch; the physical consumer and real registered-RIB/daemon witnesses are.

Current native evidence was read, not run by this auditor. job-advertiser-llgr-rib-record-renewal-13f8a4e0.log has clean PASS and whole-producer halt/red records31/91 for the two best-change roots. job-advertiser-attachment-preference-and-llgr-daemon-proofs-4caeb12d.log has .ci positive clean PASS3.9s and recorded source-scoped deletePhysicalSourceCommunityCommand kill at499. job-advertiser-two-final-llgr-daemon-retakes-4a26147f.log has .ci negative clean PASS3.3s and recorded same actual physical producer kill at310. These are legitimate producer-reach renewals, not claim-specific quiet-return mutations: the broken daemon logs show bgp-rib bridge callback panic, loss of show-RIB command registration and failure of the combined removal/retention observer. They do not independently identify wrong community matching, the exact missed-removal clause, or a surviving unmarked route under the broken tree; both polarities may fail because the shared producer halts. The source assertions, useful healthy controls and compositional peer-marked/policy-marked paths supply the semantic judgment. No new per-clause semantic mutant, destination-wire guarantee, unexecuted later path or adjacent4.3-3 evidence is claimed.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9494SweepKeepsTheBestPathWithoutNoLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_no_llgr_withdraw_test.go#L100) | unit/verify | revert, verified |
| negative | [`TestRFC9494_StaleRouteWithoutNoLLGRRetained`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L63) | unit/verify | not verified |
| negative | [`llgr-import-no-llgr.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/llgr-import-no-llgr.ci#L7) | functional/verify | revert, verified |
| positive | [`TestRFC9494ZeroRestartTimeRetainsThroughTheLLGRPeriod`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L69) | unit/verify | revert, verified |
| positive | [`TestRFC9494NoLLGRSweepWithdrawsTheBestPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_no_llgr_withdraw_test.go#L80) | unit/verify | revert, verified |
| positive | [`TestDeleteWithCommunity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_rib_gr_test.go#L662) | unit/verify | not verified |
| positive | [`llgr-import-no-llgr.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/llgr-import-no-llgr.ci#L6) | functional/verify | revert, verified |

### [`RFC9494-4.2-6`](#rfc9494-4.2-6)

* The helper router MUST perform the procedures listed in Section 4.3. (§4.2, §4.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Independent current-source rejudgment, not authored by this judge. Read the complete normative Sections4.1-4.6 and5 context, current requirement/derived ledger, every whole tagged question and actual helpers/producers for this row. Preserve the existing verdict and scope, without claiming whole-RFC conformance. Preserve WEAK, with no upgrade. This row obliges every Section4.3 procedure. The two current whole tagged tests prove only depreference: attach-community raises actual entry stale level to the threshold, the marked LOCAL_PREF500 candidate loses to fresh LOCAL_PREF100 through ComparePair/SelectBest, and an unmarked fresh candidate competes normally. They do not assert all Section4.3 received/self-attachment triggers, advertisement restrictions, and LLGR_STALE preservation on further advertisement. That is an existing test-scope deficiency, not proof of absent implementation. No machine discrimination record exists for these two tags, and no full-row credit is claimed.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9494_RouteWithoutLLGRStaleNotLeastPreferred`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L175) | unit/verify | not verified |
| positive | [`TestRFC9494_LLGRStaleRouteIsLeastPreferred`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L137) | unit/verify | not verified |

### [`RFC9494-4.2-7`](#rfc9494-4.2-7)

However, in the case of consecutive restarts, the previously marked stale routes MUST NOT be deleted before the timer for the Long-Lived Stale Time expires. (§4.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9494-4.2-7, so there is no linked test.

### [`RFC9494-4.2-8`](#rfc9494-4.2-8)

Similar to [RFC4724], once the LLGR Period begins, the Helper MUST immediately remove all the stale routes from the peer that it is retaining for that address family if any of the following occur: * the F bit for a specific address family is not set in the newly received LLGR Capability, or * a specific address family is not included in the newly received LLGR Capability, or * the LLGR and accompanying GR Capability are not received in the re-established session at all. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent current-source rejudgment, not authored by this judge. Read the complete normative Sections4.1-4.6 and5 context, current requirement/derived ledger, every whole tagged question and actual helpers/producers for this row. Preserve the existing verdict and scope, without claiming whole-RFC conformance. Read all7whole tagged roots and source helper downIntoLLGR. The prior conventional-GR/LLGR per-family scope is preserved. New-capability F-clear, missing LLGR family and nil capability cases return exactly the affected purge families and release corresponding state; a sibling retained family survives. Both F bits set keeps an LLGR family, while a family still in GR is not judged by the LLGR bullets even when its sibling entered LLGR. A new physical connection42 is distinct from retained41; the old connection cannot reestablish itself. These are state-manager immediate-removal decisions. The physical purge dispatch is read in the shared structured UP handler, but no new end-to-end LLGR F-bit reconnect wire experiment is invented. Timer/synchronization/consecutive-restart gaps outside this row remain unchanged. Fresh native receipt scratch/job-advertiser-llgr-local-record-renewal-b4406954.log wrote this row at lines1311,1369,1429,1491,1553,1615,1677, each after its own named healthy baseline and producer-halt failure. The whole-body panic proves producer dependency only; it can occur in setup, the first event-path/subcase, or an asynchronous timer. It does not independently isolate timer duration, later skipped subcases, partial deployment, or normal-return route omission. Exact useful healthy assertions and independent source judgment supply the semantic scope; no unexecuted broken branch is credited.

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

No test carries RFC9494-4.2-9, so there is no linked test.

### [`RFC9494-4.3-1`](#rfc9494-4.3-1)

A BGP speaker that has advertised the Long-Lived Graceful Restart Capability to a neighbor MUST perform the following upon receiving a route from that neighbor with the LLGR_STALE community or upon attaching the LLGR_STALE community itself per Section 4.2: * Treat the route as the least preferred in route selection (see below). (§4.3, §4.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent current selected-spec rejudgment on2026-11-19: preserve ENFORCED and exactly16tag bindings/16existing discrimination identities at the previous scope. Independently reread every current whole tagged question (14distinct roots), actual helper/input/registration assumptions, normative RFC9494 Sections4.3/4.4, and source capability/session, ingress metadata, received-storage/ranking, self-attachment and actual functional observer producers. No test, claim, polarity, annotation or canonical proof record is edited, and neighboring4.2-5/4.3-3 are not judged or closed.

Current renewed receipts: scratch/job-advertiser-llgr-rib-record-renewal-13f8a4e0.log has self-attach negative clean PASS122, named producer-halt FAIL127, record153; positive clean PASS186, FAIL191, record217. Both cut the same generic attachCommunityCommand actually called by the self-attach setup; they halt there before selection assertions and prove producer dependency, not a new normal-return least-preference mutation. The encompassing batch later stalled on a DIFFERENT4.3-3 replay producer; no overall batch success or replay proof is inferred. scratch/job-advertiser-llgr-ingress-negative-renew-011a35a7.log and positive-renew-844de18a.log each have exact named clean PASS2, intentional notifyMessageReceiverFrom halt FAIL7 and native record31. They prove their stand-in ingress/session metadata carrier reaches its actual producer. They do not become an integrated real-GR-filter or normal-return metadata-drop proof. All four current canonical entries carry those renewed producer/claim/unit bindings.

The remaining12existing records and all existing paired/solo provenance are preserved, not represented as newly rerun here. Full source review agrees with the previous bounded judgment: own-advertised capability controls, exact source meta and received stale flags, marked versus fresh candidate selection, and the actual daemon observer's2prefixes distinguish the requirement's triggers and least-preference outcome. Existing current producer files have no working-tree diff; the reviewed record/summary/test/producer/helper artifact pins are checked unchanged before and after the native light stamp. Native proof fingerprints are left to repository writers, never authored as fresh by this judge. No new whole-RFC conformance, egress replay/retention scope or runtime PASS follows from this SOURCE judgment.

Historical same-scope reasoning and observations retained below as historical evidence, including semantic daemon paired/solo limitations; none becomes a fresh run by retention:

Historical independent same-scope rejudgment of RFC9494-4.3-1. The enforced verdict, full requirement quote/MUST level and all 16 original cover keys remain. RFC 9494 Sections 4.3 and 4.4 require received LLGR_STALE from a neighbor to which Ze advertised LLGR, or LLGR_STALE self-attached under Section 4.2, to be least preferred against a route not also least preferred.

The capability-trigger pair distinguishes Ze's own advertised capabilities from the peer's capabilities. The direct ingress-helper pair confines stale metadata to LLGR_STALE from an advertised-LLGR source, retaining other-community/no-community and no-capability controls; it is not the runtime filter adapter. The reactor metadata pair uses a stand-in ingress step and checks the session fact and exact uint8(2) metadata on the dispatched event, not a real GR filter invocation. The received-route RIB cases drive handleReceivedStructured: stale LOCAL_PREF 200 loses at BestStepStale to fresh LOCAL_PREF 100, while NO_EXPORT or a non-LLGR ingress decision competes normally. Fresh refresh, stale purge and equal-byte refresh controls remain intact. Their only setup repair supplies the genuine connected 10.0.0.1/32 prerequisite; all six roots passed, race 1.575s. The comparison carriers check both directions/orders with later attributes favoring the stale loser, the below-threshold GR negative, and ordinary tie-breaking between two LLGR-stale routes.

The self-attach pair drives the real mark-stale and attach-community commands. The retained LOCAL_PREF 200 route loses at BestStepStale; a route learned after mark-stale remains fresh and wins by LOCAL_PREF, while the retained route is still depreferred. The real connected next-hop prerequisite and removal of incidental Parallel calls preserve every input and assertion. Both corrected roots passed in 08fae21a, race 1.555s. Native refreshes 677faca0/b76425bb retain their original body-panic revert routes and prove producer reachability, not normal-return discrimination. Historical self-attach semantic mutations in the earlier audit were not rerun here. The existing residual remains: onLLGREnter also dispatches mark-stale level 2 after attach-community, so either depreference producer can mask removal of the other in that complete sequence; the tagged self-attach unit pins the attach producer.

The daemon still checks both original cases: advertised-LLGR stale 10/8 loses to the lower-LOCAL_PREF fresh peer at stale-level, and the same community on non-advertised-LLGR 10.1/16 wins by local-preference. The observer now installs only two real registered-static terminal /32 covers for 10.0.0.1 and 10.0.0.2, private instance 9494/device metadata, and verifies exact candidate identities, selected/nonhidden zero-metric paths without gateways and exactly two resolved, zero-cost, nonrecursive metrics before the unchanged EOR wait. Each of the internal, forked-GR and community-strip cases starts the real rib command provider. Each replaces the runner's last-OK then=stop directive with expect=exit:code=0, allowing Observe's post-scenario quiesce and self-shutdown to finish; both exact semantic markers, all rejections, wire messages, capability distinctions, grants, linger settings and authored budgets remain. The failed load capture 8ec80164 supports premature runner stop by source and chronology, not an OS signal trace; its EOF was never ignored. Main audited the corrected retained load capture ending 20261116-053122-890783705.log: 80 exact successful invocations, 240 named cases and 1920 passing VERIFY steps, with no skips or bad invocation. Canonical 2fd7600a then passed all three cases and 24 VERIFY steps after promotion.

All 16 record identities remain: 14 producer-disable revert records and two daemon semantic-mutant records. The negative's original single advertised-capability cut stayed green in diagnostic 65078f67 because the earlier per-peer config guard omitted the ingress member from the plain peer. This is not represented as a successful proof. Native 5dc9f02b instead observed the source-derived pair: remove the predicate's advertised-LLGR guard and bypass only the insertion skip for bgp-gr:llgr-ingress. The clean case passed; the combined normal-return defect wrongly depreferred the non-LLGR neighbor's /16 and selected 127.0.0.1 instead of ::1 at stale-level 2 versus 0; both independently applied cuts passed alone. The recorded paired/solo provenance is explicit, not an original single-guard refresh. Native d997f80e observed the positive at the actual OnFilter -> evaluateGRFilter -> evaluateLLGRIngress owner: the clean case passed, and an always-early-accept guard suppressed its stale metadata set, making /8 select 127.0.0.1 instead of ::1 by LOCAL_PREF 100 versus 200. This honestly retargets the former legacy LLGRIngressFilter binding while preserving the same positive claim and citation. Both semantic failures were normal returns with the intended assertions, not panic or build failures; no new cover or broader RFC credit is claimed.

Main read the two actual Atomic notifyMessageReceiver refreshes in f4fe4e33: both named ReceivedEvent carriers passed clean, failed their original intentional producer-disable panic, and were recorded with native exit 0. They retain their stand-in ingress/metadata scope and producer-reachability limitation. The matching Atomic producer is committed at 94079ae78b1d, and the matching DelayOpen producer and trigger fixture are committed at 86b381c3b486c7eb79acef5e941411b8ab497282; both commits are ancestors of current HEAD. Independent final review confirmed reactor_notify.go and session_negotiate.go are wholly byte-identical to HEAD, and session_negotiate.go plus the trigger fixture also exactly match the landed DelayOpen commit. Main observed both existing negotiateWith revert refreshes, adcc566b and coherent 04b9b55a, with unit fingerprint a4a518c8d14e899a and producer fingerprint 045c262b56167f39; the final reviewer read both native receipts, each with clean passing assertions and the intentional producer-disable panic red. Those reverts retain their reachability-only limit. All eight distinct producer files named by the existing 16 records, including the paired negative insertion producer, are byte-identical to HEAD. The current tag inventory matches all 16 original audit tests/units keys and all 16 existing discrimination identities and polarities: 14 Go covers plus both daemon ordinals, retaining the shared negotiation unit and daemon file #2 keys. No cover or original proof limitation is excluded. That historical native rejudge requirement concerned the complete unchanged set of16test/unit keys; the current stamp recomputes all16. This is same-scope readiness, not a claim that every older record was newly rerun or that panic reverts prove normal-return discrimination; separate RFC9494-4.3-3 RIB failures remain outside this row and are not excluded or resolved by this judgment. Independent moved-producer source rejudgment: reread RFC 9494 Sections 4.3, 4.4 and the 4.6 exception, all sixteen tagged entries and their complete current bodies, and the current received-event ingress/cache/publication boundaries. The two directly affected reactor metadata units still distinguish the local advertised LLGR fact and exact uint8(2) metadata from its absent-capability control through a stand-in ingress step. They remain metadata plumbing evidence, not real GR filter execution. The unchanged broader population continues to cover actual stale route selection, self-attachment, below-threshold controls, capability direction and both real-daemon polarities with their prior recorded limitations. The notification entry wrapper now delegates to notifyMessageReceiverFrom; its continuation loop invokes notifyOneMessage, which reads the selected session's advertised LLGR fact, attaches routeMeta to the received event and retains it in the cache before delivery. Source receipt propagation preserves these decisions and original-policy continuation handling. Add both moved owning functions as source fingerprints, retain all sixteen test/unit entries, full historical notes and enforced verdict, and renew the existing two reactor body-panic records at From after the final executable span freezes. This source rejudgment does not claim those receipts have already run, upgrade intentional producer-disable checks to semantic mutation proof, or prove the new source-session authority contract. Final same-boundary receipt reconciliation: independently read the complete job-closure-rfc-renewal-10-a4878a3b.log and job-closure-rfc-renewal-11-38e4522c.log and the current owning source. Each exact metadata carrier passed clean and failed its own intentional From body-panic; native recording accepted each original polarity cover at Reactor.notifyMessageReceiverFrom. These observed renewals supersede the earlier pending language while retaining the stand-in ingress scope and reachability-only meaning. Both code-map functions are in wholly HEAD-equal reactor_notify.go at HEAD 751524b526 after foundation d518d29e1c. No new runtime claim is made for any of the remaining fourteen entries; the full sixteen-entry row basis and all earlier qualifications remain.

Independent bounded marker-foundation impact rejudgment (2026-11-17, advertiser_marker_rfc_impact): reread authoritative RFC 9494 Sections 4.3 and 4.4, the complete current sixteen tagged entries (including both trigger and daemon ordinals), their producing boundaries and the source changes since the prior stamp. Keep the enforced verdict, the same sixteen-entry basis, every historical qualification above and both explicit source fingerprints. Since that stamp the two notification functions gained SourceOwner/PeerOwner propagation: the original wire receipt owner survives continuations, filter replacements, received RawMessage and cached inline WireUpdate. These additive identity fields do not alter the local advertised-LLGR decision, the ingress step's stale metadata, its uint8(2) delivery, ordinary below-threshold handling or least-preferred route selection. The source fact still comes from session.localLLGR and reaches PeerFilterInfo.LLGRAdvertised; routeMeta is still delivered and cached before publication. The marker foundation separately adds localGR derived from the actual sent OPEN inside publishNegotiation and observes completed outbound EOR frames; retainAdvertisedLLGR, outer negotiateWith and the cited received-wire producer are unchanged. None of this is a new RFC claim or an assertion that the new selected-refresh API is covered by the old refresh tags. Main has now renewed the two existing ReceivedEvent body-panic records at Reactor.notifyMessageReceiverFrom over the current committed source-owner producer. In job-advertiser-llgr-ingress-positive-renew-844de18a.log the named positive passed clean (0.930s) and failed the original intentional producer-disable panic (0.842s); native recording accepted the original positive cover. In job-advertiser-llgr-ingress-negative-renew-011a35a7.log the named negative passed clean (0.877s) and failed that same intentional producer-disable panic (0.767s); native recording accepted its original negative cover. Main read both complete native receipts. They remain stand-in ingress/metadata plumbing and producer reachability evidence, never normal-return semantic discrimination. The original fourteen other record identities and both daemon semantic mutation histories remain exactly within their earlier scopes, without a claim they were rerun here. Current custody check: reactor_notify.go is wholly byte-identical to HEAD (git diff --exit-code HEAD returned 0), and its last landed producer commit is 4a0c0f468a, feat(bgp): add authenticated selected-route output foundations. These two observed renewals now account for the already-landed producer; future callback/source-owner changes remain independently accountable.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestLLGRIngressFilter_NoLLGRSentLeavesFresh`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_ingress_test.go#L91) | unit/verify | revert, verified |
| negative | [`TestComparePair_GRStaleCompetesNormally`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L856) | unit/verify | revert, verified |
| negative | [`TestRFC9494ReceivedLLGRStaleFromNonLLGRNeighborCompetesNormally`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_received_stale_test.go#L198) | unit/verify | revert, verified |
| negative | [`TestRFC9494ReceivedOtherCommunityCompetesNormally`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_received_stale_test.go#L146) | unit/verify | revert, verified |
| negative | [`TestRFC9494SelfAttachSkipsRouteLearnedAfterLoss`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_self_attach_test.go#L114) | unit/verify | revert, verified |
| negative | [`TestRFC9494ReceivedEventNoLLGRSessionStaysFresh`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9494_llgr_ingress_meta_test.go#L117) | unit/verify | revert, verified |
| negative | [`TestRFC9494NegotiateWithKeysOnAdvertisedLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9494_llgr_trigger_test.go#L48) | unit/verify | revert, verified |
| negative | [`llgr-stale-received-depreferenced.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/llgr-stale-received-depreferenced.ci#L11) | functional/verify | mutant, verified |
| positive | [`TestLLGRIngressFilter_MarksReceivedStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_ingress_test.go#L72) | unit/verify | revert, verified |
| positive | [`TestComparePair_LLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L820) | unit/verify | revert, verified |
| positive | [`TestRFC9494LLGRStaleLosesEveryLaterTieBreak`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_least_preferred_test.go#L37) | unit/verify | revert, verified |
| positive | [`TestRFC9494ReceivedLLGRStaleIsLeastPreferred`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_received_stale_test.go#L125) | unit/verify | revert, verified |
| positive | [`TestRFC9494SelfAttachedLLGRStaleIsLeastPreferred`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_llgr_self_attach_test.go#L95) | unit/verify | revert, verified |
| positive | [`TestRFC9494ReceivedEventCarriesIngressStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9494_llgr_ingress_meta_test.go#L101) | unit/verify | revert, verified |
| positive | [`TestRFC9494NegotiateWithKeysOnAdvertisedLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9494_llgr_trigger_test.go#L46) | unit/verify | revert, verified |
| positive | [`llgr-stale-received-depreferenced.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/llgr-stale-received-depreferenced.ci#L10) | functional/verify | mutant, verified |

### [`RFC9494-4.3-2`](#rfc9494-4.3-2)

The LLGR_STALE community MUST NOT be removed when the route is further advertised (§4.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9494-4.3-2, so there is no linked test.

### [`RFC9494-4.4-1`](#rfc9494-4.4-1)

A least preferred route MUST be treated as less preferred than any other route that is not also least preferred. (§4.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a least-preferred route beating one that is not. TestSelectBest_LLGRStaleDepreference gives the stale candidate LOCAL_PREF 300 against 100 and fails, in both input orders, unless the normal route wins, so removing Step 0 goes red. Negative: TestSelectBest_BothLLGRStale fails unless two stale candidates fall through to LOCAL_PREF, so the rule orders only across the least-preferred boundary.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestSelectBest_BothLLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L783) | unit/verify | not verified |
| positive | [`TestSelectBest_LLGRStaleDepreference`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L755) | unit/verify | not verified |

### [`RFC9494-4.5-1`](#rfc9494-4.5-1)

If the LLGR Capability is received without an accompanying GR Capability, the LLGR Capability MUST be ignored, that is, the implementation MUST behave as though no LLGR Capability has been received. (§4.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent current-source rejudgment, not authored by this judge. Read the complete normative Sections4.1-4.6 and5 context, current requirement/derived ledger, every whole tagged question and actual helpers/producers for this row. Preserve the existing verdict and scope, without claiming whole-RFC conformance. Section4.5 requires behaving as though LLGR was absent when GR is absent. Both whole JSON OPEN tests read. A valid code71-only OPEN leaves no GR or LLGR map entry; the valid code64+71 control keeps actual decoded family/F-bit/LLST3600 data and RestartTime120. The event carries owner701/connection41, and handleOpenEvent ends in forgetGRCapability when code64 was not found, so its removal is conditional on missing GR, not universal capability suppression. No new machine record exists for these tags; this is current source assertion judgment, not a new runtime observation.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestHandleEventOpenLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_event_test.go#L266) | unit/verify | not verified |
| positive | [`TestHandleEventOpenLLGR_NoGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_event_test.go#L301) | unit/verify | not verified |

### [`RFC9494-4.6-1`](#rfc9494-4.6-1)

The neighbors MUST be internal (Internal BGP (IBGP) or Confederation) neighbors. (§4.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent current-source rejudgment, not authored by this judge. Read the complete normative Sections4.1-4.6 and5 context, current requirement/derived ledger, every whole tagged question and actual helpers/producers for this row. Preserve the existing verdict and scope, without claiming whole-RFC conformance. Section4.6 permits partial delivery only to an internal neighbor. Both whole tests read. The external non-LLGR input requires mods.IsWithdraw; the internal non-LLGR input requires delivered, not withdrawn output modifications including exact NO_EXPORT0xffffff01 and LOCAL_PREF0. Current production uses dest.IsIBGP, the captured session classification, rather than comparing AS numbers; the migrated internal fixture explicitly sets it. Existing owner/physical-connection capability lookup prevents another destination capability from granting this branch. No new machine record or actual socket-wire observation exists for these filter-unit tags.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestLLGREgressFilter_EBGPNonLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L399) | unit/verify | not verified |
| positive | [`TestLLGREgressFilter_IBGPPartial`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L429) | unit/verify | not verified |

### [`RFC9494-4.6-2`](#rfc9494-4.6-2)

* The NO_EXPORT community [RFC1997] MUST be attached to the stale routes. (§4.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent current-source rejudgment, not authored by this judge. Read the complete normative Sections4.1-4.6 and5 context, current requirement/derived ledger, every whole tagged question and actual helpers/producers for this row. Preserve the existing verdict and scope, without claiming whole-RFC conformance. Section4.6 requires NO_EXPORT on the partial-deployment stale delivery. All3whole roots read. Loaded-state and unloaded-state internal non-LLGR inputs require a COMMUNITIES Add operation with exactly four bytes0xffffff01; presence is required independently of the loop, preventing an empty operations list from passing. The LLGR-capable peer is a valid contrast with mods.Len0. Current LLGREgressFilter selects partial delivery by captured IsIBGP and applies exact communityNoExport bytes. These are exact filter-operation assertions, not a newly observed destination wire encoding or machine mutant record.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestLLGREgressFilter_LLGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L369) | unit/verify | not verified |
| positive | [`TestLLGREgressFilter_IBGPPartial`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L433) | unit/verify | not verified |
| positive | [`TestLLGREgressFilter_NilStateDepreferencesIBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L182) | unit/verify | not verified |

### [`RFC9494-4.6-3`](#rfc9494-4.6-3)

* The stale routes MUST have their LOCAL_PREF set to zero. (§4.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent current-source rejudgment, not authored by this judge. Read the complete normative Sections4.1-4.6 and5 context, current requirement/derived ledger, every whole tagged question and actual helpers/producers for this row. Preserve the existing verdict and scope, without claiming whole-RFC conformance. Section4.6 requires LOCAL_PREF0 on partial-deployment stale delivery. All3whole roots read. Loaded-state and unloaded-state internal non-LLGR inputs require a LOCAL_PREF Set operation carrying exactly four zero bytes, plus explicit presence flags. The LLGR-capable peer must receive no modifications. Source owner/connection55 fixtures match their capability snapshots, and LLGREgressFilter uses captured IsIBGP for the internal exception. No new machine record or complete socket-level partial-deployment experiment is claimed.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestLLGREgressFilter_LLGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L373) | unit/verify | not verified |
| positive | [`TestLLGREgressFilter_IBGPPartial`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L436) | unit/verify | not verified |
| positive | [`TestLLGREgressFilter_NilStateDepreferencesIBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L186) | unit/verify | not verified |

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent current-source rejudgment, not authored by this judge. Read the complete normative Sections4.1-4.6 and5 context, current requirement/derived ledger, every whole tagged question and actual helpers/producers for this row. Preserve the existing verdict and scope, without claiming whole-RFC conformance. Section5 prohibits enabling LLGR by default. All4whole tagged roots and helper rfc9494Opens read. GR-only configuration emits no code71 and the actual YANG LLST leaf has no default; explicit LLST3600 yields a correctly framed seven-byte declaration. Helper controls feed actual valid sent and received OPENs, both with GR IPv4+IPv6; absent local code71 or IPv6-only local declaration prevents IPv4 LLGR commands. The configured-IPv4 control requires exact source-owner701/connection41 NO_LLGR sweep and LLGR_STALE attach only for IPv4, with IPv6 purge and no IPv6 LLGR commands. exchangedLLGRLocked intersects received tuples with the original local sent declaration. Two still-current parseLLGRCapValue revert records remain distinct from the two freshly renewed helper records. Existing affirmative-per-AFI/SAFI configuration limitation in separate5-2 is not erased. Fresh native receipt scratch/job-advertiser-llgr-local-record-renewal-b4406954.log wrote this row at lines2034,2114, each after its own named healthy baseline and producer-halt failure. The whole-body panic proves producer dependency only; it can occur in setup, the first event-path/subcase, or an asynchronous timer. It does not independently isolate timer duration, later skipped subcases, partial deployment, or normal-return route omission. Exact useful healthy assertions and independent source judgment supply the semantic scope; no unexecuted broken branch is credited.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9494HelperProceduresOffWithoutLocalConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_helper_default_red_test.go#L81) | unit/verify | revert, verified |
| negative | [`TestRFC9494_LLGREnabledByExplicitConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L144) | unit/verify | revert, verified |
| positive | [`TestRFC9494HelperProceduresOnPerConfiguredFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_helper_default_red_test.go#L117) | unit/verify | revert, verified |
| positive | [`TestRFC9494_LLGRNotEnabledByDefault`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L119) | unit/verify | revert, verified |

### [`RFC9494-5-2`](#rfc9494-5-2)

They MUST require affirmative configuration per AFI/SAFI in order to enable them. (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9494-5-2, so there is no linked test.

### [`RFC9494-4.2-10`](#rfc9494-4.2-10)

If the session subsequently resets prior to becoming synchronized, any remaining routes (for the AFI/SAFI whose LLST timer expired) MUST be removed immediately. (§4.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9494-4.2-10, so there is no linked test.

### [`RFC9494-4.3-3`](#rfc9494-4.3-3)

The route SHOULD NOT be advertised to any neighbor from which the Long-Lived Graceful Restart Capability has not been received. The exception is described in Section 4.6. (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent final source rejudgment, 2026-11-19. Read RFC 9494 Sections 4.3 and 4.6, the current requirement and old verdict, all seventeen tagged bindings in eleven distinct roots, their fixture boundaries, the current producing boundaries, sixteen discrimination records, and the renewed Main receipts. Verdict remains enforced for this requirement, with the existing SHOULD NOT strength and Section 4.6 exception preserved. This is not a whole-RFC support claim. Section 4.3 requires that the route SHOULD NOT be advertised to a neighbor from which LLGR has not been received and explains that this implies withdrawal; Section 4.6 permits advertisement to internal neighbors only with NO_EXPORT and LOCAL_PREF zero.

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
