# RFC 4213 - Basic Transition Mechanisms for IPv6 Hosts and Routers

No row in the public ledger. Every requirement this repository extracted from RFC 4213, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 0 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 0 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 0 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 0 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 23 | of 47 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 23 | of 23 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

No card above is a share of a population, so there is nothing to add up.

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
| Public status | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 47 |
| Gated MUST-level | 23 |
| Obligations that bind Ze | 0 |
| Not applicable, so out of scope | 23 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
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
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 23 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **23** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (23):** [`RFC4213-2.2-1`](#rfc4213-2.2-1), [`RFC4213-2.2-2`](#rfc4213-2.2-2), [`RFC4213-3.2-1`](#rfc4213-3.2-1), [`RFC4213-3.2-2`](#rfc4213-3.2-2), [`RFC4213-3.2.1-1`](#rfc4213-3.2.1-1), [`RFC4213-3.2.1-2`](#rfc4213-3.2.1-2), [`RFC4213-3.2.1-3`](#rfc4213-3.2.1-3), [`RFC4213-3.2.1-4`](#rfc4213-3.2.1-4), [`RFC4213-3.2-3`](#rfc4213-3.2-3), [`RFC4213-3.6-1`](#rfc4213-3.6-1), [`RFC4213-3.6-2`](#rfc4213-3.6-2), [`RFC4213-3.6-3`](#rfc4213-3.6-3), [`RFC4213-3.6-4`](#rfc4213-3.6-4), [`RFC4213-3.6-5`](#rfc4213-3.6-5), [`RFC4213-3.6-6`](#rfc4213-3.6-6), [`RFC4213-3.6-7`](#rfc4213-3.6-7), [`RFC4213-3.7-1`](#rfc4213-3.7-1), [`RFC4213-3.8-1`](#rfc4213-3.8-1), [`RFC4213-3.8-2`](#rfc4213-3.8-2), [`RFC4213-5-1`](#rfc4213-5-1), [`RFC4213-5-2`](#rfc4213-5-2), [`RFC4213-5-3`](#rfc4213-5-3), [`RFC4213-5-4`](#rfc4213-5-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4213-2.2-1` | DNS resolver libraries on IPv6/IPv4 nodes MUST be capable of handling both AAAA and A records (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is a routing daemon, not a host stub-resolver library that applications call for dual-stack connection establishment; its resolve component exposes distinct operator-facing A and AAAA query verbs (ResolveA/ResolveAAAA, internal/component/resolve/dns/resolver.go:224), not a merged getaddrinfo that returns both families unfiltered, so this resolver-library obligation has no ze code path |
| `RFC4213-2.2-2` | If the application has requested both address families, the resolver library MUST NOT filter out any records (§2.2) | MUST NOT | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is a routing daemon, not a host stub-resolver library that applications call for dual-stack connection establishment; its resolve component exposes distinct operator-facing A and AAAA query verbs (ResolveA/ResolveAAAA, internal/component/resolve/dns/resolver.go:224), not a merged getaddrinfo that returns both families unfiltered, so this resolver-library obligation has no ze code path |
| `RFC4213-3.2-1` | Encapsulator MUST NOT treat the tunnel as having an MTU of 64 kilobytes; must use static or dynamic MTU determination (§3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit module owns the 6in4 datapath and ze runs no per-packet encapsulation/decapsulation code path (proof-of-absence: no 6in4 encap/decap producer outside the netlink config across internal/*.go), so this tunnel-MTU determination obligation has no ze code path |
| `RFC4213-3.2-2` | The naive scheme of viewing the tunnel as a very large MTU link MUST NOT be used (§3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit module owns the 6in4 datapath and ze runs no per-packet encapsulation/decapsulation code path (proof-of-absence: no 6in4 encap/decap producer outside the netlink config across internal/*.go), so this tunnel-MTU determination obligation has no ze code path |
| `RFC4213-3.2.1-1` | Static tunnel MTU: by default, the MTU MUST be between 1280 and 1480 bytes inclusive (§3.2.1) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit module derives and owns the static tunnel MTU; ze selects no tunnel MTU value and runs no per-packet encapsulation code path, so this static-MTU-range obligation has no ze code path |
| `RFC4213-3.2.1-2` | If the default static MTU is not 1280 bytes, the implementation MUST have a configuration knob to change it (§3.2.1) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the antecedent never fires in ze -- ze selects no static tunnel MTU default; it programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220), and the kernel sit module owns the tunnel MTU derivation. VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only), so this static-MTU-knob obligation has no ze code path |
| `RFC4213-3.2.1-3` | IPv4 reassembly and IPv6 MRU requirements MUST be supported by all decapsulators (§3.2.1) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit module owns IPv4 reassembly and the IPv6 MRU on the tunnel; ze runs no per-packet decapsulation code path, so this reassembly/MRU obligation has no ze code path |
| `RFC4213-3.2.1-4` | When using static tunnel MTU, the Don't Fragment bit MUST NOT be set in the encapsulating IPv4 header (§3.2.1) | MUST NOT | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41; the PMtuDisc flag is set at tunnel_linux.go:234-238 but the DF bit on the wire is written by the kernel sit datapath). VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). ze runs no per-packet encapsulation code path, so this outer-header DF obligation has no ze code path |
| `RFC4213-3.2-3` | Encapsulator MUST NOT treat the tunnel as an interface with an MTU of 64 kilobytes (§3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit module owns the 6in4 datapath and ze runs no per-packet encapsulation/decapsulation code path (proof-of-absence: no 6in4 encap/decap producer outside the netlink config across internal/*.go), so this tunnel-MTU determination obligation has no ze code path |
| `RFC4213-3.6-1` | The decapsulator MUST verify that the tunnel source address matches the configured remote endpoint (anti-spoofing) (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, setting the configured Remote endpoint); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath performs the per-packet outer-source verification against that configured remote; ze runs no per-packet decapsulation code path, so this decapsulation source-verification obligation has no ze code path |
| `RFC4213-3.6-2` | Packets for which the IPv4 source address does not match MUST be discarded (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, setting the configured Remote endpoint); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath discards packets whose outer IPv4 source does not match; ze runs no per-packet decapsulation code path, so this discard obligation has no ze code path |
| `RFC4213-3.6-3` | The decapsulator MUST be capable of having an IPv6 MRU of at least max(1500 bytes, largest IPv6 interface MTU) on tunnel interfaces (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath owns the IPv6 MRU on the tunnel; ze runs no per-packet decapsulation code path, so this MRU obligation has no ze code path |
| `RFC4213-3.6-4` | The decapsulator MUST be capable of reassembling an IPv4 packet up to max(1500 bytes, largest IPv4 interface MTU) (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath owns IPv4 reassembly on the tunnel; ze runs no per-packet decapsulation code path, so this reassembly obligation has no ze code path |
| `RFC4213-3.6-5` | Tunnel reassembly buffer MUST NOT be set below the required minimum (§3.6) | MUST NOT | 3.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath sizes the tunnel reassembly buffer; ze exposes no such buffer and runs no per-packet decapsulation code path, so this reassembly-buffer obligation has no ze code path |
| `RFC4213-3.6-6` | When reconstructing the IPv6 packet, the length MUST be determined from the IPv6 payload length, not the IPv4 length (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath reconstructs the inner IPv6 packet; ze runs no per-packet decapsulation code path, so this length-reconstruction obligation has no ze code path |
| `RFC4213-3.6-7` | After decapsulation, the node MUST silently discard packets with invalid IPv6 source addresses (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath silently discards inner packets with invalid IPv6 source addresses; ze runs no per-packet decapsulation code path, so this source-filtering obligation has no ze code path |
| `RFC4213-3.7-1` | Configured tunnels MUST have link-local addresses (§3.7) | MUST | 3.7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel assigns the IPv6 link-local address to the sit netdev when it comes up; ze writes no link-local address on the tunnel, so this link-local obligation has no ze code path |
| `RFC4213-3.8-1` | Configured tunnel implementations MUST at least accept and respond to NUD probe packets (§3.8) | MUST | 3.8 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel neighbor-discovery datapath accepts and responds to NUD probes on the tunnel; ze runs no per-packet ND code path on tunnels, so this NUD obligation has no ze code path |
| `RFC4213-3.8-2` | The receiver MUST silently ignore the content of any Source/Target Link Layer Address options received on the tunnel link (§3.8) | MUST | 3.8 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel neighbor-discovery datapath processes (and ignores) SLLA/TLLA options on the tunnel link; ze runs no per-packet ND code path on tunnels, so this option-handling obligation has no ze code path |
| `RFC4213-5-1` | IPv4 source address of the packet MUST be the same as configured for the tunnel end-point (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, setting the configured Remote endpoint); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath enforces the outer IPv4 source against the configured endpoint per packet; ze runs no per-packet decapsulation code path, so this source-verification obligation has no ze code path |
| `RFC4213-5-2` | IPv6 packets with obviously invalid source addresses received from the tunnel MUST be discarded (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath discards inner IPv6 packets with invalid source addresses; ze runs no per-packet decapsulation code path, so this source-filtering obligation has no ze code path |
| `RFC4213-5-3` | An implementation MUST treat interfaces to different links as separate (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs the sit tunnel as its own distinct netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, one link per configured tunnel); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel forwarding datapath keeps per-interface scope and treats the tunnel and native links as separate; ze runs no per-packet forwarding code path, so this per-interface separation obligation has no ze code path |
| `RFC4213-5-4` | Packets failing tunnel source address verification MUST be discarded (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, setting the configured Remote endpoint); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath discards packets that fail outer-source verification; ze runs no per-packet decapsulation code path, so this discard obligation has no ze code path |
| `RFC4213-3.2.1-5` | Static tunnel MTU SHOULD be 1280 bytes by default (§3.2.1) | SHOULD | 3.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.2-4` | If both static and dynamic MTU mechanisms are implemented, the choice SHOULD be configurable per tunnel endpoint (§3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.2.2-1` | If dynamic MTU is implemented, it SHOULD have the behavior described in the document (§3.2.2) | SHOULD | 3.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.2.2-2` | The encapsulator SHOULD employ the specified algorithm for dynamic MTU determination (§3.2.2) | SHOULD | 3.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.5-1` | It SHOULD be possible to administratively specify the source address of a tunnel (§3.5) | SHOULD | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.6-8` | An ICMP message SHOULD NOT be generated when discarding packets that fail source address verification (§3.6) | SHOULD NOT | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.6-9` | The list of invalid IPv6 source addresses SHOULD include at least multicast, loopback, IPv4-compatible, and IPv4-mapped addresses (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.8-3` | Implementations SHOULD send NUD probe packets to detect tunnel failures (§3.8) | SHOULD | 3.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.8-4` | The sender of Neighbor Discovery packets SHOULD NOT include Source/Target Link Layer Address options on the tunnel link (§3.8) | SHOULD NOT | 3.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-5-5` | An ICMP error message SHOULD NOT be generated for packets failing tunnel source verification (§5) | SHOULD NOT | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-2.2-3` | The applications SHOULD be able to specify whether they want IPv4, IPv6, or both records (§2.2) | SHOULD | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.6-10` | Packets caught by optional RPF check SHOULD be discarded (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.6-11` | An ICMP message SHOULD NOT be generated for packets caught by the RPF check (§3.6) | SHOULD NOT | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.6-12` | IPv4 ingress filtering (RPF check), if done, is RECOMMENDED to be disabled by default (§3.6) | RECOMMENDED | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.6-13` | Implementations SHOULD provide a single knob to enable strict ingress filtering toward edge networks (§3.6) | RECOMMENDED | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-2-1` | IPv6/IPv4 nodes MAY provide a configuration switch to disable either their IPv4 or IPv6 stack (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-2.2-4` | The resolver library MAY order results to influence IP version preference (§2.2) | MAY | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.3-1` | Implementations MAY provide a mechanism to allow the administrator to configure the IPv4 TTL (§3.3) | MAY | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.4-1` | Encapsulator MAY extract the encapsulated IPv6 packet to generate ICMPv6 errors (§3.4) | MAY | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.4-2` | Originating node MAY send an ICMPv6 "unreachable" error to the IPv6 source (§3.4) | MAY | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.6-14` | Implementation MAY perform IPv4 ingress filtering (RPF check) on tunnel packets (§3.6) | MAY | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.6-15` | Implementation MAY have a configuration knob to set larger tunnel reassembly buffers (§3.6) | MAY | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.6-16` | If the implementation normally sends ICMP for unknown protocols, such an error message MAY be sent (§3.6, §5) | MAY | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4213-3.2.2-3` | Implementations MAY use dynamic MTU determination (§3.2.2) | MAY | 3.2.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4213-2.2-1`](#rfc4213-2.2-1) DNS resolver libraries on IPv6/IPv4 nodes MUST be capable of handling both AAAA and A records (§2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze is a routing daemon, not a host stub-resolver library that applications call for dual-stack connection establishment; its resolve component exposes distinct operator-facing A and AAAA query verbs (ResolveA/ResolveAAAA, internal/component/resolve/dns/resolver.go:224), not a merged getaddrinfo that returns both families unfiltered, so this resolver-library obligation has no ze code path |
| [`RFC4213-2.2-2`](#rfc4213-2.2-2) If the application has requested both address families, the resolver library MUST NOT filter out any records (§2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze is a routing daemon, not a host stub-resolver library that applications call for dual-stack connection establishment; its resolve component exposes distinct operator-facing A and AAAA query verbs (ResolveA/ResolveAAAA, internal/component/resolve/dns/resolver.go:224), not a merged getaddrinfo that returns both families unfiltered, so this resolver-library obligation has no ze code path |
| [`RFC4213-3.2-1`](#rfc4213-3.2-1) Encapsulator MUST NOT treat the tunnel as having an MTU of 64 kilobytes; must use static or dynamic MTU determination (§3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit module owns the 6in4 datapath and ze runs no per-packet encapsulation/decapsulation code path (proof-of-absence: no 6in4 encap/decap producer outside the netlink config across internal/*.go), so this tunnel-MTU determination obligation has no ze code path |
| [`RFC4213-3.2-2`](#rfc4213-3.2-2) The naive scheme of viewing the tunnel as a very large MTU link MUST NOT be used (§3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit module owns the 6in4 datapath and ze runs no per-packet encapsulation/decapsulation code path (proof-of-absence: no 6in4 encap/decap producer outside the netlink config across internal/*.go), so this tunnel-MTU determination obligation has no ze code path |
| [`RFC4213-3.2.1-1`](#rfc4213-3.2.1-1) Static tunnel MTU: by default, the MTU MUST be between 1280 and 1480 bytes inclusive (§3.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit module derives and owns the static tunnel MTU; ze selects no tunnel MTU value and runs no per-packet encapsulation code path, so this static-MTU-range obligation has no ze code path |
| [`RFC4213-3.2.1-2`](#rfc4213-3.2.1-2) If the default static MTU is not 1280 bytes, the implementation MUST have a configuration knob to change it (§3.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: the antecedent never fires in ze -- ze selects no static tunnel MTU default; it programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220), and the kernel sit module owns the tunnel MTU derivation. VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only), so this static-MTU-knob obligation has no ze code path |
| [`RFC4213-3.2.1-3`](#rfc4213-3.2.1-3) IPv4 reassembly and IPv6 MRU requirements MUST be supported by all decapsulators (§3.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit module owns IPv4 reassembly and the IPv6 MRU on the tunnel; ze runs no per-packet decapsulation code path, so this reassembly/MRU obligation has no ze code path |
| [`RFC4213-3.2.1-4`](#rfc4213-3.2.1-4) When using static tunnel MTU, the Don't Fragment bit MUST NOT be set in the encapsulating IPv4 header (§3.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41; the PMtuDisc flag is set at tunnel_linux.go:234-238 but the DF bit on the wire is written by the kernel sit datapath). VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). ze runs no per-packet encapsulation code path, so this outer-header DF obligation has no ze code path |
| [`RFC4213-3.2-3`](#rfc4213-3.2-3) Encapsulator MUST NOT treat the tunnel as an interface with an MTU of 64 kilobytes (§3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit module owns the 6in4 datapath and ze runs no per-packet encapsulation/decapsulation code path (proof-of-absence: no 6in4 encap/decap producer outside the netlink config across internal/*.go), so this tunnel-MTU determination obligation has no ze code path |
| [`RFC4213-3.6-1`](#rfc4213-3.6-1) The decapsulator MUST verify that the tunnel source address matches the configured remote endpoint (anti-spoofing) (§3.6) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, setting the configured Remote endpoint); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath performs the per-packet outer-source verification against that configured remote; ze runs no per-packet decapsulation code path, so this decapsulation source-verification obligation has no ze code path |
| [`RFC4213-3.6-2`](#rfc4213-3.6-2) Packets for which the IPv4 source address does not match MUST be discarded (§3.6) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, setting the configured Remote endpoint); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath discards packets whose outer IPv4 source does not match; ze runs no per-packet decapsulation code path, so this discard obligation has no ze code path |
| [`RFC4213-3.6-3`](#rfc4213-3.6-3) The decapsulator MUST be capable of having an IPv6 MRU of at least max(1500 bytes, largest IPv6 interface MTU) on tunnel interfaces (§3.6) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath owns the IPv6 MRU on the tunnel; ze runs no per-packet decapsulation code path, so this MRU obligation has no ze code path |
| [`RFC4213-3.6-4`](#rfc4213-3.6-4) The decapsulator MUST be capable of reassembling an IPv4 packet up to max(1500 bytes, largest IPv4 interface MTU) (§3.6) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath owns IPv4 reassembly on the tunnel; ze runs no per-packet decapsulation code path, so this reassembly obligation has no ze code path |
| [`RFC4213-3.6-5`](#rfc4213-3.6-5) Tunnel reassembly buffer MUST NOT be set below the required minimum (§3.6) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath sizes the tunnel reassembly buffer; ze exposes no such buffer and runs no per-packet decapsulation code path, so this reassembly-buffer obligation has no ze code path |
| [`RFC4213-3.6-6`](#rfc4213-3.6-6) When reconstructing the IPv6 packet, the length MUST be determined from the IPv6 payload length, not the IPv4 length (§3.6) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath reconstructs the inner IPv6 packet; ze runs no per-packet decapsulation code path, so this length-reconstruction obligation has no ze code path |
| [`RFC4213-3.6-7`](#rfc4213-3.6-7) After decapsulation, the node MUST silently discard packets with invalid IPv6 source addresses (§3.6) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath silently discards inner packets with invalid IPv6 source addresses; ze runs no per-packet decapsulation code path, so this source-filtering obligation has no ze code path |
| [`RFC4213-3.7-1`](#rfc4213-3.7-1) Configured tunnels MUST have link-local addresses (§3.7) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel assigns the IPv6 link-local address to the sit netdev when it comes up; ze writes no link-local address on the tunnel, so this link-local obligation has no ze code path |
| [`RFC4213-3.8-1`](#rfc4213-3.8-1) Configured tunnel implementations MUST at least accept and respond to NUD probe packets (§3.8) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel neighbor-discovery datapath accepts and responds to NUD probes on the tunnel; ze runs no per-packet ND code path on tunnels, so this NUD obligation has no ze code path |
| [`RFC4213-3.8-2`](#rfc4213-3.8-2) The receiver MUST silently ignore the content of any Source/Target Link Layer Address options received on the tunnel link (§3.8) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel neighbor-discovery datapath processes (and ignores) SLLA/TLLA options on the tunnel link; ze runs no per-packet ND code path on tunnels, so this option-handling obligation has no ze code path |
| [`RFC4213-5-1`](#rfc4213-5-1) IPv4 source address of the packet MUST be the same as configured for the tunnel end-point (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, setting the configured Remote endpoint); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath enforces the outer IPv4 source against the configured endpoint per packet; ze runs no per-packet decapsulation code path, so this source-verification obligation has no ze code path |
| [`RFC4213-5-2`](#rfc4213-5-2) IPv6 packets with obviously invalid source addresses received from the tunnel MUST be discarded (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, Proto = IPPROTO_IPV6 / protocol 41); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath discards inner IPv6 packets with invalid source addresses; ze runs no per-packet decapsulation code path, so this source-filtering obligation has no ze code path |
| [`RFC4213-5-3`](#rfc4213-5-3) An implementation MUST treat interfaces to different links as separate (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs the sit tunnel as its own distinct netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, one link per configured tunnel); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel forwarding datapath keeps per-interface scope and treats the tunnel and native links as separate; ze runs no per-packet forwarding code path, so this per-interface separation obligation has no ze code path |
| [`RFC4213-5-4`](#rfc4213-5-4) Packets failing tunnel source address verification MUST be discarded (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs only the sit tunnel netdev via netlink buildSittun (internal/plugins/iface/netlink/tunnel_linux.go:220, setting the configured Remote endpoint); VPP carries no sit backend (internal/plugins/iface/vpp/tunnel.go:7, netlink-only). The kernel sit datapath discards packets that fail outer-source verification; ze runs no per-packet decapsulation code path, so this discard obligation has no ze code path |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4213-2.2-1`](#rfc4213-2.2-1)

DNS resolver libraries on IPv6/IPv4 nodes MUST be capable of handling both AAAA and A records (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-2.2-1, so no unit is bound to it.

### [`RFC4213-2.2-2`](#rfc4213-2.2-2)

If the application has requested both address families, the resolver library MUST NOT filter out any records (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-2.2-2, so no unit is bound to it.

### [`RFC4213-3.2-1`](#rfc4213-3.2-1)

Encapsulator MUST NOT treat the tunnel as having an MTU of 64 kilobytes; must use static or dynamic MTU determination (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.2-1, so no unit is bound to it.

### [`RFC4213-3.2-2`](#rfc4213-3.2-2)

The naive scheme of viewing the tunnel as a very large MTU link MUST NOT be used (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.2-2, so no unit is bound to it.

### [`RFC4213-3.2.1-1`](#rfc4213-3.2.1-1)

Static tunnel MTU: by default, the MTU MUST be between 1280 and 1480 bytes inclusive (§3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.2.1-1, so no unit is bound to it.

### [`RFC4213-3.2.1-2`](#rfc4213-3.2.1-2)

If the default static MTU is not 1280 bytes, the implementation MUST have a configuration knob to change it (§3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.2.1-2, so no unit is bound to it.

### [`RFC4213-3.2.1-3`](#rfc4213-3.2.1-3)

IPv4 reassembly and IPv6 MRU requirements MUST be supported by all decapsulators (§3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.2.1-3, so no unit is bound to it.

### [`RFC4213-3.2.1-4`](#rfc4213-3.2.1-4)

When using static tunnel MTU, the Don't Fragment bit MUST NOT be set in the encapsulating IPv4 header (§3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.2.1-4, so no unit is bound to it.

### [`RFC4213-3.2-3`](#rfc4213-3.2-3)

Encapsulator MUST NOT treat the tunnel as an interface with an MTU of 64 kilobytes (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.2-3, so no unit is bound to it.

### [`RFC4213-3.6-1`](#rfc4213-3.6-1)

The decapsulator MUST verify that the tunnel source address matches the configured remote endpoint (anti-spoofing) (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.6-1, so no unit is bound to it.

### [`RFC4213-3.6-2`](#rfc4213-3.6-2)

Packets for which the IPv4 source address does not match MUST be discarded (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.6-2, so no unit is bound to it.

### [`RFC4213-3.6-3`](#rfc4213-3.6-3)

The decapsulator MUST be capable of having an IPv6 MRU of at least max(1500 bytes, largest IPv6 interface MTU) on tunnel interfaces (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.6-3, so no unit is bound to it.

### [`RFC4213-3.6-4`](#rfc4213-3.6-4)

The decapsulator MUST be capable of reassembling an IPv4 packet up to max(1500 bytes, largest IPv4 interface MTU) (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.6-4, so no unit is bound to it.

### [`RFC4213-3.6-5`](#rfc4213-3.6-5)

Tunnel reassembly buffer MUST NOT be set below the required minimum (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.6-5, so no unit is bound to it.

### [`RFC4213-3.6-6`](#rfc4213-3.6-6)

When reconstructing the IPv6 packet, the length MUST be determined from the IPv6 payload length, not the IPv4 length (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.6-6, so no unit is bound to it.

### [`RFC4213-3.6-7`](#rfc4213-3.6-7)

After decapsulation, the node MUST silently discard packets with invalid IPv6 source addresses (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.6-7, so no unit is bound to it.

### [`RFC4213-3.7-1`](#rfc4213-3.7-1)

Configured tunnels MUST have link-local addresses (§3.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.7-1, so no unit is bound to it.

### [`RFC4213-3.8-1`](#rfc4213-3.8-1)

Configured tunnel implementations MUST at least accept and respond to NUD probe packets (§3.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.8-1, so no unit is bound to it.

### [`RFC4213-3.8-2`](#rfc4213-3.8-2)

The receiver MUST silently ignore the content of any Source/Target Link Layer Address options received on the tunnel link (§3.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-3.8-2, so no unit is bound to it.

### [`RFC4213-5-1`](#rfc4213-5-1)

IPv4 source address of the packet MUST be the same as configured for the tunnel end-point (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-5-1, so no unit is bound to it.

### [`RFC4213-5-2`](#rfc4213-5-2)

IPv6 packets with obviously invalid source addresses received from the tunnel MUST be discarded (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-5-2, so no unit is bound to it.

### [`RFC4213-5-3`](#rfc4213-5-3)

An implementation MUST treat interfaces to different links as separate (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-5-3, so no unit is bound to it.

### [`RFC4213-5-4`](#rfc4213-5-4)

Packets failing tunnel source address verification MUST be discarded (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4213-5-4, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 4213, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 4213, so its obligations are stated where they were written.
