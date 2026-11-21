# RFC 9069 - Support for Local RIB in the BGP Monitoring Protocol (BMP)

BMP Loc-RIB monitoring. Every requirement listed for RFC 9069, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 7 | of 16 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 9 | of 16 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 16 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 16 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 16 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 29 | 29 of 30 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 16 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 16 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 16 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 16 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 16 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 16 | of 16 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 16 mandatory requirements: they add up to 16. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | BMP Loc-RIB monitoring |
| Automatic checks | Included |
| Requirements | 16 |
| Mandatory requirements (MUST level) | 16 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 30 |
| Tests linked to requirements | 30 |
| Recorded audit verdicts | 16 |
| Fault test records | 29 |
| Summary | `rfc/short/rfc9069.md` |
| Requirement table | `rfc/requirements/rfc9069.md` |
| RFC text | `rfc/full/rfc9069.txt` |

## Automatic checks

Included in automatic checks: Support for Local RIB in the BGP Monitoring Protocol (BMP): fifteen MUST-level requirements, all met (ze implements the Loc-RIB Instance Peer type, on the sender side and the receiver side). The per-peer header carries Flags 0 (x-1), a zero Peer Address (x-5), the router's own 4-octet ASN as Peer AS (x-6), the local router-id as Peer BGP ID (x-7) and the install time of the routes it encapsulates or zero where that is unknown (5.1-1); both values come from `bgp router-id` and `bgp session asn local`, so they are known before any BGP peer comes up, which is the case Section 1.1 names. The Peer Up carries a fabricated OPEN repeated into the received field (x-3) advertising the 4-octet ASN capability, one address family per family the dump delivers, and the Extended Next Hop capability for IPv4 unicast with an IPv6 next hop (5.2-1), plus the VRF/Table Name TLV naming that Loc-RIB `global` and repeated on the Peer Down (5.2.1-1); the Peer Down uses reason code 6 (5.3-1). Route Monitoring conveys locally sourced routes under the Loc-RIB peer type (4.2-1) with 4-byte ASN encoding (5.4.1-1), one Peer Up per RIB instance per session (x-2), a full-table dump when monitoring starts (x-4), and a Peer Down / Peer Up bounce when a change alters what the session carries (6.1.3-1). The receiver reads the address families off a Peer Up OPEN, for every peer type including 3, and reports them per peer (6.1.1-1). Tests bound per requirement in rfc/requirements/rfc9069.md.

## Published support status

**Area:** BMP Loc-RIB monitoring

**Test verification:** 16 of 16 MUST proven

