# RFC 8571 - BGP - Link State (BGP-LS) Advertisement of IGP Traffic Engineering Performance Metric Extensions

No row in the public ledger. Every requirement this repository extracted from RFC 8571, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 1 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 1 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 1 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 1 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 1 | of 1 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 1 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 100.0% | 1 of 1 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 1 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 1 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 7 shares marked as a part above are the whole of the 1 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
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
| Requirements | 1 |
| Gated MUST-level | 1 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc8571.md` |
| Requirement shard | `rfc/requirements/rfc8571.md` |
| RFC text | `rfc/full/rfc8571.txt` |

## Enrolment

Enrolled: BGP-LS IGP Traffic Engineering Performance Metric extensions: four MUST-level requirements. x-2 (Reserved MUST be ignored on receipt) is {single-polarity: positive} bound to new decoder tests that set reserved bits and assert the meaningful fields decode intact (internal/component/bgp/plugins/nlri/ls/attr_link.go:755-763,804-813,838-845). x-1 (Reserved 0 on transmission), x-3 (TLVs only added to Link NLRIs), x-4 (values follow RFC 8570/7471) are {not-applicable}: ze's BGP-LS codec is decode-only (plugin.go:70-71 Mode "decode", OnDecodeNLRI only) with no origination/encode path.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 8571.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **1** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (1):** [`RFC8571-x-4`](#rfc8571-x-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8571-x-4` | Semantics and values must follow RFC 8570 (IS-IS) and RFC 7471 (OSPF) (MUST Requirements) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never assigns or transmits TE metric values: it only decodes received TLVs (internal/component/bgp/plugins/nlri/ls/attr_link.go:755-763,804-813,838-845) and has no encode path that could source semantics or values from RFC 8570 or RFC 7471 |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8571-x-4`](#rfc8571-x-4) Semantics and values must follow RFC 8570 (IS-IS) and RFC 7471 (OSPF) (MUST Requirements) | no test | no test carries this requirement id; annotated {not-applicable}: ze never assigns or transmits TE metric values: it only decodes received TLVs (internal/component/bgp/plugins/nlri/ls/attr_link.go:755-763,804-813,838-845) and has no encode path that could source semantics or values from RFC 8570 or RFC 7471 |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8571-x-4`](#rfc8571-x-4)

Semantics and values must follow RFC 8570 (IS-IS) and RFC 7471 (OSPF) (MUST Requirements)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8571-x-4, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc8571.txt |
| Source fingerprint | 4418647079d036e7 |
| Record | rfc/extraction/rfc8571.json |
| Mapped sentences | 0 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | walked | not stated |
| `1` | not stated | 1 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `2.3` | not stated | 0 | walked | not stated |
| `2.4` | not stated | 0 | walked | not stated |
| `2.5` | not stated | 0 | walked | not stated |
| `2.6` | not stated | 0 | walked | not stated |
| `2.7` | not stated | 0 | walked | not stated |
| `2.8` | not stated | 0 | walked | not stated |
| `3` | not stated | 1 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
| `5.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust Legal Provisions boilerplate in the copyright notice. It binds whoever extracts code components from the document, not a BGP-LS implementation. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The Introduction states why the document exists: new TLVs are needed to carry metrics RFC 8570 and RFC 7471 already define. It places no obligation on an implementation, and this document carries no RFC 2119 keyword anywhere in its text. | New BGP-LS Link Attribute TLVs are required in order to carry the Traffic Engineering Metric Extensions defined in [RFC8570] and [RFC7471]. |
| `3:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The security and authentication mechanisms this sentence calls required are those of RFC 8570 and RFC 7471, which the sentence cites, and they bind the IGP instance that originates the metrics. RFC 8571 states its own position in the next paragraph: the advertisement presents no additional risk beyond the link attribute information RFC 7752 already carries. | It is assumed that the IGP instances originating these TLVs will support all the required security and authentication mechanisms (as described in [RFC8570] and [RFC7471]) in order to prevent any security issues when propagating the TLVs into BGP-LS. |

## Superseded

No document obsoletes RFC 8571, so its obligations are stated where they were written.
