# RFC 2003 - IP Encapsulation within IP

Partial. Every requirement this repository extracted from RFC 2003, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 13 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 13 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 13 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 13 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 13 | of 37 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 13 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 13 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 13 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 13 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |
| Linux cannot do this | 0.0% | 0 of 13 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and remains unmet; it stays in every denominator and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 100.0% | 13 of 13 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 shares marked as a part above are the whole of the 13 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 37 |
| Gated MUST-level | 13 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 13 |
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

Enrolled: IP Encapsulation within IP (IP-in-IP, protocol 4). Ze fills the encapsulator and decapsulator roles, and the router role of Section 3.2, by programming ipip tunnels: on Linux through internal/plugins/iface/netlink/tunnel_linux.go::buildIptun (linux:CONFIG_NET_IPIP), and on the VPP interface backend (iface-vpp, ze_vpp feature gate) through internal/plugins/iface/vpp/tunnel.go::createIPIPTunnel. The kernel ipip module and VPP perform the per-packet work. All thirteen MUSTs are gaps, because the VPP datapath is a shipped path no test verifies. Six are also unmet on Linux: a TTL 0 inner datagram addressed to ze is delivered (3.1-3), no source-address loop check exists (3.2-1, 3.2-2), Datagram Too Big is answered through path MTU soft state rather than relayed (4.1-1), a remote Network Unreachable returns as Code 1 (4.1-2), and a Port Unreachable reaches the sender as Host Unreachable (4.1-3). Time Exceeded is reported only for a non-zero tunnel ttl, which an operator can set to 0 (4.4-1). The other six hold on Linux and owe tests

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Ze configures ipip tunnels through the `tunnel` YANG list, built as a netlink Iptun link (`buildIptun`) with optional ttl, tos and no-pmtu-discovery, or as a VPP ipip tunnel (`createIPIPTunnel`) on the VPP interface backend
- the Linux ipip module or VPP carries the traffic.


**What the ledger says remains**

