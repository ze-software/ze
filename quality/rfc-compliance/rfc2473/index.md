# RFC 2473 - Generic Packet Tunneling in IPv6 Specification

No row in the public ledger. Every requirement this repository extracted from RFC 2473, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

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
| Gated MUSTs | 11 | of 21 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 11 | of 11 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

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
| Requirements | 21 |
| Gated MUST-level | 11 |
| Obligations that bind Ze | 0 |
| Not applicable, so out of scope | 11 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc2473.md` |
| Requirement shard | `rfc/requirements/rfc2473.md` |
| RFC text | `rfc/full/rfc2473.txt` |

## Enrolment

Enrolled: Generic Packet Tunneling in IPv6 (tunnel datapath delegated to kernel ip6_tunnel / VPP; ze configures netdevs only)

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 2473.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 11 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **11** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (11):** [`RFC2473-4.1.1-1`](#rfc2473-4.1.1-1), [`RFC2473-4.1.1-2`](#rfc2473-4.1.1-2), [`RFC2473-4.1.1-3`](#rfc2473-4.1.1-3), [`RFC2473-4.1.1-4`](#rfc2473-4.1.1-4), [`RFC2473-4.1.2-1`](#rfc2473-4.1.2-1), [`RFC2473-7.1-1`](#rfc2473-7.1-1), [`RFC2473-7.1-2`](#rfc2473-7.1-2), [`RFC2473-7.1-3`](#rfc2473-7.1-3), [`RFC2473-7.1-4`](#rfc2473-7.1-4), [`RFC2473-7.2-1`](#rfc2473-7.2-1), [`RFC2473-8-1`](#rfc2473-8-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2473-4.1.1-1` | When Tunnel Encapsulation Limit option value reaches zero, discard packet and send ICMPv6 Parameter Problem (code 0, pointer to limit octet) (§4.1.1) | MUST | 4.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the tunnel-node datapath that processes the encapsulation-limit option and emits ICMPv6 Parameter Problem is the kernel ip6_tunnel module; ze only creates and configures the tunnel netdev (internal/plugins/iface/netlink/tunnel_linux.go:44) |
| `RFC2473-4.1.1-2` | When Tunnel Encapsulation Limit option is found with non-zero value, include a new Tunnel Encapsulation Limit option in the encapsulating headers with value decremented by one (§4.1.1) | MUST | 4.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** decrementing and re-emitting the encapsulation-limit option during encapsulation is a kernel/VPP datapath action; ze supplies only the configured limit via netlink (internal/plugins/iface/netlink/tunnel_linux.go:266) |
| `RFC2473-4.1.1-3` | When no Tunnel Encapsulation Limit option is found but a limit is configured, include a Tunnel Encapsulation Limit option with the configured value (§4.1.1) | MUST | 4.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** inserting the configured encapsulation-limit option is done by the kernel ip6_tunnel datapath; ze passes IFLA_IPTUN_ENCAP_LIMIT at internal/plugins/iface/netlink/tunnel_linux.go:266-267 |
| `RFC2473-4.1.1-4` | Examine headers following the IPv6 header in strict left-to-right order when checking for Tunnel Encapsulation Limit option (§4.1.1) | MUST | 4.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** left-to-right extension-header parsing at packet time is kernel datapath parsing; ze carries no per-packet header path, only tunnel netdev creation (internal/plugins/iface/netlink/tunnel_linux.go:44) |
| `RFC2473-4.1.2-1` | Loopback encapsulation (entry-point and exit-point are the same node) must be avoided (§4.1.2) | MUST | 4.1.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** detecting loopback encapsulation at packet time binds the kernel ip6_tunnel datapath; ze only creates the tunnel netdev (internal/plugins/iface/netlink/tunnel_linux.go:44) |
| `RFC2473-7.1-1` | Tunnel entry-point node must support fragmentation of tunnel IPv6 packets (§7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** fragmenting outbound tunnel packets is a kernel/VPP datapath capability; ze holds no packet-forwarding code, only tunnel config (internal/plugins/iface/netlink/tunnel_linux.go:248) |
| `RFC2473-7.1-2` | Tunnel intermediate node must not fragment a packet undergoing forwarding (§7.1) | MUST NOT | 7.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is a routing control plane with no IPv6 forwarding datapath; the intermediate-node no-fragment rule is enforced by the kernel/VPP |
| `RFC2473-7.1-3` | If original IPv6 packet exceeds tunnel MTU and is larger than IPv6 minimum link MTU, discard and send ICMPv6 Packet Too Big with MTU = max(tunnel MTU, IPv6 minimum link MTU) (§7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** tunnel-MTU comparison, discard, and ICMPv6 Packet Too Big generation are kernel ip6_tunnel datapath actions ze does not perform |
| `RFC2473-7.1-4` | If original IPv6 packet exceeds tunnel MTU but is equal or smaller than IPv6 minimum link MTU, encapsulate then fragment the tunnel packet (§7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** encapsulate-then-fragment is a kernel/VPP datapath operation; ze only configures the tunnel interface (internal/plugins/iface/netlink/tunnel_linux.go:248) |
| `RFC2473-7.2-1` | If original IPv4 packet has DF set and exceeds tunnel MTU, discard and send ICMP unreachable/packet-too-big with tunnel MTU (§7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** DF handling and ICMP unreachable/too-big generation for encapsulated IPv4 are kernel datapath behavior; ze only sets PMtuDisc via netlink (internal/plugins/iface/netlink/tunnel_linux.go:210-214) |
| `RFC2473-8-1` | Tunnel entry-point node must relay ICMP messages from inside the tunnel to the source of the original packet (§8) | MUST | 8 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** relaying tunnel-internal ICMP errors to the original source is a kernel ip6_tunnel datapath function ze does not perform |
| `RFC2473-3.1-1` | Tunnel extension headers should appear in the order recommended by IPv6 specifications (§3.1, §5.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2473-6.3-1` | The "single-hop" mechanism should be implemented by setting tunnel hop limit independently of the original header (§6.3) | SHOULD | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2473-4.1.2-2` | Implementation should check and reject configuration of a tunnel where entry-point and exit-point addresses belong to the same node (§4.1.2) | SHOULD | 4.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2473-4.1.2-3` | Encapsulating engine should check for and reject encapsulation where tunnel endpoint addresses match original packet source/destination addresses (§4.1.2) | SHOULD | 4.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2473-4.1.3-1` | Maximum hops on a path with tunnels should be controlled by both original packet hop limit and tunnel encapsulation limit (§4.1.3) | SHOULD | 4.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2473-6.3-2` | Tunnel hop limit should be configured to ensure packets reach exit-point and expire quickly on routing loops (§6.3) | SHOULD | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2473-6.1-1` | Tunnel entry-point node address should be validated at tunnel configuration time (§6.1) | SHOULD | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2473-6.4-1` | Traffic Class in tunnel header MAY be inherited from inner packet or set per-tunnel (§6.4) | MAY | 6.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2473-4.1.1-5` | Implementations MAY allow per-tunnel override of the encapsulation limit (§4.1.1, §6.6) | MAY | 4.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2473-5.1-1` | Tunnel entry-point node may append IPv6 extension headers (Hop-by-Hop, Routing, etc.) to the tunnel header (§5.1) | MAY | 5.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2473-4.1.1-1`](#rfc2473-4.1.1-1) When Tunnel Encapsulation Limit option value reaches zero, discard packet and send ICMPv6 Parameter Problem (code 0, pointer to limit octet) (§4.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: the tunnel-node datapath that processes the encapsulation-limit option and emits ICMPv6 Parameter Problem is the kernel ip6_tunnel module; ze only creates and configures the tunnel netdev (internal/plugins/iface/netlink/tunnel_linux.go:44) |
| [`RFC2473-4.1.1-2`](#rfc2473-4.1.1-2) When Tunnel Encapsulation Limit option is found with non-zero value, include a new Tunnel Encapsulation Limit option in the encapsulating headers with value decremented by one (§4.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: decrementing and re-emitting the encapsulation-limit option during encapsulation is a kernel/VPP datapath action; ze supplies only the configured limit via netlink (internal/plugins/iface/netlink/tunnel_linux.go:266) |
| [`RFC2473-4.1.1-3`](#rfc2473-4.1.1-3) When no Tunnel Encapsulation Limit option is found but a limit is configured, include a Tunnel Encapsulation Limit option with the configured value (§4.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: inserting the configured encapsulation-limit option is done by the kernel ip6_tunnel datapath; ze passes IFLA_IPTUN_ENCAP_LIMIT at internal/plugins/iface/netlink/tunnel_linux.go:266-267 |
| [`RFC2473-4.1.1-4`](#rfc2473-4.1.1-4) Examine headers following the IPv6 header in strict left-to-right order when checking for Tunnel Encapsulation Limit option (§4.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: left-to-right extension-header parsing at packet time is kernel datapath parsing; ze carries no per-packet header path, only tunnel netdev creation (internal/plugins/iface/netlink/tunnel_linux.go:44) |
| [`RFC2473-4.1.2-1`](#rfc2473-4.1.2-1) Loopback encapsulation (entry-point and exit-point are the same node) must be avoided (§4.1.2) | no test | no test carries this requirement id; annotated {not-applicable}: detecting loopback encapsulation at packet time binds the kernel ip6_tunnel datapath; ze only creates the tunnel netdev (internal/plugins/iface/netlink/tunnel_linux.go:44) |
| [`RFC2473-7.1-1`](#rfc2473-7.1-1) Tunnel entry-point node must support fragmentation of tunnel IPv6 packets (§7.1) | no test | no test carries this requirement id; annotated {not-applicable}: fragmenting outbound tunnel packets is a kernel/VPP datapath capability; ze holds no packet-forwarding code, only tunnel config (internal/plugins/iface/netlink/tunnel_linux.go:248) |
| [`RFC2473-7.1-2`](#rfc2473-7.1-2) Tunnel intermediate node must not fragment a packet undergoing forwarding (§7.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze is a routing control plane with no IPv6 forwarding datapath; the intermediate-node no-fragment rule is enforced by the kernel/VPP |
| [`RFC2473-7.1-3`](#rfc2473-7.1-3) If original IPv6 packet exceeds tunnel MTU and is larger than IPv6 minimum link MTU, discard and send ICMPv6 Packet Too Big with MTU = max(tunnel MTU, IPv6 minimum link MTU) (§7.1) | no test | no test carries this requirement id; annotated {not-applicable}: tunnel-MTU comparison, discard, and ICMPv6 Packet Too Big generation are kernel ip6_tunnel datapath actions ze does not perform |
| [`RFC2473-7.1-4`](#rfc2473-7.1-4) If original IPv6 packet exceeds tunnel MTU but is equal or smaller than IPv6 minimum link MTU, encapsulate then fragment the tunnel packet (§7.1) | no test | no test carries this requirement id; annotated {not-applicable}: encapsulate-then-fragment is a kernel/VPP datapath operation; ze only configures the tunnel interface (internal/plugins/iface/netlink/tunnel_linux.go:248) |
| [`RFC2473-7.2-1`](#rfc2473-7.2-1) If original IPv4 packet has DF set and exceeds tunnel MTU, discard and send ICMP unreachable/packet-too-big with tunnel MTU (§7.2) | no test | no test carries this requirement id; annotated {not-applicable}: DF handling and ICMP unreachable/too-big generation for encapsulated IPv4 are kernel datapath behavior; ze only sets PMtuDisc via netlink (internal/plugins/iface/netlink/tunnel_linux.go:210-214) |
| [`RFC2473-8-1`](#rfc2473-8-1) Tunnel entry-point node must relay ICMP messages from inside the tunnel to the source of the original packet (§8) | no test | no test carries this requirement id; annotated {not-applicable}: relaying tunnel-internal ICMP errors to the original source is a kernel ip6_tunnel datapath function ze does not perform |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2473-4.1.1-1`](#rfc2473-4.1.1-1)

When Tunnel Encapsulation Limit option value reaches zero, discard packet and send ICMPv6 Parameter Problem (code 0, pointer to limit octet) (§4.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2473-4.1.1-1, so no unit is bound to it.

### [`RFC2473-4.1.1-2`](#rfc2473-4.1.1-2)

When Tunnel Encapsulation Limit option is found with non-zero value, include a new Tunnel Encapsulation Limit option in the encapsulating headers with value decremented by one (§4.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2473-4.1.1-2, so no unit is bound to it.

### [`RFC2473-4.1.1-3`](#rfc2473-4.1.1-3)

When no Tunnel Encapsulation Limit option is found but a limit is configured, include a Tunnel Encapsulation Limit option with the configured value (§4.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2473-4.1.1-3, so no unit is bound to it.

### [`RFC2473-4.1.1-4`](#rfc2473-4.1.1-4)

Examine headers following the IPv6 header in strict left-to-right order when checking for Tunnel Encapsulation Limit option (§4.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2473-4.1.1-4, so no unit is bound to it.

### [`RFC2473-4.1.2-1`](#rfc2473-4.1.2-1)

Loopback encapsulation (entry-point and exit-point are the same node) must be avoided (§4.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2473-4.1.2-1, so no unit is bound to it.

### [`RFC2473-7.1-1`](#rfc2473-7.1-1)

Tunnel entry-point node must support fragmentation of tunnel IPv6 packets (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2473-7.1-1, so no unit is bound to it.

### [`RFC2473-7.1-2`](#rfc2473-7.1-2)

Tunnel intermediate node must not fragment a packet undergoing forwarding (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2473-7.1-2, so no unit is bound to it.

### [`RFC2473-7.1-3`](#rfc2473-7.1-3)

If original IPv6 packet exceeds tunnel MTU and is larger than IPv6 minimum link MTU, discard and send ICMPv6 Packet Too Big with MTU = max(tunnel MTU, IPv6 minimum link MTU) (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2473-7.1-3, so no unit is bound to it.

### [`RFC2473-7.1-4`](#rfc2473-7.1-4)

If original IPv6 packet exceeds tunnel MTU but is equal or smaller than IPv6 minimum link MTU, encapsulate then fragment the tunnel packet (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2473-7.1-4, so no unit is bound to it.

### [`RFC2473-7.2-1`](#rfc2473-7.2-1)

If original IPv4 packet has DF set and exceeds tunnel MTU, discard and send ICMP unreachable/packet-too-big with tunnel MTU (§7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2473-7.2-1, so no unit is bound to it.

### [`RFC2473-8-1`](#rfc2473-8-1)

Tunnel entry-point node must relay ICMP messages from inside the tunnel to the source of the original packet (§8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2473-8-1, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 2473, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 2473, so its obligations are stated where they were written.
