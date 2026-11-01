# RFC 8893 - Resource Public Key Infrastructure (RPKI) Origin Validation for BGP Export

RPKI origin validation on export. Every requirement this repository extracted from RFC 8893, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 4 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 4 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 4 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 4 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 0 | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 6 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 4 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 4 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 4 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 4 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 4 | of 4 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 counts marked as a part above are the whole of the 4 gated MUSTs: they add up to 4. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. It counts TAGGED UNITS, not obligations, so this count is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public row | RPKI origin validation on export |
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

**Area:** RPKI origin validation on export

**Proof:** 0 of 4 MUST proven

**Evidence:** -

**Unmet:** RFC8893-1-1, RFC8893-1-2, RFC8893-3-1, RFC8893-3-2


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

## Figures

- **MUST bar**: Ze correctness gap 4
- **Overview bar**: SHOULD not decided 2, Ze correctness gap 4

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 4 |
| MUST rows kept (the MUST bar) | 4 |
| Ze proven | 0 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 4 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 2 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 2 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/rfc8893.json](../../../data/rfc/rfc8893.json).

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
