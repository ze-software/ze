# RFC 4787 - Network Address Translation (NAT) Behavioral Requirements for Unicast UDP

Partial. Every requirement this repository extracted from RFC 4787, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

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
| Gated MUSTs | 13 | of 29 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 13 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 13 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 53.8% | 7 of 13 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 13 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |
| Linux cannot do this | 0.0% | 0 of 13 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and remains unmet; it stays in every denominator and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 46.2% | 6 of 13 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

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
| Requirements | 29 |
| Gated MUST-level | 13 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 6 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc4787.md` |
| Requirement shard | `rfc/requirements/rfc4787.md` |
| RFC text | `rfc/full/rfc4787.txt` |

## Enrolment

Enrolled: NAT behavioral requirements for unicast UDP (BCP 127). Every MUST-level row is classified: the behaviors Linux netfilter performs on the NAT rules ze installs are {lower-layer}, and the behaviors neither ze nor the Linux defaults provide (endpoint-independent mapping, the two-minute UDP timer, hairpinning, deterministic translation, address-conflict handling) are {gap}.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Source NAT, destination NAT and masquerade are configured through the `firewall` YANG tree and lowered into nftables NAT chains
- Linux nf_nat never overloads a port, refreshes a mapping on outbound traffic, keeps a mapping across ICMP, reassembles out-of-order fragments through nf_defrag, and fragments or answers DF=1 on the forwarding path.


**What the ledger says remains**

Endpoint-independent mapping is not guaranteed: nf_nat keeps the source port only while the translated tuple is unique and remaps per destination otherwise, `masquerade { random; }` randomizes per flow, and VPP NAT44-ED is endpoint-dependent ([`RFC4787-4.1-1`](#rfc4787-4.1-1), [`RFC4787-8-1`](#rfc4787-8-1)). Ze sets no conntrack timeout, so a UDP flow seen in one direction expires after the Linux default of 30 seconds ([`RFC4787-4.3-1`](#rfc4787-4.3-1)). No hairpinning is configured ([`RFC4787-6-1`](#rfc4787-6-1), [`RFC4787-6-2`](#rfc4787-6-2)). A dynamically configured external address is not checked against the internal network ([`RFC4787-4.4-1`](#rfc4787-4.4-1)).

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

**Annotated (including scoped evidence) (13):** [`RFC4787-4.1-1`](#rfc4787-4.1-1), [`RFC4787-4.2.1-1`](#rfc4787-4.2.1-1), [`RFC4787-4.3-1`](#rfc4787-4.3-1), [`RFC4787-4.3-2`](#rfc4787-4.3-2), [`RFC4787-4.4-1`](#rfc4787-4.4-1), [`RFC4787-6-1`](#rfc4787-6-1), [`RFC4787-6-2`](#rfc4787-6-2), [`RFC4787-8-1`](#rfc4787-8-1), [`RFC4787-9-1`](#rfc4787-9-1), [`RFC4787-10-1`](#rfc4787-10-1), [`RFC4787-10-2`](#rfc4787-10-2), [`RFC4787-11-1`](#rfc4787-11-1), [`RFC4787-11-2`](#rfc4787-11-2)

## Attainable ceiling

Mandatory ceiling: 6 of 13 (46.2%); excluded: Met by a lower layer: 7 of 13 (53.8%). Optional ceiling: 16 of 16 (100.0%). Lowest mandatory ceiling: rfc4787 6 of 13 (46.2%). Lowest optional ceiling: rfc4787 16 of 16 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4787-4.1-1` | REQ-1: A NAT MUST have an "Endpoint-Independent Mapping" behavior. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nf_nat preserves the internal source port only while the translated 5-tuple is unique and otherwise allocates another port for that destination, the masquerade random and fully-random flags ze lowers randomize per flow, and VPP NAT44-ED maps per endpoint; internal/plugins/firewall/nft/lower_linux.go lowerMasquerade |
| `RFC4787-4.2.1-1` | REQ-3: A NAT MUST NOT have a "Port assignment" behavior of "Port overloading". (§4.2.1) | MUST NOT | 4.2.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux nf_nat; internal/plugins/firewall/nft/lower_linux.go::lowerMasquerade installs the masquerade expression and nf_nat's unique-tuple allocation never gives two internal endpoints one external tuple |
| `RFC4787-4.3-1` | REQ-5: A NAT UDP mapping timer MUST NOT expire in less than two minutes, unless REQ-5a applies. (§4.3) | MUST NOT | 4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze installs no conntrack timeout policy, so a UDP flow seen in one direction expires after Linux's nf_conntrack_udp_timeout default of 30 seconds; internal/plugins/firewall/nft/lower_linux.go |
| `RFC4787-4.3-2` | REQ-6: The NAT mapping Refresh Direction MUST have a "NAT Outbound refresh behavior" of "True". (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux nf_conntrack; internal/plugins/firewall/nft/lower_linux.go::lowerMasquerade installs the NAT rule and nf_conntrack refreshes the entry's timeout on every packet of the flow, outbound included |
| `RFC4787-4.4-1` | REQ-7: A NAT device whose external IP interface can be configured dynamically MUST either (1) automatically ensure that its internal network uses IP addresses that do not conflict with its external network, or (2) be able to translate and forward traffic between all internal nodes and all external nodes whose IP addresses numerically conflict with the internal network. (§4.4) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze neither checks a DHCP- or PPP-learned external address against the internal subnets nor translates traffic to a conflicting external range |
| `RFC4787-6-1` | REQ-9: A NAT MUST support "Hairpinning". (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze generates no hairpin rule, and a packet from an internal host to another internal host's external mapping matches no conntrack reply tuple and is delivered to the NAT itself |
| `RFC4787-6-2` | a) A NAT Hairpinning behavior MUST be "External source IP address and port". (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no hairpinning exists (RFC4787-6-1), so there is no hairpin source to translate |
| `RFC4787-8-1` | REQ-11: A NAT MUST have deterministic behavior, i.e., it MUST NOT change the NAT translation (Section 4) or the Filtering (Section 5) Behavior at any point in time, or under any particular conditions. (§8) | MUST | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nf_nat preserves the port when it is free and switches to another port under contention, which changes the mapping behavior under load |
| `RFC4787-9-1` | Receipt of any sort of ICMP message MUST NOT destroy the NAT mapping. (§9) | MUST NOT | 9 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux nf_conntrack; internal/plugins/firewall/nft/lower_linux.go::lowerMasquerade installs the NAT rule and nf_conntrack classifies an ICMP error as RELATED to the entry without deleting it |
| `RFC4787-10-1` | REQ-13: If the packet received on an internal IP address has DF=1, the NAT MUST send back an ICMP message "Fragmentation needed and DF set" to the host, as described in [RFC0792]. (§10) | MUST | 10 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv4 forwarding; internal/component/iface/dispatch.go::SetMTU sets the egress MTU and the kernel forwarding path answers an oversize DF=1 packet with ICMP Fragmentation Needed |
| `RFC4787-10-2` | If the packet has DF=0, the NAT MUST fragment the packet and SHOULD send the fragments in order. (§10) | MUST | 10 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv4 forwarding; internal/component/iface/dispatch.go::SetMTU sets the egress MTU and the kernel forwarding path fragments an oversize DF=0 packet in order |
| `RFC4787-11-1` | REQ-14: A NAT MUST support receiving in-order and out-of-order fragments, so it MUST have "Received Fragment Out of Order" behavior. (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux nf_defrag_ipv4; internal/plugins/firewall/nft/lower_linux.go::lowerMasquerade installs a NAT rule, which loads conntrack and its defragmentation hook, and the reassembly queue accepts fragments in any order |
| `RFC4787-11-2` | a) A NAT's out-of-order fragment processing mechanism MUST be designed so that fragmentation-based DoS attacks do not compromise the NAT's ability to process in-order and unfragmented IP packets. (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux IPv4 reassembly; internal/plugins/firewall/nft/lower_linux.go::lowerMasquerade installs the NAT rule, and the kernel bounds reassembly memory with ipfrag_high_thresh and expires queues after ipfrag_time while unfragmented packets bypass reassembly |
| `RFC4787-4.1-2` | REQ-2: It is RECOMMENDED that a NAT have an "IP address pooling" behavior of "Paired". (§4.1) | RECOMMENDED | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4787-4.2.1-2` | If the host's source port was in the range 0-1023, it is RECOMMENDED the NAT's source port be in the same range. (§4.2.1) | RECOMMENDED | 4.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4787-4.2.1-3` | If the host's source port was in the range 1024-65535, it is RECOMMENDED that the NAT's source port be in that range. (§4.2.1) | RECOMMENDED | 4.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4787-4.2.2-1` | REQ-4: It is RECOMMENDED that a NAT have a "Port parity preservation" behavior of "Yes". (§4.2.2) | RECOMMENDED | 4.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4787-4.3-5` | c) A default value of five minutes or more for the NAT UDP mapping timer is RECOMMENDED. (§4.3) | RECOMMENDED | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4787-5-1` | REQ-8: If application transparency is most important, it is RECOMMENDED that a NAT have an "Endpoint-Independent Filtering" behavior. (§5) | RECOMMENDED | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4787-5-2` | If a more stringent filtering behavior is most important, it is RECOMMENDED that a NAT have an "Address-Dependent Filtering" behavior. (§5) | RECOMMENDED | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4787-7-1` | To eliminate interference with UNSAF NAT traversal mechanisms and allow integrity protection of UDP communications, NAT ALGs for UDP-based protocols SHOULD be turned off. (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4787-7-2` | a) If a NAT includes ALGs, it is RECOMMENDED that the NAT allow the NAT administrator to enable or disable each ALG separately. (§7) | RECOMMENDED | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4787-9-2` | The NAT's default configuration SHOULD NOT filter ICMP messages based on their source IP address. (§9) | SHOULD NOT | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC4787-9-3` | Such ICMP messages SHOULD be rewritten by the NAT (specifically, the IP headers and the ICMP payload) and forwarded to the appropriate internal or external host. (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC4787-9-4` | b) It is RECOMMENDED that a NAT support ICMP Destination Unreachable messages. (§9) | RECOMMENDED | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC4787-4.3-3` | a) For specific destination ports in the well-known port range (ports 0-1023), a NAT MAY have shorter UDP mapping timers that are specific to the IANA-registered application running over that specific destination port. (§4.3) | MAY | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4787-4.3-4` | b) The value of the NAT UDP mapping timer MAY be configurable. (§4.3) | MAY | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4787-4.3-6` | a) The NAT mapping Refresh Direction MAY have a "NAT Inbound refresh behavior" of "True". (§4.3) | MAY | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4787-5-3` | a) The filtering behavior MAY be an option configurable by the administrator of the NAT. (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4787-4.1-1`](#rfc4787-4.1-1) REQ-1: A NAT MUST have an "Endpoint-Independent Mapping" behavior. (§4.1) | {gap}, no test | nf_nat preserves the internal source port only while the translated 5-tuple is unique and otherwise allocates another port for that destination, the masquerade random and fully-random flags ze lowers randomize per flow, and VPP NAT44-ED maps per endpoint; internal/plugins/firewall/nft/lower_linux.go lowerMasquerade |
| [`RFC4787-4.2.1-1`](#rfc4787-4.2.1-1) REQ-3: A NAT MUST NOT have a "Port assignment" behavior of "Port overloading". (§4.2.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux nf_nat; internal/plugins/firewall/nft/lower_linux.go::lowerMasquerade installs the masquerade expression and nf_nat's unique-tuple allocation never gives two internal endpoints one external tuple |
| [`RFC4787-4.3-1`](#rfc4787-4.3-1) REQ-5: A NAT UDP mapping timer MUST NOT expire in less than two minutes, unless REQ-5a applies. (§4.3) | {gap}, no test | ze installs no conntrack timeout policy, so a UDP flow seen in one direction expires after Linux's nf_conntrack_udp_timeout default of 30 seconds; internal/plugins/firewall/nft/lower_linux.go |
| [`RFC4787-4.3-2`](#rfc4787-4.3-2) REQ-6: The NAT mapping Refresh Direction MUST have a "NAT Outbound refresh behavior" of "True". (§4.3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux nf_conntrack; internal/plugins/firewall/nft/lower_linux.go::lowerMasquerade installs the NAT rule and nf_conntrack refreshes the entry's timeout on every packet of the flow, outbound included |
| [`RFC4787-4.4-1`](#rfc4787-4.4-1) REQ-7: A NAT device whose external IP interface can be configured dynamically MUST either (1) automatically ensure that its internal network uses IP addresses that do not conflict with its external network, or (2) be able to translate and forward traffic between all internal nodes and all external nodes whose IP addresses numerically conflict with the internal network. (§4.4) | {gap}, no test | ze neither checks a DHCP- or PPP-learned external address against the internal subnets nor translates traffic to a conflicting external range |
| [`RFC4787-6-1`](#rfc4787-6-1) REQ-9: A NAT MUST support "Hairpinning". (§6) | {gap}, no test | ze generates no hairpin rule, and a packet from an internal host to another internal host's external mapping matches no conntrack reply tuple and is delivered to the NAT itself |
| [`RFC4787-6-2`](#rfc4787-6-2) a) A NAT Hairpinning behavior MUST be "External source IP address and port". (§6) | {gap}, no test | no hairpinning exists (RFC4787-6-1), so there is no hairpin source to translate |
| [`RFC4787-8-1`](#rfc4787-8-1) REQ-11: A NAT MUST have deterministic behavior, i.e., it MUST NOT change the NAT translation (Section 4) or the Filtering (Section 5) Behavior at any point in time, or under any particular conditions. (§8) | {gap}, no test | nf_nat preserves the port when it is free and switches to another port under contention, which changes the mapping behavior under load |
| [`RFC4787-9-1`](#rfc4787-9-1) Receipt of any sort of ICMP message MUST NOT destroy the NAT mapping. (§9) | no test | no test carries this requirement id; annotated {lower-layer}: Linux nf_conntrack; internal/plugins/firewall/nft/lower_linux.go::lowerMasquerade installs the NAT rule and nf_conntrack classifies an ICMP error as RELATED to the entry without deleting it |
| [`RFC4787-10-1`](#rfc4787-10-1) REQ-13: If the packet received on an internal IP address has DF=1, the NAT MUST send back an ICMP message "Fragmentation needed and DF set" to the host, as described in [RFC0792]. (§10) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv4 forwarding; internal/component/iface/dispatch.go::SetMTU sets the egress MTU and the kernel forwarding path answers an oversize DF=1 packet with ICMP Fragmentation Needed |
| [`RFC4787-10-2`](#rfc4787-10-2) If the packet has DF=0, the NAT MUST fragment the packet and SHOULD send the fragments in order. (§10) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv4 forwarding; internal/component/iface/dispatch.go::SetMTU sets the egress MTU and the kernel forwarding path fragments an oversize DF=0 packet in order |
| [`RFC4787-11-1`](#rfc4787-11-1) REQ-14: A NAT MUST support receiving in-order and out-of-order fragments, so it MUST have "Received Fragment Out of Order" behavior. (§11) | no test | no test carries this requirement id; annotated {lower-layer}: Linux nf_defrag_ipv4; internal/plugins/firewall/nft/lower_linux.go::lowerMasquerade installs a NAT rule, which loads conntrack and its defragmentation hook, and the reassembly queue accepts fragments in any order |
| [`RFC4787-11-2`](#rfc4787-11-2) a) A NAT's out-of-order fragment processing mechanism MUST be designed so that fragmentation-based DoS attacks do not compromise the NAT's ability to process in-order and unfragmented IP packets. (§11) | no test | no test carries this requirement id; annotated {lower-layer}: Linux IPv4 reassembly; internal/plugins/firewall/nft/lower_linux.go::lowerMasquerade installs the NAT rule, and the kernel bounds reassembly memory with ipfrag_high_thresh and expires queues after ipfrag_time while unfragmented packets bypass reassembly |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4787-4.1-1`](#rfc4787-4.1-1)

REQ-1: A NAT MUST have an "Endpoint-Independent Mapping" behavior. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4787-4.1-1, so no unit is bound to it.

### [`RFC4787-4.2.1-1`](#rfc4787-4.2.1-1)

REQ-3: A NAT MUST NOT have a "Port assignment" behavior of "Port overloading". (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4787-4.2.1-1, so no unit is bound to it.

### [`RFC4787-4.3-1`](#rfc4787-4.3-1)

REQ-5: A NAT UDP mapping timer MUST NOT expire in less than two minutes, unless REQ-5a applies. (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4787-4.3-1, so no unit is bound to it.

### [`RFC4787-4.3-2`](#rfc4787-4.3-2)

REQ-6: The NAT mapping Refresh Direction MUST have a "NAT Outbound refresh behavior" of "True". (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4787-4.3-2, so no unit is bound to it.

### [`RFC4787-4.4-1`](#rfc4787-4.4-1)

REQ-7: A NAT device whose external IP interface can be configured dynamically MUST either (1) automatically ensure that its internal network uses IP addresses that do not conflict with its external network, or (2) be able to translate and forward traffic between all internal nodes and all external nodes whose IP addresses numerically conflict with the internal network. (§4.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4787-4.4-1, so no unit is bound to it.

### [`RFC4787-6-1`](#rfc4787-6-1)

REQ-9: A NAT MUST support "Hairpinning". (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4787-6-1, so no unit is bound to it.

### [`RFC4787-6-2`](#rfc4787-6-2)

a) A NAT Hairpinning behavior MUST be "External source IP address and port". (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4787-6-2, so no unit is bound to it.

### [`RFC4787-8-1`](#rfc4787-8-1)

REQ-11: A NAT MUST have deterministic behavior, i.e., it MUST NOT change the NAT translation (Section 4) or the Filtering (Section 5) Behavior at any point in time, or under any particular conditions. (§8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4787-8-1, so no unit is bound to it.

### [`RFC4787-9-1`](#rfc4787-9-1)

Receipt of any sort of ICMP message MUST NOT destroy the NAT mapping. (§9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4787-9-1, so no unit is bound to it.

### [`RFC4787-10-1`](#rfc4787-10-1)

REQ-13: If the packet received on an internal IP address has DF=1, the NAT MUST send back an ICMP message "Fragmentation needed and DF set" to the host, as described in [RFC0792]. (§10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4787-10-1, so no unit is bound to it.

### [`RFC4787-10-2`](#rfc4787-10-2)

If the packet has DF=0, the NAT MUST fragment the packet and SHOULD send the fragments in order. (§10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4787-10-2, so no unit is bound to it.

### [`RFC4787-11-1`](#rfc4787-11-1)

REQ-14: A NAT MUST support receiving in-order and out-of-order fragments, so it MUST have "Received Fragment Out of Order" behavior. (§11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4787-11-1, so no unit is bound to it.

### [`RFC4787-11-2`](#rfc4787-11-2)

a) A NAT's out-of-order fragment processing mechanism MUST be designed so that fragmentation-based DoS attacks do not compromise the NAT's ability to process in-order and unfragmented IP packets. (§11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4787-11-2, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc4787.txt |
| Source fingerprint | 5c0457a05afefb2c |
| Record | rfc/extraction/rfc4787.json |
| Mapped sentences | 13 |
| Declined as scope | 15 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 1 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `4.2.1` | not stated | 1 | walked | not stated |
| `4.2.2` | not stated | 0 | walked | not stated |
| `4.2.3` | not stated | 0 | walked | not stated |
| `4.3` | not stated | 2 | walked | not stated |
| `4.4` | not stated | 1 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 2 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 1 | walked | not stated |
| `9` | not stated | 2 | walked | not stated |
| `10` | not stated | 2 | walked | not stated |
| `11` | not stated | 2 | walked | not stated |
| `12` | not stated | 14 | walked | not stated |
| `13` | not stated | 0 | walked | not stated |
| `14` | not stated | 0 | walked | not stated |
| `15` | not stated | 0 | walked | not stated |
| `16` | not stated | 0 | walked | not stated |
| `16.1` | not stated | 0 | walked | not stated |
| `16.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `9:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | REQ-12 restates the sentence two lines above it in the same section, which RFC4787-9-1 carries. | REQ-12: Receipt of any sort of ICMP message MUST NOT terminate the NAT mapping. |
| `12:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 12 opens by defining what 'compliant with this specification' means; the MUST names the set of requirements and imposes none of its own. | A NAT that supports all the mandatory requirements of this specification (i.e., the "MUST"), is "compliant with this specification". |
| `12:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 12 summary repeats REQ-1 of Section 4.1. | REQ-1: A NAT MUST have an "Endpoint-Independent Mapping" behavior. |
| `12:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 12 summary repeats REQ-3 of Section 4.2.1. | REQ-3: A NAT MUST NOT have a "Port assignment" behavior of "Port overloading". |
| `12:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 12 summary repeats REQ-5 of Section 4.3. | REQ-5: A NAT UDP mapping timer MUST NOT expire in less than two minutes, unless REQ-5a applies. |
| `12:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 12 summary repeats REQ-6 of Section 4.3. | REQ-6: The NAT mapping Refresh Direction MUST have a "NAT Outbound refresh behavior" of "True". |
| `12:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 12 summary repeats REQ-7 of Section 4.4. | REQ-7 A NAT device whose external IP interface can be configured dynamically MUST either (1) Automatically ensure that its internal network uses IP addresses that do not conflict with its external network, or (2) Be able to translate and forward traffic between all internal nodes and all external nodes whose IP addresses numerically conflict with the internal network. |
| `12:7` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 12 summary repeats REQ-9 of Section 6. | REQ-9: A NAT MUST support "Hairpinning". |
| `12:8` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 12 summary repeats REQ-9a of Section 6. | a) A NAT Hairpinning behavior MUST be "External source IP address and port". |
| `12:9` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 12 summary repeats REQ-11 of Section 8. | REQ-11: A NAT MUST have deterministic behavior, i.e., it MUST NOT change the NAT translation (Section 4) or the Filtering (Section 5) Behavior at any point in time, or under any particular conditions. |
| `12:10` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 12 summary repeats REQ-12 of Section 9. | REQ-12: Receipt of any sort of ICMP message MUST NOT terminate the NAT mapping. |
| `12:11` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 12 summary repeats REQ-13 of Section 10. | REQ-13 If the packet received on an internal IP address has DF=1, the NAT MUST send back an ICMP message "Fragmentation needed and DF set" to the host, as described in [RFC0792]. |
| `12:12` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 12 summary repeats REQ-13a of Section 10. | a) If the packet has DF=0, the NAT MUST fragment the packet and SHOULD send the fragments in order. |
| `12:13` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 12 summary repeats REQ-14 of Section 11. | REQ-14: A NAT MUST support receiving in-order and out-of-order fragments, so it MUST have "Received Fragment Out of Order" behavior. |
| `12:14` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 12 summary repeats REQ-14a of Section 11. | a) A NAT's out-of-order fragment processing mechanism MUST be designed so that fragmentation-based DoS attacks do not compromise the NAT's ability to process in-order and unfragmented IP packets. |

## Superseded

No document obsoletes RFC 4787, so its obligations are stated where they were written.
