# RFC 9085 - Border Gateway Protocol - Link State (BGP-LS) Extensions for Segment Routing

Partial. Every requirement this repository extracted from RFC 9085, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 11 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 27.3% | 3 of 11 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 11 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 3 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 11 | of 14 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 11 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 11 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 11 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 11 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 72.7% | 8 of 11 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 3 | of 11 gated MUSTs judged | 3 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 11 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 14 |
| Gated MUST-level | 11 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 9 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 3 |
| Tagged units | 3 |
| Recorded audit verdicts | 3 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc9085.md` |
| Requirement shard | `rfc/requirements/rfc9085.md` |
| RFC text | `rfc/full/rfc9085.txt` |

## Enrolment

Enrolled: BGP-LS Segment Routing extensions: 3 single-polarity positive (SID/Label 20-bit mask, reserved + undefined flags ignored on receipt) + 8 MUST gaps (SR-SID origination/encode not implemented; decode only). The 2026-09-21 extraction walk lowered RFC9085-2.1-1 to SHOULD, the level the document's own sentence uses, so the TLV-placement rule is now a SHOULD gap and the MUST count is 11 rather than 12.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

- SR TLVs (SID/Label, Prefix-SID, Adj-SID, SR Capabilities, SRGB/SRLB) decode as part of BGP-LS TLV coverage
- the SID/Label 20-bit mask and reserved/undefined-flag fields are ignored on receipt.


**What the ledger says remains**

Eight origination/encode MUSTs unmet (decode-only plugin, no config surface): the reserved-and-flags-zero-on-transmit rules have dormant encoders but no origination path, and the LAN-Adjacency-SID (TLV 1100) and Range (TLV 1159) TLVs are not implemented at all. The TLV-placement rule [`RFC9085-2.1-1`](#rfc9085-2.1-1) is unmet too and is a SHOULD, the level the document states it at.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 11 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **11** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (11):** [`RFC9085-2.1.2-1`](#rfc9085-2.1.2-1), [`RFC9085-2.1.2-2`](#rfc9085-2.1.2-2), [`RFC9085-2.1.4-1`](#rfc9085-2.1.4-1), [`RFC9085-2.1.4-2`](#rfc9085-2.1.4-2), [`RFC9085-2.2.1-1`](#rfc9085-2.2.1-1), [`RFC9085-2.2.2-1`](#rfc9085-2.2.2-1), [`RFC9085-2.3.1-1`](#rfc9085-2.3.1-1), [`RFC9085-2.3.5-1`](#rfc9085-2.3.5-1), [`RFC9085-2.1.1-1`](#rfc9085-2.1.1-1), [`RFC9085-2.1.2-3`](#rfc9085-2.1.2-3), [`RFC9085-2.1.2-4`](#rfc9085-2.1.2-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9085-2.1.2-1` | The flags are not currently defined for OSPFv2 and OSPFv3 and MUST be set to 0 and ignored on receipt. (§2.1.2) | MUST | 2.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** transmit obligation; the LsSRCapabilities encoder writes Flags verbatim and no production path originates an SR Capabilities TLV (internal/component/bgp/plugins/nlri/ls/attr_node.go:339; plugin registers decode only at plugin.go:70-71) |
| `RFC9085-2.1.2-2` | Reserved: 1 octet that MUST be set to 0 and ignored on receipt. (§2.1.2) | MUST | 2.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the encoder hardcodes the reserved octet to 0 but ze originates no SR Capabilities TLV in production (internal/component/bgp/plugins/nlri/ls/attr_node.go:340) |
| `RFC9085-2.1.4-1` | The flags are not currently defined for OSPFv2 and OSPFv3 and MUST be set to 0 and ignored on receipt. (§2.1.4) | MUST | 2.1.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** transmit obligation; the lsSRLocalBlock encoder writes Flags verbatim and no production path originates an SRLB TLV (internal/component/bgp/plugins/nlri/ls/attr_node.go:421) |
| `RFC9085-2.1.4-2` | Reserved: 1 octet that MUST be set to 0 and ignored on receipt. (§2.1.4) | MUST | 2.1.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the encoder hardcodes the reserved octet to 0 but ze originates no SRLB TLV in production (internal/component/bgp/plugins/nlri/ls/attr_node.go:422) |
| `RFC9085-2.2.1-1` | Reserved: 2 octets that MUST be set to 0 and ignored on receipt. (§2.2.1) | MUST | 2.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the encoder hardcodes both reserved octets to 0 but ze originates no Adjacency SID TLV in production (internal/component/bgp/plugins/nlri/ls/attr_link.go:436-437) |
| `RFC9085-2.2.2-1` | Reserved: 2 octets that MUST be set to 0 and ignored on receipt. (§2.2.2) | MUST | 2.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze neither decodes nor encodes TLV 1100 (LAN Adjacency SID); it is not registered and no struct exists, so this transmit MUST is entirely unimplemented (internal/component/bgp/plugins/nlri/ls/register_attr.go) |
| `RFC9085-2.3.1-1` | Reserved: 2 octets that MUST be set to 0 and ignored on receipt. (§2.3.1) | MUST | 2.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the encoder hardcodes both reserved octets to 0 but ze originates no Prefix-SID TLV in production (internal/component/bgp/plugins/nlri/ls/attr_prefix.go:154-155) |
| `RFC9085-2.3.5-1` | Reserved: 1 octet that MUST be set to 0 and ignored on receipt. (§2.3.5) | MUST | 2.3.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze neither decodes nor encodes TLV 1159 (Range); it is not registered and no struct exists, so this transmit MUST is entirely unimplemented (internal/component/bgp/plugins/nlri/ls/register_attr.go) |
| `RFC9085-2.1.1-1` | SID/Label: If the length is set to 3, then the 20 rightmost bits represent a label (the total TLV size is 7), and the 4 leftmost bits are set to 0. (§2.1.1) | MUST | 2.1.1 | **positive:** `unit/verify` [`TestRFC9085SIDLabelMasksLeftmostFourBits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_test.go#L888). **negative:** no negative test. **{single-polarity}:** the decoder enforces the leftmost-4-bits-zero rule on receipt by masking the 3-octet value to its 20 rightmost bits (& 0xFFFFF), clearing rather than rejecting, so only a positive decode assertion is meaningful (internal/component/bgp/plugins/nlri/ls/attr_prefix.go:242) |
| `RFC9085-2.1-1` | These TLVs should only be added to the BGP-LS Attribute associated with the Node NLRI that describes the IGP node that is originating the corresponding IGP TLV/sub-TLV described below. (§2.1) | SHOULD | 2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** this is an origination placement rule and ze originates no BGP-LS; the plugin registers decode mode only, so it never adds a TLV to any NLRI (internal/component/bgp/plugins/nlri/ls/plugin.go:70-71) |
| `RFC9085-2.1.2-3` | Reserved: 1 octet that MUST be set to 0 and ignored on receipt. (§2.1.2) | MUST | 2.1.2 | **positive:** `unit/verify` [`TestRFC9085SRCapabilitiesIgnoresReservedAndUndefinedFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_test.go#L938). **negative:** no negative test. **{single-polarity}:** every SR decoder skips its reserved octets and never rejects on reserved content, so only a positive test is meaningful (internal/component/bgp/plugins/nlri/ls/attr_node.go:363, attr_link.go:478-479, attr_prefix.go:187-188) |
| `RFC9085-2.1.2-4` | The flags are not currently defined for OSPFv2 and OSPFv3 and MUST be set to 0 and ignored on receipt. (§2.1.2) | MUST | 2.1.2 | **positive:** `unit/verify` [`TestRFC9085SRCapabilitiesIgnoresReservedAndUndefinedFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_test.go#L928). **negative:** no negative test. **{single-polarity}:** decodeSRCapabilities and decodeSRLocalBlock store the Flags octet without branching on or rejecting any bit, so undefined flags are inherently ignored (internal/component/bgp/plugins/nlri/ls/attr_node.go:367, :439) |
| `RFC9085-2.2.3-1` | The TLV MAY include sub-TLVs that describe attributes associated with the bundle member. (§2.2.3) | MAY | 2.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9085-2.2.3-2` | Multiple L2 Bundle Member Attributes TLVs MAY be associated with a Link NLRI (S2.2.3) | MAY | 2.2.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9085-2.1.2-1`](#rfc9085-2.1.2-1) The flags are not currently defined for OSPFv2 and OSPFv3 and MUST be set to 0 and ignored on receipt. (§2.1.2) | {gap}, no test | transmit obligation; the LsSRCapabilities encoder writes Flags verbatim and no production path originates an SR Capabilities TLV (internal/component/bgp/plugins/nlri/ls/attr_node.go:339; plugin registers decode only at plugin.go:70-71) |
| [`RFC9085-2.1.2-2`](#rfc9085-2.1.2-2) Reserved: 1 octet that MUST be set to 0 and ignored on receipt. (§2.1.2) | {gap}, no test | the encoder hardcodes the reserved octet to 0 but ze originates no SR Capabilities TLV in production (internal/component/bgp/plugins/nlri/ls/attr_node.go:340) |
| [`RFC9085-2.1.4-1`](#rfc9085-2.1.4-1) The flags are not currently defined for OSPFv2 and OSPFv3 and MUST be set to 0 and ignored on receipt. (§2.1.4) | {gap}, no test | transmit obligation; the lsSRLocalBlock encoder writes Flags verbatim and no production path originates an SRLB TLV (internal/component/bgp/plugins/nlri/ls/attr_node.go:421) |
| [`RFC9085-2.1.4-2`](#rfc9085-2.1.4-2) Reserved: 1 octet that MUST be set to 0 and ignored on receipt. (§2.1.4) | {gap}, no test | the encoder hardcodes the reserved octet to 0 but ze originates no SRLB TLV in production (internal/component/bgp/plugins/nlri/ls/attr_node.go:422) |
| [`RFC9085-2.2.1-1`](#rfc9085-2.2.1-1) Reserved: 2 octets that MUST be set to 0 and ignored on receipt. (§2.2.1) | {gap}, no test | the encoder hardcodes both reserved octets to 0 but ze originates no Adjacency SID TLV in production (internal/component/bgp/plugins/nlri/ls/attr_link.go:436-437) |
| [`RFC9085-2.2.2-1`](#rfc9085-2.2.2-1) Reserved: 2 octets that MUST be set to 0 and ignored on receipt. (§2.2.2) | {gap}, no test | ze neither decodes nor encodes TLV 1100 (LAN Adjacency SID); it is not registered and no struct exists, so this transmit MUST is entirely unimplemented (internal/component/bgp/plugins/nlri/ls/register_attr.go) |
| [`RFC9085-2.3.1-1`](#rfc9085-2.3.1-1) Reserved: 2 octets that MUST be set to 0 and ignored on receipt. (§2.3.1) | {gap}, no test | the encoder hardcodes both reserved octets to 0 but ze originates no Prefix-SID TLV in production (internal/component/bgp/plugins/nlri/ls/attr_prefix.go:154-155) |
| [`RFC9085-2.3.5-1`](#rfc9085-2.3.5-1) Reserved: 1 octet that MUST be set to 0 and ignored on receipt. (§2.3.5) | {gap}, no test | ze neither decodes nor encodes TLV 1159 (Range); it is not registered and no struct exists, so this transmit MUST is entirely unimplemented (internal/component/bgp/plugins/nlri/ls/register_attr.go) |
| [`RFC9085-2.1-1`](#rfc9085-2.1-1) These TLVs should only be added to the BGP-LS Attribute associated with the Node NLRI that describes the IGP node that is originating the corresponding IGP TLV/sub-TLV described below. (§2.1) | {gap} | this is an origination placement rule and ze originates no BGP-LS; the plugin registers decode mode only, so it never adds a TLV to any NLRI (internal/component/bgp/plugins/nlri/ls/plugin.go:70-71) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9085-2.1.2-1`](#rfc9085-2.1.2-1)

The flags are not currently defined for OSPFv2 and OSPFv3 and MUST be set to 0 and ignored on receipt. (§2.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9085-2.1.2-1, so no unit is bound to it.

### [`RFC9085-2.1.2-2`](#rfc9085-2.1.2-2)

Reserved: 1 octet that MUST be set to 0 and ignored on receipt. (§2.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9085-2.1.2-2, so no unit is bound to it.

### [`RFC9085-2.1.4-1`](#rfc9085-2.1.4-1)

The flags are not currently defined for OSPFv2 and OSPFv3 and MUST be set to 0 and ignored on receipt. (§2.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9085-2.1.4-1, so no unit is bound to it.

### [`RFC9085-2.1.4-2`](#rfc9085-2.1.4-2)

Reserved: 1 octet that MUST be set to 0 and ignored on receipt. (§2.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9085-2.1.4-2, so no unit is bound to it.

### [`RFC9085-2.2.1-1`](#rfc9085-2.2.1-1)

Reserved: 2 octets that MUST be set to 0 and ignored on receipt. (§2.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9085-2.2.1-1, so no unit is bound to it.

### [`RFC9085-2.2.2-1`](#rfc9085-2.2.2-1)

Reserved: 2 octets that MUST be set to 0 and ignored on receipt. (§2.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9085-2.2.2-1, so no unit is bound to it.

### [`RFC9085-2.3.1-1`](#rfc9085-2.3.1-1)

Reserved: 2 octets that MUST be set to 0 and ignored on receipt. (§2.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9085-2.3.1-1, so no unit is bound to it.

### [`RFC9085-2.3.5-1`](#rfc9085-2.3.5-1)

Reserved: 1 octet that MUST be set to 0 and ignored on receipt. (§2.3.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9085-2.3.5-1, so no unit is bound to it.

### [`RFC9085-2.1.1-1`](#rfc9085-2.1.1-1)

SID/Label: If the length is set to 3, then the 20 rightmost bits represent a label (the total TLV size is 7), and the 4 leftmost bits are set to 0. (§2.1.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Two clauses. The 20 rightmost bits represent the label: the tagged test decodes 0xF12345 and assert.Equal(0x12345, sl.SID) goes red if the top nibble is folded in. The 4 leftmost bits are set to 0: a transmit duty, and no tagged unit asserts what Ze encodes (Ze originates no BGP-LS, plugin.go registers decode only), so that clause has no assertion.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9085SIDLabelMasksLeftmostFourBits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_test.go#L888) | unit/verify | unproven |

### [`RFC9085-2.1.2-3`](#rfc9085-2.1.2-3)

Reserved: 1 octet that MUST be set to 0 and ignored on receipt. (§2.1.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The quoted sentence is "Reserved: 1 octet that MUST be set to 0 and ignored on receipt." Ignored on receipt: TestRFC9085SRCapabilitiesIgnoresReservedAndUndefinedFlags sets the octet to 0xFF and require.NoError plus Range 1000 / FirstSID 16000 go red if it is rejected or consumed. Set to 0: no tagged unit asserts the transmitted octet (that duty is RFC9085-2.1.2-2, a gap). The row also claims five other sections' reserved fields (split needed), none exercised here.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9085SRCapabilitiesIgnoresReservedAndUndefinedFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_test.go#L938) | unit/verify | unproven |

### [`RFC9085-2.1.2-4`](#rfc9085-2.1.2-4)

The flags are not currently defined for OSPFv2 and OSPFv3 and MUST be set to 0 and ignored on receipt. (§2.1.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The quoted sentence is "The flags are not currently defined for OSPFv2 and OSPFv3 and MUST be set to 0 and ignored on receipt." Ignored on receipt: the same test sets undefined bit 0x02 and require.NoError plus the I-flag assertion go red if the TLV is rejected or the meaningful flag disturbed. Set to 0: no tagged unit asserts transmitted flags (RFC9085-2.1.2-1, a gap). The row also claims §2.1.4 SRLB flags (split needed), not exercised by this unit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9085SRCapabilitiesIgnoresReservedAndUndefinedFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_test.go#L928) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc9085.txt |
| Source fingerprint | fdec3413ebb35a14 |
| Record | rfc/extraction/rfc9085.json |
| Mapped sentences | 8 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.1.1` | not stated | 0 | walked | not stated |
| `2.1.2` | not stated | 2 | walked | not stated |
| `2.1.3` | not stated | 0 | walked | not stated |
| `2.1.4` | not stated | 3 | walked | not stated |
| `2.1.5` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `2.2.1` | not stated | 1 | walked | not stated |
| `2.2.2` | not stated | 1 | walked | not stated |
| `2.2.3` | not stated | 0 | walked | not stated |
| `2.3` | not stated | 0 | walked | not stated |
| `2.3.1` | not stated | 1 | walked | not stated |
| `2.3.2` | not stated | 0 | walked | not stated |
| `2.3.3` | not stated | 0 | walked | not stated |
| `2.3.4` | not stated | 0 | walked | not stated |
| `2.3.5` | not stated | 1 | walked | not stated |
| `2.4` | not stated | 0 | walked | not stated |
| `2.5` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 1 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Boilerplate from the Copyright Notice: it states the licence terms under which code components extracted from the document are provided ('must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions'). It binds a republisher of the text, not a BGP-LS implementation, and names no message, field or procedure. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `2.1.4:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The obligation to advertise the SRLB belongs to the IGP documents the same section cites three lines later: 'This information is derived from the protocol-specific advertisements. * IS-IS, as defined by the SRLB Sub-TLV in Section 3.3 of [RFC8667]. * OSPFv2/OSPFv3, as defined by the SR Local Block TLV in Section 3.3 of [RFC8665] and [RFC8666]'. RFC 9085 defines only how BGP-LS carries the range once the IGP has advertised it; the sentence explains why the IGP advertisement exists. | Therefore, in order for such applications or controllers to know the range of local SIDs available, the node is required to advertise its SRLB. |
| `5:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A Security Considerations ASSUMPTION rather than an obligation: 'The IGP instances originating these TLVs are assumed to support all the required security and authentication mechanisms (as described in [RFC8665], [RFC8666], and [RFC8667])'. It states what this document takes for granted about the IGP that feeds BGP-LS, and the mechanisms it names are required by those three documents, not by this one. | The IGP instances originating these TLVs are assumed to support all the required security and authentication mechanisms (as described in [RFC8665], [RFC8666], and [RFC8667]) in order to prevent any security issue when propagating the TLVs into BGP-LS. |

## Superseded

No document obsoletes RFC 9085, so its obligations are stated where they were written.
