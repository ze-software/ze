# RFC 7950 - The YANG 1.1 Data Modeling Language

No row in the public ledger. Every requirement this repository extracted from RFC 7950, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 4 of 4 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 4 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 4 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 4 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 18 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 9 | of 10 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 5 | of 9 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 4 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 10 |
| Gated MUST-level | 9 |
| Obligations that bind Ze | 4 |
| Not applicable, so out of scope | 5 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 18 |
| Tagged units | 18 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc7950.md` |
| Requirement shard | `rfc/requirements/rfc7950.md` |
| RFC text | `rfc/full/rfc7950.txt` |

## Enrolment

Enrolled: The YANG 1.1 Data Modeling Language (ze config validator): nine MUST-level requirements. Four are met with positive+negative tags in internal/component/config: 8.3.1-1 (a value violating a range, length, or pattern restriction is an error), 9.6-1 (an enum value must be one of the defined enums), 9.12-1 (a union value must match at least one member type), and 7.6.5-1 (a missing mandatory node is an error), with new length-violation and union-violation tests. Five are {not-applicable}: 7.5.3-1 (ze uses no must XPath statements and enforces cross-field constraints with Go validators instead), 9.2.4-1 (derived-type narrowing is resolved by goyang at module-processing time with no runtime surface), 9.4.5-1 (no ze type carries multiple pattern statements), x-1 (ze's models declare no yang-version and default to YANG 1.0, using no 1.1-only construct), and x-2 (no cross-revision position-stability tooling; critical enum values are pinned with explicit value statements).

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 7950.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated instead of tested | 5 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **9** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC7950-8.3.1-1`](#rfc7950-8.3.1-1), [`RFC7950-9.6-1`](#rfc7950-9.6-1), [`RFC7950-9.12-1`](#rfc7950-9.12-1), [`RFC7950-7.6.5-1`](#rfc7950-7.6.5-1)

**Annotated instead of tested (5):** [`RFC7950-7.5.3-1`](#rfc7950-7.5.3-1), [`RFC7950-9.2.4-1`](#rfc7950-9.2.4-1), [`RFC7950-9.4.5-1`](#rfc7950-9.4.5-1), [`RFC7950-x-1`](#rfc7950-x-1), [`RFC7950-x-2`](#rfc7950-x-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7950-8.3.1-1` | If a leaf data value does not match type constraints (range, length, pattern), server must reply with an invalid-value error (Section 8.3.1) | MUST | 8.3.1 | **positive:** `unit/verify` [`TestValidator_HoldTimeRange`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L833). **positive:** `unit/verify` [`TestValidator_ValidatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L784). **negative:** `unit/verify` [`TestValidateTree_LengthViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L351). **negative:** `unit/verify` [`TestValidateTree_PatternViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L250). **negative:** `unit/verify` [`TestValidateTree_RangeViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L162). **negative:** `unit/verify` [`TestValidator_HoldTimeRange`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L834). **negative:** `unit/verify` [`TestValidator_ValidatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L785) |
| `RFC7950-9.6-1` | An enumeration value must be one of the values specified in the type's enum statements (Section 9.6) | MUST | 9.6 | **positive:** `unit/verify` [`TestISISAuthAlgorithmEnumAcceptsAll`](https://github.com/ze-software/ze/blob/main/internal/component/config/isis_auth_algorithm_enum_test.go#L64). **positive:** `unit/verify` [`TestValidateTree_AddPathDirectionEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L552). **positive:** `unit/verify` [`TestValidateTree_FamilyModeEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L486). **negative:** `unit/verify` [`TestISISAuthAlgorithmEnumAcceptsAll`](https://github.com/ze-software/ze/blob/main/internal/component/config/isis_auth_algorithm_enum_test.go#L81). **negative:** `unit/verify` [`TestValidateTree_AddPathDirectionEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L553). **negative:** `unit/verify` [`TestValidateTree_FamilyModeEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L487) |
| `RFC7950-9.12-1` | A union value must match at least one member type (Section 9.12) | MUST | 9.12 | **positive:** `unit/verify` [`TestValidateTree_ValidConfig`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L39). **negative:** `unit/verify` [`TestValidateTree_UnionViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L300) |
| `RFC7950-7.6.5-1` | If a mandatory node does not exist, server must reply with a missing-element error (Section 7.6.5) | MUST | 7.6.5 | **positive:** `unit/verify` [`TestValidator_MandatoryField`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L868). **negative:** `unit/verify` [`TestValidateTree_MandatoryMissing`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L272). **negative:** `unit/verify` [`TestValidator_MandatoryField`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L869) |
| `RFC7950-7.5.3-1` | If a must expression evaluates to false, the data is not valid (Section 7.5.3) | MUST | 7.5.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze uses zero YANG must statements (a grep of the .yang models finds must only inside a description); it parses via goyang which evaluates no XPath, and enforces cross-field constraints with Go ze:validate functions (internal/component/config/yang/validator.go:756 applyCustomValidators) instead, so there is no must-XPath code path |
| `RFC7950-9.2.4-1` | All range, length, and pattern restrictions must be more restrictive than or equal to the base type's restrictions (Section 9.2.4) | MUST | 9.2.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this is a schema-authoring-time narrowing constraint resolved by goyang during module processing (internal/component/config/yang/loader.go); ze authors valid narrowings and has no runtime enforcement surface for it |
| `RFC7950-9.4.5-1` | Multiple pattern statements on the same type are combined as logical AND (Section 9.4.5) | MUST | 9.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no ze type carries two or more pattern statements (a scan of the .yang models finds none); the validator loop (internal/component/config/yang/validator.go:268) AND-combines patterns if present, but the multi-pattern construct is unused |
| `RFC7950-x-1` | YANG modules must declare yang-version 1.1 (Core Constructs) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's YANG models declare no yang-version statement and default to YANG 1.0 in goyang; they use no YANG-1.1-only construct that would require the 1.1 declaration, so the version-declaration obligation has no applicable module |
| `RFC7950-x-2` | Enum integer positions must not change across revisions (Type System) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's modules carry no consumed revision statements and there is no cross-revision position-stability tooling; ze pins the values that matter with explicit value statements (e.g. role.yang, ze-types.yang afi/safi) but enforces no automated cross-revision check |
| `RFC7950-7.6.4-1` | If a leaf has a default value and the leaf is not set, the default value should be used (Section 7.6.4) | SHOULD | 7.6.4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7950-7.5.3-1`](#rfc7950-7.5.3-1) If a must expression evaluates to false, the data is not valid (Section 7.5.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze uses zero YANG must statements (a grep of the .yang models finds must only inside a description); it parses via goyang which evaluates no XPath, and enforces cross-field constraints with Go ze:validate functions (internal/component/config/yang/validator.go:756 applyCustomValidators) instead, so there is no must-XPath code path |
| [`RFC7950-9.2.4-1`](#rfc7950-9.2.4-1) All range, length, and pattern restrictions must be more restrictive than or equal to the base type's restrictions (Section 9.2.4) | no test | no test carries this requirement id; annotated {not-applicable}: this is a schema-authoring-time narrowing constraint resolved by goyang during module processing (internal/component/config/yang/loader.go); ze authors valid narrowings and has no runtime enforcement surface for it |
| [`RFC7950-9.4.5-1`](#rfc7950-9.4.5-1) Multiple pattern statements on the same type are combined as logical AND (Section 9.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: no ze type carries two or more pattern statements (a scan of the .yang models finds none); the validator loop (internal/component/config/yang/validator.go:268) AND-combines patterns if present, but the multi-pattern construct is unused |
| [`RFC7950-x-1`](#rfc7950-x-1) YANG modules must declare yang-version 1.1 (Core Constructs) | no test | no test carries this requirement id; annotated {not-applicable}: ze's YANG models declare no yang-version statement and default to YANG 1.0 in goyang; they use no YANG-1.1-only construct that would require the 1.1 declaration, so the version-declaration obligation has no applicable module |
| [`RFC7950-x-2`](#rfc7950-x-2) Enum integer positions must not change across revisions (Type System) | no test | no test carries this requirement id; annotated {not-applicable}: ze's modules carry no consumed revision statements and there is no cross-revision position-stability tooling; ze pins the values that matter with explicit value statements (e.g. role.yang, ze-types.yang afi/safi) but enforces no automated cross-revision check |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7950-8.3.1-1`](#rfc7950-8.3.1-1)

If a leaf data value does not match type constraints (range, length, pattern), server must reply with an invalid-value error (Section 8.3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateTree_LengthViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L351) | unit/verify | unproven |
| negative | [`TestValidateTree_PatternViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L250) | unit/verify | unproven |
| negative | [`TestValidateTree_RangeViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L162) | unit/verify | unproven |
| negative | [`TestValidator_HoldTimeRange`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L834) | unit/verify | unproven |
| negative | [`TestValidator_ValidatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L785) | unit/verify | unproven |
| positive | [`TestValidator_HoldTimeRange`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L833) | unit/verify | unproven |
| positive | [`TestValidator_ValidatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L784) | unit/verify | unproven |

### [`RFC7950-9.6-1`](#rfc7950-9.6-1)

An enumeration value must be one of the values specified in the type's enum statements (Section 9.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISAuthAlgorithmEnumAcceptsAll`](https://github.com/ze-software/ze/blob/main/internal/component/config/isis_auth_algorithm_enum_test.go#L81) | unit/verify | unproven |
| negative | [`TestValidateTree_AddPathDirectionEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L553) | unit/verify | unproven |
| negative | [`TestValidateTree_FamilyModeEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L487) | unit/verify | unproven |
| positive | [`TestISISAuthAlgorithmEnumAcceptsAll`](https://github.com/ze-software/ze/blob/main/internal/component/config/isis_auth_algorithm_enum_test.go#L64) | unit/verify | unproven |
| positive | [`TestValidateTree_AddPathDirectionEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L552) | unit/verify | unproven |
| positive | [`TestValidateTree_FamilyModeEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L486) | unit/verify | unproven |

### [`RFC7950-9.12-1`](#rfc7950-9.12-1)

A union value must match at least one member type (Section 9.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateTree_UnionViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L300) | unit/verify | unproven |
| positive | [`TestValidateTree_ValidConfig`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L39) | unit/verify | unproven |

### [`RFC7950-7.6.5-1`](#rfc7950-7.6.5-1)

If a mandatory node does not exist, server must reply with a missing-element error (Section 7.6.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateTree_MandatoryMissing`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L272) | unit/verify | unproven |
| negative | [`TestValidator_MandatoryField`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L869) | unit/verify | unproven |
| positive | [`TestValidator_MandatoryField`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L868) | unit/verify | unproven |

### [`RFC7950-7.5.3-1`](#rfc7950-7.5.3-1)

If a must expression evaluates to false, the data is not valid (Section 7.5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.5.3-1, so no unit is bound to it.

### [`RFC7950-9.2.4-1`](#rfc7950-9.2.4-1)

All range, length, and pattern restrictions must be more restrictive than or equal to the base type's restrictions (Section 9.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.2.4-1, so no unit is bound to it.

### [`RFC7950-9.4.5-1`](#rfc7950-9.4.5-1)

Multiple pattern statements on the same type are combined as logical AND (Section 9.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.4.5-1, so no unit is bound to it.

### [`RFC7950-x-1`](#rfc7950-x-1)

YANG modules must declare yang-version 1.1 (Core Constructs)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-x-1, so no unit is bound to it.

### [`RFC7950-x-2`](#rfc7950-x-2)

Enum integer positions must not change across revisions (Type System)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-x-2, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 7950, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 7950, so its obligations are stated where they were written.
