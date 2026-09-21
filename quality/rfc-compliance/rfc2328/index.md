# RFC 2328 - OSPF Version 2

Partial. Every requirement this repository extracted from RFC 2328, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 47.1% | 24 of 51 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 51 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 51 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 59 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 51 | of 64 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 51 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 51 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 51 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 51 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 52.9% | 27 of 51 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 51 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 64 |
| Gated MUST-level | 51 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Gated with no test | 26 |
| Nightly-only evidence | 0 |
| Test tags | 61 |
| Tagged units | 59 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
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

The MUST gap recorded for [`RFC2328-13.3-2`](#rfc2328-13.3-2) stands: the InfTransDelay increment of LS age is applied on retransmission ([`internal/plugins/ospf/lsdb/flooding.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding.go)) and on a direct database-copy reply ([`internal/plugins/ospf/lsdb/flooding.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding.go)), but the normal flood path copies the LSA with transmit delay 0 (`floodExcept` -> `entry.LSA(d.now())` -> `Raw(now, 0)`, [`internal/plugins/ospf/lsdb/flooding.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding.go) and [`internal/plugins/ospf/lsdb/entry.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/entry.go)), so a first-flooded LSA carries an unincremented age. The extraction walk of 2026-09-21 added 26 MUST rows the checklist did not carry ([`RFC2328-3.6-1`](#rfc2328-3.6-1), 4.4-1 to 4.4-3, 8.1-1, 8.2-1 to 8.2-3, 9.1-1, 9.5.1-1, 10.5-1, 10.6-1, 12.1.6-1, 12.2-1, 12.4-1, 12.4.1-1, 12.4.3-1, 13-5 to 13-7, 13.3-3, 15-2, 16.1-3, A.1-1, A.1-2, A.4.4-1); each states an obligation RFC 2328 writes and none carries a tagged test yet, so they are untested MUSTs on this ledger. The feature also remains pre-production pending hardening and deployment evidence.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 24 | one part of the gated population |
| Annotated instead of tested | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 26 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **51** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (24):** [`RFC2328-A.3.1-1`](#rfc2328-a.3.1-1), [`RFC2328-A.3.1-2`](#rfc2328-a.3.1-2), [`RFC2328-12.1.7-1`](#rfc2328-12.1.7-1), [`RFC2328-13-1`](#rfc2328-13-1), [`RFC2328-13-2`](#rfc2328-13-2), [`RFC2328-13-3`](#rfc2328-13-3), [`RFC2328-13.1-1`](#rfc2328-13.1-1), [`RFC2328-13-4`](#rfc2328-13-4), [`RFC2328-13.3-1`](#rfc2328-13.3-1), [`RFC2328-14-1`](#rfc2328-14-1), [`RFC2328-14-2`](#rfc2328-14-2), [`RFC2328-13.5-1`](#rfc2328-13.5-1), [`RFC2328-13.4-1`](#rfc2328-13.4-1), [`RFC2328-16.1-1`](#rfc2328-16.1-1), [`RFC2328-16.4-1`](#rfc2328-16.4-1), [`RFC2328-16.2-1`](#rfc2328-16.2-1), [`RFC2328-16.2-2`](#rfc2328-16.2-2), [`RFC2328-D.2-1`](#rfc2328-d.2-1), [`RFC2328-D.3-1`](#rfc2328-d.3-1), [`RFC2328-D.3-2`](#rfc2328-d.3-2), [`RFC2328-A.3.3-1`](#rfc2328-a.3.3-1), [`RFC2328-10.1-1`](#rfc2328-10.1-1), [`RFC2328-10.2-1`](#rfc2328-10.2-1), [`RFC2328-C.3-1`](#rfc2328-c.3-1)

**Annotated instead of tested (1):** [`RFC2328-13.3-2`](#rfc2328-13.3-2)

**No test and no annotation (26):** [`RFC2328-3.6-1`](#rfc2328-3.6-1), [`RFC2328-4.4-1`](#rfc2328-4.4-1), [`RFC2328-4.4-2`](#rfc2328-4.4-2), [`RFC2328-4.4-3`](#rfc2328-4.4-3), [`RFC2328-8.1-1`](#rfc2328-8.1-1), [`RFC2328-8.2-1`](#rfc2328-8.2-1), [`RFC2328-8.2-2`](#rfc2328-8.2-2), [`RFC2328-8.2-3`](#rfc2328-8.2-3), [`RFC2328-9.1-1`](#rfc2328-9.1-1), [`RFC2328-9.5.1-1`](#rfc2328-9.5.1-1), [`RFC2328-10.5-1`](#rfc2328-10.5-1), [`RFC2328-10.6-1`](#rfc2328-10.6-1), [`RFC2328-12.1.6-1`](#rfc2328-12.1.6-1), [`RFC2328-12.2-1`](#rfc2328-12.2-1), [`RFC2328-12.4-1`](#rfc2328-12.4-1), [`RFC2328-12.4.1-1`](#rfc2328-12.4.1-1), [`RFC2328-12.4.3-1`](#rfc2328-12.4.3-1), [`RFC2328-13-5`](#rfc2328-13-5), [`RFC2328-13-6`](#rfc2328-13-6), [`RFC2328-13-7`](#rfc2328-13-7), [`RFC2328-13.3-3`](#rfc2328-13.3-3), [`RFC2328-15-2`](#rfc2328-15-2), [`RFC2328-16.1-3`](#rfc2328-16.1-3), [`RFC2328-A.1-1`](#rfc2328-a.1-1), [`RFC2328-A.1-2`](#rfc2328-a.1-2), [`RFC2328-A.4.4-1`](#rfc2328-a.4.4-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2328-A.3.1-1` | All OSPF packets begin with the standard 24-byte header; Version # MUST be 2 (§A.3.1) | MUST | A.3.1 | **positive:** `unit/verify` [`TestOSPFHeaderRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/header_test.go#L131). **negative:** `unit/verify` [`TestOSPFHeaderRejectsBadVersionAndLength`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/header_test.go#L153) |
| `RFC2328-A.3.1-2` | Compute the packet header IP checksum over the whole packet excluding the 64-bit authentication field (§A.3.1, §D.4) | MUST | A.3.1 | **positive:** `unit/verify` [`TestOSPFPacketChecksumExcludesAuth`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L32). **negative:** `unit/verify` [`TestPacketVerifyChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/header_test.go#L100) |
| `RFC2328-12.1.7-1` | Compute the LS (Fletcher) checksum over the complete LSA excluding the LS age field; the LS checksum MUST NOT be zero (calculation is not optional) (§12.1.7) | MUST | 12.1.7 | **positive:** `unit/verify` [`TestOSPFLSAChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L75). **positive:** `unit/verify` [`TestOSPFLSAChecksumExcludesAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L102). **negative:** `unit/verify` [`TestRFC2328ZeroLSChecksumRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc2328_test.go#L10) |
| `RFC2328-13-1` | In the flooding procedure, discard an LSA with an invalid LS checksum and discard an LSA of unknown LS type (only types 1-5 are defined) (§13) | MUST | 13 | **positive:** `unit/verify` [`TestOSPFFloodOutOtherInterfaces`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L30). **negative:** `unit/verify` [`TestDecodeLSReqRejectsMalformed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/packet_body_test.go#L145). **negative:** `unit/verify` [`TestRFC2328BadLSChecksumDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_test.go#L17) |
| `RFC2328-13-2` | Flood AS-external (Type 5) LSAs into or throughout a stub area (§13, §3.6) | MUST NOT | 13 | **positive:** `unit/verify` [`TestOSPFStubFloodFilter`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/area_type_test.go#L15). **negative:** `unit/verify` [`TestOSPFStubAreaDropsType5`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L247) |
| `RFC2328-13-3` | Drop a Link State Update / Acknowledgment from a neighbor in a state lesser than Exchange (§13, §13.7) | MUST | 13 | **positive:** `unit/verify` [`TestRFC2328FloodingRequiresExchangeOrHigher`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L17). **negative:** `unit/verify` [`TestRFC2328FloodingRequiresExchangeOrHigher`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L20) |
| `RFC2328-13.1-1` | Determine the more recent of two LSA instances using LS sequence number, then larger LS checksum, then MaxAge, then younger LS age beyond MaxAgeDiff (§13.1) | MUST | 13.1 | **positive:** `unit/verify` [`TestOSPFFreshnessCompareMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/lsdb_test.go#L126). **negative:** `unit/verify` [`TestRFC2328OlderInstanceGetsDatabaseCopyBack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_test.go#L50) |
| `RFC2328-13-4` | If the database copy is MaxAge with LS sequence number MaxSequenceNumber, discard a received older instance without acknowledging (§13, step 8) | MUST | 13 | **positive:** `unit/verify` [`TestOSPFMaxSeqMaxAgeSilentDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_edges_test.go#L19). **negative:** `unit/verify` [`TestRFC2328OlderInstanceGetsDatabaseCopyBack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_test.go#L53) |
| `RFC2328-13.3-1` | Add an LSA flooded out an adjacency to that adjacency's Link state retransmission list and retransmit at RxmtInterval until acknowledged (§13.3, §13.6) | MUST | 13.3 | **positive:** `unit/verify` [`TestOSPFFloodOutOtherInterfaces`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L31). **positive:** `unit/verify` [`TestOSPFRetransmitTimer`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L129). **negative:** `unit/verify` [`TestOSPFFloodQueuesExchangeAndLoadingNeighbors`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L57) |
| `RFC2328-13.3-2` | Increment an LSA's LS age by InfTransDelay (which MUST be > 0) when copying it into an outgoing Link State Update, capped at MaxAge (§13.3, §13.6, §14) | MUST | 13.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the InfTransDelay bump is applied on the retransmit path (RetransmitTick, lsdb/flooding.go:545) and on a direct database-copy reply (sendDirectLSUpdate, lsdb/flooding.go:745; sendDirectLinkLSUpdate, lsdb/link_scope.go:349), but NOT on the normal flood: floodExcept builds the outgoing copy with `entry.LSA(d.now())` (lsdb/flooding.go:351), and Entry.LSA calls `e.Raw(now, 0)` with transmitDelay 0 (lsdb/entry.go:62-68, 75-85), so the first flooded copy carries the unincremented LS age. The MaxAge cap itself is present wherever the bump is applied (LSAge.Add, types/lsage.go:54-63). Disclosed in docs/features/rfc-status.md RFC 2328 row |
| `RFC2328-14-1` | Never increment an LSA's LS age past MaxAge, and exclude MaxAge LSAs from the routing-table calculation (§14) | MUST | 14 | **positive:** `unit/verify` [`TestLSAgeAddSaturates`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/lsage_test.go#L39). **positive:** `unit/verify` [`TestOSPFLSDBAgeToPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/aging_test.go#L34). **negative:** `unit/verify` [`TestOSPFGraphSkipsMaxAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/graph_test.go#L40) |
| `RFC2328-14-2` | Remove a MaxAge LSA from the database only once it is on no neighbor retransmission list and no neighbor is in Exchange or Loading (§14) | MUST | 14 | **positive:** `unit/verify` [`TestOSPFASExternalPurgeRetainedAcrossAreas`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L332). **negative:** `unit/verify` [`TestOSPFPurgeRetainedForExchangeOrLoading`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L383) |
| `RFC2328-13.5-1` | Acknowledge every newly received LSA (directly or implicitly per Table 19) (§13.5) | MUST | 13.5 | **positive:** `unit/verify` [`TestOSPFAckDecisionTable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L93). **positive:** `unit/verify` [`TestOSPFUnknownMaxAgeNoCopyIsAckedAndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L360). **negative:** `unit/verify` [`TestOSPFDRRefloodsBackOutReceivingInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_edges_test.go#L80) |
| `RFC2328-13.4-1` | Detect self-originated LSAs by Advertising Router == own Router ID, or network-LSA Link State ID == own interface address, and re-originate or flush via premature aging (§13.4, §14.1) | MUST | 13.4 | **positive:** `unit/verify` [`TestOSPFOriginateSelfReceivedHigherSeq`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/origination_test.go#L426). **negative:** `unit/verify` [`TestOSPFSelfOriginatedNoLocalCopyFlush`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_edges_test.go#L43) |
| `RFC2328-16.1-1` | In intra-area SPF, include a transit-vertex link only if the neighbor LSA exists, is not MaxAge, and has a link back to the current vertex (two-way check) (§16.1) | MUST | 16.1 | **positive:** `unit/verify` [`TestOSPFSPFShortestPath`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/spf_test.go#L13). **negative:** `unit/verify` [`TestOSPFTwoWayCheck`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/spf_test.go#L31) |
| `RFC2328-16.4-1` | Prefer intra-area and inter-area paths over AS-external paths; prefer Type 1 external over Type 2; among Type 2 prefer the smallest type-2 metric (§16.4) | MUST | 16.4 | **positive:** `unit/verify` [`TestOSPFRouteTablePreference`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/route_test.go#L8). **negative:** `unit/verify` [`TestOSPFExternalE1PreferredOverE2`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external_test.go#L83) |
| `RFC2328-16.2-1` | As an ABR, examine only backbone summary-LSAs when computing inter-area routes (§16.2) | MUST | 16.2 | **positive:** `unit/verify` [`TestOSPFInterAreaRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/interarea_test.go#L43). **negative:** `unit/verify` [`TestOSPFABRBackboneOnlyAcceptance`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/interarea_test.go#L74) |
| `RFC2328-16.2-2` | Skip a summary-LSA or AS-external-LSA whose cost is LSInfinity, whose LS age is MaxAge, or that is self-originated, during the routing calculation (§16.2, §16.4) | MUST | 16.2 | **positive:** `unit/verify` [`TestOSPFInterAreaRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/interarea_test.go#L44). **negative:** `unit/verify` [`TestOSPFExternalLSInfinityDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external_test.go#L61). **negative:** `unit/verify` [`TestOSPFInterAreaLSInfinityDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/interarea_test.go#L137). **negative:** `unit/verify` [`TestRFC2328ExternalSkipsMaxAgeAndSelf`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_test.go#L43). **negative:** `unit/verify` [`TestRFC2328InterAreaSkipsMaxAgeAndSelfSummary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_test.go#L20) |
| `RFC2328-D.2-1` | Discard a packet whose Simple-password (AuType 1) authentication field does not match the configured 64-bit password (§D.2, §D.5) | MUST | D.2 | **positive:** `unit/verify` [`TestRFC2328SimplePasswordMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_test.go#L94). **negative:** `unit/verify` [`TestRFC2328SimplePasswordMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_test.go#L98) |
| `RFC2328-D.3-1` | For Cryptographic auth (AuType 2), set the header checksum to 0, append the message digest (16 bytes for MD5), and exclude the digest from the OSPF header packet length while including it in the IP length (§D.3, §D.4.3) | MUST | D.3 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L46). **negative:** `unit/verify` [`TestOSPFAuthCryptoRejectsExtraTrailerBytes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L214) |
| `RFC2328-D.3-2` | Treat the crypto sequence number as non-decreasing, reset it to 0 when the neighbor goes Down, and set it to a received packet's value when accepted as authentic (§D.3) | MUST | D.3 | **positive:** `unit/verify` [`TestNeighborDownResetsCryptoSeq`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L202). **positive:** `unit/verify` [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L115). **negative:** `unit/verify` [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L116) |
| `RFC2328-A.3.3-1` | Set Interface MTU to 0 in Database Description packets sent over virtual links (§A.3.3) | MUST | A.3.3 | **positive:** `unit/verify` [`TestRFC2328VirtualInterfaceHasNoMTU`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_test.go#L32). **positive:** `unit/verify` [`TestRFC2328VirtualLinkDBDescCarriesZeroMTU`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L58). **negative:** `unit/verify` [`TestRFC2328VirtualLinkDBDescCarriesZeroMTU`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L63) |
| `RFC2328-10.1-1` | Allow only one Database Description packet outstanding per adjacency at a time (§10.1, §10.3) | MUST | 10.1 | **positive:** `unit/verify` [`TestOSPFDDRetransmit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/nsm_test.go#L363). **negative:** `unit/verify` [`TestOSPFDuplicateDD`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/nsm_test.go#L431) |
| `RFC2328-10.2-1` | Generate the BadLSReq event and restart the Database Exchange when an LS Request names an LSA not in the database (§10.2, §13) | MUST | 10.2 | **positive:** `unit/verify` [`TestOSPFBadLSReqRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/nsm_test.go#L566). **negative:** `unit/verify` [`TestOSPFValidLSReqSendsLSUpdate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/nsm_test.go#L721). **negative:** `unit/verify` [`TestRFC2328KnownLSRequestDoesNotRestartExchange`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L121) |
| `RFC2328-C.3-1` | Use a positive Interface output cost (greater than 0) (§C.3) | MUST | C.3 | **positive:** `unit/verify` [`TestInterfaceCostAndTransmitDelayBoundary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_interface_validate_test.go#L16). **negative:** `unit/verify` [`TestInterfaceCostAndTransmitDelayBoundary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_interface_validate_test.go#L17) |
| `RFC2328-3.6-1` | One or more of a stub area's area border routers advertise a default route into the stub area via summary-LSAs (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-4.4-1` | Support receiving and sending IP multicast datagrams, with the appropriate lower-level protocol support (§4.4) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-4.4-2` | The router's IP support includes variable-length subnetting (dividing one class A, B or C network into subnets of various sizes) and IP supernetting (aggregating contiguous class A, B and C networks into supernets) (§4.4) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-4.4-3` | Lower-level protocols pass indications to OSPF as the network interface goes up and down (§4.4) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-8.1-1` | Assign at least one IP address to the router, for use as the IP source address of packets sent over unnumbered point-to-point networks and virtual links (§8.1) | MUST | 8.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-8.2-1` | Accept a received packet only when it passes the IP-level tests of Section 8.2: a correct IP checksum, IP protocol OSPF (89), and an IP destination equal to the receiving interface's address or to AllSPFRouters or AllDRouters (§8.2) | MUST | 8.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-8.2-2` | Verify the OSPF header against the receiving interface: the Area ID matches the interface's area, or names the backbone, in which case the receiving router is an area border router, the source router is the other endpoint of a configured virtual link, and the receiving interface attaches to that link's Transit area (§8.2) | MUST | 8.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-8.2-3` | Authenticate every received OSPF packet and discard one whose AuType does not match the AuType configured for the associated area; accept a packet of any type other than Hello only from an active neighbor, because all other types are sent and received only on adjacencies (§8.2, §D.4) | MUST | 8.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-9.1-1` | A router that becomes Designated Router for an attached network originates a network-LSA for that network (§9.1, §12.4.2) | MUST | 9.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-9.5.1-1` | Send Hello packets on an NBMA network per Section 9.5.1: a Designated-Router-eligible router sends periodic Hellos to every other eligible neighbor; the Designated Router or Backup Designated Router also sends periodic Hellos to all other neighbors; a router that is not eligible sends periodic Hellos to the Designated Router and Backup Designated Router and replies with a Hello to an eligible neighbor's Hello; the interface state is at least Waiting before any Hello is sent out the NBMA interface (§9.5.1) | MUST | 9.5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-10.5-1` | On receiving a Hello, check the Network Mask, HelloInterval and RouterDeadInterval against the values configured for the receiving interface and reject the packet on a mismatch; declare bidirectional communication only when the router itself is listed in the neighbor's Hello (§10.5, §9.5) | MUST | 10.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-10.6-1` | Process Database Description packets in sequence; as slave, reply to each one with a Database Description packet, repeat the last packet sent in answer to a duplicate, and keep it for RouterDeadInterval seconds after the last reply (§10.6, §10.8) | MUST | 10.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-12.1.6-1` | Before the LS sequence number wraps past MaxSequenceNumber, flush the current instance from the routing domain by premature aging, and originate the new instance at InitialSequenceNumber only after that flood is acknowledged by all adjacent neighbors (§12.1.6, §14.1) | MUST | 12.1.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-12.2-1` | Provide a database lookup of an individual LSA on LS type, Link State ID and Advertising Router, and a lookup of a network-LSA on Link State ID alone (§12.2, §16.1) | MUST | 12.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-12.4-1` | When a destination advertised in a summary-LSA or an AS-external-LSA becomes unreachable, or is no longer advertisable to an area, flush that LSA from the routing domain by setting its LS age to MaxAge and reflooding it (§12.4, §12.4.3, §12.4.4, §16.7) | MUST | 12.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-12.4.1-1` | Describe all of the router's links to an area in a single router-LSA, as the total collection of that router's interfaces to the area (§12.4.1, §A.4.2) | MUST | 12.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-12.4.3-1` | As an area border router, originate summary-LSAs into a newly attached area for all pertinent intra-area and inter-area routes in the routing table, condensing that information as the configured area address ranges require (§12.4.3, §12.4) | MUST | 12.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-13-5` | Remove an LSA deleted or replaced in the database from all neighbors' Link state retransmission lists (§13, §12.2) | MUST | 13 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-13-6` | When an installed LSA's contents differ from the previous instance, recalculate the affected routing table: the entire table for a router-LSA or a network-LSA, the destination's best route for a summary-LSA or an AS-external-LSA (§13, §16.5, §16.6) | MUST | 13 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-13-7` | Discard, without acknowledging it, a received LSA whose database copy was received via flooding and installed less than MinLSArrival seconds ago (§13, §B) | MUST | 13 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-13.3-3` | On non-broadcast networks send Link State Update packets, and delayed Link State Acknowledgments, as separate unicasts to each adjacent neighbor in state Exchange or greater (§13.3, §13.5) | MUST | 13.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-15-2` | Take a virtual link's cost and next hop from the Transit area's routing table entry for the other endpoint, and originate a new backbone router-LSA when that cost changes while a virtual adjacency is fully established (§15, §16.7) | MUST | 15 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-16.1-3` | When several vertices are equally close to the root, add network vertices to the shortest-path tree before router vertices, so that all equal-cost paths are found (§16.1) | MUST | 16.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-A.1-1` | Set the IP TTL to 1 on OSPF packets sent to the AllSPFRouters and AllDRouters multicast addresses, so that they travel one hop only (§A.1) | MUST | A.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-A.1-2` | As Designated Router or Backup Designated Router, be prepared to receive packets addressed to AllDRouters (224.0.0.6) (§A.1) | MUST | A.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-A.4.4-1` | Set the Network Mask field to 0 in a Type 4 summary-LSA, where the field is not meaningful (§A.4.4) | MUST | A.4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-A.2-1` | Reset (clear) unrecognized Options bits when sending Hellos / DD packets and when originating LSAs (§A.2) | SHOULD | A.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-A.2-2` | Ignore unrecognized Options bits on receipt and process the packet/LSA normally (§A.2) | SHOULD | A.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-9.5-1` | Set the E-bit in Hello Options iff the attached area can process AS-external-LSAs (not a stub); a mismatch causes Hello rejection (§9.5, §10.5) | SHOULD | 9.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-C.1-1` | Keep RFC1583Compatibility set identically on all routers; "disabled" (the 16.4.1 rules) when no un-updated routers are present (§C.1, §16.4.1) | SHOULD | C.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-B-1` | Refresh a self-originated LSA when its LS age reaches LSRefreshTime (30 minutes) (§B, §12) | SHOULD | B | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-14-3` | Restart the router (at least) on detecting an LS checksum failure during database aging at a CheckAge multiple (§14, §12.1.7) | SHOULD | 14 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-14.1-1` | Flush a self-originated AS-external-LSA via premature aging rather than re-originating with metric LSInfinity when the route becomes unreachable (§14.1) | SHOULD | 14.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-13.5-2` | Keep delayed-acknowledgment intervals shorter than RxmtInterval to avoid needless retransmissions (§13.5) | SHOULD | 13.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-C.3-2` | Make RouterDeadInterval some multiple of HelloInterval (e.g. 4) (§C.3) | SHOULD | C.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-A.4.5-1` | Set the Forwarding address in an AS-external-LSA to 0.0.0.0 to direct traffic to the originating ASBR (§A.4.5, §16.4) | MAY | A.4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-16.1-2` | Use a more efficient SPF algorithm (e.g. incremental SPF) provided it produces an identical shortest-path tree (§16.1) | MAY | 16.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-15-1` | Configure virtual links through non-backbone (non-stub) Transit areas to repair backbone connectivity (§15) | MAY | 15 | **positive:** no positive test. **negative:** no negative test |
| `RFC2328-D.3-3` | Configure multiple Cryptographic auth keys per interface with KeyStart/KeyStop time constants for smooth rollover (§D.3) | MAY | D.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2328-13.3-2`](#rfc2328-13.3-2) Increment an LSA's LS age by InfTransDelay (which MUST be > 0) when copying it into an outgoing Link State Update, capped at MaxAge (§13.3, §13.6, §14) | {gap}, no test | the InfTransDelay bump is applied on the retransmit path (RetransmitTick, lsdb/flooding.go:545) and on a direct database-copy reply (sendDirectLSUpdate, lsdb/flooding.go:745; sendDirectLinkLSUpdate, lsdb/link_scope.go:349), but NOT on the normal flood: floodExcept builds the outgoing copy with `entry.LSA(d.now())` (lsdb/flooding.go:351), and Entry.LSA calls `e.Raw(now, 0)` with transmitDelay 0 (lsdb/entry.go:62-68, 75-85), so the first flooded copy carries the unincremented LS age. The MaxAge cap itself is present wherever the bump is applied (LSAge.Add, types/lsage.go:54-63). Disclosed in docs/features/rfc-status.md RFC 2328 row |
| [`RFC2328-3.6-1`](#rfc2328-3.6-1) One or more of a stub area's area border routers advertise a default route into the stub area via summary-LSAs (§3.6) | no test | no test carries this requirement id |
| [`RFC2328-4.4-1`](#rfc2328-4.4-1) Support receiving and sending IP multicast datagrams, with the appropriate lower-level protocol support (§4.4) | no test | no test carries this requirement id |
| [`RFC2328-4.4-2`](#rfc2328-4.4-2) The router's IP support includes variable-length subnetting (dividing one class A, B or C network into subnets of various sizes) and IP supernetting (aggregating contiguous class A, B and C networks into supernets) (§4.4) | no test | no test carries this requirement id |
| [`RFC2328-4.4-3`](#rfc2328-4.4-3) Lower-level protocols pass indications to OSPF as the network interface goes up and down (§4.4) | no test | no test carries this requirement id |
| [`RFC2328-8.1-1`](#rfc2328-8.1-1) Assign at least one IP address to the router, for use as the IP source address of packets sent over unnumbered point-to-point networks and virtual links (§8.1) | no test | no test carries this requirement id |
| [`RFC2328-8.2-1`](#rfc2328-8.2-1) Accept a received packet only when it passes the IP-level tests of Section 8.2: a correct IP checksum, IP protocol OSPF (89), and an IP destination equal to the receiving interface's address or to AllSPFRouters or AllDRouters (§8.2) | no test | no test carries this requirement id |
| [`RFC2328-8.2-2`](#rfc2328-8.2-2) Verify the OSPF header against the receiving interface: the Area ID matches the interface's area, or names the backbone, in which case the receiving router is an area border router, the source router is the other endpoint of a configured virtual link, and the receiving interface attaches to that link's Transit area (§8.2) | no test | no test carries this requirement id |
| [`RFC2328-8.2-3`](#rfc2328-8.2-3) Authenticate every received OSPF packet and discard one whose AuType does not match the AuType configured for the associated area; accept a packet of any type other than Hello only from an active neighbor, because all other types are sent and received only on adjacencies (§8.2, §D.4) | no test | no test carries this requirement id |
| [`RFC2328-9.1-1`](#rfc2328-9.1-1) A router that becomes Designated Router for an attached network originates a network-LSA for that network (§9.1, §12.4.2) | no test | no test carries this requirement id |
| [`RFC2328-9.5.1-1`](#rfc2328-9.5.1-1) Send Hello packets on an NBMA network per Section 9.5.1: a Designated-Router-eligible router sends periodic Hellos to every other eligible neighbor; the Designated Router or Backup Designated Router also sends periodic Hellos to all other neighbors; a router that is not eligible sends periodic Hellos to the Designated Router and Backup Designated Router and replies with a Hello to an eligible neighbor's Hello; the interface state is at least Waiting before any Hello is sent out the NBMA interface (§9.5.1) | no test | no test carries this requirement id |
| [`RFC2328-10.5-1`](#rfc2328-10.5-1) On receiving a Hello, check the Network Mask, HelloInterval and RouterDeadInterval against the values configured for the receiving interface and reject the packet on a mismatch; declare bidirectional communication only when the router itself is listed in the neighbor's Hello (§10.5, §9.5) | no test | no test carries this requirement id |
| [`RFC2328-10.6-1`](#rfc2328-10.6-1) Process Database Description packets in sequence; as slave, reply to each one with a Database Description packet, repeat the last packet sent in answer to a duplicate, and keep it for RouterDeadInterval seconds after the last reply (§10.6, §10.8) | no test | no test carries this requirement id |
| [`RFC2328-12.1.6-1`](#rfc2328-12.1.6-1) Before the LS sequence number wraps past MaxSequenceNumber, flush the current instance from the routing domain by premature aging, and originate the new instance at InitialSequenceNumber only after that flood is acknowledged by all adjacent neighbors (§12.1.6, §14.1) | no test | no test carries this requirement id |
| [`RFC2328-12.2-1`](#rfc2328-12.2-1) Provide a database lookup of an individual LSA on LS type, Link State ID and Advertising Router, and a lookup of a network-LSA on Link State ID alone (§12.2, §16.1) | no test | no test carries this requirement id |
| [`RFC2328-12.4-1`](#rfc2328-12.4-1) When a destination advertised in a summary-LSA or an AS-external-LSA becomes unreachable, or is no longer advertisable to an area, flush that LSA from the routing domain by setting its LS age to MaxAge and reflooding it (§12.4, §12.4.3, §12.4.4, §16.7) | no test | no test carries this requirement id |
| [`RFC2328-12.4.1-1`](#rfc2328-12.4.1-1) Describe all of the router's links to an area in a single router-LSA, as the total collection of that router's interfaces to the area (§12.4.1, §A.4.2) | no test | no test carries this requirement id |
| [`RFC2328-12.4.3-1`](#rfc2328-12.4.3-1) As an area border router, originate summary-LSAs into a newly attached area for all pertinent intra-area and inter-area routes in the routing table, condensing that information as the configured area address ranges require (§12.4.3, §12.4) | no test | no test carries this requirement id |
| [`RFC2328-13-5`](#rfc2328-13-5) Remove an LSA deleted or replaced in the database from all neighbors' Link state retransmission lists (§13, §12.2) | no test | no test carries this requirement id |
| [`RFC2328-13-6`](#rfc2328-13-6) When an installed LSA's contents differ from the previous instance, recalculate the affected routing table: the entire table for a router-LSA or a network-LSA, the destination's best route for a summary-LSA or an AS-external-LSA (§13, §16.5, §16.6) | no test | no test carries this requirement id |
| [`RFC2328-13-7`](#rfc2328-13-7) Discard, without acknowledging it, a received LSA whose database copy was received via flooding and installed less than MinLSArrival seconds ago (§13, §B) | no test | no test carries this requirement id |
| [`RFC2328-13.3-3`](#rfc2328-13.3-3) On non-broadcast networks send Link State Update packets, and delayed Link State Acknowledgments, as separate unicasts to each adjacent neighbor in state Exchange or greater (§13.3, §13.5) | no test | no test carries this requirement id |
| [`RFC2328-15-2`](#rfc2328-15-2) Take a virtual link's cost and next hop from the Transit area's routing table entry for the other endpoint, and originate a new backbone router-LSA when that cost changes while a virtual adjacency is fully established (§15, §16.7) | no test | no test carries this requirement id |
| [`RFC2328-16.1-3`](#rfc2328-16.1-3) When several vertices are equally close to the root, add network vertices to the shortest-path tree before router vertices, so that all equal-cost paths are found (§16.1) | no test | no test carries this requirement id |
| [`RFC2328-A.1-1`](#rfc2328-a.1-1) Set the IP TTL to 1 on OSPF packets sent to the AllSPFRouters and AllDRouters multicast addresses, so that they travel one hop only (§A.1) | no test | no test carries this requirement id |
| [`RFC2328-A.1-2`](#rfc2328-a.1-2) As Designated Router or Backup Designated Router, be prepared to receive packets addressed to AllDRouters (224.0.0.6) (§A.1) | no test | no test carries this requirement id |
| [`RFC2328-A.4.4-1`](#rfc2328-a.4.4-1) Set the Network Mask field to 0 in a Type 4 summary-LSA, where the field is not meaningful (§A.4.4) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2328-A.3.1-1`](#rfc2328-a.3.1-1)

All OSPF packets begin with the standard 24-byte header; Version # MUST be 2 (§A.3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFHeaderRejectsBadVersionAndLength`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/header_test.go#L153) | unit/verify | unproven |
| positive | [`TestOSPFHeaderRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/header_test.go#L131) | unit/verify | unproven |

### [`RFC2328-A.3.1-2`](#rfc2328-a.3.1-2)

Compute the packet header IP checksum over the whole packet excluding the 64-bit authentication field (§A.3.1, §D.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPacketVerifyChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/header_test.go#L100) | unit/verify | unproven |
| positive | [`TestOSPFPacketChecksumExcludesAuth`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L32) | unit/verify | unproven |

### [`RFC2328-12.1.7-1`](#rfc2328-12.1.7-1)

Compute the LS (Fletcher) checksum over the complete LSA excluding the LS age field; the LS checksum MUST NOT be zero (calculation is not optional) (§12.1.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328ZeroLSChecksumRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc2328_test.go#L10) | unit/verify | unproven |
| positive | [`TestOSPFLSAChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L75) | unit/verify | unproven |
| positive | [`TestOSPFLSAChecksumExcludesAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L102) | unit/verify | unproven |

### [`RFC2328-13-1`](#rfc2328-13-1)

In the flooding procedure, discard an LSA with an invalid LS checksum and discard an LSA of unknown LS type (only types 1-5 are defined) (§13)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328BadLSChecksumDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_test.go#L17) | unit/verify | unproven |
| negative | [`TestDecodeLSReqRejectsMalformed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/packet_body_test.go#L145) | unit/verify | unproven |
| positive | [`TestOSPFFloodOutOtherInterfaces`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L30) | unit/verify | unproven |

### [`RFC2328-13-2`](#rfc2328-13-2)

Flood AS-external (Type 5) LSAs into or throughout a stub area (§13, §3.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFStubAreaDropsType5`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L247) | unit/verify | unproven |
| positive | [`TestOSPFStubFloodFilter`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/area_type_test.go#L15) | unit/verify | unproven |

### [`RFC2328-13-3`](#rfc2328-13-3)

Drop a Link State Update / Acknowledgment from a neighbor in a state lesser than Exchange (§13, §13.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328FloodingRequiresExchangeOrHigher`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L20) | unit/verify | unproven |
| positive | [`TestRFC2328FloodingRequiresExchangeOrHigher`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L17) | unit/verify | unproven |

### [`RFC2328-13.1-1`](#rfc2328-13.1-1)

Determine the more recent of two LSA instances using LS sequence number, then larger LS checksum, then MaxAge, then younger LS age beyond MaxAgeDiff (§13.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328OlderInstanceGetsDatabaseCopyBack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_test.go#L50) | unit/verify | unproven |
| positive | [`TestOSPFFreshnessCompareMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/lsdb_test.go#L126) | unit/verify | unproven |

### [`RFC2328-13-4`](#rfc2328-13-4)

If the database copy is MaxAge with LS sequence number MaxSequenceNumber, discard a received older instance without acknowledging (§13, step 8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328OlderInstanceGetsDatabaseCopyBack`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc2328_test.go#L53) | unit/verify | unproven |
| positive | [`TestOSPFMaxSeqMaxAgeSilentDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_edges_test.go#L19) | unit/verify | unproven |

### [`RFC2328-13.3-1`](#rfc2328-13.3-1)

Add an LSA flooded out an adjacency to that adjacency's Link state retransmission list and retransmit at RxmtInterval until acknowledged (§13.3, §13.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFFloodQueuesExchangeAndLoadingNeighbors`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L57) | unit/verify | unproven |
| positive | [`TestOSPFFloodOutOtherInterfaces`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L31) | unit/verify | unproven |
| positive | [`TestOSPFRetransmitTimer`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L129) | unit/verify | unproven |

### [`RFC2328-13.3-2`](#rfc2328-13.3-2)

Increment an LSA's LS age by InfTransDelay (which MUST be > 0) when copying it into an outgoing Link State Update, capped at MaxAge (§13.3, §13.6, §14)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2328-13.3-2, so no unit is bound to it.

### [`RFC2328-14-1`](#rfc2328-14-1)

Never increment an LSA's LS age past MaxAge, and exclude MaxAge LSAs from the routing-table calculation (§14)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFGraphSkipsMaxAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/graph_test.go#L40) | unit/verify | unproven |
| positive | [`TestOSPFLSDBAgeToPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/aging_test.go#L34) | unit/verify | unproven |
| positive | [`TestLSAgeAddSaturates`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/lsage_test.go#L39) | unit/verify | unproven |

### [`RFC2328-14-2`](#rfc2328-14-2)

Remove a MaxAge LSA from the database only once it is on no neighbor retransmission list and no neighbor is in Exchange or Loading (§14)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFPurgeRetainedForExchangeOrLoading`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L383) | unit/verify | unproven |
| positive | [`TestOSPFASExternalPurgeRetainedAcrossAreas`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L332) | unit/verify | unproven |

### [`RFC2328-13.5-1`](#rfc2328-13.5-1)

Acknowledge every newly received LSA (directly or implicitly per Table 19) (§13.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFDRRefloodsBackOutReceivingInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_edges_test.go#L80) | unit/verify | unproven |
| positive | [`TestOSPFAckDecisionTable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L93) | unit/verify | unproven |
| positive | [`TestOSPFUnknownMaxAgeNoCopyIsAckedAndDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_test.go#L360) | unit/verify | unproven |

### [`RFC2328-13.4-1`](#rfc2328-13.4-1)

Detect self-originated LSAs by Advertising Router == own Router ID, or network-LSA Link State ID == own interface address, and re-originate or flush via premature aging (§13.4, §14.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFSelfOriginatedNoLocalCopyFlush`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding_edges_test.go#L43) | unit/verify | unproven |
| positive | [`TestOSPFOriginateSelfReceivedHigherSeq`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/origination_test.go#L426) | unit/verify | unproven |

### [`RFC2328-16.1-1`](#rfc2328-16.1-1)

In intra-area SPF, include a transit-vertex link only if the neighbor LSA exists, is not MaxAge, and has a link back to the current vertex (two-way check) (§16.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFTwoWayCheck`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/spf_test.go#L31) | unit/verify | unproven |
| positive | [`TestOSPFSPFShortestPath`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/spf_test.go#L13) | unit/verify | unproven |

### [`RFC2328-16.4-1`](#rfc2328-16.4-1)

Prefer intra-area and inter-area paths over AS-external paths; prefer Type 1 external over Type 2; among Type 2 prefer the smallest type-2 metric (§16.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFExternalE1PreferredOverE2`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external_test.go#L83) | unit/verify | unproven |
| positive | [`TestOSPFRouteTablePreference`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/route_test.go#L8) | unit/verify | unproven |

### [`RFC2328-16.2-1`](#rfc2328-16.2-1)

As an ABR, examine only backbone summary-LSAs when computing inter-area routes (§16.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFABRBackboneOnlyAcceptance`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/interarea_test.go#L74) | unit/verify | unproven |
| positive | [`TestOSPFInterAreaRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/interarea_test.go#L43) | unit/verify | unproven |

### [`RFC2328-16.2-2`](#rfc2328-16.2-2)

Skip a summary-LSA or AS-external-LSA whose cost is LSInfinity, whose LS age is MaxAge, or that is self-originated, during the routing calculation (§16.2, §16.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFExternalLSInfinityDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external_test.go#L61) | unit/verify | unproven |
| negative | [`TestOSPFInterAreaLSInfinityDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/interarea_test.go#L137) | unit/verify | unproven |
| negative | [`TestRFC2328ExternalSkipsMaxAgeAndSelf`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_test.go#L43) | unit/verify | unproven |
| negative | [`TestRFC2328InterAreaSkipsMaxAgeAndSelfSummary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc2328_test.go#L20) | unit/verify | unproven |
| positive | [`TestOSPFInterAreaRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/interarea_test.go#L44) | unit/verify | unproven |

### [`RFC2328-D.2-1`](#rfc2328-d.2-1)

Discard a packet whose Simple-password (AuType 1) authentication field does not match the configured 64-bit password (§D.2, §D.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328SimplePasswordMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_test.go#L98) | unit/verify | unproven |
| positive | [`TestRFC2328SimplePasswordMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_test.go#L94) | unit/verify | unproven |

### [`RFC2328-D.3-1`](#rfc2328-d.3-1)

For Cryptographic auth (AuType 2), set the header checksum to 0, append the message digest (16 bytes for MD5), and exclude the digest from the OSPF header packet length while including it in the IP length (§D.3, §D.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthCryptoRejectsExtraTrailerBytes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L214) | unit/verify | unproven |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L46) | unit/verify | unproven |

### [`RFC2328-D.3-2`](#rfc2328-d.3-2)

Treat the crypto sequence number as non-decreasing, reset it to 0 when the neighbor goes Down, and set it to a received packet's value when accepted as authentic (§D.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L116) | unit/verify | unproven |
| positive | [`TestNeighborDownResetsCryptoSeq`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L202) | unit/verify | unproven |
| positive | [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L115) | unit/verify | unproven |

### [`RFC2328-A.3.3-1`](#rfc2328-a.3.3-1)

Set Interface MTU to 0 in Database Description packets sent over virtual links (§A.3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2328VirtualLinkDBDescCarriesZeroMTU`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L63) | unit/verify | unproven |
| positive | [`TestRFC2328VirtualLinkDBDescCarriesZeroMTU`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L58) | unit/verify | unproven |
| positive | [`TestRFC2328VirtualInterfaceHasNoMTU`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_test.go#L32) | unit/verify | unproven |

### [`RFC2328-10.1-1`](#rfc2328-10.1-1)

Allow only one Database Description packet outstanding per adjacency at a time (§10.1, §10.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFDuplicateDD`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/nsm_test.go#L431) | unit/verify | unproven |
| positive | [`TestOSPFDDRetransmit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/nsm_test.go#L363) | unit/verify | unproven |

### [`RFC2328-10.2-1`](#rfc2328-10.2-1)

Generate the BadLSReq event and restart the Database Exchange when an LS Request names an LSA not in the database (§10.2, §13)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFValidLSReqSendsLSUpdate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/nsm_test.go#L721) | unit/verify | unproven |
| negative | [`TestRFC2328KnownLSRequestDoesNotRestartExchange`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L121) | unit/verify | unproven |
| positive | [`TestOSPFBadLSReqRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/nsm_test.go#L566) | unit/verify | unproven |

### [`RFC2328-C.3-1`](#rfc2328-c.3-1)

Use a positive Interface output cost (greater than 0) (§C.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInterfaceCostAndTransmitDelayBoundary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_interface_validate_test.go#L17) | unit/verify | unproven |
| positive | [`TestInterfaceCostAndTransmitDelayBoundary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_interface_validate_test.go#L16) | unit/verify | unproven |

### [`RFC2328-3.6-1`](#rfc2328-3.6-1)

One or more of a stub area's area border routers advertise a default route into the stub area via summary-LSAs (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2328-3.6-1, so no unit is bound to it.

### [`RFC2328-4.4-1`](#rfc2328-4.4-1)

Support receiving and sending IP multicast datagrams, with the appropriate lower-level protocol support (§4.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2328-4.4-1, so no unit is bound to it.

### [`RFC2328-4.4-2`](#rfc2328-4.4-2)

The router's IP support includes variable-length subnetting (dividing one class A, B or C network into subnets of various sizes) and IP supernetting (aggregating contiguous class A, B and C networks into supernets) (§4.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2328-4.4-2, so no unit is bound to it.

### [`RFC2328-4.4-3`](#rfc2328-4.4-3)

Lower-level protocols pass indications to OSPF as the network interface goes up and down (§4.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2328-4.4-3, so no unit is bound to it.

### [`RFC2328-8.1-1`](#rfc2328-8.1-1)

Assign at least one IP address to the router, for use as the IP source address of packets sent over unnumbered point-to-point networks and virtual links (§8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2328-8.1-1, so no unit is bound to it.

### [`RFC2328-8.2-1`](#rfc2328-8.2-1)

Accept a received packet only when it passes the IP-level tests of Section 8.2: a correct IP checksum, IP protocol OSPF (89), and an IP destination equal to the receiving interface's address or to AllSPFRouters or AllDRouters (§8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2328-8.2-1, so no unit is bound to it.

### [`RFC2328-8.2-2`](#rfc2328-8.2-2)

Verify the OSPF header against the receiving interface: the Area ID matches the interface's area, or names the backbone, in which case the receiving router is an area border router, the source router is the other endpoint of a configured virtual link, and the receiving interface attaches to that link's Transit area (§8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2328-8.2-2, so no unit is bound to it.

### [`RFC2328-8.2-3`](#rfc2328-8.2-3)

Authenticate every received OSPF packet and discard one whose AuType does not match the AuType configured for the associated area; accept a packet of any type other than Hello only from an active neighbor, because all other types are sent and received only on adjacencies (§8.2, §D.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2328-8.2-3, so no unit is bound to it.

### [`RFC2328-9.1-1`](#rfc2328-9.1-1)

A router that becomes Designated Router for an attached network originates a network-LSA for that network (§9.1, §12.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2328-9.1-1, so no unit is bound to it.

### [`RFC2328-9.5.1-1`](#rfc2328-9.5.1-1)

Send Hello packets on an NBMA network per Section 9.5.1: a Designated-Router-eligible router sends periodic Hellos to every other eligible neighbor; the Designated Router or Backup Designated Router also sends periodic Hellos to all other neighbors; a router that is not eligible sends periodic Hellos to the Designated Router and Backup Designated Router and replies with a Hello to an eligible neighbor's Hello; the interface state is at least Waiting before any Hello is sent out the NBMA interface (§9.5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2328-9.5.1-1, so no unit is bound to it.

### [`RFC2328-10.5-1`](#rfc2328-10.5-1)

On receiving a Hello, check the Network Mask, HelloInterval and RouterDeadInterval against the values configured for the receiving interface and reject the packet on a mismatch; declare bidirectional communication only when the router itself is listed in the neighbor's Hello (§10.5, §9.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2328-10.5-1, so no unit is bound to it.

### [`RFC2328-10.6-1`](#rfc2328-10.6-1)

Process Database Description packets in sequence; as slave, reply to each one with a Database Description packet, repeat the last packet sent in answer to a duplicate, and keep it for RouterDeadInterval seconds after the last reply (§10.6, §10.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2328-10.6-1, so no unit is bound to it.

### [`RFC2328-12.1.6-1`](#rfc2328-12.1.6-1)

Before the LS sequence number wraps past MaxSequenceNumber, flush the current instance from the routing domain by premature aging, and originate the new instance at InitialSequenceNumber only after that flood is acknowledged by all adjacent neighbors (§12.1.6, §14.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2328-12.1.6-1, so no unit is bound to it.

### [`RFC2328-12.2-1`](#rfc2328-12.2-1)

Provide a database lookup of an individual LSA on LS type, Link State ID and Advertising Router, and a lookup of a network-LSA on Link State ID alone (§12.2, §16.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2328-12.2-1, so no unit is bound to it.

### [`RFC2328-12.4-1`](#rfc2328-12.4-1)

When a destination advertised in a summary-LSA or an AS-external-LSA becomes unreachable, or is no longer advertisable to an area, flush that LSA from the routing domain by setting its LS age to MaxAge and reflooding it (§12.4, §12.4.3, §12.4.4, §16.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2328-12.4-1, so no unit is bound to it.

### [`RFC2328-12.4.1-1`](#rfc2328-12.4.1-1)

Describe all of the router's links to an area in a single router-LSA, as the total collection of that router's interfaces to the area (§12.4.1, §A.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2328-12.4.1-1, so no unit is bound to it.

### [`RFC2328-12.4.3-1`](#rfc2328-12.4.3-1)

As an area border router, originate summary-LSAs into a newly attached area for all pertinent intra-area and inter-area routes in the routing table, condensing that information as the configured area address ranges require (§12.4.3, §12.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2328-12.4.3-1, so no unit is bound to it.

### [`RFC2328-13-5`](#rfc2328-13-5)

Remove an LSA deleted or replaced in the database from all neighbors' Link state retransmission lists (§13, §12.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2328-13-5, so no unit is bound to it.

### [`RFC2328-13-6`](#rfc2328-13-6)

When an installed LSA's contents differ from the previous instance, recalculate the affected routing table: the entire table for a router-LSA or a network-LSA, the destination's best route for a summary-LSA or an AS-external-LSA (§13, §16.5, §16.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2328-13-6, so no unit is bound to it.

### [`RFC2328-13-7`](#rfc2328-13-7)

Discard, without acknowledging it, a received LSA whose database copy was received via flooding and installed less than MinLSArrival seconds ago (§13, §B)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2328-13-7, so no unit is bound to it.

### [`RFC2328-13.3-3`](#rfc2328-13.3-3)

On non-broadcast networks send Link State Update packets, and delayed Link State Acknowledgments, as separate unicasts to each adjacent neighbor in state Exchange or greater (§13.3, §13.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2328-13.3-3, so no unit is bound to it.

### [`RFC2328-15-2`](#rfc2328-15-2)

Take a virtual link's cost and next hop from the Transit area's routing table entry for the other endpoint, and originate a new backbone router-LSA when that cost changes while a virtual adjacency is fully established (§15, §16.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2328-15-2, so no unit is bound to it.

### [`RFC2328-16.1-3`](#rfc2328-16.1-3)

When several vertices are equally close to the root, add network vertices to the shortest-path tree before router vertices, so that all equal-cost paths are found (§16.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2328-16.1-3, so no unit is bound to it.

### [`RFC2328-A.1-1`](#rfc2328-a.1-1)

Set the IP TTL to 1 on OSPF packets sent to the AllSPFRouters and AllDRouters multicast addresses, so that they travel one hop only (§A.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2328-A.1-1, so no unit is bound to it.

### [`RFC2328-A.1-2`](#rfc2328-a.1-2)

As Designated Router or Backup Designated Router, be prepared to receive packets addressed to AllDRouters (224.0.0.6) (§A.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2328-A.1-2, so no unit is bound to it.

### [`RFC2328-A.4.4-1`](#rfc2328-a.4.4-1)

Set the Network Mask field to 0 in a Type 4 summary-LSA, where the field is not meaningful (§A.4.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2328-A.4.4-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
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
| `A` | not stated | 9 | walked | not stated |
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
| `A:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | describes what the LSA header contains ('all the information required to uniquely identify the LSA'); a field description, not an obligation | It contains all the information required to uniquely identify both the LSA and the LSA's current instance. |
| `A:5` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | describes what the LSA header contains ('all the information required to uniquely identify the LSA'); a field description, not an obligation | It contains all the information required to uniquely identify both the LSA and the LSA's current instance. |
| `A:8` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | describes the # TOS field's meaning, counting metrics 'not counting the required link metric'; a field description, not an obligation | See Section 16.1.1 for more details. # TOS The number of different TOS metrics given for this link, not counting the required link metric (referred to as the TOS 0 metric in [Ref9]). |
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
