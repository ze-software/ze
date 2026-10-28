# RFC 8893 - Resource Public Key Infrastructure (RPKI) Origin Validation for BGP Export

Unsupported. Every requirement this repository extracted from RFC 8893, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 4 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 4 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 4 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 4 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 6 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 4 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 4 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 4 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |
| Linux cannot do this | 0.0% | 0 of 4 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and remains unmet; it stays in every denominator and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 100.0% | 4 of 4 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 shares marked as a part above are the whole of the 4 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Unsupported |
| Enrolment | Enrolled |
| Requirements | 6 |
| Gated MUST-level | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 4 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc8893.md` |
| Requirement shard | `rfc/requirements/rfc8893.md` |
| RFC text | `rfc/full/rfc8893.txt` |

## Enrolment

Enrolled: Four MUST-level requirements, all recorded as `{gap}`. Ze performs RPKI origin validation on received routes only: the rpki plugin validates the origin AS of a route as received (internal/component/bgp/plugins/rpki/rpki.go::handleStructuredUpdate, rpkiOriginASFromASPath) and the reactor applies the decision before export (internal/component/bgp/reactor/forward_validation.go). No export or redistribution policy can read a validation state, and no validation runs on the effective origin AS after egress modification. RFC8893-1-1 and RFC8893-1-2 bind validation after egress policy; RFC8893-3-1 binds the same policy primitives on ingress, redistribution and egress; RFC8893-3-2 binds the effective origin AS on egress. The two SHOULDs of Section 4 are not gated. The walk is recorded in rfc/extraction/rfc8893.json at register rfc2119, all four sites mapped. Enrolled 2026-10-26.

## What the public ledger says

**Status:** Unsupported

**What the ledger says is covered:**

Ze validates the origin AS of every received route and applies the per-state action (reject, log-only, accept) at ingress, under RFC 6811. It does not validate on export.

**What the ledger says remains**

[`RFC8893-1-1`](#rfc8893-1-1), [`RFC8893-1-2`](#rfc8893-1-2), [`RFC8893-3-1`](#rfc8893-3-1) and [`RFC8893-3-2`](#rfc8893-3-2) are gaps. No egress or redistribution policy primitive reads a validation state, and no validation runs against the effective origin AS after egress changes such as private-AS removal, confederation handling or local-as migration. The Section 4 SHOULDs (a selective validation policy after non-validating outbound policies, listing announcements withheld as Invalid) are also absent.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated (including scoped evidence) | 4 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Annotated (including scoped evidence) (4):** [`RFC8893-1-1`](#rfc8893-1-1), [`RFC8893-1-2`](#rfc8893-1-2), [`RFC8893-3-1`](#rfc8893-3-1), [`RFC8893-3-2`](#rfc8893-3-2)

## Attainable ceiling

Mandatory ceiling: 4 of 4 (100.0%). Optional ceiling: 2 of 2 (100.0%). Every RFC shown with measured rows can reach 100% under current classifications. Lowest mandatory ceiling: rfc8893 4 of 4 (100.0%). Lowest optional ceiling: rfc8893 2 of 2 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8893-1-1` | A validating BGP speaker MUST apply Route Origin Validation policy semantics (see Section 2 of [RFC6811] and Section 4 of [RFC8481]) after applying any egress configuration and policy. (§1) | MUST | 1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze validates only at ingress -- internal/component/bgp/plugins/rpki/rpki.go::handleStructuredUpdate validates the received origin, and internal/component/bgp/reactor/filter_ordered.go::runEgressPolicyChain runs no validation after egress policy |
| `RFC8893-1-2` | Any AS_PATH modifications resulting in effective origin AS change MUST be taken into account. (§1) | MUST | 1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no validation runs on the AS_PATH as sent -- internal/component/bgp/plugins/rpki/rpki.go::rpkiOriginASFromASPath reads the received AS_PATH only |
| `RFC8893-3-1` | BGP implementations supporting RPKI-based origin validation MUST provide the same policy configuration primitives for decisions based on the validation state available for use in ingress, redistribution, and egress policies. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the per-state action exists at ingress only (internal/component/bgp/plugins/rpki/rpki_config.go::OriginInvalidAction); no redistribution or export filter reads a validation state |
| `RFC8893-3-2` | When applied to egress policy, validation state MUST be determined using the effective origin AS of the route as it will (or would) be announced to the peer. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** there is no egress validation, so no effective origin AS is computed -- internal/component/bgp/reactor/filter_ordered.go::runEgressPolicyChain |
| `RFC8893-4-1` | It SHOULD be possible to specify a selective origin validation policy to be applied after any existing non-validating outbound policies. (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8893-4-2` | An implementation SHOULD be able to list announcements that were not sent to a peer, e.g., because they were marked Invalid, as long as the router still has them in memory. (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8893-1-1`](#rfc8893-1-1) A validating BGP speaker MUST apply Route Origin Validation policy semantics (see Section 2 of [RFC6811] and Section 4 of [RFC8481]) after applying any egress configuration and policy. (§1) | {gap}, no test | Ze validates only at ingress -- internal/component/bgp/plugins/rpki/rpki.go::handleStructuredUpdate validates the received origin, and internal/component/bgp/reactor/filter_ordered.go::runEgressPolicyChain runs no validation after egress policy |
| [`RFC8893-1-2`](#rfc8893-1-2) Any AS_PATH modifications resulting in effective origin AS change MUST be taken into account. (§1) | {gap}, no test | no validation runs on the AS_PATH as sent -- internal/component/bgp/plugins/rpki/rpki.go::rpkiOriginASFromASPath reads the received AS_PATH only |
| [`RFC8893-3-1`](#rfc8893-3-1) BGP implementations supporting RPKI-based origin validation MUST provide the same policy configuration primitives for decisions based on the validation state available for use in ingress, redistribution, and egress policies. (§3) | {gap}, no test | the per-state action exists at ingress only (internal/component/bgp/plugins/rpki/rpki_config.go::OriginInvalidAction); no redistribution or export filter reads a validation state |
| [`RFC8893-3-2`](#rfc8893-3-2) When applied to egress policy, validation state MUST be determined using the effective origin AS of the route as it will (or would) be announced to the peer. (§3) | {gap}, no test | there is no egress validation, so no effective origin AS is computed -- internal/component/bgp/reactor/filter_ordered.go::runEgressPolicyChain |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8893-1-1`](#rfc8893-1-1)

A validating BGP speaker MUST apply Route Origin Validation policy semantics (see Section 2 of [RFC6811] and Section 4 of [RFC8481]) after applying any egress configuration and policy. (§1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8893-1-1, so no unit is bound to it.

### [`RFC8893-1-2`](#rfc8893-1-2)

Any AS_PATH modifications resulting in effective origin AS change MUST be taken into account. (§1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8893-1-2, so no unit is bound to it.

### [`RFC8893-3-1`](#rfc8893-3-1)

BGP implementations supporting RPKI-based origin validation MUST provide the same policy configuration primitives for decisions based on the validation state available for use in ingress, redistribution, and egress policies. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8893-3-1, so no unit is bound to it.

### [`RFC8893-3-2`](#rfc8893-3-2)

When applied to egress policy, validation state MUST be determined using the effective origin AS of the route as it will (or would) be announced to the peer. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8893-3-2, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude (spec-rfc-profile-declared-membership phase 1) |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc8893.txt |
| Source fingerprint | 8ceba4ba1710413c |
| Record | rfc/extraction/rfc8893.json |
| Mapped sentences | 4 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | Title page, abstract, status, copyright and contents | 0 | skipped (front-matter) | Title page, abstract, status, copyright and contents |
| `1` | not stated | 2 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 2 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | No IANA actions | 0 | skipped (iana) | No IANA actions |
| `7` | References | 0 | skipped (references) | References |
| `7.1` | Normative references | 0 | skipped (references) | Normative references |
| `7.2` | Informative references | 0 | skipped (references) | Informative references |

### Excluded sentences

The walk over RFC 8893 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 8893, so its obligations are stated where they were written.
