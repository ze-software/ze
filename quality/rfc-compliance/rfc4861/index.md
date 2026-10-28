# RFC 4861 - Neighbor Discovery for IP version 6 (IPv6)

Partial. Every requirement this repository extracted from RFC 4861, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 7.2% | 10 of 138 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 14.5% | 20 of 138 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 138 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 138 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 100.0% | 49 of 49 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 138 | of 217 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 138 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 138 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 68.8% | 95 of 138 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 138 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |
| Linux cannot do this | 0.0% | 0 of 138 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and remains unmet; it stays in every denominator and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 9.4% | 13 of 138 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 shares marked as a part above are the whole of the 138 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 217 |
| Gated MUST-level | 138 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 13 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 49 |
| Tagged units | 49 |
| Recorded audit verdicts | 0 |
| Discrimination records | 49 |
| Summary | `rfc/short/rfc4861.md` |
| Requirement shard | `rfc/requirements/rfc4861.md` |
| RFC text | `rfc/full/rfc4861.txt` |

## Enrolment

Enrolled: Ze is an IPv6 router that originates Router Advertisements and answers Router Solicitations on configured interfaces (internal/plugins/iface/ra) and on L2TP subscriber sessions (internal/component/l2tp/ppp ra_send.go), and sends VRRP unsolicited Neighbor Advertisements (internal/plugins/vrrp/transport/na.go). Walked site by site on 2026-10-26 (rfc/extraction/rfc4861.json). Every MUST is classified: the rows the Linux kernel's Neighbor Discovery performs (Neighbor Solicitation and Advertisement, address resolution, NUD, Redirect, host processing of Router Advertisements) are lower-layer; the rows on Router Advertisement origination, Router Solicitation handling and unsolicited Neighbor Advertisements are Ze's and carry tagged tests where Ze meets them; thirteen of them are gaps, listed under Support remaining; the ND proxy and IPsec rows are among them, because no owner ruling names either feature unwanted (ai/META-DECISIONS.md, Goal and scope).

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

Router Advertisements with M and O flags, router lifetime, reachable and retrans timers, Source Link-Layer Address, Prefix Information and RDNSS options, sent with Hop Limit 255 from the link-local address on a randomized schedule, answered to a solicitation after a random delay, rate limited, and withdrawn with a zero router lifetime when an interface stops advertising. Router configuration variables are bounded as Section 6.2.1 requires. Neighbor Discovery other than Router Advertisement origination is performed by the Linux kernel.

**What the ledger says remains**

