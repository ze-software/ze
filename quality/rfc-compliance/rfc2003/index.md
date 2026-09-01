# RFC 2003 - IP Encapsulation within IP

No row in the public ledger. Every requirement this repository extracted from RFC 2003, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

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
| Gated MUSTs | 13 | of 36 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 13 | of 13 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

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
| Requirements | 36 |
| Gated MUST-level | 13 |
| Obligations that bind Ze | 0 |
| Not applicable, so out of scope | 13 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc2003.md` |
| Requirement shard | `rfc/requirements/rfc2003.md` |
| RFC text | `rfc/full/rfc2003.txt` |

## Enrolment

Enrolled: IP Encapsulation within IP (IP-in-IP, protocol 4): thirteen MUST-level requirements, all {not-applicable}. ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go setting IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go). The kernel ipip module and the VPP dataplane own the outer-header construction (Don't-Fragment copy, TTL-zero encap guard), decapsulation (inner-TTL-zero discard), source-address loop prevention, the ICMP relay/suppression rules, the Time-Exceeded-to-Host-Unreachable mapping, and path-MTU soft state. This is the same delegation rationale as the enrolled RFC 2784 and RFC 2890 (GRE).

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 2003.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 13 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **13** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (13):** [`RFC2003-3.1-1`](#rfc2003-3.1-1), [`RFC2003-3.1-2`](#rfc2003-3.1-2), [`RFC2003-3.1-3`](#rfc2003-3.1-3), [`RFC2003-3.2-1`](#rfc2003-3.2-1), [`RFC2003-3.2-2`](#rfc2003-3.2-2), [`RFC2003-4.1-1`](#rfc2003-4.1-1), [`RFC2003-4.1-2`](#rfc2003-4.1-2), [`RFC2003-4.1-3`](#rfc2003-4.1-3), [`RFC2003-4.1-4`](#rfc2003-4.1-4), [`RFC2003-4.4-1`](#rfc2003-4.4-1), [`RFC2003-4.3-1`](#rfc2003-4.3-1), [`RFC2003-4.5-1`](#rfc2003-4.5-1), [`RFC2003-5.1-1`](#rfc2003-5.1-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2003-3.1-1` | If the Don't Fragment bit is set in the inner IP header, it MUST be set in the outer IP header (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this outer-header/encapsulation obligation has no ze code path |
| `RFC2003-3.1-2` | An encapsulator MUST NOT encapsulate a datagram with TTL = 0 (§3.1) | MUST NOT | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this outer-header/encapsulation obligation has no ze code path |
| `RFC2003-3.1-3` | If after decapsulation the inner datagram has TTL = 0, the decapsulator MUST discard the datagram (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this decapsulation obligation has no ze code path |
| `RFC2003-3.2-1` | If IP Source Address of the datagram matches the router's own IP address, the router MUST NOT tunnel the datagram (§3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this tunnel loop-prevention check has no ze code path |
| `RFC2003-3.2-2` | If IP Source Address of the datagram matches the tunnel destination address, the router MUST NOT tunnel the datagram (§3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this tunnel loop-prevention check has no ze code path |
| `RFC2003-4.1-1` | The encapsulator MUST relay ICMP Datagram Too Big messages to the sender of the original unencapsulated datagram (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this ICMP-handling obligation has no ze code path |
| `RFC2003-4.1-2` | If returning Destination Unreachable for Network Unreachable and destination is not on the same network, the Code field MUST be set to 0 (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this ICMP-handling obligation has no ze code path |
| `RFC2003-4.1-3` | ICMP Port Unreachable MUST NOT be relayed to the sender of the original unencapsulated datagram (§4.1) | MUST NOT | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this ICMP-handling obligation has no ze code path |
| `RFC2003-4.1-4` | ICMP Source Route Failed MUST NOT be relayed to the sender of the original unencapsulated datagram (§4.1) | MUST NOT | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this ICMP-handling obligation has no ze code path |
| `RFC2003-4.4-1` | ICMP Time Exceeded MUST be reported to the sender as Host Unreachable (Type 3, Code 1) (§4.4) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this ICMP-handling obligation has no ze code path |
| `RFC2003-4.3-1` | ICMP Redirect MUST NOT be relayed to the sender of the original unencapsulated datagram (§4.3) | MUST NOT | 4.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this ICMP-handling obligation has no ze code path |
| `RFC2003-4.5-1` | If ICMP Parameter Problem points to a field inserted by the encapsulator, the message MUST NOT be relayed to the original sender (§4.5) | MUST NOT | 4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this ICMP-handling obligation has no ze code path |
| `RFC2003-5.1-1` | All encapsulator implementations MUST support Path MTU Discovery soft state within their tunnels (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this path-MTU soft-state obligation is set by the PMtuDisc flag ze programs (tunnel_linux.go:210-214) but maintained by the kernel/VPP dataplane, not by ze |
| `RFC2003-3.1-4` | If the resulting inner TTL is 0 after decrement, an ICMP Time Exceeded message SHOULD be returned to the sender (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-3.2-3` | Datagram with source address matching the router's own address SHOULD be discarded (§3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-3.2-4` | Datagram with source address matching the tunnel destination SHOULD be discarded (§3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-4.1-5` | ICMP Destination Unreachable (Network Unreachable, Code 0) SHOULD be returned to the original sender (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-4.1-6` | The encapsulator SHOULD relay Host Unreachable messages to the sender (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-4.1-7` | When the encapsulator receives ICMP Protocol Unreachable, it SHOULD send Destination Unreachable with Code 0 or 1 to the original sender (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-4.1-8` | ICMP Source Route Failed SHOULD be handled by the encapsulator itself (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-4.2-1` | The encapsulator SHOULD NOT relay ICMP Source Quench messages to the original sender (§4.2) | SHOULD NOT | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-4.2-2` | The encapsulator SHOULD activate congestion control mechanisms for Source Quench (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-5-1` | The encapsulator SHOULD maintain soft state about each tunnel: MTU, TTL, reachability (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-5.1-2` | The encapsulator SHOULD normally do Path MTU Discovery, setting DF in the outer header (§5.1) | SHOULD | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-5.1-3` | The MTU conveyed to the original sender SHOULD be the tunnel MTU minus the encapsulating IP header size (§5.1) | SHOULD | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-5.2-1` | The encapsulator SHOULD reflect congestion conditions in soft state for the tunnel (§5.2) | SHOULD | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-5.2-2` | The encapsulator SHOULD use appropriate means for controlling congestion when forwarding into the tunnel (§5.2) | SHOULD | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-5.2-3` | The encapsulator SHOULD NOT send ICMP Source Quench messages to the original sender (§5.2) | SHOULD NOT | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-6.2-1` | Host implementations receiving encapsulated datagrams SHOULD admit only those from authenticated, trusted sources or matching other security criteria (§6.2) | SHOULD | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-3.1-5` | If the Don't Fragment bit is not set in the inner header, it MAY be set in the outer header (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-3-1` | The security options of the inner IP header MAY affect the choice of security options for the outer header (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-3.1-6` | New options specific to the tunnel path MAY be added to the outer header (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-4-1` | The encapsulator MAY relay ICMP messages from within the tunnel to the original sender when enough information is available (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-4.3-2` | The encapsulator MAY handle ICMP Redirect messages itself (§4.3) | MAY | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-4.5-2` | The encapsulator MAY relay ICMP Parameter Problem to the original sender if it points to a field from the inner datagram (§4.5) | MAY | 4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-5.1-4` | The encapsulator MAY keep a copy of the sent datagram to allow fragmentation and resend on Datagram Too Big (§5.1) | MAY | 5.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2003-3.1-1`](#rfc2003-3.1-1) If the Don't Fragment bit is set in the inner IP header, it MUST be set in the outer IP header (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this outer-header/encapsulation obligation has no ze code path |
| [`RFC2003-3.1-2`](#rfc2003-3.1-2) An encapsulator MUST NOT encapsulate a datagram with TTL = 0 (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this outer-header/encapsulation obligation has no ze code path |
| [`RFC2003-3.1-3`](#rfc2003-3.1-3) If after decapsulation the inner datagram has TTL = 0, the decapsulator MUST discard the datagram (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this decapsulation obligation has no ze code path |
| [`RFC2003-3.2-1`](#rfc2003-3.2-1) If IP Source Address of the datagram matches the router's own IP address, the router MUST NOT tunnel the datagram (§3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this tunnel loop-prevention check has no ze code path |
| [`RFC2003-3.2-2`](#rfc2003-3.2-2) If IP Source Address of the datagram matches the tunnel destination address, the router MUST NOT tunnel the datagram (§3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this tunnel loop-prevention check has no ze code path |
| [`RFC2003-4.1-1`](#rfc2003-4.1-1) The encapsulator MUST relay ICMP Datagram Too Big messages to the sender of the original unencapsulated datagram (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this ICMP-handling obligation has no ze code path |
| [`RFC2003-4.1-2`](#rfc2003-4.1-2) If returning Destination Unreachable for Network Unreachable and destination is not on the same network, the Code field MUST be set to 0 (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this ICMP-handling obligation has no ze code path |
| [`RFC2003-4.1-3`](#rfc2003-4.1-3) ICMP Port Unreachable MUST NOT be relayed to the sender of the original unencapsulated datagram (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this ICMP-handling obligation has no ze code path |
| [`RFC2003-4.1-4`](#rfc2003-4.1-4) ICMP Source Route Failed MUST NOT be relayed to the sender of the original unencapsulated datagram (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this ICMP-handling obligation has no ze code path |
| [`RFC2003-4.4-1`](#rfc2003-4.4-1) ICMP Time Exceeded MUST be reported to the sender as Host Unreachable (Type 3, Code 1) (§4.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this ICMP-handling obligation has no ze code path |
| [`RFC2003-4.3-1`](#rfc2003-4.3-1) ICMP Redirect MUST NOT be relayed to the sender of the original unencapsulated datagram (§4.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this ICMP-handling obligation has no ze code path |
| [`RFC2003-4.5-1`](#rfc2003-4.5-1) If ICMP Parameter Problem points to a field inserted by the encapsulator, the message MUST NOT be relayed to the original sender (§4.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this ICMP-handling obligation has no ze code path |
| [`RFC2003-5.1-1`](#rfc2003-5.1-1) All encapsulator implementations MUST support Path MTU Discovery soft state within their tunnels (§5.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this path-MTU soft-state obligation is set by the PMtuDisc flag ze programs (tunnel_linux.go:210-214) but maintained by the kernel/VPP dataplane, not by ze |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2003-3.1-1`](#rfc2003-3.1-1)

If the Don't Fragment bit is set in the inner IP header, it MUST be set in the outer IP header (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2003-3.1-1, so no unit is bound to it.

### [`RFC2003-3.1-2`](#rfc2003-3.1-2)

An encapsulator MUST NOT encapsulate a datagram with TTL = 0 (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2003-3.1-2, so no unit is bound to it.

### [`RFC2003-3.1-3`](#rfc2003-3.1-3)

If after decapsulation the inner datagram has TTL = 0, the decapsulator MUST discard the datagram (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2003-3.1-3, so no unit is bound to it.

### [`RFC2003-3.2-1`](#rfc2003-3.2-1)

If IP Source Address of the datagram matches the router's own IP address, the router MUST NOT tunnel the datagram (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2003-3.2-1, so no unit is bound to it.

### [`RFC2003-3.2-2`](#rfc2003-3.2-2)

If IP Source Address of the datagram matches the tunnel destination address, the router MUST NOT tunnel the datagram (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2003-3.2-2, so no unit is bound to it.

### [`RFC2003-4.1-1`](#rfc2003-4.1-1)

The encapsulator MUST relay ICMP Datagram Too Big messages to the sender of the original unencapsulated datagram (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2003-4.1-1, so no unit is bound to it.

### [`RFC2003-4.1-2`](#rfc2003-4.1-2)

If returning Destination Unreachable for Network Unreachable and destination is not on the same network, the Code field MUST be set to 0 (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2003-4.1-2, so no unit is bound to it.

### [`RFC2003-4.1-3`](#rfc2003-4.1-3)

ICMP Port Unreachable MUST NOT be relayed to the sender of the original unencapsulated datagram (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2003-4.1-3, so no unit is bound to it.

### [`RFC2003-4.1-4`](#rfc2003-4.1-4)

ICMP Source Route Failed MUST NOT be relayed to the sender of the original unencapsulated datagram (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2003-4.1-4, so no unit is bound to it.

### [`RFC2003-4.4-1`](#rfc2003-4.4-1)

ICMP Time Exceeded MUST be reported to the sender as Host Unreachable (Type 3, Code 1) (§4.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2003-4.4-1, so no unit is bound to it.

### [`RFC2003-4.3-1`](#rfc2003-4.3-1)

ICMP Redirect MUST NOT be relayed to the sender of the original unencapsulated datagram (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2003-4.3-1, so no unit is bound to it.

### [`RFC2003-4.5-1`](#rfc2003-4.5-1)

If ICMP Parameter Problem points to a field inserted by the encapsulator, the message MUST NOT be relayed to the original sender (§4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2003-4.5-1, so no unit is bound to it.

### [`RFC2003-5.1-1`](#rfc2003-5.1-1)

All encapsulator implementations MUST support Path MTU Discovery soft state within their tunnels (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2003-5.1-1, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 2003, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 2003, so its obligations are stated where they were written.
