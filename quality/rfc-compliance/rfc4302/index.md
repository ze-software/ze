# RFC 4302 - IP Authentication Header

Partial. Every requirement this repository extracted from RFC 4302, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 28.1% | 9 of 32 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 32 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 32 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 100.0% | 18 of 18 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 32 | of 44 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 32 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 32 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 62.5% | 20 of 32 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 3.1% | 1 of 32 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 6.2% | 2 of 32 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 9 | of 32 gated MUSTs judged | 5 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 32 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 44 |
| Gated MUST-level | 32 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 2 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 18 |
| Tagged units | 18 |
| Recorded audit verdicts | 9 |
| Discrimination records | 18 |
| Summary | `rfc/short/rfc4302.md` |
| Requirement shard | `rfc/requirements/rfc4302.md` |
| RFC text | `rfc/full/rfc4302.txt` |

## Enrolment

Enrolled: IP Authentication Header (RFC 4302): control plane only. Ze speaks AH through RFC 4552 manually-keyed IPsec for OSPFv3: validateIPsecInterface (plugins/ospf/config_ipsec.go) accepts protocol ah with an SPI at or above 256, an HMAC-SHA algorithm and a hex key of that algorithm length, and refuses an encryption key beside AH; buildIPsecSA and buildIPsecPolicies (plugins/ospf/ipsec_install.go) install one transport-mode SA with Proto ProtoAH and a {::/0, ::/0, proto 89} state selector plus out/in/fwd policies scoped to the interface; planStateAlgos (ike/dataplane/dataplane.go) gives an AH state an integrity transform and no encryption transform, and xfrmStateFromParams (ike/dataplane/xfrm_linux.go) writes it. Every per-packet AH obligation -- header construction, sequence numbers, mutable-field zeroing, ICV, replay window, discard on failure -- is performed by Linux XFRM, which the whole-stack conformance ruling of 2026-08-31 counts as an implementation of the obligation rather than an exemption. Ze never negotiates an AH Child SA over IKEv2, so every AH SA it installs is manually keyed and carries no replay window unless the interface's replay-window leaf asks for one, which defaults to 0 as Section 5 asks. 34 gated rows, none tested and none annotated at enrolment; the interop scenario ospf-ipsec-ah-frr already reads the installed kernel state and is the carrier the coverage work will tag.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

AH algorithm planning and RFC 4552 OSPFv3 use.

**What the ledger says remains:**

