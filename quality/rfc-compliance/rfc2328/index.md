# RFC 2328 - OSPF Version 2

Partial. Every requirement this repository extracted from RFC 2328, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 96.2% | 50 of 52 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 1.9% | 1 of 52 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 52 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 41.4% | 48 of 116 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 52 | of 65 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 52 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 52 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 52 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 52 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 1.9% | 1 of 52 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 51 | of 52 gated MUSTs judged | 32 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 52 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 65 |
| Gated MUST-level | 52 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 118 |
| Tagged units | 116 |
| Recorded audit verdicts | 51 |
| Discrimination records | 48 |
| Summary | `rfc/short/rfc2328.md` |
| Requirement shard | `rfc/requirements/rfc2328.md` |
| RFC text | `rfc/full/rfc2328.txt` |

## Enrolment

Enrolled: OSPF Version 2

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

Native OSPFv2 engine, raw protocol 89: the 24-byte common header with Version-2 validation and the auth-excluding packet checksum, the Fletcher LS checksum, the Section 13 flooding procedure (checksum/unknown-type discard, stub-area Type-5 filter, Exchange-or-higher gate, Section 13.1 freshness ordering, MaxAge+MaxSequenceNumber silent discard, retransmission lists at RxmtInterval, Table 19 acknowledgment decisions, self-originated re-origination and premature-aging flush), Section 14 aging and purge retention, the Section 16 routing calculation (two-way check, ABR backbone-only summaries, LSInfinity/MaxAge/self skips, intra-over-inter-over-external path preference), Database Exchange with a single outstanding DD and the BadLSReq restart, virtual links with Interface MTU 0 in their DDs, positive interface output cost, and Appendix D authentication types 0/1/2 including the non-decreasing cryptographic sequence number. Requirements bound per line in [`rfc/short/rfc2328.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc2328.md).

**What the ledger says remains**

The [`RFC2328-13.3-2`](#rfc2328-13.3-2) normal-flood InfTransDelay gap remains recorded below. The 2026-09-21 extraction added obligations that require current positive/negative coverage and discrimination. New IP-envelope, source-address, classless-route and virtual-link carriers have been added; current runner proof and deployment evidence remain pending.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 50 | one part of the gated population |
| Annotated instead of tested | 2 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **52** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (50):** [`RFC2328-A.3.1-1`](#rfc2328-a.3.1-1), [`RFC2328-A.3.1-2`](#rfc2328-a.3.1-2), [`RFC2328-12.1.7-1`](#rfc2328-12.1.7-1), [`RFC2328-13-1`](#rfc2328-13-1), [`RFC2328-13-2`](#rfc2328-13-2), [`RFC2328-13-3`](#rfc2328-13-3), [`RFC2328-13.1-1`](#rfc2328-13.1-1), [`RFC2328-13-4`](#rfc2328-13-4), [`RFC2328-13.3-1`](#rfc2328-13.3-1), [`RFC2328-14-1`](#rfc2328-14-1), [`RFC2328-14-2`](#rfc2328-14-2), [`RFC2328-13.5-1`](#rfc2328-13.5-1), [`RFC2328-13.4-1`](#rfc2328-13.4-1), [`RFC2328-16.1-1`](#rfc2328-16.1-1), [`RFC2328-16.4-1`](#rfc2328-16.4-1), [`RFC2328-16.2-1`](#rfc2328-16.2-1), [`RFC2328-16.2-2`](#rfc2328-16.2-2), [`RFC2328-D.2-1`](#rfc2328-d.2-1), [`RFC2328-D.3-1`](#rfc2328-d.3-1), [`RFC2328-D.3-2`](#rfc2328-d.3-2), [`RFC2328-A.3.3-1`](#rfc2328-a.3.3-1), [`RFC2328-10.1-1`](#rfc2328-10.1-1), [`RFC2328-10.2-1`](#rfc2328-10.2-1), [`RFC2328-C.3-1`](#rfc2328-c.3-1), [`RFC2328-3.6-1`](#rfc2328-3.6-1), [`RFC2328-4.4-1`](#rfc2328-4.4-1), [`RFC2328-4.4-2`](#rfc2328-4.4-2), [`RFC2328-4.4-3`](#rfc2328-4.4-3), [`RFC2328-8.1-1`](#rfc2328-8.1-1), [`RFC2328-8.2-1`](#rfc2328-8.2-1), [`RFC2328-8.2-2`](#rfc2328-8.2-2), [`RFC2328-8.2-3`](#rfc2328-8.2-3), [`RFC2328-9.1-1`](#rfc2328-9.1-1), [`RFC2328-9.5.1-1`](#rfc2328-9.5.1-1), [`RFC2328-10.5-1`](#rfc2328-10.5-1), [`RFC2328-10.6-1`](#rfc2328-10.6-1), [`RFC2328-12.1.6-1`](#rfc2328-12.1.6-1), [`RFC2328-12.2-1`](#rfc2328-12.2-1), [`RFC2328-12.4-1`](#rfc2328-12.4-1), [`RFC2328-12.4.1-1`](#rfc2328-12.4.1-1), [`RFC2328-12.4.3-1`](#rfc2328-12.4.3-1), [`RFC2328-13-5`](#rfc2328-13-5), [`RFC2328-13-6`](#rfc2328-13-6), [`RFC2328-13-7`](#rfc2328-13-7), [`RFC2328-13.3-3`](#rfc2328-13.3-3), [`RFC2328-15-2`](#rfc2328-15-2), [`RFC2328-16.1-3`](#rfc2328-16.1-3), [`RFC2328-A.1-1`](#rfc2328-a.1-1), [`RFC2328-A.1-2`](#rfc2328-a.1-2), [`RFC2328-A.4.4-1`](#rfc2328-a.4.4-1)

**Annotated instead of tested (2):** [`RFC2328-13.3-2`](#rfc2328-13.3-2), [`RFC2328-D.4.3-1`](#rfc2328-d.4.3-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2328-A.3.1-1` | The version number field must specify protocol version 2. (§8.2) | MUST | 8.2 | **positive:** `unit/verify` [`TestOSPFHeaderRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/header_test.go#L131). **negative:** `unit/verify` [`TestOSPFHeaderRejectsBadVersionAndLength`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/header_test.go#L153) |
| `RFC2328-A.3.1-2` | The standard IP checksum of the entire contents of the packet, starting with the OSPF packet header but excluding the 64-bit authentication field. (§A.3.1) | MUST | A.3.1 - Appendix subsection A.3.1 | **positive:** `unit/verify` [`TestOSPFPacketChecksumExcludesAuth`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L30). **negative:** `unit/verify` [`TestPacketVerifyChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/header_test.go#L100) |
| `RFC2328-12.1.7-1` | This field is the checksum of the complete contents of the LSA, excepting the LS age field. The LS age field is excepted so that an LSA's age can be incremented without updating the checksum. The checksum used is the same that is used for ISO connectionless datagrams; it is commonly referred to as the Fletcher checksum. It is documented in Annex B of [Ref6]. The LSA header also contains the length of the LSA in bytes; subtracting the size of the LS age field (two bytes) yields the amount of data to checksum. The checksum is used to detect data corruption of an LSA. This corruption can occur while an LSA is being flooded, or while it is being held in a router's memory. The LS checksum field cannot take on the value of zero; the occurrence of such a value should be considered a checksum failure. In other words, calculation of the checksum is not optional. (§12.1.7) | MUST | 12.1.7 | **positive:** `unit/verify` [`TestFletcherIgnoresLSAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L46). **positive:** `unit/verify` [`TestFletcherRFC905Vectors`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L26). **positive:** `unit/verify` [`TestOSPFLSAChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L72). **positive:** `unit/verify` [`TestOSPFLSAChecksumExcludesAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L98). **negative:** `unit/verify` [`TestOSPFLSAChecksumExcludesAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L96). **negative:** `unit/verify` [`TestRFC2328ZeroLSChecksumRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc2328_test.go#L10) |
| `RFC2328-13-1` | Validate the LSA's LS checksum. If the checksum turns out to be invalid, discard the LSA and get the next one from the Link State Update packet. (2) Examine the LSA's LS type. If the LS type is unknown, discard the LSA and get the next one from the Link State Update Packet. (§13) | MUST | 13 | **positive:** `unit/verify` [`TestOSPFFloodOutOtherInterfaces`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L30). **negative:** `unit/verify` [`TestDecodeLSReqRejectsMalformed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/packet_body_test.go#L145). **negative:** `unit/verify` [`TestRFC2328BadLSChecksumDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_test.go#L17) |
| `RFC2328-13-2` | AS-external-LSAs are not flooded into/throughout stub areas (§13) | MUST NOT | 13 | **positive:** `unit/verify` [`TestOSPFStubFloodFilter`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/area_type_test.go#L15). **negative:** `unit/verify` [`TestOSPFStubAreaDropsType5`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L247) |
| `RFC2328-13-3` | If the neighbor is in a lesser state than Exchange, the packet should be dropped without further processing. (§13) | MUST | 13 | **positive:** `unit/verify` [`TestRFC2328FloodingRequiresExchangeOrHigher`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L17). **negative:** `unit/verify` [`TestRFC2328FloodingRequiresExchangeOrHigher`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L20) |
| `RFC2328-13.1-1` | The LSA having the newer LS sequence number is more recent. See Section 12.1.6 for an explanation of the LS sequence number space. If both instances have the same LS sequence number, then: o If the two instances have different LS checksums, then the instance having the larger LS checksum (when considered as a 16-bit unsigned integer) is considered more recent. o Else, if only one of the instances has its LS age field set to MaxAge, the instance of age MaxAge is considered to be more recent. o Else, if the LS age fields of the two instances differ by more than MaxAgeDiff, the instance having the smaller (younger) LS age is considered to be more recent. (§13.1) | MUST | 13.1 | **positive:** `unit/verify` [`TestOSPFFreshnessCompareMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/lsdb_test.go#L126). **negative:** `unit/verify` [`TestRFC2328OlderInstanceGetsDatabaseCopyBack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_test.go#L50) |
| `RFC2328-13-4` | If the database copy has LS age equal to MaxAge and LS sequence number equal to MaxSequenceNumber, simply discard the received LSA without acknowledging it. (§13) | MUST | 13 | **positive:** `unit/verify` [`TestOSPFMaxSeqMaxAgeSilentDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_edges_test.go#L19). **negative:** `unit/verify` [`TestRFC2328OlderInstanceGetsDatabaseCopyBack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_test.go#L53) |
| `RFC2328-13.3-1` | LSAs flooded out an adjacency are placed on the adjacency's Link state retransmission list. In order to ensure that flooding is reliable, these LSAs are retransmitted until they are acknowledged. The length of time between retransmissions is a configurable per-interface value, RxmtInterval. (§13.6) | MUST | 13.6 | **positive:** `unit/verify` [`TestOSPFFloodOutOtherInterfaces`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L31). **positive:** `unit/verify` [`TestOSPFRetransmitTimer`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L129). **negative:** `unit/verify` [`TestOSPFFloodQueuesExchangeAndLoadingNeighbors`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L57) |
| `RFC2328-13.3-2` | The LSA's LS age must be incremented by InfTransDelay (which must be > 0) when it is copied into the outgoing Link State Update packet (until the LS age field reaches the maximum value of MaxAge). (§13.3) | MUST | 13.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the InfTransDelay bump is applied on the retransmit path (RetransmitTick, lsdb/flooding.go:545) and on a direct database-copy reply (sendDirectLSUpdate, lsdb/flooding.go:745; sendDirectLinkLSUpdate, lsdb/link_scope.go:349), but NOT on the normal flood: floodExcept builds the outgoing copy with `entry.LSA(d.now())` (lsdb/flooding.go:351), and Entry.LSA calls `e.Raw(now, 0)` with transmitDelay 0 (lsdb/entry.go:62-68, 75-85), so the first flooded copy carries the unincremented LS age. The MaxAge cap itself is present wherever the bump is applied (LSAge.Add, types/lsage.go:54-63). Disclosed in docs/features/rfc-status.md RFC 2328 row |
| `RFC2328-14-1` | An LSA's LS age is never incremented past the value MaxAge. LSAs having age MaxAge are not used in the routing table calculation. (§14) | MUST | 14 | **positive:** `unit/verify` [`TestLSAgeAddSaturates`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/lsage_test.go#L39). **positive:** `unit/verify` [`TestOSPFLSDBAgeToPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/aging_test.go#L34). **negative:** `unit/verify` [`TestOSPFGraphSkipsMaxAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/graph_test.go#L40) |
| `RFC2328-14-2` | A MaxAge LSA must be removed immediately from the router's link state database as soon as both a) it is no longer contained on any neighbor Link state retransmission lists and b) none of the router's neighbors are in states Exchange or Loading. (§14) | MUST | 14 | **positive:** `unit/verify` [`TestOSPFASExternalPurgeRetainedAcrossAreas`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L332). **negative:** `unit/verify` [`TestOSPFPurgeRetainedForExchangeOrLoading`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L383) |
| `RFC2328-13.5-1` | Each newly received LSA must be acknowledged. This is usually done by sending Link State Acknowledgment packets. However, acknowledgments can also be accomplished implicitly by sending Link State Update packets (see step 7a of Section 13). (§13.5) | MUST | 13.5 | **positive:** `unit/verify` [`TestOSPFAckDecisionTable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L93). **positive:** `unit/verify` [`TestOSPFUnknownMaxAgeNoCopyIsAckedAndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L360). **negative:** `unit/verify` [`TestOSPFDRRefloodsBackOutReceivingInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_edges_test.go#L80) |
| `RFC2328-13.4-1` | A self-originated LSA is detected when either 1) the LSA's Advertising Router is equal to the router's own Router ID or 2) the LSA is a network- LSA and its Link State ID is equal to one of the router's own IP interface addresses. (§13.4) | MUST | 13.4 | **positive:** `unit/verify` [`TestOSPFOriginateSelfReceivedHigherSeq`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/origination_test.go#L426). **negative:** `unit/verify` [`TestOSPFSelfOriginatedNoLocalCopyFlush`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_edges_test.go#L43) |
| `RFC2328-16.1-1` | Look up the vertex W's LSA (router-LSA or network-LSA) in Area A's link state database. If the LSA does not exist, or its LS age is equal to MaxAge, or it does not have a link back to vertex V, examine the next link in V's LSA. (§16.1) | MUST | 16.1 | **positive:** `unit/verify` [`TestOSPFSPFShortestPath`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/spf_test.go#L13). **negative:** `unit/verify` [`TestOSPFTwoWayCheck`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/spf_test.go#L31) |
| `RFC2328-16.4-1` | Intra-area and inter-area paths are always preferred over AS external paths. (b) Type 1 external paths are always preferred over type 2 external paths. When all paths are type 2 external paths, the paths with the smallest advertised type 2 metric are always preferred. (§16.4) | MUST | 16.4 | **positive:** `unit/verify` [`TestOSPFRouteTablePreference`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/route_test.go#L8). **negative:** `unit/verify` [`TestOSPFExternalE1PreferredOverE2`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external_test.go#L83) |
| `RFC2328-16.2-1` | If the router is attached to multiple areas (i.e., it is an area border router), only backbone summary-LSAs are examined. (§16) | MUST | 16 | **positive:** `unit/verify` [`TestOSPFInterAreaRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/interarea_test.go#L43). **negative:** `unit/verify` [`TestOSPFABRBackboneOnlyAcceptance`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/interarea_test.go#L74) |
| `RFC2328-16.2-2` | If the cost specified by the LSA is LSInfinity, or if the LSA's LS age is equal to MaxAge, then examine the the next LSA. (2) If the LSA was originated by the calculating router itself, examine the next LSA. (§16.2) | MUST | 16.2 | **positive:** `unit/verify` [`TestOSPFInterAreaRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/interarea_test.go#L44). **negative:** `unit/verify` [`TestOSPFExternalLSInfinityDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external_test.go#L61). **negative:** `unit/verify` [`TestOSPFInterAreaLSInfinityDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/interarea_test.go#L137). **negative:** `unit/verify` [`TestRFC2328ExternalSkipsMaxAgeAndSelf`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_test.go#L43). **negative:** `unit/verify` [`TestRFC2328InterAreaSkipsMaxAgeAndSelfSummary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_test.go#L20) |
| `RFC2328-D.2-1` | The 64-bit authentication field in the OSPF packet header must be equal to the 64-bit password (i.e., authentication key) that has been configured for the interface. (§D.5.2) | MUST | D.5.2 | **positive:** `unit/verify` [`TestRFC2328SimplePasswordMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_test.go#L95). **negative:** `unit/verify` [`TestRFC2328SimplePasswordMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_test.go#L99) |
| `RFC2328-D.3-1` | The message digest appended to the OSPF packet is not actually considered part of the OSPF protocol packet: the message digest is not included in the OSPF header's packet length, although it is included in the packet's IP header length field. (§D.3) | MUST | D.3 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L47). **negative:** `unit/verify` [`TestOSPFAuthCryptoRejectsExtraTrailerBytes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L219) |
| `RFC2328-D.3-2` | Whenever an OSPF packet is accepted as authentic, the cryptographic sequence number is set to the received packet's sequence number. (§D.3) | MUST | D.3 | **positive:** `unit/verify` [`TestNeighborDownResetsCryptoSeq`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L246). **positive:** `unit/verify` [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L116). **positive:** `unit/verify` [`TestOSPFAuthReplayEqualSequenceByAuType`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L173). **negative:** `unit/verify` [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L117) |
| `RFC2328-D.4.3-1` | The checksum field in the standard OSPF header is not calculated, but is instead set to 0. (§D.4.3) | MUST | D.4.3 | **positive:** `unit/verify` [`TestOSPFPacketChecksumZeroForAuType2`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L53). **negative:** no negative test. **{single-polarity}:** generate-side convention -- WriteTo leaves the AuType2 Checksum field zero (ospf/packet/header.go:317-321) and VerifyPacketChecksum accepts the zero checksum (ospf/packet/checksum.go:28-30); only the field-is-zero positive is asserted |
| `RFC2328-A.3.3-1` | Interface MTU should be set to 0 in Database Description packets sent over virtual links. (§A.3.3) | MUST | A.3.3 - Appendix subsection A.3.3 | **positive:** `unit/verify` [`TestRFC2328VirtualInterfaceHasNoMTU`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_test.go#L32). **positive:** `unit/verify` [`TestRFC2328VirtualLinkDBDescCarriesZeroMTU`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L58). **negative:** `unit/verify` [`TestRFC2328VirtualLinkDBDescCarriesZeroMTU`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L63) |
| `RFC2328-10.1-1` | Only one Database Description Packet is allowed outstanding at any one time. (§10.1) | MUST | 10.1 | **positive:** `unit/verify` [`TestOSPFDDRetransmit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/nsm_test.go#L527). **negative:** `unit/verify` [`TestOSPFDuplicateDD`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/nsm_test.go#L595) |
| `RFC2328-10.2-1` | If an LSA cannot be found in the database, something has gone wrong with the Database Exchange process, and neighbor event BadLSReq should be generated. (§10.7) | MUST | 10.7 | **positive:** `unit/verify` [`TestOSPFBadLSReqRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/nsm_test.go#L734). **negative:** `unit/verify` [`TestOSPFValidLSReqSendsLSUpdate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/nsm_test.go#L905). **negative:** `unit/verify` [`TestRFC2328KnownLSRequestDoesNotRestartExchange`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L121) |
| `RFC2328-C.3-1` | The interface output cost must always be greater than 0. (§C.3) | MUST | C.3 | **positive:** `unit/verify` [`TestInterfaceCostAndTransmitDelayBoundary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_interface_validate_test.go#L16). **negative:** `unit/verify` [`TestInterfaceCostAndTransmitDelayBoundary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_interface_validate_test.go#L17) |
| `RFC2328-3.6-1` | One or more of the stub area's area border routers must advertise a default route into the stub area via summary-LSAs. (§3.6) | MUST | 3.6 | **positive:** `unit/verify` [`TestRFC2328StubAreaGetsDefaultSummary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_abr_summary_test.go#L34). **negative:** `unit/verify` [`TestRFC2328NormalAreaGetsNoDefaultSummary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_abr_summary_test.go#L53) |
| `RFC2328-4.4-1` | Support for receiving and sending IP multicast datagrams, along with the appropriate lower-level protocol support, is required. (§4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestOSPFMulticastMembershipInstalled`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_socket_linux_test.go#L76). **negative:** `unit/verify` [`TestOSPFMulticastMembershipRefusesForeignGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_socket_linux_test.go#L97) |
| `RFC2328-4.4-2` | The router's IP protocol support must include the ability to divide a single IP class A, B, or C network number into many subnets of various sizes. This is commonly called variable-length subnetting; see Section 3.5 for details. IP supernetting support The router's IP protocol support must include the ability to aggregate contiguous collections of IP class A, B, and C networks into larger quantities called supernets. (§4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestRFC2328ClasslessPrefixesReachIPRIB`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_classless_test.go#L34). **negative:** `unit/verify` [`TestRFC2328ClasslessWithdrawalPreservesOverlaps`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_classless_test.go#L51) |
| `RFC2328-4.4-3` | Indications must be passed from these protocols to OSPF as the network interface goes up and down. (§4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestOSPFTransportPassesLinkIndicationsToOSPF`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_link_test.go#L8). **negative:** `unit/verify` [`TestOSPFTransportPassesNoIndicationForNonOSPFInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_link_test.go#L45) |
| `RFC2328-8.1-1` | On these interfaces, the IP source should be set to any of the other IP addresses belonging to the router. For this reason, there must be at least one IP address assigned to the router. (§8.1) | MUST | 8.1 | **positive:** `unit/verify` [`TestResolveOSPFInterfaceUsesIfaceResolverOSName`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/backend_linux_test.go#L17). **negative:** `unit/verify` [`TestOSPFRefusesInterfaceWithoutIPv4Source`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/backend_linux_test.go#L101) |
| `RFC2328-8.2-1` | In order for the packet to be accepted at the IP level, it must pass a number of tests, even before the packet is passed to OSPF for processing: o The IP checksum must be correct. o The packet's IP destination address must be the IP address of the receiving interface, or one of the IP multicast addresses AllSPFRouters or AllDRouters. o The IP protocol specified must be OSPF (89). (§8.2) | MUST | 8.2 | **positive:** `unit/verify` [`TestOSPFReceiveAcceptsValidIPv4Envelope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/backend_linux_test.go#L130). **negative:** `unit/verify` [`TestOSPFReceiveRejectsInvalidIPv4Envelope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/backend_linux_test.go#L151) |
| `RFC2328-8.2-2` | The Area ID found in the OSPF header must be verified. If both of the following cases fail, the packet should be discarded. The Area ID specified in the header must either: (1) Match the Area ID of the receiving interface. In this case, the packet has been sent over a single hop. Therefore, the packet's IP source address is required to be on the same network as the receiving interface. This can be verified by comparing the packet's IP source address to the interface's IP address, after masking both addresses with the interface mask. This comparison should not be performed on point-to-point networks. On point-to-point networks, the interface addresses of each end of the link are assigned independently, if they are assigned at all. (2) Indicate the backbone. In this case, the packet has been sent over a virtual link. The receiving router must be an area border router, and the Router ID specified in the packet (the source router) must be the other end of a configured virtual link. The receiving interface must also attach to the virtual link's configured Transit area. (§8.2) | MUST | 8.2 | **positive:** `unit/verify` [`TestOSPFReceiveAcceptsMatchingAreaID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_receive_test.go#L49). **negative:** `unit/verify` [`TestOSPFReceiveDropsMismatchedAreaID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_receive_test.go#L65) |
| `RFC2328-8.2-3` | The AuType specified in the packet must match the AuType specified for the associated area. (§8.2) | MUST | 8.2 | **positive:** `unit/verify` [`TestOSPFReceiveAcceptsMatchingAuType`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_receive_test.go#L82). **negative:** `unit/verify` [`TestOSPFReceiveDropsMismatchedAuType`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_receive_test.go#L104) |
| `RFC2328-9.1-1` | The router must also originate a network-LSA for the network node. (§9.1, §12.4.2) | MUST | 9.1 | **positive:** `unit/verify` [`TestRFC2328DROriginatesNetworkLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L384). **negative:** `unit/verify` [`TestRFC2328NonDROriginatesNoNetworkLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L404) |
| `RFC2328-9.5.1-1` | If the router is eligible to become Designated Router, it must periodically send Hello Packets to all neighbors that are also eligible. In addition, if the router is itself the Designated Router or Backup Designated Router, it must also send periodic Hello Packets to all other neighbors. This means that any two eligible routers are always exchanging Hello Packets, which is necessary for the correct operation of the Designated Router election algorithm. To minimize the number of Hello Packets sent, the number of eligible routers on an NBMA network should be kept small. If the router is not eligible to become Designated Router, it must periodically send Hello Packets to both the Designated Router and the Backup Designated Router (if they exist). (§9.5.1) | MUST | 9.5.1 | **positive:** `unit/verify` [`TestOSPFNBMAEligibleRouterHellosEveryEligibleNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc2328_dr_test.go#L75). **negative:** `unit/verify` [`TestOSPFNBMAPeriodicHelloSkipsIneligibleNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc2328_dr_test.go#L92) |
| `RFC2328-10.5-1` | Next, the values of the Network Mask, HelloInterval, and RouterDeadInterval fields in the received Hello packet must be checked against the values configured for the receiving interface. Any mismatch causes processing to stop and the packet to be dropped. (§10.5) | MUST | 10.5 | **positive:** `unit/verify` [`TestRFC2328HelloMatchingParametersAndTwoWay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc2328_hello_test.go#L16). **negative:** `unit/verify` [`TestRFC2328HelloMismatchRejectedAndNoTwoWayUnlisted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc2328_hello_test.go#L35) |
| `RFC2328-10.6-1` | In states Loading and Full the slave must resend its last Database Description packet in response to duplicate Database Description packets received from the master. For this reason the slave must wait RouterDeadInterval seconds before freeing the last Database Description packet. (§10.8) | MUST | 10.8 | **positive:** `unit/verify` [`TestRFC2328SlaveRepliesAndRepeatsOnDuplicate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_dd_slave_test.go#L30). **negative:** `unit/verify` [`TestRFC2328SlaveRefusesOutOfSequenceDD`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_dd_slave_test.go#L62) |
| `RFC2328-12.1.6-1` | When an attempt is made to increment the sequence number past the maximum value of N - 1 (0x7fffffff; also referred to as MaxSequenceNumber), the current instance of the LSA must first be flushed from the routing domain. This is done by prematurely aging the LSA (see Section 14.1) and reflooding it. As soon as this flood has been acknowledged by all adjacent neighbors, a new instance can be originated with sequence number of InitialSequenceNumber. (§12.1.6) | MUST | 12.1.6 | **positive:** `unit/verify` [`TestRFC2328SequenceWrapRestartsAfterAck`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L217). **negative:** `unit/verify` [`TestRFC2328SequenceWrapWaitsForAck`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L194) |
| `RFC2328-12.2-1` | An implementation of OSPF must be able to access individual pieces of an area database. This lookup function is based on an LSA's LS type, Link State ID and Advertising Router. (§12.2) | MUST | 12.2 | **positive:** `unit/verify` [`TestRFC2328LookupByTriple`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L160). **negative:** `unit/verify` [`TestRFC2328LookupNeedsFullTriple`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L174) |
| `RFC2328-12.4-1` | If a router advertises a summary-LSA for a destination which then becomes unreachable, the router must then flush the LSA from the routing domain by setting its age to MaxAge and reflooding (see Section 14.1). (§12.4.3) | MUST | 12.4.3 | **positive:** `unit/verify` [`TestRFC2328WithdrawnSummaryFlushedAtMaxAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L249). **negative:** `unit/verify` [`TestRFC2328AdvertisedSummaryNotFlushed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L286) |
| `RFC2328-12.4.1-1` | All of the router's links to the area must be described in a single router-LSA. (§A.4.2) | MUST | A.4.2 - Appendix subsection A.4.2 | **positive:** `unit/verify` [`TestRFC2328SingleRouterLSACarriesEveryLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L318). **negative:** `unit/verify` [`TestRFC2328RouterLSAExcludesOtherAreaLinks`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L350) |
| `RFC2328-12.4.3-1` | The router must then originate summary-LSAs into the newly attached area for all pertinent intra-area and inter-area routes in the router's routing table. (§12.4) | MUST | 12.4 | **positive:** `unit/verify` [`TestRFC2328ABRSummarizesAndCondenses`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_abr_summary_test.go#L67). **negative:** `unit/verify` [`TestRFC2328ABRComponentNeverEscapesRange`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_abr_summary_test.go#L101) |
| `RFC2328-13-5` | Remove the current database copy from all neighbors' Link state retransmission lists. (§13) | MUST | 13 | **positive:** `unit/verify` [`TestRFC2328ReplacedInstanceLeavesRetransmitLists`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L110). **negative:** `unit/verify` [`TestRFC2328DuplicateKeepsOtherRetransmitLists`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L144) |
| `RFC2328-13-6` | If the contents are different, the following pieces of the routing table must be recalculated, depending on the new LSA's LS type field: Router-LSAs and network-LSAs The entire routing table must be recalculated, starting with the shortest path calculations for each area (not just the area whose link-state database has changed). (§13.2) | MUST | 13.2 | **positive:** `unit/verify` [`TestSPFRecalculatesEveryAreaOnOneAreaChange`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_recalc_test.go#L14). **negative:** `unit/verify` [`TestSPFRecalculationBoundedToConfiguredAreas`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_recalc_test.go#L47) |
| `RFC2328-13-7` | If there is already a database copy, and if the database copy was received via flooding and installed less than MinLSArrival seconds ago, discard the new LSA (without acknowledging it) and examine the next LSA (if any) listed in the Link State Update packet. (§13) | MUST | 13 | **positive:** `unit/verify` [`TestRFC2328MinLSArrivalElapsedAccepts`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L79). **negative:** `unit/verify` [`TestRFC2328MinLSArrivalDiscardsWithoutAck`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L44) |
| `RFC2328-13.3-3` | On non-broadcast networks, separate Link State Update packets must be sent, as unicasts, to each adjacent neighbor (i.e., those in state Exchange or greater). (§13.3) | MUST | 13.3 | **positive:** `unit/verify` [`TestRFC2328NBMAUnicastsToEachAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L430). **negative:** `unit/verify` [`TestRFC2328NBMANeverMulticastsNorReachesTwoWay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L464) |
| `RFC2328-15-2` | The cost of a virtual link is NOT configured. It is defined to be the cost of the intra-area path between the two defining area border routers. This cost appears in the virtual link's corresponding routing table entry. When the cost of a virtual link changes, a new router-LSA should be originated for the backbone area. (§15) | MUST | 15 | **positive:** `unit/verify` [`TestRFC2328VirtualCostChangeReoriginatesBackbone`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_virtual_route_test.go#L67). **negative:** `unit/verify` [`TestRFC2328VirtualUnusablePathWithdrawsBackboneLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_virtual_route_test.go#L98) |
| `RFC2328-16.1-3` | Note that when there is a choice of vertices closest to the root, network vertices must be chosen before router vertices in order to necessarily find all equal-cost paths. (§16.1) | MUST | 16.1 | **positive:** `unit/verify` [`TestRFC2328EqualDistanceNetworkVertexFirst`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_tiebreak_test.go#L10). **negative:** `unit/verify` [`TestRFC2328CloserRouterVertexBeforeFartherNetwork`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_tiebreak_test.go#L25) |
| `RFC2328-A.1-1` | Packets sent to these multicast addresses should never be forwarded; they are meant to travel a single hop only. To ensure that these packets will not travel multiple hops, their IP TTL must be set to 1. (§A.1) | MUST | A.1 - Appendix subsection A.1 | **positive:** `unit/verify` [`TestOSPFSocketTTLIsOne`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_socket_linux_test.go#L30). **negative:** `unit/verify` [`TestOSPFSocketTTLFailureIsAnError`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_socket_linux_test.go#L58) |
| `RFC2328-A.1-2` | Both the Designated Router and Backup Designated Router must be prepared to receive packets destined to this address. (§A.1) | MUST | A.1 - Appendix subsection A.1 | **positive:** `unit/verify` [`TestOSPFElectedBDRJoinsAllDRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc2328_dr_test.go#L29). **negative:** `unit/verify` [`TestOSPFDROtherDoesNotJoinAllDRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc2328_dr_test.go#L46) |
| `RFC2328-A.4.4-1` | This field is not meaningful and must be zero for Type 4 summary-LSAs. (§A.4.4) | MUST | A.4.4 - Appendix subsection A.4.4 | **positive:** `unit/verify` [`TestRFC2328Type4SummaryMaskIsZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L490). **negative:** `unit/verify` [`TestRFC2328Type3SummaryKeepsMask`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L509) |
| `RFC2328-A.2-1` | Routers should reset (i.e. clear) unrecognized bits in the Options field when sending Hello packets or Database Description packets and when originating LSAs. (§A.2) | SHOULD | A.2 - Appendix subsection A.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-A.2-2` | Conversely, routers encountering unrecognized Option bits in received Hello Packets, Database Description packets or LSAs should ignore the capability and process the packet/LSA normally. (§A.2) | SHOULD | A.2 - Appendix subsection A.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-9.5-1` | The E-bit of the Options field should be set if and only if the attached area is capable of processing AS-external-LSAs (i.e., it is not a stub area). If the E-bit is set incorrectly the neighboring routers will refuse to accept the Hello Packet (see Section 10.5). (§9.5) | SHOULD | 9.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-C.1-1` | In order to minimize the chance of routing loops, all OSPF routers in an OSPF routing domain should have RFC1583Compatibility set identically. When there are routers present that have not been updated with the functionality specified in Section 16.4.1 of this memo, all routers should have RFC1583Compatibility set to "enabled". Otherwise, all routers should have RFC1583Compatibility set to "disabled", preventing all routing loops. (§C.1) | SHOULD | C.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-B-1` | If the LS age field of one of the router's self-originated LSAs reaches the value LSRefreshTime, a new instance of the LSA is originated, even though the contents of the LSA (apart from the LSA header) will be the same. The value of LSRefreshTime is set to 30 minutes. (§B) | SHOULD | B | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-14-3` | When, in the process of aging the link state database, an LSA's LS age hits a multiple of CheckAge, its LS checksum should be verified. If the LS checksum is incorrect, a program or memory error has been detected, and at the very least the router itself should be restarted. (§14) | SHOULD | 14 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-14.1-1` | Premature aging can also be used when, for example, one of the router's previously advertised external routes is no longer reachable. In this circumstance, the router can flush its AS- external-LSA from the routing domain via premature aging. This procedure is preferable to the alternative, which is to originate a new LSA for the destination specifying a metric of LSInfinity. (§14.1) | SHOULD | 14.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-13.5-2` | The fixed interval between a router's delayed transmissions must be short (less than RxmtInterval) or needless retransmissions will ensue. (§13.5) | SHOULD | 13.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-C.3-2` | This is also advertised in the router's Hello Packets in their RouterDeadInterval field. This should be some multiple of the HelloInterval (say 4). (§C.3) | SHOULD | C.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-A.4.5-1` | If the Forwarding address is set to 0.0.0.0, data traffic will be forwarded instead to the LSA's originator (i.e., the responsible AS boundary router). (§A.4.5) | MAY | A.4.5 - Appendix subsection A.4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-16.1-2` | The specification does not require that the above two stage method be used to calculate the shortest path tree. However, if another algorithm is used, an identical tree must be produced. (§16.1) | MAY | 16.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-15-1` | To establish/maintain connectivity of the backbone, virtual links can be configured through non-backbone areas. Virtual links serve to connect physically separate components of the backbone. The two endpoints of a virtual link are area border routers. The virtual link must be configured in both routers. The configuration information in each router consists of the other virtual endpoint (the other area border router), and the non-backbone area the two routers have in common (called the Transit area). Virtual links cannot be configured through stub areas (see Section 3.6). (§15) | MAY | 15 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-D.3-3` | An interface may have multiple keys active at any one time. This enables smooth transition from one key to another. Each key has four time constants associated with it. (§D.3) | MAY | D.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2328-13.3-2`](#rfc2328-13.3-2) The LSA's LS age must be incremented by InfTransDelay (which must be > 0) when it is copied into the outgoing Link State Update packet (until the LS age field reaches the maximum value of MaxAge). (§13.3) | {gap}, no test | the InfTransDelay bump is applied on the retransmit path (RetransmitTick, lsdb/flooding.go:545) and on a direct database-copy reply (sendDirectLSUpdate, lsdb/flooding.go:745; sendDirectLinkLSUpdate, lsdb/link_scope.go:349), but NOT on the normal flood: floodExcept builds the outgoing copy with `entry.LSA(d.now())` (lsdb/flooding.go:351), and Entry.LSA calls `e.Raw(now, 0)` with transmitDelay 0 (lsdb/entry.go:62-68, 75-85), so the first flooded copy carries the unincremented LS age. The MaxAge cap itself is present wherever the bump is applied (LSAge.Add, types/lsage.go:54-63). Disclosed in docs/features/rfc-status.md RFC 2328 row |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2328-A.3.1-1`](#rfc2328-a.3.1-1)

The version number field must specify protocol version 2. (§8.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting or emitting a Version other than 2. Red: TestOSPFHeaderRejectsBadVersionAndLength fails unless DecodeHeader returns ErrBadVersion for Version 3. Other polarity: TestOSPFHeaderRoundTrip fails unless every packet type written by WriteTo decodes, which needs the emitted Version octet to equal 2. One clause, both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFHeaderRejectsBadVersionAndLength`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/header_test.go#L153) | unit/verify | unproven |
| positive | [`TestOSPFHeaderRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/header_test.go#L131) | unit/verify | unproven |

### [`RFC2328-A.3.1-2`](#rfc2328-a.3.1-2)

The standard IP checksum of the entire contents of the packet, starting with the OSPF packet header but excluding the 64-bit authentication field. (§A.3.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestOSPFPacketChecksum / TestPacketVerifyChecksum only round-trip and corrupt a body octet; no unit shows the 64-bit authentication field is excluded (changing auth octets must leave the checksum valid), and the verifier shares PacketChecksum with the generator.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPacketVerifyChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/header_test.go#L100) | unit/verify | unproven |
| positive | [`TestOSPFPacketChecksumExcludesAuth`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L30) | unit/verify | unproven |

### [`RFC2328-12.1.7-1`](#rfc2328-12.1.7-1)

This field is the checksum of the complete contents of the LSA, excepting the LS age field. The LS age field is excepted so that an LSA's age can be incremented without updating the checksum. The checksum used is the same that is used for ISO connectionless datagrams; it is commonly referred to as the Fletcher checksum. It is documented in Annex B of [Ref6]. The LSA header also contains the length of the LSA in bytes; subtracting the size of the LS age field (two bytes) yields the amount of data to checksum. The checksum is used to detect data corruption of an LSA. This corruption can occur while an LSA is being flooded, or while it is being held in a router's memory. The LS checksum field cannot take on the value of zero; the occurrence of such a value should be considered a checksum failure. In other words, calculation of the checksum is not optional. (§12.1.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: (a) checksum covering LS age: TestOSPFLSAChecksumExcludesAge fails when the LS age octets are mutated and the LSA stops verifying, and TestOSPFLSAChecksum checks independent Fletcher sums over lsa[2:length] (not VerifyLSAChecksum's shared code); (b) an unchecked covered region: mutating Options must fail VerifyLSAChecksum; (c) accepting LS Checksum 0: TestRFC2328ZeroLSChecksumRejected solves the Fletcher sums to zero with the field zero, so only the zero rule can reject, and fails if VerifyLSAChecksum accepts it. Positive: the encoded LSA verifies with a non-zero backfilled checksum. Every clause has its assertion. Re-judged 2026-09-27 after the RFC905-x-6 tags moved here (J102). TestOSPFLSAChecksumExcludesAge now also carries the negative tag for the covered-octet mutation, clause (b). TestFletcherRFC905Vectors and TestFletcherIgnoresLSAge (ospf/types) add positives: the first pins 0x15ff over the window from Options, and the second shows LS Age mutation leaves the checksum unchanged. The verdict is unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFLSAChecksumExcludesAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L96) | unit/verify | unproven |
| negative | [`TestRFC2328ZeroLSChecksumRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc2328_test.go#L10) | unit/verify | unproven |
| positive | [`TestOSPFLSAChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L72) | unit/verify | unproven |
| positive | [`TestOSPFLSAChecksumExcludesAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L98) | unit/verify | unproven |
| positive | [`TestFletcherIgnoresLSAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L46) | unit/verify | unproven |
| positive | [`TestFletcherRFC905Vectors`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L26) | unit/verify | unproven |

### [`RFC2328-13-1`](#rfc2328-13-1)

Validate the LSA's LS checksum. If the checksum turns out to be invalid, discard the LSA and get the next one from the Link State Update packet. (2) Examine the LSA's LS type. If the LS type is unknown, discard the LSA and get the next one from the Link State Update Packet. (§13)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. bad-checksum discard is proven on the flooding path (TestRFC2328BadLSChecksumDiscarded), but the unknown-LS-type clause is exercised only through DecodeLSReq (an LS Request decoder, TestDecodeLSReqRejectsMalformed), never an LSA of unknown type inside a Link State Update reaching ReceiveUpdate.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328BadLSChecksumDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_test.go#L17) | unit/verify | unproven |
| negative | [`TestDecodeLSReqRejectsMalformed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/packet_body_test.go#L145) | unit/verify | unproven |
| positive | [`TestOSPFFloodOutOtherInterfaces`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L30) | unit/verify | unproven |

### [`RFC2328-13-2`](#rfc2328-13-2)

AS-external-LSAs are not flooded into/throughout stub areas (§13)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clauses: not flooded INTO a stub area (send side) and not THROUGHOUT it (receive side). Receive side is proven through ReceiveUpdate: TestOSPFStubAreaDropsType5 fails if a Type-5 received on a stub interface is installed. Send side is proven only on the helper: TestOSPFStubFloodFilter asserts eligibleInterface returns false for stub/NSSA, but no unit floods a Type-5 and observes that no LS Update leaves a stub interface, so a flood path (flooding.go:360, :599) that stopped consulting eligibleInterface stays green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFStubAreaDropsType5`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L247) | unit/verify | unproven |
| positive | [`TestOSPFStubFloodFilter`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/area_type_test.go#L15) | unit/verify | unproven |

### [`RFC2328-13-3`](#rfc2328-13-3)

If the neighbor is in a lesser state than Exchange, the packet should be dropped without further processing. (§13)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: processing an LS Update or LS Ack from a neighbor below Exchange. TestRFC2328FloodingRequiresExchangeOrHigher proves AcceptsFlooding returns reasonState for Init and 2-Way and empty from Exchange. The quoted obligation is the DROP 'without further processing', which happens in the engine (instance.go:373, :413); no tagged unit dispatches a packet from a pre-Exchange neighbor and asserts it never reaches the LSDB, so an engine that ignored the returned reason stays green (ext_receive_test.go may cover it but carries no tag for this id).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328FloodingRequiresExchangeOrHigher`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L20) | unit/verify | unproven |
| positive | [`TestRFC2328FloodingRequiresExchangeOrHigher`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L17) | unit/verify | unproven |

### [`RFC2328-13.1-1`](#rfc2328-13.1-1)

The LSA having the newer LS sequence number is more recent. See Section 12.1.6 for an explanation of the LS sequence number space. If both instances have the same LS sequence number, then: o If the two instances have different LS checksums, then the instance having the larger LS checksum (when considered as a 16-bit unsigned integer) is considered more recent. o Else, if only one of the instances has its LS age field set to MaxAge, the instance of age MaxAge is considered to be more recent. o Else, if the LS age fields of the two instances differ by more than MaxAgeDiff, the instance having the smaller (younger) LS age is considered to be more recent. (§13.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestOSPFFreshnessCompareMatrix asserts each step on its own: higher sequence newer (both directions), larger checksum newer, MaxAge newer, age difference above MaxAgeDiff younger newer, within MaxAgeDiff equal. The ORDER of the steps is not discriminated: every case varies one field with the others equal, so a comparator checking checksum or MaxAge before sequence stays green. The unsigned 16-bit checksum comparison is not discriminated (10 vs 11), nor is the signed sequence space of 12.1.6. TestRFC2328OlderInstanceGetsDatabaseCopyBack proves only that an older sequence never replaces the copy.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328OlderInstanceGetsDatabaseCopyBack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_test.go#L50) | unit/verify | unproven |
| positive | [`TestOSPFFreshnessCompareMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/lsdb_test.go#L126) | unit/verify | unproven |

### [`RFC2328-13-4`](#rfc2328-13-4)

If the database copy has LS age equal to MaxAge and LS sequence number equal to MaxSequenceNumber, simply discard the received LSA without acknowledging it. (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: acknowledging, or sending the database copy back for, an LSA received while the database copy is MaxAge at MaxSequenceNumber. Red: TestOSPFMaxSeqMaxAgeSilentDiscard installs a MaxSequenceNumber copy aged to MaxAge, receives an older instance, flushes delayed acks, and fails on any send. Other polarity: TestRFC2328OlderInstanceGetsDatabaseCopyBack fails if an ordinary more-recent copy is not sent back to the sender (so a blanket silent discard goes red). One clause, both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328OlderInstanceGetsDatabaseCopyBack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_test.go#L53) | unit/verify | unproven |
| positive | [`TestOSPFMaxSeqMaxAgeSilentDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_edges_test.go#L19) | unit/verify | unproven |

### [`RFC2328-13.3-1`](#rfc2328-13.3-1)

LSAs flooded out an adjacency are placed on the adjacency's Link state retransmission list. In order to ensure that flooding is reliable, these LSAs are retransmitted until they are acknowledged. The length of time between retransmissions is a configurable per-interface value, RxmtInterval. (§13.6)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clauses: flooded LSAs go on the adjacency retransmission list (TestOSPFFloodOutOtherInterfaces, TestOSPFFloodQueuesExchangeAndLoadingNeighbors: eth1/Exchange/Loading queued, 2-Way and receiving neighbor not); retransmitted UNTIL acknowledged every RxmtInterval (TestOSPFRetransmitTimer: none at t=0, one at t=6s, list cleared on ack); RxmtInterval configurable per interface. Not proven: only one retransmission happens before the ack, so a producer that retransmits once and gives up stays green; and the 5s interval is the default, never a configured per-interface value, so a hard-coded 5s stays green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFFloodQueuesExchangeAndLoadingNeighbors`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L57) | unit/verify | unproven |
| positive | [`TestOSPFFloodOutOtherInterfaces`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L31) | unit/verify | unproven |
| positive | [`TestOSPFRetransmitTimer`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L129) | unit/verify | unproven |

### [`RFC2328-13.3-2`](#rfc2328-13.3-2)

The LSA's LS age must be incremented by InfTransDelay (which must be > 0) when it is copied into the outgoing Link State Update packet (until the LS age field reaches the maximum value of MaxAge). (§13.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2328-13.3-2, so no unit is bound to it.

### [`RFC2328-14-1`](#rfc2328-14-1)

An LSA's LS age is never incremented past the value MaxAge. LSAs having age MaxAge are not used in the routing table calculation. (§14)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: (a) LS age incremented past MaxAge: TestLSAgeAddSaturates fails unless MaxAge-10 plus 20 equals MaxAge exactly, and TestOSPFLSDBAgeToPurge fails unless the aged LSA sits at MaxAge; (b) a MaxAge LSA used in the routing calculation: TestOSPFGraphSkipsMaxAge fails if BuildGraph makes a router vertex of a MaxAge router-LSA. The fixture is isolated: BuildGraph (graph.go) turns a decodable non-MaxAge router-LSA into a vertex with no other filter, so only the age check can exclude it. Positive: age advances and saturation leaves DoNotAge untouched.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFGraphSkipsMaxAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/graph_test.go#L40) | unit/verify | unproven |
| positive | [`TestOSPFLSDBAgeToPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/aging_test.go#L34) | unit/verify | unproven |
| positive | [`TestLSAgeAddSaturates`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/lsage_test.go#L39) | unit/verify | unproven |

### [`RFC2328-14-2`](#rfc2328-14-2)

A MaxAge LSA must be removed immediately from the router's link state database as soon as both a) it is no longer contained on any neighbor Link state retransmission lists and b) none of the router's neighbors are in states Exchange or Loading. (§14)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clauses: delete the MaxAge LSA as soon as a) it is on no retransmission list and b) no neighbor is in Exchange OR Loading. (a) is proven with both polarities by TestOSPFASExternalPurgeRetainedAcrossAreas (kept after the first ack, deleted on the last). (b) is proven only for Loading (TestOSPFPurgeRetainedForExchangeOrLoading); no unit holds a neighbor in Exchange, so a guard that checked Loading alone stays green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFPurgeRetainedForExchangeOrLoading`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L383) | unit/verify | unproven |
| positive | [`TestOSPFASExternalPurgeRetainedAcrossAreas`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L332) | unit/verify | unproven |

### [`RFC2328-13.5-1`](#rfc2328-13.5-1)

Each newly received LSA must be acknowledged. This is usually done by sending Link State Acknowledgment packets. However, acknowledgments can also be accomplished implicitly by sending Link State Update packets (see step 7a of Section 13). (§13.5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: a newly received LSA left unacknowledged. The tagged units prove the implicit ack by flooding back (TestOSPFDRRefloodsBackOutReceivingInterface: no delayed ack), the implied-ack duplicate on the Backup and the MaxAge-no-copy direct ack through ReceiveUpdate. The ordinary case, a newer LSA installed through ReceiveUpdate and not flooded back, is only reached by calling ackForReceive directly with hand-set flags in TestOSPFAckDecisionTable, so a ReceiveUpdate that skipped or mis-flagged that call stays green. (TestRFC2328MinLSArrivalElapsedAccepts proves it through ReceiveUpdate but is tagged only for 13-7.)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFDRRefloodsBackOutReceivingInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_edges_test.go#L80) | unit/verify | unproven |
| positive | [`TestOSPFAckDecisionTable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L93) | unit/verify | unproven |
| positive | [`TestOSPFUnknownMaxAgeNoCopyIsAckedAndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L360) | unit/verify | unproven |

### [`RFC2328-13.4-1`](#rfc2328-13.4-1)

A self-originated LSA is detected when either 1) the LSA's Advertising Router is equal to the router's own Router ID or 2) the LSA is a network- LSA and its Link State ID is equal to one of the router's own IP interface addresses. (§13.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. only the Advertising Router == own Router ID detection is exercised; clause 2) a network-LSA whose Link State ID is one of the router own interface addresses is never tested. The positive also calls handleSelfReceived directly rather than through ReceiveUpdate.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFSelfOriginatedNoLocalCopyFlush`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_edges_test.go#L43) | unit/verify | unproven |
| positive | [`TestOSPFOriginateSelfReceivedHigherSeq`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/origination_test.go#L426) | unit/verify | unproven |

### [`RFC2328-16.1-1`](#rfc2328-16.1-1)

Look up the vertex W's LSA (router-LSA or network-LSA) in Area A's link state database. If the LSA does not exist, or its LS age is equal to MaxAge, or it does not have a link back to vertex V, examine the next link in V's LSA. (§16.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. tagged units cover only the link-back clause (one-way link excluded); the missing-LSA and MaxAge-LSA clauses are not asserted under this id (MaxAge is covered by TestOSPFGraphSkipsMaxAge under 14-1).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFTwoWayCheck`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/spf_test.go#L31) | unit/verify | unproven |
| positive | [`TestOSPFSPFShortestPath`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/spf_test.go#L13) | unit/verify | unproven |

### [`RFC2328-16.4-1`](#rfc2328-16.4-1)

Intra-area and inter-area paths are always preferred over AS external paths. (b) Type 1 external paths are always preferred over type 2 external paths. When all paths are type 2 external paths, the paths with the smallest advertised type 2 metric are always preferred. (§16.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. intra-area over external and E1 over E2 are asserted; inter-area over external and the smallest type-2 metric among E2 paths are not.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFExternalE1PreferredOverE2`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external_test.go#L83) | unit/verify | unproven |
| positive | [`TestOSPFRouteTablePreference`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/route_test.go#L8) | unit/verify | unproven |

### [`RFC2328-16.2-1`](#rfc2328-16.2-1)

If the router is attached to multiple areas (i.e., it is an area border router), only backbone summary-LSAs are examined. (§16)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: an ABR examining non-backbone summary-LSAs. Red: TestOSPFABRBackboneOnlyAcceptance fails if the cheaper area-0.0.0.1 summary is used (metric must be 30 via 2.2.2.2). The condition 'if the router is attached to multiple areas' is not discriminated: the positive TestOSPFInterAreaRoute uses a router attached only to the BACKBONE, so a producer that examined only backbone summaries for every router stays green; no unit shows a non-ABR in a non-backbone area using that area's summaries.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFABRBackboneOnlyAcceptance`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/interarea_test.go#L74) | unit/verify | unproven |
| positive | [`TestOSPFInterAreaRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/interarea_test.go#L43) | unit/verify | unproven |

### [`RFC2328-16.2-2`](#rfc2328-16.2-2)

If the cost specified by the LSA is LSInfinity, or if the LSA's LS age is equal to MaxAge, then examine the the next LSA. (2) If the LSA was originated by the calculating router itself, examine the next LSA. (§16.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. MaxAge and self-originated skips are asserted for summary and AS-external LSAs, but the LSInfinity clause is only tested by a composed cost saturating at LSInfinity; no unit feeds an LSA whose advertised cost is LSInfinity.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFExternalLSInfinityDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external_test.go#L61) | unit/verify | unproven |
| negative | [`TestOSPFInterAreaLSInfinityDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/interarea_test.go#L137) | unit/verify | unproven |
| negative | [`TestRFC2328ExternalSkipsMaxAgeAndSelf`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_test.go#L43) | unit/verify | unproven |
| negative | [`TestRFC2328InterAreaSkipsMaxAgeAndSelfSummary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_test.go#L20) | unit/verify | unproven |
| positive | [`TestOSPFInterAreaRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/interarea_test.go#L44) | unit/verify | unproven |

### [`RFC2328-D.2-1`](#rfc2328-d.2-1)

The 64-bit authentication field in the OSPF packet header must be equal to the 64-bit password (i.e., authentication key) that has been configured for the interface. (§D.5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting a Simple-password packet whose 64-bit authentication field differs from the configured password. Red: TestRFC2328SimplePasswordMismatchDiscarded fails if a packet signed with 'badpass' is accepted, and fails unless the reason is exactly password-mismatch (isolated: the AuType matches). Other polarity: the same unit fails if the configured 'goodpass' is rejected. One clause, both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328SimplePasswordMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_test.go#L99) | unit/verify | unproven |
| positive | [`TestRFC2328SimplePasswordMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_test.go#L95) | unit/verify | unproven |

### [`RFC2328-D.3-1`](#rfc2328-d.3-1)

The message digest appended to the OSPF packet is not actually considered part of the OSPF protocol packet: the message digest is not included in the OSPF header's packet length, although it is included in the packet's IP header length field. (§D.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clauses: the digest is NOT counted in the OSPF Packet Length, and IS counted in the IP header length. The first is proven: TestOSPFAuthSignVerifyCrypto fails unless Packet Length stays plen and len(signed) == plen + digest length, and TestOSPFAuthCryptoRejectsExtraTrailerBytes fails if a mismatched trailer framing is accepted. The second is not: no unit asserts that the whole signed buffer, digest included, is what Ze hands to the socket, which is the boundary Ze owns for the IP length. The note's 'the kernel sends the whole buffer' is a claim no tagged unit checks.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthCryptoRejectsExtraTrailerBytes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L219) | unit/verify | unproven |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L47) | unit/verify | unproven |

### [`RFC2328-D.3-2`](#rfc2328-d.3-2)

Whenever an OSPF packet is accepted as authentic, the cryptographic sequence number is set to the received packet's sequence number. (§D.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an authentic packet's sequence number not becoming the stored value, and an AuType 2 sequence treated as anything but non-decreasing. Red: TestOSPFAuthReplay accepts 10 then 11, accepts a second 11 (D.4.3 discards only 'less than'), then rejects 10 with reason replay, which holds only if 11 (not 10) was stored; 5 is rejected. TestOSPFAuthReplayEqualSequenceByAuType asserts the AuType 2 equal-sequence acceptance on its own. TestNeighborDownResetsCryptoSeq adds the per-neighbor reset. RFC 7474's strictly-greater rule governs only its AuType 3 (replayedSequence).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L117) | unit/verify | revert, verified |
| positive | [`TestNeighborDownResetsCryptoSeq`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L246) | unit/verify | unproven |
| positive | [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L116) | unit/verify | revert, verified |
| positive | [`TestOSPFAuthReplayEqualSequenceByAuType`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L173) | unit/verify | revert, verified |

### [`RFC2328-D.4.3-1`](#rfc2328-d.4.3-1)

The checksum field in the standard OSPF header is not calculated, but is instead set to 0. (§D.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Non-compliant behaviour: an AuType 2 packet whose standard-header Checksum is calculated rather than set to 0. TestOSPFPacketChecksumZeroForAuType2 encodes a cryptographic-auth Hello through WriteTo and fails ('AuType2 checksum = %#04x, want 0') on any non-zero field. Single-polarity positive per the row's marker: the rule is generate-side and there is no receive path to reject.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestOSPFPacketChecksumZeroForAuType2`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L53) | unit/verify | revert, verified |

### [`RFC2328-A.3.3-1`](#rfc2328-a.3.3-1)

Interface MTU should be set to 0 in Database Description packets sent over virtual links. (§A.3.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a DD sent over a virtual link carrying a non-zero Interface MTU. Red: TestRFC2328VirtualLinkDBDescCarriesZeroMTU fails on any sent DD with InterfaceMTU != 0 over the MTU-less virtual interface, and TestRFC2328VirtualInterfaceHasNoMTU fails unless the engine-created virtual interface reports MTU 0 (so the DD stamps 0). Other polarity: the same units fail unless a real interface's DD carries 1500 and the real interface reports a non-zero MTU, so a blanket zero goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328VirtualLinkDBDescCarriesZeroMTU`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L63) | unit/verify | unproven |
| positive | [`TestRFC2328VirtualLinkDBDescCarriesZeroMTU`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L58) | unit/verify | unproven |
| positive | [`TestRFC2328VirtualInterfaceHasNoMTU`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_test.go#L32) | unit/verify | unproven |

### [`RFC2328-10.1-1`](#rfc2328-10.1-1)

Only one Database Description Packet is allowed outstanding at any one time. (§10.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. positive only checks that a DBDesc is resent after RxmtInterval, not that it is the same outstanding packet or that no new DD is sent before the ack; negative exercises the slave duplicate-resend of section 10.8, not the one-outstanding limit on the master.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFDuplicateDD`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/nsm_test.go#L595) | unit/verify | unproven |
| positive | [`TestOSPFDDRetransmit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/nsm_test.go#L527) | unit/verify | unproven |

### [`RFC2328-10.2-1`](#rfc2328-10.2-1)

If an LSA cannot be found in the database, something has gone wrong with the Database Exchange process, and neighbor event BadLSReq should be generated. (§10.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an LS Request for an LSA absent from the database not generating BadLSReq. Red: TestOSPFBadLSReqRestart fails on HandleLSReq != reasonBadLSReq, on state != exstart, and on the resent DD not being the initial I|M|MS DD. Other polarity: TestOSPFValidLSReqSendsLSUpdate and TestRFC2328KnownLSRequestDoesNotRestartExchange go red if a satisfiable request restarts the exchange (state change, exstart) or is not answered with one LS Update carrying that LSA. One clause, both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFValidLSReqSendsLSUpdate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/nsm_test.go#L905) | unit/verify | unproven |
| negative | [`TestRFC2328KnownLSRequestDoesNotRestartExchange`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L121) | unit/verify | unproven |
| positive | [`TestOSPFBadLSReqRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/nsm_test.go#L734) | unit/verify | unproven |

### [`RFC2328-C.3-1`](#rfc2328-c.3-1)

The interface output cost must always be greater than 0. (§C.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: an interface running with output cost 0. TestInterfaceCostAndTransmitDelayBoundary proves validateConfig rejects an explicit cost 0 with ErrInterfaceCostZero and accepts 1. It also accepts an UNSET cost 'defaulted later' without asserting the default is greater than 0, so a producer that defaulted the cost to 0 stays green; 'must always be greater than 0' covers every source of the cost, not only the explicit one.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInterfaceCostAndTransmitDelayBoundary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_interface_validate_test.go#L17) | unit/verify | unproven |
| positive | [`TestInterfaceCostAndTransmitDelayBoundary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_interface_validate_test.go#L16) | unit/verify | unproven |

### [`RFC2328-3.6-1`](#rfc2328-3.6-1)

One or more of the stub area's area border routers must advertise a default route into the stub area via summary-LSAs. (§3.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an ABR attached to a stub area not advertising a default route into it via a summary-LSA. Red: TestRFC2328StubAreaGetsDefaultSummary fails unless the stub area holds a Type 3 summary with Link State ID 0.0.0.0, mask 0.0.0.0 and the configured cost 5. Other polarity: TestRFC2328NormalAreaGetsNoDefaultSummary fails if a normal area receives one, and still checks the ordinary summary is originated. One clause, both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328NormalAreaGetsNoDefaultSummary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_abr_summary_test.go#L53) | unit/verify | revert, verified |
| positive | [`TestRFC2328StubAreaGetsDefaultSummary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_abr_summary_test.go#L34) | unit/verify | revert, verified |

### [`RFC2328-4.4-1`](#rfc2328-4.4-1)

Support for receiving and sending IP multicast datagrams, along with the appropriate lower-level protocol support, is required. (§4.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. only the receive side is shown (AllSPFRouters membership installed, foreign group refused); nothing asserts the sending side of IP multicast (multicast interface selection or a send to the group).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFMulticastMembershipRefusesForeignGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_socket_linux_test.go#L97) | unit/verify | revert, verified |
| positive | [`TestOSPFMulticastMembershipInstalled`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_socket_linux_test.go#L76) | unit/verify | revert, verified |

### [`RFC2328-4.4-2`](#rfc2328-4.4-2)

The router's IP protocol support must include the ability to divide a single IP class A, B, or C network number into many subnets of various sizes. This is commonly called variable-length subnetting; see Section 3.5 for details. IP supernetting support The router's IP protocol support must include the ability to aggregate contiguous collections of IP class A, B, and C networks into larger quantities called supernets. (§4.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a classful routing table (mask derived from address class) that cannot carry variable-length subnets or supernets. Red: TestRFC2328ClasslessPrefixesReachIPRIB fails unless 192.0.0.0/16 (a supernet of class-C networks), 192.0.2.0/24 and 192.0.2.128/25 (a variable-length subnet) each reach the IP Loc-RIB with its own next hop. Other polarity: TestRFC2328ClasslessWithdrawalPreservesOverlaps fails if withdrawing the /24 removes the /16 or the /25. Both the VLSM and the supernet clause have a prefix that a classful table would lose.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328ClasslessWithdrawalPreservesOverlaps`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_classless_test.go#L51) | unit/verify | unproven |
| positive | [`TestRFC2328ClasslessPrefixesReachIPRIB`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_classless_test.go#L34) | unit/verify | unproven |

### [`RFC2328-4.4-3`](#rfc2328-4.4-3)

Indications must be passed from these protocols to OSPF as the network interface goes up and down. (§4.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. SB-3 strict re-read 2026-09-27. Forbidden: interface up/down transitions not reaching OSPF. TestOSPFTransportPassesLinkIndicationsToOSPF fails unless HandleLinkUp/HandleLinkDown call a callback the test registers, and the negative shows a non-OSPF interface produces none. No tagged unit drives the engine: that OSPF registers these callbacks (instance.go:307-308) and reacts to them is a code reading, so an engine that stopped registering, and so never heard of a link change, leaves both units green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFTransportPassesNoIndicationForNonOSPFInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_link_test.go#L45) | unit/verify | revert, verified |
| positive | [`TestOSPFTransportPassesLinkIndicationsToOSPF`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_link_test.go#L8) | unit/verify | revert, verified |

### [`RFC2328-8.1-1`](#rfc2328-8.1-1)

On these interfaces, the IP source should be set to any of the other IP addresses belonging to the router. For this reason, there must be at least one IP address assigned to the router. (§8.1)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. REV-OSPF independent re-audit 2026-09-27 (did not author the fix). Section 8.1: 'Interfaces to unnumbered point-to-point networks have no associated IP address.  On these interfaces, the IP source should be set to any of the other IP addresses belonging to the router.  For this reason, there must be at least one IP address assigned to the router.' The negative TestOSPFRefusesInterfaceWithoutIPv4Source asserts interfaceIPv4 (transport/backend_linux.go) REFUSES an interface with no IPv4 address, the opposite of what the sentence requires for an unnumbered point-to-point interface (source from another router address). The positive TestResolveOSPFInterfaceUsesIfaceResolverOSName proves a numbered interface sources from its own address, which is not a clause of this row. Neither unit proves a clause of the quote, so the exception for a row carrying passing tests of other clauses does not apply: the tests assert something the RFC does not say. Unnumbered point-to-point is an absent feature (no ifIndex LinkData, no config), owner scope decision owed; with that decision the row becomes unimplemented with a {gap} marker, which this job may not add because rfc/short is out of its scope.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFRefusesInterfaceWithoutIPv4Source`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/backend_linux_test.go#L101) | unit/verify | unproven |
| positive | [`TestResolveOSPFInterfaceUsesIfaceResolverOSName`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/backend_linux_test.go#L17) | unit/verify | unproven |

### [`RFC2328-8.2-1`](#rfc2328-8.2-1)

In order for the packet to be accepted at the IP level, it must pass a number of tests, even before the packet is passed to OSPF for processing: o The IP checksum must be correct. o The packet's IP destination address must be the IP address of the receiving interface, or one of the IP multicast addresses AllSPFRouters or AllDRouters. o The IP protocol specified must be OSPF (89). (§8.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Each listed test has both polarities through deliverDatagram, the Linux receive path. IP checksum: the 'checksum' case fails if a datagram with a broken header checksum is delivered. Destination: accepted for the interface address, AllSPFRouters and AllDRouters (TestOSPFReceiveAcceptsValidIPv4Envelope fails if any is rejected or not delivered), 'destination' case fails if a foreign address is delivered. Protocol: 'protocol' case (17) fails if delivered, while the valid datagrams carry 89. Rejections also assert one drop recorded and nothing on recvCh.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFReceiveRejectsInvalidIPv4Envelope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/backend_linux_test.go#L151) | unit/verify | unproven |
| positive | [`TestOSPFReceiveAcceptsValidIPv4Envelope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/backend_linux_test.go#L130) | unit/verify | unproven |

### [`RFC2328-8.2-2`](#rfc2328-8.2-2)

The Area ID found in the OSPF header must be verified. If both of the following cases fail, the packet should be discarded. The Area ID specified in the header must either: (1) Match the Area ID of the receiving interface. In this case, the packet has been sent over a single hop. Therefore, the packet's IP source address is required to be on the same network as the receiving interface. This can be verified by comparing the packet's IP source address to the interface's IP address, after masking both addresses with the interface mask. This comparison should not be performed on point-to-point networks. On point-to-point networks, the interface addresses of each end of the link are assigned independently, if they are assigned at all. (2) Indicate the backbone. In this case, the packet has been sent over a virtual link. The receiving router must be an area border router, and the Router ID specified in the packet (the source router) must be the other end of a configured virtual link. The receiving interface must also attach to the virtual link's configured Transit area. (§8.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. SB-3 strict re-read 2026-09-27, quote widened to the full span of both cases (the old row claimed case 2; the one-case quote narrowed it). Forbidden: accepting a packet that fails both cases, and discarding one that passes either. Case (1) has both polarities through dispatch: TestOSPFReceiveAcceptsMatchingAreaID fails if a matching Area ID is dropped or makes no neighbor, TestOSPFReceiveDropsMismatchedAreaID fails unless a foreign Area ID is dropped with no neighbor. Case (2), a backbone Area ID accepted over a configured virtual link from its other endpoint by an ABR on the Transit area, has no unit, so a receiver that discarded every packet failing case (1), including a legitimate virtual-link packet, stays green. The case (1) source-network check (not on point-to-point) is not asserted either. A blind reader read enforced from case (1) alone; the quote's 'if both of the following cases fail' makes case (2) a clause.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFReceiveDropsMismatchedAreaID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_receive_test.go#L65) | unit/verify | revert, verified |
| positive | [`TestOSPFReceiveAcceptsMatchingAreaID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_receive_test.go#L49) | unit/verify | revert, verified |

### [`RFC2328-8.2-3`](#rfc2328-8.2-3)

The AuType specified in the packet must match the AuType specified for the associated area. (§8.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: accepting a packet whose AuType differs from the area's. TestOSPFReceiveDropsMismatchedAuType sends AuType 0 into a simple-password area and asserts a drop, an auth failure and no neighbor; TestOSPFReceiveAcceptsMatchingAuType accepts AuType 1 with the right password. The negative is not isolated: a null-AuType packet carries no password, so a receiver that skipped the AuType check and failed on the password comparison produces the same drop and failure count; the unit never asserts the autype-mismatch reason. The reverse mismatch (AuType 1 into a null area) is not tested.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFReceiveDropsMismatchedAuType`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_receive_test.go#L104) | unit/verify | revert, verified |
| positive | [`TestOSPFReceiveAcceptsMatchingAuType`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_receive_test.go#L82) | unit/verify | revert, verified |

### [`RFC2328-9.1-1`](#rfc2328-9.1-1)

The router must also originate a network-LSA for the network node. (§9.1, §12.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: the Designated Router for a network not originating its network-LSA. Red: TestRFC2328DROriginatesNetworkLSA fails unless OriginateFromTopology produces a network-LSA for eth1 (where 1.1.1.1 is DR) listing exactly [1.1.1.1 3.3.3.3]. Other polarity: TestRFC2328NonDROriginatesNoNetworkLSA fails if the Backup on eth0 originates one. One clause, both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328NonDROriginatesNoNetworkLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L404) | unit/verify | revert, verified |
| positive | [`TestRFC2328DROriginatesNetworkLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L384) | unit/verify | revert, verified |

### [`RFC2328-9.5.1-1`](#rfc2328-9.5.1-1)

If the router is eligible to become Designated Router, it must periodically send Hello Packets to all neighbors that are also eligible. In addition, if the router is itself the Designated Router or Backup Designated Router, it must also send periodic Hello Packets to all other neighbors. This means that any two eligible routers are always exchanging Hello Packets, which is necessary for the correct operation of the Designated Router election algorithm. To minimize the number of Hello Packets sent, the number of eligible routers on an NBMA network should be kept small. If the router is not eligible to become Designated Router, it must periodically send Hello Packets to both the Designated Router and the Backup Designated Router (if they exist). (§9.5.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. SB-3 strict re-read 2026-09-27, quote widened to the full span through the ineligible-router sentence the old row claimed. Clauses: an eligible router sends periodic Hellos to every eligible neighbor; a DR or BDR also sends them to all other neighbors; an ineligible router sends them to the DR and BDR. TestOSPFNBMAEligibleRouterHellosEveryEligibleNeighbor and TestOSPFNBMAPeriodicHelloSkipsIneligibleNeighbor prove only the eligible DROther case (every eligible neighbor targeted, a priority-0 one skipped). No unit makes the router DR or BDR and asserts Hellos to ineligible neighbors, and none makes the router ineligible.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFNBMAPeriodicHelloSkipsIneligibleNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc2328_dr_test.go#L92) | unit/verify | revert, verified |
| positive | [`TestOSPFNBMAEligibleRouterHellosEveryEligibleNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc2328_dr_test.go#L75) | unit/verify | revert, verified |

### [`RFC2328-10.5-1`](#rfc2328-10.5-1)

Next, the values of the Network Mask, HelloInterval, and RouterDeadInterval fields in the received Hello packet must be checked against the values configured for the receiving interface. Any mismatch causes processing to stop and the packet to be dropped. (§10.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting a Hello whose Network Mask, HelloInterval or RouterDeadInterval differs from the receiving interface. Red: TestRFC2328HelloMismatchRejectedAndNoTwoWayUnlisted mutates each of the three fields separately and fails unless ReceiveHello returns that field's drop reason and no neighbor is created (processing stopped). Other polarity: TestRFC2328HelloMatchingParametersAndTwoWay fails if a matching Hello is refused. Every listed field has its own case.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328HelloMismatchRejectedAndNoTwoWayUnlisted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc2328_hello_test.go#L35) | unit/verify | revert, verified |
| positive | [`TestRFC2328HelloMatchingParametersAndTwoWay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc2328_hello_test.go#L16) | unit/verify | revert, verified |

### [`RFC2328-10.6-1`](#rfc2328-10.6-1)

In states Loading and Full the slave must resend its last Database Description packet in response to duplicate Database Description packets received from the master. For this reason the slave must wait RouterDeadInterval seconds before freeing the last Database Description packet. (§10.8)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the slave repeats its last DD for a duplicate right after the first exchange; no unit exercises the duplicate in Loading or Full, nor the RouterDeadInterval retention of the last DD.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328SlaveRefusesOutOfSequenceDD`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_dd_slave_test.go#L62) | unit/verify | revert, verified |
| positive | [`TestRFC2328SlaveRepliesAndRepeatsOnDuplicate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_dd_slave_test.go#L30) | unit/verify | revert, verified |

### [`RFC2328-12.1.6-1`](#rfc2328-12.1.6-1)

When an attempt is made to increment the sequence number past the maximum value of N - 1 (0x7fffffff; also referred to as MaxSequenceNumber), the current instance of the LSA must first be flushed from the routing domain. This is done by prematurely aging the LSA (see Section 14.1) and reflooding it. As soon as this flood has been acknowledged by all adjacent neighbors, a new instance can be originated with sequence number of InitialSequenceNumber. (§12.1.6)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clauses: (1) flush by premature aging AND reflooding, (2) wait until ALL adjacent neighbors acknowledge, (3) then originate at InitialSequenceNumber. TestRFC2328SequenceWrapWaitsForAck proves no InitialSequenceNumber with zero acks and TestRFC2328SequenceWrapRestartsAfterAck proves it after both neighbors ack. Not proven: no unit acks from only one of the two neighbors, so a producer that restarts after the first ack stays green; and neither unit asserts the MaxAge instance is reflooded (tx is set but never inspected), only that its age is MaxAge.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328SequenceWrapWaitsForAck`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L194) | unit/verify | revert, verified |
| positive | [`TestRFC2328SequenceWrapRestartsAfterAck`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L217) | unit/verify | revert, verified |

### [`RFC2328-12.2-1`](#rfc2328-12.2-1)

An implementation of OSPF must be able to access individual pieces of an area database. This lookup function is based on an LSA's LS type, Link State ID and Advertising Router. (§12.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: a lookup that ignores any element of the (LS type, Link State ID, Advertising Router) triple. TestRFC2328LookupByTriple proves the full triple finds the instance; TestRFC2328LookupNeedsFullTriple varies only the Advertising Router. A lookup that ignored LS type or Link State ID is not caught: no unit installs two LSAs differing only in type or only in Link State ID.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328LookupNeedsFullTriple`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L174) | unit/verify | revert, verified |
| positive | [`TestRFC2328LookupByTriple`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L160) | unit/verify | revert, verified |

### [`RFC2328-12.4-1`](#rfc2328-12.4-1)

If a router advertises a summary-LSA for a destination which then becomes unreachable, the router must then flush the LSA from the routing domain by setting its age to MaxAge and reflooding (see Section 14.1). (§12.4.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: keeping a self-originated summary-LSA for a destination that became unreachable. The units call FlushStaleSummaryLSAs directly with a hand-built keep set (empty vs containing the key) and prove MaxAge plus reflood vs no flush. Nothing makes a destination unreachable: the step from an unreachable route to its key leaving the keep set is not exercised, so a caller that kept unreachable destinations in the set stays green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328AdvertisedSummaryNotFlushed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L286) | unit/verify | revert, verified |
| positive | [`TestRFC2328WithdrawnSummaryFlushedAtMaxAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L249) | unit/verify | revert, verified |

### [`RFC2328-12.4.1-1`](#rfc2328-12.4.1-1)

All of the router's links to the area must be described in a single router-LSA. (§A.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: splitting the area's links over several router-LSAs, or omitting one. Red: TestRFC2328SingleRouterLSACarriesEveryLink fails unless exactly one router-LSA from 1.1.1.1 exists in the area and its body holds a transit link for each of the two interfaces. Other polarity: TestRFC2328RouterLSAExcludesOtherAreaLinks fails if an interface of area 0.0.0.1 is described in the backbone router-LSA, so 'links to the area' is bounded. One clause, both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328RouterLSAExcludesOtherAreaLinks`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L350) | unit/verify | revert, verified |
| positive | [`TestRFC2328SingleRouterLSACarriesEveryLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L318) | unit/verify | revert, verified |

### [`RFC2328-12.4.3-1`](#rfc2328-12.4.3-1)

The router must then originate summary-LSAs into the newly attached area for all pertinent intra-area and inter-area routes in the router's routing table. (§12.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. OriginateSummaries is called directly with a fixed topology: nothing ties origination to the router becoming newly attached to the area, and only intra-area routes are exercised, not inter-area ones.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328ABRComponentNeverEscapesRange`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_abr_summary_test.go#L101) | unit/verify | revert, verified |
| positive | [`TestRFC2328ABRSummarizesAndCondenses`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_abr_summary_test.go#L67) | unit/verify | revert, verified |

### [`RFC2328-13-5`](#rfc2328-13-5)

Remove the current database copy from all neighbors' Link state retransmission lists. (§13)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: leaving the replaced database copy on a neighbor retransmission list. TestRFC2328ReplacedInstanceLeavesRetransmitLists checks only eth1, the interface the newer instance is flooded to, and the list is a map keyed by LSAKey (retransmit[nbr][key], flooding.go queueRetransmit), so the re-flood overwrites the entry whether or not removeFromAllRetransmit runs. No unit checks a neighbor that does not receive the new instance (e.g. an entry on the receiving interface's list), so dropping the removal leaves both units green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328DuplicateKeepsOtherRetransmitLists`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L144) | unit/verify | revert, verified |
| positive | [`TestRFC2328ReplacedInstanceLeavesRetransmitLists`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L110) | unit/verify | revert, verified |

### [`RFC2328-13-6`](#rfc2328-13-6)

If the contents are different, the following pieces of the routing table must be recalculated, depending on the new LSA's LS type field: Router-LSAs and network-LSAs The entire routing table must be recalculated, starting with the shortest path calculations for each area (not just the area whose link-state database has changed). (§13.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the tagged units start from TriggerArea and show every area is recalculated; no unit installs an LSA with changed contents to trigger it, nor shows that unchanged contents trigger nothing, nor the summary/AS-external best-route recalculation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSPFRecalculationBoundedToConfiguredAreas`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_recalc_test.go#L47) | unit/verify | revert, verified |
| positive | [`TestSPFRecalculatesEveryAreaOnOneAreaChange`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_recalc_test.go#L14) | unit/verify | revert, verified |

### [`RFC2328-13-7`](#rfc2328-13-7)

If there is already a database copy, and if the database copy was received via flooding and installed less than MinLSArrival seconds ago, discard the new LSA (without acknowledging it) and examine the next LSA (if any) listed in the Link State Update packet. (§13)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clauses: discard a newer instance arriving within MinLSArrival of a flooded install, without acknowledging it, AND go on to examine the next LSA in the Link State Update. The first two are proven: TestRFC2328MinLSArrivalDiscardsWithoutAck fails if the sequence changes, any packet is sent, or the later delayed ack names the discarded instance; TestRFC2328MinLSArrivalElapsedAccepts fails if an instance after MinLSArrival is not installed and acked. Not proven: every update carries a single LSA, so a receiver that abandoned the rest of the update after a too-soon discard stays green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328MinLSArrivalDiscardsWithoutAck`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L44) | unit/verify | revert, verified |
| positive | [`TestRFC2328MinLSArrivalElapsedAccepts`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L79) | unit/verify | revert, verified |

### [`RFC2328-13.3-3`](#rfc2328-13.3-3)

On non-broadcast networks, separate Link State Update packets must be sent, as unicasts, to each adjacent neighbor (i.e., those in state Exchange or greater). (§13.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: on a non-broadcast network, flooding by multicast or omitting a unicast to an adjacent (Exchange or greater) neighbor. Red: TestRFC2328NBMAUnicastsToEachAdjacency fails unless exactly one LS Update goes to each of 10.0.0.2 (Full) and 10.0.0.3 (Exchange); TestRFC2328NBMANeverMulticastsNorReachesTwoWay fails on any send to AllSPFRouters or AllDRouters, or to the 2-Way neighbor 10.0.0.4. Both polarities over floodExcept, the flooding producer.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328NBMANeverMulticastsNorReachesTwoWay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L464) | unit/verify | revert, verified |
| positive | [`TestRFC2328NBMAUnicastsToEachAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L430) | unit/verify | revert, verified |

### [`RFC2328-15-2`](#rfc2328-15-2)

The cost of a virtual link is NOT configured. It is defined to be the cost of the intra-area path between the two defining area border routers. This cost appears in the virtual link's corresponding routing table entry. When the cost of a virtual link changes, a new router-LSA should be originated for the backbone area. (§15)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clauses: the cost is not configured but is the intra-area path cost; it appears in the virtual link's routing table entry; a cost change originates a new backbone router-LSA. TestRFC2328VirtualCostChangeReoriginatesBackbone proves the injected result.Cost (77) reaches the backbone router-LSA metric, and TestRFC2328VirtualUnusablePathWithdrawsBackboneLink proves an unusable path is withdrawn (a neighbouring rule). Not proven: no unit derives the cost from an intra-area SPF path (the cost is handed in), no unit asserts the routing table entry carries it, and none checks that the change produced a NEW instance (sequence advance or flood) rather than only the stored metric.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328VirtualUnusablePathWithdrawsBackboneLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_virtual_route_test.go#L98) | unit/verify | unproven |
| positive | [`TestRFC2328VirtualCostChangeReoriginatesBackbone`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_virtual_route_test.go#L67) | unit/verify | unproven |

### [`RFC2328-16.1-3`](#rfc2328-16.1-3)

Note that when there is a choice of vertices closest to the root, network vertices must be chosen before router vertices in order to necessarily find all equal-cost paths. (§16.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: choosing a router vertex before a network vertex at equal distance. Red: TestRFC2328EqualDistanceNetworkVertexFirst pushes two routers and one network at dist 10 and fails unless the first pop is the network vertex. Other polarity: TestRFC2328CloserRouterVertexBeforeFartherNetwork fails if the network-first preference overrides distance (router at 10 must pop before network at 11). Both over spfHeap, the SPF candidate list itself.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328CloserRouterVertexBeforeFartherNetwork`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_tiebreak_test.go#L25) | unit/verify | revert, verified |
| positive | [`TestRFC2328EqualDistanceNetworkVertexFirst`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_tiebreak_test.go#L10) | unit/verify | revert, verified |

### [`RFC2328-A.1-1`](#rfc2328-a.1-1)

Packets sent to these multicast addresses should never be forwarded; they are meant to travel a single hop only. To ensure that these packets will not travel multiple hops, their IP TTL must be set to 1. (§A.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: OSPF packets sent with a TTL other than 1. Red: TestOSPFSocketTTLIsOne first proves the kernel default is not 1, then fails unless IP_TTL and IP_MULTICAST_TTL both read back 1 after setMulticastOptions (which hard-codes 1, backend_linux.go:307-316). Other polarity: TestOSPFSocketTTLFailureIsAnError fails if a setsockopt failure is swallowed, so a socket cannot enter service with the default TTL.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFSocketTTLFailureIsAnError`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_socket_linux_test.go#L58) | unit/verify | revert, verified |
| positive | [`TestOSPFSocketTTLIsOne`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_socket_linux_test.go#L30) | unit/verify | revert, verified |

### [`RFC2328-A.1-2`](#rfc2328-a.1-2)

Both the Designated Router and Backup Designated Router must be prepared to receive packets destined to this address. (§A.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the positive shows only an elected Backup joining AllDRouters; the Designated Router join is not asserted.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFDROtherDoesNotJoinAllDRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc2328_dr_test.go#L46) | unit/verify | revert, verified |
| positive | [`TestOSPFElectedBDRJoinsAllDRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc2328_dr_test.go#L29) | unit/verify | revert, verified |

### [`RFC2328-A.4.4-1`](#rfc2328-a.4.4-1)

This field is not meaningful and must be zero for Type 4 summary-LSAs. (§A.4.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Sentence (A.4.4): the Network Mask 'must be zero for Type 4 summary-LSAs'. (a) Non-compliant behaviour: originating a Type 4 summary-LSA with a non-zero Network Mask. (b) TestRFC2328Type4SummaryMaskIsZero originates a Type 4 through OriginateSummary (origination.go, the only Type 4 producer) with mask 255.255.255.0 and fails on body.NetworkMask != 0.0.0.0 after decoding the stored LSA, so a producer that copied the caller's mask goes red. Negative: TestRFC2328Type3SummaryKeepsMask proves the zeroing is confined to Type 4 (a Type 3 keeps 255.255.255.0), so a producer that zeroed every summary mask goes red there. One clause, both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328Type3SummaryKeepsMask`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L509) | unit/verify | revert, verified |
| positive | [`TestRFC2328Type4SummaryMaskIsZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L490) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-27 |
| Register | prose |
| Source | rfc/full/rfc2328.txt |
| Source fingerprint | e2e865f4d247d1bf |
| Record | rfc/extraction/rfc2328.json |
| Mapped sentences | 136 |
| Declined as scope | 42 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 3 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 9 | walked | not stated |
| `4` | not stated | 9 | walked | not stated |
| `5` | not stated | 2 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 15 | walked | not stated |
| `9` | not stated | 17 | walked | not stated |
| `10` | not stated | 16 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `12` | not stated | 14 | walked | not stated |
| `13` | not stated | 29 | walked | not stated |
| `14` | not stated | 3 | walked | not stated |
| `15` | not stated | 2 | walked | not stated |
| `16` | not stated | 21 | walked | not stated |
| `A` | not stated | 0 | walked | not stated |
| `A.1` | Appendix subsection A.1 | 2 | walked | Appendix subsection A.1. Until 2026-09-27 the heading reader did not read a column-0 appendix subsection heading written without a trailing dot, so its text was read as part of section A, where its 2 site(s) were walked; every decision is carried forward by its verbatim quote. |
| `A.2` | Appendix subsection A.2 | 0 | walked | Appendix subsection A.2. Until 2026-09-27 the heading reader did not read a column-0 appendix subsection heading written without a trailing dot, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `A.3` | Appendix subsection A.3 | 0 | walked | Appendix subsection A.3. Until 2026-09-27 the heading reader did not read a column-0 appendix subsection heading written without a trailing dot, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `A.3.1` | Appendix subsection A.3.1 | 0 | walked | Appendix subsection A.3.1. Until 2026-09-27 the heading reader did not read a column-0 appendix subsection heading written without a trailing dot, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `A.3.2` | Appendix subsection A.3.2 | 1 | walked | Appendix subsection A.3.2. Until 2026-09-27 the heading reader did not read a column-0 appendix subsection heading written without a trailing dot, so its text was read as part of section A, where its 1 site(s) were walked; every decision is carried forward by its verbatim quote. |
| `A.3.3` | Appendix subsection A.3.3 | 1 | walked | Appendix subsection A.3.3. Until 2026-09-27 the heading reader did not read a column-0 appendix subsection heading written without a trailing dot, so its text was read as part of section A, where its 1 site(s) were walked; every decision is carried forward by its verbatim quote. |
| `A.3.4` | Appendix subsection A.3.4 | 0 | walked | Appendix subsection A.3.4. Until 2026-09-27 the heading reader did not read a column-0 appendix subsection heading written without a trailing dot, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `A.3.5` | Appendix subsection A.3.5 | 0 | walked | Appendix subsection A.3.5. Until 2026-09-27 the heading reader did not read a column-0 appendix subsection heading written without a trailing dot, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `A.3.6` | Appendix subsection A.3.6 | 1 | walked | Appendix subsection A.3.6. Until 2026-09-27 the heading reader did not read a column-0 appendix subsection heading written without a trailing dot, so its text was read as part of section A, where its 1 site(s) were walked; every decision is carried forward by its verbatim quote. |
| `A.4` | Appendix subsection A.4 | 0 | walked | Appendix subsection A.4. Until 2026-09-27 the heading reader did not read a column-0 appendix subsection heading written without a trailing dot, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `A.4.1` | Appendix subsection A.4.1 | 0 | walked | Appendix subsection A.4.1. Until 2026-09-27 the heading reader did not read a column-0 appendix subsection heading written without a trailing dot, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `A.4.2` | Appendix subsection A.4.2 | 3 | walked | Appendix subsection A.4.2. Until 2026-09-27 the heading reader did not read a column-0 appendix subsection heading written without a trailing dot, so its text was read as part of section A, where its 3 site(s) were walked; every decision is carried forward by its verbatim quote. |
| `A.4.3` | Appendix subsection A.4.3 | 0 | walked | Appendix subsection A.4.3. Until 2026-09-27 the heading reader did not read a column-0 appendix subsection heading written without a trailing dot, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `A.4.4` | Appendix subsection A.4.4 | 1 | walked | Appendix subsection A.4.4. Until 2026-09-27 the heading reader did not read a column-0 appendix subsection heading written without a trailing dot, so its text was read as part of section A, where its 1 site(s) were walked; every decision is carried forward by its verbatim quote. |
| `A.4.5` | Appendix subsection A.4.5 | 0 | walked | Appendix subsection A.4.5. Until 2026-09-27 the heading reader did not read a column-0 appendix subsection heading written without a trailing dot, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `B` | not stated | 1 | walked | not stated |
| `C` | not stated | 16 | walked | not stated |
| `D` | not stated | 9 | walked | not stated |
| `E` | not stated | 0 | walked | not stated |
| `F` | not stated | 0 | walked | not stated |
| `G` | not stated | 3 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | non-normative description of the Designated Router concept's effect on the number of adjacencies; no obligation is stated | The Designated Router concept enables a reduction in the number of adjacencies required on a broadcast or NBMA network. |
| `1:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | non-normative history of the protocol's development; 'enhanced' prose, no obligation | The Designated Router concept has been greatly enhanced to further reduce the amount of routing traffic required. |
| `1:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | non-normative document convention: it states that the memo's data structures are illustrative, imposing no specific protocol behavior | Implementations of the protocol are required to support the functionality described, but need not use the precise data structures that appear in this memo. |
| `3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | constraint on the AS topology a network operator builds (the backbone must be contiguous), not a behavior the router produces | The backbone must be contiguous. |
| `3:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | narrative about Figure 6's sample AS, naming RT3 and RT4; an example, not an obligation | Also, RT3 and RT4 must advertise into Area 1 the location of the AS boundary routers RT5 and RT7. |
| `3:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | figure text for the sample AS of Figure 6, garbled by the extractor; an example, not an obligation | Routers RT3 and RT4 must also summarize Area 1's topology for ........................... . + . . \| 3+---+ . |
| `3:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | constraint on the operator's addressing plan (mask assignment), not a behavior the router produces | Subnet masks must be assigned so that the best match for any IP destination is unambiguous. |
| `3:5` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | statement of what an operator must do to use stub areas (configure default routing), not a router behavior | In order to take advantage of the OSPF stub area support, default routing must be used in the stub area. |
| `3:7` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | example reading 'For example, Area 3 in Figure 6 could be configured as a stub area'; an illustration, not an obligation | For example, Area 3 in Figure 6 could be configured as a stub area, because all external traffic must travel though its single area border router RT11. |
| `3:8` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | constraint on the operator's address-range plan across an area partition, not a router behavior | However, in order to maintain full routing after the partition, an address range must not be split across multiple components of the area partition. |
| `3:9` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | constraint on the AS topology an operator builds (the backbone must not partition), not a router behavior | Also, the backbone itself must not partition. |
| `4:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | non-normative rationale for flooding AS-external information throughout the AS; no discrete behavior is required | To utilize external routing information, the path to all routers advertising external information must be known throughout the AS (excepting the stub areas). |
| `4:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | describes the implementation's internal timer facilities (single-shot and interval timers), not a protocol behavior | Timers Two different kind of timers are required. |
| `4:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | states a NON-requirement: 'the ability to forward IP multicast datagrams is not required' | For this reason, the ability to forward IP multicast datagrams is not required. |
| `4:8` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | introductory sentence to the area-capability discussion; the obligation it introduces is the E-bit consistency stated in the next sentence | Some capabilities must be supported by all routers attached to a specific area. |
| `9:5` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | glossary entry for the Wait Timer in the interface data structure's timer list; it names the timer, it states no behavior | WaitTimer The Wait Timer has fired, indicating the end of the waiting period that is required before electing a (Backup) Designated Router. |
| `9:7` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | explains how the interface state machine table is read ('Each entry ... describes the resulting new interface state'); document convention, not an obligation | Each entry in the state machine describes the resulting new interface state and the required set of additional actions. |
| `9:8` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | explains that state machine actions generate neighbor events; document convention, not an obligation | Some of the required actions below involve generating events for the neighbor state machine. |
| `9:9` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | illustrative example opening 'For example, when an interface becomes inoperative'; the normative action is the state machine entry it illustrates | For example, when an interface becomes inoperative, all neighbor connections associated with the interface must be destroyed. |
| `9:11` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | describes the meaning of the RouterDeadInterval field carried in the Hello packet; no behavior is required by this sentence | The Hello Packet also indicates how often a neighbor must be heard from to remain active (RouterDeadInterval). |
| `10:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | introductory pointer to Section 10.4 ('whether to become adjacent'); it names where the decision is specified, it does not state it | A decision must be made as to whether an adjacency should be established/maintained with the neighbor. |
| `10:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | explains how the neighbor state machine table is read; document convention, not an obligation | Each entry in the state machine describes the resulting new neighbor state and the required set of additional actions. |
| `10:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | state machine table entry reading 'No other action is required'; it states the absence of an action | No other action is required. |
| `10:5` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | state machine table entry reading 'No action required'; it states the absence of an action | Action: No action required. |
| `10:7` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | state machine table entry reading 'No action required'; it states the absence of an action | Action: No action required. |
| `10:8` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | state machine table entry reading 'No action required'; it states the absence of an action | Action: No action required. |
| `12:13` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | narrative about the sample routers RTA and RTX in the Section 12.4.4 example; an illustration, not an obligation | RTA must then originate AS- external-LSAs for those destinations it has learned from RTX. |
| `12:14` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | design note on the configuration of two routers advertising the same external route; it requires an unambiguous configuration, not a behavior the router produces | However, it must be unambiguously defined as to which router originates the LSAs (otherwise neither may, or the identity of the originator may oscillate). |
| `15:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | constraint on the operator's configuration of a virtual link (both endpoints configure it); the router cannot observe the other end's configuration | The virtual link must be configured in both routers. |
| `16:6` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | states a NON-requirement: 'no next hop IP address is required' for a directly connected destination | If the destination is a directly connected network, or a router which connects to the calculating router via a point-to-point interface, no next hop IP address is required. |
| `A.3.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | describes what the LSA header contains ('all the information required to uniquely identify the LSA'); a field description, not an obligation | It contains all the information required to uniquely identify both the LSA and the LSA's current instance. |
| `A.3.6:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | describes what the LSA header contains ('all the information required to uniquely identify the LSA'); a field description, not an obligation | It contains all the information required to uniquely identify both the LSA and the LSA's current instance. |
| `A.4.2:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | describes the # TOS field's meaning, counting metrics 'not counting the required link metric'; a field description, not an obligation | See Section 16.1.1 for more details. # TOS The number of different TOS metrics given for this link, not counting the required link metric (referred to as the TOS 0 metric in [Ref9]). |
| `C:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | statement that the operator configures an area's parameters consistently on all its routers; a configuration constraint, not a behavior the router produces | All routers belonging to an area must agree on that area's configuration. |
| `C:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | preamble to the list of an area's configurable items ('The following items must be configured for an area:'); a configuration inventory | The following items must be configured for an area: |
| `C:5` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | preamble to the list of a router interface's configurable parameters; a configuration inventory | The parameters that must be configured for a router interface are: IP interface address The IP protocol address for this interface. |
| `C:6` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | states a NON-requirement: 'An IP address is not required on point-to-point networks' | An IP address is not required on point-to-point networks. |
| `C:13` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | constraint on the operator's configuration of a virtual link (both area border routers configure it); the router cannot observe the other end's configuration | The virtual link must be configured in both of the area border routers. |
| `C:14` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | entry in the interface configuration inventory naming RxmtInterval, with a sizing recommendation; the protocol behavior that uses it is stated in Section 13.3 | The parameter RxmtInterval must be configured, and should be well over the expected round-trip delay between the two routers. |
| `C:15` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | entry in the NBMA configuration inventory: the operator defines each listed router's Designated Router eligibility | Also, for each router listed, that router's eligibility to become Designated Router must be defined. |
| `C:16` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | preamble to the list of a directly connected host's configurable items; a configuration inventory | For each host directly connected to the router, the following items must be configured: |
| `G:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | ISOC copyright boilerplate on modification and translation of the document; it states no protocol obligation | However, this document itself may not be modified in any way, such as by removing the copyright notice or references to the Internet Society or other Internet organizations, except as needed for the purpose of developing Internet standards in which case the procedures for copyrights defined in the Internet Standards process must be followed, or as required to translate it into languages other than English. |

## Superseded

No document obsoletes RFC 2328, so its obligations are stated where they were written.
