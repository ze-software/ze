# RFC 5082 - The Generalized TTL Security Mechanism (GTSM)

Supported on Linux. Every requirement this repository extracted from RFC 5082, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 75.0% | 3 of 4 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 25.0% | 1 of 4 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 4 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 4 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 4 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 100.0% | 14 of 14 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 10 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 4 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 4 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 4 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 4 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
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
| Public status | Supported on Linux |
| Enrolment | Enrolled |
| Requirements | 10 |
| Gated MUST-level | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 14 |
| Tagged units | 14 |
| Recorded audit verdicts | 3 |
| Discrimination records | 14 |
| Summary | `rfc/short/rfc5082.md` |
| Requirement shard | `rfc/requirements/rfc5082.md` |
| RFC text | `rfc/full/rfc5082.txt` |

## Enrolment

Enrolled: Generalized TTL Security Mechanism (GTSM): four MUST-level requirements. Ze installs the socket options, per-peer host-route hop limit and nftables related-message policy; Linux performs the packet processing. Transmit TTL is set by network.setIPTTL, network.setListenIPTTL and BFD transport socket setup. The local-output no-decrement proof captures TCP SYN and data across a configured IPv4 veth in internal/core/network/rfc5082_ttl_gtsm_egress_integration_linux_test.go; it makes no claim about transit forwarding and keeps only positive coverage under owner ruling 11 (2026-10-02 continuation). The loopback units remain calibration, not egress evidence. The receive floor is installed only for configured peers. Related ICMP transmit and receive behavior is tested by internal/component/gtsm/rfc5082_gtsm_linux_test.go; the IPv4 receive policy checks the quoted peer destination and TCP port because Linux tcp_v4_err checks the quoted TTL rather than the outer TTL. See Encoding Rules and Pitfalls for the stack boundaries.

## What the public ledger says

**Status:** Supported on Linux

**What the ledger says is covered**

