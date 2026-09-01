# RFC 3948 - UDP Encapsulation of IPsec ESP Packets

Partial. Every requirement this repository extracted from RFC 3948, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 61.5% | 8 of 13 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 30.8% | 4 of 13 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 13 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 3.8% | 1 of 26 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 14 | of 18 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 14 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 7.7% | 1 of 13 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 13 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 18 |
| Gated MUST-level | 14 |
| Obligations that bind Ze | 13 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 1 |
| Gated with no test | 0 |
| Nightly-only evidence | 2 |
| Test tags | 26 |
| Tagged units | 26 |
| Recorded audit verdicts | 0 |
| Discrimination records | 1 |
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

Section 5.1 tunnel mode conflict (`RFC3948-5.1-1`) is a gap. Ze assigns no inner address to a remote peer, so it devises no way of preventing two peers behind one NAT from reaching it with the same self-chosen inner address. The section's RECOMMENDED remedy is a locally unique address per peer, and the allocator for it is written and unreached: `Pool.Allocate` ([`internal/component/ike/eap/pool.go`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/pool.go)) has no non-test caller, and `registerIKE` ([`internal/component/ike/engine/register.go`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/register.go)) discards the pool it builds. No engine code constructs a Configuration payload, so ze sends no CFG_REPLY. Closing it is [`plan/spec-ike-virtual-ip-assignment.md`](https://github.com/ze-software/ze/blob/main/plan/spec-ike-virtual-ip-assignment.md).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 8 | one part of the gated population |
| Annotated instead of tested | 6 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 2 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **14** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (8):** [`RFC3948-2.1-1`](#rfc3948-2.1-1), [`RFC3948-3.1.2-1`](#rfc3948-3.1.2-1), [`RFC3948-3.1.2-2`](#rfc3948-3.1.2-2), [`RFC3948-2.2-1`](#rfc3948-2.2-1), [`RFC3948-2.1-3`](#rfc3948-2.1-3), [`RFC3948-1-1`](#rfc3948-1-1), [`RFC3948-4-3`](#rfc3948-4-3), [`RFC3948-5.2-1`](#rfc3948-5.2-1)

**Annotated instead of tested (6):** [`RFC3948-2.1-2`](#rfc3948-2.1-2), [`RFC3948-2.1-4`](#rfc3948-2.1-4), [`RFC3948-4-1`](#rfc3948-4-1), [`RFC3948-2.3-2`](#rfc3948-2.3-2), [`RFC3948-3.1.1-1`](#rfc3948-3.1.1-1), [`RFC3948-5.1-1`](#rfc3948-5.1-1)

**Evidence that runs nightly only (2):** [`RFC3948-3.1.2-1`](#rfc3948-3.1.2-1), [`RFC3948-3.1.2-2`](#rfc3948-3.1.2-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3948-2.1-1` | ESP SPI MUST NOT be zero (zero is reserved for the Non-ESP Marker to distinguish IKE from ESP on port 4500) (S2.1) | MUST NOT | 2.1 - UDP-Encapsulated ESP Header Format | **positive:** `unit/verify` [`TestGenerateESPSPI`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L240). **negative:** `unit/verify` [`TestGenerateESPSPI`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L254) |
| `RFC3948-2.1-2` | Source and destination ports MUST match the ports used by IKE (port 4500 after port float) (S2.1) | MUST | 2.1 - UDP-Encapsulated ESP Header Format | **positive:** `unit/verify` [`TestChildSANATTEncapPorts`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L273). **negative:** no negative test. **{single-polarity}:** the Child SA install unconditionally sets both UDP-encap ports to the fixed IKE NAT-T port 4500 (internal/component/ike/engine/child.go:237-238,265-266); no ze code path produces a non-4500 encap port, so there is no mismatched-port case to reject |
| `RFC3948-3.1.2-1` | Transport mode decapsulation MUST do one of three things with the inner TCP/UDP checksum: recompute it incrementally from the addresses received via IKE, recompute it in full, or zero a UDP checksum and flag the stack that a TCP one need not be computed (S3.1.2) | MUST | 3.1.2 - Transport Mode Decapsulation NAT Procedure | **positive:** `interop/nightly` [`checkNATTTransportInnerChecksum`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L1078). **negative:** `interop/nightly` [`checkNATTTunnelInnerChecksum`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L1117). **nightly-only:** every test bound to this requirement runs in the scheduled workflow alone, so nothing here is proven on the merge path |
| `RFC3948-3.1.2-2` | Tunnel mode TCP checksums MUST be verified (S3.1.2) | MUST | 3.1.2 - Transport Mode Decapsulation NAT Procedure | **positive:** `interop/nightly` [`checkNATTTunnelInnerChecksum`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L1114). **negative:** `interop/nightly` [`checkNATTTransportInnerChecksum`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L1081). **nightly-only:** every test bound to this requirement runs in the scheduled workflow alone, so nothing here is proven on the merge path |
| `RFC3948-2.2-1` | Non-ESP Marker (4 zero bytes) MUST be prepended to IKE packets on port 4500 (S2.2) | MUST | 2.2 - IKE Header Format for Port 4500 | **positive:** `unit/verify` [`TestNonESPMarker`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/nat_test.go#L83). **negative:** `unit/verify` [`TestNonESPMarkerESPPacket`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/nat_test.go#L115) |
| `RFC3948-2.1-3` | Receiver MUST demultiplex by inspecting first 4 bytes after UDP header: zero = IKE, non-zero = ESP, 1-byte 0xFF = keepalive (S2.1, S2.2, S2.3) | MUST | 2.1 - UDP-Encapsulated ESP Header Format | **positive:** `unit/verify` [`TestIsNATKeepalive`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/nat_test.go#L128). **positive:** `unit/verify` [`TestNonESPMarker`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/nat_test.go#L86). **negative:** `unit/verify` [`TestIsNATKeepalive`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/nat_test.go#L134). **negative:** `unit/verify` [`TestNonESPMarkerESPPacket`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/nat_test.go#L118) |
| `RFC3948-2.1-4` | Receivers MUST NOT depend on the UDP checksum being zero (S2.1) | MUST NOT | 2.1 - UDP-Encapsulated ESP Header Format | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze reads IKE/ESP demux from the OS UDP socket (internal/component/ike/engine/register.go:421) and never inspects the UDP checksum, so no ze code path can depend on it; ESP-in-UDP checksum handling is the kernel's |
| `RFC3948-4-1` | Keepalive interval MUST be shorter than the NAT binding timeout (S4) | MUST | 4 - NAT Keepalive Procedure | **positive:** `unit/verify` [`TestKeepaliveDefaultInterval`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/keepalive_test.go#L67). **positive:** `unit/verify` [`TestNATKeepalive`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/keepalive_test.go#L13). **negative:** no negative test. **{single-polarity}:** the keepalive interval is a fixed conservative 20s constant (internal/component/ike/transport/keepalive.go:13), well under a typical NAT UDP binding lifetime; there is no dynamic binding-timeout query to drive a negative |
| `RFC3948-1-1` | IPsec tunnel mode clients MUST support tunnel mode (S1) | MUST | 1 - Introduction | **positive:** `unit/verify` [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L180). **positive:** `unit/verify` [`TestTunnelModeIsTheChildSADefault`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L453). **negative:** `unit/verify` [`TestTunnelModeIsTheChildSADefault`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L494) |
| `RFC3948-2.3-2` | The NAT-keepalive sender MUST use a one-octet-long payload with the value 0xFF (S2.3) | MUST | 2.3 - NAT-Keepalive Packet Format | **positive:** `unit/verify` [`TestNATKeepalive`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/keepalive_test.go#L16). **negative:** no negative test. **{single-polarity}:** the keepalive payload is a fixed one-octet constant (internal/component/ike/transport/keepalive.go:14,48) that no input can vary, so there is no non-conforming sender case to drive |
| `RFC3948-4-3` | Reception of NAT-keepalive packets MUST NOT be used to detect whether a connection is live (S4) | MUST NOT | 4 - NAT Keepalive Procedure | **positive:** `unit/verify` [`TestNATKeepaliveReachesNoSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_test.go#L655). **negative:** `unit/verify` [`TestNATKeepaliveIsNeverDeliveredToASession`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/keepalive_test.go#L85). **negative:** `unit/verify` [`TestNATKeepaliveReachesNoSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_test.go#L664) |
| `RFC3948-3.1.1-1` | Tunnel mode decapsulation, depending on local policy, MUST do one of three things: check the inner source address against the policy, check it against the address assigned to the peer, or NAT the packet (S3.1.1) | MUST | 3.1.1 - Tunnel Mode Decapsulation NAT Procedure | **positive:** `unit/verify` [`TestChildInboundPolicyDefinesTheValidInnerSourceSpace`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc3948_inner_source_policy_test.go#L28). **positive:** `unit/verify` [`TestChildSAInboundPolicyUsesNegotiatedTS`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L408). **negative:** no negative test. **{single-polarity}:** ze takes the first option by installing the inbound policy with the negotiated remote traffic selector as its source selector (childPolicyParams, internal/component/ike/engine/child.go), and the drop of a mismatched inner packet is the kernel's, so ze holds no rejecting branch |
| `RFC3948-5.1-1` | Implementors MUST devise ways of preventing two remote peers from reaching one security gateway with overlapping inner addresses (S5.1) | MUST | 5.1 - Tunnel Mode Conflict | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze devises no such way, because it assigns no inner address at all. The section's RECOMMENDED remedy is to give each remote peer a locally unique address, and the allocator for it exists and is unreached: Pool.Allocate (internal/component/ike/eap/pool.go) leases a unique address and refuses on exhaustion with ErrPoolExhausted, yet its only callers are pool_test.go and pool_release_test.go. registerIKE (internal/component/ike/engine/register.go) builds the pool from the remote-access config and then discards it at a bare `_ = ipPool`, so no lease ever reaches a peer. No engine code constructs a wire.PayloadCP either, so ze sends no CFG_REPLY and a client keeps whatever inner address it chose for itself. Two peers behind one NAT that both chose 10.1.2.3 would therefore present the gateway with the ambiguity the section names, and ze holds nothing that prevents it. Closing this is plan/spec-ike-virtual-ip-assignment.md |
| `RFC3948-5.2-1` | Implementations MUST handle conflicting connections from clients behind one NAT, either by disallowing conflicting connections or by other means (S5.2) | MUST | 5.2 - Transport Mode Conflict | **positive:** `unit/verify` [`TestPolicyOwnerSeparatesDistinctSelectors`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/policy_owner_test.go#L263). **negative:** `unit/verify` [`TestPolicyOwnerRefusesASecondPeerOnOneSelector`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/policy_owner_test.go#L40) |
| `RFC3948-2.1-5` | IPv4 UDP checksum SHOULD be zero on transmit (S2.1) | SHOULD | 2.1 - UDP-Encapsulated ESP Header Format | **positive:** no positive test. **negative:** no negative test |
| `RFC3948-2.3-1` | NAT-keepalive packets (1 byte 0xFF) SHOULD be ignored by the receiver (S2.3) | SHOULD | 2.3 - NAT-Keepalive Packet Format | **positive:** no positive test. **negative:** no negative test |
| `RFC3948-3.1.2-3` | TCP checksum verification MAY be skipped for transport mode only if the packet is integrity-protected (S3.1.2) | MAY | 3.1.2 - Transport Mode Decapsulation NAT Procedure | **positive:** no positive test. **negative:** no negative test |
| `RFC3948-4-2` | Keepalive interval is implementation-specific; typical 20-30 seconds (S4) | MAY | 4 - NAT Keepalive Procedure | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC3948-2.1-4`](#rfc3948-2.1-4) Receivers MUST NOT depend on the UDP checksum being zero (S2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze reads IKE/ESP demux from the OS UDP socket (internal/component/ike/engine/register.go:421) and never inspects the UDP checksum, so no ze code path can depend on it; ESP-in-UDP checksum handling is the kernel's |
| [`RFC3948-5.1-1`](#rfc3948-5.1-1) Implementors MUST devise ways of preventing two remote peers from reaching one security gateway with overlapping inner addresses (S5.1) | {gap}, no test | ze devises no such way, because it assigns no inner address at all. The section's RECOMMENDED remedy is to give each remote peer a locally unique address, and the allocator for it exists and is unreached: Pool.Allocate (internal/component/ike/eap/pool.go) leases a unique address and refuses on exhaustion with ErrPoolExhausted, yet its only callers are pool_test.go and pool_release_test.go. registerIKE (internal/component/ike/engine/register.go) builds the pool from the remote-access config and then discards it at a bare `_ = ipPool`, so no lease ever reaches a peer. No engine code constructs a wire.PayloadCP either, so ze sends no CFG_REPLY and a client keeps whatever inner address it chose for itself. Two peers behind one NAT that both chose 10.1.2.3 would therefore present the gateway with the ambiguity the section names, and ze holds nothing that prevents it. Closing this is plan/spec-ike-virtual-ip-assignment.md |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC3948-2.1-1`](#rfc3948-2.1-1)

ESP SPI MUST NOT be zero (zero is reserved for the Non-ESP Marker to distinguish IKE from ESP on port 4500) (S2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGenerateESPSPI`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L254) | unit/verify | unproven |
| positive | [`TestGenerateESPSPI`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L240) | unit/verify | unproven |

### [`RFC3948-2.1-2`](#rfc3948-2.1-2)

Source and destination ports MUST match the ports used by IKE (port 4500 after port float) (S2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSANATTEncapPorts`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L273) | unit/verify | unproven |

### [`RFC3948-3.1.2-1`](#rfc3948-3.1.2-1)

Transport mode decapsulation MUST do one of three things with the inner TCP/UDP checksum: recompute it incrementally from the addresses received via IKE, recompute it in full, or zero a UDP checksum and flag the stack that a TCP one need not be computed (S3.1.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`checkNATTTunnelInnerChecksum`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L1117) | interop/nightly | unproven |
| positive | [`checkNATTTransportInnerChecksum`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L1078) | interop/nightly | unproven |

### [`RFC3948-3.1.2-2`](#rfc3948-3.1.2-2)

Tunnel mode TCP checksums MUST be verified (S3.1.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`checkNATTTransportInnerChecksum`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L1081) | interop/nightly | unproven |
| positive | [`checkNATTTunnelInnerChecksum`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L1114) | interop/nightly | unproven |

### [`RFC3948-2.2-1`](#rfc3948-2.2-1)

Non-ESP Marker (4 zero bytes) MUST be prepended to IKE packets on port 4500 (S2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNonESPMarkerESPPacket`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/nat_test.go#L115) | unit/verify | unproven |
| positive | [`TestNonESPMarker`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/nat_test.go#L83) | unit/verify | unproven |

### [`RFC3948-2.1-3`](#rfc3948-2.1-3)

Receiver MUST demultiplex by inspecting first 4 bytes after UDP header: zero = IKE, non-zero = ESP, 1-byte 0xFF = keepalive (S2.1, S2.2, S2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIsNATKeepalive`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/nat_test.go#L134) | unit/verify | unproven |
| negative | [`TestNonESPMarkerESPPacket`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/nat_test.go#L118) | unit/verify | unproven |
| positive | [`TestIsNATKeepalive`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/nat_test.go#L128) | unit/verify | unproven |
| positive | [`TestNonESPMarker`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/nat_test.go#L86) | unit/verify | unproven |

### [`RFC3948-2.1-4`](#rfc3948-2.1-4)

Receivers MUST NOT depend on the UDP checksum being zero (S2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3948-2.1-4, so no unit is bound to it.

### [`RFC3948-4-1`](#rfc3948-4-1)

Keepalive interval MUST be shorter than the NAT binding timeout (S4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestKeepaliveDefaultInterval`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/keepalive_test.go#L67) | unit/verify | unproven |
| positive | [`TestNATKeepalive`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/keepalive_test.go#L13) | unit/verify | unproven |

### [`RFC3948-1-1`](#rfc3948-1-1)

IPsec tunnel mode clients MUST support tunnel mode (S1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTunnelModeIsTheChildSADefault`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L494) | unit/verify | unproven |
| positive | [`TestChildSAInstallsInDataplane`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L180) | unit/verify | unproven |
| positive | [`TestTunnelModeIsTheChildSADefault`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L453) | unit/verify | unproven |

### [`RFC3948-2.3-2`](#rfc3948-2.3-2)

The NAT-keepalive sender MUST use a one-octet-long payload with the value 0xFF (S2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestNATKeepalive`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/keepalive_test.go#L16) | unit/verify | unproven |

### [`RFC3948-4-3`](#rfc3948-4-3)

Reception of NAT-keepalive packets MUST NOT be used to detect whether a connection is live (S4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNATKeepaliveReachesNoSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_test.go#L664) | unit/verify | unproven |
| negative | [`TestNATKeepaliveIsNeverDeliveredToASession`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/keepalive_test.go#L85) | unit/verify | unproven |
| positive | [`TestNATKeepaliveReachesNoSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_test.go#L655) | unit/verify | unproven |

### [`RFC3948-3.1.1-1`](#rfc3948-3.1.1-1)

Tunnel mode decapsulation, depending on local policy, MUST do one of three things: check the inner source address against the policy, check it against the address assigned to the peer, or NAT the packet (S3.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSAInboundPolicyUsesNegotiatedTS`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L408) | unit/verify | unproven |
| positive | [`TestChildInboundPolicyDefinesTheValidInnerSourceSpace`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc3948_inner_source_policy_test.go#L28) | unit/verify | revert, verified |

### [`RFC3948-5.1-1`](#rfc3948-5.1-1)

Implementors MUST devise ways of preventing two remote peers from reaching one security gateway with overlapping inner addresses (S5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3948-5.1-1, so no unit is bound to it.

### [`RFC3948-5.2-1`](#rfc3948-5.2-1)

Implementations MUST handle conflicting connections from clients behind one NAT, either by disallowing conflicting connections or by other means (S5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPolicyOwnerRefusesASecondPeerOnOneSelector`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/policy_owner_test.go#L40) | unit/verify | unproven |
| positive | [`TestPolicyOwnerSeparatesDistinctSelectors`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/policy_owner_test.go#L263) | unit/verify | unproven |

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
| `2.1` | UDP-Encapsulated ESP Header Format | 2 | walked | UDP-Encapsulated ESP Header Format. The wire diagram, the three UDP-header bullets and the zero-SPI ban. The splitter fuses the three bullets into one site, so only the first of them has a site of its own. The other two, and the demultiplexing rule the summary reads across 2.1, 2.2 and 2.3 together, are listed below. |
| `2.2` | IKE Header Format for Port 4500 | 0 | walked | IKE Header Format for Port 4500. The marker is stated in the indicative, 'A Non-ESP Marker is 4 zero-valued bytes aligning with the SPI field of an ESP packet', so the keyword scan sees no site. The obligation to prepend it is real and is listed below. The section states no checksum rule of its own and defers the checksum to RFC 3947. |
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
