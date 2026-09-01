# RFC 8050 - Multi-Threaded Routing Toolkit (MRT) Routing Information Export Format with BGP Additional Path Extensions

Partial. Every requirement this repository extracted from RFC 8050, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 66.7% | 4 of 6 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 16.7% | 1 of 6 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 6 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 9 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 6 | of 7 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 6 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 16.7% | 1 of 6 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 6 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 7 |
| Gated MUST-level | 6 |
| Obligations that bind Ze | 6 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 9 |
| Tagged units | 9 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc8050.md` |
| Requirement shard | `rfc/requirements/rfc8050.md` |
| RFC text | `rfc/full/rfc8050.txt` |

## Enrolment

Enrolled: MRT Routing Information Export Format with BGP Additional Path Extensions: six MUST-level requirements. Five are met with test tags in internal/mrt: 4.1-1 (the RIB add-path subtype places the 4-octet Path Identifier between Originated Time and Attribute Length), 4.2-1 (RIB_GENERIC add-path keeps the Path Identifier in the NLRI blob), x-3 (BGP4MP add-path preserves the Path Identifier inside the encapsulated message), and x-4 (add-path is distinguished purely by MRT subtype with no capability lookup) carry positive+negative tags; x-2 (the Path Identifier is big-endian) is {single-polarity: positive}. x-1 (the emitted subtype reflects whether add-path is in use) is {gap}: ze selects the subtype from a static operator config toggle rather than each peer's negotiated RFC 7911 capability. Disclosed in the docs/features/rfc-status.md RFC 8050 row.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Add-path MRT subtypes: TABLE_DUMP_V2 RIB_*_ADDPATH (8-11) carry a 4-byte big-endian Path Identifier between Originated Time and Attribute Length
- RIB_GENERIC_ADDPATH (12) keeps the Path Identifier in the raw NLRI blob and does not redefine the RIB Entry
- BGP4MP add-path subtypes (8-11) preserve the Path Identifier inside the encapsulated message's NLRI
- add-path is distinguished purely by MRT subtype. Tests bound per requirement in [`rfc/requirements/rfc8050.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc8050.md).


**What the ledger says remains**

One MUST gap, gated in [`rfc/short/rfc8050.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc8050.md): ze selects the MRT add-path subtype from a static operator config toggle ([`internal/plugins/mrt/dump.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/dump.go) ribSubtype/bgp4mpTypeSubtype, `config.go` AddPath) rather than from each peer's negotiated RFC 7911 Add-Path capability ([`internal/plugins/mrt/component.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/component.go) OnBGPMessage does not consult it), so a single dump cannot represent a mix of add-path and non-add-path peers.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated instead of tested | 2 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **6** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC8050-4.1-1`](#rfc8050-4.1-1), [`RFC8050-4.2-1`](#rfc8050-4.2-1), [`RFC8050-x-3`](#rfc8050-x-3), [`RFC8050-x-4`](#rfc8050-x-4)

