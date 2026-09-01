# RFC 7313 - Enhanced Route Refresh Capability for BGP-4

Supported. Every requirement this repository extracted from RFC 7313, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 30.0% | 3 of 10 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 30.0% | 3 of 10 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 10 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 23 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 10 | of 16 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 10 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 40.0% | 4 of 10 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 10 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 16 |
| Gated MUST-level | 10 |
| Obligations that bind Ze | 10 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 4 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 23 |
| Tagged units | 23 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
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

Four MUST-level receive-side gaps annotated in [`rfc/short/rfc7313.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc7313.md): [`RFC7313-4-4`](#rfc7313-4-4)/4-5 -- a received BoRR/EoRR is log-only ([`internal/component/bgp/plugins/rib/rib.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib.go)), so ze marks no Adj-RIB-In routes stale and purges none; and [`RFC7313-4-6`](#rfc7313-4-6)/4-7 -- neither the send nor receive path applies a Graceful-Restart End-of-RIB gate to BoRR emission or acceptance.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the gated population |
| Annotated instead of tested | 7 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **10** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`RFC7313-4-3`](#rfc7313-4-3), [`RFC7313-5-1`](#rfc7313-5-1), [`RFC7313-5-3`](#rfc7313-5-3)

**Annotated instead of tested (7):** [`RFC7313-4-1`](#rfc7313-4-1), [`RFC7313-4-2`](#rfc7313-4-2), [`RFC7313-4-4`](#rfc7313-4-4), [`RFC7313-4-5`](#rfc7313-4-5), [`RFC7313-5-2`](#rfc7313-5-2), [`RFC7313-4-6`](#rfc7313-4-6), [`RFC7313-4-7`](#rfc7313-4-7)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7313-4-1` | Before starting a route refresh (locally initiated or in response to a normal route refresh request), the speaker MUST send a BoRR message (§4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestDispatchBGPPeerBoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/route_refresh/handler/dispatch_test.go#L25). **positive:** `unit/verify` [`TestRFC7313RefreshBracketsReadvertisementWithBoRRAndEoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7313_test.go#L59). **positive:** `unit/verify` [`TestRouteRefreshSubtypes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L139). **negative:** no negative test. **{single-polarity}:** ze dispatches the BoRR before sendRoutes in one straight-line path (internal/component/bgp/plugins/rib/rib.go:1013-1014); there is no re-advertise-without-BoRR code path to drive a negative |
| `RFC7313-4-2` | After completing the re-advertisement of the entire Adj-RIB-Out to the peer, the speaker MUST send an EoRR message (§4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestDispatchBGPPeerEoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/route_refresh/handler/dispatch_test.go#L43). **positive:** `unit/verify` [`TestRFC7313RefreshBracketsReadvertisementWithBoRRAndEoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7313_test.go#L61). **positive:** `unit/verify` [`TestRouteRefreshSubtypes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L142). **negative:** no negative test. **{single-polarity}:** ze dispatches the EoRR after sendRoutes in one straight-line path (internal/component/bgp/plugins/rib/rib.go:1014-1015); there is no re-advertise-without-EoRR code path to drive a negative |
| `RFC7313-4-3` | In processing a ROUTE-REFRESH message, the BGP speaker MUST examine the "message subtype" field and take appropriate actions (§4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestHandleRouteRefreshBoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2494). **positive:** `unit/verify` [`TestHandleRouteRefreshEoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2527). **negative:** `unit/verify` [`TestHandleRouteRefreshUnknown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2562). **negative:** `unit/verify` [`TestHandleRouteRefresh_UnknownSubtype`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L410) |
| `RFC7313-4-4` | When a BGP speaker receives a BoRR, it MUST mark all routes with that <AFI, SAFI> from that peer as stale (§4) | MUST | 4 - Operation | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze logs a received BoRR but does not mark the peer's Adj-RIB-In routes stale -- internal/component/bgp/plugins/rib/rib.go:751-753 handles a received BoRR as log-only and rib_structured.go:506 returns early for a non-zero subtype, so no stale-marking occurs |
| `RFC7313-4-5` | When a BGP speaker receives an EoRR, it MUST immediately remove any routes still marked as stale for that <AFI, SAFI> (§4) | MUST | 4 - Operation | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze logs a received EoRR but performs no stale-route removal -- internal/component/bgp/plugins/rib/rib.go:754-756 handles it as log-only, and because no BoRR stale-marking exists (RFC7313-4-4) there is nothing to purge |
| `RFC7313-5-1` | If the length (excluding fixed header) of a BoRR/EoRR message is not 4, send NOTIFICATION with Error Code 7, subcode 1 (§5) | MUST | 5 - Error Handling | **positive:** `unit/verify` [`TestRouteRefreshBadLengthByMessageSubtype`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L204). **positive:** `unit/verify` [`TestRouteRefreshInvalidLengthNotDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2345). **negative:** `unit/verify` [`TestRouteRefreshBadLengthByMessageSubtype`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L208). **negative:** `unit/verify` [`TestRouteRefreshBadLengthWithoutCapability70`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L117). **negative:** `unit/verify` [`TestRouteRefreshUnknownSubtypeIsIgnoredWhateverItsLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L268). **negative:** `unit/verify` [`TestRouteRefreshValidLengthDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2423). **negative:** `unit/verify` [`TestRouteRefreshWellFormedWithoutCapability70DrawsNoNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L172) |
| `RFC7313-5-2` | The Data field of the NOTIFICATION message MUST contain the complete ROUTE-REFRESH message (§5) | MUST | 5 - Error Handling | **positive:** `unit/verify` [`TestRouteRefreshBadLengthByMessageSubtype`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L206). **positive:** `unit/verify` [`TestRouteRefreshInvalidLengthNotDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2348). **negative:** no negative test. **{single-polarity}:** routeRefreshNotificationData (internal/component/bgp/reactor/session_handlers.go) always copies the entire received body after the header, so every ROUTE-REFRESH NOTIFICATION carries the complete message; there is no truncating code path to drive a negative |
| `RFC7313-5-3` | When receiving a ROUTE-REFRESH with subtype other than 0, 1, or 2, the speaker MUST ignore the message (§5) | MUST | 5 - Error Handling | **positive:** `unit/verify` [`TestHandleRouteRefreshReserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2593). **positive:** `unit/verify` [`TestHandleRouteRefreshUnknown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2560). **positive:** `unit/verify` [`TestRouteRefreshUnknownSubtypeIsIgnoredWhateverItsLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L266). **negative:** `unit/verify` [`TestHandleRouteRefreshBoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2497) |
| `RFC7313-4-6` | A BGP speaker supporting Graceful Restart MUST NOT send a BoRR for an <AFI, SAFI> before sending the EoR for that <AFI, SAFI> (§4) | MUST NOT | 4 - Operation | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's sendRouteRefresh (internal/component/bgp/reactor/reactor_api_forward.go:112) applies no Graceful-Restart End-of-RIB gate, and rib.go:1008-1013 emits the BoRR without checking End-of-RIB state, so the GR/EoR interaction is not enforced |
| `RFC7313-4-7` | A BGP speaker that has received Graceful Restart Capability MUST ignore any BoRRs for an <AFI, SAFI> before receiving the EoR for that <AFI, SAFI> from the neighbor (§4) | MUST | 4 - Operation | **positive:** no positive test. **negative:** no negative test. **{gap}:** the received-BoRR path is log-only (internal/component/bgp/plugins/rib/rib.go:751-753) with no Graceful-Restart End-of-RIB gating, and it depends on the unimplemented stale-marking of RFC7313-4-4 |
| `RFC7313-4-8` | A BGP speaker supporting message subtypes and related procedures SHOULD advertise the Enhanced Route Refresh Capability (§4) | SHOULD | 4 - Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC7313-5-4` | When receiving unknown subtype, the speaker SHOULD log an error for further analysis (§5) | SHOULD | 5 - Error Handling | **positive:** no positive test. **negative:** no negative test |
| `RFC7313-4-9` | When ignoring BoRR before EoR (Graceful Restart), the speaker SHOULD log an error of the condition (§4) | SHOULD | 4 - Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC7313-4-10` | A BGP speaker MAY ignore any EoRR received without a prior receipt of an associated BoRR (§4) | MAY | 4 - Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC7313-4-11` | Purged stale routes MAY be logged for future analysis (§4) | MAY | 4 - Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC7313-4-12` | An implementation MAY impose a locally configurable upper bound on how long it retains stale routes (§4) | MAY | 4 - Operation | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7313-4-4`](#rfc7313-4-4) When a BGP speaker receives a BoRR, it MUST mark all routes with that <AFI, SAFI> from that peer as stale (§4) | {gap}, no test | ze logs a received BoRR but does not mark the peer's Adj-RIB-In routes stale -- internal/component/bgp/plugins/rib/rib.go:751-753 handles a received BoRR as log-only and rib_structured.go:506 returns early for a non-zero subtype, so no stale-marking occurs |
| [`RFC7313-4-5`](#rfc7313-4-5) When a BGP speaker receives an EoRR, it MUST immediately remove any routes still marked as stale for that <AFI, SAFI> (§4) | {gap}, no test | ze logs a received EoRR but performs no stale-route removal -- internal/component/bgp/plugins/rib/rib.go:754-756 handles it as log-only, and because no BoRR stale-marking exists (RFC7313-4-4) there is nothing to purge |
| [`RFC7313-4-6`](#rfc7313-4-6) A BGP speaker supporting Graceful Restart MUST NOT send a BoRR for an <AFI, SAFI> before sending the EoR for that <AFI, SAFI> (§4) | {gap}, no test | ze's sendRouteRefresh (internal/component/bgp/reactor/reactor_api_forward.go:112) applies no Graceful-Restart End-of-RIB gate, and rib.go:1008-1013 emits the BoRR without checking End-of-RIB state, so the GR/EoR interaction is not enforced |
| [`RFC7313-4-7`](#rfc7313-4-7) A BGP speaker that has received Graceful Restart Capability MUST ignore any BoRRs for an <AFI, SAFI> before receiving the EoR for that <AFI, SAFI> from the neighbor (§4) | {gap}, no test | the received-BoRR path is log-only (internal/component/bgp/plugins/rib/rib.go:751-753) with no Graceful-Restart End-of-RIB gating, and it depends on the unimplemented stale-marking of RFC7313-4-4 |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7313-4-1`](#rfc7313-4-1)

Before starting a route refresh (locally initiated or in response to a normal route refresh request), the speaker MUST send a BoRR message (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRouteRefreshSubtypes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L139) | unit/verify | unproven |
| positive | [`TestRFC7313RefreshBracketsReadvertisementWithBoRRAndEoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7313_test.go#L59) | unit/verify | unproven |
| positive | [`TestDispatchBGPPeerBoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/route_refresh/handler/dispatch_test.go#L25) | unit/verify | unproven |

### [`RFC7313-4-2`](#rfc7313-4-2)

After completing the re-advertisement of the entire Adj-RIB-Out to the peer, the speaker MUST send an EoRR message (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRouteRefreshSubtypes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L142) | unit/verify | unproven |
| positive | [`TestRFC7313RefreshBracketsReadvertisementWithBoRRAndEoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7313_test.go#L61) | unit/verify | unproven |
| positive | [`TestDispatchBGPPeerEoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/route_refresh/handler/dispatch_test.go#L43) | unit/verify | unproven |

### [`RFC7313-4-3`](#rfc7313-4-3)

In processing a ROUTE-REFRESH message, the BGP speaker MUST examine the "message subtype" field and take appropriate actions (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestHandleRouteRefresh_UnknownSubtype`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L410) | unit/verify | unproven |
| negative | [`TestHandleRouteRefreshUnknown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2562) | unit/verify | unproven |
| positive | [`TestHandleRouteRefreshBoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2494) | unit/verify | unproven |
| positive | [`TestHandleRouteRefreshEoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2527) | unit/verify | unproven |

### [`RFC7313-4-4`](#rfc7313-4-4)

When a BGP speaker receives a BoRR, it MUST mark all routes with that <AFI, SAFI> from that peer as stale (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7313-4-4, so no unit is bound to it.

### [`RFC7313-4-5`](#rfc7313-4-5)

When a BGP speaker receives an EoRR, it MUST immediately remove any routes still marked as stale for that <AFI, SAFI> (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7313-4-5, so no unit is bound to it.

### [`RFC7313-5-1`](#rfc7313-5-1)

If the length (excluding fixed header) of a BoRR/EoRR message is not 4, send NOTIFICATION with Error Code 7, subcode 1 (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRouteRefreshBadLengthByMessageSubtype`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L208) | unit/verify | unproven |
| negative | [`TestRouteRefreshBadLengthWithoutCapability70`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L117) | unit/verify | unproven |
| negative | [`TestRouteRefreshUnknownSubtypeIsIgnoredWhateverItsLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L268) | unit/verify | unproven |
| negative | [`TestRouteRefreshWellFormedWithoutCapability70DrawsNoNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L172) | unit/verify | unproven |
| negative | [`TestRouteRefreshValidLengthDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2423) | unit/verify | unproven |
| positive | [`TestRouteRefreshBadLengthByMessageSubtype`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L204) | unit/verify | unproven |
| positive | [`TestRouteRefreshInvalidLengthNotDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2345) | unit/verify | unproven |

### [`RFC7313-5-2`](#rfc7313-5-2)

The Data field of the NOTIFICATION message MUST contain the complete ROUTE-REFRESH message (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRouteRefreshBadLengthByMessageSubtype`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L206) | unit/verify | unproven |
| positive | [`TestRouteRefreshInvalidLengthNotDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2348) | unit/verify | unproven |

### [`RFC7313-5-3`](#rfc7313-5-3)

When receiving a ROUTE-REFRESH with subtype other than 0, 1, or 2, the speaker MUST ignore the message (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestHandleRouteRefreshBoRR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2497) | unit/verify | unproven |
| positive | [`TestRouteRefreshUnknownSubtypeIsIgnoredWhateverItsLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7313_error_scope_test.go#L266) | unit/verify | unproven |
| positive | [`TestHandleRouteRefreshReserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2593) | unit/verify | unproven |
| positive | [`TestHandleRouteRefreshUnknown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2560) | unit/verify | unproven |

### [`RFC7313-4-6`](#rfc7313-4-6)

A BGP speaker supporting Graceful Restart MUST NOT send a BoRR for an <AFI, SAFI> before sending the EoR for that <AFI, SAFI> (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7313-4-6, so no unit is bound to it.

### [`RFC7313-4-7`](#rfc7313-4-7)

A BGP speaker that has received Graceful Restart Capability MUST ignore any BoRRs for an <AFI, SAFI> before receiving the EoR for that <AFI, SAFI> from the neighbor (§4)

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
