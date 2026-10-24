# RFC 7950 - The YANG 1.1 Data Modeling Language

Partial. Every requirement this repository extracted from RFC 7950, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 39.6% | 36 of 91 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 1.1% | 1 of 91 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 91 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 95.5% | 149 of 156 tagged units, 0 escaped and 1 lapsed | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

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
| Partial proof; remaining gap | 3.3% | 3 of 91 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 50.5% | 46 of 91 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 39 | of 91 gated MUSTs judged | 10 weak, wrong or unimplemented, 5 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 91 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | bad | green at zero, RED above it: a tested clause cannot prove the whole requirement |
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
| Declared gaps | 46 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 156 |
| Tagged units | 156 |
| Recorded audit verdicts | 39 |
| Discrimination records | 150 |
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
- a missing mandatory leaf or choice refused at load, validate and commit, including under an absent non-presence container, in every top-level section present in the config
- min-elements and max-elements
- leaf and typedef defaults validated against their type at schema build, and a default under a choice applied only while its case is in effect
- the loader refuses a bad escape, an unprefixed external reference, a leaf or typedef without a type, an extension with a non-YANG substatement, a descending or non-numeric range, a negative or descending length, a decimal64 without fraction-digits, and an enum value that is duplicated, beyond int32 or missing after the maximum.


**What the ledger says remains**

