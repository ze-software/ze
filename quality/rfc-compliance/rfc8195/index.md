# RFC 8195 - Use of BGP Large Communities

No row in the public ledger. Every requirement this repository extracted from RFC 8195, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 0 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 0 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 0 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 0 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 0 | of 0 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 0 | of 3 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (non-normative), so every share below reads what the summary records rather than what the gate enforces |
| Out of scope | 0 | of 0 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

No card above is a share of a population, so there is nothing to add up.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| MUSTs declared | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | No row in the public ledger |
| Enrolment | Not enrolled (non-normative) |
| Requirements | 3 |
| Gated MUST-level | 0 |
| Obligations that bind Ze | 0 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc8195.md` |
| Requirement shard | `rfc/requirements/rfc8195.md` |
| RFC text | `rfc/full/rfc8195.txt` |

## Enrolment

Not enrolled (non-normative, the document imposes no MUST-level obligation on an implementation, so there is nothing to gate): Use of BGP Large Communities, IETF category Informational. A capitalised MUST / MUST NOT / SHALL / SHALL NOT / REQUIRED scan over rfc/full/rfc8195.txt hits zero keywords, and the document invokes neither RFC 2119 nor RFC 8174 nor BCP 14 anywhere, so it declares no key-words machinery for a reader to read its prose by. Its own abstract calls it examples and inspiration for operator application of BGP Large Communities. The summary written against it captures 3 requirements and gates none: RFC8195-2-1, RFC8195-2.2-1 and RFC8195-4.3.3-1, all at SHOULD. Section 3.2 and Section 4.1.1 both say an AS could assign a function number, which states a convention rather than an obligation. Thomas ruled on 2026-08-12 that this is non-normative rather than backlog.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 8195.

## Coverage

RFC 8195 declares no MUST-level requirement, so the gate counts nothing here.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8195-2-1` | Publicly publish and maintain documentation on supported Large Communities (§2, §5) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8195-2.2-1` | Publish the relative order in which Action Communities are processed in routing policy (§2.2) | SHOULD | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8195-4.3.3-1` | Take care with LOCAL_PREF manipulation that crosses preference class boundaries to avoid BGP Wedgies per RFC 4264 (§4.3.3) | SHOULD | 4.3.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 8195 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

RFC 8195 carries no gated, tagged or audited requirement, so there is no proof state to state.

## Extraction sign-off

No extraction sign-off exists for RFC 8195, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 8195, so its obligations are stated where they were written.
