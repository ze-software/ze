# RFC 8666 - OSPFv3 Extensions for Segment Routing

Partial. Every requirement this repository extracted from RFC 8666, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 77.8% | 21 of 27 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 11.1% | 3 of 27 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 27 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 46 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 31 | of 57 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 4 | of 31 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 11.1% | 3 of 27 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 27 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 57 |
| Gated MUST-level | 31 |
| Obligations that bind Ze | 27 |
| Not applicable, so out of scope | 4 |
| Declared gaps | 3 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 46 |
| Tagged units | 46 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc8666.md` |
| Requirement shard | `rfc/requirements/rfc8666.md` |
| RFC text | `rfc/full/rfc8666.txt` |

## Enrolment

Enrolled: OSPFv3 Extensions for Segment Routing

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

OSPFv3 Segment Routing (SR-MPLS) over RFC 8362 Extended LSAs: the OSPFv3 Extended-LSA registry type codes (Prefix-SID 4, Adj-SID 5, LAN Adj-SID 6, SID/Label 7, Extended Prefix Range 9), the MT-ID-free OSPFv3 sub-TLV layouts with V/L-implied SID width and reserved bits zeroed on send and ignored on receive, Prefix-SID origination under an Intra-Area Prefix TLV in the E-Intra-Area-Prefix-LSA, Adj-SID / LAN Adj-SID origination under a Router-Link TLV in the E-Router-LSA with withdrawal when the adjacency drops, reception from the E-Intra/Inter-Area-Prefix, E-AS-External and E-Type-7 LSAs through both the prefix-TLV and the Extended Prefix Range carriage, the §6 NP/E/M outgoing-label truth table against the next-hop router's SRGB with the IPv6 Explicit NULL label 2, unadvertised-algorithm and duplicate suppression, and §8.2 ABR inter-area Prefix-SID propagation with NP set and E clear. Requirements bound per line in [`rfc/short/rfc8666.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc8666.md).

**What the ledger says remains**

