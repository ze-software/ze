# RFC 2918 - Route Refresh Capability for BGP-4

Supported. Every requirement this repository extracted from RFC 2918, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 6 of 6 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 6 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 6 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 6 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 6 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 40.7% | 11 of 27 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 7 | of 6 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 6 | of 11 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 6 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 6 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 6 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 6 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 6 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 11 |
| Gated MUST-level | 6 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 27 |
| Tagged units | 27 |
| Recorded audit verdicts | 7 |
| Discrimination records | 11 |
| Summary | `rfc/short/rfc2918.md` |
| Requirement shard | `rfc/requirements/rfc2918.md` |
| RFC text | `rfc/full/rfc2918.txt` |

## Enrolment

Enrolled: BGP Route Refresh: six MUST-level requirements, all met and test-bound with positive+negative tags. 2-1 (Route Refresh capability code 2, length 0), 3-1 (ROUTE-REFRESH message type 5), and 3-2 (4-octet AFI+Res+SAFI body, receive length validated) via internal/core/bgp/capability and internal/component/bgp/message tests; 4-1 (send ROUTE-REFRESH only to peers that advertised the capability) via a new internal/component/bgp/reactor test driving the real sendRouteRefresh and SoftClearPeer against Established session state; 4-2 (ignore a refresh for a non-negotiated family) in the reactor; 4-3 (re-advertise the Adj-RIB-Out on a valid refresh) in internal/component/bgp/plugins/rib.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered:**

Route Refresh capability and ROUTE-REFRESH message handling.

**What the ledger says remains:**

