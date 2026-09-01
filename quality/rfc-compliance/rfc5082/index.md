# RFC 5082 - The Generalized TTL Security Mechanism (GTSM)

Supported on Linux. Every requirement this repository extracted from RFC 5082, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 75.0% | 3 of 4 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 4 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 4 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 100.0% | 6 of 6 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 10 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 25.0% | 1 of 4 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 4 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Supported on Linux |
| Enrolment | Enrolled |
| Requirements | 10 |
| Gated MUST-level | 4 |
| Obligations that bind Ze | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 1 |
| Nightly-only evidence | 0 |
| Test tags | 6 |
| Tagged units | 6 |
| Recorded audit verdicts | 0 |
| Discrimination records | 6 |
| Summary | `rfc/short/rfc5082.md` |
| Requirement shard | `rfc/requirements/rfc5082.md` |
| RFC text | `rfc/full/rfc5082.txt` |

## Enrolment

Enrolled: Generalized TTL Security Mechanism (GTSM): four MUST-level requirements. Ze installs the socket options that make the Linux stack perform the check, so conformance is judged on the whole stack. RFC5082-3-1 (transmit TTL 255) is produced by network.setIPTTL (IP_TTL / IPV6_UNICAST_HOPS, internal/core/network/ttl_linux.go), driven for BGP by reactor.parseTTLSettings (`ttl max N` derives OutTTL=255) and reactor.tuneTCPConnectionForSettings, for the listen socket by Reactor.listenTTLForListener plus network.setListenIPTTL, and for BFD by transport.applySocketOptions / applySocketOptionsV6 (IP_TTL=255, IPV6_UNICAST_HOPS=255). RFC5082-3-3 (no decrement) holds because Linux does not decrement locally originated packets; the observable proof is a peer socket carrying IP_MINTTL=255 accepting the connection. RFC5082-3-4 (never drop Trusted or Unknown) holds because network.setIPMinTTL is applied only when a peer configures a floor, so a packet no GTSM session claims meets no TTL gate, and bfd/engine.passesTTLGate admits TTL >= MinTTL. RFC5082-3-2 (the same TTL 255 rule for the related ICMP error messages) is produced by no layer: Linux emits ICMP errors from the IP stack with net.ipv4.ip_default_ttl, not with the socket IP_TTL, and no socket option gates the TTL of an inbound ICMP error. RFC5082-3-1, RFC5082-3-3 and RFC5082-3-4 are proven in both polarities by internal/core/network/ttl_gtsm_linux_test.go, each with a verified discrimination record in rfc/discrimination/rfc5082.json. RFC5082-3-2 carries no test: the behavior does not exist to test.

## What the public ledger says

**Status:** Supported on Linux

**What the ledger says is covered**

