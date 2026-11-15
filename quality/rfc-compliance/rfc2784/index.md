# RFC 2784 - Generic Routing Encapsulation (GRE)

GRE tunnels (gre, gretap). Every requirement listed for RFC 2784, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 11 mandatory requirements | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 11 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 11 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 11 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Tests shown to detect a fault | 0 | 0 of 0 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests counts as proof only after this check has demonstrated that they can detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 11 | of 11 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 11 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 11 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 11 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 11 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 11 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no proof credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 11 | of 13 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 11 mandatory requirements: they add up to 11. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | GRE tunnels (gre, gretap) |
| Automatic checks | Included |
| Requirements | 13 |
| Mandatory requirements (MUST level) | 11 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 11 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tests linked to requirements | 0 |
| Recorded audit verdicts | 0 |
| Fault test records | 0 |
| Summary | `rfc/short/rfc2784.md` |
| Requirement table | `rfc/requirements/rfc2784.md` |
| RFC text | `rfc/full/rfc2784.txt` |

## Automatic checks

Included in automatic checks: Generic Routing Encapsulation (GRE) base header. Ze fills the tunnel endpoint and encapsulator roles by programming gre and gretap tunnels: on Linux through internal/plugins/iface/netlink/tunnel_linux.go::buildGretun and buildGretap (linux:CONFIG_NET_IPGRE), and on the VPP interface backend (iface-vpp, ze_vpp feature gate) through internal/plugins/iface/vpp/tunnel.go::createGRETunnel. The kernel ip_gre module and VPP build and parse every GRE header. All eleven MUSTs are gaps, because the VPP datapath is a shipped path no test verifies. Two are also unmet on Linux: the kernel processes a packet with the Strict Source Route bit or a recursion bit set (2.3-1, 5.2-1). The other nine hold on Linux, from the flags ze writes (GRE_KEY only, no checksum, version 0), and owe tests

## Published support status

**Area:** GRE tunnels (gre, gretap)

**Proof:** 0 of 11 MUST proven

**Evidence:** -

**Unmet:** RFC2784-2.2-1, RFC2784-2.3-1, RFC2784-2.3-2, RFC2784-2.3-3, RFC2784-2.3.1-1, RFC2784-2.6-1, RFC2784-3-1, RFC2784-3.1-1, RFC2784-3.1-2, RFC2784-3.1-3, RFC2784-5.2-1


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 11 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **11** | every mandatory requirement appears in exactly one group above |

