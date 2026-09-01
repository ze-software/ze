# RFC 4761 - Virtual Private LAN Service (VPLS) Using BGP for Auto-Discovery and Signaling

Supported. Every requirement this repository extracted from RFC 4761, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

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
| Gated MUSTs | 18 | of 27 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 18 | of 18 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

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
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 27 |
| Gated MUST-level | 18 |
| Obligations that bind Ze | 0 |
| Not applicable, so out of scope | 18 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc4761.md` |
| Requirement shard | `rfc/requirements/rfc4761.md` |
| RFC text | `rfc/full/rfc4761.txt` |

## Enrolment

Enrolled: Virtual Private LAN Service / VPLS Using BGP (RFC 4761): all 18 gated MUSTs not-applicable. ze is not a VPLS PE -- its RFC 4761 support is a decode-mode L2VPN/VPLS (AFI 25, SAFI 65) NLRI codec (internal/component/bgp/plugins/nlri/vpls/) plus an operator-driven Layer2 Info ext-community passthrough; no VSI, no pseudowire signaling, no MAC FIB, received L2VPN routes stored as opaque raw NLRI blobs (adj_rib_in/rib.go). The gated MUSTs are all VPLS-PE control-plane signaling and data-plane forwarding duties ze does not perform. Same delegation pattern as enrolled RFC 4364

## What the public ledger says

**Status:** Supported

**What the ledger says is covered:**

L2VPN VPLS family registration, encode, decode, and route config.

**What the ledger says remains:**

No tracked gap in current source anchors.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 18 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **18** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (18):** [`RFC4761-2.3-1`](#rfc4761-2.3-1), [`RFC4761-3.2.4-1`](#rfc4761-3.2.4-1), [`RFC4761-3.2.4-2`](#rfc4761-3.2.4-2), [`RFC4761-3.2.4-3`](#rfc4761-3.2.4-3), [`RFC4761-3.2.4-4`](#rfc4761-3.2.4-4), [`RFC4761-3.2.4-5`](#rfc4761-3.2.4-5), [`RFC4761-3.2.4-6`](#rfc4761-3.2.4-6), [`RFC4761-3.2.3-1`](#rfc4761-3.2.3-1), [`RFC4761-3.2.3-2`](#rfc4761-3.2.3-2), [`RFC4761-3.3-1`](#rfc4761-3.3-1), [`RFC4761-3.4.2-1`](#rfc4761-3.4.2-1), [`RFC4761-3.5-1`](#rfc4761-3.5-1), [`RFC4761-3.5-2`](#rfc4761-3.5-2), [`RFC4761-4.2.1-1`](#rfc4761-4.2.1-1), [`RFC4761-4.2.2-1`](#rfc4761-4.2.2-1), [`RFC4761-4.2.5-1`](#rfc4761-4.2.5-1), [`RFC4761-6-1`](#rfc4761-6-1), [`RFC4761-6-2`](#rfc4761-6-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4761-2.3-1` | An implementation MUST maintain a separate routing storage for each service (Section 2.3) | MUST | 2.3 - Interactions | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** maintaining a separate routing/forwarding store per VPLS service is a VPLS-PE VSI duty; ze is not a VPLS PE: its RFC 4761 support is a control-plane L2VPN/VPLS (AFI 25, SAFI 65) NLRI codec (internal/component/bgp/plugins/nlri/vpls/vpls.go:45 Mode decode; types.go:82 ParseVPLS, types.go:163 WriteTo) plus route-injection encoders, with no VPLS instance (VSI), no pseudowire signaling, no MAC FIB, and no VPLS forwarding data path, so there is no per-service VSI store to maintain |
| `RFC4761-3.2.4-1` | MBZ bits in Control Flags MUST be set to zero when sending (Section 3.2.4) | MUST | 3.2.4 - Signaling PE Capabilities | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** signalling the Layer2 Info Extended Community (0x800A) Control Flags is a duty of a VPLS PE originating auto-discovery; ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45, no VSI, no PW signaling), and the only l2info code is an operator-driven raw ext-community encoder that packs the supplied control byte verbatim (internal/component/bgp/config/routeattr_community.go:283-305 parseL2InfoExtCommunity), not a PE Control-Flags sender, so ze originates no VPLS Control Flags whose MBZ bits it must zero |
| `RFC4761-3.2.4-2` | MBZ bits in Control Flags MUST be ignored when receiving (Section 3.2.4) | MUST | 3.2.4 - Signaling PE Capabilities | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ignoring MBZ bits on receipt of the Layer2 Info Control Flags is a VPLS-PE receive duty; ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45, no VSI, no PW state) and has no Layer2 Info decoder, so no MBZ-on-receive processing exists to exercise |
| `RFC4761-3.2.4-3` | When C flag is 1, a Control Word MUST be present when sending VPLS packets to that PE (Section 3.2.4) | MUST | 3.2.4 - Signaling PE Capabilities | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** inserting a Control Word into VPLS packets when the C flag is 1 is a pseudowire data-plane encapsulation choice; ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45, no VSI, no PW signaling, no VPLS forwarding data path), so it sends no VPLS packets and the C-flag data-plane obligation has no host |
| `RFC4761-3.2.4-4` | When C flag is 0, a Control Word MUST NOT be present when sending VPLS packets to that PE (Section 3.2.4) | MUST NOT | 3.2.4 - Signaling PE Capabilities | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** omitting the Control Word from VPLS packets when the C flag is 0 is a pseudowire data-plane choice; ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45, no VSI, no VPLS forwarding data path), so it sends no VPLS packets to constrain |
| `RFC4761-3.2.4-5` | When S flag is 1, sequenced delivery of frames MUST be used when sending VPLS packets to that PE (Section 3.2.4) | MUST | 3.2.4 - Signaling PE Capabilities | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** using sequenced delivery for VPLS packets when the S flag is 1 is a pseudowire data-plane behavior; ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45, no VSI, no VPLS forwarding data path), so it delivers no VPLS packets |
| `RFC4761-3.2.4-6` | When S flag is 0, sequenced delivery MUST NOT be used when sending VPLS packets to that PE (Section 3.2.4) | MUST NOT | 3.2.4 - Signaling PE Capabilities | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** refraining from sequenced delivery when the S flag is 0 is a pseudowire data-plane behavior; ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45, no VSI, no VPLS forwarding data path), so it delivers no VPLS packets |
| `RFC4761-3.2.3-1` | If announcing PE's VE ID is not covered by any remote VE set that PE-b announced, PE-b MUST make a new announcement covering it (Section 3.2.3) | MUST | 3.2.3 - PW Setup and Teardown | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** computing and announcing a new label block to cover a peer's VE ID is VPLS-PE auto-discovery signaling; ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45, no VSI), and runs no label-block allocation or pseudowire-setup algorithm |
| `RFC4761-3.2.3-2` | If Y withdraws an NLRI for V that X was using, X MUST tear down its ends of the pseudowire between X and Y (Section 3.2.3) | MUST | 3.2.3 - PW Setup and Teardown | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** tearing down pseudowire ends when a peer withdraws its NLRI is a VPLS-PE data-plane action; ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45, no VSI, no VPLS forwarding data path), so it establishes and tears down no pseudowires |
| `RFC4761-3.3-1` | PE-a MUST withdraw all its announcements for VPLS foo that contain VE ID V when V is removed from configuration (Section 3.3) | MUST | 3.3 - BGP VPLS Operation | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** withdrawing a PE's own VPLS announcements when a VE ID leaves the VSI configuration is a VPLS-PE lifecycle duty; ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45), and has no VSI/VE configuration model that originates or withdraws such announcements |
| `RFC4761-3.4.2-1` | Inter-AS Method (b): Length, Route Distinguisher, VE ID, VE Block Offset, and VE Block Size MUST be the same when ASBR re-advertises (Section 3.4.2) | MUST | 3.4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** re-advertising a VPLS NLRI with unchanged Length/RD/VE-ID/VE-Block-Offset/VE-Block-Size while swapping labels is a VPLS ASBR duty (Inter-AS option b); ze is not a VPLS PE or ASBR (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45, no VSI, no VPLS forwarding data path), and performs no VPLS ASBR label-swap re-advertisement |
| `RFC4761-3.5-1` | When receiving equivalent NLRIs from multiple PEs, MUST pick only one via BGP path selection (Section 3.5) | MUST | 3.5 - Multi-homing and Path Selection | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** selecting exactly one among equivalent VPLS NLRIs (same RD, VE ID, VE Block Offset) for pseudowire setup is a VPLS-PE control duty; ze is not a VPLS PE: received L2VPN routes are stored as opaque raw NLRI blobs (internal/component/bgp/plugins/adj_rib_in/rib.go:850-851), with no VPLS-specific equivalence or best-path over the (RD, VE ID, VBO) tuple, and no VSI to install a chosen pseudowire |
| `RFC4761-3.5-2` | If two PEs are assigned the same VE ID in a given VPLS, they MUST use the same Route Distinguisher (Section 3.5) | MUST | 3.5 - Multi-homing and Path Selection | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the same-VE-ID-implies-same-RD rule binds VPLS-PE provisioning; ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45, no VSI), provisions no VPLS VE identities, and holds no VE-ID-to-RD assignment to constrain |
| `RFC4761-4.2.1-1` | When a VE learns a source MAC address S on port P and later sees S on a different port P', the VE MUST update its FIB to reflect the new port (Section 4.2.1) | MUST | 4.2.1 - MAC Address Learning | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** updating the FIB when a source MAC moves to a new port is VPLS-PE data-plane bridging; ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45, no MAC FIB), so it performs no MAC learning or L2 forwarding |
| `RFC4761-4.2.2-1` | If the age of source MAC S exceeds aging time T, S MUST be flushed from the FIB (Section 4.2.2) | MUST | 4.2.2 - Aging | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** flushing an aged-out source MAC from the FIB is VPLS-PE data-plane bridging; ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45), and maintains no MAC FIB to age |
| `RFC4761-4.2.5-1` | For split horizon: PE MUST NOT send frames received from another PE to other PEs (Section 4.2.5) | MUST NOT | 4.2.5 - 'Split Horizon' Forwarding | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** split-horizon flooding (never sending a PE-sourced frame back to other PEs) is VPLS-PE data-plane forwarding; ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45, no VPLS forwarding data path), so it floods no VPLS frames |
| `RFC4761-6-1` | Any implementation using MPLS-in-IP/GRE tunnels for VPLS MUST contain an IPsec implementation (Section 6) | MUST | 6 - Security Considerations | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45, no VSI, no VPLS forwarding data path), so it tunnels no VPLS packets over MPLS-in-IP/GRE and the must-contain-IPsec trigger never fires (the IKE/IPsec engine in internal/component/ike is unrelated to VPLS tunneling) |
| `RFC4761-6-2` | If not using IPsec, implementation MUST allow egress PE to validate the IP source address of any tunneled packet (Section 6) | MUST | 6 - Security Considerations | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** validating the IP source address of a tunneled VPLS packet is an egress-PE data-plane check; ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45, no VPLS forwarding data path), so there is no MPLS-in-IP/GRE VPLS data path at an egress PE to validate a packet's source |
| `RFC4761-3.3-2` | When all CE links go down, PE SHOULD either withdraw all NLRIs or signal unavailability (Section 3.3) | SHOULD | 3.3 - BGP VPLS Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC4761-3.5-3` | Multi-homed PEs with same VE ID SHOULD announce the same VE Block Size for a given VE Offset (Section 3.5) | SHOULD | 3.5 - Multi-homing and Path Selection | **positive:** no positive test. **negative:** no negative test |
| `RFC4761-4.2.2-2` | VPLS PEs SHOULD have an aging mechanism to remove MAC addresses (Section 4.2.2) | SHOULD | 4.2.2 - Aging | **positive:** no positive test. **negative:** no negative test |
| `RFC4761-4.2.2-3` | An implementation SHOULD provide a configurable knob to set the aging time T on a per-VPLS basis (Section 4.2.2) | SHOULD | 4.2.2 - Aging | **positive:** no positive test. **negative:** no negative test |
| `RFC4761-4.2.7-1` | An implementation SHOULD allow the 802.1p to EXP mapping function to be different for each VPLS (Section 4.2.7) | SHOULD | 4.2.7 - Class of Service | **positive:** no positive test. **negative:** no negative test |
| `RFC4761-4.2.1-2` | A VE MAY implement a mechanism to damp flapping of source ports for a given MAC address (Section 4.2.1) | MAY | 4.2.1 - MAC Address Learning | **positive:** no positive test. **negative:** no negative test |
| `RFC4761-4.2.2-4` | An implementation MAY accelerate aging of all MAC addresses on topology change (Section 4.2.2) | MAY | 4.2.2 - Aging | **positive:** no positive test. **negative:** no negative test |
| `RFC4761-4.2.4-1` | A VE MAY use techniques to restrict multicast frame transmission to a smaller set of receivers (Section 4.2.4) | MAY | 4.2.4 - Broadcast and Multicast | **positive:** no positive test. **negative:** no negative test |
| `RFC4761-4.2.7-2` | An implementation MAY choose to map 802.1p bits to EXP bits for Class of Service (Section 4.2.7) | MAY | 4.2.7 - Class of Service | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4761-2.3-1`](#rfc4761-2.3-1) An implementation MUST maintain a separate routing storage for each service (Section 2.3) | no test | no test carries this requirement id; annotated {not-applicable}: maintaining a separate routing/forwarding store per VPLS service is a VPLS-PE VSI duty; ze is not a VPLS PE: its RFC 4761 support is a control-plane L2VPN/VPLS (AFI 25, SAFI 65) NLRI codec (internal/component/bgp/plugins/nlri/vpls/vpls.go:45 Mode decode; types.go:82 ParseVPLS, types.go:163 WriteTo) plus route-injection encoders, with no VPLS instance (VSI), no pseudowire signaling, no MAC FIB, and no VPLS forwarding data path, so there is no per-service VSI store to maintain |
| [`RFC4761-3.2.4-1`](#rfc4761-3.2.4-1) MBZ bits in Control Flags MUST be set to zero when sending (Section 3.2.4) | no test | no test carries this requirement id; annotated {not-applicable}: signalling the Layer2 Info Extended Community (0x800A) Control Flags is a duty of a VPLS PE originating auto-discovery; ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45, no VSI, no PW signaling), and the only l2info code is an operator-driven raw ext-community encoder that packs the supplied control byte verbatim (internal/component/bgp/config/routeattr_community.go:283-305 parseL2InfoExtCommunity), not a PE Control-Flags sender, so ze originates no VPLS Control Flags whose MBZ bits it must zero |
| [`RFC4761-3.2.4-2`](#rfc4761-3.2.4-2) MBZ bits in Control Flags MUST be ignored when receiving (Section 3.2.4) | no test | no test carries this requirement id; annotated {not-applicable}: ignoring MBZ bits on receipt of the Layer2 Info Control Flags is a VPLS-PE receive duty; ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45, no VSI, no PW state) and has no Layer2 Info decoder, so no MBZ-on-receive processing exists to exercise |
| [`RFC4761-3.2.4-3`](#rfc4761-3.2.4-3) When C flag is 1, a Control Word MUST be present when sending VPLS packets to that PE (Section 3.2.4) | no test | no test carries this requirement id; annotated {not-applicable}: inserting a Control Word into VPLS packets when the C flag is 1 is a pseudowire data-plane encapsulation choice; ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45, no VSI, no PW signaling, no VPLS forwarding data path), so it sends no VPLS packets and the C-flag data-plane obligation has no host |
| [`RFC4761-3.2.4-4`](#rfc4761-3.2.4-4) When C flag is 0, a Control Word MUST NOT be present when sending VPLS packets to that PE (Section 3.2.4) | no test | no test carries this requirement id; annotated {not-applicable}: omitting the Control Word from VPLS packets when the C flag is 0 is a pseudowire data-plane choice; ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45, no VSI, no VPLS forwarding data path), so it sends no VPLS packets to constrain |
| [`RFC4761-3.2.4-5`](#rfc4761-3.2.4-5) When S flag is 1, sequenced delivery of frames MUST be used when sending VPLS packets to that PE (Section 3.2.4) | no test | no test carries this requirement id; annotated {not-applicable}: using sequenced delivery for VPLS packets when the S flag is 1 is a pseudowire data-plane behavior; ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45, no VSI, no VPLS forwarding data path), so it delivers no VPLS packets |
| [`RFC4761-3.2.4-6`](#rfc4761-3.2.4-6) When S flag is 0, sequenced delivery MUST NOT be used when sending VPLS packets to that PE (Section 3.2.4) | no test | no test carries this requirement id; annotated {not-applicable}: refraining from sequenced delivery when the S flag is 0 is a pseudowire data-plane behavior; ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45, no VSI, no VPLS forwarding data path), so it delivers no VPLS packets |
| [`RFC4761-3.2.3-1`](#rfc4761-3.2.3-1) If announcing PE's VE ID is not covered by any remote VE set that PE-b announced, PE-b MUST make a new announcement covering it (Section 3.2.3) | no test | no test carries this requirement id; annotated {not-applicable}: computing and announcing a new label block to cover a peer's VE ID is VPLS-PE auto-discovery signaling; ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45, no VSI), and runs no label-block allocation or pseudowire-setup algorithm |
| [`RFC4761-3.2.3-2`](#rfc4761-3.2.3-2) If Y withdraws an NLRI for V that X was using, X MUST tear down its ends of the pseudowire between X and Y (Section 3.2.3) | no test | no test carries this requirement id; annotated {not-applicable}: tearing down pseudowire ends when a peer withdraws its NLRI is a VPLS-PE data-plane action; ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45, no VSI, no VPLS forwarding data path), so it establishes and tears down no pseudowires |
| [`RFC4761-3.3-1`](#rfc4761-3.3-1) PE-a MUST withdraw all its announcements for VPLS foo that contain VE ID V when V is removed from configuration (Section 3.3) | no test | no test carries this requirement id; annotated {not-applicable}: withdrawing a PE's own VPLS announcements when a VE ID leaves the VSI configuration is a VPLS-PE lifecycle duty; ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45), and has no VSI/VE configuration model that originates or withdraws such announcements |
| [`RFC4761-3.4.2-1`](#rfc4761-3.4.2-1) Inter-AS Method (b): Length, Route Distinguisher, VE ID, VE Block Offset, and VE Block Size MUST be the same when ASBR re-advertises (Section 3.4.2) | no test | no test carries this requirement id; annotated {not-applicable}: re-advertising a VPLS NLRI with unchanged Length/RD/VE-ID/VE-Block-Offset/VE-Block-Size while swapping labels is a VPLS ASBR duty (Inter-AS option b); ze is not a VPLS PE or ASBR (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45, no VSI, no VPLS forwarding data path), and performs no VPLS ASBR label-swap re-advertisement |
| [`RFC4761-3.5-1`](#rfc4761-3.5-1) When receiving equivalent NLRIs from multiple PEs, MUST pick only one via BGP path selection (Section 3.5) | no test | no test carries this requirement id; annotated {not-applicable}: selecting exactly one among equivalent VPLS NLRIs (same RD, VE ID, VE Block Offset) for pseudowire setup is a VPLS-PE control duty; ze is not a VPLS PE: received L2VPN routes are stored as opaque raw NLRI blobs (internal/component/bgp/plugins/adj_rib_in/rib.go:850-851), with no VPLS-specific equivalence or best-path over the (RD, VE ID, VBO) tuple, and no VSI to install a chosen pseudowire |
| [`RFC4761-3.5-2`](#rfc4761-3.5-2) If two PEs are assigned the same VE ID in a given VPLS, they MUST use the same Route Distinguisher (Section 3.5) | no test | no test carries this requirement id; annotated {not-applicable}: the same-VE-ID-implies-same-RD rule binds VPLS-PE provisioning; ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45, no VSI), provisions no VPLS VE identities, and holds no VE-ID-to-RD assignment to constrain |
| [`RFC4761-4.2.1-1`](#rfc4761-4.2.1-1) When a VE learns a source MAC address S on port P and later sees S on a different port P', the VE MUST update its FIB to reflect the new port (Section 4.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: updating the FIB when a source MAC moves to a new port is VPLS-PE data-plane bridging; ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45, no MAC FIB), so it performs no MAC learning or L2 forwarding |
| [`RFC4761-4.2.2-1`](#rfc4761-4.2.2-1) If the age of source MAC S exceeds aging time T, S MUST be flushed from the FIB (Section 4.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: flushing an aged-out source MAC from the FIB is VPLS-PE data-plane bridging; ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45), and maintains no MAC FIB to age |
| [`RFC4761-4.2.5-1`](#rfc4761-4.2.5-1) For split horizon: PE MUST NOT send frames received from another PE to other PEs (Section 4.2.5) | no test | no test carries this requirement id; annotated {not-applicable}: split-horizon flooding (never sending a PE-sourced frame back to other PEs) is VPLS-PE data-plane forwarding; ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45, no VPLS forwarding data path), so it floods no VPLS frames |
| [`RFC4761-6-1`](#rfc4761-6-1) Any implementation using MPLS-in-IP/GRE tunnels for VPLS MUST contain an IPsec implementation (Section 6) | no test | no test carries this requirement id; annotated {not-applicable}: ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45, no VSI, no VPLS forwarding data path), so it tunnels no VPLS packets over MPLS-in-IP/GRE and the must-contain-IPsec trigger never fires (the IKE/IPsec engine in internal/component/ike is unrelated to VPLS tunneling) |
| [`RFC4761-6-2`](#rfc4761-6-2) If not using IPsec, implementation MUST allow egress PE to validate the IP source address of any tunneled packet (Section 6) | no test | no test carries this requirement id; annotated {not-applicable}: validating the IP source address of a tunneled VPLS packet is an egress-PE data-plane check; ze is not a VPLS PE (control-plane VPLS NLRI codec at internal/component/bgp/plugins/nlri/vpls/vpls.go:45, no VPLS forwarding data path), so there is no MPLS-in-IP/GRE VPLS data path at an egress PE to validate a packet's source |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4761-2.3-1`](#rfc4761-2.3-1)

An implementation MUST maintain a separate routing storage for each service (Section 2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4761-2.3-1, so no unit is bound to it.

### [`RFC4761-3.2.4-1`](#rfc4761-3.2.4-1)

MBZ bits in Control Flags MUST be set to zero when sending (Section 3.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4761-3.2.4-1, so no unit is bound to it.

### [`RFC4761-3.2.4-2`](#rfc4761-3.2.4-2)

MBZ bits in Control Flags MUST be ignored when receiving (Section 3.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4761-3.2.4-2, so no unit is bound to it.

### [`RFC4761-3.2.4-3`](#rfc4761-3.2.4-3)

When C flag is 1, a Control Word MUST be present when sending VPLS packets to that PE (Section 3.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4761-3.2.4-3, so no unit is bound to it.

### [`RFC4761-3.2.4-4`](#rfc4761-3.2.4-4)

When C flag is 0, a Control Word MUST NOT be present when sending VPLS packets to that PE (Section 3.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4761-3.2.4-4, so no unit is bound to it.

### [`RFC4761-3.2.4-5`](#rfc4761-3.2.4-5)

When S flag is 1, sequenced delivery of frames MUST be used when sending VPLS packets to that PE (Section 3.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4761-3.2.4-5, so no unit is bound to it.

### [`RFC4761-3.2.4-6`](#rfc4761-3.2.4-6)

When S flag is 0, sequenced delivery MUST NOT be used when sending VPLS packets to that PE (Section 3.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4761-3.2.4-6, so no unit is bound to it.

### [`RFC4761-3.2.3-1`](#rfc4761-3.2.3-1)

If announcing PE's VE ID is not covered by any remote VE set that PE-b announced, PE-b MUST make a new announcement covering it (Section 3.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4761-3.2.3-1, so no unit is bound to it.

### [`RFC4761-3.2.3-2`](#rfc4761-3.2.3-2)

If Y withdraws an NLRI for V that X was using, X MUST tear down its ends of the pseudowire between X and Y (Section 3.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4761-3.2.3-2, so no unit is bound to it.

### [`RFC4761-3.3-1`](#rfc4761-3.3-1)

PE-a MUST withdraw all its announcements for VPLS foo that contain VE ID V when V is removed from configuration (Section 3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4761-3.3-1, so no unit is bound to it.

### [`RFC4761-3.4.2-1`](#rfc4761-3.4.2-1)

Inter-AS Method (b): Length, Route Distinguisher, VE ID, VE Block Offset, and VE Block Size MUST be the same when ASBR re-advertises (Section 3.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4761-3.4.2-1, so no unit is bound to it.

### [`RFC4761-3.5-1`](#rfc4761-3.5-1)

When receiving equivalent NLRIs from multiple PEs, MUST pick only one via BGP path selection (Section 3.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4761-3.5-1, so no unit is bound to it.

### [`RFC4761-3.5-2`](#rfc4761-3.5-2)

If two PEs are assigned the same VE ID in a given VPLS, they MUST use the same Route Distinguisher (Section 3.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4761-3.5-2, so no unit is bound to it.

### [`RFC4761-4.2.1-1`](#rfc4761-4.2.1-1)

When a VE learns a source MAC address S on port P and later sees S on a different port P', the VE MUST update its FIB to reflect the new port (Section 4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4761-4.2.1-1, so no unit is bound to it.

### [`RFC4761-4.2.2-1`](#rfc4761-4.2.2-1)

If the age of source MAC S exceeds aging time T, S MUST be flushed from the FIB (Section 4.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4761-4.2.2-1, so no unit is bound to it.

### [`RFC4761-4.2.5-1`](#rfc4761-4.2.5-1)

For split horizon: PE MUST NOT send frames received from another PE to other PEs (Section 4.2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4761-4.2.5-1, so no unit is bound to it.

### [`RFC4761-6-1`](#rfc4761-6-1)

Any implementation using MPLS-in-IP/GRE tunnels for VPLS MUST contain an IPsec implementation (Section 6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4761-6-1, so no unit is bound to it.

### [`RFC4761-6-2`](#rfc4761-6-2)

If not using IPsec, implementation MUST allow egress PE to validate the IP source address of any tunneled packet (Section 6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4761-6-2, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6, rfc4761 |
| Signed off | 2026-08-31 |
| Register | prose |
| Source | rfc/full/rfc4761.txt |
| Source fingerprint | 26c76afd3dd9f253 |
| Record | rfc/extraction/rfc4761.json |
| Mapped sentences | 16 |
| Declined as scope | 26 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | walked | Title block, Status of This Memo, Copyright Notice, IESG Note, Abstract and Table of Contents. Walked rather than skipped because the site scan attributes one site here, the Abstract's statement of what the document describes, excluded below. The IESG Note records that RFC 4762 and this document both perform VPLS in different, incompatible manners. |
| `1` | Introduction | 0 | walked | Introduction. Indicative prose: what a VPLS is, that it glues individual LANs across a packet switched network by MAC learning, flooding and forwarding over pseudowires, what this document adds (auto-discovery and signaling over BGP, and transport of VPLS frames over tunnels), and which alternatives exist, RFC 4762's LDP-signaled VPLS among them. No directive and no site. |
| `1.1` | Scope of This Document | 1 | walked | Scope of This Document. A roadmap: the functional model in section 2, the BGP control plane in section 3, the forwarding plane in section 4, the deployment options in section 5. Its one site is the roadmap sentence pointing at section 4 and is excluded below. |
| `1.2` | Conventions Used in This Document | 0 | walked | Conventions Used in This Document. The RFC 2119 key-words paragraph. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. |
| `2` | Functional Model | 0 | walked | Functional Model. One sentence introducing Figure 1, the example VPLS with CEs, PEs, a u-PE and five customer sites. No directive and no site. |
| `2.1` | Terminology | 0 | walked | Terminology. Defines SP, P, PE, CE, u-PE, VE and demultiplexor, records that PE and u-PE devices are VPLS-aware while a CE is not, and that the demultiplexor in this document is an MPLS label. Definitions, not directives, and no site. |
| `2.2` | Assumptions | 0 | walked | Assumptions. States the assumptions the rest of the document rests on: a packet switched SP network, PEs logically fully meshed with tunnels established outside this document, flooding and learning private to each SP device, and a bidirectional pseudowire between every pair of PEs in a VPLS. Indicative throughout and no site. |
| `2.3` | Interactions | 1 | walked | Interactions. States what makes VPLS a LAN service, private and virtual, that PE interactions are control-driven rather than data-driven, and that a PE can run VPLS and IP VPNs at once because the two use different AFI/SAFI pairs. Its one capitalised MUST-level site is the separate-routing-storage obligation that follows from that, mapped below to RFC4761-2.3-1. |
| `3` | Control Plane | 0 | walked | Control Plane. One paragraph naming the two control-plane functions, auto-discovery and signaling, and pointing at sections 3.1 to 3.5. No directive and no site. |
| `3.1` | Auto-Discovery | 2 | walked | Auto-Discovery. Contrasts configuring every PE by hand with discovering them by protocol. Both sites belong to the description of the manual alternative and are excluded below; the paragraph on auto-discovery that follows states what BGP buys and directs nobody. |
| `3.1.1` | Functions | 3 | walked | Functions. States what the discovery function has to provide: a PE must be able to tell the others it is a member of a VPLS, to declare that it no longer participates, and so to have a means of identifying a VPLS and of communicating with all other PEs. All three sites bind the VPLS PE performing auto-discovery and are excluded below. |
| `3.1.2` | Protocol Specification | 0 | walked | Protocol Specification. States that the mechanism uses the Route Target extended community of RFC 4360 to identify VPLS members with RFC 4364's semantics, that one RT suffices for a fully meshed VPLS, and how a PE announces membership by annotating its NLRIs with that RT and withdraws it by withdrawing them. Indicative throughout and no site. |
| `3.2` | Signaling | 1 | walked | Signaling. States what signaling is (each pair of PEs establishing and tearing down pseudowires, that is exchanging and withdrawing demultiplexors), what a demultiplexor carries, and why one common Update carrying demultiplexors for every remote PE beats N individual messages. Its one site is the pseudowire-establishment sentence, which binds the VPLS PE and is excluded below. |
| `3.2.1` | Label Blocks | 0 | walked | Label Blocks. Defines a label block as the contiguous set {LB, ..., LB+VBS-1}, extends it with the VE block offset so a block is {LB+VBO, ..., LB+VBO+VBS-1}, and explains how each receiving PE infers its own demultiplexor by adding its VE ID to the label base. Definitions, not directives, and no site. |
| `3.2.2` | VPLS BGP NLRI | 1 | walked | VPLS BGP NLRI. The document's wire-format section: AFI 25, SAFI 65, and the NLRI's Length, Route Distinguisher, VE ID, VE Block Offset, VE Block Size and 3-octet Label Base, with the one-to-one correspondence between the remote VE set and the label block. The layout is carried by the Wire Formats tables of rfc/short/rfc4761.md, and it is the part of this RFC ze does implement: internal/component/bgp/plugins/nlri/vpls.ParseVPLS decodes exactly these fields and EncodeRoute builds them. Its one site is the VE ID provisioning sentence, which binds the VPLS PE and is excluded below. |
| `3.2.3` | PW Setup and Teardown | 2 | walked | PW Setup and Teardown. The four-step procedure PE-b runs on an announcement from PE-a, and the teardown rule when an NLRI in use is withdrawn. Its two capitalised MUST-level sites are mapped below to RFC4761-3.2.3-1 and RFC4761-3.2.3-2; steps 1, 2 and 4 are stated in the indicative and the site scan sees nothing in them. |
| `3.2.4` | Signaling PE Capabilities | 4 | walked | Signaling PE Capabilities. Defines the Layer2 Info Extended Community (0x800A) carrying Encaps Type, Control Flags and Layer-2 MTU, with Encaps Type 19 for VPLS, and the C and S Control Flags. Three of its four sites carry the section's obligations and are mapped below to RFC4761-3.2.4-1, -3 and -5. The other half of each of those three sentences is a second declared row, listed here as an unsourced id, because the splitter keeps one sentence as one site: RFC4761-3.2.4-2 is 'MUST be ignored when receiving this community', RFC4761-3.2.4-4 is the C=0 'MUST NOT be present' half, and RFC4761-3.2.4-6 is the S=0 'MUST NOT be used' half. Site 3.2.4:1 is the Figure 4 bit vector and its legend and is excluded below. |
| `3.3` | BGP VPLS Operation | 4 | walked | BGP VPLS Operation. Walks the whole life of a BGP VPLS announcement: the administrator picks an RT, the PE is configured with a VE ID and possibly an RD, it generates a label block and a remote VE set, builds the NLRI, attaches the Layer2 Info community and the RT, sets itself as Next Hop and announces. Site 3.3:4 carries the withdrawal obligation and is mapped below to RFC4761-3.3-1; sites 3.3:1, 3.3:2 and 3.3:3 are excluded. The SHOULD listed as an unsourced id is the CE-links-down rule: 'If all of PE-a's links to its CEs in VPLS foo go down, then PE-a SHOULD either withdraw all its NLRIs for VPLS foo or let other PEs in the VPLS foo know in some way that PE-a is no longer connected to its CEs.' |
| `3.4` | Multi-AS VPLS | 0 | walked | Multi-AS VPLS. States the problem when the sites of a VPLS attach to PEs in different ASes, introduces Figure 5 and the three methods of sections 3.4.1 to 3.4.3 in order of increasing scalability. No directive and no site. |
| `3.4.1` | Method (a): VPLS-to-VPLS Connections at the ASBRs | 1 | walked | Method (a): VPLS-to-VPLS Connections at the ASBRs. Each ASBR acts as a PE for the VPLSs spanning the two ASes and views the other ASBR as a CE, which requires Ethernet on the interconnect and full PE operation on the ASBR. Its one site is the loop-prevention rule for redundant inter-AS connections and is excluded below. |
| `3.4.2` | not stated | 3 | walked | Method (b): EBGP Redistribution of VPLS Information between ASBRs. The ASBRs re-advertise the VPLS NLRI with new labels and themselves as next hop, and swap labels in the forwarding path. Site 3.4.2:1 carries the obligation that everything but the Label Base stay identical and is mapped below to RFC4761-3.4.2-1; sites 3.4.2:2 and 3.4.2:3 are the ASBR forwarding-path installations and are excluded. |
| `3.4.3` | not stated | 0 | walked | Method (c): Multi-Hop EBGP Redistribution of VPLS Information between ASes. A multi-hop E-BGP peering between the PEs or their route reflectors, with a tunnel LSP from PE1 to PE2 and no VPLS state on the ASBRs. Indicative throughout and no site. |
| `3.4.4` | Allocation of VE IDs across Multiple ASes | 0 | walked | Allocation of VE IDs across Multiple ASes. Suggests allocating a VE ID range per AS to keep VE IDs unique while minimising the number of NLRIs, with no overlap between ranges except for multi-homing. Advisory prose with no site. |
| `3.5` | Multi-homing and Path Selection | 2 | walked | Multi-homing and Path Selection. When the PEs attached to one site carry the same VE ID, BGP path selection builds the loop-free topology, and two VPLS NLRIs are equivalent when their Route Distinguisher, VE ID and VE Block Offset match. Its two capitalised MUST-level sites are mapped below to RFC4761-3.5-1 and RFC4761-3.5-2. The unsourced id is the SHOULD that closes the second of those sentences, 'they SHOULD announce the same VE Block Size for a given VE Offset', which the splitter keeps in site 3.5:2. |
| `3.6` | Hierarchical BGP VPLS | 1 | walked | Hierarchical BGP VPLS. How BGP route reflectors scale the VPLS control plane, that RRs introduce no data plane state, that no MAC addresses are exchanged over BGP, and when BGP processing for VPLS happens. Its one site states that something is NOT required and is excluded below. |
| `4` | Data Plane | 0 | walked | Data Plane. One sentence naming the two aspects sections 4.1 and 4.2 cover, encapsulation and forwarding. No directive and no site. |
| `4.1` | Encapsulation | 0 | walked | Encapsulation. One sentence: Ethernet frames from CE devices are encapsulated for transmission over the packet switched network as in RFC 4448. No obligation of its own and no site. |
| `4.2` | Forwarding | 0 | walked | Forwarding. States that a VPLS packet is classified into a service instance by the interface it arrived on, and forwarded within that instance on the destination MAC address. Indicative and no site. |
| `4.2.1` | MAC Address Learning | 2 | walked | MAC Address Learning. The SP network appears as one logical learning bridge per VPLS, learning associates a source MAC with the logical port it arrived on, and that association is the FIB. Site 4.2.1:2 carries the MAC-move obligation and is mapped below to RFC4761-4.2.1-1; site 4.2.1:1 is the learning-bridge analogy and is excluded. The unsourced id is the MAY that closes the section, 'A VE MAY implement a mechanism to damp flapping of source ports for a given MAC address.' |
| `4.2.2` | Aging | 2 | walked | Aging. Site 4.2.2:2 carries the capitalised MUST, mapped below to RFC4761-4.2.2-1, and site 4.2.2:1 is the rationale sentence inside the section's opening SHOULD and is excluded. Three declared rows are read from prose here and are the unsourced ids: RFC4761-4.2.2-2 is 'VPLS PEs SHOULD have an aging mechanism to remove a MAC address associated with a logical port', RFC4761-4.2.2-3 is 'An implementation SHOULD provide a configurable knob to set the aging time T on a per-VPLS basis', and RFC4761-4.2.2-4 is the MAY to accelerate aging of every MAC address in a VPLS on a Spanning Tree topology change. |
| `4.2.3` | Flooding | 0 | walked | Flooding. Describes flooding a frame whose destination is not in the FIB to every other VE in the VPLS, and works the Figure 1 example through PE2, PE1 and a u-PE, including a PE that announces whether it is capable of flooding. Indicative throughout and no site. |
| `4.2.4` | Broadcast and Multicast | 1 | walked | Broadcast and Multicast. Its one site is the broadcast-delivery rule and is excluded below. The unsourced id is the MAY that follows, 'a VE MAY also use certain techniques to restrict transmission of multicast frames to a smaller set of receivers', whose discussion the section puts out of scope. |
| `4.2.5` | 'Split Horizon' Forwarding | 4 | walked | 'Split Horizon' Forwarding. Four sites: three describe what a flooding PE sends where, and the fourth is the capitalised split-horizon prohibition mapped below to RFC4761-4.2.5-1. The other three bind the VPLS PE forwarding path and are excluded. The closing sentence extends the rule to broadcast and multicast packets as well as unknown MAC addresses. |
| `4.2.6` | Qualified and Unqualified Learning | 0 | walked | Qualified and Unqualified Learning. Defines the two learning keys, the MAC address alone or the customer VLAN tag with it, and weighs one global broadcast domain against one per VLAN. Definitions and trade-offs, no directive and no site. |
| `4.2.7` | Class of Service | 1 | walked | Class of Service. Its one site is the SHOULD that the mapping function be per-VPLS, mapped below to RFC4761-4.2.7-1. The unsourced id is the MAY that opens the section, 'an implementation MAY choose to map 802.1p bits in a customer Ethernet frame with a VLAN tag to an appropriate setting of EXP bits in the pseudowire and/or tunnel label'. |
| `5` | Deployment Options | 1 | walked | Deployment Options. Defines 'decoupled' operation: the VE closest to the customer can be the PE itself or a u-PE that does the Layer 2 functions and a limited set of Layer 3 functions, and PEs of both kinds mix in one network because discovery and signaling are unchanged. Its one site is the SP's deployment decision and is excluded below. |
| `6` | Security Considerations | 3 | walked | Security Considerations. States that VPLS offers no confidentiality, integrity or authentication, that both the control plane and the forwarding path have to be protected, that RFC 2385 authenticates the BGP exchanges, and that VPLS labels must be accepted only from valid interfaces. It closes on MPLS-in-IP and MPLS-in-GRE tunneling per RFC 4023: sites 6:2 and 6:3 carry the two capitalised MUST-level obligations and are mapped below to RFC4761-6-1 and RFC4761-6-2, and site 6:1 points at RFC 4023's own security considerations and is excluded. |
| `7` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records the allocated AFI 25 for L2VPN information and the allocated extended community value 0x800A for the Layer2 Info Extended Community. Binds IANA, not a speaker. |
| `8` | References | 0 | skipped (references) | References. The heading for sections 8.1 and 8.2. |
| `8.1` | not stated | 0 | skipped (references) | Normative References: RFC 2119, RFC 2385, RFC 4023, RFC 4760, RFC 4360, RFC 4364 and RFC 4448. |
| `8.2` | not stated | 0 | skipped (references) | Informative References: RFC 4456, RFC 4664, RFC 4762, the VR-based Layer-3 VPN and Constrained VPN Route Distribution drafts, RFC 4447, the Layer 2 VPNs Over Tunnels draft and IEEE 802.1D. |
| `A` | Appendix A, Contributors | 0 | skipped (acknowledgements) | Appendix A, Contributors. The list of people who contributed to the document. |
| `B` | Appendix B, Acknowledgements | 1 | walked | Appendix B, Acknowledgements. Because no numbered heading follows it, the derived span also carries the Editors' Addresses, the Full Copyright Statement, the Intellectual Property boilerplate and the RFC Editor funding note. Walked rather than skipped because the prose scan attributes one site here; that site is IPR boilerplate and is excluded below. Nothing in the span states an obligation on an implementation. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Non-normative use: the sentence is the Abstract's statement of the document's CONTENTS, 'This document describes the functions required to offer VPLS, a mechanism for signaling a VPLS, and rules for forwarding VPLS frames across a packet switched network.' The case-insensitive prose scan sees 'required' inside the noun phrase 'the functions required to offer VPLS', which names what the document describes rather than directing a speaker. | This document describes the functions required to offer VPLS, a mechanism for signaling a VPLS, and rules for forwarding VPLS frames across a packet switched network. |
| `1.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Non-normative use: a roadmap sentence in Scope of This Document. Its subject is where the material is written down -- 'The forwarding plane and the actions that a participating Provider Edge (PE) router offering the VPLS service must take is described in Section 4' -- so the lowercase 'must' sits in a relative clause naming what section 4 covers. The obligations themselves are in section 4 and are classified there. | The forwarding plane and the actions that a participating Provider Edge (PE) router offering the VPLS service must take is described in Section 4. |
| `3.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Non-normative use: a description of the OTHER approach, the one this document replaces. The paragraph characterises configuring each PE by hand ('The former approach is fairly configuration-intensive'), and the lowercase 'is required' states why that approach costs what it does, namely that the PEs of a VPLS are fully meshed with pseudowires. It directs no speaker, and the auto-discovery paragraph that follows is the mechanism this document specifies. | The former approach is fairly configuration-intensive, especially since it is required that the PEs participating in a given VPLS are fully meshed (i.e., that every PE in a given VPLS establish pseudowires to every other PE in that VPLS). |
| `3.1:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Non-normative use: the second half of the same characterisation of the manual approach. It states a CONSEQUENCE of not auto-discovering, that a topology change forces the VPLS configuration on every PE to change, and the paragraph that follows says auto-discovery removes it. It directs no speaker. | Furthermore, when the topology of a VPLS changes (i.e., a PE is added to, or removed from, the VPLS), the VPLS configuration on all PEs in that VPLS must be changed. |
| `3.1.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the VPLS PE performing auto-discovery: the sentence says a PE participating in VPLS instance V must be able to tell every other PE in V that it is a member. ze is not a VPLS PE: its RFC 4761 surface is the control-plane L2VPN/VPLS (AFI 25, SAFI 65) NLRI codec of internal/component/bgp/plugins/nlri/vpls (ParseVPLS decodes the section 3.2.2 NLRI, EncodeRoute and EncodeNLRIHex build one from operator-supplied fields, and the plugin registers the family in decode mode) plus the operator-driven Layer2 Info extended community encoder internal/component/bgp/config.parseL2InfoExtCommunity. There is no VSI, no pseudowire signaling, no MAC FIB and no VPLS forwarding path, and a received L2VPN route is stored as an opaque raw NLRI blob. It joins no VPLS and announces no membership. | A PE that participates in a given VPLS instance V must be able to tell all other PEs in VPLS V that it is also a member of V. |
| `3.1.1:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the same VPLS PE role: it must have a means of declaring that it no longer participates in a VPLS. Ze participates in no VPLS to leave. It decodes and encodes the VPLS NLRI (internal/component/bgp/plugins/nlri/vpls.ParseVPLS and EncodeRoute) and holds no VSI whose membership it could withdraw. | A PE must also have a means of declaring that it no longer participates in a VPLS. |
| `3.1.1:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the same VPLS PE role, stating what the two preceding sentences need: a means of identifying a VPLS and a means of communicating with every other PE. Ze plays no VPLS PE role, so it needs neither. The producer that would act as it if ze did is the whole of ze's RFC 4761 code, the VPLS NLRI codec `internal/component/bgp/plugins/nlri/vpls/vpls.go`, which codes the SAFI 65 route and nothing else: the tree holds no VPLS forwarding path, no MAC learning and no flooding for the role to inhabit. | To do both of these, the PE must have a means of identifying a VPLS and a means by which to communicate to all other PEs. |
| `3.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the VPLS PE performing signaling: each PAIR of PEs in a VPLS must be able to establish and tear down pseudowires to each other, exchanging and withdrawing demultiplexors. ze is not a VPLS PE: its RFC 4761 surface is the control-plane L2VPN/VPLS (AFI 25, SAFI 65) NLRI codec of internal/component/bgp/plugins/nlri/vpls (ParseVPLS decodes the section 3.2.2 NLRI, EncodeRoute and EncodeNLRIHex build one from operator-supplied fields, and the plugin registers the family in decode mode) plus the operator-driven Layer2 Info extended community encoder internal/component/bgp/config.parseL2InfoExtCommunity. There is no VSI, no pseudowire signaling, no MAC FIB and no VPLS forwarding path, and a received L2VPN route is stored as an opaque raw NLRI blob. It signals no pseudowire and holds no demultiplexor state. | Once discovery is done, each pair of PEs in a VPLS must be able to establish (and tear down) pseudowires to each other, i.e., exchange (and withdraw) demultiplexors. |
| `3.2.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds VPLS PE provisioning: a PE participating in a VPLS must have at least one VE ID, one per attached u-PE and possibly one for itself. Ze provisions no VE identity. Its VE ID handling is codec-level only: internal/component/bgp/plugins/nlri/vpls.ParseVPLS reads the 2-octet field off the wire and EncodeRoute writes the value the operator supplied. | A PE participating in a VPLS must have at least one VE ID. |
| `3.2.4:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Non-normative use: the site is Figure 4 itself, the Control Flags bit vector, and the keyword is in the figure's LEGEND expanding a field name -- '(MBZ = MUST Be Zero)'. The obligation the abbreviation stands for is stated in the sentence site 3.2.4:2 quotes, which is mapped to RFC4761-3.2.4-1. | 0 1 2 3 4 5 6 7 +-+-+-+-+-+-+-+-+ \| MBZ \|C\|S\| (MBZ = MUST Be Zero) +-+-+-+-+-+-+-+-+ |
| `3.3:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the network administrator provisioning the service: to create a VPLS, an administrator must pick a Route Target for it, which every PE serving that VPLS then uses. It is a provisioning act by a person, not behavior of a protocol speaker, and ze offers no VPLS service to provision. The producer that would act as it if ze did is the whole of ze's RFC 4761 code, the VPLS NLRI codec `internal/component/bgp/plugins/nlri/vpls/vpls.go`, which codes the SAFI 65 route and nothing else: the tree holds no VPLS forwarding path, no MAC learning and no flooding for the role to inhabit. | To create a new VPLS, say VPLS foo, a network administrator must pick an RT for VPLS foo, say RT-foo. |
| `3.3:2` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The obligation belongs to RFC 4760, which this document cites as [4] and which the sentence names explicitly: 'this association is required by [4], Section 5'. RFC 4760 section 5 is what binds the Network Layer protocol of the Next Hop address for an <AFI, SAFI> pair, and rfc/short/rfc4760.md carries it. This sentence records the association for <AFI=L2VPN, SAFI=VPLS> and adds no obligation of its own. | The Network Layer protocol associated with the Network Address of the Next Hop for the combination <AFI=L2VPN AFI, SAFI=VPLS SAFI> is IP; this association is required by [4], Section 5. |
| `3.3:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the VPLS PE that has discovered a peer: PE-b must set up its part of the VPLS pseudowire between itself and PE-a. ze is not a VPLS PE: its RFC 4761 surface is the control-plane L2VPN/VPLS (AFI 25, SAFI 65) NLRI codec of internal/component/bgp/plugins/nlri/vpls (ParseVPLS decodes the section 3.2.2 NLRI, EncodeRoute and EncodeNLRIHex build one from operator-supplied fields, and the plugin registers the family in decode mode) plus the operator-driven Layer2 Info extended community encoder internal/component/bgp/config.parseL2InfoExtCommunity. There is no VSI, no pseudowire signaling, no MAC FIB and no VPLS forwarding path, and a received L2VPN route is stored as an opaque raw NLRI blob. It sets up no pseudowire. | Similarly, PE-b will have discovered that PE-a is in the same VPLS, and PE-b must set up its part of the VPLS pseudowire. |
| `3.4.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the inter-AS VPLS ASBR and the PEs of method (a), which have to run the Spanning Tree Protocol or another loop-detection mechanism per VPLS when several links join the two ASes; the section itself puts how that is achieved out of scope. Ze runs no Spanning Tree Protocol and acts as neither a VPLS PE nor a VPLS ASBR. The producer that would act as it if ze did is the whole of ze's RFC 4761 code, the VPLS NLRI codec `internal/component/bgp/plugins/nlri/vpls/vpls.go`, which codes the SAFI 65 route and nothing else: the tree holds no VPLS forwarding path, no MAC learning and no flooding for the role to inhabit. | In this case, the Spanning Tree Protocol (STP) [15], or some other means of loop detection and prevention, must be run on each VPLS that spans these ASes, so that a loop-free topology can be constructed in each VPLS. |
| `3.4.2:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the inter-AS VPLS ASBR of method (b) in its FORWARDING path: ASBR1 installs a swap of each label of its own block for the corresponding label of PE1's block, with the tunnel label pushed. Ze is not a VPLS ASBR and installs no VPLS label swap. The MPLS entries it does program are RSVP-TE and LDP transport labels, through fibkernel.handleMPLSEntry (internal/plugins/fib/kernel/mpls.go), with no VPLS label block behind them. | Furthermore, ASBR1 must also update its forwarding path as follows: if the Label Base sent by PE1 is L1, the Label-block Size is N, the Label Base sent by ASBR1 is L2, and the tunnel label from ASBR1 to PE1 is T, then ASBR1 must install the following in the forwarding path: swap L2 with L1 and push T, |
| `3.4.2:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the second inter-AS VPLS ASBR of method (b), which acts as ASBR1 does except that a direct connection removes the need for a tunnel label. Ze plays no VPLS ASBR role. The producer that would act as it if ze did is the whole of ze's RFC 4761 code, the VPLS NLRI codec `internal/component/bgp/plugins/nlri/vpls/vpls.go`, which codes the SAFI 65 route and nothing else: the tree holds no VPLS forwarding path, no MAC learning and no flooding for the role to inhabit. | ASBR2 must act similarly, except that it may not need a tunnel label if it is directly connected with ASBR1. |
| `3.6:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Non-normative use: the sentence states that something is NOT required. It records a consequence of route reflection, that no single set of RRs has to handle every BGP message and no single RR every message from a given PE, and the rest of the paragraph gives examples of partitioning RRs by service. It directs nobody. | Another consequence of this approach is that it is not required that one set of RRs handles all BGP messages, or that a particular RR handle all messages from a given PE. |
| `4.2.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the VPLS PE bridging data path: the SP bridge must learn MAC addresses at its VEs, as a learning bridge learns them on its ports. ze is not a VPLS PE: its RFC 4761 surface is the control-plane L2VPN/VPLS (AFI 25, SAFI 65) NLRI codec of internal/component/bgp/plugins/nlri/vpls (ParseVPLS decodes the section 3.2.2 NLRI, EncodeRoute and EncodeNLRIHex build one from operator-supplied fields, and the plugin registers the family in decode mode) plus the operator-driven Layer2 Info extended community encoder internal/component/bgp/config.parseL2InfoExtCommunity. There is no VSI, no pseudowire signaling, no MAC FIB and no VPLS forwarding path, and a received L2VPN route is stored as an opaque raw NLRI blob. It learns no MAC address and holds no FIB. | Just as a learning bridge learns MAC addresses on its ports, the SP bridge must learn MAC addresses at its VEs. |
| `4.2.2:1` | `advisory-in-context` (never bound Ze): the sentence advises on applying a rule stated elsewhere and adds no obligation of its own | The enclosing construction is a SHOULD: 'VPLS PEs SHOULD have an aging mechanism to remove a MAC address associated with a logical port, much the same as learning bridges do. This is required so that a MAC address can be relearned if it "moves" from a logical port to another logical port ...'. The site is that second sentence, which gives the RATIONALE for the SHOULD rather than stating an obligation of its own, so its 'is required' asserts no level. rfc/short/rfc4761.md declares the enclosing SHOULD as RFC4761-4.2.2-2, listed as an unsourced id on this section. | This is required so that a MAC address can be relearned if it "moves" from a logical port to another logical port, either because the station to which that MAC address belongs really has moved or because of a topology change in the LAN that causes this MAC address to arrive on a new port. |
| `4.2.4:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the VPLS PE forwarding path: a frame addressed to the broadcast MAC address must reach all stations in that VPLS, by the same means used for flooding. Ze forwards no VPLS frame and floods nothing; it holds no VSI and no MAC FIB, only the control-plane NLRI codec of internal/component/bgp/plugins/nlri/vpls. | An Ethernet frame whose destination MAC address is the broadcast MAC address must be sent to all stations in that VPLS. |
| `4.2.5:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the flooding-capable VPLS PE: on a broadcast frame or one with an unknown destination MAC, it must flood. Ze is not a VPLS PE and floods no VPLS frame. The capitalised split-horizon prohibition that closes this section is site 4.2.5:4, mapped below because rfc/short/rfc4761.md declares it. The producer that would act as it if ze did is the whole of ze's RFC 4761 code, the VPLS NLRI codec `internal/component/bgp/plugins/nlri/vpls/vpls.go`, which codes the SAFI 65 route and nothing else: the tree holds no VPLS forwarding path, no MAC learning and no flooding for the role to inhabit. | When a PE capable of flooding (say PEx) receives a broadcast Ethernet frame, or one with an unknown destination MAC address, it must flood the frame. |
| `4.2.5:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the same flooding-capable VPLS PE for a frame arriving from an attached CE: a copy goes to every other attached CE and to every other PE in the VPLS. Ze has no attached CE, no VPLS peer set and no VPLS forwarding path. The producer that would act as it if ze did is the whole of ze's RFC 4761 code, the VPLS NLRI codec `internal/component/bgp/plugins/nlri/vpls/vpls.go`, which codes the SAFI 65 route and nothing else: the tree holds no VPLS forwarding path, no MAC learning and no flooding for the role to inhabit. | If the frame arrived from an attached CE, PEx must send a copy of the frame to every other attached CE, as well as to all other PEs participating in the VPLS. |
| `4.2.5:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the same PE for a frame arriving from another PE: the copy goes only to attached CEs. Ze forwards no VPLS frame from any source. The producer that would act as it if ze did is the whole of ze's RFC 4761 code, the VPLS NLRI codec `internal/component/bgp/plugins/nlri/vpls/vpls.go`, which codes the SAFI 65 route and nothing else: the tree holds no VPLS forwarding path, no MAC learning and no flooding for the role to inhabit. | If, on the other hand, the frame arrived from another PE (say PEy), PEx must send a copy of the packet only to attached CEs. |
| `5:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the Service Provider deploying the network: the SP decides which functions the VPLS-aware device closest to the customer supports, that is whether the VE is the PE itself or a u-PE. It is a deployment decision by an operator, and ze deploys no VPLS service. The producer that would act as it if ze did is the whole of ze's RFC 4761 code, the VPLS NLRI codec `internal/component/bgp/plugins/nlri/vpls/vpls.go`, which codes the SAFI 65 route and nothing else: the tree holds no VPLS forwarding path, no MAC learning and no flooding for the role to inhabit. | In deploying a network that supports VPLS, the SP must decide what functions the VPLS-aware device closest to the customer (the VE) supports. |
| `6:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The obligation belongs to RFC 4023, 'Encapsulating MPLS in IP or Generic Routing Encapsulation (GRE)', which this document cites as [3] and which the sentence points at by section: 'the security considerations described in Section 8 of that document must be fully understood'. The sentence directs a reader to another document's analysis and states no requirement of its own. The two obligations this section does state about those tunnels are sites 6:2 and 6:3, mapped below. | If it is desired to use such tunnels to carry VPLS packets, then the security considerations described in Section 8 of that document must be fully understood. |
| `B:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Non-normative use: this is the IETF's standard Intellectual Property boilerplate, which the derived section span carries into Appendix B after the Acknowledgements. The sentence invites interested parties to disclose patent rights to the IETF, and its 'may be required to implement this standard' describes what a patent might cover rather than requiring anything of an implementation. | The IETF invites any interested party to bring to its attention any copyrights, patents or patent applications, or other proprietary rights that may cover technology that may be required to implement this standard. |

## Superseded

No document obsoletes RFC 4761, so its obligations are stated where they were written.
