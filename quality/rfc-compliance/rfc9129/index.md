# RFC 9129 - YANG Data Model for the OSPF Protocol

No row in the public ledger. Every requirement this repository extracted from RFC 9129, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

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
| Gated MUSTs | 0 | of 7 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
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
| Requirements | 7 |
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
| Summary | `rfc/short/rfc9129.md` |
| Requirement shard | `rfc/requirements/rfc9129.md` |
| RFC text | this checkout does not carry the RFC's own text |

## Enrolment

Not enrolled (blocked): YANG Data Model for the OSPF Protocol. No source text at rfc/full/rfc9129.txt or rfc/drafts/rfc9129.txt, so check_enrolment refuses the enrolment. Fetch https://www.rfc-editor.org/rfc/rfc9129.txt, then extract.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 9129.

## Coverage

RFC 9129 declares no MUST-level requirement, so the gate counts nothing here.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9129-2.5-1` | Instance carries `address-family` / `router-id` semantics equivalent to §2.5 | SHOULD | 2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9129-2.6-1` | Area list keyed by `area-id` with `area-type` covering normal/stub/nssa (§2.6) | SHOULD | 2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9129-2.6-2` | Area `ranges/range` aggregation with `advertise` + `cost`, and stub/NSSA `default-cost` (§2.6) | SHOULD | 2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9129-2.7-1` | Interface carries `interface-type` (broadcast/point-to-point/nbma/point-to-multipoint), `cost`, `hello-interval`, `dead-interval`, `retransmit-interval`, `transmit-delay`, `priority`, `passive`, `mtu-ignore` (§2.7) | SHOULD | 2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9129-2.7-2` | Authentication supports a key-chain reference per RFC 8177 plus explicit key + crypto-algorithm (§2.7) | SHOULD | 2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9129-2.5-2` | Operational state exposes neighbors (with adjacency state), LSDB by scope, and statistics (§2.5, §2.6) | SHOULD | 2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9129-2.9-1` | Provide `clear-neighbor` / `clear-database` RPC equivalents (§2.9) | MAY | 2.9 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 9129 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

RFC 9129 carries no gated, tagged or audited requirement, so there is no proof state to state.

## Extraction sign-off

No extraction sign-off exists for RFC 9129, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 9129, so its obligations are stated where they were written.
