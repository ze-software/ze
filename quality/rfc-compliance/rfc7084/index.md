# RFC 7084 - Basic Requirements for IPv6 Customer Edge Routers

Partial. Every requirement this repository extracted from RFC 7084, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 41 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 34.1% | 14 of 41 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 41 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 41 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 100.0% | 14 of 14 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 41 | of 68 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 41 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 41 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 17.1% | 7 of 41 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 41 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |
| Linux cannot do this | 0.0% | 0 of 41 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and remains unmet; it stays in every denominator and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 48.8% | 20 of 41 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 shares marked as a part above are the whole of the 41 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 68 |
| Gated MUST-level | 41 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 20 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 14 |
| Tagged units | 14 |
| Recorded audit verdicts | 0 |
| Discrimination records | 14 |
| Summary | `rfc/short/rfc7084.md` |
| Requirement shard | `rfc/requirements/rfc7084.md` |
| RFC text | `rfc/full/rfc7084.txt` |

## Enrolment

Enrolled: Ze plays the IPv6 CE router of the fttx-edge use case: a DHCPv6 client that requests addresses and delegated prefixes on the WAN unit (internal/plugins/iface/dhcp), SLAAC and router discovery on the WAN through the kernel sysctls Ze writes (internal/component/iface/config_sysctl.go), and Router Advertisements on the LAN (internal/plugins/iface/ra, internal/core/ndp/ra.go). Walked site by site on 2026-10-26 (rfc/extraction/rfc7084.json). Every MUST is classified: the ICMPv6, Ethernet, Router Solicitation, L flag and weak host rows the Linux IPv6 stack performs are lower-layer; WAA-3 and L-1 roll up to rfc9915 and rfc4861; the CE glue between WAN provisioning and LAN advertisement (delegated prefix to LAN /64, Route Information Option, Router Lifetime tied to WAN state and prefixes, RA M/O flags starting DHCPv6, Reconfigure Accept, LAN DHCPv6 server, DNSSL, renumbering, ULA border filtering, IPv6 over PPP on the PPPoE client) is recorded as gaps; the remaining rows are Ze's and carry tagged tests with recorded discrimination. The 6rd and DS-Lite requirements are conditional on features this RFC makes SHOULD-level and Ze does not offer, and are excluded as feature-out-of-scope.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- WAN side: a DHCPv6 client with a persistent DUID that requests IA_NA, DNS servers and SOL_MAX_RT and, when configured, an IA_PD with an optional length hint, accepting any delegated prefix length and an IA_PD without addresses
- SLAAC and router discovery on the WAN through the kernel, with Ze installing the default route itself when the operator sets a route priority. LAN side: Router Advertisements with Prefix Information (A and L set by default), RDNSS and a zero Router Lifetime when an interface stops advertising.


**What the ledger says remains**

