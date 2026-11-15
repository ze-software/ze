# RFC 6811 - BGP Prefix Origin Validation

RPKI origin validation. Every requirement listed for RFC 6811, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

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
| Tests shown to detect a fault | 2 | 2 of 17 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests counts as proof only after this check has demonstrated that they can detect a fault |

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
| Mandatory requirements (MUST level) | 5 | of 9 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

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
| Public row | RPKI origin validation |
| Automatic checks | Included |
| Requirements | 9 |
| Mandatory requirements (MUST level) | 5 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 17 |
| Tests linked to requirements | 17 |
| Recorded audit verdicts | 1 |
| Fault test records | 2 |
| Summary | `rfc/short/rfc6811.md` |
| Requirement table | `rfc/requirements/rfc6811.md` |
| RFC text | `rfc/full/rfc6811.txt` |

## Automatic checks

Included in automatic checks: RPKI-based Prefix Origin Validation: all five MUST-level requirements tested with both polarities. RFC6811-2-1 (state reflects the lookup) and RFC6811-3-2 (four-octet AS) over ROACache.Validate (internal/component/bgp/plugins/rpki/validate.go). RFC6811-2-2 (no exclusion unless configured) and RFC6811-3-1 (match/set state in policy) over the operator-configurable per-state action buildDecisions honors (internal/component/bgp/plugins/rpki/rpki.go, default reject) -- invalid-action=reject excludes, accept/log-only retain the route marked Invalid, and the action is state-specific. RFC6811-4-1 (re-validate on VRP change) over the OriginTracker + handleROAChange path (internal/component/bgp/plugins/rpki/origin_tracker.go, rpki.go): a VRP mapping change re-validates tracked routes and re-dispatches decisions for those whose state flipped, and does nothing when no state changes. docs/features/rfc-status.md RFC 6811 row is Supported. The 2-3/2-4/2-5 SHOULDs and 2-6 MAY are not gated.

## Published support status

**Area:** RPKI origin validation

**Proof:** 1 of 5 MUST proven

**Evidence:** unit 1

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