Per-peer BGP GTSM through `connection { ttl { max; set; min } }`: `parseTTLSettings` derives an outgoing TTL of 255 and an inbound floor of 255-N+1 from `ttl max N` ([`internal/component/bgp/reactor/config.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config.go)), `tuneTCPConnectionForSettings` installs IP_TTL / IPV6_UNICAST_HOPS and IP_MINTTL / IPV6_MINHOPCOUNT on the connected socket ([`internal/component/bgp/reactor/session_connection.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_connection.go), [`internal/core/network/ttl_linux.go`](https://github.com/ze-software/ze/blob/main/internal/core/network/ttl_linux.go)), and `listenTTLForListener` carries the same outgoing TTL onto the listen socket so a GTSM peer that dials in does not drop the SYN-ACK ([`internal/component/bgp/reactor/reactor.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor.go)). BFD sets IP_TTL=255 and IPV6_UNICAST_HOPS=255 on transmit and gates the received TTL ([`internal/component/bfd/transport/udp_linux.go`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/udp_linux.go), `passesTTLGate` in [`internal/component/bfd/engine/loop.go`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/loop.go)). VRRP sends and requires TTL 255 ([`internal/plugins/vrrp/packet/validate.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate.go)). Requirements bound per requirement in [`rfc/requirements/rfc5082.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc5082.md).

**What the ledger says remains**

The socket options are Linux-only: the non-Linux build returns an unsupported error and leaves the OS default ([`internal/core/network/ttl_other.go`](https://github.com/ze-software/ze/blob/main/internal/core/network/ttl_other.go)). Dynamic GTSM capability negotiation is not offered. RFC 5082 Section 2.1 neither assumes nor defines one, ze configures GTSM statically per peer, and the obligation conditional on running such a negotiation is excluded as `feature-out-of-scope` in [`rfc/extraction/rfc5082.json`](https://github.com/ze-software/ze/blob/main/rfc/extraction/rfc5082.json). That absent feature is an implementation gap a later scope decision can revisit, and not a conformance gap.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 1 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`RFC5082-3-1`](#rfc5082-3-1), [`RFC5082-3-3`](#rfc5082-3-3), [`RFC5082-3-4`](#rfc5082-3-4)

**No test and no annotation (1):** [`RFC5082-3-2`](#rfc5082-3-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5082-3-1` | The TTL field in all IP packets used for transmission of messages associated with GTSM-enabled protocol sessions MUST be set to 255 (§3) | MUST | 3 - GTSM Procedure | **positive:** `unit/verify` [`TestGTSMDialerSetsOutgoingTTLTo255`](https://github.com/ze-software/ze/blob/main/internal/core/network/ttl_gtsm_linux_test.go#L40). **negative:** `unit/verify` [`TestGTSMDialerWithoutOutTTLLeavesTheDefault`](https://github.com/ze-software/ze/blob/main/internal/core/network/ttl_gtsm_linux_test.go#L59) |
| `RFC5082-3-2` | The TTL 255 transmit and verify rule also applies to the related ICMP error handling messages of a GTSM-enabled session (§3, restated §6.1) | MUST | 3 - GTSM Procedure | **positive:** no positive test. **negative:** no negative test |
| `RFC5082-3-3` | The TTL of GTSM-enabled sessions MUST NOT be decremented by the forwarding plane (§3) | MUST NOT | 3 - GTSM Procedure | **positive:** `unit/verify` [`TestGTSMTransmittedTTLArrivesUndecremented`](https://github.com/ze-software/ze/blob/main/internal/core/network/ttl_gtsm_linux_test.go#L83). **negative:** `unit/verify` [`TestGTSMTransmittedTTLReportsTheValueSet`](https://github.com/ze-software/ze/blob/main/internal/core/network/ttl_gtsm_linux_test.go#L111) |
| `RFC5082-3-4` | Implementations MUST NOT drop, as part of GTSM processing, packets classified as Trusted or Unknown (§3) | MUST NOT | 3 - GTSM Procedure | **positive:** `unit/verify` [`TestGTSMFloorDeliversATrustedPacket`](https://github.com/ze-software/ze/blob/main/internal/core/network/ttl_gtsm_linux_test.go#L138). **negative:** `unit/verify` [`TestGTSMNoFloorDeliversAnUnknownPacket`](https://github.com/ze-software/ze/blob/main/internal/core/network/ttl_gtsm_linux_test.go#L165) |
| `RFC5082-3-5` | GTSM added to a protocol as an additional feature SHOULD NOT be enabled by default (§3) | SHOULD NOT | 3 - GTSM Procedure | **positive:** no positive test. **negative:** no negative test |
| `RFC5082-3-6` | Implementations SHOULD ensure that packets classified as Dangerous do not compete for resources with packets classified as Trusted or Unknown (§3) | SHOULD | 3 - GTSM Procedure | **positive:** no positive test. **negative:** no negative test |
| `RFC5082-3-7` | Implementations MAY drop packets classified as Dangerous (§3) | MAY | 3 - GTSM Procedure | **positive:** no positive test. **negative:** no negative test |
| `RFC5082-3-8` | A protocol peer MAY suggest the use of GTSM when the protocol defines a built-in dynamic capability negotiation for it, provided GTSM is enabled only if both peers agree (§3) | MAY | 3 - GTSM Procedure | **positive:** no positive test. **negative:** no negative test |
| `RFC5082-2-1` | Use of GTSM is OPTIONAL and can be configured on a per-peer (group) basis (§2) | OPTIONAL | 2 - Assumptions Underlying GTSM | **positive:** no positive test. **negative:** no negative test |
| `RFC5082-5.4-1` | GTSM-protected protocols are highly RECOMMENDED to avoid fragmentation and reassembly by manual MTU tuning or Path MTU Discovery (§5.4) | RECOMMENDED | 5.4 - Fragmentation Considerations | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5082-3-2`](#rfc5082-3-2) The TTL 255 transmit and verify rule also applies to the related ICMP error handling messages of a GTSM-enabled session (§3, restated §6.1) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5082-3-1`](#rfc5082-3-1)

The TTL field in all IP packets used for transmission of messages associated with GTSM-enabled protocol sessions MUST be set to 255 (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGTSMDialerWithoutOutTTLLeavesTheDefault`](https://github.com/ze-software/ze/blob/main/internal/core/network/ttl_gtsm_linux_test.go#L59) | unit/verify | revert, verified |
| positive | [`TestGTSMDialerSetsOutgoingTTLTo255`](https://github.com/ze-software/ze/blob/main/internal/core/network/ttl_gtsm_linux_test.go#L40) | unit/verify | revert, verified |

### [`RFC5082-3-2`](#rfc5082-3-2)

The TTL 255 transmit and verify rule also applies to the related ICMP error handling messages of a GTSM-enabled session (§3, restated §6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5082-3-2, so no unit is bound to it.

### [`RFC5082-3-3`](#rfc5082-3-3)

The TTL of GTSM-enabled sessions MUST NOT be decremented by the forwarding plane (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGTSMTransmittedTTLReportsTheValueSet`](https://github.com/ze-software/ze/blob/main/internal/core/network/ttl_gtsm_linux_test.go#L111) | unit/verify | revert, verified |
| positive | [`TestGTSMTransmittedTTLArrivesUndecremented`](https://github.com/ze-software/ze/blob/main/internal/core/network/ttl_gtsm_linux_test.go#L83) | unit/verify | revert, verified |

### [`RFC5082-3-4`](#rfc5082-3-4)

Implementations MUST NOT drop, as part of GTSM processing, packets classified as Trusted or Unknown (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGTSMNoFloorDeliversAnUnknownPacket`](https://github.com/ze-software/ze/blob/main/internal/core/network/ttl_gtsm_linux_test.go#L165) | unit/verify | revert, verified |
| positive | [`TestGTSMFloorDeliversATrustedPacket`](https://github.com/ze-software/ze/blob/main/internal/core/network/ttl_gtsm_linux_test.go#L138) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6 phase, rfc5082 |
| Signed off | 2026-09-01 |
| Register | rfc2119 |
| Source | rfc/full/rfc5082.txt |
| Source fingerprint | b863c08d35ee141c |
| Record | rfc/extraction/rfc5082.json |
| Mapped sentences | 3 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of This Memo, Abstract and Table of Contents. The Abstract says the document generalizes the use of a packet's TTL or Hop Limit to verify that the packet came from an adjacent node, and that it obsoletes RFC 3682. It directs no implementation. |
| `1` | Introduction | 0 | walked | Introduction. States what GTSM protects against, the assumption that most protocol peerings are between adjacent routers, and that GTSM is not a substitute for authentication. It also fixes the term 'TTL' to mean both the IPv4 TTL and the IPv6 Hop Limit, which rfc/short/rfc5082.md carries in its TTL Semantics table. The section closes with the RFC 2119 key-words paragraph, which binds nobody and which the site derivation excludes. No site. |
| `2` | Assumptions Underlying GTSM | 0 | walked | Assumptions Underlying GTSM. Five numbered assumptions, all indicative. Assumption 3, 'Use of GTSM is OPTIONAL, and can be configured on a per-peer (group) basis', is the only one carrying an RFC 2119 keyword; OPTIONAL is advisory, so the derivation raises no MUST-level site for it and the summary records it as RFC5082-2-1. The closing paragraphs state that the document does not prescribe what a router does with non-matching packets and does not choose a resource-separation mechanism. |
| `2.1` | GTSM Negotiation | 1 | walked | GTSM Negotiation. One site, 2.1:1, excluded below as feature-out-of-scope: it is conditional on dynamic GTSM negotiation, which this document neither assumes nor defines. The section's other sentences are indicative: that GTSM is manually configured between peers, and that a new protocol designed with built-in GTSM support is recommended to always run the send and validate procedures. |
| `2.2` | Assumptions on Attack Sophistication | 0 | walked | Assumptions on Attack Sophistication. States the attacker model: control traffic that looks valid, every router on the path decrementing TTL properly, ingress filtering applied before the scarce resource, and four alternative assumptions about tunnels. It closes with the sentence the whole mechanism rests on, that a receiver can set TTL 255 on transmit and reject packets from configured peers whose inbound TTL is not 255. All indicative. No site. |
| `3` | GTSM Procedure | 3 | walked | GTSM Procedure. The document's only normative section. Three sites, mapped below to RFC5082-3-1, RFC5082-3-3 and RFC5082-3-4. The sentence that extends the transmit rule to the related ICMP error messages, 'This also applies to the related ICMP error handling messages', carries no RFC 2119 keyword, so the site scan cannot see it; Section 6.1 restates it as 'This specification mandates setting and verifying TTL=255 of those as well as the main protocol packets', and Appendix B lists it as a change since RFC 3682. It is declared unsourced here as RFC5082-3-2. The section's advisory sentences also carry no MUST-level keyword and are the remaining unsourced ids: the SHOULD NOT against enabling GTSM by default when it is added to an existing protocol (RFC5082-3-5), the SHOULD that Dangerous packets not compete for resources with Trusted or Unknown ones (RFC5082-3-6), the MAY to drop Dangerous packets (RFC5082-3-7), and the MAY for a peer to suggest GTSM where the protocol defines a built-in dynamic capability negotiation (RFC5082-3-8). The three trustworthiness categories, Unknown, Trusted and Dangerous, are definitions and rfc/short/rfc5082.md carries them in its TTL Semantics table. |
| `4` | Acknowledgments | 0 | skipped (acknowledgements) | Acknowledgments. |
| `5` | Security Considerations, opening | 0 | walked | Security Considerations, opening. States that GTSM protects single-hop protocol sessions except where the peer is compromised, and that it does not protect against on-the-wire attacks. No site. |
| `5.1` | TTL (Hop Limit) Spoofing | 0 | walked | TTL (Hop Limit) Spoofing. Explains why 255 is the value chosen: the TTL is decremented once per router, so a value of 255 cannot be engineered from a location that is not directly connected. Indicative throughout. No site. |
| `5.2` | Tunneled Packets | 0 | walked | Tunneled Packets. States that a tunnel that is not integrity-protected is the exception to the observation that TTL 255 is hard to spoof, and describes what GTSM still buys over a tunnel. Indicative. No site. |
| `5.2.1` | IP Tunneled over IP | 2 | walked | IP Tunneled over IP. Two sites, 5.2.1:1 and 5.2.1:2, both excluded below as cross-document: each is a block quotation of another RFC's decapsulator rule, RFC 2003 and RFC 2784 respectively, cited by number in the sentence that introduces it. The section's own text is an analysis of what the inner TTL can be at the protocol peer in each of the two tunnel topologies. It binds no GTSM implementation. |
| `5.2.2` | IP Tunneled over MPLS | 0 | walked | IP Tunneled over MPLS. Analyses TTL handling under the RFC 3443 Uniform, Pipe and Short Pipe models, and concludes that a GTSM check is possible over Pipe model LSPs and not over Uniform model LSPs of more than one hop. Every quoted rule is RFC 3443's and none carries an RFC 2119 keyword here. No site. |
| `5.3` | Onlink Attackers | 0 | walked | Onlink Attackers. Restates Section 2.2: an attacker on a directly connected interface can disturb a GTSM-protected session unless ingress filtering is applied, so such interfaces have to be trusted. Indicative. No site. |
| `5.4` | Fragmentation Considerations | 0 | walked | Fragmentation Considerations. Explains that a non-initial fragment carries no Layer 4 information, so it classifies as Unknown, and that a reassembled packet inherits that. Its one RFC 2119 keyword is the advisory 'it is highly RECOMMENDED for GTSM-protected protocols to avoid fragmentation and reassembly', which is not MUST-level, so the derivation raises no site; the summary records it as RFC5082-5.4-1. |
| `5.5` | Multi-Hop Protocol Sessions | 0 | walked | Multi-Hop Protocol Sessions. States that the document describes only the single-hop case, and that the protection multi-hop GTSM offers is difficult to quantify. No obligation. No site. |
| `6` | Applicability Statement | 0 | walked | Applicability Statement. Limits GTSM to environments with inherently limited topologies and to directly connected peers, and states that GTSM does not protect against an attacker as close as the legitimate peer. Its modals are lowercase 'should'. No site. |
| `6.1` | Backwards Compatibility | 0 | walked | Backwards Compatibility. Records what changed against RFC 3682: this specification mandates setting and verifying TTL=255 on related ICMP error messages as well as on the main protocol packets. That is the restatement of the Section 3 sentence declared unsourced above as RFC5082-3-2, so no id is allocated here. The rest weighs the interoperability cost against RFC 3682 senders that emit related messages with TTL 64. No site. |
| `7` | References, the container heading | 0 | skipped (references) | References, the container heading. |
| `7.1` | Normative References | 0 | skipped (references) | Normative References. RFC 791, RFC 2003, RFC 2119, RFC 2461, RFC 2784, RFC 3392, RFC 3443, RFC 4213, RFC 4271 and RFC 4301. |
| `7.2` | Informative References, and the BITW mailing-list thread | 0 | skipped (references) | Informative References, and the BITW mailing-list thread. |
| `A` | Appendix A, Multi-Hop GTSM | 0 | skipped (appendix-non-normative) | Appendix A, Multi-Hop GTSM. Its first line reads 'NOTE: This is a non-normative part of the specification.' It sketches a receiver that checks the TTL is within a configured number of hops from 255 and states that such deployment is not specified in this document. |
| `B` | Appendix B, Changes Since RFC 3682 | 0 | skipped (appendix-non-normative) | Appendix B, Changes Since RFC 3682. A change list. Its fourth entry names the related-messages rule that Section 3 states and that RFC5082-3-2 carries. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `2.1:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | The feature is OPTIONAL and ze decided not to offer it. RFC 5082 Section 2.1: 'This document assumes that, when used with existing protocols, GTSM will be manually configured between protocol peers. That is, no automatic GTSM capability negotiation, such as is provided by RFC 3392 [RFC3392], is assumed or defined.' The same section adds that 'this specification does not offer a generic GTSM capability negotiation mechanism'. This obligation is conditional on running such a negotiation, and ze runs none: parseTTLSettings (internal/component/bgp/reactor/config.go) is the only producer of a peer's GTSM TTL values and it reads the static `connection { ttl }` configuration map alone, and tuneTCPConnectionForSettings (internal/component/bgp/reactor/session_connection.go) installs IP_TTL and IP_MINTTL on the TCP socket at connectionEstablished time, before any OPEN is exchanged, so no message of ze's could carry a GTSM negotiation. The BFD and VRRP paths are the same shape: transport.applySocketOptions (internal/component/bfd/transport/udp_linux.go) sets IP_TTL=255 unconditionally at socket setup, and gtsmTTL (internal/plugins/vrrp/packet/validate.go) is a constant. There is no GTSM capability code and no GTSM negotiation message anywhere in ze. This is a SCOPE DECISION and not outstanding work. The absent feature is recorded as an implementation gap in docs/features/rfc-status.md, which a later scope decision can revisit; it is not a conformance gap. | If, however, dynamic negotiation of GTSM support is necessary, protocol messages used for such negotiation MUST be authenticated using other security mechanisms to prevent DoS attacks. |
| `5.2.1:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The obligation belongs to RFC 2003, which the sentence that introduces the block quotation cites by number: 'For IP-in-IP tunnels, RFC 2003 specifies the following decapsulator behavior'. It binds an IP-in-IP decapsulator to discard an inner datagram whose TTL is 0 after decapsulation. RFC 5082 quotes it to show what the inner TTL can be at the protocol peer, and states no obligation of its own here. | If, after decapsulation, the inner datagram has TTL = 0, the decapsulator MUST discard the datagram. |
| `5.2.1:2` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The obligation belongs to RFC 2784, which the sentence that introduces the block quotation cites by number: 'And similarly, for GRE tunnels, RFC 2784 specifies the following decapsulator behavior'. It binds a GRE tunnel endpoint to forward on the inner destination address and to decrement the payload TTL. RFC 5082 quotes it for the same reason as site 5.2.1:1 and states no obligation of its own here. | When a tunnel endpoint decapsulates a GRE packet which has an IPv4 packet as the payload, the destination address in the IPv4 payload packet header MUST be used to forward the packet and the TTL of the payload packet MUST be decremented. |

## Superseded

No document obsoletes RFC 5082, so its obligations are stated where they were written.
