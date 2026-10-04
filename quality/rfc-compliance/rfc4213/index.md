# RFC 4213 - Basic Transition Mechanisms for IPv6 Hosts and Routers

No row in the public ledger. Every requirement this repository extracted from RFC 4213, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 4.3% | 1 of 23 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 23 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 23 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 23 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 23 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 100.0% | 2 of 2 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 23 | of 47 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 22 | of 23 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 95.7% | 22 of 23 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 23 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 23 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 23 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 47 |
| Gated MUST-level | 23 |
| Not applicable, so out of scope | 22 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 2 |
| Tagged units | 2 |
| Recorded audit verdicts | 0 |
| Discrimination records | 2 |
| Summary | `rfc/short/rfc4213.md` |
| Requirement shard | `rfc/requirements/rfc4213.md` |
| RFC text | `rfc/full/rfc4213.txt` |

## Enrolment

Enrolled: Basic Transition Mechanisms for IPv6 Hosts and Routers (RFC 4213): dual-stack plus configured 6in4 tunnels (protocol 41). All 23 gated MUSTs not-applicable -- ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go, Proto = IPPROTO_IPV6) and carries no sit VPP backend (netlink-only). The kernel sit module owns the 6in4 datapath (encapsulation, decapsulation, outer-source verification, MTU/fragmentation, link-local assignment, neighbor discovery); the two DNS section 2.2 obligations belong to a host stub-resolver library ze does not provide. Same delegation pattern as enrolled RFC 2003 and RFC 2473

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 4213.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the gated population |
| Annotated (including scoped evidence) | 22 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **23** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (1):** [`RFC4213-3.6-1`](#rfc4213-3.6-1)

**Annotated (including scoped evidence) (22):** [`RFC4213-2.2-1`](#rfc4213-2.2-1), [`RFC4213-2.2-2`](#rfc4213-2.2-2), [`RFC4213-3.2-1`](#rfc4213-3.2-1), [`RFC4213-3.2-2`](#rfc4213-3.2-2), [`RFC4213-3.2.1-1`](#rfc4213-3.2.1-1), [`RFC4213-3.2.1-2`](#rfc4213-3.2.1-2), [`RFC4213-3.2.1-3`](#rfc4213-3.2.1-3), [`RFC4213-3.2.1-4`](#rfc4213-3.2.1-4), [`RFC4213-3.2-3`](#rfc4213-3.2-3), [`RFC4213-3.6-2`](#rfc4213-3.6-2), [`RFC4213-3.6-3`](#rfc4213-3.6-3), [`RFC4213-3.6-4`](#rfc4213-3.6-4), [`RFC4213-3.6-5`](#rfc4213-3.6-5), [`RFC4213-3.6-6`](#rfc4213-3.6-6), [`RFC4213-3.6-7`](#rfc4213-3.6-7), [`RFC4213-3.7-1`](#rfc4213-3.7-1), [`RFC4213-3.8-1`](#rfc4213-3.8-1), [`RFC4213-3.8-2`](#rfc4213-3.8-2), [`RFC4213-5-1`](#rfc4213-5-1), [`RFC4213-5-2`](#rfc4213-5-2), [`RFC4213-5-3`](#rfc4213-5-3), [`RFC4213-5-4`](#rfc4213-5-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4213-2.2-1` | DNS resolver libraries on IPv6/IPv4 nodes MUST be capable of handling both AAAA and A records (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is a routing daemon, not a host stub-resolver library that applications call for dual-stack connection establishment; its resolve component exposes distinct operator-facing A and AAAA query verbs (ResolveA/ResolveAAAA, internal/component/resolve/dns/resolver.go:224), not a merged getaddrinfo that returns both families unfiltered, so this resolver-library obligation has no ze code path |
| `RFC4213-2.2-2` | If there is not an application choice, or if the application has requested both, the resolver library MUST NOT filter out any records. (§2.2) | MUST NOT | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is a routing daemon, not a host stub-resolver library that applications call for dual-stack connection establishment; its resolve component exposes distinct operator-facing A and AAAA query verbs (ResolveA/ResolveAAAA, internal/component/resolve/dns/resolver.go:224), not a merged getaddrinfo that returns both families unfiltered, so this resolver-library obligation has no ze code path |
| `RFC4213-3.2-1` | the encapsulator MUST NOT treat the tunnel as an interface with an MTU of 64 kilobytes, but instead either use the fixed static MTU or OPTIONAL dynamic MTU determination based on the IPv4 path MTU to the tunnel endpoint. (§3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit module owns the 6in4 datapath and ze runs no per-packet encapsulation/decapsulation code path (proof-of-absence: no 6in4 encap/decap producer outside the netlink config across internal/*.go), so this tunnel-MTU determination obligation has no ze code path |
| `RFC4213-3.2-2` | Naively, the encapsulator could view encapsulation as IPv6 using IPv4 as a link layer with a very large MTU (65535-20 bytes at most; 20 bytes "extra" are needed for the encapsulating IPv4 header). The encapsulator would only need to report ICMPv6 "packet too big" errors back to the source for packets that exceed this MTU. However, such a scheme would be inefficient or non-interoperable for three reasons and therefore MUST NOT be used (§3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit module owns the 6in4 datapath and ze runs no per-packet encapsulation/decapsulation code path (proof-of-absence: no 6in4 encap/decap producer outside the netlink config across internal/*.go), so this tunnel-MTU determination obligation has no ze code path |
| `RFC4213-3.2.1-1` | By default, the MTU MUST be between 1280 and 1480 bytes (inclusive) (§3.2.1) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit module derives and owns the static tunnel MTU; ze selects no tunnel MTU value and runs no per-packet encapsulation code path, so this static-MTU-range obligation has no ze code path |
| `RFC4213-3.2.1-2` | If the default is not 1280 bytes, the implementation MUST have a configuration knob that can be used to change the MTU value. (§3.2.1) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the antecedent never fires in ze -- ze selects no static tunnel MTU default; it programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220), and the kernel sit module owns the tunnel MTU derivation. VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only), so this static-MTU-knob obligation has no ze code path |
| `RFC4213-3.2.1-3` | This memo also includes requirements (see Section 3.6) for the amount of IPv4 reassembly and IPv6 MRU that MUST be supported by all the decapsulators. (§3.2.1) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit module owns IPv4 reassembly and the IPv6 MRU on the tunnel; ze runs no per-packet decapsulation code path, so this reassembly/MRU obligation has no ze code path |
| `RFC4213-3.2.1-4` | When using the static tunnel MTU, the Don't Fragment bit MUST NOT be set in the encapsulating IPv4 header. (§3.2.1) | MUST NOT | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41; the PMtuDisc flag is set at tunnel_linux.go:234-238 but the DF bit on the wire is written by the kernel sit datapath). VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). ze runs no per-packet encapsulation code path, so this outer-header DF obligation has no ze code path |
| `RFC4213-3.2-3` | the encapsulator MUST NOT treat the tunnel as an interface with an MTU of 64 kilobytes (§3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit module owns the 6in4 datapath and ze runs no per-packet encapsulation/decapsulation code path (proof-of-absence: no 6in4 encap/decap producer outside the netlink config across internal/*.go), so this tunnel-MTU determination obligation has no ze code path |
| `RFC4213-3.6-1` | The decapsulator MUST verify that the tunnel source address is correct before further processing packets, to mitigate the problems with address spoofing (see Section 4). This check also applies to packets that are delivered to transport protocols on the decapsulator. This is done by verifying that the source address is the IPv4 address of the encapsulator, as configured on the decapsulator. (§3.6) | MUST | 3.6 | **positive:** `unit/verify` [`TestRFC4213SitDecapsulatesFromTheConfiguredRemote`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/netlink/tunnel_rfc_integration_linux_test.go#L324). **negative:** `unit/verify` [`TestRFC4213SitRefusesAnUnverifiedSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/netlink/tunnel_rfc_integration_linux_test.go#L354) |
| `RFC4213-3.6-2` | Packets for which the IPv4 source address does not match MUST be discarded (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, setting the configured Remote endpoint); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath discards packets whose outer IPv4 source does not match; ze runs no per-packet decapsulation code path, so this discard obligation has no ze code path |
| `RFC4213-3.6-3` | The decapsulator MUST be capable of having, on the tunnel interfaces, an IPv6 MRU of at least the maximum of 1500 bytes and the largest (IPv6) interface MTU on the decapsulator. (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath owns the IPv6 MRU on the tunnel; ze runs no per-packet decapsulation code path, so this MRU obligation has no ze code path |
| `RFC4213-3.6-4` | The decapsulator MUST be capable of reassembling an IPv4 packet that is (after the reassembly) the maximum of 1500 bytes and the largest (IPv4) interface MTU on the decapsulator. (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath owns IPv4 reassembly on the tunnel; ze runs no per-packet decapsulation code path, so this reassembly obligation has no ze code path |
| `RFC4213-3.6-5` | An implementation MAY have a configuration knob that can be used to set a larger value of the tunnel reassembly buffers than the above number, but it MUST NOT be set below the above number. (§3.6) | MUST NOT | 3.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath sizes the tunnel reassembly buffer; ze exposes no such buffer and runs no per-packet decapsulation code path, so this reassembly-buffer obligation has no ze code path |
| `RFC4213-3.6-6` | When reconstructing the IPv6 packet, the length MUST be determined from the IPv6 payload length since the IPv4 packet might be padded (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath reconstructs the inner IPv6 packet; ze runs no per-packet decapsulation code path, so this length-reconstruction obligation has no ze code path |
| `RFC4213-3.6-7` | After the decapsulation, the node MUST silently discard a packet with an invalid IPv6 source address. (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath silently discards inner packets with invalid IPv6 source addresses; ze runs no per-packet decapsulation code path, so this source-filtering obligation has no ze code path |
| `RFC4213-3.7-1` | The configured tunnels are IPv6 interfaces (over the IPv4 "link layer") and thus MUST have link-local addresses. (§3.7) | MUST | 3.7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel assigns the IPv6 link-local address to the sit netdev when it comes up; ze writes no link-local address on the tunnel, so this link-local obligation has no ze code path |
| `RFC4213-3.8-1` | Configured tunnel implementations MUST at least accept and respond to the probe packets used by Neighbor Unreachability Detection (NUD) [RFC2461]. (§3.8) | MUST | 3.8 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel neighbor-discovery datapath accepts and responds to NUD probes on the tunnel; ze runs no per-packet ND code path on tunnels, so this NUD obligation has no ze code path |
| `RFC4213-3.8-2` | - the receiver MUST, while otherwise processing the Neighbor Discovery packet, silently ignore the content of any Source Link Layer Address options or Target Link Layer Address options received on the tunnel link. (§3.8) | MUST | 3.8 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel neighbor-discovery datapath processes (and ignores) SLLA/TLLA options on the tunnel link; ze runs no per-packet ND code path on tunnels, so this option-handling obligation has no ze code path |
| `RFC4213-5-1` | IPv4 source address of the packet MUST be the same as configured for the tunnel end-point (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, setting the configured Remote endpoint); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath enforces the outer IPv4 source against the configured endpoint per packet; ze runs no per-packet decapsulation code path, so this source-verification obligation has no ze code path |
| `RFC4213-5-2` | - IPv6 packets with several, obviously invalid IPv6 source addresses received from the tunnel MUST be discarded (see Section 3.6 for details); and (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath discards inner IPv6 packets with invalid source addresses; ze runs no per-packet decapsulation code path, so this source-filtering obligation has no ze code path |
| `RFC4213-5-3` | an implementation MUST treat interfaces to different links as separate (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs the sit tunnel as its own distinct netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, one link per configured tunnel); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel forwarding datapath keeps per-interface scope and treats the tunnel and native links as separate; ze runs no per-packet forwarding code path, so this per-interface separation obligation has no ze code path |
| `RFC4213-5-4` | When dropping packets due to failing to match the allowed IPv4 source addresses for a tunnel the node should not "acknowledge" the existence of a tunnel, otherwise this could be used to probe the acceptable tunnel endpoint addresses. For that reason, the specification says that such packets MUST be discarded (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, setting the configured Remote endpoint); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath discards packets that fail outer-source verification; ze runs no per-packet decapsulation code path, so this discard obligation has no ze code path |
| `RFC4213-3.2.1-5` | By default, the MTU MUST be between 1280 and 1480 bytes (inclusive), but it SHOULD be 1280 bytes. (§3.2.1) | SHOULD | 3.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.2-4` | If both the mechanisms are implemented, the decision of which to use SHOULD be configurable on a per-tunnel endpoint basis. (§3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.2.2-1` | The dynamic MTU determination is OPTIONAL. However, if it is implemented, it SHOULD have the behavior described in this document. (§3.2.2) | SHOULD | 3.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.2.2-2` | The encapsulator SHOULD employ the following algorithm to determine when to forward an IPv6 packet that is larger than the tunnel's path MTU using IPv4 fragmentation, and when to return an ICMPv6 "packet too big" message (§3.2.2) | SHOULD | 3.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.5-1` | it SHOULD be possible to administratively specify the source address of a tunnel. (§3.5) | SHOULD | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.6-8` | Packets for which the IPv4 source address does not match MUST be discarded and an ICMP message SHOULD NOT be generated (§3.6) | SHOULD NOT | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.6-9` | The list of invalid source addresses SHOULD include at least: - all multicast addresses (FF00::/8) - the loopback address (::1) - all the IPv4-compatible IPv6 addresses [RFC3513] (::/96), excluding the unspecified address for Duplicate Address Detection (::/128) - all the IPv4-mapped IPv6 addresses (::ffff:0:0/96) (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.8-3` | The implementations SHOULD also send NUD probe packets to detect when the configured tunnel fails (§3.8) | SHOULD | 3.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.8-4` | the sender of Neighbor Discovery packets SHOULD NOT include Source Link Layer Address options or Target Link Layer Address options on the tunnel link. (§3.8) | SHOULD NOT | 3.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-5-5` | When dropping packets due to failing to match the allowed IPv4 source addresses for a tunnel the node should not "acknowledge" the existence of a tunnel, otherwise this could be used to probe the acceptable tunnel endpoint addresses. For that reason, the specification says that such packets MUST be discarded, and an ICMP error message SHOULD NOT be generated (§5) | SHOULD NOT | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-2.2-3` | The applications SHOULD be able to specify whether they want IPv4, IPv6, or both records (§2.2) | SHOULD | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.6-10` | the implementation MAY perform ingress filtering, i.e., check that the packet is arriving from the interface in the direction of the route toward the tunnel end-point, similar to a Strict Reverse Path Forwarding (RPF) check [RFC3704]. As this may cause problems on tunnels that are routed through multiple links, it is RECOMMENDED that this check, if done, is disabled by default. The packets caught by this check SHOULD be discarded (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.6-11` | the implementation MAY perform ingress filtering, i.e., check that the packet is arriving from the interface in the direction of the route toward the tunnel end-point, similar to a Strict Reverse Path Forwarding (RPF) check [RFC3704]. As this may cause problems on tunnels that are routed through multiple links, it is RECOMMENDED that this check, if done, is disabled by default. The packets caught by this check SHOULD be discarded; an ICMP message SHOULD NOT be generated by default. (§3.6) | SHOULD NOT | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.6-12` | the implementation MAY perform ingress filtering, i.e., check that the packet is arriving from the interface in the direction of the route toward the tunnel end-point, similar to a Strict Reverse Path Forwarding (RPF) check [RFC3704]. As this may cause problems on tunnels that are routed through multiple links, it is RECOMMENDED that this check, if done, is disabled by default. (§3.6) | RECOMMENDED | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.6-13` | It is RECOMMENDED that the implementations provide a single knob to make it easier to for the administrators to enable strict ingress filtering toward edge networks. (§3.6) | RECOMMENDED | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-2-1` | IPv6/IPv4 nodes MAY provide a configuration switch to disable either their IPv4 or IPv6 stack (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-2.2-4` | when a query locates an AAAA record holding an IPv6 address, and an A record holding an IPv4 address, the resolver library MAY order the results returned to the application in order to influence the version of IP packets used to communicate with that specific node -- IPv6 first, or IPv4 first. (§2.2) | MAY | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.3-1` | Implementations MAY provide a mechanism to allow the administrator to configure the IPv4 TTL (§3.3) | MAY | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.4-1` | If sufficient data bytes from the offending packet are available, the encapsulator MAY extract the encapsulated IPv6 packet and use it to generate an ICMPv6 message directed back to the originating IPv6 node (§3.4) | MAY | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.4-2` | if sufficient headers are available, then the originating node MAY send an ICMPv6 error of type "unreachable" with code "address unreachable" to the IPv6 source. (§3.4) | MAY | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.6-14` | the implementation MAY perform ingress filtering, i.e., check that the packet is arriving from the interface in the direction of the route toward the tunnel end-point, similar to a Strict Reverse Path Forwarding (RPF) check [RFC3704]. (§3.6) | MAY | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.6-15` | An implementation MAY have a configuration knob that can be used to set a larger value of the tunnel reassembly buffers than the above number (§3.6) | MAY | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.6-16` | however, if the implementation normally sends an ICMP message when receiving an unknown protocol packet, such an error message MAY be sent (e.g., ICMPv4 Protocol 41 Unreachable). (§3.6) | MAY | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.2.2-3` | The dynamic MTU determination is OPTIONAL. (§3.2.2) | OPTIONAL | 3.2.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4213-2.2-1`](#rfc4213-2.2-1) DNS resolver libraries on IPv6/IPv4 nodes MUST be capable of handling both AAAA and A records (§2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze is a routing daemon, not a host stub-resolver library that applications call for dual-stack connection establishment; its resolve component exposes distinct operator-facing A and AAAA query verbs (ResolveA/ResolveAAAA, internal/component/resolve/dns/resolver.go:224), not a merged getaddrinfo that returns both families unfiltered, so this resolver-library obligation has no ze code path |
| [`RFC4213-2.2-2`](#rfc4213-2.2-2) If there is not an application choice, or if the application has requested both, the resolver library MUST NOT filter out any records. (§2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze is a routing daemon, not a host stub-resolver library that applications call for dual-stack connection establishment; its resolve component exposes distinct operator-facing A and AAAA query verbs (ResolveA/ResolveAAAA, internal/component/resolve/dns/resolver.go:224), not a merged getaddrinfo that returns both families unfiltered, so this resolver-library obligation has no ze code path |
| [`RFC4213-3.2-1`](#rfc4213-3.2-1) the encapsulator MUST NOT treat the tunnel as an interface with an MTU of 64 kilobytes, but instead either use the fixed static MTU or OPTIONAL dynamic MTU determination based on the IPv4 path MTU to the tunnel endpoint. (§3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit module owns the 6in4 datapath and ze runs no per-packet encapsulation/decapsulation code path (proof-of-absence: no 6in4 encap/decap producer outside the netlink config across internal/*.go), so this tunnel-MTU determination obligation has no ze code path |
| [`RFC4213-3.2-2`](#rfc4213-3.2-2) Naively, the encapsulator could view encapsulation as IPv6 using IPv4 as a link layer with a very large MTU (65535-20 bytes at most; 20 bytes "extra" are needed for the encapsulating IPv4 header). The encapsulator would only need to report ICMPv6 "packet too big" errors back to the source for packets that exceed this MTU. However, such a scheme would be inefficient or non-interoperable for three reasons and therefore MUST NOT be used (§3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit module owns the 6in4 datapath and ze runs no per-packet encapsulation/decapsulation code path (proof-of-absence: no 6in4 encap/decap producer outside the netlink config across internal/*.go), so this tunnel-MTU determination obligation has no ze code path |
| [`RFC4213-3.2.1-1`](#rfc4213-3.2.1-1) By default, the MTU MUST be between 1280 and 1480 bytes (inclusive) (§3.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit module derives and owns the static tunnel MTU; ze selects no tunnel MTU value and runs no per-packet encapsulation code path, so this static-MTU-range obligation has no ze code path |
| [`RFC4213-3.2.1-2`](#rfc4213-3.2.1-2) If the default is not 1280 bytes, the implementation MUST have a configuration knob that can be used to change the MTU value. (§3.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: the antecedent never fires in ze -- ze selects no static tunnel MTU default; it programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220), and the kernel sit module owns the tunnel MTU derivation. VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only), so this static-MTU-knob obligation has no ze code path |
| [`RFC4213-3.2.1-3`](#rfc4213-3.2.1-3) This memo also includes requirements (see Section 3.6) for the amount of IPv4 reassembly and IPv6 MRU that MUST be supported by all the decapsulators. (§3.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit module owns IPv4 reassembly and the IPv6 MRU on the tunnel; ze runs no per-packet decapsulation code path, so this reassembly/MRU obligation has no ze code path |
| [`RFC4213-3.2.1-4`](#rfc4213-3.2.1-4) When using the static tunnel MTU, the Don't Fragment bit MUST NOT be set in the encapsulating IPv4 header. (§3.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41; the PMtuDisc flag is set at tunnel_linux.go:234-238 but the DF bit on the wire is written by the kernel sit datapath). VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). ze runs no per-packet encapsulation code path, so this outer-header DF obligation has no ze code path |
| [`RFC4213-3.2-3`](#rfc4213-3.2-3) the encapsulator MUST NOT treat the tunnel as an interface with an MTU of 64 kilobytes (§3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit module owns the 6in4 datapath and ze runs no per-packet encapsulation/decapsulation code path (proof-of-absence: no 6in4 encap/decap producer outside the netlink config across internal/*.go), so this tunnel-MTU determination obligation has no ze code path |
| [`RFC4213-3.6-2`](#rfc4213-3.6-2) Packets for which the IPv4 source address does not match MUST be discarded (§3.6) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, setting the configured Remote endpoint); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath discards packets whose outer IPv4 source does not match; ze runs no per-packet decapsulation code path, so this discard obligation has no ze code path |
| [`RFC4213-3.6-3`](#rfc4213-3.6-3) The decapsulator MUST be capable of having, on the tunnel interfaces, an IPv6 MRU of at least the maximum of 1500 bytes and the largest (IPv6) interface MTU on the decapsulator. (§3.6) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath owns the IPv6 MRU on the tunnel; ze runs no per-packet decapsulation code path, so this MRU obligation has no ze code path |
| [`RFC4213-3.6-4`](#rfc4213-3.6-4) The decapsulator MUST be capable of reassembling an IPv4 packet that is (after the reassembly) the maximum of 1500 bytes and the largest (IPv4) interface MTU on the decapsulator. (§3.6) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath owns IPv4 reassembly on the tunnel; ze runs no per-packet decapsulation code path, so this reassembly obligation has no ze code path |
| [`RFC4213-3.6-5`](#rfc4213-3.6-5) An implementation MAY have a configuration knob that can be used to set a larger value of the tunnel reassembly buffers than the above number, but it MUST NOT be set below the above number. (§3.6) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath sizes the tunnel reassembly buffer; ze exposes no such buffer and runs no per-packet decapsulation code path, so this reassembly-buffer obligation has no ze code path |
| [`RFC4213-3.6-6`](#rfc4213-3.6-6) When reconstructing the IPv6 packet, the length MUST be determined from the IPv6 payload length since the IPv4 packet might be padded (§3.6) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath reconstructs the inner IPv6 packet; ze runs no per-packet decapsulation code path, so this length-reconstruction obligation has no ze code path |
| [`RFC4213-3.6-7`](#rfc4213-3.6-7) After the decapsulation, the node MUST silently discard a packet with an invalid IPv6 source address. (§3.6) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath silently discards inner packets with invalid IPv6 source addresses; ze runs no per-packet decapsulation code path, so this source-filtering obligation has no ze code path |
| [`RFC4213-3.7-1`](#rfc4213-3.7-1) The configured tunnels are IPv6 interfaces (over the IPv4 "link layer") and thus MUST have link-local addresses. (§3.7) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel assigns the IPv6 link-local address to the sit netdev when it comes up; ze writes no link-local address on the tunnel, so this link-local obligation has no ze code path |
| [`RFC4213-3.8-1`](#rfc4213-3.8-1) Configured tunnel implementations MUST at least accept and respond to the probe packets used by Neighbor Unreachability Detection (NUD) [RFC2461]. (§3.8) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel neighbor-discovery datapath accepts and responds to NUD probes on the tunnel; ze runs no per-packet ND code path on tunnels, so this NUD obligation has no ze code path |
| [`RFC4213-3.8-2`](#rfc4213-3.8-2) - the receiver MUST, while otherwise processing the Neighbor Discovery packet, silently ignore the content of any Source Link Layer Address options or Target Link Layer Address options received on the tunnel link. (§3.8) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel neighbor-discovery datapath processes (and ignores) SLLA/TLLA options on the tunnel link; ze runs no per-packet ND code path on tunnels, so this option-handling obligation has no ze code path |
| [`RFC4213-5-1`](#rfc4213-5-1) IPv4 source address of the packet MUST be the same as configured for the tunnel end-point (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, setting the configured Remote endpoint); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath enforces the outer IPv4 source against the configured endpoint per packet; ze runs no per-packet decapsulation code path, so this source-verification obligation has no ze code path |
| [`RFC4213-5-2`](#rfc4213-5-2) - IPv6 packets with several, obviously invalid IPv6 source addresses received from the tunnel MUST be discarded (see Section 3.6 for details); and (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath discards inner IPv6 packets with invalid source addresses; ze runs no per-packet decapsulation code path, so this source-filtering obligation has no ze code path |
| [`RFC4213-5-3`](#rfc4213-5-3) an implementation MUST treat interfaces to different links as separate (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs the sit tunnel as its own distinct netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, one link per configured tunnel); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel forwarding datapath keeps per-interface scope and treats the tunnel and native links as separate; ze runs no per-packet forwarding code path, so this per-interface separation obligation has no ze code path |
| [`RFC4213-5-4`](#rfc4213-5-4) When dropping packets due to failing to match the allowed IPv4 source addresses for a tunnel the node should not "acknowledge" the existence of a tunnel, otherwise this could be used to probe the acceptable tunnel endpoint addresses. For that reason, the specification says that such packets MUST be discarded (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, setting the configured Remote endpoint); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath discards packets that fail outer-source verification; ze runs no per-packet decapsulation code path, so this discard obligation has no ze code path |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4213-2.2-1`](#rfc4213-2.2-1)

DNS resolver libraries on IPv6/IPv4 nodes MUST be capable of handling both AAAA and A records (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-2.2-1, so no unit is bound to it.

### [`RFC4213-2.2-2`](#rfc4213-2.2-2)

If there is not an application choice, or if the application has requested both, the resolver library MUST NOT filter out any records. (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-2.2-2, so no unit is bound to it.

### [`RFC4213-3.2-1`](#rfc4213-3.2-1)

the encapsulator MUST NOT treat the tunnel as an interface with an MTU of 64 kilobytes, but instead either use the fixed static MTU or OPTIONAL dynamic MTU determination based on the IPv4 path MTU to the tunnel endpoint. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.2-1, so no unit is bound to it.

### [`RFC4213-3.2-2`](#rfc4213-3.2-2)

Naively, the encapsulator could view encapsulation as IPv6 using IPv4 as a link layer with a very large MTU (65535-20 bytes at most; 20 bytes "extra" are needed for the encapsulating IPv4 header). The encapsulator would only need to report ICMPv6 "packet too big" errors back to the source for packets that exceed this MTU. However, such a scheme would be inefficient or non-interoperable for three reasons and therefore MUST NOT be used (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.2-2, so no unit is bound to it.

### [`RFC4213-3.2.1-1`](#rfc4213-3.2.1-1)

By default, the MTU MUST be between 1280 and 1480 bytes (inclusive) (§3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.2.1-1, so no unit is bound to it.

### [`RFC4213-3.2.1-2`](#rfc4213-3.2.1-2)

If the default is not 1280 bytes, the implementation MUST have a configuration knob that can be used to change the MTU value. (§3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.2.1-2, so no unit is bound to it.

### [`RFC4213-3.2.1-3`](#rfc4213-3.2.1-3)

This memo also includes requirements (see Section 3.6) for the amount of IPv4 reassembly and IPv6 MRU that MUST be supported by all the decapsulators. (§3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.2.1-3, so no unit is bound to it.

### [`RFC4213-3.2.1-4`](#rfc4213-3.2.1-4)

When using the static tunnel MTU, the Don't Fragment bit MUST NOT be set in the encapsulating IPv4 header. (§3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.2.1-4, so no unit is bound to it.

### [`RFC4213-3.2-3`](#rfc4213-3.2-3)

the encapsulator MUST NOT treat the tunnel as an interface with an MTU of 64 kilobytes (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.2-3, so no unit is bound to it.

### [`RFC4213-3.6-1`](#rfc4213-3.6-1)

The decapsulator MUST verify that the tunnel source address is correct before further processing packets, to mitigate the problems with address spoofing (see Section 4). This check also applies to packets that are delivered to transport protocols on the decapsulator. This is done by verifying that the source address is the IPv4 address of the encapsulator, as configured on the decapsulator. (§3.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4213SitRefusesAnUnverifiedSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/netlink/tunnel_rfc_integration_linux_test.go#L354) | unit/verify | revert, verified |
| positive | [`TestRFC4213SitDecapsulatesFromTheConfiguredRemote`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/netlink/tunnel_rfc_integration_linux_test.go#L324) | unit/verify | revert, verified |

### [`RFC4213-3.6-2`](#rfc4213-3.6-2)

Packets for which the IPv4 source address does not match MUST be discarded (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.6-2, so no unit is bound to it.

### [`RFC4213-3.6-3`](#rfc4213-3.6-3)

The decapsulator MUST be capable of having, on the tunnel interfaces, an IPv6 MRU of at least the maximum of 1500 bytes and the largest (IPv6) interface MTU on the decapsulator. (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.6-3, so no unit is bound to it.

### [`RFC4213-3.6-4`](#rfc4213-3.6-4)

The decapsulator MUST be capable of reassembling an IPv4 packet that is (after the reassembly) the maximum of 1500 bytes and the largest (IPv4) interface MTU on the decapsulator. (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.6-4, so no unit is bound to it.

### [`RFC4213-3.6-5`](#rfc4213-3.6-5)

An implementation MAY have a configuration knob that can be used to set a larger value of the tunnel reassembly buffers than the above number, but it MUST NOT be set below the above number. (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.6-5, so no unit is bound to it.

### [`RFC4213-3.6-6`](#rfc4213-3.6-6)

When reconstructing the IPv6 packet, the length MUST be determined from the IPv6 payload length since the IPv4 packet might be padded (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.6-6, so no unit is bound to it.

### [`RFC4213-3.6-7`](#rfc4213-3.6-7)

After the decapsulation, the node MUST silently discard a packet with an invalid IPv6 source address. (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.6-7, so no unit is bound to it.

### [`RFC4213-3.7-1`](#rfc4213-3.7-1)

The configured tunnels are IPv6 interfaces (over the IPv4 "link layer") and thus MUST have link-local addresses. (§3.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.7-1, so no unit is bound to it.

### [`RFC4213-3.8-1`](#rfc4213-3.8-1)

Configured tunnel implementations MUST at least accept and respond to the probe packets used by Neighbor Unreachability Detection (NUD) [RFC2461]. (§3.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.8-1, so no unit is bound to it.

### [`RFC4213-3.8-2`](#rfc4213-3.8-2)

- the receiver MUST, while otherwise processing the Neighbor Discovery packet, silently ignore the content of any Source Link Layer Address options or Target Link Layer Address options received on the tunnel link. (§3.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.8-2, so no unit is bound to it.

### [`RFC4213-5-1`](#rfc4213-5-1)

IPv4 source address of the packet MUST be the same as configured for the tunnel end-point (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-5-1, so no unit is bound to it.

### [`RFC4213-5-2`](#rfc4213-5-2)

- IPv6 packets with several, obviously invalid IPv6 source addresses received from the tunnel MUST be discarded (see Section 3.6 for details); and (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-5-2, so no unit is bound to it.

### [`RFC4213-5-3`](#rfc4213-5-3)

an implementation MUST treat interfaces to different links as separate (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-5-3, so no unit is bound to it.

### [`RFC4213-5-4`](#rfc4213-5-4)

When dropping packets due to failing to match the allowed IPv4 source addresses for a tunnel the node should not "acknowledge" the existence of a tunnel, otherwise this could be used to probe the acceptable tunnel endpoint addresses. For that reason, the specification says that such packets MUST be discarded (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-5-4, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc4213.txt |
| Source fingerprint | 00b5a45164be1272 |
| Record | rfc/extraction/rfc4213.json |
| Mapped sentences | 22 |
| Declined as scope | 2 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 2 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 2 | walked | not stated |
| `3.2.1` | not stated | 4 | walked | not stated |
| `3.2.2` | not stated | 0 | walked | not stated |
| `3.3` | not stated | 0 | walked | not stated |
| `3.4` | not stated | 0 | walked | not stated |
| `3.5` | not stated | 0 | walked | not stated |
| `3.6` | not stated | 7 | walked | not stated |
| `3.7` | not stated | 1 | walked | not stated |
| `3.8` | not stated | 2 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 4 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `7.2` | not stated | 0 | walked | not stated |
| `8` | not stated | 2 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `8:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 8 is the changelog from RFC 2893. The sentence restates the source-address check obligation already mapped at site 3.6:1 (decapsulator MUST verify the tunnel source address) and at 5:2 (invalid IPv6 source addresses MUST be discarded). | - Added stronger wording for source address checks: both IPv4 and IPv6 source addresses MUST be checked, and RPF-like ingress filtering is optional. |
| `8:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 8 changelog entry restating the section 2.2 resolver obligation already mapped at site 2.2:2 (the resolver library MUST NOT filter out any records). | - Removed/clarified DNS record filtering; an API is a SHOULD and if it does not exist, MUST NOT filter anything. |

## Superseded

No document obsoletes RFC 4213, so its obligations are stated where they were written.
