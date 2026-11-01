# RFC 2784 - Generic Routing Encapsulation (GRE)

GRE tunnels (gre, gretap). Every requirement this repository extracted from RFC 2784, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 11 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 11 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 11 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 11 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 0 | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 11 | of 13 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 11 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 11 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 11 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 11 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 11 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 11 | of 11 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 counts marked as a part above are the whole of the 11 gated MUSTs: they add up to 11. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. It counts TAGGED UNITS, not obligations, so this count is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public row | GRE tunnels (gre, gretap) |
| Enrolment | Enrolled |
| Requirements | 13 |
| Gated MUST-level | 11 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 11 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc2784.md` |
| Requirement shard | `rfc/requirements/rfc2784.md` |
| RFC text | `rfc/full/rfc2784.txt` |

## Enrolment

Enrolled: Generic Routing Encapsulation (GRE) base header. Ze fills the tunnel endpoint and encapsulator roles by programming gre and gretap tunnels: on Linux through internal/plugins/iface/netlink/tunnel_linux.go::buildGretun and buildGretap (linux:CONFIG_NET_IPGRE), and on the VPP interface backend (iface-vpp, ze_vpp feature gate) through internal/plugins/iface/vpp/tunnel.go::createGRETunnel. The kernel ip_gre module and VPP build and parse every GRE header. All eleven MUSTs are gaps, because the VPP datapath is a shipped path no test verifies. Two are also unmet on Linux: the kernel processes a packet with the Strict Source Route bit or a recursion bit set (2.3-1, 5.2-1). The other nine hold on Linux, from the flags ze writes (GRE_KEY only, no checksum, version 0), and owe tests

## What the public ledger says

**Area:** GRE tunnels (gre, gretap)

**Proof:** 0 of 11 MUST proven

**Evidence:** -

**Unmet:** RFC2784-2.2-1, RFC2784-2.3-1, RFC2784-2.3-2, RFC2784-2.3-3, RFC2784-2.3.1-1, RFC2784-2.6-1, RFC2784-3-1, RFC2784-3.1-1, RFC2784-3.1-2, RFC2784-3.1-3, RFC2784-5.2-1


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated (including scoped evidence) | 11 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **11** | every gated MUST falls in exactly one bucket above |

**Annotated (including scoped evidence) (11):** [`RFC2784-2.2-1`](#rfc2784-2.2-1), [`RFC2784-2.3-1`](#rfc2784-2.3-1), [`RFC2784-2.3-2`](#rfc2784-2.3-2), [`RFC2784-2.3-3`](#rfc2784-2.3-3), [`RFC2784-2.3.1-1`](#rfc2784-2.3.1-1), [`RFC2784-2.6-1`](#rfc2784-2.6-1), [`RFC2784-3-1`](#rfc2784-3-1), [`RFC2784-3.1-1`](#rfc2784-3.1-1), [`RFC2784-3.1-2`](#rfc2784-3.1-2), [`RFC2784-3.1-3`](#rfc2784-3.1-3), [`RFC2784-5.2-1`](#rfc2784-5.2-1)

## Figures

- **MUST bar**: Ze correctness gap 11
- **Overview bar**: SHOULD not decided 2, Ze correctness gap 11

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 11 |
| MUST rows kept (the MUST bar) | 11 |
| Ze proven | 0 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 11 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 2 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 2 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/rfc2784.json](../../../data/rfc/rfc2784.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2784-2.2-1` | Note that a compliant implementation MUST accept and process this field. (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel's gre_parse_header (net/ipv4/gre_demux.c, linux:CONFIG_NET_IPGRE_DEMUX) reads the Checksum Present bit, validates the checksum and drops a packet that fails it, on the gre or gretap link internal/plugins/iface/netlink/tunnel_linux.go::buildGretun or buildGretap creates (linux:CONFIG_NET_IPGRE); no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) is a shipped interface backend whose internal/plugins/iface/vpp/tunnel.go::createGRETunnel installs the tunnel through gre_tunnel_add_del, and no test shows VPP accepts and checks a GRE checksum |
| `RFC2784-2.3-1` | A receiver MUST discard a packet where any of bits 1-5 are non-zero, unless that receiver implements RFC 1701. (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** unmet on Linux and unverified on VPP. The kernel's gre_parse_header (net/ipv4/gre_demux.c, linux:CONFIG_NET_IPGRE_DEMUX) refuses only the Routing bit (bit 1) and a non-zero version; it accepts the Key and Sequence bits (bits 2 and 3, RFC 2890) and never checks the Strict Source Route bit (bit 4) or the recursion bits, so a packet with bit 4 or bit 5 set is processed, and Linux implements no other part of RFC 1701. On VPP, iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createGRETunnel, and no test shows VPP discards such a packet |
| `RFC2784-2.3-2` | Bits 6-12 are reserved for future use. These bits MUST be sent as zero (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel's gre_build_header (include/net/gre.h, linux:CONFIG_NET_IPGRE) writes only the flag bits the tunnel's flags carry, and internal/plugins/iface/netlink/tunnel_linux.go::buildGretun and buildGretap write no flags of their own, the vendored netlink (vendor/github.com/vishvananda/netlink) adding GRE_KEY only when a key is configured, so bits 6-12 go out as zero; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createGRETunnel, and no test shows what VPP writes in those bits |
| `RFC2784-2.3-3` | Bits 6-12 are reserved for future use. These bits MUST be sent as zero and MUST be ignored on receipt. (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel's gre_parse_header (net/ipv4/gre_demux.c, linux:CONFIG_NET_IPGRE_DEMUX) never tests bits 6-12 of a version 0 header, on the link internal/plugins/iface/netlink/tunnel_linux.go::buildGretun or buildGretap creates; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createGRETunnel, and no test shows VPP ignores those bits |
| `RFC2784-2.3.1-1` | Version Number field MUST contain the value zero (§2.3.1) | MUST | 2.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel's gre_build_header (include/net/gre.h, linux:CONFIG_NET_IPGRE) writes version 0, since internal/plugins/iface/netlink/tunnel_linux.go::buildGretun and buildGretap write no flags beyond the GRE_KEY the vendored netlink adds for a key, and gre_parse_header (net/ipv4/gre_demux.c) refuses a non-zero version; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createGRETunnel, and no test shows the version VPP writes |
| `RFC2784-2.6-1` | The Reserved1 field is reserved for future use, and if present, MUST be transmitted as zero. (§2.6) | MUST | 2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a gap in proof on one of the two interface backends. The Linux path is the verified half: Reserved1 is present only with a checksum, and internal/plugins/iface/netlink/tunnel_linux.go::buildGretun and buildGretap never set the checksum flag, so the kernel's gre_build_header (include/net/gre.h, linux:CONFIG_NET_IPGRE) never emits the field, and when a checksum is present it zeroes the four octets before writing the checksum; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createGRETunnel, and no test shows what VPP writes |
| `RFC2784-3-1` | When IPv4 is being carried as the GRE payload, the Protocol Type field MUST be set to 0x800. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel's gre_build_header (include/net/gre.h, linux:CONFIG_NET_IPGRE) writes the payload's skb->protocol, ETH_P_IP (0x0800) for IPv4, as the Protocol Type on the gre link internal/plugins/iface/netlink/tunnel_linux.go::buildGretun creates; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createGRETunnel, and no test shows the Protocol Type VPP writes |
| `RFC2784-3.1-1` | When a tunnel endpoint decapsulates a GRE packet which has an IPv4 packet as the payload, the destination address in the IPv4 payload packet header MUST be used to forward the packet (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel's ip_tunnel_rcv (net/ipv4/ip_tunnel.c, linux:CONFIG_NET_IPGRE) hands the decapsulated IPv4 packet to ip_rcv, which routes it on its own destination address, on the link internal/plugins/iface/netlink/tunnel_linux.go::buildGretun creates; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createGRETunnel, and no test shows how VPP forwards the payload |
| `RFC2784-3.1-2` | When a tunnel endpoint decapsulates a GRE packet which has an IPv4 packet as the payload, the destination address in the IPv4 payload packet header MUST be used to forward the packet and the TTL of the payload packet MUST be decremented. (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a gap in proof on one of the two interface backends. The Linux path is the verified half: after the kernel's ip_tunnel_rcv (net/ipv4/ip_tunnel.c, linux:CONFIG_NET_IPGRE) decapsulates, ip_forward routes the payload on its destination and decrements its TTL, on the link internal/plugins/iface/netlink/tunnel_linux.go::buildGretun creates; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createGRETunnel, and no test shows VPP decrements the payload TTL |
| `RFC2784-3.1-3` | Care should be taken when forwarding such a packet, since if the destination address of the payload packet is the encapsulator of the packet (i.e., the other end of the tunnel), looping can occur. In this case, the packet MUST be discarded. (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a gap in proof on one of the two interface backends. The Linux path is the verified half: when the payload's route leads back into the tunnel it came from, the kernel's ip_tunnel_xmit (net/ipv4/ip_tunnel.c, linux:CONFIG_NET_IPGRE) finds the route's device is the tunnel itself, counts a collision and drops the packet, on the link internal/plugins/iface/netlink/tunnel_linux.go::buildGretun creates; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createGRETunnel, and no test shows VPP discards a looping payload |
| `RFC2784-5.2-1` | As stated in Section 5.3, a packet with non-zero bits in any of bits 1-5 MUST be discarded unless the receiver implements RFC 1701. (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** unmet on Linux and unverified on VPP, as RFC2784-2.3-1. The kernel's gre_parse_header (net/ipv4/gre_demux.c, linux:CONFIG_NET_IPGRE_DEMUX) refuses only the Routing bit and a non-zero version, so a packet with the Strict Source Route bit or a recursion bit set is processed. On VPP, iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createGRETunnel, and no test shows VPP discards it |
| `RFC2784-2.4-1` | An implementation receiving a packet containing a Protocol Type which is not listed in [RFC1700] or [ETYPES] SHOULD discard the packet. (§2.4) | SHOULD | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2784-3.1-4` | Care should be taken when forwarding such a packet, since if the destination address of the payload packet is the encapsulator of the packet (i.e., the other end of the tunnel), looping can occur. (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2784-2.2-1`](#rfc2784-2.2-1) Note that a compliant implementation MUST accept and process this field. (§2.2) | {gap}, no test | a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel's gre_parse_header (net/ipv4/gre_demux.c, linux:CONFIG_NET_IPGRE_DEMUX) reads the Checksum Present bit, validates the checksum and drops a packet that fails it, on the gre or gretap link internal/plugins/iface/netlink/tunnel_linux.go::buildGretun or buildGretap creates (linux:CONFIG_NET_IPGRE); no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) is a shipped interface backend whose internal/plugins/iface/vpp/tunnel.go::createGRETunnel installs the tunnel through gre_tunnel_add_del, and no test shows VPP accepts and checks a GRE checksum |
| [`RFC2784-2.3-1`](#rfc2784-2.3-1) A receiver MUST discard a packet where any of bits 1-5 are non-zero, unless that receiver implements RFC 1701. (§2.3) | {gap}, no test | unmet on Linux and unverified on VPP. The kernel's gre_parse_header (net/ipv4/gre_demux.c, linux:CONFIG_NET_IPGRE_DEMUX) refuses only the Routing bit (bit 1) and a non-zero version; it accepts the Key and Sequence bits (bits 2 and 3, RFC 2890) and never checks the Strict Source Route bit (bit 4) or the recursion bits, so a packet with bit 4 or bit 5 set is processed, and Linux implements no other part of RFC 1701. On VPP, iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createGRETunnel, and no test shows VPP discards such a packet |
| [`RFC2784-2.3-2`](#rfc2784-2.3-2) Bits 6-12 are reserved for future use. These bits MUST be sent as zero (§2.3) | {gap}, no test | a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel's gre_build_header (include/net/gre.h, linux:CONFIG_NET_IPGRE) writes only the flag bits the tunnel's flags carry, and internal/plugins/iface/netlink/tunnel_linux.go::buildGretun and buildGretap write no flags of their own, the vendored netlink (vendor/github.com/vishvananda/netlink) adding GRE_KEY only when a key is configured, so bits 6-12 go out as zero; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createGRETunnel, and no test shows what VPP writes in those bits |
| [`RFC2784-2.3-3`](#rfc2784-2.3-3) Bits 6-12 are reserved for future use. These bits MUST be sent as zero and MUST be ignored on receipt. (§2.3) | {gap}, no test | a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel's gre_parse_header (net/ipv4/gre_demux.c, linux:CONFIG_NET_IPGRE_DEMUX) never tests bits 6-12 of a version 0 header, on the link internal/plugins/iface/netlink/tunnel_linux.go::buildGretun or buildGretap creates; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createGRETunnel, and no test shows VPP ignores those bits |
| [`RFC2784-2.3.1-1`](#rfc2784-2.3.1-1) Version Number field MUST contain the value zero (§2.3.1) | {gap}, no test | a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel's gre_build_header (include/net/gre.h, linux:CONFIG_NET_IPGRE) writes version 0, since internal/plugins/iface/netlink/tunnel_linux.go::buildGretun and buildGretap write no flags beyond the GRE_KEY the vendored netlink adds for a key, and gre_parse_header (net/ipv4/gre_demux.c) refuses a non-zero version; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createGRETunnel, and no test shows the version VPP writes |
| [`RFC2784-2.6-1`](#rfc2784-2.6-1) The Reserved1 field is reserved for future use, and if present, MUST be transmitted as zero. (§2.6) | {gap}, no test | a gap in proof on one of the two interface backends. The Linux path is the verified half: Reserved1 is present only with a checksum, and internal/plugins/iface/netlink/tunnel_linux.go::buildGretun and buildGretap never set the checksum flag, so the kernel's gre_build_header (include/net/gre.h, linux:CONFIG_NET_IPGRE) never emits the field, and when a checksum is present it zeroes the four octets before writing the checksum; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createGRETunnel, and no test shows what VPP writes |
| [`RFC2784-3-1`](#rfc2784-3-1) When IPv4 is being carried as the GRE payload, the Protocol Type field MUST be set to 0x800. (§3) | {gap}, no test | a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel's gre_build_header (include/net/gre.h, linux:CONFIG_NET_IPGRE) writes the payload's skb->protocol, ETH_P_IP (0x0800) for IPv4, as the Protocol Type on the gre link internal/plugins/iface/netlink/tunnel_linux.go::buildGretun creates; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createGRETunnel, and no test shows the Protocol Type VPP writes |
| [`RFC2784-3.1-1`](#rfc2784-3.1-1) When a tunnel endpoint decapsulates a GRE packet which has an IPv4 packet as the payload, the destination address in the IPv4 payload packet header MUST be used to forward the packet (§3.1) | {gap}, no test | a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel's ip_tunnel_rcv (net/ipv4/ip_tunnel.c, linux:CONFIG_NET_IPGRE) hands the decapsulated IPv4 packet to ip_rcv, which routes it on its own destination address, on the link internal/plugins/iface/netlink/tunnel_linux.go::buildGretun creates; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createGRETunnel, and no test shows how VPP forwards the payload |
| [`RFC2784-3.1-2`](#rfc2784-3.1-2) When a tunnel endpoint decapsulates a GRE packet which has an IPv4 packet as the payload, the destination address in the IPv4 payload packet header MUST be used to forward the packet and the TTL of the payload packet MUST be decremented. (§3.1) | {gap}, no test | a gap in proof on one of the two interface backends. The Linux path is the verified half: after the kernel's ip_tunnel_rcv (net/ipv4/ip_tunnel.c, linux:CONFIG_NET_IPGRE) decapsulates, ip_forward routes the payload on its destination and decrements its TTL, on the link internal/plugins/iface/netlink/tunnel_linux.go::buildGretun creates; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createGRETunnel, and no test shows VPP decrements the payload TTL |
| [`RFC2784-3.1-3`](#rfc2784-3.1-3) Care should be taken when forwarding such a packet, since if the destination address of the payload packet is the encapsulator of the packet (i.e., the other end of the tunnel), looping can occur. In this case, the packet MUST be discarded. (§3.1) | {gap}, no test | a gap in proof on one of the two interface backends. The Linux path is the verified half: when the payload's route leads back into the tunnel it came from, the kernel's ip_tunnel_xmit (net/ipv4/ip_tunnel.c, linux:CONFIG_NET_IPGRE) finds the route's device is the tunnel itself, counts a collision and drops the packet, on the link internal/plugins/iface/netlink/tunnel_linux.go::buildGretun creates; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createGRETunnel, and no test shows VPP discards a looping payload |
| [`RFC2784-5.2-1`](#rfc2784-5.2-1) As stated in Section 5.3, a packet with non-zero bits in any of bits 1-5 MUST be discarded unless the receiver implements RFC 1701. (§5.2) | {gap}, no test | unmet on Linux and unverified on VPP, as RFC2784-2.3-1. The kernel's gre_parse_header (net/ipv4/gre_demux.c, linux:CONFIG_NET_IPGRE_DEMUX) refuses only the Routing bit and a non-zero version, so a packet with the Strict Source Route bit or a recursion bit set is processed. On VPP, iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs the tunnel through internal/plugins/iface/vpp/tunnel.go::createGRETunnel, and no test shows VPP discards it |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2784-2.2-1`](#rfc2784-2.2-1)

Note that a compliant implementation MUST accept and process this field. (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2784-2.2-1, so no unit is bound to it.

### [`RFC2784-2.3-1`](#rfc2784-2.3-1)

A receiver MUST discard a packet where any of bits 1-5 are non-zero, unless that receiver implements RFC 1701. (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2784-2.3-1, so no unit is bound to it.

### [`RFC2784-2.3-2`](#rfc2784-2.3-2)

Bits 6-12 are reserved for future use. These bits MUST be sent as zero (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2784-2.3-2, so no unit is bound to it.

### [`RFC2784-2.3-3`](#rfc2784-2.3-3)

Bits 6-12 are reserved for future use. These bits MUST be sent as zero and MUST be ignored on receipt. (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2784-2.3-3, so no unit is bound to it.

### [`RFC2784-2.3.1-1`](#rfc2784-2.3.1-1)

Version Number field MUST contain the value zero (§2.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2784-2.3.1-1, so no unit is bound to it.

### [`RFC2784-2.6-1`](#rfc2784-2.6-1)

The Reserved1 field is reserved for future use, and if present, MUST be transmitted as zero. (§2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2784-2.6-1, so no unit is bound to it.

### [`RFC2784-3-1`](#rfc2784-3-1)

When IPv4 is being carried as the GRE payload, the Protocol Type field MUST be set to 0x800. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2784-3-1, so no unit is bound to it.

### [`RFC2784-3.1-1`](#rfc2784-3.1-1)

When a tunnel endpoint decapsulates a GRE packet which has an IPv4 packet as the payload, the destination address in the IPv4 payload packet header MUST be used to forward the packet (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2784-3.1-1, so no unit is bound to it.

### [`RFC2784-3.1-2`](#rfc2784-3.1-2)

When a tunnel endpoint decapsulates a GRE packet which has an IPv4 packet as the payload, the destination address in the IPv4 payload packet header MUST be used to forward the packet and the TTL of the payload packet MUST be decremented. (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2784-3.1-2, so no unit is bound to it.

### [`RFC2784-3.1-3`](#rfc2784-3.1-3)

Care should be taken when forwarding such a packet, since if the destination address of the payload packet is the encapsulator of the packet (i.e., the other end of the tunnel), looping can occur. In this case, the packet MUST be discarded. (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2784-3.1-3, so no unit is bound to it.

### [`RFC2784-5.2-1`](#rfc2784-5.2-1)

As stated in Section 5.3, a packet with non-zero bits in any of bits 1-5 MUST be discarded unless the receiver implements RFC 1701. (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2784-5.2-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc2784.txt |
| Source fingerprint | dbee3b50697d9b4c |
| Record | rfc/extraction/rfc2784.json |
| Mapped sentences | 9 |
| Declined as scope | 2 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 1 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 1 | walked | not stated |
| `2.3` | not stated | 2 | walked | not stated |
| `2.3.1` | not stated | 1 | walked | not stated |
| `2.4` | not stated | 0 | walked | not stated |
| `2.5` | not stated | 0 | walked | not stated |
| `2.6` | not stated | 1 | walked | not stated |
| `3` | not stated | 1 | walked | not stated |
| `3.1` | not stated | 2 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
| `5.2` | not stated | 1 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `7.2` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `12` | not stated | 1 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | RFC 2119 boilerplate in the Introduction: it declares how the keywords are to be interpreted and states no protocol obligation. | The keywords MUST, MUST NOT, MAY, OPTIONAL, REQUIRED, RECOMMENDED, SHALL, SHALL NOT, SHOULD, SHOULD NOT are to be interpreted as defined in RFC 2119 [RFC2119]. |
| `12:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | ISOC Full Copyright Statement. The lowercase 'must be followed' governs redistribution of the document, not GRE behaviour. | However, this document itself may not be modified in any way, such as by removing the copyright notice or references to the Internet Society or other Internet organizations, except as needed for the purpose of developing Internet standards in which case the procedures for copyrights defined in the Internet Standards process must be followed, or as required to translate it into languages other than English. |

## Superseded

No document obsoletes RFC 2784, so its obligations are stated where they were written.
