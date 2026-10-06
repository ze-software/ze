# RFC 8666 - OSPFv3 Extensions for Segment Routing

Partial. Every requirement this repository extracted from RFC 8666, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 71.0% | 22 of 31 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 9.7% | 3 of 31 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 31 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 31 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 26.7% | 16 of 60 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 31 | of 57 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 4 | of 31 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 12.9% | 4 of 31 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 31 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 31 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 6.5% | 2 of 31 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 31 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 57 |
| Gated MUST-level | 31 |
| Not applicable, so out of scope | 4 |
| Declared gaps | 2 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 60 |
| Tagged units | 60 |
| Recorded audit verdicts | 23 |
| Discrimination records | 16 |
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

Two MUST gaps, annotated in [`rfc/short/rfc8666.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc8666.md) and gated by `./le rfc check`.

- **RFC8666-5-3 and RFC8666-5-4:** reception judges Prefix-SIDs per prefix, topology, algorithm and advertising router but never consults the carrying LSA's Instance ID (srPrefixSIDScope, [`internal/plugins/ospf/sr_malformed.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_malformed.go)), so a range repeated across LSAs of one type from one originator resolves by first-seen / conflict-ignores-both rather than by smallest Instance ID. The §8.1 SR Mapping Server role and ASBR external Prefix-SID origination have no producer at all (annotated not-applicable). Same OSPF experimental status.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 22 | one part of the gated population |
| Annotated (including scoped evidence) | 9 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **31** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (22):** [`RFC8666-5-2`](#rfc8666-5-2), [`RFC8666-5-7`](#rfc8666-5-7), [`RFC8666-6-1`](#rfc8666-6-1), [`RFC8666-6-2`](#rfc8666-6-2), [`RFC8666-6-3`](#rfc8666-6-3), [`RFC8666-6-4`](#rfc8666-6-4), [`RFC8666-6-5`](#rfc8666-6-5), [`RFC8666-6-6`](#rfc8666-6-6), [`RFC8666-6-7`](#rfc8666-6-7), [`RFC8666-6-8`](#rfc8666-6-8), [`RFC8666-6-9`](#rfc8666-6-9), [`RFC8666-6-11`](#rfc8666-6-11), [`RFC8666-6-12`](#rfc8666-6-12), [`RFC8666-6-13`](#rfc8666-6-13), [`RFC8666-6-14`](#rfc8666-6-14), [`RFC8666-7.1-1`](#rfc8666-7.1-1), [`RFC8666-7.1-2`](#rfc8666-7.1-2), [`RFC8666-7.2-1`](#rfc8666-7.2-1), [`RFC8666-8.2-1`](#rfc8666-8.2-1), [`RFC8666-8.4.1-1`](#rfc8666-8.4.1-1), [`RFC8666-10-1`](#rfc8666-10-1), [`RFC8666-11-1`](#rfc8666-11-1)

**Annotated (including scoped evidence) (9):** [`RFC8666-5-1`](#rfc8666-5-1), [`RFC8666-5-3`](#rfc8666-5-3), [`RFC8666-5-4`](#rfc8666-5-4), [`RFC8666-6-10`](#rfc8666-6-10), [`RFC8666-7.1-3`](#rfc8666-7.1-3), [`RFC8666-7.2-2`](#rfc8666-7.2-2), [`RFC8666-8.1-1`](#rfc8666-8.1-1), [`RFC8666-8.1-2`](#rfc8666-8.1-2), [`RFC8666-8.1-3`](#rfc8666-8.1-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8666-5-1` | The Range Size MUST NOT exceed the number of prefixes that could be satisfied by the Prefix Length without including: Addresses from the IPv4 multicast address range (224.0.0.0/3), if the AF is IPv4 unicast. Addresses other than the IPv6 unicast addresses, if the AF is IPv6 unicast. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC8666ExtPrefixRangeSizeWithinPrefixLength`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L399). **negative:** no negative test. **{single-polarity}:** the only producer of an IPv6 Extended Prefix Range TLV is the ABR inter-area propagation, which advertises one prefix per TLV with a hardcoded Range Size of 1 (v6EInterAreaPrefixBody, sr_interarea_v6.go:50), so the advertised size is always within what the Prefix Length can satisfy. A negative is not meaningful: ze has no code path that can compute an oversize Range Size to be rejected, and the RFC constrains the sender only |
| `RFC8666-5-2` | Flags: Reserved. MUST be zero when sent and are ignored when received. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC8666ExtPrefixRangeFlagsZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L15). **negative:** `unit/verify` [`TestRFC8666ExtPrefixRangeFlagsIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L25) |
| `RFC8666-5-7` | Reserved: SHOULD be set to 0 on transmission and MUST be ignored on reception. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC8666V6ExtPrefixRangeReservedIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_codec_v6_test.go#L189). **negative:** `unit/verify` [`TestRFC8666V6ExtPrefixRangeAFOctetNotIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_codec_v6_test.go#L221) |
| `RFC8666-5-3` | If the OSPFv3 Extended Prefix Range TLVs advertising the exact same range appears in multiple LSAs of the same type, originated by the same OSPFv3 router, the LSA with the numerically smallest Instance ID MUST be used (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** reception aggregates every Extended Prefix Range TLV found in the LSDB and judges the starting Prefix-SIDs per prefix, topology, algorithm and advertising router in each area (v6ReceivedPrefixSIDs, internal/plugins/ospf/sr_reception_v6.go; srPrefixSIDScope.resolve, internal/plugins/ospf/sr_malformed.go). Neither reader consults the Link State ID / Instance ID of the carrying LSA, so two LSAs of the same type from one originator carrying the same range give that router two Prefix-SIDs for the prefix, and both are ignored under RFC 8666 Section 6, rather than the smallest-Instance-ID one being used. Disclosed in docs/features/rfc-status.md RFC 8666 row |
| `RFC8666-5-4` | If the OSPFv3 Extended Prefix Range TLVs advertising the exact same range appears in multiple LSAs of the same type, originated by the same OSPFv3 router, the LSA with the numerically smallest Instance ID MUST be used, and subsequent instances of the OSPFv3 Extended Prefix Range TLVs MUST be ignored. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** same producer as RFC8666-5-3. A subsequent instance from the same router, carrying the same SID or a different one, makes ze ignore every instance by marking the prefix Duplicate (srPrefixSIDScope.resolve, internal/plugins/ospf/sr_malformed.go) instead of keeping the smallest-Instance-ID one. Disclosed in docs/features/rfc-status.md RFC 8666 row |
| `RFC8666-6-1` | If set, then the penultimate hop MUST NOT pop the Prefix-SID before delivering packets to the node that advertised the Prefix- SID. (§6) | MUST NOT | 6 | **positive:** `unit/verify` [`TestRFC8666NoPHPKeepsPrefixSIDOnStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L92). **negative:** `unit/verify` [`TestRFC8666PHPPopsPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L126) |
| `RFC8666-6-2` | If set, any upstream neighbor of the Prefix-SID originator MUST replace the Prefix-SID with the Explicit NULL label (0 for IPv4, 2 for IPv6) before forwarding the packet. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC8666ExplicitNullReplacesPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L147). **negative:** `unit/verify` [`TestRFC8666NoPHPKeepsPrefixSIDOnStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L99) |
| `RFC8666-6-3` | Other bits: Reserved. These MUST be zero when sent and are ignored when received. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC8666PrefixSIDReservedFlagBitsZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L52). **negative:** `unit/verify` [`TestRFC8666PrefixSIDReservedFlagBitsIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L66) |
| `RFC8666-6-4` | Reserved: SHOULD be set to 0 on transmission and MUST be ignored on reception. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC8666PrefixSIDReservedFieldIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L86). **negative:** `unit/verify` [`TestRFC8666PrefixSIDAlgorithmOctetNotIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L109) |
| `RFC8666-6-5` | A router receiving a Prefix-SID from a remote node and with an algorithm value that the remote node has not advertised in the SR-Algorithm TLV [RFC8665] MUST ignore the Prefix-SID sub-TLV. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC8666PrefixSIDAdvertisedAlgorithmInstalls`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L201). **negative:** `unit/verify` [`TestRFC8666PrefixSIDUnadvertisedAlgorithmIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L212) |
| `RFC8666-6-6` | All other combinations of V-Flag and L-Flag are invalid and any SID Advertisement received with an invalid setting for V- and L-Flags MUST be ignored. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestOSPFv3SIDWidthFromVL`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_codec_v6_test.go#L59). **negative:** `unit/verify` [`TestOSPFv3PrefixSIDCodec`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_codec_v6_test.go#L32) |
| `RFC8666-6-7` | If an OSPFv3 router advertises multiple Prefix-SIDs for the same prefix, topology, and algorithm, all of them MUST be ignored. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC8666OneRouterMultiplePrefixSIDsAllIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_prefix_sid_per_router_test.go#L107). **positive:** `unit/verify` [`TestRFC8666OneRouterRepeatedPrefixSIDInstallsNothing`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_duplicate_install_test.go#L51). **positive:** `unit/verify` [`TestRFC8666PrefixSIDsKeyedByAlgorithm`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_malformed_test.go#L99). **negative:** `unit/verify` [`TestRFC8666OneRouterMultiplePrefixSIDsAllIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_prefix_sid_per_router_test.go#L123). **negative:** `unit/verify` [`TestRFC8666OneRouterRepeatedPrefixSIDInstallsNothing`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_duplicate_install_test.go#L42). **negative:** `unit/verify` [`TestRFC8666PrefixSIDsKeyedByAlgorithm`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_malformed_test.go#L109) |
| `RFC8666-6-8` | When calculating the outgoing label for the prefix, the router MUST take into account, as described below, the E-, NP-, and M-Flags advertised by the next-hop router if that router advertised the SID for the prefix. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC8666NextHopAdvertisedFlagsShapeOutgoingLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L363). **positive:** `unit/verify` [`TestRFC8666OutgoingLabelUsesNextHopRouter`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L320). **negative:** `unit/verify` [`TestRFC8666TransitHopIgnoresOriginatorPHPFlags`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L339) |
| `RFC8666-6-9` | The NP-Flag (No-PHP) MUST be set and the E-Flag MUST be clear for Prefix-SIDs allocated to prefixes that are propagated between areas by an ABR based on intra-area or inter-area reachability, unless the advertised prefix is directly attached to such ABR. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestOSPFv3InterAreaPrefixSIDRule`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_sr_interarea_v6_test.go#L19). **negative:** `unit/verify` [`TestOSPFv3InterAreaPrefixSIDRule`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_sr_interarea_v6_test.go#L21) |
| `RFC8666-6-10` | The NP-Flag (No-PHP) MUST be set and the E-Flag MUST be clear for Prefix-SIDs allocated to redistributed prefixes, unless the redistributed prefix is directly attached to the advertising ASBR. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the OSPFv3 SR origination never attaches a Prefix-SID to a redistributed prefix. v6OriginateSR builds only the E-Router-LSA and the E-Intra-Area-Prefix-LSA for configured node prefixes, plus the ABR inter-area propagation (sr_origination_v6.go:66-100); grep for extTLVExternalPrefix over internal/plugins/ospf finds it only in the RECEPTION switch (sr_reception_v6.go:106) and in the constant block (sr_origination_v6.go:33), and no producer builds an External-Prefix TLV or an E-AS-External / E-Type-7 body carrying sr.V6TypePrefixSID. With no ASBR Prefix-SID advertisement there is no flag-setting code to be right or wrong about |
| `RFC8666-6-11` | If the NP-Flag is not set, then: Any upstream neighbor of the Prefix-SID originator MUST pop the Prefix-SID. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC8666PHPPopsPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L123). **negative:** `unit/verify` [`TestRFC8666NoPHPKeepsPrefixSIDOnStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L97) |
| `RFC8666-6-12` | If the NP-Flag is set and the E-Flag is not set, then: Any upstream neighbor of the Prefix-SID originator MUST keep the Prefix-SID on top of the stack. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC8666NoPHPKeepsPrefixSIDOnStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L95). **negative:** `unit/verify` [`TestRFC8666PHPPopsPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L129) |
| `RFC8666-6-13` | Any upstream neighbor of the Prefix-SID originator MUST replace the Prefix-SID with an Explicit NULL label. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC8666ExplicitNullReplacesPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L149). **negative:** `unit/verify` [`TestRFC8666NoPHPKeepsPrefixSIDOnStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L101) |
| `RFC8666-6-14` | When the M-Flag is set, the NP-Flag and the E-Flag MUST be ignored on reception. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC8666MappingServerFlagIgnoresNPAndE`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L168). **negative:** `unit/verify` [`TestRFC8666WithoutMappingServerFlagNPAndEApply`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L187) |
| `RFC8666-7.1-1` | Other bits: Reserved. These MUST be zero when sent and are ignored when received. (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestRFC8666AdjSIDReservedFlagBitsZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L125). **negative:** `unit/verify` [`TestRFC8666AdjSIDReservedFlagBitsIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L138) |
| `RFC8666-7.1-2` | Reserved: SHOULD be set to 0 on transmission and MUST be ignored on reception. (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestRFC8666AdjSIDReservedFieldIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L158). **negative:** `unit/verify` [`TestRFC8666AdjSIDWeightOctetNotIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L181) |
| `RFC8666-7.1-3` | When the P-Flag is set, the Adj-SID MUST be persistent (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestRFC8666AdjSIDPersistentFlagClearForNonPersistentAllocation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L423). **negative:** no negative test. **{single-polarity}:** ze's Adj-SID allocation is deliberately non-persistent -- the SRLB label is taken when the neighbor reaches Full and returned to the allocator when it leaves (srAdjManager.neighborFull, sr_adjsid.go:50-74; neighborLost, sr_adjsid.go:92-104) -- and the advertised flags are correspondingly V/L only, never P (sr_adjsid.go:63). A negative is not meaningful: no code path can advertise P, so there is no P-set advertisement whose persistence could be violated |
| `RFC8666-7.2-1` | Reserved: SHOULD be set to 0 on transmission and MUST be ignored on reception. (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestRFC8666LANAdjSIDReservedFieldIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L197). **negative:** `unit/verify` [`TestRFC8666LANAdjSIDNeighborIDNotIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L221) |
| `RFC8666-7.2-2` | When the P-Flag is set, the LAN Adjacency SID MUST be persistent (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestRFC8666AdjSIDPersistentFlagClearForNonPersistentAllocation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L427). **negative:** no negative test. **{single-polarity}:** the LAN form shares the allocation path and the flags octet with the Adj-SID (srAdjManager.neighborFull sets IsLAN on the same sr.AdjSID, sr_adjsid.go:62-69, and v6AdjSubTLV picks the LAN sub-TLV from it, sr_origination_v6.go:177-182), so it is equally non-persistent and equally never sets P. A negative is not meaningful for the same reason as RFC8666-7.1-3 |
| `RFC8666-8.1-1` | Multiple Mapping Servers can advertise Prefix-SIDs for the same prefix, in which case the same Prefix-SID MUST be advertised by all of them. (§8.1) | MUST | 8.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is not an SR Mapping Server. grep for SRMS / MappingServer over internal/plugins/ospf finds only the RFC 8665 §3.4 SRMS Preference TLV of the IPv4 RI LSA (srBuildSRMS, sr.go:187-193) and its decode (sr.go:349-358) -- a preference value, not a mapping advertisement. Every Prefix-SID ze originates from configuration is for a prefix ze itself advertises reachability to, built from the operator's NoPHP/ExplicitNull leaves alone (sr.go:203-208, sr_origination_v6.go:219). ze CAN emit an M-flagged Prefix-SID, but only by propagation, not by mapping: the ABR inter-area rule copies a RECEIVED sr.PrefixSID whole (`out := src`, sr_interarea_v6.go:36) and overrides only NP and E (:40-41), so a received M survives into EncodePrefixSIDValueV6 -> SIDFlags.toByte, which sets flagM (sr/codec_v6.go:49, sr/codec.go:92-94). That is RFC 8666 §8.2 inter-area propagation of another node's advertisement, not this node advertising a prefix-to-SID mapping of its own, so there is still no Mapping Server advertisement of ze's that could disagree with another server's |
| `RFC8666-8.1-2` | An SR Mapping Server MUST use the OSPFv3 Extended Prefix Range TLVs when advertising SIDs for prefixes (§8.1) | MUST | 8.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** same grep evidence as RFC8666-8.1-1 -- ze has no Mapping Server. ze does emit the Extended Prefix Range TLV, but from the ABR inter-area propagation path (v6EInterAreaPrefixBody, sr_interarea_v6.go:48-54), which is §8.2 carriage and not a Mapping Server advertisement |
| `RFC8666-8.1-3` | The NU-bit [RFC5340] MUST be set in the PrefixOptions field of the LSA, which is used by the Mapping Server to advertise SID or SID Range, which prevents the advertisement from contributing to prefix reachability. (§8.1) | MUST | 8.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** same evidence as RFC8666-8.1-1 -- with no Mapping Server advertisement there is no LSA that must carry the NU-bit. The NU-bit constant exists (OptPrefixNU, v3/types/prefix.go:50) and no producer sets it, which is correct here: every Prefix-SID-bearing LSA ze originates is either its own configured prefix (sr_origination_v6.go:219) or an ABR §8.2 re-advertisement of a prefix another node reaches (sr_interarea_v6.go:35-42, carried by v6EInterAreaPrefixBody :48-54), and both advertise reachability, so both must NOT set NU |
| `RFC8666-8.2-1` | In order to support SR in a multiarea environment, OSPFv3 MUST propagate Prefix-SID information between areas. (§8.2) | MUST | 8.2 | **positive:** `unit/verify` [`TestOSPFv3OriginateInterAreaPropagation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_sr_interarea_v6_test.go#L38). **negative:** `unit/verify` [`TestOSPFv3OriginateInterAreaPropagation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_sr_interarea_v6_test.go#L41) |
| `RFC8666-8.4.1-1` | If the adjacency transitions to a state lower than 2-Way, then the Adj-SID Advertisement MUST be withdrawn from the area. (§8.4.1) | MUST | 8.4.1 | **positive:** `unit/verify` [`TestRFC8666AdjSIDWithdrawnWhenAdjacencyDrops`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L468). **positive:** `unit/verify` [`TestRFC8666AdjSIDWithdrawnWhenFSMFallsBelowTwoWay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_adjsid_fsm_test.go#L78). **negative:** `unit/verify` [`TestOSPFv3ERouterBodyCarriesAdjSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_sr_origination_v6_test.go#L21) |
| `RFC8666-10-1` | For any new TLVs/sub-TLVs defined in this document, if the length is invalid, the LSA in which it is advertised is considered malformed and MUST be ignored. (§10) | MUST | 10 | **positive:** `unit/verify` [`TestRFC8666AdjSIDLengthInvalidIgnoredByReader`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_reader_ignore_test.go#L82). **positive:** `unit/verify` [`TestRFC8666LengthBeforeFlagsIgnoresLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_length_before_flags_test.go#L148). **positive:** `unit/verify` [`TestRFC8666LengthInvalidSubTLVIgnoresExtendedLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_malformed_test.go#L71). **positive:** `unit/verify` [`TestRFC8666SIDLabelLengthInvalidIgnoresLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_reader_ignore_test.go#L127). **negative:** `unit/verify` [`TestRFC8666AdjSIDLengthInvalidIgnoredByReader`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_reader_ignore_test.go#L98). **negative:** `unit/verify` [`TestRFC8666LengthBeforeFlagsIgnoresLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_length_before_flags_test.go#L163). **negative:** `unit/verify` [`TestRFC8666LengthInvalidSubTLVIgnoresExtendedLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_malformed_test.go#L81). **negative:** `unit/verify` [`TestRFC8666SIDLabelLengthInvalidIgnoresLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_reader_ignore_test.go#L137) |
| `RFC8666-11-1` | Implementations MUST ensure that malformed TLVs and sub-TLVs defined in this document are detected and that they do not provide a vulnerability for attackers to crash the OSPFv3 router or routing process. (§11) | MUST | 11 | **positive:** `unit/verify` [`TestOSPFv3ReceptionMalformedNoPanic`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_sr_reception_v6_test.go#L170). **positive:** `unit/verify` [`TestOSPFv3SRTLVMalformed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_codec_v6_test.go#L127). **negative:** `unit/verify` [`TestOSPFv3ExtPrefixRangeTLVRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_codec_v6_test.go#L96) |
| `RFC8666-5-5` | Reserved: SHOULD be set to 0 on transmission (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-6-15` | Reserved: SHOULD be set to 0 on transmission (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-6-16` | As the Mapping Server does not specify the originator of a prefix advertisement, it is not possible to determine PHP behavior solely based on the Mapping Server Advertisement. However, PHP behavior SHOULD be done in the following cases: The Prefix is intra-area type and the downstream neighbor is the originator of the prefix. The Prefix is inter-area type and the downstream neighbor is an ABR, which is advertising prefix reachability and is setting the LA-bit in the Prefix Options as described in [RFC8362]. The Prefix is external type and the downstream neighbor is an ASBR, which is advertising prefix reachability and is setting the LA-bit in the Prefix Options as described in [RFC8362]. (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-7.1-4` | Reserved: SHOULD be set to 0 on transmission (§7.1) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-7.2-3` | Reserved: SHOULD be set to 0 on transmission (§7.2) | SHOULD | 7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-8.3-1` | When an ASBR, which supports SR, originates an E-AS-External-LSA, it SHOULD also include a Prefix-SID sub-TLV as described in Section 6. (§8.3) | SHOULD | 8.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-8.3-2` | When a Not-So-Stubby Area (NSSA) [RFC3101] ABR translates an E-NSSA- LSA into an E-AS-External-LSA, it SHOULD also advertise the Prefix- SID for the prefix. (§8.3) | SHOULD | 8.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-10-2` | Errors SHOULD be logged subject to rate limiting. (§10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-11-2` | Reception of a malformed TLV or sub-TLV SHOULD be counted and/or logged for further analysis (§11) | SHOULD | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-11-3` | Logging of malformed TLVs and sub-TLVs SHOULD be rate limited to prevent a Denial-of-Service (DoS) attack (distributed or otherwise) from overloading the OSPFv3 control plane. (§11) | SHOULD | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-11-4` | While OSPFv3 is under a single administrative domain, there can be deployments where potential attackers have access to one or more networks in the OSPFv3 routing domain. In these deployments, stronger authentication mechanisms, such as those specified in [RFC4552] or [RFC7166], SHOULD be used. (§11) | SHOULD | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-5-6` | Multiple OSPFv3 Extended Prefix Range TLVs MAY be advertised in each LSA mentioned above. (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-6-17` | It MAY appear more than once in the parent TLV (§6) | MAY | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-7.1-5` | An SR-capable router MAY allocate an Adj-SID for each of its adjacencies and set the B-Flag when the adjacency is eligible for protection by an FRR mechanism (IP or MPLS) as described in [RFC8402]. (§7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-7.1-6` | When set, the G-Flag indicates that the Adj-SID refers to a group of adjacencies (and therefore MAY be assigned to other adjacencies as well). (§7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-7.1-7` | An SR-capable router MAY allocate more than one Adj-SID to an adjacency (§7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-7.1-8` | An SR-capable router MAY allocate the same Adj-SID to different adjacencies (§7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-7.1-9` | The Adj-SID sub-TLV is an optional sub-TLV of the Router-Link TLV as defined in [RFC8362]. It MAY appear multiple times in the Router- Link TLV. (§7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-7.1-10` | When the P-Flag is not set, the Adj-SID MAY be persistent (§7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-7.2-4` | The LAN Adjacency SID is an optional sub-TLV of the Router-Link TLV. It MAY appear multiple times in the Router-Link TLV. (§7.2) | MAY | 7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-7.2-5` | When the P-Flag is not set, the LAN Adjacency SID MAY be persistent (§7.2) | MAY | 7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-8.1-4` | An OSPFv3 router that supports Segment Routing MAY advertise Prefix- SIDs for any prefix to which it is advertising reachability (e.g., a loopback IP address as described in Section 6). (§8.1) | MAY | 8.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-8.4.1-2` | An Adj-SID MAY be advertised for any adjacency on a point-to-point (P2P) link that is in neighbor state 2-Way or higher. (§8.4.1) | MAY | 8.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-8.4.1-3` | If the adjacency on a P2P link transitions from the FULL state, then the Adj-SID for that adjacency MAY be removed from the area. (§8.4.1) | MAY | 8.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-8.4.2-1` | When Segment Routing is used, each router on the broadcast, NBMA, or hybrid network MAY advertise the Adj-SID for its adjacency to the DR using the Adj-SID sub-TLV as described in Section 7.1. (§8.4.2) | MAY | 8.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8666-8.4.2-2` | SR-capable routers MAY also advertise a LAN Adjacency SID for other neighbors (e.g., Backup Designated Router (BDR), DR-OTHER, etc.) on the broadcast, NBMA, or hybrid network using the LAN Adj-SID sub-TLV as described in Section 7.2. (§8.4.2) | MAY | 8.4.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8666-5-3`](#rfc8666-5-3) If the OSPFv3 Extended Prefix Range TLVs advertising the exact same range appears in multiple LSAs of the same type, originated by the same OSPFv3 router, the LSA with the numerically smallest Instance ID MUST be used (§5) | {gap}, no test | reception aggregates every Extended Prefix Range TLV found in the LSDB and judges the starting Prefix-SIDs per prefix, topology, algorithm and advertising router in each area (v6ReceivedPrefixSIDs, internal/plugins/ospf/sr_reception_v6.go; srPrefixSIDScope.resolve, internal/plugins/ospf/sr_malformed.go). Neither reader consults the Link State ID / Instance ID of the carrying LSA, so two LSAs of the same type from one originator carrying the same range give that router two Prefix-SIDs for the prefix, and both are ignored under RFC 8666 Section 6, rather than the smallest-Instance-ID one being used. Disclosed in docs/features/rfc-status.md RFC 8666 row |
| [`RFC8666-5-4`](#rfc8666-5-4) If the OSPFv3 Extended Prefix Range TLVs advertising the exact same range appears in multiple LSAs of the same type, originated by the same OSPFv3 router, the LSA with the numerically smallest Instance ID MUST be used, and subsequent instances of the OSPFv3 Extended Prefix Range TLVs MUST be ignored. (§5) | {gap}, no test | same producer as RFC8666-5-3. A subsequent instance from the same router, carrying the same SID or a different one, makes ze ignore every instance by marking the prefix Duplicate (srPrefixSIDScope.resolve, internal/plugins/ospf/sr_malformed.go) instead of keeping the smallest-Instance-ID one. Disclosed in docs/features/rfc-status.md RFC 8666 row |
| [`RFC8666-6-10`](#rfc8666-6-10) The NP-Flag (No-PHP) MUST be set and the E-Flag MUST be clear for Prefix-SIDs allocated to redistributed prefixes, unless the redistributed prefix is directly attached to the advertising ASBR. (§6) | no test | no test carries this requirement id; annotated {not-applicable}: the OSPFv3 SR origination never attaches a Prefix-SID to a redistributed prefix. v6OriginateSR builds only the E-Router-LSA and the E-Intra-Area-Prefix-LSA for configured node prefixes, plus the ABR inter-area propagation (sr_origination_v6.go:66-100); grep for extTLVExternalPrefix over internal/plugins/ospf finds it only in the RECEPTION switch (sr_reception_v6.go:106) and in the constant block (sr_origination_v6.go:33), and no producer builds an External-Prefix TLV or an E-AS-External / E-Type-7 body carrying sr.V6TypePrefixSID. With no ASBR Prefix-SID advertisement there is no flag-setting code to be right or wrong about |
| [`RFC8666-8.1-1`](#rfc8666-8.1-1) Multiple Mapping Servers can advertise Prefix-SIDs for the same prefix, in which case the same Prefix-SID MUST be advertised by all of them. (§8.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze is not an SR Mapping Server. grep for SRMS / MappingServer over internal/plugins/ospf finds only the RFC 8665 §3.4 SRMS Preference TLV of the IPv4 RI LSA (srBuildSRMS, sr.go:187-193) and its decode (sr.go:349-358) -- a preference value, not a mapping advertisement. Every Prefix-SID ze originates from configuration is for a prefix ze itself advertises reachability to, built from the operator's NoPHP/ExplicitNull leaves alone (sr.go:203-208, sr_origination_v6.go:219). ze CAN emit an M-flagged Prefix-SID, but only by propagation, not by mapping: the ABR inter-area rule copies a RECEIVED sr.PrefixSID whole (`out := src`, sr_interarea_v6.go:36) and overrides only NP and E (:40-41), so a received M survives into EncodePrefixSIDValueV6 -> SIDFlags.toByte, which sets flagM (sr/codec_v6.go:49, sr/codec.go:92-94). That is RFC 8666 §8.2 inter-area propagation of another node's advertisement, not this node advertising a prefix-to-SID mapping of its own, so there is still no Mapping Server advertisement of ze's that could disagree with another server's |
| [`RFC8666-8.1-2`](#rfc8666-8.1-2) An SR Mapping Server MUST use the OSPFv3 Extended Prefix Range TLVs when advertising SIDs for prefixes (§8.1) | no test | no test carries this requirement id; annotated {not-applicable}: same grep evidence as RFC8666-8.1-1 -- ze has no Mapping Server. ze does emit the Extended Prefix Range TLV, but from the ABR inter-area propagation path (v6EInterAreaPrefixBody, sr_interarea_v6.go:48-54), which is §8.2 carriage and not a Mapping Server advertisement |
| [`RFC8666-8.1-3`](#rfc8666-8.1-3) The NU-bit [RFC5340] MUST be set in the PrefixOptions field of the LSA, which is used by the Mapping Server to advertise SID or SID Range, which prevents the advertisement from contributing to prefix reachability. (§8.1) | no test | no test carries this requirement id; annotated {not-applicable}: same evidence as RFC8666-8.1-1 -- with no Mapping Server advertisement there is no LSA that must carry the NU-bit. The NU-bit constant exists (OptPrefixNU, v3/types/prefix.go:50) and no producer sets it, which is correct here: every Prefix-SID-bearing LSA ze originates is either its own configured prefix (sr_origination_v6.go:219) or an ABR §8.2 re-advertisement of a prefix another node reaches (sr_interarea_v6.go:35-42, carried by v6EInterAreaPrefixBody :48-54), and both advertise reachability, so both must NOT set NU |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8666-5-1`](#rfc8666-5-1)

The Range Size MUST NOT exceed the number of prefixes that could be satisfied by the Prefix Length without including: Addresses from the IPv4 multicast address range (224.0.0.0/3), if the AF is IPv4 unicast. Addresses other than the IPv6 unicast addresses, if the AF is IPv6 unicast. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity positive: v6EInterAreaPrefixBody (the only range producer) decodes to Range Size 1 for a /128; 1 never exceeds what any Prefix Length satisfies

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8666ExtPrefixRangeSizeWithinPrefixLength`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L399) | unit/verify | unproven |

### [`RFC8666-5-2`](#rfc8666-5-2)

Flags: Reserved. MUST be zero when sent and are ignored when received. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive asserts the encoded Flags octet is zero; the other tag sets Flags=0xFF and asserts header, address and Prefix-SID decode unchanged

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666ExtPrefixRangeFlagsIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L25) | unit/verify | unproven |
| positive | [`TestRFC8666ExtPrefixRangeFlagsZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L15) | unit/verify | unproven |

### [`RFC8666-5-7`](#rfc8666-5-7)

Reserved: SHOULD be set to 0 on transmission and MUST be ignored on reception. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. 0xFFFFFF in Reserved decodes identically to zero (and encoder sends zero); negative bounds the ignore by showing the adjacent AF octet is honoured

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666V6ExtPrefixRangeAFOctetNotIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_codec_v6_test.go#L221) | unit/verify | unproven |
| positive | [`TestRFC8666V6ExtPrefixRangeReservedIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_codec_v6_test.go#L189) | unit/verify | unproven |

### [`RFC8666-5-3`](#rfc8666-5-3)

If the OSPFv3 Extended Prefix Range TLVs advertising the exact same range appears in multiple LSAs of the same type, originated by the same OSPFv3 router, the LSA with the numerically smallest Instance ID MUST be used (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8666-5-3, so no unit is bound to it.

### [`RFC8666-5-4`](#rfc8666-5-4)

If the OSPFv3 Extended Prefix Range TLVs advertising the exact same range appears in multiple LSAs of the same type, originated by the same OSPFv3 router, the LSA with the numerically smallest Instance ID MUST be used, and subsequent instances of the OSPFv3 Extended Prefix Range TLVs MUST be ignored. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8666-5-4, so no unit is bound to it.

### [`RFC8666-6-1`](#rfc8666-6-1)

If set, then the penultimate hop MUST NOT pop the Prefix-SID before delivering packets to the node that advertised the Prefix- SID. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. NP set through the real OSPFv3 carriage and installer: push 16009, swap 18009->16009, no pop entry; negative shows NP clear does pop

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666PHPPopsPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L126) | unit/verify | unproven |
| positive | [`TestRFC8666NoPHPKeepsPrefixSIDOnStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L92) | unit/verify | unproven |

### [`RFC8666-6-2`](#rfc8666-6-2)

If set, any upstream neighbor of the Prefix-SID originator MUST replace the Prefix-SID with the Explicit NULL label (0 for IPv4, 2 for IPv6) before forwarding the packet. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. NP+E set imposes ExplicitNullV6 (2) at ingress and in the transit swap; E clear imposes the SRGB label. IPv4 clause (label 0) does not arise: OSPFv3 reception rejects a non-IPv6 AF

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666NoPHPKeepsPrefixSIDOnStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L99) | unit/verify | unproven |
| positive | [`TestRFC8666ExplicitNullReplacesPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L147) | unit/verify | unproven |

### [`RFC8666-6-3`](#rfc8666-6-3)

Other bits: Reserved. These MUST be zero when sent and are ignored when received. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. all defined flags set encode as 0x7C with bits 0/6/7 clear; reserved bits 0x83 set on receive decode identically

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666PrefixSIDReservedFlagBitsIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L66) | unit/verify | unproven |
| positive | [`TestRFC8666PrefixSIDReservedFlagBitsZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L52) | unit/verify | unproven |

### [`RFC8666-6-4`](#rfc8666-6-4)

Reserved: SHOULD be set to 0 on transmission and MUST be ignored on reception. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Reserved=0xFFFF decodes identically; negative bounds the ignore to Reserved by showing the Algorithm octet is honoured

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666PrefixSIDAlgorithmOctetNotIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L109) | unit/verify | unproven |
| positive | [`TestRFC8666PrefixSIDReservedFieldIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L86) | unit/verify | unproven |

### [`RFC8666-6-5`](#rfc8666-6-5)

A router receiving a Prefix-SID from a remote node and with an algorithm value that the remote node has not advertised in the SR-Algorithm TLV [RFC8665] MUST ignore the Prefix-SID sub-TLV. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. algorithm 0 not in the originator's SR-Algorithm list installs no mpls-fib entry; advertised algorithm installs push 16009

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666PrefixSIDUnadvertisedAlgorithmIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L212) | unit/verify | unproven |
| positive | [`TestRFC8666PrefixSIDAdvertisedAlgorithmInstalls`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L201) | unit/verify | unproven |

### [`RFC8666-6-6`](#rfc8666-6-6)

All other combinations of V-Flag and L-Flag are invalid and any SID Advertisement received with an invalid setting for V- and L-Flags MUST be ignored. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. V=1/L=0 and V=0/L=1 are rejected by DecodePrefixSIDValueV6, whose error drops the TLV in v6PrefixSIDFromTLV; valid V/L pairs decode

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFv3PrefixSIDCodec`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_codec_v6_test.go#L32) | unit/verify | unproven |
| positive | [`TestOSPFv3SIDWidthFromVL`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_codec_v6_test.go#L59) | unit/verify | unproven |

### [`RFC8666-6-7`](#rfc8666-6-7)

If an OSPFv3 router advertises multiple Prefix-SIDs for the same prefix, topology, and algorithm, all of them MUST be ignored. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (independent judge, c12). Section 6: 'If an OSPFv3 router advertises multiple Prefix-SIDs for the same prefix, topology, and algorithm, all of them MUST be ignored.' The consequence gap (every unit asserted rs.Duplicate at the detector, none that nothing is installed) is closed: TestRFC8666OneRouterRepeatedPrefixSIDInstallsNothing feeds real LSAs through srRemotePrefixSIDsV6 into installRoutes: - one Prefix-SID from 5.5.5.5 installs a forwarding entry (scope limit, same fixture); + the same Prefix-SID repeated in two LSAs installs none. Author overlay removing '|| rs.Duplicate' from sr_install.go reds it ('installed 1 forwarding entries, want none'); judge read the diff and the log. Records +/- revert on installRoutes observed. Detector-level units (TestRFC8666OneRouterMultiplePrefixSIDsAllIgnored, TestRFC8666PrefixSIDsKeyedByAlgorithm) keep their tags.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666OneRouterRepeatedPrefixSIDInstallsNothing`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_duplicate_install_test.go#L42) | unit/verify | revert, verified |
| negative | [`TestRFC8666PrefixSIDsKeyedByAlgorithm`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_malformed_test.go#L109) | unit/verify | revert, verified |
| negative | [`TestRFC8666OneRouterMultiplePrefixSIDsAllIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_prefix_sid_per_router_test.go#L123) | unit/verify | revert, verified |
| positive | [`TestRFC8666OneRouterRepeatedPrefixSIDInstallsNothing`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_duplicate_install_test.go#L51) | unit/verify | revert, verified |
| positive | [`TestRFC8666PrefixSIDsKeyedByAlgorithm`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_malformed_test.go#L99) | unit/verify | revert, verified |
| positive | [`TestRFC8666OneRouterMultiplePrefixSIDsAllIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_prefix_sid_per_router_test.go#L107) | unit/verify | revert, verified |

### [`RFC8666-6-8`](#rfc8666-6-8)

When calculating the outgoing label for the prefix, the router MUST take into account, as described below, the E-, NP-, and M-Flags advertised by the next-hop router if that router advertised the SID for the prefix. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (continuation 5). Positive: new TestRFC8666NextHopAdvertisedFlagsShapeOutgoingLabel, next hop = SID advertiser, four flag settings each give the Section 6 outcome: NP clear -> no push and PHP pop of 18009; NP -> push 16009; NP+E -> push IPv6 Explicit NULL; M+E with NP clear -> push 16009 (M makes NP and E ignored, distinguishable from the NP-clear pop). Negative: TestRFC8666TransitHopIgnoresOriginatorPHPFlags, a non-advertising next hop does not apply the originator's NP=0; judge overlay applying the flags unconditionally in forwarding (sr_install.go) reds it. Revert record on forwarding observed red for the positive. TestRFC8666OutgoingLabelUsesNextHopRouter stays tagged as supplementary (next hop's SRGB).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666TransitHopIgnoresOriginatorPHPFlags`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L339) | unit/verify | unproven |
| positive | [`TestRFC8666NextHopAdvertisedFlagsShapeOutgoingLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L363) | unit/verify | revert, verified |
| positive | [`TestRFC8666OutgoingLabelUsesNextHopRouter`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L320) | unit/verify | unproven |

### [`RFC8666-6-9`](#rfc8666-6-9)

The NP-Flag (No-PHP) MUST be set and the E-Flag MUST be clear for Prefix-SIDs allocated to prefixes that are propagated between areas by an ABR based on intra-area or inter-area reachability, unless the advertised prefix is directly attached to such ABR. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. v6InterAreaPrefixSIDRule sets NP and clears E for a propagated prefix and keeps flags when directly attached; TestOSPFv3OriginateInterAreaPropagation asserts NP set/E clear on the real propagated LSA

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFv3InterAreaPrefixSIDRule`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_sr_interarea_v6_test.go#L21) | unit/verify | unproven |
| positive | [`TestOSPFv3InterAreaPrefixSIDRule`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_sr_interarea_v6_test.go#L19) | unit/verify | unproven |

### [`RFC8666-6-10`](#rfc8666-6-10)

The NP-Flag (No-PHP) MUST be set and the E-Flag MUST be clear for Prefix-SIDs allocated to redistributed prefixes, unless the redistributed prefix is directly attached to the advertising ASBR. (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8666-6-10, so no unit is bound to it.

### [`RFC8666-6-11`](#rfc8666-6-11)

If the NP-Flag is not set, then: Any upstream neighbor of the Prefix-SID originator MUST pop the Prefix-SID. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. NP clear programs a PHP pop of 18009 and no push or swap; negative: NP set programs no pop

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666NoPHPKeepsPrefixSIDOnStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L97) | unit/verify | unproven |
| positive | [`TestRFC8666PHPPopsPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L123) | unit/verify | unproven |

### [`RFC8666-6-12`](#rfc8666-6-12)

If the NP-Flag is set and the E-Flag is not set, then: Any upstream neighbor of the Prefix-SID originator MUST keep the Prefix-SID on top of the stack. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. NP set, E clear keeps the SRGB label on the stack (push 16009, swap to 16009); negative: NP clear pops

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666PHPPopsPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L129) | unit/verify | unproven |
| positive | [`TestRFC8666NoPHPKeepsPrefixSIDOnStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L95) | unit/verify | unproven |

### [`RFC8666-6-13`](#rfc8666-6-13)

Any upstream neighbor of the Prefix-SID originator MUST replace the Prefix-SID with an Explicit NULL label. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. NP+E set replaces the SID with Explicit NULL 2 at ingress and in the swap; negative: NP set, E clear does not

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666NoPHPKeepsPrefixSIDOnStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L101) | unit/verify | unproven |
| positive | [`TestRFC8666ExplicitNullReplacesPrefixSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L149) | unit/verify | unproven |

### [`RFC8666-6-14`](#rfc8666-6-14)

When the M-Flag is set, the NP-Flag and the E-Flag MUST be ignored on reception. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. M=1,NP=0,E=1 keeps 16009 with no pop and no Explicit NULL (both flags ignored); same input without M pops

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666WithoutMappingServerFlagNPAndEApply`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L187) | unit/verify | unproven |
| positive | [`TestRFC8666MappingServerFlagIgnoresNPAndE`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L168) | unit/verify | unproven |

### [`RFC8666-7.1-1`](#rfc8666-7.1-1)

Other bits: Reserved. These MUST be zero when sent and are ignored when received. (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. all defined Adj-SID flags encode 0xF8 with bits 5-7 clear; 0x07 set on receive decodes identically

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666AdjSIDReservedFlagBitsIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L138) | unit/verify | unproven |
| positive | [`TestRFC8666AdjSIDReservedFlagBitsZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L125) | unit/verify | unproven |

### [`RFC8666-7.1-2`](#rfc8666-7.1-2)

Reserved: SHOULD be set to 0 on transmission and MUST be ignored on reception. (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Reserved=0xFFFF decodes identically and encoder sends zero; negative bounds the ignore by showing Weight is honoured

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666AdjSIDWeightOctetNotIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L181) | unit/verify | unproven |
| positive | [`TestRFC8666AdjSIDReservedFieldIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L158) | unit/verify | unproven |

### [`RFC8666-7.1-3`](#rfc8666-7.1-3)

When the P-Flag is set, the Adj-SID MUST be persistent (§7.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8666AdjSIDPersistentFlagClearForNonPersistentAllocation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L423) | unit/verify | unproven |

### [`RFC8666-7.2-1`](#rfc8666-7.2-1)

Reserved: SHOULD be set to 0 on transmission and MUST be ignored on reception. (§7.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. LAN Adj-SID Reserved=0xFFFF decodes identically and encoder sends zero; negative shows Neighbor ID is honoured

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666LANAdjSIDNeighborIDNotIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L221) | unit/verify | unproven |
| positive | [`TestRFC8666LANAdjSIDReservedFieldIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_test.go#L197) | unit/verify | unproven |

### [`RFC8666-7.2-2`](#rfc8666-7.2-2)

When the P-Flag is set, the LAN Adjacency SID MUST be persistent (§7.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8666AdjSIDPersistentFlagClearForNonPersistentAllocation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L427) | unit/verify | unproven |

### [`RFC8666-8.1-1`](#rfc8666-8.1-1)

Multiple Mapping Servers can advertise Prefix-SIDs for the same prefix, in which case the same Prefix-SID MUST be advertised by all of them. (§8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8666-8.1-1, so no unit is bound to it.

### [`RFC8666-8.1-2`](#rfc8666-8.1-2)

An SR Mapping Server MUST use the OSPFv3 Extended Prefix Range TLVs when advertising SIDs for prefixes (§8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8666-8.1-2, so no unit is bound to it.

### [`RFC8666-8.1-3`](#rfc8666-8.1-3)

The NU-bit [RFC5340] MUST be set in the PrefixOptions field of the LSA, which is used by the Mapping Server to advertise SID or SID Range, which prevents the advertisement from contributing to prefix reachability. (§8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8666-8.1-3, so no unit is bound to it.

### [`RFC8666-8.2-1`](#rfc8666-8.2-1)

In order to support SR in a multiarea environment, OSPFv3 MUST propagate Prefix-SID information between areas. (§8.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. intra-area Prefix-SID learned in area 1 is re-originated by v6OriginateInterAreaSR into the backbone E-Inter-Area-Prefix-LSA and not back into area 1

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFv3OriginateInterAreaPropagation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_sr_interarea_v6_test.go#L41) | unit/verify | unproven |
| positive | [`TestOSPFv3OriginateInterAreaPropagation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_sr_interarea_v6_test.go#L38) | unit/verify | unproven |

### [`RFC8666-8.4.1-1`](#rfc8666-8.4.1-1)

If the adjacency transitions to a state lower than 2-Way, then the Adj-SID Advertisement MUST be withdrawn from the area. (§8.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (continuation 6, judge): the weak finding (no FSM-driven withdrawal) is closed. TestRFC8666AdjSIDWithdrawnWhenFSMFallsBelowTwoWay drives an OSPFv3 neighbor to Full through the real table (Hello + DD), then a Hello not listing this router moves it to Init; the Adj-SID leaves the manager and the origination store, the E-Router-LSA body no longer carries it, and the mpls-fib pop entry is removed. Revert record on srAdjNeighborLost observed red. Negative: sr_origination_v6_test.go keeps the Adj-SID while the neighbor is up (pre-existing, unproven record). Setup nit: the test allocates the Adj-SID itself if Full did not, which weakens nothing about the withdrawal assertion.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFv3ERouterBodyCarriesAdjSID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_sr_origination_v6_test.go#L21) | unit/verify | unproven |
| positive | [`TestRFC8666AdjSIDWithdrawnWhenFSMFallsBelowTwoWay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_adjsid_fsm_test.go#L78) | unit/verify | revert, verified |
| positive | [`TestRFC8666AdjSIDWithdrawnWhenAdjacencyDrops`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_test.go#L468) | unit/verify | unproven |

### [`RFC8666-10-1`](#rfc8666-10-1)

For any new TLVs/sub-TLVs defined in this document, if the length is invalid, the LSA in which it is advertised is considered malformed and MUST be ignored. (§10)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30. RFC 8666 s10 read: 'if the length is invalid, the LSA in which it is advertised is considered malformed and MUST be ignored.' The three gaps of the 2026-09-27 audit are closed. Adj-SID and LAN Adj-SID at a reader: TestRFC8666AdjSIDLengthInvalidIgnoredByReader installs real E-Router-LSAs and reads the BGP-LS export (sentinel LSA proves the snapshot is current): + well-formed Adj-SID/LAN Adj-SID exported with attr 1099/1100; - each one octet long or two short exports no link from the router (records on srV3ExtendedLengthInvalid). SID/Label (s3.1 'Length: 3 or 4 octets'): D-8 defect fixed in sr_malformed.go srV3SubTLVLengthInvalid (type 7 had no case); TestRFC8666SIDLabelLengthInvalidIgnoresLSA + length 3/4 keeps the Prefix-SID, - length 2/5 yields none (red before the fix; records on srV3SubTLVLengthInvalid). Prefix-SID and Extended Prefix Range stay on rfc8666_malformed_test and sr_length_before_flags_test with records.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8666LengthInvalidSubTLVIgnoresExtendedLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_malformed_test.go#L81) | unit/verify | revert, verified |
| negative | [`TestRFC8666AdjSIDLengthInvalidIgnoredByReader`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_reader_ignore_test.go#L98) | unit/verify | revert, verified |
| negative | [`TestRFC8666SIDLabelLengthInvalidIgnoresLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_reader_ignore_test.go#L137) | unit/verify | revert, verified |
| negative | [`TestRFC8666LengthBeforeFlagsIgnoresLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_length_before_flags_test.go#L163) | unit/verify | revert, verified |
| positive | [`TestRFC8666LengthInvalidSubTLVIgnoresExtendedLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_malformed_test.go#L71) | unit/verify | revert, verified |
| positive | [`TestRFC8666AdjSIDLengthInvalidIgnoredByReader`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_reader_ignore_test.go#L82) | unit/verify | revert, verified |
| positive | [`TestRFC8666SIDLabelLengthInvalidIgnoresLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_reader_ignore_test.go#L127) | unit/verify | revert, verified |
| positive | [`TestRFC8666LengthBeforeFlagsIgnoresLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr_length_before_flags_test.go#L148) | unit/verify | revert, verified |

### [`RFC8666-11-1`](#rfc8666-11-1)

Implementations MUST ensure that malformed TLVs and sub-TLVs defined in this document are detected and that they do not provide a vulnerability for attackers to crash the OSPFv3 router or routing process. (§11)

Audit verdict: enforced (the tests do what the requirement demands), fresh. malformed values are rejected with an error (not only no panic) by every v3 SR decoder and by v6PrefixSIDFromPrefixTLV; well-formed range TLVs still decode

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFv3ExtPrefixRangeTLVRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_codec_v6_test.go#L96) | unit/verify | unproven |
| positive | [`TestOSPFv3ReceptionMalformedNoPanic`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8666_sr_reception_v6_test.go#L170) | unit/verify | unproven |
| positive | [`TestOSPFv3SRTLVMalformed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/sr/rfc8666_codec_v6_test.go#L127) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc8666.txt |
| Source fingerprint | 4b07a65a8931185d |
| Record | rfc/extraction/rfc8666.json |
| Mapped sentences | 30 |
| Declined as scope | 1 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 4 | walked | not stated |
| `6` | not stated | 15 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 3 | walked | not stated |
| `7.2` | not stated | 2 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 3 | walked | not stated |
| `8.2` | not stated | 1 | walked | not stated |
| `8.3` | not stated | 0 | walked | not stated |
| `8.4` | not stated | 0 | walked | not stated |
| `8.4.1` | not stated | 1 | walked | not stated |
| `8.4.2` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 0 | walked | not stated |
| `9.2` | not stated | 0 | walked | not stated |
| `10` | not stated | 1 | walked | not stated |
| `11` | not stated | 1 | walked | not stated |
| `12` | not stated | 0 | walked | not stated |
| `12.1` | not stated | 0 | walked | not stated |
| `12.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `6:9` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Second sentence of the same paragraph; RFC8666-6-8 already carries 'regardless of whether the next-hop router contributes to the best path' as part of the outgoing-label rule. | This MUST be done regardless of whether the next-hop router contributes to the best path to the prefix. |

## Superseded

No document obsoletes RFC 8666, so its obligations are stated where they were written.
