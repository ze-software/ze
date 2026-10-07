# RFC 7313 - Enhanced Route Refresh Capability for BGP-4

Supported. Every requirement this repository extracted from RFC 7313, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 36.4% | 4 of 11 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 18.2% | 2 of 11 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 11 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 11 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 29.0% | 9 of 31 tagged units, 0 escaped and 1 lapsed | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 11 | of 17 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 11 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 11 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 11 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 9.1% | 1 of 11 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 36.4% | 4 of 11 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 11 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 17 |
| Gated MUST-level | 11 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 4 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 31 |
| Tagged units | 31 |
| Recorded audit verdicts | 5 |
| Discrimination records | 10 |
| Summary | `rfc/short/rfc7313.md` |
| Requirement shard | `rfc/requirements/rfc7313.md` |
| RFC text | `rfc/full/rfc7313.txt` |

## Enrolment

Enrolled: Enhanced Route Refresh Capability for BGP-4: ten MUST-level requirements. Six are met in internal/component/bgp: 4-3 (the ROUTE-REFRESH Message Subtype selects normal/BoRR/EoRR handling), 5-1 (an invalid-length ROUTE-REFRESH is a NOTIFICATION Route-Refresh Error), and 5-3 (an unknown Message Subtype is ignored and the session stays Established) carry positive+negative tags; 4-1 (send a BoRR before re-advertising), 4-2 (send an EoRR after re-advertising), and 5-2 (the NOTIFICATION data carries the complete ROUTE-REFRESH message) are {single-polarity: positive}. Four are {gap}: 4-4 and 4-5 (a received BoRR/EoRR is log-only, so ze marks no Adj-RIB-In routes stale and purges none) and 4-6 and 4-7 (no Graceful-Restart End-of-RIB gate on BoRR emission or acceptance). Disclosed in the docs/features/rfc-status.md RFC 7313 row.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered:**

BoRR and EoRR support, capability checks, bounded route resend.

**What the ledger says remains**

