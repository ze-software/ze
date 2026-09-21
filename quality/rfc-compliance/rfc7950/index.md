# RFC 7950 - The YANG 1.1 Data Modeling Language

No row in the public ledger. Every requirement this repository extracted from RFC 7950, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 5.3% | 4 of 76 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 76 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 76 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 10.0% | 2 of 20 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 76 | of 77 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 5 | of 76 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 6.6% | 5 of 76 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 76 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 76 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 88.2% | 67 of 76 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 76 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 77 |
| Gated MUST-level | 76 |
| Not applicable, so out of scope | 5 |
| Declared gaps | 0 |
| Gated with no test | 67 |
| Nightly-only evidence | 0 |
| Test tags | 20 |
| Tagged units | 20 |
| Recorded audit verdicts | 0 |
| Discrimination records | 2 |
| Summary | `rfc/short/rfc7950.md` |
| Requirement shard | `rfc/requirements/rfc7950.md` |
| RFC text | `rfc/full/rfc7950.txt` |

## Enrolment

Enrolled: The YANG 1.1 Data Modeling Language (ze config validator). The extraction walk of 2026-09-21 read all 227 MUST sites and rewrote this ledger: it added 67 MUST rows the checklist did not carry, covering module and submodule structure, identifier and prefix scoping, typedef and leaf definition rules, list keys and unique constraints, choice, grouping, augment, identity, feature, deviation, status and when rules, the Section 8.1 validity conditions and the Section 8.3 enforcement windows, the built-in type restrictions of Section 9, the module update rules of Section 11 and the YANG 1/1.1 coexistence rules of Section 12. None of the added rows carries a tagged test, so each is an untested MUST on this ledger. Of the nine rows that stood before the walk, four are met with positive+negative tags in internal/component/config: 8.3.1-1 (a value violating a range, length, or pattern restriction is an error), 9.6-1 (an enum value must be one of the defined enums), 9.12-1 (a union value must match at least one member type), and 7.6.5-1 (a missing mandatory node is an error). Five are {not-applicable}: 7.5.3-1 (ze uses no must XPath statements and enforces cross-field constraints with Go validators instead), 9.2.4-1 (derived-type narrowing is resolved by goyang at module-processing time with no runtime surface), 9.4.5-1 (no ze type carries multiple pattern statements), x-1 (ze's models declare no yang-version and default to YANG 1.0, using no 1.1-only construct), and x-2 (no cross-revision position-stability tooling; critical enum values are pinned with explicit value statements). The walk excluded 32 sites as the NETCONF protocol binding, the XML encoding and the YIN syntax, none of which ze uses: ze serves its YANG-modeled config over gNMI and its own CLI, and no Go file in internal/ references NETCONF except a port-name table. Three further sites delegate module-name and namespace assignment to the IANA procedure of the earlier YANG specification they cite.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 7950.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated instead of tested | 5 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 67 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **76** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC7950-8.3.1-1`](#rfc7950-8.3.1-1), [`RFC7950-9.6-1`](#rfc7950-9.6-1), [`RFC7950-9.12-1`](#rfc7950-9.12-1), [`RFC7950-7.6.5-1`](#rfc7950-7.6.5-1)

**Annotated instead of tested (5):** [`RFC7950-7.5.3-1`](#rfc7950-7.5.3-1), [`RFC7950-9.2.4-1`](#rfc7950-9.2.4-1), [`RFC7950-9.4.5-1`](#rfc7950-9.4.5-1), [`RFC7950-x-1`](#rfc7950-x-1), [`RFC7950-x-2`](#rfc7950-x-2)

**No test and no annotation (67):** [`RFC7950-5.1-1`](#rfc7950-5.1-1), [`RFC7950-5.3-1`](#rfc7950-5.3-1), [`RFC7950-5.5-1`](#rfc7950-5.5-1), [`RFC7950-5.6.5-1`](#rfc7950-5.6.5-1), [`RFC7950-6.1.3-1`](#rfc7950-6.1.3-1), [`RFC7950-6.2-1`](#rfc7950-6.2-1), [`RFC7950-6.2.1-1`](#rfc7950-6.2.1-1), [`RFC7950-6.3.1-1`](#rfc7950-6.3.1-1), [`RFC7950-6.4-1`](#rfc7950-6.4-1), [`RFC7950-6.4-2`](#rfc7950-6.4-2), [`RFC7950-6.5-1`](#rfc7950-6.5-1), [`RFC7950-7.1.4-1`](#rfc7950-7.1.4-1), [`RFC7950-7.3-1`](#rfc7950-7.3-1), [`RFC7950-7.3.4-1`](#rfc7950-7.3.4-1), [`RFC7950-7.6.1-1`](#rfc7950-7.6.1-1), [`RFC7950-7.6.3-1`](#rfc7950-7.6.3-1), [`RFC7950-7.6.4-2`](#rfc7950-7.6.4-2), [`RFC7950-7.7-1`](#rfc7950-7.7-1), [`RFC7950-7.7.2-1`](#rfc7950-7.7.2-1), [`RFC7950-7.7.4-1`](#rfc7950-7.7.4-1), [`RFC7950-7.7.5-1`](#rfc7950-7.7.5-1), [`RFC7950-7.8.2-1`](#rfc7950-7.8.2-1), [`RFC7950-7.8.3-1`](#rfc7950-7.8.3-1), [`RFC7950-7.9.2-1`](#rfc7950-7.9.2-1), [`RFC7950-7.9.3-1`](#rfc7950-7.9.3-1), [`RFC7950-7.9.4-1`](#rfc7950-7.9.4-1), [`RFC7950-7.12-1`](#rfc7950-7.12-1), [`RFC7950-7.14.2-1`](#rfc7950-7.14.2-1), [`RFC7950-7.14.3-1`](#rfc7950-7.14.3-1), [`RFC7950-7.15-1`](#rfc7950-7.15-1), [`RFC7950-7.16-1`](#rfc7950-7.16-1), [`RFC7950-7.17-1`](#rfc7950-7.17-1), [`RFC7950-7.18.2-1`](#rfc7950-7.18.2-1), [`RFC7950-7.19-1`](#rfc7950-7.19-1), [`RFC7950-7.20.1-1`](#rfc7950-7.20.1-1), [`RFC7950-7.20.2-1`](#rfc7950-7.20.2-1), [`RFC7950-7.20.3-1`](#rfc7950-7.20.3-1), [`RFC7950-7.20.3.2-1`](#rfc7950-7.20.3.2-1), [`RFC7950-7.21.2-1`](#rfc7950-7.21.2-1), [`RFC7950-7.21.5-1`](#rfc7950-7.21.5-1), [`RFC7950-8.1-1`](#rfc7950-8.1-1), [`RFC7950-8.1-2`](#rfc7950-8.1-2), [`RFC7950-8.3-1`](#rfc7950-8.3-1), [`RFC7950-8.3.1-2`](#rfc7950-8.3.1-2), [`RFC7950-8.3.2-1`](#rfc7950-8.3.2-1), [`RFC7950-8.3.3-1`](#rfc7950-8.3.3-1), [`RFC7950-9.1-1`](#rfc7950-9.1-1), [`RFC7950-9.2.4-2`](#rfc7950-9.2.4-2), [`RFC7950-9.3.2-1`](#rfc7950-9.3.2-1), [`RFC7950-9.3.4-1`](#rfc7950-9.3.4-1), [`RFC7950-9.4.4-1`](#rfc7950-9.4.4-1), [`RFC7950-9.5.1-1`](#rfc7950-9.5.1-1), [`RFC7950-9.6.4-1`](#rfc7950-9.6.4-1), [`RFC7950-9.6.4-2`](#rfc7950-9.6.4-2), [`RFC7950-9.6.4.2-1`](#rfc7950-9.6.4.2-1), [`RFC7950-9.7-1`](#rfc7950-9.7-1), [`RFC7950-9.7.4-1`](#rfc7950-9.7.4-1), [`RFC7950-9.7.4.2-1`](#rfc7950-9.7.4.2-1), [`RFC7950-9.9-1`](#rfc7950-9.9-1), [`RFC7950-9.9.2-1`](#rfc7950-9.9.2-1), [`RFC7950-9.9.3-1`](#rfc7950-9.9.3-1), [`RFC7950-9.10.2-1`](#rfc7950-9.10.2-1), [`RFC7950-9.10.3-1`](#rfc7950-9.10.3-1), [`RFC7950-9.12-2`](#rfc7950-9.12-2), [`RFC7950-9.13-1`](#rfc7950-9.13-1), [`RFC7950-11-1`](#rfc7950-11-1), [`RFC7950-12-1`](#rfc7950-12-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7950-8.3.1-1` | If a leaf data value does not match type constraints (range, length, pattern), server must reply with an invalid-value error (Section 8.3.1) | MUST | 8.3.1 | **positive:** `unit/verify` [`TestValidator_HoldTimeRange`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L893). **positive:** `unit/verify` [`TestValidator_ValidatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L844). **negative:** `unit/verify` [`TestValidateTree_LengthViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L411). **negative:** `unit/verify` [`TestValidateTree_PatternViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L310). **negative:** `unit/verify` [`TestValidateTree_RangeViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L169). **negative:** `unit/verify` [`TestValidator_HoldTimeRange`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L894). **negative:** `unit/verify` [`TestValidator_ValidatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L845) |
| `RFC7950-9.6-1` | An enumeration value must be one of the values specified in the type's enum statements (Section 9.6) | MUST | 9.6 | **positive:** `unit/verify` [`TestISISAuthAlgorithmEnumAcceptsAll`](https://github.com/ze-software/ze/blob/main/internal/component/config/isis_auth_algorithm_enum_test.go#L64). **positive:** `unit/verify` [`TestRadiusAuthMethodEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/radius_auth_method_enum_test.go#L53). **positive:** `unit/verify` [`TestValidateTree_AddPathDirectionEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L612). **positive:** `unit/verify` [`TestValidateTree_FamilyModeEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L546). **negative:** `unit/verify` [`TestISISAuthAlgorithmEnumAcceptsAll`](https://github.com/ze-software/ze/blob/main/internal/component/config/isis_auth_algorithm_enum_test.go#L81). **negative:** `unit/verify` [`TestRadiusAuthMethodEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/radius_auth_method_enum_test.go#L64). **negative:** `unit/verify` [`TestValidateTree_AddPathDirectionEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L613). **negative:** `unit/verify` [`TestValidateTree_FamilyModeEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L547) |
| `RFC7950-9.12-1` | A union value must match at least one member type (Section 9.12) | MUST | 9.12 | **positive:** `unit/verify` [`TestValidateTree_ValidConfig`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L46). **negative:** `unit/verify` [`TestValidateTree_UnionViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L360) |
| `RFC7950-7.6.5-1` | If a mandatory node does not exist, server must reply with a missing-element error (Section 7.6.5) | MUST | 7.6.5 | **positive:** `unit/verify` [`TestValidator_MandatoryField`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L928). **negative:** `unit/verify` [`TestValidateTree_MandatoryMissing`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L332). **negative:** `unit/verify` [`TestValidator_MandatoryField`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L929) |
| `RFC7950-7.5.3-1` | If a must expression evaluates to false, the data is not valid (Section 7.5.3) | MUST | 7.5.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze uses zero YANG must statements (a grep of the .yang models finds must only inside a description); it parses via goyang which evaluates no XPath, and enforces cross-field constraints with Go ze:validate functions (internal/component/config/yang/validator.go:756 applyCustomValidators) instead, so there is no must-XPath code path |
| `RFC7950-9.2.4-1` | All range, length, and pattern restrictions must be more restrictive than or equal to the base type's restrictions (Section 9.2.4) | MUST | 9.2.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this is a schema-authoring-time narrowing constraint resolved by goyang during module processing (internal/component/config/yang/loader.go); ze authors valid narrowings and has no runtime enforcement surface for it |
| `RFC7950-9.4.5-1` | Multiple pattern statements on the same type are combined as logical AND (Section 9.4.5) | MUST | 9.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no ze type carries two or more pattern statements (a scan of the .yang models finds none); the validator loop (internal/component/config/yang/validator.go:268) AND-combines patterns if present, but the multi-pattern construct is unused |
| `RFC7950-x-1` | YANG modules must declare yang-version 1.1 (Core Constructs) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's YANG models declare no yang-version statement and default to YANG 1.0 in goyang; they use no YANG-1.1-only construct that would require the 1.1 declaration, so the version-declaration obligation has no applicable module |
| `RFC7950-x-2` | Enum integer positions must not change across revisions (Type System) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's modules carry no consumed revision statements and there is no cross-revision position-stability tooling; ze pins the values that matter with explicit value statements (e.g. role.yang, ze-types.yang afi/safi) but enforces no automated cross-revision check |
| `RFC7950-5.1-1` | Module names are unique within a server; a module includes all its submodules; a submodule is included only by the module it belongs to or by another submodule of that module, never imports its own module, and never includes a submodule of another module; a submodule includes the same submodule revisions its module includes and no submodule is included at two revisions; an external module is imported before its definitions are referenced, with no circular chains of imports; a reference to an external definition uses a locally defined prefix followed by a colon (Section 5.1, Section 7.1.6, Section 7.2.2) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-5.3-1` | Choose namespace URIs so they cannot collide with standard or other enterprise namespaces, for example by using the enterprise or organization name in the namespace (Section 5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-5.5-1` | A scoped definition does not shadow a definition at a higher scope (Section 5.5) | MUST | 5.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-5.6.5-1` | Implement no more than one revision of a module, and where a supported augment or path statement uses a node from an imported module, implement a revision of that module that carries the node (Section 5.6.5) | MUST | 5.6.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-6.1.3-1` | In a double-quoted string a backslash is followed only by one of the characters the escape rules define (Section 6.1.3) | MUST | 6.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-6.2-1` | Support identifiers up to 64 characters in length (Section 6.2) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-6.2.1-1` | All identifiers defined in one namespace are unique: a descendant node defines no typedef and no grouping whose name is already visible from an ancestor (Section 6.2.1) | MUST | 6.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-6.3.1-1` | Qualify an extension keyword with the prefix of the module that defines it, including inside that module, and process a supported extension in accordance with the specification governing it (Section 6.3.1) | MUST | 6.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-6.4-1` | Enforce the requirements the data model encodes, whether or not an XPath interpreter is implemented (Section 6.4) | MUST | 6.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-6.4-2` | XPath expressions are syntactically correct and every prefix they use is present in the XPath context (Section 6.4) | MUST | 6.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-6.5-1` | Qualify a reference to an identifier defined in an external module with the appropriate prefix (Section 6.5) | MUST | 6.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.1.4-1` | All prefixes, the module's own included, are unique within the module or submodule, and where two imported modules define the same prefix at least one is imported under a different prefix (Section 7.1.4) | MUST | 7.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.3-1` | A typedef's argument is followed by a block of substatements, its type substatement is present, its name is not one of the YANG built-in types, and a top-level typedef name is unique within the module (Section 7.3, Section 7.3.2) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.3.4-1` | A typedef's default value is valid according to its type, and a derived type or leaf whose restrictions invalidate the inherited default specifies a new compatible default (Section 7.3.4) | MUST | 7.3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.6.1-1` | Use a leaf's default value when the leaf is not set and its ancestry allows it, and behave operationally as if the leaf were present in the data tree with that value (Section 7.6.1) | MUST | 7.6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.6.3-1` | A leaf's type statement is present (Section 7.6.3) | MUST | 7.6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.6.4-2` | A leaf's default value is valid according to the leaf's type, is absent where mandatory is true, and is not marked with an if-feature statement (Section 7.6.4) | MUST | 7.6.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.7-1` | In configuration data the values in a leaf-list are unique, the definitions of default values carry no if-feature statement, and the values in the data tree are in canonical form (Section 7.7) | MUST | 7.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.7.2-1` | Use a leaf-list's default values when it is not set and its ancestry allows it, and behave operationally as if the leaf-list were present in the data tree with those values (Section 7.7.2) | MUST | 7.7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.7.4-1` | A leaf-list's default value is valid according to its type and is absent where min-elements is one or more (Section 7.7.4) | MUST | 7.7.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.7.5-1` | A valid leaf-list or list has at least min-elements entries (Section 7.7.5) | MUST | 7.7.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.8.2-1` | A list that represents configuration carries a key statement; each key leaf identifier appears once, refers to a child leaf of the list, is given a value when a list entry is created, and has the same config value as the list (Section 7.8.2) | MUST | 7.8.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.8.3-1` | A unique argument names descendant-form schema node identifiers that each refer to a leaf; where one referenced leaf represents configuration all of them do, and the combined values are unique across all list entries in which every referenced leaf exists (Section 7.8.3) | MUST | 7.8.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.9.2-1` | Child node identifiers are unique across all cases of a choice, a case identifier is unique within its choice, and a schema node identifier always includes the case node identifier (Section 7.9.2) | MUST | 7.9.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.9.3-1` | A choice carries no default statement where mandatory is true, and the default case holds no mandatory node (Section 7.9.3) | MUST | 7.9.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.9.4-1` | Where a choice is mandatory, at least one node from exactly one of its case branches exists (Section 7.9.4) | MUST | 7.9.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.12-1` | A grouping never references itself, directly or through a chain of groupings, and a top-level grouping identifier is unique within the module (Section 7.12) | MUST | 7.12 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.14.2-1` | In an RPC or action invocation a mandatory input leaf is present, the server uses the input defaults in the cases Sections 7.6.1 and 7.7.2 describe and behaves as if the defaulted node were present, and no node whose when statement evaluates to false is present (Section 7.14.2) | MUST | 7.14.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.14.3-1` | In an RPC or action reply a mandatory output leaf is present, the client uses the output defaults in the cases Sections 7.6.1 and 7.7.2 describe and behaves as if the defaulted node were present, and no node whose when statement evaluates to false is present (Section 7.14.3) | MUST | 7.14.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.15-1` | An action is not defined within an rpc, another action or a notification, and has no ancestor node that is a list without a key statement (Section 7.15) | MUST | 7.15 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.16-1` | A notification is not defined within an rpc, an action or another notification, has no ancestor node that is a list without a key statement, carries its mandatory leafs, and its receiver uses the notification's default values in the cases Sections 7.6.1 and 7.7.2 describe (Section 7.16) | MUST | 7.16 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.17-1` | An augment target is a container, list, choice, case, input, output or notification node; a top-level augment uses the absolute form of a schema node identifier and an augment under uses the descendant form; an augment adds no two nodes of the same name from the same module to one target; and an augment that adds mandatory configuration nodes to another module's target is made conditional with a when statement (Section 7.17) | MUST | 7.17 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.18.2-1` | A base argument names an identity defined in the current module or an included submodule, and an identity never references itself, directly or through a chain of identities (Section 7.18.2) | MUST | 7.18.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.19-1` | The substatements of an extension usage are YANG statements, extensions included, and follow the syntactical rules of Section 14 (Section 7.19) | MUST | 7.19 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.20.1-1` | A feature never references itself, and a server that supports a feature supports every feature that feature depends on (Section 7.20.1) | MUST | 7.20.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.20.2-1` | An if-feature argument names a feature defined in the current module or an included submodule, and a leaf that is a list key carries no if-feature statement (Section 7.20.2) | MUST | 7.20.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.20.3-1` | A deviation is never part of a published standard, a server deviation is used only as a last resort, and the data model that results from applying all of a server's deviations in any order is still valid (Section 7.20.3) | MUST | 7.20.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.20.3.2-1` | A deviate add does not add a property that can appear only once and already exists in the target node; the properties a deviate replace names exist in the target node; and a deviate delete substatement matches the target node's keyword and argument string (Section 7.20.3.2) | MUST | 7.20.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.21.2-1` | A current definition references no deprecated or obsolete definition within the same module, and a deprecated definition references no obsolete definition within the same module (Section 7.21.2) | MUST | 7.21.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.21.5-1` | A leaf that is a list key carries no when statement and neither does a uses statement that brings a key leaf into a list; when expressions on referenced nodes are evaluated first; and there are no circular dependencies among when expressions (Section 7.21.5) | MUST | 7.21.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-8.1-1` | A valid data tree satisfies every Section 8.1 constraint: every leaf value matches its type constraints including range, length and pattern; all key leafs are present for every list entry; nodes are present for at most one case branch of a choice; no node tagged with an if-feature whose expression is false and no node tagged with a when whose condition is false is present; every path referential-integrity constraint is satisfied; every unique constraint is satisfied; mandatory is enforced for leafs and choices; and the constraint holds for configuration data, state data, notification content, and RPC or action input and output (Section 8.1) | MUST | 8.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-8.1-2` | The running configuration datastore is always valid (Section 8.1) | MUST | 8.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-8.3-1` | Enforce configuration constraints in each of the three windows Section 8.3 defines (Section 8.3) | MUST | 8.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-8.3.1-2` | Reject content that carries data for more than one case branch of a choice, or data for a node whose if-feature expression or when condition evaluates to false, answering with the error the management protocol defines (a bad-element or unknown-element error-tag under NETCONF) (Section 8.3.1) | MUST | 8.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-8.3.2-1` | While processing a datastore modification, detect data for a node whose if-feature expression or when condition evaluates to false and reject it (Section 8.3.2) | MUST | 8.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-8.3.3-1` | When datastore processing is complete the final contents obey every validation constraint, enforced at the end of the operation for the running and startup datastores (Section 8.3.3) | MUST | 8.3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-9.1-1` | Support all the lexical representations this specification defines, and where a type has no canonical form the value's format matches the type's lexical representation (Section 9.1) | MUST | 9.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-9.2.4-2` | The values and ranges of a range restriction are disjoint and in ascending order, and every explicit value and range boundary matches the type being restricted or is one of the special values min and max (Section 9.2.4) | MUST | 9.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-9.3.2-1` | A decimal64 value carries at least one digit before and after the decimal point, with no leading or trailing zeros (Section 9.3.2) | MUST | 9.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-9.3.4-1` | The fraction-digits statement is present where the type is decimal64 (Section 9.3.4) | MUST | 9.3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-9.4.4-1` | Length-restricting values are never negative, and the values and ranges of a length restriction are disjoint and in ascending order (Section 9.4.4) | MUST | 9.4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-9.5.1-1` | Represent a boolean value as the lowercase string true or false (Section 9.5.1) | MUST | 9.5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-9.6.4-1` | The enum statement is present where the type is enumeration; an assigned name is not zero-length and carries no leading or trailing whitespace; and all assigned names in an enumeration are unique (Section 9.6.4) | MUST | 9.6.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-9.6.4-2` | When an existing enumeration type is restricted, the new type's assigned names are a subset of the base type's and the value of an assigned name is not changed (Section 9.6.4) | MUST | 9.6.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-9.6.4.2-1` | An enum value is in the range -2147483648 to 2147483647 and unique within the enumeration type; once the highest value reaches 2147483647 a value is given explicitly for the enum substatements that follow; and a restricted enumeration either repeats the base type's value or omits the value statement (Section 9.6.4.2) | MUST | 9.6.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-9.7-1` | When an existing bits type is restricted, the new type's assigned names are a subset of the base type's and the bit position of an assigned name is not changed (Section 9.7) | MUST | 9.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-9.7.4-1` | The bit statement is present where the type is bits, and all assigned names in a bits type are unique (Section 9.7.4) | MUST | 9.7.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-9.7.4.2-1` | A bit position is in the range 0 to 4294967295 and unique within the bits type; once the highest position reaches 4294967295 a position is given explicitly for the bit substatements that follow; and a restricted bits type either repeats the base type's position or omits the position statement (Section 9.7.4.2) | MUST | 9.7.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-9.9-1` | Where require-instance is true the node a leafref refers to exists; a leafref that represents configuration refers to configuration; there are no circular chains of leafrefs; and a leafref to a feature-conditional leaf is itself conditional on at least the same features (Section 9.9) | MUST | 9.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-9.9.2-1` | The path statement is present where the type is leafref, its argument refers to a leaf or leaf-list node, and where require-instance is true the node set it selects is non-empty (Section 9.9.2) | MUST | 9.9.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-9.9.3-1` | Where require-instance is true, the instance being referred to exists for the data to be valid (Section 9.9.3) | MUST | 9.9.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-9.10.2-1` | The base statement is present at least once where the type is identityref, and its argument names an identity defined in the current module or an included submodule (Section 9.10.2) | MUST | 9.10.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-9.10.3-1` | An identityref value names an identity defined in the current module or one of its submodules (Section 9.10.3) | MUST | 9.10.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-9.12-2` | The type statement is present where the type is union (Section 9.12) | MUST | 9.12 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-9.13-1` | An instance-identifier gives one equality-test predicate per key of a list entry; where it represents configuration and require-instance is true the node it refers to represents configuration; and the nodes it references exist for the data to be valid (Section 9.13) | MUST | 9.13 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-11-1` | When a module is updated a new revision statement is placed in front of the existing ones, or added where none exist; the organization and contact metadata statements are updated as needed; the module name and the namespace statement are never changed; obsolete definitions are never removed from a published module; a change to the semantics of a definition is made through a new definition; and data definition substatements are never reordered (Section 11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-12-1` | A YANG 1.1 module includes no YANG 1 submodule and a YANG 1 module includes no YANG 1.1 submodule, and a YANG 1 module or submodule does not import a YANG 1.1 module by revision (Section 12) | MUST | 12 | **positive:** no positive test. **negative:** no negative test |
| `RFC7950-7.6.4-1` | If a leaf has a default value and the leaf is not set, the default value should be used (Section 7.6.4) | SHOULD | 7.6.4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7950-7.5.3-1`](#rfc7950-7.5.3-1) If a must expression evaluates to false, the data is not valid (Section 7.5.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze uses zero YANG must statements (a grep of the .yang models finds must only inside a description); it parses via goyang which evaluates no XPath, and enforces cross-field constraints with Go ze:validate functions (internal/component/config/yang/validator.go:756 applyCustomValidators) instead, so there is no must-XPath code path |
| [`RFC7950-9.2.4-1`](#rfc7950-9.2.4-1) All range, length, and pattern restrictions must be more restrictive than or equal to the base type's restrictions (Section 9.2.4) | no test | no test carries this requirement id; annotated {not-applicable}: this is a schema-authoring-time narrowing constraint resolved by goyang during module processing (internal/component/config/yang/loader.go); ze authors valid narrowings and has no runtime enforcement surface for it |
| [`RFC7950-9.4.5-1`](#rfc7950-9.4.5-1) Multiple pattern statements on the same type are combined as logical AND (Section 9.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: no ze type carries two or more pattern statements (a scan of the .yang models finds none); the validator loop (internal/component/config/yang/validator.go:268) AND-combines patterns if present, but the multi-pattern construct is unused |
| [`RFC7950-x-1`](#rfc7950-x-1) YANG modules must declare yang-version 1.1 (Core Constructs) | no test | no test carries this requirement id; annotated {not-applicable}: ze's YANG models declare no yang-version statement and default to YANG 1.0 in goyang; they use no YANG-1.1-only construct that would require the 1.1 declaration, so the version-declaration obligation has no applicable module |
| [`RFC7950-x-2`](#rfc7950-x-2) Enum integer positions must not change across revisions (Type System) | no test | no test carries this requirement id; annotated {not-applicable}: ze's modules carry no consumed revision statements and there is no cross-revision position-stability tooling; ze pins the values that matter with explicit value statements (e.g. role.yang, ze-types.yang afi/safi) but enforces no automated cross-revision check |
| [`RFC7950-5.1-1`](#rfc7950-5.1-1) Module names are unique within a server; a module includes all its submodules; a submodule is included only by the module it belongs to or by another submodule of that module, never imports its own module, and never includes a submodule of another module; a submodule includes the same submodule revisions its module includes and no submodule is included at two revisions; an external module is imported before its definitions are referenced, with no circular chains of imports; a reference to an external definition uses a locally defined prefix followed by a colon (Section 5.1, Section 7.1.6, Section 7.2.2) | no test | no test carries this requirement id |
| [`RFC7950-5.3-1`](#rfc7950-5.3-1) Choose namespace URIs so they cannot collide with standard or other enterprise namespaces, for example by using the enterprise or organization name in the namespace (Section 5.3) | no test | no test carries this requirement id |
| [`RFC7950-5.5-1`](#rfc7950-5.5-1) A scoped definition does not shadow a definition at a higher scope (Section 5.5) | no test | no test carries this requirement id |
| [`RFC7950-5.6.5-1`](#rfc7950-5.6.5-1) Implement no more than one revision of a module, and where a supported augment or path statement uses a node from an imported module, implement a revision of that module that carries the node (Section 5.6.5) | no test | no test carries this requirement id |
| [`RFC7950-6.1.3-1`](#rfc7950-6.1.3-1) In a double-quoted string a backslash is followed only by one of the characters the escape rules define (Section 6.1.3) | no test | no test carries this requirement id |
| [`RFC7950-6.2-1`](#rfc7950-6.2-1) Support identifiers up to 64 characters in length (Section 6.2) | no test | no test carries this requirement id |
| [`RFC7950-6.2.1-1`](#rfc7950-6.2.1-1) All identifiers defined in one namespace are unique: a descendant node defines no typedef and no grouping whose name is already visible from an ancestor (Section 6.2.1) | no test | no test carries this requirement id |
| [`RFC7950-6.3.1-1`](#rfc7950-6.3.1-1) Qualify an extension keyword with the prefix of the module that defines it, including inside that module, and process a supported extension in accordance with the specification governing it (Section 6.3.1) | no test | no test carries this requirement id |
| [`RFC7950-6.4-1`](#rfc7950-6.4-1) Enforce the requirements the data model encodes, whether or not an XPath interpreter is implemented (Section 6.4) | no test | no test carries this requirement id |
| [`RFC7950-6.4-2`](#rfc7950-6.4-2) XPath expressions are syntactically correct and every prefix they use is present in the XPath context (Section 6.4) | no test | no test carries this requirement id |
| [`RFC7950-6.5-1`](#rfc7950-6.5-1) Qualify a reference to an identifier defined in an external module with the appropriate prefix (Section 6.5) | no test | no test carries this requirement id |
| [`RFC7950-7.1.4-1`](#rfc7950-7.1.4-1) All prefixes, the module's own included, are unique within the module or submodule, and where two imported modules define the same prefix at least one is imported under a different prefix (Section 7.1.4) | no test | no test carries this requirement id |
| [`RFC7950-7.3-1`](#rfc7950-7.3-1) A typedef's argument is followed by a block of substatements, its type substatement is present, its name is not one of the YANG built-in types, and a top-level typedef name is unique within the module (Section 7.3, Section 7.3.2) | no test | no test carries this requirement id |
| [`RFC7950-7.3.4-1`](#rfc7950-7.3.4-1) A typedef's default value is valid according to its type, and a derived type or leaf whose restrictions invalidate the inherited default specifies a new compatible default (Section 7.3.4) | no test | no test carries this requirement id |
| [`RFC7950-7.6.1-1`](#rfc7950-7.6.1-1) Use a leaf's default value when the leaf is not set and its ancestry allows it, and behave operationally as if the leaf were present in the data tree with that value (Section 7.6.1) | no test | no test carries this requirement id |
| [`RFC7950-7.6.3-1`](#rfc7950-7.6.3-1) A leaf's type statement is present (Section 7.6.3) | no test | no test carries this requirement id |
| [`RFC7950-7.6.4-2`](#rfc7950-7.6.4-2) A leaf's default value is valid according to the leaf's type, is absent where mandatory is true, and is not marked with an if-feature statement (Section 7.6.4) | no test | no test carries this requirement id |
| [`RFC7950-7.7-1`](#rfc7950-7.7-1) In configuration data the values in a leaf-list are unique, the definitions of default values carry no if-feature statement, and the values in the data tree are in canonical form (Section 7.7) | no test | no test carries this requirement id |
| [`RFC7950-7.7.2-1`](#rfc7950-7.7.2-1) Use a leaf-list's default values when it is not set and its ancestry allows it, and behave operationally as if the leaf-list were present in the data tree with those values (Section 7.7.2) | no test | no test carries this requirement id |
| [`RFC7950-7.7.4-1`](#rfc7950-7.7.4-1) A leaf-list's default value is valid according to its type and is absent where min-elements is one or more (Section 7.7.4) | no test | no test carries this requirement id |
| [`RFC7950-7.7.5-1`](#rfc7950-7.7.5-1) A valid leaf-list or list has at least min-elements entries (Section 7.7.5) | no test | no test carries this requirement id |
| [`RFC7950-7.8.2-1`](#rfc7950-7.8.2-1) A list that represents configuration carries a key statement; each key leaf identifier appears once, refers to a child leaf of the list, is given a value when a list entry is created, and has the same config value as the list (Section 7.8.2) | no test | no test carries this requirement id |
| [`RFC7950-7.8.3-1`](#rfc7950-7.8.3-1) A unique argument names descendant-form schema node identifiers that each refer to a leaf; where one referenced leaf represents configuration all of them do, and the combined values are unique across all list entries in which every referenced leaf exists (Section 7.8.3) | no test | no test carries this requirement id |
| [`RFC7950-7.9.2-1`](#rfc7950-7.9.2-1) Child node identifiers are unique across all cases of a choice, a case identifier is unique within its choice, and a schema node identifier always includes the case node identifier (Section 7.9.2) | no test | no test carries this requirement id |
| [`RFC7950-7.9.3-1`](#rfc7950-7.9.3-1) A choice carries no default statement where mandatory is true, and the default case holds no mandatory node (Section 7.9.3) | no test | no test carries this requirement id |
| [`RFC7950-7.9.4-1`](#rfc7950-7.9.4-1) Where a choice is mandatory, at least one node from exactly one of its case branches exists (Section 7.9.4) | no test | no test carries this requirement id |
| [`RFC7950-7.12-1`](#rfc7950-7.12-1) A grouping never references itself, directly or through a chain of groupings, and a top-level grouping identifier is unique within the module (Section 7.12) | no test | no test carries this requirement id |
| [`RFC7950-7.14.2-1`](#rfc7950-7.14.2-1) In an RPC or action invocation a mandatory input leaf is present, the server uses the input defaults in the cases Sections 7.6.1 and 7.7.2 describe and behaves as if the defaulted node were present, and no node whose when statement evaluates to false is present (Section 7.14.2) | no test | no test carries this requirement id |
| [`RFC7950-7.14.3-1`](#rfc7950-7.14.3-1) In an RPC or action reply a mandatory output leaf is present, the client uses the output defaults in the cases Sections 7.6.1 and 7.7.2 describe and behaves as if the defaulted node were present, and no node whose when statement evaluates to false is present (Section 7.14.3) | no test | no test carries this requirement id |
| [`RFC7950-7.15-1`](#rfc7950-7.15-1) An action is not defined within an rpc, another action or a notification, and has no ancestor node that is a list without a key statement (Section 7.15) | no test | no test carries this requirement id |
| [`RFC7950-7.16-1`](#rfc7950-7.16-1) A notification is not defined within an rpc, an action or another notification, has no ancestor node that is a list without a key statement, carries its mandatory leafs, and its receiver uses the notification's default values in the cases Sections 7.6.1 and 7.7.2 describe (Section 7.16) | no test | no test carries this requirement id |
| [`RFC7950-7.17-1`](#rfc7950-7.17-1) An augment target is a container, list, choice, case, input, output or notification node; a top-level augment uses the absolute form of a schema node identifier and an augment under uses the descendant form; an augment adds no two nodes of the same name from the same module to one target; and an augment that adds mandatory configuration nodes to another module's target is made conditional with a when statement (Section 7.17) | no test | no test carries this requirement id |
| [`RFC7950-7.18.2-1`](#rfc7950-7.18.2-1) A base argument names an identity defined in the current module or an included submodule, and an identity never references itself, directly or through a chain of identities (Section 7.18.2) | no test | no test carries this requirement id |
| [`RFC7950-7.19-1`](#rfc7950-7.19-1) The substatements of an extension usage are YANG statements, extensions included, and follow the syntactical rules of Section 14 (Section 7.19) | no test | no test carries this requirement id |
| [`RFC7950-7.20.1-1`](#rfc7950-7.20.1-1) A feature never references itself, and a server that supports a feature supports every feature that feature depends on (Section 7.20.1) | no test | no test carries this requirement id |
| [`RFC7950-7.20.2-1`](#rfc7950-7.20.2-1) An if-feature argument names a feature defined in the current module or an included submodule, and a leaf that is a list key carries no if-feature statement (Section 7.20.2) | no test | no test carries this requirement id |
| [`RFC7950-7.20.3-1`](#rfc7950-7.20.3-1) A deviation is never part of a published standard, a server deviation is used only as a last resort, and the data model that results from applying all of a server's deviations in any order is still valid (Section 7.20.3) | no test | no test carries this requirement id |
| [`RFC7950-7.20.3.2-1`](#rfc7950-7.20.3.2-1) A deviate add does not add a property that can appear only once and already exists in the target node; the properties a deviate replace names exist in the target node; and a deviate delete substatement matches the target node's keyword and argument string (Section 7.20.3.2) | no test | no test carries this requirement id |
| [`RFC7950-7.21.2-1`](#rfc7950-7.21.2-1) A current definition references no deprecated or obsolete definition within the same module, and a deprecated definition references no obsolete definition within the same module (Section 7.21.2) | no test | no test carries this requirement id |
| [`RFC7950-7.21.5-1`](#rfc7950-7.21.5-1) A leaf that is a list key carries no when statement and neither does a uses statement that brings a key leaf into a list; when expressions on referenced nodes are evaluated first; and there are no circular dependencies among when expressions (Section 7.21.5) | no test | no test carries this requirement id |
| [`RFC7950-8.1-1`](#rfc7950-8.1-1) A valid data tree satisfies every Section 8.1 constraint: every leaf value matches its type constraints including range, length and pattern; all key leafs are present for every list entry; nodes are present for at most one case branch of a choice; no node tagged with an if-feature whose expression is false and no node tagged with a when whose condition is false is present; every path referential-integrity constraint is satisfied; every unique constraint is satisfied; mandatory is enforced for leafs and choices; and the constraint holds for configuration data, state data, notification content, and RPC or action input and output (Section 8.1) | no test | no test carries this requirement id |
| [`RFC7950-8.1-2`](#rfc7950-8.1-2) The running configuration datastore is always valid (Section 8.1) | no test | no test carries this requirement id |
| [`RFC7950-8.3-1`](#rfc7950-8.3-1) Enforce configuration constraints in each of the three windows Section 8.3 defines (Section 8.3) | no test | no test carries this requirement id |
| [`RFC7950-8.3.1-2`](#rfc7950-8.3.1-2) Reject content that carries data for more than one case branch of a choice, or data for a node whose if-feature expression or when condition evaluates to false, answering with the error the management protocol defines (a bad-element or unknown-element error-tag under NETCONF) (Section 8.3.1) | no test | no test carries this requirement id |
| [`RFC7950-8.3.2-1`](#rfc7950-8.3.2-1) While processing a datastore modification, detect data for a node whose if-feature expression or when condition evaluates to false and reject it (Section 8.3.2) | no test | no test carries this requirement id |
| [`RFC7950-8.3.3-1`](#rfc7950-8.3.3-1) When datastore processing is complete the final contents obey every validation constraint, enforced at the end of the operation for the running and startup datastores (Section 8.3.3) | no test | no test carries this requirement id |
| [`RFC7950-9.1-1`](#rfc7950-9.1-1) Support all the lexical representations this specification defines, and where a type has no canonical form the value's format matches the type's lexical representation (Section 9.1) | no test | no test carries this requirement id |
| [`RFC7950-9.2.4-2`](#rfc7950-9.2.4-2) The values and ranges of a range restriction are disjoint and in ascending order, and every explicit value and range boundary matches the type being restricted or is one of the special values min and max (Section 9.2.4) | no test | no test carries this requirement id |
| [`RFC7950-9.3.2-1`](#rfc7950-9.3.2-1) A decimal64 value carries at least one digit before and after the decimal point, with no leading or trailing zeros (Section 9.3.2) | no test | no test carries this requirement id |
| [`RFC7950-9.3.4-1`](#rfc7950-9.3.4-1) The fraction-digits statement is present where the type is decimal64 (Section 9.3.4) | no test | no test carries this requirement id |
| [`RFC7950-9.4.4-1`](#rfc7950-9.4.4-1) Length-restricting values are never negative, and the values and ranges of a length restriction are disjoint and in ascending order (Section 9.4.4) | no test | no test carries this requirement id |
| [`RFC7950-9.5.1-1`](#rfc7950-9.5.1-1) Represent a boolean value as the lowercase string true or false (Section 9.5.1) | no test | no test carries this requirement id |
| [`RFC7950-9.6.4-1`](#rfc7950-9.6.4-1) The enum statement is present where the type is enumeration; an assigned name is not zero-length and carries no leading or trailing whitespace; and all assigned names in an enumeration are unique (Section 9.6.4) | no test | no test carries this requirement id |
| [`RFC7950-9.6.4-2`](#rfc7950-9.6.4-2) When an existing enumeration type is restricted, the new type's assigned names are a subset of the base type's and the value of an assigned name is not changed (Section 9.6.4) | no test | no test carries this requirement id |
| [`RFC7950-9.6.4.2-1`](#rfc7950-9.6.4.2-1) An enum value is in the range -2147483648 to 2147483647 and unique within the enumeration type; once the highest value reaches 2147483647 a value is given explicitly for the enum substatements that follow; and a restricted enumeration either repeats the base type's value or omits the value statement (Section 9.6.4.2) | no test | no test carries this requirement id |
| [`RFC7950-9.7-1`](#rfc7950-9.7-1) When an existing bits type is restricted, the new type's assigned names are a subset of the base type's and the bit position of an assigned name is not changed (Section 9.7) | no test | no test carries this requirement id |
| [`RFC7950-9.7.4-1`](#rfc7950-9.7.4-1) The bit statement is present where the type is bits, and all assigned names in a bits type are unique (Section 9.7.4) | no test | no test carries this requirement id |
| [`RFC7950-9.7.4.2-1`](#rfc7950-9.7.4.2-1) A bit position is in the range 0 to 4294967295 and unique within the bits type; once the highest position reaches 4294967295 a position is given explicitly for the bit substatements that follow; and a restricted bits type either repeats the base type's position or omits the position statement (Section 9.7.4.2) | no test | no test carries this requirement id |
| [`RFC7950-9.9-1`](#rfc7950-9.9-1) Where require-instance is true the node a leafref refers to exists; a leafref that represents configuration refers to configuration; there are no circular chains of leafrefs; and a leafref to a feature-conditional leaf is itself conditional on at least the same features (Section 9.9) | no test | no test carries this requirement id |
| [`RFC7950-9.9.2-1`](#rfc7950-9.9.2-1) The path statement is present where the type is leafref, its argument refers to a leaf or leaf-list node, and where require-instance is true the node set it selects is non-empty (Section 9.9.2) | no test | no test carries this requirement id |
| [`RFC7950-9.9.3-1`](#rfc7950-9.9.3-1) Where require-instance is true, the instance being referred to exists for the data to be valid (Section 9.9.3) | no test | no test carries this requirement id |
| [`RFC7950-9.10.2-1`](#rfc7950-9.10.2-1) The base statement is present at least once where the type is identityref, and its argument names an identity defined in the current module or an included submodule (Section 9.10.2) | no test | no test carries this requirement id |
| [`RFC7950-9.10.3-1`](#rfc7950-9.10.3-1) An identityref value names an identity defined in the current module or one of its submodules (Section 9.10.3) | no test | no test carries this requirement id |
| [`RFC7950-9.12-2`](#rfc7950-9.12-2) The type statement is present where the type is union (Section 9.12) | no test | no test carries this requirement id |
| [`RFC7950-9.13-1`](#rfc7950-9.13-1) An instance-identifier gives one equality-test predicate per key of a list entry; where it represents configuration and require-instance is true the node it refers to represents configuration; and the nodes it references exist for the data to be valid (Section 9.13) | no test | no test carries this requirement id |
| [`RFC7950-11-1`](#rfc7950-11-1) When a module is updated a new revision statement is placed in front of the existing ones, or added where none exist; the organization and contact metadata statements are updated as needed; the module name and the namespace statement are never changed; obsolete definitions are never removed from a published module; a change to the semantics of a definition is made through a new definition; and data definition substatements are never reordered (Section 11) | no test | no test carries this requirement id |
| [`RFC7950-12-1`](#rfc7950-12-1) A YANG 1.1 module includes no YANG 1 submodule and a YANG 1 module includes no YANG 1.1 submodule, and a YANG 1 module or submodule does not import a YANG 1.1 module by revision (Section 12) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7950-8.3.1-1`](#rfc7950-8.3.1-1)

If a leaf data value does not match type constraints (range, length, pattern), server must reply with an invalid-value error (Section 8.3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateTree_LengthViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L411) | unit/verify | unproven |
| negative | [`TestValidateTree_PatternViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L310) | unit/verify | unproven |
| negative | [`TestValidateTree_RangeViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L169) | unit/verify | unproven |
| negative | [`TestValidator_HoldTimeRange`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L894) | unit/verify | unproven |
| negative | [`TestValidator_ValidatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L845) | unit/verify | unproven |
| positive | [`TestValidator_HoldTimeRange`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L893) | unit/verify | unproven |
| positive | [`TestValidator_ValidatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L844) | unit/verify | unproven |

### [`RFC7950-9.6-1`](#rfc7950-9.6-1)

An enumeration value must be one of the values specified in the type's enum statements (Section 9.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISAuthAlgorithmEnumAcceptsAll`](https://github.com/ze-software/ze/blob/main/internal/component/config/isis_auth_algorithm_enum_test.go#L81) | unit/verify | unproven |
| negative | [`TestRadiusAuthMethodEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/radius_auth_method_enum_test.go#L64) | unit/verify | revert, verified |
| negative | [`TestValidateTree_AddPathDirectionEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L613) | unit/verify | unproven |
| negative | [`TestValidateTree_FamilyModeEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L547) | unit/verify | unproven |
| positive | [`TestISISAuthAlgorithmEnumAcceptsAll`](https://github.com/ze-software/ze/blob/main/internal/component/config/isis_auth_algorithm_enum_test.go#L64) | unit/verify | unproven |
| positive | [`TestRadiusAuthMethodEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/radius_auth_method_enum_test.go#L53) | unit/verify | revert, verified |
| positive | [`TestValidateTree_AddPathDirectionEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L612) | unit/verify | unproven |
| positive | [`TestValidateTree_FamilyModeEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L546) | unit/verify | unproven |

### [`RFC7950-9.12-1`](#rfc7950-9.12-1)

A union value must match at least one member type (Section 9.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateTree_UnionViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L360) | unit/verify | unproven |
| positive | [`TestValidateTree_ValidConfig`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L46) | unit/verify | unproven |

### [`RFC7950-7.6.5-1`](#rfc7950-7.6.5-1)

If a mandatory node does not exist, server must reply with a missing-element error (Section 7.6.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateTree_MandatoryMissing`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L332) | unit/verify | unproven |
| negative | [`TestValidator_MandatoryField`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L929) | unit/verify | unproven |
| positive | [`TestValidator_MandatoryField`](https://github.com/ze-software/ze/blob/main/internal/component/config/validator_yang_test.go#L928) | unit/verify | unproven |

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

### [`RFC7950-5.1-1`](#rfc7950-5.1-1)

Module names are unique within a server; a module includes all its submodules; a submodule is included only by the module it belongs to or by another submodule of that module, never imports its own module, and never includes a submodule of another module; a submodule includes the same submodule revisions its module includes and no submodule is included at two revisions; an external module is imported before its definitions are referenced, with no circular chains of imports; a reference to an external definition uses a locally defined prefix followed by a colon (Section 5.1, Section 7.1.6, Section 7.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-5.1-1, so no unit is bound to it.

### [`RFC7950-5.3-1`](#rfc7950-5.3-1)

Choose namespace URIs so they cannot collide with standard or other enterprise namespaces, for example by using the enterprise or organization name in the namespace (Section 5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-5.3-1, so no unit is bound to it.

### [`RFC7950-5.5-1`](#rfc7950-5.5-1)

A scoped definition does not shadow a definition at a higher scope (Section 5.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-5.5-1, so no unit is bound to it.

### [`RFC7950-5.6.5-1`](#rfc7950-5.6.5-1)

Implement no more than one revision of a module, and where a supported augment or path statement uses a node from an imported module, implement a revision of that module that carries the node (Section 5.6.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-5.6.5-1, so no unit is bound to it.

### [`RFC7950-6.1.3-1`](#rfc7950-6.1.3-1)

In a double-quoted string a backslash is followed only by one of the characters the escape rules define (Section 6.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-6.1.3-1, so no unit is bound to it.

### [`RFC7950-6.2-1`](#rfc7950-6.2-1)

Support identifiers up to 64 characters in length (Section 6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-6.2-1, so no unit is bound to it.

### [`RFC7950-6.2.1-1`](#rfc7950-6.2.1-1)

All identifiers defined in one namespace are unique: a descendant node defines no typedef and no grouping whose name is already visible from an ancestor (Section 6.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-6.2.1-1, so no unit is bound to it.

### [`RFC7950-6.3.1-1`](#rfc7950-6.3.1-1)

Qualify an extension keyword with the prefix of the module that defines it, including inside that module, and process a supported extension in accordance with the specification governing it (Section 6.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-6.3.1-1, so no unit is bound to it.

### [`RFC7950-6.4-1`](#rfc7950-6.4-1)

Enforce the requirements the data model encodes, whether or not an XPath interpreter is implemented (Section 6.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-6.4-1, so no unit is bound to it.

### [`RFC7950-6.4-2`](#rfc7950-6.4-2)

XPath expressions are syntactically correct and every prefix they use is present in the XPath context (Section 6.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-6.4-2, so no unit is bound to it.

### [`RFC7950-6.5-1`](#rfc7950-6.5-1)

Qualify a reference to an identifier defined in an external module with the appropriate prefix (Section 6.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-6.5-1, so no unit is bound to it.

### [`RFC7950-7.1.4-1`](#rfc7950-7.1.4-1)

All prefixes, the module's own included, are unique within the module or submodule, and where two imported modules define the same prefix at least one is imported under a different prefix (Section 7.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.1.4-1, so no unit is bound to it.

### [`RFC7950-7.3-1`](#rfc7950-7.3-1)

A typedef's argument is followed by a block of substatements, its type substatement is present, its name is not one of the YANG built-in types, and a top-level typedef name is unique within the module (Section 7.3, Section 7.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.3-1, so no unit is bound to it.

### [`RFC7950-7.3.4-1`](#rfc7950-7.3.4-1)

A typedef's default value is valid according to its type, and a derived type or leaf whose restrictions invalidate the inherited default specifies a new compatible default (Section 7.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.3.4-1, so no unit is bound to it.

### [`RFC7950-7.6.1-1`](#rfc7950-7.6.1-1)

Use a leaf's default value when the leaf is not set and its ancestry allows it, and behave operationally as if the leaf were present in the data tree with that value (Section 7.6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.6.1-1, so no unit is bound to it.

### [`RFC7950-7.6.3-1`](#rfc7950-7.6.3-1)

A leaf's type statement is present (Section 7.6.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.6.3-1, so no unit is bound to it.

### [`RFC7950-7.6.4-2`](#rfc7950-7.6.4-2)

A leaf's default value is valid according to the leaf's type, is absent where mandatory is true, and is not marked with an if-feature statement (Section 7.6.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.6.4-2, so no unit is bound to it.

### [`RFC7950-7.7-1`](#rfc7950-7.7-1)

In configuration data the values in a leaf-list are unique, the definitions of default values carry no if-feature statement, and the values in the data tree are in canonical form (Section 7.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.7-1, so no unit is bound to it.

### [`RFC7950-7.7.2-1`](#rfc7950-7.7.2-1)

Use a leaf-list's default values when it is not set and its ancestry allows it, and behave operationally as if the leaf-list were present in the data tree with those values (Section 7.7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.7.2-1, so no unit is bound to it.

### [`RFC7950-7.7.4-1`](#rfc7950-7.7.4-1)

A leaf-list's default value is valid according to its type and is absent where min-elements is one or more (Section 7.7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.7.4-1, so no unit is bound to it.

### [`RFC7950-7.7.5-1`](#rfc7950-7.7.5-1)

A valid leaf-list or list has at least min-elements entries (Section 7.7.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.7.5-1, so no unit is bound to it.

### [`RFC7950-7.8.2-1`](#rfc7950-7.8.2-1)

A list that represents configuration carries a key statement; each key leaf identifier appears once, refers to a child leaf of the list, is given a value when a list entry is created, and has the same config value as the list (Section 7.8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.8.2-1, so no unit is bound to it.

### [`RFC7950-7.8.3-1`](#rfc7950-7.8.3-1)

A unique argument names descendant-form schema node identifiers that each refer to a leaf; where one referenced leaf represents configuration all of them do, and the combined values are unique across all list entries in which every referenced leaf exists (Section 7.8.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.8.3-1, so no unit is bound to it.

### [`RFC7950-7.9.2-1`](#rfc7950-7.9.2-1)

Child node identifiers are unique across all cases of a choice, a case identifier is unique within its choice, and a schema node identifier always includes the case node identifier (Section 7.9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.9.2-1, so no unit is bound to it.

### [`RFC7950-7.9.3-1`](#rfc7950-7.9.3-1)

A choice carries no default statement where mandatory is true, and the default case holds no mandatory node (Section 7.9.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.9.3-1, so no unit is bound to it.

### [`RFC7950-7.9.4-1`](#rfc7950-7.9.4-1)

Where a choice is mandatory, at least one node from exactly one of its case branches exists (Section 7.9.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.9.4-1, so no unit is bound to it.

### [`RFC7950-7.12-1`](#rfc7950-7.12-1)

A grouping never references itself, directly or through a chain of groupings, and a top-level grouping identifier is unique within the module (Section 7.12)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.12-1, so no unit is bound to it.

### [`RFC7950-7.14.2-1`](#rfc7950-7.14.2-1)

In an RPC or action invocation a mandatory input leaf is present, the server uses the input defaults in the cases Sections 7.6.1 and 7.7.2 describe and behaves as if the defaulted node were present, and no node whose when statement evaluates to false is present (Section 7.14.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.14.2-1, so no unit is bound to it.

### [`RFC7950-7.14.3-1`](#rfc7950-7.14.3-1)

In an RPC or action reply a mandatory output leaf is present, the client uses the output defaults in the cases Sections 7.6.1 and 7.7.2 describe and behaves as if the defaulted node were present, and no node whose when statement evaluates to false is present (Section 7.14.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.14.3-1, so no unit is bound to it.

### [`RFC7950-7.15-1`](#rfc7950-7.15-1)

An action is not defined within an rpc, another action or a notification, and has no ancestor node that is a list without a key statement (Section 7.15)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.15-1, so no unit is bound to it.

### [`RFC7950-7.16-1`](#rfc7950-7.16-1)

A notification is not defined within an rpc, an action or another notification, has no ancestor node that is a list without a key statement, carries its mandatory leafs, and its receiver uses the notification's default values in the cases Sections 7.6.1 and 7.7.2 describe (Section 7.16)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.16-1, so no unit is bound to it.

### [`RFC7950-7.17-1`](#rfc7950-7.17-1)

An augment target is a container, list, choice, case, input, output or notification node; a top-level augment uses the absolute form of a schema node identifier and an augment under uses the descendant form; an augment adds no two nodes of the same name from the same module to one target; and an augment that adds mandatory configuration nodes to another module's target is made conditional with a when statement (Section 7.17)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.17-1, so no unit is bound to it.

### [`RFC7950-7.18.2-1`](#rfc7950-7.18.2-1)

A base argument names an identity defined in the current module or an included submodule, and an identity never references itself, directly or through a chain of identities (Section 7.18.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.18.2-1, so no unit is bound to it.

### [`RFC7950-7.19-1`](#rfc7950-7.19-1)

The substatements of an extension usage are YANG statements, extensions included, and follow the syntactical rules of Section 14 (Section 7.19)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.19-1, so no unit is bound to it.

### [`RFC7950-7.20.1-1`](#rfc7950-7.20.1-1)

A feature never references itself, and a server that supports a feature supports every feature that feature depends on (Section 7.20.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.20.1-1, so no unit is bound to it.

### [`RFC7950-7.20.2-1`](#rfc7950-7.20.2-1)

An if-feature argument names a feature defined in the current module or an included submodule, and a leaf that is a list key carries no if-feature statement (Section 7.20.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.20.2-1, so no unit is bound to it.

### [`RFC7950-7.20.3-1`](#rfc7950-7.20.3-1)

A deviation is never part of a published standard, a server deviation is used only as a last resort, and the data model that results from applying all of a server's deviations in any order is still valid (Section 7.20.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.20.3-1, so no unit is bound to it.

### [`RFC7950-7.20.3.2-1`](#rfc7950-7.20.3.2-1)

A deviate add does not add a property that can appear only once and already exists in the target node; the properties a deviate replace names exist in the target node; and a deviate delete substatement matches the target node's keyword and argument string (Section 7.20.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.20.3.2-1, so no unit is bound to it.

### [`RFC7950-7.21.2-1`](#rfc7950-7.21.2-1)

A current definition references no deprecated or obsolete definition within the same module, and a deprecated definition references no obsolete definition within the same module (Section 7.21.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.21.2-1, so no unit is bound to it.

### [`RFC7950-7.21.5-1`](#rfc7950-7.21.5-1)

A leaf that is a list key carries no when statement and neither does a uses statement that brings a key leaf into a list; when expressions on referenced nodes are evaluated first; and there are no circular dependencies among when expressions (Section 7.21.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.21.5-1, so no unit is bound to it.

### [`RFC7950-8.1-1`](#rfc7950-8.1-1)

A valid data tree satisfies every Section 8.1 constraint: every leaf value matches its type constraints including range, length and pattern; all key leafs are present for every list entry; nodes are present for at most one case branch of a choice; no node tagged with an if-feature whose expression is false and no node tagged with a when whose condition is false is present; every path referential-integrity constraint is satisfied; every unique constraint is satisfied; mandatory is enforced for leafs and choices; and the constraint holds for configuration data, state data, notification content, and RPC or action input and output (Section 8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-8.1-1, so no unit is bound to it.

### [`RFC7950-8.1-2`](#rfc7950-8.1-2)

The running configuration datastore is always valid (Section 8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-8.1-2, so no unit is bound to it.

### [`RFC7950-8.3-1`](#rfc7950-8.3-1)

Enforce configuration constraints in each of the three windows Section 8.3 defines (Section 8.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-8.3-1, so no unit is bound to it.

### [`RFC7950-8.3.1-2`](#rfc7950-8.3.1-2)

Reject content that carries data for more than one case branch of a choice, or data for a node whose if-feature expression or when condition evaluates to false, answering with the error the management protocol defines (a bad-element or unknown-element error-tag under NETCONF) (Section 8.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-8.3.1-2, so no unit is bound to it.

### [`RFC7950-8.3.2-1`](#rfc7950-8.3.2-1)

While processing a datastore modification, detect data for a node whose if-feature expression or when condition evaluates to false and reject it (Section 8.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-8.3.2-1, so no unit is bound to it.

### [`RFC7950-8.3.3-1`](#rfc7950-8.3.3-1)

When datastore processing is complete the final contents obey every validation constraint, enforced at the end of the operation for the running and startup datastores (Section 8.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-8.3.3-1, so no unit is bound to it.

### [`RFC7950-9.1-1`](#rfc7950-9.1-1)

Support all the lexical representations this specification defines, and where a type has no canonical form the value's format matches the type's lexical representation (Section 9.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.1-1, so no unit is bound to it.

### [`RFC7950-9.2.4-2`](#rfc7950-9.2.4-2)

The values and ranges of a range restriction are disjoint and in ascending order, and every explicit value and range boundary matches the type being restricted or is one of the special values min and max (Section 9.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.2.4-2, so no unit is bound to it.

### [`RFC7950-9.3.2-1`](#rfc7950-9.3.2-1)

A decimal64 value carries at least one digit before and after the decimal point, with no leading or trailing zeros (Section 9.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.3.2-1, so no unit is bound to it.

### [`RFC7950-9.3.4-1`](#rfc7950-9.3.4-1)

The fraction-digits statement is present where the type is decimal64 (Section 9.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.3.4-1, so no unit is bound to it.

### [`RFC7950-9.4.4-1`](#rfc7950-9.4.4-1)

Length-restricting values are never negative, and the values and ranges of a length restriction are disjoint and in ascending order (Section 9.4.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.4.4-1, so no unit is bound to it.

### [`RFC7950-9.5.1-1`](#rfc7950-9.5.1-1)

Represent a boolean value as the lowercase string true or false (Section 9.5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.5.1-1, so no unit is bound to it.

### [`RFC7950-9.6.4-1`](#rfc7950-9.6.4-1)

The enum statement is present where the type is enumeration; an assigned name is not zero-length and carries no leading or trailing whitespace; and all assigned names in an enumeration are unique (Section 9.6.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.6.4-1, so no unit is bound to it.

### [`RFC7950-9.6.4-2`](#rfc7950-9.6.4-2)

When an existing enumeration type is restricted, the new type's assigned names are a subset of the base type's and the value of an assigned name is not changed (Section 9.6.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.6.4-2, so no unit is bound to it.

### [`RFC7950-9.6.4.2-1`](#rfc7950-9.6.4.2-1)

An enum value is in the range -2147483648 to 2147483647 and unique within the enumeration type; once the highest value reaches 2147483647 a value is given explicitly for the enum substatements that follow; and a restricted enumeration either repeats the base type's value or omits the value statement (Section 9.6.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.6.4.2-1, so no unit is bound to it.

### [`RFC7950-9.7-1`](#rfc7950-9.7-1)

When an existing bits type is restricted, the new type's assigned names are a subset of the base type's and the bit position of an assigned name is not changed (Section 9.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.7-1, so no unit is bound to it.

### [`RFC7950-9.7.4-1`](#rfc7950-9.7.4-1)

The bit statement is present where the type is bits, and all assigned names in a bits type are unique (Section 9.7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.7.4-1, so no unit is bound to it.

### [`RFC7950-9.7.4.2-1`](#rfc7950-9.7.4.2-1)

A bit position is in the range 0 to 4294967295 and unique within the bits type; once the highest position reaches 4294967295 a position is given explicitly for the bit substatements that follow; and a restricted bits type either repeats the base type's position or omits the position statement (Section 9.7.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.7.4.2-1, so no unit is bound to it.

### [`RFC7950-9.9-1`](#rfc7950-9.9-1)

Where require-instance is true the node a leafref refers to exists; a leafref that represents configuration refers to configuration; there are no circular chains of leafrefs; and a leafref to a feature-conditional leaf is itself conditional on at least the same features (Section 9.9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.9-1, so no unit is bound to it.

### [`RFC7950-9.9.2-1`](#rfc7950-9.9.2-1)

The path statement is present where the type is leafref, its argument refers to a leaf or leaf-list node, and where require-instance is true the node set it selects is non-empty (Section 9.9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.9.2-1, so no unit is bound to it.

### [`RFC7950-9.9.3-1`](#rfc7950-9.9.3-1)

Where require-instance is true, the instance being referred to exists for the data to be valid (Section 9.9.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.9.3-1, so no unit is bound to it.

### [`RFC7950-9.10.2-1`](#rfc7950-9.10.2-1)

The base statement is present at least once where the type is identityref, and its argument names an identity defined in the current module or an included submodule (Section 9.10.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.10.2-1, so no unit is bound to it.

### [`RFC7950-9.10.3-1`](#rfc7950-9.10.3-1)

An identityref value names an identity defined in the current module or one of its submodules (Section 9.10.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.10.3-1, so no unit is bound to it.

### [`RFC7950-9.12-2`](#rfc7950-9.12-2)

The type statement is present where the type is union (Section 9.12)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.12-2, so no unit is bound to it.

### [`RFC7950-9.13-1`](#rfc7950-9.13-1)

An instance-identifier gives one equality-test predicate per key of a list entry; where it represents configuration and require-instance is true the node it refers to represents configuration; and the nodes it references exist for the data to be valid (Section 9.13)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.13-1, so no unit is bound to it.

### [`RFC7950-11-1`](#rfc7950-11-1)

When a module is updated a new revision statement is placed in front of the existing ones, or added where none exist; the organization and contact metadata statements are updated as needed; the module name and the namespace statement are never changed; obsolete definitions are never removed from a published module; a change to the semantics of a definition is made through a new definition; and data definition substatements are never reordered (Section 11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-11-1, so no unit is bound to it.

### [`RFC7950-12-1`](#rfc7950-12-1)

A YANG 1.1 module includes no YANG 1 submodule and a YANG 1 module includes no YANG 1.1 submodule, and a YANG 1 module or submodule does not import a YANG 1.1 module by revision (Section 12)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-12-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc7950.txt |
| Source fingerprint | e4e6e164b23abaf7 |
| Record | rfc/extraction/rfc7950.json |
| Mapped sentences | 192 |
| Declined as scope | 35 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | boilerplate and bibliography carrying no protocol obligation | 0 | skipped (front-matter) | boilerplate and bibliography carrying no protocol obligation |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 0 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `4.2.1` | not stated | 0 | walked | not stated |
| `4.2.2` | not stated | 0 | walked | not stated |
| `4.2.2.1` | not stated | 0 | walked | not stated |
| `4.2.2.2` | not stated | 0 | walked | not stated |
| `4.2.2.3` | not stated | 0 | walked | not stated |
| `4.2.2.4` | not stated | 0 | walked | not stated |
| `4.2.2.5` | not stated | 0 | walked | not stated |
| `4.2.3` | not stated | 0 | walked | not stated |
| `4.2.4` | not stated | 0 | walked | not stated |
| `4.2.5` | not stated | 0 | walked | not stated |
| `4.2.6` | not stated | 0 | walked | not stated |
| `4.2.7` | not stated | 0 | walked | not stated |
| `4.2.8` | not stated | 0 | walked | not stated |
| `4.2.9` | not stated | 0 | walked | not stated |
| `4.2.10` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 7 | walked | not stated |
| `5.1.1` | not stated | 0 | walked | not stated |
| `5.1.2` | not stated | 0 | walked | not stated |
| `5.1.2.1` | not stated | 0 | walked | not stated |
| `5.2` | not stated | 0 | walked | not stated |
| `5.3` | not stated | 2 | walked | not stated |
| `5.3.1` | not stated | 0 | walked | not stated |
| `5.4` | not stated | 0 | walked | not stated |
| `5.5` | not stated | 1 | walked | not stated |
| `5.6` | not stated | 0 | walked | not stated |
| `5.6.1` | not stated | 0 | walked | not stated |
| `5.6.2` | not stated | 0 | walked | not stated |
| `5.6.3` | not stated | 0 | walked | not stated |
| `5.6.4` | not stated | 4 | walked | not stated |
| `5.6.5` | not stated | 4 | walked | not stated |
| `5.7` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.1.1` | not stated | 0 | walked | not stated |
| `6.1.2` | not stated | 0 | walked | not stated |
| `6.1.3` | not stated | 1 | walked | not stated |
| `6.1.3.1` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 1 | walked | not stated |
| `6.2.1` | not stated | 3 | walked | not stated |
| `6.3` | not stated | 0 | walked | not stated |
| `6.3.1` | not stated | 3 | walked | not stated |
| `6.4` | not stated | 2 | walked | not stated |
| `6.4.1` | not stated | 0 | walked | not stated |
| `6.4.1.1` | not stated | 0 | walked | not stated |
| `6.5` | not stated | 1 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 1 | walked | not stated |
| `7.1.1` | not stated | 0 | walked | not stated |
| `7.1.2` | not stated | 1 | walked | not stated |
| `7.1.3` | not stated | 0 | walked | not stated |
| `7.1.4` | not stated | 2 | walked | not stated |
| `7.1.5` | not stated | 0 | walked | not stated |
| `7.1.5.1` | not stated | 0 | walked | not stated |
| `7.1.6` | not stated | 1 | walked | not stated |
| `7.1.7` | not stated | 0 | walked | not stated |
| `7.1.8` | not stated | 0 | walked | not stated |
| `7.1.9` | not stated | 0 | walked | not stated |
| `7.1.9.1` | not stated | 0 | walked | not stated |
| `7.1.10` | not stated | 0 | walked | not stated |
| `7.2` | not stated | 1 | walked | not stated |
| `7.2.1` | not stated | 0 | walked | not stated |
| `7.2.2` | not stated | 1 | walked | not stated |
| `7.2.3` | not stated | 0 | walked | not stated |
| `7.3` | not stated | 3 | walked | not stated |
| `7.3.1` | not stated | 0 | walked | not stated |
| `7.3.2` | not stated | 1 | walked | not stated |
| `7.3.3` | not stated | 0 | walked | not stated |
| `7.3.4` | not stated | 2 | walked | not stated |
| `7.3.5` | not stated | 0 | walked | not stated |
| `7.4` | not stated | 0 | walked | not stated |
| `7.4.1` | not stated | 0 | walked | not stated |
| `7.5` | not stated | 0 | walked | not stated |
| `7.5.1` | not stated | 0 | walked | not stated |
| `7.5.2` | not stated | 0 | walked | not stated |
| `7.5.3` | not stated | 1 | walked | not stated |
| `7.5.4` | not stated | 0 | walked | not stated |
| `7.5.4.1` | not stated | 0 | walked | not stated |
| `7.5.4.2` | not stated | 0 | walked | not stated |
| `7.5.4.3` | not stated | 0 | walked | not stated |
| `7.5.5` | not stated | 0 | walked | not stated |
| `7.5.6` | not stated | 0 | walked | not stated |
| `7.5.7` | not stated | 0 | walked | not stated |
| `7.5.8` | not stated | 0 | walked | not stated |
| `7.5.9` | not stated | 0 | walked | not stated |
| `7.6` | not stated | 0 | walked | not stated |
| `7.6.1` | not stated | 4 | walked | not stated |
| `7.6.2` | not stated | 0 | walked | not stated |
| `7.6.3` | not stated | 1 | walked | not stated |
| `7.6.4` | not stated | 3 | walked | not stated |
| `7.6.5` | not stated | 3 | walked | not stated |
| `7.6.6` | not stated | 0 | walked | not stated |
| `7.6.7` | not stated | 0 | walked | not stated |
| `7.6.8` | not stated | 0 | walked | not stated |
| `7.7` | not stated | 3 | walked | not stated |
| `7.7.1` | not stated | 0 | walked | not stated |
| `7.7.2` | not stated | 4 | walked | not stated |
| `7.7.3` | not stated | 0 | walked | not stated |
| `7.7.4` | not stated | 2 | walked | not stated |
| `7.7.5` | not stated | 1 | walked | not stated |
| `7.7.6` | not stated | 0 | walked | not stated |
| `7.7.7` | not stated | 0 | walked | not stated |
| `7.7.7.1` | not stated | 0 | walked | not stated |
| `7.7.7.2` | not stated | 0 | walked | not stated |
| `7.7.8` | not stated | 1 | walked | not stated |
| `7.7.9` | not stated | 1 | walked | not stated |
| `7.7.10` | not stated | 0 | walked | not stated |
| `7.8` | not stated | 0 | walked | not stated |
| `7.8.1` | not stated | 0 | walked | not stated |
| `7.8.2` | not stated | 5 | walked | not stated |
| `7.8.3` | not stated | 4 | walked | not stated |
| `7.8.3.1` | not stated | 0 | walked | not stated |
| `7.8.4` | not stated | 0 | walked | not stated |
| `7.8.5` | not stated | 1 | walked | not stated |
| `7.8.6` | not stated | 1 | walked | not stated |
| `7.8.7` | not stated | 0 | walked | not stated |
| `7.9` | not stated | 0 | walked | not stated |
| `7.9.1` | not stated | 0 | walked | not stated |
| `7.9.2` | not stated | 3 | walked | not stated |
| `7.9.2.1` | not stated | 0 | walked | not stated |
| `7.9.3` | not stated | 2 | walked | not stated |
| `7.9.4` | not stated | 1 | walked | not stated |
| `7.9.5` | not stated | 1 | walked | not stated |
| `7.9.6` | not stated | 0 | walked | not stated |
| `7.10` | not stated | 0 | walked | not stated |
| `7.10.1` | not stated | 0 | walked | not stated |
| `7.10.2` | not stated | 0 | walked | not stated |
| `7.10.3` | not stated | 0 | walked | not stated |
| `7.10.4` | not stated | 0 | walked | not stated |
| `7.11` | not stated | 0 | walked | not stated |
| `7.11.1` | not stated | 0 | walked | not stated |
| `7.11.2` | not stated | 0 | walked | not stated |
| `7.11.3` | not stated | 0 | walked | not stated |
| `7.11.4` | not stated | 0 | walked | not stated |
| `7.12` | not stated | 2 | walked | not stated |
| `7.12.1` | not stated | 0 | walked | not stated |
| `7.12.2` | not stated | 0 | walked | not stated |
| `7.13` | not stated | 0 | walked | not stated |
| `7.13.1` | not stated | 0 | walked | not stated |
| `7.13.2` | not stated | 0 | walked | not stated |
| `7.13.3` | not stated | 0 | walked | not stated |
| `7.13.4` | not stated | 0 | walked | not stated |
| `7.14` | not stated | 0 | walked | not stated |
| `7.14.1` | not stated | 0 | walked | not stated |
| `7.14.2` | not stated | 6 | walked | not stated |
| `7.14.2.1` | not stated | 0 | walked | not stated |
| `7.14.3` | not stated | 6 | walked | not stated |
| `7.14.3.1` | not stated | 0 | walked | not stated |
| `7.14.4` | not stated | 0 | walked | not stated |
| `7.14.5` | not stated | 0 | walked | not stated |
| `7.15` | not stated | 2 | walked | not stated |
| `7.15.1` | not stated | 0 | walked | not stated |
| `7.15.2` | not stated | 3 | walked | not stated |
| `7.15.3` | not stated | 0 | walked | not stated |
| `7.16` | not stated | 7 | walked | not stated |
| `7.16.1` | not stated | 0 | walked | not stated |
| `7.16.2` | not stated | 2 | walked | not stated |
| `7.16.3` | not stated | 0 | walked | not stated |
| `7.17` | not stated | 5 | walked | not stated |
| `7.17.1` | not stated | 0 | walked | not stated |
| `7.17.2` | not stated | 0 | walked | not stated |
| `7.17.3` | not stated | 0 | walked | not stated |
| `7.18` | not stated | 0 | walked | not stated |
| `7.18.1` | not stated | 0 | walked | not stated |
| `7.18.2` | not stated | 2 | walked | not stated |
| `7.18.3` | not stated | 0 | walked | not stated |
| `7.19` | not stated | 2 | walked | not stated |
| `7.19.1` | not stated | 0 | walked | not stated |
| `7.19.2` | not stated | 0 | walked | not stated |
| `7.19.2.1` | not stated | 0 | walked | not stated |
| `7.19.2.2` | not stated | 0 | walked | not stated |
| `7.19.3` | not stated | 0 | walked | not stated |
| `7.20` | not stated | 0 | walked | not stated |
| `7.20.1` | not stated | 2 | walked | not stated |
| `7.20.1.1` | not stated | 0 | walked | not stated |
| `7.20.2` | not stated | 2 | walked | not stated |
| `7.20.2.1` | not stated | 0 | walked | not stated |
| `7.20.3` | not stated | 3 | walked | not stated |
| `7.20.3.1` | not stated | 0 | walked | not stated |
| `7.20.3.2` | not stated | 3 | walked | not stated |
| `7.20.3.3` | not stated | 0 | walked | not stated |
| `7.21` | not stated | 0 | walked | not stated |
| `7.21.1` | not stated | 0 | walked | not stated |
| `7.21.2` | not stated | 2 | walked | not stated |
| `7.21.3` | not stated | 0 | walked | not stated |
| `7.21.4` | not stated | 0 | walked | not stated |
| `7.21.5` | not stated | 4 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 14 | walked | not stated |
| `8.2` | not stated | 0 | walked | not stated |
| `8.3` | not stated | 1 | walked | not stated |
| `8.3.1` | not stated | 7 | walked | not stated |
| `8.3.2` | not stated | 2 | walked | not stated |
| `8.3.3` | not stated | 2 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 3 | walked | not stated |
| `9.2` | not stated | 0 | walked | not stated |
| `9.2.1` | not stated | 0 | walked | not stated |
| `9.2.2` | not stated | 0 | walked | not stated |
| `9.2.3` | not stated | 0 | walked | not stated |
| `9.2.4` | not stated | 3 | walked | not stated |
| `9.2.4.1` | not stated | 0 | walked | not stated |
| `9.2.5` | not stated | 0 | walked | not stated |
| `9.3` | not stated | 0 | walked | not stated |
| `9.3.1` | not stated | 0 | walked | not stated |
| `9.3.2` | not stated | 1 | walked | not stated |
| `9.3.3` | not stated | 0 | walked | not stated |
| `9.3.4` | not stated | 1 | walked | not stated |
| `9.3.5` | not stated | 0 | walked | not stated |
| `9.4` | not stated | 0 | walked | not stated |
| `9.4.1` | not stated | 0 | walked | not stated |
| `9.4.2` | not stated | 0 | walked | not stated |
| `9.4.3` | not stated | 0 | walked | not stated |
| `9.4.4` | not stated | 3 | walked | not stated |
| `9.4.4.1` | not stated | 0 | walked | not stated |
| `9.4.5` | not stated | 0 | walked | not stated |
| `9.4.5.1` | not stated | 0 | walked | not stated |
| `9.4.6` | not stated | 0 | walked | not stated |
| `9.4.7` | not stated | 0 | walked | not stated |
| `9.5` | not stated | 0 | walked | not stated |
| `9.5.1` | not stated | 1 | walked | not stated |
| `9.5.2` | not stated | 0 | walked | not stated |
| `9.5.3` | not stated | 0 | walked | not stated |
| `9.6` | not stated | 0 | walked | not stated |
| `9.6.1` | not stated | 0 | walked | not stated |
| `9.6.2` | not stated | 0 | walked | not stated |
| `9.6.3` | not stated | 0 | walked | not stated |
| `9.6.4` | not stated | 5 | walked | not stated |
| `9.6.4.1` | not stated | 0 | walked | not stated |
| `9.6.4.2` | not stated | 3 | walked | not stated |
| `9.6.5` | not stated | 0 | walked | not stated |
| `9.7` | not stated | 2 | walked | not stated |
| `9.7.1` | not stated | 0 | walked | not stated |
| `9.7.2` | not stated | 0 | walked | not stated |
| `9.7.3` | not stated | 0 | walked | not stated |
| `9.7.4` | not stated | 2 | walked | not stated |
| `9.7.4.1` | not stated | 0 | walked | not stated |
| `9.7.4.2` | not stated | 3 | walked | not stated |
| `9.7.5` | not stated | 0 | walked | not stated |
| `9.8` | not stated | 0 | walked | not stated |
| `9.8.1` | not stated | 0 | walked | not stated |
| `9.8.2` | not stated | 0 | walked | not stated |
| `9.8.3` | not stated | 0 | walked | not stated |
| `9.9` | not stated | 4 | walked | not stated |
| `9.9.1` | not stated | 0 | walked | not stated |
| `9.9.2` | not stated | 3 | walked | not stated |
| `9.9.3` | not stated | 1 | walked | not stated |
| `9.9.4` | not stated | 0 | walked | not stated |
| `9.9.5` | not stated | 0 | walked | not stated |
| `9.9.6` | not stated | 0 | walked | not stated |
| `9.10` | not stated | 0 | walked | not stated |
| `9.10.1` | not stated | 0 | walked | not stated |
| `9.10.2` | not stated | 2 | walked | not stated |
| `9.10.3` | not stated | 1 | walked | not stated |
| `9.10.4` | not stated | 0 | walked | not stated |
| `9.10.5` | not stated | 0 | walked | not stated |
| `9.11` | not stated | 0 | walked | not stated |
| `9.11.1` | not stated | 0 | walked | not stated |
| `9.11.2` | not stated | 0 | walked | not stated |
| `9.11.3` | not stated | 0 | walked | not stated |
| `9.11.4` | not stated | 0 | walked | not stated |
| `9.12` | not stated | 1 | walked | not stated |
| `9.12.1` | not stated | 0 | walked | not stated |
| `9.12.2` | not stated | 0 | walked | not stated |
| `9.12.3` | not stated | 0 | walked | not stated |
| `9.12.4` | not stated | 0 | walked | not stated |
| `9.13` | not stated | 3 | walked | not stated |
| `9.13.1` | not stated | 0 | walked | not stated |
| `9.13.2` | not stated | 1 | walked | not stated |
| `9.13.3` | not stated | 0 | walked | not stated |
| `9.13.4` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `10.1` | not stated | 0 | walked | not stated |
| `10.1.1` | not stated | 0 | walked | not stated |
| `10.1.1.1` | not stated | 0 | walked | not stated |
| `10.2` | not stated | 0 | walked | not stated |
| `10.2.1` | not stated | 0 | walked | not stated |
| `10.2.1.1` | not stated | 0 | walked | not stated |
| `10.3` | not stated | 0 | walked | not stated |
| `10.3.1` | not stated | 0 | walked | not stated |
| `10.3.1.1` | not stated | 0 | walked | not stated |
| `10.4` | not stated | 0 | walked | not stated |
| `10.4.1` | not stated | 0 | walked | not stated |
| `10.4.1.1` | not stated | 0 | walked | not stated |
| `10.4.2` | not stated | 0 | walked | not stated |
| `10.4.2.1` | not stated | 0 | walked | not stated |
| `10.5` | not stated | 0 | walked | not stated |
| `10.5.1` | not stated | 0 | walked | not stated |
| `10.5.1.1` | not stated | 0 | walked | not stated |
| `10.6` | not stated | 0 | walked | not stated |
| `10.6.1` | not stated | 0 | walked | not stated |
| `10.6.1.1` | not stated | 0 | walked | not stated |
| `11` | not stated | 8 | walked | not stated |
| `12` | not stated | 3 | walked | not stated |
| `13` | not stated | 0 | walked | not stated |
| `13.1` | not stated | 3 | walked | not stated |
| `13.1.1` | not stated | 0 | walked | not stated |
| `14` | not stated | 0 | walked | not stated |
| `15` | not stated | 0 | walked | not stated |
| `15.1` | not stated | 1 | walked | not stated |
| `15.2` | not stated | 1 | walked | not stated |
| `15.3` | not stated | 1 | walked | not stated |
| `15.4` | not stated | 1 | walked | not stated |
| `15.5` | not stated | 1 | walked | not stated |
| `15.6` | not stated | 1 | walked | not stated |
| `15.7` | not stated | 1 | walked | not stated |
| `16` | not stated | 0 | skipped (iana) | IANA registration actions for the YANG module names registry, carrying no obligation on an implementation |
| `17` | not stated | 0 | walked | not stated |
| `18` | boilerplate and bibliography carrying no protocol obligation | 0 | skipped (references) | boilerplate and bibliography carrying no protocol obligation |
| `18.1` | boilerplate and bibliography carrying no protocol obligation | 0 | skipped (references) | boilerplate and bibliography carrying no protocol obligation |
| `18.2` | boilerplate and bibliography carrying no protocol obligation | 0 | skipped (references) | boilerplate and bibliography carrying no protocol obligation |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `5.3:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | the sentence delegates the obligation to the IANA registration procedure of the earlier YANG specification it cites, which binds IANA and the RFC stream publication process; ze publishes no module in an RFC stream | XML namespaces for modules published in RFC streams [RFC4844] MUST be assigned by IANA; see Section 14 in [RFC6020]. |
| `5.6.4:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the NETCONF protocol binding of YANG, and Section 1 makes that binding one of several: "YANG has been used or proposed to be used for other protocols". ze serves its YANG-modeled configuration over gNMI and its own CLI and implements no NETCONF server, no ietf-yang-library module and no rpc-error envelope: no Go file under internal/ names NETCONF except a port-name table (internal/core/portname/services_table.go) | A NETCONF server MUST announce the modules it implements (see Section 5.6.5) by implementing the YANG module "ietf-yang-library" defined in [RFC7895] and listing all implemented modules in the "/modules-state/module" list. |
| `5.6.4:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the NETCONF protocol binding of YANG, and Section 1 makes that binding one of several: "YANG has been used or proposed to be used for other protocols". ze serves its YANG-modeled configuration over gNMI and its own CLI and implements no NETCONF server, no ietf-yang-library module and no rpc-error envelope: no Go file under internal/ names NETCONF except a port-name table (internal/core/portname/services_table.go) | The server also MUST advertise the following capability in the <hello> message (line breaks and whitespaces are used for formatting reasons only): |
| `5.6.4:3` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the NETCONF protocol binding of YANG, and Section 1 makes that binding one of several: "YANG has been used or proposed to be used for other protocols". ze serves its YANG-modeled configuration over gNMI and its own CLI and implements no NETCONF server, no ietf-yang-library module and no rpc-error envelope: no Go file under internal/ names NETCONF except a port-name table (internal/core/portname/services_table.go) | This parameter MUST be present. |
| `5.6.4:4` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the NETCONF protocol binding of YANG, and Section 1 makes that binding one of several: "YANG has been used or proposed to be used for other protocols". ze serves its YANG-modeled configuration over gNMI and its own CLI and implements no NETCONF server, no ietf-yang-library module and no rpc-error envelope: no Go file under internal/ names NETCONF except a port-name table (internal/core/portname/services_table.go) | This parameter MUST be present. |
| `5.6.5:3` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the NETCONF protocol binding of YANG, and Section 1 makes that binding one of several: "YANG has been used or proposed to be used for other protocols". ze serves its YANG-modeled configuration over gNMI and its own CLI and implements no NETCONF server, no ietf-yang-library module and no rpc-error envelope: no Go file under internal/ names NETCONF except a port-name table (internal/core/portname/services_table.go) | If a server implements a module A that imports a module C without specifying the revision date of module C and the server does not implement C (e.g., if C only defines some typedefs), the server MUST list module C in the "/modules-state/module" list from "ietf-yang-library" [RFC7895], and it MUST set the leaf "conformance-type" to "import" for this module. |
| `5.6.5:4` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the NETCONF protocol binding of YANG, and Section 1 makes that binding one of several: "YANG has been used or proposed to be used for other protocols". ze serves its YANG-modeled configuration over gNMI and its own CLI and implements no NETCONF server, no ietf-yang-library module and no rpc-error envelope: no Go file under internal/ names NETCONF except a port-name table (internal/core/portname/services_table.go) | If a server lists a module C in the "/modules-state/module" list from "ietf-yang-library" and there are other modules Ms listed that import C without specifying the revision date of module C, the server MUST use the definitions from the most recent revision of C listed for modules Ms. |
| `7.1:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | the sentence delegates the obligation to the IANA registration procedure of the earlier YANG specification it cites, which binds IANA and the RFC stream publication process; ze publishes no module in an RFC stream | Names of modules published in RFC streams [RFC4844] MUST be assigned by IANA; see Section 14 in [RFC6020]. |
| `7.2:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | the sentence delegates the obligation to the IANA registration procedure of the earlier YANG specification it cites, which binds IANA and the RFC stream publication process; ze publishes no module in an RFC stream | Names of submodules published in RFC streams [RFC4844] MUST be assigned by IANA; see Section 14 in [RFC6020]. |
| `7.7.8:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the XML encoding of YANG data, and Section 1 makes that encoding one of several: "Further, encodings other than XML have been proposed". ze encodes configuration in its own YANG-modeled tree and in JSON, and emits no XML instance document, so the XML element order, the insert attributes and the XML prefix rules have no producer here | The XML elements representing leaf-list entries MUST appear in the order specified by the user if the leaf-list is "ordered-by user"; otherwise, the order is implementation dependent. |
| `7.7.9:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the NETCONF protocol binding of YANG, and Section 1 makes that binding one of several: "YANG has been used or proposed to be used for other protocols". ze serves its YANG-modeled configuration over gNMI and its own CLI and implements no NETCONF server, no ietf-yang-library module and no rpc-error envelope: no Go file under internal/ names NETCONF except a port-name table (internal/core/portname/services_table.go) | If the value is "before" or "after", the "value" attribute MUST also be used to specify an existing entry in the leaf-list. |
| `7.8.5:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the XML encoding of YANG data, and Section 1 makes that encoding one of several: "Further, encodings other than XML have been proposed". ze encodes configuration in its own YANG-modeled tree and in JSON, and emits no XML instance document, so the XML element order, the insert attributes and the XML prefix rules have no producer here | The XML elements representing list entries MUST appear in the order specified by the user if the list is "ordered-by user"; otherwise, the order is implementation dependent. |
| `7.8.6:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the NETCONF protocol binding of YANG, and Section 1 makes that binding one of several: "YANG has been used or proposed to be used for other protocols". ze serves its YANG-modeled configuration over gNMI and its own CLI and implements no NETCONF server, no ietf-yang-library module and no rpc-error envelope: no Go file under internal/ names NETCONF except a port-name table (internal/core/portname/services_table.go) | If the value is "before" or "after", the "key" attribute MUST also be used, to specify an existing element in the list. |
| `7.9.5:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the XML encoding of YANG data, and Section 1 makes that encoding one of several: "Further, encodings other than XML have been proposed". ze encodes configuration in its own YANG-modeled tree and in JSON, and emits no XML instance document, so the XML element order, the insert attributes and the XML prefix rules have no producer here | The child nodes of the selected "case" statement MUST be encoded in the same order as they are defined in the "case" statement if they are part of an RPC or action input or output parameter definition. |
| `7.15.2:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the XML encoding of YANG data, and Section 1 makes that encoding one of several: "Further, encodings other than XML have been proposed". ze encodes configuration in its own YANG-modeled tree and in JSON, and emits no XML instance document, so the XML element order, the insert attributes and the XML prefix rules have no producer here | It MUST contain all containers and list nodes in the direct path from the top level down to the list or container containing the action. |
| `7.15.2:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the XML encoding of YANG data, and Section 1 makes that encoding one of several: "Further, encodings other than XML have been proposed". ze encodes configuration in its own YANG-modeled tree and in JSON, and emits no XML instance document, so the XML element order, the insert attributes and the XML prefix rules have no producer here | For lists, all key leafs MUST also be included. |
| `7.15.2:3` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the NETCONF protocol binding of YANG, and Section 1 makes that binding one of several: "YANG has been used or proposed to be used for other protocols". ze serves its YANG-modeled configuration over gNMI and its own CLI and implements no NETCONF server, no ietf-yang-library module and no rpc-error envelope: no Go file under internal/ names NETCONF except a port-name table (internal/core/portname/services_table.go) | If more than one action is present in the <rpc>, the server MUST reply with a "bad-element" <error-tag> in the <rpc-error>. |
| `7.16.2:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the XML encoding of YANG data, and Section 1 makes that encoding one of several: "Further, encodings other than XML have been proposed". ze encodes configuration in its own YANG-modeled tree and in JSON, and emits no XML instance document, so the XML element order, the insert attributes and the XML prefix rules have no producer here | It MUST contain all containers and list nodes from the top level down to the list or container containing the notification. |
| `7.16.2:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the XML encoding of YANG data, and Section 1 makes that encoding one of several: "Further, encodings other than XML have been proposed". ze encodes configuration in its own YANG-modeled tree and in JSON, and emits no XML instance document, so the XML element order, the insert attributes and the XML prefix rules have no producer here | For lists, all key leafs MUST also be included. |
| `8.3.1:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the XML encoding of YANG data, and Section 1 makes that encoding one of several: "Further, encodings other than XML have been proposed". ze encodes configuration in its own YANG-modeled tree and in JSON, and emits no XML instance document, so the XML element order, the insert attributes and the XML prefix rules have no producer here | When content arrives in RPC payloads, it MUST be well-formed XML, following the hierarchy and content rules defined by the set of models the server implements. |
| `8.3.1:6` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the NETCONF protocol binding of YANG, and Section 1 makes that binding one of several: "YANG has been used or proposed to be used for other protocols". ze serves its YANG-modeled configuration over gNMI and its own CLI and implements no NETCONF server, no ietf-yang-library module and no rpc-error envelope: no Go file under internal/ names NETCONF except a port-name table (internal/core/portname/services_table.go) | o For insert handling, if the values for the attributes "before" and "after" are not valid for the type of the appropriate key leafs, the server MUST reply with a "bad-attribute" <error-tag> in the <rpc-error>. |
| `8.3.1:7` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the NETCONF protocol binding of YANG, and Section 1 makes that binding one of several: "YANG has been used or proposed to be used for other protocols". ze serves its YANG-modeled configuration over gNMI and its own CLI and implements no NETCONF server, no ietf-yang-library module and no rpc-error envelope: no Go file under internal/ names NETCONF except a port-name table (internal/core/portname/services_table.go) | o If the attributes "before" and "after" appear in any element that is not a list whose "ordered-by" property is "user", the server MUST reply with an "unknown-attribute" <error-tag> in the <rpc-error>. |
| `9.1:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the XML encoding of YANG data, and Section 1 makes that encoding one of several: "Further, encodings other than XML have been proposed". ze encodes configuration in its own YANG-modeled tree and in JSON, and emits no XML instance document, so the XML element order, the insert attributes and the XML prefix rules have no producer here | When a server sends XML-encoded data, it MUST use the canonical form defined in this section. |
| `9.13.2:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the XML encoding of YANG data, and Section 1 makes that encoding one of several: "Further, encodings other than XML have been proposed". ze encodes configuration in its own YANG-modeled tree and in JSON, and emits no XML instance document, so the XML element order, the insert attributes and the XML prefix rules have no producer here | All node names in an instance-identifier value MUST be qualified with explicit namespace prefixes, and these prefixes MUST be declared in the XML namespace scope in the instance-identifier's XML element. |
| `12:3` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the NETCONF protocol binding of YANG, and Section 1 makes that binding one of several: "YANG has been used or proposed to be used for other protocols". ze serves its YANG-modeled configuration over gNMI and its own CLI and implements no NETCONF server, no ietf-yang-library module and no rpc-error envelope: no Go file under internal/ names NETCONF except a port-name table (internal/core/portname/services_table.go) | In such cases, a NETCONF server MUST advertise both modules using the rules defined in Section 5.6.4, and SHOULD advertise module A and the latest revision of module B that is specified with YANG version 1 according to the rules defined in [RFC6020]. |
| `13.1:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the YIN syntax, which Section 4.1 presents as a translation a module can be given: "YANG modules can be translated into an equivalent XML syntax called YANG Independent Notation (YIN)". ze parses YANG text with goyang and never produces or consumes YIN | The names of all YIN elements MUST be properly qualified with their namespaces (as specified above) using the standard mechanisms of [XML-NAMES], i.e., "xmlns" and "xmlns:xxx" attributes. |
| `13.1:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the YIN syntax, which Section 4.1 presents as a translation a module can be given: "YANG modules can be translated into an equivalent XML syntax called YANG Independent Notation (YIN)". ze parses YANG text with goyang and never produces or consumes YIN | o If the argument is represented as an element, it MUST be the first child of the keyword element. |
| `13.1:3` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the YIN syntax, which Section 4.1 presents as a translation a module can be given: "YANG modules can be translated into an equivalent XML syntax called YANG Independent Notation (YIN)". ze parses YANG text with goyang and never produces or consumes YIN | Substatements of a YANG statement are represented as (additional) children of the keyword element, and their relative order MUST be the same as the order of substatements in YANG. |
| `15.1:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the NETCONF protocol binding of YANG, and Section 1 makes that binding one of several: "YANG has been used or proposed to be used for other protocols". ze serves its YANG-modeled configuration over gNMI and its own CLI and implements no NETCONF server, no ietf-yang-library module and no rpc-error envelope: no Go file under internal/ names NETCONF except a port-name table (internal/core/portname/services_table.go) | If a NETCONF operation would result in configuration data where a "unique" constraint is invalidated, the following error MUST be returned: |
| `15.2:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the NETCONF protocol binding of YANG, and Section 1 makes that binding one of several: "YANG has been used or proposed to be used for other protocols". ze serves its YANG-modeled configuration over gNMI and its own CLI and implements no NETCONF server, no ietf-yang-library module and no rpc-error envelope: no Go file under internal/ names NETCONF except a port-name table (internal/core/portname/services_table.go) | If a NETCONF operation would result in configuration data where a list or a leaf-list would have too many entries, the following error MUST be returned: |
| `15.3:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the NETCONF protocol binding of YANG, and Section 1 makes that binding one of several: "YANG has been used or proposed to be used for other protocols". ze serves its YANG-modeled configuration over gNMI and its own CLI and implements no NETCONF server, no ietf-yang-library module and no rpc-error envelope: no Go file under internal/ names NETCONF except a port-name table (internal/core/portname/services_table.go) | If a NETCONF operation would result in configuration data where a list or a leaf-list would have too few entries, the following error MUST be returned: |
| `15.4:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the NETCONF protocol binding of YANG, and Section 1 makes that binding one of several: "YANG has been used or proposed to be used for other protocols". ze serves its YANG-modeled configuration over gNMI and its own CLI and implements no NETCONF server, no ietf-yang-library module and no rpc-error envelope: no Go file under internal/ names NETCONF except a port-name table (internal/core/portname/services_table.go) | If a NETCONF operation would result in configuration data where the restrictions imposed by a "must" statement are violated, the following error MUST be returned, unless a specific "error-app-tag" substatement is present for the "must" statement. |
| `15.5:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the NETCONF protocol binding of YANG, and Section 1 makes that binding one of several: "YANG has been used or proposed to be used for other protocols". ze serves its YANG-modeled configuration over gNMI and its own CLI and implements no NETCONF server, no ietf-yang-library module and no rpc-error envelope: no Go file under internal/ names NETCONF except a port-name table (internal/core/portname/services_table.go) | If a NETCONF operation would result in configuration data where a leaf of type "instance-identifier" or "leafref" marked with require-instance "true" refers to an instance that does not exist, the following error MUST be returned: |
| `15.6:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the NETCONF protocol binding of YANG, and Section 1 makes that binding one of several: "YANG has been used or proposed to be used for other protocols". ze serves its YANG-modeled configuration over gNMI and its own CLI and implements no NETCONF server, no ietf-yang-library module and no rpc-error envelope: no Go file under internal/ names NETCONF except a port-name table (internal/core/portname/services_table.go) | If a NETCONF operation would result in configuration data where no nodes exists in a mandatory choice, the following error MUST be returned: |
| `15.7:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the NETCONF protocol binding of YANG, and Section 1 makes that binding one of several: "YANG has been used or proposed to be used for other protocols". ze serves its YANG-modeled configuration over gNMI and its own CLI and implements no NETCONF server, no ietf-yang-library module and no rpc-error envelope: no Go file under internal/ names NETCONF except a port-name table (internal/core/portname/services_table.go) | If the "insert" and "key" or "value" attributes are used in an <edit-config> for a list or leaf-list node and the "key" or "value" refers to an instance that does not exist, the following error MUST be returned: |

## Superseded

No document obsoletes RFC 7950, so its obligations are stated where they were written.
