# RFC 8665 - OSPF Extensions for Segment Routing

Partial. Every requirement this repository extracted from RFC 8665, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 60.4% | 29 of 48 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 10.4% | 5 of 48 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 48 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 48 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 42.7% | 41 of 96 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 48 | of 83 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 48 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 48 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 48 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 48 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 29.2% | 14 of 48 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 48 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 83 |
| Gated MUST-level | 48 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 14 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 96 |
| Tagged units | 96 |
| Recorded audit verdicts | 29 |
| Discrimination records | 41 |
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
- **SR Mapping Server and prefix ranges:** [`RFC8665-4-1`](#rfc8665-4-1), 4-2, 4-3, 7.1-1, 7.1-2, 7.1-3 (ze originates no IPv4 Extended Prefix Range TLV; the value encoder at [`internal/plugins/ospf/sr/codec.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/codec.go) has no ABR caller, so the IA-Flag is never set and the Range Size capacity rule is unenforced). ABR / ASBR Prefix-SID flags and inter-area propagation: [`RFC8665-5-8`](#rfc8665-5-8), 5-9, 7.2-1 (the IPv4 Prefix-SID builder copies the configured NP/E flags and advertises only locally configured prefixes, [`internal/plugins/ospf/sr.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr.go); the equivalent rules exist only for IPv6 at [`internal/plugins/ospf/sr_interarea_v6.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_interarea_v6.go)). The feature also remains pre-production pending hardening and deployment evidence. The 2026-09-21 extraction walk added [`RFC8665-3.2-14`](#rfc8665-3.2-14), the reception half of the seven "Reserved: SHOULD be set to 0 on transmission and MUST be ignored on reception" field descriptions, which the checklist had carried only as the transmission-side SHOULD ([`RFC8665-3.2-9`](#rfc8665-3.2-9)); that MUST carries no test yet and is unannotated, because an annotation on a row is the owner's to write.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 29 | one part of the gated population |
| Annotated (including scoped evidence) | 19 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **48** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (29):** [`RFC8665-3.1-1`](#rfc8665-3.1-1), [`RFC8665-3.1-3`](#rfc8665-3.1-3), [`RFC8665-3.2-1`](#rfc8665-3.2-1), [`RFC8665-3.2-2`](#rfc8665-3.2-2), [`RFC8665-3.2-3`](#rfc8665-3.2-3), [`RFC8665-3.2-6`](#rfc8665-3.2-6), [`RFC8665-3.2-7`](#rfc8665-3.2-7), [`RFC8665-3.2-14`](#rfc8665-3.2-14), [`RFC8665-3.3-1`](#rfc8665-3.3-1), [`RFC8665-3.3-2`](#rfc8665-3.3-2), [`RFC8665-3.3-3`](#rfc8665-3.3-3), [`RFC8665-3.3-4`](#rfc8665-3.3-4), [`RFC8665-3.4-1`](#rfc8665-3.4-1), [`RFC8665-5-1`](#rfc8665-5-1), [`RFC8665-5-2`](#rfc8665-5-2), [`RFC8665-5-3`](#rfc8665-5-3), [`RFC8665-5-4`](#rfc8665-5-4), [`RFC8665-5-5`](#rfc8665-5-5), [`RFC8665-5-6`](#rfc8665-5-6), [`RFC8665-5-7`](#rfc8665-5-7), [`RFC8665-5-10`](#rfc8665-5-10), [`RFC8665-5-11`](#rfc8665-5-11), [`RFC8665-5-12`](#rfc8665-5-12), [`RFC8665-5-13`](#rfc8665-5-13), [`RFC8665-6.1-1`](#rfc8665-6.1-1), [`RFC8665-7.4.1-1`](#rfc8665-7.4.1-1), [`RFC8665-10-1`](#rfc8665-10-1), [`RFC8665-9-1`](#rfc8665-9-1), [`RFC8665-3.1-7`](#rfc8665-3.1-7)

**Annotated (including scoped evidence) (19):** [`RFC8665-3.1-2`](#rfc8665-3.1-2), [`RFC8665-3.1-4`](#rfc8665-3.1-4), [`RFC8665-3.1-5`](#rfc8665-3.1-5), [`RFC8665-3.2-4`](#rfc8665-3.2-4), [`RFC8665-3.2-5`](#rfc8665-3.2-5), [`RFC8665-3.2-8`](#rfc8665-3.2-8), [`RFC8665-3.4-2`](#rfc8665-3.4-2), [`RFC8665-3.4-3`](#rfc8665-3.4-3), [`RFC8665-4-1`](#rfc8665-4-1), [`RFC8665-4-2`](#rfc8665-4-2), [`RFC8665-4-3`](#rfc8665-4-3), [`RFC8665-5-8`](#rfc8665-5-8), [`RFC8665-5-9`](#rfc8665-5-9), [`RFC8665-6.1-2`](#rfc8665-6.1-2), [`RFC8665-6.2-1`](#rfc8665-6.2-1), [`RFC8665-7.1-1`](#rfc8665-7.1-1), [`RFC8665-7.1-2`](#rfc8665-7.1-2), [`RFC8665-7.1-3`](#rfc8665-7.1-3), [`RFC8665-7.2-1`](#rfc8665-7.2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8665-3.1-1` | If the SR-Algorithm TLV is advertised, Algorithm 0 MUST be included (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC8665SRAlgorithmTLVAdvertisesAlgorithmZeroOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L44). **negative:** `unit/verify` [`TestRFC8665NoSRAlgorithmTLVWhenSRUnconfigured`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L73) |
| `RFC8665-3.1-2` | Local policy at the node claiming support for Algorithm 1 MUST NOT alter the SPF paths computed by Algorithm 1. (§3.1, §8.5) | MUST NOT | 3.1 | **positive:** `unit/verify` [`TestRFC8665SRAlgorithmTLVAdvertisesAlgorithmZeroOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L47). **negative:** no negative test. **{single-polarity}:** ze advertises the single-entry algorithm list 0 and never claims Algorithm 1 -- srBuildAlgorithm encodes that literal list, internal/plugins/ospf/sr.go:154-160 -- and the installer refuses any Prefix-SID whose algorithm is not 0, internal/plugins/ospf/sr_install.go:89-92. There is no Algorithm 1 SPF computation to alter and no violating input to reject, so only the positive direction is meaningful |
| `RFC8665-3.1-3` | When multiple SR-Algorithm TLVs are received from a given router, the receiver MUST use the first occurrence of the TLV in the Router Information Opaque LSA. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC8665SingleAlgorithmAndSRMSInstanceUsed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L119). **negative:** `unit/verify` [`TestRFC8665RepeatedAlgorithmAndSRMSInstancesIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L138) |
| `RFC8665-3.1-4` | If the SR-Algorithm TLV appears in multiple Router Information Opaque LSAs that have different flooding scopes, the SR-Algorithm TLV in the Router Information Opaque LSA with the area-scoped flooding scope MUST be used. (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the SR capability read walks every RI Opaque LSA in the LSDB and assigns the per-router entry from whichever view it reaches last, with no flooding-scope comparison -- srRemoteCapabilities iterates e.lsdb.OpaqueLSAsByType at internal/plugins/ospf/sr_install.go:238-241 and its record closure assigns caps[router] and algos[router] at internal/plugins/ospf/sr_install.go:222-229 -- so an AS-scoped RI LSA can override the area-scoped one. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| `RFC8665-3.1-5` | If the SR-Algorithm TLV appears in multiple Router Information Opaque LSAs that have the same flooding scope, the SR-Algorithm TLV in the Router Information (RI) Opaque LSA with the numerically smallest Instance ID MUST be used and subsequent instances of the SR-Algorithm TLV MUST be ignored. (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the SR capability read compares no Instance ID. The opaque view carries OpaqueID, the RFC 7770 Instance ID, but srRemoteCapabilities ignores it and the last view processed wins, internal/plugins/ospf/sr_install.go:238-241 with the assignment at internal/plugins/ospf/sr_install.go:222-229. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| `RFC8665-3.2-1` | Range Size: 3-octet SID/label range size (i.e., the number of SIDs or labels in the range including the first SID/label). It MUST be greater than 0. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC8665RangeTLVRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L22). **negative:** `unit/verify` [`TestRFC8665RangeSizeZeroRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L54) |
| `RFC8665-3.2-2` | The SID/Label Sub-TLV MUST be included in the SID/Label Range TLV (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC8665RangeTLVRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L27). **negative:** `unit/verify` [`TestRFC8665RangeWithoutSIDLabelSubTLVRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L81) |
| `RFC8665-3.2-3` | If more than one SID/Label Sub-TLV is present, the SID/ Label Range TLV MUST be ignored. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC8665RangeWithSingleSIDLabelAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L96). **negative:** `unit/verify` [`TestRFC8665RangeWithTwoSIDLabelSubTLVsIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L107) |
| `RFC8665-3.2-4` | * The originating router MUST encode each range into a different SID/Label Range TLV. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC8665EachRangeInItsOwnTLV`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L161). **negative:** no negative test. **{single-polarity}:** srBuildSRGB emits one packet.RITLV per configured range, internal/plugins/ospf/sr.go:167-172, so the encoder cannot express two ranges in one TLV and has no violating output to produce. The receive-side rejection of a range TLV carrying two SID/Label sub-TLVs is the RFC8665-3.2-3 negative test |
| `RFC8665-3.2-5` | The originating router MUST ensure the order is the same after a graceful restart (using checkpointing, nonvolatile storage, or any other mechanism) in order to ensure the SID/Label range and SID index correspondence is preserved across graceful restarts. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC8665RangeOrderStableAcrossRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L189). **positive:** `unit/verify` [`TestRFC8665RangeOrderStableThroughConfigRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_range_order_test.go#L34). **negative:** no negative test. **{single-polarity}:** the advertised order is a pure function of the configured SRGB slice -- srBuildSRGB walks it in slice order with no sort and no map iteration, and parseSegmentRouting resolves the one YANG srgb container into a one-element slice on every start (applySRConfig, internal/plugins/ospf/sr_config.go) -- so a restart reproduces the same order by construction and there is no reordered input to reject |
| `RFC8665-3.2-6` | * The receiving router MUST adhere to the order in which the ranges are advertised when calculating a SID/Label from a SID index. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC8665ReceivedRangesMappedInAdvertisedOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_srgb_advertised_order_test.go#L16). **positive:** `unit/verify` [`TestRFC8665SRGBIndexUsesAdvertisedOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L124). **negative:** `unit/verify` [`TestRFC8665SRGBIndexOutOfRangeAndOrderSensitivity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L142) |
| `RFC8665-3.2-7` | * The originating router MUST NOT advertise overlapping ranges. (§3.2) | MUST NOT | 3.2 | **positive:** `unit/verify` [`TestRFC8665NonOverlappingRangesAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L159). **negative:** `unit/verify` [`TestRFC8665OverlappingRangesRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L175) |
| `RFC8665-3.2-8` | * When a router receives multiple overlapping ranges, it MUST conform to the procedures defined in [RFC8660]. (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the receive path appends every decoded SID/Label Range to the originator SRGB with no overlap detection, srDecodeRemoteCapabilities internal/plugins/ospf/sr.go:337-342, and SRGB.Label maps an index by plain concatenation in advertised order, internal/plugins/ospf/sr/srgb.go:93-105, so overlapping received ranges are concatenated rather than resolved per RFC 8660. The non-overlap check covers only this router's own configured ranges, internal/plugins/ospf/sr/config.go:116-121. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| `RFC8665-3.2-14` | Reserved: SHOULD be set to 0 on transmission and MUST be ignored on reception (§3.2, §3.3, §3.4, §4, §5, §6.1, §6.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC8665ReservedFieldIgnoredOnReception`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_reserved_test.go#L51). **negative:** `unit/verify` [`TestRFC8665NonReservedOctetIsRead`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_reserved_test.go#L75) |
| `RFC8665-3.3-1` | Range Size: 3-octet SID/Label range size (i.e., the number of SIDs or labels in the range including the first SID/Label). It MUST be greater than 0. (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestRFC8665RangeTLVRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L24). **negative:** `unit/verify` [`TestRFC8665RangeSizeZeroRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L58) |
| `RFC8665-3.3-2` | The SID/Label Sub-TLV MUST be included in the SRLB TLV (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestRFC8665RangeTLVRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L30). **negative:** `unit/verify` [`TestRFC8665RangeWithoutSIDLabelSubTLVRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L84) |
| `RFC8665-3.3-3` | If more than one SID/Label Sub-TLV is present, the SRLB TLV MUST be ignored. (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestRFC8665RangeWithSingleSIDLabelAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L98). **negative:** `unit/verify` [`TestRFC8665RangeWithTwoSIDLabelSubTLVsIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L110) |
| `RFC8665-3.3-4` | The originating router MUST NOT advertise overlapping ranges. (§3.3) | MUST NOT | 3.3 | **positive:** `unit/verify` [`TestRFC8665NonOverlappingRangesAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L162). **negative:** `unit/verify` [`TestRFC8665OverlappingRangesRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L178) |
| `RFC8665-3.4-1` | When multiple SRMS Preference TLVs are received from a given router, the receiver MUST use the first occurrence of the TLV in the Router Information Opaque LSA. (§3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestRFC8665SingleAlgorithmAndSRMSInstanceUsed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L121). **negative:** `unit/verify` [`TestRFC8665RepeatedAlgorithmAndSRMSInstancesIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L141) |
| `RFC8665-3.4-2` | If the SRMS Preference TLV appears in multiple Router Information Opaque LSAs that have different flooding scopes, the SRMS Preference TLV in the Router Information Opaque LSA with the narrowest flooding scope MUST be used. (§3.4) | MUST | 3.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the received SRMS preference is decoded into srRemoteCapabilities.SRMSPref, internal/plugins/ospf/sr.go:349-358, and nothing consumes it: srRemoteCapabilities keeps only the SRGB and the algorithm list, internal/plugins/ospf/sr_install.go:222-229, so no narrowest-flooding-scope selection exists. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| `RFC8665-3.4-3` | If the SRMS Preference TLV appears in multiple Router Information Opaque LSAs that have the same flooding scope, the SRMS Preference TLV in the Router Information Opaque LSA with the numerically smallest Instance ID MUST be used and subsequent instances of the SRMS Preference TLV MUST be ignored. (§3.4) | MUST | 3.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the decode keeps the first SRMS Preference TLV within one LSA body, internal/plugins/ospf/sr.go:349-358, but nothing compares instances across LSAs and the preference is never consumed, internal/plugins/ospf/sr_install.go:222-229, so there is no smallest-Instance-ID tie-break. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| `RFC8665-4-1` | all prefix ranges included in a single OSPF Extended Prefix Opaque LSA MUST have the same flooding scope. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze originates no OSPF Extended Prefix Range TLV for IPv4. extPrefixOnOriginate builds one Extended Prefix Opaque LSA per advertised prefix carrying a single Extended Prefix TLV, internal/plugins/ospf/ext_prefix.go:61-80, and never populates ExtPrefixLSA.Ranges; the range value encoder exists at internal/plugins/ospf/sr/codec.go:482-494 with no caller outside tests, so no code assigns a flooding scope to a prefix range or keeps the ranges in one LSA scope-uniform. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| `RFC8665-4-2` | An Area Border Router (ABR) that is advertising the OSPF Extended Prefix Range TLV between areas MUST set this bit. (§4, §7.1) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** EncodeExtPrefixRangeValueV4 takes an iaFlag argument and writes the IA-Flag bit, internal/plugins/ospf/sr/codec.go:482-494, but no ABR path calls it: the IPv4 Extended Prefix originator emits only Extended Prefix TLVs, internal/plugins/ospf/ext_prefix.go:61-80, and the only inter-area Prefix-SID propagation is the IPv6 one, internal/plugins/ospf/sr_interarea_v6.go:60-83, so no OSPFv2 ABR sets the IA-Flag. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| `RFC8665-4-3` | The Range Size MUST NOT exceed the number of prefixes that could be satisfied by the Prefix Length without including the IPv4 multicast address range (224.0.0.0/3). (§4) | MUST NOT | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Extended Prefix Range encoder writes the caller's Range Size verbatim with no capacity check against the Prefix Length and no 224.0.0.0/3 exclusion, EncodeExtPrefixRangeValueV4 internal/plugins/ospf/sr/codec.go:482-494, and the decoder reads it back unchecked, internal/plugins/ospf/sr/codec.go:497-523. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| `RFC8665-5-1` | Other bits: Reserved. These MUST be zero when sent and are ignored when received. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC8665PrefixSIDReservedFlagBitsZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L209). **negative:** `unit/verify` [`TestRFC8665PrefixSIDReservedFlagBitsIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L224) |
| `RFC8665-5-2` | NP-Flag: No-PHP (Penultimate Hop Popping) Flag. If set, then the penultimate hop MUST NOT pop the Prefix-SID before delivering packets to the node that advertised the Prefix-SID. (§5) | MUST NOT | 5 | **positive:** `unit/verify` [`TestRFC8665NoPHPKeepsPrefixSIDLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L306). **negative:** `unit/verify` [`TestRFC8665PHPPopsWhenNoPHPFlagClear`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L331) |
| `RFC8665-5-3` | E-Flag: Explicit Null Flag. If set, any upstream neighbor of the Prefix-SID originator MUST replace the Prefix-SID with the Explicit NULL label (0 for IPv4) before forwarding the packet. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC8665ExplicitNullPushedAsLabelZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_explicit_null_label_test.go#L16). **positive:** `unit/verify` [`TestRFC8665ExplicitNullReplacesPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L344). **negative:** `unit/verify` [`TestRFC8665NoExplicitNullWithoutEFlag`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_explicit_null_label_test.go#L31). **negative:** `unit/verify` [`TestRFC8665NoPHPKeepsPrefixSIDLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L311) |
| `RFC8665-5-4` | A router receiving a Prefix-SID from a remote node and with an algorithm value that the remote node has not advertised in the SR-Algorithm TLV (Section 3.1) MUST ignore the Prefix-SID Sub- TLV. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC8665PrefixSIDInstalledWhenAlgorithmAdvertised`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L258). **negative:** `unit/verify` [`TestRFC8665PrefixSIDIgnoredWhenAlgorithmNotAdvertised`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L286) |
| `RFC8665-5-5` | All other combinations of V-Flag and L-Flag are invalid and any SID Advertisement received with an invalid setting for V- and L-Flags MUST be ignored. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC8665ValidVLCombinationsDecode`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L247). **negative:** `unit/verify` [`TestRFC8665InvalidVLCombinationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L279) |
| `RFC8665-5-6` | If an OSPF router advertises multiple Prefix-SIDs for the same prefix, topology, and algorithm, all of them MUST be ignored (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC8665OneRouterMultiplePrefixSIDsAllIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_prefix_sid_per_router_test.go#L67). **positive:** `unit/verify` [`TestRFC8665PrefixSIDInstalledWhenAlgorithmAdvertised`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L261). **positive:** `unit/verify` [`TestRFC8665PrefixSIDsOfDifferentAlgorithmsNotDuplicate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_ext_malformed_test.go#L218). **positive:** `unit/verify` [`TestRFC8665TILFAIgnoresDuplicatePrefixSIDs`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_tilfa_duplicate_test.go#L17). **negative:** `unit/verify` [`TestRFC8665DuplicatePrefixSIDsAllIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L329). **negative:** `unit/verify` [`TestRFC8665OneRouterMultiplePrefixSIDsAllIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_prefix_sid_per_router_test.go#L50). **negative:** `unit/verify` [`TestRFC8665PrefixSIDsOfDifferentAlgorithmsNotDuplicate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_ext_malformed_test.go#L229). **negative:** `unit/verify` [`TestRFC8665TILFAIgnoresDuplicatePrefixSIDs`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_tilfa_duplicate_test.go#L19) |
| `RFC8665-5-7` | When calculating the outgoing label for the prefix, the router MUST take into account, as described below, the E-, NP-, and M-Flags advertised by the next-hop router if that router advertised the SID for the prefix. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC8665NextHopEAndMFlagsApplied`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_nexthop_flags_test.go#L46). **positive:** `unit/verify` [`TestRFC8665NextHopFlagsAppliedWhereSIDAdvertised`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L360). **negative:** `unit/verify` [`TestRFC8665OriginatorFlagsNotAppliedAtTransitHop`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L402) |
| `RFC8665-5-8` | The NP-Flag (No-PHP) MUST be set and the E-Flag MUST be clear for Prefix-SIDs allocated to inter-area prefixes that are originated by the ABR based on intra-area or inter-area reachability between areas unless the advertised prefix is directly attached to the ABR. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the IPv4 Prefix-SID builder copies the NP and E flags straight from configuration and never forces NP set with E clear for an inter-area prefix originated by an ABR, srBuildPrefixSID internal/plugins/ospf/sr.go:197-213, which matches only on the configured prefix and ignores the ctx.RouteType it is handed. The equivalent rule exists only for IPv6, v6InterAreaPrefixSIDRule internal/plugins/ospf/sr_interarea_v6.go:35-43. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| `RFC8665-5-9` | The NP-Flag (No-PHP) MUST be set and the E-Flag MUST be clear for Prefix-SIDs allocated to redistributed prefixes, unless the redistributed prefix is directly attached to the Autonomous System Boundary Router (ASBR). (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the same builder applies no NP-set / E-clear rule to a redistributed prefix, srBuildPrefixSID internal/plugins/ospf/sr.go:197-213; the AS-external Extended Prefix advertisement carries whatever flags the prefix-sid configuration sets, internal/plugins/ospf/ext_prefix.go:162-176. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| `RFC8665-5-10` | If the NP-Flag is not set, then: Any upstream neighbor of the Prefix-SID originator MUST pop the Prefix-SID. This is equivalent to the penultimate hop-popping mechanism used in the MPLS data plane. The received E-Flag is ignored. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC8665PHPPopsWhenNoPHPFlagClear`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L328). **negative:** `unit/verify` [`TestRFC8665NoPHPKeepsPrefixSIDLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L315) |
| `RFC8665-5-11` | If the NP-Flag is set and the E-Flag is not set, then: Any upstream neighbor of the Prefix-SID originator MUST keep the Prefix-SID on top of the stack. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC8665NoPHPKeepsPrefixSIDLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L309). **negative:** `unit/verify` [`TestRFC8665ExplicitNullReplacesPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L349) |
| `RFC8665-5-12` | If both the NP-Flag and E-Flag are set, then: Any upstream neighbor of the Prefix-SID originator MUST replace the Prefix-SID with an Explicit NULL label. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC8665ExplicitNullPushedAsLabelZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_explicit_null_label_test.go#L19). **positive:** `unit/verify` [`TestRFC8665ExplicitNullReplacesPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L347). **negative:** `unit/verify` [`TestRFC8665NoExplicitNullWithoutEFlag`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_explicit_null_label_test.go#L33). **negative:** `unit/verify` [`TestRFC8665NoPHPKeepsPrefixSIDLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L313) |
| `RFC8665-5-13` | When the M-Flag is set, the NP-Flag and the E-Flag MUST be ignored on reception (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC8665MappingServerFlagIgnoresNPAndE`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L362). **negative:** `unit/verify` [`TestRFC8665MappingServerFlagClearHonorsNPAndE`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L376) |
| `RFC8665-6.1-1` | Other bits: Reserved. These MUST be zero when sent and are ignored when received. (§6.1) | MUST | 6.1 | **positive:** `unit/verify` [`TestRFC8665AdjSIDReservedFlagBitsZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L389). **negative:** `unit/verify` [`TestRFC8665AdjSIDReservedFlagBitsIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L408) |
| `RFC8665-6.1-2` | When the P-Flag is set, the Adj-SID MUST be persistent (§6.1) | MUST | 6.1 | **positive:** `unit/verify` [`TestRFC8665AdjSIDNeverClaimsPersistence`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L215). **negative:** no negative test. **{single-polarity}:** every Adj-SID is allocated from the SRLB when the adjacency reaches Full and freed when it drops, and it is advertised with only the V and L flags set, srAdjManager.neighborFull internal/plugins/ospf/sr_adjsid.go:62-69 with the flag encoder at internal/plugins/ospf/sr/codec.go:115-133, so ze never sets the P-Flag and the persistence obligation never binds. A negative case needs ze to advertise P without persistence, which the encoder cannot produce |
| `RFC8665-6.2-1` | When the P-Flag is set, the LAN Adjacency SID MUST be persistent (§6.2) | MUST | 6.2 | **positive:** `unit/verify` [`TestRFC8665AdjSIDNeverClaimsPersistence`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L221). **negative:** no negative test. **{single-polarity}:** the LAN Adjacency SID is allocated by the same code path with lan set, srAdjManager.neighborFull internal/plugins/ospf/sr_adjsid.go:62-69, so it too carries the P-Flag clear and the persistence obligation never binds |
| `RFC8665-7.1-1` | An SR Mapping Server MUST use the OSPF Extended Prefix Range TLV when advertising SIDs for prefixes (§7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs no SR Mapping Server for IPv4. Nothing originates an Extended Prefix Range TLV into an Extended Prefix Opaque LSA -- ExtPrefixLSA.Ranges is populated only by the decoder, internal/plugins/ospf/packet/ext_prefix.go:165-168, and read only by the show path, internal/plugins/ospf/ext_render.go:106 -- and the M-Flag is never set on an originated Prefix-SID, internal/plugins/ospf/sr.go:204-208. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| `RFC8665-7.1-2` | When propagating an OSPF Extended Prefix Range TLV between areas, ABRs MUST set the IA-Flag (§7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the IA-Flag argument of EncodeExtPrefixRangeValueV4, internal/plugins/ospf/sr/codec.go:482-494, has no ABR caller: the IPv4 Extended Prefix originator emits only Extended Prefix TLVs and propagates no prefix range between areas, internal/plugins/ospf/ext_prefix.go:61-80 and internal/plugins/ospf/ext_prefix.go:136-160. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| `RFC8665-7.1-3` | Multiple Mapping Servers can advertise Prefix-SIDs for the same prefix; in which case, the same Prefix-SID MUST be advertised by all of them. (§7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze advertises no mapping-server Prefix-SIDs, so it enforces no consistency between mapping servers: the Prefix-SID builder emits only this router's own configured node SIDs with the M-Flag clear, internal/plugins/ospf/sr.go:197-213, and the receive path keeps one Prefix-SID per prefix and marks a second one duplicate whatever its source, internal/plugins/ospf/sr_install.go:274-278. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| `RFC8665-7.2-1` | In order to support SR in a multiarea environment, OSPFv2 MUST propagate Prefix-SID information between areas. (§7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** OSPFv2 does not propagate a learned Prefix-SID between areas. srBuildPrefixSID attaches a Prefix-SID only when the prefix matches an entry in this router's own segment-routing configuration, internal/plugins/ospf/sr.go:202-212, so the inter-area Extended Prefix TLV an ABR originates from its self Type-3 summaries, internal/plugins/ospf/ext_prefix.go:136-160, carries no Prefix-SID for a remote prefix. Inter-area propagation exists only for IPv6, v6OriginateInterAreaSR internal/plugins/ospf/sr_interarea_v6.go:60-83. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| `RFC8665-7.4.1-1` | If the adjacency transitions to a state lower than 2-Way, then the Adj-SID Advertisement MUST be withdrawn from the area. (§7.4.1) | MUST | 7.4.1 | **positive:** `unit/verify` [`TestRFC8665AdjSIDWithdrawnThroughNeighborSink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_adjsid_withdraw_test.go#L20). **positive:** `unit/verify` [`TestRFC8665AdjSIDWithdrawnWhenAdjacencyDrops`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L434). **negative:** `unit/verify` [`TestRFC8665AdjSIDWithdrawKeyedByAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L469) |
| `RFC8665-10-1` | Implementations MUST assure that malformed TLVs and sub-TLVs defined in this document are detected and do not provide a vulnerability for attackers to crash the OSPFv2 router or routing process. (§10) | MUST | 10 | **positive:** `unit/verify` [`TestRFC8665WellFormedTLVsDecode`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L465). **negative:** `unit/verify` [`TestRFC8665TruncatedTLVsRejectedWithoutPanic`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L480). **negative:** `unit/verify` [`TestRFC8665ZeroLengthSRAlgorithmIgnoresLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_sr_algorithm_length_test.go#L16) |
| `RFC8665-9-1` | For any new TLVs/sub-TLVs defined in this document, if the length is invalid, the LSA in which it is advertised is considered malformed and MUST be ignored. (§9) | MUST | 9 | **positive:** `unit/verify` [`TestRFC8665LengthBeforeFlagsIgnoresLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_length_before_flags_test.go#L56). **positive:** `unit/verify` [`TestRFC8665LengthInvalidAdjSIDIgnoresExtendedLinkLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_ext_malformed_test.go#L121). **positive:** `unit/verify` [`TestRFC8665LengthInvalidSubTLVIgnoredByBGPLSExport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_ext_malformed_test.go#L146). **positive:** `unit/verify` [`TestRFC8665LengthInvalidSubTLVIgnoresExtendedPrefixLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_ext_malformed_test.go#L72). **positive:** `unit/verify` [`TestRFC8665LengthInvalidTLVIgnoresLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_malformed_lsa_test.go#L47). **positive:** `unit/verify` [`TestRFC8665WellFormedTLVsDecode`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L463). **negative:** `unit/verify` [`TestRFC8665LengthBeforeFlagsIgnoresLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_length_before_flags_test.go#L68). **negative:** `unit/verify` [`TestRFC8665LengthInvalidAdjSIDIgnoresExtendedLinkLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_ext_malformed_test.go#L129). **negative:** `unit/verify` [`TestRFC8665LengthInvalidSRGBIgnoresLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_malformed_lsa_test.go#L20). **negative:** `unit/verify` [`TestRFC8665LengthInvalidSubTLVIgnoredByBGPLSExport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_ext_malformed_test.go#L148). **negative:** `unit/verify` [`TestRFC8665LengthInvalidSubTLVIgnoresExtendedPrefixLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_ext_malformed_test.go#L84). **negative:** `unit/verify` [`TestRFC8665LengthInvalidTLVIgnoresLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_malformed_lsa_test.go#L56). **negative:** `unit/verify` [`TestRFC8665TruncatedTLVsRejectedWithoutPanic`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L475) |
| `RFC8665-3.1-6` | The SR-Algorithm TLV is optional. It SHOULD only be advertised once in the Router Information Opaque LSA. (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-3.2-9` | Reserved: SHOULD be set to 0 on transmission (§3.2, §3.3, §3.4, §4, §5, §6.1, §6.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-3.3-5` | Each time a SID from the SRLB is allocated, it SHOULD also be reported to all components (e.g., controller or applications) in order for these components to have an up-to-date view of the current SRLB allocation. (§3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-3.4-4` | For the purpose of the SRMS Preference TLV advertisement, AS-scoped flooding SHOULD be used. (§3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-5-14` | However, PHP behavior SHOULD be done in the following cases: The Prefix is intra-area type and the downstream neighbor is the originator of the prefix. The Prefix is inter-area type and the downstream neighbor is an ABR, which is advertising prefix reachability and is also generating the Extended Prefix TLV with the A-Flag set for this prefix as described in Section 2.1 of [RFC7684]. The Prefix is external type and the downstream neighbor is an ASBR, which is advertising prefix reachability and is also generating the Extended Prefix TLV with the A-Flag set for this prefix as described in Section 2.1 of [RFC7684]. (§5) | SHOULD | 5 | **positive:** `unit/verify` [`TestRFC8665AttachedAdvertisersFromAFlag`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_mapped_sid_php_test.go#L165). **positive:** `unit/verify` [`TestRFC8665MappedSIDIntraAreaPHPThroughSPFRoutes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_mapped_sid_intra_spf_test.go#L117). **positive:** `unit/verify` [`TestRFC8665MappedSIDPHPInListedCases`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_mapped_sid_php_test.go#L74). **positive:** `unit/verify` [`TestRFC8665MappedSIDPHPThroughSPFRoutes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_mapped_sid_spf_test.go#L161). **negative:** `unit/verify` [`TestRFC8665MappedSIDIntraAreaKeepsLabelTowardTransitRouter`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_mapped_sid_intra_spf_test.go#L136). **negative:** `unit/verify` [`TestRFC8665MappedSIDKeepsLabelOutsideListedCases`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_mapped_sid_php_test.go#L105). **negative:** `unit/verify` [`TestRFC8665MappedSIDKeepsLabelThroughSPFRoutesOutsideListedCases`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_mapped_sid_spf_test.go#L187) |
| `RFC8665-7.3-1` | When an ASBR, which supports SR, generates Type-5 LSAs, it SHOULD also originate OSPF Extended Prefix Opaque LSAs as described in [RFC7684]. (§7.3) | SHOULD | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-7.3-2` | When a Not-So-Stubby Area (NSSA) [RFC3101] ABR translates Type-7 LSAs into Type-5 LSAs, it SHOULD also advertise the Prefix-SID for the prefix. (§7.3) | SHOULD | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-10-2` | While OSPF is under a single administrative domain, there can be deployments where potential attackers have access to one or more networks in the OSPF routing domain. In these deployments, stronger authentication mechanisms such as those specified in [RFC7474] SHOULD be used. (§10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-9-2` | Reception of malformed TLVs or sub-TLVs SHOULD be counted and/or logged for further analysis. (§10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-9-3` | Logging of malformed TLVs and sub-TLVs SHOULD be rate limited to prevent a Denial of Service (DoS) attack (distributed or otherwise) from overloading the OSPF control plane. (§10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-3.2-10` | Prefix-SIDs MAY be advertised in the form of an index (§3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-3.2-11` | The SID/Label Range TLV MAY appear multiple times (§3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-3.2-12` | Only a single SID/Label Sub-TLV MAY be advertised in the SID/Label Range TLV (§3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-3.2-13` | Multiple occurrences of the SID/Label Range TLV MAY be advertised in order to advertise multiple ranges. (§3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-3.3-6` | SIDs from the SRLB MAY be used for Adjacency SIDs but also by components other than the OSPF protocol. (§3.3) | MAY | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-3.3-7` | The SRLB TLV MAY appear multiple times in the Router Information Opaque LSA (§3.3) | MAY | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-3.3-8` | Only a single SID/Label Sub-TLV MAY be advertised in the SRLB TLV (§3.3) | MAY | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-3.3-9` | A router advertising the SRLB TLV MAY also have other label ranges, outside of the SRLB, used for its local allocation purposes and not advertised in the SRLB TLV. (§3.3) | MAY | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-3.4-5` | The SRMS Preference TLV MAY only be advertised once in the Router Information Opaque LSA (§3.4) | MAY | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-3.4-6` | If the SRMS advertisements from the SRMS server are only used inside the SRMS server's area, area-scoped flooding MAY be used. (§3.4) | MAY | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-4-4` | Multiple OSPF Extended Prefix Range TLVs MAY be advertised in each OSPF Extended Prefix Opaque LSA (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-5-15` | The Prefix-SID Sub-TLV is a sub-TLV of the OSPF Extended Prefix TLV described in [RFC7684] and the OSPF Extended Prefix Range TLV described in Section 4. It MAY appear more than once in the parent TLV (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-6.1-3` | Adj-SID is an optional sub-TLV of the Extended Link TLV defined in [RFC7684]. It MAY appear multiple times in the Extended Link TLV. (§6.1) | MAY | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-6.1-4` | An SR-capable router MAY allocate an Adj-SID for each of its adjacencies and set the B-Flag when the adjacency is eligible for protection by an FRR mechanism (IP or MPLS) as described in Section 3.5 of [RFC8402]. (§6.1) | MAY | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-6.1-5` | An SR-capable router MAY allocate more than one Adj-SID to an adjacency (§6.1) | MAY | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-6.1-6` | An SR-capable router MAY allocate the same Adj-SID to different adjacencies (§6.1) | MAY | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-6.1-7` | When set, the G-Flag indicates that the Adj-SID refers to a group of adjacencies (and therefore MAY be assigned to other adjacencies as well). (§6.1) | MAY | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-6.1-8` | When the P-Flag is not set, the Adj-SID MAY be persistent (§6.1) | MAY | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-6.2-2` | The LAN Adjacency SID is an optional sub-TLV of the Extended Link TLV defined in [RFC7684]. It MAY appear multiple times in the Extended Link TLV. (§6.2) | MAY | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-6.2-3` | When the P-Flag is not set, the LAN Adjacency SID MAY be persistent (§6.2) | MAY | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-7.1-4` | An OSPFv2 router that supports Segment Routing MAY advertise Prefix- SIDs for any prefix to which it is advertising reachability (e.g., a loopback IP address as described in Section 5). (§7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-7.4.1-2` | An Adj-SID MAY be advertised for any adjacency on a point-to-point (P2P) link that is in neighbor state 2-Way or higher. (§7.4.1) | MAY | 7.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-7.4.1-3` | If the adjacency on a P2P link transitions from the FULL state, then the Adj-SID for that adjacency MAY be removed from the area. (§7.4.1) | MAY | 7.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-7.4.2-1` | When Segment Routing is used, each router on the broadcast, NBMA, or hybrid network MAY advertise the Adj-SID for its adjacency to the DR using the Adj-SID Sub-TLV as described in Section 6.1. (§7.4.2) | MAY | 7.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-7.4.2-2` | SR-capable routers MAY also advertise a LAN Adjacency SID for other neighbors (e.g., Backup Designated Router, DR-OTHER, etc.) on the broadcast, NBMA, or hybrid network using the LAN Adj-SID Sub-TLV as described in Section 6.2. (§7.4.2) | MAY | 7.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8665-3.1-7` | For the purpose of SR-Algorithm TLV advertisement, area-scoped flooding is REQUIRED. (§3.1) | REQUIRED | 3.1 | **positive:** `unit/verify` [`TestRFC8665SRCapabilityTLVsAreAreaScoped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L85). **negative:** `unit/verify` [`TestRFC8665SRCapabilityTLVsAbsentFromOtherScopes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L105) |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8665-3.1-4`](#rfc8665-3.1-4) If the SR-Algorithm TLV appears in multiple Router Information Opaque LSAs that have different flooding scopes, the SR-Algorithm TLV in the Router Information Opaque LSA with the area-scoped flooding scope MUST be used. (§3.1) | {gap}, no test | the SR capability read walks every RI Opaque LSA in the LSDB and assigns the per-router entry from whichever view it reaches last, with no flooding-scope comparison -- srRemoteCapabilities iterates e.lsdb.OpaqueLSAsByType at internal/plugins/ospf/sr_install.go:238-241 and its record closure assigns caps[router] and algos[router] at internal/plugins/ospf/sr_install.go:222-229 -- so an AS-scoped RI LSA can override the area-scoped one. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| [`RFC8665-3.1-5`](#rfc8665-3.1-5) If the SR-Algorithm TLV appears in multiple Router Information Opaque LSAs that have the same flooding scope, the SR-Algorithm TLV in the Router Information (RI) Opaque LSA with the numerically smallest Instance ID MUST be used and subsequent instances of the SR-Algorithm TLV MUST be ignored. (§3.1) | {gap}, no test | the SR capability read compares no Instance ID. The opaque view carries OpaqueID, the RFC 7770 Instance ID, but srRemoteCapabilities ignores it and the last view processed wins, internal/plugins/ospf/sr_install.go:238-241 with the assignment at internal/plugins/ospf/sr_install.go:222-229. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| [`RFC8665-3.2-8`](#rfc8665-3.2-8) * When a router receives multiple overlapping ranges, it MUST conform to the procedures defined in [RFC8660]. (§3.2) | {gap}, no test | the receive path appends every decoded SID/Label Range to the originator SRGB with no overlap detection, srDecodeRemoteCapabilities internal/plugins/ospf/sr.go:337-342, and SRGB.Label maps an index by plain concatenation in advertised order, internal/plugins/ospf/sr/srgb.go:93-105, so overlapping received ranges are concatenated rather than resolved per RFC 8660. The non-overlap check covers only this router's own configured ranges, internal/plugins/ospf/sr/config.go:116-121. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| [`RFC8665-3.4-2`](#rfc8665-3.4-2) If the SRMS Preference TLV appears in multiple Router Information Opaque LSAs that have different flooding scopes, the SRMS Preference TLV in the Router Information Opaque LSA with the narrowest flooding scope MUST be used. (§3.4) | {gap}, no test | the received SRMS preference is decoded into srRemoteCapabilities.SRMSPref, internal/plugins/ospf/sr.go:349-358, and nothing consumes it: srRemoteCapabilities keeps only the SRGB and the algorithm list, internal/plugins/ospf/sr_install.go:222-229, so no narrowest-flooding-scope selection exists. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| [`RFC8665-3.4-3`](#rfc8665-3.4-3) If the SRMS Preference TLV appears in multiple Router Information Opaque LSAs that have the same flooding scope, the SRMS Preference TLV in the Router Information Opaque LSA with the numerically smallest Instance ID MUST be used and subsequent instances of the SRMS Preference TLV MUST be ignored. (§3.4) | {gap}, no test | the decode keeps the first SRMS Preference TLV within one LSA body, internal/plugins/ospf/sr.go:349-358, but nothing compares instances across LSAs and the preference is never consumed, internal/plugins/ospf/sr_install.go:222-229, so there is no smallest-Instance-ID tie-break. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| [`RFC8665-4-1`](#rfc8665-4-1) all prefix ranges included in a single OSPF Extended Prefix Opaque LSA MUST have the same flooding scope. (§4) | {gap}, no test | ze originates no OSPF Extended Prefix Range TLV for IPv4. extPrefixOnOriginate builds one Extended Prefix Opaque LSA per advertised prefix carrying a single Extended Prefix TLV, internal/plugins/ospf/ext_prefix.go:61-80, and never populates ExtPrefixLSA.Ranges; the range value encoder exists at internal/plugins/ospf/sr/codec.go:482-494 with no caller outside tests, so no code assigns a flooding scope to a prefix range or keeps the ranges in one LSA scope-uniform. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| [`RFC8665-4-2`](#rfc8665-4-2) An Area Border Router (ABR) that is advertising the OSPF Extended Prefix Range TLV between areas MUST set this bit. (§4, §7.1) | {gap}, no test | EncodeExtPrefixRangeValueV4 takes an iaFlag argument and writes the IA-Flag bit, internal/plugins/ospf/sr/codec.go:482-494, but no ABR path calls it: the IPv4 Extended Prefix originator emits only Extended Prefix TLVs, internal/plugins/ospf/ext_prefix.go:61-80, and the only inter-area Prefix-SID propagation is the IPv6 one, internal/plugins/ospf/sr_interarea_v6.go:60-83, so no OSPFv2 ABR sets the IA-Flag. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| [`RFC8665-4-3`](#rfc8665-4-3) The Range Size MUST NOT exceed the number of prefixes that could be satisfied by the Prefix Length without including the IPv4 multicast address range (224.0.0.0/3). (§4) | {gap}, no test | the Extended Prefix Range encoder writes the caller's Range Size verbatim with no capacity check against the Prefix Length and no 224.0.0.0/3 exclusion, EncodeExtPrefixRangeValueV4 internal/plugins/ospf/sr/codec.go:482-494, and the decoder reads it back unchecked, internal/plugins/ospf/sr/codec.go:497-523. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| [`RFC8665-5-8`](#rfc8665-5-8) The NP-Flag (No-PHP) MUST be set and the E-Flag MUST be clear for Prefix-SIDs allocated to inter-area prefixes that are originated by the ABR based on intra-area or inter-area reachability between areas unless the advertised prefix is directly attached to the ABR. (§5) | {gap}, no test | the IPv4 Prefix-SID builder copies the NP and E flags straight from configuration and never forces NP set with E clear for an inter-area prefix originated by an ABR, srBuildPrefixSID internal/plugins/ospf/sr.go:197-213, which matches only on the configured prefix and ignores the ctx.RouteType it is handed. The equivalent rule exists only for IPv6, v6InterAreaPrefixSIDRule internal/plugins/ospf/sr_interarea_v6.go:35-43. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| [`RFC8665-5-9`](#rfc8665-5-9) The NP-Flag (No-PHP) MUST be set and the E-Flag MUST be clear for Prefix-SIDs allocated to redistributed prefixes, unless the redistributed prefix is directly attached to the Autonomous System Boundary Router (ASBR). (§5) | {gap}, no test | the same builder applies no NP-set / E-clear rule to a redistributed prefix, srBuildPrefixSID internal/plugins/ospf/sr.go:197-213; the AS-external Extended Prefix advertisement carries whatever flags the prefix-sid configuration sets, internal/plugins/ospf/ext_prefix.go:162-176. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| [`RFC8665-7.1-1`](#rfc8665-7.1-1) An SR Mapping Server MUST use the OSPF Extended Prefix Range TLV when advertising SIDs for prefixes (§7.1) | {gap}, no test | ze runs no SR Mapping Server for IPv4. Nothing originates an Extended Prefix Range TLV into an Extended Prefix Opaque LSA -- ExtPrefixLSA.Ranges is populated only by the decoder, internal/plugins/ospf/packet/ext_prefix.go:165-168, and read only by the show path, internal/plugins/ospf/ext_render.go:106 -- and the M-Flag is never set on an originated Prefix-SID, internal/plugins/ospf/sr.go:204-208. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| [`RFC8665-7.1-2`](#rfc8665-7.1-2) When propagating an OSPF Extended Prefix Range TLV between areas, ABRs MUST set the IA-Flag (§7.1) | {gap}, no test | the IA-Flag argument of EncodeExtPrefixRangeValueV4, internal/plugins/ospf/sr/codec.go:482-494, has no ABR caller: the IPv4 Extended Prefix originator emits only Extended Prefix TLVs and propagates no prefix range between areas, internal/plugins/ospf/ext_prefix.go:61-80 and internal/plugins/ospf/ext_prefix.go:136-160. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| [`RFC8665-7.1-3`](#rfc8665-7.1-3) Multiple Mapping Servers can advertise Prefix-SIDs for the same prefix; in which case, the same Prefix-SID MUST be advertised by all of them. (§7.1) | {gap}, no test | ze advertises no mapping-server Prefix-SIDs, so it enforces no consistency between mapping servers: the Prefix-SID builder emits only this router's own configured node SIDs with the M-Flag clear, internal/plugins/ospf/sr.go:197-213, and the receive path keeps one Prefix-SID per prefix and marks a second one duplicate whatever its source, internal/plugins/ospf/sr_install.go:274-278. Disclosed in docs/features/rfc-status.md RFC 8665 row |
| [`RFC8665-7.2-1`](#rfc8665-7.2-1) In order to support SR in a multiarea environment, OSPFv2 MUST propagate Prefix-SID information between areas. (§7.2) | {gap}, no test | OSPFv2 does not propagate a learned Prefix-SID between areas. srBuildPrefixSID attaches a Prefix-SID only when the prefix matches an entry in this router's own segment-routing configuration, internal/plugins/ospf/sr.go:202-212, so the inter-area Extended Prefix TLV an ABR originates from its self Type-3 summaries, internal/plugins/ospf/ext_prefix.go:136-160, carries no Prefix-SID for a remote prefix. Inter-area propagation exists only for IPv6, v6OriginateInterAreaSR internal/plugins/ospf/sr_interarea_v6.go:60-83. Disclosed in docs/features/rfc-status.md RFC 8665 row |

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

Local policy at the node claiming support for Algorithm 1 MUST NOT alter the SPF paths computed by Algorithm 1. (§3.1, §8.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single polarity (annotated, argument holds): the obligation binds only a node claiming Algorithm 1, and the only producer of that claim is srBuildAlgorithm. Forbidden precondition: advertising Algorithm 1; TestRFC8665SRAlgorithmTLVAdvertisesAlgorithmZeroOnly decodes the emitted TLV and fails unless it is exactly {0}, so any Algorithm 1 claim goes red. With no claim there is no Algorithm 1 SPF for local policy to alter and no violating input exists

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8665SRAlgorithmTLVAdvertisesAlgorithmZeroOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L47) | unit/verify | unproven |

### [`RFC8665-3.1-3`](#rfc8665-3.1-3)

When multiple SR-Algorithm TLVs are received from a given router, the receiver MUST use the first occurrence of the TLV in the Router Information Opaque LSA. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. forbidden: using a later SR-Algorithm TLV of the same RI LSA. TestRFC8665RepeatedAlgorithmAndSRMSInstancesIgnored feeds srDecodeRemoteCapabilities (the receive decode) one body with {0} then {1} and fails unless the result is exactly {0}, red on last-wins and on union; positive TestRFC8665SingleAlgorithmAndSRMSInstanceUsed pins the single-TLV case

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665RepeatedAlgorithmAndSRMSInstancesIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L138) | unit/verify | unproven |
| positive | [`TestRFC8665SingleAlgorithmAndSRMSInstanceUsed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L119) | unit/verify | unproven |

### [`RFC8665-3.1-4`](#rfc8665-3.1-4)

If the SR-Algorithm TLV appears in multiple Router Information Opaque LSAs that have different flooding scopes, the SR-Algorithm TLV in the Router Information Opaque LSA with the area-scoped flooding scope MUST be used. (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8665-3.1-4, so no unit is bound to it.

### [`RFC8665-3.1-5`](#rfc8665-3.1-5)

If the SR-Algorithm TLV appears in multiple Router Information Opaque LSAs that have the same flooding scope, the SR-Algorithm TLV in the Router Information (RI) Opaque LSA with the numerically smallest Instance ID MUST be used and subsequent instances of the SR-Algorithm TLV MUST be ignored. (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8665-3.1-5, so no unit is bound to it.

### [`RFC8665-3.2-1`](#rfc8665-3.2-1)

Range Size: 3-octet SID/label range size (i.e., the number of SIDs or labels in the range including the first SID/label). It MUST be greater than 0. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. forbidden: a Range Size of 0 sent or accepted. Receive: TestRFC8665RangeSizeZeroRejected zeroes only the size of a valid value and requires DecodeRangeValue to error, and srDecodeRemoteCapabilities appends a range only on rerr == nil. Send: the same test requires SRConfig.Validate to reject a configured SRGB of size 0, and Validate gates the config verify (validateSRConfig, register.go) before any SRGB reaches srWire. Positive TestRFC8665RangeTLVRoundTrip decodes size 100

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

If more than one SID/Label Sub-TLV is present, the SID/ Label Range TLV MUST be ignored. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. forbidden: using a range TLV that carries two SID/Label sub-TLVs. TestRFC8665RangeWithTwoSIDLabelSubTLVsIgnored builds two valid sub-TLVs and requires DecodeRangeValue to error, red on a decoder that picks one; the SRGB case of srDecodeRemoteCapabilities appends only on rerr == nil, so the error is the ignore. Positive TestRFC8665RangeWithSingleSIDLabelAccepted accepts exactly one

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665RangeWithTwoSIDLabelSubTLVsIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L107) | unit/verify | unproven |
| positive | [`TestRFC8665RangeWithSingleSIDLabelAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L96) | unit/verify | unproven |

### [`RFC8665-3.2-4`](#rfc8665-3.2-4)

* The originating router MUST encode each range into a different SID/Label Range TLV. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single polarity (annotated, argument holds: srBuildSRGB emits one RITLV per configured range and cannot pack two). TestRFC8665EachRangeInItsOwnTLV requires two type-9 TLVs from two ranges, each decoding as a single-sub-TLV range with the expected base in order, red on packing

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8665EachRangeInItsOwnTLV`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L161) | unit/verify | unproven |

### [`RFC8665-3.2-5`](#rfc8665-3.2-5)

The originating router MUST ensure the order is the same after a graceful restart (using checkpointing, nonvolatile storage, or any other mechanism) in order to ensure the SID/Label range and SID index correspondence is preserved across graceful restarts. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29 (independent judge). TestRFC8665RangeOrderStableThroughConfigRestart now drives the configuration text through parseOSPFConfig, applySRConfig (parseSegmentRouting) and srBuildSRGB, clears the process SR store (restart) and re-applies: byte-identical SID/Label Range TLVs, identical under a different key order of the document, and exactly one SRGB range 16000/8000 (revert record on parseSegmentRouting, observed red). Single polarity is valid and the corrected annotation matches the producer: sr_config.go reads the one YANG srgb container into a one-element slice, so there is no reorderable input to reject.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8665RangeOrderStableThroughConfigRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_range_order_test.go#L34) | unit/verify | revert, verified |
| positive | [`TestRFC8665RangeOrderStableAcrossRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L189) | unit/verify | unproven |

### [`RFC8665-3.2-6`](#rfc8665-3.2-6)

* The receiving router MUST adhere to the order in which the ranges are advertised when calculating a SID/Label from a SID index. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Reception path now driven end to end: TestRFC8665ReceivedRangesMappedInAdvertisedOrder installs two RI Opaque LSAs advertising 20000/5,16000/5 and the reverse, reads engine.srRemoteCapabilities (srDecodeRemoteCapabilities) and pins index 0/4/5/9 to literal labels per advertised order; judge overlay sorting decoded ranges by base before sr.NewSRGB turns it red (2.2.2.2 index 0 = 16000). SRGB.Label units keep advertised-order positive and the order-sensitivity/out-of-range negative; all three have observed-red records.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665SRGBIndexOutOfRangeAndOrderSensitivity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L142) | unit/verify | revert, verified |
| positive | [`TestRFC8665ReceivedRangesMappedInAdvertisedOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_srgb_advertised_order_test.go#L16) | unit/verify | revert, verified |
| positive | [`TestRFC8665SRGBIndexUsesAdvertisedOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L124) | unit/verify | revert, verified |

### [`RFC8665-3.2-7`](#rfc8665-3.2-7)

* The originating router MUST NOT advertise overlapping ranges. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. forbidden: advertising overlapping SRGB ranges. TestRFC8665OverlappingRangesRejected requires SRConfig.Validate to reject 16000+100 with 16050+100, red on a missing overlap check; Validate gates the config verify (validateSRConfig, register.go) so the advertised SRGB is only a validated one. Positive TestRFC8665NonOverlappingRangesAccepted accepts disjoint ranges

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665OverlappingRangesRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L175) | unit/verify | unproven |
| positive | [`TestRFC8665NonOverlappingRangesAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L159) | unit/verify | unproven |

### [`RFC8665-3.2-8`](#rfc8665-3.2-8)

* When a router receives multiple overlapping ranges, it MUST conform to the procedures defined in [RFC8660]. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8665-3.2-8, so no unit is bound to it.

### [`RFC8665-3.2-14`](#rfc8665-3.2-14)

Reserved: SHOULD be set to 0 on transmission and MUST be ignored on reception (§3.2, §3.3, §3.4, §4, §5, §6.1, §6.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. the MUST clause is reception: TestRFC8665ReservedFieldIgnoredOnReception sets the Reserved octets of all seven SR TLV/sub-TLV decoders to 0xff and fails unless the decode is DeepEqual to the zero case and error-free, red on a decoder that reads or rejects Reserved; TestRFC8665NonReservedOctetIsRead flips a read octet and requires a changed decode, so the equality is not vacuous. The transmission clause is a SHOULD, carried by row RFC8665-3.2-9, not by this MUST row Re-stamped 2026-09-27 by DF-OSPF-C after a rename only: the tag's prose names the range decoder DecodeExtPrefixRangeValueV4 (was decodeExtPrefixRangeValueV4, now exported for the RFC 8665 Section 9 LSA-level check); no assertion changed. REV-OSPF independent re-audit 2026-09-27: kept enforced. Forbidden: a decoder that rejects or reads a Reserved octet; TestRFC8665ReservedFieldIgnoredOnReception sets every Reserved octet of all seven SR decoders to 0xff and fails unless the decode is error-free and DeepEqual to the clean one; TestRFC8665NonReservedOctetIsRead proves the equality is not vacuous. The transmission SHOULD is row RFC8665-3.2-9. The body change since the earlier stamp is the decodeExtPrefixRangeValueV4 to DecodeExtPrefixRangeValueV4 rename only.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665NonReservedOctetIsRead`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_reserved_test.go#L75) | unit/verify | revert, verified |
| positive | [`TestRFC8665ReservedFieldIgnoredOnReception`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_reserved_test.go#L51) | unit/verify | revert, verified |

### [`RFC8665-3.3-1`](#rfc8665-3.3-1)

Range Size: 3-octet SID/Label range size (i.e., the number of SIDs or labels in the range including the first SID/Label). It MUST be greater than 0. (§3.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. forbidden: an SRLB Range Size of 0 sent or accepted. Receive: the SRLB case of srDecodeRemoteCapabilities uses DecodeRangeValue with an rerr == nil gate, and TestRFC8665RangeSizeZeroRejected requires it to reject size 0. Send: the same test requires Validate to reject a configured SRLB of size 0 (the cfgLB case), and Validate gates config verify. Positive TestRFC8665RangeTLVRoundTrip

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

If more than one SID/Label Sub-TLV is present, the SRLB TLV MUST be ignored. (§3.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. forbidden: using an SRLB TLV that carries two SID/Label sub-TLVs. The V4TypeSRLB case of srDecodeRemoteCapabilities calls the same DecodeRangeValue and appends only on rerr == nil; TestRFC8665RangeWithTwoSIDLabelSubTLVsIgnored requires that decoder to reject the two-sub-TLV value, red on a decoder that picks one. Positive accepts exactly one

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665RangeWithTwoSIDLabelSubTLVsIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L110) | unit/verify | unproven |
| positive | [`TestRFC8665RangeWithSingleSIDLabelAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L98) | unit/verify | unproven |

### [`RFC8665-3.3-4`](#rfc8665-3.3-4)

The originating router MUST NOT advertise overlapping ranges. (§3.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. forbidden: advertising overlapping SRLB ranges. TestRFC8665OverlappingRangesRejected requires Validate to reject two overlapping SRLB ranges (and an SRLB overlapping the SRGB), red on a missing check; Validate gates config verify. Positive TestRFC8665NonOverlappingRangesAccepted accepts disjoint SRLB ranges

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665OverlappingRangesRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L178) | unit/verify | unproven |
| positive | [`TestRFC8665NonOverlappingRangesAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L162) | unit/verify | unproven |

### [`RFC8665-3.4-1`](#rfc8665-3.4-1)

When multiple SRMS Preference TLVs are received from a given router, the receiver MUST use the first occurrence of the TLV in the Router Information Opaque LSA. (§3.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. forbidden: using a later SRMS Preference TLV of the same RI LSA. TestRFC8665RepeatedAlgorithmAndSRMSInstancesIgnored decodes an RI body with Preference 100 then 250 via srDecodeRemoteCapabilities and fails unless SRMSPref is 100, red on last-wins; positive pins the single-TLV case. The preference has no consumer (RFC8665-3.4-2 gap), so the decode is the only use

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665RepeatedAlgorithmAndSRMSInstancesIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L141) | unit/verify | unproven |
| positive | [`TestRFC8665SingleAlgorithmAndSRMSInstanceUsed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L121) | unit/verify | unproven |

### [`RFC8665-3.4-2`](#rfc8665-3.4-2)

If the SRMS Preference TLV appears in multiple Router Information Opaque LSAs that have different flooding scopes, the SRMS Preference TLV in the Router Information Opaque LSA with the narrowest flooding scope MUST be used. (§3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8665-3.4-2, so no unit is bound to it.

### [`RFC8665-3.4-3`](#rfc8665-3.4-3)

If the SRMS Preference TLV appears in multiple Router Information Opaque LSAs that have the same flooding scope, the SRMS Preference TLV in the Router Information Opaque LSA with the numerically smallest Instance ID MUST be used and subsequent instances of the SRMS Preference TLV MUST be ignored. (§3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8665-3.4-3, so no unit is bound to it.

### [`RFC8665-4-1`](#rfc8665-4-1)

all prefix ranges included in a single OSPF Extended Prefix Opaque LSA MUST have the same flooding scope. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8665-4-1, so no unit is bound to it.

### [`RFC8665-4-2`](#rfc8665-4-2)

An Area Border Router (ABR) that is advertising the OSPF Extended Prefix Range TLV between areas MUST set this bit. (§4, §7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8665-4-2, so no unit is bound to it.

### [`RFC8665-4-3`](#rfc8665-4-3)

The Range Size MUST NOT exceed the number of prefixes that could be satisfied by the Prefix Length without including the IPv4 multicast address range (224.0.0.0/3). (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8665-4-3, so no unit is bound to it.

### [`RFC8665-5-1`](#rfc8665-5-1)

Other bits: Reserved. These MUST be zero when sent and are ignored when received. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. two clauses. Sent: TestRFC8665PrefixSIDReservedFlagBitsZeroOnSend encodes every defined flag and fails if mask 0x83 is set, red on an encoder leaking a reserved bit. Received: TestRFC8665PrefixSIDReservedFlagBitsIgnoredOnReceive sets 0x83 and requires no error and a decode equal to the clean one, red on a decoder that rejects or interprets them

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665PrefixSIDReservedFlagBitsIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L224) | unit/verify | unproven |
| positive | [`TestRFC8665PrefixSIDReservedFlagBitsZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L209) | unit/verify | unproven |

### [`RFC8665-5-2`](#rfc8665-5-2)

NP-Flag: No-PHP (Penultimate Hop Popping) Flag. If set, then the penultimate hop MUST NOT pop the Prefix-SID before delivering packets to the node that advertised the Prefix-SID. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. forbidden: popping when NP is set. TestRFC8665NoPHPKeepsPrefixSIDLabel requires OutgoingActionFor(NP=1) to be ActionKeep with the label imposed; negative TestRFC8665PHPPopsWhenNoPHPFlagClear requires NP=0 to pop, so the no-pop rule is tied to the flag. Sole producer of the penultimate-hop action; wiring is row RFC8665-5-7

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665PHPPopsWhenNoPHPFlagClear`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L331) | unit/verify | unproven |
| positive | [`TestRFC8665NoPHPKeepsPrefixSIDLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L306) | unit/verify | unproven |

### [`RFC8665-5-3`](#rfc8665-5-3)

E-Flag: Explicit Null Flag. If set, any upstream neighbor of the Prefix-SID originator MUST replace the Prefix-SID with the Explicit NULL label (0 for IPv4) before forwarding the packet. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. c22 re-judge. rfc8665_explicit_null_label_test.go drives installRoutes (rfc8665PushTowardOriginator) with the originator as next hop and reads the label in the mpls-fib push entry. Positive: NP=1 E=1 pushes the literal label 0, which covers both 'replace the Prefix-SID with the Explicit NULL label' and '(0 for IPv4)' without comparing sr.ExplicitNullV4 with itself (author overlay ExplicitNullV4=3 reds it while the older units stay green; recorded revert of sr/install.go::OutgoingLabel). Negative: E clear (NP=1) pushes the Prefix-SID label 16009 and not 0 (recorded revert of OutgoingActionFor). RFC 8665 is IPv4-only, so the IPv4 value is the whole clause.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665NoExplicitNullWithoutEFlag`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_explicit_null_label_test.go#L31) | unit/verify | revert, verified |
| negative | [`TestRFC8665NoPHPKeepsPrefixSIDLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L311) | unit/verify | unproven |
| positive | [`TestRFC8665ExplicitNullPushedAsLabelZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_explicit_null_label_test.go#L16) | unit/verify | revert, verified |
| positive | [`TestRFC8665ExplicitNullReplacesPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L344) | unit/verify | unproven |

### [`RFC8665-5-4`](#rfc8665-5-4)

A router receiving a Prefix-SID from a remote node and with an algorithm value that the remote node has not advertised in the SR-Algorithm TLV (Section 3.1) MUST ignore the Prefix-SID Sub- TLV. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. forbidden: installing a Prefix-SID whose algorithm the originator did not advertise. TestRFC8665PrefixSIDIgnoredWhenAlgorithmNotAdvertised drives installRoutes with the originator advertising {1} only and a usable SRGB and requires no mpls-fib entry at all; without the check the NP=0 penultimate-hop route would emit a pop entry, so the len == 0 assertion goes red. Positive with {0} requires push label 16009

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665PrefixSIDIgnoredWhenAlgorithmNotAdvertised`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L286) | unit/verify | unproven |
| positive | [`TestRFC8665PrefixSIDInstalledWhenAlgorithmAdvertised`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L258) | unit/verify | unproven |

### [`RFC8665-5-5`](#rfc8665-5-5)

All other combinations of V-Flag and L-Flag are invalid and any SID Advertisement received with an invalid setting for V- and L-Flags MUST be ignored. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. forbidden: accepting a SID with V set and L clear or the reverse. TestRFC8665InvalidVLCombinationIgnored requires Prefix-SID, Adj-SID and LAN Adj-SID decoders to error on both invalid pairs; the receive callers (srRemotePrefixSIDs sr_install.go, sr_tilfa.go) use a SID only on derr == nil, so the error is the ignore. Positive TestRFC8665ValidVLCombinationsDecode decodes V=0/L=0 index and V=1/L=1 label forms

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665InvalidVLCombinationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L279) | unit/verify | unproven |
| positive | [`TestRFC8665ValidVLCombinationsDecode`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L247) | unit/verify | unproven |

### [`RFC8665-5-6`](#rfc8665-5-6)

If an OSPF router advertises multiple Prefix-SIDs for the same prefix, topology, and algorithm, all of them MUST be ignored (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29 (independent judge). The second reader is now covered: TestRFC8665TILFAIgnoresDuplicatePrefixSIDs drives srTILFAResolver.PrefixSIDLabel with an absolute-label SID (no SRGB needed), resolves 20009 for one Prefix-SID and nothing once the same router advertises a second one for the same prefix, topology and algorithm; removing the Duplicate skip reddens it (records on PrefixSIDLabel). Installer proofs and per-(area,router,prefix,MT-ID,algorithm) scoping stand. A mixed case (one router repeating, another advertising once) is still not driven; the 2026-09-27 audit recorded that the positive scoping cases go red when the router key or the per-(area,router) count is dropped, which bounds that risk.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665PrefixSIDsOfDifferentAlgorithmsNotDuplicate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_ext_malformed_test.go#L229) | unit/verify | revert, verified |
| negative | [`TestRFC8665DuplicatePrefixSIDsAllIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L329) | unit/verify | unproven |
| negative | [`TestRFC8665TILFAIgnoresDuplicatePrefixSIDs`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_tilfa_duplicate_test.go#L19) | unit/verify | revert, verified |
| negative | [`TestRFC8665OneRouterMultiplePrefixSIDsAllIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_prefix_sid_per_router_test.go#L50) | unit/verify | revert, verified |
| positive | [`TestRFC8665PrefixSIDsOfDifferentAlgorithmsNotDuplicate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_ext_malformed_test.go#L218) | unit/verify | revert, verified |
| positive | [`TestRFC8665PrefixSIDInstalledWhenAlgorithmAdvertised`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L261) | unit/verify | unproven |
| positive | [`TestRFC8665TILFAIgnoresDuplicatePrefixSIDs`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_tilfa_duplicate_test.go#L17) | unit/verify | revert, verified |
| positive | [`TestRFC8665OneRouterMultiplePrefixSIDsAllIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_prefix_sid_per_router_test.go#L67) | unit/verify | revert, verified |

### [`RFC8665-5-7`](#rfc8665-5-7)

When calculating the outgoing label for the prefix, the router MUST take into account, as described below, the E-, NP-, and M-Flags advertised by the next-hop router if that router advertised the SID for the prefix. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (independent judge, ospf c27). Both c26 gaps closed. Claims now name srInstaller.forwarding / installRoutes and state what the bodies assert (bodies unchanged): + TestRFC8665NextHopFlagsAppliedWhereSIDAdvertised (next hop = SID advertiser with NP clear, transit next hop listed first: no push and a pop toward the originator next hop); - TestRFC8665OriginatorFlagsNotAppliedAtTransitHop (next hop did not advertise the SID: originator NP=0 not applied, push 17009 from the transit SRGB). Revert records on forwarding for both, observed red. Judge replayed overlays on forwarding: penultimate test forced true -> only the - red ('<nil>', PHP at transit); forced false -> only the + red (push 16009 toward the originator). TestRFC8665NextHopEAndMFlagsApplied keeps the E/NP/M cases (record on srMappedSIDAction).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665OriginatorFlagsNotAppliedAtTransitHop`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L402) | unit/verify | revert, verified |
| positive | [`TestRFC8665NextHopEAndMFlagsApplied`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_nexthop_flags_test.go#L46) | unit/verify | revert, verified |
| positive | [`TestRFC8665NextHopFlagsAppliedWhereSIDAdvertised`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L360) | unit/verify | revert, verified |

### [`RFC8665-5-8`](#rfc8665-5-8)

The NP-Flag (No-PHP) MUST be set and the E-Flag MUST be clear for Prefix-SIDs allocated to inter-area prefixes that are originated by the ABR based on intra-area or inter-area reachability between areas unless the advertised prefix is directly attached to the ABR. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8665-5-8, so no unit is bound to it.

### [`RFC8665-5-9`](#rfc8665-5-9)

The NP-Flag (No-PHP) MUST be set and the E-Flag MUST be clear for Prefix-SIDs allocated to redistributed prefixes, unless the redistributed prefix is directly attached to the Autonomous System Boundary Router (ASBR). (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8665-5-9, so no unit is bound to it.

### [`RFC8665-5-10`](#rfc8665-5-10)

If the NP-Flag is not set, then: Any upstream neighbor of the Prefix-SID originator MUST pop the Prefix-SID. This is equivalent to the penultimate hop-popping mechanism used in the MPLS data plane. The received E-Flag is ignored. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. two clauses. Pop: TestRFC8665PHPPopsWhenNoPHPFlagClear requires OutgoingActionFor(NP=0) to be ActionPHP and OutgoingLabel to impose nothing; E ignored: the same case sets E=1, so honoring E goes red. Negative TestRFC8665NoPHPKeepsPrefixSIDLabel: NP=1 must not pop. OutgoingActionFor is the sole producer of the penultimate-hop action (sr_install.go, sr_fib.go); the wiring to the next-hop case is row RFC8665-5-7

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665NoPHPKeepsPrefixSIDLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L315) | unit/verify | unproven |
| positive | [`TestRFC8665PHPPopsWhenNoPHPFlagClear`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L328) | unit/verify | unproven |

### [`RFC8665-5-11`](#rfc8665-5-11)

If the NP-Flag is set and the E-Flag is not set, then: Any upstream neighbor of the Prefix-SID originator MUST keep the Prefix-SID on top of the stack. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. forbidden: popping or replacing the SID with NP=1/E=0. TestRFC8665NoPHPKeepsPrefixSIDLabel requires ActionKeep with label 16009 imposed; negative TestRFC8665ExplicitNullReplacesPrefixSID requires NP=1/E=1 not to keep, so the keep rule is tied to E clear. The installer takes its action from OutgoingActionFor/OutgoingLabel; wiring is row RFC8665-5-7

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665ExplicitNullReplacesPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L349) | unit/verify | unproven |
| positive | [`TestRFC8665NoPHPKeepsPrefixSIDLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L309) | unit/verify | unproven |

### [`RFC8665-5-12`](#rfc8665-5-12)

If both the NP-Flag and E-Flag are set, then: Any upstream neighbor of the Prefix-SID originator MUST replace the Prefix-SID with an Explicit NULL label. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. c22 re-judge. Positive: NP and E both set pushes the literal IPv4 Explicit NULL 0 in the installed mpls-fib entry toward the originator, through installRoutes (recorded revert of sr/install.go::OutgoingLabel; overlay ExplicitNullV4=3 red). Negative: NP set with E clear is not the both-set case and pushes 16009, never 0 (recorded revert of OutgoingActionFor). The older decision-helper units stay as supplementary.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665NoExplicitNullWithoutEFlag`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_explicit_null_label_test.go#L33) | unit/verify | revert, verified |
| negative | [`TestRFC8665NoPHPKeepsPrefixSIDLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L313) | unit/verify | unproven |
| positive | [`TestRFC8665ExplicitNullPushedAsLabelZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_explicit_null_label_test.go#L19) | unit/verify | revert, verified |
| positive | [`TestRFC8665ExplicitNullReplacesPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L347) | unit/verify | unproven |

### [`RFC8665-5-13`](#rfc8665-5-13)

When the M-Flag is set, the NP-Flag and the E-Flag MUST be ignored on reception (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665MappingServerFlagClearHonorsNPAndE`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L376) | unit/verify | unproven |
| positive | [`TestRFC8665MappingServerFlagIgnoresNPAndE`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L362) | unit/verify | unproven |

### [`RFC8665-6.1-1`](#rfc8665-6.1-1)

Other bits: Reserved. These MUST be zero when sent and are ignored when received. (§6.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. two clauses. Sent: TestRFC8665AdjSIDReservedFlagBitsZeroOnSend encodes all five flags for Adj-SID and LAN Adj-SID and fails if bits 5-7 (0x07) are set. Received: TestRFC8665AdjSIDReservedFlagBitsIgnoredOnReceive sets 0x07 and requires no error and a decode equal to the clean one

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

Multiple Mapping Servers can advertise Prefix-SIDs for the same prefix; in which case, the same Prefix-SID MUST be advertised by all of them. (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8665-7.1-3, so no unit is bound to it.

### [`RFC8665-7.2-1`](#rfc8665-7.2-1)

In order to support SR in a multiarea environment, OSPFv2 MUST propagate Prefix-SID information between areas. (§7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8665-7.2-1, so no unit is bound to it.

### [`RFC8665-7.4.1-1`](#rfc8665-7.4.1-1)

If the adjacency transitions to a state lower than 2-Way, then the Adj-SID Advertisement MUST be withdrawn from the area. (§7.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29 (independent judge). TestRFC8665AdjSIDWithdrawnThroughNeighborSink reports the neighbor down through the production sink (neighborEventSinkValue().NeighborDown -> bfdNeighborLost -> srAdjNeighborLost), then asserts the Adj-SID is gone from the store the Extended Link LSA reads, its pop entry removed, its SRLB label freed and one self-LSA re-origination queued (revert record on srAdjNeighborLost). The queue is also fed by the sink's own onChange, so the unit proves the withdrawal is in the store before a re-origination that is certainly queued, which is what carries it to the area. The wrong-adjacency negative stands.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665AdjSIDWithdrawKeyedByAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L469) | unit/verify | unproven |
| positive | [`TestRFC8665AdjSIDWithdrawnThroughNeighborSink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_adjsid_withdraw_test.go#L20) | unit/verify | revert, verified |
| positive | [`TestRFC8665AdjSIDWithdrawnWhenAdjacencyDrops`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L434) | unit/verify | unproven |

### [`RFC8665-10-1`](#rfc8665-10-1)

Implementations MUST assure that malformed TLVs and sub-TLVs defined in this document are detected and do not provide a vulnerability for attackers to crash the OSPFv2 router or routing process. (§10)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (independent judge, ospf c25). Two clauses. (a) detected: the earlier gap is closed. RFC 8665 Section 3.1 makes Length 'Variable, in octets, depending on the number of algorithms advertised' and says 'If the SR-Algorithm TLV is advertised, Algorithm 0 MUST be included', so a zero-length SR-Algorithm TLV has an invalid length; DecodeAlgorithmValue (sr/codec.go) now returns ErrLength for it and srDecodeRemoteCapabilities (sr.go) length-checks every occurrence, a first-occurrence-ignored later one too, and returns no capability (Section 9). TestRFC8665TruncatedTLVsRejectedWithoutPanic now drives seven codecs including sr-algorithm (truncation to 0 octets must be rejected), TestRFC8665WellFormedTLVsDecode decodes {0,1}, and TestRFC8665ZeroLengthSRAlgorithmIgnoresLSA drives the reception seam: a control {0}+SRGB body is applied, and a zero-length TLV alone, before a well-formed one, after the SRGB, or after a {0,1} first occurrence applies nothing. Each was red on HEAD's decoder or caller (author logs r1, r2, overlay o1 proves both halves of the fix are needed). (b) no crash: the truncation loop fails on a panic for every prefix length. Observed-red records on DecodeAlgorithmValue (both polarities) and srDecodeRemoteCapabilities (negative).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665ZeroLengthSRAlgorithmIgnoresLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_sr_algorithm_length_test.go#L16) | unit/verify | revert, verified |
| negative | [`TestRFC8665TruncatedTLVsRejectedWithoutPanic`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L480) | unit/verify | revert, verified |
| positive | [`TestRFC8665WellFormedTLVsDecode`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L465) | unit/verify | revert, verified |

### [`RFC8665-9-1`](#rfc8665-9-1)

For any new TLVs/sub-TLVs defined in this document, if the length is invalid, the LSA in which it is advertised is considered malformed and MUST be ignored. (§9)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30: TestRFC8665LengthBeforeFlagsIgnoresLSA changed only by the lookupPrefix area argument (control lookup asserted before the negative, so the negative stays non-vacuous); its two records re-recorded (srRemotePrefixSIDs +, DecodePrefixSIDValue -) observed red. Earlier judgement stands: SRGB branch (TestRFC8665LengthInvalidSRGBIgnoresLSA), BGP-LS export (TestRFC8665LengthInvalidSubTLVIgnoredByBGPLSExport) and the LSA-level proofs (Prefix-SID, Extended Prefix Range, Adj-SID, LAN Adj-SID, SRLB, SRMS).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665LengthInvalidAdjSIDIgnoresExtendedLinkLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_ext_malformed_test.go#L129) | unit/verify | revert, verified |
| negative | [`TestRFC8665LengthInvalidSubTLVIgnoredByBGPLSExport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_ext_malformed_test.go#L148) | unit/verify | revert, verified |
| negative | [`TestRFC8665LengthInvalidSubTLVIgnoresExtendedPrefixLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_ext_malformed_test.go#L84) | unit/verify | revert, verified |
| negative | [`TestRFC8665LengthInvalidSRGBIgnoresLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_malformed_lsa_test.go#L20) | unit/verify | revert, verified |
| negative | [`TestRFC8665LengthInvalidTLVIgnoresLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_malformed_lsa_test.go#L56) | unit/verify | revert, verified |
| negative | [`TestRFC8665TruncatedTLVsRejectedWithoutPanic`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L475) | unit/verify | unproven |
| negative | [`TestRFC8665LengthBeforeFlagsIgnoresLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_length_before_flags_test.go#L68) | unit/verify | revert, verified |
| positive | [`TestRFC8665LengthInvalidAdjSIDIgnoresExtendedLinkLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_ext_malformed_test.go#L121) | unit/verify | revert, verified |
| positive | [`TestRFC8665LengthInvalidSubTLVIgnoredByBGPLSExport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_ext_malformed_test.go#L146) | unit/verify | revert, verified |
| positive | [`TestRFC8665LengthInvalidSubTLVIgnoresExtendedPrefixLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_ext_malformed_test.go#L72) | unit/verify | revert, verified |
| positive | [`TestRFC8665LengthInvalidTLVIgnoresLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_malformed_lsa_test.go#L47) | unit/verify | revert, verified |
| positive | [`TestRFC8665WellFormedTLVsDecode`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8665_test.go#L463) | unit/verify | unproven |
| positive | [`TestRFC8665LengthBeforeFlagsIgnoresLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_length_before_flags_test.go#L56) | unit/verify | revert, verified |

### [`RFC8665-5-14`](#rfc8665-5-14)

However, PHP behavior SHOULD be done in the following cases: The Prefix is intra-area type and the downstream neighbor is the originator of the prefix. The Prefix is inter-area type and the downstream neighbor is an ABR, which is advertising prefix reachability and is also generating the Extended Prefix TLV with the A-Flag set for this prefix as described in Section 2.1 of [RFC7684]. The Prefix is external type and the downstream neighbor is an ASBR, which is advertising prefix reachability and is also generating the Extended Prefix TLV with the A-Flag set for this prefix as described in Section 2.1 of [RFC7684]. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (independent judge, ospf c28). All three listed cases now go through a real LSDB, eng.spf.Run and srInstallFromRoutes. Intra-area (new rfc8665_mapped_sid_intra_spf_test.go): 10.0.0.1 p2p 10.0.0.2 (stub 10.3.0.0/24) p2p 10.0.0.3 (stub 10.4.0.0/24), Mapping Server M-Flag SIDs 11/12; + TestRFC8665MappedSIDIntraAreaPHPThroughSPFRoutes: next hop is the prefix originator -> no push, 18011 pops; - TestRFC8665MappedSIDIntraAreaKeepsLabelTowardTransitRouter: originator 10.0.0.3 behind next hop 10.0.0.2 -> push 16012, 18012 swaps. Judge overlays: the c27 gap overlay (Origin dropped from the srRoute srRoutes builds) now turns the intra + red ('pushed label 16011 toward its originator'); srMappedSIDAction intra case PHP for any originated route -> intra - red and the hand-built - red, + green. Inter-area/external as judged in c27 (A-Flag TLVs from the next hop, both polarities, srRoutes overlays red). Revert records observed on srRoutes and srMappedSIDAction. OSPFv3 LA-bit cases are RFC 8666's, disclosed in docs/guide/ospf.md and journaled.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665MappedSIDIntraAreaKeepsLabelTowardTransitRouter`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_mapped_sid_intra_spf_test.go#L136) | unit/verify | revert, verified |
| negative | [`TestRFC8665MappedSIDKeepsLabelOutsideListedCases`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_mapped_sid_php_test.go#L105) | unit/verify | revert, verified |
| negative | [`TestRFC8665MappedSIDKeepsLabelThroughSPFRoutesOutsideListedCases`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_mapped_sid_spf_test.go#L187) | unit/verify | revert, verified |
| positive | [`TestRFC8665MappedSIDIntraAreaPHPThroughSPFRoutes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_mapped_sid_intra_spf_test.go#L117) | unit/verify | revert, verified |
| positive | [`TestRFC8665AttachedAdvertisersFromAFlag`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_mapped_sid_php_test.go#L165) | unit/verify | revert, verified |
| positive | [`TestRFC8665MappedSIDPHPInListedCases`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_mapped_sid_php_test.go#L74) | unit/verify | revert, verified |
| positive | [`TestRFC8665MappedSIDPHPThroughSPFRoutes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_mapped_sid_spf_test.go#L161) | unit/verify | revert, verified |

### [`RFC8665-3.1-7`](#rfc8665-3.1-7)

For the purpose of SR-Algorithm TLV advertisement, area-scoped flooding is REQUIRED. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. forbidden: flooding the SR-Algorithm TLV at a non-area scope, or not at area scope. TestRFC8665SRCapabilityTLVsAbsentFromOtherScopes fails if type 8 is registered at AS or link scope; TestRFC8665SRCapabilityTLVsAreAreaScoped fails if it is missing at area scope. riTLVBuildersForScope is the sole producer of each scope's TLV set: buildRIInstances (ri.go) reads it unconditionally for the RI LSA of that scope, so the registry assertion is the emitted-LSA content

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8665SRCapabilityTLVsAbsentFromOtherScopes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L105) | unit/verify | unproven |
| positive | [`TestRFC8665SRCapabilityTLVsAreAreaScoped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8665_test.go#L85) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc8665.txt |
| Source fingerprint | 3023d4a3ef8b8441 |
| Record | rfc/extraction/rfc8665.json |
| Mapped sentences | 56 |
| Declined as scope | 2 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 6 | walked | not stated |
| `3.2` | not stated | 10 | walked | not stated |
| `3.3` | not stated | 6 | walked | not stated |
| `3.4` | not stated | 4 | walked | not stated |
| `4` | not stated | 4 | walked | not stated |
| `5` | not stated | 15 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 3 | walked | not stated |
| `6.2` | not stated | 2 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 3 | walked | not stated |
| `7.2` | not stated | 1 | walked | not stated |
| `7.3` | not stated | 0 | walked | not stated |
| `7.4` | not stated | 0 | walked | not stated |
| `7.4.1` | not stated | 1 | walked | not stated |
| `7.4.2` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |
| `8.2` | not stated | 0 | walked | not stated |
| `8.3` | not stated | 0 | walked | not stated |
| `8.4` | not stated | 0 | walked | not stated |
| `8.5` | not stated | 1 | walked | not stated |
| `9` | not stated | 1 | walked | not stated |
| `10` | not stated | 1 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `11.1` | not stated | 0 | walked | not stated |
| `11.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `5:9` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The second sentence of the same paragraph, qualifying the obligation site 5:8 carries; row RFC8665-5-7 states the qualification in its own text ("regardless of whether it contributes to the best path"). | This MUST be done regardless of whether the next-hop router contributes to the best path to the prefix. |
| `8.5:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The IANA "OSPF Segment Routing Algorithms" registry table in Section 8.5, whose Description column reproduces the Section 3.1 sentence about Algorithm 1 word for word; row RFC8665-3.1-2 cites both sections. | The algorithm is \| document \| \| \| identical to Algorithm 0, but Algorithm 1 \| \| \| \| requires that all nodes along the path \| \| \| \| will honor the SPF routing decision. \| \| \| \| Local policy at the node claiming support \| \| \| \| for Algorithm 1 MUST NOT alter the SPF \| \| \| \| paths computed by Algorithm 1. \| \| +-------+--------------------------------------------+-----------+ |

## Superseded

No document obsoletes RFC 8665, so its obligations are stated where they were written.
