# RFC 8571 - BGP - Link State (BGP-LS) Advertisement of IGP Traffic Engineering Performance Metric Extensions

No row in the public ledger. Every requirement this repository extracted from RFC 8571, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 1 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 100.0% | 1 of 1 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 1 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 1 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 3 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 4 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 3 | of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

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
| Requirements | 4 |
| Gated MUST-level | 4 |
| Obligations that bind Ze | 1 |
| Not applicable, so out of scope | 3 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 3 |
| Tagged units | 3 |
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
| Annotated instead of tested | 4 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (4):** [`RFC8571-x-1`](#rfc8571-x-1), [`RFC8571-x-2`](#rfc8571-x-2), [`RFC8571-x-3`](#rfc8571-x-3), [`RFC8571-x-4`](#rfc8571-x-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8571-x-1` | Reserved fields in all TLVs (1114-1120) must be set to 0 on transmission (Encoding Rules) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no BGP-LS origination or encode path, so there is no encoder to zero the reserved field on transmission: the plugin registers Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70-71) and the TE-metric WriteTo encoders (internal/component/bgp/plugins/nlri/ls/attr_link.go:732,776,824) have no production caller |
| `RFC8571-x-2` | Reserved fields in all TLVs must be ignored on receipt (Decoding Rules) | MUST | x | **positive:** `unit/verify` [`TestRFC8571ReservedIgnoredDelayVariation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc8571_attr_reserved_test.go#L53). **positive:** `unit/verify` [`TestRFC8571ReservedIgnoredMinMaxDelay`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc8571_attr_reserved_test.go#L40). **positive:** `unit/verify` [`TestRFC8571ReservedIgnoredUnidirectionalDelay`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc8571_attr_reserved_test.go#L28). **negative:** no negative test. **{single-polarity}:** ze decodes 1114/1115/1116 and never interprets or rejects on reserved bits (internal/component/bgp/plugins/nlri/ls/attr_link.go:755-763,804-813,838-845); RFC 8571 mandates ignore-on-receipt not reject, so no negative case exists |
| `RFC8571-x-3` | TLVs must only be added to Link NLRIs in the BGP-LS Attribute (MUST Requirements) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no Link-NLRI origination path: the plugin sets only OnDecodeNLRI (internal/component/bgp/plugins/nlri/ls/plugin.go:45) and registers Mode "decode" (plugin.go:70-71), so it never adds these TLVs to any NLRI |
| `RFC8571-x-4` | Semantics and values must follow RFC 8570 (IS-IS) and RFC 7471 (OSPF) (MUST Requirements) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never assigns or transmits TE metric values: it only decodes received TLVs (internal/component/bgp/plugins/nlri/ls/attr_link.go:755-763,804-813,838-845) and has no encode path that could source semantics or values from RFC 8570 or RFC 7471 |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8571-x-1`](#rfc8571-x-1) Reserved fields in all TLVs (1114-1120) must be set to 0 on transmission (Encoding Rules) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no BGP-LS origination or encode path, so there is no encoder to zero the reserved field on transmission: the plugin registers Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70-71) and the TE-metric WriteTo encoders (internal/component/bgp/plugins/nlri/ls/attr_link.go:732,776,824) have no production caller |
| [`RFC8571-x-3`](#rfc8571-x-3) TLVs must only be added to Link NLRIs in the BGP-LS Attribute (MUST Requirements) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no Link-NLRI origination path: the plugin sets only OnDecodeNLRI (internal/component/bgp/plugins/nlri/ls/plugin.go:45) and registers Mode "decode" (plugin.go:70-71), so it never adds these TLVs to any NLRI |
| [`RFC8571-x-4`](#rfc8571-x-4) Semantics and values must follow RFC 8570 (IS-IS) and RFC 7471 (OSPF) (MUST Requirements) | no test | no test carries this requirement id; annotated {not-applicable}: ze never assigns or transmits TE metric values: it only decodes received TLVs (internal/component/bgp/plugins/nlri/ls/attr_link.go:755-763,804-813,838-845) and has no encode path that could source semantics or values from RFC 8570 or RFC 7471 |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8571-x-1`](#rfc8571-x-1)

Reserved fields in all TLVs (1114-1120) must be set to 0 on transmission (Encoding Rules)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8571-x-1, so no unit is bound to it.

### [`RFC8571-x-2`](#rfc8571-x-2)

Reserved fields in all TLVs must be ignored on receipt (Decoding Rules)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8571ReservedIgnoredDelayVariation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc8571_attr_reserved_test.go#L53) | unit/verify | unproven |
| positive | [`TestRFC8571ReservedIgnoredMinMaxDelay`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc8571_attr_reserved_test.go#L40) | unit/verify | unproven |
| positive | [`TestRFC8571ReservedIgnoredUnidirectionalDelay`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc8571_attr_reserved_test.go#L28) | unit/verify | unproven |

### [`RFC8571-x-3`](#rfc8571-x-3)

TLVs must only be added to Link NLRIs in the BGP-LS Attribute (MUST Requirements)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8571-x-3, so no unit is bound to it.

### [`RFC8571-x-4`](#rfc8571-x-4)

Semantics and values must follow RFC 8570 (IS-IS) and RFC 7471 (OSPF) (MUST Requirements)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8571-x-4, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 8571, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 8571, so its obligations are stated where they were written.