Forty-nine MUST rows carry {gap} or {partial}, each naming its spec. Ze's loader adds no check of its own where goyang accepts a violating module (circular imports, duplicate typedefs and groupings, a shared import prefix, a typedef named after a built-in type, a shared child name across choice cases, an action or notification under a keyless list, an augment adding a duplicate, an overlapping range, an empty enumeration or union; a self-referencing grouping or identity and an augment targeting a leaf crash goyang instead of being refused): [`plan/release/fixes/spec-config-yang-loader-structural-checks.md`](https://github.com/ze-software/ze/blob/main/plan/release/fixes/spec-config-yang-loader-structural-checks.md). The validator evaluates no when or unique, and does not check that the nodes satisfying a mandatory choice come from exactly one case. The mandatory check runs over every top-level section present in the config, but a mandatory node whose only ancestors are non-presence containers is not required when its top-level section is absent from the config (owner-approved deviation of 2026-10-12, as libyang LYD_VALIDATE_PRESENT and FRR do), and by owner ruling of 2026-10-12 a bgp global router-id or local ASN that every session carries counts as present, a deviation from Section 7.6.5: [`plan/release/fixes/spec-config-yang-when-unique-choice.md`](https://github.com/ze-software/ze/blob/main/plan/release/fixes/spec-config-yang-when-unique-choice.md). A config list without a key is tolerated, leaf-list values are not deduplicated and leaf-list defaults are not applied: [`plan/release/fixes/spec-config-yang-list-key-leaf-list.md`](https://github.com/ze-software/ze/blob/main/plan/release/fixes/spec-config-yang-list-key-leaf-list.md). RPC input and output mandatory and defaults are read by no invocation path: [`plan/release/fixes/spec-config-yang-rpc-mandatory-defaults.md`](https://github.com/ze-software/ze/blob/main/plan/release/fixes/spec-config-yang-rpc-mandatory-defaults.md). No repository check reads module namespaces or revisions: [`plan/post-release/engineering/spec-config-yang-authoring-checks.md`](https://github.com/ze-software/ze/blob/main/plan/post-release/engineering/spec-config-yang-authoring-checks.md). The validator checks a bits value's names but Ze adds no check on a bits type's bit statements: [`plan/post-release/features/spec-config-yang-type-bits.md`](https://github.com/ze-software/ze/blob/main/plan/post-release/features/spec-config-yang-type-bits.md). goyang keeps one base statement per identityref type, so a type naming several bases is not resolved as Section 9.10.2 defines: [`plan/post-release/features/spec-config-yang-reference-types.md`](https://github.com/ze-software/ze/blob/main/plan/post-release/features/spec-config-yang-reference-types.md). Ze offers no feature or if-feature, no deviation, no yang-version declaration and no submodule: one plan/spec-config-yang-*.md each, for the owner to decline or schedule.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 36 | one part of the gated population |
| Annotated (including scoped evidence) | 55 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 3 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **91** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (36):** [`RFC7950-9.6-1`](#rfc7950-9.6-1), [`RFC7950-9.12-1`](#rfc7950-9.12-1), [`RFC7950-7.6.5-2`](#rfc7950-7.6.5-2), [`RFC7950-7.6.5-3`](#rfc7950-7.6.5-3), [`RFC7950-6.1.3-1`](#rfc7950-6.1.3-1), [`RFC7950-6.5-1`](#rfc7950-6.5-1), [`RFC7950-7.3.4-1`](#rfc7950-7.3.4-1), [`RFC7950-7.6.1-1`](#rfc7950-7.6.1-1), [`RFC7950-7.6.1-2`](#rfc7950-7.6.1-2), [`RFC7950-7.6.1-3`](#rfc7950-7.6.1-3), [`RFC7950-7.6.3-1`](#rfc7950-7.6.3-1), [`RFC7950-7.6.4-2`](#rfc7950-7.6.4-2), [`RFC7950-7.7.4-1`](#rfc7950-7.7.4-1), [`RFC7950-7.7.5-1`](#rfc7950-7.7.5-1), [`RFC7950-7.19-1`](#rfc7950-7.19-1), [`RFC7950-8.1-3`](#rfc7950-8.1-3), [`RFC7950-8.1-2`](#rfc7950-8.1-2), [`RFC7950-8.3-1`](#rfc7950-8.3-1), [`RFC7950-8.3.3-1`](#rfc7950-8.3.3-1), [`RFC7950-9.1-1`](#rfc7950-9.1-1), [`RFC7950-9.3.1-1`](#rfc7950-9.3.1-1), [`RFC7950-9.3.4-1`](#rfc7950-9.3.4-1), [`RFC7950-9.4.4-1`](#rfc7950-9.4.4-1), [`RFC7950-9.5.1-1`](#rfc7950-9.5.1-1), [`RFC7950-9.6.4-2`](#rfc7950-9.6.4-2), [`RFC7950-9.6.4.2-1`](#rfc7950-9.6.4.2-1), [`RFC7950-9.9-1`](#rfc7950-9.9-1), [`RFC7950-9.9.2-1`](#rfc7950-9.9.2-1), [`RFC7950-9.9.3-1`](#rfc7950-9.9.3-1), [`RFC7950-9.10.3-1`](#rfc7950-9.10.3-1), [`RFC7950-9.13-1`](#rfc7950-9.13-1), [`RFC7950-7.2.2-1`](#rfc7950-7.2.2-1), [`RFC7950-7.3.2-1`](#rfc7950-7.3.2-1), [`RFC7950-7.9.2-2`](#rfc7950-7.9.2-2), [`RFC7950-9.1-2`](#rfc7950-9.1-2), [`RFC7950-9.4.4-2`](#rfc7950-9.4.4-2)

**Annotated (including scoped evidence) (55):** [`RFC7950-7.6.5-1`](#rfc7950-7.6.5-1), [`RFC7950-7.5.3-1`](#rfc7950-7.5.3-1), [`RFC7950-9.2.4-1`](#rfc7950-9.2.4-1), [`RFC7950-9.4.5-1`](#rfc7950-9.4.5-1), [`RFC7950-x-1`](#rfc7950-x-1), [`RFC7950-x-2`](#rfc7950-x-2), [`RFC7950-5.1-1`](#rfc7950-5.1-1), [`RFC7950-5.3-1`](#rfc7950-5.3-1), [`RFC7950-5.5-1`](#rfc7950-5.5-1), [`RFC7950-5.6.5-1`](#rfc7950-5.6.5-1), [`RFC7950-6.2-1`](#rfc7950-6.2-1), [`RFC7950-6.2.1-1`](#rfc7950-6.2.1-1), [`RFC7950-6.3.1-1`](#rfc7950-6.3.1-1), [`RFC7950-6.4-1`](#rfc7950-6.4-1), [`RFC7950-6.4-2`](#rfc7950-6.4-2), [`RFC7950-7.1.4-1`](#rfc7950-7.1.4-1), [`RFC7950-7.3-1`](#rfc7950-7.3-1), [`RFC7950-7.6.1-4`](#rfc7950-7.6.1-4), [`RFC7950-7.7-1`](#rfc7950-7.7-1), [`RFC7950-7.7.2-1`](#rfc7950-7.7.2-1), [`RFC7950-7.8.2-1`](#rfc7950-7.8.2-1), [`RFC7950-7.8.3-1`](#rfc7950-7.8.3-1), [`RFC7950-7.9.2-1`](#rfc7950-7.9.2-1), [`RFC7950-7.9.3-1`](#rfc7950-7.9.3-1), [`RFC7950-7.9.4-1`](#rfc7950-7.9.4-1), [`RFC7950-7.12-1`](#rfc7950-7.12-1), [`RFC7950-7.14.2-1`](#rfc7950-7.14.2-1), [`RFC7950-7.14.3-1`](#rfc7950-7.14.3-1), [`RFC7950-7.15-1`](#rfc7950-7.15-1), [`RFC7950-7.16-1`](#rfc7950-7.16-1), [`RFC7950-7.17-1`](#rfc7950-7.17-1), [`RFC7950-7.18.2-1`](#rfc7950-7.18.2-1), [`RFC7950-7.20.1-1`](#rfc7950-7.20.1-1), [`RFC7950-7.20.2-1`](#rfc7950-7.20.2-1), [`RFC7950-7.20.3-1`](#rfc7950-7.20.3-1), [`RFC7950-7.20.3.2-1`](#rfc7950-7.20.3.2-1), [`RFC7950-7.21.2-1`](#rfc7950-7.21.2-1), [`RFC7950-7.21.5-1`](#rfc7950-7.21.5-1), [`RFC7950-8.1-1`](#rfc7950-8.1-1), [`RFC7950-8.1-4`](#rfc7950-8.1-4), [`RFC7950-8.3.1-2`](#rfc7950-8.3.1-2), [`RFC7950-8.3.2-1`](#rfc7950-8.3.2-1), [`RFC7950-8.3.3-2`](#rfc7950-8.3.3-2), [`RFC7950-9.2.4-2`](#rfc7950-9.2.4-2), [`RFC7950-9.6.4-1`](#rfc7950-9.6.4-1), [`RFC7950-9.7-1`](#rfc7950-9.7-1), [`RFC7950-9.7.4-1`](#rfc7950-9.7.4-1), [`RFC7950-9.7.4.2-1`](#rfc7950-9.7.4.2-1), [`RFC7950-9.10.2-1`](#rfc7950-9.10.2-1), [`RFC7950-9.12-2`](#rfc7950-9.12-2), [`RFC7950-11-1`](#rfc7950-11-1), [`RFC7950-11-2`](#rfc7950-11-2), [`RFC7950-11-3`](#rfc7950-11-3), [`RFC7950-7.21.5-2`](#rfc7950-7.21.5-2), [`RFC7950-12-1`](#rfc7950-12-1)

**Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) (3):** [`RFC7950-7.6.5-1`](#rfc7950-7.6.5-1), [`RFC7950-7.9.4-1`](#rfc7950-7.9.4-1), [`RFC7950-9.10.2-1`](#rfc7950-9.10.2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7950-9.6-1` | The enumeration built-in type represents values from a set of assigned names. (§9.6) | MUST | 9.6 | **positive:** `unit/verify` [`TestISISAuthAlgorithmEnumAcceptsAll`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_isis_auth_algorithm_enum_test.go#L64). **positive:** `unit/verify` [`TestRadiusAuthMethodEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_radius_auth_method_enum_test.go#L53). **positive:** `unit/verify` [`TestValidateTree_AddPathDirectionEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L619). **positive:** `unit/verify` [`TestValidateTree_FamilyModeEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L553). **negative:** `unit/verify` [`TestISISAuthAlgorithmEnumAcceptsAll`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_isis_auth_algorithm_enum_test.go#L81). **negative:** `unit/verify` [`TestRadiusAuthMethodEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_radius_auth_method_enum_test.go#L64). **negative:** `unit/verify` [`TestValidateTree_AddPathDirectionEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L620). **negative:** `unit/verify` [`TestValidateTree_FamilyModeEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L554) |
| `RFC7950-9.12-1` | The union built-in type represents a value that corresponds to one of its member types. (§9.12) | MUST | 9.12 | **positive:** `unit/verify` [`TestRFC7950UnionAcceptsEveryMemberType`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_union_test.go#L32). **positive:** `unit/verify` [`TestValidateTree_ValidConfig`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L47). **negative:** `unit/verify` [`TestValidateTree_UnionViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L367) |
| `RFC7950-7.6.5-1` | If "mandatory" is "true", the behavior of the constraint depends on the type of the leaf's closest ancestor node in the schema tree that is not a non-presence container (see Section 7.5.1): o If no such ancestor exists in the schema tree, the leaf MUST exist. (§7.6.5) | MUST | 7.6.5 | **positive:** `unit/verify` [`TestRFC7950MandatoryRefusesBGPConfig`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_mandatory_bgp_test.go#L31). **positive:** `unit/verify` [`TestRFC7950MandatoryWithNoQualifyingAncestor`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_mandatory_test.go#L70). **positive:** `unit/verify` [`TestValidator_MandatoryField`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L943). **negative:** `unit/verify` [`TestRFC7950MandatoryRefusesBGPConfig`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_mandatory_bgp_test.go#L30). **negative:** `unit/verify` [`TestRFC7950MandatoryWithNoQualifyingAncestor`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_mandatory_test.go#L69). **negative:** `unit/verify` [`TestValidateTree_MandatoryMissing`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L339). **negative:** `unit/verify` [`TestValidator_MandatoryField`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L944). **{partial}:** tested "the leaf MUST exist"; gap "If no such ancestor exists in the schema tree"; internal/component/config/validate_sections.go::ValidateCustomSections is the boundary: Tested covers a mandatory leaf whose ancestors are all non-presence containers, required whenever its top-level section is present in the config (bgp/router-id, bgp/session/asn/local, once the bgp section holds a peer or a group); Gap is the same leaf when the whole top-level section is absent, which Ze does not require, because its top-level sections are non-presence containers that stand for an enabled feature and requiring their leaves would refuse every config that omits bgp; owner-approved deviation, OWNER RULING 6 of 2026-10-12 (Thomas, "Like FRR/libyang (Recommended)"): mandatory leaves are checked only in sections present in the config, as libyang's LYD_VALIDATE_PRESENT ("Validate only modules whose data actually exist") does when FRR sets it in nb_candidate_validate_yang (lib/northbound.c) and yang_parse_data (lib/yang.c); Gap also holds bgp/router-id and bgp/session/asn/local absent at the bgp level while every BGP session carries the value itself or from its group, which bgpMandatoryFailures counts as present through EverySessionCarries: a second owner-approved deviation, ruling of 2026-10-12 (Thomas, "yes enforce", spec AC-2); that ruling binds only once a session exists: a bgp section with no peer and no group is accepted and does nothing, so EverySessionCarries counts both leaves as present there (Thomas, 2026-10-16, spec AC-2 RULING 7); a deviation is never counted conformant; plan/release/fixes/spec-config-yang-when-unique-choice.md. **Scoped evidence:** Tests and tag-claim records apply only to Tested; zero whole-requirement credit. partial (the tests enforce only the declared tested scope; the whole requirement remains unmet), stale-requirement. Independently rejudged 2026-10-22; no execution or stamping. RFC 7950 Section 7.6.5: "If "mandatory" is "true", the behavior of the constraint depends on the type of the leaf's closest ancestor node in the schema tree that is not a non-presence container (see Section 7.5.1): o If no such ancestor exists in the schema tree, the leaf MUST exist." Tested "the leaf MUST exist"; Gap "If no such ancestor exists in the schema tree"; Producer internal/component/config/validate_sections.go::ValidateCustomSections. Read all four tagged units and both polarities where present: config/rfc7950_mandatory_bgp_test.go::TestRFC7950MandatoryRefusesBGPConfig, config/rfc7950_validator_yang_test.go::TestValidateTree_MandatoryMissing and TestValidator_MandatoryField, config/yang/rfc7950_mandatory_test.go::TestRFC7950MandatoryWithNoQualifyingAncestor (under internal/component). The direct validator requires exact missing path top/mid/need under an absent non-presence ancestor, the corrective set command, and no missing errors when supplied. Config carriers require LoadConfig refusal of missing local ASN or router-id with the expected path and validation category, then accept the corresponding complete config with an actual peer. The patch prepends validateOriginalTree to ValidateCustomSections. Its registered bgp-filter-policy validator reads original bgp/group/peer import/export directions and requires an explicit final on a present direction; these mandatory-leaf fixtures contain no policy direction, so that validator returns normally and does not confound the missing-leaf assertions. The mandatory producer still calls walkTree/requireMandatoryChildren/requireMandatoryUnderAbsent; bgpMandatoryFailures retains only missing errors and EverySessionCarries implements the declared deviations. The whole top-level absent section remains unwalked, inherited session values still excuse global router-id/local ASN, and a BGP section without session sources still excuses those two leaves. Preserve every declared Tested/Gap restriction and the owner-approved deviations; none becomes RFC conformance and this row retains zero whole-requirement credit. Supplied discrimination metadata is not a new runtime observation. |
| `RFC7950-7.6.5-2` | Otherwise, if this ancestor is a case node, the leaf MUST exist if any node from the case exists in the data tree. (§7.6.5) | MUST | 7.6.5 | **positive:** `unit/verify` [`TestRFC7950MandatoryNotRequiredForAbsentCase`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_mandatory_case_test.go#L111). **negative:** `unit/verify` [`TestRFC7950MandatoryRequiredWhenCaseExists`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_mandatory_case_test.go#L69). **negative:** `functional/verify` [`iface-tunnel-invalid-gre-no-remote.ci`](https://github.com/ze-software/ze/blob/main/test/parse/iface-tunnel-invalid-gre-no-remote.ci#L16) |
| `RFC7950-7.6.5-3` | Otherwise, the leaf MUST exist if the ancestor node exists in the data tree. (§7.6.5) | MUST | 7.6.5 | **positive:** `unit/verify` [`TestRFC7950MandatoryOutsideValidatedSections`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_mandatory_sections_test.go#L34). **positive:** `unit/verify` [`TestRFC7950MandatoryRequiredWhenAncestorExists`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_mandatory_test.go#L105). **negative:** `unit/verify` [`TestRFC7950MandatoryOutsideValidatedSections`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_mandatory_sections_test.go#L33). **negative:** `unit/verify` [`TestRFC7950MandatoryRequiredWhenAncestorExists`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_mandatory_test.go#L104) |
| `RFC7950-7.5.3-1` | All such constraints MUST evaluate to "true" for the data to be valid. (§7.5.3) | MUST | 7.5.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze uses zero YANG must statements (a grep of the .yang models finds must only inside a description); it parses via goyang which evaluates no XPath, and enforces cross-field constraints with Go ze:validate functions (internal/component/config/yang/validator.go:756 applyCustomValidators) instead, so there is no must-XPath code path |
| `RFC7950-9.2.4-1` | If a range restriction is applied to a type that is already range-restricted, the new restriction MUST be equally limiting or more limiting, i.e., raising the lower bounds, reducing the upper bounds, removing explicit values or ranges, or splitting ranges into multiple ranges with intermediate gaps. (§9.2.4) | MUST | 9.2.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this is a schema-authoring-time narrowing constraint resolved by goyang during module processing (internal/component/config/yang/loader.go); ze authors valid narrowings and has no runtime enforcement surface for it |
| `RFC7950-9.4.5-1` | If the type has multiple "pattern" statements, the expressions are ANDed together, i.e., all such expressions have to match. (§9.4.5) | MUST | 9.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no ze type carries two or more pattern statements (a scan of the .yang models finds none); the validator loop (internal/component/config/yang/validator.go:268) AND-combines patterns if present, but the multi-pattern construct is unused |
| `RFC7950-x-1` | The "yang-version" statement specifies which version of the YANG language was used in developing the module. The statement's argument is a string. It MUST contain the value "1.1" for YANG modules defined based on this specification. (§7.1.2) | MUST | 7.1.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's YANG models declare no yang-version statement and default to YANG 1.0 in goyang; they use no YANG-1.1-only construct that would require the 1.1 declaration, so the version-declaration obligation has no applicable module |
| `RFC7950-x-2` | An "enumeration" type may have new enums added, provided the old enums's values do not change. (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's modules carry no consumed revision statements and there is no cross-revision position-stability tooling; ze pins the values that matter with explicit value statements (e.g. role.yang, ze-types.yang afi/safi) but enforces no automated cross-revision check |
| `RFC7950-5.1-1` | Within a server, all module names MUST be unique. A module uses the "include" statement to list all its submodules. A module, or submodule belonging to that module, can reference definitions in the module and all submodules included by the module. A module or submodule uses the "import" statement to reference external modules. Statements in the module or submodule can reference definitions in the external module using the prefix specified in the "import" statement. For backward compatibility with YANG version 1, a submodule MAY use the "include" statement to reference other submodules within its module, but this is not necessary in YANG version 1.1. A submodule can reference any definition in the module it belongs to and in all submodules included by the module. A submodule MUST NOT include different revisions of other submodules than the revisions that its module includes. A module or submodule MUST NOT include submodules from other modules, and a submodule MUST NOT import its own module. The "import" and "include" statements are used to make definitions available from other modules: o For a module or submodule to reference definitions in an external module, the external module MUST be imported. o A module MUST include all its submodules. o A module, or submodule belonging to that module, MAY reference definitions in the module and all submodules included by the module. There MUST NOT be any circular chains of imports. For example, if module "a" imports module "b", "b" cannot import "a". When a definition in an external module is referenced, a locally defined prefix MUST be used, followed by a colon (":") and then the external identifier. (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| `RFC7950-5.3-1` | Namespace URIs MUST be chosen so they cannot collide with standard or other enterprise namespaces -- for example, by using the enterprise or organization name in the namespace. (Section 5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no repository check reads the namespace or the revision history of the embedded modules; plan/post-release/engineering/spec-config-yang-authoring-checks.md |
| `RFC7950-5.5-1` | Scoped definitions MUST NOT shadow definitions at a higher scope. (§5.5) | MUST | 5.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| `RFC7950-5.6.5-1` | A server MUST NOT implement more than one revision of a module. If a server implements a module A that imports a module B, and A uses any node from B in an "augment" or "path" statement that the server supports, then the server MUST implement a revision of module B that has these nodes defined. (§5.6.5) | MUST | 5.6.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| `RFC7950-6.1.3-1` | The backslash MUST NOT be followed by any other character. (§6.1.3) | MUST | 6.1.3 | **positive:** `unit/verify` [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L116). **negative:** `unit/verify` [`TestRFC7950ModuleLoadRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L66) |
| `RFC7950-6.2-1` | Implementations MUST support identifiers up to 64 characters in length and MAY support longer identifiers. (§6.2) | MUST | 6.2 | **positive:** `unit/verify` [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L118). **negative:** no negative test. **{single-polarity}:** the RFC binds an implementation to accept an identifier of up to 64 characters and MAY accept longer, so no identifier length exists that Ze must refuse |
| `RFC7950-6.2.1-1` | All identifiers defined in a namespace MUST be unique. o All module and submodule names share the same global module identifier namespace. o All extension names defined in a module and its submodules share the same extension identifier namespace. o All feature names defined in a module and its submodules share the same feature identifier namespace. o All identity names defined in a module and its submodules share the same identity identifier namespace. o All derived type names defined within a parent node or at the top level of the module or its submodules share the same type identifier namespace. This namespace is scoped to all descendant nodes of the parent node or module. This means that any descendant node may use that typedef, and it MUST NOT define a typedef with the same name. o All grouping names defined within a parent node or at the top level of the module or its submodules share the same grouping identifier namespace. This namespace is scoped to all descendant nodes of the parent node or module. This means that any descendant node may use that grouping, and it MUST NOT define a grouping with the same name. (§6.2.1) | MUST | 6.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| `RFC7950-6.3.1-1` | When an imported extension is used, the extension's keyword MUST be qualified using the prefix with which the extension's module was imported. If an extension is used in the module where it is defined, the extension's keyword MUST be qualified with the prefix of this module. The processing of extensions depends on whether support for those extensions is claimed for a given YANG parser or the tool set in which it is embedded. An unsupported extension appearing in a YANG module as an unknown-statement (see Section 14) MAY be ignored in its entirety. Any supported extension MUST be processed in accordance with the specification governing that extension. (§6.3.1) | MUST | 6.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| `RFC7950-6.4-1` | An implementation is not required to implement an XPath interpreter but MUST ensure that the requirements encoded in the data model are enforced. (Section 6.4) | MUST | 6.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** walkTree evaluates no when, unique or choice; plan/release/fixes/spec-config-yang-when-unique-choice.md |
| `RFC7950-6.4-2` | The XPath expressions MUST be syntactically correct, and all prefixes used MUST be present in the XPath context (see Section 6.4.1). (§6.4) | MUST | 6.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** walkTree evaluates no when, unique or choice; plan/release/fixes/spec-config-yang-when-unique-choice.md |
| `RFC7950-6.5-1` | References to identifiers defined in external modules MUST be qualified with appropriate prefixes, and references to identifiers defined in the current module and its submodules MAY use a prefix. (§6.5) | MUST | 6.5 | **positive:** `unit/verify` [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L123). **negative:** `unit/verify` [`TestRFC7950ModuleLoadRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L69) |
| `RFC7950-7.1.4-1` | If there is a conflict, i.e., two different modules that both have defined the same prefix are imported, at least one of them MUST be imported with a different prefix. All prefixes, including the prefix for the module itself, MUST be unique within the module or submodule. (§7.1.4) | MUST | 7.1.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| `RFC7950-7.3-1` | The "typedef" statement's argument is an identifier that is the name of the type to be defined and MUST be followed by a block of substatements that holds detailed typedef information. The name of the type MUST NOT be one of the YANG built-in types. If the typedef is defined at the top level of a YANG module or submodule, the name of the type to be defined MUST be unique within the module. (§7.3) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| `RFC7950-7.3.4-1` | The value of the "default" statement MUST be valid according to the type specified in the "type" statement. If the base type has a default value and the new derived type does not specify a new default value, the base type's default value is also the default value of the new derived type. If the type's default value is not valid according to the new restrictions specified in a derived type or leaf definition, the derived type or leaf definition MUST specify a new default value compatible with the restrictions. (§7.3.4) | MUST | 7.3.4 | **positive:** `unit/verify` [`TestRFC7950DefaultValidForType`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_test.go#L33). **positive:** `unit/verify` [`TestRFC7950DerivedTypeInheritsOrReplacesTheDefault`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_clauses_test.go#L41). **negative:** `unit/verify` [`TestRFC7950DefaultInvalidForType`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_test.go#L48). **negative:** `unit/verify` [`TestRFC7950DerivedTypeMustReplaceAnInvalidatedDefault`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_clauses_test.go#L60) |
| `RFC7950-7.6.1-1` | The usage of the default value depends on the leaf's closest ancestor node in the schema tree that is not a non-presence container (see Section 7.5.1): o If no such ancestor exists in the schema tree, the default value MUST be used. (§7.6.1) | MUST | 7.6.1 | **positive:** `unit/verify` [`TestApplyDefaults_NonPresenceContainer`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_schema_defaults_test.go#L38). **positive:** `unit/verify` [`TestRFC7950LeafDefaultUsedWhenAbsent`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_leaf_default_test.go#L14). **negative:** `unit/verify` [`TestRFC7950LeafDefaultNotUsedWhenSet`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_leaf_default_test.go#L42) |
| `RFC7950-7.6.1-2` | Otherwise, if this ancestor is a case node, the default value MUST be used if any node from the case exists in the data tree or the case node is the choice's default case, and if no nodes from any other case exist in the data tree. (§7.6.1) | MUST | 7.6.1 | **positive:** `unit/verify` [`TestRFC7950CaseDefaultsFollowTheCaseInEffect`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_case_defaults_test.go#L62). **negative:** `unit/verify` [`TestRFC7950CaseDefaultsIgnoredOutsideTheCaseInEffect`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_case_defaults_test.go#L104) |
| `RFC7950-7.6.1-3` | Otherwise, the default value MUST be used if the ancestor node exists in the data tree. (§7.6.1) | MUST | 7.6.1 | **positive:** `unit/verify` [`TestRFC7950LeafDefaultFollowsItsAncestor`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_leaf_default_test.go#L69). **negative:** `unit/verify` [`TestRFC7950LeafDefaultFollowsItsAncestor`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_leaf_default_test.go#L70) |
| `RFC7950-7.6.1-4` | In these cases, the default value is said to be in use. Note that if the leaf or any of its ancestors has a "when" condition or "if-feature" expression that evaluates to "false", then the default value is not in use. When the default value is in use, the server MUST operationally behave as if the leaf was present in the data tree with the default value as its value. (§7.6.1) | MUST | 7.6.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no when is evaluated and Ze has no if-feature, so a default whose "when" is false stays in use; plan/release/fixes/spec-config-yang-when-unique-choice.md |
| `RFC7950-7.6.3-1` | The "type" statement, which MUST be present, takes as an argument the name of an existing built-in or derived type. (Section 7.6.3) | MUST | 7.6.3 | **positive:** `unit/verify` [`TestRFC7950LeafTypeMustNameAnExistingType`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L39). **positive:** `unit/verify` [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L130). **negative:** `unit/verify` [`TestRFC7950LeafTypeMustNameAnExistingType`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L38). **negative:** `unit/verify` [`TestRFC7950ModuleLoadRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L75) |
| `RFC7950-7.6.4-2` | The value of the "default" statement MUST be valid according to the type specified in the leaf's "type" statement. The "default" statement MUST NOT be present on nodes where "mandatory" is "true". The definition of the default value MUST NOT be marked with an "if-feature" statement. (§7.6.4) | MUST | 7.6.4 | **positive:** `unit/verify` [`TestRFC7950DefaultNotAnIfFeatureEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_clauses_test.go#L76). **positive:** `unit/verify` [`TestRFC7950DefaultNotAnIfFeatureMemberBitOrTypedef`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_clauses_test.go#L94). **positive:** `unit/verify` [`TestRFC7950DefaultValidForType`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_test.go#L34). **negative:** `unit/verify` [`TestRFC7950DefaultInvalidForType`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_test.go#L49). **negative:** `unit/verify` [`TestRFC7950DefaultNotAnIfFeatureEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_clauses_test.go#L75). **negative:** `unit/verify` [`TestRFC7950DefaultNotAnIfFeatureMemberBitOrTypedef`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_clauses_test.go#L93) |
| `RFC7950-7.7-1` | In configuration data, the values in a leaf-list MUST be unique. The definitions of the default values MUST NOT be marked with an "if-feature" statement. Conceptually, the values in the data tree MUST be in the canonical form (see Section 9.1). (§7.7) | MUST | 7.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** yangToList tolerates a keyless config list and leaf-lists are neither deduplicated nor defaulted; plan/release/fixes/spec-config-yang-list-key-leaf-list.md |
| `RFC7950-7.7.2-1` | The usage of the default values depends on the leaf-list's closest ancestor node in the schema tree that is not a non-presence container (see Section 7.5.1): o If no such ancestor exists in the schema tree, the default values MUST be used. o Otherwise, if this ancestor is a case node, the default values MUST be used if any node from the case exists in the data tree or the case node is the choice's default case, and if no nodes from any other case exist in the data tree. o Otherwise, the default values MUST be used if the ancestor node exists in the data tree. In these cases, the default values are said to be in use. Note that if the leaf-list or any of its ancestors has a "when" condition or "if-feature" expression that evaluates to "false", then the default values are not in use. When the default values are in use, the server MUST operationally behave as if the leaf-list was present in the data tree with the default values as its values. (§7.7.2) | MUST | 7.7.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** yangToList tolerates a keyless config list and leaf-lists are neither deduplicated nor defaulted; plan/release/fixes/spec-config-yang-list-key-leaf-list.md |
| `RFC7950-7.7.4-1` | The value of the "default" statement MUST be valid according to the type specified in the leaf-list's "type" statement. The "default" statement MUST NOT be present on nodes where "min-elements" has a value greater than or equal to one. (§7.7.4) | MUST | 7.7.4 | **positive:** `unit/verify` [`TestRFC7950DefaultValidForType`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_test.go#L35). **negative:** `unit/verify` [`TestRFC7950DefaultInvalidForType`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_test.go#L50) |
| `RFC7950-7.7.5-1` | A valid leaf-list or list MUST have at least min-elements entries. (Section 7.7.5) | MUST | 7.7.5 | **positive:** `unit/verify` [`TestRFC7950MinElementsMet`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_test.go#L48). **negative:** `unit/verify` [`TestRFC7950MinElementsViolated`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_test.go#L67) |
| `RFC7950-7.8.2-1` | The "key" statement, which MUST be present if the list represents configuration and MAY be present otherwise, takes as an argument a string that specifies a space-separated list of one or more leaf identifiers of this list. A leaf identifier MUST NOT appear more than once in the key. Each such leaf identifier MUST refer to a child leaf of the list. The leafs can be defined directly in substatements to the list or in groupings used in the list. The combined values of all the leafs specified in the key are used to uniquely identify a list entry. All key leafs MUST be given values when a list entry is created. Thus, any default values in the key leafs or their types are ignored. Any "mandatory" statements in the key leafs are ignored. A leaf that is part of the key can be of any built-in or derived type. All key leafs in a list MUST have the same value for their "config" as the list itself. (§7.8.2) | MUST | 7.8.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** yangToList tolerates a keyless config list and leaf-lists are neither deduplicated nor defaulted; plan/release/fixes/spec-config-yang-list-key-leaf-list.md |
| `RFC7950-7.8.3-1` | The "unique" statement is used to put constraints on valid list entries. It takes as an argument a string that contains a space- separated list of schema node identifiers, which MUST be given in the descendant form (see the rule "descendant-schema-nodeid" in Section 14). Each such schema node identifier MUST refer to a leaf. If one of the referenced leafs represents configuration data, then all of the referenced leafs MUST represent configuration data. The "unique" constraint specifies that the combined values of all the leaf instances specified in the argument string, including leafs with default values, MUST be unique within all list entry instances in which all referenced leafs exist or have default values. (§7.8.3) | MUST | 7.8.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** walkTree evaluates no when, unique or choice; plan/release/fixes/spec-config-yang-when-unique-choice.md |
| `RFC7950-7.9.2-1` | The identifiers of all these child nodes MUST be unique within all cases in a choice. (§7.9.2) | MUST | 7.9.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| `RFC7950-7.9.3-1` | The "default" statement MUST NOT be present on choices where "mandatory" is "true". The default case is only important when considering the "default" statements of nodes under the cases (i.e., default values of leafs and leaf-lists, and default cases of nested choices). The default values and nested default cases under the default case are used if none of the nodes under any of the cases are present. There MUST NOT be any mandatory nodes (Section 3) directly under the default case. (§7.9.3) | MUST | 7.9.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| `RFC7950-7.9.4-1` | If "mandatory" is "true", at least one node from exactly one of the choice's case branches MUST exist. (Section 7.9.4) | MUST | 7.9.4 | **positive:** `unit/verify` [`TestMandatoryChoiceSatisfiedByACaseNode`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_mandatory_test.go#L151). **negative:** `unit/verify` [`TestMandatoryChoiceSatisfiedByACaseNode`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_mandatory_test.go#L150). **{partial}:** tested "at least one node from"; gap "exactly one of the choice's case branches"; internal/component/config/yang/validator_mandatory.go::mandatoryPresent is the boundary: Tested covers a mandatory choice reported missing when no data node of any of its cases exists, and satisfied by a node of one case, nested choices included; Gap is data holding nodes from two cases of one choice, which mandatoryPresent accepts at the first case node it finds, and no other check compares the cases; plan/release/fixes/spec-config-yang-when-unique-choice.md. **Scoped evidence:** Tests and tag-claim records apply only to Tested; zero whole-requirement credit. partial (the tests enforce only the declared tested scope; the whole requirement remains unmet), stale-requirement. Judged 2026-10-12 (independent auditor). Section 7.9.4: "If \\"mandatory\\" is \\"true\\", at least one node from exactly one of the choice's case branches MUST exist." Tested "at least one node from"; Gap "exactly one of the choice's case branches"; Producer internal/component/config/yang/validator_mandatory.go::mandatoryPresent. Tested holds: TestMandatoryChoiceSatisfiedByACaseNode reports wrap/kind missing for a container with no node of any case (exact path), and accepts a node of one case and a node of a nested choice's case, so a lookup by the choice name alone, or one that ignores nested choices, goes red; records on mandatoryPresent (positive) and missingMandatory (negative). Real entry: walkTree's mandatory loop reaches it from ValidateCustomSections, so the static route choice action and the iface tunnel encapsulation choice kind are refused when empty. Gap holds: data with nodes from two cases is accepted at the first case node found and no other check compares the cases, so the exactly-one half is unmet; the conditional enforcement under a case or list ancestor in the following paragraph is a separate sentence. Whole requirement earns no credit; spec-config-yang-when-unique-choice. |
| `RFC7950-7.12-1` | A grouping MUST NOT reference itself, neither directly nor indirectly through a chain of other groupings. If the grouping is defined at the top level of a YANG module or submodule, the grouping's identifier MUST be unique within the module. (§7.12) | MUST | 7.12 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| `RFC7950-7.14.2-1` | If a leaf in the input tree has a "mandatory" statement with the value "true", the leaf MUST be present in an RPC invocation. If a leaf in the input tree has a default value, the server MUST use this value in the same cases as those described in Section 7.6.1. In these cases, the server MUST operationally behave as if the leaf was present in the RPC invocation with the default value as its value. If a leaf-list in the input tree has one or more default values, the server MUST use these values in the same cases as those described in Section 7.7.2. In these cases, the server MUST operationally behave as if the leaf-list was present in the RPC invocation with the default values as its values. Since the input tree is not part of any datastore, all "config" statements for nodes in the input tree are ignored. If any node has a "when" statement that would evaluate to "false", then this node MUST NOT be present in the input tree. (§7.14.2) | MUST | 7.14.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no invocation or reply path reads mandatory or default on rpc input and output; plan/release/fixes/spec-config-yang-rpc-mandatory-defaults.md |
| `RFC7950-7.14.3-1` | If a leaf in the output tree has a "mandatory" statement with the value "true", the leaf MUST be present in an RPC reply. If a leaf in the output tree has a default value, the client MUST use this value in the same cases as those described in Section 7.6.1. In these cases, the client MUST operationally behave as if the leaf was present in the RPC reply with the default value as its value. If a leaf-list in the output tree has one or more default values, the client MUST use these values in the same cases as those described in Section 7.7.2. In these cases, the client MUST operationally behave as if the leaf-list was present in the RPC reply with the default values as its values. Since the output tree is not part of any datastore, all "config" statements for nodes in the output tree are ignored. If any node has a "when" statement that would evaluate to "false", then this node MUST NOT be present in the output tree. (§7.14.3) | MUST | 7.14.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no invocation or reply path reads mandatory or default on rpc input and output; plan/release/fixes/spec-config-yang-rpc-mandatory-defaults.md |
| `RFC7950-7.15-1` | An action MUST NOT be defined within an rpc, another action, or a notification, i.e., an action node MUST NOT have an rpc, action, or a notification node as one of its ancestors in the schema tree. For example, this means that it is an error if a grouping that contains an action somewhere in its node hierarchy is used in a notification definition. An action MUST NOT have any ancestor node that is a list node without a "key" statement. (§7.15) | MUST | 7.15 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| `RFC7950-7.16-1` | A notification MUST NOT be defined within an rpc, action, or another notification, i.e., a notification node MUST NOT have an rpc, action, or a notification node as one of its ancestors in the schema tree. For example, this means that it is an error if a grouping that contains a notification somewhere in its node hierarchy is used in an rpc definition. A notification MUST NOT have any ancestor node that is a list node without a "key" statement. Since a notification cannot be defined in a "case" statement, it is an error if a grouping that contains a notification at the top of its node hierarchy is used in a case definition. If a leaf in the notification tree has a "mandatory" statement with the value "true", the leaf MUST be present in a notification instance. If a leaf in the notification tree has a default value, the client MUST use this value in the same cases as those described in Section 7.6.1. In these cases, the client MUST operationally behave as if the leaf was present in the notification instance with the default value as its value. If a leaf-list in the notification tree has one or more default values, the client MUST use these values in the same cases as those described in Section 7.7.2. In these cases, the client MUST operationally behave as if the leaf-list was present in the notification instance with the default values as its values. (§7.16) | MUST | 7.16 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| `RFC7950-7.17-1` | The target node MUST be either a container, list, choice, case, input, output, or notification node. It is augmented with the nodes defined in the substatements that follow the "augment" statement. The argument string is a schema node identifier (see Section 6.5). If the "augment" statement is on the top level in a module or submodule, the absolute form (defined by the rule "absolute-schema-nodeid" in Section 14) of a schema node identifier MUST be used. If the "augment" statement is a substatement to the "uses" statement, the descendant form (defined by the rule "descendant-schema-nodeid" in Section 14) MUST be used. If the target node is a container, list, case, input, output, or notification node, the "container", "leaf", "list", "leaf-list", "uses", and "choice" statements can be used within the "augment" statement. If the target node is a container or list node, the "action" and "notification" statements can be used within the "augment" statement. If the target node is a choice node, the "case" statement or a shorthand "case" statement (see Section 7.9.2) can be used within the "augment" statement. The "augment" statement MUST NOT add multiple nodes with the same name from the same module to the target node. If the augmentation adds mandatory nodes (see Section 3) that represent configuration to a target node in another module, the augmentation MUST be made conditional with a "when" statement. (§7.17) | MUST | 7.17 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| `RFC7950-7.18.2-1` | If a prefix is present on the base name, it refers to an identity defined in the module that was imported with that prefix, or the local module if the prefix matches the local module's prefix. Otherwise, an identity with the matching name MUST be defined in the current module or an included submodule. An identity MUST NOT reference itself, neither directly nor indirectly through a chain of other identities. (§7.18.2) | MUST | 7.18.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| `RFC7950-7.19-1` | Syntactically, the substatements MUST be YANG statements, including extensions defined using "extension" statements. YANG statements in extensions MUST follow the syntactical rules in Section 14. (§7.19) | MUST | 7.19 | **positive:** `unit/verify` [`TestRFC7950ExtensionSubstatementCardinality`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_extension_grammar_test.go#L44). **positive:** `unit/verify` [`TestRFC7950ExtensionSubstatementContextForms`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_extension_grammar_test.go#L88). **positive:** `unit/verify` [`TestRFC7950ExtensionSubstatementRequiredStatementsAndBlocks`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_extension_grammar_test.go#L63). **positive:** `unit/verify` [`TestRFC7950ExtensionSubstatementURIAndPathArguments`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_extension_grammar_test.go#L22). **positive:** `unit/verify` [`TestRFC7950ExtensionUsageSubstatementsAreYANGStatements`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_structural_test.go#L36). **positive:** `unit/verify` [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L138). **negative:** `unit/verify` [`TestRFC7950ExtensionSubstatementCardinality`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_extension_grammar_test.go#L43). **negative:** `unit/verify` [`TestRFC7950ExtensionSubstatementContextForms`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_extension_grammar_test.go#L87). **negative:** `unit/verify` [`TestRFC7950ExtensionSubstatementRequiredStatementsAndBlocks`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_extension_grammar_test.go#L62). **negative:** `unit/verify` [`TestRFC7950ExtensionSubstatementURIAndPathArguments`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_extension_grammar_test.go#L21). **negative:** `unit/verify` [`TestRFC7950ExtensionUsageSubstatementsAreYANGStatements`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_structural_test.go#L35). **negative:** `unit/verify` [`TestRFC7950ModuleLoadRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L82) |
| `RFC7950-7.20.1-1` | A feature MUST NOT reference itself, neither directly nor indirectly through a chain of other features. In order for a server to support a feature that is dependent on any other features (i.e., the feature has one or more "if-feature" substatements), the server MUST also support all the dependent features. (§7.20.1) | MUST | 7.20.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze evaluates no if-feature and declares no feature; plan/spec-config-yang-if-feature.md |
| `RFC7950-7.20.2-1` | If a prefix is present on a feature name in the boolean expression, the prefixed name refers to a feature defined in the module that was imported with that prefix, or the local module if the prefix matches the local module's prefix. Otherwise, a feature with the matching name MUST be defined in the current module or an included submodule. A leaf that is a list key MUST NOT have any "if-feature" statements. (§7.20.2) | MUST | 7.20.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze evaluates no if-feature and declares no feature; plan/spec-config-yang-if-feature.md |
| `RFC7950-7.20.3-1` | This means that deviations MUST never be part of a published standard, since they are the mechanism for learning how implementations vary from the standards. Server deviations are strongly discouraged and MUST only be used as a last resort. Telling the application how a server fails to follow a standard is no substitute for implementing the standard correctly. A server that deviates from a module is not fully compliant with the module. However, in some cases, a particular device may not have the hardware or software ability to support parts of a standard module. When this occurs, the server makes a choice to either treat attempts to configure unsupported parts of the module as an error that is reported back to the unsuspecting application or ignore those incoming requests. Neither choice is acceptable. Instead, YANG allows servers to document portions of a base module that are not supported, or that are supported but with different syntax, by using the "deviation" statement. After applying all deviations announced by a server, in any order, the resulting data model MUST still be valid. (§7.20.3) | MUST | 7.20.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze reads no deviation; plan/spec-config-yang-deviation.md |
| `RFC7950-7.20.3.2-1` | The argument "add" adds properties to the target node. The properties to add are identified by substatements to the "deviate" statement. If a property can only appear once, the property MUST NOT exist in the target node. The argument "replace" replaces properties of the target node. The properties to replace are identified by substatements to the "deviate" statement. The properties to replace MUST exist in the target node. The argument "delete" deletes properties from the target node. The properties to delete are identified by substatements to the "delete" statement. The substatement's keyword MUST match a corresponding keyword in the target node, and the argument's string MUST be equal to the corresponding keyword's argument string in the target node. (§7.20.3.2) | MUST | 7.20.3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze reads no deviation; plan/spec-config-yang-deviation.md |
| `RFC7950-7.21.2-1` | If a definition is "current", it MUST NOT reference a "deprecated" or "obsolete" definition within the same module. If a definition is "deprecated", it MUST NOT reference an "obsolete" definition within the same module. (§7.21.2) | MUST | 7.21.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| `RFC7950-7.21.5-1` | A leaf that is a list key MUST NOT have a "when" statement. If a key leaf is defined in a grouping that is used in a list, the "uses" statement MUST NOT have a "when" statement. (§7.21.5) | MUST | 7.21.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** walkTree evaluates no when, unique or choice; plan/release/fixes/spec-config-yang-when-unique-choice.md |
| `RFC7950-8.1-1` | These constraints are enforced in different ways, depending on what type of data the statement defines. o If the constraint is defined on configuration data, it MUST be true in a valid configuration data tree. o If the constraint is defined on state data, it MUST be true in a valid state data tree. o If the constraint is defined on notification content, it MUST be true in any notification data tree. o If the constraint is defined on RPC or action input parameters, it MUST be true in an invocation of the RPC or action operation. o If the constraint is defined on RPC or action output parameters, it MUST be true in the RPC or action reply. (§8.1) | MUST | 8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** walkTree evaluates no when, unique or choice; plan/release/fixes/spec-config-yang-when-unique-choice.md |
| `RFC7950-8.1-3` | All leaf data values MUST match the type constraints for the leaf, including those defined in the type's "range", "length", and "pattern" properties. (§8.1) | MUST | 8.1 | **positive:** `unit/verify` [`TestValidator_HoldTimeRange`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L900). **positive:** `unit/verify` [`TestValidator_ValidatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L851). **negative:** `unit/verify` [`TestValidateTree_LengthViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L418). **negative:** `unit/verify` [`TestValidateTree_PatternViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L317). **negative:** `unit/verify` [`TestValidateTree_RangeViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L176). **negative:** `unit/verify` [`TestValidator_HoldTimeRange`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L901). **negative:** `unit/verify` [`TestValidator_ValidatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L852) |
| `RFC7950-8.1-4` | All key leafs MUST be present for all list entries. o Nodes MUST be present for at most one case branch in all choices. o There MUST be no nodes tagged with "if-feature" present if the "if-feature" expression evaluates to "false" in the server. o There MUST be no nodes tagged with "when" present if the "when" condition evaluates to "false" in the data tree. The following properties are true in a valid data tree: o All "must" constraints MUST evaluate to "true". o All referential integrity constraints defined via the "path" statement MUST be satisfied. o All "unique" constraints on lists MUST be satisfied. o The "mandatory" constraint is enforced for leafs and choices, unless the node or any of its ancestors has a "when" condition or "if-feature" expression that evaluates to "false". (§8.1) | MUST | 8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** walkTree evaluates no when, unique or choice; plan/release/fixes/spec-config-yang-when-unique-choice.md |
| `RFC7950-8.1-2` | The running configuration datastore MUST always be valid. (Section 8.1) | MUST | 8.1 | **positive:** `unit/verify` [`TestRFC7950ReloadInstallsAValidConfig`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc7950_reload_validation_test.go#L76). **positive:** `unit/verify` [`TestRFC7950RunningDatastoreAcceptsAValidConfig`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_datastore_test.go#L33). **negative:** `unit/verify` [`TestRFC7950ReloadRefusesAConstraintViolation`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc7950_reload_validation_test.go#L94). **negative:** `unit/verify` [`TestRFC7950RunningDatastoreRefusesAViolationAfterParse`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_datastore_test.go#L77). **negative:** `unit/verify` [`TestRFC7950WebCommitRefusesAConstraintViolation`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc7950_web_commit_validation_test.go#L82) |
| `RFC7950-8.3-1` | For configuration data, there are three windows when constraints MUST be enforced: o during parsing of RPC payloads o during processing of the <edit-config> operation o during validation (§8.3) | MUST | 8.3 | **positive:** `unit/verify` [`TestRFC7950RunningDatastoreAcceptsAValidConfig`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_datastore_test.go#L34). **positive:** `unit/verify` [`TestRFC7950WebCommitWritesAValidEdit`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc7950_web_commit_validation_test.go#L58). **negative:** `unit/verify` [`TestRFC7950RunningDatastoreRefusesAViolationAfterParse`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_datastore_test.go#L78). **negative:** `unit/verify` [`TestRFC7950RunningDatastoreRefusesAViolationAtParse`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_datastore_test.go#L53). **negative:** `unit/verify` [`TestRFC7950WebCommitRefusesAConstraintViolation`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc7950_web_commit_validation_test.go#L81) |
| `RFC7950-8.3.1-2` | If data for more than one case branch of a choice is present, the server MUST reply with a "bad-element" <error-tag> in the <rpc-error>. o If data for a node tagged with "if-feature" is present and the "if-feature" expression evaluates to "false" in the server, the server MUST reply with an "unknown-element" <error-tag> in the <rpc-error>. o If data for a node tagged with "when" is present and the "when" condition evaluates to "false", the server MUST reply with an "unknown-element" <error-tag> in the <rpc-error>. (§8.3.1) | MUST | 8.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** walkTree evaluates no when, unique or choice; plan/release/fixes/spec-config-yang-when-unique-choice.md |
| `RFC7950-8.3.2-1` | During this processing, the following errors MUST be detected: o Delete requests for non-existent data. o Create requests for existent data. o Insert requests with "before" or "after" parameters that do not exist. o Modification requests for nodes tagged with "when", and the "when" condition evaluates to "false". In this case, the server MUST reply with an "unknown-element" <error-tag> in the <rpc-error>. (§8.3.2) | MUST | 8.3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** walkTree evaluates no when, unique or choice; plan/release/fixes/spec-config-yang-when-unique-choice.md |
| `RFC7950-8.3.3-1` | When datastore processing is complete, the final contents MUST obey all validation constraints. (§8.3.3) | MUST | 8.3.3 | **positive:** `unit/verify` [`TestRFC7950RunningDatastoreAcceptsAValidConfig`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_datastore_test.go#L35). **negative:** `unit/verify` [`TestRFC7950RunningDatastoreRefusesAViolationAfterParse`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_datastore_test.go#L79) |
| `RFC7950-8.3.3-2` | If the datastore is "running" or "startup", these constraints MUST be enforced at the end of the <edit-config> or <copy-config> operation. (§8.3.3) | MUST | 8.3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** LoadConfig (the <copy-config> window), the web editor commit (internal/component/cli/editor_commit.go validateStagedTree, through config/cli.ValidateContent) and the SSH editor commit (internal/component/cli/validator.go sectionErrors) all run ValidateCustomSections, which refuses a type or ze:validate violation in the sections validatedSections lists, a missing mandatory leaf in every top-level section present in the config, and, through the whole-tree reference walk (Validator.ValidateReferences, the SSH editor commit included for the bgp section), a leafref or instance-identifier naming no instance, but none of them evaluates when, unique or choice exclusivity, and Ze's modules declare unique (ze-iface-conf), so the end of the operation does not enforce these constraints; plan/release/fixes/spec-config-yang-when-unique-choice.md |
| `RFC7950-9.1-1` | Implementations MUST support all lexical representations specified in this document. (§9.1) | MUST | 9.1 | **positive:** `unit/verify` [`TestRFC7950BinaryAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_lexical_test.go#L50). **positive:** `unit/verify` [`TestRFC7950BitsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_lexical_test.go#L77). **positive:** `unit/verify` [`TestRFC7950Decimal64Accepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_decimal64_test.go#L25). **positive:** `unit/verify` [`TestRFC7950InstanceIdentifierLeafListPredicate`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_walk_test.go#L438). **positive:** `unit/verify` [`TestRFC7950IntegerModuleDefaultHexAndOctal`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_integer_default_test.go#L39). **positive:** `unit/verify` [`TestRFC7950IntegerSignedLexicalForms`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_integer_test.go#L14). **positive:** `unit/verify` [`TestRFC7950ReferenceTypesLoad`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L77). **positive:** `unit/verify` [`TestRFC7950UnionDefaultLeafrefMember`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_walk_test.go#L157). **negative:** `unit/verify` [`TestRFC7950BinaryRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_lexical_test.go#L64). **negative:** `unit/verify` [`TestRFC7950BitsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_lexical_test.go#L90). **negative:** `unit/verify` [`TestRFC7950Decimal64Refused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_decimal64_test.go#L40). **negative:** `unit/verify` [`TestRFC7950InstanceIdentifierLeafListPredicateAbsent`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_walk_test.go#L450). **negative:** `unit/verify` [`TestRFC7950IntegerModuleDefaultMalformedNotation`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_integer_default_test.go#L88). **negative:** `unit/verify` [`TestRFC7950LeafrefLexical`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L146) |
| `RFC7950-9.2.4-2` | If multiple values or ranges are given, they all MUST be disjoint and MUST be in ascending order. If a range restriction is applied to a type that is already range-restricted, the new restriction MUST be equally limiting or more limiting, i.e., raising the lower bounds, reducing the upper bounds, removing explicit values or ranges, or splitting ranges into multiple ranges with intermediate gaps. Each explicit value and range boundary value given in the range expression MUST match the type being restricted or be one of the special values "min" or "max". (§9.2.4) | MUST | 9.2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| `RFC7950-9.3.1-1` | A decimal64 value is lexically represented as an optional sign ("+" or "-"), followed by a sequence of decimal digits, optionally followed by a period ('.') as a decimal indicator and a sequence of decimal digits. (§9.3.1) | MUST | 9.3.1 | **positive:** `unit/verify` [`TestRFC7950Decimal64Accepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_decimal64_test.go#L26). **negative:** `unit/verify` [`TestRFC7950Decimal64Refused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_decimal64_test.go#L41) |
| `RFC7950-9.3.4-1` | The "fraction-digits" statement, which is a substatement to the "type" statement, MUST be present if the type is "decimal64". (Section 9.3.4) | MUST | 9.3.4 | **positive:** `unit/verify` [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L141). **negative:** `unit/verify` [`TestRFC7950ModuleLoadRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L87) |
| `RFC7950-9.4.4-1` | Length-restricting values MUST NOT be negative. If multiple values or ranges are given, they all MUST be disjoint and MUST be in ascending order. (§9.4.4) | MUST | 9.4.4 | **positive:** `unit/verify` [`TestRFC7950LengthMinMaxAreTheRestrictedTypeBounds`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_structural_test.go#L59). **positive:** `unit/verify` [`TestRFC7950LengthPartsDisjointAndAscending`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_structural_test.go#L74). **positive:** `unit/verify` [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L143). **negative:** `unit/verify` [`TestRFC7950LengthMinMaxAreTheRestrictedTypeBounds`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_structural_test.go#L58). **negative:** `unit/verify` [`TestRFC7950LengthPartsDisjointAndAscending`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_structural_test.go#L73). **negative:** `unit/verify` [`TestRFC7950ModuleLoadRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L90) |
| `RFC7950-9.5.1-1` | The lexical representation of a boolean value is a string with a value of "true" or "false". These values MUST be in lowercase. (§9.5.1) | MUST | 9.5.1 | **positive:** `unit/verify` [`TestRFC7950BooleanLowercaseAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_test.go#L93). **negative:** `unit/verify` [`TestRFC7950BooleanNotLowercaseRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_test.go#L104) |
| `RFC7950-9.6.4-1` | The "enum" statement, which is a substatement to the "type" statement, MUST be present if the type is "enumeration". It is repeatedly used to specify each assigned name of an enumeration type. It takes as an argument a string that is the assigned name. The string MUST NOT be zero-length and MUST NOT have any leading or trailing whitespace characters (any Unicode character with the "White_Space" property). The use of Unicode control codes SHOULD be avoided. The statement is optionally followed by a block of substatements that holds detailed enum information. All assigned names in an enumeration MUST be unique. (§9.6.4) | MUST | 9.6.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| `RFC7950-9.6.4-2` | When an existing enumeration type is restricted, the set of assigned names in the new type MUST be a subset of the base type's set of assigned names. The value of such an assigned name MUST NOT be changed. (§9.6.4) | MUST | 9.6.4 | **positive:** `unit/verify` [`TestRFC7950EnumRestrictionKeepsTheBaseValue`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_structural_test.go#L14). **negative:** `unit/verify` [`TestRFC7950EnumRestrictionKeepsTheBaseValue`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_structural_test.go#L13) |
| `RFC7950-9.6.4.2-1` | This integer value MUST be in the range -2147483648 to 2147483647, and it MUST be unique within the enumeration type. If a value is not specified, then one will be automatically assigned. If the "enum" substatement is the first one defined, the assigned value is zero (0); otherwise, the assigned value is one greater than the current highest enum value (i.e., the highest enum value, implicit or explicit, prior to the current "enum" substatement in the parent "type" statement). Note that the presence of an "if-feature" statement in an "enum" statement does not affect the automatically assigned value. If the current highest value is equal to 2147483647, then an enum value MUST be specified for "enum" substatements following the one with the current highest value. When an existing enumeration type is restricted, the "value" statement MUST either have the same value as in the base type or not be present, in which case the value is the same as in the base type. (§9.6.4.2) | MUST | 9.6.4.2 | **positive:** `unit/verify` [`TestRFC7950EnumImplicitValueFollowsTheHighest`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_enum_value_test.go#L29). **positive:** `unit/verify` [`TestRFC7950EnumRestrictionKeepsTheBaseValue`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_structural_test.go#L12). **positive:** `unit/verify` [`TestRFC7950EnumValueRangeAndUniqueness`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_enum_value_test.go#L62). **positive:** `unit/verify` [`TestRFC7950EnumValueWithinInt32`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L107). **positive:** `unit/verify` [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L147). **negative:** `unit/verify` [`TestRFC7950EnumImplicitValueFollowsTheHighest`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_enum_value_test.go#L28). **negative:** `unit/verify` [`TestRFC7950EnumRestrictionKeepsTheBaseValue`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_structural_test.go#L11). **negative:** `unit/verify` [`TestRFC7950EnumValueRangeAndUniqueness`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_enum_value_test.go#L61). **negative:** `unit/verify` [`TestRFC7950EnumValueWithinInt32`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L106). **negative:** `unit/verify` [`TestRFC7950ModuleLoadRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L94) |
| `RFC7950-9.7-1` | When an existing bits type is restricted, the set of assigned names in the new type MUST be a subset of the base type's set of assigned names. The bit position of such an assigned name MUST NOT be changed. (§9.7) | MUST | 9.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze checks a bits value's names (validateBits, internal/component/config/yang/validator_lexical.go) but adds no check of its own on a bits type's bit statements, and goyang builds a restricted bits type's bits afresh from the restricting statements (types.go, (*Type).resolve: NewBitfield) without comparing them with the base type; plan/post-release/features/spec-config-yang-type-bits.md |
| `RFC7950-9.7.4-1` | The "bit" statement, which is a substatement to the "type" statement, MUST be present if the type is "bits". It is repeatedly used to specify each assigned named bit of a bits type. It takes as an argument a string that is the assigned name of the bit. It is followed by a block of substatements that holds detailed bit information. The assigned name follows the same syntax rules as an identifier (see Section 6.2). All assigned names in a bits type MUST be unique. (§9.7.4) | MUST | 9.7.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze checks a bits value's names (validateBits, internal/component/config/yang/validator_lexical.go) but adds no check of its own on a bits type's bit statements; goyang refuses a repeated bit name when it resolves the type (EnumType.Set: "field %s already assigned"), and no Ze test proves that refusal or the presence of a bit statement; plan/post-release/features/spec-config-yang-type-bits.md |
| `RFC7950-9.7.4.2-1` | The position value MUST be in the range 0 to 4294967295, and it MUST be unique within the bits type. If a bit position is not specified, then one will be automatically assigned. If the "bit" substatement is the first one defined, the assigned value is zero (0); otherwise, the assigned value is one greater than the current highest bit position (i.e., the highest bit position, implicit or explicit, prior to the current "bit" substatement in the parent "type" statement). Note that the presence of an "if-feature" statement in a "bit" statement does not affect the automatically assigned position. If the current highest bit position value is equal to 4294967295, then a position value MUST be specified for "bit" substatements following the one with the current highest position value. When an existing bits type is restricted, the "position" statement MUST either have the same value as in the base type or not be present, in which case the value is the same as in the base type. (§9.7.4.2) | MUST | 9.7.4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze checks a bits value's names (validateBits, internal/component/config/yang/validator_lexical.go) but adds no check of its own on a bits type's bit statements; goyang refuses a position outside 0 to 4294967295 (NewBitfield bounds, EnumType.Set) but accepts a position assigned twice, because a bitfield is built non-unique (NewBitfield), and never compares a restricted type's positions with the base type; plan/post-release/features/spec-config-yang-type-bits.md |
| `RFC7950-9.9-1` | If the "require-instance" property (Section 9.9.3) is "true", there MUST exist a node in the data tree, or a node with a default value in use (see Sections 7.6.1 and 7.7.2), of the referred schema tree leaf or leaf-list node with the same value as the leafref value in a valid data tree. If the referring node represents configuration data and the "require-instance" property (Section 9.9.3) is "true", the referred node MUST also represent configuration. There MUST NOT be any circular chains of leafrefs. If the leaf that the leafref refers to is conditional based on one or more features (see Section 7.20.2), then the leaf with the leafref type MUST also be conditional based on at least the same set of features. (§9.9) | MUST | 9.9 | **positive:** `unit/verify` [`TestRFC7950LeafrefDefaultInUse`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_walk_test.go#L64). **positive:** `unit/verify` [`TestRFC7950LeafrefPredicateCanonicalValue`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_canonical_test.go#L98). **positive:** `unit/verify` [`TestRFC7950LeafrefValueAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L85). **negative:** `unit/verify` [`TestRFC7950LeafrefConfigAndLoop`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L190). **negative:** `unit/verify` [`TestRFC7950LeafrefDefaultNotInUse`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_walk_test.go#L77). **negative:** `unit/verify` [`TestRFC7950LeafrefInstanceRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L100). **negative:** `unit/verify` [`TestRFC7950LeafrefUnionLoop`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_walk_test.go#L183) |
| `RFC7950-9.9.2-1` | The "path" statement, which is a substatement to the "type" statement, MUST be present if the type is "leafref". It takes as an argument a string that MUST refer to a leaf or leaf-list node. The syntax for a path argument is a subset of the XPath abbreviated syntax. Predicates are used only for constraining the values for the key nodes for list entries. Each predicate consists of exactly one equality test per key, and multiple adjacent predicates MAY be present if a list has multiple keys. The syntax is formally defined by the rule "path-arg" in Section 14. The predicates are only used when more than one key reference is needed to uniquely identify a leaf instance. This occurs if a list has multiple keys or a reference to a leaf other than the key in a list is needed. In these cases, multiple leafrefs are typically specified, and predicates are used to tie them together. The "path" expression evaluates to a node set consisting of zero, one, or more nodes. If the "require-instance" property is "true", this node set MUST be non-empty. (§9.9.2) | MUST | 9.9.2 | **positive:** `unit/verify` [`TestRFC7950LeafrefPathAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L179). **negative:** `unit/verify` [`TestRFC7950LeafrefPathRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L161) |
| `RFC7950-9.9.3-1` | If "require-instance" is "true", it means that the instance being referred to MUST exist for the data to be valid. (Section 9.9.3) | MUST | 9.9.3 | **positive:** `unit/verify` [`TestRFC7950LeafrefRequireInstance`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L112). **negative:** `unit/verify` [`TestRFC7950LeafrefRequireInstanceMissing`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L122) |
| `RFC7950-9.10.2-1` | The "base" statement, which is a substatement to the "type" statement, MUST be present at least once if the type is "identityref". The argument is the name of an identity, as defined by an "identity" statement. If a prefix is present on the identity name, it refers to an identity defined in the module that was imported with that prefix. Otherwise, an identity with the matching name MUST be defined in the current module or an included submodule. (§9.10.2) | MUST | 9.10.2 | **positive:** `unit/verify` [`TestRFC7950IdentityrefAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L205). **negative:** `unit/verify` [`TestRFC7950IdentityrefRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L216). **{partial}:** tested "If a prefix is present on the identity name, it refers to an identity defined in the module that was imported with that prefix"; gap "MUST be present at least once"; internal/component/config/yang/validator_reference.go::validateIdentityref is the boundary: Tested covers an identityref with one base, prefixed by an imported module's prefix or local, resolved by goyang (which refuses an identityref with no base or a base it cannot resolve), whose value must name an identity derived from it; Gap is a type naming the base statement more than once, because goyang keeps one base statement per identityref type (Type.IdentityBase holds one value), so the values derived from every base are not computed; plan/post-release/features/spec-config-yang-reference-types.md. **Scoped evidence:** Tests and tag-claim records apply only to Tested; zero whole-requirement credit. not audited: no reader has judged these tests |
| `RFC7950-9.10.3-1` | Otherwise, an identity with the matching name MUST be defined in the current module or one of its submodules. (Section 9.10.3) | MUST | 9.10.3 | **positive:** `unit/verify` [`TestRFC7950IdentityrefOtherModule`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L228). **negative:** `unit/verify` [`TestRFC7950IdentityrefPrefixRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L134) |
| `RFC7950-9.12-2` | When the type is "union", the "type" statement (Section 7.4) MUST be present. (Section 9.12) | MUST | 9.12 | **positive:** no positive test. **negative:** no negative test. **{gap}:** goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| `RFC7950-9.13-1` | For identifying list entries with keys, each predicate consists of one equality test per key, and each key MUST have a corresponding predicate. If a key is of type "empty", it is represented as a zero-length string (""). If the leaf with the instance-identifier type represents configuration data and the "require-instance" property (Section 9.9.3) is "true", the node it refers to MUST also represent configuration. Such a leaf puts a constraint on valid data. All such leaf nodes MUST reference existing nodes or leaf or leaf-list nodes with their default value in use (see Sections 7.6.1 and 7.7.2) for the data to be valid. (§9.13) | MUST | 9.13 | **positive:** `unit/verify` [`TestRFC7950InstanceIdentifierCanonicalValue`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_canonical_test.go#L73). **positive:** `unit/verify` [`TestRFC7950InstanceIdentifierDefaultInUse`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_walk_test.go#L88). **positive:** `unit/verify` [`TestRFC7950InstanceIdentifierExists`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L278). **negative:** `unit/verify` [`TestRFC7950InstanceIdentifierKeys`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L267) |
| `RFC7950-11-1` | For any published change, a new "revision" statement (Section 7.1.9) MUST be included in front of the existing "revision" statements. If there are no existing "revision" statements, then one MUST be added to identify the new revision. Furthermore, any necessary changes MUST be applied to any metadata statements, including the "organization" and "contact" statements (Sections 7.1.7 and 7.1.8). Note that definitions contained in a module are available to be imported by any other module and are referenced in "import" statements via the module name. Thus, a module name MUST NOT be changed. Furthermore, the "namespace" statement MUST NOT be changed, since all XML elements are qualified by the namespace. Obsolete definitions MUST NOT be removed from published modules, since their identifiers may still be referenced by other modules. (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no repository check reads the namespace or the revision history of the embedded modules; plan/post-release/engineering/spec-config-yang-authoring-checks.md |
| `RFC7950-11-2` | Otherwise, if the semantics of any previous definition are changed (i.e., if a non-editorial change is made to any definition other than those specifically allowed above), then this MUST be achieved by a new definition with a new identifier. (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no repository check reads the namespace or the revision history of the embedded modules; plan/post-release/engineering/spec-config-yang-authoring-checks.md |
| `RFC7950-11-3` | In statements that have any data definition statements as substatements, those data definition substatements MUST NOT be reordered. (§11) | MUST NOT | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no repository check reads the namespace or the revision history of the embedded modules; plan/post-release/engineering/spec-config-yang-authoring-checks.md |
| `RFC7950-7.2.2-1` | A submodule MUST only be included by either the module to which it belongs or another submodule that belongs to that module. (§7.2.2) | MUST | 7.2.2 | **positive:** `unit/verify` [`TestRFC7950SubmoduleIncludedOnlyByItsModule`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_structural_test.go#L104). **negative:** `unit/verify` [`TestRFC7950SubmoduleIncludedOnlyByItsModule`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_structural_test.go#L103) |
| `RFC7950-7.3.2-1` | The "type" statement, which MUST be present, defines the base type from which this type is derived. (§7.3.2) | MUST | 7.3.2 | **positive:** `unit/verify` [`TestRFC7950TypedefTypeMustBePresent`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L54). **negative:** `unit/verify` [`TestRFC7950TypedefTypeMustBePresent`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L53) |
| `RFC7950-7.9.2-2` | The case identifier MUST be unique within a choice. (§7.9.2) | MUST | 7.9.2 | **positive:** `unit/verify` [`TestRFC7950CaseIdentifierUniqueWithinAChoice`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L78). **negative:** `unit/verify` [`TestRFC7950CaseIdentifierUniqueWithinAChoice`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L77) |
| `RFC7950-7.21.5-2` | If the XPath expression references any node that also has associated "when" statements, those "when" expressions MUST be evaluated first. There MUST NOT be any circular dependencies among "when" expressions. (§7.21.5) | MUST | 7.21.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** walkTree evaluates no when, unique or choice; plan/release/fixes/spec-config-yang-when-unique-choice.md |
| `RFC7950-9.1-2` | If the data type does not have a canonical form, the format of the value MUST match the data type's lexical representation, but the exact format is implementation dependent. (§9.1) | MUST | 9.1 | **positive:** `unit/verify` [`TestRFC7950InstanceIdentifierLexical`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L249). **negative:** `unit/verify` [`TestRFC7950IdentityrefFormat`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L289). **negative:** `unit/verify` [`TestRFC7950InstanceIdentifierKeyPrefix`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_walk_test.go#L127) |
| `RFC7950-9.4.4-2` | If a length restriction is applied to a type that is already length-restricted, the new restriction MUST be equally limiting or more limiting, i.e., raising the lower bounds, reducing the upper bounds, removing explicit length values or ranges, or splitting ranges into multiple ranges with intermediate gaps. (§9.4.4) | MUST | 9.4.4 | **positive:** `unit/verify` [`TestRFC7950LengthRestrictionMustNotWiden`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L94). **negative:** `unit/verify` [`TestRFC7950LengthRestrictionMustNotWiden`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L93) |
| `RFC7950-12-1` | A YANG version 1.1 module MUST NOT include a YANG version 1 submodule, and a YANG version 1 module MUST NOT include a YANG version 1.1 submodule. A YANG version 1 module or submodule MUST NOT import a YANG version 1.1 module by revision. (§12) | MUST NOT | 12 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no Ze module declares yang-version and none is a submodule; plan/spec-config-yang-version-submodule.md |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7950-7.6.5-1`](#rfc7950-7.6.5-1) If "mandatory" is "true", the behavior of the constraint depends on the type of the leaf's closest ancestor node in the schema tree that is not a non-presence container (see Section 7.5.1): o If no such ancestor exists in the schema tree, the leaf MUST exist. (§7.6.5) | {partial}, Partial proof; remaining gap | tested "the leaf MUST exist"; gap "If no such ancestor exists in the schema tree"; internal/component/config/validate_sections.go::ValidateCustomSections is the boundary: Tested covers a mandatory leaf whose ancestors are all non-presence containers, required whenever its top-level section is present in the config (bgp/router-id, bgp/session/asn/local, once the bgp section holds a peer or a group); Gap is the same leaf when the whole top-level section is absent, which Ze does not require, because its top-level sections are non-presence containers that stand for an enabled feature and requiring their leaves would refuse every config that omits bgp; owner-approved deviation, OWNER RULING 6 of 2026-10-12 (Thomas, "Like FRR/libyang (Recommended)"): mandatory leaves are checked only in sections present in the config, as libyang's LYD_VALIDATE_PRESENT ("Validate only modules whose data actually exist") does when FRR sets it in nb_candidate_validate_yang (lib/northbound.c) and yang_parse_data (lib/yang.c); Gap also holds bgp/router-id and bgp/session/asn/local absent at the bgp level while every BGP session carries the value itself or from its group, which bgpMandatoryFailures counts as present through EverySessionCarries: a second owner-approved deviation, ruling of 2026-10-12 (Thomas, "yes enforce", spec AC-2); that ruling binds only once a session exists: a bgp section with no peer and no group is accepted and does nothing, so EverySessionCarries counts both leaves as present there (Thomas, 2026-10-16, spec AC-2 RULING 7); a deviation is never counted conformant; plan/release/fixes/spec-config-yang-when-unique-choice.md |
| [`RFC7950-7.5.3-1`](#rfc7950-7.5.3-1) All such constraints MUST evaluate to "true" for the data to be valid. (§7.5.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze uses zero YANG must statements (a grep of the .yang models finds must only inside a description); it parses via goyang which evaluates no XPath, and enforces cross-field constraints with Go ze:validate functions (internal/component/config/yang/validator.go:756 applyCustomValidators) instead, so there is no must-XPath code path |
| [`RFC7950-9.2.4-1`](#rfc7950-9.2.4-1) If a range restriction is applied to a type that is already range-restricted, the new restriction MUST be equally limiting or more limiting, i.e., raising the lower bounds, reducing the upper bounds, removing explicit values or ranges, or splitting ranges into multiple ranges with intermediate gaps. (§9.2.4) | no test | no test carries this requirement id; annotated {not-applicable}: this is a schema-authoring-time narrowing constraint resolved by goyang during module processing (internal/component/config/yang/loader.go); ze authors valid narrowings and has no runtime enforcement surface for it |
| [`RFC7950-9.4.5-1`](#rfc7950-9.4.5-1) If the type has multiple "pattern" statements, the expressions are ANDed together, i.e., all such expressions have to match. (§9.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: no ze type carries two or more pattern statements (a scan of the .yang models finds none); the validator loop (internal/component/config/yang/validator.go:268) AND-combines patterns if present, but the multi-pattern construct is unused |
| [`RFC7950-x-1`](#rfc7950-x-1) The "yang-version" statement specifies which version of the YANG language was used in developing the module. The statement's argument is a string. It MUST contain the value "1.1" for YANG modules defined based on this specification. (§7.1.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's YANG models declare no yang-version statement and default to YANG 1.0 in goyang; they use no YANG-1.1-only construct that would require the 1.1 declaration, so the version-declaration obligation has no applicable module |
| [`RFC7950-x-2`](#rfc7950-x-2) An "enumeration" type may have new enums added, provided the old enums's values do not change. (§11) | no test | no test carries this requirement id; annotated {not-applicable}: ze's modules carry no consumed revision statements and there is no cross-revision position-stability tooling; ze pins the values that matter with explicit value statements (e.g. role.yang, ze-types.yang afi/safi) but enforces no automated cross-revision check |
| [`RFC7950-5.1-1`](#rfc7950-5.1-1) Within a server, all module names MUST be unique. A module uses the "include" statement to list all its submodules. A module, or submodule belonging to that module, can reference definitions in the module and all submodules included by the module. A module or submodule uses the "import" statement to reference external modules. Statements in the module or submodule can reference definitions in the external module using the prefix specified in the "import" statement. For backward compatibility with YANG version 1, a submodule MAY use the "include" statement to reference other submodules within its module, but this is not necessary in YANG version 1.1. A submodule can reference any definition in the module it belongs to and in all submodules included by the module. A submodule MUST NOT include different revisions of other submodules than the revisions that its module includes. A module or submodule MUST NOT include submodules from other modules, and a submodule MUST NOT import its own module. The "import" and "include" statements are used to make definitions available from other modules: o For a module or submodule to reference definitions in an external module, the external module MUST be imported. o A module MUST include all its submodules. o A module, or submodule belonging to that module, MAY reference definitions in the module and all submodules included by the module. There MUST NOT be any circular chains of imports. For example, if module "a" imports module "b", "b" cannot import "a". When a definition in an external module is referenced, a locally defined prefix MUST be used, followed by a colon (":") and then the external identifier. (§5.1) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-5.3-1`](#rfc7950-5.3-1) Namespace URIs MUST be chosen so they cannot collide with standard or other enterprise namespaces -- for example, by using the enterprise or organization name in the namespace. (Section 5.3) | {gap}, no test | no repository check reads the namespace or the revision history of the embedded modules; plan/post-release/engineering/spec-config-yang-authoring-checks.md |
| [`RFC7950-5.5-1`](#rfc7950-5.5-1) Scoped definitions MUST NOT shadow definitions at a higher scope. (§5.5) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-5.6.5-1`](#rfc7950-5.6.5-1) A server MUST NOT implement more than one revision of a module. If a server implements a module A that imports a module B, and A uses any node from B in an "augment" or "path" statement that the server supports, then the server MUST implement a revision of module B that has these nodes defined. (§5.6.5) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-6.2.1-1`](#rfc7950-6.2.1-1) All identifiers defined in a namespace MUST be unique. o All module and submodule names share the same global module identifier namespace. o All extension names defined in a module and its submodules share the same extension identifier namespace. o All feature names defined in a module and its submodules share the same feature identifier namespace. o All identity names defined in a module and its submodules share the same identity identifier namespace. o All derived type names defined within a parent node or at the top level of the module or its submodules share the same type identifier namespace. This namespace is scoped to all descendant nodes of the parent node or module. This means that any descendant node may use that typedef, and it MUST NOT define a typedef with the same name. o All grouping names defined within a parent node or at the top level of the module or its submodules share the same grouping identifier namespace. This namespace is scoped to all descendant nodes of the parent node or module. This means that any descendant node may use that grouping, and it MUST NOT define a grouping with the same name. (§6.2.1) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-6.3.1-1`](#rfc7950-6.3.1-1) When an imported extension is used, the extension's keyword MUST be qualified using the prefix with which the extension's module was imported. If an extension is used in the module where it is defined, the extension's keyword MUST be qualified with the prefix of this module. The processing of extensions depends on whether support for those extensions is claimed for a given YANG parser or the tool set in which it is embedded. An unsupported extension appearing in a YANG module as an unknown-statement (see Section 14) MAY be ignored in its entirety. Any supported extension MUST be processed in accordance with the specification governing that extension. (§6.3.1) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-6.4-1`](#rfc7950-6.4-1) An implementation is not required to implement an XPath interpreter but MUST ensure that the requirements encoded in the data model are enforced. (Section 6.4) | {gap}, no test | walkTree evaluates no when, unique or choice; plan/release/fixes/spec-config-yang-when-unique-choice.md |
| [`RFC7950-6.4-2`](#rfc7950-6.4-2) The XPath expressions MUST be syntactically correct, and all prefixes used MUST be present in the XPath context (see Section 6.4.1). (§6.4) | {gap}, no test | walkTree evaluates no when, unique or choice; plan/release/fixes/spec-config-yang-when-unique-choice.md |
| [`RFC7950-7.1.4-1`](#rfc7950-7.1.4-1) If there is a conflict, i.e., two different modules that both have defined the same prefix are imported, at least one of them MUST be imported with a different prefix. All prefixes, including the prefix for the module itself, MUST be unique within the module or submodule. (§7.1.4) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-7.3-1`](#rfc7950-7.3-1) The "typedef" statement's argument is an identifier that is the name of the type to be defined and MUST be followed by a block of substatements that holds detailed typedef information. The name of the type MUST NOT be one of the YANG built-in types. If the typedef is defined at the top level of a YANG module or submodule, the name of the type to be defined MUST be unique within the module. (§7.3) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-7.6.1-4`](#rfc7950-7.6.1-4) In these cases, the default value is said to be in use. Note that if the leaf or any of its ancestors has a "when" condition or "if-feature" expression that evaluates to "false", then the default value is not in use. When the default value is in use, the server MUST operationally behave as if the leaf was present in the data tree with the default value as its value. (§7.6.1) | {gap}, no test | no when is evaluated and Ze has no if-feature, so a default whose "when" is false stays in use; plan/release/fixes/spec-config-yang-when-unique-choice.md |
| [`RFC7950-7.7-1`](#rfc7950-7.7-1) In configuration data, the values in a leaf-list MUST be unique. The definitions of the default values MUST NOT be marked with an "if-feature" statement. Conceptually, the values in the data tree MUST be in the canonical form (see Section 9.1). (§7.7) | {gap}, no test | yangToList tolerates a keyless config list and leaf-lists are neither deduplicated nor defaulted; plan/release/fixes/spec-config-yang-list-key-leaf-list.md |
| [`RFC7950-7.7.2-1`](#rfc7950-7.7.2-1) The usage of the default values depends on the leaf-list's closest ancestor node in the schema tree that is not a non-presence container (see Section 7.5.1): o If no such ancestor exists in the schema tree, the default values MUST be used. o Otherwise, if this ancestor is a case node, the default values MUST be used if any node from the case exists in the data tree or the case node is the choice's default case, and if no nodes from any other case exist in the data tree. o Otherwise, the default values MUST be used if the ancestor node exists in the data tree. In these cases, the default values are said to be in use. Note that if the leaf-list or any of its ancestors has a "when" condition or "if-feature" expression that evaluates to "false", then the default values are not in use. When the default values are in use, the server MUST operationally behave as if the leaf-list was present in the data tree with the default values as its values. (§7.7.2) | {gap}, no test | yangToList tolerates a keyless config list and leaf-lists are neither deduplicated nor defaulted; plan/release/fixes/spec-config-yang-list-key-leaf-list.md |
| [`RFC7950-7.8.2-1`](#rfc7950-7.8.2-1) The "key" statement, which MUST be present if the list represents configuration and MAY be present otherwise, takes as an argument a string that specifies a space-separated list of one or more leaf identifiers of this list. A leaf identifier MUST NOT appear more than once in the key. Each such leaf identifier MUST refer to a child leaf of the list. The leafs can be defined directly in substatements to the list or in groupings used in the list. The combined values of all the leafs specified in the key are used to uniquely identify a list entry. All key leafs MUST be given values when a list entry is created. Thus, any default values in the key leafs or their types are ignored. Any "mandatory" statements in the key leafs are ignored. A leaf that is part of the key can be of any built-in or derived type. All key leafs in a list MUST have the same value for their "config" as the list itself. (§7.8.2) | {gap}, no test | yangToList tolerates a keyless config list and leaf-lists are neither deduplicated nor defaulted; plan/release/fixes/spec-config-yang-list-key-leaf-list.md |
| [`RFC7950-7.8.3-1`](#rfc7950-7.8.3-1) The "unique" statement is used to put constraints on valid list entries. It takes as an argument a string that contains a space- separated list of schema node identifiers, which MUST be given in the descendant form (see the rule "descendant-schema-nodeid" in Section 14). Each such schema node identifier MUST refer to a leaf. If one of the referenced leafs represents configuration data, then all of the referenced leafs MUST represent configuration data. The "unique" constraint specifies that the combined values of all the leaf instances specified in the argument string, including leafs with default values, MUST be unique within all list entry instances in which all referenced leafs exist or have default values. (§7.8.3) | {gap}, no test | walkTree evaluates no when, unique or choice; plan/release/fixes/spec-config-yang-when-unique-choice.md |
| [`RFC7950-7.9.2-1`](#rfc7950-7.9.2-1) The identifiers of all these child nodes MUST be unique within all cases in a choice. (§7.9.2) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-7.9.3-1`](#rfc7950-7.9.3-1) The "default" statement MUST NOT be present on choices where "mandatory" is "true". The default case is only important when considering the "default" statements of nodes under the cases (i.e., default values of leafs and leaf-lists, and default cases of nested choices). The default values and nested default cases under the default case are used if none of the nodes under any of the cases are present. There MUST NOT be any mandatory nodes (Section 3) directly under the default case. (§7.9.3) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-7.9.4-1`](#rfc7950-7.9.4-1) If "mandatory" is "true", at least one node from exactly one of the choice's case branches MUST exist. (Section 7.9.4) | {partial}, Partial proof; remaining gap | tested "at least one node from"; gap "exactly one of the choice's case branches"; internal/component/config/yang/validator_mandatory.go::mandatoryPresent is the boundary: Tested covers a mandatory choice reported missing when no data node of any of its cases exists, and satisfied by a node of one case, nested choices included; Gap is data holding nodes from two cases of one choice, which mandatoryPresent accepts at the first case node it finds, and no other check compares the cases; plan/release/fixes/spec-config-yang-when-unique-choice.md |
| [`RFC7950-7.12-1`](#rfc7950-7.12-1) A grouping MUST NOT reference itself, neither directly nor indirectly through a chain of other groupings. If the grouping is defined at the top level of a YANG module or submodule, the grouping's identifier MUST be unique within the module. (§7.12) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-7.14.2-1`](#rfc7950-7.14.2-1) If a leaf in the input tree has a "mandatory" statement with the value "true", the leaf MUST be present in an RPC invocation. If a leaf in the input tree has a default value, the server MUST use this value in the same cases as those described in Section 7.6.1. In these cases, the server MUST operationally behave as if the leaf was present in the RPC invocation with the default value as its value. If a leaf-list in the input tree has one or more default values, the server MUST use these values in the same cases as those described in Section 7.7.2. In these cases, the server MUST operationally behave as if the leaf-list was present in the RPC invocation with the default values as its values. Since the input tree is not part of any datastore, all "config" statements for nodes in the input tree are ignored. If any node has a "when" statement that would evaluate to "false", then this node MUST NOT be present in the input tree. (§7.14.2) | {gap}, no test | no invocation or reply path reads mandatory or default on rpc input and output; plan/release/fixes/spec-config-yang-rpc-mandatory-defaults.md |
| [`RFC7950-7.14.3-1`](#rfc7950-7.14.3-1) If a leaf in the output tree has a "mandatory" statement with the value "true", the leaf MUST be present in an RPC reply. If a leaf in the output tree has a default value, the client MUST use this value in the same cases as those described in Section 7.6.1. In these cases, the client MUST operationally behave as if the leaf was present in the RPC reply with the default value as its value. If a leaf-list in the output tree has one or more default values, the client MUST use these values in the same cases as those described in Section 7.7.2. In these cases, the client MUST operationally behave as if the leaf-list was present in the RPC reply with the default values as its values. Since the output tree is not part of any datastore, all "config" statements for nodes in the output tree are ignored. If any node has a "when" statement that would evaluate to "false", then this node MUST NOT be present in the output tree. (§7.14.3) | {gap}, no test | no invocation or reply path reads mandatory or default on rpc input and output; plan/release/fixes/spec-config-yang-rpc-mandatory-defaults.md |
| [`RFC7950-7.15-1`](#rfc7950-7.15-1) An action MUST NOT be defined within an rpc, another action, or a notification, i.e., an action node MUST NOT have an rpc, action, or a notification node as one of its ancestors in the schema tree. For example, this means that it is an error if a grouping that contains an action somewhere in its node hierarchy is used in a notification definition. An action MUST NOT have any ancestor node that is a list node without a "key" statement. (§7.15) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-7.16-1`](#rfc7950-7.16-1) A notification MUST NOT be defined within an rpc, action, or another notification, i.e., a notification node MUST NOT have an rpc, action, or a notification node as one of its ancestors in the schema tree. For example, this means that it is an error if a grouping that contains a notification somewhere in its node hierarchy is used in an rpc definition. A notification MUST NOT have any ancestor node that is a list node without a "key" statement. Since a notification cannot be defined in a "case" statement, it is an error if a grouping that contains a notification at the top of its node hierarchy is used in a case definition. If a leaf in the notification tree has a "mandatory" statement with the value "true", the leaf MUST be present in a notification instance. If a leaf in the notification tree has a default value, the client MUST use this value in the same cases as those described in Section 7.6.1. In these cases, the client MUST operationally behave as if the leaf was present in the notification instance with the default value as its value. If a leaf-list in the notification tree has one or more default values, the client MUST use these values in the same cases as those described in Section 7.7.2. In these cases, the client MUST operationally behave as if the leaf-list was present in the notification instance with the default values as its values. (§7.16) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-7.17-1`](#rfc7950-7.17-1) The target node MUST be either a container, list, choice, case, input, output, or notification node. It is augmented with the nodes defined in the substatements that follow the "augment" statement. The argument string is a schema node identifier (see Section 6.5). If the "augment" statement is on the top level in a module or submodule, the absolute form (defined by the rule "absolute-schema-nodeid" in Section 14) of a schema node identifier MUST be used. If the "augment" statement is a substatement to the "uses" statement, the descendant form (defined by the rule "descendant-schema-nodeid" in Section 14) MUST be used. If the target node is a container, list, case, input, output, or notification node, the "container", "leaf", "list", "leaf-list", "uses", and "choice" statements can be used within the "augment" statement. If the target node is a container or list node, the "action" and "notification" statements can be used within the "augment" statement. If the target node is a choice node, the "case" statement or a shorthand "case" statement (see Section 7.9.2) can be used within the "augment" statement. The "augment" statement MUST NOT add multiple nodes with the same name from the same module to the target node. If the augmentation adds mandatory nodes (see Section 3) that represent configuration to a target node in another module, the augmentation MUST be made conditional with a "when" statement. (§7.17) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-7.18.2-1`](#rfc7950-7.18.2-1) If a prefix is present on the base name, it refers to an identity defined in the module that was imported with that prefix, or the local module if the prefix matches the local module's prefix. Otherwise, an identity with the matching name MUST be defined in the current module or an included submodule. An identity MUST NOT reference itself, neither directly nor indirectly through a chain of other identities. (§7.18.2) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-7.20.1-1`](#rfc7950-7.20.1-1) A feature MUST NOT reference itself, neither directly nor indirectly through a chain of other features. In order for a server to support a feature that is dependent on any other features (i.e., the feature has one or more "if-feature" substatements), the server MUST also support all the dependent features. (§7.20.1) | {gap}, no test | Ze evaluates no if-feature and declares no feature; plan/spec-config-yang-if-feature.md |
| [`RFC7950-7.20.2-1`](#rfc7950-7.20.2-1) If a prefix is present on a feature name in the boolean expression, the prefixed name refers to a feature defined in the module that was imported with that prefix, or the local module if the prefix matches the local module's prefix. Otherwise, a feature with the matching name MUST be defined in the current module or an included submodule. A leaf that is a list key MUST NOT have any "if-feature" statements. (§7.20.2) | {gap}, no test | Ze evaluates no if-feature and declares no feature; plan/spec-config-yang-if-feature.md |
| [`RFC7950-7.20.3-1`](#rfc7950-7.20.3-1) This means that deviations MUST never be part of a published standard, since they are the mechanism for learning how implementations vary from the standards. Server deviations are strongly discouraged and MUST only be used as a last resort. Telling the application how a server fails to follow a standard is no substitute for implementing the standard correctly. A server that deviates from a module is not fully compliant with the module. However, in some cases, a particular device may not have the hardware or software ability to support parts of a standard module. When this occurs, the server makes a choice to either treat attempts to configure unsupported parts of the module as an error that is reported back to the unsuspecting application or ignore those incoming requests. Neither choice is acceptable. Instead, YANG allows servers to document portions of a base module that are not supported, or that are supported but with different syntax, by using the "deviation" statement. After applying all deviations announced by a server, in any order, the resulting data model MUST still be valid. (§7.20.3) | {gap}, no test | Ze reads no deviation; plan/spec-config-yang-deviation.md |
| [`RFC7950-7.20.3.2-1`](#rfc7950-7.20.3.2-1) The argument "add" adds properties to the target node. The properties to add are identified by substatements to the "deviate" statement. If a property can only appear once, the property MUST NOT exist in the target node. The argument "replace" replaces properties of the target node. The properties to replace are identified by substatements to the "deviate" statement. The properties to replace MUST exist in the target node. The argument "delete" deletes properties from the target node. The properties to delete are identified by substatements to the "delete" statement. The substatement's keyword MUST match a corresponding keyword in the target node, and the argument's string MUST be equal to the corresponding keyword's argument string in the target node. (§7.20.3.2) | {gap}, no test | Ze reads no deviation; plan/spec-config-yang-deviation.md |
| [`RFC7950-7.21.2-1`](#rfc7950-7.21.2-1) If a definition is "current", it MUST NOT reference a "deprecated" or "obsolete" definition within the same module. If a definition is "deprecated", it MUST NOT reference an "obsolete" definition within the same module. (§7.21.2) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-7.21.5-1`](#rfc7950-7.21.5-1) A leaf that is a list key MUST NOT have a "when" statement. If a key leaf is defined in a grouping that is used in a list, the "uses" statement MUST NOT have a "when" statement. (§7.21.5) | {gap}, no test | walkTree evaluates no when, unique or choice; plan/release/fixes/spec-config-yang-when-unique-choice.md |
| [`RFC7950-8.1-1`](#rfc7950-8.1-1) These constraints are enforced in different ways, depending on what type of data the statement defines. o If the constraint is defined on configuration data, it MUST be true in a valid configuration data tree. o If the constraint is defined on state data, it MUST be true in a valid state data tree. o If the constraint is defined on notification content, it MUST be true in any notification data tree. o If the constraint is defined on RPC or action input parameters, it MUST be true in an invocation of the RPC or action operation. o If the constraint is defined on RPC or action output parameters, it MUST be true in the RPC or action reply. (§8.1) | {gap}, no test | walkTree evaluates no when, unique or choice; plan/release/fixes/spec-config-yang-when-unique-choice.md |
| [`RFC7950-8.1-4`](#rfc7950-8.1-4) All key leafs MUST be present for all list entries. o Nodes MUST be present for at most one case branch in all choices. o There MUST be no nodes tagged with "if-feature" present if the "if-feature" expression evaluates to "false" in the server. o There MUST be no nodes tagged with "when" present if the "when" condition evaluates to "false" in the data tree. The following properties are true in a valid data tree: o All "must" constraints MUST evaluate to "true". o All referential integrity constraints defined via the "path" statement MUST be satisfied. o All "unique" constraints on lists MUST be satisfied. o The "mandatory" constraint is enforced for leafs and choices, unless the node or any of its ancestors has a "when" condition or "if-feature" expression that evaluates to "false". (§8.1) | {gap}, no test | walkTree evaluates no when, unique or choice; plan/release/fixes/spec-config-yang-when-unique-choice.md |
| [`RFC7950-8.3.1-2`](#rfc7950-8.3.1-2) If data for more than one case branch of a choice is present, the server MUST reply with a "bad-element" <error-tag> in the <rpc-error>. o If data for a node tagged with "if-feature" is present and the "if-feature" expression evaluates to "false" in the server, the server MUST reply with an "unknown-element" <error-tag> in the <rpc-error>. o If data for a node tagged with "when" is present and the "when" condition evaluates to "false", the server MUST reply with an "unknown-element" <error-tag> in the <rpc-error>. (§8.3.1) | {gap}, no test | walkTree evaluates no when, unique or choice; plan/release/fixes/spec-config-yang-when-unique-choice.md |
| [`RFC7950-8.3.2-1`](#rfc7950-8.3.2-1) During this processing, the following errors MUST be detected: o Delete requests for non-existent data. o Create requests for existent data. o Insert requests with "before" or "after" parameters that do not exist. o Modification requests for nodes tagged with "when", and the "when" condition evaluates to "false". In this case, the server MUST reply with an "unknown-element" <error-tag> in the <rpc-error>. (§8.3.2) | {gap}, no test | walkTree evaluates no when, unique or choice; plan/release/fixes/spec-config-yang-when-unique-choice.md |
| [`RFC7950-8.3.3-2`](#rfc7950-8.3.3-2) If the datastore is "running" or "startup", these constraints MUST be enforced at the end of the <edit-config> or <copy-config> operation. (§8.3.3) | {gap}, no test | LoadConfig (the <copy-config> window), the web editor commit (internal/component/cli/editor_commit.go validateStagedTree, through config/cli.ValidateContent) and the SSH editor commit (internal/component/cli/validator.go sectionErrors) all run ValidateCustomSections, which refuses a type or ze:validate violation in the sections validatedSections lists, a missing mandatory leaf in every top-level section present in the config, and, through the whole-tree reference walk (Validator.ValidateReferences, the SSH editor commit included for the bgp section), a leafref or instance-identifier naming no instance, but none of them evaluates when, unique or choice exclusivity, and Ze's modules declare unique (ze-iface-conf), so the end of the operation does not enforce these constraints; plan/release/fixes/spec-config-yang-when-unique-choice.md |
| [`RFC7950-9.2.4-2`](#rfc7950-9.2.4-2) If multiple values or ranges are given, they all MUST be disjoint and MUST be in ascending order. If a range restriction is applied to a type that is already range-restricted, the new restriction MUST be equally limiting or more limiting, i.e., raising the lower bounds, reducing the upper bounds, removing explicit values or ranges, or splitting ranges into multiple ranges with intermediate gaps. Each explicit value and range boundary value given in the range expression MUST match the type being restricted or be one of the special values "min" or "max". (§9.2.4) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-9.6.4-1`](#rfc7950-9.6.4-1) The "enum" statement, which is a substatement to the "type" statement, MUST be present if the type is "enumeration". It is repeatedly used to specify each assigned name of an enumeration type. It takes as an argument a string that is the assigned name. The string MUST NOT be zero-length and MUST NOT have any leading or trailing whitespace characters (any Unicode character with the "White_Space" property). The use of Unicode control codes SHOULD be avoided. The statement is optionally followed by a block of substatements that holds detailed enum information. All assigned names in an enumeration MUST be unique. (§9.6.4) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-9.7-1`](#rfc7950-9.7-1) When an existing bits type is restricted, the set of assigned names in the new type MUST be a subset of the base type's set of assigned names. The bit position of such an assigned name MUST NOT be changed. (§9.7) | {gap}, no test | Ze checks a bits value's names (validateBits, internal/component/config/yang/validator_lexical.go) but adds no check of its own on a bits type's bit statements, and goyang builds a restricted bits type's bits afresh from the restricting statements (types.go, (*Type).resolve: NewBitfield) without comparing them with the base type; plan/post-release/features/spec-config-yang-type-bits.md |
| [`RFC7950-9.7.4-1`](#rfc7950-9.7.4-1) The "bit" statement, which is a substatement to the "type" statement, MUST be present if the type is "bits". It is repeatedly used to specify each assigned named bit of a bits type. It takes as an argument a string that is the assigned name of the bit. It is followed by a block of substatements that holds detailed bit information. The assigned name follows the same syntax rules as an identifier (see Section 6.2). All assigned names in a bits type MUST be unique. (§9.7.4) | {gap}, no test | Ze checks a bits value's names (validateBits, internal/component/config/yang/validator_lexical.go) but adds no check of its own on a bits type's bit statements; goyang refuses a repeated bit name when it resolves the type (EnumType.Set: "field %s already assigned"), and no Ze test proves that refusal or the presence of a bit statement; plan/post-release/features/spec-config-yang-type-bits.md |
| [`RFC7950-9.7.4.2-1`](#rfc7950-9.7.4.2-1) The position value MUST be in the range 0 to 4294967295, and it MUST be unique within the bits type. If a bit position is not specified, then one will be automatically assigned. If the "bit" substatement is the first one defined, the assigned value is zero (0); otherwise, the assigned value is one greater than the current highest bit position (i.e., the highest bit position, implicit or explicit, prior to the current "bit" substatement in the parent "type" statement). Note that the presence of an "if-feature" statement in a "bit" statement does not affect the automatically assigned position. If the current highest bit position value is equal to 4294967295, then a position value MUST be specified for "bit" substatements following the one with the current highest position value. When an existing bits type is restricted, the "position" statement MUST either have the same value as in the base type or not be present, in which case the value is the same as in the base type. (§9.7.4.2) | {gap}, no test | Ze checks a bits value's names (validateBits, internal/component/config/yang/validator_lexical.go) but adds no check of its own on a bits type's bit statements; goyang refuses a position outside 0 to 4294967295 (NewBitfield bounds, EnumType.Set) but accepts a position assigned twice, because a bitfield is built non-unique (NewBitfield), and never compares a restricted type's positions with the base type; plan/post-release/features/spec-config-yang-type-bits.md |
| [`RFC7950-9.10.2-1`](#rfc7950-9.10.2-1) The "base" statement, which is a substatement to the "type" statement, MUST be present at least once if the type is "identityref". The argument is the name of an identity, as defined by an "identity" statement. If a prefix is present on the identity name, it refers to an identity defined in the module that was imported with that prefix. Otherwise, an identity with the matching name MUST be defined in the current module or an included submodule. (§9.10.2) | {partial}, Partial proof; remaining gap | tested "If a prefix is present on the identity name, it refers to an identity defined in the module that was imported with that prefix"; gap "MUST be present at least once"; internal/component/config/yang/validator_reference.go::validateIdentityref is the boundary: Tested covers an identityref with one base, prefixed by an imported module's prefix or local, resolved by goyang (which refuses an identityref with no base or a base it cannot resolve), whose value must name an identity derived from it; Gap is a type naming the base statement more than once, because goyang keeps one base statement per identityref type (Type.IdentityBase holds one value), so the values derived from every base are not computed; plan/post-release/features/spec-config-yang-reference-types.md |
| [`RFC7950-9.12-2`](#rfc7950-9.12-2) When the type is "union", the "type" statement (Section 7.4) MUST be present. (Section 9.12) | {gap}, no test | goyang accepts the violating module and Ze adds no check of its own; plan/release/fixes/spec-config-yang-loader-structural-checks.md |
| [`RFC7950-11-1`](#rfc7950-11-1) For any published change, a new "revision" statement (Section 7.1.9) MUST be included in front of the existing "revision" statements. If there are no existing "revision" statements, then one MUST be added to identify the new revision. Furthermore, any necessary changes MUST be applied to any metadata statements, including the "organization" and "contact" statements (Sections 7.1.7 and 7.1.8). Note that definitions contained in a module are available to be imported by any other module and are referenced in "import" statements via the module name. Thus, a module name MUST NOT be changed. Furthermore, the "namespace" statement MUST NOT be changed, since all XML elements are qualified by the namespace. Obsolete definitions MUST NOT be removed from published modules, since their identifiers may still be referenced by other modules. (§11) | {gap}, no test | no repository check reads the namespace or the revision history of the embedded modules; plan/post-release/engineering/spec-config-yang-authoring-checks.md |
| [`RFC7950-11-2`](#rfc7950-11-2) Otherwise, if the semantics of any previous definition are changed (i.e., if a non-editorial change is made to any definition other than those specifically allowed above), then this MUST be achieved by a new definition with a new identifier. (§11) | {gap}, no test | no repository check reads the namespace or the revision history of the embedded modules; plan/post-release/engineering/spec-config-yang-authoring-checks.md |
| [`RFC7950-11-3`](#rfc7950-11-3) In statements that have any data definition statements as substatements, those data definition substatements MUST NOT be reordered. (§11) | {gap}, no test | no repository check reads the namespace or the revision history of the embedded modules; plan/post-release/engineering/spec-config-yang-authoring-checks.md |
| [`RFC7950-7.21.5-2`](#rfc7950-7.21.5-2) If the XPath expression references any node that also has associated "when" statements, those "when" expressions MUST be evaluated first. There MUST NOT be any circular dependencies among "when" expressions. (§7.21.5) | {gap}, no test | walkTree evaluates no when, unique or choice; plan/release/fixes/spec-config-yang-when-unique-choice.md |
| [`RFC7950-12-1`](#rfc7950-12-1) A YANG version 1.1 module MUST NOT include a YANG version 1 submodule, and a YANG version 1 module MUST NOT include a YANG version 1.1 submodule. A YANG version 1 module or submodule MUST NOT import a YANG version 1.1 module by revision. (§12) | {gap}, no test | no Ze module declares yang-version and none is a submodule; plan/spec-config-yang-version-submodule.md |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7950-9.6-1`](#rfc7950-9.6-1)

The enumeration built-in type represents values from a set of assigned names. (§9.6)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/config/rfc7950_validator_yang_test.go::TestValidateTree_AddPathDirectionEnum, internal/component/config/rfc7950_validator_yang_test.go::TestValidateTree_AddPathDirectionEnum#2, internal/component/config/rfc7950_validator_yang_test.go::TestValidateTree_FamilyModeEnum, internal/component/config/rfc7950_validator_yang_test.go::TestValidateTree_FamilyModeEnum#2 moved. Re-judged 2026-10-12, second pass (independent auditor) after 60a74fc04b, 600996fb77, 975428e1b3, 3f1bec74f0 and 84382339ce. SHIFTED only: the tagged units are byte-identical (gate-verified) and validateEnumeration is unchanged. Re-read: TestValidateTree_FamilyModeEnum and TestValidateTree_AddPathDirectionEnum supply router-id and session/asn/local, so no missing-mandatory error precedes errs[0]; the invalid name is refused ErrTypeEnum and every assigned name gives no error; radius auth-method and isis algorithm refuse an unassigned name with ErrTypeEnum and accept every defined one. Re-run green 2026-10-12. Verdict stands.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISAuthAlgorithmEnumAcceptsAll`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_isis_auth_algorithm_enum_test.go#L81) | unit/verify | unproven |
| negative | [`TestRadiusAuthMethodEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_radius_auth_method_enum_test.go#L64) | unit/verify | revert, verified |
| negative | [`TestValidateTree_AddPathDirectionEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L620) | unit/verify | unproven |
| negative | [`TestValidateTree_FamilyModeEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L554) | unit/verify | unproven |
| positive | [`TestISISAuthAlgorithmEnumAcceptsAll`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_isis_auth_algorithm_enum_test.go#L64) | unit/verify | unproven |
| positive | [`TestRadiusAuthMethodEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_radius_auth_method_enum_test.go#L53) | unit/verify | revert, verified |
| positive | [`TestValidateTree_AddPathDirectionEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L619) | unit/verify | unproven |
| positive | [`TestValidateTree_FamilyModeEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L553) | unit/verify | unproven |

### [`RFC7950-9.12-1`](#rfc7950-9.12-1)

The union built-in type represents a value that corresponds to one of its member types. (§9.12)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/config/rfc7950_validator_yang_test.go::TestValidateTree_UnionViolation, internal/component/config/rfc7950_validator_yang_test.go::TestValidateTree_ValidConfig moved. Re-judged 2026-10-12, second pass (independent auditor) after 60a74fc04b, 600996fb77, 975428e1b3, 3f1bec74f0 and 84382339ce. SHIFTED only: the tagged units are byte-identical (gate-verified) and validateUnion is unchanged. negative: "not-an-ip" matching no member refused with ErrTypeType; positive: values matching only the first, only the second and only the enum member each accepted, so a union check trying one member alone goes red. Records on validateUnion for all three units. Re-run green 2026-10-12. Verdict stands.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateTree_UnionViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L367) | unit/verify | revert, verified |
| positive | [`TestRFC7950UnionAcceptsEveryMemberType`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_union_test.go#L32) | unit/verify | revert, verified |
| positive | [`TestValidateTree_ValidConfig`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L47) | unit/verify | revert, verified |

### [`RFC7950-7.6.5-1`](#rfc7950-7.6.5-1)

If "mandatory" is "true", the behavior of the constraint depends on the type of the leaf's closest ancestor node in the schema tree that is not a non-presence container (see Section 7.5.1): o If no such ancestor exists in the schema tree, the leaf MUST exist. (§7.6.5)

Scoped tag-claim records only: tested "the leaf MUST exist"; gap "If no such ancestor exists in the schema tree"; internal/component/config/validate_sections.go::ValidateCustomSections is the boundary: Tested covers a mandatory leaf whose ancestors are all non-presence containers, required whenever its top-level section is present in the config (bgp/router-id, bgp/session/asn/local, once the bgp section holds a peer or a group); Gap is the same leaf when the whole top-level section is absent, which Ze does not require, because its top-level sections are non-presence containers that stand for an enabled feature and requiring their leaves would refuse every config that omits bgp; owner-approved deviation, OWNER RULING 6 of 2026-10-12 (Thomas, "Like FRR/libyang (Recommended)"): mandatory leaves are checked only in sections present in the config, as libyang's LYD_VALIDATE_PRESENT ("Validate only modules whose data actually exist") does when FRR sets it in nb_candidate_validate_yang (lib/northbound.c) and yang_parse_data (lib/yang.c); Gap also holds bgp/router-id and bgp/session/asn/local absent at the bgp level while every BGP session carries the value itself or from its group, which bgpMandatoryFailures counts as present through EverySessionCarries: a second owner-approved deviation, ruling of 2026-10-12 (Thomas, "yes enforce", spec AC-2); that ruling binds only once a session exists: a bgp section with no peer and no group is accepted and does nothing, so EverySessionCarries counts both leaves as present there (Thomas, 2026-10-16, spec AC-2 RULING 7); a deviation is never counted conformant; plan/release/fixes/spec-config-yang-when-unique-choice.md; zero whole-requirement credit.

Audit verdict: partial (the tests enforce only the declared tested scope; the whole requirement remains unmet), stale-requirement. Independently rejudged 2026-10-22; no execution or stamping. RFC 7950 Section 7.6.5: "If "mandatory" is "true", the behavior of the constraint depends on the type of the leaf's closest ancestor node in the schema tree that is not a non-presence container (see Section 7.5.1): o If no such ancestor exists in the schema tree, the leaf MUST exist." Tested "the leaf MUST exist"; Gap "If no such ancestor exists in the schema tree"; Producer internal/component/config/validate_sections.go::ValidateCustomSections. Read all four tagged units and both polarities where present: config/rfc7950_mandatory_bgp_test.go::TestRFC7950MandatoryRefusesBGPConfig, config/rfc7950_validator_yang_test.go::TestValidateTree_MandatoryMissing and TestValidator_MandatoryField, config/yang/rfc7950_mandatory_test.go::TestRFC7950MandatoryWithNoQualifyingAncestor (under internal/component). The direct validator requires exact missing path top/mid/need under an absent non-presence ancestor, the corrective set command, and no missing errors when supplied. Config carriers require LoadConfig refusal of missing local ASN or router-id with the expected path and validation category, then accept the corresponding complete config with an actual peer. The patch prepends validateOriginalTree to ValidateCustomSections. Its registered bgp-filter-policy validator reads original bgp/group/peer import/export directions and requires an explicit final on a present direction; these mandatory-leaf fixtures contain no policy direction, so that validator returns normally and does not confound the missing-leaf assertions. The mandatory producer still calls walkTree/requireMandatoryChildren/requireMandatoryUnderAbsent; bgpMandatoryFailures retains only missing errors and EverySessionCarries implements the declared deviations. The whole top-level absent section remains unwalked, inherited session values still excuse global router-id/local ASN, and a BGP section without session sources still excuses those two leaves. Preserve every declared Tested/Gap restriction and the owner-approved deviations; none becomes RFC conformance and this row retains zero whole-requirement credit. Supplied discrimination metadata is not a new runtime observation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950MandatoryRefusesBGPConfig`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_mandatory_bgp_test.go#L30) | unit/verify | revert, verified |
| negative | [`TestValidateTree_MandatoryMissing`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L339) | unit/verify | revert, verified |
| negative | [`TestValidator_MandatoryField`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L944) | unit/verify | revert, verified |
| negative | [`TestRFC7950MandatoryWithNoQualifyingAncestor`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_mandatory_test.go#L69) | unit/verify | revert, verified |
| positive | [`TestRFC7950MandatoryRefusesBGPConfig`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_mandatory_bgp_test.go#L31) | unit/verify | revert, verified |
| positive | [`TestValidator_MandatoryField`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L943) | unit/verify | revert, verified |
| positive | [`TestRFC7950MandatoryWithNoQualifyingAncestor`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_mandatory_test.go#L70) | unit/verify | revert, verified |

### [`RFC7950-7.6.5-2`](#rfc7950-7.6.5-2)

Otherwise, if this ancestor is a case node, the leaf MUST exist if any node from the case exists in the data tree. (§7.6.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-20 (independent auditor, after 4bf9935e38 and 29419d46cf). Read at rfc/full/rfc7950.txt Section 7.6.5: "Otherwise, if this ancestor is a case node, the leaf MUST exist if any node from the case exists in the data tree." The row's {gap} is gone and the code now does it. walkTree resolves each data key through schemaChild, which descends through choice and case entries, so case data is type-checked and walked; requireMandatoryChildren handles a choice child by requireMandatoryInPresentCases, which runs requireMandatoryChildren over every case for which holdsData finds a node in the map holding the choice, and skips the rest. Shapes checked by probe (go test -overlay, nothing written to the tree): a mandatory leaf directly in a case; below non-presence containers a present case holds (walked through schemaChild, then requireMandatoryUnderAbsent); a non-presence container ABSENT inside a case that has another node present is still required (closest qualifying ancestor is the case); a presence container inside a case owes its leaf only when present; a case inside a nested choice (inner choice handled by the IsChoice branch of requireMandatoryChildren on the outer case); an implicit case: goyang's FixChoice wraps each shorthand node in a CaseEntry, so a shorthand container's mandatory leaf is checked against the container's own map and reported as wrap/shc/m, with no false error when supplied. goyang refuses a choice as shorthand of a choice, so that shape cannot be loaded. Negative TestRFC7950MandatoryRequiredWhenCaseExists asserts the exact missing path, the set command and that it is the only missing path for each of the three placements; positive TestRFC7950MandatoryNotRequiredForAbsentCase asserts zero errors with no case present and with each case complete. The .ci drives ze config validate over a gre tunnel with local ip and no remote ip and requires exit 1, the path interface/tunnel[gre0]/encapsulation/gre/remote/ip and the set command. Question 2 judged by my own mutants, not by the records: (A) requireMandatoryInPresentCases checks every case regardless of holdsData: positive goes red in all four subtests and the negative too; (B) it checks no case: negative goes red (leaf in case, leaf in nested case); (C) walkTree reverted to the pre-fix entry.Dir lookup: negative goes red (leaf below containers, empty container), and ze config validate over the .ci's config prints 'configuration valid' and exits 0, so the .ci discriminates the defect it names. The recorded revert breaks (panic in requireMandatoryInPresentCases; panic in schemaChild, which crashes ze at schema load) prove only that the units reach those producers; they are coarse, but they do not weaken this verdict because the mutants above show both polarities fail on the actual non-compliance in each direction. The .ci record is worth re-recording on the mutant route (for example the entry.Dir lookup in walkTree), since a crash at schema load turns every .ci red. Population: the case-scoped mandatory leaves in shipped modules are iface tunnel remote/ip (tunnel-v4/v6-endpoints) and vxlan vni; static and bfd hold mandatory choices (Section 7.9.4, not this row). One reading to note: an empty non-presence container written in a case (gre { }) counts as the case existing, which is the strict reading and fails closed. All tagged units green at HEAD; ./le rfc discriminate id RFC7950-7.6.5-2 lists three records, none stale or unproven.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950MandatoryRequiredWhenCaseExists`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_mandatory_case_test.go#L69) | unit/verify | revert, verified |
| negative | [`iface-tunnel-invalid-gre-no-remote.ci`](https://github.com/ze-software/ze/blob/main/test/parse/iface-tunnel-invalid-gre-no-remote.ci#L16) | functional/verify | mutant, verified |
| positive | [`TestRFC7950MandatoryNotRequiredForAbsentCase`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_mandatory_case_test.go#L111) | unit/verify | revert, verified |

### [`RFC7950-7.6.5-3`](#rfc7950-7.6.5-3)

Otherwise, the leaf MUST exist if the ancestor node exists in the data tree. (§7.6.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-20 (independent auditor, after 4bf9935e38). Section 7.6.5: "Otherwise, the leaf MUST exist if the ancestor node exists in the data tree." Tagged units unchanged; walkTree's fingerprint changed because the per-level scan moved into requireMandatoryChildren with the same checks for non-choice children (see RFC7950-7.6.5-1), and walkTree now finds data keys through schemaChild. TestRFC7950MandatoryOutsideValidatedSections (bfd auth presence container, both exact paths, from mandatoryFailures) and TestRFC7950MandatoryRequiredWhenAncestorExists (list entry and presence container) are green at HEAD, and ./le rfc discriminate id RFC7950-7.6.5-3 lists four records, none stale. The latent exclusion the previous verdict recorded, that walkTree did not descend into case data so a list or presence container inside a case was never checked, is closed: probe shows a presence container present inside a case with its mandatory leaf absent reported at its path, and the same container absent owing nothing. The revert-route records prove reachability only; the verdict rests on the exact-path negatives and the supplied-leaf positives, which an over-eager or skipped check would turn red. Verdict stands.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950MandatoryOutsideValidatedSections`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_mandatory_sections_test.go#L33) | unit/verify | revert, verified |
| negative | [`TestRFC7950MandatoryRequiredWhenAncestorExists`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_mandatory_test.go#L104) | unit/verify | revert, verified |
| positive | [`TestRFC7950MandatoryOutsideValidatedSections`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_mandatory_sections_test.go#L34) | unit/verify | revert, verified |
| positive | [`TestRFC7950MandatoryRequiredWhenAncestorExists`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_mandatory_test.go#L105) | unit/verify | revert, verified |

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-09 after 180df07922 (Resolve returns *yang.Resolved, one Resolve per Loader; process, checkExtensions, checkPatterns, checkStructure still all run and join, refusals unchanged): tagged units changed only in the 64-char tail of ModuleLoadAccepted (schema.GetEntry from the Resolved); cases unchanged, package green. RFC 6.1.3: 'The backslash MUST NOT be followed by any other character.' Prior judgement, still true: Re-judged 2026-10-09; the case edited in ModuleLoadRefused is another row's. Negative: case "bad escape" (description "bad \q escape", the only fault) requires an error containing "escape". Positive: case "valid escapes" loads \t \n \" \\ in a double-quoted string. Probe 2026-10-09: a single-quoted 'bad \q kept', where backslash is not special, loads, so the refusal is scoped to double quotes as Section 6.1.3 scopes it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950ModuleLoadRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L66) | unit/verify | revert, verified |
| positive | [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L116) | unit/verify | revert, verified |

### [`RFC7950-6.2-1`](#rfc7950-6.2-1)

Implementations MUST support identifiers up to 64 characters in length and MAY support longer identifiers. (§6.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-09 after 180df07922 (Resolve returns *yang.Resolved, one Resolve per Loader; process, checkExtensions, checkPatterns, checkStructure still all run and join, refusals unchanged): the 64-character leaf is now read from schema.GetEntry of the returned Resolved, same assertions. RFC 6.2: 'Implementations MUST support identifiers up to 64 characters in length and MAY support longer identifiers.' Prior judgement, still true: {single-polarity} on the row. (a) forbidden: refusing or truncating a 64-character identifier. (b) TestRFC7950ModuleLoadAccepted loads a module whose leaf name is 64 characters (require.NoError on AddModuleFromText and Resolve) and assert.NotNil(entry.Dir[long]), which goes red if the name were refused or shortened.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L118) | unit/verify | revert, verified |

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-09 after 180df07922 (Resolve returns *yang.Resolved, one Resolve per Loader; process, checkExtensions, checkPatterns, checkStructure still all run and join, refusals unchanged): cases unchanged. RFC 6.5: 'References to identifiers defined in external modules MUST be qualified with appropriate prefixes'. Prior judgement, still true: Re-judged 2026-10-09; unchanged cases. Clause 1: ModuleLoadRefused case "external typedef without prefix" (type port with ext imported) requires "unknown type", against Accepted case "prefixed external and bare local" (ext:port and bare local l load). Clause 2 (MAY): Accepted case "extension with own and import prefix" loads m:e under the module's own prefix. The submodule half of the MAY clause is not driven; a MAY imposes no refusal.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950ModuleLoadRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L69) | unit/verify | revert, verified |
| positive | [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L123) | unit/verify | revert, verified |

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
| negative | [`TestRFC7950DerivedTypeMustReplaceAnInvalidatedDefault`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_clauses_test.go#L60) | unit/verify | revert, verified |
| negative | [`TestRFC7950DefaultInvalidForType`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_test.go#L48) | unit/verify | revert, verified |
| positive | [`TestRFC7950DerivedTypeInheritsOrReplacesTheDefault`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_clauses_test.go#L41) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC7950DefaultValidForType`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_test.go#L33) | unit/verify | revert, verified |

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-20 (independent judge, after 7f34543a08). Section 7.6.1: "Otherwise, if this ancestor is a case node, the default value MUST be used if any node from the case exists in the data tree or the case node is the choice's default case, and if no nodes from any other case exist in the data tree." Producer: ApplyDefaults now asks choiceMembers.skipsDefaults before each child; yangToContainer and yangToList record the goyang entry of every child a choice holds (cases.record in both the container and the list choice branch), mergeContainer and mergeNode carry it (adopt), and skipsDefaults calls yang.CaseInEffect for leaves and non-presence containers, leaving presence containers and lists to their own existing-ancestor rule. CaseInEffect climbs every choice between the node and its data parent: in effect when exactly one case holds data and it is this one, or none does and this is the default case; two cases holding data put no case in effect. Positive TestRFC7950CaseDefaultsFollowTheCaseInEffect asserts exact defaults for the case holding data (a2, c2, inside a list entry) and the default case (b1 and the non-presence container bc/bd) with no data; negative TestRFC7950CaseDefaultsIgnoredOutsideTheCaseInEffect asserts exact maps: no other case's default, no non-default case without data, nothing when two cases hold data, nothing for a choice with no default case, list entry, merged container. Mutant records on skipsDefaults (data->nil; inverted predicate) verify. Judge probes via -overlay, all RFC-correct: nested choice in the default case (no data -> outer default case leaf and inner default case x; inner non-default case data -> its default plus the outer case's default; data in another outer case -> only that case's defaults; data in both outer cases -> none); default leaf in a non-presence container of a non-default case (not created with no data, created when a sibling of that case holds data, filled when the container itself is present); shorthand cases; nested choice inside a case container. Scope: no shipped ApplyDefaults caller defaults a node that holds a choice today (bgp peer, rib distance, filter-modify, the tunnel case container), so the proof is at ApplyDefaults over yangToNode, the converter the config schema uses; the reference walk reads the same CaseInEffect. go test ./internal/component/config/ ./internal/component/config/yang/ green at HEAD.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950CaseDefaultsIgnoredOutsideTheCaseInEffect`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_case_defaults_test.go#L104) | unit/verify | mutant, verified |
| positive | [`TestRFC7950CaseDefaultsFollowTheCaseInEffect`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_case_defaults_test.go#L62) | unit/verify | mutant, verified |

### [`RFC7950-7.6.1-3`](#rfc7950-7.6.1-3)

Otherwise, the default value MUST be used if the ancestor node exists in the data tree. (§7.6.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. §7.6.1 third branch, both polarities on one isolated unit (own schema build and map per subtest). Positive: an existing presence container and an existing list entry each receive their child leaf's default through ApplyDefaults. Negative: with both ancestors absent ApplyDefaults creates neither and the map stays empty. Records: positive on applyContainerDefault (revert); negative on the targeted mutant 'c.Presence' -> 'false' (a presence container treated as non-presence), observed red on the ancestor-absent subtest, so the negative discriminates the rule rather than a panic.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950LeafDefaultFollowsItsAncestor`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_leaf_default_test.go#L70) | unit/verify | mutant, verified |
| positive | [`TestRFC7950LeafDefaultFollowsItsAncestor`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_leaf_default_test.go#L69) | unit/verify | revert, verified |

### [`RFC7950-7.6.1-4`](#rfc7950-7.6.1-4)

In these cases, the default value is said to be in use. Note that if the leaf or any of its ancestors has a "when" condition or "if-feature" expression that evaluates to "false", then the default value is not in use. When the default value is in use, the server MUST operationally behave as if the leaf was present in the data tree with the default value as its value. (§7.6.1)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Re-judged 2026-10-20 (independent judge, after 7f34543a08). Section 7.6.1: "Note that if the leaf or any of its ancestors has a \"when\" condition or \"if-feature\" expression that evaluates to \"false\", then the default value is not in use." ApplyDefaults changed only to skip children whose case is not in effect (choiceMembers.skipsDefaults -> yang.CaseInEffect); it still evaluates no when and no if-feature, so a default under a false when stays in use. The {gap} annotation remains accurate. Owned by plan/pre-release/spec-config-yang-when-unique-choice.md.

No test carries RFC7950-7.6.1-4, so no unit is bound to it.

### [`RFC7950-7.6.3-1`](#rfc7950-7.6.3-1)

The "type" statement, which MUST be present, takes as an argument the name of an existing built-in or derived type. (Section 7.6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-09 after 180df07922 (Resolve returns *yang.Resolved, one Resolve per Loader; process, checkExtensions, checkPatterns, checkStructure still all run and join, refusals unchanged): cases unchanged. RFC 7.6.3: 'The "type" statement, which MUST be present, takes as an argument the name of an existing built-in or derived type.' The revert records on Resolve and AddModuleFromText for this row no longer verify (producer/unit changed) and are owed a re-record. Prior judgement, still true: Re-judged 2026-10-09; unchanged cases. MUST be present: ModuleLoadRefused case "leaf without type" refused / Accepted "leaf with type" loads. Names an existing type: TestRFC7950LeafTypeMustNameAnExistingType refuses type nosuch as unknown type against local typedef and imported ext:port twins that differ in the type name alone, with revert records on Resolve.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950LeafTypeMustNameAnExistingType`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L38) | unit/verify | revert, verified |
| negative | [`TestRFC7950ModuleLoadRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L75) | unit/verify | revert, verified |
| positive | [`TestRFC7950LeafTypeMustNameAnExistingType`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L39) | unit/verify | revert, verified |
| positive | [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L130) | unit/verify | revert, verified |

### [`RFC7950-7.6.4-2`](#rfc7950-7.6.4-2)

The value of the "default" statement MUST be valid according to the type specified in the leaf's "type" statement. The "default" statement MUST NOT be present on nodes where "mandatory" is "true". The definition of the default value MUST NOT be marked with an "if-feature" statement. (§7.6.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. all three sentences, both polarities: (1) default valid for the type and (2) no default beside mandatory true, by TestRFC7950DefaultInvalidForType/ValidForType on validateLeafDefaults; (3) if-feature on the default's definition, for an enum named directly (TestRFC7950DefaultNotAnIfFeatureEnum) and through a union member enumeration, a bits type and a typedef chain (TestRFC7950DefaultNotAnIfFeatureMemberBitOrTypedef: x / 'b1 b2' / x refused naming the if-feature, y / 'b1' / y load). The union branch was a D-8 defect fixed here: validateDefaultNotIfFeature now follows the first member type that accepts the default (Section 9.12 order); red observed before the fix. Records on validateDefaultNotIfFeature and validateLeafDefaults.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950DefaultNotAnIfFeatureEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_clauses_test.go#L75) | unit/verify | revert, verified |
| negative | [`TestRFC7950DefaultNotAnIfFeatureMemberBitOrTypedef`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_clauses_test.go#L93) | unit/verify | revert, verified |
| negative | [`TestRFC7950DefaultInvalidForType`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_test.go#L49) | unit/verify | revert, verified |
| positive | [`TestRFC7950DefaultNotAnIfFeatureEnum`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_clauses_test.go#L76) | unit/verify | revert, verified |
| positive | [`TestRFC7950DefaultNotAnIfFeatureMemberBitOrTypedef`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_clauses_test.go#L94) | unit/verify | revert, verified |
| positive | [`TestRFC7950DefaultValidForType`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_test.go#L34) | unit/verify | revert, verified |

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
| negative | [`TestRFC7950DefaultInvalidForType`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_test.go#L50) | unit/verify | revert, verified |
| positive | [`TestRFC7950DefaultValidForType`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_defaults_test.go#L35) | unit/verify | revert, verified |

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

Scoped tag-claim records only: tested "at least one node from"; gap "exactly one of the choice's case branches"; internal/component/config/yang/validator_mandatory.go::mandatoryPresent is the boundary: Tested covers a mandatory choice reported missing when no data node of any of its cases exists, and satisfied by a node of one case, nested choices included; Gap is data holding nodes from two cases of one choice, which mandatoryPresent accepts at the first case node it finds, and no other check compares the cases; plan/release/fixes/spec-config-yang-when-unique-choice.md; zero whole-requirement credit.

Audit verdict: partial (the tests enforce only the declared tested scope; the whole requirement remains unmet), stale-requirement. Judged 2026-10-12 (independent auditor). Section 7.9.4: "If \"mandatory\" is \"true\", at least one node from exactly one of the choice's case branches MUST exist." Tested "at least one node from"; Gap "exactly one of the choice's case branches"; Producer internal/component/config/yang/validator_mandatory.go::mandatoryPresent. Tested holds: TestMandatoryChoiceSatisfiedByACaseNode reports wrap/kind missing for a container with no node of any case (exact path), and accepts a node of one case and a node of a nested choice's case, so a lookup by the choice name alone, or one that ignores nested choices, goes red; records on mandatoryPresent (positive) and missingMandatory (negative). Real entry: walkTree's mandatory loop reaches it from ValidateCustomSections, so the static route choice action and the iface tunnel encapsulation choice kind are refused when empty. Gap holds: data with nodes from two cases is accepted at the first case node found and no other check compares the cases, so the exactly-one half is unmet; the conditional enforcement under a case or list ancestor in the following paragraph is a separate sentence. Whole requirement earns no credit; spec-config-yang-when-unique-choice.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMandatoryChoiceSatisfiedByACaseNode`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_mandatory_test.go#L150) | unit/verify | revert, verified |
| positive | [`TestMandatoryChoiceSatisfiedByACaseNode`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_mandatory_test.go#L151) | unit/verify | revert, verified |

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-09 after 180df07922 (Resolve returns *yang.Resolved, one Resolve per Loader; process, checkExtensions, checkPatterns, checkStructure still all run and join, refusals unchanged): only the ModuleLoadAccepted tail changed. RFC 7.19: 'the substatements MUST be YANG statements, including extensions defined using "extension" statements.' The earlier finding that sources was keyed by file name no longer holds at HEAD: sourcedModules.texts is keyed by *yang.Module (Parse binds each module added by a parse to its text). Revert records on AddModuleFromText for the ModuleLoadAccepted/Refused tags are owed a re-record. Prior judgement, still true: Re-judged 2026-10-09 after 6c06cb43af/037cc25e29. Sentence 1: TestRFC7950ExtensionUsageSubstatementsAreYANGStatements refuses m:e "x" { bogus "y"; } against description and nested m:e twins. Sentence 2 (Section 14 rules) now whole: substatements are resolved against the embedded RFC ABNF (resolveProduction) and checked for argument grammar (config maybe, namespace not a URI, path a / ../a[), cardinality (two type, two description; one type + two must loads), required substatements and blocks (leaf without type, list without data-def, refine with no block) and context forms (deviate add/not-supported mixing, type-body alternatives, augment absolute vs descendant), each tagged test with a revert record on its producer. Independent probe 2026-10-09 of ~70 valid unusual forms under a usage (quoted/unquoted/single-quoted args, + concatenation, nested usages at depth, leafref require-instance, multiple leaf-list defaults, deviate add/replace/delete in full, if-feature expressions with extra whitespace, path predicates with WSP, IPv6/percent URIs, refine across comments/newlines/tabs/UTF-8/CRLF, nested module/submodule) all load; config TRUE, max-elements 0/007, fraction-digits 19, deref() path, refine x; via comment or concatenation, empty input/augment are refused. The ModuleLoadRefused/Accepted tags drive the extension DEFINITION (extension e { bogus }), governed by the Section 14 extension-stmt rule, a neighbouring rule; their claims state what the body checks but do not bear on this sentence.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950ExtensionSubstatementCardinality`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_extension_grammar_test.go#L43) | unit/verify | revert, verified |
| negative | [`TestRFC7950ExtensionSubstatementContextForms`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_extension_grammar_test.go#L87) | unit/verify | revert, verified |
| negative | [`TestRFC7950ExtensionSubstatementRequiredStatementsAndBlocks`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_extension_grammar_test.go#L62) | unit/verify | revert, verified |
| negative | [`TestRFC7950ExtensionSubstatementURIAndPathArguments`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_extension_grammar_test.go#L21) | unit/verify | revert, verified |
| negative | [`TestRFC7950ExtensionUsageSubstatementsAreYANGStatements`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_structural_test.go#L35) | unit/verify | revert, verified |
| negative | [`TestRFC7950ModuleLoadRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L82) | unit/verify | revert, verified |
| positive | [`TestRFC7950ExtensionSubstatementCardinality`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_extension_grammar_test.go#L44) | unit/verify | revert, verified |
| positive | [`TestRFC7950ExtensionSubstatementContextForms`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_extension_grammar_test.go#L88) | unit/verify | revert, verified |
| positive | [`TestRFC7950ExtensionSubstatementRequiredStatementsAndBlocks`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_extension_grammar_test.go#L63) | unit/verify | revert, verified |
| positive | [`TestRFC7950ExtensionSubstatementURIAndPathArguments`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_extension_grammar_test.go#L22) | unit/verify | revert, verified |
| positive | [`TestRFC7950ExtensionUsageSubstatementsAreYANGStatements`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_structural_test.go#L36) | unit/verify | revert, verified |
| positive | [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L138) | unit/verify | revert, verified |

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

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Re-judged 2026-10-20 (independent judge, after 4bf9935e38). Section 8.1: "If the constraint is defined on configuration data, it MUST be true in a valid configuration data tree." walkTree now resolves data keys through schemaChild (case data is type-checked and walked) and moves the mandatory scan into requireMandatoryChildren, which also checks cases holding data. It still evaluates no unique (ListNode.Unique is read only by the web UI for display), no choice exclusivity and no when. Judge probe via -overlay: ValidateTree over a choice with data in two cases (alpha and gamma, mandatory leaves supplied) returns zero errors, so a configuration violating Section 8.1's choice constraint is accepted. {gap} stands. Owned by spec-config-yang-when-unique-choice.

No test carries RFC7950-8.1-1, so no unit is bound to it.

### [`RFC7950-8.1-3`](#rfc7950-8.1-3)

All leaf data values MUST match the type constraints for the leaf, including those defined in the type's "range", "length", and "pattern" properties. (§8.1)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/config/rfc7950_validator_yang_test.go::TestValidateTree_LengthViolation, internal/component/config/rfc7950_validator_yang_test.go::TestValidateTree_PatternViolation, internal/component/config/rfc7950_validator_yang_test.go::TestValidateTree_RangeViolation, internal/component/config/rfc7950_validator_yang_test.go::TestValidator_HoldTimeRange, internal/component/config/rfc7950_validator_yang_test.go::TestValidator_HoldTimeRange#2, internal/component/config/rfc7950_validator_yang_test.go::TestValidator_ValidatePattern, internal/component/config/rfc7950_validator_yang_test.go::TestValidator_ValidatePattern#2 moved. Re-judged 2026-10-12, second pass (independent auditor) after 60a74fc04b, 600996fb77, 975428e1b3, 3f1bec74f0 and 84382339ce. SHIFTED only: the tagged units are byte-identical (gate-verified). range: receive-hold-time 2 and port 0 refused ErrTypeRange, 1 and 2 refused and 0, 3, 180, 65535 accepted; pattern: router-id not-an-ip refused ErrTypePattern at errs[0] with session/asn/local supplied so no missing-mandatory error comes first, 192.0.2.1 accepted; length: a 256-char isis hostname refused ErrTypeLength and a within-length one raises no length error. The validateString record on TestValidateTree_PatternViolation is current again (no stale record). Base-type conformance is carried by the Section 9 rows. Re-run green 2026-10-12. Verdict stands.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateTree_LengthViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L418) | unit/verify | revert, verified |
| negative | [`TestValidateTree_PatternViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L317) | unit/verify | revert, verified |
| negative | [`TestValidateTree_RangeViolation`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L176) | unit/verify | revert, verified |
| negative | [`TestValidator_HoldTimeRange`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L901) | unit/verify | revert, verified |
| negative | [`TestValidator_ValidatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L852) | unit/verify | revert, verified |
| positive | [`TestValidator_HoldTimeRange`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L900) | unit/verify | revert, verified |
| positive | [`TestValidator_ValidatePattern`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_validator_yang_test.go#L851) | unit/verify | revert, verified |

### [`RFC7950-8.1-4`](#rfc7950-8.1-4)

All key leafs MUST be present for all list entries. o Nodes MUST be present for at most one case branch in all choices. o There MUST be no nodes tagged with "if-feature" present if the "if-feature" expression evaluates to "false" in the server. o There MUST be no nodes tagged with "when" present if the "when" condition evaluates to "false" in the data tree. The following properties are true in a valid data tree: o All "must" constraints MUST evaluate to "true". o All referential integrity constraints defined via the "path" statement MUST be satisfied. o All "unique" constraints on lists MUST be satisfied. o The "mandatory" constraint is enforced for leafs and choices, unless the node or any of its ancestors has a "when" condition or "if-feature" expression that evaluates to "false". (§8.1)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Re-judged 2026-10-20 (independent judge, after 4bf9935e38). Section 8.1: "Nodes MUST be present for at most one case branch in all choices." walkTree's change (schemaChild descent into case data, requireMandatoryChildren/requireMandatoryInPresentCases) checks each case holding data for its mandatory nodes but never counts the cases: judge probe ValidateTree("wrap", {a-flag, a-need, g-one, g-need}) with two cases of one choice present returns no error. unique constraints are extracted into ListNode.Unique and consumed only by the web UI, never validated; Ze modules declare no when or if-feature, and walkTree evaluates neither. {gap} stands. Owned by spec-config-yang-when-unique-choice.

No test carries RFC7950-8.1-4, so no unit is bound to it.

### [`RFC7950-8.1-2`](#rfc7950-8.1-2)

The running configuration datastore MUST always be valid. (Section 8.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-18 (independent auditor, after 9d4a5e79e2). Section 8.1: "The running configuration datastore MUST always be valid." The earlier finding (only config.LoadConfig was driven; no install route) is answered for the reload route: TestRFC7950ReloadInstallsAValidConfig and TestRFC7950ReloadRefusesAConstraintViolation write the file and drive diskConfigLoaders and doReload, the path every SIGHUP and editor reload notifier reaches, and assert the refused reload returns ErrCustomValidation naming the value while the reactor keeps system/host reload-valid-host. TestRFC7950WebCommitRefusesAConstraintViolation shows the web editor commit writes nothing for the same violation. Startup stays covered by the LoadConfig units. Residual: the SSH session editor (session_editor.go) sets no pre-commit validator, so its commit relies on the reload refusal the reload units prove; no tagged unit drives that editor, and none drives the gNMI/API configValidationHook route.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950ReloadRefusesAConstraintViolation`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc7950_reload_validation_test.go#L94) | unit/verify | revert, verified |
| negative | [`TestRFC7950WebCommitRefusesAConstraintViolation`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc7950_web_commit_validation_test.go#L82) | unit/verify | revert, verified |
| negative | [`TestRFC7950RunningDatastoreRefusesAViolationAfterParse`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_datastore_test.go#L77) | unit/verify | revert, verified |
| positive | [`TestRFC7950ReloadInstallsAValidConfig`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc7950_reload_validation_test.go#L76) | unit/verify | revert, verified |
| positive | [`TestRFC7950RunningDatastoreAcceptsAValidConfig`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_datastore_test.go#L33) | unit/verify | revert, verified |

### [`RFC7950-8.3-1`](#rfc7950-8.3-1)

For configuration data, there are three windows when constraints MUST be enforced: o during parsing of RPC payloads o during processing of the <edit-config> operation o during validation (§8.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-18 (independent auditor, after 9d4a5e79e2). Section 8.3: "For configuration data, there are three windows when constraints MUST be enforced": parsing, processing of <edit-config>, and validation. Parse and whole-tree validation stay proven through LoadConfig (TestRFC7950RunningDatastoreRefusesAViolationAtParse, ...AfterParse). The earlier finding (edit-processing window untested) is answered for the web editor: TestRFC7950WebCommitWritesAValidEdit (SetValue, CommitNow, file carries the edit) and TestRFC7950WebCommitRefusesAConstraintViolation (a value the YANG type accepts and the internal-plugin ze:validate rule refuses: CommitNow errors naming the value and the file is byte-identical), driven through newEditorFactory(ValidateContent), the factory service_web.go installs. Residual: the SSH CLI editor commit, which carries no pre-commit validator and is refused only at reload, and the gNMI/API session commit are not driven by a tagged unit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950WebCommitRefusesAConstraintViolation`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc7950_web_commit_validation_test.go#L81) | unit/verify | revert, verified |
| negative | [`TestRFC7950RunningDatastoreRefusesAViolationAfterParse`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_datastore_test.go#L78) | unit/verify | revert, verified |
| negative | [`TestRFC7950RunningDatastoreRefusesAViolationAtParse`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_datastore_test.go#L53) | unit/verify | revert, verified |
| positive | [`TestRFC7950WebCommitWritesAValidEdit`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc7950_web_commit_validation_test.go#L58) | unit/verify | revert, verified |
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

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Independently rejudged 2026-10-22; no execution or stamping. RFC 7950 Section 8.3.3: "If the datastore is "running" or "startup", these constraints MUST be enforced at the end of the <edit-config> or <copy-config> operation." Section 8.1 includes the complete obligations "Nodes MUST be present for at most one case branch in all choices.", "There MUST be no nodes tagged with "when" present if the "when" condition evaluates to "false" in the data tree.", and "All "unique" constraints on lists MUST be satisfied." This gap row has no tagged tests. Read internal/component/config/loader.go::LoadConfig/refuseInvalidCustomSections, config/validate_sections.go::ValidateCustomSections and referenceFailures, cli/validator.go::sectionErrors and cli/editor_commit.go::validateStagedTree. Load/validation/editor surfaces reach the shared boundary; referenceFailures walks ToPluginMap and the SSH editor retains BGP reference failures. The new original-tree validator registered in bgp/config/register.go and implemented by redistribution.go::validateFilterPolicies/parsePolicyDisposition rejects malformed explicit policy layers, but is not general evaluation of when, list unique or choice exclusivity. config/yang/validator.go::walkTree validates supplied values, cardinality and mandatory nodes; validator_mandatory.go::requireMandatoryInPresentCases visits each present case without rejecting multiple cases. The schema still declares unique mac/address and mac/match on interface ethernet and unique mac/address on veth/bridge in internal/component/iface/yang/ze-iface-conf.yang. Thus the existing unmet general constraints remain unmet and the verdict stays unimplemented. Correct an unsupported current-state claim in the prior note: its 174/200 versus 26/200 overlay observation is historical, not evidence of present nondeterministic default-reference acceptance. Current config/yang/validator_default.go::activeBranch counts present branches and CaseInEffect explicitly returns false when holding > 1, so no conflicting-case default is in effect. That correction does not enforce choice exclusivity itself: it withholds defaults, rather than refusing the conflicting data tree. Preserve the declared gap and do not claim any new test, overlay or conformance credit.

No test carries RFC7950-8.3.3-2, so no unit is bound to it.

### [`RFC7950-9.1-1`](#rfc7950-9.1-1)

Implementations MUST support all lexical representations specified in this document. (§9.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-19 (independent auditor, after 40a03eb762 and db457cb3c2). Section 9.1: "Implementations MUST support all lexical representations specified in this document." Tagged units are unchanged; the cited producers changed. db457cb3c2 splits compound guards in readInstancePredicates and readEqualsQuoted into one guard per operand with identical bodies, no behavior change. 40a03eb762 makes instanceExists a referenceWalk method that compares a leaf-list predicate value with sameValue (canonicalValue of both sides under the leaf-list's type) and selects a list entry through the canonical keyIndex (canonicalKey) instead of the raw map-key address, so a configured value and a default in use now answer the same lexical forms. This resolves the prior judgement's observation that [.='+17'] matched a default 17 but not a configured 17 (Section 9.1: "In particular, any XPath expression evaluations are done using the canonical form if the data type has a canonical form."). The '.' leaf-list-predicate parse, *WSP handling and absent-value refusal that TestRFC7950InstanceIdentifierLeafListPredicate and ...Absent assert are unaffected (string leaf-list: canonicalValue is identity), and the integer, decimal64, binary, bits, leafref and union-default units of the prior judgement stand. All tagged units and TestReferenceCanonicalValueRefusesOther pass at HEAD (go test -tags 'ze_core ze_bgp' over config/yang and config). The canonical comparison is proven by TestRFC7950InstanceIdentifierCanonicalValue under RFC7950-9.13-1, not by a 9.1-1 tag.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950IntegerModuleDefaultMalformedNotation`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_integer_default_test.go#L88) | unit/verify | revert, verified |
| negative | [`TestRFC7950LeafrefLexical`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L146) | unit/verify | revert, verified |
| negative | [`TestRFC7950InstanceIdentifierLeafListPredicateAbsent`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_walk_test.go#L450) | unit/verify | revert, verified |
| negative | [`TestRFC7950Decimal64Refused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_decimal64_test.go#L40) | unit/verify | revert, verified |
| negative | [`TestRFC7950BinaryRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_lexical_test.go#L64) | unit/verify | revert, verified |
| negative | [`TestRFC7950BitsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_lexical_test.go#L90) | unit/verify | revert, verified |
| positive | [`TestRFC7950IntegerModuleDefaultHexAndOctal`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc7950_yang_schema_integer_default_test.go#L39) | unit/verify | revert, verified |
| positive | [`TestRFC7950ReferenceTypesLoad`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L77) | unit/verify | revert, verified |
| positive | [`TestRFC7950InstanceIdentifierLeafListPredicate`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_walk_test.go#L438) | unit/verify | revert, verified |
| positive | [`TestRFC7950UnionDefaultLeafrefMember`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_walk_test.go#L157) | unit/verify | revert, verified |
| positive | [`TestRFC7950Decimal64Accepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_decimal64_test.go#L25) | unit/verify | revert, verified |
| positive | [`TestRFC7950IntegerSignedLexicalForms`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_integer_test.go#L14) | unit/verify | revert, verified |
| positive | [`TestRFC7950BinaryAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_lexical_test.go#L50) | unit/verify | revert, verified |
| positive | [`TestRFC7950BitsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_lexical_test.go#L77) | unit/verify | revert, verified |

### [`RFC7950-9.2.4-2`](#rfc7950-9.2.4-2)

If multiple values or ranges are given, they all MUST be disjoint and MUST be in ascending order. If a range restriction is applied to a type that is already range-restricted, the new restriction MUST be equally limiting or more limiting, i.e., raising the lower bounds, reducing the upper bounds, removing explicit values or ranges, or splitting ranges into multiple ranges with intermediate gaps. Each explicit value and range boundary value given in the range expression MUST match the type being restricted or be one of the special values "min" or "max". (§9.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.2.4-2, so no unit is bound to it.

### [`RFC7950-9.3.1-1`](#rfc7950-9.3.1-1)

A decimal64 value is lexically represented as an optional sign ("+" or "-"), followed by a sequence of decimal digits, optionally followed by a period ('.') as a decimal indicator and a sequence of decimal digits. (§9.3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-18 (independent auditor, after 2e4be94ae3). Read at rfc/full/rfc7950.txt Section 9.3.1: "A decimal64 value is lexically represented as an optional sign ("+" or "-"), followed by a sequence of decimal digits, optionally followed by a period ('.') as a decimal indicator and a sequence of decimal digits." The units changed only from validateYangType to ValidateType, which is the same producer path (validateLeafValue with no leaf). validateDecimal64 tests ^[+-]?\d+(\.\d+)?$ (ASCII digits in Go RE2) before the fraction-digits parse. Positive accepts +1.5, -1.5, -10.5, 1, 0.25, 0.0, 10.50; negative refuses .5, 1., +, -, 1.2.3, abc and the empty string with ErrTypeType and 'expected decimal64', with fraction-digits and range refusals asserted as distinct kinds. Each clause of the sentence has a value that would flip if the regexp lost it; records on validateDecimal64 replay.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950Decimal64Refused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_decimal64_test.go#L41) | unit/verify | revert, verified |
| positive | [`TestRFC7950Decimal64Accepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_validator_decimal64_test.go#L26) | unit/verify | revert, verified |

### [`RFC7950-9.3.4-1`](#rfc7950-9.3.4-1)

The "fraction-digits" statement, which is a substatement to the "type" statement, MUST be present if the type is "decimal64". (Section 9.3.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-09 after 180df07922 (Resolve returns *yang.Resolved, one Resolve per Loader; process, checkExtensions, checkPatterns, checkStructure still all run and join, refusals unchanged): cases unchanged. RFC 9.3.4: 'The "fraction-digits" statement, which is a substatement to the "type" statement, MUST be present if the type is "decimal64".' Prior judgement, still true: Re-judged 2026-10-09; unchanged cases. Negative: ModuleLoadRefused case "decimal64 without fraction-digits" (type decimal64; only fault) requires an error containing "[1..18]". Positive: case "decimal64 with fraction-digits" (2) loads. The twins differ in the fraction-digits statement alone.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950ModuleLoadRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L87) | unit/verify | revert, verified |
| positive | [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L141) | unit/verify | revert, verified |

### [`RFC7950-9.4.4-1`](#rfc7950-9.4.4-1)

Length-restricting values MUST NOT be negative. If multiple values or ranges are given, they all MUST be disjoint and MUST be in ascending order. (§9.4.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-09 after 180df07922 (Resolve returns *yang.Resolved, one Resolve per Loader; process, checkExtensions, checkPatterns, checkStructure still all run and join, refusals unchanged): cases unchanged. RFC 9.4.4: 'If multiple values or ranges are given, they all MUST be disjoint and MUST be in ascending order.' Prior judgement, still true: Re-judged 2026-10-09; the earlier weak (min/max read as 0 and 2^64-1, and a one-part 5..1 tagged as parts out of order) is fixed. Non-negative: "-1..5" refused. Ascending: ModuleLoadRefused now drives "10..20 | 1..5", and TestRFC7950LengthPartsDisjointAndAscending refuses it and overlapping "1..5 | 3..8" (goyang coalesces both, so the refusal is checkStructure) against "0..5 | 10..20". min/max: TestRFC7950LengthMinMaxAreTheRestrictedTypeBounds refuses "min | 2" and "10 | max" over typedef length 2..10 against "min..4 | 6..max" and "min..max"; revert records on both structural tests. Probe 2026-10-09: touching "1..5 | 5..10" and "max | max" refused, adjacent "1..5 | 6..10" and "min | max" load; min/max follow a two-level typedef chain and a typedef with no length of its own; a union member's length is checked.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950LengthMinMaxAreTheRestrictedTypeBounds`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_structural_test.go#L58) | unit/verify | revert, verified |
| negative | [`TestRFC7950LengthPartsDisjointAndAscending`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_structural_test.go#L73) | unit/verify | revert, verified |
| negative | [`TestRFC7950ModuleLoadRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L90) | unit/verify | revert, verified |
| positive | [`TestRFC7950LengthMinMaxAreTheRestrictedTypeBounds`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_structural_test.go#L59) | unit/verify | revert, verified |
| positive | [`TestRFC7950LengthPartsDisjointAndAscending`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_structural_test.go#L74) | unit/verify | revert, verified |
| positive | [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L143) | unit/verify | revert, verified |

### [`RFC7950-9.5.1-1`](#rfc7950-9.5.1-1)

The lexical representation of a boolean value is a string with a value of "true" or "false". These values MUST be in lowercase. (§9.5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-18 (independent auditor, after 2e4be94ae3). Read at rfc/full/rfc7950.txt Section 9.5.1: "The lexical representation of a boolean value is a string with a value of "true" or "false". These values MUST be in lowercase." The units changed only from validateYangType to ValidateType, the same producer path. validateBoolean accepts exactly "true" and "false"; the negative requires ErrTypeType 'expected boolean' for True, FALSE, TRUE, yes and 1, so a case-folding comparison would turn it red. Ze's config spelling enable/disable is rewritten by NormalizeBool (case-sensitive) before the validator, and TestBooleanConfigSpellingRefused pins that the validator itself refuses them.

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

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950EnumRestrictionKeepsTheBaseValue`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_structural_test.go#L13) | unit/verify | revert, verified |
| positive | [`TestRFC7950EnumRestrictionKeepsTheBaseValue`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_structural_test.go#L14) | unit/verify | revert, verified |

### [`RFC7950-9.6.4.2-1`](#rfc7950-9.6.4.2-1)

This integer value MUST be in the range -2147483648 to 2147483647, and it MUST be unique within the enumeration type. If a value is not specified, then one will be automatically assigned. If the "enum" substatement is the first one defined, the assigned value is zero (0); otherwise, the assigned value is one greater than the current highest enum value (i.e., the highest enum value, implicit or explicit, prior to the current "enum" substatement in the parent "type" statement). Note that the presence of an "if-feature" statement in an "enum" statement does not affect the automatically assigned value. If the current highest value is equal to 2147483647, then an enum value MUST be specified for "enum" substatements following the one with the current highest value. When an existing enumeration type is restricted, the "value" statement MUST either have the same value as in the base type or not be present, in which case the value is the same as in the base type. (§9.6.4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Re-judged 2026-10-09 after 180df07922 (only the ModuleLoadAccepted tail changed; cases unchanged) over the goyang fork, now committed in 66e87707f5 (go.mod replace github.com/ze-software/goyang v1.6.4-0.20261009101552-a3cf525c4f98, upstream PR openconfig/goyang#317). The STACK now meets the sentence 'otherwise, the assigned value is one greater than the current highest enum value': vendored EnumType.Set sets last on the first assignment or any higher value (len(e.ToInt) == 0 || value > e.last), so SetNext after an explicit -5 yields -4, and Loader.Resolve (goyang Modules.Process) no longer refuses enumeration { enum p { value -5; } enum q; enum r { value 0; } }. Ze's own producer (assignEnumValues) is unchanged and its tags still discriminate: TestRFC7950EnumImplicitValueFollowsTheHighest pins -4 after -5, 11 after 10 then 3, if-feature counted (2), two typedef levels down, each refused twin differing only in the value; TestRFC7950EnumValueRangeAndUniqueness refuses with ErrEnumValue an implicit value past 2147483647 and an explicit repeat of the implicit -4. Both tags' claims state only what their bodies assert, and the edited comments now say only true things (goyang v1.6.3 history, Ze's own refusals). Still weak: the module that proves the stack fix is exercised only by TestGoyangNumbersAnEnumAfterANegativeValue, which carries no RFC requirement tag, and every tagged positive sidesteps the defect (uniqueness positive uses r value -3; the implicit positives restate q -4 in a restriction goyang builds afresh). Under goyang v1.6.3 every tagged unit stays green (TestGoyangNumbersAnEnumAfterANegativeValue still carries no tag and the uniqueness positive still uses r value -3), so dropping the replace at the next goyang bump (which the go.mod comment plans) reddens no tagged test. Owed to reach enforced: tag that module as a positive of this requirement (or change the uniqueness positive's r to value 0) and record its discrimination against the reverted Set.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950EnumImplicitValueFollowsTheHighest`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_enum_value_test.go#L28) | unit/verify | revert, verified |
| negative | [`TestRFC7950EnumValueRangeAndUniqueness`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_enum_value_test.go#L61) | unit/verify | revert, verified |
| negative | [`TestRFC7950EnumValueWithinInt32`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L106) | unit/verify | revert, verified |
| negative | [`TestRFC7950EnumRestrictionKeepsTheBaseValue`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_structural_test.go#L11) | unit/verify | revert, verified |
| negative | [`TestRFC7950ModuleLoadRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L94) | unit/verify | revert, verified |
| positive | [`TestRFC7950EnumImplicitValueFollowsTheHighest`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_enum_value_test.go#L29) | unit/verify | revert, verified |
| positive | [`TestRFC7950EnumValueRangeAndUniqueness`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_enum_value_test.go#L62) | unit/verify | revert, verified |
| positive | [`TestRFC7950EnumValueWithinInt32`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L107) | unit/verify | revert, verified |
| positive | [`TestRFC7950EnumRestrictionKeepsTheBaseValue`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_structural_test.go#L12) | unit/verify | revert, verified |
| positive | [`TestRFC7950ModuleLoadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_test.go#L147) | unit/verify | revert, verified |

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

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950LeafrefConfigAndLoop`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L190) | unit/verify | revert, verified |
| negative | [`TestRFC7950LeafrefInstanceRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L100) | unit/verify | revert, verified |
| negative | [`TestRFC7950LeafrefDefaultNotInUse`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_walk_test.go#L77) | unit/verify | revert, verified |
| negative | [`TestRFC7950LeafrefUnionLoop`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_walk_test.go#L183) | unit/verify | revert, verified |
| positive | [`TestRFC7950LeafrefPredicateCanonicalValue`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_canonical_test.go#L98) | unit/verify | revert, verified |
| positive | [`TestRFC7950LeafrefValueAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L85) | unit/verify | revert, verified |
| positive | [`TestRFC7950LeafrefDefaultInUse`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_walk_test.go#L64) | unit/verify | revert, verified |

### [`RFC7950-9.9.2-1`](#rfc7950-9.9.2-1)

The "path" statement, which is a substatement to the "type" statement, MUST be present if the type is "leafref". It takes as an argument a string that MUST refer to a leaf or leaf-list node. The syntax for a path argument is a subset of the XPath abbreviated syntax. Predicates are used only for constraining the values for the key nodes for list entries. Each predicate consists of exactly one equality test per key, and multiple adjacent predicates MAY be present if a list has multiple keys. The syntax is formally defined by the rule "path-arg" in Section 14. The predicates are only used when more than one key reference is needed to uniquely identify a leaf instance. This occurs if a list has multiple keys or a reference to a leaf other than the key in a list is needed. In these cases, multiple leafrefs are typically specified, and predicates are used to tie them together. The "path" expression evaluates to a node set consisting of zero, one, or more nodes. If the "require-instance" property is "true", this node set MUST be non-empty. (§9.9.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950LeafrefPathRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L161) | unit/verify | revert, verified |
| positive | [`TestRFC7950LeafrefPathAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L179) | unit/verify | revert, verified |

### [`RFC7950-9.9.3-1`](#rfc7950-9.9.3-1)

If "require-instance" is "true", it means that the instance being referred to MUST exist for the data to be valid. (Section 9.9.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950LeafrefRequireInstanceMissing`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L122) | unit/verify | revert, verified |
| positive | [`TestRFC7950LeafrefRequireInstance`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L112) | unit/verify | revert, verified |

### [`RFC7950-9.10.2-1`](#rfc7950-9.10.2-1)

The "base" statement, which is a substatement to the "type" statement, MUST be present at least once if the type is "identityref". The argument is the name of an identity, as defined by an "identity" statement. If a prefix is present on the identity name, it refers to an identity defined in the module that was imported with that prefix. Otherwise, an identity with the matching name MUST be defined in the current module or an included submodule. (§9.10.2)

Scoped tag-claim records only: tested "If a prefix is present on the identity name, it refers to an identity defined in the module that was imported with that prefix"; gap "MUST be present at least once"; internal/component/config/yang/validator_reference.go::validateIdentityref is the boundary: Tested covers an identityref with one base, prefixed by an imported module's prefix or local, resolved by goyang (which refuses an identityref with no base or a base it cannot resolve), whose value must name an identity derived from it; Gap is a type naming the base statement more than once, because goyang keeps one base statement per identityref type (Type.IdentityBase holds one value), so the values derived from every base are not computed; plan/post-release/features/spec-config-yang-reference-types.md; zero whole-requirement credit.

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950IdentityrefRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L216) | unit/verify | revert, verified |
| positive | [`TestRFC7950IdentityrefAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L205) | unit/verify | revert, verified |

### [`RFC7950-9.10.3-1`](#rfc7950-9.10.3-1)

Otherwise, an identity with the matching name MUST be defined in the current module or one of its submodules. (Section 9.10.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950IdentityrefPrefixRefused`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L134) | unit/verify | revert, verified |
| positive | [`TestRFC7950IdentityrefOtherModule`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L228) | unit/verify | revert, verified |

### [`RFC7950-9.12-2`](#rfc7950-9.12-2)

When the type is "union", the "type" statement (Section 7.4) MUST be present. (Section 9.12)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7950-9.12-2, so no unit is bound to it.

### [`RFC7950-9.13-1`](#rfc7950-9.13-1)

For identifying list entries with keys, each predicate consists of one equality test per key, and each key MUST have a corresponding predicate. If a key is of type "empty", it is represented as a zero-length string (""). If the leaf with the instance-identifier type represents configuration data and the "require-instance" property (Section 9.9.3) is "true", the node it refers to MUST also represent configuration. Such a leaf puts a constraint on valid data. All such leaf nodes MUST reference existing nodes or leaf or leaf-list nodes with their default value in use (see Sections 7.6.1 and 7.7.2) for the data to be valid. (§9.13)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950InstanceIdentifierKeys`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L267) | unit/verify | revert, verified |
| positive | [`TestRFC7950InstanceIdentifierCanonicalValue`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_canonical_test.go#L73) | unit/verify | revert, verified |
| positive | [`TestRFC7950InstanceIdentifierExists`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L278) | unit/verify | revert, verified |
| positive | [`TestRFC7950InstanceIdentifierDefaultInUse`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_walk_test.go#L88) | unit/verify | revert, verified |

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

Audit verdict: unimplemented (no code path enforces the requirement), fresh. {gap} stands, re-judged 2026-10-09 after 180df07922: LoadEmbedded only gained the refuseLoad guard and still parses ze-extensions.yang and ze-types.yang with no comparison against a previous revision, so a reordering of data definition substatements passes. RFC 11: 'In statements that have any data definition statements as substatements, those data definition substatements MUST NOT be reordered.' Recorded under OWNER RULING 4 (2026-09-30), not closed.

No test carries RFC7950-11-3, so no unit is bound to it.

### [`RFC7950-7.2.2-1`](#rfc7950-7.2.2-1)

A submodule MUST only be included by either the module to which it belongs or another submodule that belongs to that module. (§7.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-18 (independent auditor, after bb173975ea). Read at rfc/full/rfc7950.txt Section 7.2.2: "A submodule MUST only be included by either the module to which it belongs or another submodule that belongs to that module." Producer: checkStructure (joined by Resolve) runs moduleIncludeErrors over every loaded module and submodule (sourceModules); it resolves the including unit's owner (its own name, or its belongs-to) and refuses an include whose submodule belongs to another module with ErrIncludeBelongsTo. TestRFC7950SubmoduleIncludedOnlyByItsModule covers both clauses in both polarities: negatives refuse module a including s (belongs-to b) and submodule a-s of a including s, each requiring the text "belongs to module b"; positives load module a including a-t and a-s including sibling a-t. Module b is loaded and includes s in the negatives, so only the belongs-to differs. Independent overlay mutants (tags ze_core ze_bgp), each red on an assertion: owner ignores belongs-to (submodule-sibling positive red), modules skipped (submodule negative red), submodules skipped (module negative red), comparison always passes (both negatives red), comparison always fails (both positives red). Two equivalent-in-test mutants survive: dropping either lookup in includedSubmodule (include.Module, or the SubModules map fallback) leaves the test green because the other still finds the submodule; the redundancy changes no verdict but no test proves the fallback is needed. The recorded revert tuples (panic in moduleIncludeErrors) prove reach only. {gap} removed from the row in bb173975ea; Ze ships no submodule of its own, so the check guards loaded module sets.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950SubmoduleIncludedOnlyByItsModule`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_structural_test.go#L103) | unit/verify | revert, verified |
| positive | [`TestRFC7950SubmoduleIncludedOnlyByItsModule`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_structural_test.go#L104) | unit/verify | revert, verified |

### [`RFC7950-7.3.2-1`](#rfc7950-7.3.2-1)

The "type" statement, which MUST be present, defines the base type from which this type is derived. (§7.3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-09 after 180df07922 (Resolve returns *yang.Resolved, one Resolve per Loader; process, checkExtensions, checkPatterns, checkStructure still all run and join, refusals unchanged): the positive now reads the leaf from schema.GetEntry of the returned Resolved, same assertion (Yuint8). RFC 7.3.2: 'The "type" statement, which MUST be present, defines the base type from which this type is derived.' negative: a typedef with only a description refused; positive: the same typedef with type uint8 loads and the leaf using it resolves to Yuint8 (proves the type defines the base type). Twins differ by the type statement alone, so the loose "type" substring cannot be met by another fault. Records on Resolve. Those Resolve revert records no longer verify (unit-changed) and are owed a re-record.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950TypedefTypeMustBePresent`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L53) | unit/verify | revert, verified |
| positive | [`TestRFC7950TypedefTypeMustBePresent`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L54) | unit/verify | revert, verified |

### [`RFC7950-7.9.2-2`](#rfc7950-7.9.2-2)

The case identifier MUST be unique within a choice. (§7.9.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. negative: two cases named x in one choice refused as duplicate; positive: cases x and y load, and case x in two different choices loads, which pins the scope to one choice. Records on Resolve. (2026-10-09: tagged units byte-identical, re-stamped for a file shift; the Resolve revert records no longer verify since 180df07922 changed Resolve, re-record owed.)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950CaseIdentifierUniqueWithinAChoice`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L77) | unit/verify | revert, verified |
| positive | [`TestRFC7950CaseIdentifierUniqueWithinAChoice`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L78) | unit/verify | revert, verified |

### [`RFC7950-7.21.5-2`](#rfc7950-7.21.5-2)

If the XPath expression references any node that also has associated "when" statements, those "when" expressions MUST be evaluated first. There MUST NOT be any circular dependencies among "when" expressions. (§7.21.5)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Re-judged 2026-10-20 (independent judge, after 4bf9935e38). Section 7.21.5: "There MUST NOT be any circular dependencies among \"when\" expressions." walkTree's change is the case-data descent through schemaChild and the mandatory checks in requireMandatoryChildren; no when expression is parsed, ordered or evaluated anywhere in config/yang, so neither the evaluation order nor the cycle rule is applied. Ze's modules declare no when today. {gap} stands. Owned by spec-config-yang-when-unique-choice.

No test carries RFC7950-7.21.5-2, so no unit is bound to it.

### [`RFC7950-9.1-2`](#rfc7950-9.1-2)

If the data type does not have a canonical form, the format of the value MUST match the data type's lexical representation, but the exact format is implementation dependent. (§9.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-18 (independent auditor, after 2e4be94ae3). Read at rfc/full/rfc7950.txt Section 9.1: "If the data type does not have a canonical form, the format of the value MUST match the data type's lexical representation, but the exact format is implementation dependent." The types are identityref (Section 9.10.4) and instance-identifier (Section 9.13.3). Both prior findings are fixed. (1) Section 9.13.2 "All node names in an instance-identifier value MUST be qualified with explicit namespace prefixes": checkInstanceKeyPrefix now refuses an unprefixed or foreign key prefix; TestRFC7950InstanceIdentifierKeyPrefix (negative) asserts both messages and the accepted rf:name form, and overlay mutants dropping the empty-prefix branch and the entryPrefixed branch each turned it red. (2) Cascade confounding: TestRFC7950IdentityrefFormat now asserts the format check's own message, and TestRFC7950InstanceIdentifierLexical asserts 'not an instance-identifier' for rf:sys and the unprefixed-node message for /sys/mgmt; overlay mutants removing the identifierRefLength check and the leading-slash check turned each red. A probe of further malformed values (rf:1aes, rf:, :aes, trailing space, //rf:sys, an unquoted key value, [0], mismatched and embedded quotes, a space between steps) found each refused with ErrTypeType.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950IdentityrefFormat`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L289) | unit/verify | revert, verified |
| negative | [`TestRFC7950InstanceIdentifierKeyPrefix`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_walk_test.go#L127) | unit/verify | revert, verified |
| positive | [`TestRFC7950InstanceIdentifierLexical`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_reference_test.go#L249) | unit/verify | revert, verified |

### [`RFC7950-9.4.4-2`](#rfc7950-9.4.4-2)

If a length restriction is applied to a type that is already length-restricted, the new restriction MUST be equally limiting or more limiting, i.e., raising the lower bounds, reducing the upper bounds, removing explicit length values or ranges, or splitting ranges into multiple ranges with intermediate gaps. (§9.4.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. negative: widening a typedef's length 2..5 to 1..10 refused naming length; positive: equal 2..5, narrowed 3..4, and split 2..3 | 5..5 load, covering the allowed restriction kinds. Records on Resolve. (2026-10-09: tagged units byte-identical, re-stamped for a file shift; the Resolve revert records no longer verify since 180df07922 changed Resolve, re-record owed.)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7950LengthRestrictionMustNotWiden`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L93) | unit/verify | revert, verified |
| positive | [`TestRFC7950LengthRestrictionMustNotWiden`](https://github.com/ze-software/ze/blob/main/internal/component/config/yang/rfc7950_loader_clauses_test.go#L94) | unit/verify | revert, verified |

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
