# RFC 2545 - Use of BGP-4 Multiprotocol Extensions for IPv6 Inter-Domain Routing

Supported. Every requirement this repository extracted from RFC 2545, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 4 of 4 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 4 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 4 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 4 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 28 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 6 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 4 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 6 |
| Gated MUST-level | 4 |
| Obligations that bind Ze | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 28 |
| Tagged units | 28 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc2545.md` |
| Requirement shard | `rfc/requirements/rfc2545.md` |
| RFC text | `rfc/full/rfc2545.txt` |

## Enrolment

Enrolled: Use of BGP-4 Multiprotocol Extensions for IPv6 Inter-Domain Routing: four SHALL-level requirements, all in Section 3 and all about the MP_REACH_NLRI Network Address of Next Hop field. Each is proven in both polarities by the exabgp-compat encoding suite (verify tier, ./le functional exabgp-test): conf-llnh-update.ci pins the 32-octet form (global ::1 followed by link-local fe80::1, Length octet 0x20) and conf-llnh-lla-only.ci pins the 16-octet form (one address, Length octet 0x10). The two configs differ in ONE variable, the route's next hop: both declare the same local-link-local fe80::1, both enable the link-local-nexthop capability, and both peer with ::1. So the pair binds inclusion to the shared-subnet condition of Section 3 rather than to the leaf being set: ::1 shares the loopback subnet with the speaker, 2001:db8::ffff shares none. The pair discriminates: an encoder that always appended a second address would fail the second test, and one that never appended would fail the first. Section 2 binds network administrators and restates [ND]/[RIP], Section 4 restates the [BGP-4] BGP Identifier; those sites are classified in rfc/extraction/rfc2545.json. Enrolled 2026-08-07.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered**

- Enrolled 2026-08-07. All four SHALL-level requirements sit in Section 3 and govern the MP_REACH_NLRI Network Address of Next Hop field
- each is proven in both polarities by verify-tier functional tests, bound per line in [`rfc/short/rfc2545.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc2545.md) and listed in [`rfc/requirements/rfc2545.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc2545.md). Send side: ze evaluates the Section 3 condition itself. `linkScope.linkLocalNextHop` ([`internal/component/bgp/reactor/link_scope.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/link_scope.go)) emits the 32-octet form (global address first, link-local second, length octet 0x20) only when the speaker shares a locally connected subnet with the peer AND with the entity named by the global next hop, and the 16-octet form (one address, length octet 0x10) in every other case. Connected subnets come from `network.ConnectedPrefixes` ([`internal/core/network/connected.go`](https://github.com/ze-software/ze/blob/main/internal/core/network/connected.go)), snapshotted per session. The `session > link-local` leaf supplies the address
- it does not decide inclusion. The snapshot is re-settled for every established peer when an address is added to or removed from ANY interface, `refreshPeerLinkScopes` ([`internal/component/bgp/reactor/reactor_iface.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_iface.go)), so a change on an interface other than the session's does not leave a stale answer behind a surviving TCP session. A link-local address is refused wherever it could reach the FIRST slot of that field: the three route-level entry points (`ParseRouteAttributes`, `handleAnnounceUnicast`, `parseNhopFlat`) and the two peer config leaves `connection > local > ip` and `session > next-hop` (`ValidatePeerGlobalNextHop`, [`internal/component/bgp/reactor/config_nexthop_form.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_nexthop_form.go)) all call `ValidateGlobalNextHop` ([`internal/core/bgp/attribute/nexthop_form.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/nexthop_form.go)), and `linkScope.linkLocalNextHop` refuses independently of all of them, appending nothing when the global address it is given is not a global IPv6 address. Receive side: `parseNextHops` ([`internal/core/bgp/attribute/mpnlri.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri.go)) accepts lengths 16 and 32 for an IPv6 next hop and rejects every other length for AFI 2.


**What the ledger says remains**

No tracked gap in current source anchors. Sections 2 and 4 bind a network administrator, an IPv6 router's neighbor-discovery behavior, and the operator who configures a session; those sites are classified in [`rfc/extraction/rfc2545.json`](https://github.com/ze-software/ze/blob/main/rfc/extraction/rfc2545.json) rather than gated here. The BGP Identifier uniqueness that Section 4 restates is gated under RFC 6286. Capability code 77 is draft-based; RFC 2545 covers the next-hop wire behavior.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC2545-3-1`](#rfc2545-3-1), [`RFC2545-3-2`](#rfc2545-3-2), [`RFC2545-3-3`](#rfc2545-3-3), [`RFC2545-3-4`](#rfc2545-3-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2545-3-1` | "A BGP speaker shall advertise to its peer in the Network Address of Next Hop field the global IPv6 address of the next hop, potentially followed by the link-local IPv6 address of the next hop" (§3) | SHALL | 3 - Constructing the Next Hop field | **positive:** `unit/verify` [`TestDefaultOriginateAppendsLinkLocalWhenSection3Holds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L666). **positive:** `unit/verify` [`TestSendAnnounceAppendsLinkLocalWhenSection3Holds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_send_test.go#L136). **positive:** `unit/verify` [`TestSessionSendAnnounceAcceptsLinkLocalSecondAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/announce_nexthop_guard_test.go#L133). **negative:** `unit/verify` [`TestSendAnnounceRefusesUnusableIPv6NextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/announce_nexthop_guard_test.go#L67). **negative:** `unit/verify` [`TestSessionSendAnnounceRefusesNonLinkLocalSecondAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/announce_nexthop_guard_test.go#L105). **positive:** `functional/verify` [`adj-rib-in-replay-rfc2545-next-hop.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/adj-rib-in-replay-rfc2545-next-hop.ci#L17). **positive:** `functional/verify` [`conf-llnh-update.ci`](https://github.com/ze-software/ze/blob/main/test/exabgp-compat/encoding/conf-llnh-update.ci#L7). **negative:** `functional/verify` [`new-v6.ci`](https://github.com/ze-software/ze/blob/main/test/encode/new-v6.ci#L1) |
| `RFC2545-3-2` | "The value of the Length of Next Hop Network Address field on a MP_REACH_NLRI attribute shall be set to 16, when only a global address is present, or 32 if a link-local address is also included in the Next Hop field" (§3) | SHALL | 3 - Constructing the Next Hop field | **positive:** `unit/verify` [`TestDefaultOriginateAppendsLinkLocalWhenSection3Holds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L670). **positive:** `unit/verify` [`TestSendAnnounceAppendsLinkLocalWhenSection3Holds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_send_test.go#L140). **positive:** `unit/verify` [`TestSessionSendAnnounceAcceptsLinkLocalSecondAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/announce_nexthop_guard_test.go#L136). **negative:** `unit/verify` [`TestSessionSendAnnounceRefusesNonLinkLocalSecondAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/announce_nexthop_guard_test.go#L108). **positive:** `functional/verify` [`adj-rib-in-replay-rfc2545-next-hop.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/adj-rib-in-replay-rfc2545-next-hop.ci#L21). **positive:** `functional/verify` [`conf-llnh-update.ci`](https://github.com/ze-software/ze/blob/main/test/exabgp-compat/encoding/conf-llnh-update.ci#L13). **negative:** `functional/verify` [`new-v6.ci`](https://github.com/ze-software/ze/blob/main/test/encode/new-v6.ci#L8). **positive:** `interop/nightly` [`checkRFC2545NextHops`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L519) |
| `RFC2545-3-3` | "The link-local address shall be included in the Next Hop field if and only if the BGP speaker shares a common subnet with the entity identified by the global IPv6 address carried in the Network Address of Next Hop field and the peer the route is being advertised to" (§3) | SHALL | 3 - Constructing the Next Hop field | **positive:** `unit/verify` [`TestDefaultOriginateAppendsLinkLocalWhenSection3Holds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L673). **positive:** `unit/verify` [`TestSendAnnounceAppendsLinkLocalWhenSection3Holds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_send_test.go#L143). **negative:** `unit/verify` [`TestDefaultOriginateOmitsLinkLocalWhenPeerOffLink`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L698). **negative:** `unit/verify` [`TestSendAnnounceOmitsLinkLocalWhenPeerOffLink`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_send_test.go#L176). **positive:** `functional/verify` [`conf-llnh-update.ci`](https://github.com/ze-software/ze/blob/main/test/exabgp-compat/encoding/conf-llnh-update.ci#L16). **negative:** `functional/verify` [`conf-llnh-lla-only.ci`](https://github.com/ze-software/ze/blob/main/test/exabgp-compat/encoding/conf-llnh-lla-only.ci#L7). **positive:** `interop/nightly` [`checkRFC2545NextHops`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L525). **negative:** `interop/nightly` [`checkRFC2545NextHops`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L531) |
| `RFC2545-3-4` | "In all other cases a BGP speaker shall advertise to its peer in the Network Address field only the global IPv6 address of the next hop (the value of the Length of Network Address of Next Hop field shall be set to 16)" (§3) | SHALL | 3 - Constructing the Next Hop field | **positive:** `unit/verify` [`TestDefaultOriginateOmitsLinkLocalWhenPeerOffLink`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L702). **positive:** `unit/verify` [`TestSendAnnounceOmitsLinkLocalWhenPeerOffLink`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_send_test.go#L180). **positive:** `functional/verify` [`new-v6.ci`](https://github.com/ze-software/ze/blob/main/test/encode/new-v6.ci#L11). **negative:** `functional/verify` [`conf-llnh-update.ci`](https://github.com/ze-software/ze/blob/main/test/exabgp-compat/encoding/conf-llnh-update.ci#L22) |
| `RFC2545-4-1` | The BGP Identifier "should be derived from an IPv4 address regardless of the network protocol(s) a particular BGP-4 instance is configured to convey at a given moment" (§4) | SHOULD | 4 - Transport | **positive:** no positive test. **negative:** no negative test |
| `RFC2545-3-5` | "a BGP speaker that advertises a route to an internal peer may modify the Network Address of Next Hop field by removing the link-local IPv6 address of the next hop" (§3) | MAY | 3 - Constructing the Next Hop field | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 2545 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2545-3-1`](#rfc2545-3-1)

"A BGP speaker shall advertise to its peer in the Network Address of Next Hop field the global IPv6 address of the next hop, potentially followed by the link-local IPv6 address of the next hop" (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSendAnnounceRefusesUnusableIPv6NextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/announce_nexthop_guard_test.go#L67) | unit/verify | unproven |
| negative | [`TestSessionSendAnnounceRefusesNonLinkLocalSecondAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/announce_nexthop_guard_test.go#L105) | unit/verify | unproven |
| negative | [`new-v6.ci`](https://github.com/ze-software/ze/blob/main/test/encode/new-v6.ci#L1) | functional/verify | unproven |
| positive | [`TestSessionSendAnnounceAcceptsLinkLocalSecondAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/announce_nexthop_guard_test.go#L133) | unit/verify | unproven |
| positive | [`TestDefaultOriginateAppendsLinkLocalWhenSection3Holds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L666) | unit/verify | unproven |
| positive | [`TestSendAnnounceAppendsLinkLocalWhenSection3Holds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_send_test.go#L136) | unit/verify | unproven |
| positive | [`conf-llnh-update.ci`](https://github.com/ze-software/ze/blob/main/test/exabgp-compat/encoding/conf-llnh-update.ci#L7) | functional/verify | unproven |
| positive | [`adj-rib-in-replay-rfc2545-next-hop.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/adj-rib-in-replay-rfc2545-next-hop.ci#L17) | functional/verify | unproven |

### [`RFC2545-3-2`](#rfc2545-3-2)

"The value of the Length of Next Hop Network Address field on a MP_REACH_NLRI attribute shall be set to 16, when only a global address is present, or 32 if a link-local address is also included in the Next Hop field" (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSessionSendAnnounceRefusesNonLinkLocalSecondAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/announce_nexthop_guard_test.go#L108) | unit/verify | unproven |
| negative | [`new-v6.ci`](https://github.com/ze-software/ze/blob/main/test/encode/new-v6.ci#L8) | functional/verify | unproven |
| positive | [`TestSessionSendAnnounceAcceptsLinkLocalSecondAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/announce_nexthop_guard_test.go#L136) | unit/verify | unproven |
| positive | [`TestDefaultOriginateAppendsLinkLocalWhenSection3Holds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L670) | unit/verify | unproven |
| positive | [`TestSendAnnounceAppendsLinkLocalWhenSection3Holds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_send_test.go#L140) | unit/verify | unproven |
| positive | [`checkRFC2545NextHops`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L519) | interop/nightly | unproven |
| positive | [`conf-llnh-update.ci`](https://github.com/ze-software/ze/blob/main/test/exabgp-compat/encoding/conf-llnh-update.ci#L13) | functional/verify | unproven |
| positive | [`adj-rib-in-replay-rfc2545-next-hop.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/adj-rib-in-replay-rfc2545-next-hop.ci#L21) | functional/verify | unproven |

### [`RFC2545-3-3`](#rfc2545-3-3)

"The link-local address shall be included in the Next Hop field if and only if the BGP speaker shares a common subnet with the entity identified by the global IPv6 address carried in the Network Address of Next Hop field and the peer the route is being advertised to" (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDefaultOriginateOmitsLinkLocalWhenPeerOffLink`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L698) | unit/verify | unproven |
| negative | [`TestSendAnnounceOmitsLinkLocalWhenPeerOffLink`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_send_test.go#L176) | unit/verify | unproven |
| negative | [`checkRFC2545NextHops`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L531) | interop/nightly | unproven |
| negative | [`conf-llnh-lla-only.ci`](https://github.com/ze-software/ze/blob/main/test/exabgp-compat/encoding/conf-llnh-lla-only.ci#L7) | functional/verify | unproven |
| positive | [`TestDefaultOriginateAppendsLinkLocalWhenSection3Holds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L673) | unit/verify | unproven |
| positive | [`TestSendAnnounceAppendsLinkLocalWhenSection3Holds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_send_test.go#L143) | unit/verify | unproven |
| positive | [`checkRFC2545NextHops`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L525) | interop/nightly | unproven |
| positive | [`conf-llnh-update.ci`](https://github.com/ze-software/ze/blob/main/test/exabgp-compat/encoding/conf-llnh-update.ci#L16) | functional/verify | unproven |

### [`RFC2545-3-4`](#rfc2545-3-4)

"In all other cases a BGP speaker shall advertise to its peer in the Network Address field only the global IPv6 address of the next hop (the value of the Length of Network Address of Next Hop field shall be set to 16)" (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`conf-llnh-update.ci`](https://github.com/ze-software/ze/blob/main/test/exabgp-compat/encoding/conf-llnh-update.ci#L22) | functional/verify | unproven |
| positive | [`TestDefaultOriginateOmitsLinkLocalWhenPeerOffLink`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_initial_sync_test.go#L702) | unit/verify | unproven |
| positive | [`TestSendAnnounceOmitsLinkLocalWhenPeerOffLink`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_send_test.go#L180) | unit/verify | unproven |
| positive | [`new-v6.ci`](https://github.com/ze-software/ze/blob/main/test/encode/new-v6.ci#L11) | functional/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-rfc phase agent, deferral fixit-stored-route-relay-hardening (rfc2545 enrolment) |
| Signed off | 2026-08-07 |
| Register | prose |
| Source | rfc/full/rfc2545.txt |
| Source fingerprint | 80e7f63fb7ad995a |
| Record | rfc/extraction/rfc2545.json |
| Mapped sentences | 5 |
| Declined as scope | 5 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of this Memo, copyright notice and Abstract. The Abstract states what the document defines and binds no speaker. |
| `1` | Introduction | 1 | walked | Introduction. States that the BGP-4 procedures apply unchanged unless this document says otherwise, and that the document concerns itself with IPv6 address scope. Its one site describes what the IPv6 addressing architecture defines and is excluded below. |
| `2` | IPv6 Address Scopes | 2 | walked | IPv6 Address Scopes. Defines the terms 'global' and 'non-link-local' for this document, and explains why a Next Hop field sometimes carries two addresses. It states no rule for building that field; every such rule is in section 3. Its two sites bind a network administrator and an IPv6 router's neighbour-discovery behaviour, and both are excluded below. |
| `3` | Constructing the Next Hop field | 4 | walked | Constructing the Next Hop field. The only section that binds a BGP speaker. All four sites are mapped, one per requirement, to RFC2545-3-1 through RFC2545-3-4. The section's fifth sentence is the MAY captured as RFC2545-3-5; 'may' is not a derived site under either scan, which is why the site count is four rather than five. |
| `4` | Transport | 2 | walked | Transport. Explains that BGP-4 runs over IPv4 or IPv6 and reads implicit configuration from the session address, then separates that address from the BGP Identifier. Site 4:1 binds the operator who configures the session. Site 4:2 is the source of RFC2545-4-1 and is mapped to it. |
| `5` | Security Considerations | 0 | walked | Security Considerations. One sentence: carrying IPv6 reachability raises no new security issue beyond those of BGP-4 with IPv4. No countermeasure is directed at a speaker. |
| `6` | Acknowledgments | 0 | skipped (acknowledgements) | Acknowledgments. Credits the authors of the BGP-4 Multiprotocol Extensions work. |
| `7` | not stated | 0 | skipped (references) | References: RFC 2373, RFC 1771, RFC 2283, RFC 2460, RFC 2461, RFC 2080. |
| `8` | Author Information | 0 | walked | Author Information. Postal addresses, telephone numbers and e-mail addresses for the two authors. No obligation. |
| `9` | Full Copyright Statement | 1 | walked | Full Copyright Statement. The Internet Society boilerplate governing copying and translation of the document. Its one site is a condition on republishing the text and is excluded below. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A description of another document, in the Introduction. The subject of 'must' is IPv6 itself: the sentence reports that the IPv6 addressing architecture [ADDR-ARCH] defines situations calling for a given scope. It directs no BGP speaker and states no rule this document adds. The rules this document does add begin at section 3. | In terms of routing information, the most significant difference between IPv6 and IPv4 (for which BGP was originally designed) is the fact that IPv6 introduces scoped unicast addresses and defines particular situations when a particular address scope must be used. |
| `2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the network administrator, named as the subject of the sentence. The obligation is to respect address scope restrictions when planning a deployment, and the awareness clause is about how a person reasons about routing domains against sites. Neither is an act a BGP implementation performs; the sentence exists because section 2 has just declared that the document itself makes no distinction between global and site-local addresses, and it hands the distinction back to the operator. The role is the AS operator, so no producer could act as it. Ze CONSUMES the operator's decision: the capability is negotiated per session in the reactor (`internal/component/bgp/reactor`), which advertises what it is configured to and decides no AS-wide policy. | Network administrators must however respect address scope restrictions and should be aware that the concepts of a BGP-4 routing domain and "site" are orthogonal notions and that they may or may not coincide in a given situation. |
| `2:2` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The sentence opens with 'This restrictions does imply', referring to the restrictions of the two documents cited immediately above it: [ND] (RFC 2461), which permits only a link-local address when generating ICMP Redirect messages, and [RIP] (RFC 2080), which permits only a link-local address as a RIPng next hop. The obligation to hold a link-local next hop for a directly connected route is theirs, and it binds the IPv6 neighbour-discovery and interior-routing layers rather than the BGP-4 Next Hop field. RFC 2545 draws the consequence to motivate section 3, and adds no obligation of its own here. | This restrictions does imply that an IPv6 router must have a link- local next hop address for all directly connected routes (routes for which the given router and the next hop router share a common subnet prefix). |
| `4:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the operator who configures the session. The sentence draws a consequence of the two before it: BGP-4 reads the peer's network address implicitly from the address that established the session, and the route dissemination procedure uses it, so an IPv4 transport carrying IPv6 reachability leaves that address undetermined and somebody has to supply it. What is required is a configuration act, not a wire behaviour or a decision procedure, and the sentence names no message, field or timer for a speaker to get right. The role is the AS operator, so no producer could act as it. Ze CONSUMES the operator's decision: the capability is negotiated per session in the reactor (`internal/component/bgp/reactor`), which advertises what it is configured to and decides no AS-wide policy. | Thus, when using TCP over IPv4 as a transport for IPv6 reachability information, additional explicit configuration of the peer's network address is required. |
| `9:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Internet Society copyright boilerplate that the site scanner did not strip. The 'must' governs the procedures for republishing or translating the document text, and binds a publisher of the document rather than an implementation of the protocol it specifies. | However, this document itself may not be modified in any way, such as by removing the copyright notice or references to the Internet Society or other Internet organizations, except as needed for the purpose of developing Internet standards in which case the procedures for copyrights defined in the Internet Standards process must be followed, or as required to translate it into languages other than English. |

## Superseded

No document obsoletes RFC 2545, so its obligations are stated where they were written.
