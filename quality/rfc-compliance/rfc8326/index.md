# RFC 8326 - Graceful BGP Session Shutdown

No row in the public ledger. Every requirement this repository extracted from RFC 8326, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

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
| Gated MUSTs | 0 | of 3 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 0 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

No card above is a share of a population, so there is nothing to add up.

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
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | No row in the public ledger |
| Enrolment | Not enrolled (blocked) |
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
| Summary | `rfc/short/rfc8326.md` |
| Requirement shard | `rfc/requirements/rfc8326.md` |
| RFC text | this checkout does not carry the RFC's own text |

## Enrolment

Not enrolled (blocked): Graceful BGP Session Shutdown. No source text at rfc/full/rfc8326.txt or rfc/drafts/rfc8326.txt, so check_enrolment refuses the enrolment. Fetch https://www.rfc-editor.org/rfc/rfc8326.txt, then extract.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 8326.

## Coverage

RFC 8326 declares no MUST-level requirement, so the gate counts nothing here.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8326-x-1` | Receiver should set LOCAL_PREF to lowest configured value for routes carrying GRACEFUL_SHUTDOWN community (Key Requirements) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC8326-x-2` | GSHUT community should be propagated to iBGP peers (Key Requirements) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC8326-x-3` | Initiator should strip GSHUT community before advertising to eBGP peers (unless the peer is the target session) (Key Requirements) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 8326 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

RFC 8326 carries no gated, tagged or audited requirement, so there is no proof state to state.

## Extraction sign-off

No extraction sign-off exists for RFC 8326, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 8326, so its obligations are stated where they were written.
