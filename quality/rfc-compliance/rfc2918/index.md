# RFC 2918 - Route Refresh Capability for BGP-4

Route Refresh. Every requirement this repository extracted from RFC 2918, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 6 | of 6 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 6 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 6 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 6 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0 | of 6 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 11 | 11 of 27 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 7 | of 6 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 6 | of 11 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 6 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 6 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 6 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 6 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 6 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

The 9 counts marked as a part above are the whole of the 6 gated MUSTs: they add up to 6. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. It counts TAGGED UNITS, not obligations, so this count is not one of the parts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public row | Route Refresh |
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

**Area:** Route Refresh

**Proof:** 3 of 6 MUST proven

**Evidence:** functional 1, unit 2

**Unmet:** -


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

## Figures

- **MUST bar**: Ze proven 3, Ze correctness gap 3
- **Overview bar**: Ze MUST proven 3, SHOULD proven 1, SHOULD not decided 4, Ze correctness gap 3

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 6 |
| MUST rows kept (the MUST bar) | 6 |
| Ze proven | 3 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 3 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 5 |
| SHOULD proven | 1 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 4 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/rfc2918.json](../../../data/rfc/rfc2918.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2918-2-1` | This capability is advertised using the Capability code 2 and Capability length 0. (§2) | MUST | 2 - Route Refresh Capability | **positive:** `unit/verify` [`TestCapabilityWriteTo`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L706). **positive:** `unit/verify` [`TestRFC2918RouteRefreshCapabilityOctetsInSentOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_b_open_caps_test.go#L90). **negative:** `unit/verify` [`TestOpenRejectsMalformedKnownCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L182) |
| `RFC2918-3-1` | The ROUTE-REFRESH message is a new BGP message type defined as follows: Type: 5 - ROUTE-REFRESH (§3) | MUST | 3 - Route-REFRESH Message | **positive:** `unit/verify` [`TestRouteRefreshType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L16). **negative:** `unit/verify` [`TestParseHeaderAllTypes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L39) |
| `RFC2918-3-2` | Message Format: One <AFI, SAFI> encoded as 0 7 15 23 31 +-------+-------+-------+-------+ \| AFI \| Res. \| SAFI \| +-------+-------+-------+-------+ (§3) | MUST | 3 - Route-REFRESH Message | **positive:** `unit/verify` [`TestRouteRefreshPack`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L31). **negative:** `unit/verify` [`TestHandleRouteRefresh_InvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L459). **negative:** `unit/verify` [`TestRouteRefreshUnpackShort`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L75) |
| `RFC2918-4-1` | A BGP speaker may send a ROUTE-REFRESH message to its peer only if it has received the Route Refresh Capability from its peer. (§4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestRFC2918SendRouteRefreshToCapablePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_route_refresh_test.go#L94). **positive:** `unit/verify` [`TestRFC2918SoftClearPeerSendsRefreshToCapablePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_route_refresh_test.go#L158). **negative:** `unit/verify` [`TestRFC2918SendRouteRefreshSkipsPeerWithoutCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_route_refresh_test.go#L115). **negative:** `unit/verify` [`TestRFC2918SoftClearPeerSkipsPeerWithoutCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_route_refresh_test.go#L173) |
| `RFC2918-4-2` | If a BGP speaker receives from its peer a ROUTE-REFRESH message with the <AFI, SAFI> that the speaker didn't advertise to the peer at the session establishment time via capability advertisement, the speaker shall ignore such a message. (S4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestHandleRouteRefresh_NonNegotiatedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L544). **positive:** `unit/verify` [`TestRFC2918RouteRefreshForAnUnadvertisedFamilyIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_unadvertised_family_test.go#L40). **negative:** `unit/verify` [`TestRFC2918RouteRefreshForAnUnadvertisedFamilyIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_unadvertised_family_test.go#L41). **negative:** `unit/verify` [`TestRouteRefreshValidLengthDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2454) |
| `RFC2918-4-3` | Otherwise, the BGP speaker shall re- advertise to that peer the Adj-RIB-Out of the <AFI, SAFI> carried in the message, based on its outbound route filtering policy. (S4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestHandleRefresh_InternalState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_test.go#L1011). **positive:** `unit/verify` [`TestRFC2918ConfigStaticRetainedForRefresh`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc2918_config_static_test.go#L19). **positive:** `unit/verify` [`TestRFC2918RefreshRunsCurrentExportPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_replay_feedback_test.go#L69). **positive:** `unit/verify` [`TestRefreshSentFeedbackRetainsReplayOrigin`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_replay_feedback_test.go#L33). **negative:** `unit/verify` [`TestHandleRefresh_PeerNotUp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_test.go#L1084). **negative:** `unit/verify` [`TestRFC2918RefreshDoesNotSendToDownPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc2918_config_static_test.go#L87). **negative:** `unit/verify` [`TestRFC2918RefreshRunsCurrentExportPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_replay_feedback_test.go#L70). **positive:** `functional/verify` [`refresh-config-static.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/refresh-config-static.ci#L10) |
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
| negative | [`TestHandleRouteRefresh_InvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L459) | unit/verify | unproven |
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
| negative | [`TestRouteRefreshValidLengthDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2454) | unit/verify | unproven |
| positive | [`TestRFC2918RouteRefreshForAnUnadvertisedFamilyIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_unadvertised_family_test.go#L40) | unit/verify | revert, verified |
| positive | [`TestHandleRouteRefresh_NonNegotiatedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L544) | unit/verify | unproven |

### [`RFC2918-4-3`](#rfc2918-4-3)

Otherwise, the BGP speaker shall re- advertise to that peer the Adj-RIB-Out of the <AFI, SAFI> carried in the message, based on its outbound route filtering policy. (S4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31 by an independent auditor who authored none of faf7c5c380. RFC 2918 Section 4: "Otherwise, the BGP speaker shall re- advertise to that peer the Adj-RIB-Out of the <AFI, SAFI> carried in the message, based on its outbound route filtering policy." faf7c5c380 (filter chain model moved into internal/component/policy) changed one line in the tagged unit internal/component/bgp/reactor/rfc2918_replay_feedback_test.go::TestRFC2918RefreshRunsCurrentExportPolicy: mustPolicy(t, []filterapi.FilterRef{{Name: "refresh-policy"}}, filterapi.FinalAccept) became mustPolicy(t, []routepolicy.Ref{{Name: "refresh-policy"}}, routepolicy.FinalAccept). mustPolicy still calls filterapi.NewPolicy with the same single unbound ref and the explicit accept final (NewPolicy now wraps policy.NewChain; an unbound ref stays external as the zero inProcess field did), so the policy the fixture installs, every input and every assertion are unchanged. No other commit touched the file since the verdict was stamped. The unit still requires that an accepted replay grows the wire output with exactly one policy call, and that flipping only the policy response to reject calls it again and leaves the wire bytes identical, so the outbound policy decides the re-advertisement in both polarities. Every other tagged carrier is byte-identical and its judgement stands. Verdict unchanged: enforced. Ran: go test -tags ze_bgp -run over the six faf7c5c380-changed reactor units, PASS on 2026-10-31. Earlier judgements are in this file's git history.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2918RefreshDoesNotSendToDownPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc2918_config_static_test.go#L87) | unit/verify | revert, verified |
| negative | [`TestHandleRefresh_PeerNotUp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_test.go#L1084) | unit/verify | unproven |
| negative | [`TestRFC2918RefreshRunsCurrentExportPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_replay_feedback_test.go#L70) | unit/verify | revert, verified |
| positive | [`TestRFC2918ConfigStaticRetainedForRefresh`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc2918_config_static_test.go#L19) | unit/verify | revert, verified |
| positive | [`TestHandleRefresh_InternalState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_test.go#L1011) | unit/verify | revert, verified |
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
