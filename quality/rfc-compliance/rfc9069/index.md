# RFC 9069 - Support for Local RIB in the BGP Monitoring Protocol (BMP)

Supported. Every requirement this repository extracted from RFC 9069, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 46.7% | 7 of 15 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 53.3% | 8 of 15 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 15 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 15 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 41.7% | 10 of 24 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 15 | of 15 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 15 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 15 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 15 |
| Gated MUST-level | 15 |
| Obligations that bind Ze | 15 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 24 |
| Tagged units | 24 |
| Recorded audit verdicts | 0 |
| Discrimination records | 10 |
| Summary | `rfc/short/rfc9069.md` |
| Requirement shard | `rfc/requirements/rfc9069.md` |
| RFC text | `rfc/full/rfc9069.txt` |

## Enrolment

Enrolled: Support for Local RIB in the BGP Monitoring Protocol (BMP): seven MUST-level requirements, all met (ze implements the Loc-RIB Instance Peer type). x-1 (per-peer V/L/A/O flags cleared), x-3 (empty sent/received OPEN messages), x-5 (Address field zero), x-6 (Peer AS zero), x-7 (Peer BGP ID set to the local router-id), x-2 (exactly one Loc-RIB Peer Up per RIB instance regardless of BGP-peer count), and x-4 (Loc-RIB monitoring start triggers a full-table replay) are each {single-polarity: positive}, bound to header-construction and emission tests in internal/component/bgp/plugins/bmp (including two new tests for x-2 and x-4). These are construction and emission invariants with no reject path.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered**