**Evidence:** unit 16

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 7 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 9 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **16** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (7):** [`RFC9069-5.4.1-1`](#rfc9069-5.4.1-1), [`RFC9069-x-6`](#rfc9069-x-6), [`RFC9069-4.2-1`](#rfc9069-4.2-1), [`RFC9069-5.1-1`](#rfc9069-5.1-1), [`RFC9069-5.2.1-1`](#rfc9069-5.2.1-1), [`RFC9069-6.1.1-1`](#rfc9069-6.1.1-1), [`RFC9069-6.1.3-1`](#rfc9069-6.1.3-1)

**Annotated (including scoped evidence) (9):** [`RFC9069-x-1`](#rfc9069-x-1), [`RFC9069-x-2`](#rfc9069-x-2), [`RFC9069-x-3`](#rfc9069-x-3), [`RFC9069-x-4`](#rfc9069-x-4), [`RFC9069-x-5`](#rfc9069-x-5), [`RFC9069-5.2-1`](#rfc9069-5.2-1), [`RFC9069-5.3-1`](#rfc9069-5.3-1), [`RFC9069-x-7`](#rfc9069-x-7), [`RFC9069-6.1.1-2`](#rfc9069-6.1.1-2)

## Figures

- **MUST bar**: Ze verified 16
- **Overview bar**: Ze MUST verified 16

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 16 |
| MUST rows kept (the MUST bar) | 16 |
| Ze verified | 16 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 0 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 0 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc9069.json](../../../data/rfc/rfc9069.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9069-x-1` | The unused bits are reserved for future use. They MUST be transmitted as 0 (§4.2) | MUST | 4.2 - Peer Flags | **positive:** `unit/verify` [`TestLocRIBPeerHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go#L82). **negative:** no negative test. **{single-polarity}:** the Loc-RIB PeerType=3 per-peer header is constructed with Flags 0 (locRIBPeerHeader, internal/component/bgp/plugins/bmp/bmp_locrib.go) and no code path ever sets the V/L/A/O bits for a Loc-RIB header, so there is no non-conformant flag-setting case to assert as a negative; the positive (Flags==0) is proven in TestLocRIBPeerHeader |
| `RFC9069-x-2` | There MUST be at least one emulated peer for each Loc-RIB instance, such as with VRFs. (§6.1.1) | MUST | 6.1.1 - Multiple Loc-RIB Peers | **positive:** `unit/verify` [`TestLocRIBSinglePeerUpPerInstance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go#L538). **negative:** no negative test. **{single-polarity}:** the Loc-RIB Peer Up is emitted once per RIB instance per collector session, behind the one-shot senderSession.locRIBUpSent.CompareAndSwap taken by ensureLocRIBPeerUp and primeLocRIBPeerUp (internal/component/bgp/plugins/bmp/bmp_locrib.go), and best changes from any number of BGP peers pass through that same guard. Corrected 2026-09-01: this row named BMPPlugin.locRIBUp as the one-shot, which is set AFTER the announcement and records only that Loc-RIB monitoring has been announced at all -- it is what sendLocRIBPeerDown keys off, and it guards nothing, so "one Peer Up per instance" is a single count-equals-one assertion with no separate per-peer path to reject; TestLocRIBSinglePeerUpPerInstance drives two peers' best changes and asserts exactly one Peer Up |
| `RFC9069-x-3` | Received OPEN Message: Repeat of the same sent OPEN message. (§5.2) | MUST | 5.2 - Peer Up Notification | **positive:** `unit/verify` [`TestHandleBestChangeEmitsPeerUpThenRM`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go#L471). **negative:** no negative test. **{single-polarity}:** ensureLocRIBPeerUp and primeLocRIBPeerUp (internal/component/bgp/plugins/bmp/bmp_locrib.go) are the only two producers of a Loc-RIB Peer Up and each passes one fabricateLocRIBOpen result to BOTH fields, so there is no path that sends a different received OPEN to assert as a negative; the positive (both fields present, equal, and decoding as a BGP OPEN) is proven in TestHandleBestChangeEmitsPeerUpThenRM. Corrected 2026-08-31: this row declared "zero-length OPEN messages", which RFC 9069 Section 5.2 requires the opposite of, and its test asserted the defect |
| `RFC9069-x-4` | Route Monitoring messages are used for initial synchronization of the Loc-RIB. (§5.4, Route Monitoring) | MUST | 5.4 - Route Monitoring | **positive:** `unit/verify` [`TestStartLocRIBDeliversTheInitialDumpAsRouteMonitoring`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_verdict_test.go#L215). **positive:** `unit/verify` [`TestStartLocRIBTriggersInitialDump`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go#L641). **negative:** no negative test. **{single-polarity}:** startLocRIB in internal/component/bgp/plugins/bmp/bmp_locrib.go emits exactly one replay-request carrying a per-dump token from nextDumpToken, and the RIB's replayBestPaths walks its whole best-path table for any token, so the initial synchronization either happens or the subscription is absent and no non-conformant input exists to reject. TestStartLocRIBDeliversTheInitialDumpAsRouteMonitoring proves the dump reaches the collector: after the Loc-RIB Peer Up, the replayed route arrives as a Route Monitoring of Peer Type 3 carrying its prefix. TestStartLocRIBTriggersInitialDump proves the trigger: one replay-request with a token, and distinct tokens for successive dumps |
| `RFC9069-x-5` | Peer Address: Zero-filled. The remote peer address is not applicable. (§5.1) | MUST | 5.1 - Per-Peer Header | **positive:** `unit/verify` [`TestLocRIBPeerHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go#L99). **negative:** no negative test. **{single-polarity}:** the Loc-RIB per-peer header leaves Peer Address all-zero by construction (locRIBPeerHeader, internal/component/bgp/plugins/bmp/bmp_locrib.go, no Address assignment) and no path sets it, so there is no non-zero-address case to assert as a negative; the positive (Address==0) is proven in TestLocRIBPeerHeader |
| `RFC9069-5.4.1-1` | Loc-RIB Route Monitoring messages MUST use a 4-byte ASN encoding as indicated in the Peer Up sent OPEN message (Section 5.2) capability. (§5.4.1). Proven in both polarities by TestLocRIBRouteMonitoringUsesFourByteASNs: the encoded AS_PATH segment is 4 octets per ASN, and an ASN above 65535 round-trips whole, which a 2-byte encoding cannot represent at all | MUST | 5.4.1 - ASN Encoding | **positive:** `unit/verify` [`TestLocRIBRouteMonitoringUsesFourByteASNs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go#L163). **negative:** `unit/verify` [`TestLocRIBRouteMonitoringUsesFourByteASNs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go#L166) |
| `RFC9069-5.2-1` | Sent OPEN Message: This is a fabricated BGP OPEN message. Capabilities MUST include the 4-octet ASN and all necessary capabilities to represent the Loc-RIB Route Monitoring messages. Only include capabilities if they will be used for Loc-RIB monitoring messages. (§5.2) | MUST | 5.2 - Peer Up Notification | **positive:** `unit/verify` [`TestFabricatedLocRIBOpenCarriesOnlyCapabilitiesTheRouteMonitoringUses`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_verdict_test.go#L88). **positive:** `unit/verify` [`TestFabricatedLocRIBOpenCarriesTheRequiredCapabilities`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go#L374). **positive:** `unit/verify` [`TestLocRIBIPv4RouteWithIPv6NextHopIsBackedByExtendedNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_verdict_test.go#L135). **negative:** no negative test. **{single-polarity}:** fabricateLocRIBOpen (internal/component/bgp/plugins/bmp/bmp_locrib.go) builds the capability list from dumpFamilies, the same list the dump closes with End-of-RIB markers, plus one Extended Next Hop capability for IPv4 unicast with an IPv6 next hop, the one Route Monitoring encoding buildLocRIBUpdateBody writes that the Multiprotocol capabilities do not cover. There is no second list that could advertise a family the dump never delivers and no divergent case to assert as a negative. The count is asserted as well as the membership, which is what "Only include capabilities if they will be used for Loc-RIB monitoring messages" forbids going past; proven in TestFabricatedLocRIBOpenCarriesTheRequiredCapabilities, TestFabricatedLocRIBOpenCarriesOnlyCapabilitiesTheRouteMonitoringUses and TestLocRIBIPv4RouteWithIPv6NextHopIsBackedByExtendedNextHop, which encodes that route and requires the capability |
| `RFC9069-x-6` | Peer Autonomous System (AS): Set to the primary router BGP autonomous system number (ASN). (§5.1). Corrected 2026-08-31: this row read "Peer AS MUST be 0", which is what the RFC requires of the Peer ADDRESS, row RFC9069-x-5, and the reverse of what it requires of this field. The value is the 4-octet ASN, read from the `bgp session asn local` leaf rather than from a session that may not exist: an OPEN cache is empty until a peer comes up, which is the very case this RFC exists to serve, and an ASN read from the two-octet My AS field of an AS4 speaker is AS_TRANS rather than the router's. Proven positive by TestLocRIBPeerHeader and by TestRFC9069ReloadTurningLocRIBOnAnnouncesTheRouterIdentity, which commits the leaf over the reload rail with the OPEN cache EMPTY, and negative by TestLocRIBPeerHeaderCarriesTheIdentityItIsGiven | MUST | 5.1 - Per-Peer Header | **positive:** `unit/verify` [`TestLocRIBPeerHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go#L88). **positive:** `unit/verify` [`TestRFC9069ReloadTurningLocRIBOnAnnouncesTheRouterIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L1284). **negative:** `unit/verify` [`TestLocRIBPeerHeaderCarriesTheIdentityItIsGiven`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go#L106) |
| `RFC9069-5.3-1` | The Peer Down notification MUST use reason code 6. (§5.3) | MUST | 5.3 - Peer Down Notification | **positive:** `unit/verify` [`TestBMPReloadTurningLocRIBOffUnsubscribesAndSaysSo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L1194). **negative:** no negative test. **{single-polarity}:** sendLocRIBPeerDown (internal/component/bgp/plugins/bmp/bmp_locrib.go) is the only producer of a Loc-RIB Peer Down and it passes one constant reason, so there is no second Loc-RIB path to reject as a negative. The neighbouring risk -- reason 6 used for a peer the requirement does not cover -- is closed elsewhere: TestRFC8671BehaviorChangeBouncesEachPeerAndKeepsTheSession requires a PeerTypeGlobal Peer Down to carry reason 5, and TestPeerDownReasonMapping requires every mapped BGP close reason to be one of 1 to 5. The positive (reason 6 on the Loc-RIB peer) is proven in TestBMPReloadTurningLocRIBOffUnsubscribesAndSaysSo, over the plugin's own reload rail |
| `RFC9069-x-7` | Peer BGP ID: Set the ID to the router-id of the VRF instance if VRF is used; otherwise, set to the global instance router-id. (§5.1) | MUST | 5.1 - Per-Peer Header | **positive:** `unit/verify` [`TestLocRIBPeerHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go#L94). **negative:** no negative test. **{single-polarity}:** the Loc-RIB per-peer header sets Peer BGP ID to the `bgp router-id` leaf (locRIBPeerHeader and BMPPlugin.localIdentity, internal/component/bgp/plugins/bmp/bmp_locrib.go) and ze has exactly one local router-id, so there is no non-conformant BGP-ID value to reject as a negative; a plugin that has been handed no configuration emits nothing rather than the zero identifier RFC 6286 forbids; the positive (Peer BGP ID == router-id) is proven in TestLocRIBPeerHeader and TestHandleBestChangeEmitsPeerUpThenRM |
| `RFC9069-4.2-1` | If locally sourced routes are communicated using BMP, they MUST be conveyed using the Loc-RIB Instance Peer Type. (§4.2). Proven positive by TestLocRIBFeedConveysRoutesWithTheLocRIBPeerType, which reads the Peer Up and the Route Monitoring one best change puts on the wire and requires Peer Type 3 on both, and by TestLocallySourcedRouteIsConveyedWithTheLocRIBPeerType, which does the same for a route this router originated, one with an empty AS_PATH, and negative by TestLocallySourcedRouteIsNotConveyedUnderTheMonitoredPeer, which pushes the input toward the violation: the locally sourced route names a monitored BGP peer's address as its next hop, and the Peer Up and Route Monitoring still carry Peer Type 3, a zero peer address and the router's own AS and BGP ID | MUST | 4.2 - Peer Flags | **positive:** `unit/verify` [`TestLocRIBFeedConveysRoutesWithTheLocRIBPeerType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L137). **positive:** `unit/verify` [`TestLocallySourcedRouteIsConveyedWithTheLocRIBPeerType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_verdict_test.go#L253). **negative:** `unit/verify` [`TestLocallySourcedRouteIsNotConveyedUnderTheMonitoredPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_verdict_test.go#L278) |
| `RFC9069-5.1-1` | Timestamp: The time when the encapsulated routes were installed in the Loc-RIB, expressed in seconds and microseconds since midnight (zero hour), January 1, 1970 (UTC). If zero, the time is unavailable. (§5.1). Ze knows it for an incremental best change, which the RIB delivers on the goroutine that installed it; a full-table replay, a Peer Up, a Peer Down and an End-of-RIB marker each carry zero, which the same paragraph defines: "If zero, the time is unavailable." Proven positive by TestLocRIBIncrementalRouteMonitoringCarriesTheInstallTime, negative by TestLocRIBReplayAndPeerUpCarryNoTimestamp | MUST | 5.1 - Per-Peer Header | **positive:** `unit/verify` [`TestLocRIBIncrementalRouteMonitoringCarriesTheInstallTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L89). **negative:** `unit/verify` [`TestLocRIBReplayAndPeerUpCarryNoTimestamp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L113) |
| `RFC9069-5.2.1-1` | Type = 3: VRF/Table Name. The Information field contains a UTF-8 string whose value MUST be equal to the value of the VRF or table name (e.g., RD instance name) being conveyed. The string size MUST be within the range of 1 to 255 bytes. The VRF/Table Name TLV is optionally included to support implementations that may not have defined a name. If a name is configured, it MUST be included. The default value of "global" MUST be used for the default Loc-RIB instance with a zero-filled distinguisher. If the TLV is included, then it MUST also be included in the Peer Down notification. (§5.2.1). Proven positive by TestLocRIBPeerUpAndPeerDownCarryTheGlobalTableName, which reads both messages of one session off the wire, and negative by TestMonitoredPeerUpCarriesNoTableNameTLV: a monitored BGP peer's Peer Up carries no such TLV, so the name is the Loc-RIB instance's rather than every peer's | MUST | 5.2.1 - Peer Up Information | **positive:** `unit/verify` [`TestLocRIBPeerUpAndPeerDownCarryTheGlobalTableName`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L187). **negative:** `unit/verify` [`TestMonitoredPeerUpCarriesNoTableNameTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L253) |
| `RFC9069-6.1.1-2` | Each emulated peer instance MUST send a Peer Up with the OPEN message indicating the address family capabilities. (§6.1.1) | MUST | 6.1.1 - Multiple Loc-RIB Peers | **positive:** `unit/verify` [`TestLocRIBPeerUpOpenIndicatesTheAddressFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_verdict_test.go#L193). **negative:** no negative test. **{single-polarity}:** ze has one emulated peer, and every producer of its Peer Up (ensureLocRIBPeerUp, primeLocRIBPeerUp, internal/component/bgp/plugins/bmp/bmp_locrib.go) passes the one fabricateLocRIBOpen result, whose Multiprotocol capabilities are built from dumpFamilies, so no input yields a Loc-RIB Peer Up without the families to refuse. Proven positive by TestLocRIBPeerUpOpenIndicatesTheAddressFamilies, which reads both OPEN fields of the Peer Up off the wire |
| `RFC9069-6.1.1-1` | A BMP receiver MUST process these capabilities to know which peer belongs to which address family. (§6.1.1). Proven positive by TestReceiverRecordsThePeerUpAddressFamilies and by TestReceiverKeepsEachPeersAddressFamiliesApart, where two peers of one router advertise different families and each keeps its own, and negative by TestReceiverRecordsNoFamiliesWhenThePeerUpAdvertisesNone: an OPEN advertising no Multiprotocol capability records no family, because a defaulted one would answer the association with an invention | MUST | 6.1.1 - Multiple Loc-RIB Peers | **positive:** `unit/verify` [`TestReceiverKeepsEachPeersAddressFamiliesApart`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_verdict_test.go#L35). **positive:** `unit/verify` [`TestReceiverRecordsThePeerUpAddressFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L285). **negative:** `unit/verify` [`TestReceiverRecordsNoFamiliesWhenThePeerUpAdvertisesNone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L318) |
| `RFC9069-6.1.3-1` | In case of any change that results in the alteration of behavior of an existing BMP session, i.e., changes to filtering and table names, the session MUST be bounced with a Peer Down / Peer Up sequence. (§6.1.3). Ze bounces the peers inside the session and leaves the session up, which is the same answer RFC8671-7.2-1 records for the identical sentence in RFC 8671 §7.2; a behavior change on a session that keeps `loc-rib` on also bounces the Loc-RIB emulated peer: applySenderConfig in sender_config.go. Proven positive by TestBehaviorChangeBouncesThePeersOfALocRIBSession, for a monitored BGP peer, and TestBehaviorChangeBouncesTheLocRIBEmulatedPeer, for the Peer Type 3 Peer Down then Peer Up, and negative by TestAConfigurationThatAltersNoBehaviorBouncesNothing: the bounce is owed to a CHANGE, so a reload that moves no leaf deciding what the session carries bounces nothing | MUST | 6.1.3 - Changes to Existing BMP Sessions | **positive:** `unit/verify` [`TestBehaviorChangeBouncesTheLocRIBEmulatedPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_verdict_test.go#L313). **positive:** `unit/verify` [`TestBehaviorChangeBouncesThePeersOfALocRIBSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L457). **negative:** `unit/verify` [`TestAConfigurationThatAltersNoBehaviorBouncesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L493) |

## Gaps and untested MUSTs

RFC 9069 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC9069-x-1`](#rfc9069-x-1)

The unused bits are reserved for future use. They MUST be transmitted as 0 (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go::TestLocRIBPeerHeader moved. Forbidden: a reserved (non-F) per-peer flag bit transmitted as 1 on the Loc-RIB header. internal/component/bgp/plugins/bmp/bmp_locrib_test.go::TestLocRIBPeerHeader: ph.Flags != 0 is Errorf, red on any set bit. Single-polarity marker: no producer sets a flag, so no negative input exists. The row claims only the transmit half; the 'ignored on receipt' clause of the same sentence is not quoted and has no row.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestLocRIBPeerHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go#L82) | unit/verify | mutant, verified |

### [`RFC9069-x-2`](#rfc9069-x-2)

There MUST be at least one emulated peer for each Loc-RIB instance, such as with VRFs. (§6.1.1)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go::TestLocRIBSinglePeerUpPerInstance moved. Re-read the full RFC 9069 section 6.1.1 emulated-peer obligation, the current TestLocRIBSinglePeerUpPerInstance, its complete stored-path fixtures, and both Peer Up producers. Two valid best-change batches with distinct prefixes and next hops must produce exactly one Loc-RIB Peer Up plus two non-End-of-RIB Route Monitoring messages; the test reads all three and fails on no peer or a duplicate per-source peer. Its one-shot assertion is stricter than the required at-least-one lower bound for Ze's single supported Loc-RIB instance. The single-polarity annotation correctly names the shared per-collector senderSession.locRIBUpSent guard in ensureLocRIBPeerUp/primeLocRIBPeerUp; both feed producers construct the same emulated instance, rather than exposing a separate input-selected peer-creation mode. Runtime admission now checks validateLocRIBRuntimeConfig before any startup mutation and before any reload candidate is published: a process without the RIB event bus refuses a requested Loc-RIB configuration with its cause and exact internal-plugin remedy. It therefore does not accept an instance whose required peer/dump cannot exist. Supported available-bus monitoring retains the subscription, initial replay and actual collector delivery checked by the existing initial-dump test. The main-thread external functional proof now observes the exact unavailable-bus refusal and internal-plugin remedy, successful daemon SSH, and absence of the refused BMP process from the real subsystem list. The main-thread internal functional proof observes a complete Peer Type 3 UPDATE over the collector transport; its generic collector does not itself assert a prefix or Peer Up ordering. The exact one-peer/two-route count and replayed-prefix delivery assertions remain in the independently read unit collector tests, which also passed in the main-thread full BMP run. These together discharge the runtime-mode concern without changing the daemon's existing nonfatal plugin-refusal policy.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestLocRIBSinglePeerUpPerInstance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go#L538) | unit/verify | mutant, verified |

### [`RFC9069-x-3`](#rfc9069-x-3)

Received OPEN Message: Repeat of the same sent OPEN message. (§5.2)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go::TestHandleBestChangeEmitsPeerUpThenRM moved. Re-read RFC 9069 section 5.2 and the current TestHandleBestChangeEmitsPeerUpThenRM after its fixture gained the complete stored four-octet AS_PATH for the same ASN 65001. The collector must read a Peer Up with a nonempty sent OPEN, byte-identical received OPEN, decodable BGP OPEN, the local router-id, ASN4 capability and a family capability before Route Monitoring. Empty OPENs, a different received OPEN, undecodable bytes or lost capabilities fail exact assertions. The single-polarity annotation is valid: ensureLocRIBPeerUp and primeLocRIBPeerUp each build one fabricateLocRIBOpen result and pass that same byte slice to both OPEN fields; there is no separate valid input selecting a received OPEN. The fixture repair changes neither these assertions nor their coverage.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestHandleBestChangeEmitsPeerUpThenRM`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go#L471) | unit/verify | mutant, verified |

### [`RFC9069-x-4`](#rfc9069-x-4)

Route Monitoring messages are used for initial synchronization of the Loc-RIB. (§5.4, Route Monitoring)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go::TestStartLocRIBTriggersInitialDump moved. Re-judged 2026-09-29 (row prose rewritten, claim unchanged). rfc9069_verdict_test.go::TestStartLocRIBDeliversTheInitialDumpAsRouteMonitoring subscribes a stand-in RIB that answers startLocRIB's replay-request with one best change carrying its token, and requires after the Loc-RIB Peer Up a Route Monitoring of Peer Type 3 carrying 10.20.30.0/24: the initial synchronization arrives as Route Monitoring. bmp_locrib_test.go::TestStartLocRIBTriggersInitialDump asserts the trigger and its per-dump tokens; it carries no discrimination record, the delivery unit does (startLocRIB break). {single-polarity: positive} holds: no non-conformant input to refuse.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestStartLocRIBTriggersInitialDump`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go#L641) | unit/verify | not verified |
| positive | [`TestStartLocRIBDeliversTheInitialDumpAsRouteMonitoring`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_verdict_test.go#L215) | unit/verify | revert, verified |

### [`RFC9069-x-5`](#rfc9069-x-5)

Peer Address: Zero-filled. The remote peer address is not applicable. (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go::TestLocRIBPeerHeader moved. Forbidden: a non-zero Peer Address on the Loc-RIB header. internal/component/bgp/plugins/bmp/bmp_locrib_test.go::TestLocRIBPeerHeader: ph.Address != [16]byte{} is Errorf. Single-polarity marker.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestLocRIBPeerHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go#L99) | unit/verify | mutant, verified |

### [`RFC9069-5.4.1-1`](#rfc9069-5.4.1-1)

Loc-RIB Route Monitoring messages MUST use a 4-byte ASN encoding as indicated in the Peer Up sent OPEN message (Section 5.2) capability. (§5.4.1). Proven in both polarities by TestLocRIBRouteMonitoringUsesFourByteASNs: the encoded AS_PATH segment is 4 octets per ASN, and an ASN above 65535 round-trips whole, which a 2-byte encoding cannot represent at all

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go::TestLocRIBRouteMonitoringUsesFourByteASNs, internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go::TestLocRIBRouteMonitoringUsesFourByteASNs#2 moved. Re-judged 2026-10-23. The Route Monitoring UPDATE now carries the best path's stored attributes, so AGGREGATOR became a second ASN carrier and the unit was extended to it. TestLocRIBRouteMonitoringUsesFourByteASNs asserts: AS_PATH value 2+4n octets (Fatal) with ASN 4200000001 whole (Errorf); AGGREGATOR value 8 octets with AS 4200000001 (Fatal/Errorf); AS4_PATH and AS4_AGGREGATOR absent (RFC 6793 Section 4.1), red observed with the attrCodeAfter AS4 filter removed. The stored AGGREGATOR's four-octet form is established at ingest (attribute.ReconcileASPathFamily, per storage.ParseAttributes), outside this unit: the unit proves the encoder carries it as stored and adds no two-octet companion. Both polarity tags on one unit.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestLocRIBRouteMonitoringUsesFourByteASNs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go#L166) | unit/verify | revert, verified |
| positive | [`TestLocRIBRouteMonitoringUsesFourByteASNs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go#L163) | unit/verify | revert, verified |

### [`RFC9069-5.2-1`](#rfc9069-5.2-1)

Sent OPEN Message: This is a fabricated BGP OPEN message. Capabilities MUST include the 4-octet ASN and all necessary capabilities to represent the Loc-RIB Route Monitoring messages. Only include capabilities if they will be used for Loc-RIB monitoring messages. (§5.2)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go::TestFabricatedLocRIBOpenCarriesTheRequiredCapabilities moved. Re-read the whole RFC 9069 section 5.2 fabricated-OPEN sentence, the single-polarity annotation, all three tagged tests and fabricateLocRIBOpen/buildLocRIBUpdateBody. The required-capability test decodes the fabricated BGP OPEN and requires the router's 4200000001 ASN4 capability and the dump's exact family set. The separate capability census requires exactly one ASN4 capability, the exact Multiprotocol list, exactly the one IPv4-unicast/IPv6-next-hop pair and no other capability code. The IPv4-with-IPv6-next-hop test now supplies a complete stored ORIGIN plus valid empty AS_PATH, proves the produced MP_REACH_NLRI uses AFI 1 with a 16-octet next hop, and requires the OPEN's matching Extended Next Hop capability. Thus missing, surplus and representation-incompatible capabilities fail distinct assertions. The single producer's fixed capability declaration is shared with the dump families; no separate nonconformant input is needed for the valid single-polarity annotation.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestFabricatedLocRIBOpenCarriesTheRequiredCapabilities`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go#L374) | unit/verify | revert, verified |
| positive | [`TestFabricatedLocRIBOpenCarriesOnlyCapabilitiesTheRouteMonitoringUses`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_verdict_test.go#L88) | unit/verify | revert, verified |
| positive | [`TestLocRIBIPv4RouteWithIPv6NextHopIsBackedByExtendedNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_verdict_test.go#L135) | unit/verify | revert, verified |

### [`RFC9069-x-6`](#rfc9069-x-6)

Peer Autonomous System (AS): Set to the primary router BGP autonomous system number (ASN). (§5.1). Corrected 2026-08-31: this row read "Peer AS MUST be 0", which is what the RFC requires of the Peer ADDRESS, row RFC9069-x-5, and the reverse of what it requires of this field. The value is the 4-octet ASN, read from the `bgp session asn local` leaf rather than from a session that may not exist: an OPEN cache is empty until a peer comes up, which is the very case this RFC exists to serve, and an ASN read from the two-octet My AS field of an AS4 speaker is AS_TRANS rather than the router's. Proven positive by TestLocRIBPeerHeader and by TestRFC9069ReloadTurningLocRIBOnAnnouncesTheRouterIdentity, which commits the leaf over the reload rail with the OPEN cache EMPTY, and negative by TestLocRIBPeerHeaderCarriesTheIdentityItIsGiven

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go::TestLocRIBPeerHeader moved. Forbidden: Peer AS other than the router's primary ASN (zero, a constant, AS_TRANS). internal/component/bgp/plugins/bmp/bmp_locrib_test.go::TestLocRIBPeerHeader PeerAS != 4200000001 Errorf; ::TestLocRIBPeerHeaderCarriesTheIdentityItIsGiven two identities must give 65001 and 65002; internal/component/bgp/plugins/bmp/rfc8671_test.go::TestRFC9069ReloadTurningLocRIBOnAnnouncesTheRouterIdentity reads Peer AS 4200000001 off the wire with the OPEN cache empty.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestLocRIBPeerHeaderCarriesTheIdentityItIsGiven`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go#L106) | unit/verify | mutant, verified |
| positive | [`TestRFC9069ReloadTurningLocRIBOnAnnouncesTheRouterIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L1284) | unit/verify | mutant, verified |
| positive | [`TestLocRIBPeerHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go#L88) | unit/verify | mutant, verified |

### [`RFC9069-5.3-1`](#rfc9069-5.3-1)

The Peer Down notification MUST use reason code 6. (§5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read RFC 9069 section 5.3, the checklist single-polarity annotation, sendLocRIBPeerDown and the full current TestBMPReloadTurningLocRIBOffUnsubscribesAndSaysSo. The fixture now installs a real test event bus before runtime startup, requires startup to establish its subscription, and wraps the actual unsubscribe callback instead of replacing an absent subscription with a counter. The same real SDK reload disables Loc-RIB and the collector must read a Loc-RIB Peer Down with reason exactly PeerDownTLVData (constant 6), followed by Termination. The test also requires the real subscription to be removed and the announced state cleared. A missing Peer Down, wrong peer type, reason other than 6, or a reload that fails to stop the feed fails an exact assertion. The single-polarity annotation remains valid: sendLocRIBPeerDown is the one Loc-RIB Peer Down producer and always supplies reason 6; the existing separate monitored-peer reason assertions exclude misuse outside this conditional Loc-RIB requirement. The new bus fixture supplies the live-feed precondition introduced by runtime admission without weakening any original expectation. The main-thread native producer-halt record establishes only dependence on sendLocRIBPeerDown (the halt is reached at initial configuration); the reason-code judgment rests on the primary RFC text, the unchanged actual wire assertion after reload, and the reviewed constant 6 supplied by the real producer, not on that panic.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestBMPReloadTurningLocRIBOffUnsubscribesAndSaysSo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L1194) | unit/verify | revert, verified |

### [`RFC9069-x-7`](#rfc9069-x-7)

Peer BGP ID: Set the ID to the router-id of the VRF instance if VRF is used; otherwise, set to the global instance router-id. (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go::TestLocRIBPeerHeader moved. Forbidden: Peer BGP ID other than the global instance router-id. internal/component/bgp/plugins/bmp/bmp_locrib_test.go::TestLocRIBPeerHeader PeerBGPID != 0x0a141e01 Errorf. Single-polarity marker. The VRF clause is conditional on a VRF Loc-RIB instance, which ze does not implement (one global instance), so it has no reachable case.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestLocRIBPeerHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go#L94) | unit/verify | mutant, verified |

### [`RFC9069-4.2-1`](#rfc9069-4.2-1)

If locally sourced routes are communicated using BMP, they MUST be conveyed using the Loc-RIB Instance Peer Type. (§4.2). Proven positive by TestLocRIBFeedConveysRoutesWithTheLocRIBPeerType, which reads the Peer Up and the Route Monitoring one best change puts on the wire and requires Peer Type 3 on both, and by TestLocallySourcedRouteIsConveyedWithTheLocRIBPeerType, which does the same for a route this router originated, one with an empty AS_PATH, and negative by TestLocallySourcedRouteIsNotConveyedUnderTheMonitoredPeer, which pushes the input toward the violation: the locally sourced route names a monitored BGP peer's address as its next hop, and the Peer Up and Route Monitoring still carry Peer Type 3, a zero peer address and the router's own AS and BGP ID

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read RFC 9069 section 4.2 and all three tagged tests. The feed test requires Peer Type 3 on the Peer Up and Route Monitoring. The locally sourced positive and negative now carry an explicit present empty AS_PATH in their complete stored attributes, matching their empty flattened summary, so they still exercise a valid local route. They require the advertised 10.20.30.0/24 prefix and Peer Type 3 at the collector. The negative deliberately gives that local route the monitored BGP peer's next hop and additionally requires zero peer address plus the router's own ASN and BGP ID on both headers: attributing it to that monitored peer fails. These inputs isolate locally sourced route attribution rather than relying on malformed or absent mandatory attributes. locRIBPeerHeader and both Peer Up producers preserve that identity independently of the path's source peer.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestLocallySourcedRouteIsNotConveyedUnderTheMonitoredPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_verdict_test.go#L278) | unit/verify | revert, verified |
| positive | [`TestLocRIBFeedConveysRoutesWithTheLocRIBPeerType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L137) | unit/verify | revert, verified |
| positive | [`TestLocallySourcedRouteIsConveyedWithTheLocRIBPeerType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_verdict_test.go#L253) | unit/verify | revert, verified |

### [`RFC9069-5.1-1`](#rfc9069-5.1-1)

Timestamp: The time when the encapsulated routes were installed in the Loc-RIB, expressed in seconds and microseconds since midnight (zero hour), January 1, 1970 (UTC). If zero, the time is unavailable. (§5.1). Ze knows it for an incremental best change, which the RIB delivers on the goroutine that installed it; a full-table replay, a Peer Up, a Peer Down and an End-of-RIB marker each carry zero, which the same paragraph defines: "If zero, the time is unavailable." Proven positive by TestLocRIBIncrementalRouteMonitoringCarriesTheInstallTime, negative by TestLocRIBReplayAndPeerUpCarryNoTimestamp

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a timestamp that is not the install time (a wall clock on a replay), or zero where the install time is known. internal/component/bgp/plugins/bmp/rfc9069_test.go::TestLocRIBIncrementalRouteMonitoringCarriesTheInstallTime: got==0 Fatal and outside [before,after] Errorf; ::TestLocRIBReplayAndPeerUpCarryNoTimestamp: replay RM and Peer Up timestamps must be (0,0). Precision is implementation dependent, so microseconds are not owed.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestLocRIBReplayAndPeerUpCarryNoTimestamp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L113) | unit/verify | revert, verified |
| positive | [`TestLocRIBIncrementalRouteMonitoringCarriesTheInstallTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L89) | unit/verify | revert, verified |

### [`RFC9069-5.2.1-1`](#rfc9069-5.2.1-1)

Type = 3: VRF/Table Name. The Information field contains a UTF-8 string whose value MUST be equal to the value of the VRF or table name (e.g., RD instance name) being conveyed. The string size MUST be within the range of 1 to 255 bytes. The VRF/Table Name TLV is optionally included to support implementations that may not have defined a name. If a name is configured, it MUST be included. The default value of "global" MUST be used for the default Loc-RIB instance with a zero-filled distinguisher. If the TLV is included, then it MUST also be included in the Peer Down notification. (§5.2.1). Proven positive by TestLocRIBPeerUpAndPeerDownCarryTheGlobalTableName, which reads both messages of one session off the wire, and negative by TestMonitoredPeerUpCarriesNoTableNameTLV: a monitored BGP peer's Peer Up carries no such TLV, so the name is the Loc-RIB instance's rather than every peer's

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Loc-RIB Peer Up without the type-3 TLV, a name other than 'global' for the zero-distinguisher default instance, a value outside 1..255 bytes, or a Peer Down missing the TLV. internal/component/bgp/plugins/bmp/rfc9069_test.go::TestLocRIBPeerUpAndPeerDownCarryTheGlobalTableName: Distinguisher 0, exactly one TLV, assertTableNameTLV (type, value, 1..255, length field) on Peer Up and on the Peer Down after reason 6. Negative ::TestMonitoredPeerUpCarriesNoTableNameTLV. 'If a name is configured' is conditional on a table-name leaf ze does not have.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestMonitoredPeerUpCarriesNoTableNameTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L253) | unit/verify | revert, verified |
| positive | [`TestLocRIBPeerUpAndPeerDownCarryTheGlobalTableName`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L187) | unit/verify | revert, verified |

### [`RFC9069-6.1.1-2`](#rfc9069-6.1.1-2)

Each emulated peer instance MUST send a Peer Up with the OPEN message indicating the address family capabilities. (§6.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. First verdict 2026-09-29. rfc9069_verdict_test.go::TestLocRIBPeerUpOpenIndicatesTheAddressFamilies drives one best change, reads the Loc-RIB Peer Up off the collector end of the pipe (Peer Type 3), and requires both the sent and the received OPEN to carry Multiprotocol capabilities equal to dumpFamilies, the families the emulated peer's Route Monitoring and End-of-RIB carry. {single-polarity: positive} holds: ze has one emulated peer and every producer of its Peer Up passes the one fabricateLocRIBOpen result, so no input yields a Loc-RIB Peer Up to refuse.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestLocRIBPeerUpOpenIndicatesTheAddressFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_verdict_test.go#L193) | unit/verify | revert, verified |

### [`RFC9069-6.1.1-1`](#rfc9069-6.1.1-1)

A BMP receiver MUST process these capabilities to know which peer belongs to which address family. (§6.1.1). Proven positive by TestReceiverRecordsThePeerUpAddressFamilies and by TestReceiverKeepsEachPeersAddressFamiliesApart, where two peers of one router advertise different families and each keeps its own, and negative by TestReceiverRecordsNoFamiliesWhenThePeerUpAdvertisesNone: an OPEN advertising no Multiprotocol capability records no family, because a defaulted one would answer the association with an invention

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-14 (independent auditor). TestReceiverRecordsThePeerUpAddressFamilies changed only by calling mustFabricateLocRIBOpen; it still reads back through the receiver the families the Peer Up OPEN advertised. With TestReceiverKeepsEachPeersAddressFamiliesApart (two peers of one router keep their own family) and TestReceiverRecordsNoFamiliesWhenThePeerUpAdvertisesNone (negative), a receiver that ignored, defaulted or merged the capabilities goes red. Separate finding: the discrimination record for the positive at TestReceiverKeepsEachPeersAddressFamiliesApart is producer-changed and needs re-recording.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestReceiverRecordsNoFamiliesWhenThePeerUpAdvertisesNone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L318) | unit/verify | revert, verified |
| positive | [`TestReceiverRecordsThePeerUpAddressFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L285) | unit/verify | revert, verified |
| positive | [`TestReceiverKeepsEachPeersAddressFamiliesApart`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_verdict_test.go#L35) | unit/verify | revert, verified |

### [`RFC9069-6.1.3-1`](#rfc9069-6.1.3-1)

In case of any change that results in the alteration of behavior of an existing BMP session, i.e., changes to filtering and table names, the session MUST be bounced with a Peer Down / Peer Up sequence. (§6.1.3). Ze bounces the peers inside the session and leaves the session up, which is the same answer RFC8671-7.2-1 records for the identical sentence in RFC 8671 §7.2; a behavior change on a session that keeps `loc-rib` on also bounces the Loc-RIB emulated peer: applySenderConfig in sender_config.go. Proven positive by TestBehaviorChangeBouncesThePeersOfALocRIBSession, for a monitored BGP peer, and TestBehaviorChangeBouncesTheLocRIBEmulatedPeer, for the Peer Type 3 Peer Down then Peer Up, and negative by TestAConfigurationThatAltersNoBehaviorBouncesNothing: the bounce is owed to a CHANGE, so a reload that moves no leaf deciding what the session carries bounces nothing

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29. rfc9069_verdict_test.go::TestBehaviorChangeBouncesTheLocRIBEmulatedPeer applies a route-monitoring-policy change through applySenderConfig on a session with the Loc-RIB Peer Up sent, and requires the Peer Type 3 messages to be exactly [Peer Down, Peer Up]; TestBehaviorChangeBouncesThePeersOfALocRIBSession requires the monitored BGP peer's Peer Down then Peer Up; TestAConfigurationThatAltersNoBehaviorBouncesNothing requires no bounce on a reload that moves no behavior leaf. Row prose now names these units. Table names: ze has the one fixed name 'global', so no rename input exists.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestAConfigurationThatAltersNoBehaviorBouncesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L493) | unit/verify | revert, verified |
| positive | [`TestBehaviorChangeBouncesThePeersOfALocRIBSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_test.go#L457) | unit/verify | revert, verified |
| positive | [`TestBehaviorChangeBouncesTheLocRIBEmulatedPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc9069_verdict_test.go#L313) | unit/verify | revert, verified |

## Requirement list review

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
| `1` | Introduction | 0 | reviewed | Introduction. Indicative prose: BMP defines no method to send the Loc-RIB, three use cases for Loc-RIB access, and the statement that this document replaces Section 8.2 of RFC 7854. No sentence directs a speaker. |
| `1.1` | Alternative Method to Monitor Loc-RIB | 0 | reviewed | Alternative Method to Monitor Loc-RIB. Argues why deriving a Loc-RIB from a second router's Adj-RIB-In pre-policy is complex and error prone. Wholly indicative; no directive. |
| `2` | Terminology | 0 | reviewed | Terminology. The BCP 14 key-words paragraph, which binds the key words only when they appear in all capitals. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. |
| `3` | Definitions | 1 | reviewed | Definitions. Five terms: BGP Instance, Adj-RIB-In and Adj-RIB-Out quoted from RFC 4271, Loc-RIB quoted from RFC 4271 Section 1.1, and the pre- and post-policy Adj-RIB-Out pair. The one capitalised keyword sits in the post-policy Adj-RIB-Out definition and is excluded below; RFC 9069 never uses that term again (the only 'Post-Policy' occurrence in the source is this definition) and states no Adj-RIB-Out obligation of its own. |
| `4` | Per-Peer Header | 0 | reviewed | Per-Peer Header. A heading with no body text: 4.1 and 4.2 carry the section. |
| `4.1` | Peer Type | 0 | reviewed | Peer Type. Registers Peer Type 3, Loc-RIB Instance Peer, and contrasts it with the RFC 7854 Section 4.2 Local Instance Peer. Stated indicatively as a value definition, so the site scan sees nothing; ze's PeerTypeLocRIB (internal/component/bgp/plugins/bmp/header.go) is that value, and the obligation to USE it is the first site of section 4.2. |
| `4.2` | Peer Flags | 3 | reviewed | Peer Flags. Redefines the per-peer header flags byte for Peer Type 3: bit 7 is the F flag and bits 0 to 6 are reserved. Three capitalised MUSTs, all three classified below: the peer type locally sourced routes are conveyed with, the F flag under a filter ze does not offer, and the reserved bits. |
| `5` | Loc-RIB Monitoring | 0 | reviewed | Loc-RIB Monitoring. Two indicative paragraphs: what the Loc-RIB contains, per RFC 4271 Section 9.1 and 9.4, and that a subset MAY be sent by setting the F flag. The MAY gates nothing, and it is the sentence that makes Loc-RIB filtering optional, which two exclusions below quote. |
| `5.1` | Per-Peer Header | 1 | reviewed | Per-Peer Header. One capitalised MUST introducing an indented definition list of six field values: Peer Type, Peer Distinguisher, Peer Address, Peer Autonomous System, Peer BGP ID and Timestamp. The site scan sees the introducing sentence only, and the six values are stated indicatively beneath it, so the sentence is the single site and it is mapped below. The three values rfc/short/rfc9069.md renders as rows of their own -- RFC9069-x-5 Peer Address, RFC9069-x-6 Peer AS, RFC9069-x-7 Peer BGP ID -- are read from those indicative lines and are listed unsourced here. |
| `5.2` | Peer Up Notification | 1 | reviewed | Peer Up Notification. Clarifies Section 4.10 of RFC 7854 for the Loc-RIB peer: Local Address zero-filled, Local Port and Remote Port 0, a fabricated sent OPEN whose capabilities carry one capitalised MUST (mapped below), and a received OPEN that repeats it. The zero addresses and ports and the repeated received OPEN are stated indicatively, so the site scan cannot see them; RFC9069-x-3 is read from the Received OPEN sentence and is listed unsourced here. |
| `5.2.1` | Peer Up Information | 11 | reviewed | Peer Up Information. Defines Peer Up Information TLV type 3, VRF/Table Name. Eleven capitalised MUSTs. The RFC states the paragraph twice, once inside the bullet and once outside it, so sites 6 to 10 repeat sites 1 to 5 word for word and are excluded as duplicates of the five originals. Sites 1 to 5 are mapped to RFC9069-5.2.1-1: ze's Loc-RIB IS the default instance with a zero-filled distinguisher, so "The default value of "global" MUST be used" names it directly, and ze sends that TLV on the Peer Up and repeats it on the Peer Down (locRIBTableNameTLV, ensureLocRIBPeerUp, primeLocRIBPeerUp and sendLocRIBPeerDown, internal/component/bgp/plugins/bmp/bmp_locrib.go). Site 11 is the only one the VRF scope decision reaches: it governs MULTIPLE names for one Loc-RIB. |
| `5.3` | Peer Down Notification | 4 | reviewed | Peer Down Notification. Four capitalised MUSTs. The reason-code-6 sentence is mapped to RFC9069-5.3-1. The other three restate the VRF/Table Name TLV's content and size rules and require the TLV in the Peer Down when it was in the Peer Up; all three are mapped to RFC9069-5.2.1-1, whose test reads the Peer Up and the Peer Down of one session off the wire. |
| `5.4` | Route Monitoring | 0 | reviewed | Route Monitoring. Two indicative sentences: Route Monitoring is used for initial synchronization of the Loc-RIB and for incremental changes, and the per-peer header is followed by a BGP Update PDU, quoted from Section 4.6 of RFC 7854. No capitalised keyword, so the site scan sees nothing; RFC9069-x-4, which records that Loc-RIB monitoring starting triggers a full-table dump, is read from the initial-synchronization sentence and is listed unsourced here. |
| `5.4.1` | ASN Encoding | 1 | reviewed | ASN Encoding. One sentence, one capitalised MUST, mapped below to RFC9069-5.4.1-1. |
| `5.4.2` | Granularity | 0 | reviewed | Granularity. State compression and throttling SHOULD be used, with a worked example, and a receiver should expect granularity to vary. A SHOULD and two indicative sentences; none gates. ze applies no state compression: handleBestChange (internal/component/bgp/plugins/bmp/bmp_locrib.go) writes one Route Monitoring per entry of each best-change batch, which is permitted. |
| `5.5` | Route Mirroring | 0 | reviewed | Route Mirroring. States that verbatim duplication is not applicable to the Loc-RIB because the PDUs are originated by the router, and that received Route Mirroring messages SHOULD be ignored. A SHOULD; it never gates. ze's receiver meets it anyway: processRouteMirroring (internal/component/bgp/plugins/bmp/bmp.go) logs the message and acts on nothing. |
| `5.6` | Statistics Report | 0 | reviewed | Statistics Report. Lists the two Stat Types relevant to the Loc-RIB, 8 and 10, as value definitions. No capitalised keyword and no directive: nothing here requires a sender to emit either. ze declares both constants (StatRoutesLocRIB, StatRoutesPerAFILocRIB, internal/component/bgp/plugins/bmp/tlv.go) and emits neither, which this section permits. |
| `6` | Other Considerations | 0 | reviewed | Other Considerations. A heading with no body text. |
| `6.1` | Loc-RIB Implementation | 0 | reviewed | Loc-RIB Implementation. One indicative paragraph: the implementation emulates a peer with Peer Up, Peer Down and Route Monitoring messages. No directive. |
| `6.1.1` | Multiple Loc-RIB Peers | 3 | reviewed | Multiple Loc-RIB Peers. Three capitalised MUSTs, all three mapped below: at least one emulated peer per Loc-RIB instance, a Peer Up per emulated peer carrying the address-family capabilities, and the receiver's duty to process them. The section's other two sentences are indicative and name what identifies a Loc-RIB -- "the peer header distinguisher and BGP ID" -- which is what bmpCompositeKey (internal/component/bgp/plugins/bmp/bmp.go) now keys a Peer Type 3 peer by. |
| `6.1.2` | Filtering Loc-RIB to BMP Receivers | 1 | reviewed | Filtering Loc-RIB to BMP Receivers. Describes the F flag's use case, then one capitalised MUST conditioned on multiple filters against the same Loc-RIB. Excluded below: ze offers no Loc-RIB filter, which section 5 makes optional. |
| `6.1.3` | Changes to Existing BMP Sessions | 1 | reviewed | Changes to Existing BMP Sessions. One sentence, one capitalised MUST: a change that alters the behavior of an existing BMP session bounces it with a Peer Down / Peer Up sequence. Mapped below to RFC9069-6.1.3-1. RFC 8671 Section 7.2 states the same obligation word for word and ze declares that one as RFC8671-7.2-1; this row is RFC 9069's own, because a cross-document exclusion would dismiss a sentence that binds ze here. |
| `7` | Security Considerations | 0 | reviewed | Security Considerations. Imports Section 11 of RFC 7854, states that implementations SHOULD require sessions only with authorized and trusted monitoring devices, and states that this document adds no further consideration. The SHOULD is advisory and never gates. |
| `8` | IANA Considerations | 0 | reviewed | IANA Considerations. Names the BMP Parameters registry the five subsections write to. Walked rather than skipped because 8.3 below carries two derived sites, and a reader who saw the parent skipped would not look for them. |
| `8.1` | Registration of BMP Peer Type 3, Loc-RIB Instance Peer | 0 | skipped (iana) | Registration of BMP Peer Type 3, Loc-RIB Instance Peer. Binds IANA, not a speaker. |
| `8.2` | not stated | 0 | skipped (iana) | Creation of the 'BMP Peer Flags for Loc-RIB Instance Peer Type 3' registry and registration of flag 0, the F flag. Binds IANA, not a speaker. |
| `8.3` | not stated | 2 | reviewed | Registration of Peer Up Information TLV type 3, VRF/Table Name. The registry action binds IANA, but the section restates the TLV's content and size rules word for word and the derivation reads two sites from that restatement. Walked rather than skipped so both are visible; both are excluded below as duplicates of the section 5.2.1 originals. |
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
