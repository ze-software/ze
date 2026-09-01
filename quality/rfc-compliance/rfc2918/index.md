# RFC 2918 - Route Refresh Capability for BGP-4

Supported. Every requirement this repository extracted from RFC 2918, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 6 of 6 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 6 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 6 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 6 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 17 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 6 | of 11 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 6 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 6 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 11 |
| Gated MUST-level | 6 |
| Obligations that bind Ze | 6 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 17 |
| Tagged units | 17 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
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
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **6** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (6):** [`RFC2918-2-1`](#rfc2918-2-1), [`RFC2918-3-1`](#rfc2918-3-1), [`RFC2918-3-2`](#rfc2918-3-2), [`RFC2918-4-1`](#rfc2918-4-1), [`RFC2918-4-2`](#rfc2918-4-2), [`RFC2918-4-3`](#rfc2918-4-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2918-2-1` | Capability code MUST be 2, capability length MUST be 0 (S2) | MUST | 2 - Route Refresh Capability | **positive:** `unit/verify` [`TestCapabilityWriteTo`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L650). **negative:** `unit/verify` [`TestOpenRejectsMalformedKnownCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L87) |
| `RFC2918-3-1` | ROUTE-REFRESH message type MUST be 5 (S3) | MUST | 3 - Route-REFRESH Message | **positive:** `unit/verify` [`TestRouteRefreshType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L16). **negative:** `unit/verify` [`TestParseHeaderAllTypes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L39) |
| `RFC2918-3-2` | ROUTE-REFRESH payload MUST be exactly 4 bytes (AFI 2 + Reserved 1 + SAFI 1) (S3) | MUST | 3 - Route-REFRESH Message | **positive:** `unit/verify` [`TestRouteRefreshPack`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L31). **negative:** `unit/verify` [`TestHandleRouteRefresh_InvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L381). **negative:** `unit/verify` [`TestRouteRefreshUnpackShort`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L75) |
| `RFC2918-4-1` | A BGP speaker MUST NOT send a ROUTE-REFRESH message to a peer unless it has received the Route Refresh Capability from that peer (S4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestRFC2918SendRouteRefreshToCapablePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_route_refresh_test.go#L94). **positive:** `unit/verify` [`TestRFC2918SoftClearPeerSendsRefreshToCapablePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_route_refresh_test.go#L158). **negative:** `unit/verify` [`TestRFC2918SendRouteRefreshSkipsPeerWithoutCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_route_refresh_test.go#L115). **negative:** `unit/verify` [`TestRFC2918SoftClearPeerSkipsPeerWithoutCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_route_refresh_test.go#L173) |
| `RFC2918-4-2` | If a ROUTE-REFRESH is received with an AFI/SAFI not advertised by the receiver at session establishment, the receiver SHALL ignore the message (S4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestHandleRouteRefresh_NonNegotiatedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L466). **negative:** `unit/verify` [`TestRouteRefreshValidLengthDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2418) |
| `RFC2918-4-3` | Otherwise, the receiver SHALL re-advertise the Adj-RIB-Out of the requested AFI/SAFI based on its outbound route filtering policy (S4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestHandleRefresh_InternalState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_test.go#L986). **negative:** `unit/verify` [`TestHandleRefresh_PeerNotUp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_test.go#L1029) |
| `RFC2918-4-4` | A BGP speaker willing to receive ROUTE-REFRESH SHOULD advertise the Route Refresh Capability to the peer (S4) | SHOULD | 4 - Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC2918-4-5` | The AFI/SAFI in a ROUTE-REFRESH SHOULD be one the peer advertised at session establishment (S4) | SHOULD | 4 - Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC2918-3-3` | Reserved field SHOULD be set to 0 by the sender (S3) | SHOULD | 3 - Route-REFRESH Message | **positive:** no positive test. **negative:** no negative test |
| `RFC2918-4-6` | A BGP speaker MAY send a ROUTE-REFRESH message to its peer (sending is optional) (S4) | MAY | 4 - Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC2918-3-4` | Reserved field SHOULD be ignored by the receiver (even if non-zero) (S3) | SHOULD | 3 - Route-REFRESH Message | **positive:** `unit/verify` [`TestRFC2918ReservedOctetIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reserved_field_test.go#L112). **negative:** `unit/verify` [`TestRFC2918ReservedOctetDoesNotExemptTheMessage`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reserved_field_test.go#L188) |

## Gaps and untested MUSTs

RFC 2918 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2918-2-1`](#rfc2918-2-1)

Capability code MUST be 2, capability length MUST be 0 (S2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpenRejectsMalformedKnownCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L87) | unit/verify | unproven |
| positive | [`TestCapabilityWriteTo`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L650) | unit/verify | unproven |

### [`RFC2918-3-1`](#rfc2918-3-1)

ROUTE-REFRESH message type MUST be 5 (S3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseHeaderAllTypes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L39) | unit/verify | unproven |
| positive | [`TestRouteRefreshType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L16) | unit/verify | unproven |

### [`RFC2918-3-2`](#rfc2918-3-2)

ROUTE-REFRESH payload MUST be exactly 4 bytes (AFI 2 + Reserved 1 + SAFI 1) (S3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRouteRefreshUnpackShort`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L75) | unit/verify | unproven |
| negative | [`TestHandleRouteRefresh_InvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L381) | unit/verify | unproven |
| positive | [`TestRouteRefreshPack`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/routerefresh_test.go#L31) | unit/verify | unproven |

### [`RFC2918-4-1`](#rfc2918-4-1)

A BGP speaker MUST NOT send a ROUTE-REFRESH message to a peer unless it has received the Route Refresh Capability from that peer (S4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2918SendRouteRefreshSkipsPeerWithoutCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_route_refresh_test.go#L115) | unit/verify | unproven |
| negative | [`TestRFC2918SoftClearPeerSkipsPeerWithoutCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_route_refresh_test.go#L173) | unit/verify | unproven |
| positive | [`TestRFC2918SendRouteRefreshToCapablePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_route_refresh_test.go#L94) | unit/verify | unproven |
| positive | [`TestRFC2918SoftClearPeerSendsRefreshToCapablePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reactor_route_refresh_test.go#L158) | unit/verify | unproven |

### [`RFC2918-4-2`](#rfc2918-4-2)

If a ROUTE-REFRESH is received with an AFI/SAFI not advertised by the receiver at session establishment, the receiver SHALL ignore the message (S4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRouteRefreshValidLengthDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2418) | unit/verify | unproven |
| positive | [`TestHandleRouteRefresh_NonNegotiatedFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L466) | unit/verify | unproven |

### [`RFC2918-4-3`](#rfc2918-4-3)

Otherwise, the receiver SHALL re-advertise the Adj-RIB-Out of the requested AFI/SAFI based on its outbound route filtering policy (S4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestHandleRefresh_PeerNotUp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_test.go#L1029) | unit/verify | unproven |
| positive | [`TestHandleRefresh_InternalState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_test.go#L986) | unit/verify | unproven |

### [`RFC2918-3-4`](#rfc2918-3-4)

Reserved field SHOULD be ignored by the receiver (even if non-zero) (S3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2918ReservedOctetDoesNotExemptTheMessage`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2918_reserved_field_test.go#L188) | unit/verify | unproven |
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
