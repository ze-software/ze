# RFC 3787 - Recommendations for Interoperable IP Networks using Intermediate System to Intermediate System (IS-IS)

Partial. Every requirement this repository extracted from RFC 3787, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 66.7% | 2 of 3 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 3 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 3 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 4 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 3 | of 6 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 3 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 33.3% | 1 of 3 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 3 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 6 |
| Gated MUST-level | 3 |
| Obligations that bind Ze | 3 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 4 |
| Tagged units | 4 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc3787.md` |
| Requirement shard | `rfc/requirements/rfc3787.md` |
| RFC text | `rfc/full/rfc3787.txt` |

## Enrolment

Enrolled: IS-IS interoperability guidelines: three MUST-level requirements over the IS-IS TLV codec and IIH origination. RFC3787-x-1 (ignore TLV 131 Inter-Domain Routing Protocol Info and TLV 133 old Authentication if received) has both polarities in TestISISIgnoreObsoleteTLVs131And133: neither is a recognized codec type constant so both fall through the opaque-unknown passthrough (retained for verbatim re-flood, never interpreted) and TLV 133 is never selected by AuthTLVIndex (only TLV 10 is auth), while a recognized TLV 129 in the same region IS still decoded (ignore is scoped, not a blanket drop). RFC3787-x-2 (generate Protocols Supported TLV 129 including IP + IP Interface Address TLV 132 in IIH) has both polarities: TestISISIIHOriginationTLVs proves the originated LAN and P2P IIH both carry TLV 129 and 132, and TestISISHelloTLV132RequiresInterfaceAddr proves TLV 132 is emitted only from a real interface address (a circuit with no IPv4 omits it) while TLV 129 still advertises the IPv4 NLPID. RFC3787-5-1 (continue narrow metrics unless all devices support wide) is {gap}: Ze originates ONLY wide metrics (TLV 22/135) and never the narrow TLV 2 by umbrella decision, so it does not fall back to narrow-metric origination for a mixed domain; disclosed in the docs/features/rfc-status.md RFC 3787 row (Partial). The 4-1/4-2 SHOULDs (overload bit), 8-1 MAY (default routes in L1) and the FORMAT rows are not gated.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Obsolete TLV 131/133 are ignored on receipt (no codec decoder, opaque passthrough, TLV 133 never treated as auth)
- the originated IIH carries the Protocols Supported TLV (129, IPv4 NLPID) and IP Interface Address TLV (132) for mixed-environment interoperability. Tests bound per requirement in [`rfc/requirements/rfc3787.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc3787.md).


**What the ledger says remains**

One MUST gap, gated in [`rfc/short/rfc3787.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc3787.md): Ze originates ONLY wide metrics (TLV 22/135) and never the narrow TLV 2, so it does not fall back to narrow-metric origination for a mixed narrow/wide domain -- it requires every device to be wide-capable. Ze still DECODES a legacy neighbor's narrow TLV 2.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the gated population |
| Annotated instead of tested | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **3** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC3787-x-1`](#rfc3787-x-1), [`RFC3787-x-2`](#rfc3787-x-2)

**Annotated instead of tested (1):** [`RFC3787-5-1`](#rfc3787-5-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3787-x-1` | Ignore TLV 131 (Inter-Domain Routing Protocol Information) and TLV 133 (Authentication, replaced by TLV 10) if received (Sections 3.1, 3.2) | MUST | x | **positive:** `unit/verify` [`TestISISIgnoreObsoleteTLVs131And133`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_opaque_test.go#L54). **negative:** `unit/verify` [`TestISISIgnoreObsoleteTLVs131And133`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_opaque_test.go#L61) |
| `RFC3787-4-1` | Use the Overload Bit to signal not ready for transit traffic; set it in non-pseudonode LSP number Zero, not in PseudoNode LSPs; ignore OL in PseudoNode LSPs (Section 4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3787-4-2` | On receiving an OL-set LSP number Zero, treat all IP reachability advertisements as directly connected in SPF (Section 4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3787-5-1` | Continue using narrow metrics unless all devices in the domain support wide metrics (Section 5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze originates ONLY wide metrics by umbrella decision -- it never emits the narrow TLV 2 (internal/plugins/isis/packet/tlv_neighbours.go:59-60 "Ze never originates TLV 2"; internal/plugins/isis/types/metric.go:15 "Only wide metrics are originated by Ze"). Ze DECODES a legacy neighbor's narrow TLV 2 (decode-only) so it can parse mixed-domain LSPs, but it does not fall back to narrow-metric ORIGINATION, so a narrow-only router cannot interpret Ze's advertisements. Ze therefore requires every device in the domain to support wide metrics rather than continuing with narrow until the whole domain is wide-capable. Disclosed in docs/features/rfc-status.md |
| `RFC3787-8-1` | Generate default routes in Level 1 (Section 8) | MAY | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC3787-x-2` | Generate a Protocol Supported TLV (code 129) including IP, and include an IP Interface Address TLV (132) in IIH PDUs for mixed-environment interoperability (Sections 9, 10) | MUST | x | **positive:** `unit/verify` [`TestISISIIHOriginationTLVs`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/hello_test.go#L136). **negative:** `unit/verify` [`TestISISHelloTLV132RequiresInterfaceAddr`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/hello_test.go#L186) |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC3787-5-1`](#rfc3787-5-1) Continue using narrow metrics unless all devices in the domain support wide metrics (Section 5) | {gap}, no test | Ze originates ONLY wide metrics by umbrella decision -- it never emits the narrow TLV 2 (internal/plugins/isis/packet/tlv_neighbours.go:59-60 "Ze never originates TLV 2"; internal/plugins/isis/types/metric.go:15 "Only wide metrics are originated by Ze"). Ze DECODES a legacy neighbor's narrow TLV 2 (decode-only) so it can parse mixed-domain LSPs, but it does not fall back to narrow-metric ORIGINATION, so a narrow-only router cannot interpret Ze's advertisements. Ze therefore requires every device in the domain to support wide metrics rather than continuing with narrow until the whole domain is wide-capable. Disclosed in docs/features/rfc-status.md |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC3787-x-1`](#rfc3787-x-1)

Ignore TLV 131 (Inter-Domain Routing Protocol Information) and TLV 133 (Authentication, replaced by TLV 10) if received (Sections 3.1, 3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISIgnoreObsoleteTLVs131And133`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_opaque_test.go#L61) | unit/verify | unproven |
| positive | [`TestISISIgnoreObsoleteTLVs131And133`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_opaque_test.go#L54) | unit/verify | unproven |

### [`RFC3787-5-1`](#rfc3787-5-1)

Continue using narrow metrics unless all devices in the domain support wide metrics (Section 5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3787-5-1, so no unit is bound to it.

### [`RFC3787-x-2`](#rfc3787-x-2)

Generate a Protocol Supported TLV (code 129) including IP, and include an IP Interface Address TLV (132) in IIH PDUs for mixed-environment interoperability (Sections 9, 10)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISHelloTLV132RequiresInterfaceAddr`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/hello_test.go#L186) | unit/verify | unproven |
| positive | [`TestISISIIHOriginationTLVs`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/hello_test.go#L136) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 3787, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 3787, so its obligations are stated where they were written.
