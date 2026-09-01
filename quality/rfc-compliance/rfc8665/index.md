# RFC 8665 - OSPF Extensions for Segment Routing

Partial. Every requirement this repository extracted from RFC 8665, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 59.6% | 28 of 47 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 10.6% | 5 of 47 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 47 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 61 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 47 | of 82 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 47 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 29.8% | 14 of 47 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 47 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 82 |
| Gated MUST-level | 47 |
| Obligations that bind Ze | 47 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 14 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 61 |
| Tagged units | 61 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc8665.md` |
| Requirement shard | `rfc/requirements/rfc8665.md` |
| RFC text | `rfc/full/rfc8665.txt` |

## Enrolment

Enrolled: OSPF Extensions for Segment Routing

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- OSPFv2 Segment Routing over the RFC 7770 Router Information LSA and the RFC 7684 Extended Prefix / Extended Link Opaque LSAs: the SR-Algorithm TLV (always Algorithm 0), one SID/Label Range TLV per SRGB range and one SR Local Block TLV per SRLB range, all area-scoped, each carrying exactly one SID/Label sub-TLV
- reception enforcing Range Size greater than 0, exactly one SID/Label sub-TLV, the first-occurrence rule for a repeated SR-Algorithm or SRMS Preference TLV within one LSA, and the reserved-label hardening
- SRGB index-to-label arithmetic in advertised range order
- Prefix-SID, Adj-SID and LAN-Adj-SID sub-TLV codecs with reserved-bit and V/L validation
- the NP/M/E outgoing-label truth table applied at the penultimate hop with the next-hop router's SRGB
- algorithm-not-advertised and duplicate Prefix-SID rejection
- SRLB-allocated Adj-SIDs installed and withdrawn with the adjacency. Requirements bound per line in [`rfc/short/rfc8665.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc8665.md) and gated by `./le rfc check`.


**What the ledger says remains**

