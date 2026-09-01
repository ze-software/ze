# RFC 4303 - IP Encapsulating Security Payload (ESP)

Supported. Every requirement this repository extracted from RFC 4303, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 75.0% | 6 of 8 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 25.0% | 2 of 8 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 8 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 8 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 17 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 22 | of 25 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 14 | of 22 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 8 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 25 |
| Gated MUST-level | 22 |
| Obligations that bind Ze | 8 |
| Not applicable, so out of scope | 14 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 17 |
| Tagged units | 17 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc4303.md` |
| Requirement shard | `rfc/requirements/rfc4303.md` |
| RFC text | `rfc/full/rfc4303.txt` |

## Enrolment

Enrolled: IP Encapsulating Security Payload (RFC 4303): 1 MET (SPI 0/reserved never on the wire) + 2 single-polarity positive (32-packet anti-replay window projected, anti-replay never without integrity) + 14 not-applicable (per-packet ESP datapath -- sequence, padding, TFC, SAD lookup, ICV, anti-replay check -- kernel XFRM)

## What the public ledger says

**Status:** Supported

**What the ledger says is covered:**

ESP SA parameter model, protocol 50, tunnel and transport modes, XFRM installation, OSPFv3 manual ESP.

**What the ledger says remains:**

No tracked gap in current source anchors.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 6 | one part of the gated population |
| Annotated instead of tested | 16 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **22** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (6):** [`RFC4303-1-1`](#rfc4303-1-1), [`RFC4303-2-1`](#rfc4303-2-1), [`RFC4303-2.1-1`](#rfc4303-2.1-1), [`RFC4303-2.1-2`](#rfc4303-2.1-2), [`RFC4303-3.2-1`](#rfc4303-3.2-1), [`RFC4303-2.2.1-2`](#rfc4303-2.2.1-2)

**Annotated instead of tested (16):** [`RFC4303-2.2-1`](#rfc4303-2.2-1), [`RFC4303-2.2-2`](#rfc4303-2.2-2), [`RFC4303-2.2-3`](#rfc4303-2.2-3), [`RFC4303-2.4-1`](#rfc4303-2.4-1), [`RFC4303-2.6-1`](#rfc4303-2.6-1), [`RFC4303-2.6-2`](#rfc4303-2.6-2), [`RFC4303-3.3.3-1`](#rfc4303-3.3.3-1), [`RFC4303-3.3.4-1`](#rfc4303-3.3.4-1), [`RFC4303-3.3.4-2`](#rfc4303-3.3.4-2), [`RFC4303-3.4.3-1`](#rfc4303-3.4.3-1), [`RFC4303-3.4.3-2`](#rfc4303-3.4.3-2), [`RFC4303-3.4.3-3`](#rfc4303-3.4.3-3), [`RFC4303-3.4.2-1`](#rfc4303-3.4.2-1), [`RFC4303-3.4.2-2`](#rfc4303-3.4.2-2), [`RFC4303-3.4.4.1-1`](#rfc4303-3.4.4.1-1), [`RFC4303-3.4.4.2-1`](#rfc4303-3.4.4.2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4303-1-1` | Integrity-only ESP MUST be offered as a service selection option and MUST be configurable via management interfaces (§1) | MUST | 1 - Introduction | **positive:** `unit/verify` [`TestIPsecESPRequiresIntegrity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L126). **negative:** `unit/verify` [`TestIPsecESPRequiresIntegrity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L111) |
| `RFC4303-2-1` | The outer protocol header that precedes the ESP header SHALL carry the value 50 in its Protocol or Next Header field (§2) | SHALL | 2 - Packet format | **positive:** `unit/verify` [`TestIPsecSAProtocolNumber`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L290). **negative:** `unit/verify` [`TestIPsecSAProtocolNumber`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L295) |
| `RFC4303-2.1-1` | SPI value 0 MUST NOT appear on the wire (reserved for local use) (§2.1) | MUST NOT | 2.1 - Security Parameters Index | **positive:** `unit/verify` [`TestGenerateESPSPI`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L243). **positive:** `unit/verify` [`TestIPsecSPIBoundary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L191). **negative:** `unit/verify` [`TestGenerateESPSPI`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L257). **negative:** `unit/verify` [`TestIPsecSPIBoundary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L185) |
| `RFC4303-2.1-2` | Whether source and destination address matching is required to map inbound traffic to an SA MUST be set by manual SA configuration or by SA management protocol negotiation (§2.1) | MUST | 2.1 - Security Parameters Index | **positive:** `unit/verify` [`TestIPsecSAAddressMatchIndication`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L313). **negative:** `unit/verify` [`TestIPsecSAAddressMatchIndication`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L319) |
| `RFC4303-2.2-1` | Sequence Number MUST be incremented for every transmitted packet (§2.2) | MUST | 2.2 - Sequence Number | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the per-SA sequence counter is kernel XFRM/ESP per-packet state; ze installs the SA but never touches sequence numbers (internal/component/ike/dataplane/dataplane.go:80-108 has no sequence field) |
| `RFC4303-2.2-2` | Sender MUST NOT send a packet that would cause the sequence counter to cycle (overflow) (§2.2) | MUST NOT | 2.2 - Sequence Number | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** 32-bit counter overflow protection is enforced by the kernel XFRM datapath (it expires the state rather than wrapping); ze's control plane holds no per-packet counter |
| `RFC4303-2.2-3` | Counter and receiver window MUST be reset before the 2^32nd packet on a non-ESN SA (§2.2) | MUST | 2.2 - Sequence Number | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the sequence counter and receive window are kernel per-packet SA state; the kernel enforces the 2^32 boundary and a fresh counter/window arises from installing a new SA |
| `RFC4303-2.4-1` | Padding MUST bring plaintext to the block size of the cipher and ensure 4-byte alignment of the resulting ciphertext (§2.4) | MUST | 2.4 - Padding for encryption | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ESP padding is applied by the kernel encryption datapath during packet construction; ze projects only the algorithm choice and never builds ESP payloads |
| `RFC4303-2.6-1` | Transmitter MUST be capable of generating dummy packets (Next Header = 59) for traffic-flow confidentiality (§2.6) | MUST | 2.6 - Next Header | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** dummy/TFC packet generation is an ESP transmit-datapath function; ze emits no ESP packets, so it plays no role in producing Next-Header-59 dummies |
| `RFC4303-2.6-2` | Receiver MUST silently discard dummy packets (Next Header = 59) (§2.6) | MUST | 2.6 - Next Header | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** inbound Next-Header-59 dummy discard occurs in the kernel ESP receive path after decryption; ze processes no inbound ESP payloads |
| `RFC4303-3.2-1` | The encryption and integrity algorithms MUST NOT both be NULL; at least one ESP service is always selected (§3.2) | MUST NOT | 3.2 - Algorithms | **positive:** `unit/verify` [`TestIPsecESPRequiresIntegrity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L130). **negative:** `unit/verify` [`TestIPsecESPRequiresIntegrity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L107) |
| `RFC4303-3.3.3-1` | Sender initializes sequence counter to 0 at SA establishment; first transmitted packet carries Sequence Number 1 (§3.3.3) | MUST | 3.3.3 - Sequence Number generation | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the kernel initializes the sequence counter when the SA state is added; ze installs a fresh SA with no seq field and never sets or reads the counter (internal/component/ike/dataplane/xfrm_linux.go:21-86) |
| `RFC4303-3.3.4-1` | Transport-mode ESP is applied only to whole IP datagrams, never to fragments (§3.3.4) | MUST | 3.3.4 - Fragmentation | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ordering ESP relative to fragmentation is an outbound kernel datapath decision; ze selects transport vs tunnel mode as an SA parameter but does not apply ESP to packets (internal/plugins/ospf/ipsec_install.go:413) |
| `RFC4303-3.3.4-2` | Implementation MUST support Path MTU Discovery (§3.3.4) | MUST | 3.3.4 - Fragmentation | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Path MTU Discovery is provided by the kernel networking/XFRM stack; ze's control plane installs SAs and does not run the datapath MTU machinery |
| `RFC4303-3.4.3-1` | Anti-replay sliding window: minimum window size of 32 packets MUST be supported (§3.4.3) | MUST | 3.4.3 - Sequence Number verification, the anti-replay section | **positive:** `unit/verify` [`TestChildSAReplayWindowDefault`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L308). **negative:** no negative test. **{single-polarity}:** ze projects a 64-packet anti-replay window on IKE child SAs (ReplayWin=64) to the kernel state, which exceeds the minimum at the SA-parameter level, while the sliding-window check itself runs in the kernel (internal/component/ike/engine/child.go:62, :377, :444, internal/component/ike/dataplane/xfrm_linux.go:132-133) |
| `RFC4303-3.4.3-2` | Anti-replay MUST NOT be enabled unless the ESP integrity service is also enabled (§3.4.3) | MUST NOT | 3.4.3 - Sequence Number verification, the anti-replay section | **positive:** `unit/verify` [`TestChildSAReplayRequiresIntegrity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L334). **negative:** no negative test. **{single-polarity}:** ze's ESP SA model always projects integrity (planStateAlgos returns Crypt+Auth or AEAD for ESP) and the replay window is set only on those integrity-bearing child SAs, so anti-replay is structurally never enabled without integrity (internal/component/ike/dataplane/dataplane.go:53-62, internal/component/ike/engine/child.go:218-261) |
| `RFC4303-3.4.3-3` | Window advance occurs only after integrity verification succeeds (never on unverified packets) (§3.4.3) | MUST | 3.4.3 - Sequence Number verification, the anti-replay section | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** window advancement is gated on per-packet ICV verification inside the kernel ESP receive path; ze has no control over when the window advances |
| `RFC4303-3.4.2-1` | For unicast, SA lookup uses SPI (or SPI plus protocol); invalid SA causes discard (auditable event) (§3.4.2) | MUST | 3.4.2 - Inbound SA lookup | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** inbound SAD lookup by SPI and discard/audit on miss is performed per packet by the kernel; ze installs SAs keyed by SPI but never performs the lookup (internal/plugins/ospf/ipsec_install.go:484-498 samples kernel drop counters only) |
| `RFC4303-3.4.2-2` | For multicast, destination address is also used in SA lookup (§3.4.2) | MUST | 3.4.2 - Inbound SA lookup | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** multicast SA resolution is a kernel per-packet lookup; ze only installs the wildcard state and proto-89 selector that lets the kernel resolve OSPFv3 multicast flows (internal/plugins/ospf/ipsec_install.go:415-419) |
| `RFC4303-3.4.4.1-1` | Separate-algorithm processing: verify ICV first, then decrypt, then check padding (§3.4.4.1) | MUST | 3.4.4.1 - Separate confidentiality and integrity algorithms, inbound | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the verify-then-decrypt-then-check-padding ordering is executed by the kernel ESP inbound datapath; ze projects the crypt+auth algorithm pair but does no packet processing (internal/component/ike/dataplane/xfrm_linux.go:59-71) |
| `RFC4303-3.4.4.2-1` | Combined-mode processing: decrypt and verify integrity in a single algorithm call (§3.4.4.2) | MUST | 3.4.4.2 - Combined confidentiality and integrity algorithms, inbound | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the single AEAD decrypt-and-verify call is performed by the kernel; ze projects the AEAD transform name and ICV length as SA parameters only (internal/component/ike/dataplane/xfrm_linux.go:52-58) |
| `RFC4303-2.2.1-1` | Extended Sequence Numbers (64-bit) SHOULD be implemented (§2.2.1) | SHOULD | 2.2.1 - Extended (64-bit) Sequence Number | **positive:** no positive test. **negative:** no negative test |
| `RFC4303-2.2.1-2` | ESN use MUST be negotiated by the SA management protocol (e.g., IKEv2) (§2.2.1) | MUST | 2.2.1 - Extended (64-bit) Sequence Number | **positive:** `unit/verify` [`TestEsnInitiatorRefusesAnESNValueItNeverOffered`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_esn_test.go#L152). **negative:** `unit/verify` [`TestEsnInitiatorRefusesAnESNValueItNeverOffered`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_esn_test.go#L156) |
| `RFC4303-3.4.3-4` | A window size of 64 is preferred and SHOULD be employed as the default (§3.4.3) | SHOULD | 3.4.3 - Sequence Number verification, the anti-replay section | **positive:** `unit/verify` [`TestChildSAReplayWindowDefault`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L311). **negative:** no negative test. **{single-polarity}:** a default value has no conforming negative. The requirement is met or it is not, and there is no input a receiver must refuse, so the sibling rows -3.4.3-1 and -3.4.3-2 carry the same annotation. Proven by the production Child SA path installing ReplayWin=64 on both directions (internal/component/ike/engine/child.go:62, :377, :444) |
| `RFC4303-3.3.4-3` | Tunnel-mode ESP may encapsulate a fragment (§3.3.4) | MAY | 3.3.4 - Fragmentation | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4303-2.2-1`](#rfc4303-2.2-1) Sequence Number MUST be incremented for every transmitted packet (§2.2) | no test | no test carries this requirement id; annotated {not-applicable}: the per-SA sequence counter is kernel XFRM/ESP per-packet state; ze installs the SA but never touches sequence numbers (internal/component/ike/dataplane/dataplane.go:80-108 has no sequence field) |
| [`RFC4303-2.2-2`](#rfc4303-2.2-2) Sender MUST NOT send a packet that would cause the sequence counter to cycle (overflow) (§2.2) | no test | no test carries this requirement id; annotated {not-applicable}: 32-bit counter overflow protection is enforced by the kernel XFRM datapath (it expires the state rather than wrapping); ze's control plane holds no per-packet counter |
| [`RFC4303-2.2-3`](#rfc4303-2.2-3) Counter and receiver window MUST be reset before the 2^32nd packet on a non-ESN SA (§2.2) | no test | no test carries this requirement id; annotated {not-applicable}: the sequence counter and receive window are kernel per-packet SA state; the kernel enforces the 2^32 boundary and a fresh counter/window arises from installing a new SA |
| [`RFC4303-2.4-1`](#rfc4303-2.4-1) Padding MUST bring plaintext to the block size of the cipher and ensure 4-byte alignment of the resulting ciphertext (§2.4) | no test | no test carries this requirement id; annotated {not-applicable}: ESP padding is applied by the kernel encryption datapath during packet construction; ze projects only the algorithm choice and never builds ESP payloads |
| [`RFC4303-2.6-1`](#rfc4303-2.6-1) Transmitter MUST be capable of generating dummy packets (Next Header = 59) for traffic-flow confidentiality (§2.6) | no test | no test carries this requirement id; annotated {not-applicable}: dummy/TFC packet generation is an ESP transmit-datapath function; ze emits no ESP packets, so it plays no role in producing Next-Header-59 dummies |
| [`RFC4303-2.6-2`](#rfc4303-2.6-2) Receiver MUST silently discard dummy packets (Next Header = 59) (§2.6) | no test | no test carries this requirement id; annotated {not-applicable}: inbound Next-Header-59 dummy discard occurs in the kernel ESP receive path after decryption; ze processes no inbound ESP payloads |
| [`RFC4303-3.3.3-1`](#rfc4303-3.3.3-1) Sender initializes sequence counter to 0 at SA establishment; first transmitted packet carries Sequence Number 1 (§3.3.3) | no test | no test carries this requirement id; annotated {not-applicable}: the kernel initializes the sequence counter when the SA state is added; ze installs a fresh SA with no seq field and never sets or reads the counter (internal/component/ike/dataplane/xfrm_linux.go:21-86) |
| [`RFC4303-3.3.4-1`](#rfc4303-3.3.4-1) Transport-mode ESP is applied only to whole IP datagrams, never to fragments (§3.3.4) | no test | no test carries this requirement id; annotated {not-applicable}: ordering ESP relative to fragmentation is an outbound kernel datapath decision; ze selects transport vs tunnel mode as an SA parameter but does not apply ESP to packets (internal/plugins/ospf/ipsec_install.go:413) |
| [`RFC4303-3.3.4-2`](#rfc4303-3.3.4-2) Implementation MUST support Path MTU Discovery (§3.3.4) | no test | no test carries this requirement id; annotated {not-applicable}: Path MTU Discovery is provided by the kernel networking/XFRM stack; ze's control plane installs SAs and does not run the datapath MTU machinery |
| [`RFC4303-3.4.3-3`](#rfc4303-3.4.3-3) Window advance occurs only after integrity verification succeeds (never on unverified packets) (§3.4.3) | no test | no test carries this requirement id; annotated {not-applicable}: window advancement is gated on per-packet ICV verification inside the kernel ESP receive path; ze has no control over when the window advances |
| [`RFC4303-3.4.2-1`](#rfc4303-3.4.2-1) For unicast, SA lookup uses SPI (or SPI plus protocol); invalid SA causes discard (auditable event) (§3.4.2) | no test | no test carries this requirement id; annotated {not-applicable}: inbound SAD lookup by SPI and discard/audit on miss is performed per packet by the kernel; ze installs SAs keyed by SPI but never performs the lookup (internal/plugins/ospf/ipsec_install.go:484-498 samples kernel drop counters only) |
| [`RFC4303-3.4.2-2`](#rfc4303-3.4.2-2) For multicast, destination address is also used in SA lookup (§3.4.2) | no test | no test carries this requirement id; annotated {not-applicable}: multicast SA resolution is a kernel per-packet lookup; ze only installs the wildcard state and proto-89 selector that lets the kernel resolve OSPFv3 multicast flows (internal/plugins/ospf/ipsec_install.go:415-419) |
| [`RFC4303-3.4.4.1-1`](#rfc4303-3.4.4.1-1) Separate-algorithm processing: verify ICV first, then decrypt, then check padding (§3.4.4.1) | no test | no test carries this requirement id; annotated {not-applicable}: the verify-then-decrypt-then-check-padding ordering is executed by the kernel ESP inbound datapath; ze projects the crypt+auth algorithm pair but does no packet processing (internal/component/ike/dataplane/xfrm_linux.go:59-71) |
| [`RFC4303-3.4.4.2-1`](#rfc4303-3.4.4.2-1) Combined-mode processing: decrypt and verify integrity in a single algorithm call (§3.4.4.2) | no test | no test carries this requirement id; annotated {not-applicable}: the single AEAD decrypt-and-verify call is performed by the kernel; ze projects the AEAD transform name and ICV length as SA parameters only (internal/component/ike/dataplane/xfrm_linux.go:52-58) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4303-1-1`](#rfc4303-1-1)

Integrity-only ESP MUST be offered as a service selection option and MUST be configurable via management interfaces (§1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPsecESPRequiresIntegrity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L111) | unit/verify | unproven |
| positive | [`TestIPsecESPRequiresIntegrity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L126) | unit/verify | unproven |

### [`RFC4303-2-1`](#rfc4303-2-1)

The outer protocol header that precedes the ESP header SHALL carry the value 50 in its Protocol or Next Header field (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPsecSAProtocolNumber`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L295) | unit/verify | unproven |
| positive | [`TestIPsecSAProtocolNumber`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L290) | unit/verify | unproven |

### [`RFC4303-2.1-1`](#rfc4303-2.1-1)

SPI value 0 MUST NOT appear on the wire (reserved for local use) (§2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGenerateESPSPI`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L257) | unit/verify | unproven |
| negative | [`TestIPsecSPIBoundary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L185) | unit/verify | unproven |
| positive | [`TestGenerateESPSPI`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L243) | unit/verify | unproven |
| positive | [`TestIPsecSPIBoundary`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L191) | unit/verify | unproven |

### [`RFC4303-2.1-2`](#rfc4303-2.1-2)

Whether source and destination address matching is required to map inbound traffic to an SA MUST be set by manual SA configuration or by SA management protocol negotiation (§2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPsecSAAddressMatchIndication`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L319) | unit/verify | unproven |
| positive | [`TestIPsecSAAddressMatchIndication`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L313) | unit/verify | unproven |

### [`RFC4303-2.2-1`](#rfc4303-2.2-1)

Sequence Number MUST be incremented for every transmitted packet (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4303-2.2-1, so no unit is bound to it.

### [`RFC4303-2.2-2`](#rfc4303-2.2-2)

Sender MUST NOT send a packet that would cause the sequence counter to cycle (overflow) (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4303-2.2-2, so no unit is bound to it.

### [`RFC4303-2.2-3`](#rfc4303-2.2-3)

Counter and receiver window MUST be reset before the 2^32nd packet on a non-ESN SA (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4303-2.2-3, so no unit is bound to it.

### [`RFC4303-2.4-1`](#rfc4303-2.4-1)

Padding MUST bring plaintext to the block size of the cipher and ensure 4-byte alignment of the resulting ciphertext (§2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4303-2.4-1, so no unit is bound to it.

### [`RFC4303-2.6-1`](#rfc4303-2.6-1)

Transmitter MUST be capable of generating dummy packets (Next Header = 59) for traffic-flow confidentiality (§2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4303-2.6-1, so no unit is bound to it.

### [`RFC4303-2.6-2`](#rfc4303-2.6-2)

Receiver MUST silently discard dummy packets (Next Header = 59) (§2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4303-2.6-2, so no unit is bound to it.

### [`RFC4303-3.2-1`](#rfc4303-3.2-1)

The encryption and integrity algorithms MUST NOT both be NULL; at least one ESP service is always selected (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPsecESPRequiresIntegrity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L107) | unit/verify | unproven |
| positive | [`TestIPsecESPRequiresIntegrity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L130) | unit/verify | unproven |

### [`RFC4303-3.3.3-1`](#rfc4303-3.3.3-1)

Sender initializes sequence counter to 0 at SA establishment; first transmitted packet carries Sequence Number 1 (§3.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4303-3.3.3-1, so no unit is bound to it.

### [`RFC4303-3.3.4-1`](#rfc4303-3.3.4-1)

Transport-mode ESP is applied only to whole IP datagrams, never to fragments (§3.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4303-3.3.4-1, so no unit is bound to it.

### [`RFC4303-3.3.4-2`](#rfc4303-3.3.4-2)

Implementation MUST support Path MTU Discovery (§3.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4303-3.3.4-2, so no unit is bound to it.

### [`RFC4303-3.4.3-1`](#rfc4303-3.4.3-1)

Anti-replay sliding window: minimum window size of 32 packets MUST be supported (§3.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSAReplayWindowDefault`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L308) | unit/verify | unproven |

### [`RFC4303-3.4.3-2`](#rfc4303-3.4.3-2)

Anti-replay MUST NOT be enabled unless the ESP integrity service is also enabled (§3.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSAReplayRequiresIntegrity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L334) | unit/verify | unproven |

### [`RFC4303-3.4.3-3`](#rfc4303-3.4.3-3)

Window advance occurs only after integrity verification succeeds (never on unverified packets) (§3.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4303-3.4.3-3, so no unit is bound to it.

### [`RFC4303-3.4.2-1`](#rfc4303-3.4.2-1)

For unicast, SA lookup uses SPI (or SPI plus protocol); invalid SA causes discard (auditable event) (§3.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4303-3.4.2-1, so no unit is bound to it.

### [`RFC4303-3.4.2-2`](#rfc4303-3.4.2-2)

For multicast, destination address is also used in SA lookup (§3.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4303-3.4.2-2, so no unit is bound to it.

### [`RFC4303-3.4.4.1-1`](#rfc4303-3.4.4.1-1)

Separate-algorithm processing: verify ICV first, then decrypt, then check padding (§3.4.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4303-3.4.4.1-1, so no unit is bound to it.

### [`RFC4303-3.4.4.2-1`](#rfc4303-3.4.4.2-1)

Combined-mode processing: decrypt and verify integrity in a single algorithm call (§3.4.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4303-3.4.4.2-1, so no unit is bound to it.

### [`RFC4303-2.2.1-2`](#rfc4303-2.2.1-2)

ESN use MUST be negotiated by the SA management protocol (e.g., IKEv2) (§2.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEsnInitiatorRefusesAnESNValueItNeverOffered`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_esn_test.go#L156) | unit/verify | unproven |
| positive | [`TestEsnInitiatorRefusesAnESNValueItNeverOffered`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_esn_test.go#L152) | unit/verify | unproven |

### [`RFC4303-3.4.3-4`](#rfc4303-3.4.3-4)

A window size of 64 is preferred and SHOULD be employed as the default (§3.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChildSAReplayWindowDefault`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L311) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6 phase 2, rfc4303 |
| Signed off | 2026-08-31 |
| Register | rfc2119 |
| Source | rfc/full/rfc4303.txt |
| Source fingerprint | ec4c9d4414570513 |
| Record | rfc/extraction/rfc4303.json |
| Mapped sentences | 19 |
| Declined as scope | 36 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of this Memo, copyright notice, Abstract and table of contents. The Abstract summarises the protocol and states no obligation. |
| `1` | Introduction | 2 | walked | Introduction. It states the service model and the three service combinations, and its two sites carry the integrity-only obligation this walk declares as RFC4303-1-1. The RFC 2119 key-words paragraph sits here too and binds nobody, which is why the derivation excludes it from the site inventory. |
| `2` | Packet format | 2 | walked | Packet format. Site 2:1 is the protocol number 50 obligation and site 2:2 is the backward-compatibility sentence. The field figure and the coverage prose are descriptive. |
| `2.1` | Security Parameters Index | 6 | walked | Security Parameters Index. Six sites: the unicast and multicast lookup obligations, the SAD search equivalence, the address-matching indication, and the SPI zero prohibition. |
| `2.2` | Sequence Number | 5 | walked | Sequence Number. Five sites covering the sender increment, the mandatory presence of the field, the capability to perform the section 3.3.3 and 3.4.3 processing, and the counter reset before the 2^32nd packet. |
| `2.2.1` | Extended (64-bit) Sequence Number | 1 | walked | Extended (64-bit) Sequence Number. Its one site is the negotiation MUST. The 'ESNs SHOULD be implemented' sentence in the same paragraph is advisory, so the capitalised keyword scan does not see it. |
| `2.3` | Payload Data | 4 | walked | Payload Data. Four sites, three of which bind the author of an algorithm-definition RFC and one the packet builder's header alignment. |
| `2.4` | Padding for encryption | 4 | walked | Padding for encryption. Four sites: the implementation's obligation to support padding, the default padding contents, and two obligations on the algorithm-definition RFC. |
| `2.5` | Pad Length | 0 | walked | Pad Length. A field description with a value range and a mandatory-field statement. No obligation is directed at a speaker or a receiver. |
| `2.6` | Next Header | 3 | walked | Next Header. Three sites: the value 59 convention, the transmitter and receiver dummy packet obligations fused in one sentence, and the field-presence rule for dummy packets. RFC4303-2.6-2 is the receiver half of site 2.6:2 and has no site of its own. |
| `2.7` | Traffic Flow Confidentiality padding | 2 | walked | Traffic Flow Confidentiality padding. Both sites bind a transmitter that employs TFC padding, which Ze does not implement. |
| `2.8` | Integrity Check Value | 1 | walked | Integrity Check Value. Its one site binds the integrity algorithm specification. |
| `3` | Processing | 0 | walked | Processing. A heading with no text of its own. |
| `3.1` | ESP header location | 0 | walked | ESP header location. Descriptive prose introducing the two modes. |
| `3.1.1` | Transport mode processing | 0 | walked | Transport mode processing. Diagrams and placement prose, with no capitalised keyword and no obligation stated in indicative prose either. |
| `3.1.2` | Tunnel mode processing | 0 | walked | Tunnel mode processing. Diagrams and placement prose. The outer and inner header rules it states are RFC 4301's, which the Security Architecture document owns. |
| `3.2` | Algorithms | 1 | walked | Algorithms. Its one site is the prohibition on both algorithms being NULL, declared by this walk as RFC4303-3.2-1. |
| `3.2.1` | Encryption algorithms | 0 | walked | Encryption algorithms. Descriptive, with lowercase 'must' statements binding the algorithm RFC to specify a padding modulus. |
| `3.2.2` | Integrity algorithms | 0 | walked | Integrity algorithms. Descriptive, with the same lowercase obligation on the algorithm RFC as section 3.2.1. |
| `3.2.3` | Combined mode algorithms | 0 | walked | Combined mode algorithms. Descriptive: it says a combined mode algorithm provides both services and that no payload substructure is defined. |
| `3.3` | Outbound packet processing | 0 | walked | Outbound packet processing. A heading with an ordering list and no obligation. |
| `3.3.1` | Outbound SA lookup | 0 | walked | Outbound SA lookup. It points at the Security Architecture document for how an SA is found for an outbound packet. |
| `3.3.2` | Packet encryption and ICV calculation | 0 | walked | Packet encryption and ICV calculation. A heading introducing 3.3.2.1 and 3.3.2.2. |
| `3.3.2.1` | Separate confidentiality and integrity algorithms, outbound | 3 | walked | Separate confidentiality and integrity algorithms, outbound. Three sites, all binding the ICV computation or the integrity algorithm's defining document. |
| `3.3.2.2` | Combined confidentiality and integrity algorithms, outbound | 1 | walked | Combined confidentiality and integrity algorithms, outbound. Its one site binds the combined mode algorithm's defining RFC. |
| `3.3.3` | Sequence Number generation | 2 | walked | Sequence Number generation. Two sites: the counter cycling prohibition and the manually keyed counter maintenance. RFC4303-3.3.3-1 renders the section's opening sentences, 'The sender's counter is initialized to 0 when an SA is established' and 'the first packet sent using a given SA will contain a sequence number of 1', which state the rule in indicative prose and carry no keyword for the site scan to see. |
| `3.3.4` | Fragmentation | 1 | walked | Fragmentation. Its one site is the PMTU obligation. RFC4303-3.3.4-1 renders 'Thus, transport mode ESP is applied only to whole IP datagrams (not to IP fragments)', which is indicative prose, and RFC4303-3.3.4-3 renders the tunnel mode permission in the same paragraph. |
| `3.4` | Inbound packet processing | 0 | walked | Inbound packet processing. A heading. |
| `3.4.1` | Reassembly | 1 | walked | Reassembly. Its one site is the fragment discard obligation. |
| `3.4.2` | Inbound SA lookup | 1 | walked | Inbound SA lookup. Its one site is the discard when no valid SA exists; the lookup rule itself is stated in section 2.1 and mapped from there. |
| `3.4.3` | Sequence Number verification, the anti-replay section | 6 | walked | Sequence Number verification, the anti-replay section. Six sites: support for the service, the integrity precondition, the receive counter initialisation, the duplicate check, the discard on integrity failure, and the window size floor. RFC4303-3.4.3-3 renders 'The receive window is updated only if the integrity verification succeeds', which is indicative prose with no keyword. Read at the producer before classifying: every ESP SA Ze installs carries integrity (parseESPProposal refuses a non-AEAD proposal with no hash; planStateAlgos gives ESP Crypt+Auth or AEAD), the IKE Child SA path sets a 64-packet window on both directions, and the manually keyed OSPFv3 SA sets no window at all. The window check, the duplicate check and the ICV verification run in the kernel. |
| `3.4.4` | ICV verification | 0 | walked | ICV verification. A heading introducing 3.4.4.1 and 3.4.4.2. |
| `3.4.4.1` | Separate confidentiality and integrity algorithms, inbound | 2 | walked | Separate confidentiality and integrity algorithms, inbound. Two sites: the discard on ICV mismatch and the ordering rule that integrity completes before the decrypted packet goes on. |
| `3.4.4.2` | Combined confidentiality and integrity algorithms, inbound | 1 | walked | Combined confidentiality and integrity algorithms, inbound. Its one site is the discard on integrity failure. RFC4303-3.4.4.2-1 renders step 1 of the numbered list, 'Decrypts and integrity checks the ESP Payload Data ... using the key, algorithm, algorithm mode', which is indicative prose. |
| `4` | Auditing | 1 | walked | Auditing. Its one site binds the ESP implementation that processes packets and the audit subsystem of the system that hosts it. |
| `5` | Conformance requirements | 4 | walked | Conformance requirements. Four sites: the conformance umbrella and the cross-reference to RFC 4301, the multicast restatement, the NULL integrity negotiation for a confidentiality-only implementation, and the both-NULL prohibition restated. |
| `6` | Security Considerations | 0 | walked | Security Considerations. Two sentences saying security permeates the specification and pointing at the Security Architecture document. No countermeasure is directed at a speaker. |
| `7` | Differences from RFC 2406 | 1 | walked | Differences from RFC 2406. A change log; its one site reports a level change rather than stating an obligation. |
| `8` | Backward compatibility considerations | 0 | walked | Backward compatibility considerations. It explains that ESP has no version number and what an ESP v2 receiver may do with Next Header 59. Every statement is indicative and the normative half is site 2:2 in section 2. |
| `9` | Acknowledgements | 0 | skipped (acknowledgements) | Acknowledgements. |
| `10` | References heading | 0 | skipped (references) | References heading. |
| `10.1` | not stated | 0 | skipped (references) | Normative References: RFC 2119, RFC 4301, RFC 4302, RFC 2434 and the ESP algorithm requirements document. |
| `10.2` | not stated | 0 | walked | The derived span carries the Informative References and Appendix A, Extended (64-bit) Sequence Numbers. The appendix describes the receiver's ESN window management and its optional resynchronisation heuristic. It states no MUST-level obligation: no capitalised MUST, SHALL or REQUIRED appears anywhere after the section 10 heading. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `2:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The obligation binds the deployment that has ESP version-compatibility concerns, and the remedy it names is a deployment choice: a signaling mechanism between the peers or an out-of-band configuration mechanism. Ze implements no ESP version field and no version signaling of its own. It provides both named mechanisms, IKEv2 Child SA negotiation (internal/component/ike/engine/child.go) and manual OSPFv3 configuration (internal/plugins/ospf/ipsec_install.go), so an operator on Ze always has one. | ESP does not contain a version number, therefore if there are concerns about backward compatibility, they MUST be addressed by using a signaling mechanism between the two IPsec peers to ensure compatible versions of ESP (e.g., Internet Key Exchange (IKEv2) [Kau05]) or an out-of-band configuration mechanism. |
| `2.1:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the inbound SAD demultiplexer, which is the ESP receive datapath. Ze installs SA parameters (dataplane.SAParams) into the Linux kernel XFRM state (xfrmStateFromParams, internal/component/ike/dataplane/xfrm_linux.go) and builds, pads, encrypts or parses no ESP packet. SPI collision handling happens inside the kernel lookup. | A multicast-capable IPsec implementation MUST correctly de-multiplex inbound traffic even in the context of SPI collisions. |
| `2.1:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the implementation of the SAD search itself, including any acceleration of it. Ze installs SA parameters (dataplane.SAParams) into the Linux kernel XFRM state (xfrmStateFromParams, internal/component/ike/dataplane/xfrm_linux.go) and builds, pads, encrypts or parses no ESP packet. | In practice, an implementation MAY choose any method to accelerate this search, although its externally visible behavior MUST be functionally equivalent to having searched the SAD in the above order. |
| `2.2:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the ESP transmit datapath that builds the header. Ze installs SA parameters (dataplane.SAParams) into the Linux kernel XFRM state (xfrmStateFromParams, internal/component/ike/dataplane/xfrm_linux.go) and builds, pads, encrypts or parses no ESP packet. | The field is mandatory and MUST always be present even if the receiver does not elect to enable the anti-replay service for a specific SA. |
| `2.2:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The sentence adds no obligation of its own. It says an ESP implementation must be capable of the processing described in sections 3.3.3 and 3.4.3, whose obligations this walk already accounts for: RFC4303-3.3.3-1, RFC4303-2.2-2 and the RFC4303-3.4.3 rows. | Processing of the Sequence Number field is at the discretion of the receiver, but all ESP implementations MUST be capable of performing the processing described in Sections 3.3.3 and 3.4.3. |
| `2.2:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the ESP transmit datapath that builds the header, as site 2.2:2 does. Ze installs SA parameters (dataplane.SAParams) into the Linux kernel XFRM state (xfrmStateFromParams, internal/component/ike/dataplane/xfrm_linux.go) and builds, pads, encrypts or parses no ESP packet. | Thus, the sender MUST always transmit this field, but the receiver need not act upon it (see the discussion of Sequence Number Verification in the "Inbound Packet Processing" section (3.4.3) below). |
| `2.3:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the author of the RFC that specifies how an encryption algorithm is used with ESP: that document must state the length, structure and location of explicit per-packet synchronization data. Ze writes no algorithm specification. The role is a document author, so no producer could act as it. Ze CONSUMES such a specification: `xfrmStateFromParams` (`internal/component/ike/dataplane/xfrm_linux.go`) projects the negotiated algorithm and key onto the SA it installs, and the kernel applies whatever that document defined. | (See Figure 2.) Any encryption algorithm that requires such explicit, per-packet synchronization data MUST indicate the length, any structure for such data, and the location of this data as part of an RFC specifying how the algorithm is used with ESP. |
| `2.3:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the author of an algorithm-definition RFC, as site 2.3:1 does: implicit synchronization data must be derived by an algorithm the defining RFC states. The role is a document author, so no producer could act as it. Ze CONSUMES such a specification: `xfrmStateFromParams` (`internal/component/ike/dataplane/xfrm_linux.go`) projects the negotiated algorithm and key onto the SA it installs, and the kernel applies whatever that document defined. | See Figure 2.) If such synchronization data is implicit, the algorithm for deriving the data MUST be part of the algorithm definition RFC. |
| `2.3:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the ESP packet builder, which must align the next layer protocol header relative to the ESP header. Ze installs SA parameters (dataplane.SAParams) into the Linux kernel XFRM state (xfrmStateFromParams, internal/component/ike/dataplane/xfrm_linux.go) and builds, pads, encrypts or parses no ESP packet. | Note that the beginning of the next layer protocol header MUST be aligned relative to the beginning of the ESP header as follows. |
| `2.3:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the author of an algorithm specification, which must say how ciphertext alignment is achieved when the receiver reads the IV separately. The role is a document author, so no producer could act as it. Ze CONSUMES such a specification: `xfrmStateFromParams` (`internal/component/ike/dataplane/xfrm_linux.go`) projects the negotiated algorithm and key onto the SA it installs, and the kernel applies whatever that document defined. | In these cases, the algorithm specification MUST address how alignment of the (real) ciphertext is to be achieved. |
| `2.4:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the ESP transmit datapath, which fills the padding bytes with the default monotonically increasing series when the algorithm states no contents. Ze installs SA parameters (dataplane.SAParams) into the Linux kernel XFRM state (xfrmStateFromParams, internal/component/ike/dataplane/xfrm_linux.go) and builds, pads, encrypts or parses no ESP packet. | If Padding bytes are needed but the encryption algorithm does not specify the padding contents, then the following default processing MUST be used. |
| `2.4:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the author of the RFC that defines how an encryption or combined mode algorithm is employed with ESP, which must specify any constraint on padding byte values. The role is a document author, so no producer could act as it. Ze CONSUMES such a specification: `xfrmStateFromParams` (`internal/component/ike/dataplane/xfrm_linux.go`) projects the negotiated algorithm and key onto the SA it installs, and the kernel applies whatever that document defined. | If an encryption or combined mode algorithm imposes constraints on the values of the bytes used for padding, they MUST be specified by the RFC defining how the algorithm is employed with ESP. |
| `2.4:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the same algorithm-definition RFC as site 2.4:3, which must say whether padding values are checked. The role is a document author, so no producer could act as it. Ze CONSUMES such a specification: `xfrmStateFromParams` (`internal/component/ike/dataplane/xfrm_linux.go`) projects the negotiated algorithm and key onto the SA it installs, and the kernel applies whatever that document defined. | If the algorithm requires checking of the values of the bytes used for padding, this too MUST be specified in that RFC. |
| `2.6:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The value convention and the capability are one obligation stated twice in adjacent sentences. RFC4303-2.6-1 names the value 59 and the dummy packet, and site 2.6:2 maps it. | To facilitate the rapid generation and discarding of the padding traffic in support of traffic flow confidentiality (see Section 2.4), the protocol value 59 (which means "no next header") MUST be used to designate a "dummy" packet. |
| `2.6:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the ESP transmit datapath that constructs a dummy packet and fills its header and trailer fields. Ze installs SA parameters (dataplane.SAParams) into the Linux kernel XFRM state (xfrmStateFromParams, internal/component/ike/dataplane/xfrm_linux.go) and builds, pads, encrypts or parses no ESP packet. | All other ESP header and trailer fields (SPI, Sequence Number, Padding, Pad Length, Next Header, and ICV) MUST be present in dummy packets, but the plaintext portion of the payload, other than this Next Header field, need not be well-formed, e.g., the rest of the Payload Data may consist of only random bytes. |
| `2.7:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds an ESP transmitter that adds TFC padding. Ze implements none: dataplane.SAParams carries no TFC padding field and xfrmStateFromParams sets none, so no SA Ze installs can add TFC padding or modify a datagram length field. The producer that would act as it if ze did is `xfrmStateFromParams` (`internal/component/ike/dataplane/xfrm_linux.go`), which builds every SA ze installs and sets no TFC padding field on any of them. | (ESP trailer fields are located by counting back from the end of the ESP packet.) Accordingly, if TFC padding is added, the field containing the specification of the length of the IP datagram MUST NOT be modified to reflect this padding. |
| `2.7:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the SA management protocol only once a transmitter employs TFC padding, and Ze implements no such transmitter (site 2.7:1). dataplane.SAParams carries no TFC padding field, so no Ze-installed SA can employ the service that would trigger the negotiation. The producer that would act as it if ze did is `xfrmStateFromParams` (`internal/component/ike/dataplane/xfrm_linux.go`), which builds every SA ze installs and sets no TFC padding field on any of them. | However, because receivers may not have been prepared to deal with this padding, the SA management protocol MUST negotiate this service prior to a transmitter employing it, to ensure backward compatibility. |
| `2.8:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the author of an integrity algorithm specification, which must state the ICV length and the comparison rules. Ze writes no algorithm specification; it projects the negotiated algorithm name and key onto the SA. The role is a document author, so no producer could act as it. Ze CONSUMES such a specification: `xfrmStateFromParams` (`internal/component/ike/dataplane/xfrm_linux.go`) projects the negotiated algorithm and key onto the SA it installs, and the kernel applies whatever that document defined. | The integrity algorithm specification MUST specify the length of the ICV and the comparison rules and processing steps for validation. |
| `3.3.2.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the ESP transmit datapath that computes the ICV and appends implicit padding to reach the integrity algorithm's block size. Ze installs SA parameters (dataplane.SAParams) into the Linux kernel XFRM state (xfrmStateFromParams, internal/component/ike/dataplane/xfrm_linux.go) and builds, pads, encrypts or parses no ESP packet. | If the length of ESP packet (as described above) does not match the block size requirements for the algorithm, implicit padding MUST be appended to the end of the ESP packet. |
| `3.3.2.1:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the ESP implementation that computes the ICV, which must read the integrity algorithm's defining document to learn whether implicit padding applies. Ze installs SA parameters (dataplane.SAParams) into the Linux kernel XFRM state (xfrmStateFromParams, internal/component/ike/dataplane/xfrm_linux.go) and builds, pads, encrypts or parses no ESP packet. | The document that defines an integrity algorithm MUST be consulted to determine if implicit padding is required as described above. |
| `3.3.2.1:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the ESP transmit datapath, which must use zero-valued implicit padding octets when the algorithm states no contents. Ze installs SA parameters (dataplane.SAParams) into the Linux kernel XFRM state (xfrmStateFromParams, internal/component/ike/dataplane/xfrm_linux.go) and builds, pads, encrypts or parses no ESP packet. | If the document does not specify an answer to this question, then the default is to assume that implicit padding is required (as needed to match the packet length to the algorithm's block size.) If padding bytes are needed but the algorithm does not specify the padding contents, then the padding octets MUST have a value of zero. |
| `3.3.2.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the author of the RFC that defines the use of a combined mode algorithm with ESP, which must state where the integrity fields sit and how the SPI and Sequence Number enter the computation. The role is a document author, so no producer could act as it. Ze CONSUMES such a specification: `xfrmStateFromParams` (`internal/component/ike/dataplane/xfrm_linux.go`) projects the negotiated algorithm and key onto the SA it installs, and the kernel applies whatever that document defined. | The location of any integrity fields, and the means by which the Sequence Number and SPI are included in the integrity computation, MUST be defined in an RFC that defines the use of the combined mode algorithm with ESP. |
| `3.3.3:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the ESP transmit datapath that maintains the per-SA sequence counter across reboots, and only once a user employs anti-replay with a manually keyed SA. Ze enables neither half: buildIPsecSA (internal/plugins/ospf/ipsec_install.go) sets no ReplayWin on the manually keyed OSPFv3 SA, so anti-replay is off there, and the counter itself is kernel state. Ze installs SA parameters (dataplane.SAParams) into the Linux kernel XFRM state (xfrmStateFromParams, internal/component/ike/dataplane/xfrm_linux.go) and builds, pads, encrypts or parses no ESP packet. | If a user chooses to employ anti-replay in conjunction with SAs that are manually keyed, the sequence number counter at the sender MUST be correctly maintained across local reboots, etc., until the key is replaced. |
| `3.4.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the ESP receive datapath, which discards a packet that arrives as an IP fragment. Ze installs SA parameters (dataplane.SAParams) into the Linux kernel XFRM state (xfrmStateFromParams, internal/component/ike/dataplane/xfrm_linux.go) and builds, pads, encrypts or parses no ESP packet. | If a packet offered to ESP for processing appears to be an IP fragment, i.e., the OFFSET field is non-zero or the MORE FRAGMENTS flag is set, the receiver MUST discard the packet; this is an auditable event. |
| `3.4.3:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Supporting the anti-replay service and supporting its sliding window are the same obligation at two grains, and RFC4303-3.4.3-1 is proven by the evidence this sentence would need: the production Child SA path installs a 64-packet window on both directions (internal/component/ike/engine/child.go). A second row proven by the same assertion would be one fact declared twice. | All ESP implementations MUST support the anti-replay service, though its use may be enabled or disabled by the receiver on a per-SA basis. |
| `3.4.3:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the kernel XFRM replay state. Ze sets only the window size on an installed state (state.ReplayWindow from p.ReplayWin, xfrm_linux.go lines 132 to 133) and carries no replay counter into a new state, so a state Ze adds starts with a zero receive counter and Ze has no way to set a non-zero one. | If the receiver has enabled the anti-replay service for this SA, the receive packet counter for the SA MUST be initialized to zero when the SA is established. |
| `3.4.3:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the ESP receive datapath, which checks each arriving packet's Sequence Number against the window. Ze projects the window size that makes the kernel perform this check (replayWindow = 64, internal/component/ike/engine/child.go) and never sees a packet. Ze installs SA parameters (dataplane.SAParams) into the Linux kernel XFRM state (xfrmStateFromParams, internal/component/ike/dataplane/xfrm_linux.go) and builds, pads, encrypts or parses no ESP packet. | For each received packet, the receiver MUST verify that the packet contains a Sequence Number that does not duplicate the Sequence Number of any other packets received during the life of this SA. |
| `3.4.3:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the ESP receive datapath, which verifies the ICV and discards the datagram when verification fails. Ze projects the integrity algorithm and key onto the SA (planStateAlgos, internal/component/ike/dataplane/dataplane.go) and verifies no ICV. Ze installs SA parameters (dataplane.SAParams) into the Linux kernel XFRM state (xfrmStateFromParams, internal/component/ike/dataplane/xfrm_linux.go) and builds, pads, encrypts or parses no ESP packet. | In either case, if the integrity check fails, the receiver MUST discard the received IP datagram as invalid; this is an auditable event. |
| `3.4.4.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the ESP receive datapath, which discards a datagram whose computed and received ICVs differ. Ze installs SA parameters (dataplane.SAParams) into the Linux kernel XFRM state (xfrmStateFromParams, internal/component/ike/dataplane/xfrm_linux.go) and builds, pads, encrypts or parses no ESP packet. | If the test fails, then the receiver MUST discard the received IP datagram as invalid; this is an auditable event. |
| `3.4.4.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the ESP receive datapath, which discards a datagram when the combined mode algorithm reports an integrity failure. Ze projects the AEAD transform name and ICV length as SA parameters (planStateAlgos returns AEAD for a combined mode cipher) and runs no algorithm. Ze installs SA parameters (dataplane.SAParams) into the Linux kernel XFRM state (xfrmStateFromParams, internal/component/ike/dataplane/xfrm_linux.go) and builds, pads, encrypts or parses no ESP packet. | If the integrity check performed by the combined mode algorithm fails, the receiver MUST discard the received IP datagram as invalid; this is an auditable event. |
| `4:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the ESP implementation that processes packets, whose auditable events are the five this section lists: no valid SA, an IP fragment offered to ESP, sequence number overflow, an anti-replay failure and an integrity failure. Every one of them occurs in the kernel receive or transmit path, so the kernel audit subsystem is the auditor. Ze samples the kernel's XFRM drop counters for its own metrics (readXfrmDrops, internal/plugins/ospf/ipsec_install.go), which is a report of the kernel's decisions and not the ESP audit trail this sentence requires. | However, if ESP is incorporated into a system that supports auditing, then the ESP implementation MUST also support auditing and MUST allow a system administrator to enable or disable auditing for ESP. |
| `5:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The sentence's own added obligation is compliance with the packet processing requirements of the Security Architecture document [Ken-Arch], which is RFC 4301. That RFC is enrolled and summarised at rfc/short/rfc4301.md and carries its own requirement rows. The first clause is a conformance umbrella over this document's own obligations, each of which this walk maps or excludes on its own site. | Implementations that claim conformance or compliance with this specification MUST implement the ESP syntax and processing described here for unicast traffic, and MUST comply with all additional packet processing requirements levied by the Security Architecture document [Ken-Arch]. |
| `5:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The conformance section restates the multicast obligation captured as RFC4303-3.4.2-2, which site 2.1:2 maps. | Additionally, if an implementation claims to support multicast traffic, it MUST comply with the additional requirements specified for support of such traffic. |
| `5:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds an implementation that offers the confidentiality-only ESP service, which Ze does not offer. parseESPProposal (internal/component/ike/ipsec/config.go) refuses a non-AEAD ESP proposal that carries no hash, and validateIPsecInterface (internal/plugins/ospf/config_ipsec.go) refuses an ESP interface with no integrity algorithm and key, so no configuration reaches the antecedent of this sentence. | If an implementation offers this service, it MUST also support the negotiation of the "NULL" integrity algorithm. |
| `5:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The conformance section restates the both-NULL prohibition captured as RFC4303-3.2-1, which site 3.2:1 maps. | NOTE that although integrity and encryption may each be "NULL" under the circumstances noted above, they MUST NOT both be "NULL". |
| `7:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 7 is the change log against RFC 2406. The keywords appear inside a description of what this revision changed, 'Confidentiality-only service -- now a MAY, not a MUST', so the sentence reports a level rather than stating one. The obligations it points at are stated normatively in sections 1, 2.1, 2.2.1, 2.6, 2.7 and 3.4.4. | o Confidentiality-only service -- now a MAY, not a MUST. o SPI -- modified to specify a uniform algorithm for SAD lookup for unicast and multicast SAs, covering a wider range of multicast technologies. |

## Superseded

No document obsoletes RFC 4303, so its obligations are stated where they were written.
