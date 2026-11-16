# RFC 4862 - IPv6 Stateless Address Autoconfiguration

IPv6 stateless address autoconfiguration. Every requirement listed for RFC 4862, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 16 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 16 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 16 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 16 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 0 | 0 of 0 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 10 | of 16 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 16 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 16 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 6 | of 16 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 16 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 16 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 16 | of 30 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 16 mandatory requirements: they add up to 16. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | IPv6 stateless address autoconfiguration |
| Automatic checks | Included |
| Requirements | 30 |
| Mandatory requirements (MUST level) | 16 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 10 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tests linked to requirements | 0 |
| Recorded audit verdicts | 0 |
| Fault test records | 0 |
| Summary | `rfc/short/rfc4862.md` |
| Requirement table | `rfc/requirements/rfc4862.md` |
| RFC text | `rfc/full/rfc4862.txt` |

## Automatic checks

Included in automatic checks: IPv6 Stateless Address Autoconfiguration. Ze is a router on every interface and a host where the operator enables autoconf and accept-ra (netlink backend only). Linux addrconf (linux:CONFIG_IPV6) forms the addresses and runs DAD; ze writes the autoconf, accept-ra and forwarding sysctls (internal/component/iface/config_sysctl.go), exposes any other kernel key through its sysctl setting list, and reads the results over netlink. Six MUSTs are lower-layer rows on Linux autoconfiguration and address lifetimes, which no VPP path carries. Ten are gaps: the nine DAD rows hold on Linux but are unverified on the VPP interface backend, whose AddAddress runs no DAD, and the VPP interface backend runs no Router Advertisement processing (5.5-2). On Linux that processing is on by default: ze writes no accept-ra or autoconf sysctl for an unset leaf, so the kernel defaults accept_ra = 1 and autoconf = 1 stay in force, honoured only while forwarding is off

## Published support status

**Area:** IPv6 stateless address autoconfiguration

**Test verification:** 0 of 16 MUST proven

**Evidence:** -

**Unmet:** RFC4862-5.1-1, RFC4862-5.4-1, RFC4862-5.4-2, RFC4862-5.4-5, RFC4862-5.4.1-1, RFC4862-5.4.2-1, RFC4862-5.4.2-4, RFC4862-5.4.3-1, RFC4862-5.4.5-1, RFC4862-5.5-2


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 16 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **16** | every mandatory requirement appears in exactly one group above |

