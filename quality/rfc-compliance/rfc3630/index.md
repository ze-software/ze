# RFC 3630 - Traffic Engineering (TE) Extensions to OSPF Version 2

Experimental. Every requirement this repository extracted from RFC 3630, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

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
| Gated MUSTs | 5 | of 10 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 4 | of 5 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

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
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 10 |
| Gated MUST-level | 5 |
| Obligations that bind Ze | 1 |
| Not applicable, so out of scope | 4 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 2 |
| Tagged units | 2 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc3630.md` |
| Requirement shard | `rfc/requirements/rfc3630.md` |
| RFC text | `rfc/full/rfc3630.txt` |

## Enrolment

Enrolled: OSPF Traffic Engineering (TE) LSA -- the Type 10 area-local Opaque LSA (Opaque type 1). Five MUST-level requirements gated: one tested, four {not-applicable}. RFC3630-1-1 (non-TE-capable nodes flood TE LSAs as ordinary Type-10 area-local Opaque LSAs) is covered by TestRFC3630NonTECapableFloodsTELSAByScope in internal/plugins/ospf/lsdb/rfc3630_te_test.go: positive, the consumer-agnostic LSDB carrier floods a Type-10 opaque type-1 (packet.TEOpaqueType) LSA to a Flood-eligible same-area neighbor purely by area scope with no TE code in the path (producer eligibleInterface default iface.AreaID==area at flooding.go:401, floodExcept at flooding.go:318 whose only opaque gate is the RFC 5250 O-bit at flooding.go:369, never a TE-type check); negative, the same area-local LSA is NOT flooded out an interface in a different area, bounding it exactly like any other Type-10 opaque LSA. RFC3630-6-1..6-4 (top-level and sub-TLV Type ranges 32768-32777 experimental / 32778-65535 reserved pending a Standards Track RFC) are {not-applicable}: ze is an implementation, not the IANA registry nor an RFC author, so it neither allocates nor documents these type ranges. The 2.4.1/2.5.7/3/2.5.4 SHOULDs and MAYs are not gated.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

TE LSA body and sub-TLV support.

**What the ledger says remains:**

Same OSPF experimental status.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the gated population |
| Annotated instead of tested | 4 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **5** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (1):** [`RFC3630-1-1`](#rfc3630-1-1)

**Annotated instead of tested (4):** [`RFC3630-6-1`](#rfc3630-6-1), [`RFC3630-6-2`](#rfc3630-6-2), [`RFC3630-6-3`](#rfc3630-6-3), [`RFC3630-6-4`](#rfc3630-6-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3630-1-1` | Non-TE capable nodes must flood TE LSAs as any other type 10 (area-local scope) Opaque LSAs (§1) -- the ext-1 opaque carrier floods Type 10 by scope regardless of any TE consumer (Ze: spec-ospf-ext-2) | MUST | 1 | **positive:** `unit/verify` [`TestRFC3630NonTECapableFloodsTELSAByScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3630_te_test.go#L23). **negative:** `unit/verify` [`TestRFC3630NonTECapableFloodsTELSAByScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3630_te_test.go#L28) |
| `RFC3630-6-1` | Top-level Types in the range 32768-32777 are for experimental use, must not be mentioned by RFCs (§6) | MUST NOT | 6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is an implementation, not an RFC author; it neither authors RFCs nor mentions the experimental top-level Type range 32768-32777 |
| `RFC3630-6-2` | Top-level Types 32778-65535: before any assignment, there must be a Standards Track RFC specifying IANA Considerations covering the range (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is an implementation, not the IANA registry nor an RFC author; it neither assigns nor documents the reserved top-level Type range 32778-65535 |
| `RFC3630-6-3` | Sub-TLV Types in the range 32768-32777 are for experimental use, must not be mentioned by RFCs (§6) | MUST NOT | 6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is an implementation, not an RFC author; it neither authors RFCs nor mentions the experimental sub-TLV Type range 32768-32777 |
| `RFC3630-6-4` | Sub-TLV Types 32778-65535: before any assignment, there must be a Standards Track RFC specifying IANA Considerations covering the range (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is an implementation, not the IANA registry nor an RFC author; it neither assigns nor documents the reserved sub-TLV Type range 32778-65535 |
| `RFC3630-2.4.1-1` | If a router advertises BGP routes with the BGP next hop attribute set to the BGP router ID, the Router Address should be the same as the BGP router ID (§2.4.1) | SHOULD | 2.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3630-2.5.7-1` | Maximum Reservable Bandwidth should be user-configurable; default value should be the Maximum Bandwidth (§2.5.7) -- Ze: `max-reservable-bandwidth` leaf, defaulting to `max-bandwidth` (applyTELinkAttributes) | SHOULD | 2.5.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC3630-3-1` | Origination of Traffic Engineering LSAs should be rate-limited to at most one every MinLSInterval (§3) -- Ze reuses the carrier's MinLSInterval rate-limit (OriginateSelf); a pull-model unchanged body floods nothing | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3630-2.5.4-1` | An implementation may choose not to send the Remote Interface IP Address sub-TLV for Multi-access links (§2.5.4) -- Ze omits sub-TLV 4 on multi-access links | MAY | 2.5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3630-3-2` | An implementation may set thresholds (e.g., a bandwidth change threshold) that trigger immediate flooding (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC3630-6-1`](#rfc3630-6-1) Top-level Types in the range 32768-32777 are for experimental use, must not be mentioned by RFCs (§6) | no test | no test carries this requirement id; annotated {not-applicable}: ze is an implementation, not an RFC author; it neither authors RFCs nor mentions the experimental top-level Type range 32768-32777 |
| [`RFC3630-6-2`](#rfc3630-6-2) Top-level Types 32778-65535: before any assignment, there must be a Standards Track RFC specifying IANA Considerations covering the range (§6) | no test | no test carries this requirement id; annotated {not-applicable}: ze is an implementation, not the IANA registry nor an RFC author; it neither assigns nor documents the reserved top-level Type range 32778-65535 |
| [`RFC3630-6-3`](#rfc3630-6-3) Sub-TLV Types in the range 32768-32777 are for experimental use, must not be mentioned by RFCs (§6) | no test | no test carries this requirement id; annotated {not-applicable}: ze is an implementation, not an RFC author; it neither authors RFCs nor mentions the experimental sub-TLV Type range 32768-32777 |
| [`RFC3630-6-4`](#rfc3630-6-4) Sub-TLV Types 32778-65535: before any assignment, there must be a Standards Track RFC specifying IANA Considerations covering the range (§6) | no test | no test carries this requirement id; annotated {not-applicable}: ze is an implementation, not the IANA registry nor an RFC author; it neither assigns nor documents the reserved sub-TLV Type range 32778-65535 |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC3630-1-1`](#rfc3630-1-1)

Non-TE capable nodes must flood TE LSAs as any other type 10 (area-local scope) Opaque LSAs (§1) -- the ext-1 opaque carrier floods Type 10 by scope regardless of any TE consumer (Ze: spec-ospf-ext-2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3630NonTECapableFloodsTELSAByScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3630_te_test.go#L28) | unit/verify | unproven |
| positive | [`TestRFC3630NonTECapableFloodsTELSAByScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3630_te_test.go#L23) | unit/verify | unproven |

### [`RFC3630-6-1`](#rfc3630-6-1)

Top-level Types in the range 32768-32777 are for experimental use, must not be mentioned by RFCs (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3630-6-1, so no unit is bound to it.

### [`RFC3630-6-2`](#rfc3630-6-2)

Top-level Types 32778-65535: before any assignment, there must be a Standards Track RFC specifying IANA Considerations covering the range (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3630-6-2, so no unit is bound to it.

### [`RFC3630-6-3`](#rfc3630-6-3)

Sub-TLV Types in the range 32768-32777 are for experimental use, must not be mentioned by RFCs (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3630-6-3, so no unit is bound to it.

### [`RFC3630-6-4`](#rfc3630-6-4)

Sub-TLV Types 32778-65535: before any assignment, there must be a Standards Track RFC specifying IANA Considerations covering the range (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3630-6-4, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 3630, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 3630, so its obligations are stated where they were written.
