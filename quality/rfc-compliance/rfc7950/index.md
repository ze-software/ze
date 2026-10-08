# RFC 7950 - The YANG 1.1 Data Modeling Language

Partial. Every requirement this repository extracted from RFC 7950, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 28.6% | 26 of 91 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 1.1% | 1 of 91 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 91 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 91 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 75.9% | 60 of 79 tagged units, 0 escaped and 10 lapsed | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 91 | of 91 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 5 | of 91 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 5.5% | 5 of 91 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 91 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 91 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 64.8% | 59 of 91 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 38 | of 91 gated MUSTs judged | 19 weak, wrong or unimplemented, 1 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 91 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 91 |
| Gated MUST-level | 91 |
| Not applicable, so out of scope | 5 |
| Declared gaps | 59 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 79 |
| Tagged units | 79 |
| Recorded audit verdicts | 38 |
| Discrimination records | 70 |
| Summary | `rfc/short/rfc7950.md` |
| Requirement shard | `rfc/requirements/rfc7950.md` |
| RFC text | `rfc/full/rfc7950.txt` |

## Enrolment

Enrolled: The YANG 1.1 Data Modeling Language (ze config validator). The extraction walk of 2026-09-21 read all 227 MUST sites and rewrote this ledger: it added 67 MUST rows the checklist did not carry, covering module and submodule structure, identifier and prefix scoping, typedef and leaf definition rules, list keys and unique constraints, choice, grouping, augment, identity, feature, deviation, status and when rules, the Section 8.1 validity conditions and the Section 8.3 enforcement windows, the built-in type restrictions of Section 9, the module update rules of Section 11 and the YANG 1/1.1 coexistence rules of Section 12. On 2026-09-21 twelve of the added rows gained positive and negative tags (seven at the loader boundary in internal/component/config/yang/rfc7950_loader_test.go, decimal64 and default validation in the validator and the schema builder), one is single-polarity, and the forty-eight left carry {gap} rows naming their specs. Of the nine rows that stood before the walk, four are met with positive+negative tags in internal/component/config: 8.3.1-1 (a value violating a range, length, or pattern restriction is an error), 9.6-1 (an enum value must be one of the defined enums), 9.12-1 (a union value must match at least one member type), and 7.6.5-1 (a missing mandatory node is an error). Five are {not-applicable}: 7.5.3-1 (ze uses no must XPath statements and enforces cross-field constraints with Go validators instead), 9.2.4-1 (derived-type narrowing is resolved by goyang at module-processing time with no runtime surface), 9.4.5-1 (no ze type carries multiple pattern statements), x-1 (ze's models declare no yang-version and default to YANG 1.0, using no 1.1-only construct), and x-2 (no cross-revision position-stability tooling; critical enum values are pinned with explicit value statements). The walk excluded 32 sites as the NETCONF protocol binding, the XML encoding and the YIN syntax, none of which ze uses: ze serves its YANG-modeled config over gNMI and its own CLI, and no Go file in internal/ references NETCONF except a port-name table. Three further sites delegate module-name and namespace assignment to the IANA procedure of the earlier YANG specification they cite.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Config schema loaded from Ze's own YANG modules through goyang
- value validation for string (length, pattern), the integer types (range, with negative bounds), decimal64 (lexical form, fraction-digits, range), boolean, enumeration and union
- mandatory leaves reported as a warning, not refused
- min-elements and max-elements
- leaf and typedef defaults validated against their type at schema build
- the loader refuses a bad escape, an unprefixed external reference, a leaf or typedef without a type, an extension with a non-YANG substatement, a descending or non-numeric range, a negative or descending length, a decimal64 without fraction-digits, and an enum value that is duplicated, beyond int32 or missing after the maximum.


**What the ledger says remains**

Fifty-nine MUST rows carry {gap}, each naming its spec. Ze's loader adds no check of its own where goyang accepts a violating module (circular imports, duplicate typedefs and groupings, a shared import prefix, a typedef named after a built-in type, a shared child name across choice cases, an action or notification under a keyless list, an augment adding a duplicate, an overlapping range, an empty enumeration or union, a restricted enumeration adding a name; a self-referencing grouping or identity and an augment targeting a leaf crash goyang instead of being refused): [`plan/pre-release/spec-config-yang-loader-structural-checks.md`](https://github.com/ze-software/ze/blob/main/plan/pre-release/spec-config-yang-loader-structural-checks.md). The validator evaluates no when, unique or choice, requires no mandatory leaf below a non-presence container absent from the data or inside a case, and grades a missing mandatory leaf a warning rather than a refusal at load, validate and commit: [`plan/pre-release/spec-config-yang-when-unique-choice.md`](https://github.com/ze-software/ze/blob/main/plan/pre-release/spec-config-yang-when-unique-choice.md). A config list without a key is tolerated, leaf-list values are not deduplicated and leaf-list defaults are not applied: [`plan/pre-release/spec-config-yang-list-key-leaf-list.md`](https://github.com/ze-software/ze/blob/main/plan/pre-release/spec-config-yang-list-key-leaf-list.md). RPC input and output mandatory and defaults are read by no invocation path: [`plan/pre-release/spec-config-yang-rpc-mandatory-defaults.md`](https://github.com/ze-software/ze/blob/main/plan/pre-release/spec-config-yang-rpc-mandatory-defaults.md). No repository check reads module namespaces or revisions: [`plan/spec-config-yang-authoring-checks.md`](https://github.com/ze-software/ze/blob/main/plan/spec-config-yang-authoring-checks.md). Ze offers no bits, leafref, identityref or instance-identifier type, no feature or if-feature, no deviation, no yang-version declaration and no submodule: one plan/spec-config-yang-*.md each, for the owner to decline or schedule.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 26 | one part of the gated population |
| Annotated (including scoped evidence) | 65 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **91** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (26):** [`RFC7950-9.6-1`](#rfc7950-9.6-1), [`RFC7950-9.12-1`](#rfc7950-9.12-1), [`RFC7950-7.6.5-1`](#rfc7950-7.6.5-1), [`RFC7950-6.1.3-1`](#rfc7950-6.1.3-1), [`RFC7950-6.5-1`](#rfc7950-6.5-1), [`RFC7950-7.3.4-1`](#rfc7950-7.3.4-1), [`RFC7950-7.6.1-1`](#rfc7950-7.6.1-1), [`RFC7950-7.6.1-3`](#rfc7950-7.6.1-3), [`RFC7950-7.6.3-1`](#rfc7950-7.6.3-1), [`RFC7950-7.6.4-2`](#rfc7950-7.6.4-2), [`RFC7950-7.7.4-1`](#rfc7950-7.7.4-1), [`RFC7950-7.7.5-1`](#rfc7950-7.7.5-1), [`RFC7950-7.19-1`](#rfc7950-7.19-1), [`RFC7950-8.1-3`](#rfc7950-8.1-3), [`RFC7950-8.1-2`](#rfc7950-8.1-2), [`RFC7950-8.3-1`](#rfc7950-8.3-1), [`RFC7950-8.3.3-1`](#rfc7950-8.3.3-1), [`RFC7950-9.1-1`](#rfc7950-9.1-1), [`RFC7950-9.3.1-1`](#rfc7950-9.3.1-1), [`RFC7950-9.3.4-1`](#rfc7950-9.3.4-1), [`RFC7950-9.4.4-1`](#rfc7950-9.4.4-1), [`RFC7950-9.5.1-1`](#rfc7950-9.5.1-1), [`RFC7950-9.6.4.2-1`](#rfc7950-9.6.4.2-1), [`RFC7950-7.3.2-1`](#rfc7950-7.3.2-1), [`RFC7950-7.9.2-2`](#rfc7950-7.9.2-2), [`RFC7950-9.4.4-2`](#rfc7950-9.4.4-2)

**Annotated (including scoped evidence) (65):** [`RFC7950-7.6.5-2`](#rfc7950-7.6.5-2), [`RFC7950-7.6.5-3`](#rfc7950-7.6.5-3), [`RFC7950-7.5.3-1`](#rfc7950-7.5.3-1), [`RFC7950-9.2.4-1`](#rfc7950-9.2.4-1), [`RFC7950-9.4.5-1`](#rfc7950-9.4.5-1), [`RFC7950-x-1`](#rfc7950-x-1), [`RFC7950-x-2`](#rfc7950-x-2), [`RFC7950-5.1-1`](#rfc7950-5.1-1), [`RFC7950-5.3-1`](#rfc7950-5.3-1), [`RFC7950-5.5-1`](#rfc7950-5.5-1), [`RFC7950-5.6.5-1`](#rfc7950-5.6.5-1), [`RFC7950-6.2-1`](#rfc7950-6.2-1), [`RFC7950-6.2.1-1`](#rfc7950-6.2.1-1), [`RFC7950-6.3.1-1`](#rfc7950-6.3.1-1), [`RFC7950-6.4-1`](#rfc7950-6.4-1), [`RFC7950-6.4-2`](#rfc7950-6.4-2), [`RFC7950-7.1.4-1`](#rfc7950-7.1.4-1), [`RFC7950-7.3-1`](#rfc7950-7.3-1), [`RFC7950-7.6.1-2`](#rfc7950-7.6.1-2), [`RFC7950-7.6.1-4`](#rfc7950-7.6.1-4), [`RFC7950-7.7-1`](#rfc7950-7.7-1), [`RFC7950-7.7.2-1`](#rfc7950-7.7.2-1), [`RFC7950-7.8.2-1`](#rfc7950-7.8.2-1), [`RFC7950-7.8.3-1`](#rfc7950-7.8.3-1), [`RFC7950-7.9.2-1`](#rfc7950-7.9.2-1), [`RFC7950-7.9.3-1`](#rfc7950-7.9.3-1), [`RFC7950-7.9.4-1`](#rfc7950-7.9.4-1), [`RFC7950-7.12-1`](#rfc7950-7.12-1), [`RFC7950-7.14.2-1`](#rfc7950-7.14.2-1), [`RFC7950-7.14.3-1`](#rfc7950-7.14.3-1), [`RFC7950-7.15-1`](#rfc7950-7.15-1), [`RFC7950-7.16-1`](#rfc7950-7.16-1), [`RFC7950-7.17-1`](#rfc7950-7.17-1), [`RFC7950-7.18.2-1`](#rfc7950-7.18.2-1), [`RFC7950-7.20.1-1`](#rfc7950-7.20.1-1), [`RFC7950-7.20.2-1`](#rfc7950-7.20.2-1), [`RFC7950-7.20.3-1`](#rfc7950-7.20.3-1), [`RFC7950-7.20.3.2-1`](#rfc7950-7.20.3.2-1), [`RFC7950-7.21.2-1`](#rfc7950-7.21.2-1), [`RFC7950-7.21.5-1`](#rfc7950-7.21.5-1), [`RFC7950-8.1-1`](#rfc7950-8.1-1), [`RFC7950-8.1-4`](#rfc7950-8.1-4), [`RFC7950-8.3.1-2`](#rfc7950-8.3.1-2), [`RFC7950-8.3.2-1`](#rfc7950-8.3.2-1), [`RFC7950-8.3.3-2`](#rfc7950-8.3.3-2), [`RFC7950-9.2.4-2`](#rfc7950-9.2.4-2), [`RFC7950-9.6.4-1`](#rfc7950-9.6.4-1), [`RFC7950-9.6.4-2`](#rfc7950-9.6.4-2), [`RFC7950-9.7-1`](#rfc7950-9.7-1), [`RFC7950-9.7.4-1`](#rfc7950-9.7.4-1), [`RFC7950-9.7.4.2-1`](#rfc7950-9.7.4.2-1), [`RFC7950-9.9-1`](#rfc7950-9.9-1), [`RFC7950-9.9.2-1`](#rfc7950-9.9.2-1), [`RFC7950-9.9.3-1`](#rfc7950-9.9.3-1), [`RFC7950-9.10.2-1`](#rfc7950-9.10.2-1), [`RFC7950-9.10.3-1`](#rfc7950-9.10.3-1), [`RFC7950-9.12-2`](#rfc7950-9.12-2), [`RFC7950-9.13-1`](#rfc7950-9.13-1), [`RFC7950-11-1`](#rfc7950-11-1), [`RFC7950-11-2`](#rfc7950-11-2), [`RFC7950-11-3`](#rfc7950-11-3), [`RFC7950-7.2.2-1`](#rfc7950-7.2.2-1), [`RFC7950-7.21.5-2`](#rfc7950-7.21.5-2), [`RFC7950-9.1-2`](#rfc7950-9.1-2), [`RFC7950-12-1`](#rfc7950-12-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7950-9.6-1` | The enumeration built-in type represents values from a set of assigned names. (§9.6) | MUST | 9.6 | **positive:** `unit/verify` [`TestISISAuthAlgorithmEnumAcceptsAll`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_isis_auth_algorithm_enum_test.go#L64). **positive:** `unit/verify` [`TestRadiusAuthMethodEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_radius_auth_method_enum_test.go#L53). **positive:** `unit/verify` [`TestValidateTree_AddPathDirectionEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L618). **positive:** `unit/verify` [`TestValidateTree_FamilyModeEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L552). **negative:** `unit/verify` [`TestISISAuthAlgorithmEnumAcceptsAll`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_isis_auth_algorithm_enum_test.go#L81). **negative:** `unit/verify` [`TestRadiusAuthMethodEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_radius_auth_method_enum_test.go#L64). **negative:** `unit/verify` [`TestValidateTree_AddPathDirectionEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L619). **negative:** `unit/verify` [`TestValidateTree_FamilyModeEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L553) |
| `RFC7950-9.12-1` | The union built-in type represents a value that corresponds to one of its member types. (§9.12) | MUST | 9.12 | **positive:** `unit/verify` [`TestRFC7950UnionAcceptsEveryMemberType`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_union_test.go#L32). **positive:** `unit/verify` [`TestValidateTree_ValidConfig`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L46). **negative:** `unit/verify` [`TestValidateTree_UnionViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L366) |
| `RFC7950-7.6.5-1` | If "mandatory" is "true", the behavior of the constraint depends on the type of the leaf's closest ancestor node in the schema tree that is not a non-presence container (see Section 7.5.1): o If no such ancestor exists in the schema tree, the leaf MUST exist. (§7.6.5) | MUST | 7.6.5 | **positive:** `unit/verify` [`TestValidator_MandatoryField`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L934). **negative:** `unit/verify` [`TestValidateTree_MandatoryMissing`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L338). **negative:** `unit/verify` [`TestValidator_MandatoryField`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L935) |
| `RFC7950-7.6.5-2` | Otherwise, if this ancestor is a case node, the leaf MUST exist if any node from the case exists in the data tree. (§7.6.5) | MUST | 7.6.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang keeps a case's leaves under the choice and case entries, and walkTree looks each child up only in the container's own Dir, so a mandatory leaf inside a case is never required whatever nodes of that case exist; plan/pre-release/spec-config-yang-when-unique-choice.md |
| `RFC7950-7.6.5-3` | Otherwise, the leaf MUST exist if the ancestor node exists in the data tree. (§7.6.5) | MUST | 7.6.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** walkTree checks a mandatory leaf only inside containers present in the data, so a mandatory leaf below an absent non-presence container is not required even when its ancestor list entry or presence container exists; plan/pre-release/spec-config-yang-when-unique-choice.md |
| `RFC7950-7.5.3-1` | All such constraints MUST evaluate to "true" for the data to be valid. (§7.5.3) | MUST | 7.5.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze uses zero YANG must statements (a grep of the .yang models finds must only inside a description); it parses via goyang which evaluates no XPath, and enforces cross-field constraints with Go ze:validate functions (internal/component/config/yang/validator.go:756 applyCustomValidators) instead, so there is no must-XPath code path |
| `RFC7950-9.2.4-1` | If a range restriction is applied to a type that is already range-restricted, the new restriction MUST be equally limiting or more limiting, i.e., raising the lower bounds, reducing the upper bounds, removing explicit values or ranges, or splitting ranges into multiple ranges with intermediate gaps. (§9.2.4) | MUST | 9.2.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this is a schema-authoring-time narrowing constraint resolved by goyang during module processing (internal/component/config/yang/loader.go); ze authors valid narrowings and has no runtime enforcement surface for it |
| `RFC7950-9.4.5-1` | If the type has multiple "pattern" statements, the expressions are ANDed together, i.e., all such expressions have to match. (§9.4.5) | MUST | 9.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no ze type carries two or more pattern statements (a scan of the .yang models finds none); the validator loop (internal/component/config/yang/validator.go:268) AND-combines patterns if present, but the multi-pattern construct is unused |
| `RFC7950-x-1` | The "yang-version" statement specifies which version of the YANG language was used in developing the module. The statement's argument is a string. It MUST contain the value "1.1" for YANG modules defined based on this specification. (§7.1.2) | MUST | 7.1.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's YANG models declare no yang-version statement and default to YANG 1.0 in goyang; they use no YANG-1.1-only construct that would require the 1.1 declaration, so the version-declaration obligation has no applicable module |
| `RFC7950-x-2` | An "enumeration" type may have new enums added, provided the old enums's values do not change. (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's modules carry no consumed revision statements and there is no cross-revision position-stability tooling; ze pins the values that matter with explicit value statements (e.g. role.yang, ze-types.yang afi/safi) but enforces no automated cross-revision check |
| `RFC7950-5.1-1` | Within a server, all module names MUST be unique. A module uses the "include" statement to list all its submodules. A module, or submodule belonging to that module, can reference definitions in the module and all submodules included by the module. A module or submodule uses the "import" statement to reference external modules. Statements in the module or submodule can reference definitions in the external module using the prefix specified in the "import" statement. For backward compatibility with YANG version 1, a submodule MAY use the "include" statement to reference other submodules within its module, but this is not necessary in YANG version 1.1. A submodule can reference any definition in the module it belongs to and in all submodules included by the module. A submodule MUST NOT include different revisions of other submodules than the revisions that its module includes. A module or submodule MUST NOT include submodules from other modules, and a submodule MUST NOT import its own module. The "import" and "include" statements are used to make definitions available from other modules: o For a module or submodule to reference definitions in an external module, the external module MUST be imported. o A module MUST include all its submodules. o A module, or submodule belonging to that module, MAY reference definitions in the module and all submodules included by the module. There MUST NOT be any circular chains of imports. For example, if module "a" imports module "b", "b" cannot import "a". When a definition in an external module is referenced, a locally defined prefix MUST be used, followed by a colon (":") and then the external identifier. (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| `RFC7950-5.3-1` | Namespace URIs MUST be chosen so they cannot collide with standard or other enterprise namespaces -- for example, by using the enterprise or organization name in the namespace. (Section 5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no repository check reads the namespace or the revision history of the embedded modules; plan/spec-config-yang-authoring-checks.md |
| `RFC7950-5.5-1` | Scoped definitions MUST NOT shadow definitions at a higher scope. (§5.5) | MUST | 5.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| `RFC7950-5.6.5-1` | A server MUST NOT implement more than one revision of a module. If a server implements a module A that imports a module B, and A uses any node from B in an "augment" or "path" statement that the server supports, then the server MUST implement a revision of module B that has these nodes defined. (§5.6.5) | MUST | 5.6.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| `RFC7950-6.1.3-1` | The backslash MUST NOT be followed by any other character. (§6.1.3) | MUST | 6.1.3 | **positive:** `unit/verify` [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L114). **negative:** `unit/verify` [`TestRFC7950ModuleLoadRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L65) |
| `RFC7950-6.2-1` | Implementations MUST support identifiers up to 64 characters in length and MAY support longer identifiers. (§6.2) | MUST | 6.2 | **positive:** `unit/verify` [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L116). **negative:** no negative test. **{single-polarity}:** the RFC binds an implementation to accept an identifier of up to 64 characters and MAY accept longer, so no identifier length exists that Ze must refuse |
| `RFC7950-6.2.1-1` | All identifiers defined in a namespace MUST be unique. o All module and submodule names share the same global module identifier namespace. o All extension names defined in a module and its submodules share the same extension identifier namespace. o All feature names defined in a module and its submodules share the same feature identifier namespace. o All identity names defined in a module and its submodules share the same identity identifier namespace. o All derived type names defined within a parent node or at the top level of the module or its submodules share the same type identifier namespace. This namespace is scoped to all descendant nodes of the parent node or module. This means that any descendant node may use that typedef, and it MUST NOT define a typedef with the same name. o All grouping names defined within a parent node or at the top level of the module or its submodules share the same grouping identifier namespace. This namespace is scoped to all descendant nodes of the parent node or module. This means that any descendant node may use that grouping, and it MUST NOT define a grouping with the same name. (§6.2.1) | MUST | 6.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| `RFC7950-6.3.1-1` | When an imported extension is used, the extension's keyword MUST be qualified using the prefix with which the extension's module was imported. If an extension is used in the module where it is defined, the extension's keyword MUST be qualified with the prefix of this module. The processing of extensions depends on whether support for those extensions is claimed for a given YANG parser or the tool set in which it is embedded. An unsupported extension appearing in a YANG module as an unknown-statement (see Section 14) MAY be ignored in its entirety. Any supported extension MUST be processed in accordance with the specification governing that extension. (§6.3.1) | MUST | 6.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| `RFC7950-6.4-1` | An implementation is not required to implement an XPath interpreter but MUST ensure that the requirements encoded in the data model are enforced. (Section 6.4) | MUST | 6.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** walkTree evaluates no when, unique or choice; plan/pre-release/spec-config-yang-when-unique-choice.md |
| `RFC7950-6.4-2` | The XPath expressions MUST be syntactically correct, and all prefixes used MUST be present in the XPath context (see Section 6.4.1). (§6.4) | MUST | 6.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** walkTree evaluates no when, unique or choice; plan/pre-release/spec-config-yang-when-unique-choice.md |
| `RFC7950-6.5-1` | References to identifiers defined in external modules MUST be qualified with appropriate prefixes, and references to identifiers defined in the current module and its submodules MAY use a prefix. (§6.5) | MUST | 6.5 | **positive:** `unit/verify` [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L121). **negative:** `unit/verify` [`TestRFC7950ModuleLoadRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L68) |
| `RFC7950-7.1.4-1` | If there is a conflict, i.e., two different modules that both have defined the same prefix are imported, at least one of them MUST be imported with a different prefix. All prefixes, including the prefix for the module itself, MUST be unique within the module or submodule. (§7.1.4) | MUST | 7.1.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| `RFC7950-7.3-1` | The "typedef" statement's argument is an identifier that is the name of the type to be defined and MUST be followed by a block of substatements that holds detailed typedef information. The name of the type MUST NOT be one of the YANG built-in types. If the typedef is defined at the top level of a YANG module or submodule, the name of the type to be defined MUST be unique within the module. (§7.3) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| `RFC7950-7.3.4-1` | The value of the "default" statement MUST be valid according to the type specified in the "type" statement. If the base type has a default value and the new derived type does not specify a new default value, the base type's default value is also the default value of the new derived type. If the type's default value is not valid according to the new restrictions specified in a derived type or leaf definition, the derived type or leaf definition MUST specify a new default value compatible with the restrictions. (§7.3.4) | MUST | 7.3.4 | **positive:** `unit/verify` [`TestRFC7950DefaultValidForType`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_test.go#L32). **positive:** `unit/verify` [`TestRFC7950DerivedTypeInheritsOrReplacesTheDefault`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_clauses_test.go#L40). **negative:** `unit/verify` [`TestRFC7950DefaultInvalidForType`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_test.go#L47). **negative:** `unit/verify` [`TestRFC7950DerivedTypeMustReplaceAnInvalidatedDefault`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_clauses_test.go#L59) |
| `RFC7950-7.6.1-1` | The usage of the default value depends on the leaf's closest ancestor node in the schema tree that is not a non-presence container (see Section 7.5.1): o If no such ancestor exists in the schema tree, the default value MUST be used. (§7.6.1) | MUST | 7.6.1 | **positive:** `unit/verify` [`TestApplyDefaults_NonPresenceContainer`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_schema_defaults_test.go#L38). **positive:** `unit/verify` [`TestRFC7950LeafDefaultUsedWhenAbsent`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_leaf_default_test.go#L14). **negative:** `unit/verify` [`TestRFC7950LeafDefaultNotUsedWhenSet`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_leaf_default_test.go#L42) |
| `RFC7950-7.6.1-2` | Otherwise, if this ancestor is a case node, the default value MUST be used if any node from the case exists in the data tree or the case node is the choice's default case, and if no nodes from any other case exist in the data tree. (§7.6.1) | MUST | 7.6.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** flattenChoiceCases drops the case node, so ApplyDefaults fills every case's defaults; plan/pre-release/spec-config-yang-when-unique-choice.md |
| `RFC7950-7.6.1-3` | Otherwise, the default value MUST be used if the ancestor node exists in the data tree. (§7.6.1) | MUST | 7.6.1 | **positive:** `unit/verify` [`TestRFC7950LeafDefaultFollowsItsAncestor`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_leaf_default_test.go#L69). **negative:** `unit/verify` [`TestRFC7950LeafDefaultFollowsItsAncestor`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_leaf_default_test.go#L70) |
| `RFC7950-7.6.1-4` | In these cases, the default value is said to be in use. Note that if the leaf or any of its ancestors has a "when" condition or "if-feature" expression that evaluates to "false", then the default value is not in use. When the default value is in use, the server MUST operationally behave as if the leaf was present in the data tree with the default value as its value. (§7.6.1) | MUST | 7.6.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no when is evaluated and Ze has no if-feature, so a default whose "when" is false stays in use; plan/pre-release/spec-config-yang-when-unique-choice.md |
| `RFC7950-7.6.3-1` | The "type" statement, which MUST be present, takes as an argument the name of an existing built-in or derived type. (Section 7.6.3) | MUST | 7.6.3 | **positive:** `unit/verify` [`TestRFC7950LeafTypeMustNameAnExistingType`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L39). **positive:** `unit/verify` [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L128). **negative:** `unit/verify` [`TestRFC7950LeafTypeMustNameAnExistingType`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L38). **negative:** `unit/verify` [`TestRFC7950ModuleLoadRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L74) |
| `RFC7950-7.6.4-2` | The value of the "default" statement MUST be valid according to the type specified in the leaf's "type" statement. The "default" statement MUST NOT be present on nodes where "mandatory" is "true". The definition of the default value MUST NOT be marked with an "if-feature" statement. (§7.6.4) | MUST | 7.6.4 | **positive:** `unit/verify` [`TestRFC7950DefaultNotAnIfFeatureEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_clauses_test.go#L75). **positive:** `unit/verify` [`TestRFC7950DefaultNotAnIfFeatureMemberBitOrTypedef`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_clauses_test.go#L93). **positive:** `unit/verify` [`TestRFC7950DefaultValidForType`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_test.go#L33). **negative:** `unit/verify` [`TestRFC7950DefaultInvalidForType`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_test.go#L48). **negative:** `unit/verify` [`TestRFC7950DefaultNotAnIfFeatureEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_clauses_test.go#L74). **negative:** `unit/verify` [`TestRFC7950DefaultNotAnIfFeatureMemberBitOrTypedef`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_clauses_test.go#L92) |
| `RFC7950-7.7-1` | In configuration data, the values in a leaf-list MUST be unique. The definitions of the default values MUST NOT be marked with an "if-feature" statement. Conceptually, the values in the data tree MUST be in the canonical form (see Section 9.1). (§7.7) | MUST | 7.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** yangToList tolerates a keyless config list and leaf-lists are neither deduplicated nor defaulted; plan/pre-release/spec-config-yang-list-key-leaf-list.md |
| `RFC7950-7.7.2-1` | The usage of the default values depends on the leaf-list's closest ancestor node in the schema tree that is not a non-presence container (see Section 7.5.1): o If no such ancestor exists in the schema tree, the default values MUST be used. o Otherwise, if this ancestor is a case node, the default values MUST be used if any node from the case exists in the data tree or the case node is the choice's default case, and if no nodes from any other case exist in the data tree. o Otherwise, the default values MUST be used if the ancestor node exists in the data tree. In these cases, the default values are said to be in use. Note that if the leaf-list or any of its ancestors has a "when" condition or "if-feature" expression that evaluates to "false", then the default values are not in use. When the default values are in use, the server MUST operationally behave as if the leaf-list was present in the data tree with the default values as its values. (§7.7.2) | MUST | 7.7.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** yangToList tolerates a keyless config list and leaf-lists are neither deduplicated nor defaulted; plan/pre-release/spec-config-yang-list-key-leaf-list.md |
| `RFC7950-7.7.4-1` | The value of the "default" statement MUST be valid according to the type specified in the leaf-list's "type" statement. The "default" statement MUST NOT be present on nodes where "min-elements" has a value greater than or equal to one. (§7.7.4) | MUST | 7.7.4 | **positive:** `unit/verify` [`TestRFC7950DefaultValidForType`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_test.go#L34). **negative:** `unit/verify` [`TestRFC7950DefaultInvalidForType`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_test.go#L49) |
| `RFC7950-7.7.5-1` | A valid leaf-list or list MUST have at least min-elements entries. (Section 7.7.5) | MUST | 7.7.5 | **positive:** `unit/verify` [`TestRFC7950MinElementsMet`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_test.go#L48). **negative:** `unit/verify` [`TestRFC7950MinElementsViolated`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_test.go#L67) |
| `RFC7950-7.8.2-1` | The "key" statement, which MUST be present if the list represents configuration and MAY be present otherwise, takes as an argument a string that specifies a space-separated list of one or more leaf identifiers of this list. A leaf identifier MUST NOT appear more than once in the key. Each such leaf identifier MUST refer to a child leaf of the list. The leafs can be defined directly in substatements to the list or in groupings used in the list. The combined values of all the leafs specified in the key are used to uniquely identify a list entry. All key leafs MUST be given values when a list entry is created. Thus, any default values in the key leafs or their types are ignored. Any "mandatory" statements in the key leafs are ignored. A leaf that is part of the key can be of any built-in or derived type. All key leafs in a list MUST have the same value for their "config" as the list itself. (§7.8.2) | MUST | 7.8.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** yangToList tolerates a keyless config list and leaf-lists are neither deduplicated nor defaulted; plan/pre-release/spec-config-yang-list-key-leaf-list.md |
| `RFC7950-7.8.3-1` | The "unique" statement is used to put constraints on valid list entries. It takes as an argument a string that contains a space- separated list of schema node identifiers, which MUST be given in the descendant form (see the rule "descendant-schema-nodeid" in Section 14). Each such schema node identifier MUST refer to a leaf. If one of the referenced leafs represents configuration data, then all of the referenced leafs MUST represent configuration data. The "unique" constraint specifies that the combined values of all the leaf instances specified in the argument string, including leafs with default values, MUST be unique within all list entry instances in which all referenced leafs exist or have default values. (§7.8.3) | MUST | 7.8.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** walkTree evaluates no when, unique or choice; plan/pre-release/spec-config-yang-when-unique-choice.md |
| `RFC7950-7.9.2-1` | The identifiers of all these child nodes MUST be unique within all cases in a choice. (§7.9.2) | MUST | 7.9.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| `RFC7950-7.9.3-1` | The "default" statement MUST NOT be present on choices where "mandatory" is "true". The default case is only important when considering the "default" statements of nodes under the cases (i.e., default values of leafs and leaf-lists, and default cases of nested choices). The default values and nested default cases under the default case are used if none of the nodes under any of the cases are present. There MUST NOT be any mandatory nodes (Section 3) directly under the default case. (§7.9.3) | MUST | 7.9.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| `RFC7950-7.9.4-1` | If "mandatory" is "true", at least one node from exactly one of the choice's case branches MUST exist. (Section 7.9.4) | MUST | 7.9.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** walkTree evaluates no when, unique or choice; plan/pre-release/spec-config-yang-when-unique-choice.md |
| `RFC7950-7.12-1` | A grouping MUST NOT reference itself, neither directly nor indirectly through a chain of other groupings. If the grouping is defined at the top level of a YANG module or submodule, the grouping's identifier MUST be unique within the module. (§7.12) | MUST | 7.12 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| `RFC7950-7.14.2-1` | If a leaf in the input tree has a "mandatory" statement with the value "true", the leaf MUST be present in an RPC invocation. If a leaf in the input tree has a default value, the server MUST use this value in the same cases as those described in Section 7.6.1. In these cases, the server MUST operationally behave as if the leaf was present in the RPC invocation with the default value as its value. If a leaf-list in the input tree has one or more default values, the server MUST use these values in the same cases as those described in Section 7.7.2. In these cases, the server MUST operationally behave as if the leaf-list was present in the RPC invocation with the default values as its values. Since the input tree is not part of any datastore, all "config" statements for nodes in the input tree are ignored. If any node has a "when" statement that would evaluate to "false", then this node MUST NOT be present in the input tree. (§7.14.2) | MUST | 7.14.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no invocation or reply path reads mandatory or default on rpc input and output; plan/pre-release/spec-config-yang-rpc-mandatory-defaults.md |
| `RFC7950-7.14.3-1` | If a leaf in the output tree has a "mandatory" statement with the value "true", the leaf MUST be present in an RPC reply. If a leaf in the output tree has a default value, the client MUST use this value in the same cases as those described in Section 7.6.1. In these cases, the client MUST operationally behave as if the leaf was present in the RPC reply with the default value as its value. If a leaf-list in the output tree has one or more default values, the client MUST use these values in the same cases as those described in Section 7.7.2. In these cases, the client MUST operationally behave as if the leaf-list was present in the RPC reply with the default values as its values. Since the output tree is not part of any datastore, all "config" statements for nodes in the output tree are ignored. If any node has a "when" statement that would evaluate to "false", then this node MUST NOT be present in the output tree. (§7.14.3) | MUST | 7.14.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no invocation or reply path reads mandatory or default on rpc input and output; plan/pre-release/spec-config-yang-rpc-mandatory-defaults.md |
| `RFC7950-7.15-1` | An action MUST NOT be defined within an rpc, another action, or a notification, i.e., an action node MUST NOT have an rpc, action, or a notification node as one of its ancestors in the schema tree. For example, this means that it is an error if a grouping that contains an action somewhere in its node hierarchy is used in a notification definition. An action MUST NOT have any ancestor node that is a list node without a "key" statement. (§7.15) | MUST | 7.15 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| `RFC7950-7.16-1` | A notification MUST NOT be defined within an rpc, action, or another notification, i.e., a notification node MUST NOT have an rpc, action, or a notification node as one of its ancestors in the schema tree. For example, this means that it is an error if a grouping that contains a notification somewhere in its node hierarchy is used in an rpc definition. A notification MUST NOT have any ancestor node that is a list node without a "key" statement. Since a notification cannot be defined in a "case" statement, it is an error if a grouping that contains a notification at the top of its node hierarchy is used in a case definition. If a leaf in the notification tree has a "mandatory" statement with the value "true", the leaf MUST be present in a notification instance. If a leaf in the notification tree has a default value, the client MUST use this value in the same cases as those described in Section 7.6.1. In these cases, the client MUST operationally behave as if the leaf was present in the notification instance with the default value as its value. If a leaf-list in the notification tree has one or more default values, the client MUST use these values in the same cases as those described in Section 7.7.2. In these cases, the client MUST operationally behave as if the leaf-list was present in the notification instance with the default values as its values. (§7.16) | MUST | 7.16 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| `RFC7950-7.17-1` | The target node MUST be either a container, list, choice, case, input, output, or notification node. It is augmented with the nodes defined in the substatements that follow the "augment" statement. The argument string is a schema node identifier (see Section 6.5). If the "augment" statement is on the top level in a module or submodule, the absolute form (defined by the rule "absolute-schema-nodeid" in Section 14) of a schema node identifier MUST be used. If the "augment" statement is a substatement to the "uses" statement, the descendant form (defined by the rule "descendant-schema-nodeid" in Section 14) MUST be used. If the target node is a container, list, case, input, output, or notification node, the "container", "leaf", "list", "leaf-list", "uses", and "choice" statements can be used within the "augment" statement. If the target node is a container or list node, the "action" and "notification" statements can be used within the "augment" statement. If the target node is a choice node, the "case" statement or a shorthand "case" statement (see Section 7.9.2) can be used within the "augment" statement. The "augment" statement MUST NOT add multiple nodes with the same name from the same module to the target node. If the augmentation adds mandatory nodes (see Section 3) that represent configuration to a target node in another module, the augmentation MUST be made conditional with a "when" statement. (§7.17) | MUST | 7.17 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| `RFC7950-7.18.2-1` | If a prefix is present on the base name, it refers to an identity defined in the module that was imported with that prefix, or the local module if the prefix matches the local module's prefix. Otherwise, an identity with the matching name MUST be defined in the current module or an included submodule. An identity MUST NOT reference itself, neither directly nor indirectly through a chain of other identities. (§7.18.2) | MUST | 7.18.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| `RFC7950-7.19-1` | Syntactically, the substatements MUST be YANG statements, including extensions defined using "extension" statements. YANG statements in extensions MUST follow the syntactical rules in Section 14. (§7.19) | MUST | 7.19 | **positive:** `unit/verify` [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L136). **negative:** `unit/verify` [`TestRFC7950ModuleLoadRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L81) |
| `RFC7950-7.20.1-1` | A feature MUST NOT reference itself, neither directly nor indirectly through a chain of other features. In order for a server to support a feature that is dependent on any other features (i.e., the feature has one or more "if-feature" substatements), the server MUST also support all the dependent features. (§7.20.1) | MUST | 7.20.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze evaluates no if-feature and declares no feature; plan/spec-config-yang-if-feature.md |
| `RFC7950-7.20.2-1` | If a prefix is present on a feature name in the boolean expression, the prefixed name refers to a feature defined in the module that was imported with that prefix, or the local module if the prefix matches the local module's prefix. Otherwise, a feature with the matching name MUST be defined in the current module or an included submodule. A leaf that is a list key MUST NOT have any "if-feature" statements. (§7.20.2) | MUST | 7.20.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze evaluates no if-feature and declares no feature; plan/spec-config-yang-if-feature.md |
| `RFC7950-7.20.3-1` | This means that deviations MUST never be part of a published standard, since they are the mechanism for learning how implementations vary from the standards. Server deviations are strongly discouraged and MUST only be used as a last resort. Telling the application how a server fails to follow a standard is no substitute for implementing the standard correctly. A server that deviates from a module is not fully compliant with the module. However, in some cases, a particular device may not have the hardware or software ability to support parts of a standard module. When this occurs, the server makes a choice to either treat attempts to configure unsupported parts of the module as an error that is reported back to the unsuspecting application or ignore those incoming requests. Neither choice is acceptable. Instead, YANG allows servers to document portions of a base module that are not supported, or that are supported but with different syntax, by using the "deviation" statement. After applying all deviations announced by a server, in any order, the resulting data model MUST still be valid. (§7.20.3) | MUST | 7.20.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze reads no deviation; plan/spec-config-yang-deviation.md |
| `RFC7950-7.20.3.2-1` | The argument "add" adds properties to the target node. The properties to add are identified by substatements to the "deviate" statement. If a property can only appear once, the property MUST NOT exist in the target node. The argument "replace" replaces properties of the target node. The properties to replace are identified by substatements to the "deviate" statement. The properties to replace MUST exist in the target node. The argument "delete" deletes properties from the target node. The properties to delete are identified by substatements to the "delete" statement. The substatement's keyword MUST match a corresponding keyword in the target node, and the argument's string MUST be equal to the corresponding keyword's argument string in the target node. (§7.20.3.2) | MUST | 7.20.3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze reads no deviation; plan/spec-config-yang-deviation.md |
| `RFC7950-7.21.2-1` | If a definition is "current", it MUST NOT reference a "deprecated" or "obsolete" definition within the same module. If a definition is "deprecated", it MUST NOT reference an "obsolete" definition within the same module. (§7.21.2) | MUST | 7.21.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| `RFC7950-7.21.5-1` | A leaf that is a list key MUST NOT have a "when" statement. If a key leaf is defined in a grouping that is used in a list, the "uses" statement MUST NOT have a "when" statement. (§7.21.5) | MUST | 7.21.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** walkTree evaluates no when, unique or choice; plan/pre-release/spec-config-yang-when-unique-choice.md |
| `RFC7950-8.1-1` | These constraints are enforced in different ways, depending on what type of data the statement defines. o If the constraint is defined on configuration data, it MUST be true in a valid configuration data tree. o If the constraint is defined on state data, it MUST be true in a valid state data tree. o If the constraint is defined on notification content, it MUST be true in any notification data tree. o If the constraint is defined on RPC or action input parameters, it MUST be true in an invocation of the RPC or action operation. o If the constraint is defined on RPC or action output parameters, it MUST be true in the RPC or action reply. (§8.1) | MUST | 8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** walkTree evaluates no when, unique or choice; plan/pre-release/spec-config-yang-when-unique-choice.md |
| `RFC7950-8.1-3` | All leaf data values MUST match the type constraints for the leaf, including those defined in the type's "range", "length", and "pattern" properties. (§8.1) | MUST | 8.1 | **positive:** `unit/verify` [`TestValidator_HoldTimeRange`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L899). **positive:** `unit/verify` [`TestValidator_ValidatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L850). **negative:** `unit/verify` [`TestValidateTree_LengthViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L417). **negative:** `unit/verify` [`TestValidateTree_PatternViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L316). **negative:** `unit/verify` [`TestValidateTree_RangeViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L175). **negative:** `unit/verify` [`TestValidator_HoldTimeRange`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L900). **negative:** `unit/verify` [`TestValidator_ValidatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L851) |
| `RFC7950-8.1-4` | All key leafs MUST be present for all list entries. o Nodes MUST be present for at most one case branch in all choices. o There MUST be no nodes tagged with "if-feature" present if the "if-feature" expression evaluates to "false" in the server. o There MUST be no nodes tagged with "when" present if the "when" condition evaluates to "false" in the data tree. The following properties are true in a valid data tree: o All "must" constraints MUST evaluate to "true". o All referential integrity constraints defined via the "path" statement MUST be satisfied. o All "unique" constraints on lists MUST be satisfied. o The "mandatory" constraint is enforced for leafs and choices, unless the node or any of its ancestors has a "when" condition or "if-feature" expression that evaluates to "false". (§8.1) | MUST | 8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** walkTree evaluates no when, unique or choice; plan/pre-release/spec-config-yang-when-unique-choice.md |
| `RFC7950-8.1-2` | The running configuration datastore MUST always be valid. (Section 8.1) | MUST | 8.1 | **positive:** `unit/verify` [`TestRFC7950RunningDatastoreAcceptsAValidConfig`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_datastore_test.go#L33). **negative:** `unit/verify` [`TestRFC7950RunningDatastoreRefusesAViolationAfterParse`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_datastore_test.go#L77) |
| `RFC7950-8.3-1` | For configuration data, there are three windows when constraints MUST be enforced: o during parsing of RPC payloads o during processing of the <edit-config> operation o during validation (§8.3) | MUST | 8.3 | **positive:** `unit/verify` [`TestRFC7950RunningDatastoreAcceptsAValidConfig`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_datastore_test.go#L34). **negative:** `unit/verify` [`TestRFC7950RunningDatastoreRefusesAViolationAfterParse`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_datastore_test.go#L78). **negative:** `unit/verify` [`TestRFC7950RunningDatastoreRefusesAViolationAtParse`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_datastore_test.go#L53) |
| `RFC7950-8.3.1-2` | If data for more than one case branch of a choice is present, the server MUST reply with a "bad-element" <error-tag> in the <rpc-error>. o If data for a node tagged with "if-feature" is present and the "if-feature" expression evaluates to "false" in the server, the server MUST reply with an "unknown-element" <error-tag> in the <rpc-error>. o If data for a node tagged with "when" is present and the "when" condition evaluates to "false", the server MUST reply with an "unknown-element" <error-tag> in the <rpc-error>. (§8.3.1) | MUST | 8.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** walkTree evaluates no when, unique or choice; plan/pre-release/spec-config-yang-when-unique-choice.md |
| `RFC7950-8.3.2-1` | During this processing, the following errors MUST be detected: o Delete requests for non-existent data. o Create requests for existent data. o Insert requests with "before" or "after" parameters that do not exist. o Modification requests for nodes tagged with "when", and the "when" condition evaluates to "false". In this case, the server MUST reply with an "unknown-element" <error-tag> in the <rpc-error>. (§8.3.2) | MUST | 8.3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** walkTree evaluates no when, unique or choice; plan/pre-release/spec-config-yang-when-unique-choice.md |
| `RFC7950-8.3.3-1` | When datastore processing is complete, the final contents MUST obey all validation constraints. (§8.3.3) | MUST | 8.3.3 | **positive:** `unit/verify` [`TestRFC7950RunningDatastoreAcceptsAValidConfig`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_datastore_test.go#L35). **negative:** `unit/verify` [`TestRFC7950RunningDatastoreRefusesAViolationAfterParse`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_datastore_test.go#L79) |
| `RFC7950-8.3.3-2` | If the datastore is "running" or "startup", these constraints MUST be enforced at the end of the <edit-config> or <copy-config> operation. (§8.3.3) | MUST | 8.3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** LoadConfig (the <copy-config> window) and the web editor commit (internal/component/cli/editor_commit.go validateStagedTree, through config/cli.ValidateContent) refuse a type or ze:validate violation, but both grade a missing mandatory leaf a warning (SectionValidationError.Blocking) and evaluate no when, unique or choice, so the end of the operation does not enforce these constraints; plan/pre-release/spec-config-yang-when-unique-choice.md |
| `RFC7950-9.1-1` | Implementations MUST support all lexical representations specified in this document. (§9.1) | MUST | 9.1 | **positive:** `unit/verify` [`TestRFC7950Decimal64Accepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_decimal64_test.go#L25). **positive:** `unit/verify` [`TestRFC7950IntegerSignedLexicalForms`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_integer_test.go#L14). **negative:** `unit/verify` [`TestRFC7950Decimal64Refused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_decimal64_test.go#L40) |
| `RFC7950-9.2.4-2` | If multiple values or ranges are given, they all MUST be disjoint and MUST be in ascending order. If a range restriction is applied to a type that is already range-restricted, the new restriction MUST be equally limiting or more limiting, i.e., raising the lower bounds, reducing the upper bounds, removing explicit values or ranges, or splitting ranges into multiple ranges with intermediate gaps. Each explicit value and range boundary value given in the range expression MUST match the type being restricted or be one of the special values "min" or "max". (§9.2.4) | MUST | 9.2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| `RFC7950-9.3.1-1` | A decimal64 value is lexically represented as an optional sign ("+" or "-"), followed by a sequence of decimal digits, optionally followed by a period ('.') as a decimal indicator and a sequence of decimal digits. (§9.3.1) | MUST | 9.3.1 | **positive:** `unit/verify` [`TestRFC7950Decimal64Accepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_decimal64_test.go#L26). **negative:** `unit/verify` [`TestRFC7950Decimal64Refused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_decimal64_test.go#L41) |
| `RFC7950-9.3.4-1` | The "fraction-digits" statement, which is a substatement to the "type" statement, MUST be present if the type is "decimal64". (Section 9.3.4) | MUST | 9.3.4 | **positive:** `unit/verify` [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L139). **negative:** `unit/verify` [`TestRFC7950ModuleLoadRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L85) |
| `RFC7950-9.4.4-1` | Length-restricting values MUST NOT be negative. If multiple values or ranges are given, they all MUST be disjoint and MUST be in ascending order. (§9.4.4) | MUST | 9.4.4 | **positive:** `unit/verify` [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L141). **negative:** `unit/verify` [`TestRFC7950ModuleLoadRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L88) |
| `RFC7950-9.5.1-1` | The lexical representation of a boolean value is a string with a value of "true" or "false". These values MUST be in lowercase. (§9.5.1) | MUST | 9.5.1 | **positive:** `unit/verify` [`TestRFC7950BooleanLowercaseAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_test.go#L93). **negative:** `unit/verify` [`TestRFC7950BooleanNotLowercaseRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_test.go#L104) |
| `RFC7950-9.6.4-1` | The "enum" statement, which is a substatement to the "type" statement, MUST be present if the type is "enumeration". It is repeatedly used to specify each assigned name of an enumeration type. It takes as an argument a string that is the assigned name. The string MUST NOT be zero-length and MUST NOT have any leading or trailing whitespace characters (any Unicode character with the "White_Space" property). The use of Unicode control codes SHOULD be avoided. The statement is optionally followed by a block of substatements that holds detailed enum information. All assigned names in an enumeration MUST be unique. (§9.6.4) | MUST | 9.6.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| `RFC7950-9.6.4-2` | When an existing enumeration type is restricted, the set of assigned names in the new type MUST be a subset of the base type's set of assigned names. The value of such an assigned name MUST NOT be changed. (§9.6.4) | MUST | 9.6.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| `RFC7950-9.6.4.2-1` | This integer value MUST be in the range -2147483648 to 2147483647, and it MUST be unique within the enumeration type. If a value is not specified, then one will be automatically assigned. If the "enum" substatement is the first one defined, the assigned value is zero (0); otherwise, the assigned value is one greater than the current highest enum value (i.e., the highest enum value, implicit or explicit, prior to the current "enum" substatement in the parent "type" statement). Note that the presence of an "if-feature" statement in an "enum" statement does not affect the automatically assigned value. If the current highest value is equal to 2147483647, then an enum value MUST be specified for "enum" substatements following the one with the current highest value. When an existing enumeration type is restricted, the "value" statement MUST either have the same value as in the base type or not be present, in which case the value is the same as in the base type. (§9.6.4.2) | MUST | 9.6.4.2 | **positive:** `unit/verify` [`TestRFC7950EnumValueWithinInt32`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L106). **positive:** `unit/verify` [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L145). **negative:** `unit/verify` [`TestRFC7950EnumValueWithinInt32`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L105). **negative:** `unit/verify` [`TestRFC7950ModuleLoadRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L92) |
| `RFC7950-9.7-1` | When an existing bits type is restricted, the set of assigned names in the new type MUST be a subset of the base type's set of assigned names. The bit position of such an assigned name MUST NOT be changed. (§9.7) | MUST | 9.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze offers no bits type; plan/spec-config-yang-type-bits.md |
| `RFC7950-9.7.4-1` | The "bit" statement, which is a substatement to the "type" statement, MUST be present if the type is "bits". It is repeatedly used to specify each assigned named bit of a bits type. It takes as an argument a string that is the assigned name of the bit. It is followed by a block of substatements that holds detailed bit information. The assigned name follows the same syntax rules as an identifier (see Section 6.2). All assigned names in a bits type MUST be unique. (§9.7.4) | MUST | 9.7.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze offers no bits type; plan/spec-config-yang-type-bits.md |
| `RFC7950-9.7.4.2-1` | The position value MUST be in the range 0 to 4294967295, and it MUST be unique within the bits type. If a bit position is not specified, then one will be automatically assigned. If the "bit" substatement is the first one defined, the assigned value is zero (0); otherwise, the assigned value is one greater than the current highest bit position (i.e., the highest bit position, implicit or explicit, prior to the current "bit" substatement in the parent "type" statement). Note that the presence of an "if-feature" statement in a "bit" statement does not affect the automatically assigned position. If the current highest bit position value is equal to 4294967295, then a position value MUST be specified for "bit" substatements following the one with the current highest position value. When an existing bits type is restricted, the "position" statement MUST either have the same value as in the base type or not be present, in which case the value is the same as in the base type. (§9.7.4.2) | MUST | 9.7.4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze offers no bits type; plan/spec-config-yang-type-bits.md |
| `RFC7950-9.9-1` | If the "require-instance" property (Section 9.9.3) is "true", there MUST exist a node in the data tree, or a node with a default value in use (see Sections 7.6.1 and 7.7.2), of the referred schema tree leaf or leaf-list node with the same value as the leafref value in a valid data tree. If the referring node represents configuration data and the "require-instance" property (Section 9.9.3) is "true", the referred node MUST also represent configuration. There MUST NOT be any circular chains of leafrefs. If the leaf that the leafref refers to is conditional based on one or more features (see Section 7.20.2), then the leaf with the leafref type MUST also be conditional based on at least the same set of features. (§9.9) | MUST | 9.9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze offers no leafref type; plan/spec-config-yang-type-leafref.md |
| `RFC7950-9.9.2-1` | The "path" statement, which is a substatement to the "type" statement, MUST be present if the type is "leafref". It takes as an argument a string that MUST refer to a leaf or leaf-list node. The syntax for a path argument is a subset of the XPath abbreviated syntax. Predicates are used only for constraining the values for the key nodes for list entries. Each predicate consists of exactly one equality test per key, and multiple adjacent predicates MAY be present if a list has multiple keys. The syntax is formally defined by the rule "path-arg" in Section 14. The predicates are only used when more than one key reference is needed to uniquely identify a leaf instance. This occurs if a list has multiple keys or a reference to a leaf other than the key in a list is needed. In these cases, multiple leafrefs are typically specified, and predicates are used to tie them together. The "path" expression evaluates to a node set consisting of zero, one, or more nodes. If the "require-instance" property is "true", this node set MUST be non-empty. (§9.9.2) | MUST | 9.9.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze offers no leafref type; plan/spec-config-yang-type-leafref.md |
| `RFC7950-9.9.3-1` | If "require-instance" is "true", it means that the instance being referred to MUST exist for the data to be valid. (Section 9.9.3) | MUST | 9.9.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze offers no leafref type; plan/spec-config-yang-type-leafref.md |
| `RFC7950-9.10.2-1` | The "base" statement, which is a substatement to the "type" statement, MUST be present at least once if the type is "identityref". The argument is the name of an identity, as defined by an "identity" statement. If a prefix is present on the identity name, it refers to an identity defined in the module that was imported with that prefix. Otherwise, an identity with the matching name MUST be defined in the current module or an included submodule. (§9.10.2) | MUST | 9.10.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze offers no identityref type; plan/spec-config-yang-type-identityref.md |
| `RFC7950-9.10.3-1` | Otherwise, an identity with the matching name MUST be defined in the current module or one of its submodules. (Section 9.10.3) | MUST | 9.10.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze offers no identityref type; plan/spec-config-yang-type-identityref.md |
| `RFC7950-9.12-2` | When the type is "union", the "type" statement (Section 7.4) MUST be present. (Section 9.12) | MUST | 9.12 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| `RFC7950-9.13-1` | For identifying list entries with keys, each predicate consists of one equality test per key, and each key MUST have a corresponding predicate. If a key is of type "empty", it is represented as a zero-length string (""). If the leaf with the instance-identifier type represents configuration data and the "require-instance" property (Section 9.9.3) is "true", the node it refers to MUST also represent configuration. Such a leaf puts a constraint on valid data. All such leaf nodes MUST reference existing nodes or leaf or leaf-list nodes with their default value in use (see Sections 7.6.1 and 7.7.2) for the data to be valid. (§9.13) | MUST | 9.13 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze offers no instance-identifier type; plan/spec-config-yang-type-instance-identifier.md |
| `RFC7950-11-1` | For any published change, a new "revision" statement (Section 7.1.9) MUST be included in front of the existing "revision" statements. If there are no existing "revision" statements, then one MUST be added to identify the new revision. Furthermore, any necessary changes MUST be applied to any metadata statements, including the "organization" and "contact" statements (Sections 7.1.7 and 7.1.8). Note that definitions contained in a module are available to be imported by any other module and are referenced in "import" statements via the module name. Thus, a module name MUST NOT be changed. Furthermore, the "namespace" statement MUST NOT be changed, since all XML elements are qualified by the namespace. Obsolete definitions MUST NOT be removed from published modules, since their identifiers may still be referenced by other modules. (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no repository check reads the namespace or the revision history of the embedded modules; plan/spec-config-yang-authoring-checks.md |
| `RFC7950-11-2` | Otherwise, if the semantics of any previous definition are changed (i.e., if a non-editorial change is made to any definition other than those specifically allowed above), then this MUST be achieved by a new definition with a new identifier. (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no repository check reads the namespace or the revision history of the embedded modules; plan/spec-config-yang-authoring-checks.md |
| `RFC7950-11-3` | In statements that have any data definition statements as substatements, those data definition substatements MUST NOT be reordered. (§11) | MUST NOT | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no repository check reads the namespace or the revision history of the embedded modules; plan/spec-config-yang-authoring-checks.md |
| `RFC7950-7.2.2-1` | A submodule MUST only be included by either the module to which it belongs or another submodule that belongs to that module. (§7.2.2) | MUST | 7.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| `RFC7950-7.3.2-1` | The "type" statement, which MUST be present, defines the base type from which this type is derived. (§7.3.2) | MUST | 7.3.2 | **positive:** `unit/verify` [`TestRFC7950TypedefTypeMustBePresent`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L54). **negative:** `unit/verify` [`TestRFC7950TypedefTypeMustBePresent`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L53) |
| `RFC7950-7.9.2-2` | The case identifier MUST be unique within a choice. (§7.9.2) | MUST | 7.9.2 | **positive:** `unit/verify` [`TestRFC7950CaseIdentifierUniqueWithinAChoice`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L77). **negative:** `unit/verify` [`TestRFC7950CaseIdentifierUniqueWithinAChoice`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L76) |
| `RFC7950-7.21.5-2` | If the XPath expression references any node that also has associated "when" statements, those "when" expressions MUST be evaluated first. There MUST NOT be any circular dependencies among "when" expressions. (§7.21.5) | MUST | 7.21.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** walkTree evaluates no when, unique or choice; plan/pre-release/spec-config-yang-when-unique-choice.md |
| `RFC7950-9.1-2` | If the data type does not have a canonical form, the format of the value MUST match the data type's lexical representation, but the exact format is implementation dependent. (§9.1) | MUST | 9.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the types with no canonical form, identityref and instance-identifier, are absent from Ze; plan/spec-config-yang-type-identityref.md |
| `RFC7950-9.4.4-2` | If a length restriction is applied to a type that is already length-restricted, the new restriction MUST be equally limiting or more limiting, i.e., raising the lower bounds, reducing the upper bounds, removing explicit length values or ranges, or splitting ranges into multiple ranges with intermediate gaps. (§9.4.4) | MUST | 9.4.4 | **positive:** `unit/verify` [`TestRFC7950LengthRestrictionMustNotWiden`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L93). **negative:** `unit/verify` [`TestRFC7950LengthRestrictionMustNotWiden`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L92) |
| `RFC7950-12-1` | A YANG version 1.1 module MUST NOT include a YANG version 1 submodule, and a YANG version 1 module MUST NOT include a YANG version 1.1 submodule. A YANG version 1 module or submodule MUST NOT import a YANG version 1.1 module by revision. (§12) | MUST NOT | 12 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no Ze module declares yang-version and none is a submodule; plan/spec-config-yang-version-submodule.md |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7950-7.6.5-2`](#rfc7950-7.6.5-2) Otherwise, if this ancestor is a case node, the leaf MUST exist if any node from the case exists in the data tree. (§7.6.5) | {gap}, no test | goyang keeps a case's leaves under the choice and case entries, and walkTree looks each child up only in the container's own Dir, so a mandatory leaf inside a case is never required whatever nodes of that case exist; plan/pre-release/spec-config-yang-when-unique-choice.md |
| [`RFC7950-7.6.5-3`](#rfc7950-7.6.5-3) Otherwise, the leaf MUST exist if the ancestor node exists in the data tree. (§7.6.5) | {gap}, no test | walkTree checks a mandatory leaf only inside containers present in the data, so a mandatory leaf below an absent non-presence container is not required even when its ancestor list entry or presence container exists; plan/pre-release/spec-config-yang-when-unique-choice.md |
| [`RFC7950-7.5.3-1`](#rfc7950-7.5.3-1) All such constraints MUST evaluate to "true" for the data to be valid. (§7.5.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze uses zero YANG must statements (a grep of the .yang models finds must only inside a description); it parses via goyang which evaluates no XPath, and enforces cross-field constraints with Go ze:validate functions (internal/component/config/yang/validator.go:756 applyCustomValidators) instead, so there is no must-XPath code path |
| [`RFC7950-9.2.4-1`](#rfc7950-9.2.4-1) If a range restriction is applied to a type that is already range-restricted, the new restriction MUST be equally limiting or more limiting, i.e., raising the lower bounds, reducing the upper bounds, removing explicit values or ranges, or splitting ranges into multiple ranges with intermediate gaps. (§9.2.4) | no test | no test carries this requirement id; annotated {not-applicable}: this is a schema-authoring-time narrowing constraint resolved by goyang during module processing (internal/component/config/yang/loader.go); ze authors valid narrowings and has no runtime enforcement surface for it |
| [`RFC7950-9.4.5-1`](#rfc7950-9.4.5-1) If the type has multiple "pattern" statements, the expressions are ANDed together, i.e., all such expressions have to match. (§9.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: no ze type carries two or more pattern statements (a scan of the .yang models finds none); the validator loop (internal/component/config/yang/validator.go:268) AND-combines patterns if present, but the multi-pattern construct is unused |
| [`RFC7950-x-1`](#rfc7950-x-1) The "yang-version" statement specifies which version of the YANG language was used in developing the module. The statement's argument is a string. It MUST contain the value "1.1" for YANG modules defined based on this specification. (§7.1.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's YANG models declare no yang-version statement and default to YANG 1.0 in goyang; they use no YANG-1.1-only construct that would require the 1.1 declaration, so the version-declaration obligation has no applicable module |
| [`RFC7950-x-2`](#rfc7950-x-2) An "enumeration" type may have new enums added, provided the old enums's values do not change. (§11) | no test | no test carries this requirement id; annotated {not-applicable}: ze's modules carry no consumed revision statements and there is no cross-revision position-stability tooling; ze pins the values that matter with explicit value statements (e.g. role.yang, ze-types.yang afi/safi) but enforces no automated cross-revision check |
| [`RFC7950-5.1-1`](#rfc7950-5.1-1) Within a server, all module names MUST be unique. A module uses the "include" statement to list all its submodules. A module, or submodule belonging to that module, can reference definitions in the module and all submodules included by the module. A module or submodule uses the "import" statement to reference external modules. Statements in the module or submodule can reference definitions in the external module using the prefix specified in the "import" statement. For backward compatibility with YANG version 1, a submodule MAY use the "include" statement to reference other submodules within its module, but this is not necessary in YANG version 1.1. A submodule can reference any definition in the module it belongs to and in all submodules included by the module. A submodule MUST NOT include different revisions of other submodules than the revisions that its module includes. A module or submodule MUST NOT include submodules from other modules, and a submodule MUST NOT import its own module. The "import" and "include" statements are used to make definitions available from other modules: o For a module or submodule to reference definitions in an external module, the external module MUST be imported. o A module MUST include all its submodules. o A module, or submodule belonging to that module, MAY reference definitions in the module and all submodules included by the module. There MUST NOT be any circular chains of imports. For example, if module "a" imports module "b", "b" cannot import "a". When a definition in an external module is referenced, a locally defined prefix MUST be used, followed by a colon (":") and then the external identifier. (§5.1) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-5.3-1`](#rfc7950-5.3-1) Namespace URIs MUST be chosen so they cannot collide with standard or other enterprise namespaces -- for example, by using the enterprise or organization name in the namespace. (Section 5.3) | {gap}, no test | no repository check reads the namespace or the revision history of the embedded modules; plan/spec-config-yang-authoring-checks.md |
| [`RFC7950-5.5-1`](#rfc7950-5.5-1) Scoped definitions MUST NOT shadow definitions at a higher scope. (§5.5) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-5.6.5-1`](#rfc7950-5.6.5-1) A server MUST NOT implement more than one revision of a module. If a server implements a module A that imports a module B, and A uses any node from B in an "augment" or "path" statement that the server supports, then the server MUST implement a revision of module B that has these nodes defined. (§5.6.5) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-6.2.1-1`](#rfc7950-6.2.1-1) All identifiers defined in a namespace MUST be unique. o All module and submodule names share the same global module identifier namespace. o All extension names defined in a module and its submodules share the same extension identifier namespace. o All feature names defined in a module and its submodules share the same feature identifier namespace. o All identity names defined in a module and its submodules share the same identity identifier namespace. o All derived type names defined within a parent node or at the top level of the module or its submodules share the same type identifier namespace. This namespace is scoped to all descendant nodes of the parent node or module. This means that any descendant node may use that typedef, and it MUST NOT define a typedef with the same name. o All grouping names defined within a parent node or at the top level of the module or its submodules share the same grouping identifier namespace. This namespace is scoped to all descendant nodes of the parent node or module. This means that any descendant node may use that grouping, and it MUST NOT define a grouping with the same name. (§6.2.1) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-6.3.1-1`](#rfc7950-6.3.1-1) When an imported extension is used, the extension's keyword MUST be qualified using the prefix with which the extension's module was imported. If an extension is used in the module where it is defined, the extension's keyword MUST be qualified with the prefix of this module. The processing of extensions depends on whether support for those extensions is claimed for a given YANG parser or the tool set in which it is embedded. An unsupported extension appearing in a YANG module as an unknown-statement (see Section 14) MAY be ignored in its entirety. Any supported extension MUST be processed in accordance with the specification governing that extension. (§6.3.1) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-6.4-1`](#rfc7950-6.4-1) An implementation is not required to implement an XPath interpreter but MUST ensure that the requirements encoded in the data model are enforced. (Section 6.4) | {gap}, no test | walkTree evaluates no when, unique or choice; plan/pre-release/spec-config-yang-when-unique-choice.md |
| [`RFC7950-6.4-2`](#rfc7950-6.4-2) The XPath expressions MUST be syntactically correct, and all prefixes used MUST be present in the XPath context (see Section 6.4.1). (§6.4) | {gap}, no test | walkTree evaluates no when, unique or choice; plan/pre-release/spec-config-yang-when-unique-choice.md |
| [`RFC7950-7.1.4-1`](#rfc7950-7.1.4-1) If there is a conflict, i.e., two different modules that both have defined the same prefix are imported, at least one of them MUST be imported with a different prefix. All prefixes, including the prefix for the module itself, MUST be unique within the module or submodule. (§7.1.4) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-7.3-1`](#rfc7950-7.3-1) The "typedef" statement's argument is an identifier that is the name of the type to be defined and MUST be followed by a block of substatements that holds detailed typedef information. The name of the type MUST NOT be one of the YANG built-in types. If the typedef is defined at the top level of a YANG module or submodule, the name of the type to be defined MUST be unique within the module. (§7.3) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-7.6.1-2`](#rfc7950-7.6.1-2) Otherwise, if this ancestor is a case node, the default value MUST be used if any node from the case exists in the data tree or the case node is the choice's default case, and if no nodes from any other case exist in the data tree. (§7.6.1) | {gap}, no test | flattenChoiceCases drops the case node, so ApplyDefaults fills every case's defaults; plan/pre-release/spec-config-yang-when-unique-choice.md |
| [`RFC7950-7.6.1-4`](#rfc7950-7.6.1-4) In these cases, the default value is said to be in use. Note that if the leaf or any of its ancestors has a "when" condition or "if-feature" expression that evaluates to "false", then the default value is not in use. When the default value is in use, the server MUST operationally behave as if the leaf was present in the data tree with the default value as its value. (§7.6.1) | {gap}, no test | no when is evaluated and Ze has no if-feature, so a default whose "when" is false stays in use; plan/pre-release/spec-config-yang-when-unique-choice.md |
| [`RFC7950-7.7-1`](#rfc7950-7.7-1) In configuration data, the values in a leaf-list MUST be unique. The definitions of the default values MUST NOT be marked with an "if-feature" statement. Conceptually, the values in the data tree MUST be in the canonical form (see Section 9.1). (§7.7) | {gap}, no test | yangToList tolerates a keyless config list and leaf-lists are neither deduplicated nor defaulted; plan/pre-release/spec-config-yang-list-key-leaf-list.md |
| [`RFC7950-7.7.2-1`](#rfc7950-7.7.2-1) The usage of the default values depends on the leaf-list's closest ancestor node in the schema tree that is not a non-presence container (see Section 7.5.1): o If no such ancestor exists in the schema tree, the default values MUST be used. o Otherwise, if this ancestor is a case node, the default values MUST be used if any node from the case exists in the data tree or the case node is the choice's default case, and if no nodes from any other case exist in the data tree. o Otherwise, the default values MUST be used if the ancestor node exists in the data tree. In these cases, the default values are said to be in use. Note that if the leaf-list or any of its ancestors has a "when" condition or "if-feature" expression that evaluates to "false", then the default values are not in use. When the default values are in use, the server MUST operationally behave as if the leaf-list was present in the data tree with the default values as its values. (§7.7.2) | {gap}, no test | yangToList tolerates a keyless config list and leaf-lists are neither deduplicated nor defaulted; plan/pre-release/spec-config-yang-list-key-leaf-list.md |
| [`RFC7950-7.8.2-1`](#rfc7950-7.8.2-1) The "key" statement, which MUST be present if the list represents configuration and MAY be present otherwise, takes as an argument a string that specifies a space-separated list of one or more leaf identifiers of this list. A leaf identifier MUST NOT appear more than once in the key. Each such leaf identifier MUST refer to a child leaf of the list. The leafs can be defined directly in substatements to the list or in groupings used in the list. The combined values of all the leafs specified in the key are used to uniquely identify a list entry. All key leafs MUST be given values when a list entry is created. Thus, any default values in the key leafs or their types are ignored. Any "mandatory" statements in the key leafs are ignored. A leaf that is part of the key can be of any built-in or derived type. All key leafs in a list MUST have the same value for their "config" as the list itself. (§7.8.2) | {gap}, no test | yangToList tolerates a keyless config list and leaf-lists are neither deduplicated nor defaulted; plan/pre-release/spec-config-yang-list-key-leaf-list.md |
| [`RFC7950-7.8.3-1`](#rfc7950-7.8.3-1) The "unique" statement is used to put constraints on valid list entries. It takes as an argument a string that contains a space- separated list of schema node identifiers, which MUST be given in the descendant form (see the rule "descendant-schema-nodeid" in Section 14). Each such schema node identifier MUST refer to a leaf. If one of the referenced leafs represents configuration data, then all of the referenced leafs MUST represent configuration data. The "unique" constraint specifies that the combined values of all the leaf instances specified in the argument string, including leafs with default values, MUST be unique within all list entry instances in which all referenced leafs exist or have default values. (§7.8.3) | {gap}, no test | walkTree evaluates no when, unique or choice; plan/pre-release/spec-config-yang-when-unique-choice.md |
| [`RFC7950-7.9.2-1`](#rfc7950-7.9.2-1) The identifiers of all these child nodes MUST be unique within all cases in a choice. (§7.9.2) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-7.9.3-1`](#rfc7950-7.9.3-1) The "default" statement MUST NOT be present on choices where "mandatory" is "true". The default case is only important when considering the "default" statements of nodes under the cases (i.e., default values of leafs and leaf-lists, and default cases of nested choices). The default values and nested default cases under the default case are used if none of the nodes under any of the cases are present. There MUST NOT be any mandatory nodes (Section 3) directly under the default case. (§7.9.3) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-7.9.4-1`](#rfc7950-7.9.4-1) If "mandatory" is "true", at least one node from exactly one of the choice's case branches MUST exist. (Section 7.9.4) | {gap}, no test | walkTree evaluates no when, unique or choice; plan/pre-release/spec-config-yang-when-unique-choice.md |
| [`RFC7950-7.12-1`](#rfc7950-7.12-1) A grouping MUST NOT reference itself, neither directly nor indirectly through a chain of other groupings. If the grouping is defined at the top level of a YANG module or submodule, the grouping's identifier MUST be unique within the module. (§7.12) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-7.14.2-1`](#rfc7950-7.14.2-1) If a leaf in the input tree has a "mandatory" statement with the value "true", the leaf MUST be present in an RPC invocation. If a leaf in the input tree has a default value, the server MUST use this value in the same cases as those described in Section 7.6.1. In these cases, the server MUST operationally behave as if the leaf was present in the RPC invocation with the default value as its value. If a leaf-list in the input tree has one or more default values, the server MUST use these values in the same cases as those described in Section 7.7.2. In these cases, the server MUST operationally behave as if the leaf-list was present in the RPC invocation with the default values as its values. Since the input tree is not part of any datastore, all "config" statements for nodes in the input tree are ignored. If any node has a "when" statement that would evaluate to "false", then this node MUST NOT be present in the input tree. (§7.14.2) | {gap}, no test | no invocation or reply path reads mandatory or default on rpc input and output; plan/pre-release/spec-config-yang-rpc-mandatory-defaults.md |
| [`RFC7950-7.14.3-1`](#rfc7950-7.14.3-1) If a leaf in the output tree has a "mandatory" statement with the value "true", the leaf MUST be present in an RPC reply. If a leaf in the output tree has a default value, the client MUST use this value in the same cases as those described in Section 7.6.1. In these cases, the client MUST operationally behave as if the leaf was present in the RPC reply with the default value as its value. If a leaf-list in the output tree has one or more default values, the client MUST use these values in the same cases as those described in Section 7.7.2. In these cases, the client MUST operationally behave as if the leaf-list was present in the RPC reply with the default values as its values. Since the output tree is not part of any datastore, all "config" statements for nodes in the output tree are ignored. If any node has a "when" statement that would evaluate to "false", then this node MUST NOT be present in the output tree. (§7.14.3) | {gap}, no test | no invocation or reply path reads mandatory or default on rpc input and output; plan/pre-release/spec-config-yang-rpc-mandatory-defaults.md |
| [`RFC7950-7.15-1`](#rfc7950-7.15-1) An action MUST NOT be defined within an rpc, another action, or a notification, i.e., an action node MUST NOT have an rpc, action, or a notification node as one of its ancestors in the schema tree. For example, this means that it is an error if a grouping that contains an action somewhere in its node hierarchy is used in a notification definition. An action MUST NOT have any ancestor node that is a list node without a "key" statement. (§7.15) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-7.16-1`](#rfc7950-7.16-1) A notification MUST NOT be defined within an rpc, action, or another notification, i.e., a notification node MUST NOT have an rpc, action, or a notification node as one of its ancestors in the schema tree. For example, this means that it is an error if a grouping that contains a notification somewhere in its node hierarchy is used in an rpc definition. A notification MUST NOT have any ancestor node that is a list node without a "key" statement. Since a notification cannot be defined in a "case" statement, it is an error if a grouping that contains a notification at the top of its node hierarchy is used in a case definition. If a leaf in the notification tree has a "mandatory" statement with the value "true", the leaf MUST be present in a notification instance. If a leaf in the notification tree has a default value, the client MUST use this value in the same cases as those described in Section 7.6.1. In these cases, the client MUST operationally behave as if the leaf was present in the notification instance with the default value as its value. If a leaf-list in the notification tree has one or more default values, the client MUST use these values in the same cases as those described in Section 7.7.2. In these cases, the client MUST operationally behave as if the leaf-list was present in the notification instance with the default values as its values. (§7.16) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-7.17-1`](#rfc7950-7.17-1) The target node MUST be either a container, list, choice, case, input, output, or notification node. It is augmented with the nodes defined in the substatements that follow the "augment" statement. The argument string is a schema node identifier (see Section 6.5). If the "augment" statement is on the top level in a module or submodule, the absolute form (defined by the rule "absolute-schema-nodeid" in Section 14) of a schema node identifier MUST be used. If the "augment" statement is a substatement to the "uses" statement, the descendant form (defined by the rule "descendant-schema-nodeid" in Section 14) MUST be used. If the target node is a container, list, case, input, output, or notification node, the "container", "leaf", "list", "leaf-list", "uses", and "choice" statements can be used within the "augment" statement. If the target node is a container or list node, the "action" and "notification" statements can be used within the "augment" statement. If the target node is a choice node, the "case" statement or a shorthand "case" statement (see Section 7.9.2) can be used within the "augment" statement. The "augment" statement MUST NOT add multiple nodes with the same name from the same module to the target node. If the augmentation adds mandatory nodes (see Section 3) that represent configuration to a target node in another module, the augmentation MUST be made conditional with a "when" statement. (§7.17) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-7.18.2-1`](#rfc7950-7.18.2-1) If a prefix is present on the base name, it refers to an identity defined in the module that was imported with that prefix, or the local module if the prefix matches the local module's prefix. Otherwise, an identity with the matching name MUST be defined in the current module or an included submodule. An identity MUST NOT reference itself, neither directly nor indirectly through a chain of other identities. (§7.18.2) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-7.20.1-1`](#rfc7950-7.20.1-1) A feature MUST NOT reference itself, neither directly nor indirectly through a chain of other features. In order for a server to support a feature that is dependent on any other features (i.e., the feature has one or more "if-feature" substatements), the server MUST also support all the dependent features. (§7.20.1) | {gap}, no test | Ze evaluates no if-feature and declares no feature; plan/spec-config-yang-if-feature.md |
| [`RFC7950-7.20.2-1`](#rfc7950-7.20.2-1) If a prefix is present on a feature name in the boolean expression, the prefixed name refers to a feature defined in the module that was imported with that prefix, or the local module if the prefix matches the local module's prefix. Otherwise, a feature with the matching name MUST be defined in the current module or an included submodule. A leaf that is a list key MUST NOT have any "if-feature" statements. (§7.20.2) | {gap}, no test | Ze evaluates no if-feature and declares no feature; plan/spec-config-yang-if-feature.md |
| [`RFC7950-7.20.3-1`](#rfc7950-7.20.3-1) This means that deviations MUST never be part of a published standard, since they are the mechanism for learning how implementations vary from the standards. Server deviations are strongly discouraged and MUST only be used as a last resort. Telling the application how a server fails to follow a standard is no substitute for implementing the standard correctly. A server that deviates from a module is not fully compliant with the module. However, in some cases, a particular device may not have the hardware or software ability to support parts of a standard module. When this occurs, the server makes a choice to either treat attempts to configure unsupported parts of the module as an error that is reported back to the unsuspecting application or ignore those incoming requests. Neither choice is acceptable. Instead, YANG allows servers to document portions of a base module that are not supported, or that are supported but with different syntax, by using the "deviation" statement. After applying all deviations announced by a server, in any order, the resulting data model MUST still be valid. (§7.20.3) | {gap}, no test | Ze reads no deviation; plan/spec-config-yang-deviation.md |
| [`RFC7950-7.20.3.2-1`](#rfc7950-7.20.3.2-1) The argument "add" adds properties to the target node. The properties to add are identified by substatements to the "deviate" statement. If a property can only appear once, the property MUST NOT exist in the target node. The argument "replace" replaces properties of the target node. The properties to replace are identified by substatements to the "deviate" statement. The properties to replace MUST exist in the target node. The argument "delete" deletes properties from the target node. The properties to delete are identified by substatements to the "delete" statement. The substatement's keyword MUST match a corresponding keyword in the target node, and the argument's string MUST be equal to the corresponding keyword's argument string in the target node. (§7.20.3.2) | {gap}, no test | Ze reads no deviation; plan/spec-config-yang-deviation.md |
| [`RFC7950-7.21.2-1`](#rfc7950-7.21.2-1) If a definition is "current", it MUST NOT reference a "deprecated" or "obsolete" definition within the same module. If a definition is "deprecated", it MUST NOT reference an "obsolete" definition within the same module. (§7.21.2) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-7.21.5-1`](#rfc7950-7.21.5-1) A leaf that is a list key MUST NOT have a "when" statement. If a key leaf is defined in a grouping that is used in a list, the "uses" statement MUST NOT have a "when" statement. (§7.21.5) | {gap}, no test | walkTree evaluates no when, unique or choice; plan/pre-release/spec-config-yang-when-unique-choice.md |
| [`RFC7950-8.1-1`](#rfc7950-8.1-1) These constraints are enforced in different ways, depending on what type of data the statement defines. o If the constraint is defined on configuration data, it MUST be true in a valid configuration data tree. o If the constraint is defined on state data, it MUST be true in a valid state data tree. o If the constraint is defined on notification content, it MUST be true in any notification data tree. o If the constraint is defined on RPC or action input parameters, it MUST be true in an invocation of the RPC or action operation. o If the constraint is defined on RPC or action output parameters, it MUST be true in the RPC or action reply. (§8.1) | {gap}, no test | walkTree evaluates no when, unique or choice; plan/pre-release/spec-config-yang-when-unique-choice.md |
| [`RFC7950-8.1-4`](#rfc7950-8.1-4) All key leafs MUST be present for all list entries. o Nodes MUST be present for at most one case branch in all choices. o There MUST be no nodes tagged with "if-feature" present if the "if-feature" expression evaluates to "false" in the server. o There MUST be no nodes tagged with "when" present if the "when" condition evaluates to "false" in the data tree. The following properties are true in a valid data tree: o All "must" constraints MUST evaluate to "true". o All referential integrity constraints defined via the "path" statement MUST be satisfied. o All "unique" constraints on lists MUST be satisfied. o The "mandatory" constraint is enforced for leafs and choices, unless the node or any of its ancestors has a "when" condition or "if-feature" expression that evaluates to "false". (§8.1) | {gap}, no test | walkTree evaluates no when, unique or choice; plan/pre-release/spec-config-yang-when-unique-choice.md |
| [`RFC7950-8.3.1-2`](#rfc7950-8.3.1-2) If data for more than one case branch of a choice is present, the server MUST reply with a "bad-element" <error-tag> in the <rpc-error>. o If data for a node tagged with "if-feature" is present and the "if-feature" expression evaluates to "false" in the server, the server MUST reply with an "unknown-element" <error-tag> in the <rpc-error>. o If data for a node tagged with "when" is present and the "when" condition evaluates to "false", the server MUST reply with an "unknown-element" <error-tag> in the <rpc-error>. (§8.3.1) | {gap}, no test | walkTree evaluates no when, unique or choice; plan/pre-release/spec-config-yang-when-unique-choice.md |
| [`RFC7950-8.3.2-1`](#rfc7950-8.3.2-1) During this processing, the following errors MUST be detected: o Delete requests for non-existent data. o Create requests for existent data. o Insert requests with "before" or "after" parameters that do not exist. o Modification requests for nodes tagged with "when", and the "when" condition evaluates to "false". In this case, the server MUST reply with an "unknown-element" <error-tag> in the <rpc-error>. (§8.3.2) | {gap}, no test | walkTree evaluates no when, unique or choice; plan/pre-release/spec-config-yang-when-unique-choice.md |
| [`RFC7950-8.3.3-2`](#rfc7950-8.3.3-2) If the datastore is "running" or "startup", these constraints MUST be enforced at the end of the <edit-config> or <copy-config> operation. (§8.3.3) | {gap}, no test | LoadConfig (the <copy-config> window) and the web editor commit (internal/component/cli/editor_commit.go validateStagedTree, through config/cli.ValidateContent) refuse a type or ze:validate violation, but both grade a missing mandatory leaf a warning (SectionValidationError.Blocking) and evaluate no when, unique or choice, so the end of the operation does not enforce these constraints; plan/pre-release/spec-config-yang-when-unique-choice.md |
| [`RFC7950-9.2.4-2`](#rfc7950-9.2.4-2) If multiple values or ranges are given, they all MUST be disjoint and MUST be in ascending order. If a range restriction is applied to a type that is already range-restricted, the new restriction MUST be equally limiting or more limiting, i.e., raising the lower bounds, reducing the upper bounds, removing explicit values or ranges, or splitting ranges into multiple ranges with intermediate gaps. Each explicit value and range boundary value given in the range expression MUST match the type being restricted or be one of the special values "min" or "max". (§9.2.4) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-9.6.4-1`](#rfc7950-9.6.4-1) The "enum" statement, which is a substatement to the "type" statement, MUST be present if the type is "enumeration". It is repeatedly used to specify each assigned name of an enumeration type. It takes as an argument a string that is the assigned name. The string MUST NOT be zero-length and MUST NOT have any leading or trailing whitespace characters (any Unicode character with the "White_Space" property). The use of Unicode control codes SHOULD be avoided. The statement is optionally followed by a block of substatements that holds detailed enum information. All assigned names in an enumeration MUST be unique. (§9.6.4) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-9.6.4-2`](#rfc7950-9.6.4-2) When an existing enumeration type is restricted, the set of assigned names in the new type MUST be a subset of the base type's set of assigned names. The value of such an assigned name MUST NOT be changed. (§9.6.4) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-9.7-1`](#rfc7950-9.7-1) When an existing bits type is restricted, the set of assigned names in the new type MUST be a subset of the base type's set of assigned names. The bit position of such an assigned name MUST NOT be changed. (§9.7) | {gap}, no test | Ze offers no bits type; plan/spec-config-yang-type-bits.md |
| [`RFC7950-9.7.4-1`](#rfc7950-9.7.4-1) The "bit" statement, which is a substatement to the "type" statement, MUST be present if the type is "bits". It is repeatedly used to specify each assigned named bit of a bits type. It takes as an argument a string that is the assigned name of the bit. It is followed by a block of substatements that holds detailed bit information. The assigned name follows the same syntax rules as an identifier (see Section 6.2). All assigned names in a bits type MUST be unique. (§9.7.4) | {gap}, no test | Ze offers no bits type; plan/spec-config-yang-type-bits.md |
| [`RFC7950-9.7.4.2-1`](#rfc7950-9.7.4.2-1) The position value MUST be in the range 0 to 4294967295, and it MUST be unique within the bits type. If a bit position is not specified, then one will be automatically assigned. If the "bit" substatement is the first one defined, the assigned value is zero (0); otherwise, the assigned value is one greater than the current highest bit position (i.e., the highest bit position, implicit or explicit, prior to the current "bit" substatement in the parent "type" statement). Note that the presence of an "if-feature" statement in a "bit" statement does not affect the automatically assigned position. If the current highest bit position value is equal to 4294967295, then a position value MUST be specified for "bit" substatements following the one with the current highest position value. When an existing bits type is restricted, the "position" statement MUST either have the same value as in the base type or not be present, in which case the value is the same as in the base type. (§9.7.4.2) | {gap}, no test | Ze offers no bits type; plan/spec-config-yang-type-bits.md |
| [`RFC7950-9.9-1`](#rfc7950-9.9-1) If the "require-instance" property (Section 9.9.3) is "true", there MUST exist a node in the data tree, or a node with a default value in use (see Sections 7.6.1 and 7.7.2), of the referred schema tree leaf or leaf-list node with the same value as the leafref value in a valid data tree. If the referring node represents configuration data and the "require-instance" property (Section 9.9.3) is "true", the referred node MUST also represent configuration. There MUST NOT be any circular chains of leafrefs. If the leaf that the leafref refers to is conditional based on one or more features (see Section 7.20.2), then the leaf with the leafref type MUST also be conditional based on at least the same set of features. (§9.9) | {gap}, no test | Ze offers no leafref type; plan/spec-config-yang-type-leafref.md |
| [`RFC7950-9.9.2-1`](#rfc7950-9.9.2-1) The "path" statement, which is a substatement to the "type" statement, MUST be present if the type is "leafref". It takes as an argument a string that MUST refer to a leaf or leaf-list node. The syntax for a path argument is a subset of the XPath abbreviated syntax. Predicates are used only for constraining the values for the key nodes for list entries. Each predicate consists of exactly one equality test per key, and multiple adjacent predicates MAY be present if a list has multiple keys. The syntax is formally defined by the rule "path-arg" in Section 14. The predicates are only used when more than one key reference is needed to uniquely identify a leaf instance. This occurs if a list has multiple keys or a reference to a leaf other than the key in a list is needed. In these cases, multiple leafrefs are typically specified, and predicates are used to tie them together. The "path" expression evaluates to a node set consisting of zero, one, or more nodes. If the "require-instance" property is "true", this node set MUST be non-empty. (§9.9.2) | {gap}, no test | Ze offers no leafref type; plan/spec-config-yang-type-leafref.md |
| [`RFC7950-9.9.3-1`](#rfc7950-9.9.3-1) If "require-instance" is "true", it means that the instance being referred to MUST exist for the data to be valid. (Section 9.9.3) | {gap}, no test | Ze offers no leafref type; plan/spec-config-yang-type-leafref.md |
| [`RFC7950-9.10.2-1`](#rfc7950-9.10.2-1) The "base" statement, which is a substatement to the "type" statement, MUST be present at least once if the type is "identityref". The argument is the name of an identity, as defined by an "identity" statement. If a prefix is present on the identity name, it refers to an identity defined in the module that was imported with that prefix. Otherwise, an identity with the matching name MUST be defined in the current module or an included submodule. (§9.10.2) | {gap}, no test | Ze offers no identityref type; plan/spec-config-yang-type-identityref.md |
| [`RFC7950-9.10.3-1`](#rfc7950-9.10.3-1) Otherwise, an identity with the matching name MUST be defined in the current module or one of its submodules. (Section 9.10.3) | {gap}, no test | Ze offers no identityref type; plan/spec-config-yang-type-identityref.md |
| [`RFC7950-9.12-2`](#rfc7950-9.12-2) When the type is "union", the "type" statement (Section 7.4) MUST be present. (Section 9.12) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-9.13-1`](#rfc7950-9.13-1) For identifying list entries with keys, each predicate consists of one equality test per key, and each key MUST have a corresponding predicate. If a key is of type "empty", it is represented as a zero-length string (""). If the leaf with the instance-identifier type represents configuration data and the "require-instance" property (Section 9.9.3) is "true", the node it refers to MUST also represent configuration. Such a leaf puts a constraint on valid data. All such leaf nodes MUST reference existing nodes or leaf or leaf-list nodes with their default value in use (see Sections 7.6.1 and 7.7.2) for the data to be valid. (§9.13) | {gap}, no test | Ze offers no instance-identifier type; plan/spec-config-yang-type-instance-identifier.md |
| [`RFC7950-11-1`](#rfc7950-11-1) For any published change, a new "revision" statement (Section 7.1.9) MUST be included in front of the existing "revision" statements. If there are no existing "revision" statements, then one MUST be added to identify the new revision. Furthermore, any necessary changes MUST be applied to any metadata statements, including the "organization" and "contact" statements (Sections 7.1.7 and 7.1.8). Note that definitions contained in a module are available to be imported by any other module and are referenced in "import" statements via the module name. Thus, a module name MUST NOT be changed. Furthermore, the "namespace" statement MUST NOT be changed, since all XML elements are qualified by the namespace. Obsolete definitions MUST NOT be removed from published modules, since their identifiers may still be referenced by other modules. (§11) | {gap}, no test | no repository check reads the namespace or the revision history of the embedded modules; plan/spec-config-yang-authoring-checks.md |
| [`RFC7950-11-2`](#rfc7950-11-2) Otherwise, if the semantics of any previous definition are changed (i.e., if a non-editorial change is made to any definition other than those specifically allowed above), then this MUST be achieved by a new definition with a new identifier. (§11) | {gap}, no test | no repository check reads the namespace or the revision history of the embedded modules; plan/spec-config-yang-authoring-checks.md |
| [`RFC7950-11-3`](#rfc7950-11-3) In statements that have any data definition statements as substatements, those data definition substatements MUST NOT be reordered. (§11) | {gap}, no test | no repository check reads the namespace or the revision history of the embedded modules; plan/spec-config-yang-authoring-checks.md |
| [`RFC7950-7.2.2-1`](#rfc7950-7.2.2-1) A submodule MUST only be included by either the module to which it belongs or another submodule that belongs to that module. (§7.2.2) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/pre-release/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-7.21.5-2`](#rfc7950-7.21.5-2) If the XPath expression references any node that also has associated "when" statements, those "when" expressions MUST be evaluated first. There MUST NOT be any circular dependencies among "when" expressions. (§7.21.5) | {gap}, no test | walkTree evaluates no when, unique or choice; plan/pre-release/spec-config-yang-when-unique-choice.md |
| [`RFC7950-9.1-2`](#rfc7950-9.1-2) If the data type does not have a canonical form, the format of the value MUST match the data type's lexical representation, but the exact format is implementation dependent. (§9.1) | {gap}, no test | the types with no canonical form, identityref and instance-identifier, are absent from Ze; plan/spec-config-yang-type-identityref.md |
| [`RFC7950-12-1`](#rfc7950-12-1) A YANG version 1.1 module MUST NOT include a YANG version 1 submodule, and a YANG version 1 module MUST NOT include a YANG version 1.1 submodule. A YANG version 1 module or submodule MUST NOT import a YANG version 1.1 module by revision. (§12) | {gap}, no test | no Ze module declares yang-version and none is a submodule; plan/spec-config-yang-version-submodule.md |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7950-9.6-1`](#rfc7950-9.6-1)

The enumeration built-in type represents values from a set of assigned names. (§9.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: accepting a name not assigned in the enumeration, or refusing an assigned one. (b) TestValidateTree_FamilyModeEnum case "invalid-mode" and TestValidateTree_AddPathDirectionEnum case "both" require errs non-empty with errs[0].Type == ErrTypeEnum, and every defined name (enable/disable/require/ignore; send/receive/send/receive) assert.Empty; the radius auth-method ("mschapv2") and isis algorithm ("hmac-sha-999") negatives are refused ErrTypeEnum while every defined value is accepted.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISAuthAlgorithmEnumAcceptsAll`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_isis_auth_algorithm_enum_test.go#L81) | unit/verify | unproven |
| negative | [`TestRadiusAuthMethodEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_radius_auth_method_enum_test.go#L64) | unit/verify | revert, verified |
| negative | [`TestValidateTree_AddPathDirectionEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L619) | unit/verify | unproven |
| negative | [`TestValidateTree_FamilyModeEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L553) | unit/verify | unproven |
| positive | [`TestISISAuthAlgorithmEnumAcceptsAll`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_isis_auth_algorithm_enum_test.go#L64) | unit/verify | unproven |
| positive | [`TestRadiusAuthMethodEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_radius_auth_method_enum_test.go#L53) | unit/verify | revert, verified |
| positive | [`TestValidateTree_AddPathDirectionEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L618) | unit/verify | unproven |
| positive | [`TestValidateTree_FamilyModeEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L552) | unit/verify | unproven |

### [`RFC7950-9.12-1`](#rfc7950-9.12-1)

The union built-in type represents a value that corresponds to one of its member types. (§9.12)

Audit verdict: enforced (the tests do what the requirement demands), fresh. negative: "not-an-ip" matching no member refused with ErrTypeType (TestValidateTree_UnionViolation); positive: a value matching only the first (192.0.2.2), only the second (2001:db8::1) and only the third, enum "dynamic", member each accepted, so a union check trying one member alone goes red. Records on validateUnion for all three units.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateTree_UnionViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L366) | unit/verify | revert, verified |
| positive | [`TestRFC7950UnionAcceptsEveryMemberType`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_union_test.go#L32) | unit/verify | revert, verified |
| positive | [`TestValidateTree_ValidConfig`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L46) | unit/verify | revert, verified |

### [`RFC7950-7.6.5-1`](#rfc7950-7.6.5-1)

If "mandatory" is "true", the behavior of the constraint depends on the type of the leaf's closest ancestor node in the schema tree that is not a non-presence container (see Section 7.5.1): o If no such ancestor exists in the schema tree, the leaf MUST exist. (§7.6.5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Row narrowed 2026-09-30 to the lead-in and the no-ancestor branch. The three tags prove only that yang.Validator.ValidateTree reports ErrTypeMissing for a leaf inside a container present in the data. The stack does not make the leaf exist: SectionValidationError.Blocking grades ErrTypeMissing non-blocking, so LoadConfig, ze config validate and the editor commit all accept the config; walkTree never visits a non-presence container absent from the data, so a mandatory leaf whose only ancestors are such containers is not required; and bgp is not in validatedSections. TestValidator_MandatoryField checks the type only when errors.AsType succeeds. Owner ruling 4: carried by spec-config-yang-when-unique-choice AC-2.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateTree_MandatoryMissing`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L338) | unit/verify | unproven |
| negative | [`TestValidator_MandatoryField`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L935) | unit/verify | unproven |
| positive | [`TestValidator_MandatoryField`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L934) | unit/verify | unproven |

### [`RFC7950-7.6.5-2`](#rfc7950-7.6.5-2)

Otherwise, if this ancestor is a case node, the leaf MUST exist if any node from the case exists in the data tree. (§7.6.5)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. No producer handles the case branch: goyang keeps a case's leaves under ChoiceEntry and CaseEntry entries, and walkTree checks mandatory children and recurses only through the container's own Dir, so a mandatory leaf inside a case is never required. Owner ruling 4 (YANG third-party, gaps not closed); spec-config-yang-when-unique-choice.

No test carries RFC7950-7.6.5-2, so no unit is bound to it.

### [`RFC7950-7.6.5-3`](#rfc7950-7.6.5-3)

Otherwise, the leaf MUST exist if the ancestor node exists in the data tree. (§7.6.5)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. walkTree recurses only into data present in the map, so a mandatory leaf below a non-presence container absent from the data is never checked even when its list-entry or presence-container ancestor exists; and a missing mandatory leaf is non-blocking everywhere (SectionValidationError.Blocking). Owner ruling 4; spec-config-yang-when-unique-choice.

No test carries RFC7950-7.6.5-3, so no unit is bound to it.

### [`RFC7950-7.5.3-1`](#rfc7950-7.5.3-1)

All such constraints MUST evaluate to "true" for the data to be valid. (§7.5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.5.3-1, so no unit is bound to it.

### [`RFC7950-9.2.4-1`](#rfc7950-9.2.4-1)

If a range restriction is applied to a type that is already range-restricted, the new restriction MUST be equally limiting or more limiting, i.e., raising the lower bounds, reducing the upper bounds, removing explicit values or ranges, or splitting ranges into multiple ranges with intermediate gaps. (§9.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.2.4-1, so no unit is bound to it.

### [`RFC7950-9.4.5-1`](#rfc7950-9.4.5-1)

If the type has multiple "pattern" statements, the expressions are ANDed together, i.e., all such expressions have to match. (§9.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.4.5-1, so no unit is bound to it.

### [`RFC7950-x-1`](#rfc7950-x-1)

The "yang-version" statement specifies which version of the YANG language was used in developing the module. The statement's argument is a string. It MUST contain the value "1.1" for YANG modules defined based on this specification. (§7.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-x-1, so no unit is bound to it.

### [`RFC7950-x-2`](#rfc7950-x-2)

An "enumeration" type may have new enums added, provided the old enums's values do not change. (§11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-x-2, so no unit is bound to it.

### [`RFC7950-5.1-1`](#rfc7950-5.1-1)

Within a server, all module names MUST be unique. A module uses the "include" statement to list all its submodules. A module, or submodule belonging to that module, can reference definitions in the module and all submodules included by the module. A module or submodule uses the "import" statement to reference external modules. Statements in the module or submodule can reference definitions in the external module using the prefix specified in the "import" statement. For backward compatibility with YANG version 1, a submodule MAY use the "include" statement to reference other submodules within its module, but this is not necessary in YANG version 1.1. A submodule can reference any definition in the module it belongs to and in all submodules included by the module. A submodule MUST NOT include different revisions of other submodules than the revisions that its module includes. A module or submodule MUST NOT include submodules from other modules, and a submodule MUST NOT import its own module. The "import" and "include" statements are used to make definitions available from other modules: o For a module or submodule to reference definitions in an external module, the external module MUST be imported. o A module MUST include all its submodules. o A module, or submodule belonging to that module, MAY reference definitions in the module and all submodules included by the module. There MUST NOT be any circular chains of imports. For example, if module "a" imports module "b", "b" cannot import "a". When a definition in an external module is referenced, a locally defined prefix MUST be used, followed by a colon (":") and then the external identifier. (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-5.1-1, so no unit is bound to it.

### [`RFC7950-5.3-1`](#rfc7950-5.3-1)

Namespace URIs MUST be chosen so they cannot collide with standard or other enterprise namespaces -- for example, by using the enterprise or organization name in the namespace. (Section 5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-5.3-1, so no unit is bound to it.

### [`RFC7950-5.5-1`](#rfc7950-5.5-1)

Scoped definitions MUST NOT shadow definitions at a higher scope. (§5.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-5.5-1, so no unit is bound to it.

### [`RFC7950-5.6.5-1`](#rfc7950-5.6.5-1)

A server MUST NOT implement more than one revision of a module. If a server implements a module A that imports a module B, and A uses any node from B in an "augment" or "path" statement that the server supports, then the server MUST implement a revision of module B that has these nodes defined. (§5.6.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-5.6.5-1, so no unit is bound to it.

### [`RFC7950-6.1.3-1`](#rfc7950-6.1.3-1)

The backslash MUST NOT be followed by any other character. (§6.1.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a double-quoted string whose backslash is followed by a character other than n, t, double quote or backslash loading. (b) TestRFC7950ModuleLoadRefused case "bad escape" (description "bad \q escape") requires loadModuleTexts to return an error containing "escape"; positive TestRFC7950ModuleLoadAccepted case "valid escapes" loads a description carrying \t \n \" and \\ (require.NoError).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950ModuleLoadRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L65) | unit/verify | revert, verified |
| positive | [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L114) | unit/verify | revert, verified |

### [`RFC7950-6.2-1`](#rfc7950-6.2-1)

Implementations MUST support identifiers up to 64 characters in length and MAY support longer identifiers. (§6.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. {single-polarity} on the row. (a) forbidden: refusing or truncating a 64-character identifier. (b) TestRFC7950ModuleLoadAccepted loads a module whose leaf name is 64 characters (require.NoError on AddModuleFromText and Resolve) and assert.NotNil(entry.Dir[long]), which goes red if the name were refused or shortened.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L116) | unit/verify | revert, verified |

### [`RFC7950-6.2.1-1`](#rfc7950-6.2.1-1)

All identifiers defined in a namespace MUST be unique. o All module and submodule names share the same global module identifier namespace. o All extension names defined in a module and its submodules share the same extension identifier namespace. o All feature names defined in a module and its submodules share the same feature identifier namespace. o All identity names defined in a module and its submodules share the same identity identifier namespace. o All derived type names defined within a parent node or at the top level of the module or its submodules share the same type identifier namespace. This namespace is scoped to all descendant nodes of the parent node or module. This means that any descendant node may use that typedef, and it MUST NOT define a typedef with the same name. o All grouping names defined within a parent node or at the top level of the module or its submodules share the same grouping identifier namespace. This namespace is scoped to all descendant nodes of the parent node or module. This means that any descendant node may use that grouping, and it MUST NOT define a grouping with the same name. (§6.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-6.2.1-1, so no unit is bound to it.

### [`RFC7950-6.3.1-1`](#rfc7950-6.3.1-1)

When an imported extension is used, the extension's keyword MUST be qualified using the prefix with which the extension's module was imported. If an extension is used in the module where it is defined, the extension's keyword MUST be qualified with the prefix of this module. The processing of extensions depends on whether support for those extensions is claimed for a given YANG parser or the tool set in which it is embedded. An unsupported extension appearing in a YANG module as an unknown-statement (see Section 14) MAY be ignored in its entirety. Any supported extension MUST be processed in accordance with the specification governing that extension. (§6.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-6.3.1-1, so no unit is bound to it.

### [`RFC7950-6.4-1`](#rfc7950-6.4-1)

An implementation is not required to implement an XPath interpreter but MUST ensure that the requirements encoded in the data model are enforced. (Section 6.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-6.4-1, so no unit is bound to it.

### [`RFC7950-6.4-2`](#rfc7950-6.4-2)

The XPath expressions MUST be syntactically correct, and all prefixes used MUST be present in the XPath context (see Section 6.4.1). (§6.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-6.4-2, so no unit is bound to it.

### [`RFC7950-6.5-1`](#rfc7950-6.5-1)

References to identifiers defined in external modules MUST be qualified with appropriate prefixes, and references to identifiers defined in the current module and its submodules MAY use a prefix. (§6.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden, clause 1: resolving an unprefixed reference to an imported module's typedef; clause 2 (MAY): refusing a local reference written with or without the module's own prefix. (b) TestRFC7950ModuleLoadRefused case "external typedef without prefix" (type port with ext imported) requires an error containing "unknown type"; TestRFC7950ModuleLoadAccepted case "prefixed external and bare local" loads ext:port and bare local typedef l, and case "extension with own and import prefix" in the same tagged unit loads m:e written with the module's own prefix.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950ModuleLoadRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L68) | unit/verify | revert, verified |
| positive | [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L121) | unit/verify | revert, verified |

### [`RFC7950-7.1.4-1`](#rfc7950-7.1.4-1)

If there is a conflict, i.e., two different modules that both have defined the same prefix are imported, at least one of them MUST be imported with a different prefix. All prefixes, including the prefix for the module itself, MUST be unique within the module or submodule. (§7.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.1.4-1, so no unit is bound to it.

### [`RFC7950-7.3-1`](#rfc7950-7.3-1)

The "typedef" statement's argument is an identifier that is the name of the type to be defined and MUST be followed by a block of substatements that holds detailed typedef information. The name of the type MUST NOT be one of the YANG built-in types. If the typedef is defined at the top level of a YANG module or submodule, the name of the type to be defined MUST be unique within the module. (§7.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.3-1, so no unit is bound to it.

### [`RFC7950-7.3.4-1`](#rfc7950-7.3.4-1)

The value of the "default" statement MUST be valid according to the type specified in the "type" statement. If the base type has a default value and the new derived type does not specify a new default value, the base type's default value is also the default value of the new derived type. If the type's default value is not valid according to the new restrictions specified in a derived type or leaf definition, the derived type or leaf definition MUST specify a new default value compatible with the restrictions. (§7.3.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. all three sentences, both polarities, isolated by one-statement twins: (1) typedef default in/out of its own range (old pair, records on validateLeafDefaults); (2) a leaf of the typedef with no default carries 5, a narrowing leaf or typedef giving a new default carries 7/8 with no error (record on yangToLeaf); (3) a leaf and a typedef narrowing 1..10 to 6..10 with no new default each refused naming "5" (record on validateTypedefDefault).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950DerivedTypeMustReplaceAnInvalidatedDefault`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_clauses_test.go#L59) | unit/verify | revert, verified |
| negative | [`TestRFC7950DefaultInvalidForType`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_test.go#L47) | unit/verify | revert, verified |
| positive | [`TestRFC7950DerivedTypeInheritsOrReplacesTheDefault`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_clauses_test.go#L40) | unit/verify | revert, verified |
| positive | [`TestRFC7950DefaultValidForType`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_test.go#L32) | unit/verify | revert, verified |

### [`RFC7950-7.6.1-1`](#rfc7950-7.6.1-1)

The usage of the default value depends on the leaf's closest ancestor node in the schema tree that is not a non-presence container (see Section 7.5.1): o If no such ancestor exists in the schema tree, the default value MUST be used. (§7.6.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both halves of the no-ancestor branch are now tagged: TestRFC7950LeafDefaultUsedWhenAbsent (top-level leaf gets its default) and TestApplyDefaults_NonPresenceContainer (a leaf whose only ancestor is a non-presence container absent from the data: ApplyDefaults creates the container and fills 90/120, both values asserted). Negative TestRFC7950LeafDefaultNotUsedWhenSet (a set value keeps its value) is valid. Observed-red records: applyChildDefault for both RFC units, applyContainerDefault for the non-presence unit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950LeafDefaultNotUsedWhenSet`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_leaf_default_test.go#L42) | unit/verify | revert, verified |
| positive | [`TestRFC7950LeafDefaultUsedWhenAbsent`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_leaf_default_test.go#L14) | unit/verify | revert, verified |
| positive | [`TestApplyDefaults_NonPresenceContainer`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_schema_defaults_test.go#L38) | unit/verify | revert, verified |

### [`RFC7950-7.6.1-2`](#rfc7950-7.6.1-2)

Otherwise, if this ancestor is a case node, the default value MUST be used if any node from the case exists in the data tree or the case node is the choice's default case, and if no nodes from any other case exist in the data tree. (§7.6.1)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. flattenChoiceCases yields the data nodes of every case with no case node left in the schema, so ApplyDefaults fills the defaults of every case whether or not a node of that case exists or it is the default case. Owned by plan/pre-release/spec-config-yang-when-unique-choice.md.

No test carries RFC7950-7.6.1-2, so no unit is bound to it.

### [`RFC7950-7.6.1-3`](#rfc7950-7.6.1-3)

Otherwise, the default value MUST be used if the ancestor node exists in the data tree. (§7.6.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. §7.6.1 third branch, both polarities on one isolated unit (own schema build and map per subtest). Positive: an existing presence container and an existing list entry each receive their child leaf's default through ApplyDefaults. Negative: with both ancestors absent ApplyDefaults creates neither and the map stays empty. Records: positive on applyContainerDefault (revert); negative on the targeted mutant 'c.Presence' -> 'false' (a presence container treated as non-presence), observed red on the ancestor-absent subtest, so the negative discriminates the rule rather than a panic.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950LeafDefaultFollowsItsAncestor`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_leaf_default_test.go#L70) | unit/verify | mutant, verified |
| positive | [`TestRFC7950LeafDefaultFollowsItsAncestor`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_leaf_default_test.go#L69) | unit/verify | revert, verified |

### [`RFC7950-7.6.1-4`](#rfc7950-7.6.1-4)

In these cases, the default value is said to be in use. Note that if the leaf or any of its ancestors has a "when" condition or "if-feature" expression that evaluates to "false", then the default value is not in use. When the default value is in use, the server MUST operationally behave as if the leaf was present in the data tree with the default value as its value. (§7.6.1)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. ApplyDefaults evaluates no 'when' and no 'if-feature', so a default whose leaf or ancestor has a false 'when' stays in use. Owned by plan/pre-release/spec-config-yang-when-unique-choice.md.

No test carries RFC7950-7.6.1-4, so no unit is bound to it.

### [`RFC7950-7.6.3-1`](#rfc7950-7.6.3-1)

The "type" statement, which MUST be present, takes as an argument the name of an existing built-in or derived type. (Section 7.6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. both clauses, both polarities: MUST be present (leaf without type refused / built-in loads, loader_rfc7950_test) and names an existing type (type nosuch refused as unknown type / local typedef and imported prefixed typedef load; twins differ in the type name alone). Records on Resolve and AddModuleFromText.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950LeafTypeMustNameAnExistingType`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L38) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC7950ModuleLoadRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L74) | unit/verify | revert, verified |
| positive | [`TestRFC7950LeafTypeMustNameAnExistingType`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L39) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L128) | unit/verify | revert, verified |

### [`RFC7950-7.6.4-2`](#rfc7950-7.6.4-2)

The value of the "default" statement MUST be valid according to the type specified in the leaf's "type" statement. The "default" statement MUST NOT be present on nodes where "mandatory" is "true". The definition of the default value MUST NOT be marked with an "if-feature" statement. (§7.6.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. all three sentences, both polarities: (1) default valid for the type and (2) no default beside mandatory true, by TestRFC7950DefaultInvalidForType/ValidForType on validateLeafDefaults; (3) if-feature on the default's definition, for an enum named directly (TestRFC7950DefaultNotAnIfFeatureEnum) and through a union member enumeration, a bits type and a typedef chain (TestRFC7950DefaultNotAnIfFeatureMemberBitOrTypedef: x / 'b1 b2' / x refused naming the if-feature, y / 'b1' / y load). The union branch was a D-8 defect fixed here: validateDefaultNotIfFeature now follows the first member type that accepts the default (Section 9.12 order); red observed before the fix. Records on validateDefaultNotIfFeature and validateLeafDefaults.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950DefaultNotAnIfFeatureEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_clauses_test.go#L74) | unit/verify | revert, verified |
| negative | [`TestRFC7950DefaultNotAnIfFeatureMemberBitOrTypedef`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_clauses_test.go#L92) | unit/verify | revert, verified |
| negative | [`TestRFC7950DefaultInvalidForType`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_test.go#L48) | unit/verify | revert, verified |
| positive | [`TestRFC7950DefaultNotAnIfFeatureEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_clauses_test.go#L75) | unit/verify | revert, verified |
| positive | [`TestRFC7950DefaultNotAnIfFeatureMemberBitOrTypedef`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_clauses_test.go#L93) | unit/verify | revert, verified |
| positive | [`TestRFC7950DefaultValidForType`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_test.go#L33) | unit/verify | revert, verified |

### [`RFC7950-7.7-1`](#rfc7950-7.7-1)

In configuration data, the values in a leaf-list MUST be unique. The definitions of the default values MUST NOT be marked with an "if-feature" statement. Conceptually, the values in the data tree MUST be in the canonical form (see Section 9.1). (§7.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.7-1, so no unit is bound to it.

### [`RFC7950-7.7.2-1`](#rfc7950-7.7.2-1)

The usage of the default values depends on the leaf-list's closest ancestor node in the schema tree that is not a non-presence container (see Section 7.5.1): o If no such ancestor exists in the schema tree, the default values MUST be used. o Otherwise, if this ancestor is a case node, the default values MUST be used if any node from the case exists in the data tree or the case node is the choice's default case, and if no nodes from any other case exist in the data tree. o Otherwise, the default values MUST be used if the ancestor node exists in the data tree. In these cases, the default values are said to be in use. Note that if the leaf-list or any of its ancestors has a "when" condition or "if-feature" expression that evaluates to "false", then the default values are not in use. When the default values are in use, the server MUST operationally behave as if the leaf-list was present in the data tree with the default values as its values. (§7.7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.7.2-1, so no unit is bound to it.

### [`RFC7950-7.7.4-1`](#rfc7950-7.7.4-1)

The value of the "default" statement MUST be valid according to the type specified in the leaf-list's "type" statement. The "default" statement MUST NOT be present on nodes where "min-elements" has a value greater than or equal to one. (§7.7.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a leaf-list default invalid for its type loading; a leaf-list default beside min-elements >= 1 loading. (b) TestRFC7950DefaultInvalidForType: leaf-list range 1..10 default 30 requires an error containing `default "30" is not valid for type uint8`, and leaf-list min-elements 1 default 1 requires "min-elements 1 is invalid"; positive TestRFC7950DefaultValidForType: leaf-list default 3 inside 1..10 with no min-elements, assert.NoError. min-elements above 1 is not cased separately; 1 is the boundary.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950DefaultInvalidForType`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_test.go#L49) | unit/verify | revert, verified |
| positive | [`TestRFC7950DefaultValidForType`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_test.go#L34) | unit/verify | revert, verified |

### [`RFC7950-7.7.5-1`](#rfc7950-7.7.5-1)

A valid leaf-list or list MUST have at least min-elements entries. (Section 7.7.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: accepting a leaf-list or list with fewer entries than min-elements. (b) TestRFC7950MinElementsViolated: leaf-list with 1 of min 2, absent leaf-list with min 1, and list with 1 of min 2 each require.Len 1 and Contains "too few entries: N (minimum M)"; TestRFC7950MinElementsMet: leaf-list at exactly min 2 and list of 2 above min 1 assert.Empty.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950MinElementsViolated`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_test.go#L67) | unit/verify | revert, verified |
| positive | [`TestRFC7950MinElementsMet`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_test.go#L48) | unit/verify | revert, verified |

### [`RFC7950-7.8.2-1`](#rfc7950-7.8.2-1)

The "key" statement, which MUST be present if the list represents configuration and MAY be present otherwise, takes as an argument a string that specifies a space-separated list of one or more leaf identifiers of this list. A leaf identifier MUST NOT appear more than once in the key. Each such leaf identifier MUST refer to a child leaf of the list. The leafs can be defined directly in substatements to the list or in groupings used in the list. The combined values of all the leafs specified in the key are used to uniquely identify a list entry. All key leafs MUST be given values when a list entry is created. Thus, any default values in the key leafs or their types are ignored. Any "mandatory" statements in the key leafs are ignored. A leaf that is part of the key can be of any built-in or derived type. All key leafs in a list MUST have the same value for their "config" as the list itself. (§7.8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.8.2-1, so no unit is bound to it.

### [`RFC7950-7.8.3-1`](#rfc7950-7.8.3-1)

The "unique" statement is used to put constraints on valid list entries. It takes as an argument a string that contains a space- separated list of schema node identifiers, which MUST be given in the descendant form (see the rule "descendant-schema-nodeid" in Section 14). Each such schema node identifier MUST refer to a leaf. If one of the referenced leafs represents configuration data, then all of the referenced leafs MUST represent configuration data. The "unique" constraint specifies that the combined values of all the leaf instances specified in the argument string, including leafs with default values, MUST be unique within all list entry instances in which all referenced leafs exist or have default values. (§7.8.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.8.3-1, so no unit is bound to it.

### [`RFC7950-7.9.2-1`](#rfc7950-7.9.2-1)

The identifiers of all these child nodes MUST be unique within all cases in a choice. (§7.9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.9.2-1, so no unit is bound to it.

### [`RFC7950-7.9.3-1`](#rfc7950-7.9.3-1)

The "default" statement MUST NOT be present on choices where "mandatory" is "true". The default case is only important when considering the "default" statements of nodes under the cases (i.e., default values of leafs and leaf-lists, and default cases of nested choices). The default values and nested default cases under the default case are used if none of the nodes under any of the cases are present. There MUST NOT be any mandatory nodes (Section 3) directly under the default case. (§7.9.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.9.3-1, so no unit is bound to it.

### [`RFC7950-7.9.4-1`](#rfc7950-7.9.4-1)

If "mandatory" is "true", at least one node from exactly one of the choice's case branches MUST exist. (Section 7.9.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.9.4-1, so no unit is bound to it.

### [`RFC7950-7.12-1`](#rfc7950-7.12-1)

A grouping MUST NOT reference itself, neither directly nor indirectly through a chain of other groupings. If the grouping is defined at the top level of a YANG module or submodule, the grouping's identifier MUST be unique within the module. (§7.12)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.12-1, so no unit is bound to it.

### [`RFC7950-7.14.2-1`](#rfc7950-7.14.2-1)

If a leaf in the input tree has a "mandatory" statement with the value "true", the leaf MUST be present in an RPC invocation. If a leaf in the input tree has a default value, the server MUST use this value in the same cases as those described in Section 7.6.1. In these cases, the server MUST operationally behave as if the leaf was present in the RPC invocation with the default value as its value. If a leaf-list in the input tree has one or more default values, the server MUST use these values in the same cases as those described in Section 7.7.2. In these cases, the server MUST operationally behave as if the leaf-list was present in the RPC invocation with the default values as its values. Since the input tree is not part of any datastore, all "config" statements for nodes in the input tree are ignored. If any node has a "when" statement that would evaluate to "false", then this node MUST NOT be present in the input tree. (§7.14.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.14.2-1, so no unit is bound to it.

### [`RFC7950-7.14.3-1`](#rfc7950-7.14.3-1)

If a leaf in the output tree has a "mandatory" statement with the value "true", the leaf MUST be present in an RPC reply. If a leaf in the output tree has a default value, the client MUST use this value in the same cases as those described in Section 7.6.1. In these cases, the client MUST operationally behave as if the leaf was present in the RPC reply with the default value as its value. If a leaf-list in the output tree has one or more default values, the client MUST use these values in the same cases as those described in Section 7.7.2. In these cases, the client MUST operationally behave as if the leaf-list was present in the RPC reply with the default values as its values. Since the output tree is not part of any datastore, all "config" statements for nodes in the output tree are ignored. If any node has a "when" statement that would evaluate to "false", then this node MUST NOT be present in the output tree. (§7.14.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.14.3-1, so no unit is bound to it.

### [`RFC7950-7.15-1`](#rfc7950-7.15-1)

An action MUST NOT be defined within an rpc, another action, or a notification, i.e., an action node MUST NOT have an rpc, action, or a notification node as one of its ancestors in the schema tree. For example, this means that it is an error if a grouping that contains an action somewhere in its node hierarchy is used in a notification definition. An action MUST NOT have any ancestor node that is a list node without a "key" statement. (§7.15)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.15-1, so no unit is bound to it.

### [`RFC7950-7.16-1`](#rfc7950-7.16-1)

A notification MUST NOT be defined within an rpc, action, or another notification, i.e., a notification node MUST NOT have an rpc, action, or a notification node as one of its ancestors in the schema tree. For example, this means that it is an error if a grouping that contains a notification somewhere in its node hierarchy is used in an rpc definition. A notification MUST NOT have any ancestor node that is a list node without a "key" statement. Since a notification cannot be defined in a "case" statement, it is an error if a grouping that contains a notification at the top of its node hierarchy is used in a case definition. If a leaf in the notification tree has a "mandatory" statement with the value "true", the leaf MUST be present in a notification instance. If a leaf in the notification tree has a default value, the client MUST use this value in the same cases as those described in Section 7.6.1. In these cases, the client MUST operationally behave as if the leaf was present in the notification instance with the default value as its value. If a leaf-list in the notification tree has one or more default values, the client MUST use these values in the same cases as those described in Section 7.7.2. In these cases, the client MUST operationally behave as if the leaf-list was present in the notification instance with the default values as its values. (§7.16)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.16-1, so no unit is bound to it.

### [`RFC7950-7.17-1`](#rfc7950-7.17-1)

The target node MUST be either a container, list, choice, case, input, output, or notification node. It is augmented with the nodes defined in the substatements that follow the "augment" statement. The argument string is a schema node identifier (see Section 6.5). If the "augment" statement is on the top level in a module or submodule, the absolute form (defined by the rule "absolute-schema-nodeid" in Section 14) of a schema node identifier MUST be used. If the "augment" statement is a substatement to the "uses" statement, the descendant form (defined by the rule "descendant-schema-nodeid" in Section 14) MUST be used. If the target node is a container, list, case, input, output, or notification node, the "container", "leaf", "list", "leaf-list", "uses", and "choice" statements can be used within the "augment" statement. If the target node is a container or list node, the "action" and "notification" statements can be used within the "augment" statement. If the target node is a choice node, the "case" statement or a shorthand "case" statement (see Section 7.9.2) can be used within the "augment" statement. The "augment" statement MUST NOT add multiple nodes with the same name from the same module to the target node. If the augmentation adds mandatory nodes (see Section 3) that represent configuration to a target node in another module, the augmentation MUST be made conditional with a "when" statement. (§7.17)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.17-1, so no unit is bound to it.

### [`RFC7950-7.18.2-1`](#rfc7950-7.18.2-1)

If a prefix is present on the base name, it refers to an identity defined in the module that was imported with that prefix, or the local module if the prefix matches the local module's prefix. Otherwise, an identity with the matching name MUST be defined in the current module or an included submodule. An identity MUST NOT reference itself, neither directly nor indirectly through a chain of other identities. (§7.18.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.18.2-1, so no unit is bound to it.

### [`RFC7950-7.19-1`](#rfc7950-7.19-1)

Syntactically, the substatements MUST be YANG statements, including extensions defined using "extension" statements. YANG statements in extensions MUST follow the syntactical rules in Section 14. (§7.19)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. §7.19 binds the substatements of an extension USAGE (prefix:ext arg { ... }); both cases put the statement inside the extension DEFINITION, whose grammar refuses bogus for a different rule; a negative needs m:e "x" { bogus "y"; }

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950ModuleLoadRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L81) | unit/verify | revert, verified |
| positive | [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L136) | unit/verify | revert, verified |

### [`RFC7950-7.20.1-1`](#rfc7950-7.20.1-1)

A feature MUST NOT reference itself, neither directly nor indirectly through a chain of other features. In order for a server to support a feature that is dependent on any other features (i.e., the feature has one or more "if-feature" substatements), the server MUST also support all the dependent features. (§7.20.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.20.1-1, so no unit is bound to it.

### [`RFC7950-7.20.2-1`](#rfc7950-7.20.2-1)

If a prefix is present on a feature name in the boolean expression, the prefixed name refers to a feature defined in the module that was imported with that prefix, or the local module if the prefix matches the local module's prefix. Otherwise, a feature with the matching name MUST be defined in the current module or an included submodule. A leaf that is a list key MUST NOT have any "if-feature" statements. (§7.20.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.20.2-1, so no unit is bound to it.

### [`RFC7950-7.20.3-1`](#rfc7950-7.20.3-1)

This means that deviations MUST never be part of a published standard, since they are the mechanism for learning how implementations vary from the standards. Server deviations are strongly discouraged and MUST only be used as a last resort. Telling the application how a server fails to follow a standard is no substitute for implementing the standard correctly. A server that deviates from a module is not fully compliant with the module. However, in some cases, a particular device may not have the hardware or software ability to support parts of a standard module. When this occurs, the server makes a choice to either treat attempts to configure unsupported parts of the module as an error that is reported back to the unsuspecting application or ignore those incoming requests. Neither choice is acceptable. Instead, YANG allows servers to document portions of a base module that are not supported, or that are supported but with different syntax, by using the "deviation" statement. After applying all deviations announced by a server, in any order, the resulting data model MUST still be valid. (§7.20.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.20.3-1, so no unit is bound to it.

### [`RFC7950-7.20.3.2-1`](#rfc7950-7.20.3.2-1)

The argument "add" adds properties to the target node. The properties to add are identified by substatements to the "deviate" statement. If a property can only appear once, the property MUST NOT exist in the target node. The argument "replace" replaces properties of the target node. The properties to replace are identified by substatements to the "deviate" statement. The properties to replace MUST exist in the target node. The argument "delete" deletes properties from the target node. The properties to delete are identified by substatements to the "delete" statement. The substatement's keyword MUST match a corresponding keyword in the target node, and the argument's string MUST be equal to the corresponding keyword's argument string in the target node. (§7.20.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.20.3.2-1, so no unit is bound to it.

### [`RFC7950-7.21.2-1`](#rfc7950-7.21.2-1)

If a definition is "current", it MUST NOT reference a "deprecated" or "obsolete" definition within the same module. If a definition is "deprecated", it MUST NOT reference an "obsolete" definition within the same module. (§7.21.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.21.2-1, so no unit is bound to it.

### [`RFC7950-7.21.5-1`](#rfc7950-7.21.5-1)

A leaf that is a list key MUST NOT have a "when" statement. If a key leaf is defined in a grouping that is used in a list, the "uses" statement MUST NOT have a "when" statement. (§7.21.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-7.21.5-1, so no unit is bound to it.

### [`RFC7950-8.1-1`](#rfc7950-8.1-1)

These constraints are enforced in different ways, depending on what type of data the statement defines. o If the constraint is defined on configuration data, it MUST be true in a valid configuration data tree. o If the constraint is defined on state data, it MUST be true in a valid state data tree. o If the constraint is defined on notification content, it MUST be true in any notification data tree. o If the constraint is defined on RPC or action input parameters, it MUST be true in an invocation of the RPC or action operation. o If the constraint is defined on RPC or action output parameters, it MUST be true in the RPC or action reply. (§8.1)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. the prefix says every constraint MUST be true in a valid configuration tree; walkTree evaluates no 'when', 'unique' or choice constraint, so a configuration violating one is accepted. Owned by plan/pre-release/spec-config-yang-when-unique-choice.md.

No test carries RFC7950-8.1-1, so no unit is bound to it.

### [`RFC7950-8.1-3`](#rfc7950-8.1-3)

All leaf data values MUST match the type constraints for the leaf, including those defined in the type's "range", "length", and "pattern" properties. (§8.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. range: receive-hold-time 2 and port 0 refused ErrTypeRange (TestValidateTree_RangeViolation), 1 and 2 refused and 0, 3, 180, 65535 accepted (TestValidator_HoldTimeRange); pattern: router-id not-an-ip refused ErrTypePattern and 192.0.2.1 accepted; length: a 256-char isis hostname refused ErrTypeLength, and the same unit asserts a within-length hostname raises no length error (the positive half is asserted in the negative-tagged unit). Each input trips only the constraint named. Records on checkYangRange and validateString. Base-type conformance (a value of the wrong type) is carried by the Section 9 rows.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateTree_LengthViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L417) | unit/verify | revert, verified |
| negative | [`TestValidateTree_PatternViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L316) | unit/verify | revert, verified |
| negative | [`TestValidateTree_RangeViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L175) | unit/verify | revert, verified |
| negative | [`TestValidator_HoldTimeRange`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L900) | unit/verify | revert, verified |
| negative | [`TestValidator_ValidatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L851) | unit/verify | revert, verified |
| positive | [`TestValidator_HoldTimeRange`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L899) | unit/verify | revert, verified |
| positive | [`TestValidator_ValidatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L850) | unit/verify | revert, verified |

### [`RFC7950-8.1-4`](#rfc7950-8.1-4)

All key leafs MUST be present for all list entries. o Nodes MUST be present for at most one case branch in all choices. o There MUST be no nodes tagged with "if-feature" present if the "if-feature" expression evaluates to "false" in the server. o There MUST be no nodes tagged with "when" present if the "when" condition evaluates to "false" in the data tree. The following properties are true in a valid data tree: o All "must" constraints MUST evaluate to "true". o All referential integrity constraints defined via the "path" statement MUST be satisfied. o All "unique" constraints on lists MUST be satisfied. o The "mandatory" constraint is enforced for leafs and choices, unless the node or any of its ancestors has a "when" condition or "if-feature" expression that evaluates to "false". (§8.1)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. walkTree checks mandatory, type and leaf-list cardinality but evaluates no 'when', no 'unique', no choice exclusivity and no if-feature, so the at-most-one-case, when, unique and mandatory-under-false-when bullets are not enforced. Owned by plan/pre-release/spec-config-yang-when-unique-choice.md.

No test carries RFC7950-8.1-4, so no unit is bound to it.

### [`RFC7950-8.1-2`](#rfc7950-8.1-2)

The running configuration datastore MUST always be valid. (Section 8.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The tagged units drive config.LoadConfig directly: TestRFC7950RunningDatastoreRefusesAViolationAfterParse requires a nil result and ErrCustomValidation for a ze:validate violation, and TestRFC7950RunningDatastoreAcceptsAValidConfig loads a valid config. "Always" binds every route into the running configuration, and no tagged unit drives one: neither the daemon install path (the load closure runReload receives, cmd/ze/hub/main_reload.go) nor the editor commit (internal/component/cli/editor_commit.go, whose validateStagedTree is a no-op when preCommitValidate is nil). A route that installed a tree without LoadConfig leaves every tagged assertion green. The YANG-type violation is asserted only in TestRFC7950RunningDatastoreRefusesAViolationAtParse, tagged to RFC7950-8.3-1, not to this row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950RunningDatastoreRefusesAViolationAfterParse`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_datastore_test.go#L77) | unit/verify | revert, verified |
| positive | [`TestRFC7950RunningDatastoreAcceptsAValidConfig`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_datastore_test.go#L33) | unit/verify | revert, verified |

### [`RFC7950-8.3-1`](#rfc7950-8.3-1)

For configuration data, there are three windows when constraints MUST be enforced: o during parsing of RPC payloads o during processing of the <edit-config> operation o during validation (§8.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. parse window and whole-tree validation window are proven through LoadConfig; the edit-processing window (the config editor set/commit path) is untested

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950RunningDatastoreRefusesAViolationAfterParse`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_datastore_test.go#L78) | unit/verify | revert, verified |
| negative | [`TestRFC7950RunningDatastoreRefusesAViolationAtParse`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_datastore_test.go#L53) | unit/verify | revert, verified |
| positive | [`TestRFC7950RunningDatastoreAcceptsAValidConfig`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_datastore_test.go#L34) | unit/verify | revert, verified |

### [`RFC7950-8.3.1-2`](#rfc7950-8.3.1-2)

If data for more than one case branch of a choice is present, the server MUST reply with a "bad-element" <error-tag> in the <rpc-error>. o If data for a node tagged with "if-feature" is present and the "if-feature" expression evaluates to "false" in the server, the server MUST reply with an "unknown-element" <error-tag> in the <rpc-error>. o If data for a node tagged with "when" is present and the "when" condition evaluates to "false", the server MUST reply with an "unknown-element" <error-tag> in the <rpc-error>. (§8.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-8.3.1-2, so no unit is bound to it.

### [`RFC7950-8.3.2-1`](#rfc7950-8.3.2-1)

During this processing, the following errors MUST be detected: o Delete requests for non-existent data. o Create requests for existent data. o Insert requests with "before" or "after" parameters that do not exist. o Modification requests for nodes tagged with "when", and the "when" condition evaluates to "false". In this case, the server MUST reply with an "unknown-element" <error-tag> in the <rpc-error>. (§8.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-8.3.2-1, so no unit is bound to it.

### [`RFC7950-8.3.3-1`](#rfc7950-8.3.3-1)

When datastore processing is complete, the final contents MUST obey all validation constraints. (§8.3.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Row narrowed 2026-09-30 to the first sentence. The tags prove LoadConfig refuses a ze:validate violation with ErrCustomValidation and no partial tree, and accepts a valid config. "All validation constraints" is not met by the stack: a missing mandatory leaf is only a warning (SectionValidationError.Blocking), and when, unique and choice are evaluated nowhere, so a final datastore content violating those is accepted. Owner ruling 4: carried by spec-config-yang-when-unique-choice AC-2.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950RunningDatastoreRefusesAViolationAfterParse`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_datastore_test.go#L79) | unit/verify | revert, verified |
| positive | [`TestRFC7950RunningDatastoreAcceptsAValidConfig`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_datastore_test.go#L35) | unit/verify | revert, verified |

### [`RFC7950-8.3.3-2`](#rfc7950-8.3.3-2)

If the datastore is "running" or "startup", these constraints MUST be enforced at the end of the <edit-config> or <copy-config> operation. (§8.3.3)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. LoadConfig (copy-config window) and the web editor commit (validateStagedTree through config/cli.ValidateContent) refuse type and ze:validate violations, but both grade a missing mandatory leaf a warning via SectionValidationError.Blocking and evaluate no when, unique or choice, so the constraints are not enforced at the end of the operation. Owner ruling 4; spec-config-yang-when-unique-choice.

No test carries RFC7950-8.3.3-2, so no unit is bound to it.

### [`RFC7950-9.1-1`](#rfc7950-9.1-1)

Implementations MUST support all lexical representations specified in this document. (§9.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. decimal64 lexical forms both polarities, and integer "+17" now accepted for uint8 (validateUnsigned fixed) and int8 (positive only, record on validateUnsigned). Missing: hexadecimal and octal integer forms in a module default (Section 9.2.1, "0x10" refused by ParseUint base 10), and bits, binary, empty, identityref fall to validateYangType's default arm with no lexical check or tagged case. Re-judged 2026-09-30: the decimal64 units changed only by their RFC7950-9.3.2-1 tags moving to RFC7950-9.3.1-1; the gaps named stand.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950Decimal64Refused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_decimal64_test.go#L40) | unit/verify | revert, verified |
| positive | [`TestRFC7950Decimal64Accepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_decimal64_test.go#L25) | unit/verify | revert, verified |
| positive | [`TestRFC7950IntegerSignedLexicalForms`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_integer_test.go#L14) | unit/verify | revert, verified |

### [`RFC7950-9.2.4-2`](#rfc7950-9.2.4-2)

If multiple values or ranges are given, they all MUST be disjoint and MUST be in ascending order. If a range restriction is applied to a type that is already range-restricted, the new restriction MUST be equally limiting or more limiting, i.e., raising the lower bounds, reducing the upper bounds, removing explicit values or ranges, or splitting ranges into multiple ranges with intermediate gaps. Each explicit value and range boundary value given in the range expression MUST match the type being restricted or be one of the special values "min" or "max". (§9.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.2.4-2, so no unit is bound to it.

### [`RFC7950-9.3.1-1`](#rfc7950-9.3.1-1)

A decimal64 value is lexically represented as an optional sign ("+" or "-"), followed by a sequence of decimal digits, optionally followed by a period ('.') as a decimal indicator and a sequence of decimal digits. (§9.3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. every element of the §9.3.1 lexical form, both polarities through validateYangType on one decimal64 type: '+1.5', '-1.5', '1', '0.25', '0.0', '10.50' accepted; '.5' (no leading digit), '1.' (period without digits), '+', '-', '1.2.3', 'abc', '' each refused with ErrTypeType and 'expected decimal64'. Fraction-digits and range refusals in the same unit are distinct asserted kinds. Records on validateDecimal64.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950Decimal64Refused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_decimal64_test.go#L41) | unit/verify | revert, verified |
| positive | [`TestRFC7950Decimal64Accepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_decimal64_test.go#L26) | unit/verify | revert, verified |

### [`RFC7950-9.3.4-1`](#rfc7950-9.3.4-1)

The "fraction-digits" statement, which is a substatement to the "type" statement, MUST be present if the type is "decimal64". (Section 9.3.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a decimal64 type with no fraction-digits substatement loading. loader_rfc7950_test.go TestRFC7950ModuleLoadRefused case "decimal64 without fraction-digits" requires loadModuleTexts to return an error containing "[1..18]"; positive: TestRFC7950ModuleLoadAccepted case "decimal64 with fraction-digits" requires the module with fraction-digits 2 to load (require.NoError).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950ModuleLoadRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L85) | unit/verify | revert, verified |
| positive | [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L139) | unit/verify | revert, verified |

### [`RFC7950-9.4.4-1`](#rfc7950-9.4.4-1)

Length-restricting values MUST NOT be negative. If multiple values or ranges are given, they all MUST be disjoint and MUST be in ascending order. (§9.4.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clause 1 (MUST NOT be negative) is enforced: case "length negative" refuses length "-1..5". Clauses 2 and 3 (multiple parts MUST be disjoint and in ascending order) have no red assertion: the only other refused case "length descending" is a single range "5..1" with reversed bounds, not multiple parts out of order; no case refuses overlapping parts such as "1..5 | 3..8" or descending parts such as "10..20 | 1..5".

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950ModuleLoadRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L88) | unit/verify | revert, verified |
| positive | [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L141) | unit/verify | revert, verified |

### [`RFC7950-9.5.1-1`](#rfc7950-9.5.1-1)

The lexical representation of a boolean value is a string with a value of "true" or "false". These values MUST be in lowercase. (§9.5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting a boolean not spelled lowercase true or false. validator_rfc7950_test.go TestRFC7950BooleanNotLowercaseRefused requires validateYangType to return a ValidationError of type ErrTypeType with message "expected boolean" for "True", "FALSE", "TRUE", "yes" and "1"; TestRFC7950BooleanLowercaseAccepted requires "true" and "false" to pass (assert.NoError).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950BooleanNotLowercaseRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_test.go#L104) | unit/verify | revert, verified |
| positive | [`TestRFC7950BooleanLowercaseAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_test.go#L93) | unit/verify | revert, verified |

### [`RFC7950-9.6.4-1`](#rfc7950-9.6.4-1)

The "enum" statement, which is a substatement to the "type" statement, MUST be present if the type is "enumeration". It is repeatedly used to specify each assigned name of an enumeration type. It takes as an argument a string that is the assigned name. The string MUST NOT be zero-length and MUST NOT have any leading or trailing whitespace characters (any Unicode character with the "White_Space" property). The use of Unicode control codes SHOULD be avoided. The statement is optionally followed by a block of substatements that holds detailed enum information. All assigned names in an enumeration MUST be unique. (§9.6.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.6.4-1, so no unit is bound to it.

### [`RFC7950-9.6.4-2`](#rfc7950-9.6.4-2)

When an existing enumeration type is restricted, the set of assigned names in the new type MUST be a subset of the base type's set of assigned names. The value of such an assigned name MUST NOT be changed. (§9.6.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.6.4-2, so no unit is bound to it.

### [`RFC7950-9.6.4.2-1`](#rfc7950-9.6.4.2-1)

This integer value MUST be in the range -2147483648 to 2147483647, and it MUST be unique within the enumeration type. If a value is not specified, then one will be automatically assigned. If the "enum" substatement is the first one defined, the assigned value is zero (0); otherwise, the assigned value is one greater than the current highest enum value (i.e., the highest enum value, implicit or explicit, prior to the current "enum" substatement in the parent "type" statement). Note that the presence of an "if-feature" statement in an "enum" statement does not affect the automatically assigned value. If the current highest value is equal to 2147483647, then an enum value MUST be specified for "enum" substatements following the one with the current highest value. When an existing enumeration type is restricted, the "value" statement MUST either have the same value as in the base type or not be present, in which case the value is the same as in the base type. (§9.6.4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. int32 range now proven at both ends (-2147483649 refused / -2147483648 loads, 2147483648 refused), uniqueness and the value-required-after-2147483647 rule proven. Missing: the last sentence, a restricted enumeration changing a base value, is ACCEPTED by goyang (untagged red TestRFC7950EnumRestrictionKeepsTheBaseValue, spec-config-yang-loader-structural-checks); automatic value assignment (0 first, highest+1, if-feature ignored) has no tagged assertion.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950EnumValueWithinInt32`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L105) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC7950ModuleLoadRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L92) | unit/verify | revert, verified |
| positive | [`TestRFC7950EnumValueWithinInt32`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L106) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L145) | unit/verify | revert, verified |

### [`RFC7950-9.7-1`](#rfc7950-9.7-1)

When an existing bits type is restricted, the set of assigned names in the new type MUST be a subset of the base type's set of assigned names. The bit position of such an assigned name MUST NOT be changed. (§9.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.7-1, so no unit is bound to it.

### [`RFC7950-9.7.4-1`](#rfc7950-9.7.4-1)

The "bit" statement, which is a substatement to the "type" statement, MUST be present if the type is "bits". It is repeatedly used to specify each assigned named bit of a bits type. It takes as an argument a string that is the assigned name of the bit. It is followed by a block of substatements that holds detailed bit information. The assigned name follows the same syntax rules as an identifier (see Section 6.2). All assigned names in a bits type MUST be unique. (§9.7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.7.4-1, so no unit is bound to it.

### [`RFC7950-9.7.4.2-1`](#rfc7950-9.7.4.2-1)

The position value MUST be in the range 0 to 4294967295, and it MUST be unique within the bits type. If a bit position is not specified, then one will be automatically assigned. If the "bit" substatement is the first one defined, the assigned value is zero (0); otherwise, the assigned value is one greater than the current highest bit position (i.e., the highest bit position, implicit or explicit, prior to the current "bit" substatement in the parent "type" statement). Note that the presence of an "if-feature" statement in a "bit" statement does not affect the automatically assigned position. If the current highest bit position value is equal to 4294967295, then a position value MUST be specified for "bit" substatements following the one with the current highest position value. When an existing bits type is restricted, the "position" statement MUST either have the same value as in the base type or not be present, in which case the value is the same as in the base type. (§9.7.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.7.4.2-1, so no unit is bound to it.

### [`RFC7950-9.9-1`](#rfc7950-9.9-1)

If the "require-instance" property (Section 9.9.3) is "true", there MUST exist a node in the data tree, or a node with a default value in use (see Sections 7.6.1 and 7.7.2), of the referred schema tree leaf or leaf-list node with the same value as the leafref value in a valid data tree. If the referring node represents configuration data and the "require-instance" property (Section 9.9.3) is "true", the referred node MUST also represent configuration. There MUST NOT be any circular chains of leafrefs. If the leaf that the leafref refers to is conditional based on one or more features (see Section 7.20.2), then the leaf with the leafref type MUST also be conditional based on at least the same set of features. (§9.9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.9-1, so no unit is bound to it.

### [`RFC7950-9.9.2-1`](#rfc7950-9.9.2-1)

The "path" statement, which is a substatement to the "type" statement, MUST be present if the type is "leafref". It takes as an argument a string that MUST refer to a leaf or leaf-list node. The syntax for a path argument is a subset of the XPath abbreviated syntax. Predicates are used only for constraining the values for the key nodes for list entries. Each predicate consists of exactly one equality test per key, and multiple adjacent predicates MAY be present if a list has multiple keys. The syntax is formally defined by the rule "path-arg" in Section 14. The predicates are only used when more than one key reference is needed to uniquely identify a leaf instance. This occurs if a list has multiple keys or a reference to a leaf other than the key in a list is needed. In these cases, multiple leafrefs are typically specified, and predicates are used to tie them together. The "path" expression evaluates to a node set consisting of zero, one, or more nodes. If the "require-instance" property is "true", this node set MUST be non-empty. (§9.9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.9.2-1, so no unit is bound to it.

### [`RFC7950-9.9.3-1`](#rfc7950-9.9.3-1)

If "require-instance" is "true", it means that the instance being referred to MUST exist for the data to be valid. (Section 9.9.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.9.3-1, so no unit is bound to it.

### [`RFC7950-9.10.2-1`](#rfc7950-9.10.2-1)

The "base" statement, which is a substatement to the "type" statement, MUST be present at least once if the type is "identityref". The argument is the name of an identity, as defined by an "identity" statement. If a prefix is present on the identity name, it refers to an identity defined in the module that was imported with that prefix. Otherwise, an identity with the matching name MUST be defined in the current module or an included submodule. (§9.10.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.10.2-1, so no unit is bound to it.

### [`RFC7950-9.10.3-1`](#rfc7950-9.10.3-1)

Otherwise, an identity with the matching name MUST be defined in the current module or one of its submodules. (Section 9.10.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.10.3-1, so no unit is bound to it.

### [`RFC7950-9.12-2`](#rfc7950-9.12-2)

When the type is "union", the "type" statement (Section 7.4) MUST be present. (Section 9.12)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.12-2, so no unit is bound to it.

### [`RFC7950-9.13-1`](#rfc7950-9.13-1)

For identifying list entries with keys, each predicate consists of one equality test per key, and each key MUST have a corresponding predicate. If a key is of type "empty", it is represented as a zero-length string (""). If the leaf with the instance-identifier type represents configuration data and the "require-instance" property (Section 9.9.3) is "true", the node it refers to MUST also represent configuration. Such a leaf puts a constraint on valid data. All such leaf nodes MUST reference existing nodes or leaf or leaf-list nodes with their default value in use (see Sections 7.6.1 and 7.7.2) for the data to be valid. (§9.13)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.13-1, so no unit is bound to it.

### [`RFC7950-11-1`](#rfc7950-11-1)

For any published change, a new "revision" statement (Section 7.1.9) MUST be included in front of the existing "revision" statements. If there are no existing "revision" statements, then one MUST be added to identify the new revision. Furthermore, any necessary changes MUST be applied to any metadata statements, including the "organization" and "contact" statements (Sections 7.1.7 and 7.1.8). Note that definitions contained in a module are available to be imported by any other module and are referenced in "import" statements via the module name. Thus, a module name MUST NOT be changed. Furthermore, the "namespace" statement MUST NOT be changed, since all XML elements are qualified by the namespace. Obsolete definitions MUST NOT be removed from published modules, since their identifiers may still be referenced by other modules. (§11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-11-1, so no unit is bound to it.

### [`RFC7950-11-2`](#rfc7950-11-2)

Otherwise, if the semantics of any previous definition are changed (i.e., if a non-editorial change is made to any definition other than those specifically allowed above), then this MUST be achieved by a new definition with a new identifier. (§11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-11-2, so no unit is bound to it.

### [`RFC7950-11-3`](#rfc7950-11-3)

In statements that have any data definition statements as substatements, those data definition substatements MUST NOT be reordered. (§11)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. {gap} under OWNER RULING 4 (2026-09-30): no check compares an embedded module with its previous revision, so a reordering of data definition substatements passes. Recorded, not closed.

No test carries RFC7950-11-3, so no unit is bound to it.

### [`RFC7950-7.2.2-1`](#rfc7950-7.2.2-1)

A submodule MUST only be included by either the module to which it belongs or another submodule that belongs to that module. (§7.2.2)

Audit verdict: unimplemented (no code path enforces the requirement), stale-unit: internal/component/config/yang/loader.go::Resolve moved. {gap} under OWNER RULING 4: goyang accepts a submodule included by a foreign module and Ze adds no check. Recorded, not closed.

No test carries RFC7950-7.2.2-1, so no unit is bound to it.

### [`RFC7950-7.3.2-1`](#rfc7950-7.3.2-1)

The "type" statement, which MUST be present, defines the base type from which this type is derived. (§7.3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. negative: a typedef with only a description refused; positive: the same typedef with type uint8 loads and the leaf using it resolves to Yuint8 (proves the type defines the base type). Twins differ by the type statement alone, so the loose "type" substring cannot be met by another fault. Records on Resolve.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950TypedefTypeMustBePresent`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L53) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC7950TypedefTypeMustBePresent`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L54) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`RFC7950-7.9.2-2`](#rfc7950-7.9.2-2)

The case identifier MUST be unique within a choice. (§7.9.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. negative: two cases named x in one choice refused as duplicate; positive: cases x and y load, and case x in two different choices loads, which pins the scope to one choice. Records on Resolve.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950CaseIdentifierUniqueWithinAChoice`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L76) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC7950CaseIdentifierUniqueWithinAChoice`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L77) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`RFC7950-7.21.5-2`](#rfc7950-7.21.5-2)

If the XPath expression references any node that also has associated "when" statements, those "when" expressions MUST be evaluated first. There MUST NOT be any circular dependencies among "when" expressions. (§7.21.5)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. {gap} under OWNER RULING 4: walkTree evaluates no when expression, so neither the order nor the no-cycle rule is applied. Recorded, not closed.

No test carries RFC7950-7.21.5-2, so no unit is bound to it.

### [`RFC7950-9.1-2`](#rfc7950-9.1-2)

If the data type does not have a canonical form, the format of the value MUST match the data type's lexical representation, but the exact format is implementation dependent. (§9.1)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. {gap} under OWNER RULING 4: the types without a canonical form (identityref, instance-identifier) are not validated by Ze. Recorded, not closed.

No test carries RFC7950-9.1-2, so no unit is bound to it.

### [`RFC7950-9.4.4-2`](#rfc7950-9.4.4-2)

If a length restriction is applied to a type that is already length-restricted, the new restriction MUST be equally limiting or more limiting, i.e., raising the lower bounds, reducing the upper bounds, removing explicit length values or ranges, or splitting ranges into multiple ranges with intermediate gaps. (§9.4.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. negative: widening a typedef's length 2..5 to 1..10 refused naming length; positive: equal 2..5, narrowed 3..4, and split 2..3 | 5..5 load, covering the allowed restriction kinds. Records on Resolve.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950LengthRestrictionMustNotWiden`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L92) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC7950LengthRestrictionMustNotWiden`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L93) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`RFC7950-12-1`](#rfc7950-12-1)

A YANG version 1.1 module MUST NOT include a YANG version 1 submodule, and a YANG version 1 module MUST NOT include a YANG version 1.1 submodule. A YANG version 1 module or submodule MUST NOT import a YANG version 1.1 module by revision. (§12)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-12-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-30 |
| Register | rfc2119 |
| Source | rfc/full/rfc7950.txt |
| Source fingerprint | e4e6e164b23abaf7 |
| Record | rfc/extraction/rfc7950.json |
| Mapped sentences | 190 |
| Declined as scope | 37 |
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
| `8.3.1:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the NETCONF protocol binding of YANG, and Section 1 makes that binding one of several: "YANG has been used or proposed to be used for other protocols". ze serves its YANG-modeled configuration over gNMI and its own CLI and implements no NETCONF server, no ietf-yang-library module and no rpc-error envelope: no Go file under internal/ names NETCONF except a port-name table (internal/core/portname/services_table.go). The type constraints the reply follows are Section 8.1's, row RFC7950-8.1-3 | o If a leaf data value does not match the type constraints for the leaf, including those defined in the type's "range", "length", and "pattern" properties, the server MUST reply with an "invalid-value" <error-tag> in the <rpc-error>, and with the error-app-tag (Section 7.5.4.2) and error-message (Section 7.5.4.1) associated with the constraint, if any exist. o If all keys of a list entry are not present, the server MUST reply with a "missing-element" <error-tag> in the <rpc-error>. |
| `8.3.1:6` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the NETCONF protocol binding of YANG, and Section 1 makes that binding one of several: "YANG has been used or proposed to be used for other protocols". ze serves its YANG-modeled configuration over gNMI and its own CLI and implements no NETCONF server, no ietf-yang-library module and no rpc-error envelope: no Go file under internal/ names NETCONF except a port-name table (internal/core/portname/services_table.go) | o For insert handling, if the values for the attributes "before" and "after" are not valid for the type of the appropriate key leafs, the server MUST reply with a "bad-attribute" <error-tag> in the <rpc-error>. |
| `8.3.1:7` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the NETCONF protocol binding of YANG, and Section 1 makes that binding one of several: "YANG has been used or proposed to be used for other protocols". ze serves its YANG-modeled configuration over gNMI and its own CLI and implements no NETCONF server, no ietf-yang-library module and no rpc-error envelope: no Go file under internal/ names NETCONF except a port-name table (internal/core/portname/services_table.go) | o If the attributes "before" and "after" appear in any element that is not a list whose "ordered-by" property is "user", the server MUST reply with an "unknown-attribute" <error-tag> in the <rpc-error>. |
| `9.1:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation binds the XML encoding of YANG data, and Section 1 makes that encoding one of several: "Further, encodings other than XML have been proposed". ze encodes configuration in its own YANG-modeled tree and in JSON, and emits no XML instance document, so the XML element order, the insert attributes and the XML prefix rules have no producer here | When a server sends XML-encoded data, it MUST use the canonical form defined in this section. |
| `9.3.2:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the sentence defines the decimal64 canonical form, and Section 9.1 binds the canonical form to the XML encoding only: "When a server sends XML-encoded data, it MUST use the canonical form defined in this section." (site 9.1:2, excluded for the same reason). ze encodes configuration in its own YANG-modeled tree and in JSON and emits no XML instance document. The decimal64 lexical form ze parses is Section 9.3.1, row RFC7950-9.3.1-1 | Leading and trailing zeros are prohibited, subject to the rule that there MUST be at least one digit before and after the decimal point. |
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