**Positive and negative tests (5):** [`RFC6811-2-1`](#rfc6811-2-1), [`RFC6811-2-2`](#rfc6811-2-2), [`RFC6811-3-1`](#rfc6811-3-1), [`RFC6811-3-2`](#rfc6811-3-2), [`RFC6811-4-1`](#rfc6811-4-1)

## Figures

- **MUST bar**: Ze proven 1, Proof pending 4
- **Overview bar**: Ze MUST proven 1, Proof pending 4, Not assessed 4

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 5 |
| MUST rows kept (the MUST bar) | 5 |
| Ze proven | 1 |
| OS verified | 0 |
| Proof pending | 4 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 4 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 4 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc6811.json](../../../data/rfc/rfc6811.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6811-2-1` | The validation state of the Route MUST be set to reflect the result of the lookup. (Section 2) | MUST | 2 - Prefix-to-AS Mapping Database | **positive:** `unit/verify` [`TestBatchValidateTypedMatchesString`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rfc6811_rib_batch_test.go#L398). **positive:** `unit/verify` [`TestRPKIOriginASFromASPathRFC6811`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_test.go#L216). **positive:** `unit/verify` [`TestRouteStateIsSetFromTheLookup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_state_test.go#L31). **positive:** `unit/verify` [`TestValidateValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_test.go#L18). **negative:** `unit/verify` [`TestRPKIOriginASFromASPathRFC6811`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_test.go#L218). **negative:** `unit/verify` [`TestRouteStateIsSetFromTheLookup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_state_test.go#L53). **negative:** `unit/verify` [`TestValidateInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_test.go#L33) |
| `RFC6811-2-2` | An implementation MUST NOT exclude a route from the Adj-RIB-In or from consideration in the decision process as a side effect of its validation state, unless explicitly configured to do so (Section 2) | MUST NOT | 2 - Prefix-to-AS Mapping Database | **positive:** `unit/verify` [`TestBuildDecisionsOriginInvalidAction`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_rpki_batch_test.go#L105). **negative:** `unit/verify` [`TestBuildDecisionsOriginInvalidAction`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_rpki_batch_test.go#L107) |
| `RFC6811-3-1` | An implementation MUST provide the ability to match and set the validation state of routes as part of its route policy filtering function (Section 3) | MUST | 3 - Policy Control | **positive:** `unit/verify` [`TestBuildDecisionsOriginInvalidAction`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_rpki_batch_test.go#L110). **negative:** `unit/verify` [`TestBuildDecisionsOriginInvalidAction`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_rpki_batch_test.go#L113) |
| `RFC6811-3-2` | An implementation MUST also support four-octet AS numbers (Section 3) | MUST | 3 - Policy Control | **positive:** `unit/verify` [`TestValidateFourOctetAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_test.go#L94). **negative:** `unit/verify` [`TestValidateFourOctetAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_test.go#L96) |
| `RFC6811-4-1` | When a mapping is added or deleted, the implementation MUST re-validate any affected prefixes and run the BGP decision process if needed (Section 4) | MUST | 4 - Interaction with Local Cache | **positive:** `unit/verify` [`TestHandleROAChangeReValidates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_origin_tracker_test.go#L45). **positive:** `unit/verify` [`TestReValidationAppliesToInstalledRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rfc6811_rib_validation_test.go#L375). **negative:** `unit/verify` [`TestHandleROAChangeReValidates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_origin_tracker_test.go#L48). **negative:** `unit/verify` [`TestReValidationAppliesToInstalledRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rfc6811_rib_validation_test.go#L378) |
| `RFC6811-2-3` | When a BGP speaker receives an UPDATE from a neighbor, it SHOULD perform a lookup as described above for each of the Routes in the UPDATE message. (Section 2) | SHOULD | 2 - Prefix-to-AS Mapping Database | **positive:** no positive test. **negative:** no negative test |
| `RFC6811-2-4` | The lookup SHOULD also be applied to routes that are redistributed into BGP from another source, such as another protocol or a locally defined static route. (Section 2) | SHOULD | 2 - Prefix-to-AS Mapping Database | **positive:** no positive test. **negative:** no negative test |
| `RFC6811-2-5` | If validation is not performed on a Route, the implementation SHOULD initialize the validation state of such a route to "NotFound". (Section 2) | SHOULD | 2 - Prefix-to-AS Mapping Database | **positive:** no positive test. **negative:** no negative test |
| `RFC6811-2-6` | An implementation MAY provide configuration options to control which routes the lookup is applied to (Section 2) | MAY | 2 - Prefix-to-AS Mapping Database | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 6811 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Proof state

A test reads unproven when no record shows it failing after a deliberate fault. Until then, its claim to check the requirement remains unproven.

### [`RFC6811-2-1`](#rfc6811-2-1)

The validation state of the Route MUST be set to reflect the result of the lookup. (Section 2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRouteStateIsSetFromTheLookup drives validateNLRIs, the UPDATE path: for Valid, Invalid and NotFound lookups both the returned per-route state and the state carried by the validateCh request equal the lookup (and each is checked against the expected constant, so a shared wrong default goes red). Negative: after the VRP set changes Valid to Invalid, the route's state is Invalid, not the stale Valid.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRouteStateIsSetFromTheLookup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_state_test.go#L53) | unit/verify | revert, verified |
| negative | [`TestRPKIOriginASFromASPathRFC6811`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_test.go#L218) | unit/verify | unproven |
| negative | [`TestValidateInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_test.go#L33) | unit/verify | unproven |
| positive | [`TestBatchValidateTypedMatchesString`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rfc6811_rib_batch_test.go#L398) | unit/verify | unproven |
| positive | [`TestRouteStateIsSetFromTheLookup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_state_test.go#L31) | unit/verify | revert, verified |
| positive | [`TestRPKIOriginASFromASPathRFC6811`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_test.go#L216) | unit/verify | unproven |
| positive | [`TestValidateValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_test.go#L18) | unit/verify | unproven |

### [`RFC6811-2-2`](#rfc6811-2-2)

An implementation MUST NOT exclude a route from the Adj-RIB-In or from consideration in the decision process as a side effect of its validation state, unless explicitly configured to do so (Section 2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildDecisionsOriginInvalidAction`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_rpki_batch_test.go#L107) | unit/verify | unproven |
| positive | [`TestBuildDecisionsOriginInvalidAction`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_rpki_batch_test.go#L105) | unit/verify | unproven |

### [`RFC6811-3-1`](#rfc6811-3-1)

An implementation MUST provide the ability to match and set the validation state of routes as part of its route policy filtering function (Section 3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildDecisionsOriginInvalidAction`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_rpki_batch_test.go#L113) | unit/verify | unproven |
| positive | [`TestBuildDecisionsOriginInvalidAction`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_rpki_batch_test.go#L110) | unit/verify | unproven |

### [`RFC6811-3-2`](#rfc6811-3-2)

An implementation MUST also support four-octet AS numbers (Section 3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateFourOctetAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_test.go#L96) | unit/verify | unproven |
| positive | [`TestValidateFourOctetAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_validate_test.go#L94) | unit/verify | unproven |

### [`RFC6811-4-1`](#rfc6811-4-1)

When a mapping is added or deleted, the implementation MUST re-validate any affected prefixes and run the BGP decision process if needed (Section 4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReValidationAppliesToInstalledRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rfc6811_rib_validation_test.go#L378) | unit/verify | unproven |
| negative | [`TestHandleROAChangeReValidates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_origin_tracker_test.go#L48) | unit/verify | unproven |
| positive | [`TestReValidationAppliesToInstalledRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rfc6811_rib_validation_test.go#L375) | unit/verify | unproven |
| positive | [`TestHandleROAChangeReValidates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc6811_origin_tracker_test.go#L45) | unit/verify | unproven |

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
