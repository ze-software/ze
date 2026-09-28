# RFC 3948 - UDP Encapsulation of IPsec ESP Packets

Partial. Every requirement this repository extracted from RFC 3948, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 54.5% | 6 of 11 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 27.3% | 3 of 11 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 11 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 13.6% | 3 of 22 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 11 | of 16 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 11 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 9.1% | 1 of 11 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 11 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 11 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 9.1% | 1 of 11 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 9 | of 11 gated MUSTs judged | 6 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 11 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 16 |
| Gated MUST-level | 11 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 1 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 2 |
| Test tags | 22 |
| Tagged units | 22 |
| Recorded audit verdicts | 9 |
| Discrimination records | 3 |
| Summary | `rfc/short/rfc3948.md` |
| Requirement shard | `rfc/requirements/rfc3948.md` |
| RFC text | `rfc/full/rfc3948.txt` |

## Enrolment

Enrolled: UDP Encapsulation of IPsec ESP Packets (NAT-Traversal): eleven MUST-level requirements. Eight are met with tags in internal/component/ike: 2.1-1 (the ESP SPI is never zero), 2.1-3 (demultiplex on port 4500 -- a four-zero-byte marker is IKE, a single 0xFF byte is a NAT keepalive, otherwise ESP), 2.2-1 (the non-ESP marker of four zero bytes is prepended to IKE), 1-1 (a tunnel-mode client supports tunnel mode: every Child SA that negotiated no USE_TRANSPORT_MODE installs as tunnel, and a peer request the operator did not configure cannot change that), and 4-3 (a received NAT-keepalive reaches no SA, so it is never read as evidence the connection is live) carry positive+negative tags; 2.1-2 (the ESP SA uses UDP port 4500 for encapsulation), 4-1 (NAT keepalives are sent at a conservative sub-binding-timeout interval) and 2.3-2 (the keepalive payload is one octet of 0xFF) are {single-polarity: positive} with new tests. The last three ids were extracted by the 2026-08-31 extraction sign-off, which found their sites unmapped. 3.1.2-1 (ESP-in-UDP encapsulation and transport-mode checksum fixup), 3.1.2-2 (inner checksum handling on decapsulation), and 2.1-4 (do not depend on a zero UDP checksum) are {not-applicable}: ze delegates UDP-ESP encapsulation and decapsulation to the kernel via XFRM_ENCAP_ESPINUDP and never inspects the UDP checksum.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

NAT-T non-ESP marker, UDP 4500 encapsulation, NAT keepalive, XFRM UDP encap attributes.

**What the ledger says remains**