Four MUST-level receive-side gaps annotated in [`rfc/short/rfc7313.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc7313.md): [`RFC7313-4-4`](#rfc7313-4-4)/4-5 -- a received BoRR/EoRR is log-only ([`internal/component/bgp/plugins/rib/rib.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib.go)), so ze marks no Adj-RIB-In routes stale and purges none; and [`RFC7313-4-6`](#rfc7313-4-6)/4-7 -- neither the send nor receive path applies a Graceful-Restart End-of-RIB gate to BoRR emission or acceptance. One feature is absent by decision, and it is not a conformance gap: ze starts no locally initiated route refresh (soft clear only asks the peer for its refresh), so the BoRR obligation conditional on one, [`RFC7313-4-13`](#rfc7313-4-13), is annotated {feature-declined}.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated (including scoped evidence) | 7 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **11** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC7313-4-2`](#rfc7313-4-2), [`RFC7313-4-3`](#rfc7313-4-3), [`RFC7313-5-1`](#rfc7313-5-1), [`RFC7313-5-3`](#rfc7313-5-3)

**Annotated (including scoped evidence) (7):** [`RFC7313-4-13`](#rfc7313-4-13), [`RFC7313-4-1`](#rfc7313-4-1), [`RFC7313-4-4`](#rfc7313-4-4), [`RFC7313-4-5`](#rfc7313-4-5), [`RFC7313-5-2`](#rfc7313-5-2), [`RFC7313-4-6`](#rfc7313-4-6), [`RFC7313-4-7`](#rfc7313-4-7)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7313-4-13` | Before the speaker starts a route refresh that is either initiated locally (§4) | MUST | 4 - Operation | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "changes, only the modified route entry needs to be advertised"; a locally initiated refresh, re-advertising the entire Adj-RIB-Out on the speaker's own initiative, is a feature RFC 7313 never requires, because an Adj-RIB-Out change needs only the modified routes. ze starts no such refresh: internal/component/bgp/plugins/rib/rib.go::handleRefresh and internal/component/bgp/plugins/rib/rib_structured.go::handleRefreshStructured are the only producers of a BoRR and of a whole-table re-advertisement, and each runs only on a received ROUTE-REFRESH subtype 0 from the peer; internal/component/bgp/reactor/reactor_api.go::SoftClearPeer, ze's one operator refresh, sends the peer an inbound ROUTE-REFRESH request and re-advertises nothing. The peer-requested clause of the same sentence is RFC7313-4-1 |
| `RFC7313-4-1` | or in response to a "normal route refresh request" from the peer, the speaker MUST send a BoRR message. (§4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestDispatchBGPPeerBoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/route_refresh/handler/rfc7313_dispatch_test.go#L25). **positive:** `unit/verify` [`TestRFC7313BoRRRoutesEoRRInOneSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7313_sequence_test.go#L149). **positive:** `unit/verify` [`TestRFC7313RefreshBracketsReadvertisementWithBoRRAndEoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7313_test.go#L59). **positive:** `unit/verify` [`TestRouteRefreshSubtypes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L139). **negative:** no negative test. **{single-polarity}:** both refresh producers, RIBManager.handleRefresh (internal/component/bgp/plugins/rib/rib.go) and handleRefreshStructured (rib_structured.go), dispatch the BoRR before sendRoutes in one straight-line path with no input that selects a re-advertisement without it, so no refusal path exists to drive a negative; the reading that takes the negative from a peer's re-advertisement lacking the BoRR has nothing to test either, because ze's receipt of BoRR/EoRR is log-only (RFC7313-4-4/4-5 gaps) |
| `RFC7313-4-2` | After the speaker completes the re-advertisement of the entire Adj-RIB-Out to the peer, it MUST send an EoRR message. (§4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestDispatchBGPPeerEoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/route_refresh/handler/rfc7313_dispatch_test.go#L43). **positive:** `unit/verify` [`TestRFC7313BoRRRoutesEoRRInOneSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7313_sequence_test.go#L152). **positive:** `unit/verify` [`TestRFC7313EoRRFollowsTheRoutesItCloses`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_refresh_order_test.go#L79). **positive:** `unit/verify` [`TestRFC7313RefreshBracketsReadvertisementWithBoRRAndEoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7313_test.go#L61). **positive:** `unit/verify` [`TestRouteRefreshSubtypes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L142). **negative:** `unit/verify` [`TestRFC7313EoRRFollowsTheRoutesItCloses`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_refresh_order_test.go#L81) |
| `RFC7313-4-3` | In processing a ROUTE-REFRESH message from a peer, the BGP speaker MUST examine the "message subtype" field of the message and take the appropriate actions. (§4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestHandleRouteRefreshBoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2529). **positive:** `unit/verify` [`TestHandleRouteRefreshEoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2562). **positive:** `unit/verify` [`TestRFC7313SubtypeZeroStartsTheRefresh`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7313_subtype_action_test.go#L61). **negative:** `unit/verify` [`TestHandleRouteRefreshUnknown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2597). **negative:** `unit/verify` [`TestHandleRouteRefresh_UnknownSubtype`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L487). **negative:** `unit/verify` [`TestRFC7313PeerMarkersStartNoRefresh`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7313_subtype_action_test.go#L69) |
| `RFC7313-4-4` | When a BGP speaker receives a BoRR message from a peer, it MUST mark all the routes with the given Address Family Identifier and Subsequent Address Family Identifier, <AFI, SAFI> [RFC2918], from that peer as stale. (§4) | MUST | 4 - Operation | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze logs a received BoRR but does not mark the peer's Adj-RIB-In routes stale -- internal/component/bgp/plugins/rib/rib.go:751-753 handles a received BoRR as log-only and rib_structured.go:506 returns early for a non-zero subtype, so no stale-marking occurs |
| `RFC7313-4-5` | When a BGP speaker receives an EoRR message from a peer, it MUST immediately remove any routes from the peer that are still marked as stale for that <AFI, SAFI>. (§4) | MUST | 4 - Operation | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze logs a received EoRR but performs no stale-route removal -- internal/component/bgp/plugins/rib/rib.go:754-756 handles it as log-only, and because no BoRR stale-marking exists (RFC7313-4-4) there is nothing to purge |
| `RFC7313-5-1` | If the length, excluding the fixed-size message header, of the received ROUTE-REFRESH message with Message Subtype 1 and 2 is not 4, then the BGP speaker MUST send a NOTIFICATION message with the Error Code of "ROUTE-REFRESH Message Error" and the subcode of "Invalid Message Length". (§5) | MUST | 5 - Error Handling | **positive:** `unit/verify` [`TestRouteRefreshBadLengthByMessageSubtype`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L204). **positive:** `unit/verify` [`TestRouteRefreshInvalidLengthNotDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2380). **negative:** `unit/verify` [`TestRouteRefreshBadLengthByMessageSubtype`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L208). **negative:** `unit/verify` [`TestRouteRefreshBadLengthWithoutCapability70`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L117). **negative:** `unit/verify` [`TestRouteRefreshUnknownSubtypeIsIgnoredWhateverItsLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L268). **negative:** `unit/verify` [`TestRouteRefreshValidLengthDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2458). **negative:** `unit/verify` [`TestRouteRefreshWellFormedWithoutCapability70DrawsNoNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L172) |
| `RFC7313-5-2` | The Data field of the NOTIFICATION message MUST contain the complete ROUTE-REFRESH message (§5) | MUST | 5 - Error Handling | **positive:** `unit/verify` [`TestRouteRefreshBadLengthByMessageSubtype`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L206). **positive:** `unit/verify` [`TestRouteRefreshInvalidLengthNotDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2383). **negative:** no negative test. **{single-polarity}:** routeRefreshNotificationData (internal/component/bgp/reactor/session_handlers.go) always copies the entire received body after the header, so every ROUTE-REFRESH NOTIFICATION carries the complete message; there is no truncating code path to drive a negative |
| `RFC7313-5-3` | When the BGP speaker receives a ROUTE-REFRESH message with a "Message Subtype" field other than 0, 1, or 2, it MUST ignore the received ROUTE-REFRESH message. (§5) | MUST | 5 - Error Handling | **positive:** `unit/verify` [`TestHandleRouteRefreshReserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2628). **positive:** `unit/verify` [`TestHandleRouteRefreshUnknown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2595). **positive:** `unit/verify` [`TestRouteRefreshUnknownSubtypeIsIgnoredWhateverItsLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L266). **positive:** `unit/verify` [`TestRouteRefreshUnknownSubtypeNeverDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_reactor_b_test.go#L25). **negative:** `unit/verify` [`TestHandleRouteRefreshBoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2532). **negative:** `unit/verify` [`TestRouteRefreshUnknownSubtypeNeverDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_reactor_b_test.go#L26) |
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
| [`RFC7313-4-13`](#rfc7313-4-13) Before the speaker starts a route refresh that is either initiated locally (§4) | no test | no test carries this requirement id; annotated {feature-declined}: "changes, only the modified route entry needs to be advertised"; a locally initiated refresh, re-advertising the entire Adj-RIB-Out on the speaker's own initiative, is a feature RFC 7313 never requires, because an Adj-RIB-Out change needs only the modified routes. ze starts no such refresh: internal/component/bgp/plugins/rib/rib.go::handleRefresh and internal/component/bgp/plugins/rib/rib_structured.go::handleRefreshStructured are the only producers of a BoRR and of a whole-table re-advertisement, and each runs only on a received ROUTE-REFRESH subtype 0 from the peer; internal/component/bgp/reactor/reactor_api.go::SoftClearPeer, ze's one operator refresh, sends the peer an inbound ROUTE-REFRESH request and re-advertises nothing. The peer-requested clause of the same sentence is RFC7313-4-1 |
| [`RFC7313-4-4`](#rfc7313-4-4) When a BGP speaker receives a BoRR message from a peer, it MUST mark all the routes with the given Address Family Identifier and Subsequent Address Family Identifier, <AFI, SAFI> [RFC2918], from that peer as stale. (§4) | {gap}, no test | ze logs a received BoRR but does not mark the peer's Adj-RIB-In routes stale -- internal/component/bgp/plugins/rib/rib.go:751-753 handles a received BoRR as log-only and rib_structured.go:506 returns early for a non-zero subtype, so no stale-marking occurs |
| [`RFC7313-4-5`](#rfc7313-4-5) When a BGP speaker receives an EoRR message from a peer, it MUST immediately remove any routes from the peer that are still marked as stale for that <AFI, SAFI>. (§4) | {gap}, no test | ze logs a received EoRR but performs no stale-route removal -- internal/component/bgp/plugins/rib/rib.go:754-756 handles it as log-only, and because no BoRR stale-marking exists (RFC7313-4-4) there is nothing to purge |
| [`RFC7313-4-6`](#rfc7313-4-6) For a BGP speaker that supports the BGP Graceful Restart, it MUST NOT send a BoRR for an <AFI, SAFI> to a neighbor before it sends the EoR for the <AFI, SAFI> to the neighbor. (§4) | {gap}, no test | ze's sendRouteRefresh (internal/component/bgp/reactor/reactor_api_forward.go:112) applies no Graceful-Restart End-of-RIB gate, and rib.go:1008-1013 emits the BoRR without checking End-of-RIB state, so the GR/EoR interaction is not enforced |
| [`RFC7313-4-7`](#rfc7313-4-7) A BGP speaker that has received the Graceful Restart Capability from its neighbor MUST ignore any BoRRs for an <AFI, SAFI> from the neighbor before the speaker receives the EoR for the given <AFI, SAFI> from the neighbor. (§4) | {gap}, no test | the received-BoRR path is log-only (internal/component/bgp/plugins/rib/rib.go:751-753) with no Graceful-Restart End-of-RIB gating, and it depends on the unimplemented stale-marking of RFC7313-4-4 |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7313-4-13`](#rfc7313-4-13)

Before the speaker starts a route refresh that is either initiated locally (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7313-4-13, so no unit is bound to it.

### [`RFC7313-4-1`](#rfc7313-4-1)

or in response to a "normal route refresh request" from the peer, the speaker MUST send a BoRR message. (§4)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/plugins/rib/rfc7313_sequence_test.go::TestRFC7313BoRRRoutesEoRRInOneSequence moved. Single-polarity positive, marker accepted (no input selects a re-advertisement without the BoRR; BoRR/EoRR receipt is log-only, RFC7313-4-4/4-5 gaps, so no handling-of-absence negative exists). TestRFC7313BoRRRoutesEoRRInOneSequence asserts [BoRR, route, route, EoRR] on both rails (handleRefresh, handleRefreshStructured) in response to a peer normal ROUTE-REFRESH; observed red with handleRefresh broken. The BoRR is written raw at once and the routes after it, so no queue can put a route ahead of it. Supplementary positives: BoRR subtype encoding (routerefresh_test.go), dispatch to SendBoRR, BoRR-first on both rails (rfc7313_test.go); no records for those.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRouteRefreshSubtypes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L139) | unit/verify | unproven |
| positive | [`TestRFC7313BoRRRoutesEoRRInOneSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7313_sequence_test.go#L149) | unit/verify | revert, verified |
| positive | [`TestRFC7313RefreshBracketsReadvertisementWithBoRRAndEoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7313_test.go#L59) | unit/verify | unproven |
| positive | [`TestDispatchBGPPeerBoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/route_refresh/handler/rfc7313_dispatch_test.go#L25) | unit/verify | unproven |

### [`RFC7313-4-2`](#rfc7313-4-2)

After the speaker completes the re-advertisement of the entire Adj-RIB-Out to the peer, it MUST send an EoRR message. (§4)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/plugins/rib/rfc7313_sequence_test.go::TestRFC7313BoRRRoutesEoRRInOneSequence moved. Re-judged after the D-8 fix: BoRR/EoRR now ride the peer's opQueue as PeerOpRefreshMarker (Peer.sendRefreshMarker, gate read and append under one p.mu hold, same predicate as shouldQueue minus the state check sendRouteRefresh already makes); both drains write the marker in place, count RefreshSent on send, and drop a marker whose session is no longer the peer's. Order at the rib (sendRoutes then EoRR) stays proven by TestRFC7313BoRRRoutesEoRRInOneSequence (observed red). Order on the wire now proven by TestRFC7313EoRRFollowsTheRoutesItCloses through reactorAPIAdapter SendBoRR/AnnounceNLRIBatch/SendEoRR on a recording socket: + outside the sync [eor, borr, route, eorr], - during the sync (inputs forced toward the violation: route held in the queue) nothing leaves early and after the drain [borr, route, eorr, eor]; both counters asserted. Author's red-first log showed [borr eorr route eor] before the fix; records observed on sendQueuedRefreshMarker (+) and sendRefreshMarker (-). {single-polarity} marker correctly removed (R1 route b). Not covered by a test: the stale-session drop branch (a guard, logged).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7313EoRRFollowsTheRoutesItCloses`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_refresh_order_test.go#L81) | unit/verify | revert, verified |
| positive | [`TestRouteRefreshSubtypes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L142) | unit/verify | unproven |
| positive | [`TestRFC7313BoRRRoutesEoRRInOneSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7313_sequence_test.go#L152) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC7313RefreshBracketsReadvertisementWithBoRRAndEoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7313_test.go#L61) | unit/verify | unproven |
| positive | [`TestDispatchBGPPeerEoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/route_refresh/handler/rfc7313_dispatch_test.go#L43) | unit/verify | unproven |
| positive | [`TestRFC7313EoRRFollowsTheRoutesItCloses`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_refresh_order_test.go#L79) | unit/verify | revert, verified |

### [`RFC7313-4-3`](#rfc7313-4-3)

In processing a ROUTE-REFRESH message from a peer, the BGP speaker MUST examine the "message subtype" field of the message and take the appropriate actions. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. The differentiated action is now asserted on the RIB, which takes it. TestRFC7313SubtypeZeroStartsTheRefresh: body 00 01 00 01 on handleRefreshStructured and event kind refresh on dispatch each yield exactly BoRR, the two IPv4 routes of the Adj-RIB-Out, EoRR (requireBracketedReadvertisement, IPv6 route excluded). TestRFC7313PeerMarkersStartNoRefresh: the same up peer and Adj-RIB-Out with subtype 1 or 2 (both rails) emits nothing. Judge break (overlay, rib_structured.go: subtype != 0 widened to subtype > 2) turned the new negative red and the positive green. Revert records on handleRefreshStructured both polarities. The HEAD reactor tags (NoError/Established for every subtype) remain supplementary and unrecorded. The BoRR/EoRR stale handling itself stays the {gap} rows 4-4/4-5.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7313PeerMarkersStartNoRefresh`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7313_subtype_action_test.go#L69) | unit/verify | revert, verified |
| negative | [`TestHandleRouteRefresh_UnknownSubtype`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L487) | unit/verify | unproven |
| negative | [`TestHandleRouteRefreshUnknown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2597) | unit/verify | unproven |
| positive | [`TestRFC7313SubtypeZeroStartsTheRefresh`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7313_subtype_action_test.go#L61) | unit/verify | revert, verified |
| positive | [`TestHandleRouteRefreshBoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2529) | unit/verify | unproven |
| positive | [`TestHandleRouteRefreshEoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2562) | unit/verify | unproven |

### [`RFC7313-4-4`](#rfc7313-4-4)

When a BGP speaker receives a BoRR message from a peer, it MUST mark all the routes with the given Address Family Identifier and Subsequent Address Family Identifier, <AFI, SAFI> [RFC2918], from that peer as stale. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7313-4-4, so no unit is bound to it.

### [`RFC7313-4-5`](#rfc7313-4-5)

When a BGP speaker receives an EoRR message from a peer, it MUST immediately remove any routes from the peer that are still marked as stale for that <AFI, SAFI>. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7313-4-5, so no unit is bound to it.

### [`RFC7313-5-1`](#rfc7313-5-1)

If the length, excluding the fixed-size message header, of the received ROUTE-REFRESH message with Message Subtype 1 and 2 is not 4, then the BGP speaker MUST send a NOTIFICATION message with the Error Code of "ROUTE-REFRESH Message Error" and the subcode of "Invalid Message Length". (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: not sending Error 7/subcode 1 for a BoRR or EoRR whose body is not 4 octets; (b) TestRouteRefreshBadLengthByMessageSubtype asserts the exact NOTIFICATION for subtypes 1 and 2, too short and too long, and TestRouteRefreshInvalidLengthNotDelivered asserts octets 7/1. Negatives: subtype 0 draws Bad Message Length, a peer without capability 70 draws Bad Message Length, a 4-octet body draws nothing, and an unknown subtype of any length draws no NOTIFICATION and is not delivered (TestRouteRefreshUnknownSubtypeIsIgnoredWhateverItsLength, whose well-formed cases now expect 0 deliveries after the RFC7313-5-3 fix).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRouteRefreshBadLengthByMessageSubtype`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L208) | unit/verify | unproven |
| negative | [`TestRouteRefreshBadLengthWithoutCapability70`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L117) | unit/verify | unproven |
| negative | [`TestRouteRefreshUnknownSubtypeIsIgnoredWhateverItsLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L268) | unit/verify | revert, verified |
| negative | [`TestRouteRefreshWellFormedWithoutCapability70DrawsNoNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L172) | unit/verify | unproven |
| negative | [`TestRouteRefreshValidLengthDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2458) | unit/verify | unproven |
| positive | [`TestRouteRefreshBadLengthByMessageSubtype`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L204) | unit/verify | unproven |
| positive | [`TestRouteRefreshInvalidLengthNotDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2380) | unit/verify | unproven |

### [`RFC7313-5-2`](#rfc7313-5-2)

The Data field of the NOTIFICATION message MUST contain the complete ROUTE-REFRESH message (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRouteRefreshBadLengthByMessageSubtype`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L206) | unit/verify | unproven |
| positive | [`TestRouteRefreshInvalidLengthNotDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2383) | unit/verify | unproven |

### [`RFC7313-5-3`](#rfc7313-5-3)

When the BGP speaker receives a ROUTE-REFRESH message with a "Message Subtype" field other than 0, 1, or 2, it MUST ignore the received ROUTE-REFRESH message. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: acting on a ROUTE-REFRESH whose subtype is not 0, 1 or 2. Defect fixed at the producer: screenRouteRefresh now calls routeRefreshSubtypeUnknown (peer sent capability 70), so the read path drops the message before onMessageReceived. TestRouteRefreshUnknownSubtypeNeverDelivered asserts 0 onMessageReceived and 0 onRefreshRecv, no NOTIFICATION and Established for 4-octet subtypes 3/5/42/255; negative: subtypes 0/1/2 through the same exchange are delivered once each. TestRouteRefreshUnknownSubtypeIsIgnoredWhateverItsLength extends this to wrong-length unknown subtypes (0 deliveries, no NOTIFICATION). The session_test.go units prove only no teardown.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRouteRefreshUnknownSubtypeNeverDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_reactor_b_test.go#L26) | unit/verify | revert, verified |
| negative | [`TestHandleRouteRefreshBoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2532) | unit/verify | unproven |
| positive | [`TestRouteRefreshUnknownSubtypeIsIgnoredWhateverItsLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L266) | unit/verify | revert, verified |
| positive | [`TestRouteRefreshUnknownSubtypeNeverDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_reactor_b_test.go#L25) | unit/verify | revert, verified |
| positive | [`TestHandleRouteRefreshReserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2628) | unit/verify | unproven |
| positive | [`TestHandleRouteRefreshUnknown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2595) | unit/verify | unproven |

### [`RFC7313-4-6`](#rfc7313-4-6)

For a BGP speaker that supports the BGP Graceful Restart, it MUST NOT send a BoRR for an <AFI, SAFI> to a neighbor before it sends the EoR for the <AFI, SAFI> to the neighbor. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7313-4-6, so no unit is bound to it.

### [`RFC7313-4-7`](#rfc7313-4-7)

A BGP speaker that has received the Graceful Restart Capability from its neighbor MUST ignore any BoRRs for an <AFI, SAFI> from the neighbor before the speaker receives the EoR for the given <AFI, SAFI> from the neighbor. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7313-4-7, so no unit is bound to it.

## Extraction sign-off

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
| `1` | Introduction | 0 | walked | Introduction. Indicative prose: why consistency validation is needed, what the document enhances, and that it updates RFC 2918 by redefining the ROUTE-REFRESH Reserved field. No sentence directs a speaker. |
| `2` | Requirements Language | 0 | walked | Requirements Language. The RFC 2119 key-words paragraph, which also states that the key words bind only when they appear in all upper case. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. |
| `3` | Protocol Extensions | 0 | walked | Protocol Extensions. One sentence naming what sections 3.1 and 3.2 define: the Enhanced Route Refresh Capability and the ROUTE-REFRESH message subtypes. No directive. |
| `3.1` | Enhanced Route Refresh Capability | 0 | walked | Enhanced Route Refresh Capability. Value assignment: Capability Code 70, Capability Length zero, and what advertising it conveys. Stated indicatively, so under this document's own section 2 it carries no RFC 2119 level. The obligation to advertise it is the SHOULD in section 4 (RFC7313-4-8); the wire values are carried by the Wire Formats and Constants tables of rfc/short/rfc7313.md. |
| `3.2` | Subtypes for ROUTE-REFRESH Message | 0 | walked | Subtypes for ROUTE-REFRESH Message. Value assignment for subtypes 0, 1, 2 and 255, with the remaining values reserved for future use. A registry table, not a directive: the obligation to USE and to check each value is in sections 4 and 5. |
| `4` | Operation | 7 | walked | Operation. The document's main normative section: seven capitalised MUST-level sites, all mapped below to RFC7313-4-1 through RFC7313-4-7. Its remaining directives are the SHOULD to advertise the capability, the SHOULD to log an ignored early BoRR, and three MAYs (log purged routes, ignore an unsolicited EoRR, bound stale-route retention); those are captured as the unsourced ids below. Two sentences the site scan cannot see are scoping, not obligations: "The following procedures are applicable only if a BGP speaker has received the 'Enhanced Route Refresh Capability' from a peer" conditions RFC7313-4-1 through RFC7313-4-7 rather than adding a requirement, and the "entire Adj-RIB-Out" paragraph defines a term used by RFC7313-4-2. Both are indicative, so section 2 gives them no normative meaning. |
| `5` | Error Handling | 3 | walked | Error Handling. Assigns NOTIFICATION Error Code 7 ("ROUTE-REFRESH Message Error") and subcode 1 ("Invalid Message Length") as value definitions, then states three capitalised MUSTs, mapped below to RFC7313-5-1 through RFC7313-5-3. Its one SHOULD (log an error on an unknown subtype) is the unsourced id below. As in section 4, the applicability sentence conditions the three MUSTs on having received the capability from the peer and is indicative. |
| `6` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records Capability Code 70, the new "BGP Route Refresh Subcodes" registry, the rename of "BGP Error Codes", error code 7 and the "BGP ROUTE-REFRESH Message Error subcodes" registry. Binds IANA, not a speaker. |
| `7` | Security Considerations | 0 | walked | Security Considerations. States that RFC 4272 does not cover Route-Refresh and that this document does not significantly change the underlying security issues. No countermeasure is directed at a speaker. |
| `8` | Acknowledgements | 0 | skipped (acknowledgements) | Acknowledgements. |
| `9` | not stated | 0 | skipped (references) | Normative References: RFC 2119, RFC 2918, RFC 4271, RFC 4272, RFC 4724, RFC 5291, RFC 5492. |

### Excluded sentences

The walk over RFC 7313 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 7313, so its obligations are stated where they were written.
