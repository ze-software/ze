# RFC 4213 - Basic Transition Mechanisms for IPv6 Hosts and Routers

IPv6 over IPv4 configured tunnels (6in4, sit). Every requirement listed for RFC 4213, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 1 | of 23 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 23 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 23 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 23 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 2 | 2 of 2 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 5 | of 23 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 23 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 23 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 17 | of 23 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 23 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 23 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 23 | of 47 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 23 mandatory requirements: they add up to 23. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Public row | IPv6 over IPv4 configured tunnels (6in4, sit) |
| Automatic checks | Included |
| Requirements | 47 |
| Mandatory requirements (MUST level) | 23 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 5 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 2 |
| Tests linked to requirements | 2 |
| Recorded audit verdicts | 0 |
| Fault test records | 2 |
| Summary | `rfc/short/rfc4213.md` |
| Requirement table | `rfc/requirements/rfc4213.md` |
| RFC text | `rfc/full/rfc4213.txt` |

## Automatic checks

Included in automatic checks: Basic Transition Mechanisms for IPv6 Hosts and Routers (RFC 4213): dual-stack plus configured 6in4 tunnels (protocol 41). Ze fills the configured-tunnel roles, encapsulator and decapsulator, by programming the sit tunnel netdev via netlink (internal/plugins/iface/netlink/tunnel_linux.go::buildSittun, Proto = IPPROTO_IPV6) with no sit VPP backend, and the Linux sit module (linux:CONFIG_IPV6_SIT) performs the 6in4 datapath: encapsulation, decapsulation, outer-source verification, MTU and DF, link-local assignment and neighbor discovery. Those MUSTs are lower-layer rows naming the values ze installs that the kernel reads (Remote, Local, PMtuDisc, Ttl, the mtu leaf), each owing a test over that value. The two Section 2.2 resolver MUSTs are gaps in proof: ze carries its own stub resolver (internal/component/resolve/dns/resolver.go) beside the Go standard library one, and no test shows it handles A and AAAA records and filters none on a request for both. Two MUSTs are unmet gaps: the default tunnel MTU follows the underlay and exceeds 1480 on a jumbo underlay, and nothing floors an operator's reassembly-buffer sysctl. The configuration-knob MUST (3.2.1-2) is met by ze's own mtu leaf and is a gap in proof until a test exercises it. Same delegation pattern as enrolled RFC 2003 and RFC 2473

## Published support status

**Area:** IPv6 over IPv4 configured tunnels (6in4, sit)

**Test verification:** 1 of 23 MUST proven

**Evidence:** unit 1