Loc-RIB Instance Peer (Peer Type 3): per-peer flags cleared, zero Peer Address, Peer AS set to the router's own 4-octet ASN, Peer BGP ID set to the local router-id, a fabricated OPEN carried in both the sent and the received field of the Peer Up (Section 5.2) advertising the 4-octet ASN capability and one address-family capability per family the dump delivers, a Peer Down carrying reason code 6, the VRF/Table Name Information TLV (type 3) naming that Loc-RIB `global` on the Peer Up and repeated on the Peer Down (Section 5.2.1), a per-peer Timestamp that is the install time of an incremental change and ZERO where that time is unavailable (Section 5.1), the receiver reading the address-family capabilities off a Peer Up's OPEN and reporting them per peer (Section 6.1.1), exactly one Loc-RIB Peer Up per RIB instance per BMP session, and a full-table dump requested both when monitoring starts and whenever a collector connects, closed with an RFC 4724 End-of-RIB marker for EVERY base unicast family the dump owes one for -- the families it carried and the families the RIB stayed silent about alike (RFC 9069 itself specifies no End-of-RIB; the marker is ze's own dump-complete signal, in the RFC 4724 Section 2 form, and Section 4 of that RFC owes it "including the case when there is no update to send" for a family). Each dump also carries a per-dump correlation token, so a replay another subsystem requested is never mis-claimed as this plugin's dump. Tests bound per requirement in [`rfc/requirements/rfc9069.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc9069.md).

**What the ledger says remains**

None outstanding.

- **Closed 2026-07-27:** (1) the mixed-table gap -- a table whose IPv4 half was empty used to get an IPv6 marker and no IPv4 one, because the RIB emits no batch for an empty family ([`internal/component/bgp/plugins/rib/rib_bestchange.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange.go)), leaving a collector waiting on IPv4 waiting forever for a dump that had finished; `closeDumpFamilies` now closes every owed family (`TestMixedFamilyDumpClosesTheSilentFamily`). (2) A replay requested by sysrib that landed while a dump was in flight was claimed as this plugin's: the other collectors silently lost the batch and were sent an End-of-RIB for a dump they never requested. The claim is now made on a per-dump token the RIB echoes back (`TestForeignReplayIsNotClaimedAsOurDump`). Closed 2026-08-31, found by an extraction walk of the RFC's own text: (3) the Loc-RIB per-peer header sent Peer AS 0 against Section 5.1's "Set to the primary router BGP autonomous system number (ASN)", and the summary declared the defect as the requirement; (4) the Loc-RIB Peer Up carried two zero-length OPENs against Section 5.2's "This is a fabricated BGP OPEN message. Capabilities MUST include the 4-octet ASN", and its summary row did the same; (5) the RECEIVER shared the belief -- `decodePeerUp` skipped OPEN extraction for Peer Type 3, so a conformant Loc-RIB Peer Up from any implementation was misparsed from the first Information TLV onward; (6) the Loc-RIB Peer Down carried reason 2 against Section 5.3's "The Peer Down notification MUST use reason code 6". Closed 2026-09-01 by the extraction sign-off of the same text: (7) the per-peer Timestamp read a wall clock, which dated every route of a full-table replay to the moment the collector connected, where Section 5.1's own answer for an install time the sender does not know is zero; (8) no VRF/Table Name TLV was sent, against Section 5.2.1's "The default value of "global" MUST be used for the default Loc-RIB instance with a zero-filled distinguisher"; (9) the receiver read the capabilities of no Peer Up OPEN, against Section 6.1.1's "A BMP receiver MUST process these capabilities to know which peer belongs to which address family", and keyed every Peer Type 3 peer of one router by its zero-filled address, so two Loc-RIB instances shared one RIB-In pool. TWO FEATURES ARE OUT OF SCOPE and are implementation gaps rather than conformance gaps.
- **VRF:** ze runs one Loc-RIB, the default global instance, and offers no VRF and no second named table (owner decision, 2026-09-01), so the Section 5.2.1 obligations conditional on a configured name or on multiple names for one Loc-RIB do not bind it.
- **Loc-RIB FILTERING:** Section 5 makes it optional -- "a subset of Loc-RIB routes MAY be sent to a BMP collector by setting the F flag" -- and ze sends the whole Loc-RIB, so the F flag stays 0 and the Section 6.1.2 table-name obligation for multiple filters against one Loc-RIB does not bind it either. A later scope decision can revisit both.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 7 | one part of the gated population |
| Annotated instead of tested | 8 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **15** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (7):** [`RFC9069-5.4.1-1`](#rfc9069-5.4.1-1), [`RFC9069-x-6`](#rfc9069-x-6), [`RFC9069-4.2-1`](#rfc9069-4.2-1), [`RFC9069-5.1-1`](#rfc9069-5.1-1), [`RFC9069-5.2.1-1`](#rfc9069-5.2.1-1), [`RFC9069-6.1.1-1`](#rfc9069-6.1.1-1), [`RFC9069-6.1.3-1`](#rfc9069-6.1.3-1)

**Annotated instead of tested (8):** [`RFC9069-x-1`](#rfc9069-x-1), [`RFC9069-x-2`](#rfc9069-x-2), [`RFC9069-x-3`](#rfc9069-x-3), [`RFC9069-x-4`](#rfc9069-x-4), [`RFC9069-x-5`](#rfc9069-x-5), [`RFC9069-5.2-1`](#rfc9069-5.2-1), [`RFC9069-5.3-1`](#rfc9069-5.3-1), [`RFC9069-x-7`](#rfc9069-x-7)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9069-x-1` | Peer Type 3 MUST NOT have V, L, A, or O flags set (only F flag is valid) (Key Constraints) | MUST NOT | x | **positive:** `unit/verify` [`TestLocRIBPeerHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_locrib_test.go#L70). **negative:** no negative test. **{single-polarity}:** the Loc-RIB PeerType=3 per-peer header is constructed with Flags 0 (locRIBPeerHeader, internal/component/bgp/plugins/bmp/bmp_locrib.go) and no code path ever sets the V/L/A/O bits for a Loc-RIB header, so there is no non-conformant flag-setting case to assert as a negative; the positive (Flags==0) is proven in TestLocRIBPeerHeader |
| `RFC9069-x-2` | Loc-RIB monitoring is per-instance, not per-peer (Key Constraints) | MUST | x | **positive:** `unit/verify` [`TestLocRIBSinglePeerUpPerInstance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_locrib_test.go#L493). **negative:** no negative test. **{single-polarity}:** the Loc-RIB Peer Up is emitted once per RIB instance per collector session, behind the one-shot senderSession.locRIBUpSent.CompareAndSwap taken by ensureLocRIBPeerUp and primeLocRIBPeerUp (internal/component/bgp/plugins/bmp/bmp_locrib.go), and best changes from any number of BGP peers pass through that same guard. Corrected 2026-09-01: this row named BMPPlugin.locRIBUp as the one-shot, which is set AFTER the announcement and records only that Loc-RIB monitoring has been announced at all -- it is what sendLocRIBPeerDown keys off, and it guards nothing, so "one Peer Up per instance" is a single count-equals-one assertion with no separate per-peer path to reject; TestLocRIBSinglePeerUpPerInstance drives two peers' best changes and asserts exactly one Peer Up |
| `RFC9069-x-3` | Peer Up for Loc-RIB carries a fabricated OPEN in both the sent and the received field, the received one a repeat of the sent one (Affected Message Types) | MUST | x | **positive:** `unit/verify` [`TestHandleBestChangeEmitsPeerUpThenRM`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_locrib_test.go#L426). **negative:** no negative test. **{single-polarity}:** ensureLocRIBPeerUp and primeLocRIBPeerUp (internal/component/bgp/plugins/bmp/bmp_locrib.go) are the only two producers of a Loc-RIB Peer Up and each passes one fabricateLocRIBOpen result to BOTH fields, so there is no path that sends a different received OPEN to assert as a negative; the positive (both fields present, equal, and decoding as a BGP OPEN) is proven in TestHandleBestChangeEmitsPeerUpThenRM. Corrected 2026-08-31: this row declared "zero-length OPEN messages", which RFC 9069 Section 5.2 requires the opposite of, and its test asserted the defect |
| `RFC9069-x-4` | Initial dump sends full Loc-RIB contents as Route Monitoring (Key Constraints) | MUST | x | **positive:** `unit/verify` [`TestStartLocRIBTriggersInitialDump`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_locrib_test.go#L592). **negative:** no negative test. **{single-polarity}:** startLocRIB emits exactly one replay-request to trigger the initial full-table dump, carrying a per-dump correlation token (startLocRIB and nextDumpToken, internal/component/bgp/plugins/bmp/bmp_locrib.go); the RIB walks its ENTIRE best-path table for any token (replayBestPaths, internal/component/bgp/plugins/rib/rib_bestchange.go) and echoes the token onto each batch, so the token addresses the answer without narrowing the dump. The trigger either fires or the subscription is absent, so there is no non-conformant "dump without trigger" input to reject; TestStartLocRIBTriggersInitialDump asserts one replay-request is emitted, that it carries a replay token, and that successive dumps carry DISTINCT tokens -- the property that lets a batch be attributed to the dump that asked for it |
| `RFC9069-x-5` | Peer Address MUST be 0 for Loc-RIB (Peer Type 3) (Per-Peer Header) | MUST | x | **positive:** `unit/verify` [`TestLocRIBPeerHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_locrib_test.go#L87). **negative:** no negative test. **{single-polarity}:** the Loc-RIB per-peer header leaves Peer Address all-zero by construction (locRIBPeerHeader, internal/component/bgp/plugins/bmp/bmp_locrib.go, no Address assignment) and no path sets it, so there is no non-zero-address case to assert as a negative; the positive (Address==0) is proven in TestLocRIBPeerHeader |
| `RFC9069-5.4.1-1` | Loc-RIB Route Monitoring messages use a 4-byte ASN encoding, matching the capability the Peer Up's sent OPEN advertises (§5.4.1). Proven in both polarities by TestLocRIBRouteMonitoringUsesFourByteASNs: the encoded AS_PATH segment is 4 octets per ASN, and an ASN above 65535 round-trips whole, which a 2-byte encoding cannot represent at all | MUST | 5.4.1 - ASN Encoding | **positive:** `unit/verify` [`TestLocRIBRouteMonitoringUsesFourByteASNs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_locrib_test.go#L150). **negative:** `unit/verify` [`TestLocRIBRouteMonitoringUsesFourByteASNs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_locrib_test.go#L153) |
| `RFC9069-5.2-1` | The Loc-RIB Peer Up's fabricated OPEN carries the 4-octet ASN capability and one address-family capability per family the Loc-RIB dump delivers (§5.2, §6.1.1) | MUST | 5.2 - Peer Up Notification | **positive:** `unit/verify` [`TestFabricatedLocRIBOpenCarriesTheRequiredCapabilities`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_locrib_test.go#L331). **negative:** no negative test. **{single-polarity}:** fabricateLocRIBOpen (internal/component/bgp/plugins/bmp/bmp_locrib.go) builds the capability list from dumpFamilies, the same list the dump closes with End-of-RIB markers, so there is no second list that could advertise a family the dump never delivers and no divergent case to assert as a negative. The count is asserted as well as the membership, which is what "Only include capabilities if they will be used for Loc-RIB monitoring messages" forbids going past; proven in TestFabricatedLocRIBOpenCarriesTheRequiredCapabilities |
| `RFC9069-x-6` | Peer AS MUST be set to the primary router BGP autonomous system number for Loc-RIB (Peer Type 3) (Per-Peer Header). Corrected 2026-08-31: this row read "Peer AS MUST be 0", which is what the RFC requires of the Peer ADDRESS (RFC9069-x-5) and the reverse of what it requires of this field. The value is the 4-octet ASN, read from the 4-octet ASN capability of a cached sent OPEN rather than from the two-octet My AS field an AS4 speaker fills with AS_TRANS. Proven positive by TestLocRIBPeerHeader, negative by TestLocRIBPeerHeaderCarriesTheIdentityItIsGiven, and at the source by TestBgpIdentityPrefersTheFourOctetASNCapability | MUST | x | **positive:** `unit/verify` [`TestLocRIBPeerHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_locrib_test.go#L76). **positive:** `unit/verify` [`TestRFC9069ReloadTurningLocRIBOnAnnouncesTheRouterIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L1208). **negative:** `unit/verify` [`TestBgpIdentityPrefersTheFourOctetASNCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_locrib_test.go#L299). **negative:** `unit/verify` [`TestLocRIBPeerHeaderCarriesTheIdentityItIsGiven`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_locrib_test.go#L94) |
| `RFC9069-5.3-1` | The Loc-RIB Peer Down notification MUST use reason code 6 (§5.3) | MUST | 5.3 - Peer Down Notification | **positive:** `unit/verify` [`TestBMPReloadTurningLocRIBOffUnsubscribesAndSaysSo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L1135). **negative:** no negative test. **{single-polarity}:** sendLocRIBPeerDown (internal/component/bgp/plugins/bmp/bmp_locrib.go) is the only producer of a Loc-RIB Peer Down and it passes one constant reason, so there is no second Loc-RIB path to reject as a negative. The neighbouring risk -- reason 6 used for a peer the requirement does not cover -- is closed elsewhere: TestRFC8671BehaviorChangeBouncesEachPeerAndKeepsTheSession requires a PeerTypeGlobal Peer Down to carry reason 5, and TestPeerDownReasonMapping requires every mapped BGP close reason to be one of 1 to 5. The positive (reason 6 on the Loc-RIB peer) is proven in TestBMPReloadTurningLocRIBOffUnsubscribesAndSaysSo, over the plugin's own reload rail |
| `RFC9069-x-7` | Peer BGP ID MUST be Local Router ID for Loc-RIB (Peer Type 3) (Per-Peer Header) | MUST | x | **positive:** `unit/verify` [`TestLocRIBPeerHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_locrib_test.go#L82). **negative:** no negative test. **{single-polarity}:** the Loc-RIB per-peer header sets Peer BGP ID to the local router-id (locRIBPeerHeader and BMPPlugin.localIdentity, internal/component/bgp/plugins/bmp/bmp_locrib.go) and ze has exactly one local router-id, so there is no non-conformant BGP-ID value to reject as a negative; the positive (Peer BGP ID == router-id) is proven in TestLocRIBPeerHeader and TestHandleBestChangeEmitsPeerUpThenRM |
| `RFC9069-4.2-1` | Locally sourced routes communicated by BMP are conveyed using the Loc-RIB Instance Peer Type (§4.2). Proven positive by TestLocRIBFeedConveysRoutesWithTheLocRIBPeerType, which reads the Peer Up and the Route Monitoring one best change puts on the wire and requires Peer Type 3 on both, and negative by TestPeerHeaderFromEvent, where the header of a monitored BGP peer carries Peer Type 0: the type discriminates the Loc-RIB feed rather than being stamped on every message | MUST | 4.2 - Peer Flags | **positive:** `unit/verify` [`TestLocRIBFeedConveysRoutesWithTheLocRIBPeerType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L135). **negative:** `unit/verify` [`TestMonitoredPeerRouteMonitoringIsNotTheLocRIBPeerType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L156) |
| `RFC9069-5.1-1` | The Loc-RIB per-peer header Timestamp is the time the encapsulated routes were installed in the Loc-RIB, and zero where ze does not know it (§5.1). Ze knows it for an incremental best change, which the RIB delivers on the goroutine that installed it; a full-table replay, a Peer Up, a Peer Down and an End-of-RIB marker each carry zero, which the same paragraph defines: "If zero, the time is unavailable." Proven positive by TestLocRIBIncrementalRouteMonitoringCarriesTheInstallTime, negative by TestLocRIBReplayAndPeerUpCarryNoTimestamp | MUST | 5.1 - Per-Peer Header | **positive:** `unit/verify` [`TestLocRIBIncrementalRouteMonitoringCarriesTheInstallTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L87). **negative:** `unit/verify` [`TestLocRIBReplayAndPeerUpCarryNoTimestamp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L111) |
| `RFC9069-5.2.1-1` | The Loc-RIB Peer Up carries the VRF/Table Name Information TLV (type 3), whose UTF-8 value is "global" -- the name §5.2.1 fixes for the default Loc-RIB instance with a zero-filled distinguisher -- inside the 1-to-255-byte size bound, and the Loc-RIB Peer Down repeats that TLV after reason 6 (§5.2.1, §5.3, §8.3). Proven positive by TestLocRIBPeerUpAndPeerDownCarryTheGlobalTableName, which reads both messages of one session off the wire, and negative by TestMonitoredPeerUpCarriesNoTableNameTLV: a monitored BGP peer's Peer Up carries no such TLV, so the name is the Loc-RIB instance's rather than every peer's | MUST | 5.2.1 - Peer Up Information | **positive:** `unit/verify` [`TestLocRIBPeerUpAndPeerDownCarryTheGlobalTableName`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L186). **negative:** `unit/verify` [`TestMonitoredPeerUpCarriesNoTableNameTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L252) |
| `RFC9069-6.1.1-1` | The BMP receiver processes the capabilities of a Peer Up's OPEN to know which address families that peer carries, and records the association where `show bmp peers` reports it (§6.1.1). Proven positive by TestReceiverRecordsThePeerUpAddressFamilies and negative by TestReceiverRecordsNoFamiliesWhenThePeerUpAdvertisesNone: an OPEN advertising no Multiprotocol capability records no family, because a defaulted one would answer the association with an invention | MUST | 6.1.1 - Multiple Loc-RIB Peers | **positive:** `unit/verify` [`TestReceiverRecordsThePeerUpAddressFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L284). **negative:** `unit/verify` [`TestReceiverRecordsNoFamiliesWhenThePeerUpAdvertisesNone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L317) |
| `RFC9069-6.1.3-1` | A change that alters the behavior of an existing BMP session bounces it with a Peer Down / Peer Up sequence (§6.1.3). Ze bounces the peers inside the session and leaves the session up, which is the same answer RFC8671-7.2-1 records for the identical sentence in RFC 8671 §7.2; the Loc-RIB emulated peer is bounced by the pair of reload rails, a Peer Down when `loc-rib` goes off and a Peer Up when it comes on. Proven positive by TestRFC8671BehaviorChangeBouncesEachPeerAndKeepsTheSession and negative by TestRFC8671UnrelatedBGPChangeBouncesNothing: the bounce is owed to a CHANGE, so a reload that moves no leaf deciding what the session carries bounces nothing | MUST | 6.1.3 - Changes to Existing BMP Sessions | **positive:** `unit/verify` [`TestBehaviorChangeBouncesThePeersOfALocRIBSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L456). **negative:** `unit/verify` [`TestAConfigurationThatAltersNoBehaviorBouncesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L492) |

## Gaps and untested MUSTs

RFC 9069 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9069-x-1`](#rfc9069-x-1)

Peer Type 3 MUST NOT have V, L, A, or O flags set (only F flag is valid) (Key Constraints)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestLocRIBPeerHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_locrib_test.go#L70) | unit/verify | unproven |

### [`RFC9069-x-2`](#rfc9069-x-2)

Loc-RIB monitoring is per-instance, not per-peer (Key Constraints)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestLocRIBSinglePeerUpPerInstance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_locrib_test.go#L493) | unit/verify | unproven |

### [`RFC9069-x-3`](#rfc9069-x-3)

Peer Up for Loc-RIB carries a fabricated OPEN in both the sent and the received field, the received one a repeat of the sent one (Affected Message Types)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestHandleBestChangeEmitsPeerUpThenRM`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_locrib_test.go#L426) | unit/verify | unproven |

### [`RFC9069-x-4`](#rfc9069-x-4)

Initial dump sends full Loc-RIB contents as Route Monitoring (Key Constraints)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestStartLocRIBTriggersInitialDump`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_locrib_test.go#L592) | unit/verify | unproven |

### [`RFC9069-x-5`](#rfc9069-x-5)

Peer Address MUST be 0 for Loc-RIB (Peer Type 3) (Per-Peer Header)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestLocRIBPeerHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_locrib_test.go#L87) | unit/verify | unproven |

### [`RFC9069-5.4.1-1`](#rfc9069-5.4.1-1)

Loc-RIB Route Monitoring messages use a 4-byte ASN encoding, matching the capability the Peer Up's sent OPEN advertises (§5.4.1). Proven in both polarities by TestLocRIBRouteMonitoringUsesFourByteASNs: the encoded AS_PATH segment is 4 octets per ASN, and an ASN above 65535 round-trips whole, which a 2-byte encoding cannot represent at all

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLocRIBRouteMonitoringUsesFourByteASNs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_locrib_test.go#L153) | unit/verify | unproven |
| positive | [`TestLocRIBRouteMonitoringUsesFourByteASNs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_locrib_test.go#L150) | unit/verify | unproven |

### [`RFC9069-5.2-1`](#rfc9069-5.2-1)

The Loc-RIB Peer Up's fabricated OPEN carries the 4-octet ASN capability and one address-family capability per family the Loc-RIB dump delivers (§5.2, §6.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestFabricatedLocRIBOpenCarriesTheRequiredCapabilities`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_locrib_test.go#L331) | unit/verify | unproven |

### [`RFC9069-x-6`](#rfc9069-x-6)

Peer AS MUST be set to the primary router BGP autonomous system number for Loc-RIB (Peer Type 3) (Per-Peer Header). Corrected 2026-08-31: this row read "Peer AS MUST be 0", which is what the RFC requires of the Peer ADDRESS (RFC9069-x-5) and the reverse of what it requires of this field. The value is the 4-octet ASN, read from the 4-octet ASN capability of a cached sent OPEN rather than from the two-octet My AS field an AS4 speaker fills with AS_TRANS. Proven positive by TestLocRIBPeerHeader, negative by TestLocRIBPeerHeaderCarriesTheIdentityItIsGiven, and at the source by TestBgpIdentityPrefersTheFourOctetASNCapability

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBgpIdentityPrefersTheFourOctetASNCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_locrib_test.go#L299) | unit/verify | unproven |
| negative | [`TestLocRIBPeerHeaderCarriesTheIdentityItIsGiven`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_locrib_test.go#L94) | unit/verify | unproven |
| positive | [`TestLocRIBPeerHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_locrib_test.go#L76) | unit/verify | unproven |
| positive | [`TestRFC9069ReloadTurningLocRIBOnAnnouncesTheRouterIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L1208) | unit/verify | unproven |

### [`RFC9069-5.3-1`](#rfc9069-5.3-1)

The Loc-RIB Peer Down notification MUST use reason code 6 (§5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBMPReloadTurningLocRIBOffUnsubscribesAndSaysSo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L1135) | unit/verify | unproven |

### [`RFC9069-x-7`](#rfc9069-x-7)

Peer BGP ID MUST be Local Router ID for Loc-RIB (Peer Type 3) (Per-Peer Header)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestLocRIBPeerHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_locrib_test.go#L82) | unit/verify | unproven |

### [`RFC9069-4.2-1`](#rfc9069-4.2-1)

Locally sourced routes communicated by BMP are conveyed using the Loc-RIB Instance Peer Type (§4.2). Proven positive by TestLocRIBFeedConveysRoutesWithTheLocRIBPeerType, which reads the Peer Up and the Route Monitoring one best change puts on the wire and requires Peer Type 3 on both, and negative by TestPeerHeaderFromEvent, where the header of a monitored BGP peer carries Peer Type 0: the type discriminates the Loc-RIB feed rather than being stamped on every message

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMonitoredPeerRouteMonitoringIsNotTheLocRIBPeerType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L156) | unit/verify | revert, verified |
| positive | [`TestLocRIBFeedConveysRoutesWithTheLocRIBPeerType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L135) | unit/verify | revert, verified |

### [`RFC9069-5.1-1`](#rfc9069-5.1-1)

The Loc-RIB per-peer header Timestamp is the time the encapsulated routes were installed in the Loc-RIB, and zero where ze does not know it (§5.1). Ze knows it for an incremental best change, which the RIB delivers on the goroutine that installed it; a full-table replay, a Peer Up, a Peer Down and an End-of-RIB marker each carry zero, which the same paragraph defines: "If zero, the time is unavailable." Proven positive by TestLocRIBIncrementalRouteMonitoringCarriesTheInstallTime, negative by TestLocRIBReplayAndPeerUpCarryNoTimestamp

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLocRIBReplayAndPeerUpCarryNoTimestamp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L111) | unit/verify | revert, verified |
| positive | [`TestLocRIBIncrementalRouteMonitoringCarriesTheInstallTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L87) | unit/verify | revert, verified |

### [`RFC9069-5.2.1-1`](#rfc9069-5.2.1-1)

The Loc-RIB Peer Up carries the VRF/Table Name Information TLV (type 3), whose UTF-8 value is "global" -- the name §5.2.1 fixes for the default Loc-RIB instance with a zero-filled distinguisher -- inside the 1-to-255-byte size bound, and the Loc-RIB Peer Down repeats that TLV after reason 6 (§5.2.1, §5.3, §8.3). Proven positive by TestLocRIBPeerUpAndPeerDownCarryTheGlobalTableName, which reads both messages of one session off the wire, and negative by TestMonitoredPeerUpCarriesNoTableNameTLV: a monitored BGP peer's Peer Up carries no such TLV, so the name is the Loc-RIB instance's rather than every peer's

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMonitoredPeerUpCarriesNoTableNameTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L252) | unit/verify | revert, verified |
| positive | [`TestLocRIBPeerUpAndPeerDownCarryTheGlobalTableName`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L186) | unit/verify | revert, verified |

### [`RFC9069-6.1.1-1`](#rfc9069-6.1.1-1)

The BMP receiver processes the capabilities of a Peer Up's OPEN to know which address families that peer carries, and records the association where `show bmp peers` reports it (§6.1.1). Proven positive by TestReceiverRecordsThePeerUpAddressFamilies and negative by TestReceiverRecordsNoFamiliesWhenThePeerUpAdvertisesNone: an OPEN advertising no Multiprotocol capability records no family, because a defaulted one would answer the association with an invention

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReceiverRecordsNoFamiliesWhenThePeerUpAdvertisesNone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L317) | unit/verify | revert, verified |
| positive | [`TestReceiverRecordsThePeerUpAddressFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L284) | unit/verify | revert, verified |

### [`RFC9069-6.1.3-1`](#rfc9069-6.1.3-1)

A change that alters the behavior of an existing BMP session bounces it with a Peer Down / Peer Up sequence (§6.1.3). Ze bounces the peers inside the session and leaves the session up, which is the same answer RFC8671-7.2-1 records for the identical sentence in RFC 8671 §7.2; the Loc-RIB emulated peer is bounced by the pair of reload rails, a Peer Down when `loc-rib` goes off and a Peer Up when it comes on. Proven positive by TestRFC8671BehaviorChangeBouncesEachPeerAndKeepsTheSession and negative by TestRFC8671UnrelatedBGPChangeBouncesNothing: the bounce is owed to a CHANGE, so a reload that moves no leaf deciding what the session carries bounces nothing

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAConfigurationThatAltersNoBehaviorBouncesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L492) | unit/verify | revert, verified |
| positive | [`TestBehaviorChangeBouncesThePeersOfALocRIBSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L456) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6 phase 5, rfc9069 |
| Signed off | 2026-09-01 |
| Register | rfc2119 |
| Source | rfc/full/rfc9069.txt |
| Source fingerprint | 94c44432aead6e40 |
| Record | rfc/extraction/rfc9069.json |
| Mapped sentences | 18 |
| Declined as scope | 11 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Abstract, Status of This Memo, Copyright Notice and Table of Contents. The Abstract restates section 1 and states no obligation. |
| `1` | Introduction | 0 | walked | Introduction. Indicative prose: BMP defines no method to send the Loc-RIB, three use cases for Loc-RIB access, and the statement that this document replaces Section 8.2 of RFC 7854. No sentence directs a speaker. |
| `1.1` | Alternative Method to Monitor Loc-RIB | 0 | walked | Alternative Method to Monitor Loc-RIB. Argues why deriving a Loc-RIB from a second router's Adj-RIB-In pre-policy is complex and error prone. Wholly indicative; no directive. |
| `2` | Terminology | 0 | walked | Terminology. The BCP 14 key-words paragraph, which binds the key words only when they appear in all capitals. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. |
| `3` | Definitions | 1 | walked | Definitions. Five terms: BGP Instance, Adj-RIB-In and Adj-RIB-Out quoted from RFC 4271, Loc-RIB quoted from RFC 4271 Section 1.1, and the pre- and post-policy Adj-RIB-Out pair. The one capitalised keyword sits in the post-policy Adj-RIB-Out definition and is excluded below; RFC 9069 never uses that term again (the only 'Post-Policy' occurrence in the source is this definition) and states no Adj-RIB-Out obligation of its own. |
| `4` | Per-Peer Header | 0 | walked | Per-Peer Header. A heading with no body text: 4.1 and 4.2 carry the section. |
| `4.1` | Peer Type | 0 | walked | Peer Type. Registers Peer Type 3, Loc-RIB Instance Peer, and contrasts it with the RFC 7854 Section 4.2 Local Instance Peer. Stated indicatively as a value definition, so the site scan sees nothing; ze's PeerTypeLocRIB (internal/component/bgp/plugins/bmp/header.go) is that value, and the obligation to USE it is the first site of section 4.2. |
| `4.2` | Peer Flags | 3 | walked | Peer Flags. Redefines the per-peer header flags byte for Peer Type 3: bit 7 is the F flag and bits 0 to 6 are reserved. Three capitalised MUSTs, all three classified below: the peer type locally sourced routes are conveyed with, the F flag under a filter ze does not offer, and the reserved bits. |
| `5` | Loc-RIB Monitoring | 0 | walked | Loc-RIB Monitoring. Two indicative paragraphs: what the Loc-RIB contains, per RFC 4271 Section 9.1 and 9.4, and that a subset MAY be sent by setting the F flag. The MAY gates nothing, and it is the sentence that makes Loc-RIB filtering optional, which two exclusions below quote. |
| `5.1` | Per-Peer Header | 1 | walked | Per-Peer Header. One capitalised MUST introducing an indented definition list of six field values: Peer Type, Peer Distinguisher, Peer Address, Peer Autonomous System, Peer BGP ID and Timestamp. The site scan sees the introducing sentence only, and the six values are stated indicatively beneath it, so the sentence is the single site and it is mapped below. The three values rfc/short/rfc9069.md renders as rows of their own -- RFC9069-x-5 Peer Address, RFC9069-x-6 Peer AS, RFC9069-x-7 Peer BGP ID -- are read from those indicative lines and are listed unsourced here. |
| `5.2` | Peer Up Notification | 1 | walked | Peer Up Notification. Clarifies Section 4.10 of RFC 7854 for the Loc-RIB peer: Local Address zero-filled, Local Port and Remote Port 0, a fabricated sent OPEN whose capabilities carry one capitalised MUST (mapped below), and a received OPEN that repeats it. The zero addresses and ports and the repeated received OPEN are stated indicatively, so the site scan cannot see them; RFC9069-x-3 is read from the Received OPEN sentence and is listed unsourced here. |
| `5.2.1` | Peer Up Information | 11 | walked | Peer Up Information. Defines Peer Up Information TLV type 3, VRF/Table Name. Eleven capitalised MUSTs. The RFC states the paragraph twice, once inside the bullet and once outside it, so sites 6 to 10 repeat sites 1 to 5 word for word and are excluded as duplicates of the five originals. Sites 1 to 5 are mapped to RFC9069-5.2.1-1: ze's Loc-RIB IS the default instance with a zero-filled distinguisher, so "The default value of "global" MUST be used" names it directly, and ze sends that TLV on the Peer Up and repeats it on the Peer Down (locRIBTableNameTLV, ensureLocRIBPeerUp, primeLocRIBPeerUp and sendLocRIBPeerDown, internal/component/bgp/plugins/bmp/bmp_locrib.go). Site 11 is the only one the VRF scope decision reaches: it governs MULTIPLE names for one Loc-RIB. |
| `5.3` | Peer Down Notification | 4 | walked | Peer Down Notification. Four capitalised MUSTs. The reason-code-6 sentence is mapped to RFC9069-5.3-1. The other three restate the VRF/Table Name TLV's content and size rules and require the TLV in the Peer Down when it was in the Peer Up; all three are mapped to RFC9069-5.2.1-1, whose test reads the Peer Up and the Peer Down of one session off the wire. |
| `5.4` | Route Monitoring | 0 | walked | Route Monitoring. Two indicative sentences: Route Monitoring is used for initial synchronization of the Loc-RIB and for incremental changes, and the per-peer header is followed by a BGP Update PDU, quoted from Section 4.6 of RFC 7854. No capitalised keyword, so the site scan sees nothing; RFC9069-x-4, which records that Loc-RIB monitoring starting triggers a full-table dump, is read from the initial-synchronization sentence and is listed unsourced here. |
| `5.4.1` | ASN Encoding | 1 | walked | ASN Encoding. One sentence, one capitalised MUST, mapped below to RFC9069-5.4.1-1. |
| `5.4.2` | Granularity | 0 | walked | Granularity. State compression and throttling SHOULD be used, with a worked example, and a receiver should expect granularity to vary. A SHOULD and two indicative sentences; none gates. ze applies no state compression: handleBestChange (internal/component/bgp/plugins/bmp/bmp_locrib.go) writes one Route Monitoring per entry of each best-change batch, which is permitted. |
| `5.5` | Route Mirroring | 0 | walked | Route Mirroring. States that verbatim duplication is not applicable to the Loc-RIB because the PDUs are originated by the router, and that received Route Mirroring messages SHOULD be ignored. A SHOULD; it never gates. ze's receiver meets it anyway: processRouteMirroring (internal/component/bgp/plugins/bmp/bmp.go) logs the message and acts on nothing. |
| `5.6` | Statistics Report | 0 | walked | Statistics Report. Lists the two Stat Types relevant to the Loc-RIB, 8 and 10, as value definitions. No capitalised keyword and no directive: nothing here requires a sender to emit either. ze declares both constants (StatRoutesLocRIB, StatRoutesPerAFILocRIB, internal/component/bgp/plugins/bmp/tlv.go) and emits neither, which this section permits. |
| `6` | Other Considerations | 0 | walked | Other Considerations. A heading with no body text. |
| `6.1` | Loc-RIB Implementation | 0 | walked | Loc-RIB Implementation. One indicative paragraph: the implementation emulates a peer with Peer Up, Peer Down and Route Monitoring messages. No directive. |
| `6.1.1` | Multiple Loc-RIB Peers | 3 | walked | Multiple Loc-RIB Peers. Three capitalised MUSTs, all three mapped below: at least one emulated peer per Loc-RIB instance, a Peer Up per emulated peer carrying the address-family capabilities, and the receiver's duty to process them. The section's other two sentences are indicative and name what identifies a Loc-RIB -- "the peer header distinguisher and BGP ID" -- which is what bmpCompositeKey (internal/component/bgp/plugins/bmp/bmp.go) now keys a Peer Type 3 peer by. |
| `6.1.2` | Filtering Loc-RIB to BMP Receivers | 1 | walked | Filtering Loc-RIB to BMP Receivers. Describes the F flag's use case, then one capitalised MUST conditioned on multiple filters against the same Loc-RIB. Excluded below: ze offers no Loc-RIB filter, which section 5 makes optional. |
| `6.1.3` | Changes to Existing BMP Sessions | 1 | walked | Changes to Existing BMP Sessions. One sentence, one capitalised MUST: a change that alters the behavior of an existing BMP session bounces it with a Peer Down / Peer Up sequence. Mapped below to RFC9069-6.1.3-1. RFC 8671 Section 7.2 states the same obligation word for word and ze declares that one as RFC8671-7.2-1; this row is RFC 9069's own, because a cross-document exclusion would dismiss a sentence that binds ze here. |
| `7` | Security Considerations | 0 | walked | Security Considerations. Imports Section 11 of RFC 7854, states that implementations SHOULD require sessions only with authorized and trusted monitoring devices, and states that this document adds no further consideration. The SHOULD is advisory and never gates. |
| `8` | IANA Considerations | 0 | walked | IANA Considerations. Names the BMP Parameters registry the five subsections write to. Walked rather than skipped because 8.3 below carries two derived sites, and a reader who saw the parent skipped would not look for them. |
| `8.1` | Registration of BMP Peer Type 3, Loc-RIB Instance Peer | 0 | skipped (iana) | Registration of BMP Peer Type 3, Loc-RIB Instance Peer. Binds IANA, not a speaker. |
| `8.2` | not stated | 0 | skipped (iana) | Creation of the 'BMP Peer Flags for Loc-RIB Instance Peer Type 3' registry and registration of flag 0, the F flag. Binds IANA, not a speaker. |
| `8.3` | not stated | 2 | walked | Registration of Peer Up Information TLV type 3, VRF/Table Name. The registry action binds IANA, but the section restates the TLV's content and size rules word for word and the derivation reads two sites from that restatement. Walked rather than skipped so both are visible; both are excluded below as duplicates of the section 5.2.1 originals. |
| `8.4` | not stated | 0 | skipped (iana) | Registration of BMP Peer Down reason code 6, 'Local system closed, TLV data follows'. Binds IANA, not a speaker; the obligation to USE it is section 5.3. |
| `8.5` | not stated | 0 | skipped (iana) | Deprecation of the F Flag entry in the 'BMP Peer Flags for Peer Types 0 through 2' registry. Binds IANA, not a speaker. |
| `9` | References | 0 | skipped (references) | References. A heading over 9.1 and 9.2. |
| `9.1` | Normative References: RFC 2119, RFC 4271, RFC 7854, RFC 8174 | 0 | skipped (references) | Normative References: RFC 2119, RFC 4271, RFC 7854, RFC 8174. |
| `9.2` | Informative References: RFC 7911 | 0 | skipped (references) | Informative References: RFC 7911. The section also absorbs the Acknowledgements and Authors' Addresses blocks, neither of which states an obligation. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | Section 3 is the Definitions glossary and this sentence completes the entry for 'Post-Policy Adj-RIB-Out'. The obligation is RFC 8671's: RFC 8671 Section 3 carries the identical glossary sentence and RFC 8671 Section 5.1 states it as the requirement, which ze declares as RFC8671-5.1-1 in rfc/short/rfc8671.md and rfc/extraction/rfc8671.json maps site 5.1:1 to. RFC 9069 defines no Adj-RIB-Out behavior of its own and never uses the term again -- this definition is the only occurrence of 'Post-Policy' in the source. | This MUST be what is actually sent to the peer. |
| `4.2:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "This MUST be set when a filter is applied to Loc-RIB routes sent to the BMP collector." The obligation is conditional on applying a filter. RFC 9069 Section 5 makes Loc-RIB filtering optional: "As described in Section 6.1.2, a subset of Loc-RIB routes MAY be sent to a BMP collector by setting the F flag." ze sends the whole Loc-RIB and offers no filter: ze-bmp-conf.yang (internal/component/bgp/plugins/bmp/yang/) carries one loc-rib boolean and no filter leaf, route-monitoring-policy governs the Adj-RIB feed alone, and handleBestChange (internal/component/bgp/plugins/bmp/bmp_locrib.go) writes a Route Monitoring for every entry of every best-change batch. The absent FEATURE is disclosed in docs/features/rfc-status.md, in the RFC 9069 row, as an implementation gap a later scope decision can revisit. locRIBPeerHeader therefore sets Flags 0, and the F bit reports the truth: this Loc-RIB is not filtered. | This MUST be set when a filter is applied to Loc-RIB routes sent to the BMP collector. |
| `5.2.1:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The RFC states the paragraph twice, once inside the type-3 bullet and once outside it. This is site 5.2.1:1 word for word, and 5.2.1:1 maps the row. | The Information field contains a UTF-8 string whose value MUST be equal to the value of the VRF or table name (e.g., RD instance name) being conveyed. |
| `5.2.1:7` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The second statement of site 5.2.1:2, word for word. | The string size MUST be within the range of 1 to 255 bytes. |
| `5.2.1:8` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The second statement of site 5.2.1:3, word for word. | If a name is configured, it MUST be included. |
| `5.2.1:9` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The second statement of site 5.2.1:4, word for word. | The default value of "global" MUST be used for the default Loc-RIB instance with a zero-filled distinguisher. |
| `5.2.1:10` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The second statement of site 5.2.1:5, word for word. | If the TLV is included, then it MUST also be included in the Peer Down notification. |
| `5.2.1:11` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "If multiple strings are included, their ordering MUST be preserved when they are reported." The obligation is conditional on conveying MULTIPLE names for one Loc-RIB, which the same paragraph offers for "alternate or additional names for the same peer" and "a filtered view of a VRF". RFC 9069 Section 5.2.1 makes the name optional and says why: "The VRF/Table Name TLV is optionally included to support implementations that may not have defined a name." ze runs exactly one Loc-RIB, the default global instance, and offers no VRF and no second named table (owner decision, 2026-09-01: VRF is not in scope for ze at this time). locRIBTableName (internal/component/bgp/plugins/bmp/bmp_locrib.go) is the single constant "global" and locRIBPeerHeader leaves the distinguisher zero, so ze conveys one name for one instance. The absent FEATURE is disclosed in docs/features/rfc-status.md, in the RFC 9069 row, as an implementation gap a later scope decision can revisit. | If multiple strings are included, their ordering MUST be preserved when they are reported. |
| `6.1.2:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "If multiple filters are associated with the same Loc-RIB, a table name MUST be used in order to allow a BMP receiver to make the right associations." The obligation is conditional on associating multiple filters with one Loc-RIB. RFC 9069 Section 5 makes Loc-RIB filtering optional: "As described in Section 6.1.2, a subset of Loc-RIB routes MAY be sent to a BMP collector by setting the F flag." ze sends the whole Loc-RIB and offers no filter: ze-bmp-conf.yang (internal/component/bgp/plugins/bmp/yang/) carries one loc-rib boolean and no filter leaf, route-monitoring-policy governs the Adj-RIB feed alone, and handleBestChange (internal/component/bgp/plugins/bmp/bmp_locrib.go) writes a Route Monitoring for every entry of every best-change batch. The absent FEATURE is disclosed in docs/features/rfc-status.md, in the RFC 9069 row, as an implementation gap a later scope decision can revisit. | If multiple filters are associated with the same Loc-RIB, a table name MUST be used in order to allow a BMP receiver to make the right associations. |
| `8.3:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The IANA registration restates the content rule of site 5.2.1:1 word for word. The registry action itself binds IANA; this sentence adds no obligation the section 5.2.1 original does not already state. | The Information field contains a UTF-8 string whose value MUST be equal to the value of the VRF or table name (e.g., RD instance name) being conveyed. |
| `8.3:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The IANA registration restates the size rule of site 5.2.1:2 word for word. | The string size MUST be within the range of 1 to 255 bytes. |

## Superseded

No document obsoletes RFC 9069, so its obligations are stated where they were written.
