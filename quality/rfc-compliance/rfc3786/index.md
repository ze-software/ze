# RFC 3786 - Extending the Number of Intermediate System to Intermediate System (IS-IS) Link State PDU (LSP) Fragments Beyond the 256 Limit

No row in the public ledger. Every requirement this repository extracted from RFC 3786, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

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

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 7 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 4 | of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

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
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 7 |
| Gated MUST-level | 4 |
| Obligations that bind Ze | 0 |
| Not applicable, so out of scope | 4 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc3786.md` |
| Requirement shard | `rfc/requirements/rfc3786.md` |
| RFC text | `rfc/full/rfc3786.txt` |

## Enrolment

Enrolled: Extending the Number of IS-IS LSP Fragments Beyond the 256 Limit: four MUST-level requirements, all {not-applicable} to Ze. Ze does not implement the RFC 3786 extended-LSP-fragment mechanism -- its IS-IS codec recognizes no IS Alias ID TLV (type 24) and originates no extended LSP sets or virtual system (recognized TLV set 1/2/6/8/9/10/22/129/132/135/137/232/236/240 in internal/plugins/isis/packet/tlv.go), running only standard ISO/IEC 10589 LSPs bounded by the 256-fragment limit. RFC3786-2-1 (IS Alias ID TLV in fragment 0) and RFC3786-3.1-1 (generate extended fragment zero) have no extended-LSP origination path; RFC3786-5-1 (SPF exclusion of an expired extended fragment 0 set) governs extended sets Ze does not have (its SPF operates only on standard LSPs); RFC3786-x-2 (Mode 1 Originating-to-Virtual adjacency zero metric) governs a virtual-system model Ze does not implement. No SHOULD/MAY requirements are gated.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 3786.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 4 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (4):** [`RFC3786-2-1`](#rfc3786-2-1), [`RFC3786-3.1-1`](#rfc3786-3.1-1), [`RFC3786-5-1`](#rfc3786-5-1), [`RFC3786-x-2`](#rfc3786-x-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3786-2-1` | IS Alias ID TLV included in fragment 0 of every LSP set in either mode (Section 2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze does not implement the RFC 3786 extended-LSP-fragment mechanism. Its IS-IS codec does not recognize the IS Alias ID TLV (type 24) -- the recognized TLV set is 1/2/6/8/9/10/22/129/132/135/137/232/236/240 (internal/plugins/isis/packet/tlv.go) with no type 24 and no extended-LSP-set origination, so Ze never generates an extended LSP set that would need an IS Alias ID TLV. |
| `RFC3786-3.1-1` | Generate an extended LSP fragment zero for every extended LSP set (Section 3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze generates no extended LSP sets -- it originates only standard IS-IS LSPs (bounded by the ISO/IEC 10589 256-fragment limit) and has no IS Alias ID TLV (type 24) / virtual-system origination path (internal/plugins/isis/packet/tlv.go recognizes no type 24), so there is no extended LSP set for which an extended fragment zero would be generated. |
| `RFC3786-5-1` | Consider any of a system's LSPs in SPF when its Original LSP fragment 0 is missing or has zero RemainingLifetime; for an expired extended fragment 0, exclude only that set (Section 5) | MUST NOT | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** The RFC 3786-specific clause -- "for an expired extended fragment 0, exclude only that set" -- governs extended LSP sets, which Ze does not implement (no IS Alias ID TLV type 24, no extended LSP sets; internal/plugins/isis/packet/tlv.go). Ze's SPF operates only on standard (non-extended) IS-IS LSPs per ISO/IEC 10589, so the extended-set exclusion rule has no applicable code path. |
| `RFC3786-x-1` | Set ATT bits and the Partition Repair bit to zero on all extended LSPs (Sections 3.1.1, 3.1.2) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC3786-3.1.4-1` | Set the overload bit consistently across all original and extended LSPs to reflect the Originating System's overload state (Section 3.1.4) | SHOULD | 3.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3786-x-2` | In Mode 1, metric for Originating-to-Virtual adjacencies is zero and no other neighbors are specified in an Extended LSP (Sections 3.2, 3.2.1) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Mode 1 (the virtual-system model with Originating-to-Virtual adjacencies in Extended LSPs) is part of the RFC 3786 extended-fragment mechanism Ze does not implement -- Ze has no virtual system, no extended LSP, and no IS Alias ID TLV (type 24) origination (internal/plugins/isis/packet/tlv.go), so there is no Originating-to-Virtual adjacency to assign a zero metric to. |
| `RFC3786-7-1` | Provide a config parameter for LSP origination behavior, defaulting to ISO/IEC 10589 behavior with neither mode enabled (Section 7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC3786-2-1`](#rfc3786-2-1) IS Alias ID TLV included in fragment 0 of every LSP set in either mode (Section 2) | no test | no test carries this requirement id; annotated {not-applicable}: Ze does not implement the RFC 3786 extended-LSP-fragment mechanism. Its IS-IS codec does not recognize the IS Alias ID TLV (type 24) -- the recognized TLV set is 1/2/6/8/9/10/22/129/132/135/137/232/236/240 (internal/plugins/isis/packet/tlv.go) with no type 24 and no extended-LSP-set origination, so Ze never generates an extended LSP set that would need an IS Alias ID TLV. |
| [`RFC3786-3.1-1`](#rfc3786-3.1-1) Generate an extended LSP fragment zero for every extended LSP set (Section 3.1) | no test | no test carries this requirement id; annotated {not-applicable}: Ze generates no extended LSP sets -- it originates only standard IS-IS LSPs (bounded by the ISO/IEC 10589 256-fragment limit) and has no IS Alias ID TLV (type 24) / virtual-system origination path (internal/plugins/isis/packet/tlv.go recognizes no type 24), so there is no extended LSP set for which an extended fragment zero would be generated. |
| [`RFC3786-5-1`](#rfc3786-5-1) Consider any of a system's LSPs in SPF when its Original LSP fragment 0 is missing or has zero RemainingLifetime; for an expired extended fragment 0, exclude only that set (Section 5) | no test | no test carries this requirement id; annotated {not-applicable}: The RFC 3786-specific clause -- "for an expired extended fragment 0, exclude only that set" -- governs extended LSP sets, which Ze does not implement (no IS Alias ID TLV type 24, no extended LSP sets; internal/plugins/isis/packet/tlv.go). Ze's SPF operates only on standard (non-extended) IS-IS LSPs per ISO/IEC 10589, so the extended-set exclusion rule has no applicable code path. |
| [`RFC3786-x-2`](#rfc3786-x-2) In Mode 1, metric for Originating-to-Virtual adjacencies is zero and no other neighbors are specified in an Extended LSP (Sections 3.2, 3.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: Mode 1 (the virtual-system model with Originating-to-Virtual adjacencies in Extended LSPs) is part of the RFC 3786 extended-fragment mechanism Ze does not implement -- Ze has no virtual system, no extended LSP, and no IS Alias ID TLV (type 24) origination (internal/plugins/isis/packet/tlv.go), so there is no Originating-to-Virtual adjacency to assign a zero metric to. |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC3786-2-1`](#rfc3786-2-1)

IS Alias ID TLV included in fragment 0 of every LSP set in either mode (Section 2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3786-2-1, so no unit is bound to it.

### [`RFC3786-3.1-1`](#rfc3786-3.1-1)

Generate an extended LSP fragment zero for every extended LSP set (Section 3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3786-3.1-1, so no unit is bound to it.

### [`RFC3786-5-1`](#rfc3786-5-1)

Consider any of a system's LSPs in SPF when its Original LSP fragment 0 is missing or has zero RemainingLifetime; for an expired extended fragment 0, exclude only that set (Section 5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3786-5-1, so no unit is bound to it.

### [`RFC3786-x-2`](#rfc3786-x-2)

In Mode 1, metric for Originating-to-Virtual adjacencies is zero and no other neighbors are specified in an Extended LSP (Sections 3.2, 3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3786-x-2, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 3786, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 3786, so its obligations are stated where they were written.