**Annotated (including scoped evidence) (11):** [`RFC2784-2.2-1`](#rfc2784-2.2-1), [`RFC2784-2.3-1`](#rfc2784-2.3-1), [`RFC2784-2.3-2`](#rfc2784-2.3-2), [`RFC2784-2.3-3`](#rfc2784-2.3-3), [`RFC2784-2.3.1-1`](#rfc2784-2.3.1-1), [`RFC2784-2.6-1`](#rfc2784-2.6-1), [`RFC2784-3-1`](#rfc2784-3-1), [`RFC2784-3.1-1`](#rfc2784-3.1-1), [`RFC2784-3.1-2`](#rfc2784-3.1-2), [`RFC2784-3.1-3`](#rfc2784-3.1-3), [`RFC2784-5.2-1`](#rfc2784-5.2-1)

## Figures

- **MUST bar**: Not built 11
- **Overview bar**: Not assessed 2, Not built 11

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 11 |
| MUST rows kept (the MUST bar) | 11 |
| Ze proven | 0 |
| OS verified | 0 |
| Proof pending | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 11 |
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
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 2 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

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

A test reads unproven when no record shows it failing after a deliberate fault. Until then, its claim to check the requirement remains unproven.

### [`RFC2784-2.2-1`](#rfc2784-2.2-1)

Note that a compliant implementation MUST accept and process this field. (§2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2784-2.2-1, so there is no linked test.

### [`RFC2784-2.3-1`](#rfc2784-2.3-1)

A receiver MUST discard a packet where any of bits 1-5 are non-zero, unless that receiver implements RFC 1701. (§2.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2784-2.3-1, so there is no linked test.

### [`RFC2784-2.3-2`](#rfc2784-2.3-2)

Bits 6-12 are reserved for future use. These bits MUST be sent as zero (§2.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2784-2.3-2, so there is no linked test.

### [`RFC2784-2.3-3`](#rfc2784-2.3-3)

Bits 6-12 are reserved for future use. These bits MUST be sent as zero and MUST be ignored on receipt. (§2.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2784-2.3-3, so there is no linked test.

### [`RFC2784-2.3.1-1`](#rfc2784-2.3.1-1)

Version Number field MUST contain the value zero (§2.3.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2784-2.3.1-1, so there is no linked test.

### [`RFC2784-2.6-1`](#rfc2784-2.6-1)

The Reserved1 field is reserved for future use, and if present, MUST be transmitted as zero. (§2.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2784-2.6-1, so there is no linked test.

### [`RFC2784-3-1`](#rfc2784-3-1)

When IPv4 is being carried as the GRE payload, the Protocol Type field MUST be set to 0x800. (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2784-3-1, so there is no linked test.

### [`RFC2784-3.1-1`](#rfc2784-3.1-1)

When a tunnel endpoint decapsulates a GRE packet which has an IPv4 packet as the payload, the destination address in the IPv4 payload packet header MUST be used to forward the packet (§3.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2784-3.1-1, so there is no linked test.

### [`RFC2784-3.1-2`](#rfc2784-3.1-2)

When a tunnel endpoint decapsulates a GRE packet which has an IPv4 packet as the payload, the destination address in the IPv4 payload packet header MUST be used to forward the packet and the TTL of the payload packet MUST be decremented. (§3.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2784-3.1-2, so there is no linked test.

### [`RFC2784-3.1-3`](#rfc2784-3.1-3)

Care should be taken when forwarding such a packet, since if the destination address of the payload packet is the encapsulator of the packet (i.e., the other end of the tunnel), looping can occur. In this case, the packet MUST be discarded. (§3.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2784-3.1-3, so there is no linked test.

### [`RFC2784-5.2-1`](#rfc2784-5.2-1)

As stated in Section 5.3, a packet with non-zero bits in any of bits 1-5 MUST be discarded unless the receiver implements RFC 1701. (§5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2784-5.2-1, so there is no linked test.

## Requirement list review

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
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 1 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `2.1` | not stated | 0 | reviewed | not stated |
| `2.2` | not stated | 1 | reviewed | not stated |
| `2.3` | not stated | 2 | reviewed | not stated |
| `2.3.1` | not stated | 1 | reviewed | not stated |
| `2.4` | not stated | 0 | reviewed | not stated |
| `2.5` | not stated | 0 | reviewed | not stated |
| `2.6` | not stated | 1 | reviewed | not stated |
| `3` | not stated | 1 | reviewed | not stated |
| `3.1` | not stated | 2 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `5.1` | not stated | 0 | reviewed | not stated |
| `5.2` | not stated | 1 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `7.1` | not stated | 0 | reviewed | not stated |
| `7.2` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `10` | not stated | 0 | reviewed | not stated |
| `11` | not stated | 0 | reviewed | not stated |
| `12` | not stated | 1 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | RFC 2119 boilerplate in the Introduction: it declares how the keywords are to be interpreted and states no protocol obligation. | The keywords MUST, MUST NOT, MAY, OPTIONAL, REQUIRED, RECOMMENDED, SHALL, SHALL NOT, SHOULD, SHOULD NOT are to be interpreted as defined in RFC 2119 [RFC2119]. |
| `12:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | ISOC Full Copyright Statement. The lowercase 'must be followed' governs redistribution of the document, not GRE behaviour. | However, this document itself may not be modified in any way, such as by removing the copyright notice or references to the Internet Society or other Internet organizations, except as needed for the purpose of developing Internet standards in which case the procedures for copyrights defined in the Internet Standards process must be followed, or as required to translate it into languages other than English. |

## Superseded

No document obsoletes RFC 2784, so its obligations are stated where they were written.