**Annotated instead of tested (2):** [`RFC8050-x-1`](#rfc8050-x-1), [`RFC8050-x-2`](#rfc8050-x-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8050-x-1` | A collector that receives add-path BGP sessions must use the add-path subtypes (BGP4MP_MESSAGE_ADDPATH, etc.) when writing MRT records (Compatibility) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze selects the MRT add-path subtype from a static operator config toggle (internal/plugins/mrt/dump.go:206-250, config.go:19) rather than from each peer's negotiated RFC 7911 Add-Path capability (component.go:113 does not consult it), so a single dump cannot represent a mix of add-path and non-add-path peers |
| `RFC8050-4.1-1` | For AFI/SAFI-specific RIB subtypes (8-11), the RIB Entry must include a 4-byte Path Identifier between Originated Time and Attribute Length (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRIBRecordAddPathRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L258). **negative:** `unit/verify` [`TestRIBRecordRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L210) |
| `RFC8050-4.2-1` | For RIB_GENERIC_ADDPATH (subtype 12), RIB Entries must not be redefined; the Path Identifier is in the raw NLRI blob, not in the RIB Entry (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRIBGenericAddPathRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L514). **negative:** `unit/verify` [`TestRIBGenericRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L306) |
| `RFC8050-x-2` | Path Identifier must be 4 bytes in network byte order, per RFC 7911 (Encoding Rules) | MUST | x | **positive:** `unit/verify` [`TestRIBRecordAddPathRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L262). **negative:** no negative test. **{single-polarity}:** the Path ID is written and read big-endian (internal/mrt/encode.go:107, decode.go:295); there is no alternate-endianness path |
| `RFC8050-x-3` | For BGP4MP add-path subtypes, the Path Identifier is inside the encapsulated BGP message's NLRI, not in the MRT header (Encoding Rules) | MUST | x | **positive:** `unit/verify` [`TestBGP4MPMessageRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L361). **negative:** `unit/verify` [`TestBGP4MPStateChangeRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L426) |
| `RFC8050-x-4` | The subtype alone determines whether add-path encoding is present; no capability negotiation or flag bits (Decoding Rules) | MUST | x | **positive:** `unit/verify` [`TestIsAddPathHelpers`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L664). **negative:** `unit/verify` [`TestIsAddPathHelpers`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L673) |
| `RFC8050-x-5` | Non-add-path-aware MRT parsers should skip records with unknown subtype codes per normal MRT parsing (Compatibility) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8050-x-1`](#rfc8050-x-1) A collector that receives add-path BGP sessions must use the add-path subtypes (BGP4MP_MESSAGE_ADDPATH, etc.) when writing MRT records (Compatibility) | {gap}, no test | ze selects the MRT add-path subtype from a static operator config toggle (internal/plugins/mrt/dump.go:206-250, config.go:19) rather than from each peer's negotiated RFC 7911 Add-Path capability (component.go:113 does not consult it), so a single dump cannot represent a mix of add-path and non-add-path peers |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8050-x-1`](#rfc8050-x-1)

A collector that receives add-path BGP sessions must use the add-path subtypes (BGP4MP_MESSAGE_ADDPATH, etc.) when writing MRT records (Compatibility)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8050-x-1, so no unit is bound to it.

### [`RFC8050-4.1-1`](#rfc8050-4.1-1)

For AFI/SAFI-specific RIB subtypes (8-11), the RIB Entry must include a 4-byte Path Identifier between Originated Time and Attribute Length (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRIBRecordRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L210) | unit/verify | unproven |
| positive | [`TestRIBRecordAddPathRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L258) | unit/verify | unproven |

### [`RFC8050-4.2-1`](#rfc8050-4.2-1)

For RIB_GENERIC_ADDPATH (subtype 12), RIB Entries must not be redefined; the Path Identifier is in the raw NLRI blob, not in the RIB Entry (Section 4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRIBGenericRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L306) | unit/verify | unproven |
| positive | [`TestRIBGenericAddPathRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L514) | unit/verify | unproven |

### [`RFC8050-x-2`](#rfc8050-x-2)

Path Identifier must be 4 bytes in network byte order, per RFC 7911 (Encoding Rules)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRIBRecordAddPathRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L262) | unit/verify | unproven |

### [`RFC8050-x-3`](#rfc8050-x-3)

For BGP4MP add-path subtypes, the Path Identifier is inside the encapsulated BGP message's NLRI, not in the MRT header (Encoding Rules)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBGP4MPStateChangeRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L426) | unit/verify | unproven |
| positive | [`TestBGP4MPMessageRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L361) | unit/verify | unproven |

### [`RFC8050-x-4`](#rfc8050-x-4)

The subtype alone determines whether add-path encoding is present; no capability negotiation or flag bits (Decoding Rules)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIsAddPathHelpers`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L673) | unit/verify | unproven |
| positive | [`TestIsAddPathHelpers`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L664) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 8050, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 8050, so its obligations are stated where they were written.