A received Router Solicitation is not checked for Hop Limit 255, Code 0, length or zero-length options ([`RFC4861-6.1.1-2`](#rfc4861-6.1.1-2), [`RFC4861-4.6-1`](#rfc4861-4.6-1)). The configuration does not cover every conceptual variable ([`RFC4861-6.2.1-1`](#rfc4861-6.2.1-1)), and the advertised reachable-time, retransmit-timer and hop-limit are not the values the router itself uses ([`RFC4861-6.2.1-8`](#rfc4861-6.2.1-8)). An interface that is not forwarding still advertises, and turning forwarding off does not zero the Router Lifetime ([`RFC4861-6.2.4-1`](#rfc4861-6.2.4-1), [`RFC4861-6.2.5-2`](#rfc4861-6.2.5-2)). VRRP refuses no multicast IPv6 virtual address as a Neighbor Advertisement target, and does not space its repeated unsolicited Neighbor Advertisements by RetransTimer ([`RFC4861-4.4-3`](#rfc4861-4.4-3), [`RFC4861-7.2.6-1`](#rfc4861-7.2.6-1)). A Router Advertisement can exceed the link MTU, because the prefix list is unbounded ([`RFC4861-9-7`](#rfc4861-9-7)). No MTU option is sent. No Neighbor Discovery proxy exists ([`RFC4861-7.2.8-1`](#rfc4861-7.2.8-1), [`RFC4861-7.2.8-2`](#rfc4861-7.2.8-2)), and Neighbor Discovery cannot be secured with AH or ESP ([`RFC4861-11.2-1`](#rfc4861-11.2-1), [`RFC4861-11.2-2`](#rfc4861-11.2-2)).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 10 | one part of the gated population |
| Annotated (including scoped evidence) | 128 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **138** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (10):** [`RFC4861-4.2-2`](#rfc4861-4.2-2), [`RFC4861-4.6.2-3`](#rfc4861-4.6.2-3), [`RFC4861-4.6.2-4`](#rfc4861-4.6.2-4), [`RFC4861-4.6.2-6`](#rfc4861-4.6.2-6), [`RFC4861-6.2.1-3`](#rfc4861-6.2.1-3), [`RFC4861-6.2.1-4`](#rfc4861-6.2.1-4), [`RFC4861-6.2.1-5`](#rfc4861-6.2.1-5), [`RFC4861-6.2.1-6`](#rfc4861-6.2.1-6), [`RFC4861-6.2.1-7`](#rfc4861-6.2.1-7), [`RFC4861-6.2.2-1`](#rfc4861-6.2.2-1)

**Annotated (including scoped evidence) (128):** [`RFC4861-2.3-1`](#rfc4861-2.3-1), [`RFC4861-4.1-1`](#rfc4861-4.1-1), [`RFC4861-4.1-2`](#rfc4861-4.1-2), [`RFC4861-4.1-3`](#rfc4861-4.1-3), [`RFC4861-4.2-1`](#rfc4861-4.2-1), [`RFC4861-4.2-3`](#rfc4861-4.2-3), [`RFC4861-4.3-1`](#rfc4861-4.3-1), [`RFC4861-4.3-2`](#rfc4861-4.3-2), [`RFC4861-4.3-3`](#rfc4861-4.3-3), [`RFC4861-4.3-4`](#rfc4861-4.3-4), [`RFC4861-4.3-5`](#rfc4861-4.3-5), [`RFC4861-4.4-1`](#rfc4861-4.4-1), [`RFC4861-4.4-2`](#rfc4861-4.4-2), [`RFC4861-4.4-3`](#rfc4861-4.4-3), [`RFC4861-4.4-4`](#rfc4861-4.4-4), [`RFC4861-4.4-5`](#rfc4861-4.4-5), [`RFC4861-4.4-6`](#rfc4861-4.4-6), [`RFC4861-4.5-1`](#rfc4861-4.5-1), [`RFC4861-4.5-2`](#rfc4861-4.5-2), [`RFC4861-4.5-3`](#rfc4861-4.5-3), [`RFC4861-4.5-4`](#rfc4861-4.5-4), [`RFC4861-4.5-5`](#rfc4861-4.5-5), [`RFC4861-4.6-1`](#rfc4861-4.6-1), [`RFC4861-4.6.1-1`](#rfc4861-4.6.1-1), [`RFC4861-4.6.2-1`](#rfc4861-4.6.2-1), [`RFC4861-4.6.2-2`](#rfc4861-4.6.2-2), [`RFC4861-4.6.2-5`](#rfc4861-4.6.2-5), [`RFC4861-4.6.2-7`](#rfc4861-4.6.2-7), [`RFC4861-4.6.3-1`](#rfc4861-4.6.3-1), [`RFC4861-4.6.3-2`](#rfc4861-4.6.3-2), [`RFC4861-4.6.4-1`](#rfc4861-4.6.4-1), [`RFC4861-4.6.4-2`](#rfc4861-4.6.4-2), [`RFC4861-6.1.1-1`](#rfc4861-6.1.1-1), [`RFC4861-6.1.1-2`](#rfc4861-6.1.1-2), [`RFC4861-6.1.1-3`](#rfc4861-6.1.1-3), [`RFC4861-6.1.1-4`](#rfc4861-6.1.1-4), [`RFC4861-6.1.2-1`](#rfc4861-6.1.2-1), [`RFC4861-6.1.2-2`](#rfc4861-6.1.2-2), [`RFC4861-6.1.2-3`](#rfc4861-6.1.2-3), [`RFC4861-6.2.1-1`](#rfc4861-6.2.1-1), [`RFC4861-6.2.1-2`](#rfc4861-6.2.1-2), [`RFC4861-6.2.1-8`](#rfc4861-6.2.1-8), [`RFC4861-6.2.2-2`](#rfc4861-6.2.2-2), [`RFC4861-6.2.4-1`](#rfc4861-6.2.4-1), [`RFC4861-6.2.5-1`](#rfc4861-6.2.5-1), [`RFC4861-6.2.5-2`](#rfc4861-6.2.5-2), [`RFC4861-6.2.6-1`](#rfc4861-6.2.6-1), [`RFC4861-6.2.6-2`](#rfc4861-6.2.6-2), [`RFC4861-6.2.6-3`](#rfc4861-6.2.6-3), [`RFC4861-6.2.6-4`](#rfc4861-6.2.6-4), [`RFC4861-6.2.6-5`](#rfc4861-6.2.6-5), [`RFC4861-6.2.6-6`](#rfc4861-6.2.6-6), [`RFC4861-6.2.6-7`](#rfc4861-6.2.6-7), [`RFC4861-6.3.4-1`](#rfc4861-6.3.4-1), [`RFC4861-6.3.4-2`](#rfc4861-6.3.4-2), [`RFC4861-6.3.4-3`](#rfc4861-6.3.4-3), [`RFC4861-6.3.4-4`](#rfc4861-6.3.4-4), [`RFC4861-6.3.4-5`](#rfc4861-6.3.4-5), [`RFC4861-6.3.4-6`](#rfc4861-6.3.4-6), [`RFC4861-6.3.4-7`](#rfc4861-6.3.4-7), [`RFC4861-6.3.4-8`](#rfc4861-6.3.4-8), [`RFC4861-6.3.5-1`](#rfc4861-6.3.5-1), [`RFC4861-6.3.7-1`](#rfc4861-6.3.7-1), [`RFC4861-7.1.1-1`](#rfc4861-7.1.1-1), [`RFC4861-7.1.1-2`](#rfc4861-7.1.1-2), [`RFC4861-7.1.1-3`](#rfc4861-7.1.1-3), [`RFC4861-7.1.2-1`](#rfc4861-7.1.2-1), [`RFC4861-7.1.2-2`](#rfc4861-7.1.2-2), [`RFC4861-7.1.2-3`](#rfc4861-7.1.2-3), [`RFC4861-7.2-1`](#rfc4861-7.2-1), [`RFC4861-7.2.1-1`](#rfc4861-7.2.1-1), [`RFC4861-7.2.1-2`](#rfc4861-7.2.1-2), [`RFC4861-7.2.1-3`](#rfc4861-7.2.1-3), [`RFC4861-7.2.2-1`](#rfc4861-7.2.2-1), [`RFC4861-7.2.2-2`](#rfc4861-7.2.2-2), [`RFC4861-7.2.2-3`](#rfc4861-7.2.2-3), [`RFC4861-7.2.2-4`](#rfc4861-7.2.2-4), [`RFC4861-7.2.2-5`](#rfc4861-7.2.2-5), [`RFC4861-7.2.3-1`](#rfc4861-7.2.3-1), [`RFC4861-7.2.3-2`](#rfc4861-7.2.3-2), [`RFC4861-7.2.3-3`](#rfc4861-7.2.3-3), [`RFC4861-7.2.3-4`](#rfc4861-7.2.3-4), [`RFC4861-7.2.4-1`](#rfc4861-7.2.4-1), [`RFC4861-7.2.4-2`](#rfc4861-7.2.4-2), [`RFC4861-7.2.4-3`](#rfc4861-7.2.4-3), [`RFC4861-7.2.4-4`](#rfc4861-7.2.4-4), [`RFC4861-7.2.5-1`](#rfc4861-7.2.5-1), [`RFC4861-7.2.5-2`](#rfc4861-7.2.5-2), [`RFC4861-7.2.5-3`](#rfc4861-7.2.5-3), [`RFC4861-7.2.5-4`](#rfc4861-7.2.5-4), [`RFC4861-7.2.5-5`](#rfc4861-7.2.5-5), [`RFC4861-7.2.5-6`](#rfc4861-7.2.5-6), [`RFC4861-7.2.5-7`](#rfc4861-7.2.5-7), [`RFC4861-7.2.6-1`](#rfc4861-7.2.6-1), [`RFC4861-7.2.6-2`](#rfc4861-7.2.6-2), [`RFC4861-7.2.6-3`](#rfc4861-7.2.6-3), [`RFC4861-7.2.8-1`](#rfc4861-7.2.8-1), [`RFC4861-7.2.8-2`](#rfc4861-7.2.8-2), [`RFC4861-7.3.1-1`](#rfc4861-7.3.1-1), [`RFC4861-7.3.3-1`](#rfc4861-7.3.3-1), [`RFC4861-7.3.3-2`](#rfc4861-7.3.3-2), [`RFC4861-7.3.3-3`](#rfc4861-7.3.3-3), [`RFC4861-7.3.3-4`](#rfc4861-7.3.3-4), [`RFC4861-8-1`](#rfc4861-8-1), [`RFC4861-8.1-1`](#rfc4861-8.1-1), [`RFC4861-8.1-2`](#rfc4861-8.1-2), [`RFC4861-8.1-3`](#rfc4861-8.1-3), [`RFC4861-8.1-4`](#rfc4861-8.1-4), [`RFC4861-8.2-1`](#rfc4861-8.2-1), [`RFC4861-8.2-2`](#rfc4861-8.2-2), [`RFC4861-8.2-3`](#rfc4861-8.2-3), [`RFC4861-8.2-4`](#rfc4861-8.2-4), [`RFC4861-8.3-1`](#rfc4861-8.3-1), [`RFC4861-8.3-2`](#rfc4861-8.3-2), [`RFC4861-8.3-3`](#rfc4861-8.3-3), [`RFC4861-8.3-4`](#rfc4861-8.3-4), [`RFC4861-8.3-5`](#rfc4861-8.3-5), [`RFC4861-9-1`](#rfc4861-9-1), [`RFC4861-9-2`](#rfc4861-9-2), [`RFC4861-9-3`](#rfc4861-9-3), [`RFC4861-9-4`](#rfc4861-9-4), [`RFC4861-9-5`](#rfc4861-9-5), [`RFC4861-9-6`](#rfc4861-9-6), [`RFC4861-9-7`](#rfc4861-9-7), [`RFC4861-9-8`](#rfc4861-9-8), [`RFC4861-11.2-1`](#rfc4861-11.2-1), [`RFC4861-11.2-2`](#rfc4861-11.2-2), [`RFC4861-D-1`](#rfc4861-d-1)

## Attainable ceiling

Mandatory ceiling: 43 of 138 (31.2%); excluded: Met by a lower layer: 95 of 138 (68.8%). Optional ceiling: 79 of 79 (100.0%). Lowest mandatory ceiling: rfc4861 43 of 138 (31.2%). Lowest optional ceiling: rfc4861 79 of 79 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4861-2.3-1` | All interfaces on routers MUST have a link-local address. (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 address configuration (net/ipv6/addrconf.c, addrconf_dev_config and addrconf_addr_gen) gives every interface a link-local address when it comes up; internal/plugins/iface/netlink/manage_linux.go::SetAdminUp brings the interface up and Ze leaves the address generation mode at its default, the one exception being the VRRP virtual-MAC macvlan, which carries the virtual link-local instead because RFC 9568 Section 7.4 forbids deriving an identifier from the virtual MAC |
| `RFC4861-4.1-1` | It MUST be initialized to zero by the sender and MUST be ignored by the receiver. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4861SolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/ra/rfc4861_solicitation_reader_linux_test.go#L53). **positive:** `unit/verify` [`TestRFC4861SubscriberSolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc4861_solicitation_reader_linux_test.go#L55). **negative:** no negative test. **{single-polarity}:** the reader acts on no field of the message and only reports that one arrived, so no input exists for which the correct behavior is to stop processing; the validity checks that would refuse a malformed solicitation are the RFC4861-6.1.1-2 gap |
| `RFC4861-4.1-2` | MUST NOT be included if the Source Address is the unspecified address. (§4.1) | MUST NOT | 4.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-4.1-3` | Receivers MUST silently ignore any options they do not recognize and continue processing the message. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4861SolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/ra/rfc4861_solicitation_reader_linux_test.go#L54). **positive:** `unit/verify` [`TestRFC4861SubscriberSolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc4861_solicitation_reader_linux_test.go#L56). **negative:** no negative test. **{single-polarity}:** the reader acts on no field of the message and only reports that one arrived, so no input exists for which the correct behavior is to stop processing; the validity checks that would refuse a malformed solicitation are the RFC4861-6.1.1-2 gap |
| `RFC4861-4.2-1` | Source Address MUST be the link-local address assigned to the interface from which this message is sent. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRASenderWireFormat`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/ra/rfc4861_ra_integration_linux_test.go#L31). **negative:** no negative test. **{single-polarity}:** the kernel selects the source for the link-local all-nodes destination on the interface Ze names, and no configuration of Ze yields another source, so there is no input to refuse |
| `RFC4861-4.2-2` | It MUST be initialized to zero by the sender and MUST be ignored by the receiver. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC4861RouterAdvertisementReservedFieldsAreZero`](https://github.com/ze-software/ze/blob/main/internal/core/ndp/rfc4861_router_advertisement_test.go#L30). **negative:** `unit/verify` [`TestRFC4861RouterAdvertisementReservedFieldsAreZero`](https://github.com/ze-software/ze/blob/main/internal/core/ndp/rfc4861_router_advertisement_test.go#L40) |
| `RFC4861-4.2-3` | Receivers MUST silently ignore any options they do not recognize and continue processing the message. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-4.3-1` | It MUST be initialized to zero by the sender and MUST be ignored by the receiver. (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-4.3-2` | It MUST NOT be a multicast address. (§4.3) | MUST NOT | 4.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-4.3-3` | MUST NOT be included when the source IP address is the unspecified address. (§4.3) | MUST NOT | 4.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-4.3-4` | Otherwise, on link layers that have addresses this option MUST be included in multicast solicitations and SHOULD be included in unicast solicitations. (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-4.3-5` | Receivers MUST silently ignore any options they do not recognize and continue processing the message. (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-4.4-1` | It MUST NOT be set in multicast advertisements or in unsolicited unicast advertisements. (§4.4) | MUST NOT | 4.4 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC4861UnsolicitedNAClearsTheSolicitedFlag`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc4861_unsolicited_na_test.go#L33). **{single-polarity}:** every Neighbor Advertisement Ze writes is unsolicited and multicast, so there is no case in which Ze must set the Solicited flag |
| `RFC4861-4.4-2` | It MUST be initialized to zero by the sender and MUST be ignored by the receiver. (§4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestRFC4861UnsolicitedNAReservedBitsAreZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc4861_unsolicited_na_test.go#L52). **negative:** no negative test. **{single-polarity}:** the Reserved bits are written as constant zero, and no input to BuildNA reaches them |
| `RFC4861-4.4-3` | The Target Address MUST NOT be a multicast address. (§4.4) | MUST NOT | 4.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** internal/plugins/vrrp/groups.go requires only the first IPv6 virtual-address to be link-local and refuses no multicast address after it, and internal/plugins/vrrp/transport/na.go::BuildNA copies each virtual address into the Target Address unchecked, so a configured multicast virtual address becomes the target of an unsolicited Neighbor Advertisement |
| `RFC4861-4.4-4` | This option MUST be included on link layers that have addresses when responding to multicast solicitations. (§4.4) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-4.4-5` | The option MUST be included for multicast solicitations in order to avoid infinite Neighbor Solicitation "recursion" when the peer node does not have a cache entry to return a Neighbor Advertisements message. (§4.4) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-4.4-6` | Receivers MUST silently ignore any options they do not recognize and continue processing the message. (§4.4) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-4.5-1` | Source Address MUST be the link-local address assigned to the interface from which this message is sent. (§4.5) | MUST | 4.5 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-4.5-2` | It MUST be initialized to zero by the sender and MUST be ignored by the receiver. (§4.5) | MUST | 4.5 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-4.5-3` | When the target is the actual endpoint of communication, i.e., the destination is a neighbor, the Target Address field MUST contain the same value as the ICMP Destination Address field. (§4.5) | MUST | 4.5 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-4.5-4` | Otherwise, the target is a better first-hop router and the Target Address MUST be the router's link-local address so that hosts can uniquely identify routers. (§4.5) | MUST | 4.5 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-4.5-5` | In such cases, the option MUST be included in Redirect messages. (§4.5) | MUST | 4.5 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-4.6-1` | Nodes MUST silently discard an ND packet that contains an option with length zero. (§4.6) | MUST | 4.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** both Router Solicitation readers count every packet the ICMPv6 type filter passes as a solicitation and check none of Hop Limit 255, Code 0, ICMP length or option lengths; internal/plugins/iface/ra/sender_linux.go::readSolicitations, internal/component/l2tp/ppp/ra_linux.go::rsReaderLoop |
| `RFC4861-4.6.1-1` | These options MUST be silently ignored for other Neighbor Discovery messages. (§4.6.1) | MUST | 4.6.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-4.6.2-1` | In other words, if the L flag is not set a host MUST NOT conclude that an address derived from the prefix is off-link. (§4.6.2) | MUST NOT | 4.6.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-4.6.2-2` | That is, it MUST NOT update a previous indication that the address is on-link. (§4.6.2) | MUST NOT | 4.6.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-4.6.2-3` | It MUST be initialized to zero by the sender and MUST be ignored by the receiver. (§4.6.2) | MUST | 4.6.2 | **positive:** `unit/verify` [`TestRFC4861RouterAdvertisementReservedFieldsAreZero`](https://github.com/ze-software/ze/blob/main/internal/core/ndp/rfc4861_router_advertisement_test.go#L33). **negative:** `unit/verify` [`TestRFC4861RouterAdvertisementReservedFieldsAreZero`](https://github.com/ze-software/ze/blob/main/internal/core/ndp/rfc4861_router_advertisement_test.go#L41) |
| `RFC4861-4.6.2-4` | Note that the value of this field MUST NOT exceed the Valid Lifetime field to avoid preferring addresses that are no longer valid. (§4.6.2) | MUST NOT | 4.6.2 | **positive:** `unit/verify` [`TestRFC4861PreferredLifetimeNotAboveValid`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L76). **negative:** `unit/verify` [`TestRFC4861PreferredLifetimeNotAboveValid`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L72) |
| `RFC4861-4.6.2-5` | It MUST be initialized to zero by the sender and MUST be ignored by the receiver. (§4.6.2) | MUST | 4.6.2 | **positive:** `unit/verify` [`TestRFC4861RouterAdvertisementReservedFieldsAreZero`](https://github.com/ze-software/ze/blob/main/internal/core/ndp/rfc4861_router_advertisement_test.go#L36). **negative:** no negative test. **{single-polarity}:** Reserved2 is written as a constant zero beside no other field of the octets it occupies, so there is no set bit to show the write is not a blanket clear |
| `RFC4861-4.6.2-6` | The bits in the prefix after the prefix length are reserved and MUST be initialized to zero by the sender and ignored by the receiver. (§4.6.2) | MUST | 4.6.2 | **positive:** `unit/verify` [`TestRFC4861RouterAdvertisementReservedFieldsAreZero`](https://github.com/ze-software/ze/blob/main/internal/core/ndp/rfc4861_router_advertisement_test.go#L38). **negative:** `unit/verify` [`TestRFC4861RouterAdvertisementReservedFieldsAreZero`](https://github.com/ze-software/ze/blob/main/internal/core/ndp/rfc4861_router_advertisement_test.go#L42) |
| `RFC4861-4.6.2-7` | The Prefix Information option appears in Router Advertisement packets and MUST be silently ignored for other messages. (§4.6.2) | MUST | 4.6.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-4.6.3-1` | They MUST be initialized to zero by the sender and MUST be ignored by the receiver. (§4.6.3) | MUST | 4.6.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-4.6.3-2` | This option MUST be silently ignored for other Neighbor Discovery messages. (§4.6.3) | MUST | 4.6.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-4.6.4-1` | It MUST be initialized to zero by the sender and MUST be ignored by the receiver. (§4.6.4) | MUST | 4.6.4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-4.6.4-2` | This option MUST be silently ignored for other Neighbor Discovery messages. (§4.6.4) | MUST | 4.6.4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-6.1.1-1` | Hosts MUST silently discard any received Router Solicitation Messages. (§6.1.1) | MUST | 6.1.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-6.1.1-2` | A router MUST silently discard any received Router Solicitation messages that do not satisfy all of the following validity checks: (§6.1.1) | MUST | 6.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** both Router Solicitation readers count every packet the ICMPv6 type filter passes as a solicitation and check none of Hop Limit 255, Code 0, ICMP length or option lengths; internal/plugins/iface/ra/sender_linux.go::readSolicitations, internal/component/l2tp/ppp/ra_linux.go::rsReaderLoop |
| `RFC4861-6.1.1-3` | The contents of the Reserved field, and of any unrecognized options, MUST be ignored. (§6.1.1) | MUST | 6.1.1 | **positive:** `unit/verify` [`TestRFC4861SolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/ra/rfc4861_solicitation_reader_linux_test.go#L55). **positive:** `unit/verify` [`TestRFC4861SubscriberSolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc4861_solicitation_reader_linux_test.go#L57). **negative:** no negative test. **{single-polarity}:** the reader acts on no field of the message and only reports that one arrived, so no input exists for which the correct behavior is to stop processing; the validity checks that would refuse a malformed solicitation are the RFC4861-6.1.1-2 gap |
| `RFC4861-6.1.1-4` | The contents of any defined options that are not specified to be used with Router Solicitation messages MUST be ignored and the packet processed as normal. (§6.1.1) | MUST | 6.1.1 | **positive:** `unit/verify` [`TestRFC4861SolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/ra/rfc4861_solicitation_reader_linux_test.go#L56). **positive:** `unit/verify` [`TestRFC4861SubscriberSolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc4861_solicitation_reader_linux_test.go#L58). **negative:** no negative test. **{single-polarity}:** the reader acts on no field of the message and only reports that one arrived, so no input exists for which the correct behavior is to stop processing; the validity checks that would refuse a malformed solicitation are the RFC4861-6.1.1-2 gap |
| `RFC4861-6.1.2-1` | A node MUST silently discard any received Router Advertisement messages that do not satisfy all of the following validity checks: (§6.1.2) | MUST | 6.1.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-6.1.2-2` | The contents of the Reserved field, and of any unrecognized options, MUST be ignored. (§6.1.2) | MUST | 6.1.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-6.1.2-3` | The contents of any defined options that are not specified to be used with Router Advertisement messages MUST be ignored and the packet processed as normal. (§6.1.2) | MUST | 6.1.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-6.2.1-1` | A router MUST allow for the following conceptual variables to be configured by system management. (§6.2.1) | MUST | 6.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the router-advertisement container (internal/component/iface/yang/ze-iface-conf.yang, internal/component/iface/config_ra.go::parseRAConfig) offers every Section 6.2.1 variable except AdvLinkMTU, so no MTU option can be configured or sent |
| `RFC4861-6.2.1-2` | Note that AdvSendAdvertisements MUST be FALSE by default so that a node will not accidentally start acting as a router unless it is explicitly configured by system management to send Router Advertisements. (§6.2.1) | MUST | 6.2.1 | **positive:** `unit/verify` [`TestRFC4861AdvSendAdvertisementsIsFalseByDefault`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L82). **negative:** no negative test. **{single-polarity}:** the requirement fixes the default value, and an explicitly enabled unit starting a sender is the ordinary behaviour TestReconcileRA proves |
| `RFC4861-6.2.1-3` | MUST be no less than 4 seconds and no greater than 1800 seconds. (§6.2.1) | MUST | 6.2.1 | **positive:** `unit/verify` [`TestRFC4861MaxRtrAdvIntervalBounds`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L30). **negative:** `unit/verify` [`TestRFC4861MaxRtrAdvIntervalBounds`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L27) |
| `RFC4861-6.2.1-4` | MUST be no less than 3 seconds and no greater than .75 * MaxRtrAdvInterval. (§6.2.1) | MUST | 6.2.1 | **positive:** `unit/verify` [`TestRFC4861MinRtrAdvIntervalBounds`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L41). **negative:** `unit/verify` [`TestRFC4861MinRtrAdvIntervalBounds`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L38) |
| `RFC4861-6.2.1-5` | MUST be no greater than 3,600,000 milliseconds (1 hour). (§6.2.1) | MUST | 6.2.1 | **positive:** `unit/verify` [`TestRFC4861AdvReachableTimeBound`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L51). **negative:** `unit/verify` [`TestRFC4861AdvReachableTimeBound`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L49) |
| `RFC4861-6.2.1-6` | MUST be either zero or between MaxRtrAdvInterval and 9000 seconds. (§6.2.1) | MUST | 6.2.1 | **positive:** `unit/verify` [`TestRFC4861AdvDefaultLifetimeBounds`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L60). **negative:** `unit/verify` [`TestRFC4861AdvDefaultLifetimeBounds`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L57) |
| `RFC4861-6.2.1-7` | This value MUST NOT be larger than AdvValidLifetime. (§6.2.1) | MUST NOT | 6.2.1 | **positive:** `unit/verify` [`TestRFC4861PreferredLifetimeNotAboveValid`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L75). **negative:** `unit/verify` [`TestRFC4861PreferredLifetimeNotAboveValid`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L69) |
| `RFC4861-6.2.1-8` | However, external router behavior MUST be the same as host behavior with respect to these variables. (§6.2.1) | MUST | 6.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze advertises the configured reachable-time, retransmit-timer and hop-limit (internal/component/iface/config_ra.go) but never sets the advertising interface's own net.ipv6.neigh.<if>.base_reachable_time_ms, retrans_time_ms or net.ipv6.conf.<if>.hop_limit, so the router's own Neighbor Discovery keeps the kernel values whatever it advertises |
| `RFC4861-6.2.2-1` | A router MUST NOT send Router Advertisements out any interface that is not an advertising interface. (§6.2.2) | MUST NOT | 6.2.2 | **positive:** `unit/verify` [`TestRFC4861OnlyAnAdvertisingInterfaceSends`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L100). **negative:** `unit/verify` [`TestRFC4861OnlyAnAdvertisingInterfaceSends`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L97) |
| `RFC4861-6.2.2-2` | A router MUST join the all-routers multicast address on an advertising interface. (§6.2.2) | MUST | 6.2.2 | **positive:** `unit/verify` [`TestRASolicitedResponse`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/ra/rfc4861_ra_integration_linux_test.go#L130). **negative:** no negative test. **{single-polarity}:** the sender joins the group unconditionally when it opens its socket, so no configuration of an advertising interface exists in which not joining would be correct |
| `RFC4861-6.2.4-1` | A host MUST NOT send Router Advertisement messages at any time. (§6.2.4) | MUST NOT | 6.2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a unit with router-advertisement enabled advertises whatever the interface forwarding state; internal/plugins/iface/ra/doctor.go only reports forwarding off (design decision D-6), so a node acting as a host still sends Router Advertisements |
| `RFC4861-6.2.5-1` | In addition, the host MUST ensure that subsequent Neighbor Advertisement messages sent from the interface have the Router flag set to zero. (§6.2.5) | MUST | 6.2.5 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-6.2.5-2` | In such cases, subsequent Router Advertisements MUST set the Router Lifetime field to zero. (§6.2.5) | MUST | 6.2.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no sender watches the forwarding state; when forwarding is turned off on an interface that stays an advertising interface, internal/plugins/iface/ra/sender_linux.go keeps advertising the configured Router Lifetime rather than zero |
| `RFC4861-6.2.6-1` | A host MUST silently discard any received Router Solicitation messages. (§6.2.6) | MUST | 6.2.6 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-6.2.6-2` | In all cases, Router Advertisements sent in response to a Router Solicitation MUST be delayed by a random time between 0 and MAX_RA_DELAY_TIME seconds. (§6.2.6) | MUST | 6.2.6 | **positive:** `unit/verify` [`TestRFC4861SolicitedAnswerDelayIsRandomUpToMaxRADelayTime`](https://github.com/ze-software/ze/blob/main/internal/core/ndp/rfc4861_router_advertisement_test.go#L80). **negative:** no negative test. **{single-polarity}:** the delay is a random draw with no input, so there is no case in which a delay outside the range is correct |
| `RFC4861-6.2.6-3` | (If a single advertisement is sent in response to multiple solicitations, the delay is relative to the first solicitation.) In addition, consecutive Router Advertisements sent to the all-nodes multicast address MUST be rate limited to no more than one advertisement every MIN_DELAY_BETWEEN_RAS seconds. (§6.2.6) | MUST | 6.2.6 | **positive:** `unit/verify` [`TestRFC4861SolicitedAnswerIsRateLimited`](https://github.com/ze-software/ze/blob/main/internal/core/ndp/rfc4861_router_advertisement_test.go#L101). **negative:** no negative test. **{single-polarity}:** the rate limit has no case in which an earlier answer is correct, and the undelayed answer outside the window is ordinary scheduling TestRARateLimit covers |
| `RFC4861-6.2.6-4` | In all cases, however, unsolicited multicast advertisements MUST NOT be sent more frequently than indicated by MinRtrAdvInterval. (§6.2.6) | MUST NOT | 6.2.6 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC4861UnsolicitedIntervalIsNeverBelowMinRtrAdvInterval`](https://github.com/ze-software/ze/blob/main/internal/core/ndp/rfc4861_router_advertisement_test.go#L113). **{single-polarity}:** the interval draw has no input for which an interval below the minimum is correct past the initial burst, which Section 6.2.4 governs |
| `RFC4861-6.2.6-5` | Router Solicitations in which the Source Address is the unspecified address MUST NOT update the router's Neighbor Cache; solicitations with a proper source address update the Neighbor Cache as follows. (§6.2.6) | MUST NOT | 6.2.6 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-6.2.6-6` | If the router already has a Neighbor Cache entry for the solicitation's sender, the solicitation contains a Source Link-Layer Address option, and the received link-layer address differs from that already in the cache, then the link-layer address SHOULD be updated in the appropriate Neighbor Cache entry, and its reachability state MUST also be set to STALE. (§6.2.6) | MUST | 6.2.6 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-6.2.6-7` | Whether or not a Source Link-Layer Address option is provided, if a Neighbor Cache entry for the solicitation's sender exists (or is created) the entry's IsRouter flag MUST be set to FALSE. (§6.2.6) | MUST | 6.2.6 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-6.3.4-1` | Hosts accept the union of all received information; the receipt of a Router Advertisement MUST NOT invalidate all information received in a previous advertisement or from another source. (§6.3.4) | MUST NOT | 6.3.4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-6.3.4-2` | In particular, a host MUST NOT interpret the unspecified value as meaning change back to the default value that was in use before the first Router Advertisement was received. (§6.3.4) | MUST NOT | 6.3.4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-6.3.4-3` | However, a host MUST retain at least two router addresses and SHOULD retain more. (§6.3.4) | MUST | 6.3.4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-6.3.4-4` | If the advertisement contains a Source Link-Layer Address option, the link-layer address SHOULD be recorded in the Neighbor Cache entry for the router (creating an entry if necessary) and the IsRouter flag in the Neighbor Cache entry MUST be set to TRUE. (§6.3.4) | MUST | 6.3.4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-6.3.4-5` | If no Source Link-Layer Address is included, but a corresponding Neighbor Cache entry exists, its IsRouter flag MUST be set to TRUE. (§6.3.4) | MUST | 6.3.4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-6.3.4-6` | If a Neighbor Cache entry is created for the router, its reachability state MUST be set to STALE as specified in Section 7.3.3. (§6.3.4) | MUST | 6.3.4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-6.3.4-7` | If a cache entry already exists and is updated with a different link-layer address, the reachability state MUST also be set to STALE. (§6.3.4) | MUST | 6.3.4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-6.3.4-8` | Note, however, that a Prefix Information option with the on-link flag set to zero conveys no information concerning on-link determination and MUST NOT be interpreted to mean that addresses covered by the prefix are off-link. (§6.3.4) | MUST NOT | 6.3.4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-6.3.5-1` | When removing a router from the Default Router list, the node MUST update the Destination Cache in such a way that all entries using the router perform next-hop determination again rather than continue sending traffic to the (deleted) router. (§6.3.5) | MUST | 6.3.5 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-6.3.7-1` | Once the host sends a Router Solicitation, and receives a valid Router Advertisement with a non-zero Router Lifetime, the host MUST desist from sending additional solicitations on that interface, until the next time one of the above events occurs. (§6.3.7) | MUST | 6.3.7 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.1.1-1` | A node MUST silently discard any received Neighbor Solicitation messages that do not satisfy all of the following validity checks: (§7.1.1) | MUST | 7.1.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.1.1-2` | The contents of the Reserved field, and of any unrecognized options, MUST be ignored. (§7.1.1) | MUST | 7.1.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.1.1-3` | The contents of any defined options that are not specified to be used with Neighbor Solicitation messages MUST be ignored and the packet processed as normal. (§7.1.1) | MUST | 7.1.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.1.2-1` | A node MUST silently discard any received Neighbor Advertisement messages that do not satisfy all of the following validity checks: (§7.1.2) | MUST | 7.1.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.1.2-2` | The contents of the Reserved field, and of any unrecognized options, MUST be ignored. (§7.1.2) | MUST | 7.1.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.1.2-3` | The contents of any defined options that are not specified to be used with Neighbor Advertisement messages MUST be ignored and the packet processed as normal. (§7.1.2) | MUST | 7.1.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.2-1` | These messages MUST NOT create or update neighbor cache entries, except with respect to the IsRouter flag as specified in Sections 6.3.4 and 7.2.5. (§7.2) | MUST NOT | 7.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.2.1-1` | When a multicast-capable interface becomes enabled, the node MUST join the all-nodes multicast address on that interface, as well as the solicited-node multicast address corresponding to each of the IP addresses assigned to the interface. (§7.2.1) | MUST | 7.2.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.2.1-2` | In such cases the node MUST join and leave the solicited-node multicast address corresponding to the new and old addresses, respectively. (§7.2.1) | MUST | 7.2.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.2.1-3` | Note that multiple unicast addresses may map into the same solicited-node multicast address; a node MUST NOT leave the solicited-node multicast group until all assigned addresses corresponding to that multicast address have been removed. (§7.2.1) | MUST NOT | 7.2.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.2.2-1` | If the solicitation is being sent to a solicited-node multicast address, the sender MUST include its link-layer address (if it has one) as a Source Link-Layer Address option. (§7.2.2) | MUST | 7.2.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.2.2-2` | While waiting for address resolution to complete, the sender MUST, for each neighbor, retain a small queue of packets waiting for address resolution to complete. (§7.2.2) | MUST | 7.2.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.2.2-3` | The queue MUST hold at least one packet, and MAY contain more. (§7.2.2) | MUST | 7.2.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.2.2-4` | Retransmissions MUST be rate-limited to at most one solicitation per neighbor every RetransTimer milliseconds. (§7.2.2) | MUST | 7.2.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.2.2-5` | The sender MUST return ICMP destination unreachable indications with code 3 (Address Unreachable) for each packet queued awaiting address resolution. (§7.2.2) | MUST | 7.2.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.2.3-1` | A valid Neighbor Solicitation that does not meet any of the following requirements MUST be silently discarded: (§7.2.3) | MUST | 7.2.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.2.3-2` | If an entry already exists, and the cached link-layer address differs from the one in the received Source Link-Layer option, the cached address should be replaced by the received address, and the entry's reachability state MUST be set to STALE. (§7.2.3) | MUST | 7.2.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.2.3-3` | If a Neighbor Cache entry already exists, its IsRouter flag MUST NOT be modified. (§7.2.3) | MUST NOT | 7.2.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.2.3-4` | If the Source Address is the unspecified address, the node MUST NOT create or update the Neighbor Cache entry. (§7.2.3) | MUST NOT | 7.2.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.2.4-1` | If the solicitation's IP Destination Address is a multicast address, the Target Link-Layer option MUST be included in the advertisement. (§7.2.4) | MUST | 7.2.4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.2.4-2` | Furthermore, if the node is a router, it MUST set the Router flag to one; otherwise, it MUST set the flag to zero. (§7.2.4) | MUST | 7.2.4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.2.4-3` | If the source of the solicitation is the unspecified address, the node MUST set the Solicited flag to zero and multicast the advertisement to the all-nodes address. (§7.2.4) | MUST | 7.2.4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.2.4-4` | Otherwise, the node MUST set the Solicited flag to one and unicast the advertisement to the Source Address of the solicitation. (§7.2.4) | MUST | 7.2.4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.2.5-1` | Otherwise, the received advertisement should be ignored and MUST NOT update the cache. (§7.2.5) | MUST NOT | 7.2.5 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.2.5-2` | If the Override flag is set, or the supplied link-layer address is the same as that in the cache, or no Target Link-Layer Address option was supplied, the received advertisement MUST update the Neighbor Cache entry as follows: (§7.2.5) | MUST | 7.2.5 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.2.5-3` | - The link-layer address in the Target Link-Layer Address option MUST be inserted in the cache (if one is supplied and differs from the already recorded address). (§7.2.5) | MUST | 7.2.5 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.2.5-4` | - If the Solicited flag is set, the state of the entry MUST be set to REACHABLE. (§7.2.5) | MUST | 7.2.5 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.2.5-5` | If the Solicited flag is zero and the link- layer address was updated with a different address, the state MUST be set to STALE. (§7.2.5) | MUST | 7.2.5 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.2.5-6` | - The IsRouter flag in the cache entry MUST be set based on the Router flag in the received advertisement. (§7.2.5) | MUST | 7.2.5 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.2.5-7` | In those cases where the IsRouter flag changes from TRUE to FALSE as a result of this update, the node MUST remove that router from the Default Router List and update the Destination Cache entries for all destinations using that neighbor as a router as specified in Section 7.3.3. (§7.2.5) | MUST | 7.2.5 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.2.6-1` | These advertisements MUST be separated by at least RetransTimer seconds. (§7.2.6) | MUST | 7.2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** internal/plugins/vrrp/transport/announce.go sends each unsolicited Neighbor Advertisement announceRepeatCount (3) times announceRepeatInterval (100 ms) apart, below RetransTimer (RETRANS_TIMER is 1,000 ms) |
| `RFC4861-7.2.6-2` | The Solicited flag MUST be set to zero, in order to avoid confusing the Neighbor Unreachability Detection algorithm. (§7.2.6) | MUST | 7.2.6 | **positive:** `unit/verify` [`TestRFC4861UnsolicitedNAClearsTheSolicitedFlag`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc4861_unsolicited_na_test.go#L36). **negative:** no negative test. **{single-polarity}:** every Neighbor Advertisement Ze writes is unsolicited, so there is no case in which the Solicited flag is set |
| `RFC4861-7.2.6-3` | If the node is a router, it MUST set the Router flag to one; otherwise, it MUST set it to zero. (§7.2.6) | MUST | 7.2.6 | **positive:** `unit/verify` [`TestRFC4861UnsolicitedNASetsTheRouterFlag`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc4861_unsolicited_na_test.go#L44). **negative:** no negative test. **{single-polarity}:** Ze writes Neighbor Advertisements only as a VRRP Master, which is a router, so the Router-flag-zero branch for a host never arises in Ze |
| `RFC4861-7.2.8-1` | A proxy MUST join the solicited-node multicast address(es) that correspond to the IP address(es) assigned to the node for which it is proxying. (§7.2.8) | MUST | 7.2.8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze offers no Neighbor Discovery proxy: no Ze code answers a Neighbor Solicitation or joins a solicited-node group on another node's behalf, and none sets the proxy_ndp sysctl or adds a proxy neighbour entry; no owner ruling names this feature unwanted (ai/META-DECISIONS.md, Goal and scope) |
| `RFC4861-7.2.8-2` | All solicited proxy Neighbor Advertisement messages MUST have the Override flag set to zero. (§7.2.8) | MUST | 7.2.8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze offers no Neighbor Discovery proxy: no Ze code answers a Neighbor Solicitation or joins a solicited-node group on another node's behalf, and none sets the proxy_ndp sysctl or adds a proxy neighbour entry; no owner ruling names this feature unwanted (ai/META-DECISIONS.md, Goal and scope) |
| `RFC4861-7.3.1-1` | Receipt of other Neighbor Discovery messages, such as Router Advertisements and Neighbor Advertisement with the Solicited flag set to zero, MUST NOT be treated as a reachability confirmation. (§7.3.1) | MUST NOT | 7.3.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.3.3-1` | A node MUST NOT send Neighbor Solicitations to the same neighbor more frequently than once every RetransTimer milliseconds. (§7.3.3) | MUST NOT | 7.3.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.3.3-2` | To properly detect the case where a router switches from being a router to being a host (e.g., if its IP forwarding capability is turned off by system management), a node MUST compare the Router flag field in all received Neighbor Advertisement messages with the IsRouter flag recorded in the Neighbor Cache entry. (§7.3.3) | MUST | 7.3.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.3.3-3` | When a node detects that a neighbor has changed from being a router to being a host, the node MUST remove that router from the Default Router List and update the Destination Cache as described in Section 6.3.5. (§7.3.3) | MUST | 7.3.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-7.3.3-4` | However, link-specific information MUST NOT be used to confirm the reachability of a neighbor; such information does not provide end-to-end confirmation between neighboring IP layers. (§7.3.3) | MUST NOT | 7.3.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-8-1` | A router MUST be able to determine the link-local address for each of its neighboring routers in order to ensure that the target address in a Redirect message identifies the neighbor router by its link-local address. (§8) | MUST | 8 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-8.1-1` | A host MUST silently discard any received Redirect message that does not satisfy all of the following validity checks: (§8.1) | MUST | 8.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-8.1-2` | The contents of the Reserved field, and of any unrecognized options, MUST be ignored. (§8.1) | MUST | 8.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-8.1-3` | The contents of any defined options that are not specified to be used with Redirect messages MUST be ignored and the packet processed as normal. (§8.1) | MUST | 8.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-8.1-4` | A host MUST NOT consider a redirect invalid just because the Target Address of the redirect is not covered under one of the link's prefixes. (§8.1) | MUST NOT | 8.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-8.2-1` | If the target is a router, that router's link-local address MUST be used. (§8.2) | MUST | 8.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-8.2-2` | If the target is a host, the target address field MUST be set to the same value as the Destination Address field. (§8.2) | MUST | 8.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-8.2-3` | A router MUST limit the rate at which Redirect messages are sent, in order to limit the bandwidth and processing costs incurred by the Redirect messages when the source does not correctly respond to the Redirects, or the source chooses to ignore unauthenticated Redirect messages. (§8.2) | MUST | 8.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-8.2-4` | A router MUST NOT update its routing tables upon receipt of a Redirect. (§8.2) | MUST NOT | 8.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-8.3-1` | If a Neighbor Cache entry is created for the target, its reachability state MUST be set to STALE as specified in Section 7.3.3. (§8.3) | MUST | 8.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-8.3-2` | If a cache entry already existed and it is updated with a different link-layer address, its reachability state MUST also be set to STALE. (§8.3) | MUST | 8.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-8.3-3` | If the Target and Destination Addresses are the same, the host MUST treat the Target as on-link. (§8.3) | MUST | 8.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-8.3-4` | If the Target Address is not the same as the Destination Address, the host MUST set IsRouter to TRUE for the target. (§8.3) | MUST | 8.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-8.3-5` | A host MUST NOT send Redirect messages. (§8.3) | MUST NOT | 8.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-9-1` | In order to ensure that future extensions properly coexist with current implementations, all nodes MUST silently ignore any options they do not recognize in received ND packets and continue processing the packet. (§9) | MUST | 9 | **positive:** `unit/verify` [`TestRFC4861SolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/ra/rfc4861_solicitation_reader_linux_test.go#L57). **positive:** `unit/verify` [`TestRFC4861SubscriberSolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc4861_solicitation_reader_linux_test.go#L59). **negative:** no negative test. **{single-polarity}:** the reader acts on no field of the message and only reports that one arrived, so no input exists for which the correct behavior is to stop processing; the validity checks that would refuse a malformed solicitation are the RFC4861-6.1.1-2 gap |
| `RFC4861-9-2` | All options specified in this document MUST be recognized. (§9) | MUST | 9 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-9-3` | A node MUST NOT ignore valid options just because the ND message contains unrecognized ones. (§9) | MUST NOT | 9 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC4861SolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/ra/rfc4861_solicitation_reader_linux_test.go#L58). **negative:** `unit/verify` [`TestRFC4861SubscriberSolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc4861_solicitation_reader_linux_test.go#L60). **{single-polarity}:** the reader acts on no field of the message and only reports that one arrived, so no input exists for which the correct behavior is to stop processing; the validity checks that would refuse a malformed solicitation are the RFC4861-6.1.1-2 gap |
| `RFC4861-9-4` | Thus, a receiver MUST NOT associate any action with the absence of an option in a particular packet. (§9) | MUST NOT | 9 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC4861SolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/ra/rfc4861_solicitation_reader_linux_test.go#L59). **negative:** `unit/verify` [`TestRFC4861SubscriberSolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc4861_solicitation_reader_linux_test.go#L61). **{single-polarity}:** the reader acts on no field of the message and only reports that one arrived, so no input exists for which the correct behavior is to stop processing; the validity checks that would refuse a malformed solicitation are the RFC4861-6.1.1-2 gap |
| `RFC4861-9-5` | Options in Neighbor Discovery packets can appear in any order; receivers MUST be prepared to process them independently of their order. (§9) | MUST | 9 | **positive:** `unit/verify` [`TestRFC4861SolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/ra/rfc4861_solicitation_reader_linux_test.go#L60). **positive:** `unit/verify` [`TestRFC4861SubscriberSolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc4861_solicitation_reader_linux_test.go#L62). **negative:** no negative test. **{single-polarity}:** the reader acts on no field of the message and only reports that one arrived, so no input exists for which the correct behavior is to stop processing; the validity checks that would refuse a malformed solicitation are the RFC4861-6.1.1-2 gap |
| `RFC4861-9-6` | The amount of data to include in the Redirected Header option MUST be limited so that the entire redirect packet does not exceed the minimum MTU required to support IPv6 as specified in [IPv6]. (§9) | MUST | 9 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-9-7` | When adding options to an ND packet, a node MUST NOT exceed the link MTU. (§9) | MUST NOT | 9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the router-advertisement prefix list (internal/component/iface/yang/ze-iface-conf.yang) has no max-elements and internal/plugins/iface/ra/sender_linux.go sends every option in one advertisement, so about 40 prefixes make the advertisement larger than a 1500 octet link MTU, and Ze never splits it |
| `RFC4861-9-8` | Receivers MUST silently ignore any options they do not recognize and continue processing the message. (§9) | MUST | 9 | **positive:** `unit/verify` [`TestRFC4861SolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/ra/rfc4861_solicitation_reader_linux_test.go#L61). **positive:** `unit/verify` [`TestRFC4861SubscriberSolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc4861_solicitation_reader_linux_test.go#L63). **negative:** no negative test. **{single-polarity}:** the reader acts on no field of the message and only reports that one arrived, so no input exists for which the correct behavior is to stop processing; the validity checks that would refuse a malformed solicitation are the RFC4861-6.1.1-2 gap |
| `RFC4861-11.2-1` | In any case, if either [IPv6-AUTH] or [IPv6-ESP] is used, ND packets MUST be verified for the purpose of authentication. (§11.2) | MUST | 11.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze has no configuration that secures Neighbor Discovery with AH or ESP, so nothing Ze sets up verifies ND packets for authentication or discards those that fail it; no owner ruling names this feature unwanted (ai/META-DECISIONS.md, Goal and scope) |
| `RFC4861-11.2-2` | Packets that fail authentication checks MUST be silently discarded. (§11.2) | MUST | 11.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze has no configuration that secures Neighbor Discovery with AH or ESP, so nothing Ze sets up verifies ND packets for authentication or discards those that fail it; no owner ruling names this feature unwanted (ai/META-DECISIONS.md, Goal and scope) |
| `RFC4861-D-1` | But when there is no host vs. router information in the ND message, the receipt of the message MUST NOT cause a change to the IsRouter state. (§D) | MUST NOT | D | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| `RFC4861-4.1-4` | Otherwise, it SHOULD be included on link layers that have addresses. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-4.2-4` | A Lifetime of 0 indicates that the router is not a default router and SHOULD NOT appear on the default router list. (§4.2) | SHOULD NOT | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-4.2-6` | MTU SHOULD be sent on links that have a variable MTU (as specified in the document that describes how to run IP over the particular link type). (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-4.2-8` | A router SHOULD include all its on-link prefixes (except the link-local prefix) so that multihomed hosts have complete prefix information about on-link destinations for the links to which they attach. (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-4.4-7` | It SHOULD NOT be set in solicited advertisements for anycast addresses and in solicited proxy advertisements. (§4.4) | SHOULD NOT | 4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-4.4-8` | It SHOULD be set in other solicited advertisements and in unsolicited advertisements. (§4.4) | SHOULD | 4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-4.4-9` | When responding to a unicast Neighbor Solicitation this option SHOULD be included. (§4.4) | SHOULD | 4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-4.5-6` | It SHOULD be included (if known). (§4.5) | SHOULD | 4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-4.6.2-8` | A router SHOULD NOT send a prefix option for the link-local prefix and a host SHOULD ignore such a prefix option. (§4.6.2) | SHOULD NOT | 4.6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-5.1-1` | Received Router Advertisements SHOULD NOT modify the invalidation timer for the link-local prefix. (§5.1) | SHOULD NOT | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.2.1-9` | The link-local prefix SHOULD NOT be included in the list of advertised prefixes. (§6.2.1) | SHOULD NOT | 6.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.2.3-3` | However, when responding to a Router Solicitation or while sending the first few initial unsolicited advertisements, a router SHOULD include all options so that all information (e.g., prefixes) is propagated quickly during system initialization. (§6.2.3) | SHOULD | 6.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.2.4-2` | For the first few advertisements (up to MAX_INITIAL_RTR_ADVERTISEMENTS) sent from an interface when it becomes an advertising interface, if the randomly chosen interval is greater than MAX_INITIAL_RTR_ADVERT_INTERVAL, the timer SHOULD be set to MAX_INITIAL_RTR_ADVERT_INTERVAL instead. (§6.2.4) | SHOULD | 6.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.2.5-3` | In such cases, the router SHOULD transmit one or more (but not more than MAX_FINAL_RTR_ADVERTISEMENTS) final multicast Router Advertisements on the interface with a Router Lifetime field of zero. (§6.2.5) | SHOULD | 6.2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.2.5-4` | In the case of a router becoming a host, the system SHOULD also depart from the all-routers IP multicast group on all interfaces on which the router supports IP multicast (whether or not they had been advertising interfaces). (§6.2.5) | SHOULD | 6.2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.2.7-1` | Routers SHOULD inspect valid Router Advertisements sent by other routers and verify that the routers are advertising consistent information on a link. (§6.2.7) | SHOULD | 6.2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.2.7-2` | Detected inconsistencies indicate that one or more routers might be misconfigured and SHOULD be logged to system or network management. (§6.2.7) | SHOULD | 6.2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.2.7-3` | The minimum set of information to check includes: - Cur Hop Limit values (except for the unspecified value of zero other inconsistencies SHOULD be logged to system network management). - Values of the M or O flags. - Reachable Time values (except for the unspecified value of zero). - Retrans Timer values (except for the unspecified value of zero). - Values in the MTU options. - Preferred and Valid Lifetimes for the same prefix. (§6.2.7) | SHOULD | 6.2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.2.7-4` | Due to link propagation delays and potentially poorly synchronized clocks between the routers such comparison SHOULD allow some time skew. (§6.2.7) | SHOULD | 6.2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.2.7-5` | The logging of errors SHOULD be restricted to conflicting information that causes hosts to switch from one value to another with each received advertisement. (§6.2.7) | SHOULD | 6.2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.2.8-1` | If a router changes the link-local address for one of its interfaces, it SHOULD inform hosts of this change. (§6.2.8) | SHOULD | 6.2.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.2.8-2` | The router SHOULD multicast a few Router Advertisements from the old link-local address with the Router Lifetime field set to zero and also multicast a few Router Advertisements from the new link-local address. (§6.2.8) | SHOULD | 6.2.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.3.4-10` | If the received Cur Hop Limit value is non-zero, the host SHOULD set its CurHopLimit variable to the received value. (§6.3.4) | SHOULD | 6.3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.3.4-11` | If the received Reachable Time value is non-zero, the host SHOULD set its BaseReachableTime variable to the received value. (§6.3.4) | SHOULD | 6.3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.3.4-12` | If the new value differs from the previous value, the host SHOULD re-compute a new random ReachableTime value. (§6.3.4) | SHOULD | 6.3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.3.4-13` | In such cases, an implementation SHOULD ensure that a new random value gets re-computed at least once every few hours. (§6.3.4) | SHOULD | 6.3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.3.4-14` | The RetransTimer variable SHOULD be copied from the Retrans Timer field, if the received value is non-zero. (§6.3.4) | SHOULD | 6.3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.3.4-15` | If the MTU option is present, hosts SHOULD copy the option's value into LinkMTU so long as the value is greater than or equal to the minimum link MTU [IPv6] and does not exceed the maximum LinkMTU value specified in the link-type-specific document (e.g., [IPv6-ETHER]). (§6.3.4) | SHOULD | 6.3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.3.6-1` | The policy for selecting routers from the Default Router List is as follows: 1) Routers that are reachable or probably reachable (i.e., in any state other than INCOMPLETE) SHOULD be preferred over routers whose reachability is unknown or suspect (i.e., in the INCOMPLETE state, or for which no Neighbor Cache entry exists). (§6.3.6) | SHOULD | 6.3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.3.6-2` | Further implementation hints on default router selection when multiple equivalent routers are available are discussed in [LD-SHRE]. 2) When no routers on the list are known to be reachable or probably reachable, routers SHOULD be selected in a round-robin fashion, so that subsequent requests for a default router do not return the same router until all other routers have been selected. (§6.3.6) | SHOULD | 6.3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.3.7-2` | To obtain Router Advertisements quickly, a host SHOULD transmit up to MAX_RTR_SOLICITATIONS Router Solicitation messages, each separated by at least RTR_SOLICITATION_INTERVAL seconds. (§6.3.7) | SHOULD | 6.3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.3.7-3` | The Source Link-Layer Address option SHOULD be set to the host's link-layer address, if the IP source address is not the unspecified address. (§6.3.7) | SHOULD | 6.3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.3.7-4` | Before a host sends an initial solicitation, it SHOULD delay the transmission for a random amount of time between 0 and MAX_RTR_SOLICITATION_DELAY. (§6.3.7) | SHOULD | 6.3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.3.7-7` | Moreover, a host SHOULD send at least one solicitation in the case where an advertisement is received prior to having sent a solicitation. (§6.3.7) | SHOULD | 6.3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.2.2-6` | If the source address of the packet prompting the solicitation is the same as one of the addresses assigned to the outgoing interface, that address SHOULD be placed in the IP Source Address of the outgoing solicitation. (§7.2.2) | SHOULD | 7.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.2.2-7` | Otherwise, the sender SHOULD include its link-layer address (if it has one) as a Source Link-Layer Address option. (§7.2.2) | SHOULD | 7.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.2.2-9` | However, the number of queued packets per neighbor SHOULD be limited to some small value. (§7.2.2) | SHOULD | 7.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.2.2-10` | When a queue overflows, the new arrival SHOULD replace the oldest entry. (§7.2.2) | SHOULD | 7.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.2.2-11` | While awaiting a response, the sender SHOULD retransmit Neighbor Solicitation messages approximately every RetransTimer milliseconds, even in the absence of additional traffic to the neighbor. (§7.2.2) | SHOULD | 7.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.2.3-5` | If the Source Address is not the unspecified address and, on link layers that have addresses, the solicitation includes a Source Link-Layer Address option, then the recipient SHOULD create or update the Neighbor Cache entry for the IP Source Address of the solicitation. (§7.2.3) | SHOULD | 7.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.2.3-6` | If an entry does not already exist, the node SHOULD create a new one and set its reachability state to STALE as specified in Section 7.3.3. (§7.2.3) | SHOULD | 7.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.2.3-7` | If a Neighbor Cache entry is created, the IsRouter flag SHOULD be set to FALSE. (§7.2.3) | SHOULD | 7.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.2.4-6` | If the Target Address is either an anycast address or a unicast address for which the node is providing proxy service, or the Target Link-Layer Address option is not included, the Override flag SHOULD be set to zero. (§7.2.4) | SHOULD | 7.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.2.4-7` | Otherwise, the Override flag SHOULD be set to one. (§7.2.4) | SHOULD | 7.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.2.4-8` | If the Target Address is an anycast address, the sender SHOULD delay sending a response for a random time between 0 and MAX_ANYCAST_DELAY_TIME seconds. (§7.2.4) | SHOULD | 7.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.2.5-8` | If no entry exists, the advertisement SHOULD be silently discarded. (§7.2.5) | SHOULD | 7.2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.2.5-9` | If the link layer has addresses and no Target Link-Layer Address option is included, the receiving node SHOULD silently discard the received advertisement. (§7.2.5) | SHOULD | 7.2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.2.6-7` | In such a case, the node SHOULD introduce a small delay between the sending of each advertisement to reduce the probability of the advertisements being lost due to congestion. (§7.2.6) | SHOULD | 7.2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.2.7-1` | First, Neighbor Advertisements sent in response to a Neighbor Solicitation SHOULD be delayed by a random time between 0 and MAX_ANYCAST_DELAY_TIME to reduce the probability of network congestion. (§7.2.7) | SHOULD | 7.2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.2.7-2` | Second, the Override flag in Neighbor Advertisements SHOULD be set to 0, so that when multiple advertisements are received, the first received advertisement is used rather than the most recently received advertisement. (§7.2.7) | SHOULD | 7.2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.2.8-4` | This SHOULD be done using a multicast listener discovery protocol such as [MLD] or [MLDv2]. (§7.2.8) | SHOULD | 7.2.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.3.1-2` | When available, this upper-layer information SHOULD be used. (§7.3.1) | SHOULD | 7.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.3.3-5` | If address resolution fails, the entry SHOULD be deleted, so that subsequent traffic to that neighbor invokes the next-hop determination procedure again. (§7.3.3) | SHOULD | 7.3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.3.3-6` | If no response is received after waiting RetransTimer milliseconds after sending the MAX_UNICAST_SOLICIT solicitations, retransmissions cease and the entry SHOULD be deleted. (§7.3.3) | SHOULD | 7.3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.3.3-7` | In addition, should a cached link-layer address be modified due to receiving one of the above messages, the state SHOULD also be set to STALE to provide prompt verification that the path to the new link-layer address is working. (§7.3.3) | SHOULD | 7.3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-8.2-5` | A router SHOULD send a redirect message, subject to rate limiting, whenever it forwards a packet that is not explicitly addressed to itself (i.e., a packet that is not source routed through the router) in which: - the Source Address field of the packet identifies a neighbor, and - the router determines (by means outside the scope of this specification) that a better first-hop node resides on the same link as the sending node for the Destination Address of the packet being forwarded, and - the Destination Address of the packet is not a multicast address. (§8.2) | SHOULD | 8.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-8.3-6` | A host receiving a valid redirect SHOULD update its Destination Cache accordingly so that subsequent traffic goes to the specified target. (§8.3) | SHOULD | 8.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-8.3-7` | If no Destination Cache entry exists for the destination, an implementation SHOULD create such an entry. (§8.3) | SHOULD | 8.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-4.2-5` | A router MAY omit this option in order to enable inbound load sharing across multiple link-layer addresses. (§4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-4.2-7` | MAY be sent on other links. (§4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.2.1-10` | Implementations MAY allow AdvValidLifetime to be specified in two ways: - a time that decrements in real time, that is, one that will result in a Lifetime of zero at the specified time in the future, or - a fixed time that stays the same in consecutive advertisements. (§6.2.1) | MAY | 6.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.2.1-11` | Implementations MAY allow AdvPreferredLifetime to be specified in two ways: - a time that decrements in real time, that is, one that will result in a Lifetime of zero at a specified time in the future, or - a fixed time that stays the same in consecutive advertisements. (§6.2.1) | MAY | 6.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.2.3-1` | This option MAY be omitted to facilitate in-bound load balancing over replicated interfaces. o MTU option: the interface's configured AdvLinkMTU value if the value is non-zero. (§6.2.3) | MAY | 6.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.2.3-2` | A router MAY choose not to include some or all options when sending unsolicited Router Advertisements. (§6.2.3) | MAY | 6.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.2.4-3` | In such cases, the router MAY transmit up to MAX_INITIAL_RTR_ADVERTISEMENTS unsolicited advertisements, using the same rules as when an interface becomes an advertising interface. (§6.2.4) | MAY | 6.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.2.6-8` | A router MAY choose to unicast the response directly to the soliciting host's address (if the solicitation's source address is not the unspecified address), but the usual case is to multicast the response to the all-nodes group. (§6.2.6) | MAY | 6.2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.3.4-9` | To limit the storage needed for the Default Router List, a host MAY choose not to store all of the router addresses discovered via advertisements. (§6.3.4) | MAY | 6.3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.3.7-5` | In some cases, the random delay MAY be omitted if necessary. (§6.3.7) | MAY | 6.3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-6.3.7-6` | Hence, if a mobile node received link-layer information indicating that movement might have taken place, it MAY send a Router Solicitation immediately, without random delays. (§6.3.7) | MAY | 6.3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.2.2-8` | On unicast solicitations, an implementation MAY omit the Source Link-Layer Address option. (§7.2.2) | MAY | 7.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.2.4-5` | If the solicitation's IP Destination Address is not a multicast address, the Target Link-Layer Address option MAY be omitted; the neighboring node's cached value must already be current in order for the solicitation to have been received. (§7.2.4) | MAY | 7.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.2.6-4` | In such cases, a node MAY send up to MAX_NEIGHBOR_ADVERTISEMENT unsolicited Neighbor Advertisement messages to the all-nodes multicast address. (§7.2.6) | MAY | 7.2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.2.6-5` | The Override flag MAY be set to either zero or one. (§7.2.6) | MAY | 7.2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.2.6-6` | A node that has multiple IP addresses assigned to an interface MAY multicast a separate Neighbor Advertisement for each address. (§7.2.6) | MAY | 7.2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.2.6-8` | A proxy MAY multicast Neighbor Advertisements when its link-layer address changes or when it is configured (by system management or other mechanisms) to proxy for an address. (§7.2.6) | MAY | 7.2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.2.6-9` | Also, a node belonging to an anycast address MAY multicast unsolicited Neighbor Advertisements for the anycast address when the node's link-layer address changes. (§7.2.6) | MAY | 7.2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.2.8-3` | Under limited circumstances, a router MAY proxy for one or more other nodes, that is, through Neighbor Advertisements indicate that it is willing to accept packets not explicitly addressed to itself. (§7.2.8) | MAY | 7.2.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-7.2.8-5` | A proxy MAY send unsolicited advertisements with the Override flag set to one as specified in Section 7.2.6, but doing so may cause the proxy advertisement to override a valid entry created by the node itself. (§7.2.8) | MAY | 7.2.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC4861-9-9` | For example, an implementation can choose to process the Prefix Information option contained in a Router Advertisement message in a user-space process while the link-layer address option in the same message is processed by routines in the kernel. 2) Should the number of options cause a packet to exceed a link's MTU, multiple packets can carry subsets of the options without any change in semantics. 3) Senders MAY send a subset of options in different packets. (§9) | MAY | 9 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4861-2.3-1`](#rfc4861-2.3-1) All interfaces on routers MUST have a link-local address. (§2.3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 address configuration (net/ipv6/addrconf.c, addrconf_dev_config and addrconf_addr_gen) gives every interface a link-local address when it comes up; internal/plugins/iface/netlink/manage_linux.go::SetAdminUp brings the interface up and Ze leaves the address generation mode at its default, the one exception being the VRRP virtual-MAC macvlan, which carries the virtual link-local instead because RFC 9568 Section 7.4 forbids deriving an identifier from the virtual MAC |
| [`RFC4861-4.1-2`](#rfc4861-4.1-2) MUST NOT be included if the Source Address is the unspecified address. (§4.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-4.2-3`](#rfc4861-4.2-3) Receivers MUST silently ignore any options they do not recognize and continue processing the message. (§4.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-4.3-1`](#rfc4861-4.3-1) It MUST be initialized to zero by the sender and MUST be ignored by the receiver. (§4.3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-4.3-2`](#rfc4861-4.3-2) It MUST NOT be a multicast address. (§4.3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-4.3-3`](#rfc4861-4.3-3) MUST NOT be included when the source IP address is the unspecified address. (§4.3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-4.3-4`](#rfc4861-4.3-4) Otherwise, on link layers that have addresses this option MUST be included in multicast solicitations and SHOULD be included in unicast solicitations. (§4.3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-4.3-5`](#rfc4861-4.3-5) Receivers MUST silently ignore any options they do not recognize and continue processing the message. (§4.3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-4.4-3`](#rfc4861-4.4-3) The Target Address MUST NOT be a multicast address. (§4.4) | {gap}, no test | internal/plugins/vrrp/groups.go requires only the first IPv6 virtual-address to be link-local and refuses no multicast address after it, and internal/plugins/vrrp/transport/na.go::BuildNA copies each virtual address into the Target Address unchecked, so a configured multicast virtual address becomes the target of an unsolicited Neighbor Advertisement |
| [`RFC4861-4.4-4`](#rfc4861-4.4-4) This option MUST be included on link layers that have addresses when responding to multicast solicitations. (§4.4) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-4.4-5`](#rfc4861-4.4-5) The option MUST be included for multicast solicitations in order to avoid infinite Neighbor Solicitation "recursion" when the peer node does not have a cache entry to return a Neighbor Advertisements message. (§4.4) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-4.4-6`](#rfc4861-4.4-6) Receivers MUST silently ignore any options they do not recognize and continue processing the message. (§4.4) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-4.5-1`](#rfc4861-4.5-1) Source Address MUST be the link-local address assigned to the interface from which this message is sent. (§4.5) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-4.5-2`](#rfc4861-4.5-2) It MUST be initialized to zero by the sender and MUST be ignored by the receiver. (§4.5) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-4.5-3`](#rfc4861-4.5-3) When the target is the actual endpoint of communication, i.e., the destination is a neighbor, the Target Address field MUST contain the same value as the ICMP Destination Address field. (§4.5) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-4.5-4`](#rfc4861-4.5-4) Otherwise, the target is a better first-hop router and the Target Address MUST be the router's link-local address so that hosts can uniquely identify routers. (§4.5) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-4.5-5`](#rfc4861-4.5-5) In such cases, the option MUST be included in Redirect messages. (§4.5) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-4.6-1`](#rfc4861-4.6-1) Nodes MUST silently discard an ND packet that contains an option with length zero. (§4.6) | {gap}, no test | both Router Solicitation readers count every packet the ICMPv6 type filter passes as a solicitation and check none of Hop Limit 255, Code 0, ICMP length or option lengths; internal/plugins/iface/ra/sender_linux.go::readSolicitations, internal/component/l2tp/ppp/ra_linux.go::rsReaderLoop |
| [`RFC4861-4.6.1-1`](#rfc4861-4.6.1-1) These options MUST be silently ignored for other Neighbor Discovery messages. (§4.6.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-4.6.2-1`](#rfc4861-4.6.2-1) In other words, if the L flag is not set a host MUST NOT conclude that an address derived from the prefix is off-link. (§4.6.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-4.6.2-2`](#rfc4861-4.6.2-2) That is, it MUST NOT update a previous indication that the address is on-link. (§4.6.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-4.6.2-7`](#rfc4861-4.6.2-7) The Prefix Information option appears in Router Advertisement packets and MUST be silently ignored for other messages. (§4.6.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-4.6.3-1`](#rfc4861-4.6.3-1) They MUST be initialized to zero by the sender and MUST be ignored by the receiver. (§4.6.3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-4.6.3-2`](#rfc4861-4.6.3-2) This option MUST be silently ignored for other Neighbor Discovery messages. (§4.6.3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-4.6.4-1`](#rfc4861-4.6.4-1) It MUST be initialized to zero by the sender and MUST be ignored by the receiver. (§4.6.4) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-4.6.4-2`](#rfc4861-4.6.4-2) This option MUST be silently ignored for other Neighbor Discovery messages. (§4.6.4) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-6.1.1-1`](#rfc4861-6.1.1-1) Hosts MUST silently discard any received Router Solicitation Messages. (§6.1.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-6.1.1-2`](#rfc4861-6.1.1-2) A router MUST silently discard any received Router Solicitation messages that do not satisfy all of the following validity checks: (§6.1.1) | {gap}, no test | both Router Solicitation readers count every packet the ICMPv6 type filter passes as a solicitation and check none of Hop Limit 255, Code 0, ICMP length or option lengths; internal/plugins/iface/ra/sender_linux.go::readSolicitations, internal/component/l2tp/ppp/ra_linux.go::rsReaderLoop |
| [`RFC4861-6.1.2-1`](#rfc4861-6.1.2-1) A node MUST silently discard any received Router Advertisement messages that do not satisfy all of the following validity checks: (§6.1.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-6.1.2-2`](#rfc4861-6.1.2-2) The contents of the Reserved field, and of any unrecognized options, MUST be ignored. (§6.1.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-6.1.2-3`](#rfc4861-6.1.2-3) The contents of any defined options that are not specified to be used with Router Advertisement messages MUST be ignored and the packet processed as normal. (§6.1.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-6.2.1-1`](#rfc4861-6.2.1-1) A router MUST allow for the following conceptual variables to be configured by system management. (§6.2.1) | {gap}, no test | the router-advertisement container (internal/component/iface/yang/ze-iface-conf.yang, internal/component/iface/config_ra.go::parseRAConfig) offers every Section 6.2.1 variable except AdvLinkMTU, so no MTU option can be configured or sent |
| [`RFC4861-6.2.1-8`](#rfc4861-6.2.1-8) However, external router behavior MUST be the same as host behavior with respect to these variables. (§6.2.1) | {gap}, no test | Ze advertises the configured reachable-time, retransmit-timer and hop-limit (internal/component/iface/config_ra.go) but never sets the advertising interface's own net.ipv6.neigh.<if>.base_reachable_time_ms, retrans_time_ms or net.ipv6.conf.<if>.hop_limit, so the router's own Neighbor Discovery keeps the kernel values whatever it advertises |
| [`RFC4861-6.2.4-1`](#rfc4861-6.2.4-1) A host MUST NOT send Router Advertisement messages at any time. (§6.2.4) | {gap}, no test | a unit with router-advertisement enabled advertises whatever the interface forwarding state; internal/plugins/iface/ra/doctor.go only reports forwarding off (design decision D-6), so a node acting as a host still sends Router Advertisements |
| [`RFC4861-6.2.5-1`](#rfc4861-6.2.5-1) In addition, the host MUST ensure that subsequent Neighbor Advertisement messages sent from the interface have the Router flag set to zero. (§6.2.5) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-6.2.5-2`](#rfc4861-6.2.5-2) In such cases, subsequent Router Advertisements MUST set the Router Lifetime field to zero. (§6.2.5) | {gap}, no test | no sender watches the forwarding state; when forwarding is turned off on an interface that stays an advertising interface, internal/plugins/iface/ra/sender_linux.go keeps advertising the configured Router Lifetime rather than zero |
| [`RFC4861-6.2.6-1`](#rfc4861-6.2.6-1) A host MUST silently discard any received Router Solicitation messages. (§6.2.6) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-6.2.6-5`](#rfc4861-6.2.6-5) Router Solicitations in which the Source Address is the unspecified address MUST NOT update the router's Neighbor Cache; solicitations with a proper source address update the Neighbor Cache as follows. (§6.2.6) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-6.2.6-6`](#rfc4861-6.2.6-6) If the router already has a Neighbor Cache entry for the solicitation's sender, the solicitation contains a Source Link-Layer Address option, and the received link-layer address differs from that already in the cache, then the link-layer address SHOULD be updated in the appropriate Neighbor Cache entry, and its reachability state MUST also be set to STALE. (§6.2.6) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-6.2.6-7`](#rfc4861-6.2.6-7) Whether or not a Source Link-Layer Address option is provided, if a Neighbor Cache entry for the solicitation's sender exists (or is created) the entry's IsRouter flag MUST be set to FALSE. (§6.2.6) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-6.3.4-1`](#rfc4861-6.3.4-1) Hosts accept the union of all received information; the receipt of a Router Advertisement MUST NOT invalidate all information received in a previous advertisement or from another source. (§6.3.4) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-6.3.4-2`](#rfc4861-6.3.4-2) In particular, a host MUST NOT interpret the unspecified value as meaning change back to the default value that was in use before the first Router Advertisement was received. (§6.3.4) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-6.3.4-3`](#rfc4861-6.3.4-3) However, a host MUST retain at least two router addresses and SHOULD retain more. (§6.3.4) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-6.3.4-4`](#rfc4861-6.3.4-4) If the advertisement contains a Source Link-Layer Address option, the link-layer address SHOULD be recorded in the Neighbor Cache entry for the router (creating an entry if necessary) and the IsRouter flag in the Neighbor Cache entry MUST be set to TRUE. (§6.3.4) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-6.3.4-5`](#rfc4861-6.3.4-5) If no Source Link-Layer Address is included, but a corresponding Neighbor Cache entry exists, its IsRouter flag MUST be set to TRUE. (§6.3.4) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-6.3.4-6`](#rfc4861-6.3.4-6) If a Neighbor Cache entry is created for the router, its reachability state MUST be set to STALE as specified in Section 7.3.3. (§6.3.4) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-6.3.4-7`](#rfc4861-6.3.4-7) If a cache entry already exists and is updated with a different link-layer address, the reachability state MUST also be set to STALE. (§6.3.4) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-6.3.4-8`](#rfc4861-6.3.4-8) Note, however, that a Prefix Information option with the on-link flag set to zero conveys no information concerning on-link determination and MUST NOT be interpreted to mean that addresses covered by the prefix are off-link. (§6.3.4) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-6.3.5-1`](#rfc4861-6.3.5-1) When removing a router from the Default Router list, the node MUST update the Destination Cache in such a way that all entries using the router perform next-hop determination again rather than continue sending traffic to the (deleted) router. (§6.3.5) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-6.3.7-1`](#rfc4861-6.3.7-1) Once the host sends a Router Solicitation, and receives a valid Router Advertisement with a non-zero Router Lifetime, the host MUST desist from sending additional solicitations on that interface, until the next time one of the above events occurs. (§6.3.7) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.1.1-1`](#rfc4861-7.1.1-1) A node MUST silently discard any received Neighbor Solicitation messages that do not satisfy all of the following validity checks: (§7.1.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.1.1-2`](#rfc4861-7.1.1-2) The contents of the Reserved field, and of any unrecognized options, MUST be ignored. (§7.1.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.1.1-3`](#rfc4861-7.1.1-3) The contents of any defined options that are not specified to be used with Neighbor Solicitation messages MUST be ignored and the packet processed as normal. (§7.1.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.1.2-1`](#rfc4861-7.1.2-1) A node MUST silently discard any received Neighbor Advertisement messages that do not satisfy all of the following validity checks: (§7.1.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.1.2-2`](#rfc4861-7.1.2-2) The contents of the Reserved field, and of any unrecognized options, MUST be ignored. (§7.1.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.1.2-3`](#rfc4861-7.1.2-3) The contents of any defined options that are not specified to be used with Neighbor Advertisement messages MUST be ignored and the packet processed as normal. (§7.1.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.2-1`](#rfc4861-7.2-1) These messages MUST NOT create or update neighbor cache entries, except with respect to the IsRouter flag as specified in Sections 6.3.4 and 7.2.5. (§7.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.2.1-1`](#rfc4861-7.2.1-1) When a multicast-capable interface becomes enabled, the node MUST join the all-nodes multicast address on that interface, as well as the solicited-node multicast address corresponding to each of the IP addresses assigned to the interface. (§7.2.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.2.1-2`](#rfc4861-7.2.1-2) In such cases the node MUST join and leave the solicited-node multicast address corresponding to the new and old addresses, respectively. (§7.2.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.2.1-3`](#rfc4861-7.2.1-3) Note that multiple unicast addresses may map into the same solicited-node multicast address; a node MUST NOT leave the solicited-node multicast group until all assigned addresses corresponding to that multicast address have been removed. (§7.2.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.2.2-1`](#rfc4861-7.2.2-1) If the solicitation is being sent to a solicited-node multicast address, the sender MUST include its link-layer address (if it has one) as a Source Link-Layer Address option. (§7.2.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.2.2-2`](#rfc4861-7.2.2-2) While waiting for address resolution to complete, the sender MUST, for each neighbor, retain a small queue of packets waiting for address resolution to complete. (§7.2.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.2.2-3`](#rfc4861-7.2.2-3) The queue MUST hold at least one packet, and MAY contain more. (§7.2.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.2.2-4`](#rfc4861-7.2.2-4) Retransmissions MUST be rate-limited to at most one solicitation per neighbor every RetransTimer milliseconds. (§7.2.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.2.2-5`](#rfc4861-7.2.2-5) The sender MUST return ICMP destination unreachable indications with code 3 (Address Unreachable) for each packet queued awaiting address resolution. (§7.2.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.2.3-1`](#rfc4861-7.2.3-1) A valid Neighbor Solicitation that does not meet any of the following requirements MUST be silently discarded: (§7.2.3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.2.3-2`](#rfc4861-7.2.3-2) If an entry already exists, and the cached link-layer address differs from the one in the received Source Link-Layer option, the cached address should be replaced by the received address, and the entry's reachability state MUST be set to STALE. (§7.2.3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.2.3-3`](#rfc4861-7.2.3-3) If a Neighbor Cache entry already exists, its IsRouter flag MUST NOT be modified. (§7.2.3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.2.3-4`](#rfc4861-7.2.3-4) If the Source Address is the unspecified address, the node MUST NOT create or update the Neighbor Cache entry. (§7.2.3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.2.4-1`](#rfc4861-7.2.4-1) If the solicitation's IP Destination Address is a multicast address, the Target Link-Layer option MUST be included in the advertisement. (§7.2.4) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.2.4-2`](#rfc4861-7.2.4-2) Furthermore, if the node is a router, it MUST set the Router flag to one; otherwise, it MUST set the flag to zero. (§7.2.4) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.2.4-3`](#rfc4861-7.2.4-3) If the source of the solicitation is the unspecified address, the node MUST set the Solicited flag to zero and multicast the advertisement to the all-nodes address. (§7.2.4) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.2.4-4`](#rfc4861-7.2.4-4) Otherwise, the node MUST set the Solicited flag to one and unicast the advertisement to the Source Address of the solicitation. (§7.2.4) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.2.5-1`](#rfc4861-7.2.5-1) Otherwise, the received advertisement should be ignored and MUST NOT update the cache. (§7.2.5) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.2.5-2`](#rfc4861-7.2.5-2) If the Override flag is set, or the supplied link-layer address is the same as that in the cache, or no Target Link-Layer Address option was supplied, the received advertisement MUST update the Neighbor Cache entry as follows: (§7.2.5) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.2.5-3`](#rfc4861-7.2.5-3) - The link-layer address in the Target Link-Layer Address option MUST be inserted in the cache (if one is supplied and differs from the already recorded address). (§7.2.5) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.2.5-4`](#rfc4861-7.2.5-4) - If the Solicited flag is set, the state of the entry MUST be set to REACHABLE. (§7.2.5) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.2.5-5`](#rfc4861-7.2.5-5) If the Solicited flag is zero and the link- layer address was updated with a different address, the state MUST be set to STALE. (§7.2.5) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.2.5-6`](#rfc4861-7.2.5-6) - The IsRouter flag in the cache entry MUST be set based on the Router flag in the received advertisement. (§7.2.5) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.2.5-7`](#rfc4861-7.2.5-7) In those cases where the IsRouter flag changes from TRUE to FALSE as a result of this update, the node MUST remove that router from the Default Router List and update the Destination Cache entries for all destinations using that neighbor as a router as specified in Section 7.3.3. (§7.2.5) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.2.6-1`](#rfc4861-7.2.6-1) These advertisements MUST be separated by at least RetransTimer seconds. (§7.2.6) | {gap}, no test | internal/plugins/vrrp/transport/announce.go sends each unsolicited Neighbor Advertisement announceRepeatCount (3) times announceRepeatInterval (100 ms) apart, below RetransTimer (RETRANS_TIMER is 1,000 ms) |
| [`RFC4861-7.2.8-1`](#rfc4861-7.2.8-1) A proxy MUST join the solicited-node multicast address(es) that correspond to the IP address(es) assigned to the node for which it is proxying. (§7.2.8) | {gap}, no test | Ze offers no Neighbor Discovery proxy: no Ze code answers a Neighbor Solicitation or joins a solicited-node group on another node's behalf, and none sets the proxy_ndp sysctl or adds a proxy neighbour entry; no owner ruling names this feature unwanted (ai/META-DECISIONS.md, Goal and scope) |
| [`RFC4861-7.2.8-2`](#rfc4861-7.2.8-2) All solicited proxy Neighbor Advertisement messages MUST have the Override flag set to zero. (§7.2.8) | {gap}, no test | Ze offers no Neighbor Discovery proxy: no Ze code answers a Neighbor Solicitation or joins a solicited-node group on another node's behalf, and none sets the proxy_ndp sysctl or adds a proxy neighbour entry; no owner ruling names this feature unwanted (ai/META-DECISIONS.md, Goal and scope) |
| [`RFC4861-7.3.1-1`](#rfc4861-7.3.1-1) Receipt of other Neighbor Discovery messages, such as Router Advertisements and Neighbor Advertisement with the Solicited flag set to zero, MUST NOT be treated as a reachability confirmation. (§7.3.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.3.3-1`](#rfc4861-7.3.3-1) A node MUST NOT send Neighbor Solicitations to the same neighbor more frequently than once every RetransTimer milliseconds. (§7.3.3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.3.3-2`](#rfc4861-7.3.3-2) To properly detect the case where a router switches from being a router to being a host (e.g., if its IP forwarding capability is turned off by system management), a node MUST compare the Router flag field in all received Neighbor Advertisement messages with the IsRouter flag recorded in the Neighbor Cache entry. (§7.3.3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.3.3-3`](#rfc4861-7.3.3-3) When a node detects that a neighbor has changed from being a router to being a host, the node MUST remove that router from the Default Router List and update the Destination Cache as described in Section 6.3.5. (§7.3.3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-7.3.3-4`](#rfc4861-7.3.3-4) However, link-specific information MUST NOT be used to confirm the reachability of a neighbor; such information does not provide end-to-end confirmation between neighboring IP layers. (§7.3.3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-8-1`](#rfc4861-8-1) A router MUST be able to determine the link-local address for each of its neighboring routers in order to ensure that the target address in a Redirect message identifies the neighbor router by its link-local address. (§8) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-8.1-1`](#rfc4861-8.1-1) A host MUST silently discard any received Redirect message that does not satisfy all of the following validity checks: (§8.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-8.1-2`](#rfc4861-8.1-2) The contents of the Reserved field, and of any unrecognized options, MUST be ignored. (§8.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-8.1-3`](#rfc4861-8.1-3) The contents of any defined options that are not specified to be used with Redirect messages MUST be ignored and the packet processed as normal. (§8.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-8.1-4`](#rfc4861-8.1-4) A host MUST NOT consider a redirect invalid just because the Target Address of the redirect is not covered under one of the link's prefixes. (§8.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-8.2-1`](#rfc4861-8.2-1) If the target is a router, that router's link-local address MUST be used. (§8.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-8.2-2`](#rfc4861-8.2-2) If the target is a host, the target address field MUST be set to the same value as the Destination Address field. (§8.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-8.2-3`](#rfc4861-8.2-3) A router MUST limit the rate at which Redirect messages are sent, in order to limit the bandwidth and processing costs incurred by the Redirect messages when the source does not correctly respond to the Redirects, or the source chooses to ignore unauthenticated Redirect messages. (§8.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-8.2-4`](#rfc4861-8.2-4) A router MUST NOT update its routing tables upon receipt of a Redirect. (§8.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-8.3-1`](#rfc4861-8.3-1) If a Neighbor Cache entry is created for the target, its reachability state MUST be set to STALE as specified in Section 7.3.3. (§8.3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-8.3-2`](#rfc4861-8.3-2) If a cache entry already existed and it is updated with a different link-layer address, its reachability state MUST also be set to STALE. (§8.3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-8.3-3`](#rfc4861-8.3-3) If the Target and Destination Addresses are the same, the host MUST treat the Target as on-link. (§8.3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-8.3-4`](#rfc4861-8.3-4) If the Target Address is not the same as the Destination Address, the host MUST set IsRouter to TRUE for the target. (§8.3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-8.3-5`](#rfc4861-8.3-5) A host MUST NOT send Redirect messages. (§8.3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-9-2`](#rfc4861-9-2) All options specified in this document MUST be recognized. (§9) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-9-6`](#rfc4861-9-6) The amount of data to include in the Redirected Header option MUST be limited so that the entire redirect packet does not exceed the minimum MTU required to support IPv6 as specified in [IPv6]. (§9) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |
| [`RFC4861-9-7`](#rfc4861-9-7) When adding options to an ND packet, a node MUST NOT exceed the link MTU. (§9) | {gap}, no test | the router-advertisement prefix list (internal/component/iface/yang/ze-iface-conf.yang) has no max-elements and internal/plugins/iface/ra/sender_linux.go sends every option in one advertisement, so about 40 prefixes make the advertisement larger than a 1500 octet link MTU, and Ze never splits it |
| [`RFC4861-11.2-1`](#rfc4861-11.2-1) In any case, if either [IPv6-AUTH] or [IPv6-ESP] is used, ND packets MUST be verified for the purpose of authentication. (§11.2) | {gap}, no test | Ze has no configuration that secures Neighbor Discovery with AH or ESP, so nothing Ze sets up verifies ND packets for authentication or discards those that fail it; no owner ruling names this feature unwanted (ai/META-DECISIONS.md, Goal and scope) |
| [`RFC4861-11.2-2`](#rfc4861-11.2-2) Packets that fail authentication checks MUST be silently discarded. (§11.2) | {gap}, no test | Ze has no configuration that secures Neighbor Discovery with AH or ESP, so nothing Ze sets up verifies ND packets for authentication or discards those that fail it; no owner ruling names this feature unwanted (ai/META-DECISIONS.md, Goal and scope) |
| [`RFC4861-D-1`](#rfc4861-d-1) But when there is no host vs. router information in the ND message, the receipt of the message MUST NOT cause a change to the IsRouter state. (§D) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 neighbour discovery (net/ipv6/ndisc.c, with the neighbour cache in net/core/neighbour.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the interface the kernel runs Neighbor Discovery on, and Ze writes no field of this message and holds no state this behaviour reads |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4861-2.3-1`](#rfc4861-2.3-1)

All interfaces on routers MUST have a link-local address. (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-2.3-1, so no unit is bound to it.

### [`RFC4861-4.1-1`](#rfc4861-4.1-1)

It MUST be initialized to zero by the sender and MUST be ignored by the receiver. (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4861SubscriberSolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc4861_solicitation_reader_linux_test.go#L55) | unit/verify | revert, verified |
| positive | [`TestRFC4861SolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/ra/rfc4861_solicitation_reader_linux_test.go#L53) | unit/verify | revert, verified |

### [`RFC4861-4.1-2`](#rfc4861-4.1-2)

MUST NOT be included if the Source Address is the unspecified address. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-4.1-2, so no unit is bound to it.

### [`RFC4861-4.1-3`](#rfc4861-4.1-3)

Receivers MUST silently ignore any options they do not recognize and continue processing the message. (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4861SubscriberSolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc4861_solicitation_reader_linux_test.go#L56) | unit/verify | revert, verified |
| positive | [`TestRFC4861SolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/ra/rfc4861_solicitation_reader_linux_test.go#L54) | unit/verify | revert, verified |

### [`RFC4861-4.2-1`](#rfc4861-4.2-1)

Source Address MUST be the link-local address assigned to the interface from which this message is sent. (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRASenderWireFormat`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/ra/rfc4861_ra_integration_linux_test.go#L31) | unit/verify | revert, verified |

### [`RFC4861-4.2-2`](#rfc4861-4.2-2)

It MUST be initialized to zero by the sender and MUST be ignored by the receiver. (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4861RouterAdvertisementReservedFieldsAreZero`](https://github.com/ze-software/ze/blob/main/internal/core/ndp/rfc4861_router_advertisement_test.go#L40) | unit/verify | revert, verified |
| positive | [`TestRFC4861RouterAdvertisementReservedFieldsAreZero`](https://github.com/ze-software/ze/blob/main/internal/core/ndp/rfc4861_router_advertisement_test.go#L30) | unit/verify | revert, verified |

### [`RFC4861-4.2-3`](#rfc4861-4.2-3)

Receivers MUST silently ignore any options they do not recognize and continue processing the message. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-4.2-3, so no unit is bound to it.

### [`RFC4861-4.3-1`](#rfc4861-4.3-1)

It MUST be initialized to zero by the sender and MUST be ignored by the receiver. (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-4.3-1, so no unit is bound to it.

### [`RFC4861-4.3-2`](#rfc4861-4.3-2)

It MUST NOT be a multicast address. (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-4.3-2, so no unit is bound to it.

### [`RFC4861-4.3-3`](#rfc4861-4.3-3)

MUST NOT be included when the source IP address is the unspecified address. (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-4.3-3, so no unit is bound to it.

### [`RFC4861-4.3-4`](#rfc4861-4.3-4)

Otherwise, on link layers that have addresses this option MUST be included in multicast solicitations and SHOULD be included in unicast solicitations. (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-4.3-4, so no unit is bound to it.

### [`RFC4861-4.3-5`](#rfc4861-4.3-5)

Receivers MUST silently ignore any options they do not recognize and continue processing the message. (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-4.3-5, so no unit is bound to it.

### [`RFC4861-4.4-1`](#rfc4861-4.4-1)

It MUST NOT be set in multicast advertisements or in unsolicited unicast advertisements. (§4.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4861UnsolicitedNAClearsTheSolicitedFlag`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc4861_unsolicited_na_test.go#L33) | unit/verify | revert, verified |

### [`RFC4861-4.4-2`](#rfc4861-4.4-2)

It MUST be initialized to zero by the sender and MUST be ignored by the receiver. (§4.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4861UnsolicitedNAReservedBitsAreZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc4861_unsolicited_na_test.go#L52) | unit/verify | revert, verified |

### [`RFC4861-4.4-3`](#rfc4861-4.4-3)

The Target Address MUST NOT be a multicast address. (§4.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-4.4-3, so no unit is bound to it.

### [`RFC4861-4.4-4`](#rfc4861-4.4-4)

This option MUST be included on link layers that have addresses when responding to multicast solicitations. (§4.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-4.4-4, so no unit is bound to it.

### [`RFC4861-4.4-5`](#rfc4861-4.4-5)

The option MUST be included for multicast solicitations in order to avoid infinite Neighbor Solicitation "recursion" when the peer node does not have a cache entry to return a Neighbor Advertisements message. (§4.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-4.4-5, so no unit is bound to it.

### [`RFC4861-4.4-6`](#rfc4861-4.4-6)

Receivers MUST silently ignore any options they do not recognize and continue processing the message. (§4.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-4.4-6, so no unit is bound to it.

### [`RFC4861-4.5-1`](#rfc4861-4.5-1)

Source Address MUST be the link-local address assigned to the interface from which this message is sent. (§4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-4.5-1, so no unit is bound to it.

### [`RFC4861-4.5-2`](#rfc4861-4.5-2)

It MUST be initialized to zero by the sender and MUST be ignored by the receiver. (§4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-4.5-2, so no unit is bound to it.

### [`RFC4861-4.5-3`](#rfc4861-4.5-3)

When the target is the actual endpoint of communication, i.e., the destination is a neighbor, the Target Address field MUST contain the same value as the ICMP Destination Address field. (§4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-4.5-3, so no unit is bound to it.

### [`RFC4861-4.5-4`](#rfc4861-4.5-4)

Otherwise, the target is a better first-hop router and the Target Address MUST be the router's link-local address so that hosts can uniquely identify routers. (§4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-4.5-4, so no unit is bound to it.

### [`RFC4861-4.5-5`](#rfc4861-4.5-5)

In such cases, the option MUST be included in Redirect messages. (§4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-4.5-5, so no unit is bound to it.

### [`RFC4861-4.6-1`](#rfc4861-4.6-1)

Nodes MUST silently discard an ND packet that contains an option with length zero. (§4.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-4.6-1, so no unit is bound to it.

### [`RFC4861-4.6.1-1`](#rfc4861-4.6.1-1)

These options MUST be silently ignored for other Neighbor Discovery messages. (§4.6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-4.6.1-1, so no unit is bound to it.

### [`RFC4861-4.6.2-1`](#rfc4861-4.6.2-1)

In other words, if the L flag is not set a host MUST NOT conclude that an address derived from the prefix is off-link. (§4.6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-4.6.2-1, so no unit is bound to it.

### [`RFC4861-4.6.2-2`](#rfc4861-4.6.2-2)

That is, it MUST NOT update a previous indication that the address is on-link. (§4.6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-4.6.2-2, so no unit is bound to it.

### [`RFC4861-4.6.2-3`](#rfc4861-4.6.2-3)

It MUST be initialized to zero by the sender and MUST be ignored by the receiver. (§4.6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4861RouterAdvertisementReservedFieldsAreZero`](https://github.com/ze-software/ze/blob/main/internal/core/ndp/rfc4861_router_advertisement_test.go#L41) | unit/verify | revert, verified |
| positive | [`TestRFC4861RouterAdvertisementReservedFieldsAreZero`](https://github.com/ze-software/ze/blob/main/internal/core/ndp/rfc4861_router_advertisement_test.go#L33) | unit/verify | revert, verified |

### [`RFC4861-4.6.2-4`](#rfc4861-4.6.2-4)

Note that the value of this field MUST NOT exceed the Valid Lifetime field to avoid preferring addresses that are no longer valid. (§4.6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4861PreferredLifetimeNotAboveValid`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L72) | unit/verify | revert, verified |
| positive | [`TestRFC4861PreferredLifetimeNotAboveValid`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L76) | unit/verify | revert, verified |

### [`RFC4861-4.6.2-5`](#rfc4861-4.6.2-5)

It MUST be initialized to zero by the sender and MUST be ignored by the receiver. (§4.6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4861RouterAdvertisementReservedFieldsAreZero`](https://github.com/ze-software/ze/blob/main/internal/core/ndp/rfc4861_router_advertisement_test.go#L36) | unit/verify | revert, verified |

### [`RFC4861-4.6.2-6`](#rfc4861-4.6.2-6)

The bits in the prefix after the prefix length are reserved and MUST be initialized to zero by the sender and ignored by the receiver. (§4.6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4861RouterAdvertisementReservedFieldsAreZero`](https://github.com/ze-software/ze/blob/main/internal/core/ndp/rfc4861_router_advertisement_test.go#L42) | unit/verify | revert, verified |
| positive | [`TestRFC4861RouterAdvertisementReservedFieldsAreZero`](https://github.com/ze-software/ze/blob/main/internal/core/ndp/rfc4861_router_advertisement_test.go#L38) | unit/verify | revert, verified |

### [`RFC4861-4.6.2-7`](#rfc4861-4.6.2-7)

The Prefix Information option appears in Router Advertisement packets and MUST be silently ignored for other messages. (§4.6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-4.6.2-7, so no unit is bound to it.

### [`RFC4861-4.6.3-1`](#rfc4861-4.6.3-1)

They MUST be initialized to zero by the sender and MUST be ignored by the receiver. (§4.6.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-4.6.3-1, so no unit is bound to it.

### [`RFC4861-4.6.3-2`](#rfc4861-4.6.3-2)

This option MUST be silently ignored for other Neighbor Discovery messages. (§4.6.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-4.6.3-2, so no unit is bound to it.

### [`RFC4861-4.6.4-1`](#rfc4861-4.6.4-1)

It MUST be initialized to zero by the sender and MUST be ignored by the receiver. (§4.6.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-4.6.4-1, so no unit is bound to it.

### [`RFC4861-4.6.4-2`](#rfc4861-4.6.4-2)

This option MUST be silently ignored for other Neighbor Discovery messages. (§4.6.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-4.6.4-2, so no unit is bound to it.

### [`RFC4861-6.1.1-1`](#rfc4861-6.1.1-1)

Hosts MUST silently discard any received Router Solicitation Messages. (§6.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-6.1.1-1, so no unit is bound to it.

### [`RFC4861-6.1.1-2`](#rfc4861-6.1.1-2)

A router MUST silently discard any received Router Solicitation messages that do not satisfy all of the following validity checks: (§6.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-6.1.1-2, so no unit is bound to it.

### [`RFC4861-6.1.1-3`](#rfc4861-6.1.1-3)

The contents of the Reserved field, and of any unrecognized options, MUST be ignored. (§6.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4861SubscriberSolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc4861_solicitation_reader_linux_test.go#L57) | unit/verify | revert, verified |
| positive | [`TestRFC4861SolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/ra/rfc4861_solicitation_reader_linux_test.go#L55) | unit/verify | revert, verified |

### [`RFC4861-6.1.1-4`](#rfc4861-6.1.1-4)

The contents of any defined options that are not specified to be used with Router Solicitation messages MUST be ignored and the packet processed as normal. (§6.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4861SubscriberSolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc4861_solicitation_reader_linux_test.go#L58) | unit/verify | revert, verified |
| positive | [`TestRFC4861SolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/ra/rfc4861_solicitation_reader_linux_test.go#L56) | unit/verify | revert, verified |

### [`RFC4861-6.1.2-1`](#rfc4861-6.1.2-1)

A node MUST silently discard any received Router Advertisement messages that do not satisfy all of the following validity checks: (§6.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-6.1.2-1, so no unit is bound to it.

### [`RFC4861-6.1.2-2`](#rfc4861-6.1.2-2)

The contents of the Reserved field, and of any unrecognized options, MUST be ignored. (§6.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-6.1.2-2, so no unit is bound to it.

### [`RFC4861-6.1.2-3`](#rfc4861-6.1.2-3)

The contents of any defined options that are not specified to be used with Router Advertisement messages MUST be ignored and the packet processed as normal. (§6.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-6.1.2-3, so no unit is bound to it.

### [`RFC4861-6.2.1-1`](#rfc4861-6.2.1-1)

A router MUST allow for the following conceptual variables to be configured by system management. (§6.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-6.2.1-1, so no unit is bound to it.

### [`RFC4861-6.2.1-2`](#rfc4861-6.2.1-2)

Note that AdvSendAdvertisements MUST be FALSE by default so that a node will not accidentally start acting as a router unless it is explicitly configured by system management to send Router Advertisements. (§6.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4861AdvSendAdvertisementsIsFalseByDefault`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L82) | unit/verify | revert, verified |

### [`RFC4861-6.2.1-3`](#rfc4861-6.2.1-3)

MUST be no less than 4 seconds and no greater than 1800 seconds. (§6.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4861MaxRtrAdvIntervalBounds`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L27) | unit/verify | revert, verified |
| positive | [`TestRFC4861MaxRtrAdvIntervalBounds`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L30) | unit/verify | revert, verified |

### [`RFC4861-6.2.1-4`](#rfc4861-6.2.1-4)

MUST be no less than 3 seconds and no greater than .75 * MaxRtrAdvInterval. (§6.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4861MinRtrAdvIntervalBounds`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L38) | unit/verify | revert, verified |
| positive | [`TestRFC4861MinRtrAdvIntervalBounds`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L41) | unit/verify | revert, verified |

### [`RFC4861-6.2.1-5`](#rfc4861-6.2.1-5)

MUST be no greater than 3,600,000 milliseconds (1 hour). (§6.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4861AdvReachableTimeBound`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L49) | unit/verify | revert, verified |
| positive | [`TestRFC4861AdvReachableTimeBound`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L51) | unit/verify | revert, verified |

### [`RFC4861-6.2.1-6`](#rfc4861-6.2.1-6)

MUST be either zero or between MaxRtrAdvInterval and 9000 seconds. (§6.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4861AdvDefaultLifetimeBounds`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L57) | unit/verify | revert, verified |
| positive | [`TestRFC4861AdvDefaultLifetimeBounds`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L60) | unit/verify | revert, verified |

### [`RFC4861-6.2.1-7`](#rfc4861-6.2.1-7)

This value MUST NOT be larger than AdvValidLifetime. (§6.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4861PreferredLifetimeNotAboveValid`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L69) | unit/verify | revert, verified |
| positive | [`TestRFC4861PreferredLifetimeNotAboveValid`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L75) | unit/verify | revert, verified |

### [`RFC4861-6.2.1-8`](#rfc4861-6.2.1-8)

However, external router behavior MUST be the same as host behavior with respect to these variables. (§6.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-6.2.1-8, so no unit is bound to it.

### [`RFC4861-6.2.2-1`](#rfc4861-6.2.2-1)

A router MUST NOT send Router Advertisements out any interface that is not an advertising interface. (§6.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4861OnlyAnAdvertisingInterfaceSends`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L97) | unit/verify | revert, verified |
| positive | [`TestRFC4861OnlyAnAdvertisingInterfaceSends`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L100) | unit/verify | revert, verified |

### [`RFC4861-6.2.2-2`](#rfc4861-6.2.2-2)

A router MUST join the all-routers multicast address on an advertising interface. (§6.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRASolicitedResponse`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/ra/rfc4861_ra_integration_linux_test.go#L130) | unit/verify | mutant, verified |

### [`RFC4861-6.2.4-1`](#rfc4861-6.2.4-1)

A host MUST NOT send Router Advertisement messages at any time. (§6.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-6.2.4-1, so no unit is bound to it.

### [`RFC4861-6.2.5-1`](#rfc4861-6.2.5-1)

In addition, the host MUST ensure that subsequent Neighbor Advertisement messages sent from the interface have the Router flag set to zero. (§6.2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-6.2.5-1, so no unit is bound to it.

### [`RFC4861-6.2.5-2`](#rfc4861-6.2.5-2)

In such cases, subsequent Router Advertisements MUST set the Router Lifetime field to zero. (§6.2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-6.2.5-2, so no unit is bound to it.

### [`RFC4861-6.2.6-1`](#rfc4861-6.2.6-1)

A host MUST silently discard any received Router Solicitation messages. (§6.2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-6.2.6-1, so no unit is bound to it.

### [`RFC4861-6.2.6-2`](#rfc4861-6.2.6-2)

In all cases, Router Advertisements sent in response to a Router Solicitation MUST be delayed by a random time between 0 and MAX_RA_DELAY_TIME seconds. (§6.2.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4861SolicitedAnswerDelayIsRandomUpToMaxRADelayTime`](https://github.com/ze-software/ze/blob/main/internal/core/ndp/rfc4861_router_advertisement_test.go#L80) | unit/verify | revert, verified |

### [`RFC4861-6.2.6-3`](#rfc4861-6.2.6-3)

(If a single advertisement is sent in response to multiple solicitations, the delay is relative to the first solicitation.) In addition, consecutive Router Advertisements sent to the all-nodes multicast address MUST be rate limited to no more than one advertisement every MIN_DELAY_BETWEEN_RAS seconds. (§6.2.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4861SolicitedAnswerIsRateLimited`](https://github.com/ze-software/ze/blob/main/internal/core/ndp/rfc4861_router_advertisement_test.go#L101) | unit/verify | revert, verified |

### [`RFC4861-6.2.6-4`](#rfc4861-6.2.6-4)

In all cases, however, unsolicited multicast advertisements MUST NOT be sent more frequently than indicated by MinRtrAdvInterval. (§6.2.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4861UnsolicitedIntervalIsNeverBelowMinRtrAdvInterval`](https://github.com/ze-software/ze/blob/main/internal/core/ndp/rfc4861_router_advertisement_test.go#L113) | unit/verify | revert, verified |

### [`RFC4861-6.2.6-5`](#rfc4861-6.2.6-5)

Router Solicitations in which the Source Address is the unspecified address MUST NOT update the router's Neighbor Cache; solicitations with a proper source address update the Neighbor Cache as follows. (§6.2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-6.2.6-5, so no unit is bound to it.

### [`RFC4861-6.2.6-6`](#rfc4861-6.2.6-6)

If the router already has a Neighbor Cache entry for the solicitation's sender, the solicitation contains a Source Link-Layer Address option, and the received link-layer address differs from that already in the cache, then the link-layer address SHOULD be updated in the appropriate Neighbor Cache entry, and its reachability state MUST also be set to STALE. (§6.2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-6.2.6-6, so no unit is bound to it.

### [`RFC4861-6.2.6-7`](#rfc4861-6.2.6-7)

Whether or not a Source Link-Layer Address option is provided, if a Neighbor Cache entry for the solicitation's sender exists (or is created) the entry's IsRouter flag MUST be set to FALSE. (§6.2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-6.2.6-7, so no unit is bound to it.

### [`RFC4861-6.3.4-1`](#rfc4861-6.3.4-1)

Hosts accept the union of all received information; the receipt of a Router Advertisement MUST NOT invalidate all information received in a previous advertisement or from another source. (§6.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-6.3.4-1, so no unit is bound to it.

### [`RFC4861-6.3.4-2`](#rfc4861-6.3.4-2)

In particular, a host MUST NOT interpret the unspecified value as meaning change back to the default value that was in use before the first Router Advertisement was received. (§6.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-6.3.4-2, so no unit is bound to it.

### [`RFC4861-6.3.4-3`](#rfc4861-6.3.4-3)

However, a host MUST retain at least two router addresses and SHOULD retain more. (§6.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-6.3.4-3, so no unit is bound to it.

### [`RFC4861-6.3.4-4`](#rfc4861-6.3.4-4)

If the advertisement contains a Source Link-Layer Address option, the link-layer address SHOULD be recorded in the Neighbor Cache entry for the router (creating an entry if necessary) and the IsRouter flag in the Neighbor Cache entry MUST be set to TRUE. (§6.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-6.3.4-4, so no unit is bound to it.

### [`RFC4861-6.3.4-5`](#rfc4861-6.3.4-5)

If no Source Link-Layer Address is included, but a corresponding Neighbor Cache entry exists, its IsRouter flag MUST be set to TRUE. (§6.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-6.3.4-5, so no unit is bound to it.

### [`RFC4861-6.3.4-6`](#rfc4861-6.3.4-6)

If a Neighbor Cache entry is created for the router, its reachability state MUST be set to STALE as specified in Section 7.3.3. (§6.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-6.3.4-6, so no unit is bound to it.

### [`RFC4861-6.3.4-7`](#rfc4861-6.3.4-7)

If a cache entry already exists and is updated with a different link-layer address, the reachability state MUST also be set to STALE. (§6.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-6.3.4-7, so no unit is bound to it.

### [`RFC4861-6.3.4-8`](#rfc4861-6.3.4-8)

Note, however, that a Prefix Information option with the on-link flag set to zero conveys no information concerning on-link determination and MUST NOT be interpreted to mean that addresses covered by the prefix are off-link. (§6.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-6.3.4-8, so no unit is bound to it.

### [`RFC4861-6.3.5-1`](#rfc4861-6.3.5-1)

When removing a router from the Default Router list, the node MUST update the Destination Cache in such a way that all entries using the router perform next-hop determination again rather than continue sending traffic to the (deleted) router. (§6.3.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-6.3.5-1, so no unit is bound to it.

### [`RFC4861-6.3.7-1`](#rfc4861-6.3.7-1)

Once the host sends a Router Solicitation, and receives a valid Router Advertisement with a non-zero Router Lifetime, the host MUST desist from sending additional solicitations on that interface, until the next time one of the above events occurs. (§6.3.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-6.3.7-1, so no unit is bound to it.

### [`RFC4861-7.1.1-1`](#rfc4861-7.1.1-1)

A node MUST silently discard any received Neighbor Solicitation messages that do not satisfy all of the following validity checks: (§7.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.1.1-1, so no unit is bound to it.

### [`RFC4861-7.1.1-2`](#rfc4861-7.1.1-2)

The contents of the Reserved field, and of any unrecognized options, MUST be ignored. (§7.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.1.1-2, so no unit is bound to it.

### [`RFC4861-7.1.1-3`](#rfc4861-7.1.1-3)

The contents of any defined options that are not specified to be used with Neighbor Solicitation messages MUST be ignored and the packet processed as normal. (§7.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.1.1-3, so no unit is bound to it.

### [`RFC4861-7.1.2-1`](#rfc4861-7.1.2-1)

A node MUST silently discard any received Neighbor Advertisement messages that do not satisfy all of the following validity checks: (§7.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.1.2-1, so no unit is bound to it.

### [`RFC4861-7.1.2-2`](#rfc4861-7.1.2-2)

The contents of the Reserved field, and of any unrecognized options, MUST be ignored. (§7.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.1.2-2, so no unit is bound to it.

### [`RFC4861-7.1.2-3`](#rfc4861-7.1.2-3)

The contents of any defined options that are not specified to be used with Neighbor Advertisement messages MUST be ignored and the packet processed as normal. (§7.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.1.2-3, so no unit is bound to it.

### [`RFC4861-7.2-1`](#rfc4861-7.2-1)

These messages MUST NOT create or update neighbor cache entries, except with respect to the IsRouter flag as specified in Sections 6.3.4 and 7.2.5. (§7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.2-1, so no unit is bound to it.

### [`RFC4861-7.2.1-1`](#rfc4861-7.2.1-1)

When a multicast-capable interface becomes enabled, the node MUST join the all-nodes multicast address on that interface, as well as the solicited-node multicast address corresponding to each of the IP addresses assigned to the interface. (§7.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.2.1-1, so no unit is bound to it.

### [`RFC4861-7.2.1-2`](#rfc4861-7.2.1-2)

In such cases the node MUST join and leave the solicited-node multicast address corresponding to the new and old addresses, respectively. (§7.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.2.1-2, so no unit is bound to it.

### [`RFC4861-7.2.1-3`](#rfc4861-7.2.1-3)

Note that multiple unicast addresses may map into the same solicited-node multicast address; a node MUST NOT leave the solicited-node multicast group until all assigned addresses corresponding to that multicast address have been removed. (§7.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.2.1-3, so no unit is bound to it.

### [`RFC4861-7.2.2-1`](#rfc4861-7.2.2-1)

If the solicitation is being sent to a solicited-node multicast address, the sender MUST include its link-layer address (if it has one) as a Source Link-Layer Address option. (§7.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.2.2-1, so no unit is bound to it.

### [`RFC4861-7.2.2-2`](#rfc4861-7.2.2-2)

While waiting for address resolution to complete, the sender MUST, for each neighbor, retain a small queue of packets waiting for address resolution to complete. (§7.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.2.2-2, so no unit is bound to it.

### [`RFC4861-7.2.2-3`](#rfc4861-7.2.2-3)

The queue MUST hold at least one packet, and MAY contain more. (§7.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.2.2-3, so no unit is bound to it.

### [`RFC4861-7.2.2-4`](#rfc4861-7.2.2-4)

Retransmissions MUST be rate-limited to at most one solicitation per neighbor every RetransTimer milliseconds. (§7.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.2.2-4, so no unit is bound to it.

### [`RFC4861-7.2.2-5`](#rfc4861-7.2.2-5)

The sender MUST return ICMP destination unreachable indications with code 3 (Address Unreachable) for each packet queued awaiting address resolution. (§7.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.2.2-5, so no unit is bound to it.

### [`RFC4861-7.2.3-1`](#rfc4861-7.2.3-1)

A valid Neighbor Solicitation that does not meet any of the following requirements MUST be silently discarded: (§7.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.2.3-1, so no unit is bound to it.

### [`RFC4861-7.2.3-2`](#rfc4861-7.2.3-2)

If an entry already exists, and the cached link-layer address differs from the one in the received Source Link-Layer option, the cached address should be replaced by the received address, and the entry's reachability state MUST be set to STALE. (§7.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.2.3-2, so no unit is bound to it.

### [`RFC4861-7.2.3-3`](#rfc4861-7.2.3-3)

If a Neighbor Cache entry already exists, its IsRouter flag MUST NOT be modified. (§7.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.2.3-3, so no unit is bound to it.

### [`RFC4861-7.2.3-4`](#rfc4861-7.2.3-4)

If the Source Address is the unspecified address, the node MUST NOT create or update the Neighbor Cache entry. (§7.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.2.3-4, so no unit is bound to it.

### [`RFC4861-7.2.4-1`](#rfc4861-7.2.4-1)

If the solicitation's IP Destination Address is a multicast address, the Target Link-Layer option MUST be included in the advertisement. (§7.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.2.4-1, so no unit is bound to it.

### [`RFC4861-7.2.4-2`](#rfc4861-7.2.4-2)

Furthermore, if the node is a router, it MUST set the Router flag to one; otherwise, it MUST set the flag to zero. (§7.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.2.4-2, so no unit is bound to it.

### [`RFC4861-7.2.4-3`](#rfc4861-7.2.4-3)

If the source of the solicitation is the unspecified address, the node MUST set the Solicited flag to zero and multicast the advertisement to the all-nodes address. (§7.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.2.4-3, so no unit is bound to it.

### [`RFC4861-7.2.4-4`](#rfc4861-7.2.4-4)

Otherwise, the node MUST set the Solicited flag to one and unicast the advertisement to the Source Address of the solicitation. (§7.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.2.4-4, so no unit is bound to it.

### [`RFC4861-7.2.5-1`](#rfc4861-7.2.5-1)

Otherwise, the received advertisement should be ignored and MUST NOT update the cache. (§7.2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.2.5-1, so no unit is bound to it.

### [`RFC4861-7.2.5-2`](#rfc4861-7.2.5-2)

If the Override flag is set, or the supplied link-layer address is the same as that in the cache, or no Target Link-Layer Address option was supplied, the received advertisement MUST update the Neighbor Cache entry as follows: (§7.2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.2.5-2, so no unit is bound to it.

### [`RFC4861-7.2.5-3`](#rfc4861-7.2.5-3)

- The link-layer address in the Target Link-Layer Address option MUST be inserted in the cache (if one is supplied and differs from the already recorded address). (§7.2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.2.5-3, so no unit is bound to it.

### [`RFC4861-7.2.5-4`](#rfc4861-7.2.5-4)

- If the Solicited flag is set, the state of the entry MUST be set to REACHABLE. (§7.2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.2.5-4, so no unit is bound to it.

### [`RFC4861-7.2.5-5`](#rfc4861-7.2.5-5)

If the Solicited flag is zero and the link- layer address was updated with a different address, the state MUST be set to STALE. (§7.2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.2.5-5, so no unit is bound to it.

### [`RFC4861-7.2.5-6`](#rfc4861-7.2.5-6)

- The IsRouter flag in the cache entry MUST be set based on the Router flag in the received advertisement. (§7.2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.2.5-6, so no unit is bound to it.

### [`RFC4861-7.2.5-7`](#rfc4861-7.2.5-7)

In those cases where the IsRouter flag changes from TRUE to FALSE as a result of this update, the node MUST remove that router from the Default Router List and update the Destination Cache entries for all destinations using that neighbor as a router as specified in Section 7.3.3. (§7.2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.2.5-7, so no unit is bound to it.

### [`RFC4861-7.2.6-1`](#rfc4861-7.2.6-1)

These advertisements MUST be separated by at least RetransTimer seconds. (§7.2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.2.6-1, so no unit is bound to it.

### [`RFC4861-7.2.6-2`](#rfc4861-7.2.6-2)

The Solicited flag MUST be set to zero, in order to avoid confusing the Neighbor Unreachability Detection algorithm. (§7.2.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4861UnsolicitedNAClearsTheSolicitedFlag`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc4861_unsolicited_na_test.go#L36) | unit/verify | revert, verified |

### [`RFC4861-7.2.6-3`](#rfc4861-7.2.6-3)

If the node is a router, it MUST set the Router flag to one; otherwise, it MUST set it to zero. (§7.2.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4861UnsolicitedNASetsTheRouterFlag`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc4861_unsolicited_na_test.go#L44) | unit/verify | revert, verified |

### [`RFC4861-7.2.8-1`](#rfc4861-7.2.8-1)

A proxy MUST join the solicited-node multicast address(es) that correspond to the IP address(es) assigned to the node for which it is proxying. (§7.2.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.2.8-1, so no unit is bound to it.

### [`RFC4861-7.2.8-2`](#rfc4861-7.2.8-2)

All solicited proxy Neighbor Advertisement messages MUST have the Override flag set to zero. (§7.2.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.2.8-2, so no unit is bound to it.

### [`RFC4861-7.3.1-1`](#rfc4861-7.3.1-1)

Receipt of other Neighbor Discovery messages, such as Router Advertisements and Neighbor Advertisement with the Solicited flag set to zero, MUST NOT be treated as a reachability confirmation. (§7.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.3.1-1, so no unit is bound to it.

### [`RFC4861-7.3.3-1`](#rfc4861-7.3.3-1)

A node MUST NOT send Neighbor Solicitations to the same neighbor more frequently than once every RetransTimer milliseconds. (§7.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.3.3-1, so no unit is bound to it.

### [`RFC4861-7.3.3-2`](#rfc4861-7.3.3-2)

To properly detect the case where a router switches from being a router to being a host (e.g., if its IP forwarding capability is turned off by system management), a node MUST compare the Router flag field in all received Neighbor Advertisement messages with the IsRouter flag recorded in the Neighbor Cache entry. (§7.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.3.3-2, so no unit is bound to it.

### [`RFC4861-7.3.3-3`](#rfc4861-7.3.3-3)

When a node detects that a neighbor has changed from being a router to being a host, the node MUST remove that router from the Default Router List and update the Destination Cache as described in Section 6.3.5. (§7.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.3.3-3, so no unit is bound to it.

### [`RFC4861-7.3.3-4`](#rfc4861-7.3.3-4)

However, link-specific information MUST NOT be used to confirm the reachability of a neighbor; such information does not provide end-to-end confirmation between neighboring IP layers. (§7.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-7.3.3-4, so no unit is bound to it.

### [`RFC4861-8-1`](#rfc4861-8-1)

A router MUST be able to determine the link-local address for each of its neighboring routers in order to ensure that the target address in a Redirect message identifies the neighbor router by its link-local address. (§8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-8-1, so no unit is bound to it.

### [`RFC4861-8.1-1`](#rfc4861-8.1-1)

A host MUST silently discard any received Redirect message that does not satisfy all of the following validity checks: (§8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-8.1-1, so no unit is bound to it.

### [`RFC4861-8.1-2`](#rfc4861-8.1-2)

The contents of the Reserved field, and of any unrecognized options, MUST be ignored. (§8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-8.1-2, so no unit is bound to it.

### [`RFC4861-8.1-3`](#rfc4861-8.1-3)

The contents of any defined options that are not specified to be used with Redirect messages MUST be ignored and the packet processed as normal. (§8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-8.1-3, so no unit is bound to it.

### [`RFC4861-8.1-4`](#rfc4861-8.1-4)

A host MUST NOT consider a redirect invalid just because the Target Address of the redirect is not covered under one of the link's prefixes. (§8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-8.1-4, so no unit is bound to it.

### [`RFC4861-8.2-1`](#rfc4861-8.2-1)

If the target is a router, that router's link-local address MUST be used. (§8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-8.2-1, so no unit is bound to it.

### [`RFC4861-8.2-2`](#rfc4861-8.2-2)

If the target is a host, the target address field MUST be set to the same value as the Destination Address field. (§8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-8.2-2, so no unit is bound to it.

### [`RFC4861-8.2-3`](#rfc4861-8.2-3)

A router MUST limit the rate at which Redirect messages are sent, in order to limit the bandwidth and processing costs incurred by the Redirect messages when the source does not correctly respond to the Redirects, or the source chooses to ignore unauthenticated Redirect messages. (§8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-8.2-3, so no unit is bound to it.

### [`RFC4861-8.2-4`](#rfc4861-8.2-4)

A router MUST NOT update its routing tables upon receipt of a Redirect. (§8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-8.2-4, so no unit is bound to it.

### [`RFC4861-8.3-1`](#rfc4861-8.3-1)

If a Neighbor Cache entry is created for the target, its reachability state MUST be set to STALE as specified in Section 7.3.3. (§8.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-8.3-1, so no unit is bound to it.

### [`RFC4861-8.3-2`](#rfc4861-8.3-2)

If a cache entry already existed and it is updated with a different link-layer address, its reachability state MUST also be set to STALE. (§8.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-8.3-2, so no unit is bound to it.

### [`RFC4861-8.3-3`](#rfc4861-8.3-3)

If the Target and Destination Addresses are the same, the host MUST treat the Target as on-link. (§8.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-8.3-3, so no unit is bound to it.

### [`RFC4861-8.3-4`](#rfc4861-8.3-4)

If the Target Address is not the same as the Destination Address, the host MUST set IsRouter to TRUE for the target. (§8.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-8.3-4, so no unit is bound to it.

### [`RFC4861-8.3-5`](#rfc4861-8.3-5)

A host MUST NOT send Redirect messages. (§8.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-8.3-5, so no unit is bound to it.

### [`RFC4861-9-1`](#rfc4861-9-1)

In order to ensure that future extensions properly coexist with current implementations, all nodes MUST silently ignore any options they do not recognize in received ND packets and continue processing the packet. (§9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4861SubscriberSolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc4861_solicitation_reader_linux_test.go#L59) | unit/verify | revert, verified |
| positive | [`TestRFC4861SolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/ra/rfc4861_solicitation_reader_linux_test.go#L57) | unit/verify | revert, verified |

### [`RFC4861-9-2`](#rfc4861-9-2)

All options specified in this document MUST be recognized. (§9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-9-2, so no unit is bound to it.

### [`RFC4861-9-3`](#rfc4861-9-3)

A node MUST NOT ignore valid options just because the ND message contains unrecognized ones. (§9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4861SubscriberSolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc4861_solicitation_reader_linux_test.go#L60) | unit/verify | revert, verified |
| negative | [`TestRFC4861SolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/ra/rfc4861_solicitation_reader_linux_test.go#L58) | unit/verify | revert, verified |

### [`RFC4861-9-4`](#rfc4861-9-4)

Thus, a receiver MUST NOT associate any action with the absence of an option in a particular packet. (§9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4861SubscriberSolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc4861_solicitation_reader_linux_test.go#L61) | unit/verify | revert, verified |
| negative | [`TestRFC4861SolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/ra/rfc4861_solicitation_reader_linux_test.go#L59) | unit/verify | revert, verified |

### [`RFC4861-9-5`](#rfc4861-9-5)

Options in Neighbor Discovery packets can appear in any order; receivers MUST be prepared to process them independently of their order. (§9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4861SubscriberSolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc4861_solicitation_reader_linux_test.go#L62) | unit/verify | revert, verified |
| positive | [`TestRFC4861SolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/ra/rfc4861_solicitation_reader_linux_test.go#L60) | unit/verify | revert, verified |

### [`RFC4861-9-6`](#rfc4861-9-6)

The amount of data to include in the Redirected Header option MUST be limited so that the entire redirect packet does not exceed the minimum MTU required to support IPv6 as specified in [IPv6]. (§9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-9-6, so no unit is bound to it.

### [`RFC4861-9-7`](#rfc4861-9-7)

When adding options to an ND packet, a node MUST NOT exceed the link MTU. (§9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-9-7, so no unit is bound to it.

### [`RFC4861-9-8`](#rfc4861-9-8)

Receivers MUST silently ignore any options they do not recognize and continue processing the message. (§9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4861SubscriberSolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc4861_solicitation_reader_linux_test.go#L63) | unit/verify | revert, verified |
| positive | [`TestRFC4861SolicitationReaderIgnoresReservedAndOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/ra/rfc4861_solicitation_reader_linux_test.go#L61) | unit/verify | revert, verified |

### [`RFC4861-11.2-1`](#rfc4861-11.2-1)

In any case, if either [IPv6-AUTH] or [IPv6-ESP] is used, ND packets MUST be verified for the purpose of authentication. (§11.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-11.2-1, so no unit is bound to it.

### [`RFC4861-11.2-2`](#rfc4861-11.2-2)

Packets that fail authentication checks MUST be silently discarded. (§11.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-11.2-2, so no unit is bound to it.

### [`RFC4861-D-1`](#rfc4861-d-1)

But when there is no host vs. router information in the ND message, the receipt of the message MUST NOT cause a change to the IsRouter state. (§D)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4861-D-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc4861.txt |
| Source fingerprint | 888f8e523e6703cd |
| Record | rfc/extraction/rfc4861.json |
| Mapped sentences | 138 |
| Declined as scope | 2 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of this Memo, copyright notice, Abstract and table of contents. |
| `1` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.1` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.2` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.3` | not stated | 1 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.4` | not stated | 1 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3.1` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3.2` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3.3` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `4` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `4.1` | not stated | 3 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `4.2` | not stated | 3 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `4.3` | not stated | 5 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `4.4` | not stated | 6 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `4.5` | not stated | 5 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `4.6` | not stated | 1 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `4.6.1` | not stated | 1 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `4.6.2` | not stated | 7 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `4.6.3` | not stated | 2 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `4.6.4` | not stated | 2 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `5` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `5.1` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `5.2` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `5.3` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6.1` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6.1.1` | not stated | 4 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6.1.2` | not stated | 3 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6.2` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6.2.1` | not stated | 8 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6.2.2` | not stated | 2 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6.2.3` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6.2.4` | not stated | 1 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6.2.5` | not stated | 2 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6.2.6` | not stated | 7 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6.2.7` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6.2.8` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6.3` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6.3.1` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6.3.2` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6.3.3` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6.3.4` | not stated | 8 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6.3.5` | not stated | 1 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6.3.6` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6.3.7` | not stated | 1 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `7` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `7.1` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `7.1.1` | not stated | 3 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `7.1.2` | not stated | 3 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `7.2` | not stated | 1 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `7.2.1` | not stated | 3 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `7.2.2` | not stated | 5 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `7.2.3` | not stated | 4 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `7.2.4` | not stated | 4 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `7.2.5` | not stated | 7 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `7.2.6` | not stated | 3 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `7.2.7` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `7.2.8` | not stated | 2 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `7.3` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `7.3.1` | not stated | 1 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `7.3.2` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `7.3.3` | not stated | 4 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `8` | not stated | 1 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `8.1` | not stated | 4 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `8.2` | not stated | 4 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `8.3` | not stated | 5 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `9` | not stated | 9 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `10` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `11` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `11.1` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `11.2` | not stated | 2 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `12` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `13` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `14` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `14.1` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `14.2` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `A` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `B` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `C` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `D` | not stated | 1 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `E` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `E.1` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `F` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `2.4:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The RFC 2119 key-words paragraph, written as 'The keywords MUST, MUST NOT ...', which the extractor did not strip as boilerplate. | The keywords MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, SHOULD NOT, RECOMMENDED, MAY, and OPTIONAL, when they appear in this document, are to be interpreted as described in [KEYWORDS]. |
| `9:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the author of a future ND option specification, who must design an option whose meaning does not depend on another option. No implementation, Ze's or the kernel's, writes an option definition. | The option MUST NOT depend on the presence or absence of any other options. |

## Superseded

No document obsoletes RFC 4861, so its obligations are stated where they were written.
