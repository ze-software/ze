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
| Partial proof; remaining gap | 0.0% | 0 of 52 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 65.3% | 115 of 176 tagged units, 2 escaped and 0 lapsed | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

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
| Audit verdicts | 52 | of 52 gated MUSTs judged | 1 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 52 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Test tags | 178 |
| Tagged units | 176 |
| Recorded audit verdicts | 52 |
| Discrimination records | 117 |
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

The 2026-09-21 extraction added obligations that require current positive/negative coverage and discrimination. New IP-envelope, source-address, classless-route and virtual-link carriers have been added; current runner proof and deployment evidence remain pending.

- **One MUST is unmet:** unnumbered point-to-point interfaces are not implemented, so an OSPF interface needs its own IPv4 address ([`RFC2328-8.1-1`](#rfc2328-8.1-1), gap).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 50 | one part of the gated population |
| Annotated (including scoped evidence) | 2 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **52** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (50):** [`RFC2328-A.3.1-1`](#rfc2328-a.3.1-1), [`RFC2328-A.3.1-2`](#rfc2328-a.3.1-2), [`RFC2328-12.1.7-1`](#rfc2328-12.1.7-1), [`RFC2328-13-1`](#rfc2328-13-1), [`RFC2328-13-2`](#rfc2328-13-2), [`RFC2328-13-3`](#rfc2328-13-3), [`RFC2328-13.1-1`](#rfc2328-13.1-1), [`RFC2328-13-4`](#rfc2328-13-4), [`RFC2328-13.3-1`](#rfc2328-13.3-1), [`RFC2328-13.3-2`](#rfc2328-13.3-2), [`RFC2328-14-1`](#rfc2328-14-1), [`RFC2328-14-2`](#rfc2328-14-2), [`RFC2328-13.5-1`](#rfc2328-13.5-1), [`RFC2328-13.4-1`](#rfc2328-13.4-1), [`RFC2328-16.1-1`](#rfc2328-16.1-1), [`RFC2328-16.4-1`](#rfc2328-16.4-1), [`RFC2328-16.2-1`](#rfc2328-16.2-1), [`RFC2328-16.2-2`](#rfc2328-16.2-2), [`RFC2328-D.2-1`](#rfc2328-d.2-1), [`RFC2328-D.3-1`](#rfc2328-d.3-1), [`RFC2328-D.3-2`](#rfc2328-d.3-2), [`RFC2328-A.3.3-1`](#rfc2328-a.3.3-1), [`RFC2328-10.1-1`](#rfc2328-10.1-1), [`RFC2328-10.2-1`](#rfc2328-10.2-1), [`RFC2328-C.3-1`](#rfc2328-c.3-1), [`RFC2328-3.6-1`](#rfc2328-3.6-1), [`RFC2328-4.4-1`](#rfc2328-4.4-1), [`RFC2328-4.4-2`](#rfc2328-4.4-2), [`RFC2328-4.4-3`](#rfc2328-4.4-3), [`RFC2328-8.2-1`](#rfc2328-8.2-1), [`RFC2328-8.2-2`](#rfc2328-8.2-2), [`RFC2328-8.2-3`](#rfc2328-8.2-3), [`RFC2328-9.1-1`](#rfc2328-9.1-1), [`RFC2328-9.5.1-1`](#rfc2328-9.5.1-1), [`RFC2328-10.5-1`](#rfc2328-10.5-1), [`RFC2328-10.6-1`](#rfc2328-10.6-1), [`RFC2328-12.1.6-1`](#rfc2328-12.1.6-1), [`RFC2328-12.2-1`](#rfc2328-12.2-1), [`RFC2328-12.4-1`](#rfc2328-12.4-1), [`RFC2328-12.4.1-1`](#rfc2328-12.4.1-1), [`RFC2328-12.4.3-1`](#rfc2328-12.4.3-1), [`RFC2328-13-5`](#rfc2328-13-5), [`RFC2328-13-6`](#rfc2328-13-6), [`RFC2328-13-7`](#rfc2328-13-7), [`RFC2328-13.3-3`](#rfc2328-13.3-3), [`RFC2328-15-2`](#rfc2328-15-2), [`RFC2328-16.1-3`](#rfc2328-16.1-3), [`RFC2328-A.1-1`](#rfc2328-a.1-1), [`RFC2328-A.1-2`](#rfc2328-a.1-2), [`RFC2328-A.4.4-1`](#rfc2328-a.4.4-1)

**Annotated (including scoped evidence) (2):** [`RFC2328-D.4.3-1`](#rfc2328-d.4.3-1), [`RFC2328-8.1-1`](#rfc2328-8.1-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2328-A.3.1-1` | The version number field must specify protocol version 2. (§8.2) | MUST | 8.2 | **positive:** `unit/verify` [`TestOSPFHeaderRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc2328_header_test.go#L131). **negative:** `unit/verify` [`TestOSPFHeaderRejectsBadVersionAndLength`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc2328_header_test.go#L153) |
| `RFC2328-A.3.1-2` | The standard IP checksum of the entire contents of the packet, starting with the OSPF packet header but excluding the 64-bit authentication field. (§A.3.1) | MUST | A.3.1 - Appendix subsection A.3.1 | **positive:** `unit/verify` [`TestOSPFPacketChecksumExcludesAuth`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L30). **positive:** `unit/verify` [`TestRFC2328PacketChecksumCoversHeaderExcludesAuth`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc2328_checksum_coverage_test.go#L31). **negative:** `unit/verify` [`TestPacketVerifyChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc2328_header_test.go#L100). **negative:** `unit/verify` [`TestRFC2328PacketChecksumWrongRangeRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc2328_checksum_coverage_test.go#L61) |
| `RFC2328-12.1.7-1` | This field is the checksum of the complete contents of the LSA, excepting the LS age field. The LS age field is excepted so that an LSA's age can be incremented without updating the checksum. The checksum used is the same that is used for ISO connectionless datagrams; it is commonly referred to as the Fletcher checksum. It is documented in Annex B of [Ref6]. The LSA header also contains the length of the LSA in bytes; subtracting the size of the LS age field (two bytes) yields the amount of data to checksum. The checksum is used to detect data corruption of an LSA. This corruption can occur while an LSA is being flooded, or while it is being held in a router's memory. The LS checksum field cannot take on the value of zero; the occurrence of such a value should be considered a checksum failure. In other words, calculation of the checksum is not optional. (§12.1.7) | MUST | 12.1.7 | **positive:** `unit/verify` [`TestFletcherIgnoresLSAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L46). **positive:** `unit/verify` [`TestFletcherRFC905Vectors`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L26). **positive:** `unit/verify` [`TestOSPFLSAChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L72). **positive:** `unit/verify` [`TestOSPFLSAChecksumExcludesAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L98). **negative:** `unit/verify` [`TestOSPFLSAChecksumExcludesAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L96). **negative:** `unit/verify` [`TestRFC2328ZeroLSChecksumRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc2328_test.go#L10) |
| `RFC2328-13-1` | Validate the LSA's LS checksum. If the checksum turns out to be invalid, discard the LSA and get the next one from the Link State Update packet. (2) Examine the LSA's LS type. If the LS type is unknown, discard the LSA and get the next one from the Link State Update Packet. (§13) | MUST | 13 | **positive:** `unit/verify` [`TestOSPFFloodOutOtherInterfaces`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_decisions_test.go#L30). **positive:** `unit/verify` [`TestRFC2328DiscardedLSAThenNextLSAProcessed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_lsupdate_discard_next_test.go#L92). **negative:** `unit/verify` [`TestRFC2328BadLSChecksumDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_test.go#L17). **negative:** `unit/verify` [`TestRFC2328DiscardedLSAThenNextLSAProcessed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_lsupdate_discard_next_test.go#L91) |
| `RFC2328-13-2` | AS-external-LSAs are not flooded into/throughout stub areas (§13) | MUST NOT | 13 | **positive:** `unit/verify` [`TestOSPFStubFloodFilter`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_area_type_test.go#L15). **positive:** `unit/verify` [`TestRFC2328ASExternalNotFloodedIntoStubArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_stub_external_flood_test.go#L63). **negative:** `unit/verify` [`TestOSPFStubAreaDropsType5`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_decisions_test.go#L248). **negative:** `unit/verify` [`TestRFC2328ASExternalFromStubInterfaceGoesNowhere`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_stub_external_flood_test.go#L112) |
| `RFC2328-13-3` | If the neighbor is in a lesser state than Exchange, the packet should be dropped without further processing. (§13) | MUST | 13 | **positive:** `unit/verify` [`TestRFC2328FloodingRequiresExchangeOrHigher`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L17). **positive:** `unit/verify` [`TestRFC2328LSUpdateFromExchangeNeighborProcessed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_flood_state_gate_test.go#L120). **negative:** `unit/verify` [`TestRFC2328FloodingRequiresExchangeOrHigher`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L20). **negative:** `unit/verify` [`TestRFC2328LSUpdateBelowExchangeDroppedByEngine`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_flood_state_gate_test.go#L83) |
| `RFC2328-13.1-1` | The LSA having the newer LS sequence number is more recent. See Section 12.1.6 for an explanation of the LS sequence number space. If both instances have the same LS sequence number, then: o If the two instances have different LS checksums, then the instance having the larger LS checksum (when considered as a 16-bit unsigned integer) is considered more recent. o Else, if only one of the instances has its LS age field set to MaxAge, the instance of age MaxAge is considered to be more recent. o Else, if the LS age fields of the two instances differ by more than MaxAgeDiff, the instance having the smaller (younger) LS age is considered to be more recent. (§13.1) | MUST | 13.1 | **positive:** `unit/verify` [`TestOSPFFreshnessCompareMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_lsdb_test.go#L126). **positive:** `unit/verify` [`TestRFC2328FreshnessStepsAppliedInOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_freshness_order_test.go#L17). **negative:** `unit/verify` [`TestRFC2328LessRecentByEarlierStepRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_freshness_order_test.go#L55). **negative:** `unit/verify` [`TestRFC2328OlderInstanceGetsDatabaseCopyBack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_test.go#L51) |
| `RFC2328-13-4` | If the database copy has LS age equal to MaxAge and LS sequence number equal to MaxSequenceNumber, simply discard the received LSA without acknowledging it. (§13) | MUST | 13 | **positive:** `unit/verify` [`TestOSPFMaxSeqMaxAgeSilentDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_edges_test.go#L19). **negative:** `unit/verify` [`TestRFC2328OlderInstanceGetsDatabaseCopyBack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_test.go#L54) |
| `RFC2328-13.3-1` | LSAs flooded out an adjacency are placed on the adjacency's Link state retransmission list. In order to ensure that flooding is reliable, these LSAs are retransmitted until they are acknowledged. The length of time between retransmissions is a configurable per-interface value, RxmtInterval. (§13.6) | MUST | 13.6 | **positive:** `unit/verify` [`TestOSPFFloodOutOtherInterfaces`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_decisions_test.go#L31). **positive:** `unit/verify` [`TestOSPFRetransmitTimer`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_decisions_test.go#L129). **positive:** `unit/verify` [`TestRFC2328RetransmitEveryConfiguredRxmtInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_retransmit_interval_test.go#L14). **negative:** `unit/verify` [`TestOSPFFloodQueuesExchangeAndLoadingNeighbors`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_decisions_test.go#L57). **negative:** `unit/verify` [`TestRFC2328RetransmitEveryConfiguredRxmtInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_retransmit_interval_test.go#L19) |
| `RFC2328-13.3-2` | The LSA's LS age must be incremented by InfTransDelay (which must be > 0) when it is copied into the outgoing Link State Update packet (until the LS age field reaches the maximum value of MaxAge). (§13.3) | MUST | 13.3 | **positive:** `unit/verify` [`TestRFC2328FloodIncrementsAgeByInfTransDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flood_age_test.go#L90). **positive:** `unit/verify` [`TestRFC2328TransmitDelayMustBePositive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_transmit_delay_test.go#L11). **negative:** `unit/verify` [`TestRFC2328FloodIncrementsAgeByInfTransDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flood_age_test.go#L106). **negative:** `unit/verify` [`TestRFC2328TransmitDelayMustBePositive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_transmit_delay_test.go#L14) |
| `RFC2328-14-1` | An LSA's LS age is never incremented past the value MaxAge. LSAs having age MaxAge are not used in the routing table calculation. (§14) | MUST | 14 | **positive:** `unit/verify` [`TestLSAgeAddSaturates`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/rfc2328_lsage_test.go#L39). **positive:** `unit/verify` [`TestOSPFLSDBAgeToPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_aging_test.go#L34). **negative:** `unit/verify` [`TestOSPFGraphSkipsMaxAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_graph_test.go#L40) |
| `RFC2328-14-2` | A MaxAge LSA must be removed immediately from the router's link state database as soon as both a) it is no longer contained on any neighbor Link state retransmission lists and b) none of the router's neighbors are in states Exchange or Loading. (§14) | MUST | 14 | **positive:** `unit/verify` [`TestOSPFASExternalPurgeRetainedAcrossAreas`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_decisions_test.go#L333). **positive:** `unit/verify` [`TestRFC2328MaxAgeRemovedOnceNoNeighborExchangingOrLoading`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_maxage_exchange_test.go#L47). **negative:** `unit/verify` [`TestOSPFPurgeRetainedForExchangeOrLoading`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_decisions_test.go#L384). **negative:** `unit/verify` [`TestRFC2328MaxAgeKeptWhileNeighborInExchangeOrLoading`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_maxage_exchange_test.go#L36) |
| `RFC2328-13.5-1` | Each newly received LSA must be acknowledged. This is usually done by sending Link State Acknowledgment packets. However, acknowledgments can also be accomplished implicitly by sending Link State Update packets (see step 7a of Section 13). (§13.5) | MUST | 13.5 | **positive:** `unit/verify` [`TestOSPFAckDecisionTable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_decisions_test.go#L93). **positive:** `unit/verify` [`TestOSPFUnknownMaxAgeNoCopyIsAckedAndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_decisions_test.go#L361). **positive:** `unit/verify` [`TestRFC2328NewerLSAAcknowledgedThroughReceiveUpdate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_newer_lsa_ack_test.go#L17). **negative:** `unit/verify` [`TestOSPFDRRefloodsBackOutReceivingInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_edges_test.go#L80) |
| `RFC2328-13.4-1` | A self-originated LSA is detected when either 1) the LSA's Advertising Router is equal to the router's own Router ID or 2) the LSA is a network- LSA and its Link State ID is equal to one of the router's own IP interface addresses. (§13.4) | MUST | 13.4 | **positive:** `unit/verify` [`TestOSPFOriginateSelfReceivedHigherSeq`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_origination_test.go#L426). **positive:** `unit/verify` [`TestRFC2328SelfOriginatedDetectedByRouterIDOrInterfaceAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_self_detect_test.go#L60). **negative:** `unit/verify` [`TestOSPFSelfOriginatedNoLocalCopyFlush`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_edges_test.go#L43). **negative:** `unit/verify` [`TestRFC2328ForeignLSANotTakenAsSelfOriginated`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_self_detect_test.go#L83) |
| `RFC2328-16.1-1` | Look up the vertex W's LSA (router-LSA or network-LSA) in Area A's link state database. If the LSA does not exist, or its LS age is equal to MaxAge, or it does not have a link back to vertex V, examine the next link in V's LSA. (§16.1) | MUST | 16.1 | **positive:** `unit/verify` [`TestOSPFSPFShortestPath`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_spf_test.go#L13). **positive:** `unit/verify` [`TestRFC2328SPFExaminesNextLinkAfterSkip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_spf_vertex_lookup_test.go#L22). **negative:** `unit/verify` [`TestOSPFTwoWayCheck`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_spf_test.go#L31). **negative:** `unit/verify` [`TestRFC2328SPFSkipsMissingOrMaxAgeVertex`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_spf_vertex_lookup_test.go#L58) |
| `RFC2328-16.4-1` | Intra-area and inter-area paths are always preferred over AS external paths. (b) Type 1 external paths are always preferred over type 2 external paths. When all paths are type 2 external paths, the paths with the smallest advertised type 2 metric are always preferred. (§16.4) | MUST | 16.4 | **positive:** `unit/verify` [`TestOSPFRouteTablePreference`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_route_test.go#L8). **positive:** `unit/verify` [`TestRFC2328PreferredPathTypeSelected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_path_preference_test.go#L36). **negative:** `unit/verify` [`TestOSPFExternalE1PreferredOverE2`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_external_test.go#L83). **negative:** `unit/verify` [`TestRFC2328LowerPreferencePathRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_path_preference_test.go#L51) |
| `RFC2328-16.2-1` | If the router is attached to multiple areas (i.e., it is an area border router), only backbone summary-LSAs are examined. (§16) | MUST | 16 | **positive:** `unit/verify` [`TestOSPFInterAreaRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_interarea_test.go#L43). **positive:** `unit/verify` [`TestRFC2328NonABRUsesItsAreaSummaries`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_nonabr_summary_test.go#L15). **negative:** `unit/verify` [`TestOSPFABRBackboneOnlyAcceptance`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_interarea_test.go#L74) |
| `RFC2328-16.2-2` | If the cost specified by the LSA is LSInfinity, or if the LSA's LS age is equal to MaxAge, then examine the the next LSA. (2) If the LSA was originated by the calculating router itself, examine the next LSA. (§16.2) | MUST | 16.2 | **positive:** `unit/verify` [`TestOSPFInterAreaRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_interarea_test.go#L44). **positive:** `unit/verify` [`TestRFC2328SummaryAdvertisingLSInfinitySkipped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_summary_lsinfinity_test.go#L35). **negative:** `unit/verify` [`TestOSPFExternalLSInfinityDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_external_test.go#L61). **negative:** `unit/verify` [`TestOSPFInterAreaLSInfinityDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_interarea_test.go#L137). **negative:** `unit/verify` [`TestRFC2328ExternalSkipsMaxAgeAndSelf`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_test.go#L43). **negative:** `unit/verify` [`TestRFC2328InterAreaSkipsMaxAgeAndSelfSummary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_test.go#L20). **negative:** `unit/verify` [`TestRFC2328SummaryAdvertisingLSInfinitySkipped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_summary_lsinfinity_test.go#L40) |
| `RFC2328-D.2-1` | The 64-bit authentication field in the OSPF packet header must be equal to the 64-bit password (i.e., authentication key) that has been configured for the interface. (§D.5.2) | MUST | D.5.2 | **positive:** `unit/verify` [`TestRFC2328SimplePasswordMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_test.go#L95). **negative:** `unit/verify` [`TestRFC2328SimplePasswordMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_test.go#L99) |
| `RFC2328-D.3-1` | The message digest appended to the OSPF packet is not actually considered part of the OSPF protocol packet: the message digest is not included in the OSPF header's packet length, although it is included in the packet's IP header length field. (§D.3) | MUST | D.3 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L47). **positive:** `unit/verify` [`TestRFC2328DigestCountedInIPLengthNotPacketLength`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_digest_ip_length_test.go#L35). **negative:** `unit/verify` [`TestOSPFAuthCryptoRejectsExtraTrailerBytes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L219) |
| `RFC2328-D.3-2` | Whenever an OSPF packet is accepted as authentic, the cryptographic sequence number is set to the received packet's sequence number. (§D.3) | MUST | D.3 | **positive:** `unit/verify` [`TestNeighborDownResetsCryptoSeq`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L246). **positive:** `unit/verify` [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L116). **positive:** `unit/verify` [`TestOSPFAuthReplayEqualSequenceByAuType`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L173). **negative:** `unit/verify` [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L117) |
| `RFC2328-D.4.3-1` | The checksum field in the standard OSPF header is not calculated, but is instead set to 0. (§D.4.3) | MUST | D.4.3 | **positive:** `unit/verify` [`TestOSPFPacketChecksumZeroForAuType2`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L53). **negative:** no negative test. **{single-polarity}:** generate-side convention -- WriteTo leaves the AuType2 Checksum field zero (ospf/packet/header.go:317-321) and VerifyPacketChecksum accepts the zero checksum (ospf/packet/checksum.go:28-30); only the field-is-zero positive is asserted |
| `RFC2328-A.3.3-1` | Interface MTU should be set to 0 in Database Description packets sent over virtual links. (§A.3.3) | MUST | A.3.3 - Appendix subsection A.3.3 | **positive:** `unit/verify` [`TestRFC2328VirtualInterfaceHasNoMTU`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_test.go#L32). **positive:** `unit/verify` [`TestRFC2328VirtualLinkDBDescCarriesZeroMTU`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L58). **negative:** `unit/verify` [`TestRFC2328VirtualLinkDBDescCarriesZeroMTU`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L63) |
| `RFC2328-10.1-1` | Only one Database Description Packet is allowed outstanding at any one time. (§10.1) | MUST | 10.1 | **positive:** `unit/verify` [`TestOSPFDDRetransmit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_nsm_test.go#L527). **positive:** `unit/verify` [`TestRFC2328MasterHoldsOneOutstandingDD`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_dd_outstanding_test.go#L81). **negative:** `unit/verify` [`TestOSPFDuplicateDD`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_nsm_test.go#L595). **negative:** `unit/verify` [`TestRFC2328MasterSendsNoSecondDDWithoutAck`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_dd_outstanding_test.go#L118) |
| `RFC2328-10.2-1` | If an LSA cannot be found in the database, something has gone wrong with the Database Exchange process, and neighbor event BadLSReq should be generated. (§10.7) | MUST | 10.7 | **positive:** `unit/verify` [`TestOSPFBadLSReqRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_nsm_test.go#L734). **negative:** `unit/verify` [`TestOSPFValidLSReqSendsLSUpdate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_nsm_test.go#L905). **negative:** `unit/verify` [`TestRFC2328KnownLSRequestDoesNotRestartExchange`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L121) |
| `RFC2328-C.3-1` | The interface output cost must always be greater than 0. (§C.3) | MUST | C.3 | **positive:** `unit/verify` [`TestInterfaceCostAndTransmitDelayBoundary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_config_interface_validate_test.go#L16). **positive:** `unit/verify` [`TestRFC2328DefaultedInterfaceCostIsPositive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_cost_default_test.go#L35). **negative:** `unit/verify` [`TestInterfaceCostAndTransmitDelayBoundary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_config_interface_validate_test.go#L17). **negative:** `unit/verify` [`TestRFC2328DefaultedInterfaceCostIsPositive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_cost_default_test.go#L38) |
| `RFC2328-3.6-1` | One or more of the stub area's area border routers must advertise a default route into the stub area via summary-LSAs. (§3.6) | MUST | 3.6 | **positive:** `unit/verify` [`TestRFC2328StubAreaGetsDefaultSummary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_abr_summary_test.go#L34). **negative:** `unit/verify` [`TestRFC2328NormalAreaGetsNoDefaultSummary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_abr_summary_test.go#L53) |
| `RFC2328-4.4-1` | Support for receiving and sending IP multicast datagrams, along with the appropriate lower-level protocol support, is required. (§4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestOSPFMulticastMembershipInstalled`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_socket_linux_test.go#L76). **positive:** `unit/verify` [`TestOSPFTransportVethMulticastRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_transport_integration_linux_test.go#L31). **negative:** `unit/verify` [`TestOSPFMulticastMembershipRefusesForeignGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_socket_linux_test.go#L97). **negative:** `unit/verify` [`TestOSPFTransportAllDRoutersReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_transport_integration_linux_test.go#L81) |
| `RFC2328-4.4-2` | The router's IP protocol support must include the ability to divide a single IP class A, B, or C network number into many subnets of various sizes. This is commonly called variable-length subnetting; see Section 3.5 for details. IP supernetting support The router's IP protocol support must include the ability to aggregate contiguous collections of IP class A, B, and C networks into larger quantities called supernets. (§4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestRFC2328ClasslessPrefixesReachIPRIB`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_classless_test.go#L34). **negative:** `unit/verify` [`TestRFC2328ClasslessWithdrawalPreservesOverlaps`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_classless_test.go#L51) |
| `RFC2328-4.4-3` | Indications must be passed from these protocols to OSPF as the network interface goes up and down. (§4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestOSPFTransportPassesLinkIndicationsToOSPF`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_link_test.go#L8). **positive:** `unit/verify` [`TestRFC2328LinkIndicationsReachTheOSPFEngine`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_link_indication_test.go#L39). **negative:** `unit/verify` [`TestOSPFTransportPassesNoIndicationForNonOSPFInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_link_test.go#L45) |
| `RFC2328-8.1-1` | On these interfaces, the IP source should be set to any of the other IP addresses belonging to the router. For this reason, there must be at least one IP address assigned to the router. (§8.1) | MUST | 8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** unnumbered point-to-point interfaces are not implemented: an OSPF interface needs its own IPv4 address (interfaceIPv4, internal/plugins/ospf/transport/backend_linux.go), so no interface sources from another router address; owner ruling 8 (g), 2026-10-02 |
| `RFC2328-8.2-1` | In order for the packet to be accepted at the IP level, it must pass a number of tests, even before the packet is passed to OSPF for processing: o The IP checksum must be correct. o The packet's IP destination address must be the IP address of the receiving interface, or one of the IP multicast addresses AllSPFRouters or AllDRouters. o The IP protocol specified must be OSPF (89). (§8.2) | MUST | 8.2 | **positive:** `unit/verify` [`TestOSPFReceiveAcceptsValidIPv4Envelope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_backend_linux_test.go#L136). **negative:** `unit/verify` [`TestOSPFReceiveRejectsInvalidIPv4Envelope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_backend_linux_test.go#L157) |
| `RFC2328-8.2-2` | The Area ID found in the OSPF header must be verified. If both of the following cases fail, the packet should be discarded. The Area ID specified in the header must either: (1) Match the Area ID of the receiving interface. In this case, the packet has been sent over a single hop. Therefore, the packet's IP source address is required to be on the same network as the receiving interface. This can be verified by comparing the packet's IP source address to the interface's IP address, after masking both addresses with the interface mask. This comparison should not be performed on point-to-point networks. On point-to-point networks, the interface addresses of each end of the link are assigned independently, if they are assigned at all. (2) Indicate the backbone. In this case, the packet has been sent over a virtual link. The receiving router must be an area border router, and the Router ID specified in the packet (the source router) must be the other end of a configured virtual link. The receiving interface must also attach to the virtual link's configured Transit area. (§8.2) | MUST | 8.2 | **positive:** `unit/verify` [`TestOSPFReceiveAcceptsMatchingAreaID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_receive_test.go#L49). **positive:** `unit/verify` [`TestOSPFReceiveAcceptsOffNetworkSourceOnPointToPoint`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_receive_source_test.go#L97). **positive:** `unit/verify` [`TestOSPFReceiveAcceptsOnNetworkSourceOnBroadcast`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_receive_source_test.go#L83). **positive:** `unit/verify` [`TestRFC2328VirtualLinkAreaIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_virtual_area_test.go#L52). **negative:** `unit/verify` [`TestOSPFReceiveDropsMismatchedAreaID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_receive_test.go#L65). **negative:** `unit/verify` [`TestOSPFReceiveDropsOffNetworkSourceOnBroadcast`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_receive_source_test.go#L68). **negative:** `unit/verify` [`TestRFC2328VirtualLinkAreaIDRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_virtual_area_test.go#L70) |
| `RFC2328-8.2-3` | The AuType specified in the packet must match the AuType specified for the associated area. (§8.2) | MUST | 8.2 | **positive:** `unit/verify` [`TestOSPFReceiveAcceptsMatchingAuType`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_receive_test.go#L82). **negative:** `unit/verify` [`TestOSPFReceiveDropsMismatchedAuType`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_receive_test.go#L104). **negative:** `unit/verify` [`TestRFC2328NullAuTypeIntoPasswordAreaDroppedForAuType`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_autype_test.go#L48). **negative:** `unit/verify` [`TestRFC2328PasswordAuTypeIntoNullAreaDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_autype_test.go#L72) |
| `RFC2328-9.1-1` | The router must also originate a network-LSA for the network node. (§9.1, §12.4.2) | MUST | 9.1 | **positive:** `unit/verify` [`TestRFC2328DROriginatesNetworkLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L384). **negative:** `unit/verify` [`TestRFC2328NonDROriginatesNoNetworkLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L404) |
| `RFC2328-9.5.1-1` | If the router is eligible to become Designated Router, it must periodically send Hello Packets to all neighbors that are also eligible. In addition, if the router is itself the Designated Router or Backup Designated Router, it must also send periodic Hello Packets to all other neighbors. This means that any two eligible routers are always exchanging Hello Packets, which is necessary for the correct operation of the Designated Router election algorithm. To minimize the number of Hello Packets sent, the number of eligible routers on an NBMA network should be kept small. If the router is not eligible to become Designated Router, it must periodically send Hello Packets to both the Designated Router and the Backup Designated Router (if they exist). (§9.5.1) | MUST | 9.5.1 | **positive:** `unit/verify` [`TestOSPFNBMAEligibleRouterHellosEveryEligibleNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc2328_dr_test.go#L75). **positive:** `unit/verify` [`TestRFC2328NBMAHelloTargetsByRole`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc2328_nbma_hello_roles_test.go#L15). **negative:** `unit/verify` [`TestOSPFNBMAPeriodicHelloSkipsIneligibleNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc2328_dr_test.go#L92) |
| `RFC2328-10.5-1` | Next, the values of the Network Mask, HelloInterval, and RouterDeadInterval fields in the received Hello packet must be checked against the values configured for the receiving interface. Any mismatch causes processing to stop and the packet to be dropped. (§10.5) | MUST | 10.5 | **positive:** `unit/verify` [`TestRFC2328HelloMatchingParametersAndTwoWay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc2328_hello_test.go#L16). **negative:** `unit/verify` [`TestRFC2328HelloMismatchRejectedAndNoTwoWayUnlisted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc2328_hello_test.go#L35) |
| `RFC2328-10.6-1` | In states Loading and Full the slave must resend its last Database Description packet in response to duplicate Database Description packets received from the master. For this reason the slave must wait RouterDeadInterval seconds before freeing the last Database Description packet. (§10.8) | MUST | 10.8 | **positive:** `unit/verify` [`TestRFC2328SlaveRepliesAndRepeatsOnDuplicate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_dd_slave_test.go#L30). **positive:** `unit/verify` [`TestRFC2328SlaveResendsLastDDInLoadingAndFull`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_dd_slave_after_exchange_test.go#L72). **negative:** `unit/verify` [`TestRFC2328SlaveRefusesOutOfSequenceDD`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_dd_slave_test.go#L62). **negative:** `unit/verify` [`TestRFC2328SlaveRepeatsOnlyForTheLastMasterDD`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_dd_slave_after_exchange_test.go#L103) |
| `RFC2328-12.1.6-1` | When an attempt is made to increment the sequence number past the maximum value of N - 1 (0x7fffffff; also referred to as MaxSequenceNumber), the current instance of the LSA must first be flushed from the routing domain. This is done by prematurely aging the LSA (see Section 14.1) and reflooding it. As soon as this flood has been acknowledged by all adjacent neighbors, a new instance can be originated with sequence number of InitialSequenceNumber. (§12.1.6) | MUST | 12.1.6 | **positive:** `unit/verify` [`TestRFC2328SequenceWrapFloodsFlushThenRestarts`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_sequence_wrap_test.go#L69). **positive:** `unit/verify` [`TestRFC2328SequenceWrapRestartsAfterAck`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L217). **negative:** `unit/verify` [`TestRFC2328SequenceWrapWaitsForAck`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L194). **negative:** `unit/verify` [`TestRFC2328SequenceWrapWaitsForEveryNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_sequence_wrap_test.go#L100) |
| `RFC2328-12.2-1` | An implementation of OSPF must be able to access individual pieces of an area database. This lookup function is based on an LSA's LS type, Link State ID and Advertising Router. (§12.2) | MUST | 12.2 | **positive:** `unit/verify` [`TestRFC2328LookupByTriple`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L160). **positive:** `unit/verify` [`TestRFC2328LookupTellsApartLSAsSharingTwoElements`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_lookup_triple_test.go#L47). **negative:** `unit/verify` [`TestRFC2328LookupMissesWhenOneElementDiffers`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_lookup_triple_test.go#L73). **negative:** `unit/verify` [`TestRFC2328LookupNeedsFullTriple`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L174) |
| `RFC2328-12.4-1` | If a router advertises a summary-LSA for a destination which then becomes unreachable, the router must then flush the LSA from the routing domain by setting its age to MaxAge and reflooding (see Section 14.1). (§12.4.3) | MUST | 12.4.3 | **positive:** `unit/verify` [`TestRFC2328SummaryFlushedWhenDestinationUnreachable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_unreachable_summary_test.go#L94). **positive:** `unit/verify` [`TestRFC2328WithdrawnSummaryFlushedAtMaxAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L249). **negative:** `unit/verify` [`TestRFC2328AdvertisedSummaryNotFlushed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L286). **negative:** `unit/verify` [`TestRFC2328SummaryKeptWhileDestinationReachable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_unreachable_summary_test.go#L120) |
| `RFC2328-12.4.1-1` | All of the router's links to the area must be described in a single router-LSA. (§A.4.2) | MUST | A.4.2 - Appendix subsection A.4.2 | **positive:** `unit/verify` [`TestRFC2328SingleRouterLSACarriesEveryLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L318). **negative:** `unit/verify` [`TestRFC2328RouterLSAExcludesOtherAreaLinks`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L350) |
| `RFC2328-12.4.3-1` | The router must then originate summary-LSAs into the newly attached area for all pertinent intra-area and inter-area routes in the router's routing table. (§12.4) | MUST | 12.4 | **positive:** `unit/verify` [`TestRFC2328ABRSummarizesAndCondenses`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_abr_summary_test.go#L67). **positive:** `unit/verify` [`TestRFC2328NewlyAttachedAreaGetsIntraAndInterAreaSummaries`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_newly_attached_area_test.go#L82). **negative:** `unit/verify` [`TestRFC2328ABRComponentNeverEscapesRange`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_abr_summary_test.go#L101). **negative:** `unit/verify` [`TestRFC2328NewlyAttachedAreaGetsNoSummaryOfItsOwnRoutes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_newly_attached_area_test.go#L106) |
| `RFC2328-13-5` | Remove the current database copy from all neighbors' Link state retransmission lists. (§13) | MUST | 13 | **positive:** `unit/verify` [`TestRFC2328ReplacedInstanceLeavesRetransmitLists`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L110). **positive:** `unit/verify` [`TestRFC2328ReplacedInstanceLeavesSenderRetransmitList`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_replaced_retransmit_test.go#L18). **negative:** `unit/verify` [`TestRFC2328DuplicateKeepsOtherRetransmitLists`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L144) |
| `RFC2328-13-6` | If the contents are different, the following pieces of the routing table must be recalculated, depending on the new LSA's LS type field: Router-LSAs and network-LSAs The entire routing table must be recalculated, starting with the shortest path calculations for each area (not just the area whose link-state database has changed). (§13.2) | MUST | 13.2 | **positive:** `unit/verify` [`TestRFC2328ChangedContentsRecalculateEveryArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_contents_change_recalc_test.go#L113). **positive:** `unit/verify` [`TestSPFRecalculatesEveryAreaOnOneAreaChange`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_recalc_test.go#L14). **negative:** `unit/verify` [`TestSPFRecalculationBoundedToConfiguredAreas`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_recalc_test.go#L47) |
| `RFC2328-13-7` | If there is already a database copy, and if the database copy was received via flooding and installed less than MinLSArrival seconds ago, discard the new LSA (without acknowledging it) and examine the next LSA (if any) listed in the Link State Update packet. (§13) | MUST | 13 | **positive:** `unit/verify` [`TestRFC2328MinLSArrivalElapsedAccepts`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L79). **positive:** `unit/verify` [`TestRFC2328TooSoonDiscardThenNextLSAExamined`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_too_soon_next_lsa_test.go#L17). **negative:** `unit/verify` [`TestRFC2328MinLSArrivalDiscardsWithoutAck`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L44) |
| `RFC2328-13.3-3` | On non-broadcast networks, separate Link State Update packets must be sent, as unicasts, to each adjacent neighbor (i.e., those in state Exchange or greater). (§13.3) | MUST | 13.3 | **positive:** `unit/verify` [`TestRFC2328NBMAUnicastsToEachAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L430). **negative:** `unit/verify` [`TestRFC2328NBMANeverMulticastsNorReachesTwoWay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L464) |
| `RFC2328-15-2` | The cost of a virtual link is NOT configured. It is defined to be the cost of the intra-area path between the two defining area border routers. This cost appears in the virtual link's corresponding routing table entry. When the cost of a virtual link changes, a new router-LSA should be originated for the backbone area. (§15) | MUST | 15 | **positive:** `unit/verify` [`TestRFC2328VirtualCostChangeOriginatesNewInstance`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_virtual_cost_test.go#L30). **positive:** `unit/verify` [`TestRFC2328VirtualCostChangeReoriginatesBackbone`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_virtual_route_test.go#L67). **positive:** `unit/verify` [`TestRFC2328VirtualCostIsTransitPathCost`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_virtual_cost_test.go#L15). **negative:** `unit/verify` [`TestRFC2328VirtualLinkCostNotConfigurable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_virtual_cost_config_test.go#L69). **negative:** `unit/verify` [`TestRFC2328VirtualLinkCostNotConfigurableV3`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_virtual_cost_config_test.go#L139). **negative:** `unit/verify` [`TestRFC2328VirtualUnusablePathWithdrawsBackboneLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_virtual_route_test.go#L98) |
| `RFC2328-16.1-3` | Note that when there is a choice of vertices closest to the root, network vertices must be chosen before router vertices in order to necessarily find all equal-cost paths. (§16.1) | MUST | 16.1 | **positive:** `unit/verify` [`TestRFC2328EqualDistanceNetworkVertexFirst`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_tiebreak_test.go#L10). **negative:** `unit/verify` [`TestRFC2328CloserRouterVertexBeforeFartherNetwork`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_tiebreak_test.go#L25) |
| `RFC2328-A.1-1` | Packets sent to these multicast addresses should never be forwarded; they are meant to travel a single hop only. To ensure that these packets will not travel multiple hops, their IP TTL must be set to 1. (§A.1) | MUST | A.1 - Appendix subsection A.1 | **positive:** `unit/verify` [`TestOSPFSocketTTLIsOne`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_socket_linux_test.go#L30). **negative:** `unit/verify` [`TestOSPFSocketTTLFailureIsAnError`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_socket_linux_test.go#L58) |
| `RFC2328-A.1-2` | Both the Designated Router and Backup Designated Router must be prepared to receive packets destined to this address. (§A.1) | MUST | A.1 - Appendix subsection A.1 | **positive:** `unit/verify` [`TestOSPFElectedBDRJoinsAllDRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc2328_dr_test.go#L29). **positive:** `unit/verify` [`TestOSPFElectedDRJoinsAllDRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc2328_dr_alldrouters_test.go#L29). **negative:** `unit/verify` [`TestOSPFDROtherDoesNotJoinAllDRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc2328_dr_test.go#L46). **negative:** `unit/verify` [`TestOSPFElectedDRJoinsAllDRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc2328_dr_alldrouters_test.go#L44) |
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
| [`RFC2328-8.1-1`](#rfc2328-8.1-1) On these interfaces, the IP source should be set to any of the other IP addresses belonging to the router. For this reason, there must be at least one IP address assigned to the router. (§8.1) | {gap}, no test | unnumbered point-to-point interfaces are not implemented: an OSPF interface needs its own IPv4 address (interfaceIPv4, internal/plugins/ospf/transport/backend_linux.go), so no interface sources from another router address; owner ruling 8 (g), 2026-10-02 |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2328-A.3.1-1`](#rfc2328-a.3.1-1)

The version number field must specify protocol version 2. (§8.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting or emitting a Version other than 2. Red: TestOSPFHeaderRejectsBadVersionAndLength fails unless DecodeHeader returns ErrBadVersion for Version 3. Other polarity: TestOSPFHeaderRoundTrip fails unless every packet type written by WriteTo decodes, which needs the emitted Version octet to equal 2. One clause, both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFHeaderRejectsBadVersionAndLength`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc2328_header_test.go#L153) | unit/verify | unproven |
| positive | [`TestOSPFHeaderRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc2328_header_test.go#L131) | unit/verify | unproven |

### [`RFC2328-A.3.1-2`](#rfc2328-a.3.1-2)

The standard IP checksum of the entire contents of the packet, starting with the OSPF packet header but excluding the 64-bit authentication field. (§A.3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. c20 judge: independent arithmetic (test-side RFC 1071 fold) for both sides. TestRFC2328PacketChecksumCoversHeaderExcludesAuth: with a simple password the generated field equals the reference over header+body excluding Auth, differs from the whole-packet sum, is unchanged by another password and moves with the Router ID. TestRFC2328PacketChecksumWrongRangeRefused: VerifyPacketChecksum refuses an Auth-included and a body-excluded checksum, accepts the correct one. Revert records PacketChecksum (+), VerifyPacketChecksum (-).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328PacketChecksumWrongRangeRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc2328_checksum_coverage_test.go#L61) | unit/verify | revert, verified |
| negative | [`TestPacketVerifyChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc2328_header_test.go#L100) | unit/verify | unproven |
| positive | [`TestOSPFPacketChecksumExcludesAuth`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L30) | unit/verify | unproven |
| positive | [`TestRFC2328PacketChecksumCoversHeaderExcludesAuth`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc2328_checksum_coverage_test.go#L31) | unit/verify | revert, verified |

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both steps proven on the real wire path: TestRFC2328DiscardedLSAThenNextLSAProcessed encodes one LS Update (bad-checksum LSA, or LS type 6 with a valid Fletcher, then a good router-LSA) and dispatches it through the engine from an Exchange neighbor; the bad LSA is neither stored nor acknowledged and the next is installed and acknowledged. Red against the pre-fix producers (ReceiveUpdate returned on the first bad checksum; DecodeLSUpdate failed the whole packet on an unknown type). D-8 fix read at the producer: LSAIterator.Next advances at least 20 octets per pass and a truncated or short Length still ends the packet with an error (fuzzed 20 s each, iterator and DecodePacket); skipped LSAs count against # LSAs. The DecodeLSReq unit lost its 13-1 tag (LS Request decoder, D-15).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328BadLSChecksumDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_test.go#L17) | unit/verify | unproven |
| negative | [`TestRFC2328DiscardedLSAThenNextLSAProcessed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_lsupdate_discard_next_test.go#L91) | unit/verify | revert, verified |
| positive | [`TestOSPFFloodOutOtherInterfaces`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_decisions_test.go#L30) | unit/verify | unproven |
| positive | [`TestRFC2328DiscardedLSAThenNextLSAProcessed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_lsupdate_discard_next_test.go#L92) | unit/verify | revert, verified |

### [`RFC2328-13-2`](#rfc2328-13-2)

AS-external-LSAs are not flooded into/throughout stub areas (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. c20 judge: both clauses on the real flood path. Into: TestRFC2328ASExternalNotFloodedIntoStubArea, four-area ABR, Type-5 received on eth0 and self-originated, reaches the normal-area control eth3 and never leaves the stub eth1 or NSSA eth2, no stub/NSSA retransmission entry (red with eligibleInterface reverted). Throughout: TestRFC2328ASExternalFromStubInterfaceGoesNowhere, Type-5 from the stub/NSSA neighbor not installed and flooded out no interface, eth1 back included (red with shouldDropByArea reverted). Old helper-only units kept as supplementary. Helper rename by sed re-read: lsaFloodedOn consistent, no leftover.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFStubAreaDropsType5`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_decisions_test.go#L248) | unit/verify | unproven |
| negative | [`TestRFC2328ASExternalFromStubInterfaceGoesNowhere`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_stub_external_flood_test.go#L112) | unit/verify | revert, verified |
| positive | [`TestOSPFStubFloodFilter`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_area_type_test.go#L15) | unit/verify | unproven |
| positive | [`TestRFC2328ASExternalNotFloodedIntoStubArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_stub_external_flood_test.go#L63) | unit/verify | revert, verified |

### [`RFC2328-13-3`](#rfc2328-13-3)

If the neighbor is in a lesser state than Exchange, the packet should be dropped without further processing. (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Engine drop now exercised: LS Updates dispatched through eng.dispatch from an unknown neighbor, Init, and ExStart are not stored and draw no LS Ack or LS Update even after FlushDelayedAcks; the same packet after Exchange is installed and acknowledged. Disabling the AcceptsFlooding drop in handleLSUpdate turns all three negatives red. 2-Way (not reachable on point-to-point) stays on the neighbor-package AcceptsFlooding units, which gate the same call.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328FloodingRequiresExchangeOrHigher`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L20) | unit/verify | unproven |
| negative | [`TestRFC2328LSUpdateBelowExchangeDroppedByEngine`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_flood_state_gate_test.go#L83) | unit/verify | revert, verified |
| positive | [`TestRFC2328FloodingRequiresExchangeOrHigher`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L17) | unit/verify | unproven |
| positive | [`TestRFC2328LSUpdateFromExchangeNeighborProcessed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_flood_state_gate_test.go#L120) | unit/verify | revert, verified |

### [`RFC2328-13.1-1`](#rfc2328-13.1-1)

The LSA having the newer LS sequence number is more recent. See Section 12.1.6 for an explanation of the LS sequence number space. If both instances have the same LS sequence number, then: o If the two instances have different LS checksums, then the instance having the larger LS checksum (when considered as a 16-bit unsigned integer) is considered more recent. o Else, if only one of the instances has its LS age field set to MaxAge, the instance of age MaxAge is considered to be more recent. o Else, if the LS age fields of the two instances differ by more than MaxAgeDiff, the instance having the smaller (younger) LS age is considered to be more recent. (§13.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. c20 judge: order and signedness now discriminated. TestRFC2328FreshnessStepsAppliedInOrder pits an earlier step against a later one in both argument orders (sequence over checksum+MaxAge, unsigned 0x8001 over 0x7FFF against MaxAge, MaxAge over age, age > MaxAgeDiff, MaxSequenceNumber over InitialSequenceNumber signed); TestRFC2328LessRecentByEarlierStepRefused through ReceiveUpdate (past MinLSArrival) keeps the database copy against a lower sequence or smaller checksum at MaxAge. Matrix and Older-sequence units kept. Revert records both polarities on CompareHeaders; author overlays (order swaps, signed checksum, unsigned sequence) red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328LessRecentByEarlierStepRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_freshness_order_test.go#L55) | unit/verify | revert, verified |
| negative | [`TestRFC2328OlderInstanceGetsDatabaseCopyBack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_test.go#L51) | unit/verify | unproven |
| positive | [`TestRFC2328FreshnessStepsAppliedInOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_freshness_order_test.go#L17) | unit/verify | revert, verified |
| positive | [`TestOSPFFreshnessCompareMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_lsdb_test.go#L126) | unit/verify | unproven |

### [`RFC2328-13-4`](#rfc2328-13-4)

If the database copy has LS age equal to MaxAge and LS sequence number equal to MaxSequenceNumber, simply discard the received LSA without acknowledging it. (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: acknowledging, or sending the database copy back for, an LSA received while the database copy is MaxAge at MaxSequenceNumber. Red: TestOSPFMaxSeqMaxAgeSilentDiscard installs a MaxSequenceNumber copy aged to MaxAge, receives an older instance, flushes delayed acks, and fails on any send. Other polarity: TestRFC2328OlderInstanceGetsDatabaseCopyBack fails if an ordinary more-recent copy is not sent back to the sender (so a blanket silent discard goes red). One clause, both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328OlderInstanceGetsDatabaseCopyBack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_test.go#L54) | unit/verify | unproven |
| positive | [`TestOSPFMaxSeqMaxAgeSilentDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_edges_test.go#L19) | unit/verify | unproven |

### [`RFC2328-13.3-1`](#rfc2328-13.3-1)

LSAs flooded out an adjacency are placed on the adjacency's Link state retransmission list. In order to ensure that flooding is reliable, these LSAs are retransmitted until they are acknowledged. The length of time between retransmissions is a configurable per-interface value, RxmtInterval. (§13.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Clauses: (a) flooded LSAs go on the adjacency's retransmission list: TestOSPFFloodOutOtherInterfaces + (eth1 Full neighbor queued; record on queueRetransmit observed red) and TestOSPFFloodQueuesExchangeAndLoadingNeighbors - (2-Way and the receiving neighbor never queued; record on isFloodEligibleNeighborState observed red). (b) retransmitted until acknowledged, every configurable per-interface RxmtInterval: TestRFC2328RetransmitEveryConfiguredRxmtInterval parses a real OSPFv2 config (eth1 retransmit-interval 3, eth2 11, eth0 default), takes RxmtInterval from engine.lsdbTopology, floods via ReceiveUpdate and ticks RetransmitTick each second 1..24: + eth1 at 3,6,9,12 and eth2 at 11,22 (repeated, per interface); - nothing on any other second, so a hard-coded 5 s or a single retransmission fails, and after eth1's ack at 12 no eth1 resend at 15..24 while unacknowledged eth2 still resends at 22. Exact per-second set compared, buffers isolated (one LSA, one neighbor per interface). TestOSPFRetransmitTimer keeps its earlier default-interval pin. Records on RetransmitTick observed red for both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFFloodQueuesExchangeAndLoadingNeighbors`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_decisions_test.go#L57) | unit/verify | revert, verified |
| negative | [`TestRFC2328RetransmitEveryConfiguredRxmtInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_retransmit_interval_test.go#L19) | unit/verify | revert, verified |
| positive | [`TestOSPFFloodOutOtherInterfaces`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_decisions_test.go#L31) | unit/verify | revert, verified |
| positive | [`TestOSPFRetransmitTimer`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_decisions_test.go#L129) | unit/verify | revert, verified |
| positive | [`TestRFC2328RetransmitEveryConfiguredRxmtInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_retransmit_interval_test.go#L14) | unit/verify | revert, verified |

### [`RFC2328-13.3-2`](#rfc2328-13.3-2)

The LSA's LS age must be incremented by InfTransDelay (which must be > 0) when it is copied into the outgoing Link State Update packet (until the LS age field reaches the maximum value of MaxAge). (§13.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Clauses: LS age incremented by InfTransDelay and capped at MaxAge: TestRFC2328FloodIncrementsAgeByInfTransDelay (unchanged; records on floodCopy). The '(which must be > 0)' clause: TestRFC2328TransmitDelayMustBePositive + an OSPFv2 interface with transmit-delay 1 accepted by validateConfig, - transmit-delay 0 refused with ErrTransmitDelayZero (the config.go validateConfigAF check that production reaches through register.go validateConfig); each case its own single-interface config, so the refusal is this rule's and no other. Records on validateConfigAF observed red for both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328FloodIncrementsAgeByInfTransDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flood_age_test.go#L106) | unit/verify | revert, verified |
| negative | [`TestRFC2328TransmitDelayMustBePositive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_transmit_delay_test.go#L14) | unit/verify | revert, verified |
| positive | [`TestRFC2328FloodIncrementsAgeByInfTransDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flood_age_test.go#L90) | unit/verify | revert, verified |
| positive | [`TestRFC2328TransmitDelayMustBePositive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_transmit_delay_test.go#L11) | unit/verify | revert, verified |

### [`RFC2328-14-1`](#rfc2328-14-1)

An LSA's LS age is never incremented past the value MaxAge. LSAs having age MaxAge are not used in the routing table calculation. (§14)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: (a) LS age incremented past MaxAge: TestLSAgeAddSaturates fails unless MaxAge-10 plus 20 equals MaxAge exactly, and TestOSPFLSDBAgeToPurge fails unless the aged LSA sits at MaxAge; (b) a MaxAge LSA used in the routing calculation: TestOSPFGraphSkipsMaxAge fails if BuildGraph makes a router vertex of a MaxAge router-LSA. The fixture is isolated: BuildGraph (graph.go) turns a decodable non-MaxAge router-LSA into a vertex with no other filter, so only the age check can exclude it. Positive: age advances and saturation leaves DoNotAge untouched.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFGraphSkipsMaxAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_graph_test.go#L40) | unit/verify | unproven |
| positive | [`TestOSPFLSDBAgeToPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_aging_test.go#L34) | unit/verify | unproven |
| positive | [`TestLSAgeAddSaturates`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/rfc2328_lsage_test.go#L39) | unit/verify | unproven |

### [`RFC2328-14-2`](#rfc2328-14-2)

A MaxAge LSA must be removed immediately from the router's link state database as soon as both a) it is no longer contained on any neighbor Link state retransmission lists and b) none of the router's neighbors are in states Exchange or Loading. (§14)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Clause (b) now holds Exchange as well as Loading: a purged MaxAge LSA on no retransmission list is kept with the only neighbor in exchange or loading and removed at once in full or exstart; a Loading-only guard turns the exchange subtest red while the older Loading unit stays green. Clause (a) stays on the retransmission-list units. The units call deletePurgedIfAcked directly, the LSDB seam every trigger uses.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFPurgeRetainedForExchangeOrLoading`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_decisions_test.go#L384) | unit/verify | unproven |
| negative | [`TestRFC2328MaxAgeKeptWhileNeighborInExchangeOrLoading`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_maxage_exchange_test.go#L36) | unit/verify | revert, verified |
| positive | [`TestOSPFASExternalPurgeRetainedAcrossAreas`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_decisions_test.go#L333) | unit/verify | unproven |
| positive | [`TestRFC2328MaxAgeRemovedOnceNoNeighborExchangingOrLoading`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_maxage_exchange_test.go#L47) | unit/verify | revert, verified |

### [`RFC2328-13.5-1`](#rfc2328-13.5-1)

Each newly received LSA must be acknowledged. This is usually done by sending Link State Acknowledgment packets. However, acknowledgments can also be accomplished implicitly by sending Link State Update packets (see step 7a of Section 13). (§13.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. c20 judge: TestRFC2328NewerLSAAcknowledgedThroughReceiveUpdate reaches the ordinary newer-LSA ack through ReceiveUpdate, not hand-set flags: point-to-point, DROther and Backup (from the DR) each send an LS Ack carrying the header on eth0 after FlushDelayedAcks; the DR receiving from a DROther acks implicitly by flooding back out eth0. Negative TestOSPFDRRefloodsBackOutReceivingInterface (flooded back, implicit ack, no explicit ack, Table 19) now carries a judge-observed revert record on ackForReceive. Direct ack of unknown MaxAge held by TestOSPFUnknownMaxAgeNoCopyIsAckedAndDiscarded.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFDRRefloodsBackOutReceivingInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_edges_test.go#L80) | unit/verify | revert, verified |
| positive | [`TestOSPFAckDecisionTable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_decisions_test.go#L93) | unit/verify | unproven |
| positive | [`TestOSPFUnknownMaxAgeNoCopyIsAckedAndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_decisions_test.go#L361) | unit/verify | unproven |
| positive | [`TestRFC2328NewerLSAAcknowledgedThroughReceiveUpdate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_newer_lsa_ack_test.go#L17) | unit/verify | revert, verified |

### [`RFC2328-13.4-1`](#rfc2328-13.4-1)

A self-originated LSA is detected when either 1) the LSA's Advertising Router is equal to the router's own Router ID or 2) the LSA is a network- LSA and its Link State ID is equal to one of the router's own IP interface addresses. (§13.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both detection rules through ReceiveUpdate: router-LSA adv = own Router ID, and network-LSAs from 9.9.9.9 whose LSID is either own interface address, are each installed at MaxAge with sequence+1 and flooded at MaxAge. Negatives: a network-LSA with a foreign LSID, another router's router-LSA, and an AS-external whose LSID equals an own address (rule 2 is network-LSA only) are installed as received and never flushed. Disabling rule 2, or dropping its type check, turns the matching subtests red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFSelfOriginatedNoLocalCopyFlush`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_edges_test.go#L43) | unit/verify | unproven |
| negative | [`TestRFC2328ForeignLSANotTakenAsSelfOriginated`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_self_detect_test.go#L83) | unit/verify | revert, verified |
| positive | [`TestOSPFOriginateSelfReceivedHigherSeq`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_origination_test.go#L426) | unit/verify | unproven |
| positive | [`TestRFC2328SelfOriginatedDetectedByRouterIDOrInterfaceAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_self_detect_test.go#L60) | unit/verify | revert, verified |

### [`RFC2328-16.1-1`](#rfc2328-16.1-1)

Look up the vertex W's LSA (router-LSA or network-LSA) in Area A's link state database. If the LSA does not exist, or its LS age is equal to MaxAge, or it does not have a link back to vertex V, examine the next link in V's LSA. (§16.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. c20 judge: every skip clause and the continuation. TestRFC2328SPFExaminesNextLinkAfterSkip: root lists links to a missing router-LSA, a MaxAge router-LSA, a router with no link back, and a transit net with no network-LSA BEFORE 5.5.5.5, which is still reached at 10 with only 192.0.2.0/24 at 15 routed. TestRFC2328SPFSkipsMissingOrMaxAgeVertex isolates missing/MaxAge for router and network LSAs with a below-MaxAge control. Link-back clause by TestOSPFSPFShortestPath / TestOSPFTwoWayCheck. Revert records Compute (+), BuildGraph (-).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFTwoWayCheck`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_spf_test.go#L31) | unit/verify | unproven |
| negative | [`TestRFC2328SPFSkipsMissingOrMaxAgeVertex`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_spf_vertex_lookup_test.go#L58) | unit/verify | revert, verified |
| positive | [`TestOSPFSPFShortestPath`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_spf_test.go#L13) | unit/verify | unproven |
| positive | [`TestRFC2328SPFExaminesNextLinkAfterSkip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_spf_vertex_lookup_test.go#L22) | unit/verify | revert, verified |

### [`RFC2328-16.4-1`](#rfc2328-16.4-1)

Intra-area and inter-area paths are always preferred over AS external paths. (b) Type 1 external paths are always preferred over type 2 external paths. When all paths are type 2 external paths, the paths with the smallest advertised type 2 metric are always preferred. (§16.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. All four orderings asserted: intra over external (route_test), inter-area at 100 over E1 and E2 at 1 (selectBestRoutes, both input orders), E1 over E2 regardless of metric (external_test), and among E2 the smallest advertised metric (7 via an ASBR 50 away) over 9 via an ASBR 1 away through ComputeExternal. Ranking inter-area below externals, or letting distance decide E2, turns the new units red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFExternalE1PreferredOverE2`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_external_test.go#L83) | unit/verify | unproven |
| negative | [`TestRFC2328LowerPreferencePathRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_path_preference_test.go#L51) | unit/verify | revert, verified |
| positive | [`TestRFC2328PreferredPathTypeSelected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_path_preference_test.go#L36) | unit/verify | revert, verified |
| positive | [`TestOSPFRouteTablePreference`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_route_test.go#L8) | unit/verify | unproven |

### [`RFC2328-16.2-1`](#rfc2328-16.2-1)

If the router is attached to multiple areas (i.e., it is an area border router), only backbone summary-LSAs are examined. (§16)

Audit verdict: enforced (the tests do what the requirement demands), fresh. c20 judge: the 'attached to multiple areas' condition now discriminated: TestRFC2328NonABRUsesItsAreaSummaries, a router in area 0.0.0.1 only installs 10.40.0.0/24 from that area's summary at 2 via 3.3.3.3 (red under a backbone-only-for-every-router producer). Negative TestOSPFABRBackboneOnlyAcceptance: the ABR ignores the cheaper area-1 summary, metric 30 via 2.2.2.2; judge-observed revert record on ComputeInterAreaWith.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFABRBackboneOnlyAcceptance`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_interarea_test.go#L74) | unit/verify | revert, verified |
| positive | [`TestOSPFInterAreaRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_interarea_test.go#L43) | unit/verify | unproven |
| positive | [`TestRFC2328NonABRUsesItsAreaSummaries`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_nonabr_summary_test.go#L15) | unit/verify | revert, verified |

### [`RFC2328-16.2-2`](#rfc2328-16.2-2)

If the cost specified by the LSA is LSInfinity, or if the LSA's LS age is equal to MaxAge, then examine the the next LSA. (2) If the LSA was originated by the calculating router itself, examine the next LSA. (§16.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Closes the earlier gap (advertised LSInfinity unfed). + TestRFC2328SummaryAdvertisingLSInfinitySkipped: summary cost 20 through an ABR 10 away installs one route; - the same summary with cost LSInfinity installs none. MaxAge and self-originated skips stay on rfc2328_test.go; composed-cost saturation on interarea_test. Records: + v4SummaryReader, - ComputeInterAreaWith, observed red. Note: the advertised-cost check (v4SummaryReader, interarea.go:189) is subsumed by the composed-cost check (ComputeInterAreaWith, interarea.go:153), because clampMetric saturates any sum with an LSInfinity term to LSInfinity; a mutant removing only :189 stays green, but no input then installs a route, so the behaviour the clause demands is enforced by the unit either way. The AS-external tags on external_test/rfc2328_test prove s16.4 neighbours and are supplementary.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFExternalLSInfinityDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_external_test.go#L61) | unit/verify | unproven |
| negative | [`TestOSPFInterAreaLSInfinityDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_interarea_test.go#L137) | unit/verify | unproven |
| negative | [`TestRFC2328SummaryAdvertisingLSInfinitySkipped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_summary_lsinfinity_test.go#L40) | unit/verify | revert, verified |
| negative | [`TestRFC2328ExternalSkipsMaxAgeAndSelf`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_test.go#L43) | unit/verify | unproven |
| negative | [`TestRFC2328InterAreaSkipsMaxAgeAndSelfSummary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_test.go#L20) | unit/verify | unproven |
| positive | [`TestOSPFInterAreaRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_interarea_test.go#L44) | unit/verify | unproven |
| positive | [`TestRFC2328SummaryAdvertisingLSInfinitySkipped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_summary_lsinfinity_test.go#L35) | unit/verify | revert, verified |

### [`RFC2328-D.2-1`](#rfc2328-d.2-1)

The 64-bit authentication field in the OSPF packet header must be equal to the 64-bit password (i.e., authentication key) that has been configured for the interface. (§D.5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting a Simple-password packet whose 64-bit authentication field differs from the configured password. Red: TestRFC2328SimplePasswordMismatchDiscarded fails if a packet signed with 'badpass' is accepted, and fails unless the reason is exactly password-mismatch (isolated: the AuType matches). Other polarity: the same unit fails if the configured 'goodpass' is rejected. One clause, both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328SimplePasswordMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_test.go#L99) | unit/verify | unproven |
| positive | [`TestRFC2328SimplePasswordMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_test.go#L95) | unit/verify | unproven |

### [`RFC2328-D.3-1`](#rfc2328-d.3-1)

The message digest appended to the OSPF packet is not actually considered part of the OSPF protocol packet: the message digest is not included in the OSPF header's packet length, although it is included in the packet's IP header length field. (§D.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. c21 judge (independent). Clause 1 (digest not in Packet Length): TestOSPFAuthSignVerifyCrypto fails unless Packet Length stays plen and len(signed) == plen + digest. Clause 2 (digest in the IP length): new TestRFC2328DigestCountedInIPLengthNotPacketLength, both send paths (SendPacket, SendPacketRouted) with packet.Sign keyed MD5 as signer: the socket receives 44+16 octets, Packet Length still 44, trailer equals an independent D.4.3 MD5; a raw proto-89 socket (backend_linux Sendto, no IP_HDRINCL) builds the IP header around exactly that payload. Trim-after-signer overlay red on both subtests (author log, judge diffed the overlay); record on SendPacket observed red. engine.signPacket returns Sign output unchanged and its framing is pinned by the untagged TestEngineSignPacketCrypto Verify round-trip (Verify requires plen+L == len). Negative TestOSPFAuthCryptoRejectsExtraTrailerBytes refuses a wire whose trailer exceeds the digest framing, now with a Verify record observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthCryptoRejectsExtraTrailerBytes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L219) | unit/verify | revert, verified |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L47) | unit/verify | unproven |
| positive | [`TestRFC2328DigestCountedInIPLengthNotPacketLength`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_digest_ip_length_test.go#L35) | unit/verify | revert, verified |

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged c18. Forbidden: a second DD leaving the master while one is unacknowledged. Setup gives the master (10.0.0.3) a summary list longer than one DD, so a non-compliant master has a DD n+1 to send. Positive TestRFC2328MasterHoldsOneOutstandingDD: three RxmtInterval expiries each send exactly one packet, DD n unchanged (sameDD), exactly three DDs all seq n; the slave DD acking n releases exactly one DD n+1 with the remaining 3 headers. Negative TestRFC2328MasterSendsNoSecondDDWithoutAck: a repeat of the slave's previous DD -> duplicate-drop with nothing sent; a slave DD n+2 -> SeqNumberMismatch and an Init DD; no non-Init DD n+1 is ever sent. Records: + resendLastDDLocked, - handleExchangeDDLocked, observed red. Old nsm_test tags kept as supplementary; TestOSPFDuplicateDD (negative) proves the slave's 10.8 duplicate, a neighbour, and carries no weight here.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328MasterSendsNoSecondDDWithoutAck`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_dd_outstanding_test.go#L118) | unit/verify | revert, verified |
| negative | [`TestOSPFDuplicateDD`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_nsm_test.go#L595) | unit/verify | unproven |
| positive | [`TestRFC2328MasterHoldsOneOutstandingDD`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_dd_outstanding_test.go#L81) | unit/verify | revert, verified |
| positive | [`TestOSPFDDRetransmit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_nsm_test.go#L527) | unit/verify | unproven |

### [`RFC2328-10.2-1`](#rfc2328-10.2-1)

If an LSA cannot be found in the database, something has gone wrong with the Database Exchange process, and neighbor event BadLSReq should be generated. (§10.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an LS Request for an LSA absent from the database not generating BadLSReq. Red: TestOSPFBadLSReqRestart fails on HandleLSReq != reasonBadLSReq, on state != exstart, and on the resent DD not being the initial I|M|MS DD. Other polarity: TestOSPFValidLSReqSendsLSUpdate and TestRFC2328KnownLSRequestDoesNotRestartExchange go red if a satisfiable request restarts the exchange (state change, exstart) or is not answered with one LS Update carrying that LSA. One clause, both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFValidLSReqSendsLSUpdate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_nsm_test.go#L905) | unit/verify | unproven |
| negative | [`TestRFC2328KnownLSRequestDoesNotRestartExchange`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L121) | unit/verify | unproven |
| positive | [`TestOSPFBadLSReqRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_nsm_test.go#L734) | unit/verify | unproven |

### [`RFC2328-C.3-1`](#rfc2328-c.3-1)

The interface output cost must always be greater than 0. (§C.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29 (independent judge). Explicit cost 0 stays refused (TestInterfaceCostAndTransmitDelayBoundary). The unset-cost gap is closed: TestRFC2328DefaultedInterfaceCostIsPositive resolves interfaces from config text with no cost leaf and reads interfaceCost, the one producer every consumer uses (instance.go Router-LSA and routing-table cost, ldp_sync.go, te_originate.go): 1 Gbit/s under 100 Gbit/s -> 100, and the inputs that divide to 0 (100 Gbit/s under a 1 Mbit/s reference, unknown speed) -> 1. Records: revert on interfaceCost (positive) and on DefaultMetric (negative); a mutant dropping DefaultMetric's MetricMin clamp would be sharper.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInterfaceCostAndTransmitDelayBoundary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_config_interface_validate_test.go#L17) | unit/verify | unproven |
| negative | [`TestRFC2328DefaultedInterfaceCostIsPositive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_cost_default_test.go#L38) | unit/verify | revert, verified |
| positive | [`TestInterfaceCostAndTransmitDelayBoundary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_config_interface_validate_test.go#L16) | unit/verify | unproven |
| positive | [`TestRFC2328DefaultedInterfaceCostIsPositive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_cost_default_test.go#L35) | unit/verify | revert, verified |

### [`RFC2328-3.6-1`](#rfc2328-3.6-1)

One or more of the stub area's area border routers must advertise a default route into the stub area via summary-LSAs. (§3.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an ABR attached to a stub area not advertising a default route into it via a summary-LSA. Red: TestRFC2328StubAreaGetsDefaultSummary fails unless the stub area holds a Type 3 summary with Link State ID 0.0.0.0, mask 0.0.0.0 and the configured cost 5. Other polarity: TestRFC2328NormalAreaGetsNoDefaultSummary fails if a normal area receives one, and still checks the ordinary summary is originated. One clause, both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328NormalAreaGetsNoDefaultSummary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_abr_summary_test.go#L53) | unit/verify | revert, verified |
| positive | [`TestRFC2328StubAreaGetsDefaultSummary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_abr_summary_test.go#L34) | unit/verify | revert, verified |

### [`RFC2328-4.4-1`](#rfc2328-4.4-1)

Support for receiving and sending IP multicast datagrams, along with the appropriate lower-level protocol support, is required. (§4.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent judgment: RFC 2328 section 4.4 says, "Support for receiving and sending IP multicast datagrams, along with the appropriate lower-level protocol support, is required." The production raw protocol-89 backend now has proof on both sides: TestOSPFTransportVethMulticastRoundTrip reads IP_MULTICAST_IF == 192.0.2.1 and TTL 1, sends AllSPFRouters over a real veth into a peer namespace, and asserts receiving interface, source and exact payload. TestOSPFTransportAllDRoutersReceive first receives on that same link, leaves AllDRouters and requires subsequent delivery to be absent. The original duplicate-join/drop errno assertions prove actual socket membership installation/removal; the foreign-group refusal is supplementary OSPF API confinement, not a general IP multicast forwarding obligation. Inspected runInNS: caller pins its thread, namespace entry and deferred restoration each load the real netlink backend, LoadBackend closes the old counter socket and the replacement lazily opens in its lookup namespace; no fake resolver bypass. Inspected four native recorded producer-halt observations and fresh records. This proves Ze's configured socket/send/receive boundary, not Linux multicast forwarding (expressly not required by this section). No new judge test run.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFMulticastMembershipRefusesForeignGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_socket_linux_test.go#L97) | unit/verify | revert, verified |
| negative | [`TestOSPFTransportAllDRoutersReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_transport_integration_linux_test.go#L81) | unit/verify | revert, verified |
| positive | [`TestOSPFMulticastMembershipInstalled`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_socket_linux_test.go#L76) | unit/verify | revert, verified |
| positive | [`TestOSPFTransportVethMulticastRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_transport_integration_linux_test.go#L31) | unit/verify | revert, verified |

### [`RFC2328-4.4-2`](#rfc2328-4.4-2)

The router's IP protocol support must include the ability to divide a single IP class A, B, or C network number into many subnets of various sizes. This is commonly called variable-length subnetting; see Section 3.5 for details. IP supernetting support The router's IP protocol support must include the ability to aggregate contiguous collections of IP class A, B, and C networks into larger quantities called supernets. (§4.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a classful routing table (mask derived from address class) that cannot carry variable-length subnets or supernets. Red: TestRFC2328ClasslessPrefixesReachIPRIB fails unless 192.0.0.0/16 (a supernet of class-C networks), 192.0.2.0/24 and 192.0.2.128/25 (a variable-length subnet) each reach the IP Loc-RIB with its own next hop. Other polarity: TestRFC2328ClasslessWithdrawalPreservesOverlaps fails if withdrawing the /24 removes the /16 or the /25. Both the VLSM and the supernet clause have a prefix that a classful table would lose.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328ClasslessWithdrawalPreservesOverlaps`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_classless_test.go#L51) | unit/verify | unproven |
| positive | [`TestRFC2328ClasslessPrefixesReachIPRIB`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_classless_test.go#L34) | unit/verify | unproven |

### [`RFC2328-4.4-3`](#rfc2328-4.4-3)

Indications must be passed from these protocols to OSPF as the network interface goes up and down. (§4.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. c21 judge (independent). New positive TestRFC2328LinkIndicationsReachTheOSPFEngine builds the engine over the real transport.New(fakeBackend): an up neighbor on p2p eth0, then Transport.HandleLinkDown puts the OSPF interface in Down and the neighbor in down, and HandleLinkUp brings a running eth0 back to Point-to-point. Judge diffed the overlay that drops only instance.go OnInterfaceDown registration: red "state point-to-point, want Down"; dropping both registrations reds at setup while the transport-level TestOSPFTransportPassesLinkIndicationsToOSPF stays green, which was the old gap. Record on onInterfaceDown observed red. Transport-level positive kept. Negative TestOSPFTransportPassesNoIndicationForNonOSPFInterface: a non-OSPF interface link-up/down passes no indication, recorded.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFTransportPassesNoIndicationForNonOSPFInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_link_test.go#L45) | unit/verify | revert, verified |
| positive | [`TestRFC2328LinkIndicationsReachTheOSPFEngine`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_link_indication_test.go#L39) | unit/verify | revert, verified |
| positive | [`TestOSPFTransportPassesLinkIndicationsToOSPF`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_link_test.go#L8) | unit/verify | revert, verified |

### [`RFC2328-8.1-1`](#rfc2328-8.1-1)

On these interfaces, the IP source should be set to any of the other IP addresses belonging to the router. For this reason, there must be at least one IP address assigned to the router. (§8.1)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Re-judged 2026-10-02 under OWNER RULING 8 (g) (the stamping agent authored neither test). Section 8.1: 'Interfaces to unnumbered point-to-point networks have no associated IP address.  On these interfaces, the IP source should be set to any of the other IP addresses belonging to the router.' Ze does not implement unnumbered point-to-point interfaces: transport/backend_linux.go::interfaceIPv4 sources OSPF packets from the interface's own IPv4 address and refuses an interface without one, and no config or ifIndex LinkData exists for an unnumbered link, so no layer performs the sourcing rule. The row carries {gap} citing owner ruling 8 (g) and the rfc2328 Support remaining text discloses it. The two former tags (TestResolveOSPFInterfaceUsesIfaceResolverOSName +, TestOSPFRefusesInterfaceWithoutIPv4Source -) proved no clause of the quote and left the row under D-15 approvals citing owner ruling 8; the tests and their assertions stay as Ze's numbered-interface rule. Recording the gap authorizes no unnumbered implementation.

No test carries RFC2328-8.1-1, so no unit is bound to it.

### [`RFC2328-8.2-1`](#rfc2328-8.2-1)

In order for the packet to be accepted at the IP level, it must pass a number of tests, even before the packet is passed to OSPF for processing: o The IP checksum must be correct. o The packet's IP destination address must be the IP address of the receiving interface, or one of the IP multicast addresses AllSPFRouters or AllDRouters. o The IP protocol specified must be OSPF (89). (§8.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Each listed test has both polarities through deliverDatagram, the Linux receive path. IP checksum: the 'checksum' case fails if a datagram with a broken header checksum is delivered. Destination: accepted for the interface address, AllSPFRouters and AllDRouters (TestOSPFReceiveAcceptsValidIPv4Envelope fails if any is rejected or not delivered), 'destination' case fails if a foreign address is delivered. Protocol: 'protocol' case (17) fails if delivered, while the valid datagrams carry 89. Rejections also assert one drop recorded and nothing on recvCh.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFReceiveRejectsInvalidIPv4Envelope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_backend_linux_test.go#L157) | unit/verify | unproven |
| positive | [`TestOSPFReceiveAcceptsValidIPv4Envelope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/transport/rfc2328_backend_linux_test.go#L136) | unit/verify | unproven |

### [`RFC2328-8.2-2`](#rfc2328-8.2-2)

The Area ID found in the OSPF header must be verified. If both of the following cases fail, the packet should be discarded. The Area ID specified in the header must either: (1) Match the Area ID of the receiving interface. In this case, the packet has been sent over a single hop. Therefore, the packet's IP source address is required to be on the same network as the receiving interface. This can be verified by comparing the packet's IP source address to the interface's IP address, after masking both addresses with the interface mask. This comparison should not be performed on point-to-point networks. On point-to-point networks, the interface addresses of each end of the link are assigned independently, if they are assigned at all. (2) Indicate the backbone. In this case, the packet has been sent over a virtual link. The receiving router must be an area border router, and the Router ID specified in the packet (the source router) must be the other end of a configured virtual link. The receiving interface must also attach to the virtual link's configured Transit area. (§8.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (independent judge, c10/c11). Case (1) as the prior judgement: Area-ID clause TestOSPFReceiveAcceptsMatchingAreaID + / TestOSPFReceiveDropsMismatchedAreaID -; source clause under the interface mask TestOSPFReceiveDropsOffNetworkSourceOnBroadcast -, TestOSPFReceiveAcceptsOnNetworkSourceOnBroadcast +, point-to-point skip TestOSPFReceiveAcceptsOffNetworkSourceOnPointToPoint +. Case (2), the gap of the prior verdict, now proven through engine.acceptsArea on an ABR fixture (eth0 transit 0.0.0.1, eth1 backbone, eth2 0.0.0.2, VL to 10.0.0.2 resolved): TestRFC2328VirtualLinkAreaIDAccepted + (backbone header from the VL endpoint on the Transit-area interface accepted); TestRFC2328VirtualLinkAreaIDRefused - with one assertion per clause, each discriminated by a targeted overlay mutants (go test -overlay, tree untouched, scratch/ospf-judge-mut) of virtualLinkTargetLocked: dropping the backbone check -> red at the non-backbone 0.0.0.2 assertion; dropping the neighbor Router ID match -> red at the 9.9.9.9 assertion; dropping the Transit-area match -> red at the eth2 assertion. The recorded negative red is the panic stub; these mutants are the clause-level evidence. 'Receiving router must be an ABR' is structural: a configured virtual link attaches the router to the backbone, and no separate refusal path exists. Behaviour note (R42, documented in ospf-4-component-config.md): only the interface's first IPv4 address/prefix forms the OSPF interface, so a neighbour on a secondary subnet is dropped.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFReceiveDropsOffNetworkSourceOnBroadcast`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_receive_source_test.go#L68) | unit/verify | revert, verified |
| negative | [`TestOSPFReceiveDropsMismatchedAreaID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_receive_test.go#L65) | unit/verify | revert, verified |
| negative | [`TestRFC2328VirtualLinkAreaIDRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_virtual_area_test.go#L70) | unit/verify | revert, verified |
| positive | [`TestOSPFReceiveAcceptsOffNetworkSourceOnPointToPoint`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_receive_source_test.go#L97) | unit/verify | revert, verified |
| positive | [`TestOSPFReceiveAcceptsOnNetworkSourceOnBroadcast`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_receive_source_test.go#L83) | unit/verify | revert, verified |
| positive | [`TestOSPFReceiveAcceptsMatchingAreaID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_receive_test.go#L49) | unit/verify | revert, verified |
| positive | [`TestRFC2328VirtualLinkAreaIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_virtual_area_test.go#L52) | unit/verify | revert, verified |

### [`RFC2328-8.2-3`](#rfc2328-8.2-3)

The AuType specified in the packet must match the AuType specified for the associated area. (§8.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29 (independent judge). Isolation gap closed and a product defect fixed (D-8): TestRFC2328NullAuTypeIntoPasswordAreaDroppedForAuType asserts the single auth failure carries reason 'autype-mismatch', not a password failure; TestRFC2328PasswordAuTypeIntoNullAreaDropped sends a Hello signed AuType 1 into a Null-auth interface and asserts the drop and no neighbor (red before the fix: dropped 0). Producer checked: authStore.verify now refuses any non-Null AuType on an interface with no key chain, RFC 2328 s8.2 quoted above the check; revert records on verify. Positive TestOSPFReceiveAcceptsMatchingAuType unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328NullAuTypeIntoPasswordAreaDroppedForAuType`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_autype_test.go#L48) | unit/verify | revert, verified |
| negative | [`TestRFC2328PasswordAuTypeIntoNullAreaDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_autype_test.go#L72) | unit/verify | revert, verified |
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

Audit verdict: enforced (the tests do what the requirement demands), fresh. c20 judge: eligible router to every eligible neighbor (TestOSPFNBMAEligibleRouterHellosEveryEligibleNeighbor) and DROther skipping priority-0 neighbors (negative TestOSPFNBMAPeriodicHelloSkipsIneligibleNeighbor); DR and Backup sending to the priority-0 neighbor too (TestRFC2328NBMAHelloTargetsByRole dr/backup, red under an overlay gating priority-0 in every state). Ineligible-router clause: the subtest asserts a priority-0 DROther targets both DR and BDR; no producer branch reads the router's own priority (it sends to every eligible neighbor, a superset the sentence allows), so no targeted overlay exists; the assertion fails on any producer that drops DR or BDR from that router's targets, and the unit's revert record is observed red. Single positive for that clause is the sentence's own shape (no MUST NOT).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFNBMAPeriodicHelloSkipsIneligibleNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc2328_dr_test.go#L92) | unit/verify | revert, verified |
| positive | [`TestOSPFNBMAEligibleRouterHellosEveryEligibleNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc2328_dr_test.go#L75) | unit/verify | revert, verified |
| positive | [`TestRFC2328NBMAHelloTargetsByRole`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc2328_nbma_hello_roles_test.go#L15) | unit/verify | revert, verified |

### [`RFC2328-10.5-1`](#rfc2328-10.5-1)

Next, the values of the Network Mask, HelloInterval, and RouterDeadInterval fields in the received Hello packet must be checked against the values configured for the receiving interface. Any mismatch causes processing to stop and the packet to be dropped. (§10.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting a Hello whose Network Mask, HelloInterval or RouterDeadInterval differs from the receiving interface. Red: TestRFC2328HelloMismatchRejectedAndNoTwoWayUnlisted mutates each of the three fields separately and fails unless ReceiveHello returns that field's drop reason and no neighbor is created (processing stopped). Other polarity: TestRFC2328HelloMatchingParametersAndTwoWay fails if a matching Hello is refused. Every listed field has its own case.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328HelloMismatchRejectedAndNoTwoWayUnlisted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc2328_hello_test.go#L35) | unit/verify | revert, verified |
| positive | [`TestRFC2328HelloMatchingParametersAndTwoWay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc2328_hello_test.go#L16) | unit/verify | revert, verified |

### [`RFC2328-10.6-1`](#rfc2328-10.6-1)

In states Loading and Full the slave must resend its last Database Description packet in response to duplicate Database Description packets received from the master. For this reason the slave must wait RouterDeadInterval seconds before freeing the last Database Description packet. (§10.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged c18. Clauses: (1) in Loading and Full the slave resends its last DD for a duplicate of the master's DD, (2) it keeps that DD RouterDeadInterval before freeing. Positive TestRFC2328SlaveResendsLastDDInLoadingAndFull (Loading and Full subtests): a duplicate of the master's last DD at 0 s and at RouterDeadInterval-1 s (table clock, aligned with the exchange at Unix(1,0)) gets exactly one packet, the slave's last DD (sameDD), state unchanged. Negative TestRFC2328SlaveRepeatsOnlyForTheLastMasterDD: the master's earlier Init DD 99 or a new DD 101 in Loading/Full -> SeqNumberMismatch, ExStart, next DD Init, never the slave's last DD. Records: + resendLastDDLocked, - sameDD and handleExchangeDDLocked, observed red. Retention read at the producer: lastSentDD is assigned only in dd.go and reset only on exchange restart/teardown, never on a timer, so no early-free path exists; the -1 s probe catches a time check on the receive path, not a timer-driven free, which no code has.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328SlaveRepeatsOnlyForTheLastMasterDD`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_dd_slave_after_exchange_test.go#L103) | unit/verify | revert, verified |
| negative | [`TestRFC2328SlaveRefusesOutOfSequenceDD`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_dd_slave_test.go#L62) | unit/verify | revert, verified |
| positive | [`TestRFC2328SlaveResendsLastDDInLoadingAndFull`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_dd_slave_after_exchange_test.go#L72) | unit/verify | revert, verified |
| positive | [`TestRFC2328SlaveRepliesAndRepeatsOnDuplicate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_dd_slave_test.go#L30) | unit/verify | revert, verified |

### [`RFC2328-12.1.6-1`](#rfc2328-12.1.6-1)

When an attempt is made to increment the sequence number past the maximum value of N - 1 (0x7fffffff; also referred to as MaxSequenceNumber), the current instance of the LSA must first be flushed from the routing domain. This is done by prematurely aging the LSA (see Section 14.1) and reflooding it. As soon as this flood has been acknowledged by all adjacent neighbors, a new instance can be originated with sequence number of InitialSequenceNumber. (§12.1.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged c18. Clauses: (1) flush by premature aging AND reflooding, (2) wait for ALL adjacent neighbors' acks, (3) then originate at InitialSequenceNumber. Positive TestRFC2328SequenceWrapFloodsFlushThenRestarts: the flush is decoded from LS Updates at MaxAge + MaxSequenceNumber out of eth0 AND eth1; after 2.2.2.2 and 3.3.3.3 both ack it is deleted; next origination is InitialSequenceNumber age 0 flooded out of both. Negative TestRFC2328SequenceWrapWaitsForEveryNeighbor (subtest per neighbor): one ack alone leaves it in the database, the next origination is the MaxAge instance at MaxSequenceNumber, and no InitialSequenceNumber instance is flooded, so a restart after the first ack goes red. Records: nextOwnSequenceForce and deletePurgedIfAcked, both polarities, observed red. Old flooding_test tags kept.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328SequenceWrapWaitsForAck`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L194) | unit/verify | revert, verified |
| negative | [`TestRFC2328SequenceWrapWaitsForEveryNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_sequence_wrap_test.go#L100) | unit/verify | revert, verified |
| positive | [`TestRFC2328SequenceWrapRestartsAfterAck`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L217) | unit/verify | revert, verified |
| positive | [`TestRFC2328SequenceWrapFloodsFlushThenRestarts`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_sequence_wrap_test.go#L69) | unit/verify | revert, verified |

### [`RFC2328-12.2-1`](#rfc2328-12.2-1)

An implementation of OSPF must be able to access individual pieces of an area database. This lookup function is based on an LSA's LS type, Link State ID and Advertising Router. (§12.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a lookup that ignores any element of the (LS type, Link State ID, Advertising Router) triple. + TestRFC2328LookupTellsApartLSAsSharingTwoElements: three LSAs from 4.4.4.4 (Router 4.4.4.4; Summary 4.4.4.4 differing only in type; Summary 9.9.9.9 differing only in LSID) each found by its own triple with its own sequence. - TestRFC2328LookupMissesWhenOneElementDiffers: a triple differing in LS type alone, LSID alone, or Advertising Router alone finds nothing. Records on lsdb.go::Lookup observed red; TestRFC2328LookupByTriple/NeedsFullTriple unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328LookupNeedsFullTriple`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L174) | unit/verify | revert, verified |
| negative | [`TestRFC2328LookupMissesWhenOneElementDiffers`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_lookup_triple_test.go#L73) | unit/verify | revert, verified |
| positive | [`TestRFC2328LookupByTriple`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L160) | unit/verify | revert, verified |
| positive | [`TestRFC2328LookupTellsApartLSAsSharingTwoElements`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_lookup_triple_test.go#L47) | unit/verify | revert, verified |

### [`RFC2328-12.4-1`](#rfc2328-12.4-1)

If a router advertises a summary-LSA for a destination which then becomes unreachable, the router must then flush the LSA from the routing domain by setting its age to MaxAge and reflooding (see Section 14.1). (§12.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged c18. Forbidden: keeping a self-originated summary-LSA for a destination that became unreachable. TestRFC2328SummaryFlushedWhenDestinationUnreachable drives OriginateSummaries with a real lsdb.LSDB as sink: pass 1 with 2.2.2.2 in the backbone SPF Result originates the Type 3 for 10.20.0.0/24 into 0.0.0.1; pass 2 with 2.2.2.2 unreached sets it to MaxAge AND a decoded LS Update carries it at MaxAge out of eth1, while the reachable 10.10.0.0/24 summary stays below MaxAge. The step from unreachable destination to keep-set exit (collectSummaryNetworks) is now exercised. Negative TestRFC2328SummaryKeptWhileDestinationReachable: two reachable passes leave it below MaxAge with no MaxAge copy flooded. Records: FlushStaleSummaryLSAs both polarities, collectSummaryNetworks negative, observed red. Unreachability is a hand-built SPF Result, not a full SPF run. Old lsdb-seam tags kept.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328AdvertisedSummaryNotFlushed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L286) | unit/verify | revert, verified |
| negative | [`TestRFC2328SummaryKeptWhileDestinationReachable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_unreachable_summary_test.go#L120) | unit/verify | revert, verified |
| positive | [`TestRFC2328WithdrawnSummaryFlushedAtMaxAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L249) | unit/verify | revert, verified |
| positive | [`TestRFC2328SummaryFlushedWhenDestinationUnreachable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_unreachable_summary_test.go#L94) | unit/verify | revert, verified |

### [`RFC2328-12.4.1-1`](#rfc2328-12.4.1-1)

All of the router's links to the area must be described in a single router-LSA. (§A.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: splitting the area's links over several router-LSAs, or omitting one. Red: TestRFC2328SingleRouterLSACarriesEveryLink fails unless exactly one router-LSA from 1.1.1.1 exists in the area and its body holds a transit link for each of the two interfaces. Other polarity: TestRFC2328RouterLSAExcludesOtherAreaLinks fails if an interface of area 0.0.0.1 is described in the backbone router-LSA, so 'links to the area' is bounded. One clause, both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328RouterLSAExcludesOtherAreaLinks`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L350) | unit/verify | revert, verified |
| positive | [`TestRFC2328SingleRouterLSACarriesEveryLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L318) | unit/verify | revert, verified |

### [`RFC2328-12.4.3-1`](#rfc2328-12.4.3-1)

The router must then originate summary-LSAs into the newly attached area for all pertinent intra-area and inter-area routes in the router's routing table. (§12.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (independent judge, ospf c27). Section 12.4 event (7): 'The router must then originate summary-LSAs into the newly attached area for all pertinent intra-area and inter-area routes in the router's routing table.' New spf/rfc2328_newly_attached_area_test.go: real LSDB as Source and SummarySink, NewComputer, root 1.1.1.1 p2p to ABR 2.2.2.2; Run with area 0.0.0.1 configured but no Router-LSA of 1.1.1.1 in it (asserted: area holds 0 LSAs), then the area-1 Router-LSA installed (attachment; production triggers SPF via triggerSPF on the LSDB change) and Run again. + TestRFC2328NewlyAttachedAreaGetsIntraAndInterAreaSummaries: Type 3 for backbone intra-area 10.10.0.0/24 metric 7 and inter-area 10.30.0.0/24 metric 30 in area 1. - TestRFC2328NewlyAttachedAreaGetsNoSummaryOfItsOwnRoutes ('pertinent', section 12.4.3): no Type 3 for area 1's own 10.20.0.0/24 into area 1 (present in the backbone, metric 3), no inter-area summary into the backbone. Judge replayed overlays on summaryDesiredForArea: inter-area loop off -> only + red; src==dst skip off -> only - red; inter-area into the backbone -> only - red. Revert records on summaryDesiredForArea observed. HEAD units (range condensation) keep their records.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328ABRComponentNeverEscapesRange`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_abr_summary_test.go#L101) | unit/verify | revert, verified |
| negative | [`TestRFC2328NewlyAttachedAreaGetsNoSummaryOfItsOwnRoutes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_newly_attached_area_test.go#L106) | unit/verify | revert, verified |
| positive | [`TestRFC2328ABRSummarizesAndCondenses`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_abr_summary_test.go#L67) | unit/verify | revert, verified |
| positive | [`TestRFC2328NewlyAttachedAreaGetsIntraAndInterAreaSummaries`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_newly_attached_area_test.go#L82) | unit/verify | revert, verified |

### [`RFC2328-13-5`](#rfc2328-13-5)

Remove the current database copy from all neighbors' Link state retransmission lists. (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC2328ReplacedInstanceLeavesSenderRetransmitList queues the old instance for 3.3.3.3, then the newer instance arrives FROM 3.3.3.3 so no flood can overwrite that entry; after install the list is empty for the key and a retransmit tick sends nothing on eth1. A no-op removeFromAllRetransmit turns it red while the older positive stays green. Negative: a duplicate leaves other lists intact (unchanged).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328DuplicateKeepsOtherRetransmitLists`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L144) | unit/verify | revert, verified |
| positive | [`TestRFC2328ReplacedInstanceLeavesRetransmitLists`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L110) | unit/verify | revert, verified |
| positive | [`TestRFC2328ReplacedInstanceLeavesSenderRetransmitList`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_replaced_retransmit_test.go#L18) | unit/verify | revert, verified |

### [`RFC2328-13-6`](#rfc2328-13-6)

If the contents are different, the following pieces of the routing table must be recalculated, depending on the new LSA's LS type field: Router-LSAs and network-LSAs The entire routing table must be recalculated, starting with the shortest path calculations for each area (not just the area whose link-state database has changed). (§13.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. c21 judge (independent). Positive TestRFC2328ChangedContentsRecalculateEveryArea drives the production wiring (newEngine -> initSPF: lsdb.SetOnChange -> Computer.TriggerArea -> Run), nothing triggers by hand: a changed router-LSA and a changed network-LSA installed in area 0.0.0.1 each withdraw the neighbor stub 203.0.113.64/26 and move BOTH areas last-run stamps past the settled baseline; not-advertised ranges keep the ABR summary origination from triggering the backbone, so only the 13.2 whole-table behaviour recalculates area 0 (isolated). Judge re-ran the dirty-only Computer.Run overlay: both subtests red "area 0.0.0.0 was not recalculated"; record on TriggerArea observed red. Run is monolithic (every configured area, then summary and external steps), so the entire table follows. TestSPFRecalculatesEveryAreaOnOneAreaChange stays as unit-level supplement. Negative TestSPFRecalculationBoundedToConfiguredAreas bounds the recalculation to configured areas; 13.2 offers no refusal path (unchanged contents is a permission, not an obligation), so the row is judged on its positive, the negative held as supplementary.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSPFRecalculationBoundedToConfiguredAreas`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_recalc_test.go#L47) | unit/verify | revert, verified |
| positive | [`TestRFC2328ChangedContentsRecalculateEveryArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_contents_change_recalc_test.go#L113) | unit/verify | revert, verified |
| positive | [`TestSPFRecalculatesEveryAreaOnOneAreaChange`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_recalc_test.go#L14) | unit/verify | revert, verified |

### [`RFC2328-13-7`](#rfc2328-13-7)

If there is already a database copy, and if the database copy was received via flooding and installed less than MinLSArrival seconds ago, discard the new LSA (without acknowledging it) and examine the next LSA (if any) listed in the Link State Update packet. (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. c20 judge: discard without ack and keep the earlier copy proven by TestRFC2328MinLSArrivalDiscardsWithoutAck (negative) and the elapsed case by TestRFC2328MinLSArrivalElapsedAccepts; the 'examine the next LSA' clause now by TestRFC2328TooSoonDiscardThenNextLSAExamined: one LS Update [too-soon 4.4.4.4, new 5.5.5.5], 4.4.4.4 keeps its sequence and is not flooded, 5.5.5.5 installed, flooded and the ONLY header acknowledged. Author overlay (continue->return on the TooSoon branch) red on the new unit only; revert record on ReceiveUpdate.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328MinLSArrivalDiscardsWithoutAck`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L44) | unit/verify | revert, verified |
| positive | [`TestRFC2328MinLSArrivalElapsedAccepts`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L79) | unit/verify | revert, verified |
| positive | [`TestRFC2328TooSoonDiscardThenNextLSAExamined`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_too_soon_next_lsa_test.go#L17) | unit/verify | revert, verified |

### [`RFC2328-13.3-3`](#rfc2328-13.3-3)

On non-broadcast networks, separate Link State Update packets must be sent, as unicasts, to each adjacent neighbor (i.e., those in state Exchange or greater). (§13.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: on a non-broadcast network, flooding by multicast or omitting a unicast to an adjacent (Exchange or greater) neighbor. Red: TestRFC2328NBMAUnicastsToEachAdjacency fails unless exactly one LS Update goes to each of 10.0.0.2 (Full) and 10.0.0.3 (Exchange); TestRFC2328NBMANeverMulticastsNorReachesTwoWay fails on any send to AllSPFRouters or AllDRouters, or to the 2-Way neighbor 10.0.0.4. Both polarities over floodExcept, the flooding producer.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328NBMANeverMulticastsNorReachesTwoWay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L464) | unit/verify | revert, verified |
| positive | [`TestRFC2328NBMAUnicastsToEachAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_flooding_test.go#L430) | unit/verify | revert, verified |

### [`RFC2328-15-2`](#rfc2328-15-2)

The cost of a virtual link is NOT configured. It is defined to be the cost of the intra-area path between the two defining area border routers. This cost appears in the virtual link's corresponding routing table entry. When the cost of a virtual link changes, a new router-LSA should be originated for the backbone area. (§15)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (independent judge, c10/c11). Prior clauses unchanged: cost = intra-area path cost carried by the routing table entry (spf/TestRFC2328VirtualCostIsTransitPathCost), a cost change originates a new backbone router-LSA (TestRFC2328VirtualCostChangeOriginatesNewInstance, TestRFC2328VirtualCostChangeReoriginatesBackbone +, TestRFC2328VirtualUnusablePathWithdrawsBackboneLink -). The missing MUST clause 'The cost of a virtual link is NOT configured' now has negatives on both virtual-link lists through the operator path (config text -> config.ParseTreeWithYANG over ZeOSPFConfYANG -> ToPluginMap -> parseOSPFConfig): TestRFC2328VirtualLinkCostNotConfigurable (ospf areas area virtual-link) and TestRFC2328VirtualLinkCostNotConfigurableV3 (address-family ipv6, ospf-af-topology grouping), each accepting the link without cost (reaches cfg.VirtualLinks / cfg.V6.VirtualLinks) and refusing `cost 10` with an error naming cost. Their records are no-break declaration-only (the tool cannot mutate an embedded YANG file); the judge observed the red with an overlay mutants (go test -overlay, tree untouched, scratch/ospf-judge-mut) adding `leaf cost { type uint16; }` to both lists of ze-ospf-conf.yang: both units red at the refusal assertion.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328VirtualLinkCostNotConfigurable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_virtual_cost_config_test.go#L69) | unit/verify | no-break escape (declaration-only), which is not a proof, verified |
| negative | [`TestRFC2328VirtualLinkCostNotConfigurableV3`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_virtual_cost_config_test.go#L139) | unit/verify | no-break escape (declaration-only), which is not a proof, verified |
| negative | [`TestRFC2328VirtualUnusablePathWithdrawsBackboneLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_virtual_route_test.go#L98) | unit/verify | unproven |
| positive | [`TestRFC2328VirtualCostChangeOriginatesNewInstance`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_virtual_cost_test.go#L30) | unit/verify | revert, verified |
| positive | [`TestRFC2328VirtualCostChangeReoriginatesBackbone`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_virtual_route_test.go#L67) | unit/verify | unproven |
| positive | [`TestRFC2328VirtualCostIsTransitPathCost`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_virtual_cost_test.go#L15) | unit/verify | revert, verified |

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both halves now asserted. + TestOSPFElectedDRJoinsAllDRouters: priority 5 vs 2 -> StateDR and exactly one JoinAllDRouters(eth0), no leave; TestOSPFElectedBDRJoinsAllDRouters keeps the BDR join. - the same unit: priority dropped to 0, re-election -> DROther and LeaveAllDRouters(eth0); TestOSPFDROtherDoesNotJoinAllDRouters keeps the never-joined case. Records on iface.go::runElectionLocked observed red; author's overlay making membership BDR-only turned the DR positive red. Judge lint fix (unparam on the shared electWith helper): the unit's neighbour priority is 2, not 1; records re-recorded.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFElectedDRJoinsAllDRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc2328_dr_alldrouters_test.go#L44) | unit/verify | revert, verified |
| negative | [`TestOSPFDROtherDoesNotJoinAllDRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc2328_dr_test.go#L46) | unit/verify | revert, verified |
| positive | [`TestOSPFElectedDRJoinsAllDRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/rfc2328_dr_alldrouters_test.go#L29) | unit/verify | revert, verified |
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