Fourteen MUST gaps, each annotated in [`rfc/short/rfc8665.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc8665.md).

- **Multi-LSA capability resolution:** [`RFC8665-3.1-4`](#rfc8665-3.1-4), 3.1-5, 3.4-2, 3.4-3 (no flooding-scope or Instance-ID tie-break across RI LSAs; the last LSA read wins, [`internal/plugins/ospf/sr_install.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_install.go), and the received SRMS preference is decoded but unused).
- **Overlapping received ranges:** [`RFC8665-3.2-8`](#rfc8665-3.2-8) (concatenated with no overlap detection, [`internal/plugins/ospf/sr.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr.go)).
- **SR Mapping Server and prefix ranges:** [`RFC8665-4-1`](#rfc8665-4-1), 4-2, 4-3, 7.1-1, 7.1-2, 7.1-3 (ze originates no IPv4 Extended Prefix Range TLV; the value encoder at [`internal/plugins/ospf/sr/codec.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/codec.go) has no ABR caller, so the IA-Flag is never set and the Range Size capacity rule is unenforced). ABR / ASBR Prefix-SID flags and inter-area propagation: [`RFC8665-5-8`](#rfc8665-5-8), 5-9, 7.2-1 (the IPv4 Prefix-SID builder copies the configured NP/E flags and advertises only locally configured prefixes, [`internal/plugins/ospf/sr.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr.go); the equivalent rules exist only for IPv6 at [`internal/plugins/ospf/sr_interarea_v6.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_interarea_v6.go)). The feature also remains pre-production pending hardening and deployment evidence.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 28 | one part of the gated population |
| Annotated instead of tested | 19 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **47** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (28):** [`RFC8665-3.1-1`](#rfc8665-3.1-1), [`RFC8665-3.1-3`](#rfc8665-3.1-3), [`RFC8665-3.2-1`](#rfc8665-3.2-1), [`RFC8665-3.2-2`](#rfc8665-3.2-2), [`RFC8665-3.2-3`](#rfc8665-3.2-3), [`RFC8665-3.2-6`](#rfc8665-3.2-6), [`RFC8665-3.2-7`](#rfc8665-3.2-7), [`RFC8665-3.3-1`](#rfc8665-3.3-1), [`RFC8665-3.3-2`](#rfc8665-3.3-2), [`RFC8665-3.3-3`](#rfc8665-3.3-3), [`RFC8665-3.3-4`](#rfc8665-3.3-4), [`RFC8665-3.4-1`](#rfc8665-3.4-1), [`RFC8665-5-1`](#rfc8665-5-1), [`RFC8665-5-2`](#rfc8665-5-2), [`RFC8665-5-3`](#rfc8665-5-3), [`RFC8665-5-4`](#rfc8665-5-4), [`RFC8665-5-5`](#rfc8665-5-5), [`RFC8665-5-6`](#rfc8665-5-6), [`RFC8665-5-7`](#rfc8665-5-7), [`RFC8665-5-10`](#rfc8665-5-10), [`RFC8665-5-11`](#rfc8665-5-11), [`RFC8665-5-12`](#rfc8665-5-12), [`RFC8665-5-13`](#rfc8665-5-13), [`RFC8665-6.1-1`](#rfc8665-6.1-1), [`RFC8665-7.4.1-1`](#rfc8665-7.4.1-1), [`RFC8665-10-1`](#rfc8665-10-1), [`RFC8665-9-1`](#rfc8665-9-1), [`RFC8665-3.1-7`](#rfc8665-3.1-7)

**Annotated instead of tested (19):** [`RFC8665-3.1-2`](#rfc8665-3.1-2), [`RFC8665-3.1-4`](#rfc8665-3.1-4), [`RFC8665-3.1-5`](#rfc8665-3.1-5), [`RFC8665-3.2-4`](#rfc8665-3.2-4), [`RFC8665-3.2-5`](#rfc8665-3.2-5), [`RFC8665-3.2-8`](#rfc8665-3.2-8), [`RFC8665-3.4-2`](#rfc8665-3.4-2), [`RFC8665-3.4-3`](#rfc8665-3.4-3), [`RFC8665-4-1`](#rfc8665-4-1), [`RFC8665-4-2`](#rfc8665-4-2), [`RFC8665-4-3`](#rfc8665-4-3), [`RFC8665-5-8`](#rfc8665-5-8), [`RFC8665-5-9`](#rfc8665-5-9), [`RFC8665-6.1-2`](#rfc8665-6.1-2), [`RFC8665-6.2-1`](#rfc8665-6.2-1), [`RFC8665-7.1-1`](#rfc8665-7.1-1), [`RFC8665-7.1-2`](#rfc8665-7.1-2), [`RFC8665-7.1-3`](#rfc8665-7.1-3), [`RFC8665-7.2-1`](#rfc8665-7.2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8665-3.1-1` | If the SR-Algorithm TLV is advertised, Algorithm 0 MUST be included (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC8665SRAlgorithmTLVAdvertisesAlgorithmZeroOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L44). **negative:** `unit/verify` [`TestRFC8665NoSRAlgorithmTLVWhenSRUnconfigured`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L73) |
| `RFC8665-3.1-2` | Local policy at a node claiming support for Algorithm 1 MUST NOT alter the SPF paths computed by Algorithm 1 (§3.1, §8.5) | MUST NOT | 3.1 | **positive:** `unit/verify` [`TestRFC8665SRAlgorithmTLVAdvertisesAlgorithmZeroOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L47). **negative:** no negative test. **{single-polarity}:** ze advertises the single-entry algorithm list 0 and never claims Algorithm 1 -- srBuildAlgorithm encodes that literal list, internal/plugins/ospf/sr.go:154-160 -- and the installer refuses any Prefix-SID whose algorithm is not 0, internal/plugins/ospf/sr_install.go:89-92. There is no Algorithm 1 SPF computation to alter and no violating input to reject, so only the positive direction is meaningful |
| `RFC8665-3.1-3` | When multiple SR-Algorithm TLVs are received from a router, use the first occurrence of the TLV in the RI Opaque LSA (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC8665SingleAlgorithmAndSRMSInstanceUsed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L119). **negative:** `unit/verify` [`TestRFC8665RepeatedAlgorithmAndSRMSInstancesIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L138) |
| `RFC8665-3.1-4` | If the SR-Algorithm TLV appears in RI Opaque LSAs with different flooding scopes, use the one in the area-scoped LSA (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the SR capability read walks every RI Opaque LSA in the LSDB and assigns the per-router entry from whichever view it reaches last, with no flooding-scope comparison -- srRemoteCapabilities iterates e.lsdb.OpaqueLSAsByType at internal/plugins/ospf/sr_install.go:238-241 and its record closure assigns caps[router] and algos[router] at internal/plugins/ospf/sr_install.go:222-229 -- so an AS-scoped RI LSA can override the area-scoped one. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| `RFC8665-3.1-5` | If the SR-Algorithm TLV appears in RI Opaque LSAs with the same flooding scope, use the one with the numerically smallest Instance ID and ignore subsequent instances (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the SR capability read compares no Instance ID. The opaque view carries OpaqueID, the RFC 7770 Instance ID, but srRemoteCapabilities ignores it and the last view processed wins, internal/plugins/ospf/sr_install.go:238-241 with the assignment at internal/plugins/ospf/sr_install.go:222-229. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| `RFC8665-3.2-1` | Range Size in the SID/Label Range TLV MUST be greater than 0 (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC8665RangeTLVRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L22). **negative:** `unit/verify` [`TestRFC8665RangeSizeZeroRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L54) |
| `RFC8665-3.2-2` | The SID/Label Sub-TLV MUST be included in the SID/Label Range TLV (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC8665RangeTLVRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L27). **negative:** `unit/verify` [`TestRFC8665RangeWithoutSIDLabelSubTLVRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L81) |
| `RFC8665-3.2-3` | If more than one SID/Label Sub-TLV is present in the SID/Label Range TLV, the TLV MUST be ignored (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC8665RangeWithSingleSIDLabelAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L96). **negative:** `unit/verify` [`TestRFC8665RangeWithTwoSIDLabelSubTLVsIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L107) |
| `RFC8665-3.2-4` | When advertising multiple ranges, the originating router MUST encode each range into a different SID/Label Range TLV (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC8665EachRangeInItsOwnTLV`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L161). **negative:** no negative test. **{single-polarity}:** srBuildSRGB emits one packet.RITLV per configured range, internal/plugins/ospf/sr.go:167-172, so the encoder cannot express two ranges in one TLV and has no violating output to produce. The receive-side rejection of a range TLV carrying two SID/Label sub-TLVs is the RFC8665-3.2-3 negative test |
| `RFC8665-3.2-5` | The originating router MUST ensure the SID/Label Range TLV order is the same after a graceful restart (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC8665RangeOrderStableAcrossRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L189). **negative:** no negative test. **{single-polarity}:** the advertised order is a pure function of the configured SRGB slice -- srBuildSRGB walks it in slice order with no sort and no map iteration, internal/plugins/ospf/sr.go:168-172, and parseSegmentRouting rebuilds that slice in configuration document order on every start, internal/plugins/ospf/sr_config.go:27-30 -- so a restart reproduces the same order by construction and there is no reordered input to reject |
| `RFC8665-3.2-6` | The receiving router MUST adhere to the advertised range order when calculating a SID/Label from a SID index (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC8665SRGBIndexUsesAdvertisedOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L124). **negative:** `unit/verify` [`TestRFC8665SRGBIndexOutOfRangeAndOrderSensitivity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L142) |
| `RFC8665-3.2-7` | The originating router MUST NOT advertise overlapping ranges (SID/Label Range TLV) (§3.2) | MUST NOT | 3.2 | **positive:** `unit/verify` [`TestRFC8665NonOverlappingRangesAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L159). **negative:** `unit/verify` [`TestRFC8665OverlappingRangesRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L175) |
| `RFC8665-3.2-8` | When a router receives multiple overlapping ranges, it MUST conform to RFC 8660 (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the receive path appends every decoded SID/Label Range to the originator SRGB with no overlap detection, srDecodeRemoteCapabilities internal/plugins/ospf/sr.go:337-342, and SRGB.Label maps an index by plain concatenation in advertised order, internal/plugins/ospf/sr/srgb.go:93-105, so overlapping received ranges are concatenated rather than resolved per RFC 8660. The non-overlap check covers only this router's own configured ranges, internal/plugins/ospf/sr/config.go:116-121. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| `RFC8665-3.3-1` | Range Size in the SRLB TLV MUST be greater than 0 (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestRFC8665RangeTLVRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L24). **negative:** `unit/verify` [`TestRFC8665RangeSizeZeroRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L58) |
| `RFC8665-3.3-2` | The SID/Label Sub-TLV MUST be included in the SRLB TLV (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestRFC8665RangeTLVRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L30). **negative:** `unit/verify` [`TestRFC8665RangeWithoutSIDLabelSubTLVRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L84) |
| `RFC8665-3.3-3` | If more than one SID/Label Sub-TLV is present in the SRLB TLV, the SRLB TLV MUST be ignored (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestRFC8665RangeWithSingleSIDLabelAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L98). **negative:** `unit/verify` [`TestRFC8665RangeWithTwoSIDLabelSubTLVsIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L110) |
| `RFC8665-3.3-4` | The originating router MUST NOT advertise overlapping ranges (SRLB TLV) (§3.3) | MUST NOT | 3.3 | **positive:** `unit/verify` [`TestRFC8665NonOverlappingRangesAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L162). **negative:** `unit/verify` [`TestRFC8665OverlappingRangesRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L178) |
| `RFC8665-3.4-1` | When multiple SRMS Preference TLVs are received from a router, use the first occurrence of the TLV in the RI Opaque LSA (§3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestRFC8665SingleAlgorithmAndSRMSInstanceUsed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L121). **negative:** `unit/verify` [`TestRFC8665RepeatedAlgorithmAndSRMSInstancesIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L141) |
| `RFC8665-3.4-2` | If the SRMS Preference TLV appears in RI Opaque LSAs with different flooding scopes, use the one with the narrowest flooding scope (§3.4) | MUST | 3.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the received SRMS preference is decoded into srRemoteCapabilities.SRMSPref, internal/plugins/ospf/sr.go:349-358, and nothing consumes it: srRemoteCapabilities keeps only the SRGB and the algorithm list, internal/plugins/ospf/sr_install.go:222-229, so no narrowest-flooding-scope selection exists. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| `RFC8665-3.4-3` | If the SRMS Preference TLV appears in RI Opaque LSAs with the same flooding scope, use the one with the numerically smallest Instance ID and ignore subsequent instances (§3.4) | MUST | 3.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the decode keeps the first SRMS Preference TLV within one LSA body, internal/plugins/ospf/sr.go:349-358, but nothing compares instances across LSAs and the preference is never consumed, internal/plugins/ospf/sr_install.go:222-229, so there is no smallest-Instance-ID tie-break. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| `RFC8665-4-1` | All prefix ranges in a single OSPF Extended Prefix Opaque LSA MUST have the same flooding scope (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze originates no OSPF Extended Prefix Range TLV for IPv4. extPrefixOnOriginate builds one Extended Prefix Opaque LSA per advertised prefix carrying a single Extended Prefix TLV, internal/plugins/ospf/ext_prefix.go:61-80, and never populates ExtPrefixLSA.Ranges; the range value encoder exists at internal/plugins/ospf/sr/codec.go:482-494 with no caller outside tests, so no code assigns a flooding scope to a prefix range or keeps the ranges in one LSA scope-uniform. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| `RFC8665-4-2` | An ABR advertising the OSPF Extended Prefix Range TLV between areas MUST set the IA-Flag (§4, §7.1) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** EncodeExtPrefixRangeValueV4 takes an iaFlag argument and writes the IA-Flag bit, internal/plugins/ospf/sr/codec.go:482-494, but no ABR path calls it: the IPv4 Extended Prefix originator emits only Extended Prefix TLVs, internal/plugins/ospf/ext_prefix.go:61-80, and the only inter-area Prefix-SID propagation is the IPv6 one, internal/plugins/ospf/sr_interarea_v6.go:60-83, so no OSPFv2 ABR sets the IA-Flag. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| `RFC8665-4-3` | The Range Size MUST NOT exceed the number of prefixes satisfiable by the Prefix Length without including 224.0.0.0/3 (§4) | MUST NOT | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Extended Prefix Range encoder writes the caller's Range Size verbatim with no capacity check against the Prefix Length and no 224.0.0.0/3 exclusion, EncodeExtPrefixRangeValueV4 internal/plugins/ospf/sr/codec.go:482-494, and the decoder reads it back unchecked, internal/plugins/ospf/sr/codec.go:497-523. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| `RFC8665-5-1` | Reserved bits (other than NP/M/E/V/L) in the Prefix-SID Flags MUST be zero when sent and are ignored when received (§5) | MUST NOT | 5 | **positive:** `unit/verify` [`TestRFC8665PrefixSIDReservedFlagBitsZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L209). **negative:** `unit/verify` [`TestRFC8665PrefixSIDReservedFlagBitsIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L224) |
| `RFC8665-5-2` | If the NP-Flag is set, the penultimate hop MUST NOT pop the Prefix-SID before delivering to the advertising node (§5) | MUST NOT | 5 | **positive:** `unit/verify` [`TestRFC8665NoPHPKeepsPrefixSIDLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L306). **negative:** `unit/verify` [`TestRFC8665PHPPopsWhenNoPHPFlagClear`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L331) |
| `RFC8665-5-3` | If the E-Flag is set, any upstream neighbor MUST replace the Prefix-SID with the Explicit NULL label (0 for IPv4) before forwarding (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC8665ExplicitNullReplacesPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L344). **negative:** `unit/verify` [`TestRFC8665NoPHPKeepsPrefixSIDLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L311) |
| `RFC8665-5-4` | A router receiving a Prefix-SID with an algorithm value not advertised in the remote node's SR-Algorithm TLV MUST ignore the Prefix-SID Sub-TLV (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC8665PrefixSIDInstalledWhenAlgorithmAdvertised`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L258). **negative:** `unit/verify` [`TestRFC8665PrefixSIDIgnoredWhenAlgorithmNotAdvertised`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L286) |
| `RFC8665-5-5` | Any invalid combination of V- and L-Flags in a received SID Advertisement MUST cause it to be ignored (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC8665ValidVLCombinationsDecode`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L247). **negative:** `unit/verify` [`TestRFC8665InvalidVLCombinationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L279) |
| `RFC8665-5-6` | If an OSPF router advertises multiple Prefix-SIDs for the same prefix, topology, and algorithm, all of them MUST be ignored (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC8665PrefixSIDInstalledWhenAlgorithmAdvertised`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L261). **negative:** `unit/verify` [`TestRFC8665DuplicatePrefixSIDsAllIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L329) |
| `RFC8665-5-7` | When calculating the outgoing label, the router MUST take into account the next-hop router's E-, NP-, and M-Flags if that router advertised the SID, regardless of whether it contributes to the best path (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC8665NextHopFlagsAppliedWhereSIDAdvertised`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L360). **negative:** `unit/verify` [`TestRFC8665OriginatorFlagsNotAppliedAtTransitHop`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L401) |
| `RFC8665-5-8` | The NP-Flag MUST be set and the E-Flag MUST be clear for Prefix-SIDs allocated to inter-area prefixes originated by the ABR, unless the advertised prefix is directly attached to the ABR (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the IPv4 Prefix-SID builder copies the NP and E flags straight from configuration and never forces NP set with E clear for an inter-area prefix originated by an ABR, srBuildPrefixSID internal/plugins/ospf/sr.go:197-213, which matches only on the configured prefix and ignores the ctx.RouteType it is handed. The equivalent rule exists only for IPv6, v6InterAreaPrefixSIDRule internal/plugins/ospf/sr_interarea_v6.go:35-43. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| `RFC8665-5-9` | The NP-Flag MUST be set and the E-Flag MUST be clear for Prefix-SIDs allocated to redistributed prefixes, unless the redistributed prefix is directly attached to the ASBR (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the same builder applies no NP-set / E-clear rule to a redistributed prefix, srBuildPrefixSID internal/plugins/ospf/sr.go:197-213; the AS-external Extended Prefix advertisement carries whatever flags the prefix-sid configuration sets, internal/plugins/ospf/ext_prefix.go:162-176. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| `RFC8665-5-10` | If the NP-Flag is not set, any upstream neighbor of the Prefix-SID originator MUST pop the Prefix-SID (PHP) and the received E-Flag is ignored (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC8665PHPPopsWhenNoPHPFlagClear`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L328). **negative:** `unit/verify` [`TestRFC8665NoPHPKeepsPrefixSIDLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L315) |
| `RFC8665-5-11` | If the NP-Flag is set and the E-Flag is not set, any upstream neighbor MUST keep the Prefix-SID on top of the stack (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC8665NoPHPKeepsPrefixSIDLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L309). **negative:** `unit/verify` [`TestRFC8665ExplicitNullReplacesPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L349) |
| `RFC8665-5-12` | If both NP-Flag and E-Flag are set, any upstream neighbor MUST replace the Prefix-SID with an Explicit NULL label (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC8665ExplicitNullReplacesPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L347). **negative:** `unit/verify` [`TestRFC8665NoPHPKeepsPrefixSIDLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L313) |
| `RFC8665-5-13` | When the M-Flag is set, the NP-Flag and the E-Flag MUST be ignored on reception (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC8665MappingServerFlagIgnoresNPAndE`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L362). **negative:** `unit/verify` [`TestRFC8665MappingServerFlagClearHonorsNPAndE`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L376) |
| `RFC8665-6.1-1` | Reserved bits (5-7) in the Adj-SID Flags MUST be zero when sent and are ignored when received (§6.1) | MUST NOT | 6.1 | **positive:** `unit/verify` [`TestRFC8665AdjSIDReservedFlagBitsZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L389). **negative:** `unit/verify` [`TestRFC8665AdjSIDReservedFlagBitsIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L408) |
| `RFC8665-6.1-2` | When the P-Flag is set, the Adj-SID MUST be persistent (§6.1) | MUST | 6.1 | **positive:** `unit/verify` [`TestRFC8665AdjSIDNeverClaimsPersistence`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L215). **negative:** no negative test. **{single-polarity}:** every Adj-SID is allocated from the SRLB when the adjacency reaches Full and freed when it drops, and it is advertised with only the V and L flags set, srAdjManager.neighborFull internal/plugins/ospf/sr_adjsid.go:62-69 with the flag encoder at internal/plugins/ospf/sr/codec.go:115-133, so ze never sets the P-Flag and the persistence obligation never binds. A negative case needs ze to advertise P without persistence, which the encoder cannot produce |
| `RFC8665-6.2-1` | When the P-Flag is set, the LAN Adjacency SID MUST be persistent (§6.2) | MUST | 6.2 | **positive:** `unit/verify` [`TestRFC8665AdjSIDNeverClaimsPersistence`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L221). **negative:** no negative test. **{single-polarity}:** the LAN Adjacency SID is allocated by the same code path with lan set, srAdjManager.neighborFull internal/plugins/ospf/sr_adjsid.go:62-69, so it too carries the P-Flag clear and the persistence obligation never binds |
| `RFC8665-7.1-1` | An SR Mapping Server MUST use the OSPF Extended Prefix Range TLV when advertising SIDs for prefixes (§7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs no SR Mapping Server for IPv4. Nothing originates an Extended Prefix Range TLV into an Extended Prefix Opaque LSA -- ExtPrefixLSA.Ranges is populated only by the decoder, internal/plugins/ospf/packet/ext_prefix.go:165-168, and read only by the show path, internal/plugins/ospf/ext_render.go:106 -- and the M-Flag is never set on an originated Prefix-SID, internal/plugins/ospf/sr.go:204-208. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| `RFC8665-7.1-2` | When propagating an OSPF Extended Prefix Range TLV between areas, ABRs MUST set the IA-Flag (§7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the IA-Flag argument of EncodeExtPrefixRangeValueV4, internal/plugins/ospf/sr/codec.go:482-494, has no ABR caller: the IPv4 Extended Prefix originator emits only Extended Prefix TLVs and propagates no prefix range between areas, internal/plugins/ospf/ext_prefix.go:61-80 and internal/plugins/ospf/ext_prefix.go:136-160. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| `RFC8665-7.1-3` | Multiple Mapping Servers advertising Prefix-SIDs for the same prefix MUST advertise the same Prefix-SID (§7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze advertises no mapping-server Prefix-SIDs, so it enforces no consistency between mapping servers: the Prefix-SID builder emits only this router's own configured node SIDs with the M-Flag clear, internal/plugins/ospf/sr.go:197-213, and the receive path keeps one Prefix-SID per prefix and marks a second one duplicate whatever its source, internal/plugins/ospf/sr_install.go:274-278. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| `RFC8665-7.2-1` | To support SR in a multiarea environment, OSPFv2 MUST propagate Prefix-SID information between areas (§7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** OSPFv2 does not propagate a learned Prefix-SID between areas. srBuildPrefixSID attaches a Prefix-SID only when the prefix matches an entry in this router's own segment-routing configuration, internal/plugins/ospf/sr.go:202-212, so the inter-area Extended Prefix TLV an ABR originates from its self Type-3 summaries, internal/plugins/ospf/ext_prefix.go:136-160, carries no Prefix-SID for a remote prefix. Inter-area propagation exists only for IPv6, v6OriginateInterAreaSR internal/plugins/ospf/sr_interarea_v6.go:60-83. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| `RFC8665-7.4.1-1` | If a P2P-link adjacency transitions to a state lower than 2-Way, the Adj-SID Advertisement MUST be withdrawn from the area (§7.4.1) | MUST | 7.4.1 | **positive:** `unit/verify` [`TestRFC8665AdjSIDWithdrawnWhenAdjacencyDrops`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L433). **negative:** `unit/verify` [`TestRFC8665AdjSIDWithdrawKeyedByAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L468) |
| `RFC8665-10-1` | Implementations MUST ensure malformed TLVs/sub-TLVs are detected and do not provide a crash vulnerability (§10) | MUST | 10 | **positive:** `unit/verify` [`TestRFC8665WellFormedTLVsDecode`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L462). **negative:** `unit/verify` [`TestRFC8665TruncatedTLVsRejectedWithoutPanic`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L477) |
| `RFC8665-9-1` | If the length of a new TLV/sub-TLV is invalid, the LSA is considered malformed and MUST be ignored (§9) | MUST | 9 | **positive:** `unit/verify` [`TestRFC8665WellFormedTLVsDecode`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L460). **negative:** `unit/verify` [`TestRFC8665TruncatedTLVsRejectedWithoutPanic`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L472) |
| `RFC8665-3.1-6` | The SR-Algorithm TLV SHOULD only be advertised once in the RI Opaque LSA (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-3.2-9` | The Reserved field SHOULD be set to 0 on transmission (SID/Label Range, SRLB, SRMS Preference, Extended Prefix Range, Prefix-SID, Adj-SID, LAN Adj-SID) (§3.2, §3.3, §3.4, §4, §5, §6.1, §6.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-3.3-5` | Each time a SID from the SRLB is allocated, it SHOULD also be reported to all components (controller/applications) (§3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-3.4-4` | For the SRMS Preference TLV, AS-scoped flooding SHOULD be used (§3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-5-14` | PHP behavior SHOULD be done for Mapping-Server-advertised SIDs in the intra-area / inter-area / external downstream-neighbor cases described (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-7.3-1` | When an SR-capable ASBR generates Type-5 LSAs, it SHOULD also originate OSPF Extended Prefix Opaque LSAs (§7.3) | SHOULD | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-7.3-2` | When an NSSA ABR translates Type-7 LSAs into Type-5 LSAs, it SHOULD also advertise the Prefix-SID for the prefix (§7.3) | SHOULD | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-10-2` | Stronger authentication mechanisms such as RFC 7474 SHOULD be used where attackers may have network access (§10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-9-2` | Reception of malformed TLVs or sub-TLVs SHOULD be counted and/or logged for further analysis (§9, §10) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-9-3` | Logging of malformed TLVs and sub-TLVs SHOULD be rate limited to prevent a DoS attack (§9, §10) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-3.2-10` | Prefix-SIDs MAY be advertised in the form of an index (§3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-3.2-11` | The SID/Label Range TLV MAY appear multiple times (§3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-3.2-12` | Only a single SID/Label Sub-TLV MAY be advertised in the SID/Label Range TLV (§3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-3.2-13` | Multiple occurrences of the SID/Label Range TLV MAY be advertised to advertise multiple ranges (§3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-3.3-6` | SIDs from the SRLB MAY be used for Adjacency SIDs and by components other than OSPF (§3.3) | MAY | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-3.3-7` | The SRLB TLV MAY appear multiple times in the RI Opaque LSA (§3.3) | MAY | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-3.3-8` | Only a single SID/Label Sub-TLV MAY be advertised in the SRLB TLV (§3.3) | MAY | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-3.3-9` | A router advertising the SRLB TLV MAY also have other label ranges outside the SRLB for local allocation (§3.3) | MAY | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-3.4-5` | The SRMS Preference TLV MAY only be advertised once in the RI Opaque LSA (§3.4) | MAY | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-3.4-6` | If SRMS advertisements are only used inside the server's area, area-scoped flooding MAY be used (§3.4) | MAY | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-4-4` | Multiple OSPF Extended Prefix Range TLVs MAY be advertised in each Extended Prefix Opaque LSA (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-5-15` | The Prefix-SID Sub-TLV MAY appear more than once in the parent TLV (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-6.1-3` | The Adj-SID Sub-TLV MAY appear multiple times in the Extended Link TLV (§6.1) | MAY | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-6.1-4` | An SR-capable router MAY allocate an Adj-SID for each of its adjacencies and set the B-Flag when eligible for FRR protection (§6.1) | MAY | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-6.1-5` | An SR-capable router MAY allocate more than one Adj-SID to an adjacency (§6.1) | MAY | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-6.1-6` | An SR-capable router MAY allocate the same Adj-SID to different adjacencies (§6.1) | MAY | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-6.1-7` | When the G-Flag is set, the Adj-SID MAY be assigned to other adjacencies as well (§6.1) | MAY | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-6.1-8` | When the P-Flag is not set, the Adj-SID MAY be persistent (§6.1) | MAY | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-6.2-2` | The LAN Adj-SID Sub-TLV MAY appear multiple times in the Extended Link TLV (§6.2) | MAY | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-6.2-3` | When the P-Flag is not set, the LAN Adjacency SID MAY be persistent (§6.2) | MAY | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-7.1-4` | An OSPFv2 router that supports SR MAY advertise Prefix-SIDs for any prefix to which it advertises reachability (§7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-7.4.1-2` | An Adj-SID MAY be advertised for any adjacency on a P2P link in neighbor state 2-Way or higher (§7.4.1) | MAY | 7.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-7.4.1-3` | If a P2P-link adjacency transitions from the FULL state, the Adj-SID for that adjacency MAY be removed from the area (§7.4.1) | MAY | 7.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-7.4.2-1` | Each router on a broadcast/NBMA/hybrid network MAY advertise the Adj-SID for its adjacency to the DR (§7.4.2) | MAY | 7.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-7.4.2-2` | SR-capable routers MAY advertise a LAN Adjacency SID for other neighbors (BDR, DR-OTHER, etc.) on broadcast/NBMA/hybrid networks (§7.4.2) | MAY | 7.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-3.1-7` | For SR-Algorithm TLV, SID/Label Range TLV, and SRLB TLV advertisement, area-scoped flooding is REQUIRED (§3.1, §3.2, §3.3) | REQUIRED | 3.1 | **positive:** `unit/verify` [`TestRFC8665SRCapabilityTLVsAreAreaScoped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L85). **negative:** `unit/verify` [`TestRFC8665SRCapabilityTLVsAbsentFromOtherScopes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L105) |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8665-3.1-4`](#rfc8665-3.1-4) If the SR-Algorithm TLV appears in RI Opaque LSAs with different flooding scopes, use the one in the area-scoped LSA (§3.1) | {gap}, no test | the SR capability read walks every RI Opaque LSA in the LSDB and assigns the per-router entry from whichever view it reaches last, with no flooding-scope comparison -- srRemoteCapabilities iterates e.lsdb.OpaqueLSAsByType at internal/plugins/ospf/sr_install.go:238-241 and its record closure assigns caps[router] and algos[router] at internal/plugins/ospf/sr_install.go:222-229 -- so an AS-scoped RI LSA can override the area-scoped one. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| [`RFC8665-3.1-5`](#rfc8665-3.1-5) If the SR-Algorithm TLV appears in RI Opaque LSAs with the same flooding scope, use the one with the numerically smallest Instance ID and ignore subsequent instances (§3.1) | {gap}, no test | the SR capability read compares no Instance ID. The opaque view carries OpaqueID, the RFC 7770 Instance ID, but srRemoteCapabilities ignores it and the last view processed wins, internal/plugins/ospf/sr_install.go:238-241 with the assignment at internal/plugins/ospf/sr_install.go:222-229. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| [`RFC8665-3.2-8`](#rfc8665-3.2-8) When a router receives multiple overlapping ranges, it MUST conform to RFC 8660 (§3.2) | {gap}, no test | the receive path appends every decoded SID/Label Range to the originator SRGB with no overlap detection, srDecodeRemoteCapabilities internal/plugins/ospf/sr.go:337-342, and SRGB.Label maps an index by plain concatenation in advertised order, internal/plugins/ospf/sr/srgb.go:93-105, so overlapping received ranges are concatenated rather than resolved per RFC 8660. The non-overlap check covers only this router's own configured ranges, internal/plugins/ospf/sr/config.go:116-121. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| [`RFC8665-3.4-2`](#rfc8665-3.4-2) If the SRMS Preference TLV appears in RI Opaque LSAs with different flooding scopes, use the one with the narrowest flooding scope (§3.4) | {gap}, no test | the received SRMS preference is decoded into srRemoteCapabilities.SRMSPref, internal/plugins/ospf/sr.go:349-358, and nothing consumes it: srRemoteCapabilities keeps only the SRGB and the algorithm list, internal/plugins/ospf/sr_install.go:222-229, so no narrowest-flooding-scope selection exists. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| [`RFC8665-3.4-3`](#rfc8665-3.4-3) If the SRMS Preference TLV appears in RI Opaque LSAs with the same flooding scope, use the one with the numerically smallest Instance ID and ignore subsequent instances (§3.4) | {gap}, no test | the decode keeps the first SRMS Preference TLV within one LSA body, internal/plugins/ospf/sr.go:349-358, but nothing compares instances across LSAs and the preference is never consumed, internal/plugins/ospf/sr_install.go:222-229, so there is no smallest-Instance-ID tie-break. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| [`RFC8665-4-1`](#rfc8665-4-1) All prefix ranges in a single OSPF Extended Prefix Opaque LSA MUST have the same flooding scope (§4) | {gap}, no test | ze originates no OSPF Extended Prefix Range TLV for IPv4. extPrefixOnOriginate builds one Extended Prefix Opaque LSA per advertised prefix carrying a single Extended Prefix TLV, internal/plugins/ospf/ext_prefix.go:61-80, and never populates ExtPrefixLSA.Ranges; the range value encoder exists at internal/plugins/ospf/sr/codec.go:482-494 with no caller outside tests, so no code assigns a flooding scope to a prefix range or keeps the ranges in one LSA scope-uniform. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| [`RFC8665-4-2`](#rfc8665-4-2) An ABR advertising the OSPF Extended Prefix Range TLV between areas MUST set the IA-Flag (§4, §7.1) | {gap}, no test | EncodeExtPrefixRangeValueV4 takes an iaFlag argument and writes the IA-Flag bit, internal/plugins/ospf/sr/codec.go:482-494, but no ABR path calls it: the IPv4 Extended Prefix originator emits only Extended Prefix TLVs, internal/plugins/ospf/ext_prefix.go:61-80, and the only inter-area Prefix-SID propagation is the IPv6 one, internal/plugins/ospf/sr_interarea_v6.go:60-83, so no OSPFv2 ABR sets the IA-Flag. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| [`RFC8665-4-3`](#rfc8665-4-3) The Range Size MUST NOT exceed the number of prefixes satisfiable by the Prefix Length without including 224.0.0.0/3 (§4) | {gap}, no test | the Extended Prefix Range encoder writes the caller's Range Size verbatim with no capacity check against the Prefix Length and no 224.0.0.0/3 exclusion, EncodeExtPrefixRangeValueV4 internal/plugins/ospf/sr/codec.go:482-494, and the decoder reads it back unchecked, internal/plugins/ospf/sr/codec.go:497-523. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| [`RFC8665-5-8`](#rfc8665-5-8) The NP-Flag MUST be set and the E-Flag MUST be clear for Prefix-SIDs allocated to inter-area prefixes originated by the ABR, unless the advertised prefix is directly attached to the ABR (§5) | {gap}, no test | the IPv4 Prefix-SID builder copies the NP and E flags straight from configuration and never forces NP set with E clear for an inter-area prefix originated by an ABR, srBuildPrefixSID internal/plugins/ospf/sr.go:197-213, which matches only on the configured prefix and ignores the ctx.RouteType it is handed. The equivalent rule exists only for IPv6, v6InterAreaPrefixSIDRule internal/plugins/ospf/sr_interarea_v6.go:35-43. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| [`RFC8665-5-9`](#rfc8665-5-9) The NP-Flag MUST be set and the E-Flag MUST be clear for Prefix-SIDs allocated to redistributed prefixes, unless the redistributed prefix is directly attached to the ASBR (§5) | {gap}, no test | the same builder applies no NP-set / E-clear rule to a redistributed prefix, srBuildPrefixSID internal/plugins/ospf/sr.go:197-213; the AS-external Extended Prefix advertisement carries whatever flags the prefix-sid configuration sets, internal/plugins/ospf/ext_prefix.go:162-176. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| [`RFC8665-7.1-1`](#rfc8665-7.1-1) An SR Mapping Server MUST use the OSPF Extended Prefix Range TLV when advertising SIDs for prefixes (§7.1) | {gap}, no test | ze runs no SR Mapping Server for IPv4. Nothing originates an Extended Prefix Range TLV into an Extended Prefix Opaque LSA -- ExtPrefixLSA.Ranges is populated only by the decoder, internal/plugins/ospf/packet/ext_prefix.go:165-168, and read only by the show path, internal/plugins/ospf/ext_render.go:106 -- and the M-Flag is never set on an originated Prefix-SID, internal/plugins/ospf/sr.go:204-208. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| [`RFC8665-7.1-2`](#rfc8665-7.1-2) When propagating an OSPF Extended Prefix Range TLV between areas, ABRs MUST set the IA-Flag (§7.1) | {gap}, no test | the IA-Flag argument of EncodeExtPrefixRangeValueV4, internal/plugins/ospf/sr/codec.go:482-494, has no ABR caller: the IPv4 Extended Prefix originator emits only Extended Prefix TLVs and propagates no prefix range between areas, internal/plugins/ospf/ext_prefix.go:61-80 and internal/plugins/ospf/ext_prefix.go:136-160. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| [`RFC8665-7.1-3`](#rfc8665-7.1-3) Multiple Mapping Servers advertising Prefix-SIDs for the same prefix MUST advertise the same Prefix-SID (§7.1) | {gap}, no test | ze advertises no mapping-server Prefix-SIDs, so it enforces no consistency between mapping servers: the Prefix-SID builder emits only this router's own configured node SIDs with the M-Flag clear, internal/plugins/ospf/sr.go:197-213, and the receive path keeps one Prefix-SID per prefix and marks a second one duplicate whatever its source, internal/plugins/ospf/sr_install.go:274-278. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| [`RFC8665-7.2-1`](#rfc8665-7.2-1) To support SR in a multiarea environment, OSPFv2 MUST propagate Prefix-SID information between areas (§7.2) | {gap}, no test | OSPFv2 does not propagate a learned Prefix-SID between areas. srBuildPrefixSID attaches a Prefix-SID only when the prefix matches an entry in this router's own segment-routing configuration, internal/plugins/ospf/sr.go:202-212, so the inter-area Extended Prefix TLV an ABR originates from its self Type-3 summaries, internal/plugins/ospf/ext_prefix.go:136-160, carries no Prefix-SID for a remote prefix. Inter-area propagation exists only for IPv6, v6OriginateInterAreaSR internal/plugins/ospf/sr_interarea_v6.go:60-83. Disclosed in docs/features/rfc-status.md RFC 8665 row |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8665-3.1-1`](#rfc8665-3.1-1)

If the SR-Algorithm TLV is advertised, Algorithm 0 MUST be included (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665NoSRAlgorithmTLVWhenSRUnconfigured`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L73) | unit/verify | unproven |
| positive | [`TestRFC8665SRAlgorithmTLVAdvertisesAlgorithmZeroOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L44) | unit/verify | unproven |

### [`RFC8665-3.1-2`](#rfc8665-3.1-2)

Local policy at a node claiming support for Algorithm 1 MUST NOT alter the SPF paths computed by Algorithm 1 (§3.1, §8.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8665SRAlgorithmTLVAdvertisesAlgorithmZeroOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L47) | unit/verify | unproven |

### [`RFC8665-3.1-3`](#rfc8665-3.1-3)

When multiple SR-Algorithm TLVs are received from a router, use the first occurrence of the TLV in the RI Opaque LSA (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665RepeatedAlgorithmAndSRMSInstancesIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L138) | unit/verify | unproven |
| positive | [`TestRFC8665SingleAlgorithmAndSRMSInstanceUsed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L119) | unit/verify | unproven |

### [`RFC8665-3.1-4`](#rfc8665-3.1-4)

If the SR-Algorithm TLV appears in RI Opaque LSAs with different flooding scopes, use the one in the area-scoped LSA (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8665-3.1-4, so no unit is bound to it.

### [`RFC8665-3.1-5`](#rfc8665-3.1-5)

If the SR-Algorithm TLV appears in RI Opaque LSAs with the same flooding scope, use the one with the numerically smallest Instance ID and ignore subsequent instances (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8665-3.1-5, so no unit is bound to it.

### [`RFC8665-3.2-1`](#rfc8665-3.2-1)

Range Size in the SID/Label Range TLV MUST be greater than 0 (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665RangeSizeZeroRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L54) | unit/verify | unproven |
| positive | [`TestRFC8665RangeTLVRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L22) | unit/verify | unproven |

### [`RFC8665-3.2-2`](#rfc8665-3.2-2)

The SID/Label Sub-TLV MUST be included in the SID/Label Range TLV (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665RangeWithoutSIDLabelSubTLVRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L81) | unit/verify | unproven |
| positive | [`TestRFC8665RangeTLVRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L27) | unit/verify | unproven |

### [`RFC8665-3.2-3`](#rfc8665-3.2-3)

If more than one SID/Label Sub-TLV is present in the SID/Label Range TLV, the TLV MUST be ignored (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665RangeWithTwoSIDLabelSubTLVsIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L107) | unit/verify | unproven |
| positive | [`TestRFC8665RangeWithSingleSIDLabelAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L96) | unit/verify | unproven |

### [`RFC8665-3.2-4`](#rfc8665-3.2-4)

When advertising multiple ranges, the originating router MUST encode each range into a different SID/Label Range TLV (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8665EachRangeInItsOwnTLV`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L161) | unit/verify | unproven |

### [`RFC8665-3.2-5`](#rfc8665-3.2-5)

The originating router MUST ensure the SID/Label Range TLV order is the same after a graceful restart (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8665RangeOrderStableAcrossRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L189) | unit/verify | unproven |

### [`RFC8665-3.2-6`](#rfc8665-3.2-6)

The receiving router MUST adhere to the advertised range order when calculating a SID/Label from a SID index (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665SRGBIndexOutOfRangeAndOrderSensitivity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L142) | unit/verify | unproven |
| positive | [`TestRFC8665SRGBIndexUsesAdvertisedOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L124) | unit/verify | unproven |

### [`RFC8665-3.2-7`](#rfc8665-3.2-7)

The originating router MUST NOT advertise overlapping ranges (SID/Label Range TLV) (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665OverlappingRangesRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L175) | unit/verify | unproven |
| positive | [`TestRFC8665NonOverlappingRangesAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L159) | unit/verify | unproven |

### [`RFC8665-3.2-8`](#rfc8665-3.2-8)

When a router receives multiple overlapping ranges, it MUST conform to RFC 8660 (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8665-3.2-8, so no unit is bound to it.

### [`RFC8665-3.3-1`](#rfc8665-3.3-1)

Range Size in the SRLB TLV MUST be greater than 0 (§3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665RangeSizeZeroRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L58) | unit/verify | unproven |
| positive | [`TestRFC8665RangeTLVRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L24) | unit/verify | unproven |

### [`RFC8665-3.3-2`](#rfc8665-3.3-2)

The SID/Label Sub-TLV MUST be included in the SRLB TLV (§3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665RangeWithoutSIDLabelSubTLVRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L84) | unit/verify | unproven |
| positive | [`TestRFC8665RangeTLVRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L30) | unit/verify | unproven |

### [`RFC8665-3.3-3`](#rfc8665-3.3-3)

If more than one SID/Label Sub-TLV is present in the SRLB TLV, the SRLB TLV MUST be ignored (§3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665RangeWithTwoSIDLabelSubTLVsIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L110) | unit/verify | unproven |
| positive | [`TestRFC8665RangeWithSingleSIDLabelAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L98) | unit/verify | unproven |

### [`RFC8665-3.3-4`](#rfc8665-3.3-4)

The originating router MUST NOT advertise overlapping ranges (SRLB TLV) (§3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665OverlappingRangesRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L178) | unit/verify | unproven |
| positive | [`TestRFC8665NonOverlappingRangesAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L162) | unit/verify | unproven |

### [`RFC8665-3.4-1`](#rfc8665-3.4-1)

When multiple SRMS Preference TLVs are received from a router, use the first occurrence of the TLV in the RI Opaque LSA (§3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665RepeatedAlgorithmAndSRMSInstancesIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L141) | unit/verify | unproven |
| positive | [`TestRFC8665SingleAlgorithmAndSRMSInstanceUsed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L121) | unit/verify | unproven |

### [`RFC8665-3.4-2`](#rfc8665-3.4-2)

If the SRMS Preference TLV appears in RI Opaque LSAs with different flooding scopes, use the one with the narrowest flooding scope (§3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8665-3.4-2, so no unit is bound to it.

### [`RFC8665-3.4-3`](#rfc8665-3.4-3)

If the SRMS Preference TLV appears in RI Opaque LSAs with the same flooding scope, use the one with the numerically smallest Instance ID and ignore subsequent instances (§3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8665-3.4-3, so no unit is bound to it.

### [`RFC8665-4-1`](#rfc8665-4-1)

All prefix ranges in a single OSPF Extended Prefix Opaque LSA MUST have the same flooding scope (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8665-4-1, so no unit is bound to it.

### [`RFC8665-4-2`](#rfc8665-4-2)

An ABR advertising the OSPF Extended Prefix Range TLV between areas MUST set the IA-Flag (§4, §7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8665-4-2, so no unit is bound to it.

### [`RFC8665-4-3`](#rfc8665-4-3)

The Range Size MUST NOT exceed the number of prefixes satisfiable by the Prefix Length without including 224.0.0.0/3 (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8665-4-3, so no unit is bound to it.

### [`RFC8665-5-1`](#rfc8665-5-1)

Reserved bits (other than NP/M/E/V/L) in the Prefix-SID Flags MUST be zero when sent and are ignored when received (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665PrefixSIDReservedFlagBitsIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L224) | unit/verify | unproven |
| positive | [`TestRFC8665PrefixSIDReservedFlagBitsZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L209) | unit/verify | unproven |

### [`RFC8665-5-2`](#rfc8665-5-2)

If the NP-Flag is set, the penultimate hop MUST NOT pop the Prefix-SID before delivering to the advertising node (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665PHPPopsWhenNoPHPFlagClear`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L331) | unit/verify | unproven |
| positive | [`TestRFC8665NoPHPKeepsPrefixSIDLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L306) | unit/verify | unproven |

### [`RFC8665-5-3`](#rfc8665-5-3)

If the E-Flag is set, any upstream neighbor MUST replace the Prefix-SID with the Explicit NULL label (0 for IPv4) before forwarding (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665NoPHPKeepsPrefixSIDLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L311) | unit/verify | unproven |
| positive | [`TestRFC8665ExplicitNullReplacesPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L344) | unit/verify | unproven |

### [`RFC8665-5-4`](#rfc8665-5-4)

A router receiving a Prefix-SID with an algorithm value not advertised in the remote node's SR-Algorithm TLV MUST ignore the Prefix-SID Sub-TLV (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665PrefixSIDIgnoredWhenAlgorithmNotAdvertised`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L286) | unit/verify | unproven |
| positive | [`TestRFC8665PrefixSIDInstalledWhenAlgorithmAdvertised`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L258) | unit/verify | unproven |

### [`RFC8665-5-5`](#rfc8665-5-5)

Any invalid combination of V- and L-Flags in a received SID Advertisement MUST cause it to be ignored (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665InvalidVLCombinationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L279) | unit/verify | unproven |
| positive | [`TestRFC8665ValidVLCombinationsDecode`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L247) | unit/verify | unproven |

### [`RFC8665-5-6`](#rfc8665-5-6)

If an OSPF router advertises multiple Prefix-SIDs for the same prefix, topology, and algorithm, all of them MUST be ignored (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665DuplicatePrefixSIDsAllIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L329) | unit/verify | unproven |
| positive | [`TestRFC8665PrefixSIDInstalledWhenAlgorithmAdvertised`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L261) | unit/verify | unproven |

### [`RFC8665-5-7`](#rfc8665-5-7)

When calculating the outgoing label, the router MUST take into account the next-hop router's E-, NP-, and M-Flags if that router advertised the SID, regardless of whether it contributes to the best path (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665OriginatorFlagsNotAppliedAtTransitHop`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L401) | unit/verify | unproven |
| positive | [`TestRFC8665NextHopFlagsAppliedWhereSIDAdvertised`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L360) | unit/verify | unproven |

### [`RFC8665-5-8`](#rfc8665-5-8)

The NP-Flag MUST be set and the E-Flag MUST be clear for Prefix-SIDs allocated to inter-area prefixes originated by the ABR, unless the advertised prefix is directly attached to the ABR (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8665-5-8, so no unit is bound to it.

### [`RFC8665-5-9`](#rfc8665-5-9)

The NP-Flag MUST be set and the E-Flag MUST be clear for Prefix-SIDs allocated to redistributed prefixes, unless the redistributed prefix is directly attached to the ASBR (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8665-5-9, so no unit is bound to it.

### [`RFC8665-5-10`](#rfc8665-5-10)

If the NP-Flag is not set, any upstream neighbor of the Prefix-SID originator MUST pop the Prefix-SID (PHP) and the received E-Flag is ignored (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665NoPHPKeepsPrefixSIDLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L315) | unit/verify | unproven |
| positive | [`TestRFC8665PHPPopsWhenNoPHPFlagClear`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L328) | unit/verify | unproven |

### [`RFC8665-5-11`](#rfc8665-5-11)

If the NP-Flag is set and the E-Flag is not set, any upstream neighbor MUST keep the Prefix-SID on top of the stack (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665ExplicitNullReplacesPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L349) | unit/verify | unproven |
| positive | [`TestRFC8665NoPHPKeepsPrefixSIDLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L309) | unit/verify | unproven |

### [`RFC8665-5-12`](#rfc8665-5-12)

If both NP-Flag and E-Flag are set, any upstream neighbor MUST replace the Prefix-SID with an Explicit NULL label (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665NoPHPKeepsPrefixSIDLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L313) | unit/verify | unproven |
| positive | [`TestRFC8665ExplicitNullReplacesPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L347) | unit/verify | unproven |

### [`RFC8665-5-13`](#rfc8665-5-13)

When the M-Flag is set, the NP-Flag and the E-Flag MUST be ignored on reception (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665MappingServerFlagClearHonorsNPAndE`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L376) | unit/verify | unproven |
| positive | [`TestRFC8665MappingServerFlagIgnoresNPAndE`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L362) | unit/verify | unproven |

### [`RFC8665-6.1-1`](#rfc8665-6.1-1)

Reserved bits (5-7) in the Adj-SID Flags MUST be zero when sent and are ignored when received (§6.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665AdjSIDReservedFlagBitsIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L408) | unit/verify | unproven |
| positive | [`TestRFC8665AdjSIDReservedFlagBitsZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L389) | unit/verify | unproven |

### [`RFC8665-6.1-2`](#rfc8665-6.1-2)

When the P-Flag is set, the Adj-SID MUST be persistent (§6.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8665AdjSIDNeverClaimsPersistence`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L215) | unit/verify | unproven |

### [`RFC8665-6.2-1`](#rfc8665-6.2-1)

When the P-Flag is set, the LAN Adjacency SID MUST be persistent (§6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8665AdjSIDNeverClaimsPersistence`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L221) | unit/verify | unproven |

### [`RFC8665-7.1-1`](#rfc8665-7.1-1)

An SR Mapping Server MUST use the OSPF Extended Prefix Range TLV when advertising SIDs for prefixes (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8665-7.1-1, so no unit is bound to it.

### [`RFC8665-7.1-2`](#rfc8665-7.1-2)

When propagating an OSPF Extended Prefix Range TLV between areas, ABRs MUST set the IA-Flag (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8665-7.1-2, so no unit is bound to it.

### [`RFC8665-7.1-3`](#rfc8665-7.1-3)

Multiple Mapping Servers advertising Prefix-SIDs for the same prefix MUST advertise the same Prefix-SID (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8665-7.1-3, so no unit is bound to it.

### [`RFC8665-7.2-1`](#rfc8665-7.2-1)

To support SR in a multiarea environment, OSPFv2 MUST propagate Prefix-SID information between areas (§7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8665-7.2-1, so no unit is bound to it.

### [`RFC8665-7.4.1-1`](#rfc8665-7.4.1-1)

If a P2P-link adjacency transitions to a state lower than 2-Way, the Adj-SID Advertisement MUST be withdrawn from the area (§7.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665AdjSIDWithdrawKeyedByAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L468) | unit/verify | unproven |
| positive | [`TestRFC8665AdjSIDWithdrawnWhenAdjacencyDrops`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L433) | unit/verify | unproven |

### [`RFC8665-10-1`](#rfc8665-10-1)

Implementations MUST ensure malformed TLVs/sub-TLVs are detected and do not provide a crash vulnerability (§10)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665TruncatedTLVsRejectedWithoutPanic`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L477) | unit/verify | unproven |
| positive | [`TestRFC8665WellFormedTLVsDecode`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L462) | unit/verify | unproven |

### [`RFC8665-9-1`](#rfc8665-9-1)

If the length of a new TLV/sub-TLV is invalid, the LSA is considered malformed and MUST be ignored (§9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665TruncatedTLVsRejectedWithoutPanic`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L472) | unit/verify | unproven |
| positive | [`TestRFC8665WellFormedTLVsDecode`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L460) | unit/verify | unproven |

### [`RFC8665-3.1-7`](#rfc8665-3.1-7)

For SR-Algorithm TLV, SID/Label Range TLV, and SRLB TLV advertisement, area-scoped flooding is REQUIRED (§3.1, §3.2, §3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665SRCapabilityTLVsAbsentFromOtherScopes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L105) | unit/verify | unproven |
| positive | [`TestRFC8665SRCapabilityTLVsAreAreaScoped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L85) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 8665, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 8665, so its obligations are stated where they were written.
