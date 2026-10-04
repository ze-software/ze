# RFC 2003 - IP Encapsulation within IP

No row in the public ledger. Every requirement this repository extracted from RFC 2003, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 13 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 13 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 13 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 13 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 13 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 13 | of 37 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (third-party), so every share below reads what the summary records rather than what the gate enforces |
| Out of scope | 13 | of 13 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 100.0% | 13 of 13 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 13 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 13 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 13 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| MUSTs declared | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
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
| Enrolment | Not enrolled (third-party) |
| Requirements | 37 |
| Gated MUST-level | 13 |
| Not applicable, so out of scope | 13 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
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

Not enrolled (third-party, a layer under or beside Ze performs the document and Ze holds no Go code for it, so the reason beside this kind names the component that does): The Linux ipip module builds and parses every outer header. Ze only builds the netlink link descriptor, internal/plugins/iface/netlink/tunnel_linux.go::buildIptun.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 2003.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated (including scoped evidence) | 13 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **13** | every gated MUST falls in exactly one bucket above |

**Annotated (including scoped evidence) (13):** [`RFC2003-3.1-1`](#rfc2003-3.1-1), [`RFC2003-3.1-2`](#rfc2003-3.1-2), [`RFC2003-3.1-3`](#rfc2003-3.1-3), [`RFC2003-3.2-1`](#rfc2003-3.2-1), [`RFC2003-3.2-2`](#rfc2003-3.2-2), [`RFC2003-4.1-1`](#rfc2003-4.1-1), [`RFC2003-4.1-2`](#rfc2003-4.1-2), [`RFC2003-4.1-3`](#rfc2003-4.1-3), [`RFC2003-4.1-4`](#rfc2003-4.1-4), [`RFC2003-4.4-1`](#rfc2003-4.4-1), [`RFC2003-4.3-1`](#rfc2003-4.3-1), [`RFC2003-4.5-1`](#rfc2003-4.5-1), [`RFC2003-5.1-1`](#rfc2003-5.1-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2003-3.1-1` | if the "Don't Fragment" bit is set in the inner IP header, it MUST be set in the outer IP header (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this outer-header/encapsulation obligation has no ze code path |
| `RFC2003-3.1-2` | An encapsulator MUST NOT encapsulate a datagram with TTL = 0 (§3.1) | MUST NOT | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this outer-header/encapsulation obligation has no ze code path |
| `RFC2003-3.1-3` | If, after decapsulation, the inner datagram has TTL = 0, the decapsulator MUST discard the datagram. (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this decapsulation obligation has no ze code path |
| `RFC2003-3.2-1` | If the IP Source Address of the datagram matches the router's own IP address on any of its network interfaces, the router MUST NOT tunnel the datagram (§3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this tunnel loop-prevention check has no ze code path |
| `RFC2003-3.2-2` | If the IP Source Address of the datagram matches the IP address of the tunnel destination (the tunnel exit point is typically chosen by the router based on the Destination Address in the datagram's IP header), the router MUST NOT tunnel the datagram (§3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this tunnel loop-prevention check has no ze code path |
| `RFC2003-4.1-1` | The encapsulator MUST relay ICMP Datagram Too Big messages to the sender of the original unencapsulated datagram (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this ICMP-handling obligation has no ze code path |
| `RFC2003-4.1-2` | If the original destination in the unencapsulated datagram is on the same network as the encapsulator, the newly generated Destination Unreachable message sent by the encapsulator MAY have Code 1 (Host Unreachable), since presumably the datagram arrived at the correct network and the encapsulator is trying to create the appearance that the original destination is local to that network even if it is not. Otherwise, if the encapsulator returns a Destination Unreachable message, the Code field MUST be set to 0 (Network Unreachable). (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this ICMP-handling obligation has no ze code path |
| `RFC2003-4.1-3` | It MUST NOT be relayed to the sender of the original unencapsulated datagram. (§4.1) | MUST NOT | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this ICMP-handling obligation has no ze code path |
| `RFC2003-4.1-4` | It MUST NOT be relayed to the sender of the original unencapsulated datagram. (§4.1) | MUST NOT | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this ICMP-handling obligation has no ze code path |
| `RFC2003-4.4-1` | Reception of Time Exceeded messages by the encapsulator MUST be reported to the sender of the original unencapsulated datagram as Host Unreachable (Type 3, Code 1). (§4.4) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this ICMP-handling obligation has no ze code path |
| `RFC2003-4.3-1` | It MUST NOT not relay the Redirect to the sender of the original unencapsulated datagram. (§4.3) | MUST NOT | 4.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this ICMP-handling obligation has no ze code path |
| `RFC2003-4.5-1` | if the problem occurs with an IP option inserted by the encapsulator, then the encapsulator MUST NOT relay the ICMP message to the original sender. (§4.5) | MUST NOT | 4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this ICMP-handling obligation has no ze code path |
| `RFC2003-5.1-1` | To support sending nodes which use Path MTU Discovery, all encapsulator implementations MUST support Path MTU Discovery [5, 7] soft state within their tunnels. (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this path-MTU soft-state obligation is set by the PMtuDisc flag ze programs (tunnel_linux.go:210-214) but maintained by the kernel/VPP dataplane, not by ze |
| `RFC2003-3.1-4` | If the resulting TTL in the inner IP header is 0, the datagram is discarded and an ICMP Time Exceeded message SHOULD be returned to the sender. (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-3.2-3` | If the IP Source Address of the datagram matches the router's own IP address on any of its network interfaces, the router MUST NOT tunnel the datagram; instead, the datagram SHOULD be discarded. (§3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-3.2-4` | If the IP Source Address of the datagram matches the IP address of the tunnel destination (the tunnel exit point is typically chosen by the router based on the Destination Address in the datagram's IP header), the router MUST NOT tunnel the datagram; instead, the datagram SHOULD be discarded. (§3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-4.1-5` | Network Unreachable (Code 0) An ICMP Destination Unreachable message SHOULD be returned to the original sender. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-4.1-6` | The encapsulator SHOULD relay Host Unreachable messages to the sender (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-4.1-7` | When the encapsulator receives an ICMP Protocol Unreachable message, it SHOULD send a Destination Unreachable message with Code 0 or 1 (see the discussion for Code 0) to the sender of the original unencapsulated datagram. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-4.1-8` | Source Route Failed (Code 5) This Code SHOULD be handled by the encapsulator itself. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-4.2-1` | The encapsulator SHOULD NOT relay ICMP Source Quench messages to the sender of the original unencapsulated datagram (§4.2) | SHOULD NOT | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-4.2-2` | The encapsulator SHOULD NOT relay ICMP Source Quench messages to the sender of the original unencapsulated datagram, but instead SHOULD activate whatever congestion control mechanisms it implements to help alleviate the congestion detected within the tunnel. (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-5-1` | The encapsulator SHOULD maintain at least the following soft state information about each tunnel: - MTU of the tunnel (Section 5.1) - TTL (path length) of the tunnel - Reachability of the end of the tunnel (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-5.1-2` | the encapsulator SHOULD normally do Path MTU Discovery, requiring it to send all datagrams into the tunnel with the "Don't Fragment" bit set in the outer IP header. (§5.1) | SHOULD | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-5.1-3` | the MTU that is conveyed to the original sender by the encapsulator SHOULD be the MTU of the tunnel minus the size of the encapsulating IP header. (§5.1) | SHOULD | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-5.2-1` | The encapsulator SHOULD reflect conditions of congestion in its "soft state" for the tunnel (§5.2) | SHOULD | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-5.2-2` | when subsequently forwarding datagrams into the tunnel, the encapsulator SHOULD use appropriate means for controlling congestion (§5.2) | SHOULD | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-5.2-3` | the encapsulator SHOULD NOT send ICMP Source Quench messages to the original sender of the unencapsulated datagram. (§5.2) | SHOULD NOT | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-6.2-1` | Host implementations that are capable of receiving encapsulated IP datagrams SHOULD admit only those datagrams fitting into one or more of the following categories: - The protocol is harmless: source address-based authentication is not needed. - The encapsulating (outer) datagram comes from an authentically identified, trusted source. The authenticity of the source could be established by relying on physical security in addition to border router configuration, but is more likely to come from use of the IP Authentication header [1]. - The encapuslated (inner) datagram includes an IP Authentication header. - The encapsulated (inner) datagram is addressed to a network interface belonging to the decapsulator, or to a node with which the decapsulator has entered into a special relationship for delivering such encapsulated datagrams. (§6.2) | SHOULD | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-3.1-5` | if the "Don't Fragment" bit is not set in the inner IP header, it MAY be set in the outer IP header, as described in Section 5.1. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-3-1` | the security options of the inner IP header MAY affect the choice of security options for the encapsulating (outer) IP header. (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-3.1-6` | new options specific to the tunnel path MAY be added. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-4-1` | When the received message contains enough information, the encapsulator MAY use the incoming message to create a similar ICMP message, to be sent to the originator of the original unencapsulated IP datagram (the original sender). (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-4.3-2` | The encapsulator MAY handle the ICMP Redirect messages itself. (§4.3) | MAY | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-4.5-2` | If the Parameter Problem message points to a field copied from the original unencapsulated datagram, the encapsulator MAY relay the ICMP message to the sender of the original unencapsulated datagram (§4.5) | MAY | 4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-5.1-4` | The encapsulator MAY keep a copy of the sent datagram whenever it tries increasing the tunnel MTU, in order to allow it to fragment and resend the datagram if it gets a Datagram Too Big response. (§5.1) | MAY | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2003-5.1-5` | the encapsulator MAY be configured for certain types of datagrams not to set the "Don't Fragment" bit when the original sender of the unencapsulated datagram has not set the "Don't Fragment" bit. (§5.1) | MAY | 5.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2003-3.1-1`](#rfc2003-3.1-1) if the "Don't Fragment" bit is set in the inner IP header, it MUST be set in the outer IP header (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this outer-header/encapsulation obligation has no ze code path |
| [`RFC2003-3.1-2`](#rfc2003-3.1-2) An encapsulator MUST NOT encapsulate a datagram with TTL = 0 (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this outer-header/encapsulation obligation has no ze code path |
| [`RFC2003-3.1-3`](#rfc2003-3.1-3) If, after decapsulation, the inner datagram has TTL = 0, the decapsulator MUST discard the datagram. (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this decapsulation obligation has no ze code path |
| [`RFC2003-3.2-1`](#rfc2003-3.2-1) If the IP Source Address of the datagram matches the router's own IP address on any of its network interfaces, the router MUST NOT tunnel the datagram (§3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this tunnel loop-prevention check has no ze code path |
| [`RFC2003-3.2-2`](#rfc2003-3.2-2) If the IP Source Address of the datagram matches the IP address of the tunnel destination (the tunnel exit point is typically chosen by the router based on the Destination Address in the datagram's IP header), the router MUST NOT tunnel the datagram (§3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this tunnel loop-prevention check has no ze code path |
| [`RFC2003-4.1-1`](#rfc2003-4.1-1) The encapsulator MUST relay ICMP Datagram Too Big messages to the sender of the original unencapsulated datagram (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this ICMP-handling obligation has no ze code path |
| [`RFC2003-4.1-2`](#rfc2003-4.1-2) If the original destination in the unencapsulated datagram is on the same network as the encapsulator, the newly generated Destination Unreachable message sent by the encapsulator MAY have Code 1 (Host Unreachable), since presumably the datagram arrived at the correct network and the encapsulator is trying to create the appearance that the original destination is local to that network even if it is not. Otherwise, if the encapsulator returns a Destination Unreachable message, the Code field MUST be set to 0 (Network Unreachable). (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this ICMP-handling obligation has no ze code path |
| [`RFC2003-4.1-3`](#rfc2003-4.1-3) It MUST NOT be relayed to the sender of the original unencapsulated datagram. (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this ICMP-handling obligation has no ze code path |
| [`RFC2003-4.1-4`](#rfc2003-4.1-4) It MUST NOT be relayed to the sender of the original unencapsulated datagram. (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this ICMP-handling obligation has no ze code path |
| [`RFC2003-4.4-1`](#rfc2003-4.4-1) Reception of Time Exceeded messages by the encapsulator MUST be reported to the sender of the original unencapsulated datagram as Host Unreachable (Type 3, Code 1). (§4.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this ICMP-handling obligation has no ze code path |
| [`RFC2003-4.3-1`](#rfc2003-4.3-1) It MUST NOT not relay the Redirect to the sender of the original unencapsulated datagram. (§4.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this ICMP-handling obligation has no ze code path |
| [`RFC2003-4.5-1`](#rfc2003-4.5-1) if the problem occurs with an IP option inserted by the encapsulator, then the encapsulator MUST NOT relay the ICMP message to the original sender. (§4.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this ICMP-handling obligation has no ze code path |
| [`RFC2003-5.1-1`](#rfc2003-5.1-1) To support sending nodes which use Path MTU Discovery, all encapsulator implementations MUST support Path MTU Discovery [5, 7] soft state within their tunnels. (§5.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze constructs no IP-in-IP header and runs no encapsulation, decapsulation, ICMP-relay, or loop-prevention datapath: it programs only the tunnel configuration via netlink buildIptun (internal/plugins/iface/netlink/tunnel_linux.go:196, Proto IPPROTO_IPIP) and VPP ipip_add_tunnel (internal/plugins/iface/vpp/tunnel.go:113), and the kernel ipip module and VPP dataplane own the outer-header construction, decapsulation, ICMP handling, path-MTU soft state, and loop prevention, so this path-MTU soft-state obligation is set by the PMtuDisc flag ze programs (tunnel_linux.go:210-214) but maintained by the kernel/VPP dataplane, not by ze |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2003-3.1-1`](#rfc2003-3.1-1)

if the "Don't Fragment" bit is set in the inner IP header, it MUST be set in the outer IP header (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2003-3.1-1, so no unit is bound to it.

### [`RFC2003-3.1-2`](#rfc2003-3.1-2)

An encapsulator MUST NOT encapsulate a datagram with TTL = 0 (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2003-3.1-2, so no unit is bound to it.

### [`RFC2003-3.1-3`](#rfc2003-3.1-3)

If, after decapsulation, the inner datagram has TTL = 0, the decapsulator MUST discard the datagram. (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2003-3.1-3, so no unit is bound to it.

### [`RFC2003-3.2-1`](#rfc2003-3.2-1)

If the IP Source Address of the datagram matches the router's own IP address on any of its network interfaces, the router MUST NOT tunnel the datagram (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2003-3.2-1, so no unit is bound to it.

### [`RFC2003-3.2-2`](#rfc2003-3.2-2)

If the IP Source Address of the datagram matches the IP address of the tunnel destination (the tunnel exit point is typically chosen by the router based on the Destination Address in the datagram's IP header), the router MUST NOT tunnel the datagram (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2003-3.2-2, so no unit is bound to it.

### [`RFC2003-4.1-1`](#rfc2003-4.1-1)

The encapsulator MUST relay ICMP Datagram Too Big messages to the sender of the original unencapsulated datagram (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2003-4.1-1, so no unit is bound to it.

### [`RFC2003-4.1-2`](#rfc2003-4.1-2)

If the original destination in the unencapsulated datagram is on the same network as the encapsulator, the newly generated Destination Unreachable message sent by the encapsulator MAY have Code 1 (Host Unreachable), since presumably the datagram arrived at the correct network and the encapsulator is trying to create the appearance that the original destination is local to that network even if it is not. Otherwise, if the encapsulator returns a Destination Unreachable message, the Code field MUST be set to 0 (Network Unreachable). (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2003-4.1-2, so no unit is bound to it.

### [`RFC2003-4.1-3`](#rfc2003-4.1-3)

It MUST NOT be relayed to the sender of the original unencapsulated datagram. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2003-4.1-3, so no unit is bound to it.

### [`RFC2003-4.1-4`](#rfc2003-4.1-4)

It MUST NOT be relayed to the sender of the original unencapsulated datagram. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2003-4.1-4, so no unit is bound to it.

### [`RFC2003-4.4-1`](#rfc2003-4.4-1)

Reception of Time Exceeded messages by the encapsulator MUST be reported to the sender of the original unencapsulated datagram as Host Unreachable (Type 3, Code 1). (§4.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2003-4.4-1, so no unit is bound to it.

### [`RFC2003-4.3-1`](#rfc2003-4.3-1)

It MUST NOT not relay the Redirect to the sender of the original unencapsulated datagram. (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2003-4.3-1, so no unit is bound to it.

### [`RFC2003-4.5-1`](#rfc2003-4.5-1)

if the problem occurs with an IP option inserted by the encapsulator, then the encapsulator MUST NOT relay the ICMP message to the original sender. (§4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2003-4.5-1, so no unit is bound to it.

### [`RFC2003-5.1-1`](#rfc2003-5.1-1)

To support sending nodes which use Path MTU Discovery, all encapsulator implementations MUST support Path MTU Discovery [5, 7] soft state within their tunnels. (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2003-5.1-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc2003.txt |
| Source fingerprint | 89b0add22ddc2311 |
| Record | rfc/extraction/rfc2003.json |
| Mapped sentences | 13 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 3 | walked | not stated |
| `3.2` | not stated | 2 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 4 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `4.3` | not stated | 1 | walked | not stated |
| `4.4` | not stated | 1 | walked | not stated |
| `4.5` | not stated | 1 | walked | not stated |
| `4.6` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 1 | walked | not stated |
| `5.2` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |

### Excluded sentences

The walk over RFC 2003 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 2003, so its obligations are stated where they were written.
