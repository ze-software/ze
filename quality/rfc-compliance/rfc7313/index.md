# RFC 7313 - Enhanced Route Refresh Capability for BGP-4

Enhanced Route Refresh. Every requirement listed for RFC 7313, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 4 | of 11 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 2 | of 11 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 11 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 11 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 11 | 11 of 31 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 5 | of 11 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 11 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 11 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 11 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 11 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 11 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 11 | of 17 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 11 mandatory requirements: they add up to 11. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
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
| Public row | Enhanced Route Refresh |
| Automatic checks | Included |
| Requirements | 17 |
| Mandatory requirements (MUST level) | 11 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 5 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 31 |
| Tests linked to requirements | 31 |
| Recorded audit verdicts | 5 |
| Fault test records | 11 |
| Summary | `rfc/short/rfc7313.md` |
| Requirement table | `rfc/requirements/rfc7313.md` |
| RFC text | `rfc/full/rfc7313.txt` |

## Automatic checks

Included in automatic checks: Enhanced Route Refresh Capability for BGP-4: eleven MUST-level requirements. Six are met in internal/component/bgp: 4-3 (the ROUTE-REFRESH Message Subtype selects normal/BoRR/EoRR handling), 5-1 (an invalid-length ROUTE-REFRESH is a NOTIFICATION Route-Refresh Error), and 5-3 (an unknown Message Subtype is ignored and the session stays Established) carry positive+negative tags; 4-1 (send a BoRR before re-advertising), 4-2 (send an EoRR after re-advertising), and 5-2 (the NOTIFICATION data carries the complete ROUTE-REFRESH message) are {single-polarity: positive}. Five are {gap}: 4-4 and 4-5 (a received BoRR/EoRR is log-only, so ze marks no Adj-RIB-In routes stale and purges none), 4-6 and 4-7 (no Graceful-Restart End-of-RIB gate on BoRR emission or acceptance), and 4-13 (ze starts no locally initiated route refresh, and no decision of Thomas's declines one). Disclosed in the docs/features/rfc-status.md RFC 7313 row.

## Published support status

**Area:** Enhanced Route Refresh

**Test verification:** 5 of 11 MUST proven

**Evidence:** unit 5

**Unmet:** RFC7313-4-13, RFC7313-4-4, RFC7313-4-5, RFC7313-4-6, RFC7313-4-7


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 7 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **11** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (4):** [`RFC7313-4-2`](#rfc7313-4-2), [`RFC7313-4-3`](#rfc7313-4-3), [`RFC7313-5-1`](#rfc7313-5-1), [`RFC7313-5-3`](#rfc7313-5-3)

**Annotated (including scoped evidence) (7):** [`RFC7313-4-13`](#rfc7313-4-13), [`RFC7313-4-1`](#rfc7313-4-1), [`RFC7313-4-4`](#rfc7313-4-4), [`RFC7313-4-5`](#rfc7313-4-5), [`RFC7313-5-2`](#rfc7313-5-2), [`RFC7313-4-6`](#rfc7313-4-6), [`RFC7313-4-7`](#rfc7313-4-7)

## Figures

- **MUST bar**: Ze verified 5, Tests awaiting verification 1, Not built 5
- **Overview bar**: Ze MUST verified 5, Tests awaiting verification 1, Not assessed 6, Not built 5

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 11 |
| MUST rows kept (the MUST bar) | 11 |
| Ze verified | 5 |
| OS verified | 0 |
| Tests awaiting verification | 1 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 5 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 6 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 6 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc7313.json](../../../data/rfc/rfc7313.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7313-4-13` | Before the speaker starts a route refresh that is either initiated locally (§4) | MUST | 4 - Operation | **positive:** no positive test. **negative:** no negative test. **{gap}:** no owner decision declines a locally initiated route refresh. RFC 7313 Section 4 says "When a route entry in the "Adj-RIB-Out" changes, only the modified route entry needs to be advertised.", which makes a whole-table refresh unnecessary, not declined. ze starts no such refresh: internal/component/bgp/plugins/rib/rib.go::handleRefresh and internal/component/bgp/plugins/rib/rib_structured.go::handleRefreshStructured are the only producers of a BoRR and of a whole-table re-advertisement, and each runs only on a received ROUTE-REFRESH subtype 0 from the peer; internal/component/bgp/reactor/reactor_api.go::SoftClearPeer, ze's one operator refresh, sends the peer an inbound ROUTE-REFRESH request and re-advertises nothing. The peer-requested clause of the same sentence is RFC7313-4-1 |
| `RFC7313-4-1` | or in response to a "normal route refresh request" from the peer, the speaker MUST send a BoRR message. (§4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestDispatchBGPPeerBoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/route_refresh/handler/rfc7313_dispatch_test.go#L25). **positive:** `unit/verify` [`TestRFC7313BoRRRoutesEoRRInOneSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7313_sequence_test.go#L149). **positive:** `unit/verify` [`TestRFC7313RefreshBracketsReadvertisementWithBoRRAndEoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7313_test.go#L59). **positive:** `unit/verify` [`TestRouteRefreshSubtypes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L139). **negative:** no negative test. **{single-polarity}:** both refresh producers, RIBManager.handleRefresh (internal/component/bgp/plugins/rib/rib.go) and handleRefreshStructured (rib_structured.go), dispatch the BoRR before sendRoutes in one straight-line path with no input that selects a re-advertisement without it, so no refusal path exists to drive a negative; the reading that takes the negative from a peer's re-advertisement lacking the BoRR has nothing to test either, because ze's receipt of BoRR/EoRR is log-only (RFC7313-4-4/4-5 gaps) |
| `RFC7313-4-2` | After the speaker completes the re-advertisement of the entire Adj-RIB-Out to the peer, it MUST send an EoRR message. (§4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestDispatchBGPPeerEoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/route_refresh/handler/rfc7313_dispatch_test.go#L43). **positive:** `unit/verify` [`TestRFC7313BoRRRoutesEoRRInOneSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7313_sequence_test.go#L152). **positive:** `unit/verify` [`TestRFC7313EoRRFollowsTheRoutesItCloses`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_refresh_order_test.go#L79). **positive:** `unit/verify` [`TestRFC7313RefreshBracketsReadvertisementWithBoRRAndEoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7313_test.go#L61). **positive:** `unit/verify` [`TestRouteRefreshSubtypes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L142). **negative:** `unit/verify` [`TestRFC7313EoRRFollowsTheRoutesItCloses`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_refresh_order_test.go#L81) |
| `RFC7313-4-3` | In processing a ROUTE-REFRESH message from a peer, the BGP speaker MUST examine the "message subtype" field of the message and take the appropriate actions. (§4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestHandleRouteRefreshBoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2530). **positive:** `unit/verify` [`TestHandleRouteRefreshEoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2563). **positive:** `unit/verify` [`TestRFC7313SubtypeZeroStartsTheRefresh`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7313_subtype_action_test.go#L61). **negative:** `unit/verify` [`TestHandleRouteRefreshUnknown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2598). **negative:** `unit/verify` [`TestHandleRouteRefresh_UnknownSubtype`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L503). **negative:** `unit/verify` [`TestRFC7313PeerMarkersStartNoRefresh`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7313_subtype_action_test.go#L69) |
| `RFC7313-4-4` | When a BGP speaker receives a BoRR message from a peer, it MUST mark all the routes with the given Address Family Identifier and Subsequent Address Family Identifier, <AFI, SAFI> [RFC2918], from that peer as stale. (§4) | MUST | 4 - Operation | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze logs a received BoRR but does not mark the peer's Adj-RIB-In routes stale -- internal/component/bgp/plugins/rib/rib.go:751-753 handles a received BoRR as log-only and rib_structured.go:506 returns early for a non-zero subtype, so no stale-marking occurs |
| `RFC7313-4-5` | When a BGP speaker receives an EoRR message from a peer, it MUST immediately remove any routes from the peer that are still marked as stale for that <AFI, SAFI>. (§4) | MUST | 4 - Operation | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze logs a received EoRR but performs no stale-route removal -- internal/component/bgp/plugins/rib/rib.go:754-756 handles it as log-only, and because no BoRR stale-marking exists (RFC7313-4-4) there is nothing to purge |
| `RFC7313-5-1` | If the length, excluding the fixed-size message header, of the received ROUTE-REFRESH message with Message Subtype 1 and 2 is not 4, then the BGP speaker MUST send a NOTIFICATION message with the Error Code of "ROUTE-REFRESH Message Error" and the subcode of "Invalid Message Length". (§5) | MUST | 5 - Error Handling | **positive:** `unit/verify` [`TestRouteRefreshBadLengthByMessageSubtype`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L204). **positive:** `unit/verify` [`TestRouteRefreshInvalidLengthNotDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2381). **negative:** `unit/verify` [`TestRouteRefreshBadLengthByMessageSubtype`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L208). **negative:** `unit/verify` [`TestRouteRefreshBadLengthWithoutCapability70`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L117). **negative:** `unit/verify` [`TestRouteRefreshUnknownSubtypeIsIgnoredWhateverItsLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L268). **negative:** `unit/verify` [`TestRouteRefreshValidLengthDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2459). **negative:** `unit/verify` [`TestRouteRefreshWellFormedWithoutCapability70DrawsNoNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L172) |
| `RFC7313-5-2` | The Data field of the NOTIFICATION message MUST contain the complete ROUTE-REFRESH message (§5) | MUST | 5 - Error Handling | **positive:** `unit/verify` [`TestRouteRefreshBadLengthByMessageSubtype`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L206). **positive:** `unit/verify` [`TestRouteRefreshInvalidLengthNotDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2384). **negative:** no negative test. **{single-polarity}:** routeRefreshNotificationData (internal/component/bgp/reactor/session_handlers.go) always copies the entire received body after the header, so every ROUTE-REFRESH NOTIFICATION carries the complete message; there is no truncating code path to drive a negative |
| `RFC7313-5-3` | When the BGP speaker receives a ROUTE-REFRESH message with a "Message Subtype" field other than 0, 1, or 2, it MUST ignore the received ROUTE-REFRESH message. (§5) | MUST | 5 - Error Handling | **positive:** `unit/verify` [`TestHandleRouteRefreshReserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2629). **positive:** `unit/verify` [`TestHandleRouteRefreshUnknown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2596). **positive:** `unit/verify` [`TestRouteRefreshUnknownSubtypeIsIgnoredWhateverItsLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L266). **positive:** `unit/verify` [`TestRouteRefreshUnknownSubtypeNeverDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_reactor_b_test.go#L25). **negative:** `unit/verify` [`TestHandleRouteRefreshBoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2533). **negative:** `unit/verify` [`TestRouteRefreshUnknownSubtypeNeverDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_reactor_b_test.go#L26) |
| `RFC7313-4-6` | For a BGP speaker that supports the BGP Graceful Restart, it MUST NOT send a BoRR for an <AFI, SAFI> to a neighbor before it sends the EoR for the <AFI, SAFI> to the neighbor. (§4) | MUST NOT | 4 - Operation | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's sendRouteRefresh (internal/component/bgp/reactor/reactor_api_forward.go:112) applies no Graceful-Restart End-of-RIB gate, and rib.go:1008-1013 emits the BoRR without checking End-of-RIB state, so the GR/EoR interaction is not enforced |
| `RFC7313-4-7` | A BGP speaker that has received the Graceful Restart Capability from its neighbor MUST ignore any BoRRs for an <AFI, SAFI> from the neighbor before the speaker receives the EoR for the given <AFI, SAFI> from the neighbor. (§4) | MUST | 4 - Operation | **positive:** no positive test. **negative:** no negative test. **{gap}:** the received-BoRR path is log-only (internal/component/bgp/plugins/rib/rib.go:751-753) with no Graceful-Restart End-of-RIB gating, and it depends on the unimplemented stale-marking of RFC7313-4-4 |
| `RFC7313-4-8` | A BGP speaker that supports the message subtypes for the ROUTE- REFRESH message and the related procedures SHOULD advertise the "Enhanced Route Refresh Capability". (§4) | SHOULD | 4 - Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC7313-5-4` | When the BGP speaker receives a ROUTE-REFRESH message with a "Message Subtype" field other than 0, 1, or 2, it MUST ignore the received ROUTE-REFRESH message. It SHOULD log an error for further analysis. (§5) | SHOULD | 5 - Error Handling | **positive:** no positive test. **negative:** no negative test |
| `RFC7313-4-9` | A BGP speaker that has received the Graceful Restart Capability from its neighbor MUST ignore any BoRRs for an <AFI, SAFI> from the neighbor before the speaker receives the EoR for the given <AFI, SAFI> from the neighbor. The BGP speaker SHOULD log an error of the condition for further analysis. (§4) | SHOULD | 4 - Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC7313-4-10` | A BGP speaker MAY ignore any EoRR message received without a prior receipt of an associated BoRR message. (§4) | MAY | 4 - Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC7313-4-11` | When a BGP speaker receives an EoRR message from a peer, it MUST immediately remove any routes from the peer that are still marked as stale for that <AFI, SAFI>. Such purged routes MAY be logged for future analysis. (§4) | MAY | 4 - Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC7313-4-12` | An implementation MAY impose a locally configurable upper bound on how long it would retain any stale routes. (§4) | MAY | 4 - Operation | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7313-4-13`](#rfc7313-4-13) Before the speaker starts a route refresh that is either initiated locally (§4) | {gap}, no test | no owner decision declines a locally initiated route refresh. RFC 7313 Section 4 says "When a route entry in the "Adj-RIB-Out" changes, only the modified route entry needs to be advertised.", which makes a whole-table refresh unnecessary, not declined. ze starts no such refresh: internal/component/bgp/plugins/rib/rib.go::handleRefresh and internal/component/bgp/plugins/rib/rib_structured.go::handleRefreshStructured are the only producers of a BoRR and of a whole-table re-advertisement, and each runs only on a received ROUTE-REFRESH subtype 0 from the peer; internal/component/bgp/reactor/reactor_api.go::SoftClearPeer, ze's one operator refresh, sends the peer an inbound ROUTE-REFRESH request and re-advertises nothing. The peer-requested clause of the same sentence is RFC7313-4-1 |
| [`RFC7313-4-4`](#rfc7313-4-4) When a BGP speaker receives a BoRR message from a peer, it MUST mark all the routes with the given Address Family Identifier and Subsequent Address Family Identifier, <AFI, SAFI> [RFC2918], from that peer as stale. (§4) | {gap}, no test | ze logs a received BoRR but does not mark the peer's Adj-RIB-In routes stale -- internal/component/bgp/plugins/rib/rib.go:751-753 handles a received BoRR as log-only and rib_structured.go:506 returns early for a non-zero subtype, so no stale-marking occurs |
| [`RFC7313-4-5`](#rfc7313-4-5) When a BGP speaker receives an EoRR message from a peer, it MUST immediately remove any routes from the peer that are still marked as stale for that <AFI, SAFI>. (§4) | {gap}, no test | ze logs a received EoRR but performs no stale-route removal -- internal/component/bgp/plugins/rib/rib.go:754-756 handles it as log-only, and because no BoRR stale-marking exists (RFC7313-4-4) there is nothing to purge |
| [`RFC7313-4-6`](#rfc7313-4-6) For a BGP speaker that supports the BGP Graceful Restart, it MUST NOT send a BoRR for an <AFI, SAFI> to a neighbor before it sends the EoR for the <AFI, SAFI> to the neighbor. (§4) | {gap}, no test | ze's sendRouteRefresh (internal/component/bgp/reactor/reactor_api_forward.go:112) applies no Graceful-Restart End-of-RIB gate, and rib.go:1008-1013 emits the BoRR without checking End-of-RIB state, so the GR/EoR interaction is not enforced |
| [`RFC7313-4-7`](#rfc7313-4-7) A BGP speaker that has received the Graceful Restart Capability from its neighbor MUST ignore any BoRRs for an <AFI, SAFI> from the neighbor before the speaker receives the EoR for the given <AFI, SAFI> from the neighbor. (§4) | {gap}, no test | the received-BoRR path is log-only (internal/component/bgp/plugins/rib/rib.go:751-753) with no Graceful-Restart End-of-RIB gating, and it depends on the unimplemented stale-marking of RFC7313-4-4 |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC7313-4-13`](#rfc7313-4-13)

Before the speaker starts a route refresh that is either initiated locally (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7313-4-13, so there is no linked test.

### [`RFC7313-4-1`](#rfc7313-4-1)

or in response to a "normal route refresh request" from the peer, the speaker MUST send a BoRR message. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity positive, marker accepted (no input selects a re-advertisement without the BoRR; BoRR/EoRR receipt is log-only, RFC7313-4-4/4-5 gaps, so no handling-of-absence negative exists). TestRFC7313BoRRRoutesEoRRInOneSequence asserts [BoRR, route, route, EoRR] on both rails (handleRefresh, handleRefreshStructured) in response to a peer normal ROUTE-REFRESH; observed red with handleRefresh broken. The BoRR is written raw at once and the routes after it, so no queue can put a route ahead of it. Supplementary positives: BoRR subtype encoding (routerefresh_test.go), dispatch to SendBoRR, BoRR-first on both rails (rfc7313_test.go); no records for those.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRouteRefreshSubtypes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L139) | unit/verify | not verified |
| positive | [`TestRFC7313BoRRRoutesEoRRInOneSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7313_sequence_test.go#L149) | unit/verify | revert, verified |
| positive | [`TestRFC7313RefreshBracketsReadvertisementWithBoRRAndEoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7313_test.go#L59) | unit/verify | not verified |
| positive | [`TestDispatchBGPPeerBoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/route_refresh/handler/rfc7313_dispatch_test.go#L25) | unit/verify | not verified |

### [`RFC7313-4-2`](#rfc7313-4-2)

After the speaker completes the re-advertisement of the entire Adj-RIB-Out to the peer, it MUST send an EoRR message. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-11-14 by an independent auditor who authored neither 08fe903803 nor the uncommitted phase 2 of spec-single-owner-lifecycle-2-bgp-session. RFC 7313 Section 4 read: "After the speaker completes the re-advertisement of the entire Adj-RIB-Out to the peer, it MUST send an EoRR message." Unit read: TestRFC7313EoRRFollowsTheRoutesItCloses (both tags); only its sync calls changed. 08fe903803 moved the initial-sync gate (sendingInitialRoutes) and the End-of-RIB owed mark (initialSyncEOROwed) from Peer onto Session. In the tagged units named stale it replaced peer.sendInitialRoutes() with peer.runInitialSyncNow(), which is Peer.sendInitialRoutes(p.testInitialSync()) (initial_sync_helpers_test.go): the same production sync body, run on the session the fixture installed (or one built with NewSession(p.settings) and published through establishedSession as enterEstablished does); it replaced peer.<flag>.Store(v) with peer.testInitialSync().<flag>.Store(v) or setTestInitialSync, and drainAndCloseQueueGate now takes that session. git diff 08fe903803^ against the working tree shows no assertion, expected byte, driven input or tag prose changed in any of them, each under the RFC-approved line 08fe903803 carries. Questions 1, 3 and 4 answer exactly as the earlier judgement below; question 2 still rests on the same producers, whose discrimination records are retaken by ./le rfc record after this stamp. Verdict unchanged: enforced. Earlier judgement follows. Re-judged after the D-8 fix: BoRR/EoRR now ride the peer's opQueue as PeerOpRefreshMarker (Peer.sendRefreshMarker, gate read and append under one p.mu hold, same predicate as shouldQueue minus the state check sendRouteRefresh already makes); both drains write the marker in place, count RefreshSent on send, and drop a marker whose session is no longer the peer's. Order at the rib (sendRoutes then EoRR) stays proven by TestRFC7313BoRRRoutesEoRRInOneSequence (observed red). Order on the wire now proven by TestRFC7313EoRRFollowsTheRoutesItCloses through reactorAPIAdapter SendBoRR/AnnounceNLRIBatch/SendEoRR on a recording socket: + outside the sync [eor, borr, route, eorr], - during the sync (inputs forced toward the violation: route held in the queue) nothing leaves early and after the drain [borr, route, eorr, eor]; both counters asserted. Author's red-first log showed [borr eorr route eor] before the fix; records observed on sendQueuedRefreshMarker (+) and sendRefreshMarker (-). {single-polarity} marker correctly removed (R1 route b). Not covered by a test: the stale-session drop branch (a guard, logged).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC7313EoRRFollowsTheRoutesItCloses`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_refresh_order_test.go#L81) | unit/verify | revert, verified |
| positive | [`TestRouteRefreshSubtypes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L142) | unit/verify | not verified |
| positive | [`TestRFC7313BoRRRoutesEoRRInOneSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7313_sequence_test.go#L152) | unit/verify | revert, verified |
| positive | [`TestRFC7313RefreshBracketsReadvertisementWithBoRRAndEoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7313_test.go#L61) | unit/verify | not verified |
| positive | [`TestDispatchBGPPeerEoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/route_refresh/handler/rfc7313_dispatch_test.go#L43) | unit/verify | not verified |
| positive | [`TestRFC7313EoRRFollowsTheRoutesItCloses`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_refresh_order_test.go#L79) | unit/verify | revert, verified |

### [`RFC7313-4-3`](#rfc7313-4-3)

In processing a ROUTE-REFRESH message from a peer, the BGP speaker MUST examine the "message subtype" field of the message and take the appropriate actions. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. The differentiated action is now asserted on the RIB, which takes it. TestRFC7313SubtypeZeroStartsTheRefresh: body 00 01 00 01 on handleRefreshStructured and event kind refresh on dispatch each yield exactly BoRR, the two IPv4 routes of the Adj-RIB-Out, EoRR (requireBracketedReadvertisement, IPv6 route excluded). TestRFC7313PeerMarkersStartNoRefresh: the same up peer and Adj-RIB-Out with subtype 1 or 2 (both rails) emits nothing. Judge break (overlay, rib_structured.go: subtype != 0 widened to subtype > 2) turned the new negative red and the positive green. Revert records on handleRefreshStructured both polarities. The HEAD reactor tags (NoError/Established for every subtype) remain supplementary and unrecorded. The BoRR/EoRR stale handling itself stays the {gap} rows 4-4/4-5. Independent companion-fixture rejudgment against RFC 7313 Sections 3.2, 4 and 5, every one of the six tagged entries and their assertion-producing helpers. The repaired handler fixture is Established with Enhanced Route Refresh negotiated before subtype 3, 100 and 255 inputs; the narrowed tag correctly asserts only the observed NoError behavior. It cannot by itself establish consumer-action suppression. The row's stronger subtype-action pair remains unchanged: a normal subtype 0 yields exactly the requested family's two routes bracketed by BoRR then EoRR, while peer markers 1 and 2 yield no fresh route-refresh sequence, on both DirectBridge and JSON event rails. The existing reactor marker/unknown units remain supplementary. The exact six-entry population, enforced verdict and existing separate stale-route handling gap qualifications remain unchanged; no new runtime or stale-route conformance is inferred from fixture maintenance.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC7313PeerMarkersStartNoRefresh`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7313_subtype_action_test.go#L69) | unit/verify | revert, verified |
| negative | [`TestHandleRouteRefresh_UnknownSubtype`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L503) | unit/verify | mutant, verified |
| negative | [`TestHandleRouteRefreshUnknown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2598) | unit/verify | not verified |
| positive | [`TestRFC7313SubtypeZeroStartsTheRefresh`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7313_subtype_action_test.go#L61) | unit/verify | revert, verified |
| positive | [`TestHandleRouteRefreshBoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2530) | unit/verify | not verified |
| positive | [`TestHandleRouteRefreshEoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2563) | unit/verify | not verified |

### [`RFC7313-4-4`](#rfc7313-4-4)

When a BGP speaker receives a BoRR message from a peer, it MUST mark all the routes with the given Address Family Identifier and Subsequent Address Family Identifier, <AFI, SAFI> [RFC2918], from that peer as stale. (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7313-4-4, so there is no linked test.

### [`RFC7313-4-5`](#rfc7313-4-5)

When a BGP speaker receives an EoRR message from a peer, it MUST immediately remove any routes from the peer that are still marked as stale for that <AFI, SAFI>. (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7313-4-5, so there is no linked test.

### [`RFC7313-5-1`](#rfc7313-5-1)

If the length, excluding the fixed-size message header, of the received ROUTE-REFRESH message with Message Subtype 1 and 2 is not 4, then the BGP speaker MUST send a NOTIFICATION message with the Error Code of "ROUTE-REFRESH Message Error" and the subcode of "Invalid Message Length". (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: not sending Error 7/subcode 1 for a BoRR or EoRR whose body is not 4 octets; (b) TestRouteRefreshBadLengthByMessageSubtype asserts the exact NOTIFICATION for subtypes 1 and 2, too short and too long, and TestRouteRefreshInvalidLengthNotDelivered asserts octets 7/1. Negatives: subtype 0 draws Bad Message Length, a peer without capability 70 draws Bad Message Length, a 4-octet body draws nothing, and an unknown subtype of any length draws no NOTIFICATION and is not delivered (TestRouteRefreshUnknownSubtypeIsIgnoredWhateverItsLength, whose well-formed cases now expect 0 deliveries after the RFC7313-5-3 fix).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRouteRefreshBadLengthByMessageSubtype`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L208) | unit/verify | not verified |
| negative | [`TestRouteRefreshBadLengthWithoutCapability70`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L117) | unit/verify | not verified |
| negative | [`TestRouteRefreshUnknownSubtypeIsIgnoredWhateverItsLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L268) | unit/verify | revert, verified |
| negative | [`TestRouteRefreshWellFormedWithoutCapability70DrawsNoNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L172) | unit/verify | not verified |
| negative | [`TestRouteRefreshValidLengthDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2459) | unit/verify | not verified |
| positive | [`TestRouteRefreshBadLengthByMessageSubtype`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L204) | unit/verify | not verified |
| positive | [`TestRouteRefreshInvalidLengthNotDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2381) | unit/verify | not verified |

### [`RFC7313-5-2`](#rfc7313-5-2)

The Data field of the NOTIFICATION message MUST contain the complete ROUTE-REFRESH message (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRouteRefreshBadLengthByMessageSubtype`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L206) | unit/verify | not verified |
| positive | [`TestRouteRefreshInvalidLengthNotDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2384) | unit/verify | not verified |

### [`RFC7313-5-3`](#rfc7313-5-3)

When the BGP speaker receives a ROUTE-REFRESH message with a "Message Subtype" field other than 0, 1, or 2, it MUST ignore the received ROUTE-REFRESH message. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: acting on a ROUTE-REFRESH whose subtype is not 0, 1 or 2. Defect fixed at the producer: screenRouteRefresh now calls routeRefreshSubtypeUnknown (peer sent capability 70), so the read path drops the message before onMessageReceived. TestRouteRefreshUnknownSubtypeNeverDelivered asserts 0 onMessageReceived and 0 onRefreshRecv, no NOTIFICATION and Established for 4-octet subtypes 3/5/42/255; negative: subtypes 0/1/2 through the same exchange are delivered once each. TestRouteRefreshUnknownSubtypeIsIgnoredWhateverItsLength extends this to wrong-length unknown subtypes (0 deliveries, no NOTIFICATION). The session_test.go units prove only no teardown.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRouteRefreshUnknownSubtypeNeverDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_reactor_b_test.go#L26) | unit/verify | revert, verified |
| negative | [`TestHandleRouteRefreshBoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2533) | unit/verify | not verified |
| positive | [`TestRouteRefreshUnknownSubtypeIsIgnoredWhateverItsLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L266) | unit/verify | revert, verified |
| positive | [`TestRouteRefreshUnknownSubtypeNeverDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_reactor_b_test.go#L25) | unit/verify | revert, verified |
| positive | [`TestHandleRouteRefreshReserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2629) | unit/verify | not verified |
| positive | [`TestHandleRouteRefreshUnknown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2596) | unit/verify | not verified |

### [`RFC7313-4-6`](#rfc7313-4-6)

For a BGP speaker that supports the BGP Graceful Restart, it MUST NOT send a BoRR for an <AFI, SAFI> to a neighbor before it sends the EoR for the <AFI, SAFI> to the neighbor. (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7313-4-6, so there is no linked test.

### [`RFC7313-4-7`](#rfc7313-4-7)

A BGP speaker that has received the Graceful Restart Capability from its neighbor MUST ignore any BoRRs for an <AFI, SAFI> from the neighbor before the speaker receives the EoR for the given <AFI, SAFI> from the neighbor. (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7313-4-7, so there is no linked test.

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6 phase 5, rfc7313 |
| Signed off | 2026-08-31 |
| Register | rfc2119 |
| Source | rfc/full/rfc7313.txt |
| Source fingerprint | 963962f4ec722372 |
| Record | rfc/extraction/rfc7313.json |
| Mapped sentences | 10 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of This Memo, Copyright Notice, Abstract and Table of Contents. The Abstract restates section 1 and states no obligation. |
| `1` | Introduction | 0 | reviewed | Introduction. Indicative prose: why consistency validation is needed, what the document enhances, and that it updates RFC 2918 by redefining the ROUTE-REFRESH Reserved field. No sentence directs a speaker. |
| `2` | Requirements Language | 0 | reviewed | Requirements Language. The RFC 2119 key-words paragraph, which also states that the key words bind only when they appear in all upper case. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. |
| `3` | Protocol Extensions | 0 | reviewed | Protocol Extensions. One sentence naming what sections 3.1 and 3.2 define: the Enhanced Route Refresh Capability and the ROUTE-REFRESH message subtypes. No directive. |
| `3.1` | Enhanced Route Refresh Capability | 0 | reviewed | Enhanced Route Refresh Capability. Value assignment: Capability Code 70, Capability Length zero, and what advertising it conveys. Stated indicatively, so under this document's own section 2 it carries no RFC 2119 level. The obligation to advertise it is the SHOULD in section 4 (RFC7313-4-8); the wire values are carried by the Wire Formats and Constants tables of rfc/short/rfc7313.md. |
| `3.2` | Subtypes for ROUTE-REFRESH Message | 0 | reviewed | Subtypes for ROUTE-REFRESH Message. Value assignment for subtypes 0, 1, 2 and 255, with the remaining values reserved for future use. A registry table, not a directive: the obligation to USE and to check each value is in sections 4 and 5. |
| `4` | Operation | 7 | reviewed | Operation. The document's main normative section: seven capitalised MUST-level sites, all mapped below to RFC7313-4-1 through RFC7313-4-7. Its remaining directives are the SHOULD to advertise the capability, the SHOULD to log an ignored early BoRR, and three MAYs (log purged routes, ignore an unsolicited EoRR, bound stale-route retention); those are captured as the unsourced ids below. Two sentences the site scan cannot see are scoping, not obligations: "The following procedures are applicable only if a BGP speaker has received the 'Enhanced Route Refresh Capability' from a peer" conditions RFC7313-4-1 through RFC7313-4-7 rather than adding a requirement, and the "entire Adj-RIB-Out" paragraph defines a term used by RFC7313-4-2. Both are indicative, so section 2 gives them no normative meaning. |
| `5` | Error Handling | 3 | reviewed | Error Handling. Assigns NOTIFICATION Error Code 7 ("ROUTE-REFRESH Message Error") and subcode 1 ("Invalid Message Length") as value definitions, then states three capitalised MUSTs, mapped below to RFC7313-5-1 through RFC7313-5-3. Its one SHOULD (log an error on an unknown subtype) is the unsourced id below. As in section 4, the applicability sentence conditions the three MUSTs on having received the capability from the peer and is indicative. |
| `6` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records Capability Code 70, the new "BGP Route Refresh Subcodes" registry, the rename of "BGP Error Codes", error code 7 and the "BGP ROUTE-REFRESH Message Error subcodes" registry. Binds IANA, not a speaker. |
| `7` | Security Considerations | 0 | reviewed | Security Considerations. States that RFC 4272 does not cover Route-Refresh and that this document does not significantly change the underlying security issues. No countermeasure is directed at a speaker. |
| `8` | Acknowledgements | 0 | skipped (acknowledgements) | Acknowledgements. |
| `9` | not stated | 0 | skipped (references) | Normative References: RFC 2119, RFC 2918, RFC 4271, RFC 4272, RFC 4724, RFC 5291, RFC 5492. |

### Excluded sentences

The review of RFC 7313 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 7313, so its obligations are stated where they were written.