**Annotated (including scoped evidence) (16):** [`RFC4862-5.1-1`](#rfc4862-5.1-1), [`RFC4862-5.4-1`](#rfc4862-5.4-1), [`RFC4862-5.4-2`](#rfc4862-5.4-2), [`RFC4862-5.4-5`](#rfc4862-5.4-5), [`RFC4862-5.4.1-1`](#rfc4862-5.4.1-1), [`RFC4862-5.4.2-1`](#rfc4862-5.4.2-1), [`RFC4862-5.4.2-4`](#rfc4862-5.4.2-4), [`RFC4862-5.4.3-1`](#rfc4862-5.4.3-1), [`RFC4862-5.4.5-1`](#rfc4862-5.4.5-1), [`RFC4862-5.5-2`](#rfc4862-5.5-2), [`RFC4862-5.5.3-2`](#rfc4862-5.5.3-2), [`RFC4862-5.5.4-3`](#rfc4862-5.5.4-3), [`RFC4862-5.5.4-5`](#rfc4862-5.5.4-5), [`RFC4862-5.5.4-6`](#rfc4862-5.5.4-6), [`RFC4862-5.5.4-7`](#rfc4862-5.5.4-7), [`RFC4862-5.5.4-8`](#rfc4862-5.5.4-8)

## Figures

- **MUST bar**: OS testing gap 6, Not built 10
- **Overview bar**: OS testing gap 6, Not assessed 14, Not built 10

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 16 |
| MUST rows kept (the MUST bar) | 16 |
| Ze verified | 0 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 6 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 10 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 14 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 14 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc4862.json](../../../data/rfc/rfc4862.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4862-5.1-1` | A node MUST allow the following autoconfiguration-related variable to be configured by system management for each multicast-capable interface: DupAddrDetectTransmits The number of consecutive Neighbor Solicitation messages sent while performing Duplicate Address Detection on a tentative address. (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel keeps DupAddrDetectTransmits per interface as net.ipv6.conf.<if>.dad_transmits (net/ipv6/addrconf.c, linux:CONFIG_IPV6), and an operator sets it through ze's sysctl setting list (internal/component/sysctl), which writes any kernel key; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) is a shipped interface backend that offers no such variable, since its internal/plugins/iface/vpp/ifacevpp.go::AddAddress installs addresses with no DAD step and VPP's address dump reports no DAD outcome (internal/plugins/iface/vpp/query.go::fillAddresses) |
| `RFC4862-5.4-1` | Duplicate Address Detection MUST be performed on all unicast addresses prior to assigning them to an interface, regardless of whether they are obtained through stateless autoconfiguration, DHCPv6, or manual configuration (§5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel runs DAD (net/ipv6/addrconf.c addrconf_dad_begin and addrconf_dad_work, linux:CONFIG_IPV6) on every unicast address it forms and on every address ze adds through netlink, because ze sets no IFA_F_NODAD (the DHCPv6 client relies on this, internal/plugins/iface/dhcp/dhcp_v6_attach_linux.go); no test covers the manual and autoconfigured cases. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) is a shipped interface backend whose internal/plugins/iface/vpp/ifacevpp.go::AddAddress installs addresses through sw_interface_add_del_address with no DAD step, VPP's address dump reports no DAD outcome (internal/plugins/iface/vpp/query.go::fillAddresses), and no test shows VPP performs DAD; the DHCPv6 leased-address half is owned by plan/release/fixes/spec-dhcpv6-dad-on-vpp.md |
| `RFC4862-5.4-2` | Duplicate Address Detection MUST NOT be performed on anycast addresses (§5.4) | MUST NOT | 5.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel holds anycast addresses apart from unicast ones (net/ipv6/anycast.c) and starts DAD only for a unicast inet6_ifaddr (linux:CONFIG_IPV6), and ze adds addresses through netlink as unicast only; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs addresses through internal/plugins/iface/vpp/ifacevpp.go::AddAddress, and no test shows what VPP does for an anycast address |
| `RFC4862-5.4-5` | Note that there are implementations deployed that only perform Duplicate Address Detection for the link-local address and skip the test for the global address that uses the same interface identifier as that of the link-local address. Whereas this document does not invalidate such implementations, this kind of "optimization" is NOT RECOMMENDED, and new implementations MUST NOT do that optimization. (§5.4) | MUST NOT | 5.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel starts DAD for each address it creates, a global address that shares the link-local interface identifier included (net/ipv6/addrconf.c addrconf_dad_begin, linux:CONFIG_IPV6), with no link-local-only shortcut; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs addresses through internal/plugins/iface/vpp/ifacevpp.go::AddAddress with no DAD step, and no test shows VPP tests any address |
| `RFC4862-5.4.1-1` | A node MUST silently discard any Neighbor Solicitation or Advertisement message that does not pass the validity checks specified in [RFC4861] (§5.4.1) | MUST | 5.4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel's ndisc_recv_ns and ndisc_recv_na (net/ipv6/ndisc.c, linux:CONFIG_IPV6) drop a Neighbor Solicitation or Advertisement that fails the RFC 4861 validity checks before it can affect a tentative address; no test covers it. The VPP half is unverified: on iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) VPP's own neighbor discovery receives these messages, and no test shows it discards an invalid one |
| `RFC4862-5.4.2-1` | Before sending a Neighbor Solicitation, an interface MUST join the all-nodes multicast address and the solicited-node multicast address of the tentative address (§5.4.2) | MUST | 5.4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel joins the all-nodes group when it brings IPv6 up on the device, and addrconf_dad_begin joins the tentative address's solicited-node group (addrconf_join_solict, net/ipv6/addrconf.c, linux:CONFIG_IPV6) before addrconf_dad_work sends the first solicitation; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs addresses through internal/plugins/iface/vpp/ifacevpp.go::AddAddress with no DAD step, so nothing shows VPP joins these groups before any solicitation |
| `RFC4862-5.4.2-4` | In order to improve the robustness of the Duplicate Address Detection algorithm, an interface MUST receive and process datagrams sent to the all-nodes multicast address or solicited-node multicast address of the tentative address during the delay period. (§5.4.2) | MUST | 5.4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a gap in proof on one of the two interface backends. The Linux path is the verified half: once addrconf_dad_begin has joined the groups (net/ipv6/addrconf.c, linux:CONFIG_IPV6), the kernel's ndisc_recv_ns and ndisc_recv_na (net/ipv6/ndisc.c) process datagrams to them during the delay and check them against the tentative address; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) runs no DAD for the addresses internal/plugins/iface/vpp/ifacevpp.go::AddAddress installs, and no test shows VPP processes these datagrams for a tentative address |
| `RFC4862-5.4.3-1` | In all cases, a node MUST NOT respond to a Neighbor Solicitation for a tentative address (§5.4.3) | MUST NOT | 5.4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel's ndisc_recv_ns (net/ipv6/ndisc.c, linux:CONFIG_IPV6) never answers a solicitation for a tentative address; it runs addrconf_dad_failure or drops the solicitation instead; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs addresses through internal/plugins/iface/vpp/ifacevpp.go::AddAddress with no tentative state, so VPP answers a solicitation for an address that was never tested, and no test covers it |
| `RFC4862-5.4.5-1` | A tentative address that is determined to be a duplicate as described above MUST NOT be assigned to an interface (§5.4.5) | MUST NOT | 5.4.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel's addrconf_dad_failure (net/ipv6/addrconf.c, linux:CONFIG_IPV6) marks a duplicate tentative address IFA_F_DADFAILED and never makes it usable, and ze reads that state through netlink (internal/plugins/iface/netlink/show_linux.go); no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs addresses through internal/plugins/iface/vpp/ifacevpp.go::AddAddress with no DAD step, so a duplicate address is assigned, and no test covers it |
| `RFC4862-5.5-2` | However, the processing described below MUST be enabled by default. (§5.5) | MUST | 5.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** unmet on the VPP interface backend, where no Router Advertisement processing runs: the autoconf and accept-ra leaves carry ze:backend netlink (internal/component/iface/yang/ze-iface-conf.yang), internal/plugins/iface/vpp configures no address autoconfiguration, and no advertisement reaches the Linux Control Plane tap. On Linux the requirement is met by the kernel defaults when ze writes nothing: internal/component/iface/config.go::parseIPv6Settings sets AcceptRA and Autoconf only when their leaves are present, internal/component/iface/config_sysctl.go::applySysctl writes net.ipv6.conf.<if>.accept_ra and autoconf only when they are set, and no code applies the YANG default 0 of accept-ra to unit ipv6 (docs/architecture/config/syntax.md, "YANG Defaults Do Not Reach a Config On Their Own"), so the kernel defaults accept_ra = 1 and autoconf = 1 (net/ipv6/addrconf.c, ipv6_devconf and ipv6_devconf_dflt, linux:CONFIG_IPV6) stay in force. The kernel honours accept_ra = 1 only while forwarding is off (include/net/ipv6.h::ipv6_accept_ra), and internal/plugins/fib/kernel/fibkernel.go::emitForwardingDefaults sets net.ipv6.conf.all.forwarding to 1, so with fib-kernel loaded the processing runs only where the operator writes accept-ra 2. A test is owed that ze writes no accept_ra for an unset leaf |
| `RFC4862-5.5.3-2` | If the sum of the prefix length and interface identifier length does not equal 128 bits, the Prefix Information option MUST be ignored (§5.5.3) | MUST | 5.5.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 addrconf (net/ipv6/addrconf.c, addrconf_prefix_rcv), linux:CONFIG_IPV6; internal/component/iface/config_sysctl.go::applySysctl writes the autoconf and accept-ra leaves into net.ipv6.conf.<if>.autoconf and accept_ra, and when they let the kernel process a Prefix Information option it forms an address only for a 64-bit prefix, the length of its interface identifier, and ignores any other length for autoconfiguration; those two values decide whether the kernel processes the option at all, so a test over them is owed. Both leaves are netlink-only (ze:backend netlink), so VPP processes no advertisement on ze's behalf |
| `RFC4862-5.5.4-3` | IP and higher layers (e.g., TCP, UDP) MUST continue to accept and process datagrams destined to a deprecated address as normal (§5.5.4) | MUST | 5.5.4 - Four sites | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 stack (net/ipv6/addrconf.c and net/ipv6/ip6_input.c), linux:CONFIG_IPV6; internal/component/iface/config_sysctl.go::applySysctl writes the autoconf and accept-ra leaves that let the kernel form addresses with lifetimes, and internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the other addresses; a deprecated address stays on the device, and the kernel keeps accepting and delivering datagrams sent to it until its valid lifetime ends, with no value ze writes changing the acceptance. VPP holds no address lifetime (internal/plugins/iface/vpp/ifacevpp.go::AddAddress carries none), so no address deprecates there |
| `RFC4862-5.5.4-5` | An implementation MAY prevent any new communication from using a deprecated address, but system management MUST have the ability to disable such a facility (§5.5.4) | MUST | 5.5.4 - Four sites | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 source address selection (net/ipv6/addrconf.c, ipv6_dev_get_saddr), linux:CONFIG_IPV6; internal/component/iface/config_sysctl.go::applySysctl installs the IPv6 autoconfiguration sysctls (autoconf, accept-ra and forwarding only), and the kernel offers no facility that prevents new communication from a deprecated address: its rule 3 only prefers a preferred address and still selects a deprecated one when nothing better exists, so there is no facility for system management to disable, and ze adds none. VPP holds no address lifetime (internal/plugins/iface/vpp/ifacevpp.go::AddAddress carries none), so no address deprecates there |
| `RFC4862-5.5.4-6` | An implementation MAY prevent any new communication from using a deprecated address, but system management MUST have the ability to disable such a facility, and the facility MUST be disabled by default. (§5.5.4) | MUST | 5.5.4 - Four sites | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 source address selection (net/ipv6/addrconf.c, ipv6_dev_get_saddr), linux:CONFIG_IPV6; internal/component/iface/config_sysctl.go::applySysctl installs the IPv6 autoconfiguration sysctls and nothing else for this, and the kernel carries no facility that blocks new communication from a deprecated address, so none is ever enabled by default, and ze adds none. VPP holds no address lifetime (internal/plugins/iface/vpp/ifacevpp.go::AddAddress carries none), so no address deprecates there |
| `RFC4862-5.5.4-7` | An invalid address MUST NOT be used as a source address in outgoing communications (§5.5.4) | MUST NOT | 5.5.4 - Four sites | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 addrconf (net/ipv6/addrconf.c, addrconf_verify_rtnl), linux:CONFIG_IPV6; internal/component/iface/config_sysctl.go::applySysctl writes the autoconf and accept-ra leaves under which the kernel forms addresses with lifetimes, and the kernel deletes an address whose valid lifetime has expired (ipv6_del_addr), so an invalid address is never left to be chosen as a source; ze reads the lifetimes through netlink (internal/plugins/iface/netlink/slaac_linux.go) and writes no value that keeps an expired address. VPP holds no address lifetime (internal/plugins/iface/vpp/ifacevpp.go::AddAddress carries none), so no address becomes invalid there |
| `RFC4862-5.5.4-8` | An invalid address MUST NOT be used as a source address in outgoing communications and MUST NOT be recognized as a destination on a receiving interface. (§5.5.4) | MUST NOT | 5.5.4 - Four sites | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 addrconf (net/ipv6/addrconf.c, addrconf_verify_rtnl), linux:CONFIG_IPV6; internal/component/iface/config_sysctl.go::applySysctl writes the autoconf and accept-ra leaves under which the kernel forms addresses with lifetimes, and the kernel deletes an address whose valid lifetime has expired (ipv6_del_addr), so it neither sources from it nor recognises it as a destination on input; ze writes no value that keeps an expired address (internal/plugins/iface/netlink/slaac_linux.go reads the lifetimes). VPP holds no address lifetime (internal/plugins/iface/vpp/ifacevpp.go::AddAddress carries none), so no address becomes invalid there |
| `RFC4862-5.4-3` | Each individual unicast address SHOULD be tested for uniqueness (§5.4) | SHOULD | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.4-4` | Note that there are implementations deployed that only perform Duplicate Address Detection for the link-local address and skip the test for the global address that uses the same interface identifier as that of the link-local address. Whereas this document does not invalidate such implementations, this kind of "optimization" is NOT RECOMMENDED (§5.4) | NOT RECOMMENDED | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.4.2-2` | If the Neighbor Solicitation is going to be the first message sent from an interface after interface (re)initialization, the node SHOULD delay joining the solicited-node multicast address by a random delay between 0 and MAX_RTR_SOLICITATION_DELAY as specified in [RFC4861]. (§5.4.2) | SHOULD | 5.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.4.2-3` | Even if the Neighbor Solicitation is not going to be the first message sent, the node SHOULD delay joining the solicited-node multicast address by a random delay between 0 and MAX_RTR_SOLICITATION_DELAY if the address being checked is configured by a router advertisement message sent to a multicast address. (§5.4.2) | SHOULD | 5.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.4.5-2` | A tentative address that is determined to be a duplicate as described above MUST NOT be assigned to an interface, and the node SHOULD log a system management error. (§5.4.5) | SHOULD | 5.4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.4.5-3` | If the address is a link-local address formed from an interface identifier based on the hardware address, which is supposed to be uniquely assigned (e.g., EUI-64 for an Ethernet interface), IP operation on the interface SHOULD be disabled. (§5.4.5) | SHOULD | 5.4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.5-1` | Creation of global addresses as described in this section SHOULD be locally configurable (§5.5) | SHOULD | 5.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.5.4-1` | A deprecated address SHOULD continue to be used as a source address in existing communications (§5.5.4) | SHOULD | 5.5.4 - Four sites | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.5.4-2` | A deprecated address SHOULD continue to be used as a source address in existing communications, but SHOULD NOT be used to initiate new communications if an alternate (non-deprecated) address of sufficient scope can easily be used instead. (§5.5.4) | SHOULD NOT | 5.5.4 - Four sites | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.6-1` | In any case, if there is no security difference, the most recently obtained values SHOULD have precedence over information learned earlier. (§5.6) | SHOULD | 5.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.4.5-4` | On the other hand, if the duplicate link-local address is not formed from an interface identifier based on the hardware address, which is supposed to be uniquely assigned, IP operation on the interface MAY be continued. (§5.4.5) | MAY | 5.4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.5.3-1` | If the preferred lifetime is greater than the valid lifetime, silently ignore the Prefix Information option. A node MAY wish to log a system management error in this case. (§5.5.3) | MAY | 5.5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.5.3-3` | If the sum of the prefix length and interface identifier length does not equal 128 bits, the Prefix Information option MUST be ignored. An implementation MAY wish to log a system management error in this case. (§5.5.3) | MAY | 5.5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.5.4-4` | An implementation MAY prevent any new communication from using a deprecated address (§5.5.4) | MAY | 5.5.4 - Four sites | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4862-5.1-1`](#rfc4862-5.1-1) A node MUST allow the following autoconfiguration-related variable to be configured by system management for each multicast-capable interface: DupAddrDetectTransmits The number of consecutive Neighbor Solicitation messages sent while performing Duplicate Address Detection on a tentative address. (§5.1) | {gap}, no test | a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel keeps DupAddrDetectTransmits per interface as net.ipv6.conf.<if>.dad_transmits (net/ipv6/addrconf.c, linux:CONFIG_IPV6), and an operator sets it through ze's sysctl setting list (internal/component/sysctl), which writes any kernel key; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) is a shipped interface backend that offers no such variable, since its internal/plugins/iface/vpp/ifacevpp.go::AddAddress installs addresses with no DAD step and VPP's address dump reports no DAD outcome (internal/plugins/iface/vpp/query.go::fillAddresses) |
| [`RFC4862-5.4-1`](#rfc4862-5.4-1) Duplicate Address Detection MUST be performed on all unicast addresses prior to assigning them to an interface, regardless of whether they are obtained through stateless autoconfiguration, DHCPv6, or manual configuration (§5.4) | {gap}, no test | a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel runs DAD (net/ipv6/addrconf.c addrconf_dad_begin and addrconf_dad_work, linux:CONFIG_IPV6) on every unicast address it forms and on every address ze adds through netlink, because ze sets no IFA_F_NODAD (the DHCPv6 client relies on this, internal/plugins/iface/dhcp/dhcp_v6_attach_linux.go); no test covers the manual and autoconfigured cases. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) is a shipped interface backend whose internal/plugins/iface/vpp/ifacevpp.go::AddAddress installs addresses through sw_interface_add_del_address with no DAD step, VPP's address dump reports no DAD outcome (internal/plugins/iface/vpp/query.go::fillAddresses), and no test shows VPP performs DAD; the DHCPv6 leased-address half is owned by plan/release/fixes/spec-dhcpv6-dad-on-vpp.md |
| [`RFC4862-5.4-2`](#rfc4862-5.4-2) Duplicate Address Detection MUST NOT be performed on anycast addresses (§5.4) | {gap}, no test | a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel holds anycast addresses apart from unicast ones (net/ipv6/anycast.c) and starts DAD only for a unicast inet6_ifaddr (linux:CONFIG_IPV6), and ze adds addresses through netlink as unicast only; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs addresses through internal/plugins/iface/vpp/ifacevpp.go::AddAddress, and no test shows what VPP does for an anycast address |
| [`RFC4862-5.4-5`](#rfc4862-5.4-5) Note that there are implementations deployed that only perform Duplicate Address Detection for the link-local address and skip the test for the global address that uses the same interface identifier as that of the link-local address. Whereas this document does not invalidate such implementations, this kind of "optimization" is NOT RECOMMENDED, and new implementations MUST NOT do that optimization. (§5.4) | {gap}, no test | a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel starts DAD for each address it creates, a global address that shares the link-local interface identifier included (net/ipv6/addrconf.c addrconf_dad_begin, linux:CONFIG_IPV6), with no link-local-only shortcut; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs addresses through internal/plugins/iface/vpp/ifacevpp.go::AddAddress with no DAD step, and no test shows VPP tests any address |
| [`RFC4862-5.4.1-1`](#rfc4862-5.4.1-1) A node MUST silently discard any Neighbor Solicitation or Advertisement message that does not pass the validity checks specified in [RFC4861] (§5.4.1) | {gap}, no test | a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel's ndisc_recv_ns and ndisc_recv_na (net/ipv6/ndisc.c, linux:CONFIG_IPV6) drop a Neighbor Solicitation or Advertisement that fails the RFC 4861 validity checks before it can affect a tentative address; no test covers it. The VPP half is unverified: on iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) VPP's own neighbor discovery receives these messages, and no test shows it discards an invalid one |
| [`RFC4862-5.4.2-1`](#rfc4862-5.4.2-1) Before sending a Neighbor Solicitation, an interface MUST join the all-nodes multicast address and the solicited-node multicast address of the tentative address (§5.4.2) | {gap}, no test | a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel joins the all-nodes group when it brings IPv6 up on the device, and addrconf_dad_begin joins the tentative address's solicited-node group (addrconf_join_solict, net/ipv6/addrconf.c, linux:CONFIG_IPV6) before addrconf_dad_work sends the first solicitation; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs addresses through internal/plugins/iface/vpp/ifacevpp.go::AddAddress with no DAD step, so nothing shows VPP joins these groups before any solicitation |
| [`RFC4862-5.4.2-4`](#rfc4862-5.4.2-4) In order to improve the robustness of the Duplicate Address Detection algorithm, an interface MUST receive and process datagrams sent to the all-nodes multicast address or solicited-node multicast address of the tentative address during the delay period. (§5.4.2) | {gap}, no test | a gap in proof on one of the two interface backends. The Linux path is the verified half: once addrconf_dad_begin has joined the groups (net/ipv6/addrconf.c, linux:CONFIG_IPV6), the kernel's ndisc_recv_ns and ndisc_recv_na (net/ipv6/ndisc.c) process datagrams to them during the delay and check them against the tentative address; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) runs no DAD for the addresses internal/plugins/iface/vpp/ifacevpp.go::AddAddress installs, and no test shows VPP processes these datagrams for a tentative address |
| [`RFC4862-5.4.3-1`](#rfc4862-5.4.3-1) In all cases, a node MUST NOT respond to a Neighbor Solicitation for a tentative address (§5.4.3) | {gap}, no test | a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel's ndisc_recv_ns (net/ipv6/ndisc.c, linux:CONFIG_IPV6) never answers a solicitation for a tentative address; it runs addrconf_dad_failure or drops the solicitation instead; no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs addresses through internal/plugins/iface/vpp/ifacevpp.go::AddAddress with no tentative state, so VPP answers a solicitation for an address that was never tested, and no test covers it |
| [`RFC4862-5.4.5-1`](#rfc4862-5.4.5-1) A tentative address that is determined to be a duplicate as described above MUST NOT be assigned to an interface (§5.4.5) | {gap}, no test | a gap in proof on one of the two interface backends. The Linux path is the verified half: the kernel's addrconf_dad_failure (net/ipv6/addrconf.c, linux:CONFIG_IPV6) marks a duplicate tentative address IFA_F_DADFAILED and never makes it usable, and ze reads that state through netlink (internal/plugins/iface/netlink/show_linux.go); no test covers it. The VPP half is unverified: iface-vpp (internal/plugins/iface/vpp/register.go, ze_vpp feature gate) installs addresses through internal/plugins/iface/vpp/ifacevpp.go::AddAddress with no DAD step, so a duplicate address is assigned, and no test covers it |
| [`RFC4862-5.5-2`](#rfc4862-5.5-2) However, the processing described below MUST be enabled by default. (§5.5) | {gap}, no test | unmet on the VPP interface backend, where no Router Advertisement processing runs: the autoconf and accept-ra leaves carry ze:backend netlink (internal/component/iface/yang/ze-iface-conf.yang), internal/plugins/iface/vpp configures no address autoconfiguration, and no advertisement reaches the Linux Control Plane tap. On Linux the requirement is met by the kernel defaults when ze writes nothing: internal/component/iface/config.go::parseIPv6Settings sets AcceptRA and Autoconf only when their leaves are present, internal/component/iface/config_sysctl.go::applySysctl writes net.ipv6.conf.<if>.accept_ra and autoconf only when they are set, and no code applies the YANG default 0 of accept-ra to unit ipv6 (docs/architecture/config/syntax.md, "YANG Defaults Do Not Reach a Config On Their Own"), so the kernel defaults accept_ra = 1 and autoconf = 1 (net/ipv6/addrconf.c, ipv6_devconf and ipv6_devconf_dflt, linux:CONFIG_IPV6) stay in force. The kernel honours accept_ra = 1 only while forwarding is off (include/net/ipv6.h::ipv6_accept_ra), and internal/plugins/fib/kernel/fibkernel.go::emitForwardingDefaults sets net.ipv6.conf.all.forwarding to 1, so with fib-kernel loaded the processing runs only where the operator writes accept-ra 2. A test is owed that ze writes no accept_ra for an unset leaf |
| [`RFC4862-5.5.3-2`](#rfc4862-5.5.3-2) If the sum of the prefix length and interface identifier length does not equal 128 bits, the Prefix Information option MUST be ignored (§5.5.3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 addrconf (net/ipv6/addrconf.c, addrconf_prefix_rcv), linux:CONFIG_IPV6; internal/component/iface/config_sysctl.go::applySysctl writes the autoconf and accept-ra leaves into net.ipv6.conf.<if>.autoconf and accept_ra, and when they let the kernel process a Prefix Information option it forms an address only for a 64-bit prefix, the length of its interface identifier, and ignores any other length for autoconfiguration; those two values decide whether the kernel processes the option at all, so a test over them is owed. Both leaves are netlink-only (ze:backend netlink), so VPP processes no advertisement on ze's behalf |
| [`RFC4862-5.5.4-3`](#rfc4862-5.5.4-3) IP and higher layers (e.g., TCP, UDP) MUST continue to accept and process datagrams destined to a deprecated address as normal (§5.5.4) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 stack (net/ipv6/addrconf.c and net/ipv6/ip6_input.c), linux:CONFIG_IPV6; internal/component/iface/config_sysctl.go::applySysctl writes the autoconf and accept-ra leaves that let the kernel form addresses with lifetimes, and internal/plugins/iface/netlink/manage_linux.go::AddAddress installs the other addresses; a deprecated address stays on the device, and the kernel keeps accepting and delivering datagrams sent to it until its valid lifetime ends, with no value ze writes changing the acceptance. VPP holds no address lifetime (internal/plugins/iface/vpp/ifacevpp.go::AddAddress carries none), so no address deprecates there |
| [`RFC4862-5.5.4-5`](#rfc4862-5.5.4-5) An implementation MAY prevent any new communication from using a deprecated address, but system management MUST have the ability to disable such a facility (§5.5.4) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 source address selection (net/ipv6/addrconf.c, ipv6_dev_get_saddr), linux:CONFIG_IPV6; internal/component/iface/config_sysctl.go::applySysctl installs the IPv6 autoconfiguration sysctls (autoconf, accept-ra and forwarding only), and the kernel offers no facility that prevents new communication from a deprecated address: its rule 3 only prefers a preferred address and still selects a deprecated one when nothing better exists, so there is no facility for system management to disable, and ze adds none. VPP holds no address lifetime (internal/plugins/iface/vpp/ifacevpp.go::AddAddress carries none), so no address deprecates there |
| [`RFC4862-5.5.4-6`](#rfc4862-5.5.4-6) An implementation MAY prevent any new communication from using a deprecated address, but system management MUST have the ability to disable such a facility, and the facility MUST be disabled by default. (§5.5.4) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 source address selection (net/ipv6/addrconf.c, ipv6_dev_get_saddr), linux:CONFIG_IPV6; internal/component/iface/config_sysctl.go::applySysctl installs the IPv6 autoconfiguration sysctls and nothing else for this, and the kernel carries no facility that blocks new communication from a deprecated address, so none is ever enabled by default, and ze adds none. VPP holds no address lifetime (internal/plugins/iface/vpp/ifacevpp.go::AddAddress carries none), so no address deprecates there |
| [`RFC4862-5.5.4-7`](#rfc4862-5.5.4-7) An invalid address MUST NOT be used as a source address in outgoing communications (§5.5.4) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 addrconf (net/ipv6/addrconf.c, addrconf_verify_rtnl), linux:CONFIG_IPV6; internal/component/iface/config_sysctl.go::applySysctl writes the autoconf and accept-ra leaves under which the kernel forms addresses with lifetimes, and the kernel deletes an address whose valid lifetime has expired (ipv6_del_addr), so an invalid address is never left to be chosen as a source; ze reads the lifetimes through netlink (internal/plugins/iface/netlink/slaac_linux.go) and writes no value that keeps an expired address. VPP holds no address lifetime (internal/plugins/iface/vpp/ifacevpp.go::AddAddress carries none), so no address becomes invalid there |
| [`RFC4862-5.5.4-8`](#rfc4862-5.5.4-8) An invalid address MUST NOT be used as a source address in outgoing communications and MUST NOT be recognized as a destination on a receiving interface. (§5.5.4) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 addrconf (net/ipv6/addrconf.c, addrconf_verify_rtnl), linux:CONFIG_IPV6; internal/component/iface/config_sysctl.go::applySysctl writes the autoconf and accept-ra leaves under which the kernel forms addresses with lifetimes, and the kernel deletes an address whose valid lifetime has expired (ipv6_del_addr), so it neither sources from it nor recognises it as a destination on input; ze writes no value that keeps an expired address (internal/plugins/iface/netlink/slaac_linux.go reads the lifetimes). VPP holds no address lifetime (internal/plugins/iface/vpp/ifacevpp.go::AddAddress carries none), so no address becomes invalid there |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC4862-5.1-1`](#rfc4862-5.1-1)

A node MUST allow the following autoconfiguration-related variable to be configured by system management for each multicast-capable interface: DupAddrDetectTransmits The number of consecutive Neighbor Solicitation messages sent while performing Duplicate Address Detection on a tentative address. (§5.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4862-5.1-1, so there is no linked test.

### [`RFC4862-5.4-1`](#rfc4862-5.4-1)

Duplicate Address Detection MUST be performed on all unicast addresses prior to assigning them to an interface, regardless of whether they are obtained through stateless autoconfiguration, DHCPv6, or manual configuration (§5.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4862-5.4-1, so there is no linked test.

### [`RFC4862-5.4-2`](#rfc4862-5.4-2)

Duplicate Address Detection MUST NOT be performed on anycast addresses (§5.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4862-5.4-2, so there is no linked test.

### [`RFC4862-5.4-5`](#rfc4862-5.4-5)

Note that there are implementations deployed that only perform Duplicate Address Detection for the link-local address and skip the test for the global address that uses the same interface identifier as that of the link-local address. Whereas this document does not invalidate such implementations, this kind of "optimization" is NOT RECOMMENDED, and new implementations MUST NOT do that optimization. (§5.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4862-5.4-5, so there is no linked test.

### [`RFC4862-5.4.1-1`](#rfc4862-5.4.1-1)

A node MUST silently discard any Neighbor Solicitation or Advertisement message that does not pass the validity checks specified in [RFC4861] (§5.4.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4862-5.4.1-1, so there is no linked test.

### [`RFC4862-5.4.2-1`](#rfc4862-5.4.2-1)

Before sending a Neighbor Solicitation, an interface MUST join the all-nodes multicast address and the solicited-node multicast address of the tentative address (§5.4.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4862-5.4.2-1, so there is no linked test.

### [`RFC4862-5.4.2-4`](#rfc4862-5.4.2-4)

In order to improve the robustness of the Duplicate Address Detection algorithm, an interface MUST receive and process datagrams sent to the all-nodes multicast address or solicited-node multicast address of the tentative address during the delay period. (§5.4.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4862-5.4.2-4, so there is no linked test.

### [`RFC4862-5.4.3-1`](#rfc4862-5.4.3-1)

In all cases, a node MUST NOT respond to a Neighbor Solicitation for a tentative address (§5.4.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4862-5.4.3-1, so there is no linked test.

### [`RFC4862-5.4.5-1`](#rfc4862-5.4.5-1)

A tentative address that is determined to be a duplicate as described above MUST NOT be assigned to an interface (§5.4.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4862-5.4.5-1, so there is no linked test.

### [`RFC4862-5.5-2`](#rfc4862-5.5-2)

However, the processing described below MUST be enabled by default. (§5.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4862-5.5-2, so there is no linked test.

### [`RFC4862-5.5.3-2`](#rfc4862-5.5.3-2)

If the sum of the prefix length and interface identifier length does not equal 128 bits, the Prefix Information option MUST be ignored (§5.5.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4862-5.5.3-2, so there is no linked test.

### [`RFC4862-5.5.4-3`](#rfc4862-5.5.4-3)

IP and higher layers (e.g., TCP, UDP) MUST continue to accept and process datagrams destined to a deprecated address as normal (§5.5.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4862-5.5.4-3, so there is no linked test.

### [`RFC4862-5.5.4-5`](#rfc4862-5.5.4-5)

An implementation MAY prevent any new communication from using a deprecated address, but system management MUST have the ability to disable such a facility (§5.5.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4862-5.5.4-5, so there is no linked test.

### [`RFC4862-5.5.4-6`](#rfc4862-5.5.4-6)

An implementation MAY prevent any new communication from using a deprecated address, but system management MUST have the ability to disable such a facility, and the facility MUST be disabled by default. (§5.5.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4862-5.5.4-6, so there is no linked test.

### [`RFC4862-5.5.4-7`](#rfc4862-5.5.4-7)

An invalid address MUST NOT be used as a source address in outgoing communications (§5.5.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4862-5.5.4-7, so there is no linked test.

### [`RFC4862-5.5.4-8`](#rfc4862-5.5.4-8)

An invalid address MUST NOT be used as a source address in outgoing communications and MUST NOT be recognized as a destination on a receiving interface. (§5.5.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4862-5.5.4-8, so there is no linked test.

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc4862.txt |
| Source fingerprint | d3b85654e334cbd7 |
| Record | rfc/extraction/rfc4862.json |
| Mapped sentences | 14 |
| Declined as scope | 18 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 2 | reviewed | not stated |
| `2.1` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 2 | reviewed | not stated |
| `4` | not stated | 2 | reviewed | not stated |
| `4.1` | not stated | 1 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `5.1` | not stated | 1 | reviewed | not stated |
| `5.2` | not stated | 0 | reviewed | not stated |
| `5.3` | not stated | 1 | reviewed | not stated |
| `5.4` | not stated | 5 | reviewed | not stated |
| `5.4.1` | not stated | 1 | reviewed | not stated |
| `5.4.2` | not stated | 3 | reviewed | not stated |
| `5.4.3` | not stated | 1 | reviewed | not stated |
| `5.4.4` | not stated | 0 | reviewed | not stated |
| `5.4.5` | not stated | 1 | reviewed | not stated |
| `5.5` | not stated | 1 | reviewed | not stated |
| `5.5.1` | not stated | 0 | reviewed | not stated |
| `5.5.2` | not stated | 1 | reviewed | not stated |
| `5.5.3` | not stated | 1 | reviewed | not stated |
| `5.5.4` | Four sites | 4 | reviewed | Four sites. Two of them carry two independent obligations each and a site maps to one id. Site 5.5.4:3 maps to RFC4862-5.5.4-5 (management MUST be able to disable the facility) and its second clause, "and the facility MUST be disabled by default", is RFC4862-5.5.4-6. Site 5.5.4:4 maps to RFC4862-5.5.4-7 (MUST NOT be used as a source address) and its second clause, "and MUST NOT be recognized as a destination on a receiving interface", is RFC4862-5.5.4-8. |
| `5.6` | not stated | 0 | reviewed | not stated |
| `5.7` | not stated | 1 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `8.1` | not stated | 0 | reviewed | not stated |
| `8.2` | not stated | 0 | reviewed | not stated |
| `A` | not stated | 2 | reviewed | not stated |
| `B` | not stated | 1 | reviewed | not stated |
| `C` | not stated | 1 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 2 is Terminology; the sentence is part of the definition of an address lifetime and carries no RFC 2119 keyword. | The valid lifetime must be greater than or equal to the preferred lifetime. |
| `2:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Terminology note reconciling this document's definitions with RFC 4291; descriptive, no RFC 2119 keyword. | Note that the address architecture [RFC4291] also defines the length of the interface identifiers for some set of addresses, but the two sets of definitions must be consistent. |
| `3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 3 is Design Goals; the bullet states a goal for the mechanism, not an obligation on an implementation. | o Manual configuration of individual machines before connecting them to the network should not be required. |
| `3:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 3 Design Goals; the sentence motivates the need for router advertisements and states no implementation obligation. | In order to generate global addresses, hosts must determine the prefixes that identify the subnets to which they attach. |
| `4:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4 is the Protocol Overview; this sentence restates the Duplicate Address Detection obligation Section 5.4 states normatively. | Before the link-local address can be assigned to an interface and used, however, a node must attempt to verify that this "tentative" address is not already in use by another node on the link. |
| `4:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 4 Protocol Overview; indicative prose describing the outcome of a link-local DAD failure, with no RFC 2119 keyword. | If a node determines that its tentative link-local address is not unique, autoconfiguration stops and manual configuration of the interface is required. |
| `4.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 4.1 discusses site renumbering; the sentence explains why addresses stay stable during a packet exchange and binds no implementation. | Even when applications use UDP as a transport protocol, addresses must generally remain the same during a packet exchange. |
| `5.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Indicative outcome prose with no RFC 2119 keyword: it states that autoconfiguration fails, and Section 2.1 confines this document's keywords to the normative statements. | If the sum of the link-local prefix length and N is larger than 128, autoconfiguration fails and manual configuration is required. |
| `5.4:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The sentence explains tentative-address packet handling that Section 5.4.2 states normatively as the MUST receive-and-process obligation. | That is, the interface must accept Neighbor Solicitation and Advertisement messages containing the tentative address in the Target Address field, but processes such packets differently from those whose Target Address matches an address assigned to the interface. |
| `5.4:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | "It should also be noted that" reintroduces the Section 5.4 DAD-before-assignment MUST already mapped. | It should also be noted that Duplicate Address Detection must be performed prior to assigning an address to an interface in order to prevent multiple nodes from using the same address simultaneously. |
| `5.4.2:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Explanatory note about why an MLD report is sent; it describes the purpose of a message RFC 4861 and RFC 2710 require, not an obligation of this document. | In the case of Duplicate Address Detection, the MLD report message is required in order to inform MLD- snooping switches, rather than routers, to forward multicast packets. |
| `5.5.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 5.5.2 explains the consequence of having no Router Advertisement; descriptive prose about operator action, no RFC 2119 keyword. | In this case, the forwarding node's address must be manually configured in hosts to be able to send packets off-link, since the only mechanism to configure the default router's address automatically is the one using Router Advertisements. |
| `5.5.4:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | An example illustrating the preceding SHOULD about deprecated source addresses; the sentence gives the reason for the SHOULD and states no separate obligation. | For example, if an application explicitly specifies that the protocol stack use a deprecated address as a source address, the protocol stack must accept that; the application might request it because that IP address is used in higher-level communication and there might be a requirement that the multiple connections in such a grouping use the same pair of IP addresses. |
| `5.7:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 5.7 describes an optional stable-storage extension ("An implementation that has stable storage may want to retain addresses in the storage") and the sentence opens "it should also be noted that"; it is advisory, and the section closes "Further details on this kind of extension are beyond the scope of this document." | When this technique is used, it should also be noted that the expiration times of the preferred and valid lifetimes must be retained, in order to prevent the use of an address after it has become deprecated or invalid. |
| `A:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Appendix A is informational: it discusses loopback suppression and DAD interaction and states no obligation. | o If a node performing Duplicate Address Detection discards received packets that have the same source link-layer address as the receiving interface, it will also discard packets from other nodes that also use the same link-layer address, including Neighbor Advertisement and Neighbor Solicitation messages required to make Duplicate Address Detection work correctly. |
| `A:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Appendix A informational discussion of multicast loopback semantics; no RFC 2119 keyword and no obligation of this document. | Thus, to perform Duplicate Address Detection correctly in the case where two interfaces are using the same link-layer address, an implementation must have a good understanding of the interface's multicast loopback semantics, and the interface cannot discard received packets simply because the source link-layer address is the same as the interface's. |
| `B:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Appendix B is the change log against RFC 2462; it describes an editorial clarification, not an obligation. | o Clarified wording in Section 5.5.4 to make clear that all upper layer protocols must process (i.e., send and receive) packets sent to deprecated addresses. |
| `C:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF IPR boilerplate. | The IETF invites any interested party to bring to its attention any copyrights, patents or patent applications, or other proprietary rights that may cover technology that may be required to implement this standard. |

## Superseded

No document obsoletes RFC 4862, so its obligations are stated where they were written.