Scoped to configured manual IPsec support. No counter reset before sequence-number rollover, and no AH auditing surface.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 9 | one part of the gated population |
| Annotated instead of tested | 23 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 2 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **32** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (9):** [`RFC4302-2-1`](#rfc4302-2-1), [`RFC4302-2-2`](#rfc4302-2-2), [`RFC4302-2.4-1`](#rfc4302-2.4-1), [`RFC4302-2.4-5`](#rfc4302-2.4-5), [`RFC4302-2.4-6`](#rfc4302-2.4-6), [`RFC4302-3.3.4-1`](#rfc4302-3.3.4-1), [`RFC4302-3.4.2-1`](#rfc4302-3.4.2-1), [`RFC4302-3.4.3-1`](#rfc4302-3.4.3-1), [`RFC4302-3.4.3-5`](#rfc4302-3.4.3-5)

**Annotated instead of tested (23):** [`RFC4302-2.3-1`](#rfc4302-2.3-1), [`RFC4302-2.4-2`](#rfc4302-2.4-2), [`RFC4302-2.4-3`](#rfc4302-2.4-3), [`RFC4302-2.4-4`](#rfc4302-2.4-4), [`RFC4302-2.5-1`](#rfc4302-2.5-1), [`RFC4302-2.5-2`](#rfc4302-2.5-2), [`RFC4302-2.5-3`](#rfc4302-2.5-3), [`RFC4302-2.5-5`](#rfc4302-2.5-5), [`RFC4302-2.5.1-1`](#rfc4302-2.5.1-1), [`RFC4302-2.6-1`](#rfc4302-2.6-1), [`RFC4302-3.3.2-1`](#rfc4302-3.3.2-1), [`RFC4302-3.3.3.1-1`](#rfc4302-3.3.3.1-1), [`RFC4302-3.3.3.2.2-1`](#rfc4302-3.3.3.2.2-1), [`RFC4302-3.3.3.2.2-2`](#rfc4302-3.3.3.2.2-2), [`RFC4302-3.3.3.2.2-3`](#rfc4302-3.3.3.2.2-3), [`RFC4302-3.3.3.2.2-4`](#rfc4302-3.3.3.2.2-4), [`RFC4302-3.4.1-1`](#rfc4302-3.4.1-1), [`RFC4302-3.4.3-2`](#rfc4302-3.4.3-2), [`RFC4302-3.4.3-3`](#rfc4302-3.4.3-3), [`RFC4302-3.4.3-4`](#rfc4302-3.4.3-4), [`RFC4302-4-1`](#rfc4302-4-1), [`RFC4302-A2-1`](#rfc4302-a2-1), [`RFC4302-A2-2`](#rfc4302-a2-2)

**Derived from other rows (2):** [`RFC4302-5-1`](#rfc4302-5-1), [`RFC4302-5-2`](#rfc4302-5-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4302-2-1` | The protocol header (IPv4, IPv6, or IPv6 Extension) immediately preceding the AH header SHALL contain the value 51 in its Protocol (IPv4) or Next Header (IPv6, Extension) fields [DH98]. (§2) | SHALL | 2 | **positive:** `unit/verify` [`TestIPsecSAProtocolNumber`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L370). **negative:** `unit/verify` [`TestIPsecSAProtocolNumber`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L375) |
| `RFC4302-2-2` | AH does not contain a version number, therefore if there are concerns about backward compatibility, they MUST be addressed by using a signaling mechanism between the two IPsec peers to ensure compatible versions of AH, e.g., IKE [IKEv2] or an out-of-band configuration mechanism. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestAHParametersComeFromConfiguration`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_rfc4302_test.go#L171). **negative:** `unit/verify` [`TestAHParametersComeFromConfiguration`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_rfc4302_test.go#L179) |
| `RFC4302-2.3-1` | It MUST be set to "zero" by the sender (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM; internal/plugins/ospf/ipsec_install.go::buildIPsecSA installs the transport-mode AH SA, and the kernel's AH output builds every AH header from it, so no value Ze writes decides this field |
| `RFC4302-2.4-1` | The SPI field is mandatory, and this mechanism for mapping inbound traffic to unicast SAs described above MUST be supported by all AH implementations. (§2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestAHSAIdentifiedBySPIAndProtocol`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_rfc4302_test.go#L37). **negative:** `unit/verify` [`TestAHSAIdentifiedBySPIAndProtocol`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_rfc4302_test.go#L46) |
| `RFC4302-2.4-2` | If an IPsec implementation supports multicast, then it MUST support multicast SAs using the algorithm below for mapping inbound IPsec datagrams to SAs. (§2.4) | MUST | 2.4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams writes each SAD entry with its SPI and its address selector, and Linux XFRM maps an inbound AH datagram to an entry in the longest-match order this section gives; ze performs no SAD search of its own |
| `RFC4302-2.4-3` | A multicast-capable IPsec implementation MUST correctly de-multiplex inbound traffic even in the context of SPI collisions. (§2.4) | MUST | 2.4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams writes a state Linux XFRM keys by destination, SPI and protocol, and Linux XFRM de-multiplexes on that key, so a group SA and a unicast SA that share an SPI stay distinct entries |
| `RFC4302-2.4-4` | any method to accelerate this search, although its externally visible behavior MUST be functionally equivalent to having searched the SAD in the above order. (§2.4) | MUST | 2.4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams installs the SAD entries, and Linux XFRM owns both the search and whatever hashing accelerates it, so the equivalence this sentence demands is the kernel's to keep |
| `RFC4302-2.4-5` | The indication of whether source and destination address matching is required to map inbound IPsec traffic to SAs MUST be set either as a side effect of manual SA configuration or via negotiation using an SA management protocol, e.g., IKE or Group Domain of Interpretation (GDOI) [RFC3547]. (§2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestAHSAAddressMatchIndication`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_rfc4302_test.go#L69). **negative:** `unit/verify` [`TestAHSAAddressMatchIndication`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_rfc4302_test.go#L77) |
| `RFC4302-2.4-6` | The SPI value of zero (0) is reserved for local, implementation-specific use and MUST NOT be sent on the wire. (§2.4) | MUST NOT | 2.4 | **positive:** `unit/verify` [`TestAHSPIReservedRangeRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_rfc4302_test.go#L99). **negative:** `unit/verify` [`TestAHSPIReservedRangeRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_rfc4302_test.go#L94) |
| `RFC4302-2.5-1` | For a unicast SA or a single-sender multicast SA, the sender MUST increment this field for every transmitted packet. (§2.5) | MUST | 2.5 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams writes the XFRM state that carries the sequence counter, and the kernel increments it for each packet it sends on the SA; Ze sees no AH packet |
| `RFC4302-2.5-2` | The field is mandatory and MUST always be present even if the receiver does not elect to enable the anti-replay service for a specific SA. (§2.5) | MUST | 2.5 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams writes the XFRM state, and the kernel emits the Sequence Number on every AH packet whether or not the state carries a replay window |
| `RFC4302-2.5-3` | Processing of the Sequence Number field is at the discretion of the receiver, but all AH implementations MUST be capable of performing the processing described in Section 3.3.2, "Sequence Number Generation", and Section 3.4.3, "Sequence Number Verification". (§2.5) | MUST | 2.5 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams writes the state for both directions of the SA, and the kernel generates the sequence numbers it sends and verifies the ones it receives |
| `RFC4302-2.5-5` | Thus, the sender's counter and the receiver's counter MUST be reset (by establishing a new SA and thus a new key) prior to the transmission of the 2^32nd packet on an SA. (§2.5) | MUST | 2.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nothing resets the counters before the 2^32nd packet. The OSPFv3 SA is manually keyed and ze never rekeys it: setConfig installs one SPI and key per interface (internal/plugins/ospf/ipsec_install.go) and no code establishes a replacement SA, so an operator who enables anti-replay with the replay-window leaf gets an SA that stops sending at the rollover rather than one that is re-established. Disclosed in docs/features/rfc-status.md RFC 4302 row |
| `RFC4302-2.5.1-1` | Use of an Extended Sequence Number (ESN) MUST be negotiated by an SA management protocol. (§2.5.1) | MUST | 2.5.1 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "a new option for sequence numbers SHOULD be offered, as an extension to the current, 32-bit sequence number field"; ze offers no ESN, so nothing ever uses one. internal/plugins/ospf/ipsec_install.go::buildIPsecSA is the only code that creates an AH SA, and it builds one manually keyed transport-mode state from static interface configuration with no ESN in it; espProposalToWire, internal/component/ike/engine/initiator.go, proposes ESP alone and keys Transform Type 5 to espESNNotExtended, so ze runs no SA management protocol that could negotiate an AH ESN |
| `RFC4302-2.6-1` | All implementations MUST support such padding and MUST insert only enough padding to satisfy the IPv4/IPv6 alignment requirements. (§2.6) | MUST | 2.6 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams sets the integrity transform and its truncation length, and the kernel's AH output sizes the ICV field and pads it to the alignment the address family needs |
| `RFC4302-3.3.2-1` | In other words, the sender MUST NOT send a packet on an SA if doing so would cause the sequence number to cycle. (§3.3.2) | MUST NOT | 3.3.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams writes the per-SA replay window the interface replay-window leaf asks for, and Linux XFRM refuses to send a packet on that state once the sequence number would cycle; ze emits no AH packet of its own |
| `RFC4302-3.3.3.1-1` | If a field may be modified during transit, the value of the field is set to zero for purposes of the ICV computation. If a field is mutable, but its value at the (IPsec) receiver is predictable, then that value is inserted into the field for purposes of the ICV calculation. The Integrity Check Value field is also set to zero in preparation for this computation. (§3.3.3.1) | MUST | 3.3.3.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM; internal/plugins/ospf/ipsec_install.go::buildIPsecSA installs the transport-mode AH SA, and the kernel's AH code zeroes the mutable fields and the ICV field before it computes the ICV |
| `RFC4302-3.3.3.2.2-1` | If the IP packet length (including AH and the 32 high-order bits of the ESN, if enabled) does not match the blocksize requirements for the algorithm, implicit padding MUST be appended to the end of the packet, prior to ICV computation. (§3.3.3.2.2) | MUST | 3.3.3.2.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams names the integrity algorithm, and the kernel's crypto layer appends whatever implicit padding that algorithm's blocksize needs |
| `RFC4302-3.3.3.2.2-2` | The padding octets MUST have a value of zero. (§3.3.3.2.2) | MUST | 3.3.3.2.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams names the integrity algorithm, and the kernel's crypto layer chooses the value of the implicit padding octets |
| `RFC4302-3.3.3.2.2-3` | The document that defines an integrity algorithm MUST be consulted to determine if implicit padding is required as described above. (§3.3.3.2.2) | MUST | 3.3.3.2.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams names the integrity algorithm, and the kernel's implementation of that algorithm decides whether implicit padding is required |
| `RFC4302-3.3.3.2.2-4` | If the document does not specify an answer to this, then the default is to assume that implicit padding is required (as needed to match the packet length to the algorithm's blocksize.) If padding bytes are needed but the algorithm does not specify the padding contents, then the padding octets MUST have a value of zero. (§3.3.3.2.2) | MUST | 3.3.3.2.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams names the integrity algorithm, and the kernel's implementation of that algorithm supplies the default when the algorithm gives no answer |
| `RFC4302-3.3.4-1` | In any case, an AH implementation MUST support generation of ICMP PMTU messages (or equivalent internal signaling for native host implementations) to minimize the likelihood of fragmentation. (§3.3.4) | MUST | 3.3.4 | **positive:** `unit/verify` [`TestAHPathMTUIsSignaledToTheLocalSender`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_pmtu_integration_linux_test.go#L133). **negative:** `unit/verify` [`TestAHPathMTUIsSignaledToTheLocalSender`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_pmtu_integration_linux_test.go#L143) |
| `RFC4302-3.4.1-1` | If a packet offered to AH for processing appears to be an IP fragment, i.e., the OFFSET field is nonzero or the MORE FRAGMENTS flag is set, the receiver MUST discard the packet; this is an auditable event. (§3.4.1) | MUST | 3.4.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM; internal/plugins/ospf/ipsec_install.go::buildIPsecPolicies installs the inbound require-policy that puts OSPF traffic through the kernel's AH input, which is where a packet offered to AH that is an IP fragment is dropped |
| `RFC4302-3.4.2-1` | If no valid Security Association exists for this packet the receiver MUST discard the packet; this is an auditable event. (§3.4.2) | MUST | 3.4.2 | **positive:** `unit/verify` [`TestAHInboundRequirePolicyInstalled`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_rfc4302_test.go#L122). **negative:** `unit/verify` [`TestAHInboundRequirePolicyInstalled`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_rfc4302_test.go#L130) |
| `RFC4302-3.4.3-1` | All AH implementations MUST support the anti-replay service, though its use may be enabled or disabled by the receiver on a per-SA basis. (§3.4.3) | MUST | 3.4.3 | **positive:** `unit/verify` [`TestIPsecSAReplayWindow`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L434). **negative:** `unit/verify` [`TestIPsecSAReplayWindow`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L439) |
| `RFC4302-3.4.3-2` | If the receiver has enabled the anti-replay service for this SA, the receive packet counter for the SA MUST be initialized to zero when the SA is established. (§3.4.3) | MUST | 3.4.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams writes the state and presets no replay counter, and Linux XFRM initializes the receive packet counter to zero as it creates the state |
| `RFC4302-3.4.3-3` | For each received packet, the receiver MUST verify that the packet contains a Sequence Number that does not duplicate the Sequence Number of any other packets received during the life of this SA. (§3.4.3) | MUST | 3.4.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams writes the replay window the receiver asked for, and Linux XFRM checks every received sequence number against that window and rejects a duplicate |
| `RFC4302-3.4.3-4` | If the ICV validation fails, the receiver MUST discard the received IP datagram as invalid. (§3.4.3) | MUST | 3.4.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams installs the integrity transform the kernel validates the ICV with, and the kernel discards the datagram when that validation fails; Ze holds no rejecting branch of its own |
| `RFC4302-3.4.3-5` | A MINIMUM window size of 32 packets MUST be supported (§3.4.3) | MUST | 3.4.3 | **positive:** `unit/verify` [`TestIPsecReplayWindowRange`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L256). **negative:** `unit/verify` [`TestIPsecReplayWindowRange`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L260) |
| `RFC4302-4-1` | However, if AH is incorporated into a system that supports auditing, then the AH implementation MUST also support auditing and MUST allow a system administrator to enable or disable auditing for AH. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze supports auditing for AH nowhere. readXfrmDropsPlatform (internal/plugins/ospf/ipsec_drops_linux.go) polls the node-global /proc/net/xfrm_stat counters into one OSPF metric, which carries no SPI, no per-SA identity, no AH-versus-ESP split and no administrator switch to enable or disable AH auditing. Disclosed in docs/features/rfc-status.md RFC 4302 row |
| `RFC4302-5-1` | Implementations that claim conformance or compliance with this specification MUST fully implement the AH syntax and processing described here for unicast traffic, and MUST comply with all requirements of the Security Architecture document [Ken-Arch]. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{rollup}:** RFC4302-2-1, RFC4302-2-2, RFC4302-2.3-1, RFC4302-2.4-1, RFC4302-2.4-5, RFC4302-2.4-6, RFC4302-2.5-1, RFC4302-2.5-2, RFC4302-2.5-3, RFC4302-2.5-5, RFC4302-2.5.1-1, RFC4302-2.6-1, RFC4302-3.3.2-1, RFC4302-3.3.3.1-1, RFC4302-3.3.3.2.2-1, RFC4302-3.3.3.2.2-2, RFC4302-3.3.3.2.2-3, RFC4302-3.3.3.2.2-4, RFC4302-3.3.4-1, RFC4302-3.4.1-1, RFC4302-3.4.2-1, RFC4302-3.4.3-1, RFC4302-3.4.3-2, RFC4302-3.4.3-3, RFC4302-3.4.3-4, RFC4302-3.4.3-5, RFC4302-4-1, RFC4302-A2-1, RFC4302-A2-2, rfc4301; Section 5 binds a conforming implementation to "the AH syntax and processing described here for unicast traffic", which is every gated row of this summary except the three multicast rows of Section 2.4, and to "all requirements of the Security Architecture document", which is the whole of RFC 4301. **derived:** gap: RFC4302-2.5-5 is annotated {gap} |
| `RFC4302-5-2` | Additionally, if an implementation claims to support multicast traffic, it MUST comply with the additional requirements specified for support of such traffic. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{rollup}:** RFC4302-2.4-2, RFC4302-2.4-3, RFC4302-2.4-4; Section 5 binds a multicast implementation to "the additional requirements specified for support of such traffic", which are the three multicast rows of Section 2.4. **derived:** met |
| `RFC4302-A2-1` | Note that on the receive side, the IP implementation could leave a Fragmentation Extension Header in place when it does re-assembly. If this happens, then when AH receives the packet, before doing ICV processing, AH MUST "remove" (or skip over) this header and change the previous header's "Next Header" field to be the "Next Header" field in the Fragmentation Extension Header. (§A2) | MUST | A2 - Appendix A2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM; internal/plugins/ospf/ipsec_install.go::buildIPsecSA installs the transport-mode AH SA, and the kernel's IPv6 AH input skips a Fragmentation Extension Header the IP layer left in place and repairs the preceding Next Header |
| `RFC4302-A2-2` | Note that on the send side, the IP implementation could give the IPsec code a packet with a Fragmentation Extension Header with Offset of 0 (first fragment) and a More Fragments Flag of 0 (last fragment). If this happens, then before doing ICV processing, AH MUST first "remove" (or skip over) this header and change the previous header's "Next Header" field to be the "Next Header" field in the Fragmentation Extension Header. (§A2) | MUST | A2 - Appendix A2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM; internal/plugins/ospf/ipsec_install.go::buildIPsecSA installs the transport-mode AH SA, and the kernel's IPv6 AH output skips a Fragmentation Extension Header the IP layer handed it and repairs the preceding Next Header |
| `RFC4302-2.3-2` | it SHOULD be ignored by the recipient (§2.3) | SHOULD | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-2.5.1-2` | To support high-speed IPsec implementations, a new option for sequence numbers SHOULD be offered, as an extension to the current, 32-bit sequence number field. (§2.5.1) | SHOULD | 2.5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-3.4.3-6` | a window size of 64 is preferred and SHOULD be employed as the default. (§3.4.3) | SHOULD | 3.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-3.4.3-7` | This SHOULD be the first AH check applied to a packet after it has been matched to an SA, to speed rejection of duplicate packets. (§3.4.3) | SHOULD | 3.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-3.4.3-8` | if an SA establishment protocol such as IKE is employed, the receiver SHOULD notify the sender, during SA establishment, if the receiver will not provide anti-replay protection. (§3.4.3) | SHOULD | 3.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-5-3` | If the key used to compute an ICV is manually distributed, correct provision of the anti-replay service would require correct maintenance of the counter state at the sender, until the key is replaced, and there likely would be no automated recovery provision if counter overflow were imminent. Thus, a compliant implementation SHOULD NOT provide this service in conjunction with SAs that are manually keyed. (§5) | SHOULD NOT | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-2.4-7` | an implementation MAY choose any method to accelerate this search (§2.4) | MAY | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-3.3.4-2` | an AH implementation MAY choose to not support fragmentation and may mark transmitted packets with the DF bit, to facilitate Path MTU (PMTU) discovery. (§3.3.4) | MAY | 3.3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-3.4.3-9` | Another window size (larger than the MINIMUM) MAY be chosen by the receiver. (§3.4.3) | MAY | 3.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-5-4` | Additional algorithms, beyond those mandated for AH, MAY be supported. (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4302-2.3-1`](#rfc4302-2.3-1) It MUST be set to "zero" by the sender (§2.3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM; internal/plugins/ospf/ipsec_install.go::buildIPsecSA installs the transport-mode AH SA, and the kernel's AH output builds every AH header from it, so no value Ze writes decides this field |
| [`RFC4302-2.4-2`](#rfc4302-2.4-2) If an IPsec implementation supports multicast, then it MUST support multicast SAs using the algorithm below for mapping inbound IPsec datagrams to SAs. (§2.4) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams writes each SAD entry with its SPI and its address selector, and Linux XFRM maps an inbound AH datagram to an entry in the longest-match order this section gives; ze performs no SAD search of its own |
| [`RFC4302-2.4-3`](#rfc4302-2.4-3) A multicast-capable IPsec implementation MUST correctly de-multiplex inbound traffic even in the context of SPI collisions. (§2.4) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams writes a state Linux XFRM keys by destination, SPI and protocol, and Linux XFRM de-multiplexes on that key, so a group SA and a unicast SA that share an SPI stay distinct entries |
| [`RFC4302-2.4-4`](#rfc4302-2.4-4) any method to accelerate this search, although its externally visible behavior MUST be functionally equivalent to having searched the SAD in the above order. (§2.4) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams installs the SAD entries, and Linux XFRM owns both the search and whatever hashing accelerates it, so the equivalence this sentence demands is the kernel's to keep |
| [`RFC4302-2.5-1`](#rfc4302-2.5-1) For a unicast SA or a single-sender multicast SA, the sender MUST increment this field for every transmitted packet. (§2.5) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams writes the XFRM state that carries the sequence counter, and the kernel increments it for each packet it sends on the SA; Ze sees no AH packet |
| [`RFC4302-2.5-2`](#rfc4302-2.5-2) The field is mandatory and MUST always be present even if the receiver does not elect to enable the anti-replay service for a specific SA. (§2.5) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams writes the XFRM state, and the kernel emits the Sequence Number on every AH packet whether or not the state carries a replay window |
| [`RFC4302-2.5-3`](#rfc4302-2.5-3) Processing of the Sequence Number field is at the discretion of the receiver, but all AH implementations MUST be capable of performing the processing described in Section 3.3.2, "Sequence Number Generation", and Section 3.4.3, "Sequence Number Verification". (§2.5) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams writes the state for both directions of the SA, and the kernel generates the sequence numbers it sends and verifies the ones it receives |
| [`RFC4302-2.5-5`](#rfc4302-2.5-5) Thus, the sender's counter and the receiver's counter MUST be reset (by establishing a new SA and thus a new key) prior to the transmission of the 2^32nd packet on an SA. (§2.5) | {gap}, no test | nothing resets the counters before the 2^32nd packet. The OSPFv3 SA is manually keyed and ze never rekeys it: setConfig installs one SPI and key per interface (internal/plugins/ospf/ipsec_install.go) and no code establishes a replacement SA, so an operator who enables anti-replay with the replay-window leaf gets an SA that stops sending at the rollover rather than one that is re-established. Disclosed in docs/features/rfc-status.md RFC 4302 row |
| [`RFC4302-2.5.1-1`](#rfc4302-2.5.1-1) Use of an Extended Sequence Number (ESN) MUST be negotiated by an SA management protocol. (§2.5.1) | no test | no test carries this requirement id; annotated {feature-declined}: "a new option for sequence numbers SHOULD be offered, as an extension to the current, 32-bit sequence number field"; ze offers no ESN, so nothing ever uses one. internal/plugins/ospf/ipsec_install.go::buildIPsecSA is the only code that creates an AH SA, and it builds one manually keyed transport-mode state from static interface configuration with no ESN in it; espProposalToWire, internal/component/ike/engine/initiator.go, proposes ESP alone and keys Transform Type 5 to espESNNotExtended, so ze runs no SA management protocol that could negotiate an AH ESN |
| [`RFC4302-2.6-1`](#rfc4302-2.6-1) All implementations MUST support such padding and MUST insert only enough padding to satisfy the IPv4/IPv6 alignment requirements. (§2.6) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams sets the integrity transform and its truncation length, and the kernel's AH output sizes the ICV field and pads it to the alignment the address family needs |
| [`RFC4302-3.3.2-1`](#rfc4302-3.3.2-1) In other words, the sender MUST NOT send a packet on an SA if doing so would cause the sequence number to cycle. (§3.3.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams writes the per-SA replay window the interface replay-window leaf asks for, and Linux XFRM refuses to send a packet on that state once the sequence number would cycle; ze emits no AH packet of its own |
| [`RFC4302-3.3.3.1-1`](#rfc4302-3.3.3.1-1) If a field may be modified during transit, the value of the field is set to zero for purposes of the ICV computation. If a field is mutable, but its value at the (IPsec) receiver is predictable, then that value is inserted into the field for purposes of the ICV calculation. The Integrity Check Value field is also set to zero in preparation for this computation. (§3.3.3.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM; internal/plugins/ospf/ipsec_install.go::buildIPsecSA installs the transport-mode AH SA, and the kernel's AH code zeroes the mutable fields and the ICV field before it computes the ICV |
| [`RFC4302-3.3.3.2.2-1`](#rfc4302-3.3.3.2.2-1) If the IP packet length (including AH and the 32 high-order bits of the ESN, if enabled) does not match the blocksize requirements for the algorithm, implicit padding MUST be appended to the end of the packet, prior to ICV computation. (§3.3.3.2.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams names the integrity algorithm, and the kernel's crypto layer appends whatever implicit padding that algorithm's blocksize needs |
| [`RFC4302-3.3.3.2.2-2`](#rfc4302-3.3.3.2.2-2) The padding octets MUST have a value of zero. (§3.3.3.2.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams names the integrity algorithm, and the kernel's crypto layer chooses the value of the implicit padding octets |
| [`RFC4302-3.3.3.2.2-3`](#rfc4302-3.3.3.2.2-3) The document that defines an integrity algorithm MUST be consulted to determine if implicit padding is required as described above. (§3.3.3.2.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams names the integrity algorithm, and the kernel's implementation of that algorithm decides whether implicit padding is required |
| [`RFC4302-3.3.3.2.2-4`](#rfc4302-3.3.3.2.2-4) If the document does not specify an answer to this, then the default is to assume that implicit padding is required (as needed to match the packet length to the algorithm's blocksize.) If padding bytes are needed but the algorithm does not specify the padding contents, then the padding octets MUST have a value of zero. (§3.3.3.2.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams names the integrity algorithm, and the kernel's implementation of that algorithm supplies the default when the algorithm gives no answer |
| [`RFC4302-3.4.1-1`](#rfc4302-3.4.1-1) If a packet offered to AH for processing appears to be an IP fragment, i.e., the OFFSET field is nonzero or the MORE FRAGMENTS flag is set, the receiver MUST discard the packet; this is an auditable event. (§3.4.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM; internal/plugins/ospf/ipsec_install.go::buildIPsecPolicies installs the inbound require-policy that puts OSPF traffic through the kernel's AH input, which is where a packet offered to AH that is an IP fragment is dropped |
| [`RFC4302-3.4.3-2`](#rfc4302-3.4.3-2) If the receiver has enabled the anti-replay service for this SA, the receive packet counter for the SA MUST be initialized to zero when the SA is established. (§3.4.3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams writes the state and presets no replay counter, and Linux XFRM initializes the receive packet counter to zero as it creates the state |
| [`RFC4302-3.4.3-3`](#rfc4302-3.4.3-3) For each received packet, the receiver MUST verify that the packet contains a Sequence Number that does not duplicate the Sequence Number of any other packets received during the life of this SA. (§3.4.3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams writes the replay window the receiver asked for, and Linux XFRM checks every received sequence number against that window and rejects a duplicate |
| [`RFC4302-3.4.3-4`](#rfc4302-3.4.3-4) If the ICV validation fails, the receiver MUST discard the received IP datagram as invalid. (§3.4.3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams installs the integrity transform the kernel validates the ICV with, and the kernel discards the datagram when that validation fails; Ze holds no rejecting branch of its own |
| [`RFC4302-4-1`](#rfc4302-4-1) However, if AH is incorporated into a system that supports auditing, then the AH implementation MUST also support auditing and MUST allow a system administrator to enable or disable auditing for AH. (§4) | {gap}, no test | ze supports auditing for AH nowhere. readXfrmDropsPlatform (internal/plugins/ospf/ipsec_drops_linux.go) polls the node-global /proc/net/xfrm_stat counters into one OSPF metric, which carries no SPI, no per-SA identity, no AH-versus-ESP split and no administrator switch to enable or disable AH auditing. Disclosed in docs/features/rfc-status.md RFC 4302 row |
| [`RFC4302-5-1`](#rfc4302-5-1) Implementations that claim conformance or compliance with this specification MUST fully implement the AH syntax and processing described here for unicast traffic, and MUST comply with all requirements of the Security Architecture document [Ken-Arch]. (§5) | no test | no test carries this requirement id; annotated {rollup}: RFC4302-2-1, RFC4302-2-2, RFC4302-2.3-1, RFC4302-2.4-1, RFC4302-2.4-5, RFC4302-2.4-6, RFC4302-2.5-1, RFC4302-2.5-2, RFC4302-2.5-3, RFC4302-2.5-5, RFC4302-2.5.1-1, RFC4302-2.6-1, RFC4302-3.3.2-1, RFC4302-3.3.3.1-1, RFC4302-3.3.3.2.2-1, RFC4302-3.3.3.2.2-2, RFC4302-3.3.3.2.2-3, RFC4302-3.3.3.2.2-4, RFC4302-3.3.4-1, RFC4302-3.4.1-1, RFC4302-3.4.2-1, RFC4302-3.4.3-1, RFC4302-3.4.3-2, RFC4302-3.4.3-3, RFC4302-3.4.3-4, RFC4302-3.4.3-5, RFC4302-4-1, RFC4302-A2-1, RFC4302-A2-2, rfc4301; Section 5 binds a conforming implementation to "the AH syntax and processing described here for unicast traffic", which is every gated row of this summary except the three multicast rows of Section 2.4, and to "all requirements of the Security Architecture document", which is the whole of RFC 4301 |
| [`RFC4302-5-2`](#rfc4302-5-2) Additionally, if an implementation claims to support multicast traffic, it MUST comply with the additional requirements specified for support of such traffic. (§5) | no test | no test carries this requirement id; annotated {rollup}: RFC4302-2.4-2, RFC4302-2.4-3, RFC4302-2.4-4; Section 5 binds a multicast implementation to "the additional requirements specified for support of such traffic", which are the three multicast rows of Section 2.4 |
| [`RFC4302-A2-1`](#rfc4302-a2-1) Note that on the receive side, the IP implementation could leave a Fragmentation Extension Header in place when it does re-assembly. If this happens, then when AH receives the packet, before doing ICV processing, AH MUST "remove" (or skip over) this header and change the previous header's "Next Header" field to be the "Next Header" field in the Fragmentation Extension Header. (§A2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM; internal/plugins/ospf/ipsec_install.go::buildIPsecSA installs the transport-mode AH SA, and the kernel's IPv6 AH input skips a Fragmentation Extension Header the IP layer left in place and repairs the preceding Next Header |
| [`RFC4302-A2-2`](#rfc4302-a2-2) Note that on the send side, the IP implementation could give the IPsec code a packet with a Fragmentation Extension Header with Offset of 0 (first fragment) and a More Fragments Flag of 0 (last fragment). If this happens, then before doing ICV processing, AH MUST first "remove" (or skip over) this header and change the previous header's "Next Header" field to be the "Next Header" field in the Fragmentation Extension Header. (§A2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM; internal/plugins/ospf/ipsec_install.go::buildIPsecSA installs the transport-mode AH SA, and the kernel's IPv6 AH output skips a Fragmentation Extension Header the IP layer handed it and repairs the preceding Next Header |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4302-2-1`](#rfc4302-2-1)

The protocol header (IPv4, IPv6, or IPv6 Extension) immediately preceding the AH header SHALL contain the value 51 in its Protocol (IPv4) or Next Header (IPv6, Extension) fields [DH98]. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: an AH SA whose protocol is not 51, so the kernel writes another value in the header preceding AH. (b) TestIPsecSAProtocolNumber: ah.Proto != 51 goes red; the esp case (Proto 50) goes red on a blanket-51 installer. Ze's boundary is the SA protocol number; Linux XFRM writes the Next Header from it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPsecSAProtocolNumber`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L375) | unit/verify | mutant, verified |
| positive | [`TestIPsecSAProtocolNumber`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L370) | unit/verify | mutant, verified |

### [`RFC4302-2-2`](#rfc4302-2-2)

AH does not contain a version number, therefore if there are concerns about backward compatibility, they MUST be addressed by using a signaling mechanism between the two IPsec peers to ensure compatible versions of AH, e.g., IKE [IKEv2] or an out-of-band configuration mechanism. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: AH parameters settled without a signalling or out-of-band mechanism both peers hold, e.g. a defaulted integrity algorithm. (b) TestAHParametersComeFromConfiguration: protocol, SPI, algorithm and key length are asserted from the parsed RFC 4552 block (positive), and an AH block with no algorithm must fail with ErrIPsecAuthAlgo (negative). Ze uses the out-of-band configuration mechanism the sentence names; it runs no AH signalling.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAHParametersComeFromConfiguration`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_rfc4302_test.go#L179) | unit/verify | mutant, verified |
| positive | [`TestAHParametersComeFromConfiguration`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_rfc4302_test.go#L171) | unit/verify | mutant, verified |

### [`RFC4302-2.3-1`](#rfc4302-2.3-1)

It MUST be set to "zero" by the sender (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-2.3-1, so no unit is bound to it.

### [`RFC4302-2.4-1`](#rfc4302-2.4-1)

The SPI field is mandatory, and this mechanism for mapping inbound traffic to unicast SAs described above MUST be supported by all AH implementations. (§2.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. (a) forbidden: an AH implementation that cannot map inbound traffic to a UNICAST SA by its SPI (alone or with the protocol). (b) TestAHSAIdentifiedBySPIAndProtocol asserts SPI, Proto AH, Dst and wildcard Src, but only for the ff02::5 AllSPFRouters state, a multicast destination. The unicast state setConfig also builds (buildIPsecSA(ifindex, local, SADirIn, c), ipsec_install.go) is never asserted, so the unicast clause the sentence is about has no red assertion.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAHSAIdentifiedBySPIAndProtocol`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_rfc4302_test.go#L46) | unit/verify | revert, verified |
| positive | [`TestAHSAIdentifiedBySPIAndProtocol`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_rfc4302_test.go#L37) | unit/verify | mutant, verified |

### [`RFC4302-2.4-2`](#rfc4302-2.4-2)

If an IPsec implementation supports multicast, then it MUST support multicast SAs using the algorithm below for mapping inbound IPsec datagrams to SAs. (§2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-2.4-2, so no unit is bound to it.

### [`RFC4302-2.4-3`](#rfc4302-2.4-3)

A multicast-capable IPsec implementation MUST correctly de-multiplex inbound traffic even in the context of SPI collisions. (§2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-2.4-3, so no unit is bound to it.

### [`RFC4302-2.4-4`](#rfc4302-2.4-4)

any method to accelerate this search, although its externally visible behavior MUST be functionally equivalent to having searched the SAD in the above order. (§2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-2.4-4, so no unit is bound to it.

### [`RFC4302-2.4-5`](#rfc4302-2.4-5)

The indication of whether source and destination address matching is required to map inbound IPsec traffic to SAs MUST be set either as a side effect of manual SA configuration or via negotiation using an SA management protocol, e.g., IKE or Group Domain of Interpretation (GDOI) [RFC3547]. (§2.4)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. The sentence is about the SAD entry's indication of which addresses join the SPI in the inbound SA lookup; in XFRM that is the state's daddr and whether its saddr is wildcarded (SAParams.Src/Dst). TestAHSAAddressMatchIndication asserts the state's traffic selector (Sel.Src/Sel.Dst ::/0, Sel.UpperProto 89), which filters traffic on an SA already found and does not decide the lookup key. The Src-wildcard indication is asserted in TestAHSAIdentifiedBySPIAndProtocol, which is not tagged for this row. The negative (upper protocol 89) proves a neighbouring selector rule.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAHSAAddressMatchIndication`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_rfc4302_test.go#L77) | unit/verify | revert, verified |
| positive | [`TestAHSAAddressMatchIndication`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_rfc4302_test.go#L69) | unit/verify | revert, verified |

### [`RFC4302-2.4-6`](#rfc4302-2.4-6)

The SPI value of zero (0) is reserved for local, implementation-specific use and MUST NOT be sent on the wire. (§2.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: an AH SA carrying SPI 0 installed and so sent on the wire. (b) TestAHSPIReservedRangeRefused: validateConfig on spi 0 must return ErrIPsecSPIReserved, else red; spi 256 must validate (positive bound). SPI 255 refusal is IANA-range, beyond this row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAHSPIReservedRangeRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_rfc4302_test.go#L94) | unit/verify | mutant, verified |
| positive | [`TestAHSPIReservedRangeRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_rfc4302_test.go#L99) | unit/verify | mutant, verified |

### [`RFC4302-2.5-1`](#rfc4302-2.5-1)

For a unicast SA or a single-sender multicast SA, the sender MUST increment this field for every transmitted packet. (§2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-2.5-1, so no unit is bound to it.

### [`RFC4302-2.5-2`](#rfc4302-2.5-2)

The field is mandatory and MUST always be present even if the receiver does not elect to enable the anti-replay service for a specific SA. (§2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-2.5-2, so no unit is bound to it.

### [`RFC4302-2.5-3`](#rfc4302-2.5-3)

Processing of the Sequence Number field is at the discretion of the receiver, but all AH implementations MUST be capable of performing the processing described in Section 3.3.2, "Sequence Number Generation", and Section 3.4.3, "Sequence Number Verification". (§2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-2.5-3, so no unit is bound to it.

### [`RFC4302-2.5-5`](#rfc4302-2.5-5)

Thus, the sender's counter and the receiver's counter MUST be reset (by establishing a new SA and thus a new key) prior to the transmission of the 2^32nd packet on an SA. (§2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-2.5-5, so no unit is bound to it.

### [`RFC4302-2.5.1-1`](#rfc4302-2.5.1-1)

Use of an Extended Sequence Number (ESN) MUST be negotiated by an SA management protocol. (§2.5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-2.5.1-1, so no unit is bound to it.

### [`RFC4302-2.6-1`](#rfc4302-2.6-1)

All implementations MUST support such padding and MUST insert only enough padding to satisfy the IPv4/IPv6 alignment requirements. (§2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-2.6-1, so no unit is bound to it.

### [`RFC4302-3.3.2-1`](#rfc4302-3.3.2-1)

In other words, the sender MUST NOT send a packet on an SA if doing so would cause the sequence number to cycle. (§3.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-3.3.2-1, so no unit is bound to it.

### [`RFC4302-3.3.3.1-1`](#rfc4302-3.3.3.1-1)

If a field may be modified during transit, the value of the field is set to zero for purposes of the ICV computation. If a field is mutable, but its value at the (IPsec) receiver is predictable, then that value is inserted into the field for purposes of the ICV calculation. The Integrity Check Value field is also set to zero in preparation for this computation. (§3.3.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-3.3.3.1-1, so no unit is bound to it.

### [`RFC4302-3.3.3.2.2-1`](#rfc4302-3.3.3.2.2-1)

If the IP packet length (including AH and the 32 high-order bits of the ESN, if enabled) does not match the blocksize requirements for the algorithm, implicit padding MUST be appended to the end of the packet, prior to ICV computation. (§3.3.3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-3.3.3.2.2-1, so no unit is bound to it.

### [`RFC4302-3.3.3.2.2-2`](#rfc4302-3.3.3.2.2-2)

The padding octets MUST have a value of zero. (§3.3.3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-3.3.3.2.2-2, so no unit is bound to it.

### [`RFC4302-3.3.3.2.2-3`](#rfc4302-3.3.3.2.2-3)

The document that defines an integrity algorithm MUST be consulted to determine if implicit padding is required as described above. (§3.3.3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-3.3.3.2.2-3, so no unit is bound to it.

### [`RFC4302-3.3.3.2.2-4`](#rfc4302-3.3.3.2.2-4)

If the document does not specify an answer to this, then the default is to assume that implicit padding is required (as needed to match the packet length to the algorithm's blocksize.) If padding bytes are needed but the algorithm does not specify the padding contents, then the padding octets MUST have a value of zero. (§3.3.3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-3.3.3.2.2-4, so no unit is bound to it.

### [`RFC4302-3.3.4-1`](#rfc4302-3.3.4-1)

In any case, an AH implementation MUST support generation of ICMP PMTU messages (or equivalent internal signaling for native host implementations) to minimize the likelihood of fragmentation. (§3.3.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: an AH native-host implementation that gives the local sender no PMTU signal for the AH overhead. (b) TestAHPathMTUIsSignaledToTheLocalSender (real XFRM): the IPV6_MTU of a raw OSPF socket must drop by at least ahHeaderOctets after the installer writes the AH state and policy (protected >= clear is red), and return to the clear value when the state is removed. Ze is transport-mode host only, so the internal-signalling alternative the sentence gives native hosts is the one owed; ICMP generation for forwarded traffic is not exercised.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAHPathMTUIsSignaledToTheLocalSender`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_pmtu_integration_linux_test.go#L143) | unit/verify | revert, verified |
| positive | [`TestAHPathMTUIsSignaledToTheLocalSender`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_pmtu_integration_linux_test.go#L133) | unit/verify | revert, verified |

### [`RFC4302-3.4.1-1`](#rfc4302-3.4.1-1)

If a packet offered to AH for processing appears to be an IP fragment, i.e., the OFFSET field is nonzero or the MORE FRAGMENTS flag is set, the receiver MUST discard the packet; this is an auditable event. (§3.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-3.4.1-1, so no unit is bound to it.

### [`RFC4302-3.4.2-1`](#rfc4302-3.4.2-1)

If no valid Security Association exists for this packet the receiver MUST discard the packet; this is an auditable event. (§3.4.2)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. The sentence covers a received packet CONTAINING an AH header whose SA lookup fails: the discard, and that it is an auditable event. TestAHInboundRequirePolicyInstalled asserts the SADirIn require-policy (proto AH, transport, upper proto 89, ifindex), which drops UNPROTECTED OSPF that arrives without AH (an RFC 4301 policy check), a neighbouring rule. An AH packet with an unknown SPI is dropped in the kernel's xfrm_input state lookup whatever policy Ze installs; no tagged unit sends one or observes the drop, and the auditable-event clause has no assertion at all.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAHInboundRequirePolicyInstalled`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_rfc4302_test.go#L130) | unit/verify | mutant, verified |
| positive | [`TestAHInboundRequirePolicyInstalled`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_rfc4302_test.go#L122) | unit/verify | revert, verified |

### [`RFC4302-3.4.3-1`](#rfc4302-3.4.3-1)

All AH implementations MUST support the anti-replay service, though its use may be enabled or disabled by the receiver on a per-SA basis. (§3.4.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. (a) forbidden: an AH implementation with no anti-replay, or one that cannot enable/disable it per SA. (b) TestIPsecSAReplayWindow asserts SAParams.ReplayWin 64 when configured and 0 when not, for ah and esp. Both per-SA directions are asserted, but only on the SAParams struct: xfrmStateFromParams (ike/dataplane/xfrm_linux.go) sets the kernel window only for a non-zero ReplayWin, and a break there stays green. The kernel-state assertion that exists (rfc4301_boundary_linux_test.go, ReplayWindow 64) is not tagged for this row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPsecSAReplayWindow`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L439) | unit/verify | revert, verified |
| positive | [`TestIPsecSAReplayWindow`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L434) | unit/verify | revert, verified |

### [`RFC4302-3.4.3-2`](#rfc4302-3.4.3-2)

If the receiver has enabled the anti-replay service for this SA, the receive packet counter for the SA MUST be initialized to zero when the SA is established. (§3.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-3.4.3-2, so no unit is bound to it.

### [`RFC4302-3.4.3-3`](#rfc4302-3.4.3-3)

For each received packet, the receiver MUST verify that the packet contains a Sequence Number that does not duplicate the Sequence Number of any other packets received during the life of this SA. (§3.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-3.4.3-3, so no unit is bound to it.

### [`RFC4302-3.4.3-4`](#rfc4302-3.4.3-4)

If the ICV validation fails, the receiver MUST discard the received IP datagram as invalid. (§3.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-3.4.3-4, so no unit is bound to it.

### [`RFC4302-3.4.3-5`](#rfc4302-3.4.3-5)

A MINIMUM window size of 32 packets MUST be supported (§3.4.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. (a) forbidden: an implementation that cannot run an SA with a 32-packet replay window. (b) TestIPsecReplayWindowRange only asserts that validateConfig accepts replay-window 32; no tagged unit shows a 32 window reaching the SA or the kernel state. The negative (1 and 31 refused with ErrIPsecReplayWindow) does not violate this requirement: the RFC sets a minimum that must be SUPPORTED and forbids no smaller window, so the negative asserts a Ze policy, not the RFC.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPsecReplayWindowRange`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L260) | unit/verify | mutant, verified |
| positive | [`TestIPsecReplayWindowRange`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L256) | unit/verify | mutant, verified |

### [`RFC4302-4-1`](#rfc4302-4-1)

However, if AH is incorporated into a system that supports auditing, then the AH implementation MUST also support auditing and MUST allow a system administrator to enable or disable auditing for AH. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-4-1, so no unit is bound to it.

### [`RFC4302-5-1`](#rfc4302-5-1)

Implementations that claim conformance or compliance with this specification MUST fully implement the AH syntax and processing described here for unicast traffic, and MUST comply with all requirements of the Security Architecture document [Ken-Arch]. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-5-1, so no unit is bound to it.

### [`RFC4302-5-2`](#rfc4302-5-2)

Additionally, if an implementation claims to support multicast traffic, it MUST comply with the additional requirements specified for support of such traffic. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-5-2, so no unit is bound to it.

### [`RFC4302-A2-1`](#rfc4302-a2-1)

Note that on the receive side, the IP implementation could leave a Fragmentation Extension Header in place when it does re-assembly. If this happens, then when AH receives the packet, before doing ICV processing, AH MUST "remove" (or skip over) this header and change the previous header's "Next Header" field to be the "Next Header" field in the Fragmentation Extension Header. (§A2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-A2-1, so no unit is bound to it.

### [`RFC4302-A2-2`](#rfc4302-a2-2)

Note that on the send side, the IP implementation could give the IPsec code a packet with a Fragmentation Extension Header with Offset of 0 (first fragment) and a More Fragments Flag of 0 (last fragment). If this happens, then before doing ICV processing, AH MUST first "remove" (or skip over) this header and change the previous header's "Next Header" field to be the "Next Header" field in the Fragmentation Extension Header. (§A2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-A2-2, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-27 |
| Register | rfc2119 |
| Source | rfc/full/rfc4302.txt |
| Source fingerprint | 9966a2ddb94e62b7 |
| Record | rfc/extraction/rfc4302.json |
| Mapped sentences | 33 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Status of This Memo, Copyright Notice, Abstract and Table of Contents. |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 2 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `2.3` | not stated | 1 | walked | not stated |
| `2.4` | not stated | 6 | walked | not stated |
| `2.5` | not stated | 5 | walked | not stated |
| `2.5.1` | not stated | 1 | walked | not stated |
| `2.6` | not stated | 2 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.1.1` | not stated | 0 | walked | not stated |
| `3.1.2` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `3.3` | not stated | 0 | walked | not stated |
| `3.3.1` | not stated | 0 | walked | not stated |
| `3.3.2` | not stated | 1 | walked | not stated |
| `3.3.3` | not stated | 0 | walked | not stated |
| `3.3.3.1` | not stated | 0 | walked | not stated |
| `3.3.3.1.1` | not stated | 0 | walked | not stated |
| `3.3.3.1.1.1` | not stated | 0 | walked | not stated |
| `3.3.3.1.1.2` | not stated | 0 | walked | not stated |
| `3.3.3.1.2` | not stated | 0 | walked | not stated |
| `3.3.3.1.2.1` | not stated | 0 | walked | not stated |
| `3.3.3.1.2.2` | not stated | 0 | walked | not stated |
| `3.3.3.1.2.3` | not stated | 0 | walked | not stated |
| `3.3.3.2` | not stated | 0 | walked | not stated |
| `3.3.3.2.1` | not stated | 0 | walked | not stated |
| `3.3.3.2.2` | not stated | 4 | walked | not stated |
| `3.3.4` | not stated | 1 | walked | not stated |
| `3.4` | not stated | 0 | walked | not stated |
| `3.4.1` | not stated | 1 | walked | not stated |
| `3.4.2` | not stated | 1 | walked | not stated |
| `3.4.3` | not stated | 5 | walked | not stated |
| `3.4.4` | not stated | 1 | walked | not stated |
| `4` | not stated | 1 | walked | not stated |
| `5` | not stated | 2 | walked | not stated |
| `6` | not stated | 0 | skipped (appendix-non-normative) | Security Considerations: it discusses the assurance AH provides and states no obligation the extractor or this reviewer could find. |
| `7` | not stated | 0 | skipped (appendix-non-normative) | Differences from RFC 2402: a change log against the document this one obsoletes. |
| `8` | Acknowledgements | 0 | skipped (acknowledgements) | Acknowledgements. |
| `9` | References | 0 | skipped (references) | References. |
| `9.1` | Normative References | 0 | skipped (references) | Normative References. |
| `9.2` | not stated | 0 | walked | not stated |
| `A` | Appendix A | 0 | walked | Appendix A. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `A1` | Appendix A1 | 0 | walked | Appendix A1. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `A2` | Appendix A2 | 2 | walked | Appendix A2. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of section 9.2, where its 2 site(s) were walked; every decision is carried forward by its verbatim quote. |
| `B` | Appendix B | 0 | walked | Appendix B. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `B1` | Appendix B1 | 0 | walked | Appendix B1. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `B2` | Appendix B2 | 0 | walked | Appendix B2. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `B2.1` | Appendix B2.1 | 0 | walked | Appendix B2.1. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `B2.2` | Appendix B2.2 | 0 | walked | Appendix B2.2. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `B2.3` | Appendix B2.3 | 0 | walked | Appendix B2.3. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `B3` | Appendix B3 | 0 | walked | Appendix B3. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `B3.1` | Appendix B3.1 | 0 | walked | Appendix B3.1. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `B3.2` | Appendix B3.2 | 0 | walked | Appendix B3.2. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of the section before it. It carries no site, so no decision moved. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `2.5:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the obligation site 2.5:2 already carries: the Sequence Number field is mandatory and the sender always transmits it. The clause after the comma, that the receiver need not act upon it, is a permission rather than an obligation and site 3.4.3:1 carries the receiver's own rule. | Thus, the sender MUST always transmit this field, but the receiver need not act upon it. |
| `2.6:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an integrity-algorithm specification, and the sentence obliges that document to state the ICV length and the validation rules. Ze publishes no integrity-algorithm specification and no producer in this repository could: it CONSUMES them, and ipsecAuthKeyLen (internal/plugins/ospf/config.go) plus xfrmAuthTruncLen (internal/component/ike/dataplane/xfrm_linux.go) hold the key length and the ICV truncation RFC 4868 already specified for HMAC-SHA-256-128, SHA-384-192 and SHA-512-256. The obligation the sentence places on ze as a reader of such a document is site 3.3.3.2.2:3, which is mapped. | The integrity algorithm specification MUST specify the length of the ICV and the comparison rules and processing steps for validation. |
| `3.4.4:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the discard obligation site 3.4.3:4 carries. Both sentences say that a failed ICV comparison discards the datagram as invalid; this one is the Integrity Check Value Verification section repeating the Sequence Number Verification section. | If the test fails, then the receiver MUST discard the received IP datagram as invalid. |

## Superseded

No document obsoletes RFC 4302, so its obligations are stated where they were written.
