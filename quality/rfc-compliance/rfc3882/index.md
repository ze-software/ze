# RFC 3882 - Configuring BGP to Block Denial-of-Service Attacks

No row in the public ledger. Every requirement this repository extracted from RFC 3882, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 0 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 0 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 0 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 0 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 0 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 0 | of 0 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 0 | of 0 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (non-normative), so every share below reads what the summary records rather than what the gate enforces |
| Out of scope | 0 | of 0 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 0 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 0 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 0 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

No card above is a share of a population, so there is nothing to add up.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| MUSTs declared | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | No row in the public ledger |
| Enrolment | Not enrolled (non-normative) |
| Requirements | 0 |
| Gated MUST-level | 0 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc3882.md` |
| Requirement shard | no requirement declared, so no shard is generated |
| RFC text | `rfc/full/rfc3882.txt` |

## Enrolment

Not enrolled (non-normative, the document imposes no MUST-level obligation on an implementation, so there is nothing to gate): Configuring BGP to Block Denial-of-Service Attacks, IETF category Informational. A capitalised MUST / MUST NOT / SHALL / SHALL NOT / REQUIRED / SHOULD / MAY scan over rfc/full/rfc3882.txt hits zero keywords, and the document invokes neither RFC 2119 nor BCP 14 anywhere, so it declares no key-words machinery for a reader to read its prose by. It describes an operator configuration recipe: per-border-router BGP communities that rewrite the next hop of an attacked prefix to a null or sinkhole tunnel interface. The four lower-case modal sentences the prose inventory finds are steps of that recipe ("R1 and R2 must be configured with the following", "The advertisement must contain the community value"), a description ("It might also be required"), and copyright boilerplate. The summary captures no requirement. The walk is recorded in rfc/extraction/rfc3882.json at register prose, all 4 sites excluded as not-a-requirement.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 3882.

## Coverage

RFC 3882 declares no MUST-level requirement, so the gate counts nothing here.

## Requirements

RFC 3882 declares no requirement, so this summary generates no shard.

## Gaps and untested MUSTs

RFC 3882 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

RFC 3882 carries no gated, tagged or audited requirement, so there is no proof state to state.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude (spec-rfc-profile-declared-membership phase 1) |
| Signed off | 2026-10-26 |
| Register | prose |
| Source | rfc/full/rfc3882.txt |
| Source fingerprint | 0686f642ef5a26e9 |
| Record | rfc/extraction/rfc3882.json |
| Mapped sentences | 0 |
| Declined as scope | 4 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | Title page, abstract, status, copyright and contents | 0 | skipped (front-matter) | Title page, abstract, status, copyright and contents |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 2 | walked | not stated |
| `3` | not stated | 1 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | Acknowledgments | 0 | skipped (acknowledgements) | Acknowledgments |
| `6` | Informative references | 0 | skipped (references) | Informative references |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 1 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A step of the operator configuration recipe the document describes ('R1 and R2 must be configured with the following'); the document carries no RFC 2119 keyword and binds no implementation | After the BGP community assignment, R1 and R2 must be configured with the following: |
| `2:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A step of the operator configuration recipe: what the trigger announcement carries for the described technique to work; the document carries no RFC 2119 keyword and binds no implementation | The advertisement must contain the community value associated with the router(s) where the attack is arriving in addition to the well-known community (no-export). |
| `3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive: 'It might also be required to allow the traffic to reach its destination' describes an optional sinkhole use, no obligation | It might also be required to allow the traffic to reach its destination after being captured. |
| `8:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Boilerplate: the IETF intellectual property statement in the Full Copyright Statement | The IETF invites any interested party to bring to its attention any copyrights, patents or patent applications, or other proprietary rights that may cover technology that may be required to implement this standard. |

## Superseded

No document obsoletes RFC 3882, so its obligations are stated where they were written.
