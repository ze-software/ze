# RFC 2328 - OSPF Version 2

Partial. Every requirement this repository extracted from RFC 2328, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 96.0% | 24 of 25 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 25 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 25 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 59 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 25 | of 38 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 25 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 4.0% | 1 of 25 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 25 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 38 |
| Gated MUST-level | 25 |
| Obligations that bind Ze | 25 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Gated with no test | 0 |
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

One MUST gap, annotated in [`rfc/short/rfc2328.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc2328.md) and gated by `./le rfc check`: [`RFC2328-13.3-2`](#rfc2328-13.3-2) -- the InfTransDelay increment of LS age is applied on retransmission ([`internal/plugins/ospf/lsdb/flooding.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding.go)) and on a direct database-copy reply ([`internal/plugins/ospf/lsdb/flooding.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding.go)), but the normal flood path copies the LSA with transmit delay 0 (`floodExcept` -> `entry.LSA(d.now())` -> `Raw(now, 0)`, [`internal/plugins/ospf/lsdb/flooding.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/flooding.go) and [`internal/plugins/ospf/lsdb/entry.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/entry.go)), so a first-flooded LSA carries an unincremented age. The feature also remains pre-production pending hardening and deployment evidence.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 24 | one part of the gated population |
| Annotated instead of tested | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **25** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (24):** [`RFC2328-A.3.1-1`](#rfc2328-a.3.1-1), [`RFC2328-A.3.1-2`](#rfc2328-a.3.1-2), [`RFC2328-12.1.7-1`](#rfc2328-12.1.7-1), [`RFC2328-13-1`](#rfc2328-13-1), [`RFC2328-13-2`](#rfc2328-13-2), [`RFC2328-13-3`](#rfc2328-13-3), [`RFC2328-13.1-1`](#rfc2328-13.1-1), [`RFC2328-13-4`](#rfc2328-13-4), [`RFC2328-13.3-1`](#rfc2328-13.3-1), [`RFC2328-14-1`](#rfc2328-14-1), [`RFC2328-14-2`](#rfc2328-14-2), [`RFC2328-13.5-1`](#rfc2328-13.5-1), [`RFC2328-13.4-1`](#rfc2328-13.4-1), [`RFC2328-16.1-1`](#rfc2328-16.1-1), [`RFC2328-16.4-1`](#rfc2328-16.4-1), [`RFC2328-16.2-1`](#rfc2328-16.2-1), [`RFC2328-16.2-2`](#rfc2328-16.2-2), [`RFC2328-D.2-1`](#rfc2328-d.2-1), [`RFC2328-D.3-1`](#rfc2328-d.3-1), [`RFC2328-D.3-2`](#rfc2328-d.3-2), [`RFC2328-A.3.3-1`](#rfc2328-a.3.3-1), [`RFC2328-10.1-1`](#rfc2328-10.1-1), [`RFC2328-10.2-1`](#rfc2328-10.2-1), [`RFC2328-C.3-1`](#rfc2328-c.3-1)

**Annotated instead of tested (1):** [`RFC2328-13.3-2`](#rfc2328-13.3-2)

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
| `RFC2328-D.3-2` | Treat the crypto sequence number as non-decreasing, reset it to 0 when the neighbor goes Down, and set it to a received packet's value when accepted as authentic (§D.3) | MUST | D.3 | **positive:** `unit/verify` [`TestNeighborDownResetsCryptoSeq`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L198). **positive:** `unit/verify` [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L111). **negative:** `unit/verify` [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L112) |
| `RFC2328-A.3.3-1` | Set Interface MTU to 0 in Database Description packets sent over virtual links (§A.3.3) | MUST | A.3.3 | **positive:** `unit/verify` [`TestRFC2328VirtualInterfaceHasNoMTU`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_test.go#L32). **positive:** `unit/verify` [`TestRFC2328VirtualLinkDBDescCarriesZeroMTU`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L58). **negative:** `unit/verify` [`TestRFC2328VirtualLinkDBDescCarriesZeroMTU`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L63) |
| `RFC2328-10.1-1` | Allow only one Database Description packet outstanding per adjacency at a time (§10.1, §10.3) | MUST | 10.1 | **positive:** `unit/verify` [`TestOSPFDDRetransmit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/nsm_test.go#L363). **negative:** `unit/verify` [`TestOSPFDuplicateDD`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/nsm_test.go#L431) |
| `RFC2328-10.2-1` | Generate the BadLSReq event and restart the Database Exchange when an LS Request names an LSA not in the database (§10.2, §13) | MUST | 10.2 | **positive:** `unit/verify` [`TestOSPFBadLSReqRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/nsm_test.go#L566). **negative:** `unit/verify` [`TestOSPFValidLSReqSendsLSUpdate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/nsm_test.go#L721). **negative:** `unit/verify` [`TestRFC2328KnownLSRequestDoesNotRestartExchange`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/rfc2328_test.go#L121) |
| `RFC2328-C.3-1` | Use a positive Interface output cost (greater than 0) (§C.3) | MUST | C.3 | **positive:** `unit/verify` [`TestInterfaceCostAndTransmitDelayBoundary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_interface_validate_test.go#L16). **negative:** `unit/verify` [`TestInterfaceCostAndTransmitDelayBoundary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_interface_validate_test.go#L17) |
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
| negative | [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L112) | unit/verify | unproven |
| positive | [`TestNeighborDownResetsCryptoSeq`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L198) | unit/verify | unproven |
| positive | [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L111) | unit/verify | unproven |

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

## Extraction sign-off

No extraction sign-off exists for RFC 2328, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 2328, so its obligations are stated where they were written.
