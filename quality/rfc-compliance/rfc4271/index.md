# RFC 4271 - A Border Gateway Protocol 4 (BGP-4)

Partial. Every requirement this repository extracted from RFC 4271, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 80.6% | 75 of 93 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 3.2% | 3 of 93 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 93 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 230 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 100 | of 134 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 7 | of 100 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 16.1% | 15 of 93 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 93 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 134 |
| Gated MUST-level | 100 |
| Obligations that bind Ze | 93 |
| Not applicable, so out of scope | 7 |
| Declared gaps | 15 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 230 |
| Tagged units | 230 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc4271.md` |
| Requirement shard | `rfc/requirements/rfc4271.md` |
| RFC text | `rfc/full/rfc4271.txt` |

## Enrolment

Enrolled: A Border Gateway Protocol 4 (BGP-4): Section 5.1.4 requires a local-configuration mechanism that removes MULTI_EXIT_DISC from a route. Ze exposes the full obligation as `modify NAME { del { med; } }` on an import chain, before Decision Process phases 1 and 2.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- FSM, OPEN/UPDATE/NOTIFICATION/KEEPALIVE encode and decode, message-header and hold-time validation, well-known attribute recognition and flag rules, connection collision resolution, per-peer FSM and hold timer, the complete Section 8.2.2 Event 10 action list on a hold-timer expiry, which is the Hold Timer Expired NOTIFICATION sent before the connection is dropped ([`RFC4271-8.2.2-1`](#rfc4271-8.2.2-1)), the ConnectRetryTimer zeroed ([`RFC4271-8.2.2-2`](#rfc4271-8.2.2-2)), the BGP resources released ([`RFC4271-8.2.2-3`](#rfc4271-8.2.2-3)), the TCP connection dropped ([`RFC4271-8.2.2-4`](#rfc4271-8.2.2-4)) and the state changed to Idle ([`RFC4271-8.2.2-5`](#rfc4271-8.2.2-5)), all on the FIRST expiry with no reprieve
- the Section 8.2.2 ManualStop (Event 2) action list, which is the Cease NOTIFICATION sent before the connection is dropped, with RFC 4486 subcode 2 Administrative Shutdown, on every peer an administrative stop of the daemon ends a connection with, from OpenSent and OpenConfirm as well as Established ([`RFC4271-8.2.2-18`](#rfc4271-8.2.2-18), [`internal/component/bgp/reactor/reactor.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor.go) `Stop`)
- prefix-limit Cease, TCP MD5, the Adj-RIB-In, the RFC 4271 Section 9.1.2.2 decision process and the Loc-RIB install
- the Section 5.1.4 propagation rule, which keeps a MULTI_EXIT_DISC received from one neighboring AS off every session toward another ([`RFC4271-5.1.4-1`](#rfc4271-5.1.4-1), `applyFactsMED`, [`internal/component/bgp/reactor/forward_med.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med.go)) while a metric ze or an egress filter originates still reaches the peer, and while an RS client keeps the value RFC 7947 Section 2.2.3 exempts
- the Section 5.1.4 configured removal, which is the mechanism a speaker MUST implement ([`RFC4271-5.1.4-4`](#rfc4271-5.1.4-4)): the `del { med; }` directive of a modify policy, on a policy attached to a peer's IMPORT chain drops MULTI_EXIT_DISC from the route, and it drops it before Decision Process phases 1 and 2 as [`RFC4271-5.1.4-2`](#rfc4271-5.1.4-2) requires, because the import chain's rewritten payload replaces the WireUpdate before the UPDATE is dispatched (`ExtractMEDRemoveOps`, [`internal/component/bgp/reactor/filter_delta.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/filter_delta.go); `appendMEDRemove`, [`internal/component/bgp/plugins/filter_modify/filter_modify.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/filter_modify.go), which refuses the directive on an export chain)
- the Section 5 and Section 9 pass-along rule for an attribute ze does not recognize, which sets the Partial bit to 1 on an unrecognized transitive optional attribute at receipt, on the bytes ze retains and relays ([`RFC4271-5-3`](#rfc4271-5-3), `publishBase`, [`internal/component/bgp/reactor/session_validation.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation.go), and `SetPartialOnUnrecognizedTransitive`, [`internal/core/bgp/attribute/partial.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/partial.go)), while a well-known attribute, an optional non-transitive one and an attribute ze does recognize keep the bit the sender chose, and a bit an earlier AS set is never cleared
- the Section 5.1.3 NEXT_HOP loop rules, which are the two halves of one hazard: on egress a route is withheld from the peer whose OWN address the NEXT_HOP names, whether ze RELAYS that route or ORIGINATES it ([`RFC4271-5.1.3-1`](#rfc4271-5.1.3-1)). A relayed route is answered on both forward rails by `egressNextHopIsPeerOwn` ([`internal/component/bgp/reactor/forward_next_hop.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop.go)), where the address arrives as the third-party next hop Section 5.1.3 case 2 permits. An originated route is answered by `originatedNextHopIsPeerOwn` in the same file, asked at the two writers that put such a route on the wire, `writeUpdateGated` and `SendAnnounce` ([`internal/component/bgp/reactor/session_write.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_write.go)), rather than at each of the five rails that produce one: configured static routes and default-originate, the RIB op-queue drain, the announce batch and the RFC 9494 stale re-advertise. On both sides the withdrawals travelling in the same UPDATE still reach that peer, a third-party next hop naming anyone else is still advertised, and the route is WITHHELD rather than rewritten, because the section states a prohibition on advertising and a rewrite would invent a next hop the operator never configured
- and on install a route naming one of ze's OWN session addresses is excluded from the decision process rather than installed ([`RFC4271-5.1.3-2`](#rfc4271-5.1.3-2), `gatherCandidatesLocked`, [`internal/component/bgp/plugins/rib/rib_commands.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_commands.go)), so a sound alternative path to the same prefix still wins
- requirements bound per line in [`rfc/short/rfc4271.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc4271.md).


**What the ledger says remains**

Fifteen MUST/SHALL-level gaps, each annotated in [`rfc/short/rfc4271.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc4271.md).

- **Header error reporting:** [`RFC4271-6.1-1`](#rfc4271-6.1-1), 6.1-2 and 6.1-3 (a bad marker or a sub-19 length is detected but no NOTIFICATION is sent, [`internal/component/bgp/message/header.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header.go) and [`internal/component/bgp/reactor/session_read.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_read.go); the per-type minima and the ceiling do send a conformant Bad Message Length, session_read.go) and 6.1-4 (an unknown message type is reported with subcode 0 and a text string, not Bad Message Type with the erroneous Type octet, [`internal/component/bgp/reactor/session_handlers.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers.go)).
- **OPEN error reporting:** [`RFC4271-6.2-3`](#rfc4271-6.2-3) (an OPEN body that fails to decode returns from handleOpen with no NOTIFICATION and without closing the connection, [`internal/component/bgp/reactor/session_handlers.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers.go); every other OPEN error rail does send Error Code 2).
- **Next-hop resolvability:** [`RFC4271-3.1-2`](#rfc4271-3.1-2), 9.1.2-1, 9.1.2-4 and 9.1.2.1-2 (the decision process neither excludes an unresolvable NEXT_HOP nor re-runs on an IGP-cost change, and the Loc-RIB is not purged of unresolvable routes, [`internal/component/bgp/plugins/rib/rib_commands.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_commands.go)).
- **Adj-RIB-Out:** [`RFC4271-9.2-2`](#rfc4271-9.2-2) (no forwardability gate) and 9.2-3 (a route excluded by an egress filter is skipped without withdrawing the previous advertisement, [`internal/component/bgp/reactor/forward_rs.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rs.go)).
- **Timers:** [`RFC4271-9.2.1.1-2`](#rfc4271-9.2.1.1-2) and 9.2.1.1-3 (no MinRouteAdvertisementIntervalTimer, [`internal/component/bgp/fsm/timer.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/timer.go)).
- **Connections:** [`RFC4271-8.2.1-3`](#rfc4271-8.2.1-3) (an inbound connection reuses the peer's session rather than getting its own FSM, [`internal/component/bgp/reactor/reactor_connection.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_connection.go)). Seven further requirements are recorded {not-applicable}: ze performs no route aggregation and never disaggregates a received route.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 75 | one part of the gated population |
| Annotated instead of tested | 25 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **100** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (75):** [`RFC4271-4.1-1`](#rfc4271-4.1-1), [`RFC4271-4.1-2`](#rfc4271-4.1-2), [`RFC4271-4.1-3`](#rfc4271-4.1-3), [`RFC4271-4.3-1`](#rfc4271-4.3-1), [`RFC4271-4.3-2`](#rfc4271-4.3-2), [`RFC4271-4.3-4`](#rfc4271-4.3-4), [`RFC4271-4.4-1`](#rfc4271-4.4-1), [`RFC4271-4.4-2`](#rfc4271-4.4-2), [`RFC4271-6-1`](#rfc4271-6-1), [`RFC4271-4.2-1`](#rfc4271-4.2-1), [`RFC4271-4.2-2`](#rfc4271-4.2-2), [`RFC4271-6.2-1`](#rfc4271-6.2-1), [`RFC4271-6.2-2`](#rfc4271-6.2-2), [`RFC4271-5-1`](#rfc4271-5-1), [`RFC4271-5-2`](#rfc4271-5-2), [`RFC4271-5-3`](#rfc4271-5-3), [`RFC4271-5-4`](#rfc4271-5-4), [`RFC4271-5-5`](#rfc4271-5-5), [`RFC4271-5-6`](#rfc4271-5-6), [`RFC4271-5.1.3-1`](#rfc4271-5.1.3-1), [`RFC4271-5.1.3-2`](#rfc4271-5.1.3-2), [`RFC4271-5.1.3-3`](#rfc4271-5.1.3-3), [`RFC4271-5.1.4-1`](#rfc4271-5.1.4-1), [`RFC4271-5.1.4-4`](#rfc4271-5.1.4-4), [`RFC4271-5.1.4-2`](#rfc4271-5.1.4-2), [`RFC4271-5.1.5-1`](#rfc4271-5.1.5-1), [`RFC4271-5.1.5-2`](#rfc4271-5.1.5-2), [`RFC4271-5.1.5-3`](#rfc4271-5.1.5-3), [`RFC4271-5.1.5-4`](#rfc4271-5.1.5-4), [`RFC4271-6.3-1`](#rfc4271-6.3-1), [`RFC4271-6.7-1`](#rfc4271-6.7-1), [`RFC4271-8.2.1-1`](#rfc4271-8.2.1-1), [`RFC4271-8.2.1-2`](#rfc4271-8.2.1-2), [`RFC4271-8.2.2-1`](#rfc4271-8.2.2-1), [`RFC4271-8.2.2-2`](#rfc4271-8.2.2-2), [`RFC4271-8.2.2-3`](#rfc4271-8.2.2-3), [`RFC4271-8.2.2-4`](#rfc4271-8.2.2-4), [`RFC4271-8.2.2-5`](#rfc4271-8.2.2-5), [`RFC4271-8.2.2-7`](#rfc4271-8.2.2-7), [`RFC4271-8.2.2-8`](#rfc4271-8.2.2-8), [`RFC4271-8.2.2-9`](#rfc4271-8.2.2-9), [`RFC4271-8.2.2-10`](#rfc4271-8.2.2-10), [`RFC4271-8.2.2-11`](#rfc4271-8.2.2-11), [`RFC4271-8.2.2-12`](#rfc4271-8.2.2-12), [`RFC4271-8.2.2-13`](#rfc4271-8.2.2-13), [`RFC4271-8.2.2-14`](#rfc4271-8.2.2-14), [`RFC4271-8.2.2-15`](#rfc4271-8.2.2-15), [`RFC4271-8.2.2-16`](#rfc4271-8.2.2-16), [`RFC4271-8.2.2-17`](#rfc4271-8.2.2-17), [`RFC4271-8.2.2-18`](#rfc4271-8.2.2-18), [`RFC4271-10-1`](#rfc4271-10-1), [`RFC4271-5.1.2-2`](#rfc4271-5.1.2-2), [`RFC4271-5.1.2-3`](#rfc4271-5.1.2-3), [`RFC4271-5.1.5-5`](#rfc4271-5.1.5-5), [`RFC4271-6.7-4`](#rfc4271-6.7-4), [`RFC4271-6.8-1`](#rfc4271-6.8-1), [`RFC4271-6.8-2`](#rfc4271-6.8-2), [`RFC4271-9-1`](#rfc4271-9-1), [`RFC4271-9-2`](#rfc4271-9-2), [`RFC4271-9-3`](#rfc4271-9-3), [`RFC4271-9.1.1-1`](#rfc4271-9.1.1-1), [`RFC4271-9.1.1-2`](#rfc4271-9.1.1-2), [`RFC4271-9.1.2-2`](#rfc4271-9.1.2-2), [`RFC4271-9.1.2-3`](#rfc4271-9.1.2-3), [`RFC4271-9.1.2.1-1`](#rfc4271-9.1.2.1-1), [`RFC4271-9.1.2.2-1`](#rfc4271-9.1.2.2-1), [`RFC4271-9.1.2.2-3`](#rfc4271-9.1.2.2-3), [`RFC4271-9.2-4`](#rfc4271-9.2-4), [`RFC4271-9.2-5`](#rfc4271-9.2-5), [`RFC4271-Security-1`](#rfc4271-security-1), [`RFC4271-9.2-6`](#rfc4271-9.2-6), [`RFC4271-9.2-7`](#rfc4271-9.2-7), [`RFC4271-9.2-8`](#rfc4271-9.2-8), [`RFC4271-9.2-9`](#rfc4271-9.2-9), [`RFC4271-9.2-10`](#rfc4271-9.2-10)

**Annotated instead of tested (25):** [`RFC4271-4.3-3`](#rfc4271-4.3-3), [`RFC4271-4.3-5`](#rfc4271-4.3-5), [`RFC4271-5.1.6-1`](#rfc4271-5.1.6-1), [`RFC4271-6.1-1`](#rfc4271-6.1-1), [`RFC4271-6.1-2`](#rfc4271-6.1-2), [`RFC4271-6.1-3`](#rfc4271-6.1-3), [`RFC4271-6.1-4`](#rfc4271-6.1-4), [`RFC4271-6.2-3`](#rfc4271-6.2-3), [`RFC4271-8.2.1-3`](#rfc4271-8.2.1-3), [`RFC4271-3.1-2`](#rfc4271-3.1-2), [`RFC4271-5.1.4-3`](#rfc4271-5.1.4-3), [`RFC4271-5.1.7-1`](#rfc4271-5.1.7-1), [`RFC4271-9.1.2-1`](#rfc4271-9.1.2-1), [`RFC4271-9.1.2-4`](#rfc4271-9.1.2-4), [`RFC4271-9.1.2.1-2`](#rfc4271-9.1.2.1-2), [`RFC4271-9.1.2.2-2`](#rfc4271-9.1.2.2-2), [`RFC4271-9.2-2`](#rfc4271-9.2-2), [`RFC4271-9.2-3`](#rfc4271-9.2-3), [`RFC4271-9.2.1.1-2`](#rfc4271-9.2.1.1-2), [`RFC4271-9.2.2.2-1`](#rfc4271-9.2.2.2-1), [`RFC4271-9.2.2.2-2`](#rfc4271-9.2.2.2-2), [`RFC4271-9.2.2.2-3`](#rfc4271-9.2.2.2-3), [`RFC4271-9.2.2.2-4`](#rfc4271-9.2.2.2-4), [`RFC4271-9.2.2.2-5`](#rfc4271-9.2.2.2-5), [`RFC4271-9.2.1.1-3`](#rfc4271-9.2.1.1-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4271-4.1-1` | Marker field MUST be set to all ones (16 bytes of 0xFF) (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4271MarkerAllOnesOnSend`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L19). **negative:** `unit/verify` [`TestRFC4271MarkerNotAllOnesRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L42) |
| `RFC4271-4.1-2` | Length field MUST have the smallest value required given the rest of the message (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4271SmallestLengthOnSend`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L62). **negative:** `unit/verify` [`TestRFC4271NonSmallestLengthRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L95) |
| `RFC4271-4.1-3` | Message Length MUST be between 19 and 4096 octets (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4271MessageLengthWithinBounds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L124). **negative:** `unit/verify` [`TestRFC4271MessageLengthOutOfBounds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L150) |
| `RFC4271-4.3-1` | For well-known attributes, the Transitive bit MUST be set to 1 (§4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC4271WellKnownAttributesAreTransitive`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L31). **negative:** `unit/verify` [`TestRFC4271WellKnownAttributeErrorsAreCaught`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L500) |
| `RFC4271-4.3-2` | Partial bit MUST be set to 0 for well-known and optional non-transitive attributes (§4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC4271PartialBitClearOnSend`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L55). **positive:** `unit/verify` [`TestRFC4271PartialNotSetOnRecognizedOrNonTransitive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1116). **negative:** `unit/verify` [`TestRFC4271PartialBitClearedOnReadvertisedWellKnown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L47). **negative:** `unit/verify` [`TestRFC4271PartialNotStampedOnExcludedClasses`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L170) |
| `RFC4271-4.3-3` | Lower-order four bits of attribute flags MUST be zero when sent (§4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC4271AttributeFlagsLowNibbleZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L80). **negative:** no negative test. **{single-polarity}:** the obligation is on the sender -- the flags octet ze writes must have its low-order four bits zero -- so there is no non-conformant input to reject. The receive-side mirror of the same rule ("MUST be ignored when received") is RFC4271-4.3-4 and is proven both ways there |
| `RFC4271-4.3-4` | Lower-order four bits of attribute flags MUST be ignored when received (§4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC4271AttrFlagsLowNibbleIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L232). **negative:** `unit/verify` [`TestRFC4271AttrFlagsHighBitsNotIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L259) |
| `RFC4271-4.4-1` | KEEPALIVE messages MUST NOT be sent more frequently than one per second (§4.4) | MUST NOT | 4.4 | **positive:** `unit/verify` [`TestRFC4271KeepaliveNotFasterThanOnePerSecond`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_test.go#L18). **negative:** `unit/verify` [`TestRFC4271KeepaliveIntervalNeverSubSecond`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_test.go#L50) |
| `RFC4271-4.4-2` | If the negotiated Hold Time is zero, periodic KEEPALIVE messages MUST NOT be sent (§4.4) | MUST NOT | 4.4 | **positive:** `unit/verify` [`TestTimersKeepaliveTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/timer_test.go#L144). **negative:** `unit/verify` [`TestKeepaliveWithZeroHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/timer_test.go#L411) |
| `RFC4271-6-1` | If no Error Subcode is specified, a zero MUST be used (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC4271NotificationUnspecifiedSubcodeIsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L353). **negative:** `unit/verify` [`TestRFC4271NotificationSpecifiedSubcodePreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L376) |
| `RFC4271-4.2-1` | Hold Time MUST be either zero or at least three seconds (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestOpenValidateHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L339). **negative:** `unit/verify` [`TestOpenValidateHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L341) |
| `RFC4271-4.2-2` | BGP speaker MUST calculate Hold Timer by using the smaller of its configured Hold Time and the received Hold Time (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestNegotiateWith_HoldTimeMinOfBoth`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L43). **negative:** `unit/verify` [`TestNegotiateWith_HoldTimeZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L72) |
| `RFC4271-6.2-1` | An implementation MUST reject Hold Time values of one or two seconds (§6.2) | MUST | 6.2 | **positive:** `unit/verify` [`TestOpenValidateHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L343). **negative:** `unit/verify` [`TestOpenValidateHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L345) |
| `RFC4271-6.2-2` | An implementation that accepts a Hold Time MUST use the negotiated value (§6.2) | MUST | 6.2 | **positive:** `unit/verify` [`TestRFC4271NegotiatedHoldTimeDrivesTimers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L171). **negative:** `unit/verify` [`TestRFC4271LocalHoldTimeNotUsedWhenPeerProposesSmaller`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L192) |
| `RFC4271-5-1` | BGP implementations MUST recognize all well-known attributes (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC4271WellKnownAttributesAreRecognized`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L463). **negative:** `unit/verify` [`TestRFC4271WellKnownAttributeErrorsAreCaught`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L492) |
| `RFC4271-5-2` | Well-known mandatory attributes MUST be included in every UPDATE containing NLRI (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC4271WellKnownAttributesAreRecognized`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L466). **negative:** `unit/verify` [`TestRFC4271WellKnownAttributeErrorsAreCaught`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L497) |
| `RFC4271-5-3` | Unrecognized transitive optional attributes MUST be passed along with the Partial bit set to 1 (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC4271PartialSetOnUnrecognizedTransitiveOptional`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1084). **positive:** `unit/verify` [`TestRFC4271PartialStampedOnUnrecognizedTransitive`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L132). **negative:** `unit/verify` [`TestRFC4271PartialNotSetOnRecognizedOrNonTransitive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1113). **negative:** `unit/verify` [`TestRFC4271PartialNotStampedOnExcludedClasses`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L167). **positive:** `functional/verify` [`rfc4271-partial-unknown-transitive.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc4271-partial-unknown-transitive.ci#L27) |
| `RFC4271-5-4` | Partial bit set to 1 by a previous AS MUST NOT be set back to 0 (§5) | MUST NOT | 5 | **positive:** `unit/verify` [`TestRFC4271PartialBitPreservedOnUnknownTransitive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L82). **positive:** `unit/verify` [`TestRFC4271PartialFromPreviousASNeverCleared`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1152). **negative:** `unit/verify` [`TestRFC4271PartialBitSurvivesLengthReframing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L115). **negative:** `unit/verify` [`TestRFC4271PartialFromPreviousASNotCleared`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L197) |
| `RFC4271-5-5` | Unrecognized non-transitive optional attributes MUST be quietly ignored (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC4271UnrecognizedNonTransitiveIsNotPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1210). **negative:** `unit/verify` [`TestRFC4271TheNonTransitiveDropSparesEveryOtherClass`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1251) |
| `RFC4271-5-6` | Receiver of an UPDATE MUST be prepared to handle path attributes that are out of order (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC4271AttributesOutOfOrderAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L293). **negative:** `unit/verify` [`TestRFC4271OutOfOrderDoesNotMaskMalformation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L323) |
| `RFC4271-4.3-5` | A BGP speaker MUST be able to process UPDATE messages with the same prefix in both WITHDRAWN and NLRI (§4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRIBInjectSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L119). **positive:** `unit/verify` [`TestRIBPoolPathSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L85). **positive:** `unit/verify` [`TestRIBSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L39). **negative:** no negative test. **{single-polarity}:** the obligation is to ACCEPT a message shape, so there is no non-conformant input to reject -- every UPDATE of this form must be processed, and a negative case would have to assert the absence of an error, which proves nothing (ai/rules/testing.md). The consequence the same paragraph asks for, treating the UPDATE as though WITHDRAWN did not contain the prefix, is RFC4271-4.3-7 and is proven by the same test |
| `RFC4271-5.1.3-1` | A route originated by a BGP speaker SHALL NOT be advertised to a peer using that peer's address as NEXT_HOP (§5.1.3) | SHALL NOT | 5.1.3 | **positive:** `unit/verify` [`TestEgressNextHopIsPeerOwnReadsTheRewrittenAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L282). **positive:** `unit/verify` [`TestForwardRSWithholdsRouteWhoseNextHopIsTheClientsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L252). **positive:** `unit/verify` [`TestForwardWithdrawsFromDestinationWhoseNextHopIsItsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L217). **positive:** `unit/verify` [`TestForwardWithholdsRouteWhoseNextHopIsTheDestinationsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L181). **positive:** `unit/verify` [`TestSendAnnounceWithholdsRouteWithPeerOwnNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L464). **positive:** `unit/verify` [`TestSendUpdateWithholdsOriginatedRouteWithPeerOwnNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L425). **negative:** `unit/verify` [`TestEgressNextHopIsPeerOwnReadsTheRewrittenAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L286). **negative:** `unit/verify` [`TestForwardRSWithholdsRouteWhoseNextHopIsTheClientsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L255). **negative:** `unit/verify` [`TestForwardWithholdsRouteWhoseNextHopIsTheDestinationsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L186). **negative:** `unit/verify` [`TestSendAnnounceWithholdsRouteWithPeerOwnNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L466). **negative:** `unit/verify` [`TestSendUpdateWithholdsOriginatedRouteWithPeerOwnNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L429). **positive:** `functional/verify` [`originated-nexthop-peer-own.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/originated-nexthop-peer-own.ci#L7). **negative:** `functional/verify` [`originated-nexthop-peer-own.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/originated-nexthop-peer-own.ci#L10). **positive:** `interop/nightly` [`checkSelfNextHopWithheld`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L961). **negative:** `interop/nightly` [`checkSelfNextHopWithheld`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L962) |
| `RFC4271-5.1.3-2` | A BGP speaker SHALL NOT install a route with itself as the next hop (§5.1.3) | SHALL NOT | 5.1.3 | **positive:** `unit/verify` [`TestRFC4271SelfNextHopRouteIsNotInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L43). **positive:** `unit/verify` [`TestRFC4271SelfNextHopSetComesFromPeerEvents`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L122). **negative:** `unit/verify` [`TestRFC4271SelfNextHopDoesNotShadowASoundAlternative`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L89). **negative:** `unit/verify` [`TestRFC4271SelfNextHopRouteIsNotInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L47). **negative:** `unit/verify` [`TestRFC4271SelfNextHopSetComesFromPeerEvents`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L126) |
| `RFC4271-5.1.3-3` | A BGP speaker MUST be able to support disabling advertisement of third-party NEXT_HOP attributes (§5.1.3) | MUST | 5.1.3 | **positive:** `unit/verify` [`TestRFC4271ThirdPartyNextHopCanBeDisabled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L341). **negative:** `unit/verify` [`TestRFC4271ThirdPartyNextHopDisableFailsClosed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L373) |
| `RFC4271-5.1.4-1` | MULTI_EXIT_DISC received from a neighboring AS MUST NOT be propagated to other neighboring ASes (§5.1.4) | MUST NOT | 5.1.4 | **positive:** `unit/verify` [`TestForwardSuppressesReceivedMEDToAnotherAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L213). **negative:** `unit/verify` [`TestForwardKeepsFilterSetMED`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L314). **negative:** `unit/verify` [`TestForwardSuppressesReceivedMEDToAnotherAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L218). **negative:** `unit/verify` [`TestForwardWritesLocallySetMED`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L281). **negative:** `unit/verify` [`TestMEDPropagationAllowedTo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L418). **positive:** `functional/verify` [`med-not-propagated-across-as.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-not-propagated-across-as.ci#L4). **negative:** `functional/verify` [`med-locally-set-reaches-peer.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-locally-set-reaches-peer.ci#L4). **negative:** `functional/verify` [`med-not-propagated-across-as.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-not-propagated-across-as.ci#L8). **positive:** `interop/nightly` [`checkMEDAcrossAS`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L196). **negative:** `interop/nightly` [`checkMEDAcrossAS`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L200) |
| `RFC4271-5.1.4-4` | A BGP speaker MUST implement a mechanism (based on local configuration) that allows the MULTI_EXIT_DISC attribute to be removed from a route (§5.1.4) | MUST | 5.1.4 | **positive:** `unit/verify` [`TestMEDRemovalMechanismIsConfigurable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L460). **positive:** `unit/verify` [`TestParseModifyDefsMEDRemove`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/modify_test.go#L548). **negative:** `unit/verify` [`TestMEDRemovalMechanismIsConfigurable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L467). **negative:** `unit/verify` [`TestMEDRemoveDirectiveIsValueless`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L560). **negative:** `unit/verify` [`TestParseModifyDefsMEDRemove`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/modify_test.go#L553). **positive:** `functional/verify` [`med-removal-configured.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-configured.ci#L4). **positive:** `interop/nightly` [`checkMEDRemovalConfiguration`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L318). **negative:** `interop/nightly` [`checkMEDRemovalConfiguration`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L322) |
| `RFC4271-5.1.4-2` | MULTI_EXIT_DISC removal from routes MUST be done before the route is used in Phase 2 of the decision process (§5.1.4) | MUST | 5.1.4 | **positive:** `unit/verify` [`TestHandleFilterUpdateMEDRemoveIsImportOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/modify_test.go#L621). **positive:** `unit/verify` [`TestMEDRemovalMechanismIsConfigurable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L472). **negative:** `unit/verify` [`TestHandleFilterUpdateMEDRemoveIsImportOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/modify_test.go#L628). **negative:** `unit/verify` [`TestMEDRemovalMechanismIsConfigurable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L480). **positive:** `functional/verify` [`med-removal-before-decision.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-before-decision.ci#L4). **positive:** `functional/verify` [`med-removal-configured.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-configured.ci#L8). **negative:** `functional/verify` [`med-removal-before-decision.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-before-decision.ci#L10). **negative:** `functional/verify` [`med-removal-export-refused.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-export-refused.ci#L4) |
| `RFC4271-5.1.5-1` | LOCAL_PREF SHALL be included in all UPDATE messages sent to internal peers (§5.1.5) | SHALL | 5.1.5 | **positive:** `unit/verify` [`TestRFC4271LocalPrefIncludedForInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L81). **negative:** `unit/verify` [`TestAnnounceStripsLocalPrefTowardExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_origin_test.go#L307). **negative:** `unit/verify` [`TestForwardLocalPrefStrippedToExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_local_pref_test.go#L45). **negative:** `unit/verify` [`TestRFC4271LocalPrefOmittedForExternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L107) |
| `RFC4271-5.1.5-2` | A BGP speaker MUST NOT include LOCAL_PREF in UPDATE messages sent to external peers (§5.1.5) | MUST NOT | 5.1.5 | **positive:** `unit/verify` [`TestAnnounceStripsLocalPrefTowardExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_origin_test.go#L305). **positive:** `unit/verify` [`TestForwardLocalPrefStripBeatsAFilterSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_local_pref_test.go#L113). **positive:** `unit/verify` [`TestForwardLocalPrefStrippedToExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_local_pref_test.go#L41). **positive:** `unit/verify` [`TestRFC4271LocalPrefOmittedForExternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L104). **negative:** `unit/verify` [`TestLocalPrefAllowedToIsTheOnlyAnswer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_local_pref_test.go#L146). **negative:** `unit/verify` [`TestRFC4271LocalPrefIncludedForInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L83). **positive:** `functional/verify` [`local-pref-strip-ebgp.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/local-pref-strip-ebgp.ci#L14). **negative:** `functional/verify` [`local-pref-strip-ebgp.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/local-pref-strip-ebgp.ci#L19). **positive:** `interop/nightly` [`checkLocalPrefStrip`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L96) |
| `RFC4271-5.1.5-3` | If LOCAL_PREF is received over EBGP, it MUST be ignored (§5.1.5) | MUST | 5.1.5 | **positive:** `unit/verify` [`TestRFC4271LocalPrefKeptOnInternalSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L566). **negative:** `unit/verify` [`TestRFC4271LocalPrefIgnoredOnExternalSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L589) |
| `RFC4271-5.1.5-4` | The higher degree of preference (LOCAL_PREF) MUST be preferred (§5.1.5) | MUST | 5.1.5 | **positive:** `unit/verify` [`TestBestPath_LocalPref`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L197). **negative:** `unit/verify` [`TestBestPath_LocalPref`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L199) |
| `RFC4271-5.1.6-1` | A BGP speaker that receives a route with the ATOMIC_AGGREGATE attribute MUST NOT make any NLRI of that route more specific when advertising this route to other BGP speakers (§5.1.6) | MUST NOT | 5.1.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the obligation binds the RECEIVER/re-advertiser, and ze is one -- it stores a received ATOMIC_AGGREGATE and copies it through on readvertisement (internal/component/bgp/reactor/peer_rib_routes.go:141) -- but the prohibited act has no producer. `grep -rniE "more specific\|deaggregat\|de-aggregat\|disaggregat" --include=*.go internal/component/bgp/ \| grep -v _test` returns only substring hits inside `encodeAggregatorValue` and `attrCodeAggregator` (internal/component/bgp/reactor/filter_delta.go:294,396, internal/component/bgp/message/rfc7606.go:64,421); no code path splits a prefix. Both readvertisement encoders write the stored route's own prefix verbatim through nlri.WriteNLRI (internal/component/bgp/reactor/peer_rib_routes.go:103-104), so the advertised NLRI is byte-identical to what was received and can be neither more nor less specific. With no length-altering producer there is no behavior to exercise in either polarity |
| `RFC4271-6.1-1` | All header errors MUST be indicated by sending NOTIFICATION with Error Code Message Header Error (§6.1) | MUST | 6.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** one class of header error is detected but never reported. A bad marker or a Length below 19 makes ParseHeader return a bare sentinel (internal/component/bgp/message/header.go:96-108), and the read loop turns that into an FSM event and a returned error with no NOTIFICATION sent (internal/component/bgp/reactor/session_read.go:98-102). The per-type and over-maximum length errors on the following lines do send Message Header Error (session_read.go:105-117) |
| `RFC4271-6.1-2` | Marker not all ones: Error Subcode MUST be Connection Not Synchronized (§6.1) | MUST | 6.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** NotifyHeaderConnectionNotSync is declared (internal/component/bgp/message/notification.go:52) but no producer ever sends it. ParseHeader returns ErrInvalidMarker, a plain sentinel carrying no NOTIFICATION (internal/component/bgp/message/header.go:96-99), and the read loop's marker-error branch sends nothing before returning (internal/component/bgp/reactor/session_read.go:98-102) |
| `RFC4271-6.1-3` | Invalid length: Error Subcode MUST be Bad Message Length; Data field MUST contain the erroneous Length field (§6.1) | MUST | 6.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** RFC 4271 §6.1 lists five length conditions and ze reports only four of them. The per-type minima and the 4096/65535 ceiling do produce a conformant Notification -- ValidateLength and ValidateLengthWithMax return a *Notification carrying NotifyHeaderBadLength and the two big-endian octets of the offending Length (internal/component/bgp/message/header.go:155-171 and :207-213), which the read loop sends before closing (internal/component/bgp/reactor/session_read.go:105-117). The first listed condition, "Length field of the message header is less than 19", does not: ParseHeader returns the bare sentinel ErrInvalidLength with no Notification and no Data (internal/component/bgp/message/header.go:106-108), and the read loop logs an FSM event and returns without writing anything (internal/component/bgp/reactor/session_read.go:98-102). The same code fact is recorded as the NOTIFICATION-absence gap on RFC4271-6.1-1. Disclosed in docs/features/rfc-status.md RFC 4271 row |
| `RFC4271-6.1-4` | Invalid type: Error Subcode MUST be Bad Message Type; Data field MUST contain the erroneous Type field (§6.1) | MUST | 6.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** an unknown message type is reported with the wrong subcode and the wrong Data. handleUnknownType sends Message Header Error with subcode 0 and a human-readable text string rather than subcode 3 (Bad Message Type) with the erroneous Type octet (internal/component/bgp/reactor/session_handlers.go:20-36); NotifyHeaderBadType is declared at internal/component/bgp/message/notification.go:54 and has no producer |
| `RFC4271-6.2-3` | All OPEN errors MUST be indicated by NOTIFICATION with Error Code OPEN Message Error (§6.2) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** one class of OPEN error is detected and never reported. UnpackOpen returns the bare sentinel ErrShortRead when the body is under 10 octets or when the Optional Parameters Length (standard or RFC 9072 extended) overruns the body (internal/component/bgp/message/open.go:167-168, :193-194, :199-200, :209-210), and handleOpen turns that into an FSM event and a returned error, writing no NOTIFICATION and not even closing the connection (internal/component/bgp/reactor/session_handlers.go:43-47); session_read.go:264 only propagates it. Every other OPEN error path does send Error Code 2 -- unsupported version (session_handlers.go:54-60), unacceptable Hold Time (:70-77) and a malformed capability (rejectOpenCapabilityError, :185-199) -- so the obligation holds everywhere except the decode failure. Disclosed in docs/features/rfc-status.md RFC 4271 row |
| `RFC4271-6.3-1` | All UPDATE errors MUST be indicated by NOTIFICATION with Error Code UPDATE Message Error (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestRFC4271UpdateErrorReportedAsUpdateMessageError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L633). **negative:** `unit/verify` [`TestRFC4271ConformantUpdateSendsNoUpdateError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L696) |
| `RFC4271-6.7-1` | Cease NOTIFICATION MUST NOT be used when a fatal error does exist (§6.7) | MUST NOT | 6.7 | **positive:** `unit/verify` [`TestPrefixExceedTeardown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_prefix_test.go#L110). **negative:** `unit/verify` [`TestRFC4271UpdateErrorReportedAsUpdateMessageError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L636) |
| `RFC4271-8.2.1-1` | BGP MUST maintain a separate FSM for each configured peer (§8.2.1) | MUST | 8.2.1 | **positive:** `unit/verify` [`TestRFC4271SeparateFSMPerPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L213). **negative:** `unit/verify` [`TestRFC4271PerPeerFSMDoesNotShareTimers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L237) |
| `RFC4271-8.2.1-2` | A BGP implementation MUST connect to and listen on TCP port 179 (§8.2.1) | MUST | 8.2.1 | **positive:** `unit/verify` [`TestRFC4271DefaultBGPPortIs179`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L306). **negative:** `unit/verify` [`TestRFC4271ExplicitPortOverridesDefault`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L322) |
| `RFC4271-8.2.1-3` | For each incoming connection, a state machine MUST be instantiated (§8.2.1) | MUST | 8.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** an incoming connection does not get its own state machine. acceptOrReject hands the accepted connection to the peer's existing session (internal/component/bgp/reactor/reactor_connection.go:117-163), and a connection queued for collision resolution is read raw by handlePendingCollision with no FSM behind it (internal/component/bgp/reactor/reactor_connection.go:196-249). An FSM is created per session, i.e. per connection attempt of a configured peer (internal/component/bgp/reactor/session.go:396), not per inbound connection |
| `RFC4271-8.2.2-1` | Event 10 (HoldTimer_Expires) lists "sends a NOTIFICATION message with the error code Hold Timer Expired" before "drops the TCP connection", in OpenSent, OpenConfirm and Established alike; a peer whose connection is dropped without it is told nothing. §8 states the FSM description is conceptual but binds "the same externally visible behavior", and the NOTIFICATION is the externally visible part, so the level is MUST (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271HoldTimerExpirySendsNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L807). **negative:** `unit/verify` [`TestRFC4271HoldTimerNotYetExpiredSendsNoNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L836). **positive:** `functional/verify` [`deadpeer-holddown.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/deadpeer-holddown.ci#L3) |
| `RFC4271-8.2.2-2` | Event 10 (HoldTimer_Expires) lists "sets the ConnectRetryTimer to zero", in OpenSent, OpenConfirm and Established alike. A ConnectRetryTimer left running past the teardown fires against a connection that no longer exists, and the retry it schedules is externally visible as an unexpected connection attempt, so the level is MUST on the same §8 "same externally visible behavior" ground as -1 (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271HoldExpiryRunsTheEvent10ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L929). **negative:** `unit/verify` [`TestRFC4271NoHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1000) |
| `RFC4271-8.2.2-3` | Event 10 (HoldTimer_Expires) lists "releases all BGP resources", in OpenSent, OpenConfirm and Established alike. The externally visible part is the KeepaliveTimer: a session torn down for silence that keeps its keepalive chain running writes KEEPALIVEs after the teardown, so the level is MUST (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271HoldExpiryRunsTheEvent10ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L932). **negative:** `unit/verify` [`TestRFC4271NoHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1002) |
| `RFC4271-8.2.2-4` | Event 10 (HoldTimer_Expires) lists "drops the TCP connection", in OpenSent, OpenConfirm and Established alike. The peer must see the connection go away and not merely stop receiving; a half-open socket is externally visible, so the level is MUST (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271HoldExpiryRunsTheEvent10ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L935). **negative:** `unit/verify` [`TestRFC4271NoHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1005) |
| `RFC4271-8.2.2-5` | Event 10 (HoldTimer_Expires) lists "changes its state to Idle", in OpenSent, OpenConfirm and Established alike. The Established->Idle transition is what deletes the routes learned over the connection, which is externally visible to every other peer, so the level is MUST (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271HoldExpiryRunsTheEvent10ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L938). **negative:** `unit/verify` [`TestRFC4271NoHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1007) |
| `RFC4271-8.2.2-6` | Event 10 (HoldTimer_Expires) lists "(optionally) performs peer oscillation damping if the DampPeerOscillations attribute is set to TRUE" (§8.2.2) | MAY | 8.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-8.2.2-7` | ManualStart (Event 1) in Idle lists "sets ConnectRetryCounter to zero", once for the Connect branch (Events 1 and 3) and again, in the same words, for the passive Active branch (Events 4 and 5). §8 makes ConnectRetryCounter a mandatory session attribute and defines it as "the number of times a BGP peer has tried to establish a peer session", so an operator start that left a stale retry history behind would misreport that number; the level is MUST (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterZeroedOnManualStart`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L42). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterSurvivesDampedStart`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L75) |
| `RFC4271-8.2.2-8` | ManualStop (Event 2) lists "sets the ConnectRetryCounter to zero", in Connect, Active, OpenSent, OpenConfirm and Established alike. Same §8 mandatory-attribute ground as -7: the operator stopping the peer ends the run of attempts the counter was counting (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterZeroedOnManualStop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L108). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterNotZeroedByIdleManualStop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L130) |
| `RFC4271-8.2.2-9` | HoldTimer_Expires (Event 10) lists "increments the ConnectRetryCounter", in OpenSent, OpenConfirm and Established alike (OpenSent omits the words "by 1" that the other two carry; the counter is an integer count, so the step is one). A session lost to silence is an attempt that failed, and §8 makes the count mandatory (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterIncrementsOnHoldTimerExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L151). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterQuietOnHealthyEstablishedTraffic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L174) |
| `RFC4271-8.2.2-10` | BGPHeaderErr (Event 21) and BGPOpenMsgErr (Event 22) list "increments the ConnectRetryCounter by 1" in Connect, Active, OpenSent and OpenConfirm, and in Established through "In response to any other event (Events 9, 12-13, 20-22)", which names both (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterIncrementsOnHeaderAndOpenErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L210). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterNotIncrementedByIdleErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L236) |
| `RFC4271-8.2.2-11` | NotifMsg (Event 25) leads to "increments the ConnectRetryCounter by 1" in every state that can see it: explicitly in OpenConfirm ("a TcpConnectionFails event (Event 18) [...] or a NOTIFICATION message (Event 25)") and Established ("a NOTIFICATION message (Event 24 or Event 25)"), and through the "any other event" list that names Event 25 in Connect, Active and OpenSent (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterIncrementsOnNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L258). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterStepsByExactlyOnePerNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L283) |
| `RFC4271-8.2.2-12` | NotifMsgVerErr (Event 24) increments the ConnectRetryCounter in Connect, Active and Established, and MUST NOT in OpenSent or OpenConfirm. In Connect and Active the clause sits in the DelayOpenTimer-is-not-running branch, which is the only branch a speaker without a DelayOpenTimer can take, and Established groups Event 24 with Events 25 and 18; in OpenSent and OpenConfirm the Event 24 action list is four items long and has no ConnectRetryCounter line at all (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterOnVersionErrorPerState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L308). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterQuietOnVersionErrorInOpenStates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L332) |
| `RFC4271-8.2.2-13` | TcpConnectionFails (Event 18) increments the ConnectRetryCounter in Active, OpenConfirm and Established, and MUST NOT in Connect or OpenSent. Connect's Event 18 has two branches and neither carries the clause, and OpenSent's Event 18 leaves for Active rather than tearing the peering down (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterOnTCPFailurePerState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L355). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterQuietOnTCPFailureInConnectAndOpenSent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L378) |
| `RFC4271-8.2.2-14` | UpdateMsgErr (Event 28) in Established lists "increments the ConnectRetryCounter by 1". An UPDATE error ends the session, which makes it a failed attempt by the §8 definition of the attribute (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterIncrementsOnUpdateError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L401). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterQuietOnGoodUpdate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L421) |
| `RFC4271-8.2.2-15` | The "any other event" action list lists "increments the ConnectRetryCounter by 1" ("by one" in Active) in all five non-Idle states: Connect and Active (Events 8, 10-11, 13, 19, 23, 25-28), OpenSent (Events 9, 11-13, 20, 25-28), OpenConfirm (Events 9, 12-13, 20, 27-28) and Established (Events 9, 12-13, 20-22). Idle is excluded on purpose: its own "any other event" clause says the event "does not cause change in the state of the local system" and lists no actions (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterIncrementsOnAnyOtherEvent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L443). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterIdleDefaultArmCountsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L478) |
| `RFC4271-8.2.2-16` | AutomaticStop (Event 8) lists "increments the ConnectRetryCounter by 1" in OpenSent, OpenConfirm and Established, and Connect and Active name Event 8 in their "any other events" list, which carries the same line. Its action list differs from ManualStop's by that one line, and the difference is the point: §8 defines the attribute as "the number of times a BGP peer has tried to establish a peer session", so a stop the local system chose records a failed attempt where an operator stop ends the count (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterIncrementsOnAutomaticStop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L549). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterAutomaticStopIsNotAManualStop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L575) |
| `RFC4271-8.2.2-17` | OpenCollisionDump (Event 23) lists "increments the ConnectRetryCounter by 1" in OpenSent, OpenConfirm and Established, and Connect and Active name Event 23 in their "any other events" list, which carries the same line. A connection closed by §6.8 collision resolution is an attempt that did not become a session (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterIncrementsOnOpenCollisionDump`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L606). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterCollisionDumpIsQuietInIdle`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L630) |
| `RFC4271-8.2.2-18` | ManualStop (Event 2) lists the Cease NOTIFICATION FIRST in its action list, in every state that has a connection to send it on: OpenSent writes "sends the NOTIFICATION with a Cease", OpenConfirm and Established write "sends the NOTIFICATION message with a Cease", and all three list it before "drops the TCP connection". The level is MUST on the same §8 ground as -1: the FSM description is conceptual, but an implementation "MUST support the described functionality and exhibit the same externally visible behavior", and the NOTIFICATION is the externally visible part -- it is the ONLY signal that says the operator stopped this speaker rather than the network dropping it. §6.7's MAY governs a different act, a speaker CHOOSING to Cease "at any given time" of its own accord; once the operator has issued Event 2 the action list is what the FSM owes. Connect and Active list no Cease clause and are excluded: neither state holds a BGP connection (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestShutdownNotifySendsCeaseFromEveryConnectedState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/shutdown_notify_test.go#L174). **negative:** `unit/verify` [`TestRFC4271NoCeaseWithoutAManualStop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/shutdown_notify_test.go#L207). **positive:** `functional/verify` [`signal-stop-cease.ci`](https://github.com/ze-software/ze/blob/main/test/reload/signal-stop-cease.ci#L3) |
| `RFC4271-10-1` | An implementation MUST allow the HoldTimer to be configurable on a per-peer basis (§10) | MUST | 10 | **positive:** `unit/verify` [`TestRFC4271HoldTimeConfigurablePerPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L261). **negative:** `unit/verify` [`TestRFC4271PerPeerHoldTimeSurvivesNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L285) |
| `RFC4271-3-1` | A BGP speaker SHOULD retain current routes from all peers or use Route Refresh (RFC 2918) (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-5-7` | Path attributes SHOULD be ordered in ascending order of attribute type (§5) | SHOULD | 5 | **positive:** `unit/verify` [`TestAnnounceBatchRail_AS4PathOrderedAgainstLargeCommunity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_batch_attr_order_test.go#L328). **positive:** `unit/verify` [`TestAnnounceBatchRail_AscendingTypeCodeOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_batch_attr_order_test.go#L268). **positive:** `unit/verify` [`TestAnnounceQueuedRail_AscendingTypeCodeOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_batch_attr_order_test.go#L289). **positive:** `unit/verify` [`TestSplitMP_PreservesAscendingAttributeOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_split_attr_order_test.go#L72). **negative:** no negative test |
| `RFC4271-5.1.6-2` | A BGP speaker SHOULD include ATOMIC_AGGREGATE when an aggregate excludes AS numbers (§5.1.6) | SHOULD | 5.1.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-5.1.6-3` | A BGP speaker SHOULD NOT remove ATOMIC_AGGREGATE when propagating the route (§5.1.6) | SHOULD NOT | 5.1.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-6.3-2` | Semantically incorrect NEXT_HOP SHOULD be logged and the route SHOULD be ignored (§6.3) | SHOULD | 6.3 | **positive:** `unit/verify` [`TestRFC4271SelfNextHopRouteIsNotInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L52). **negative:** no negative test |
| `RFC4271-6.3-3` | For semantically incorrect NEXT_HOP, a NOTIFICATION SHOULD NOT be sent (§6.3) | SHOULD NOT | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.2-1` | A BGP speaker SHOULD NOT advertise a feasible route if it would produce a duplicate UPDATE (§9.2) | SHOULD NOT | 9.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.2.1.1-1` | MinRouteAdvertisementIntervalTimer SHOULD NOT apply to routes sent to internal peers (§9.2.1.1) | SHOULD NOT | 9.2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-10-2` | Jitter SHOULD be applied to timers (§10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-5.1.1-1` | ORIGIN value SHOULD NOT be changed by any other speaker (§5.1.1) | SHOULD | 5.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-4.2-3` | A BGP speaker MAY reject connections on the basis of the Hold Time (§4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-3.1-1` | A BGP speaker MAY add to or modify path attributes before advertising (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-5.1.2-1` | A BGP speaker MAY include/prepend more than one instance of its own AS number in AS_PATH (§5.1.2) | MAY | 5.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-6.7-2` | A BGP speaker MAY support imposing a locally-configured upper bound on address prefixes (§6.7) | MAY | 6.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-10-3` | An implementation MAY allow other timers to be configurable (§10) | MAY | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-6.2-4` | An implementation MAY reject any proposed Hold Time (§6.2) | MAY | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-6.7-3` | The speaker MAY also log a Cease locally (§6.7) | MAY | 6.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-3.1-2` | Next hop for routes in Loc-RIB MUST be resolvable via the local BGP speaker's Routing Table (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the BGP Loc-RIB install performs no reachability check on the route's next hop. mirrorToLocRIB inserts the winning path with whatever next-hop address the attribute carried (internal/component/bgp/plugins/rib/rib_bestchange.go:797-830), and the candidate gather step filters only on SRv6 ineligibility (internal/component/bgp/plugins/rib/rib_commands.go:1039-1057). Resolvability is enforced downstream at FIB-install time, which removes the route from the routing table but leaves it in the Loc-RIB |
| `RFC4271-5.1.2-2` | When advertising a route to an internal peer, the speaker SHALL NOT modify the AS_PATH attribute (§5.1.2) | SHALL NOT | 5.1.2 | **positive:** `unit/verify` [`TestEstablishedAnnounce_ExplicitASPath_IBGPVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_batch_test.go#L567). **positive:** `unit/verify` [`TestRFC4271ASPathUnmodifiedTowardInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L131). **negative:** `unit/verify` [`TestRFC4271ASPathPrependedTowardExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L148) |
| `RFC4271-5.1.2-3` | When advertising a route to an external peer, the speaker prepends its own AS number to the leading AS_SEQUENCE of AS_PATH (creating one when the path is empty or led by an AS_SET) (§5.1.2) | SHALL | 5.1.2 | **positive:** `unit/verify` [`TestASPathSlotPrependOnlyWhenAdvertising`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/advertise_test.go#L97). **positive:** `unit/verify` [`TestEstablishedAnnounce_ExplicitASPath_PrependsLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_batch_test.go#L541). **negative:** `unit/verify` [`TestASPathSlotPrependOnlyWhenAdvertising`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/advertise_test.go#L99). **negative:** `unit/verify` [`TestEstablishedAnnounce_ExplicitASPath_IBGPVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_batch_test.go#L565). **positive:** `interop/nightly` [`checkRelayWithdrawalShape`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L431). **negative:** `interop/nightly` [`checkRelayWithdrawalShape`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L432) |
| `RFC4271-5.1.4-3` | If altering MULTI_EXIT_DISC received over EBGP, alteration MUST be done prior to decision process phases 1 and 2 (§5.1.4) | MUST | 5.1.4 | **positive:** `unit/verify` [`TestRFC4271MEDAlterationHappensAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L452). **negative:** no negative test. **{single-polarity}:** the requirement constrains only the ORDER of an alteration that a speaker chooses to make, so there is no non-conformant input a receiver could reject. ze's only place to alter a received MULTI_EXIT_DISC is the ingress filter chain, whose rewritten payload replaces the WireUpdate before the UPDATE is dispatched to the RIB plugin that runs phases 1 and 2 (internal/component/bgp/reactor/reactor_notify.go:427-466) |
| `RFC4271-5.1.5-5` | A BGP speaker SHALL calculate the degree of preference for each external route based on locally-configured policy (§5.1.5) | SHALL | 5.1.5 | **positive:** `unit/verify` [`TestRFC4271ExternalRouteDegreeOfPreferenceFromLocalPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L141). **negative:** `unit/verify` [`TestRFC4271DegreeOfPreferenceNotAHardcodedConstant`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L168) |
| `RFC4271-5.1.7-1` | A BGP speaker that performs aggregation and adds AGGREGATOR SHALL include its own AS number and IP address (§5.1.7) | SHALL | 5.1.7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never performs aggregation, so it never adds an AGGREGATOR of its own. The same grep as RFC4271-5.1.6-1 finds no aggregation producer; AGGREGATOR is only interned from the wire (internal/component/bgp/plugins/rib/storage/attrparse.go:96-102), replayed on readvertise (internal/component/bgp/plugins/rib/storage/familyrib.go:817-819) or emitted from operator configuration (internal/component/bgp/message/update_build_grouped.go:141-148) |
| `RFC4271-6.7-4` | When terminating due to prefix limit, speaker MUST send NOTIFICATION with Error Code Cease (§6.7) | MUST | 6.7 | **positive:** `unit/verify` [`TestPrefixExceedTeardown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_prefix_test.go#L107). **negative:** `unit/verify` [`TestPrefixExceedDrop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_prefix_test.go#L144) |
| `RFC4271-6.8-1` | In the event of connection collision, one of the connections MUST be closed (§6.8) | MUST | 6.8 | **positive:** `unit/verify` [`TestCollisionOpenConfirmLocalWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L127). **negative:** `unit/verify` [`TestCollisionOpenSentNoCollision`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L177) |
| `RFC4271-6.8-2` | Upon receipt of an OPEN message, the local system MUST examine all connections in OpenConfirm state for collision (§6.8) | MUST | 6.8 | **positive:** `unit/verify` [`TestCollisionOpenConfirmLocalWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L130). **negative:** `unit/verify` [`TestCollisionNonCollisionStates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L536) |
| `RFC4271-9-1` | Withdrawn routes SHALL be removed from the Adj-RIB-In and the Decision Process SHALL be run (§9) | SHALL | 9 | **positive:** `unit/verify` [`TestRFC4271WithdrawRemovesFromAdjRIBIn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L215). **negative:** `unit/verify` [`TestRFC4271WithdrawRemovesFromAdjRIBIn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L220) |
| `RFC4271-9-2` | A new route with identical NLRI to an existing route SHALL replace the older route in Adj-RIB-In (§9) | SHALL | 9 | **positive:** `unit/verify` [`TestRFC4271SamePrefixReplacesRatherThanAccumulates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L189). **negative:** `unit/verify` [`TestRFC4271WithdrawRemovesFromAdjRIBIn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L217) |
| `RFC4271-9-3` | Once the Adj-RIB-In is updated, the speaker SHALL run its Decision Process (§9) | SHALL | 9 | **positive:** `unit/verify` [`TestRIBBestChangeWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L685). **negative:** `unit/verify` [`TestRIBBestChangeNoPublishSameBest`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L649) |
| `RFC4271-9.1.1-1` | The degree of preference function SHALL NOT use the existence or attributes of other routes as inputs (§9.1.1) | SHALL NOT | 9.1.1 | **positive:** `unit/verify` [`TestRFC4271DegreeOfPreferenceIgnoresOtherRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L33). **negative:** `unit/verify` [`TestRFC4271DegreeOfPreferenceFollowsOwnAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L68) |
| `RFC4271-9.1.1-2` | For external routes, the computed degree of preference MUST be used as the LOCAL_PREF value in IBGP readvertisement (§9.1.1) | MUST | 9.1.1 | **positive:** `unit/verify` [`TestRFC4271LocalPrefIncludedForInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L86). **negative:** `unit/verify` [`TestRFC4271LocalPrefOmittedForExternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L110) |
| `RFC4271-9.1.2-1` | If NEXT_HOP is not resolvable, the BGP route MUST be excluded from Phase 2 decision function (§9.1.2) | MUST | 9.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** an unresolvable NEXT_HOP does not exclude the route from Phase 2. gatherCandidatesLocked skips only SRv6-ineligible entries (internal/component/bgp/plugins/rib/rib_commands.go:1039-1057), and extractCandidate uses the next hop solely to look up an IGP cost (internal/component/bgp/plugins/rib/rib_commands.go:1123-1131), so an unreachable next hop yields a cost of zero and the route competes normally |
| `RFC4271-9.1.2-2` | The local speaker SHALL install the best route in the Loc-RIB (§9.1.2) | SHALL | 9.1.2 | **positive:** `unit/verify` [`TestLocRIBMirror`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L1512). **negative:** `unit/verify` [`TestRIBBestChangeWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L690) |
| `RFC4271-9.1.2-3` | The local speaker MUST determine the immediate next-hop address from the NEXT_HOP attribute (§9.1.2) | MUST | 9.1.2 | **positive:** `unit/verify` [`TestLocRIBMirror`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L1514). **negative:** `unit/verify` [`TestRFC4271LocRIBNextHopComesFromNextHopAttribute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L94) |
| `RFC4271-9.1.2-4` | If immediate next-hop or IGP cost to NEXT_HOP changes, Phase 2 Route Selection MUST be performed again (§9.1.2) | MUST | 9.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nothing re-runs Phase 2 when the immediate next-hop or the IGP cost to the NEXT_HOP changes. The only entry points to checkBestPathChange are the UPDATE ingest path and the peer-state paths (internal/component/bgp/plugins/rib/rib_structured.go:271-286), and the IGP cost function is a passive lookup registered once with no invalidation callback (internal/component/bgp/plugins/rib/bestpath.go:30-43) |
| `RFC4271-9.1.2.1-1` | When installing a BGP route in the Routing Table, implementations MUST recalculate and take into account next-hops (§9.1.2.1) | MUST | 9.1.2.1 | **positive:** `unit/verify` [`TestLocRIBMirror`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L1517). **negative:** `unit/verify` [`TestRFC4271LocRIBNextHopComesFromNextHopAttribute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L98) |
| `RFC4271-9.1.2.1-2` | Unresolvable routes SHALL be removed from the Loc-RIB and the routing table (§9.1.2.1) | SHALL | 9.1.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** an unresolvable route is not removed from the Loc-RIB. The only Loc-RIB removal in the BGP plugin is the no-candidate-remains branch of checkBestPathChange (internal/component/bgp/plugins/rib/rib_bestchange.go:766-782), which is driven by the Adj-RIB-In losing its last path and never by next-hop resolvability; nothing in the plugin consults a resolver |
| `RFC4271-9.1.2.2-1` | The tie-breaking criteria MUST be applied in the order specified (§9.1.2.2) | MUST | 9.1.2.2 | **positive:** `unit/verify` [`TestBestPathStepFComparesThePeerBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_bgp_identifier_test.go#L88). **positive:** `unit/verify` [`TestBestPathStepFComparesThePeerBGPIdentifierOnTheJSONRail`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_bgp_identifier_json_test.go#L78). **positive:** `unit/verify` [`TestBestPath_FullTiebreak`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L530). **negative:** `unit/verify` [`TestBestPathEqualBGPIdentifiersFallThroughToPeerAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_bgp_identifier_test.go#L116). **negative:** `unit/verify` [`TestBestPathEqualBGPIdentifiersOnTheJSONRailFallThroughToPeerAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_bgp_identifier_json_test.go#L106). **negative:** `unit/verify` [`TestBestPath_MED_SameNeighborAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L315) |
| `RFC4271-9.1.2.2-2` | If MULTI_EXIT_DISC is removed before IBGP readvertisement, the optional MED comparison MUST be performed only among EBGP-learned routes (§9.1.2.2) | MUST | 9.1.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the common egress guard is implemented, but normal BGP selected-route readvertisement has no runnable producer and no discriminating proof. `bgp-rib` records selected Loc-RIB state (internal/component/bgp/plugins/rib/rib_bestchange.go:738-790), route-server and route-reflector plugins forward cached UPDATEs instead (internal/component/bgp/plugins/rs/server.go:433-434 and internal/component/bgp/plugins/rr/rr.go:188-195), and BGP-to-BGP redistribution is rejected as same-protocol redistribution (internal/core/redistevents/registry.go:144-146) |
| `RFC4271-9.1.2.2-3` | For IBGP-learned routes, MULTI_EXIT_DISC MUST be used in comparisons that reach the MED step (§9.1.2.2) | MUST | 9.1.2.2 | **positive:** `unit/verify` [`TestBestPath_MED_SameNeighborAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L319). **negative:** `unit/verify` [`TestBestPath_MED_SameNeighborAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L322) |
| `RFC4271-9.2-2` | A route SHALL NOT be installed in Adj-RIB-Out unless its destination and NEXT_HOP may be forwarded by the Routing Table (§9.2) | SHALL NOT | 9.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nothing gates Adj-RIB-Out installation on the destination and NEXT_HOP being forwardable. QueueAnnounce records the route unconditionally (internal/component/bgp/rib/outgoing.go:65-101), and the forwarding rails decide only on filters, family negotiation and the route-reflection rules (internal/component/bgp/reactor/forward_rs.go:295-333) |
| `RFC4271-9.2-3` | If a route in Loc-RIB is excluded from a particular Adj-RIB-Out, the previously advertised route MUST be withdrawn (§9.2) | MUST | 9.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a route excluded from a peer's Adj-RIB-Out by an egress filter is skipped silently, leaving the peer's previous advertisement in place instead of withdrawing it. Both forwarding rails `continue` on suppression with no withdrawal built (internal/component/bgp/reactor/forward_rs.go:320-333 and internal/component/bgp/reactor/reactor_api_forward.go:496-506); the one announce-to-withdraw conversion is LLGR-specific and filter-requested, not exclusion-driven (internal/component/bgp/reactor/reactor_api_forward.go:588-601) |
| `RFC4271-9.2-4` | The Decision Process MUST consider both overlapping routes based on acceptance policy (§9.2) | MUST | 9.2 | **positive:** `unit/verify` [`TestRFC4271OverlappingRoutesBothInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L148). **negative:** `unit/verify` [`TestRFC4271SamePrefixReplacesRatherThanAccumulates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L182) |
| `RFC4271-9.2-5` | If both less and more specific overlapping routes are accepted, the Decision Process MUST install both or an aggregate in Loc-RIB (§9.2) | MUST | 9.2 | **positive:** `unit/verify` [`TestRFC4271OverlappingRoutesBothInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L151). **negative:** `unit/verify` [`TestRFC4271SamePrefixReplacesRatherThanAccumulates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L186) |
| `RFC4271-9.2.1.1-2` | Two UPDATE messages advertising to common destinations MUST be separated by at least MinRouteAdvertisementIntervalTimer (§9.2.1.1) | MUST | 9.2.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze has no MinRouteAdvertisementIntervalTimer, so successive UPDATEs to a common set of destinations are not spaced. The timer set implements only ConnectRetry, Hold and Keepalive and records the omission in its own doc comment (internal/component/bgp/fsm/timer.go:34-42, "MinRouteAdvertisementIntervalTimer (Section 9.2.1.1) - not implemented here"); `grep -rniE 'minroute\|mrai' --include=*.go internal/` finds no producer |
| `RFC4271-9.2.2.2-1` | Routes with different MULTI_EXIT_DISC attributes SHALL NOT be aggregated (§9.2.2.2) | SHALL NOT | 9.2.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never aggregates routes, so no producer can aggregate two routes with different MULTI_EXIT_DISC values. `grep -rniE 'aggregate-address\|AggregateRoute\|route aggregation' --include=*.go .` returns no hit outside rfc/ and plan/, and no code path synthesizes an aggregate route from more-specifics |
| `RFC4271-9.2.2.2-2` | If any aggregated route has ORIGIN INCOMPLETE, the aggregate MUST have ORIGIN INCOMPLETE; else if any has EGP, the aggregate MUST have ORIGIN EGP (§9.2.2.2) | MUST | 9.2.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never aggregates routes, so no producer computes an aggregate ORIGIN. ORIGIN is only parsed (internal/core/bgp/attribute/origin.go:146-160), interned (internal/component/bgp/plugins/rib/storage/attrparse.go) and re-emitted verbatim (internal/component/bgp/plugins/rib/storage/familyrib.go:799-801); the same aggregation grep returns nothing |
| `RFC4271-9.2.2.2-3` | When aggregating routes with different NEXT_HOP, the aggregated NEXT_HOP SHALL identify an interface on the aggregating speaker (§9.2.2.2) | SHALL | 9.2.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never aggregates routes, so no producer chooses an aggregated NEXT_HOP. The only next-hop selection is per-route egress policy (internal/component/bgp/reactor/peer_forward_facts.go:153-193); the same aggregation grep returns nothing |
| `RFC4271-9.2.2.2-4` | If at least one aggregated route has ATOMIC_AGGREGATE, the aggregate SHALL have it as well (§9.2.2.2) | SHALL | 9.2.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never aggregates routes, so no producer decides whether an aggregate carries ATOMIC_AGGREGATE. The attribute is only decoded, stored and replayed (internal/core/bgp/attribute/simple.go:175-195, internal/component/bgp/plugins/rib/storage/familyrib.go:815-817); the same aggregation grep returns nothing |
| `RFC4271-9.2.2.2-5` | AGGREGATOR attributes from aggregated routes MUST NOT be included in the aggregated route (§9.2.2.2) | MUST NOT | 9.2.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never aggregates routes, so no producer builds an aggregated route from which a contributing AGGREGATOR would have to be excluded. AGGREGATOR is only interned from the wire or emitted from operator configuration (internal/component/bgp/plugins/rib/storage/attrparse.go:96-102, internal/component/bgp/message/update_build_grouped.go:141-148); the same aggregation grep returns nothing |
| `RFC4271-Security-1` | A BGP implementation MUST support TCP MD5 authentication (RFC 2385) (§Security, Appendix E) | MUST | Security | **positive:** `unit/verify` [`TestMD5PeersForListener`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_test.go#L2357). **negative:** `unit/verify` [`TestMD5PeersForListener`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_test.go#L2360) |
| `RFC4271-9.2.1.1-3` | The last route selected while awaiting MinRouteAdvertisementIntervalTimer SHALL be advertised at expiry (§9.2.1.1) | SHALL | 9.2.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** with no MinRouteAdvertisementIntervalTimer there is no expiry at which a last-selected route could be advertised. The timer is absent by design note (internal/component/bgp/fsm/timer.go:39) and no producer buffers a pending best-route advertisement against such a timer; best-path changes are published as they are computed (internal/component/bgp/plugins/rib/rib_bestchange.go:832-880) |
| `RFC4271-9.2-6` | A BGP speaker SHALL NOT redistribute routing information from an internal peer to other internal peers (unless route reflector) (§9.2) | SHALL NOT | 9.2 | **positive:** `unit/verify` [`TestRFC4271NoIBGPToIBGPRedistribution`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L508). **negative:** `unit/verify` [`TestRFC4271IBGPRedistributionAllowedForReflectorClient`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L567) |
| `RFC4271-9.2-7` | Newly unfeasible routes for which there is no replacement SHALL be advertised via UPDATE (§9.2) | SHALL | 9.2 | **positive:** `unit/verify` [`TestRIBBestChangeWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L688). **negative:** `unit/verify` [`TestRIBBestChangeNoPublishSameBest`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L652) |
| `RFC4271-9.2-8` | Any routes in the Loc-RIB marked as unfeasible SHALL be removed (§9.2) | SHALL | 9.2 | **positive:** `unit/verify` [`TestLocRIBMirror`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L1520). **negative:** `unit/verify` [`TestRIBBestChangeNoPublishSameBest`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L654) |
| `RFC4271-9.2-9` | Changes to reachable destinations within the speaker's own AS SHALL be advertised in an UPDATE (§9.2) | SHALL | 9.2 | **positive:** `unit/verify` [`TestRFC4271OwnASReachabilityChangeAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L398). **negative:** `unit/verify` [`TestRFC4271OwnASUnreachabilityChangeAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L426) |
| `RFC4271-9.2-10` | If a single route does not fit in an UPDATE message, the speaker MUST NOT advertise it and MAY log an error (§9.2) | MUST | 9.2 | **positive:** `unit/verify` [`TestRFC4271OversizeSingleRouteNotAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L402). **negative:** `unit/verify` [`TestRFC4271FittingRouteIsAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L431) |
| `RFC4271-Appendix-1` | Each BGP message SHOULD be transmitted with the TCP PUSH flag set (§Appendix E) | SHOULD | Appendix | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-Appendix-2` | The TCP connection used by BGP SHOULD be opened with DSCP bits 0-2 set to 110 (§Appendix E) | SHOULD | Appendix | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.3-1` | An AS SHOULD avoid using unstable routes (§9.3) | SHOULD | 9.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.3-2` | An AS SHOULD NOT make rapid, spontaneous changes to its choice of route (§9.3) | SHOULD NOT | 9.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.4-1` | Distribution of non-BGP acquired routes within an AS via BGP SHOULD be controlled via configuration (§9.4) | SHOULD | 9.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.1.2-5` | If the AS_PATH attribute of a BGP route contains an AS loop, the BGP route should be excluded from the Phase 2 decision function (§9.1.2). Detection scans the full AS path and checks that the local autonomous system number does not appear in it. RFC 4271 writes this keyword in lower case, so the level is a recommendation and not a capitalized RFC 2119 SHOULD. The same paragraph places a speaker configured to accept routes with its own autonomous system number in the AS path outside the scope of the document. That out-of-scope case is what the allow-own-as setting selects, so a non-zero allow-own-as is not a deviation from this line. | SHOULD | 9.1.2 | **positive:** `unit/verify` [`TestDetectASLoop_NotPresent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/filter/loop_test.go#L136). **negative:** `unit/verify` [`TestDetectASLoop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/filter/loop_test.go#L114). **negative:** `unit/verify` [`TestDetectASLoop_ASSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/filter/loop_test.go#L125). **negative:** `functional/verify` [`loop-as.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/loop-as.ci#L7) |
| `RFC4271-9.1.2.1-3` | Unresolvable routes SHOULD be kept in the Adj-RIB-In for future re-evaluation (§9.1.2.1) | SHOULD | 9.1.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.1.2.1-4` | If only an aggregate is available, only the longest matching route SHOULD be announced (§9.1.2.1) | SHOULD | 9.1.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.1.2.1-5` | Connection establishment failure SHOULD be logged (§9.1.2.1) | SHOULD | 9.1.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-8.2.1.4-1` | Connection that is closed in collision SHOULD be disposed (§8.2.1.4) | SHOULD | 8.2.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-4.3-6` | An UPDATE message SHOULD NOT include the same address prefix in both WITHDRAWN and NLRI (§4.3) | SHOULD | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-4.3-7` | An UPDATE containing the same prefix in WITHDRAWN and NLRI SHOULD be treated as if the prefix is not in WITHDRAWN (§4.3) | SHOULD | 4.3 | **positive:** `unit/verify` [`TestRIBInjectSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L121). **positive:** `unit/verify` [`TestRIBPoolPathSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L87). **positive:** `unit/verify` [`TestRIBSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L42). **negative:** no negative test |
| `RFC4271-5.1.7-2` | AGGREGATOR IP address SHOULD be the same as the BGP Identifier of the speaker (§5.1.7) | SHOULD | 5.1.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.2-11` | A BGP speaker that chooses to aggregate SHOULD either include all ASes in an AS_SET or add ATOMIC_AGGREGATE (§9.2) | SHOULD | 9.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.2-12` | Routes SHOULD NOT be de-aggregated (§9.2) | SHOULD NOT | 9.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.2.2.2-6` | If aggregated AS_PATH begins with AS_SET, the originator SHOULD NOT advertise MULTI_EXIT_DISC (§9.2.2.2) | SHOULD NOT | 9.2.2.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4271-5.1.6-1`](#rfc4271-5.1.6-1) A BGP speaker that receives a route with the ATOMIC_AGGREGATE attribute MUST NOT make any NLRI of that route more specific when advertising this route to other BGP speakers (§5.1.6) | no test | no test carries this requirement id; annotated {not-applicable}: the obligation binds the RECEIVER/re-advertiser, and ze is one -- it stores a received ATOMIC_AGGREGATE and copies it through on readvertisement (internal/component/bgp/reactor/peer_rib_routes.go:141) -- but the prohibited act has no producer. `grep -rniE "more specific\|deaggregat\|de-aggregat\|disaggregat" --include=*.go internal/component/bgp/ \| grep -v _test` returns only substring hits inside `encodeAggregatorValue` and `attrCodeAggregator` (internal/component/bgp/reactor/filter_delta.go:294,396, internal/component/bgp/message/rfc7606.go:64,421); no code path splits a prefix. Both readvertisement encoders write the stored route's own prefix verbatim through nlri.WriteNLRI (internal/component/bgp/reactor/peer_rib_routes.go:103-104), so the advertised NLRI is byte-identical to what was received and can be neither more nor less specific. With no length-altering producer there is no behavior to exercise in either polarity |
| [`RFC4271-6.1-1`](#rfc4271-6.1-1) All header errors MUST be indicated by sending NOTIFICATION with Error Code Message Header Error (§6.1) | {gap}, no test | one class of header error is detected but never reported. A bad marker or a Length below 19 makes ParseHeader return a bare sentinel (internal/component/bgp/message/header.go:96-108), and the read loop turns that into an FSM event and a returned error with no NOTIFICATION sent (internal/component/bgp/reactor/session_read.go:98-102). The per-type and over-maximum length errors on the following lines do send Message Header Error (session_read.go:105-117) |
| [`RFC4271-6.1-2`](#rfc4271-6.1-2) Marker not all ones: Error Subcode MUST be Connection Not Synchronized (§6.1) | {gap}, no test | NotifyHeaderConnectionNotSync is declared (internal/component/bgp/message/notification.go:52) but no producer ever sends it. ParseHeader returns ErrInvalidMarker, a plain sentinel carrying no NOTIFICATION (internal/component/bgp/message/header.go:96-99), and the read loop's marker-error branch sends nothing before returning (internal/component/bgp/reactor/session_read.go:98-102) |
| [`RFC4271-6.1-3`](#rfc4271-6.1-3) Invalid length: Error Subcode MUST be Bad Message Length; Data field MUST contain the erroneous Length field (§6.1) | {gap}, no test | RFC 4271 §6.1 lists five length conditions and ze reports only four of them. The per-type minima and the 4096/65535 ceiling do produce a conformant Notification -- ValidateLength and ValidateLengthWithMax return a *Notification carrying NotifyHeaderBadLength and the two big-endian octets of the offending Length (internal/component/bgp/message/header.go:155-171 and :207-213), which the read loop sends before closing (internal/component/bgp/reactor/session_read.go:105-117). The first listed condition, "Length field of the message header is less than 19", does not: ParseHeader returns the bare sentinel ErrInvalidLength with no Notification and no Data (internal/component/bgp/message/header.go:106-108), and the read loop logs an FSM event and returns without writing anything (internal/component/bgp/reactor/session_read.go:98-102). The same code fact is recorded as the NOTIFICATION-absence gap on RFC4271-6.1-1. Disclosed in docs/features/rfc-status.md RFC 4271 row |
| [`RFC4271-6.1-4`](#rfc4271-6.1-4) Invalid type: Error Subcode MUST be Bad Message Type; Data field MUST contain the erroneous Type field (§6.1) | {gap}, no test | an unknown message type is reported with the wrong subcode and the wrong Data. handleUnknownType sends Message Header Error with subcode 0 and a human-readable text string rather than subcode 3 (Bad Message Type) with the erroneous Type octet (internal/component/bgp/reactor/session_handlers.go:20-36); NotifyHeaderBadType is declared at internal/component/bgp/message/notification.go:54 and has no producer |
| [`RFC4271-6.2-3`](#rfc4271-6.2-3) All OPEN errors MUST be indicated by NOTIFICATION with Error Code OPEN Message Error (§6.2) | {gap}, no test | one class of OPEN error is detected and never reported. UnpackOpen returns the bare sentinel ErrShortRead when the body is under 10 octets or when the Optional Parameters Length (standard or RFC 9072 extended) overruns the body (internal/component/bgp/message/open.go:167-168, :193-194, :199-200, :209-210), and handleOpen turns that into an FSM event and a returned error, writing no NOTIFICATION and not even closing the connection (internal/component/bgp/reactor/session_handlers.go:43-47); session_read.go:264 only propagates it. Every other OPEN error path does send Error Code 2 -- unsupported version (session_handlers.go:54-60), unacceptable Hold Time (:70-77) and a malformed capability (rejectOpenCapabilityError, :185-199) -- so the obligation holds everywhere except the decode failure. Disclosed in docs/features/rfc-status.md RFC 4271 row |
| [`RFC4271-8.2.1-3`](#rfc4271-8.2.1-3) For each incoming connection, a state machine MUST be instantiated (§8.2.1) | {gap}, no test | an incoming connection does not get its own state machine. acceptOrReject hands the accepted connection to the peer's existing session (internal/component/bgp/reactor/reactor_connection.go:117-163), and a connection queued for collision resolution is read raw by handlePendingCollision with no FSM behind it (internal/component/bgp/reactor/reactor_connection.go:196-249). An FSM is created per session, i.e. per connection attempt of a configured peer (internal/component/bgp/reactor/session.go:396), not per inbound connection |
| [`RFC4271-3.1-2`](#rfc4271-3.1-2) Next hop for routes in Loc-RIB MUST be resolvable via the local BGP speaker's Routing Table (§3.1) | {gap}, no test | the BGP Loc-RIB install performs no reachability check on the route's next hop. mirrorToLocRIB inserts the winning path with whatever next-hop address the attribute carried (internal/component/bgp/plugins/rib/rib_bestchange.go:797-830), and the candidate gather step filters only on SRv6 ineligibility (internal/component/bgp/plugins/rib/rib_commands.go:1039-1057). Resolvability is enforced downstream at FIB-install time, which removes the route from the routing table but leaves it in the Loc-RIB |
| [`RFC4271-5.1.7-1`](#rfc4271-5.1.7-1) A BGP speaker that performs aggregation and adds AGGREGATOR SHALL include its own AS number and IP address (§5.1.7) | no test | no test carries this requirement id; annotated {not-applicable}: ze never performs aggregation, so it never adds an AGGREGATOR of its own. The same grep as RFC4271-5.1.6-1 finds no aggregation producer; AGGREGATOR is only interned from the wire (internal/component/bgp/plugins/rib/storage/attrparse.go:96-102), replayed on readvertise (internal/component/bgp/plugins/rib/storage/familyrib.go:817-819) or emitted from operator configuration (internal/component/bgp/message/update_build_grouped.go:141-148) |
| [`RFC4271-9.1.2-1`](#rfc4271-9.1.2-1) If NEXT_HOP is not resolvable, the BGP route MUST be excluded from Phase 2 decision function (§9.1.2) | {gap}, no test | an unresolvable NEXT_HOP does not exclude the route from Phase 2. gatherCandidatesLocked skips only SRv6-ineligible entries (internal/component/bgp/plugins/rib/rib_commands.go:1039-1057), and extractCandidate uses the next hop solely to look up an IGP cost (internal/component/bgp/plugins/rib/rib_commands.go:1123-1131), so an unreachable next hop yields a cost of zero and the route competes normally |
| [`RFC4271-9.1.2-4`](#rfc4271-9.1.2-4) If immediate next-hop or IGP cost to NEXT_HOP changes, Phase 2 Route Selection MUST be performed again (§9.1.2) | {gap}, no test | nothing re-runs Phase 2 when the immediate next-hop or the IGP cost to the NEXT_HOP changes. The only entry points to checkBestPathChange are the UPDATE ingest path and the peer-state paths (internal/component/bgp/plugins/rib/rib_structured.go:271-286), and the IGP cost function is a passive lookup registered once with no invalidation callback (internal/component/bgp/plugins/rib/bestpath.go:30-43) |
| [`RFC4271-9.1.2.1-2`](#rfc4271-9.1.2.1-2) Unresolvable routes SHALL be removed from the Loc-RIB and the routing table (§9.1.2.1) | {gap}, no test | an unresolvable route is not removed from the Loc-RIB. The only Loc-RIB removal in the BGP plugin is the no-candidate-remains branch of checkBestPathChange (internal/component/bgp/plugins/rib/rib_bestchange.go:766-782), which is driven by the Adj-RIB-In losing its last path and never by next-hop resolvability; nothing in the plugin consults a resolver |
| [`RFC4271-9.1.2.2-2`](#rfc4271-9.1.2.2-2) If MULTI_EXIT_DISC is removed before IBGP readvertisement, the optional MED comparison MUST be performed only among EBGP-learned routes (§9.1.2.2) | {gap}, no test | the common egress guard is implemented, but normal BGP selected-route readvertisement has no runnable producer and no discriminating proof. `bgp-rib` records selected Loc-RIB state (internal/component/bgp/plugins/rib/rib_bestchange.go:738-790), route-server and route-reflector plugins forward cached UPDATEs instead (internal/component/bgp/plugins/rs/server.go:433-434 and internal/component/bgp/plugins/rr/rr.go:188-195), and BGP-to-BGP redistribution is rejected as same-protocol redistribution (internal/core/redistevents/registry.go:144-146) |
| [`RFC4271-9.2-2`](#rfc4271-9.2-2) A route SHALL NOT be installed in Adj-RIB-Out unless its destination and NEXT_HOP may be forwarded by the Routing Table (§9.2) | {gap}, no test | nothing gates Adj-RIB-Out installation on the destination and NEXT_HOP being forwardable. QueueAnnounce records the route unconditionally (internal/component/bgp/rib/outgoing.go:65-101), and the forwarding rails decide only on filters, family negotiation and the route-reflection rules (internal/component/bgp/reactor/forward_rs.go:295-333) |
| [`RFC4271-9.2-3`](#rfc4271-9.2-3) If a route in Loc-RIB is excluded from a particular Adj-RIB-Out, the previously advertised route MUST be withdrawn (§9.2) | {gap}, no test | a route excluded from a peer's Adj-RIB-Out by an egress filter is skipped silently, leaving the peer's previous advertisement in place instead of withdrawing it. Both forwarding rails `continue` on suppression with no withdrawal built (internal/component/bgp/reactor/forward_rs.go:320-333 and internal/component/bgp/reactor/reactor_api_forward.go:496-506); the one announce-to-withdraw conversion is LLGR-specific and filter-requested, not exclusion-driven (internal/component/bgp/reactor/reactor_api_forward.go:588-601) |
| [`RFC4271-9.2.1.1-2`](#rfc4271-9.2.1.1-2) Two UPDATE messages advertising to common destinations MUST be separated by at least MinRouteAdvertisementIntervalTimer (§9.2.1.1) | {gap}, no test | ze has no MinRouteAdvertisementIntervalTimer, so successive UPDATEs to a common set of destinations are not spaced. The timer set implements only ConnectRetry, Hold and Keepalive and records the omission in its own doc comment (internal/component/bgp/fsm/timer.go:34-42, "MinRouteAdvertisementIntervalTimer (Section 9.2.1.1) - not implemented here"); `grep -rniE 'minroute\|mrai' --include=*.go internal/` finds no producer |
| [`RFC4271-9.2.2.2-1`](#rfc4271-9.2.2.2-1) Routes with different MULTI_EXIT_DISC attributes SHALL NOT be aggregated (§9.2.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze never aggregates routes, so no producer can aggregate two routes with different MULTI_EXIT_DISC values. `grep -rniE 'aggregate-address\|AggregateRoute\|route aggregation' --include=*.go .` returns no hit outside rfc/ and plan/, and no code path synthesizes an aggregate route from more-specifics |
| [`RFC4271-9.2.2.2-2`](#rfc4271-9.2.2.2-2) If any aggregated route has ORIGIN INCOMPLETE, the aggregate MUST have ORIGIN INCOMPLETE; else if any has EGP, the aggregate MUST have ORIGIN EGP (§9.2.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze never aggregates routes, so no producer computes an aggregate ORIGIN. ORIGIN is only parsed (internal/core/bgp/attribute/origin.go:146-160), interned (internal/component/bgp/plugins/rib/storage/attrparse.go) and re-emitted verbatim (internal/component/bgp/plugins/rib/storage/familyrib.go:799-801); the same aggregation grep returns nothing |
| [`RFC4271-9.2.2.2-3`](#rfc4271-9.2.2.2-3) When aggregating routes with different NEXT_HOP, the aggregated NEXT_HOP SHALL identify an interface on the aggregating speaker (§9.2.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze never aggregates routes, so no producer chooses an aggregated NEXT_HOP. The only next-hop selection is per-route egress policy (internal/component/bgp/reactor/peer_forward_facts.go:153-193); the same aggregation grep returns nothing |
| [`RFC4271-9.2.2.2-4`](#rfc4271-9.2.2.2-4) If at least one aggregated route has ATOMIC_AGGREGATE, the aggregate SHALL have it as well (§9.2.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze never aggregates routes, so no producer decides whether an aggregate carries ATOMIC_AGGREGATE. The attribute is only decoded, stored and replayed (internal/core/bgp/attribute/simple.go:175-195, internal/component/bgp/plugins/rib/storage/familyrib.go:815-817); the same aggregation grep returns nothing |
| [`RFC4271-9.2.2.2-5`](#rfc4271-9.2.2.2-5) AGGREGATOR attributes from aggregated routes MUST NOT be included in the aggregated route (§9.2.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze never aggregates routes, so no producer builds an aggregated route from which a contributing AGGREGATOR would have to be excluded. AGGREGATOR is only interned from the wire or emitted from operator configuration (internal/component/bgp/plugins/rib/storage/attrparse.go:96-102, internal/component/bgp/message/update_build_grouped.go:141-148); the same aggregation grep returns nothing |
| [`RFC4271-9.2.1.1-3`](#rfc4271-9.2.1.1-3) The last route selected while awaiting MinRouteAdvertisementIntervalTimer SHALL be advertised at expiry (§9.2.1.1) | {gap}, no test | with no MinRouteAdvertisementIntervalTimer there is no expiry at which a last-selected route could be advertised. The timer is absent by design note (internal/component/bgp/fsm/timer.go:39) and no producer buffers a pending best-route advertisement against such a timer; best-path changes are published as they are computed (internal/component/bgp/plugins/rib/rib_bestchange.go:832-880) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4271-4.1-1`](#rfc4271-4.1-1)

Marker field MUST be set to all ones (16 bytes of 0xFF) (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271MarkerNotAllOnesRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L42) | unit/verify | unproven |
| positive | [`TestRFC4271MarkerAllOnesOnSend`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L19) | unit/verify | unproven |

### [`RFC4271-4.1-2`](#rfc4271-4.1-2)

Length field MUST have the smallest value required given the rest of the message (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271NonSmallestLengthRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L95) | unit/verify | unproven |
| positive | [`TestRFC4271SmallestLengthOnSend`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L62) | unit/verify | unproven |

### [`RFC4271-4.1-3`](#rfc4271-4.1-3)

Message Length MUST be between 19 and 4096 octets (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271MessageLengthOutOfBounds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L150) | unit/verify | unproven |
| positive | [`TestRFC4271MessageLengthWithinBounds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L124) | unit/verify | unproven |

### [`RFC4271-4.3-1`](#rfc4271-4.3-1)

For well-known attributes, the Transitive bit MUST be set to 1 (§4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271WellKnownAttributeErrorsAreCaught`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L500) | unit/verify | unproven |
| positive | [`TestRFC4271WellKnownAttributesAreTransitive`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L31) | unit/verify | unproven |

### [`RFC4271-4.3-2`](#rfc4271-4.3-2)

Partial bit MUST be set to 0 for well-known and optional non-transitive attributes (§4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271PartialBitClearedOnReadvertisedWellKnown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L47) | unit/verify | unproven |
| negative | [`TestRFC4271PartialNotStampedOnExcludedClasses`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L170) | unit/verify | unproven |
| positive | [`TestRFC4271PartialNotSetOnRecognizedOrNonTransitive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1116) | unit/verify | unproven |
| positive | [`TestRFC4271PartialBitClearOnSend`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L55) | unit/verify | unproven |

### [`RFC4271-4.3-3`](#rfc4271-4.3-3)

Lower-order four bits of attribute flags MUST be zero when sent (§4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4271AttributeFlagsLowNibbleZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L80) | unit/verify | unproven |

### [`RFC4271-4.3-4`](#rfc4271-4.3-4)

Lower-order four bits of attribute flags MUST be ignored when received (§4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271AttrFlagsHighBitsNotIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L259) | unit/verify | unproven |
| positive | [`TestRFC4271AttrFlagsLowNibbleIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L232) | unit/verify | unproven |

### [`RFC4271-4.4-1`](#rfc4271-4.4-1)

KEEPALIVE messages MUST NOT be sent more frequently than one per second (§4.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271KeepaliveIntervalNeverSubSecond`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_test.go#L50) | unit/verify | unproven |
| positive | [`TestRFC4271KeepaliveNotFasterThanOnePerSecond`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_test.go#L18) | unit/verify | unproven |

### [`RFC4271-4.4-2`](#rfc4271-4.4-2)

If the negotiated Hold Time is zero, periodic KEEPALIVE messages MUST NOT be sent (§4.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestKeepaliveWithZeroHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/timer_test.go#L411) | unit/verify | unproven |
| positive | [`TestTimersKeepaliveTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/timer_test.go#L144) | unit/verify | unproven |

### [`RFC4271-6-1`](#rfc4271-6-1)

If no Error Subcode is specified, a zero MUST be used (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271NotificationSpecifiedSubcodePreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L376) | unit/verify | unproven |
| positive | [`TestRFC4271NotificationUnspecifiedSubcodeIsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L353) | unit/verify | unproven |

### [`RFC4271-4.2-1`](#rfc4271-4.2-1)

Hold Time MUST be either zero or at least three seconds (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpenValidateHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L341) | unit/verify | unproven |
| positive | [`TestOpenValidateHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L339) | unit/verify | unproven |

### [`RFC4271-4.2-2`](#rfc4271-4.2-2)

BGP speaker MUST calculate Hold Timer by using the smaller of its configured Hold Time and the received Hold Time (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegotiateWith_HoldTimeZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L72) | unit/verify | unproven |
| positive | [`TestNegotiateWith_HoldTimeMinOfBoth`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L43) | unit/verify | unproven |

### [`RFC4271-6.2-1`](#rfc4271-6.2-1)

An implementation MUST reject Hold Time values of one or two seconds (§6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpenValidateHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L345) | unit/verify | unproven |
| positive | [`TestOpenValidateHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L343) | unit/verify | unproven |

### [`RFC4271-6.2-2`](#rfc4271-6.2-2)

An implementation that accepts a Hold Time MUST use the negotiated value (§6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271LocalHoldTimeNotUsedWhenPeerProposesSmaller`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L192) | unit/verify | unproven |
| positive | [`TestRFC4271NegotiatedHoldTimeDrivesTimers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L171) | unit/verify | unproven |

### [`RFC4271-5-1`](#rfc4271-5-1)

BGP implementations MUST recognize all well-known attributes (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271WellKnownAttributeErrorsAreCaught`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L492) | unit/verify | unproven |
| positive | [`TestRFC4271WellKnownAttributesAreRecognized`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L463) | unit/verify | unproven |

### [`RFC4271-5-2`](#rfc4271-5-2)

Well-known mandatory attributes MUST be included in every UPDATE containing NLRI (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271WellKnownAttributeErrorsAreCaught`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L497) | unit/verify | unproven |
| positive | [`TestRFC4271WellKnownAttributesAreRecognized`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L466) | unit/verify | unproven |

### [`RFC4271-5-3`](#rfc4271-5-3)

Unrecognized transitive optional attributes MUST be passed along with the Partial bit set to 1 (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271PartialNotSetOnRecognizedOrNonTransitive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1113) | unit/verify | unproven |
| negative | [`TestRFC4271PartialNotStampedOnExcludedClasses`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L167) | unit/verify | unproven |
| positive | [`TestRFC4271PartialSetOnUnrecognizedTransitiveOptional`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1084) | unit/verify | unproven |
| positive | [`TestRFC4271PartialStampedOnUnrecognizedTransitive`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L132) | unit/verify | unproven |
| positive | [`rfc4271-partial-unknown-transitive.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc4271-partial-unknown-transitive.ci#L27) | functional/verify | unproven |

### [`RFC4271-5-4`](#rfc4271-5-4)

Partial bit set to 1 by a previous AS MUST NOT be set back to 0 (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271PartialBitSurvivesLengthReframing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L115) | unit/verify | unproven |
| negative | [`TestRFC4271PartialFromPreviousASNotCleared`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L197) | unit/verify | unproven |
| positive | [`TestRFC4271PartialBitPreservedOnUnknownTransitive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L82) | unit/verify | unproven |
| positive | [`TestRFC4271PartialFromPreviousASNeverCleared`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1152) | unit/verify | unproven |

### [`RFC4271-5-5`](#rfc4271-5-5)

Unrecognized non-transitive optional attributes MUST be quietly ignored (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271TheNonTransitiveDropSparesEveryOtherClass`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1251) | unit/verify | unproven |
| positive | [`TestRFC4271UnrecognizedNonTransitiveIsNotPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1210) | unit/verify | unproven |

### [`RFC4271-5-6`](#rfc4271-5-6)

Receiver of an UPDATE MUST be prepared to handle path attributes that are out of order (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271OutOfOrderDoesNotMaskMalformation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L323) | unit/verify | unproven |
| positive | [`TestRFC4271AttributesOutOfOrderAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L293) | unit/verify | unproven |

### [`RFC4271-4.3-5`](#rfc4271-4.3-5)

A BGP speaker MUST be able to process UPDATE messages with the same prefix in both WITHDRAWN and NLRI (§4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRIBInjectSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L119) | unit/verify | unproven |
| positive | [`TestRIBPoolPathSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L85) | unit/verify | unproven |
| positive | [`TestRIBSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L39) | unit/verify | unproven |

### [`RFC4271-5.1.3-1`](#rfc4271-5.1.3-1)

A route originated by a BGP speaker SHALL NOT be advertised to a peer using that peer's address as NEXT_HOP (§5.1.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEgressNextHopIsPeerOwnReadsTheRewrittenAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L286) | unit/verify | unproven |
| negative | [`TestForwardRSWithholdsRouteWhoseNextHopIsTheClientsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L255) | unit/verify | unproven |
| negative | [`TestForwardWithholdsRouteWhoseNextHopIsTheDestinationsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L186) | unit/verify | unproven |
| negative | [`TestSendAnnounceWithholdsRouteWithPeerOwnNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L466) | unit/verify | unproven |
| negative | [`TestSendUpdateWithholdsOriginatedRouteWithPeerOwnNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L429) | unit/verify | unproven |
| negative | [`checkSelfNextHopWithheld`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L962) | interop/nightly | unproven |
| negative | [`originated-nexthop-peer-own.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/originated-nexthop-peer-own.ci#L10) | functional/verify | unproven |
| positive | [`TestEgressNextHopIsPeerOwnReadsTheRewrittenAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L282) | unit/verify | unproven |
| positive | [`TestForwardRSWithholdsRouteWhoseNextHopIsTheClientsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L252) | unit/verify | unproven |
| positive | [`TestForwardWithdrawsFromDestinationWhoseNextHopIsItsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L217) | unit/verify | unproven |
| positive | [`TestForwardWithholdsRouteWhoseNextHopIsTheDestinationsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L181) | unit/verify | unproven |
| positive | [`TestSendAnnounceWithholdsRouteWithPeerOwnNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L464) | unit/verify | unproven |
| positive | [`TestSendUpdateWithholdsOriginatedRouteWithPeerOwnNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L425) | unit/verify | unproven |
| positive | [`checkSelfNextHopWithheld`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L961) | interop/nightly | unproven |
| positive | [`originated-nexthop-peer-own.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/originated-nexthop-peer-own.ci#L7) | functional/verify | unproven |

### [`RFC4271-5.1.3-2`](#rfc4271-5.1.3-2)

A BGP speaker SHALL NOT install a route with itself as the next hop (§5.1.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271SelfNextHopDoesNotShadowASoundAlternative`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L89) | unit/verify | unproven |
| negative | [`TestRFC4271SelfNextHopRouteIsNotInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L47) | unit/verify | unproven |
| negative | [`TestRFC4271SelfNextHopSetComesFromPeerEvents`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L126) | unit/verify | unproven |
| positive | [`TestRFC4271SelfNextHopRouteIsNotInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L43) | unit/verify | unproven |
| positive | [`TestRFC4271SelfNextHopSetComesFromPeerEvents`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L122) | unit/verify | unproven |

### [`RFC4271-5.1.3-3`](#rfc4271-5.1.3-3)

A BGP speaker MUST be able to support disabling advertisement of third-party NEXT_HOP attributes (§5.1.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ThirdPartyNextHopDisableFailsClosed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L373) | unit/verify | unproven |
| positive | [`TestRFC4271ThirdPartyNextHopCanBeDisabled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L341) | unit/verify | unproven |

### [`RFC4271-5.1.4-1`](#rfc4271-5.1.4-1)

MULTI_EXIT_DISC received from a neighboring AS MUST NOT be propagated to other neighboring ASes (§5.1.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestForwardKeepsFilterSetMED`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L314) | unit/verify | unproven |
| negative | [`TestForwardSuppressesReceivedMEDToAnotherAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L218) | unit/verify | unproven |
| negative | [`TestForwardWritesLocallySetMED`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L281) | unit/verify | unproven |
| negative | [`TestMEDPropagationAllowedTo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L418) | unit/verify | unproven |
| negative | [`checkMEDAcrossAS`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L200) | interop/nightly | unproven |
| negative | [`med-locally-set-reaches-peer.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-locally-set-reaches-peer.ci#L4) | functional/verify | unproven |
| negative | [`med-not-propagated-across-as.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-not-propagated-across-as.ci#L8) | functional/verify | unproven |
| positive | [`TestForwardSuppressesReceivedMEDToAnotherAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L213) | unit/verify | unproven |
| positive | [`checkMEDAcrossAS`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L196) | interop/nightly | unproven |
| positive | [`med-not-propagated-across-as.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-not-propagated-across-as.ci#L4) | functional/verify | unproven |

### [`RFC4271-5.1.4-4`](#rfc4271-5.1.4-4)

A BGP speaker MUST implement a mechanism (based on local configuration) that allows the MULTI_EXIT_DISC attribute to be removed from a route (§5.1.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseModifyDefsMEDRemove`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/modify_test.go#L553) | unit/verify | unproven |
| negative | [`TestMEDRemovalMechanismIsConfigurable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L467) | unit/verify | unproven |
| negative | [`TestMEDRemoveDirectiveIsValueless`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L560) | unit/verify | unproven |
| negative | [`checkMEDRemovalConfiguration`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L322) | interop/nightly | unproven |
| positive | [`TestParseModifyDefsMEDRemove`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/modify_test.go#L548) | unit/verify | unproven |
| positive | [`TestMEDRemovalMechanismIsConfigurable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L460) | unit/verify | unproven |
| positive | [`checkMEDRemovalConfiguration`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L318) | interop/nightly | unproven |
| positive | [`med-removal-configured.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-configured.ci#L4) | functional/verify | unproven |

### [`RFC4271-5.1.4-2`](#rfc4271-5.1.4-2)

MULTI_EXIT_DISC removal from routes MUST be done before the route is used in Phase 2 of the decision process (§5.1.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestHandleFilterUpdateMEDRemoveIsImportOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/modify_test.go#L628) | unit/verify | unproven |
| negative | [`TestMEDRemovalMechanismIsConfigurable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L480) | unit/verify | unproven |
| negative | [`med-removal-before-decision.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-before-decision.ci#L10) | functional/verify | unproven |
| negative | [`med-removal-export-refused.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-export-refused.ci#L4) | functional/verify | unproven |
| positive | [`TestHandleFilterUpdateMEDRemoveIsImportOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/modify_test.go#L621) | unit/verify | unproven |
| positive | [`TestMEDRemovalMechanismIsConfigurable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L472) | unit/verify | unproven |
| positive | [`med-removal-before-decision.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-before-decision.ci#L4) | functional/verify | unproven |
| positive | [`med-removal-configured.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-configured.ci#L8) | functional/verify | unproven |

### [`RFC4271-5.1.5-1`](#rfc4271-5.1.5-1)

LOCAL_PREF SHALL be included in all UPDATE messages sent to internal peers (§5.1.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestForwardLocalPrefStrippedToExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_local_pref_test.go#L45) | unit/verify | unproven |
| negative | [`TestAnnounceStripsLocalPrefTowardExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_origin_test.go#L307) | unit/verify | unproven |
| negative | [`TestRFC4271LocalPrefOmittedForExternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L107) | unit/verify | unproven |
| positive | [`TestRFC4271LocalPrefIncludedForInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L81) | unit/verify | unproven |

### [`RFC4271-5.1.5-2`](#rfc4271-5.1.5-2)

A BGP speaker MUST NOT include LOCAL_PREF in UPDATE messages sent to external peers (§5.1.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLocalPrefAllowedToIsTheOnlyAnswer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_local_pref_test.go#L146) | unit/verify | unproven |
| negative | [`TestRFC4271LocalPrefIncludedForInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L83) | unit/verify | unproven |
| negative | [`local-pref-strip-ebgp.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/local-pref-strip-ebgp.ci#L19) | functional/verify | unproven |
| positive | [`TestForwardLocalPrefStripBeatsAFilterSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_local_pref_test.go#L113) | unit/verify | unproven |
| positive | [`TestForwardLocalPrefStrippedToExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_local_pref_test.go#L41) | unit/verify | unproven |
| positive | [`TestAnnounceStripsLocalPrefTowardExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_origin_test.go#L305) | unit/verify | unproven |
| positive | [`TestRFC4271LocalPrefOmittedForExternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L104) | unit/verify | unproven |
| positive | [`checkLocalPrefStrip`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L96) | interop/nightly | unproven |
| positive | [`local-pref-strip-ebgp.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/local-pref-strip-ebgp.ci#L14) | functional/verify | unproven |

### [`RFC4271-5.1.5-3`](#rfc4271-5.1.5-3)

If LOCAL_PREF is received over EBGP, it MUST be ignored (§5.1.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271LocalPrefIgnoredOnExternalSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L589) | unit/verify | unproven |
| positive | [`TestRFC4271LocalPrefKeptOnInternalSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L566) | unit/verify | unproven |

### [`RFC4271-5.1.5-4`](#rfc4271-5.1.5-4)

The higher degree of preference (LOCAL_PREF) MUST be preferred (§5.1.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBestPath_LocalPref`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L199) | unit/verify | unproven |
| positive | [`TestBestPath_LocalPref`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L197) | unit/verify | unproven |

### [`RFC4271-5.1.6-1`](#rfc4271-5.1.6-1)

A BGP speaker that receives a route with the ATOMIC_AGGREGATE attribute MUST NOT make any NLRI of that route more specific when advertising this route to other BGP speakers (§5.1.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-5.1.6-1, so no unit is bound to it.

### [`RFC4271-6.1-1`](#rfc4271-6.1-1)

All header errors MUST be indicated by sending NOTIFICATION with Error Code Message Header Error (§6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-6.1-1, so no unit is bound to it.

### [`RFC4271-6.1-2`](#rfc4271-6.1-2)

Marker not all ones: Error Subcode MUST be Connection Not Synchronized (§6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-6.1-2, so no unit is bound to it.

### [`RFC4271-6.1-3`](#rfc4271-6.1-3)

Invalid length: Error Subcode MUST be Bad Message Length; Data field MUST contain the erroneous Length field (§6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-6.1-3, so no unit is bound to it.

### [`RFC4271-6.1-4`](#rfc4271-6.1-4)

Invalid type: Error Subcode MUST be Bad Message Type; Data field MUST contain the erroneous Type field (§6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-6.1-4, so no unit is bound to it.

### [`RFC4271-6.2-3`](#rfc4271-6.2-3)

All OPEN errors MUST be indicated by NOTIFICATION with Error Code OPEN Message Error (§6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-6.2-3, so no unit is bound to it.

### [`RFC4271-6.3-1`](#rfc4271-6.3-1)

All UPDATE errors MUST be indicated by NOTIFICATION with Error Code UPDATE Message Error (§6.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConformantUpdateSendsNoUpdateError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L696) | unit/verify | unproven |
| positive | [`TestRFC4271UpdateErrorReportedAsUpdateMessageError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L633) | unit/verify | unproven |

### [`RFC4271-6.7-1`](#rfc4271-6.7-1)

Cease NOTIFICATION MUST NOT be used when a fatal error does exist (§6.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271UpdateErrorReportedAsUpdateMessageError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L636) | unit/verify | unproven |
| positive | [`TestPrefixExceedTeardown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_prefix_test.go#L110) | unit/verify | unproven |

### [`RFC4271-8.2.1-1`](#rfc4271-8.2.1-1)

BGP MUST maintain a separate FSM for each configured peer (§8.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271PerPeerFSMDoesNotShareTimers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L237) | unit/verify | unproven |
| positive | [`TestRFC4271SeparateFSMPerPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L213) | unit/verify | unproven |

### [`RFC4271-8.2.1-2`](#rfc4271-8.2.1-2)

A BGP implementation MUST connect to and listen on TCP port 179 (§8.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ExplicitPortOverridesDefault`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L322) | unit/verify | unproven |
| positive | [`TestRFC4271DefaultBGPPortIs179`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L306) | unit/verify | unproven |

### [`RFC4271-8.2.1-3`](#rfc4271-8.2.1-3)

For each incoming connection, a state machine MUST be instantiated (§8.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-8.2.1-3, so no unit is bound to it.

### [`RFC4271-8.2.2-1`](#rfc4271-8.2.2-1)

Event 10 (HoldTimer_Expires) lists "sends a NOTIFICATION message with the error code Hold Timer Expired" before "drops the TCP connection", in OpenSent, OpenConfirm and Established alike; a peer whose connection is dropped without it is told nothing. §8 states the FSM description is conceptual but binds "the same externally visible behavior", and the NOTIFICATION is the externally visible part, so the level is MUST (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271HoldTimerNotYetExpiredSendsNoNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L836) | unit/verify | unproven |
| positive | [`TestRFC4271HoldTimerExpirySendsNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L807) | unit/verify | unproven |
| positive | [`deadpeer-holddown.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/deadpeer-holddown.ci#L3) | functional/verify | unproven |

### [`RFC4271-8.2.2-2`](#rfc4271-8.2.2-2)

Event 10 (HoldTimer_Expires) lists "sets the ConnectRetryTimer to zero", in OpenSent, OpenConfirm and Established alike. A ConnectRetryTimer left running past the teardown fires against a connection that no longer exists, and the retry it schedules is externally visible as an unexpected connection attempt, so the level is MUST on the same §8 "same externally visible behavior" ground as -1 (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271NoHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1000) | unit/verify | unproven |
| positive | [`TestRFC4271HoldExpiryRunsTheEvent10ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L929) | unit/verify | unproven |

### [`RFC4271-8.2.2-3`](#rfc4271-8.2.2-3)

Event 10 (HoldTimer_Expires) lists "releases all BGP resources", in OpenSent, OpenConfirm and Established alike. The externally visible part is the KeepaliveTimer: a session torn down for silence that keeps its keepalive chain running writes KEEPALIVEs after the teardown, so the level is MUST (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271NoHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1002) | unit/verify | unproven |
| positive | [`TestRFC4271HoldExpiryRunsTheEvent10ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L932) | unit/verify | unproven |

### [`RFC4271-8.2.2-4`](#rfc4271-8.2.2-4)

Event 10 (HoldTimer_Expires) lists "drops the TCP connection", in OpenSent, OpenConfirm and Established alike. The peer must see the connection go away and not merely stop receiving; a half-open socket is externally visible, so the level is MUST (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271NoHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1005) | unit/verify | unproven |
| positive | [`TestRFC4271HoldExpiryRunsTheEvent10ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L935) | unit/verify | unproven |

### [`RFC4271-8.2.2-5`](#rfc4271-8.2.2-5)

Event 10 (HoldTimer_Expires) lists "changes its state to Idle", in OpenSent, OpenConfirm and Established alike. The Established->Idle transition is what deletes the routes learned over the connection, which is externally visible to every other peer, so the level is MUST (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271NoHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1007) | unit/verify | unproven |
| positive | [`TestRFC4271HoldExpiryRunsTheEvent10ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L938) | unit/verify | unproven |

### [`RFC4271-8.2.2-7`](#rfc4271-8.2.2-7)

ManualStart (Event 1) in Idle lists "sets ConnectRetryCounter to zero", once for the Connect branch (Events 1 and 3) and again, in the same words, for the passive Active branch (Events 4 and 5). §8 makes ConnectRetryCounter a mandatory session attribute and defines it as "the number of times a BGP peer has tried to establish a peer session", so an operator start that left a stale retry history behind would misreport that number; the level is MUST (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterSurvivesDampedStart`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L75) | unit/verify | unproven |
| positive | [`TestRFC4271ConnectRetryCounterZeroedOnManualStart`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L42) | unit/verify | unproven |

### [`RFC4271-8.2.2-8`](#rfc4271-8.2.2-8)

ManualStop (Event 2) lists "sets the ConnectRetryCounter to zero", in Connect, Active, OpenSent, OpenConfirm and Established alike. Same §8 mandatory-attribute ground as -7: the operator stopping the peer ends the run of attempts the counter was counting (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterNotZeroedByIdleManualStop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L130) | unit/verify | unproven |
| positive | [`TestRFC4271ConnectRetryCounterZeroedOnManualStop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L108) | unit/verify | unproven |

### [`RFC4271-8.2.2-9`](#rfc4271-8.2.2-9)

HoldTimer_Expires (Event 10) lists "increments the ConnectRetryCounter", in OpenSent, OpenConfirm and Established alike (OpenSent omits the words "by 1" that the other two carry; the counter is an integer count, so the step is one). A session lost to silence is an attempt that failed, and §8 makes the count mandatory (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterQuietOnHealthyEstablishedTraffic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L174) | unit/verify | unproven |
| positive | [`TestRFC4271ConnectRetryCounterIncrementsOnHoldTimerExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L151) | unit/verify | unproven |

### [`RFC4271-8.2.2-10`](#rfc4271-8.2.2-10)

BGPHeaderErr (Event 21) and BGPOpenMsgErr (Event 22) list "increments the ConnectRetryCounter by 1" in Connect, Active, OpenSent and OpenConfirm, and in Established through "In response to any other event (Events 9, 12-13, 20-22)", which names both (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterNotIncrementedByIdleErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L236) | unit/verify | unproven |
| positive | [`TestRFC4271ConnectRetryCounterIncrementsOnHeaderAndOpenErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L210) | unit/verify | unproven |

### [`RFC4271-8.2.2-11`](#rfc4271-8.2.2-11)

NotifMsg (Event 25) leads to "increments the ConnectRetryCounter by 1" in every state that can see it: explicitly in OpenConfirm ("a TcpConnectionFails event (Event 18) [...] or a NOTIFICATION message (Event 25)") and Established ("a NOTIFICATION message (Event 24 or Event 25)"), and through the "any other event" list that names Event 25 in Connect, Active and OpenSent (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterStepsByExactlyOnePerNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L283) | unit/verify | unproven |
| positive | [`TestRFC4271ConnectRetryCounterIncrementsOnNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L258) | unit/verify | unproven |

### [`RFC4271-8.2.2-12`](#rfc4271-8.2.2-12)

NotifMsgVerErr (Event 24) increments the ConnectRetryCounter in Connect, Active and Established, and MUST NOT in OpenSent or OpenConfirm. In Connect and Active the clause sits in the DelayOpenTimer-is-not-running branch, which is the only branch a speaker without a DelayOpenTimer can take, and Established groups Event 24 with Events 25 and 18; in OpenSent and OpenConfirm the Event 24 action list is four items long and has no ConnectRetryCounter line at all (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterQuietOnVersionErrorInOpenStates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L332) | unit/verify | unproven |
| positive | [`TestRFC4271ConnectRetryCounterOnVersionErrorPerState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L308) | unit/verify | unproven |

### [`RFC4271-8.2.2-13`](#rfc4271-8.2.2-13)

TcpConnectionFails (Event 18) increments the ConnectRetryCounter in Active, OpenConfirm and Established, and MUST NOT in Connect or OpenSent. Connect's Event 18 has two branches and neither carries the clause, and OpenSent's Event 18 leaves for Active rather than tearing the peering down (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterQuietOnTCPFailureInConnectAndOpenSent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L378) | unit/verify | unproven |
| positive | [`TestRFC4271ConnectRetryCounterOnTCPFailurePerState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L355) | unit/verify | unproven |

### [`RFC4271-8.2.2-14`](#rfc4271-8.2.2-14)

UpdateMsgErr (Event 28) in Established lists "increments the ConnectRetryCounter by 1". An UPDATE error ends the session, which makes it a failed attempt by the §8 definition of the attribute (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterQuietOnGoodUpdate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L421) | unit/verify | unproven |
| positive | [`TestRFC4271ConnectRetryCounterIncrementsOnUpdateError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L401) | unit/verify | unproven |

### [`RFC4271-8.2.2-15`](#rfc4271-8.2.2-15)

The "any other event" action list lists "increments the ConnectRetryCounter by 1" ("by one" in Active) in all five non-Idle states: Connect and Active (Events 8, 10-11, 13, 19, 23, 25-28), OpenSent (Events 9, 11-13, 20, 25-28), OpenConfirm (Events 9, 12-13, 20, 27-28) and Established (Events 9, 12-13, 20-22). Idle is excluded on purpose: its own "any other event" clause says the event "does not cause change in the state of the local system" and lists no actions (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterIdleDefaultArmCountsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L478) | unit/verify | unproven |
| positive | [`TestRFC4271ConnectRetryCounterIncrementsOnAnyOtherEvent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L443) | unit/verify | unproven |

### [`RFC4271-8.2.2-16`](#rfc4271-8.2.2-16)

AutomaticStop (Event 8) lists "increments the ConnectRetryCounter by 1" in OpenSent, OpenConfirm and Established, and Connect and Active name Event 8 in their "any other events" list, which carries the same line. Its action list differs from ManualStop's by that one line, and the difference is the point: §8 defines the attribute as "the number of times a BGP peer has tried to establish a peer session", so a stop the local system chose records a failed attempt where an operator stop ends the count (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterAutomaticStopIsNotAManualStop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L575) | unit/verify | unproven |
| positive | [`TestRFC4271ConnectRetryCounterIncrementsOnAutomaticStop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L549) | unit/verify | unproven |

### [`RFC4271-8.2.2-17`](#rfc4271-8.2.2-17)

OpenCollisionDump (Event 23) lists "increments the ConnectRetryCounter by 1" in OpenSent, OpenConfirm and Established, and Connect and Active name Event 23 in their "any other events" list, which carries the same line. A connection closed by §6.8 collision resolution is an attempt that did not become a session (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterCollisionDumpIsQuietInIdle`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L630) | unit/verify | unproven |
| positive | [`TestRFC4271ConnectRetryCounterIncrementsOnOpenCollisionDump`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L606) | unit/verify | unproven |

### [`RFC4271-8.2.2-18`](#rfc4271-8.2.2-18)

ManualStop (Event 2) lists the Cease NOTIFICATION FIRST in its action list, in every state that has a connection to send it on: OpenSent writes "sends the NOTIFICATION with a Cease", OpenConfirm and Established write "sends the NOTIFICATION message with a Cease", and all three list it before "drops the TCP connection". The level is MUST on the same §8 ground as -1: the FSM description is conceptual, but an implementation "MUST support the described functionality and exhibit the same externally visible behavior", and the NOTIFICATION is the externally visible part -- it is the ONLY signal that says the operator stopped this speaker rather than the network dropping it. §6.7's MAY governs a different act, a speaker CHOOSING to Cease "at any given time" of its own accord; once the operator has issued Event 2 the action list is what the FSM owes. Connect and Active list no Cease clause and are excluded: neither state holds a BGP connection (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271NoCeaseWithoutAManualStop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/shutdown_notify_test.go#L207) | unit/verify | unproven |
| positive | [`TestShutdownNotifySendsCeaseFromEveryConnectedState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/shutdown_notify_test.go#L174) | unit/verify | unproven |
| positive | [`signal-stop-cease.ci`](https://github.com/ze-software/ze/blob/main/test/reload/signal-stop-cease.ci#L3) | functional/verify | unproven |

### [`RFC4271-10-1`](#rfc4271-10-1)

An implementation MUST allow the HoldTimer to be configurable on a per-peer basis (§10)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271PerPeerHoldTimeSurvivesNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L285) | unit/verify | unproven |
| positive | [`TestRFC4271HoldTimeConfigurablePerPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L261) | unit/verify | unproven |

### [`RFC4271-5-7`](#rfc4271-5-7)

Path attributes SHOULD be ordered in ascending order of attribute type (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSplitMP_PreservesAscendingAttributeOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_split_attr_order_test.go#L72) | unit/verify | unproven |
| positive | [`TestAnnounceBatchRail_AS4PathOrderedAgainstLargeCommunity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_batch_attr_order_test.go#L328) | unit/verify | unproven |
| positive | [`TestAnnounceBatchRail_AscendingTypeCodeOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_batch_attr_order_test.go#L268) | unit/verify | unproven |
| positive | [`TestAnnounceQueuedRail_AscendingTypeCodeOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_batch_attr_order_test.go#L289) | unit/verify | unproven |

### [`RFC4271-6.3-2`](#rfc4271-6.3-2)

Semantically incorrect NEXT_HOP SHOULD be logged and the route SHOULD be ignored (§6.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4271SelfNextHopRouteIsNotInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L52) | unit/verify | unproven |

### [`RFC4271-3.1-2`](#rfc4271-3.1-2)

Next hop for routes in Loc-RIB MUST be resolvable via the local BGP speaker's Routing Table (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-3.1-2, so no unit is bound to it.

### [`RFC4271-5.1.2-2`](#rfc4271-5.1.2-2)

When advertising a route to an internal peer, the speaker SHALL NOT modify the AS_PATH attribute (§5.1.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ASPathPrependedTowardExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L148) | unit/verify | unproven |
| positive | [`TestEstablishedAnnounce_ExplicitASPath_IBGPVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_batch_test.go#L567) | unit/verify | unproven |
| positive | [`TestRFC4271ASPathUnmodifiedTowardInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L131) | unit/verify | unproven |

### [`RFC4271-5.1.2-3`](#rfc4271-5.1.2-3)

When advertising a route to an external peer, the speaker prepends its own AS number to the leading AS_SEQUENCE of AS_PATH (creating one when the path is empty or led by an AS_SET) (§5.1.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEstablishedAnnounce_ExplicitASPath_IBGPVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_batch_test.go#L565) | unit/verify | unproven |
| negative | [`TestASPathSlotPrependOnlyWhenAdvertising`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/advertise_test.go#L99) | unit/verify | unproven |
| negative | [`checkRelayWithdrawalShape`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L432) | interop/nightly | unproven |
| positive | [`TestEstablishedAnnounce_ExplicitASPath_PrependsLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_batch_test.go#L541) | unit/verify | unproven |
| positive | [`TestASPathSlotPrependOnlyWhenAdvertising`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/advertise_test.go#L97) | unit/verify | unproven |
| positive | [`checkRelayWithdrawalShape`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L431) | interop/nightly | unproven |

### [`RFC4271-5.1.4-3`](#rfc4271-5.1.4-3)

If altering MULTI_EXIT_DISC received over EBGP, alteration MUST be done prior to decision process phases 1 and 2 (§5.1.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4271MEDAlterationHappensAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L452) | unit/verify | unproven |

### [`RFC4271-5.1.5-5`](#rfc4271-5.1.5-5)

A BGP speaker SHALL calculate the degree of preference for each external route based on locally-configured policy (§5.1.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271DegreeOfPreferenceNotAHardcodedConstant`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L168) | unit/verify | unproven |
| positive | [`TestRFC4271ExternalRouteDegreeOfPreferenceFromLocalPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L141) | unit/verify | unproven |

### [`RFC4271-5.1.7-1`](#rfc4271-5.1.7-1)

A BGP speaker that performs aggregation and adds AGGREGATOR SHALL include its own AS number and IP address (§5.1.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-5.1.7-1, so no unit is bound to it.

### [`RFC4271-6.7-4`](#rfc4271-6.7-4)

When terminating due to prefix limit, speaker MUST send NOTIFICATION with Error Code Cease (§6.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPrefixExceedDrop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_prefix_test.go#L144) | unit/verify | unproven |
| positive | [`TestPrefixExceedTeardown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_prefix_test.go#L107) | unit/verify | unproven |

### [`RFC4271-6.8-1`](#rfc4271-6.8-1)

In the event of connection collision, one of the connections MUST be closed (§6.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCollisionOpenSentNoCollision`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L177) | unit/verify | unproven |
| positive | [`TestCollisionOpenConfirmLocalWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L127) | unit/verify | unproven |

### [`RFC4271-6.8-2`](#rfc4271-6.8-2)

Upon receipt of an OPEN message, the local system MUST examine all connections in OpenConfirm state for collision (§6.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCollisionNonCollisionStates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L536) | unit/verify | unproven |
| positive | [`TestCollisionOpenConfirmLocalWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L130) | unit/verify | unproven |

### [`RFC4271-9-1`](#rfc4271-9-1)

Withdrawn routes SHALL be removed from the Adj-RIB-In and the Decision Process SHALL be run (§9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271WithdrawRemovesFromAdjRIBIn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L220) | unit/verify | unproven |
| positive | [`TestRFC4271WithdrawRemovesFromAdjRIBIn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L215) | unit/verify | unproven |

### [`RFC4271-9-2`](#rfc4271-9-2)

A new route with identical NLRI to an existing route SHALL replace the older route in Adj-RIB-In (§9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271WithdrawRemovesFromAdjRIBIn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L217) | unit/verify | unproven |
| positive | [`TestRFC4271SamePrefixReplacesRatherThanAccumulates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L189) | unit/verify | unproven |

### [`RFC4271-9-3`](#rfc4271-9-3)

Once the Adj-RIB-In is updated, the speaker SHALL run its Decision Process (§9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRIBBestChangeNoPublishSameBest`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L649) | unit/verify | unproven |
| positive | [`TestRIBBestChangeWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L685) | unit/verify | unproven |

### [`RFC4271-9.1.1-1`](#rfc4271-9.1.1-1)

The degree of preference function SHALL NOT use the existence or attributes of other routes as inputs (§9.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271DegreeOfPreferenceFollowsOwnAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L68) | unit/verify | unproven |
| positive | [`TestRFC4271DegreeOfPreferenceIgnoresOtherRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L33) | unit/verify | unproven |

### [`RFC4271-9.1.1-2`](#rfc4271-9.1.1-2)

For external routes, the computed degree of preference MUST be used as the LOCAL_PREF value in IBGP readvertisement (§9.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271LocalPrefOmittedForExternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L110) | unit/verify | unproven |
| positive | [`TestRFC4271LocalPrefIncludedForInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L86) | unit/verify | unproven |

### [`RFC4271-9.1.2-1`](#rfc4271-9.1.2-1)

If NEXT_HOP is not resolvable, the BGP route MUST be excluded from Phase 2 decision function (§9.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.1.2-1, so no unit is bound to it.

### [`RFC4271-9.1.2-2`](#rfc4271-9.1.2-2)

The local speaker SHALL install the best route in the Loc-RIB (§9.1.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRIBBestChangeWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L690) | unit/verify | unproven |
| positive | [`TestLocRIBMirror`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L1512) | unit/verify | unproven |

### [`RFC4271-9.1.2-3`](#rfc4271-9.1.2-3)

The local speaker MUST determine the immediate next-hop address from the NEXT_HOP attribute (§9.1.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271LocRIBNextHopComesFromNextHopAttribute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L94) | unit/verify | unproven |
| positive | [`TestLocRIBMirror`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L1514) | unit/verify | unproven |

### [`RFC4271-9.1.2-4`](#rfc4271-9.1.2-4)

If immediate next-hop or IGP cost to NEXT_HOP changes, Phase 2 Route Selection MUST be performed again (§9.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.1.2-4, so no unit is bound to it.

### [`RFC4271-9.1.2.1-1`](#rfc4271-9.1.2.1-1)

When installing a BGP route in the Routing Table, implementations MUST recalculate and take into account next-hops (§9.1.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271LocRIBNextHopComesFromNextHopAttribute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L98) | unit/verify | unproven |
| positive | [`TestLocRIBMirror`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L1517) | unit/verify | unproven |

### [`RFC4271-9.1.2.1-2`](#rfc4271-9.1.2.1-2)

Unresolvable routes SHALL be removed from the Loc-RIB and the routing table (§9.1.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.1.2.1-2, so no unit is bound to it.

### [`RFC4271-9.1.2.2-1`](#rfc4271-9.1.2.2-1)

The tie-breaking criteria MUST be applied in the order specified (§9.1.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBestPath_MED_SameNeighborAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L315) | unit/verify | unproven |
| negative | [`TestBestPathEqualBGPIdentifiersOnTheJSONRailFallThroughToPeerAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_bgp_identifier_json_test.go#L106) | unit/verify | unproven |
| negative | [`TestBestPathEqualBGPIdentifiersFallThroughToPeerAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_bgp_identifier_test.go#L116) | unit/verify | unproven |
| positive | [`TestBestPath_FullTiebreak`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L530) | unit/verify | unproven |
| positive | [`TestBestPathStepFComparesThePeerBGPIdentifierOnTheJSONRail`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_bgp_identifier_json_test.go#L78) | unit/verify | unproven |
| positive | [`TestBestPathStepFComparesThePeerBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_bgp_identifier_test.go#L88) | unit/verify | unproven |

### [`RFC4271-9.1.2.2-2`](#rfc4271-9.1.2.2-2)

If MULTI_EXIT_DISC is removed before IBGP readvertisement, the optional MED comparison MUST be performed only among EBGP-learned routes (§9.1.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.1.2.2-2, so no unit is bound to it.

### [`RFC4271-9.1.2.2-3`](#rfc4271-9.1.2.2-3)

For IBGP-learned routes, MULTI_EXIT_DISC MUST be used in comparisons that reach the MED step (§9.1.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBestPath_MED_SameNeighborAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L322) | unit/verify | unproven |
| positive | [`TestBestPath_MED_SameNeighborAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L319) | unit/verify | unproven |

### [`RFC4271-9.2-2`](#rfc4271-9.2-2)

A route SHALL NOT be installed in Adj-RIB-Out unless its destination and NEXT_HOP may be forwarded by the Routing Table (§9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.2-2, so no unit is bound to it.

### [`RFC4271-9.2-3`](#rfc4271-9.2-3)

If a route in Loc-RIB is excluded from a particular Adj-RIB-Out, the previously advertised route MUST be withdrawn (§9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.2-3, so no unit is bound to it.

### [`RFC4271-9.2-4`](#rfc4271-9.2-4)

The Decision Process MUST consider both overlapping routes based on acceptance policy (§9.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271SamePrefixReplacesRatherThanAccumulates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L182) | unit/verify | unproven |
| positive | [`TestRFC4271OverlappingRoutesBothInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L148) | unit/verify | unproven |

### [`RFC4271-9.2-5`](#rfc4271-9.2-5)

If both less and more specific overlapping routes are accepted, the Decision Process MUST install both or an aggregate in Loc-RIB (§9.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271SamePrefixReplacesRatherThanAccumulates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L186) | unit/verify | unproven |
| positive | [`TestRFC4271OverlappingRoutesBothInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L151) | unit/verify | unproven |

### [`RFC4271-9.2.1.1-2`](#rfc4271-9.2.1.1-2)

Two UPDATE messages advertising to common destinations MUST be separated by at least MinRouteAdvertisementIntervalTimer (§9.2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.2.1.1-2, so no unit is bound to it.

### [`RFC4271-9.2.2.2-1`](#rfc4271-9.2.2.2-1)

Routes with different MULTI_EXIT_DISC attributes SHALL NOT be aggregated (§9.2.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.2.2.2-1, so no unit is bound to it.

### [`RFC4271-9.2.2.2-2`](#rfc4271-9.2.2.2-2)

If any aggregated route has ORIGIN INCOMPLETE, the aggregate MUST have ORIGIN INCOMPLETE; else if any has EGP, the aggregate MUST have ORIGIN EGP (§9.2.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.2.2.2-2, so no unit is bound to it.

### [`RFC4271-9.2.2.2-3`](#rfc4271-9.2.2.2-3)

When aggregating routes with different NEXT_HOP, the aggregated NEXT_HOP SHALL identify an interface on the aggregating speaker (§9.2.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.2.2.2-3, so no unit is bound to it.

### [`RFC4271-9.2.2.2-4`](#rfc4271-9.2.2.2-4)

If at least one aggregated route has ATOMIC_AGGREGATE, the aggregate SHALL have it as well (§9.2.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.2.2.2-4, so no unit is bound to it.

### [`RFC4271-9.2.2.2-5`](#rfc4271-9.2.2.2-5)

AGGREGATOR attributes from aggregated routes MUST NOT be included in the aggregated route (§9.2.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.2.2.2-5, so no unit is bound to it.

### [`RFC4271-Security-1`](#rfc4271-security-1)

A BGP implementation MUST support TCP MD5 authentication (RFC 2385) (§Security, Appendix E)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMD5PeersForListener`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_test.go#L2360) | unit/verify | unproven |
| positive | [`TestMD5PeersForListener`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_test.go#L2357) | unit/verify | unproven |

### [`RFC4271-9.2.1.1-3`](#rfc4271-9.2.1.1-3)

The last route selected while awaiting MinRouteAdvertisementIntervalTimer SHALL be advertised at expiry (§9.2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.2.1.1-3, so no unit is bound to it.

### [`RFC4271-9.2-6`](#rfc4271-9.2-6)

A BGP speaker SHALL NOT redistribute routing information from an internal peer to other internal peers (unless route reflector) (§9.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271IBGPRedistributionAllowedForReflectorClient`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L567) | unit/verify | unproven |
| positive | [`TestRFC4271NoIBGPToIBGPRedistribution`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L508) | unit/verify | unproven |

### [`RFC4271-9.2-7`](#rfc4271-9.2-7)

Newly unfeasible routes for which there is no replacement SHALL be advertised via UPDATE (§9.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRIBBestChangeNoPublishSameBest`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L652) | unit/verify | unproven |
| positive | [`TestRIBBestChangeWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L688) | unit/verify | unproven |

### [`RFC4271-9.2-8`](#rfc4271-9.2-8)

Any routes in the Loc-RIB marked as unfeasible SHALL be removed (§9.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRIBBestChangeNoPublishSameBest`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L654) | unit/verify | unproven |
| positive | [`TestLocRIBMirror`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L1520) | unit/verify | unproven |

### [`RFC4271-9.2-9`](#rfc4271-9.2-9)

Changes to reachable destinations within the speaker's own AS SHALL be advertised in an UPDATE (§9.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271OwnASUnreachabilityChangeAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L426) | unit/verify | unproven |
| positive | [`TestRFC4271OwnASReachabilityChangeAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L398) | unit/verify | unproven |

### [`RFC4271-9.2-10`](#rfc4271-9.2-10)

If a single route does not fit in an UPDATE message, the speaker MUST NOT advertise it and MAY log an error (§9.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271FittingRouteIsAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L431) | unit/verify | unproven |
| positive | [`TestRFC4271OversizeSingleRouteNotAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L402) | unit/verify | unproven |

### [`RFC4271-9.1.2-5`](#rfc4271-9.1.2-5)

If the AS_PATH attribute of a BGP route contains an AS loop, the BGP route should be excluded from the Phase 2 decision function (§9.1.2). Detection scans the full AS path and checks that the local autonomous system number does not appear in it. RFC 4271 writes this keyword in lower case, so the level is a recommendation and not a capitalized RFC 2119 SHOULD. The same paragraph places a speaker configured to accept routes with its own autonomous system number in the AS path outside the scope of the document. That out-of-scope case is what the allow-own-as setting selects, so a non-zero allow-own-as is not a deviation from this line.

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDetectASLoop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/filter/loop_test.go#L114) | unit/verify | unproven |
| negative | [`TestDetectASLoop_ASSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/filter/loop_test.go#L125) | unit/verify | unproven |
| negative | [`loop-as.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/loop-as.ci#L7) | functional/verify | unproven |
| positive | [`TestDetectASLoop_NotPresent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/filter/loop_test.go#L136) | unit/verify | unproven |

### [`RFC4271-4.3-7`](#rfc4271-4.3-7)

An UPDATE containing the same prefix in WITHDRAWN and NLRI SHOULD be treated as if the prefix is not in WITHDRAWN (§4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRIBInjectSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L121) | unit/verify | unproven |
| positive | [`TestRIBPoolPathSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L87) | unit/verify | unproven |
| positive | [`TestRIBSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L42) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 4271, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 4271, so its obligations are stated where they were written.