No tracked gap in current source anchors.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 6 | one part of the gated population |
| Annotated (including scoped evidence) | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **6** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (6):** [`RFC2918-2-1`](#rfc2918-2-1), [`RFC2918-3-1`](#rfc2918-3-1), [`RFC2918-3-2`](#rfc2918-3-2), [`RFC2918-4-1`](#rfc2918-4-1), [`RFC2918-4-2`](#rfc2918-4-2), [`RFC2918-4-3`](#rfc2918-4-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2918-2-1` | This capability is advertised using the Capability code 2 and Capability length 0. (§2) | MUST | 2 - Route Refresh Capability | **positive:** `unit/verify` [`TestCapabilityWriteTo`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L706). **positive:** `unit/verify` [`TestRFC2918RouteRefreshCapabilityOctetsInSentOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_b_open_caps_test.go#L90). **negative:** `unit/verify` [`TestOpenRejectsMalformedKnownCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L182) |
| `RFC2918-3-1` | The ROUTE-REFRESH message is a new BGP message type defined as follows: Type: 5 - ROUTE-REFRESH (§3) | MUST | 3 - Route-REFRESH Message | **positive:** `unit/verify` [`TestRouteRefreshType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L16). **negative:** `unit/verify` [`TestParseHeaderAllTypes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L39) |
| `RFC2918-3-2` | Message Format: One <AFI, SAFI> encoded as 0 7 15 23 31 +-------+-------+-------+-------+ \| AFI \| Res. \| SAFI \| +-------+-------+-------+-------+ (§3) | MUST | 3 - Route-REFRESH Message | **positive:** `unit/verify` [`TestRouteRefreshPack`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L31). **negative:** `unit/verify` [`TestHandleRouteRefresh_InvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L458). **negative:** `unit/verify` [`TestRouteRefreshUnpackShort`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L75) |
| `RFC2918-4-1` | A BGP speaker may send a ROUTE-REFRESH message to its peer only if it has received the Route Refresh Capability from its peer. (§4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestRFC2918SendRouteRefreshToCapablePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_route_refresh_test.go#L94). **positive:** `unit/verify` [`TestRFC2918SoftClearPeerSendsRefreshToCapablePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_route_refresh_test.go#L158). **negative:** `unit/verify` [`TestRFC2918SendRouteRefreshSkipsPeerWithoutCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_route_refresh_test.go#L115). **negative:** `unit/verify` [`TestRFC2918SoftClearPeerSkipsPeerWithoutCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_route_refresh_test.go#L173) |
| `RFC2918-4-2` | If a BGP speaker receives from its peer a ROUTE-REFRESH message with the <AFI, SAFI> that the speaker didn't advertise to the peer at the session establishment time via capability advertisement, the speaker shall ignore such a message. (S4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestHandleRouteRefresh_NonNegotiatedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L543). **positive:** `unit/verify` [`TestRFC2918RouteRefreshForAnUnadvertisedFamilyIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_unadvertised_family_test.go#L40). **negative:** `unit/verify` [`TestRFC2918RouteRefreshForAnUnadvertisedFamilyIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_unadvertised_family_test.go#L41). **negative:** `unit/verify` [`TestRouteRefreshValidLengthDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2453) |
| `RFC2918-4-3` | Otherwise, the BGP speaker shall re- advertise to that peer the Adj-RIB-Out of the <AFI, SAFI> carried in the message, based on its outbound route filtering policy. (S4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestHandleRefresh_InternalState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_test.go#L1007). **positive:** `unit/verify` [`TestRFC2918ConfigStaticRetainedForRefresh`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc2918_config_static_test.go#L19). **positive:** `unit/verify` [`TestRFC2918RefreshRunsCurrentExportPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_replay_feedback_test.go#L69). **positive:** `unit/verify` [`TestRefreshSentFeedbackRetainsReplayOrigin`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_replay_feedback_test.go#L33). **negative:** `unit/verify` [`TestHandleRefresh_PeerNotUp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_test.go#L1080). **negative:** `unit/verify` [`TestRFC2918RefreshDoesNotSendToDownPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc2918_config_static_test.go#L82). **negative:** `unit/verify` [`TestRFC2918RefreshRunsCurrentExportPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_replay_feedback_test.go#L70). **positive:** `functional/verify` [`refresh-config-static.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/refresh-config-static.ci#L10) |
| `RFC2918-4-4` | A BGP speaker that is willing to receive the ROUTE-REFRESH message from its peer should advertise the Route Refresh Capability to the peer using BGP Capabilities advertisement [BGP-CAP]. (§4) | SHOULD | 4 - Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC2918-4-5` | The <AFI, SAFI> carried in such a message should be one of the <AFI, SAFI> that the peer has advertised to the speaker at the session establishment time via capability advertisement. (§4) | SHOULD | 4 - Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC2918-3-3` | Res. - Reserved (8 bit) field. Should be set to 0 by the sender (§3) | SHOULD | 3 - Route-REFRESH Message | **positive:** no positive test. **negative:** no negative test |
| `RFC2918-4-6` | A BGP speaker may send a ROUTE-REFRESH message to its peer (§4) | MAY | 4 - Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC2918-3-4` | Should be set to 0 by the sender and ignored by the receiver. (§3) | SHOULD | 3 - Route-REFRESH Message | **positive:** `unit/verify` [`TestRFC2918ReservedOctetIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reserved_field_test.go#L112). **positive:** `unit/verify` [`TestRFC2918SentRequestReservedOctetIsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_b_test.go#L24). **negative:** `unit/verify` [`TestRFC2918ReservedOctetDoesNotExemptTheMessage`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reserved_field_test.go#L188) |

## Gaps and untested MUSTs

RFC 2918 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2918-2-1`](#rfc2918-2-1)

This capability is advertised using the Capability code 2 and Capability length 0. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: advertising Route Refresh with a code other than 2 or a length other than 0. TestRFC2918RouteRefreshCapabilityOctetsInSentOpen walks the octets sendOpen writes and asserts exactly one TLV with code 2 and an empty value, and none when not configured; red on another code, a payload, or a duplicate. Negative TestOpenRejectsMalformedKnownCapability: a received code 2 with length 1 (a violation of this definition by the peer) is refused with OPEN Message Error before negotiation. TestCapabilityWriteTo remains a self-consistency check only.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpenRejectsMalformedKnownCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L182) | unit/verify | unproven |
| positive | [`TestRFC2918RouteRefreshCapabilityOctetsInSentOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_b_open_caps_test.go#L90) | unit/verify | revert, verified |
| positive | [`TestCapabilityWriteTo`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L706) | unit/verify | unproven |

### [`RFC2918-3-1`](#rfc2918-3-1)

The ROUTE-REFRESH message is a new BGP message type defined as follows: Type: 5 - ROUTE-REFRESH (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a ROUTE-REFRESH carrying a type other than 5. TestRouteRefreshType asserts r.Type() == msgtype.MessageType(5), red if the send-side type changes. TestParseHeaderAllTypes asserts header type byte 5 decodes to TypeROUTEREFRESH and bytes 1..4 decode to the other types, red if the receive mapping moves.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseHeaderAllTypes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L39) | unit/verify | unproven |
| positive | [`TestRouteRefreshType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L16) | unit/verify | unproven |

### [`RFC2918-3-2`](#rfc2918-3-2)

Message Format: One <AFI, SAFI> encoded as 0 7 15 23 31 +-------+-------+-------+-------+ | AFI | Res. | SAFI | +-------+-------+-------+-------+ (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a body other than one <AFI, SAFI> in 32 bits (AFI 16, Res 8, SAFI 8). TestRouteRefreshPack asserts Len(data) == HeaderLen+4 and each of the four body bytes, red on a wrong size or layout. TestHandleRouteRefresh_InvalidLength asserts ErrInvalidMessage for 3-, 5- and 0-byte bodies, and TestRouteRefreshUnpackShort asserts ErrShortRead for 3 bytes, red if a body of another length is accepted.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRouteRefreshUnpackShort`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L75) | unit/verify | unproven |
| negative | [`TestHandleRouteRefresh_InvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L458) | unit/verify | unproven |
| positive | [`TestRouteRefreshPack`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L31) | unit/verify | unproven |

### [`RFC2918-4-1`](#rfc2918-4-1)

A BGP speaker may send a ROUTE-REFRESH message to its peer only if it has received the Route Refresh Capability from its peer. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: sending ROUTE-REFRESH to a peer that did not advertise the capability. TestRFC2918SendRouteRefreshSkipsPeerWithoutCapability (both subtests) and TestRFC2918SoftClearPeerSkipsPeerWithoutCapability assert require.Empty(conn.written()) and RefreshSent == 0, red if either send entry skips the gate. The positives assert the message reaches the wire for a capable peer.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2918SendRouteRefreshSkipsPeerWithoutCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_route_refresh_test.go#L115) | unit/verify | unproven |
| negative | [`TestRFC2918SoftClearPeerSkipsPeerWithoutCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_route_refresh_test.go#L173) | unit/verify | unproven |
| positive | [`TestRFC2918SendRouteRefreshToCapablePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_route_refresh_test.go#L94) | unit/verify | unproven |
| positive | [`TestRFC2918SoftClearPeerSendsRefreshToCapablePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_route_refresh_test.go#L158) | unit/verify | unproven |

### [`RFC2918-4-2`](#rfc2918-4-2)

If a BGP speaker receives from its peer a ROUTE-REFRESH message with the <AFI, SAFI> that the speaker didn't advertise to the peer at the session establishment time via capability advertisement, the speaker shall ignore such a message. (S4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: acting on a ROUTE-REFRESH for an <AFI, SAFI> the speaker did not advertise. D-8 fix: Session.screenRouteRefresh now applies the family rule on the read path before onMessageReceived, for both call sites. TestRFC2918RouteRefreshForAnUnadvertisedFamilyIsIgnored drives ReadAndProcess on an Established session that advertised IPv4/unicast only: positive, an IPv6/unicast refresh reaches no onMessageReceived consumer, draws no bytes on the unbuffered pipe and leaves the session Established; negative, an IPv4/unicast refresh is delivered exactly once with its body unchanged. Observed-red revert records on routeRefreshFamilyUnadvertised for both polarities. The check keys on the negotiated set, a subset of what the speaker advertised, so every unadvertised family is ignored; it additionally ignores a family advertised but not negotiated, which RFC 4760 already forbids acting on. The older positive TestHandleRouteRefresh_NonNegotiatedFamily (NoError only) adds nothing on its own.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2918RouteRefreshForAnUnadvertisedFamilyIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_unadvertised_family_test.go#L41) | unit/verify | revert, verified |
| negative | [`TestRouteRefreshValidLengthDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2453) | unit/verify | unproven |
| positive | [`TestRFC2918RouteRefreshForAnUnadvertisedFamilyIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_unadvertised_family_test.go#L40) | unit/verify | revert, verified |
| positive | [`TestHandleRouteRefresh_NonNegotiatedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L543) | unit/verify | unproven |

### [`RFC2918-4-3`](#rfc2918-4-3)

Otherwise, the BGP speaker shall re- advertise to that peer the Adj-RIB-Out of the <AFI, SAFI> carried in the message, based on its outbound route filtering policy. (S4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Read all of RFC 2918 Section 4, including the advertised-family precondition: 'Otherwise, the BGP speaker shall re-advertise to that peer the Adj-RIB-Out of the <AFI, SAFI> carried in the message, based on its outbound route filtering policy.' Read all seven tagged carriers, including both policy polarities. rib_test.go::TestHandleRefresh_InternalState seeds two IPv4 routes plus an IPv6 control, requests IPv4, parses every emitted command with the production update parser, and asserts exactly the two requested routes plus replay metadata. It also checks both markers and retained families; its separate marker capture does not itself prove ordering. rfc2918_config_static_test.go::TestRFC2918ConfigStaticRetainedForRefresh ingests a real sent UPDATE through handleSentStructured, invokes handleRefreshStructured, parses its replay, and checks the exact prefix and packed attributes. It also distinguishes refresh ownership feedback from an ordinary replacement. The same file's TestRFC2918RefreshDoesNotSendToDownPeer observes both output hooks and forbids routes and markers. The older rib_test.go::TestHandleRefresh_PeerNotUp remains weak by itself: it asserts only peerUp remains false and cannot detect a send. Its weakness is not used as negative evidence. reactor/rfc2918_replay_feedback_test.go::TestRFC2918RefreshRunsCurrentExportPolicy supplies an established peer and negotiated IPv4 family, sends the same Replay batch with policy accept then reject, and asserts one policy call plus new wire bytes for acceptance, then a second policy call and byte-identical wire for rejection. These are two distinct assertions, not one wearing both polarities; current export rejection is the meaningful negative of the policy clause. TestRefreshSentFeedbackRetainsReplayOrigin checks [true,false] sent-event metadata for immediate and queued refresh, followed by an ordinary announcement; it observes feedback rather than decoding socket bytes. test/plugin/refresh-config-static.ci supplies the independent full-stack proof boundary: exact initial UPDATE, live ROUTE-REFRESH, then exact BoRR, identical configured UPDATE, and EoRR. Producers read: rib.go::handleRefresh and rib_structured.go::handleRefreshStructured select the peer/family inventory through ribout_entry.go::collectRibOutRoutes; sendRoutes applies replay metadata to FormatAnnounceCommand output; handleSentStructured retains configured advertisements and ignores replay ownership feedback; reactor_api_batch.go::AnnounceNLRIBatch and announceBatchToPeers, peer_initial_sync.go::sendInitialRoutes, session_write.go::sendUpdateCounted/writeUpdateGated and egress_inject_filter.go::exportFilterForBody carry replay through current egress policy. Together the tests discriminate omitted replay, wrong-family replay, configured-route omission, ownership loss and bypassed current policy, covering the whole sentence rather than storage-only state. This is a source-based semantic rejudgment, not a claim of new execution. Parent's observed gate reports stale positive records for TestRFC2918ConfigStaticRetainedForRefresh against handleSentStructured, TestHandleRefresh_InternalState against sendRoutes, and refresh-config-static.ci against handleSentStructured; native renewals are owed after the ADD-PATH changes. The down-peer and reactor replay/policy records are not reported stale for this row in that gate, and unrelated reactor records must not be refreshed on this judgment. No tests, mutations, builds, stamping, resealing or checks were run here.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2918RefreshDoesNotSendToDownPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc2918_config_static_test.go#L82) | unit/verify | revert, verified |
| negative | [`TestHandleRefresh_PeerNotUp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_test.go#L1080) | unit/verify | unproven |
| negative | [`TestRFC2918RefreshRunsCurrentExportPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_replay_feedback_test.go#L70) | unit/verify | revert, verified |
| positive | [`TestRFC2918ConfigStaticRetainedForRefresh`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc2918_config_static_test.go#L19) | unit/verify | revert, verified |
| positive | [`TestHandleRefresh_InternalState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_test.go#L1007) | unit/verify | revert, verified |
| positive | [`TestRFC2918RefreshRunsCurrentExportPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_replay_feedback_test.go#L69) | unit/verify | revert, verified |
| positive | [`TestRefreshSentFeedbackRetainsReplayOrigin`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_replay_feedback_test.go#L33) | unit/verify | revert, verified |
| positive | [`refresh-config-static.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/refresh-config-static.ci#L10) | functional/verify | revert, verified |

### [`RFC2918-3-4`](#rfc2918-3-4)

Should be set to 0 by the sender and ignored by the receiver. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a sender writing a non-zero Reserved octet, or a receiver acting on it. Sender: TestRFC2918SentRequestReservedOctetIsZero reads the four body octets written by both request producers (sendRouteRefresh, SoftClearPeer) and asserts exactly 00 01 00 01. Receiver: TestRFC2918ReservedOctetIgnoredOnReceive asserts delivery, Established and no NOTIFICATION for octets 00/01/02/AA/FF; TestRFC2918ReservedOctetDoesNotExemptTheMessage asserts a 5-octet body is still refused. The sender half is single-polarity by nature (no violating input exists for an emitter).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2918ReservedOctetDoesNotExemptTheMessage`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reserved_field_test.go#L188) | unit/verify | unproven |
| positive | [`TestRFC2918SentRequestReservedOctetIsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_b_test.go#L24) | unit/verify | revert, verified |
| positive | [`TestRFC2918ReservedOctetIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reserved_field_test.go#L112) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-fixit-rfc-drain-quota-never-armed WP-1 |
| Signed off | 2026-08-31 |
| Register | prose |
| Source | rfc/full/rfc2918.txt |
| Source fingerprint | 705e36a852d934fb |
| Record | rfc/extraction/rfc2918.json |
| Mapped sentences | 2 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of this Memo, Copyright Notice and Abstract. The Abstract says what the document defines, a capability and a message that let one speaker ask another to re-advertise its Adj-RIB-Out, and it binds no speaker. |
| `1` | Introduction | 2 | walked | Introduction. States the problem: BGP-4 has no way to ask a peer to re-advertise its Adj-RIB-Out, so the common answer is soft-reconfiguration, which keeps an unmodified copy of every route from the peer. Both of its sites describe that problem in indicative prose and are excluded below. The document's own rules start at section 2. |
| `2` | Route Refresh Capability | 0 | walked | Route Refresh Capability. Fixes the capability code at 2 and the capability length at 0, and says what advertising the capability conveys to the peer. Every sentence is indicative ('This capability is advertised using the Capability code 2 and Capability length 0'), so no site derives here and the one requirement the section carries is listed unsourced. |
| `3` | Route-REFRESH Message | 0 | walked | Route-REFRESH Message. Fixes the message type at 5 and the message body at one 4-byte <AFI, SAFI>, and states the Reserved field's handling. The type line and the field diagram carry no modal at all, and the Reserved field's 'Should be set to 0 by the sender and ignored by the receiver' is not sited here by either scan, so all four requirements read from this section are listed unsourced. |
| `4` | Operation | 2 | walked | Operation. The only section that binds a speaker on the wire. Its two sites are the two 'shall' sentences of the receive path and are mapped to RFC2918-4-2 and RFC2918-4-3. The four other requirements read from this section come from its 'may ... only if', 'should advertise' and 'should be one of' sentences, which no scan sites, and are listed unsourced. |
| `5` | Security Considerations | 0 | walked | Security Considerations. One sentence: this extension to BGP does not change the underlying security issues. It directs no countermeasure at a speaker. |
| `6` | Acknowledgments | 0 | skipped (acknowledgements) | Acknowledgments. Names IDRP as the source of the Route Refresh concept and thanks four reviewers. |
| `7` | References: RFC 1771, RFC 2858, RFC 2842 | 0 | skipped (references) | References: RFC 1771, RFC 2858, RFC 2842. |
| `8` | Author's Address | 0 | walked | Author's Address. Postal address and e-mail for the author. No obligation. |
| `9` | Full Copyright Statement | 1 | walked | Full Copyright Statement. The Internet Society boilerplate governing copying and translation of the document. Its one site is a condition on republishing the text and is excluded below. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A description of the problem, in the Introduction, written in indicative prose. The 'must' states what necessarily follows from a policy change, that the prefixes have to be available again to be re-examined, and it is the motivation for the document rather than a rule the document adds. It names no message, field or timer for a speaker to get right. | When the inbound routing policy for a peer changes, all prefixes from that peer must be somehow made available and then re- examined against the new policy. |
| `1:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A description of another approach, in the Introduction. The subject is soft-reconfiguration, which the sentence before it defines as storing an unmodified copy of all routes from the peer. 'Are required' reports the cost of that approach, which is what motivates this document; it directs nobody. | Additional memory and CPU are required to maintain these routes. |
| `9:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Internet Society copyright boilerplate that the site scanner did not strip. Its 'must' governs the copyright procedures to be followed when the document text is republished or translated, and it binds a publisher of the document rather than an implementation of the protocol the document specifies. | However, this document itself may not be modified in any way, such as by removing the copyright notice or references to the Internet Society or other Internet organizations, except as needed for the purpose of developing Internet standards in which case the procedures for copyrights defined in the Internet Standards process must be followed, or as required to translate it into languages other than English. |

## Superseded

No document obsoletes RFC 2918, so its obligations are stated where they were written.