Three MUST gaps, annotated in [`rfc/short/rfc8666.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc8666.md) and gated by `./le rfc check`.

- **RFC8666-5-3 and RFC8666-5-4:** reception keys Prefix-SIDs by prefix alone and never consults the carrying LSA's Instance ID ([`internal/plugins/ospf/sr_reception_v6.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_reception_v6.go) and 163-178), so a range repeated across LSAs of one type from one originator resolves by first-seen / conflict-ignores-both rather than by smallest Instance ID.
- **RFC8666-10-1:** an invalid TOP-LEVEL TLV length drops the whole LSA ([`internal/plugins/ospf/v3/packet/lsa_extended.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/v3/packet/lsa_extended.go) via [`internal/plugins/ospf/sr_reception_v6.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_reception_v6.go)), but an invalid SUB-TLV length drops only its own TLV while the remaining TLVs of the same LSA are still consumed ([`internal/plugins/ospf/sr_reception_v6.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_reception_v6.go)). The §8.1 SR Mapping Server role and ASBR external Prefix-SID origination have no producer at all (annotated not-applicable). Same OSPF experimental status.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 21 | one part of the gated population |
| Annotated instead of tested | 10 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **31** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (21):** [`RFC8666-5-2`](#rfc8666-5-2), [`RFC8666-5-7`](#rfc8666-5-7), [`RFC8666-6-1`](#rfc8666-6-1), [`RFC8666-6-2`](#rfc8666-6-2), [`RFC8666-6-3`](#rfc8666-6-3), [`RFC8666-6-4`](#rfc8666-6-4), [`RFC8666-6-5`](#rfc8666-6-5), [`RFC8666-6-6`](#rfc8666-6-6), [`RFC8666-6-7`](#rfc8666-6-7), [`RFC8666-6-8`](#rfc8666-6-8), [`RFC8666-6-9`](#rfc8666-6-9), [`RFC8666-6-11`](#rfc8666-6-11), [`RFC8666-6-12`](#rfc8666-6-12), [`RFC8666-6-13`](#rfc8666-6-13), [`RFC8666-6-14`](#rfc8666-6-14), [`RFC8666-7.1-1`](#rfc8666-7.1-1), [`RFC8666-7.1-2`](#rfc8666-7.1-2), [`RFC8666-7.2-1`](#rfc8666-7.2-1), [`RFC8666-8.2-1`](#rfc8666-8.2-1), [`RFC8666-8.4.1-1`](#rfc8666-8.4.1-1), [`RFC8666-11-1`](#rfc8666-11-1)

**Annotated instead of tested (10):** [`RFC8666-5-1`](#rfc8666-5-1), [`RFC8666-5-3`](#rfc8666-5-3), [`RFC8666-5-4`](#rfc8666-5-4), [`RFC8666-6-10`](#rfc8666-6-10), [`RFC8666-7.1-3`](#rfc8666-7.1-3), [`RFC8666-7.2-2`](#rfc8666-7.2-2), [`RFC8666-8.1-1`](#rfc8666-8.1-1), [`RFC8666-8.1-2`](#rfc8666-8.1-2), [`RFC8666-8.1-3`](#rfc8666-8.1-3), [`RFC8666-10-1`](#rfc8666-10-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8666-5-1` | Range Size MUST NOT exceed prefixes satisfiable by Prefix Length, excluding IPv4 multicast 224.0.0.0/3 (IPv4) and non-unicast addresses (IPv6) (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC8666ExtPrefixRangeSizeWithinPrefixLength`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L355). **negative:** no negative test. **{single-polarity}:** the only producer of an IPv6 Extended Prefix Range TLV is the ABR inter-area propagation, which advertises one prefix per TLV with a hardcoded Range Size of 1 (v6EInterAreaPrefixBody, sr_interarea_v6.go:50), so the advertised size is always within what the Prefix Length can satisfy. A negative is not meaningful: ze has no code path that can compute an oversize Range Size to be rejected, and the RFC constrains the sender only |
| `RFC8666-5-2` | Flags field of Extended Prefix Range TLV MUST be zero when sent and is ignored when received (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC8666ExtPrefixRangeFlagsZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L15). **negative:** `unit/verify` [`TestRFC8666ExtPrefixRangeFlagsIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L25) |
| `RFC8666-5-7` | Reserved field of Extended Prefix Range TLV MUST be ignored on reception (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC8666V6ExtPrefixRangeReservedIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/codec_v6_test.go#L189). **negative:** `unit/verify` [`TestRFC8666V6ExtPrefixRangeAFOctetNotIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/codec_v6_test.go#L221) |
| `RFC8666-5-3` | Duplicate Extended Prefix Range TLV (same range, same type, same originator): LSA with numerically smallest Instance ID MUST be used (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** reception aggregates every Extended Prefix Range TLV found in the LSDB in whatever order LSAViewsByType returns and keys the result by prefix alone (v6ReceivedPrefixSIDs, sr_reception_v6.go:56-88; srRemotePrefixSIDsV6, sr_reception_v6.go:163-178). Neither reader consults the Link State ID / Instance ID of the carrying LSA, so two LSAs of the same type from one originator carrying the same range are resolved by "first seen wins, conflicting second marks the prefix Duplicate", not by smallest Instance ID. Disclosed in docs/features/rfc-status.md RFC 8666 row |
| `RFC8666-5-4` | Subsequent instances of a duplicate Extended Prefix Range TLV MUST be ignored (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** same producer as RFC8666-5-3. A subsequent instance carrying an identical SID is skipped only because it compares equal (v6PrefixSIDEqual, sr_reception_v6.go:182), and a subsequent instance carrying a different SID makes ze ignore BOTH by setting Duplicate (sr_reception_v6.go:171-176) instead of keeping the smallest-Instance-ID one. Disclosed in docs/features/rfc-status.md RFC 8666 row |
| `RFC8666-6-1` | If NP-Flag set, penultimate hop MUST NOT pop the Prefix-SID before delivering to the advertising node (§6) | MUST NOT | 6 | **positive:** `unit/verify` [`TestRFC8666NoPHPKeepsPrefixSIDOnStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L92). **negative:** `unit/verify` [`TestRFC8666PHPPopsPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L126) |
| `RFC8666-6-2` | If E-Flag set, upstream neighbor MUST replace the Prefix-SID with the Explicit NULL label (0 IPv4, 2 IPv6) before forwarding (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC8666ExplicitNullReplacesPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L147). **negative:** `unit/verify` [`TestRFC8666NoPHPKeepsPrefixSIDOnStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L99) |
| `RFC8666-6-3` | Prefix-SID Flags reserved bits (0, 6, 7) MUST be zero when sent and are ignored when received (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC8666PrefixSIDReservedFlagBitsZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L52). **negative:** `unit/verify` [`TestRFC8666PrefixSIDReservedFlagBitsIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L66) |
| `RFC8666-6-4` | Reserved field (Prefix-SID) MUST be ignored on reception (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC8666PrefixSIDReservedFieldIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L86). **negative:** `unit/verify` [`TestRFC8666PrefixSIDAlgorithmOctetNotIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L109) |
| `RFC8666-6-5` | A Prefix-SID with an Algorithm value not advertised by the remote node in its SR-Algorithm TLV MUST be ignored (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC8666PrefixSIDAdvertisedAlgorithmInstalls`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L201). **negative:** `unit/verify` [`TestRFC8666PrefixSIDUnadvertisedAlgorithmIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L212) |
| `RFC8666-6-6` | A SID advertisement with an invalid V-/L-Flag combination MUST be ignored (§6) | MUST | 6 | **positive:** `unit/verify` [`TestOSPFv3SIDWidthFromVL`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/codec_v6_test.go#L59). **negative:** `unit/verify` [`TestOSPFv3PrefixSIDCodec`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/codec_v6_test.go#L32) |
| `RFC8666-6-7` | If a router advertises multiple Prefix-SIDs for the same prefix, topology, and algorithm, all MUST be ignored (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC8666DuplicatePrefixSIDsDetectedAndIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L261). **negative:** `unit/verify` [`TestOSPFv3ReceptionSameSIDNotDuplicate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_reception_v6_test.go#L151) |
| `RFC8666-6-8` | When computing the outgoing label, the router MUST take the E-, NP-, and M-Flags of the next-hop router into account, regardless of whether it contributes to the best path (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC8666OutgoingLabelUsesNextHopRouter`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L318). **negative:** `unit/verify` [`TestRFC8666TransitHopIgnoresOriginatorPHPFlags`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L337) |
| `RFC8666-6-9` | NP-Flag set, E-Flag clear, for inter-area-propagated prefixes by an ABR (unless directly attached to the ABR) (§6) | MUST | 6 | **positive:** `unit/verify` [`TestOSPFv3InterAreaPrefixSIDRule`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_interarea_v6_test.go#L19). **negative:** `unit/verify` [`TestOSPFv3InterAreaPrefixSIDRule`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_interarea_v6_test.go#L21) |
| `RFC8666-6-10` | NP-Flag set, E-Flag clear, for redistributed prefixes (unless directly attached to the ASBR) (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the OSPFv3 SR origination never attaches a Prefix-SID to a redistributed prefix. v6OriginateSR builds only the E-Router-LSA and the E-Intra-Area-Prefix-LSA for configured node prefixes, plus the ABR inter-area propagation (sr_origination_v6.go:66-100); grep for extTLVExternalPrefix over internal/plugins/ospf finds it only in the RECEPTION switch (sr_reception_v6.go:106) and in the constant block (sr_origination_v6.go:33), and no producer builds an External-Prefix TLV or an E-AS-External / E-Type-7 body carrying sr.V6TypePrefixSID. With no ASBR Prefix-SID advertisement there is no flag-setting code to be right or wrong about |
| `RFC8666-6-11` | If NP-Flag not set, any upstream neighbor of the originator MUST pop the Prefix-SID (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC8666PHPPopsPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L123). **negative:** `unit/verify` [`TestRFC8666NoPHPKeepsPrefixSIDOnStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L97) |
| `RFC8666-6-12` | If NP-Flag set and E-Flag not set, any upstream neighbor MUST keep the Prefix-SID on top of the stack (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC8666NoPHPKeepsPrefixSIDOnStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L95). **negative:** `unit/verify` [`TestRFC8666PHPPopsPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L129) |
| `RFC8666-6-13` | If both NP-Flag and E-Flag set, any upstream neighbor MUST replace the Prefix-SID with an Explicit NULL label (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC8666ExplicitNullReplacesPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L149). **negative:** `unit/verify` [`TestRFC8666NoPHPKeepsPrefixSIDOnStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L101) |
| `RFC8666-6-14` | When the M-Flag is set, the NP-Flag and E-Flag MUST be ignored on reception (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC8666MappingServerFlagIgnoresNPAndE`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L168). **negative:** `unit/verify` [`TestRFC8666WithoutMappingServerFlagNPAndEApply`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L187) |
| `RFC8666-7.1-1` | Adj-SID Flags reserved bits (5-7) MUST be zero when sent and are ignored when received (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestRFC8666AdjSIDReservedFlagBitsZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L125). **negative:** `unit/verify` [`TestRFC8666AdjSIDReservedFlagBitsIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L138) |
| `RFC8666-7.1-2` | Reserved field (Adj-SID) MUST be ignored on reception (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestRFC8666AdjSIDReservedFieldIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L158). **negative:** `unit/verify` [`TestRFC8666AdjSIDWeightOctetNotIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L181) |
| `RFC8666-7.1-3` | When the P-Flag is set, the Adj-SID MUST be persistent (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestRFC8666AdjSIDPersistentFlagClearForNonPersistentAllocation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L379). **negative:** no negative test. **{single-polarity}:** ze's Adj-SID allocation is deliberately non-persistent -- the SRLB label is taken when the neighbor reaches Full and returned to the allocator when it leaves (srAdjManager.neighborFull, sr_adjsid.go:50-74; neighborLost, sr_adjsid.go:92-104) -- and the advertised flags are correspondingly V/L only, never P (sr_adjsid.go:63). A negative is not meaningful: no code path can advertise P, so there is no P-set advertisement whose persistence could be violated |
| `RFC8666-7.2-1` | Reserved field (LAN Adj-SID) MUST be ignored on reception (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestRFC8666LANAdjSIDReservedFieldIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L197). **negative:** `unit/verify` [`TestRFC8666LANAdjSIDNeighborIDNotIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L221) |
| `RFC8666-7.2-2` | When the P-Flag is set, the LAN Adjacency SID MUST be persistent (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestRFC8666AdjSIDPersistentFlagClearForNonPersistentAllocation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L383). **negative:** no negative test. **{single-polarity}:** the LAN form shares the allocation path and the flags octet with the Adj-SID (srAdjManager.neighborFull sets IsLAN on the same sr.AdjSID, sr_adjsid.go:62-69, and v6AdjSubTLV picks the LAN sub-TLV from it, sr_origination_v6.go:177-182), so it is equally non-persistent and equally never sets P. A negative is not meaningful for the same reason as RFC8666-7.1-3 |
| `RFC8666-8.1-1` | Multiple Mapping Servers advertising the same prefix MUST advertise the same Prefix-SID (§8.1) | MUST | 8.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is not an SR Mapping Server. grep for SRMS / MappingServer over internal/plugins/ospf finds only the RFC 8665 §3.4 SRMS Preference TLV of the IPv4 RI LSA (srBuildSRMS, sr.go:187-193) and its decode (sr.go:349-358) -- a preference value, not a mapping advertisement. Every Prefix-SID ze originates from configuration is for a prefix ze itself advertises reachability to, built from the operator's NoPHP/ExplicitNull leaves alone (sr.go:203-208, sr_origination_v6.go:219). ze CAN emit an M-flagged Prefix-SID, but only by propagation, not by mapping: the ABR inter-area rule copies a RECEIVED sr.PrefixSID whole (`out := src`, sr_interarea_v6.go:36) and overrides only NP and E (:40-41), so a received M survives into EncodePrefixSIDValueV6 -> SIDFlags.toByte, which sets flagM (sr/codec_v6.go:49, sr/codec.go:92-94). That is RFC 8666 §8.2 inter-area propagation of another node's advertisement, not this node advertising a prefix-to-SID mapping of its own, so there is still no Mapping Server advertisement of ze's that could disagree with another server's |
| `RFC8666-8.1-2` | An SR Mapping Server MUST use the OSPFv3 Extended Prefix Range TLVs when advertising SIDs for prefixes (§8.1) | MUST | 8.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** same grep evidence as RFC8666-8.1-1 -- ze has no Mapping Server. ze does emit the Extended Prefix Range TLV, but from the ABR inter-area propagation path (v6EInterAreaPrefixBody, sr_interarea_v6.go:48-54), which is §8.2 carriage and not a Mapping Server advertisement |
| `RFC8666-8.1-3` | The NU-bit MUST be set in the PrefixOptions field of the LSA used by the Mapping Server to advertise SID or SID Range (§8.1) | MUST | 8.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** same evidence as RFC8666-8.1-1 -- with no Mapping Server advertisement there is no LSA that must carry the NU-bit. The NU-bit constant exists (OptPrefixNU, v3/types/prefix.go:50) and no producer sets it, which is correct here: every Prefix-SID-bearing LSA ze originates is either its own configured prefix (sr_origination_v6.go:219) or an ABR §8.2 re-advertisement of a prefix another node reaches (sr_interarea_v6.go:35-42, carried by v6EInterAreaPrefixBody :48-54), and both advertise reachability, so both must NOT set NU |
| `RFC8666-8.2-1` | OSPFv3 MUST propagate Prefix-SID information between areas to support multi-area SR (§8.2) | MUST | 8.2 | **positive:** `unit/verify` [`TestOSPFv3OriginateInterAreaPropagation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_interarea_v6_test.go#L38). **negative:** `unit/verify` [`TestOSPFv3OriginateInterAreaPropagation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_interarea_v6_test.go#L41) |
| `RFC8666-8.4.1-1` | If a P2P adjacency transitions to a state lower than 2-Way, the Adj-SID advertisement MUST be withdrawn from the area (§8.4.1) | MUST | 8.4.1 | **positive:** `unit/verify` [`TestRFC8666AdjSIDWithdrawnWhenAdjacencyDrops`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L424). **negative:** `unit/verify` [`TestOSPFv3ERouterBodyCarriesAdjSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_origination_v6_test.go#L21) |
| `RFC8666-10-1` | If a TLV/sub-TLV length is invalid, the LSA MUST be ignored (§10) | MUST | 10 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the rule holds at the TOP-LEVEL TLV layer only. An invalid top-level TLV length makes DecodeExtendedLSABody return an error (v3/packet/lsa_extended.go:42-54) and the reader drops the whole LSA (sr_reception_v6.go:67-71). An invalid SUB-TLV length inside an otherwise-parseable TLV does not: v6PrefixSIDFromTLV returns false for that one TLV (sr_reception_v6.go:94-111, 129-143) and the caller's loop keeps consuming the remaining TLVs of the same LSA (sr_reception_v6.go:72-85), so the other Prefix-SIDs of a malformed LSA are still installed instead of the LSA being ignored. Disclosed in docs/features/rfc-status.md RFC 8666 row |
| `RFC8666-11-1` | Implementations MUST ensure malformed TLVs/sub-TLVs are detected and do not let an attacker crash the OSPFv3 router or routing process (§11) | MUST | 11 | **positive:** `unit/verify` [`TestOSPFv3ReceptionMalformedNoPanic`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_reception_v6_test.go#L168). **positive:** `unit/verify` [`TestOSPFv3SRTLVMalformed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/codec_v6_test.go#L127). **negative:** `unit/verify` [`TestOSPFv3ExtPrefixRangeTLVRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/codec_v6_test.go#L96) |
| `RFC8666-5-5` | Reserved field of Extended Prefix Range TLV SHOULD be set to 0 on transmission (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-6-15` | Reserved field of Prefix-SID sub-TLV SHOULD be set to 0 on transmission (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-6-16` | PHP behavior SHOULD be applied for Mapping-Server SIDs in the three enumerated downstream cases (originator intra-area; ABR inter-area with LA-bit; ASBR external with LA-bit) (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-7.1-4` | Reserved field of Adj-SID sub-TLV SHOULD be set to 0 on transmission (§7.1) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-7.2-3` | Reserved field of LAN Adj-SID sub-TLV SHOULD be set to 0 on transmission (§7.2) | SHOULD | 7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-8.3-1` | An ASBR supporting SR SHOULD include a Prefix-SID sub-TLV when originating an E-AS-External-LSA (§8.3) | SHOULD | 8.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-8.3-2` | An NSSA ABR translating an E-NSSA-LSA into an E-AS-External-LSA SHOULD also advertise the Prefix-SID (§8.3) | SHOULD | 8.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-10-2` | Errors (invalid TLV/sub-TLV length) SHOULD be logged subject to rate limiting (§10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-11-2` | Reception of a malformed TLV or sub-TLV SHOULD be counted and/or logged for further analysis (§11) | SHOULD | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-11-3` | Logging of malformed TLVs and sub-TLVs SHOULD be rate limited to prevent a DoS attack from overloading the OSPFv3 control plane (§11) | SHOULD | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-11-4` | Stronger authentication mechanisms ([RFC4552] or [RFC7166]) SHOULD be used in deployments where potential attackers have network access (§11) | SHOULD | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-5-6` | Multiple OSPFv3 Extended Prefix Range TLVs MAY be advertised in each eligible LSA (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-6-17` | The Prefix-SID sub-TLV MAY appear more than once in the parent TLV (§6) | MAY | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-7.1-5` | An SR-capable router MAY allocate an Adj-SID for each adjacency and set the B-Flag when FRR-eligible (§7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-7.1-6` | When the G-Flag is set, the Adj-SID MAY be assigned to other adjacencies (§7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-7.1-7` | An SR-capable router MAY allocate more than one Adj-SID to an adjacency (§7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-7.1-8` | An SR-capable router MAY allocate the same Adj-SID to different adjacencies (§7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-7.1-9` | The Adj-SID sub-TLV MAY appear multiple times in the Router-Link TLV (§7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-7.1-10` | When the P-Flag is not set, the Adj-SID MAY be persistent (§7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-7.2-4` | The LAN Adj-SID sub-TLV MAY appear multiple times in the Router-Link TLV (§7.2) | MAY | 7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-7.2-5` | When the P-Flag is not set, the LAN Adjacency SID MAY be persistent (§7.2) | MAY | 7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-8.1-4` | An OSPFv3 router that supports SR MAY advertise Prefix-SIDs for any prefix to which it advertises reachability (§8.1) | MAY | 8.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-8.4.1-2` | An Adj-SID MAY be advertised for any P2P adjacency in neighbor state 2-Way or higher (§8.4.1) | MAY | 8.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-8.4.1-3` | If a P2P adjacency transitions from the FULL state, the Adj-SID for that adjacency MAY be removed from the area (§8.4.1) | MAY | 8.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-8.4.2-1` | Each router on a broadcast/NBMA/hybrid network MAY advertise the Adj-SID for its adjacency to the DR (§8.4.2) | MAY | 8.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-8.4.2-2` | SR-capable routers MAY advertise a LAN Adjacency SID for other neighbors (BDR, DR-OTHER, etc.) on broadcast/NBMA/hybrid networks (§8.4.2) | MAY | 8.4.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8666-5-3`](#rfc8666-5-3) Duplicate Extended Prefix Range TLV (same range, same type, same originator): LSA with numerically smallest Instance ID MUST be used (§5) | {gap}, no test | reception aggregates every Extended Prefix Range TLV found in the LSDB in whatever order LSAViewsByType returns and keys the result by prefix alone (v6ReceivedPrefixSIDs, sr_reception_v6.go:56-88; srRemotePrefixSIDsV6, sr_reception_v6.go:163-178). Neither reader consults the Link State ID / Instance ID of the carrying LSA, so two LSAs of the same type from one originator carrying the same range are resolved by "first seen wins, conflicting second marks the prefix Duplicate", not by smallest Instance ID. Disclosed in docs/features/rfc-status.md RFC 8666 row |
| [`RFC8666-5-4`](#rfc8666-5-4) Subsequent instances of a duplicate Extended Prefix Range TLV MUST be ignored (§5) | {gap}, no test | same producer as RFC8666-5-3. A subsequent instance carrying an identical SID is skipped only because it compares equal (v6PrefixSIDEqual, sr_reception_v6.go:182), and a subsequent instance carrying a different SID makes ze ignore BOTH by setting Duplicate (sr_reception_v6.go:171-176) instead of keeping the smallest-Instance-ID one. Disclosed in docs/features/rfc-status.md RFC 8666 row |
| [`RFC8666-6-10`](#rfc8666-6-10) NP-Flag set, E-Flag clear, for redistributed prefixes (unless directly attached to the ASBR) (§6) | no test | no test carries this requirement id; annotated {not-applicable}: the OSPFv3 SR origination never attaches a Prefix-SID to a redistributed prefix. v6OriginateSR builds only the E-Router-LSA and the E-Intra-Area-Prefix-LSA for configured node prefixes, plus the ABR inter-area propagation (sr_origination_v6.go:66-100); grep for extTLVExternalPrefix over internal/plugins/ospf finds it only in the RECEPTION switch (sr_reception_v6.go:106) and in the constant block (sr_origination_v6.go:33), and no producer builds an External-Prefix TLV or an E-AS-External / E-Type-7 body carrying sr.V6TypePrefixSID. With no ASBR Prefix-SID advertisement there is no flag-setting code to be right or wrong about |
| [`RFC8666-8.1-1`](#rfc8666-8.1-1) Multiple Mapping Servers advertising the same prefix MUST advertise the same Prefix-SID (§8.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze is not an SR Mapping Server. grep for SRMS / MappingServer over internal/plugins/ospf finds only the RFC 8665 §3.4 SRMS Preference TLV of the IPv4 RI LSA (srBuildSRMS, sr.go:187-193) and its decode (sr.go:349-358) -- a preference value, not a mapping advertisement. Every Prefix-SID ze originates from configuration is for a prefix ze itself advertises reachability to, built from the operator's NoPHP/ExplicitNull leaves alone (sr.go:203-208, sr_origination_v6.go:219). ze CAN emit an M-flagged Prefix-SID, but only by propagation, not by mapping: the ABR inter-area rule copies a RECEIVED sr.PrefixSID whole (`out := src`, sr_interarea_v6.go:36) and overrides only NP and E (:40-41), so a received M survives into EncodePrefixSIDValueV6 -> SIDFlags.toByte, which sets flagM (sr/codec_v6.go:49, sr/codec.go:92-94). That is RFC 8666 §8.2 inter-area propagation of another node's advertisement, not this node advertising a prefix-to-SID mapping of its own, so there is still no Mapping Server advertisement of ze's that could disagree with another server's |
| [`RFC8666-8.1-2`](#rfc8666-8.1-2) An SR Mapping Server MUST use the OSPFv3 Extended Prefix Range TLVs when advertising SIDs for prefixes (§8.1) | no test | no test carries this requirement id; annotated {not-applicable}: same grep evidence as RFC8666-8.1-1 -- ze has no Mapping Server. ze does emit the Extended Prefix Range TLV, but from the ABR inter-area propagation path (v6EInterAreaPrefixBody, sr_interarea_v6.go:48-54), which is §8.2 carriage and not a Mapping Server advertisement |
| [`RFC8666-8.1-3`](#rfc8666-8.1-3) The NU-bit MUST be set in the PrefixOptions field of the LSA used by the Mapping Server to advertise SID or SID Range (§8.1) | no test | no test carries this requirement id; annotated {not-applicable}: same evidence as RFC8666-8.1-1 -- with no Mapping Server advertisement there is no LSA that must carry the NU-bit. The NU-bit constant exists (OptPrefixNU, v3/types/prefix.go:50) and no producer sets it, which is correct here: every Prefix-SID-bearing LSA ze originates is either its own configured prefix (sr_origination_v6.go:219) or an ABR §8.2 re-advertisement of a prefix another node reaches (sr_interarea_v6.go:35-42, carried by v6EInterAreaPrefixBody :48-54), and both advertise reachability, so both must NOT set NU |
| [`RFC8666-10-1`](#rfc8666-10-1) If a TLV/sub-TLV length is invalid, the LSA MUST be ignored (§10) | {gap}, no test | the rule holds at the TOP-LEVEL TLV layer only. An invalid top-level TLV length makes DecodeExtendedLSABody return an error (v3/packet/lsa_extended.go:42-54) and the reader drops the whole LSA (sr_reception_v6.go:67-71). An invalid SUB-TLV length inside an otherwise-parseable TLV does not: v6PrefixSIDFromTLV returns false for that one TLV (sr_reception_v6.go:94-111, 129-143) and the caller's loop keeps consuming the remaining TLVs of the same LSA (sr_reception_v6.go:72-85), so the other Prefix-SIDs of a malformed LSA are still installed instead of the LSA being ignored. Disclosed in docs/features/rfc-status.md RFC 8666 row |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8666-5-1`](#rfc8666-5-1)

Range Size MUST NOT exceed prefixes satisfiable by Prefix Length, excluding IPv4 multicast 224.0.0.0/3 (IPv4) and non-unicast addresses (IPv6) (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8666ExtPrefixRangeSizeWithinPrefixLength`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L355) | unit/verify | unproven |

### [`RFC8666-5-2`](#rfc8666-5-2)

Flags field of Extended Prefix Range TLV MUST be zero when sent and is ignored when received (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666ExtPrefixRangeFlagsIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L25) | unit/verify | unproven |
| positive | [`TestRFC8666ExtPrefixRangeFlagsZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L15) | unit/verify | unproven |

### [`RFC8666-5-7`](#rfc8666-5-7)

Reserved field of Extended Prefix Range TLV MUST be ignored on reception (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666V6ExtPrefixRangeAFOctetNotIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/codec_v6_test.go#L221) | unit/verify | unproven |
| positive | [`TestRFC8666V6ExtPrefixRangeReservedIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/codec_v6_test.go#L189) | unit/verify | unproven |

### [`RFC8666-5-3`](#rfc8666-5-3)

Duplicate Extended Prefix Range TLV (same range, same type, same originator): LSA with numerically smallest Instance ID MUST be used (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8666-5-3, so no unit is bound to it.

### [`RFC8666-5-4`](#rfc8666-5-4)

Subsequent instances of a duplicate Extended Prefix Range TLV MUST be ignored (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8666-5-4, so no unit is bound to it.

### [`RFC8666-6-1`](#rfc8666-6-1)

If NP-Flag set, penultimate hop MUST NOT pop the Prefix-SID before delivering to the advertising node (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666PHPPopsPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L126) | unit/verify | unproven |
| positive | [`TestRFC8666NoPHPKeepsPrefixSIDOnStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L92) | unit/verify | unproven |

### [`RFC8666-6-2`](#rfc8666-6-2)

If E-Flag set, upstream neighbor MUST replace the Prefix-SID with the Explicit NULL label (0 IPv4, 2 IPv6) before forwarding (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666NoPHPKeepsPrefixSIDOnStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L99) | unit/verify | unproven |
| positive | [`TestRFC8666ExplicitNullReplacesPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L147) | unit/verify | unproven |

### [`RFC8666-6-3`](#rfc8666-6-3)

Prefix-SID Flags reserved bits (0, 6, 7) MUST be zero when sent and are ignored when received (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666PrefixSIDReservedFlagBitsIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L66) | unit/verify | unproven |
| positive | [`TestRFC8666PrefixSIDReservedFlagBitsZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L52) | unit/verify | unproven |

### [`RFC8666-6-4`](#rfc8666-6-4)

Reserved field (Prefix-SID) MUST be ignored on reception (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666PrefixSIDAlgorithmOctetNotIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L109) | unit/verify | unproven |
| positive | [`TestRFC8666PrefixSIDReservedFieldIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L86) | unit/verify | unproven |

### [`RFC8666-6-5`](#rfc8666-6-5)

A Prefix-SID with an Algorithm value not advertised by the remote node in its SR-Algorithm TLV MUST be ignored (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666PrefixSIDUnadvertisedAlgorithmIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L212) | unit/verify | unproven |
| positive | [`TestRFC8666PrefixSIDAdvertisedAlgorithmInstalls`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L201) | unit/verify | unproven |

### [`RFC8666-6-6`](#rfc8666-6-6)

A SID advertisement with an invalid V-/L-Flag combination MUST be ignored (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFv3PrefixSIDCodec`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/codec_v6_test.go#L32) | unit/verify | unproven |
| positive | [`TestOSPFv3SIDWidthFromVL`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/codec_v6_test.go#L59) | unit/verify | unproven |

### [`RFC8666-6-7`](#rfc8666-6-7)

If a router advertises multiple Prefix-SIDs for the same prefix, topology, and algorithm, all MUST be ignored (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFv3ReceptionSameSIDNotDuplicate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_reception_v6_test.go#L151) | unit/verify | unproven |
| positive | [`TestRFC8666DuplicatePrefixSIDsDetectedAndIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L261) | unit/verify | unproven |

### [`RFC8666-6-8`](#rfc8666-6-8)

When computing the outgoing label, the router MUST take the E-, NP-, and M-Flags of the next-hop router into account, regardless of whether it contributes to the best path (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666TransitHopIgnoresOriginatorPHPFlags`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L337) | unit/verify | unproven |
| positive | [`TestRFC8666OutgoingLabelUsesNextHopRouter`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L318) | unit/verify | unproven |

### [`RFC8666-6-9`](#rfc8666-6-9)

NP-Flag set, E-Flag clear, for inter-area-propagated prefixes by an ABR (unless directly attached to the ABR) (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFv3InterAreaPrefixSIDRule`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_interarea_v6_test.go#L21) | unit/verify | unproven |
| positive | [`TestOSPFv3InterAreaPrefixSIDRule`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_interarea_v6_test.go#L19) | unit/verify | unproven |

### [`RFC8666-6-10`](#rfc8666-6-10)

NP-Flag set, E-Flag clear, for redistributed prefixes (unless directly attached to the ASBR) (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8666-6-10, so no unit is bound to it.

### [`RFC8666-6-11`](#rfc8666-6-11)

If NP-Flag not set, any upstream neighbor of the originator MUST pop the Prefix-SID (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666NoPHPKeepsPrefixSIDOnStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L97) | unit/verify | unproven |
| positive | [`TestRFC8666PHPPopsPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L123) | unit/verify | unproven |

### [`RFC8666-6-12`](#rfc8666-6-12)

If NP-Flag set and E-Flag not set, any upstream neighbor MUST keep the Prefix-SID on top of the stack (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666PHPPopsPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L129) | unit/verify | unproven |
| positive | [`TestRFC8666NoPHPKeepsPrefixSIDOnStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L95) | unit/verify | unproven |

### [`RFC8666-6-13`](#rfc8666-6-13)

If both NP-Flag and E-Flag set, any upstream neighbor MUST replace the Prefix-SID with an Explicit NULL label (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666NoPHPKeepsPrefixSIDOnStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L101) | unit/verify | unproven |
| positive | [`TestRFC8666ExplicitNullReplacesPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L149) | unit/verify | unproven |

### [`RFC8666-6-14`](#rfc8666-6-14)

When the M-Flag is set, the NP-Flag and E-Flag MUST be ignored on reception (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666WithoutMappingServerFlagNPAndEApply`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L187) | unit/verify | unproven |
| positive | [`TestRFC8666MappingServerFlagIgnoresNPAndE`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L168) | unit/verify | unproven |

### [`RFC8666-7.1-1`](#rfc8666-7.1-1)

Adj-SID Flags reserved bits (5-7) MUST be zero when sent and are ignored when received (§7.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666AdjSIDReservedFlagBitsIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L138) | unit/verify | unproven |
| positive | [`TestRFC8666AdjSIDReservedFlagBitsZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L125) | unit/verify | unproven |

### [`RFC8666-7.1-2`](#rfc8666-7.1-2)

Reserved field (Adj-SID) MUST be ignored on reception (§7.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666AdjSIDWeightOctetNotIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L181) | unit/verify | unproven |
| positive | [`TestRFC8666AdjSIDReservedFieldIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L158) | unit/verify | unproven |

### [`RFC8666-7.1-3`](#rfc8666-7.1-3)

When the P-Flag is set, the Adj-SID MUST be persistent (§7.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8666AdjSIDPersistentFlagClearForNonPersistentAllocation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L379) | unit/verify | unproven |

### [`RFC8666-7.2-1`](#rfc8666-7.2-1)

Reserved field (LAN Adj-SID) MUST be ignored on reception (§7.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666LANAdjSIDNeighborIDNotIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L221) | unit/verify | unproven |
| positive | [`TestRFC8666LANAdjSIDReservedFieldIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L197) | unit/verify | unproven |

### [`RFC8666-7.2-2`](#rfc8666-7.2-2)

When the P-Flag is set, the LAN Adjacency SID MUST be persistent (§7.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8666AdjSIDPersistentFlagClearForNonPersistentAllocation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L383) | unit/verify | unproven |

### [`RFC8666-8.1-1`](#rfc8666-8.1-1)

Multiple Mapping Servers advertising the same prefix MUST advertise the same Prefix-SID (§8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8666-8.1-1, so no unit is bound to it.

### [`RFC8666-8.1-2`](#rfc8666-8.1-2)

An SR Mapping Server MUST use the OSPFv3 Extended Prefix Range TLVs when advertising SIDs for prefixes (§8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8666-8.1-2, so no unit is bound to it.

### [`RFC8666-8.1-3`](#rfc8666-8.1-3)

The NU-bit MUST be set in the PrefixOptions field of the LSA used by the Mapping Server to advertise SID or SID Range (§8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8666-8.1-3, so no unit is bound to it.

### [`RFC8666-8.2-1`](#rfc8666-8.2-1)

OSPFv3 MUST propagate Prefix-SID information between areas to support multi-area SR (§8.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFv3OriginateInterAreaPropagation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_interarea_v6_test.go#L41) | unit/verify | unproven |
| positive | [`TestOSPFv3OriginateInterAreaPropagation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_interarea_v6_test.go#L38) | unit/verify | unproven |

### [`RFC8666-8.4.1-1`](#rfc8666-8.4.1-1)

If a P2P adjacency transitions to a state lower than 2-Way, the Adj-SID advertisement MUST be withdrawn from the area (§8.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFv3ERouterBodyCarriesAdjSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_origination_v6_test.go#L21) | unit/verify | unproven |
| positive | [`TestRFC8666AdjSIDWithdrawnWhenAdjacencyDrops`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L424) | unit/verify | unproven |

### [`RFC8666-10-1`](#rfc8666-10-1)

If a TLV/sub-TLV length is invalid, the LSA MUST be ignored (§10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8666-10-1, so no unit is bound to it.

### [`RFC8666-11-1`](#rfc8666-11-1)

Implementations MUST ensure malformed TLVs/sub-TLVs are detected and do not let an attacker crash the OSPFv3 router or routing process (§11)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFv3ExtPrefixRangeTLVRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/codec_v6_test.go#L96) | unit/verify | unproven |
| positive | [`TestOSPFv3SRTLVMalformed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/codec_v6_test.go#L127) | unit/verify | unproven |
| positive | [`TestOSPFv3ReceptionMalformedNoPanic`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_reception_v6_test.go#L168) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 8666, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 8666, so its obligations are stated where they were written.
