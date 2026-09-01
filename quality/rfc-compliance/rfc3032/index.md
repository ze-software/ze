# RFC 3032 - MPLS Label Stack Encoding

Supported as dependency. Every requirement this repository extracted from RFC 3032, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 0 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 0 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 0 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 0 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 17 | of 23 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 17 | of 17 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

No card above is a share of a population, so there is nothing to add up.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Supported as dependency |
| Enrolment | Enrolled |
| Requirements | 23 |
| Gated MUST-level | 17 |
| Obligations that bind Ze | 0 |
| Not applicable, so out of scope | 17 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc3032.md` |
| Requirement shard | `rfc/requirements/rfc3032.md` |
| RFC text | `rfc/full/rfc3032.txt` |

## Enrolment

Enrolled: MPLS Label Stack Encoding (RFC 3032): all 17 gated MUSTs not-applicable -- data-plane (TTL processing, S-bit-on-wire, fragmentation, MTU, ICMP, disposition) is kernel AF_MPLS / VPP; ze is an MPLS control plane (encodes label values, validates, programs the FIB). Same pattern as rfc3031/rfc4364

## What the public ledger says

**Status:** Supported as dependency

**What the ledger says is covered:**

20-bit label stack encoding and validation used by labeled unicast and VPN NLRI.

**What the ledger says remains:**

No tracked gap in current source anchors.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 17 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **17** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (17):** [`RFC3032-1-1`](#rfc3032-1-1), [`RFC3032-2.2-1`](#rfc3032-2.2-1), [`RFC3032-2.2-2`](#rfc3032-2.2-2), [`RFC3032-2.2-3`](#rfc3032-2.2-3), [`RFC3032-2.4.2-1`](#rfc3032-2.4.2-1), [`RFC3032-2.4.2-2`](#rfc3032-2.4.2-2), [`RFC3032-2.4.2-3`](#rfc3032-2.4.2-3), [`RFC3032-2.4.3-1`](#rfc3032-2.4.3-1), [`RFC3032-3.3-1`](#rfc3032-3.3-1), [`RFC3032-3.3-2`](#rfc3032-3.3-2), [`RFC3032-3.4-1`](#rfc3032-3.4-1), [`RFC3032-3.4-2`](#rfc3032-3.4-2), [`RFC3032-3.4-3`](#rfc3032-3.4-3), [`RFC3032-3.5-1`](#rfc3032-3.5-1), [`RFC3032-3.5-2`](#rfc3032-3.5-2), [`RFC3032-3.6-1`](#rfc3032-3.6-1), [`RFC3032-3.6-2`](#rfc3032-3.6-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3032-1-1` | When top labels use different encoding (e.g., ATM), this encoding MUST be used for additional label stack entries (S1) | MUST | 1 - Introduction | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no ATM/Frame-Relay MPLS top-label path so the condition never arises, and the on-wire shim for additional entries is built by the kernel/VPP dataplane; ze's only shim encoder is the 3-octet BGP NLRI (internal/core/bgp/nlri/helpers.go:61), which carries no TTL |
| `RFC3032-2.2-1` | Network layer protocol MUST be inferable from the label value at bottom of stack and/or inspection of the network layer header (S2.2) | MUST | 2.2 - Determining the Network Layer Protocol | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** disposition-time protocol identification when the stack empties is performed by the kernel AF_MPLS route (loopback re-injection into the IP path) or VPP dataplane, not by any ze control-plane function (internal/plugins/fib/kernel/mplsentry_linux.go:44) |
| `RFC3032-2.2-2` | When the first label is pushed, it MUST be used ONLY for packets of a particular network layer, OR ONLY for a specified set distinguishable by header inspection (S2.2) | MUST | 2.2 - Determining the Network Layer Protocol | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze binds every label to a single FEC within one address family by construction, but the operative used-only-for-one-protocol forwarding disposition is realized by the kernel's per-in-label AF_MPLS entry, with no dedicated ze guard (internal/plugins/fib/kernel/mplsentry.go:69) |
| `RFC3032-2.2-3` | If a packet cannot be forwarded and its network layer protocol cannot be identified or no protocol-dependent error rules exist, the packet MUST be silently discarded (S2.2) | MUST | 2.2 - Determining the Network Layer Protocol | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the discard-on-unidentifiable-protocol decision is a forwarding-plane action of the kernel/VPP MPLS datapath; ze forwards no MPLS packets in-process |
| `RFC3032-2.4.2-1` | If outgoing TTL is 0, the labeled packet MUST NOT be further forwarded (S2.4.2) | MUST | 2.4.2 - Protocol-independent rules | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** TTL decrement and TTL-zero discard are per-packet forwarding operations performed by the kernel AF_MPLS path or VPP dataplane; no TTL logic exists in any ze MPLS Go path (internal/plugins/fib/kernel/) |
| `RFC3032-2.4.2-2` | When TTL=0, the label stack MUST NOT be stripped off and the packet forwarded as unlabeled (S2.4.2) | MUST NOT | 2.4.2 - Protocol-independent rules | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the TTL-expiry no-strip-and-forward decision is the same forwarding action executed by the kernel/VPP dataplane, not by ze's control plane |
| `RFC3032-2.4.2-3` | When forwarding, the TTL field of the top label stack entry MUST be set to the outgoing TTL value (S2.4.2) | MUST | 2.4.2 - Protocol-independent rules | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** rewriting the shim TTL on a swapped/forwarded packet is a dataplane write done by the kernel/VPP; ze passes only label values (and a static VPP route TTL), never per-packet TTL (internal/plugins/fib/vpp/mpls.go:75) |
| `RFC3032-2.4.3-1` | When an IP packet is first labeled, the label TTL field MUST be set to the value of the IP TTL field (S2.4.3) | MUST | 2.4.3 - IP-dependent rules | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** copying IP TTL into the imposed shim at ingress is a dataplane operation of the kernel/VPP push path; ze's push programming carries no TTL propagation (internal/plugins/fib/kernel/mplsentry.go:88) |
| `RFC3032-3.3-1` | A labeled packet that is not "too big" MUST be transmitted without fragmentation (S3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** MTU comparison and non-fragmentation of a not-too-big labeled packet are forwarding-plane behaviors of the kernel/VPP; ze has no labeled-packet transmit path |
| `RFC3032-3.3-2` | A labeled IP datagram whose size exceeds the True Maximum Frame Payload Size MUST be considered "too big" (S3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the too-big MTU determination on forwarded labeled datagrams is a dataplane classification made by the kernel/VPP, not by ze |
| `RFC3032-3.4-1` | If a labeled IPv4 datagram is too big and has the DF bit set, the LSR MUST execute the strip/fragment/ICMP algorithm (S3.4) | MUST | 3.4 - Processing Labeled IPv4 Datagrams which are Too Big | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** strip-labels/fragment/emit-ICMP for a too-big DF-set IPv4 datagram is entirely the kernel/VPP IPv4 forwarding path; ze neither fragments nor originates ICMP for forwarded packets |
| `RFC3032-3.4-2` | Each IPv4 fragment MUST be at least N bytes less than the Effective Maximum Frame Payload Size (S3.4) | MUST | 3.4 - Processing Labeled IPv4 Datagrams which are Too Big | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** fragment sizing during labeled-packet forwarding is a dataplane computation performed by the kernel/VPP; ze has no fragmentation code |
| `RFC3032-3.4-3` | If the DF bit is set and packet is too big, the datagram MUST NOT be forwarded (S3.4) | MUST NOT | 3.4 - Processing Labeled IPv4 Datagrams which are Too Big | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the DF-set too-big drop decision is a forwarding-plane action of the kernel/VPP datapath |
| `RFC3032-3.5-1` | To process a labeled IPv6 datagram that is too big, the LSR MUST execute the specified algorithm (S3.5) | MUST | 3.5 - Processing Labeled IPv6 Datagrams which are Too Big | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the IPv6 too-big strip/ICMP-Packet-Too-Big/fragment algorithm is the kernel/VPP IPv6 forwarding path; ze runs no such path |
| `RFC3032-3.5-2` | Each IPv6 fragment MUST be at least N bytes less than the Effective Maximum Frame Payload Size (S3.5) | MUST | 3.5 - Processing Labeled IPv6 Datagrams which are Too Big | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** IPv6 fragment sizing during forwarding is a dataplane computation of the kernel/VPP, absent from ze |
| `RFC3032-3.6-1` | The tunnel transmitting endpoint MUST be able to determine the MTU of the tunnel as a whole (S3.6) | MUST | 3.6 - Implications with respect to Path MTU Discovery | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** LSP-tunnel MTU/PMTU determination is a forwarding-plane concern of the kernel/VPP tunnel ingress; ze's RSVP-TE/LDP signaling sets up the LSP but runs no in-process tunnel MTU discovery |
| `RFC3032-3.6-2` | The tunnel transmitting endpoint MUST send ICMP Destination Unreachable when a DF-set packet exceeds tunnel MTU (S3.6) | MUST | 3.6 - Implications with respect to Path MTU Discovery | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** generating ICMP Destination Unreachable for oversized DF-set packets entering a tunnel is a kernel/VPP forwarding-plane action; ze originates no such ICMP |
| `RFC3032-2.4.3-2` | When the last label is popped (stack empty), the IP TTL field SHOULD be replaced with the outgoing TTL value (S2.4.3) | SHOULD | 2.4.3 - IP-dependent rules | **positive:** no positive test. **negative:** no negative test |
| `RFC3032-3.2-1` | LSR SHOULD support a "Maximum Initially Labeled IP Datagram Size" configuration parameter (S3.2) | SHOULD | 3.2 - Maximum Initially Labeled IP Datagram Size | **positive:** no positive test. **negative:** no negative test |
| `RFC3032-2.4.2-4` | When outgoing TTL is 0, the packet MAY be simply discarded or passed to the network layer for error processing depending on the label value (S2.4.2) | MAY | 2.4.2 - Protocol-independent rules | **positive:** no positive test. **negative:** no negative test |
| `RFC3032-3.3-3` | A labeled IP datagram exceeding the Conventional Maximum Frame Payload Size MAY be considered "too big" (S3.3) | MAY | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3032-3.4-4` | If a labeled IPv4 datagram is too big and DF is not set, the LSR MAY silently discard it (S3.4) | MAY | 3.4 - Processing Labeled IPv4 Datagrams which are Too Big | **positive:** no positive test. **negative:** no negative test |
| `RFC3032-3.6-3` | The tunnel endpoint MAY determine tunnel MTU by sending packets and performing Path MTU Discovery (S3.6) | MAY | 3.6 - Implications with respect to Path MTU Discovery | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC3032-1-1`](#rfc3032-1-1) When top labels use different encoding (e.g., ATM), this encoding MUST be used for additional label stack entries (S1) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no ATM/Frame-Relay MPLS top-label path so the condition never arises, and the on-wire shim for additional entries is built by the kernel/VPP dataplane; ze's only shim encoder is the 3-octet BGP NLRI (internal/core/bgp/nlri/helpers.go:61), which carries no TTL |
| [`RFC3032-2.2-1`](#rfc3032-2.2-1) Network layer protocol MUST be inferable from the label value at bottom of stack and/or inspection of the network layer header (S2.2) | no test | no test carries this requirement id; annotated {not-applicable}: disposition-time protocol identification when the stack empties is performed by the kernel AF_MPLS route (loopback re-injection into the IP path) or VPP dataplane, not by any ze control-plane function (internal/plugins/fib/kernel/mplsentry_linux.go:44) |
| [`RFC3032-2.2-2`](#rfc3032-2.2-2) When the first label is pushed, it MUST be used ONLY for packets of a particular network layer, OR ONLY for a specified set distinguishable by header inspection (S2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze binds every label to a single FEC within one address family by construction, but the operative used-only-for-one-protocol forwarding disposition is realized by the kernel's per-in-label AF_MPLS entry, with no dedicated ze guard (internal/plugins/fib/kernel/mplsentry.go:69) |
| [`RFC3032-2.2-3`](#rfc3032-2.2-3) If a packet cannot be forwarded and its network layer protocol cannot be identified or no protocol-dependent error rules exist, the packet MUST be silently discarded (S2.2) | no test | no test carries this requirement id; annotated {not-applicable}: the discard-on-unidentifiable-protocol decision is a forwarding-plane action of the kernel/VPP MPLS datapath; ze forwards no MPLS packets in-process |
| [`RFC3032-2.4.2-1`](#rfc3032-2.4.2-1) If outgoing TTL is 0, the labeled packet MUST NOT be further forwarded (S2.4.2) | no test | no test carries this requirement id; annotated {not-applicable}: TTL decrement and TTL-zero discard are per-packet forwarding operations performed by the kernel AF_MPLS path or VPP dataplane; no TTL logic exists in any ze MPLS Go path (internal/plugins/fib/kernel/) |
| [`RFC3032-2.4.2-2`](#rfc3032-2.4.2-2) When TTL=0, the label stack MUST NOT be stripped off and the packet forwarded as unlabeled (S2.4.2) | no test | no test carries this requirement id; annotated {not-applicable}: the TTL-expiry no-strip-and-forward decision is the same forwarding action executed by the kernel/VPP dataplane, not by ze's control plane |
| [`RFC3032-2.4.2-3`](#rfc3032-2.4.2-3) When forwarding, the TTL field of the top label stack entry MUST be set to the outgoing TTL value (S2.4.2) | no test | no test carries this requirement id; annotated {not-applicable}: rewriting the shim TTL on a swapped/forwarded packet is a dataplane write done by the kernel/VPP; ze passes only label values (and a static VPP route TTL), never per-packet TTL (internal/plugins/fib/vpp/mpls.go:75) |
| [`RFC3032-2.4.3-1`](#rfc3032-2.4.3-1) When an IP packet is first labeled, the label TTL field MUST be set to the value of the IP TTL field (S2.4.3) | no test | no test carries this requirement id; annotated {not-applicable}: copying IP TTL into the imposed shim at ingress is a dataplane operation of the kernel/VPP push path; ze's push programming carries no TTL propagation (internal/plugins/fib/kernel/mplsentry.go:88) |
| [`RFC3032-3.3-1`](#rfc3032-3.3-1) A labeled packet that is not "too big" MUST be transmitted without fragmentation (S3.3) | no test | no test carries this requirement id; annotated {not-applicable}: MTU comparison and non-fragmentation of a not-too-big labeled packet are forwarding-plane behaviors of the kernel/VPP; ze has no labeled-packet transmit path |
| [`RFC3032-3.3-2`](#rfc3032-3.3-2) A labeled IP datagram whose size exceeds the True Maximum Frame Payload Size MUST be considered "too big" (S3.3) | no test | no test carries this requirement id; annotated {not-applicable}: the too-big MTU determination on forwarded labeled datagrams is a dataplane classification made by the kernel/VPP, not by ze |
| [`RFC3032-3.4-1`](#rfc3032-3.4-1) If a labeled IPv4 datagram is too big and has the DF bit set, the LSR MUST execute the strip/fragment/ICMP algorithm (S3.4) | no test | no test carries this requirement id; annotated {not-applicable}: strip-labels/fragment/emit-ICMP for a too-big DF-set IPv4 datagram is entirely the kernel/VPP IPv4 forwarding path; ze neither fragments nor originates ICMP for forwarded packets |
| [`RFC3032-3.4-2`](#rfc3032-3.4-2) Each IPv4 fragment MUST be at least N bytes less than the Effective Maximum Frame Payload Size (S3.4) | no test | no test carries this requirement id; annotated {not-applicable}: fragment sizing during labeled-packet forwarding is a dataplane computation performed by the kernel/VPP; ze has no fragmentation code |
| [`RFC3032-3.4-3`](#rfc3032-3.4-3) If the DF bit is set and packet is too big, the datagram MUST NOT be forwarded (S3.4) | no test | no test carries this requirement id; annotated {not-applicable}: the DF-set too-big drop decision is a forwarding-plane action of the kernel/VPP datapath |
| [`RFC3032-3.5-1`](#rfc3032-3.5-1) To process a labeled IPv6 datagram that is too big, the LSR MUST execute the specified algorithm (S3.5) | no test | no test carries this requirement id; annotated {not-applicable}: the IPv6 too-big strip/ICMP-Packet-Too-Big/fragment algorithm is the kernel/VPP IPv6 forwarding path; ze runs no such path |
| [`RFC3032-3.5-2`](#rfc3032-3.5-2) Each IPv6 fragment MUST be at least N bytes less than the Effective Maximum Frame Payload Size (S3.5) | no test | no test carries this requirement id; annotated {not-applicable}: IPv6 fragment sizing during forwarding is a dataplane computation of the kernel/VPP, absent from ze |
| [`RFC3032-3.6-1`](#rfc3032-3.6-1) The tunnel transmitting endpoint MUST be able to determine the MTU of the tunnel as a whole (S3.6) | no test | no test carries this requirement id; annotated {not-applicable}: LSP-tunnel MTU/PMTU determination is a forwarding-plane concern of the kernel/VPP tunnel ingress; ze's RSVP-TE/LDP signaling sets up the LSP but runs no in-process tunnel MTU discovery |
| [`RFC3032-3.6-2`](#rfc3032-3.6-2) The tunnel transmitting endpoint MUST send ICMP Destination Unreachable when a DF-set packet exceeds tunnel MTU (S3.6) | no test | no test carries this requirement id; annotated {not-applicable}: generating ICMP Destination Unreachable for oversized DF-set packets entering a tunnel is a kernel/VPP forwarding-plane action; ze originates no such ICMP |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC3032-1-1`](#rfc3032-1-1)

When top labels use different encoding (e.g., ATM), this encoding MUST be used for additional label stack entries (S1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3032-1-1, so no unit is bound to it.

### [`RFC3032-2.2-1`](#rfc3032-2.2-1)

Network layer protocol MUST be inferable from the label value at bottom of stack and/or inspection of the network layer header (S2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3032-2.2-1, so no unit is bound to it.

### [`RFC3032-2.2-2`](#rfc3032-2.2-2)

When the first label is pushed, it MUST be used ONLY for packets of a particular network layer, OR ONLY for a specified set distinguishable by header inspection (S2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3032-2.2-2, so no unit is bound to it.

### [`RFC3032-2.2-3`](#rfc3032-2.2-3)

If a packet cannot be forwarded and its network layer protocol cannot be identified or no protocol-dependent error rules exist, the packet MUST be silently discarded (S2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3032-2.2-3, so no unit is bound to it.

### [`RFC3032-2.4.2-1`](#rfc3032-2.4.2-1)

If outgoing TTL is 0, the labeled packet MUST NOT be further forwarded (S2.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3032-2.4.2-1, so no unit is bound to it.

### [`RFC3032-2.4.2-2`](#rfc3032-2.4.2-2)

When TTL=0, the label stack MUST NOT be stripped off and the packet forwarded as unlabeled (S2.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3032-2.4.2-2, so no unit is bound to it.

### [`RFC3032-2.4.2-3`](#rfc3032-2.4.2-3)

When forwarding, the TTL field of the top label stack entry MUST be set to the outgoing TTL value (S2.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3032-2.4.2-3, so no unit is bound to it.

### [`RFC3032-2.4.3-1`](#rfc3032-2.4.3-1)

When an IP packet is first labeled, the label TTL field MUST be set to the value of the IP TTL field (S2.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3032-2.4.3-1, so no unit is bound to it.

### [`RFC3032-3.3-1`](#rfc3032-3.3-1)

A labeled packet that is not "too big" MUST be transmitted without fragmentation (S3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3032-3.3-1, so no unit is bound to it.

### [`RFC3032-3.3-2`](#rfc3032-3.3-2)

A labeled IP datagram whose size exceeds the True Maximum Frame Payload Size MUST be considered "too big" (S3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3032-3.3-2, so no unit is bound to it.

### [`RFC3032-3.4-1`](#rfc3032-3.4-1)

If a labeled IPv4 datagram is too big and has the DF bit set, the LSR MUST execute the strip/fragment/ICMP algorithm (S3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3032-3.4-1, so no unit is bound to it.

### [`RFC3032-3.4-2`](#rfc3032-3.4-2)

Each IPv4 fragment MUST be at least N bytes less than the Effective Maximum Frame Payload Size (S3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3032-3.4-2, so no unit is bound to it.

### [`RFC3032-3.4-3`](#rfc3032-3.4-3)

If the DF bit is set and packet is too big, the datagram MUST NOT be forwarded (S3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3032-3.4-3, so no unit is bound to it.

### [`RFC3032-3.5-1`](#rfc3032-3.5-1)

To process a labeled IPv6 datagram that is too big, the LSR MUST execute the specified algorithm (S3.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3032-3.5-1, so no unit is bound to it.

### [`RFC3032-3.5-2`](#rfc3032-3.5-2)

Each IPv6 fragment MUST be at least N bytes less than the Effective Maximum Frame Payload Size (S3.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3032-3.5-2, so no unit is bound to it.

### [`RFC3032-3.6-1`](#rfc3032-3.6-1)

The tunnel transmitting endpoint MUST be able to determine the MTU of the tunnel as a whole (S3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3032-3.6-1, so no unit is bound to it.

### [`RFC3032-3.6-2`](#rfc3032-3.6-2)

The tunnel transmitting endpoint MUST send ICMP Destination Unreachable when a DF-set packet exceeds tunnel MTU (S3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3032-3.6-2, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6, rfc3032 |
| Signed off | 2026-08-31 |
| Register | prose |
| Source | rfc/full/rfc3032.txt |
| Source fingerprint | 4907c7ce709476f7 |
| Record | rfc/extraction/rfc3032.json |
| Mapped sentences | 16 |
| Declined as scope | 21 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 2 | walked | Title block, Status of this Memo, Copyright Notice, Abstract and Table of Contents. Walked rather than skipped because the site scan attributes two sites here: both are Abstract sentences, and both are classified below. The Abstract restates section 1 -- MPLS needs procedures for augmenting network layer packets with label stacks, and this document specifies the encoding on PPP and LAN data links. |
| `1` | Introduction | 2 | walked | Introduction. States why the encoding is needed, what the document specifies (the encoding on PPP and LAN links, plus protocol-independent and IPv4/IPv6-dependent field-processing rules), and closes with the one obligation this section places on an implementation: an LSR using a different encoding for the top one or two entries, as an ATM switch does, still uses this encoding for the additional entries. Site 1:2 carries that obligation and is mapped to RFC3032-1-1; site 1:1 is the indicative premise sentence and is excluded below. |
| `1.1` | Specification of Requirements | 0 | walked | Specification of Requirements. The RFC 2119 key-words paragraph. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. |
| `2` | The Label Stack | 0 | walked | The Label Stack. A heading with no body text of its own; sections 2.1 to 2.4 carry the content. |
| `2.1` | Encoding the Label Stack | 2 | walked | Encoding the Label Stack. The document's wire-format section: the 4-octet label stack entry with its 20-bit Label, 3-bit Exp, 1-bit S and 8-bit TTL fields, the ordering rule that the top of the stack appears earliest and the network layer packet follows the entry with S set, what a successful top-label lookup yields, and the reserved label values 0 to 15. Written entirely in the indicative, and the layout, the ordering and the reserved values are carried by the Wire Formats, Encoding Rules and Constants tables of rfc/short/rfc3032.md. Its two sites both state the disposition of an Explicit NULL label and are excluded below. Ze's shim encoder is the 3-octet BGP NLRI form of internal/core/bgp/nlri.WriteLabelStack, which carries the Label and the S bit and no TTL; the 4-octet on-wire entry is built by the kernel or by VPP. |
| `2.2` | Determining the Network Layer Protocol | 6 | walked | Determining the Network Layer Protocol. The document's protocol-identification rules: the LSR that pops the last label must be able to identify the network layer protocol, the identity must be inferable from the bottom label and possibly the network layer header, a first-pushed label and every label that replaces it must meet that criterion, and a packet that cannot be forwarded whose protocol cannot be identified is silently discarded. Sites 2.2:2, 2.2:3 and 2.2:6 carry the three obligations rfc/short/rfc3032.md declares as RFC3032-2.2-1, RFC3032-2.2-2 and RFC3032-2.2-3 and are mapped below; 2.2:1 and 2.2:4 restate two of them and 2.2:5 sits inside a desirability construction. |
| `2.3` | Generating ICMP Messages for Labeled IP Packets | 3 | walked | Generating ICMP Messages for Labeled IP Packets. States the two conditions that must hold for an LSR to send an ICMP packet to the source of a labeled IP packet, points at section 2.2 for the first, at 2.3.1 and 2.3.2 for the second, and records that in some cases the second does not hold at all and no ICMP message can be generated. All three sites bind the LSR that originates ICMP for a labeled packet and are excluded below. |
| `2.3.1` | Tunneling through a Transit Routing Domain | 1 | walked | Tunneling through a Transit Routing Domain. A deployment discussion: when external routes are not leaked into a transit domain's interior routers, only an ASBR knows how to route to an arbitrary source, and injecting default into the IGP lets an unlabeled ICMP packet reach a router with full routing information. It offers a network design, not an implementation rule; its one site is excluded below. |
| `2.3.2` | Tunneling Private Addresses through a Public Backbone | 1 | walked | Tunneling Private Addresses through a Public Backbone. The alternative when the source address cannot be routed at all: copy the label stack from the original packet onto the ICMP message and label switch it, so the message leaves the MPLS domain where the source is routable. Its one site directs the LSR that builds such an ICMP message and is excluded below. |
| `2.4` | Processing the Time to Live Field | 0 | walked | Processing the Time to Live Field. A heading with no body text of its own; sections 2.4.1 to 2.4.4 carry the content. |
| `2.4.1` | Definitions | 0 | walked | Definitions. Defines the incoming TTL as the TTL field of the top label stack entry on receipt, and the outgoing TTL as the larger of one less than the incoming TTL and zero. Definitions, not directives. |
| `2.4.2` | Protocol-independent rules | 2 | walked | Protocol-independent rules. Two capitalised MUST-level sites, mapped below to RFC3032-2.4.2-1 and RFC3032-2.4.2-3. Two further declared rows are read from the same paragraphs and are listed as unsourced ids: RFC3032-2.4.2-2 is the second clause of the sentence site 2.4.2:1 quotes, 'nor may the label stack be stripped off and the packet forwarded as an unlabeled packet', which the splitter keeps in one site because it is one sentence; RFC3032-2.4.2-4 is the MAY that follows, 'the packet MAY be simply discarded, or it may be passed to the appropriate "ordinary" network layer for error processing'. The closing paragraph, that the outgoing TTL is a function solely of the incoming TTL and that a non-top entry's TTL has no significance, is indicative. |
| `2.4.3` | IP-dependent rules | 1 | walked | IP-dependent rules. Defines the IP TTL as the IPv4 TTL or the IPv6 Hop Limit, states the one capitalised MUST mapped below to RFC3032-2.4.3-1, and then the SHOULD listed as an unsourced id: 'the value of the IP TTL field SHOULD BE replaced with the outgoing TTL value' when a pop empties the stack, which in IPv4 also requires modifying the header checksum. It closes by recognizing that an administration may prefer a single IPv4 TTL decrement across an MPLS domain, which is an observation and not a directive. |
| `2.4.4` | Translating Between Different Encapsulations | 0 | walked | Translating Between Different Encapsulations. States where the incoming and outgoing TTL values come from when one side of the LSR uses a different encapsulation, such as LC-ATM, and defers to that encapsulation's own procedures. Indicative throughout, and no site. |
| `3` | Fragmentation and Path MTU Discovery | 1 | walked | Fragmentation and Path MTU Discovery. Introduces the problem: a received packet can be too large for its output link, and pushing labels can make a packet that fitted no longer fit. It then states what the section provides, namely rules that let Path MTU Discovery hosts and IPv6 hosts avoid fragmentation, and discusses which hosts are at risk. Its one site is a parenthetical remark about likelihood and is excluded below. |
| `3.1` | Terminology | 0 | walked | Terminology. Defines Frame Payload, Conventional Maximum Frame Payload Size, True Maximum Frame Payload Size, Effective Maximum Frame Payload Size for Labeled Packets, Initially Labeled IP Datagram and Previously Labeled IP Datagram. Definitions, not directives, and no site. |
| `3.2` | Maximum Initially Labeled IP Datagram Size | 1 | walked | Maximum Initially Labeled IP Datagram Size. One SHOULD, listed as the unsourced id below: an LSR that can receive an unlabeled IP datagram, add a label stack and forward the result 'SHOULD support a configuration parameter known as the "Maximum Initially Labeled IP Datagram Size"'. The rest of the section states what a zero and a positive setting do, and its one site is the positive setting's fragment-before-labeling behavior, which sits inside that SHOULD and is excluded below. |
| `3.3` | not stated | 2 | walked | When are Labeled IP Datagrams Too Big? Two capitalised MUST-level sites, mapped below to RFC3032-3.3-2 and RFC3032-3.3-1. Its MAY, 'A labeled IP datagram whose size exceeds the Conventional Maximum Frame Payload Size ... MAY be considered to be "too big"', is the unsourced id below. |
| `3.4` | Processing Labeled IPv4 Datagrams which are Too Big | 4 | walked | Processing Labeled IPv4 Datagrams which are Too Big. The five-step algorithm an LSR runs on a too-big labeled IPv4 datagram: strip the stack, compute N, fragment and relabel when DF is clear, and when DF is set do not forward, build an ICMP Destination Unreachable with the Fragmentation Required code and a Next-Hop MTU of the Effective Maximum Frame Payload Size less N, and transmit it if possible. Sites 3.4:1, 3.4:2 and 3.4:3 carry the three MUSTs and are mapped below to RFC3032-3.4-1, RFC3032-3.4-2 and RFC3032-3.4-3. Site 3.4:4 is the ICMP Code field value and is excluded below. The opening MAY, 'the LSR MAY silently discard the datagram' when DF is not set, is the unsourced id. |
| `3.5` | Processing Labeled IPv6 Datagrams which are Too Big | 2 | walked | Processing Labeled IPv6 Datagrams which are Too Big. The IPv6 algorithm: strip the stack, compute N, send an ICMP Packet Too Big and discard when the datagram exceeds 1280 bytes or has no fragment header, otherwise fragment, relabel and forward. Its two capitalised MUST-level sites are mapped below to RFC3032-3.5-1 and RFC3032-3.5-2, and every remaining step is an unnumbered clause of the algorithm those two ids gate. |
| `3.6` | Implications with respect to Path MTU Discovery | 3 | walked | Implications with respect to Path MTU Discovery. States what the too-big rules mean for RFC 1191 Path MTU Discovery, then the tunnel case: when an ICMP message cannot be forwarded out of an MPLS tunnel to the source, the transmitting endpoint must know the tunnel MTU and must send the ICMP Destination Unreachable itself. Sites 3.6:2 and 3.6:3 carry those two obligations and are mapped below to RFC3032-3.6-1 and RFC3032-3.6-2; site 3.6:1 is the conditional lead-in and is excluded. The MAY that offers Path MTU Discovery through the tunnel as the way to learn the tunnel MTU is the unsourced id. |
| `4` | Transporting Labeled Packets over PPP | 0 | walked | Transporting Labeled Packets over PPP. One paragraph naming what section 4 defines: the PPP Network Control Protocol for establishing and configuring label switching over PPP. No directive and no site. |
| `4.1` | Introduction | 2 | walked | Introduction. Names PPP's three components and states the link-establishment sequence: LCP first, then the MPLS Control Protocol, then labeled packets once MPLSCP reaches the Opened state. Its two sites are excluded below, the first to RFC 1661 and the second to the MPLS-over-PPP role. |
| `4.2` | A PPP Network Control Protocol for MPLS | 0 | walked | A PPP Network Control Protocol for MPLS. Defines MPLSCP as LCP with five exceptions: negotiated frame modifications, PPP Protocol field 0x8281, only Codes 1 through 7, the timeout advice, and no configuration option types. Its directives are lowercase 'should' recommendations about discarding early packets and Code-Rejecting other codes, which the site scan does not read as MUST-level and rfc/short/rfc3032.md does not declare; the values are carried by its Transport Encapsulations table. Ze runs no MPLSCP: no source file names the protocol or the 0x8281 PPP type. |
| `4.3` | Sending Labeled Packets | 1 | walked | Sending Labeled Packets. States the precondition for sending labeled packets over PPP, the PPP Protocol field values 0x0281 for unicast and 0x0283 for multicast, the maximum length, and that the Information field format is the one from section 2. Its one site is the precondition and is excluded below; the values are carried by the Transport Encapsulations table of rfc/short/rfc3032.md. |
| `4.4` | Label Switching Control Protocol Configuration Options | 0 | walked | Label Switching Control Protocol Configuration Options. One sentence: there are no configuration options. No site. |
| `5` | Transporting Labeled Packets over LAN Media | 0 | walked | Transporting Labeled Packets over LAN Media. Value assignment: one labeled packet per frame, the label stack entries immediately precede the network layer header and follow every data link header including 802.1Q, ethertype 0x8847 for unicast and 0x8848 for multicast, usable with either the ethernet or the 802.3 LLC/SNAP encapsulation. Indicative throughout, and the values are carried by the Transport Encapsulations and Constants tables of rfc/short/rfc3032.md. |
| `6` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records that label values 0-15 have special meaning, that 0-3 are specified in section 2.1, and that 4-15 may be assigned by IANA on IETF Consensus. Binds IANA, not a speaker. |
| `7` | Security Considerations | 0 | walked | Security Considerations. States that the encapsulation raises no security issue not already present in the MPLS architecture or in the encapsulated network layer protocol, then names two inherited considerations: a fixed-offset security check fails against a variable-size encapsulation, and the label stack carries no identity of the label writer, so accepting labels from untrusted sources can route packets illegitimately. No countermeasure is directed at a speaker and no site. |
| `8` | Intellectual Property | 0 | walked | Intellectual Property. The IETF's standard statement about intellectual property rights covering the technology: the IETF takes no position on validity or scope, and points a reader at BCP 11 and the IETF Executive Director. No obligation on an implementation and no site. |
| `9` | Authors' Addresses | 0 | walked | Authors' Addresses. Postal and electronic addresses for the eight authors. No site. |
| `10` | not stated | 0 | skipped (references) | References: RFC 3031, RFC 2119, RFC 792, RFC 1191, RFC 2113, RFC 1661, RFC 2460, RFC 1885, and the ATM and Frame Relay label switching documents. |
| `11` | Full Copyright Statement | 1 | walked | Full Copyright Statement. The Internet Society boilerplate governing copying and translation of the document, and, because no numbered heading follows it, the Acknowledgement block that closes the document. Walked rather than skipped because the prose scan attributes one site here; that site is boilerplate and is excluded below. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the LSR data-link transmit path: the sentence says an LSR must support an encoding technique that, given a label stack and a network layer packet, PRODUCES a labeled packet on a data link. Ze produces no labeled packet on a wire. It is an MPLS control plane: it programs AF_MPLS swap and pop entries and label-imposing routes through fibkernel.handleMPLSEntry (internal/plugins/fib/kernel/mpls.go), reads the kernel forwarding table for `show mpls forwarding` (internal/component/mpls.dumpMPLSRoutes), and its only label encoder is the 3-octet BGP NLRI form of internal/core/bgp/nlri.WriteLabelStack, which is RFC 8277 and carries no TTL. The 4-octet on-wire entry this sentence demands is built by the kernel AF_MPLS datapath or by VPP. This is the Abstract's copy of the sentence that reappears as site 1:1. | In order to transmit a labeled packet on a particular data link, an LSR must support an encoding technique which, given a label stack and a network layer packet, produces a labeled packet. |
| `front:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Abstract's copy of the section 1 obligation that additional label stack entries use this encoding when the top entries use another. Site 1:2 maps the section 1 sentence to RFC3032-1-1; this one restates it in the Abstract with no new obligation. | On some data links, the label at the top of the stack may be encoded in a different manner, but the techniques described here MUST be used to encode the remainder of the label stack. |
| `1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the LSR data-link transmit path, the same role as site front:1: the LSR must support an encoding technique that produces a labeled packet from a label stack and a network layer packet. Ze builds no labeled packet for transmission; the kernel AF_MPLS datapath or VPP does, and ze only programs the entries that drive it (internal/plugins/fib/kernel/mpls.go, addMPLSSwap and the rich-route push path). | In order to transmit a labeled packet on a particular data link, an LSR must support an encoding technique which, given a label stack and a network layer packet, produces a labeled packet. |
| `2.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the LSR label-disposition role: the sentence says what an IPv4 Explicit NULL label at the bottom of the stack INDICATES to the LSR that receives it, namely that the stack is popped and forwarding then follows the IPv4 header. Popping a stack and forwarding on the exposed IP header is a per-packet action of the kernel AF_MPLS datapath or VPP; no ze function forwards a labeled packet. Ze's own use of label 0 is control-plane signaling of the value (internal/plugins/ospf/sr.ExplicitNullV4, internal/plugins/ldp/wire.ExplicitNull), and the value assignment itself is carried by the Constants table of rfc/short/rfc3032.md rather than by any gated row. | It indicates that the label stack must be popped, and the forwarding of the packet must then be based on the IPv4 header. |
| `2.1:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The IPv6 Explicit NULL counterpart of site 2.1:1, binding the same LSR label-disposition role: pop the stack, forward on the IPv6 header. That disposition is performed by the kernel AF_MPLS datapath or VPP. Ze signals the value from its control plane (internal/plugins/ospf/sr.ExplicitNullV6) and forwards no labeled packet. | It indicates that the label stack must be popped, and the forwarding of the packet must then be based on the IPv6 header. |
| `2.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the protocol-identification obligation as a consequence: the LSR that pops the last label must be able to identify the network layer protocol. The document then says how, in the sentence site 2.2:2 quotes, which is mapped to RFC3032-2.2-1. Same obligation, stated once as the need and once as the mechanism. | The LSR which pops the last label off the stack must therefore be able to identify the packet's network layer protocol. |
| `2.2:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Extends the first-label criterion of site 2.2:3 to every label that replaces it in transit: 'the new value must also be one which meets the same criteria'. It restates RFC3032-2.2-2, which site 2.2:3 maps, for the swap case and adds no separate obligation. | Furthermore, whenever that label is replaced by another label value during a packet's transit, the new value must also be one which meets the same criteria. |
| `2.2:5` | `advisory-in-context` (never bound Ze): the sentence advises on applying a rule stated elsewhere and adds no obligation of its own | The enclosing construction makes it advisory: 'Adherence to these conditions does not necessarily enable intermediate nodes to identify a packet's network layer protocol. Under ordinary conditions, this is not necessary, but there are error conditions under which it is desirable. For instance, if an intermediate LSR determines that a labeled packet is undeliverable, it may be desirable for that LSR to generate error messages which are specific to the packet's network layer.' The site's own sentence keeps that frame ('So if intermediate nodes are to be able to generate protocol-specific error messages'), so the lowercase 'must' states what follows from a desirable capability rather than an obligation on every speaker, and rfc/short/rfc3032.md declares no row for it. | So if intermediate nodes are to be able to generate protocol-specific error messages for labeled packets, all labels in the stack must meet the criteria specified above for labels which appear at the bottom of the stack. |
| `2.3:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the LSR that ORIGINATES an ICMP message for a labeled IP packet: the sentence states the two conditions that must hold for that LSR to build the ICMP packet and have it reach the IP source, and sites 2.3:2 and 2.3:3 are those two conditions. Ze originates no ICMP for a forwarded packet: TTL expiry and too-big handling for labeled packets are per-packet actions of the kernel AF_MPLS datapath or VPP, and no ze function builds an ICMP message on a forwarding path. The producer that would act as it if ze did is ze's MPLS code, which reads the kernel's label forwarding entries (`dumpMPLSRoutes`, `internal/component/mpls/forwarding_linux.go`) and codes labeled NLRI. It switches no packet and builds no ICMP message. | In order for a particular LSR to be able to generate an ICMP packet and have that packet sent to the source of the IP packet, two conditions must hold: |
| `2.3:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The first of the two conditions on the ICMP-originating LSR named by site 2.3:1: it must be possible for that LSR to determine that a particular labeled packet is an IP packet. Ze inspects no labeled packet and originates no ICMP; the kernel AF_MPLS datapath or VPP does both. The producer that would act as it if ze did is ze's MPLS code, which reads the kernel's label forwarding entries (`dumpMPLSRoutes`, `internal/component/mpls/forwarding_linux.go`) and codes labeled NLRI. It switches no packet and builds no ICMP message. | 1. it must be possible for that LSR to determine that a particular labeled packet is an IP packet; |
| `2.3:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The second condition on the same ICMP-originating LSR: it must be possible for that LSR to route to the packet's IP source address. Ze originates no ICMP for a forwarded packet, so it never plays the role this condition qualifies. The producer that would act as it if ze did is ze's MPLS code, which reads the kernel's label forwarding entries (`dumpMPLSRoutes`, `internal/component/mpls/forwarding_linux.go`) and codes labeled NLRI. It switches no packet and builds no ICMP message. | 2. it must be possible for that LSR to route to the packet's IP source address. |
| `2.3.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Non-normative use: the sentence describes what a deployment choice would ACHIEVE, not what an implementation does. Its subject is the ASBRs injecting default into the IGP, and the lowercase 'must' sits in a relative clause qualifying which packets are affected ('any unlabeled packet which must leave the domain (such as an ICMP packet)'), so it describes a packet rather than directing a speaker. The paragraph offers one network design and the next paragraph names its limits. | (N.B.: this does NOT require that there be a "default" carried by BGP.) This would then ensure that any unlabeled packet which must leave the domain (such as an ICMP packet) gets sent to a router which has full routing information. |
| `2.3.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the LSR that builds an ICMP message for a labeled packet and label switches it: the sentence says how to copy the label stack, exactly for the label values and with the ICMP header's TTL for the TTL fields. Ze builds no such message. ICMP origination on the MPLS forwarding path belongs to the kernel AF_MPLS datapath or VPP, and no ze function copies a label stack onto an ICMP message. The producer that would act as it if ze did is ze's MPLS code, which reads the kernel's label forwarding entries (`dumpMPLSRoutes`, `internal/component/mpls/forwarding_linux.go`) and codes labeled NLRI. It switches no packet and builds no ICMP message. | When copying the label stack from the original packet to the ICMP message, the label values must be copied exactly, but the TTL values in the label stack should be set to the TTL value that is placed in the IP header of the ICMP message. |
| `3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Non-normative use: the sentence is a parenthetical estimate of LIKELIHOOD, '(Even so, fragmentation is not likely unless the packet must traverse an ethernet of some sort between the time it first gets labeled and the time it gets unlabeled.)', closing a paragraph about hosts that send 1500-byte datagrams within a classful network number. The lowercase 'must' qualifies the path a packet takes, and the sentence directs nobody. | (Even so, fragmentation is not likely unless the packet must traverse an ethernet of some sort between the time it first gets labeled and the time it gets unlabeled.) |
| `3.2:1` | `advisory-in-context` (never bound Ze): the sentence advises on applying a rule stated elsewhere and adds no obligation of its own | The enclosing construction is a SHOULD: 'Every LSR which is capable of a) receiving an unlabeled IP datagram, b) adding a label stack to the datagram, and c) forwarding the resulting labeled packet, SHOULD support a configuration parameter known as the "Maximum Initially Labeled IP Datagram Size", which can be set to a non-negative value.' This site is the parameter's effect when it is set to a positive value ('If it is set to a positive value, it is used in the following way'), so its lowercase 'must' describes the behavior of an optional parameter rather than an independent obligation. rfc/short/rfc3032.md declares the enclosing SHOULD as RFC3032-3.2-1, listed in this section's unsourced ids. | a) an unlabeled IP datagram is received, and b) that datagram does not have the DF bit set in its IP header, and c) that datagram needs to be labeled before being forwarded, and d) the size of the datagram (before labeling) exceeds the value of the parameter, then a) the datagram must be broken into fragments, each of whose size is no greater than the value of the parameter, and b) each fragment must be labeled and then forwarded. |
| `3.4:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Non-normative use: the keyword is inside a quoted VALUE NAME, not a directive. The step reads 'set its Code field [3] to "Fragmentation Required and DF Set"', and 'Fragmentation Required and DF Set' is the name of ICMP Destination Unreachable code 4 as RFC 792 and RFC 1191 spell it. The case-insensitive prose scan sees 'Required' inside that quoted name. The step itself is one clause of the algorithm gated by RFC3032-3.4-1, which site 3.4:1 maps. | i. set its Code field [3] to "Fragmentation Required and DF Set", |
| `3.6:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Non-normative use: the sentence is the conditional lead-in that scopes the two bulleted obligations, and it ends in a colon rather than stating one. Its lowercase 'must' sits in a relative clause describing the packets ('packets that must go through the tunnel, but are too large to pass through the tunnel unfragmented'), and the obligations it introduces are sites 3.6:2 and 3.6:3, mapped below. | If it is not possible to forward an ICMP message from within an MPLS "tunnel" to a packet's source address, but the network configuration makes it possible for the LSR at the transmitting end of the tunnel to receive packets that must go through the tunnel, but are too large to pass through the tunnel unfragmented, then: |
| `4.1:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The obligation belongs to RFC 1661, the Point-to-Point Protocol, which this section cites as [6] and whose link-establishment procedure the sentence restates as background: each end sends LCP packets to configure and test the data link before communications are established. It is not an MPLS rule, and rfc/short/rfc3032.md declares no row for it. Ze does implement LCP under RFC 1661, in internal/component/l2tp/ppp, for L2TP and PPPoE subscriber access; what it does not implement is this section's MPLSCP, which sites 4.1:2 and 4.3:1 govern. | In order to establish communications over a point-to-point link, each end of the PPP link must first send LCP packets to configure and test the data link. |
| `4.1:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the MPLS-over-PPP LSR role, the PPP endpoint that runs the MPLS Control Protocol so labeled packets can be transmitted on the link. Ze does not play it: no source file names MPLSCP or the 0x8281 PPP protocol type, and ze's PPP implementation (internal/component/l2tp/ppp) carries L2TP and PPPoE subscriber sessions, never labeled packets. Its MPLS forwarding entries are programmed into the kernel AF_MPLS datapath or VPP, which owns whatever link encapsulation the packets then take. | After the link has been established and optional facilities have been negotiated as needed by the LCP, PPP must send "MPLS Control Protocol" packets to enable the transmission of labeled packets. |
| `4.3:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the same MPLS-over-PPP LSR role as site 4.1:2: before labeled packets are communicated, PPP must reach the Network-Layer Protocol phase and MPLSCP must reach the Opened state. Ze negotiates no MPLSCP and sends no labeled packet over a PPP link. The producer that would act as it if ze did is ze's MPLS code, which reads the kernel's label forwarding entries (`dumpMPLSRoutes`, `internal/component/mpls/forwarding_linux.go`) and codes labeled NLRI. It switches no packet and builds no ICMP message. | Before any labeled packets may be communicated, PPP must reach the Network-Layer Protocol phase, and the MPLS Control Protocol must reach the Opened state. |
| `11:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Non-normative use: this is the Internet Society's Full Copyright Statement, which the derivation attributes to section 11. The sentence governs how the DOCUMENT may be copied and translated, saying it may not be modified except as needed for developing Internet standards or for translation. It states nothing about an implementation. | However, this document itself may not be modified in any way, such as by removing the copyright notice or references to the Internet Society or other Internet organizations, except as needed for the purpose of developing Internet standards in which case the procedures for copyrights defined in the Internet Standards process must be followed, or as required to translate it into languages other than English. |

## Superseded

No document obsoletes RFC 3032, so its obligations are stated where they were written.
