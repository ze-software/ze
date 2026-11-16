# RFC 6811 - BGP Prefix Origin Validation

RPKI origin validation. Every requirement listed for RFC 6811, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 5 | of 5 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 5 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 5 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 5 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 5 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 19 | 19 of 19 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Audit verdicts | 4 | of 5 mandatory requirements reviewed | 3 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 5 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 5 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 5 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 5 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 5 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 5 | of 9 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 5 mandatory requirements: they add up to 5. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | RPKI origin validation |
| Automatic checks | Included |
| Requirements | 9 |
| Mandatory requirements (MUST level) | 5 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 19 |
| Tests linked to requirements | 19 |
| Recorded audit verdicts | 4 |
| Fault test records | 19 |
| Summary | `rfc/short/rfc6811.md` |
| Requirement table | `rfc/requirements/rfc6811.md` |
| RFC text | `rfc/full/rfc6811.txt` |

## Automatic checks

Included in automatic checks: Ze implements RPKI origin validation and keeps these requirements enrolled for conformance auditing. Mandatory conformance is incomplete: implicit rejection of Invalid routes remains an open Section 2 defect, and Section 4 lacks complete mapping-deletion and decision-process proof. Existing explicit-action and revalidation tests establish bounded behavior, not full RFC support. SHOULD and MAY evidence is separate from mandatory evidence.

## Published support status

**Area:** RPKI origin validation

**Test verification:** 5 of 5 MUST proven