**Unmet:** RFC4213-2.2-1, RFC4213-2.2-2, RFC4213-3.2.1-1, RFC4213-3.2.1-2, RFC4213-3.6-5


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 22 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **23** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (1):** [`RFC4213-3.6-1`](#rfc4213-3.6-1)

**Annotated (including scoped evidence) (22):** [`RFC4213-2.2-1`](#rfc4213-2.2-1), [`RFC4213-2.2-2`](#rfc4213-2.2-2), [`RFC4213-3.2-1`](#rfc4213-3.2-1), [`RFC4213-3.2-2`](#rfc4213-3.2-2), [`RFC4213-3.2.1-1`](#rfc4213-3.2.1-1), [`RFC4213-3.2.1-2`](#rfc4213-3.2.1-2), [`RFC4213-3.2.1-3`](#rfc4213-3.2.1-3), [`RFC4213-3.2.1-4`](#rfc4213-3.2.1-4), [`RFC4213-3.2-3`](#rfc4213-3.2-3), [`RFC4213-3.6-2`](#rfc4213-3.6-2), [`RFC4213-3.6-3`](#rfc4213-3.6-3), [`RFC4213-3.6-4`](#rfc4213-3.6-4), [`RFC4213-3.6-5`](#rfc4213-3.6-5), [`RFC4213-3.6-6`](#rfc4213-3.6-6), [`RFC4213-3.6-7`](#rfc4213-3.6-7), [`RFC4213-3.7-1`](#rfc4213-3.7-1), [`RFC4213-3.8-1`](#rfc4213-3.8-1), [`RFC4213-3.8-2`](#rfc4213-3.8-2), [`RFC4213-5-1`](#rfc4213-5-1), [`RFC4213-5-2`](#rfc4213-5-2), [`RFC4213-5-3`](#rfc4213-5-3), [`RFC4213-5-4`](#rfc4213-5-4)

## Figures

- **MUST bar**: Ze verified 1, OS testing gap 17, Not built 5
- **Overview bar**: Ze MUST verified 1, OS testing gap 17, Not assessed 24, Not built 5

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 23 |
| MUST rows kept (the MUST bar) | 23 |
| Ze verified | 1 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 17 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 5 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 24 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 24 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc4213.json](../../../data/rfc/rfc4213.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4213-2.2-1` | DNS resolver libraries on IPv6/IPv4 nodes MUST be capable of handling both AAAA and A records (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a gap in proof. ze has a stub resolver of its own, internal/component/resolve/dns/resolver.go, whose (*Resolver).ResolveA and (*Resolver).ResolveAAAA query A and AAAA records for internal/component/resolve/cmd/resolve.go and internal/component/resolve/cli/cmd_dns.go, beside the Go standard library resolver (net.Resolver) that internal/core/probe/icmp.go::ResolveTarget calls with LookupNetIP. The missing proof is a test that ze's resolver handles both AAAA and A records |
| `RFC4213-2.2-2` | If there is not an application choice, or if the application has requested both, the resolver library MUST NOT filter out any records. (§2.2) | MUST NOT | 2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a gap in proof. ze has a stub resolver of its own, internal/component/resolve/dns/resolver.go, whose (*Resolver).ResolveA and (*Resolver).ResolveAAAA query A and AAAA records for internal/component/resolve/cmd/resolve.go and internal/component/resolve/cli/cmd_dns.go, beside the Go standard library resolver (net.Resolver) that internal/core/probe/icmp.go::ResolveTarget calls with LookupNetIP, passing the network ip for FamilyAny. The missing proof is a test that, on a request for both families, ze's resolver filters out none of the A and AAAA records it received |
| `RFC4213-3.2-1` | the encapsulator MUST NOT treat the tunnel as an interface with an MTU of 64 kilobytes, but instead either use the fixed static MTU or OPTIONAL dynamic MTU determination based on the IPv4 path MTU to the tunnel endpoint. (§3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux sit tunnel (net/ipv6/sit.c, ipip6_tunnel_xmit), linux:CONFIG_IPV6_SIT; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun creates the tunnel, and the kernel sizes it from the underlay (ipip6_tunnel_bind_dev sets the device MTU to the underlay MTU less the 20-byte IPv4 header, at least 1280) or, with DF set, from the IPv4 path MTU to the endpoint less 20, answering ICMPv6 Packet Too Big above it, so it never treats the tunnel as a 64 KB link. The PMtuDisc value buildSittun writes (1 unless no-pmtu-discovery) and a non-zero Ttl choose the dynamic path, and the operator's mtu leaf, written by internal/component/iface/config_apply.go::applyConfigPass through LinkSetMTU, sets the static value: these are the values ze installs that this behaviour reads, so a test over them is owed |
| `RFC4213-3.2-2` | Naively, the encapsulator could view encapsulation as IPv6 using IPv4 as a link layer with a very large MTU (65535-20 bytes at most; 20 bytes "extra" are needed for the encapsulating IPv4 header). The encapsulator would only need to report ICMPv6 "packet too big" errors back to the source for packets that exceed this MTU. However, such a scheme would be inefficient or non-interoperable for three reasons and therefore MUST NOT be used (§3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux sit tunnel (net/ipv6/sit.c, ipip6_tunnel_xmit), linux:CONFIG_IPV6_SIT; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun creates the tunnel, and the kernel uses the device MTU derived from the underlay or, with DF set, the IPv4 path MTU to the endpoint, never a 65535-20 byte link MTU. The PMtuDisc value and Ttl buildSittun writes choose between the two, and the operator's mtu leaf (internal/component/iface/config_apply.go::applyConfigPass, LinkSetMTU) sets the static value: these are the values ze installs that this behaviour reads, so a test over them is owed |
| `RFC4213-3.2.1-1` | By default, the MTU MUST be between 1280 and 1480 bytes (inclusive) (§3.2.1) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** with no mtu leaf ze makes no MTU call, and internal/plugins/iface/netlink/tunnel_linux.go::buildSittun leaves the default to the kernel, whose ipip6_tunnel_bind_dev (net/ipv6/sit.c) sets it to the underlay MTU less 20 bytes with a floor of 1280: 1480 on a 1500-byte underlay, but above 1480 on a jumbo underlay, so nothing holds the default inside 1280 to 1480 |
| `RFC4213-3.2.1-2` | If the default is not 1280 bytes, the implementation MUST have a configuration knob that can be used to change the MTU value. (§3.2.1) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the missing proof is a test that the mtu leaf changes a sit tunnel's MTU; ze has the knob, since the tunnel list uses the interface-common mtu leaf (internal/component/iface/yang/ze-iface-conf.yang, 68 to 16000) and internal/component/iface/config_apply.go::applyConfigPass writes it with LinkSetMTU, but no test tagged to this row exercises it |
| `RFC4213-3.2.1-3` | This memo also includes requirements (see Section 3.6) for the amount of IPv4 reassembly and IPv6 MRU that MUST be supported by all the decapsulators. (§3.2.1) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv4 reassembly and the sit receive path (net/ipv4/ip_fragment.c, net/ipv6/sit.c ipip6_rcv), linux:CONFIG_IPV6_SIT; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun creates the tunnel, and the kernel reassembles protocol 41 datagrams before ipip6_rcv decapsulates them and applies no device MTU to received packets; buildSittun installs no value the reassembly or the receive size reads, so no value ze writes decides this |
| `RFC4213-3.2.1-4` | When using the static tunnel MTU, the Don't Fragment bit MUST NOT be set in the encapsulating IPv4 header. (§3.2.1) | MUST NOT | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux sit tunnel (net/ipv6/sit.c ipip6_tunnel_xmit, net/ipv4/ip_tunnel_core.c ip_tunnel_netlink_parms), linux:CONFIG_IPV6_SIT; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun writes PMtuDisc 0 under no-pmtu-discovery and 1 otherwise, and the kernel sets DF in the outer header exactly when PMtuDisc is 1 or a non-zero Ttl is set, so the static-MTU case (PMtuDisc 0, no ttl) sends DF clear. PMtuDisc and Ttl are the values ze installs that decide this bit, so a test over them is owed |
| `RFC4213-3.2-3` | the encapsulator MUST NOT treat the tunnel as an interface with an MTU of 64 kilobytes (§3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux sit tunnel (net/ipv6/sit.c, ipip6_tunnel_xmit), linux:CONFIG_IPV6_SIT; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun creates the tunnel, and the kernel sizes it from the underlay MTU less 20 (at least 1280) or, with DF set, from the IPv4 path MTU less 20, so it never treats the tunnel as a 64 KB link. The PMtuDisc value and Ttl buildSittun writes choose the dynamic path, and the operator's mtu leaf (internal/component/iface/config_apply.go::applyConfigPass, LinkSetMTU) sets the static value: these are the values ze installs that this behaviour reads, so a test over them is owed |
| `RFC4213-3.6-1` | The decapsulator MUST verify that the tunnel source address is correct before further processing packets, to mitigate the problems with address spoofing (see Section 4). This check also applies to packets that are delivered to transport protocols on the decapsulator. This is done by verifying that the source address is the IPv4 address of the encapsulator, as configured on the decapsulator. (§3.6) | MUST | 3.6 | **positive:** `unit/verify` [`TestRFC4213SitDecapsulatesFromTheConfiguredRemote`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/netlink/tunnel_rfc_integration_linux_test.go#L331). **negative:** `unit/verify` [`TestRFC4213SitRefusesAnUnverifiedSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/netlink/tunnel_rfc_integration_linux_test.go#L364) |
| `RFC4213-3.6-2` | Packets for which the IPv4 source address does not match MUST be discarded (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux sit receive path (net/ipv6/sit.c, ipip6_rcv and ipip6_tunnel_lookup), linux:CONFIG_IPV6_SIT; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun writes the mandatory remote endpoint into the netlink Remote field, and ipip6_rcv delivers a packet only through a tunnel whose remote matches its outer IPv4 source, freeing the rest. Remote is the value ze installs that decides the discard, so a test over it is owed The discard holds only while the kernel's sit0 fallback device stays down: ipip6_tunnel_lookup (net/ipv6/sit.c) hands a packet no configured tunnel claims to the wildcard tunnel tunnels_wc[0], sit0, when it is up. Ze never brings sit0 up on its own: internal/component/iface/config_apply.go::applyConfigPass brings up only the interfaces the config names and dhcp-auto only the primary ethernet, though the operator command request interface <name> up (internal/component/iface/cmd/manage.go::handleInterfaceUp) accepts sit0 and nothing refuses it. |
| `RFC4213-3.6-3` | The decapsulator MUST be capable of having, on the tunnel interfaces, an IPv6 MRU of at least the maximum of 1500 bytes and the largest (IPv6) interface MTU on the decapsulator. (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux sit receive path and IPv6 input (net/ipv6/sit.c ipip6_rcv, net/ipv6/ip6_input.c), linux:CONFIG_IPV6_SIT; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun creates the tunnel, and the kernel's receive path applies no device MTU to a decapsulated packet, accepting any IPv6 packet up to 65535 bytes; buildSittun installs no value the receive size reads, so no value ze writes decides this |
| `RFC4213-3.6-4` | The decapsulator MUST be capable of reassembling an IPv4 packet that is (after the reassembly) the maximum of 1500 bytes and the largest (IPv4) interface MTU on the decapsulator. (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv4 reassembly (net/ipv4/ip_fragment.c), linux:CONFIG_INET; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun creates the tunnel, and the kernel reassembles IPv4 datagrams up to 65535 bytes before protocol 41 delivery; buildSittun installs no value the reassembly reads, so no value ze writes decides this |
| `RFC4213-3.6-5` | An implementation MAY have a configuration knob that can be used to set a larger value of the tunnel reassembly buffers than the above number, but it MUST NOT be set below the above number. (§3.6) | MUST NOT | 3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze has no tunnel reassembly-buffer knob of its own, but its generic sysctl component (internal/component/sysctl/sysctl.go::validateKey accepts any key) writes whatever net.ipv4.ipfrag_high_thresh value an operator configures, and neither ze nor Linux refuses a value below what reassembling the 1500-byte or largest-MTU datagram of Section 3.6 needs |
| `RFC4213-3.6-6` | When reconstructing the IPv6 packet, the length MUST be determined from the IPv6 payload length since the IPv4 packet might be padded (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 input (net/ipv6/ip6_input.c, ip6_rcv_core), linux:CONFIG_IPV6; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun creates the tunnel, and the decapsulated packet enters ip6_rcv_core, which trims it to the IPv6 payload length (pskb_trim_rcsum) whatever the IPv4 padding; buildSittun installs no value this reads, so no value ze writes decides this |
| `RFC4213-3.6-7` | After the decapsulation, the node MUST silently discard a packet with an invalid IPv6 source address. (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 input (net/ipv6/ip6_input.c, ip6_rcv_core), linux:CONFIG_IPV6; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun creates the tunnel, and ip6_rcv_core silently drops a decapsulated packet whose source is multicast or the loopback address; buildSittun installs no value this reads. It does not drop IPv4-mapped or IPv4-compatible sources, which the SHOULD list of row RFC4213-3.6-9 adds |
| `RFC4213-3.7-1` | The configured tunnels are IPv6 interfaces (over the IPv4 "link layer") and thus MUST have link-local addresses. (§3.7) | MUST | 3.7 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 address configuration (net/ipv6/addrconf.c, addrconf_sit_config and add_v4_addrs), linux:CONFIG_IPV6_SIT; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun creates the point-to-point sit link with the Local endpoint, and on link up the kernel assigns it the link-local address fe80::<local IPv4>/64, or with no Local one fe80:: address per IPv4 address of the node; ze sets no addr_gen_mode on a tunnel, the one setting that suppresses it. Local is the value ze installs that decides the interface identifier, so a test over the assigned address is owed |
| `RFC4213-3.8-1` | Configured tunnel implementations MUST at least accept and respond to the probe packets used by Neighbor Unreachability Detection (NUD) [RFC2461]. (§3.8) | MUST | 3.8 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux Neighbor Discovery (net/ipv6/ndisc.c, ndisc_recv_ns), linux:CONFIG_IPV6; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun creates the sit link, a NOARP device, and the kernel answers a Neighbor Solicitation for an address on it with a Neighbor Advertisement; buildSittun installs no value the ND code reads, so no value ze writes decides this |
| `RFC4213-3.8-2` | - the receiver MUST, while otherwise processing the Neighbor Discovery packet, silently ignore the content of any Source Link Layer Address options or Target Link Layer Address options received on the tunnel link. (§3.8) | MUST | 3.8 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux Neighbor Discovery and neighbour table (net/ipv6/ndisc.c, net/core/neighbour.c neigh_update), linux:CONFIG_IPV6; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun creates the sit link, a NOARP device whose neighbours are NUD_NOARP, and neigh_update refuses a link-layer address change on a NUD_NOARP entry, so the content of a received Source or Target Link-Layer Address option is ignored; buildSittun installs no value this reads |
| `RFC4213-5-1` | IPv4 source address of the packet MUST be the same as configured for the tunnel end-point (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux sit receive path (net/ipv6/sit.c, ipip6_rcv and ipip6_tunnel_lookup), linux:CONFIG_IPV6_SIT; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun writes the mandatory remote endpoint into the netlink Remote field, and ipip6_rcv accepts a packet only through a tunnel whose remote equals its outer IPv4 source. Remote is the value ze installs that decides the check, so a test over it is owed The discard holds only while the kernel's sit0 fallback device stays down: ipip6_tunnel_lookup (net/ipv6/sit.c) hands a packet no configured tunnel claims to the wildcard tunnel tunnels_wc[0], sit0, when it is up. Ze never brings sit0 up on its own: internal/component/iface/config_apply.go::applyConfigPass brings up only the interfaces the config names and dhcp-auto only the primary ethernet, though the operator command request interface <name> up (internal/component/iface/cmd/manage.go::handleInterfaceUp) accepts sit0 and nothing refuses it. |
| `RFC4213-5-2` | - IPv6 packets with several, obviously invalid IPv6 source addresses received from the tunnel MUST be discarded (see Section 3.6 for details); and (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv6 input (net/ipv6/ip6_input.c, ip6_rcv_core), linux:CONFIG_IPV6; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun creates the tunnel, and ip6_rcv_core drops a decapsulated packet whose IPv6 source is multicast or the loopback address; buildSittun installs no value this reads. IPv4-mapped and IPv4-compatible sources pass (row RFC4213-3.6-9) |
| `RFC4213-5-3` | an implementation MUST treat interfaces to different links as separate (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux network devices and IPv6 scoping (net/ipv6/sit.c, net/ipv6/addrconf.c), linux:CONFIG_IPV6_SIT; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun creates one sit netdev per configured tunnel, and the kernel keeps each device's link-local addresses, neighbours and routes scoped to that device; no value ze writes merges two links |
| `RFC4213-5-4` | When dropping packets due to failing to match the allowed IPv4 source addresses for a tunnel the node should not "acknowledge" the existence of a tunnel, otherwise this could be used to probe the acceptable tunnel endpoint addresses. For that reason, the specification says that such packets MUST be discarded (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux sit receive path (net/ipv6/sit.c ipip6_rcv, net/ipv4/tunnel4.c), linux:CONFIG_IPV6_SIT; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun writes the mandatory remote endpoint into the netlink Remote field, and a packet whose outer source matches no tunnel's remote is freed, not delivered. Remote is the value ze installs that decides the discard, so a test over it is owed. tunnel4 then sends ICMP Port Unreachable for it, which the SHOULD NOT of row RFC4213-5-5 counts against The discard holds only while the kernel's sit0 fallback device stays down: ipip6_tunnel_lookup (net/ipv6/sit.c) hands a packet no configured tunnel claims to the wildcard tunnel tunnels_wc[0], sit0, when it is up. Ze never brings sit0 up on its own: internal/component/iface/config_apply.go::applyConfigPass brings up only the interfaces the config names and dhcp-auto only the primary ethernet, though the operator command request interface <name> up (internal/component/iface/cmd/manage.go::handleInterfaceUp) accepts sit0 and nothing refuses it. |
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
| [`RFC4213-2.2-1`](#rfc4213-2.2-1) DNS resolver libraries on IPv6/IPv4 nodes MUST be capable of handling both AAAA and A records (§2.2) | {gap}, no test | a gap in proof. ze has a stub resolver of its own, internal/component/resolve/dns/resolver.go, whose (*Resolver).ResolveA and (*Resolver).ResolveAAAA query A and AAAA records for internal/component/resolve/cmd/resolve.go and internal/component/resolve/cli/cmd_dns.go, beside the Go standard library resolver (net.Resolver) that internal/core/probe/icmp.go::ResolveTarget calls with LookupNetIP. The missing proof is a test that ze's resolver handles both AAAA and A records |
| [`RFC4213-2.2-2`](#rfc4213-2.2-2) If there is not an application choice, or if the application has requested both, the resolver library MUST NOT filter out any records. (§2.2) | {gap}, no test | a gap in proof. ze has a stub resolver of its own, internal/component/resolve/dns/resolver.go, whose (*Resolver).ResolveA and (*Resolver).ResolveAAAA query A and AAAA records for internal/component/resolve/cmd/resolve.go and internal/component/resolve/cli/cmd_dns.go, beside the Go standard library resolver (net.Resolver) that internal/core/probe/icmp.go::ResolveTarget calls with LookupNetIP, passing the network ip for FamilyAny. The missing proof is a test that, on a request for both families, ze's resolver filters out none of the A and AAAA records it received |
| [`RFC4213-3.2-1`](#rfc4213-3.2-1) the encapsulator MUST NOT treat the tunnel as an interface with an MTU of 64 kilobytes, but instead either use the fixed static MTU or OPTIONAL dynamic MTU determination based on the IPv4 path MTU to the tunnel endpoint. (§3.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux sit tunnel (net/ipv6/sit.c, ipip6_tunnel_xmit), linux:CONFIG_IPV6_SIT; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun creates the tunnel, and the kernel sizes it from the underlay (ipip6_tunnel_bind_dev sets the device MTU to the underlay MTU less the 20-byte IPv4 header, at least 1280) or, with DF set, from the IPv4 path MTU to the endpoint less 20, answering ICMPv6 Packet Too Big above it, so it never treats the tunnel as a 64 KB link. The PMtuDisc value buildSittun writes (1 unless no-pmtu-discovery) and a non-zero Ttl choose the dynamic path, and the operator's mtu leaf, written by internal/component/iface/config_apply.go::applyConfigPass through LinkSetMTU, sets the static value: these are the values ze installs that this behaviour reads, so a test over them is owed |
| [`RFC4213-3.2-2`](#rfc4213-3.2-2) Naively, the encapsulator could view encapsulation as IPv6 using IPv4 as a link layer with a very large MTU (65535-20 bytes at most; 20 bytes "extra" are needed for the encapsulating IPv4 header). The encapsulator would only need to report ICMPv6 "packet too big" errors back to the source for packets that exceed this MTU. However, such a scheme would be inefficient or non-interoperable for three reasons and therefore MUST NOT be used (§3.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux sit tunnel (net/ipv6/sit.c, ipip6_tunnel_xmit), linux:CONFIG_IPV6_SIT; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun creates the tunnel, and the kernel uses the device MTU derived from the underlay or, with DF set, the IPv4 path MTU to the endpoint, never a 65535-20 byte link MTU. The PMtuDisc value and Ttl buildSittun writes choose between the two, and the operator's mtu leaf (internal/component/iface/config_apply.go::applyConfigPass, LinkSetMTU) sets the static value: these are the values ze installs that this behaviour reads, so a test over them is owed |
| [`RFC4213-3.2.1-1`](#rfc4213-3.2.1-1) By default, the MTU MUST be between 1280 and 1480 bytes (inclusive) (§3.2.1) | {gap}, no test | with no mtu leaf ze makes no MTU call, and internal/plugins/iface/netlink/tunnel_linux.go::buildSittun leaves the default to the kernel, whose ipip6_tunnel_bind_dev (net/ipv6/sit.c) sets it to the underlay MTU less 20 bytes with a floor of 1280: 1480 on a 1500-byte underlay, but above 1480 on a jumbo underlay, so nothing holds the default inside 1280 to 1480 |
| [`RFC4213-3.2.1-2`](#rfc4213-3.2.1-2) If the default is not 1280 bytes, the implementation MUST have a configuration knob that can be used to change the MTU value. (§3.2.1) | {gap}, no test | the missing proof is a test that the mtu leaf changes a sit tunnel's MTU; ze has the knob, since the tunnel list uses the interface-common mtu leaf (internal/component/iface/yang/ze-iface-conf.yang, 68 to 16000) and internal/component/iface/config_apply.go::applyConfigPass writes it with LinkSetMTU, but no test tagged to this row exercises it |
| [`RFC4213-3.2.1-3`](#rfc4213-3.2.1-3) This memo also includes requirements (see Section 3.6) for the amount of IPv4 reassembly and IPv6 MRU that MUST be supported by all the decapsulators. (§3.2.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv4 reassembly and the sit receive path (net/ipv4/ip_fragment.c, net/ipv6/sit.c ipip6_rcv), linux:CONFIG_IPV6_SIT; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun creates the tunnel, and the kernel reassembles protocol 41 datagrams before ipip6_rcv decapsulates them and applies no device MTU to received packets; buildSittun installs no value the reassembly or the receive size reads, so no value ze writes decides this |
| [`RFC4213-3.2.1-4`](#rfc4213-3.2.1-4) When using the static tunnel MTU, the Don't Fragment bit MUST NOT be set in the encapsulating IPv4 header. (§3.2.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux sit tunnel (net/ipv6/sit.c ipip6_tunnel_xmit, net/ipv4/ip_tunnel_core.c ip_tunnel_netlink_parms), linux:CONFIG_IPV6_SIT; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun writes PMtuDisc 0 under no-pmtu-discovery and 1 otherwise, and the kernel sets DF in the outer header exactly when PMtuDisc is 1 or a non-zero Ttl is set, so the static-MTU case (PMtuDisc 0, no ttl) sends DF clear. PMtuDisc and Ttl are the values ze installs that decide this bit, so a test over them is owed |
| [`RFC4213-3.2-3`](#rfc4213-3.2-3) the encapsulator MUST NOT treat the tunnel as an interface with an MTU of 64 kilobytes (§3.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux sit tunnel (net/ipv6/sit.c, ipip6_tunnel_xmit), linux:CONFIG_IPV6_SIT; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun creates the tunnel, and the kernel sizes it from the underlay MTU less 20 (at least 1280) or, with DF set, from the IPv4 path MTU less 20, so it never treats the tunnel as a 64 KB link. The PMtuDisc value and Ttl buildSittun writes choose the dynamic path, and the operator's mtu leaf (internal/component/iface/config_apply.go::applyConfigPass, LinkSetMTU) sets the static value: these are the values ze installs that this behaviour reads, so a test over them is owed |
| [`RFC4213-3.6-2`](#rfc4213-3.6-2) Packets for which the IPv4 source address does not match MUST be discarded (§3.6) | no test | no test carries this requirement id; annotated {lower-layer}: Linux sit receive path (net/ipv6/sit.c, ipip6_rcv and ipip6_tunnel_lookup), linux:CONFIG_IPV6_SIT; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun writes the mandatory remote endpoint into the netlink Remote field, and ipip6_rcv delivers a packet only through a tunnel whose remote matches its outer IPv4 source, freeing the rest. Remote is the value ze installs that decides the discard, so a test over it is owed The discard holds only while the kernel's sit0 fallback device stays down: ipip6_tunnel_lookup (net/ipv6/sit.c) hands a packet no configured tunnel claims to the wildcard tunnel tunnels_wc[0], sit0, when it is up. Ze never brings sit0 up on its own: internal/component/iface/config_apply.go::applyConfigPass brings up only the interfaces the config names and dhcp-auto only the primary ethernet, though the operator command request interface <name> up (internal/component/iface/cmd/manage.go::handleInterfaceUp) accepts sit0 and nothing refuses it. |
| [`RFC4213-3.6-3`](#rfc4213-3.6-3) The decapsulator MUST be capable of having, on the tunnel interfaces, an IPv6 MRU of at least the maximum of 1500 bytes and the largest (IPv6) interface MTU on the decapsulator. (§3.6) | no test | no test carries this requirement id; annotated {lower-layer}: Linux sit receive path and IPv6 input (net/ipv6/sit.c ipip6_rcv, net/ipv6/ip6_input.c), linux:CONFIG_IPV6_SIT; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun creates the tunnel, and the kernel's receive path applies no device MTU to a decapsulated packet, accepting any IPv6 packet up to 65535 bytes; buildSittun installs no value the receive size reads, so no value ze writes decides this |
| [`RFC4213-3.6-4`](#rfc4213-3.6-4) The decapsulator MUST be capable of reassembling an IPv4 packet that is (after the reassembly) the maximum of 1500 bytes and the largest (IPv4) interface MTU on the decapsulator. (§3.6) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv4 reassembly (net/ipv4/ip_fragment.c), linux:CONFIG_INET; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun creates the tunnel, and the kernel reassembles IPv4 datagrams up to 65535 bytes before protocol 41 delivery; buildSittun installs no value the reassembly reads, so no value ze writes decides this |
| [`RFC4213-3.6-5`](#rfc4213-3.6-5) An implementation MAY have a configuration knob that can be used to set a larger value of the tunnel reassembly buffers than the above number, but it MUST NOT be set below the above number. (§3.6) | {gap}, no test | ze has no tunnel reassembly-buffer knob of its own, but its generic sysctl component (internal/component/sysctl/sysctl.go::validateKey accepts any key) writes whatever net.ipv4.ipfrag_high_thresh value an operator configures, and neither ze nor Linux refuses a value below what reassembling the 1500-byte or largest-MTU datagram of Section 3.6 needs |
| [`RFC4213-3.6-6`](#rfc4213-3.6-6) When reconstructing the IPv6 packet, the length MUST be determined from the IPv6 payload length since the IPv4 packet might be padded (§3.6) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 input (net/ipv6/ip6_input.c, ip6_rcv_core), linux:CONFIG_IPV6; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun creates the tunnel, and the decapsulated packet enters ip6_rcv_core, which trims it to the IPv6 payload length (pskb_trim_rcsum) whatever the IPv4 padding; buildSittun installs no value this reads, so no value ze writes decides this |
| [`RFC4213-3.6-7`](#rfc4213-3.6-7) After the decapsulation, the node MUST silently discard a packet with an invalid IPv6 source address. (§3.6) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 input (net/ipv6/ip6_input.c, ip6_rcv_core), linux:CONFIG_IPV6; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun creates the tunnel, and ip6_rcv_core silently drops a decapsulated packet whose source is multicast or the loopback address; buildSittun installs no value this reads. It does not drop IPv4-mapped or IPv4-compatible sources, which the SHOULD list of row RFC4213-3.6-9 adds |
| [`RFC4213-3.7-1`](#rfc4213-3.7-1) The configured tunnels are IPv6 interfaces (over the IPv4 "link layer") and thus MUST have link-local addresses. (§3.7) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 address configuration (net/ipv6/addrconf.c, addrconf_sit_config and add_v4_addrs), linux:CONFIG_IPV6_SIT; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun creates the point-to-point sit link with the Local endpoint, and on link up the kernel assigns it the link-local address fe80::<local IPv4>/64, or with no Local one fe80:: address per IPv4 address of the node; ze sets no addr_gen_mode on a tunnel, the one setting that suppresses it. Local is the value ze installs that decides the interface identifier, so a test over the assigned address is owed |
| [`RFC4213-3.8-1`](#rfc4213-3.8-1) Configured tunnel implementations MUST at least accept and respond to the probe packets used by Neighbor Unreachability Detection (NUD) [RFC2461]. (§3.8) | no test | no test carries this requirement id; annotated {lower-layer}: Linux Neighbor Discovery (net/ipv6/ndisc.c, ndisc_recv_ns), linux:CONFIG_IPV6; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun creates the sit link, a NOARP device, and the kernel answers a Neighbor Solicitation for an address on it with a Neighbor Advertisement; buildSittun installs no value the ND code reads, so no value ze writes decides this |
| [`RFC4213-3.8-2`](#rfc4213-3.8-2) - the receiver MUST, while otherwise processing the Neighbor Discovery packet, silently ignore the content of any Source Link Layer Address options or Target Link Layer Address options received on the tunnel link. (§3.8) | no test | no test carries this requirement id; annotated {lower-layer}: Linux Neighbor Discovery and neighbour table (net/ipv6/ndisc.c, net/core/neighbour.c neigh_update), linux:CONFIG_IPV6; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun creates the sit link, a NOARP device whose neighbours are NUD_NOARP, and neigh_update refuses a link-layer address change on a NUD_NOARP entry, so the content of a received Source or Target Link-Layer Address option is ignored; buildSittun installs no value this reads |
| [`RFC4213-5-1`](#rfc4213-5-1) IPv4 source address of the packet MUST be the same as configured for the tunnel end-point (§5) | no test | no test carries this requirement id; annotated {lower-layer}: Linux sit receive path (net/ipv6/sit.c, ipip6_rcv and ipip6_tunnel_lookup), linux:CONFIG_IPV6_SIT; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun writes the mandatory remote endpoint into the netlink Remote field, and ipip6_rcv accepts a packet only through a tunnel whose remote equals its outer IPv4 source. Remote is the value ze installs that decides the check, so a test over it is owed The discard holds only while the kernel's sit0 fallback device stays down: ipip6_tunnel_lookup (net/ipv6/sit.c) hands a packet no configured tunnel claims to the wildcard tunnel tunnels_wc[0], sit0, when it is up. Ze never brings sit0 up on its own: internal/component/iface/config_apply.go::applyConfigPass brings up only the interfaces the config names and dhcp-auto only the primary ethernet, though the operator command request interface <name> up (internal/component/iface/cmd/manage.go::handleInterfaceUp) accepts sit0 and nothing refuses it. |
| [`RFC4213-5-2`](#rfc4213-5-2) - IPv6 packets with several, obviously invalid IPv6 source addresses received from the tunnel MUST be discarded (see Section 3.6 for details); and (§5) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv6 input (net/ipv6/ip6_input.c, ip6_rcv_core), linux:CONFIG_IPV6; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun creates the tunnel, and ip6_rcv_core drops a decapsulated packet whose IPv6 source is multicast or the loopback address; buildSittun installs no value this reads. IPv4-mapped and IPv4-compatible sources pass (row RFC4213-3.6-9) |
| [`RFC4213-5-3`](#rfc4213-5-3) an implementation MUST treat interfaces to different links as separate (§5) | no test | no test carries this requirement id; annotated {lower-layer}: Linux network devices and IPv6 scoping (net/ipv6/sit.c, net/ipv6/addrconf.c), linux:CONFIG_IPV6_SIT; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun creates one sit netdev per configured tunnel, and the kernel keeps each device's link-local addresses, neighbours and routes scoped to that device; no value ze writes merges two links |
| [`RFC4213-5-4`](#rfc4213-5-4) When dropping packets due to failing to match the allowed IPv4 source addresses for a tunnel the node should not "acknowledge" the existence of a tunnel, otherwise this could be used to probe the acceptable tunnel endpoint addresses. For that reason, the specification says that such packets MUST be discarded (§5) | no test | no test carries this requirement id; annotated {lower-layer}: Linux sit receive path (net/ipv6/sit.c ipip6_rcv, net/ipv4/tunnel4.c), linux:CONFIG_IPV6_SIT; internal/plugins/iface/netlink/tunnel_linux.go::buildSittun writes the mandatory remote endpoint into the netlink Remote field, and a packet whose outer source matches no tunnel's remote is freed, not delivered. Remote is the value ze installs that decides the discard, so a test over it is owed. tunnel4 then sends ICMP Port Unreachable for it, which the SHOULD NOT of row RFC4213-5-5 counts against The discard holds only while the kernel's sit0 fallback device stays down: ipip6_tunnel_lookup (net/ipv6/sit.c) hands a packet no configured tunnel claims to the wildcard tunnel tunnels_wc[0], sit0, when it is up. Ze never brings sit0 up on its own: internal/component/iface/config_apply.go::applyConfigPass brings up only the interfaces the config names and dhcp-auto only the primary ethernet, though the operator command request interface <name> up (internal/component/iface/cmd/manage.go::handleInterfaceUp) accepts sit0 and nothing refuses it. |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC4213-2.2-1`](#rfc4213-2.2-1)

DNS resolver libraries on IPv6/IPv4 nodes MUST be capable of handling both AAAA and A records (§2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4213-2.2-1, so there is no linked test.

### [`RFC4213-2.2-2`](#rfc4213-2.2-2)

If there is not an application choice, or if the application has requested both, the resolver library MUST NOT filter out any records. (§2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4213-2.2-2, so there is no linked test.

### [`RFC4213-3.2-1`](#rfc4213-3.2-1)

the encapsulator MUST NOT treat the tunnel as an interface with an MTU of 64 kilobytes, but instead either use the fixed static MTU or OPTIONAL dynamic MTU determination based on the IPv4 path MTU to the tunnel endpoint. (§3.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4213-3.2-1, so there is no linked test.

### [`RFC4213-3.2-2`](#rfc4213-3.2-2)

Naively, the encapsulator could view encapsulation as IPv6 using IPv4 as a link layer with a very large MTU (65535-20 bytes at most; 20 bytes "extra" are needed for the encapsulating IPv4 header). The encapsulator would only need to report ICMPv6 "packet too big" errors back to the source for packets that exceed this MTU. However, such a scheme would be inefficient or non-interoperable for three reasons and therefore MUST NOT be used (§3.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4213-3.2-2, so there is no linked test.

### [`RFC4213-3.2.1-1`](#rfc4213-3.2.1-1)

By default, the MTU MUST be between 1280 and 1480 bytes (inclusive) (§3.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4213-3.2.1-1, so there is no linked test.

### [`RFC4213-3.2.1-2`](#rfc4213-3.2.1-2)

If the default is not 1280 bytes, the implementation MUST have a configuration knob that can be used to change the MTU value. (§3.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4213-3.2.1-2, so there is no linked test.

### [`RFC4213-3.2.1-3`](#rfc4213-3.2.1-3)

This memo also includes requirements (see Section 3.6) for the amount of IPv4 reassembly and IPv6 MRU that MUST be supported by all the decapsulators. (§3.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4213-3.2.1-3, so there is no linked test.

### [`RFC4213-3.2.1-4`](#rfc4213-3.2.1-4)

When using the static tunnel MTU, the Don't Fragment bit MUST NOT be set in the encapsulating IPv4 header. (§3.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4213-3.2.1-4, so there is no linked test.

### [`RFC4213-3.2-3`](#rfc4213-3.2-3)

the encapsulator MUST NOT treat the tunnel as an interface with an MTU of 64 kilobytes (§3.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4213-3.2-3, so there is no linked test.

### [`RFC4213-3.6-1`](#rfc4213-3.6-1)

The decapsulator MUST verify that the tunnel source address is correct before further processing packets, to mitigate the problems with address spoofing (see Section 4). This check also applies to packets that are delivered to transport protocols on the decapsulator. This is done by verifying that the source address is the IPv4 address of the encapsulator, as configured on the decapsulator. (§3.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC4213SitRefusesAnUnverifiedSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/netlink/tunnel_rfc_integration_linux_test.go#L364) | unit/verify | revert, verified |
| positive | [`TestRFC4213SitDecapsulatesFromTheConfiguredRemote`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/netlink/tunnel_rfc_integration_linux_test.go#L331) | unit/verify | revert, verified |

### [`RFC4213-3.6-2`](#rfc4213-3.6-2)

Packets for which the IPv4 source address does not match MUST be discarded (§3.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4213-3.6-2, so there is no linked test.

### [`RFC4213-3.6-3`](#rfc4213-3.6-3)

The decapsulator MUST be capable of having, on the tunnel interfaces, an IPv6 MRU of at least the maximum of 1500 bytes and the largest (IPv6) interface MTU on the decapsulator. (§3.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4213-3.6-3, so there is no linked test.

### [`RFC4213-3.6-4`](#rfc4213-3.6-4)

The decapsulator MUST be capable of reassembling an IPv4 packet that is (after the reassembly) the maximum of 1500 bytes and the largest (IPv4) interface MTU on the decapsulator. (§3.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4213-3.6-4, so there is no linked test.

### [`RFC4213-3.6-5`](#rfc4213-3.6-5)

An implementation MAY have a configuration knob that can be used to set a larger value of the tunnel reassembly buffers than the above number, but it MUST NOT be set below the above number. (§3.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4213-3.6-5, so there is no linked test.

### [`RFC4213-3.6-6`](#rfc4213-3.6-6)

When reconstructing the IPv6 packet, the length MUST be determined from the IPv6 payload length since the IPv4 packet might be padded (§3.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4213-3.6-6, so there is no linked test.

### [`RFC4213-3.6-7`](#rfc4213-3.6-7)

After the decapsulation, the node MUST silently discard a packet with an invalid IPv6 source address. (§3.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4213-3.6-7, so there is no linked test.

### [`RFC4213-3.7-1`](#rfc4213-3.7-1)

The configured tunnels are IPv6 interfaces (over the IPv4 "link layer") and thus MUST have link-local addresses. (§3.7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4213-3.7-1, so there is no linked test.

### [`RFC4213-3.8-1`](#rfc4213-3.8-1)

Configured tunnel implementations MUST at least accept and respond to the probe packets used by Neighbor Unreachability Detection (NUD) [RFC2461]. (§3.8)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4213-3.8-1, so there is no linked test.

### [`RFC4213-3.8-2`](#rfc4213-3.8-2)

- the receiver MUST, while otherwise processing the Neighbor Discovery packet, silently ignore the content of any Source Link Layer Address options or Target Link Layer Address options received on the tunnel link. (§3.8)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4213-3.8-2, so there is no linked test.

### [`RFC4213-5-1`](#rfc4213-5-1)

IPv4 source address of the packet MUST be the same as configured for the tunnel end-point (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4213-5-1, so there is no linked test.

### [`RFC4213-5-2`](#rfc4213-5-2)

- IPv6 packets with several, obviously invalid IPv6 source addresses received from the tunnel MUST be discarded (see Section 3.6 for details); and (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4213-5-2, so there is no linked test.

### [`RFC4213-5-3`](#rfc4213-5-3)

an implementation MUST treat interfaces to different links as separate (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4213-5-3, so there is no linked test.

### [`RFC4213-5-4`](#rfc4213-5-4)

When dropping packets due to failing to match the allowed IPv4 source addresses for a tunnel the node should not "acknowledge" the existence of a tunnel, otherwise this could be used to probe the acceptable tunnel endpoint addresses. For that reason, the specification says that such packets MUST be discarded (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4213-5-4, so there is no linked test.

## Requirement list review

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
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `2.1` | not stated | 0 | reviewed | not stated |
| `2.2` | not stated | 2 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `3.1` | not stated | 0 | reviewed | not stated |
| `3.2` | not stated | 2 | reviewed | not stated |
| `3.2.1` | not stated | 4 | reviewed | not stated |
| `3.2.2` | not stated | 0 | reviewed | not stated |
| `3.3` | not stated | 0 | reviewed | not stated |
| `3.4` | not stated | 0 | reviewed | not stated |
| `3.5` | not stated | 0 | reviewed | not stated |
| `3.6` | not stated | 7 | reviewed | not stated |
| `3.7` | not stated | 1 | reviewed | not stated |
| `3.8` | not stated | 2 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 4 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `7.1` | not stated | 0 | reviewed | not stated |
| `7.2` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 2 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `8:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 8 is the changelog from RFC 2893. The sentence restates the source-address check obligation already mapped at site 3.6:1 (decapsulator MUST verify the tunnel source address) and at 5:2 (invalid IPv6 source addresses MUST be discarded). | - Added stronger wording for source address checks: both IPv4 and IPv6 source addresses MUST be checked, and RPF-like ingress filtering is optional. |
| `8:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 8 changelog entry restating the section 2.2 resolver obligation already mapped at site 2.2:2 (the resolver library MUST NOT filter out any records). | - Removed/clarified DNS record filtering; an API is a SHOULD and if it does not exist, MUST NOT filter anything. |

## Superseded

No document obsoletes RFC 4213, so its obligations are stated where they were written.