Per-peer BGP GTSM through `connection { ttl { max; set; min } }`: `parseTTLSettings` derives an outgoing TTL of 255 and an inbound floor of 255-N+1 from `ttl max N` ([`internal/component/bgp/reactor/config.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config.go)), `tuneTCPConnectionForSettings` installs IP_TTL / IPV6_UNICAST_HOPS and IP_MINTTL / IPV6_MINHOPCOUNT on the connected socket ([`internal/component/bgp/reactor/session_connection.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_connection.go), [`internal/core/network/ttl_linux.go`](https://github.com/ze-software/ze/blob/main/internal/core/network/ttl_linux.go)), and `listenTTLForListener` carries the same outgoing TTL onto the listen socket so a GTSM peer that dials in does not drop the SYN-ACK ([`internal/component/bgp/reactor/reactor.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor.go)). BFD sets IP_TTL=255 and IPV6_UNICAST_HOPS=255 on transmit and gates the received TTL ([`internal/component/bfd/transport/udp_linux.go`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/udp_linux.go), `passesTTLGate` in [`internal/component/bfd/engine/loop.go`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/loop.go)). VRRP sends and requires TTL 255 ([`internal/plugins/vrrp/packet/validate.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate.go)). The related ICMP error messages of a BGP GTSM peer are carried by `internal/component/gtsm`, which the reactor's peer reconcile publishes the peer set to (`Reactor.gtsmPeers`, [`internal/component/bgp/reactor/gtsm.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/gtsm.go)): a host route carrying RTAX_HOPLIMIT 255 for the transmit rule, and the `ze_gtsm` nftables input table for the IPv4 receive rule. Requirements bound per requirement in [`rfc/requirements/rfc5082.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc5082.md).

**What the ledger says remains**

- **The socket options are Linux-only:** the non-Linux build returns an unsupported error and leaves the OS default ([`internal/core/network/ttl_other.go`](https://github.com/ze-software/ze/blob/main/internal/core/network/ttl_other.go)). Dynamic GTSM capability negotiation is not offered. RFC 5082 Section 2.1 neither assumes nor defines one, ze configures GTSM statically per peer, and the obligation conditional on running such a negotiation is excluded as `feature-out-of-scope` in [`rfc/extraction/rfc5082.json`](https://github.com/ze-software/ze/blob/main/rfc/extraction/rfc5082.json). That absent feature is an implementation gap a later scope decision can revisit, and not a conformance gap. The related-message filter claims a BGP GTSM session: it associates an ICMP error with a session by the header the error quotes alone, the quoted IPv4 destination (the peer) and the quoted TCP port on either side (peerTerms, [`internal/component/gtsm/gtsm.go`](https://github.com/ze-software/ze/blob/main/internal/component/gtsm/gtsm.go), and lowerICMPErrorQuotedDestinationMatch, [`internal/plugins/firewall/nft/lower_linux.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/firewall/nft/lower_linux.go)), reads no outer source, and drops the error below the floor, so the related messages of a BFD or VRRP GTSM session, which quote no TCP header, are not claimed by it. The drop policy for a Dangerous related message is not configurable; ze applies the answer it already applies to a Dangerous main packet.
- **Owner ruling, 2026-09-15:** the Section 3 sentence "are expected to be configurable" is a suggestion outside the RFC 2119 vocabulary and not a requirement, so the drop stays fixed and no option is added.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the gated population |
| Annotated (including scoped evidence) | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`RFC5082-3-1`](#rfc5082-3-1), [`RFC5082-3-2`](#rfc5082-3-2), [`RFC5082-3-4`](#rfc5082-3-4)

**Annotated (including scoped evidence) (1):** [`RFC5082-3-3`](#rfc5082-3-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5082-3-1` | The TTL field in all IP packets used for transmission of messages associated with GTSM-enabled protocol sessions MUST be set to 255 (§3) | MUST | 3 - GTSM Procedure | **positive:** `unit/verify` [`TestGTSMDialerSetsOutgoingTTLTo255`](https://github.com/ze-software/ze/blob/main/internal/core/network/rfc5082_ttl_gtsm_linux_test.go#L40). **negative:** `unit/verify` [`TestGTSMDialerWithoutOutTTLLeavesTheDefault`](https://github.com/ze-software/ze/blob/main/internal/core/network/rfc5082_ttl_gtsm_linux_test.go#L59) |
| `RFC5082-3-2` | RFC 3682 [RFC3682] did not specify how to handle "related messages" (ICMP errors).  This specification mandates setting and verifying TTL=255 of those as well as the main protocol packets. (§6.1) | MUST | 6.1 - Backwards Compatibility | **positive:** `unit/verify` [`TestGTSMDropsADangerousQuotedICMPError`](https://github.com/ze-software/ze/blob/main/internal/component/gtsm/rfc5082_gtsm_linux_test.go#L194). **positive:** `unit/verify` [`TestGTSMMinHopCountDropsALowHopLimitICMPv6Error`](https://github.com/ze-software/ze/blob/main/internal/component/gtsm/rfc5082_gtsm_linux_test.go#L333). **positive:** `unit/verify` [`TestGTSMTransmittedICMPErrorCarriesTTL255`](https://github.com/ze-software/ze/blob/main/internal/component/gtsm/rfc5082_gtsm_linux_test.go#L70). **positive:** `unit/verify` [`TestGTSMTransmittedICMPv6ErrorCarriesHopLimit255`](https://github.com/ze-software/ze/blob/main/internal/component/gtsm/rfc5082_gtsm_linux_test.go#L116). **negative:** `unit/verify` [`TestGTSMDeliversAQuotedICMPErrorAtTTL255`](https://github.com/ze-software/ze/blob/main/internal/component/gtsm/rfc5082_gtsm_linux_test.go#L234). **negative:** `unit/verify` [`TestGTSMTransmittedICMPErrorWithoutTheRouteMetricIsNot255`](https://github.com/ze-software/ze/blob/main/internal/component/gtsm/rfc5082_gtsm_linux_test.go#L92). **negative:** `unit/verify` [`TestGTSMTransmittedICMPv6ErrorWithoutTheRouteMetricIsNot255`](https://github.com/ze-software/ze/blob/main/internal/component/gtsm/rfc5082_gtsm_linux_test.go#L134). **negative:** `unit/verify` [`TestGTSMWithoutMinHopCountDeliversTheSameICMPv6Error`](https://github.com/ze-software/ze/blob/main/internal/component/gtsm/rfc5082_gtsm_linux_test.go#L359) |
| `RFC5082-3-3` | The TTL of GTSM-enabled sessions MUST NOT be decremented. (§3) | MUST NOT | 3 - GTSM Procedure | **positive:** `unit/verify` [`TestGTSMConfiguredEgressDoesNotDecrement`](https://github.com/ze-software/ze/blob/main/internal/core/network/rfc5082_ttl_gtsm_egress_integration_linux_test.go#L27). **negative:** no negative test. **{single-polarity}:** owner ruling 11 (2026-10-02 continuation ba93202e): local-output proof over configured egress, not transit forwarding; no legitimate negative input exists. |
| `RFC5082-3-4` | + MUST NOT drop (as part of GTSM processing) packets classified as Trusted or Unknown. (§3) | MUST NOT | 3 - GTSM Procedure | **positive:** `unit/verify` [`TestGTSMDeliversAnICMPErrorNoSessionClaims`](https://github.com/ze-software/ze/blob/main/internal/component/gtsm/rfc5082_gtsm_linux_test.go#L279). **positive:** `unit/verify` [`TestGTSMFloorDeliversATrustedPacket`](https://github.com/ze-software/ze/blob/main/internal/core/network/rfc5082_ttl_gtsm_linux_test.go#L129). **negative:** `unit/verify` [`TestGTSMNoFloorDeliversAnUnknownPacket`](https://github.com/ze-software/ze/blob/main/internal/core/network/rfc5082_ttl_gtsm_linux_test.go#L156) |
| `RFC5082-3-5` | If GTSM is not built into the protocol and is used as an additional feature (e.g., for BGP, LDP, or MSDP), it SHOULD NOT be enabled by default in order to remain backward-compatible with the unmodified protocol. (§3) | SHOULD NOT | 3 - GTSM Procedure | **positive:** no positive test. **negative:** no negative test |
| `RFC5082-3-6` | SHOULD ensure that packets classified as Dangerous do not compete for resources with packets classified as Trusted or Unknown. (§3) | SHOULD | 3 - GTSM Procedure | **positive:** no positive test. **negative:** no negative test |
| `RFC5082-3-7` | MAY drop packets classified as Dangerous. (§3) | MAY | 3 - GTSM Procedure | **positive:** no positive test. **negative:** no negative test |
| `RFC5082-3-8` | However, if the protocol defines a built-in dynamic capability negotiation for GTSM, a protocol peer MAY suggest the use of GTSM provided that GTSM would only be enabled if both peers agree to use it. (§3) | MAY | 3 - GTSM Procedure | **positive:** no positive test. **negative:** no negative test |
| `RFC5082-2-1` | Use of GTSM is OPTIONAL, and can be configured on a per-peer (group) basis. (§2) | OPTIONAL | 2 - Assumptions Underlying GTSM | **positive:** no positive test. **negative:** no negative test |
| `RFC5082-5.4-1` | As such, it is highly RECOMMENDED for GTSM-protected protocols to avoid fragmentation and reassembly by manual MTU tuning, using adaptive measures such as Path MTU Discovery (PMTUD), or any other available method [RFC1191], [RFC1981], or [RFC4821]. (§5.4) | RECOMMENDED | 5.4 - Fragmentation Considerations | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 5082 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5082-3-1`](#rfc5082-3-1)

The TTL field in all IP packets used for transmission of messages associated with GTSM-enabled protocol sessions MUST be set to 255 (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGTSMDialerWithoutOutTTLLeavesTheDefault`](https://github.com/ze-software/ze/blob/main/internal/core/network/rfc5082_ttl_gtsm_linux_test.go#L59) | unit/verify | revert, verified |
| positive | [`TestGTSMDialerSetsOutgoingTTLTo255`](https://github.com/ze-software/ze/blob/main/internal/core/network/rfc5082_ttl_gtsm_linux_test.go#L40) | unit/verify | revert, verified |

### [`RFC5082-3-2`](#rfc5082-3-2)

RFC 3682 [RFC3682] did not specify how to handle "related messages" (ICMP errors).  This specification mandates setting and verifying TTL=255 of those as well as the main protocol packets. (§6.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read 2026-09-27 against the §6.1 quote (setting AND verifying TTL=255 on related ICMP errors). Setting: forbidden is an ICMP error toward a GTSM peer leaving with TTL other than 255; TestGTSMTransmittedICMPErrorCarriesTTL255 fails on packet[8]!=255 and TestGTSMTransmittedICMPv6ErrorCarriesHopLimit255 on packet[7]!=255, both over the route applyHopLimitRoutes installs; the withdraw negatives bind the 255 to that route. Verifying: forbidden is a below-floor ICMP error about the session being delivered; TestGTSMDropsADangerousQuotedICMPError fails when icmpInType rises, TestGTSMMinHopCountDropsALowHopLimitICMPv6Error when TCPMinTTLDrop does not; the TTL-255 negatives show delivery at 255.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGTSMDeliversAQuotedICMPErrorAtTTL255`](https://github.com/ze-software/ze/blob/main/internal/component/gtsm/rfc5082_gtsm_linux_test.go#L234) | unit/verify | revert, verified |
| negative | [`TestGTSMTransmittedICMPErrorWithoutTheRouteMetricIsNot255`](https://github.com/ze-software/ze/blob/main/internal/component/gtsm/rfc5082_gtsm_linux_test.go#L92) | unit/verify | revert, verified |
| negative | [`TestGTSMTransmittedICMPv6ErrorWithoutTheRouteMetricIsNot255`](https://github.com/ze-software/ze/blob/main/internal/component/gtsm/rfc5082_gtsm_linux_test.go#L134) | unit/verify | revert, verified |
| negative | [`TestGTSMWithoutMinHopCountDeliversTheSameICMPv6Error`](https://github.com/ze-software/ze/blob/main/internal/component/gtsm/rfc5082_gtsm_linux_test.go#L359) | unit/verify | revert, verified |
| positive | [`TestGTSMDropsADangerousQuotedICMPError`](https://github.com/ze-software/ze/blob/main/internal/component/gtsm/rfc5082_gtsm_linux_test.go#L194) | unit/verify | revert, verified |
| positive | [`TestGTSMMinHopCountDropsALowHopLimitICMPv6Error`](https://github.com/ze-software/ze/blob/main/internal/component/gtsm/rfc5082_gtsm_linux_test.go#L333) | unit/verify | revert, verified |
| positive | [`TestGTSMTransmittedICMPErrorCarriesTTL255`](https://github.com/ze-software/ze/blob/main/internal/component/gtsm/rfc5082_gtsm_linux_test.go#L70) | unit/verify | revert, verified |
| positive | [`TestGTSMTransmittedICMPv6ErrorCarriesHopLimit255`](https://github.com/ze-software/ze/blob/main/internal/component/gtsm/rfc5082_gtsm_linux_test.go#L116) | unit/verify | revert, verified |

### [`RFC5082-3-3`](#rfc5082-3-3)

The TTL of GTSM-enabled sessions MUST NOT be decremented. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent post-lint judgment after reading the full RFC and actual socket boundary. RFC 5082 Section 3: 'On some architectures, the TTL of control plane originated traffic is under some configurations decremented in the forwarding plane. The TTL of GTSM-enabled sessions MUST NOT be decremented.' TestGTSMConfiguredEgressDoesNotDecrement configures RealDialer.OutTTL=255 and observes both SYN and exact data payload at the adjacent veth in a separate namespace. Every captured sender packet must carry TTL exactly 255. newTCPEgress creates two network namespaces, addresses the MTU-1500 veth and checks the route's egress link, so local delivery cannot substitute for the claimed path. Its promoted-field Veth literal retains Name, MTU, PeerName and PeerNamespace. receive/receiveFrame now return errors that the calling test makes fatal; IP total length, IP/TCP header bounds, peer link, bounded frame count, socket timeout and owned packet clone still guard the observer. Actual producer network.go::DialContext sets IP_TTL before the SYN through ttl_linux.go::setIPTTL; Linux's local-output path emits the captured packets. A TTL-setting omission or decrement to 254 fails the exact on-wire assertion. The owner's single-positive local-output scope is retained: there is no legitimate negative input for this invariant, and normal transit forwarding decrement is not the prohibited local-output behavior. This does not invent IPv6, ICMP or transit proof for this IPv4 carrier. Reviewed parent's job-network-wire-observers-privileged-cdc8e23e.log lines 43-46: this unit PASS, package PASS, no skip. Parent reports native Linux -race and only the test executable elevated via -exec 'sudo -n' within fixture-created private namespaces. Earlier pre-body privilege/restore failure proves nothing. Existing native setIPTTL-panic discrimination must be renewed for the changed unit; this audit neither executed nor claims that red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestGTSMConfiguredEgressDoesNotDecrement`](https://github.com/ze-software/ze/blob/main/internal/core/network/rfc5082_ttl_gtsm_egress_integration_linux_test.go#L27) | unit/verify | revert, verified |

### [`RFC5082-3-4`](#rfc5082-3-4)

+ MUST NOT drop (as part of GTSM processing) packets classified as Trusted or Unknown. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read 2026-09-27. Two clauses. Trusted dropped: TestGTSMFloorDeliversATrustedPacket fails if the packet at the floor is not received (receiveByte). Unknown dropped: TestGTSMNoFloorDeliversAnUnknownPacket fails if a low-TTL packet on a no-floor socket is not delivered, and TestGTSMDeliversAnICMPErrorNoSessionClaims fails if icmpInType does not rise for three unclaimed ICMP errors at TTL 1.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGTSMNoFloorDeliversAnUnknownPacket`](https://github.com/ze-software/ze/blob/main/internal/core/network/rfc5082_ttl_gtsm_linux_test.go#L156) | unit/verify | revert, verified |
| positive | [`TestGTSMDeliversAnICMPErrorNoSessionClaims`](https://github.com/ze-software/ze/blob/main/internal/component/gtsm/rfc5082_gtsm_linux_test.go#L279) | unit/verify | revert, verified |
| positive | [`TestGTSMFloorDeliversATrustedPacket`](https://github.com/ze-software/ze/blob/main/internal/core/network/rfc5082_ttl_gtsm_linux_test.go#L129) | unit/verify | revert, verified |

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