**Evidence:** functional 1, unit 4

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 5 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **5** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (5):** [`RFC6811-2-1`](#rfc6811-2-1), [`RFC6811-2-2`](#rfc6811-2-2), [`RFC6811-3-1`](#rfc6811-3-1), [`RFC6811-3-2`](#rfc6811-3-2), [`RFC6811-4-1`](#rfc6811-4-1)

## Figures

- **MUST bar**: Ze verified 5
- **Overview bar**: Ze MUST verified 5, Not assessed 4

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 5 |
| MUST rows kept (the MUST bar) | 5 |
| Ze verified | 5 |
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
| SHOULD / MAY rows | 4 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 4 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc6811.json](../../../data/rfc/rfc6811.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6811-2-1` | The validation state of the Route MUST be set to reflect the result of the lookup. (Section 2) | MUST | 2 - Prefix-to-AS Mapping Database | **positive:** `unit/verify` [`TestBatchValidateTypedMatchesString`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rfc6811_rib_batch_test.go#L398). **positive:** `unit/verify` [`TestRPKIOriginASFromASPathRFC6811`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_test.go#L216). **positive:** `unit/verify` [`TestRouteStateIsSetFromTheLookup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_state_test.go#L31). **positive:** `unit/verify` [`TestValidateValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_test.go#L18). **negative:** `unit/verify` [`TestRPKIOriginASFromASPathRFC6811`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_test.go#L218). **negative:** `unit/verify` [`TestRouteStateIsSetFromTheLookup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_state_test.go#L53). **negative:** `unit/verify` [`TestValidateInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_test.go#L33) |
| `RFC6811-2-2` | An implementation MUST NOT exclude a route from the Adj-RIB-In or from consideration in the decision process as a side effect of its validation state, unless explicitly configured to do so (Section 2) | MUST NOT | 2 - Prefix-to-AS Mapping Database | **positive:** `unit/verify` [`TestBuildDecisionsOriginInvalidAction`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_rpki_batch_test.go#L105). **negative:** `unit/verify` [`TestBuildDecisionsOriginInvalidAction`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_rpki_batch_test.go#L106) |
| `RFC6811-3-1` | An implementation MUST provide the ability to match and set the validation state of routes as part of its route policy filtering function (Section 3) | MUST | 3 - Policy Control | **positive:** `unit/verify` [`TestBuildDecisionsOriginInvalidAction`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_rpki_batch_test.go#L107). **negative:** `unit/verify` [`TestBuildDecisionsOriginInvalidAction`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_rpki_batch_test.go#L108) |
| `RFC6811-3-2` | An implementation MUST also support four-octet AS numbers (Section 3) | MUST | 3 - Policy Control | **positive:** `unit/verify` [`TestValidateFourOctetAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_test.go#L94). **negative:** `unit/verify` [`TestValidateFourOctetAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_test.go#L96) |
| `RFC6811-4-1` | When a mapping is added or deleted, the implementation MUST re-validate any affected prefixes and run the BGP decision process if needed (Section 4) | MUST | 4 - Interaction with Local Cache | **positive:** `unit/verify` [`TestHandleROAChangeReValidates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_origin_tracker_test.go#L45). **positive:** `unit/verify` [`TestReValidationAppliesToInstalledRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rfc6811_rib_validation_test.go#L375). **negative:** `unit/verify` [`TestHandleROAChangeReValidates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_origin_tracker_test.go#L48). **negative:** `unit/verify` [`TestReValidationAppliesToInstalledRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rfc6811_rib_validation_test.go#L378). **positive:** `functional/verify` [`rpki-revalidate-late-sync.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rpki-revalidate-late-sync.ci#L3). **positive:** `functional/verify` [`rpki-revalidate-selected-route.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rpki-revalidate-selected-route.ci#L3) |
| `RFC6811-2-3` | When a BGP speaker receives an UPDATE from a neighbor, it SHOULD perform a lookup as described above for each of the Routes in the UPDATE message. (Section 2) | SHOULD | 2 - Prefix-to-AS Mapping Database | **positive:** no positive test. **negative:** no negative test |
| `RFC6811-2-4` | The lookup SHOULD also be applied to routes that are redistributed into BGP from another source, such as another protocol or a locally defined static route. (Section 2) | SHOULD | 2 - Prefix-to-AS Mapping Database | **positive:** no positive test. **negative:** no negative test |
| `RFC6811-2-5` | If validation is not performed on a Route, the implementation SHOULD initialize the validation state of such a route to "NotFound". (Section 2) | SHOULD | 2 - Prefix-to-AS Mapping Database | **positive:** no positive test. **negative:** no negative test |
| `RFC6811-2-6` | An implementation MAY provide configuration options to control which routes the lookup is applied to (Section 2) | MAY | 2 - Prefix-to-AS Mapping Database | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 6811 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC6811-2-1`](#rfc6811-2-1)

The validation state of the Route MUST be set to reflect the result of the lookup. (Section 2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 6811 Section 2 requires the Route's validation state to reflect the lookup result. The lookup uses the final AS_PATH segment: a final AS_SEQUENCE supplies its rightmost AS, an empty or confederation-ending path uses the local speaker AS, and AS_SET supplies NONE. TestValidateValid and TestValidateInvalid use the same covered prefix with respectively authorized and unauthorized origins and assert the exact Valid and Invalid constants. TestRouteStateIsSetFromTheLookup drives validateNLRIs for independently expected Valid, Invalid and NotFound results and checks both the returned per-prefix state and the queued decision state; its second arm performs a changed lookup and requires Invalid instead of the earlier Valid. TestBatchValidateTypedMatchesString retains every original typed-versus-text parity assertion and now also requires each transport to settle all four pending inputs, retain exactly the two accepted keys with their absolute Valid and Invalid states, and omit the two explicitly discarded keys. An accepted Invalid lookup therefore cannot become Valid merely because policy accepted it, and a shared wrong state or empty result on both transports cannot pass. TestRPKIOriginASFromASPathRFC6811 retains empty, sequence, set and confederation controls and adds distinct first/final sequence values plus a sequence followed by a final AS_SET. These require final-segment selection and NONE rather than an AS_SET member. Source review traces validateNLRIs, handleBatchValidateTyped, batchValidateCommand, applyDecision and promoteToInstalled, together with ROACache.Validate and rpkiOriginASFromASPath. The five new native clean runs in job-rpki-origin-existing-native-proofs-e302411f.log passed their named tests. Normal-return producer faults then failed the intended exact assertions: forcing promotion to Valid changed both transports' accepted Invalid state from 3 to 1; reversing the Valid and Invalid lookup returns failed their respective lookup tests; selecting the first segment failed only the two new multisegment cases; using an AS_SET member as origin failed both final-set cases while empty, sequence and confederation controls passed. The seven canonical records comprise those five semantic observations and the two earlier validateNLRIs boundary-disable observations; the latter prove boundary reachability, while the test's exact expected-state assertions supply the semantic checks. The current production delivery contract is not a flattened JSON path: runRPKIPlugin subscribes to full events, handleStructuredUpdate uses segment-bearing attributes, handleEvent prefers raw attribute bytes or hex and uses the same segment-aware helper, and configuration revalidation decodes retained raw attributes before its lookup. Supplied malformed raw attributes do not fall back to the flattened path. Source also implements AS_CONFED_SET through the same local-AS branch as AS_CONFED_SEQUENCE; the existing untagged TestJSONOriginUsesLocalASAndRawSegmentTypes checks both confederation forms, empty path and AS_SET for byte and hex event forms. Those supporting tests are not counted as additional tagged discrimination records. Only the no-raw compatibility projection lacks segment types and uses the flattened path; this audit does not claim that projection can reconstruct information it never carries. The enforced verdict is for the stated MUST that a performed lookup's result becomes the route's state, with origin selection and acceptance-preservation assertions protecting that result. It does not certify every compatibility input, policy defaults or policy assignment, mapping-deletion revalidation, every address-family delivery path, or whole-RFC support; those are separate obligations and evidence. The earlier blanket claim that the normal JSON route necessarily misclassifies AS_SET or confederation origins is contradicted by the current full-event subscription and raw-attribute producers and is not retained.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRouteStateIsSetFromTheLookup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_state_test.go#L53) | unit/verify | revert, verified |
| negative | [`TestRPKIOriginASFromASPathRFC6811`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_test.go#L218) | unit/verify | mutant, verified |
| negative | [`TestValidateInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_test.go#L33) | unit/verify | mutant, verified |
| positive | [`TestBatchValidateTypedMatchesString`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rfc6811_rib_batch_test.go#L398) | unit/verify | mutant, verified |
| positive | [`TestRouteStateIsSetFromTheLookup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_state_test.go#L31) | unit/verify | revert, verified |
| positive | [`TestRPKIOriginASFromASPathRFC6811`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_test.go#L216) | unit/verify | mutant, verified |
| positive | [`TestValidateValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_test.go#L18) | unit/verify | mutant, verified |

### [`RFC6811-2-2`](#rfc6811-2-2)

An implementation MUST NOT exclude a route from the Adj-RIB-In or from consideration in the decision process as a side effect of its validation state, unless explicitly configured to do so (Section 2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RFC 6811 Section 2 states: 'An implementation MUST NOT exclude a route from the Adj-RIB-In or from consideration in the decision process as a side effect of its validation state, unless explicitly configured to do so.' TestBuildDecisionsOriginInvalidAction always assigns an explicit accept or reject action before calling buildDecisions. Its negative tests explicit acceptance, not the absence of explicit configuration. It asserts typed Accept results and preserves the accepted Invalid state, not installed Adj-RIB-In or BGP route-selection state. parseRPKIConfig initializes OriginInvalidAction to ASPAPolicyReject even for a cache-only configuration, and buildDecisions converts that implicit action to Accept=false and Ineligible=true for Invalid routes. The separate untagged TestRPKIInvalidExclusionRequiresExplicitConfiguration exposes that defect with cache-only input and preserves explicit-reject and Valid controls; a failing probe is not conformance proof. This bounded decision-builder evidence cannot justify the whole requirement as enforced.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestBuildDecisionsOriginInvalidAction`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_rpki_batch_test.go#L106) | unit/verify | mutant, verified |
| positive | [`TestBuildDecisionsOriginInvalidAction`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_rpki_batch_test.go#L105) | unit/verify | mutant, verified |

### [`RFC6811-3-1`](#rfc6811-3-1)

An implementation MUST provide the ability to match and set the validation state of routes as part of its route policy filtering function (Section 3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RFC 6811 Section 3 states: 'An implementation MUST provide the ability to match and set the validation state of routes as part of its route policy filtering function.' TestBuildDecisionsOriginInvalidAction directly initializes the Invalid disposition, supplies preexisting Invalid and Valid validation states, and calls buildDecisions. It proves explicit accept/reject actions alter typed Accept results, a Valid control remains accepted, and accepting Invalid preserves its existing ValState. The native positive break rejects Invalid regardless of the stated accept action; the negative applies the Invalid action to Valid too. Both produce the intended named assertion failures. Neither test input contains a policy that assigns a new validation state, and buildDecisions copies req.state into ValState rather than this test exercising a policy set operation. No configured policy entry point or installed-route policy result is asserted here. The bounded disposition proofs do not prove the whole match-and-set requirement. This is an evidence finding, not a claim that no implementation of the missing behavior exists elsewhere; no whole-row or whole-RFC credit is justified.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestBuildDecisionsOriginInvalidAction`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_rpki_batch_test.go#L108) | unit/verify | mutant, verified |
| positive | [`TestBuildDecisionsOriginInvalidAction`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_rpki_batch_test.go#L107) | unit/verify | mutant, verified |

### [`RFC6811-3-2`](#rfc6811-3-2)

An implementation MUST also support four-octet AS numbers (Section 3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestValidateFourOctetAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_test.go#L96) | unit/verify | mutant, verified |
| positive | [`TestValidateFourOctetAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_test.go#L94) | unit/verify | mutant, verified |

### [`RFC6811-4-1`](#rfc6811-4-1)

When a mapping is added or deleted, the implementation MUST re-validate any affected prefixes and run the BGP decision process if needed (Section 4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RFC 6811 Section 4 states: 'When a mapping is added or deleted, the implementation MUST re-validate any affected prefixes and run the BGP decision process if needed.' TestHandleROAChangeReValidates asserts a covering mapping addition changes a tracked NotFound route to Valid and enqueues a fresh validation request. Its negative calls the handler again with unchanged cache and tracked state and asserts no queued request; it does not observe unchanged or unaffected routes through the complete daemon or selection stack. TestReValidationAppliesToInstalledRoutes directly exercises stored NotFound routes through typed batch, text batch and accept-routes decisions: each accept rewrites the state to Valid, and the typed rewrite preserves replay sequence seven. Its discard control explicitly sends Accept=false and Ineligible=false and removes that stored route. This is not the current bgp-rpki Invalid-policy rejection, which sends Accept=false and Ineligible=true and retains received bytes. The unit's negative accepts a decision for an unknown prefix, requires an early buffer and leaves the existing route count unchanged without creating the absent route; this is a receive-store boundary, not a full cache-to-selection exchange. Both installed-unit native clean runs passed all five subcases. Forcing applyToInstalled to keep ValidationNotFound caused exactly the typed batch, text batch and accept-routes state assertions to fail with expected Valid and actual NotFound; explicit discard and unknown-prefix controls passed. Replacing only applyDecision's absent-route early-buffer fallback with promoteToInstalled caused only the unknown-prefix subcase to fail: it observed a second stored route, the phantom named prefix and no early buffer, while the other four subcases passed. Both were normal-return semantic failures, not build errors, panic, skip or timeout. The promoted test/plugin/rpki-revalidate-late-sync.ci asserts the real daemon's retained Adj-RIB-In route changes from NotFound/eligible to Invalid/ineligible after its first covering VRP arrives, with nonempty received attribute, next-hop and NLRI fields unchanged. That carrier loads bgp-rpki and bgp-adj-rib-in, not bgp-rib. Its native clean run passed; inverting handleROAChange's decisionRequired guard produced the named observer failure after cache synchronization, with the retained subject still NotFound and eligible. The promoted test/plugin/rpki-revalidate-selected-route.ci and rpki_selected_revalidation.go additionally require real namespace connected reachability and both received prefixes selected before releasing the first covering VRP. After release the explicitly rejected subject remains retained as Invalid/ineligible with identical received bytes and ceases to be selected in BGP best, authoritative Loc-RIB candidates and the selected system view, while the control becomes Valid and remains selected in all three views. The candidate assertion permits the subject to remain as an unselected candidate; it does not claim the candidate is absent. Source review traced handleROAChange, Adj-RIB-In applyToInstalled and validation-change publication, RIBManager.validationChanged, checkRouteBestChange and removeLocRIB. The selected carrier passed a counted own-system per-test-namespace load run with iterations=80, completed=80, executed=80 and no skipped phase. Its fresh native clean run then passed the sole selected test in 5.0 seconds with one executed test and zero skipped phases. Omitting only removeLocRIB in checkRouteBestChange's no-candidates branch produced the named selected-carrier failure in 12.2 seconds: after synchronization the authoritative Loc-RIB still selected subject 10.0.1.0/24 while control 10.0.2.0/24 remained selected. The source reaches that assertion only after both exact retained verdicts, immutable received bytes and the subject-absent/control-present BGP-best check succeed; the broken run does not reach the subsequent selected-system-view check. Both native guest runs stopped normally on Ze's 7.3.0-ze runtime kernel with matching SHA-256, and the red was the observer's semantic selection assertion, not setup, compilation, panic, skip or an outer timeout. All six native records, covering both installed-unit polarities, both tracker polarities and the two promoted functional positives, are retained in rfc/discrimination/rfc6811/RFC6811-4-1.json. These establish bounded mapping-addition, retained-validation and required selection behavior. No mapping-deletion carrier is exercised by any of these checks, so mapping deletion remains unproven and the whole requirement stays weak with no whole-row or whole-RFC credit.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestReValidationAppliesToInstalledRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rfc6811_rib_validation_test.go#L378) | unit/verify | mutant, verified |
| negative | [`TestHandleROAChangeReValidates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_origin_tracker_test.go#L48) | unit/verify | mutant, verified |
| positive | [`TestReValidationAppliesToInstalledRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rfc6811_rib_validation_test.go#L375) | unit/verify | mutant, verified |
| positive | [`TestHandleROAChangeReValidates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_origin_tracker_test.go#L45) | unit/verify | mutant, verified |
| positive | [`rpki-revalidate-late-sync.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rpki-revalidate-late-sync.ci#L3) | functional/verify | mutant, verified |
| positive | [`rpki-revalidate-selected-route.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rpki-revalidate-selected-route.ci#L3) | functional/verify | mutant, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6 phase 4, rfc6811 |
| Signed off | 2026-08-31 |
| Register | rfc2119 |
| Source | rfc/full/rfc6811.txt |
| Source fingerprint | 1d1449c6d19aa61f |
| Record | rfc/extraction/rfc6811.json |
| Mapped sentences | 5 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of This Memo, Copyright Notice, Abstract and Table of Contents. The Abstract restates the Introduction and states no obligation. |
| `1` | Introduction | 0 | reviewed | Introduction. Indicative prose: the threat the document addresses, the three components of the RPKI architecture, that the cache retrieval protocol is out of scope (RFC 6810), that full AS_PATH attestation is out of scope, and that the global RPKI is only loosely consistent across caches. Its only modal is the lowercase 'The cache must also be refreshed periodically', which section 1.1 gives no special meaning. No sentence directs a speaker. |
| `1.1` | Requirements Language | 0 | reviewed | Requirements Language. The RFC 2119 key-words paragraph, which also states that the key words bind only when they appear in all upper case, and that is what disqualifies the lowercase modals in sections 1, 5 and 6. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. |
| `2` | Prefix-to-AS Mapping Database | 2 | reviewed | Prefix-to-AS Mapping Database. The document's main normative section. It defines VRP, Prefix, Route, VRP Prefix, VRP ASN, Route Prefix, Route Origin ASN, Covered and Matched, then the three validation states NotFound, Valid and Invalid, then states the two capitalised MUST-level sites mapped below to RFC6811-2-1 and RFC6811-2-2. Its remaining directives are three SHOULDs and one MAY, which are the unsourced ids below: perform the lookup for each Route in a received UPDATE, apply it to redistributed routes, initialize an unvalidated route to NotFound, and offer configuration options controlling which routes the lookup applies to. The definitions are indicative, so under section 1.1 they carry no RFC 2119 level. "A Route Prefix is said to be Covered by a VRP when the VRP prefix length is less than or equal to the Route prefix length" names a term the lookup uses rather than directing a speaker, and the same holds for Matched and for the three states. Two further indicative sentences are observations that scope the lookup rather than adding to it: a Route whose Origin ASN is "NONE" cannot be Matched by any VRP, and a Route can be Covered or Matched by more than one VRP while the state output stays fully determined. |
| `2.1` | Pseudo-Code | 0 | reviewed | Pseudo-Code. A C-like listing of the section 2 procedure. The section opens by declaring itself subordinate: "In case of ambiguity, the procedure above, rather than the pseudo-code, should be taken as authoritative." It therefore adds no obligation. Its constant names BGP_PFXV_STATE_NOT_FOUND, BGP_PFXV_STATE_VALID and BGP_PFXV_STATE_INVALID are carried by the Constants table of rfc/short/rfc6811.md. |
| `3` | Policy Control | 2 | reviewed | Policy Control. Two capitalised MUST-level sites, mapped below to RFC6811-3-1 and RFC6811-3-2. Its two remaining sentences point the reader at section 5 and at [ORIGIN-OPS] for operational policy guidance, and neither directs a speaker. |
| `4` | Interaction with Local Cache | 1 | reviewed | Interaction with Local Cache. One capitalised MUST-level site, mapped below to RFC6811-4-1. Its remaining sentences state that a speaker is expected to talk to one or more RPKI caches over the RFC 6810 protocol, and define an "affected prefix" as any prefix that was matched by a deleted or updated mapping, or could be matched by an added or updated mapping. That definition scopes RFC6811-4-1 rather than adding a requirement, and it is indicative. |
| `5` | Deployment Considerations | 0 | reviewed | Deployment Considerations. Names policies an operator can implement, filtering Invalid routes and adjusting LOCAL_PREF among them, and observes that propagating the validation state to an IBGP peer through a community can be necessary for routing correctness. Its modals are all lowercase, "should be done with utmost care" and "it could be necessary", which section 1.1 gives no special meaning. No sentence directs a speaker. |
| `6` | Security Considerations | 0 | reviewed | Security Considerations. States that the security of the whole rests on the database, that blocking "invalid" routes opens a denial-of-service vector when an attacker can inject or remove validation records, that an attacker defeats the mechanism by prepending the authorized origin AS to a forged announcement, and that no path validation is provided. Its one lowercase "must be considered" addresses the reader assessing the overall solution rather than a speaker, and no countermeasure is directed at a speaker. |
| `7` | Acknowledgments | 0 | skipped (acknowledgements) | Acknowledgments. |
| `8` | References | 0 | skipped (references) | References. The section header only; its entries are in sections 8.1 and 8.2. |
| `8.1` | not stated | 0 | skipped (references) | Normative References: RFC 2119, RFC 3779, RFC 4271, RFC 4632, RFC 6482, RFC 6793. |
| `8.2` | not stated | 0 | skipped (references) | Informational References: [AS0], [ORIGIN-OPS], RFC 6480, RFC 6810. |

### Excluded sentences

The review of RFC 6811 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 6811, so its obligations are stated where they were written.
