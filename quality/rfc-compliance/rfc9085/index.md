# RFC 9085 - Border Gateway Protocol - Link State (BGP-LS) Extensions for Segment Routing

Partial. Every requirement this repository extracted from RFC 9085, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 12 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 25.0% | 3 of 12 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 12 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 3 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 12 | of 14 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 12 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 75.0% | 9 of 12 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 12 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 14 |
| Gated MUST-level | 12 |
| Obligations that bind Ze | 12 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 9 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 3 |
| Tagged units | 3 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc9085.md` |
| Requirement shard | `rfc/requirements/rfc9085.md` |
| RFC text | `rfc/full/rfc9085.txt` |

## Enrolment

Enrolled: BGP-LS Segment Routing extensions: 3 single-polarity positive (SID/Label 20-bit mask, reserved + undefined flags ignored on receipt) + 9 gap (SR-SID origination/encode not implemented; decode only)

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

- SR TLVs (SID/Label, Prefix-SID, Adj-SID, SR Capabilities, SRGB/SRLB) decode as part of BGP-LS TLV coverage
- the SID/Label 20-bit mask and reserved/undefined-flag fields are ignored on receipt.


**What the ledger says remains**

Nine origination/encode MUSTs unmet (decode-only plugin, no config surface): the reserved-and-flags-zero-on-transmit and TLV-placement rules have dormant encoders but no origination path; the LAN-Adjacency-SID (TLV 1100) and Range (TLV 1159) TLVs are not implemented at all.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 12 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **12** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (12):** [`RFC9085-2.1.2-1`](#rfc9085-2.1.2-1), [`RFC9085-2.1.2-2`](#rfc9085-2.1.2-2), [`RFC9085-2.1.4-1`](#rfc9085-2.1.4-1), [`RFC9085-2.1.4-2`](#rfc9085-2.1.4-2), [`RFC9085-2.2.1-1`](#rfc9085-2.2.1-1), [`RFC9085-2.2.2-1`](#rfc9085-2.2.2-1), [`RFC9085-2.3.1-1`](#rfc9085-2.3.1-1), [`RFC9085-2.3.5-1`](#rfc9085-2.3.5-1), [`RFC9085-2.1.1-1`](#rfc9085-2.1.1-1), [`RFC9085-2.1-1`](#rfc9085-2.1-1), [`RFC9085-2.1.2-3`](#rfc9085-2.1.2-3), [`RFC9085-2.1.2-4`](#rfc9085-2.1.2-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9085-2.1.2-1` | SR Capabilities TLV (1034): Flags MUST be set to 0 for OSPFv2 and OSPFv3 (S2.1.2) | MUST | 2.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** transmit obligation; the LsSRCapabilities encoder writes Flags verbatim and no production path originates an SR Capabilities TLV (internal/component/bgp/plugins/nlri/ls/attr_node.go:339; plugin registers decode only at plugin.go:70-71) |
| `RFC9085-2.1.2-2` | SR Capabilities TLV (1034): Reserved field MUST be set to 0 (S2.1.2) | MUST | 2.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the encoder hardcodes the reserved octet to 0 but ze originates no SR Capabilities TLV in production (internal/component/bgp/plugins/nlri/ls/attr_node.go:340) |
| `RFC9085-2.1.4-1` | SR Local Block TLV (1036): Flags MUST be set to 0 for OSPFv2 and OSPFv3 (S2.1.4) | MUST | 2.1.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** transmit obligation; the LsSRLocalBlock encoder writes Flags verbatim and no production path originates an SRLB TLV (internal/component/bgp/plugins/nlri/ls/attr_node.go:421) |
| `RFC9085-2.1.4-2` | SR Local Block TLV (1036): Reserved field MUST be set to 0 (S2.1.4) | MUST | 2.1.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the encoder hardcodes the reserved octet to 0 but ze originates no SRLB TLV in production (internal/component/bgp/plugins/nlri/ls/attr_node.go:422) |
| `RFC9085-2.2.1-1` | Adjacency SID TLV (1099): Reserved field (2 octets) MUST be set to 0 (S2.2.1) | MUST | 2.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the encoder hardcodes both reserved octets to 0 but ze originates no Adjacency SID TLV in production (internal/component/bgp/plugins/nlri/ls/attr_link.go:436-437) |
| `RFC9085-2.2.2-1` | LAN Adjacency SID TLV (1100): Reserved field (2 octets) MUST be set to 0 (S2.2.2) | MUST | 2.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze neither decodes nor encodes TLV 1100 (LAN Adjacency SID); it is not registered and no struct exists, so this transmit MUST is entirely unimplemented (internal/component/bgp/plugins/nlri/ls/register_attr.go) |
| `RFC9085-2.3.1-1` | Prefix-SID TLV (1158): Reserved field (2 octets) MUST be set to 0 (S2.3.1) | MUST | 2.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the encoder hardcodes both reserved octets to 0 but ze originates no Prefix-SID TLV in production (internal/component/bgp/plugins/nlri/ls/attr_prefix.go:154-155) |
| `RFC9085-2.3.5-1` | Range TLV (1159): Reserved field MUST be set to 0 (S2.3.5) | MUST | 2.3.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze neither decodes nor encodes TLV 1159 (Range); it is not registered and no struct exists, so this transmit MUST is entirely unimplemented (internal/component/bgp/plugins/nlri/ls/register_attr.go) |
| `RFC9085-2.1.1-1` | SID/Label TLV (1161): When Length=3, the 4 leftmost bits MUST be 0 (S2.1.1) | MUST | 2.1.1 | **positive:** `unit/verify` [`TestRFC9085SIDLabelMasksLeftmostFourBits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_test.go#L886). **negative:** no negative test. **{single-polarity}:** the decoder enforces the leftmost-4-bits-zero rule on receipt by masking the 3-octet value to its 20 rightmost bits (& 0xFFFFF), clearing rather than rejecting, so only a positive decode assertion is meaningful (internal/component/bgp/plugins/nlri/ls/attr_prefix.go:242) |
| `RFC9085-2.1-1` | TLVs MUST only be added to appropriate NLRI type (Node/Link/Prefix) (S2.1, S2.2, S2.3) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** this is an origination placement rule and ze originates no BGP-LS; the plugin registers decode mode only, so it never adds a TLV to any NLRI (internal/component/bgp/plugins/nlri/ls/plugin.go:70-71) |
| `RFC9085-2.1.2-3` | Reserved fields MUST be ignored on receipt (S2.1.2, S2.1.4, S2.2.1, S2.2.2, S2.3.1, S2.3.5) | MUST | 2.1.2 | **positive:** `unit/verify` [`TestRFC9085SRCapabilitiesIgnoresReservedAndUndefinedFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_test.go#L936). **negative:** no negative test. **{single-polarity}:** every SR decoder skips its reserved octets and never rejects on reserved content, so only a positive test is meaningful (internal/component/bgp/plugins/nlri/ls/attr_node.go:363, attr_link.go:478-479, attr_prefix.go:187-188) |
| `RFC9085-2.1.2-4` | OSPF-undefined flags in SR Capabilities/SRLB MUST be ignored on receipt (S2.1.2, S2.1.4) | MUST | 2.1.2 | **positive:** `unit/verify` [`TestRFC9085SRCapabilitiesIgnoresReservedAndUndefinedFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_test.go#L926). **negative:** no negative test. **{single-polarity}:** decodeSRCapabilities and decodeSRLocalBlock store the Flags octet without branching on or rejecting any bit, so undefined flags are inherently ignored (internal/component/bgp/plugins/nlri/ls/attr_node.go:367, :439) |
| `RFC9085-2.2.3-1` | L2 Bundle Member Attributes TLV MAY include sub-TLVs describing bundle member attributes (S2.2.3) | MAY | 2.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9085-2.2.3-2` | Multiple L2 Bundle Member Attributes TLVs MAY be associated with a Link NLRI (S2.2.3) | MAY | 2.2.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9085-2.1.2-1`](#rfc9085-2.1.2-1) SR Capabilities TLV (1034): Flags MUST be set to 0 for OSPFv2 and OSPFv3 (S2.1.2) | {gap}, no test | transmit obligation; the LsSRCapabilities encoder writes Flags verbatim and no production path originates an SR Capabilities TLV (internal/component/bgp/plugins/nlri/ls/attr_node.go:339; plugin registers decode only at plugin.go:70-71) |
| [`RFC9085-2.1.2-2`](#rfc9085-2.1.2-2) SR Capabilities TLV (1034): Reserved field MUST be set to 0 (S2.1.2) | {gap}, no test | the encoder hardcodes the reserved octet to 0 but ze originates no SR Capabilities TLV in production (internal/component/bgp/plugins/nlri/ls/attr_node.go:340) |
| [`RFC9085-2.1.4-1`](#rfc9085-2.1.4-1) SR Local Block TLV (1036): Flags MUST be set to 0 for OSPFv2 and OSPFv3 (S2.1.4) | {gap}, no test | transmit obligation; the LsSRLocalBlock encoder writes Flags verbatim and no production path originates an SRLB TLV (internal/component/bgp/plugins/nlri/ls/attr_node.go:421) |
| [`RFC9085-2.1.4-2`](#rfc9085-2.1.4-2) SR Local Block TLV (1036): Reserved field MUST be set to 0 (S2.1.4) | {gap}, no test | the encoder hardcodes the reserved octet to 0 but ze originates no SRLB TLV in production (internal/component/bgp/plugins/nlri/ls/attr_node.go:422) |
| [`RFC9085-2.2.1-1`](#rfc9085-2.2.1-1) Adjacency SID TLV (1099): Reserved field (2 octets) MUST be set to 0 (S2.2.1) | {gap}, no test | the encoder hardcodes both reserved octets to 0 but ze originates no Adjacency SID TLV in production (internal/component/bgp/plugins/nlri/ls/attr_link.go:436-437) |
| [`RFC9085-2.2.2-1`](#rfc9085-2.2.2-1) LAN Adjacency SID TLV (1100): Reserved field (2 octets) MUST be set to 0 (S2.2.2) | {gap}, no test | ze neither decodes nor encodes TLV 1100 (LAN Adjacency SID); it is not registered and no struct exists, so this transmit MUST is entirely unimplemented (internal/component/bgp/plugins/nlri/ls/register_attr.go) |
| [`RFC9085-2.3.1-1`](#rfc9085-2.3.1-1) Prefix-SID TLV (1158): Reserved field (2 octets) MUST be set to 0 (S2.3.1) | {gap}, no test | the encoder hardcodes both reserved octets to 0 but ze originates no Prefix-SID TLV in production (internal/component/bgp/plugins/nlri/ls/attr_prefix.go:154-155) |
| [`RFC9085-2.3.5-1`](#rfc9085-2.3.5-1) Range TLV (1159): Reserved field MUST be set to 0 (S2.3.5) | {gap}, no test | ze neither decodes nor encodes TLV 1159 (Range); it is not registered and no struct exists, so this transmit MUST is entirely unimplemented (internal/component/bgp/plugins/nlri/ls/register_attr.go) |
| [`RFC9085-2.1-1`](#rfc9085-2.1-1) TLVs MUST only be added to appropriate NLRI type (Node/Link/Prefix) (S2.1, S2.2, S2.3) | {gap}, no test | this is an origination placement rule and ze originates no BGP-LS; the plugin registers decode mode only, so it never adds a TLV to any NLRI (internal/component/bgp/plugins/nlri/ls/plugin.go:70-71) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9085-2.1.2-1`](#rfc9085-2.1.2-1)

SR Capabilities TLV (1034): Flags MUST be set to 0 for OSPFv2 and OSPFv3 (S2.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9085-2.1.2-1, so no unit is bound to it.

### [`RFC9085-2.1.2-2`](#rfc9085-2.1.2-2)

SR Capabilities TLV (1034): Reserved field MUST be set to 0 (S2.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9085-2.1.2-2, so no unit is bound to it.

### [`RFC9085-2.1.4-1`](#rfc9085-2.1.4-1)

SR Local Block TLV (1036): Flags MUST be set to 0 for OSPFv2 and OSPFv3 (S2.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9085-2.1.4-1, so no unit is bound to it.

### [`RFC9085-2.1.4-2`](#rfc9085-2.1.4-2)

SR Local Block TLV (1036): Reserved field MUST be set to 0 (S2.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9085-2.1.4-2, so no unit is bound to it.

### [`RFC9085-2.2.1-1`](#rfc9085-2.2.1-1)

Adjacency SID TLV (1099): Reserved field (2 octets) MUST be set to 0 (S2.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9085-2.2.1-1, so no unit is bound to it.

### [`RFC9085-2.2.2-1`](#rfc9085-2.2.2-1)

LAN Adjacency SID TLV (1100): Reserved field (2 octets) MUST be set to 0 (S2.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9085-2.2.2-1, so no unit is bound to it.

### [`RFC9085-2.3.1-1`](#rfc9085-2.3.1-1)

Prefix-SID TLV (1158): Reserved field (2 octets) MUST be set to 0 (S2.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9085-2.3.1-1, so no unit is bound to it.

### [`RFC9085-2.3.5-1`](#rfc9085-2.3.5-1)

Range TLV (1159): Reserved field MUST be set to 0 (S2.3.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9085-2.3.5-1, so no unit is bound to it.

### [`RFC9085-2.1.1-1`](#rfc9085-2.1.1-1)

SID/Label TLV (1161): When Length=3, the 4 leftmost bits MUST be 0 (S2.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9085SIDLabelMasksLeftmostFourBits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_test.go#L886) | unit/verify | unproven |

### [`RFC9085-2.1-1`](#rfc9085-2.1-1)

TLVs MUST only be added to appropriate NLRI type (Node/Link/Prefix) (S2.1, S2.2, S2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9085-2.1-1, so no unit is bound to it.

### [`RFC9085-2.1.2-3`](#rfc9085-2.1.2-3)

Reserved fields MUST be ignored on receipt (S2.1.2, S2.1.4, S2.2.1, S2.2.2, S2.3.1, S2.3.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9085SRCapabilitiesIgnoresReservedAndUndefinedFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_test.go#L936) | unit/verify | unproven |

### [`RFC9085-2.1.2-4`](#rfc9085-2.1.2-4)

OSPF-undefined flags in SR Capabilities/SRLB MUST be ignored on receipt (S2.1.2, S2.1.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9085SRCapabilitiesIgnoresReservedAndUndefinedFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_test.go#L926) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 9085, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 9085, so its obligations are stated where they were written.