Thirteen MUST gaps in the requirement checklist. No test verifies the VPP ipip datapath for any of them. On Linux a TTL 0 inner datagram addressed to ze is delivered ([`RFC2003-3.1-3`](#rfc2003-3.1-3)), nothing refuses a datagram sourced from ze's own address or from the tunnel destination ([`RFC2003-3.2-1`](#rfc2003-3.2-1), 3.2-2), Datagram Too Big is answered through path MTU soft state rather than relayed ([`RFC2003-4.1-1`](#rfc2003-4.1-1)), a remote Network Unreachable is returned as Host Unreachable ([`RFC2003-4.1-2`](#rfc2003-4.1-2)), a Port Unreachable reaches the sender as Host Unreachable ([`RFC2003-4.1-3`](#rfc2003-4.1-3)), and an operator's ttl 0 stops Time Exceeded being reported ([`RFC2003-4.4-1`](#rfc2003-4.4-1)). The other six hold on Linux and owe tests over the link ze installs.

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

## Attainable ceiling

Mandatory ceiling: 13 of 13 (100.0%). Optional ceiling: 24 of 24 (100.0%). Every RFC shown with measured rows can reach 100% under current classifications. Lowest mandatory ceiling: rfc2003 13 of 13 (100.0%). Lowest optional ceiling: rfc2003 24 of 24 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2003-3.1-1` | if the "Don't Fragment" bit is set in the inner IP header, it MUST be set in the outer IP header (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a gap in proof on one of the two interface backends. The Linux path is the verified half: internal/plugins/iface/netlink/tunnel_linux.go::buildIptun creates the ipip link (linux:CONFIG_NET_IPIP), and the kernel's ip_tunnel_xmit (net/ipv4/ip_tunnel.c) ORs the inner Don't Fragment bit into the outer header for every IPv4 payload, since ipip has no ignore_df knob, whichever PMtuDisc value buildIptun writes (1 unless no-pmtu-discovery sets 0); no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) is a shipped interface backend whose internal/plugins/iface/vpp/tunnel.go::createIPIPTunnel installs the tunnel through ipip_add_tunnel, and no test shows what VPP's ipip datapath writes in the outer header |
| `RFC2003-3.1-2` | An encapsulator MUST NOT encapsulate a datagram with TTL = 0 (§3.1) | MUST NOT | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a gap in proof on one of the two interface backends. On Linux (linux:CONFIG_NET_IPIP, link built by internal/plugins/iface/netlink/tunnel_linux.go::buildIptun) a forwarded datagram reaches the tunnel only after ip_forward has dropped every TTL of 1 or less, and a local socket cannot set TTL 0, so no TTL 0 datagram is encapsulated; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) is a shipped interface backend whose internal/plugins/iface/vpp/tunnel.go::createIPIPTunnel installs the tunnel through ipip_add_tunnel, and no test shows VPP refuses to encapsulate a TTL 0 datagram |
| `RFC2003-3.1-3` | If, after decapsulation, the inner datagram has TTL = 0, the decapsulator MUST discard the datagram. (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** unmet in part on Linux and unverified on VPP. On Linux (linux:CONFIG_NET_IPIP, link built by internal/plugins/iface/netlink/tunnel_linux.go::buildIptun) the decapsulated datagram re-enters ip_rcv, and ip_forward discards a TTL 0 datagram it would forward, but ip_local_deliver checks no TTL, so a TTL 0 inner datagram addressed to ze itself is delivered rather than discarded. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createIPIPTunnel, and no test shows VPP discards an inner TTL 0 datagram |
| `RFC2003-3.2-1` | If the IP Source Address of the datagram matches the router's own IP address on any of its network interfaces, the router MUST NOT tunnel the datagram (§3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** unmet on Linux and unverified on VPP. The kernel's ip_tunnel_xmit (net/ipv4/ip_tunnel.c, linux:CONFIG_NET_IPIP) compares no source address with the router's own addresses before encapsulating, and nothing ze installs (internal/plugins/iface/netlink/tunnel_linux.go::buildIptun, no firewall rule) refuses such a datagram, so a datagram sourced from ze's own address is tunneled. On VPP, iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createIPIPTunnel and no test shows VPP refuses it |
| `RFC2003-3.2-2` | If the IP Source Address of the datagram matches the IP address of the tunnel destination (the tunnel exit point is typically chosen by the router based on the Destination Address in the datagram's IP header), the router MUST NOT tunnel the datagram (§3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** unmet on Linux and unverified on VPP. The kernel's ip_tunnel_xmit (net/ipv4/ip_tunnel.c, linux:CONFIG_NET_IPIP) compares no inner source address with the tunnel destination before encapsulating, and nothing ze installs (internal/plugins/iface/netlink/tunnel_linux.go::buildIptun, no firewall rule) refuses such a datagram. On VPP, iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createIPIPTunnel and no test shows VPP refuses it |
| `RFC2003-4.1-1` | The encapsulator MUST relay ICMP Datagram Too Big messages to the sender of the original unencapsulated datagram (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** unmet on Linux and unverified on VPP. The kernel's ipip_err (net/ipv4/ipip.c, linux:CONFIG_NET_IPIP) relays no Datagram Too Big message: it lowers the tunnel's path MTU (ipv4_update_pmtu), and only the sender's next oversized Don't Fragment datagram draws a Fragmentation Needed from tnl_update_pmtu, so the datagram that met the error is never reported. On VPP, iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createIPIPTunnel and no test shows what VPP does with the message |
| `RFC2003-4.1-2` | If the original destination in the unencapsulated datagram is on the same network as the encapsulator, the newly generated Destination Unreachable message sent by the encapsulator MAY have Code 1 (Host Unreachable), since presumably the datagram arrived at the correct network and the encapsulator is trying to create the appearance that the original destination is local to that network even if it is not. Otherwise, if the encapsulator returns a Destination Unreachable message, the Code field MUST be set to 0 (Network Unreachable). (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** unmet on Linux and unverified on VPP. The kernel's ipip_err (net/ipv4/ipip.c, linux:CONFIG_NET_IPIP) translates every Destination Unreachable other than Source Route Failed and Fragmentation Needed into a later Host Unreachable (err_count, then dst_link_failure in ip_tunnel_xmit), whether or not the original destination is on the encapsulator's network, so a Network Unreachable from a remote network is returned with Code 1 where this requires Code 0. On VPP, iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createIPIPTunnel and no test shows what VPP returns |
| `RFC2003-4.1-3` | It MUST NOT be relayed to the sender of the original unencapsulated datagram. (§4.1) | MUST NOT | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** unmet on Linux and unverified on VPP. The kernel's ipip_err (net/ipv4/ipip.c, linux:CONFIG_NET_IPIP) counts a Port Unreachable from inside the tunnel like any other Destination Unreachable, and ip_tunnel_xmit then sends the original sender a Host Unreachable through dst_link_failure, so the error reaches the sender. On VPP, iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createIPIPTunnel and no test shows VPP withholds it |
| `RFC2003-4.1-4` | It MUST NOT be relayed to the sender of the original unencapsulated datagram. (§4.1) | MUST NOT | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel's ipip_err (net/ipv4/ipip.c, linux:CONFIG_NET_IPIP) returns on ICMP_SR_FAILED before anything is counted or relayed, on the tunnel internal/plugins/iface/netlink/tunnel_linux.go::buildIptun creates; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createIPIPTunnel, and no test shows VPP withholds a Source Route Failed |
| `RFC2003-4.4-1` | Reception of Time Exceeded messages by the encapsulator MUST be reported to the sender of the original unencapsulated datagram as Host Unreachable (Type 3, Code 1). (§4.4) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** met on Linux only for a non-zero tunnel TTL, and unverified on VPP. The kernel's ipip_err (net/ipv4/ipip.c, linux:CONFIG_NET_IPIP) counts a Time Exceeded and ip_tunnel_xmit reports it to the sender as Host Unreachable through dst_link_failure, but it drops the message when the tunnel TTL is 0 (inherit). internal/plugins/iface/netlink/tunnel_linux.go::buildIptun writes the ttl leaf (default 64) into Ttl, and the uint8 leaf admits 0 with nothing refusing it, so an operator's ttl 0 defeats this. On VPP, iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createIPIPTunnel and no test shows what VPP reports |
| `RFC2003-4.3-1` | It MUST NOT not relay the Redirect to the sender of the original unencapsulated datagram. (§4.3) | MUST NOT | 4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel's ipip_err (net/ipv4/ipip.c, linux:CONFIG_NET_IPIP) handles a Redirect itself through ipv4_redirect and returns before anything is counted or relayed, on the tunnel internal/plugins/iface/netlink/tunnel_linux.go::buildIptun creates; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createIPIPTunnel, and no test shows VPP withholds a Redirect |
| `RFC2003-4.5-1` | if the problem occurs with an IP option inserted by the encapsulator, then the encapsulator MUST NOT relay the ICMP message to the original sender. (§4.5) | MUST NOT | 4.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel's ipip_err (net/ipv4/ipip.c, linux:CONFIG_NET_IPIP) returns on a Parameter Problem without relaying it, and the kernel inserts no IP option into the outer header of the tunnel internal/plugins/iface/netlink/tunnel_linux.go::buildIptun creates; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createIPIPTunnel, and no test shows what VPP does with a Parameter Problem |
| `RFC2003-5.1-1` | To support sending nodes which use Path MTU Discovery, all encapsulator implementations MUST support Path MTU Discovery [5, 7] soft state within their tunnels. (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel keeps path MTU soft state per tunnel (net/ipv4/ip_tunnel.c tnl_update_pmtu, ipv4_update_pmtu from ipip_err, linux:CONFIG_NET_IPIP) and answers an oversized Don't Fragment datagram with Fragmentation Needed; the PMtuDisc value internal/plugins/iface/netlink/tunnel_linux.go::buildIptun writes (1 unless no-pmtu-discovery sets 0) decides whether the outer header carries DF, and no test covers either. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createIPIPTunnel with no path MTU, and no test shows VPP keeps any |
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
| [`RFC2003-3.1-1`](#rfc2003-3.1-1) if the "Don't Fragment" bit is set in the inner IP header, it MUST be set in the outer IP header (§3.1) | {gap}, no test | a gap in proof on one of the two interface backends. The Linux path is the verified half: internal/plugins/iface/netlink/tunnel_linux.go::buildIptun creates the ipip link (linux:CONFIG_NET_IPIP), and the kernel's ip_tunnel_xmit (net/ipv4/ip_tunnel.c) ORs the inner Don't Fragment bit into the outer header for every IPv4 payload, since ipip has no ignore_df knob, whichever PMtuDisc value buildIptun writes (1 unless no-pmtu-discovery sets 0); no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) is a shipped interface backend whose internal/plugins/iface/vpp/tunnel.go::createIPIPTunnel installs the tunnel through ipip_add_tunnel, and no test shows what VPP's ipip datapath writes in the outer header |
| [`RFC2003-3.1-2`](#rfc2003-3.1-2) An encapsulator MUST NOT encapsulate a datagram with TTL = 0 (§3.1) | {gap}, no test | a gap in proof on one of the two interface backends. On Linux (linux:CONFIG_NET_IPIP, link built by internal/plugins/iface/netlink/tunnel_linux.go::buildIptun) a forwarded datagram reaches the tunnel only after ip_forward has dropped every TTL of 1 or less, and a local socket cannot set TTL 0, so no TTL 0 datagram is encapsulated; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) is a shipped interface backend whose internal/plugins/iface/vpp/tunnel.go::createIPIPTunnel installs the tunnel through ipip_add_tunnel, and no test shows VPP refuses to encapsulate a TTL 0 datagram |
| [`RFC2003-3.1-3`](#rfc2003-3.1-3) If, after decapsulation, the inner datagram has TTL = 0, the decapsulator MUST discard the datagram. (§3.1) | {gap}, no test | unmet in part on Linux and unverified on VPP. On Linux (linux:CONFIG_NET_IPIP, link built by internal/plugins/iface/netlink/tunnel_linux.go::buildIptun) the decapsulated datagram re-enters ip_rcv, and ip_forward discards a TTL 0 datagram it would forward, but ip_local_deliver checks no TTL, so a TTL 0 inner datagram addressed to ze itself is delivered rather than discarded. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createIPIPTunnel, and no test shows VPP discards an inner TTL 0 datagram |
| [`RFC2003-3.2-1`](#rfc2003-3.2-1) If the IP Source Address of the datagram matches the router's own IP address on any of its network interfaces, the router MUST NOT tunnel the datagram (§3.2) | {gap}, no test | unmet on Linux and unverified on VPP. The kernel's ip_tunnel_xmit (net/ipv4/ip_tunnel.c, linux:CONFIG_NET_IPIP) compares no source address with the router's own addresses before encapsulating, and nothing ze installs (internal/plugins/iface/netlink/tunnel_linux.go::buildIptun, no firewall rule) refuses such a datagram, so a datagram sourced from ze's own address is tunneled. On VPP, iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createIPIPTunnel and no test shows VPP refuses it |
| [`RFC2003-3.2-2`](#rfc2003-3.2-2) If the IP Source Address of the datagram matches the IP address of the tunnel destination (the tunnel exit point is typically chosen by the router based on the Destination Address in the datagram's IP header), the router MUST NOT tunnel the datagram (§3.2) | {gap}, no test | unmet on Linux and unverified on VPP. The kernel's ip_tunnel_xmit (net/ipv4/ip_tunnel.c, linux:CONFIG_NET_IPIP) compares no inner source address with the tunnel destination before encapsulating, and nothing ze installs (internal/plugins/iface/netlink/tunnel_linux.go::buildIptun, no firewall rule) refuses such a datagram. On VPP, iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createIPIPTunnel and no test shows VPP refuses it |
| [`RFC2003-4.1-1`](#rfc2003-4.1-1) The encapsulator MUST relay ICMP Datagram Too Big messages to the sender of the original unencapsulated datagram (§4.1) | {gap}, no test | unmet on Linux and unverified on VPP. The kernel's ipip_err (net/ipv4/ipip.c, linux:CONFIG_NET_IPIP) relays no Datagram Too Big message: it lowers the tunnel's path MTU (ipv4_update_pmtu), and only the sender's next oversized Don't Fragment datagram draws a Fragmentation Needed from tnl_update_pmtu, so the datagram that met the error is never reported. On VPP, iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createIPIPTunnel and no test shows what VPP does with the message |
| [`RFC2003-4.1-2`](#rfc2003-4.1-2) If the original destination in the unencapsulated datagram is on the same network as the encapsulator, the newly generated Destination Unreachable message sent by the encapsulator MAY have Code 1 (Host Unreachable), since presumably the datagram arrived at the correct network and the encapsulator is trying to create the appearance that the original destination is local to that network even if it is not. Otherwise, if the encapsulator returns a Destination Unreachable message, the Code field MUST be set to 0 (Network Unreachable). (§4.1) | {gap}, no test | unmet on Linux and unverified on VPP. The kernel's ipip_err (net/ipv4/ipip.c, linux:CONFIG_NET_IPIP) translates every Destination Unreachable other than Source Route Failed and Fragmentation Needed into a later Host Unreachable (err_count, then dst_link_failure in ip_tunnel_xmit), whether or not the original destination is on the encapsulator's network, so a Network Unreachable from a remote network is returned with Code 1 where this requires Code 0. On VPP, iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createIPIPTunnel and no test shows what VPP returns |
| [`RFC2003-4.1-3`](#rfc2003-4.1-3) It MUST NOT be relayed to the sender of the original unencapsulated datagram. (§4.1) | {gap}, no test | unmet on Linux and unverified on VPP. The kernel's ipip_err (net/ipv4/ipip.c, linux:CONFIG_NET_IPIP) counts a Port Unreachable from inside the tunnel like any other Destination Unreachable, and ip_tunnel_xmit then sends the original sender a Host Unreachable through dst_link_failure, so the error reaches the sender. On VPP, iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createIPIPTunnel and no test shows VPP withholds it |
| [`RFC2003-4.1-4`](#rfc2003-4.1-4) It MUST NOT be relayed to the sender of the original unencapsulated datagram. (§4.1) | {gap}, no test | a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel's ipip_err (net/ipv4/ipip.c, linux:CONFIG_NET_IPIP) returns on ICMP_SR_FAILED before anything is counted or relayed, on the tunnel internal/plugins/iface/netlink/tunnel_linux.go::buildIptun creates; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createIPIPTunnel, and no test shows VPP withholds a Source Route Failed |
| [`RFC2003-4.4-1`](#rfc2003-4.4-1) Reception of Time Exceeded messages by the encapsulator MUST be reported to the sender of the original unencapsulated datagram as Host Unreachable (Type 3, Code 1). (§4.4) | {gap}, no test | met on Linux only for a non-zero tunnel TTL, and unverified on VPP. The kernel's ipip_err (net/ipv4/ipip.c, linux:CONFIG_NET_IPIP) counts a Time Exceeded and ip_tunnel_xmit reports it to the sender as Host Unreachable through dst_link_failure, but it drops the message when the tunnel TTL is 0 (inherit). internal/plugins/iface/netlink/tunnel_linux.go::buildIptun writes the ttl leaf (default 64) into Ttl, and the uint8 leaf admits 0 with nothing refusing it, so an operator's ttl 0 defeats this. On VPP, iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createIPIPTunnel and no test shows what VPP reports |
| [`RFC2003-4.3-1`](#rfc2003-4.3-1) It MUST NOT not relay the Redirect to the sender of the original unencapsulated datagram. (§4.3) | {gap}, no test | a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel's ipip_err (net/ipv4/ipip.c, linux:CONFIG_NET_IPIP) handles a Redirect itself through ipv4_redirect and returns before anything is counted or relayed, on the tunnel internal/plugins/iface/netlink/tunnel_linux.go::buildIptun creates; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createIPIPTunnel, and no test shows VPP withholds a Redirect |
| [`RFC2003-4.5-1`](#rfc2003-4.5-1) if the problem occurs with an IP option inserted by the encapsulator, then the encapsulator MUST NOT relay the ICMP message to the original sender. (§4.5) | {gap}, no test | a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel's ipip_err (net/ipv4/ipip.c, linux:CONFIG_NET_IPIP) returns on a Parameter Problem without relaying it, and the kernel inserts no IP option into the outer header of the tunnel internal/plugins/iface/netlink/tunnel_linux.go::buildIptun creates; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createIPIPTunnel, and no test shows what VPP does with a Parameter Problem |
| [`RFC2003-5.1-1`](#rfc2003-5.1-1) To support sending nodes which use Path MTU Discovery, all encapsulator implementations MUST support Path MTU Discovery [5, 7] soft state within their tunnels. (§5.1) | {gap}, no test | a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel keeps path MTU soft state per tunnel (net/ipv4/ip_tunnel.c tnl_update_pmtu, ipv4_update_pmtu from ipip_err, linux:CONFIG_NET_IPIP) and answers an oversized Don't Fragment datagram with Fragmentation Needed; the PMtuDisc value internal/plugins/iface/netlink/tunnel_linux.go::buildIptun writes (1 unless no-pmtu-discovery sets 0) decides whether the outer header carries DF, and no test covers either. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createIPIPTunnel with no path MTU, and no test shows VPP keeps any |

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
