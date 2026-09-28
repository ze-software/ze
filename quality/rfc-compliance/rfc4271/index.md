# RFC 4271 - A Border Gateway Protocol 4 (BGP-4)

Partial. Every requirement this repository extracted from RFC 4271, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 80.0% | 100 of 125 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 2.4% | 3 of 125 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 125 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 9.3% | 27 of 289 tagged units, 0 escaped and 2 lapsed | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 125 | of 159 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 7 | of 125 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 5.6% | 7 of 125 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 125 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 125 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 12.0% | 15 of 125 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 80 | of 125 gated MUSTs judged | 58 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 125 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 159 |
| Gated MUST-level | 125 |
| Not applicable, so out of scope | 7 |
| Declared gaps | 15 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 289 |
| Tagged units | 289 |
| Recorded audit verdicts | 80 |
| Discrimination records | 29 |
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
- the Section 5 and Section 9 pass-along rule for an attribute ze does not recognize, which sets the Partial bit to 1 on an unrecognized transitive optional attribute at receipt, on the bytes ze retains and relays ([`RFC4271-5-3`](#rfc4271-5-3), `publishBase`, [`internal/component/bgp/reactor/session_validation.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation.go), and `SetPartialOnUnrecognizedTransitive`, [`internal/core/bgp/attribute/partial.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/partial.go)), while an attribute ze does recognize keeps the bit the sender chose and a bit an earlier AS set on an optional transitive attribute is never cleared
- the Section 4.3 companion rule on the same octet, which clears the Partial bit on a well-known attribute and on an optional non-transitive one at the same site ([`RFC4271-4.3-2`](#rfc4271-4.3-2), `ClearPartialOnWellKnownAndNonTransitive`, [`internal/core/bgp/attribute/partial.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/partial.go)), because RFC 7606 Section 3(c) accepts such an octet on receipt and the route-server relay then copies it onward byte for byte
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
- **Connections:** [`RFC4271-8.2.1-3`](#rfc4271-8.2.1-3) (an inbound connection reuses the peer's session rather than getting its own FSM, [`internal/component/bgp/reactor/reactor_connection.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_connection.go)). Seven further requirements are recorded {not-applicable}: ze performs no route aggregation and never disaggregates a received route. The 2026-09-21 extraction walk added twenty-four rows this checklist had never carried, and none of them is tagged yet: the six OPEN Error Subcode rules of Section 6.2 ([`RFC4271-6.2-5`](#rfc4271-6.2-5) to 6.2-10), the fourteen UPDATE Error Subcode and Data-field rules of Section 6.3 ([`RFC4271-6.3-4`](#rfc4271-6.3-4) to 6.3-17), the Section 5 pass-along rule for an updated well-known attribute ([`RFC4271-5-8`](#rfc4271-5-8)), the Section 8.2.2 tracking of a second connection until it sends an OPEN ([`RFC4271-8.2.2-19`](#rfc4271-8.2.2-19)), the Section 9 placement of a new route in the Adj-RIB-In ([`RFC4271-9-4`](#rfc4271-9-4)) and the Section 10 jitter factor ([`RFC4271-10-4`](#rfc4271-10-4)). Whether ze produces each behavior is unassessed; the rows record the obligation, not a verdict.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 100 | one part of the gated population |
| Annotated instead of tested | 25 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **125** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (100):** [`RFC4271-4.1-1`](#rfc4271-4.1-1), [`RFC4271-4.1-2`](#rfc4271-4.1-2), [`RFC4271-4.1-3`](#rfc4271-4.1-3), [`RFC4271-4.3-1`](#rfc4271-4.3-1), [`RFC4271-4.3-2`](#rfc4271-4.3-2), [`RFC4271-4.3-4`](#rfc4271-4.3-4), [`RFC4271-4.4-1`](#rfc4271-4.4-1), [`RFC4271-4.4-2`](#rfc4271-4.4-2), [`RFC4271-6-1`](#rfc4271-6-1), [`RFC4271-4.2-1`](#rfc4271-4.2-1), [`RFC4271-4.2-2`](#rfc4271-4.2-2), [`RFC4271-6.2-1`](#rfc4271-6.2-1), [`RFC4271-6.2-2`](#rfc4271-6.2-2), [`RFC4271-5-1`](#rfc4271-5-1), [`RFC4271-5-2`](#rfc4271-5-2), [`RFC4271-5-3`](#rfc4271-5-3), [`RFC4271-5-4`](#rfc4271-5-4), [`RFC4271-5-5`](#rfc4271-5-5), [`RFC4271-5-6`](#rfc4271-5-6), [`RFC4271-5.1.3-1`](#rfc4271-5.1.3-1), [`RFC4271-5.1.3-2`](#rfc4271-5.1.3-2), [`RFC4271-5.1.3-3`](#rfc4271-5.1.3-3), [`RFC4271-5.1.4-1`](#rfc4271-5.1.4-1), [`RFC4271-5.1.4-4`](#rfc4271-5.1.4-4), [`RFC4271-5.1.4-2`](#rfc4271-5.1.4-2), [`RFC4271-5.1.5-1`](#rfc4271-5.1.5-1), [`RFC4271-5.1.5-2`](#rfc4271-5.1.5-2), [`RFC4271-5.1.5-3`](#rfc4271-5.1.5-3), [`RFC4271-5.1.5-4`](#rfc4271-5.1.5-4), [`RFC4271-6.3-1`](#rfc4271-6.3-1), [`RFC4271-6.7-1`](#rfc4271-6.7-1), [`RFC4271-8.2.1-1`](#rfc4271-8.2.1-1), [`RFC4271-8.2.1-2`](#rfc4271-8.2.1-2), [`RFC4271-8.2.2-1`](#rfc4271-8.2.2-1), [`RFC4271-8.2.2-2`](#rfc4271-8.2.2-2), [`RFC4271-8.2.2-3`](#rfc4271-8.2.2-3), [`RFC4271-8.2.2-4`](#rfc4271-8.2.2-4), [`RFC4271-8.2.2-5`](#rfc4271-8.2.2-5), [`RFC4271-8.2.2-7`](#rfc4271-8.2.2-7), [`RFC4271-8.2.2-8`](#rfc4271-8.2.2-8), [`RFC4271-8.2.2-9`](#rfc4271-8.2.2-9), [`RFC4271-8.2.2-10`](#rfc4271-8.2.2-10), [`RFC4271-8.2.2-11`](#rfc4271-8.2.2-11), [`RFC4271-8.2.2-12`](#rfc4271-8.2.2-12), [`RFC4271-8.2.2-13`](#rfc4271-8.2.2-13), [`RFC4271-8.2.2-14`](#rfc4271-8.2.2-14), [`RFC4271-8.2.2-15`](#rfc4271-8.2.2-15), [`RFC4271-8.2.2-16`](#rfc4271-8.2.2-16), [`RFC4271-8.2.2-17`](#rfc4271-8.2.2-17), [`RFC4271-8.2.2-18`](#rfc4271-8.2.2-18), [`RFC4271-10-1`](#rfc4271-10-1), [`RFC4271-5.1.2-2`](#rfc4271-5.1.2-2), [`RFC4271-5.1.2-3`](#rfc4271-5.1.2-3), [`RFC4271-5.1.5-5`](#rfc4271-5.1.5-5), [`RFC4271-6.7-4`](#rfc4271-6.7-4), [`RFC4271-6.8-1`](#rfc4271-6.8-1), [`RFC4271-6.8-2`](#rfc4271-6.8-2), [`RFC4271-9-1`](#rfc4271-9-1), [`RFC4271-9-2`](#rfc4271-9-2), [`RFC4271-9-3`](#rfc4271-9-3), [`RFC4271-9.1.1-1`](#rfc4271-9.1.1-1), [`RFC4271-9.1.1-2`](#rfc4271-9.1.1-2), [`RFC4271-9.1.2-2`](#rfc4271-9.1.2-2), [`RFC4271-9.1.2-3`](#rfc4271-9.1.2-3), [`RFC4271-9.1.2.1-1`](#rfc4271-9.1.2.1-1), [`RFC4271-9.1.2.2-1`](#rfc4271-9.1.2.2-1), [`RFC4271-9.1.2.2-3`](#rfc4271-9.1.2.2-3), [`RFC4271-9.1.2.2-4`](#rfc4271-9.1.2.2-4), [`RFC4271-9.2-4`](#rfc4271-9.2-4), [`RFC4271-9.2-5`](#rfc4271-9.2-5), [`RFC4271-Security-1`](#rfc4271-security-1), [`RFC4271-9.2-6`](#rfc4271-9.2-6), [`RFC4271-9.2-7`](#rfc4271-9.2-7), [`RFC4271-9.2-8`](#rfc4271-9.2-8), [`RFC4271-9.2-9`](#rfc4271-9.2-9), [`RFC4271-9.2-10`](#rfc4271-9.2-10), [`RFC4271-5-8`](#rfc4271-5-8), [`RFC4271-6.2-5`](#rfc4271-6.2-5), [`RFC4271-6.2-6`](#rfc4271-6.2-6), [`RFC4271-6.2-7`](#rfc4271-6.2-7), [`RFC4271-6.2-8`](#rfc4271-6.2-8), [`RFC4271-6.2-9`](#rfc4271-6.2-9), [`RFC4271-6.2-10`](#rfc4271-6.2-10), [`RFC4271-6.3-4`](#rfc4271-6.3-4), [`RFC4271-6.3-5`](#rfc4271-6.3-5), [`RFC4271-6.3-6`](#rfc4271-6.3-6), [`RFC4271-6.3-7`](#rfc4271-6.3-7), [`RFC4271-6.3-8`](#rfc4271-6.3-8), [`RFC4271-6.3-9`](#rfc4271-6.3-9), [`RFC4271-6.3-10`](#rfc4271-6.3-10), [`RFC4271-6.3-11`](#rfc4271-6.3-11), [`RFC4271-6.3-12`](#rfc4271-6.3-12), [`RFC4271-6.3-13`](#rfc4271-6.3-13), [`RFC4271-6.3-14`](#rfc4271-6.3-14), [`RFC4271-6.3-15`](#rfc4271-6.3-15), [`RFC4271-6.3-16`](#rfc4271-6.3-16), [`RFC4271-6.3-17`](#rfc4271-6.3-17), [`RFC4271-8.2.2-19`](#rfc4271-8.2.2-19), [`RFC4271-9-4`](#rfc4271-9-4), [`RFC4271-10-4`](#rfc4271-10-4)

**Annotated instead of tested (25):** [`RFC4271-4.3-3`](#rfc4271-4.3-3), [`RFC4271-4.3-5`](#rfc4271-4.3-5), [`RFC4271-5.1.6-1`](#rfc4271-5.1.6-1), [`RFC4271-6.1-1`](#rfc4271-6.1-1), [`RFC4271-6.1-2`](#rfc4271-6.1-2), [`RFC4271-6.1-3`](#rfc4271-6.1-3), [`RFC4271-6.1-4`](#rfc4271-6.1-4), [`RFC4271-6.2-3`](#rfc4271-6.2-3), [`RFC4271-8.2.1-3`](#rfc4271-8.2.1-3), [`RFC4271-3.1-2`](#rfc4271-3.1-2), [`RFC4271-5.1.4-3`](#rfc4271-5.1.4-3), [`RFC4271-5.1.7-1`](#rfc4271-5.1.7-1), [`RFC4271-9.1.2-1`](#rfc4271-9.1.2-1), [`RFC4271-9.1.2-4`](#rfc4271-9.1.2-4), [`RFC4271-9.1.2.1-2`](#rfc4271-9.1.2.1-2), [`RFC4271-9.1.2.2-2`](#rfc4271-9.1.2.2-2), [`RFC4271-9.2-2`](#rfc4271-9.2-2), [`RFC4271-9.2-3`](#rfc4271-9.2-3), [`RFC4271-9.2.1.1-2`](#rfc4271-9.2.1.1-2), [`RFC4271-9.2.2.2-1`](#rfc4271-9.2.2.2-1), [`RFC4271-9.2.2.2-2`](#rfc4271-9.2.2.2-2), [`RFC4271-9.2.2.2-3`](#rfc4271-9.2.2.2-3), [`RFC4271-9.2.2.2-4`](#rfc4271-9.2.2.2-4), [`RFC4271-9.2.2.2-5`](#rfc4271-9.2.2.2-5), [`RFC4271-9.2.1.1-3`](#rfc4271-9.2.1.1-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4271-4.1-1` | This 16-octet field is included for compatibility; it MUST be set to all ones. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4271MarkerAllOnesOnSend`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L19). **negative:** `unit/verify` [`TestRFC4271MarkerNotAllOnesRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L42) |
| `RFC4271-4.1-2` | Therefore, the Length field MUST have the smallest value required, given the rest of the message. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4271SmallestLengthOnSend`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L62). **negative:** `unit/verify` [`TestRFC4271NonSmallestLengthRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L95) |
| `RFC4271-4.1-3` | The value of the Length field MUST always be at least 19 and no greater than 4096 (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4271MessageLengthWithinBounds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L124). **negative:** `unit/verify` [`TestRFC4271MessageLengthOutOfBounds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L150) |
| `RFC4271-4.3-1` | For well-known attributes, the Transitive bit MUST be set to 1 (§4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC4271WellKnownAttributesAreTransitive`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L31). **negative:** `unit/verify` [`TestRFC4271WellKnownAttributeErrorsAreCaught`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L500) |
| `RFC4271-4.3-2` | For well-known attributes and for optional non-transitive attributes, the Partial bit MUST be set to 0. (§4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC4271PartialBitClearOnSend`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L55). **positive:** `unit/verify` [`TestRFC4271PartialClearedOnTheRelayedWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_relay_partial_test.go#L48). **positive:** `unit/verify` [`TestRFC4271PartialNotSetOnRecognizedOrNonTransitive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1116). **negative:** `unit/verify` [`TestRFC4271PartialBitClearedOnReadvertisedWellKnown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L47). **negative:** `unit/verify` [`TestRFC4271PartialClearedWhenTheRailReadvertises`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc4271_partial_test.go#L75). **negative:** `unit/verify` [`TestRFC4271PartialNotStampedOnExcludedClasses`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L170) |
| `RFC4271-4.3-3` | The lower-order four bits of the Attribute Flags octet are unused. They MUST be zero when sent (§4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC4271AttributeFlagsLowNibbleZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L80). **negative:** no negative test. **{single-polarity}:** the obligation is on the sender -- the flags octet ze writes must have its low-order four bits zero -- so there is no non-conformant input to reject. The receive-side mirror of the same rule ("MUST be ignored when received") is RFC4271-4.3-4 and is proven both ways there |
| `RFC4271-4.3-4` | The lower-order four bits of the Attribute Flags octet are unused. They MUST be zero when sent and MUST be ignored when received. (§4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC4271AttrFlagsLowNibbleIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L232). **negative:** `unit/verify` [`TestRFC4271AttrFlagsHighBitsNotIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L259) |
| `RFC4271-4.4-1` | KEEPALIVE messages MUST NOT be sent more frequently than one per second (§4.4) | MUST NOT | 4.4 | **positive:** `unit/verify` [`TestRFC4271KeepaliveNotFasterThanOnePerSecond`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_test.go#L18). **negative:** `unit/verify` [`TestRFC4271KeepaliveIntervalNeverSubSecond`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_test.go#L50) |
| `RFC4271-4.4-2` | If the negotiated Hold Time interval is zero, then periodic KEEPALIVE messages MUST NOT be sent. (§4.4) | MUST NOT | 4.4 | **positive:** `unit/verify` [`TestTimersKeepaliveTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/timer_test.go#L144). **negative:** `unit/verify` [`TestKeepaliveWithZeroHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/timer_test.go#L365) |
| `RFC4271-6-1` | If no Error Subcode is specified, then a zero MUST be used. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC4271NotificationUnspecifiedSubcodeIsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L353). **negative:** `unit/verify` [`TestRFC4271NotificationSpecifiedSubcodePreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L376) |
| `RFC4271-4.2-1` | Hold Time MUST be either zero or at least three seconds (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestOpenValidateHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L339). **negative:** `unit/verify` [`TestOpenValidateHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L341). **positive:** `functional/verify` [`open-hold-time-peer-lower-wins.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/open-hold-time-peer-lower-wins.ci#L3) |
| `RFC4271-4.2-2` | Upon receipt of an OPEN message, a BGP speaker MUST calculate the value of the Hold Timer by using the smaller of its configured Hold Time and the Hold Time received in the OPEN message. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestNegotiateWith_HoldTimeMinOfBoth`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L45). **negative:** `unit/verify` [`TestNegotiateWith_HoldTimeZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L74) |
| `RFC4271-6.2-1` | An implementation MUST reject Hold Time values of one or two seconds (§6.2) | MUST | 6.2 | **positive:** `unit/verify` [`TestOpenValidateHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L343). **negative:** `unit/verify` [`TestOpenValidateHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L345) |
| `RFC4271-6.2-2` | An implementation that accepts a Hold Time MUST use the negotiated value (§6.2) | MUST | 6.2 | **positive:** `unit/verify` [`TestRFC4271NegotiatedHoldTimeDrivesTimers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L171). **negative:** `unit/verify` [`TestRFC4271LocalHoldTimeNotUsedWhenPeerProposesSmaller`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L192) |
| `RFC4271-5-1` | BGP implementations MUST recognize all well-known attributes (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC4271WellKnownAttributesAreRecognized`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L463). **negative:** `unit/verify` [`TestRFC4271WellKnownAttributeErrorsAreCaught`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L492) |
| `RFC4271-5-2` | Some of these attributes are mandatory and MUST be included in every UPDATE message that contains NLRI. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC4271WellKnownAttributesAreRecognized`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L466). **negative:** `unit/verify` [`TestRFC4271WellKnownAttributeErrorsAreCaught`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L497) |
| `RFC4271-5-3` | If a path with an unrecognized transitive optional attribute is accepted and passed to other BGP peers, then the unrecognized transitive optional attribute of that path MUST be passed, along with the path, to other BGP peers with the Partial bit in the Attribute Flags octet set to 1. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC4271PartialSetOnUnrecognizedTransitiveOptional`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1084). **positive:** `unit/verify` [`TestRFC4271PartialStampedOnUnrecognizedTransitive`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L132). **negative:** `unit/verify` [`TestRFC4271PartialNotSetOnRecognizedOrNonTransitive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1113). **negative:** `unit/verify` [`TestRFC4271PartialNotStampedOnExcludedClasses`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L167). **positive:** `functional/verify` [`rfc4271-partial-unknown-transitive.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc4271-partial-unknown-transitive.ci#L27) |
| `RFC4271-5-4` | If a path with a recognized, transitive optional attribute is accepted and passed along to other BGP peers and the Partial bit in the Attribute Flags octet is set to 1 by some previous AS, it MUST NOT be set back to 0 by the current AS. (§5) | MUST NOT | 5 | **positive:** `unit/verify` [`TestRFC4271PartialBitPreservedOnUnknownTransitive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L82). **positive:** `unit/verify` [`TestRFC4271PartialFromPreviousASNeverCleared`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1152). **negative:** `unit/verify` [`TestRFC4271PartialBitSurvivesLengthReframing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L115). **negative:** `unit/verify` [`TestRFC4271PartialFromPreviousASNotCleared`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L197) |
| `RFC4271-5-5` | Unrecognized non-transitive optional attributes MUST be quietly ignored (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC4271UnrecognizedNonTransitiveIsNotPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1210). **negative:** `unit/verify` [`TestRFC4271TheNonTransitiveDropSparesEveryOtherClass`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1251) |
| `RFC4271-5-6` | The receiver of an UPDATE message MUST be prepared to handle path attributes within UPDATE messages that are out of order. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC4271AttributesOutOfOrderAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L293). **negative:** `unit/verify` [`TestRFC4271OutOfOrderDoesNotMaskMalformation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L323) |
| `RFC4271-4.3-5` | However, a BGP speaker MUST be able to process UPDATE messages in this form. (§4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRIBInjectSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L119). **positive:** `unit/verify` [`TestRIBPoolPathSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L85). **positive:** `unit/verify` [`TestRIBSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L39). **negative:** no negative test. **{single-polarity}:** the obligation is to ACCEPT a message shape, so there is no non-conformant input to reject -- every UPDATE of this form must be processed, and a negative case would have to assert the absence of an error, which proves nothing (ai/rules/testing.md). The consequence the same paragraph asks for, treating the UPDATE as though WITHDRAWN did not contain the prefix, is RFC4271-4.3-7 and is proven by the same test |
| `RFC4271-5.1.3-1` | A route originated by a BGP speaker SHALL NOT be advertised to a peer using an address of that peer as NEXT_HOP. (§5.1.3) | SHALL NOT | 5.1.3 | **positive:** `unit/verify` [`TestEgressNextHopIsPeerOwnReadsTheRewrittenAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L282). **positive:** `unit/verify` [`TestForwardRSWithholdsRouteWhoseNextHopIsTheClientsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L252). **positive:** `unit/verify` [`TestForwardWithdrawsFromDestinationWhoseNextHopIsItsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L217). **positive:** `unit/verify` [`TestForwardWithholdsRouteWhoseNextHopIsTheDestinationsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L181). **positive:** `unit/verify` [`TestSendAnnounceWithholdsRouteWithPeerOwnNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L464). **positive:** `unit/verify` [`TestSendUpdateWithholdsOriginatedRouteWithPeerOwnNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L425). **negative:** `unit/verify` [`TestEgressNextHopIsPeerOwnReadsTheRewrittenAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L286). **negative:** `unit/verify` [`TestForwardRSWithholdsRouteWhoseNextHopIsTheClientsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L255). **negative:** `unit/verify` [`TestForwardWithholdsRouteWhoseNextHopIsTheDestinationsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L186). **negative:** `unit/verify` [`TestSendAnnounceWithholdsRouteWithPeerOwnNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L466). **negative:** `unit/verify` [`TestSendUpdateWithholdsOriginatedRouteWithPeerOwnNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L429). **positive:** `functional/verify` [`originated-nexthop-peer-own.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/originated-nexthop-peer-own.ci#L7). **negative:** `functional/verify` [`originated-nexthop-peer-own.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/originated-nexthop-peer-own.ci#L10). **positive:** `interop/nightly` [`checkSelfNextHopWithheld`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L959). **negative:** `interop/nightly` [`checkSelfNextHopWithheld`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L960) |
| `RFC4271-5.1.3-2` | A BGP speaker SHALL NOT install a route with itself as the next hop (§5.1.3) | SHALL NOT | 5.1.3 | **positive:** `unit/verify` [`TestRFC4271SelfNextHopRouteIsNotInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L43). **positive:** `unit/verify` [`TestRFC4271SelfNextHopSetComesFromPeerEvents`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L122). **negative:** `unit/verify` [`TestRFC4271SelfNextHopDoesNotShadowASoundAlternative`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L89). **negative:** `unit/verify` [`TestRFC4271SelfNextHopRouteIsNotInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L47). **negative:** `unit/verify` [`TestRFC4271SelfNextHopSetComesFromPeerEvents`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L126) |
| `RFC4271-5.1.3-3` | A BGP speaker MUST be able to support the disabling advertisement of third party NEXT_HOP attributes in order to handle imperfectly bridged media. (§5.1.3) | MUST | 5.1.3 | **positive:** `unit/verify` [`TestRFC4271ThirdPartyNextHopCanBeDisabled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L341). **negative:** `unit/verify` [`TestRFC4271ThirdPartyNextHopDisableFailsClosed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L373) |
| `RFC4271-5.1.4-1` | The MULTI_EXIT_DISC attribute received from a neighboring AS MUST NOT be propagated to other neighboring ASes. (§5.1.4) | MUST NOT | 5.1.4 | **positive:** `unit/verify` [`TestForwardSuppressesReceivedMEDToAnotherAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L221). **negative:** `unit/verify` [`TestForwardKeepsFilterSetMED`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L322). **negative:** `unit/verify` [`TestForwardSuppressesReceivedMEDToAnotherAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L226). **negative:** `unit/verify` [`TestForwardWritesLocallySetMED`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L289). **negative:** `unit/verify` [`TestMEDPropagationAllowedTo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L426). **positive:** `functional/verify` [`med-not-propagated-across-as.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-not-propagated-across-as.ci#L4). **negative:** `functional/verify` [`med-locally-set-reaches-peer.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-locally-set-reaches-peer.ci#L4). **negative:** `functional/verify` [`med-not-propagated-across-as.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-not-propagated-across-as.ci#L8). **positive:** `interop/nightly` [`checkMEDAcrossAS`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L194). **negative:** `interop/nightly` [`checkMEDAcrossAS`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L198) |
| `RFC4271-5.1.4-4` | A BGP speaker MUST implement a mechanism (based on local configuration) that allows the MULTI_EXIT_DISC attribute to be removed from a route (§5.1.4) | MUST | 5.1.4 | **positive:** `unit/verify` [`TestMEDRemovalMechanismIsConfigurable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L468). **positive:** `unit/verify` [`TestParseModifyDefsMEDRemove`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/modify_test.go#L642). **negative:** `unit/verify` [`TestMEDRemovalMechanismIsConfigurable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L475). **negative:** `unit/verify` [`TestMEDRemoveDirectiveIsValueless`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L568). **negative:** `unit/verify` [`TestParseModifyDefsMEDRemove`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/modify_test.go#L647). **positive:** `functional/verify` [`med-removal-configured.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-configured.ci#L4). **positive:** `interop/nightly` [`checkMEDRemovalConfiguration`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L316). **negative:** `interop/nightly` [`checkMEDRemovalConfiguration`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L320) |
| `RFC4271-5.1.4-2` | If a BGP speaker is configured to remove the MULTI_EXIT_DISC attribute from a route, then this removal MUST be done prior to determining the degree of preference of the route and prior to performing route selection (Decision Process phases 1 and 2). (§5.1.4) | MUST | 5.1.4 | **positive:** `unit/verify` [`TestHandleFilterUpdateMEDRemoveIsImportOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/modify_test.go#L715). **positive:** `unit/verify` [`TestMEDRemovalMechanismIsConfigurable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L480). **negative:** `unit/verify` [`TestHandleFilterUpdateMEDRemoveIsImportOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/modify_test.go#L722). **negative:** `unit/verify` [`TestMEDRemovalMechanismIsConfigurable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L488). **positive:** `functional/verify` [`med-removal-before-decision.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-before-decision.ci#L4). **positive:** `functional/verify` [`med-removal-configured.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-configured.ci#L8). **negative:** `functional/verify` [`med-removal-before-decision.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-before-decision.ci#L10). **negative:** `functional/verify` [`med-removal-export-refused.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-export-refused.ci#L4) |
| `RFC4271-5.1.5-1` | LOCAL_PREF is a well-known attribute that SHALL be included in all UPDATE messages that a given BGP speaker sends to other internal peers. (§5.1.5) | SHALL | 5.1.5 | **positive:** `unit/verify` [`TestRFC4271LocalPrefIncludedForInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L81). **negative:** `unit/verify` [`TestAnnounceStripsLocalPrefTowardExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_origin_test.go#L307). **negative:** `unit/verify` [`TestForwardLocalPrefStrippedToExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_local_pref_test.go#L45). **negative:** `unit/verify` [`TestRFC4271LocalPrefOmittedForExternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L107) |
| `RFC4271-5.1.5-2` | A BGP speaker MUST NOT include this attribute in UPDATE messages it sends to external peers, except in the case of BGP Confederations [RFC3065]. (§5.1.5) | MUST NOT | 5.1.5 | **positive:** `unit/verify` [`TestAnnounceStripsLocalPrefTowardExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_origin_test.go#L305). **positive:** `unit/verify` [`TestForwardLocalPrefStripBeatsAFilterSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_local_pref_test.go#L113). **positive:** `unit/verify` [`TestForwardLocalPrefStrippedToExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_local_pref_test.go#L41). **positive:** `unit/verify` [`TestRFC4271LocalPrefOmittedForExternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L104). **negative:** `unit/verify` [`TestLocalPrefAllowedToIsTheOnlyAnswer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_local_pref_test.go#L146). **negative:** `unit/verify` [`TestRFC4271LocalPrefIncludedForInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L83). **positive:** `functional/verify` [`local-pref-strip-ebgp.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/local-pref-strip-ebgp.ci#L14). **negative:** `functional/verify` [`local-pref-strip-ebgp.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/local-pref-strip-ebgp.ci#L19). **positive:** `interop/nightly` [`checkLocalPrefStrip`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L94) |
| `RFC4271-5.1.5-3` | If it is contained in an UPDATE message that is received from an external peer, then this attribute MUST be ignored by the receiving speaker, except in the case of BGP Confederations [RFC3065]. (§5.1.5) | MUST | 5.1.5 | **positive:** `unit/verify` [`TestRFC4271LocalPrefKeptOnInternalSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L566). **negative:** `unit/verify` [`TestRFC4271LocalPrefIgnoredOnExternalSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L589) |
| `RFC4271-5.1.5-4` | The higher degree of preference MUST be preferred. (§5.1.5) | MUST | 5.1.5 | **positive:** `unit/verify` [`TestBestPath_LocalPref`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L197). **negative:** `unit/verify` [`TestBestPath_LocalPref`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L199) |
| `RFC4271-5.1.6-1` | A BGP speaker that receives a route with the ATOMIC_AGGREGATE attribute MUST NOT make any NLRI of that route more specific (as defined in 9.1.4) when advertising this route to other BGP speakers. (§5.1.6) | MUST NOT | 5.1.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the obligation binds the RECEIVER/re-advertiser, and ze is one -- it stores a received ATOMIC_AGGREGATE and copies it through on readvertisement (internal/component/bgp/reactor/peer_rib_routes.go:141) -- but the prohibited act has no producer. `grep -rniE "more specific\|deaggregat\|de-aggregat\|disaggregat" --include=*.go internal/component/bgp/ \| grep -v _test` returns only substring hits inside `encodeAggregatorValue` and `attrCodeAggregator` (internal/component/bgp/reactor/filter_delta.go:294,396, internal/component/bgp/message/rfc7606.go:64,421); no code path splits a prefix. Both readvertisement encoders write the stored route's own prefix verbatim through nlri.WriteNLRI (internal/component/bgp/reactor/peer_rib_routes.go:103-104), so the advertised NLRI is byte-identical to what was received and can be neither more nor less specific. With no length-altering producer there is no behavior to exercise in either polarity |
| `RFC4271-6.1-1` | All errors detected while processing the Message Header MUST be indicated by sending the NOTIFICATION message with the Error Code Message Header Error. (§6.1) | MUST | 6.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** one class of header error is detected but never reported. A bad marker or a Length below 19 makes ParseHeader return a bare sentinel (internal/component/bgp/message/header.go:96-108), and the read loop turns that into an FSM event and a returned error with no NOTIFICATION sent (internal/component/bgp/reactor/session_read.go:98-102). The per-type and over-maximum length errors on the following lines do send Message Header Error (session_read.go:105-117) |
| `RFC4271-6.1-2` | If the Marker field of the message header is not as expected, then a synchronization error has occurred and the Error Subcode MUST be set to Connection Not Synchronized. (§6.1) | MUST | 6.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** NotifyHeaderConnectionNotSync is declared (internal/component/bgp/message/notification.go:52) but no producer ever sends it. ParseHeader returns ErrInvalidMarker, a plain sentinel carrying no NOTIFICATION (internal/component/bgp/message/header.go:96-99), and the read loop's marker-error branch sends nothing before returning (internal/component/bgp/reactor/session_read.go:98-102) |
| `RFC4271-6.1-3` | then the Error Subcode MUST be set to Bad Message Length. (§6.1) | MUST | 6.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** RFC 4271 §6.1 lists five length conditions and ze reports only four of them. The per-type minima and the 4096/65535 ceiling do produce a conformant Notification -- ValidateLength and ValidateLengthWithMax return a *Notification carrying NotifyHeaderBadLength and the two big-endian octets of the offending Length (internal/component/bgp/message/header.go:155-171 and :207-213), which the read loop sends before closing (internal/component/bgp/reactor/session_read.go:105-117). The first listed condition, "Length field of the message header is less than 19", does not: ParseHeader returns the bare sentinel ErrInvalidLength with no Notification and no Data (internal/component/bgp/message/header.go:106-108), and the read loop logs an FSM event and returns without writing anything (internal/component/bgp/reactor/session_read.go:98-102). The same code fact is recorded as the NOTIFICATION-absence gap on RFC4271-6.1-1. Disclosed in docs/features/rfc-status.md RFC 4271 row |
| `RFC4271-6.1-4` | If the Type field of the message header is not recognized, then the Error Subcode MUST be set to Bad Message Type. The Data field MUST contain the erroneous Type field. (§6.1) | MUST | 6.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** an unknown message type is reported with the wrong subcode and the wrong Data. handleUnknownType sends Message Header Error with subcode 0 and a human-readable text string rather than subcode 3 (Bad Message Type) with the erroneous Type octet (internal/component/bgp/reactor/session_handlers.go:20-36); NotifyHeaderBadType is declared at internal/component/bgp/message/notification.go:54 and has no producer |
| `RFC4271-6.2-3` | All errors detected while processing the OPEN message MUST be indicated by sending the NOTIFICATION message with the Error Code OPEN Message Error. (§6.2) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** one class of OPEN error is detected and never reported. UnpackOpen returns the bare sentinel ErrShortRead when the body is under 10 octets or when the Optional Parameters Length (standard or RFC 9072 extended) overruns the body (internal/component/bgp/message/open.go:167-168, :193-194, :199-200, :209-210), and handleOpen turns that into an FSM event and a returned error, writing no NOTIFICATION and not even closing the connection (internal/component/bgp/reactor/session_handlers.go:43-47); session_read.go:264 only propagates it. Every other OPEN error path does send Error Code 2 -- unsupported version (session_handlers.go:54-60), unacceptable Hold Time (:70-77) and a malformed capability (rejectOpenCapabilityError, :185-199) -- so the obligation holds everywhere except the decode failure. Disclosed in docs/features/rfc-status.md RFC 4271 row |
| `RFC4271-6.3-1` | All errors detected while processing the UPDATE message MUST be indicated by sending the NOTIFICATION message with the Error Code UPDATE Message Error. (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestRFC4271UpdateErrorReportedAsUpdateMessageError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L633). **negative:** `unit/verify` [`TestRFC4271ConformantUpdateSendsNoUpdateError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L696) |
| `RFC4271-6.7-1` | However, the Cease NOTIFICATION message MUST NOT be used when a fatal error indicated by this section does exist. (§6.7) | MUST NOT | 6.7 | **positive:** `unit/verify` [`TestPrefixExceedTeardown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_prefix_test.go#L110). **negative:** `unit/verify` [`TestRFC4271UpdateErrorReportedAsUpdateMessageError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L636) |
| `RFC4271-8.2.1-1` | BGP MUST maintain a separate FSM for each configured peer (§8.2.1) | MUST | 8.2.1 | **positive:** `unit/verify` [`TestRFC4271SeparateFSMPerPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L213). **negative:** `unit/verify` [`TestRFC4271PerPeerFSMDoesNotShareTimers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L237) |
| `RFC4271-8.2.1-2` | A BGP implementation MUST connect to and listen on TCP port 179 (§8.2.1) | MUST | 8.2.1 | **positive:** `unit/verify` [`TestRFC4271DefaultBGPPortIs179`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L306). **negative:** `unit/verify` [`TestRFC4271ExplicitPortOverridesDefault`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L322) |
| `RFC4271-8.2.1-3` | For each incoming connection, a state machine MUST be instantiated (§8.2.1) | MUST | 8.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** an incoming connection does not get its own state machine. acceptOrReject hands the accepted connection to the peer's existing session (internal/component/bgp/reactor/reactor_connection.go:117-163), and a connection queued for collision resolution is read raw by handlePendingCollision with no FSM behind it (internal/component/bgp/reactor/reactor_connection.go:196-249). An FSM is created per session, i.e. per connection attempt of a configured peer (internal/component/bgp/reactor/session.go:396), not per inbound connection |
| `RFC4271-8.2.2-1` | If the HoldTimer_Expires event occurs (Event 10), the local system: - sends a NOTIFICATION message with the Error Code Hold Timer Expired (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271HoldTimerExpirySendsNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L807). **negative:** `unit/verify` [`TestRFC4271HoldTimerNotYetExpiredSendsNoNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L836). **positive:** `functional/verify` [`deadpeer-holddown.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/deadpeer-holddown.ci#L3) |
| `RFC4271-8.2.2-2` | If the HoldTimer_Expires event occurs (Event 10), the local system: - sends a NOTIFICATION message with the Error Code Hold Timer Expired, - sets the ConnectRetryTimer to zero (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271HoldExpiryRunsTheEvent10ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L929). **negative:** `unit/verify` [`TestRFC4271NoHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1000) |
| `RFC4271-8.2.2-3` | If the HoldTimer_Expires event occurs (Event 10), the local system: - sends a NOTIFICATION message with the Error Code Hold Timer Expired, - sets the ConnectRetryTimer to zero, - releases all BGP resources (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271HoldExpiryRunsTheEvent10ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L932). **negative:** `unit/verify` [`TestRFC4271NoHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1002) |
| `RFC4271-8.2.2-4` | If the HoldTimer_Expires event occurs (Event 10), the local system: - sends a NOTIFICATION message with the Error Code Hold Timer Expired, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271HoldExpiryRunsTheEvent10ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L935). **negative:** `unit/verify` [`TestRFC4271NoHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1005) |
| `RFC4271-8.2.2-5` | If the HoldTimer_Expires event occurs (Event 10), the local system: - sends a NOTIFICATION message with the Error Code Hold Timer Expired, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1, - (optionally) performs peer oscillation damping if the DampPeerOscillations attribute is set to TRUE, and - changes its state to Idle. (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271HoldExpiryRunsTheEvent10ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L938). **negative:** `unit/verify` [`TestRFC4271NoHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1007) |
| `RFC4271-8.2.2-6` | If the HoldTimer_Expires event occurs (Event 10), the local system: - sends a NOTIFICATION message with the Error Code Hold Timer Expired, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1, - (optionally) performs peer oscillation damping if the DampPeerOscillations attribute is set to TRUE (§8.2.2) | MAY | 8.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-8.2.2-7` | In response to a ManualStart event (Event 1) or an AutomaticStart event (Event 3), the local system: - initializes all BGP resources for the peer connection, - sets ConnectRetryCounter to zero (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterZeroedOnManualStart`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L42). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterSurvivesDampedStart`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L75) |
| `RFC4271-8.2.2-8` | If a ManualStop event (Event 2) is issued in the OpenSent state, the local system: - sends the NOTIFICATION with a Cease, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - sets the ConnectRetryCounter to zero (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterZeroedOnManualStop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L108). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterNotZeroedByIdleManualStop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L130) |
| `RFC4271-8.2.2-9` | If the HoldTimer_Expires event occurs (Event 10), the local system: - sends a NOTIFICATION message with the Error Code Hold Timer Expired, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterIncrementsOnHoldTimerExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L151). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterQuietOnHealthyEstablishedTraffic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L174) |
| `RFC4271-8.2.2-10` | If the BGP message header checking (Event 21) or OPEN message checking detects an error (Event 22)(see Section 6.2), the local system: - sends a NOTIFICATION message with the appropriate error code, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterIncrementsOnHeaderAndOpenErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L210). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterNotIncrementedByIdleErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L236) |
| `RFC4271-8.2.2-11` | If the local system receives a TcpConnectionFails event (Event 18) from the underlying TCP or a NOTIFICATION message (Event 25), the local system: - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterIncrementsOnNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L258). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterStepsByExactlyOnePerNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L283) |
| `RFC4271-8.2.2-12` | If the local system receives a NOTIFICATION message (Event 24 or Event 25) or a TcpConnectionFails (Event 18) from the underlying TCP, the local system: - sets the ConnectRetryTimer to zero, - deletes all routes associated with this connection, - releases all the BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterOnVersionErrorPerState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L308). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterQuietOnVersionErrorInOpenStates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L332) |
| `RFC4271-8.2.2-13` | If the local system receives a TcpConnectionFails event (Event 18) from the underlying TCP or a NOTIFICATION message (Event 25), the local system: - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterOnTCPFailurePerState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L355). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterQuietOnTCPFailureInConnectAndOpenSent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L378) |
| `RFC4271-8.2.2-14` | If the local system receives an UPDATE message, and the UPDATE message error handling procedure (see Section 6.3) detects an error (Event 28), the local system: - sends a NOTIFICATION message with an Update error, - sets the ConnectRetryTimer to zero, - deletes all routes associated with this connection, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterIncrementsOnUpdateError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L401). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterQuietOnGoodUpdate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L421) |
| `RFC4271-8.2.2-15` | In response to any other events (Events 8, 10-11, 13, 19, 23, 25-28), the local system: - if the ConnectRetryTimer is running, stops and resets the ConnectRetryTimer (sets to zero), - if the DelayOpenTimer is running, stops and resets the DelayOpenTimer (sets to zero), - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterIncrementsOnAnyOtherEvent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L443). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterIdleDefaultArmCountsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L478) |
| `RFC4271-8.2.2-16` | If an AutomaticStop event (Event 8) is issued in the OpenSent state, the local system: - sends the NOTIFICATION with a Cease, - sets the ConnectRetryTimer to zero, - releases all the BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterIncrementsOnAutomaticStop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L549). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterAutomaticStopIsNotAManualStop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L575) |
| `RFC4271-8.2.2-17` | If a connection in the OpenSent state is determined to be the connection that must be closed, an OpenCollisionDump (Event 23) is signaled to the state machine. If such an event is received in the OpenSent state, the local system: - sends a NOTIFICATION with a Cease, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterIncrementsOnOpenCollisionDump`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L606). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterCollisionDumpIsQuietInIdle`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L630) |
| `RFC4271-8.2.2-18` | If a ManualStop event (Event 2) is issued in the OpenSent state, the local system: - sends the NOTIFICATION with a Cease, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestShutdownNotifySendsCeaseFromEveryConnectedState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/shutdown_notify_test.go#L174). **negative:** `unit/verify` [`TestRFC4271NoCeaseWithoutAManualStop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/shutdown_notify_test.go#L207). **positive:** `functional/verify` [`signal-stop-cease.ci`](https://github.com/ze-software/ze/blob/main/test/reload/signal-stop-cease.ci#L3) |
| `RFC4271-10-1` | An implementation of BGP MUST allow the HoldTimer to be configurable on a per-peer basis (§10) | MUST | 10 | **positive:** `unit/verify` [`TestRFC4271HoldTimeConfigurablePerPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L261). **negative:** `unit/verify` [`TestRFC4271PerPeerHoldTimeSurvivesNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L285) |
| `RFC4271-3-1` | To allow local policy changes to have the correct effect without resetting any BGP connections, a BGP speaker SHOULD either (a) retain the current version of the routes advertised to it by all of its peers for the duration of the connection, or (b) make use of the Route Refresh extension [RFC2918]. (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-5-7` | The sender of an UPDATE message SHOULD order path attributes within the UPDATE message in ascending order of attribute type. (§5) | SHOULD | 5 | **positive:** `unit/verify` [`TestAnnounceBatchRail_AS4PathOrderedAgainstLargeCommunity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_batch_attr_order_test.go#L328). **positive:** `unit/verify` [`TestAnnounceBatchRail_AscendingTypeCodeOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_batch_attr_order_test.go#L268). **positive:** `unit/verify` [`TestAnnounceQueuedRail_AscendingTypeCodeOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_batch_attr_order_test.go#L289). **positive:** `unit/verify` [`TestSplitMP_PreservesAscendingAttributeOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_split_attr_order_test.go#L72). **negative:** no negative test |
| `RFC4271-5.1.6-2` | If an aggregate excludes at least some of the AS numbers present in the AS_PATH of the routes that are aggregated as a result of dropping the AS_SET, the aggregated route, when advertised to the peer, SHOULD include the ATOMIC_AGGREGATE attribute. (§5.1.6) | SHOULD | 5.1.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-5.1.6-3` | A BGP speaker that receives a route with the ATOMIC_AGGREGATE attribute SHOULD NOT remove the attribute when propagating the route to other speakers. (§5.1.6) | SHOULD NOT | 5.1.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-6.3-2` | If the NEXT_HOP attribute is semantically incorrect, the error SHOULD be logged, and the route SHOULD be ignored. (§6.3) | SHOULD | 6.3 | **positive:** `unit/verify` [`TestRFC4271SelfNextHopRouteIsNotInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L52). **negative:** no negative test |
| `RFC4271-6.3-3` | In this case, a NOTIFICATION message SHOULD NOT be sent (§6.3) | SHOULD NOT | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.2-1` | A BGP speaker SHOULD NOT advertise a given feasible BGP route from its Adj-RIB-Out if it would produce an UPDATE message containing the same BGP route as was previously advertised. (§9.2) | SHOULD NOT | 9.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.2.1.1-1` | Since fast convergence is needed within an autonomous system, either (a) the MinRouteAdvertisementIntervalTimer used for internal peers SHOULD be shorter than the MinRouteAdvertisementIntervalTimer used for external peers, or (b) the procedure describe in this section SHOULD NOT apply to routes sent to internal peers. (§9.2.1.1) | SHOULD NOT | 9.2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-10-2` | To minimize the likelihood that the distribution of BGP messages by a given BGP speaker will contain peaks, jitter SHOULD be applied to the timers associated with MinASOriginationIntervalTimer, KeepaliveTimer, MinRouteAdvertisementIntervalTimer, and ConnectRetryTimer. (§10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-5.1.1-1` | The ORIGIN attribute is generated by the speaker that originates the associated routing information. Its value SHOULD NOT be changed by any other speaker. (§5.1.1) | SHOULD | 5.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-4.2-3` | An implementation MAY reject connections on the basis of the Hold Time. (§4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-3.1-1` | If a BGP speaker chooses to advertise a previously received route, it MAY add to, or modify, the path attributes of the route before advertising it to a peer. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-5.1.2-1` | Whenever the modification of the AS_PATH attribute calls for including or prepending the AS number of the local system, the local system MAY include/prepend more than one instance of its own AS number in the AS_PATH attribute. (§5.1.2) | MAY | 5.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-6.7-2` | A BGP speaker MAY support the ability to impose a locally-configured, upper bound on the number of address prefixes the speaker is willing to accept from a neighbor. (§6.7) | MAY | 6.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-10-3` | MAY allow the other timers to be configurable. (§10) | MAY | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-6.2-4` | An implementation MAY reject any proposed Hold Time (§6.2) | MAY | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-6.7-3` | The speaker MAY also log this locally. (§6.7) | MAY | 6.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-3.1-2` | The next hop for each of these routes MUST be resolvable via the local BGP speaker's Routing Table. (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the BGP Loc-RIB install performs no reachability check on the route's next hop. mirrorToLocRIB inserts the winning path with whatever next-hop address the attribute carried (internal/component/bgp/plugins/rib/rib_bestchange.go:797-830), and the candidate gather step filters only on SRv6 ineligibility (internal/component/bgp/plugins/rib/rib_commands.go:1039-1057). Resolvability is enforced downstream at FIB-install time, which removes the route from the routing table but leaves it in the Loc-RIB |
| `RFC4271-5.1.2-2` | a) When a given BGP speaker advertises the route to an internal peer, the advertising speaker SHALL NOT modify the AS_PATH attribute associated with the route. (§5.1.2) | SHALL NOT | 5.1.2 | **positive:** `unit/verify` [`TestEstablishedAnnounce_ExplicitASPath_IBGPVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_batch_test.go#L569). **positive:** `unit/verify` [`TestRFC4271ASPathUnmodifiedTowardInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L131). **negative:** `unit/verify` [`TestRFC4271ASPathPrependedTowardExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L148) |
| `RFC4271-5.1.2-3` | When a given BGP speaker advertises the route to an external peer, the advertising speaker updates the AS_PATH attribute as follows: 1) if the first path segment of the AS_PATH is of type AS_SEQUENCE, the local system prepends its own AS number as the last element of the sequence (put it in the leftmost position with respect to the position of octets in the protocol message). If the act of prepending will cause an overflow in the AS_PATH segment (i.e., more than 255 ASes), it SHOULD prepend a new segment of type AS_SEQUENCE and prepend its own AS number to this new segment. 2) if the first path segment of the AS_PATH is of type AS_SET, the local system prepends a new path segment of type AS_SEQUENCE to the AS_PATH, including its own AS number in that segment. 3) if the AS_PATH is empty, the local system creates a path segment of type AS_SEQUENCE, places its own AS into that segment, and places that segment into the AS_PATH. (§5.1.2) | SHALL | 5.1.2 | **positive:** `unit/verify` [`TestASPathSlotPrependOnlyWhenAdvertising`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/advertise_test.go#L97). **positive:** `unit/verify` [`TestEstablishedAnnounce_ExplicitASPath_PrependsLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_batch_test.go#L543). **negative:** `unit/verify` [`TestASPathSlotPrependOnlyWhenAdvertising`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/advertise_test.go#L99). **negative:** `unit/verify` [`TestEstablishedAnnounce_ExplicitASPath_IBGPVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_batch_test.go#L567). **positive:** `interop/nightly` [`checkRelayWithdrawalShape`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L429). **negative:** `interop/nightly` [`checkRelayWithdrawalShape`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L430) |
| `RFC4271-5.1.4-3` | If a BGP speaker is configured to alter the value of the MULTI_EXIT_DISC attribute received over EBGP, then altering the value MUST be done prior to determining the degree of preference of the route and prior to performing route selection (Decision Process phases 1 and 2). (§5.1.4) | MUST | 5.1.4 | **positive:** `unit/verify` [`TestRFC4271MEDAlterationHappensAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L452). **negative:** no negative test. **{single-polarity}:** the requirement constrains only the ORDER of an alteration that a speaker chooses to make, so there is no non-conformant input a receiver could reject. ze's only place to alter a received MULTI_EXIT_DISC is the ingress filter chain, whose rewritten payload replaces the WireUpdate before the UPDATE is dispatched to the RIB plugin that runs phases 1 and 2 (internal/component/bgp/reactor/reactor_notify.go:427-466) |
| `RFC4271-5.1.5-5` | A BGP speaker SHALL calculate the degree of preference for each external route based on the locally-configured policy, and include the degree of preference when advertising a route to its internal peers. (§5.1.5) | SHALL | 5.1.5 | **positive:** `unit/verify` [`TestRFC4271ExternalRouteDegreeOfPreferenceFromLocalPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L141). **negative:** `unit/verify` [`TestRFC4271DegreeOfPreferenceNotAHardcodedConstant`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L168) |
| `RFC4271-5.1.7-1` | A BGP speaker that performs route aggregation MAY add the AGGREGATOR attribute, which SHALL contain its own AS number and IP address. (§5.1.7) | SHALL | 5.1.7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never performs aggregation, so it never adds an AGGREGATOR of its own. The same grep as RFC4271-5.1.6-1 finds no aggregation producer; AGGREGATOR is only interned from the wire (internal/component/bgp/plugins/rib/storage/attrparse.go:96-102), replayed on readvertise (internal/component/bgp/plugins/rib/storage/familyrib.go:817-819) or emitted from operator configuration (internal/component/bgp/message/update_build_grouped.go:141-148) |
| `RFC4271-6.7-4` | If the BGP speaker decides to terminate its BGP connection with a neighbor because the number of address prefixes received from the neighbor exceeds the locally-configured, upper bound, then the speaker MUST send the neighbor a NOTIFICATION message with the Error Code Cease. (§6.7) | MUST | 6.7 | **positive:** `unit/verify` [`TestPrefixExceedTeardown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_prefix_test.go#L107). **negative:** `unit/verify` [`TestPrefixExceedDrop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_prefix_test.go#L144) |
| `RFC4271-6.8-1` | In the event of connection collision, one of the connections MUST be closed (§6.8) | MUST | 6.8 | **positive:** `unit/verify` [`TestRFC4271CollisionClosesExactlyOneConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_rfc7611_ingress_test.go#L44). **negative:** `unit/verify` [`TestCollisionOpenSentNoCollision`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L176) |
| `RFC4271-6.8-2` | Upon receipt of an OPEN message, the local system MUST examine all of its connections that are in the OpenConfirm state. (§6.8) | MUST | 6.8 | **positive:** `unit/verify` [`TestCollisionOpenConfirmLocalWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L129). **negative:** `unit/verify` [`TestCollisionNonCollisionStates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L535) |
| `RFC4271-9-1` | If the UPDATE message contains a non-empty WITHDRAWN ROUTES field, the previously advertised routes, whose destinations (expressed as IP prefixes) are contained in this field, SHALL be removed from the Adj-RIB-In. (§9) | SHALL | 9 | **positive:** `unit/verify` [`TestRFC4271WithdrawRemovesFromAdjRIBIn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L215). **negative:** `unit/verify` [`TestRFC4271WithdrawRemovesFromAdjRIBIn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L220) |
| `RFC4271-9-2` | If the UPDATE message contains a feasible route, the Adj-RIB-In will be updated with this route as follows: if the NLRI of the new route is identical to the one the route currently has stored in the Adj- RIB-In, then the new route SHALL replace the older route in the Adj- RIB-In, thus implicitly withdrawing the older route from service. (§9) | SHALL | 9 | **positive:** `unit/verify` [`TestRFC4271SamePrefixReplacesRatherThanAccumulates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L189). **negative:** `unit/verify` [`TestRFC4271WithdrawRemovesFromAdjRIBIn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L217) |
| `RFC4271-9-3` | Once the BGP speaker updates the Adj-RIB-In, the speaker SHALL run its Decision Process. (§9) | SHALL | 9 | **positive:** `unit/verify` [`TestRIBBestChangeWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L686). **negative:** `unit/verify` [`TestRIBBestChangeNoPublishSameBest`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L650) |
| `RFC4271-9.1.1-1` | The function that calculates the degree of preference for a given route SHALL NOT use any of the following as its inputs: the existence of other routes, the non-existence of other routes, or the path attributes of other routes. (§9.1) | SHALL NOT | 9.1 | **positive:** `unit/verify` [`TestRFC4271DegreeOfPreferenceIgnoresOtherRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L33). **negative:** `unit/verify` [`TestRFC4271DegreeOfPreferenceFollowsOwnAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L68) |
| `RFC4271-9.1.1-2` | the return value MUST be used as the LOCAL_PREF value in any IBGP readvertisement. (§9.1.1) | MUST | 9.1.1 | **positive:** `unit/verify` [`TestRFC4271LocalPrefIncludedForInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L86). **negative:** `unit/verify` [`TestRFC4271LocalPrefOmittedForExternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L110) |
| `RFC4271-9.1.2-1` | If the NEXT_HOP attribute of a BGP route depicts an address that is not resolvable, or if it would become unresolvable if the route was installed in the routing table, the BGP route MUST be excluded from the Phase 2 decision function. (§9.1.2) | MUST | 9.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** an unresolvable NEXT_HOP does not exclude the route from Phase 2. gatherCandidatesLocked skips only SRv6-ineligible entries (internal/component/bgp/plugins/rib/rib_commands.go:1039-1057), and extractCandidate uses the next hop solely to look up an IGP cost (internal/component/bgp/plugins/rib/rib_commands.go:1123-1131), so an unreachable next hop yields a cost of zero and the route competes normally |
| `RFC4271-9.1.2-2` | The local speaker SHALL then install that route in the Loc-RIB, replacing any route to the same destination that is currently being held in the Loc-RIB. (§9.1.2) | SHALL | 9.1.2 | **positive:** `unit/verify` [`TestLocRIBMirror`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L1513). **negative:** `unit/verify` [`TestRIBBestChangeWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L691) |
| `RFC4271-9.1.2-3` | The local speaker MUST determine the immediate next-hop address from the NEXT_HOP attribute (§9.1.2) | MUST | 9.1.2 | **positive:** `unit/verify` [`TestLocRIBMirror`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L1515). **negative:** `unit/verify` [`TestRFC4271LocRIBNextHopComesFromNextHopAttribute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L94) |
| `RFC4271-9.1.2-4` | If either the immediate next-hop or the IGP cost to the NEXT_HOP (where the NEXT_HOP is resolved through an IGP route) changes, Phase 2 Route Selection MUST be performed again. (§9.1.2) | MUST | 9.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nothing re-runs Phase 2 when the immediate next-hop or the IGP cost to the NEXT_HOP changes. The only entry points to checkBestPathChange are the UPDATE ingest path and the peer-state paths (internal/component/bgp/plugins/rib/rib_structured.go:271-286), and the IGP cost function is a passive lookup registered once with no invalidation callback (internal/component/bgp/plugins/rib/bestpath.go:30-43) |
| `RFC4271-9.1.2.1-1` | Notice that even though BGP routes do not have to be installed in the Routing Table with the immediate next-hop(s), implementations MUST take care that, before any packets are forwarded along a BGP route, its associated NEXT_HOP address is resolved to the immediate (directly connected) next-hop address, and that this address (or multiple addresses) is finally used for actual packet forwarding. (§9.1.2) | MUST | 9.1.2 | **positive:** `unit/verify` [`TestLocRIBMirror`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L1518). **negative:** `unit/verify` [`TestRFC4271LocRIBNextHopComesFromNextHopAttribute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L98) |
| `RFC4271-9.1.2.1-2` | Unresolvable routes SHALL be removed from the Loc-RIB and the routing table. (§9.1.2) | SHALL | 9.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** an unresolvable route is not removed from the Loc-RIB. The only Loc-RIB removal in the BGP plugin is the no-candidate-remains branch of checkBestPathChange (internal/component/bgp/plugins/rib/rib_bestchange.go:766-782), which is driven by the Adj-RIB-In losing its last path and never by next-hop resolvability; nothing in the plugin consults a resolver |
| `RFC4271-9.1.2.2-1` | The criteria MUST be applied in the order specified. (§9.1.2.2) | MUST | 9.1.2.2 | **positive:** `unit/verify` [`TestBestPathStepFComparesThePeerBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_bgp_identifier_test.go#L88). **positive:** `unit/verify` [`TestBestPathStepFComparesThePeerBGPIdentifierOnTheJSONRail`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_bgp_identifier_json_test.go#L78). **positive:** `unit/verify` [`TestBestPath_FullTiebreak`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L530). **negative:** `unit/verify` [`TestBestPathEqualBGPIdentifiersFallThroughToPeerAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_bgp_identifier_test.go#L116). **negative:** `unit/verify` [`TestBestPathEqualBGPIdentifiersOnTheJSONRailFallThroughToPeerAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_bgp_identifier_json_test.go#L106). **negative:** `unit/verify` [`TestBestPath_MED_SameNeighborAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L315) |
| `RFC4271-9.1.2.2-2` | If an implementation chooses to remove MULTI_EXIT_DISC, then the optional comparison on MULTI_EXIT_DISC, if performed, MUST be performed only among EBGP-learned routes. (§9.1.2.2) | MUST | 9.1.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the common egress guard is implemented, but normal BGP selected-route readvertisement has no runnable producer and no discriminating proof. `bgp-rib` records selected Loc-RIB state (internal/component/bgp/plugins/rib/rib_bestchange.go:738-790), route-server and route-reflector plugins forward cached UPDATEs instead (internal/component/bgp/plugins/rs/server.go:433-434 and internal/component/bgp/plugins/rr/rr.go:188-195), and BGP-to-BGP redistribution is rejected as same-protocol redistribution (internal/core/redistevents/registry.go:144-146) |
| `RFC4271-9.1.2.2-3` | For IBGP- learned routes, the MULTI_EXIT_DISC MUST be used in route comparisons that reach this step in the Decision Process. (§9.1.2.2) | MUST | 9.1.2.2 | **positive:** `unit/verify` [`TestBestPath_MED_SameNeighborAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L319). **negative:** `unit/verify` [`TestBestPath_MED_SameNeighborAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L322) |
| `RFC4271-9.1.2.2-4` | Routes that do not have the MULTI_EXIT_DISC attribute are considered to have the lowest possible MULTI_EXIT_DISC value (§9.1.2.2) | MUST | 9.1.2.2 | **positive:** `unit/verify` [`TestAbsentMedStillComparesAsZeroInPhaseTwo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L225). **negative:** `unit/verify` [`TestAbsentMedTiesAnExplicitMedOfZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L257) |
| `RFC4271-9.2-2` | A route SHALL NOT be installed in the Adj-Rib-Out unless the destination, and NEXT_HOP described by this route, may be forwarded appropriately by the Routing Table. (§9.1.3) | SHALL NOT | 9.1.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nothing gates Adj-RIB-Out installation on the destination and NEXT_HOP being forwardable. QueueAnnounce records the route unconditionally (internal/component/bgp/rib/outgoing.go:65-101), and the forwarding rails decide only on filters, family negotiation and the route-reflection rules (internal/component/bgp/reactor/forward_rs.go:295-333) |
| `RFC4271-9.2-3` | If a route in Loc-RIB is excluded from a particular Adj-RIB-Out, the previously advertised route in that Adj-RIB-Out MUST be withdrawn from service by means of an UPDATE message (§9.1.3) | MUST | 9.1.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a route excluded from a peer's Adj-RIB-Out by an egress filter is skipped silently, leaving the peer's previous advertisement in place instead of withdrawing it. Both forwarding rails `continue` on suppression with no withdrawal built (internal/component/bgp/reactor/forward_rs.go:320-333 and internal/component/bgp/reactor/reactor_api_forward.go:496-506); the one announce-to-withdraw conversion is LLGR-specific and filter-requested, not exclusion-driven (internal/component/bgp/reactor/reactor_api_forward.go:588-601) |
| `RFC4271-9.2-4` | If a BGP speaker receives overlapping routes, the Decision Process MUST consider both routes based on the configured acceptance policy. (§9.1.4) | MUST | 9.1.4 | **positive:** `unit/verify` [`TestRFC4271OverlappingRoutesBothInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L148). **negative:** `unit/verify` [`TestRFC4271SamePrefixReplacesRatherThanAccumulates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L182) |
| `RFC4271-9.2-5` | If both a less and a more specific route are accepted, then the Decision Process MUST install, in Loc-RIB, either both the less and the more specific routes or aggregate the two routes and install, in Loc-RIB, the aggregated route, provided that both routes have the same value of the NEXT_HOP attribute. (§9.1.4) | MUST | 9.1.4 | **positive:** `unit/verify` [`TestRFC4271OverlappingRoutesBothInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L151). **negative:** `unit/verify` [`TestRFC4271SamePrefixReplacesRatherThanAccumulates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L186) |
| `RFC4271-9.2.1.1-2` | Two UPDATE messages sent by a BGP speaker to a peer that advertise feasible routes and/or withdrawal of unfeasible routes to some common set of destinations MUST be separated by at least MinRouteAdvertisementIntervalTimer. (§9.2.1.1) | MUST | 9.2.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze has no MinRouteAdvertisementIntervalTimer, so successive UPDATEs to a common set of destinations are not spaced. The timer set implements only ConnectRetry, Hold and Keepalive and records the omission in its own doc comment (internal/component/bgp/fsm/timer.go:34-42, "MinRouteAdvertisementIntervalTimer (Section 9.2.1.1) - not implemented here"); `grep -rniE 'minroute\|mrai' --include=*.go internal/` finds no producer |
| `RFC4271-9.2.2.2-1` | Routes that have different MULTI_EXIT_DISC attributes SHALL NOT be aggregated. (§9.2.2.2) | SHALL NOT | 9.2.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never aggregates routes, so no producer can aggregate two routes with different MULTI_EXIT_DISC values. `grep -rniE 'aggregate-address\|AggregateRoute\|route aggregation' --include=*.go .` returns no hit outside rfc/ and plan/, and no code path synthesizes an aggregate route from more-specifics |
| `RFC4271-9.2.2.2-2` | ORIGIN attribute: If at least one route among routes that are aggregated has ORIGIN with the value INCOMPLETE, then the aggregated route MUST have the ORIGIN attribute with the value INCOMPLETE. (§9.2.2.2) | MUST | 9.2.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never aggregates routes, so no producer computes an aggregate ORIGIN. ORIGIN is only parsed (internal/core/bgp/attribute/origin.go:146-160), interned (internal/component/bgp/plugins/rib/storage/attrparse.go) and re-emitted verbatim (internal/component/bgp/plugins/rib/storage/familyrib.go:799-801); the same aggregation grep returns nothing |
| `RFC4271-9.2.2.2-3` | NEXT_HOP: When aggregating routes that have different NEXT_HOP attributes, the NEXT_HOP attribute of the aggregated route SHALL identify an interface on the BGP speaker that performs the aggregation. (§9.2.2.2) | SHALL | 9.2.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never aggregates routes, so no producer chooses an aggregated NEXT_HOP. The only next-hop selection is per-route egress policy (internal/component/bgp/reactor/peer_forward_facts.go:153-193); the same aggregation grep returns nothing |
| `RFC4271-9.2.2.2-4` | ATOMIC_AGGREGATE: If at least one of the routes to be aggregated has ATOMIC_AGGREGATE path attribute, then the aggregated route SHALL have this attribute as well. (§9.2.2.2) | SHALL | 9.2.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never aggregates routes, so no producer decides whether an aggregate carries ATOMIC_AGGREGATE. The attribute is only decoded, stored and replayed (internal/core/bgp/attribute/simple.go:175-195, internal/component/bgp/plugins/rib/storage/familyrib.go:815-817); the same aggregation grep returns nothing |
| `RFC4271-9.2.2.2-5` | AGGREGATOR: Any AGGREGATOR attributes from the routes to be aggregated MUST NOT be included in the aggregated route. (§9.2.2.2) | MUST NOT | 9.2.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never aggregates routes, so no producer builds an aggregated route from which a contributing AGGREGATOR would have to be excluded. AGGREGATOR is only interned from the wire or emitted from operator configuration (internal/component/bgp/plugins/rib/storage/attrparse.go:96-102, internal/component/bgp/message/update_build_grouped.go:141-148); the same aggregation grep returns nothing |
| `RFC4271-Security-1` | An implementation MUST support the TCP MD5 option [RFC2385]. (§E) | MUST | E | **positive:** `unit/verify` [`TestMD5PeersForListener`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_test.go#L2357). **negative:** `unit/verify` [`TestMD5PeersForListener`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_test.go#L2360) |
| `RFC4271-9.2.1.1-3` | If new routes are selected multiple times while awaiting the expiration of MinRouteAdvertisementIntervalTimer, the last route selected SHALL be advertised at the end of MinRouteAdvertisementIntervalTimer. (§9.2.1.1) | SHALL | 9.2.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** with no MinRouteAdvertisementIntervalTimer there is no expiry at which a last-selected route could be advertised. The timer is absent by design note (internal/component/bgp/fsm/timer.go:39) and no producer buffers a pending best-route advertisement against such a timer; best-path changes are published as they are computed (internal/component/bgp/plugins/rib/rib_bestchange.go:832-880) |
| `RFC4271-9.2-6` | When a BGP speaker receives an UPDATE message from an internal peer, the receiving BGP speaker SHALL NOT re-distribute the routing information contained in that UPDATE message to other internal peers (unless the speaker acts as a BGP Route Reflector [RFC2796]). (§9.2) | SHALL NOT | 9.2 | **positive:** `unit/verify` [`TestRFC4271NoIBGPToIBGPRedistribution`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L508). **negative:** `unit/verify` [`TestRFC4271IBGPRedistributionAllowedForReflectorClient`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L567) |
| `RFC4271-9.2-7` | All newly installed routes and all newly unfeasible routes for which there is no replacement route SHALL be advertised to its peers by means of an UPDATE message. (§9.2) | SHALL | 9.2 | **positive:** `unit/verify` [`TestRIBBestChangeWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L689). **negative:** `unit/verify` [`TestRIBBestChangeNoPublishSameBest`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L653) |
| `RFC4271-9.2-8` | Any routes in the Loc-RIB marked as unfeasible SHALL be removed (§9.2) | SHALL | 9.2 | **positive:** `unit/verify` [`TestLocRIBMirror`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L1521). **negative:** `unit/verify` [`TestRIBBestChangeNoPublishSameBest`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L655) |
| `RFC4271-9.2-9` | Changes to the reachable destinations within its own autonomous system SHALL also be advertised in an UPDATE message. (§9.2) | SHALL | 9.2 | **positive:** `unit/verify` [`TestRFC4271OwnASReachabilityChangeAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L398). **negative:** `unit/verify` [`TestRFC4271OwnASUnreachabilityChangeAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L426) |
| `RFC4271-9.2-10` | If, due to the limits on the maximum size of an UPDATE message (see Section 4), a single route doesn't fit into the message, the BGP speaker MUST not advertise the route to its peers and MAY choose to log an error locally. (§9.2) | MUST | 9.2 | **positive:** `unit/verify` [`TestRFC4271OversizeSingleRouteNotAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L402). **negative:** `unit/verify` [`TestRFC4271FittingRouteIsAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L431) |
| `RFC4271-Appendix-1` | If a local system TCP user interface supports the TCP PUSH function, then each BGP message SHOULD be transmitted with PUSH flag set. (§E) | SHOULD | E | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-Appendix-2` | If a local system TCP user interface supports setting the DSCP field [RFC2474] for TCP connections, then the TCP connection used by BGP SHOULD be opened with bits 0-2 of the DSCP field set to 110 (binary). (§E) | SHOULD | E | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.3-1` | an AS SHOULD avoid using unstable routes (§9.3) | SHOULD | 9.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.3-2` | Thus, an AS SHOULD avoid using unstable routes, and it SHOULD NOT make rapid, spontaneous changes to its choice of route. (§9.3) | SHOULD NOT | 9.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.4-1` | The decision of whether to distribute non-BGP acquired routes within an AS via BGP depends on the environment within the AS (e.g., type of IGP) and SHOULD be controlled via configuration. (§9.4) | SHOULD | 9.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.1.2-5` | If the AS_PATH attribute of a BGP route contains an AS loop, the BGP route should be excluded from the Phase 2 decision function (§9.1.2). Detection scans the full AS path and checks that the local autonomous system number does not appear in it. RFC 4271 writes this keyword in lower case, so the level is a recommendation and not a capitalized RFC 2119 SHOULD. The same paragraph places a speaker configured to accept routes with its own autonomous system number in the AS path outside the scope of the document. That out-of-scope case is what the allow-own-as setting selects, so a non-zero allow-own-as is not a deviation from this line. | SHOULD | 9.1.2 | **positive:** `unit/verify` [`TestDetectASLoop_NotPresent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/filter/loop_test.go#L136). **negative:** `unit/verify` [`TestDetectASLoop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/filter/loop_test.go#L114). **negative:** `unit/verify` [`TestDetectASLoop_ASSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/filter/loop_test.go#L125). **negative:** `functional/verify` [`loop-as.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/loop-as.ci#L7) |
| `RFC4271-9.1.2.1-3` | However, corresponding unresolvable routes SHOULD be kept in the Adj-RIBs-In (in case they become resolvable). (§9.1.2) | SHOULD | 9.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.1.2.1-4` | If multiple matching routes are available, only the longest matching route SHOULD be considered. (§9.1.2.1) | SHOULD | 9.1.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.1.2.1-5` | Whenever a BGP speaker identifies a route that fails the resolvability check because of mutual recursion, an error message SHOULD be logged. (§9.1.2.1) | SHOULD | 9.1.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-8.2.1.4-1` | After the connection collision is resolved (see Section 6.8), the FSM for the connection that is closed SHOULD be disposed. (§8.2.1.2) | SHOULD | 8.2.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-4.3-6` | An UPDATE message SHOULD NOT include the same address prefix in the WITHDRAWN ROUTES and Network Layer Reachability Information fields. (§4.3) | SHOULD NOT | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-4.3-7` | A BGP speaker SHOULD treat an UPDATE message of this form as though the WITHDRAWN ROUTES do not contain the address prefix. (§4.3) | SHOULD | 4.3 | **positive:** `unit/verify` [`TestRIBInjectSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L121). **positive:** `unit/verify` [`TestRIBPoolPathSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L87). **positive:** `unit/verify` [`TestRIBSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L42). **negative:** no negative test |
| `RFC4271-5.1.7-2` | The IP address SHOULD be the same as the BGP Identifier of the speaker. (§5.1.7) | SHOULD | 5.1.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.2-11` | If a BGP speaker chooses to aggregate, then it SHOULD either include all ASes used to form the aggregate in an AS_SET, or add the ATOMIC_AGGREGATE attribute to the route. (§9.1.4) | SHOULD | 9.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.2-12` | Routes SHOULD NOT be de-aggregated. (§9.1.4) | SHOULD NOT | 9.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.2.2.2-6` | If the aggregated route has an AS_SET as the first element in its AS_PATH attribute, then the router that originates the route SHOULD NOT advertise the MULTI_EXIT_DISC attribute with this route. (§9.2.2.2) | SHOULD NOT | 9.2.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-5-8` | Once a BGP peer has updated any well-known attributes, it MUST pass these attributes to its peers in any updates it transmits (§5) | MUST | 5 | **positive:** `unit/verify` [`TestForwardTransmitsUpdatedWellKnownAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rfc4271_section5_test.go#L145). **negative:** `unit/verify` [`TestForwardNeverTransmitsTheSupersededWellKnownAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rfc4271_section5_test.go#L166) |
| `RFC4271-6.2-5` | If the version number in the Version field of the received OPEN message is not supported, then the Error Subcode MUST be set to Unsupported Version Number (§6.2) | MUST | 6.2 | **positive:** `unit/verify` [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_error_rfc4271_test.go#L64). **negative:** `unit/verify` [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_error_rfc4271_test.go#L66) |
| `RFC4271-6.2-6` | If the Autonomous System field of the OPEN message is unacceptable, then the Error Subcode MUST be set to Bad Peer AS (§6.2) | MUST | 6.2 | **positive:** `unit/verify` [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_error_rfc4271_test.go#L67). **negative:** `unit/verify` [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_error_rfc4271_test.go#L69) |
| `RFC4271-6.2-7` | If the Hold Time field of the OPEN message is unacceptable, then the Error Subcode MUST be set to Unacceptable Hold Time (§6.2) | MUST | 6.2 | **positive:** `unit/verify` [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_error_rfc4271_test.go#L71). **negative:** `unit/verify` [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_error_rfc4271_test.go#L73) |
| `RFC4271-6.2-8` | If the BGP Identifier field of the OPEN message is syntactically incorrect, then the Error Subcode MUST be set to Bad BGP Identifier (§6.2) | MUST | 6.2 | **positive:** `unit/verify` [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_error_rfc4271_test.go#L74). **negative:** `unit/verify` [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_error_rfc4271_test.go#L76) |
| `RFC4271-6.2-9` | If one of the Optional Parameters in the OPEN message is not recognized, then the Error Subcode MUST be set to Unsupported Optional Parameters (§6.2) | MUST | 6.2 | **positive:** `unit/verify` [`TestSessionRFC4271OptionalParameterErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L36). **negative:** `unit/verify` [`TestSessionRFC4271OptionalParameterErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L37) |
| `RFC4271-6.2-10` | If one of the Optional Parameters in the OPEN message is recognized, but is malformed, then the Error Subcode MUST be set to 0 (Unspecific) (§6.2) | MUST | 6.2 | **positive:** `unit/verify` [`TestSessionRFC4271OptionalParameterErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L38). **negative:** `unit/verify` [`TestSessionRFC4271OptionalParameterErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L39) |
| `RFC4271-6.3-4` | If the Withdrawn Routes Length or Total Attribute Length is too large (i.e., if Withdrawn Routes Length + Total Attribute Length + 23 exceeds the message Length), then the Error Subcode MUST be set to Malformed Attribute List. (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestRFC4271UpdateMalformedAttributeList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_update_error_rfc4271_test.go#L54). **negative:** `unit/verify` [`TestRFC4271UpdateMalformedAttributeList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_update_error_rfc4271_test.go#L57) |
| `RFC4271-6.3-5` | If any recognized attribute has Attribute Flags that conflict with the Attribute Type Code, then the Error Subcode MUST be set to Attribute Flags Error. The Data field MUST contain the erroneous attribute (type, length, and value). (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L102). **negative:** `unit/verify` [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L103) |
| `RFC4271-6.3-6` | If any recognized attribute has an Attribute Length that conflicts with the expected length (based on the attribute type code), then the Error Subcode MUST be set to Attribute Length Error. The Data field MUST contain the erroneous attribute (type, length, and value). (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L104). **negative:** `unit/verify` [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L105) |
| `RFC4271-6.3-7` | If any of the well-known mandatory attributes are not present, then the Error Subcode MUST be set to Missing Well-known Attribute. The Data field MUST contain the Attribute Type Code of the missing, well-known attribute. (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestRFC4271MandatoryAttributesAcrossUpdateForms`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_mandatory_test.go#L10). **positive:** `unit/verify` [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L106). **negative:** `unit/verify` [`TestRFC4271MandatoryAttributesAcrossUpdateForms`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_mandatory_test.go#L11). **negative:** `unit/verify` [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L107) |
| `RFC4271-6.3-8` | If any of the well-known mandatory attributes are not recognized, then the Error Subcode MUST be set to Unrecognized Well-known Attribute. The Data field MUST contain the unrecognized attribute (type, length, and value). (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestSessionRFC4271RetainedUpdateNotifications`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L156). **negative:** `unit/verify` [`TestSessionRFC4271RetainedUpdateNotifications`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L157) |
| `RFC4271-6.3-9` | If the ORIGIN attribute has an undefined value, then the Error Sub- code MUST be set to Invalid Origin Attribute. The Data field MUST contain the unrecognized attribute (type, length, and value). (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L108). **negative:** `unit/verify` [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L109) |
| `RFC4271-6.3-10` | If the NEXT_HOP attribute field is syntactically incorrect, then the Error Subcode MUST be set to Invalid NEXT_HOP Attribute. The Data field MUST contain the incorrect attribute (type, length, and value). (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L110). **negative:** `unit/verify` [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L111) |
| `RFC4271-6.3-11` | The IP address in the NEXT_HOP MUST meet the following criteria to be considered semantically correct: a) It MUST NOT be the IP address of the receiving speaker. b) In the case of an EBGP, where the sender and receiver are one IP hop away from each other, either the IP address in the NEXT_HOP MUST be the sender's IP address that is used to establish the BGP connection, or the interface associated with the NEXT_HOP IP address MUST share a common subnet with the receiving BGP speaker. (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestSessionRFC4271IBGPNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_next_hop_test.go#L62). **positive:** `unit/verify` [`TestSessionRFC4271NextHopMixedUpdateAndAddressChange`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_next_hop_test.go#L15). **positive:** `unit/verify` [`TestSessionRFC4271NextHopSemantics`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L205). **negative:** `unit/verify` [`TestSessionRFC4271IBGPNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_next_hop_test.go#L63). **negative:** `unit/verify` [`TestSessionRFC4271NextHopMixedUpdateAndAddressChange`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_next_hop_test.go#L16). **negative:** `unit/verify` [`TestSessionRFC4271NextHopSemantics`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L206) |
| `RFC4271-6.3-12` | If the path is syntactically incorrect, then the Error Subcode MUST be set to Malformed AS_PATH. (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L112). **negative:** `unit/verify` [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L113) |
| `RFC4271-6.3-13` | If the UPDATE message is received from an external peer, the local system MAY check whether the leftmost (with respect to the position of octets in the protocol message) AS in the AS_PATH attribute is equal to the autonomous system number of the peer that sent the message. If the check determines this is not the case, the Error Subcode MUST be set to Malformed AS_PATH. (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestRFC4271LeftmostASMismatchIsMalformedASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_rfc7611_ingress_test.go#L29). **negative:** `unit/verify` [`TestRFC4271LeftmostASMismatchIsMalformedASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_rfc7611_ingress_test.go#L32) |
| `RFC4271-6.3-14` | If an optional attribute is recognized, then the value of this attribute MUST be checked. If an error is detected, the attribute MUST be discarded, and the Error Subcode MUST be set to Optional Attribute Error. The Data field MUST contain the attribute (type, length, and value). (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L114). **negative:** `unit/verify` [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L115) |
| `RFC4271-6.3-15` | If any attribute appears more than once in the UPDATE message, then the Error Subcode MUST be set to Malformed Attribute List (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestRFC4271UpdateMalformedAttributeList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_update_error_rfc4271_test.go#L59). **negative:** `unit/verify` [`TestRFC4271UpdateMalformedAttributeList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_update_error_rfc4271_test.go#L61) |
| `RFC4271-6.3-16` | If the field is syntactically incorrect, then the Error Subcode MUST be set to Invalid Network Field. (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestSessionRFC4271RetainedUpdateNotifications`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L158). **negative:** `unit/verify` [`TestSessionRFC4271RetainedUpdateNotifications`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L159) |
| `RFC4271-6.3-17` | An UPDATE message that contains correct path attributes, but no NLRI, SHALL be treated as a valid UPDATE message (§6.3) | SHALL | 6.3 | **positive:** `unit/verify` [`TestRFC4271UpdateWithoutNLRIIsValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_update_error_rfc4271_test.go#L125). **negative:** `unit/verify` [`TestRFC4271UpdateWithoutNLRIIsValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_update_error_rfc4271_test.go#L128) |
| `RFC4271-8.2.2-19` | In response to an indication that the TCP connection is successfully established (Event 16 or Event 17), the second connection SHALL be tracked until it sends an OPEN message (§8.2.2) | SHALL | 8.2.2 | **positive:** `unit/verify` [`TestSessionRFC4271EstablishedCollisionWaitsForOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L246). **negative:** `unit/verify` [`TestSessionRFC4271EstablishedCollisionWaitsForOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L247) |
| `RFC4271-9-4` | Otherwise, if the Adj-RIB-In has no route with NLRI identical to the new route, the new route SHALL be placed in the Adj-RIB-In. (§9) | SHALL | 9 | **positive:** `unit/verify` [`TestRFC4271AdjRIBInPlacesNewRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rib_rfc4271_test.go#L41). **negative:** `unit/verify` [`TestRFC4271AdjRIBInPlacesNewRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rib_rfc4271_test.go#L44) |
| `RFC4271-10-4` | The suggested default amount of jitter SHALL be determined by multiplying the base value of the appropriate timer by a random factor, which is uniformly distributed in the range from 0.75 to 1.0 (§10) | SHALL | 10 | **positive:** `unit/verify` [`TestTimersRFC4271Jitter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/timer_jitter_test.go#L11). **negative:** `unit/verify` [`TestTimersRFC4271Jitter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/timer_jitter_test.go#L12) |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4271-5.1.6-1`](#rfc4271-5.1.6-1) A BGP speaker that receives a route with the ATOMIC_AGGREGATE attribute MUST NOT make any NLRI of that route more specific (as defined in 9.1.4) when advertising this route to other BGP speakers. (§5.1.6) | no test | no test carries this requirement id; annotated {not-applicable}: the obligation binds the RECEIVER/re-advertiser, and ze is one -- it stores a received ATOMIC_AGGREGATE and copies it through on readvertisement (internal/component/bgp/reactor/peer_rib_routes.go:141) -- but the prohibited act has no producer. `grep -rniE "more specific\|deaggregat\|de-aggregat\|disaggregat" --include=*.go internal/component/bgp/ \| grep -v _test` returns only substring hits inside `encodeAggregatorValue` and `attrCodeAggregator` (internal/component/bgp/reactor/filter_delta.go:294,396, internal/component/bgp/message/rfc7606.go:64,421); no code path splits a prefix. Both readvertisement encoders write the stored route's own prefix verbatim through nlri.WriteNLRI (internal/component/bgp/reactor/peer_rib_routes.go:103-104), so the advertised NLRI is byte-identical to what was received and can be neither more nor less specific. With no length-altering producer there is no behavior to exercise in either polarity |
| [`RFC4271-6.1-1`](#rfc4271-6.1-1) All errors detected while processing the Message Header MUST be indicated by sending the NOTIFICATION message with the Error Code Message Header Error. (§6.1) | {gap}, no test | one class of header error is detected but never reported. A bad marker or a Length below 19 makes ParseHeader return a bare sentinel (internal/component/bgp/message/header.go:96-108), and the read loop turns that into an FSM event and a returned error with no NOTIFICATION sent (internal/component/bgp/reactor/session_read.go:98-102). The per-type and over-maximum length errors on the following lines do send Message Header Error (session_read.go:105-117) |
| [`RFC4271-6.1-2`](#rfc4271-6.1-2) If the Marker field of the message header is not as expected, then a synchronization error has occurred and the Error Subcode MUST be set to Connection Not Synchronized. (§6.1) | {gap}, no test | NotifyHeaderConnectionNotSync is declared (internal/component/bgp/message/notification.go:52) but no producer ever sends it. ParseHeader returns ErrInvalidMarker, a plain sentinel carrying no NOTIFICATION (internal/component/bgp/message/header.go:96-99), and the read loop's marker-error branch sends nothing before returning (internal/component/bgp/reactor/session_read.go:98-102) |
| [`RFC4271-6.1-3`](#rfc4271-6.1-3) then the Error Subcode MUST be set to Bad Message Length. (§6.1) | {gap}, no test | RFC 4271 §6.1 lists five length conditions and ze reports only four of them. The per-type minima and the 4096/65535 ceiling do produce a conformant Notification -- ValidateLength and ValidateLengthWithMax return a *Notification carrying NotifyHeaderBadLength and the two big-endian octets of the offending Length (internal/component/bgp/message/header.go:155-171 and :207-213), which the read loop sends before closing (internal/component/bgp/reactor/session_read.go:105-117). The first listed condition, "Length field of the message header is less than 19", does not: ParseHeader returns the bare sentinel ErrInvalidLength with no Notification and no Data (internal/component/bgp/message/header.go:106-108), and the read loop logs an FSM event and returns without writing anything (internal/component/bgp/reactor/session_read.go:98-102). The same code fact is recorded as the NOTIFICATION-absence gap on RFC4271-6.1-1. Disclosed in docs/features/rfc-status.md RFC 4271 row |
| [`RFC4271-6.1-4`](#rfc4271-6.1-4) If the Type field of the message header is not recognized, then the Error Subcode MUST be set to Bad Message Type. The Data field MUST contain the erroneous Type field. (§6.1) | {gap}, no test | an unknown message type is reported with the wrong subcode and the wrong Data. handleUnknownType sends Message Header Error with subcode 0 and a human-readable text string rather than subcode 3 (Bad Message Type) with the erroneous Type octet (internal/component/bgp/reactor/session_handlers.go:20-36); NotifyHeaderBadType is declared at internal/component/bgp/message/notification.go:54 and has no producer |
| [`RFC4271-6.2-3`](#rfc4271-6.2-3) All errors detected while processing the OPEN message MUST be indicated by sending the NOTIFICATION message with the Error Code OPEN Message Error. (§6.2) | {gap}, no test | one class of OPEN error is detected and never reported. UnpackOpen returns the bare sentinel ErrShortRead when the body is under 10 octets or when the Optional Parameters Length (standard or RFC 9072 extended) overruns the body (internal/component/bgp/message/open.go:167-168, :193-194, :199-200, :209-210), and handleOpen turns that into an FSM event and a returned error, writing no NOTIFICATION and not even closing the connection (internal/component/bgp/reactor/session_handlers.go:43-47); session_read.go:264 only propagates it. Every other OPEN error path does send Error Code 2 -- unsupported version (session_handlers.go:54-60), unacceptable Hold Time (:70-77) and a malformed capability (rejectOpenCapabilityError, :185-199) -- so the obligation holds everywhere except the decode failure. Disclosed in docs/features/rfc-status.md RFC 4271 row |
| [`RFC4271-8.2.1-3`](#rfc4271-8.2.1-3) For each incoming connection, a state machine MUST be instantiated (§8.2.1) | {gap}, no test | an incoming connection does not get its own state machine. acceptOrReject hands the accepted connection to the peer's existing session (internal/component/bgp/reactor/reactor_connection.go:117-163), and a connection queued for collision resolution is read raw by handlePendingCollision with no FSM behind it (internal/component/bgp/reactor/reactor_connection.go:196-249). An FSM is created per session, i.e. per connection attempt of a configured peer (internal/component/bgp/reactor/session.go:396), not per inbound connection |
| [`RFC4271-3.1-2`](#rfc4271-3.1-2) The next hop for each of these routes MUST be resolvable via the local BGP speaker's Routing Table. (§3.2) | {gap}, no test | the BGP Loc-RIB install performs no reachability check on the route's next hop. mirrorToLocRIB inserts the winning path with whatever next-hop address the attribute carried (internal/component/bgp/plugins/rib/rib_bestchange.go:797-830), and the candidate gather step filters only on SRv6 ineligibility (internal/component/bgp/plugins/rib/rib_commands.go:1039-1057). Resolvability is enforced downstream at FIB-install time, which removes the route from the routing table but leaves it in the Loc-RIB |
| [`RFC4271-5.1.7-1`](#rfc4271-5.1.7-1) A BGP speaker that performs route aggregation MAY add the AGGREGATOR attribute, which SHALL contain its own AS number and IP address. (§5.1.7) | no test | no test carries this requirement id; annotated {not-applicable}: ze never performs aggregation, so it never adds an AGGREGATOR of its own. The same grep as RFC4271-5.1.6-1 finds no aggregation producer; AGGREGATOR is only interned from the wire (internal/component/bgp/plugins/rib/storage/attrparse.go:96-102), replayed on readvertise (internal/component/bgp/plugins/rib/storage/familyrib.go:817-819) or emitted from operator configuration (internal/component/bgp/message/update_build_grouped.go:141-148) |
| [`RFC4271-9.1.2-1`](#rfc4271-9.1.2-1) If the NEXT_HOP attribute of a BGP route depicts an address that is not resolvable, or if it would become unresolvable if the route was installed in the routing table, the BGP route MUST be excluded from the Phase 2 decision function. (§9.1.2) | {gap}, no test | an unresolvable NEXT_HOP does not exclude the route from Phase 2. gatherCandidatesLocked skips only SRv6-ineligible entries (internal/component/bgp/plugins/rib/rib_commands.go:1039-1057), and extractCandidate uses the next hop solely to look up an IGP cost (internal/component/bgp/plugins/rib/rib_commands.go:1123-1131), so an unreachable next hop yields a cost of zero and the route competes normally |
| [`RFC4271-9.1.2-4`](#rfc4271-9.1.2-4) If either the immediate next-hop or the IGP cost to the NEXT_HOP (where the NEXT_HOP is resolved through an IGP route) changes, Phase 2 Route Selection MUST be performed again. (§9.1.2) | {gap}, no test | nothing re-runs Phase 2 when the immediate next-hop or the IGP cost to the NEXT_HOP changes. The only entry points to checkBestPathChange are the UPDATE ingest path and the peer-state paths (internal/component/bgp/plugins/rib/rib_structured.go:271-286), and the IGP cost function is a passive lookup registered once with no invalidation callback (internal/component/bgp/plugins/rib/bestpath.go:30-43) |
| [`RFC4271-9.1.2.1-2`](#rfc4271-9.1.2.1-2) Unresolvable routes SHALL be removed from the Loc-RIB and the routing table. (§9.1.2) | {gap}, no test | an unresolvable route is not removed from the Loc-RIB. The only Loc-RIB removal in the BGP plugin is the no-candidate-remains branch of checkBestPathChange (internal/component/bgp/plugins/rib/rib_bestchange.go:766-782), which is driven by the Adj-RIB-In losing its last path and never by next-hop resolvability; nothing in the plugin consults a resolver |
| [`RFC4271-9.1.2.2-2`](#rfc4271-9.1.2.2-2) If an implementation chooses to remove MULTI_EXIT_DISC, then the optional comparison on MULTI_EXIT_DISC, if performed, MUST be performed only among EBGP-learned routes. (§9.1.2.2) | {gap}, no test | the common egress guard is implemented, but normal BGP selected-route readvertisement has no runnable producer and no discriminating proof. `bgp-rib` records selected Loc-RIB state (internal/component/bgp/plugins/rib/rib_bestchange.go:738-790), route-server and route-reflector plugins forward cached UPDATEs instead (internal/component/bgp/plugins/rs/server.go:433-434 and internal/component/bgp/plugins/rr/rr.go:188-195), and BGP-to-BGP redistribution is rejected as same-protocol redistribution (internal/core/redistevents/registry.go:144-146) |
| [`RFC4271-9.2-2`](#rfc4271-9.2-2) A route SHALL NOT be installed in the Adj-Rib-Out unless the destination, and NEXT_HOP described by this route, may be forwarded appropriately by the Routing Table. (§9.1.3) | {gap}, no test | nothing gates Adj-RIB-Out installation on the destination and NEXT_HOP being forwardable. QueueAnnounce records the route unconditionally (internal/component/bgp/rib/outgoing.go:65-101), and the forwarding rails decide only on filters, family negotiation and the route-reflection rules (internal/component/bgp/reactor/forward_rs.go:295-333) |
| [`RFC4271-9.2-3`](#rfc4271-9.2-3) If a route in Loc-RIB is excluded from a particular Adj-RIB-Out, the previously advertised route in that Adj-RIB-Out MUST be withdrawn from service by means of an UPDATE message (§9.1.3) | {gap}, no test | a route excluded from a peer's Adj-RIB-Out by an egress filter is skipped silently, leaving the peer's previous advertisement in place instead of withdrawing it. Both forwarding rails `continue` on suppression with no withdrawal built (internal/component/bgp/reactor/forward_rs.go:320-333 and internal/component/bgp/reactor/reactor_api_forward.go:496-506); the one announce-to-withdraw conversion is LLGR-specific and filter-requested, not exclusion-driven (internal/component/bgp/reactor/reactor_api_forward.go:588-601) |
| [`RFC4271-9.2.1.1-2`](#rfc4271-9.2.1.1-2) Two UPDATE messages sent by a BGP speaker to a peer that advertise feasible routes and/or withdrawal of unfeasible routes to some common set of destinations MUST be separated by at least MinRouteAdvertisementIntervalTimer. (§9.2.1.1) | {gap}, no test | ze has no MinRouteAdvertisementIntervalTimer, so successive UPDATEs to a common set of destinations are not spaced. The timer set implements only ConnectRetry, Hold and Keepalive and records the omission in its own doc comment (internal/component/bgp/fsm/timer.go:34-42, "MinRouteAdvertisementIntervalTimer (Section 9.2.1.1) - not implemented here"); `grep -rniE 'minroute\|mrai' --include=*.go internal/` finds no producer |
| [`RFC4271-9.2.2.2-1`](#rfc4271-9.2.2.2-1) Routes that have different MULTI_EXIT_DISC attributes SHALL NOT be aggregated. (§9.2.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze never aggregates routes, so no producer can aggregate two routes with different MULTI_EXIT_DISC values. `grep -rniE 'aggregate-address\|AggregateRoute\|route aggregation' --include=*.go .` returns no hit outside rfc/ and plan/, and no code path synthesizes an aggregate route from more-specifics |
| [`RFC4271-9.2.2.2-2`](#rfc4271-9.2.2.2-2) ORIGIN attribute: If at least one route among routes that are aggregated has ORIGIN with the value INCOMPLETE, then the aggregated route MUST have the ORIGIN attribute with the value INCOMPLETE. (§9.2.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze never aggregates routes, so no producer computes an aggregate ORIGIN. ORIGIN is only parsed (internal/core/bgp/attribute/origin.go:146-160), interned (internal/component/bgp/plugins/rib/storage/attrparse.go) and re-emitted verbatim (internal/component/bgp/plugins/rib/storage/familyrib.go:799-801); the same aggregation grep returns nothing |
| [`RFC4271-9.2.2.2-3`](#rfc4271-9.2.2.2-3) NEXT_HOP: When aggregating routes that have different NEXT_HOP attributes, the NEXT_HOP attribute of the aggregated route SHALL identify an interface on the BGP speaker that performs the aggregation. (§9.2.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze never aggregates routes, so no producer chooses an aggregated NEXT_HOP. The only next-hop selection is per-route egress policy (internal/component/bgp/reactor/peer_forward_facts.go:153-193); the same aggregation grep returns nothing |
| [`RFC4271-9.2.2.2-4`](#rfc4271-9.2.2.2-4) ATOMIC_AGGREGATE: If at least one of the routes to be aggregated has ATOMIC_AGGREGATE path attribute, then the aggregated route SHALL have this attribute as well. (§9.2.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze never aggregates routes, so no producer decides whether an aggregate carries ATOMIC_AGGREGATE. The attribute is only decoded, stored and replayed (internal/core/bgp/attribute/simple.go:175-195, internal/component/bgp/plugins/rib/storage/familyrib.go:815-817); the same aggregation grep returns nothing |
| [`RFC4271-9.2.2.2-5`](#rfc4271-9.2.2.2-5) AGGREGATOR: Any AGGREGATOR attributes from the routes to be aggregated MUST NOT be included in the aggregated route. (§9.2.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze never aggregates routes, so no producer builds an aggregated route from which a contributing AGGREGATOR would have to be excluded. AGGREGATOR is only interned from the wire or emitted from operator configuration (internal/component/bgp/plugins/rib/storage/attrparse.go:96-102, internal/component/bgp/message/update_build_grouped.go:141-148); the same aggregation grep returns nothing |
| [`RFC4271-9.2.1.1-3`](#rfc4271-9.2.1.1-3) If new routes are selected multiple times while awaiting the expiration of MinRouteAdvertisementIntervalTimer, the last route selected SHALL be advertised at the end of MinRouteAdvertisementIntervalTimer. (§9.2.1.1) | {gap}, no test | with no MinRouteAdvertisementIntervalTimer there is no expiry at which a last-selected route could be advertised. The timer is absent by design note (internal/component/bgp/fsm/timer.go:39) and no producer buffers a pending best-route advertisement against such a timer; best-path changes are published as they are computed (internal/component/bgp/plugins/rib/rib_bestchange.go:832-880) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4271-4.1-1`](#rfc4271-4.1-1)

This 16-octet field is included for compatibility; it MUST be set to all ones. (§4.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The sentence binds the sender. Forbidden: ze writing a marker octet other than 0xFF. TestRFC4271MarkerAllOnesOnSend asserts data[i]==0xFF for KEEPALIVE and NOTIFICATION only; OPEN and UPDATE encoders are not asserted in this row's units. The tagged negative (ParseHeader -> ErrInvalidMarker) proves the Section 6.1 receive check, a neighbouring rule, and the row carries no {single-polarity} marker.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271MarkerNotAllOnesRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L42) | unit/verify | unproven |
| positive | [`TestRFC4271MarkerAllOnesOnSend`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L19) | unit/verify | unproven |

### [`RFC4271-4.1-2`](#rfc4271-4.1-2)

Therefore, the Length field MUST have the smallest value required, given the rest of the message. (§4.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Sender obligation. TestRFC4271SmallestLengthOnSend asserts the exact Length for a KEEPALIVE and one UPDATE; OPEN and NOTIFICATION Lengths are not asserted. The tagged negative (a padded KEEPALIVE refused as Bad Message Length) is the Section 6.1 receive check, a neighbouring rule, not a violation of what ze sends; no {single-polarity} marker.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271NonSmallestLengthRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L95) | unit/verify | unproven |
| positive | [`TestRFC4271SmallestLengthOnSend`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L62) | unit/verify | unproven |

### [`RFC4271-4.1-3`](#rfc4271-4.1-3)

The value of the Length field MUST always be at least 19 and no greater than 4096 (§4.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Units prove only the receive bound check (18 -> ErrInvalidLength, 4097/65535 -> 1/2 Bad Message Length, 19..4096 accepted), which is the Section 6.1 rule. The send half of 'MUST always be ... no greater than 4096' (ze never emits a message above 4096 without Extended Message, e.g. an UPDATE the builder must split) has no assertion in a tagged unit.

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

For well-known attributes and for optional non-transitive attributes, the Partial bit MUST be set to 0. (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. send side (attribute encoders), receive-path clear (enforceRFC7606), readvertise rails (rib CommitService, storage ToWireBytes) and the RS relay each fed 0x60/0xA0 and asserted 0x40/0x80 on the wire, COMMUNITIES 0xE0 kept

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271PartialBitClearedOnReadvertisedWellKnown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L47) | unit/verify | unproven |
| negative | [`TestRFC4271PartialClearedWhenTheRailReadvertises`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc4271_partial_test.go#L75) | unit/verify | revert, verified |
| negative | [`TestRFC4271PartialNotStampedOnExcludedClasses`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L170) | unit/verify | unproven |
| positive | [`TestRFC4271PartialClearedOnTheRelayedWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_relay_partial_test.go#L48) | unit/verify | revert, verified |
| positive | [`TestRFC4271PartialNotSetOnRecognizedOrNonTransitive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1116) | unit/verify | unproven |
| positive | [`TestRFC4271PartialBitClearOnSend`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L55) | unit/verify | unproven |

### [`RFC4271-4.3-3`](#rfc4271-4.3-3)

The lower-order four bits of the Attribute Flags octet are unused. They MUST be zero when sent (§4.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. {single-polarity} accepted, but the assertion (flags&0x0F==0) covers only the five well-known encoders plus MED via WriteAttrTo. Opaque/relayed attributes are written with the flags octet as received (attribute/opaque.go Flags 'preserved for forwarding'; no low-nibble mask found in attribute/ or reactor/), so a received low nibble set on an unrecognized transitive attribute is untested on the relay and forward rails and may be re-sent non-zero.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4271AttributeFlagsLowNibbleZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L80) | unit/verify | unproven |

### [`RFC4271-4.3-4`](#rfc4271-4.3-4)

The lower-order four bits of the Attribute Flags octet are unused. They MUST be zero when sent and MUST be ignored when received. (§4.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The quote carries both clauses. 'ignored when received': TestRFC4271AttrFlagsLowNibbleIgnoredOnReceive asserts RFC7606ActionNone with 0x4F flags on ORIGIN/AS_PATH/NEXT_HOP (validator only). 'zero when sent': no assertion in this row's units (it lives under RFC4271-4.3-3, itself limited to the local encoders). The tagged negative (Optional/Transitive bits still enforced) proves RFC 7606 3(c), a neighbouring rule.

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

If the negotiated Hold Time interval is zero, then periodic KEEPALIVE messages MUST NOT be sent. (§4.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestKeepaliveWithZeroHoldTime proves Timers with SetHoldTime(0) does not start or fire the keepalive timer. Nothing asserts that a session whose NEGOTIATED hold time is 0 (configured non-zero, peer sent 0) feeds 0 into the timers, nor that no KEEPALIVE reaches the wire; a session passing the configured hold time would keep both units green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestKeepaliveWithZeroHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/timer_test.go#L365) | unit/verify | unproven |
| positive | [`TestTimersKeepaliveTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/timer_test.go#L144) | unit/verify | unproven |

### [`RFC4271-6-1`](#rfc4271-6-1)

If no Error Subcode is specified, then a zero MUST be used. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a non-zero subcode when none is specified. TestRFC4271NotificationUnspecifiedSubcodeIsZero asserts data[HeaderLen+1]==0 on the packed NOTIFICATION and 0 after decode. The negative is a same-producer contrast (three specified subcodes packed verbatim and non-zero), which rules out an encoder that writes a constant zero, so the zero is shown to come from absence.

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
| positive | [`open-hold-time-peer-lower-wins.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/open-hold-time-peer-lower-wins.ci#L3) | functional/verify | unproven |

### [`RFC4271-4.2-2`](#rfc4271-4.2-2)

Upon receipt of an OPEN message, a BGP speaker MUST calculate the value of the Hold Timer by using the smaller of its configured Hold Time and the Hold Time received in the OPEN message. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive cases local-smaller, peer-smaller and equal would fail on max() or on always-local; the negative (zero from either side) is a mirror of min rather than a violating input

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegotiateWith_HoldTimeZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L74) | unit/verify | unproven |
| positive | [`TestNegotiateWith_HoldTimeMinOfBoth`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L45) | unit/verify | unproven |

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

Some of these attributes are mandatory and MUST be included in every UPDATE message that contains NLRI. (§5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. both tagged units are receive-side (ValidateUpdateRFC7606 accepts ORIGIN+AS_PATH+NEXT_HOP with NLRI, refuses a missing ORIGIN). The sentence binds the sender: no tagged unit asserts that an UPDATE ze builds with NLRI carries all three mandatory attributes

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271WellKnownAttributeErrorsAreCaught`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L497) | unit/verify | unproven |
| positive | [`TestRFC4271WellKnownAttributesAreRecognized`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L466) | unit/verify | unproven |

### [`RFC4271-5-3`](#rfc4271-5-3)

If a path with an unrecognized transitive optional attribute is accepted and passed to other BGP peers, then the unrecognized transitive optional attribute of that path MUST be passed, along with the path, to other BGP peers with the Partial bit in the Attribute Flags octet set to 1. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. attribute walk (both header forms), live receive path enforceRFC7606 and the .ci assert 0xC0 -> 0xE0 on an unrecognized optional transitive; negatives assert well-known, optional non-transitive and recognized optional transitive untouched

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271PartialNotSetOnRecognizedOrNonTransitive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1113) | unit/verify | unproven |
| negative | [`TestRFC4271PartialNotStampedOnExcludedClasses`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L167) | unit/verify | unproven |
| positive | [`TestRFC4271PartialSetOnUnrecognizedTransitiveOptional`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1084) | unit/verify | unproven |
| positive | [`TestRFC4271PartialStampedOnUnrecognizedTransitive`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L132) | unit/verify | unproven |
| positive | [`rfc4271-partial-unknown-transitive.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc4271-partial-unknown-transitive.ci#L27) | functional/verify | unproven |

### [`RFC4271-5-4`](#rfc4271-5-4)

If a path with a recognized, transitive optional attribute is accepted and passed along to other BGP peers and the Partial bit in the Attribute Flags octet is set to 1 by some previous AS, it MUST NOT be set back to 0 by the current AS. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. receive path, attribute walk and storage readvertise (including the extended-length reframing branch) each keep a Partial bit a previous AS set on recognized and unrecognized transitive attributes

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

The receiver of an UPDATE message MUST be prepared to handle path attributes within UPDATE messages that are out of order. (§5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Positive asserts only that ValidateUpdateRFC7606 returns None for descending and interleaved orders; no unit shows the live receive path indexing and installing out-of-order attributes. The tagged negative (malformed ORIGIN or truncated attribute still treat-as-withdraw) proves RFC 7606 malformation detection, a neighbouring rule; the acceptance obligation carries no {single-polarity} marker.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271OutOfOrderDoesNotMaskMalformation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L323) | unit/verify | unproven |
| positive | [`TestRFC4271AttributesOutOfOrderAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L293) | unit/verify | unproven |

### [`RFC4271-4.3-5`](#rfc4271-4.3-5)

However, a BGP speaker MUST be able to process UPDATE messages in this form. (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity annotated; structured, pool and injection paths each process an UPDATE naming 10.0.0.0/8 in WITHDRAWN and NLRI and assert the prefix installed with no withdrawal

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRIBInjectSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L119) | unit/verify | unproven |
| positive | [`TestRIBPoolPathSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L85) | unit/verify | unproven |
| positive | [`TestRIBSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L39) | unit/verify | unproven |

### [`RFC4271-5.1.3-1`](#rfc4271-5.1.3-1)

A route originated by a BGP speaker SHALL NOT be advertised to a peer using an address of that peer as NEXT_HOP. (§5.1.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. originated-route units (writeUpdateGated/SendAnnounce tests and originated-nexthop-peer-own.ci) assert the route naming the peer own address is withheld from that peer while other next hops are written; forward-rail units extend the refusal to relayed routes, which this sentence does not name

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEgressNextHopIsPeerOwnReadsTheRewrittenAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L286) | unit/verify | unproven |
| negative | [`TestForwardRSWithholdsRouteWhoseNextHopIsTheClientsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L255) | unit/verify | unproven |
| negative | [`TestForwardWithholdsRouteWhoseNextHopIsTheDestinationsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L186) | unit/verify | unproven |
| negative | [`TestSendAnnounceWithholdsRouteWithPeerOwnNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L466) | unit/verify | unproven |
| negative | [`TestSendUpdateWithholdsOriginatedRouteWithPeerOwnNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L429) | unit/verify | unproven |
| negative | [`checkSelfNextHopWithheld`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L960) | interop/nightly | unproven |
| negative | [`originated-nexthop-peer-own.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/originated-nexthop-peer-own.ci#L10) | functional/verify | unproven |
| positive | [`TestEgressNextHopIsPeerOwnReadsTheRewrittenAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L282) | unit/verify | unproven |
| positive | [`TestForwardRSWithholdsRouteWhoseNextHopIsTheClientsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L252) | unit/verify | unproven |
| positive | [`TestForwardWithdrawsFromDestinationWhoseNextHopIsItsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L217) | unit/verify | unproven |
| positive | [`TestForwardWithholdsRouteWhoseNextHopIsTheDestinationsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L181) | unit/verify | unproven |
| positive | [`TestSendAnnounceWithholdsRouteWithPeerOwnNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L464) | unit/verify | unproven |
| positive | [`TestSendUpdateWithholdsOriginatedRouteWithPeerOwnNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop_test.go#L425) | unit/verify | unproven |
| positive | [`checkSelfNextHopWithheld`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L959) | interop/nightly | unproven |
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

A BGP speaker MUST be able to support the disabling advertisement of third party NEXT_HOP attributes in order to handle imperfectly bridged media. (§5.1.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Units assert only internal state: precomputeNextHop arms nhModeSelf4 for NextHopSelf and resolveNextHop errors without a local address. No unit shows a configured next-hop self replacing a third-party NEXT_HOP on the forward rail wire, and the 'fails closed' negative leaves nhMode none, which on the forward rail passes the third-party next hop through unchanged; no operator-config path is exercised.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ThirdPartyNextHopDisableFailsClosed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L373) | unit/verify | revert, verified |
| positive | [`TestRFC4271ThirdPartyNextHopCanBeDisabled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L341) | unit/verify | unproven |

### [`RFC4271-5.1.4-1`](#rfc4271-5.1.4-1)

The MULTI_EXIT_DISC attribute received from a neighboring AS MUST NOT be propagated to other neighboring ASes. (§5.1.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. forward_med_test and med-not-propagated-across-as.ci assert attribute 4 stripped toward a plain eBGP peer; negatives keep it toward iBGP, RS clients and for a locally originated metric (med-locally-set-reaches-peer.ci)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestForwardKeepsFilterSetMED`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L322) | unit/verify | unproven |
| negative | [`TestForwardSuppressesReceivedMEDToAnotherAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L226) | unit/verify | unproven |
| negative | [`TestForwardWritesLocallySetMED`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L289) | unit/verify | unproven |
| negative | [`TestMEDPropagationAllowedTo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L426) | unit/verify | unproven |
| negative | [`checkMEDAcrossAS`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L198) | interop/nightly | unproven |
| negative | [`med-locally-set-reaches-peer.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-locally-set-reaches-peer.ci#L4) | functional/verify | unproven |
| negative | [`med-not-propagated-across-as.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-not-propagated-across-as.ci#L8) | functional/verify | unproven |
| positive | [`TestForwardSuppressesReceivedMEDToAnotherAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L221) | unit/verify | unproven |
| positive | [`checkMEDAcrossAS`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L194) | interop/nightly | unproven |
| positive | [`med-not-propagated-across-as.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-not-propagated-across-as.ci#L4) | functional/verify | unproven |

### [`RFC4271-5.1.4-4`](#rfc4271-5.1.4-4)

A BGP speaker MUST implement a mechanism (based on local configuration) that allows the MULTI_EXIT_DISC attribute to be removed from a route (§5.1.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseModifyDefsMEDRemove`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/modify_test.go#L647) | unit/verify | unproven |
| negative | [`TestMEDRemovalMechanismIsConfigurable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L475) | unit/verify | unproven |
| negative | [`TestMEDRemoveDirectiveIsValueless`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L568) | unit/verify | unproven |
| negative | [`checkMEDRemovalConfiguration`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L320) | interop/nightly | unproven |
| positive | [`TestParseModifyDefsMEDRemove`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/modify_test.go#L642) | unit/verify | unproven |
| positive | [`TestMEDRemovalMechanismIsConfigurable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L468) | unit/verify | unproven |
| positive | [`checkMEDRemovalConfiguration`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L316) | interop/nightly | unproven |
| positive | [`med-removal-configured.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-configured.ci#L4) | functional/verify | unproven |

### [`RFC4271-5.1.4-2`](#rfc4271-5.1.4-2)

If a BGP speaker is configured to remove the MULTI_EXIT_DISC attribute from a route, then this removal MUST be done prior to determining the degree of preference of the route and prior to performing route selection (Decision Process phases 1 and 2). (§5.1.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. modify_test, forward_med_test and three .ci tests assert the del med directive runs on the import chain before the RIB stores the route, is refused on export, and removes only attribute 4

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestHandleFilterUpdateMEDRemoveIsImportOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/modify_test.go#L722) | unit/verify | unproven |
| negative | [`TestMEDRemovalMechanismIsConfigurable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L488) | unit/verify | unproven |
| negative | [`med-removal-before-decision.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-before-decision.ci#L10) | functional/verify | unproven |
| negative | [`med-removal-export-refused.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-export-refused.ci#L4) | functional/verify | unproven |
| positive | [`TestHandleFilterUpdateMEDRemoveIsImportOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/modify_test.go#L715) | unit/verify | unproven |
| positive | [`TestMEDRemovalMechanismIsConfigurable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L480) | unit/verify | unproven |
| positive | [`med-removal-before-decision.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-before-decision.ci#L4) | functional/verify | unproven |
| positive | [`med-removal-configured.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-configured.ci#L8) | functional/verify | unproven |

### [`RFC4271-5.1.5-1`](#rfc4271-5.1.5-1)

LOCAL_PREF is a well-known attribute that SHALL be included in all UPDATE messages that a given BGP speaker sends to other internal peers. (§5.1.5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. tagged units cover the originated announce rails (WriteAnnounceUpdate default 100, batch and queued API rails) toward iBGP. The sentence says ALL UPDATEs to internal peers: no unit relays a route that arrived without LOCAL_PREF (eBGP-learned, 5.1.5-3 discards it) to an internal peer, and applyFactsLocalPref (forward_local_pref.go) only strips, never adds, so the forward rail may omit it. Unverified whether another step adds it

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestForwardLocalPrefStrippedToExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_local_pref_test.go#L45) | unit/verify | unproven |
| negative | [`TestAnnounceStripsLocalPrefTowardExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_origin_test.go#L307) | unit/verify | revert, verified |
| negative | [`TestRFC4271LocalPrefOmittedForExternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L107) | unit/verify | unproven |
| positive | [`TestRFC4271LocalPrefIncludedForInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L81) | unit/verify | unproven |

### [`RFC4271-5.1.5-2`](#rfc4271-5.1.5-2)

A BGP speaker MUST NOT include this attribute in UPDATE messages it sends to external peers, except in the case of BGP Confederations [RFC3065]. (§5.1.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. announce (WriteAnnounceUpdate), API batch and queued rails, forward rail and local-pref-strip-ebgp.ci assert no attribute 5 toward an external peer, including over a filter Set; ze has no confederation surface, so the exception is unreachable

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLocalPrefAllowedToIsTheOnlyAnswer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_local_pref_test.go#L146) | unit/verify | unproven |
| negative | [`TestRFC4271LocalPrefIncludedForInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L83) | unit/verify | unproven |
| negative | [`local-pref-strip-ebgp.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/local-pref-strip-ebgp.ci#L19) | functional/verify | unproven |
| positive | [`TestForwardLocalPrefStripBeatsAFilterSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_local_pref_test.go#L113) | unit/verify | unproven |
| positive | [`TestForwardLocalPrefStrippedToExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_local_pref_test.go#L41) | unit/verify | unproven |
| positive | [`TestAnnounceStripsLocalPrefTowardExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_origin_test.go#L305) | unit/verify | revert, verified |
| positive | [`TestRFC4271LocalPrefOmittedForExternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L104) | unit/verify | unproven |
| positive | [`checkLocalPrefStrip`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L94) | interop/nightly | unproven |
| positive | [`local-pref-strip-ebgp.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/local-pref-strip-ebgp.ci#L14) | functional/verify | unproven |

### [`RFC4271-5.1.5-3`](#rfc4271-5.1.5-3)

If it is contained in an UPDATE message that is received from an external peer, then this attribute MUST be ignored by the receiving speaker, except in the case of BGP Confederations [RFC3065]. (§5.1.5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Units assert only the action ValidateUpdateRFC7606 selects (AttributeDiscard of code 5 on external, None on internal). No unit shows the live receive path removing the LOCAL_PREF before the RIB, or best-path ignoring an eBGP-received LOCAL_PREF; a session that computed the discard and then stored the attribute would keep both units green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271LocalPrefIgnoredOnExternalSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L589) | unit/verify | unproven |
| positive | [`TestRFC4271LocalPrefKeptOnInternalSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L566) | unit/verify | unproven |

### [`RFC4271-5.1.5-4`](#rfc4271-5.1.5-4)

The higher degree of preference MUST be preferred. (§5.1.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. SelectBest cases: higher wins, lower loses, equal falls through to AS_PATH; an inverted comparison fails two cases

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBestPath_LocalPref`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L199) | unit/verify | unproven |
| positive | [`TestBestPath_LocalPref`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L197) | unit/verify | unproven |

### [`RFC4271-5.1.6-1`](#rfc4271-5.1.6-1)

A BGP speaker that receives a route with the ATOMIC_AGGREGATE attribute MUST NOT make any NLRI of that route more specific (as defined in 9.1.4) when advertising this route to other BGP speakers. (§5.1.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-5.1.6-1, so no unit is bound to it.

### [`RFC4271-6.1-1`](#rfc4271-6.1-1)

All errors detected while processing the Message Header MUST be indicated by sending the NOTIFICATION message with the Error Code Message Header Error. (§6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-6.1-1, so no unit is bound to it.

### [`RFC4271-6.1-2`](#rfc4271-6.1-2)

If the Marker field of the message header is not as expected, then a synchronization error has occurred and the Error Subcode MUST be set to Connection Not Synchronized. (§6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-6.1-2, so no unit is bound to it.

### [`RFC4271-6.1-3`](#rfc4271-6.1-3)

then the Error Subcode MUST be set to Bad Message Length. (§6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-6.1-3, so no unit is bound to it.

### [`RFC4271-6.1-4`](#rfc4271-6.1-4)

If the Type field of the message header is not recognized, then the Error Subcode MUST be set to Bad Message Type. The Data field MUST contain the erroneous Type field. (§6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-6.1-4, so no unit is bound to it.

### [`RFC4271-6.2-3`](#rfc4271-6.2-3)

All errors detected while processing the OPEN message MUST be indicated by sending the NOTIFICATION message with the Error Code OPEN Message Error. (§6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-6.2-3, so no unit is bound to it.

### [`RFC4271-6.3-1`](#rfc4271-6.3-1)

All errors detected while processing the UPDATE message MUST be indicated by sending the NOTIFICATION message with the Error Code UPDATE Message Error. (§6.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. 'All errors' is proven by one class: TestRFC4271UpdateErrorReportedAsUpdateMessageError asserts Error Code 3 for a duplicate MP_REACH_NLRI only. The other UPDATE errors RFC 7606 still resolves by session reset (attribute list overrun, NLRI field) are asserted under other rows, not in this row's units.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConformantUpdateSendsNoUpdateError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L696) | unit/verify | unproven |
| positive | [`TestRFC4271UpdateErrorReportedAsUpdateMessageError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L633) | unit/verify | unproven |

### [`RFC4271-6.7-1`](#rfc4271-6.7-1)

However, the Cease NOTIFICATION message MUST NOT be used when a fatal error indicated by this section does exist. (§6.7)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The sentence covers every fatal error of Section 6. The negative (TestRFC4271UpdateErrorReportedAsUpdateMessageError) asserts code 3 and not Cease for one UPDATE error, a duplicate MP_REACH_NLRI. No tagged unit asserts that a message header error, an OPEN error, a hold timer expiry or an FSM error is not reported as Cease, so a Cease on any of those passes. The positive calls checkPrefixLimits directly.

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

If the HoldTimer_Expires event occurs (Event 10), the local system: - sends a NOTIFICATION message with the Error Code Hold Timer Expired (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: hold expiry with no Hold Timer Expired NOTIFICATION. TestRFC4271HoldTimerExpirySendsNotification reads the production hold-expiry write and asserts 21 octets, type NOTIFICATION, code 4, subcode 0; it is red on a bare close or another code. Negative: at 1ms before expiry nothing is written, red on a NOTIFICATION sent early. deadpeer-holddown.ci is byte-exact.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271HoldTimerNotYetExpiredSendsNoNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L836) | unit/verify | unproven |
| positive | [`TestRFC4271HoldTimerExpirySendsNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L807) | unit/verify | unproven |
| positive | [`deadpeer-holddown.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/deadpeer-holddown.ci#L3) | functional/verify | unproven |

### [`RFC4271-8.2.2-2`](#rfc4271-8.2.2-2)

If the HoldTimer_Expires event occurs (Event 10), the local system: - sends a NOTIFICATION message with the Error Code Hold Timer Expired, - sets the ConnectRetryTimer to zero (§8.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. ConnectRetryTimer clause: the positive asserts the armed timer is stopped after expiry and the negative asserts it survives a live session, so both polarities exist. NOTIFICATION clause: the positive scans the wire for code 4, but the negative (TestRFC4271NoHoldExpiryLeavesTheSessionIntact) discards the wire. A NOTIFICATION sent while the session is still live passes both tagged units. That polarity is in 8.2.2-1's units, which carry no tag for this row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271NoHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1000) | unit/verify | unproven |
| positive | [`TestRFC4271HoldExpiryRunsTheEvent10ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L929) | unit/verify | unproven |

### [`RFC4271-8.2.2-3`](#rfc4271-8.2.2-3)

If the HoldTimer_Expires event occurs (Event 10), the local system: - sends a NOTIFICATION message with the Error Code Hold Timer Expired, - sets the ConnectRetryTimer to zero, - releases all BGP resources (§8.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. 'releases all BGP resources' is asserted only as IsKeepaliveTimerRunning and IsHoldTimerRunning false after expiry. Routes, buffers, the forward pool entry and the peer goroutines are not asserted, so a teardown that stops the two timers and leaks everything else passes. The NOTIFICATION clause has no negative in these units, as for 8.2.2-2.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271NoHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1002) | unit/verify | unproven |
| positive | [`TestRFC4271HoldExpiryRunsTheEvent10ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L932) | unit/verify | unproven |

### [`RFC4271-8.2.2-4`](#rfc4271-8.2.2-4)

If the HoldTimer_Expires event occurs (Event 10), the local system: - sends a NOTIFICATION message with the Error Code Hold Timer Expired, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection (§8.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The drop clause is proven: requireConnClosed after expiry, and the drain stays open inside the hold time. The quote also carries 'releases all BGP resources', which these units prove only for two timers (see 8.2.2-3), and the NOTIFICATION clause has no negative in them.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271NoHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1005) | unit/verify | unproven |
| positive | [`TestRFC4271HoldExpiryRunsTheEvent10ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L935) | unit/verify | unproven |

### [`RFC4271-8.2.2-5`](#rfc4271-8.2.2-5)

If the HoldTimer_Expires event occurs (Event 10), the local system: - sends a NOTIFICATION message with the Error Code Hold Timer Expired, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1, - (optionally) performs peer oscillation damping if the DampPeerOscillations attribute is set to TRUE, and - changes its state to Idle. (§8.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The Idle clause is proven: state Idle after expiry and Established inside the hold time. The quote carries the whole Event 10 list. 'increments the ConnectRetryCounter by 1' has no assertion in either tagged unit (it is asserted only at FSM level, under 8.2.2-9), and 'releases all BGP resources' is proven only for two timers (see 8.2.2-3).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271NoHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1007) | unit/verify | unproven |
| positive | [`TestRFC4271HoldExpiryRunsTheEvent10ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L938) | unit/verify | unproven |

### [`RFC4271-8.2.2-7`](#rfc4271-8.2.2-7)

In response to a ManualStart event (Event 1) or an AutomaticStart event (Event 3), the local system: - initializes all BGP resources for the peer connection, - sets ConnectRetryCounter to zero (§8.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The counter clause is proven: ManualStart in Idle zeroes a counter of 7 on both branches, and damped starts leave it. 'initializes all BGP resources for the peer connection' has no assertion. AutomaticStart (Event 3) is not driven, because Ze has no such event, and no {gap} or annotation records that absence.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterSurvivesDampedStart`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L75) | unit/verify | unproven |
| positive | [`TestRFC4271ConnectRetryCounterZeroedOnManualStart`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L42) | unit/verify | unproven |

### [`RFC4271-8.2.2-8`](#rfc4271-8.2.2-8)

If a ManualStop event (Event 2) is issued in the OpenSent state, the local system: - sends the NOTIFICATION with a Cease, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - sets the ConnectRetryCounter to zero (§8.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The FSM units assert only ConnectRetryCounter zeroing and the Idle state on ManualStop. The quote's first four clauses have no assertion in these units: the Cease NOTIFICATION in OpenSent, the ConnectRetryTimer zeroed, BGP resources released and the TCP connection dropped. An FSM whose ManualStop arm only reset the counter passes.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterNotZeroedByIdleManualStop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L130) | unit/verify | unproven |
| positive | [`TestRFC4271ConnectRetryCounterZeroedOnManualStop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L108) | unit/verify | unproven |

### [`RFC4271-8.2.2-9`](#rfc4271-8.2.2-9)

If the HoldTimer_Expires event occurs (Event 10), the local system: - sends a NOTIFICATION message with the Error Code Hold Timer Expired, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The FSM units assert only the counter step 3 to 4 and the healthy-traffic negative. The NOTIFICATION, ConnectRetryTimer, resources and TCP-drop clauses of this Event 10 list have no assertion in these units.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterQuietOnHealthyEstablishedTraffic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L174) | unit/verify | unproven |
| positive | [`TestRFC4271ConnectRetryCounterIncrementsOnHoldTimerExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L151) | unit/verify | unproven |

### [`RFC4271-8.2.2-10`](#rfc4271-8.2.2-10)

If the BGP message header checking (Event 21) or OPEN message checking detects an error (Event 22)(see Section 6.2), the local system: - sends a NOTIFICATION message with the appropriate error code, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The FSM units assert only the counter increment for Events 21 and 22, with Idle as the negative. The quote also carries the NOTIFICATION with the appropriate error code, the ConnectRetryTimer zeroed, BGP resources released and the TCP connection dropped, and none of these has an assertion in these units.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterNotIncrementedByIdleErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L236) | unit/verify | unproven |
| positive | [`TestRFC4271ConnectRetryCounterIncrementsOnHeaderAndOpenErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L210) | unit/verify | unproven |

### [`RFC4271-8.2.2-11`](#rfc4271-8.2.2-11)

If the local system receives a TcpConnectionFails event (Event 18) from the underlying TCP or a NOTIFICATION message (Event 25), the local system: - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Only Event 25 is driven. The quote's other trigger, TcpConnectionFails (Event 18), has no assertion under this tag. Only the counter is asserted, not the ConnectRetryTimer, resources or TCP-drop clauses. The negative calls one FSM ten times and proves the step size, not a non-triggering input.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterStepsByExactlyOnePerNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L283) | unit/verify | unproven |
| positive | [`TestRFC4271ConnectRetryCounterIncrementsOnNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L258) | unit/verify | unproven |

### [`RFC4271-8.2.2-12`](#rfc4271-8.2.2-12)

If the local system receives a NOTIFICATION message (Event 24 or Event 25) or a TcpConnectionFails (Event 18) from the underlying TCP, the local system: - sets the ConnectRetryTimer to zero, - deletes all routes associated with this connection, - releases all the BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Only the counter is asserted for Event 24. Event 25 and Event 18 are not driven under this tag, and 'deletes all routes associated with this connection', the ConnectRetryTimer, the resources and the TCP-drop clauses have no assertion.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterQuietOnVersionErrorInOpenStates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L332) | unit/verify | unproven |
| positive | [`TestRFC4271ConnectRetryCounterOnVersionErrorPerState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L308) | unit/verify | unproven |

### [`RFC4271-8.2.2-13`](#rfc4271-8.2.2-13)

If the local system receives a TcpConnectionFails event (Event 18) from the underlying TCP or a NOTIFICATION message (Event 25), the local system: - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Only the counter is asserted, not the ConnectRetryTimer, resources or TCP-drop clauses, and Event 25 is not driven under this tag. The negative also pins OpenSent + Event 18 to StateIdle, where RFC 4271 Section 8.2.2 OpenSent says 'changes its state to Active'. It asserts non-conformant behaviour, and its own comment says 'leaves for Active'.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterQuietOnTCPFailureInConnectAndOpenSent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L378) | unit/verify | unproven |
| positive | [`TestRFC4271ConnectRetryCounterOnTCPFailurePerState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L355) | unit/verify | unproven |

### [`RFC4271-8.2.2-14`](#rfc4271-8.2.2-14)

If the local system receives an UPDATE message, and the UPDATE message error handling procedure (see Section 6.3) detects an error (Event 28), the local system: - sends a NOTIFICATION message with an Update error, - sets the ConnectRetryTimer to zero, - deletes all routes associated with this connection, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Only the counter is asserted for Event 28 in Established. 'sends a NOTIFICATION message with an Update error', the ConnectRetryTimer zeroed, 'deletes all routes associated with this connection', resources released and the TCP connection dropped have no assertion in these units.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterQuietOnGoodUpdate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L421) | unit/verify | unproven |
| positive | [`TestRFC4271ConnectRetryCounterIncrementsOnUpdateError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L401) | unit/verify | unproven |

### [`RFC4271-8.2.2-15`](#rfc4271-8.2.2-15)

In response to any other events (Events 8, 10-11, 13, 19, 23, 25-28), the local system: - if the ConnectRetryTimer is running, stops and resets the ConnectRetryTimer (sets to zero), - if the DelayOpenTimer is running, stops and resets the DelayOpenTimer (sets to zero), - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The quote lists Events 8, 10-11, 13, 19, 23 and 25-28. The positive drives one event per state (KeepaliveMsg in Connect/Active), so the other listed events are unasserted. Only the counter and Idle are checked: the ConnectRetryTimer and DelayOpenTimer stop-and-reset, resources released and TCP dropped have no assertion.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterIdleDefaultArmCountsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L478) | unit/verify | unproven |
| positive | [`TestRFC4271ConnectRetryCounterIncrementsOnAnyOtherEvent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L443) | unit/verify | unproven |

### [`RFC4271-8.2.2-16`](#rfc4271-8.2.2-16)

If an AutomaticStop event (Event 8) is issued in the OpenSent state, the local system: - sends the NOTIFICATION with a Cease, - sets the ConnectRetryTimer to zero, - releases all the BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Only the counter is asserted for AutomaticStop. The quote's Cease NOTIFICATION in OpenSent, the ConnectRetryTimer zeroed, resources released and the TCP connection dropped have no assertion in these FSM units.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterAutomaticStopIsNotAManualStop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L575) | unit/verify | unproven |
| positive | [`TestRFC4271ConnectRetryCounterIncrementsOnAutomaticStop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L549) | unit/verify | unproven |

### [`RFC4271-8.2.2-17`](#rfc4271-8.2.2-17)

If a connection in the OpenSent state is determined to be the connection that must be closed, an OpenCollisionDump (Event 23) is signaled to the state machine. If such an event is received in the OpenSent state, the local system: - sends a NOTIFICATION with a Cease, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Only the counter is asserted for OpenCollisionDump. The quote's Cease NOTIFICATION, the ConnectRetryTimer zeroed, resources released and the TCP connection dropped have no assertion in these FSM units.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterCollisionDumpIsQuietInIdle`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L630) | unit/verify | unproven |
| positive | [`TestRFC4271ConnectRetryCounterIncrementsOnOpenCollisionDump`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L606) | unit/verify | unproven |

### [`RFC4271-8.2.2-18`](#rfc4271-8.2.2-18)

If a ManualStop event (Event 2) is issued in the OpenSent state, the local system: - sends the NOTIFICATION with a Cease, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, (§8.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The Cease clause is proven: exact bytes are read off the socket from OpenSent, OpenConfirm and Established after shutdownNotify, and no octet is written without a stop. The quote also requires the ConnectRetryTimer zeroed, BGP resources released and the TCP connection dropped. TestShutdownNotifySendsCeaseFromEveryConnectedState asserts none of these, and signal-stop-cease.ci says itself that it does not guard the drop ordering.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271NoCeaseWithoutAManualStop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/shutdown_notify_test.go#L207) | unit/verify | unproven |
| positive | [`TestShutdownNotifySendsCeaseFromEveryConnectedState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/shutdown_notify_test.go#L174) | unit/verify | unproven |
| positive | [`signal-stop-cease.ci`](https://github.com/ze-software/ze/blob/main/test/reload/signal-stop-cease.ci#L3) | functional/verify | unproven |

### [`RFC4271-10-1`](#rfc4271-10-1)

An implementation of BGP MUST allow the HoldTimer to be configurable on a per-peer basis (§10)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The units set PeerSettings.ReceiveHoldTime directly and show it reaches that peer's timers and negotiation. The operator configuration path (per-peer hold-time in config, parsePeerFromTree) is not exercised, so 'allow ... to be configurable' is proven only below the entry point.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271PerPeerHoldTimeSurvivesNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L285) | unit/verify | unproven |
| positive | [`TestRFC4271HoldTimeConfigurablePerPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L261) | unit/verify | unproven |

### [`RFC4271-5-7`](#rfc4271-5-7)

The sender of an UPDATE message SHOULD order path attributes within the UPDATE message in ascending order of attribute type. (§5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. no negative polarity: the batch and queued announce rails and the MP splitter are asserted only to emit ascending type-code sequences on valid input, and no test drives a non-ascending order and sees it refused or reordered, so the positive alone cannot prove the requirement.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSplitMP_PreservesAscendingAttributeOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_split_attr_order_test.go#L72) | unit/verify | unproven |
| positive | [`TestAnnounceBatchRail_AS4PathOrderedAgainstLargeCommunity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_batch_attr_order_test.go#L328) | unit/verify | unproven |
| positive | [`TestAnnounceBatchRail_AscendingTypeCodeOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_batch_attr_order_test.go#L268) | unit/verify | unproven |
| positive | [`TestAnnounceQueuedRail_AscendingTypeCodeOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_batch_attr_order_test.go#L289) | unit/verify | revert, verified |

### [`RFC4271-6.3-2`](#rfc4271-6.3-2)

If the NEXT_HOP attribute is semantically incorrect, the error SHOULD be logged, and the route SHOULD be ignored. (§6.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestRFC4271SelfNextHopRouteIsNotInstalled proves the route is ignored for criterion a) (NEXT_HOP is the receiving speaker). 'the error SHOULD be logged' is not asserted, and criterion b) (EBGP one-hop common subnet) is not covered.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4271SelfNextHopRouteIsNotInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L52) | unit/verify | unproven |

### [`RFC4271-3.1-2`](#rfc4271-3.1-2)

The next hop for each of these routes MUST be resolvable via the local BGP speaker's Routing Table. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-3.1-2, so no unit is bound to it.

### [`RFC4271-5.1.2-2`](#rfc4271-5.1.2-2)

a) When a given BGP speaker advertises the route to an internal peer, the advertising speaker SHALL NOT modify the AS_PATH attribute associated with the route. (§5.1.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Tagged units cover only locally originated announce rails (WriteAnnounceUpdate empty path, API explicit path kept byte-identical). The route this sentence mostly concerns, one learned from a peer and relayed or reflected to an internal peer on the forward rail, has no assertion that its AS_PATH is left unmodified. The eBGP prepend negative proves RFC4271-5.1.2-3, a neighbouring rule.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ASPathPrependedTowardExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L148) | unit/verify | unproven |
| positive | [`TestEstablishedAnnounce_ExplicitASPath_IBGPVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_batch_test.go#L569) | unit/verify | unproven |
| positive | [`TestRFC4271ASPathUnmodifiedTowardInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L131) | unit/verify | unproven |

### [`RFC4271-5.1.2-3`](#rfc4271-5.1.2-3)

When a given BGP speaker advertises the route to an external peer, the advertising speaker updates the AS_PATH attribute as follows: 1) if the first path segment of the AS_PATH is of type AS_SEQUENCE, the local system prepends its own AS number as the last element of the sequence (put it in the leftmost position with respect to the position of octets in the protocol message). If the act of prepending will cause an overflow in the AS_PATH segment (i.e., more than 255 ASes), it SHOULD prepend a new segment of type AS_SEQUENCE and prepend its own AS number to this new segment. 2) if the first path segment of the AS_PATH is of type AS_SET, the local system prepends a new path segment of type AS_SEQUENCE to the AS_PATH, including its own AS number in that segment. 3) if the AS_PATH is empty, the local system creates a path segment of type AS_SEQUENCE, places its own AS into that segment, and places that segment into the AS_PATH. (§5.1.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Tagged units prove case 1 (leading AS_SEQUENCE gets the local AS prepended) and the advertise-only condition. Case 2 (AS_SET-led path gets a new AS_SEQUENCE) is proven by no tagged unit; case 3 (empty path) is only in TestRFC4271ASPathPrependedTowardExternalPeer, which is tagged RFC4271-5.1.2-2.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEstablishedAnnounce_ExplicitASPath_IBGPVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_batch_test.go#L567) | unit/verify | unproven |
| negative | [`TestASPathSlotPrependOnlyWhenAdvertising`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/advertise_test.go#L99) | unit/verify | unproven |
| negative | [`checkRelayWithdrawalShape`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L430) | interop/nightly | unproven |
| positive | [`TestEstablishedAnnounce_ExplicitASPath_PrependsLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_batch_test.go#L543) | unit/verify | unproven |
| positive | [`TestASPathSlotPrependOnlyWhenAdvertising`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/advertise_test.go#L97) | unit/verify | unproven |
| positive | [`checkRelayWithdrawalShape`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L429) | interop/nightly | unproven |

### [`RFC4271-5.1.4-3`](#rfc4271-5.1.4-3)

If a BGP speaker is configured to alter the value of the MULTI_EXIT_DISC attribute received over EBGP, then altering the value MUST be done prior to determining the degree of preference of the route and prior to performing route selection (Decision Process phases 1 and 2). (§5.1.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestRFC4271MEDAlterationHappensAtIngress asserts only that safeIngressFilter returns the rewritten payload. That the rewritten MED is what phases 1 and 2 see (installed before dispatch to the RIB, reactor_notify.go) is stated in the comment, not asserted.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4271MEDAlterationHappensAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L452) | unit/verify | unproven |

### [`RFC4271-5.1.5-5`](#rfc4271-5.1.5-5)

A BGP speaker SHALL calculate the degree of preference for each external route based on the locally-configured policy, and include the degree of preference when advertising a route to its internal peers. (§5.1.5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. extractCandidate seeding 100 is a hard-coded default, and no unit shows a locally configured policy (import local-preference) setting the degree of preference; the clause 'include the degree of preference when advertising a route to its internal peers' is not in these units.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271DegreeOfPreferenceNotAHardcodedConstant`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L168) | unit/verify | unproven |
| positive | [`TestRFC4271ExternalRouteDegreeOfPreferenceFromLocalPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L141) | unit/verify | unproven |

### [`RFC4271-5.1.7-1`](#rfc4271-5.1.7-1)

A BGP speaker that performs route aggregation MAY add the AGGREGATOR attribute, which SHALL contain its own AS number and IP address. (§5.1.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-5.1.7-1, so no unit is bound to it.

### [`RFC4271-6.7-4`](#rfc4271-6.7-4)

If the BGP speaker decides to terminate its BGP connection with a neighbor because the number of address prefixes received from the neighbor exceeds the locally-configured, upper bound, then the speaker MUST send the neighbor a NOTIFICATION message with the Error Code Cease. (§6.7)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestPrefixExceedTeardown asserts checkPrefixLimits returns a Cease NOTIFICATION; the send to the neighbor (session_read.go:295) is not exercised by any tagged unit. Interop scenario bgp-max-prefix-cease-frr exists but carries no tag.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPrefixExceedDrop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_prefix_test.go#L144) | unit/verify | unproven |
| positive | [`TestPrefixExceedTeardown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_prefix_test.go#L107) | unit/verify | unproven |

### [`RFC4271-6.8-1`](#rfc4271-6.8-1)

In the event of connection collision, one of the connections MUST be closed (§6.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCollisionOpenSentNoCollision`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L176) | unit/verify | unproven |
| positive | [`TestRFC4271CollisionClosesExactlyOneConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_rfc7611_ingress_test.go#L44) | unit/verify | revert, verified |

### [`RFC4271-6.8-2`](#rfc4271-6.8-2)

Upon receipt of an OPEN message, the local system MUST examine all of its connections that are in the OpenConfirm state. (§6.8)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The units call session.detectCollision directly; the obligation's trigger, receipt of an OPEN, never reaches it in a tagged unit. The negative covers Idle only.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCollisionNonCollisionStates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L535) | unit/verify | unproven |
| positive | [`TestCollisionOpenConfirmLocalWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L129) | unit/verify | unproven |

### [`RFC4271-9-1`](#rfc4271-9-1)

If the UPDATE message contains a non-empty WITHDRAWN ROUTES field, the previously advertised routes, whose destinations (expressed as IP prefixes) are contained in this field, SHALL be removed from the Adj-RIB-In. (§9)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestRFC4271WithdrawRemovesFromAdjRIBIn calls FamilyRIB.Remove directly; no tagged unit feeds an UPDATE with a non-empty WITHDRAWN ROUTES field through the RIB plugin's receive path.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271WithdrawRemovesFromAdjRIBIn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L220) | unit/verify | unproven |
| positive | [`TestRFC4271WithdrawRemovesFromAdjRIBIn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L215) | unit/verify | unproven |

### [`RFC4271-9-2`](#rfc4271-9-2)

If the UPDATE message contains a feasible route, the Adj-RIB-In will be updated with this route as follows: if the NLRI of the new route is identical to the one the route currently has stored in the Adj- RIB-In, then the new route SHALL replace the older route in the Adj- RIB-In, thus implicitly withdrawing the older route from service. (§9)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Replacement is proven on FamilyRIB.Insert directly, not from a received UPDATE. The negative tag asserts that removal is keyed on NLRI, which is a neighbouring rule, not a violation of replacement.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271WithdrawRemovesFromAdjRIBIn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L217) | unit/verify | unproven |
| positive | [`TestRFC4271SamePrefixReplacesRatherThanAccumulates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L189) | unit/verify | unproven |

### [`RFC4271-9-3`](#rfc4271-9-3)

Once the BGP speaker updates the Adj-RIB-In, the speaker SHALL run its Decision Process. (§9)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Both units call checkBestPathChange themselves after Insert/Remove, so the obligation (the speaker runs the Decision Process once the Adj-RIB-In is updated, rib_structured.go) is not exercised. The negative asserts no change on unchanged input, not a missed run.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRIBBestChangeNoPublishSameBest`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L650) | unit/verify | unproven |
| positive | [`TestRIBBestChangeWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L686) | unit/verify | unproven |

### [`RFC4271-9.1.1-1`](#rfc4271-9.1.1-1)

The function that calculates the degree of preference for a given route SHALL NOT use any of the following as its inputs: the existence of other routes, the non-existence of other routes, or the path attributes of other routes. (§9.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. In TestRFC4271DegreeOfPreferenceIgnoresOtherRoutes, withNoise := ComparePair(a, b) never receives the noise candidates, so that assertion cannot fail; only the SelectBest winner check can. No test shows a preference function that did read other routes going red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271DegreeOfPreferenceFollowsOwnAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L68) | unit/verify | unproven |
| positive | [`TestRFC4271DegreeOfPreferenceIgnoresOtherRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L33) | unit/verify | unproven |

### [`RFC4271-9.1.1-2`](#rfc4271-9.1.1-2)

the return value MUST be used as the LOCAL_PREF value in any IBGP readvertisement. (§9.1.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. rfc4271Announce builds a locally originated route with no policy; the asserted LOCAL_PREF 100 is the default, so a hard-coded 100 passes. No external-learned route with a policy-computed preference is readvertised to iBGP. The negative (no LOCAL_PREF to eBGP) is RFC4271-5.1.5-2's rule.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271LocalPrefOmittedForExternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L110) | unit/verify | unproven |
| positive | [`TestRFC4271LocalPrefIncludedForInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L86) | unit/verify | unproven |

### [`RFC4271-9.1.2-1`](#rfc4271-9.1.2-1)

If the NEXT_HOP attribute of a BGP route depicts an address that is not resolvable, or if it would become unresolvable if the route was installed in the routing table, the BGP route MUST be excluded from the Phase 2 decision function. (§9.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.1.2-1, so no unit is bound to it.

### [`RFC4271-9.1.2-2`](#rfc4271-9.1.2-2)

The local speaker SHALL then install that route in the Loc-RIB, replacing any route to the same destination that is currently being held in the Loc-RIB. (§9.1.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestLocRIBMirror proves the best route is installed; 'replacing any route to the same destination that is currently being held in the Loc-RIB' is not asserted. The negative is removal on no candidate, a neighbouring rule.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRIBBestChangeWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L691) | unit/verify | unproven |
| positive | [`TestLocRIBMirror`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L1513) | unit/verify | unproven |

### [`RFC4271-9.1.2-3`](#rfc4271-9.1.2-3)

The local speaker MUST determine the immediate next-hop address from the NEXT_HOP attribute (§9.1.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271LocRIBNextHopComesFromNextHopAttribute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L94) | unit/verify | unproven |
| positive | [`TestLocRIBMirror`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L1515) | unit/verify | unproven |

### [`RFC4271-9.1.2-4`](#rfc4271-9.1.2-4)

If either the immediate next-hop or the IGP cost to the NEXT_HOP (where the NEXT_HOP is resolved through an IGP route) changes, Phase 2 Route Selection MUST be performed again. (§9.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.1.2-4, so no unit is bound to it.

### [`RFC4271-9.1.2.1-1`](#rfc4271-9.1.2.1-1)

Notice that even though BGP routes do not have to be installed in the Routing Table with the immediate next-hop(s), implementations MUST take care that, before any packets are forwarded along a BGP route, its associated NEXT_HOP address is resolved to the immediate (directly connected) next-hop address, and that this address (or multiple addresses) is finally used for actual packet forwarding. (§9.1.2)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. The tagged units (TestLocRIBMirror, TestRFC4271LocRIBNextHopComesFromNextHopAttribute) assert that the Loc-RIB NextHop is copied unresolved from the NEXT_HOP attribute. That is a neighbouring rule. No unit resolves the NEXT_HOP to an immediate, directly connected next hop before forwarding, which is what the sentence requires. The RFC4271-3.1-2 gap places resolution at FIB install.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271LocRIBNextHopComesFromNextHopAttribute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L98) | unit/verify | unproven |
| positive | [`TestLocRIBMirror`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L1518) | unit/verify | unproven |

### [`RFC4271-9.1.2.1-2`](#rfc4271-9.1.2.1-2)

Unresolvable routes SHALL be removed from the Loc-RIB and the routing table. (§9.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.1.2.1-2, so no unit is bound to it.

### [`RFC4271-9.1.2.2-1`](#rfc4271-9.1.2.2-1)

The criteria MUST be applied in the order specified. (§9.1.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The units prove step f) decides before g) and falls through on a tie, and that c) is skipped across neighbor ASes. A reordering among steps a) to e) (e.g. AS_PATH length before LOCAL_PREF) passes every tagged unit; TestBestPath_FullTiebreak only shows g) deciding after skips.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBestPath_MED_SameNeighborAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L315) | unit/verify | unproven |
| negative | [`TestBestPathEqualBGPIdentifiersOnTheJSONRailFallThroughToPeerAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_bgp_identifier_json_test.go#L106) | unit/verify | unproven |
| negative | [`TestBestPathEqualBGPIdentifiersFallThroughToPeerAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_bgp_identifier_test.go#L116) | unit/verify | unproven |
| positive | [`TestBestPath_FullTiebreak`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L530) | unit/verify | unproven |
| positive | [`TestBestPathStepFComparesThePeerBGPIdentifierOnTheJSONRail`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_bgp_identifier_json_test.go#L78) | unit/verify | unproven |
| positive | [`TestBestPathStepFComparesThePeerBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_bgp_identifier_test.go#L88) | unit/verify | unproven |

### [`RFC4271-9.1.2.2-2`](#rfc4271-9.1.2.2-2)

If an implementation chooses to remove MULTI_EXIT_DISC, then the optional comparison on MULTI_EXIT_DISC, if performed, MUST be performed only among EBGP-learned routes. (§9.1.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.1.2.2-2, so no unit is bound to it.

### [`RFC4271-9.1.2.2-3`](#rfc4271-9.1.2.2-3)

For IBGP- learned routes, the MULTI_EXIT_DISC MUST be used in route comparisons that reach this step in the Decision Process. (§9.1.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The unit proves lower MED wins between candidates with equal non-zero FirstAS. No candidate is iBGP-learned (LocalASN 0 means unknown in bestpath.go), so the 'IBGP-learned' qualifier has no assertion. The negative, a different neighbor AS, is step c)'s scope rule. Code defect: MED is skipped when FirstAS is 0 (bestpath.go step 4). Two iBGP routes originated in the local AS (empty AS_PATH) therefore never compare MED, although Section 9.1.2.2 makes neighborAS the local AS for them.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBestPath_MED_SameNeighborAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L322) | unit/verify | unproven |
| positive | [`TestBestPath_MED_SameNeighborAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L319) | unit/verify | unproven |

### [`RFC4271-9.1.2.2-4`](#rfc4271-9.1.2.2-4)

Routes that do not have the MULTI_EXIT_DISC attribute are considered to have the lowest possible MULTI_EXIT_DISC value (§9.1.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAbsentMedTiesAnExplicitMedOfZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L257) | unit/verify | revert, verified |
| positive | [`TestAbsentMedStillComparesAsZeroInPhaseTwo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L225) | unit/verify | revert, verified |

### [`RFC4271-9.2-2`](#rfc4271-9.2-2)

A route SHALL NOT be installed in the Adj-Rib-Out unless the destination, and NEXT_HOP described by this route, may be forwarded appropriately by the Routing Table. (§9.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.2-2, so no unit is bound to it.

### [`RFC4271-9.2-3`](#rfc4271-9.2-3)

If a route in Loc-RIB is excluded from a particular Adj-RIB-Out, the previously advertised route in that Adj-RIB-Out MUST be withdrawn from service by means of an UPDATE message (§9.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.2-3, so no unit is bound to it.

### [`RFC4271-9.2-4`](#rfc4271-9.2-4)

If a BGP speaker receives overlapping routes, the Decision Process MUST consider both routes based on the configured acceptance policy. (§9.1.4)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. Both units call FamilyRIB.Insert on the Adj-RIB-In store and assert that /8 and /24 are held as two entries, which is NLRI keying of storage. The sentence binds the Decision Process to consider both routes under the configured acceptance policy. No tagged unit runs best-path selection or an import policy over overlapping routes.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271SamePrefixReplacesRatherThanAccumulates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L182) | unit/verify | unproven |
| positive | [`TestRFC4271OverlappingRoutesBothInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L148) | unit/verify | unproven |

### [`RFC4271-9.2-5`](#rfc4271-9.2-5)

If both a less and a more specific route are accepted, then the Decision Process MUST install, in Loc-RIB, either both the less and the more specific routes or aggregate the two routes and install, in Loc-RIB, the aggregated route, provided that both routes have the same value of the NEXT_HOP attribute. (§9.1.4)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. The positive asserts that Adj-RIB-In storage (FamilyRIB) holds both routes, and the negative asserts identical-NLRI replacement (9-2). Both are neighbouring rules. Nothing asserts that the Loc-RIB (the Decision Process output) installs both the less and the more specific route, or their aggregate.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271SamePrefixReplacesRatherThanAccumulates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L186) | unit/verify | unproven |
| positive | [`TestRFC4271OverlappingRoutesBothInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L151) | unit/verify | unproven |

### [`RFC4271-9.2.1.1-2`](#rfc4271-9.2.1.1-2)

Two UPDATE messages sent by a BGP speaker to a peer that advertise feasible routes and/or withdrawal of unfeasible routes to some common set of destinations MUST be separated by at least MinRouteAdvertisementIntervalTimer. (§9.2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.2.1.1-2, so no unit is bound to it.

### [`RFC4271-9.2.2.2-1`](#rfc4271-9.2.2.2-1)

Routes that have different MULTI_EXIT_DISC attributes SHALL NOT be aggregated. (§9.2.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.2.2.2-1, so no unit is bound to it.

### [`RFC4271-9.2.2.2-2`](#rfc4271-9.2.2.2-2)

ORIGIN attribute: If at least one route among routes that are aggregated has ORIGIN with the value INCOMPLETE, then the aggregated route MUST have the ORIGIN attribute with the value INCOMPLETE. (§9.2.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.2.2.2-2, so no unit is bound to it.

### [`RFC4271-9.2.2.2-3`](#rfc4271-9.2.2.2-3)

NEXT_HOP: When aggregating routes that have different NEXT_HOP attributes, the NEXT_HOP attribute of the aggregated route SHALL identify an interface on the BGP speaker that performs the aggregation. (§9.2.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.2.2.2-3, so no unit is bound to it.

### [`RFC4271-9.2.2.2-4`](#rfc4271-9.2.2.2-4)

ATOMIC_AGGREGATE: If at least one of the routes to be aggregated has ATOMIC_AGGREGATE path attribute, then the aggregated route SHALL have this attribute as well. (§9.2.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.2.2.2-4, so no unit is bound to it.

### [`RFC4271-9.2.2.2-5`](#rfc4271-9.2.2.2-5)

AGGREGATOR: Any AGGREGATOR attributes from the routes to be aggregated MUST NOT be included in the aggregated route. (§9.2.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.2.2.2-5, so no unit is bound to it.

### [`RFC4271-Security-1`](#rfc4271-security-1)

An implementation MUST support the TCP MD5 option [RFC2385]. (§E)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestMD5PeersForListener proves md5PeersForListener collects the right keys per listen port; no tagged unit shows TCP_MD5SIG installed on the listener or on the active (dial) socket, which is what Ze produces for the layer below

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMD5PeersForListener`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_test.go#L2360) | unit/verify | unproven |
| positive | [`TestMD5PeersForListener`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_test.go#L2357) | unit/verify | unproven |

### [`RFC4271-9.2.1.1-3`](#rfc4271-9.2.1.1-3)

If new routes are selected multiple times while awaiting the expiration of MinRouteAdvertisementIntervalTimer, the last route selected SHALL be advertised at the end of MinRouteAdvertisementIntervalTimer. (§9.2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.2.1.1-3, so no unit is bound to it.

### [`RFC4271-9.2-6`](#rfc4271-9.2-6)

When a BGP speaker receives an UPDATE message from an internal peer, the receiving BGP speaker SHALL NOT re-distribute the routing information contained in that UPDATE message to other internal peers (unless the speaker acts as a BGP Route Reflector [RFC2796]). (§9.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Only reactorForwardRS is exercised. The same split-horizon check on the API forward rail (reactor_api_forward.go:721) has no tagged unit. The prohibition's assertion is assert.Empty after a fixed 50ms sleep, so a dispatch slower than that passes. The Route Reflector exception is shown with a client source only, not with a client destination.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271IBGPRedistributionAllowedForReflectorClient`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L567) | unit/verify | unproven |
| positive | [`TestRFC4271NoIBGPToIBGPRedistribution`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L508) | unit/verify | unproven |

### [`RFC4271-9.2-7`](#rfc4271-9.2-7)

All newly installed routes and all newly unfeasible routes for which there is no replacement route SHALL be advertised to its peers by means of an UPDATE message. (§9.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. positive calls checkBestPathChange directly after FamilyRIB.Remove and asserts a BestChangeWithdraw event; no UPDATE to a peer is asserted, and the 'newly installed routes' half of the sentence carries no tag. Negative is an unchanged-best re-run

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRIBBestChangeNoPublishSameBest`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L653) | unit/verify | unproven |
| positive | [`TestRIBBestChangeWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L689) | unit/verify | unproven |

### [`RFC4271-9.2-8`](#rfc4271-9.2-8)

Any routes in the Loc-RIB marked as unfeasible SHALL be removed (§9.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRIBBestChangeNoPublishSameBest`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L655) | unit/verify | unproven |
| positive | [`TestLocRIBMirror`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange_test.go#L1521) | unit/verify | unproven |

### [`RFC4271-9.2-9`](#rfc4271-9.2-9)

Changes to the reachable destinations within its own autonomous system SHALL also be advertised in an UPDATE message. (§9.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. units call the encoders writeAnnounceUpdate and writeWithdrawUpdate directly and check UPDATE layout; no unit shows a change to a locally originated destination causing an UPDATE to be sent to a peer

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271OwnASUnreachabilityChangeAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L426) | unit/verify | unproven |
| positive | [`TestRFC4271OwnASReachabilityChangeAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L398) | unit/verify | unproven |

### [`RFC4271-9.2-10`](#rfc4271-9.2-10)

If, due to the limits on the maximum size of an UPDATE message (see Section 4), a single route doesn't fit into the message, the BGP speaker MUST not advertise the route to its peers and MAY choose to log an error locally. (§9.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. units drive message.Splitter.Split directly: ErrAttributesTooLarge and zero emits for a route whose attributes exceed the ceiling. The forward rail also splits through wireu.SplitWireUpdate (forward_body.go) and no unit shows a caller withholding the route rather than sending it

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

A BGP speaker SHOULD treat an UPDATE message of this form as though the WITHDRAWN ROUTES do not contain the address prefix. (§4.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Substance is proven: all three paths assert the prefix installed and no withdrawal published for an UPDATE naming it in both WITHDRAWN and NLRI, which goes red on a withdraw-applied-last implementation. Every tag is positive and the row carries no {single-polarity} marker; the discriminating contrast (TestRIBWithdrawAndAnnounceDifferentPrefixesBothApply) is untagged. Tag it negative or add the marker to reach enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRIBInjectSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L121) | unit/verify | unproven |
| positive | [`TestRIBPoolPathSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L87) | unit/verify | unproven |
| positive | [`TestRIBSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L42) | unit/verify | unproven |

### [`RFC4271-5-8`](#rfc4271-5-8)

Once a BGP peer has updated any well-known attributes, it MUST pass these attributes to its peers in any updates it transmits (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestForwardNeverTransmitsTheSupersededWellKnownAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rfc4271_section5_test.go#L166) | unit/verify | revert, verified |
| positive | [`TestForwardTransmitsUpdatedWellKnownAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rfc4271_section5_test.go#L145) | unit/verify | revert, verified |

### [`RFC4271-6.2-5`](#rfc4271-6.2-5)

If the version number in the Version field of the received OPEN message is not supported, then the Error Subcode MUST be set to Unsupported Version Number (§6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_error_rfc4271_test.go#L66) | unit/verify | revert, verified |
| positive | [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_error_rfc4271_test.go#L64) | unit/verify | revert, verified |

### [`RFC4271-6.2-6`](#rfc4271-6.2-6)

If the Autonomous System field of the OPEN message is unacceptable, then the Error Subcode MUST be set to Bad Peer AS (§6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_error_rfc4271_test.go#L69) | unit/verify | revert, verified |
| positive | [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_error_rfc4271_test.go#L67) | unit/verify | revert, verified |

### [`RFC4271-6.2-7`](#rfc4271-6.2-7)

If the Hold Time field of the OPEN message is unacceptable, then the Error Subcode MUST be set to Unacceptable Hold Time (§6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_error_rfc4271_test.go#L73) | unit/verify | revert, verified |
| positive | [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_error_rfc4271_test.go#L71) | unit/verify | revert, verified |

### [`RFC4271-6.2-8`](#rfc4271-6.2-8)

If the BGP Identifier field of the OPEN message is syntactically incorrect, then the Error Subcode MUST be set to Bad BGP Identifier (§6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_error_rfc4271_test.go#L76) | unit/verify | revert, verified |
| positive | [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_error_rfc4271_test.go#L74) | unit/verify | revert, verified |

### [`RFC4271-6.2-9`](#rfc4271-6.2-9)

If one of the Optional Parameters in the OPEN message is not recognized, then the Error Subcode MUST be set to Unsupported Optional Parameters (§6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSessionRFC4271OptionalParameterErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L37) | unit/verify | unproven |
| positive | [`TestSessionRFC4271OptionalParameterErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L36) | unit/verify | unproven |

### [`RFC4271-6.2-10`](#rfc4271-6.2-10)

If one of the Optional Parameters in the OPEN message is recognized, but is malformed, then the Error Subcode MUST be set to 0 (Unspecific) (§6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSessionRFC4271OptionalParameterErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L39) | unit/verify | unproven |
| positive | [`TestSessionRFC4271OptionalParameterErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L38) | unit/verify | unproven |

### [`RFC4271-6.3-4`](#rfc4271-6.3-4)

If the Withdrawn Routes Length or Total Attribute Length is too large (i.e., if Withdrawn Routes Length + Total Attribute Length + 23 exceeds the message Length), then the Error Subcode MUST be set to Malformed Attribute List. (§6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive drives Total Attribute Length and Withdrawn Routes Length overruns through enforceRFC7606 and asserts SessionReset plus NOTIFICATION 3/1 read off the written bytes (RFC 7606 3(b) keeps this subcode); negative: consistent lengths draw no NOTIFICATION

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271UpdateMalformedAttributeList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_update_error_rfc4271_test.go#L57) | unit/verify | revert, verified |
| positive | [`TestRFC4271UpdateMalformedAttributeList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_update_error_rfc4271_test.go#L54) | unit/verify | revert, verified |

### [`RFC4271-6.3-5`](#rfc4271-6.3-5)

If any recognized attribute has Attribute Flags that conflict with the Attribute Type Code, then the Error Subcode MUST be set to Attribute Flags Error. The Data field MUST contain the erroneous attribute (type, length, and value). (§6.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Revised by RFC 7606 3(c) to treat-as-withdraw 'unless the specification for the attribute mandates different handling'. 'Any recognized attribute' is shown for ORIGIN with 0x80 only; no unit covers another well-known or a recognized optional attribute whose flags conflict (where 7606 mandates attribute-discard, e.g. AGGREGATOR), so the per-attribute flag table is proven for one entry.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L103) | unit/verify | unproven |
| positive | [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L102) | unit/verify | unproven |

### [`RFC4271-6.3-6`](#rfc4271-6.3-6)

If any recognized attribute has an Attribute Length that conflicts with the expected length (based on the attribute type code), then the Error Subcode MUST be set to Attribute Length Error. The Data field MUST contain the erroneous attribute (type, length, and value). (§6.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Revised by RFC 7606 to treat-as-withdraw. 'Any recognized attribute' with a conflicting length is shown for NEXT_HOP (3 octets) only; the per-attribute length checks for ORIGIN, LOCAL_PREF, ATOMIC_AGGREGATE, AGGREGATOR and others, each a separate expected length and some mandated attribute-discard by 7606, have no assertion in a tagged unit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L105) | unit/verify | unproven |
| positive | [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L104) | unit/verify | unproven |

### [`RFC4271-6.3-7`](#rfc4271-6.3-7)

If any of the well-known mandatory attributes are not present, then the Error Subcode MUST be set to Missing Well-known Attribute. The Data field MUST contain the Attribute Type Code of the missing, well-known attribute. (§6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7606 Section 3(d) (Updates: 4271) replaces the subcode and Data clauses with treat-as-withdraw, so the forbidden behaviour is a reset or a silent keep. TestSessionRFC4271RevisedAttributeErrors: missing ORIGIN, missing AS_PATH and missing NEXT_HOP each require.Equal the payload to a withdrawal of 203.0.113.0/24 (red on a keep or a drop), and the good UPDATE after it on the same session is received (red on a reset). Negative: all three present, NLRI announced. TestRFC4271MandatoryAttributesAcrossUpdateForms pins the MP_REACH/legacy NEXT_HOP split.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271MandatoryAttributesAcrossUpdateForms`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_mandatory_test.go#L11) | unit/verify | unproven |
| negative | [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L107) | unit/verify | unproven |
| positive | [`TestRFC4271MandatoryAttributesAcrossUpdateForms`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_mandatory_test.go#L10) | unit/verify | unproven |
| positive | [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L106) | unit/verify | unproven |

### [`RFC4271-6.3-8`](#rfc4271-6.3-8)

If any of the well-known mandatory attributes are not recognized, then the Error Subcode MUST be set to Unrecognized Well-known Attribute. The Data field MUST contain the unrecognized attribute (type, length, and value). (§6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Subcode clause: unknown well-known code 250 (flags 0x50, extended length) asserts the NOTIFICATION body equals 3/2 exactly, red on any other subcode. Data clause: the same Equal pins Data to the whole attribute, header and extended length included, red on a truncated or absent Data field. Negative: the same code with flags 0xd0 (optional transitive) is accepted and its NLRI announced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSessionRFC4271RetainedUpdateNotifications`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L157) | unit/verify | unproven |
| positive | [`TestSessionRFC4271RetainedUpdateNotifications`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L156) | unit/verify | unproven |

### [`RFC4271-6.3-9`](#rfc4271-6.3-9)

If the ORIGIN attribute has an undefined value, then the Error Sub- code MUST be set to Invalid Origin Attribute. The Data field MUST contain the unrecognized attribute (type, length, and value). (§6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7606 Section 7.1 (Updates: 4271) replaces the subcode and Data clauses with treat-as-withdraw for an undefined ORIGIN. ORIGIN 3, the first undefined value, require.Equals the payload to a withdrawal (red on a keep) and the following good UPDATE is received (red on a reset). Negative: ORIGIN IGP announces the prefix.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L109) | unit/verify | unproven |
| positive | [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L108) | unit/verify | unproven |

### [`RFC4271-6.3-10`](#rfc4271-6.3-10)

If the NEXT_HOP attribute field is syntactically incorrect, then the Error Subcode MUST be set to Invalid NEXT_HOP Attribute. The Data field MUST contain the incorrect attribute (type, length, and value). (§6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7606 (Updates: 4271) revises this: the error is handled by treat-as-withdraw, so no NOTIFICATION, subcode or Data field is sent. multicast NEXT_HOP 224.0.0.1 (not a valid host address) withdraws the prefix exactly (7.3); unicast NEXT_HOP announces it

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L111) | unit/verify | unproven |
| positive | [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L110) | unit/verify | unproven |

### [`RFC4271-6.3-11`](#rfc4271-6.3-11)

The IP address in the NEXT_HOP MUST meet the following criteria to be considered semantically correct: a) It MUST NOT be the IP address of the receiving speaker. b) In the case of an EBGP, where the sender and receiver are one IP hop away from each other, either the IP address in the NEXT_HOP MUST be the sender's IP address that is used to establish the BGP connection, or the interface associated with the NEXT_HOP IP address MUST share a common subnet with the receiving BGP speaker. (§6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. criterion a) receiver's own address (eBGP and iBGP) and b) off-link next hop on a one-hop eBGP session each withhold the legacy announcement on the live receive path; sender address and on-subnet third party accepted; multihop and iBGP accept off-link; interface snapshot change flips the verdict. The reaction (ignore, no NOTIFICATION) is RFC4271-6.3-2/6.3-3

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSessionRFC4271NextHopSemantics`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L206) | unit/verify | unproven |
| negative | [`TestSessionRFC4271IBGPNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_next_hop_test.go#L63) | unit/verify | unproven |
| negative | [`TestSessionRFC4271NextHopMixedUpdateAndAddressChange`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_next_hop_test.go#L16) | unit/verify | unproven |
| positive | [`TestSessionRFC4271NextHopSemantics`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L205) | unit/verify | unproven |
| positive | [`TestSessionRFC4271IBGPNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_next_hop_test.go#L62) | unit/verify | unproven |
| positive | [`TestSessionRFC4271NextHopMixedUpdateAndAddressChange`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_next_hop_test.go#L15) | unit/verify | unproven |

### [`RFC4271-6.3-12`](#rfc4271-6.3-12)

If the path is syntactically incorrect, then the Error Subcode MUST be set to Malformed AS_PATH. (§6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7606 (Updates: 4271) revises this: the error is handled by treat-as-withdraw, so no NOTIFICATION, subcode or Data field is sent. unknown segment type, zero segment count, segment overrun and trailing octet each withdraw the prefix exactly (7.2); a well-framed AS_SEQUENCE announces

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L113) | unit/verify | unproven |
| positive | [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L112) | unit/verify | unproven |

### [`RFC4271-6.3-13`](#rfc4271-6.3-13)

If the UPDATE message is received from an external peer, the local system MAY check whether the leftmost (with respect to the position of octets in the protocol message) AS in the AS_PATH attribute is equal to the autonomous system number of the peer that sent the message. If the check determines this is not the case, the Error Subcode MUST be set to Malformed AS_PATH. (§6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7606 (Updates: 4271) revises this: the error is handled by treat-as-withdraw, so no NOTIFICATION, subcode or Data field is sent. an external UPDATE whose leftmost AS 65003 is not the peer AS 65002 is delivered as exactly a withdrawal of its prefix (7.2); leftmost AS equal to the peer AS announces unchanged

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271LeftmostASMismatchIsMalformedASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_rfc7611_ingress_test.go#L32) | unit/verify | revert, verified |
| positive | [`TestRFC4271LeftmostASMismatchIsMalformedASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_rfc7611_ingress_test.go#L29) | unit/verify | revert, verified |

### [`RFC4271-6.3-14`](#rfc4271-6.3-14)

If an optional attribute is recognized, then the value of this attribute MUST be checked. If an error is detected, the attribute MUST be discarded, and the Error Subcode MUST be set to Optional Attribute Error. The Data field MUST contain the attribute (type, length, and value). (§6.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Revised by RFC 7606. 'If an optional attribute is recognized, then the value MUST be checked' is shown for MED (3 octets -> exact withdrawal, 4 octets kept) only; no unit covers the value check of another recognized optional attribute (COMMUNITIES, ORIGINATOR_ID, CLUSTER_LIST, AGGREGATOR) or the attribute-discard outcome 7606 mandates for some of them.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L115) | unit/verify | unproven |
| positive | [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L114) | unit/verify | unproven |

### [`RFC4271-6.3-15`](#rfc4271-6.3-15)

If any attribute appears more than once in the UPDATE message, then the Error Subcode MUST be set to Malformed Attribute List (§6.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271UpdateMalformedAttributeList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_update_error_rfc4271_test.go#L61) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC4271UpdateMalformedAttributeList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_update_error_rfc4271_test.go#L59) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`RFC4271-6.3-16`](#rfc4271-6.3-16)

If the field is syntactically incorrect, then the Error Subcode MUST be set to Invalid Network Field. (§6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. impossible prefix length and truncated NLRI (and truncated withdrawn) reset with NOTIFICATION 3/10 read off the socket after a good UPDATE was accepted on the same session; a valid prefix is announced

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSessionRFC4271RetainedUpdateNotifications`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L159) | unit/verify | unproven |
| positive | [`TestSessionRFC4271RetainedUpdateNotifications`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L158) | unit/verify | unproven |

### [`RFC4271-6.3-17`](#rfc4271-6.3-17)

An UPDATE message that contains correct path attributes, but no NLRI, SHALL be treated as a valid UPDATE message (§6.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271UpdateWithoutNLRIIsValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_update_error_rfc4271_test.go#L128) | unit/verify | revert, verified |
| positive | [`TestRFC4271UpdateWithoutNLRIIsValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_update_error_rfc4271_test.go#L125) | unit/verify | revert, verified |

### [`RFC4271-8.2.2-19`](#rfc4271-8.2.2-19)

In response to an indication that the TCP connection is successfully established (Event 16 or Event 17), the second connection SHALL be tracked until it sends an OPEN message (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSessionRFC4271EstablishedCollisionWaitsForOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L247) | unit/verify | unproven |
| positive | [`TestSessionRFC4271EstablishedCollisionWaitsForOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_core4271_test.go#L246) | unit/verify | unproven |

### [`RFC4271-9-4`](#rfc4271-9-4)

Otherwise, if the Adj-RIB-In has no route with NLRI identical to the new route, the new route SHALL be placed in the Adj-RIB-In. (§9)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a new NLRI not placed. handleReceivedStructured (the reactor UPDATE entry) asserts Len 1 then 2 and Get(10.0.0.0/8) with its own NEXT_HOP, red on a route not stored. Negative (the condition false, identical NLRI): Len stays 2 and the slot holds the newer NEXT_HOP, red on a route placed beside the existing one.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271AdjRIBInPlacesNewRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rib_rfc4271_test.go#L44) | unit/verify | revert, verified |
| positive | [`TestRFC4271AdjRIBInPlacesNewRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rib_rfc4271_test.go#L41) | unit/verify | revert, verified |

### [`RFC4271-10-4`](#rfc4271-10-4)

The suggested default amount of jitter SHALL be determined by multiplying the base value of the appropriate timer by a random factor, which is uniformly distributed in the range from 0.75 to 1.0 (§10)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTimersRFC4271Jitter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/timer_jitter_test.go#L12) | unit/verify | unproven |
| positive | [`TestTimersRFC4271Jitter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/timer_jitter_test.go#L11) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc4271.txt |
| Source fingerprint | d5034568e80da453 |
| Record | rfc/extraction/rfc4271.json |
| Mapped sentences | 105 |
| Declined as scope | 24 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 1 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 3 | walked | not stated |
| `4.2` | not stated | 2 | walked | not stated |
| `4.3` | not stated | 4 | walked | not stated |
| `4.4` | not stated | 2 | walked | not stated |
| `4.5` | not stated | 0 | walked | not stated |
| `5` | not stated | 8 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
| `5.1.1` | not stated | 0 | walked | not stated |
| `5.1.2` | not stated | 1 | walked | not stated |
| `5.1.3` | not stated | 3 | walked | not stated |
| `5.1.4` | not stated | 4 | walked | not stated |
| `5.1.5` | not stated | 5 | walked | not stated |
| `5.1.6` | not stated | 1 | walked | not stated |
| `5.1.7` | not stated | 1 | walked | not stated |
| `6` | not stated | 1 | walked | not stated |
| `6.1` | not stated | 6 | walked | not stated |
| `6.2` | not stated | 9 | walked | not stated |
| `6.3` | not stated | 25 | walked | not stated |
| `6.4` | not stated | 0 | walked | not stated |
| `6.5` | not stated | 0 | walked | not stated |
| `6.6` | not stated | 0 | walked | not stated |
| `6.7` | not stated | 2 | walked | not stated |
| `6.8` | not stated | 2 | walked | not stated |
| `7` | not stated | 1 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |
| `8.1.1` | not stated | 0 | walked | not stated |
| `8.1.2` | not stated | 0 | walked | not stated |
| `8.1.3` | not stated | 0 | walked | not stated |
| `8.1.4` | not stated | 0 | walked | not stated |
| `8.1.5` | not stated | 0 | walked | not stated |
| `8.2` | not stated | 0 | walked | not stated |
| `8.2.1` | not stated | 3 | walked | not stated |
| `8.2.1.1` | not stated | 0 | walked | not stated |
| `8.2.1.2` | not stated | 0 | walked | not stated |
| `8.2.1.3` | not stated | 0 | walked | not stated |
| `8.2.1.4` | not stated | 0 | walked | not stated |
| `8.2.1.5` | not stated | 0 | walked | not stated |
| `8.2.2` | not stated | 1 | walked | not stated |
| `9` | not stated | 5 | walked | not stated |
| `9.1` | not stated | 1 | walked | not stated |
| `9.1.1` | not stated | 1 | walked | not stated |
| `9.1.2` | not stated | 6 | walked | not stated |
| `9.1.2.1` | not stated | 0 | walked | not stated |
| `9.1.2.2` | not stated | 3 | walked | not stated |
| `9.1.3` | not stated | 2 | walked | not stated |
| `9.1.4` | not stated | 3 | walked | not stated |
| `9.2` | not stated | 5 | walked | not stated |
| `9.2.1` | not stated | 0 | walked | not stated |
| `9.2.1.1` | not stated | 2 | walked | not stated |
| `9.2.1.2` | not stated | 0 | walked | not stated |
| `9.2.2` | not stated | 0 | walked | not stated |
| `9.2.2.1` | not stated | 0 | walked | not stated |
| `9.2.2.2` | not stated | 11 | walked | not stated |
| `9.3` | not stated | 0 | walked | not stated |
| `9.4` | not stated | 0 | walked | not stated |
| `10` | not stated | 2 | walked | not stated |
| `A` | not stated | 1 | walked | not stated |
| `B` | not stated | 0 | walked | not stated |
| `C` | not stated | 0 | walked | not stated |
| `D` | not stated | 0 | walked | not stated |
| `E` | not stated | 1 | walked | not stated |
| `F` | not stated | 0 | walked | not stated |
| `F.1` | not stated | 0 | walked | not stated |
| `F.2` | not stated | 0 | walked | not stated |
| `F.3` | not stated | 0 | walked | not stated |
| `F.4` | not stated | 0 | walked | not stated |
| `F.5` | not stated | 0 | walked | not stated |
| `F.6` | not stated | 1 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `5:8` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the sentence defines the mandatory category by restating the obligation site 5:2 already carries, that a mandatory attribute be present when the UPDATE contains NLRI | The mandatory category refers to an attribute that MUST be present in both IBGP and EBGP exchanges if NLRI are contained in the UPDATE message. |
| `6.1:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Data field clause belongs to the Bad Message Length obligation, and the row already states both halves | The Data field MUST contain the erroneous Length field. |
| `6.1:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Data field clause belongs to the Bad Message Type obligation, and the row already states both halves | The Data field MUST contain the erroneous Type field. |
| `6.3:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Data field clause belongs to the Attribute Flags Error obligation, and the row states both halves | The Data field MUST contain the erroneous attribute (type, length, and value). |
| `6.3:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Data field clause belongs to the Attribute Length Error obligation, and the row states both halves | The Data field MUST contain the erroneous attribute (type, length, and value). |
| `6.3:8` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Data field clause belongs to the Missing Well-known Attribute obligation, and the row states both halves | The Data field MUST contain the Attribute Type Code of the missing, well-known attribute. |
| `6.3:10` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Data field clause belongs to the Unrecognized Well-known Attribute obligation, and the row states both halves | The Data field MUST contain the unrecognized attribute (type, length, and value). |
| `6.3:12` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Data field clause belongs to the Invalid Origin Attribute obligation, and the row states both halves | The Data field MUST contain the unrecognized attribute (type, length, and value). |
| `6.3:14` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Data field clause belongs to the Invalid NEXT_HOP Attribute obligation, and the row states both halves | The Data field MUST contain the incorrect attribute (type, length, and value). |
| `6.3:16` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | criterion a) is one of the two criteria the row states | a) It MUST NOT be the IP address of the receiving speaker. |
| `6.3:17` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | criterion b) is the other of the two criteria the row states | b) In the case of an EBGP, where the sender and receiver are one IP hop away from each other, either the IP address in the NEXT_HOP MUST be the sender's IP address that is used to establish the BGP connection, or the interface associated with the NEXT_HOP IP address MUST share a common subnet with the receiving BGP speaker. |
| `6.3:21` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the discard and the Optional Attribute Error subcode are the consequence half of the same obligation, and the row states them | If an error is detected, the attribute MUST be discarded, and the Error Subcode MUST be set to Optional Attribute Error. |
| `6.3:22` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Data field clause belongs to the Optional Attribute Error obligation, and the row states it | The Data field MUST contain the attribute (type, length, and value). |
| `7:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | the sentence binds the DESIGN of future BGP versions, not the behavior of a BGP-4 implementation: it tells whoever specifies BGP-5 to keep the OPEN and NOTIFICATION formats. No code ze runs can conform to it or violate it, because the obligation is discharged by a future document rather than by a speaker on the wire | In order to support BGP version negotiation, future versions of BGP MUST retain the format of the OPEN and NOTIFICATION messages. |
| `9:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | running the Decision Process is the second half of the withdrawal obligation, and the row states both halves | This BGP speaker SHALL run its Decision Process because the previously advertised route is no longer available for use. |
| `9.1.4:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the next sentence defines the act: 'That is, the NLRI of this route cannot be more specific.' That is the Section 5.1.6 prohibition on making the NLRI of an ATOMIC_AGGREGATE route more specific, restated where overlapping routes are discussed | In particular, a route that carries the ATOMIC_AGGREGATE attribute MUST NOT be de-aggregated. |
| `9.2.2.2:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the EGP arm is the second clause of the aggregated ORIGIN rule, and the row states both arms | Otherwise, if at least one route among routes that are aggregated has ORIGIN with the value EGP, then the aggregated route MUST have the ORIGIN attribute with the value EGP. |
| `9.2.2.2:5` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation is conditional on aggregating, and ze performs no route aggregation (the scope decision recorded in the Support remaining row of rfc/short/rfc4271.md). The RFC makes the choice the speaker's: "If a BGP speaker chooses to aggregate, then it SHOULD either include" | If the routes to be aggregated have different AS_PATH attributes, then the aggregated AS_PATH attribute SHALL satisfy all of the following conditions: |
| `9.2.2.2:6` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | an AS_SEQUENCE condition on an aggregated AS_PATH ze never builds, because ze performs no route aggregation. The RFC makes the choice the speaker's: "If a BGP speaker chooses to aggregate, then it SHOULD either include" | - all tuples of type AS_SEQUENCE in the aggregated AS_PATH SHALL appear in all of the AS_PATHs in the initial set of routes to be aggregated. |
| `9.2.2.2:7` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | an AS_SET condition on an aggregated AS_PATH ze never builds, because ze performs no route aggregation. The RFC makes the choice the speaker's: "If a BGP speaker chooses to aggregate, then it SHOULD either include" | - all tuples of type AS_SET in the aggregated AS_PATH SHALL appear in at least one of the AS_PATHs in the initial set (they may appear as either AS_SET or AS_SEQUENCE types). |
| `9.2.2.2:8` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | a duplicate-AS_SET condition on an aggregated AS_PATH ze never builds, because ze performs no route aggregation. The RFC makes the choice the speaker's: "If a BGP speaker chooses to aggregate, then it SHOULD either include" | - No tuple of type AS_SET with the same value SHALL appear more than once in the aggregated AS_PATH. |
| `9.2.2.2:9` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the conformance floor is an aggregation algorithm, and ze performs no route aggregation. The RFC makes the choice the speaker's: "If a BGP speaker chooses to aggregate, then it SHOULD either include" | At a minimum, a conformant implementation SHALL be able to perform the following algorithm that meets all of the above conditions: |
| `E:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Appendix E restates the Security Considerations obligation to support the TCP MD5 option of RFC 2385 | An implementation MUST support the TCP MD5 option [RFC2385]. |
| `F.6:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Appendix F.6 restates the same RFC 2385 authentication obligation as a change from RFC 1771 | A BGP implementation MUST support the authentication mechanism specified in RFC 2385 [RFC2385]. |

## Superseded

No document obsoletes RFC 4271, so its obligations are stated where they were written.
