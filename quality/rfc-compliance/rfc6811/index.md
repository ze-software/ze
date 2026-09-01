# RFC 6811 - BGP Prefix Origin Validation

Supported. Every requirement this repository extracted from RFC 6811, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 5 of 5 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 5 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 5 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 5 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 15 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 5 | of 9 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 5 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 5 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 9 |
| Gated MUST-level | 5 |
| Obligations that bind Ze | 5 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 15 |
| Tagged units | 15 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc6811.md` |
| Requirement shard | `rfc/requirements/rfc6811.md` |
| RFC text | `rfc/full/rfc6811.txt` |

## Enrolment

Enrolled: RPKI-based Prefix Origin Validation: all five MUST-level requirements tested with both polarities. RFC6811-2-1 (state reflects the lookup) and RFC6811-3-2 (four-octet AS) over ROACache.Validate (internal/component/bgp/plugins/rpki/validate.go). RFC6811-2-2 (no exclusion unless configured) and RFC6811-3-1 (match/set state in policy) over the operator-configurable per-state action buildDecisions honors (internal/component/bgp/plugins/rpki/rpki.go, default reject) -- invalid-action=reject excludes, accept/log-only retain the route marked Invalid, and the action is state-specific. RFC6811-4-1 (re-validate on VRP change) over the OriginTracker + handleROAChange path (internal/component/bgp/plugins/rpki/origin_tracker.go, rpki.go): a VRP mapping change re-validates tracked routes and re-dispatches decisions for those whose state flipped, and does nothing when no state changes. docs/features/rfc-status.md RFC 6811 row is Supported. The 2-3/2-4/2-5 SHOULDs and 2-6 MAY are not gated.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered**

Origin validation states (Valid/Invalid/NotFound) from ROA/VRP lookup with four-octet ASN support, RTR transport, an operator-configurable per-state action (invalid-action reject/log-only/accept, default reject) so exclusion of Invalid routes is an explicit policy choice, automatic re-validation of installed routes when the VRP set changes, and a fail-open guard.

**What the ledger says remains:**

No tracked gap in current source anchors.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 5 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **5** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (5):** [`RFC6811-2-1`](#rfc6811-2-1), [`RFC6811-2-2`](#rfc6811-2-2), [`RFC6811-3-1`](#rfc6811-3-1), [`RFC6811-3-2`](#rfc6811-3-2), [`RFC6811-4-1`](#rfc6811-4-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6811-2-1` | Validation state of the Route MUST be set to reflect the result of the lookup (Section 2) | MUST | 2 - Prefix-to-AS Mapping Database | **positive:** `unit/verify` [`TestBatchValidateTypedMatchesString`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rib_batch_test.go#L398). **positive:** `unit/verify` [`TestRPKIOriginASFromASPathRFC6811`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/validate_test.go#L225). **positive:** `unit/verify` [`TestValidateValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/validate_test.go#L18). **negative:** `unit/verify` [`TestRPKIOriginASFromASPathRFC6811`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/validate_test.go#L227). **negative:** `unit/verify` [`TestValidateInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/validate_test.go#L33) |
| `RFC6811-2-2` | An implementation MUST NOT exclude a route from the Adj-RIB-In or from consideration in the decision process as a side effect of its validation state, unless explicitly configured to do so (Section 2) | MUST NOT | 2 - Prefix-to-AS Mapping Database | **positive:** `unit/verify` [`TestBuildDecisionsOriginInvalidAction`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rpki_batch_test.go#L105). **negative:** `unit/verify` [`TestBuildDecisionsOriginInvalidAction`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rpki_batch_test.go#L107) |
| `RFC6811-3-1` | An implementation MUST provide the ability to match and set the validation state of routes as part of its route policy filtering function (Section 3) | MUST | 3 - Policy Control | **positive:** `unit/verify` [`TestBuildDecisionsOriginInvalidAction`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rpki_batch_test.go#L110). **negative:** `unit/verify` [`TestBuildDecisionsOriginInvalidAction`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rpki_batch_test.go#L113) |
| `RFC6811-3-2` | An implementation MUST also support four-octet AS numbers (Section 3) | MUST | 3 - Policy Control | **positive:** `unit/verify` [`TestValidateFourOctetAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/validate_test.go#L94). **negative:** `unit/verify` [`TestValidateFourOctetAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/validate_test.go#L96) |
| `RFC6811-4-1` | When a mapping is added or deleted, the implementation MUST re-validate any affected prefixes and run the BGP decision process if needed (Section 4) | MUST | 4 - Interaction with Local Cache | **positive:** `unit/verify` [`TestHandleROAChangeReValidates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/origin_tracker_test.go#L42). **positive:** `unit/verify` [`TestReValidationAppliesToInstalledRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rib_validation_test.go#L390). **negative:** `unit/verify` [`TestHandleROAChangeReValidates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/origin_tracker_test.go#L45). **negative:** `unit/verify` [`TestReValidationAppliesToInstalledRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rib_validation_test.go#L393) |
| `RFC6811-2-3` | When receiving an UPDATE, the implementation SHOULD perform a validation lookup for each Route in the message (Section 2) | SHOULD | 2 - Prefix-to-AS Mapping Database | **positive:** no positive test. **negative:** no negative test |
| `RFC6811-2-4` | The lookup SHOULD also be applied to routes redistributed into BGP from other sources (Section 2) | SHOULD | 2 - Prefix-to-AS Mapping Database | **positive:** no positive test. **negative:** no negative test |
| `RFC6811-2-5` | If validation is not performed on a Route, the implementation SHOULD initialize the validation state to "NotFound" (Section 2) | SHOULD | 2 - Prefix-to-AS Mapping Database | **positive:** no positive test. **negative:** no negative test |
| `RFC6811-2-6` | An implementation MAY provide configuration options to control which routes the lookup is applied to (Section 2) | MAY | 2 - Prefix-to-AS Mapping Database | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 6811 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC6811-2-1`](#rfc6811-2-1)

Validation state of the Route MUST be set to reflect the result of the lookup (Section 2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRPKIOriginASFromASPathRFC6811`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/validate_test.go#L227) | unit/verify | unproven |
| negative | [`TestValidateInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/validate_test.go#L33) | unit/verify | unproven |
| positive | [`TestBatchValidateTypedMatchesString`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rib_batch_test.go#L398) | unit/verify | unproven |
| positive | [`TestRPKIOriginASFromASPathRFC6811`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/validate_test.go#L225) | unit/verify | unproven |
| positive | [`TestValidateValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/validate_test.go#L18) | unit/verify | unproven |

### [`RFC6811-2-2`](#rfc6811-2-2)

An implementation MUST NOT exclude a route from the Adj-RIB-In or from consideration in the decision process as a side effect of its validation state, unless explicitly configured to do so (Section 2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildDecisionsOriginInvalidAction`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rpki_batch_test.go#L107) | unit/verify | unproven |
| positive | [`TestBuildDecisionsOriginInvalidAction`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rpki_batch_test.go#L105) | unit/verify | unproven |

### [`RFC6811-3-1`](#rfc6811-3-1)

An implementation MUST provide the ability to match and set the validation state of routes as part of its route policy filtering function (Section 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildDecisionsOriginInvalidAction`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rpki_batch_test.go#L113) | unit/verify | unproven |
| positive | [`TestBuildDecisionsOriginInvalidAction`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rpki_batch_test.go#L110) | unit/verify | unproven |

### [`RFC6811-3-2`](#rfc6811-3-2)

An implementation MUST also support four-octet AS numbers (Section 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateFourOctetAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/validate_test.go#L96) | unit/verify | unproven |
| positive | [`TestValidateFourOctetAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/validate_test.go#L94) | unit/verify | unproven |

### [`RFC6811-4-1`](#rfc6811-4-1)

When a mapping is added or deleted, the implementation MUST re-validate any affected prefixes and run the BGP decision process if needed (Section 4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReValidationAppliesToInstalledRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rib_validation_test.go#L393) | unit/verify | unproven |
| negative | [`TestHandleROAChangeReValidates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/origin_tracker_test.go#L45) | unit/verify | unproven |
| positive | [`TestReValidationAppliesToInstalledRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rib_validation_test.go#L390) | unit/verify | unproven |
| positive | [`TestHandleROAChangeReValidates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/origin_tracker_test.go#L42) | unit/verify | unproven |

## Extraction sign-off

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
| `1` | Introduction | 0 | walked | Introduction. Indicative prose: the threat the document addresses, the three components of the RPKI architecture, that the cache retrieval protocol is out of scope (RFC 6810), that full AS_PATH attestation is out of scope, and that the global RPKI is only loosely consistent across caches. Its only modal is the lowercase 'The cache must also be refreshed periodically', which section 1.1 gives no special meaning. No sentence directs a speaker. |
| `1.1` | Requirements Language | 0 | walked | Requirements Language. The RFC 2119 key-words paragraph, which also states that the key words bind only when they appear in all upper case, and that is what disqualifies the lowercase modals in sections 1, 5 and 6. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. |
| `2` | Prefix-to-AS Mapping Database | 2 | walked | Prefix-to-AS Mapping Database. The document's main normative section. It defines VRP, Prefix, Route, VRP Prefix, VRP ASN, Route Prefix, Route Origin ASN, Covered and Matched, then the three validation states NotFound, Valid and Invalid, then states the two capitalised MUST-level sites mapped below to RFC6811-2-1 and RFC6811-2-2. Its remaining directives are three SHOULDs and one MAY, which are the unsourced ids below: perform the lookup for each Route in a received UPDATE, apply it to redistributed routes, initialize an unvalidated route to NotFound, and offer configuration options controlling which routes the lookup applies to. The definitions are indicative, so under section 1.1 they carry no RFC 2119 level. "A Route Prefix is said to be Covered by a VRP when the VRP prefix length is less than or equal to the Route prefix length" names a term the lookup uses rather than directing a speaker, and the same holds for Matched and for the three states. Two further indicative sentences are observations that scope the lookup rather than adding to it: a Route whose Origin ASN is "NONE" cannot be Matched by any VRP, and a Route can be Covered or Matched by more than one VRP while the state output stays fully determined. |
| `2.1` | Pseudo-Code | 0 | walked | Pseudo-Code. A C-like listing of the section 2 procedure. The section opens by declaring itself subordinate: "In case of ambiguity, the procedure above, rather than the pseudo-code, should be taken as authoritative." It therefore adds no obligation. Its constant names BGP_PFXV_STATE_NOT_FOUND, BGP_PFXV_STATE_VALID and BGP_PFXV_STATE_INVALID are carried by the Constants table of rfc/short/rfc6811.md. |
| `3` | Policy Control | 2 | walked | Policy Control. Two capitalised MUST-level sites, mapped below to RFC6811-3-1 and RFC6811-3-2. Its two remaining sentences point the reader at section 5 and at [ORIGIN-OPS] for operational policy guidance, and neither directs a speaker. |
| `4` | Interaction with Local Cache | 1 | walked | Interaction with Local Cache. One capitalised MUST-level site, mapped below to RFC6811-4-1. Its remaining sentences state that a speaker is expected to talk to one or more RPKI caches over the RFC 6810 protocol, and define an "affected prefix" as any prefix that was matched by a deleted or updated mapping, or could be matched by an added or updated mapping. That definition scopes RFC6811-4-1 rather than adding a requirement, and it is indicative. |
| `5` | Deployment Considerations | 0 | walked | Deployment Considerations. Names policies an operator can implement, filtering Invalid routes and adjusting LOCAL_PREF among them, and observes that propagating the validation state to an IBGP peer through a community can be necessary for routing correctness. Its modals are all lowercase, "should be done with utmost care" and "it could be necessary", which section 1.1 gives no special meaning. No sentence directs a speaker. |
| `6` | Security Considerations | 0 | walked | Security Considerations. States that the security of the whole rests on the database, that blocking "invalid" routes opens a denial-of-service vector when an attacker can inject or remove validation records, that an attacker defeats the mechanism by prepending the authorized origin AS to a forged announcement, and that no path validation is provided. Its one lowercase "must be considered" addresses the reader assessing the overall solution rather than a speaker, and no countermeasure is directed at a speaker. |
| `7` | Acknowledgments | 0 | skipped (acknowledgements) | Acknowledgments. |
| `8` | References | 0 | skipped (references) | References. The section header only; its entries are in sections 8.1 and 8.2. |
| `8.1` | not stated | 0 | skipped (references) | Normative References: RFC 2119, RFC 3779, RFC 4271, RFC 4632, RFC 6482, RFC 6793. |
| `8.2` | not stated | 0 | skipped (references) | Informational References: [AS0], [ORIGIN-OPS], RFC 6480, RFC 6810. |

### Excluded sentences

The walk over RFC 6811 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 6811, so its obligations are stated where they were written.
