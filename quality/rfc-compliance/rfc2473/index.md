# RFC 2473 - Generic Packet Tunneling in IPv6 Specification

No row in the public ledger. Every requirement this repository extracted from RFC 2473, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 9.1% | 1 of 11 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 11 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 11 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 11 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 11 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 100.0% | 2 of 2 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 11 | of 21 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (third-party), so every share below reads what the summary records rather than what the gate enforces |
| Out of scope | 10 | of 11 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 90.9% | 10 of 11 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 11 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 11 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 11 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 21 |
| Gated MUST-level | 11 |
| Not applicable, so out of scope | 10 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 2 |
| Tagged units | 2 |
| Recorded audit verdicts | 0 |
| Discrimination records | 2 |
| Summary | `rfc/short/rfc2473.md` |
| Requirement shard | `rfc/requirements/rfc2473.md` |
| RFC text | `rfc/full/rfc2473.txt` |

## Enrolment

Not enrolled (third-party, a layer under or beside Ze performs the document and Ze holds no Go code for it, so the reason beside this kind names the component that does): The Linux ip6_tunnel module builds the outer IPv6 header and handles the Encapsulation Limit option. Ze only builds the netlink link descriptor, internal/plugins/iface/netlink/tunnel_linux.go::buildIp6tnl.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 2473.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the gated population |
| Annotated (including scoped evidence) | 10 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **11** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (1):** [`RFC2473-4.1.1-2`](#rfc2473-4.1.1-2)

**Annotated (including scoped evidence) (10):** [`RFC2473-4.1.1-1`](#rfc2473-4.1.1-1), [`RFC2473-4.1.1-3`](#rfc2473-4.1.1-3), [`RFC2473-4.1.1-4`](#rfc2473-4.1.1-4), [`RFC2473-4.1.2-1`](#rfc2473-4.1.2-1), [`RFC2473-7.1-1`](#rfc2473-7.1-1), [`RFC2473-7.1-2`](#rfc2473-7.1-2), [`RFC2473-7.1-3`](#rfc2473-7.1-3), [`RFC2473-7.1-4`](#rfc2473-7.1-4), [`RFC2473-7.2-1`](#rfc2473-7.2-1), [`RFC2473-8-1`](#rfc2473-8-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2473-4.1.1-1` | If a Tunnel Encapsulation Limit option is found in the packet entering the tunnel and its limit value is zero, the packet is discarded and an ICMP Parameter Problem message [ICMP-Spec] is sent to the source of the packet, which is the previous tunnel entry-point node. The Code field of the Parameter Problem message is set to zero ("erroneous header field encountered") and the Pointer field is set to point to the third octet of the Tunnel Encapsulation Limit option (i.e., the octet containing the limit value of zero). (§4.1.1) | MUST | 4.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the tunnel-node datapath that processes the encapsulation-limit option and emits ICMPv6 Parameter Problem is the kernel ip6_tunnel module; ze only creates and configures the tunnel netdev (internal/plugins/iface/netlink/tunnel_linux.go:44) |
| `RFC2473-4.1.1-2` | (c) If a Tunnel Encapsulation Limit option is found in the packet entering the tunnel and its limit value is non-zero, an additional Tunnel Encapsulation Limit option must be included as part of the encapsulating headers being added at this entry point. The limit value in the encapsulating option is set to one less than the limit value found in the packet being encapsulated. (§4.1.1) | MUST | 4.1.1 | **positive:** `unit/verify` [`TestRFC2473EncapLimitDecrementedIntoTheOuterHeader`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/netlink/tunnel_rfc_integration_linux_test.go#L188). **negative:** `unit/verify` [`TestRFC2473EncapLimitFromThePacketNotTheConfiguredLimit`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/netlink/tunnel_rfc_integration_linux_test.go#L220) |
| `RFC2473-4.1.1-3` | If a Tunnel Encapsulation Limit option is not found in the packet entering the tunnel and if an encapsulation limit has been configured for this tunnel, a Tunnel Encapsulation Limit option must be included as part of the encapsulating headers being added at this entry point. The limit value in the option is set to the configured limit. (§4.1.1) | MUST | 4.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** inserting the configured encapsulation-limit option is done by the kernel ip6_tunnel datapath; ze passes IFLA_IPTUN_ENCAP_LIMIT at internal/plugins/iface/netlink/tunnel_linux.go:266-267 |
| `RFC2473-4.1.1-4` | Examine the packet to see if a Tunnel Encapsulation Limit option is present following its IPv6 header. The headers following the IPv6 header must be examined in strict "left-to-right" order (§4.1.1) | MUST | 4.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** left-to-right extension-header parsing at packet time is kernel datapath parsing; ze carries no per-packet header path, only tunnel netdev creation (internal/plugins/iface/netlink/tunnel_linux.go:44) |
| `RFC2473-4.1.2-1` | A particular case of encapsulation which must be avoided is the loopback encapsulation. Loopback encapsulation takes place when a tunnel IPv6 entry-point node encapsulates tunnel IPv6 packets originated from itself, and destined to itself. (§4.1.2) | MUST | 4.1.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** detecting loopback encapsulation at packet time binds the kernel ip6_tunnel datapath; ze only creates the tunnel netdev (internal/plugins/iface/netlink/tunnel_linux.go:44) |
| `RFC2473-7.1-1` | Therefore, like any source of an IPv6 packet, a tunnel entry-point node must support fragmentation of tunnel IPv6 packets. (§7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** fragmenting outbound tunnel packets is a kernel/VPP datapath capability; ze holds no packet-forwarding code, only tunnel config (internal/plugins/iface/netlink/tunnel_linux.go:248) |
| `RFC2473-7.1-2` | A tunnel intermediate node that forwards a tunnel packet to another node in the tunnel follows the general IPv6 rule that it must not fragment a packet undergoing forwarding. (§7) | MUST NOT | 7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is a routing control plane with no IPv6 forwarding datapath; the intermediate-node no-fragment rule is enforced by the kernel/VPP |
| `RFC2473-7.1-3` | if the original IPv6 packet size is larger than the IPv6 minimum link MTU [IPv6-Spec], the entry-point node discards the packet and sends an ICMPv6 "Packet Too Big" message to the source address of the original packet with the recommended MTU size field set to the tunnel MTU or the IPv6 minimum link MTU, whichever is larger, i.e. max (tunnel MTU, IPv6 minimum link MTU). (§7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** tunnel-MTU comparison, discard, and ICMPv6 Packet Too Big generation are kernel ip6_tunnel datapath actions ze does not perform |
| `RFC2473-7.1-4` | if the original IPv6 packet is equal or smaller than the IPv6 minimum link MTU, the tunnel entry-point node encapsulates the original packet, and subsequently fragments the resulting IPv6 tunnel packet into IPv6 fragments that do not exceed the Path MTU to the tunnel exit-point. (§7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** encapsulate-then-fragment is a kernel/VPP datapath operation; ze only configures the tunnel interface (internal/plugins/iface/netlink/tunnel_linux.go:248) |
| `RFC2473-7.2-1` | if in the original IPv4 packet header the Don't Fragment - DF - bit flag is SET, the entry-point node discards the packet and returns an ICMP message. The ICMP message has the type = "unreachable", the code = "packet too big", and the recommended MTU size field set to the size of the tunnel MTU - see sections 6.7 and 8.3. (§7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** DF handling and ICMP unreachable/too-big generation for encapsulated IPv4 are kernel datapath behavior; ze only sets PMtuDisc via netlink (internal/plugins/iface/netlink/tunnel_linux.go:210-214) |
| `RFC2473-8-1` | To report a problem detected inside the tunnel to the source of an original packet, the tunnel entry point node must relay the ICMP message received from inside the tunnel to the source of that original IPv6 packet. (§8) | MUST | 8 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** relaying tunnel-internal ICMP errors to the original source is a kernel ip6_tunnel datapath function ze does not perform |
| `RFC2473-3.1-1` | Tunnel extension headers should appear in the order recommended by the specifications that define the extension headers (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2473-6.3-1` | The "single-hop" mechanism should be implemented by having the tunnel entry point node set a tunnel IPv6 header hop limit independently of the hop limit of the original header. (§6.3) | SHOULD | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2473-4.1.2-2` | To avoid such a case, it is recommended that an implementation have a mechanism that checks and rejects the configuration of a tunnel in which both the entry-point and exit-point node addresses belong to the same node. (§4.1.2) | SHOULD | 4.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2473-4.1.2-3` | It is also recommended that the encapsulating engine check for and reject the encapsulation of a packet that has the pair of tunnel entry-point and exit-point addresses identical with the pair of original packet source and final destination addresses. (§4.1.2) | SHOULD | 4.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2473-4.1.3-1` | When the path of a packet from source to final destination includes tunnels, the maximum number of hops that the packet can traverse should be controlled by two mechanisms used together to avoid the negative effects of recursive encapsulation in routing loops: (a) the original packet hop limit. It is decremented at each forwarding operation performed on an original packet. This includes each encapsulation of the original packet. It does not include nested encapsulations of the original packet (b) the tunnel IPv6 packet encapsulation limit. (§4.1.3) | SHOULD | 4.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2473-6.3-2` | It is recommended that the tunnel hop limit be configured with a value that ensures: (a) that tunnel IPv6 packets can reach the tunnel exit-point node (b) a quick expiration of the tunnel packet if a routing loop occurs within the IPv6 tunnel. (§6.3) | SHOULD | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2473-6.1-1` | The tunnel entry-point node address is one of the valid IPv6 unicast addresses of the entry-point node - the validation of the address at tunnel configuration time is recommended. (§6.1) | SHOULD | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2473-6.4-1` | The configured Packet Traffic Class can also indicate whether the value of the Traffic Class field in the tunnel header is copied from the original header, or it is set to the pre-configured value. (§6.4) | MAY | 6.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2473-4.1.1-5` | A tunnel entry-point node may be configured to include a Tunnel Encapsulation Limit option as part of the information prepended to all packets entering a tunnel at that node. (§4.1.1) | MAY | 4.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2473-5.1-1` | Depending on IPv6 node configuration parameters, a tunnel entry-point node may append to the tunnel IPv6 main header one or more IPv6 extension headers, such as a Hop-by-Hop Options header, a Routing header, or others. (§5.1) | MAY | 5.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2473-4.1.1-1`](#rfc2473-4.1.1-1) If a Tunnel Encapsulation Limit option is found in the packet entering the tunnel and its limit value is zero, the packet is discarded and an ICMP Parameter Problem message [ICMP-Spec] is sent to the source of the packet, which is the previous tunnel entry-point node. The Code field of the Parameter Problem message is set to zero ("erroneous header field encountered") and the Pointer field is set to point to the third octet of the Tunnel Encapsulation Limit option (i.e., the octet containing the limit value of zero). (§4.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: the tunnel-node datapath that processes the encapsulation-limit option and emits ICMPv6 Parameter Problem is the kernel ip6_tunnel module; ze only creates and configures the tunnel netdev (internal/plugins/iface/netlink/tunnel_linux.go:44) |
| [`RFC2473-4.1.1-3`](#rfc2473-4.1.1-3) If a Tunnel Encapsulation Limit option is not found in the packet entering the tunnel and if an encapsulation limit has been configured for this tunnel, a Tunnel Encapsulation Limit option must be included as part of the encapsulating headers being added at this entry point. The limit value in the option is set to the configured limit. (§4.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: inserting the configured encapsulation-limit option is done by the kernel ip6_tunnel datapath; ze passes IFLA_IPTUN_ENCAP_LIMIT at internal/plugins/iface/netlink/tunnel_linux.go:266-267 |
| [`RFC2473-4.1.1-4`](#rfc2473-4.1.1-4) Examine the packet to see if a Tunnel Encapsulation Limit option is present following its IPv6 header. The headers following the IPv6 header must be examined in strict "left-to-right" order (§4.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: left-to-right extension-header parsing at packet time is kernel datapath parsing; ze carries no per-packet header path, only tunnel netdev creation (internal/plugins/iface/netlink/tunnel_linux.go:44) |
| [`RFC2473-4.1.2-1`](#rfc2473-4.1.2-1) A particular case of encapsulation which must be avoided is the loopback encapsulation. Loopback encapsulation takes place when a tunnel IPv6 entry-point node encapsulates tunnel IPv6 packets originated from itself, and destined to itself. (§4.1.2) | no test | no test carries this requirement id; annotated {not-applicable}: detecting loopback encapsulation at packet time binds the kernel ip6_tunnel datapath; ze only creates the tunnel netdev (internal/plugins/iface/netlink/tunnel_linux.go:44) |
| [`RFC2473-7.1-1`](#rfc2473-7.1-1) Therefore, like any source of an IPv6 packet, a tunnel entry-point node must support fragmentation of tunnel IPv6 packets. (§7) | no test | no test carries this requirement id; annotated {not-applicable}: fragmenting outbound tunnel packets is a kernel/VPP datapath capability; ze holds no packet-forwarding code, only tunnel config (internal/plugins/iface/netlink/tunnel_linux.go:248) |
| [`RFC2473-7.1-2`](#rfc2473-7.1-2) A tunnel intermediate node that forwards a tunnel packet to another node in the tunnel follows the general IPv6 rule that it must not fragment a packet undergoing forwarding. (§7) | no test | no test carries this requirement id; annotated {not-applicable}: ze is a routing control plane with no IPv6 forwarding datapath; the intermediate-node no-fragment rule is enforced by the kernel/VPP |
| [`RFC2473-7.1-3`](#rfc2473-7.1-3) if the original IPv6 packet size is larger than the IPv6 minimum link MTU [IPv6-Spec], the entry-point node discards the packet and sends an ICMPv6 "Packet Too Big" message to the source address of the original packet with the recommended MTU size field set to the tunnel MTU or the IPv6 minimum link MTU, whichever is larger, i.e. max (tunnel MTU, IPv6 minimum link MTU). (§7.1) | no test | no test carries this requirement id; annotated {not-applicable}: tunnel-MTU comparison, discard, and ICMPv6 Packet Too Big generation are kernel ip6_tunnel datapath actions ze does not perform |
| [`RFC2473-7.1-4`](#rfc2473-7.1-4) if the original IPv6 packet is equal or smaller than the IPv6 minimum link MTU, the tunnel entry-point node encapsulates the original packet, and subsequently fragments the resulting IPv6 tunnel packet into IPv6 fragments that do not exceed the Path MTU to the tunnel exit-point. (§7.1) | no test | no test carries this requirement id; annotated {not-applicable}: encapsulate-then-fragment is a kernel/VPP datapath operation; ze only configures the tunnel interface (internal/plugins/iface/netlink/tunnel_linux.go:248) |
| [`RFC2473-7.2-1`](#rfc2473-7.2-1) if in the original IPv4 packet header the Don't Fragment - DF - bit flag is SET, the entry-point node discards the packet and returns an ICMP message. The ICMP message has the type = "unreachable", the code = "packet too big", and the recommended MTU size field set to the size of the tunnel MTU - see sections 6.7 and 8.3. (§7.2) | no test | no test carries this requirement id; annotated {not-applicable}: DF handling and ICMP unreachable/too-big generation for encapsulated IPv4 are kernel datapath behavior; ze only sets PMtuDisc via netlink (internal/plugins/iface/netlink/tunnel_linux.go:210-214) |
| [`RFC2473-8-1`](#rfc2473-8-1) To report a problem detected inside the tunnel to the source of an original packet, the tunnel entry point node must relay the ICMP message received from inside the tunnel to the source of that original IPv6 packet. (§8) | no test | no test carries this requirement id; annotated {not-applicable}: relaying tunnel-internal ICMP errors to the original source is a kernel ip6_tunnel datapath function ze does not perform |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2473-4.1.1-1`](#rfc2473-4.1.1-1)

If a Tunnel Encapsulation Limit option is found in the packet entering the tunnel and its limit value is zero, the packet is discarded and an ICMP Parameter Problem message [ICMP-Spec] is sent to the source of the packet, which is the previous tunnel entry-point node. The Code field of the Parameter Problem message is set to zero ("erroneous header field encountered") and the Pointer field is set to point to the third octet of the Tunnel Encapsulation Limit option (i.e., the octet containing the limit value of zero). (§4.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2473-4.1.1-1, so no unit is bound to it.

### [`RFC2473-4.1.1-2`](#rfc2473-4.1.1-2)

(c) If a Tunnel Encapsulation Limit option is found in the packet entering the tunnel and its limit value is non-zero, an additional Tunnel Encapsulation Limit option must be included as part of the encapsulating headers being added at this entry point. The limit value in the encapsulating option is set to one less than the limit value found in the packet being encapsulated. (§4.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2473EncapLimitFromThePacketNotTheConfiguredLimit`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/netlink/tunnel_rfc_integration_linux_test.go#L220) | unit/verify | revert, verified |
| positive | [`TestRFC2473EncapLimitDecrementedIntoTheOuterHeader`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/netlink/tunnel_rfc_integration_linux_test.go#L188) | unit/verify | revert, verified |

### [`RFC2473-4.1.1-3`](#rfc2473-4.1.1-3)

If a Tunnel Encapsulation Limit option is not found in the packet entering the tunnel and if an encapsulation limit has been configured for this tunnel, a Tunnel Encapsulation Limit option must be included as part of the encapsulating headers being added at this entry point. The limit value in the option is set to the configured limit. (§4.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2473-4.1.1-3, so no unit is bound to it.

### [`RFC2473-4.1.1-4`](#rfc2473-4.1.1-4)

Examine the packet to see if a Tunnel Encapsulation Limit option is present following its IPv6 header. The headers following the IPv6 header must be examined in strict "left-to-right" order (§4.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2473-4.1.1-4, so no unit is bound to it.

### [`RFC2473-4.1.2-1`](#rfc2473-4.1.2-1)

A particular case of encapsulation which must be avoided is the loopback encapsulation. Loopback encapsulation takes place when a tunnel IPv6 entry-point node encapsulates tunnel IPv6 packets originated from itself, and destined to itself. (§4.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2473-4.1.2-1, so no unit is bound to it.

### [`RFC2473-7.1-1`](#rfc2473-7.1-1)

Therefore, like any source of an IPv6 packet, a tunnel entry-point node must support fragmentation of tunnel IPv6 packets. (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2473-7.1-1, so no unit is bound to it.

### [`RFC2473-7.1-2`](#rfc2473-7.1-2)

A tunnel intermediate node that forwards a tunnel packet to another node in the tunnel follows the general IPv6 rule that it must not fragment a packet undergoing forwarding. (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2473-7.1-2, so no unit is bound to it.

### [`RFC2473-7.1-3`](#rfc2473-7.1-3)

if the original IPv6 packet size is larger than the IPv6 minimum link MTU [IPv6-Spec], the entry-point node discards the packet and sends an ICMPv6 "Packet Too Big" message to the source address of the original packet with the recommended MTU size field set to the tunnel MTU or the IPv6 minimum link MTU, whichever is larger, i.e. max (tunnel MTU, IPv6 minimum link MTU). (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2473-7.1-3, so no unit is bound to it.

### [`RFC2473-7.1-4`](#rfc2473-7.1-4)

if the original IPv6 packet is equal or smaller than the IPv6 minimum link MTU, the tunnel entry-point node encapsulates the original packet, and subsequently fragments the resulting IPv6 tunnel packet into IPv6 fragments that do not exceed the Path MTU to the tunnel exit-point. (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2473-7.1-4, so no unit is bound to it.

### [`RFC2473-7.2-1`](#rfc2473-7.2-1)

if in the original IPv4 packet header the Don't Fragment - DF - bit flag is SET, the entry-point node discards the packet and returns an ICMP message. The ICMP message has the type = "unreachable", the code = "packet too big", and the recommended MTU size field set to the size of the tunnel MTU - see sections 6.7 and 8.3. (§7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2473-7.2-1, so no unit is bound to it.

### [`RFC2473-8-1`](#rfc2473-8-1)

To report a problem detected inside the tunnel to the source of an original packet, the tunnel entry point node must relay the ICMP message received from inside the tunnel to the source of that original IPv6 packet. (§8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2473-8-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-27 |
| Register | prose |
| Source | rfc/full/rfc2473.txt |
| Source fingerprint | 59956643eb9ece1a |
| Record | rfc/extraction/rfc2473.json |
| Mapped sentences | 7 |
| Declined as scope | 5 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 1 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `3.3` | not stated | 0 | walked | not stated |
| `3.4` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 0 | walked | not stated |
| `4.1.1` | not stated | 4 | walked | not stated |
| `4.1.2` | not stated | 1 | walked | not stated |
| `4.1.3` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |
| `6.3` | not stated | 0 | walked | not stated |
| `6.4` | not stated | 0 | walked | not stated |
| `6.5` | not stated | 0 | walked | not stated |
| `6.6` | not stated | 0 | walked | not stated |
| `6.7` | not stated | 0 | walked | not stated |
| `7` | not stated | 3 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `7.2` | not stated | 0 | walked | not stated |
| `8` | not stated | 1 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |
| `8.2` | not stated | 1 | walked | not stated |
| `8.3` | not stated | 0 | walked | not stated |
| `8.4` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `A.1` | Appendix subsection A.1 | 0 | walked | Appendix subsection A.1. Until 2026-09-27 the heading reader did not read a column-0 appendix subsection heading written without a trailing dot, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `A.1.1` | Appendix subsection A.1.1 | 0 | walked | Appendix subsection A.1.1. Until 2026-09-27 the heading reader did not read a column-0 appendix subsection heading written without a trailing dot, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `A.1.2` | Appendix subsection A.1.2 | 1 | walked | Appendix subsection A.1.2. Until 2026-09-27 the heading reader did not read a column-0 appendix subsection heading written without a trailing dot, so its text was read as part of section 11, where its 1 site(s) were walked; every decision is carried forward by its verbatim quote. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | the RFC 2119 boilerplate sentence that defines the keywords; it states no obligation of this protocol | The keywords MUST, MUST NOT, MAY, OPTIONAL, REQUIRED, RECOMMENDED, SHALL, SHALL NOT, SHOULD, SHOULD NOT are to be interpreted as defined in RFC 2119. |
| `4.1.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | the lead-in that introduces the enumerated procedure (a) to (e); the obligations are the steps themselves, carried by sites 4.1.1:2 to 4.1.1:4 and by the section's unsourced id RFC2473-4.1.1-1 for step (b) | A tunnel entry-point node is required to execute the following procedure for every packet entering a tunnel at that node: |
| `7:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | descriptive prose explaining why anycast destinations are unsuitable: "a requirement that is not necessarily satisfied by packets sent to an anycast address". It describes the reassembly property of IPv6 fragmentation, and imposes nothing on a tunnel node | The problem, which is similar to that of original fragmented IPv6 packets destined to nodes identified by an anycast address, is that all the fragments of a packet must arrive at the same destination node for that node to be able to perform a successful reassembly, a requirement that is not necessarily satisfied by packets sent to an anycast address. |
| `8.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | the keyword sits inside the noun phrase "the minimum link MTU size required for IPv6 [IPv6-Spec]", which names a constant; the sentence itself restates the rule of section 7.1 in indicative prose | According to the general rules described in 7.1, an ICMP "packet too big" message is sent to the source of the original packet only if the original packet size is larger than the minimum link MTU size required for IPv6 [IPv6-Spec]. |
| `A.1.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | the Internet Society copyright notice; its "must" binds anyone republishing the document, not an implementation | However, this document itself may not be modified in any way, such as by removing the copyright notice or references to the Internet Society or other Internet organizations, except as needed for the purpose of developing Internet standards in which case the procedures for copyrights defined in the Internet Standards process must be followed, or as required to translate it into languages other than English. |

## Superseded

No document obsoletes RFC 2473, so its obligations are stated where they were written.
