# RFC 2918 - Route Refresh Capability for BGP-4

Route Refresh. Every requirement listed for RFC 2918, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 6 | of 6 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 6 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 6 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 6 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 6 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 17 | 17 of 27 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 6 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 6 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 6 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 6 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 6 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 6 | of 11 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 6 mandatory requirements: they add up to 6. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Public row | Route Refresh |
| Automatic checks | Included |
| Requirements | 11 |
| Mandatory requirements (MUST level) | 6 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 27 |
| Tests linked to requirements | 27 |
| Recorded audit verdicts | 7 |
| Fault test records | 17 |
| Summary | `rfc/short/rfc2918.md` |
| Requirement table | `rfc/requirements/rfc2918.md` |
| RFC text | `rfc/full/rfc2918.txt` |

## Automatic checks

Included in automatic checks: BGP Route Refresh: six MUST-level requirements, all met and test-bound with positive+negative tags. 2-1 (Route Refresh capability code 2, length 0), 3-1 (ROUTE-REFRESH message type 5), and 3-2 (4-octet AFI+Res+SAFI body, receive length validated) via internal/core/bgp/capability and internal/component/bgp/message tests; 4-1 (send ROUTE-REFRESH only to peers that advertised the capability) via a new internal/component/bgp/reactor test driving the real sendRouteRefresh and SoftClearPeer against Established session state; 4-2 (ignore a refresh for a non-negotiated family) in the reactor; 4-3 (re-advertise the Adj-RIB-Out on a valid refresh) in internal/component/bgp/plugins/rib.

## Published support status

**Area:** Route Refresh

**Test verification:** 6 of 6 MUST proven

