# RFC 6996 - Autonomous System (AS) Reservation for Private Use

No row in the public ledger. Every requirement this repository extracted from RFC 6996, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 1 of 1 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 1 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 1 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 1 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 2 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 1 | of 3 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 1 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 1 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 3 |
| Gated MUST-level | 1 |
| Obligations that bind Ze | 1 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 2 |
| Tagged units | 2 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc6996.md` |
| Requirement shard | `rfc/requirements/rfc6996.md` |
| RFC text | `rfc/full/rfc6996.txt` |

## Enrolment

Enrolled: Private Use ASN removal: the sole MUST (RFC6996-4-1) requires Private Use ASNs be stripped from the AS path before advertisement to the global Internet, which the remove-private-as filter enforces (internal/component/bgp/plugins/filter_remove_private_as). The positive/negative pair pins the strip to exactly the 64512-65534 / 4200000000-4294967294 ranges, so a globally-routable ASN is never removed. The 4-2 SHOULD and 4-3 MAY are operator guidance, not gated.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 6996.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **1** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (1):** [`RFC6996-4-1`](#rfc6996-4-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6996-4-1` | Private Use ASNs MUST be removed from AS path attributes (including AS4_PATH) before being advertised to the global Internet (Section 4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC6996StripsPrivateUseASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_remove_private_as/private_as_test.go#L47). **negative:** `unit/verify` [`TestRFC6996KeepsPublicASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_remove_private_as/private_as_test.go#L56) |
| `RFC6996-4-2` | Operators SHOULD ensure that all EBGP speakers support RFC 6793 extensions and that implementation-specific features recognizing Private Use ASNs have been updated to recognize both ranges prior to using the four-octet Private Use ASN range (Section 4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC6996-4-3` | Normal AS path filtering MAY also be used to prevent prefixes originating from Private Use ASNs from being advertised to the global Internet (Section 4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 6996 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC6996-4-1`](#rfc6996-4-1)

Private Use ASNs MUST be removed from AS path attributes (including AS4_PATH) before being advertised to the global Internet (Section 4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6996KeepsPublicASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_remove_private_as/private_as_test.go#L56) | unit/verify | unproven |
| positive | [`TestRFC6996StripsPrivateUseASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_remove_private_as/private_as_test.go#L47) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 6996, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 6996, so its obligations are stated where they were written.