Section 5.1 tunnel mode conflict (`RFC3948-5.1-1`) is a gap. Ze assigns no inner address to a remote peer, so it devises no way of preventing two peers behind one NAT from reaching it with the same self-chosen inner address. The section's RECOMMENDED remedy is a locally unique address per peer, and the allocator for it is written and unreached: `Pool.Allocate` ([`internal/core/eap/pool.go`](https://github.com/ze-software/ze/blob/main/internal/core/eap/pool.go)) has no non-test caller, and `reloadPool` ([`internal/component/ike/engine/apply.go`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/apply.go)) stores the pool it builds in a field no code reads. No engine code constructs a Configuration payload, so ze sends no CFG_REPLY. Closing it is [`plan/immediate/spec-ike-virtual-ip-assignment.md`](https://github.com/ze-software/ze/blob/main/plan/immediate/spec-ike-virtual-ip-assignment.md).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 6 | one part of the gated population |
| Annotated instead of tested | 5 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 2 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **11** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (6):** [`RFC3948-2.1-1`](#rfc3948-2.1-1), [`RFC3948-3.1.2-1`](#rfc3948-3.1.2-1), [`RFC3948-3.1.2-2`](#rfc3948-3.1.2-2), [`RFC3948-1-1`](#rfc3948-1-1), [`RFC3948-4-3`](#rfc3948-4-3), [`RFC3948-5.2-1`](#rfc3948-5.2-1)

**Annotated instead of tested (5):** [`RFC3948-2.1-2`](#rfc3948-2.1-2), [`RFC3948-2.1-4`](#rfc3948-2.1-4), [`RFC3948-2.3-2`](#rfc3948-2.3-2), [`RFC3948-3.1.1-1`](#rfc3948-3.1.1-1), [`RFC3948-5.1-1`](#rfc3948-5.1-1)

**Evidence that runs nightly only (2):** [`RFC3948-3.1.2-1`](#rfc3948-3.1.2-1), [`RFC3948-3.1.2-2`](#rfc3948-3.1.2-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3948-2.1-1` | The SPI field in the ESP header MUST NOT be a zero value. (S2.1) | MUST NOT | 2.1 - UDP-Encapsulated ESP Header Format | **positive:** `unit/verify` [`TestGenerateESPSPI`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L247). **negative:** `unit/verify` [`TestGenerateESPSPI`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L261) |
| `RFC3948-2.1-2` | the Source Port and Destination Port MUST be the same as that used by IKE traffic (S2.1) | MUST | 2.1 - UDP-Encapsulated ESP Header Format | **positive:** `unit/verify` [`TestChildSANATTEncapPorts`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L280). **negative:** no negative test. **{single-polarity}:** the Child SA install unconditionally sets both UDP-encap ports to the fixed IKE NAT-T port 4500 (internal/component/ike/engine/child.go:237-238,265-266); no ze code path produces a non-4500 encap port, so there is no mismatched-port case to reject |
| `RFC3948-3.1.2-1` | Depending on local policy, one of the following MUST be done: 1. If the protocol header after the ESP header is a TCP/UDP header and the peer's real source and destination IP address have been received according to [RFC3947], incrementally recompute the TCP/UDP checksum: * Subtract the IP source address in the received packet from the checksum. * Add the real IP source address received via IKE to the checksum (obtained from the NAT-OA) * Subtract the IP destination address in the received packet from the checksum. * Add the real IP destination address received via IKE to the checksum (obtained from the NAT-OA). Note: If the received and real address are the same for a given address (e.g., say the source address), the operations cancel and don't need to be performed. 2. If the protocol header after the ESP header is a TCP/UDP header, recompute the checksum field in the TCP/UDP header. 3. If the protocol header after the ESP header is a UDP header, set the checksum field to zero in the UDP header. If the protocol after the ESP header is a TCP header, and if there is an option to flag to the stack that the TCP checksum does not need to be computed, then that flag MAY be used. (S3.1.2) | MUST | 3.1.2 - Transport Mode Decapsulation NAT Procedure | **positive:** `interop/nightly` [`checkNATTTransportInnerChecksum`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L1316). **negative:** `interop/nightly` [`checkNATTTunnelInnerChecksum`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L1355). **nightly-only:** every test bound to this requirement runs in the scheduled workflow alone, so nothing here is proven on the merge path |
| `RFC3948-3.1.2-2` | Tunnel mode TCP checksums MUST be verified (S3.1.2) | MUST | 3.1.2 - Transport Mode Decapsulation NAT Procedure | **positive:** `interop/nightly` [`checkNATTTunnelInnerChecksum`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L1352). **negative:** `interop/nightly` [`checkNATTTransportInnerChecksum`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L1319). **nightly-only:** every test bound to this requirement runs in the scheduled workflow alone, so nothing here is proven on the merge path |
| `RFC3948-2.1-4` | receivers MUST NOT depend on the UDP checksum being a zero value (S2.1) | MUST NOT | 2.1 - UDP-Encapsulated ESP Header Format | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze reads IKE/ESP demux from the OS UDP socket (internal/component/ike/engine/register.go:421) and never inspects the UDP checksum, so no ze code path can depend on it; ESP-in-UDP checksum handling is the kernel's |
| `RFC3948-4-1` | A peer SHOULD send a NAT-keepalive packet if a need for it is detected according to [RFC3947] and if no other packet to the peer has been sent in M seconds. (S4) | SHOULD | 4 - NAT Keepalive Procedure | **positive:** `unit/verify` [`TestKeepaliveDefaultInterval`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/keepalive_test.go#L62). **positive:** `unit/verify` [`TestNATKeepalive`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/keepalive_test.go#L13). **negative:** no negative test. **{single-polarity}:** the keepalive interval is a fixed conservative 20s constant (internal/component/ike/transport/keepalive.go:13), well under a typical NAT UDP binding lifetime; there is no dynamic binding-timeout query to drive a negative |
| `RFC3948-1-1` | IPsec tunnel mode clients MUST support tunnel mode (S1) | MUST | 1 - Introduction | **positive:** `unit/verify` [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L187). **positive:** `unit/verify` [`TestTunnelModeIsTheChildSADefault`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L460). **negative:** `unit/verify` [`TestTunnelModeIsTheChildSADefault`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L501) |
| `RFC3948-2.3-2` | The sender MUST use a one-octet-long payload with the value 0xFF. (S2.3) | MUST | 2.3 - NAT-Keepalive Packet Format | **positive:** `unit/verify` [`TestNATKeepalive`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/keepalive_test.go#L16). **negative:** no negative test. **{single-polarity}:** the keepalive payload is a fixed one-octet constant (internal/component/ike/transport/keepalive.go:14,48) that no input can vary, so there is no non-conforming sender case to drive |
| `RFC3948-4-3` | Reception of NAT-keepalive packets MUST NOT be used to detect whether a connection is live (S4) | MUST NOT | 4 - NAT Keepalive Procedure | **positive:** `unit/verify` [`TestNATKeepaliveReachesNoSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_test.go#L657). **negative:** `unit/verify` [`TestNATKeepaliveIsNeverDeliveredToASession`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/keepalive_test.go#L80). **negative:** `unit/verify` [`TestNATKeepaliveReachesNoSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_test.go#L666) |
| `RFC3948-3.1.1-1` | Depending on local policy, one of the following MUST be done: 1. If a valid source IP address space has been defined in the policy for the encapsulated packets from the peer, check that the source IP address of the inner packet is valid according to the policy. 2. If an address has been assigned for the remote peer, check that the source IP address used in the inner packet is the assigned IP address. 3. NAT is performed for the packet, making it suitable for transport in the local network. (S3.1.1) | MUST | 3.1.1 - Tunnel Mode Decapsulation NAT Procedure | **positive:** `unit/verify` [`TestChildInboundPolicyDefinesTheValidInnerSourceSpace`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc3948_inner_source_policy_test.go#L28). **positive:** `unit/verify` [`TestChildSAInboundPolicyUsesNegotiatedTS`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L415). **negative:** no negative test. **{single-polarity}:** ze takes the first option by installing the inbound policy with the negotiated remote traffic selector as its source selector (childPolicyParams, internal/component/ike/engine/child.go), and the drop of a mismatched inner packet is the kernel's, so ze holds no rejecting branch |
| `RFC3948-5.1-1` | Because SGW will now see two possible SAs that lead to 10.1.2.3, it can become confused about where to send packets coming from Suzy's server. Implementors MUST devise ways of preventing this from occurring. (S5.1) | MUST | 5.1 - Tunnel Mode Conflict | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze devises no such way, because it assigns no inner address at all. The section's RECOMMENDED remedy is to give each remote peer a locally unique address, and the allocator for it exists and is unreached: Pool.Allocate (internal/core/eap/pool.go) leases a unique address and refuses on exhaustion with ErrPoolExhausted, yet its only callers are pool_test.go and pool_release_test.go. reloadPool (internal/component/ike/engine/apply.go) builds the pool from the remote-access config and stores it in ikeEngineState.pool, which no code reads, so no lease ever reaches a peer. No engine code constructs a wire.PayloadCP either, so ze sends no CFG_REPLY and a client keeps whatever inner address it chose for itself. Two peers behind one NAT that both chose 10.1.2.3 would therefore present the gateway with the ambiguity the section names, and ze holds nothing that prevents it. Closing this is plan/immediate/spec-ike-virtual-ip-assignment.md |
| `RFC3948-5.2-1` | Implementations MUST handle this situation, either by disallowing conflicting connections, or by other means. (S5.2) | MUST | 5.2 - Transport Mode Conflict | **positive:** `unit/verify` [`TestPolicyOwnerSeparatesDistinctSelectors`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/policy_owner_test.go#L263). **negative:** `unit/verify` [`TestPolicyOwnerRefusesASecondPeerOnOneSelector`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/policy_owner_test.go#L40) |
| `RFC3948-2.1-5` | the IPv4 UDP Checksum SHOULD be transmitted as a zero value (S2.1) | SHOULD | 2.1 - UDP-Encapsulated ESP Header Format | **positive:** no positive test. **negative:** no negative test |
| `RFC3948-2.3-1` | The receiver SHOULD ignore a received NAT-keepalive packet. (S2.3) | SHOULD | 2.3 - NAT-Keepalive Packet Format | **positive:** `unit/verify` [`TestIsNATKeepalive`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/nat_test.go#L122). **negative:** `unit/verify` [`TestIsNATKeepalive`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/nat_test.go#L128) |
| `RFC3948-3.1.2-3` | If the protocol after the ESP header is a TCP header, and if there is an option to flag to the stack that the TCP checksum does not need to be computed, then that flag MAY be used. This SHOULD only be done for transport mode, and if the packet is integrity protected. (S3.1.2) | MAY | 3.1.2 - Transport Mode Decapsulation NAT Procedure | **positive:** no positive test. **negative:** no negative test |
| `RFC3948-4-2` | M is a locally configurable parameter with a default value of 20 seconds. (S4) | MAY | 4 - NAT Keepalive Procedure | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC3948-2.1-4`](#rfc3948-2.1-4) receivers MUST NOT depend on the UDP checksum being a zero value (S2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze reads IKE/ESP demux from the OS UDP socket (internal/component/ike/engine/register.go:421) and never inspects the UDP checksum, so no ze code path can depend on it; ESP-in-UDP checksum handling is the kernel's |
| [`RFC3948-5.1-1`](#rfc3948-5.1-1) Because SGW will now see two possible SAs that lead to 10.1.2.3, it can become confused about where to send packets coming from Suzy's server. Implementors MUST devise ways of preventing this from occurring. (S5.1) | {gap}, no test | ze devises no such way, because it assigns no inner address at all. The section's RECOMMENDED remedy is to give each remote peer a locally unique address, and the allocator for it exists and is unreached: Pool.Allocate (internal/core/eap/pool.go) leases a unique address and refuses on exhaustion with ErrPoolExhausted, yet its only callers are pool_test.go and pool_release_test.go. reloadPool (internal/component/ike/engine/apply.go) builds the pool from the remote-access config and stores it in ikeEngineState.pool, which no code reads, so no lease ever reaches a peer. No engine code constructs a wire.PayloadCP either, so ze sends no CFG_REPLY and a client keeps whatever inner address it chose for itself. Two peers behind one NAT that both chose 10.1.2.3 would therefore present the gateway with the ambiguity the section names, and ze holds nothing that prevents it. Closing this is plan/immediate/spec-ike-virtual-ip-assignment.md |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC3948-2.1-1`](#rfc3948-2.1-1)

The SPI field in the ESP header MUST NOT be a zero value. (S2.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden behaviour: an ESP SA whose SPI is 0. TestGenerateESPSPI asserts spi != 0 over 100 calls to generateESPSPI, and both tags sit on that one assertion (two hats). The assertion cannot observe the loss of the producer's guard: with the retry loop in child.go removed, a random 32-bit draw is 0 with probability 2^-32 per call, so the test stays green. No unit injects a zero draw into the generator.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGenerateESPSPI`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L261) | unit/verify | unproven |
| positive | [`TestGenerateESPSPI`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L247) | unit/verify | unproven |

### [`RFC3948-2.1-2`](#rfc3948-2.1-2)

the Source Port and Destination Port MUST be the same as that used by IKE traffic (S2.1)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. Forbidden behaviour: UDP-encapsulated ESP whose source or destination port differs from the ports IKE traffic uses, which is the NAT-translated case (Section 5.2 shows IKE reaching the server as <Y,4500>). TestChildSANATTEncapPorts asserts both encap ports equal the constant transport.NATTPort (4500), which is what the code does, not what the RFC says: installChildSA (child.go) takes the peer's real IKE port only on the MOBIKE path (udpRemotePort) and otherwise falls back to 4500, so a peer whose IKE arrives from a translated port Y gets ESP on 4500 and the test stays green. The {single-polarity} marker's claim that no path produces a non-4500 port is also false for the MOBIKE path.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSANATTEncapPorts`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L280) | unit/verify | unproven |

### [`RFC3948-3.1.2-1`](#rfc3948-3.1.2-1)

Depending on local policy, one of the following MUST be done: 1. If the protocol header after the ESP header is a TCP/UDP header and the peer's real source and destination IP address have been received according to [RFC3947], incrementally recompute the TCP/UDP checksum: * Subtract the IP source address in the received packet from the checksum. * Add the real IP source address received via IKE to the checksum (obtained from the NAT-OA) * Subtract the IP destination address in the received packet from the checksum. * Add the real IP destination address received via IKE to the checksum (obtained from the NAT-OA). Note: If the received and real address are the same for a given address (e.g., say the source address), the operations cancel and don't need to be performed. 2. If the protocol header after the ESP header is a TCP/UDP header, recompute the checksum field in the TCP/UDP header. 3. If the protocol header after the ESP header is a UDP header, set the checksum field to zero in the UDP header. If the protocol after the ESP header is a TCP header, and if there is an option to flag to the stack that the TCP checksum does not need to be computed, then that flag MAY be used. (S3.1.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: dropping a transport-mode ESP-in-UDP datagram whose inner TCP/UDP checksum a NAT invalidated, when none of the three alternatives is done. checkNATTTransportInnerChecksum asserts Ze installs transport mode with the espinudp template and that nping --badsum TCP and UDP probes are delivered (checkInnerChecksums true). The tunnel-mode scenario, differing in mode alone, refuses the same probes, proving the corruption is real.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`checkNATTTunnelInnerChecksum`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L1355) | interop/nightly | unproven |
| positive | [`checkNATTTransportInnerChecksum`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L1316) | interop/nightly | unproven |

### [`RFC3948-3.1.2-2`](#rfc3948-3.1.2-2)

Tunnel mode TCP checksums MUST be verified (S3.1.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`checkNATTTransportInnerChecksum`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L1319) | interop/nightly | unproven |
| positive | [`checkNATTTunnelInnerChecksum`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L1352) | interop/nightly | unproven |

### [`RFC3948-2.1-4`](#rfc3948-2.1-4)

receivers MUST NOT depend on the UDP checksum being a zero value (S2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3948-2.1-4, so no unit is bound to it.

### [`RFC3948-4-1`](#rfc3948-4-1)

A peer SHOULD send a NAT-keepalive packet if a need for it is detected according to [RFC3947] and if no other packet to the peer has been sent in M seconds. (S4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden behaviour: not sending a NAT-keepalive when a need is detected (NAT behind this end) and nothing else was sent to the peer for M seconds. TestNATKeepalive proves Keepalive.Run emits a keepalive on its timer and TestKeepaliveDefaultInterval proves the default interval is positive and <= 20s. Neither clause of the condition is asserted: no unit shows keepalives start only when the RFC 3947 need is detected, and none shows the M-second window is measured from the last packet sent to the peer rather than a free-running ticker.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestKeepaliveDefaultInterval`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/keepalive_test.go#L62) | unit/verify | unproven |
| positive | [`TestNATKeepalive`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/keepalive_test.go#L13) | unit/verify | unproven |

### [`RFC3948-1-1`](#rfc3948-1-1)

IPsec tunnel mode clients MUST support tunnel mode (S1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTunnelModeIsTheChildSADefault`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L501) | unit/verify | unproven |
| positive | [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L187) | unit/verify | unproven |
| positive | [`TestTunnelModeIsTheChildSADefault`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L460) | unit/verify | unproven |

### [`RFC3948-2.3-2`](#rfc3948-2.3-2)

The sender MUST use a one-octet-long payload with the value 0xFF. (S2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden behaviour: a NAT-keepalive whose payload is longer than one octet or not 0xFF. TestNATKeepalive runs Keepalive.Run against a real UDP socket and fails unless the datagram read is exactly n == 1 and buf[0] == 0xFF, so both clauses (one octet, value 0xFF) go red on a change of keepaliveByte or of the write length. Single-polarity marker present.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestNATKeepalive`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/keepalive_test.go#L16) | unit/verify | unproven |

### [`RFC3948-4-3`](#rfc3948-4-3)

Reception of NAT-keepalive packets MUST NOT be used to detect whether a connection is live (S4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNATKeepaliveReachesNoSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_test.go#L666) | unit/verify | unproven |
| negative | [`TestNATKeepaliveIsNeverDeliveredToASession`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/keepalive_test.go#L80) | unit/verify | unproven |
| positive | [`TestNATKeepaliveReachesNoSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_test.go#L657) | unit/verify | unproven |

### [`RFC3948-3.1.1-1`](#rfc3948-3.1.1-1)

Depending on local policy, one of the following MUST be done: 1. If a valid source IP address space has been defined in the policy for the encapsulated packets from the peer, check that the source IP address of the inner packet is valid according to the policy. 2. If an address has been assigned for the remote peer, check that the source IP address used in the inner packet is the assigned IP address. 3. NAT is performed for the packet, making it suitable for transport in the local network. (S3.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden behaviour: tunnel-mode decapsulation that takes none of the three actions, here accepting an inner packet whose source lies outside the policy's defined space. Ze takes option 1 through the kernel, so the proof is the selector Ze installs: TestChildInboundPolicyDefinesTheValidInnerSourceSpace asserts the inbound policy is SPActionProtect with Src == the negotiated remote selector 10.2.0.0/24, and the child_test.go unit asserts inPol.Src == NegotiatedTSr; a wider or wrong source selector goes red (discrimination record on childPolicyParams). Options 2 and 3 are alternatives the RFC leaves to local policy. Single-polarity marker present.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSAInboundPolicyUsesNegotiatedTS`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L415) | unit/verify | unproven |
| positive | [`TestChildInboundPolicyDefinesTheValidInnerSourceSpace`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc3948_inner_source_policy_test.go#L28) | unit/verify | revert, verified |

### [`RFC3948-5.1-1`](#rfc3948-5.1-1)

Because SGW will now see two possible SAs that lead to 10.1.2.3, it can become confused about where to send packets coming from Suzy's server. Implementors MUST devise ways of preventing this from occurring. (S5.1)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. {gap} confirmed at the producer: ze devises no way to stop two peers behind one NAT reaching it with the same inner address, because it assigns no inner address. eap.Pool.Allocate (internal/core/eap/pool.go) leases a locally unique address and has no non-test caller; the engine keeps the remote-access pool in s.pool via reloadPool (internal/component/ike/engine/apply.go), whose own comment states nothing reads s.pool yet. No tagged unit exists.

No test carries RFC3948-5.1-1, so no unit is bound to it.

### [`RFC3948-5.2-1`](#rfc3948-5.2-1)

Implementations MUST handle this situation, either by disallowing conflicting connections, or by other means. (S5.2)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. Forbidden behaviour: two transport-mode clients behind one NAT whose traffic descriptions overlap both admitted with nothing deciding which SA carries the traffic. TestPolicyOwnerRefusesASecondPeerOnOneSelector refuses only a second claim on an identical selector (0.0.0.0/0 tunnel, same fields), which is the XFRM policy-identity rule, a neighbouring one. TestPolicyOwnerSeparatesDistinctSelectors (positive) asserts that overlapping descriptions ARE admitted: Dst 192.0.2.0/24 against 0.0.0.0/0, and a port-500 or port-4500 selector against an any-port one, all overlap, so its tag prose ('do NOT overlap') is false and the unit pins the non-compliant behaviour. Transport mode is never exercised.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPolicyOwnerRefusesASecondPeerOnOneSelector`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/policy_owner_test.go#L40) | unit/verify | unproven |
| positive | [`TestPolicyOwnerSeparatesDistinctSelectors`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/policy_owner_test.go#L263) | unit/verify | unproven |

### [`RFC3948-2.3-1`](#rfc3948-2.3-1)

The receiver SHOULD ignore a received NAT-keepalive packet. (S2.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The tagged unit TestIsNATKeepalive proves only classification: IsNATKeepalive answers true for the single byte 0xFF and false for 0xFF 0x00 and 0x00. The obligation is that the receiver ignores the keepalive; the ignoring is the 'continue' in the receive loop of internal/component/ike/engine/register.go, and no tagged unit goes red if that loop processes a keepalive instead of dropping it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIsNATKeepalive`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/nat_test.go#L128) | unit/verify | revert, verified |
| positive | [`TestIsNATKeepalive`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/nat_test.go#L122) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6 phase 2, rfc3948 |
| Signed off | 2026-08-31 |
| Register | rfc2119 |
| Source | rfc/full/rfc3948.txt |
| Source fingerprint | 70b3d9d30da1c716 |
| Record | rfc/extraction/rfc3948.json |
| Mapped sentences | 10 |
| Declined as scope | 5 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of this Memo, Copyright Notice, Abstract and Table of Contents. The Abstract says what the document defines and binds no speaker. |
| `1` | Introduction | 3 | walked | Introduction. Scope, the shared-port rationale, the two mode-support sentences, the zero-SPI ban, the IPv6 note, the exclusion of AH and of manual keying, and the RFC 2119 key-words paragraph. Its three sites are classified below. |
| `2` | Packet Formats | 0 | walked | Packet Formats. A heading with no text of its own; every sentence belongs to 2.1, 2.2 or 2.3. |
| `2.1` | UDP-Encapsulated ESP Header Format | 2 | walked | UDP-Encapsulated ESP Header Format. The wire diagram, the three UDP-header bullets and the zero-SPI ban. The splitter fuses the three bullets into one site, so only the first of them has a site of its own. The other two are listed below. The demultiplexing rule once filed here is RFC 7296 Section 2.23, and lives in that summary. |
| `2.2` | IKE Header Format for Port 4500 | 0 | walked | IKE Header Format for Port 4500. The marker is stated in the indicative, 'A Non-ESP Marker is 4 zero-valued bytes aligning with the SPI field of an ESP packet', so the keyword scan sees no site. The obligation to prepend it is stated by RFC 7296 Section 2.23, whose own row RFC7296-2.23-3 carries it, so no row of this summary claims it. The section states no checksum rule of its own and defers the checksum to RFC 3947. |
| `2.3` | NAT-Keepalive Packet Format | 2 | walked | NAT-Keepalive Packet Format. The diagram, the three UDP-header bullets repeated from 2.1, the sender payload rule and the receiver's SHOULD. The SHOULD sentence carries no MUST-level keyword, so it makes no site and is listed below as RFC3948-2.3-1, which is advisory and gates nothing. |
| `3` | Encapsulation and Decapsulation Procedures | 0 | walked | Encapsulation and Decapsulation Procedures. A heading with no text of its own. |
| `3.1` | Auxiliary Procedures | 0 | walked | Auxiliary Procedures. A heading with no text of its own. |
| `3.1.1` | Tunnel Mode Decapsulation NAT Procedure | 1 | walked | Tunnel Mode Decapsulation NAT Procedure. One MUST opening a three-way choice about the inner packet's addresses. Its site is mapped below to RFC3948-3.1.1-1: ze takes the first choice by defining the valid source address space in the inbound Security Policy it installs, and the kernel checks each decapsulated packet against it. |
| `3.1.2` | Transport Mode Decapsulation NAT Procedure | 2 | walked | Transport Mode Decapsulation NAT Procedure. Two sites, both mapped. The third choice carries a MAY about skipping the TCP checksum, and the section closes with 'an implementation MAY fix any contained protocols'. Both are advisory, make no site, and are listed below as RFC3948-3.1.2-3. |
| `3.2` | Transport Mode ESP Encapsulation | 0 | walked | Transport Mode ESP Encapsulation. A before-and-after diagram and three numbered steps written in the indicative (ordinary ESP, insert the UDP header, edit Total Length, Protocol and Header Checksum). No keyword, and no obligation the mapped sections do not already carry. |
| `3.3` | Transport Mode ESP Decapsulation | 0 | walked | Transport Mode ESP Decapsulation. Four numbered steps in the indicative, the last of which invokes the Section 3.1.2 procedure. No keyword, and no requirement id is read from it. |
| `3.4` | Tunnel Mode ESP Encapsulation | 0 | walked | Tunnel Mode ESP Encapsulation. A before-and-after diagram and three numbered steps in the indicative, the same shape as Section 3.2 with a new outer header. No keyword, and no requirement id is read from it. |
| `3.5` | Tunnel Mode ESP Decapsulation | 0 | walked | Tunnel Mode ESP Decapsulation. Four numbered steps in the indicative, the last of which invokes the Section 3.1.1 procedure. No keyword, and no requirement id is read from it. |
| `4` | NAT Keepalive Procedure | 1 | walked | NAT Keepalive Procedure. One MUST NOT site, mapped below. The sending rules are a MAY over N minutes and a SHOULD over M seconds, with locally configurable defaults of 5 minutes and 20 seconds, neither of which makes a site, so the two interval rows the summary reads from them are listed below: RFC3948-4-2 carries the intervals, and RFC3948-4-1 is read from the section's indicative opening. |
| `5` | Security Considerations | 0 | walked | Security Considerations. A heading with no text of its own. |
| `5.1` | Tunnel Mode Conflict | 1 | walked | Tunnel Mode Conflict. The overlapping-inner-address hazard at a security gateway, one MUST and one RECOMMENDED remedy. Its site is mapped below to RFC3948-5.1-1, which the summary carries as a gap: ze is the security gateway of the diagram and assigns no inner address, so nothing it holds prevents the overlap. |
| `5.2` | Transport Mode Conflict | 2 | walked | Transport Mode Conflict. The same hazard for two transport-mode clients behind one NAT, with a worked policy example. Two sites, both excluded below. |
| `6` | IAB Considerations | 0 | walked | IAB Considerations. One sentence handing the UNSAF questions of RFC 3424 to RFC 3715. It directs no implementation. |
| `7` | Acknowledgments | 0 | skipped (acknowledgements) | Acknowledgments. |
| `8` | References heading | 0 | skipped (references) | References heading. |
| `8.1` | not stated | 0 | skipped (references) | Normative References: RFC 768, RFC 2119, RFC 2401, RFC 2406, RFC 2409, RFC 3947. |
| `8.2` | not stated | 0 | skipped (references) | Informative References: RFC 1122, RFC 3193, RFC 3424, RFC 3715 and the IKEv2 draft. |
| `A` | not stated | 1 | walked | Appendix A, Clarification of Potential NAT Multiple Client Solutions. Non-normative by its own statement: the subject is 'not a matter of wire protocol, but a matter local implementation' and the mechanisms 'do not belong in the protocol specification itself'. It lists implementation options Tr1 to Tr5 and Tn1 to Tn4. It is walked rather than skipped because its one site is classified below. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The modes this sentence mandates are defined by RFC 3193, Securing L2TP using IPsec, which the sentence cites by name. RFC 3948 states none of them, so the obligation is owed against that document rather than against this summary. Ze's L2TP component (internal/component/l2tp) is an LAC/LNS for PPP subscriber access and never runs a session inside IPsec, and no file in the tree implements RFC 3193. | L2TP/IPsec clients MUST support the modes as defined in [RFC3193]. |
| `1:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Introduction restates for the IKE implementation the zero-SPI ban that Section 2.1 states for the packet in its own right, because a zero SPI there is the non-ESP marker. Site 2.1:2 is the sentence RFC3948-2.1-1 maps. | An IKE implementation supporting this protocol specification MUST NOT use the ESP SPI field zero for ESP packets. |
| `2.3:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 2.3 repeats Section 2.1's three UDP-header bullets for the keepalive packet, and its first bullet points back at Section 2.1 by name. The port rule is RFC3948-2.1-2, which site 2.1:1 maps; the checksum pair is RFC3948-2.1-5 and RFC3948-2.1-4, both recorded on section 2.1. | o the Source Port and Destination Port MUST be the same as used by UDP-ESP encapsulation of Section 2.1, o the IPv4 UDP Checksum SHOULD be transmitted as a zero value, and o receivers MUST NOT depend upon the UDP checksum being a zero value. |
| `5.2:2` | `advisory-in-context` (never bound Ze): the sentence advises on applying a rule stated elsewhere and adds no obligation of its own | The splitter cut this MUST NOT away from the sentence that completes it. The enclosing construction is 'For security guarantees, the above problematic scenario MUST NOT be allowed on servers.  For best effort security, this scenario MAY be used.' The pair states an either-or the operator picks between when writing the server's security policy, not an obligation the implementation can meet or fail. | For security guarantees, the above problematic scenario MUST NOT be allowed on servers. |
| `A:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A description of what Sections 5.1 and 5.2 say, reported inside Appendix A. The same paragraph states that the subject is 'not a matter of wire protocol, but a matter local implementation' and that the mechanisms 'do not belong in the protocol specification itself', so the keyword is a report of another section rather than a directive of its own. | Sections 5.1 and 5.2 say that you MUST avoid this problem. |

## Superseded

No document obsoletes RFC 3948, so its obligations are stated where they were written.