**Evidence:** functional 1, unit 5

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 6 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **6** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (6):** [`RFC2918-2-1`](#rfc2918-2-1), [`RFC2918-3-1`](#rfc2918-3-1), [`RFC2918-3-2`](#rfc2918-3-2), [`RFC2918-4-1`](#rfc2918-4-1), [`RFC2918-4-2`](#rfc2918-4-2), [`RFC2918-4-3`](#rfc2918-4-3)

## Figures

- **MUST bar**: Ze verified 6
- **Overview bar**: Ze MUST verified 6, SHOULD verified 1, Not assessed 4

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 6 |
| MUST rows kept (the MUST bar) | 6 |
| Ze verified | 6 |
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
| SHOULD / MAY rows | 5 |
| SHOULD verified | 1 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 4 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc2918.json](../../../data/rfc/rfc2918.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2918-2-1` | This capability is advertised using the Capability code 2 and Capability length 0. (§2) | MUST | 2 - Route Refresh Capability | **positive:** `unit/verify` [`TestCapabilityWriteTo`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L706). **positive:** `unit/verify` [`TestRFC2918RouteRefreshCapabilityOctetsInSentOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_b_open_caps_test.go#L90). **negative:** `unit/verify` [`TestOpenRejectsMalformedKnownCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L196) |
| `RFC2918-3-1` | The ROUTE-REFRESH message is a new BGP message type defined as follows: Type: 5 - ROUTE-REFRESH (§3) | MUST | 3 - Route-REFRESH Message | **positive:** `unit/verify` [`TestRouteRefreshType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L16). **negative:** `unit/verify` [`TestParseHeaderAllTypes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L39) |
| `RFC2918-3-2` | Message Format: One <AFI, SAFI> encoded as 0 7 15 23 31 +-------+-------+-------+-------+ \| AFI \| Res. \| SAFI \| +-------+-------+-------+-------+ (§3) | MUST | 3 - Route-REFRESH Message | **positive:** `unit/verify` [`TestRouteRefreshPack`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L31). **negative:** `unit/verify` [`TestHandleRouteRefresh_InvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L473). **negative:** `unit/verify` [`TestRouteRefreshUnpackShort`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L75) |
| `RFC2918-4-1` | A BGP speaker may send a ROUTE-REFRESH message to its peer only if it has received the Route Refresh Capability from its peer. (§4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestRFC2918SendRouteRefreshToCapablePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_route_refresh_test.go#L94). **positive:** `unit/verify` [`TestRFC2918SoftClearPeerSendsRefreshToCapablePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_route_refresh_test.go#L158). **negative:** `unit/verify` [`TestRFC2918SendRouteRefreshSkipsPeerWithoutCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_route_refresh_test.go#L115). **negative:** `unit/verify` [`TestRFC2918SoftClearPeerSkipsPeerWithoutCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_route_refresh_test.go#L173) |
| `RFC2918-4-2` | If a BGP speaker receives from its peer a ROUTE-REFRESH message with the <AFI, SAFI> that the speaker didn't advertise to the peer at the session establishment time via capability advertisement, the speaker shall ignore such a message. (S4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestHandleRouteRefresh_NonNegotiatedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L560). **positive:** `unit/verify` [`TestRFC2918RouteRefreshForAnUnadvertisedFamilyIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_unadvertised_family_test.go#L40). **negative:** `unit/verify` [`TestRFC2918RouteRefreshForAnUnadvertisedFamilyIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_unadvertised_family_test.go#L41). **negative:** `unit/verify` [`TestRouteRefreshValidLengthDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2454) |
| `RFC2918-4-3` | Otherwise, the BGP speaker shall re- advertise to that peer the Adj-RIB-Out of the <AFI, SAFI> carried in the message, based on its outbound route filtering policy. (S4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestHandleRefresh_InternalState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_test.go#L1047). **positive:** `unit/verify` [`TestRFC2918ConfigStaticRetainedForRefresh`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc2918_config_static_test.go#L19). **positive:** `unit/verify` [`TestRFC2918RefreshRunsCurrentExportPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_replay_feedback_test.go#L68). **positive:** `unit/verify` [`TestRefreshSentFeedbackRetainsReplayOrigin`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_replay_feedback_test.go#L32). **negative:** `unit/verify` [`TestHandleRefresh_PeerNotUp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_test.go#L1125). **negative:** `unit/verify` [`TestRFC2918RefreshDoesNotSendToDownPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc2918_config_static_test.go#L91). **negative:** `unit/verify` [`TestRFC2918RefreshRunsCurrentExportPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_replay_feedback_test.go#L69). **positive:** `functional/verify` [`refresh-config-static.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/refresh-config-static.ci#L10) |
| `RFC2918-4-4` | A BGP speaker that is willing to receive the ROUTE-REFRESH message from its peer should advertise the Route Refresh Capability to the peer using BGP Capabilities advertisement [BGP-CAP]. (§4) | SHOULD | 4 - Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC2918-4-5` | The <AFI, SAFI> carried in such a message should be one of the <AFI, SAFI> that the peer has advertised to the speaker at the session establishment time via capability advertisement. (§4) | SHOULD | 4 - Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC2918-3-3` | Res. - Reserved (8 bit) field. Should be set to 0 by the sender (§3) | SHOULD | 3 - Route-REFRESH Message | **positive:** no positive test. **negative:** no negative test |
| `RFC2918-4-6` | A BGP speaker may send a ROUTE-REFRESH message to its peer (§4) | MAY | 4 - Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC2918-3-4` | Should be set to 0 by the sender and ignored by the receiver. (§3) | SHOULD | 3 - Route-REFRESH Message | **positive:** `unit/verify` [`TestRFC2918ReservedOctetIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reserved_field_test.go#L112). **positive:** `unit/verify` [`TestRFC2918SentRequestReservedOctetIsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_b_test.go#L24). **negative:** `unit/verify` [`TestRFC2918ReservedOctetDoesNotExemptTheMessage`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reserved_field_test.go#L188) |

## Gaps and untested MUSTs

RFC 2918 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC2918-2-1`](#rfc2918-2-1)

This capability is advertised using the Capability code 2 and Capability length 0. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: advertising Route Refresh with a code other than 2 or a length other than 0. TestRFC2918RouteRefreshCapabilityOctetsInSentOpen walks the octets sendOpen writes and asserts exactly one TLV with code 2 and an empty value, and none when not configured; red on another code, a payload, or a duplicate. Negative TestOpenRejectsMalformedKnownCapability: a received code 2 with length 1 (a violation of this definition by the peer) is refused with OPEN Message Error before negotiation. TestCapabilityWriteTo remains a self-consistency check only.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestOpenRejectsMalformedKnownCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L196) | unit/verify | not verified |
| positive | [`TestRFC2918RouteRefreshCapabilityOctetsInSentOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_b_open_caps_test.go#L90) | unit/verify | revert, verified |
| positive | [`TestCapabilityWriteTo`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L706) | unit/verify | not verified |

### [`RFC2918-3-1`](#rfc2918-3-1)

The ROUTE-REFRESH message is a new BGP message type defined as follows: Type: 5 - ROUTE-REFRESH (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a ROUTE-REFRESH carrying a type other than 5. TestRouteRefreshType asserts r.Type() == msgtype.MessageType(5), red if the send-side type changes. TestParseHeaderAllTypes asserts header type byte 5 decodes to TypeROUTEREFRESH and bytes 1..4 decode to the other types, red if the receive mapping moves.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestParseHeaderAllTypes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L39) | unit/verify | revert, verified |
| positive | [`TestRouteRefreshType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L16) | unit/verify | revert, verified |

### [`RFC2918-3-2`](#rfc2918-3-2)

Message Format: One <AFI, SAFI> encoded as 0 7 15 23 31 +-------+-------+-------+-------+ | AFI | Res. | SAFI | +-------+-------+-------+-------+ (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a body other than one <AFI, SAFI> in 32 bits (AFI 16, Res 8, SAFI 8). TestRouteRefreshPack asserts Len(data) == HeaderLen+4 and each of the four body bytes, red on a wrong size or layout. TestHandleRouteRefresh_InvalidLength asserts ErrInvalidMessage for 3-, 5- and 0-byte bodies, and TestRouteRefreshUnpackShort asserts ErrShortRead for 3 bytes, red if a body of another length is accepted. Independent companion-fixture rejudgment against RFC 2918 Section 3's exact AFI(16), Reserved(8), SAFI(8) format, all three tagged units and current handler producer. The repaired handler fixture now establishes the session and confirms Enhanced Route Refresh negotiation before presenting the same independent 3-, 5- and zero-byte bodies. Each still must return ErrInvalidMessage with the exact invalid-length diagnostic; its assertions and malformed input isolation are unchanged. The independent pack carrier still asserts exactly four body bytes in the required layout, and the short decoder carrier still rejects three bytes. The state precondition avoids an earlier FSM rejection masking the length rule. All existing entries and the enforced verdict remain.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRouteRefreshUnpackShort`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L75) | unit/verify | not verified |
| negative | [`TestHandleRouteRefresh_InvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L473) | unit/verify | mutant, verified |
| positive | [`TestRouteRefreshPack`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L31) | unit/verify | not verified |

### [`RFC2918-4-1`](#rfc2918-4-1)

A BGP speaker may send a ROUTE-REFRESH message to its peer only if it has received the Route Refresh Capability from its peer. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: sending ROUTE-REFRESH to a peer that did not advertise the capability. TestRFC2918SendRouteRefreshSkipsPeerWithoutCapability (both subtests) and TestRFC2918SoftClearPeerSkipsPeerWithoutCapability assert require.Empty(conn.written()) and RefreshSent == 0, red if either send entry skips the gate. The positives assert the message reaches the wire for a capable peer.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2918SendRouteRefreshSkipsPeerWithoutCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_route_refresh_test.go#L115) | unit/verify | not verified |
| negative | [`TestRFC2918SoftClearPeerSkipsPeerWithoutCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_route_refresh_test.go#L173) | unit/verify | revert, verified |
| positive | [`TestRFC2918SendRouteRefreshToCapablePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_route_refresh_test.go#L94) | unit/verify | not verified |
| positive | [`TestRFC2918SoftClearPeerSendsRefreshToCapablePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_route_refresh_test.go#L158) | unit/verify | revert, verified |

### [`RFC2918-4-2`](#rfc2918-4-2)

If a BGP speaker receives from its peer a ROUTE-REFRESH message with the <AFI, SAFI> that the speaker didn't advertise to the peer at the session establishment time via capability advertisement, the speaker shall ignore such a message. (S4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent current-source rejudgment against RFC2918 Section4's own family-advertisement clause, preserving enforced and all four existing tagged entries. Read every whole tagged root and its actual capability/handshake/callback observers, the current screenRouteRefresh and routeRefreshFamilyUnadvertised producers, and the pre-consumer Session read/delivery boundary. The real read-path root negotiates IPv4 unicast and no capability70, first sends IPv6 unicast and requires no consumer delivery, no bytes during the bounded unbuffered-pipe read, and Established; it then sends advertised IPv4 unicast and requires exactly one consumer delivery with the exact original body and Established. Advertised-family delivery is the useful control against unconditional suppression. Current callback migration merely adds the unused captured *Session argument, preserving the original oracle. The supplementary handler root is Established with IPv4-only Route Refresh before the same IPv6 input and asserts only NoError; it alone proves no suppression. Separate TestRouteRefreshValidLengthDelivered requires exact IPv4 body, one message/refresh callback and Established; actual job-unit-pkg-59a2fc52.log25-27 observes that unchanged root and package PASS, with Main retaining race-run custody. The family producer reads the negotiated family set, a subset of local advertisement, so it ignores every unadvertised family and also one advertised but not negotiated; this fixture does not distinguish those two cases. Current exact positive/negative pair receipts are job-advertiser-legacy-refresh-and-receive-framing-proof-ae214313.log: whole root clean PASS4 and88; disabling routeRefreshFamilyUnadvertised panics on the FIRST IPv6 request at root59 (log9-38 and93-122), and native records45/129 bind both current claims. Both breaks therefore detect the earlier ignored-family branch before the later advertised-family control executes; they are whole-producer dependency observations, not separate selective over-suppression mutations. Current clean assertions and source judgment, not those panics alone, supply the advertised-family control. Preserve the separate existing handler NoError mutation record as historical evidence with its narrower scope; no fresh execution is attributed to it. No external peer re-advertisement, plugin authorization or selected-output completion is inferred from this receive-screening proof.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2918RouteRefreshForAnUnadvertisedFamilyIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_unadvertised_family_test.go#L41) | unit/verify | revert, verified |
| negative | [`TestRouteRefreshValidLengthDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2454) | unit/verify | not verified |
| positive | [`TestRFC2918RouteRefreshForAnUnadvertisedFamilyIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_unadvertised_family_test.go#L40) | unit/verify | revert, verified |
| positive | [`TestHandleRouteRefresh_NonNegotiatedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L560) | unit/verify | mutant, verified |

### [`RFC2918-4-3`](#rfc2918-4-3)

Otherwise, the BGP speaker shall re- advertise to that peer the Adj-RIB-Out of the <AFI, SAFI> carried in the message, based on its outbound route filtering policy. (S4)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/reactor/egress_inject_filter.go::Reactor.exportFilterFromFacts moved. Independent re-judgment against RFC 2918 Section 4 ('Otherwise, the BGP speaker shall re-advertise to that peer the Adj-RIB-Out of the <AFI, SAFI> carried in the message, based on its outbound route filtering policy.') after adding emitPrePolicyAdjRIBOut in Reactor.exportFilterFromFacts (egress_inject_filter.go) for RFC 8671 Section 5.2 and updating doctor-check registration in runRIBPlugin (rib.go, commit 04c75f30c3). In exportFilterFromFacts, the added pre-policy Adj-RIB-Out observation reads facts/body without mutating body, facts, peer, export-policy evaluation (runEgressPolicyChainASN4), withdrawal handling, or the returned (suppress, override, follow) values. In runRIBPlugin, switching OnDoctorCheck to r.doctorCheck and registering two unresolved-next-hop DoctorChecks does not affect refresh dispatch, Adj-RIB-Out replay, or structured event handling. All eight tagged tests (four RIB unit roots, three reactor unit tags, and refresh-config-static.ci) and all other cited producers are byte-identical to the prior enforced audit, so enforced stands with all 31 code bindings preserved.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2918RefreshDoesNotSendToDownPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc2918_config_static_test.go#L91) | unit/verify | revert, verified |
| negative | [`TestHandleRefresh_PeerNotUp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_test.go#L1125) | unit/verify | not verified |
| negative | [`TestRFC2918RefreshRunsCurrentExportPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_replay_feedback_test.go#L69) | unit/verify | revert, verified |
| positive | [`TestRFC2918ConfigStaticRetainedForRefresh`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc2918_config_static_test.go#L19) | unit/verify | revert, verified |
| positive | [`TestHandleRefresh_InternalState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_test.go#L1047) | unit/verify | revert, verified |
| positive | [`TestRFC2918RefreshRunsCurrentExportPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_replay_feedback_test.go#L68) | unit/verify | revert, verified |
| positive | [`TestRefreshSentFeedbackRetainsReplayOrigin`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_replay_feedback_test.go#L32) | unit/verify | revert, verified |
| positive | [`refresh-config-static.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/refresh-config-static.ci#L10) | functional/verify | revert, verified |

### [`RFC2918-3-4`](#rfc2918-3-4)

Should be set to 0 by the sender and ignored by the receiver. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent current-source rejudgment of this advertiser's callback-only migration after reading Main's actual four-root healthy receipt. Git attributes the precise changed lines to ba0f3a3a7d, adapting the MessageCallback signature introduced by a27a951239 (Spec-work: bgp-best-route-advertised-to-peers). The only changed text in the four selected whole test roots is an unused first *Session parameter in the existing observer callback. Every input, callback count, exact wire-byte/error/state assertion, bound and tag remains intact. Session.deliverReceivedMessage passes the actual producing Session before the original arguments; ignoring that added argument leaves the original observer meaningful and does not replace protocol processing. No receiver fixture becomes a claim of BGP plugin authorization, selected output or external peer acceptance. Read RFC2918's own Section3 and all three complete tagged roots plus their actual handshake/recording helpers. Preserve enforced for both halves of the reserved-octet rule. The unchanged sender root drives both sendRouteRefresh and SoftClearPeer and requires exactly four body bytes 00 01 00 01 on its recording connection. Receiver setup actually exchanges OPEN/KEEPALIVE over net.Pipe with capability2 and IPv4 unicast but explicitly no capability70, so RFC7313's subtype semantics cannot confound the Reserved field. Receiver positive varies only Reserved through 00/01/02/AA/FF, requires the exact original body at the observer, one message and refresh callback, Established and a bounded read deadline with no notification. Its negative uses a five-octet body with Reserved AA and requires ErrInvalidMessage plus zero callbacks: ignoring the value must not mean accepting an invalid message. Exact NOTIFICATION code is intentionally not part of this receiver-negative claim. The sole existing canonical producer-break record is sender-positive on RouteRefresh.WriteTo; it was not marked stale by the initial native freshness report. Preserve that historical evidence without claiming fresh receiver discrimination or an unrun mutation. Actual native job-unit-pkg-59a2fc52.log observes TestRFC2918ReservedOctetIgnoredOnReceive and all five value children PASS at7-12, TestRFC2918ReservedOctetDoesNotExemptTheMessage PASS at15, then package PASS at26-27 (3.696s). Main's retained batch custody identifies this as the unmodified race run. All fourteen reviewed source/requirement/proof file pins remain unchanged at receipt-time check. No new producer-break proof is credited by this healthy rerun.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2918ReservedOctetDoesNotExemptTheMessage`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reserved_field_test.go#L188) | unit/verify | not verified |
| positive | [`TestRFC2918SentRequestReservedOctetIsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_b_test.go#L24) | unit/verify | revert, verified |
| positive | [`TestRFC2918ReservedOctetIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reserved_field_test.go#L112) | unit/verify | not verified |

## Requirement list review

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
| `1` | Introduction | 2 | reviewed | Introduction. States the problem: BGP-4 has no way to ask a peer to re-advertise its Adj-RIB-Out, so the common answer is soft-reconfiguration, which keeps an unmodified copy of every route from the peer. Both of its sites describe that problem in indicative prose and are excluded below. The document's own rules start at section 2. |
| `2` | Route Refresh Capability | 0 | reviewed | Route Refresh Capability. Fixes the capability code at 2 and the capability length at 0, and says what advertising the capability conveys to the peer. Every sentence is indicative ('This capability is advertised using the Capability code 2 and Capability length 0'), so no site derives here and the one requirement the section carries is listed unsourced. |
| `3` | Route-REFRESH Message | 0 | reviewed | Route-REFRESH Message. Fixes the message type at 5 and the message body at one 4-byte <AFI, SAFI>, and states the Reserved field's handling. The type line and the field diagram carry no modal at all, and the Reserved field's 'Should be set to 0 by the sender and ignored by the receiver' is not sited here by either scan, so all four requirements read from this section are listed unsourced. |
| `4` | Operation | 2 | reviewed | Operation. The only section that binds a speaker on the wire. Its two sites are the two 'shall' sentences of the receive path and are mapped to RFC2918-4-2 and RFC2918-4-3. The four other requirements read from this section come from its 'may ... only if', 'should advertise' and 'should be one of' sentences, which no scan sites, and are listed unsourced. |
| `5` | Security Considerations | 0 | reviewed | Security Considerations. One sentence: this extension to BGP does not change the underlying security issues. It directs no countermeasure at a speaker. |
| `6` | Acknowledgments | 0 | skipped (acknowledgements) | Acknowledgments. Names IDRP as the source of the Route Refresh concept and thanks four reviewers. |
| `7` | References: RFC 1771, RFC 2858, RFC 2842 | 0 | skipped (references) | References: RFC 1771, RFC 2858, RFC 2842. |
| `8` | Author's Address | 0 | reviewed | Author's Address. Postal address and e-mail for the author. No obligation. |
| `9` | Full Copyright Statement | 1 | reviewed | Full Copyright Statement. The Internet Society boilerplate governing copying and translation of the document. Its one site is a condition on republishing the text and is excluded below. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A description of the problem, in the Introduction, written in indicative prose. The 'must' states what necessarily follows from a policy change, that the prefixes have to be available again to be re-examined, and it is the motivation for the document rather than a rule the document adds. It names no message, field or timer for a speaker to get right. | When the inbound routing policy for a peer changes, all prefixes from that peer must be somehow made available and then re- examined against the new policy. |
| `1:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A description of another approach, in the Introduction. The subject is soft-reconfiguration, which the sentence before it defines as storing an unmodified copy of all routes from the peer. 'Are required' reports the cost of that approach, which is what motivates this document; it directs nobody. | Additional memory and CPU are required to maintain these routes. |
| `9:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Internet Society copyright boilerplate that the site scanner did not strip. Its 'must' governs the copyright procedures to be followed when the document text is republished or translated, and it binds a publisher of the document rather than an implementation of the protocol the document specifies. | However, this document itself may not be modified in any way, such as by removing the copyright notice or references to the Internet Society or other Internet organizations, except as needed for the purpose of developing Internet standards in which case the procedures for copyrights defined in the Internet Standards process must be followed, or as required to translate it into languages other than English. |

## Superseded

No document obsoletes RFC 2918, so its obligations are stated where they were written.