A delegated prefix is not assigned to LAN interfaces and no address is formed from it ([`RFC7084-4.3-5`](#rfc7084-4.3-5), [`RFC7084-4.2-16`](#rfc7084-4.2-16)); no Route Information Option ([`RFC7084-4.3-6`](#rfc7084-4.3-6)); the Router Lifetime is not tied to WAN default router state, WAN loss, configured prefixes or ULA-only prefixes ([`RFC7084-4.1-4`](#rfc7084-4.1-4), [`RFC7084-4.1-5`](#rfc7084-4.1-5), [`RFC7084-4.3-7`](#rfc7084-4.3-7), [`RFC7084-4.3-3`](#rfc7084-4.3-3)); forwarding is not held until acquisition completes ([`RFC7084-4.1-3`](#rfc7084-4.1-3)); DHCPv6 is not started from the RA M or O flags ([`RFC7084-4.2-15`](#rfc7084-4.2-15), [`RFC7084-4.2-22`](#rfc7084-4.2-22)); no Reconfigure Accept ([`RFC7084-4.2-14`](#rfc7084-4.2-14)); the prefix hint is not derived from the interface count ([`RFC7084-4.2-20`](#rfc7084-4.2-20)); no LAN DHCPv6 server and no DNS Search List option ([`RFC7084-4.3-10`](#rfc7084-4.3-10), [`RFC7084-4.3-11`](#rfc7084-4.3-11), [`RFC7084-4.3-12`](#rfc7084-4.3-12)); no renumbering of a replaced delegated prefix and no ICMPv6 code 5 ([`RFC7084-4.3-13`](#rfc7084-4.3-13), [`RFC7084-4.3-14`](#rfc7084-4.3-14)); no default ULA border filtering ([`RFC7084-4.3-2`](#rfc7084-4.3-2)); no IPv6 over PPP on the PPPoE client ([`RFC7084-4.2-9`](#rfc7084-4.2-9), [`RFC7084-4.2-10`](#rfc7084-4.2-10)). 6rd and DS-Lite are not offered. No Ze-owned requirement is yet proven by a tagged test.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated (including scoped evidence) | 41 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 2 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **41** | every gated MUST falls in exactly one bucket above |

**Annotated (including scoped evidence) (41):** [`RFC7084-4.1-1`](#rfc7084-4.1-1), [`RFC7084-4.1-2`](#rfc7084-4.1-2), [`RFC7084-4.1-3`](#rfc7084-4.1-3), [`RFC7084-4.1-4`](#rfc7084-4.1-4), [`RFC7084-4.1-5`](#rfc7084-4.1-5), [`RFC7084-4.2-1`](#rfc7084-4.2-1), [`RFC7084-4.2-2`](#rfc7084-4.2-2), [`RFC7084-4.2-3`](#rfc7084-4.2-3), [`RFC7084-4.2-4`](#rfc7084-4.2-4), [`RFC7084-4.2-5`](#rfc7084-4.2-5), [`RFC7084-4.2-6`](#rfc7084-4.2-6), [`RFC7084-4.2-7`](#rfc7084-4.2-7), [`RFC7084-4.2-8`](#rfc7084-4.2-8), [`RFC7084-4.2-9`](#rfc7084-4.2-9), [`RFC7084-4.2-10`](#rfc7084-4.2-10), [`RFC7084-4.2-11`](#rfc7084-4.2-11), [`RFC7084-4.2-12`](#rfc7084-4.2-12), [`RFC7084-4.2-14`](#rfc7084-4.2-14), [`RFC7084-4.2-15`](#rfc7084-4.2-15), [`RFC7084-4.2-16`](#rfc7084-4.2-16), [`RFC7084-4.2-17`](#rfc7084-4.2-17), [`RFC7084-4.2-18`](#rfc7084-4.2-18), [`RFC7084-4.2-19`](#rfc7084-4.2-19), [`RFC7084-4.2-20`](#rfc7084-4.2-20), [`RFC7084-4.2-21`](#rfc7084-4.2-21), [`RFC7084-4.2-22`](#rfc7084-4.2-22), [`RFC7084-4.2-23`](#rfc7084-4.2-23), [`RFC7084-4.2-24`](#rfc7084-4.2-24), [`RFC7084-4.3-1`](#rfc7084-4.3-1), [`RFC7084-4.3-2`](#rfc7084-4.3-2), [`RFC7084-4.3-3`](#rfc7084-4.3-3), [`RFC7084-4.3-5`](#rfc7084-4.3-5), [`RFC7084-4.3-6`](#rfc7084-4.3-6), [`RFC7084-4.3-7`](#rfc7084-4.3-7), [`RFC7084-4.3-8`](#rfc7084-4.3-8), [`RFC7084-4.3-9`](#rfc7084-4.3-9), [`RFC7084-4.3-10`](#rfc7084-4.3-10), [`RFC7084-4.3-11`](#rfc7084-4.3-11), [`RFC7084-4.3-12`](#rfc7084-4.3-12), [`RFC7084-4.3-13`](#rfc7084-4.3-13), [`RFC7084-4.3-14`](#rfc7084-4.3-14)

**Derived from other rows (2):** [`RFC7084-4.2-13`](#rfc7084-4.2-13), [`RFC7084-4.3-4`](#rfc7084-4.3-4)

## Attainable ceiling

Mandatory ceiling: 34 of 41 (82.9%); excluded: Met by a lower layer: 7 of 41 (17.1%). Optional ceiling: 25 of 25 (100.0%). Lowest mandatory ceiling: rfc7084 34 of 41 (82.9%). Lowest optional ceiling: rfc7084 25 of 25 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7084-4.1-1` | G-2: The IPv6 CE router MUST implement ICMPv6 according to [RFC4443]. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 ICMP (net/ipv6/icmp.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses the kernel originates and answers ICMPv6 from, and Ze builds no ICMPv6 error or echo message and holds no state this behaviour reads |
| `RFC7084-4.1-2` | In particular, point-to-point links MUST be handled as described in Section 3.1 of [RFC4443]. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 ICMP (net/ipv6/icmp.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses the kernel originates and answers ICMPv6 from, and Ze builds no ICMPv6 error or echo message and holds no state this behaviour reads |
| `RFC7084-4.1-3` | G-3: The IPv6 CE router MUST NOT forward any IPv6 traffic between its LAN interface(s) and its WAN interface until the router has successfully completed the IPv6 address and the delegated prefix acquisition process. (§4.1) | MUST NOT | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** IPv6 forwarding follows the operator's forwarding leaf alone and is never held off until the DHCPv6 address and delegated prefix are acquired; internal/component/iface/config_sysctl.go::applySysctl |
| `RFC7084-4.1-4` | G-4: By default, an IPv6 CE router that has no default router(s) on its WAN interface MUST NOT advertise itself as an IPv6 default router on its LAN interfaces. (§4.1) | MUST NOT | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the advertised Router Lifetime does not depend on whether the WAN interface has a default router; internal/component/iface/config_ra.go parseRAConfig takes router-lifetime from the operator (default 1800 seconds) and nothing lowers it |
| `RFC7084-4.1-5` | G-5: By default, if the IPv6 CE router is an advertising router and loses its IPv6 default router(s) and/or detects loss of connectivity on the WAN interface, it MUST explicitly invalidate itself as an IPv6 default router on each of its advertising interfaces by immediately transmitting one or more Router Advertisement messages with the "Router Lifetime" field set to zero [RFC4861]. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a zero Router Lifetime is sent only when an interface stops advertising, never when the WAN loses its default router or its connectivity; internal/plugins/iface/ra/sender_linux.go Sender.sendFinal |
| `RFC7084-4.2-1` | W-1: When the router is attached to the WAN interface link, it MUST act as an IPv6 host for the purposes of stateless [RFC4862] or stateful [RFC3315] interface address assignment. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC7084WANUnitTurnsOnSLAAC`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc7084_ce_router_test.go#L29). **negative:** no negative test. **{single-polarity}:** the row asks for a capability, host address assignment on the WAN, and no input exists under which a conforming router declines it, so only its presence can be asserted |
| `RFC7084-4.2-2` | W-2: The IPv6 CE router MUST generate a link-local address and finish Duplicate Address Detection according to [RFC4862] prior to sending any Router Solicitations on the interface. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 address configuration (net/ipv6/addrconf.c, which sends the first Router Solicitation from addrconf_dad_completed once the link-local address has passed Duplicate Address Detection, sourced from that address); internal/component/iface/config_sysctl.go::applySysctl writes the accept_ra and autoconf values that decide whether solicitations are sent at all, and no value Ze writes decides their timing or source address |
| `RFC7084-4.2-3` | The source address used in the subsequent Router Solicitation MUST be the link-local address on the WAN interface. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 address configuration (net/ipv6/addrconf.c, which sends the first Router Solicitation from addrconf_dad_completed once the link-local address has passed Duplicate Address Detection, sourced from that address); internal/component/iface/config_sysctl.go::applySysctl writes the accept_ra and autoconf values that decide whether solicitations are sent at all, and no value Ze writes decides their timing or source address |
| `RFC7084-4.2-4` | W-3: Absent other routing information, the IPv6 CE router MUST use Router Discovery as specified in [RFC4861] to discover a default router(s) and install a default route(s) in its routing table with the discovered router's address as the next hop. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC7084DiscoveredRouterBecomesTheDefaultRoute`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc7084_ce_router_test.go#L89). **negative:** no negative test. **{single-polarity}:** the row asks the router to install a default route from a discovered router, and no input exists under which a conforming router must refuse one |
| `RFC7084-4.2-5` | W-4: The router MUST act as a requesting router for the purposes of DHCPv6 prefix delegation ([RFC3633]). (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC7084RequestingRouterSolicitsIAPD`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc7084_dhcp_v6_ce_linux_test.go#L32). **negative:** no negative test. **{single-polarity}:** the row asks for a capability, acting as a requesting router, and no input exists under which a conforming router declines it |
| `RFC7084-4.2-6` | W-5: The IPv6 CE router MUST use a persistent DHCP Unique Identifier (DUID) for DHCPv6 messages. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestDHCPv6DefaultDUIDStable`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L620). **negative:** no negative test. **{single-polarity}:** the row asks for a persistent DUID, which only its use across messages and restarts can show |
| `RFC7084-4.2-7` | The DUID MUST NOT change between network-interface resets or IPv6 CE router reboots. (§4.2) | MUST NOT | 4.2 | **positive:** no positive test. **negative:** `unit/verify` [`TestDHCPv6DefaultDUIDStable`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L621). **{single-polarity}:** the row forbids a DUID change, which only the absence of a change across a restart can show |
| `RFC7084-4.2-8` | WLL-1: If the WAN interface supports Ethernet encapsulation, then the IPv6 CE router MUST support IPv6 over Ethernet [RFC2464]. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 over Ethernet (net/ipv6 with the Ethernet interface identifier in net/ipv6/addrconf.c and address resolution in net/ipv6/ndisc.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the Ethernet interface, and Ze writes no frame and no value the encapsulation reads |
| `RFC7084-4.2-9` | WLL-2: If the WAN interface supports PPP encapsulation, the IPv6 CE router MUST support IPv6 over PPP [RFC5072]. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the PPPoE client negotiates LCP, authentication and IPCP only, with no IPV6CP, so no IPv6 runs over a PPP WAN interface; internal/component/l2tp/pppoeclient/session.go negotiateSession |
| `RFC7084-4.2-10` | WLL-3: If the WAN interface supports PPP encapsulation, in a dual- stack environment with IPCP and IPV6CP running over one PPP logical channel, the Network Control Protocols (NCPs) MUST be treated as independent of each other and start and terminate independently. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the PPPoE client runs IPCP alone and no IPV6CP, so the dual-stack pair of independent NCPs does not exist; internal/component/l2tp/pppoeclient/session.go negotiateSession |
| `RFC7084-4.2-11` | WAA-1: The IPv6 CE router MUST support Stateless Address Autoconfiguration (SLAAC) [RFC4862]. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC7084WANUnitTurnsOnSLAAC`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc7084_ce_router_test.go#L28). **negative:** no negative test. **{single-polarity}:** the row asks for SLAAC support, and no input exists under which a conforming router declines it |
| `RFC7084-4.2-12` | WAA-2: The IPv6 CE router MUST follow the recommendations in Section 4 of [RFC5942], and in particular the handling of the L flag in the Router Advertisement Prefix Information option. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 address configuration (net/ipv6/addrconf.c addrconf_prefix_rcv installs an on-link prefix route only for a Prefix Information option whose L flag is set, and forms an address from the A flag independently of it); internal/component/iface/config_sysctl.go::applySysctl writes the autoconf and accept_ra values, and no value Ze writes decides how the L flag is read |
| `RFC7084-4.2-13` | WAA-3: The IPv6 CE router MUST support DHCPv6 [RFC3315] client behavior. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{rollup}:** rfc9915; WAA-3 asks for the DHCPv6 client behaviour of RFC 3315, which RFC 9915 now states, and asserts nothing of its own. **derived:** gap: RFC9915-15-2 is annotated {gap} |
| `RFC7084-4.2-14` | WAA-4: The IPv6 CE router MUST be able to support the following DHCPv6 options: Identity Association for Non-temporary Address (IA_NA), Reconfigure Accept [RFC3315], and DNS_SERVERS [RFC3646]. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the client sends IA_NA and asks for DNS_SERVERS (option 23), but never sends Reconfigure Accept and handles no Reconfigure message; internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go v6Worker.buildSolicit and requestOptions |
| `RFC7084-4.2-15` | WAA-6: If the IPv6 CE router receives a Router Advertisement message (described in [RFC4861]) with the M flag set to 1, the IPv6 CE router MUST do DHCPv6 address assignment (request an IA_NA option). (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the DHCPv6 client starts from the dhcpv6 enabled leaf and never reads the M flag of a received Router Advertisement; internal/plugins/iface/dhcp/dhcp_v6_linux.go DHCPClient.runV6 |
| `RFC7084-4.2-16` | WAA-7: If the IPv6 CE router does not acquire a global IPv6 address(es) from either SLAAC or DHCPv6, then it MUST create a global IPv6 address(es) from its delegated prefix(es) and configure those on one of its internal virtual network interfaces, unless configured to require a global IPv6 address on the WAN interface. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no global address is formed from a delegated prefix on any interface; internal/plugins/iface/dhcp/dhcp_v6_linux.go v6Worker.effect publishes each delegated prefix as a lease event on the WAN unit and nothing consumes it to address an interface |
| `RFC7084-4.2-17` | WAA-8: The CE router MUST support the SOL_MAX_RT option [RFC7083] and request the SOL_MAX_RT option in an Option Request Option (ORO). (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC7084SolMaxRTRequestedAndHonoured`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc7084_dhcp_v6_ce_linux_test.go#L55). **negative:** no negative test. **{single-polarity}:** the row asks for SOL_MAX_RT support and its request in the ORO, and no input exists under which a conforming client omits them |
| `RFC7084-4.2-18` | WAA-9: As a router, the IPv6 CE router MUST follow the weak host (Weak End System) model [RFC1122]. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 source address selection (net/ipv6/addrconf.c ipv6_dev_get_saddr, which takes a source address from another interface when the outgoing one has none of suitable scope); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the addresses it chooses from, and Ze sets no source selection policy |
| `RFC7084-4.2-19` | WPD-1: The IPv6 CE router MUST support DHCPv6 prefix delegation requesting router behavior as specified in [RFC3633] (Identity Association for Prefix Delegation (IA_PD) option). (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC7084RequestingRouterSolicitsIAPD`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc7084_dhcp_v6_ce_linux_test.go#L33). **negative:** no negative test. **{single-polarity}:** the row asks for IA_PD requesting router behaviour, and no input exists under which a conforming router declines it |
| `RFC7084-4.2-20` | If so, it MUST ask for a prefix large enough to assign one /64 for each of its interfaces, rounded up to the nearest nibble, and SHOULD be configurable to ask for more. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the prefix hint is the operator's pd length (48 to 64) sent unchanged, never derived from the number of interfaces and rounded up to a nibble; internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go v6Worker.emptyIAPD |
| `RFC7084-4.2-21` | WPD-3: The IPv6 CE router MUST be prepared to accept a delegated prefix size different from what is given in the hint. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC7084DelegatedPrefixOtherThanHintWithoutAddresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc7084_dhcp_v6_ce_linux_test.go#L73). **negative:** no negative test. **{single-polarity}:** the row asks the router to accept a delegated prefix of any length, and no length exists that a conforming router refuses on that ground |
| `RFC7084-4.2-22` | WPD-4: By default, the IPv6 CE router MUST initiate DHCPv6 prefix delegation when either the M or O flags are set to 1 in a received Router Advertisement (RA) message. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** prefix delegation starts from the configured pd container, never from the M or O flag of a received Router Advertisement; internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go v6Worker.emptyIAPD |
| `RFC7084-4.2-23` | WPD-6: If the IPv6 CE router requests both an IA_NA and an IA_PD option in DHCPv6, it MUST accept an IA_PD option in DHCPv6 Advertise/Reply messages, even if the message does not contain any addresses, unless configured to only obtain its WAN IPv6 address via DHCPv6; see [DHCPv6-STATEFUL-ISSUES]. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC7084DelegatedPrefixOtherThanHintWithoutAddresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc7084_dhcp_v6_ce_linux_test.go#L74). **negative:** no negative test. **{single-polarity}:** the row asks the router to accept an IA_PD without addresses, and no such Reply exists that a conforming router refuses on that ground |
| `RFC7084-4.2-24` | WPD-7: By default, an IPv6 CE router MUST NOT initiate any dynamic routing protocol on its WAN interface. (§4.2) | MUST NOT | 4.2 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC7084InterfaceOnlyConfigStartsNoRoutingProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/plugin/server/rfc7084_wan_routing_test.go#L26). **{single-polarity}:** the row forbids starting a routing protocol by default, which only the absence of one under a configuration that names none can show |
| `RFC7084-4.3-1` | ULA-2: An IPv6 CE router with a ULA prefix MUST maintain this prefix consistently across reboots. (§4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC7084ULAPrefixComesFromTheConfigAlone`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc7084_ce_router_test.go#L71). **negative:** no negative test. **{single-polarity}:** the row asks the router to keep its ULA prefix, which only the same prefix on each load of one configuration can show |
| `RFC7084-4.3-2` | ULA-4: By default, the IPv6 CE router MUST act as a site border router according to Section 4.3 of [RFC4193] and filter packets with local IPv6 source or destination addresses accordingly. (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no filter of ULA source or destination addresses at the site border is installed by default; such filtering exists only as firewall rules the operator writes |
| `RFC7084-4.3-3` | ULA-5: An IPv6 CE router MUST NOT advertise itself as a default router with a Router Lifetime greater than zero whenever all of its configured and delegated prefixes are ULA prefixes. (§4.3) | MUST NOT | 4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Router Lifetime does not depend on whether the advertised prefixes are all ULA prefixes; internal/component/iface/config_ra.go parseRAConfig takes router-lifetime from the operator (default 1800 seconds) and nothing lowers it |
| `RFC7084-4.3-4` | L-1: The IPv6 CE router MUST support router behavior according to Neighbor Discovery for IPv6 [RFC4861]. (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{rollup}:** rfc4861; L-1 asks for the router behaviour RFC 4861 states and asserts nothing of its own. **derived:** gap: RFC4861-4.4-3 is annotated {gap} |
| `RFC7084-4.3-5` | L-2: The IPv6 CE router MUST assign a separate /64 from its delegated prefix(es) (and ULA prefix if configured to provide ULA addressing) for each of its LAN interfaces. (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no /64 is assigned from a delegated prefix to any LAN interface; internal/plugins/iface/dhcp/dhcp_v6_linux.go v6Worker.effect publishes each delegated prefix as a lease event on the WAN unit and nothing consumes it to address an interface |
| `RFC7084-4.3-6` | L-3: An IPv6 CE router MUST advertise itself as a router for the delegated prefix(es) (and ULA prefix if configured to provide ULA addressing) using the "Route Information Option" specified in Section 2.3 of [RFC4191]. (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Router Advertisement encoder writes Source Link-layer Address, Prefix Information and RDNSS options only, and no Route Information Option; internal/core/ndp/ra.go BuildRA |
| `RFC7084-4.3-7` | L-4: An IPv6 CE router MUST NOT advertise itself as a default router with a Router Lifetime [RFC4861] greater than zero if it has no prefixes configured or delegated to it. (§4.3) | MUST NOT | 4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Router Lifetime does not depend on whether any prefix is configured or delegated; internal/component/iface/config_ra.go parseRAConfig takes router-lifetime from the operator (default 1800 seconds) and nothing lowers it |
| `RFC7084-4.3-8` | L-5: The IPv6 CE router MUST make each LAN interface an advertising interface according to [RFC4861]. (§4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC4861OnlyAnAdvertisingInterfaceSends`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L101). **negative:** no negative test. **{single-polarity}:** the row asks that a LAN interface advertise, and no input exists under which a conforming router keeps a LAN interface silent |
| `RFC7084-4.3-9` | L-6: In Router Advertisement messages ([RFC4861]), the Prefix Information option's A and L flags MUST be set to 1 by default. (§4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRAConfigParse`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc7084_config_ra_test.go#L104). **negative:** no negative test. **{single-polarity}:** the row fixes a default, and no input exists under which a conforming default leaves A or L clear, so only the default itself can be asserted |
| `RFC7084-4.3-10` | L-8: The IPv6 CE router MUST support a DHCPv6 server capable of IPv6 address assignment according to [RFC3315] OR a stateless DHCPv6 server according to [RFC3736] on its LAN interfaces. (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the DHCP server is IPv4 only and refuses an IPv6 subnet (internal/plugins/dhcpserver/config.go), and the DHCPv6 server on L2TP subscriber sessions only delegates prefixes (internal/component/l2tp/ppp/dhcpv6.go), so no LAN interface has a DHCPv6 address-assignment or stateless server |
| `RFC7084-4.3-11` | L-10: The IPv6 CE router MUST support providing DNS information in the DHCPv6 DNS_SERVERS and DOMAIN_LIST options [RFC3646]. (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no LAN-side DHCPv6 server exists to send DNS_SERVERS or DOMAIN_LIST (internal/plugins/dhcpserver/config.go refuses an IPv6 subnet) |
| `RFC7084-4.3-12` | L-11: The IPv6 CE router MUST support providing DNS information in the Router Advertisement Recursive DNS Server (RDNSS) and DNS Search List options. (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** RDNSS is advertised, but the Router Advertisement encoder has no DNS Search List option; internal/core/ndp/ra.go BuildRA |
| `RFC7084-4.3-13` | L-13: If the delegated prefix changes, i.e., the current prefix is replaced with a new prefix without any overlapping time period, then the IPv6 CE router MUST immediately advertise the old prefix with a Preferred Lifetime of zero and a Valid Lifetime of either a) zero or b) the lower of the current Valid Lifetime and two hours (which must be decremented in real time) in a Router Advertisement message as described in Section 5.5.3, (e) of [RFC4862]. (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Router Advertisements carry only configured prefixes, so a replaced delegated prefix is never re-advertised with a zero Preferred Lifetime; internal/component/iface/config_ra.go raParsePrefixes |
| `RFC7084-4.3-14` | L-14: The IPv6 CE router MUST send an ICMPv6 Destination Unreachable message, code 5 (Source address failed ingress/egress policy) for packets forwarded to it that use an address from a prefix that has been invalidated. (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no prefix is ever invalidated and nothing sends ICMPv6 Destination Unreachable code 5 for one |
| `RFC7084-4.2-25` | W-6: The WAN interface of the CE router SHOULD support a Port Control Protocol (PCP) client as specified in [RFC6887] for use by applications on the CE router. (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7084-4.2-26` | The PCP client SHOULD follow the procedure specified in Section 8.1 of [RFC6887] to discover its PCP server. (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7084-4.2-27` | The IPv6 CE router SHOULD be able to support the DNS Search List (DNSSL) option as specified in [RFC3646]. (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7084-4.2-28` | WAA-5: The IPv6 CE router SHOULD implement the Network Time Protocol (NTP) as specified in [RFC5905] to provide a time reference common to the service provider for other protocols, such as DHCPv6, to use. (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7084-4.2-29` | WAA-10: The IPv6 CE router SHOULD implement the Information Refresh Time option and associated client behavior as specified in [RFC4242]. (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7084-4.2-30` | and SHOULD be configurable to ask for more. (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7084-4.2-31` | If the delegated prefix is too small to address all of its interfaces, the IPv6 CE router SHOULD log a system management error. (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7084-4.2-32` | The IPv6 CE router SHOULD send an ICMPv6 Destination Unreachable message in accordance with Section 3.1 of [RFC4443] back to the source of the packet, if the packet is to be dropped due to this rule. (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7084-4.2-33` | WPD-8: The IPv6 CE router SHOULD support the [RFC6603] Prefix Exclude option. (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7084-4.3-15` | ULA-1: The IPv6 CE router SHOULD be capable of generating a ULA prefix [RFC4193]. (§4.3) | SHOULD | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7084-4.3-16` | ULA-3: The value of the ULA prefix SHOULD be configurable. (§4.3) | SHOULD | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7084-4.3-17` | L-7: The A and L flags' ([RFC4861]) settings SHOULD be user configurable. (§4.3) | SHOULD | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7084-4.3-18` | L-9: Unless the IPv6 CE router is configured to support the DHCPv6 IA_NA option, it SHOULD set the M flag to zero and the O flag to 1 in its Router Advertisement messages [RFC4861]. (§4.3) | SHOULD | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7084-4.3-19` | L-12: The IPv6 CE router SHOULD make available a subset of DHCPv6 options (as listed in Section 5.3 of [RFC3736]) received from the DHCPv6 client on its WAN interface to its LAN-side DHCPv6 server. (§4.3) | SHOULD | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7084-4.4.1-1` | The CE router SHOULD support 6rd functionality. (§4.4.1) | SHOULD | 4.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7084-4.4.1-2` | If the CE router has obtained an IPv4 network address through some other means such as PPP, it SHOULD use the DHCPINFORM request message [RFC2131] to request the 6rd DHCPv4 Option. (§4.4.1) | SHOULD | 4.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7084-4.4.2-1` | The IPv6 CE router SHOULD implement DS-Lite functionality. (§4.4.2) | SHOULD | 4.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7084-4.4.2-2` | DLW-3: If the IPv6 CE router is configured with an IPv4 address on its WAN interface, then the IPv6 CE router SHOULD disable the DS-Lite Basic Bridging BroadBand (B4) element. (§4.4.2) | SHOULD | 4.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7084-4.5-1` | S-1: The IPv6 CE router SHOULD support [RFC6092]. (§4.5) | SHOULD | 4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7084-4.5-2` | In particular, the IPv6 CE router SHOULD support functionality sufficient for implementing the set of recommendations in [RFC6092], Section 4. (§4.5) | SHOULD | 4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7084-4.5-3` | S-2: The IPv6 CE router SHOULD support ingress filtering in accordance with BCP 38 [RFC2827]. (§4.5) | SHOULD | 4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7084-4.5-4` | S-3: If the IPv6 CE router firewall is configured to filter incoming tunneled data, the firewall SHOULD provide the capability to filter decapsulated packets from a tunnel. (§4.5) | SHOULD | 4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7084-4.2-34` | WPD-2: The IPv6 CE router MAY indicate as a hint to the delegating router the size of the prefix it requires. (§4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7084-4.4.1-3` | The IPv6 CE router MAY use other mechanisms to configure 6rd parameters. (§4.4.1) | MAY | 4.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7084-4.4.2-3` | The IPv6 CE router MAY use other mechanisms to configure DS-Lite parameters. (§4.4.2) | MAY | 4.4.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7084-4.1-1`](#rfc7084-4.1-1) G-2: The IPv6 CE router MUST implement ICMPv6 according to [RFC4443]. (§4.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 ICMP (net/ipv6/icmp.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses the kernel originates and answers ICMPv6 from, and Ze builds no ICMPv6 error or echo message and holds no state this behaviour reads |
| [`RFC7084-4.1-2`](#rfc7084-4.1-2) In particular, point-to-point links MUST be handled as described in Section 3.1 of [RFC4443]. (§4.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 ICMP (net/ipv6/icmp.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses the kernel originates and answers ICMPv6 from, and Ze builds no ICMPv6 error or echo message and holds no state this behaviour reads |
| [`RFC7084-4.1-3`](#rfc7084-4.1-3) G-3: The IPv6 CE router MUST NOT forward any IPv6 traffic between its LAN interface(s) and its WAN interface until the router has successfully completed the IPv6 address and the delegated prefix acquisition process. (§4.1) | {gap}, no test | IPv6 forwarding follows the operator's forwarding leaf alone and is never held off until the DHCPv6 address and delegated prefix are acquired; internal/component/iface/config_sysctl.go::applySysctl |
| [`RFC7084-4.1-4`](#rfc7084-4.1-4) G-4: By default, an IPv6 CE router that has no default router(s) on its WAN interface MUST NOT advertise itself as an IPv6 default router on its LAN interfaces. (§4.1) | {gap}, no test | the advertised Router Lifetime does not depend on whether the WAN interface has a default router; internal/component/iface/config_ra.go parseRAConfig takes router-lifetime from the operator (default 1800 seconds) and nothing lowers it |
| [`RFC7084-4.1-5`](#rfc7084-4.1-5) G-5: By default, if the IPv6 CE router is an advertising router and loses its IPv6 default router(s) and/or detects loss of connectivity on the WAN interface, it MUST explicitly invalidate itself as an IPv6 default router on each of its advertising interfaces by immediately transmitting one or more Router Advertisement messages with the "Router Lifetime" field set to zero [RFC4861]. (§4.1) | {gap}, no test | a zero Router Lifetime is sent only when an interface stops advertising, never when the WAN loses its default router or its connectivity; internal/plugins/iface/ra/sender_linux.go Sender.sendFinal |
| [`RFC7084-4.2-2`](#rfc7084-4.2-2) W-2: The IPv6 CE router MUST generate a link-local address and finish Duplicate Address Detection according to [RFC4862] prior to sending any Router Solicitations on the interface. (§4.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 address configuration (net/ipv6/addrconf.c, which sends the first Router Solicitation from addrconf_dad_completed once the link-local address has passed Duplicate Address Detection, sourced from that address); internal/component/iface/config_sysctl.go::applySysctl writes the accept_ra and autoconf values that decide whether solicitations are sent at all, and no value Ze writes decides their timing or source address |
| [`RFC7084-4.2-3`](#rfc7084-4.2-3) The source address used in the subsequent Router Solicitation MUST be the link-local address on the WAN interface. (§4.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 address configuration (net/ipv6/addrconf.c, which sends the first Router Solicitation from addrconf_dad_completed once the link-local address has passed Duplicate Address Detection, sourced from that address); internal/component/iface/config_sysctl.go::applySysctl writes the accept_ra and autoconf values that decide whether solicitations are sent at all, and no value Ze writes decides their timing or source address |
| [`RFC7084-4.2-8`](#rfc7084-4.2-8) WLL-1: If the WAN interface supports Ethernet encapsulation, then the IPv6 CE router MUST support IPv6 over Ethernet [RFC2464]. (§4.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 over Ethernet (net/ipv6 with the Ethernet interface identifier in net/ipv6/addrconf.c and address resolution in net/ipv6/ndisc.c); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the IPv6 addresses of the Ethernet interface, and Ze writes no frame and no value the encapsulation reads |
| [`RFC7084-4.2-9`](#rfc7084-4.2-9) WLL-2: If the WAN interface supports PPP encapsulation, the IPv6 CE router MUST support IPv6 over PPP [RFC5072]. (§4.2) | {gap}, no test | the PPPoE client negotiates LCP, authentication and IPCP only, with no IPV6CP, so no IPv6 runs over a PPP WAN interface; internal/component/l2tp/pppoeclient/session.go negotiateSession |
| [`RFC7084-4.2-10`](#rfc7084-4.2-10) WLL-3: If the WAN interface supports PPP encapsulation, in a dual- stack environment with IPCP and IPV6CP running over one PPP logical channel, the Network Control Protocols (NCPs) MUST be treated as independent of each other and start and terminate independently. (§4.2) | {gap}, no test | the PPPoE client runs IPCP alone and no IPV6CP, so the dual-stack pair of independent NCPs does not exist; internal/component/l2tp/pppoeclient/session.go negotiateSession |
| [`RFC7084-4.2-12`](#rfc7084-4.2-12) WAA-2: The IPv6 CE router MUST follow the recommendations in Section 4 of [RFC5942], and in particular the handling of the L flag in the Router Advertisement Prefix Information option. (§4.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 address configuration (net/ipv6/addrconf.c addrconf_prefix_rcv installs an on-link prefix route only for a Prefix Information option whose L flag is set, and forms an address from the A flag independently of it); internal/component/iface/config_sysctl.go::applySysctl writes the autoconf and accept_ra values, and no value Ze writes decides how the L flag is read |
| [`RFC7084-4.2-13`](#rfc7084-4.2-13) WAA-3: The IPv6 CE router MUST support DHCPv6 [RFC3315] client behavior. (§4.2) | no test | no test carries this requirement id; annotated {rollup}: rfc9915; WAA-3 asks for the DHCPv6 client behaviour of RFC 3315, which RFC 9915 now states, and asserts nothing of its own |
| [`RFC7084-4.2-14`](#rfc7084-4.2-14) WAA-4: The IPv6 CE router MUST be able to support the following DHCPv6 options: Identity Association for Non-temporary Address (IA_NA), Reconfigure Accept [RFC3315], and DNS_SERVERS [RFC3646]. (§4.2) | {gap}, no test | the client sends IA_NA and asks for DNS_SERVERS (option 23), but never sends Reconfigure Accept and handles no Reconfigure message; internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go v6Worker.buildSolicit and requestOptions |
| [`RFC7084-4.2-15`](#rfc7084-4.2-15) WAA-6: If the IPv6 CE router receives a Router Advertisement message (described in [RFC4861]) with the M flag set to 1, the IPv6 CE router MUST do DHCPv6 address assignment (request an IA_NA option). (§4.2) | {gap}, no test | the DHCPv6 client starts from the dhcpv6 enabled leaf and never reads the M flag of a received Router Advertisement; internal/plugins/iface/dhcp/dhcp_v6_linux.go DHCPClient.runV6 |
| [`RFC7084-4.2-16`](#rfc7084-4.2-16) WAA-7: If the IPv6 CE router does not acquire a global IPv6 address(es) from either SLAAC or DHCPv6, then it MUST create a global IPv6 address(es) from its delegated prefix(es) and configure those on one of its internal virtual network interfaces, unless configured to require a global IPv6 address on the WAN interface. (§4.2) | {gap}, no test | no global address is formed from a delegated prefix on any interface; internal/plugins/iface/dhcp/dhcp_v6_linux.go v6Worker.effect publishes each delegated prefix as a lease event on the WAN unit and nothing consumes it to address an interface |
| [`RFC7084-4.2-18`](#rfc7084-4.2-18) WAA-9: As a router, the IPv6 CE router MUST follow the weak host (Weak End System) model [RFC1122]. (§4.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 source address selection (net/ipv6/addrconf.c ipv6_dev_get_saddr, which takes a source address from another interface when the outgoing one has none of suitable scope); internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the addresses it chooses from, and Ze sets no source selection policy |
| [`RFC7084-4.2-20`](#rfc7084-4.2-20) If so, it MUST ask for a prefix large enough to assign one /64 for each of its interfaces, rounded up to the nearest nibble, and SHOULD be configurable to ask for more. (§4.2) | {gap}, no test | the prefix hint is the operator's pd length (48 to 64) sent unchanged, never derived from the number of interfaces and rounded up to a nibble; internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go v6Worker.emptyIAPD |
| [`RFC7084-4.2-22`](#rfc7084-4.2-22) WPD-4: By default, the IPv6 CE router MUST initiate DHCPv6 prefix delegation when either the M or O flags are set to 1 in a received Router Advertisement (RA) message. (§4.2) | {gap}, no test | prefix delegation starts from the configured pd container, never from the M or O flag of a received Router Advertisement; internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go v6Worker.emptyIAPD |
| [`RFC7084-4.3-2`](#rfc7084-4.3-2) ULA-4: By default, the IPv6 CE router MUST act as a site border router according to Section 4.3 of [RFC4193] and filter packets with local IPv6 source or destination addresses accordingly. (§4.3) | {gap}, no test | no filter of ULA source or destination addresses at the site border is installed by default; such filtering exists only as firewall rules the operator writes |
| [`RFC7084-4.3-3`](#rfc7084-4.3-3) ULA-5: An IPv6 CE router MUST NOT advertise itself as a default router with a Router Lifetime greater than zero whenever all of its configured and delegated prefixes are ULA prefixes. (§4.3) | {gap}, no test | the Router Lifetime does not depend on whether the advertised prefixes are all ULA prefixes; internal/component/iface/config_ra.go parseRAConfig takes router-lifetime from the operator (default 1800 seconds) and nothing lowers it |
| [`RFC7084-4.3-4`](#rfc7084-4.3-4) L-1: The IPv6 CE router MUST support router behavior according to Neighbor Discovery for IPv6 [RFC4861]. (§4.3) | no test | no test carries this requirement id; annotated {rollup}: rfc4861; L-1 asks for the router behaviour RFC 4861 states and asserts nothing of its own |
| [`RFC7084-4.3-5`](#rfc7084-4.3-5) L-2: The IPv6 CE router MUST assign a separate /64 from its delegated prefix(es) (and ULA prefix if configured to provide ULA addressing) for each of its LAN interfaces. (§4.3) | {gap}, no test | no /64 is assigned from a delegated prefix to any LAN interface; internal/plugins/iface/dhcp/dhcp_v6_linux.go v6Worker.effect publishes each delegated prefix as a lease event on the WAN unit and nothing consumes it to address an interface |
| [`RFC7084-4.3-6`](#rfc7084-4.3-6) L-3: An IPv6 CE router MUST advertise itself as a router for the delegated prefix(es) (and ULA prefix if configured to provide ULA addressing) using the "Route Information Option" specified in Section 2.3 of [RFC4191]. (§4.3) | {gap}, no test | the Router Advertisement encoder writes Source Link-layer Address, Prefix Information and RDNSS options only, and no Route Information Option; internal/core/ndp/ra.go BuildRA |
| [`RFC7084-4.3-7`](#rfc7084-4.3-7) L-4: An IPv6 CE router MUST NOT advertise itself as a default router with a Router Lifetime [RFC4861] greater than zero if it has no prefixes configured or delegated to it. (§4.3) | {gap}, no test | the Router Lifetime does not depend on whether any prefix is configured or delegated; internal/component/iface/config_ra.go parseRAConfig takes router-lifetime from the operator (default 1800 seconds) and nothing lowers it |
| [`RFC7084-4.3-10`](#rfc7084-4.3-10) L-8: The IPv6 CE router MUST support a DHCPv6 server capable of IPv6 address assignment according to [RFC3315] OR a stateless DHCPv6 server according to [RFC3736] on its LAN interfaces. (§4.3) | {gap}, no test | the DHCP server is IPv4 only and refuses an IPv6 subnet (internal/plugins/dhcpserver/config.go), and the DHCPv6 server on L2TP subscriber sessions only delegates prefixes (internal/component/l2tp/ppp/dhcpv6.go), so no LAN interface has a DHCPv6 address-assignment or stateless server |
| [`RFC7084-4.3-11`](#rfc7084-4.3-11) L-10: The IPv6 CE router MUST support providing DNS information in the DHCPv6 DNS_SERVERS and DOMAIN_LIST options [RFC3646]. (§4.3) | {gap}, no test | no LAN-side DHCPv6 server exists to send DNS_SERVERS or DOMAIN_LIST (internal/plugins/dhcpserver/config.go refuses an IPv6 subnet) |
| [`RFC7084-4.3-12`](#rfc7084-4.3-12) L-11: The IPv6 CE router MUST support providing DNS information in the Router Advertisement Recursive DNS Server (RDNSS) and DNS Search List options. (§4.3) | {gap}, no test | RDNSS is advertised, but the Router Advertisement encoder has no DNS Search List option; internal/core/ndp/ra.go BuildRA |
| [`RFC7084-4.3-13`](#rfc7084-4.3-13) L-13: If the delegated prefix changes, i.e., the current prefix is replaced with a new prefix without any overlapping time period, then the IPv6 CE router MUST immediately advertise the old prefix with a Preferred Lifetime of zero and a Valid Lifetime of either a) zero or b) the lower of the current Valid Lifetime and two hours (which must be decremented in real time) in a Router Advertisement message as described in Section 5.5.3, (e) of [RFC4862]. (§4.3) | {gap}, no test | Router Advertisements carry only configured prefixes, so a replaced delegated prefix is never re-advertised with a zero Preferred Lifetime; internal/component/iface/config_ra.go raParsePrefixes |
| [`RFC7084-4.3-14`](#rfc7084-4.3-14) L-14: The IPv6 CE router MUST send an ICMPv6 Destination Unreachable message, code 5 (Source address failed ingress/egress policy) for packets forwarded to it that use an address from a prefix that has been invalidated. (§4.3) | {gap}, no test | no prefix is ever invalidated and nothing sends ICMPv6 Destination Unreachable code 5 for one |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7084-4.1-1`](#rfc7084-4.1-1)

G-2: The IPv6 CE router MUST implement ICMPv6 according to [RFC4443]. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7084-4.1-1, so no unit is bound to it.

### [`RFC7084-4.1-2`](#rfc7084-4.1-2)

In particular, point-to-point links MUST be handled as described in Section 3.1 of [RFC4443]. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7084-4.1-2, so no unit is bound to it.

### [`RFC7084-4.1-3`](#rfc7084-4.1-3)

G-3: The IPv6 CE router MUST NOT forward any IPv6 traffic between its LAN interface(s) and its WAN interface until the router has successfully completed the IPv6 address and the delegated prefix acquisition process. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7084-4.1-3, so no unit is bound to it.

### [`RFC7084-4.1-4`](#rfc7084-4.1-4)

G-4: By default, an IPv6 CE router that has no default router(s) on its WAN interface MUST NOT advertise itself as an IPv6 default router on its LAN interfaces. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7084-4.1-4, so no unit is bound to it.

### [`RFC7084-4.1-5`](#rfc7084-4.1-5)

G-5: By default, if the IPv6 CE router is an advertising router and loses its IPv6 default router(s) and/or detects loss of connectivity on the WAN interface, it MUST explicitly invalidate itself as an IPv6 default router on each of its advertising interfaces by immediately transmitting one or more Router Advertisement messages with the "Router Lifetime" field set to zero [RFC4861]. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7084-4.1-5, so no unit is bound to it.

### [`RFC7084-4.2-1`](#rfc7084-4.2-1)

W-1: When the router is attached to the WAN interface link, it MUST act as an IPv6 host for the purposes of stateless [RFC4862] or stateful [RFC3315] interface address assignment. (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7084WANUnitTurnsOnSLAAC`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc7084_ce_router_test.go#L29) | unit/verify | revert, verified |

### [`RFC7084-4.2-2`](#rfc7084-4.2-2)

W-2: The IPv6 CE router MUST generate a link-local address and finish Duplicate Address Detection according to [RFC4862] prior to sending any Router Solicitations on the interface. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7084-4.2-2, so no unit is bound to it.

### [`RFC7084-4.2-3`](#rfc7084-4.2-3)

The source address used in the subsequent Router Solicitation MUST be the link-local address on the WAN interface. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7084-4.2-3, so no unit is bound to it.

### [`RFC7084-4.2-4`](#rfc7084-4.2-4)

W-3: Absent other routing information, the IPv6 CE router MUST use Router Discovery as specified in [RFC4861] to discover a default router(s) and install a default route(s) in its routing table with the discovered router's address as the next hop. (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7084DiscoveredRouterBecomesTheDefaultRoute`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc7084_ce_router_test.go#L89) | unit/verify | revert, verified |

### [`RFC7084-4.2-5`](#rfc7084-4.2-5)

W-4: The router MUST act as a requesting router for the purposes of DHCPv6 prefix delegation ([RFC3633]). (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7084RequestingRouterSolicitsIAPD`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc7084_dhcp_v6_ce_linux_test.go#L32) | unit/verify | revert, verified |

### [`RFC7084-4.2-6`](#rfc7084-4.2-6)

W-5: The IPv6 CE router MUST use a persistent DHCP Unique Identifier (DUID) for DHCPv6 messages. (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestDHCPv6DefaultDUIDStable`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L620) | unit/verify | revert, verified |

### [`RFC7084-4.2-7`](#rfc7084-4.2-7)

The DUID MUST NOT change between network-interface resets or IPv6 CE router reboots. (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDHCPv6DefaultDUIDStable`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L621) | unit/verify | revert, verified |

### [`RFC7084-4.2-8`](#rfc7084-4.2-8)

WLL-1: If the WAN interface supports Ethernet encapsulation, then the IPv6 CE router MUST support IPv6 over Ethernet [RFC2464]. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7084-4.2-8, so no unit is bound to it.

### [`RFC7084-4.2-9`](#rfc7084-4.2-9)

WLL-2: If the WAN interface supports PPP encapsulation, the IPv6 CE router MUST support IPv6 over PPP [RFC5072]. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7084-4.2-9, so no unit is bound to it.

### [`RFC7084-4.2-10`](#rfc7084-4.2-10)

WLL-3: If the WAN interface supports PPP encapsulation, in a dual- stack environment with IPCP and IPV6CP running over one PPP logical channel, the Network Control Protocols (NCPs) MUST be treated as independent of each other and start and terminate independently. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7084-4.2-10, so no unit is bound to it.

### [`RFC7084-4.2-11`](#rfc7084-4.2-11)

WAA-1: The IPv6 CE router MUST support Stateless Address Autoconfiguration (SLAAC) [RFC4862]. (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7084WANUnitTurnsOnSLAAC`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc7084_ce_router_test.go#L28) | unit/verify | revert, verified |

### [`RFC7084-4.2-12`](#rfc7084-4.2-12)

WAA-2: The IPv6 CE router MUST follow the recommendations in Section 4 of [RFC5942], and in particular the handling of the L flag in the Router Advertisement Prefix Information option. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7084-4.2-12, so no unit is bound to it.

### [`RFC7084-4.2-13`](#rfc7084-4.2-13)

WAA-3: The IPv6 CE router MUST support DHCPv6 [RFC3315] client behavior. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7084-4.2-13, so no unit is bound to it.

### [`RFC7084-4.2-14`](#rfc7084-4.2-14)

WAA-4: The IPv6 CE router MUST be able to support the following DHCPv6 options: Identity Association for Non-temporary Address (IA_NA), Reconfigure Accept [RFC3315], and DNS_SERVERS [RFC3646]. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7084-4.2-14, so no unit is bound to it.

### [`RFC7084-4.2-15`](#rfc7084-4.2-15)

WAA-6: If the IPv6 CE router receives a Router Advertisement message (described in [RFC4861]) with the M flag set to 1, the IPv6 CE router MUST do DHCPv6 address assignment (request an IA_NA option). (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7084-4.2-15, so no unit is bound to it.

### [`RFC7084-4.2-16`](#rfc7084-4.2-16)

WAA-7: If the IPv6 CE router does not acquire a global IPv6 address(es) from either SLAAC or DHCPv6, then it MUST create a global IPv6 address(es) from its delegated prefix(es) and configure those on one of its internal virtual network interfaces, unless configured to require a global IPv6 address on the WAN interface. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7084-4.2-16, so no unit is bound to it.

### [`RFC7084-4.2-17`](#rfc7084-4.2-17)

WAA-8: The CE router MUST support the SOL_MAX_RT option [RFC7083] and request the SOL_MAX_RT option in an Option Request Option (ORO). (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7084SolMaxRTRequestedAndHonoured`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc7084_dhcp_v6_ce_linux_test.go#L55) | unit/verify | revert, verified |

### [`RFC7084-4.2-18`](#rfc7084-4.2-18)

WAA-9: As a router, the IPv6 CE router MUST follow the weak host (Weak End System) model [RFC1122]. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7084-4.2-18, so no unit is bound to it.

### [`RFC7084-4.2-19`](#rfc7084-4.2-19)

WPD-1: The IPv6 CE router MUST support DHCPv6 prefix delegation requesting router behavior as specified in [RFC3633] (Identity Association for Prefix Delegation (IA_PD) option). (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7084RequestingRouterSolicitsIAPD`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc7084_dhcp_v6_ce_linux_test.go#L33) | unit/verify | revert, verified |

### [`RFC7084-4.2-20`](#rfc7084-4.2-20)

If so, it MUST ask for a prefix large enough to assign one /64 for each of its interfaces, rounded up to the nearest nibble, and SHOULD be configurable to ask for more. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7084-4.2-20, so no unit is bound to it.

### [`RFC7084-4.2-21`](#rfc7084-4.2-21)

WPD-3: The IPv6 CE router MUST be prepared to accept a delegated prefix size different from what is given in the hint. (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7084DelegatedPrefixOtherThanHintWithoutAddresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc7084_dhcp_v6_ce_linux_test.go#L73) | unit/verify | revert, verified |

### [`RFC7084-4.2-22`](#rfc7084-4.2-22)

WPD-4: By default, the IPv6 CE router MUST initiate DHCPv6 prefix delegation when either the M or O flags are set to 1 in a received Router Advertisement (RA) message. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7084-4.2-22, so no unit is bound to it.

### [`RFC7084-4.2-23`](#rfc7084-4.2-23)

WPD-6: If the IPv6 CE router requests both an IA_NA and an IA_PD option in DHCPv6, it MUST accept an IA_PD option in DHCPv6 Advertise/Reply messages, even if the message does not contain any addresses, unless configured to only obtain its WAN IPv6 address via DHCPv6; see [DHCPv6-STATEFUL-ISSUES]. (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7084DelegatedPrefixOtherThanHintWithoutAddresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc7084_dhcp_v6_ce_linux_test.go#L74) | unit/verify | revert, verified |

### [`RFC7084-4.2-24`](#rfc7084-4.2-24)

WPD-7: By default, an IPv6 CE router MUST NOT initiate any dynamic routing protocol on its WAN interface. (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7084InterfaceOnlyConfigStartsNoRoutingProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/plugin/server/rfc7084_wan_routing_test.go#L26) | unit/verify | revert, verified |

### [`RFC7084-4.3-1`](#rfc7084-4.3-1)

ULA-2: An IPv6 CE router with a ULA prefix MUST maintain this prefix consistently across reboots. (§4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7084ULAPrefixComesFromTheConfigAlone`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc7084_ce_router_test.go#L71) | unit/verify | revert, verified |

### [`RFC7084-4.3-2`](#rfc7084-4.3-2)

ULA-4: By default, the IPv6 CE router MUST act as a site border router according to Section 4.3 of [RFC4193] and filter packets with local IPv6 source or destination addresses accordingly. (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7084-4.3-2, so no unit is bound to it.

### [`RFC7084-4.3-3`](#rfc7084-4.3-3)

ULA-5: An IPv6 CE router MUST NOT advertise itself as a default router with a Router Lifetime greater than zero whenever all of its configured and delegated prefixes are ULA prefixes. (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7084-4.3-3, so no unit is bound to it.

### [`RFC7084-4.3-4`](#rfc7084-4.3-4)

L-1: The IPv6 CE router MUST support router behavior according to Neighbor Discovery for IPv6 [RFC4861]. (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7084-4.3-4, so no unit is bound to it.

### [`RFC7084-4.3-5`](#rfc7084-4.3-5)

L-2: The IPv6 CE router MUST assign a separate /64 from its delegated prefix(es) (and ULA prefix if configured to provide ULA addressing) for each of its LAN interfaces. (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7084-4.3-5, so no unit is bound to it.

### [`RFC7084-4.3-6`](#rfc7084-4.3-6)

L-3: An IPv6 CE router MUST advertise itself as a router for the delegated prefix(es) (and ULA prefix if configured to provide ULA addressing) using the "Route Information Option" specified in Section 2.3 of [RFC4191]. (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7084-4.3-6, so no unit is bound to it.

### [`RFC7084-4.3-7`](#rfc7084-4.3-7)

L-4: An IPv6 CE router MUST NOT advertise itself as a default router with a Router Lifetime [RFC4861] greater than zero if it has no prefixes configured or delegated to it. (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7084-4.3-7, so no unit is bound to it.

### [`RFC7084-4.3-8`](#rfc7084-4.3-8)

L-5: The IPv6 CE router MUST make each LAN interface an advertising interface according to [RFC4861]. (§4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4861OnlyAnAdvertisingInterfaceSends`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc4861_router_configuration_test.go#L101) | unit/verify | revert, verified |

### [`RFC7084-4.3-9`](#rfc7084-4.3-9)

L-6: In Router Advertisement messages ([RFC4861]), the Prefix Information option's A and L flags MUST be set to 1 by default. (§4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRAConfigParse`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc7084_config_ra_test.go#L104) | unit/verify | revert, verified |

### [`RFC7084-4.3-10`](#rfc7084-4.3-10)

L-8: The IPv6 CE router MUST support a DHCPv6 server capable of IPv6 address assignment according to [RFC3315] OR a stateless DHCPv6 server according to [RFC3736] on its LAN interfaces. (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7084-4.3-10, so no unit is bound to it.

### [`RFC7084-4.3-11`](#rfc7084-4.3-11)

L-10: The IPv6 CE router MUST support providing DNS information in the DHCPv6 DNS_SERVERS and DOMAIN_LIST options [RFC3646]. (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7084-4.3-11, so no unit is bound to it.

### [`RFC7084-4.3-12`](#rfc7084-4.3-12)

L-11: The IPv6 CE router MUST support providing DNS information in the Router Advertisement Recursive DNS Server (RDNSS) and DNS Search List options. (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7084-4.3-12, so no unit is bound to it.

### [`RFC7084-4.3-13`](#rfc7084-4.3-13)

L-13: If the delegated prefix changes, i.e., the current prefix is replaced with a new prefix without any overlapping time period, then the IPv6 CE router MUST immediately advertise the old prefix with a Preferred Lifetime of zero and a Valid Lifetime of either a) zero or b) the lower of the current Valid Lifetime and two hours (which must be decremented in real time) in a Router Advertisement message as described in Section 5.5.3, (e) of [RFC4862]. (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7084-4.3-13, so no unit is bound to it.

### [`RFC7084-4.3-14`](#rfc7084-4.3-14)

L-14: The IPv6 CE router MUST send an ICMPv6 Destination Unreachable message, code 5 (Source address failed ingress/egress policy) for packets forwarded to it that use an address from a prefix that has been invalidated. (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7084-4.3-14, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc7084.txt |
| Source fingerprint | c4cebb97a6ddc13d |
| Record | rfc/extraction/rfc7084.json |
| Mapped sentences | 43 |
| Declined as scope | 12 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `3.2.1` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 5 | walked | not stated |
| `4.2` | not stated | 24 | walked | not stated |
| `4.3` | not stated | 14 | walked | not stated |
| `4.4` | not stated | 0 | walked | not stated |
| `4.4.1` | not stated | 8 | walked | not stated |
| `4.4.2` | not stated | 3 | walked | not stated |
| `4.5` | not stated | 1 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `7.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `4.4.1:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "The CE router SHOULD support 6rd functionality." The obligation applies only to a CE router that offers 6rd, and ze offers none: the DHCPv4 client asks for no 6rd option 212 (internal/plugins/iface/dhcp/dhcp_v4_linux.go DHCPClient.v4RequestModifiers adds only the host name and client identifier) and no interface derives a 6rd prefix. The absent feature is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc7084.md | If 6rd is supported, it MUST be implemented according to [RFC5969]. |
| `4.4.1:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "The CE router SHOULD support 6rd functionality." The obligation applies only to a CE router that offers 6rd, and ze offers none: the DHCPv4 client asks for no 6rd option 212 (internal/plugins/iface/dhcp/dhcp_v4_linux.go DHCPClient.v4RequestModifiers adds only the host name and client identifier) and no interface derives a 6rd prefix. The absent feature is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc7084.md | 6RD-1: The IPv6 CE router MUST support 6rd configuration via the 6rd DHCPv4 Option 212. |
| `4.4.1:3` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "The CE router SHOULD support 6rd functionality." The obligation applies only to a CE router that offers 6rd, and ze offers none: the DHCPv4 client asks for no 6rd option 212 (internal/plugins/iface/dhcp/dhcp_v4_linux.go DHCPClient.v4RequestModifiers adds only the host name and client identifier) and no interface derives a 6rd prefix. The absent feature is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc7084.md | 6RD-2: If the IPv6 CE router is capable of automated configuration of IPv4 through IPCP (i.e., over a PPP connection), it MUST support user-entered configuration of 6rd. |
| `4.4.1:4` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "The CE router SHOULD support 6rd functionality." The obligation applies only to a CE router that offers 6rd, and ze offers none: the DHCPv4 client asks for no 6rd option 212 (internal/plugins/iface/dhcp/dhcp_v4_linux.go DHCPClient.v4RequestModifiers adds only the host name and client identifier) and no interface derives a 6rd prefix. The absent feature is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc7084.md | 6RD-3: If the CE router supports configuration mechanisms other than the 6rd DHCPv4 Option 212 (user-entered, TR-069 [TR-069], etc.), the CE router MUST support 6rd in "hub and spoke" mode. 6rd in "hub and spoke" requires all IPv6 traffic to go to the 6rd Border Relay. |
| `4.4.1:5` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "The CE router SHOULD support 6rd functionality." The obligation applies only to a CE router that offers 6rd, and ze offers none: the DHCPv4 client asks for no 6rd option 212 (internal/plugins/iface/dhcp/dhcp_v4_linux.go DHCPClient.v4RequestModifiers adds only the host name and client identifier) and no interface derives a 6rd prefix. The absent feature is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc7084.md | 6RD-4: A CE router MUST allow 6rd and native IPv6 WAN interfaces to be active alone as well as simultaneously in order to support coexistence of the two technologies during an incremental migration period such as a migration from 6rd to native IPv6. |
| `4.4.1:6` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "The CE router SHOULD support 6rd functionality." The obligation applies only to a CE router that offers 6rd, and ze offers none: the DHCPv4 client asks for no 6rd option 212 (internal/plugins/iface/dhcp/dhcp_v4_linux.go DHCPClient.v4RequestModifiers adds only the host name and client identifier) and no interface derives a 6rd prefix. The absent feature is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc7084.md | 6RD-5: Each packet sent on a 6rd or native WAN interface MUST be directed such that its source IP address is derived from the delegated prefix associated with the particular interface from which the packet is being sent (Section 4.3 of [RFC3704]). |
| `4.4.1:7` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "The CE router SHOULD support 6rd functionality." The obligation applies only to a CE router that offers 6rd, and ze offers none: the DHCPv4 client asks for no 6rd option 212 (internal/plugins/iface/dhcp/dhcp_v4_linux.go DHCPClient.v4RequestModifiers adds only the host name and client identifier) and no interface derives a 6rd prefix. The absent feature is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc7084.md | 6RD-6: The CE router MUST allow different as well as identical delegated prefixes to be configured via each (6rd or native) WAN interface. |
| `4.4.1:8` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "The CE router SHOULD support 6rd functionality." The obligation applies only to a CE router that offers 6rd, and ze offers none: the DHCPv4 client asks for no 6rd option 212 (internal/plugins/iface/dhcp/dhcp_v4_linux.go DHCPClient.v4RequestModifiers adds only the host name and client identifier) and no interface derives a 6rd prefix. The absent feature is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc7084.md | 6RD-7: In the event that forwarding rules produce a tie between 6rd and native IPv6, by default, the IPv6 CE router MUST prefer native IPv6. |
| `4.4.2:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "The IPv6 CE router SHOULD implement DS-Lite functionality." The obligation applies only to a CE router that offers DS-Lite, and ze offers none: the DHCPv6 client asks for no AFTR-Name option 64 (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go requestOptions lists codes 23 and 82 only) and no B4 element exists. The absent feature is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc7084.md | If DS-Lite is supported, it MUST be implemented according to [RFC6333]. |
| `4.4.2:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "The IPv6 CE router SHOULD implement DS-Lite functionality." The obligation applies only to a CE router that offers DS-Lite, and ze offers none: the DHCPv6 client asks for no AFTR-Name option 64 (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go requestOptions lists codes 23 and 82 only) and no B4 element exists. The absent feature is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc7084.md | DLW-1: The CE router MUST support configuration of DS-Lite via the DS-Lite DHCPv6 option [RFC6334]. |
| `4.4.2:3` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "The IPv6 CE router SHOULD implement DS-Lite functionality." The obligation applies only to a CE router that offers DS-Lite, and ze offers none: the DHCPv6 client asks for no AFTR-Name option 64 (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go requestOptions lists codes 23 and 82 only) and no B4 element exists. The absent feature is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc7084.md | DLW-2: The IPv6 CE router MUST NOT perform IPv4 Network Address Translation (NAT) on IPv4 traffic encapsulated using DS-Lite. |
| `4.5:1` | `advisory-in-context` (never bound Ze): the sentence advises on applying a rule stated elsewhere and adds no obligation of its own | advisory: the sentence names the level RFC 6204 gave S-2. The requirement itself reads "S-2: The IPv6 CE router SHOULD support ingress filtering in accordance with BCP 38 [RFC2827].", so the MUST is not an obligation of this document | Note that this requirement was downgraded from a MUST from RFC 6204 due to the difficulty of implementation in the CE router and the feature's redundancy with upstream router ingress filtering. |

## Superseded

No document obsoletes RFC 7084, so its obligations are stated where they were written.
